<script lang="ts">
	// Kredit Saldo — wallet, konfigurasi TC, ledger, transaksi (port section credits).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Coins, Lock, Wallet } from '@lucide/svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const brandId = $derived(page.params.brandId as string);
	type Credits = {
		pending_payment: { id: string; payment_url: string | null; amount_idr: number; tc: number | null; status: string } | null;
		balance: { total: number; used: number; remaining: number; package_count: number };
		wallet: { available_tc: number; reserved_tc: number; captured_tc: number; total_tc: number; rate_idr: number; available_idr: number };
		tc_config: { rate_idr: number; topup_packages: Array<{ tc?: number; price_idr?: number; label?: string }> };
		ledger: Array<{ id: string; ref_type: string; amount_tc: number; balance_after_tc: number; reason: string | null; document_title: string | null; media_name: string | null; created_at: string }>;
		ledger_total: number;
		transactions: Array<{ id: string; amount: number; reason: string; balance_after: number; package_name: string | null; document_title: string | null; created_at: string }>;
		total: number;
	};

	let data = $state<Credits | null>(null);
	let loading = $state(true);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			const res = await apiRequest(`/v1/brands/${brandId}/credits?limit=50`);
			if (res.ok) data = await res.json();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	const refLabel: Record<string, string> = {
		topup: 'Top up', reserve: 'Reservasi', release: 'Pelepasan', capture: 'Pemakaian',
		refund: 'Refund', adjustment: 'Penyesuaian'
	};
	const fmt = (n: number) => n.toLocaleString('id-ID');
	const fmtDate = (v: string) => new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' });
</script>

<section class="mx-auto max-w-5xl">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Kredit Saldo</h1>
		<p class="mt-1 text-sm text-muted-foreground">Saldo Tulisio Credit (TC) untuk eksekusi publikasi.</p>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat saldo…</p>
	{:else if data}
		{#if data.pending_payment}
			<div class="app-card mt-4 flex flex-wrap items-center justify-between gap-3 border-l-4 p-4" style="border-left-color: var(--warning);">
				<div>
					<p class="text-sm font-medium">Ada pembayaran top up yang belum selesai</p>
					<p class="text-xs text-muted-foreground">
						{data.pending_payment.tc ?? 0} TC · Rp {fmt(data.pending_payment.amount_idr)} · status {data.pending_payment.status}
					</p>
				</div>
				{#if data.pending_payment.payment_url}
					<a class="app-btn app-btn-primary" href={data.pending_payment.payment_url}>Lanjutkan pembayaran</a>
				{/if}
			</div>
		{/if}

		<div class="mt-4 grid gap-3 sm:grid-cols-3">
			<div class="app-card flex items-center gap-3 p-4">
				<Wallet size={18} style="color: var(--primary);" />
				<div>
					<p class="text-xs text-muted-foreground">Saldo tersedia</p>
					<p class="text-2xl font-bold tabular-nums">{fmt(data.wallet.available_tc)} <span class="text-sm font-medium text-muted-foreground">TC</span></p>
				</div>
			</div>
			<div class="app-card flex items-center gap-3 p-4">
				<Lock size={18} style="color: var(--warning);" />
				<div>
					<p class="text-xs text-muted-foreground">Tertahan</p>
					<p class="text-2xl font-bold tabular-nums">{fmt(data.wallet.reserved_tc)} <span class="text-sm font-medium text-muted-foreground">TC</span></p>
				</div>
			</div>
			<div class="app-card flex items-center gap-3 p-4">
				<Coins size={18} style="color: var(--success);" />
				<div>
					<p class="text-xs text-muted-foreground">Total terpakai</p>
					<p class="text-2xl font-bold tabular-nums">{fmt(data.wallet.captured_tc)} <span class="text-sm font-medium text-muted-foreground">TC</span></p>
				</div>
			</div>
		</div>

		<p class="mt-2 text-xs text-muted-foreground">
			Kurs: 1 TC = Rp {fmt(data.tc_config.rate_idr)} · nilai saldo ≈ Rp {fmt(data.wallet.available_idr)}
		</p>

		<div class="app-card mt-4 overflow-x-auto">
			<div class="flex items-center justify-between border-b px-4 py-3">
				<h2 class="text-sm font-semibold">Riwayat ledger</h2>
				<span class="text-xs text-muted-foreground">{fmt(data.ledger_total)} entri</span>
			</div>
			{#if data.ledger.length === 0}
				<p class="px-4 py-8 text-center text-sm text-muted-foreground">
					Belum ada mutasi saldo. Top up atau publikasi artikel untuk melihat riwayat.
				</p>
			{:else}
				<table class="w-full text-sm">
					<thead>
						<tr class="border-b text-left text-[11px] tracking-wide text-muted-foreground">
							<th class="px-4 py-2.5 font-semibold">JENIS</th>
							<th class="px-4 py-2.5 font-semibold">REFERENSI</th>
							<th class="px-4 py-2.5 text-right font-semibold">JUMLAH</th>
							<th class="px-4 py-2.5 text-right font-semibold">SALDO SETELAH</th>
							<th class="px-4 py-2.5 font-semibold">WAKTU</th>
						</tr>
					</thead>
					<tbody>
						{#each data.ledger as row (row.id)}
							<tr class="border-b last:border-0">
								<td class="px-4 py-2.5">{refLabel[row.ref_type] ?? row.ref_type}</td>
								<td class="px-4 py-2.5 text-muted-foreground">
									{row.document_title ?? row.media_name ?? row.reason ?? '—'}
								</td>
								<td class="px-4 py-2.5 text-right tabular-nums" style={row.amount_tc < 0 ? 'color: var(--destructive);' : 'color: var(--success);'}>
									{row.amount_tc > 0 ? '+' : ''}{fmt(row.amount_tc)}
								</td>
								<td class="px-4 py-2.5 text-right tabular-nums">{fmt(row.balance_after_tc)}</td>
								<td class="px-4 py-2.5 text-xs text-muted-foreground">{fmtDate(row.created_at)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	{/if}
</section>
