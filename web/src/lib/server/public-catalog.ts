/**
 * Klien server untuk katalog publik (API Go /v1/public/*).
 *
 * Dipakai +page.server.ts supaya HTML terkirim sudah terisi (penting untuk
 * crawler). `API_URL` sama dengan gate.ts: default http://localhost:8080 —
 * target proxy Vite di dev dan nginx di produksi.
 */
import { env } from '$env/dynamic/private';
import { catalogEnabled } from './data-site';
import { randomShuffleSeed, seededShuffle } from '$lib/media/shuffle';
import type { PublicCategory, PublicList, PublicMedia, PublicStats, SimResult } from '$lib/public/types';

const API_URL = env.API_URL ?? 'http://localhost:8080';

type Fetch = typeof globalThis.fetch;

async function apiGet<T>(fetch: Fetch, path: string): Promise<T | null> {
	if (!catalogEnabled()) return null;
	try {
		const res = await fetch(`${API_URL}${path}`, {
			headers: { accept: 'application/json' },
			signal: AbortSignal.timeout(8_000)
		});
		if (!res.ok) return null;
		return (await res.json()) as T;
	} catch {
		// API mati/lambat: halaman publik tetap tampil dengan data kosong,
		// bukan 500. Kesalahan dicatat oleh API sendiri.
		return null;
	}
}

export async function fetchMediaList(fetch: Fetch, params: URLSearchParams): Promise<PublicList> {
	const data = await apiGet<PublicList>(fetch, `/v1/public/media?${params.toString()}`);
	return data ?? { items: [], total: 0, page: 1, limit: 24, pages: 0 };
}

export async function fetchMediaDetail(fetch: Fetch, slug: string): Promise<PublicMedia | null> {
	return apiGet<PublicMedia>(fetch, `/v1/public/media/${encodeURIComponent(slug)}`);
}

// Cache in-process singkat untuk data yang berubah jarang. Setiap halaman
// katalog memuat kategori + statistik; tanpa cache, tiap kunjungan menambah
// dua query ke API.
//
// Statistik sengaja lebih pendek dari kategori: angka "N media aktif" di hero
// harus ikut berubah begitu katalog bertambah atau berkurang, jadi menahannya
// lama berarti angka di halaman bisa basi.
const memo = new Map<string, { at: number; value: unknown }>();

async function cached<T>(key: string, ttlMs: number, load: () => Promise<T>): Promise<T> {
	const hit = memo.get(key) as { at: number; value: T } | undefined;
	if (hit && Date.now() - hit.at < ttlMs) return hit.value;
	const value = await load();
	memo.set(key, { at: Date.now(), value });
	return value;
}

export async function fetchCategories(fetch: Fetch): Promise<PublicCategory[]> {
	return cached('categories', 30_000, async () => {
		const data = await apiGet<{ items: PublicCategory[] }>(fetch, '/v1/public/kategori');
		return data?.items ?? [];
	});
}

export async function fetchStats(fetch: Fetch): Promise<PublicStats> {
	return cached('stats', 10_000, async () => {
		const data = await apiGet<PublicStats>(fetch, '/v1/public/statistik');
		return (
			data ?? {
				total: 0,
				categories: 0,
				tier1: 0,
				tier2: 0,
				tier3: 0,
				price_min: 0,
				avg_da: 0,
				updated_at: null
			}
		);
	});
}

export async function fetchSitemapSlugs(fetch: Fetch): Promise<Array<{ slug: string; updated_at: string | null }>> {
	const data = await apiGet<{ items: Array<{ slug: string; updated_at: string | null }> }>(
		fetch,
		'/v1/public/sitemap'
	);
	return data?.items ?? [];
}

/**
 * Kandidat preview hero: satu request per 60 detik (bukan per kunjungan),
 * diurut DA tertinggi supaya contoh yang tampil representatif.
 */
export async function fetchHeroCandidates(fetch: Fetch): Promise<PublicMedia[]> {
	return cached('hero', 60_000, async () => {
		const data = await apiGet<PublicList>(fetch, '/v1/public/media?limit=60&urut=da');
		return data?.items ?? [];
	});
}

/**
 * Ambil `count` media acak dari kandidat, tanpa domain ganda. Yang diacak hanya
 * pemilihan record — atributnya tetap apa adanya dari API. Seed dibuat per
 * pemanggilan (per request), bukan per render komponen, jadi daftar tidak
 * berubah-ubah saat halaman di-hydrate.
 */
export function pickHeroMedia(pool: PublicMedia[], count = 3): PublicMedia[] {
	if (pool.length === 0) return [];
	const out: PublicMedia[] = [];
	const seen = new Set<string>();
	for (const item of seededShuffle(pool, randomShuffleSeed())) {
		const key = (item.domain || item.slug).toLowerCase();
		if (seen.has(key)) continue;
		seen.add(key);
		out.push(item);
		if (out.length === count) break;
	}
	return out;
}

/** Parameter filter katalog yang dipakai bersama oleh halaman daftar. */
export function catalogParams(url: URL): URLSearchParams {
	const params = new URLSearchParams();
	const q = (url.searchParams.get('q') ?? '').trim().slice(0, 120);
	if (q) params.set('q', q);
	const kategori = (url.searchParams.get('kategori') ?? '').trim().slice(0, 60);
	if (kategori) params.set('kategori', kategori);
	const tier = (url.searchParams.get('tier') ?? '').trim();
	if (tier === 'tier1' || tier === 'tier2' || tier === 'tier3') params.set('tier', tier);
	const da = Number(url.searchParams.get('da') ?? 0);
	if (da > 0 && da <= 100) params.set('da_awal', String(Math.trunc(da)));
	const harga = Number(url.searchParams.get('harga') ?? 0);
	if (harga > 0) params.set('harga_maks', String(Math.trunc(harga)));
	const urut = (url.searchParams.get('urut') ?? '').trim();
	if (['nama', 'da', 'trafik', 'harga_asc', 'harga_desc', 'baru'].includes(urut)) params.set('urut', urut);
	const page = Math.max(1, Number(url.searchParams.get('page') ?? 1) || 1);
	params.set('page', String(page));
	params.set('limit', '24');
	return params;
}

/** Apakah URL memuat filter aktif (dipakai untuk canonical/noindex). */
export function hasActiveFilters(url: URL): boolean {
	return ['q', 'kategori', 'tier', 'da', 'harga', 'urut', 'page'].some((k) => {
		const v = url.searchParams.get(k);
		return v !== null && v !== '' && !(k === 'page' && v === '1');
	});
}

/** Simulasi paket: tiga paket contoh dari media publik untuk satu anggaran. */
export async function fetchSimulation(fetch: Fetch, params: URLSearchParams): Promise<SimResult | null> {
	return apiGet<SimResult>(fetch, `/v1/public/simulasi?${params.toString()}`);
}
