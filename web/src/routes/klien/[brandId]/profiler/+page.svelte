<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Profiler — dashboard daftar workspace knowledge milik klien.
	// Satu klien bisa punya banyak Profiler (profil bisnis, gaya penulisan,
	// pedoman konten, dokumen/gambar/URL referensi) yang jadi acuan The Writer.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Copy, Pencil, Plus, Sparkles, Trash2 } from '@lucide/svelte';
	import PageTitle from '$lib/components/page-title.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT, showToast } from '$lib/auth';
	import { getBrandUrlId } from '$lib/brand-url.svelte';

	type ProfileRow = {
		id: string;
		name: string;
		description: string | null;
		readiness: 'empty' | 'review' | 'ready';
		documents_count: number;
		images_count: number;
		urls_count: number;
		blocks_count: number;
		updated_at: string | null;
	};

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/profiler`);

	let rows = $state<ProfileRow[]>([]);
	let loading = $state(true);
	let error = $state('');
	let search = $state('');
	let busy = $state(false);
	let createOpen = $state(false);
	let createForm = $state({ name: '', description: '' });
	let editing = $state<ProfileRow | null>(null);
	let editForm = $state({ name: '', description: '' });

	const filtered = $derived(
		rows.filter((r) => r.name.toLowerCase().includes(search.trim().toLowerCase()))
	);

	const READINESS: Record<string, { label: string; tone: string }> = {
		empty: { label: 'Belum Diisi', tone: 'var(--muted-foreground)' },
		review: { label: 'Perlu Diperiksa', tone: 'var(--warning)' },
		ready: { label: 'Siap Digunakan', tone: 'var(--success)' }
	};

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		error = '';
		const res = await apiRequest(`${apiBase}/profiles`);
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal memuat Profiler.'));
			return;
		}
		rows = (await res.json()).profiles ?? [];
	}

	async function create(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		try {
			const res = await apiRequest(`${apiBase}/profiles`, {
				method: 'POST',
				body: JSON.stringify({ name: createForm.name.trim(), description: createForm.description.trim() })
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal membuat Profiler.'));
				return;
			}
			const data = await res.json();
			createOpen = false;
			createForm = { name: '', description: '' };
			await load();
			if (data?.id) void goto(`/klien/${brandUrl}/profiler/${data.id}`);
		} finally {
			busy = false;
		}
	}

	async function saveEdit(e: SubmitEvent) {
		e.preventDefault();
		if (!editing) return;
		busy = true;
		try {
			const res = await apiRequest(`${apiBase}/profiles/${editing.id}`, {
				method: 'PUT',
				body: JSON.stringify({ name: editForm.name.trim(), description: editForm.description.trim() })
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menyimpan Profiler.'));
				return;
			}
			editing = null;
			await load();
		} finally {
			busy = false;
		}
	}

	async function duplicate(row: ProfileRow) {
		busy = true;
		try {
			const res = await apiRequest(`${apiBase}/profiles/${row.id}/duplicate`, { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menduplikasi Profiler.'));
				return;
			}
			await load();
		} finally {
			busy = false;
		}
	}

	async function remove(row: ProfileRow) {
		if (!confirm(tr('Hapus Profiler ini? Kampanye yang sudah memakainya tidak terpengaruh.'))) return;
		busy = true;
		try {
			const res = await apiRequest(`${apiBase}/profiles/${row.id}`, { method: 'DELETE' });
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menghapus Profiler.'));
				return;
			}
			await load();
		} finally {
			busy = false;
		}
	}

	function openEdit(row: ProfileRow) {
		editing = row;
		editForm = { name: row.name, description: row.description ?? '' };
	}

	const fmtDate = (v: string | null) =>
		v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—';
</script>

<PageTitle title={tr('Profiler')} />

<section>
	<header class="flex flex-wrap items-center justify-between gap-3">
		<div>
			<h1 class="text-xl font-bold">{tr('Profiler')}</h1>
			<p class="mt-1 max-w-2xl text-sm text-muted-foreground">
				{tr('Simpan identitas brand, gaya penulisan, pedoman konten, referensi, dan instruksi khusus yang jadi acuan AI The Writer. Satu akun boleh punya banyak Profiler.')}
			</p>
		</div>
		<button class="app-btn app-btn-primary" type="button" onclick={() => (createOpen = true)}>
			<Plus size={16} /> {tr('Profiler Baru')}
		</button>
	</header>

	{#if error}
		<p class="mt-3 rounded-lg px-3 py-2 text-sm" style="background: var(--destructive-soft, #fdecee); color: var(--destructive);">{error}</p>
	{/if}

	{#if !loading && rows.length > 0}
		<label class="app-input mt-4 flex max-w-sm items-center gap-2 !py-1.5">
			<input class="w-full bg-transparent text-sm outline-none" placeholder={tr('Cari Profiler…')} bind:value={search} />
		</label>
	{/if}

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">{tr('Memuat…')}</p>
	{:else if rows.length === 0}
		<div class="app-card mt-4 flex flex-col items-center gap-2 px-6 py-12 text-center">
			<div class="flex h-12 w-12 items-center justify-center rounded-xl" style="background: var(--accent); color: var(--primary);"><Sparkles size={22} /></div>
			<h2 class="mt-2 text-base font-semibold">{tr('Belum ada Profiler')}</h2>
			<p class="max-w-md text-sm text-muted-foreground">
				{tr('Profiler menyimpan seluruh konteks brand-mu — identitas bisnis, gaya bahasa, pedoman SEO, dokumen & tautan referensi, serta instruksi khusus. Sekali isi, semua artikel AI berikutnya mengikuti acuan ini.')}
			</p>
			<button class="app-btn app-btn-primary mt-3" type="button" onclick={() => (createOpen = true)}><Plus size={16} /> {tr('Buat Profiler Pertama')}</button>
		</div>
	{:else}
		<div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
			{#each filtered as row (row.id)}
				<div class="app-card flex flex-col p-4">
					<div class="flex items-start justify-between gap-2">
						<button class="min-w-0 text-left" type="button" onclick={() => void goto(`/klien/${brandUrl}/profiler/${row.id}`)}>
							<p class="truncate text-sm font-semibold">{row.name}</p>
							<p class="mt-0.5 line-clamp-2 text-xs text-muted-foreground">{row.description || tr('Tanpa deskripsi')}</p>
						</button>
						<span class="shrink-0 rounded-full px-2 py-0.5 text-[11px] font-semibold" style="background: var(--secondary); color: {READINESS[row.readiness]?.tone ?? 'var(--muted-foreground)'};">
							{tr(READINESS[row.readiness]?.label ?? row.readiness)}
						</span>
					</div>
					<p class="mt-3 text-[11px] text-muted-foreground">
						{row.documents_count} {tr('dokumen')} · {row.images_count} {tr('gambar')} · {row.urls_count} {tr('tautan')} · {row.blocks_count} {tr('blok')}
					</p>
					<p class="mt-0.5 text-[11px] text-muted-foreground">{tr('Diperbarui')}: {fmtDate(row.updated_at)}</p>
					<div class="mt-3 flex items-center gap-1.5 border-t pt-3" style="border-color: var(--border);">
						<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" type="button" onclick={() => void goto(`/klien/${brandUrl}/profiler/${row.id}`)}>{tr('Buka')}</button>
						<button class="app-btn app-btn-soft !min-h-8 !px-2.5 !text-xs" type="button" title={tr('Ganti nama')} onclick={() => openEdit(row)}><Pencil size={13} /></button>
						<button class="app-btn app-btn-soft !min-h-8 !px-2.5 !text-xs" type="button" disabled={busy} title={tr('Duplikasi')} onclick={() => void duplicate(row)}><Copy size={13} /></button>
						<button class="app-btn app-btn-soft !min-h-8 !px-2.5 !text-xs ml-auto" type="button" style="color: var(--destructive);" disabled={busy} title={tr('Hapus')} onclick={() => void remove(row)}><Trash2 size={13} /></button>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</section>

{#if createOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(15,23,42,.45);" role="presentation" onclick={() => (createOpen = false)}>
		<div class="app-card w-full max-w-md p-5" role="dialog" tabindex="-1" aria-label={tr('Profiler Baru')} onclick={(e) => e.stopPropagation()}>
			<h2 class="text-base font-semibold">{tr('Profiler Baru')}</h2>
			<form class="mt-3 flex flex-col gap-3" onsubmit={create}>
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">{tr('Nama Profiler')}</span>
					<input class="app-input mt-1 w-full" required maxlength={160} placeholder={tr('Contoh: Brand Utama 2026')} bind:value={createForm.name} />
				</label>
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">{tr('Deskripsi (opsional)')}</span>
					<input class="app-input mt-1 w-full" placeholder={tr('Catatan singkat untuk apa Profiler ini')} bind:value={createForm.description} />
				</label>
				<div class="mt-1 flex justify-end gap-2">
					<button class="app-btn app-btn-soft" type="button" onclick={() => (createOpen = false)}>{tr('Batal')}</button>
					<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? tr('Menyimpan…') : tr('Buat')}</button>
				</div>
			</form>
		</div>
	</div>
{/if}

{#if editing}
	<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background: rgba(15,23,42,.45);" role="presentation" onclick={() => (editing = null)}>
		<div class="app-card w-full max-w-md p-5" role="dialog" tabindex="-1" aria-label={tr('Ganti Nama')} onclick={(e) => e.stopPropagation()}>
			<h2 class="text-base font-semibold">{tr('Ganti Nama Profiler')}</h2>
			<form class="mt-3 flex flex-col gap-3" onsubmit={saveEdit}>
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">{tr('Nama Profiler')}</span>
					<input class="app-input mt-1 w-full" required maxlength={160} bind:value={editForm.name} />
				</label>
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">{tr('Deskripsi')}</span>
					<input class="app-input mt-1 w-full" bind:value={editForm.description} />
				</label>
				<div class="mt-1 flex justify-end gap-2">
					<button class="app-btn app-btn-soft" type="button" onclick={() => (editing = null)}>{tr('Batal')}</button>
					<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? tr('Menyimpan…') : tr('Simpan')}</button>
				</div>
			</form>
		</div>
	</div>
{/if}
