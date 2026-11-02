/**
 * Egress bridge lokal — dijalankan di Mac admin.
 *
 * Kenapa: situs di belakang Cloudflare menolak IP datacenter VPS *dan* IP rumah
 * (challenge "Just a moment"). Tetapi Cloudflare Worker `crawl-egress` berhasil
 * bila dipanggil dari jaringan Mac ini (konteks IP pemanggil yang diterima).
 * Proxy ini meneruskan request ke Worker tersebut, dan VPS mengakses proxy ini
 * lewat SSH reverse tunnel (127.0.0.1:8787).
 *
 * Kontrak SAMA dengan Worker:
 *   POST /  header x-crawl-token, JSON {"url":"https://..."} -> {status,headers,body}
 *   GET /health -> {ok:true}
 *
 * Pemakaian:
 *   CRAWL_EGRESS_TOKEN=<token> \
 *   CF_WORKER_URL=https://tauthub-crawl-egress.<sub>.workers.dev \
 *   bun run local-worker/egress-proxy.ts
 *
 * Jika CF_WORKER_URL kosong, proxy menarik langsung lewat IP rumah.
 */

const TOKEN = process.env.CRAWL_EGRESS_TOKEN ?? "";
const CF_URL = (process.env.CF_WORKER_URL ?? "").replace(/\/$/, "");
const PORT = Number(process.env.PORT ?? 8787);
const HOST = process.env.HOST ?? "127.0.0.1";
const MAX_BODY = 2 * 1024 * 1024;

const UA =
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36";

const json = (obj: unknown, status = 200) =>
	new Response(JSON.stringify(obj), {
		status,
		headers: { "content-type": "application/json; charset=utf-8", "cache-control": "no-store" }
	});

function isPrivateIPv4(ip: string) {
	const [a, b] = ip.split(".").map(Number);
	if (a === 10 || a === 127 || a === 0) return true;
	if (a === 169 && b === 254) return true;
	if (a === 172 && b >= 16 && b <= 31) return true;
	if (a === 192 && b === 168) return true;
	if (a === 100 && b >= 64 && b <= 127) return true;
	if (a >= 224) return true;
	return false;
}

function isPrivateIPv6(ip: string) {
	const v = ip.toLowerCase();
	return v === "::1" || v.startsWith("fc") || v.startsWith("fd") || v.startsWith("fe80");
}

/** Resolve via public DNS supaya workers.dev tidak dibajak blockpage ISP. */
async function publicLookup(hostname: string): Promise<string> {
	const { Resolver } = await import("node:dns/promises");
	const resolver = new Resolver({ timeout: 3000, tries: 2 });
	resolver.setServers(["1.1.1.1", "8.8.8.8"]);
	try {
		const addrs = await resolver.resolve4(hostname);
		if (addrs.length) return addrs[0];
	} catch {
		/* fallback di bawah */
	}
	const { lookup } = await import("node:dns/promises");
	const all = await lookup(hostname, { all: true, verbatim: true });
	const ip = all.find((a) => !(a.address.includes(":") ? isPrivateIPv6(a.address) : isPrivateIPv4(a.address)));
	if (!ip) throw new Error("DNS tidak menemukan alamat publik");
	return ip.address;
}

/** Teruskan ke Cloudflare Worker, resolve host lewat public DNS. */
async function viaWorker(url: string): Promise<Response> {
	const { request } = await import("node:https");
	const u = new URL(CF_URL + "/");
	const payload = JSON.stringify({ url });
	const ip = await publicLookup(u.hostname);
	return await new Promise<Response>((resolve, reject) => {
		const req = request(
			{
				hostname: ip,
				servername: u.hostname,
				port: 443,
				path: "/",
				method: "POST",
				headers: {
					host: u.hostname,
					"content-type": "application/json",
					"x-crawl-token": TOKEN,
					"content-length": Buffer.byteLength(payload)
				},
				timeout: 25000
			},
			(res) => {
				let data = "";
				res.setEncoding("utf8");
				res.on("data", (c) => (data += c));
				res.on("end", () => resolve(new Response(data, { status: res.statusCode ?? 502 })));
			}
		);
		req.on("error", reject);
		req.on("timeout", () => req.destroy(new Error("timeout ke Worker")));
		req.end(payload);
	});
}

async function direct(url: string): Promise<Response> {
	const res = await fetch(url, {
		headers: {
			"user-agent": UA,
			accept: "text/html,application/xhtml+xml,application/xml,application/json;q=0.9,*/*;q=0.8",
			"accept-language": "id-ID,id;q=0.9,en;q=0.8"
		},
		redirect: "follow",
		signal: AbortSignal.timeout(15000)
	});
	const text = (await res.text()).slice(0, MAX_BODY);
	const headers: Record<string, string> = {};
	for (const k of ["content-type", "x-wp-total", "retry-after", "cf-mitigated", "server", "location"]) {
		const v = res.headers.get(k);
		if (v) headers[k] = v;
	}
	return json({ status: res.status, headers, body: text, final_url: res.url });
}

async function handle(req: Request): Promise<Response> {
	const url = new URL(req.url);
	if (req.method === "GET" && (url.pathname === "/" || url.pathname === "/health")) {
		return json({ ok: true, service: "tauthub-egress-proxy", via: CF_URL ? "worker" : "direct" });
	}
	if (req.method !== "POST") return json({ error: "method not allowed" }, 405);
	if (!TOKEN || req.headers.get("x-crawl-token") !== TOKEN) return json({ error: "unauthorized" }, 401);

	let body: any;
	try {
		body = await req.json();
	} catch {
		return json({ error: "invalid json" }, 400);
	}
	const target = String(body?.url ?? "").trim();
	let t: URL;
	try {
		t = new URL(target);
	} catch {
		return json({ error: "invalid url" }, 400);
	}
	if (t.protocol !== "http:" && t.protocol !== "https:") return json({ error: "unsupported protocol" }, 400);

	try {
		return CF_URL ? await viaWorker(target) : await direct(target);
	} catch (e) {
		return json({ error: String((e as Error)?.message ?? e) }, 502);
	}
}

Bun.serve({ hostname: HOST, port: PORT, fetch: handle });
console.log(`[egress-proxy] listening ${HOST}:${PORT} via=${CF_URL ? "worker" : "direct"} token_set=${TOKEN !== ""}`);
