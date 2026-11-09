# AGENTS.md — aturan kerja di repo ini

## 0. JANGAN jalankan verifikasi/build kecuali diminta

**Dilarang** menjalankan perintah berikut atas inisiatif sendiri, termasuk
"sekadar memastikan" setelah mengubah kode:

- `bun run check` / `svelte-check` / `bun run build`
- `go build ./...` / `go vet ./...` / `go test ./...`
- `npm run …`, `npx …`, `tsc`, `gofmt -l` untuk seluruh repo

Tunggu perintah eksplisit dari user ("cek dulu", "build", "test", "pastikan
hijau", dan sejenisnya). Kalau perubahan sudah selesai, **berhenti dan
laporkan** apa yang diubah — biarkan user yang memutuskan kapan memverifikasi.

Alasan: user adalah penguji. Menjalankan check/build sendiri memakan waktu,
mengotori log, dan menimbulkan kesan pekerjaan sudah tervalidasi padahal user
belum memintanya.

Yang tetap boleh tanpa diminta: membaca file, `grep`, query DB read-only, dan
menjalankan dev server/screenshot **hanya bila** itu bagian dari tugas yang
diminta (mis. user minta perbaiki tampilan lalu ingin melihat hasilnya).

## 1. Dua tree — edit yang benar

| Path | Status |
|---|---|
| `/Users/mini/tauthub-go` | **Live.** Repo git. Semua perubahan ke sini. |
| `/Users/mini/tauthub-ke-go` | Snapshot app Next.js lama. **Read-only**, jangan diedit. |

- Frontend yang dijalankan user: `/Users/mini/tauthub-go/web` (Vite dev, port 3001).
- API: `/Users/mini/tauthub-go/api` (port 8080).
- `go` tidak ada di PATH: `export PATH="$HOME/sdk/go1.27.1/bin:$PATH"`.
- `bun` di `~/.bun/bin`.

## 2. Konvensi kode

Baca `docs/CONVENTIONS_AGENT.md` sebelum menulis kode Go atau Svelte. Isinya:
pola `Deps`/`adminshared`, kelas CSS `app-*`, token warna, dan pola halaman
(`ensureValidSession` + `apiRequest` + `readApiError`).

## 3. Commit

Jangan commit atau push kecuali diminta. Biarkan perubahan di working tree dan
sebutkan file apa saja yang berubah di laporan akhir.

## 4. Deploy: HANYA production, JANGAN sentuh staging & quality

**Satu-satunya target deploy yang dipakai sekarang: production.**

| Target | Host / unit | Aturan |
|---|---|---|
| **Production** `app.tauthub.com` | `ubuntu@43.156.237.97`, `/home/tauthub/htdocs/app.tauthub.com`, unit `tauthub-{api,web,scheduler,worker}` | **Satu-satunya yang boleh di-deploy.** |
| Staging `staging.tauthub.com` | `/opt/tauthub/app`, unit `tauthub-staging-{api,web,scheduler}` | **JANGAN di-deploy.** Sementara tidak dipakai. |
| Quality (`quality.yml`) | GitHub Actions, `go vet`/`go test`/`bun run check` | **JANGAN dijalankan/diurus.** Sementara tidak dipakai. |

Alasannya: staging dan quality cuma menyisakan noise (job merah, notifikasi
email, waktu CI) tanpa memberi nilai selama alur kerja masih satu orang dan
production sudah jadi tempat uji sebenarnya. Fokus ke production saja.

### Jebakan yang harus diingat

`.github/workflows/deploy.yml` (Deploy Staging) punya pemicu
`on: push: branches: [master]`. Artinya **setiap push ke master ikut
memicu deploy staging** ke `/opt/tauthub/app`.

**Status: sudah di-disable di GitHub** (3 Nov 2026) lewat
`PUT /repos/…/actions/workflows/<id>/disable`:

| Workflow | ID | State |
|---|---|---|
| Deploy Staging (Go) | `372359243` | `disabled_manually` |
| quality | `371006538` | `disabled_manually` |
| Deploy Production | `375065652` | `active` |

Jadi push ke master sekarang aman — tidak ada lagi deploy staging atau CI
yang jalan. Kalau suatu saat workflow ini di-enable lagi, pemicunya harus
diputus dulu sebelum push berikutnya.

Catatan: sebelum di-disable, dua run terakhir (push `0455b65` dan `d20cff1`)
memang jalan dan **gagal** di tahap rsync (`ssh: connect to host … port 22:
Connection timed out`), jadi staging di `/opt/tauthub/app` tidak pernah
tersentuh — unit staging masih `ActiveEnterTimestamp 2026-10-30 21:33:53`.

Cara men-disable/mengecek tanpa `gh` (tidak terpasang di mesin ini): ambil
token dari keychain lalu pakai GitHub REST API.

```sh
TOKEN=$(printf 'protocol=https\nhost=github.com\n\n' | git credential-osxkeychain get | sed -n 's/^password=//p')
curl -s -H "Authorization: Bearer $TOKEN" \
  https://api.github.com/repos/rahmaangriawan/tauthub-go/actions/workflows \
  | python3 -c "import sys,json;[print(w['id'],w['state'],w['name']) for w in json.load(sys.stdin)['workflows']]"
```

### Prosedur deploy production (ringkas)

1. Build binary linux di lokal: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build
   -trimpath -ldflags="-s -w" -o bin/<cmd> ./cmd/<cmd>` untuk `api`, `worker`,
   `scheduler`, `migrate` — **worker wajib ikut**, pernah hilang dan bikin
   `tauthub-worker` mati dengan `203/EXEC`.
2. `pg_dump` ke `/home/tauthub/backups/pre-deploy-<ts>.sql.gz` sebelum apa pun.
3. `rsync` ke `~/tauthub-sync/` lalu `sudo rsync` ke
   `/home/tauthub/htdocs/app.tauthub.com/` (exclude `.env`, `api/.data/`,
   `web/build/`, `web/node_modules/`).
   Folder `Dokumentasi/` HARUS ikut terkirim: halaman publik `/dokumentasi`
   dibangun dari `Dokumentasi/**/*.md` saat `bun run build` di server.
4. `./bin/migrate up` sebagai user `tauthub`.
5. Build web di server: `bun install --frozen-lockfile && bun run build`.
6. `systemctl restart tauthub-api tauthub-web tauthub-scheduler tauthub-worker`.
7. Verifikasi: `systemctl is-active` keempatnya, `healthz` 200, dan halaman
   publik merespons. Jangan lapor "selesai" tanpa bukti ini.
