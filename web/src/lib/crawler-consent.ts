/**
 * Dokumen persetujuan akses crawler mitra (port lib/crawler/consent.ts app lama).
 *
 * Teks TIDAK boleh diubah tanpa menaikkan versinya: jejak audit menyimpan versi
 * yang disetujui, sehingga persetujuan lama tetap bisa dibedakan.
 */
export const CRAWLER_CONSENT_VERSION = '2';

/** Jendela crawl harian — dipakai dokumen persetujuan dan keterangan di layar. */
export const CRAWLER_CRAWL_WINDOW = '00.00 – 04.00 WIB';

export const CRAWLER_CONSENT_TEXT = `PERJANJIAN AKSES CRAWLER TAUTHUB

1. TUJUAN
Tauthub memeriksa website secara otomatis (crawling) untuk menilai kualitas
konten dan performa website. Hasil pemeriksaan ini yang membuat website mitra
tampil lengkap dan terpercaya di katalog tauthub.com, sekaligus menjadi dasar
penerbitan artikel klien secara otomatis.

2. YANG TAUTHUB AKSES
- Halaman publik website, sama seperti pengunjung biasa.
- Data teknis dasar (kecepatan, struktur halaman, ketersediaan konten).

3. KAPAN TAUTHUB AKSES
- Pemeriksaan dijadwalkan otomatis pada jendela crawl harian pukul
  ${CRAWLER_CRAWL_WINDOW}, saat trafik website paling sepi.
- Di luar jendela tersebut crawler tidak menjalankan pemeriksaan, sehingga
  trafik dan performa website mitra tidak terganggu.

4. YANG TIDAK PERNAH TAUTHUB AKSES
- Panel admin, akun, atau bagian website yang butuh login.
- Data pengunjung atau data pribadi.
- Konfigurasi keamanan website (Cloudflare/WAF tetap aktif untuk semua orang).

5. KEAMANAN KREDENSIAL
- Setiap website mendapat kredensial yang berbeda dan hanya berlaku untuk
  website itu sendiri.
- Setiap permintaan crawler ditandatangani secara kriptografis dan kedaluwarsa
  dalam hitungan menit, sehingga tidak bisa dipakai ulang oleh pihak lain.
- Mitra dapat menonaktifkan akses ini kapan saja tanpa menghubungi siapa pun.

6. PERSETUJUAN
Dengan membuat kredensial, mitra menyatakan memahami dan menyetujui seluruh
poin di atas, dan mengizinkan crawler Tauthub untuk mengakses website sesuai
batasan poin 2 dan 3 setelah pemasangan kredensial selesai.

Versi dokumen: 2`;
