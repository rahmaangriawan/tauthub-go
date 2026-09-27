<script lang="ts">
	// Tab Blogger — port BloggerSection app lama
	// (app/mitra/[partnerId]/[[...section]]/sections.tsx baris ~2824): hubungkan
	// akun Google media Blogger, pilih blog tujuan, uji koneksi, putuskan.
	// OAuth return dari /api/blogger/callback mendarat di sini sebagai
	// ?blogger_connected=1&media=<id>#blogger → langsung buka pemilih blog.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Globe, Newspaper, Unplug, X } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

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

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			await handleOAuthReturn();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

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
			notice = 'Kredensial Blogger tersimpan. Pilih blog untuk melanjutkan.';
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
				error = await readApiError(res, 'Gagal memuat media Blogger.');
				return;
			}
			const all = (await res.json()).media ?? [];
			rows = all.filter((m: any) => m.integration_type === 'blogger' && m.status !== 'hidden');
		} finally {
			loading = false;
		}
	}

	async function connect(item: any) {
		busyId = item.id;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/partner/media/${item.id}/blogger/connect`);
			if (!res.ok) {
				error = await readApiError(res, 'Tidak dapat memulai koneksi Blogger.');
				return;
			}
			const result = await res.json();
			if (result.url) window.location.href = result.url;
			else error = 'Tidak dapat memulai koneksi Blogger.';
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
				blogError = await readApiError(res, 'Gagal memuat daftar blog.');
				return;
			}
			const data = await res.json();
			blogs = data.blogs ?? [];
			if (blogs.length === 0) blogError = 'Tidak ada blog ditemukan di akun Google ini.';
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
				blogError = await readApiError(res, 'Gagal menyimpan pilihan blog.');
				return;
			}
			pickTarget = null;
			notice = `Blog "${blog?.name ?? ''}" dipilih. Koneksi Blogger aktif.`;
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
				error = await readApiError(res, 'Uji koneksi Blogger gagal.');
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
		if (!confirm(`Putuskan koneksi Blogger untuk "${item.name}"? Artikel berikutnya tidak akan terkirim.`)) return;
		busyId = item.id;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/partner/media/${item.id}/blogger`, { method: 'DELETE' });
			if (!res.ok) {
				error = await readApiError(res, 'Gagal memutuskan koneksi Blogger.');
				return;
			}
			notice = 'Koneksi Blogger diputuskan.';
			await load();
		} finally {
			busyId = null;
		}
	}

	const statusLabel = (status: string) =>
		status === 'connected'
			? '✓ Terhubung'
			: status === 'failed'
				? '✕ Koneksi gagal'
				: status === 'testing'
					? 'Menguji…'
					: 'Belum terhubung';
	const statusColor = (status: string) =>
		status === 'connected'
			? 'var(--success)'
			: status === 'failed'
				? 'var(--destructive)'
				: status === 'testing'
					? 'var(--warning)'
					: 'var(--muted-foreground)';

	const dt = (v: string | null) =>
		v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—';
</script>

<div class="app-card mt-4">
	<div class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3">
		<div class="flex items-center gap-2">
			<Newspaper size={16} />
			<h2 class="text-sm font-semibold">Blogger</h2>
			<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--secondary); color: var(--muted-foreground);">{rows.length} media</span>
		</div>
	</div>
	<p class="px-4 pt-3 text-xs text-muted-foreground">
		Hubungkan akun Google Anda, pilih blog, lalu artikel dari klien terkirim otomatis ke Blogger (draft atau terbit).
	</p>

	{#if error}<p class="px-4 pt-2 text-sm" style="color: var(--destructive);">{error}</p>{/if}
	{#if notice}<p class="px-4 pt-2 text-sm" style="color: var(--success);">{notice}</p>{/if}

	{#if loading && rows.length === 0}
		<p class="px-4 py-8 text-center text-sm text-muted-foreground">Memuat Blogger…</p>
	{:else if rows.length === 0}
		<div class="px-4 py-8 text-center">
			<Globe size={26} class="mx-auto" style="color: var(--muted-foreground);" />
			<p class="mt-2 text-sm text-muted-foreground">
				Belum ada media platform Blogger. Pilih platform Blogger saat menambah media, lalu hubungkan di sini.
			</p>
		</div>
	{:else}
		<div class="space-y-3 p-4">
			{#each rows as item (item.id)}
				<div class="rounded-lg border p-3">
					<div class="flex flex-wrap items-center justify-between gap-2">
						<div class="flex items-center gap-2">
							<Newspaper size={15} />
							<p class="text-sm font-bold">{item.name}</p>
						</div>
						<div class="flex flex-wrap items-center gap-2">
							<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--secondary); color: {statusColor(item.blogger_status)};">{statusLabel(item.blogger_status)}</span>
							{#if item.blogger_blog_name}
								<span class="text-sm font-medium">Blog: {item.blogger_blog_name}</span>
							{/if}
							{#if item.blogger_last_test_at}
								<span class="text-xs text-muted-foreground">Terakhir dicek: {dt(item.blogger_last_test_at)}</span>
							{/if}
						</div>
					</div>

					<div class="mt-3 flex flex-wrap items-center gap-2">
						{#if !item.blogger_connected}
							<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" type="button" disabled={busyId === item.id} onclick={() => connect(item)}>
								{busyId === item.id ? 'Menghubungkan…' : 'Hubungkan Blogger'}
							</button>
						{:else}
							<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={busyId === item.id} onclick={() => openBlogPicker(item)}>Ganti Blog</button>
							<button
								class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
								type="button"
								disabled={busyId === item.id || !item.blogger_blog_id}
								onclick={() => testConnection(item)}
							>{busyId === item.id ? 'Menguji…' : 'Test Connection'}</button>
							<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" style="color: var(--destructive);" disabled={busyId === item.id} onclick={() => disconnect(item)}>
								<Unplug size={12} /> Putuskan
							</button>
						{/if}
					</div>

					{#if item.blogger_last_test_message && item.blogger_status === 'failed'}
						<p class="mt-2 text-xs" style="color: var(--destructive);">{item.blogger_last_test_message}</p>
					{/if}
					{#if !item.blogger_blog_id && item.blogger_connected}
						<p class="mt-2 text-xs text-muted-foreground">
							Kredensial tersimpan — pilih tombol <strong>Ganti Blog</strong> untuk menyelesaikan koneksi.
						</p>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

{#if pickTarget}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card my-8 w-full max-w-md p-5" aria-label="Pilih Blog Blogger">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<h2 class="text-lg font-semibold">Pilih Blog Blogger</h2>
				<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => (pickTarget = null)}><X size={12} /> Batal</button>
			</div>
			<p class="mt-1 text-sm text-muted-foreground">Pilih blog untuk media "{pickTarget.name}".</p>

			<div class="mt-3 space-y-2">
				{#if blogBusy}
					<p class="text-sm text-muted-foreground">Memuat daftar blog…</p>
				{/if}
				{#if blogError}
					<p class="text-sm" style="color: var(--destructive);">{blogError}</p>
				{/if}
				{#if !blogBusy && blogs.length > 0}
					{#each blogs as b (b.id)}
						<button
							class="flex w-full items-start gap-2.5 rounded-lg border px-3 py-2.5 text-left"
							type="button"
							style="border-color: {selectedBlogId === b.id ? 'var(--accent)' : 'var(--border)'}; background: {selectedBlogId === b.id ? 'var(--secondary)' : 'transparent'};"
							onclick={() => (selectedBlogId = b.id)}
						>
							<span
								class="mt-1 inline-block h-3.5 w-3.5 shrink-0 rounded-full border"
								style="border-color: {selectedBlogId === b.id ? 'var(--accent)' : 'var(--border)'}; background: {selectedBlogId === b.id ? 'var(--accent)' : 'transparent'};"
							></span>
							<span class="min-w-0">
								<span class="block text-sm font-medium">{b.name || '(tanpa nama)'}</span>
								{#if b.url}<span class="block truncate text-xs text-muted-foreground">{b.url}</span>{/if}
							</span>
						</button>
					{/each}
				{/if}
			</div>

			<div class="mt-4 flex justify-end gap-2 border-t pt-3">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (pickTarget = null)}>Batal</button>
				<button class="app-btn app-btn-primary" type="button" disabled={blogBusy || !selectedBlogId} onclick={saveBlog}>
					{blogBusy ? 'Menyimpan…' : 'Gunakan Blog Ini'}
				</button>
			</div>
		</dialog>
	</div>
{/if}
