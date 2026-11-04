<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Panel authentication — satu komponen untuk mode Masuk & Daftar.
	// Port login-form.tsx: logika bisnis dipertahankan apa adanya (endpoint,
	// payload, referral code, auto-login setelah daftar, 2FA challenge,
	// Turnstile single-use). Yang berubah hanya UI/UX.
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Eye, EyeOff, Info, CircleAlert, Check } from '@lucide/svelte';
	import {
		storeSession,
		authErrorMessage,
		apiRequest,
		readApiError,
		resolveHomeRoute,
		type TokenResponse
	} from '$lib/auth';
	import TurnstileWidget from '$lib/components/turnstile.svelte';
	
	type Mode = 'login' | 'register';
	type Role = 'client' | 'partner';

	let {
		initialMode = 'login',
		lockedRole = null
	}: { initialMode?: Mode; lockedRole?: Role | null } = $props();

	// initialMode/lockedRole sengaja hanya dipakai sebagai nilai awal saat mount
	// (dibaca lewat untrack agar tidak dianggap dependensi reaktif). Setelah itu
	// mode dikendalikan pengguna lewat segmented switch dan parameter URL.
	let mode = $state<Mode>(untrack(() => initialMode));
	let role = $state<Role>(untrack(() => lockedRole) ?? 'client');

	let email = $state('');
	let password = $state('');
	let displayName = $state('');

	let showPassword = $state(false);
	let busy = $state(false);
	let error = $state('');
	let success = $state('');

	// Error per-field: hanya untuk validasi klien (brief §12: pesan dekat field).
	let fieldErrors = $state<{ displayName?: string; email?: string; password?: string }>({});

	// 2FA step-2
	let challenge = $state<string | null>(null);
	let twoFactorCode = $state('');

	// Daftar lewat email: isi nama/email/password → "Daftar" mengirim kode OTP dan
	// mengganti form dengan panel kode (seperti panel 2FA) → "Verifikasi" memverifikasi
	// kode lalu langsung melanjutkan pendaftaran dengan tiketnya (/v1/auth/register).
	let otpPending = $state(false);
	let otpCode = $state('');
	let emailTicket = $state('');
	let resendAt = $state(0);
	let nowTick = $state(Date.now());
	let otpInputEl = $state<HTMLInputElement | null>(null);
	const inOtp = $derived(mode === 'register' && otpPending && !challenge);
	$effect(() => {
		if (!inOtp) return;
		const t = setInterval(() => (nowTick = Date.now()), 1000);
		return () => clearInterval(t);
	});
	$effect(() => {
		if (inOtp) otpInputEl?.focus();
	});
	const resendLeft = $derived(Math.max(0, Math.ceil((resendAt - nowTick) / 1000)));
	const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
		// Turnstile (parity: token single-use, reset + render ulang setiap dipakai).
	let turnstileEnabled = $state(false);
	let turnstileSiteKey = $state('');
	let turnstileToken = $state('');
	let turnstileNonce = $state(0);
	let verified = $state(false);

	let googleBusy = $state(false);

	// Turnstile menjaga langkah yang mengirim sesuatu ke publik: login dan "Daftar"
	// (kirim kode). Di panel OTP widget hanya perlu saat kirim ulang kode.
	const needsTurnstile = $derived(turnstileEnabled && !challenge && (!inOtp || resendLeft === 0));
	const turnstileBlocked = $derived(
		turnstileEnabled && !turnstileToken && !challenge && !inOtp
	);

		const home = (r?: string) => resolveHomeRoute(r);

	// Turnstile aktif? (endpoint publik /v1/auth/turnstile)
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

	// Sinkronkan mode dengan URL (?mode=register) tanpa memuat ulang halaman.
	const modeParam = $derived(page.url.searchParams.get('mode'));
	$effect(() => {
		if (modeParam === 'register') mode = 'register';
		else if (modeParam === 'login') mode = 'login';
	});

	// /register-role mengembalikan ke sini saat tiket OTP kedaluwarsa.
	$effect(() => {
		if (page.url.searchParams.get('otp') === 'expired') {
			mode = 'register';
			error = tr('Verifikasi email kedaluwarsa. Minta kode baru.');
		}
	});

	/** Kembali dari panel OTP ke form pendaftaran (isian tetap terisi). */
	function backToForm() {
		otpPending = false;
		otpCode = '';
		emailTicket = '';
		success = '';
		error = '';
		resetTurnstile();
	}

	/** Kirim kode OTP ke email di form lalu tampilkan panel kode. */
	async function requestCode() {
		const target = email.trim().toLowerCase();
		const res = await fetch('/v1/auth/register/request-code', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ email: target, turnstile_token: turnstileToken || undefined })
		});
		if (!res.ok) throw new Error(await readApiError(res, tr('Gagal mengirim kode. Coba lagi.')));
		const data = (await res.json()) as { retry_after?: number };
		otpPending = true;
		otpCode = '';
		emailTicket = '';
		resendAt = Date.now() + (data.retry_after ?? 60) * 1000;
		nowTick = Date.now();
		success = `${tr('Kode verifikasi dikirim ke')} ${target}. ${tr('Cek kotak masuk dan folder spam.')}`;
	}

	async function resendCode() {
		if (busy || resendLeft > 0) return;
		if (turnstileEnabled && !turnstileToken) {
			error = tr('Selesaikan verifikasi keamanan untuk melanjutkan.');
			return;
		}
		busy = true;
		error = '';
		try {
			await requestCode();
		} catch (reason) {
			error = authErrorMessage(reason);
		} finally {
			resetTurnstile();
			busy = false;
		}
	}

	/** Kolom kode: hanya angka (paste "123 456" tetap jadi 123456). */
	function onCodeInput(value: string) {
		otpCode = value.replace(/\D/g, '').slice(0, 6);
	}

	/** Tukar kode dengan tiket verifikasi email; melempar bila kode salah. */
	async function verifyCode() {
		const res = await fetch('/v1/auth/register/verify-code', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ email: email.trim().toLowerCase(), code: otpCode.trim() })
		});
		if (!res.ok) throw new Error(await readApiError(res, tr('Kode salah atau sudah kedaluwarsa.')));
		const data = (await res.json()) as { ticket?: string };
		if (!data.ticket) throw new Error(tr('Kode salah atau sudah kedaluwarsa.'));
		emailTicket = data.ticket;
	}

	function switchMode(next: Mode) {
		if (mode === next) return;
		mode = next;
		error = '';
		success = '';
		fieldErrors = {};
		otpPending = false;
		otpCode = '';
		emailTicket = '';
		// Jangan biarkan token Turnstile bekas dipakai ulang.
		turnstileToken = '';
		verified = false;
		turnstileNonce += 1;
	}

		function applyServerError(message: string) {
		const lower = message.toLowerCase();
		if (lower.includes('email')) fieldErrors = { ...fieldErrors, email: message };
		else if (lower.includes('password')) fieldErrors = { ...fieldErrors, password: message };
		else error = message;
	}

	function validateRegister(): boolean {
		const next: typeof fieldErrors = {};
		if (displayName.trim().length < 2) next.displayName = tr('Nama minimal 2 karakter.');
		if (!EMAIL_RE.test(email.trim())) next.email = tr('Email tidak valid.');
		if (password.length < 10) next.password = tr('Password minimal 10 karakter.');
		fieldErrors = next;
		return Object.keys(next).length === 0;
	}

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (busy) return;
		if (turnstileBlocked) {
			error = tr('Selesaikan verifikasi keamanan untuk melanjutkan.');
			return;
		}
		if (mode === 'register') {
			if (inOtp) {
				if (!emailTicket && !/^\d{6}$/.test(otpCode.trim())) {
					error = tr('Kode harus 6 digit.');
					return;
				}
			} else if (!validateRegister()) {
				return;
			}
		}

		busy = true;
		error = '';
		if (!inOtp) success = '';
		try {
			if (mode === 'register') {
				if (inOtp) {
					// Tiket sudah ada bila verifikasi berhasil tapi pendaftaran sempat gagal:
					// jangan verifikasi ulang (kode sudah terpakai), langsung daftar.
					if (!emailTicket) await verifyCode();
					success = '';
					await doRegister();
				} else {
					await requestCode();
				}
			} else {
				await doLogin();
			}
		} catch (reason) {
			const message = authErrorMessage(reason);
			if (message.includes('Verifikasi email')) {
				// Tiket OTP kedaluwarsa: minta kode baru dari panel OTP.
				emailTicket = '';
				otpCode = '';
				error = tr('Verifikasi email kedaluwarsa. Minta kode baru.');
			} else if (inOtp) {
				// Salah kode / gagal daftar: tampil sebagai banner di panel OTP.
				if (!emailTicket) otpCode = '';
				error = message;
			} else {
				applyServerError(message);
			}
			if (!inOtp) resetTurnstile();
		} finally {
			busy = false;
		}
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

	async function doRegister() {
		// Welcome Gate: form daftar tidak lagi menanyakan peran. Setelah isian
		// lengkap, draf disimpan supaya halaman /register-role (pemilihan peran)
		// bisa melanjutkan pendaftaran dengan peran pilihan user.
		if (!lockedRole) {
			const draft = {
				display_name: displayName.trim(),
				email: email.trim().toLowerCase(),
				password,
				turnstile_token: '',
				email_ticket: emailTicket
			};
			try {
				sessionStorage.setItem('tauthub_reg_draft', JSON.stringify(draft));
			} catch {
				// sessionStorage bisa diblokir: jatuh ke pendaftaran dengan default,
				// lebih baik daripada pendaftaran macet.
				const payload: Record<string, unknown> = {
					email: draft.email, password: draft.password, display_name: draft.display_name,
					role: 'client', email_ticket: draft.email_ticket
				};
				await postRegister(payload);
				return;
			}
			const params = new URLSearchParams();
			const ref = referralCode();
			if (ref) params.set('ref', ref);
			const returnTo = page.url.searchParams.get('returnTo');
			if (returnTo) params.set('returnTo', returnTo);
			await goto('/register-role' + (params.size ? `?${params}` : ''));
			return;
		}

		const ref = referralCode();
		const payload: Record<string, unknown> = {
			email: email.trim().toLowerCase(),
			password,
			display_name: displayName.trim(),
			role: lockedRole,
			// App lama tidak lagi menanyakan kategori media; nilai tetap 'lainnya'
			// karena backend mewajibkannya untuk peran Mitra.
			partner_category: lockedRole === 'partner' ? 'lainnya' : undefined,
			// Turnstile sudah dipakai saat minta kode; /register memeriksa tiket OTP.
			email_ticket: emailTicket
		};
		await postRegister(payload);
	}

	/** Kirim pendaftaran + auto-login; dipakai dari panel auth dan gate. */
	async function postRegister(payload: Record<string, unknown>) {
		const ref = referralCode();
		if (ref) (payload as Record<string, unknown>).referral_code = ref;

		const headers: Record<string, string> = { 'Content-Type': 'application/json' };
		if (ref) headers['x-referral-code'] = ref;

		const res = await fetch('/v1/auth/register', {
			method: 'POST',
			headers,
			body: JSON.stringify(payload)
		});
		if (!res.ok) throw new Error(await readApiError(res, tr('Pendaftaran gagal. Periksa isian lalu coba lagi.')));

		// Auto-login agar user langsung masuk ke dasbor perannya (parity legacy).
		const loginRes = await fetch('/v1/auth/login', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ email: payload.email, password: payload.password })
		});
		if (!loginRes.ok) {
			// Akun dibuat tetapi auto-login gagal: arahkan ke mode Masuk.
			mode = 'login';
			success = tr('Pendaftaran berhasil! Silakan masuk dengan email dan password Anda.');
			try {
				const body = (await loginRes.json()) as { detail?: string };
				if (typeof body.detail === 'string') error = body.detail;
			} catch {
				// pesan default sudah cukup
			}
			resetTurnstile();
			return;
		}
		const data = (await loginRes.json()) as TokenResponse;
		storeSession(data);
		await goto(await home(data.user?.role));
	}

	async function doLogin() {
		if (challenge) {
			const res = await fetch('/v1/auth/2fa/verify', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ challenge, code: twoFactorCode.trim() })
			});
			const data = await res.json();
			if (!res.ok) throw new Error(await readApiError(res, tr('Kode verifikasi salah.')));
			storeSession(data as TokenResponse);
			await goto(await home((data as TokenResponse).user?.role));
			return;
		}

		const res = await fetch('/v1/auth/login', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				email: email.trim().toLowerCase(),
				password,
				turnstile_token: turnstileToken
			})
		});
		if (!res.ok) throw new Error(await readApiError(res, tr('Email atau password tidak cocok.')));

		const data = (await res.json()) as TokenResponse & { two_factor_required?: boolean; challenge?: string };
		if (data.two_factor_required && data.challenge) {
			challenge = data.challenge;
			// Token Turnstile sudah terpakai pada langkah password.
			resetTurnstile();
			return;
		}
		storeSession(data);
		await goto(await home(data.user?.role));
	}

	function resetTurnstile() {
		if (!turnstileEnabled) return;
		turnstileToken = '';
		verified = false;
		turnstileNonce += 1;
	}

	/** Tombol Google: mode Daftar langsung ke consent Google; peran dipilih di
	 *  Welcome Gate (/register-role) setelah callback — bukan sebelum consent. */
	function startGoogle() {
		if (mode === 'login') {
			void goGoogle('login', 'client');
			return;
		}
		// Halaman peran terkunci (mis. /register/mitra) langsung ke OAuth tanpa
		// gate: perannya sudah pasti dari URL.
		if (lockedRole) {
			void goGoogle('register', lockedRole);
			return;
		}
		// Tanpa peran → callback menaruh user di gate pemilihan peran.
		void goGoogle('register', '');
	}

	function googleUrl(m: Mode, r: Role | ''): string {
		const params = new URLSearchParams({ mode: m });
		// Gate: tanpa role → callback menaruh user di /register-role (pemilihan
		// peran) alih-alih langsung membuat akun dengan peran tertentu.
		if (r) params.set('role', r);
		const returnTo = page.url.searchParams.get('returnTo');
		if (returnTo) params.set('returnTo', returnTo);
		return `/v1/auth/google/url?${params}`;
	}

	/**
	 * Cek dulu apakah Google Sign-In siap (server balas 302 → opaqueredirect).
	 * Tanpa ini, konfigurasi yang belum lengkap melempar user ke halaman JSON
	 * 503 dan mereka buntu tanpa cara kembali selain tombol back.
	 */
	async function goGoogle(m: Mode, r: Role | '') {
		const url = googleUrl(m, r);
		googleBusy = true;
		error = '';
		try {
			const probe = await fetch(url, { redirect: 'manual', cache: 'no-store' });
			if (probe.type !== 'opaqueredirect' && !probe.ok) {
				// Pesan server tidak pernah ditampilkan ke publik (bisa memuat
				// detail konfigurasi internal). Tampilkan pesan generik saja.
				error = tr('Error, coba lagi nanti.');
				googleBusy = false;
				return;
			}
		} catch {
			// Pratinjau gagal (mis. CORS) — biarkan navigasi yang menentukan.
		}
		window.location.href = url;
	}

</script>

<h1 class="auth-title">
	{challenge
		? tr('Verifikasi Dua Langkah')
		: inOtp
			? tr('Verifikasi email')
			: mode === 'login'
				? tr('Selamat Datang Kembali')
				: tr('Buat Akun Tauthub')}
</h1>
<p class="auth-subtitle">
	{#if challenge}
		{tr('Masukkan kode 6 digit dari aplikasi autentikator Anda.')}
	{:else if inOtp}
		{tr('Masukkan 6 digit kode yang kami kirim ke')} <strong>{email.trim().toLowerCase()}</strong>.
	{:else if mode === 'login'}
		{tr('Masuk untuk melanjutkan pekerjaan brand Anda.')}
	{:else}
		{tr('Daftar untuk membuat brand pertama Anda.')}
	{/if}
</p>

{#if !challenge && !inOtp}
	<!-- Segmented switch: tetap di posisi yang sama untuk kedua mode (brief §4-§5). -->
	<div class="auth-switch" role="tablist" aria-label={tr("Pilih Masuk atau Daftar")}>
		<button
			type="button"
			role="tab"
			aria-selected={mode === 'login'}
			id="tab-masuk"
			aria-controls="auth-panel"
			onclick={() => switchMode('login')}
		>{tr("Masuk")}</button>
		<button
			type="button"
			role="tab"
			aria-selected={mode === 'register'}
			id="tab-daftar"
			aria-controls="auth-panel"
			onclick={() => switchMode('register')}
		>{tr("Daftar")}</button>
	</div>

	<button type="button" class="auth-google" disabled={busy || googleBusy} onclick={startGoogle}>
		<svg width="16" height="16" viewBox="0 0 18 18" aria-hidden="true">
			<path fill="#4285F4" d="M17.64 9.2c0-.64-.06-1.25-.16-1.84H9v3.48h4.84a4.14 4.14 0 0 1-1.8 2.72v2.26h2.92c1.7-1.57 2.68-3.88 2.68-6.62z" />
			<path fill="#34A853" d="M9 18c2.43 0 4.47-.8 5.96-2.18l-2.92-2.26c-.8.54-1.84.86-3.04.86-2.34 0-4.32-1.58-5.03-3.7H.96v2.33A9 9 0 0 0 9 18z" />
			<path fill="#FBBC05" d="M3.97 10.72a5.41 5.41 0 0 1 0-3.44V4.95H.96a9 9 0 0 0 0 8.1l3.01-2.33z" />
			<path fill="#EA4335" d="M9 3.58c1.32 0 2.5.45 3.44 1.35l2.58-2.58C13.46.9 11.43 0 9 0A9 9 0 0 0 .96 4.95l3.01 2.33C4.68 5.16 6.66 3.58 9 3.58z" />
		</svg>
		{mode === 'login' ? tr('Masuk dengan Google') : tr('Daftar dengan Google')}
	</button>

	<div class="auth-divider"><span>{tr("atau")}</span></div>
{/if}

<!-- aria-label menggantikan role tabpanel: panel ini satu-satunya panel,
	     jadi tidak ada pasangan tab yang perlu dihubungkan. -->
<form
	class="auth-form"
	class:auth-form--register={mode === 'register' && !challenge && !inOtp}
	id="auth-panel"
	aria-label={mode === 'login' ? tr('Form masuk') : tr('Form pendaftaran')}
	novalidate={mode === 'register'}
	onsubmit={submit}
>
	{#if challenge}
		<div class="auth-2fa-note">{tr("Buka aplikasi autentikator (Google Authenticator / Authy) atau gunakan salah satu kode pemulihan Anda.")}</div>
		<label class="auth-field">
			<span class="auth-label">{tr("Kode verifikasi")}</span>
			<input
				class="auth-input"
				type="text"
				inputmode="numeric"
				autocomplete="one-time-code"
				bind:value={twoFactorCode}
				placeholder="123456"
				required
			/>
		</label>
	{:else if inOtp}
		<label class="auth-field">
			<span class="auth-label">{tr("Kode verifikasi")}</span>
			<input
				class="auth-input auth-input--code"
				type="text"
				inputmode="numeric"
				autocomplete="one-time-code"
				value={otpCode}
				placeholder="000000"
				bind:this={otpInputEl}
				oninput={(e) => {
					// Saring ke 6 digit angka (paste "123 456" tetap utuh) lalu paksa isi kolom
					// sama dengan state, supaya huruf yang diketik tidak sempat tampil.
					const el = e.currentTarget as HTMLInputElement;
					onCodeInput(el.value);
					el.value = otpCode;
				}}
			/>
		</label>
		<p class="auth-resend">
			{tr('Belum menerima kode?')}
			<button type="button" class="auth-link" disabled={busy || resendLeft > 0} onclick={resendCode}>
				{tr('Kirim ulang kode')}{resendLeft > 0 ? ` (${resendLeft}s)` : ''}
			</button>
		</p>
	{:else}
		{#if mode === 'register'}
			<label class="auth-field">
				<span class="auth-label">{tr("Nama atau perusahaan")}</span>
				<input
					class="auth-input"
					type="text"
					bind:value={displayName}
					placeholder={tr("Budi Santoso atau PT Maju Jaya")}
					autocomplete="name"
					minlength={2}
					maxlength={120}
					aria-invalid={fieldErrors.displayName ? 'true' : undefined}
					required
				/>
				{#if fieldErrors.displayName}
					<p class="auth-field-error"><CircleAlert size={13} />{fieldErrors.displayName}</p>
				{/if}
			</label>
		{/if}

		<label class="auth-field">
			<span class="auth-label">{tr("Email")}</span>
			<input
				class="auth-input"
				type="email"
				bind:value={email}
				placeholder={tr("nama@gmail.com")}
				autocomplete="email"
				aria-invalid={fieldErrors.email ? 'true' : undefined}
				required
			/>
			{#if fieldErrors.email}
				<p class="auth-field-error"><CircleAlert size={13} />{fieldErrors.email}</p>
			{/if}
		</label>

				<div class="auth-field" class:auth-field--span={mode === 'register' && !lockedRole}>
			<div class="auth-field-head">
				<label class="auth-label" for="auth-password">{tr("Password")}</label>
				{#if mode === 'login'}
					<a class="auth-link" href="/forgot-password">{tr("Lupa password?")}</a>
				{:else if !fieldErrors.password}
					<p class="auth-helper">{tr("Minimal 10 karakter.")}</p>
				{/if}
			</div>
			<span class="auth-input-wrap">
				<input
					id="auth-password"
					class="auth-input has-affix"
					type={showPassword ? 'text' : 'password'}
					bind:value={password}
					placeholder={tr("Masukkan password")}
					autocomplete={mode === 'register' ? 'new-password' : 'current-password'}
					minlength={mode === 'register' ? 10 : undefined}
					aria-invalid={fieldErrors.password ? 'true' : undefined}
					required
				/>
				<button
					type="button"
					class="auth-eye"
					aria-label={showPassword ? tr('Sembunyikan password') : tr('Tampilkan password')}
					aria-pressed={showPassword}
					onclick={() => (showPassword = !showPassword)}
				>
					{#if showPassword}<EyeOff size={16} />{:else}<Eye size={16} />{/if}
				</button>
			</span>
			{#if fieldErrors.password}
				<p class="auth-field-error"><CircleAlert size={13} />{fieldErrors.password}</p>
			{/if}
		</div>

		<!-- Welcome Gate: peran dipilih di /register-role setelah form; radios ini
		     tersisa hanya untuk halaman peran terkunci (/register/klien, /register/mitra). -->
		{#if mode === 'register' && lockedRole}
			<div class="auth-field">
				<span class="auth-label">{tr("Daftar sebagai")}<!-- Tooltip penjelas peran: muncul saat hover DAN focus agar dapat
					     diakses keyboard (parity Tooltip app lama pada label ini). -->
					<span class="auth-tip">
						<button
							type="button"
							class="auth-info"
							aria-label={tr("Penjelasan peran Klien dan Mitra")}
							aria-describedby="auth-role-tip"
						>
							<Info size={13} />
						</button>
						<span class="auth-tip-panel" role="tooltip" id="auth-role-tip">
							<span class="auth-tip-row"><b>{tr("Klien:")}</b> {tr("kelola kampanye, analisis, dan distribusikan lewat mitra.")}</span>
							<span class="auth-tip-row"><b>{tr("Mitra:")}</b> {tr("terima publikasi dari klien dan hasilkan uang dari distribusi konten.")}</span>
						</span>
					</span>
				</span>
				<div class="auth-roles">
					<label class="auth-role" data-checked={role === 'client'}>
						<input type="radio" name="role" value="client" checked={role === 'client'} onchange={() => (role = 'client')} />{tr("Klien")}</label>
					<label class="auth-role" data-checked={role === 'partner'}>
						<input type="radio" name="role" value="partner" checked={role === 'partner'} onchange={() => (role = 'partner')} />{tr("Mitra")}</label>
				</div>
			</div>
		{/if}
	{/if}

		{#if error}
		<p class="auth-alert" role="alert"><CircleAlert size={15} />{error}</p>
	{:else if page.url.searchParams.get('oauth') === 'invalid_state'}
		<!-- Callback Google memanggil /login?oauth=invalid_state saat cookie tiket
		     (goauth_state, 10 menit, sekali pakai) tidak ada — biasanya karena
		     alur dibuka dari tautan/tab lama yang sudah kedaluwarsa. -->
		<p class="auth-alert" role="alert">
			<CircleAlert size={15} />
			{tr('Sesi Google sudah kedaluwarsa. Klik tombol Google sekali lagi untuk mulai dari awal.')}
		</p>
	{/if}
	{#if success}
		<p class="auth-verify-ok" role="status"><Check size={15} />{success}</p>
	{/if}

	<!-- Verifikasi keamanan Cloudflare (brief §4-§5). -->
	{#if needsTurnstile}
		<div class="auth-verify">
			{#if verified && turnstileToken}
				<div class="auth-verify-ok" role="status">
					<Check size={15} />
					<div>{tr("Berhasil!")}<small>{tr("Verifikasi keamanan berhasil.")}</small>
					</div>
				</div>
			{:else}
				{#key turnstileNonce}
					<TurnstileWidget
						siteKey={turnstileSiteKey}
						onToken={(token) => {
							turnstileToken = token;
							verified = true;
						}}
						onExpire={() => {
							turnstileToken = '';
							verified = false;
						}}
					/>
				{/key}
			{/if}
		</div>
	{/if}

	<button
		type="submit"
		class="auth-cta"
		disabled={busy || turnstileBlocked || (inOtp && !emailTicket && otpCode.length < 6)}
	>
		{#if busy}<span class="auth-spinner" aria-hidden="true"></span>{/if}
		{busy
			? challenge
				? 'Memverifikasi…'
				: mode === 'login'
					? 'Memproses…'
					: inOtp
						? emailTicket
							? 'Mendaftarkan…'
							: 'Memverifikasi…'
						: tr('Mengirim kode…')
			: challenge
				? tr('Verifikasi & masuk')
				: mode === 'login'
					? tr('Masuk')
					: inOtp
						? tr('Verifikasi')
						: tr('Daftar')}
	</button>
</form>

{#if challenge}
	<p class="auth-2fa-back">
		<button
			type="button"
			class="auth-link"
			onclick={() => {
				challenge = null;
				twoFactorCode = '';
				error = '';
				resetTurnstile();
			}}
		>{tr("Kembali ke form masuk")}</button>
	</p>
{:else if inOtp}
	<p class="auth-2fa-back">
		<button type="button" class="auth-link" onclick={backToForm}>{tr("Ubah data pendaftaran")}</button>
	</p>
{:else}
	<p class="auth-privacy">
		{tr('Dengan melanjutkan, Anda menyetujui')}
		<a href="https://tauthub.com/kebijakan-privasi/" target="_blank" rel="noreferrer">{tr('kebijakan privasi')}</a>.
	</p>
{/if}
