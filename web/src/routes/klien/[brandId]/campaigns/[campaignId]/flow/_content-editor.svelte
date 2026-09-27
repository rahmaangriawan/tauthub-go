<script lang="ts">
	// Port content-editor.tsx — editor konten untuk flow klien. H1 pertama
	// diekstrak ke field judul dan dihapus dari editor body. Tombol "Simpan"
	// memunculkan konfirmasi "Sekalian publish ke media?".
	import {
		Bold,
		Heading2,
		Heading3,
		Italic,
		List,
		ListOrdered,
		Quote,
		Redo2,
		Underline,
		Undo2,
		X
	} from '@lucide/svelte';
	import { countWords, extractH1 } from './_flow-state.svelte';

	let {
		initialTitle,
		initialHtml,
		saving = false,
		saveError = null,
		onSave
	}: {
		initialTitle: string;
		initialHtml: string;
		saving?: boolean;
		saveError?: string | null;
		onSave: (title: string, html: string, publish: boolean) => void;
	} = $props();

	const TOOLS = [
		{ cmd: 'bold', icon: Bold, label: 'Tebal' },
		{ cmd: 'italic', icon: Italic, label: 'Miring' },
		{ cmd: 'underline', icon: Underline, label: 'Garis bawah' },
		{ cmd: 'formatBlock', arg: 'h2', icon: Heading2, label: 'Judul bagian (H2)' },
		{ cmd: 'formatBlock', arg: 'h3', icon: Heading3, label: 'Sub-judul (H3)' },
		{ cmd: 'formatBlock', arg: 'blockquote', icon: Quote, label: 'Kutipan' },
		{ cmd: 'insertUnorderedList', icon: List, label: 'Daftar poin' },
		{ cmd: 'insertOrderedList', icon: ListOrdered, label: 'Daftar bernomor' },
		{ cmd: 'undo', icon: Undo2, label: 'Urungkan' },
		{ cmd: 'redo', icon: Redo2, label: 'Ulangi' }
	] as const;

	let title = $state('');
	$effect(() => {
		title = initialTitle ?? '';
	});
	let words = $state(0);
	let confirmOpen = $state(false);
	let bodyEl = $state<HTMLDivElement | null>(null);

	function sync() {
		if (bodyEl) words = countWords(bodyEl.innerText || '');
	}

	// Load: ekstrak H1 dari initialHtml ke field judul, sisanya jadi body.
	$effect(() => {
		const html = initialHtml;
		const el = bodyEl;
		if (!el) return;
		const { h1, body } = extractH1(html);
		el.innerHTML = body || html;
		const text = el.innerText || '';
		const t = window.setTimeout(() => {
			if (h1) title = h1;
			words = countWords(text);
		}, 0);
		return () => window.clearTimeout(t);
	});

	function exec(cmd: string, arg?: string) {
		if (!bodyEl) return;
		bodyEl.focus();
		window.document.execCommand(cmd, false, arg);
		sync();
	}

	function confirmSave(publish: boolean) {
		confirmOpen = false;
		onSave(title.trim() || 'Artikel', bodyEl?.innerHTML ?? '', publish);
	}
</script>

<div class="ce-wrap">
	<div class="ce-title-field">
		<label class="ce-field-label" for="content-editor-title">Judul artikel</label>
		<input
			id="content-editor-title"
			class="ce-title-input"
			bind:value={title}
			placeholder="Judul artikel (diambil dari H1)"
			maxlength="200"
		/>
	</div>

	<div class="ce-editor">
		<div class="ce-toolbar" role="toolbar" aria-label="Format teks">
			{#each TOOLS as tool (tool.cmd + ('arg' in tool ? tool.arg : ''))}
				{@const Icon = tool.icon}
				<button
					type="button"
					class="ce-tool-btn"
					title={tool.label}
					aria-label={tool.label}
					onmousedown={(event) => event.preventDefault()}
					onclick={() => exec(tool.cmd, 'arg' in tool ? tool.arg : undefined)}
				>
					<Icon size={17} />
				</button>
			{/each}
		</div>
		<div
			class="ce-body"
			bind:this={bodyEl}
			contenteditable="true"
			role="textbox"
			aria-multiline="true"
			oninput={sync}
			onblur={sync}
		></div>
		<div class="ce-editor-bar">{words.toLocaleString('id-ID')} kata</div>
	</div>

	{#if saveError}<p class="ce-error">{saveError}</p>{/if}

	<div class="ce-actions">
		<button type="button" class="app-btn app-btn-primary" disabled={saving} onclick={() => (confirmOpen = true)}>
			{saving ? 'Menyimpan…' : 'Simpan'}
		</button>
	</div>

	{#if confirmOpen}
		<div
			class="overlay"
			role="presentation"
		>
			<dialog
				open
				class="modal-card"
				aria-label="Sekalian publish ke media?"
				onclick={(e) => e.stopPropagation()}
				onkeydown={(e) => { if (e.key === 'Escape' && !saving) confirmOpen = false; }}
			>
				<div class="modal-head">
					<span>Sekalian publish ke media?</span>
					<button type="button" class="modal-close" onclick={() => (confirmOpen = false)} aria-label="Tutup">
						<X size={16} />
					</button>
				</div>
				<div style="padding: 16px 18px;">
					<p style="margin:0;font-size:13px;color:var(--muted-foreground);line-height:1.6;">
						Perubahan akan disimpan. Lanjutkan langsung ke pemilihan media untuk dipublikasikan?
					</p>
				</div>
				<div class="modal-foot">
					<button type="button" class="app-btn app-btn-soft" disabled={saving} onclick={() => (confirmOpen = false)}>
						Kembali
					</button>
					<button type="button" class="app-btn app-btn-primary" disabled={saving} onclick={() => confirmSave(false)}>
						{saving ? 'Menyimpan…' : 'Simpan Saja'}
					</button>
					<button type="button" class="ce-publish-btn" disabled={saving} onclick={() => confirmSave(true)}>
						{saving ? 'Menyimpan…' : 'Ya, Publish'}
					</button>
				</div>
			</dialog>
		</div>
	{/if}
</div>
