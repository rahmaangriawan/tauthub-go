<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Port /reset-password — token dari query, password baru min 10 + konfirmasi
	// (parity app lama). Shell + gaya sama dengan halaman auth lain (auth.css).
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { Check, CircleAlert } from '@lucide/svelte';
	import AuthShell from '$lib/components/auth/auth-shell.svelte';
	import PageTitle from '$lib/components/page-title.svelte';
	import '$lib/components/auth/auth.css';

	let newPassword = $state('');
	let confirm = $state('');
	let busy = $state(false);
	let error = $state('');
	let success = $state(false);

	const token = $derived(new URLSearchParams(page.url.search).get('token') ?? '');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (busy) return;
		if (newPassword !== confirm) {
			error = tr('Password tidak cocok.');
			return;
		}
		if (newPassword.length < 10) {
			error = tr('Password minimal 10 karakter.');
			return;
		}
		busy = true;
		error = '';
		try {
			const res = await fetch('/v1/auth/reset-password', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ token, new_password: newPassword })
			});
			const data = await res.json();
			if (res.ok && data.ok) {
				success = true;
				setTimeout(() => void goto('/login'), 1500);
			} else {
				// Halaman publik: jangan tampilkan detail server apa adanya.
				error = tr('Token reset tidak valid atau sudah kedaluwarsa.');
			}
		} catch {
			error = tr('Tidak dapat terhubung ke server. Coba lagi.');
		} finally {
			busy = false;
		}
	}
</script>

<PageTitle title={tr("Reset Password")} />

<AuthShell>
	<h1 class="auth-title">{tr("Reset Password")}</h1>
	<p class="auth-subtitle">{tr("Masukkan password baru untuk akun Tauthub Anda.")}</p>

	<div class="auth-body">
	{#if !token}
		<p class="auth-alert" role="alert">
			<CircleAlert size={15} />{tr("Token reset tidak ditemukan. Minta tautan baru.")}
		</p>
		<a class="auth-cta" href="/forgot-password">{tr("Minta tautan baru")}</a>
	{:else if success}
		<div class="auth-verify-ok" role="status">
			<Check size={15} />
			<div>
				{tr("Password berhasil diubah.")}
				<small>{tr("Mengarahkan ke halaman login…")}</small>
			</div>
		</div>
	{:else}
		<form class="auth-form" onsubmit={submit}>
			{#if error}
				<p class="auth-alert" role="alert"><CircleAlert size={15} />{error}</p>
			{/if}
			<label class="auth-field">
				<span class="auth-label">{tr("Password baru")}</span>
				<input
					class="auth-input"
					type="password"
					bind:value={newPassword}
					placeholder={tr("Minimal 10 karakter")}
					autocomplete="new-password"
					minlength={10}
					required
				/>
			</label>
			<label class="auth-field">
				<span class="auth-label">{tr("Konfirmasi password")}</span>
				<input
					class="auth-input"
					type="password"
					bind:value={confirm}
					placeholder={tr("Ulangi password baru")}
					autocomplete="new-password"
					minlength={10}
					required
				/>
			</label>
			<button type="submit" class="auth-cta" disabled={busy}>
				{#if busy}<span class="auth-spinner" aria-hidden="true"></span>{/if}
				{busy ? tr('Menyimpan…') : tr('Simpan password baru')}
			</button>
		</form>
	{/if}
	</div>

	<p class="auth-foot"><a class="auth-link" href="/login">{tr("Kembali ke login")}</a></p>
</AuthShell>

<style>
	/* Jarak subtitle → konten, ritme sama dengan .auth-switch di halaman login. */
	.auth-body { margin-top: 22px; }
</style>
