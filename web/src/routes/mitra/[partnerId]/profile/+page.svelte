<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Konfigurasi mitra — port penuh PartnerProfileSection app lama ke dalam
	// tab: Profil (identitas + RSS + dokumen), Pengaturan Withdraw (daftar
	// bank/e-wallet + validasi angka), Notifikasi (preferensi), dan Keamanan
	// (ganti password + panel 2FA TOTP). Struktur & label mengikuti
	// partner-profile.tsx; token warna memakai design token app ini.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		Building2, CircleCheck, Eye, EyeOff, Globe, Info, Lock, Mail, Newspaper, Pencil, Rss, Save,
		ShieldCheck, Smartphone, User, Wallet, Bell
	} from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import {
		apiRequest, readApiError, updateStoredUser, ensureValidSession, AUTH_EXPIRED_EVENT
	} from '$lib/auth';
	import type { AuthUser } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
	import DevicePushCard from '$lib/components/device-push-card.svelte';
	import { renderSVG } from 'uqr';

	// Daftar persis dari partner-profile.tsx app lama.
	const BANK_OPTIONS = [
		'BCA (Bank Central Asia)',
		'Bank Mandiri',
		'BRI (Bank Rakyat Indonesia)',
		'BNI (Bank Negara Indonesia)',
		'BSI (Bank Syariah Indonesia)',
		'CIMB Niaga',
		'Permata Bank',
		'Bank Danamon',
		'Bank Jago',
		'Bank Tabungan Negara (BTN)',
		'Bank Lainnya'
	];
	const EWALLET_OPTIONS = ['GoPay', 'OVO', 'DANA', 'ShopeePay', 'LinkAja'];

	type TwoFaState = {
		enabled: boolean;
		busy: boolean;
		error: string;
		notice: string;
		secret: string | null;
		otpauthUrl: string | null;
		step: 'idle' | 'setup';
		code: string;
		recoveryCodes: string[] | null;
	};

	let loading = $state(true);
	let loadError = $state('');
	let user = $state<AuthUser | null>(null);
	let partner = $state<any>(null);
	// GET /v1/partner/me tidak mengembalikan rss_enabled (hanya PATCH yang
	// mengembalikannya), jadi status RSS "tidak diketahui" (null) sampai ada
	// respons PATCH — bukan menebak Nonaktif.
	let rssEnabled = $state<boolean | null>(null);

	// Tab: "profile" | "withdraw" | "notification" | "security" (urutan app lama).
	// Bisa ditentukan lewat URL (?tab=notification) supaya tautan dari dalam app
	// — mis. banner "Isi nomor WhatsApp sekarang" — mendarat di tab yang memuat
	// kolomnya, bukan selalu di tab Profil.
	const TABS = ['profile', 'withdraw', 'notification', 'security'] as const;
	type Tab = (typeof TABS)[number];
	let activeTab = $state<Tab>(TABS.find((t) => t === page.url.searchParams.get('tab')) ?? 'profile');

	// Profil
	let editing = $state(false);
	let displayName = $state('');
	let mitraName = $state('');
	let profileBusy = $state(false);
	let profileNotice = $state('');
	let profileError = $state('');

	// Withdraw
	let payoutType = $state<'bank' | 'ewallet'>('bank');
	let bankName = $state(BANK_OPTIONS[0]);
	let customBankName = $state('');
	let ewalletName = $state(EWALLET_OPTIONS[0]);
	let bankAccountNumber = $state('');
	let bankAccountName = $state('');
	let withdrawBusy = $state(false);
	let withdrawNotice = $state('');
	let withdrawError = $state('');
	let minimumWithdrawIdr = $state<number | null>(null);
	let bankFeeIdr = $state<number | null>(null);
	let ewalletFeePct = $state<number | null>(null);

	// Notifikasi
	let prefs = $state({ email_alt1: '', email_alt2: '', wa_number: '', wa_consent: false });
	// Simpan preferensi notifikasi hanya aktif bila email tambahan 1 & 2, nomor
	// WA, dan persetujuan WA sudah lengkap.
	const notifSiap = $derived(
		prefs.email_alt1.trim() !== '' &&
			prefs.email_alt2.trim() !== '' &&
			prefs.wa_number.trim() !== '' &&
			prefs.wa_consent
	);
	let notificationBusy = $state(false);
	let notificationNotice = $state('');
	let notificationError = $state('');

	// Kartu nomor WhatsApp di tab Profil: bisa disimpan sendiri tanpa menunggu
	// kedua email tambahan. Server memang tidak mewajibkan email — PUT
	// /v1/partner/notification-preferences hanya memvalidasi format email bila
	// diisi (api/internal/domains/partner/misc.go). Kartu ini memakai `prefs`
	// yang sama dengan form di tab Notifikasi, jadi mengetik di salah satu
	// langsung mengisi yang lain.
	let waBusy = $state(false);
	let waNotice = $state('');
	let waError = $state('');
	const waSiap = $derived(prefs.wa_number.trim() !== '' && prefs.wa_consent);

	// Keamanan
	let showPasswordForm = $state(false);
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let showCurrent = $state(false);
	let showNew = $state(false);
	let showConfirm = $state(false);
	let passwordBusy = $state(false);
	let passwordNotice = $state('');
	let passwordError = $state('');
	// Validasi langsung form kata sandi (paritas dengan profil klien): cek kata
	// sandi saat ini ke database via /v1/auth/verify-password (debounce 700ms),
	// aturan kekuatan sandi baru, dan kecocokan kolom ulangi.
	let pwCheck = $state<'unknown' | 'checking' | 'ok' | 'mismatch'>('unknown');
	let pwCheckTimer: ReturnType<typeof setTimeout> | null = null;
	const sandiBaruValid = $derived(
		newPassword.length >= 10 &&
			/[A-Z]/.test(newPassword) &&
			/[a-z]/.test(newPassword) &&
			/[0-9]/.test(newPassword) &&
			/[^A-Za-z0-9]/.test(newPassword)
	);

	function onCurrentPasswordInput() {
		passwordError = '';
		pwCheck = 'unknown';
		if (pwCheckTimer) clearTimeout(pwCheckTimer);
		if (!currentPassword) return;
		pwCheckTimer = setTimeout(() => void verifyCurrentPassword(), 700);
	}

	async function verifyCurrentPassword() {
		const v = currentPassword;
		if (!v) return;
		pwCheck = 'checking';
		try {
			const res = await apiRequest('/v1/auth/verify-password', {
				method: 'POST',
				body: JSON.stringify({ current_password: v })
			});
			if (currentPassword !== v) return;
			if (!res.ok) {
				pwCheck = 'unknown'; // 429 dll: netral, server memvalidasi ulang saat submit
				return;
			}
			const d = await res.json();
			pwCheck = d.ok ? 'ok' : 'mismatch';
		} catch {
			if (currentPassword === v) pwCheck = 'unknown';
		}
	}

	let twoFa = $state<TwoFaState>({
		enabled: false, busy: false, error: '', notice: '', secret: null, otpauthUrl: null, step: 'idle', code: '', recoveryCodes: null
	});
	// QR otpauth dirender lokal (uqr) dari URL bawaan server /v1/auth/2fa/setup.
	const twoFaQr = $derived(twoFa.otpauthUrl ? renderSVG(twoFa.otpauthUrl, { pixelSize: 4 }) : '');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			user = fresh.user;
			await Promise.all([load(), loadPrefs(), loadWithdrawConfig()]);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		loadError = '';
		const res = await apiRequest('/v1/partner/me');
		if (res.ok) {
			const data = await res.json();
			partner = data.partner;
			if (partner) {
				mitraName = partner.name ?? '';
				// payout_type selain ewallet dipetakan ke bank (parity app lama).
				payoutType = partner.payout_type === 'ewallet' ? 'ewallet' : 'bank';
				const storedBank = partner.bank_name ?? '';
				if (BANK_OPTIONS.includes(storedBank)) {
					bankName = storedBank;
				} else if (storedBank) {
					// Rekening tersimpan di luar daftar (bank kecil) — tampilkan sebagai "Bank Lainnya".
					bankName = 'Bank Lainnya';
					customBankName = storedBank;
				}
				if (EWALLET_OPTIONS.includes(storedBank)) ewalletName = storedBank;
				bankAccountNumber = partner.bank_account_number ?? '';
				bankAccountName = partner.bank_account_name ?? user?.display_name ?? '';
			}
		} else {
			loadError = await readApiError(res, tr('Gagal memuat konfigurasi.'));
		}
	}

	async function loadWithdrawConfig() {
		// Saldo minimal & biaya transfer diatur Admin (tc_config) — tampilkan dinamis.
		try {
			const [earnRes, setRes] = await Promise.all([
				apiRequest('/v1/partner/earnings?limit=1'),
				apiRequest('/v1/partner/settlements?limit=1')
			]);
			if (earnRes.ok) {
				const earnings = await earnRes.json();
				if (typeof earnings.minimum_withdraw_idr === 'number') minimumWithdrawIdr = earnings.minimum_withdraw_idr;
			}
			if (setRes.ok) {
				const cfg = await setRes.json();
				if (typeof cfg.minimum_withdraw_idr === 'number') minimumWithdrawIdr = cfg.minimum_withdraw_idr;
				bankFeeIdr = cfg.withdraw_bank_fee_idr ?? null;
				ewalletFeePct = cfg.withdraw_ewallet_fee_percent ?? null;
			}
		} catch {
			// Info biaya bersifat opsional; form tetap terbuka tanpa itu.
		}
	}


	async function loadPrefs() {
		const res = await apiRequest('/v1/partner/notification-preferences');
		if (res.ok) prefs = { ...prefs, ...(await res.json()) };
	}

	// ---- Tab Profil ----

	function startEditing() {
		displayName = user?.display_name ?? '';
		profileNotice = '';
		profileError = '';
		editing = true;
	}

	function cancelEditing() {
		editing = false;
		profileError = '';
	}

	async function saveProfile(e: SubmitEvent) {
		e.preventDefault();
		if (displayName.trim().length < 2) {
			profileError = tr('Nama penanggung jawab minimal 2 karakter.');
			return;
		}
		if (mitraName.trim().length === 0) {
			profileError = tr('Nama mitra wajib diisi');
			return;
		}
		profileBusy = true;
		profileError = '';
		profileNotice = '';
		try {
			// Nama Penanggung Jawab: PATCH /v1/auth/me { display_name } (parity).
			const meRes = await apiRequest('/v1/auth/me', {
				method: 'PATCH',
				body: JSON.stringify({ display_name: displayName.trim() })
			});
			if (!meRes.ok) {
				profileError = await readApiError(meRes, tr('Gagal memperbarui profil.'));
				return;
			}
			const updated = (await meRes.json()) as AuthUser;
			user = updated;
			updateStoredUser(updated);
			// Nama entitas mitra: PATCH /v1/partner/me { name } + syarat integrasi.
			const partnerRes = await apiRequest('/v1/partner/me', {
				method: 'PATCH',
				body: JSON.stringify({
					name: mitraName.trim(),
					accept_integration_terms: !partner?.integration_terms_accepted_at
				})
			});
			if (!partnerRes.ok) {
				profileError = await readApiError(partnerRes, tr('Gagal memperbarui profil.'));
				return;
			}
			const partnerData = await partnerRes.json().catch(() => null);
			if (partnerData?.partner?.rss_enabled != null) rssEnabled = Boolean(partnerData.partner.rss_enabled);
			profileNotice = tr('Informasi profil media partner berhasil disimpan.');
			editing = false;
			await load();
		} finally {
			profileBusy = false;
		}
	}

	async function disableRSS() {
		if (!confirm(tr('Matikan sindikasi konten (RSS)? Pengaktifan hanya bisa dilakukan admin.'))) return;
		const res = await apiRequest('/v1/partner/me', {
			method: 'PATCH',
			body: JSON.stringify({ rss_enabled: false })
		});
		const data = await res.json().catch(() => null);
		if (!res.ok) {
			profileError = tr('Gagal mematikan RSS. Coba lagi nanti.');
			return;
		}
		if (data?.partner?.rss_enabled != null) rssEnabled = Boolean(data.partner.rss_enabled);
		else rssEnabled = false;
		profileNotice = 'Sindikasi RSS dimatikan.';
		await load();
	}



	// ---- Tab Withdraw ----

	function setPayoutType(next: 'bank' | 'ewallet') {
		payoutType = next;
		withdrawError = '';
	}

	function onAccountNumberInput(value: string) {
		// Angka saja — karakter non-digit dibuang langsung saat diketik.
		bankAccountNumber = value.replace(/[^0-9]/g, '');
	}

	async function saveWithdraw(e: SubmitEvent) {
		e.preventDefault();
		if (!bankAccountNumber.trim()) {
			withdrawError = payoutType === 'bank' ? tr('Harap isi nomor rekening bank.') : tr('Harap isi nomor HP e-wallet.');
			return;
		}
		if (!/^[0-9]+$/.test(bankAccountNumber.trim())) {
			withdrawError = tr('Nomor rekening hanya boleh berisi angka.');
			return;
		}
		if (!bankAccountName.trim()) {
			withdrawError = tr('Harap isi nama pemilik rekening / nama terdaftar akun.');
			return;
		}
		if (payoutType === 'bank' && bankName === 'Bank Lainnya' && !customBankName.trim()) {
			withdrawError = tr('Tuliskan nama bank Anda pada kolom nama bank.');
			return;
		}
		const storedBankName = payoutType === 'ewallet' ? ewalletName : bankName === 'Bank Lainnya' ? customBankName.trim() : bankName;
		// Mengubah tujuan payout butuh konfirmasi password (sesi curian tidak bisa mengalihkan dana).
		const changed =
			payoutType !== (partner?.payout_type === 'ewallet' ? 'ewallet' : 'bank') ||
			storedBankName.trim() !== String(partner?.bank_name ?? '').trim() ||
			bankAccountNumber.trim() !== String(partner?.bank_account_number ?? '').trim() ||
			bankAccountName.trim() !== String(partner?.bank_account_name ?? '').trim();
		let currentPassword = '';
		if (changed) {
			const entered = window.prompt(tr('Konfirmasi password untuk mengubah rekening penarikan:'));
			if (!entered) return;
			currentPassword = entered;
		}
		withdrawBusy = true;
		withdrawError = '';
		withdrawNotice = '';
		try {
			const res = await apiRequest('/v1/partner/me', {
				method: 'PATCH',
				body: JSON.stringify({
					payout_type: payoutType,
					bank_name: storedBankName.trim(),
					bank_account_number: bankAccountNumber.trim(),
					bank_account_name: bankAccountName.trim(),
					...(currentPassword ? { current_password: currentPassword } : {})
				})
			});
			if (!res.ok) {
				withdrawError = await readApiError(res, tr('Gagal menyimpan pengaturan withdraw.'));
				return;
			}
			withdrawNotice = tr('Pengaturan rekening penarikan dana (withdraw) berhasil disimpan!');
			await load();
		} finally {
			withdrawBusy = false;
		}
	}

	// ---- Tab Notifikasi ----

	// Endpoint preferensi notifikasi mengganti keempat kolom sekaligus, jadi
	// kedua form (email di tab Notifikasi, nomor di tab Profil) mengirim muatan
	// yang sama dari `prefs` — tidak ada kolom yang tertimpa kosong.
	async function putNotificationPrefs() {
		const res = await apiRequest('/v1/partner/notification-preferences', {
			method: 'PUT',
			body: JSON.stringify({
				email_alt1: prefs.email_alt1.trim(),
				email_alt2: prefs.email_alt2.trim(),
				wa_number: prefs.wa_number.trim(),
				wa_consent: prefs.wa_consent
			})
		});
		if (res.ok) await loadPrefs();
		return res;
	}

	async function savePrefs(e: SubmitEvent) {
		e.preventDefault();
		notificationBusy = true;
		notificationError = '';
		notificationNotice = '';
		try {
			const res = await putNotificationPrefs();
			if (!res.ok) {
				notificationError = await readApiError(res, "Permintaan gagal.");
				return;
			}
			notificationNotice = tr('Pengaturan notifikasi berhasil disimpan!');
		} finally {
			notificationBusy = false;
		}
	}

	async function saveWa(e: SubmitEvent) {
		e.preventDefault();
		waBusy = true;
		waError = '';
		waNotice = '';
		try {
			const res = await putNotificationPrefs();
			if (!res.ok) {
				waError = await readApiError(res, "Permintaan gagal.");
				return;
			}
			waNotice = tr('Nomor WhatsApp berhasil disimpan!');
		} finally {
			waBusy = false;
		}
	}

	// ---- Tab Keamanan ----

	async function savePassword(e: SubmitEvent) {
		e.preventDefault();
		if (newPassword.length < 10) {
			passwordError = tr('Password baru minimal 10 karakter.');
			return;
		}
		if (newPassword !== confirmPassword) {
			passwordError = tr('Konfirmasi password baru tidak cocok.');
			return;
		}
		passwordBusy = true;
		passwordError = '';
		passwordNotice = '';
		try {
			const res = await apiRequest('/v1/auth/change-password', {
				method: 'POST',
				body: JSON.stringify({
					current_password: currentPassword,
					new_password: newPassword
				})
			});
			if (!res.ok) {
				passwordError = await readApiError(res, tr('Gagal mengubah password.'));
				return;
			}
			passwordNotice = tr('Kata sandi berhasil diperbarui.');
			currentPassword = '';
			newPassword = '';
			confirmPassword = '';
			pwCheck = 'unknown';
			showPasswordForm = false;
		} finally {
			passwordBusy = false;
		}
	}

	async function startTwoFaSetup() {
		const password = window.prompt(tr('Konfirmasi password untuk setup 2FA:'));
		if (!password) return;
		twoFa.busy = true;
		twoFa.error = '';
		twoFa.notice = '';
		try {
			const res = await apiRequest('/v1/auth/2fa/setup', { method: 'POST', body: JSON.stringify({ password }) });
			const data = await res.json().catch(() => null);
			if (!res.ok || !data) {
				twoFa.error = data ? await readApiError(res, "Permintaan gagal.") : tr('Permintaan gagal.');
				return;
			}
			twoFa.secret = data.secret;
			twoFa.otpauthUrl = data.otpauth_url;
			twoFa.step = 'setup';
			twoFa.recoveryCodes = null;
		} finally {
			twoFa.busy = false;
		}
	}

	async function enableTwoFa() {
		if (twoFa.code.trim().length < 6) {
			twoFa.error = tr('Masukkan kode 6 digit dari aplikasi autentikator.');
			return;
		}
		twoFa.busy = true;
		twoFa.error = '';
		try {
			const res = await apiRequest('/v1/auth/2fa/enable', {
				method: 'POST',
				body: JSON.stringify({ code: twoFa.code.trim() })
			});
			const data = await res.json().catch(() => null);
			if (!res.ok || !data) {
				twoFa.error = data ? await readApiError(res, 'Kode verifikasi salah. Coba lagi.') : tr('Kode verifikasi salah. Coba lagi.');
				return;
			}
			twoFa.enabled = true;
			twoFa.recoveryCodes = data.recovery_codes ?? null;
			twoFa.code = '';
			twoFa.secret = null;
			twoFa.otpauthUrl = null;
			twoFa.step = 'idle';
			twoFa.notice = tr('2FA aktif. Simpan kode pemulihan sekarang.');
		} finally {
			twoFa.busy = false;
		}
	}

	async function disableTwoFa() {
		const password = prompt(tr('Masukkan password Anda untuk menonaktifkan verifikasi dua langkah:'));
		if (!password) return;
		twoFa.busy = true;
		twoFa.error = '';
		twoFa.notice = '';
		try {
			const res = await apiRequest('/v1/auth/2fa/disable', {
				method: 'POST',
				body: JSON.stringify({ password })
			});
			const data = await res.json().catch(() => null);
			if (!res.ok) {
				twoFa.error = await readApiError(res, 'Gagal menonaktifkan 2FA.');
				// 409 "2FA belum aktif." berarti status server tidak aktif — perbaiki state lokal.
				if (data && typeof data.detail === 'string' && data.detail.includes('belum aktif')) {
					twoFa.enabled = false;
				}
				return;
			}
			twoFa.enabled = false;
			twoFa.step = 'idle';
			twoFa.secret = null;
			twoFa.otpauthUrl = null;
			twoFa.recoveryCodes = null;
			twoFa.notice = tr('Verifikasi dua langkah dinonaktifkan.');
		} finally {
			twoFa.busy = false;
		}
	}

	const initials = $derived(
		String(partner?.name || user?.display_name || 'MT')
			.split(/\s+/)
			.slice(0, 2)
			.map((p: string) => p[0])
			.join('')
			.toUpperCase()
	);

	const dt = (v: string | null) => (v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '-');
	// Warna badge status dokumen memakai token semantik app.

	const tabItems = $derived([
		{ id: 'profile' as const, label: tr('Profil'), Icon: User },
		{ id: 'withdraw' as const, label: tr('Withdraw'), Icon: Wallet },
		{ id: 'notification' as const, label: tr('Notifikasi'), Icon: Bell },
		{ id: 'security' as const, label: tr('Keamanan'), Icon: ShieldCheck }
	]);
</script>

<PageTitle title={tr('Konfigurasi')} />

<section class="cfg">
	<header>
		<h1 class="page-title">{tr("Konfigurasi Akun")}</h1>
		<p class="page-desc">{tr("Kelola informasi identitas media partner, pengaturan rekening withdraw, keamanan, dan dokumen legalitas.")}</p>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat konfigurasi…")}</p>
	{:else if loadError}
		<div class="cfg-note mt-4" style="border-color: var(--destructive); color: var(--destructive);">{loadError}</div>
	{:else}
		<!-- Identitas mitra -->
		<div class="cfg-id">
			<span class="cfg-avatar" aria-hidden="true">{initials}</span>
			<div class="cfg-id-text">
				<h2>{partner?.name || 'Mitra Media Tauthub'}</h2>
				<p class="cfg-meta">
					<span>{tr("Mitra Media")}</span>
					<span class="cfg-meta-sep" aria-hidden="true"></span>
					<span class="cfg-status"><span class="cfg-dot" style="background-color: var(--success);"></span>{partner?.status || 'active'}</span>
					<span class="cfg-meta-sep" aria-hidden="true"></span>
					<span class="tabular-nums">{partner?.media_count ?? 0} {tr("Media Terdaftar")}</span>
				</p>
			</div>
			{#if activeTab === 'profile' && !editing}
				<button class="app-btn app-btn-soft cfg-id-action" onclick={startEditing}><Pencil size={14} /> {tr("Edit Profil")}</button>
			{/if}
		</div>

		<div class="cfg-shell">
			<!-- Navigasi bagian: kolom di desktop, strip yang bisa digeser di ponsel -->
			<div class="cfg-nav" role="tablist" aria-label={tr("Bagian konfigurasi")} aria-orientation="vertical">
				{#each tabItems as item (item.id)}
					<button
						type="button"
						role="tab"
						aria-selected={activeTab === item.id}
						class="focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
						onclick={() => (activeTab = item.id)}
					>
						<item.Icon size={16} />
						{item.label}
					</button>
				{/each}
			</div>

			<div class="cfg-content">
				{#if activeTab === 'profile'}
					{#if profileNotice}<div class="cfg-note" style="border-color: var(--success); background-color: var(--success-soft); color: var(--success);">{profileNotice}</div>{/if}
					{#if profileError}<div class="cfg-note" style="border-color: var(--destructive); color: var(--destructive);">{profileError}</div>{/if}

					<section class="cfg-sec">
						<div class="cfg-sec-head">
							<div>
								<h2>{tr("Informasi Akun & Entitas Media")}</h2>
								<p>{tr("Data kontak dan identitas resmi penerbit media partner di jaringan Tauthub")}</p>
							</div>
						</div>

						{#if !editing}
							<dl class="cfg-panel">
								<div class="cfg-row">
									<dt class="cfg-label">{tr("Nama Entitas Media")}</dt>
									<dd class="cfg-value">{partner?.name || '-'}</dd>
								</div>
								<div class="cfg-row">
									<dt class="cfg-label">{tr("Nama Penanggung Jawab")}</dt>
									<dd class="cfg-value">{user?.display_name || '-'}</dd>
								</div>
								<div class="cfg-row">
									<dt class="cfg-label">{tr("Email Login (Penerbit)")}</dt>
									<dd class="cfg-value">
										{user?.email || '-'}
										<span class="cfg-ok"><CircleCheck size={13} />{tr("Email akun terverifikasi")}</span>
									</dd>
								</div>
								<div class="cfg-row">
									<dt class="cfg-label">{tr("Status Kemitraan")}</dt>
									<dd class="cfg-value"><span class="cfg-status"><span class="cfg-dot" style="background-color: var(--success);"></span>{partner?.status || 'active'}</span></dd>
								</div>
								<div class="cfg-row">
									<dt class="cfg-label">{tr("Media Terdaftar")}</dt>
									<dd class="cfg-value tabular-nums">{partner?.media_count ?? 0}</dd>
								</div>
							</dl>
						{:else}
							<form class="cfg-panel" onsubmit={saveProfile}>
								<label class="cfg-row">
									<span class="cfg-label">{tr("Nama Penanggung Jawab *")}</span>
									<span class="cfg-field"><input class="app-input w-full" required bind:value={displayName} placeholder={tr("Nama penanggung jawab akun")} disabled={profileBusy} /></span>
								</label>
								<label class="cfg-row">
									<span class="cfg-label">{tr("Nama Mitra *")}</span>
									<span class="cfg-field"><input class="app-input w-full" required bind:value={mitraName} disabled={profileBusy} /></span>
								</label>
								<p class="cfg-help">{tr("Nama Penanggung Jawab disimpan ke akun login Anda; nama entitas media disimpan ke profil mitra.")}</p>
								<div class="cfg-actions">
									<button type="button" class="app-btn app-btn-soft" onclick={cancelEditing} disabled={profileBusy}>{tr("Batal")}</button>
									<button type="submit" class="app-btn app-btn-primary min-h-11" disabled={profileBusy}>
										<Save size={15} /> {profileBusy ? tr('Menyimpan…') : tr('Simpan Profil')}
									</button>
								</div>
							</form>
						{/if}
					</section>

					<section class="cfg-sec">
						<div class="cfg-sec-head">
							<div>
								<h2>{tr("Nomor WhatsApp")}</h2>
								<p>{tr("Nomor untuk konfirmasi pesanan klien. Menambah media baru diblokir sampai nomor dan izin dihubungi diisi.")}</p>
							</div>
						</div>
						{#if waNotice}<div class="cfg-note" style="border-color: var(--success); background-color: var(--success-soft); color: var(--success);">{waNotice}</div>{/if}
						{#if waError}<div class="cfg-note" style="border-color: var(--destructive); color: var(--destructive);">{waError}</div>{/if}
						<form class="cfg-panel" onsubmit={saveWa}>
							<label class="cfg-row">
								<span class="cfg-label">{tr("Nomor WhatsApp")}</span>
								<span class="cfg-field"><input class="app-input w-full" placeholder="6281234567890" aria-label={tr("Nomor WhatsApp")} bind:value={prefs.wa_number} disabled={waBusy} /></span>
							</label>
							<label class="cfg-row cfg-row-check">
								<input type="checkbox" bind:checked={prefs.wa_consent} disabled={waBusy} />
								<span class="cfg-help-inline">{tr("Saya setuju nomor ini disimpan dan dihubungi lewat WhatsApp untuk konfirmasi pesanan klien.")}</span>
							</label>
							<div class="cfg-actions">
								<button class="app-btn app-btn-primary" type="submit" disabled={waBusy || !waSiap}>
									{waBusy ? tr('Menyimpan…') : tr('Simpan nomor WhatsApp')}
								</button>
							</div>
						</form>
					</section>

					{#if partner && !partner.integration_terms_accepted_at}
						<div class="cfg-note" style="border-color: var(--warning); background-color: var(--warning-soft);">
							<p class="font-medium" style="color: var(--warning);">{tr("Syarat Integrasi Belum Disetujui")}</p>
							<p class="mt-1 text-muted-foreground">{tr("Dengan menyimpan profil ini, Anda menyatakan menyetujui syarat integrasi publikasi Tauthub.")}</p>
						</div>
					{/if}

					<section class="cfg-sec">
						<div class="cfg-sec-head">
							<div>
								<h2>{tr("Sindikasi Konten (RSS)")}</h2>
								<p>{tr("Pengaktifan hanya dapat dilakukan admin Tauthub. Fitur ini masih dalam pengembangan")}</p>
							</div>
							{#if rssEnabled != null}
								<span class="cfg-status shrink-0"><span class="cfg-dot" style="background-color: {rssEnabled ? 'var(--success)' : 'var(--muted-foreground)'};"></span>{rssEnabled ? tr('Aktif') : tr('Nonaktif')}</span>
							{/if}
						</div>
						{#if rssEnabled}
							<div class="cfg-panel">
								<div class="cfg-row cfg-row-end">
									<button class="app-btn app-btn-soft" style="color: var(--destructive);" onclick={disableRSS}>{tr("Matikan RSS")}</button>
								</div>
							</div>
						{/if}
					</section>
				{/if}

				{#if activeTab === 'withdraw'}
					{#if withdrawNotice}<div class="cfg-note" style="border-color: var(--success); background-color: var(--success-soft); color: var(--success);">{withdrawNotice}</div>{/if}
					{#if withdrawError}<div class="cfg-note" style="border-color: var(--destructive); color: var(--destructive);">{withdrawError}</div>{/if}

					<section class="cfg-sec">
						<div class="cfg-sec-head">
							<div>
								<h2>{tr("Pengaturan Rekening & E-Wallet Penarikan Dana")}</h2>
								<p>{tr("Tentukan rekening atau e-wallet yang akan digunakan untuk menerima pencairan pendapatan dari publikasi.")}</p>
							</div>
						</div>

						<form class="cfg-panel" onsubmit={saveWithdraw}>
							<div class="cfg-row cfg-row-stack">
								<p class="cfg-label">{tr("Metode Penerimaan Dana")}</p>
								<div class="cfg-methods">
									<button type="button" class="cfg-method focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" aria-pressed={payoutType === 'bank'} onclick={() => setPayoutType('bank')}>
										<Wallet size={18} class="shrink-0" />
										<span class="min-w-0 flex-1">
											<span class="block text-sm font-medium leading-tight">{tr("Transfer Rekening Bank")}</span>
											<span class="mt-1 block text-xs leading-snug text-muted-foreground">BCA, Mandiri, BRI, BNI, BSI, dll</span>
										</span>
										{#if payoutType === 'bank'}<CircleCheck size={16} class="shrink-0" style="color: var(--primary);" />{/if}
									</button>
									<button type="button" class="cfg-method focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" aria-pressed={payoutType === 'ewallet'} onclick={() => setPayoutType('ewallet')}>
										<Smartphone size={18} class="shrink-0" />
										<span class="min-w-0 flex-1">
											<span class="block text-sm font-medium leading-tight">{tr("E-Wallet Digital")}</span>
											<span class="mt-1 block text-xs leading-snug text-muted-foreground">GoPay, OVO, DANA, ShopeePay</span>
										</span>
										{#if payoutType === 'ewallet'}<CircleCheck size={16} class="shrink-0" style="color: var(--primary);" />{/if}
									</button>
								</div>
							</div>

							<div class="cfg-callout">
								<Info size={14} class="mt-0.5 shrink-0" />
								<div>
									<p class="font-medium">{tr("Tentang Penarikan Dana")}</p>
									<p class="mt-1 text-muted-foreground">
										{tr("Biaya transfer ditanggung mitra:")}
										<strong>{tr("Bank")} {bankFeeIdr != null ? `Rp ${bankFeeIdr.toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}` : '-'}</strong>
										{tr("atau")}
										<strong>{tr("E-Wallet")} {ewalletFeePct != null ? `${String(ewalletFeePct).replace('.', ',')}%` : '-'}</strong>
										{tr("dari nominal. Penarikan dapat diajukan melalui menu")} <strong>{tr("Penghasilan")}</strong>.
										{tr("Saldo minimal penarikan")}
										<strong>{minimumWithdrawIdr != null ? `Rp ${minimumWithdrawIdr.toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}` : '-'}</strong>.
									</p>
								</div>
							</div>

							{#if payoutType === 'bank'}
								<label class="cfg-row">
									<span class="cfg-label">{tr("Nama Bank *")}</span>
									<span class="cfg-field">
										<Select
											bind:value={bankName}
											ariaLabel={tr("Nama Bank")}
											placeholder={tr("Pilih bank")}
											minWidth={240}
											disabled={withdrawBusy}
											searchable
											searchPlaceholder={tr("Cari bank…")}
											options={BANK_OPTIONS.map((opt) => ({ value: opt, label: opt }))}
										/>
									</span>
								</label>
							{:else}
								<label class="cfg-row">
									<span class="cfg-label">{tr("Penyedia E-Wallet *")}</span>
									<span class="cfg-field">
										<Select
											bind:value={ewalletName}
											ariaLabel={tr("Penyedia E-Wallet")}
											placeholder={tr("Pilih e-wallet")}
											minWidth={240}
											disabled={withdrawBusy}
											searchable
											searchPlaceholder={tr("Cari e-wallet…")}
											options={EWALLET_OPTIONS.map((opt) => ({ value: opt, label: opt }))}
										/>
									</span>
								</label>
							{/if}
							{#if payoutType === 'bank' && bankName === 'Bank Lainnya'}
								<label class="cfg-row">
									<span class="cfg-label">{tr("Nama Bank Lainnya *")}</span>
									<span class="cfg-field"><input class="app-input w-full" required placeholder={tr("Contoh: Bank Neo Commerce")} bind:value={customBankName} disabled={withdrawBusy} /></span>
								</label>
							{/if}
							<label class="cfg-row">
								<span class="cfg-label">
									{payoutType === 'bank' ? tr("Nomor Rekening Bank") : tr("Nomor Akun E-Wallet (Nomor HP)")} *
								</span>
								<span class="cfg-field">
									<input
										class="app-input w-full tabular-nums"
										inputmode="numeric"
										required
										placeholder={payoutType === 'bank' ? tr("Contoh: 1234567890") : tr("Contoh: 081234567890")}
										value={bankAccountNumber}
										oninput={(e) => onAccountNumberInput(e.currentTarget.value)}
										disabled={withdrawBusy}
									/>
								</span>
							</label>
							<label class="cfg-row">
								<span class="cfg-label">
									{payoutType === 'bank' ? tr("Nama Pemilik Rekening") : tr("Nama Terdaftar Akun E-Wallet")} *
								</span>
								<span class="cfg-field">
									<input
										class="app-input w-full"
										required
										placeholder={payoutType === 'bank' ? tr("Nama sesuai buku tabungan") : tr("Nama sesuai verifikasi KTP")}
										bind:value={bankAccountName}
										disabled={withdrawBusy}
									/>
									<span class="cfg-hint">
										{payoutType === 'bank'
											? tr("Gunakan nama yang sama persis dengan buku tabungan bank Anda.")
											: tr("Pastikan nama sesuai akun e-wallet yang sudah terverifikasi KTP.")}
									</span>
								</span>
							</label>

							<div class="cfg-actions">
								<button type="submit" class="app-btn app-btn-primary min-h-11" disabled={withdrawBusy}>
									<Save size={15} /> {withdrawBusy ? tr('Menyimpan…') : tr('Simpan Pengaturan')}
								</button>
							</div>
						</form>
					</section>
				{/if}

				{#if activeTab === 'notification'}
					{#if notificationNotice}<div class="cfg-note" style="border-color: var(--success); background-color: var(--success-soft); color: var(--success);">{notificationNotice}</div>{/if}
					{#if notificationError}<div class="cfg-note" style="border-color: var(--destructive); color: var(--destructive);">{notificationError}</div>{/if}

					<section class="cfg-sec">
						<div class="cfg-sec-head">
							<div>
								<h2>{tr("Notifikasi")}</h2>
								<p>{tr("Email tambahan & nomor WhatsApp penerima notifikasi dashboard Tauthub.")}</p>
							</div>
						</div>

						{#if (partner?.media_count ?? 0) > 1 && ((prefs.wa_number ?? '').trim() === '' || !prefs.wa_consent)}
							<div class="cfg-note" style="border-color: var(--warning); background-color: var(--warning-soft);" role="alert">
								<p class="font-medium" style="color: var(--warning);">{tr('Nomor WhatsApp wajib diisi')}</p>
								<p class="mt-1 text-muted-foreground">{tr('Media Anda sudah lebih dari satu, jadi nomor WhatsApp dan izin dihubungi wajib diisi. Menambah media baru diblokir sampai bagian ini lengkap.')}</p>
							</div>
						{/if}

						<form class="cfg-panel" onsubmit={savePrefs}>
							<label class="cfg-row">
								<span class="cfg-label">{tr("Email Tambahan 1")}</span>
								<span class="cfg-field"><input class="app-input w-full" type="email" placeholder="notif1@domain.com" bind:value={prefs.email_alt1} disabled={notificationBusy} /></span>
							</label>
							<label class="cfg-row">
								<span class="cfg-label">{tr("Email Tambahan 2")}</span>
								<span class="cfg-field"><input class="app-input w-full" type="email" placeholder="notif2@domain.com" bind:value={prefs.email_alt2} disabled={notificationBusy} /></span>
							</label>
							<label class="cfg-row">
								<span class="cfg-label">{tr("Nomor WhatsApp")}</span>
								<span class="cfg-field"><input class="app-input w-full tabular-nums" placeholder="6281234567890" bind:value={prefs.wa_number} disabled={notificationBusy} /></span>
							</label>
							<label class="cfg-row cfg-row-check">
								<input type="checkbox" bind:checked={prefs.wa_consent} disabled={notificationBusy} />
								<span class="cfg-help-inline">{tr("Klik setuju maka anda menyetujui bahwa nomor WA anda akan menjadi penerima notifikasi dari dashboard tauthub")}</span>
							</label>
							<div class="cfg-actions">
								<!-- Tombol aktif hanya bila kedua email tambahan dan nomor WA terisi
								     serta persetujuan WA sudah dicentang. -->
								<button class="app-btn app-btn-primary min-h-11" type="submit" disabled={notificationBusy || !notifSiap}>
									{notificationBusy ? tr('Menyimpan…') : tr('Simpan Pengaturan Notifikasi')}
								</button>
							</div>
						</form>
					</section>

					<section class="cfg-sec">
						<div class="cfg-panel cfg-push">
							<DevicePushCard />
						</div>
					</section>
				{/if}

				{#if activeTab === 'security'}
					{#if passwordNotice}<div class="cfg-note" style="border-color: var(--success); background-color: var(--success-soft); color: var(--success);">{passwordNotice}</div>{/if}
					{#if passwordError}<div class="cfg-note" style="border-color: var(--destructive); color: var(--destructive);">{passwordError}</div>{/if}

					<section class="cfg-sec">
						<div class="cfg-sec-head">
							<div>
								<h2>{tr("Keamanan Akun & Kata Sandi")}</h2>
								<p>{tr("Perbarui kata sandi akun penerbit media Anda secara berkala")}</p>
							</div>
							{#if !showPasswordForm}
								<button class="app-btn app-btn-soft shrink-0" onclick={() => { showPasswordForm = true; passwordNotice = ''; passwordError = ''; }}>
									<Lock size={13} /> {tr("Ubah Password")}
								</button>
							{/if}
						</div>

						{#if showPasswordForm}
							<form class="cfg-panel" onsubmit={savePassword}>
								<label class="cfg-row">
									<span class="cfg-label">{tr("Kata Sandi Saat Ini *")}</span>
									<span class="cfg-field relative block">
										<input class="app-input w-full pr-9" type={showCurrent ? 'text' : 'password'} required bind:value={currentPassword} placeholder={tr("Masukkan kata sandi lama")} disabled={passwordBusy} oninput={onCurrentPasswordInput} />
										<button type="button" class="cfg-eye focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" aria-label={tr("Tampilkan kata sandi")} onclick={() => (showCurrent = !showCurrent)}>
											{#if showCurrent}<EyeOff size={14} />{:else}<Eye size={14} />{/if}
										</button>
									</span>
								</label>
								<label class="cfg-row">
									<span class="cfg-label">{tr("Kata Sandi Baru *")}</span>
									<span class="cfg-field relative block">
										<input class="app-input w-full pr-9" type={showNew ? 'text' : 'password'} required bind:value={newPassword} placeholder={tr("Minimal 10 karakter")} disabled={passwordBusy} />
										<button type="button" class="cfg-eye focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" aria-label={tr("Tampilkan kata sandi")} onclick={() => (showNew = !showNew)}>
											{#if showNew}<EyeOff size={14} />{:else}<Eye size={14} />{/if}
										</button>
									</span>
								</label>
								<label class="cfg-row">
									<span class="cfg-label">{tr("Ulangi Kata Sandi Baru *")}</span>
									<span class="cfg-field relative block">
										<input class="app-input w-full pr-9" type={showConfirm ? 'text' : 'password'} required bind:value={confirmPassword} placeholder={tr("Ketik ulang kata sandi baru")} disabled={passwordBusy} />
										<button type="button" class="cfg-eye focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" aria-label={tr("Tampilkan kata sandi")} onclick={() => (showConfirm = !showConfirm)}>
											{#if showConfirm}<EyeOff size={14} />{:else}<Eye size={14} />{/if}
										</button>
									</span>
								</label>
								<!-- Umpan balik langsung: muncul hanya saat isian belum memenuhi
								     aturan; tanpa teks statis. -->
								{#if pwCheck === 'checking' || pwCheck === 'mismatch' || pwCheck === 'ok' || (newPassword && !sandiBaruValid) || (confirmPassword && newPassword && confirmPassword !== newPassword)}
									<div class="cfg-feedback">
										{#if pwCheck === 'checking'}
											<p class="text-muted-foreground">{tr("Memeriksa password saat ini…")}</p>
										{:else if pwCheck === 'mismatch'}
											<p style="color: var(--destructive);" role="alert">{tr("Password saat ini tidak cocok dengan yang terdaftar.")}</p>
										{:else if pwCheck === 'ok'}
											<p style="color: var(--success);">{tr("Password saat ini cocok.")}</p>
										{/if}
										{#if newPassword && !sandiBaruValid}
											<p style="color: var(--destructive);" role="status">{tr("Minimal 10 karakter dengan kombinasi huruf besar, huruf kecil, angka, dan karakter khusus.")}</p>
										{/if}
										{#if confirmPassword && newPassword && confirmPassword !== newPassword}
											<p style="color: var(--destructive);" role="status">{tr("Ulangi kata sandi belum sama dengan kata sandi baru.")}</p>
										{/if}
									</div>
								{/if}
								<div class="cfg-actions">
									<button type="button" class="app-btn app-btn-soft" onclick={() => (showPasswordForm = false)} disabled={passwordBusy}>{tr("Batal")}</button>
									<button type="submit" class="app-btn app-btn-primary min-h-11" disabled={passwordBusy || !sandiBaruValid || confirmPassword !== newPassword}>
										<ShieldCheck size={15} /> {passwordBusy ? tr('Menyimpan…') : tr('Perbarui Kata Sandi')}
									</button>
								</div>
							</form>
						{/if}
					</section>

					<section class="cfg-sec">
						<div class="cfg-sec-head">
							<div>
								<h2>{tr("Verifikasi dua langkah (2FA)")}</h2>
								<p>
									{twoFa.enabled
										? tr("Aktif, setiap login memerlukan kode dari aplikasi autentikator Anda.")
										: tr("Nonaktif, tambahkan lapisan keamanan ekstra untuk akun Anda.")}
								</p>
							</div>
							<span class="cfg-status shrink-0"><span class="cfg-dot" style="background-color: {twoFa.enabled ? 'var(--success)' : 'var(--muted-foreground)'};"></span>{twoFa.enabled ? tr('Aktif') : tr('Nonaktif')}</span>
						</div>

						{#if twoFa.error}<div class="cfg-note" style="border-color: var(--destructive); color: var(--destructive);">{twoFa.error}</div>{/if}
						{#if twoFa.notice}<div class="cfg-note" style="border-color: var(--success); background-color: var(--success-soft); color: var(--success);">{twoFa.notice}</div>{/if}

						{#if !twoFa.enabled && twoFa.step === 'idle'}
							<div class="cfg-panel">
								<div class="cfg-row cfg-row-split">
									<p class="cfg-help-inline">{tr("Dengan 2FA aktif, setiap login membutuhkan password dan kode 6 digit dari aplikasi autentikator, melindungi akun Anda meski password bocor.")}</p>
									<button class="app-btn app-btn-soft shrink-0" disabled={twoFa.busy} onclick={startTwoFaSetup}>
										{twoFa.busy ? tr('Menyiapkan…') : tr('Aktifkan verifikasi dua langkah')}
									</button>
								</div>
							</div>
						{/if}

						{#if !twoFa.enabled && twoFa.step === 'setup' && twoFa.otpauthUrl}
							<div class="cfg-panel">
								<div class="cfg-row cfg-row-stack">
									<p class="text-sm font-medium">{tr("1. Tambahkan akun di aplikasi autentikator")}</p>
									<p class="cfg-help-inline">{tr("Buka Google Authenticator / Authy, tambah akun, lalu pindai QR berikut atau masukkan kunci manual:")}</p>
									<div class="flex flex-col gap-3 sm:flex-row sm:items-start">
										<!-- SVG dari uqr hanya punya viewBox; tanpa lebar eksplisit dia
										     runtuh 0px di dalam flex, jadi wrapper memaksa ukurannya. -->
										<div class="w-40 shrink-0 rounded-lg border p-2 [&>svg]:block [&>svg]:h-auto [&>svg]:w-full" style="border-color: var(--border); background-color: #fff;" aria-hidden="true">
											{@html twoFaQr}
										</div>
										<div class="min-w-0 flex-1 rounded-lg border p-3" style="border-color: var(--border); background-color: var(--muted);">
											<p class="text-xs text-muted-foreground">{tr("Kunci rahasia (secret)")}</p>
											<p class="mt-1 break-all font-mono text-sm">{twoFa.secret}</p>
											<a class="mt-2 inline-block break-all text-xs" style="color: var(--primary);" href={twoFa.otpauthUrl}>{tr("Buka tautan otpauth di aplikasi autentikator")}</a>
										</div>
									</div>
								</div>
								<div class="cfg-row cfg-row-stack">
									<p class="text-sm font-medium">{tr("2. Verifikasi kode")}</p>
									<p class="cfg-help-inline">{tr("Masukkan kode 6 digit yang muncul di aplikasi untuk mengaktifkan:")}</p>
									<div class="flex flex-wrap items-center gap-2">
										<input
											class="app-input w-36 font-mono tabular-nums"
											value={twoFa.code}
											oninput={(e) => (twoFa.code = e.currentTarget.value.replace(/\D/g, '').slice(0, 6))}
											placeholder="000000"
											inputmode="numeric"
											autocomplete="one-time-code"
										/>
										<button class="app-btn app-btn-primary min-h-11" disabled={twoFa.busy || twoFa.code.length < 6} onclick={enableTwoFa}>
											{twoFa.busy ? tr('Mengaktifkan…') : tr('Aktifkan 2FA')}
										</button>
										<button class="app-btn app-btn-soft min-h-11" disabled={twoFa.busy} onclick={() => { twoFa.step = 'idle'; twoFa.secret = null; twoFa.otpauthUrl = null; twoFa.code = ''; twoFa.error = ''; }}>
											{tr("Batal")}
										</button>
									</div>
								</div>
							</div>
						{/if}

						{#if twoFa.enabled && twoFa.recoveryCodes && twoFa.recoveryCodes.length > 0}
							<div class="cfg-note" style="border-color: var(--warning); background-color: var(--warning-soft);">
								<p class="font-medium" style="color: var(--warning);">{tr("Simpan kode pemulihan Anda")}</p>
								<p class="mt-1 text-muted-foreground">{tr("Kode ini hanya ditampilkan sekali. Simpan di tempat aman, dipakai jika HP Anda hilang.")}</p>
								<div class="mt-3 flex flex-wrap gap-2">
									{#each twoFa.recoveryCodes as code (code)}
										<span class="rounded-md px-2 py-1 font-mono text-xs tabular-nums" style="background-color: var(--muted);">{code}</span>
									{/each}
								</div>
							</div>
						{/if}

						{#if twoFa.enabled}
							<div class="cfg-panel">
								<div class="cfg-row cfg-row-end">
									<button class="app-btn app-btn-soft" style="color: var(--destructive);" disabled={twoFa.busy} onclick={disableTwoFa}>
										{twoFa.busy ? tr('Memproses…') : tr('Nonaktifkan 2FA')}
									</button>
								</div>
							</div>
						{/if}
					</section>
				{/if}
			</div>
		</div>
	{/if}
</section>

<style>
	/* Halaman Konfigurasi: identitas di atas, navigasi bagian di kiri, isi
	   berupa panel baris label/isian. Netral semua, aksen hanya di pilihan aktif. */
	.cfg {
		max-width: 1120px;
	}

	/* Identitas mitra */
	.cfg-id {
		display: flex;
		align-items: center;
		gap: 16px;
		margin-top: 24px;
		padding-bottom: 24px;
		border-bottom: 1px solid var(--border);
	}
	.cfg-avatar {
		display: flex;
		width: 56px;
		height: 56px;
		flex-shrink: 0;
		align-items: center;
		justify-content: center;
		border: 1px solid var(--border);
		border-radius: 16px;
		background-color: var(--muted);
		color: var(--foreground, inherit);
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
		font-size: 16px;
		font-weight: 500;
		letter-spacing: 0.02em;
	}
	.cfg-id-text {
		min-width: 0;
		flex: 1;
	}
	.cfg-id-text h2 {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 20px;
		font-weight: 500;
		line-height: 28px;
	}
	.cfg-meta {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 4px 12px;
		margin-top: 4px;
		font-size: 13px;
		line-height: 20px;
		color: var(--muted-foreground);
	}
	.cfg-meta-sep {
		width: 1px;
		height: 12px;
		background-color: var(--border);
	}
	.cfg-status {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
	}
	.cfg-dot {
		width: 8px;
		height: 8px;
		flex-shrink: 0;
		border-radius: 9999px;
	}
	.cfg-id-action {
		flex-shrink: 0;
	}

	/* Rangka: navigasi + isi */
	.cfg-shell {
		display: grid;
		gap: 24px;
		margin-top: 24px;
	}
	.cfg-nav {
		display: flex;
		gap: 4px;
		overflow-x: auto;
		padding-bottom: 8px;
		border-bottom: 1px solid var(--border);
	}
	.cfg-nav button {
		display: flex;
		flex-shrink: 0;
		align-items: center;
		gap: 12px;
		height: 36px;
		padding: 0 12px;
		border-radius: 8px;
		color: var(--muted-foreground);
		font-size: 14px;
		white-space: nowrap;
		transition:
			background-color 120ms ease,
			color 120ms ease;
	}
	.cfg-nav button:hover {
		background-color: var(--muted);
		color: var(--foreground, inherit);
	}
	.cfg-nav button[aria-selected='true'] {
		background-color: var(--accent);
		color: var(--primary);
		font-weight: 500;
	}
	.cfg-content {
		display: flex;
		min-width: 0;
		flex-direction: column;
		gap: 32px;
	}
	@media (min-width: 1024px) {
		.cfg-shell {
			grid-template-columns: 208px minmax(0, 1fr);
			gap: 40px;
		}
		.cfg-nav {
			/* Di bawah bilah utilitas admin yang sticky (64px) + jarak 24px. */
			position: sticky;
			top: 88px;
			flex-direction: column;
			align-self: start;
			overflow: visible;
			padding-bottom: 0;
			border-bottom: 0;
		}
	}

	/* Bagian dan panel */
	.cfg-sec {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.cfg-sec-head {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px;
	}
	.cfg-sec-head h2 {
		font-size: 16px;
		font-weight: 500;
		line-height: 24px;
	}
	.cfg-sec-head p {
		margin-top: 4px;
		max-width: 60ch;
		font-size: 13px;
		line-height: 20px;
		color: var(--muted-foreground);
	}
	.cfg-panel {
		margin: 0;
		border: 1px solid var(--border);
		border-radius: 12px;
		background-color: var(--card, transparent);
	}
	.cfg-panel > :first-child {
		border-top: 0;
		border-top-left-radius: 11px;
		border-top-right-radius: 11px;
	}
	.cfg-row {
		display: grid;
		gap: 8px;
		padding: 16px 24px;
		border-top: 1px solid var(--border);
	}
	.cfg-row-stack {
		gap: 12px;
	}
	.cfg-row-end {
		display: flex;
		justify-content: flex-end;
	}
	.cfg-row-split {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
	}
	.cfg-row-check {
		display: flex;
		align-items: flex-start;
		gap: 12px;
	}
	.cfg-row-check input {
		margin-top: 2px;
	}
	@media (min-width: 768px) {
		.cfg-row:not(.cfg-row-stack, .cfg-row-end, .cfg-row-split, .cfg-row-check) {
			grid-template-columns: 220px minmax(0, 1fr);
			column-gap: 24px;
			align-items: center;
		}
	}
	.cfg-label {
		font-size: 13px;
		line-height: 20px;
		color: var(--muted-foreground);
	}
	.cfg-value {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 4px 12px;
		min-width: 0;
		overflow-wrap: anywhere;
		font-size: 14px;
		line-height: 20px;
	}
	.cfg-ok {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		font-size: 12px;
		color: var(--success);
	}
	.cfg-field {
		display: block;
		min-width: 0;
	}
	.cfg-hint,
	.cfg-help,
	.cfg-help-inline {
		font-size: 12px;
		line-height: 16px;
		color: var(--muted-foreground);
	}
	.cfg-hint {
		display: block;
		margin-top: 8px;
	}
	.cfg-help {
		padding: 0 24px 16px;
	}
	.cfg-help-inline {
		max-width: 64ch;
	}
	.cfg-actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		padding: 16px 24px;
		border-top: 1px solid var(--border);
		border-radius: 0 0 11px 11px;
		background-color: var(--muted);
	}
	.cfg-feedback {
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 12px 24px;
		border-top: 1px solid var(--border);
		font-size: 12px;
		line-height: 16px;
	}
	.cfg-eye {
		position: absolute;
		top: 0;
		right: 4px;
		bottom: 0;
		display: flex;
		align-items: center;
		padding: 0 4px;
		border-radius: 4px;
		color: var(--muted-foreground);
	}

	/* Kartu notifikasi perangkat membawa garis atas dan jarak sendiri. */
	.cfg-push {
		padding: 16px 24px;
	}
	.cfg-push :global(> div) {
		margin-top: 0;
		padding-top: 0;
		border-top: 0;
	}

	/* Pilihan metode penarikan */
	.cfg-methods {
		display: grid;
		gap: 12px;
	}
	@media (min-width: 640px) {
		.cfg-methods {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
	}
	.cfg-method {
		display: flex;
		min-height: 72px;
		align-items: center;
		gap: 12px;
		padding: 12px 16px;
		border: 1px solid var(--border);
		border-radius: 10px;
		text-align: left;
		transition:
			background-color 120ms ease,
			border-color 120ms ease;
	}
	.cfg-method:hover {
		background-color: var(--muted);
	}
	.cfg-method[aria-pressed='true'] {
		border-color: var(--primary);
		background-color: var(--accent);
	}

	/* Keterangan dan pemberitahuan */
	.cfg-callout {
		display: flex;
		gap: 12px;
		padding: 16px 24px;
		border-top: 1px solid var(--border);
		background-color: var(--muted);
		font-size: 13px;
		line-height: 20px;
	}
	.cfg-note {
		padding: 12px 16px;
		border: 1px solid var(--border);
		border-radius: 10px;
		font-size: 13px;
		line-height: 20px;
	}

	@media (max-width: 767px) {
		.cfg-id {
			flex-wrap: wrap;
			margin-top: 16px;
			padding-bottom: 16px;
		}
		.cfg-id-action {
			width: 100%;
		}
		.cfg-avatar {
			width: 48px;
			height: 48px;
		}
		.cfg-row,
		.cfg-callout,
		.cfg-actions,
		.cfg-feedback {
			padding-left: 16px;
			padding-right: 16px;
		}
		.cfg-help {
			padding: 0 16px 16px;
		}
		.cfg-actions > :global(.app-btn) {
			flex: 1;
		}
	}
</style>
