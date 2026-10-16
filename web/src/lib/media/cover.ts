/**
 * Thumbnail default Tauthub untuk artikel yang tidak diberi cover klien.
 *
 * Dipakai dua tempat: opsi "pakai logo Tauthub" di editor artikel, dan
 * konfirmasi "lanjut tanpa cover" di The Writer. Satu konstanta supaya
 * keduanya tidak pernah berbeda.
 */
export const TAUTHUB_FALLBACK_COVER = '/tauthub-logo.webp';

/**
 * Cover pengganti saat klien memilih lanjut tanpa cover.
 *
 * Memakai aset statis aplikasi (`static/images/article-fallback.webp`, hasil
 * konversi WebP dari `article-fallback.png`). Path relatif dipakai supaya
 * nilainya tidak terikat host: gambar tetap benar di localhost maupun produksi.
 *
 * Catatan jalur publikasi: path relatif tanpa skema/host dulu selalu dilewati
 * `peUploadFeaturedImage` sehingga artikel terbit tanpa featured image. Sekarang
 * path relatif diresolusikan ke origin publik server sebelum diunduh, jadi
 * covernya ikut terkirim. Di localhost pengunduhan itu tetap diblokir penjaga
 * SSRF (host lokal), jadi ujilah di produksi untuk memastikan featured image-nya.
 */
export const ARTICLE_DEFAULT_COVER = '/images/article-fallback.webp';

/**
 * URL berkas unggahan platform yang tersimpan absolut (mis.
 * `https://app.tauthub.com/uploads/x.webp`, dari PUBLIC_APP_URL server) dibalik
 * ke path relatif supaya selalu dimuat dari origin yang sedang dibuka.
 *
 * Penting saat dev lokal: file ada di server lokal, jadi URL absolut ke
 * produksi pasti gagal (401/404) — dulu setiap halaman menambal sendiri dengan
 * host hardcoded. URL non-`/uploads/` (CDN/partner) dibiarkan apa adanya.
 */
export function sameOriginAssetUrl(url: string | null | undefined): string | null {
	if (!url) return null;
	if (/^https?:\/\//i.test(url)) {
		try {
			const parsed = new URL(url);
			if (parsed.pathname.startsWith('/uploads/')) return parsed.pathname + parsed.search;
		} catch {
			return url;
		}
	}
	return url;
}
