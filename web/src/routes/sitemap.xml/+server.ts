import type { RequestHandler } from './$types';
import { canonicalOrigin, catalogEnabled, isAppHost } from '$lib/server/data-site';
import { lastmodOf, loadSitemapData, mediaChunkCount, xmlResponse } from '$lib/server/sitemap';

/** Sitemap index: halaman statis + chunk media. */
export const GET: RequestHandler = async ({ url, fetch }) => {
	// Sitemap hanya bermakna di host katalog; host dashboard tidak punya URL
	// publik untuk didaftarkan ke Google.
	if (isAppHost(url.hostname)) return new Response('Sitemap tidak tersedia.', { status: 404 });
	if (!catalogEnabled()) return new Response('Katalog publik tidak aktif.', { status: 503 });

	const origin = canonicalOrigin(url.hostname);
	const data = await loadSitemapData(fetch);
	const chunks = mediaChunkCount(data.entries.length);
	const newest = lastmodOf(data.entries[0]?.updated_at);

	const lines = [
		'<?xml version="1.0" encoding="UTF-8"?>',
		'<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
		`  <sitemap><loc>${origin}/sitemap-pages.xml</loc>${newest ? `<lastmod>${newest}</lastmod>` : ''}</sitemap>`
	];
	for (let i = 1; i <= chunks; i++) {
		lines.push(
			`  <sitemap><loc>${origin}/sitemap-media-${i}.xml</loc>${newest ? `<lastmod>${newest}</lastmod>` : ''}</sitemap>`
		);
	}
	lines.push('</sitemapindex>');

	return xmlResponse(lines.join('\n'));
};
