<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Tab Web Custom — port PublishingApiSection app lama
	// (app/mitra/[partnerId]/[[...section]]/sections.tsx baris ~471): kunci API
	// publishing media Tulisio API + konfigurasi Custom Web Publishing
	// (Publisher Protocol v1) per media.
	import { onMount } from 'svelte';
	import { Check, Copy, Plus, X } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import Status from './_status.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	let { onchange }: { onchange?: () => void } = $props();

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
				error = await readApiError(keyRes, tr('Gagal memuat API key.'));
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
			onchange?.();
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
				error = await readApiError(res, tr('Gagal membuat API key.'));
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
				error = await readApiError(res, tr('Gagal merotasi API key.'));
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
				error = await readApiError(res, tr('Gagal mencabut API key.'));
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
			pushCustomWebNotice(item.id, 'error', tr('Endpoint URL wajib diisi.'));
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
				pushCustomWebNotice(item.id, 'error', await readApiError(res, tr('Gagal menyimpan konfigurasi.')));
				return;
			}
			pushCustomWebNotice(item.id, 'ok', tr('Konfigurasi tersimpan. Jalankan Uji koneksi untuk memverifikasi.'));
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
				pushCustomWebNotice(item.id, 'error', await readApiError(res, tr('Uji koneksi gagal.')));
				return;
			}
			const result = await res.json();
			pushCustomWebNotice(
				item.id,
				result.status === 'connected' ? 'ok' : 'error',
				result.message || (result.status === 'connected' ? tr('Endpoint terhubung.') : tr('Gagal terhubung.'))
			);
			await load();
		} finally {
			customWebBusyId = null;
		}
	}

	async function regenerateCustomWebKey(item: any) {
		if (!confirm('Buat API key baru? Key lama tidak berlaku — perbarui konfigurasi di website Anda.')) return;
		customWebBusyId = item.id;
		pushCustomWebNotice(item.id, 'ok', '');
		try {
			const res = await apiRequest(`/v1/partner/media/${item.id}/custom-web`, { method: 'POST' });
			if (!res.ok) {
				pushCustomWebNotice(item.id, 'error', await readApiError(res, tr('Gagal membuat API key baru.')));
				return;
			}
			const result = await res.json();
			pushCustomWebNotice(
				item.id,
				'ok',
				result.message ?? tr('API key baru telah dibuat. Perbarui konfigurasi di website Anda.')
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
			error = tr('Tidak dapat menyalin ke clipboard.');
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
				docsError = await readApiError(res, tr('Dokumen belum tersedia di server.'));
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

	const customWebTone = (status: string): 'ok' | 'error' | 'warn' | 'idle' =>
		status === 'connected' ? 'ok' : status === 'failed' ? 'error' : status === 'testing' ? 'warn' : 'idle';
	const customWebStatusLabel = (status: string) =>
		status === 'connected'
			? tr('Terhubung')
			: status === 'failed'
				? tr('Gagal terhubung')
				: status === 'testing'
					? tr('Sedang diuji')
					: tr('Belum terhubung');
	const keyStatusLabel = (status: string) => (status === 'active' ? tr('Aktif') : status === 'revoked' ? tr('Dicabut') : status);
</script>

<div class="app-card int-card overflow-hidden">
	<div class="int-card__head int-card__head--split">
		<div>
			<h2 class="int-card__title">Publishing API <span class="int-card__caption">{keys.length} {tr('kunci')}</span></h2>
			<p class="int-card__desc">{tr('Kunci untuk menarik atau menerima artikel lewat Tauthub Publishing API. Berlaku untuk mitra ini, pada media Tulisio API yang aktif')}</p>
		</div>
		<div class="int-card__actions">
			<button class="app-btn app-btn-soft int-btn" type="button" onclick={() => openDocs('publishing')}>{tr('Panduan')}</button>
			<button class="app-btn app-btn-primary int-btn" type="button" onclick={() => (createOpen = true)}><Plus size={16} strokeWidth={1.5} />{tr('Buat API key')}</button>
		</div>
	</div>

	{#if error}<p class="int-msg int-msg--error" style="padding-top: 12px;">{error}</p>{/if}

	{#if revealed}
		<div class="int-reveal" style="margin-top: 16px;">
			<div class="int-reveal__head">
				<p class="int-reveal__title">{tr('Kredensial baru, simpan sekarang')}</p>
				<button class="app-btn app-btn-soft int-btn" type="button" onclick={() => (revealed = null)}><X size={14} strokeWidth={1.5} />{tr('Tutup')}</button>
			</div>
			<p class="int-item__note" style="margin-top: 4px;">
				{revealed.note ?? tr('API key dan secret hanya ditampilkan sekali ini. Salin ke konfigurasi adapter website Anda.')}
			</p>
			<div class="int-form" style="margin-top: 12px; gap: 12px;">
				<div class="int-field">
					<span class="int-field__label">API key</span>
					<div class="int-reveal__row">
						<input class="app-input" readonly value={revealed.api_key} />
						<button class="app-btn app-btn-soft int-btn shrink-0" type="button" onclick={() => copy('API key', revealed.api_key)}>
							{#if copied === 'key'}<Check size={14} strokeWidth={1.5} />{tr('Tersalin')}{:else}<Copy size={14} strokeWidth={1.5} />{tr('Salin')}{/if}
						</button>
					</div>
				</div>
				<div class="int-field">
					<span class="int-field__label">Secret <span class="int-meta">({tr('untuk verifikasi signature')})</span></span>
					<div class="int-reveal__row">
						<input class="app-input" readonly value={revealed.secret} />
						<button class="app-btn app-btn-soft int-btn shrink-0" type="button" onclick={() => copy('Secret', revealed.secret)}>
							{#if copied === 'secret'}<Check size={14} strokeWidth={1.5} />{tr('Tersalin')}{:else}<Copy size={14} strokeWidth={1.5} />{tr('Salin')}{/if}
						</button>
					</div>
				</div>
			</div>
		</div>
	{/if}

	{#if loading && keys.length === 0}
		<p class="int-empty">{tr('Memuat kunci API…')}</p>
	{:else if keys.length === 0}
		<div class="int-empty">
			<p>{tr('Belum ada API key. Buat kunci untuk media Tauthub Publishing API Anda')}</p>
		</div>
	{:else}
		<div class="overflow-x-auto">
			<table class="int-table">
				<thead>
					<tr>
						<th>Label</th>
						<th>{tr('Media')}</th>
						<th>Status</th>
						<th>{tr('Terakhir dipakai')}</th>
						<th class="is-right">{tr('Aksi')}</th>
					</tr>
				</thead>
				<tbody>
					{#each visibleKeys as row (row.id)}
						<tr>
							<td>
								<p class="font-medium">{row.label}</p>
								<p class="int-table__sub int-mono">{String(row.id).slice(0, 8)}</p>
							</td>
							<td>{row.media_name ?? tr('Semua media')}</td>
							<td><Status tone={row.status === 'active' ? 'ok' : 'idle'} label={keyStatusLabel(row.status)} /></td>
							<td><span class="int-meta">{row.last_used_at ? dt(row.last_used_at) : tr('Belum dipakai')}</span></td>
							<td>
								{#if row.status === 'active'}
									<div class="int-table__actions">
										<button class="app-btn app-btn-soft int-btn" type="button" disabled={busy} onclick={() => rotate(row)}>{tr('Rotasi')}</button>
										<button class="app-btn app-btn-soft int-btn int-btn--danger" type="button" disabled={busy} onclick={() => revoke(row)}>{tr('Cabut')}</button>
									</div>
								{:else}
									<p class="int-meta text-right">{tr('Dicabut')}</p>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
		{#if keys.length > PAGE_SIZE}
			<div class="int-pager">
				<span>{(safePage - 1) * PAGE_SIZE + 1}–{Math.min(safePage * PAGE_SIZE, keys.length)} {tr('dari')} {keys.length}</span>
				<div class="int-pager__nav">
					<button class="app-btn app-btn-soft" type="button" disabled={safePage <= 1} onclick={() => (page = safePage - 1)}>{tr('Sebelumnya')}</button>
					<span>{safePage} / {totalPages}</span>
					<button class="app-btn app-btn-soft" type="button" disabled={safePage >= totalPages} onclick={() => (page = safePage + 1)}>{tr('Berikutnya')}</button>
				</div>
			</div>
		{/if}
	{/if}
</div>

<div class="app-card int-card">
	<div class="int-card__head" style="padding-bottom: 20px;">
		<div>
			<h2 class="int-card__title">{tr('Website tanpa endpoint penerima')} <span class="int-card__caption">{tr('alternatif push API')}</span></h2>
			<p class="int-card__desc">{tr('Website statis atau yang tidak bisa membuka endpoint penerima dapat menarik artikel lewat JSON pull feed; script atau CI Anda yang mengatur jadwal dan tampilannya')}</p>
		</div>
		<button class="int-link" type="button" onclick={() => openDocs('publishing')}>{tr('Panduan Publishing API')}</button>
	</div>
</div>

<div class="app-card int-card overflow-hidden">
	<div class="int-card__head int-card__head--split">
		<div>
			<h2 class="int-card__title">Custom Web Publishing <span class="int-card__caption">Publisher Protocol v1</span></h2>
			<p class="int-card__desc">{tr('Untuk media berplatform Custom Web. Isi endpoint URL dan API key, simpan, lalu uji koneksi; setelah terhubung, artikel klien terkirim otomatis ke website Anda')}</p>
		</div>
		<button class="int-link" type="button" onclick={() => openDocs('protocol')}>{tr('Panduan Publisher Protocol')}</button>
	</div>

	{#if loading && customWebMedia.length === 0}
		<p class="int-empty">{tr('Memuat…')}</p>
	{:else if customWebMedia.length === 0}
		<div class="int-empty">
			<p>{tr('Belum ada media Custom Web. Pilih platform Custom Web saat menambah media, lalu konfigurasikan di sini')}</p>
		</div>
	{:else}
		<div class="int-list" style="padding-top: 20px;">
			{#each customWebMedia as item (item.id)}
				{@const formValue = customWebForms[item.id] ?? { endpoint_url: item.custom_web_endpoint_url ?? '', api_key: '' }}
				{@const notice = customWebNotice[item.id]}
				<div class="int-item">
					<div class="int-item__head">
						<p class="int-item__name">{item.name}</p>
						<div class="int-item__meta">
							<Status tone={customWebTone(item.custom_web_status)} label={customWebStatusLabel(item.custom_web_status)} />
							{#if item.custom_web_last_test_at}
								<span class="int-meta">{tr('Dicek')} {dt(item.custom_web_last_test_at)}</span>
							{/if}
						</div>
					</div>

					<div class="int-form mt-4">
						<label class="int-field">
							<span class="int-field__label">Endpoint URL</span>
							<input
								class="app-input"
								type="url"
								placeholder="https://mitra.com/v1/tulisio/publish"
								value={formValue.endpoint_url}
								oninput={(e) => updateCustomWebForm(item.id, { endpoint_url: e.currentTarget.value })}
							/>
							<span class="int-field__hint">{tr('Production wajib HTTPS publik')}</span>
						</label>
						<label class="int-field">
							<span class="int-field__label">API key</span>
							<input
								class="app-input"
								type="password"
								placeholder={item.custom_web_api_key_configured ? '••••••••••••••••' : 'tli_...'}
								value={formValue.api_key}
								oninput={(e) => updateCustomWebForm(item.id, { api_key: e.currentTarget.value })}
							/>
							<span class="int-field__hint">
								{item.custom_web_api_key_configured
									? tr('API key tersimpan (disamarkan); isi hanya jika ingin mengganti')
									: tr('API key dari endpoint Custom Web Anda')}
							</span>
						</label>
					</div>

					<div class="int-item__actions">
						<button class="app-btn app-btn-primary int-btn" type="button" disabled={customWebBusyId === item.id} onclick={() => saveCustomWeb(item)}>
							{customWebBusyId === item.id ? tr('Menyimpan…') : tr('Simpan')}
						</button>
						<button
							class="app-btn app-btn-soft int-btn"
							type="button"
							disabled={customWebBusyId === item.id || !item.custom_web_endpoint_url}
							onclick={() => testCustomWeb(item)}
						>
							{customWebBusyId === item.id ? tr('Menguji…') : tr('Uji koneksi')}
						</button>
						{#if item.custom_web_api_key_configured}
							<button class="app-btn app-btn-soft int-btn" type="button" disabled={customWebBusyId === item.id} onclick={() => regenerateCustomWebKey(item)}>{tr('Buat ulang API key')}</button>
						{/if}
					</div>

					{#if notice?.text}
						<p class="int-item__note" style="color: {notice.kind === 'ok' ? 'var(--success)' : 'var(--destructive)'};">{notice.text}</p>
					{/if}
					{#if !item.custom_web_endpoint_url}
						<p class="int-item__note">{tr('Isi endpoint URL dan API key lalu simpan untuk mengaktifkan distribusi ke website ini')}</p>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

{#if createOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card w-full max-w-lg p-6" aria-label={tr('Buat API key')}>
			<h2 class="int-dialog__title">{tr('Buat API key')}</h2>
			{#if error}<p class="mt-2 text-sm" style="color: var(--destructive);">{error}</p>{/if}
			<form class="int-form mt-5" onsubmit={create}>
				<label class="int-field">
					<span class="int-field__label">Label</span>
					<input class="app-input" required placeholder={tr('Contoh: Website utama')} bind:value={form.label} />
				</label>
				<label class="int-field">
					<span class="int-field__label">{tr('Media')} <span class="int-meta">({tr('opsional')})</span></span>
					<Select
						bind:value={form.media_id}
						ariaLabel={tr('Media')}
						placeholder={tr('Semua media Tauthub API')}
						minWidth={240}
						options={[
							{ value: '', label: tr('Semua media Tauthub API') },
							...media.map((m) => ({ value: m.id, label: m.name }))
						]}
					/>
					<span class="int-field__hint">{tr('Kunci dibatasi ke mitra ini. Bila dipilih media tertentu, artikel ke media itu memakai kunci ini, selebihnya memakai kunci mitra')}</span>
				</label>
				<div class="int-form__foot">
					<button class="app-btn app-btn-soft int-btn" type="button" onclick={() => (createOpen = false)}>{tr('Batal')}</button>
					<button class="app-btn app-btn-primary int-btn" type="submit" disabled={busy}>{busy ? tr('Membuat…') : tr('Buat kunci')}</button>
				</div>
			</form>
		</dialog>
	</div>
{/if}

{#if docsOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card w-full max-w-3xl p-6" aria-label={tr('Panduan teknis')}>
			<div class="int-dialog__head">
				<h2 class="int-dialog__title">{tr('Panduan teknis')}</h2>
				<button class="app-btn app-btn-soft int-btn" type="button" onclick={() => (docsOpen = false)}><X size={14} strokeWidth={1.5} />{tr('Tutup')}</button>
			</div>
			<div class="int-doc-tabs" role="tablist">
				<button class="int-doc-tab" class:int-doc-tab--active={docTab === 'publishing'} role="tab" aria-selected={docTab === 'publishing'} type="button" onclick={() => openDocs('publishing')}>Publishing API</button>
				<button class="int-doc-tab" class:int-doc-tab--active={docTab === 'protocol'} role="tab" aria-selected={docTab === 'protocol'} type="button" onclick={() => openDocs('protocol')}>Publisher Protocol v1</button>
			</div>

			{#if docsLoading}
				<p class="mt-3 text-sm text-muted-foreground">{tr('Memuat dokumen…')}</p>
			{:else if docsError}
				<p class="mt-3 text-sm" style="color: var(--destructive);">{docsError}</p>
			{:else}
				{#if docsUpdatedAt}<p class="int-meta mt-3">{tr('Diperbarui')} {dt(docsUpdatedAt)}</p>{/if}
				<pre class="int-docs">{docsMarkdown}</pre>
			{/if}
		</dialog>
	</div>
{/if}

<style>
	.int-doc-tabs {
		display: flex;
		gap: 4px;
		margin-top: 16px;
		border-bottom: 1px solid var(--border);
	}
	.int-doc-tab {
		height: 36px;
		margin-bottom: -1px;
		padding: 0 12px;
		border-bottom: 2px solid transparent;
		font-size: 13px;
		font-weight: 500;
		color: var(--muted-foreground);
		transition: color 160ms cubic-bezier(0.2, 0.8, 0.2, 1), border-color 160ms cubic-bezier(0.2, 0.8, 0.2, 1);
	}
	.int-doc-tab:hover {
		color: var(--foreground);
	}
	.int-doc-tab--active {
		color: var(--foreground);
		border-bottom-color: var(--primary);
	}
</style>
