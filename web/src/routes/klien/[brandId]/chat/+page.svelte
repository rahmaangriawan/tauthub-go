<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Halaman Pesan (klien): semua obrolan dengan mitra per artikel.
	// Dibuka dari tombol "Hubungi" di Ringkasan Distribusi dengan
	// ?pub=<publication_id> (find-or-create forum), atau ?t=<threadId>.
	import { page } from '$app/state';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import PageTitle from '$lib/components/page-title.svelte';
	import ChatWorkspace from '$lib/chat/chat-workspace.svelte';

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const initialThread = $derived((page.url.searchParams.get('t') ?? '').trim());
	const initialPublication = $derived((page.url.searchParams.get('pub') ?? '').trim());
	const apiBase = $derived(`/v1/brands/${brandId}`);
</script>

<PageTitle title="Pesan" />

<div class="px-1">
	<div class="mb-4">
		<h1 class="text-xl font-bold text-slate-900">{tr('Pesan')}</h1>
		<p class="mt-1 text-[13px] text-muted-foreground">
			{tr('Obrolan langsung dengan mitra per artikel. Sesi berlaku 3×24 jam sejak pesan pertama; setelah itu riwayat tetap dapat dibaca.')}
		</p>
	</div>

	<ChatWorkspace {apiBase} side="client" {initialThread} {initialPublication} />
</div>
