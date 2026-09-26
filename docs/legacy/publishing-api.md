# Tauthub Publishing API — Kontrak untuk Website Custom Mitra

Dokumen ini adalah **kontrak** yang harus diimplementasikan oleh website custom
milik Mitra (media partner) agar dapat menerima artikel dari Tauthub secara
otomatis. Tauthub hanya menyediakan kontrak ini; adaptor (endpoint penerima)
dibangun oleh tim website Mitra sendiri.

Pendekatan ini berbeda dari integrasi WordPress (Tauthub menulis langsung ke
REST API WordPress). Dengan Publishing API, **Mitra mengendalikan endpoint**
dan Tauthub mengirim artikel dengan aman ke endpoint tersebut.

---

## 1. Alur

```
Tauthub (Distribution Engine)
   │  POST https://situs-mitra.example/api/tauthub/publication
   │  Authorization: Bearer <api_key>
   │  X-Tauthub-Timestamp: <unix seconds> (X-Tulisio-Timestamp alias tetap diterima)
   │  X-Tauthub-Signature: hex(hmac-sha256(secret, "<timestamp>.<body>"))
   │  X-Tauthub-Idempotency-Key: <idempotency key>
   │  body: JSON (lihat §4)
   ▼
Adaptor website Mitra
   ├── verifikasi signature (HMAC-SHA256) + timestamp
   ├── dedupe berdasarkan X-Tauthub-Idempotency-Key (alias X-Tulisio-Idempotency-Key tetap diterima)
   ├── simpan/terbitkan artikel (publish / draft)
   └── balas: { "accepted": true, "externalId": "...", "status": "publish" }
```

## 2. Kredensial

Mitra membuat API key di portal Mitra → **Publishing API**:

- **API Key** — dikirim sebagai `Authorization: Bearer <api_key>`.
- **Secret** — dipakai untuk memverifikasi `X-Tauthub-Signature` (alias `X-Tulisio-Signature` tetap diterima).

Kedua nilai hanya ditampilkan **sekali** saat dibuat/dirotasi. Simpan di
konfigurasi adaptor Anda. Jika hilang, rotasi kunci dari portal Mitra.

## 3. Header

| Header | Wajib | Keterangan |
| --- | --- | --- |
| `Authorization` | ✅ | `Bearer <api_key>` |
| `Content-Type` | ✅ | `application/json` |
| `X-Tauthub-Timestamp` | ✅ | Unix time (detik). Tauthub menolak selisih > 5 menit. Alias `X-Tulisio-Timestamp` tetap diterima. |
| `X-Tauthub-Signature` | ✅ | `hex(hmac-sha256(secret, "<timestamp>.<body>"))` — alias `X-Tulisio-Signature` tetap diterima |
| `X-Tauthub-Idempotency-Key` | ✅ | ID unik pengiriman (lihat §5). Alias `X-Tulisio-Idempotency-Key` tetap diterima. |

Contoh verifikasi signature (Node.js):

```js
const crypto = require("node:crypto");

function verify(req, secret) {
  const timestamp = req.headers["x-tauthub-timestamp"];
  const signature = req.headers["x-tauthub-signature"];
  const body = rawBody(req); // string mentah, persis seperti dikirim
  const expected = crypto
    .createHmac("sha256", secret)
    .update(`${timestamp}.${body}`)
    .digest("hex");
  const ok = crypto.timingSafeEqual(
    Buffer.from(expected, "hex"),
    Buffer.from(signature, "hex"),
  );
  return ok && Math.abs(Date.now() / 1000 - Number(timestamp)) <= 300;
}
```

## 4. Payload

```json
{
  "publication": {
    "id": "uuid-publikasi",
    "publish_status": "draft",
    "scheduled_at": null,
    "published_at": null,
    "client_price": 250000
  },
  "article": {
    "id": "uuid-dokumen",
    "title": "Judul Artikel",
    "slug": "judul-artikel",
    "content_markdown": "## Bagian\n\nParagraf ...",
    "content_html": "<h2>Bagian</h2><p>Paragraf ...</p>",
    "excerpt": "Ringkasan singkat",
    "meta_title": "Judul SEO",
    "meta_description": "Deskripsi meta",
    "language": "id",
    "primary_keyword": "kata kunci"
  },
  "media": {
    "id": "uuid-media",
    "name": "Nama Media",
    "website_url": "https://situs-mitra.example",
    "partner_id": "uuid-partner"
  },
  "tracking": {
    "publication_id": "uuid-publikasi"
  }
}
```

Catatan:
- `content_markdown` dan `content_html` diberikan keduanya; pilih yang paling
  cocok dengan CMS Anda.
- `article.excerpt` / `meta_*` boleh `null` — sesuaikan fallback.
- `publish_status` bernilai `draft` atau `publish` sesuai pengaturan publikasi
  (default `draft`).
- `tracking.publication_id` dapat dipakai untuk membangun link tracking
  (`go.tauthub.com/...`) setelah artikel terbit.

## 5. Idempotency

`X-Tauthub-Idempotency-Key` (alias `X-Tulisio-Idempotency-Key`) adalah ID pengiriman yang stabil. Jika Tauthub
mengirim ulang (retry), key-nya **sama**. Adaptor Anda **wajib** menyimpan key
ini dan mengembalikan hasil yang sudah ada untuk key yang sama — jangan
membuat artikel duplikat saat retry.

Contoh:

```
key A → simpan artikel → balas accepted
key A (retry) → temukan key A sudah diproses → balas accepted dengan externalId yang sama
```

## 6. Respons yang diharapkan

Sukses (HTTP 2xx):

```json
{
  "accepted": true,
  "externalId": "post-12345",
  "status": "https://situs-mitra.example/artikel/judul-artikel"
}
```

- `accepted` **wajib** `true` — tanpa ini Tauthub menganggap pengiriman gagal.
- `externalId` — ID artikel di sistem Anda (opsional, direkam Tauthub).
- `status` — boleh berupa URL artikel yang terbit; Tauthub menyimpannya sebagai
  `published_url`.

Gagal (HTTP 4xx/5xx):

```json
{ "accepted": false, "message": "alasan penolakan" }
```

Tauthub mencatat pesan ini di `publication_attempts` dan dashboard admin
(Error Reports).

## 7. Test Connection (event `ping`)

Tombol **Test** di Portal Mitra → Media Anda (media ber-integrasi Tauthub
Publishing API) mengirim event `ping` ke Adapter URL dengan header yang sama
seperti pengiriman artikel (bearer + timestamp + signature + idempotency):

```json
{ "event": "ping", "timestamp": "2026-08-10T07:00:00.000Z", "sender": "tauthub", "media_id": "uuid-media" }
```

Adaptor cukup membalas `{ "accepted": true }` (verifikasi signature tetap
wajib) agar tombol Test menampilkan **Koneksi berhasil**. Pesan penolakan
ditampilkan apa adanya di portal mitra.

## 8. Setup di Portal Mitra

1. Portal Mitra → **Media Anda** → tambah/edit media → integrasi
   **Tauthub Publishing API** → isi **Adapter URL**
   (contoh: `https://situs-mitra.example/api/tauthub/publication`).
2. Portal Mitra → **Publishing API** → **Buat API key** → salin
   `api_key` + `secret`.
3. Pasang kedua nilai di konfigurasi adaptor website Anda.
4. Publikasi klien ke media tersebut akan dikirim ke Adapter URL dengan
   signature yang valid. Status pengiriman tampil di portal Mitra
   (Publications) dan admin (Error Reports).

Opsional: buat key terikat ke satu media agar kredensial per media berbeda.

## 9. Callback status dua arah (adaptor → Tauthub)

Untuk distribusi asinkron (mis. artikel masuk draft lalu terbit belakangan),
adaptor dapat melaporkan status sebenarnya kembali ke Tauthub:

```
POST https://tauthub.com/v1/publishing/callbacks
Authorization: Bearer <api_key>
X-Tauthub-Timestamp / X-Tauthub-Signature / X-Tauthub-Idempotency-Key (alias X-Tulisio-* tetap diterima)

{
  "publication_id": "uuid-publikasi",
  "status": "published" | "failed",
  "external_id": "post-12345",
  "url": "https://situs-mitra.example/artikel/...",
  "message": "alasan bila gagal"
}
```

- Signature dihitung atas **body mentah** dengan secret yang sama seperti
  pengiriman; replay > 5 menit ditolak.
- Adaptor **hanya** bisa mengupdate publikasi untuk media miliknya sendiri
  (ownership dicek server-side).
- Idempoten: melaporkan ulang status terminal (sudah `published`/`failed`)
  diterima tanpa efek samping.
- `published` → Tauthub menandai terbit, menyimpan `url` sebagai
  `published_url`, merekam attempt + earning partner, dan menotifikasi klien.
- `failed` → Tauthub menandai gagal (`adapter_callback_failed`), mencatat
  attempt, dan menotifikasi admin Tauthub.

## 10. JSON Pull (fallback terstruktur — tanpa endpoint)

Untuk website custom yang **tidak bisa membuka endpoint penerima** (mis. situs
statis/SSG atau tim tidak bisa deploy adaptor), Tauthub menyediakan pull
tersruktur memakai **token RSS feed yang sama**:

```
GET https://tauthub.com/v1/publishing/feed.json?token=<token-feed-rss>
```

Respons (JSON):

```json
{
  "feed": { "id": "uuid-feed", "name": "Nama Media", "media_id": null, "website_url": "https://situs-mitra.example", "generated_at": "..." },
  "items": [
    {
      "publication_id": "uuid-publikasi",
      "article": {
        "title": "Judul Artikel",
        "slug": "judul-artikel",
        "content_markdown": "## Bagian\n\n...",
        "content_html": "<h2>Bagian</h2><p>...</p>",
        "excerpt": "Ringkasan",
        "meta_title": "Judul SEO",
        "meta_description": "Deskripsi meta",
        "language": "id-ID"
      },
      "media": { "id": "uuid-media", "name": "Nama Media", "website_url": "..." },
      "tracking": { "publication_id": "uuid-publikasi" },
      "published_url": null,
      "published_at": "..."
    }
  ]
}
```

- Token sama dengan RSS feed (menu Mitra → **RSS Feeds**). Satu token, dua
  format: XML untuk RSS importer, JSON untuk script/CI website Anda.
- `content_html` dirender dari Markdown otomatis — pilih yang cocok dengan
  sistem Anda; `content_markdown` diberikan bila format sumber Markdown.
- Rate limit 60 pull/menit per feed. Pull-based, jadi tidak ada jaminan waktu
  nyata: cocok untuk sinkronisasi berkala (cron/CI), bukan publikasi instan.
- Untuk publikasi instan + status + retry, tetap gunakan push API (§1-§9).

## 11. Keamanan

- `secret` tidak pernah dikirim ke browser klien; hanya disimpan terenkripsi
  di sisi server Tauthub.
- Signature berbasis **body mentah** — verifikasi harus memakai string body
  persis seperti diterima (jangan re-stringify JSON).
- Reject request dengan timestamp lebih tua dari 5 menit (anti-replay).
- Rotasi kunci segera bila dicurigai bocor (portal Mitra → Rotasi).
