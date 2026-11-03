<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Antrean tiket dukungan mitra. Semua admin boleh membalas (tanpa
	// permission khusus). Balasan disimpan sebagai pesan admin dan otomatis
	// memindahkan tiket "buka" ke "diproses"; backend juga mengirim lonceng,
	// email, dan WhatsApp ke mitra.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import MultilineText from '$lib/components/multiline-text.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';
	import { Send, ArrowDown, ArrowUpDown, Search, Inbox, CheckCircle } from '@lucide/svelte';

	type TicketStatus = 'buka' | 'diproses' | 'selesai';
	type Ticket = {
		id: string;
		code: string;
		subject: string;
		category: string;
		priority: string;
		status: TicketStatus;
		last_message_at: string | null;
		last_message_by: string;
		last_body: string;
		unread: number;
		created_at: string | null;
		partner_name: string;
		partner_public_id: string;
	};
	type Message = { id: string; author_role: string; body: string; created_at: string | null };

	const STATUS_LABELS = $derived<Record<TicketStatus, string>>({
		buka: tr('Buka'),
		diproses: tr('Diproses'),
		selesai: tr('Selesai')
	});
	const PRIORITY_LABELS = $derived<Record<string, string>>({ rendah: tr('Rendah'), normal: tr('Normal'), tinggi: tr('Tinggi') });
	const CATEGORY_LABELS = $derived<Record<string, string>>({
		teknis: tr('Teknis'),
		pembayaran: tr('Pembayaran'),
		publikasi: tr('Publikasi'),
		lainnya: tr('Lainnya')
	});

	const PAGE_SIZE = 20;

	let loading = $state(true);
	let error = $state('');
	let rows = $state<Ticket[]>([]);
	let total = $state(0);
	// Hitungan per status untuk ringkasan berwarna: dari server (bukan dari
	// 20 baris halaman aktif), supaya angkanya tetap benar saat difilter/paginasi.
	let counts = $state<{ buka: number; diproses: number; selesai: number } | null>(null);
	let status = $state<string>('');
	let priority = $state<string>('');
	// Urutan kolom Prioritas: default tinggi→rendah (dari API), klik header
	// membalik jadi rendah→tinggi. Dikerjakan server supaya konsisten lintas
	// halaman — sortir di klien hanya akan mengurutkan 20 baris halaman aktif.
	let priorityDesc = $state(false);
	let query = $state('');
	let pageNo = $state(1);

	let detail = $state<Ticket | null>(null);
	let messages = $state<Message[]>([]);
	let detailLoading = $state(false);
	let draft = $state('');
	let sending = $state(false);
	let statusBusy = $state(false);

	let debounceTimer: ReturnType<typeof setTimeout> | undefined;

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
			clearTimeout(debounceTimer);
		};
	});

	// Notifikasi "Tiket dukungan baru" membawa ?t=<id>. Efek (bukan onMount)
	// supaya tetap jalan saat halaman dukungan sudah terbuka — pindah query
	// tidak me-remount komponen. Parameter dibersihkan sesudahnya agar klik
	// notifikasi yang sama berikutnya memicu popup lagi.
	$effect(() => {
		const id = (page.url.searchParams.get('t') ?? '').trim();
		if (!id) return;
		void openDetail(rows.find((r) => r.id === id) ?? ({ id } as Ticket));
		const url = new URL(page.url);
		url.searchParams.delete('t');
		void goto(`${url.pathname}${url.search}`, { replaceState: true, noScroll: true, keepFocus: true });
	});

	function onQueryInput(value: string) {
		query = value;
		pageNo = 1;
		clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => void load(), 300);
	}

	function togglePrioritySort() {
		priorityDesc = !priorityDesc;
		pageNo = 1;
		void load();
	}

	async function load() {
		loading = true;
		error = '';
		const qs = new URLSearchParams();
		if (status) qs.set('status', status);
		if (priority) qs.set('priority', priority);
		if (priorityDesc) qs.set('urut', 'prioritas_desc');
		if (query.trim()) qs.set('q', query.trim());
		qs.set('limit', String(PAGE_SIZE));
		qs.set('offset', String((pageNo - 1) * PAGE_SIZE));
		try {
			const [res] = await Promise.all([
				apiRequest(`/v1/platform/tickets?${qs.toString()}`),
				loadCounts()
			]);
			if (!res.ok) {
				const msg = await readApiError(res, 'Permintaan platform gagal.');
				error = msg === 'Resource not found' ? 'Akses platform administrator tidak tersedia.' : msg;
				return;
			}
			const data = await res.json();
			rows = data.tickets ?? [];
			total = data.total ?? 0;
		} catch {
			error = tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		} finally {
			loading = false;
		}
	}

	// Tiga panggilan ringan: total per status, tanpa filter aktif.
	async function loadCounts() {
		const one = async (s: TicketStatus) => {
			const r = await apiRequest(`/v1/platform/tickets?limit=1&status=${s}`);
			if (!r.ok) return 0;
			const d = await r.json();
			return Number(d.total) || 0;
		};
		const [buka, diproses, selesai] = await Promise.all([one('buka'), one('diproses'), one('selesai')]);
		counts = { buka, diproses, selesai };
	}

	/** Klik kartu ringkasan = saring tabel ke status itu (klik dua kali = semua). */
	function filterByStatus(s: TicketStatus) {
		status = status === s ? '' : s;
		pageNo = 1;
		void load();
	}

	async function openDetail(ticket: Ticket) {
		detailLoading = true;
		detail = ticket;
		messages = [];
		draft = '';
		try {
			const res = await apiRequest(`/v1/platform/tickets/${ticket.id}`);
			if (!res.ok) return;
			const data = await res.json();
			detail = data.ticket as Ticket;
			messages = (data.messages ?? []) as Message[];
			rows = rows.map((r) => (r.id === detail?.id ? { ...r, unread: 0 } : r));
		} finally {
			detailLoading = false;
		}
	}

	async function sendReply() {
		if (!detail || sending || !draft.trim()) return;
		sending = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/tickets/${detail.id}/messages`, {
				method: 'POST',
				body: JSON.stringify({ body: draft })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal mengirim balasan.');
				return;
			}
			const data = await res.json();
			detail = data.ticket as Ticket;
			draft = '';
			const fresh = await apiRequest(`/v1/platform/tickets/${detail.id}`);
			if (fresh.ok) {
				const parsed = await fresh.json();
				detail = parsed.ticket as Ticket;
				messages = (parsed.messages ?? []) as Message[];
			}
			await load();
		} finally {
			sending = false;
		}
	}

	async function updateStatus(next: TicketStatus) {
		if (!detail || statusBusy) return;
		statusBusy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/tickets/${detail.id}`, {
				method: 'PATCH',
				body: JSON.stringify({ status: next })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal mengubah status.');
				return;
			}
			const data = await res.json();
			detail = data.ticket as Ticket;
			rows = rows.map((r) => (r.id === detail?.id ? { ...r, status: next } : r));
		} finally {
			statusBusy = false;
		}
	}

	function preview(value: string) {
		return value.length > 90 ? value.slice(0, 90) + '…' : value;
	}
</script>

<section>
	<PageHeader title={tr('Dukungan')} description={tr('Tiket dukungan dari mitra. Balasan dikirim ke lonceng, email, dan WhatsApp mitra.')} />

	<Feedback {loading} {error} loadingText={tr('Memuat tiket…')} />

	{#if !loading && rows.length === 0 && error.includes('Akses platform')}
		<div class="app-card mt-4 px-4 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr('Akun ini tidak memiliki akses ke tiket dukungan.')}</p>
		</div>
	{:else}
		<!-- Ringkasan per status: satu warna per keadaan, sekaligus filter -->
		<div class="summary mt-5">
			<button
				class="sum-card"
				data-tone="buka"
				data-zero={!(counts?.['buka'] ?? 0)}
				data-active={status === 'buka'}
				type="button"
				aria-pressed={status === 'buka'}
				onclick={() => filterByStatus('buka')}
			>
				<span class="sum-top">
					<span class="sum-dot" aria-hidden="true"></span>
					<span class="sum-label">{tr('Belum dijawab')}</span>
				</span>
				<span class="sum-value tabular-nums">{counts?.buka ?? '—'}</span>
				<span class="sum-hint">{tr('menunggu balasan admin')}</span>
			</button>
			<button
				class="sum-card"
				data-tone="diproses"
				data-zero={!(counts?.['diproses'] ?? 0)}
				data-active={status === 'diproses'}
				type="button"
				aria-pressed={status === 'diproses'}
				onclick={() => filterByStatus('diproses')}
			>
				<span class="sum-top">
					<span class="sum-dot" aria-hidden="true"></span>
					<span class="sum-label">{tr('Sedang ditangani')}</span>
				</span>
				<span class="sum-value tabular-nums">{counts?.diproses ?? '—'}</span>
				<span class="sum-hint">{tr('sudah dibalas, belum ditutup')}</span>
			</button>
			<button
				class="sum-card"
				data-tone="selesai"
				data-zero={!(counts?.['selesai'] ?? 0)}
				data-active={status === 'selesai'}
				type="button"
				aria-pressed={status === 'selesai'}
				onclick={() => filterByStatus('selesai')}
			>
				<span class="sum-top">
					<span class="sum-dot" aria-hidden="true"></span>
					<span class="sum-label">{tr('Selesai')}</span>
				</span>
				<span class="sum-value tabular-nums">{counts?.selesai ?? '—'}</span>
				<span class="sum-hint">{tr('sudah ditutup')}</span>
			</button>
		</div>

		<div class="mt-5 flex flex-wrap items-center gap-2">
			<label class="search-box">
				<Search size={15} strokeWidth={1.5} class="shrink-0 text-muted-foreground" />
				<input
					class="w-full bg-transparent text-sm outline-none"
					name="q"
					aria-label={tr('Cari tiket')}
					placeholder={tr('Cari subjek atau nama mitra')}
					value={query}
					oninput={(e) => onQueryInput((e.currentTarget as HTMLInputElement).value)}
				/>
			</label>
			<Select
				ariaLabel={tr('Filter status')}
				minWidth={170}
				value={status}
				onchange={(v) => {
					status = v;
					pageNo = 1;
					void load();
				}}
				options={[
					{ value: '', label: tr('Semua status') },
					{ value: 'buka', label: STATUS_LABELS.buka },
					{ value: 'diproses', label: STATUS_LABELS.diproses },
					{ value: 'selesai', label: STATUS_LABELS.selesai }
				]}
			/>
			<Select
				ariaLabel={tr('Filter prioritas')}
				minWidth={170}
				value={priority}
				onchange={(v) => {
					priority = v;
					pageNo = 1;
					void load();
				}}
				options={[
					{ value: '', label: tr('Semua prioritas') },
					{ value: 'tinggi', label: PRIORITY_LABELS.tinggi },
					{ value: 'normal', label: PRIORITY_LABELS.normal },
					{ value: 'rendah', label: PRIORITY_LABELS.rendah }
				]}
			/>
			{#if !loading}
				<span class="ml-auto text-xs tabular-nums text-muted-foreground">{total} {tr('tiket')}</span>
			{/if}
		</div>

		<div class="app-card mt-3 overflow-x-auto">
			<table class="data">
				<thead>
					<tr>
						<th class="c-status">{tr('Status')}</th>
						<th class="c-subject">{tr('Subjek')}</th>
						<th class="c-partner">{tr('Mitra')}</th>
						<th class="c-cat">{tr('Kategori')}</th>
						<th class="c-prio" aria-sort={priorityDesc ? 'ascending' : 'descending'}>
							<button class="sort-btn" type="button" onclick={togglePrioritySort} aria-label={priorityDesc ? tr('Urutkan prioritas dari yang tertinggi') : tr('Urutkan prioritas dari yang terendah')}>
								{tr('Prioritas')}
								{#if priorityDesc}<ArrowDown size={12} strokeWidth={1.5} />{:else}<ArrowUpDown size={12} strokeWidth={1.5} />{/if}
							</button>
						</th>
						<th class="c-last">{tr('Pesan terakhir')}</th>
						<th class="c-act"></th>
					</tr>
				</thead>
				<tbody>
					{#each rows as t (t.id)}
						<tr>
							<td class="c-status">
								<span class="status-pill" data-tone={t.status}>
									<span class="pill-dot" aria-hidden="true"></span>
									{STATUS_LABELS[t.status]}
								</span>
							</td>
							<td class="c-subject">
								<button class="subject-btn" type="button" onclick={() => void openDetail(t)}>
									<span class="truncate">{t.subject}</span>
									{#if t.unread > 0}<span class="unread" aria-label="{t.unread} pesan belum dibaca">{t.unread}</span>{/if}
								</button>
								<span class="block text-xs text-muted-foreground">#{t.code}</span>
							</td>
							<td class="c-partner"><span class="truncate">{t.partner_name || '—'}</span></td>
							<td class="c-cat"><span class="cat-chip">{CATEGORY_LABELS[t.category] ?? t.category}</span></td>
							<td class="c-prio">
								<span class="prio" data-prio={t.priority}>{PRIORITY_LABELS[t.priority] ?? t.priority}</span>
							</td>
							<td class="c-last">
								<span class="block truncate text-muted-foreground">{preview(t.last_body) || '—'}</span>
								<span class="block text-xs text-muted-foreground">
									{t.last_message_by === 'admin' ? tr('admin') : t.partner_name || tr('mitra')}
									<span aria-hidden="true"> · </span>{fmtDateTime(t.last_message_at)}
								</span>
							</td>
							<td class="c-act">
								<button class="btn-open" type="button" onclick={() => void openDetail(t)}>{tr('Buka')}</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && rows.length === 0}
				<div class="empty">
					<Inbox size={18} strokeWidth={1.5} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">{tr('Tidak ada tiket pada filter ini.')}</p>
					{#if status || priority || query.trim()}
						<button
							class="mt-3 text-xs font-medium text-primary hover:underline"
							type="button"
							onclick={() => {
								status = '';
								priority = '';
								query = '';
								pageNo = 1;
								void load();
							}}>{tr('Hapus filter')}</button
						>
					{/if}
				</div>
			{:else if !loading && status === '' && (counts?.buka ?? 0) === 0 && (counts?.diproses ?? 0) === 0}
				<!-- Penutup: tidak ada yang perlu dikerjakan, jadi tabel tidak berakhir kosong -->
				<div class="footer-note">
					<CheckCircle size={15} strokeWidth={1.5} />
					<span>{tr('Semua tiket sudah ditangani')}</span>
				</div>
			{/if}
		</div>
		<Pagination total={total} limit={PAGE_SIZE} offset={(pageNo - 1) * PAGE_SIZE} onChange={(next) => { pageNo = next / PAGE_SIZE + 1; void load(); }} />
	{/if}
</section>

{#if detail}
	<Modal title={[detail.code ? `#${detail.code}` : '', detail.subject].filter(Boolean).join(' · ')} label={tr('Detail tiket dukungan')} maxWidth="xl" onClose={() => (detail = null)}>
		<div class="space-y-3">
			<div class="flex flex-wrap items-center gap-2">
				<span class="status-pill" data-tone={detail.status}>
					<span class="pill-dot" aria-hidden="true"></span>
					{STATUS_LABELS[detail.status]}
				</span>
				<span class="cat-chip">{CATEGORY_LABELS[detail.category] ?? detail.category}</span>
				<span class="prio" data-prio={detail.priority}>{PRIORITY_LABELS[detail.priority] ?? detail.priority}</span>
				<span class="text-xs text-muted-foreground">{tr('Dibuat')} {fmtDateTime(detail.created_at)}</span>
			</div>
			<p class="text-sm">
				<span class="text-xs text-muted-foreground">{tr('Mitra')}: </span>
				{detail.partner_name || '-'}
			</p>

			<div class="max-h-[46vh] space-y-2 overflow-y-auto pr-1">
				{#if detailLoading}
					<p class="text-xs text-muted-foreground">{tr('Memuat percakapan…')}</p>
				{/if}
				{#each messages as m (m.id)}
					<div class="flex" class:justify-end={m.author_role === 'admin'}>
						<div
							class="max-w-[85%] rounded-xl border px-3 py-2"
							style={m.author_role === 'admin'
								? 'border-color: transparent; background-color: color-mix(in srgb, var(--primary) 10%, #fff);'
								: 'border-color: var(--border);'}
						>
							<p class="text-[11px] font-semibold text-muted-foreground">
								{m.author_role === 'admin' ? tr('Admin') : detail.partner_name || tr('Mitra')}
							</p>
							<MultilineText class="mt-0.5 whitespace-pre-wrap break-words text-sm" text={m.body} />
							<p class="mt-1 text-[10px] text-muted-foreground">{fmtDateTime(m.created_at)}</p>
						</div>
					</div>
				{/each}
			</div>

			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">{tr('Balas ke mitra')}</span>
				<textarea class="app-input mt-1 w-full" rows="4" maxlength="5000" bind:value={draft} placeholder={tr('Tulis balasan…')}></textarea>
			</label>
		</div>
		{#snippet footer()}
			<span class="mr-auto">
				<Select
					ariaLabel={tr('Ubah status tiket')}
					minWidth={170}
					disabled={statusBusy}
					value={detail?.status ?? ''}
					onchange={(v) => void updateStatus(v as TicketStatus)}
					options={[
						{ value: 'buka', label: STATUS_LABELS.buka },
						{ value: 'diproses', label: STATUS_LABELS.diproses },
						{ value: 'selesai', label: STATUS_LABELS.selesai }
					]}
				/>
			</span>
			<button class="app-btn app-btn-soft" type="button" onclick={() => (detail = null)}>{tr('Tutup')}</button>
			<button class="app-btn app-btn-primary" type="button" disabled={sending || !draft.trim()} onclick={() => void sendReply()}>
				<Send size={14} /> {sending ? tr('Mengirim…') : tr('Kirim balasan')}
			</button>
		{/snippet}
	</Modal>
{/if}

<style>
	/* ── Ringkasan status: satu warna per keadaan ── */
	.summary {
		display: grid;
		gap: 12px;
	}
	@media (min-width: 720px) {
		.summary { grid-template-columns: repeat(3, minmax(0, 1fr)); }
	}
	.sum-card {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 2px;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 14px 16px;
		background-color: var(--card);
		text-align: left;
		transition:
			background-color 160ms cubic-bezier(0.22, 1, 0.36, 1),
			border-color 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.sum-card:hover { background-color: var(--secondary); }
	.sum-card:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; }

	.sum-top { display: inline-flex; align-items: center; gap: 6px; }
	.sum-dot {
		display: inline-block;
		width: 6px;
		height: 6px;
		border-radius: 999px;
		background-color: var(--muted-foreground);
	}
	.sum-label { color: var(--muted-foreground); font-size: 12px; line-height: 18px; }
	.sum-value {
		font-size: 26px;
		font-weight: 500;
		line-height: 32px;
		color: var(--foreground);
	}
	.sum-hint { color: var(--muted-foreground); font-size: 11px; line-height: 16px; }

	/* Warna per status — hanya saat jumlahnya > 0, supaya angka nol tidak berteriak */
	.sum-card[data-tone='buka'] .sum-dot { background-color: var(--warning); }
	.sum-card[data-tone='diproses'] .sum-dot { background-color: var(--primary); }
	.sum-card[data-tone='selesai'] .sum-dot { background-color: var(--success); }

	.sum-card[data-tone='buka']:not([data-zero='true']) .sum-value { color: var(--warning); }
	.sum-card[data-tone='diproses']:not([data-zero='true']) .sum-value { color: var(--primary); }
	.sum-card[data-tone='selesai']:not([data-zero='true']) .sum-value { color: var(--success); }

	.sum-card[data-tone='buka'][data-zero='true'] .sum-value,
	.sum-card[data-tone='diproses'][data-zero='true'] .sum-value,
	.sum-card[data-tone='selesai'][data-zero='true'] .sum-value { color: var(--muted-foreground); }

	.sum-card[data-tone='buka'] { background-color: color-mix(in srgb, var(--warning-soft) 60%, var(--card)); border-color: color-mix(in srgb, var(--warning) 22%, var(--border)); }
	.sum-card[data-tone='diproses'] { background-color: color-mix(in srgb, var(--accent) 70%, var(--card)); border-color: color-mix(in srgb, var(--primary) 20%, var(--border)); }
	.sum-card[data-tone='selesai'] { background-color: color-mix(in srgb, var(--success-soft) 70%, var(--card)); border-color: color-mix(in srgb, var(--success) 20%, var(--border)); }
	.sum-card[data-active='true'] {
		border-color: var(--foreground);
		box-shadow: inset 0 0 0 1px var(--foreground);
	}

	/* ── Pencarian ── */
	.search-box {
		display: flex;
		flex: 1;
		align-items: center;
		gap: 8px;
		min-width: 220px;
		max-width: 380px;
		min-height: 34px;
		border: 1px solid var(--input);
		border-radius: var(--radius-control);
		padding: 0 12px;
		background-color: var(--card);
	}
	.search-box:focus-within {
		border-color: var(--ring);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--ring) 15%, transparent);
	}

	/* ── Tabel ── */
	.data {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
		table-layout: fixed;
	}
	.data thead th {
		padding: 10px 12px;
		border-bottom: 1px solid var(--border);
		color: var(--muted-foreground);
		font-size: 11px;
		font-weight: 500;
		text-align: left;
		white-space: nowrap;
	}
	.data tbody td {
		padding: 10px 12px;
		vertical-align: middle;
		min-width: 0;
		overflow: hidden;
	}
	.data tbody tr + tr td { border-top: 1px solid color-mix(in srgb, var(--foreground) 6%, transparent); }
	.data tbody tr:hover td { background-color: color-mix(in srgb, var(--secondary) 55%, transparent); }

	.c-status { width: 116px; }
	.c-subject { width: 22%; }
	.c-partner { width: 13%; }
	.c-cat { width: 116px; }
	.c-prio { width: 88px; }
	.c-last { width: 27%; }
	.c-act { width: 68px; text-align: right !important; }
	.data thead th.c-act { text-align: right; }

	.sort-btn {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		color: inherit;
		font-size: inherit;
		font-weight: 500;
		transition: color 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.sort-btn:hover { color: var(--foreground); }
	.sort-btn:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; border-radius: 4px; }

	/* ── Chip status ── */
	.status-pill {
		display: inline-flex;
		align-items: center;
		border-radius: 5px;
		padding: 1px 7px;
		font-size: 11px;
		font-weight: 500;
		line-height: 18px;
		white-space: nowrap;
	}
	.pill-dot {
		display: inline-block;
		width: 5px;
		height: 5px;
		border-radius: 999px;
		margin-right: 6px;
		background-color: currentColor;
	}
	.status-pill[data-tone='buka'] { background-color: var(--warning-soft); color: var(--warning); }
	.status-pill[data-tone='diproses'] { background-color: var(--accent); color: var(--accent-foreground); }
	.status-pill[data-tone='selesai'] { background-color: var(--success-soft); color: var(--success); }

	/* ── Prioritas & kategori ── */
	.prio {
		display: inline-flex;
		align-items: center;
		border-radius: 5px;
		padding: 1px 7px;
		font-size: 11px;
		font-weight: 500;
		line-height: 18px;
		white-space: nowrap;
	}
	.prio[data-prio='tinggi'] { background-color: var(--destructive-soft, #fdecee); color: var(--destructive); }
	.prio[data-prio='normal'] { background-color: var(--secondary); color: var(--foreground); }
	.prio[data-prio='rendah'] { background-color: var(--secondary); color: var(--muted-foreground); }

	.cat-chip {
		display: inline-block;
		max-width: 100%;
		border-radius: 5px;
		padding: 1px 7px;
		background-color: var(--secondary);
		color: var(--muted-foreground);
		font-size: 11px;
		line-height: 18px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	/* ── Subjek ── */
	.subject-btn {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		min-width: 0;
		color: var(--foreground);
		font-size: 13px;
		font-weight: 500;
		text-align: left;
	}
	.subject-btn:hover .truncate { text-decoration: underline; text-underline-offset: 2px; }
	.subject-btn:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; border-radius: 4px; }
	.unread {
		flex: 0 0 auto;
		min-width: 16px;
		border-radius: 999px;
		padding: 0 4px;
		background-color: var(--destructive);
		color: #fff;
		font-size: 10px;
		font-weight: 500;
		line-height: 16px;
		text-align: center;
	}

	.btn-open {
		min-height: 28px;
		border: 1px solid var(--border);
		border-radius: 7px;
		padding: 0 10px;
		background-color: var(--card);
		color: var(--foreground);
		font-size: 12px;
		font-weight: 500;
		transition: background-color 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.btn-open:hover { background-color: var(--secondary); }

	/* ── Kosong & penutup ── */
	.empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		padding: 40px 16px;
		text-align: center;
	}
	.footer-note {
		display: flex;
		align-items: center;
		gap: 8px;
		border-top: 1px solid color-mix(in srgb, var(--foreground) 6%, transparent);
		padding: 12px 16px;
		color: var(--success);
		font-size: 12px;
	}
</style>
