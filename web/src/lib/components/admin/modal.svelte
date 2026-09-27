<script lang="ts">
	// Modal admin (dialog bawaan browser): fokus terkelola, Escape menutup.
	import type { Snippet } from 'svelte';
	import { X } from '@lucide/svelte';

	let {
		title,
		label,
		maxWidth = 'lg',
		onClose,
		children,
		footer
	}: {
		title: string;
		label?: string;
		maxWidth?: 'sm' | 'md' | 'lg' | 'xl' | '2xl';
		onClose: () => void;
		children: Snippet;
		footer?: Snippet;
	} = $props();

	const widthClass = $derived(
		maxWidth === 'sm' ? 'max-w-sm'
		: maxWidth === 'md' ? 'max-w-md'
		: maxWidth === 'xl' ? 'max-w-xl'
		: maxWidth === '2xl' ? 'max-w-2xl'
		: 'max-w-lg'
	);
</script>

<div
	class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4"
	style="background-color: rgba(0,0,0,.45);"
	role="presentation"
	onclick={onClose}
	onkeydown={(e) => {
		if (e.key === 'Escape') onClose();
	}}
>
	<dialog
		open
		class="app-card my-8 w-full {widthClass} p-5"
		aria-label={label ?? title}
		onclick={(e) => e.stopPropagation()}
	>
		<div class="flex items-start justify-between gap-3">
			<h2 class="text-lg font-semibold">{title}</h2>
			<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label="Tutup" onclick={onClose}>
				<X size={18} />
			</button>
		</div>
		<div class="mt-4">
			{@render children()}
		</div>
		{#if footer}
			<div class="mt-4 flex justify-end gap-2 border-t pt-4">{@render footer()}</div>
		{/if}
	</dialog>
</div>
