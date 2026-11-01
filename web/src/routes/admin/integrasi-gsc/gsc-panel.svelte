<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Integrasi GSC — port app/admin/integrasi-gsc/page.tsx.
	// Tab Search Console (config OAuth client platform) + Google Sign-In
	// (dipakai bersama halaman Konfigurasi). Secret disegel server-side.
	import { onMount } from 'svelte';
	import { CheckCircle2, Globe, TriangleAlert } from '@lucide/svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type GscConfig = {
		client_id: string | null;
		client_secret_configured: boolean;
		redirect_uri: string | null;
		source: string;
		client_secret_decryptable?: boolean;
		config_error?: string;
		config_error_detail?: string;
		missing_fields?: string[];
	};

	let tab = $state<'gsc' | 'google-signin'>('gsc');
	let loading = $state(true);

	let config = $state<GscConfig | null>(null);
	let error = $state('');
	let notice = $state('');
	let busy = $state(false);
	let form = $state({ client_id: '', client_secret: '', redirect_uri: '' });

	// Google Sign-In
	let google = $state<{ client_id: string | null; redirect_uri: string | null; callback_url: string } | null>(null);
	let googleForm = $state({ client_id: '', client_secret: '', redirect_uri: '' });
	let googleBusy = $state(false);
	let googleMsg = $state({ text: '', ok: false });

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([loadGsc(), loadGoogle()]);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function loadGsc() {
		error = '';
		try {
			const res = await apiRequest('/v1/platform/gsc-config');
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			const data = (await res.json()) as GscConfig;
			config = data;
			form = {
				client_id: data.client_id ?? form.client_id,
				redirect_uri: data.redirect_uri ?? form.redirect_uri,
				client_secret: ''
			};
		} catch {
			error = tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		}
	}

	async function saveGsc(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		notice = '';
		try {
			const redirect = form.redirect_uri.trim() || config?.redirect_uri || 'http://localhost:3000/api/gsc/callback';
			const res = await apiRequest('/v1/platform/gsc-config', {
				method: 'PUT',
				body: JSON.stringify({ client_id: form.client_id.trim(), client_secret: form.client_secret.trim(), redirect_uri: redirect })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			notice = 'Konfigurasi GSC tersimpan di database. Silakan pastikan redirect URI sama dengan yang ada di Google Cloud OAuth Client.';
			form = { ...form, client_secret: '' };
			await loadGsc();
		} finally {
			busy = false;
		}
	}

	async function loadGoogle() {
		try {
			const res = await apiRequest('/v1/platform/oauth/google');
			if (!res.ok) return;
			const data = await res.json();
			google = data;
			googleForm = { ...googleForm, client_id: data.client_id ?? '', redirect_uri: data.redirect_uri ?? '' };
		} catch {
			/* opsional */
		}
	}

	async function saveGoogle(e: SubmitEvent) {
		e.preventDefault();
		googleBusy = true;
		googleMsg = { text: '', ok: false };
		try {
			const res = await apiRequest('/v1/platform/oauth/google', {
				method: 'PUT',
				body: JSON.stringify({
					client_id: googleForm.client_id.trim(),
					client_secret: googleForm.client_secret.trim() || null,
					redirect_uri: googleForm.redirect_uri.trim()
				})
			});
			if (!res.ok) {
				googleMsg = { text: await readApiError(res, 'Permintaan platform gagal.'), ok: false };
				return;
			}
			googleForm = { ...googleForm, client_secret: '' };
			googleMsg = { text: 'Konfigurasi Google Sign-In berhasil disimpan.', ok: true };
			await loadGoogle();
		} finally {
			googleBusy = false;
		}
	}
</script>

<section>
		{#if loading}
			<div class="app-card mt-3 p-4"><p class="text-sm text-muted-foreground">{tr("Memuat konfigurasi GSC…")}</p></div>
		{:else}
			<!-- Sidebar kanan: dua bagian konfigurasi Google, pola yang sama
			     dengan tab Email/SMTP (Konfigurasi / Template). -->
			<div class="mt-3 grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_220px]">
				<div>
					{#if tab === 'gsc'}
						<div>
							{#if error}<div class="app-card px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}
							{#if notice}<div class="app-card px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}

							<form class="app-card mt-3 space-y-3 p-4" onsubmit={saveGsc}>
								<div class="flex flex-wrap items-center gap-2">
									<Globe size={18} />
									<h2 class="text-base font-semibold">Google Search Console</h2>
								</div>
								<p class="text-sm text-muted-foreground">{tr("Konfigurasi OAuth Client disimpan di database (sealed) — ENV hanya fallback. Redirect URI harus sama dengan Authorized redirect URIs di Google Cloud Console.")}</p>
								<p class="text-xs text-muted-foreground">
									Source: {config?.source ?? 'none'} {config?.client_secret_configured ? '· Secret tersimpan ✓' : '· Secret belum ada'}
								</p>
								{#if config?.config_error_detail}
									<div class="flex items-start gap-2 rounded-lg border px-3 py-2 text-xs" style="border-color: var(--warning);">
										<TriangleAlert size={15} class="mt-0.5 shrink-0" style="color: var(--warning);" />
										<div>
											<p>{config.config_error_detail}</p>
											{#if config.missing_fields?.length}
												<p class="mt-1">{tr("Field kurang:")} <code>{config.missing_fields.join(', ')}</code></p>
											{/if}
										</div>
									</div>
								{/if}
								<label class="block">
									<span class="text-sm font-medium">GOOGLE_CLIENT_ID</span>
									<span class="block text-xs text-muted-foreground">Dari Google Cloud Console → APIs & Services → Credentials → OAuth client ID (Web application).</span>
									<input class="app-input mt-1 w-full" required placeholder="xxx.apps.googleusercontent.com" bind:value={form.client_id} />
								</label>
								<label class="block">
									<span class="text-sm font-medium">GOOGLE_CLIENT_SECRET</span>
									<span class="block text-xs text-muted-foreground">{tr("Akan di-seal dengan APP_SECRET dan disimpan di DB. Kosongkan jika tidak ingin mengganti.")}</span>
									<input class="app-input mt-1 w-full" type="password" placeholder="••••••••" bind:value={form.client_secret} />
								</label>
								<label class="block">
									<span class="text-sm font-medium">GOOGLE_REDIRECT_URI</span>
									<span class="block text-xs text-muted-foreground">Harus sama persis dengan Authorized redirect URIs di Google Cloud. Contoh dev: http://localhost:3000/api/gsc/callback</span>
									<input class="app-input mt-1 w-full" required placeholder="http://localhost:3000/api/gsc/callback" bind:value={form.redirect_uri} />
								</label>
								<div class="flex justify-end">
									<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? 'Menyimpan…' : tr('Simpan konfigurasi')}</button>
								</div>
								<div class="rounded-lg px-3 py-2 text-xs text-muted-foreground" style="background-color: var(--secondary);">
									<p class="font-medium">Checklist Google Cloud:</p>
									<p>{tr("1. Buat Project → Enable “Search Console API” → OAuth consent screen (External/Internal) → Scopes: https://www.googleapis.com/auth/webmasters.readonly")}</p>
									<p>2. Credentials → Create OAuth client ID (Web application) → Authorized redirect URIs: masukkan nilai Redirect URI di atas.</p>
									<p>3. Untuk localhost: tambahkan http://localhost:3000/api/gsc/callback secara eksplisit (Google memperbolehkan localhost tanpa HTTPS).</p>
									<p>{tr("4. Salin Client ID & Secret ke form ini → Simpan. Tidak perlu hardcode di .env (ENV hanya fallback).")}</p>
								</div>
							</form>
						</div>
					{:else}
						<div>
							<div class="app-card space-y-3 p-4">
								<div class="flex flex-wrap items-center gap-2">
									<CheckCircle2 size={18} />
									<h2 class="text-base font-semibold">Google Sign-In</h2>
									<Badge tone={google?.client_id ? 'success' : 'warning'} label={google?.client_id ? 'Terhubung' : tr('Belum Dikonfigurasi')} />
								</div>
								<p class="text-sm text-muted-foreground">{tr("Konfigurasi OAuth untuk login/register. Disimpan di database (sealed) — ENV hanya fallback. Redirect URI harus sama dengan Authorized redirect URIs di Google Cloud Console.")}</p>

								<p class="text-sm text-muted-foreground">
									Google Sign-In callback: <code class="text-xs">{google?.callback_url ?? 'https://app.tauthub.com/v1/auth/google/callback'}</code>
								</p>
								<div class="flex items-start gap-2 rounded-lg border px-3 py-2 text-xs" style="border-color: var(--warning);">
									<TriangleAlert size={15} class="mt-0.5 shrink-0" style="color: var(--warning);" />
									<p>{tr("Daftarkan callback di Google Cloud Console → OAuth 2.0 Client → Authorized redirect URIs. Jangan pakai /api/gsc/callback (itu untuk Search Console).")}</p>
								</div>
								<form class="space-y-3" onsubmit={saveGoogle}>
									<div class="grid gap-3 md:grid-cols-2">
										<label class="block">
											<span class="text-sm font-medium">Client ID</span>
											<input class="app-input mt-1 w-full" required placeholder="xxxxx.apps.googleusercontent.com" bind:value={googleForm.client_id} />
										</label>
										<label class="block">
											<span class="text-sm font-medium">Client Secret</span>
											<input class="app-input mt-1 w-full" type="password" placeholder={tr("Kosongkan jika tidak diubah")} bind:value={googleForm.client_secret} />
										</label>
									</div>
									<label class="block">
										<span class="text-sm font-medium">Redirect URI</span>
										<input class="app-input mt-1 w-full" required type="url" placeholder="https://app.tauthub.com/v1/auth/google/callback" bind:value={googleForm.redirect_uri} />
									</label>
									{#if googleMsg.text}
										<div class="flex items-center gap-2 text-sm" style="color: {googleMsg.ok ? 'var(--success)' : 'var(--destructive)'};">
											{#if googleMsg.ok}<CheckCircle2 size={15} />{:else}<TriangleAlert size={15} />{/if}
											{googleMsg.text}
										</div>
									{/if}
									<button class="app-btn app-btn-primary" type="submit" disabled={googleBusy}>{googleBusy ? 'Menyimpan…' : tr('Simpan Konfigurasi')}</button>
								</form>
							</div>
						</div>
					{/if}
				</div>

				<aside class="app-card p-2 lg:sticky lg:top-20">
					<nav class="flex gap-1 lg:flex-col" aria-label={tr('Bagian integrasi Google')}>
						<button
							type="button"
							class="app-btn !min-h-8 !justify-start !px-3 !text-xs"
							class:app-btn-primary={tab === 'gsc'}
							class:app-btn-soft={tab !== 'gsc'}
							onclick={() => (tab = 'gsc')}>{tr('Search Console (GSC)')}</button
						>
						<button
							type="button"
							class="app-btn !min-h-8 !justify-start !px-3 !text-xs"
							class:app-btn-primary={tab === 'google-signin'}
							class:app-btn-soft={tab !== 'google-signin'}
							onclick={() => (tab = 'google-signin')}>{tr('Google Sign-In')}</button
						>
					</nav>
				</aside>
			</div>
		{/if}
	</section>
