<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// /admin/changelog — impor entri "Apa yang Baru" dari web/static/changelog.md
	// lalu kelola (terbitkan/sembunyikan, hapus). Entri BARU saat impor otomatis
	// memicu lonceng notifikasi + web push lewat API (tipe changelog.new).
	// Sumber kebenaran isi entri tetap file repo; tabel hanya salinannya.
	import { onMount } from 'svelte';
	import { CircleCheck, FileText, LoaderCircle, RefreshCw, Trash, TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { platformError } from '../_platform';
	import { fmtDateTime } from '$lib/admin-format';
	import { parseChangelog, categoryLabel, type ParsedChangelogEntry } from '$lib/changelog';

	type Entry = {
		id: string;
		version: string;
		released_on: string;
		category: string;
		title: string;
		summary: string;
		body: string;
		source_key: string;
		published: boolean;
		read_count: number;
		created_at: string | null;
	};

	const SOURCE_URL = '/changelog.md';

	let items = $state<Entry[]>([]);
	let parsed = $state<ParsedChangelogEntry[]>([]);
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');
	let sourceError = $state('');
	let confirmDelete = $state<Entry | null>(null);

	const newEntries = $derived(parsed.filter((p) => !items.some((i) => i.source_key === p.source_key)));
	const updatedEntries = $derived(parsed.filter((p) => items.some((i) => i.source_key === p.source_key)));

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([load(), loadSource()]);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		error = '';
		try {
			const res = await apiRequest('/v1/platform/changelog');
			if (!res.ok) throw new Error(await platformError(res));
			const data = (await res.json()) as { items?: Entry[] };
			items = data.items ?? [];
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal memuat entri changelog.');
		}
	}

	async function loadSource() {
		sourceError = '';
		try {
			const res = await fetch(`${SOURCE_URL}?v=${Date.now()}`);
			if (!res.ok) throw new Error(`HTTP ${res.status}`);
			parsed = parseChangelog(await res.text());
		} catch {
			sourceError = tr('File changelog.md tidak terbaca. Pastikan web/static/changelog.md ada dan sudah ter-deploy.');
		}
	}

	async function importEntries() {
		if (busy || parsed.length === 0) return;
		busy = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest('/v1/platform/changelog/import', {
				method: 'POST',
				body: JSON.stringify({ entries: parsed })
			});
			if (!res.ok) throw new Error(await platformError(res));
			const data = (await res.json()) as { created?: number; updated?: number; notified?: number };
			notice = tr('Impor selesai') + `: ${data.created ?? 0} ${tr('entri baru')}, ${data.updated ?? 0} ${tr('diperbarui')}, ${data.notified ?? 0} ${tr('notifikasi terkirim')}.`;
			await load();
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal mengimpor entri.');
		} finally {
			busy = false;
		}
	}

	async function togglePublish(entry: Entry) {
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/changelog/${entry.id}`, {
				method: 'PATCH',
				body: JSON.stringify({ published: !entry.published })
			});
			if (!res.ok) throw new Error(await platformError(res));
			await load();
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal mengubah status terbit.');
		} finally {
			busy = false;
		}
	}

	async function removeEntry() {
		if (!confirmDelete) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/changelog/${confirmDelete.id}`, { method: 'DELETE' });
			if (!res.ok) throw new Error(await platformError(res));
			confirmDelete = null;
			await load();
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal menghapus entri.');
		} finally {
			busy = false;
		}
	}

	const categoryTone = (category: string) =>
		category === 'fitur' ? 'accent' : category === 'perbaikan' ? 'warning' : category === 'performa' ? 'success' : 'neutral';
</script>

<PageHeader title={tr('Changelog')} description={tr('Catatan rilis yang tampil di dashboard klien & mitra sebagai "Apa yang Baru".')} />

<Feedback {loading} {error} {notice} loadingText={tr('Memuat changelog…')} />

<section class="app-card mt-4 p-4">
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h2 class="font-semibold">{tr('Sumber entri')}</h2>
			<p class="mt-1 max-w-2xl text-sm text-muted-foreground">
				{tr('Tulis entri baru di')} <code class="rounded bg-[var(--muted)] px-1.5 py-0.5 text-xs">web/static/changelog.md</code>
				{tr('(blok paling atas = terbaru), lalu klik Impor. Entri baru otomatis dikirim ke lonceng + web push klien & mitra.')}
			</p>
			<p class="mt-2 text-sm">
				<FileText size={14} class="inline" /> {parsed.length} {tr('entri terbaca')} ·
				<strong>{newEntries.length}</strong> {tr('baru')} ·
				<strong>{updatedEntries.length}</strong> {tr('sudah ada (akan diperbarui)')}
			</p>
			{#if sourceError}
				<p class="mt-2 flex items-start gap-2 text-sm" style="color: var(--warning);">
					<TriangleAlert size={16} class="mt-0.5 shrink-0" />{sourceError}
				</p>
			{/if}
		</div>
		<div class="flex flex-wrap items-center gap-2">
			<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => void loadSource()}>
				<RefreshCw size={15} /> {tr('Baca ulang file')}
			</button>
			<button class="app-btn app-btn-primary" type="button" disabled={busy || parsed.length === 0} onclick={() => void importEntries()}>
				{#if busy}<LoaderCircle size={15} class="animate-spin" />{:else}<CircleCheck size={15} />{/if}
				{tr('Impor entri')}
			</button>
		</div>
	</div>

	{#if newEntries.length > 0}
		<div class="mt-3 border-t pt-3">
			<p class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">{tr('Akan diimpor sebagai entri baru')}</p>
			<ul class="mt-1 grid gap-1 text-sm">
				{#each newEntries as entry (entry.source_key)}
					<li class="text-muted-foreground">• <strong class="text-foreground">{entry.title}</strong> — {entry.summary.slice(0, 90)}{entry.summary.length > 90 ? '…' : ''}</li>
				{/each}
			</ul>
		</div>
	{/if}
</section>

<section class="app-card mt-4 overflow-x-auto">
	<table class="w-full min-w-[820px] text-sm">
		<thead>
			<tr class="border-b text-left text-xs uppercase tracking-wide text-muted-foreground">
				<th class="px-4 py-2.5">{tr('Versi')}</th>
				<th class="px-4 py-2.5">{tr('Kategori')}</th>
				<th class="px-4 py-2.5">{tr('Judul')}</th>
				<th class="px-4 py-2.5">{tr('Tanggal')}</th>
				<th class="px-4 py-2.5">{tr('Status')}</th>
				<th class="px-4 py-2.5 text-right">{tr('Dibaca')}</th>
				<th class="px-4 py-2.5 text-right">{tr('Aksi')}</th>
			</tr>
		</thead>
		<tbody>
			{#each items as entry (entry.id)}
				<tr class="border-b last:border-0">
					<td class="px-4 py-3 font-semibold">{entry.version}</td>
					<td class="px-4 py-3"><Badge tone={categoryTone(entry.category)} label={categoryLabel(entry.category)} /></td>
					<td class="px-4 py-3">
						<p class="font-medium">{entry.title}</p>
						<p class="mt-0.5 max-w-[380px] truncate text-xs text-muted-foreground">{entry.summary}</p>
					</td>
					<td class="px-4 py-3 text-xs text-muted-foreground">{entry.released_on}</td>
					<td class="px-4 py-3">
						<Badge tone={entry.published ? 'success' : 'neutral'} label={entry.published ? tr('Terbit') : tr('Draf')} />
					</td>
					<td class="px-4 py-3 text-right tabular-nums">{entry.read_count}</td>
					<td class="px-4 py-3">
						<div class="flex items-center justify-end gap-2">
							<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" disabled={busy} onclick={() => void togglePublish(entry)}>
								{entry.published ? tr('Sembunyikan') : tr('Terbitkan')}
							</button>
							<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label={tr('Hapus entri')} disabled={busy} onclick={() => (confirmDelete = entry)}>
								<Trash size={15} />
							</button>
						</div>
					</td>
				</tr>
			{:else}
				<tr><td class="px-4 py-8 text-center text-sm text-muted-foreground" colspan="7">{tr('Belum ada entri tersimpan. Klik Impor entri.')}</td></tr>
			{/each}
		</tbody>
	</table>
</section>

{#if confirmDelete}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<div class="app-card w-full max-w-md p-5" role="dialog" aria-modal="true">
			<h2 class="font-semibold">{tr('Hapus entri changelog?')}</h2>
			<p class="mt-2 text-sm text-muted-foreground">
				<strong>{confirmDelete.title}</strong> {tr('akan dihapus dari dashboard semua pengguna. Entri yang masih ada di changelog.md akan muncul lagi bila diimpor ulang.')}
			</p>
			<div class="mt-4 flex justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => (confirmDelete = null)}>{tr('Batal')}</button>
				<button class="app-btn app-btn-primary" type="button" style="background-color: var(--destructive);" disabled={busy} onclick={() => void removeEntry()}>
					{busy ? tr('Menghapus…') : tr('Ya, hapus')}
				</button>
			</div>
		</div>
	</div>
{/if}
