<script lang="ts">
	// Integrasi mitra — port section "Integrasi": koneksi CMS (WordPress/webhook)
	// dengan uji koneksi, kunci Publishing API, sindikasi RSS, dan GSC.
	import { onMount } from 'svelte';
	import { Check, Copy, Key, PlugZap, Plus, RefreshCw, Rss, Trash2, Zap } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	let loading = $state(true);
	let connections = $state<any[]>([]);
	let keys = $state<any[]>([]);
	let feeds = $state<any[]>([]);
	let gsc = $state<any>(null);
	let notice = $state('');
	let error = $state('');
	let busy = $state(false);
	let newKeyLabel = $state('Default');
	let newKeyMedia = $state('');
	let issuedKey = $state<any>(null);
	let newCMS = $state({ name: '', provider: 'wordpress', base_url: '', username: '', application_password: '', secret: '' });
	let showCMS = $state(false);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([loadCMS(), loadKeys(), loadFeeds(), loadGSC()]);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function loadCMS() {
		const res = await apiRequest('/v1/partner/cms-connections');
		if (res.ok) connections = (await res.json()).connections ?? [];
	}
	async function loadKeys() {
		const res = await apiRequest('/v1/partner/publishing-api-keys');
		if (res.ok) keys = (await res.json()).keys ?? [];
	}
	async function loadFeeds() {
		const res = await apiRequest('/v1/partner/rss-feeds');
		if (res.ok) feeds = (await res.json()).feeds ?? [];
	}
	async function loadGSC() {
		const res = await apiRequest('/v1/partner/gsc');
		if (res.ok) gsc = await res.json();
	}

	async function createCMS(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const payload: Record<string, unknown> = {
				name: newCMS.name.trim(),
				provider: newCMS.provider,
				base_url: newCMS.base_url.trim()
			};
			if (newCMS.provider === 'wordpress') {
				payload.username = newCMS.username.trim();
				payload.application_password = newCMS.application_password.trim();
			} else {
				payload.secret = newCMS.secret.trim();
			}
			const res = await apiRequest('/v1/partner/cms-connections', { method: 'POST', body: JSON.stringify(payload) });
			if (!res.ok) {
				error = await readApiError(res, "Permintaan gagal.");
				return;
			}
			notice = 'Koneksi CMS ditambahkan.';
			showCMS = false;
			newCMS = { name: '', provider: 'wordpress', base_url: '', username: '', application_password: '', secret: '' };
			await loadCMS();
		} finally {
			busy = false;
		}
	}

	async function testCMS(id: string) {
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/cms-connections/${id}/test`, { method: 'POST' });
			const data = await res.json();
			if (!res.ok) {
				error = data?.detail ?? data?.message ?? 'Uji koneksi gagal.';
				return;
			}
			notice = data?.message ?? 'Koneksi berhasil.';
			await loadCMS();
		} finally {
			busy = false;
		}
	}

	async function revokeCMS(id: string) {
		if (!confirm('Cabut koneksi ini?')) return;
		const res = await apiRequest(`/v1/partner/cms-connections/${id}`, { method: 'DELETE' });
		const data = await res.json().catch(() => null);
		if (!res.ok) {
			error = data?.detail ?? 'Gagal mencabut koneksi.';
			return;
		}
		notice = 'Koneksi dicabut.';
		await loadCMS();
	}

	async function createKey(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const payload: Record<string, unknown> = { label: newKeyLabel.trim() || 'Default' };
			if (newKeyMedia) payload.media_id = newKeyMedia;
			const res = await apiRequest('/v1/partner/publishing-api-keys', { method: 'POST', body: JSON.stringify(payload) });
			const data = await res.json();
			if (!res.ok) {
				error = data?.detail ?? 'Gagal membuat API key.';
				return;
			}
			issuedKey = data;
			notice = 'API key dibuat. Salin sekarang — hanya ditampilkan sekali.';
			await loadKeys();
		} finally {
			busy = false;
		}
	}

	async function rotateKey(id: string) {
		if (!confirm('Rotasi API key ini? Kredensial lama langsung tidak berlaku.')) return;
		const res = await apiRequest(`/v1/partner/publishing-api-keys/${id}`, { method: 'POST' });
		const data = await res.json();
		if (!res.ok) {
			error = data?.detail ?? 'Gagal merotasi API key.';
			return;
		}
		issuedKey = data;
		notice = 'API key dirotasi. Salin sekarang.';
		await loadKeys();
	}

	async function revokeKey(id: string) {
		if (!confirm('Cabut API key ini?')) return;
		const res = await apiRequest(`/v1/partner/publishing-api-keys/${id}`, { method: 'DELETE' });
		const data = await res.json().catch(() => null);
		if (!res.ok) {
			error = data?.detail ?? 'Gagal mencabut API key.';
			return;
		}
		notice = 'API key dicabut.';
		await loadKeys();
	}

	async function rotateFeed(id: string) {
		if (!confirm('Rotasi token feed? Tautan lama langsung tidak berlaku.')) return;
		const res = await apiRequest(`/v1/partner/rss-feeds/${id}/rotate`, { method: 'POST' });
		const data = await res.json().catch(() => null);
		if (!res.ok) {
			error = data?.detail ?? 'Gagal merotasi token.';
			return;
		}
		notice = 'Token feed dirotasi.';
		await loadFeeds();
	}

	async function verifyFeed(id: string) {
		const res = await apiRequest(`/v1/partner/rss-feeds/${id}/verify`, { method: 'POST' });
		const data = await res.json();
		if (!res.ok) {
			error = data?.detail ?? 'Verifikasi gagal.';
			return;
		}
		notice = data.installed ? 'Feed terverifikasi terpasang di website Anda.' : (data.install_error ?? 'Feed belum terpasang.');
		await loadFeeds();
	}

	async function connectGSC() {
		const res = await apiRequest('/v1/partner/gsc/connect');
		const data = await res.json();
		if (!res.ok) {
			error = data?.detail ?? 'Gagal memulai OAuth GSC.';
			return;
		}
		window.location.href = data.url;
	}

	async function copy(label: string, value: string) {
		try {
			await navigator.clipboard.writeText(value);
			notice = `${label} disalin.`;
		} catch {
			/* clipboard tidak tersedia */
		}
	}

	const dt = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '—');
</script>

<section class="mx-auto max-w-4xl">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Integrasi</h1>
		<p class="mt-1 text-sm text-muted-foreground">Hubungkan website media Anda agar artikel dapat dikirim otomatis.</p>
	</header>

	{#if notice}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}
	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat integrasi…</p>
	{:else}
		<!-- CMS connections -->
		<div class="app-card mt-4 p-4">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<div class="flex items-center gap-2">
					<PlugZap size={16} />
					<h2 class="text-sm font-semibold">Koneksi CMS</h2>
				</div>
				<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" onclick={() => (showCMS = !showCMS)}><Plus size={13} /> Tambah</button>
			</div>

			{#if showCMS}
				<form class="mt-3 space-y-2 rounded-lg border p-3" onsubmit={createCMS}>
					<div class="grid gap-2 sm:grid-cols-2">
						<input class="app-input w-full" required placeholder="Nama koneksi" bind:value={newCMS.name} />
						<select class="app-input w-full" bind:value={newCMS.provider}>
							<option value="wordpress">WordPress</option>
							<option value="webhook">Webhook</option>
						</select>
						<input class="app-input w-full sm:col-span-2" required placeholder="https://website-anda.example" bind:value={newCMS.base_url} />
						{#if newCMS.provider === 'wordpress'}
							<input class="app-input w-full" required placeholder="Username WordPress" bind:value={newCMS.username} />
							<input class="app-input w-full" required type="password" placeholder="Application Password" bind:value={newCMS.application_password} />
						{:else}
							<input class="app-input w-full sm:col-span-2" required placeholder="Secret webhook (min. 16 karakter)" bind:value={newCMS.secret} />
						{/if}
					</div>
					<div class="flex justify-end gap-2">
						<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={() => (showCMS = false)}>Batal</button>
						<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" type="submit" disabled={busy}>Simpan Koneksi</button>
					</div>
				</form>
			{/if}

			{#if connections.length === 0}
				<p class="mt-3 text-sm text-muted-foreground">Belum ada koneksi CMS.</p>
			{:else}
				<ul class="mt-3 divide-y">
					{#each connections as c (c.id)}
						<li class="flex flex-wrap items-center justify-between gap-2 py-2.5">
							<div class="min-w-0">
								<p class="text-sm font-medium">{c.name}</p>
								<p class="truncate text-xs text-muted-foreground">{c.provider} · {c.base_url} · {c.status}</p>
								{#if c.last_test_message}<p class="text-[11px] text-muted-foreground">{c.last_test_message}</p>{/if}
							</div>
							<div class="flex gap-1">
								<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" disabled={busy || c.status !== 'active'} onclick={() => testCMS(c.id)}><Zap size={12} /> Uji</button>
								{#if c.status === 'active'}
									<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label="Cabut koneksi" onclick={() => revokeCMS(c.id)}><Trash2 size={14} /></button>
								{/if}
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</div>

		<!-- Publishing API keys -->
		<div class="app-card mt-4 p-4">
			<div class="flex items-center gap-2">
				<Key size={16} />
				<h2 class="text-sm font-semibold">Publishing API</h2>
			</div>
			<p class="mt-0.5 text-xs text-muted-foreground">Untuk media dengan platform Custom Web / Tulisio API.</p>

			{#if issuedKey}
				<div class="mt-3 rounded-lg border p-3" style="border-color: var(--success, #16794b);">
					<p class="text-xs font-semibold" style="color: var(--success);">Salin kredensial sekarang — tidak ditampilkan lagi.</p>
					<div class="mt-2 space-y-2">
						<div class="flex gap-2">
							<input class="app-input w-full font-mono text-xs" readonly value={issuedKey.api_key} />
							<button class="app-btn app-btn-soft shrink-0" onclick={() => copy('API key', issuedKey.api_key)}><Copy size={13} /></button>
						</div>
						<div class="flex gap-2">
							<input class="app-input w-full font-mono text-xs" readonly value={issuedKey.secret} />
							<button class="app-btn app-btn-soft shrink-0" onclick={() => copy('Secret', issuedKey.secret)}><Copy size={13} /></button>
						</div>
					</div>
					<button class="app-btn app-btn-soft mt-2 !min-h-7 !px-2.5 !text-xs" onclick={() => (issuedKey = null)}>Tutup</button>
				</div>
			{/if}

			<form class="mt-3 flex flex-wrap gap-2" onsubmit={createKey}>
				<input class="app-input min-w-[180px] flex-1" placeholder="Label kunci" bind:value={newKeyLabel} />
				<input class="app-input min-w-[220px] flex-1 font-mono text-xs" placeholder="media_id (opsional)" bind:value={newKeyMedia} />
				<button class="app-btn app-btn-primary" type="submit" disabled={busy}>Buat Kunci</button>
			</form>

			{#if keys.length > 0}
				<ul class="mt-3 divide-y">
					{#each keys as k (k.id)}
						<li class="flex items-center justify-between gap-2 py-2.5">
							<div>
								<p class="text-sm font-medium">{k.label}</p>
								<p class="text-xs text-muted-foreground">{k.status}{#if k.last_used_at} · dipakai {dt(k.last_used_at)}{/if}</p>
							</div>
							<div class="flex gap-1">
								<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label="Rotasi kunci" onclick={() => rotateKey(k.id)}><RefreshCw size={14} /></button>
								<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label="Cabut kunci" onclick={() => revokeKey(k.id)} disabled={k.status === 'revoked'}><Trash2 size={14} /></button>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</div>

		<!-- RSS feeds -->
		<div class="app-card mt-4 p-4">
			<div class="flex items-center gap-2">
				<Rss size={16} />
				<h2 class="text-sm font-semibold">Sindikasi Konten (RSS)</h2>
			</div>
			{#if feeds.length === 0}
				<p class="mt-2 text-sm text-muted-foreground">Belum ada feed RSS. Feed dibuat oleh admin untuk media Anda.</p>
			{:else}
				<ul class="mt-3 divide-y">
					{#each feeds as f (f.id)}
						<li class="py-2.5">
							<div class="flex items-center justify-between gap-2">
								<div class="min-w-0">
									<p class="text-sm font-medium">{f.name ?? f.media_name ?? 'Feed RSS'}</p>
									<p class="truncate font-mono text-[11px] text-muted-foreground">{f.feed_url}</p>
									<p class="text-[11px] text-muted-foreground">Status pasang: {f.install_status}{#if f.install_error} · {f.install_error.slice(0, 100)}{/if}</p>
								</div>
								<div class="flex shrink-0 gap-1">
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" onclick={() => copy('URL feed', f.feed_url)}><Copy size={12} /></button>
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" onclick={() => verifyFeed(f.id)}><Check size={12} /> Verifikasi</button>
									<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label="Rotasi token" onclick={() => rotateFeed(f.id)}><RefreshCw size={14} /></button>
								</div>
							</div>
						</li>
					{/each}
				</ul>
			{/if}
		</div>

		<!-- GSC -->
		<div class="app-card mt-4 p-4">
			<h2 class="text-sm font-semibold">Google Search Console</h2>
			{#if gsc?.connected}
				<p class="mt-1 text-xs" style="color: var(--success);">Terhubung ke {gsc.connections?.[0]?.email ?? 'akun Google'}.</p>
				<ul class="mt-2 space-y-1 text-xs text-muted-foreground">
					{#each gsc.properties ?? [] as p (p.id)}
						<li>{p.site_url} · {p.permission_level}</li>
					{/each}
				</ul>
			{:else}
				<p class="mt-1 text-xs text-muted-foreground">Hubungkan Search Console untuk melihat performa pencarian artikel.</p>
				<button class="app-btn app-btn-soft mt-2" onclick={connectGSC}>Hubungkan GSC</button>
			{/if}
		</div>
	{/if}
</section>
