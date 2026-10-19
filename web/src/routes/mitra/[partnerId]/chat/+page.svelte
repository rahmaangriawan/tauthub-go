<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Halaman Pesan (mitra): semua obrolan dari klien per artikel.
	// Mitra bisa membalas selama sesi 3×24 jam masih berjalan (dihitung
	// sejak pesan pertama klien); setelah berakhir, riwayat tetap terbaca.
	import { page } from '$app/state';
	import PageTitle from '$lib/components/page-title.svelte';
	import ChatWorkspace from '$lib/chat/chat-workspace.svelte';

	const initialThread = $derived((page.url.searchParams.get('t') ?? '').trim());
</script>

<PageTitle title="Pesan" />

<div class="flex min-h-0 flex-col px-1 lg:h-[calc(100dvh-160px)] lg:min-h-[560px]">
	<div class="mb-4 shrink-0">
		<h1 class="text-xl font-bold text-slate-900">{tr('Pesan')}</h1>
		<p class="mt-1 text-[13px] text-muted-foreground">
			{tr('Obrolan dari klien untuk artikel di media Anda. Balas selama sesi 3×24 jam masih berjalan; setelah itu riwayat tetap dapat dibaca.')}
		</p>
	</div>

	<div class="min-h-0 flex-1">
		<ChatWorkspace apiBase="/v1/partner" side="partner" {initialThread} />
	</div>
</div>
