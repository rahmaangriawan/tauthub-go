<script lang="ts">
	// Editor teks kaya untuk halaman editor dokumen — dipisahkan dari toolbar
	// contentEditable _editor-modal.svelte (Taut Pintar Writer) supaya bisa
	// dipakai di luar modal: perintah format, unggah gambar ke /brands/:id/assets,
	// dialog tautan, dan sinkronisasi HTML dua arah.
	import { Image, Link, List, ListOrdered, LoaderCircle, Quote, RotateCcw, RotateCw } from '@lucide/svelte';
	import type { Component } from 'svelte';
	import { apiRequest } from '$lib/auth';
	import { sanitizeHtml } from '$lib/sanitize-html';

	let {
		value = $bindable(''),
		brandId,
		disabled = false,
		minHeight = 420,
		ariaLabel
	}: {
		value?: string;
		brandId: string;
		disabled?: boolean;
		minHeight?: number;
		ariaLabel?: string;
	} = $props();

	let bodyEl = $state<HTMLDivElement | null>(null);
	let imageInput = $state<HTMLInputElement | undefined>(undefined);
	let imageBusy = $state(false);
	let imageError = $state('');
	let mode = $state<'rich' | 'html'>('rich');
	// Saat kembali dari mode HTML, elemen contenteditable dibuat ulang oleh
	// {#if} — isinya harus diisi ulang dari value.
	let pendingRender = $state(false);
	let linkDialog = $state({ open: false, url: '', isEdit: false });
	let savedRange: Range | null = null;

	// Isi awal & saat pergantian sumber luar (mis. pulihkan versi).
	let lastExternal = '';
	$effect(() => {
		if (value !== lastExternal && bodyEl && mode === 'rich') {
			bodyEl.innerHTML = sanitizeHtml(value);
			lastExternal = value;
		}
	});

	$effect(() => {
		if (pendingRender && bodyEl) {
			bodyEl.innerHTML = sanitizeHtml(value);
			lastExternal = value;
			pendingRender = false;
		}
	});

	function sync() {
		if (!bodyEl) return;
		lastExternal = bodyEl.innerHTML;
		value = bodyEl.innerHTML;
	}

	function exec(cmd: string, arg?: string) {
		const body = bodyEl;
		if (!body || disabled) return;
		body.focus();
		document.execCommand(cmd, false, arg);
		sync();
	}

	const tools: Array<{ cmd: string; label?: string; icon?: Component; arg?: string; name: string }> = [
		{ cmd: 'bold', label: 'B', name: 'Tebal' },
		{ cmd: 'italic', label: 'I', name: 'Miring' },
		{ cmd: 'underline', label: 'U', name: 'Garis bawah' },
		{ cmd: 'formatBlock', label: 'H2', arg: 'h2', name: 'Judul bagian' },
		{ cmd: 'formatBlock', label: 'H3', arg: 'h3', name: 'Sub-judul' },
		{ cmd: 'formatBlock', arg: 'blockquote', icon: Quote, name: 'Kutipan' },
		{ cmd: 'insertUnorderedList', icon: List, name: 'Daftar poin' },
		{ cmd: 'insertOrderedList', icon: ListOrdered, name: 'Daftar bernomor' },
		{ cmd: 'undo', icon: RotateCcw, name: 'Urungkan' },
		{ cmd: 'redo', icon: RotateCw, name: 'Ulangi' }
	];

	async function pickImage(file: File) {
		if (!file || imageBusy || !file.type.startsWith('image/')) return;
		imageBusy = true;
		imageError = '';
		try {
			const form = new FormData();
			form.append('file', file);
			const res = await apiRequest(`/v1/brands/${brandId}/assets`, { method: 'POST', body: form });
			if (!res.ok) throw new Error();
			const data = (await res.json()) as { url?: string };
			if (data.url && bodyEl) {
				bodyEl.focus();
				document.execCommand('insertImage', false, data.url);
				sync();
			}
		} catch {
			imageError = 'Gagal mengunggah gambar. Periksa koneksi lalu coba lagi.';
		} finally {
			imageBusy = false;
			if (imageInput) imageInput.value = '';
		}
	}

	/** Anchor pada seleksi saat ini (bila ada). */
	function closestAnchor(): HTMLAnchorElement | null {
		const body = bodyEl;
		if (!body) return null;
		const sel = window.getSelection();
		if (!sel || sel.rangeCount === 0) return null;
		for (const node of [sel.anchorNode, sel.focusNode]) {
			const el = node instanceof Element ? node : (node?.parentElement ?? null);
			const anchor = el?.closest('a');
			if (anchor && body.contains(anchor)) return anchor as HTMLAnchorElement;
		}
		return null;
	}

	function openLinkDialog() {
		if (!bodyEl || disabled) return;
		const sel = window.getSelection();
		savedRange = sel && sel.rangeCount > 0 ? sel.getRangeAt(0).cloneRange() : null;
		const anchor = closestAnchor();
		linkDialog = { open: true, url: anchor?.getAttribute('href') ?? '', isEdit: Boolean(anchor) };
	}

	function applyLink() {
		const body = bodyEl;
		if (!body) {
			linkDialog = { ...linkDialog, open: false };
			return;
		}
		const url = linkDialog.url.trim();
		body.focus();
		const sel = window.getSelection();
		if (sel && savedRange) {
			sel.removeAllRanges();
			try {
				sel.addRange(savedRange);
			} catch {
				/* range basi */
			}
		}
		const anchor = closestAnchor();
		if (!url) {
			if (anchor) {
				const range = document.createRange();
				range.selectNodeContents(anchor);
				sel?.removeAllRanges();
				sel?.addRange(range);
				document.execCommand('unlink', false);
			}
		} else if (anchor && sel?.isCollapsed) {
			anchor.setAttribute('href', url);
		} else {
			document.execCommand('createLink', false, url);
		}
		sync();
		linkDialog = { open: false, url: '', isEdit: false };
	}

	function handleBodyClick(e: MouseEvent) {
		const body = bodyEl;
		const el = e.target instanceof Element ? e.target : null;
		const anchor = el?.closest('a');
		if (!body || !anchor || !body.contains(anchor)) return;
		e.preventDefault();
		const range = document.createRange();
		range.selectNodeContents(anchor);
		const sel = window.getSelection();
		sel?.removeAllRanges();
		sel?.addRange(range);
	}
</script>

<div class="rte">
	<div class="flex flex-wrap items-center gap-1 rounded-lg border p-1" role="toolbar" aria-label="Format teks">
		{#each tools as t (t.cmd + (t.arg ?? ''))}
			{@const ToolIcon = t.icon}
			<button
				class="rte-tool"
				type="button"
				title={t.name}
				aria-label={t.name}
				disabled={disabled || mode === 'html'}
				onclick={() => exec(t.cmd, t.arg)}
			>
				{#if ToolIcon}<ToolIcon size={15} />{:else}{t.label}{/if}
			</button>
		{/each}
		<input
			bind:this={imageInput}
			type="file"
			accept="image/*"
			class="hidden"
			onchange={(e) => {
				const f = (e.currentTarget as HTMLInputElement).files?.[0];
				if (f) void pickImage(f);
			}}
		/>
		<button
			class="rte-tool"
			type="button"
			title="Unggah gambar"
			aria-label="Unggah gambar"
			disabled={disabled || imageBusy || mode === 'html'}
			onclick={() => imageInput?.click()}
		>
			{#if imageBusy}<LoaderCircle size={15} class="animate-spin" />{:else}<Image size={15} />{/if}
		</button>
		<button
			class="rte-tool"
			type="button"
			title="Sisipkan tautan"
			aria-label="Sisipkan tautan"
			disabled={disabled || mode === 'html'}
			onmousedown={(e: MouseEvent) => e.preventDefault()}
			onclick={openLinkDialog}
		><Link size={15} /></button>
		<span class="flex-1"></span>
		<button
			class="rte-tool rte-tool-toggle"
			class:rte-tool-active={mode === 'html'}
			type="button"
			aria-pressed={mode === 'html'}
			title={mode === 'html' ? 'Kembali ke editor visual' : 'Edit HTML mentah'}
			onclick={() => {
				if (mode === 'html') {
					pendingRender = true;
					mode = 'rich';
				} else {
					mode = 'html';
				}
			}}
		>
			<span class="font-mono text-[11px]">&lt;/&gt;</span>
		</button>
	</div>

	{#if mode === 'rich'}
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<div
			bind:this={bodyEl}
			class="rte-body prose prose-sm overflow-y-auto rounded-lg border p-3 text-sm"
			style="min-height: {minHeight}px; max-height: 60vh;"
			contenteditable={!disabled}
			role="textbox"
			tabindex="0"
			aria-label={ariaLabel ?? 'Isi artikel'}
			oninput={sync}
			onclick={handleBodyClick}
		></div>
	{:else}
		<textarea
			class="rte-body w-full resize-y rounded-lg border p-3 font-mono text-xs"
			style="min-height: {minHeight}px; max-height: 60vh;"
			bind:value
			disabled={disabled}
			aria-label={ariaLabel ?? 'Isi artikel (HTML)'}
		></textarea>
	{/if}

	{#if imageError}<p class="mt-1 text-sm" style="color: var(--destructive);">{imageError}</p>{/if}

	{#if linkDialog.open}
		<div class="rte-link-backdrop" role="presentation" onclick={() => (linkDialog = { ...linkDialog, open: false })}></div>
		<div class="rte-link-pop" role="dialog" aria-label="Tautan">
			<input
				class="app-input w-72 text-sm"
				placeholder="https://contoh.com/artikel"
				bind:value={linkDialog.url}
				onkeydown={(e) => {
					if (e.key === 'Enter') { e.preventDefault(); applyLink(); }
					if (e.key === 'Escape') linkDialog = { ...linkDialog, open: false };
				}}
			/>
			<button class="app-btn app-btn-primary" type="button" onclick={applyLink}>Simpan</button>
			<button class="app-btn app-btn-soft" type="button" onclick={() => (linkDialog = { ...linkDialog, open: false })}>Batal</button>
		</div>
	{/if}
</div>

<style>
	.rte {
		position: relative;
	}
	.rte-tool {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-height: 32px;
		min-width: 32px;
		padding: 0 8px;
		border: none;
		border-radius: 6px;
		background: transparent;
		color: var(--foreground);
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
	}
	.rte-tool:hover:not(:disabled) {
		background: var(--secondary);
	}
	.rte-tool:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 1px;
	}
	.rte-tool:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}
	.rte-tool-toggle {
		border: 1px solid var(--border);
	}
	.rte-tool-active {
		background: var(--primary);
		color: var(--primary-foreground);
		border-color: var(--primary);
	}
	.rte-body {
		margin-top: 8px;
		background: var(--background);
	}
	.rte-link-backdrop {
		position: fixed;
		inset: 0;
		z-index: 1090;
	}
	.rte-link-pop {
		position: absolute;
		z-index: 1100;
		right: 0;
		top: 42px;
		display: flex;
		gap: 8px;
		align-items: center;
		padding: 10px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--card, #fff);
		box-shadow: 0 12px 32px rgba(15, 23, 42, 0.14);
	}
</style>
