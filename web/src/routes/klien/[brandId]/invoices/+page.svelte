<script lang="ts">
	// Invoice & Pajak (read-only; pembuatan invoice dari admin).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Receipt } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import Select from '$lib/components/select.svelte';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);
	type Invoice = {
		id: string; number: string; status: string; amount: number; campaign_name: string | null;
		issued_at: string | null; due_at: string | null; paid_at: string | null; notes: string | null;
	};

	let invoices = $state<Invoice[]>([]);
	let total = $state(0);
	let status = $state('');
	let loading = $state(true);

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
		if (status) params.set('status', status);
		const res = await apiRequest(`/v1/brands/${brandId}/invoices?${params}`);
		if (res.ok) {
			const data = await res.json();
			invoices = data.invoices ?? [];
			total = data.total ?? 0;
		}
	}

	const statusLabel: Record<string, string> = { draft: 'Draf', issued: 'Diterbitkan', paid: 'Lunas', void: 'Dibatalkan' };
	const stateStyle = (s: string) =>
		s === 'paid' ? 'background-color: var(--success-soft); color: var(--success);'
		: s === 'issued' ? 'background-color: var(--warning-soft); color: var(--warning);'
		: 'background-color: var(--muted); color: var(--muted-foreground);';
	const fmt = (n: number) => n.toLocaleString('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0 });
	const fmtDate = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID') : '—');
</script>

<section class="mx-auto max-w-5xl">
	<header class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Invoice &amp; Pajak</h1>
			<p class="mt-1 text-sm text-muted-foreground">{total} invoice tercatat untuk brand ini.</p>
		</div>
		<Select
			bind:value={status}
			ariaLabel="Status invoice"
			placeholder="Semua status"
			align="end"
			onchange={() => load()}
			options={[
				{ value: '', label: 'Semua status' },
				{ value: 'draft', label: 'Draf' },
				{ value: 'issued', label: 'Diterbitkan' },
				{ value: 'paid', label: 'Lunas' },
				{ value: 'void', label: 'Dibatalkan' }
			]}
		/>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat invoice…</p>
	{:else if invoices.length === 0}
		<div class="app-card mt-6 flex flex-col items-center px-6 py-12 text-center">
			<Receipt size={22} style="color: var(--primary);" />
			<p class="mt-2 text-sm font-medium">{status ? 'Tidak ada invoice dengan status ini.' : 'Belum ada invoice.'}</p>
			<p class="mt-1 text-sm text-muted-foreground">Invoice diterbitkan admin setelah kampanye atau pembelian paket.</p>
		</div>
	{:else}
		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="border-b text-left text-[11px] tracking-wide text-muted-foreground">
						<th class="px-4 py-2.5 font-semibold">NOMOR</th>
						<th class="px-4 py-2.5 font-semibold">KAMPANYE</th>
						<th class="px-4 py-2.5 font-semibold">STATUS</th>
						<th class="px-4 py-2.5 text-right font-semibold">JUMLAH</th>
						<th class="px-4 py-2.5 font-semibold">JATUH TEMPO</th>
					</tr>
				</thead>
				<tbody>
					{#each invoices as inv (inv.id)}
						<tr class="border-b last:border-0">
							<td class="px-4 py-2.5 font-medium">{inv.number}</td>
							<td class="px-4 py-2.5 text-muted-foreground">{inv.campaign_name ?? '—'}</td>
							<td class="px-4 py-2.5">
								<span class="rounded-full px-2 py-0.5 text-xs font-semibold" style={stateStyle(inv.status)}>
									{statusLabel[inv.status] ?? inv.status}
								</span>
							</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{fmt(inv.amount)}</td>
							<td class="px-4 py-2.5 text-muted-foreground">{fmtDate(inv.due_at)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>
