import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { canonicalOrigin } from '$lib/server/data-site';
import { canonicalUrl } from '$lib/server/canonical';
import { catalogParams, fetchCategories, fetchMediaList } from '$lib/server/public-catalog';
import {
	breadcrumbNode,
	itemListNode,
	jsonLdGraph,
	organizationNode,
	webPageNode,
	websiteNode
} from '$lib/public/schema';

/** Halaman kategori: landing SEO per kategori media. */
export const load: PageServerLoad = async ({ params, url, fetch, setHeaders }) => {
	const slug = decodeURIComponent(params.slug ?? '')
		.trim()
		.toLowerCase();
	const categories = await fetchCategories(fetch);
	const category = categories.find((c) => c.slug === slug);
	if (!category) throw error(404, 'Kategori tidak ditemukan.');

	const apiParams = catalogParams(url);
	apiParams.set('kategori', slug);
	const list = await fetchMediaList(fetch, apiParams);

	setHeaders({ 'cache-control': 'public, max-age=60, stale-while-revalidate=300' });

	const origin = canonicalOrigin(url.hostname);
	const pageNo = list.page;
	const canonical = canonicalUrl(origin, `/kategori/${encodeURIComponent(slug)}`, url.searchParams);

	// `q` ikut dihitung sebagai filter: halaman hasil pencarian tidak boleh
	// diindeks (isinya bergantung query), dan tautan pagination harus membawa
	// kata kunci supaya halaman 2 tidak diam-diam kehilangan pencarian.
	const facetKeys = ['q', 'tier', 'da', 'harga', 'urut'];
	const hasFacet = facetKeys.some((k) => (url.searchParams.get(k) ?? '') !== '');
	const query: Record<string, string> = {};
	for (const k of facetKeys) {
		const v = (url.searchParams.get(k) ?? '').trim();
		if (v) query[k] = v;
	}

	const description = `${category.count} media kategori ${category.name} di Tauthub. Tiap media memuat angka Domain Authority, estimasi trafik, harga per artikel, dan aturan backlink.`;

	const path = `/kategori/${slug}`;
	const title =
		pageNo > 1
			? `Media ${category.name}, halaman ${pageNo} | Tauthub Data`
			: `Media ${category.name}: ${category.count} Media & Harga Publikasi | Tauthub Data`;

	const jsonLd = jsonLdGraph([
		organizationNode(origin),
		websiteNode(origin),
		webPageNode(origin, path, { name: title, description, type: 'CollectionPage' }),
		breadcrumbNode(origin, [
			{ name: 'Katalog media', path: '/' },
			{ name: category.name, path }
		]),
		hasFacet
			? null
			: itemListNode(
					origin,
					path,
					`Media kategori ${category.name}`,
					list.items.map((m) => ({ name: m.name, path: m.url })),
					(list.page - 1) * list.limit
				)
	]);

	return {
		category,
		categories,
		list,
		query,
		filters: {
			q: query.q ?? '',
			kategori: slug,
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
