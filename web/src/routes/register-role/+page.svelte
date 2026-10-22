<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// /register-role tetap ada agar tautan lama tidak mati (R-24). Halaman ini
	// sekarang memakai panel auth yang sama, dimulai pada mode Daftar.
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import AuthShell from '$lib/components/auth/auth-shell.svelte';
	import AuthPanel from '$lib/components/auth/auth-panel.svelte';
	import '$lib/components/auth/auth.css';
	import { resolveHomeRoute, storeSession, type TokenResponse } from '$lib/auth';

	// Callback OAuth Google mengarah ke sini dengan ?role=client|partner. Peran
	// sudah dipilih di modal sebelum consent, jadi di sini tinggal menukar tiket
	// Google (cookie HttpOnly sekali pakai) menjadi akun + sesi.
	const ROLE_ALIAS: Record<string, 'client' | 'partner'> = {
		client: 'client',
		klien: 'client',
		partner: 'partner',
		mitra: 'partner'
	};

	let completing = $state(false);

	/** returnTo dari callback Google, hanya path same-origin yang aman. */
	function safeReturnTo(): string | null {
		const value = new URLSearchParams(window.location.search).get('returnTo');
		if (!value || !value.startsWith('/') || value.startsWith('//')) return null;
		if (/[\u0000-\u001f\u007f]/.test(value)) return null;
		const path = value.split(/[?#]/)[0].replace(/\/+$/, '') || '/';
		if (path === '/login' || path.startsWith('/login/')) return null;
		return value;
	}

	function referralCode(): string | null {
		try {
			const fromStore = localStorage.getItem('tauthub_ref');
			if (fromStore) return fromStore;
			const fromQuery = new URLSearchParams(window.location.search).get('ref');
			if (fromQuery) return fromQuery;
			const m = document.cookie.match(/(?:^|; )tauthub_ref=([^;]+)/);
			if (m) return decodeURIComponent(m[1]);
		} catch {
			// localStorage/cookie bisa diblokir; abaikan.
		}
		return null;
	}

	async function completeGoogle(role: 'client' | 'partner') {
		const ref = referralCode();
		const payload: Record<string, unknown> = { role };
		if (ref) payload.referral_code = ref;
		try {
			const res = await fetch('/v1/auth/google/complete', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(payload)
			});
			if (res.ok) {
				const data = (await res.json()) as TokenResponse;
				storeSession(data);
				const target = safeReturnTo() ?? (await resolveHomeRoute(data.user?.role ?? role));
				await goto(target, { replaceState: true });
				return;
			}
		} catch {
			// Jaringan gagal: jatuh ke form daftar biasa di bawah.
		}
		// Tanpa tiket Google (mis. tautan lama /register-role?role=partner) atau
		// tiket kedaluwarsa: kembalikan ke form daftar, perilaku lama.
		await goto('/login?mode=register', { replaceState: true });
	}

	onMount(() => {
		const roleParam = new URLSearchParams(window.location.search).get('role');
		const role = roleParam ? ROLE_ALIAS[roleParam] : null;
		// Tanpa peran: tampilkan panel daftar seperti sebelumnya.
		if (!role) return;
		completing = true;
		void completeGoogle(role);
	});
</script>

<svelte:head>
	<title>{tr("Daftar · Tauthub")}</title>
</svelte:head>

<AuthShell>
	{#if completing}
		<h1 class="auth-title">{tr("Menyelesaikan pendaftaran…")}</h1>
		<p class="auth-subtitle">{tr("Sebentar, akun Google Anda sedang dihubungkan.")}</p>
	{:else}
		<AuthPanel initialMode="register" lockedRole={null} />
	{/if}
</AuthShell>
