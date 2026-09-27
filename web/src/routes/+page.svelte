<script lang="ts">
	// Port app/page.tsx — halaman publik lama (landing) sudah dihapus; frontpage
	// langsung mengarahkan pengunjung ke layar pendaftaran. Di app lama tujuannya
	// `/login?mode=register` (satu layar gabungan); di SvelteKit pendaftaran
	// terpisah, jadi arahkan ke `/register-role`. `/login?mode=register` tetap
	// didukung sebagai jalur legacy (lihat routes/login/+page.svelte).
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	onMount(() => {
		void goto('/register-role', { replaceState: true });
	});
</script>

<svelte:head>
	<meta name="robots" content="noindex, nofollow" />
	<title>Tauthub</title>
</svelte:head>

<main class="flex min-h-screen items-center justify-center bg-background px-4">
	<p class="text-sm text-muted-foreground">Mengalihkan ke pendaftaran…</p>
</main>
