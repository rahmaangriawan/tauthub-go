<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Log Obrolan — pemantauan admin atas percakapan klien ↔ mitra.
	// HANYA BACA: admin tidak bisa mengirim pesan, dan membuka percakapan di
	// sini tidak mengubah status "sudah dibaca" milik peserta.
	import { onMount } from 'svelte';
	import { MessageCircle, Search } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';
	import { platformError } from '../_platform';

	type Thread = {
		id: string;
		brand_name: string;
		media_name: string;
		partner_name: string;
		article_title: string;
		status: 'waiting' | 'open' | 'expired';
		message_count: number;
		started_at: string | null;
		expires_at: string | null;
		last_message_at: string | null;
		last_message_by: string | null;
		created_at: string;
		is_flagged?: boolean;
		flagged_count?: number;
		last_message?: { body: string; author_role: string } | null;
	};
	type Message = {
		id: string;
		author_role: 'client' | 'partner';
		author_name: string;
		body: string;
		is_flagged?: boolean;
		/** Hanya dikirim server untuk super admin. */
		original_body?: string;
		created_at: string;
	};

	let loading = $state(true);
	let error = $state('');
	let threads = $state<Thread[]>([]);
	let query = $state('');
	let detail = $state<Thread | null>(null);
	let messages = $state<Message[]>([]);
	let detailLoading = $state(false);
	let detailError = $state('');
	let revealed = $state<Record<string, boolean>>({});
	let searchTimer: ReturnType<typeof setTimeout> | null = null;

	async function load() {
		loading = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/chat/threads?q=${encodeURIComponent(query.trim())}`);
			if (!res.ok) throw new Error(await platformError(res, tr('Gagal memuat log obrolan.')));
			threads = ((await res.json()).threads ?? []) as Thread[];
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal memuat log obrolan.');
		} finally {
			loading = false;
		}
	}

	function onSearchInput() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => void load(), 350);
	}

	async function openDetail(t: Thread) {
		detail = t;
		detailLoading = true;
		detailError = '';
		messages = [];
		revealed = {};
		try {
			const res = await apiRequest(`/v1/platform/chat/threads/${t.id}`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat pesan.')));
			const d = await res.json();
			messages = (d.messages ?? []) as Message[];
		} catch (reason) {
			detailError = reason instanceof Error ? reason.message : tr('Gagal memuat pesan.');
		} finally {
			detailLoading = false;
		}
	}

	const statusLabel = (s: string) =>
		s === 'open' ? tr('Berjalan') : s === 'waiting' ? tr('Belum dimulai') : tr('Selesai');
	const statusTone = (s: string): 'success' | 'neutral' | 'warning' =>
		s === 'open' ? 'success' : s === 'waiting' ? 'neutral' : 'warning';

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			if (searchTimer) clearTimeout(searchTimer);
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
		};
	});
</script>

<PageHeader
	title={tr('Log Obrolan')}
	description={tr('Riwayat percakapan klien ↔ mitra. Hanya baca — admin tidak dapat mengirim pesan ke ruang obrolan.')}
/>

<div class="mt-4">
	<div class="relative max-w-md">
		<Search size={14} class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
		<input
			class="app-input pl-8"
			type="text"
			placeholder={tr('Cari media, brand, atau mitra…')}
			bind:value={query}
			oninput={onSearchInput}
		/>
	</div>
</div>

{#if error}
	<p class="mt-3 text-sm" style="color: var(--destructive);">{error}</p>
{/if}

<section class="app-card mt-3 overflow-hidden p-0">
	{#if loading}
		<p class="px-4 py-8 text-center text-sm text-muted-foreground">{tr('Memuat…')}</p>
	{:else if threads.length === 0}
		<p class="px-4 py-8 text-center text-sm text-muted-foreground">{tr('Belum ada percakapan.')}</p>
	{:else}
		<div class="overflow-x-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="border-b border-slate-100 text-left text-[10.5px] font-bold uppercase tracking-[0.07em] text-slate-400">
						<th class="px-4 py-3">Media</th>
						<th class="px-4 py-3">Brand</th>
						<th class="px-4 py-3">Mitra</th>
						<th class="px-4 py-3">Status</th>
						<th class="px-4 py-3">Pesan</th>
						<th class="px-4 py-3">Aktivitas Terakhir</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-slate-100">
					{#each threads as t (t.id)}
						<tr class="cursor-pointer transition-colors hover:bg-slate-50/60" onclick={() => void openDetail(t)}>
							<td class="px-4 py-3">
								<div class="flex items-center gap-2">
									<p class="text-[13px] font-semibold text-slate-900">{t.media_name || '—'}</p>
									{#if t.is_flagged}
										<span class="shrink-0 rounded-full border border-rose-200 bg-rose-50 px-2 py-0.5 text-[10px] font-bold text-rose-600">
											{tr('Indikasi Bypass')}{#if (t.flagged_count ?? 0) > 0} · {t.flagged_count}{/if}
										</span>
									{/if}
								</div>
								{#if t.article_title}<p class="mt-0.5 max-w-[280px] truncate text-[11.5px] text-slate-500">{t.article_title}</p>{/if}
							</td>
							<td class="px-4 py-3 text-[12.5px] text-slate-700">{t.brand_name || '—'}</td>
							<td class="px-4 py-3 text-[12.5px] text-slate-700">{t.partner_name || '—'}</td>
							<td class="px-4 py-3"><Badge label={statusLabel(t.status)} tone={statusTone(t.status)} /></td>
							<td class="px-4 py-3 tabular-nums text-slate-700">{t.message_count}</td>
							<td class="px-4 py-3 whitespace-nowrap text-[12px] text-slate-500">{fmtDateTime(t.last_message_at ?? t.created_at)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>

{#if detail}
	<Modal title={`${detail.media_name} · ${tr('Log Obrolan')}`} label={`${detail.brand_name} ↔ ${detail.partner_name}`} maxWidth="2xl" onClose={() => (detail = null)}>
		<div class="flex items-center gap-2 border-b border-slate-100 pb-3">
			<Badge label={statusLabel(detail.status)} tone={statusTone(detail.status)} />
			<span class="text-[11.5px] text-muted-foreground">
				{tr('Mulai')}: {fmtDateTime(detail.started_at)} · {tr('Berakhir')}: {fmtDateTime(detail.expires_at)}
			</span>
		</div>
		<p class="mt-2 flex items-center gap-1.5 text-[11.5px] text-muted-foreground">
			<MessageCircle size={12} /> {tr('Tampilan admin — hanya baca.')}
		</p>
		<p class="mt-1 text-[10.5px] text-slate-400">
			{tr('Akses Anda ke ruang obrolan ini tercatat di dalam log audit sistem.')}
		</p>

		{#if detailLoading}
			<p class="py-6 text-center text-sm text-muted-foreground">{tr('Memuat…')}</p>
		{:else if detailError}
			<p class="py-3 text-sm" style="color: var(--destructive);">{detailError}</p>
		{:else if messages.length === 0}
			<p class="py-6 text-center text-sm text-muted-foreground">{tr('Belum ada pesan.')}</p>
		{:else}
			<div class="mt-3 max-h-[58vh] space-y-2 overflow-y-auto pr-1">
				{#each messages as m (m.id)}
					{@const mine = m.author_role === 'client'}
					<div class="flex {mine ? 'justify-start' : 'justify-end'}">
						<div class="max-w-[80%] rounded-2xl border {m.is_flagged ? 'border-rose-200 bg-rose-50/40' : 'border-slate-200 bg-white'} px-3.5 py-2.5 text-[12.5px] leading-relaxed text-slate-800">
							<p class="mb-0.5 text-[10.5px] font-bold text-slate-500">
								{m.author_role === 'client' ? tr('Klien') : tr('Mitra')}{#if m.author_name} · {m.author_name}{/if}
							</p>
							{#if m.is_flagged}
								<p class="mb-1 inline-flex items-center gap-1 rounded-full bg-rose-100 px-2 py-0.5 text-[10px] font-bold text-rose-700">
									{tr('Terdeteksi indikasi bypass')}
								</p>
							{/if}
							<p class="whitespace-pre-wrap break-words">{m.body}</p>
							{#if m.original_body && m.original_body !== m.body}
								<button
									class="mt-1 text-[10.5px] font-semibold text-blue-600 hover:underline"
									type="button"
									onclick={() => (revealed = { ...revealed, [m.id]: !revealed[m.id] })}
								>
									{revealed[m.id] ? tr('Sembunyikan pesan asli') : tr('Lihat pesan asli')}
								</button>
								{#if revealed[m.id]}
									<p class="mt-1 rounded-lg bg-white px-2 py-1.5 text-[11.5px] text-slate-700 ring-1 ring-rose-100">{m.original_body}</p>
								{/if}
							{/if}
							<p class="mt-1 text-right text-[10px] text-slate-400">{fmtDateTime(m.created_at)}</p>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</Modal>
{/if}
