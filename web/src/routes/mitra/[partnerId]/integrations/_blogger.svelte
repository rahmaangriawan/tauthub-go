<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Tab Blogspot — port BloggerSection app lama
	// (app/mitra/[partnerId]/[[...section]]/sections.tsx baris ~2824): hubungkan
	// akun Google media Blogspot, pilih blog tujuan, uji koneksi, putuskan.
	// OAuth return dari /api/blogger/callback mendarat di sini sebagai
	// ?blogger_connected=1&media=<id>#blogger → langsung buka pemilih blog.
	import { onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { X } from '@lucide/svelte';
	import Status from './_status.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	let { onchange, highlight = '', onhighlighted }: { onchange?: () => void; highlight?: string; onhighlighted?: () => void } = $props();

	// Media yang disorot setelah deep link dari Daftar Media (?media=<id>).
	let highlightActive = $state('');
	let highlightTimer: ReturnType<typeof setTimeout> | undefined;

	let rows = $state<any[]>([]);
	let loading = $state(true);
	let error = $state('');
	let busyId = $state<string | null>(null);
	let notice = $state('');
	let pickTarget = $state<any>(null);
	let blogs = $state<Array<{ id: string; name: string; url: string }>>([]);
	let blogBusy = $state(false);
	let blogError = $state('');
	let selectedBlogId = $state('');
	const base = $derived(`/mitra/${page.params.partnerId}`);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			await handleOAuthReturn();
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
		if (!rows.some((r) => r.id === id)) return;
		highlightActive = id;
		await tick();
		document.getElementById(`int-media-${id}`)?.scrollIntoView({ block: 'center', behavior: 'smooth' });
		highlightTimer = setTimeout(() => (highlightActive = ''), 3200);
	}

	// Setelah callback OAuth → buka pemilih blog untuk media tersebut, lalu bersihkan
	// query agar reload tidak membuka picker lagi (parity effect app lama).
	//
	// Catatan: callback app lama mengirim `#blogger?blogger_connected=1&media=<id>`,
	// yaitu query menempel di dalam fragment — karena itu parameter dibaca dari
	// search params maupun dari hash.
	function readParam(name: string): string | null {
		const direct = page.url.searchParams.get(name);
		if (direct) return direct;
		const q = page.url.hash.split('?')[1];
		if (!q) return null;
		return new URLSearchParams(q).get(name);
	}

	async function handleOAuthReturn() {
		const connected = readParam('blogger_connected');
		const mediaId = readParam('media');
		if (connected !== '1' || !mediaId) return;
		const target = rows.find((m) => m.id === mediaId);
		if (target && !target.blogger_blog_id) {
			notice = tr('Kredensial Blogspot tersimpan. Pilih blog untuk melanjutkan.');
			await openBlogPicker(target);
		} else if (target?.blogger_blog_id) {
			notice = `Blog "${target.blogger_blog_name ?? ''}" sudah terhubung.`;
		}
		const next = new URL(page.url);
		next.searchParams.delete('blogger_connected');
		next.searchParams.delete('media');
		next.hash = 'blogger';
		await goto(`${next.pathname}${next.search}${next.hash}`, { replaceState: true, keepFocus: true, noScroll: true });
	}

	async function load() {
		loading = true;
		error = '';
		try {
			const res = await apiRequest('/v1/partner/media?limit=500');
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal memuat media Blogspot.'));
				return;
			}
			const all = (await res.json()).media ?? [];
			rows = all.filter((m: any) => m.integration_type === 'blogger' && m.status !== 'hidden');
		} finally {
			loading = false;
			onchange?.();
		}
	}

	async function connect(item: any) {
		busyId = item.id;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/partner/media/${item.id}/blogger/connect`);
			if (!res.ok) {
				error = await readApiError(res, tr('Tidak dapat memulai koneksi Blogspot.'));
				return;
			}
			const result = await res.json();
			if (result.url) window.location.href = result.url;
			else error = tr('Tidak dapat memulai koneksi Blogspot.');
		} finally {
			busyId = null;
		}
	}

	async function openBlogPicker(item: any) {
		pickTarget = item;
		blogError = '';
		blogs = [];
		selectedBlogId = '';
		blogBusy = true;
		try {
			const res = await apiRequest(`/v1/partner/media/${item.id}/blogger/blogs`);
			if (!res.ok) {
				blogError = await readApiError(res, tr('Gagal memuat daftar blog.'));
				return;
			}
			const data = await res.json();
			blogs = data.blogs ?? [];
			if (blogs.length === 0) blogError = tr('Tidak ada blog ditemukan di akun Google ini.');
		} finally {
			blogBusy = false;
		}
	}

	async function saveBlog() {
		if (!pickTarget || !selectedBlogId) return;
		blogBusy = true;
		blogError = '';
		try {
			const blog = blogs.find((b) => b.id === selectedBlogId);
			const res = await apiRequest(`/v1/partner/media/${pickTarget.id}/blogger`, {
				method: 'POST',
				body: JSON.stringify({ blog_id: selectedBlogId, blog_name: blog?.name ?? '' })
			});
			if (!res.ok) {
				blogError = await readApiError(res, tr('Gagal menyimpan pilihan blog.'));
				return;
			}
			pickTarget = null;
			notice = `Blog "${blog?.name ?? ''}" dipilih. Koneksi Blogspot aktif.`;
			await load();
		} finally {
			blogBusy = false;
		}
	}

	async function testConnection(item: any) {
		busyId = item.id;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/partner/media/${item.id}/blogger/test-connection`, { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, tr('Uji koneksi Blogspot gagal.'));
				return;
			}
			const result = await res.json();
			notice = result.message ?? '';
			await load();
		} finally {
			busyId = null;
		}
	}

	async function disconnect(item: any) {
		if (!confirm(`Putuskan koneksi Blogspot untuk "${item.name}"? Artikel berikutnya tidak akan terkirim.`)) return;
		busyId = item.id;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/partner/media/${item.id}/blogger`, { method: 'DELETE' });
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal memutuskan koneksi Blogspot.'));
				return;
			}
			notice = tr('Koneksi Blogspot diputuskan.');
			await load();
		} finally {
			busyId = null;
		}
	}

	const statusTone = (status: string): 'ok' | 'error' | 'warn' | 'idle' =>
		status === 'connected' ? 'ok' : status === 'failed' ? 'error' : status === 'testing' ? 'warn' : 'idle';
	const statusLabel = (status: string) =>
		status === 'connected'
			? tr('Terhubung')
			: status === 'failed'
				? tr('Koneksi gagal')
				: status === 'testing'
					? tr('Menguji…')
					: tr('Belum terhubung');

	const dt = (v: string | null) =>
		v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—';
</script>

<div class="app-card int-card overflow-hidden">
	<div class="int-card__head int-card__head--split">
		<div>
			<h2 class="int-card__title">Blogspot <span class="int-card__caption">{rows.length} media</span></h2>
			<p class="int-card__desc">{tr('Hubungkan akun Google media Anda dan pilih blog tujuan; artikel klien terkirim otomatis ke Blogspot sebagai draf atau terbit')}</p>
		</div>
	</div>

	{#if error}<p class="int-msg int-msg--error" style="padding-top: 12px;">{error}</p>{/if}
	{#if notice}<p class="int-msg int-msg--ok" style="padding-top: 12px;">{notice}</p>{/if}

	{#if loading && rows.length === 0}
		<p class="int-empty">{tr('Memuat Blogspot…')}</p>
	{:else if rows.length === 0}
		<div class="int-empty">
			<p>{tr('Belum ada media berplatform Blogspot. Buat media dengan platform Blogspot di halaman Media; setelah disetujui admin, hubungkan di sini')}</p>
			<button class="app-btn app-btn-primary int-btn" type="button" onclick={() => goto(`${base}/media?integration_type=blogger`)}>
				{tr('Buat media Blogspot')}
			</button>
		</div>
	{:else}
		<div class="int-list" style="padding-top: 20px;">
			{#each rows as item (item.id)}
				<div class="int-item" id="int-media-{item.id}" class:int-item--highlight={item.id === highlightActive}>
					<div class="int-item__head">
						<p class="int-item__name">{item.name}</p>
						<div class="int-item__meta">
							<Status tone={statusTone(item.blogger_status)} label={statusLabel(item.blogger_status)} />
							{#if item.blogger_blog_name}
								<span class="text-sm">{item.blogger_blog_name}</span>
							{/if}
							{#if item.blogger_last_test_at}
								<span class="int-meta">{tr('Dicek')} {dt(item.blogger_last_test_at)}</span>
							{/if}
						</div>
					</div>

					<div class="int-item__actions">
						{#if !item.blogger_connected}
							<button class="app-btn app-btn-primary int-btn" type="button" disabled={busyId === item.id} onclick={() => connect(item)}>
								{busyId === item.id ? tr('Menghubungkan…') : tr('Hubungkan Blogspot')}
							</button>
						{:else}
							<button class="app-btn app-btn-soft int-btn" type="button" disabled={busyId === item.id} onclick={() => openBlogPicker(item)}>{tr('Ganti blog')}</button>
							<button
								class="app-btn app-btn-soft int-btn"
								type="button"
								disabled={busyId === item.id || !item.blogger_blog_id}
								onclick={() => testConnection(item)}
							>{busyId === item.id ? tr('Menguji…') : tr('Uji koneksi')}</button>
							<button class="app-btn app-btn-soft int-btn" type="button" disabled={busyId === item.id} onclick={() => connect(item)}>{tr('Hubungkan ulang')}</button>
							<button class="app-btn app-btn-soft int-btn int-btn--danger" type="button" disabled={busyId === item.id} onclick={() => disconnect(item)}>{tr('Putuskan')}</button>
						{/if}
					</div>

					{#if item.blogger_last_test_message && item.blogger_status === 'failed'}
						<p class="int-item__note" style="color: var(--destructive);">{item.blogger_last_test_message}</p>
					{/if}
					{#if item.blogger_connected}
						<p class="int-item__note">
							{tr('Gambar artikel disimpan ke Google Drive mitra sehingga tidak bergantung pada server Tauthub. Hubungkan ulang bila artikel terbit dengan gambar dari tautan Tauthub')}
						</p>
					{/if}
					{#if !item.blogger_blog_id && item.blogger_connected}
						<p class="int-item__note">{tr('Kredensial tersimpan. Pilih Ganti blog untuk menyelesaikan koneksi')}</p>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

{#if pickTarget}
	<div class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card w-full max-w-md p-6" aria-label={tr('Pilih blog')}>
			<div class="int-dialog__head">
				<h2 class="int-dialog__title">{tr('Pilih blog')}</h2>
				<button class="app-btn app-btn-soft int-btn" type="button" onclick={() => (pickTarget = null)}><X size={14} strokeWidth={1.5} />{tr('Tutup')}</button>
			</div>
			<p class="mt-1 text-sm text-muted-foreground">{tr('Blog tujuan untuk media')} {pickTarget.name}</p>

			<div class="mt-4 flex flex-col gap-2">
				{#if blogBusy}
					<p class="text-sm text-muted-foreground">{tr('Memuat daftar blog…')}</p>
				{/if}
				{#if blogError}
					<p class="text-sm" style="color: var(--destructive);">{blogError}</p>
				{/if}
				{#if !blogBusy && blogs.length > 0}
					{#each blogs as b (b.id)}
						<button
							class="int-choice"
							class:int-choice--selected={selectedBlogId === b.id}
							type="button"
							role="radio"
							aria-checked={selectedBlogId === b.id}
							onclick={() => (selectedBlogId = b.id)}
						>
							<span class="int-choice__radio" aria-hidden="true"></span>
							<span class="min-w-0">
								<span class="block text-sm font-medium">{b.name || tr('(tanpa nama)')}</span>
								{#if b.url}<span class="int-mono block truncate">{b.url}</span>{/if}
							</span>
						</button>
					{/each}
				{/if}
			</div>

			<div class="int-form__foot mt-6">
				<button class="app-btn app-btn-soft int-btn" type="button" onclick={() => (pickTarget = null)}>{tr('Batal')}</button>
				<button class="app-btn app-btn-primary int-btn" type="button" disabled={blogBusy || !selectedBlogId} onclick={saveBlog}>
					{blogBusy ? tr('Menyimpan…') : tr('Gunakan blog ini')}
				</button>
			</div>
		</dialog>
	</div>
{/if}
