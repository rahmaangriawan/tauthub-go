// Tauthub Crawl Egress — proxy fetch untuk fallback Content Velocity.
//
// Kenapa ada: IP VPS (datacenter) sering ditolak WAF/Cloudflare mitra (403),
// sementara request yang keluar dari jaringan Cloudflare sering lolos. API
// memanggil worker ini untuk mengambil halaman/sitemap/feed media.
//
// POST /  (header x-crawl-token: <CRAWL_TOKEN>, JSON {"url": "https://..."})
//   -> {"status": 200, "headers": {...}, "body": "..."}
// GET /health -> {"ok": true}
//
// Secret CRAWL_TOKEN di-set lewat `wrangler secret put CRAWL_TOKEN`.
// Hanya fetch http/https publik; host privat/loopback ditolak.

const UA =
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36";
const MAX_BODY = 2 * 1024 * 1024;

function isPrivateHost(host) {
	const h = String(host || "").toLowerCase();
	if (!h) return true;
	if (h === "localhost" || h.endsWith(".localhost") || h.endsWith(".internal") || h.endsWith(".local")) {
		return true;
	}
	const m = h.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/);
	if (m) {
		const a = Number(m[1]);
		const b = Number(m[2]);
		if (a === 0 || a === 10 || a === 127) return true;
		if (a === 169 && b === 254) return true;
		if (a === 172 && b >= 16 && b <= 31) return true;
		if (a === 192 && b === 168) return true;
		if (a === 100 && b >= 64 && b <= 127) return true;
	}
	return false;
}

const json = (obj, status = 200) =>
	new Response(JSON.stringify(obj), {
		status,
		headers: { "content-type": "application/json; charset=utf-8", "cache-control": "no-store" }
	});

export default {
	async fetch(request, env) {
		const url = new URL(request.url);
		if (request.method === "GET" && (url.pathname === "/" || url.pathname === "/health")) {
			return json({ ok: true, service: "tauthub-crawl-egress" });
		}
		if (request.method !== "POST") {
			return json({ error: "method not allowed" }, 405);
		}
		if (!env.CRAWL_TOKEN || request.headers.get("x-crawl-token") !== env.CRAWL_TOKEN) {
			return json({ error: "unauthorized" }, 401);
		}
		let body;
		try {
			body = await request.json();
		} catch {
			return json({ error: "invalid json" }, 400);
		}
		const target = String(body && body.url ? body.url : "").trim();
		let t;
		try {
			t = new URL(target);
		} catch {
			return json({ error: "invalid url" }, 400);
		}
		if (t.protocol !== "http:" && t.protocol !== "https:") {
			return json({ error: "unsupported protocol" }, 400);
		}
		if (isPrivateHost(t.hostname)) {
			return json({ error: "private host blocked" }, 400);
		}
		try {
			const res = await fetch(t.toString(), {
				headers: {
					"user-agent": UA,
					accept: "text/html,application/xhtml+xml,application/xml,application/json;q=0.9,*/*;q=0.8",
					"accept-language": "id-ID,id;q=0.9,en;q=0.8"
				},
				redirect: "follow",
				signal: AbortSignal.timeout(15000)
			});
			const text = (await res.text()).slice(0, MAX_BODY);
			const headers = {};
			for (const k of ["content-type", "x-wp-total", "retry-after", "cf-mitigated", "server", "location"]) {
				const v = res.headers.get(k);
				if (v) headers[k] = v;
			}
			return json({ status: res.status, headers, body: text, final_url: res.url });
		} catch (e) {
			return json({ error: String((e && e.message) || e) }, 502);
		}
	}
};
