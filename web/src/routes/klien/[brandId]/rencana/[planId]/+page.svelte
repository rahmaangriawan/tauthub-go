<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Rincian rencana: tinjau, sunting topik, lewati artikel, setujui sekali.
	// Setelah disetujui, tiap minggu sistem membuat kampanye draf berisi media dan
	// topik itu. Saldo tidak terpotong dan tidak ada yang didistribusikan otomatis.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ArrowLeft, LoaderCircle } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	type Item = {
		id: string; week: number; run_on: string; media_id: string; media_name: string; website_url: string | null;
		topic: string; est_media_tc: number; est_writer_tc: number; status: string; campaign_id: string | null; note: string | null;
	};
	type Plan = {
		id: string; name: string; brief: string; budget_idr: number; weeks: number; per_week: number; start_date: string;
		status: string; auto_renew: boolean; items: Item[]; est_total_tc: number; rate_idr: number;
	};

	const brandId = $derived(page.params.brandId as string);
	const planId = $derived(page.params.planId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);

	let loading = $state(true);
	let error = $state('');
	let notice = $state('');
	let busy = $state(false);
	let plan = $state<Plan | null>(null);
	let edits = $state<Record<string, string>>({});

	async function load() {
		const res = await apiRequest(`/v1/brands/${brandId}/taut/plans/${planId}`);
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal memuat rencana.'));
			return;
		}
		plan = (await res.json()) as Plan;
		edits = Object.fromEntries(plan.items.map((i) => [i.id, i.topic]));
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

	async function call(path: string, method: string, body?: unknown, ok?: string) {
		busy = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/taut/plans/${planId}${path}`, {
				method,
				body: body === undefined ? undefined : JSON.stringify(body)
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal memproses rencana.'));
				return false;
			}
			if (ok) notice = ok;
			await load();
			return true;
		} finally {
			busy = false;
		}
	}

	const saveTopic = (i: Item) => {
		if (edits[i.id] !== i.topic) void call(`/items/${i.id}`, 'PATCH', { topic: edits[i.id] });
	};
	const approve = () => {
		if (!window.confirm(tr('Setujui rencana ini? Mulai tanggal yang tertera, kampanye draf dibuat tiap minggu. Saldo tidak terpotong sampai Anda sendiri mendistribusikan artikelnya.'))) return;
		void call('/approve', 'POST', undefined, tr('Rencana disetujui.'));
	};
	const cancel = () => {
		if (!window.confirm(tr('Batalkan rencana ini? Artikel yang belum jalan tidak akan dibuat. Kampanye draf yang sudah ada tetap ada.'))) return;
		void call('/cancel', 'POST', undefined, tr('Rencana dibatalkan.'));
	};

	const fmt = (n: number) => (n ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0 });
	const rp = (tc: number) => `Rp ${fmt(tc * (plan?.rate_idr ?? 1000))}`;
	const statusLabel: Record<string, string> = { draft: 'Draf', approved: 'Disetujui', running: 'Berjalan', done: 'Selesai', cancelled: 'Dibatalkan' };
	const itemLabel: Record<string, string> = { pending: 'Menunggu', running: 'Dibuat…', ready: 'Siap ditulis', needs_attention: 'Perlu perhatian', skipped: 'Dilewati' };
	const draft = $derived(plan?.status === 'draft');
</script>

<PageTitle title={plan?.name ?? tr('Rencana Bulanan')} />

<section class="pd">
	<a class="pd-back" href={`/klien/${brandUrl}/rencana`}><ArrowLeft size={15} />{tr('Rencana Bulanan')}</a>

	{#if error}<div class="app-card pd-error" role="alert">{error}</div>{/if}
	{#if notice}<div class="app-card pd-notice" role="status">{notice}</div>{/if}

	{#if loading}
		<div class="app-card pd-loading">{tr('Memuat rencana…')}</div>
	{:else if plan}
		<header class="pd-head">
			<div>
				<h1 class="page-title">{plan.name}</h1>
				<p class="page-desc">{tr(statusLabel[plan.status] ?? plan.status)} · {plan.weeks} {tr('minggu')}, {tr('mulai')} {plan.start_date}</p>
			</div>
			{#if draft}
				<button class="app-btn app-btn-primary pd-btn" type="button" disabled={busy} onclick={approve}>
					{#if busy}<LoaderCircle size={15} class="animate-spin" />{/if}{tr('Setujui rencana')}
				</button>
			{/if}
		</header>

		<dl class="pd-tiles">
			<div class="app-card pd-tile"><dt>{tr('Anggaran')}</dt><dd>Rp {fmt(plan.budget_idr)}</dd></div>
			<div class="app-card pd-tile"><dt>{tr('Perkiraan biaya rencana')}</dt><dd class:pd-bad={plan.est_total_tc * plan.rate_idr > plan.budget_idr}>{rp(plan.est_total_tc)}</dd></div>
			<div class="app-card pd-tile"><dt>{tr('Artikel')}</dt><dd>{plan.items.filter((i) => i.status !== 'skipped').length}</dd></div>
		</dl>
		<p class="pd-muted">{tr('Perkiraan memakai harga media hari ini, termasuk biaya tulis. Saldo baru terpotong saat Anda menulis dan mendistribusikan tiap artikel.')}</p>

		{#if plan.items.length === 0}
			<div class="app-card pd-empty"><p class="pd-muted">{tr('Belum ada media yang cocok dan cukup anggaran. Buat rencana baru dengan anggaran lebih besar atau kategori lain.')}</p></div>
		{:else}
			<ul class="app-card pd-list">
				{#each plan.items as i (i.id)}
					<li class="pd-row" class:pd-skipped={i.status === 'skipped'}>
						<div class="pd-when">
							<b>{tr('Minggu')} {i.week}</b>
							<span class="pd-muted">{i.run_on}</span>
						</div>
						<div class="pd-main">
							<p class="pd-media">{i.media_name}</p>
							{#if draft && i.status !== 'skipped'}
								<input class="app-input" bind:value={edits[i.id]} maxlength="160" aria-label={tr('Topik')} onblur={() => saveTopic(i)} />
							{:else}
								<p class="pd-topic">{i.topic}</p>
							{/if}
							{#if i.note}<p class="pd-bad">{i.note}</p>{/if}
						</div>
						<div class="pd-side">
							<span class="pd-muted">{rp(i.est_media_tc + i.est_writer_tc)}</span>
							<span class="pd-chip" class:pd-chip-bad={i.status === 'needs_attention'}>{tr(itemLabel[i.status] ?? i.status)}</span>
							{#if i.campaign_id}
								<a class="pd-link" href={`/klien/${brandUrl}/taut-pintar/${i.campaign_id}`}>{tr('Buka kampanye')}</a>
							{/if}
							{#if draft}
								<button class="pd-link" type="button" disabled={busy} onclick={() => void call(`/items/${i.id}`, 'PATCH', { skip: i.status !== 'skipped' })}>
									{i.status === 'skipped' ? tr('Aktifkan lagi') : tr('Lewati')}
								</button>
							{/if}
						</div>
					</li>
				{/each}
			</ul>
		{/if}

		{#if plan.status === 'draft' || plan.status === 'approved' || plan.status === 'running'}
			<div class="app-card pd-foot">
				<label class="pd-check">
					<input type="checkbox" checked={plan.auto_renew} disabled={busy} onchange={(e) => void call('', 'PATCH', { auto_renew: e.currentTarget.checked })} />
					<span>{tr('Siapkan rencana lanjutan saat rencana ini selesai')}
						<small>{tr('Hanya menyiapkan draf dengan harga hari itu. Tidak ada yang dijalankan atau ditagih sebelum Anda menyetujuinya.')}</small></span>
				</label>
				<button class="app-btn app-btn-soft pd-btn" type="button" disabled={busy} onclick={cancel}>{tr('Batalkan rencana')}</button>
			</div>
		{/if}
	{/if}
</section>

<style>
	.pd { display: flex; flex-direction: column; gap: 20px; }
	.pd-back { display: inline-flex; align-items: center; gap: 8px; min-height: 44px; align-self: flex-start; font-size: 13px; font-weight: 500; color: var(--primary); text-decoration: none; }
	.pd-back:hover { text-decoration: underline; }
	.pd-error { padding: 12px 16px; font-size: 14px; border-color: var(--destructive); color: var(--destructive); }
	.pd-notice { padding: 12px 16px; font-size: 14px; border-color: var(--success); background: var(--success-soft); color: var(--success); }
	.pd-loading { padding: 20px; text-align: center; font-size: 14px; color: var(--muted-foreground); }
	.pd-head { display: flex; flex-wrap: wrap; align-items: flex-start; justify-content: space-between; gap: 12px; }
	.pd-btn { min-height: 44px; display: inline-flex; align-items: center; gap: 8px; }
	.pd-tiles { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
	.pd-tile { padding: 16px 20px; display: flex; flex-direction: column; gap: 4px; }
	.pd-tile dt { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.pd-tile dd { font-size: 24px; line-height: 32px; font-weight: 600; letter-spacing: -0.01em; color: var(--foreground); font-variant-numeric: tabular-nums; }
	.pd-muted { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.pd-bad { color: var(--destructive) !important; font-size: 13px; }
	.pd-empty { padding: 20px; }
	.pd-list { padding: 0; list-style: none; overflow: hidden; }
	.pd-row { display: grid; grid-template-columns: 96px minmax(0, 1fr) auto; gap: 16px; padding: 16px 20px; align-items: start; }
	.pd-row + .pd-row { border-top: 1px solid var(--border); }
	.pd-skipped { opacity: 0.55; }
	.pd-when { display: flex; flex-direction: column; font-size: 13px; color: var(--foreground); }
	.pd-main { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
	.pd-media { font-size: 14px; font-weight: 600; color: var(--foreground); }
	.pd-topic { font-size: 14px; line-height: 22px; color: var(--foreground); word-break: break-word; }
	.pd-side { display: flex; flex-direction: column; align-items: flex-end; gap: 6px; }
	.pd-chip { padding: 4px 12px; border-radius: 999px; font-size: 12px; line-height: 16px; font-weight: 500; color: var(--muted-foreground); background: var(--muted); }
	.pd-chip-bad { color: var(--destructive); background: color-mix(in srgb, var(--destructive) 12%, transparent); }
	.pd-link { display: inline-flex; align-items: center; min-height: 32px; font-size: 13px; font-weight: 500; color: var(--primary); text-decoration: none; }
	.pd-link:hover { text-decoration: underline; }
	.pd-foot { padding: 16px 20px; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 16px; }
	.pd-check { display: flex; align-items: flex-start; gap: 12px; font-size: 14px; color: var(--foreground); min-width: 0; flex: 1 1 320px; }
	.pd-check input { margin-top: 4px; width: 18px; height: 18px; }
	.pd-check small { display: block; font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	@media (max-width: 760px) {
		.pd-tiles { grid-template-columns: 1fr; }
		.pd-row { grid-template-columns: 1fr; gap: 8px; }
		.pd-side { align-items: flex-start; flex-direction: row; flex-wrap: wrap; align-items: center; }
	}
</style>
