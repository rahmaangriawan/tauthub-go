<script lang="ts">
	// Tahap 4 — Distribusi (parity blok `flowStatus === "distributing"`).
	import { CircleCheck, FileText, Plus, RefreshCw, TriangleAlert } from '@lucide/svelte';
	import type { FlowState } from './_flow-state.svelte';
	import DewanPers from './_dewan-pers.svelte';

	let { flow }: { flow: FlowState } = $props();

	const isPub = (item: { publication_status: string | null; status: string }) =>
		item.publication_status === 'published' || item.status === 'published';
	const isFail = (item: { publication_status: string | null; status: string }) =>
		item.publication_status === 'failed' || item.status === 'failed';

	const publishedCount = $derived(flow.mediaItems.filter(isPub).length);
	const failedCount = $derived(flow.mediaItems.filter(isFail).length);
	const processingCount = $derived(
		flow.mediaItems.filter(
			(m) =>
				(m.status === 'processing' ||
					(!!m.publication_status && m.publication_status !== 'published' && m.publication_status !== 'failed')) &&
				m.publication_status !== 'published' &&
				m.status !== 'published' &&
				m.publication_status !== 'failed' &&
				m.status !== 'failed'
		).length
	);
	const totalCount = $derived(flow.mediaItems.length);
	const publishedPercent = $derived(totalCount > 0 ? (publishedCount / totalCount) * 100 : 0);
	const processingPercent = $derived(totalCount > 0 ? (processingCount / totalCount) * 100 : 0);
</script>

<div class="stage-header">
	{#if flow.campaign}<div class="stage-eyebrow">Kampanye: {flow.campaign.name}</div>{/if}
	<h1 class="stage-title">Distribusi</h1>
	<p class="stage-desc">Pantau status dan tautan publikasi artikel ke seluruh media tujuan secara langsung.</p>
</div>

<div style="display:grid;grid-template-columns:repeat(3,1fr);gap:14px;margin:20px 0;">
	<div class="review-metric-card">
		<div class="review-metric-header">
			<span class="review-metric-label">Published</span>
			<CircleCheck size={18} style="color:var(--success);" />
		</div>
		<div class="review-metric-value review-metric-value-green">{publishedCount}</div>
		<div class="review-metric-sub">artikel berhasil terbit</div>
	</div>

	<div class="review-metric-card">
		<div class="review-metric-header">
			<span class="review-metric-label">Processing</span>
			<RefreshCw size={18} class="spin-icon" style="color:var(--warning);" />
		</div>
		<div class="review-metric-value" style="color:var(--warning);">{processingCount}</div>
		<div class="review-metric-sub">dalam antrean / diproses</div>
	</div>

	<div class="review-metric-card">
		<div class="review-metric-header">
			<span class="review-metric-label">Failed</span>
			<TriangleAlert size={18} style="color:var(--destructive);" />
		</div>
		<div class="review-metric-value" style="color:{failedCount > 0 ? 'var(--destructive)' : 'var(--muted-foreground)'};">{failedCount}</div>
		<div class="review-metric-sub">perlu penanganan</div>
	</div>
</div>

{#if totalCount > 0}
	<div class="dist-progress-section">
		<div class="dist-progress-header">
			<div class="dist-progress-title">
				<span>Progres Distribusi</span>
				{#if processingCount > 0}
					<span class="dist-live-pulse"><span class="pulse-dot"></span><span>Live Processing</span></span>
				{/if}
			</div>
			<span class="dist-progress-ratio">{publishedCount} dari {totalCount} media terbit</span>
		</div>
		<div class="dist-progress-bar-track">
			<div class="dist-progress-bar-fill" style="width:{publishedPercent}%;"></div>
			<div class="dist-progress-bar-processing" style="width:{processingPercent}%;"></div>
		</div>
		<div class="dist-progress-footer">
			<span>
				{processingCount > 0
					? 'Sistem sedang memproses publikasi artikel, Anda cukup menunggu…'
					: publishedCount === totalCount
						? '✓ Seluruh artikel telah sukses terbit ke media tujuan.'
						: 'Distribusi telah selesai diproses.'}
			</span>
			<span>{Math.round(publishedPercent)}% Selesai</span>
		</div>
	</div>
{/if}

<div class="distribution-section">
	<div class="distribution-section-header">
		<div class="distribution-section-title-group">
			<h3 class="distribution-section-title">Daftar Publikasi Media</h3>
			<span class="distribution-count-badge">{totalCount} media</span>
		</div>
	</div>

	{#if flow.mediaItems.length > 0}
		<div class="dist-table-head">
			<div>Media</div>
			<div>Artikel</div>
			<div>Status Publikasi</div>
			<div style="text-align:right;">Aksi &amp; Tautan</div>
		</div>
		<div class="distribution-list">
			{#each flow.mediaItems as item (item.id)}
				{@const doc = item.variant_document ?? item.source_document}
				{@const pub = isPub(item)}
				{@const fail = isFail(item)}
				<div class="dist-row">
					<div class="distribution-media-col">
						<span class="distribution-media-name" title={item.media_name ?? ''}>
							<span>{item.media_name}</span>
							<DewanPers verified={item.media_dewan_pers_verified} size={10} />
						</span>
						<span class="distribution-media-cat">{item.media_category ?? 'Media Partner'}</span>
						{#if item.publication_order_code}
							<span class="distribution-order-code" title="Nomor order publikasi ke media ini">{item.publication_order_code}</span>
						{/if}
					</div>

					<div class="distribution-article-col">
						<div class="distribution-article-title" title={doc?.title ?? ''}>
							<FileText size={15} style="flex-shrink:0;color:var(--muted-foreground);" />
							<span class="distribution-article-text">{doc?.title ?? 'Naskah artikel'}</span>
						</div>
						{#if item.variant_document?.title && item.source_document?.title && item.variant_document.title !== item.source_document.title}
							<div class="distribution-article-angle">Varian Artikel Khusus</div>
						{:else if item.source_document?.title || item.source_document_id}
							<div class="distribution-article-original">Artikel Original</div>
						{/if}
					</div>

					<div class="distribution-status-col">
						{#if pub}
							<span class="badge-success"><CircleCheck size={13} /><span>Published</span></span>
						{:else if fail}
							<span class="badge-danger"><TriangleAlert size={13} /><span>Failed</span></span>
						{:else}
							<span class="badge-processing"><RefreshCw size={13} class="spin-icon" /><span>Processing…</span></span>
						{/if}
					</div>

					<div class="dist-action-col">
						{#if item.publication_id}
							<a href={`/klien/${flow.segment}/publications?pub=${item.publication_id}`} class="dist-detail-btn">
								<span>Detail publikasi</span>
							</a>
						{/if}
						{#if item.publication_url}
							<a href={item.publication_url} target="_blank" rel="noopener noreferrer" class="dist-pub-link">
								<span>Lihat artikel terbit</span>
								<span>↗</span>
							</a>
						{/if}
						{#if item.tracking_code}
							<a href={`/t/${item.tracking_code}`} target="_blank" rel="noopener noreferrer" class="dist-tracking-link">
								/t/{item.tracking_code}
							</a>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{:else}
		<div class="empty" style="padding:24px;">Belum ada data distribusi.</div>
	{/if}
</div>

<div class="dist-monitoring-bar">
	<div class="dist-monitoring-info">
		<span class="dist-live-pulse"><span class="pulse-dot"></span><span>Auto-refresh aktif (setiap 5 detik)</span></span>
		<span class="dist-polling-pill">pembaruan #{flow.distPoll}</span>
	</div>
	<button class="dist-refresh-btn" type="button" disabled={flow.loading} onclick={() => void flow.loadFlow()}>
		<RefreshCw size={14} class={flow.loading ? 'spin-icon' : ''} />
		<span>{flow.loading ? 'Menyegarkan…' : 'Segarkan Sekarang'}</span>
	</button>
</div>

<div style="display:flex;align-items:center;justify-content:space-between;gap:12px;margin-top:20px;flex-wrap:wrap;">
	<button type="button" class="secondary-action-btn" disabled={flow.busy} onclick={() => void flow.gotoStage('final_review')}>
		← Kembali ke Review &amp; Checkout
	</button>
	<button type="button" class="primary-action-btn" disabled={flow.busy} onclick={() => void flow.startNewArticleFlow()}>
		<Plus size={15} />
		<span>{flow.busy ? 'Menyiapkan…' : 'Buat Artikel Baru di Kampanye Ini'}</span>
	</button>
</div>
