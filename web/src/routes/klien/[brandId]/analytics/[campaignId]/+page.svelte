<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Rincian hasil satu kampanye: tiap artikel terbit, medianya, kondisi tautan
	// menurut Link Guardian, dan masa garansi. Tanpa data klik atau pembaca.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ArrowLeft, ExternalLink, FileText } from '@lucide/svelte';
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

	const fmtDate = (v: string | null) =>
		v ? new Date(v).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' }) : '—';

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

	// Titik status: hijau tautan sehat, merah bermasalah, abu belum diperiksa.
	function linkLabel(p: Pub): { text: string; tone: 'good' | 'bad' | 'muted' } {
		if (p.status !== 'published') return { text: tr(statusText[p.status] ?? p.status), tone: 'muted' };
		if (p.problem_kind) return { text: tr(problemText[p.problem_kind] ?? 'Tautan bermasalah'), tone: 'bad' };
		if (p.link_state === 'live') return { text: tr('Tautan sehat'), tone: 'good' };
		if (p.link_state === 'unreachable') return { text: tr('Situs media belum bisa diperiksa'), tone: 'muted' };
		return { text: tr('Belum diperiksa'), tone: 'muted' };
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
	<a class="ad-back" href={`/klien/${brandUrl}/analytics`}>
		<ArrowLeft size={16} strokeWidth={1.5} />
		{tr('Analisis Kinerja')}
	</a>

	{#if error}
		<p class="app-card ad-error" role="alert">{error}</p>
	{/if}

	{#if loading}
		<p class="ad-loading">{tr('Memuat rincian kampanye...')}</p>
	{:else if data}
		<header>
			<h1 class="page-title">{data.campaign.name}</h1>
			<p class="page-desc">{tr('Hasil kampanye ini dari data yang tercatat di Tauthub.')}</p>
		</header>

		<dl class="app-card ad-summary">
			<div class="ad-cell">
				<dt>{tr('Artikel terbit')}</dt>
				<dd>{data.summary.published.toLocaleString('id-ID')}</dd>
			</div>
			<div class="ad-cell">
				<dt>{tr('Media')}</dt>
				<dd>{data.summary.media.toLocaleString('id-ID')}</dd>
			</div>
			<div class="ad-cell">
				<dt>{tr('Tautan sehat')}</dt>
				<dd class:is-good={data.summary.healthy > 0}>{data.summary.healthy.toLocaleString('id-ID')}</dd>
			</div>
			<div class="ad-cell">
				<dt>{tr('Tautan bermasalah')}</dt>
				<dd class:is-bad={data.summary.problem > 0}>{data.summary.problem.toLocaleString('id-ID')}</dd>
			</div>
		</dl>

		{#if data.publications.length === 0}
			<div class="app-card ad-empty">
				<span class="ad-empty-icon"><FileText size={20} strokeWidth={1.5} /></span>
				<p class="ad-empty-title">{tr('Belum ada artikel di kampanye ini.')}</p>
			</div>
		{:else}
			<div class="app-card ad-card">
				<div class="ad-cols ad-headrow" aria-hidden="true">
					<span>{tr('Artikel')}</span>
					<span>{tr('Tautan')}</span>
				</div>

				<ul class="ad-list">
					{#each data.publications as p (p.id)}
						{@const lk = linkLabel(p)}
						{@const warranty = warrantyLabel(p)}
						<li class="ad-cols ad-row">
							<div class="ad-main">
								<p class="ad-title">{p.document_title}</p>
								<p class="ad-meta">
									{p.media_name}{p.published_at ? ` · ${tr('Terbit')} ${fmtDate(p.published_at)}` : ''}
								</p>
								{#if p.published_url}
									<a class="ad-url" href={safeHref(p.published_url)} target="_blank" rel="noreferrer">
										{tr('Lihat artikel')}
										<ExternalLink size={13} strokeWidth={1.5} />
									</a>
								{/if}
							</div>

							<div class="ad-side">
								<p class="ad-state">
									<span class="ad-dot" class:is-good={lk.tone === 'good'} class:is-bad={lk.tone === 'bad'}></span>
									<span class:is-good={lk.tone === 'good'} class:is-bad={lk.tone === 'bad'}>{lk.text}</span>
								</p>
								{#if warranty}<p class="ad-meta">{warranty}</p>{/if}
								{#if p.last_checked_at}<p class="ad-meta">{tr('Diperiksa')} {fmtDate(p.last_checked_at)}</p>{/if}
							</div>
						</li>
					{/each}
				</ul>
			</div>
		{/if}

		<p class="ad-note">
			{tr('Tauthub tidak memantau klik atau perilaku pembaca. Kondisi tautan diperiksa otomatis oleh Link Guardian.')}
			<a href={`/klien/${brandUrl}/link-guardian`}>{tr('Buka Link Guardian')}</a>
			<a href={`/klien/${brandUrl}/campaigns`}>{tr('Berita acara dan bagikan laporan')}</a>
		</p>
	{/if}
</section>

<style>
	.ad {
		display: flex;
		flex-direction: column;
		gap: 24px;
	}
	.ad-back {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		min-height: 36px;
		align-self: flex-start;
		margin-bottom: -12px;
		font-size: 13px;
		color: var(--muted-foreground);
		text-decoration: none;
	}
	.ad-back:hover {
		color: var(--foreground);
	}
	.ad-error {
		margin: 0;
		padding: 12px 16px;
		font-size: 13px;
		line-height: 18px;
		border-color: color-mix(in srgb, var(--destructive) 40%, transparent);
		color: var(--destructive);
	}
	.ad-loading {
		font-size: 13px;
		color: var(--muted-foreground);
	}
	.ad-note {
		font-size: 12.5px;
		line-height: 20px;
		color: var(--muted-foreground);
	}
	.ad-note a {
		margin-left: 12px;
		color: var(--foreground);
		text-decoration: underline;
		text-underline-offset: 2px;
	}
	.ad-note a:hover {
		color: var(--primary);
	}

	/* ── Papan angka: satu bidang bergaris rambut ── */
	.ad-summary {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 1px;
		margin: 0;
		background-color: var(--border);
		overflow: hidden;
	}
	.ad-cell {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 20px;
		background-color: var(--card);
	}
	.ad-cell dt {
		font-size: 13px;
		line-height: 18px;
		color: var(--muted-foreground);
	}
	.ad-cell dd {
		margin: 0;
		font-size: 30px;
		line-height: 34px;
		font-weight: 500;
		letter-spacing: 0;
		color: var(--foreground);
		font-variant-numeric: tabular-nums;
	}
	.ad-cell dd.is-good {
		color: var(--success);
	}
	.ad-cell dd.is-bad {
		color: var(--destructive);
	}

	/* ── Daftar artikel ── */
	.ad-card {
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}
	.ad-cols {
		display: grid;
		grid-template-columns: minmax(0, 1.4fr) minmax(0, 1fr);
		gap: 24px;
	}
	.ad-headrow {
		padding: 10px 20px;
		border-bottom: 1px solid var(--border);
		font-size: 12px;
		line-height: 16px;
		color: var(--muted-foreground);
	}
	.ad-list {
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.ad-list li + li {
		border-top: 1px solid var(--border);
	}
	.ad-row {
		align-items: start;
		padding: 16px 20px;
	}
	.ad-main {
		display: flex;
		flex-direction: column;
		gap: 4px;
		min-width: 0;
	}
	.ad-title {
		font-size: 14px;
		line-height: 20px;
		font-weight: 500;
		color: var(--foreground);
		word-break: break-word;
	}
	.ad-meta {
		font-size: 12.5px;
		line-height: 18px;
		color: var(--muted-foreground);
	}
	.ad-url {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		align-self: flex-start;
		min-height: 32px;
		margin-top: 2px;
		font-size: 13px;
		color: var(--primary);
		text-decoration: underline;
		text-underline-offset: 2px;
	}
	.ad-side {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.ad-state {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		line-height: 18px;
		color: var(--muted-foreground);
	}
	.ad-dot {
		flex: 0 0 6px;
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background-color: var(--muted-foreground);
	}
	.ad-dot.is-good {
		background-color: var(--success);
	}
	.ad-dot.is-bad {
		background-color: var(--destructive);
	}
	.ad-state .is-good {
		color: var(--success);
	}
	.ad-state .is-bad {
		color: var(--destructive);
	}

	/* ── Kosong ── */
	.ad-empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 10px;
		padding: 40px 24px;
		text-align: center;
	}
	.ad-empty-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 44px;
		height: 44px;
		border-radius: 12px;
		background-color: var(--secondary);
		color: var(--muted-foreground);
	}
	.ad-empty-title {
		font-size: 13px;
		color: var(--muted-foreground);
	}

	@media (max-width: 860px) {
		.ad-summary {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
		.ad-cols {
			grid-template-columns: minmax(0, 1fr);
			gap: 12px;
		}
		.ad-headrow {
			display: none;
		}
	}
</style>
