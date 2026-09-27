<script lang="ts">
	// Profil & Akun — identitas brand, profil brand, aturan editorial, keamanan
	// akun (nama/password/2FA), preferensi notifikasi. Semua endpoint sudah ada
	// (fase 1 & 2), jadi tidak ada data placeholder.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { KeyRound, ShieldCheck, UserCircle, Bell } from '@lucide/svelte';
	import {
		apiRequest, readApiError, ensureValidSession, getStoredSession,
		updateStoredUser, AUTH_EXPIRED_EVENT, type AuthUser
	} from '$lib/auth';

	const brandId = $derived(page.params.brandId as string);

	let user = $state<AuthUser | null>(null);
	let brand = $state<any>(null);
	let profile = $state({ positioning: '', value_proposition: '', target_audience: '' });
	let rules = $state({ required_rules: '', forbidden_rules: '', required_disclaimer: '', citation_policy: '' });
	let notif = $state({ wa_number: '', wa_consent: false });
	let password = $state({ current: '', next: '' });
	let twoFactorSecret = $state('');
	let twoFactorCode = $state('');
	let recoveryCodes = $state<string[]>([]);

	let busy = $state(false);
	let msg = $state('');
	let error = $state('');

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
		const detail = await apiRequest(`/v1/brands/${brandId}`);
		if (detail.ok) brand = await detail.json();
		const cfg = await apiRequest(`/v1/brands/${brandId}/configuration`);
		if (cfg.ok) {
			const d = await cfg.json();
			if (d.profile) {
				profile = {
					positioning: d.profile.positioning ?? '',
					value_proposition: d.profile.value_proposition ?? '',
					target_audience: d.profile.target_audience ?? ''
				};
			}
			if (d.editorial_rules) {
				rules = {
					required_rules: (d.editorial_rules.required_rules ?? []).join(', '),
					forbidden_rules: (d.editorial_rules.forbidden_rules ?? []).join(', '),
					required_disclaimer: d.editorial_rules.required_disclaimer ?? '',
					citation_policy: d.editorial_rules.citation_policy ?? ''
				};
			}
		}
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

	async function saveProfile(e: SubmitEvent) {
		e.preventDefault();
		await withBusy(async () => {
			const res = await apiRequest(`/v1/brands/${brandId}/profile`, {
				method: 'PUT',
				body: JSON.stringify({
					positioning: profile.positioning || null,
					value_proposition: profile.value_proposition || null,
					target_audience: profile.target_audience || null
				})
			});
			msg = res.ok ? 'Profil brand tersimpan.' : await readApiError(res, 'Gagal menyimpan profil brand.');
			if (res.ok) await load();
		});
	}

	async function saveRules(e: SubmitEvent) {
		e.preventDefault();
		await withBusy(async () => {
			const res = await apiRequest(`/v1/brands/${brandId}/editorial-rules`, {
				method: 'PUT',
				body: JSON.stringify({
					required_rules: rules.required_rules.split(',').map((s) => s.trim()).filter(Boolean),
					forbidden_rules: rules.forbidden_rules.split(',').map((s) => s.trim()).filter(Boolean),
					required_disclaimer: rules.required_disclaimer,
					citation_policy: rules.citation_policy
				})
			});
			msg = res.ok ? 'Aturan editorial tersimpan.' : await readApiError(res, 'Gagal menyimpan aturan editorial.');
			if (res.ok) await load();
		});
	}

	async function saveNotif(e: SubmitEvent) {
		e.preventDefault();
		await withBusy(async () => {
			const res = await apiRequest('/v1/me/notification-preferences', {
				method: 'PUT',
				body: JSON.stringify({ wa_number: notif.wa_number, wa_consent: notif.wa_consent })
			});
			msg = res.ok ? 'Preferensi notifikasi tersimpan.' : await readApiError(res, 'Gagal menyimpan preferensi.');
		});
	}

	async function rename() {
		if (!user) return;
		const name = window.prompt('Nama tampilan baru:', user.display_name);
		if (!name || name.trim().length < 2) return;
		await withBusy(async () => {
			const res = await apiRequest('/v1/auth/me', { method: 'PATCH', body: JSON.stringify({ display_name: name.trim() }) });
			if (res.ok) {
				const updated = (await res.json()) as AuthUser;
				user = updated;
				updateStoredUser(updated);
				msg = 'Nama tampilan diperbarui.';
			} else {
				error = await readApiError(res, 'Gagal memperbarui nama.');
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
				error = await readApiError(res, 'Gagal mengubah password.');
			}
		});
	}

	async function startTwoFactor() {
		await withBusy(async () => {
			const res = await apiRequest('/v1/auth/2fa/setup', { method: 'POST' });
			if (res.ok) {
				const d = await res.json();
				twoFactorSecret = d.secret;
				msg = 'Masukkan secret ke aplikasi autentikator lalu isi kode 6 digit.';
			} else {
				error = await readApiError(res, 'Gagal memulai setup 2FA.');
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
				twoFactorCode = '';
				msg = '2FA aktif. Simpan kode pemulihan sekarang.';
			} else {
				error = await readApiError(res, 'Kode verifikasi salah.');
			}
		});
	}
</script>

<section class="mx-auto max-w-3xl">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Profil &amp; Akun</h1>
		<p class="mt-1 text-sm text-muted-foreground">
			Identitas akun, profil brand, aturan editorial, dan keamanan.
		</p>
	</header>

	{#if msg}<p class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{msg}</p>{/if}
	{#if error}<p class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</p>{/if}

	<!-- Akun -->
	<div class="app-card mt-4 p-5">
		<div class="flex items-center gap-3">
			<UserCircle size={18} style="color: var(--primary);" />
			<h2 class="font-semibold">Akun</h2>
		</div>
		{#if user}
			<dl class="mt-3 grid gap-3 sm:grid-cols-3 text-sm">
				<div><dt class="text-xs text-muted-foreground">Nama tampilan</dt><dd class="font-medium">{user.display_name || '—'}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Email</dt><dd class="font-medium">{user.email}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Peran</dt><dd class="font-medium uppercase">{user.role ?? '—'}</dd></div>
			</dl>
			<button class="app-btn app-btn-soft mt-3" disabled={busy} onclick={rename}>Ubah nama tampilan</button>
		{/if}
	</div>

	<!-- Brand -->
	<div class="app-card mt-4 p-5">
		<div class="flex items-center justify-between gap-3">
			<h2 class="font-semibold">Identitas brand</h2>
			{#if brand}
				<span class="rounded-full px-2 py-0.5 text-[10px] font-semibold" style="background-color: var(--accent); color: var(--accent-foreground);">
					readiness {brand.readiness_score}%
				</span>
			{/if}
		</div>
		{#if brand}
			<dl class="mt-3 grid gap-3 sm:grid-cols-2 text-sm">
				<div><dt class="text-xs text-muted-foreground">Nama</dt><dd class="font-medium">{brand.name}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Slug</dt><dd class="font-medium">/{brand.slug}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Tipe</dt><dd class="font-medium">{brand.ownership_type}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Industri</dt><dd class="font-medium">{brand.industry ?? '—'}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Status</dt><dd class="font-medium">{brand.status}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Bahasa utama</dt><dd class="font-medium">{brand.primary_language}</dd></div>
			</dl>
		{/if}
	</div>

	<!-- Profil brand -->
	<form class="app-card mt-4 p-5" onsubmit={saveProfile}>
		<h2 class="font-semibold">Profil brand (konteks AI)</h2>
		<p class="mt-1 text-xs text-muted-foreground">Dipakai sebagai konteks grounding saat AI menulis.</p>
		<div class="mt-3 grid gap-3">
			<label class="flex flex-col gap-1 text-sm">Positioning<input class="app-input" bind:value={profile.positioning} /></label>
			<label class="flex flex-col gap-1 text-sm">Value proposition<input class="app-input" bind:value={profile.value_proposition} /></label>
			<label class="flex flex-col gap-1 text-sm">Target audiens<input class="app-input" bind:value={profile.target_audience} /></label>
		</div>
		<button class="app-btn app-btn-primary mt-3" type="submit" disabled={busy}>Simpan profil</button>
	</form>

	<!-- Aturan editorial -->
	<form class="app-card mt-4 p-5" onsubmit={saveRules}>
		<h2 class="font-semibold">Aturan editorial</h2>
		<div class="mt-3 grid gap-3">
			<label class="flex flex-col gap-1 text-sm">Aturan wajib (pisah koma)<input class="app-input" bind:value={rules.required_rules} /></label>
			<label class="flex flex-col gap-1 text-sm">Aturan terlarang (pisah koma)<input class="app-input" bind:value={rules.forbidden_rules} /></label>
			<label class="flex flex-col gap-1 text-sm">Disclaimer wajib<input class="app-input" bind:value={rules.required_disclaimer} /></label>
			<label class="flex flex-col gap-1 text-sm">Kebijakan sitasi<input class="app-input" bind:value={rules.citation_policy} /></label>
		</div>
		<button class="app-btn app-btn-primary mt-3" type="submit" disabled={busy}>Simpan aturan</button>
	</form>

	<!-- Notifikasi -->
	<form class="app-card mt-4 p-5" onsubmit={saveNotif}>
		<div class="flex items-center gap-3">
			<Bell size={18} style="color: var(--primary);" />
			<h2 class="font-semibold">Preferensi notifikasi</h2>
		</div>
		<p class="mt-1 text-xs text-muted-foreground">Nomor WhatsApp dipakai worker untuk mengirim pemberitahuan status publikasi.</p>
		<div class="mt-3 grid gap-3 sm:grid-cols-2">
			<label class="flex flex-col gap-1 text-sm">Nomor WhatsApp
				<input class="app-input" placeholder="62812xxxxxxx" bind:value={notif.wa_number} />
			</label>
			<label class="mt-5 flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={notif.wa_consent} />
				Setuju dihubungi via WhatsApp
			</label>
		</div>
		<button class="app-btn app-btn-primary mt-3" type="submit" disabled={busy}>Simpan preferensi</button>
	</form>

	<!-- Keamanan -->
	<div class="app-card mt-4 p-5">
		<div class="flex items-center gap-3">
			<KeyRound size={18} style="color: var(--primary);" />
			<h2 class="font-semibold">Keamanan</h2>
		</div>

		<form class="mt-4" onsubmit={changePassword}>
			<p class="text-sm font-medium">Ganti password</p>
			<div class="mt-2 grid gap-2 sm:grid-cols-2">
				<input class="app-input" type="password" placeholder="Password saat ini" bind:value={password.current} required />
				<input class="app-input" type="password" placeholder="Password baru (min. 10)" bind:value={password.next} minlength={10} required />
			</div>
			<button class="app-btn app-btn-primary mt-2" type="submit" disabled={busy}>Simpan password</button>
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
				<p class="mt-2 text-xs text-muted-foreground">Masukkan secret ini ke aplikasi autentikator:</p>
				<code class="mt-1 block break-all rounded-lg p-3 text-xs" style="background-color: var(--muted);">{twoFactorSecret}</code>
				<form class="mt-2 flex gap-2" onsubmit={enableTwoFactor}>
					<input class="app-input !w-36" placeholder="123456" bind:value={twoFactorCode} required />
					<button class="app-btn app-btn-primary" type="submit" disabled={busy}>Aktifkan</button>
				</form>
			{:else}
				<p class="mt-1 text-xs text-muted-foreground">Tambahkan lapisan kedua saat login dengan kode TOTP.</p>
				<button class="app-btn app-btn-soft mt-2" disabled={busy} onclick={startTwoFactor}>Mulai setup 2FA</button>
			{/if}
		</div>
	</div>
</section>
