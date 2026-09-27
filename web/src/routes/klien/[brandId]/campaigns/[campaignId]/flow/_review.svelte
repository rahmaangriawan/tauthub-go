<script lang="ts">
	// Tahap 3 — Review & Checkout (parity blok `flowStatus === "final_review"`).
	import { ArrowRight, CircleCheck, Coins, FileText, Globe, RefreshCw, TriangleAlert, Wallet } from '@lucide/svelte';
	import type { FlowState } from './_flow-state.svelte';
	import DewanPers from './_dewan-pers.svelte';

	let { flow }: { flow: FlowState } = $props();

	const insufficient = $derived(!!flow.wallet && flow.totalTc > (flow.wallet?.available_tc ?? 0));
</script>

<div class="stage-header">
	{#if flow.campaign}<div class="stage-eyebrow">Kampanye: {flow.campaign.name}</div>{/if}
	<h1 class="stage-title">Review &amp; Checkout</h1>
	<p class="stage-desc">Periksa kembali seluruh media, artikel, dan penggunaan TC sebelum melanjutkan distribusi.</p>
</div>

<div class="review-metrics-grid">
	<div class="review-metric-card">
		<div class="review-metric-header">
			<span class="review-metric-label">Total Media</span>
			<Globe size={18} class="review-metric-icon" />
		</div>
		<div class="review-metric-value">{flow.mediaItems.length}</div>
		<div class="review-metric-sub">media terpilih</div>
	</div>

	<div class="review-metric-card">
		<div class="review-metric-header">
			<span class="review-metric-label">Total Artikel</span>
			<FileText size={18} class="review-metric-icon" />
		</div>
		<div class="review-metric-value">{flow.itemsReady}</div>
		<div class="review-metric-sub">artikel siap kirim</div>
	</div>

	<div class="review-metric-card review-metric-card-blue">
		<div class="review-metric-header">
			<span class="review-metric-label">Total Penggunaan</span>
			<Coins size={18} class="review-metric-icon" />
		</div>
		<div class="review-metric-value review-metric-value-blue">
			{flow.totalTc.toLocaleString('id-ID')} <span style="font-size:13px;font-weight:500;">TC</span>
		</div>
		<div class="review-metric-sub">biaya distribusi</div>
	</div>

	<div class="review-metric-card{!insufficient && flow.wallet ? ' review-metric-card-highlight' : ''}">
		<div class="review-metric-header">
			<span class="review-metric-label">Saldo TC Anda</span>
			<Wallet size={18} class="review-metric-icon" />
		</div>
		<div class="review-metric-value{insufficient ? '' : ' review-metric-value-green'}" style={insufficient ? 'color:var(--destructive);' : ''}>
			{flow.wallet?.available_tc.toLocaleString('id-ID') ?? '—'} <span style="font-size:13px;font-weight:500;">TC</span>
		</div>
		<div class="review-metric-sub {insufficient ? 'review-metric-sub-negative' : 'review-metric-sub-positive'}">
			{insufficient ? '✕ Saldo tidak mencukupi' : '✓ Saldo mencukupi'}
		</div>
	</div>
</div>

<div class="distribution-section">
	<div class="distribution-section-header">
		<div class="distribution-section-title-group">
			<h3 class="distribution-section-title">Ringkasan Distribusi</h3>
			<span class="distribution-count-badge">{flow.mediaItems.length} media</span>
		</div>
	</div>

	{#if flow.mediaItems.length > 0}
		<div class="distribution-table-head">
			<div>Media</div>
			<div>Artikel</div>
			<div>Biaya Distribusi</div>
			<div>Status</div>
		</div>
		<div class="distribution-list">
			{#each flow.mediaItems as item (item.id)}
				{@const doc = item.variant_document ?? item.source_document}
				<div class="distribution-row">
					<div class="distribution-media-col">
						<span class="distribution-media-name" title={item.media_name ?? ''}>
							<span>{item.media_name}</span>
							<DewanPers verified={item.media_dewan_pers_verified} size={10} />
						</span>
						<span class="distribution-media-cat">{item.media_category ?? '—'}</span>
					</div>

					<div class="distribution-article-col">
						<div class="distribution-article-title" title={doc?.title ?? ''}>
							<FileText size={15} style="flex-shrink:0;color:var(--muted-foreground);" />
							<span class="distribution-article-text">{doc?.title ?? 'Belum ada naskah artikel'}</span>
						</div>
						{#if item.angle_label}<div class="distribution-article-angle">Angle: {item.angle_label}</div>{/if}
					</div>

					<div class="distribution-cost-col">
						<div class="distribution-cost-badge">
							<Coins size={13} />
							<span>{item.tc_cost.toLocaleString('id-ID')} TC</span>
						</div>
					</div>

					<div class="distribution-status-col">
						{#if item.status === 'article_ready'}
							<span class="badge-success"><CircleCheck size={13} /><span>Siap distribusi</span></span>
						{:else if item.status === 'needs_rewrite'}
							<span class="badge-warning"><TriangleAlert size={13} /><span>Perlu perbaikan</span></span>
						{:else}
							<span class="badge-processing"><RefreshCw size={13} class="spin-icon" /><span>Memproses…</span></span>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{:else}
		<div class="empty" style="padding:24px;">Belum ada media dipilih.</div>
	{/if}
</div>

<div class="checkout-confirm-section">
	<div class="checkout-confirm-content">
		<div class="checkout-confirm-left">
			<h4 class="checkout-confirm-title">Konfirmasi Sebelum Checkout</h4>
			<p class="checkout-confirm-desc">
				Pastikan seluruh artikel, media tujuan, dan penggunaan TC sudah sesuai sebelum memulai distribusi.
			</p>
			<label class="checkout-checkbox-label">
				<input type="checkbox" bind:checked={flow.agreed} class="checkout-checkbox-input" />
				<span>Saya telah memeriksa artikel, media tujuan, dan total penggunaan TC.</span>
			</label>
		</div>
		<button
			class="checkout-cta-button"
			type="button"
			disabled={!flow.agreed || flow.checkoutBusy}
			onclick={() => void flow.checkout()}
		>
			{#if flow.checkoutBusy}
				Memproses…
			{:else}
				<span>Checkout &amp; Distribusikan</span>
				<ArrowRight size={16} />
			{/if}
		</button>
	</div>
</div>

<div style="display:flex;align-items:center;justify-content:flex-start;gap:12px;margin-top:20px;">
	<button
		type="button"
		class="secondary-action-btn"
		disabled={flow.checkoutBusy || flow.busy}
		onclick={() => void flow.gotoStage('media_selection')}
	>
		← Kembali ke Media &amp; Artikel
	</button>
</div>
