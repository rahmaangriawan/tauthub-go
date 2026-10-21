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
					bank_account_name: bankAccountName.trim()
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
		twoFa.busy = true;
		twoFa.error = '';
		twoFa.notice = '';
		try {
			const res = await apiRequest('/v1/auth/2fa/setup', { method: 'POST' });
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

<section>
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">{tr("Konfigurasi Akun")}</h1>
		<p class="mt-1 text-sm text-muted-foreground">{tr("Kelola informasi identitas media partner, pengaturan rekening withdraw, keamanan, dan dokumen legalitas.")}</p>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat konfigurasi…")}</p>
	{:else if loadError}
		<div class="app-card mt-4 px-4 py-2.5 text-sm" style="border-color: var(--destructive); color: var(--destructive);">{loadError}</div>
	{:else}
		<!-- Hero identitas mitra -->
		<div class="app-card cfg-hero mt-4 flex flex-wrap items-center justify-between gap-3 p-5">
			<div class="flex min-w-0 items-center gap-3.5">
				<span class="cfg-avatar flex h-12 w-12 shrink-0 items-center justify-center rounded-full border text-sm font-bold" style="background-color: var(--success-soft); color: var(--success); border-color: var(--success);">{initials}</span>
				<div class="min-w-0">
					<h2 class="truncate text-lg font-semibold">{partner?.name || 'Mitra Media Tauthub'}</h2>
					<span class="mt-1 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--accent); color: var(--accent-foreground);">
						<Newspaper size={11} />
						{tr("Mitra Media")}
					</span>
				</div>
			</div>
			{#if activeTab === 'profile' && !editing}
				<button class="app-btn app-btn-soft shrink-0" onclick={startEditing}><Pencil size={14} /> {tr("Edit Profil")}</button>
			{/if}
		</div>

		<!-- Navigasi tab: underline, satu sistem untuk keempat tab -->
		<div class="cfg-tabs mt-4 flex gap-1 overflow-x-auto border-b" role="tablist" aria-label={tr("Bagian konfigurasi")}>
			{#each tabItems as item (item.id)}
				<button
					type="button"
					role="tab"
					aria-selected={activeTab === item.id}
					class="-mb-px inline-flex shrink-0 items-center gap-2 border-b-2 px-3.5 py-2.5 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
					style="border-color: {activeTab === item.id ? 'var(--primary)' : 'transparent'}; color: {activeTab === item.id ? 'var(--primary)' : 'var(--muted-foreground)'};"
					onclick={() => (activeTab = item.id)}
				>
					<item.Icon size={14} />
					{item.label}
				</button>
			{/each}
		</div>

		{#if activeTab === 'profile'}
			{#if profileNotice}<div class="app-card mt-4 px-4 py-2.5 text-sm" style="border-color: var(--success); background-color: var(--success-soft); color: var(--success);">{profileNotice}</div>{/if}
			{#if profileError}<div class="app-card mt-4 px-4 py-2.5 text-sm" style="border-color: var(--destructive); color: var(--destructive);">{profileError}</div>{/if}

			<div class="app-card cfg-info mt-4 p-5">
					<div class="flex items-start gap-2.5">
						<Building2 size={16} class="mt-0.5 shrink-0 text-muted-foreground" />
						<div class="min-w-0">
							<h2 class="text-sm font-semibold leading-tight">{tr("Informasi Akun & Entitas Media")}</h2>
							<p class="mt-1 text-xs leading-snug text-muted-foreground">{tr("Data kontak dan identitas resmi penerbit media partner di jaringan Tauthub")}</p>
						</div>
					</div>
					<div class="cfg-divider mt-4 border-t" style="border-color: var(--border);"></div>

					{#if !editing}
						<div class="cfg-grid mt-4 grid gap-x-8 gap-y-5 sm:grid-cols-2">
							<dl class="space-y-5">
								<div>
									<dt class="flex items-center gap-1.5 text-xs text-muted-foreground"><Newspaper size={13} class="shrink-0" />{tr("Nama Entitas Media")}</dt>
									<dd class="mt-1.5 break-words text-sm font-semibold">{partner?.name || '-'}</dd>
								</div>
								<div>
									<dt class="flex items-center gap-1.5 text-xs text-muted-foreground"><Mail size={13} class="shrink-0" />{tr("Email Login (Penerbit)")}</dt>
									<dd class="mt-1.5 break-words text-sm font-semibold">{user?.email || '-'}</dd>
									<dd class="mt-1 text-[11px] font-medium" style="color: var(--success);">{tr("Email akun terverifikasi")}</dd>
								</div>
								<div>
									<dt class="flex items-center gap-1.5 text-xs text-muted-foreground"><Globe size={13} class="shrink-0" />{tr("Media Terdaftar")}</dt>
									<dd class="mt-1.5 text-sm font-semibold tabular-nums">{partner?.media_count ?? 0}</dd>
								</div>
							</dl>
							<dl class="space-y-5">
								<div>
									<dt class="flex items-center gap-1.5 text-xs text-muted-foreground"><User size={13} class="shrink-0" />{tr("Nama Penanggung Jawab")}</dt>
									<dd class="mt-1.5 break-words text-sm font-semibold">{user?.display_name || '-'}</dd>
								</div>
								<div>
									<dt class="flex items-center gap-1.5 text-xs text-muted-foreground"><ShieldCheck size={13} class="shrink-0" />{tr("Status Kemitraan")}</dt>
									<dd class="mt-1.5">
										<span class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium" style="background-color: var(--success-soft); color: var(--success);">
											<span class="h-1.5 w-1.5 rounded-full" style="background-color: var(--success);"></span>
											{partner?.status || 'active'}
										</span>
									</dd>
								</div>
							</dl>
						</div>
				{:else}
					<form class="mt-4 space-y-4" onsubmit={saveProfile}>
						<div class="grid gap-4 sm:grid-cols-2">
							<label class="block">
								<span class="text-xs font-medium text-muted-foreground">{tr("Nama Penanggung Jawab *")}</span>
								<input class="app-input mt-1 w-full" required bind:value={displayName} placeholder={tr("Nama penanggung jawab akun")} disabled={profileBusy} />
							</label>
							<label class="block">
								<span class="text-xs font-medium text-muted-foreground">{tr("Nama Mitra *")}</span>
								<input class="app-input mt-1 w-full" required bind:value={mitraName} disabled={profileBusy} />
							</label>
						</div>
						<p class="text-xs text-muted-foreground">{tr("Nama Penanggung Jawab disimpan ke akun login Anda; nama entitas media disimpan ke profil mitra.")}</p>
						<div class="flex justify-end gap-2 border-t pt-4">
							<button type="button" class="app-btn app-btn-soft" onclick={cancelEditing} disabled={profileBusy}>{tr("Batal")}</button>
							<button type="submit" class="app-btn app-btn-primary min-h-11" disabled={profileBusy}>
								<Save size={15} /> {profileBusy ? tr('Menyimpan…') : tr('Simpan Profil')}
							</button>
						</div>
					</form>
				{/if}
			</div>

			<div class="app-card mt-3 p-4">
				<div class="flex items-start gap-2.5">
					<Smartphone size={15} class="mt-0.5 shrink-0 text-muted-foreground" />
					<div class="min-w-0">
						<h2 class="text-sm font-semibold leading-tight">{tr("Nomor WhatsApp")}</h2>
						<p class="mt-1 text-xs leading-snug text-muted-foreground">{tr("Nomor untuk konfirmasi pesanan klien. Menambah media baru diblokir sampai nomor dan izin dihubungi diisi.")}</p>
					</div>
				</div>
				{#if waNotice}<div class="mt-3 rounded-lg border px-3 py-2 text-xs" style="border-color: var(--success); background-color: var(--success-soft); color: var(--success);">{waNotice}</div>{/if}
				{#if waError}<div class="mt-3 rounded-lg border px-3 py-2 text-xs" style="border-color: var(--destructive); color: var(--destructive);">{waError}</div>{/if}
				<form class="mt-3 space-y-3" onsubmit={saveWa}>
					<input class="app-input w-full" placeholder="6281234567890" aria-label={tr("Nomor WhatsApp")} bind:value={prefs.wa_number} disabled={waBusy} />
					<label class="flex items-start gap-2.5 text-sm">
						<input class="mt-0.5" type="checkbox" bind:checked={prefs.wa_consent} disabled={waBusy} />
						<span class="text-xs leading-relaxed text-muted-foreground">{tr("Saya setuju nomor ini disimpan dan dihubungi lewat WhatsApp untuk konfirmasi pesanan klien.")}</span>
					</label>
					<div class="flex justify-end border-t pt-3">
						<button class="app-btn app-btn-primary" type="submit" disabled={waBusy || !waSiap}>
							{waBusy ? tr('Menyimpan…') : tr('Simpan nomor WhatsApp')}
						</button>
					</div>
				</form>
			</div>

			{#if partner && !partner.integration_terms_accepted_at}
				<div class="app-card mt-3 p-4" style="border-color: var(--warning); background-color: var(--warning-soft);">
					<h2 class="text-sm font-semibold" style="color: var(--warning);">{tr("Syarat Integrasi Belum Disetujui")}</h2>
					<p class="mt-1 text-xs text-muted-foreground">{tr("Dengan menyimpan profil ini, Anda menyatakan menyetujui syarat integrasi publikasi Tauthub.")}</p>
				</div>
			{/if}

			<div class="app-card cfg-rss mt-3 p-4">
				<div class="flex items-center justify-between gap-3">
					<div class="flex items-center gap-2">
						<Rss size={15} class="text-muted-foreground" />
						<h2 class="text-sm font-semibold">{tr("Sindikasi Konten (RSS)")}</h2>
					</div>
					{#if rssEnabled != null}
						<span class="shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: {rssEnabled ? 'var(--success-soft)' : 'var(--secondary)'}; color: {rssEnabled ? 'var(--success)' : 'var(--muted-foreground)'};">
							{rssEnabled ? tr('Aktif') : tr('Nonaktif')}
						</span>
					{/if}
				</div>
				{#if rssEnabled == null}
					<p class="mt-2 text-xs text-muted-foreground">{tr("Pengaktifan hanya dapat dilakukan admin Tauthub. Fitur ini masih dalam pengembangan")}</p>
				{:else}
					<p class="mt-2 text-xs text-muted-foreground">{tr("Pengaktifan hanya dapat dilakukan admin Tauthub. Fitur ini masih dalam pengembangan")}</p>
					{#if rssEnabled}
						<button class="app-btn app-btn-soft mt-3" style="color: var(--destructive);" onclick={disableRSS}>{tr("Matikan RSS")}</button>
					{/if}
				{/if}
			</div>

		{/if}

		{#if activeTab === 'withdraw'}
			{#if withdrawNotice}<div class="app-card mt-4 px-4 py-2.5 text-sm" style="border-color: var(--success); background-color: var(--success-soft); color: var(--success);">{withdrawNotice}</div>{/if}
			{#if withdrawError}<div class="app-card mt-4 px-4 py-2.5 text-sm" style="border-color: var(--destructive); color: var(--destructive);">{withdrawError}</div>{/if}

			<div class="app-card mt-4 p-5">
				<div class="flex items-start gap-2.5">
					<Wallet size={16} class="mt-0.5 shrink-0 text-muted-foreground" />
					<div class="min-w-0">
						<h2 class="text-sm font-semibold leading-tight">{tr("Pengaturan Rekening & E-Wallet Penarikan Dana")}</h2>
						<p class="mt-1 text-xs leading-snug text-muted-foreground">{tr("Tentukan rekening atau e-wallet yang akan digunakan untuk menerima pencairan pendapatan dari publikasi.")}</p>
					</div>
				</div>

				<form class="mt-4 space-y-4" onsubmit={saveWithdraw}>
					<div>
						<p class="text-sm font-semibold leading-tight">{tr("Metode Penerimaan Dana")}</p>
						<div class="mt-2 grid gap-2 sm:grid-cols-2">
							<button type="button" class="flex min-h-[68px] w-full items-center justify-between gap-3 rounded-lg border p-3 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" style="border-color: {payoutType === 'bank' ? 'var(--primary)' : 'var(--border)'}; background-color: {payoutType === 'bank' ? 'var(--accent)' : 'transparent'};" onclick={() => setPayoutType('bank')}>
								<span class="flex min-w-0 items-center gap-2.5">
									<Wallet size={16} class="shrink-0" />
									<span class="min-w-0">
										<span class="block text-sm font-semibold leading-tight">{tr("Transfer Rekening Bank")}</span>
										<span class="mt-0.5 block text-[11px] leading-snug text-muted-foreground">BCA, Mandiri, BRI, BNI, BSI, dll</span>
									</span>
								</span>
								{#if payoutType === 'bank'}<CircleCheck size={16} class="shrink-0" style="color: var(--primary);" />{/if}
							</button>
							<button type="button" class="flex min-h-[68px] w-full items-center justify-between gap-3 rounded-lg border p-3 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" style="border-color: {payoutType === 'ewallet' ? 'var(--primary)' : 'var(--border)'}; background-color: {payoutType === 'ewallet' ? 'var(--accent)' : 'transparent'};" onclick={() => setPayoutType('ewallet')}>
								<span class="flex min-w-0 items-center gap-2.5">
									<Smartphone size={16} class="shrink-0" />
									<span class="min-w-0">
										<span class="block text-sm font-semibold leading-tight">{tr("E-Wallet Digital")}</span>
										<span class="mt-0.5 block text-[11px] leading-snug text-muted-foreground">GoPay, OVO, DANA, ShopeePay</span>
									</span>
								</span>
								{#if payoutType === 'ewallet'}<CircleCheck size={16} class="shrink-0" style="color: var(--primary);" />{/if}
							</button>
						</div>
					</div>

					<div class="rounded-lg border p-3" style="border-color: var(--border); background-color: var(--muted);">
						<p class="flex items-center gap-1.5 text-xs font-semibold leading-tight"><Info size={13} class="shrink-0" />{tr("Tentang Penarikan Dana")}</p>
						<p class="mt-1 text-xs leading-snug text-muted-foreground">
							{tr("Biaya transfer ditanggung mitra:")}
							<strong>{tr("Bank")} {bankFeeIdr != null ? `Rp ${bankFeeIdr.toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}` : '-'}</strong>
							{tr("atau")}
							<strong>{tr("E-Wallet")} {ewalletFeePct != null ? `${String(ewalletFeePct).replace('.', ',')}%` : '-'}</strong>
							{tr("dari nominal. Penarikan dapat diajukan melalui menu")} <strong>{tr("Penghasilan")}</strong>.
							{tr("Saldo minimal penarikan")}
							<strong>{minimumWithdrawIdr != null ? `Rp ${minimumWithdrawIdr.toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}` : '-'}</strong>.
						</p>
					</div>

					<div>
						<p class="text-sm font-semibold leading-tight">{tr("Detail Rekening Penerima")}</p>
						<div class="mt-2 grid gap-x-4 gap-y-3 sm:grid-cols-2">
							{#if payoutType === 'bank'}
								<label class="block">
									<span class="text-xs font-medium text-muted-foreground">{tr("Nama Bank *")}</span>
									<span class="mt-1 block">
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
								<label class="block">
									<span class="text-xs font-medium text-muted-foreground">{tr("Penyedia E-Wallet *")}</span>
									<span class="mt-1 block">
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
							<label class="block">
								<span class="text-xs font-medium text-muted-foreground">
									{payoutType === 'bank' ? tr("Nomor Rekening Bank") : tr("Nomor Akun E-Wallet (Nomor HP)")} *
								</span>
								<input
									class="app-input mt-1 w-full"
									inputmode="numeric"
									required
									placeholder={payoutType === 'bank' ? tr("Contoh: 1234567890") : tr("Contoh: 081234567890")}
									value={bankAccountNumber}
									oninput={(e) => onAccountNumberInput(e.currentTarget.value)}
									disabled={withdrawBusy}
								/>
							</label>
							{#if payoutType === 'bank' && bankName === 'Bank Lainnya'}
								<label class="block sm:col-span-2">
									<span class="text-xs font-medium text-muted-foreground">{tr("Nama Bank Lainnya *")}</span>
									<input class="app-input mt-1 w-full" required placeholder={tr("Contoh: Bank Neo Commerce")} bind:value={customBankName} disabled={withdrawBusy} />
								</label>
							{/if}
							<label class="block sm:col-span-2">
								<span class="text-xs font-medium text-muted-foreground">
									{payoutType === 'bank' ? tr("Nama Pemilik Rekening") : tr("Nama Terdaftar Akun E-Wallet")} *
								</span>
								<input
									class="app-input mt-1 w-full"
									required
									placeholder={payoutType === 'bank' ? tr("Nama sesuai buku tabungan") : tr("Nama sesuai verifikasi KTP")}
									bind:value={bankAccountName}
									disabled={withdrawBusy}
								/>
								<p class="mt-1 text-[11px] leading-snug text-muted-foreground">
									{payoutType === 'bank'
										? tr("Gunakan nama yang sama persis dengan buku tabungan bank Anda.")
										: tr("Pastikan nama sesuai akun e-wallet yang sudah terverifikasi KTP.")}
								</p>
							</label>
						</div>
					</div>

					<div class="flex justify-end pt-2">
						<button type="submit" class="app-btn app-btn-primary min-h-11" disabled={withdrawBusy}>
							<Save size={15} /> {withdrawBusy ? tr('Menyimpan…') : tr('Simpan Pengaturan')}
						</button>
					</div>
				</form>
			</div>
		{/if}

		{#if activeTab === 'notification'}
			{#if notificationNotice}<div class="app-card mt-4 px-4 py-2.5 text-sm" style="border-color: var(--success); background-color: var(--success-soft); color: var(--success);">{notificationNotice}</div>{/if}
			{#if notificationError}<div class="app-card mt-4 px-4 py-2.5 text-sm" style="border-color: var(--destructive); color: var(--destructive);">{notificationError}</div>{/if}

			<div class="app-card mt-4 p-5">
				<div class="flex items-center gap-2">
					<Bell size={16} class="text-muted-foreground" />
					<h2 class="text-sm font-semibold">{tr("Notifikasi")}</h2>
				</div>
				<p class="mt-0.5 text-xs text-muted-foreground">{tr("Email tambahan & nomor WhatsApp penerima notifikasi dashboard Tauthub.")}</p>
				{#if (partner?.media_count ?? 0) > 1 && ((prefs.wa_number ?? '').trim() === '' || !prefs.wa_consent)}
					<div class="mt-5 rounded-xl border p-4" style="border-color: var(--destructive); background-color: var(--warning-soft, #fff7ed);" role="alert">
						<p class="font-semibold" style="color: var(--destructive);">{tr('Nomor WhatsApp wajib diisi')}</p>
						<p class="mt-1 text-sm text-muted-foreground">{tr('Media Anda sudah lebih dari satu, jadi nomor WhatsApp dan izin dihubungi wajib diisi. Menambah media baru diblokir sampai bagian ini lengkap.')}</p>
					</div>
				{/if}
				<form class="mt-5 space-y-4" onsubmit={savePrefs}>
					<div class="grid gap-4 sm:grid-cols-2">
						<label class="block">
							<span class="text-xs font-medium text-muted-foreground">{tr("Email Tambahan 1")}</span>
							<input class="app-input mt-1 w-full" type="email" placeholder="notif1@domain.com" bind:value={prefs.email_alt1} disabled={notificationBusy} />
						</label>
						<label class="block">
							<span class="text-xs font-medium text-muted-foreground">{tr("Email Tambahan 2")}</span>
							<input class="app-input mt-1 w-full" type="email" placeholder="notif2@domain.com" bind:value={prefs.email_alt2} disabled={notificationBusy} />
						</label>
						<label class="block">
							<span class="text-xs font-medium text-muted-foreground">{tr("Nomor WhatsApp")}</span>
							<input class="app-input mt-1 w-full" placeholder="6281234567890" bind:value={prefs.wa_number} disabled={notificationBusy} />
						</label>
					</div>
					<label class="flex items-start gap-2.5 text-sm">
						<input class="mt-0.5" type="checkbox" bind:checked={prefs.wa_consent} disabled={notificationBusy} />
						<span class="text-xs leading-relaxed text-muted-foreground">{tr("Klik setuju maka anda menyetujui bahwa nomor WA anda akan menjadi penerima notifikasi dari dashboard tauthub")}</span>
					</label>
					<div class="flex justify-end border-t pt-4">
						<!-- Tombol aktif hanya bila kedua email tambahan dan nomor WA terisi
						     serta persetujuan WA sudah dicentang. -->
						<button class="app-btn app-btn-primary min-h-11" type="submit" disabled={notificationBusy || !notifSiap}>
							{notificationBusy ? tr('Menyimpan…') : tr('Simpan Pengaturan Notifikasi')}
						</button>
					</div>
				</form>
				<DevicePushCard />
			</div>
		{/if}

		{#if activeTab === 'security'}
			{#if passwordNotice}<div class="app-card mt-4 px-4 py-2.5 text-sm" style="border-color: var(--success); background-color: var(--success-soft); color: var(--success);">{passwordNotice}</div>{/if}
			{#if passwordError}<div class="app-card mt-4 px-4 py-2.5 text-sm" style="border-color: var(--destructive); color: var(--destructive);">{passwordError}</div>{/if}

			<div class="app-card mt-4 p-5">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<div class="flex items-center gap-2">
						<Lock size={15} class="text-muted-foreground" />
						<h2 class="text-sm font-semibold">{tr("Keamanan Akun & Kata Sandi")}</h2>
					</div>
					{#if !showPasswordForm}
						<button class="app-btn app-btn-soft" onclick={() => { showPasswordForm = true; passwordNotice = ''; passwordError = ''; }}>
							<Lock size={13} /> {tr("Ubah Password")}
						</button>
					{/if}
				</div>
				<p class="mt-0.5 text-xs text-muted-foreground">{tr("Perbarui kata sandi akun penerbit media Anda secara berkala")}</p>

				{#if showPasswordForm}
					<form class="mt-5 space-y-4" onsubmit={savePassword}>
						<div class="grid gap-4 sm:grid-cols-3">
							<label class="block">
								<span class="text-xs font-medium text-muted-foreground">{tr("Kata Sandi Saat Ini *")}</span>
								<span class="relative mt-1 block">
									<input class="app-input w-full pr-9" type={showCurrent ? 'text' : 'password'} required bind:value={currentPassword} placeholder={tr("Masukkan kata sandi lama")} disabled={passwordBusy} oninput={onCurrentPasswordInput} />
									<button type="button" class="absolute inset-y-0 right-1 flex items-center rounded px-1 text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" aria-label={tr("Tampilkan kata sandi")} onclick={() => (showCurrent = !showCurrent)}>
										{#if showCurrent}<EyeOff size={14} />{:else}<Eye size={14} />{/if}
									</button>
								</span>
							</label>
							<label class="block">
								<span class="text-xs font-medium text-muted-foreground">{tr("Kata Sandi Baru *")}</span>
								<span class="relative mt-1 block">
									<input class="app-input w-full pr-9" type={showNew ? 'text' : 'password'} required bind:value={newPassword} placeholder={tr("Minimal 10 karakter")} disabled={passwordBusy} />
									<button type="button" class="absolute inset-y-0 right-1 flex items-center rounded px-1 text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" aria-label={tr("Tampilkan kata sandi")} onclick={() => (showNew = !showNew)}>
										{#if showNew}<EyeOff size={14} />{:else}<Eye size={14} />{/if}
									</button>
								</span>
							</label>
							<label class="block">
								<span class="text-xs font-medium text-muted-foreground">{tr("Ulangi Kata Sandi Baru *")}</span>
								<span class="relative mt-1 block">
									<input class="app-input w-full pr-9" type={showConfirm ? 'text' : 'password'} required bind:value={confirmPassword} placeholder={tr("Ketik ulang kata sandi baru")} disabled={passwordBusy} />
									<button type="button" class="absolute inset-y-0 right-1 flex items-center rounded px-1 text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" aria-label={tr("Tampilkan kata sandi")} onclick={() => (showConfirm = !showConfirm)}>
										{#if showConfirm}<EyeOff size={14} />{:else}<Eye size={14} />{/if}
									</button>
								</span>
							</label>
						</div>
						<!-- Umpan balik langsung: muncul hanya saat isian belum memenuhi
						     aturan; tanpa teks statis. -->
						{#if pwCheck === 'checking'}
							<p class="text-xs text-muted-foreground">{tr("Memeriksa password saat ini…")}</p>
						{:else if pwCheck === 'mismatch'}
							<p class="animate-in fade-in slide-in-from-top-1 text-xs duration-200" style="color: var(--destructive);" role="alert">{tr("Password saat ini tidak cocok dengan yang terdaftar.")}</p>
						{:else if pwCheck === 'ok'}
							<p class="animate-in fade-in slide-in-from-top-1 text-xs duration-200" style="color: var(--success);">{tr("Password saat ini cocok.")}</p>
						{/if}
						{#if newPassword && !sandiBaruValid}
							<p class="animate-in fade-in slide-in-from-top-1 text-xs duration-200" style="color: var(--destructive);" role="status">{tr("Minimal 10 karakter dengan kombinasi huruf besar, huruf kecil, angka, dan karakter khusus.")}</p>
						{/if}
						{#if confirmPassword && newPassword && confirmPassword !== newPassword}
							<p class="animate-in fade-in slide-in-from-top-1 text-xs duration-200" style="color: var(--destructive);" role="status">{tr("Ulangi kata sandi belum sama dengan kata sandi baru.")}</p>
						{/if}
						<div class="flex justify-end gap-2 border-t pt-4">
							<button type="button" class="app-btn app-btn-soft" onclick={() => (showPasswordForm = false)} disabled={passwordBusy}>{tr("Batal")}</button>
							<button type="submit" class="app-btn app-btn-primary min-h-11" disabled={passwordBusy || !sandiBaruValid || confirmPassword !== newPassword}>
								<ShieldCheck size={15} /> {passwordBusy ? tr('Menyimpan…') : tr('Perbarui Kata Sandi')}
							</button>
						</div>
					</form>
				{/if}
			</div>

			<div class="app-card mt-3 p-5">
				<div class="flex flex-wrap items-center justify-between gap-3">
					<div class="flex items-center gap-2.5">
						<ShieldCheck size={17} class="shrink-0" style="color: {twoFa.enabled ? 'var(--success)' : 'var(--muted-foreground)'};" />
						<div>
							<h2 class="text-sm font-semibold">{tr("Verifikasi dua langkah (2FA)")}</h2>
							<p class="mt-0.5 text-xs text-muted-foreground">
								{twoFa.enabled
									? tr("Aktif, setiap login memerlukan kode dari aplikasi autentikator Anda.")
									: tr("Nonaktif, tambahkan lapisan keamanan ekstra untuk akun Anda.")}
							</p>
						</div>
					</div>
					<span class="shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: {twoFa.enabled ? 'var(--success-soft)' : 'var(--secondary)'}; color: {twoFa.enabled ? 'var(--success)' : 'var(--muted-foreground)'};">
						{twoFa.enabled ? tr('Aktif') : tr('Nonaktif')}
					</span>
				</div>

				{#if twoFa.error}<p class="mt-3 text-sm" style="color: var(--destructive);">{twoFa.error}</p>{/if}
				{#if twoFa.notice}<p class="mt-3 text-sm" style="color: var(--success);">{twoFa.notice}</p>{/if}

				{#if !twoFa.enabled && twoFa.step === 'idle'}
					<button class="app-btn app-btn-soft mt-4" disabled={twoFa.busy} onclick={startTwoFaSetup}>
						{twoFa.busy ? tr('Menyiapkan…') : tr('Aktifkan verifikasi dua langkah')}
					</button>
					<p class="mt-2 text-xs text-muted-foreground">{tr("Dengan 2FA aktif, setiap login membutuhkan password dan kode 6 digit dari aplikasi autentikator, melindungi akun Anda meski password bocor.")}</p>
				{/if}

				{#if !twoFa.enabled && twoFa.step === 'setup' && twoFa.otpauthUrl}
					<div class="mt-4 space-y-3 text-sm">
						<div>
							<p class="font-medium">{tr("1. Tambahkan akun di aplikasi autentikator")}</p>
							<p class="mt-1 text-xs text-muted-foreground">{tr("Buka Google Authenticator / Authy, tambah akun, lalu pindai QR berikut atau masukkan kunci manual:")}</p>
						</div>
						<div class="flex flex-col gap-3 sm:flex-row sm:items-start">
							<!-- SVG dari uqr hanya punya viewBox; tanpa lebar eksplisit dia
							     runtuh 0px di dalam flex, jadi wrapper memaksa ukurannya. -->
							<div class="w-40 shrink-0 rounded-lg border p-2 [&>svg]:block [&>svg]:h-auto [&>svg]:w-full" style="border-color: var(--border); background-color: #fff;" aria-hidden="true">
								{@html twoFaQr}
							</div>
							<div class="min-w-0 flex-1 rounded-lg border p-3" style="border-color: var(--border); background-color: var(--muted);">
								<p class="text-[11px] text-muted-foreground">{tr("Kunci rahasia (secret)")}</p>
								<p class="break-all font-mono text-sm font-medium">{twoFa.secret}</p>
								<a class="mt-2 inline-block break-all text-xs" style="color: var(--primary);" href={twoFa.otpauthUrl}>{tr("Buka tautan otpauth di aplikasi autentikator")}</a>
							</div>
						</div>
						<div>
							<p class="font-medium">{tr("2. Verifikasi kode")}</p>
							<p class="mt-1 text-xs text-muted-foreground">{tr("Masukkan kode 6 digit yang muncul di aplikasi untuk mengaktifkan:")}</p>
							<div class="mt-2 flex flex-wrap items-center gap-2">
								<input
									class="app-input w-36 font-mono"
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
					<div class="mt-4 rounded-lg border p-3.5" style="background-color: var(--warning-soft); border-color: var(--warning);">
						<p class="text-sm font-bold" style="color: var(--warning);">{tr("Simpan kode pemulihan Anda")}</p>
						<p class="mt-1 text-xs text-muted-foreground">{tr("Kode ini hanya ditampilkan sekali. Simpan di tempat aman, dipakai jika HP Anda hilang.")}</p>
						<div class="mt-2 flex flex-wrap gap-2">
							{#each twoFa.recoveryCodes as code (code)}
								<span class="rounded-md px-2 py-0.5 font-mono text-xs" style="background-color: var(--muted);">{code}</span>
							{/each}
						</div>
					</div>
				{/if}

				{#if twoFa.enabled}
					<button class="app-btn app-btn-soft mt-4" style="color: var(--destructive);" disabled={twoFa.busy} onclick={disableTwoFa}>
						{twoFa.busy ? tr('Memproses…') : tr('Nonaktifkan 2FA')}
					</button>
				{/if}
			</div>
		{/if}
	{/if}
</section>

<style>
	/* Mobile: rapatkan tinggi halaman Konfigurasi supaya isi tab Profil muat
	   tanpa menggulir di viewport ponsel (~780-900px). Desktop tidak berubah.
	   Override Tailwind di sini menang karena CSS scoped tak berlayer. */
	@media (max-width: 767px) {
		header h1 {
			font-size: 20px;
			line-height: 1.25;
		}
		header p {
			margin-top: 3px;
			font-size: 12px;
		}
		.cfg-hero {
			margin-top: 12px;
			padding: 12px 16px;
			gap: 10px;
		}
		.cfg-avatar {
			width: 40px;
			height: 40px;
		}
		.cfg-hero h2 {
			font-size: 16px;
		}
		.cfg-tabs {
			margin-top: 10px;
			/* Bilah gulir tetap tampil sebagai penanda strip ini bisa digeser. */
		}
		.cfg-tabs button {
			padding-top: 7px;
			padding-bottom: 7px;
		}
		.cfg-info {
			margin-top: 12px;
			padding: 16px 18px;
		}
		.cfg-divider {
			margin-top: 14px;
		}
		/* Dua kolom seperti desktop: enam baris data searah lebih tinggi daripada
		   viewport ponsel, dua kolom memotongnya hingga separuh. */
		.cfg-grid {
			margin-top: 14px;
			grid-template-columns: repeat(2, minmax(0, 1fr));
			row-gap: 16px;
			column-gap: 16px;
		}
		.cfg-info dl > div + div {
			margin-top: 16px;
		}
		.cfg-info dt {
			font-size: 11px;
		}
		.cfg-info dd {
			margin-top: 6px;
			font-size: 13px;
			line-height: 1.4;
		}
		.cfg-rss {
			padding: 14px 18px;
		}
	}

	/* Tingkat kedua untuk ponsel kecil (≤379px): supaya isi tab Profil tetap
	   muat tanpa menggulir di viewport sempit dan pendek (360x780). */
	@media (max-width: 379px) {
		header h1 {
			font-size: 18px;
		}
		header p {
			margin-top: 2px;
			font-size: 11px;
		}
		.cfg-hero {
			margin-top: 10px;
			padding: 10px 14px;
		}
		.cfg-avatar {
			width: 36px;
			height: 36px;
		}
		.cfg-tabs {
			margin-top: 8px;
		}
		.cfg-tabs button {
			padding-top: 6px;
			padding-bottom: 6px;
		}
		.cfg-info {
			margin-top: 12px;
			padding: 14px 16px;
		}
		.cfg-divider {
			margin-top: 12px;
		}
		.cfg-grid {
			margin-top: 12px;
			row-gap: 12px;
			column-gap: 12px;
		}
		.cfg-info dl > div + div {
			margin-top: 12px;
		}
		.cfg-info dt {
			font-size: 10.5px;
		}
		.cfg-info dd {
			margin-top: 4px;
			font-size: 12.5px;
			line-height: 1.35;
		}
		.cfg-rss {
			margin-top: 12px;
			padding: 12px 16px;
		}
		.cfg-rss p {
			font-size: 11px;
		}
	}
</style>
