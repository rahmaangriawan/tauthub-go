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
