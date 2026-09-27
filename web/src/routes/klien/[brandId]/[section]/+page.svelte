<script lang="ts">
	// Placeholder section belum di-port — sesuai urutan fase (REWRITE/07).
	import { page } from '$app/state';
	import { FileText } from '@lucide/svelte';

	const section = $derived(page.params.section as string);
	const fase: Record<string, string> = {
		media: '5 — Media & Mitra',
		campaigns: '7 — Campaigns & Flow',
		publications: '6 — Publications & Publishing',
		'link-monitor': '6 — Publications & Publishing',
		analytics: '10 — PR & Analytics',
		audit: '10 — PR & Analytics',
		credits: '8 — Credits, Payments & Finance',
		invoices: '8 — Credits, Payments & Finance'
	};
	const label = $derived(fase[section] ?? 'fase berikutnya');
</script>

<main class="rounded-xl border border-border bg-card p-10 text-center">
	<FileText size={32} class="mx-auto text-muted-foreground" />
	<h1 class="mt-3 text-lg font-semibold capitalize">{section.replace(/-/g, ' ')}</h1>
	<p class="mt-1 text-sm text-muted-foreground">
		Section ini menyusul di <strong>fase {label}</strong> — backend/UI-nya sedang diantrikan
		sesuai urutan port (REWRITE/07_EXECUTION_PHASES.md).
	</p>
	<a class="mt-4 inline-block rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground" href={`/klien/${page.params.brandId}`}>
		Kembali ke Beranda
	</a>
</main>
