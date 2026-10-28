import type { RequestHandler } from './$types';
import { canonicalOrigin, catalogEnabled, isAppHost } from '$lib/server/data-site';
import {
	SITEMAP_CHUNK,
	lastmodOf,
	loadSitemapData,
	xmlEscape,
	xmlResponse
} from '$lib/server/sitemap';

/**
 * /sitemap-pages.xml  → beranda + semua halaman kategori
 * /sitemap-media-N.xml → potongan daftar media (5.000 URL per berkas)
 */
export const GET: RequestHandler = async ({ params, url, fetch }) => {
	if (isAppHost(url.hostname)) return new Response('Sitemap tidak tersedia.', { status: 404 });
	if (!catalogEnabled()) return new Response('Katalog publik tidak aktif.', { status: 503 });

	const chunk = params.chunk ?? '';
	const origin = canonicalOrigin(url.hostname);
	const data = await loadSitemapData(fetch);

	const urls: string[] = [];
	const header = [
		'<?xml version="1.0" encoding="UTF-8"?>',
		'<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">'
	];

	const push = (loc: string, lastmod: string | null, changefreq: string, priority: string) => {
		urls.push(
			`  <url><loc>${xmlEscape(loc)}</loc>${lastmod ? `<lastmod>${lastmod}</lastmod>` : ''}` +
				`<changefreq>${changefreq}</changefreq><priority>${priority}</priority></url>`
		);
	};

	if (chunk === 'pages') {
		const newest = lastmodOf(data.entries[0]?.updated_at);
		push(`${origin}/`, newest, 'daily', '1.0');
		for (const cat of data.categories) {
			push(`${origin}/kategori/${encodeURIComponent(cat.slug)}`, newest, 'weekly', '0.8');
		}
	} else {
		const match = /^media-(\d+)$/.exec(chunk);
		if (!match) return new Response('Sitemap tidak ditemukan.', { status: 404 });
		const page = Number(match[1]);
		if (page < 1) return new Response('Sitemap tidak ditemukan.', { status: 404 });
		const slice = data.entries.slice((page - 1) * SITEMAP_CHUNK, page * SITEMAP_CHUNK);
		if (slice.length === 0) return new Response('Sitemap tidak ditemukan.', { status: 404 });
		for (const entry of slice) {
			push(`${origin}/media/${encodeURIComponent(entry.slug)}`, lastmodOf(entry.updated_at), 'weekly', '0.6');
		}
	}

	return xmlResponse([...header, ...urls, '</urlset>'].join('\n'));
};
