<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Rencana Bulanan: klien memberi brief dan anggaran, sistem menyusun rencana
	// beberapa minggu (media dan topik per artikel). Klien meninjau, menyunting,
	// dan menyetujui sekali. Tiap minggu sistem membuat kampanye draf; tidak ada
	// saldo terpotong dan tidak ada distribusi otomatis.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { ArrowRight, LoaderCircle } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	type PlanRow = {
		id: string; name: string; status: string; weeks: number; per_week: number; budget_idr: number;
		start_date: string; items_total: number; items_ready: number; est_total_tc: number;
	};

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);

	let loading = $state(true);
	let error = $state('');
	let plans = $state<PlanRow[]>([]);
	let rate = $state(1000);

	let brief = $state('');
	let budget = $state('');
	let weeks = $state(4);
	let perWeek = $state(1);
	let category = $state('');
	let creating = $state(false);
	let createError = $state('');

	async function load() {
		const res = await apiRequest(`/v1/brands/${brandId}/taut/plans`);
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal memuat rencana.'));
			return;
		}
		const j = await res.json();
		plans = j.plans ?? [];
		rate = j.rate_idr ?? 1000;
	}

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

	const budgetIdr = $derived(parseInt(budget.replace(/\D/g, '') || '0', 10));
	const fmt = (n: number) => (n ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0 });
	const statusLabel: Record<string, string> = { draft: 'Draf', approved: 'Disetujui', running: 'Berjalan', done: 'Selesai', cancelled: 'Dibatalkan' };

	async function create() {
		createError = '';
		creating = true;
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/taut/plans`, {
				method: 'POST',
				body: JSON.stringify({ brief, budget_idr: budgetIdr, weeks, per_week: perWeek, category })
			});
			if (!res.ok) {
				createError = await readApiError(res, tr('Gagal menyusun rencana.'));
				return;
			}
			const j = await res.json();
			await goto(`/klien/${brandUrl}/rencana/${j.id}`);
		} finally {
			creating = false;
		}
	}
</script>

<PageTitle title={tr('Rencana Bulanan')} />

<section class="rc">
	<header>
		<h1 class="page-title">{tr('Rencana Bulanan')}</h1>
		<p class="page-desc">{tr('Susun rencana artikel beberapa minggu sekali jalan. Setujui sekali, lalu tiap minggu kampanye draf dibuat untuk Anda.')}</p>
	</header>

	{#if error}<div class="app-card rc-error" role="alert">{error}</div>{/if}

	<form class="app-card rc-form" onsubmit={(e) => { e.preventDefault(); void create(); }}>
		<h2 class="rc-h2">{tr('Rencana baru')}</h2>
		<label class="rc-field">{tr('Brief')}
			<textarea class="app-input" rows="4" bind:value={brief} maxlength="3000"
				placeholder={tr('Apa yang dijual, untuk siapa, dan pesan utamanya.')}></textarea>
		</label>
		<div class="rc-grid">
			<label class="rc-field">{tr('Anggaran total (Rp)')}
				<input class="app-input" inputmode="numeric" bind:value={budget} placeholder="1.000.000" />
			</label>
			<label class="rc-field">{tr('Jumlah minggu')}
				<select class="app-input" bind:value={weeks}>{#each [1, 2, 3, 4, 5, 6, 7, 8] as n (n)}<option value={n}>{n}</option>{/each}</select>
			</label>
			<label class="rc-field">{tr('Artikel per minggu')}
				<select class="app-input" bind:value={perWeek}>{#each [1, 2, 3] as n (n)}<option value={n}>{n}</option>{/each}</select>
			</label>
			<label class="rc-field">{tr('Kategori media (opsional)')}
				<input class="app-input" bind:value={category} maxlength="120" placeholder={tr('mis. Ekonomi')} />
			</label>
		</div>
		<p class="rc-muted">{tr('Biaya adalah perkiraan dari harga media hari ini. Saldo baru terpotong saat Anda menulis dan mendistribusikan tiap artikel.')}
			{#if budgetIdr > 0}{tr('Anggaran setara')} {fmt(Math.floor(budgetIdr / rate))} TC.{/if}</p>
		{#if createError}<p class="rc-bad">{createError}</p>{/if}
		<button class="app-btn app-btn-primary rc-btn" type="submit" disabled={creating || brief.trim().length < 20 || budgetIdr < 50000}>
			{#if creating}<LoaderCircle size={15} class="animate-spin" />{/if}
			{tr('Susun rencana')}
		</button>
	</form>

	{#if loading}
		<div class="app-card rc-loading">{tr('Memuat rencana…')}</div>
	{:else if plans.length > 0}
		<section aria-labelledby="rc-list">
			<h2 id="rc-list" class="rc-h2">{tr('Rencana Anda')}</h2>
			<ul class="app-card rc-list">
				{#each plans as p (p.id)}
					<li>
						<a class="rc-row" href={`/klien/${brandUrl}/rencana/${p.id}`}>
							<span class="rc-name">
								<span class="rc-title">{p.name}</span>
								<span class="rc-muted">{p.weeks} {tr('minggu')}, {p.items_total} {tr('artikel')}, {tr('mulai')} {p.start_date}</span>
							</span>
							<span class="rc-meta">{tr(statusLabel[p.status] ?? p.status)} · {p.items_ready}/{p.items_total} {tr('siap')}</span>
							<ArrowRight size={16} class="rc-arrow" />
						</a>
					</li>
				{/each}
			</ul>
		</section>
	{/if}
</section>

<style>
	.rc { display: flex; flex-direction: column; gap: 24px; }
	.rc-error { padding: 12px 16px; font-size: 14px; border-color: var(--destructive); color: var(--destructive); }
	.rc-loading { padding: 20px; text-align: center; font-size: 14px; color: var(--muted-foreground); }
	.rc-form { padding: 20px; display: flex; flex-direction: column; gap: 16px; }
	.rc-h2 { font-size: 18px; line-height: 24px; font-weight: 600; color: var(--foreground); }
	.rc-field { display: flex; flex-direction: column; gap: 4px; font-size: 13px; color: var(--muted-foreground); }
	.rc-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px; }
	.rc-muted { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.rc-bad { font-size: 13px; color: var(--destructive); }
	.rc-btn { align-self: flex-start; min-height: 44px; display: inline-flex; align-items: center; gap: 8px; }
	.rc-list { padding: 0; list-style: none; overflow: hidden; margin-top: 12px; }
	.rc-list li + li { border-top: 1px solid var(--border); }
	.rc-row { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: 16px; padding: 16px 20px; color: inherit; text-decoration: none; transition: background-color 140ms ease; }
	.rc-row:hover { background: color-mix(in srgb, var(--secondary) 60%, transparent); }
	.rc-name { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
	.rc-title { font-size: 14px; font-weight: 600; line-height: 22px; color: var(--foreground); word-break: break-word; }
	.rc-meta { font-size: 13px; color: var(--muted-foreground); }
	:global(.rc-arrow) { color: var(--muted-foreground); transition: transform 140ms ease; }
	.rc-row:hover :global(.rc-arrow) { transform: translateX(4px); }
	@media (max-width: 860px) { .rc-grid { grid-template-columns: 1fr 1fr; } }
	@media (max-width: 560px) { .rc-grid { grid-template-columns: 1fr; } .rc-row { grid-template-columns: minmax(0, 1fr) auto; } .rc-meta { grid-column: 1 / -1; } }
	@media (prefers-reduced-motion: reduce) { .rc-row, :global(.rc-arrow) { transition: none; } }
</style>
