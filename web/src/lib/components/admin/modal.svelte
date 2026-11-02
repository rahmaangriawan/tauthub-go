<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Modal admin (dialog bawaan browser): fokus terkelola, Escape menutup.
	import type { Snippet } from 'svelte';
	import { X } from '@lucide/svelte';

	let {
		title,
		label,
		maxWidth = 'lg',
		onClose,
		children,
		footer,
		titleExtra,
		titleLead
	}: {
		title: string;
		label?: string;
		maxWidth?: 'sm' | 'md' | 'lg' | 'xl' | '2xl' | '4xl' | 'workspace';
		onClose: () => void;
		children: Snippet;
		footer?: Snippet;
		titleExtra?: Snippet;
		titleLead?: Snippet;
	} = $props();

	const workspace = $derived(maxWidth === 'workspace');
	const widthClass = $derived(
		maxWidth === 'sm' ? 'max-w-sm'
		: maxWidth === 'md' ? 'max-w-md'
		: maxWidth === 'xl' ? 'max-w-xl'
		: maxWidth === '2xl' ? 'max-w-2xl'
		: maxWidth === '4xl' ? 'max-w-4xl'
		: workspace ? 'max-w-[1200px]'
		: 'max-w-lg'
	);
</script>

<div
	class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
	style="background-color: rgba(0,0,0,.45);"
	role="presentation"
	onclick={onClose}
	onkeydown={(e) => {
		if (e.key === 'Escape') onClose();
	}}
>
	{#if workspace}
		<!-- Varian workspace: header + footer menempel, isi di tengah scroll sendiri. -->
		<dialog
			open
			class="app-card m-0 flex max-h-[92vh] w-full flex-col overflow-hidden p-0 {widthClass}"
			style="border-radius: 14px; box-shadow: 0 24px 60px -20px rgba(15, 30, 60, .35);"
			aria-label={label ?? title}
			onclick={(e) => e.stopPropagation()}
		>
			<div
				class="flex shrink-0 items-center justify-between gap-3 border-b px-5 py-3.5"
				style="background: linear-gradient(180deg, #f6faff 0%, #ffffff 100%);"
			>
				<div class="flex min-w-0 flex-wrap items-center gap-2.5">
					{#if titleLead}{@render titleLead()}{/if}
					<h2 class="truncate text-xl font-semibold" style="color: #14233f;">{title}</h2>
					{#if titleExtra}{@render titleExtra()}{/if}
				</div>
				<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label={tr("Tutup")} onclick={onClose}>
					<X size={18} />
				</button>
			</div>
			<div class="min-h-0 flex-1 overflow-y-auto px-5 py-4">
				{@render children()}
			</div>
			{#if footer}
				<div class="flex shrink-0 justify-end gap-2.5 border-t bg-white px-5 py-3">{@render footer()}</div>
			{/if}
		</dialog>
	{:else}
	<dialog
		open
		class="app-card w-full {widthClass} p-5"
		aria-label={label ?? title}
		onclick={(e) => e.stopPropagation()}
	>
		<div class="flex items-start justify-between gap-3">
			<h2 class="text-lg font-semibold">{title}</h2>
			<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label={tr("Tutup")} onclick={onClose}>
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
	{/if}
</div>
