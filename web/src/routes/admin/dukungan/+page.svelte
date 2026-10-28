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
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import MultilineText from '$lib/components/multiline-text.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';
	import { Send, ArrowDown, ArrowUpDown } from '@lucide/svelte';

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
	const STATUS_TONES: Record<TicketStatus, 'warning' | 'accent' | 'success'> = {
		buka: 'warning',
		diproses: 'accent',
		selesai: 'success'
	};
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
			const res = await apiRequest(`/v1/platform/tickets?${qs.toString()}`);
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
	<PageHeader title={tr('Dukungan')} description={tr('Tiket dukungan dari mitra. Balasan dikirim ke lonceng, email, dan WhatsApp mitra.')}>
		<Badge tone="accent" label={`${total} tiket`} />
	</PageHeader>

	<Feedback {loading} {error} loadingText={tr('Memuat tiket…')} />

	{#if !loading && rows.length === 0 && error.includes('Akses platform')}
		<div class="app-card mt-4 px-4 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr('Akun ini tidak memiliki akses ke tiket dukungan.')}</p>
		</div>
	{:else}
		<div class="mt-4 flex flex-wrap items-center gap-3">
			<input
				class="app-input w-full max-w-md"
				placeholder={tr('Cari subjek atau nama mitra…')}
				value={query}
				oninput={(e) => onQueryInput((e.currentTarget as HTMLInputElement).value)}
			/>
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
		</div>

		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 940px;">
				<thead>
					<tr class="text-xs text-muted-foreground">
						<th class="px-4 py-2.5 text-left font-medium">{tr('Status')}</th>
						<th class="px-4 py-2.5 text-left font-medium">{tr('Subjek')}</th>
						<th class="px-4 py-2.5 text-left font-medium">{tr('Mitra')}</th>
						<th class="px-4 py-2.5 text-left font-medium">{tr('Kategori')}</th>
						<th class="px-4 py-2.5 text-left font-medium" aria-sort={priorityDesc ? 'ascending' : 'descending'}>
							<button
								class="inline-flex items-center gap-1 font-medium hover:text-foreground"
								type="button"
								onclick={togglePrioritySort}
								aria-label={priorityDesc ? tr('Urutkan prioritas dari yang tertinggi') : tr('Urutkan prioritas dari yang terendah')}
							>
								{tr('Prioritas')}
								{#if priorityDesc}<ArrowDown size={12} strokeWidth={2.4} />{:else}<ArrowUpDown size={12} strokeWidth={2.4} />{/if}
							</button>
						</th>
						<th class="px-4 py-2.5 text-left font-medium">{tr('Pesan terakhir')}</th>
						<th class="px-4 py-2.5 text-left font-medium">{tr('Aksi')}</th>
					</tr>
				</thead>
				<tbody>
					{#each rows as t (t.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5"><Badge tone={STATUS_TONES[t.status]} label={STATUS_LABELS[t.status]} /></td>
							<td class="px-4 py-2.5">
								<button class="text-left font-medium hover:underline" type="button" onclick={() => void openDetail(t)}>
									{t.subject}
									{#if t.unread > 0}<span class="ml-1.5 rounded-full px-1.5 text-[10px] font-bold text-white" style="background-color: var(--destructive, #e11d48);">{t.unread}</span>{/if}
								</button>
								<p class="text-[11px] text-muted-foreground">#{t.code}</p>
							</td>
							<td class="px-4 py-2.5">{t.partner_name || '-'}</td>
							<td class="px-4 py-2.5 text-muted-foreground">{CATEGORY_LABELS[t.category] ?? t.category}</td>
							<td class="px-4 py-2.5 text-muted-foreground">{PRIORITY_LABELS[t.priority] ?? t.priority}</td>
							<td class="px-4 py-2.5 text-muted-foreground">
								<p>{preview(t.last_body) || '-'}</p>
								<p class="text-[11px]">{fmtDateTime(t.last_message_at)}</p>
							</td>
							<td class="px-4 py-2.5">
								<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void openDetail(t)}>{tr('Buka')}</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && rows.length === 0}
				<p class="px-4 py-12 text-center text-sm text-muted-foreground">{tr('Tidak ada tiket pada filter ini.')}</p>
			{/if}
		</div>
		<Pagination total={total} limit={PAGE_SIZE} offset={(pageNo - 1) * PAGE_SIZE} onChange={(next) => { pageNo = next / PAGE_SIZE + 1; void load(); }} />
	{/if}
</section>

{#if detail}
	<Modal title={[detail.code ? `#${detail.code}` : '', detail.subject].filter(Boolean).join(' · ')} label={tr('Detail tiket dukungan')} maxWidth="xl" onClose={() => (detail = null)}>
		<div class="space-y-3">
			<div class="flex flex-wrap items-center gap-2">
				<Badge tone={STATUS_TONES[detail.status]} label={STATUS_LABELS[detail.status]} />
				<Badge tone="neutral" label={CATEGORY_LABELS[detail.category] ?? detail.category} />
				<Badge tone={detail.priority === 'tinggi' ? 'danger' : 'neutral'} label={PRIORITY_LABELS[detail.priority] ?? detail.priority} />
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
