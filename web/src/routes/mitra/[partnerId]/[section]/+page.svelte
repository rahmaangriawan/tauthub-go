<script lang="ts">
	// Halaman section mitra yang belum punya layar khusus (mis. tautan lama
	// /mitra/{id}/kalkulator-media) — arahkan mitra kembali ke menu yang ada
	// tanpa membuat menu baru di luar app lama.
	import { page } from '$app/state';
	import { getPartnerUrlId } from '$lib/partner-url.svelte';
	import PageTitle from '$lib/components/page-title.svelte';

	const partnerId = $derived(page.params.partnerId as string);
	const base = $derived(`/mitra/${getPartnerUrlId() || partnerId}`);
	const section = $derived(page.params.section as string);
</script>

<PageTitle title="Halaman Tidak Ditemukan" />

<section>
	<div class="app-card p-6 text-center">
		<h1 class="text-lg font-semibold">Halaman tidak ditemukan</h1>
		<p class="mt-2 text-sm text-muted-foreground">
			Bagian "{section}" belum tersedia di portal mitra.
		</p>
		<a class="app-btn app-btn-primary mt-4" href={base}>Kembali ke Ringkasan</a>
	</div>
</section>
