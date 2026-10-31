# Konvensi Port UI & Backend (internal, untuk agen)

> Dokumen kerja. Bukan bagian dari `REWRITE/` yang dipublikasikan. Tujuannya
> menyamakan hasil kerja banyak agen: UI harus sama persis dengan app Next.js
> lama, alur harus sama, backend harus mendukung UI-nya.

## 0. Aturan wajib

> Aturan kerja umum (termasuk larangan menjalankan build/check tanpa perintah
> user) ada di `AGENTS.md` di root repo. Baca itu dulu.

1. **Jangan menebak.** Baca dulu sumber app lama di `/Users/mini/tauthub-ke-go/apps/web/src/`
   (untuk UI: `app/<area>/**/page.tsx` + `sections.tsx`; untuk API: `app/v1/**/route.ts`).
2. **Port perilaku, bukan kira-kira.** String error, status code, urutan validasi,
   dan bentuk JSON harus sama dengan app lama.
3. **Jangan jalankan verifikasi.** Jangan `go build`/`go vet`/`svelte-check`/`npm run build`/
   curl/test. Fokus menulis kode. Verifikasi terpusat dilakukan di akhir oleh koordinator.
   Verifikasi hanya boleh dijalankan bila user memintanya secara eksplisit.
4. **Jangan mengubah file milik agen lain.** Hanya sentuh file di daftar tugasmu.
5. **Jangan `gofmt` seluruh repo.** Format hanya file yang kamu buat/ubah.
6. **Jangan commit.** Koordinator yang commit.

## 1. Gaya kode Go (backend)

- Paket domain per area: `api/internal/domains/<area>/`. Pakai `adminshared` untuk
  guard admin + helper respons:

```go
func (d *Deps) Handler(w http.ResponseWriter, r *http.Request) {
    user, ctx, ok := d.Admin.Require(w, r, "media") // permission: "" bila bebas admin
    if !ok { return }
    _ = user; _ = ctx
    adminshared.WriteJSON(w, http.StatusOK, map[string]any{...})
}
```

- Tipe `Deps` di paketmu:

```go
type Deps struct {
    Pool  *pgxpool.Pool
    Cfg   *config.Config
    Admin *adminshared.Deps   // set di main.go
}
```

- Error: `adminshared.Err(status, "pesan")`, tulis lewat `d.Admin.WriteErr(w, err)`.
  Envelope selalu `{"detail": "..."}`.
- Query: `pool.Query`/`QueryRow` dengan placeholder `$1..$n`. Untuk hasil dinamis
  (SELECT * + kolom tambahan) pakai pola `rowMap` dari
  `api/internal/domains/partner/rowmap.go` — **jangan copy**, ekspor ulang bila perlu
  atau baca kolom eksplisit.
- Waktu ke JSON: `adminshared.IsoMilli(t)`.
- Secret: seal/unseal pakai `internal/crypto` (`Seal`/`Unseal` = v1/v2 `t2:`,
  `SealV3`/`UnsealV3` = `t3:`). Jangan pernah menulis format lain.
- Audit: `INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES (gen_random_uuid(), $1, $2, $3, $4)`.
- Notifikasi: lihat `api/internal/domains/partner/shared.go` (fan-out admin/partner/brand).
- Rate limit: `auth.CheckAuthRate(ctx, pool, bucket, limit)`.

## 2. Gaya kode Svelte (frontend)

Stack: SvelteKit 2 + Svelte 5 runes, `$app/state` (`page`), Tailwind v4, `@lucide/svelte`.

**Wajib ikuti pola yang sudah ada** — baca contoh ini sebelum menulis:

| Kebutuhan | Contoh yang sudah ada |
|---|---|
| Shell + sidebar + URL public_id | `web/src/routes/mitra/[partnerId]/+layout.svelte` |
| Halaman list + filter + modal | `web/src/routes/mitra/[partnerId]/media/+page.svelte` |
| Halaman dengan aksi/alur | `web/src/routes/mitra/[partnerId]/publications/+page.svelte` |
| Halaman form sederhana | `web/src/routes/mitra/[partnerId]/profile/+page.svelte` |
| Kartu ringkasan + statistik | `web/src/routes/mitra/[partnerId]/+page.svelte` |

Kelas CSS bersama (dari `web/src/app.css`, token asli app lama):
`app-shell`, `app-sidebar`, `app-sidebar-inner`, `app-sidebar-brand`, `app-sidebar-nav`,
`app-sidebar-label`, `app-sidebar-link`, `app-account-trigger`, `app-account-info`,
`app-account-name`, `app-account-email`, `app-role-badge`, `app-utility-bar`,
`app-credit-pill`, `app-content`, `app-card`, `app-input`, `app-btn`,
`app-btn-primary`, `app-btn-soft`.

Variabel CSS: `--background --card --border --foreground --muted --muted-foreground
--primary --primary-foreground --secondary --accent --accent-foreground --success
--success-soft --warning --warning-soft --destructive --radius --radius-panel
--radius-control --radius-button`.

Pola wajib setiap halaman:

```svelte
<script lang="ts">
  import { onMount } from 'svelte';
  import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

  let loading = $state(true);
  let error = $state('');
  let notice = $state('');

  onMount(() => {
    void (async () => {
      const fresh = await ensureValidSession();
      if (!fresh) return;              // layout/gating yang mengarahkan ke /login
      await load();
      loading = false;
    })();
    const onExpired = () => (window.location.href = '/login');
    window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
    return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
  });
</script>
```

- `apiRequest(path, init)` sudah: menambah Bearer token, refresh otomatis saat 401,
  toast sukses/gagal untuk non-GET, `credentials: 'include'`.
- Error tampilkan lewat `readApiError(res, 'fallback pesan')` — selalu beri fallback.
- Format uang: `Rp ${(n ?? 0).toLocaleString('id-ID')}`. Tanggal:
  `new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })`.
- Jangan menambah menu/section yang tidak ada di app lama.
- Tulis teks UI dalam bahasa Indonesia, sama seperti app lama.

## 3. Yang sudah tersedia (jangan dibangun ulang)

**Frontend sudah ada**: `/login`, `/register-role`, `/register/klien`, `/register/mitra`,
`/forgot-password`, `/reset-password`, `/login/google-callback`, `/login/oauth-link`,
`/klien` + `/klien/[brandId]/*` (14 route), `/mitra` + `/mitra/[partnerId]/*` (13 route).

**Backend sudah ada**: auth (18), brands (133 sebagian — fase 2–4), knowledge, content
pipeline, clientread, **partner portal 63 endpoint lengkap**.

**Backend BELUM ada (harus dibangun)**: seluruh `/v1/platform/*` (173 endpoint),
`/v1/payments/*` (4), publik `/v1/publishing/*` yang belum.

## 4. Endpoint yang harus dibangun agen backend

Lihat `docs/REWRITE/04_API_ENDPOINTS.md` §5 (payments) dan §6 (platform, 173 endpoint).
Kontrak asli: `/Users/mini/tauthub-ke-go/apps/web/src/app/v1/platform/**` (173 `route.ts`)
dan `/Users/mini/tauthub-ke-go/apps/web/src/app/v1/payments/**`.

## 5. Atomic Write & import di Go

Tulis file lengkap dengan Write (bukan patch sebagian). Kalau file sudah ada dan
harus diubah, baca dulu lalu tulis utuh.

Wire route **tidak** dilakukan agen — koordinator yang menambahkan ke
`api/cmd/api/main.go` supaya tidak ada konflik. Agen cukup menyediakan method handler
dengan nama jelas, contoh `func (d *Deps) UsersList(w http.ResponseWriter, r *http.Request)`,
dan melaporkan daftar nama method + path di laporan akhir.
