<script lang="ts">
	// Fallback untuk section klien yang belum di-port (peta fase di
	// REWRITE/07_EXECUTION_PHASES.md). Section yang sudah ada punya route
	// sendiri dan tidak pernah masuk ke sini.
	import { page } from '$app/state';
	import { Construction } from '@lucide/svelte';
	import PageTitle from '$lib/components/page-title.svelte';

	const section = $derived(page.params.section as string);
	const fase: Record<string, string> = {
		distribution: '6 — Publications & Publishing',
		members: '2 — Brands & Workspace',
		notifications: '12 — Notifications & Comms',
		'pr-calculator': '10 — PR & Analytics'
	};
	const label = $derived(fase[section] ?? 'fase berikutnya');
	const title = $derived(section.replace(/-/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase()));
</script>

<PageTitle title="Bagian Klien" />

<section class="app-card px-8 py-10 text-center">
	<div class="mx-auto flex h-11 w-11 items-center justify-center rounded-xl" style="background-color: var(--accent); color: var(--accent-foreground);">
		<Construction size={20} />
	</div>
	<h1 class="mt-4 text-lg font-semibold">{title}</h1>
	<p class="mt-1.5 text-sm text-muted-foreground">
		Section ini dikerjakan pada <strong>fase {label}</strong> sesuai urutan port di
		REWRITE/07_EXECUTION_PHASES.md — backend dan UI-nya dibangun bersamaan.
	</p>
	<a class="app-btn app-btn-primary mt-5" href={`/klien/${page.params.brandId}`}>Kembali ke Beranda</a>
</section>
