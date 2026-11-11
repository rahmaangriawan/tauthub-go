<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Rincian hasil satu kampanye: tiap artikel terbit, medianya, kondisi tautan
	// menurut Link Guardian, dan masa garansi. Tanpa data klik atau pembaca.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ArrowLeft, CircleCheck, ExternalLink, TriangleAlert } from '@lucide/svelte';
	import { safeHref } from '$lib/safe-url';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	type Pub = {
		id: string;
		status: string;
		document_title: string;
		media_name: string;
		published_at: string | null;
		published_url: string | null;
		link_state: string | null;
		problem_kind: string | null;
		last_checked_at: string | null;
		warranty_until: string | null;
		in_warranty: boolean;
		claim_status: string | null;
	};
	type Detail = {
		campaign: { id: string; name: string };
		summary: { published: number; media: number; healthy: number; problem: number };
		publications: Pub[];
	};

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.campaignId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);

	let loading = $state(true);
	let error = $state('');
	let data = $state<Detail | null>(null);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			try {
				const res = await apiRequest(`/v1/brands/${brandId}/campaigns/${campaignId}/performance`);
				if (!res.ok) error = await readApiError(res, tr('Gagal memuat rincian kampanye.'));
				else data = (await res.json()) as Detail;
			} catch {
				error = tr('Gagal memuat rincian kampanye.');
			} finally {
				loading = false;
			}
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	const fmtDate = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '-');

	const statusText: Record<string, string> = {
		published: 'Terbit', waiting_partner: 'Menunggu media', partner_approved: 'Disetujui media', queued: 'Dalam antrean',
		processing: 'Diproses', pending: 'Menunggu', retrying: 'Mencoba ulang', failed: 'Gagal', partner_rejected: 'Ditolak media'
	};
	const problemText: Record<string, string> = {
		gone: 'Artikel tidak ditemukan lagi',
		link_removed: 'Tautan di dalam artikel dicabut',
		noindex: 'Artikel ditandai noindex',
		rel_changed: 'Atribut tautan berubah'
	};

	function linkLabel(p: Pub): { text: string; bad: boolean } {
		if (p.status !== 'published') return { text: tr(statusText[p.status] ?? p.status), bad: false };
		if (p.problem_kind) return { text: tr(problemText[p.problem_kind] ?? 'Tautan bermasalah'), bad: true };
		if (p.link_state === 'live') return { text: tr('Tautan sehat'), bad: false };
		if (p.link_state === 'unreachable') return { text: tr('Situs media belum bisa diperiksa'), bad: false };
		return { text: tr('Belum diperiksa'), bad: false };
	}
	function warrantyLabel(p: Pub): string {
		if (p.claim_status === 'refunded') return tr('Garansi dipakai: kredit dikembalikan');
		if (p.claim_status === 'replaced') return tr('Garansi dipakai: artikel diganti');
		if (p.claim_status) return tr('Klaim garansi sedang diproses');
		if (p.status !== 'published') return '';
		if (!p.in_warranty) return tr('Masa garansi berakhir');
		return p.warranty_until ? `${tr('Garansi sampai')} ${fmtDate(p.warranty_until)}` : tr('Garansi tanpa batas waktu');
	}
</script>

<PageTitle title={data?.campaign.name ?? tr('Analisis Kinerja')} />

<section class="ad">
	<a class="ad-back" href={`/klien/${brandUrl}/analytics`}><ArrowLeft size={15} />{tr('Analisis Kinerja')}</a>

	{#if error}
		<div class="app-card ad-error" role="alert">{error}</div>
	{/if}

	{#if loading}
		<div class="app-card ad-loading">{tr('Memuat rincian kampanye…')}</div>
	{:else if data}
		<header>
			<h1 class="page-title">{data.campaign.name}</h1>
			<p class="page-desc">{tr('Hasil kampanye ini dari data yang tercatat di Tauthub.')}</p>
		</header>

		<dl class="ad-tiles">
			<div class="app-card ad-tile"><dt>{tr('Artikel terbit')}</dt><dd>{data.summary.published}</dd></div>
			<div class="app-card ad-tile"><dt>{tr('Media')}</dt><dd>{data.summary.media}</dd></div>
			<div class="app-card ad-tile"><dt>{tr('Tautan sehat')}</dt><dd>{data.summary.healthy}</dd></div>
			<div class="app-card ad-tile"><dt>{tr('Tautan bermasalah')}</dt><dd class:ad-bad={data.summary.problem > 0}>{data.summary.problem}</dd></div>
		</dl>

		{#if data.publications.length === 0}
			<div class="app-card ad-empty"><p class="ad-muted">{tr('Belum ada artikel di kampanye ini.')}</p></div>
		{:else}
			<ul class="app-card ad-list">
				{#each data.publications as p (p.id)}
					{@const lk = linkLabel(p)}
					<li class="ad-row">
						<div class="ad-main">
							<p class="ad-title">{p.document_title}</p>
							<p class="ad-muted">{p.media_name}{p.published_at ? `, ${tr('terbit')} ${fmtDate(p.published_at)}` : ''}</p>
							{#if p.published_url}
								<a class="ad-url" href={safeHref(p.published_url)} target="_blank" rel="noreferrer">{tr('Lihat artikel')}<ExternalLink size={13} /></a>
							{/if}
						</div>
						<div class="ad-side">
							<p class="ad-state" class:ad-bad={lk.bad}>
								{#if lk.bad}<TriangleAlert size={14} strokeWidth={1.5} />{:else if p.status === 'published' && p.link_state === 'live'}<CircleCheck size={14} strokeWidth={1.5} />{/if}
								{lk.text}
							</p>
							{#if warrantyLabel(p)}<p class="ad-muted">{warrantyLabel(p)}</p>{/if}
							{#if p.last_checked_at}<p class="ad-note">{tr('Diperiksa')} {fmtDate(p.last_checked_at)}</p>{/if}
						</div>
					</li>
				{/each}
			</ul>
		{/if}

		<div class="ad-links">
			<a href={`/klien/${brandUrl}/link-guardian`}>{tr('Buka Link Guardian')}</a>
			<a href={`/klien/${brandUrl}/campaigns`}>{tr('Berita acara dan bagikan laporan')}</a>
		</div>
		<p class="ad-note">{tr('Tauthub tidak memantau klik atau perilaku pembaca. Kondisi tautan diperiksa otomatis oleh Link Guardian.')}</p>
	{/if}
</section>

<style>
	.ad { display: flex; flex-direction: column; gap: 20px; }
	.ad-back { display: inline-flex; align-items: center; gap: 8px; min-height: 44px; align-self: flex-start; font-size: 13px; font-weight: 500; color: var(--primary); text-decoration: none; }
	.ad-back:hover { text-decoration: underline; }
	.ad-error { padding: 12px 16px; font-size: 14px; border-color: var(--destructive); color: var(--destructive); }
	.ad-loading { padding: 20px; text-align: center; font-size: 14px; color: var(--muted-foreground); }
	.ad-muted { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.ad-note { font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	.ad-title { font-size: 14px; font-weight: 600; line-height: 22px; color: var(--foreground); word-break: break-word; }
	.ad-bad { color: var(--destructive) !important; }
	.ad-tiles { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px; }
	.ad-tile { padding: 16px 20px; display: flex; flex-direction: column; gap: 4px; }
	.ad-tile dt { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.ad-tile dd { font-size: 28px; line-height: 36px; font-weight: 600; letter-spacing: -0.02em; color: var(--foreground); font-variant-numeric: tabular-nums; }
	.ad-empty { padding: 20px; }
	.ad-list { padding: 0; list-style: none; overflow: hidden; }
	.ad-row { display: grid; grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr); gap: 20px; padding: 16px 20px; }
	.ad-row + .ad-row { border-top: 1px solid var(--border); }
	.ad-main { min-width: 0; display: flex; flex-direction: column; gap: 2px; }
	.ad-url { display: inline-flex; align-items: center; gap: 6px; min-height: 32px; font-size: 13px; font-weight: 500; color: var(--primary); text-decoration: none; }
	.ad-url:hover { text-decoration: underline; }
	.ad-side { display: flex; flex-direction: column; gap: 2px; }
	.ad-state { display: flex; align-items: center; gap: 8px; font-size: 13px; line-height: 20px; font-weight: 500; color: var(--success); }
	.ad-links { display: flex; flex-wrap: wrap; gap: 8px 24px; }
	.ad-links a { display: inline-flex; align-items: center; min-height: 44px; font-size: 13px; font-weight: 500; color: var(--primary); text-decoration: none; }
	.ad-links a:hover { text-decoration: underline; }
	@media (max-width: 860px) {
		.ad-tiles { grid-template-columns: repeat(2, minmax(0, 1fr)); }
		.ad-row { grid-template-columns: 1fr; gap: 8px; }
	}
</style>
