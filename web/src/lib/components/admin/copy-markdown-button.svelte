<script lang="ts">
	// Tombol "Salin Markdown" untuk halaman log/laporan. `build` dipanggil saat
	// diklik supaya isi yang tersalin selalu sama dengan yang sedang tampil
	// (filter, halaman, dan data terbaru ikut).
	import { Check, Copy, TriangleAlert } from '@lucide/svelte';
	import { copyText } from '$lib/copy-markdown';

	let {
		build,
		label = 'Salin Markdown',
		disabled = false
	}: { build: () => string; label?: string; disabled?: boolean } = $props();

	let state = $state<'idle' | 'done' | 'fail'>('idle');
	let timer: ReturnType<typeof setTimeout> | undefined;

	async function copy() {
		const ok = await copyText(build());
		state = ok ? 'done' : 'fail';
		clearTimeout(timer);
		timer = setTimeout(() => (state = 'idle'), 2000);
	}
</script>

<button class="app-btn app-btn-soft" type="button" {disabled} onclick={copy} aria-live="polite">
	{#if state === 'done'}
		<Check size={14} /> Tersalin
	{:else if state === 'fail'}
		<TriangleAlert size={14} /> Gagal menyalin
	{:else}
		<Copy size={14} /> {label}
	{/if}
</button>
