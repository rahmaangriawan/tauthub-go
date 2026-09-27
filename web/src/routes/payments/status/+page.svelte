<script lang="ts">
	// Port app/payments/status/page.tsx — halaman polling status pembayaran publik
	// setelah redirect dari gateway. Tidak butuh login; hanya status + info ringkas.
	// Return URL gateway membawa transactionId internal (UUID) tetapi provider
	// bisa menempelkan id-nya sendiri dengan join `?`/`&`, jadi potong di query
	// pertama (parity app lama).
	import { onMount } from 'svelte';
	import { page } from '$app/state';

	type TxStatus = {
		status: string;
		amount: number;
		brand_name: string | null;
		brand_id: string | null;
		paid_at: string | null;
		reference: string | null;
	};

	const STATUS_LABEL: Record<string, { label: string; color: string; icon: string }> = {
		paid: { label: 'Pembayaran berhasil', color: '#16a34a', icon: '✓' },
		pending: { label: 'Menunggu pembayaran', color: '#d97706', icon: '⏳' },
		processing: { label: 'Memproses pembayaran', color: '#d97706', icon: '⏳' },
		failed: { label: 'Pembayaran gagal', color: '#dc2626', icon: '✕' },
		expired: { label: 'Pembayaran kedaluwarsa', color: '#6b7280', icon: '!' },
		cancelled: { label: 'Pembayaran dibatalkan', color: '#6b7280', icon: '!' },
		refunded: { label: 'Pembayaran dikembalikan', color: '#6b7280', icon: '↩' }
	};
	const FINAL = ['paid', 'failed', 'expired', 'cancelled', 'refunded'];

	const rawTxId = $derived(
		page.url.searchParams.get('transactionId') ?? page.url.searchParams.get('transaction_id') ?? ''
	);
	const txId = $derived(rawTxId.split(/[?&]/)[0]);

	let tx = $state<TxStatus | null>(null);
	let loading = $state(true);
	let error = $state('');
	let redirectIn = $state<number | null>(null);

	const idr = (n: number) =>
		new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0 }).format(n);

	onMount(() => {
		let pollTimer: number | null = null;
		let redirectTimer: number | null = null;

		async function load() {
			if (!txId) {
				loading = false;
				error = 'Transaction ID tidak ditemukan di URL.';
				return;
			}
			try {
				const res = await fetch(`/v1/payments/status/${encodeURIComponent(txId)}`, { cache: 'no-store' });
				if (res.status === 404) {
					error = 'Pembayaran tidak ditemukan.';
					loading = false;
					return;
				}
				if (!res.ok) throw new Error('Gagal memuat status.');
				const data = (await res.json()) as TxStatus;
				tx = data;
				loading = false;
				if (FINAL.includes(data.status) && pollTimer) {
					window.clearInterval(pollTimer);
					pollTimer = null;
				}
			} catch {
				error = 'Gagal memuat status pembayaran.';
				loading = false;
			}
		}

		void load();
		if (txId) pollTimer = window.setInterval(() => void load(), 3000);

		const stopRedirect = $effect.root(() => {
			$effect(() => {
				if (!tx || tx.status !== 'paid' || redirectTimer) return;
				const target = tx.brand_id ? `/klien/${tx.brand_id}` : '/klien';
				redirectIn = 5;
				redirectTimer = window.setInterval(() => {
					redirectIn = (redirectIn ?? 1) - 1;
					if ((redirectIn ?? 0) <= 0) {
						if (redirectTimer) window.clearInterval(redirectTimer);
						redirectTimer = null;
						window.location.replace(target);
					}
				}, 1000);
			});
		});

		return () => {
			if (pollTimer) window.clearInterval(pollTimer);
			if (redirectTimer) window.clearInterval(redirectTimer);
			stopRedirect();
		};
	});

	const meta = $derived(tx ? (STATUS_LABEL[tx.status] ?? { label: tx.status, color: '#6b7280', icon: '?' }) : null);
	const isFinal = $derived(Boolean(tx && FINAL.includes(tx.status)));
	const dashboardUrl = $derived(tx?.brand_id ? `/klien/${tx.brand_id}` : '/klien');
</script>

<svelte:head>
	<title>Status Pembayaran — Tauthub</title>
</svelte:head>

<main
	class="flex min-h-screen items-center justify-center p-6"
	style="background: linear-gradient(135deg, #f8fafc, #eef2f7);"
>
	<div
		class="w-full max-w-[440px] rounded-[18px] border border-[#e2e8f0] bg-white p-7 text-center"
		style="box-shadow: 0 12px 40px rgba(0,0,0,.08);"
	>
		{#if tx?.status === 'paid'}
			<div class="mb-4 flex justify-center">
				<svg width="88" height="88" viewBox="0 0 88 88" fill="none" aria-hidden="true">
					<circle cx="44" cy="44" r="40" stroke="#16a34a" stroke-width="5" stroke-linecap="round" fill="rgba(22,163,74,0.08)" />
					<path d="M28 45.5 L39 56.5 L61 33" stroke="#16a34a" stroke-width="7" stroke-linecap="round" stroke-linejoin="round" fill="none" />
				</svg>
			</div>
		{:else}
			<div
				class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full text-3xl font-bold text-white"
				style="background-color: {meta?.color ?? '#e2e8f0'};"
			>
				{loading && !tx ? '…' : (meta?.icon ?? '…')}
			</div>
		{/if}

		{#if loading && !tx}
			<p class="text-[15px] text-slate-600">Memeriksa status pembayaran…</p>
		{:else if error}
			<div>
				<h1 class="mb-2 text-xl font-bold text-slate-900">Terjadi Kesalahan</h1>
				<p class="mb-5 text-sm text-slate-500">{error}</p>
				<a class="inline-block rounded-[10px] bg-[#2563eb] px-5 py-2.5 text-sm font-semibold text-white" href="/">Ke Beranda</a>
			</div>
		{:else if tx && meta}
			<div>
				<h1 class="mb-1.5 text-xl font-bold text-slate-900">{meta.label}</h1>
				<p class="mb-4 text-sm text-slate-500">
					{tx.brand_name ? `Untuk: ${tx.brand_name}` : 'Transaksi Tauthub'}{tx.reference ? ` · ${tx.reference}` : ''}
				</p>
				<div class="mb-4 text-[26px] font-extrabold text-slate-900">{idr(tx.amount)}</div>

				{#if tx.status === 'paid'}
					<div class="mb-3 rounded-[10px] border border-[#bbf7d0] bg-[#f0fdf4] px-3.5 py-3 text-left text-[13.5px] leading-relaxed text-[#15803d]">
						Tauthub Credit berhasil masuk ke saldo akun Anda.
					</div>
					{#if redirectIn !== null}
						<p class="mb-5 text-[13px] text-slate-500">
							Mengalihkan ke dashboard klien dalam <strong>{redirectIn}</strong> detik…
						</p>
					{/if}
				{:else if !isFinal}
					<p class="mb-5 text-[13px] text-slate-500">Halaman ini akan diperbarui otomatis saat pembayaran terverifikasi.</p>
				{:else}
					<p class="mb-5 text-[13px] text-slate-500">Jika ini tidak sesuai, hubungi dukungan Tauthub.</p>
				{/if}

				<a class="inline-block rounded-[10px] bg-[#2563eb] px-5 py-2.5 text-sm font-semibold text-white" href={dashboardUrl}>
					Ke Dashboard Klien
				</a>
			</div>
		{/if}
	</div>
</main>
