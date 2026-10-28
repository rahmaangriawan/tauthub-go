/**
 * Schema.org JSON-LD untuk situs katalog publik.
 *
 * Semua halaman memakai satu `@graph` dengan `@id` stabil, supaya Google
 * membaca satu organisasi + satu situs yang sama di seluruh halaman, bukan
 * entitas berbeda tiap URL. Tidak ada field yang dikarang: hanya nilai yang
 * memang ada di record media atau di katalog.
 */

export const SITE_NAME = 'Tauthub Data';
const ORG_ID = (origin: string) => `${origin}/#organization`;
const SITE_ID = (origin: string) => `${origin}/#website`;

type Json = Record<string, unknown>;

export function organizationNode(origin: string): Json {
	return {
		'@type': 'Organization',
		'@id': ORG_ID(origin),
		name: 'Tauthub',
		url: origin,
		logo: `${origin}/tauthub-icon.png`,
		description: 'Platform distribusi konten yang menghubungkan klien dengan media partner.'
	};
}

export function websiteNode(origin: string): Json {
	return {
		'@type': 'WebSite',
		'@id': SITE_ID(origin),
		url: `${origin}/`,
		name: SITE_NAME,
		inLanguage: 'id-ID',
		publisher: { '@id': ORG_ID(origin) },
		potentialAction: {
			'@type': 'SearchAction',
			target: {
				'@type': 'EntryPoint',
				urlTemplate: `${origin}/?q={search_term_string}`
			},
			'query-input': 'required name=search_term_string'
		}
	};
}

export function webPageNode(
	origin: string,
	path: string,
	opts: { name: string; description: string; type?: string }
): Json {
	return {
		'@type': opts.type ?? 'WebPage',
		'@id': `${origin}${path}#webpage`,
		url: `${origin}${path}`,
		name: opts.name,
		description: opts.description,
		inLanguage: 'id-ID',
		isPartOf: { '@id': SITE_ID(origin) },
		publisher: { '@id': ORG_ID(origin) }
	};
}

export type Crumb = { name: string; path: string };

export function breadcrumbNode(origin: string, crumbs: Crumb[]): Json {
	return {
		'@type': 'BreadcrumbList',
		'@id': `${origin}${crumbs[crumbs.length - 1]?.path ?? '/'}#breadcrumb`,
		itemListElement: crumbs.map((c, i) => ({
			'@type': 'ListItem',
			position: i + 1,
			name: c.name,
			item: `${origin}${c.path}`
		}))
	};
}

export type ListEntry = { name: string; path: string };

/** ItemList untuk halaman daftar. `position` melanjutkan offset halaman. */
export function itemListNode(origin: string, path: string, name: string, entries: ListEntry[], offset = 0): Json {
	return {
		'@type': 'ItemList',
		'@id': `${origin}${path}#list`,
		name,
		numberOfItems: entries.length,
		itemListOrder: 'https://schema.org/ItemListOrderAscending',
		itemListElement: entries.map((e, i) => ({
			'@type': 'ListItem',
			position: offset + i + 1,
			url: `${origin}${e.path}`,
			name: e.name
		}))
	};
}

export type MediaSchemaInput = {
	slug: string;
	name: string;
	description: string | null;
	domain: string;
	websiteUrl: string | null;
	price: { amount: number; currency: string; type: 'nett' | 'nego' };
	categories: string[];
	da: number | null;
	rating: { avg: number; count: number } | null;
	publishedCount: number;
	updatedAt: string | null;
	verifiedPublisher: boolean;
};

/**
 * Entitas media. `Product` dipilih (bukan `Service`) karena Google menampilkan
 * harga dan ketersediaan dari `Offer` pada Product; `additionalType` menandai
 * bahwa ini layanan publikasi artikel, bukan barang fisik.
 */
export function mediaNode(origin: string, input: MediaSchemaInput): Json {
	const url = `${origin}/media/${input.slug}`;
	const node: Json = {
		'@type': 'Product',
		'@id': `${url}#product`,
		name: input.name,
		url,
		description:
			input.description ??
			`Layanan publikasi artikel di ${input.name}${input.domain ? ` (${input.domain})` : ''} lewat Tauthub.`,
		additionalType: 'https://schema.org/Service',
		category: input.categories[0] ?? undefined,
		brand: { '@id': ORG_ID(origin) },
		offers: {
			'@type': 'Offer',
			'@id': `${url}#offer`,
			url,
			price: input.price.amount,
			priceCurrency: input.price.currency || 'IDR',
			availability: 'https://schema.org/InStock',
			seller: { '@id': ORG_ID(origin) }
		}
	};

	if (input.websiteUrl) node.subjectOf = { '@type': 'WebSite', url: input.websiteUrl, name: input.domain };
	if (input.updatedAt) node.dateModified = input.updatedAt;

	// aggregateRating hanya boleh dikirim bila ada penilaian nyata.
	if (input.rating && input.rating.count > 0 && input.rating.avg > 0) {
		node.aggregateRating = {
			'@type': 'AggregateRating',
			ratingValue: Number(input.rating.avg.toFixed(1)),
			ratingCount: input.rating.count,
			bestRating: 5,
			worstRating: 1
		};
	}

	return node;
}

/** Bungkus node jadi satu skrip JSON-LD siap render. */
export function jsonLdGraph(nodes: Array<Json | null | undefined>): Json {
	return {
		'@context': 'https://schema.org',
		'@graph': nodes.filter((n): n is Json => Boolean(n))
	};
}
