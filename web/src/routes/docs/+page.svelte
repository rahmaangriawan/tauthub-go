<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Port app/docs/page.tsx + docs-content.tsx — dokumentasi publik API Web Custom
	// (kontrak push artikel, verifikasi HMAC, contoh implementasi).
	import { Copy, Check } from '@lucide/svelte';

	let copied = $state('');
	async function copy(label: string, code: string) {
		try {
			await navigator.clipboard.writeText(code);
			copied = label;
			window.setTimeout(() => (copied = ''), 1500);
		} catch {
			/* clipboard tidak tersedia */
		}
	}

	const PING_PAYLOAD = `{
  "event": "content.ping",
  "event_id": "4f8c1b2e-3a7d-4f9e-9b1c-2d5e6f7a8b9c",
  "timestamp": "2026-08-05T09:00:00Z"
}`;

	const DELIVERED_PAYLOAD = `{
  "event": "content.delivered",
  "event_id": "7c2f9c1e-5b4a-4d3e-8f2a-1b6c7d8e9f0a",
  "timestamp": "2026-08-05T09:00:00Z",
  "document": {
    "id": "uuid-dokumen-tauthub",
    "title": "Cara Memulai UMKM Digital",
    "slug": "cara-memulai-umkm-digital",
    "language": "id-ID",
    "content_markdown": "# Cara Memulai UMKM Digital\\n\\nKonten markdown lengkap artikel...",
    "content_html": "<h2>Cara Memulai UMKM Digital</h2>\\n<p>Konten HTML lengkap artikel...</p>",
    "excerpt": "Ringkasan singkat artikel",
    "primary_keyword": "umkm digital",
    "secondary_keywords": ["modal kecil", "platform"],
    "tags": ["umkm", "digital"],
    "categories": ["Bisnis"],
    "author": null,
    "publish_status": "draft",
    "generated_at": "2026-08-05T09:00:00Z"
  }
}`;

	const SIGNATURE_RULE = `signature = "sha256=" + hex( HMAC-SHA256( secret, timestamp + "." + eventId + "." + rawBody ) )`;

	const EXPRESS_EXAMPLE = `const express = require("express");
const crypto = require("crypto");
const app = express();

// Secret dari owner Tauthub — simpan di env, jangan di repo
const SECRET = process.env.TULISIO_SECRET;

app.post("/api/tauthub/content", express.json({ verify: (req, res, buf) => { req.rawBody = buf; } }), (req, res) => {
  const signature = req.headers["x-tulisio-signature"] || "";
  const timestamp = req.headers["x-tulisio-timestamp"];
  const eventId = req.headers["x-tulisio-event-id"];
  const raw = req.rawBody.toString("utf8");

  // 1. Cek timestamp (maks 5 menit drift)
  if (Math.abs(Date.now() - Date.parse(timestamp)) > 5 * 60 * 1000) {
    return res.status(401).json({ ok: false });
  }

  // 2. Verifikasi HMAC (timing-safe)
  const mac = crypto.createHmac("sha256", SECRET).update(\`\${timestamp}.\${eventId}.\${raw}\`).digest("hex");
  const expected = Buffer.from("sha256=" + mac);
  const received = Buffer.from(signature);
  if (expected.length !== received.length || !crypto.timingSafeEqual(expected, received)) {
    return res.status(401).json({ ok: false });
  }

  // 3. Idempotency — jangan proses event yang sama dua kali
  if (alreadyProcessed(eventId)) return res.json({ ok: true });

  const { event, document } = req.body;
  if (event === "content.ping") return res.json({ ok: true }); // uji koneksi dari UI

  // 4. Simpan artikel (draft) di database web custom Anda
  saveArticle(document);
  markProcessed(eventId);
  res.json({ ok: true, remote_id: "post-" + document.id });
});`;

	const LARAVEL_EXAMPLE = `// routes/api.php
use Illuminate\\Support\\Facades\\Route;
use Illuminate\\Http\\Request;

Route::post('/tauthub/content', function (Request $request) {
    $secret = env('TULISIO_SECRET');
    $signature = $request->header('x-tulisio-signature');
    $timestamp = $request->header('x-tulisio-timestamp');
    $eventId = $request->header('x-tulisio-event-id');
    $raw = $request->getContent();

    // 1. Cek timestamp (maks 5 menit drift)
    if (abs(time() - strtotime($timestamp)) > 300) {
        return response()->json(['ok' => false], 401);
    }

    // 2. Verifikasi HMAC (hash_equals = timing-safe)
    $mac = 'sha256=' . hash_hmac('sha256', "{$timestamp}.{$eventId}.{$raw}", $secret);
    if (!hash_equals($mac, $signature)) {
        return response()->json(['ok' => false], 401);
    }

    // 3. Idempotency
    if (alreadyProcessed($eventId)) {
        return response()->json(['ok' => true]);
    }

    $data = json_decode($raw, true);
    if (($data['event'] ?? null) === 'content.ping') {
        return response()->json(['ok' => true]); // uji koneksi
    }

    // 4. Simpan artikel (draft) di database web custom Anda
    saveArticle($data['document']);
    markProcessed($eventId);
    return response()->json(['ok' => true, 'remote_id' => 'post-' . $data['document']['id']]);
});`;

	const NEXTJS_EXAMPLE = `// app/api/tauthub/content/route.ts
import crypto from "node:crypto";
import { NextResponse } from "next/server";

const SECRET = process.env.TULISIO_SECRET ?? "";

export async function POST(request: Request) {
  const raw = await request.text();
  const signature = request.headers.get("x-tulisio-signature") ?? "";
  const timestamp = request.headers.get("x-tulisio-timestamp") ?? "";
  const eventId = request.headers.get("x-tulisio-event-id") ?? "";

  // 1. Cek timestamp (maks 5 menit drift)
  if (Math.abs(Date.now() - Date.parse(timestamp)) > 5 * 60 * 1000) {
    return NextResponse.json({ ok: false }, { status: 401 });
  }

  // 2. Verifikasi HMAC (timing-safe)
  const mac = crypto.createHmac("sha256", SECRET).update(\`\${timestamp}.\${eventId}.\${raw}\`).digest("hex");
  const expected = Buffer.from("sha256=" + mac);
  const received = Buffer.from(signature);
  if (expected.length !== received.length || !crypto.timingSafeEqual(expected, received)) {
    return NextResponse.json({ ok: false }, { status: 401 });
  }

  // 3. Idempotency — cek eventId sudah pernah diproses
  if (await alreadyProcessed(eventId)) {
    return NextResponse.json({ ok: true });
  }

  const { event, document } = JSON.parse(raw) as { event: string; document?: Record<string, unknown> };
  if (event === "content.ping") {
    return NextResponse.json({ ok: true }); // uji koneksi dari UI
  }

  // 4. Simpan artikel (draft) di database web custom Anda
  await saveArticle(document);
  await markProcessed(eventId);
  return NextResponse.json({ ok: true, remote_id: "post-" + document?.id });
}`;

	const EXAMPLES = $derived([
		{ key: 'node', label: 'Node.js / Express', code: EXPRESS_EXAMPLE },
		{ key: 'php', label: 'PHP / Laravel', code: LARAVEL_EXAMPLE },
		{ key: 'next', label: 'Next.js Route', code: NEXTJS_EXAMPLE }
	]);
	let tab = $state<'node' | 'php' | 'next'>('node');
	const activeExample = $derived(EXAMPLES.find((e) => e.key === tab) ?? EXAMPLES[0]);

	const HEADERS = $derived([
		['POST {base_url}', tr('Method dan URL yang Anda daftarkan di Integrations')],
		['Content-Type: application/json', 'Body selalu JSON'],
		['x-tulisio-event', 'content.delivered | content.ping'],
		['x-tulisio-event-id', tr('UUID unik per event — untuk idempotency')],
		['x-tulisio-timestamp', 'ISO-8601 waktu pengiriman'],
		['x-tulisio-signature', 'sha256=hex — HMAC-SHA256, lihat bagian 5']
	]);
	const RESPONSES = $derived([
		['200', '{ "ok": true }', tr('Berhasil — Tauthub menandai completed')],
		['200', '{ "ok": true, "remote_id": "post-123" }', tr('Berhasil + id artikel di sistem Anda')],
		['4xx', '{ "ok": false }', 'Request ditolak — tidak di-retry, cek error_message'],
		['5xx / timeout', '—', 'Akan di-retry otomatis (lihat bagian 6)']
	]);
	const VERIFY_STEPS = $derived([
		['1', 'Baca raw body mentah (sebelum parse JSON)'],
		['2', tr('Cek x-tulisio-timestamp — tolak bila drift lebih dari 5 menit')],
		['3', 'Hitung HMAC dengan rumus di atas lalu bandingkan dengan x-tulisio-signature memakai timing-safe compare'],
		['4', 'Cek x-tulisio-event-id belum pernah diproses (idempotency)']
	]);
	const RETRY = $derived([
		['200 + ok:true', 'Sukses, tidak ada pengiriman ulang'],
		['4xx', tr('Gagal permanen — tidak di-retry, status failed di platform')],
		['5xx / timeout / network', 'Retry hingga 3x dengan jeda 1 menit, 5 menit, 15 menit'],
		['Retry', tr('Mengirim event_id yang SAMA — Anda wajib dedupe agar tidak ada artikel ganda')]
	]);
	const ERRORS = $derived([
		['200', tr('Berhasil — respons harus { ok: true }')],
		['401', 'Signature/timestamp tidak valid — periksa secret dan waktu server'],
		['422', 'Payload tidak sesuai kontrak'],
		['500', tr('Kesalahan internal Tauthub — akan di-retry')]
	]);
</script>

<svelte:head>
	<title>Dokumentasi API Web Custom — Tauthub</title>
	<meta
		name="description"
		content="Kontrak API push Tauthub untuk integrasi web custom: endpoint, payload, verifikasi HMAC, dan contoh implementasi."
	/>
</svelte:head>

<!-- Layout landing: header + footer sederhana (parity landing-layout app lama). -->
<div class="min-h-screen bg-white text-gray-900">
	<header class="border-b border-gray-200">
		<div class="mx-auto flex max-w-3xl items-center justify-between px-4 py-4 sm:px-6">
			<a href="/" class="text-lg font-extrabold tracking-tight">
				tauthub<span style="color: var(--primary);">.</span>
			</a>
			<a class="text-sm font-medium" style="color: var(--primary);" href="/register/mitra">{tr("Daftar Mitra")}</a>
		</div>
	</header>

	<main class="mx-auto max-w-3xl px-4 pb-24 pt-10 sm:px-6">
		<div class="text-xs font-semibold uppercase tracking-widest" style="color: var(--primary);">Dokumentasi Integrasi</div>
		<h1 class="mt-2 text-3xl font-bold text-gray-900">{tr("Web Custom API — Push Artikel ke Website Anda")}</h1>
		<p class="mt-3 text-[15px] leading-relaxed text-gray-600">{tr("Tauthub mengirim artikel hasil generate ke website custom Anda melalui endpoint yang Anda buat. Anda hanya perlu membuat satu endpoint")}<code class="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-[13px] text-gray-800">POST</code>{tr("di aplikasi Anda, lalu hubungkan di halaman")}<strong>Integrations</strong> platform Tauthub.
		</p>

		<h2 class="mt-12 text-xl font-semibold text-gray-900">1. Konsep</h2>
		<p class="mt-3 text-[15px] leading-relaxed text-gray-600">
			Ketika pengguna menekan <strong>{tr("Kirim ke Web Custom")}</strong>, Tauthub mengirim <code>content.delivered</code>{tr("ke endpoint Anda. Artikel selalu dikirim sebagai")}<strong>draft</strong>{tr("— keputusan publish final ada di website Anda. Setiap request ditandatangani dengan HMAC-SHA256 agar Anda yakin bahwa request benar-benar berasal dari Tauthub.")}</p>

		<h2 class="mt-12 text-xl font-semibold text-gray-900">2. Persiapan</h2>
		<p class="mt-3 text-[15px] leading-relaxed text-gray-600">{tr("Pemilik akun Tauthub memberi Anda dua hal:")}<strong>Endpoint URL</strong> (contoh
			<code class="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-[13px] text-gray-800">https://web-anda.example/api/tauthub/content</code>{tr(") dan")}<strong>Shared Secret</strong>{tr(". Simpan secret di environment variable")}<code class="rounded bg-gray-100 px-1.5 py-0.5 font-mono text-[13px] text-gray-800">TULISIO_SECRET</code>{tr("atau field konfigurasi aplikasi Anda. Jangan pernah menulis secret di kode atau commit ke repository publik.")}</p>

		<h2 class="mt-12 text-xl font-semibold text-gray-900">{tr("3. Endpoint yang Harus Dibuat")}</h2>
		<div class="my-4 overflow-x-auto rounded-xl border border-gray-200">
			<table class="w-full border-collapse text-left text-sm">
				<thead>
					<tr class="bg-gray-50">
						<th class="border-b border-gray-200 px-4 py-2 font-semibold text-gray-700">Header</th>
						<th class="border-b border-gray-200 px-4 py-2 font-semibold text-gray-700">Keterangan</th>
					</tr>
				</thead>
				<tbody>
					{#each HEADERS as row (row[0])}
						<tr class="border-b border-gray-100 last:border-0">
							<td class="px-4 py-2 font-mono text-[13px] text-gray-800">{row[0]}</td>
							<td class="px-4 py-2 text-gray-600">{row[1]}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
		<p class="mt-3 text-[15px] leading-relaxed text-gray-600">{tr("Respons yang diharapkan:")}</p>
		<div class="my-4 overflow-x-auto rounded-xl border border-gray-200">
			<table class="w-full border-collapse text-left text-sm">
				<thead>
					<tr class="bg-gray-50">
						<th class="border-b border-gray-200 px-4 py-2 font-semibold text-gray-700">Status</th>
						<th class="border-b border-gray-200 px-4 py-2 font-semibold text-gray-700">Body</th>
						<th class="border-b border-gray-200 px-4 py-2 font-semibold text-gray-700">Arti</th>
					</tr>
				</thead>
				<tbody>
					{#each RESPONSES as row (row[0] + row[1])}
						<tr class="border-b border-gray-100 last:border-0">
							<td class="px-4 py-2 font-mono text-[13px] text-gray-800">{row[0]}</td>
							<td class="px-4 py-2 font-mono text-[13px] text-gray-600">{row[1]}</td>
							<td class="px-4 py-2 text-gray-600">{row[2]}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<h2 class="mt-12 text-xl font-semibold text-gray-900">4. Payload</h2>
		<h3 class="mt-8 text-base font-semibold text-gray-900">content.ping (uji koneksi)</h3>
		<p class="mt-3 text-[15px] leading-relaxed text-gray-600">{tr("Dikirim saat tombol")}<strong>Test</strong>{tr("ditekan di halaman Integrations. Balas dengan ok untuk lulus uji.")}</p>
		<div class="my-4 overflow-hidden rounded-xl border border-gray-200 bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-700/60 px-4 py-2">
				<span class="font-mono text-xs text-gray-400">json</span>
				<button class="rounded-md bg-gray-700/60 px-2 py-1 font-mono text-xs text-gray-200" onclick={() => copy('ping', PING_PAYLOAD)}>
					{copied === 'ping' ? 'Tersalin' : 'Salin'}
				</button>
			</div>
			<pre class="overflow-x-auto p-4 font-mono text-[13px] leading-relaxed text-gray-100">{PING_PAYLOAD}</pre>
		</div>

		<h3 class="mt-8 text-base font-semibold text-gray-900">content.delivered (artikel)</h3>
		<div class="my-4 overflow-hidden rounded-xl border border-gray-200 bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-700/60 px-4 py-2">
				<span class="font-mono text-xs text-gray-400">json</span>
				<button class="rounded-md bg-gray-700/60 px-2 py-1 font-mono text-xs text-gray-200" onclick={() => copy('delivered', DELIVERED_PAYLOAD)}>
					{copied === 'delivered' ? 'Tersalin' : 'Salin'}
				</button>
			</div>
			<pre class="overflow-x-auto p-4 font-mono text-[13px] leading-relaxed text-gray-100">{DELIVERED_PAYLOAD}</pre>
		</div>

		<h2 class="mt-12 text-xl font-semibold text-gray-900">{tr("5. Verifikasi Signature (Wajib)")}</h2>
		<p class="mt-3 text-[15px] leading-relaxed text-gray-600">Rumus HMAC:</p>
		<div class="my-4 overflow-hidden rounded-xl border border-gray-200 bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-700/60 px-4 py-2">
				<span class="font-mono text-xs text-gray-400">pseudocode</span>
				<button class="rounded-md bg-gray-700/60 px-2 py-1 font-mono text-xs text-gray-200" onclick={() => copy('sig', SIGNATURE_RULE)}>
					{copied === 'sig' ? 'Tersalin' : 'Salin'}
				</button>
			</div>
			<pre class="overflow-x-auto p-4 font-mono text-[13px] leading-relaxed text-gray-100">{SIGNATURE_RULE}</pre>
		</div>
		<p class="mt-3 text-[15px] leading-relaxed text-gray-600">Langkah verifikasi:</p>
		<div class="my-4 overflow-x-auto rounded-xl border border-gray-200">
			<table class="w-full border-collapse text-left text-sm">
				<thead>
					<tr class="bg-gray-50">
						<th class="border-b border-gray-200 px-4 py-2 font-semibold text-gray-700">#</th>
						<th class="border-b border-gray-200 px-4 py-2 font-semibold text-gray-700">Langkah</th>
					</tr>
				</thead>
				<tbody>
					{#each VERIFY_STEPS as row (row[0])}
						<tr class="border-b border-gray-100 last:border-0">
							<td class="px-4 py-2 font-mono text-[13px] text-gray-800">{row[0]}</td>
							<td class="px-4 py-2 text-gray-600">{row[1]}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<h2 class="mt-12 text-xl font-semibold text-gray-900">6. Contoh Implementasi</h2>
		<div class="mt-4 flex flex-wrap gap-2">
			{#each EXAMPLES as example (example.key)}
				<button
					class="rounded-lg px-3 py-1.5 text-sm font-medium transition-colors"
					style="background: {tab === example.key ? '#1f2937' : '#f3f4f6'}; color: {tab === example.key ? '#fff' : '#4b5563'};"
					onclick={() => (tab = example.key as 'node' | 'php' | 'next')}
				>{example.label}</button>
			{/each}
		</div>
		<div class="my-4 overflow-hidden rounded-xl border border-gray-200 bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-700/60 px-4 py-2">
				<span class="font-mono text-xs text-gray-400">{activeExample.label.toLowerCase()}</span>
				<button class="rounded-md bg-gray-700/60 px-2 py-1 font-mono text-xs text-gray-200" onclick={() => copy('code', activeExample.code)}>
					{#if copied === 'code'}<Check size={12} class="inline" /> Tersalin{:else}<Copy size={12} class="inline" /> Salin{/if}
				</button>
			</div>
			<pre class="overflow-x-auto p-4 font-mono text-[13px] leading-relaxed text-gray-100">{activeExample.code}</pre>
		</div>

		<h2 class="mt-12 text-xl font-semibold text-gray-900">{tr("7. Retry dan Idempotency")}</h2>
		<div class="my-4 overflow-x-auto rounded-xl border border-gray-200">
			<table class="w-full border-collapse text-left text-sm">
				<thead>
					<tr class="bg-gray-50">
						<th class="border-b border-gray-200 px-4 py-2 font-semibold text-gray-700">Situasi</th>
						<th class="border-b border-gray-200 px-4 py-2 font-semibold text-gray-700">Perilaku Tauthub</th>
					</tr>
				</thead>
				<tbody>
					{#each RETRY as row (row[0])}
						<tr class="border-b border-gray-100 last:border-0">
							<td class="px-4 py-2 font-mono text-[13px] text-gray-800">{row[0]}</td>
							<td class="px-4 py-2 text-gray-600">{row[1]}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
		<p class="mt-3 text-[15px] leading-relaxed text-gray-600">
			Karena retry memakai <code>event_id</code>{tr("yang sama, simpan daftar event_id yang sudah diproses (misalnya di tabel database) dan langsung balas ok bila event_id sudah pernah diterima.")}</p>

		<h2 class="mt-12 text-xl font-semibold text-gray-900">{tr("8. Kode Status Kesalahan")}</h2>
		<div class="my-4 overflow-x-auto rounded-xl border border-gray-200">
			<table class="w-full border-collapse text-left text-sm">
				<thead>
					<tr class="bg-gray-50">
						<th class="border-b border-gray-200 px-4 py-2 font-semibold text-gray-700">Kode</th>
						<th class="border-b border-gray-200 px-4 py-2 font-semibold text-gray-700">Arti</th>
					</tr>
				</thead>
				<tbody>
					{#each ERRORS as row (row[0])}
						<tr class="border-b border-gray-100 last:border-0">
							<td class="px-4 py-2 font-mono text-[13px] text-gray-800">{row[0]}</td>
							<td class="px-4 py-2 text-gray-600">{row[1]}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<h2 class="mt-12 text-xl font-semibold text-gray-900">9. FAQ</h2>
		<h3 class="mt-8 text-base font-semibold text-gray-900">{tr("Apakah artikel bisa langsung publish?")}</h3>
		<p class="mt-3 text-[15px] leading-relaxed text-gray-600">{tr("Tidak di versi ini. Tauthub selalu mengirim draft; Anda yang memutuskan kapan mempublikasikannya.")}</p>
		<h3 class="mt-8 text-base font-semibold text-gray-900">Secret saya bocor, bagaimana?</h3>
		<p class="mt-3 text-[15px] leading-relaxed text-gray-600">{tr("Hubungi pemilik akun Tauthub untuk mencabut koneksi dan membuat secret baru di halaman Integrations.")}</p>
		<h3 class="mt-8 text-base font-semibold text-gray-900">Bagaimana cara menguji koneksi?</h3>
		<p class="mt-3 text-[15px] leading-relaxed text-gray-600">
			Di halaman Integrations, tekan tombol <strong>Test</strong>{tr("pada koneksi Web Custom Anda. Tauthub mengirim")}<code>content.ping</code>{tr("dan menampilkan hasilnya.")}</p>
		<h3 class="mt-8 text-base font-semibold text-gray-900">{tr("Apakah Tauthub membaca data dari website saya?")}</h3>
		<p class="mt-3 text-[15px] leading-relaxed text-gray-600">{tr("Tidak. Arah integrasi satu arah: Tauthub hanya mengirim artikel ke endpoint Anda.")}</p>
	</main>
</div>
