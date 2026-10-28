<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Detail tiket dukungan: percakapan mitra ↔ admin. Membuka halaman ini
	// menandai balasan admin sudah dibaca di server. Polling 20 detik selama
	// tab terlihat supaya balasan baru muncul tanpa reload.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
	import MultilineText from '$lib/components/multiline-text.svelte';
	import { ArrowLeft, Check, RotateCcw, Send } from '@lucide/svelte';
	import { refreshSupportUnreadNow } from '$lib/partner/support-unread.svelte';

	type Ticket = {
		id: string;
		code: string;
		subject: string;
		category: string;
		priority: string;
		status: string;
		last_message_at: string | null;
		created_at: string | null;
		closed_at: string | null;
	};
	type Message = {
		id: string;
		author_role: string;
		body: string;
		created_at: string | null;
	};

	const base = $derived(`/mitra/${page.params.partnerId}`);
	const ticketId = $derived(page.params.id as string);
	let ticket = $state<Ticket | null>(null);
	let messages = $state<Message[]>([]);
	let loading = $state(true);
	let error = $state('');
	let sending = $state(false);
	let statusBusy = $state(false);
	let draft = $state('');
	let threadEl = $state<HTMLDivElement | null>(null);
	let lastMsgCount = 0;

	// Pesan baru → gulir ke bawah. Polling 20 detik yang tidak menambah pesan
	// tidak menggeser posisi baca.
	$effect(() => {
		if (messages.length > lastMsgCount && threadEl) {
			threadEl.scrollTop = threadEl.scrollHeight;
		}
		lastMsgCount = messages.length;
	});

	const statusLabels = $derived<Record<string, string>>({ buka: tr('Buka'), diproses: tr('Diproses'), selesai: tr('Selesai') });
	const categoryLabels = $derived<Record<string, string>>({
		teknis: tr('Teknis'),
		pembayaran: tr('Pembayaran'),
		publikasi: tr('Publikasi'),
		lainnya: tr('Lainnya')
	});
	const priorityLabels = $derived<Record<string, string>>({ rendah: tr('Rendah'), normal: tr('Normal'), tinggi: tr('Tinggi') });

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		const timer = setInterval(() => {
			if (document.visibilityState === 'visible') void load(true);
		}, 20_000);
		return () => {
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
			clearInterval(timer);
		};
	});

	async function load(silent = false) {
		if (!silent) error = '';
		try {
			const res = await apiRequest(`/v1/partner/tickets/${ticketId}`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Tiket tidak ditemukan.')));
			const data = await res.json();
			ticket = data.ticket as Ticket;
			messages = (data.messages ?? []) as Message[];
			await refreshSupportUnreadNow();
		} catch (e) {
			if (!silent) error = e instanceof Error ? e.message : tr('Tiket tidak ditemukan.');
		}
	}

	async function send() {
		if (sending || !draft.trim()) return;
		sending = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/tickets/${ticketId}/messages`, {
				method: 'POST',
				body: JSON.stringify({ body: draft })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal mengirim balasan.')));
			draft = '';
			await load(true);
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal mengirim balasan.');
		} finally {
			sending = false;
		}
	}

	async function setStatus(next: 'buka' | 'selesai') {
		if (statusBusy) return;
		statusBusy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/tickets/${ticketId}/status`, {
				method: 'POST',
				body: JSON.stringify({ status: next })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal mengubah status tiket.')));
			const data = await res.json();
			ticket = data.ticket as Ticket;
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal mengubah status tiket.');
		} finally {
			statusBusy = false;
		}
	}

	function parse(iso: string | null): Date | null {
		if (!iso) return null;
		const d = new Date(iso);
		return Number.isNaN(d.getTime()) ? null : d;
	}

	function when(iso: string | null): string {
		const d = parse(iso);
		if (!d) return '';
		return d.toLocaleString('id-ID', { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' });
	}

	function dayKey(iso: string | null): string {
		const d = parse(iso);
		return d ? `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}` : '';
	}

	function dayLabel(iso: string | null): string {
		const d = parse(iso);
		if (!d) return '';
		return d.toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' });
	}

	function statusStyle(status: string): string {
		if (status === 'buka') return 'background-color: color-mix(in srgb, var(--primary) 12%, transparent); color: var(--primary);';
		if (status === 'diproses') return 'background-color: color-mix(in srgb, #d97706 14%, transparent); color: #b45309;';
		return 'background-color: var(--secondary); color: var(--muted-foreground);';
	}
</script>

<PageTitle title={ticket ? ticket.subject : tr('Dukungan')} />

<section class="mx-auto flex max-w-3xl flex-col gap-3 lg:h-[calc(100dvh-160px)] lg:min-h-[540px]">
	<a class="inline-flex shrink-0 items-center gap-1.5 text-xs font-medium" style="color: var(--muted-foreground);" href={`${base}/dukungan`}>
		<ArrowLeft size={13} /> {tr('Kembali ke Dukungan')}
	</a>

	{#if loading}
		<div class="app-card h-40 animate-pulse"></div>
	{:else if !ticket}
		<div class="app-card px-4 py-3 text-sm" style="color: var(--destructive);">{error || tr('Tiket tidak ditemukan.')}</div>
	{:else}
		<header class="app-card shrink-0 px-5 py-4">
			<div class="flex flex-wrap items-start justify-between gap-4">
				<div class="min-w-0">
					<h1 class="text-base font-semibold leading-snug">{ticket.subject}</h1>
					<p class="mt-1.5 flex flex-wrap items-center gap-x-2 text-[11px] text-muted-foreground">
						<span>#{ticket.code}</span>
						<span>·</span>
						<span>{categoryLabels[ticket.category] ?? ticket.category}</span>
						<span>·</span>
						<span>{tr('Prioritas')} {priorityLabels[ticket.priority] ?? ticket.priority}</span>
						<span>·</span>
						<span>{tr('Dibuat')} {when(ticket.created_at)}</span>
					</p>
				</div>
				<div class="flex shrink-0 items-center gap-3">
					<span class="rounded-full px-2 py-0.5 text-[11px] font-semibold" style={statusStyle(ticket.status)}>
						{statusLabels[ticket.status] ?? ticket.status}
					</span>
					<span class="h-6 w-px" style="background-color: var(--border);" aria-hidden="true"></span>
					{#if ticket.status === 'selesai'}
						<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" disabled={statusBusy} onclick={() => void setStatus('buka')}>
							<RotateCcw size={13} /> {tr('Buka lagi')}
						</button>
					{:else}
						<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" disabled={statusBusy} onclick={() => void setStatus('selesai')}>
							<Check size={13} /> {tr('Tandai selesai')}
						</button>
					{/if}
				</div>
			</div>
		</header>

		{#if error}<div class="app-card px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

		<section class="app-card flex min-h-0 flex-1 flex-col overflow-hidden" aria-label={tr('Percakapan')}>
			<div class="shrink-0 px-5 pt-4">
				<h2 class="text-sm font-semibold">{tr('Percakapan')}</h2>
			</div>

			<div class="mt-1 min-h-0 flex-1 overflow-y-auto px-5" bind:this={threadEl}>
				<ol class="flex min-h-full flex-col justify-end gap-3 pb-4">
					{#each messages as m, i (m.id)}
						{@const prev = i > 0 ? messages[i - 1] : null}
						{#if i === 0 || dayKey(m.created_at) !== dayKey(prev?.created_at ?? null)}
							<li class="flex list-none items-center gap-3 py-1" aria-hidden="true">
								<span class="h-px flex-1" style="background-color: var(--border);"></span>
								<span class="text-[11px]" style="color: var(--muted-foreground);">{dayLabel(m.created_at)}</span>
								<span class="h-px flex-1" style="background-color: var(--border);"></span>
							</li>
						{/if}
						<li class="flex" class:justify-end={m.author_role === 'mitra'}>
							<div
								class="max-w-[78%] rounded-xl px-3.5 py-2.5"
								style={m.author_role === 'mitra'
									? 'background-color: color-mix(in srgb, var(--primary) 9%, #fff);'
									: 'background-color: var(--secondary);'}
							>
								<p class="text-[11px] font-semibold" style="color: var(--muted-foreground);">
									{m.author_role === 'mitra' ? tr('Anda') : tr('Admin Tauthub')}
								</p>
								<MultilineText class="mt-1 break-words text-sm leading-relaxed" text={m.body} />
								<p class="mt-1.5 text-[10px]" style="color: var(--muted-foreground);">{when(m.created_at)}</p>
							</div>
						</li>
					{/each}
				</ol>
			</div>

			<form
				class="shrink-0 border-t px-5 py-4"
				style="border-color: var(--border);"
				onsubmit={(e) => { e.preventDefault(); void send(); }}
			>
				<label class="block text-sm font-semibold" for="ticket-reply">{tr('Balas')}</label>
				<textarea
					id="ticket-reply"
					class="app-input mt-2 min-h-28 w-full resize-y"
					maxlength="5000"
					bind:value={draft}
					placeholder={tr('Tulis balasan untuk admin…')}
				></textarea>
				<div class="mt-2 flex items-center justify-between gap-2">
					<p class="text-[11px] text-muted-foreground">
						{#if ticket.status === 'selesai'}
							{tr('Tiket sudah selesai; balasan baru akan membukanya kembali.')}
						{:else}
							{tr('Admin biasanya membalas pada jam kerja.')}
						{/if}
					</p>
					<button type="submit" class="app-btn app-btn-primary" disabled={sending || !draft.trim()}>
						<Send size={14} /> {sending ? tr('Mengirim…') : tr('Kirim')}
					</button>
				</div>
			</form>
		</section>
	{/if}
</section>
