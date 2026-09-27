<script lang="ts">
	// Tahap 2 — Media & Artikel (parity blok `flowStatus === "media_selection"`).
	import {
		ChevronDown,
		ChevronUp,
		CircleCheck,
		CircleCheckBig,
		FileText,
		Pencil,
		RefreshCw,
		Trash,
		TriangleAlert,
		Upload
	} from '@lucide/svelte';
	import { thumbOnError, type CampaignMediaItem } from './_flow-state.svelte';
	import type { FlowState } from './_flow-state.svelte';
	import DewanPers from './_dewan-pers.svelte';
	import AngleModal from './_angle-modal.svelte';
	import PostEditor from './_post-editor.svelte';

	let { flow }: { flow: FlowState } = $props();

	function articleTitleFor(item: CampaignMediaItem | undefined) {
		return (
			item?.variant_document?.title ||
			item?.source_document?.title ||
			flow.intake?.topic ||
			flow.campaign?.name
		);
	}

	function standardClass(status: string) {
		return status === 'fail' ? 'standard-alert standard-fail' : status === 'warning' ? 'standard-alert standard-warn' : 'standard-alert standard-pass';
	}

	async function pickThumb(itemId: string, event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		await flow.uploadThumbnail(itemId, file);
		input.value = '';
	}
</script>

<h2 class="card-title">Media &amp; Artikel</h2>
<p class="card-hint">Pilih media tujuan dan periksa status artikel</p>

<div class="media-layout">
	<!-- KIRI — pemilihan media -->
	<div>
		{#if flow.recommendations.length > 0}
			<div class="recommendation-box">
				<div class="recommendation-header" onclick={() => (flow.recommendationsExpanded = !flow.recommendationsExpanded)} role="presentation">
					<div class="recommendation-header-left">
						<h3 class="recommendation-title">Media yang direkomendasikan Tauthub</h3>
						<span class="recommendation-count-badge">{flow.recommendations.length} rekomendasi</span>
					</div>
					<div class="recommendation-header-actions" onclick={(e) => e.stopPropagation()} role="presentation">
						<button
							type="button"
							class="recommendation-shuffle-btn"
							onclick={() => void flow.loadRecommendations()}
							disabled={flow.recommendBusy}
							title="Acak variasi rekomendasi media"
						>
							{flow.recommendBusy ? 'Mengacak…' : '🔄 Acak Rekomendasi'}
						</button>
						<button
							type="button"
							class="recommendation-toggle-btn"
							onclick={() => (flow.recommendationsExpanded = !flow.recommendationsExpanded)}
						>
							{#if flow.recommendationsExpanded}
								<span>Tutup</span>
								<ChevronUp size={13} />
							{:else}
								<span>Buka</span>
								<ChevronDown size={13} />
							{/if}
						</button>
					</div>
				</div>

				{#if flow.recommendationsExpanded}
					<div class="recommendation-body">
						{#each flow.recommendations as rec (rec.id)}
							{@const chosen = flow.selectedMedia.has(rec.id)}
							{@const unavailable = rec.is_available === false}
							<div
								aria-disabled={unavailable}
								style="padding:10px 12px;border:1px solid var(--border);border-radius:var(--radius-control);background:{unavailable
									? 'var(--muted)'
									: 'var(--card)'};opacity:{unavailable ? 0.62 : 1};"
							>
								<div style="display:flex;justify-content:space-between;gap:10px;align-items:center;">
									<div style="min-width:0;">
										<div class="name-with-badge" style="font-size:13px;font-weight:600;">
											<span>{rec.name}</span><DewanPers verified={rec.dewan_pers_verified} size={10} />
										</div>
										<div class="row-meta">
											{rec.category ?? '—'} · {rec.credit_cost.toLocaleString('id-ID')} TC · skor {Math.round(rec.score)}/100
										</div>
										{#if rec.reasons.length > 0}
											<div class="row-meta">{rec.reasons.slice(0, 2).join(' · ')}</div>
										{/if}
									</div>
									{#if unavailable}
										<span class="badge">Menunggu Review Admin</span>
									{:else if chosen}
										<span class="badge" style="background:var(--success-soft);color:var(--success);">Terpilih</span>
									{:else}
										<button type="button" class="select-media-ghost-btn" onclick={() => void flow.toggleMedia(rec.id)}>
											Gunakan media ini
										</button>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		{/if}
		{#if flow.recommendBusy}<p class="card-hint">Mencari media yang cocok…</p>{/if}

		<input class="app-input" type="text" placeholder="Cari media…" bind:value={flow.mediaSearch} style="margin-bottom:12px;" />

		<div class="media-list-container">
			{#each flow.filteredMedia as m (m.id)}
				{@const isSelected = flow.selectedMedia.has(m.id)}
				{@const unavailable = m.is_available === false}
				{@const item = flow.mediaItems.find((mi) => mi.media_id === m.id)}
				{@const articleTitle = articleTitleFor(item)}
				<div
					aria-disabled={unavailable}
					class="media-card{isSelected ? ' media-card-selected' : ''}{unavailable ? ' media-card-unavailable' : ''}"
				>
					<div class="media-card-header">
						<div class="media-card-identity">
							<input
								type="checkbox"
								id="media-{m.id}"
								checked={isSelected}
								disabled={unavailable}
								onchange={() => void flow.toggleMedia(m.id)}
								class="media-checkbox"
							/>
							<div class="media-name-group">
								<div class="media-name-row">
									<label for="media-{m.id}" class="media-name">{m.name}</label>
									<DewanPers verified={m.dewan_pers_verified} size={10} />
								</div>
								<div class="media-meta">
									{#if m.category}<span>{m.category}</span>{/if}
									{#if m.category}<span class="meta-dot">·</span>{/if}
									<span class="meta-cost">{m.credit_cost.toLocaleString('id-ID')} TC</span>
									{#if m.health_score}
										<span class="meta-dot">·</span>
										<span class="meta-score">Skor {m.health_score}</span>
									{/if}
								</div>
							</div>
						</div>

						<div class="media-card-status-group">
							{#if item}
								{#if item.status === 'article_ready'}
									<span class="badge-success"><CircleCheck size={13} /><span>Artikel siap</span></span>
								{/if}
								{#if item.status === 'needs_rewrite'}
									<span class="badge-warning"><TriangleAlert size={13} /><span>Perlu rewrite</span></span>
								{/if}
								{#if item.status === 'rewriting' || item.status === 'angle_select'}
									<span class="badge-processing"><RefreshCw size={13} class="spin-icon" /><span>Memproses…</span></span>
								{/if}
								<button
									type="button"
									class="delete-media-btn"
									disabled={flow.busy}
									onclick={() => void flow.removeItem(item.id)}
									title="Hapus media dari alur kampanye"
								>
									<Trash size={13} />
									<span>Hapus</span>
								</button>
							{:else if unavailable}
								<span class="badge">Menunggu Review Admin</span>
							{:else}
								<button type="button" class="select-media-ghost-btn" onclick={() => void flow.toggleMedia(m.id)}>+ Pilih media</button>
							{/if}
						</div>
					</div>

					{#if item}
						<div class="media-card-body">
							{#if articleTitle}
								<div class="media-article-context">
									<FileText size={14} class="context-icon" />
									<span class="context-title" title={articleTitle}>{articleTitle}</span>
									{#if item.variant_document?.title && item.source_document?.title && item.variant_document.title !== item.source_document.title}
										<span class="variant-badge">Varian</span>
									{:else if item.source_document?.title || item.source_document_id}
										<span class="original-badge">Original</span>
									{/if}
								</div>
							{/if}

							{#if flow.standardResults[item.media_id]}
								{@const sr = flow.standardResults[item.media_id]}
								<div class={standardClass(sr.status)}>
									{#if sr.status === 'pass'}
										<span>✓ Standar media terpenuhi</span>
									{:else if sr.status === 'warning'}
										<span>⚠ Ada peringatan standar</span>
									{:else}
										<span>✕ Standar belum terpenuhi: {(sr.failed ?? []).slice(0, 2).join('; ')}</span>
									{/if}
								</div>
							{/if}

							<div class="media-card-footer">
								<div class="thumbnail-control-group">
									<span class="thumbnail-label">Thumbnail</span>
									<div class="thumbnail-input-wrapper">
										<input
											type="url"
											class="thumbnail-url-input"
											value={item.thumbnail_url ?? ''}
											oninput={(e) => flow.setThumbnailLocal(item.id, e.currentTarget.value)}
											onblur={(e) => void flow.saveThumbnail(item.id, e.currentTarget.value)}
											placeholder="URL thumbnail..."
										/>
										<label class="thumbnail-upload-btn" title="Upload gambar thumbnail">
											<Upload size={13} />
											<span>Upload</span>
											<input
												type="file"
												accept="image/*"
												class="hidden-file-input"
												onchange={(e) => void pickThumb(item.id, e)}
											/>
										</label>
										{#if flow.uploadingThumbId === item.id}<span class="thumbnail-uploading">Mengunggah…</span>{/if}
									</div>
									{#if item.thumbnail_url}
										<img src={item.thumbnail_url} width="32" height="32" alt="Thumbnail preview" class="thumbnail-preview-img" onerror={thumbOnError} />
									{/if}
									{#if flow.thumbUploadError}<span class="thumbnail-upload-error">{flow.thumbUploadError}</span>{/if}
								</div>

								<div class="media-actions-group">
									{#if item.status === 'needs_rewrite'}
										<button
											type="button"
											class="secondary-action-btn"
											disabled={!!flow.paraphraseBusy}
											onclick={(e) => {
												e.preventDefault();
												void flow.paraphraseArticle(item.id);
											}}
										>
											{flow.paraphraseBusy === item.id ? 'Memparafrase…' : 'Parafrase Naskah'}
										</button>
										<button
											type="button"
											class="primary-action-btn"
											disabled={!!flow.paraphraseBusy}
											onclick={(e) => {
												e.preventDefault();
												void flow.openAngleModal(item.id);
											}}
										>
											Buat Angle Berbeda
										</button>
									{/if}

									{#if item.status === 'article_ready' && (item.variant_document_id ?? item.source_document_id ?? flow.doc?.id)}
										<button
											type="button"
											class="primary-action-btn"
											onclick={(e) => {
												e.preventDefault();
												void flow.openPostEditor(item);
											}}
										>
											<Pencil size={13} />
											<span>Edit Post (Isi, Judul &amp; Gambar)</span>
										</button>
									{/if}
								</div>
							</div>
						</div>
					{/if}
				</div>
			{/each}
			{#if flow.filteredMedia.length === 0}<div class="empty">Media tidak ditemukan.</div>{/if}
		</div>
	</div>

	<!-- KANAN — ringkasan -->
	<div class="sidebar-summary">
		<h3 class="summary-title">Ringkasan</h3>
		<div class="summary-rows">
			<div class="summary-row"><span>Media dipilih:</span><strong>{flow.selectedMedia.size}</strong></div>
			<div class="summary-row"><span>Artikel siap:</span><strong style="color:var(--success);">{flow.itemsReady}</strong></div>
			<div class="summary-row">
				<span>Perlu rewrite:</span>
				<strong style="color:{flow.itemsNeedsRewrite > 0 ? 'var(--warning)' : 'var(--foreground)'};">{flow.itemsNeedsRewrite}</strong>
			</div>
			<div class="summary-row"><span>Diproses:</span><strong style="color:var(--warning);">{flow.itemsProcessing}</strong></div>
			<div class="summary-row summary-sep">
				<span>Total TC:</span><strong>{flow.totalTc.toLocaleString('id-ID')}</strong>
			</div>
			<div class="summary-row">
				<span>Saldo TC:</span>
				<strong style="color:{flow.wallet && flow.totalTc > (flow.wallet?.available_tc ?? 0) ? 'var(--destructive)' : 'var(--success)'};">
					{flow.wallet ? flow.wallet.available_tc.toLocaleString('id-ID') : '—'}
				</strong>
			</div>
		</div>

		{#if flow.standardSummary}
			{@const ss = flow.standardSummary}
			<div
				class="sidebar-standard-summary{ss.type === 'pass'
					? ' sidebar-standard-pass'
					: ss.type === 'warn'
						? ' sidebar-standard-warn'
						: ' sidebar-standard-fail'}"
			>
				{#if ss.type === 'pass'}<CircleCheckBig size={14} />{:else}<TriangleAlert size={14} />{/if}
				<span>{ss.text}</span>
			</div>
		{/if}

		<button
			class="app-btn app-btn-soft"
			type="button"
			style="width:100%;margin-top:{flow.standardSummary ? 8 : 12}px;"
			disabled={flow.busy || flow.selectedMedia.size === 0 || !flow.activeDocId}
			onclick={() => void flow.checkStandards()}
		>
			{flow.busy ? 'Memproses…' : 'Cek standar media'}
		</button>
		<button
			class="app-btn app-btn-primary"
			type="button"
			style="width:100%;margin-top:8px;"
			disabled={flow.busy || flow.mediaItems.length === 0 || (!flow.allReady && !flow.doc?.id) || flow.itemsProcessing > 0}
			onclick={() => void flow.advanceToReview()}
		>
			{flow.busy ? 'Memproses…' : 'Lanjut ke Review & Checkout →'}
		</button>
		<button
			type="button"
			class="app-btn app-btn-soft"
			style="width:100%;margin-top:8px;"
			disabled={flow.busy}
			onclick={() => void flow.gotoStage('content')}
		>
			← Kembali ke Content
		</button>
	</div>
</div>

<AngleModal {flow} />
<PostEditor {flow} />
