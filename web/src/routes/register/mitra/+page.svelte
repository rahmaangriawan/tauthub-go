<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Pendaftaran khusus Mitra: panel auth dengan peran terkunci.
	import AuthShell from '$lib/components/auth/auth-shell.svelte';
	import AuthPanel from '$lib/components/auth/auth-panel.svelte';
	import '$lib/components/auth/auth.css';
</script>

<svelte:head>
	<title>{tr("Daftar Mitra · Tauthub")}</title>
</svelte:head>

<AuthShell>
	<AuthPanel initialMode="register" lockedRole="partner" />
</AuthShell>
