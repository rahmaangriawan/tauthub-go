<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Komponen bersama halaman tujuan redirect gateway (success/cancelled).
	// Parity app/payments/_components/return-page.tsx:
	//   1. menulis kunci localStorage → tab dashboard klien mendengar event
	//      `storage` dan me-refresh saldo/ledger;
	//   2. mencoba menutup tab checkout;
	//   3. setelah ~2,5 detik redirect ke dashboard klien.
	// TC dikreditkan oleh webhook gateway — halaman ini hanya urusan UX.
	import { onMount } from 'svelte';

	let { mode, brandId = null }: { mode: 'success' | 'cancelled'; brandId?: string | null } = $props();

	const TOPUP_SUCCESS_KEY = 'tauthub:topup-success';
	const TOPUP_CANCELLED_KEY = 'tauthub:topup-cancelled';

	// mode adalah prop statis per halaman (success/cancelled) — nilai awalnya
	// sudah final, tetapi dibaca di template agar reaktif bila prop berubah.
	const success = $derived(mode === 'success');

	onMount(() => {
		const timer = window.setTimeout(() => {
			window.location.replace(brandId ? `/klien/${brandId}` : '/klien');
		}, 2500);

		try {
			localStorage.setItem(
				mode === 'success' ? TOPUP_SUCCESS_KEY : TOPUP_CANCELLED_KEY,
				JSON.stringify({ at: Date.now(), brandId: brandId ?? null })
			);
		} catch {
			// storage penuh / diblokir — abaikan
		}
		try {
			window.close();
		} catch {
			// abaikan — timeout di atas yang redirect
		}

		return () => window.clearTimeout(timer);
	});
</script>

<main class="flex min-h-screen flex-col items-center justify-center gap-3 bg-background px-6 text-center">
	{#if success}
		<svg width="56" height="56" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="color: var(--success);">
			<circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" />
			<path d="M8 12.5l2.5 2.5L16 9.5" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
		</svg>
		<h1 class="text-xl font-bold">Pembayaran berhasil</h1>
		<p class="text-sm text-muted-foreground">{tr("Saldo Tauthub Credit sedang diperbarui…")}</p>
	{:else}
		<svg width="56" height="56" viewBox="0 0 24 24" fill="none" aria-hidden="true" style="color: var(--muted-foreground);">
			<circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" />
			<path d="M9 9l6 6M15 9l-6 6" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" />
		</svg>
		<h1 class="text-xl font-bold">Pembayaran dibatalkan</h1>
		<p class="text-sm text-muted-foreground">{tr("Kamu bisa mengulang top-up kapan saja dari dashboard.")}</p>
	{/if}
</main>
