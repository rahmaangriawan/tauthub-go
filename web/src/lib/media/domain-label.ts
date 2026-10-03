/**
 * Label domain untuk identitas media di katalog & The Matchmaker.
 *
 * Nama tampil memakai domain (bukan nama brand website) dengan huruf pertama
 * dibesarkan: "arusberita.com" → "Arusberita.com". Dipakai bersama supaya
 * kedua halaman menampilkan identitas yang sama.
 */
export function mediaDomainLabel(websiteUrl: string | null | undefined, fallback = ''): string {
	const base =
		(websiteUrl ?? '')
			.replace(/^https?:\/\/(www\.)?/i, '')
			.replace(/\/+$/, '')
			.trim() || fallback.trim();
	if (!base) return '—';
	return base.charAt(0).toUpperCase() + base.slice(1);
}
