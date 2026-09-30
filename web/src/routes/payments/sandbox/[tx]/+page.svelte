<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Port app/payments/sandbox/[tx]/page.tsx + simulate.tsx — halaman checkout
	// gateway sandbox lokal. Simulasi mengirim webhook bertanda tangan HMAC ke
	// /v1/payments/webhooks/sandbox, jalur kode yang sama dengan callback gateway
	// asli (verifikasi signature → applyWebhook → kredit dompet TC).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiRequest } from '$lib/auth';

	const SANDBOX_SECRET = 'tauthub-sandbox-secret';
	const txId = $derived(page.params.tx as string);

	let loading = $state(true);
	let tx = $state<any>(null);
	let busy = $state(false);
	let result = $state<{ ok: boolean; message: string } | null>(null);

	const idr = (n: number) =>
		new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0, roundingMode: 'trunc' }).format(n);

	async function hmacSha256Hex(message: string, secret: string): Promise<string> {
		const encoder = new TextEncoder();
		const key = await crypto.subtle.importKey('raw', encoder.encode(secret), { name: 'HMAC', hash: 'SHA-256' }, false, ['sign']);
		const signature = await crypto.subtle.sign('HMAC', key, encoder.encode(message));
		return [...new Uint8Array(signature)].map((b) => b.toString(16).padStart(2, '0')).join('');
	}

	onMount(() => {
		void (async () => {
			const res = await apiRequest(`/v1/payments/status/${encodeURIComponent(txId)}`);
			if (res.ok) tx = await res.json();
			loading = false;
		})();
	});

	async function simulate() {
		busy = true;
		result = null;
		try {
			const body = JSON.stringify({ order_id: txId, status: 'PAID' });
			const signature = await hmacSha256Hex(body, SANDBOX_SECRET);
			const res = await fetch('/v1/payments/webhooks/sandbox', {
				method: 'POST',
				headers: { 'content-type': 'application/json', 'x-sandbox-signature': signature },
				body
			});
			const data = (await res.json().catch(() => null)) as { ok?: boolean } | null;
			if (!res.ok || data?.ok !== true) {
				throw new Error(`Webhook ditolak (HTTP ${res.status}).`);
			}
			result = { ok: true, message: 'Pembayaran diterima — Tauthub Credit dikreditkan ke saldo brand.' };
			window.setTimeout(() => {
				window.location.replace(`/payments/status?transactionId=${encodeURIComponent(txId)}`);
			}, 2500);
		} catch (reason) {
			result = { ok: false, message: reason instanceof Error ? reason.message : 'Simulasi gagal.' };
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Gateway Sandbox Tauthub</title>
</svelte:head>

<main class="grid min-h-screen place-items-center bg-[#0d1424] p-6 text-[#eef2f9]">
	<div class="w-full max-w-[420px] rounded-2xl border border-white/10 bg-[#141d33] p-7">
		<div class="text-[13px] font-semibold uppercase tracking-[.08em] text-[#9aa7c0]">Gateway Sandbox Tauthub</div>
		<h1 class="mt-2 mb-1 text-xl">Simulasi pembayaran</h1>
		<p class="text-[13px] text-[#9aa7c0]">{tr("Halaman ini hanya tersedia untuk pengujian lokal. Klik tombol di bawah untuk mengirim callback")}<code class="rounded bg-white/10 px-1.5 py-0.5">PAID</code> bertanda tangan HMAC ke webhook Tauthub —
			persis seperti callback gateway asli.
		</p>

		{#if loading}
			<p class="mt-5 text-sm text-[#9aa7c0]">{tr("Memuat transaksi…")}</p>
		{:else if !tx}
			<p class="mt-5 text-sm text-[#f87171]">Transaksi sandbox tidak ditemukan.</p>
		{:else}
			<dl class="my-5 grid gap-2.5 text-sm">
				<div class="flex justify-between gap-4">
					<dt class="text-[#9aa7c0]">Transaksi</dt>
					<dd class="font-semibold tabular-nums">{tx.reference ?? `PAY-${txId.slice(0, 8).toUpperCase()}`}</dd>
				</div>
				<div class="flex justify-between gap-4">
					<dt class="text-[#9aa7c0]">{tr("Jumlah")}</dt>
					<dd>{idr(Math.round(Number(tx.amount ?? 0)))}</dd>
				</div>
				<div class="flex justify-between gap-4">
					<dt class="text-[#9aa7c0]">{tr("Status saat ini")}</dt>
					<dd class="font-semibold" style="color: {tx.status === 'paid' ? '#4ade80' : '#fbbf24'};">{tx.status}</dd>
				</div>
			</dl>

			<button
				type="button"
				class="w-full rounded-[10px] px-4 py-3 text-sm font-semibold text-white"
				style="background: {tx.status === 'paid' ? '#1f2b45' : '#3b6ef5'}; cursor: {tx.status === 'paid' ? 'default' : 'pointer'};"
				disabled={busy || tx.status === 'paid'}
				onclick={simulate}
			>
				{busy ? 'Mengirim webhook…' : tx.status === 'paid' ? tr('Pembayaran sudah lunas') : 'Simulasikan pembayaran berhasil'}
			</button>

			{#if result}
				<p
					class="mt-3.5 rounded-[10px] px-3 py-2.5 text-[13px]"
					style="background: {result.ok ? 'rgba(74,222,128,.1)' : 'rgba(248,113,113,.1)'}; color: {result.ok ? '#4ade80' : '#f87171'};"
				>
					{result.message}
				</p>
			{/if}

			<p class="mt-3.5 text-xs text-[#6b7794]">{tr("Kembali ke dashboard klien:")}<a class="text-[#8fb0ff]" href="/klien">klien</a> ·
				<a class="text-[#8fb0ff]" href="/admin/credits">admin credits</a>
			</p>
		{/if}
	</div>
</main>
