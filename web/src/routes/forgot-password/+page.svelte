<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Port /forgot-password — selalu netral (anti-enumerasi, parity API).
	// Shell + gaya dipakai bersama /login dan /register (auth.css), plus parity
	// Turnstile app lama: token wajib sebelum submit saat verifikasi aktif.
	import { Check, CircleAlert } from '@lucide/svelte';
	import AuthShell from '$lib/components/auth/auth-shell.svelte';
	import TurnstileWidget from '$lib/components/turnstile.svelte';
	import PageTitle from '$lib/components/page-title.svelte';
	import '$lib/components/auth/auth.css';

	let email = $state('');
	let busy = $state(false);
	let sent = $state(false);
	let error = $state('');

	// Turnstile (parity: token single-use, render ulang tiap dipakai/kedaluwarsa).
	let turnstileEnabled = $state(false);
	let turnstileSiteKey = $state('');
	let turnstileToken = $state('');
	let turnstileNonce = $state(0);

	$effect(() => {
		void (async () => {
			try {
				const res = await fetch('/v1/auth/turnstile', { cache: 'no-store' });
				if (!res.ok) return;
				const data = (await res.json()) as { enabled?: boolean; site_key?: string | null };
				turnstileEnabled = Boolean(data.enabled);
				turnstileSiteKey = data.site_key ?? '';
			} catch {
				// Turnstile tak tersedia: form tetap bisa dipakai (parity legacy).
			}
		})();
	});

	function resetTurnstile() {
		if (!turnstileEnabled) return;
		turnstileToken = '';
		turnstileNonce += 1;
	}

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (busy) return;
		if (turnstileEnabled && !turnstileToken) {
			error = tr('Selesaikan verifikasi keamanan untuk melanjutkan.');
			return;
		}
		busy = true;
		error = '';
		try {
			const res = await fetch('/v1/auth/forgot-password', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ email: email.trim().toLowerCase(), turnstile_token: turnstileToken })
			});
			if (res.ok) {
				// Anti-enumerasi: backend selalu ok:true, jadi pesan tetap netral.
				sent = true;
			} else {
				error = tr('Gagal mengirim permintaan. Coba lagi.');
				resetTurnstile();
			}
		} catch {
			error = tr('Tidak dapat terhubung ke server. Coba lagi.');
			resetTurnstile();
		} finally {
			busy = false;
		}
	}
</script>

<PageTitle title={tr("Lupa Password")} />

<AuthShell>
	<h1 class="auth-title">{tr("Lupa Password")}</h1>
	<p class="auth-subtitle">{tr("Masukkan email akun Tauthub Anda. Kami akan mengirimkan link reset password.")}</p>

	<div class="auth-body">
	{#if sent}
		<div class="auth-verify-ok" role="status">
			<Check size={15} />
			<div>
				{tr("Jika email terdaftar, link reset password telah dikirim.")}
				<small>{tr("Periksa folder inbox dan spam Anda. Link berlaku selama 10 menit.")}</small>
			</div>
		</div>
	{:else}
		<form class="auth-form" onsubmit={submit}>
			{#if error}
				<p class="auth-alert" role="alert"><CircleAlert size={15} />{error}</p>
			{/if}
			<label class="auth-field">
				<span class="auth-label">{tr("Email")}</span>
				<input
					class="auth-input"
					type="email"
					bind:value={email}
					placeholder={tr("email@contoh.com")}
					autocomplete="email"
					required
				/>
			</label>
			{#if turnstileEnabled}
				<div class="auth-verify">
					{#key turnstileNonce}
						<TurnstileWidget
							siteKey={turnstileSiteKey}
							onToken={(token) => (turnstileToken = token)}
							onExpire={() => (turnstileToken = '')}
						/>
					{/key}
				</div>
			{/if}
			<button type="submit" class="auth-cta" disabled={busy || (turnstileEnabled && !turnstileToken)}>
				{#if busy}<span class="auth-spinner" aria-hidden="true"></span>{/if}
				{busy ? tr('Mengirim…') : tr('Kirim tautan reset')}
			</button>
		</form>
	{/if}
	</div>

	<p class="auth-foot"><a class="auth-link" href="/login">← {tr("Kembali ke login")}</a></p>
</AuthShell>

<style>
	/* Jarak subtitle → konten, ritme sama dengan .auth-switch di halaman login. */
	.auth-body { margin-top: 22px; }
</style>
