<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Welcome Gate pendaftaran — satu pintu untuk dua jalur:
	//   • Email: AuthPanel menyimpan draf isian (tauthub_reg_draft) lalu menaruh
	//     user di sini; peran dipilih lewat kartu, pendaftaran dikirim setelahnya.
	//   • Google: callback menaruh user di sini dengan tiket "register" (cookie
	//     HttpOnly sekali pakai); peran dipilih, lalu /v1/auth/google/complete.
	// Tautan lama /register-role?role=client|partner tetap langsung menyelesaikan
	// pendaftaran Google dengan peran itu (perilaku kompatibilitas).
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { Briefcase, CircleAlert, Store } from '@lucide/svelte';
	import AuthShell from '$lib/components/auth/auth-shell.svelte';
	import TurnstileWidget from '$lib/components/turnstile.svelte';
	import '$lib/components/auth/auth.css';
	import { readApiError, resolveHomeRoute, storeSession, type TokenResponse } from '$lib/auth';

	type Role = 'client' | 'partner';
	type RegDraft = {
		display_name: string;
		email: string;
		password: string;
		turnstile_token: string;
	};

	const ROLE_ALIAS: Record<string, Role> = {
		client: 'client',
		klien: 'client',
		partner: 'partner',
		mitra: 'partner'
	};

	let kind = $state<'google' | 'email' | 'legacy'>('legacy');
	// Token Turnstile segar untuk langkah login-otomatis setelah akun dibuat
	// (token dari /register dipakai sekali oleh pendaftaran).
	let gateTurnstileEnabled = $state(false);
	let gateTurnstileSiteKey = $state('');
	let gateToken = $state('');
	let gateNonce = $state(0);
	let draft = $state<RegDraft | null>(null);
	let roleChoice = $state<Role | null>(null);
	let busy = $state(false);
	let error = $state('');

	/** returnTo dari callback Google, hanya path same-origin yang aman. */
	function safeReturnTo(): string | null {
		const value = new URLSearchParams(window.location.search).get('returnTo');
		if (!value || !value.startsWith('/') || value.startsWith('//')) return null;
		if (/[\u0000-\u001f\u007f]/.test(value)) return null;
		const path = value.split(/[?#]/)[0].replace(/\/+$/, '') || '/';
		if (path === '/login' || path.startsWith('/login/')) return null;
		return value;
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

	function clearDraft() {
		try {
			sessionStorage.removeItem('tauthub_reg_draft');
		} catch {
			// abaikan
		}
	}

	async function homeAfter(data: TokenResponse, fallbackRole: Role) {
		const target = safeReturnTo() ?? (await resolveHomeRoute(data.user?.role ?? fallbackRole));
		await goto(target, { replaceState: true });
	}

	/** Jalur kompatibilitas: /register-role?role=... langsung menyelesaikan Google. */
	async function completeGoogle(role: Role) {
		const ref = referralCode();
		const payload: Record<string, unknown> = { role };
		if (ref) payload.referral_code = ref;
		try {
			const res = await fetch('/v1/auth/google/complete', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(payload)
			});
			if (res.ok) {
				const data = (await res.json()) as TokenResponse;
				storeSession(data);
				await homeAfter(data, role);
				return;
			}
		} catch {
			// Jaringan gagal: jatuh ke form daftar biasa di bawah.
		}
		// Tanpa tiket Google (mis. tautan lama) atau tiket kedaluwarsa: kembalikan
		// ke form daftar, perilaku lama.
		await goto('/login?mode=register', { replaceState: true });
	}

	onMount(() => {
		// Konfigurasi Turnstile dibaca langsung supaya gate punya token sendiri
		// untuk auto-login (register sudah memakai token dari halaman /register).
		void (async () => {
			try {
				const res = await fetch('/v1/auth/turnstile', { cache: 'no-store' });
				if (res.ok) {
					const data = await res.json();
					gateTurnstileEnabled = Boolean(data.enabled);
					gateTurnstileSiteKey = data.site_key ?? '';
				}
			} catch {
				// Widget digagalkan: pendaftaran tetap jalan bila server melonggarkan.
			}
		})();
		const roleParam = new URLSearchParams(window.location.search).get('role');
		const role = roleParam ? ROLE_ALIAS[roleParam] : null;
		if (role) {
			void completeGoogle(role);
			return;
		}
		try {
			const raw = sessionStorage.getItem('tauthub_reg_draft');
			if (raw?.trim()) {
				const parsed = JSON.parse(raw) as RegDraft;
				if (parsed?.email && parsed?.password) {
					draft = {
						display_name: String(parsed.display_name ?? '').slice(0, 120),
						email: String(parsed.email).slice(0, 320).toLowerCase(),
						password: String(parsed.password),
						turnstile_token: String(parsed.turnstile_token ?? '')
					};
					kind = 'email';
					return;
				}
			}
		} catch {
			// sessionStorage bisa diblokir; tetap coba jalur Google di bawah.
		}
		kind = 'google';
	});

	/** Pilih peran → lanjutkan jalur yang sesuai. */
	async function submitRole() {
		if (!roleChoice || busy) return;
		busy = true;
		error = '';
		try {
			if (kind === 'email' && draft) {
				const payload: Record<string, unknown> = {
					email: draft.email,
					password: draft.password,
					display_name: draft.display_name,
					role: roleChoice,
					// App lama tidak lagi menanyakan kategori media; backend mewajibkan
					// 'lainnya' untuk peran Mitra.
					partner_category: roleChoice === 'partner' ? 'lainnya' : undefined,
					turnstile_token: draft.turnstile_token
				};
				const ref = referralCode();
				const headers: Record<string, string> = { 'Content-Type': 'application/json' };
				if (ref) {
					payload.referral_code = ref;
					headers['x-referral-code'] = ref;
				}
				if (gateTurnstileEnabled) payload.turnstile_token = gateToken || draft.turnstile_token;
				const res = await fetch('/v1/auth/register', {
					method: 'POST',
					headers,
					body: JSON.stringify(payload)
				});
				if (!res.ok) {
					error = await readApiError(res, tr('Pendaftaran gagal. Periksa isian lalu coba lagi.'));
					busy = false;
					return;
				}
				clearDraft();
				const loginRes = await fetch('/v1/auth/login', {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ email: draft.email, password: draft.password, turnstile_token: gateToken || draft.turnstile_token })
				});
				if (!loginRes.ok) {
					await goto('/login?mode=register', { replaceState: true });
					return;
				}
				const data = (await loginRes.json()) as TokenResponse;
				storeSession(data);
				await homeAfter(data, roleChoice);
				return;
			}

			const ref = referralCode();
			const payload: Record<string, unknown> = { role: roleChoice };
			if (ref) payload.referral_code = ref;
			const res = await fetch('/v1/auth/google/complete', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(payload)
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Sesi pendaftaran kedaluwarsa. Silakan daftar ulang.'));
				busy = false;
				return;
			}
			const data = (await res.json()) as TokenResponse;
			storeSession(data);
			await homeAfter(data, roleChoice);
		} catch {
			error = tr('Error, coba lagi nanti.');
			busy = false;
		}
	}

	function backToRegister() {
		clearDraft();
		void goto('/login?mode=register', { replaceState: true });
	}
</script>

<svelte:head>
	<title>{tr("Buat Akun · Pilih Peran")}</title>
</svelte:head>

<AuthShell>
	{#if kind === 'legacy'}
		<h1 class="auth-title">{tr("Menyelesaikan pendaftaran…")}</h1>
		<p class="auth-subtitle">{tr("Sebentar, akun Google Anda sedang dihubungkan.")}</p>
	{:else}
		<h1 class="auth-title">{tr('Selamat datang di Tauthub')}</h1>
		<p class="auth-subtitle">
			{kind === 'email' && draft
				? tr('Mendaftar sebagai {nama} ({email}). Pilih peran kamu untuk membuat akun.')
						.replace('{nama}', draft.display_name || draft.email)
						.replace('{email}', draft.email)
				: tr('Pilih peran kamu untuk melanjutkan pendaftaran.')}
		</p>

		{#if error}
			<p class="auth-alert" role="alert"><CircleAlert size={15} />{error}</p>
			<a class="auth-link" href="/login?mode=register">{tr('Kembali ke form daftar')}</a>
		{/if}

		<div class="gate-grid">
			<label class="gate-card" data-checked={roleChoice === 'client'}>
				<input
					type="radio"
					name="gate-role"
					value="client"
					checked={roleChoice === 'client'}
					onchange={() => (roleChoice = 'client')}
				/>
				<span class="gate-icon"><Briefcase size={38} /></span>
				<span class="gate-card-body">
					<span class="gate-card-title">{tr('Klien')}</span>
					<span class="gate-card-desc">{tr('Untuk perusahaan, tim, atau individu yang ingin menggunakan layanan Tauthub.')}</span>
				</span>
			</label>
			<label class="gate-card" data-checked={roleChoice === 'partner'}>
				<input
					type="radio"
					name="gate-role"
					value="partner"
					checked={roleChoice === 'partner'}
					onchange={() => (roleChoice = 'partner')}
				/>
				<span class="gate-icon"><Store size={38} /></span>
				<span class="gate-card-body">
					<span class="gate-card-title">{tr('Mitra')}</span>
					<span class="gate-card-desc">{tr('Untuk media partner, penerbit, atau pemilik media yang ingin bekerja sama dengan Tauthub.')}</span>
				</span>
			</label>
		</div>

		{#if kind === 'email' && gateTurnstileEnabled && gateTurnstileSiteKey}
			{#key gateNonce}
				<TurnstileWidget
					siteKey={gateTurnstileSiteKey}
					onToken={(token) => (gateToken = token)}
					onExpire={() => {
						gateToken = '';
						gateNonce += 1;
					}}
				/>
			{/key}
		{/if}

		<button class="auth-cta gate-cta" type="button" disabled={!roleChoice || busy} onclick={() => void submitRole()}>
			{busy ? tr('Sedang membuat akun…') : tr('Buat Akun')}
		</button>

		{#if kind === 'email'}
			<p class="gate-back-row">
				<a class="auth-link" href="/login?mode=register" onclick={() => backToRegister()}>{tr('Bukan kamu? Kembali ke form daftar')}</a>
			</p>
		{/if}
	{/if}
</AuthShell>

<style>
	/* Gerbang butuh kanvas lebih lebar daripada batas 440px form auth di
	   auth.css — tanpa ini kartu perannya sesak. Hanya berlaku di halaman ini
	   (style halaman dibongkar saat pindah route), login/register tetap 440px. */
	:global(.auth-form-inner) {
		max-width: 720px;
	}
	/* Dua kartu peran: ikon di kiri, judul + deskripsi di kanan, lingkaran radio
	   kecil di pojok kanan atas. Terpilih = ring biru + latar biru sangat lembut. */
	.gate-grid {
		display: grid;
		grid-template-columns: 1fr;
		gap: 14px;
		margin-top: 20px;
		width: 100%;
	}
	@media (min-width: 640px) {
		.gate-grid {
			grid-template-columns: 1fr 1fr;
		}
	}
	.gate-card {
		position: relative;
		display: flex;
		flex-direction: column;
		gap: 14px;
		padding: 18px;
		border: 1px solid var(--border);
		border-radius: 16px;
		background: #f8fafc;
		cursor: pointer;
		transition:
			border-color 0.15s ease,
			background-color 0.15s ease,
			box-shadow 0.15s ease;
	}
	.gate-card:hover {
		border-color: color-mix(in srgb, var(--primary) 35%, var(--border));
	}
	.gate-card:has(input:focus-visible) {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
	}
	.gate-card[data-checked='true'] {
		border-color: var(--primary);
		box-shadow: inset 0 0 0 1px var(--primary);
		background: color-mix(in srgb, var(--accent) 55%, #fff);
	}
	.gate-card input {
		position: absolute;
		top: 14px;
		right: 14px;
		width: 18px;
		height: 18px;
		margin: 0;
		accent-color: var(--primary);
	}
	.gate-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 104px;
		height: 104px;
		border-radius: 9999px;
		background: #e3e8ef;
		color: #667085;
	}
	.gate-card-body {
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding-right: 26px;
	}
	.gate-card-title {
		font-size: 15px;
		font-weight: 700;
		color: var(--foreground, #111827);
	}
	.gate-card-desc {
		font-size: 12.5px;
		line-height: 1.55;
		color: var(--muted-foreground);
	}
		.gate-cta {
		width: 100%;
		margin-top: 18px;
		text-align: center;
	}
	.gate-back-row {
		margin-top: 14px;
		text-align: center;
	}
</style>
