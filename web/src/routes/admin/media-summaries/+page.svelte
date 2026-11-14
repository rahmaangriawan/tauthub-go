<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Monitoring Kesimpulan Website — daftar event tracking (queued/success/failed)
	// beserta metrik ringkas. Port apps/web/src/app/admin/media-summaries/page.tsx.
	import { onMount } from 'svelte';
	import { Pencil, Search } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import CopyMarkdownButton from '$lib/components/admin/copy-markdown-button.svelte';
	import { mdReport, mdTable, mdList } from '$lib/copy-markdown';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const LIMIT = 50;
	const SUMMARY_MAX_CHARS = 2000;
	const SOURCE_MAX_CHARS = 60000;

	interface SummaryEvent {
		id: string;
		media_id: string;
		media_name: string | null;
		website_url: string | null;
		status: string;
		source?: string;
		pages_tried: string | Array<{ url: string; ok: boolean; chars: number; note?: string }>;
		error_message: string | null;
		attempt: number;
		created_at: string;
		ai_summary?: string | null;
		ai_summary_generated_at?: string | null;
		media_has_summary?: boolean;
	}
	interface Metrics {
		queued_events: number;
		success: number;
		failed: number;
		jobs_pending: number;
		jobs_running: number;
		jobs_failed: number;
	}
	type Page = { total: number; items: SummaryEvent[]; metrics?: Metrics };

	function parsePages(raw: SummaryEvent['pages_tried']): Array<{ url: string; ok: boolean; chars: number; note?: string }> {
		let parsed: unknown = raw;
		if (typeof raw === 'string') {
			try {
				parsed = JSON.parse(raw);
			} catch {
				return [];
			}
		}
		if (!Array.isArray(parsed)) return [];
		return parsed.filter(
			(p) => p && typeof p === 'object' && typeof (p as { url?: unknown }).url === 'string'
		) as Array<{ url: string; ok: boolean; chars: number; note?: string }>;
	}

	function statusLabel(status: string, pages: Array<{ ok: boolean }>, errorMessage?: string | null) {
		if (status === 'manual') return tr('Manual');
		if (status === 'success') {
			// Semua halaman gagal ditarik → kesimpulan dibuat dari data mitra (fallback).
			if (pages.length > 0 && pages.every((p) => !p.ok)) return tr('Sukses · Data Mitra');
			return 'Sukses';
		}
		if (status === 'failed') return tr('Gagal');
		// Crawl VPS ditolak WAF → request sudah dibuat untuk worker lokal
		// (komputer admin menarik halamannya lewat IP ISP).
		if ((errorMessage ?? '').includes('worker lokal')) return tr('Menunggu worker lokal');
		return tr('Menunggu Antrean');
	}

	function statusBadgeTone(status: string, pages: Array<{ ok: boolean }>, errorMessage?: string | null): 'neutral' | 'accent' | 'success' | 'warning' | 'danger' {
		if (status === 'manual') return 'accent';
		if (status === 'success') return pages.length > 0 && pages.every((p) => !p.ok) ? 'warning' : 'success';
		if (status === 'failed') return 'danger';
		if ((errorMessage ?? '').includes('worker lokal')) return 'warning';
		return 'neutral';
	}

	let data = $state<Page>({ total: 0, items: [] });
	let offset = $state(0);
	let loading = $state(true);
	let error = $state('');
	let search = $state('');
	let status = $state('');
	let busyId = $state<string | null>(null);
	let notice = $state('');

	// Modal "Isi manual / AI": satu media = satu sesi edit.
	let editItem = $state<SummaryEvent | null>(null);
	let sourceText = $state('');
	let summaryDraft = $state('');
	let editedByAI = $state(false);
	let draftBusy = $state(false);
	let saveBusy = $state(false);
	let editError = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load(0, '', '');
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load(nextOffset: number, nextStatus: string, nextSearch: string) {
		loading = true;
		error = '';
		const query = new URLSearchParams({ offset: String(nextOffset), limit: String(LIMIT) });
		if (nextStatus) query.set('status', nextStatus);
		if (nextSearch) query.set('search', nextSearch);
		try {
			const res = await apiRequest(`/v1/platform/media-summaries?${query}`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const page = await res.json();
			data = { total: page.total ?? 0, items: page.items ?? [], metrics: page.metrics };
			offset = nextOffset;
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	function submitFilters(event: SubmitEvent) {
		event.preventDefault();
		notice = '';
		void load(0, status, search.trim());
	}

	async function regenerate(mediaId: string) {
		busyId = mediaId;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/platform/media-summaries/${mediaId}/regenerate`, { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			notice = tr('Regenerasi dijadwalkan. Kesimpulan akan diperbarui lewat antrean global AI.');
			await load(offset, status, search.trim());
		} finally {
			busyId = null;
		}
	}

	// ---------- Modal Isi manual / AI ----------

	function openEdit(item: SummaryEvent) {
		editItem = item;
		sourceText = '';
		summaryDraft = item.ai_summary?.trim() ?? '';
		editedByAI = false;
		editError = '';
	}

	function closeEdit() {
		if (!draftBusy && !saveBusy) editItem = null;
	}

	/** Minta AI merangkum teks yang ditempel (atau crawl ulang bila kosong). */
	async function draftWithAI() {
		if (!editItem) return;
		const pasted = sourceText.trim();
		if (pasted.length > 0 && pasted.length < 80) {
			editError = tr('Teks sumber terlalu pendek untuk diringkas (minimal 80 karakter). Tempel lebih banyak isi halaman.');
			return;
		}
		draftBusy = true;
		editError = '';
		try {
			// Timeout panjang (2 menit): AI butuh 10-60 detik. Timeout bawaan
			// apiRequest 12 detik — justru itu penyebab "Ringkas dengan AI"
			// selalu mati di tengah jalan (context canceled) sebelum hasil
			// sampai ke Kesimpulan akhir.
			const res = await apiRequest(
				`/v1/platform/media-summaries/${editItem.media_id}/draft`,
				{ method: 'POST', body: JSON.stringify({ text: sourceText.trim() }) },
				120_000
			);
			if (!res.ok) {
				editError = await readApiError(res, 'Gagal merangkum dengan AI.');
				return;
			}
			const data = (await res.json()) as { summary?: string };
			if (!data.summary) {
				editError = tr('AI tidak mengembalikan hasil. Coba lagi dengan teks sumber lebih panjang.');
				return;
			}
			summaryDraft = data.summary;
			editedByAI = true;
		} finally {
			draftBusy = false;
		}
	}

	/** Simpan kesimpulan akhir ke media (status event: manual). */
	async function saveManual() {
		if (!editItem) return;
		const text = summaryDraft.trim();
		if (text.length < 40) {
			editError = tr('Kesimpulan minimal 40 karakter.');
			return;
		}
		if (text.length > SUMMARY_MAX_CHARS) {
			editError = tr('Kesimpulan maksimal 2000 karakter.');
			return;
		}
		saveBusy = true;
		editError = '';
		try {
			const res = await apiRequest(`/v1/platform/media-summaries/${editItem.media_id}/manual`, {
				method: 'POST',
				body: JSON.stringify({ summary: text, source: editedByAI ? 'ai' : 'manual' })
			});
			if (!res.ok) {
				editError = await readApiError(res, 'Gagal menyimpan kesimpulan.');
				return;
			}
			editItem = null;
			notice = tr('Kesimpulan tersimpan dan langsung dipakai katalog, portal mitra, serta matchmaker.');
			await load(offset, status, search.trim());
		} finally {
			saveBusy = false;
		}
	}

	const metrics = $derived([
		{ label: 'Sukses', value: data.metrics?.success ?? 0, color: 'var(--success)' },
		{
			label: tr('Gagal'),
			value: data.metrics?.failed ?? 0,
			color: (data.metrics?.failed ?? 0) > 0 ? 'var(--destructive)' : 'var(--muted-foreground)'
		},
		{ label: tr('Menunggu Antrean'), value: data.metrics?.queued_events ?? 0, color: undefined },
		{
			label: tr('Job Aktif'),
			value: (data.metrics?.jobs_pending ?? 0) + (data.metrics?.jobs_running ?? 0),
			color: undefined
		},
		{
			label: tr('Job Gagal Permanen'),
			value: data.metrics?.jobs_failed ?? 0,
			color: (data.metrics?.jobs_failed ?? 0) > 0 ? 'var(--destructive)' : 'var(--muted-foreground)'
		}
	]);

	// Salin halaman event yang sedang tampil sebagai Markdown, lengkap dengan
	// halaman yang ditarik dan pesan error utuh untuk keperluan debug.
	function toMarkdown(): string {
		const rows = data.items.map((e) => {
			const pages = parsePages(e.pages_tried);
			return [
				e.media_name ?? e.media_id,
				e.website_url,
				statusLabel(e.status, pages, e.error_message),
				e.source,
				e.attempt,
				fmtDateTime(e.created_at),
				pages.length ? `${pages.filter((p) => p.ok).length}/${pages.length}` : '-',
				e.error_message
			];
		});
		const details = data.items
			.map((e) => {
				const pages = parsePages(e.pages_tried);
				if (pages.length === 0 && !e.error_message) return '';
				const lines = pages.map((p) => `- ${p.ok ? 'OK' : 'GAGAL'} ${p.url} (${p.chars} karakter)${p.note ? ` ${p.note}` : ''}`);
				if (e.error_message) lines.push(`- Error: ${e.error_message}`);
				return `### ${e.media_name ?? e.media_id} (${e.media_id})\n${lines.join('\n')}`;
			})
			.filter(Boolean)
			.join('\n\n');
		return mdReport(
			'Monitoring Kesimpulan Website',
			mdList([
				...metrics.map((m): [string, unknown] => [m.label, m.value]),
				['Total cocok', data.total],
				['Ditampilkan', data.items.length ? `${offset + 1}-${offset + data.items.length}` : '0'],
				['Pencarian', search],
				['Status', status]
			]),
			mdTable(['Media', 'Website', 'Status', 'Sumber', 'Attempt', 'Waktu', 'Halaman OK', 'Error'], rows),
			details ? `## Rincian\n\n${details}` : ''
		);
	}
</script>

{#snippet statusCell(item: SummaryEvent)}
	{@const pages = parsePages(item.pages_tried)}
	<Badge tone={statusBadgeTone(item.status, pages, item.error_message)} label={statusLabel(item.status, pages, item.error_message)} />
{/snippet}

{#snippet mediaCell(item: SummaryEvent)}
	<div class="max-w-[320px]">
		<p class="font-medium">{item.media_name ?? item.media_id.slice(0, 8)}</p>
		{#if item.website_url}
			<p class="text-xs text-muted-foreground">
				{item.website_url.replace(/^https?:\/\/(www\.)?/, '').replace(/\/$/, '')}
			</p>
		{/if}
		<p class="text-xs text-muted-foreground">{fmtDateTime(item.created_at)}</p>
	</div>
{/snippet}

{#snippet pagesCell(item: SummaryEvent)}
	{@const pages = parsePages(item.pages_tried)}
	{@const okPages = pages.filter((p) => p.ok)}
	<div class="max-w-[300px]">
		<p class="text-xs" style={pages.length > 0 && okPages.length === 0 ? 'color: var(--destructive);' : undefined}>
			{pages.length > 0 ? `${okPages.length}/${pages.length} halaman` : '—'}
		</p>
		{#if okPages.length > 0}
			<p
				class="max-w-[280px] truncate text-xs text-muted-foreground"
				title={okPages.map((p) => p.url.replace(/^https?:\/\/(www\.)?/, '')).join(', ')}
			>
				{okPages.map((p) => p.url.replace(/^https?:\/\/(www\.)?/, '').replace(/\/$/, '')).join(', ')}
			</p>
		{/if}
		{#if pages.length > 0 && okPages.length === 0}
			<p
				class="max-w-[280px] truncate text-xs"
				style="color: var(--destructive);"
				title={pages[0]?.note ?? 'Web tidak terjangkau'}
			>
				{pages[0]?.note ?? 'Web tidak terjangkau'}
			</p>
		{/if}
	</div>
{/snippet}

{#snippet errorCell(item: SummaryEvent)}
	<div class="max-w-[240px]">
		{#if item.error_message}
			<p class="truncate text-xs" style="color: var(--destructive);" title={item.error_message}>{item.error_message}</p>
		{:else}
			<p class="text-xs text-muted-foreground">—</p>
		{/if}
		<p class="text-xs text-muted-foreground">{item.attempt} percobaan</p>
	</div>
{/snippet}

{#snippet regenCell(item: SummaryEvent)}
	<div class="flex items-center justify-end gap-1.5">
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			disabled={busyId !== null}
			title={tr("Buka modal isi kesimpulan manual atau ringkas dengan AI")}
			onclick={() => openEdit(item)}
		><Pencil size={13} />{tr('Isi manual / AI')}</button>
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			disabled={busyId !== null}
			title={tr("Hapus kesimpulan lama dan antrekan penerbitan ulang")}
			onclick={() => void regenerate(item.media_id)}
			>{busyId === item.media_id ? 'Mengantrekan…' : 'Generate ulang'}</button
		>
	</div>
{/snippet}

<section>
	<PageHeader
		title="Monitoring Kesimpulan Website"
		description={tr("Tracking AI Tauthub saat menarik data web untuk profil Kesimpulan. Sukses dan gagal semuanya tercatat di sini.")}
	>
		<CopyMarkdownButton build={toMarkdown} disabled={loading || data.items.length === 0} />
	</PageHeader>

	<Feedback {loading} {error} {notice} />

	<div class="mt-4 flex flex-wrap gap-3">
		{#each metrics as m (m.label)}
			<div class="app-card min-w-[140px] flex-1 p-3">
				<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{m.label}</p>
				<p class="text-2xl font-bold" style={m.color ? `color: ${m.color};` : undefined}>{m.value}</p>
			</div>
		{/each}
	</div>

	<form class="mt-4 flex flex-wrap items-center gap-2" onsubmit={submitFilters}>
		<label class="app-input flex w-[280px] items-center gap-2 !py-1.5">
			<Search size={14} class="text-muted-foreground" />
			<input class="w-full bg-transparent text-sm outline-none" placeholder={tr("Cari nama media atau website…")} bind:value={search} />
		</label>
		<input class="app-input" style="width: 220px;" placeholder={tr("Sukses / gagal / queued")} bind:value={status} />
		<button type="submit" class="app-btn app-btn-soft">Terapkan</button>
		<button
			type="button"
			class="app-btn app-btn-soft"
			onclick={() => {
				search = '';
				status = '';
				notice = '';
				void load(0, '', '');
			}}>Reset</button
		>
	</form>

	<DataTable
		columns={[
			{ key: 'media', label: tr('Media'), render: mediaCell },
			{ key: 'status', label: 'Status', render: statusCell },
			{ key: 'pages', label: tr('Halaman Ditarik'), render: pagesCell },
			{ key: 'error', label: 'Error', render: errorCell },
			{ key: 'actions', label: 'Aksi', align: 'right' as const, render: regenCell }
		]}
		rows={data.items}
		rowKey={(item: SummaryEvent) => item.id}
		{loading}
		empty={tr("Belum ada event tracking yang cocok.")}
		minWidth="1150px"
	/>

	<Pagination total={data.total} limit={LIMIT} {offset} onChange={(next) => void load(next, status, search.trim())} />

	<p class="mt-4 text-xs text-muted-foreground">{tr("Kesimpulan dibuat otomatis oleh AI lewat antrean global. Gagal diproses otomatis dicoba ulang oleh sistem; job yang gagal permanen terlihat pada kartu Job Gagal Permanen.")}</p>
	<p class="mt-1 text-xs text-muted-foreground">{tr("Untuk situs yang menolak crawler (HTTP 403/429, DNS gagal), tempel isi halamannya di modal Isi manual / AI lalu minta AI merangkumnya.")}</p>
</section>

{#if editItem}
	<Modal
		title={tr('Isi kesimpulan: {nama}').replace('{nama}', editItem.media_name ?? editItem.media_id.slice(0, 8))}
		label={tr('Isi kesimpulan manual / AI')}
		maxWidth="lg"
		onClose={closeEdit}
	>
		<div class="space-y-3">
			<p class="text-xs text-muted-foreground">
				{editItem.website_url ?? tr('Tanpa website')} ·
				{editItem.media_has_summary
					? tr('Sudah punya kesimpulan') + (editItem.ai_summary_generated_at ? ` · ${fmtDateTime(editItem.ai_summary_generated_at)}` : '')
					: tr('Belum punya kesimpulan')}
			</p>

			<label class="block">
				<span class="flex items-center justify-between text-sm font-medium">
					{tr('Teks sumber (opsional)')}
					<span class="text-xs font-normal text-muted-foreground">{sourceText.trim().length}/{SOURCE_MAX_CHARS}</span>
				</span>
				<span class="block text-xs text-muted-foreground">
					{tr('Tempel isi halaman website (About/Homepage) bila situs menolak crawler (HTTP 403/429, DNS gagal). Kosongkan untuk mencoba crawl otomatis. AI merangkum teks DI KOLOM INI, bukan yang ada di Kesimpulan akhir.')}
				</span>
				<textarea
					class="app-input mt-1 w-full font-mono text-xs"
					rows="6"
					bind:value={sourceText}
					maxlength={SOURCE_MAX_CHARS}
				></textarea>
			</label>

			<button
				class="app-btn app-btn-soft"
				type="button"
				disabled={draftBusy || saveBusy}
				title={tr('Merangkum isi kolom Teks sumber di atas menjadi satu paragraf kesimpulan.')}
				onclick={() => void draftWithAI()}
			>
				{draftBusy ? tr('Merangkum dengan AI… (bisa sampai 1 menit)') : tr('Ringkas teks sumber dengan AI')}
			</button>

			<label class="block">
				<span class="flex items-center justify-between text-sm font-medium">
					{tr('Kesimpulan akhir')}
					<span class="text-xs font-normal text-muted-foreground">{summaryDraft.trim().length}/{SUMMARY_MAX_CHARS}</span>
				</span>
				<textarea
					class="app-input mt-1 w-full"
					rows="8"
					bind:value={summaryDraft}
					maxlength={SUMMARY_MAX_CHARS}
					placeholder={tr('Hasil AI muncul di sini dan bisa kamu edit, atau tulis sendiri dari awal.')}
				></textarea>
			</label>

			{#if editError}
				<p class="text-sm" style="color: var(--destructive);">{editError}</p>
			{/if}

			<p class="text-xs text-muted-foreground">
				{tr('Kesimpulan tersimpan langsung dipakai di katalog publik, portal mitra, dan konteks matchmaker. Job crawl otomatis tidak akan menimpanya.')}
			</p>
		</div>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" disabled={draftBusy || saveBusy} onclick={closeEdit}>{tr('Batal')}</button>
			<button class="app-btn app-btn-primary" type="button" disabled={draftBusy || saveBusy} onclick={() => void saveManual()}>
				{saveBusy ? tr('Menyimpan…') : tr('Simpan kesimpulan')}
			</button>
		{/snippet}
	</Modal>
{/if}
