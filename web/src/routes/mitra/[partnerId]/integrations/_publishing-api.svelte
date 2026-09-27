<script lang="ts">
	// Tab Web Custom — port PublishingApiSection app lama
	// (app/mitra/[partnerId]/[[...section]]/sections.tsx baris ~471): kunci API
	// publishing media Tulisio API + konfigurasi Custom Web Publishing
	// (Publisher Protocol v1) per media.
	import { onMount } from 'svelte';
	import {
		BookOpenText,
		Check,
		Code,
		Copy,
		ExternalLink,
		Globe,
		Info,
		Key,
		Plus,
		PlugZap,
		RefreshCw,
		Trash2,
		TriangleAlert,
		X
	} from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const PAGE_SIZE = 20;

	let keys = $state<any[]>([]);
	let media = $state<any[]>([]);
	let customWebMedia = $state<any[]>([]);
	let customWebForms = $state<Record<string, { endpoint_url: string; api_key: string }>>({});
	let customWebBusyId = $state<string | null>(null);
	let customWebNotice = $state<Record<string, { kind: 'ok' | 'error'; text: string }>>({});
	let error = $state('');
	let loading = $state(true);
	let busy = $state(false);
	let createOpen = $state(false);
	let form = $state({ label: '', media_id: '' });
	let revealed = $state<any>(null);
	let copied = $state<'key' | 'secret' | null>(null);
	let page = $state(1);
	let docsOpen = $state(false);
	let docTab = $state<'publishing' | 'protocol'>('publishing');
	let docsMarkdown = $state('');
	let docsUpdatedAt = $state<string | null>(null);
	let docsError = $state('');
	let docsLoading = $state(false);

	const totalPages = $derived(Math.max(1, Math.ceil(keys.length / PAGE_SIZE)));
	const safePage = $derived(Math.min(page, totalPages));
	const visibleKeys = $derived(keys.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE));

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		loading = true;
		error = '';
		try {
			const [keyRes, mediaRes] = await Promise.all([
				apiRequest('/v1/partner/publishing-api-keys'),
				apiRequest('/v1/partner/media?limit=500')
			]);
			if (!keyRes.ok) {
				error = await readApiError(keyRes, 'Gagal memuat API key.');
				return;
			}
			keys = (await keyRes.json()).keys ?? [];
			if (mediaRes.ok) {
				const all = (await mediaRes.json()).media ?? [];
				media = all.filter((m: any) => m.integration_type === 'tulisio_api' && m.status === 'active');
				customWebMedia = all.filter((m: any) => m.integration_type === 'custom_web' && m.status !== 'hidden');
				// Seed endpoint dari nilai tersimpan; API key selalu kosong (masked).
				const next = { ...customWebForms };
				for (const m of customWebMedia) {
					if (!(m.id in next))
						next[m.id] = { endpoint_url: m.custom_web_endpoint_url ?? '', api_key: '' };
				}
				customWebForms = next;
			}
		} finally {
			loading = false;
		}
	}

	async function create(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const body: Record<string, unknown> = { label: form.label.trim() || 'Default' };
			if (form.media_id) body.media_id = form.media_id;
			const res = await apiRequest('/v1/partner/publishing-api-keys', {
				method: 'POST',
				body: JSON.stringify(body)
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal membuat API key.');
				return;
			}
			const data = await res.json();
			createOpen = false;
			form = { label: '', media_id: '' };
			copied = null;
			revealed = data;
			await load();
		} finally {
			busy = false;
		}
	}

	async function rotate(key: any) {
		if (!confirm('Rotasi API key ini? Kredensial lama langsung tidak berlaku.')) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/publishing-api-keys/${key.id}`, { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, 'Gagal merotasi API key.');
				return;
			}
			copied = null;
			revealed = await res.json();
			await load();
		} finally {
			busy = false;
		}
	}

	async function revoke(key: any) {
		if (!confirm('Cabut API key ini?')) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/publishing-api-keys/${key.id}`, { method: 'DELETE' });
			if (!res.ok) {
				error = await readApiError(res, 'Gagal mencabut API key.');
				return;
			}
			await load();
		} finally {
			busy = false;
		}
	}

	/* ─── Custom Web Publishing (Publisher Protocol v1) ─────────────── */

	function updateCustomWebForm(mediaId: string, patch: Partial<{ endpoint_url: string; api_key: string }>) {
		customWebForms = { ...customWebForms, [mediaId]: { ...customWebForms[mediaId], ...patch } };
	}

	function pushCustomWebNotice(mediaId: string, kind: 'ok' | 'error', text: string) {
		customWebNotice = { ...customWebNotice, [mediaId]: { kind, text } };
	}

	async function saveCustomWeb(item: any) {
		const formValue = customWebForms[item.id];
		const endpoint = (formValue?.endpoint_url ?? '').trim();
		if (!endpoint) {
			pushCustomWebNotice(item.id, 'error', 'Endpoint URL wajib diisi.');
			return;
		}
		customWebBusyId = item.id;
		pushCustomWebNotice(item.id, 'ok', '');
		try {
			const body: Record<string, string> = { endpoint_url: endpoint };
			if (formValue?.api_key.trim()) body.api_key = formValue.api_key.trim();
			const res = await apiRequest(`/v1/partner/media/${item.id}/custom-web`, {
				method: 'PATCH',
				body: JSON.stringify(body)
			});
			if (!res.ok) {
				pushCustomWebNotice(item.id, 'error', await readApiError(res, 'Gagal menyimpan konfigurasi.'));
				return;
			}
			pushCustomWebNotice(item.id, 'ok', 'Konfigurasi tersimpan. Jalankan Test Connection untuk memverifikasi.');
			await load();
		} finally {
			customWebBusyId = null;
		}
	}

	async function testCustomWeb(item: any) {
		customWebBusyId = item.id;
		pushCustomWebNotice(item.id, 'ok', '');
		try {
			const res = await apiRequest(`/v1/partner/media/${item.id}/custom-web/test-connection`, {
				method: 'POST'
			});
			if (!res.ok) {
				pushCustomWebNotice(item.id, 'error', await readApiError(res, 'Uji koneksi gagal.'));
				return;
			}
			const result = await res.json();
			pushCustomWebNotice(
				item.id,
				result.status === 'connected' ? 'ok' : 'error',
				result.message || (result.status === 'connected' ? 'Endpoint terhubung.' : 'Gagal terhubung.')
			);
			await load();
		} finally {
			customWebBusyId = null;
		}
	}

	async function regenerateCustomWebKey(item: any) {
		if (!confirm('Buat API Key baru? Key lama tidak berlaku — perbarui konfigurasi di website Anda.')) return;
		customWebBusyId = item.id;
		pushCustomWebNotice(item.id, 'ok', '');
		try {
			const res = await apiRequest(`/v1/partner/media/${item.id}/custom-web`, { method: 'POST' });
			if (!res.ok) {
				pushCustomWebNotice(item.id, 'error', await readApiError(res, 'Gagal membuat API Key baru.'));
				return;
			}
			const result = await res.json();
			pushCustomWebNotice(
				item.id,
				'ok',
				result.message ?? 'API Key baru telah dibuat. Perbarui konfigurasi di website Anda.'
			);
			updateCustomWebForm(item.id, { api_key: '' });
			await load();
		} finally {
			customWebBusyId = null;
		}
	}

	async function copy(label: string, value: string) {
		try {
			await navigator.clipboard.writeText(value);
			copied = label === 'API key' ? 'key' : 'secret';
			window.setTimeout(() => (copied = null), 1500);
		} catch {
			error = 'Tidak dapat menyalin ke clipboard.';
		}
	}

	/* ─── Panduan teknis (GET /publishing-api/docs) ─────────────────── */

	async function openDocs(doc: 'publishing' | 'protocol' = docTab) {
		docsOpen = true;
		docTab = doc;
		docsLoading = true;
		docsError = '';
		try {
			const res = await apiRequest(`/v1/partner/publishing-api/docs?doc=${doc}`);
			if (!res.ok) {
				docsMarkdown = '';
				docsError = await readApiError(res, 'Dokumen belum tersedia di server.');
				return;
			}
			const data = await res.json();
			docsMarkdown = data.markdown ?? '';
			docsUpdatedAt = data.updated_at ?? null;
		} finally {
			docsLoading = false;
		}
	}

	const dt = (v: string | null) =>
		v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—';

	const customWebStatusLabel = (status: string) =>
		status === 'connected'
			? '✓ Terhubung'
			: status === 'failed'
				? '✕ Gagal Terhubung'
				: status === 'testing'
					? 'Sedang diuji'
					: 'Belum Terhubung';
	const customWebStatusColor = (status: string) =>
		status === 'connected'
			? 'var(--success)'
			: status === 'failed'
				? 'var(--destructive)'
				: status === 'testing'
					? 'var(--warning)'
					: 'var(--muted-foreground)';
</script>

<div class="app-card mt-4 overflow-x-auto">
	<div class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3">
		<div class="flex items-center gap-2">
			<Key size={16} />
			<h2 class="text-sm font-semibold">Publishing API</h2>
			<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--secondary); color: var(--muted-foreground);">{keys.length} kunci</span>
		</div>
		<div class="flex flex-wrap gap-2">
			<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={() => openDocs('publishing')}>
				<BookOpenText size={13} /> Dokumentasi
			</button>
			<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" type="button" onclick={() => (createOpen = true)}>
				<Plus size={13} /> Buat API key
			</button>
		</div>
	</div>

	<p class="px-4 pt-3 text-xs text-muted-foreground">
		Kelola kunci API dan konfigurasi koneksi website custom Anda. Panduan teknis lengkap ada di halaman Dokumentasi.
		Kunci terbatas pada partner ini — untuk media <strong>Tulisio API</strong> yang statusnya aktif.
	</p>

	{#if error}<p class="px-4 pt-2 text-sm" style="color: var(--destructive);">{error}</p>{/if}

	{#if revealed}
		<div class="mx-4 mt-3 rounded-lg border p-3" style="border-color: var(--warning);">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<p class="text-sm font-semibold" style="color: var(--warning);">Kredensial baru — simpan sekarang</p>
				<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => (revealed = null)}><X size={12} /> Tutup</button>
			</div>
			<p class="mt-1 text-xs text-muted-foreground">
				{revealed.note ?? 'api_key dan secret hanya ditampilkan sekali ini. Salin ke konfigurasi adapter website Anda.'}
			</p>
			<div class="mt-3 space-y-2">
				<div>
					<span class="text-[11px] font-medium text-muted-foreground">API Key</span>
					<div class="mt-1 flex gap-2">
						<input class="app-input w-full font-mono text-xs" readonly value={revealed.api_key} />
						<button class="app-btn app-btn-soft shrink-0" type="button" onclick={() => copy('API key', revealed.api_key)}>
							{#if copied === 'key'}<Check size={13} /> Tersalin{:else}<Copy size={13} /> Salin{/if}
						</button>
					</div>
				</div>
				<div>
					<span class="text-[11px] font-medium text-muted-foreground">Secret (untuk verifikasi signature)</span>
					<div class="mt-1 flex gap-2">
						<input class="app-input w-full font-mono text-xs" readonly value={revealed.secret} />
						<button class="app-btn app-btn-soft shrink-0" type="button" onclick={() => copy('Secret', revealed.secret)}>
							{#if copied === 'secret'}<Check size={13} /> Tersalin{:else}<Copy size={13} /> Salin{/if}
						</button>
					</div>
				</div>
			</div>
		</div>
	{/if}

	{#if loading && keys.length === 0}
		<p class="px-4 py-8 text-center text-sm text-muted-foreground">Memuat kunci API…</p>
	{:else if keys.length === 0}
		<div class="px-4 py-8 text-center">
			<Code size={26} class="mx-auto" style="color: var(--muted-foreground);" />
			<p class="mt-2 text-sm text-muted-foreground">Belum ada API key. Buat kunci untuk media Tauthub Publishing API Anda.</p>
		</div>
	{:else}
		<table class="w-full text-sm">
			<thead>
				<tr class="text-left text-xs text-muted-foreground">
					<th class="px-4 py-2.5 font-medium">Label</th>
					<th class="px-4 py-2.5 font-medium">Media</th>
					<th class="px-4 py-2.5 font-medium">Status</th>
					<th class="px-4 py-2.5 font-medium">Terakhir dipakai</th>
					<th class="px-4 py-2.5 text-right font-medium">Aksi</th>
				</tr>
			</thead>
			<tbody>
				{#each visibleKeys as row (row.id)}
					<tr class="border-t">
						<td class="px-4 py-2.5">
							<div class="flex items-center gap-2">
								<Code size={16} />
								<div>
									<p class="font-medium">{row.label}</p>
									<p class="text-xs text-muted-foreground">{String(row.id).slice(0, 8)}</p>
								</div>
							</div>
						</td>
						<td class="px-4 py-2.5">{row.media_name ?? 'Semua media'}</td>
						<td class="px-4 py-2.5">
							<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: {row.status === 'active' ? 'var(--success-soft)' : 'var(--secondary)'}; color: {row.status === 'active' ? 'var(--success)' : 'var(--muted-foreground)'};">{row.status}</span>
						</td>
						<td class="px-4 py-2.5 text-xs text-muted-foreground">{row.last_used_at ? dt(row.last_used_at) : '—'}</td>
						<td class="px-4 py-2.5">
							{#if row.status === 'active'}
								<div class="flex justify-end gap-2">
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" disabled={busy} onclick={() => rotate(row)}><RefreshCw size={12} /> Rotasi</button>
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" style="color: var(--destructive);" disabled={busy} onclick={() => revoke(row)}><Trash2 size={12} /> Cabut</button>
								</div>
							{:else}
								<p class="text-right text-xs text-muted-foreground">Dicabut</p>
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
		{#if keys.length > PAGE_SIZE}
			<div class="flex flex-wrap items-center justify-between gap-2 border-t px-4 py-3 text-xs text-muted-foreground">
				<span>Menampilkan {(safePage - 1) * PAGE_SIZE + 1}–{Math.min(safePage * PAGE_SIZE, keys.length)} dari {keys.length}</span>
				<div class="flex items-center gap-2">
					<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" disabled={safePage <= 1} onclick={() => (page = safePage - 1)}>‹ Sebelumnya</button>
					<span>Halaman {safePage} dari {totalPages}</span>
					<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" disabled={safePage >= totalPages} onclick={() => (page = safePage + 1)}>Berikutnya ›</button>
				</div>
			</div>
		{/if}
	{/if}
</div>

<div class="app-card mt-4 p-4">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<div class="flex items-center gap-2">
			<ExternalLink size={16} />
			<h2 class="text-sm font-semibold">Website Tanpa Endpoint Penerima</h2>
		</div>
		<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--secondary); color: var(--muted-foreground);">Alternatif push API</span>
	</div>
	<p class="mt-2 text-xs text-muted-foreground">
		Website statis atau yang tidak bisa membuka endpoint penerima dapat menarik artikel secara terstruktur lewat
		JSON Pull feed — script atau CI Anda yang mengatur jadwal menarik dan cara menampilkannya.
	</p>
	<button class="mt-3 inline-flex items-center gap-1.5 text-xs font-semibold" style="color: var(--accent);" type="button" onclick={() => openDocs('publishing')}>
		<BookOpenText size={14} /> Baca panduan teknis di Dokumentasi
	</button>
</div>

<div class="app-card mt-4 p-4">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<div class="flex items-center gap-2">
			<PlugZap size={16} />
			<h2 class="text-sm font-semibold">Custom Web Publishing</h2>
		</div>
		<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--secondary); color: var(--muted-foreground);">Publisher Protocol v1</span>
	</div>
	<p class="mt-2 text-xs text-muted-foreground">
		Untuk media ber-platform <strong>Custom Web</strong>: artikel Tauthub terkirim otomatis ke website Anda setelah
		koneksi berhasil. Cukup isi endpoint URL dan API key di bawah, lalu klik Test Connection.
		<button class="font-semibold underline" type="button" style="color: var(--accent);" onclick={() => openDocs('protocol')}>Baca panduan teknis di Dokumentasi</button>.
	</p>

	{#if loading && customWebMedia.length === 0}
		<p class="mt-3 text-xs text-muted-foreground">Memuat…</p>
	{:else if customWebMedia.length === 0}
		<div class="mt-4 py-6 text-center">
			<Globe size={26} class="mx-auto" style="color: var(--muted-foreground);" />
			<p class="mt-2 text-sm text-muted-foreground">
				Belum ada media Custom Web. Pilih platform Custom Web saat menambah media, lalu konfigurasikan di sini.
			</p>
		</div>
	{:else}
		{#each customWebMedia as item (item.id)}
			{@const formValue = customWebForms[item.id] ?? { endpoint_url: item.custom_web_endpoint_url ?? '', api_key: '' }}
			{@const notice = customWebNotice[item.id]}
			<div class="mt-3 rounded-lg border p-3">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<div class="flex items-center gap-2">
						<Globe size={15} />
						<p class="text-sm font-bold">{item.name}</p>
					</div>
					<div class="flex flex-wrap items-center gap-2">
						<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--secondary); color: {customWebStatusColor(item.custom_web_status)};">{customWebStatusLabel(item.custom_web_status)}</span>
						{#if item.custom_web_last_test_at}
							<span class="text-xs text-muted-foreground">Terakhir dicek: {dt(item.custom_web_last_test_at)}</span>
						{/if}
					</div>
				</div>

				<div class="mt-3 space-y-2">
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Endpoint URL</span>
						<input
							class="app-input mt-1 w-full"
							type="url"
							placeholder="https://mitra.com/v1/tulisio/publish"
							value={formValue.endpoint_url}
							oninput={(e) => updateCustomWebForm(item.id, { endpoint_url: e.currentTarget.value })}
						/>
						<span class="mt-1 block text-[11px] text-muted-foreground">Production wajib HTTPS publik. Contoh: https://mitra.com/v1/tulisio/publish</span>
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">API Key</span>
						<input
							class="app-input mt-1 w-full"
							type="password"
							placeholder={item.custom_web_api_key_configured ? '••••••••••••••••' : 'tli_...'}
							value={formValue.api_key}
							oninput={(e) => updateCustomWebForm(item.id, { api_key: e.currentTarget.value })}
						/>
						<span class="mt-1 block text-[11px] text-muted-foreground">
							{item.custom_web_api_key_configured
								? 'API Key tersimpan aman (masked). Isi hanya jika ingin mengganti.'
								: 'API Key dari endpoint Custom Web Anda.'}
						</span>
					</label>

					<div class="flex flex-wrap items-center gap-2">
						<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" type="button" disabled={customWebBusyId === item.id} onclick={() => saveCustomWeb(item)}>
							{customWebBusyId === item.id ? 'Menyimpan…' : 'Simpan'}
						</button>
						<button
							class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
							type="button"
							disabled={customWebBusyId === item.id || !item.custom_web_endpoint_url}
							onclick={() => testCustomWeb(item)}
						>
							{customWebBusyId === item.id ? 'Menguji koneksi…' : 'Test Connection'}
						</button>
						{#if item.custom_web_api_key_configured}
							<button
								class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
								type="button"
								disabled={customWebBusyId === item.id}
								onclick={() => regenerateCustomWebKey(item)}
							>Regenerate API Key</button>
						{/if}
					</div>

					{#if notice?.text}
						<p class="flex items-start gap-1.5 text-xs" style="color: {notice.kind === 'ok' ? 'var(--success)' : 'var(--destructive)'};">
							{#if notice.kind === 'ok'}<Check size={13} class="mt-0.5 shrink-0" />{:else}<TriangleAlert size={13} class="mt-0.5 shrink-0" />{/if}
							<span>{notice.text}</span>
						</p>
					{/if}

					{#if !item.custom_web_endpoint_url}
						<p class="flex items-start gap-1.5 text-[11px] text-muted-foreground">
							<Info size={12} class="mt-0.5 shrink-0" />
							<span>Isi Endpoint URL + API Key lalu Simpan untuk mengaktifkan distribusi ke website custom ini.</span>
						</p>
					{/if}
				</div>
			</div>
		{/each}
	{/if}
</div>

{#if createOpen}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card my-8 w-full max-w-lg p-5" aria-label="Buat API key">
			<h2 class="text-lg font-semibold">Buat API key</h2>
			{#if error}<p class="mt-2 text-sm" style="color: var(--destructive);">{error}</p>{/if}
			<form class="mt-4 space-y-3" onsubmit={create}>
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">Label</span>
					<input class="app-input mt-1 w-full" required placeholder="Contoh: Website utama" bind:value={form.label} />
				</label>
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">Media (opsional)</span>
					<select class="app-input mt-1 w-full" bind:value={form.media_id}>
						<option value="">Semua media Tauthub API</option>
						{#each media as m (m.id)}
							<option value={m.id}>{m.name}</option>
						{/each}
					</select>
				</label>
				<p class="text-[11px] text-muted-foreground">
					Kunci dibatasi ke partner ini. Untuk media tertentu, artikel ke media tersebut memakai kunci ini
					(fallback ke kunci partner bila tidak ada).
				</p>
				<div class="flex justify-end gap-2 border-t pt-3">
					<button class="app-btn app-btn-soft" type="button" onclick={() => (createOpen = false)}>Batal</button>
					<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? 'Membuat…' : 'Buat kunci'}</button>
				</div>
			</form>
		</dialog>
	</div>
{/if}

{#if docsOpen}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card my-8 w-full max-w-3xl p-5" aria-label="Dokumentasi API">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<h2 class="text-lg font-semibold">Dokumentasi API</h2>
				<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => (docsOpen = false)}><X size={12} /> Tutup</button>
			</div>
			<div class="mt-3 flex flex-wrap gap-2">
				<button
					class="app-btn !min-h-8 !px-3 !text-xs"
					class:app-btn-primary={docTab === 'publishing'}
					class:app-btn-soft={docTab !== 'publishing'}
					type="button"
					onclick={() => openDocs('publishing')}
				>Publishing API</button>
				<button
					class="app-btn !min-h-8 !px-3 !text-xs"
					class:app-btn-primary={docTab === 'protocol'}
					class:app-btn-soft={docTab !== 'protocol'}
					type="button"
					onclick={() => openDocs('protocol')}
				>Publisher Protocol v1</button>
			</div>

			{#if docsLoading}
				<p class="mt-3 text-sm text-muted-foreground">Memuat dokumen…</p>
			{:else if docsError}
				<p class="mt-3 text-sm" style="color: var(--destructive);">{docsError}</p>
			{:else}
				{#if docsUpdatedAt}<p class="mt-2 text-[11px] text-muted-foreground">Diperbarui {dt(docsUpdatedAt)}</p>{/if}
				<pre class="mt-2 max-h-[60vh] overflow-auto whitespace-pre-wrap rounded-lg border p-3 font-mono text-[12px] leading-relaxed">{docsMarkdown}</pre>
			{/if}
		</dialog>
	</div>
{/if}
