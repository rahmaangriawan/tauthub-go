// Cache in-memory GET daftar (stale-while-revalidate ringan): saat halaman
// dibuka ulang, data lama tampil seketika lalu di-refresh di latar — jadi
// pindah-balik halaman tidak menampilkan layar kosong. Cache dibersihkan
// otomatis setiap mutasi sukses (lihat $lib/auth.ts), sehingga daftar tidak
// pernah menampilkan data basi setelah aksi tulis.
type Entry = { value: unknown; at: number };

const store = new Map<string, Entry>();
const DEFAULT_MAX_AGE_MS = 10 * 60_000;

export function swrPeek<T>(key: string, maxAgeMs: number = DEFAULT_MAX_AGE_MS): T | null {
	if (typeof window === 'undefined') return null;
	const hit = store.get(key);
	if (!hit) return null;
	if (Date.now() - hit.at > maxAgeMs) return null;
	return hit.value as T;
}

export function swrPut<T>(key: string, value: T): void {
	if (typeof window === 'undefined') return;
	store.set(key, { value, at: Date.now() });
}

export function swrClear(): void {
	store.clear();
}
