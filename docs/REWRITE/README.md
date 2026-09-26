# REWRITE — Master Plan: Next.js Monolith → Go + Svelte

Pedoman lengkap untuk rewrite total **Tulisio/Tauthub** (saat ini Next.js 16 monolith di `apps/web`, ±148K baris) menjadi:

- **Backend**: Go (chi + pgx + sqlc)
- **Frontend**: SvelteKit 5 (Svelte 5 + shadcn-svelte + Tailwind v4)
- **Deployment**: VPS systemd (api + worker + web)
- **Repo**: repo baru terpisah; repo lama hanya referensi
- **Database**: PostgreSQL `tauthub-go` — **tetap dipakai apa adanya**, schema tidak dirombak

## Isi folder

| File | Isi | Kapan dibaca |
|---|---|---|
| [00_MASTER_PLAN.md](00_MASTER_PLAN.md) | Tujuan, definisi parity, prinsip, stack, struktur repo, cutover, risiko | Sebelum mulai apa pun |
| [01_BACKEND_GO.md](01_BACKEND_GO.md) | Arsitektur backend Go: layout, auth, DB, job queue, crypto compat | Saat setup & port backend |
| [02_FRONTEND_SVELTE.md](02_FRONTEND_SVELTE.md) | Arsitektur SvelteKit: routing, auth client, komponen, polling | Saat setup & port frontend |
| [03_DATABASE.md](03_DATABASE.md) | Inventaris 160 tabel per domain + strategi migration + aturan kompat | Saat menyentuh DB |
| [04_API_ENDPOINTS.md](04_API_ENDPOINTS.md) | Inventaris lengkap 406 endpoint (checklist parity) | Saat port tiap domain |
| [05_UI_PAGES.md](05_UI_PAGES.md) | Inventaris 89 halaman + section + layout (checklist parity) | Saat port tiap section UI |
| [06_INTEGRATIONS.md](06_INTEGRATIONS.md) | Kontrak semua integrasi eksternal (AI, payment, publishing, dll.) | Saat port integrasi |
| [07_EXECUTION_PHASES.md](07_EXECUTION_PHASES.md) | 13 fase eksekusi vertical-slice + definition of done | Peta kerja harian |

## Cara pakai

1. Baca `00_MASTER_PLAN.md` sekali untuk align arah.
2. Kerjakan fase sesuai `07_EXECUTION_PHASES.md`, satu fase = backend + frontend + verifikasi.
3. Setiap fase, centang item di `04_API_ENDPOINTS.md` dan `05_UI_PAGES.md` — **selesai = 100% checklist fase itu hijau**, bukan "kira-kira sudah jalan".
4. Detail teknis yang mudah salah (crypto, JWT, job queue, HMAC) ada di `01_BACKEND_GO.md` dan `06_INTEGRATIONS.md` — jangan improvise dari nama endpoint saja.

## Angka kunci (baseline codebase lama, diukur 2026-09-26)

- **148.199 baris / 994 file** TS+TSX di `apps/web/src`
- **406 endpoint API** (brands 133 · platform 173 · partner 63 · auth 18 · lainnya 19)
- **89 halaman UI + 33 layout** (admin 54 · klien 15 · mitra 2 route/19 section · publik 18)
- **160 tabel PostgreSQL** (termasuk `schema_migrations`)
- **153 test unit** (Vitest) · **12+ job kind** · **tanpa SSE/WebSocket** (murni polling)
