import type { PageServerLoad } from './$types';
import { canonicalOrigin } from '$lib/server/data-site';
import { canonicalUrl } from '$lib/server/canonical';
import { fetchCategories, fetchSimulation } from '$lib/server/public-catalog';
import { jsonLdGraph, organizationNode, websiteNode, webPageNode } from '$lib/public/schema';

const BUDGET_MIN = 100_000;
const BUDGET_MAX = 10_000_000_000;

/**
 * Simulasi paket publikasi. Tanpa anggaran: hanya formulir. Dengan anggaran:
 * tiga paket contoh (hemat, seimbang, premium) dari media publik yang nyata.
 * Halaman hasil tidak diindeks; hanya halaman formulir kosong yang boleh.
 */
export const load: PageServerLoad = async ({ url, fetch, setHeaders }) => {
	const raw = (url.searchParams.get('anggaran') ?? '').replace(/[^0-9]/g, '');
	const budget = raw ? Number(raw) : 0;
	const kategori = (url.searchParams.get('kategori') ?? '').trim().toLowerCase();
	const tierRaw = (url.searchParams.get('tier') ?? '').trim();
	const tier = tierRaw === 'tier1' || tierRaw === 'tier2' || tierRaw === 'tier3' ? tierRaw : '';
	const da = Math.min(100, Math.max(0, Math.trunc(Number(url.searchParams.get('da') ?? 0)) || 0));
	const jumlah = Math.min(30, Math.max(0, Math.trunc(Number(url.searchParams.get('jumlah') ?? 0)) || 0));

	const submitted = url.searchParams.has('anggaran');
	let budgetError = '';
	if (submitted && (budget < BUDGET_MIN || budget > BUDGET_MAX)) {
		budgetError = 'Isi anggaran antara Rp100.000 dan Rp10.000.000.000.';
	}

	const categories = await fetchCategories(fetch);

	let result = null;
	let failed = false;
	if (submitted && !budgetError) {
		const params = new URLSearchParams({ anggaran: String(budget) });
		if (kategori) params.set('kategori', kategori);
		if (tier) params.set('tier', tier);
		if (da > 0) params.set('da_awal', String(da));
		if (jumlah > 0) params.set('jumlah', String(jumlah));
		result = await fetchSimulation(fetch, params);
		failed = result === null;
	}

	setHeaders({ 'cache-control': 'public, max-age=30, stale-while-revalidate=60' });

	const origin = canonicalOrigin(url.hostname);
	const title = 'Simulasi Paket Publikasi Media Online | Tauthub Data';
	const description =
		'Masukkan anggaran, lihat paket media online yang bisa Anda dapat: berapa media, rata-rata Domain Authority, dan estimasi trafik. Harga sama dengan katalog.';

	return {
		categories,
		result,
		failed,
		budgetError,
		form: {
			anggaran: budget > 0 ? String(budget) : '',
			kategori,
			tier,
			da: da > 0 ? String(da) : '',
			jumlah: jumlah > 0 ? String(jumlah) : ''
		},
		seo: {
			title,
			description,
			canonical: canonicalUrl(origin, url.pathname, new URLSearchParams()),
			robots: submitted ? 'noindex,follow' : 'index,follow',
			jsonLd: jsonLdGraph([
				organizationNode(origin),
				websiteNode(origin),
				webPageNode(origin, '/simulasi', { name: title, description })
			])
		}
	};
};
