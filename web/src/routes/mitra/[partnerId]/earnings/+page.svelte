<script lang="ts">
	// Penghasilan mitra — port section "Penghasilan": ringkasan saldo, daftar
	// earning per publikasi, dan penarikan dana (bank/e-wallet, minimal + biaya
	// transfer dari tc_config).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Banknote, Wallet } from '@lucide/svelte';
	import { getPartnerUrlId } from '$lib/partner-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const partnerId = $derived(page.params.partnerId as string);
	const base = $derived(`/mitra/${getPartnerUrlId() || partnerId}`);

	let loading = $state(true);
	let data = $state<any>(null);
	let settlements = $state<any>(null);
	let error = $state('');
	let notice = $state('');
	let busy = $state(false);
	let showWithdraw = $state(false);
	let mode = $state<'all' | 'partial'>('all');
	let amount = $state(0);
	let notes = $state('');

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
		const [earnRes, setRes] = await Promise.all([
			apiRequest('/v1/partner/earnings?limit=100'),
			apiRequest('/v1/partner/settlements?limit=20')
		]);
		if (earnRes.ok) data = await earnRes.json();
		if (setRes.ok) settlements = await setRes.json();
	}

	async function withdraw() {
		busy = true;
		error = '';
		try {
			const payload: Record<string, unknown> = { mode };
			if (mode === 'partial') payload.amount_idr = Number(amount) || 0;
			if (notes.trim()) payload.notes = notes.trim();
			const res = await apiRequest('/v1/partner/settlements', { method: 'POST', body: JSON.stringify(payload) });
			if (!res.ok) {
				error = await readApiError(res, "Permintaan gagal.");
				return;
			}
			notice = 'Permintaan penarikan dikirim dan menunggu review admin.';
			showWithdraw = false;
			await load();
		} finally {
			busy = false;
		}
	}

	const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID')}`;
	const dt = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '—');
	const statusLabel: Record<string, string> = {
		pending: 'Menunggu cair', settled: 'Siap ditarik', voided: 'Dibatalkan'
	};
</script>

<section class="mx-auto max-w-6xl">
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Penghasilan</h1>
			<p class="mt-1 text-sm text-muted-foreground">Saldo mitra bertambah saat artikel terbit di media Anda.</p>
		</div>
		<button class="app-btn app-btn-primary" onclick={() => (showWithdraw = true)}><Banknote size={15} /> Tarik Saldo</button>
	</header>

	{#if notice}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}
	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat penghasilan…</p>
	{:else if data}
		<div class="mt-4 grid gap-3 sm:grid-cols-3">
			<div class="app-card p-4">
				<p class="text-xs font-semibold tracking-wide text-muted-foreground">SIAP DITARIK</p>
				<p class="mt-2 text-2xl font-bold tabular-nums">{rp(data.summary?.settled)}</p>
				<p class="mt-1 text-xs text-muted-foreground">Minimal penarikan {rp(data.minimum_withdraw_idr)}</p>
			</div>
			<div class="app-card p-4">
				<p class="text-xs font-semibold tracking-wide text-muted-foreground">MENUNGGU CAIR</p>
				<p class="mt-2 text-2xl font-bold tabular-nums">{rp(data.summary?.pending)}</p>
			</div>
			<div class="app-card p-4">
				<p class="text-xs font-semibold tracking-wide text-muted-foreground">TOTAL DIBYARKAN</p>
				<p class="mt-2 text-2xl font-bold tabular-nums">{rp(data.summary?.paid_settlements)}</p>
			</div>
		</div>

		<div class="app-card mt-4 overflow-x-auto">
			<div class="border-b px-4 py-3"><h2 class="text-sm font-semibold">Rincian Penghasilan</h2></div>
			{#if (data.earnings ?? []).length === 0}
				<p class="px-4 py-8 text-center text-sm text-muted-foreground">Belum ada penghasilan.</p>
			{:else}
				<table class="w-full min-w-[720px] text-sm">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="px-4 py-2 font-medium">Artikel</th>
							<th class="px-4 py-2 font-medium">Media</th>
							<th class="px-4 py-2 text-right font-medium">Nilai</th>
							<th class="px-4 py-2 text-right font-medium">Harga Klien</th>
							<th class="px-4 py-2 font-medium">Status</th>
							<th class="px-4 py-2 font-medium">Tanggal</th>
						</tr>
					</thead>
					<tbody>
						{#each data.earnings as e (e.id)}
							<tr class="border-t">
								<td class="max-w-[280px] truncate px-4 py-2">{e.document_title ?? '—'}</td>
								<td class="px-4 py-2 text-muted-foreground">{e.media_name ?? '—'}</td>
								<td class="px-4 py-2 text-right font-medium tabular-nums">{rp(e.amount)}</td>
								<td class="px-4 py-2 text-right tabular-nums text-muted-foreground">{rp(e.client_price)}</td>
								<td class="px-4 py-2">
									<span class="rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--accent); color: var(--accent-foreground);">
										{statusLabel[e.status] ?? e.status}
									</span>
								</td>
								<td class="px-4 py-2 text-xs text-muted-foreground">{dt(e.created_at)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	{/if}
</section>

{#if showWithdraw}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card w-full max-w-md p-5" aria-label="Tarik saldo">
			<h2 class="text-lg font-semibold">Tarik Saldo</h2>
			<p class="mt-1 text-xs text-muted-foreground">
				Saldo siap ditarik {rp(data?.summary?.settled ?? 0)} · biaya transfer bank {rp(data?.summary ? 2400 : 2400)}
			</p>
			{#if error}<p class="mt-3 text-sm" style="color: var(--destructive);">{error}</p>{/if}
			<div class="mt-4 space-y-3">
				<label class="flex items-center gap-2 text-sm">
					<input type="radio" value="all" bind:group={mode} /> Tarik seluruh saldo
				</label>
				<label class="flex items-center gap-2 text-sm">
					<input type="radio" value="partial" bind:group={mode} /> Tarik sebagian
				</label>
				{#if mode === 'partial'}
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Nominal (IDR)</span>
						<input class="app-input mt-1 w-full" type="number" min="0" bind:value={amount} />
					</label>
				{/if}
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">Catatan (opsional)</span>
					<input class="app-input mt-1 w-full" bind:value={notes} />
				</label>
				<p class="text-xs text-muted-foreground">
					Metode pencairan mengikuti pengaturan di <a class="font-medium" style="color: var(--primary);" href={`${base}/profile`}>Konfigurasi</a>.
				</p>
			</div>
			<div class="mt-5 flex justify-end gap-2">
				<button class="app-btn app-btn-soft" onclick={() => (showWithdraw = false)}>Batal</button>
				<button class="app-btn app-btn-primary" disabled={busy} onclick={withdraw}>{busy ? 'Mengirim…' : 'Ajukan Penarikan'}</button>
			</div>
		</dialog>
	</div>
{/if}
