<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Diagnostik Verifikasi Domain — menjawab "kenapa media ini gagal verifikasi"
	// dari sisi admin. Port apps/web/src/app/admin/domain-diagnostics/page.tsx.
	import { onMount } from 'svelte';
	import { Globe, RefreshCw, Search, TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const PAGE_SIZE = 20;

	type DiagnosticRow = {
		media_id: string;
		name: string;
		website_url: string;
		partner_name: string | null;
		domain_verified: boolean;
		domain_health: string;
		domain_check_fail_count: number;
		domain_last_checked_at: string | null;
		domain_last_result: string | null;
		domain_last_error: string | null;
		domain_last_error_at: string | null;
		domain_last_stage: string | null;
		domain_last_trace: string | null;
	};

	type Summary = {
		unreachable: number;
		blocked_by_waf: number;
		token_not_found: number;
		token_mismatch: number;
		verified: number;
		never_checked: number;
	};

	/** Sebab kegagalan → label + tone, satu sumber untuk kartu dan tabel. */
	const RESULT_META: Record<
		string,
		{ label: string; tone: 'neutral' | 'accent' | 'success' | 'warning' | 'danger' }
	> = {
		verified: { label: 'Terverifikasi', tone: 'success' },
		blocked_by_waf: { label: 'Diblokir WAF mitra', tone: 'warning' },
		token_not_found: { label: 'Kode tidak ditemukan', tone: 'warning' },
		token_mismatch: { label: 'Kode lama terpasang', tone: 'warning' },
		unreachable: { label: 'Situs tak terjangkau', tone: 'danger' }
	};

	/**
	 * Bagian mana yang gagal → kalimat + saran tindakan. Ini yang membuat
	 * diagnostik bisa ditindaklanjuti.
	 */
	const STAGE_META: Record<string, { label: string; hint: string }> = $derived({
		no_website: { label: 'Alamat kosong', hint: 'Media belum mengisi Website/Domain. Minta mitra melengkapinya.' },
		dns: {
			label: tr('Gagal di DNS'),
			hint: 'Nama domain tidak bisa diterjemahkan ke IP. Domain salah ketik, kedaluwarsa, atau DNS-nya bermasalah.'
		},
		connect: {
			label: tr('Gagal menyambung'),
			hint: 'Alamat ketemu tapi koneksi/TLS tidak terbentuk. Situs mati, port tertutup, atau koneksi server diblokir.'
		},
		http: {
			label: 'Server menolak',
			hint: 'Tersambung, tapi server mitra membalas error. Sering 403 dari Cloudflare/WAF yang memblokir akses dari IP server.'
		},
		waf: {
			label: 'Diblokir WAF mitra',
			hint: 'Situs hidup, tapi WAF/Cloudflare mitra menolak akses dari IP server (403). Tidak dihitung gagal — verifikasi bisa lewat DNS TXT.'
		},
		txt: {
			label: 'Kode via DNS TXT',
			hint: 'Token ditemukan di TXT record _tauthub-verify.<domain>, bukan di meta tag halaman.'
		},
		bot_challenge: {
			label: tr('Halaman challenge bot'),
			hint: 'WAF mengirim halaman verifikasi bot, bukan isi situs. Akses dari IP server ditantang.'
		},
		token: { label: 'Kode verifikasi', hint: 'Halaman terbaca normal. Masalahnya di kode: belum dipasang, atau masih kode lama.' },
		ok: { label: tr('Berhasil'), hint: 'Kode ditemukan di halaman.' }
	});

	let rows = $state<DiagnosticRow[]>([]);
	let summary = $state<Summary | null>(null);
	let error = $state('');
	let notice = $state('');
	let loading = $state(true);
	let filterResult = $state('');
	let query = $state('');
	let appliedQuery = $state('');
	let busyId = $state<string | null>(null);
	let page = $state(1);

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
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams({ limit: '500' });
			if (filterResult) params.set('result', filterResult);
			if (appliedQuery) params.set('q', appliedQuery);
			const [listRes, sumRes] = await Promise.all([
				apiRequest(`/v1/platform/domain-diagnostics?${params}`),
				apiRequest('/v1/platform/domain-diagnostics?view=summary')
			]);
			if (!listRes.ok) {
				error = await readApiError(listRes, 'Permintaan gagal.');
				return;
			}
			const listData = await listRes.json();
			rows = listData.items ?? [];
			if (sumRes.ok) summary = await sumRes.json();
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	/** Cek ulang sekarang — hasilnya sekaligus menyegarkan sebab terakhir di tabel. */
	async function recheck(row: DiagnosticRow) {
		busyId = row.media_id;
		error = '';
		notice = '';
		try {
			const res = await apiRequest('/v1/platform/domain-diagnostics/recheck', {
				method: 'POST',
				body: JSON.stringify({ media_id: row.media_id })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const result = await res.json();
			notice = `Cek ulang ${row.name}: ${RESULT_META[result.status]?.label ?? result.status}${result.reason ? ` — ${result.reason}` : ''}`;
			await load();
		} finally {
			busyId = null;
		}
	}

	/** Recheck massal: semua media ber-website masuk antrean domain_verify. */
	let recheckingAll = $state(false);

	async function recheckAll() {
		if (recheckingAll) return;
		if (!window.confirm('Cek ulang SEMUA media ber-website sekarang? Pemeriksaan berjalan lewat antrean latar belakang dan hasilnya muncul bertahap saat halaman disegarkan.')) return;
		recheckingAll = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest('/v1/platform/domain-diagnostics/recheck-all', { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const data = (await res.json()) as { enqueued?: number };
			notice = `${data.enqueued ?? 0} media masuk antrean pemeriksaan. Segarkan halaman sesekali untuk melihat hasil terbaru.`;
			await load();
		} finally {
			recheckingAll = false;
		}
	}

	const pageRows = $derived(rows.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE));
	const showAccess = $derived(rows.length === 0 && error.includes('Akses platform'));
</script>

{#snippet mediaCell(row: DiagnosticRow)}
	<div>
		<p class="font-medium">{row.name}</p>
		<p class="text-xs text-muted-foreground">{row.website_url}</p>
	</div>
{/snippet}

{#snippet partnerCell(row: DiagnosticRow)}
	<span>{row.partner_name ?? '—'}</span>
{/snippet}

{#snippet reasonCell(row: DiagnosticRow)}
	{@const meta = row.domain_last_result ? RESULT_META[row.domain_last_result] : null}
	{@const stage = row.domain_last_stage ? STAGE_META[row.domain_last_stage] : null}
	<div>
		{#if meta}
			<Badge tone={meta.tone} label={meta.label} />
		{:else}
			<Badge tone="neutral" label={tr("Belum pernah dicek")} />
		{/if}
		{#if stage}
			<p class="mt-1 text-xs text-muted-foreground" title={stage.hint}>{tr("Gagal di:")}<strong>{stage.label}</strong></p>
		{/if}
		{#if row.domain_check_fail_count > 0}
			<p class="mt-1 text-xs text-muted-foreground">{tr("Gagal")} {row.domain_check_fail_count}× berturut-turut</p>
		{/if}
	</div>
{/snippet}

{#snippet errorDetailCell(row: DiagnosticRow)}
	{@const stage = row.domain_last_stage ? STAGE_META[row.domain_last_stage] : null}
	<div class="max-w-[360px]">
		{#if row.domain_last_error}
			<p class="break-words text-xs">{row.domain_last_error}</p>
		{:else}
			<p class="text-xs text-muted-foreground">—</p>
		{/if}
		{#if stage}
			<p class="mt-1 text-xs text-muted-foreground">{stage.hint}</p>
		{/if}
		{#if row.domain_last_trace}
			<details class="mt-1.5 max-w-[420px]">
				<summary class="cursor-pointer text-[11px] text-muted-foreground"
					>Jejak langkah ({row.domain_last_trace.split('\n').length})</summary
				>
				<pre
					class="mt-1.5 whitespace-pre-wrap break-words rounded-md p-2 text-[11px] leading-relaxed"
					style="background-color: var(--secondary);">{row.domain_last_trace}</pre>
			</details>
		{/if}
	</div>
{/snippet}

{#snippet checkedCell(row: DiagnosticRow)}
	<span class="text-xs text-muted-foreground">{fmtDateTime(row.domain_last_error_at ?? row.domain_last_checked_at)}</span>
{/snippet}

{#snippet recheckCell(row: DiagnosticRow)}
	<button
		class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
		disabled={busyId === row.media_id}
		onclick={() => void recheck(row)}
	>
		<Globe size={14} />
		{busyId === row.media_id ? 'Memeriksa…' : 'Cek Ulang'}
	</button>
{/snippet}

<section class="w-full">
	<PageHeader
		title="Diagnostik Verifikasi Domain"
		description={tr("Sebab kegagalan verifikasi domain mitra, tersimpan dari hasil pengecekan terakhir. Pakai ini untuk membedakan situs yang mati, situs yang diblokir, dan kode verifikasi yang salah tempel. Default hanya menampilkan domain yang belum terverifikasi.")}
	>
		<button class="app-btn app-btn-soft" onclick={load} disabled={loading}>
			<RefreshCw size={15} /> Segarkan
		</button>
		<button class="app-btn app-btn-primary" onclick={() => void recheckAll()} disabled={recheckingAll}>
			<Globe size={15} /> {recheckingAll ? 'Memasukkan antrean…' : 'Recheck Semua'}
		</button>
	</PageHeader>

	{#if summary}
		<div class="mt-4 flex flex-wrap gap-3">
			{#each [['blocked_by_waf', 'Diblokir WAF mitra'], ['unreachable', 'Situs tak terjangkau'], ['token_mismatch', 'Kode lama terpasang'], ['token_not_found', 'Kode tidak ditemukan'], ['never_checked', tr('Belum pernah dicek')]] as [key, label] (key)}
				<div class="app-card min-w-[190px] p-3">
					<p class="text-xs text-muted-foreground">{label}</p>
					<p class="text-2xl font-bold">{summary[key as keyof Summary]}</p>
				</div>
			{/each}
		</div>
	{/if}

	<div class="mt-4 flex flex-wrap items-end gap-3">
		<label class="block" style="min-width: 220px;">
			<span class="mb-1 block text-xs text-muted-foreground">Sebab terakhir</span>
			<Select
				ariaLabel="Sebab terakhir"
				minWidth={220}
				value={filterResult || 'all'}
				onchange={(v) => {
					filterResult = v === 'all' ? '' : v;
					page = 1;
					void load();
				}}
				options={[
					{ value: 'all', label: 'Perlu perhatian' },
					{ value: 'blocked_by_waf', label: 'Diblokir WAF mitra' },
					{ value: 'unreachable', label: 'Situs tak terjangkau' },
					{ value: 'token_mismatch', label: 'Kode lama terpasang' },
					{ value: 'token_not_found', label: 'Kode tidak ditemukan' },
					{ value: 'verified', label: 'Terverifikasi' }
				]}
			/>
		</label>
		<label class="block min-w-[240px] flex-1">
			<span class="mb-1 block text-xs text-muted-foreground">{tr("Cari media, domain, atau mitra")}</span>
			<span class="app-input flex items-center gap-2 !py-1.5">
				<Search size={15} class="text-muted-foreground" />
				<input
					class="w-full bg-transparent text-sm outline-none"
					placeholder={tr("mis. tauthub.com")}
					bind:value={query}
					onkeydown={(e) => {
						if (e.key === 'Enter') {
							appliedQuery = query.trim();
							page = 1;
							void load();
						}
					}}
				/>
			</span>
		</label>
		<button
			class="app-btn app-btn-soft"
			onclick={() => {
				appliedQuery = query.trim();
				page = 1;
				void load();
			}}>Terapkan</button
		>
	</div>

	<Feedback {loading} {error} {notice} />

	{#if showAccess}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke diagnostik domain.")}</p>
		</div>
	{:else if loading && rows.length === 0}
		<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat diagnostik…")}</p>
	{:else if rows.length === 0}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Tidak ada domain yang perlu perhatian. Semua media mitra yang punya website sudah terverifikasi.")}</p>
		</div>
	{:else}
		<DataTable
			columns={[
				{ key: 'media', label: tr('Media'), render: mediaCell },
				{ key: 'partner', label: tr('Mitra'), render: partnerCell },
				{ key: 'reason', label: 'Sebab terakhir', render: reasonCell },
				{ key: 'error', label: tr('Detail error'), render: errorDetailCell },
				{ key: 'checked', label: 'Dicek', render: checkedCell },
				{ key: 'actions', label: 'Aksi', render: recheckCell }
			]}
			rows={pageRows}
			rowKey={(row: DiagnosticRow) => row.media_id}
			empty={tr("Tidak ada domain yang perlu perhatian.")}
			minWidth="1200px"
		/>
		<Pagination
			total={rows.length}
			limit={PAGE_SIZE}
			offset={(page - 1) * PAGE_SIZE}
			onChange={(next) => (page = next / PAGE_SIZE + 1)}
		/>
	{/if}

	<p class="mt-4 flex items-start gap-1.5 text-xs text-muted-foreground">
		<TriangleAlert size={13} class="mt-0.5 shrink-0" />{tr("Sebab berasal dari pengecekan terakhir (manual oleh mitra, admin, atau sweep mingguan). Tombol Cek Ulang memeriksa saat itu juga tanpa menunggu jadwal.")}</p>
</section>
