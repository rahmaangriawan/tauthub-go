<script lang="ts">
	// Invoice / Settlements mitra — port section "Invoice": daftar penarikan
	// (settlement) beserta status, nomor invoice, biaya transfer, dan bukti bayar.
	import { onMount } from 'svelte';
	import { ExternalLink, Receipt } from '@lucide/svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	let loading = $state(true);
	let data = $state<any>(null);
	let statusFilter = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		const params = new URLSearchParams({ limit: '100' });
		if (statusFilter) params.set('status', statusFilter);
		const res = await apiRequest(`/v1/partner/settlements?${params}`);
		if (res.ok) data = await res.json();
	}

	const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID')}`;
	const dt = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '—');
	const statusLabel: Record<string, string> = {
		pending: 'Menunggu review', approved: 'Disetujui', processing: 'Diproses',
		paid: 'Dibayar', failed: 'Gagal'
	};
</script>

<section class="mx-auto max-w-6xl">
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Invoice & Penarikan</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				Riwayat penarikan dana Anda. Minimal penarikan {rp(data?.minimum_withdraw_idr ?? 0)}.
			</p>
		</div>
		<select class="app-input !w-auto" bind:value={statusFilter} onchange={load}>
			<option value="">Semua status</option>
			<option value="pending">Menunggu review</option>
			<option value="approved">Disetujui</option>
			<option value="processing">Diproses</option>
			<option value="paid">Dibayar</option>
			<option value="failed">Gagal</option>
		</select>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat penarikan…</p>
	{:else if !data || (data.settlements ?? []).length === 0}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<Receipt size={28} class="mx-auto text-muted-foreground" />
			<p class="mt-3 text-sm text-muted-foreground">Belum ada penarikan dana.</p>
		</div>
	{:else}
		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">No. Invoice</th>
						<th class="px-4 py-2.5 font-medium">Periode</th>
						<th class="px-4 py-2.5 text-right font-medium">Bruto</th>
						<th class="px-4 py-2.5 text-right font-medium">Biaya</th>
						<th class="px-4 py-2.5 text-right font-medium">Net</th>
						<th class="px-4 py-2.5 font-medium">Status</th>
						<th class="px-4 py-2.5 font-medium">Bukti</th>
					</tr>
				</thead>
				<tbody>
					{#each data.settlements as s (s.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5 font-mono text-xs">{s.invoice_number ?? s.id.slice(0, 8)}</td>
							<td class="px-4 py-2.5 text-muted-foreground">{dt(s.period_start)} – {dt(s.period_end)}</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{rp(s.total_amount)}</td>
							<td class="px-4 py-2.5 text-right tabular-nums text-muted-foreground">{rp(s.transfer_fee_idr)}</td>
							<td class="px-4 py-2.5 text-right font-medium tabular-nums">{rp(s.net_amount_idr)}</td>
							<td class="px-4 py-2.5">
								<span class="rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--accent); color: var(--accent-foreground);">
									{statusLabel[s.status] ?? s.status}
								</span>
								{#if s.rejection_reason}<p class="mt-0.5 text-[11px]" style="color: var(--destructive);">{s.rejection_reason}</p>{/if}
							</td>
							<td class="px-4 py-2.5">
								{#if s.receipt_url}
									<a class="inline-flex items-center gap-1 text-xs" style="color: var(--primary);" href={s.receipt_url} target="_blank" rel="noreferrer">Lihat <ExternalLink size={11} /></a>
								{:else}
									<span class="text-xs text-muted-foreground">—</span>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>
