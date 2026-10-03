# Plan: Ganti "Plagiarism Checker" → "AI Overview Checker" di Analisis QC (Gatekeeper)

Dokumen ini adalah rencana implementasi yang disetujui; status pelaksanaan dicatat di bagian akhir.

## Latar (hasil eksplorasi)

- Kartu "Plagiarism Checker" di QC panel ternyata **stub di jalur live**: brandflow selalu mengembalikan `{status:"review_required", error:"Provider plagiarisme belum dikonfigurasi"}` — tidak pernah benar-benar memeriksa. Menggantinya berarti mengganti kode mati.
- Audit tersimpan di `taut_gatekeeper_audits.checks` (jsonb, kunci `plagiarism|contentSafety|linkHealth|brandVoice`), riwayat penuh per revisi. Status: `passed|failed|review_required|pending`.
- Backend sebelumnya **tidak pernah membaca** `taut_agent_configs.settings` (toggle/threshold admin tidak berpengaruh) — sekarang dibaca khusus untuk AI Overview.
- Pass revisi akhir Gatekeeper (`final_html, verdict, catatan`) adalah tempat artikel ditulis ulang → titik sambung catatan revisi AI Overview.

## Keputusan (dari user)

1. **Non-blocking**: skor di bawah ambang TIDAK memblokir distribusi (gate:false).
2. **Catatan revisi disambungkan** ke prompt pass revisi akhir Gatekeeper.
3. **Skor hijau untuk artikel buatan AI Tauthub sendiri**, dengan tiga lapis jujur:
   a. Prompt Writer ditambah aturan selaras AI Overview (BLUF 150 kata pertama, entitas spesifik, tanpa fluff; blogger: subjudul berbasis pertanyaan).
   b. Bila artikel `attribution='ai_generated'` dan skor di bawah ambang → pass revisi (dengan catatan) berjalan → skor dievaluasi ulang satu kali pada hasil revisi → skor tersimpan = hasil akhir.
   c. Bila tetap di bawah ambang setelah satu putaran → tersimpan jujur `review_required`.

## Desain indikator (4 kategori, skor 1-10, total 40)

| Kunci | Kategori | Yang dinilai |
|---|---|---|
| `struktur` | Struktur & Pemformatan | BLUF (jawaban utama di 150 kata pertama), heading berbasis pertanyaan (blogger), tabel/list |
| `entitas` | Kepadatan Entitas & Semantik | entitas bernama (tokoh/institusi/lokasi/angka/istilah), signal-to-noise, kedalaman LSI |
| `eeat` | Sinyal E-E-A-T & Orisinalitas | kutipan/rujukan otoritatif, klaim objektif tanpa hiperbola, information gain |
| `niatPengguna` | Penyelarasan Niat Pengguna | pemenuhan PAA, kesimpulan rekomendasi bersyarat |

**Aturan gate (dihitung di Go, bukan LLM):** LULUS = total ≥ 32 DAN tidak ada indikator < 7.
Status: lulus → `passed`; ditolak → `review_required` (non-blocking); error LLM/parse → `review_required` + `error`.

**System prompt** (via `callAgentLLM` + systemOverride, json_mode — pola sama seperti content-safety).
Output LLM HANYA JSON valid:

```json
{"indikator":[{"kunci":"struktur","skor":8,"ringkasan":"..."},{"kunci":"entitas","skor":7,"ringkasan":"..."},{"kunci":"eeat","skor":9,"ringkasan":"..."},{"kunci":"niat_pengguna","skor":6,"ringkasan":"..."}],"catatan_revisi":["..."]}
```

User prompt: judul + tipe media (mainstream tanpa heading = jangan dipenalti) + jumlah kata + isi HTML (truncate `max_chars`, default 8000).
Model memakai config agent **gatekeeper** yang sudah ada (admin bisa mengarahkan ke Deepseek flash di Admin > Taut Agents).

### Bentuk `checks.aiOverview` yang disimpan

```json
{
  "status": "passed|review_required|pending",
  "totalScore": 34, "maxScore": 40,
  "verdict": "lulus|ditolak",
  "indicators": [{"key":"struktur","title":"Struktur & Pemformatan","score":9,"summary":"..."}, ...4],
  "revisionNotes": ["maks 3, hanya saat ditolak"],
  "checkedAt": "2026-...Z",
  "error": "opsional saat gagal"
}
```

### Settings (dibaca dari `taut_agent_configs.settings.aiOverview`)

`{enabled: true, pass_total: 32, pass_min_indicator: 7, max_chars: 8000}` — default di Go; diatur dari Admin > Taut Agents.

## Perubahan backend

### brandflow (jalur live)
1. `taut_engine.go`: `agentConfig` dapat field `Settings map[string]any`; SELECT `getAgentConfig` menambah kolom `settings`.
2. `taut_gatekeeper.go`:
   - Hapus stub plagiarism; tambah `runAIOverviewCheck(...)` + `aiOverviewSettings(...)` + `aiOverviewSystemPrompt`.
   - Integrasi di `runGatekeeper`: aiOverview setelah brandVoice; catatan revisi ditempel ke prompt pass akhir bila ditolak; bila `attribution='ai_generated'` dan ditolak → evaluasi ulang sekali pada `final_html` lalu UPDATE baris audit (checks final tersimpan).
   - Query media menambah `m.media_type`; query versi menambah `attribution`.
3. **Paralelisasi `runGatekeeper`** (4 pekerja) — wajib karena +1 panggilan LLM per media; tiap media independen.
4. `taut_extras.go`: default checks GET audits → `aiOverview: {status:"pending"}`.

### clienttaut (jalur wizard finish)
5. `agent.go`: `agentConfig` + `Settings` dan SELECT `settings`.
6. `gatekeeper.go`: ganti blok plagiarism (Smodin) dengan aiOverview yang sama (kode duplikat sesuai pola paket).

### Writer
7. `taut_writer.go`: aturan selaras AI Overview (BLUF, entitas konkret, tanpa fluff; blogger: subjudul berbasis pertanyaan + rekomendasi bersyarat).

## Perubahan frontend
8. `_qc-panel.svelte`: `Checks` type & `CHECK_CARDS` & `summaryOf` plagiarism → aiOverview; detail expand dengan 4 indikator + catatan revisi; ikon.
9. `help-modal.svelte`: entri `GATE_CHECKS` plagiarism → AI Overview Checker.
10. `admin/taut-agents/+page.svelte`: form settings gatekeeper plagiarism → aiOverview (enabled, pass_total, pass_min_indicator, max_chars).

## Yang tidak berubah
- Distribusi & `canDistribute` (hard gate tetap contentSafety + linkHealth).
- Status audit & struktur tabel; baris histori lama tetap utuh (kunci plagiarism di jsonb tidak dirender).
- Endpoint & alur trigger (audit-all, audits, distribute).

## Perbaikan pasca-rilis (3 Okt)

- **"Scan QC Sekarang" tidak bergerak**: `audit-all` tanpa `mediaIds` menganggap semua media siap = `alreadyAudited` (idempoten), dan `runGatekeeper` hanya memindai artikel `draft/revision` — artikel `ready` tak pernah dipindai ulang sehingga kartu AI Overview tetap kosong. Perbaikan: (a) audit lama tanpa kunci `aiOverview` dianggap basi, (b) dukungan subset `mediaIds` di `runGatekeeper` yang menyertakan artikel `ready`, (c) tombol panel QC kini mengirim `mediaIds` artikel yang dibuka, (d) label tombol menjadi "Scan Ulang Artikel Ini".
- **Cek per artikel sekarang paralel**: safety, link health, dan AI Overview berjalan bersamaan (sebelumnya berurutan).
- **Aturan re-evaluasi diperluas**: `ai_generated` ATAU `ai_edited` (hasil Gatekeeper sendiri tetap dihitung produksi AI Tauthub); editan manual tidak.
- **Panel QC menampilkan timer berjalan** + penjelasan bahwa pemindaian wajar 1-4 menit (4 panggilan LLM per artikel).
- Terverifikasi end-to-end di lokal: audit lama (tanpa kunci) → rescan menghasilkan skor 23/40 (pra-revisi) lalu 31/40 (pasca-revisi, skor tertimpa oleh re-evaluasi).

## Optimasi kecepatan (3 Okt, lanjutan)

Timing nyata per tahap (log `[gatekeeper] audit selesai`):
| Tahap | mimo-v2.5-pro | deepseek-v4-flash |
|---|---|---|
| Batch cek paralel (safety+link+AI Overview) | 82 dtk | 15,6 dtk |
| Pass revisi (tulis ulang artikel) | 62 dtk | 38,8 dtk |
| Re-evaluasi AI Overview pasca-revisi | 86 dtk | 16,5 dtk |
| **Total** | **230 dtk** | **71 dtk** |

Perubahan kode:
- Pass revisi akhir hanya dijalankan bila AI Overview MENOLAK (sebelumnya juga
  jalan saat safety/link `review_required`, padahal menulis ulang tidak
  menyelesaikan link "perlu review" — keputusan manusia). Tanpa revisi, status
  artikel diturunkan deterministik: failed -> blocked, review_required -> revision.
- Versi artikel baru hanya ditulis bila isi benar-benar berubah (tidak ada
  churn versi tiap scan).
- Probe tautan dijalankan paralel (maks 6) — sebelumnya serial 8 dtk/URL.
- Tiga cek independen sudah paralel; log timing per tahap ditambahkan supaya
  penyebab lambat selalu bisa dibuktikan.

Lever terbesar adalah kecepatan MODEL agent gatekeeper (satu field di
Admin > Taut Agents) — disarankan model flash-class.

## Status pelaksanaan
- [x] docs/plan-ai-overview-qc.md
- [x] internal/ai/overview.go (prompt + parsing + aturan gate, dipakai bersama) + unit test
- [x] brandflow: checks.aiOverview + settings + sambung catatan revisi + re-evaluasi pasca-revisi (ai_generated) + paralelisasi 4 pekerja
- [x] taut_extras.go: default checks GET audits → aiOverview
- [x] clienttaut: ganti plagiarism (Smodin) dengan aiOverview + alur yang sama
- [x] Writer: aturan selaras AI Overview (BLUF, entitas konkret, subjudul pertanyaan untuk blogger)
- [x] Frontend: _qc-panel (kartu + ringkasan + detail 4 indikator + catatan revisi), help-modal, form admin gatekeeper
- [x] verifikasi deploy staging (hash aset web berubah, healthz OK, route marker 401)
