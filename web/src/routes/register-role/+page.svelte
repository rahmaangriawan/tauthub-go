<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// /register-role tetap ada agar tautan lama tidak mati (R-24). Halaman ini
	// sekarang memakai panel auth yang sama, dimulai pada mode Daftar.
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import AuthShell from '$lib/components/auth/auth-shell.svelte';
	import AuthPanel from '$lib/components/auth/auth-panel.svelte';
	import '$lib/components/auth/auth.css';

	// Dipertahankan demi kompatibilitas tautan /register-role?role=partner.
	// Panel sudah menyediakan pilihan peran di dalam form.
	onMount(() => {
		const role = new URLSearchParams(window.location.search).get('role');
		if (role) void goto('/login?mode=register', { replaceState: true });
	});
</script>

<svelte:head>
	<title>{tr("Daftar · Tauthub")}</title>
</svelte:head>

<AuthShell>
	<AuthPanel initialMode="register" lockedRole={null} />
</AuthShell>
