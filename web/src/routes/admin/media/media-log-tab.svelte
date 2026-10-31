<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Tab "Log" di /admin/media — riwayat job pemeriksaan media.
	//
	// Sebelumnya pesan kegagalan (mis. pelanggaran CHECK constraint saat cek DA)
	// hanya tampil sebagai panel kuning di atas tabel direktori: memakan ruang
	// halaman kerja, hilang saat halaman dimuat ulang, dan tidak menyimpan
	// riwayat. Di sini riwayatnya utuh dan bisa disaring per status/jenis job.
	import { onMount } from 'svelte';
	import { CircleAlert, CircleCheck, Clock, RefreshCw } from '@lucide/svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import { errMessage, isAccessError, mountWithSession, platformRequest } from './_shared';

	type LogItem = {
		id: string;
		kind: string;
		status: string;
		attempts: number;
		max_attempts: number;
		last_error: string;
		media_id: string | null;
		media_name: string;
		website_url: string;
		created_at: string;
		updated_at: string;
	};

	type Summary = { failed: number; pending: number; running: number; completed: number; cancelled: number };

	// Ringkasan antrean cek trafik (khusus SimilarWeb) — dulu tampil sebagai teks
	// di atas tabel direktori dan mengganggu halaman kerja. Di sini tempatnya:
	// satu panel kecil di atas daftar log, hanya muncul bila ada antrean atau
	// kegagalan yang perlu diketahui.
	type TrafficStatus = {
		pending_jobs: number;
		running_jobs: number;
		failed_last_hour: number;
		last_error: string;
	};

	let items = $state<LogItem[]>([]);
	let summary = $state<Summary>({ failed: 0, pending: 0, running: 0, completed: 0, cancelled: 0 });
	let traffic = $state<TrafficStatus | null>(null);
	let loading = $state(true);
	let error = $state('');
	let statusFilter = $state('');
	let kindFilter = $state('');
	/** Baris yang pesan errornya dibuka penuh. */
	let expanded = $state<string | null>(null);

	let ready = $state(false);
	onMount(() => mountWithSession(async () => (ready = true)));

	$effect(() => {
		if (!ready) return;
		void statusFilter;
		void kindFilter;
		void load();
	});

	async function load() {
		error = '';
		loading = true;
		try {
			const params = new URLSearchParams({ limit: '200' });
			if (statusFilter) params.set('status', statusFilter);
			if (kindFilter) params.set('kind', kindFilter);
			const data = await platformRequest<{ items: LogItem[]; summary_24h: Summary }>(`/media/job-log?${params}`);
			items = data.items ?? [];
			summary = data.summary_24h ?? summary;
			// Status antrean trafik bersifat pelengkap: kegagalan endpoint ini
			// tidak boleh menggagalkan muatan log utama.
			try {
				traffic = await platformRequest<TrafficStatus>('/media/similarweb-status');
			} catch {
				traffic = null;
			}
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			loading = false;
		}
	}

	/** Nama job dalam bahasa manusia, bukan nama kind internal. */
	const KIND_LABEL: Record<string, string> = {
		media_seo_refresh: 'Cek DA & AI Trust',
		media_geo_refresh: 'Cek GEO saja',
		media_similarweb_refresh: 'Cek trafik',
		media_ai_summary: 'Kesimpulan AI',
		domain_verify: 'Verifikasi domain',
		crawl_target_sync: 'Sinkron target crawl'
	};
	const kindLabel = (k: string) => KIND_LABEL[k] ?? k;

	const statusLabel = (s: string) =>
		s === 'failed' ? tr('Gagal') : s === 'completed' ? tr('Selesai') : s === 'running' ? tr('Berjalan') : s === 'pending' ? tr('Menunggu') : s;
</script>

{#if error && !isAccessError(error)}
	<div class="app-card mt-4 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>
{/if}

<div class="mt-4 flex flex-wrap items-center gap-2">
	<div class="flex flex-wrap items-center gap-1.5 text-xs">
		{#if summary.failed > 0}
			<span class="inline-flex items-center gap-1 rounded-full px-2.5 py-1" style="background-color: var(--warning-soft); color: var(--warning);">
				<CircleAlert size={12} /> {summary.failed} {tr('gagal')}
			</span>
		{/if}
		{#if summary.pending + summary.running > 0}
			<span class="inline-flex items-center gap-1 rounded-full px-2.5 py-1" style="background-color: var(--accent); color: var(--accent-foreground);">
				<Clock size={12} /> {summary.pending + summary.running} {tr('dalam antrean')}
			</span>
		{/if}
		{#if summary.completed > 0}
			<span class="inline-flex items-center gap-1 rounded-full px-2.5 py-1" style="background-color: var(--success-soft, #eaf8f1); color: var(--success, #16794b);">
				<CircleCheck size={12} /> {summary.completed} {tr('selesai')}
			</span>
		{/if}
		<span class="text-muted-foreground">({tr('24 jam terakhir')})</span>
	</div>
	<div class="flex-1"></div>
	<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={loading} onclick={() => void load()}>
		<RefreshCw size={14} class={loading ? 'animate-spin' : ''} /> {loading ? tr('Memuat…') : tr('Muat ulang')}
	</button>
</div>

{#if traffic && (traffic.pending_jobs > 0 || traffic.failed_last_hour > 0 || traffic.last_error)}
	<div class="app-card mt-3 px-4 py-2.5 text-xs">
		<p class="font-medium">{tr('Antrean cek trafik (SimilarWeb)')}</p>
		<p class="mt-1 text-muted-foreground">
			{traffic.pending_jobs} {tr('menunggu')}{traffic.running_jobs
				? `, ${traffic.running_jobs} ${tr('berjalan')}`
				: ''}{traffic.failed_last_hour ? `, ${traffic.failed_last_hour} ${tr('gagal dalam sejam')}` : ''}.
			{#if traffic.last_error}
				<span class="block mt-1">{tr('Sebab gagal terakhir')}: {traffic.last_error}</span>
			{/if}
		</p>
	</div>
{/if}

<div class="mt-2 flex flex-wrap items-center gap-1 text-xs">
	<button
		type="button"
		class="app-btn !min-h-7 !px-2.5 !text-xs"
		class:app-btn-primary={statusFilter === ''}
		class:app-btn-soft={statusFilter !== ''}
		onclick={() => (statusFilter = '')}>{tr('Semua status')}</button
	>
	<button
		type="button"
		class="app-btn !min-h-7 !px-2.5 !text-xs"
		class:app-btn-primary={statusFilter === 'failed'}
		class:app-btn-soft={statusFilter !== 'failed'}
		onclick={() => (statusFilter = 'failed')}>{tr('Gagal')}</button
	>
	<button
		type="button"
		class="app-btn !min-h-7 !px-2.5 !text-xs"
		class:app-btn-primary={statusFilter === 'completed'}
		class:app-btn-soft={statusFilter !== 'completed'}
		onclick={() => (statusFilter = 'completed')}>{tr('Selesai')}</button
	>
	<button
		type="button"
		class="app-btn !min-h-7 !px-2.5 !text-xs"
		class:app-btn-primary={statusFilter === 'pending'}
		class:app-btn-soft={statusFilter !== 'pending'}
		onclick={() => (statusFilter = 'pending')}>{tr('Menunggu')}</button
	>
	<span class="mx-1 text-muted-foreground">·</span>
	<button
		type="button"
		class="app-btn !min-h-7 !px-2.5 !text-xs"
		class:app-btn-primary={kindFilter === ''}
		class:app-btn-soft={kindFilter !== ''}
		onclick={() => (kindFilter = '')}>{tr('Semua jenis')}</button
	>
	{#each Object.entries(KIND_LABEL) as [value, label] (value)}
		<button
			type="button"
			class="app-btn !min-h-7 !px-2.5 !text-xs"
			class:app-btn-primary={kindFilter === value}
			class:app-btn-soft={kindFilter !== value}
			onclick={() => (kindFilter = value)}>{label}</button
		>
	{/each}
</div>

{#if loading && items.length === 0}
	<p class="mt-6 text-sm text-muted-foreground">{tr('Memuat log…')}</p>
{:else if items.length === 0}
	<div class="app-card mt-4 px-6 py-10 text-center">
		<p class="text-sm font-medium">{tr('Belum ada log')}</p>
		<p class="mt-1 text-xs text-muted-foreground">
			{tr('Job pemeriksaan media (cek DA, trafik, verifikasi domain) akan tercatat di sini.')}
		</p>
	</div>
{:else}
	<div class="app-card mt-4 overflow-x-auto">
		<table class="w-full text-sm" style="min-width: 1000px;">
			<thead>
				<tr class="text-left text-xs text-muted-foreground">
					<th class="px-4 py-2.5 font-medium">{tr('Waktu')}</th>
					<th class="px-4 py-2.5 font-medium">{tr('Jenis')}</th>
					<th class="px-4 py-2.5 font-medium">{tr('Media')}</th>
					<th class="px-4 py-2.5 font-medium">{tr('Status')}</th>
					<th class="px-4 py-2.5 font-medium">{tr('Percobaan')}</th>
					<th class="px-4 py-2.5 font-medium min-w-[320px]">{tr('Pesan')}</th>
				</tr>
			</thead>
			<tbody>
				{#each items as it (it.id)}
					<tr class="border-t align-top">
						<td class="px-4 py-2.5 whitespace-nowrap text-xs text-muted-foreground">{fmtDateTime(it.updated_at)}</td>
						<td class="px-4 py-2.5 whitespace-nowrap text-xs">{kindLabel(it.kind)}</td>
						<td class="px-4 py-2.5 text-xs">
							{#if it.media_name || it.website_url}
								<p class="font-medium">{it.media_name || '—'}</p>
								{#if it.website_url}<p class="text-muted-foreground">{it.website_url}</p>{/if}
							{:else}
								<span class="text-muted-foreground">—</span>
							{/if}
						</td>
						<td class="px-4 py-2.5 whitespace-nowrap">
							<span
								class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold"
								style={it.status === 'failed'
									? 'background-color: var(--warning-soft); color: var(--warning);'
									: it.status === 'completed'
										? 'background-color: var(--success-soft, #eaf8f1); color: var(--success, #16794b);'
										: 'background-color: var(--muted); color: var(--muted-foreground);'}
							>{statusLabel(it.status)}</span>
						</td>
						<td class="px-4 py-2.5 text-xs tabular-nums text-muted-foreground">{it.attempts}/{it.max_attempts}</td>
						<td class="px-4 py-2.5 text-xs">
							{#if it.last_error}
								<button
									type="button"
									class="text-left"
									style="color: var(--destructive);"
									title={expanded === it.id ? tr('Sembunyikan') : tr('Tampilkan pesan lengkap')}
									onclick={() => (expanded = expanded === it.id ? null : it.id)}
								>
									<span class={expanded === it.id ? 'whitespace-pre-wrap break-words' : 'line-clamp-2 break-words'}>
										{it.last_error}
									</span>
								</button>
							{:else}
								<span class="text-muted-foreground">—</span>
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
	<p class="mt-2 text-xs text-muted-foreground">{items.length} {tr('baris ditampilkan (maks 200)')}</p>
{/if}
