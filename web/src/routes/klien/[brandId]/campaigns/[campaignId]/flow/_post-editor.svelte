<script lang="ts">
	// Modal Editor Post (WYSIWYG judul, thumbnail, konten) — parity blok
	// `editingPostItem` app lama.
	import { Bold, Italic, Link, List, ListOrdered, Quote, RefreshCw, Save, Strikethrough, Underline, Upload, X } from '@lucide/svelte';
	import { countWords, thumbOnError, type DocumentVersion } from './_flow-state.svelte';
	import type { FlowState } from './_flow-state.svelte';

	let { flow }: { flow: FlowState } = $props();

	let canvasEl = $state<HTMLDivElement | null>(null);
	let words = $state(0);
	let fileEl = $state<HTMLInputElement | null>(null);

	const latestVersion = $derived(flow.editPostVersions.length ? Math.max(...flow.editPostVersions.map((v) => v.version)) : 0);

	// Isi kanvas dengan HTML awal setelah dokumen dimuat.
	$effect(() => {
		const html = flow.editPostHtml;
		const el = canvasEl;
		if (!el || flow.editPostLoading) return;
		if (el.innerHTML !== html) el.innerHTML = html;
		sync();
	});

	function sync() {
		if (canvasEl) words = countWords(canvasEl.innerText || '');
	}

	function exec(cmd: string, arg?: string) {
		if (!canvasEl) return;
		canvasEl.focus();
		window.document.execCommand(cmd, false, arg);
		sync();
	}

	function viewVersion(version: number) {
		const ver = flow.editPostVersions.find((v) => v.version === version);
		if (!ver) return;
		flow.editPostActiveVersion = version;
		if (canvasEl) {
			canvasEl.innerHTML = ver.html ?? '';
			sync();
		}
	}

	async function uploadThumb(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		const url = await flow.uploadAsset(file);
		if (url) flow.editPostThumbnail = url;
		input.value = '';
	}

	const versionTitle = (v: DocumentVersion) =>
		`${v.change_note ?? 'Versi ' + v.version}${v.created_at ? ' · ' + new Date(v.created_at).toLocaleString('id-ID') : ''}`;
</script>

{#if flow.editingPostItem}
	{@const postItem = flow.editingPostItem}
	<div class="overlay" role="dialog" aria-modal="true" aria-label="Edit Post Artikel">
		<div class="post-editor-modal">
			<div class="post-editor-header">
				<div style="min-width:0;">
					<span class="post-editor-header-title">Editor Post: {postItem.media_name || 'Artikel Media'}</span>
					<span class="post-editor-header-sub">Edit judul, gambar thumbnail, dan konten naskah sebelum dipublikasikan.</span>
				</div>
				<button type="button" class="modal-close" aria-label="Tutup" onclick={() => (flow.editingPostItem = null)}>
					<X size={16} />
				</button>
			</div>

			{#if flow.editPostVersions.length > 1}
				<div class="revision-bar">
					<span style="font-size:12px;font-weight:600;color:var(--muted-foreground);">Revisi:</span>
					{#each flow.editPostVersions as v (v.version)}
						{@const isLatest = v.version === latestVersion}
						{@const isActive = flow.editPostActiveVersion === v.version}
						<button
							type="button"
							class="revision-chip{isLatest ? ' revision-chip-latest' : ''}{isActive ? ' revision-chip-active' : ''}"
							title={versionTitle(v)}
							onclick={() => viewVersion(v.version)}
						>
							{isLatest ? `Revisi ${v.version} (terbaru)` : `Revisi ${v.version}`}
						</button>
					{/each}
				</div>
			{/if}

			<div class="post-editor-scroll-content">
				{#if flow.editPostLoading}
					<div style="display:grid;gap:14px;padding:30px 0;">
						<div class="skeleton-row" style="height:38px;"></div>
						<div class="skeleton-row" style="height:38px;"></div>
						<div class="skeleton-row" style="height:260px;"></div>
					</div>
				{:else}
					<div class="post-editor-meta-grid">
						<div class="post-editor-field-group">
							<label class="post-editor-field-label" for="post-editor-title">Judul Artikel</label>
							<input
								id="post-editor-title"
								type="text"
								class="post-editor-text-input"
								bind:value={flow.editPostTitle}
								placeholder="Ketik judul artikel..."
							/>
						</div>

						<div class="post-editor-field-group">
							<label class="post-editor-field-label" for="post-editor-thumb">Gambar Thumbnail / Unggulan</label>
							<div class="post-editor-thumbnail-row">
								{#if flow.editPostThumbnail}
									<img
										src={flow.editPostThumbnail}
										width="38"
										height="38"
										alt="Thumbnail"
										class="post-editor-thumbnail-preview"
										onerror={thumbOnError}
									/>
								{/if}
								<input
									id="post-editor-thumb"
									type="url"
									class="post-editor-text-input"
									bind:value={flow.editPostThumbnail}
									placeholder="https://... URL gambar thumbnail"
								/>
								<button type="button" class="thumbnail-upload-btn" style="flex:none;height:36px;padding:0 12px;" title="Upload gambar" onclick={() => fileEl?.click()}>
									<Upload size={14} />
									<span>Upload</span>
								</button>
								<input bind:this={fileEl} type="file" accept="image/*" class="hidden-file-input" onchange={uploadThumb} />
							</div>
						</div>
					</div>

					<div class="post-editor-canvas-container">
						<div class="post-editor-toolbar">
							<button type="button" class="post-editor-tool-btn" title="Tebal (Ctrl+B)" onclick={() => exec('bold')}><Bold size={15} /></button>
							<button type="button" class="post-editor-tool-btn" title="Miring (Ctrl+I)" onclick={() => exec('italic')}><Italic size={15} /></button>
							<button type="button" class="post-editor-tool-btn" title="Garis Bawah (Ctrl+U)" onclick={() => exec('underline')}><Underline size={15} /></button>
							<button type="button" class="post-editor-tool-btn" title="Coret" onclick={() => exec('strikeThrough')}><Strikethrough size={15} /></button>

							<div class="post-editor-toolbar-sep"></div>

							<button type="button" class="post-editor-tool-btn" title="Heading 2" style="font-size:12px;font-weight:700;" onclick={() => exec('formatBlock', '<h2>')}>H2</button>
							<button type="button" class="post-editor-tool-btn" title="Heading 3" style="font-size:12px;font-weight:700;" onclick={() => exec('formatBlock', '<h3>')}>H3</button>
							<button type="button" class="post-editor-tool-btn" title="Paragraf Biasa" style="font-size:12px;" onclick={() => exec('formatBlock', '<p>')}>¶</button>

							<div class="post-editor-toolbar-sep"></div>

							<button type="button" class="post-editor-tool-btn" title="Daftar Bullet" onclick={() => exec('insertUnorderedList')}><List size={15} /></button>
							<button type="button" class="post-editor-tool-btn" title="Daftar Bernomor" onclick={() => exec('insertOrderedList')}><ListOrdered size={15} /></button>
							<button type="button" class="post-editor-tool-btn" title="Kutipan / Blockquote" onclick={() => exec('formatBlock', '<blockquote>')}><Quote size={15} /></button>

							<div class="post-editor-toolbar-sep"></div>

							<button
								type="button"
								class="post-editor-tool-btn"
								title="Sisipkan Tautan"
								onclick={() => {
									const url = prompt('Masukkan URL tautan (http:// atau https://):');
									if (url) exec('createLink', url);
								}}
							>
								<Link size={15} />
							</button>
						</div>

						<div
							class="post-editor-canvas"
							bind:this={canvasEl}
							contenteditable="true"
							role="textbox"
							aria-multiline="true"
							oninput={sync}
							onblur={sync}
						></div>
					</div>

					<div class="post-editor-field-group">
						<label class="post-editor-field-label" for="post-editor-meta">Meta Description / Ringkasan Artikel</label>
						<textarea
							id="post-editor-meta"
							rows="2"
							class="post-editor-text-input"
							style="resize:vertical;"
							bind:value={flow.editPostMetaDesc}
							placeholder="Ringkasan singkat artikel untuk preview atau meta description..."
						></textarea>
					</div>
				{/if}
			</div>

			<div class="post-editor-footer">
				<div class="post-editor-footer-stats"><span>{words.toLocaleString('id-ID')} kata</span></div>
				<div class="post-editor-footer-actions">
					<button
						type="button"
						class="secondary-action-btn"
						disabled={flow.editPostBusy}
						onclick={() => void flow.openRegenModal(flow.editPostDocId)}
					>
						<RefreshCw size={15} />
						<span>Buat Ulang (5 TC)</span>
					</button>
					<button type="button" class="secondary-action-btn" disabled={flow.editPostBusy} onclick={() => (flow.editingPostItem = null)}>
						Batal
					</button>
					<button
						type="button"
						class="primary-action-btn"
						disabled={flow.editPostBusy || flow.editPostLoading}
						onclick={() => void flow.savePostEditor(canvasEl?.innerHTML ?? '')}
					>
						<Save size={15} />
						<span>{flow.editPostBusy ? 'Menyimpan…' : 'Simpan Perubahan'}</span>
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
