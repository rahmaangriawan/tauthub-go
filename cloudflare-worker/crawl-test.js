// Tauthub — Cloudflare Worker TEST crawler.
//
// Tujuan: membuktikan (atau menolak) apakah request keluar dari jaringan
// Cloudflare lolos dari WAF situs mitra yang memblokir IP datacenter
// (bekasikinian.com, kpopsquad.com, dst. semuanya 403 dari VPS Tencent).
//
// Pemakaian:
//   GET /                                  → tes daftar domain bawaan
//   GET /?domains=a.com,b.com              → tes domain tertentu (homepage + sitemap)
//   GET /?url=https://a.com/halaman        → tes satu URL apa adanya
//
// Worker ini tidak menyimpan apa pun dan tidak butuh auth (read-only tes).

const UA =
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36";

const HEADERS = {
	"user-agent": UA,
	accept: "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	"accept-language": "id-ID,id;q=0.9,en;q=0.8",
};

const DEFAULT_DOMAINS = [
	"bekasikinian.com",
	"kpopsquad.com",
	"patinews.com",
	"edisi.co.id",
];

const PATHS = ["/", "/sitemap.xml"];

async function testUrl(target) {
	const entry = {
		url: target,
		status: null,
		ok: false,
		server: null,
		cf_mitigated: null,
		final_url: null,
		error: null,
		ms: 0,
	};
	const started = Date.now();
	try {
		const res = await fetch(target, {
			headers: HEADERS,
			redirect: "follow",
			signal: AbortSignal.timeout(15000),
		});
		entry.status = res.status;
		entry.ok = res.status >= 200 && res.status < 300;
		entry.server = res.headers.get("server");
		entry.cf_mitigated = res.headers.get("cf-mitigated");
		entry.final_url = res.url;
	} catch (e) {
		entry.error = String((e && e.message) || e);
	}
	entry.ms = Date.now() - started;
	return entry;
}

export default {
	async fetch(request) {
		const url = new URL(request.url);
		const singleUrl = (url.searchParams.get("url") || "").trim();
		const domainsParam = (url.searchParams.get("domains") || "").trim();

		let targets = [];
		if (singleUrl) {
			targets = [singleUrl];
		} else {
			const domains = (
				domainsParam
					? domainsParam.split(",")
					: DEFAULT_DOMAINS
			)
				.map((s) => s.trim().replace(/^https?:\/\//, "").replace(/\/$/, ""))
				.filter(Boolean);
			for (const d of domains) {
				for (const p of PATHS) targets.push(`https://${d}${p}`);
			}
		}

		const results = [];
		for (const target of targets) {
			results.push(await testUrl(target));
		}

		// Info egress: dari negara/colo/ASN mana request keluar. Ini kunci
		// diagnosa — WAF pemilik situs menilai berdasarkan IP/ASN sumber.
		const egress = {
			country: (request.cf && request.cf.country) || null,
			colo: (request.cf && request.cf.colo) || null,
			asn: (request.cf && request.cf.asn) || null,
			as_organization: (request.cf && request.cf.asOrganization) || null,
		};

		const body = JSON.stringify(
			{ egress, tested: results.length, results },
			null,
			2
		);
		return new Response(body, {
			headers: {
				"content-type": "application/json; charset=utf-8",
				"cache-control": "no-store",
			},
		});
	},
};
