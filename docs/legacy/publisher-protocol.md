# Tauthub Publisher Protocol v1

Spesifikasi resmi untuk website custom Mitra yang ingin menerima artikel dari Tauthub secara otomatis (push). Framework-agnostic — PHP/Laravel, Node/Express, Next.js, Django, atau custom PHP mana pun yang bisa melayani HTTP POST JSON dapat mengimplementasikan protocol ini.

> Untuk media WordPress, gunakan integrasi WordPress REST API existing (bukan protocol ini). Protocol ini khusus platform media **Custom Web**.

---

## Arsitektur

```
TAUTHUB
  ↓  HTTPS + API Key
https://mitra.com/v1/tauthub/publish   ← endpoint MILIK MITRA
  ↓
CUSTOM WEB MITRA (CMS / database / publishing logic milik Mitra)
  ↓  response JSON
TAUTHUB (publication report: remote_id, remote_url, status)
```

Tauthub **tidak pernah** membuat route di server Mitra. Mitra menyediakan endpoint receiver sesuai spesifikasi di bawah; Tauthub menyediakan protocol, adapter pengiriman, test connection, dan reporting.

---

## Autentikasi

Setiap request membawa header:

```
Authorization: Bearer thb_xxxxxxxxxxxxxxxx
X-Tauthub-Protocol-Version: 1.0
X-Tauthub-Request-Id: pub_<uuid>        (unik per request; retry memakai id yang sama)
Content-Type: application/json
```

Alias `X-Tulisio-Protocol-Version` / `X-Tulisio-Request-Id` tetap diterima untuk kompatibilitas (`thb_` & `tli_` juga).

- API Key dibuat & disegel (AES-256-GCM) di halaman **Mitra → Koneksi API → Custom Web Publishing** (`thb_` + 32 byte base64url). Tidak pernah ditampilkan plaintext setelah disimpan, tidak pernah masuk frontend client.
- Mitra **wajib memvalidasi** header `Authorization` di server-nya sendiri (bandingkan constant-time dengan key yang disimpan).
- Tolak request dengan `X-Tauthub-Protocol-Version` yang tidak dikenal → `UNSUPPORTED_PROTOCOL_VERSION`.

---

## Endpoint 1 — Health check / Ping

`POST {endpoint}/v1/tauthub/ping` (alias `/v1/tulisio/ping` tetap diterima)

Request body:

```json
{
  "protocol_version": "1.0",
  "timestamp": "2026-08-21T09:00:00.000Z",
  "request_id": "ping_<uuid>"
}
```

Expected response (HTTP 200):

```json
{
  "success": true,
  "service": "tauthub",
  "protocol_version": "1.0"
}
```

Dipakai tombol **Test Connection** di halaman Koneksi API. Timeout Tauthub: 15 detik.

---

## Endpoint 2 — Publish artikel

`POST {endpoint}/v1/tauthub/publish` (alias `/v1/tulisio/publish` tetap diterima)

Request body:

```json
{
  "publication_id": "uuid",
  "title": "Judul Artikel",
  "content": "<p>Isi artikel…</p>",
  "excerpt": "Ringkasan artikel",
  "slug": "judul-artikel",
  "status": "publish",
  "category": { "name": "Bisnis", "slug": "bisnis" },
  "featured_image": { "url": "https://…", "alt": "…" },
  "published_at": null
}
```

| Field | Wajib | Catatan |
| --- | --- | --- |
| `publication_id` | ya | UUID publikasi Tauthub — pakai sebagai kunci dedup internal. |
| `title` | ya | Judul artikel. |
| `content` | ya | HTML final (H1 sudah dipisah ke `title`). |
| `excerpt` | tidak | Boleh null. |
| `slug` | tidak | Slug URL yang disarankan. |
| `status` | ya | `draft` = simpan sebagai draft; `publish` = terbitkan. Source of truth status tetap publication flow Tauthub. |
| `category` | tidak | Objek `{name, slug}` — Mitra memetakan ke taxonomy internal; jika tidak ditemukan kembalikan `CATEGORY_NOT_FOUND`. |
| `featured_image` | tidak | `{url, alt}`. URL yang dikirim adalah URL publik yang dapat diakses internet (bukan localhost). Jika Mitra gagal mengambil gambar, **jangan gagalkan artikel** — terbitkan tanpa featured image. |
| `published_at` | tidak | ISO timestamp, atau null (Tauthub yang mengisi setelah konfirmasi). |

### Response sukses (HTTP 200/201)

```json
{
  "success": true,
  "status": "published",
  "remote_id": "5842",
  "remote_url": "https://mitra.com/judul-artikel",
  "message": "Article published successfully"
}
```

- Untuk draft: `"status": "draft"` dan `remote_url` boleh `null` bila belum ada URL publik.
- `remote_id` disimpan Tauthub sebagai `publication_jobs.remote_post_id`; `remote_url` otomatis menjadi URL publikasi (muncul sebagai "Lihat di Web" di dashboard, tanpa input manual Mitra).

### Response error

```json
{
  "success": false,
  "status": "failed",
  "error_code": "CATEGORY_NOT_FOUND",
  "message": "Category bisnis tidak ditemukan"
}
```

`error_code` yang dikenal Tauthub:

| error_code | Arti | Tauthub retry? |
| --- | --- | --- |
| `INVALID_API_KEY` | Bearer key salah | Tidak (4xx, permanen) |
| `INVALID_PAYLOAD` | Body tidak valid | Tidak |
| `UNSUPPORTED_PROTOCOL_VERSION` | Version header tidak dikenal | Tidak |
| `CATEGORY_NOT_FOUND` | Kategori tidak ada di sisi Mitra | Tidak |
| `MEDIA_UPLOAD_FAILED` | Gagal proses featured image | Tidak |
| `DUPLICATE_PUBLICATION` | request_id sudah pernah diproses — kembalikan hasil pertama | Tidak (dianggap sukses) |
| `PUBLISH_FAILED` | Gagal umum di sisi Mitra | Ya (5xx-equivalent) |
| `INTERNAL_ERROR` | Error tak terduga Mitra | Ya |

Jangan pernah mengembalikan stack trace, kredensial database, secret, atau server path.

---

## Idempotency & Retry

- `X-Tauthub-Request-Id` (`X-Tulisio-Request-Id` alias tetap diterima) unik per publication job dan **dipertahankan saat retry**. Mitra disarankan menyimpan `request_id → hasil` sehingga request ulang (timeout/network) membalas hasil pertama, bukan membuat artikel kedua (`DUPLICATE_PUBLICATION` atau balas sukses lama).
- Sisi Tauthub: retry dimiliki job system existing (backoff, max attempts). 5xx/timeout → retry terbatas; 4xx permanen → tidak di-retry. Tauthub juga melakukan reconciliation by slug sebelum membuat post baru.
- Timeout request Tauthub: 30 detik (publish), 15 detik (ping).

---

## Checklist implementasi Mitra

1. Buat endpoint `POST /v1/tauthub/ping` dan `POST /v1/tauthub/publish` (alias `tulisio` tetap diterima; path dasar dikonfigurasi di halaman Koneksi API).
2. Validasi `Authorization: Bearer <key>` (constant-time compare) dan `X-Tauthub-Protocol-Version: 1.0` (alias `X-Tulisio-Protocol-Version` tetap diterima).
3. Simpan artikel sesuai `status` (`draft`/`publish`); petakan `category` ke taxonomy internal.
4. Kembalikan JSON sesuai kontrak di atas (`success`, `status`, `remote_id`, `remote_url`) — `service` di ping membalas `tauthub` (atau `tulisio` untuk kompatibilitas).
5. Simpan `X-Tauthub-Request-Id` (alias `X-Tulisio-Request-Id`) untuk dedup.
6. Set endpoint URL + API Key di **Mitra → Koneksi API → Custom Web Publishing**, klik **Test Connection** sampai ✓ Terhubung.

Setelah terhubung, setiap artikel ke media tersebut terkirim otomatis; URL terbit muncul otomatis di dashboard Mitra dari `remote_url`.
