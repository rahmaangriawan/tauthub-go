import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { canonicalOrigin, isAppHost } from '$lib/server/data-site';
import { canonicalUrl } from '$lib/server/canonical';
import {
	breadcrumbNode,
	itemListNode,
	jsonLdGraph,
	organizationNode,
	webPageNode,
	websiteNode
} from '$lib/public/schema';
import {
	catalogParams,
	fetchCategories,
	fetchHeroCandidates,
	fetchMediaList,
	fetchStats,
	pickHeroMedia
} from '$lib/server/public-catalog';

/**
 * Beranda.
 * - Host dashboard (app.tauthub.com): perilaku lama, langsung ke layar MASUK.
 * - Host publik (data.tauthub.com): landing + katalog media, dirender server
 *   supaya isinya terbaca crawler.
 * - Host lain (localhost/staging): menampilkan katalog supaya bisa dipreview.
 */
export const load: PageServerLoad = async ({ url, fetch, setHeaders }) => {
	if (isAppHost(url.hostname)) throw redirect(307, '/login');

	const apiParams = catalogParams(url);
	const [list, categories, stats, heroPool] = await Promise.all([
		fetchMediaList(fetch, apiParams),
		fetchCategories(fetch),
		fetchStats(fetch),
		fetchHeroCandidates(fetch)
	]);

	// Preview hero: 3 media acak dari katalog nyata (bukan hasil filter
	// pengguna), domain tidak boleh berulang.
	const heroMedia = pickHeroMedia(heroPool, 3);

	// Chip "pencarian populer": semuanya tautan filter yang benar-benar
	// dijalankan katalog (kategori teratas + ambang DA), bukan hiasan.
	const popular = [
		...categories.slice(0, 3).map((cat) => ({
			label: cat.name,
			href: `/?kategori=${encodeURIComponent(cat.slug)}`
		})),
		{ label: 'DA 50+', href: '/?da=50' }
	];

	// max-age pendek: angka "N media aktif" dan tabel ikut berubah saat katalog
	// berubah. stale-while-revalidate tetap ada supaya pengunjung tidak menunggu
	// saat trafik ramai, tapi paling lama 60 detik sebelum revalidasi.
	setHeaders({ 'cache-control': 'public, max-age=30, stale-while-revalidate=120' });

	const origin = canonicalOrigin(url.hostname);
	const facetKeys = ['q', 'kategori', 'tier', 'da', 'harga', 'urut'];
	const hasFacet = facetKeys.some((k) => (url.searchParams.get(k) ?? '') !== '');
	const pageNo = Number(apiParams.get('page') ?? 1) || 1;

	// URL yang benar-benar diminta, tanpa parameter pelacak. Halaman filter
	// tidak diindeks (robots noindex) tetapi canonical-nya menunjuk dirinya
	// sendiri, bukan beranda — dua sinyal itu tidak boleh bertentangan.
	const canonical = canonicalUrl(origin, url.pathname, url.searchParams);

	// Parameter untuk tautan pagination (tanpa `page` — ditambahkan per tautan).
	const query: Record<string, string> = {};
	for (const k of facetKeys) {
		const v = (url.searchParams.get(k) ?? '').trim();
		if (v) query[k] = v;
	}

	const description =
		stats.total > 0
			? `Direktori ${stats.total} media online Indonesia. Tiap media memuat angka Domain Authority, estimasi trafik bulanan, harga per artikel, dan aturan backlink.`
			: 'Direktori media online Indonesia dengan angka Domain Authority, estimasi trafik, harga per artikel, dan aturan backlink.';

	const title =
		pageNo > 1
			? `Katalog Media Online Indonesia, halaman ${pageNo} | Tauthub Data`
			: 'Katalog Media Online Indonesia: DA, Trafik, dan Harga Publikasi | Tauthub Data';

	// JSON-LD: satu graph dengan @id stabil. ItemList hanya untuk halaman yang
	// boleh diindeks (bukan hasil filter), supaya Google tidak menerima ribuan
	// daftar berbeda dari URL facet.
	const jsonLd = jsonLdGraph([
		organizationNode(origin),
		websiteNode(origin),
		webPageNode(origin, '/', { name: title, description }),
		hasFacet
			? null
			: itemListNode(
					origin,
					'/',
					'Katalog media Tauthub',
					list.items.map((m) => ({ name: m.name, path: m.url })),
					(list.page - 1) * list.limit
				)
	]);

	return {
		list,
		categories,
		stats,
		heroMedia,
		popular,
		query,
		filters: {
			q: query.q ?? '',
			kategori: query.kategori ?? '',
			tier: query.tier ?? '',
			da: query.da ?? '',
			harga: query.harga ?? '',
			urut: query.urut ?? 'nama'
		},
		seo: {
			title,
			description,
			canonical,
			robots: hasFacet ? 'noindex,follow' : 'index,follow',
			jsonLd
		}
	};
};
