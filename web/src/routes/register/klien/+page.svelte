<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Pendaftaran khusus Klien: panel auth dengan peran terkunci (parity
	// app/register/klien yang mengunci role agar tidak salah daftar).
	import AuthShell from '$lib/components/auth/auth-shell.svelte';
	import AuthPanel from '$lib/components/auth/auth-panel.svelte';
	import '$lib/components/auth/auth.css';
</script>

<svelte:head>
	<title>{tr("Daftar Klien · Tauthub")}</title>
</svelte:head>

<AuthShell>
	<AuthPanel initialMode="register" lockedRole="client" />
</AuthShell>
