#!/usr/bin/env node
// verify-bundle.mjs — pastikan string tertentu benar-benar ada (atau sudah
// hilang) di bundle web yang sedang dilayani sebuah environment.
//
// Pakai:
//   node deploy/verify-bundle.mjs <base-url> <string> [string ...]
//
//   string tanpa awalan  = WAJIB ada
//   string berawalan "!" = WAJIB tidak ada
//
// Contoh (verifikasi deploy):
//   node deploy/verify-bundle.mjs https://staging.tauthub.com \
//     "Rencana Penempatan Gambar" "!Generate Gambar per 3 Paragraf"
//
// Tidak ada domain yang dipatok: base-url datang dari argumen, dan berkas
// entry ditemukan sendiri dari HTML halaman (namanya mengandung hash build,
// jadi berubah tiap deploy — mematoknya berarti skrip basi tanpa suara).
//
// Env: VERIFY_PAGE (default "/login"), VERIFY_MAX_FILES (default 400)
// Keluar 0 bila semua harapan terpenuhi, 1 bila ada yang tak terpenuhi,
// 2 bila pemindaian tidak bisa dimulai.

import { setTimeout as sleep } from "node:timers/promises";

const DEFAULT_PAGE = process.env.VERIFY_PAGE || "/login";
const MAX_FILES = Number(process.env.VERIFY_MAX_FILES || 400);
const FETCH_TIMEOUT_MS = 20000;

const [rawBase, ...rawExpectations] = process.argv.slice(2);
if (!rawBase || rawExpectations.length === 0) {
  console.error(
    'Pakai: node deploy/verify-bundle.mjs <base-url> <string> ["!"string ...]'
  );
  process.exit(2);
}

const base = rawBase.replace(/\/+$/, "");
const pageUrl = new URL(DEFAULT_PAGE, base).href;

const expectations = rawExpectations.map((raw) => ({
  mustExist: !raw.startsWith("!"),
  text: raw.startsWith("!") ? raw.slice(1) : raw
}));

const label = (e) => `${e.mustExist ? "ADA" : "TIDAK ADA"}: "${e.text}"`;

async function get(url) {
  const res = await fetch(url, {
    signal: AbortSignal.timeout(FETCH_TIMEOUT_MS),
    redirect: "follow"
  });
  return res;
}

// Kumpulkan berkas entry yang direferensikan halaman (HTML/header "link").
// Nama chunk SvelteKit selalu berakhiran ".js" di bawah /_app/immutable/;
// sisanya (css, gambar) diabaikan.
function collectRefs(text, fromUrl) {
  const out = new Set();
  for (const m of text.matchAll(/["'(]?((?:\.{0,2}\/)?_app\/immutable\/[^"'()\s;]+?\.js)/g)) {
    try {
      out.add(new URL(m[1], fromUrl).pathname);
    } catch {
      // referensi aneh — lewati saja
    }
  }
  return [...out];
}

// Import statis antar-chunk ditulis relatif (mis. import"./D73UTUvB.js") tanpa
// menyebut "_app/immutable", jadi dicocokkan terpisah lalu disaring setelah
// diresolusi. Tanpa jalur ini penelusuran hanya bergantung pada header "link".
function collectRelativeJs(text, fromUrl, prefix) {
  const out = new Set();
  for (const m of text.matchAll(/["']((?:\.{1,2}\/|\/)[^"']*?\.js)["']/g)) {
    try {
      const path = new URL(m[1], fromUrl).pathname;
      if (path.startsWith(prefix)) out.add(path);
    } catch {
      // specifier tidak valid — lewati
    }
  }
  return [...out];
}

let pageHtml;
let pageHeaders;
try {
  const res = await get(pageUrl);
  if (!res.ok) {
    console.error(`Halaman ${pageUrl} menjawab HTTP ${res.status} — pemindaian dibatalkan.`);
    process.exit(2);
  }
  pageHtml = await res.text();
  pageHeaders = res.headers;
} catch (err) {
  console.error(`Tidak bisa mengambil ${pageUrl}: ${err.message} — pemindaian dibatalkan.`);
  process.exit(2);
}

// Sumber referensi: badan HTML + header "link" (modulepreload) bila ada.
const entries = new Set(collectRefs(pageHtml, pageUrl));
const linkHeader = pageHeaders?.get?.("link");
if (linkHeader) for (const p of collectRefs(linkHeader, pageUrl)) entries.add(p);

if (entries.size === 0) {
  console.error(
    `Tidak menemukan berkas entry di ${pageUrl}. Halaman mungkin bukan aplikasi ` +
      `SvelteKit, sedang dilayani server dev (vite menyajikan modul mentah, bukan ` +
      `bundle /_app/immutable), di balik challenge, atau build-nya rusak — keluar ` +
      `agar "tidak ada string" tidak pernah berarti ambigu.`
  );
  process.exit(2);
}

const started = entries.size;
const queue = [...entries];
const seen = new Set();
const matches = new Map(expectations.map((e) => [e.text, []]));
let scanned = 0;
let failed = 0;

while (queue.length && scanned < MAX_FILES) {
  const path = queue.shift();
  if (seen.has(path)) continue;
  seen.add(path);

  let body;
  try {
    const res = await get(base + path);
    if (!res.ok) {
      failed++;
      continue;
    }
    body = await res.text();
  } catch {
    failed++;
    continue;
  }
  scanned++;

  for (const e of expectations) if (body.includes(e.text)) matches.get(e.text).push(path);

  const dir = path.slice(0, path.lastIndexOf("/") + 1);
  for (const ref of collectRelativeJs(body, base + dir, "/_app/immutable/")) {
    if (!seen.has(ref)) queue.push(ref);
  }

  // Beri napas sesekali supaya tidak membanjiri origin.
  if (scanned % 25 === 0) await sleep(50);
}

console.log(`Base        : ${base}`);
console.log(`Halaman     : ${pageUrl}`);
console.log(`Entry awal  : ${started}, dipindai: ${scanned}, gagal: ${failed}`);
if (queue.length && scanned >= MAX_FILES) {
  console.log(`Catatan     : berhenti di batas VERIFY_MAX_FILES=${MAX_FILES} (masih ada sisa antrean).`);
}
console.log("");

let ok = true;
for (const e of expectations) {
  const found = matches.get(e.text);
  const satisfied = e.mustExist ? found.length > 0 : found.length === 0;
  if (!satisfied) ok = false;
  console.log(`${satisfied ? "OK  " : "GAGAL"}  ${label(e)}`);
  if (found.length > 0) {
    for (const f of found.slice(0, 5)) console.log(`         ada di ${f}`);
    if (found.length > 5) console.log(`         …dan ${found.length - 5} berkas lain`);
  }
}

process.exit(ok ? 0 : 1);
