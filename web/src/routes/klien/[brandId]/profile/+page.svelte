<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Profil & Akun — tab Akun / Preferensi notifikasi / Keamanan. Semua endpoint
	// sudah ada (fase 1 & 2), jadi tidak ada data placeholder.
	import { onMount } from 'svelte';
	import { KeyRound, ShieldCheck, UserCircle, Bell } from '@lucide/svelte';
	import {
		apiRequest, readApiError, ensureValidSession, getStoredSession,
		updateStoredUser, AUTH_EXPIRED_EVENT, type AuthUser
	} from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
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
		await withBusy(async () => {
			const res = await apiRequest('/v1/auth/change-password', {
				method: 'POST',
				body: JSON.stringify({ current_password: password.current, new_password: password.next })
			});
			if (res.status === 204) {
				msg = 'Password diubah. Sesi lain dicabut.';
				password = { current: '', next: '' };
			} else {
				error = await readApiError(res, tr('Gagal mengubah password.'));
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
		<h1 class="text-2xl font-semibold tracking-tight">{tr("Profil & Akun")}</h1>
		<p class="mt-1 text-sm text-muted-foreground">{tr("Kelola identitas akun, preferensi notifikasi, dan keamanan.")}</p>
	</header>

	{#if msg}<p class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{msg}</p>{/if}
	{#if error}<p class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</p>{/if}

	<!-- Tab: Akun / Preferensi notifikasi / Keamanan -->
	<nav class="mt-4 flex items-center gap-1 border-b" style="border-color: var(--border);" aria-label={tr('Bagian profil')}>
		{#each TABS as t (t.key)}
			<button
				type="button"
				class="-mb-px border-b-2 px-3 py-2 text-sm font-medium transition-colors"
				style={tab === t.key ? 'color: var(--foreground); border-color: var(--primary);' : 'color: var(--muted-foreground); border-color: transparent;'}
				aria-current={tab === t.key ? 'page' : undefined}
				onclick={() => (tab = t.key)}
			>{t.label}</button>
		{/each}
	</nav>

	{#if tab === 'akun'}
		<!-- Akun -->
		<div class="app-card mt-4 p-5">
			<div class="flex items-center gap-3">
				<UserCircle size={18} style="color: var(--primary);" />
				<h2 class="font-semibold">{tr("Akun")}</h2>
			</div>
			{#if user}
				<dl class="mt-3 grid gap-3 text-sm sm:grid-cols-3">
					<div class={editingName ? 'sm:col-span-3' : ''}>
						<dt class="text-xs text-muted-foreground">{tr("Nama tampilan")}</dt>
						{#if editingName}
							<form class="mt-1.5 flex flex-wrap items-center gap-2" onsubmit={saveName}>
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
								<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" type="submit" disabled={busy}>{tr("Simpan")}</button>
								<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={busy} onclick={() => (editingName = false)}>{tr("Batal")}</button>
							</form>
						{:else}
							<dd class="mt-1.5 font-medium">{user.display_name || '—'}</dd>
						{/if}
					</div>
					<div><dt class="text-xs text-muted-foreground">{tr("Email")}</dt><dd class="mt-1.5 font-medium">{user.email}</dd></div>
					<div><dt class="text-xs text-muted-foreground">{tr("Peran")}</dt><dd class="mt-1.5 font-medium uppercase">{user.role ?? '—'}</dd></div>
				</dl>
				{#if !editingName}
					<button class="app-btn app-btn-soft mt-3" disabled={busy} onclick={rename}>{tr("Ubah nama tampilan")}</button>
				{/if}
			{/if}
		</div>
	{:else if tab === 'notifikasi'}
		<!-- Notifikasi -->
		<form class="app-card mt-4 p-5" onsubmit={saveNotif}>
			<div class="flex items-center gap-3">
				<Bell size={18} style="color: var(--primary);" />
				<h2 class="font-semibold">Preferensi notifikasi</h2>
			</div>
			<p class="mt-1 text-xs text-muted-foreground">{tr("Nomor WhatsApp dipakai worker untuk mengirim pemberitahuan status publikasi.")}</p>
			<div class="mt-3 grid gap-3 sm:grid-cols-2">
				<label class="flex flex-col gap-1 text-sm">Nomor WhatsApp
					<input class="app-input" placeholder="62812xxxxxxx" bind:value={notif.wa_number} />
				</label>
				<label class="mt-5 flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={notif.wa_consent} />
					Setuju dihubungi via WhatsApp
				</label>
			</div>
			<button class="app-btn app-btn-primary mt-3" type="submit" disabled={busy}>{tr("Simpan preferensi")}</button>
		</form>
	{:else}
		<!-- Keamanan -->
		<div class="app-card mt-4 p-5">
			<div class="flex items-center gap-3">
				<KeyRound size={18} style="color: var(--primary);" />
				<h2 class="font-semibold">Keamanan</h2>
			</div>

			<form class="mt-4" onsubmit={changePassword}>
				<p class="text-sm font-medium">Ganti password</p>
				<div class="mt-2 grid gap-2 sm:grid-cols-2">
					<input class="app-input" type="password" placeholder={tr("Password saat ini")} bind:value={password.current} required />
					<input class="app-input" type="password" placeholder={tr("Password baru (min. 10)")} bind:value={password.next} minlength={10} required />
				</div>
				<button class="app-btn app-btn-primary mt-2" type="submit" disabled={busy}>{tr("Simpan password")}</button>
			</form>

			<div class="mt-5 border-t pt-4">
				<div class="flex items-center gap-2">
					<ShieldCheck size={16} style="color: var(--success);" />
					<p class="text-sm font-medium">Two-Factor Authentication</p>
				</div>
				{#if recoveryCodes.length > 0}
					<p class="mt-2 text-sm" style="color: var(--success);">2FA aktif. Kode pemulihan (hanya tampil sekali):</p>
					<code class="mt-2 block whitespace-pre-wrap rounded-lg p-3 text-xs" style="background-color: var(--muted);">{recoveryCodes.join('\n')}</code>
			{:else if twoFactorSecret}
				<div class="mt-3 flex flex-col gap-4 sm:flex-row sm:items-start">
					{#if twoFactorQr}
						<!-- SVG dari uqr hanya punya viewBox; tanpa lebar eksplisit dia
						     runtuh 0px di dalam flex, jadi wrapper memaksa ukurannya. -->
						<div class="w-40 shrink-0 self-start rounded-lg border p-2 [&>svg]:block [&>svg]:h-auto [&>svg]:w-full" style="border-color: var(--border); background-color: #fff;" aria-hidden="true">
							{@html twoFactorQr}
						</div>
					{/if}
					<div class="min-w-0">
						<p class="text-xs text-muted-foreground">{tr('Pindai QR dengan aplikasi autentikator (Google Authenticator, Authy, dan sejenisnya), atau salin secret ini secara manual:')}</p>
						<code class="mt-1 block break-all rounded-lg p-3 text-xs" style="background-color: var(--muted);">{twoFactorSecret}</code>
						<form class="mt-2 flex gap-2" onsubmit={enableTwoFactor}>
							<input class="app-input !w-36" placeholder="123456" bind:value={twoFactorCode} required />
							<button class="app-btn app-btn-primary" type="submit" disabled={busy}>Aktifkan</button>
						</form>
					</div>
				</div>
			{:else}
					<p class="mt-1 text-xs text-muted-foreground">{tr("Tambahkan lapisan kedua saat login dengan kode TOTP.")}</p>
					<button class="app-btn app-btn-soft mt-2" disabled={busy} onclick={startTwoFactor}>Mulai setup 2FA</button>
				{/if}
			</div>
		</div>
	{/if}
</section>
