<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Profil & Akun — tab Akun / Preferensi notifikasi / Keamanan. Semua endpoint
	// sudah ada (fase 1 & 2), jadi tidak ada data placeholder.
	import { onMount } from 'svelte';
	import { Check, CircleAlert, KeyRound, MessageCircle, ShieldCheck, UserCircle, Eye, EyeOff } from '@lucide/svelte';
	import {
		apiRequest, readApiError, ensureValidSession, getStoredSession,
		updateStoredUser, AUTH_EXPIRED_EVENT, type AuthUser
	} from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
	import DevicePushCard from '$lib/components/device-push-card.svelte';
	import { renderSVG } from 'uqr';

	// Tab halaman; label lewat $derived karena locale baru siap setelah layout init.
	const TABS = $derived<Array<{ key: 'akun' | 'notifikasi' | 'keamanan'; label: string }>>([
		{ key: 'akun', label: tr('Akun') },
		{ key: 'notifikasi', label: tr('Preferensi notifikasi') },
		{ key: 'keamanan', label: tr('Keamanan') }
	]);
	let tab = $state<'akun' | 'notifikasi' | 'keamanan'>('akun');

	let user = $state<AuthUser | null>(null);
	let notif = $state({ wa_number: '', wa_consent: false });
	let password = $state({ current: '', next: '' });
	// Pesan gagal ganti password ditampilkan merah di bawah form (bukan banner
	// atas), mis. password saat ini tidak cocok. Dikosongkan saat user mengetik.
	let pwError = $state('');
	// Validasi "password saat ini" terhadap database lewat /v1/auth/verify-password
	// (debounce 700ms setelah berhenti mengetik) — hash tidak bisa dicek di browser.
	let pwCheck = $state<'unknown' | 'checking' | 'ok' | 'mismatch'>('unknown');
	let pwCheckTimer: ReturnType<typeof setTimeout> | null = null;
	// Tampil/sembunyi isi kolom password (icon mata).
	let showCurrent = $state(false);
	let showNew = $state(false);
	// Password baru wajib min. 10 karakter dengan kombinasi huruf besar, huruf
	// kecil, angka, dan karakter khusus; tombol simpan nonaktif sampai lengkap.
	const passwordBaruValid = $derived(
		password.next.length >= 10 &&
			/[A-Z]/.test(password.next) &&
			/[a-z]/.test(password.next) &&
			/[0-9]/.test(password.next) &&
			/[^A-Za-z0-9]/.test(password.next)
	);

	function onCurrentPasswordInput() {
		pwError = '';
		pwCheck = 'unknown';
		if (pwCheckTimer) clearTimeout(pwCheckTimer);
		if (!password.current) return;
		pwCheckTimer = setTimeout(() => void verifyCurrentPassword(), 700);
	}

	async function verifyCurrentPassword() {
		const v = password.current;
		if (!v) return;
		pwCheck = 'checking';
		try {
			const res = await apiRequest('/v1/auth/verify-password', {
				method: 'POST',
				body: JSON.stringify({ current_password: v })
			});
			// Kalau user sudah mengetik lagi, buang hasil lama.
			if (password.current !== v) return;
			if (!res.ok) {
				pwCheck = 'unknown'; // 429 dll: biarkan netral, server akan memvalidasi saat submit
				return;
			}
			const d = await res.json();
			pwCheck = d.ok ? 'ok' : 'mismatch';
		} catch {
			if (password.current === v) pwCheck = 'unknown';
		}
	}
	let twoFactorSecret = $state('');
	let twoFactorUrl = $state('');
	let twoFactorCode = $state('');
	let recoveryCodes = $state<string[]>([]);
	// QR otpauth dirender lokal (uqr) dari URL bawaan server — secret tidak
	// pernah dikirim ke layanan pihak ketiga.
	const twoFactorQr = $derived(twoFactorUrl ? renderSVG(twoFactorUrl, { pixelSize: 4 }) : '');

	let busy = $state(false);
	let msg = $state('');
	let error = $state('');
	// Edit nama tampilan inline (menggantikan window.prompt yang tidak andal
	// di browser ter-embed).
	let editingName = $state(false);
	let nameValue = $state('');
	let nameInput = $state<HTMLInputElement | null>(null);
	$effect(() => {
		if (editingName) nameInput?.focus();
	});

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			user = fresh.user;
			await load();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		const prefs = await apiRequest('/v1/me/notification-preferences');
		if (prefs.ok) {
			const d = await prefs.json();
			notif = { wa_number: d.wa_number ?? '', wa_consent: Boolean(d.wa_consent) };
		}
	}

	async function withBusy(fn: () => Promise<void>) {
		busy = true; msg = ''; error = '';
		try { await fn(); } finally { busy = false; }
	}

	async function saveNotif(e: SubmitEvent) {
		e.preventDefault();
		await withBusy(async () => {
			const res = await apiRequest('/v1/me/notification-preferences', {
				method: 'PUT',
				body: JSON.stringify({ wa_number: notif.wa_number, wa_consent: notif.wa_consent })
			});
			msg = res.ok ? 'Preferensi notifikasi tersimpan.' : await readApiError(res, tr('Gagal menyimpan preferensi.'));
		});
	}

	async function rename() {
		if (!user) return;
		nameValue = user.display_name ?? '';
		editingName = true;
	}

	async function saveName(e: SubmitEvent) {
		e.preventDefault();
		if (!user) return;
		const name = nameValue.trim();
		if (name.length < 2) return;
		if (name === user.display_name) {
			editingName = false;
			return;
		}
		await withBusy(async () => {
			const res = await apiRequest('/v1/auth/me', { method: 'PATCH', body: JSON.stringify({ display_name: name }) });
			if (res.ok) {
				const updated = (await res.json()) as AuthUser;
				user = updated;
				updateStoredUser(updated);
				msg = tr('Nama tampilan diperbarui.');
				editingName = false;
			} else {
				error = await readApiError(res, tr('Gagal memperbarui nama.'));
			}
		});
	}

	async function changePassword(e: SubmitEvent) {
		e.preventDefault();
		pwError = '';
		await withBusy(async () => {
			const res = await apiRequest('/v1/auth/change-password', {
				method: 'POST',
				body: JSON.stringify({ current_password: password.current, new_password: password.next })
			});
			if (res.status === 204) {
				msg = 'Password diubah. Sesi lain dicabut.';
				password = { current: '', next: '' };
				pwCheck = 'unknown';
			} else {
				pwError = await readApiError(res, tr('Gagal mengubah password.'));
			}
		});
	}

	async function startTwoFactor() {
		await withBusy(async () => {
			const res = await apiRequest('/v1/auth/2fa/setup', { method: 'POST' });
			if (res.ok) {
				const d = await res.json();
				twoFactorSecret = d.secret;
				twoFactorUrl = d.otpauth_url ?? '';
				msg = 'Pindai QR atau masukkan secret ke aplikasi autentikator lalu isi kode 6 digit.';
			} else {
				error = await readApiError(res, tr('Gagal memulai setup 2FA.'));
			}
		});
	}

	async function enableTwoFactor(e: SubmitEvent) {
		e.preventDefault();
		await withBusy(async () => {
			const res = await apiRequest('/v1/auth/2fa/enable', { method: 'POST', body: JSON.stringify({ code: twoFactorCode.trim() }) });
			if (res.ok) {
				const d = await res.json();
				recoveryCodes = d.recovery_codes ?? [];
				twoFactorSecret = '';
				twoFactorUrl = '';
				twoFactorCode = '';
				msg = tr('2FA aktif. Simpan kode pemulihan sekarang.');
			} else {
				error = await readApiError(res, 'Kode verifikasi salah.');
			}
		});
	}
</script>

<PageTitle title={tr("Profil & Akun")} />

<section>
	<header>
		<h1 class="page-title">{tr("Profil & Akun")}</h1>
		<p class="page-desc">{tr("Kelola identitas akun, preferensi notifikasi, dan keamanan.")}</p>
	</header>

	{#if msg}
		<p class="pf-alert pf-alert-ok" role="status"><Check size={16} strokeWidth={1.5} />{msg}</p>
	{/if}
	{#if error}
		<p class="pf-alert pf-alert-bad" role="alert"><CircleAlert size={16} strokeWidth={1.5} />{error}</p>
	{/if}

	<!-- Tab: Akun / Preferensi notifikasi / Keamanan -->
	<nav class="pf-tabs" aria-label={tr('Bagian profil')}>
		{#each TABS as t (t.key)}
			<button
				type="button"
				class="pf-tab"
				aria-current={tab === t.key ? 'page' : undefined}
				onclick={() => (tab = t.key)}
			>{t.label}</button>
		{/each}
	</nav>

	{#if tab === 'akun'}
		<!-- Akun -->
		<div class="app-card pf-card">
			<div class="pf-head">
				<span class="pf-tile pf-tile-blue"><UserCircle size={18} strokeWidth={1.5} /></span>
				<h2 class="pf-h2">{tr("Akun")}</h2>
			</div>
			{#if user}
				<div class="pf-identity">
					<span class="pf-avatar" aria-hidden="true">{(user.display_name || user.email || '?').trim().charAt(0).toUpperCase()}</span>
					<div class="min-w-0">
						<p class="pf-name">{user.display_name || '—'}</p>
						<p class="pf-mail">{user.email}</p>
					</div>
					<span class="pf-role">{user.role ?? '—'}</span>
				</div>
				<dl class="pf-dl">
					<div class={editingName ? 'sm:col-span-3' : ''}>
						<dt>{tr("Nama tampilan")}</dt>
						{#if editingName}
							<form class="pf-inline" onsubmit={saveName}>
								<input
									bind:this={nameInput}
									bind:value={nameValue}
									class="app-input !w-64"
									minlength={2}
									maxlength={100}
									required
									aria-label={tr('Nama tampilan baru')}
									onkeydown={(e) => { if (e.key === 'Escape') editingName = false; }}
								/>
								<!-- Simpan nonaktif bila nama belum diubah dari nilai semula
								     atau masih di bawah panjang minimum. -->
								<button class="app-btn app-btn-primary pf-btn-sm" type="submit" disabled={busy || nameValue.trim() === (user?.display_name ?? '') || nameValue.trim().length < 2}>{tr("Simpan")}</button>
								<button class="app-btn app-btn-soft pf-btn-sm" type="button" disabled={busy} onclick={() => (editingName = false)}>{tr("Batal")}</button>
							</form>
						{:else}
							<dd>{user.display_name || '—'}</dd>
						{/if}
					</div>
					<div><dt>{tr("Email")}</dt><dd>{user.email}</dd></div>
					<div><dt>{tr("Peran")}</dt><dd class="pf-upper">{user.role ?? '—'}</dd></div>
				</dl>
				{#if !editingName}
					<button class="app-btn app-btn-soft pf-action" disabled={busy} onclick={rename}>{tr("Ubah nama tampilan")}</button>
				{/if}
			{/if}
		</div>
	{:else if tab === 'notifikasi'}
		<!-- Notifikasi -->
		<form class="app-card pf-card" onsubmit={saveNotif}>
			<div class="pf-head">
				<span class="pf-tile pf-tile-green"><MessageCircle size={18} strokeWidth={1.5} /></span>
				<div>
					<h2 class="pf-h2">Preferensi notifikasi</h2>
					<p class="pf-sub">{tr("Nomor WhatsApp dipakai worker untuk mengirim pemberitahuan status publikasi.")}</p>
				</div>
			</div>
			<div class="pf-grid2">
				<label class="pf-field">Nomor WhatsApp
					<input class="app-input" placeholder="62812xxxxxxx" bind:value={notif.wa_number} />
				</label>
				<label class="pf-check">
					<input type="checkbox" bind:checked={notif.wa_consent} />
					Setuju dihubungi via WhatsApp
				</label>
			</div>
			<!-- Simpan hanya bisa saat kotak persetujuan WhatsApp dicentang. -->
			<button class="app-btn app-btn-primary pf-action" type="submit" disabled={busy || !notif.wa_consent}>{tr("Simpan preferensi")}</button>
		</form>
		<div class="app-card pf-card pf-device">
			<DevicePushCard />
		</div>
	{:else}
		<!-- Keamanan -->
		<div class="app-card pf-card">
			<div class="pf-head">
				<span class="pf-tile pf-tile-blue"><KeyRound size={18} strokeWidth={1.5} /></span>
				<h2 class="pf-h2">Keamanan</h2>
			</div>

			<form class="pf-section" onsubmit={changePassword}>
				<p class="pf-h3">Ganti password</p>
				<div class="pf-grid2">
					<div class="relative">
						<input class="app-input pr-10" type={showCurrent ? 'text' : 'password'} placeholder={tr("Password saat ini")} bind:value={password.current} required oninput={onCurrentPasswordInput} />
						<button class="pf-eye" type="button" aria-label={showCurrent ? tr('Sembunyikan password') : tr('Tampilkan password')} aria-pressed={showCurrent} onclick={() => (showCurrent = !showCurrent)}>
							{#if showCurrent}<EyeOff size={16} strokeWidth={1.5} />{:else}<Eye size={16} strokeWidth={1.5} />{/if}
						</button>
					</div>
					<div class="relative">
						<input class="app-input pr-10" type={showNew ? 'text' : 'password'} placeholder={tr("Password baru (min. 10)")} bind:value={password.next} minlength={10} required oninput={() => (pwError = '')} />
						<button class="pf-eye" type="button" aria-label={showNew ? tr('Sembunyikan password') : tr('Tampilkan password')} aria-pressed={showNew} onclick={() => (showNew = !showNew)}>
							{#if showNew}<EyeOff size={16} strokeWidth={1.5} />{:else}<Eye size={16} strokeWidth={1.5} />{/if}
						</button>
					</div>
				</div>
				{#if pwCheck === 'checking'}
					<p class="pf-hint">{tr("Memeriksa password saat ini…")}</p>
				{:else if pwCheck === 'mismatch'}
					<p class="pf-hint pf-bad animate-in fade-in slide-in-from-top-1 duration-200" role="alert">{tr("Password saat ini tidak cocok dengan yang terdaftar.")}</p>
				{:else if pwCheck === 'ok'}
					<p class="pf-hint pf-good animate-in fade-in slide-in-from-top-1 duration-200">{tr("Password saat ini cocok.")}</p>
				{/if}
				{#if pwError}
					<p class="pf-hint pf-bad" role="alert">{pwError}</p>
				{/if}
				<!-- Validasi langsung saat mengetik: muncul (merah, beranimasi) hanya
				     bila isian belum memenuhi aturan; tidak ada teks statis. -->
				{#if password.next && !passwordBaruValid}
					<p class="pf-hint pf-bad animate-in fade-in slide-in-from-top-1 duration-200" role="status">{tr("Minimal 10 karakter dengan kombinasi huruf besar, huruf kecil, angka, dan karakter khusus.")}</p>
				{/if}
				<button class="app-btn app-btn-primary pf-action" type="submit" disabled={busy || !passwordBaruValid}>{tr("Simpan password")}</button>
			</form>
		</div>

		<div class="app-card pf-card">
			<div class="pf-head">
				<span class="pf-tile pf-tile-green"><ShieldCheck size={18} strokeWidth={1.5} /></span>
				<h2 class="pf-h2">Two-Factor Authentication</h2>
			</div>
			{#if recoveryCodes.length > 0}
				<p class="pf-hint pf-good mt-3">2FA aktif. Kode pemulihan (hanya tampil sekali):</p>
				<code class="pf-code">{recoveryCodes.join('\n')}</code>
			{:else if twoFactorSecret}
				<div class="pf-2fa">
					{#if twoFactorQr}
						<!-- SVG dari uqr hanya punya viewBox; tanpa lebar eksplisit dia
						     runtuh 0px di dalam flex, jadi wrapper memaksa ukurannya. -->
						<div class="pf-qr [&>svg]:block [&>svg]:h-auto [&>svg]:w-full" aria-hidden="true">
							{@html twoFactorQr}
						</div>
					{/if}
					<div class="min-w-0">
						<p class="pf-sub">{tr('Pindai QR dengan aplikasi autentikator (Google Authenticator, Authy, dan sejenisnya), atau salin secret ini secara manual:')}</p>
						<code class="pf-code break-all">{twoFactorSecret}</code>
						<form class="pf-inline" onsubmit={enableTwoFactor}>
							<input class="app-input !w-36" placeholder="123456" bind:value={twoFactorCode} required />
							<button class="app-btn app-btn-primary" type="submit" disabled={busy}>Aktifkan</button>
						</form>
					</div>
				</div>
			{:else}
				<p class="pf-sub mt-2">{tr("Tambahkan lapisan kedua saat login dengan kode TOTP.")}</p>
				<button class="app-btn app-btn-soft pf-action" disabled={busy} onclick={startTwoFactor}>Mulai setup 2FA</button>
			{/if}
		</div>
	{/if}
</section>

<style>
	/* Profil & Akun klien: kartu per topik, ikon dalam kotak berwarna sesuai arti
	   (biru identitas/keamanan, hijau WhatsApp & 2FA). Ritme 4px, bobot 400–500.
	   UI lama: +page.legacy.svelte.txt (salin ke +page.svelte untuk rollback). */
	.pf-alert { display: flex; align-items: center; gap: 8px; margin-top: 16px; padding: 12px 16px; border-radius: 12px; font-size: 14px; line-height: 20px; }
	.pf-alert-ok { background-color: var(--success-soft); color: var(--success); }
	.pf-alert-bad { background-color: color-mix(in srgb, var(--destructive) 10%, var(--card)); color: var(--destructive); }

	.pf-tabs { display: flex; gap: 24px; margin-top: 24px; border-bottom: 1px solid var(--border); overflow-x: auto; scrollbar-width: none; }
	.pf-tabs::-webkit-scrollbar { display: none; }
	.pf-tab {
		position: relative; flex: 0 0 auto; margin-bottom: -1px; padding: 12px 2px; border: 0; background: transparent; white-space: nowrap;
		font-size: 14px; font-weight: 500; color: var(--muted-foreground); transition: color 120ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.pf-tab::after { content: ''; position: absolute; left: 0; right: 0; bottom: 0; height: 2px; border-radius: 2px 2px 0 0; background: var(--primary); transform: scaleX(0); transition: transform 160ms cubic-bezier(0.22, 1, 0.36, 1); }
	.pf-tab:hover { color: var(--foreground); }
	.pf-tab[aria-current='page'] { color: var(--accent-foreground); }
	.pf-tab[aria-current='page']::after { transform: scaleX(1); }
	.pf-tab:focus-visible { outline: 2px solid var(--primary); outline-offset: 2px; border-radius: 4px; }

	.pf-card { margin-top: 16px; padding: 24px; }
	.pf-head { display: flex; align-items: center; gap: 12px; }
	.pf-tile { display: inline-flex; width: 40px; height: 40px; flex-shrink: 0; align-items: center; justify-content: center; border-radius: 12px; }
	.pf-tile-blue { background-color: var(--accent); color: var(--accent-foreground); }
	.pf-tile-green { background-color: var(--success-soft); color: var(--success); }
	.pf-h2 { font-size: 16px; line-height: 24px; font-weight: 500; }
	.pf-h3 { font-size: 14px; line-height: 20px; font-weight: 500; }
	.pf-sub { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }

	.pf-identity { display: flex; align-items: center; gap: 16px; margin-top: 24px; padding: 16px; border-radius: 12px; background-color: color-mix(in srgb, var(--foreground) 2.5%, transparent); }
	.pf-avatar { display: inline-flex; width: 48px; height: 48px; flex-shrink: 0; align-items: center; justify-content: center; border-radius: 9999px; background-color: var(--primary); color: var(--primary-foreground); font-size: 20px; font-weight: 500; }
	.pf-name { font-size: 16px; line-height: 24px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.pf-mail { font-size: 13px; line-height: 20px; color: var(--muted-foreground); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
	.pf-role { margin-left: auto; padding: 2px 10px; border-radius: 9999px; background-color: var(--accent); color: var(--accent-foreground); font-size: 12px; line-height: 16px; font-weight: 500; text-transform: capitalize; }

	.pf-dl { display: grid; grid-template-columns: 1fr; gap: 16px; margin-top: 24px; }
	@media (min-width: 640px) { .pf-dl { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
	.pf-dl dt { font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	.pf-dl dd { margin-top: 4px; font-size: 14px; line-height: 20px; font-weight: 500; overflow-wrap: anywhere; }
	.pf-upper { text-transform: capitalize; }
	.pf-inline { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-top: 8px; }
	.pf-btn-sm { min-height: 32px; padding: 0 12px; font-size: 13px; }
	.pf-action { margin-top: 16px; font-weight: 500; }

	.pf-grid2 { display: grid; grid-template-columns: 1fr; gap: 16px; margin-top: 20px; }
	@media (min-width: 640px) { .pf-grid2 { grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: end; } }
	.pf-field { display: flex; flex-direction: column; gap: 6px; font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.pf-check { display: flex; align-items: center; gap: 8px; min-height: 40px; font-size: 14px; }
	.pf-check input { width: 16px; height: 16px; accent-color: var(--primary); }

	.pf-device :global(> div) { margin-top: 0; padding-top: 0; border-top: 0; }

	.pf-section { margin-top: 20px; }
	.pf-section .pf-grid2 { margin-top: 12px; }
	.pf-eye { position: absolute; top: 50%; right: 8px; display: inline-flex; width: 28px; height: 28px; align-items: center; justify-content: center; transform: translateY(-50%); border-radius: 6px; color: var(--muted-foreground); transition: background-color 120ms cubic-bezier(0.22, 1, 0.36, 1); }
	.pf-eye:hover { background-color: var(--secondary); color: var(--foreground); }
	.pf-eye:focus-visible { outline: 2px solid var(--primary); outline-offset: 1px; }
	.pf-hint { margin-top: 8px; font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	.pf-bad { color: var(--destructive); }
	.pf-good { color: var(--success); }

	.pf-code { display: block; margin-top: 12px; padding: 12px 16px; border-radius: 10px; background-color: var(--muted); font-size: 13px; line-height: 20px; white-space: pre-wrap; }
	.pf-2fa { display: flex; flex-direction: column; gap: 20px; margin-top: 16px; }
	@media (min-width: 640px) { .pf-2fa { flex-direction: row; align-items: flex-start; } }
	.pf-qr { width: 160px; flex-shrink: 0; align-self: flex-start; padding: 8px; border: 1px solid var(--border); border-radius: 12px; background-color: #fff; }
</style>
