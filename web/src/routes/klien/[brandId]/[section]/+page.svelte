<script lang="ts">
	// Section belum di-port — sesuai urutan fase (REWRITE/07). Empty state
	// menyebut sebab + aksi berikutnya, bukan sekadar "belum tersedia".
	import { page } from '$app/state';
	import { Construction } from '@lucide/svelte';

	const section = $derived(page.params.section as string);
	const fase: Record<string, string> = {
		media: '5 — Media & Mitra',
		campaigns: '7 — Campaigns & Flow',
		publications: '6 — Publications & Publishing',
		'link-monitor': '6 — Publications & Publishing',
		analytics: '10 — PR & Analytics',
		'audit-publikasi': '10 — PR & Analytics',
		credits: '8 — Credits, Payments & Finance',
		invoices: '8 — Credits, Payments & Finance'
	};
	const label = $derived(fase[section] ?? 'fase berikutnya');
	const title = $derived(section.replace(/-/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase()));
</script>

<section class="app-card mx-auto max-w-xl px-8 py-10 text-center">
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
