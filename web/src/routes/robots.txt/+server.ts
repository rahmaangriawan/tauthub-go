import type { RequestHandler } from './$types';
import { catalogEnabled, dataOrigin, isDataHost } from '$lib/server/data-site';

/**
 * robots.txt sadar-host. Diganti dari berkas statis karena host publik dan
 * host aplikasi butuh aturan berbeda; berkas statis akan menutupi route ini.
 */
const APP_ROBOTS = `# Konten publik boleh dirayapi; area privat (portal klien/mitra/admin dan
# halaman berita acara ber-token) tidak untuk mesin pencari.
User-agent: *
Disallow: /admin
Disallow: /klien
Disallow: /mitra
Disallow: /berita-acara/
`;

export const GET: RequestHandler = async ({ url }) => {
	let body: string;

	if (!catalogEnabled()) {
		// Kill switch: sembunyikan seluruh situs dari crawler.
		body = 'User-agent: *\nDisallow: /\n';
	} else if (isDataHost(url.hostname)) {
		body = `# Katalog media publik Tauthub.
User-agent: *
Allow: /
Disallow: /login
Disallow: /admin
Disallow: /klien
Disallow: /mitra

Sitemap: ${dataOrigin()}/sitemap.xml
`;
	} else {
		body = APP_ROBOTS;
	}

	return new Response(body, {
		headers: {
			'content-type': 'text/plain; charset=utf-8',
			// no-store: Cloudflare meng-cache berkas .txt berdasarkan ekstensi dan
			// mengabaikan max-age origin. Tanpa ini, perubahan aturan robots bisa
			// tertahan berjam-jam di edge — dan sempat menyajikan robots host
			// aplikasi untuk host katalog.
			'cache-control': 'no-store'
		}
	});
};
