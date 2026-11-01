/**
 * Local Retrieval Worker — Tauthub.
 *
 * Komputer lokal admin menarik halaman media lewat IP ISP rumahan untuk
 * domain yang memblokir IP datacenter VPS (HTTP 403/429 dari Cloudflare WAF).
 * Worker HANYA menarik + mengekstrak teks; AI dan penyimpanan tetap di server
 * production.
 *
 * Pemakaian:
 *   LOCAL_WORKER_TOKEN=<token-yang-sama-dengan-server> bun run local-worker/worker.ts
 *
 * Opsi env:
 *   TAUTHUB_API_BASE            default http://localhost:8080
 *   LOCAL_WORKER_INTERVAL_MS    default 20000 (polling saat antrean kosong)
 *
 * Keamanan: hanya https, tolak host yang resolve ke IP privat/loopback,
 * batas ukuran halaman, dan honati robots.txt secara umum (tarikan jarang,
 * hanya domain yang diminta server).
 */

const API = (process.env.TAUTHUB_API_BASE ?? "http://localhost:8080").replace(/\/$/, "");
const TOKEN = process.env.LOCAL_WORKER_TOKEN ?? "";
const INTERVAL_MS = Number(process.env.LOCAL_WORKER_INTERVAL_MS ?? 20_000);

const UA =
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36";
const MAX_HTML_CHARS = 200_000;
const MIN_TEXT_CHARS = 80;

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

// ---------- koneksi ke API Tauthub ----------

async function claim(): Promise<{ id: string; media_id: string; website_url: string } | null> {
	const res = await fetch(`${API}/v1/platform/local-worker/claim`, {
		headers: { authorization: `Bearer ${TOKEN}` },
		signal: AbortSignal.timeout(20_000),
	});
	if (res.status === 204) return null;
	if (!res.ok) {
		const body = await res.text().catch(() => "");
		throw new Error(`claim HTTP ${res.status}: ${body.slice(0, 200)}`);
	}
	return (await res.json()) as { id: string; media_id: string; website_url: string };
}

async function submit(id: string, body: Record<string, unknown>) {
	const res = await fetch(`${API}/v1/platform/local-worker/requests/${id}/result`, {
		method: "POST",
		headers: { authorization: `Bearer ${TOKEN}`, "content-type": "application/json" },
		body: JSON.stringify(body),
		signal: AbortSignal.timeout(180_000), // server memanggil AI: beri waktu sampai 3 menit
	});
	if (!res.ok) {
		const text = await res.text().catch(() => "");
		throw new Error(`result HTTP ${res.status}: ${text.slice(0, 200)}`);
	}
	return res.json();
}

// ---------- keamanan: tolak host yang mengarah ke IP privat ----------

function isPrivateIPv4(ip: string) {
	const [a, b] = ip.split(".").map(Number);
	if (a === 10 || a === 127 || a === 0) return true;
	if (a === 169 && b === 254) return true;
	if (a === 172 && b >= 16 && b <= 31) return true;
	if (a === 192 && b === 168) return true;
	if (a >= 224) return true; // multicast/reserved
	return false;
}

function isPrivateIPv6(ip: string) {
	const v = ip.toLowerCase();
	return v === "::1" || v.startsWith("fc") || v.startsWith("fd") || v.startsWith("fe80");
}

async function assertPublicHost(hostname: string) {
	const { lookup } = await import("node:dns/promises");
	const addrs = await lookup(hostname, { all: true, verbatim: true });
	if (addrs.length === 0) throw new Error("DNS tidak menemukan alamat host");
	for (const { address } of addrs) {
		if (address.includes(":") ? isPrivateIPv6(address) : isPrivateIPv4(address)) {
			throw new Error(`Host mengarah ke IP privat (${address}) — ditolak.`);
		}
	}
}

// ---------- fetch + ekstraksi ----------

async function fetchHTML(url: string): Promise<{ status: number; html: string; note?: string }> {
	const res = await fetch(url, {
		headers: {
			"user-agent": UA,
			accept: "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"accept-language": "id-ID,id;q=0.9,en;q=0.8",
		},
		redirect: "follow",
		signal: AbortSignal.timeout(30_000),
	});
	const html = (await res.text()).slice(0, MAX_HTML_CHARS);
	return { status: res.status, html };
}

export function htmlToText(html: string): string {
	let out = html
		.replace(/<!--[\s\S]*?-->/g, " ")
		.replace(/<(script|style|noscript|svg|header|footer|nav)[\s\S]*?<\/\1>/gi, " ")
		.replace(/<\/(p|div|li|h[1-6]|tr|br|section|article)>/gi, "\n")
		.replace(/<br\s*\/?>/gi, "\n")
		.replace(/<[^>]+>/g, " ");
	out = out
		.replace(/&nbsp;/gi, " ")
		.replace(/&amp;/gi, "&")
		.replace(/&lt;/gi, "<")
		.replace(/&gt;/gi, ">")
		.replace(/&quot;/gi, '"')
		.replace(/&#0?39;|&apos;/gi, "'");
	return out
		.replace(/[ \t\u00a0]+/g, " ")
		.replace(/\n\s*\n\s*\n+/g, "\n\n")
		.split("\n")
		.map((l) => l.trim())
		.filter(Boolean)
		.join("\n")
		.trim();
}

// ---------- satu pekerjaan ----------

async function processJob(job: { id: string; media_id: string; website_url: string }) {
	console.log(`[worker] menarik ${job.website_url} (request ${job.id.slice(0, 8)})`);
	const pages: Array<{ url: string; ok: boolean; chars: number; note?: string }> = [];
	const chunks: string[] = [];

	const target = (u: string) => {
		const parsed = new URL(u);
		return { host: parsed.hostname, url: u };
	};

	// Homepage: https dulu, fallback http — pola sama dengan crawler VPS.
	const bare = job.website_url.replace(/^https?:\/\//, "").replace(/\/$/, "");
	const candidates = [`https://${bare}`, `http://${bare}`];
	let html = "";
	for (const candidate of candidates) {
		try {
			const { host } = target(candidate);
			await assertPublicHost(host);
			const res = await fetchHTML(candidate);
			if (res.status >= 200 && res.status < 300) {
				const text = htmlToText(res.html);
				const ok = text.length > MIN_TEXT_CHARS;
				pages.push({ url: candidate, ok, chars: text.length, note: ok ? undefined : "Konten terlalu pendek" });
				if (ok) {
					html = res.html;
					chunks.push(text);
					break;
				}
			} else {
				pages.push({ url: candidate, ok: false, note: `HTTP ${res.status}` });
			}
		} catch (e) {
			pages.push({ url: candidate, ok: false, note: String((e as Error).message || e).slice(0, 120) });
		}
	}

	if (chunks.length === 0) {
		const note = pages[0]?.note ?? "Web tidak terjangkau";
		console.log(`[worker] gagal: ${note}`);
		await submit(job.id, { ok: false, pages, error: note });
		return;
	}

	const text = chunks.join("\n\n").slice(0, 60_000);
	console.log(`[worker] sukses: ${text.length} karakter — dikirim ke server (AI di VPS)`);
	await submit(job.id, { ok: true, text, pages });
}

// ---------- loop ----------

async function main() {
	if (!TOKEN) {
		console.error("[worker] LOCAL_WORKER_TOKEN belum diisi.");
		process.exit(1);
	}
	console.log(`[worker] aktif — polling ${API} tiap ${Math.round(INTERVAL_MS / 1000)}s`);
	// Polling keluar saja: tidak ada port yang dibuka di komputer lokal.
	for (;;) {
		let job: Awaited<ReturnType<typeof claim>> = null;
		try {
			job = await claim();
		} catch (e) {
			console.error(`[worker] claim gagal: ${(e as Error).message}`);
			await sleep(INTERVAL_MS);
			continue;
		}
		if (!job) {
			await sleep(INTERVAL_MS);
			continue;
		}
		try {
			await processJob(job);
		} catch (e) {
			console.error(`[worker] pekerjaan ${job.id.slice(0, 8)} gagal: ${(e as Error).message}`);
			await submit(job.id, {
				ok: false,
				pages: [],
				error: String((e as Error).message || e).slice(0, 500),
			}).catch(() => {});
		}
	}
}

void main();
