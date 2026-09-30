<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Fallback untuk URL section klien yang tidak punya halaman sendiri
	// (mis. sisa deep link ke fitur yang sudah ditiadakan: paket, intake,
	// pr-calculator, chat, history). Section yang ada route-nya tidak pernah
	// masuk ke sini.
	import { page } from '$app/state';
	import { Construction } from '@lucide/svelte';
	import PageTitle from '$lib/components/page-title.svelte';

	const section = $derived(page.params.section as string);
	const title = $derived(section.replace(/-/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase()));
</script>

<PageTitle title={tr("Bagian Klien")} />

<section class="app-card px-8 py-10 text-center">
	<div class="mx-auto flex h-11 w-11 items-center justify-center rounded-xl" style="background-color: var(--accent); color: var(--accent-foreground);">
		<Construction size={20} />
	</div>
	<h1 class="mt-4 text-lg font-semibold">{title}</h1>
	<p class="mt-1.5 text-sm text-muted-foreground">{tr("Halaman ini tidak tersedia untuk brand. Kembali ke beranda untuk melihat ringkasan dan menu lengkap.")}</p>
	<a class="app-btn app-btn-primary mt-5" href={`/klien/${page.params.brandId}`}>{tr("Kembali ke Beranda")}</a>
</section>
