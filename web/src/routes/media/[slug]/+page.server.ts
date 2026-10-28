import { error } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import type { PublicMedia } from '$lib/public/types';
import { categorySlug } from '$lib/public/format';
import { canonicalOrigin } from '$lib/server/data-site';
import { fetchMediaDetail, fetchMediaList } from '$lib/server/public-catalog';
import {
	breadcrumbNode,
	jsonLdGraph,
	mediaNode,
	organizationNode,
	webPageNode,
	websiteNode
} from '$lib/public/schema';

/** Detail satu media. 404 bila tidak ada / tidak aktif / disembunyikan admin. */
export const load: PageServerLoad = async ({ params, url, fetch, setHeaders }) => {
	const media = await fetchMediaDetail(fetch, params.slug);
	if (!media) throw error(404, 'Media tidak ditemukan.');

	setHeaders({ 'cache-control': 'public, max-age=300, stale-while-revalidate=600' });

	// Media lain di kategori yang sama: tautan internal untuk crawler.
	let related: PublicMedia[] = [];
	if (media.categories.length) {
		const relParams = new URLSearchParams({ limit: '8', kategori: categorySlug(media.categories[0]) });
		const rel = await fetchMediaList(fetch, relParams);
		related = rel.items.filter((m) => m.slug !== media.slug).slice(0, 3);
	}

	const origin = canonicalOrigin(url.hostname);
	// Halaman detail tidak punya variasi isi berbasis query: semua parameter
	// (utm, ref, page, dsb.) dibuang supaya canonical selalu URL bersih.
	const canonical = `${origin}${media.url}`;

	const da = media.seo.da;
	const priceLabel = media.price.type === 'nego' ? 'harga nego' : 'harga publikasi';
	const description = (
		media.description ??
		`Profil media ${media.name}${media.domain ? ` (${media.domain})` : ''}: Domain Authority, estimasi trafik, ${priceLabel}, dan kebijakan tautan.`
	).slice(0, 300);

	const title = `${media.name}: Profil Media${da != null ? `, DA ${da}` : ''}, Harga Publikasi | Tauthub Data`;

	const crumbs = [
		{ name: 'Katalog media', path: '/' },
		...(media.categories.length
			? [{ name: media.categories[0], path: `/kategori/${categorySlug(media.categories[0])}` }]
			: []),
		{ name: media.name, path: media.url }
	];

	const jsonLd = jsonLdGraph([
		organizationNode(origin),
		websiteNode(origin),
		webPageNode(origin, media.url, { name: title, description, type: 'ItemPage' }),
		breadcrumbNode(origin, crumbs),
		mediaNode(origin, {
			slug: media.slug,
			name: media.name,
			description: media.description,
			domain: media.domain,
			websiteUrl: media.website_url,
			price: media.price,
			categories: media.categories,
			da,
			rating:
				media.trust.rating_avg && media.trust.rating_count > 0
					? { avg: media.trust.rating_avg, count: media.trust.rating_count }
					: null,
			publishedCount: media.trust.published_count,
			updatedAt: media.updated_at,
			verifiedPublisher: media.trust.dewan_pers_verified || media.trust.domain_verified
		})
	]);

	return {
		media,
		related,
		seo: {
			title,
			description,
			canonical,
			ogType: 'article',
			jsonLd
		}
	};
};
