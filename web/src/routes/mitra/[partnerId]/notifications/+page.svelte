<script lang="ts">
	// Notifikasi mitra — port section "Notifikasi": daftar notifikasi in-app
	// (filter semua/belum dibaca/sudah dibaca), tandai dibaca, preferensi email
	// tambahan & WhatsApp, serta webhook notifikasi.
	import { onMount } from 'svelte';
	import { Bell, Check, Plus, Trash2, Webhook } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	let loading = $state(true);
	let items = $state<any[]>([]);
	let unread = $state(0);
	let statusFilter = $state('');
	let prefs = $state({ email_alt1: '', email_alt2: '', wa_number: '', wa_consent: false });
	let webhooks = $state<any[]>([]);
	let newWebhook = $state({ url: '', secret: '' });
	let notice = $state('');
	let error = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([load(), loadPrefs(), loadWebhooks()]);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		const params = new URLSearchParams({ limit: '100' });
		if (statusFilter) params.set('status', statusFilter);
		const res = await apiRequest(`/v1/partner/notifications?${params}`);
		if (res.ok) {
			const data = await res.json();
			items = data.notifications ?? [];
			unread = data.unread ?? 0;
		}
	}

	async function loadPrefs() {
		const res = await apiRequest('/v1/partner/notification-preferences');
		if (res.ok) prefs = await res.json();
	}

	async function loadWebhooks() {
		const res = await apiRequest('/v1/partner/notification-webhooks');
		if (res.ok) webhooks = (await res.json()).webhooks ?? [];
	}

	async function markAll() {
		const res = await apiRequest('/v1/partner/notifications', { method: 'POST' });
		if (res.ok) {
			notice = 'Semua notifikasi ditandai sudah dibaca.';
			await load();
		}
	}

	async function markOne(id: string) {
		const res = await apiRequest(`/v1/partner/notifications/${id}/read`, { method: 'POST' });
		if (res.ok) await load();
	}

	async function savePrefs(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		const res = await apiRequest('/v1/partner/notification-preferences', {
			method: 'PUT',
			body: JSON.stringify(prefs)
		});
		if (!res.ok) {
			error = await readApiError(res, "Permintaan gagal.");
			return;
		}
		notice = 'Preferensi notifikasi tersimpan.';
		await loadPrefs();
	}

	async function addWebhook(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		const payload: Record<string, unknown> = { url: newWebhook.url.trim() };
		if (newWebhook.secret.trim()) payload.secret = newWebhook.secret.trim();
		const res = await apiRequest('/v1/partner/notification-webhooks', {
			method: 'POST',
			body: JSON.stringify(payload)
		});
		if (!res.ok) {
			error = await readApiError(res, "Permintaan gagal.");
			return;
		}
		notice = 'Webhook ditambahkan.';
		newWebhook = { url: '', secret: '' };
		await loadWebhooks();
	}

	async function deleteWebhook(id: string) {
		if (!confirm('Hapus webhook ini?')) return;
		const res = await apiRequest(`/v1/partner/notification-webhooks/${id}`, { method: 'DELETE' });
		if (res.ok) {
			notice = 'Webhook dihapus.';
			await loadWebhooks();
		}
	}

	const dt = (v: string | null) => (v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—');
</script>

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Notifikasi</h1>
			<p class="mt-1 text-sm text-muted-foreground">{unread} notifikasi belum dibaca.</p>
		</div>
		{#if unread > 0}
			<button class="app-btn app-btn-soft" onclick={markAll}><Check size={15} /> Tandai semua dibaca</button>
		{/if}
	</header>

	{#if notice}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}
	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	<div class="mt-4 flex gap-2">
		{#each [['', 'Semua'], ['unread', 'Belum dibaca'], ['read', 'Sudah dibaca']] as [value, label] (value)}
			<button
				class="app-btn !min-h-8 !px-3 !text-xs"
				class:app-btn-primary={statusFilter === value}
				class:app-btn-soft={statusFilter !== value}
				onclick={() => { statusFilter = value; void load(); }}
			>{label}</button>
		{/each}
	</div>

	<div class="app-card mt-3 divide-y">
		{#if loading}
			<p class="px-4 py-8 text-center text-sm text-muted-foreground">Memuat notifikasi…</p>
		{:else if items.length === 0}
			<div class="px-4 py-10 text-center">
				<Bell size={26} class="mx-auto text-muted-foreground" />
				<p class="mt-3 text-sm text-muted-foreground">Tidak ada notifikasi.</p>
			</div>
		{:else}
			{#each items as n (n.id)}
				<button class="flex w-full items-start gap-3 px-4 py-3 text-left" onclick={() => markOne(n.id)}>
					<span class="mt-1.5 h-2 w-2 shrink-0 rounded-full" style="background-color: {n.read_at ? 'transparent' : 'var(--primary)'};"></span>
					<span class="min-w-0 flex-1">
						<span class="flex flex-wrap items-center gap-2">
							<strong class="text-sm">{n.title}</strong>
							{#if n.expired_at}<span class="text-[11px] text-muted-foreground">Kedaluwarsa</span>{/if}
						</span>
						{#if n.body}<span class="mt-0.5 block text-xs text-muted-foreground">{n.body}</span>{/if}
						{#if n.link}
							<a class="mt-1 inline-block text-xs font-medium" style="color: var(--primary);" href={n.link}>Buka detail</a>
						{/if}
					</span>
					<span class="shrink-0 text-[11px] text-muted-foreground">{dt(n.created_at)}</span>
				</button>
			{/each}
		{/if}
	</div>

	<form class="app-card mt-4 p-4" onsubmit={savePrefs}>
		<h2 class="text-sm font-semibold">Saluran Notifikasi</h2>
		<p class="mt-0.5 text-xs text-muted-foreground">Email tambahan dan WhatsApp untuk menerima pemberitahuan publikasi.</p>
		<div class="mt-3 grid gap-3 sm:grid-cols-2">
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Email tambahan 1</span>
				<input class="app-input mt-1 w-full" type="email" bind:value={prefs.email_alt1} />
			</label>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Email tambahan 2</span>
				<input class="app-input mt-1 w-full" type="email" bind:value={prefs.email_alt2} />
			</label>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Nomor WhatsApp</span>
				<input class="app-input mt-1 w-full" placeholder="62812xxxxxxx" bind:value={prefs.wa_number} />
			</label>
			<label class="mt-6 flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={prefs.wa_consent} />
				Setuju menerima notifikasi WhatsApp
			</label>
		</div>
		<div class="mt-3 flex justify-end">
			<button class="app-btn app-btn-primary" type="submit">Simpan Preferensi</button>
		</div>
	</form>

	<div class="app-card mt-4 p-4">
		<div class="flex items-center gap-2">
			<Webhook size={16} />
			<h2 class="text-sm font-semibold">Webhook Notifikasi</h2>
		</div>
		<p class="mt-0.5 text-xs text-muted-foreground">Kirim event publikasi ke sistem Anda sendiri.</p>

		{#if webhooks.length > 0}
			<ul class="mt-3 space-y-2">
				{#each webhooks as w (w.id)}
					<li class="flex items-center justify-between gap-2 rounded-lg border px-3 py-2">
						<span class="min-w-0">
							<span class="block truncate font-mono text-xs">{w.url}</span>
							<span class="text-[11px] text-muted-foreground">{w.active ? 'Aktif' : 'Nonaktif'}</span>
						</span>
						<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label="Hapus webhook" onclick={() => deleteWebhook(w.id)}><Trash2 size={14} /></button>
					</li>
				{/each}
			</ul>
		{/if}

		<form class="mt-3 space-y-2" onsubmit={addWebhook}>
			<input class="app-input w-full" required placeholder="https://sistem-anda.example/webhook" bind:value={newWebhook.url} />
			<input class="app-input w-full" placeholder="Secret (min. 16 karakter, opsional)" bind:value={newWebhook.secret} />
			<button class="app-btn app-btn-soft" type="submit"><Plus size={14} /> Tambah Webhook</button>
		</form>
	</div>
</section>
