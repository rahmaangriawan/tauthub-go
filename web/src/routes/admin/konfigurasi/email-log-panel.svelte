<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Log pengiriman email (tab "Log Email" di Konfigurasi): semua email transaksional
	// yang dikirim ke mitra dan klien, lengkap dengan status dan sebab gagalnya.
	import { onMount } from 'svelte';
	import { Search } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, ensureValidSession } from '$lib/auth';
	import { fmtDateTime, fmtNum } from '$lib/admin-format';
	import { platformError } from '../_platform';

	type Item = {
		id: string;
		template: string;
		template_label: string;
		subject: string | null;
		recipient_email: string;
		recipient_role: string | null;
		recipient_name: string | null;
		status: string;
		attempts: number;
		max_attempts: number;
		error_code: string | null;
		error_message: string | null;
		sent_at: string | null;
		created_at: string | null;
	};

	const PAGE = 50;

	let items = $state<Item[]>([]);
	let total = $state(0);
	let offset = $state(0);
	let summary = $state<Record<string, number>>({});
	let templates = $state<Array<{ template: string; label: string; count: number }>>([]);
	let status = $state('');
	let template = $state('');
	let role = $state('');
	let query = $state('');
	let loading = $state(true);
	let error = $state('');
	let timer: ReturnType<typeof setTimeout> | null = null;

	const STATUS_LABEL: Record<string, string> = {
		sent: 'Terkirim',
		failed: 'Gagal',
		queued: 'Antre',
		sending: 'Mengirim',
		rate_limited: 'Ditahan batas kirim',
		skipped: 'Dilewati'
	};
	const statusTone = (s: string): 'success' | 'danger' | 'warning' | 'neutral' =>
		s === 'sent' ? 'success' : s === 'failed' ? 'danger' : s === 'skipped' ? 'neutral' : 'warning';
	const roleLabel = (r: string | null) => (r === 'partner' ? tr('Mitra') : r === 'client' ? tr('Klien') : r === 'admin' ? 'Admin' : '—');

	async function load() {
		loading = true;
		error = '';
		const params = new URLSearchParams({ limit: String(PAGE), offset: String(offset) });
		if (status) params.set('status', status);
		if (template) params.set('template', template);
		if (role) params.set('role', role);
		if (query.trim()) params.set('q', query.trim());
		const res = await apiRequest(`/v1/platform/settings/email/deliveries?${params}`);
		if (res.ok) {
			const data = await res.json();
			items = data.items ?? [];
			total = data.total ?? 0;
			summary = data.summary ?? {};
			templates = data.templates ?? [];
		} else {
			error = await platformError(res);
		}
		loading = false;
	}

	function refilter() {
		offset = 0;
		void load();
	}

	function onSearch(value: string) {
		query = value;
		if (timer) clearTimeout(timer);
		timer = setTimeout(refilter, 300);
	}

	onMount(() => {
		void (async () => {
			if (!(await ensureValidSession())) return;
			await load();
		})();
		return () => {
			if (timer) clearTimeout(timer);
		};
	});

	const queuedCount = $derived((summary.queued ?? 0) + (summary.sending ?? 0) + (summary.rate_limited ?? 0));
</script>

<div class="flex flex-col gap-4">
	<div class="grid grid-cols-2 gap-3 md:grid-cols-4">
		{#each [['sent', 'Terkirim', summary.sent ?? 0, 'sent'], ['failed', 'Gagal', summary.failed ?? 0, 'failed'], ['queued', 'Antre', queuedCount, 'queued'], ['skipped', 'Dilewati', summary.skipped ?? 0, 'skipped']] as [key, label, count, filterValue] (key)}
			<button
				type="button"
				class="app-card px-4 py-3 text-left"
				style={status === filterValue ? 'box-shadow: inset 0 0 0 1.5px var(--primary);' : ''}
				onclick={() => {
					status = status === filterValue ? '' : String(filterValue);
					refilter();
				}}
			>
				<p class="text-xs text-muted-foreground">{tr(String(label))}</p>
				<p class="mt-1 text-xl font-medium tabular-nums" style={key === 'failed' && Number(count) > 0 ? 'color: var(--destructive);' : ''}>{fmtNum(count)}</p>
			</button>
		{/each}
	</div>

	<div class="app-card overflow-x-auto">
		<div class="border-b px-5 py-4" style="border-color: var(--border);">
			<h2 class="text-sm font-medium">{tr('Log pengiriman email')}</h2>
			<p class="mt-0.5 text-xs text-muted-foreground">
				{tr('Email yang dikirim sistem ke mitra dan klien. Email gagal menampilkan sebab dari server email.')}
			</p>
		</div>

		<div class="flex flex-wrap items-center gap-2 px-5 py-3">
			<div class="relative min-w-[220px] flex-1">
				<Search size={14} strokeWidth={1.5} class="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-muted-foreground" />
				<input
					class="app-input !min-h-8 w-full !py-1 !pl-8 text-sm"
					type="search"
					placeholder={tr('Cari alamat email')}
					aria-label={tr('Cari alamat email')}
					value={query}
					oninput={(e) => onSearch(e.currentTarget.value)}
				/>
			</div>
			<Select
				ariaLabel={tr('Status')}
				minWidth={150}
				value={status}
				onchange={(v) => {
					status = v;
					refilter();
				}}
				options={[
					{ value: '', label: tr('Semua status') },
					...Object.entries(STATUS_LABEL).map(([value, label]) => ({ value, label: tr(label) }))
				]}
			/>
			<Select
				ariaLabel={tr('Jenis email')}
				minWidth={190}
				value={template}
				onchange={(v) => {
					template = v;
					refilter();
				}}
				options={[{ value: '', label: tr('Semua jenis email') }, ...templates.map((t) => ({ value: t.template, label: `${t.label} (${fmtNum(t.count)})` }))]}
			/>
			<Select
				ariaLabel={tr('Penerima')}
				minWidth={140}
				value={role}
				onchange={(v) => {
					role = v;
					refilter();
				}}
				options={[
					{ value: '', label: tr('Semua penerima') },
					{ value: 'partner', label: tr('Mitra') },
					{ value: 'client', label: tr('Klien') }
				]}
			/>
		</div>

		{#if error}<p class="px-5 pb-2 text-xs" style="color: var(--destructive);">{error}</p>{/if}

		{#if items.length > 0}
			<table class="admin-table w-full text-sm">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-5 py-2.5 font-medium">{tr('Waktu')}</th>
						<th class="px-5 py-2.5 font-medium">{tr('Penerima')}</th>
						<th class="px-5 py-2.5 font-medium">{tr('Jenis email')}</th>
						<th class="px-5 py-2.5 font-medium">Status</th>
						<th class="px-5 py-2.5 font-medium">{tr('Keterangan')}</th>
					</tr>
				</thead>
				<tbody>
					{#each items as row (row.id)}
						<tr class="border-t align-top">
							<td class="whitespace-nowrap px-5 py-2.5 text-xs text-muted-foreground tabular-nums">{fmtDateTime(row.sent_at ?? row.created_at)}</td>
							<td class="px-5 py-2.5">
								<p class="text-xs">{row.recipient_email}</p>
								<p class="text-[11px] text-muted-foreground">{roleLabel(row.recipient_role)}{row.recipient_name ? ` · ${row.recipient_name}` : ''}</p>
							</td>
							<td class="max-w-[280px] px-5 py-2.5">
								<p class="text-xs font-medium">{row.template_label}</p>
								{#if row.subject}<p class="text-[11px] text-muted-foreground">{tr('Subjek')}: {row.subject}</p>{/if}
								<p class="font-mono text-[10px] text-muted-foreground/70">{row.template}</p>
							</td>
							<td class="px-5 py-2.5"><Badge tone={statusTone(row.status)} label={tr(STATUS_LABEL[row.status] ?? row.status)} /></td>
							<td class="max-w-[360px] px-5 py-2.5 text-xs text-muted-foreground">
								{#if row.error_message}
									<span style="color: var(--destructive);">{row.error_message}</span>
								{/if}
								{#if row.status !== 'sent' && row.attempts > 0}
									<span class="block tabular-nums">{tr('Percobaan')} {row.attempts}/{row.max_attempts}</span>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{:else if loading}
			<p class="px-5 pb-4 text-xs text-muted-foreground">{tr('Memuat…')}</p>
		{:else}
			<p class="px-5 pb-4 text-xs text-muted-foreground">{tr('Tidak ada email yang cocok dengan filter ini.')}</p>
		{/if}

		<div class="px-5 pb-4">
			<Pagination {total} limit={PAGE} {offset} onChange={(next) => { offset = next; void load(); }} />
		</div>
	</div>
</div>
