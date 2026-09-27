<script lang="ts">
	// Port halaman login (fase 1): email+password, 2FA challenge bila aktif,
	// redirect per role. Turnstile widget menyusul (config belum enabled).
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import {
		storeSession,
		authErrorMessage,
		apiRequest,
		readApiError,
		AUTH_EXPIRED_EVENT,
		resolveHomeRoute,
		type AuthUser
	} from '$lib/auth';

	let email = $state('');
	let password = $state('');
	let busy = $state(false);
	let error = $state('');

	// Jalur legacy `/login?mode=register` (CTA landing & tautan lama di app lama).
	// Di app lama ini satu layar gabungan; di SvelteKit pendaftaran punya halaman
	// sendiri, jadi dialihkan sebelum form login dirender.
	const wantsRegister = $derived(page.url.searchParams.get('mode') === 'register');
	$effect(() => {
		if (wantsRegister) void goto('/register-role', { replaceState: true });
	});

	// 2FA step-2
	let challenge = $state<string | null>(null);
	let twoFactorCode = $state('');

	type TokenResponse = {
		access_token: string;
		refresh_token: string;
		expires_in: number;
		refresh_expires_in: number;
		session_expires_at: string;
		user: AuthUser;
	};

	// Tujuan pasca-login mengikuti routeAfterLogin app lama: klien langsung ke
	// dashboard brand pertamanya, bukan halaman pemilih brand.
	const home = (role?: string) => resolveHomeRoute(role);

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (busy) return;
		busy = true;
		error = '';
		try {
			if (challenge) {
				// Step-2: tukar kode TOTP/recovery → sesi penuh
				const res = await fetch('/v1/auth/2fa/verify', {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ challenge, code: twoFactorCode.trim() })
				});
				const data = await res.json();
				if (!res.ok) {
					error = await readApiError(res, 'Kode verifikasi salah.');
					return;
				}
				storeSession(data as TokenResponse);
				await goto(await home((data as TokenResponse).user?.role));
				return;
			}

			const res = await fetch('/v1/auth/login', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ email: email.trim().toLowerCase(), password })
			});
			const data = await res.json();
			if (!res.ok) {
				error = await readApiError(res, 'Gagal masuk. Coba lagi.');
				return;
			}
			if (data.two_factor_required) {
				challenge = data.challenge as string;
				return;
			}
			storeSession(data as TokenResponse);
			await goto(await home((data as TokenResponse).user?.role));
		} catch (reason) {
			error = authErrorMessage(reason);
		} finally {
			busy = false;
		}
	}
</script>

<main class="flex min-h-screen items-center justify-center bg-background px-4">
	<div class="w-full max-w-sm rounded-lg border border-border bg-card p-6 shadow-sm">
		<h1 class="text-xl font-semibold tracking-tight">Masuk ke Tulisio</h1>
		<p class="mt-1 text-sm text-muted-foreground">Rewrite Go + SvelteKit</p>

		{#if challenge}
			<p class="mt-4 text-sm">
				Masukkan kode verifikasi <strong>6 digit</strong> dari aplikasi autentikator, atau kode
				pemulihan.
			</p>
		{/if}

		<form class="mt-4 flex flex-col gap-3" onsubmit={submit}>
			{#if challenge}
				<label class="flex flex-col gap-1 text-sm">
					Kode verifikasi
					<input
						class="app-input"
						type="text"
						inputmode="numeric"
						autocomplete="one-time-code"
						bind:value={twoFactorCode}
						placeholder="123456 / kode pemulihan"
						required
					/>
				</label>
			{:else}
				<label class="flex flex-col gap-1 text-sm">
					Email
					<input
						class="app-input"
						type="email"
						bind:value={email}
						placeholder="nama@perusahaan.id"
						autocomplete="email"
						required
					/>
				</label>
				<label class="flex flex-col gap-1 text-sm">
					Password
					<input
						class="app-input"
						type="password"
						bind:value={password}
						autocomplete="current-password"
						required
					/>
				</label>
			{/if}

			{#if error}
				<p class="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>
			{/if}

			<button
				type="submit"
				disabled={busy}
				class="app-btn app-btn-primary w-full"
			>
				{busy ? 'Memproses…' : challenge ? 'Verifikasi' : 'Masuk'}
			</button>
		</form>
	</div>
</main>
