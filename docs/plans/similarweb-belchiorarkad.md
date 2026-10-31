# Ganti provider trafik: Similarweb API Pro (DataBee) → Similar Web API (belchiorarkad)

## Kenapa

Provider lama (`similarweb-api-pro.p.rapidapi.com`, publisher DataBee) mewajibkan
DUA kredensial: key RapidAPI **dan** DataBee Token sebagai `Authorization:
Bearer`. Di produksi token itu tidak pernah terisi, dan langganan listing-nya
juga tidak aktif. Hasilnya semua job `media_similarweb_refresh` gagal (337 job
dibersihkan) dan kolom trafik kosong.

Provider baru (`similar-web-api.p.rapidapi.com`, developer belchiorarkad) hanya
butuh satu key RapidAPI — kredensial yang sudah ada di
`platform_seo_config.sealed_similarweb_api_key`.

Listing: https://rapidapi.com/belchiorarkad-FqvHs2EDOtP/api/similar-web-api
Endpoint: `GET /analyticsv1?url=<domain>`
Header: `x-rapidapi-key`, `x-rapidapi-host`.

## Yang berubah

### Backend

| Berkas | Perubahan |
|---|---|
| `platform/media_support.go` | Host/URL baru, `mpSimilarWebProviderURL`, `mpSimilarWebDiagnoseDomain`. `mpSimilarWebCredentials` → satu key. `mpSimilarWebHeaders(apiKey)` tanpa Bearer. `mpSimilarWebFetch(ctx, domain, apiKey)`. |
| `platform/media_support.go` | `mpMapSimilarWebAnalytics` + `mpSWUnwrap` + `mpSWFindVisits` + `mpSWPositive`: pemetaan respons `/analyticsv1` yang bentuknya belum didokumentasikan. |
| `platform/media_support.go` | `mpSWAsNumber` menerima `int`, `int64`, `json.Number`, suffix `K/M/B`, dan pemisah ribuan `1.234.567`. |
| `platform/media_checks.go` | Guard kesiapan trafik: cukup key RapidAPI. `last_da_error` tidak lagi memblokir trafik (kolom itu milik provider DA, langganannya terpisah). |
| `brandflow/scheduler.go` | `schTrafficProviderReady` sepadan: hanya key RapidAPI. |
| `platform/misc_integrations.go` | `databee_token` dihapus dari GET/PUT konfigurasi SEO. `igSEOJSON` mengirim `similarweb_provider_url` + `similarweb_host`. Diagnose memakai `/analyticsv1` dengan domain uji, ikut mengembalikan `visits` + `snapshot_date`. `swDiagnoseHint` tanpa menyebut DataBee. |
| `platform/similarweb_headers_test.go` | Kontrak satu key. |
| `migrations/additive/293_...sql` | Hapus job trafik `failed` yang pesannya khas kredensial provider lama. |

`sealed_databee_token` **tidak** di-drop dari skema. Kolom itu tidak dibaca lagi,
dan membiarkannya membuat perubahan ini bisa dibalik tanpa migrasi.

### Frontend (`/admin/konfigurasi`)

- Input DataBee Token dihapus.
- Kredit provider: "Data SimilarWeb di Tauthub diambil lewat **Similar Web API**
  di RapidAPI, dikembangkan oleh **belchiorarkad**" + tautan listing (nofollow).
- Baris host + endpoint yang dipakai backend.
- `<details>` tutorial Go CLI (6 langkah + endpoint lain + error handling).
- Hasil "Tes koneksi provider" menampilkan domain uji, angka kunjungan yang
  terbaca, dan tanggal snapshot.

## Risiko yang belum tertutup

Bentuk JSON `/analyticsv1` tidak dipublikasikan di listing. Pemetaan mencoba
beberapa bentuk yang dikenal (`domain_analytics`, `Engagments.Visits`,
`EstimatedMonthlyVisits`, `traffic.history`, field datar). Kalau tidak ada yang
cocok, `visits` bernilai nil dan tombol Tes koneksi akan bilang begitu sambil
menampilkan jawaban mentah — dari situ pemetaan disesuaikan.

Sampai `visits` terbaca, sweep scheduler akan mengulang media yang sama
(cooldown 24 jam per media), jadi tidak ada ledakan job.
