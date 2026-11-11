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

	const PER_WEEK_MAX = 3;
	let perWeekMode = $state<'preset' | 'custom'>('preset');
	let perWeekInput = $state('');
	const perWeekValidCustom = $derived(/^\d+$/.test(perWeekInput) && Number(perWeekInput) >= 1 && Number(perWeekInput) <= PER_WEEK_MAX);
	const perWeekValid = $derived(perWeekMode === 'preset' || perWeekValidCustom);
	const perWeekError = $derived(perWeekMode === 'custom' && !perWeekValidCustom ? tr('Isi angka bulat 1 sampai 3.') : '');
	const totalArticles = $derived(weeks * perWeek);

	function setPreset(n: number) {
		perWeekMode = 'preset';
		perWeek = n;
	}
	function onCustomInput(raw: string) {
		perWeekInput = raw.trim();
		if (/^\d+$/.test(perWeekInput)) {
			const n = Number(perWeekInput);
			if (n >= 1 && n <= PER_WEEK_MAX) perWeek = n;
		}
	}
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

	<div class="rc-top">
		<form class="app-card rc-form" onsubmit={(e) => { e.preventDefault(); void create(); }}>
			<h2 class="rc-h2">{tr('Rencana baru')}</h2>
			<label class="rc-field">{tr('Brief')}
				<textarea class="app-input" rows="5" bind:value={brief} maxlength="3000"
					placeholder={tr('Apa yang dijual, untuk siapa, dan pesan utamanya.')}></textarea>
				<span class="rc-hint">{brief.trim().length}/3000 · {tr('minimal 20 karakter')}</span>
			</label>

			<div class="rc-split">
				<label class="rc-field">{tr('Anggaran total (Rp)')}
					<span class="rc-money">
						<span class="rc-prefix">Rp</span>
						<input class="app-input" inputmode="numeric" bind:value={budget} placeholder="1.000.000" />
					</span>
					<span class="rc-hint">{tr('Minimal Rp50.000')}</span>
				</label>
				<label class="rc-field">{tr('Kategori media (opsional)')}
					<input class="app-input" bind:value={category} maxlength="120" placeholder={tr('mis. Ekonomi')} />
				</label>
			</div>

			<div class="rc-split">
				<div class="rc-field">
					<span class="rc-row-top">{tr('Jumlah minggu')} <b class="rc-val">{weeks} {tr('minggu')}</b></span>
					<input class="rc-range" type="range" min="1" max="8" step="1" value={weeks}
						oninput={(e) => (weeks = Number((e.currentTarget as HTMLInputElement).value))}
						aria-label={tr('Jumlah minggu')} />
					<span class="rc-scale"><span>1</span><span>8</span></span>
				</div>
				<div class="rc-field" role="group" aria-label={tr('Artikel per minggu')}>
					<span class="rc-row-top">{tr('Artikel per minggu')}</span>
					<div class="rc-perweek">
						<div class="rc-seg">
							<button type="button" aria-pressed={perWeekMode === 'preset' && perWeek === 1} class:on={perWeekMode === 'preset' && perWeek === 1} onclick={() => setPreset(1)}>1</button>
							<button type="button" aria-pressed={perWeekMode === 'preset' && perWeek === 2} class:on={perWeekMode === 'preset' && perWeek === 2} onclick={() => setPreset(2)}>2</button>
							<button type="button" aria-pressed={perWeekMode === 'custom'} class:on={perWeekMode === 'custom'} onclick={() => (perWeekMode = 'custom')}>{tr('Lainnya')}</button>
						</div>
						{#if perWeekMode === 'custom'}
							<input class="app-input rc-custom" type="number" inputmode="numeric" min="1" max={PER_WEEK_MAX} step="1"
								placeholder="1–{PER_WEEK_MAX}" value={perWeekInput} aria-invalid={!!perWeekError}
								oninput={(e) => onCustomInput((e.currentTarget as HTMLInputElement).value)} />
						{/if}
					</div>
					{#if perWeekError}<span class="rc-bad">{perWeekError}</span>{/if}
				</div>
			</div>

			<dl class="rc-sum">
				<div><dt>{tr('Total artikel')}</dt><dd>{totalArticles}</dd></div>
				<div><dt>{tr('Anggaran per artikel')}</dt><dd>{budgetIdr > 0 ? `Rp${fmt(Math.floor(budgetIdr / totalArticles))}` : '-'}</dd></div>
				<div><dt>{tr('Setara')}</dt><dd>{budgetIdr > 0 ? `${fmt(Math.floor(budgetIdr / rate))} TC` : '-'}</dd></div>
			</dl>

			{#if createError}<p class="rc-bad">{createError}</p>{/if}
			<button class="app-btn app-btn-primary rc-btn" type="submit" disabled={creating || brief.trim().length < 20 || budgetIdr < 50000 || !perWeekValid}>
				{#if creating}<LoaderCircle size={15} class="animate-spin" />{/if}
				{tr('Susun rencana')}
			</button>
		</form>

		<aside class="app-card rc-how" aria-labelledby="rc-how-h">
			<h2 id="rc-how-h" class="rc-h2">{tr('Cara kerja')}</h2>
			<ol>
				<li><b>{tr('Susun')}</b><span>{tr('Sistem memilih media yang muat di anggaran dan mengusulkan topik tiap artikel dari brief Anda.')}</span></li>
				<li><b>{tr('Tinjau')}</b><span>{tr('Ganti media atau topik per baris, lalu setujui sekali.')}</span></li>
				<li><b>{tr('Draf tiap minggu')}</b><span>{tr('Tiap minggu sistem membuat kampanye draf dan memberi tahu Anda.')}</span></li>
				<li><b>{tr('Tulis dan distribusikan')}</b><span>{tr('Anda menulis, memeriksa mutu, dan mendistribusikan sendiri. Saldo baru terpotong saat distribusi.')}</span></li>
			</ol>
			<p class="rc-hint">{tr('Biaya adalah perkiraan dari harga media hari ini. Bila media tidak bisa dipesan lagi, barisnya ditandai perlu perhatian.')}</p>
		</aside>
	</div>

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
	.rc-top { display: grid; grid-template-columns: minmax(0, 1fr) 320px; gap: 24px; align-items: start; }
	.rc-error { padding: 12px 16px; font-size: 14px; border-color: var(--destructive); color: var(--destructive); }
	.rc-loading { padding: 20px; text-align: center; font-size: 14px; color: var(--muted-foreground); }
	.rc-form { padding: 24px; display: flex; flex-direction: column; gap: 20px; }
	.rc-h2 { font-size: 16px; line-height: 24px; font-weight: 600; color: var(--foreground); }
	.rc-field { display: flex; flex-direction: column; gap: 6px; font-size: 13px; font-weight: 500; color: var(--foreground); }
	.rc-split { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px; }
	.rc-hint { font-size: 12px; line-height: 16px; font-weight: 400; color: var(--muted-foreground); }
	.rc-bad { font-size: 13px; color: var(--destructive); }
	.rc-money { position: relative; display: block; }
	.rc-money :global(input) { width: 100%; padding-left: 38px; }
	.rc-prefix { position: absolute; left: 12px; top: 50%; transform: translateY(-50%); font-size: 13px; color: var(--muted-foreground); pointer-events: none; }
	.rc-seg { display: flex; gap: 4px; padding: 4px; border: 1px solid var(--border); border-radius: 10px; background: var(--card); }
	.rc-seg button { flex: 1; min-width: 0; height: 36px; border-radius: 7px; font-size: 13px; font-weight: 500; color: var(--muted-foreground); transition: background-color 140ms ease, color 140ms ease; }
	.rc-seg button:hover { background: var(--secondary); }
	.rc-seg button.on { background: var(--primary); color: #fff; }
	.rc-sum { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 0; border: 1px solid var(--border); border-radius: 10px; overflow: hidden; margin: 0; }
	.rc-sum div { padding: 12px 16px; display: flex; flex-direction: column; gap: 2px; }
	.rc-sum div + div { border-left: 1px solid var(--border); }
	.rc-sum dt { font-size: 12px; color: var(--muted-foreground); }
	.rc-sum dd { margin: 0; font-size: 18px; line-height: 26px; font-weight: 600; font-variant-numeric: tabular-nums; color: var(--foreground); }
	.rc-row-top { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; font-size: 13px; font-weight: 500; color: var(--foreground); }
	.rc-val { font-size: 13px; font-weight: 600; font-variant-numeric: tabular-nums; color: var(--primary); }
	.rc-range { width: 100%; accent-color: var(--primary); margin: 6px 0 0; }
	.rc-scale { display: flex; justify-content: space-between; font-size: 12px; color: var(--muted-foreground); }
	.rc-perweek { display: grid; grid-template-columns: minmax(0, 1fr) 96px; gap: 8px; align-items: center; }
	.rc-custom { height: 44px; text-align: center; font-variant-numeric: tabular-nums; }
	.rc-btn { align-self: flex-start; min-height: 44px; display: inline-flex; align-items: center; gap: 8px; }
	.rc-how { padding: 24px; display: flex; flex-direction: column; gap: 16px; }
	.rc-how ol { list-style: none; margin: 0; padding: 0; counter-reset: step; display: flex; flex-direction: column; gap: 18px; }
	.rc-how li { position: relative; display: flex; flex-direction: column; gap: 2px; padding-left: 36px; counter-increment: step; font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.rc-how li b { font-size: 14px; font-weight: 600; color: var(--foreground); }
	.rc-how li::before { content: counter(step); position: absolute; left: 0; top: 0; width: 24px; height: 24px; border-radius: 999px; border: 1px solid var(--border); background: var(--card); display: grid; place-items: center; font-size: 12px; font-weight: 600; color: var(--foreground); }
	.rc-how li::before { z-index: 1; }
	.rc-how li:not(:last-child)::after { content: ''; position: absolute; left: 12px; top: 28px; bottom: -22px; width: 1px; background: var(--border); }
	.rc-muted { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.rc-list { padding: 0; list-style: none; overflow: hidden; margin-top: 12px; }
	.rc-list li + li { border-top: 1px solid var(--border); }
	.rc-row { display: grid; grid-template-columns: minmax(0, 1fr) auto auto; align-items: center; gap: 16px; padding: 16px 20px; color: inherit; text-decoration: none; transition: background-color 140ms ease; }
	.rc-row:hover { background: color-mix(in srgb, var(--secondary) 60%, transparent); }
	.rc-name { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
	.rc-title { font-size: 14px; font-weight: 600; line-height: 22px; color: var(--foreground); word-break: break-word; }
	.rc-meta { font-size: 13px; color: var(--muted-foreground); }
	:global(.rc-arrow) { color: var(--muted-foreground); transition: transform 140ms ease; }
	.rc-row:hover :global(.rc-arrow) { transform: translateX(4px); }
	@media (max-width: 1100px) { .rc-top { grid-template-columns: 1fr; } }
	@media (max-width: 560px) { .rc-split { grid-template-columns: 1fr; } .rc-form, .rc-how { padding: 16px; } .rc-sum { grid-template-columns: 1fr; } .rc-sum div + div { border-left: 0; border-top: 1px solid var(--border); } .rc-row { grid-template-columns: minmax(0, 1fr) auto; } .rc-meta { grid-column: 1 / -1; } }
	@media (prefers-reduced-motion: reduce) { .rc-row, .rc-seg button, :global(.rc-arrow) { transition: none; } }
</style>
