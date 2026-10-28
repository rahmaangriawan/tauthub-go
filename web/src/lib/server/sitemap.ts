/**
 * Data & helper sitemap XML untuk katalog publik.
 *
 * Satu sitemap index → satu sitemap halaman (/ + kategori) dan beberapa
 * sitemap media (maks 5.000 URL per berkas, batas aman protokol sitemap).
 */
import { fetchCategories, fetchSitemapSlugs } from './public-catalog';

export const SITEMAP_CHUNK = 5_000;
const CACHE_TTL_MS = 10 * 60 * 1000;

export type SitemapEntry = { slug: string; updated_at: string | null };
export type SitemapCategory = { slug: string; name: string; count: number };

type SitemapData = { at: number; entries: SitemapEntry[]; categories: SitemapCategory[] };

let cache: SitemapData | null = null;

export async function loadSitemapData(fetch: typeof globalThis.fetch): Promise<SitemapData> {
	if (cache && Date.now() - cache.at < CACHE_TTL_MS) return cache;
	const [entries, categories] = await Promise.all([fetchSitemapSlugs(fetch), fetchCategories(fetch)]);
	cache = { at: Date.now(), entries, categories };
	return cache;
}

export function mediaChunkCount(entries: number): number {
	return Math.max(1, Math.ceil(entries / SITEMAP_CHUNK));
}

export function xmlEscape(value: string): string {
	return value.replace(/[<>&'"]/g, (c) =>
		c === '<' ? '&lt;' : c === '>' ? '&gt;' : c === '&' ? '&amp;' : c === "'" ? '&apos;' : '&quot;'
	);
}

/** Tanggal ISO → YYYY-MM-DD untuk <lastmod>. */
export function lastmodOf(iso: string | null | undefined): string | null {
	if (!iso) return null;
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return null;
	return d.toISOString().slice(0, 10);
}

export function xmlResponse(body: string, maxAge = 3_600): Response {
	return new Response(body, {
		headers: {
			'content-type': 'application/xml; charset=utf-8',
			'cache-control': `public, max-age=${maxAge}, stale-while-revalidate=86400`
		}
	});
}
