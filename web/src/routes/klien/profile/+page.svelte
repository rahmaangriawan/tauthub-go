<script lang="ts">
	// Minimal /klien/profile — port fungsi profil: ganti password + 2FA setup/
	// enable/disable. Section profil penuh ada di fase 2.
	import { onMount } from 'svelte';
	import {
		apiRequest,
		readApiError,
		getStoredSession,
		ensureValidSession,
		updateStoredUser,
		type AuthUser
	} from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	let user = $state<AuthUser | null>(null);

	// Ganti password
	let currentPassword = $state('');
	let newPassword = $state('');
	let passwordMsg = $state('');

	// 2FA
	let twoFactorSecret = $state('');
	let otpauthUrl = $state('');
	let totpCode = $state('');
	let twoFactorMsg = $state('');
	let recoveryCodes = $state<string[]>([]);
	let twoFactorBusy = $state(false);

	onMount(async () => {
		const fresh = await ensureValidSession();
		if (fresh) user = fresh.user;
	});

	async function changePassword(e: SubmitEvent) {
		e.preventDefault();
		passwordMsg = '';
		const res = await apiRequest('/v1/auth/change-password', {
			method: 'POST',
			body: JSON.stringify({ current_password: currentPassword, new_password: newPassword })
		});
		if (res.status === 204) {
			passwordMsg = 'Password berhasil diubah.';
			currentPassword = '';
			newPassword = '';
		} else {
			passwordMsg = await readApiError(res, 'Gagal mengubah password.');
		}
	}

	async function startTwoFactor() {
		twoFactorBusy = true;
		twoFactorMsg = '';
		try {
			const res = await apiRequest('/v1/auth/2fa/setup', { method: 'POST' });
			const data = await res.json();
			if (res.ok) {
				twoFactorSecret = data.secret;
				otpauthUrl = data.otpauth_url;
			} else {
				twoFactorMsg = await readApiError(res, 'Gagal memulai setup 2FA.');
			}
		} finally {
			twoFactorBusy = false;
		}
	}

	async function enableTwoFactor(e: SubmitEvent) {
		e.preventDefault();
		twoFactorBusy = true;
		twoFactorMsg = '';
		try {
			const res = await apiRequest('/v1/auth/2fa/enable', {
				method: 'POST',
				body: JSON.stringify({ code: totpCode.trim() })
			});
			const data = await res.json();
			if (res.ok) {
				recoveryCodes = data.recovery_codes ?? [];
				twoFactorSecret = '';
				totpCode = '';
				twoFactorMsg = '2FA aktif. Simpan kode pemulihan Anda.';
			} else {
				twoFactorMsg = await readApiError(res, 'Kode verifikasi salah. Coba lagi.');
			}
		} finally {
			twoFactorBusy = false;
		}
	}

	async function disableTwoFactor() {
		const password = window.prompt('Konfirmasi password untuk menonaktifkan 2FA:');
		if (!password) return;
		twoFactorBusy = true;
		twoFactorMsg = '';
		try {
			const res = await apiRequest('/v1/auth/2fa/disable', {
				method: 'POST',
				body: JSON.stringify({ password })
			});
			const data = await res.json();
			if (res.ok) {
				recoveryCodes = [];
				twoFactorMsg = '2FA dinonaktifkan.';
			} else {
				twoFactorMsg = await readApiError(res, 'Gagal menonaktifkan 2FA.');
			}
		} finally {
			twoFactorBusy = false;
		}
	}

	async function rename() {
		if (!user) return;
		const name = window.prompt('Nama tampilan baru:', user.display_name);
		if (!name || name.trim().length < 2) return;
		const res = await apiRequest('/v1/auth/me', {
			method: 'PATCH',
			body: JSON.stringify({ display_name: name.trim() })
		});
		if (res.ok) {
			const updated = (await res.json()) as AuthUser;
			user = updated;
			updateStoredUser(updated);
		}
	}
</script>

<PageTitle title="Profil" />

<main class="px-4 py-8">
	<h1 class="text-2xl font-semibold tracking-tight">Profil</h1>
	{#if user}
		<p class="mt-1 text-sm text-muted-foreground">{user.email} · <span class="uppercase">{user.role}</span></p>
		<button class="mt-2 rounded-md border border-border px-3 py-1.5 text-sm hover:bg-accent" onclick={rename}>
			Ubah nama tampilan
		</button>

		<section class="app-card mt-8 p-4">
			<h2 class="font-medium">Ganti password</h2>
			<form class="mt-3 flex flex-col gap-3" onsubmit={changePassword}>
				<input
					class="app-input"
					type="password" placeholder="Password saat ini" bind:value={currentPassword} required
				/>
				<input
					class="app-input"
					type="password" placeholder="Password baru (min. 10)" bind:value={newPassword} minlength={10} required
				/>
				{#if passwordMsg}<p class="text-sm text-muted-foreground">{passwordMsg}</p>{/if}
				<button class="app-btn app-btn-primary w-fit" type="submit">
					Simpan password
				</button>
			</form>
		</section>

		<section class="app-card mt-6 p-4">
			<h2 class="font-medium">Two-Factor Authentication</h2>
			{#if recoveryCodes.length > 0}
				<p class="mt-2 text-sm text-emerald-600">2FA aktif. Kode pemulihan (simpan sekarang, hanya sekali):</p>
				<code class="mt-2 block whitespace-pre-wrap rounded bg-muted p-2 text-xs">{recoveryCodes.join('\n')}</code>
				<button
					class="mt-3 rounded-md border border-destructive px-3 py-1.5 text-sm text-destructive hover:bg-destructive/10"
					disabled={twoFactorBusy} onclick={disableTwoFactor}
				>
					Nonaktifkan 2FA
				</button>
			{:else if twoFactorSecret}
				<p class="mt-2 text-sm text-muted-foreground">
					Pindai QR / masukkan secret ke aplikasi autentikator, lalu masukkan kode 6 digit pertama.
				</p>
				<code class="mt-2 block break-all rounded bg-muted p-2 text-xs">{twoFactorSecret}</code>
				{#if otpauthUrl}
					<p class="mt-2 break-all text-xs text-muted-foreground">{otpauthUrl}</p>
				{/if}
				<form class="mt-3 flex gap-2" onsubmit={enableTwoFactor}>
					<input
						class="w-32 rounded-md border border-input bg-background px-3 py-2 text-sm"
						type="text" inputmode="numeric" placeholder="123456" bind:value={totpCode} required
					/>
					<button class="app-btn app-btn-primary" type="submit" disabled={twoFactorBusy}>
						Aktifkan
					</button>
				</form>
			{:else}
				<button
					class="mt-3 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground"
					disabled={twoFactorBusy} onclick={startTwoFactor}
				>
					{twoFactorBusy ? 'Menyiapkan…' : 'Mulai setup 2FA'}
				</button>
			{/if}
			{#if twoFactorMsg}<p class="mt-2 text-sm text-muted-foreground">{twoFactorMsg}</p>{/if}
		</section>
	{:else}
		<p class="mt-2 text-sm text-muted-foreground">Memuat…</p>
	{/if}
</main>
