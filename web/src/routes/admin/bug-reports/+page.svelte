<script lang="ts">
	// Laporan bug — port app/admin/bug-reports/page.tsx.
	// Daftar laporan bug user + penanganan: ubah status cepat dari tabel dan
	// buka modal detail (GET /bug-reports/{id}) untuk menambah catatan admin.
	import { onMount } from 'svelte';
	import { CheckCircle2, Clock, TriangleAlert, XCircle } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';

	type BugStatus = 'open' | 'triage' | 'resolved' | 'closed';
	type BugReport = {
		id: string;
		user_id: string;
		brand_id?: string | null;
		message: string;
		context_info?: string | null;
		status: BugStatus;
		admin_notes?: string | null;
		user_email?: string | null;
		user_name?: string | null;
		brand_name?: string | null;
		created_at: string;
		updated_at?: string | null;
	};

	const STATUS_LABELS: Record<BugStatus, string> = {
		open: 'Open',
		triage: 'Triage',
		resolved: 'Resolved',
		closed: 'Closed'
	};
	const STATUS_TONES: Record<BugStatus, 'warning' | 'accent' | 'success' | 'neutral'> = {
		open: 'warning',
		triage: 'accent',
		resolved: 'success',
		closed: 'neutral'
	};
	const STATUS_FILTERS: Array<BugStatus | 'all'> = ['all', 'open', 'triage', 'resolved', 'closed'];

	const PAGE_SIZE = 20;

	let loading = $state(true);
	let error = $state('');
	let rows = $state<BugReport[]>([]);
	let total = $state(0);
	let status = $state<string>('all');
	let query = $state('');
	let page = $state(1);
	let busy = $state<string | null>(null);
	let detail = $state<BugReport | null>(null);
	let detailLoading = $state(false);
	let notes = $state('');
	let notesBusy = $state(false);

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

	function onQueryInput(value: string) {
		query = value;
		page = 1;
		clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => void load(), 300);
	}

	async function load() {
		loading = true;
		error = '';
		const qs = new URLSearchParams();
		if (status !== 'all') qs.set('status', status);
		if (query.trim()) qs.set('q', query.trim());
		qs.set('limit', String(PAGE_SIZE));
		qs.set('offset', String((page - 1) * PAGE_SIZE));
		try {
			const res = await apiRequest(`/v1/platform/bug-reports?${qs.toString()}`);
			if (!res.ok) {
				const msg = await readApiError(res, 'Permintaan platform gagal.');
				error = msg === 'Resource not found' ? 'Akses platform administrator tidak tersedia.' : msg;
				return;
			}
			const data = await res.json();
			rows = data.items ?? [];
			total = data.total ?? 0;
		} catch {
			error = 'Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.';
		} finally {
			loading = false;
		}
	}

	async function updateStatus(report: BugReport, newStatus: BugStatus) {
		busy = report.id;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/bug-reports/${report.id}`, {
				method: 'PATCH',
				body: JSON.stringify({ status: newStatus })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			rows = rows.map((item) => (item.id === report.id ? { ...item, status: newStatus } : item));
			if (detail?.id === report.id) detail = { ...detail, status: newStatus };
		} finally {
			busy = null;
		}
	}

	async function openDetail(report: BugReport) {
		detailLoading = true;
		detail = report;
		notes = report.admin_notes ?? '';
		try {
			const res = await apiRequest(`/v1/platform/bug-reports/${report.id}`);
			if (res.ok) {
				const data = (await res.json()) as BugReport;
				detail = data;
				notes = data.admin_notes ?? '';
			}
		} finally {
			detailLoading = false;
		}
	}

	async function saveNotes() {
		if (!detail) return;
		notesBusy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/bug-reports/${detail.id}`, {
				method: 'PATCH',
				body: JSON.stringify({ admin_notes: notes.trim() || null })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			const id = detail.id;
			rows = rows.map((item) => (item.id === id ? { ...item, admin_notes: notes } : item));
			detail = { ...detail, admin_notes: notes };
		} finally {
			notesBusy = false;
		}
	}

	function preview(message: string) {
		return message.length > 120 ? message.slice(0, 120) + '…' : message;
	}

	const statusIcon = (value: BugStatus) =>
		value === 'open' ? Clock : value === 'triage' ? TriangleAlert : value === 'resolved' ? CheckCircle2 : XCircle;
</script>

<section class="mx-auto max-w-7xl">
	<PageHeader title="Bug Reports" description="Laporan dari user tentang bug atau hasil AI yang tidak akurat.">
		<Badge tone="accent" label="{total} laporan" />
	</PageHeader>

	<Feedback {loading} {error} loadingText="Memuat laporan…" />

	{#if rows.length === 0 && error.includes('Akses platform')}
		<div class="app-card mt-4 px-4 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke laporan masuk.</p>
		</div>
	{:else}
		<div class="mt-4 flex flex-wrap items-center gap-3">
			<input
				class="app-input w-full max-w-md"
				placeholder="Cari pesan, user, atau brand…"
				value={query}
				oninput={(e) => onQueryInput((e.currentTarget as HTMLInputElement).value)}
			/>
			<select
				class="app-input w-auto min-w-[160px]"
				aria-label="Filter status"
				bind:value={status}
				onchange={() => {
					page = 1;
					void load();
				}}
			>
				{#each STATUS_FILTERS as value (value)}
					<option value={value}>{value === 'all' ? 'Semua' : STATUS_LABELS[value as BugStatus]}</option>
				{/each}
			</select>
		</div>

		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 900px;">
				<thead>
					<tr class="text-xs text-muted-foreground">
						<th class="px-4 py-2.5 text-left font-medium">Status</th>
						<th class="px-4 py-2.5 text-left font-medium">Pesan</th>
						<th class="px-4 py-2.5 text-left font-medium">Pengguna</th>
						<th class="px-4 py-2.5 text-left font-medium">Brand</th>
						<th class="px-4 py-2.5 text-left font-medium">Dibuat</th>
						<th class="px-4 py-2.5 text-left font-medium">Aksi</th>
					</tr>
				</thead>
				<tbody>
					{#each rows as r (r.id)}
						{@const Icon = statusIcon(r.status)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<Badge tone={STATUS_TONES[r.status]} label={STATUS_LABELS[r.status]} />
							</td>
							<td class="px-4 py-2.5">
								<button class="text-left hover:underline" type="button" onclick={() => void openDetail(r)}>{preview(r.message)}</button>
							</td>
							<td class="px-4 py-2.5">{r.user_name ?? r.user_email ?? '-'}</td>
							<td class="px-4 py-2.5 text-muted-foreground">{r.brand_name ?? '-'}</td>
							<td class="px-4 py-2.5 text-muted-foreground">{fmtDateTime(r.created_at)}</td>
							<td class="px-4 py-2.5">
								<div class="flex gap-1.5">
									<button
										class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
										type="button"
										disabled={busy === r.id}
										title={`Tandai ${r.status === 'open' ? 'triage' : 'resolved'}`}
										onclick={() => void updateStatus(r, r.status === 'open' ? 'triage' : 'resolved')}
									>
										<Icon size={13} />
										{busy === r.id ? '…' : r.status === 'open' ? 'Triage' : 'Resolved'}
									</button>
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void openDetail(r)}>Detail</button>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && rows.length === 0}
				<p class="px-4 py-12 text-center text-sm text-muted-foreground">Belum ada laporan masuk.</p>
			{/if}
		</div>
		<Pagination total={total} limit={PAGE_SIZE} offset={(page - 1) * PAGE_SIZE} onChange={(next) => { page = next / PAGE_SIZE + 1; void load(); }} />
	{/if}
</section>

{#if detail}
	<Modal title="Detail laporan bug" label="Detail laporan bug" maxWidth="xl" onClose={() => (detail = null)}>
		<div class="space-y-3">
			<div class="flex flex-wrap items-center gap-2">
				<Badge tone={STATUS_TONES[detail.status]} label={STATUS_LABELS[detail.status]} />
				<span class="text-xs text-muted-foreground">Dibuat {fmtDateTime(detail.created_at)}</span>
				{#if detail.updated_at}<span class="text-xs text-muted-foreground">· Diperbarui {fmtDateTime(detail.updated_at)}</span>{/if}
			</div>
			<div class="grid gap-2 text-sm sm:grid-cols-2">
				<p><span class="text-xs text-muted-foreground">Pengguna: </span>{detail.user_name ?? detail.user_email ?? '-'}</p>
				<p><span class="text-xs text-muted-foreground">Brand: </span>{detail.brand_name ?? '-'}</p>
			</div>
			<div>
				<p class="text-xs font-medium text-muted-foreground">Pesan</p>
				<p class="mt-1 whitespace-pre-wrap rounded-lg border px-3 py-2 text-sm">{detail.message}</p>
			</div>
			{#if detail.context_info}
				<div>
					<p class="text-xs font-medium text-muted-foreground">Konteks</p>
					<pre class="mt-1 max-h-48 overflow-auto whitespace-pre-wrap rounded-lg border px-3 py-2 text-xs">{detail.context_info}</pre>
				</div>
			{/if}
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Catatan admin (penanganan)</span>
				<textarea class="app-input mt-1 w-full" rows="4" bind:value={notes} placeholder="Catatan tindak lanjut…"></textarea>
			</label>
			{#if detailLoading}<p class="text-xs text-muted-foreground">Memuat detail…</p>{/if}
		</div>
		{#snippet footer()}
			<select
				class="app-input mr-auto w-auto"
				aria-label="Ubah status laporan"
				disabled={!detail}
				value={detail?.status ?? ''}
				onchange={(e) => { if (detail) void updateStatus(detail, (e.currentTarget as HTMLSelectElement).value as BugStatus); }}
			>
				{#each ['open', 'triage', 'resolved', 'closed'] as value (value)}
					<option value={value}>{STATUS_LABELS[value as BugStatus]}</option>
				{/each}
			</select>
			<button class="app-btn app-btn-soft" type="button" onclick={() => (detail = null)}>Tutup</button>
			<button class="app-btn app-btn-primary" type="button" disabled={notesBusy} onclick={() => void saveNotes()}>
				{notesBusy ? 'Menyimpan…' : 'Simpan catatan'}
			</button>
		{/snippet}
	</Modal>
{/if}
