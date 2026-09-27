<script lang="ts">
	// Halaman Masuk & Daftar (split-screen). Mode ditentukan ?mode=register.
	// Halaman /register/klien dan /register/mitra memakai panel yang sama dengan
	// peran terkunci lewat prop lockedRole.
	import AuthShell from '$lib/components/auth/auth-shell.svelte';
	import AuthPanel from '$lib/components/auth/auth-panel.svelte';
	import '$lib/components/auth/auth.css';

	let { data }: { data?: { lockedRole?: 'client' | 'partner' | null; initialMode?: 'login' | 'register' } } =
		$props();
</script>

<svelte:head>
	<title>Masuk · Tauthub</title>
	<meta name="description" content="Masuk atau daftar akun Tauthub untuk mengelola distribusi konten dan PR multi-brand." />
</svelte:head>

<AuthShell>
	<AuthPanel initialMode={data?.initialMode ?? 'login'} lockedRole={data?.lockedRole ?? null} />
</AuthShell>
