import type { RequestHandler } from './$types';
import { appOrigin } from '$lib/server/data-site';
import { allPages } from '$lib/server/docs';

/** Sitemap halaman dokumentasi (host aplikasi tidak punya sitemap lain). */
export const GET: RequestHandler = () => {
	const origin = appOrigin();
	const urls = ['/dokumentasi', ...allPages().map((p) => p.url)];
	const body = [
		'<?xml version="1.0" encoding="UTF-8"?>',
		'<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
		...urls.map((u) => `  <url><loc>${origin}${u}</loc></url>`),
		'</urlset>'
	].join('\n');
	return new Response(body, {
		headers: {
			'content-type': 'application/xml; charset=utf-8',
			'cache-control': 'public, max-age=3600'
		}
	});
};
