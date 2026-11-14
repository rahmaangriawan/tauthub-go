<script lang="ts">
	// Panel aksi yang diusulkan agent MCP (Hermes / Claude Code). Hanya baca:
	// persetujuan dilakukan di bot Telegram, bukan di sini. Halaman ini
	// memberi admin cara meninjau aksi yang menunggu, melihat params penuh,
	// dan mengecek status eksekusi + error.
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';
	import CopyMarkdownButton from '$lib/components/admin/copy-markdown-button.svelte';
	import { mdReport, mdTable, mdList, mdCode } from '$lib/copy-markdown';

	type Action = {
		id: string;
		code: string;
		tool: string;
		summary: string;
		required_permission?: string;
		status: string;
		approved_by?: string;
		approver_email?: string;
		client_name?: string;
		error?: string;
		telegram_chat_id?: number | null;
		telegram_message_id?: number | null;
		created_at: string;
		decided_at?: string | null;
		executed_at?: string | null;
		expires_at: string;
	};
	type Detail = Action & { params?: unknown; result?: unknown };
	type Page = { total: number; items: Action[]; limit: number; offset: number };

	const PAGE_SIZE_OPTIONS = [10, 20, 50];
	const STATUS_OPTIONS = [
		{ value: '', label: 'Semua status' },
		{ value: 'proposed', label: 'Menunggu persetujuan' },
		{ value: 'executing', label: 'Sedang dijalankan' },
		{ value: 'executed', label: 'Berhasil' },
		{ value: 'failed', label: 'Gagal' },
		{ value: 'rejected', label: 'Ditolak' },
		{ value: 'expired', label: 'Kedaluwarsa' }
	];

	let loading = $state(true);
	let error = $state('');
	let data = $state<Page>({ total: 0, items: [], limit: 20, offset: 0 });
	let offset = $state(0);
	let pageSize = $state(20);
	let status = $state('');
	let tool = $state('');
	let detailOpen = $state<string | null>(null);
	let detail = $state<Detail | null>(null);
	let detailLoading = $state(false);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load(0, pageSize, status, tool);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load(nextOffset: number, nextLimit: number, s: string, t: string) {
		loading = true;
		error = '';
		const params = new URLSearchParams({ offset: String(nextOffset), limit: String(nextLimit) });
		if (s) params.set('status', s);
		if (t) params.set('tool', t);
		try {
			const res = await apiRequest(`/v1/platform/agent-actions?${params.toString()}`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			data = await res.json();
			offset = nextOffset;
		} catch {
			error = 'Tidak dapat terhubung ke server API.';
		} finally {
			loading = false;
		}
	}

	async function openDetail(id: string) {
		detailOpen = id;
		detail = null;
		detailLoading = true;
		try {
			const res = await apiRequest(`/v1/platform/agent-actions/${id}`);
			if (res.ok) detail = await res.json();
		} finally {
			detailLoading = false;
		}
	}

	function statusTone(s: string): 'neutral' | 'success' | 'danger' | 'warning' | 'accent' {
		switch (s) {
			case 'executed':
				return 'success';
			case 'failed':
			case 'rejected':
				return 'danger';
			case 'expired':
				return 'warning';
			case 'executing':
				return 'accent';
			default:
				return 'neutral';
		}
	}

	// Field sensitif pada params/result tidak ikut disalin.
	const SENSITIVE_KEY = /pass(word|wd)?|secret|token|api[_-]?key|authorization|credential|private[_-]?key|sealed|cookie|signature/i;
	function redact(value: unknown, found: Set<string>): unknown {
		if (Array.isArray(value)) return value.map((v) => redact(v, found));
		if (value && typeof value === 'object') {
			const out: Record<string, unknown> = {};
			for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
				if (SENSITIVE_KEY.test(k)) {
					found.add(k);
					out[k] = '[DIREDAKSI]';
				} else out[k] = redact(v, found);
			}
			return out;
		}
		return value;
	}

	// Salin halaman daftar yang sedang tampil (filter dan halaman ikut).
	function toMarkdown(): string {
		const statusLabel = STATUS_OPTIONS.find((o) => o.value === status)?.label ?? status;
		return mdReport(
			'Aksi Agent',
			mdList([
				['Total cocok', data.total],
				['Ditampilkan', data.items.length ? `${offset + 1}-${offset + data.items.length}` : '0'],
				['Filter status', status ? statusLabel : ''],
				['Filter tool', tool]
			]),
			mdTable(
				['Waktu', 'ID', 'Tool', 'Kode', 'Ringkasan', 'Status', 'Disetujui', 'Error'],
				data.items.map((a) => [
					fmtDateTime(a.created_at),
					a.id,
					a.tool,
					a.code,
					a.summary,
					a.status,
					a.approver_email || a.approved_by,
					a.error
				])
			)
		);
	}

	// Salin detail satu aksi yang sedang dibuka di dialog.
	function detailToMarkdown(): string {
		const d = detail;
		if (!d) return '';
		const found = new Set<string>();
		const params = redact(d.params ?? {}, found);
		const result = d.result ? redact(d.result, found) : null;
		return mdReport(
			`Aksi Agent ${d.code}`,
			mdList([
				['ID', d.id],
				['Tool', d.tool],
				['Kode', d.code],
				['Status', d.status],
				['Klien agent', d.client_name],
				['Izin diperlukan', d.required_permission],
				['Dibuat', fmtDateTime(d.created_at)],
				['Kedaluwarsa', fmtDateTime(d.expires_at)],
				['Diputuskan', d.decided_at ? fmtDateTime(d.decided_at) : null],
				['Dieksekusi', d.executed_at ? fmtDateTime(d.executed_at) : null],
				['Oleh admin', d.approver_email || d.approved_by],
				['Error', d.error]
			]),
			'## Ringkasan\n\n' + mdCode(d.summary),
			'## Parameter\n\n' +
				(found.size ? `_Field sensitif diredaksi: ${[...found].join(', ')}_\n\n` : '') +
				mdCode(JSON.stringify(params, null, 2), 'json'),
			result ? '## Hasil\n\n' + mdCode(JSON.stringify(result, null, 2), 'json') : ''
		);
	}
</script>

<section>
	<PageHeader
		title="Aksi Agent"
		description="Aksi admin yang diusulkan Hermes / Claude Code via MCP. Persetujuan dilakukan di bot Telegram; halaman ini untuk melihat status dan histori."
	>
		<Badge tone="accent" label="{data.total.toLocaleString('id-ID')} aksi" />
		<CopyMarkdownButton build={toMarkdown} disabled={loading || data.items.length === 0} />
	</PageHeader>

	<Feedback {loading} {error} loadingText="Memuat aksi agent…" />

	<form
		class="app-card mt-4 flex flex-wrap items-center gap-3 p-3"
		onsubmit={(e) => {
			e.preventDefault();
			void load(0, pageSize, status, tool);
		}}
	>
		<Select
			ariaLabel="Filter status"
			minWidth={200}
			value={status}
			onchange={(v) => {
				status = v;
				void load(0, pageSize, status, tool);
			}}
			options={STATUS_OPTIONS}
		/>
		<input
			class="app-input min-w-[200px] flex-1"
			placeholder="Filter nama tool (mis. review_media)"
			bind:value={tool}
		/>
		<button class="app-btn app-btn-soft" type="submit" disabled={loading}>Terapkan</button>
	</form>

	<div class="app-card mt-4 overflow-x-auto">
		<table class="w-full text-sm" style="min-width: 960px;">
			<thead>
				<tr class="text-xs text-muted-foreground">
					<th class="px-4 py-2.5 text-left font-medium">Waktu</th>
					<th class="px-4 py-2.5 text-left font-medium">Tool</th>
					<th class="px-4 py-2.5 text-left font-medium">Kode</th>
					<th class="px-4 py-2.5 text-left font-medium">Ringkasan</th>
					<th class="px-4 py-2.5 text-left font-medium">Status</th>
					<th class="px-4 py-2.5 text-left font-medium">Disetujui</th>
					<th class="px-4 py-2.5 text-left font-medium"></th>
				</tr>
			</thead>
			<tbody>
				{#each data.items as a (a.id)}
					<tr class="border-t align-top">
						<td class="px-4 py-2.5 whitespace-nowrap">{fmtDateTime(a.created_at)}</td>
						<td class="px-4 py-2.5"><Badge tone="neutral" label={a.tool} /></td>
						<td class="px-4 py-2.5 font-mono text-xs">{a.code}</td>
						<td class="px-4 py-2.5 max-w-[360px]">
							<div class="text-sm" style="white-space: pre-wrap;">{a.summary}</div>
							{#if a.error}
								<div class="mt-1 text-xs text-red-600">{a.error}</div>
							{/if}
						</td>
						<td class="px-4 py-2.5"><Badge tone={statusTone(a.status)} label={a.status} /></td>
						<td class="px-4 py-2.5 text-xs">
							{a.approver_email || (a.approved_by ? a.approved_by.slice(0, 8) : '—')}
						</td>
						<td class="px-4 py-2.5">
							<button class="app-btn app-btn-soft" type="button" onclick={() => openDetail(a.id)}>Detail</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
		{#if !loading && data.items.length === 0}
			<p class="px-4 py-12 text-center text-sm text-muted-foreground">Belum ada aksi agent.</p>
		{/if}
	</div>

	<div class="mt-4 flex flex-wrap items-center justify-between gap-3">
		<div class="flex items-center gap-3">
			<p class="text-sm text-muted-foreground">
				{data.total ? `${offset + 1}–${Math.min(offset + pageSize, data.total)} dari ${data.total}` : '0 aksi'}
			</p>
			<Select
				ariaLabel="Baris per halaman"
				minWidth={140}
				value={String(pageSize)}
				onchange={(v) => {
					pageSize = Number(v);
					void load(0, pageSize, status, tool);
				}}
				options={PAGE_SIZE_OPTIONS.map((size) => ({ value: String(size), label: `${size} / halaman` }))}
			/>
		</div>
		<Pagination
			total={data.total}
			limit={pageSize}
			{offset}
			onChange={(next) => void load(next, pageSize, status, tool)}
		/>
	</div>

	{#if detailOpen}
		<div
			class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
			role="dialog"
			aria-modal="true"
		>
			<div class="app-card w-full max-w-3xl overflow-hidden">
				<div class="flex items-center justify-between border-b px-4 py-3">
					<h2 class="text-lg font-medium">Detail aksi agent</h2>
					<CopyMarkdownButton build={detailToMarkdown} disabled={detailLoading || !detail} />
					<button
						class="app-btn app-btn-soft"
						type="button"
						onclick={() => {
							detailOpen = null;
							detail = null;
						}}>Tutup</button
					>
				</div>
				<div class="max-h-[70vh] overflow-y-auto px-4 py-4 text-sm">
					{#if detailLoading}
						<p class="text-muted-foreground">Memuat…</p>
					{:else if detail}
						<dl class="grid grid-cols-[140px_1fr] gap-x-4 gap-y-2">
							<dt class="text-muted-foreground">Tool</dt>
							<dd>{detail.tool}</dd>
							<dt class="text-muted-foreground">Kode</dt>
							<dd class="font-mono text-xs">{detail.code}</dd>
							<dt class="text-muted-foreground">Status</dt>
							<dd><Badge tone={statusTone(detail.status)} label={detail.status} /></dd>
							<dt class="text-muted-foreground">Klien agent</dt>
							<dd>{detail.client_name || '—'}</dd>
							<dt class="text-muted-foreground">Izin diperlukan</dt>
							<dd>{detail.required_permission || '—'}</dd>
							<dt class="text-muted-foreground">Dibuat</dt>
							<dd>{fmtDateTime(detail.created_at)}</dd>
							<dt class="text-muted-foreground">Kedaluwarsa</dt>
							<dd>{fmtDateTime(detail.expires_at)}</dd>
							<dt class="text-muted-foreground">Diputuskan</dt>
							<dd>{detail.decided_at ? fmtDateTime(detail.decided_at) : '—'}</dd>
							<dt class="text-muted-foreground">Dieksekusi</dt>
							<dd>{detail.executed_at ? fmtDateTime(detail.executed_at) : '—'}</dd>
							<dt class="text-muted-foreground">Oleh admin</dt>
							<dd>{detail.approver_email || detail.approved_by || '—'}</dd>
							{#if detail.error}
								<dt class="text-muted-foreground">Error</dt>
								<dd class="text-red-600">{detail.error}</dd>
							{/if}
						</dl>
						<div class="mt-4">
							<p class="text-muted-foreground text-xs">Ringkasan</p>
							<pre
								class="bg-muted/30 mt-1 whitespace-pre-wrap rounded p-3 text-xs"
							>{detail.summary}</pre>
						</div>
						<div class="mt-4">
							<p class="text-muted-foreground text-xs">Parameter</p>
							<pre
								class="bg-muted/30 mt-1 overflow-x-auto rounded p-3 text-xs"
							>{JSON.stringify(detail.params ?? {}, null, 2)}</pre>
						</div>
						{#if detail.result}
							<div class="mt-4">
								<p class="text-muted-foreground text-xs">Hasil</p>
								<pre
									class="bg-muted/30 mt-1 overflow-x-auto rounded p-3 text-xs"
								>{JSON.stringify(detail.result, null, 2)}</pre>
							</div>
						{/if}
					{:else}
						<p class="text-muted-foreground">Gagal memuat detail.</p>
					{/if}
				</div>
			</div>
		</div>
	{/if}
</section>
