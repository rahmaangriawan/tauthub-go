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
