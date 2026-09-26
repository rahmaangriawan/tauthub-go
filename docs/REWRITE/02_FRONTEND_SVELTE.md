# 02 — Arsitektur Frontend SvelteKit

> Target: **SvelteKit 2 + Svelte 5 (runes)**, adapter-node, Tailwind v4 + shadcn-svelte.
> Frontend baru HANYA melayani halaman (89 halaman + layout); semua API `/v1/*` diserve backend Go — di VPS disatukan lewat nginx (path routing, lihat §9).

---

## 1. Setup Dasar

- `npx sv create` (SvelteKit + TypeScript) → adapter **`@sveltejs/adapter-node`** (VPS systemd, port 3000).
- Tailwind **v4** + `tw-animate-css` (pengganti dependensi animate).
- shadcn-svelte (`npx shadcn-svelte@latest init`) — komponen copy-paste, basis **bits-ui**.
- Font: `@fontsource/geist`, `@fontsource/geist-mono`, `@fontsource/instrument-serif`, `@fontsource/inter` (paket yang sama, jalan di Vite).
- Theme light/dark: **mode-watcher** (port `THEME_LIGHT_DARK_OVERHAUL.md` — token warna via CSS variables, class `.dark`).

## 2. Peta Route (mirror dari Next App Router)

Struktur `src/routes/` mengikuti persis daftar di `05_UI_PAGES.md`:

| Route | Isi |
|---|---|
| `/` | landing publik |
| `/login`, `/register/*`, `/forgot-password`, `/reset-password` | auth (register/klien & register/mitra terpisah) |
| `/klien/[brandId]/[[...section]]` | workspace klien — **catch-all section** (overview, analytics, campaigns, distribution, media, publications, knowledge, members, credits, invoices, notifications, pr-calculator, profile, dashboard) |
| `/klien/editor/[documentId]` | editor dokumen |
| `/klien/taut-pintar/[id]/*` | 8 sub-halaman taut (page, berita-acara, gatekeeper, matchmaker, profiler*, ringkasan, writer) |
| `/mitra/[partnerId]/[[...section]]` | portal mitra — **19 section** (blogger, connections, crawler, earnings, integrations, kalkulator-media, link-monitor, media, notifications, profile, publications, publishing-api, referral, rss-feeds, seo-tool, settlements, sindikasi-konten, vip, vip-dashboard) |
| `/admin/*` | 54 halaman admin (lihat `05_UI_PAGES.md`) |
| `/payments/*`, `/berita-acara/[token]`, `/docs`, `/blogger-pick-blog`, `/google-callback`, `/oauth-link` | publik/semua-role |
| redirects legacy | `/brands→/klien`, `/platform→/admin`, `/partner→/mitra` (permanent, salin dari next.config) |

**Catatan porting penting**: halaman "klien/mitra" bukan 2 halaman — isi aslinya adalah file section raksasa (lihat §7). Ukuran kerja UI = jumlah **section**, bukan jumlah file route.

## 3. Gating & Middleware (R9)

Port `src/proxy.ts` (edge middleware Next) → **`hooks.server.ts`**:

1. Untuk path `/admin/*`, `/mitra/*`, `/klien/*`: verifikasi **refresh_token cookie** (JWT HS256, `APP_SECRET` — sama dengan backend Go, env di-share) → gagal → redirect `/login` + simpan deep-link `returnTo` (header → port jadi cookie/param).
2. Cek role detail tetap di **server layout** (`+layout.server.ts`): admin layout cek role admin; klien layout port `gate.ts` → `assertBrandAccess` (refresh token + brand membership); mitra layout cek role partner.
3. `/v1/*` TIDAK lewat SvelteKit — auth API ada di Go (CORS + Bearer di sana).

## 4. Auth Client (port `src/lib/auth.ts`, 266 baris — port 1:1)

- **Access token di localStorage**, dikirim `Authorization: Bearer` pada tiap request API.
- 401 → **auto-refresh** via `POST /v1/auth/refresh` (cookie httpOnly, `credentials: include`) → simpan access token baru → retry request.
- Window/custom events: `toast`, `user-updated`, `auth-expired` → port jadi Svelte store + dispatcher yang sama supaya komponen (notification bell, shells) tak perlu diubah logikanya.
- `me` di-cache + invalidasi pada auth change.

## 5. Data Fetching & Cache

- Port `src/lib/swr-cache.ts` (169 baris): cache in-memory **TTL 60 detik**, dedupe request, invalidasi on auth change → jadikan `$lib/cache` (Svelte store / Map reaktif). JANGAN ganti dengan TanStack Query dulu — perilaku cache adalah bagian dari parity (freshness polling).
- **Polling**: 13 file lama pakai `setInterval` (notification-bell, payments/status, admin/media, klien overview/distribution/campaign-flow/knowledge, taut agent-steps/gatekeeper/profiler, mitra sections, product-shell). Port dengan `$effect` + `setInterval` yang sama, interval sama. **Tanpa SSE/WebSocket** (dilarang di fase parity).
- AI generation: request → job → **polling** `current_step/progress` (port persis; tidak ada streaming di API lama).

## 6. shadcn-svelte — Mapping dari @radix-ui/themes

| Radix Themes (lama) | shadcn-svelte (baru) |
|---|---|
| `<Theme>` + token | CSS variables Tailwind + mode-watcher |
| Dialog | `dialog` (bits-ui) |
| DropdownMenu | `dropdown-menu` |
| Popover / HoverCard | `popover` / `hover-card` |
| Tabs | `tabs` |
| Select | `select` |
| Switch / Checkbox | `switch` / `checkbox` |
| TextField / TextArea | `input` / `textarea` + `label` |
| Table | `table` |
| Badge / Card | `badge` / `card` |
| Toast | `sonner` |
| Tooltip | `tooltip` |
| AlertDialog (confirm) | `alert-dialog` |
| Progress / Spinner | `progress` / `skeleton` |
| Sidebar nav (app-shell custom) | `sidebar` (shadcn) atau port `shells/product-shell.tsx` manual |

Icons: **`phosphor-svelte`** + **`@lucide/svelte`** (keduanya dipakai di app lama; `optimizePackageImports` tidak diperlukan — Svelte tree-shake otomatis).

Charts (recharts dipakai di dashboard/PR): **shadcn-svelte `chart` (LayerCake)** sebagai default; fallback keputusan: Chart.js bila butuh tipe chart recharts tertentu. Uji dulu di halaman dashboard klien.

Animasi: `motion` 12 + `aos` → svelte `transition`/`animate` + `tw-animate-css`; aos → Svelte action `IntersectionObserver` (halaman landing saja).

## 7. Dekomposisi File Raksasa (target ukuran file baru)

File raksasa lama (React) → pecah jadi komponen Svelte per section. **Kontrak visual & perilaku tidak berubah**, hanya strukturnya:

| File lama | Baris | Pecah jadi |
|---|---|---|
| `mitra/[partnerId]/[[...section]]/sections.tsx` | 5.866 | 19 komponen section (satu per section di §2) |
| `klien/.../distribution.tsx` | 4.251 | per-tab komponen distribution |
| `campaign-flow-module.tsx` | 3.749 | wizard per-step komponen |
| `media-section.tsx` | 1.856 | tabel + filter + modal |
| `admin/media/_sections.tsx` | 1.383 | per-subsection admin |
| `writer/client.tsx` | 1.258 | editor panel komponen |
| `brand-workspace.tsx` | 1.168 | per-section |
| `admin/ai/page.tsx` | 1.143 | tab per fungsi AI |
| `matchmaker/client.tsx` | 1.072 | step komponen |
| `pr-calculator.tsx` | 998 | form + hasil + grafik |
| `profile/partner-profile.tsx` | 1.082 | tab komponen |
| `domain/media-detail-modal.tsx` | 721 | modal + tab |
| `shells/product-shell.tsx` | 574 | shell + sidebar |
| `profile/client-profile.tsx` | 508 | tab komponen |

## 8. Library Client — Bawa / Ganti / Buang

| Kebutuhan | Lama | Keputusan |
|---|---|---|
| XLSX import batch & export | exceljs | **bawa dulu** (jalan di browser, menjaga kontrak: file diparse di client, row dikirim ke API). Alternatif nanti: pindah parse ke Go excelize — itu perubahan kontrak, JANGAN di fase parity |
| Sanitize HTML AI | dompurify | bawa (client-side) |
| Markdown render | `lib/markdown.ts` custom | port manual (84 baris) |
| Excel export laporan | exceljs | bawa |
| SWR cache | custom `swr-cache.ts` | port sendiri (§5) |
| Radix Themes | @radix-ui/themes | BUANG → shadcn-svelte |
| recharts | recharts | BUANG → LayerCake/Chart.js |
| motion, aos | — | BUANG → svelte transitions |

## 9. Nginx & Headers (R10)

- nginx: `web:3000` untuk semua path kecuali `/v1/*`, `/uploads/*`, `/t/*`, `/feed/*` → `api:8080`.
- Security headers salin dari `next.config.ts`: nosniff, X-Frame-Options DENY, HSTS, Permissions-Policy, **CSP Report-Only** (izinkan challenges.cloudflare.com + cloudflareinsights; `unsafe-eval` hanya perlu bila zod jalan di client — di arsitektur baru validasi pindah ke Go, catat bila CSP bisa diperketat).
- Gambar lokal: cache 30 hari, AVIF/WebP hanya bila file memang sudah format itu (transformasi server pindah ke Go, lihat `01 §10`).

## 10. i18n

Port `dictionaries.ts` (485 baris, custom dictionary EN/ID) → `$lib/i18n/dictionaries.ts` **key yang sama persis** (halaman UI label Indonesia). Jangan rombak key saat porting — diff nanti mustahil dilacak.

## 11. Checklist "Definition of Done" Frontend per section

- [ ] Semua aksi (CRUD, submit, approve/reject, polling) memanggil endpoint yang sama dengan payload sama
- [ ] Pesan error dari `{detail}` ditampilkan sama
- [ ] Cache 60s + polling interval sama
- [ ] Guard role sama (admin/partner/klien + brand membership)
- [ ] Label i18n key sama
- [ ] Visual setara (bukan piksel-perfect, tapi struktur & alur sama; tema light/dark berfungsi)
