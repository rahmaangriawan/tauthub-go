<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Tab WordPress dan EmDash (Koneksi CMS) — port ConnectionsSection app lama
	// (app/mitra/[partnerId]/[[...section]]/sections.tsx). Satu komponen untuk
	// dua provider (prop provider); daftar media diambil dari media mitra
	// ber-website_url.
	import { onMount, tick } from 'svelte';
	import { Plus } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import Status from './_status.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	let {
		provider = 'wordpress',
		onchange,
		highlight = '',
		onhighlighted
	}: { provider?: 'wordpress' | 'emdash'; onchange?: () => void; highlight?: string; onhighlighted?: () => void } = $props();
	const isEmdash = $derived(provider === 'emdash');
	const providerLabel = $derived(isEmdash ? 'EmDash' : 'WordPress');

	// Deep link dari Daftar Media (?media=<id>): dialog Tambah koneksi terbuka
	// dengan media itu terpilih. Catatan tampil bila media belum punya website URL.
	let highlightNote = $state('');
	// Baris koneksi yang disorot (media sudah punya koneksi).
	let highlightConnId = $state('');
	let highlightTimer: ReturnType<typeof setTimeout> | undefined;
	const hostOf = (u: string) => (u ?? '').replace(/^https?:\/\//i, '').replace(/^www\./i, '').split('/')[0].toLowerCase();

	const EMDASH_TOKEN_STEPS = [
		'Masuk ke panel admin situs EmDash Anda',
		'Buka pengaturan akun lalu bagian Personal Access Token (token akses pribadi)',
		'Buat token baru dengan nama "Tauthub" dan centang izin media:write serta content:write',
		'Salin token yang muncul (diawali ec_pat_), hanya ditampilkan sekali',
		'Tempel di kolom Access Token di atas'
	];

	const APP_PASSWORD_STEPS = [
		'Masuk ke wp-admin situs Anda, contoh: https://situs-media.example/wp-admin',
		'Buka menu Pengguna → Profil',
		'Gulir ke bagian Application Passwords',
		'Isi nama aplikasi (contoh: "Tauthub"), klik Add New Application Password',
		'Salin password yang muncul (format xxxx xxxx xxxx xxxx xxxx xxxx), hanya ditampilkan sekali',
		'Tempel di kolom Application Password di atas'
	];

	// GET /v1/partner/cms-connections mengembalikan SELURUH koneksi dalam satu
	// respons (tanpa limit/offset server), jadi paginasi dilakukan di klien
	// (parity usePagination app lama, 20 baris per halaman).
	const PAGE_SIZE = 20;

	let rows = $state<any[]>([]);
	let pageNumber = $state(1);
	let mediaOptions = $state<any[]>([]);
	let loading = $state(true);
	let error = $state('');
	let busy = $state(false);
	let testingId = $state<string | null>(null);
	let createOpen = $state(false);
	let form = $state({ website_id: '', name: '', base_url: '', username: '', application_password: '', access_token: '' });

	const totalPages = $derived(Math.max(1, Math.ceil(rows.length / PAGE_SIZE)));
	const visibleRows = $derived(rows.slice((pageNumber - 1) * PAGE_SIZE, pageNumber * PAGE_SIZE));

	function goToPage(next: number) {
		pageNumber = Math.min(Math.max(1, next), totalPages);
	}

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			await applyHighlight();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
			clearTimeout(highlightTimer);
		};
	});

	async function applyHighlight() {
		const id = highlight;
		if (!id) return;
		onhighlighted?.();
		const media = mediaOptions.find((m) => m.id === id);
		// Sudah ada koneksi aktif untuk domain media ini: sorot barisnya.
		const host = media ? hostOf(media.website_url) : '';
		const conn = host ? rows.find((r) => r.status === 'active' && (r.website_id === id || hostOf(r.base_url) === host)) : null;
		if (conn) {
			const idx = rows.findIndex((r) => r.id === conn.id);
			if (idx >= 0) goToPage(Math.floor(idx / PAGE_SIZE) + 1);
			highlightConnId = conn.id;
			await tick();
			document.getElementById(`int-conn-${conn.id}`)?.scrollIntoView({ block: 'center', behavior: 'smooth' });
			highlightTimer = setTimeout(() => (highlightConnId = ''), 3200);
			return;
		}
		if (media) pickMedia(id);
		else highlightNote = tr('Media ini belum punya website URL. Isi dulu di Daftar Media, lalu sambungkan di sini.');
		createOpen = true;
	}

	async function load() {
		loading = true;
		error = '';
		try {
			const [connRes, mediaRes] = await Promise.all([
				apiRequest('/v1/partner/cms-connections'),
				apiRequest('/v1/partner/media?limit=500&offset=0')
			]);
			if (connRes.ok) {
				// Endpoint mengembalikan semua provider; tiap tab hanya menampilkan miliknya.
				const all: any[] = (await connRes.json()).connections ?? [];
				rows = all.filter((r) => (isEmdash ? r.provider === 'emdash' : r.provider !== 'emdash'));
				goToPage(pageNumber);
			} else {
				error = await readApiError(connRes, tr('Gagal memuat koneksi.'));
			}
			if (mediaRes.ok) {
				const media = (await mediaRes.json()).media ?? [];
				mediaOptions = media.filter((item: any) => Boolean(item.website_url));
			}
		} finally {
			loading = false;
			onchange?.();
		}
	}

	function pickMedia(id: string) {
		const media = mediaOptions.find((item) => item.id === id);
		form.website_id = id;
		if (media?.website_url) form.base_url = media.website_url;
		if (!form.name.trim() && media) form.name = `${media.name} ${providerLabel}`;
	}

	async function create(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			// Provider terkunci per tab: WordPress (Application Password) atau EmDash (token).
			const body: Record<string, unknown> = isEmdash
				? {
						provider: 'emdash',
						website_id: form.website_id || null,
						name: form.name.trim(),
						base_url: form.base_url.trim(),
						secret: form.access_token.trim()
					}
				: {
						provider: 'wordpress',
						website_id: form.website_id || null,
						name: form.name.trim(),
						base_url: form.base_url.trim(),
						username: form.username.trim() || null,
						application_password: form.application_password
					};
			const res = await apiRequest('/v1/partner/cms-connections', { method: 'POST', body: JSON.stringify(body) });
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menyimpan koneksi.'));
				return;
			}
			createOpen = false;
			form = { website_id: '', name: '', base_url: '', username: '', application_password: '', access_token: '' };
			await load();
		} finally {
			busy = false;
		}
	}

	async function test(row: any) {
		error = '';
		testingId = row.id;
		try {
			const res = await apiRequest(`/v1/partner/cms-connections/${row.id}/test`, { method: 'POST' }, 30_000);
			if (!res.ok) error = await readApiError(res, tr('Uji koneksi gagal.'));
			await load();
		} finally {
			testingId = null;
		}
	}

	async function revoke(row: any) {
		error = '';
		const res = await apiRequest(`/v1/partner/cms-connections/${row.id}`, { method: 'DELETE' });
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal mencabut koneksi.'));
			return;
		}
		await load();
	}

	async function removeConnection(row: any) {
		if (!confirm(`Hapus permanen koneksi "${row.name}"? Tindakan ini tidak bisa dibatalkan.`)) return;
		error = '';
		const res = await apiRequest(`/v1/partner/cms-connections/${row.id}?mode=delete`, { method: 'DELETE' });
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal menghapus koneksi.'));
			return;
		}
		await load();
	}

	// Satu bahasa status: titik + label Indonesia, bukan nilai mentah API.
	const statusTone = (row: any): 'ok' | 'error' | 'idle' =>
		row.status !== 'active' ? 'idle' : row.last_test_status === 'ok' ? 'ok' : row.last_test_status === 'error' ? 'error' : 'idle';
	const statusLabel = (row: any) =>
		row.status !== 'active'
			? tr('Dicabut')
			: row.last_test_status === 'ok'
				? tr('Terhubung')
				: row.last_test_status === 'error'
					? tr('Gagal')
					: tr('Belum diuji');
	const testTone = (row: any) =>
		row.last_test_status === 'ok' ? 'var(--success)' : row.last_test_status === 'error' ? 'var(--destructive)' : 'var(--muted-foreground)';
</script>

<div class="app-card int-card overflow-hidden">
	<div class="int-card__head int-card__head--split">
		<div>
			<h2 class="int-card__title">{isEmdash ? tr('Koneksi EmDash') : tr('Koneksi WordPress')} <span class="int-card__caption">{rows.length}</span></h2>
			<p class="int-card__desc">{isEmdash
				? tr('Artikel untuk media Anda yang berplatform EmDash dikirim lewat koneksi ini memakai token akses pribadi. Judul, deskripsi meta, isi, cover, dan gambar artikel ikut dipindahkan ke media library situs Anda')
				: tr('Artikel untuk media Anda yang berplatform WordPress dikirim lewat koneksi ini memakai Application Password, bukan koneksi brand klien')}</p>
		</div>
		<button class="app-btn app-btn-primary int-btn" type="button" onclick={() => (createOpen = true)}><Plus size={16} strokeWidth={1.5} />{tr('Tambah koneksi')}</button>
	</div>

	{#if !isEmdash}
	<p class="px-5 py-4 text-[13px] leading-5 text-muted-foreground" style="border-bottom: 1px solid var(--border);">{tr('Untuk koneksi WordPress, akun yang Anda hubungkan sebaiknya berperan Editor. Akun berperan Penulis tetap bisa menerbitkan artikel, tetapi tidak bisa membuat kategori baru di WordPress. Jika kategori artikel belum ada di situs Anda, artikel akan terbit di kategori bawaan (Uncategorized). Anda bisa membuat kategorinya sendiri sekali saja di WordPress, atau menaikkan peran akun menjadi Editor agar kategori terbuat otomatis.')}</p>
	{/if}

	{#if error}<p class="int-msg int-msg--error" style="padding-top: 12px;">{error}</p>{/if}

	{#if loading && rows.length === 0}
		<p class="int-empty">{tr('Memuat koneksi…')}</p>
	{:else if rows.length === 0}
		<div class="int-empty">
			<p>{isEmdash ? tr('Belum ada koneksi. Tambahkan koneksi EmDash untuk media berplatform EmDash') : tr('Belum ada koneksi. Tambahkan koneksi WordPress untuk media berplatform WordPress')}</p>
		</div>
	{:else}
		<!-- Desktop (≥768px): tabel -->
		<div class="hidden md:block">
			<table class="int-table">
				<thead>
					<tr>
						<th>{tr('Koneksi')}</th>
						<th>Endpoint</th>
						<th>Status</th>
						<th>{tr('Uji terakhir')}</th>
						<th class="is-right">{tr('Aksi')}</th>
					</tr>
				</thead>
				<tbody>
					{#each visibleRows as row (row.id)}
						<tr id="int-conn-{row.id}" class:int-tr--highlight={row.id === highlightConnId}>
							<td>
								<p class="font-medium">{row.name}</p>
								<p class="int-table__sub">{isEmdash ? String(row.id).slice(0, 8) : (row.username ?? String(row.id).slice(0, 8))}</p>
							</td>
							<td class="max-w-[260px]"><span class="int-mono">{row.base_url}</span></td>
							<td><Status tone={statusTone(row)} label={statusLabel(row)} /></td>
							<td class="max-w-[280px]"><span class="int-meta line-clamp-3" style="color: {testTone(row)};" title={row.last_test_message ?? ''}>{row.last_test_message ?? tr('Belum diuji')}</span></td>
							<td>
								<div class="int-table__actions">
									{@render connActionButtons(row)}
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<!-- Mobile (<768px): kartu per koneksi -->
		<div class="md:hidden">
		<div class="int-list">
			{#each visibleRows as row (row.id)}
				<article class="int-item" id="int-conn-{row.id}" class:int-item--highlight={row.id === highlightConnId}>
					<div class="int-item__head">
						<div class="min-w-0">
							<p class="int-item__name">{row.name}</p>
							<p class="int-meta">{isEmdash ? String(row.id).slice(0, 8) : (row.username ?? String(row.id).slice(0, 8))}</p>
						</div>
						<Status tone={statusTone(row)} label={statusLabel(row)} />
					</div>
					<dl class="mt-3 flex flex-col gap-2">
						<div>
							<dt class="int-meta">Endpoint</dt>
							<dd class="int-mono" style="color: var(--foreground);">{row.base_url}</dd>
						</div>
						{#if row.last_test_message}
							<div>
								<dt class="int-meta">{tr('Uji terakhir')}</dt>
								<dd class="text-[13px] leading-5" style="color: {testTone(row)};">{row.last_test_message}</dd>
							</div>
						{/if}
					</dl>
					<div class="int-item__actions grid grid-cols-2">
						{@render connActionButtons(row)}
					</div>
				</article>
			{/each}
		</div>
		</div>
	{/if}

	{#if rows.length > PAGE_SIZE}
		<div class="int-pager">
			<span>{(pageNumber - 1) * PAGE_SIZE + 1}–{Math.min(pageNumber * PAGE_SIZE, rows.length)} {tr('dari')} {rows.length}</span>
			<div class="int-pager__nav">
				<button class="app-btn app-btn-soft" disabled={pageNumber <= 1} onclick={() => goToPage(pageNumber - 1)}>{tr('Sebelumnya')}</button>
				<span>{pageNumber} / {totalPages}</span>
				<button class="app-btn app-btn-soft" disabled={pageNumber >= totalPages} onclick={() => goToPage(pageNumber + 1)}>{tr('Berikutnya')}</button>
			</div>
		</div>
	{/if}
</div>

{#if createOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card w-full max-w-lg p-6" aria-label={tr('Tambah koneksi')}>
			<h2 class="int-dialog__title">{isEmdash ? tr('Tambah koneksi EmDash') : tr('Tambah koneksi WordPress')}</h2>
			{#if highlightNote}<p class="int-field__hint" style="color: var(--warning);">{highlightNote}</p>{/if}
			{#if error}<p class="mt-2 text-sm" style="color: var(--destructive);">{error}</p>{/if}
			<form class="int-form mt-5" onsubmit={create}>
				<div class="int-field">
					<span class="int-field__label">Provider</span>
					<div class="int-field--locked">
						<span>{isEmdash ? 'EmDash REST API' : 'WordPress REST API'}</span>
						<span class="int-meta">{tr('Terkunci')}</span>
					</div>
					<span class="int-field__hint">{isEmdash ? tr('Tab ini khusus koneksi EmDash') : tr('Koneksi media mitra hanya mendukung WordPress')}</span>
				</div>

				<label class="int-field">
					<span class="int-field__label">{tr('Media')}</span>
					<Select
						ariaLabel={tr('Pilih media')}
						placeholder={mediaOptions.length > 0 ? tr('Pilih media yang akan dikoneksikan') : tr('Belum ada media dengan website URL')}
						minWidth={260}
						value={form.website_id}
						onchange={(v) => pickMedia(v)}
						options={[
							{
								value: '',
								label: mediaOptions.length > 0 ? tr('Pilih media yang akan dikoneksikan') : tr('Belum ada media dengan website URL'),
								disabled: true
							},
							...mediaOptions.map((media) => ({ value: media.id, label: `${media.name} (${media.website_url})` }))
						]}
					/>
					<span class="int-field__hint">{isEmdash ? tr('Diambil dari daftar media Anda; URL EmDash terisi otomatis') : tr('Diambil dari daftar media Anda; URL WordPress terisi otomatis')}</span>
				</label>

				<label class="int-field">
					<span class="int-field__label">{tr('Nama koneksi')}</span>
					<input class="app-input" required placeholder={tr('Nama koneksi')} bind:value={form.name} />
				</label>
				<label class="int-field">
					<span class="int-field__label">{isEmdash ? tr('URL EmDash') : tr('URL WordPress')}</span>
					<input class="app-input" required readonly placeholder={tr('Pilih media terlebih dahulu')} bind:value={form.base_url} />
				</label>
				{#if isEmdash}
					<label class="int-field">
						<span class="int-field__label">Access Token</span>
						<input
							class="app-input"
							required
							type="password"
							minlength="16"
							placeholder="ec_pat_…"
							value={form.access_token}
							oninput={(e) => (form.access_token = e.currentTarget.value.replace(/\s+/g, ''))}
						/>
						<div class="int-steps">
							<p class="int-steps__title">{tr('Cara membuat Access Token')}</p>
							<ol>
								{#each EMDASH_TOKEN_STEPS as step, i (i)}
									<li><span>{i + 1}</span><span>{step}</span></li>
								{/each}
							</ol>
						</div>
					</label>
				{:else}
				<label class="int-field">
					<span class="int-field__label">{tr('Username atau email WordPress')}</span>
					<input class="app-input" required bind:value={form.username} />
				</label>
				<label class="int-field">
					<span class="int-field__label">Application Password</span>
					<input
						class="app-input"
						required
						type="password"
						placeholder="xxxx xxxx xxxx xxxx xxxx xxxx"
						value={form.application_password}
						oninput={(e) => (form.application_password = e.currentTarget.value.replace(/\s+/g, ''))}
					/>
					<div class="int-steps">
						<p class="int-steps__title">{tr('Cara mengambil Application Password')}</p>
						<ol>
							{#each APP_PASSWORD_STEPS as step, i (i)}
								<li><span>{i + 1}</span><span>{step}</span></li>
							{/each}
						</ol>
					</div>
				</label>
				{/if}

				<div class="int-form__foot">
					<button class="app-btn app-btn-soft int-btn" type="button" onclick={() => { createOpen = false; highlightNote = ''; }}>{tr('Batal')}</button>
					<button class="app-btn app-btn-primary int-btn" type="submit" disabled={busy}>{tr('Simpan koneksi')}</button>
				</div>
			</form>
		</dialog>
	</div>
{/if}

{#snippet connActionButtons(row: any)}
	{#if row.status === 'active'}
		<button class="app-btn app-btn-soft int-btn" disabled={testingId !== null} onclick={() => test(row)}>
			{testingId === row.id ? tr('Menguji…') : tr('Uji koneksi')}
		</button>
		<button class="app-btn app-btn-soft int-btn int-btn--danger" onclick={() => revoke(row)}>{tr('Cabut')}</button>
	{:else}
		<button class="app-btn app-btn-soft int-btn int-btn--danger" onclick={() => removeConnection(row)}>{tr('Hapus')}</button>
	{/if}
{/snippet}
