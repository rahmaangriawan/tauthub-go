# Plan: Monitoring Order Kampanye per Klien (ADMIN)

> Status: **DIEKSEKUSI — 2026-10-27** (endpoint
> `/v1/platform/orders/campaigns[/{campaignId}]` di
> `api/internal/domains/platform/misc_finance.go`, tab Per Kampanye di
> `/admin/orders`, detail `/admin/orders/kampanye/[campaignId]`).

## Latar

Dari klien (langsung, dalam percakapan):

- "ketika ada klien order, kampanye tertentu, ini aku bisa cek di halaman apa"
- "ketika ada klien top up, ini arsipnya aku bisa lihat di halaman apa"

Kesepakatan akhir di obrolan:

> "admin perlu bisa ngecek soal order / kampanye masuk (per klien), bisa lihat
> detail yang di order, bisa lihat kampanyenya habis berapa rupiah"

Halaman yang sudah dikenal klien: `https://app.tauthub.com/admin/orders`
(Monitoring Order per-artikel). Kondisi saat ini: satu baris = satu publikasi,
order kampanye tidak berdiri sebagai entitas yang bisa dieleve detail-nya.

## 1. Backend — `api/internal/domains/platform/operations.go` (+ route di `cmd/api/main.go`)

Dua endpoint baru, `adminGuard` (pola sama seperti `OrdersList`):

**a) `GET /v1/platform/orders/campaigns?q=&limit=&offset=`** — daftar kampanye
level order, terbaru dulu:

- Kolom: `campaign_id`, `order_code` (CAM-), `name`, `brand_name`,
  `brand_public_id`, `taut_status`/`status`, `media_count` (media terorder),
  `publication_count` (publikasi non-cancelled), `published_count`,
  `planned_cost = SUM(COALESCE(taut_campaign_media.price, media.price))`,
  `realized_cost = SUM(publications.client_price WHERE status <> 'cancelled')`,
  `created_at`.
- `q` mencocokkan order_code/kampanye/brand. Limit/offset pola `OrdersList`
  (`platform/misc_finance.go:910-983`).

**b) `GET /v1/platform/orders/campaigns/{campaignId}`** — detail satu
order/kampanye:

- Info kampanye + klien + tanggal.
- Daftar media terorder (dari `taut_campaign_media` LEFT JOIN `media`), tiap
  baris: media nama/website, kategori, harga, judul artikel (`content_documents`),
  status artikel, status publikasi + URL terbit, `publication_id`.
- Kampanye jalur lama tanpa `taut_campaign_media`: fallback daftar dari
  `publications` dikelompokkan per media.
- Total: `total_planned` (semua media terorder), `total_realized` (publikasi
  non-cancelled), `remaining`, dan sisa vs `campaigns.budget` bila diisi.
- Harga bila publikasi belum ada: `media.price * (1 +
  pricing_config.markup/100)` — paritas dengan matchmaker.

## 2. Frontend — halaman Monitoring Order jadi dua tab

`web/src/routes/admin/orders/+page.svelte`:

- Tab pill di bawah PageHeader: **"Per Artikel"** (tabel berjalan sekarang,
  kode tidak disentuh) dan **"Per Kampanye"** (baru, default dari
  `?tab=kampanye`).
- Tab Per Kampanye: pencarian (code/nama/brand), tabel: `No. Kampanye (CAM-)`
  → link ke detail, `Nama`, `Klien`, `Status` (Badge dipetakan `taut_status` /
  `status`), `Media` (jumlah), `Terbit`, `Rencana` (Rp), `Terpakai` (Rp),
  `Waktu`. Pagination pola yang sudah ada.

## 3. Halaman detail `web/src/routes/admin/orders/kampanye/[campaignId]/+page.svelte`

Mengikuti pola `/admin/publications/[publicationId]`:

- Header: kode order CAM- + nama kampanye + klien; tombol kembali ke list.
- Kartu ringkas: **Rencana** (Rp semua media terorder), **Terpakai** (Rp
  publikasi non-cancelled), **Sisa budget** (bila `campaigns.budget` > 0),
  jumlah media vs jumlah terbit.
- Tabel per media: Media, Kategori, Artikel, Harga, Status artikel, Status
  terbit (Badge), URL Terbit link-out; baris dengan `publication_id` link ke
  `/admin/publications/{id}` untuk detail teknis.

## Definisi rupiah

- **Terpakai (realisasi)** = `SUM(publications.client_price)` non-cancelled —
  uang nyata yang sudah/sedang dibayar.
- **Rencana** = `SUM` harga seluruh media terorder
  (`taut_campaign_media.price`, fallback `media.price × markup`) — angka yang
  sama yang dipakai Ringkasan klien (`totalCostIdr`).

## Pengajuan top-up & penarikan dana: TIDAK perlu dibuat baru

- Pengajuan top-up / klaim klien → `/admin/credit-reports`
  (`OperationsCreditReports`), permission `topup`.
- Pembayaran & invoice → `/admin/payments`, `/admin/invoices`.
- Penarikan dana mitra → `/admin/settlements` (approve/reject/kirim bukti).

Cukup diberi tahu ke klien di chat;回答说 dengan tiga link itu.

## Urutan implementasi saat dieksekusi

1. Endpoint daftar kampanye (`OperationsOrders`) + route.
2. Endpoint detail kampanye (`OperationsOrderDetail`) + route.
3. Tab "Per Kampanye" di `/admin/orders`.
4. Halaman detail `/admin/orders/kampanye/[campaignId]`.
5. i18n (`dictionaries.ts` + `phrases.ts`), `go build`, `bun run check`.
