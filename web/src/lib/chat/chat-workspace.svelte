<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Obrolan klien ↔ mitra: satu komponen untuk dua portal (klien & mitra),
	// karena bentuk respons API-nya identik. Layout tiga kolom:
	// sidebar daftar forum — kolom pesan — sidebar info.
	//
	// Sesi: window 3×24 jam mulai saat pesan pertama KLIEN dikirim. Sebelum
	// itu "Belum dimulai"; setelah lewat "expired" → komposer mati, riwayat
	// tetap bisa dibaca dua pihak.
	import { onMount, onDestroy } from 'svelte';
	import { SendHorizonal, MessagesSquare, ExternalLink, Info, Clock3 } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type Thread = {
		id: string;
		media_id: string;
		media_name: string;
		media_type: string;
		website_url: string;
		da: number | null;
		partner_name: string;
		partner_public_id: string;
		brand_name: string;
		brand_public_id: string;
		publication_id: string | null;
		article_title: string;
		pub_status: string | null;
		pub_url: string | null;
		started_at: string | null;
		expires_at: string | null;
		last_message_at: string | null;
		last_message_by: string | null;
		status: 'waiting' | 'open' | 'expired';
		unread_count?: number;
		last_message?: { body: string; author_role: string; created_at: string } | null;
	};

	type Message = {
		id: string;
		author_role: 'client' | 'partner';
		author_name: string;
		body: string;
		read_at: string | null;
		created_at: string;
	};

	let {
		apiBase,
		side,
		initialThread = '',
		initialPublication = ''
	}: {
		apiBase: string;
		side: 'client' | 'partner';
		initialThread?: string;
		initialPublication?: string;
	} = $props();

	let threads = $state<Thread[]>([]);
	let active = $state<Thread | null>(null);
	let messages = $state<Message[]>([]);
	let draft = $state('');
	let sending = $state(false);
	let sendError = $state('');
	let bootError = $state('');
	let loadingList = $state(true);
	let loadingThread = $state(false);
	let now = $state(Date.now());

	let messagesEl = $state<HTMLDivElement | null>(null);
	let composerEl = $state<HTMLTextAreaElement | null>(null);

	const DAY_MS = 86_400_000;

	// Palet avatar sama dengan tabel ringkasan (hash nama → indeks).
	const AVA_BG = ['#e0e7ff', '#dcfce7', '#fef3c7', '#f1f5f9', '#dbeafe', '#fce7f3'];
	const AVA_FG = ['#4338ca', '#15803d', '#b45309', '#475569', '#1d4ed8', '#be185d'];
	const avatarTone = (name: string) => {
		let h = 0;
		for (let i = 0; i < name.length; i += 1) h = (h * 31 + name.charCodeAt(i)) % 6;
		return h;
	};
	const initialOf = (name: string) => (name || '?').trim().charAt(0).toUpperCase() || '?';

	// Favicon media dari website_url (pola sama dengan tabel Ringkasan);
	// gagal muat → jatuh kembali ke inisial nama.
	const cleanDomain = (url: string | null | undefined, name: string): string =>
		((url ?? '').replace(/^https?:\/\/(www\.)?/i, '').replace(/\/+$/, '').trim() || name.trim()).toLowerCase();
	let faviconFailed = $state<Record<string, boolean>>({});

	const otherName = (t: Thread) => (side === 'client' ? t.partner_name || t.media_name : t.brand_name || tr('Klien'));
	const otherLabel = $derived(side === 'client' ? tr('Mitra') : tr('Klien'));

	// ---------- waktu ----------

	const fmtTime = (iso: string | null) =>
		iso ? new Date(iso).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' }) : '';
	const fmtDay = (iso: string) => {
		const d = new Date(iso);
		const today = new Date();
		const yesterday = new Date(today.getTime() - DAY_MS);
		const same = (a: Date, b: Date) => a.toDateString() === b.toDateString();
		if (same(d, today)) return tr('Hari ini');
		if (same(d, yesterday)) return tr('Kemarin');
		return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'long', year: 'numeric' });
	};
	const fmtDateTime = (iso: string | null) =>
		iso
			? new Date(iso).toLocaleString('id-ID', { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })
			: '—';

	// Hitung mundur berbasis `now` (state yang di-tick tiap 30 detik).
	const countdown = (t: Thread | null): string => {
		if (!t) return '';
		if (t.status === 'waiting' || !t.expires_at) return tr('Belum dimulai');
		const left = new Date(t.expires_at).getTime() - now;
		if (left <= 0) return tr('Sesi berakhir');
		const h = Math.floor(left / 3_600_000);
		const m = Math.max(0, Math.floor((left % 3_600_000) / 60_000));
		if (h <= 0 && m <= 0) return tr('Kurang dari 1 menit');
		return `${tr('Sisa')} ${h > 0 ? `${h}j ` : ''}${m}m`;
	};

	const previewOf = (t: Thread) => {
		if (!t.last_message) return tr('Belum ada pesan');
		const who = t.last_message.author_role === side ? tr('Kamu') : otherName(t);
		return `${who}: ${t.last_message.body}`;
	};

	// ---------- data ----------

	async function loadThreads(silent = false) {
		if (!silent) loadingList = true;
		try {
			const res = await apiRequest(`${apiBase}/chat/threads`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat daftar obrolan.')));
			const d = await res.json();
			threads = (d.threads ?? []) as Thread[];
			if (active) {
				const fresh = threads.find((t) => t.id === active?.id);
				if (fresh) active = { ...active, ...fresh, unread_count: 0 };
			}
		} catch (reason) {
			if (!silent) bootError = reason instanceof Error ? reason.message : tr('Gagal memuat daftar obrolan.');
		} finally {
			loadingList = false;
		}
	}

	function scrollToBottom() {
		if (messagesEl) messagesEl.scrollTop = messagesEl.scrollHeight;
	}

	async function openThread(id: string, opts: { push?: boolean } = {}) {
		if (!id) return;
		loadingThread = true;
		sendError = '';
		try {
			const res = await apiRequest(`${apiBase}/chat/threads/${id}`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal membuka obrolan.')));
			const d = await res.json();
			active = d.thread as Thread;
			messages = (d.messages ?? []) as Message[];
			if (opts.push !== false) {
				const url = new URL(window.location.href);
				url.searchParams.set('t', id);
				url.searchParams.delete('pub');
				history.replaceState({}, '', url);
			}
			await Promise.resolve();
			scrollToBottom();
			composerEl?.focus();
			if (threads.some((t) => t.id === id)) void loadThreads(true);
		} catch (reason) {
			bootError = reason instanceof Error ? reason.message : tr('Gagal membuka obrolan.');
		} finally {
			loadingThread = false;
		}
	}

	// Deep link dari tombol "Hubungi": ?pub=<publication_id> → find-or-create.
	async function openByPublication(publicationID: string) {
		loadingThread = true;
		try {
			const res = await apiRequest(`${apiBase}/chat/threads`, {
				method: 'POST',
				body: JSON.stringify({ publication_id: publicationID })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal membuka obrolan.')));
			const d = await res.json();
			const id = (d.thread?.id as string) ?? '';
			await loadThreads(true);
			if (id) await openThread(id);
		} catch (reason) {
			bootError = reason instanceof Error ? reason.message : tr('Gagal membuka obrolan.');
		} finally {
			loadingThread = false;
		}
	}

	async function refreshActive(silent = true) {
		if (!active || sending) return;
		try {
			const res = await apiRequest(`${apiBase}/chat/threads/${active.id}`);
			if (!res.ok) return;
			const d = await res.json();
			const before = messages.length;
			active = d.thread as Thread;
			messages = (d.messages ?? []) as Message[];
			if (messages.length !== before) {
				// Auto-scroll hanya kalau pembaca sudah di dasar kolom.
				const nearBottom = messagesEl ? messagesEl.scrollHeight - messagesEl.scrollTop - messagesEl.clientHeight < 120 : true;
				if (nearBottom || messages[messages.length - 1]?.author_role === side) {
					await Promise.resolve();
					scrollToBottom();
				}
				if (!silent) void loadThreads(true);
			}
		} catch {
			/* polling diam-diam */
		}
	}

	const canSend = $derived(
		Boolean(active) && (active?.status === 'open' || (side === 'client' && active?.status === 'waiting')) && !sending
	);

	async function send() {
		const body = draft.trim();
		if (!body || !active || !canSend) return;
		sending = true;
		sendError = '';
		try {
			const res = await apiRequest(`${apiBase}/chat/threads/${active.id}/messages`, {
				method: 'POST',
				body: JSON.stringify({ body })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Pesan gagal dikirim.')));
			const d = await res.json();
			messages = [...messages, d.message as Message];
			if (d.thread) active = d.thread as Thread;
			draft = '';
			await Promise.resolve();
			scrollToBottom();
			void loadThreads(true);
		} catch (reason) {
			sendError = reason instanceof Error ? reason.message : tr('Pesan gagal dikirim.');
		} finally {
			sending = false;
		}
	}

	function onComposerKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
			e.preventDefault();
			void send();
		}
	}

	// ---------- lifecycle ----------

	let listTimer: ReturnType<typeof setInterval> | null = null;
	let msgTimer: ReturnType<typeof setInterval> | null = null;
	let tickTimer: ReturnType<typeof setInterval> | null = null;

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await loadThreads();
			if (initialPublication && side === 'client') {
				await openByPublication(initialPublication);
			} else if (initialThread) {
				await openThread(initialThread, { push: false });
			} else if (threads.length > 0) {
				await openThread(threads[0].id, { push: false });
			}
		})();

		listTimer = setInterval(() => {
			if (document.visibilityState === 'visible') void loadThreads(true);
		}, 15_000);
		msgTimer = setInterval(() => {
			if (document.visibilityState !== 'visible') return;
			if (active?.status === 'open') void refreshActive(true);
		}, 4_000);
		tickTimer = setInterval(() => (now = Date.now()), 30_000);

		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
		};
	});

	onDestroy(() => {
		if (listTimer) clearInterval(listTimer);
		if (msgTimer) clearInterval(msgTimer);
		if (tickTimer) clearInterval(tickTimer);
	});

	// ---------- turunan tampilan ----------

	type DisplayRow = { kind: 'day'; label: string } | { kind: 'msg'; m: Message };
	const rows = $derived.by<DisplayRow[]>(() => {
		const out: DisplayRow[] = [];
		let lastDay = '';
		for (const m of messages) {
			const day = new Date(m.created_at).toDateString();
			if (day !== lastDay) {
				out.push({ kind: 'day', label: fmtDay(m.created_at) });
				lastDay = day;
			}
			out.push({ kind: 'msg', m });
		}
		return out;
	});

	const statusChip = $derived.by(() => {
		if (!active) return { label: '', cls: '' };
		if (active.status === 'open') return { label: countdown(active), cls: 'bg-emerald-50 text-emerald-700 border-emerald-200' };
		if (active.status === 'waiting') return { label: countdown(active), cls: 'bg-slate-100 text-slate-600 border-slate-200' };
		return { label: countdown(active), cls: 'bg-rose-50 text-rose-700 border-rose-200' };
	});
</script>

<div class="grid gap-3 lg:grid-cols-[290px_minmax(0,1fr)] xl:grid-cols-[290px_minmax(0,1fr)_310px]">
	<!-- Kolom 1: daftar forum -->
	<section class="app-card flex max-h-[74dvh] min-h-[420px] flex-col overflow-hidden p-0 lg:max-h-[calc(100dvh-150px)]">
		<div class="flex items-center gap-2 border-b border-slate-100 px-4 py-3">
			<MessagesSquare size={15} style="color: var(--primary);" />
			<p class="text-[13px] font-bold">{tr('Percakapan')}</p>
			<span class="ml-auto rounded-full bg-slate-100 px-2 py-0.5 text-[10.5px] font-semibold text-slate-500">{threads.length}</span>
		</div>
		{#if bootError}
			<p class="px-4 py-3 text-xs" style="color: var(--destructive);">{bootError}</p>
		{/if}
		<div class="flex-1 overflow-y-auto">
			{#if loadingList}
				<p class="px-4 py-6 text-center text-xs text-muted-foreground">{tr('Memuat…')}</p>
			{:else if threads.length === 0}
				<div class="px-4 py-8 text-center">
					<p class="text-[13px] font-semibold text-slate-700">{tr('Belum ada obrolan')}</p>
					<p class="mt-1 text-xs leading-relaxed text-muted-foreground">
						{side === 'client'
							? tr('Buka halaman Ringkasan Distribusi lalu klik "Hubungi" pada media yang ingin ditanyakan.')
							: tr('Obrolan muncul di sini begitu ada klien yang menghubungi medianya.')}
					</p>
				</div>
			{:else}
				{#each threads as t (t.id)}
					{@const tone = avatarTone(t.media_name)}
					<button
						type="button"
						class="flex w-full items-start gap-2.5 border-b border-slate-50 px-3.5 py-3 text-left transition-colors hover:bg-slate-50 {active?.id === t.id ? 'bg-blue-50/60' : ''}"
						onclick={() => void openThread(t.id)}
					>
						<span class="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-full text-[13px] font-bold" style="background:{AVA_BG[tone]};color:{AVA_FG[tone]};">
							{#if cleanDomain(t.website_url, t.media_name) && !faviconFailed[t.id]}
								<img
									src={`https://www.google.com/s2/favicons?domain=${encodeURIComponent(cleanDomain(t.website_url, t.media_name))}&sz=128`}
									alt=""
									class="h-full w-full object-cover"
									loading="lazy"
									decoding="async"
									onerror={() => (faviconFailed = { ...faviconFailed, [t.id]: true })}
								/>
							{:else}
								{initialOf(t.media_name)}
							{/if}
						</span>
						<span class="min-w-0 flex-1">
							<span class="flex items-baseline gap-2">
								<span class="min-w-0 flex-1 truncate text-[12.5px] font-bold text-slate-900">{t.media_name}</span>
								<span class="shrink-0 text-[10px] text-slate-400">{fmtTime(t.last_message_at ?? t.expires_at)}</span>
							</span>
							<span class="mt-0.5 block truncate text-[11px] text-slate-500">
								{side === 'client' ? t.partner_name || tr('Mitra') : t.brand_name}
							</span>
							<span class="mt-1 flex items-center gap-1.5">
								<span class="h-1.5 w-1.5 shrink-0 rounded-full {t.status === 'open' ? 'bg-emerald-500' : t.status === 'waiting' ? 'bg-slate-300' : 'bg-rose-400'}"></span>
								<span class="min-w-0 flex-1 truncate text-[11.5px] text-slate-600">{previewOf(t)}</span>
								{#if (t.unread_count ?? 0) > 0}
									<span class="inline-flex h-4 min-w-4 shrink-0 items-center justify-center rounded-full bg-blue-600 px-1 text-[10px] font-bold text-white">{t.unread_count}</span>
								{/if}
							</span>
						</span>
					</button>
				{/each}
			{/if}
		</div>
	</section>

	<!-- Kolom 2: ruang pesan -->
	<section class="app-card flex max-h-[74dvh] min-h-[420px] flex-col overflow-hidden p-0 lg:max-h-[calc(100dvh-150px)]">
		{#if active}
			<div class="flex flex-wrap items-center gap-2 border-b border-slate-100 px-4 py-3">
				<div class="min-w-0 flex-1">
					<p class="truncate text-[13.5px] font-bold text-slate-900">{active.media_name}</p>
					<p class="truncate text-[11.5px] text-muted-foreground">
						{otherLabel}: {otherName(active)}{#if active.article_title} · {active.article_title}{/if}
					</p>
				</div>
				{#if active.pub_url}
					<a class="inline-flex items-center gap-1 rounded-full border border-blue-200 bg-blue-50/60 px-2.5 py-1 text-[11px] font-semibold text-blue-600 hover:bg-blue-100/70" href={active.pub_url} target="_blank" rel="noreferrer">
						<ExternalLink size={11} />{tr('Buka artikel')}
					</a>
				{/if}
				<span class="shrink-0 rounded-full border px-2.5 py-1 text-[11px] font-semibold {statusChip.cls}">
					{statusChip.label}
				</span>
			</div>

			<div bind:this={messagesEl} class="flex-1 space-y-1 overflow-y-auto bg-slate-50/60 px-4 py-4">
				<div class="mx-auto mb-3 max-w-md rounded-xl bg-white/80 px-3 py-2 text-center text-[11px] leading-relaxed text-slate-500 ring-1 ring-slate-100">
					{tr('Sesi obrolan berlaku 3×24 jam sejak pesan pertama dari klien. Setelah berakhir, riwayat tetap dapat dibaca.')}
				</div>
				{#if active.status === 'waiting'}
					<div class="mx-auto mb-3 max-w-md rounded-xl bg-blue-50 px-3 py-2 text-center text-[11px] leading-relaxed text-blue-700">
						{side === 'client'
							? tr('Sesi dimulai begitu kamu mengirim pesan pertama.')
							: tr('Menunggu pesan pertama dari klien.')}
					</div>
				{/if}
				{#each rows as row (row.kind === 'day' ? `d-${row.label}` : row.m.id)}
					{#if row.kind === 'day'}
						<p class="py-2 text-center text-[10.5px] font-semibold uppercase tracking-wide text-slate-400">{row.label}</p>
					{:else}
						{@const mine = row.m.author_role === side}
						<div class="flex {mine ? 'justify-end' : 'justify-start'}">
							<div class="max-w-[78%] rounded-2xl px-3.5 py-2.5 text-[12.5px] leading-relaxed shadow-2xs {mine ? 'rounded-br-md text-white' : 'rounded-bl-md border border-slate-200 bg-white text-slate-800'}" style={mine ? 'background-color: var(--primary);' : ''}>
								{#if !mine}
									<p class="mb-0.5 text-[10.5px] font-bold opacity-70">{row.m.author_role === 'partner' ? active.partner_name || tr('Mitra') : active.brand_name}</p>
								{/if}
								<p class="whitespace-pre-wrap break-words">{row.m.body}</p>
								<p class="mt-1 text-right text-[10px] {mine ? 'text-white/70' : 'text-slate-400'}">{fmtTime(row.m.created_at)}</p>
							</div>
						</div>
					{/if}
				{/each}
				{#if loadingThread}
					<p class="py-3 text-center text-xs text-muted-foreground">{tr('Memuat…')}</p>
				{/if}
			</div>

			<div class="border-t border-slate-100 bg-white px-3.5 py-3">
				{#if sendError}
					<p class="mb-2 text-xs" style="color: var(--destructive);">{sendError}</p>
				{/if}
				{#if canSend}
					<div class="flex items-end gap-2">
						<textarea
							bind:this={composerEl}
							bind:value={draft}
							rows="1"
							class="app-input max-h-32 min-h-10 flex-1 resize-none py-2.5 text-[13px]"
							placeholder={tr('Tulis pesan…')}
							onkeydown={onComposerKeydown}
						></textarea>
						<button
							type="button"
							class="inline-flex h-10 shrink-0 items-center gap-1.5 rounded-[5px] px-4 text-[12.5px] font-semibold text-white transition-colors disabled:opacity-50"
							style="background-color: var(--primary);"
							disabled={!draft.trim() || sending}
							onclick={() => void send()}
						>
							<SendHorizonal size={14} />{sending ? tr('Mengirim…') : tr('Kirim')}
						</button>
					</div>
					<p class="mt-1.5 text-[10.5px] text-slate-400">{tr('Enter untuk kirim, Shift+Enter baris baru.')} · {countdown(active)}</p>
				{:else if active.status === 'expired'}
					<div class="flex items-center gap-2.5 rounded-xl bg-slate-100 px-3.5 py-3 text-[12px] text-slate-600">
						<Clock3 size={15} class="shrink-0 text-slate-400" />
						{tr('Sesi 3×24 jam sudah berakhir — riwayat tetap dapat dibaca.')}
					</div>
				{:else if side === 'partner' && active.status === 'waiting'}
					<div class="flex items-center gap-2.5 rounded-xl bg-slate-100 px-3.5 py-3 text-[12px] text-slate-600">
						<Clock3 size={15} class="shrink-0 text-slate-400" />
						{tr('Menunggu pesan pertama dari klien sebelum bisa membalas.')}
					</div>
				{/if}
			</div>
		{:else}
			<div class="flex flex-1 flex-col items-center justify-center gap-2 px-6 text-center">
				<MessagesSquare size={26} class="text-slate-300" />
				<p class="text-[13px] font-semibold text-slate-600">{tr('Pilih percakapan di samping')}</p>
				<p class="max-w-xs text-xs leading-relaxed text-muted-foreground">{tr('Semua obrolan dengan mitra per artikel ada di daftar sebelah.')}</p>
			</div>
		{/if}
	</section>

	<!-- Kolom 3: info forum (desktop lebar) -->
	<aside class="hidden flex-col gap-3 xl:flex">
		{#if active}
			{@const tone = avatarTone(active.media_name)}
			<div class="app-card p-4">
				<p class="text-[11px] font-bold uppercase tracking-wide text-slate-400">{tr('Info media')}</p>
				<div class="mt-2.5 flex items-center gap-2.5">
					<span class="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full text-[14px] font-bold" style="background:{AVA_BG[tone]};color:{AVA_FG[tone]};">
						{#if cleanDomain(active.website_url, active.media_name) && !faviconFailed[active.id]}
							<img
								src={`https://www.google.com/s2/favicons?domain=${encodeURIComponent(cleanDomain(active.website_url, active.media_name))}&sz=128`}
								alt=""
								class="h-full w-full object-cover"
								loading="lazy"
								decoding="async"
								onerror={() => (faviconFailed = { ...faviconFailed, [active?.id ?? '']: true })}
							/>
						{:else}
							{initialOf(active.media_name)}
						{/if}
					</span>
					<div class="min-w-0">
						<p class="truncate text-[13px] font-bold text-slate-900">{active.media_name}</p>
						{#if active.website_url}
							<a class="truncate text-[11.5px] text-blue-600 hover:underline" href={active.website_url} target="_blank" rel="noreferrer">{active.website_url.replace(/^https?:\/\//, '')}</a>
						{/if}
					</div>
				</div>
				<div class="mt-3 space-y-1.5 text-[12px]">
					<div class="flex justify-between gap-2"><span class="text-slate-500">{tr('Tipe')}</span><span class="font-semibold capitalize">{active.media_type || '—'}</span></div>
					<div class="flex justify-between gap-2"><span class="text-slate-500">DA</span><span class="font-semibold">{active.da ?? '—'}</span></div>
					<div class="flex justify-between gap-2"><span class="text-slate-500">{otherLabel}</span><span class="max-w-[60%] truncate text-right font-semibold">{otherName(active)}</span></div>
				</div>
			</div>

			<div class="app-card p-4">
				<p class="text-[11px] font-bold uppercase tracking-wide text-slate-400">{tr('Artikel')}</p>
				<p class="mt-2 text-[12.5px] font-semibold leading-snug text-slate-900">{active.article_title || tr('Belum ada artikel')}</p>
				{#if active.pub_status}
					<p class="mt-1 text-[11.5px] text-slate-500">{tr('Status terbit')}: <span class="font-semibold capitalize">{active.pub_status}</span></p>
				{/if}
				{#if active.pub_url}
					<a class="mt-2 inline-flex items-center gap-1 text-[12px] font-semibold text-blue-600 hover:underline" href={active.pub_url} target="_blank" rel="noreferrer">
						<ExternalLink size={12} />{tr('Buka artikel di situs mitra')}
					</a>
				{/if}
			</div>

			<div class="app-card p-4">
				<p class="text-[11px] font-bold uppercase tracking-wide text-slate-400">{tr('Sesi obrolan')}</p>
				<div class="mt-2 space-y-1.5 text-[12px]">
					<div class="flex justify-between gap-2"><span class="text-slate-500">{tr('Mulai')}</span><span class="font-semibold">{fmtDateTime(active.started_at)}</span></div>
					<div class="flex justify-between gap-2"><span class="text-slate-500">{tr('Berakhir')}</span><span class="font-semibold">{fmtDateTime(active.expires_at)}</span></div>
					<div class="flex justify-between gap-2"><span class="text-slate-500">{tr('Status')}</span><span class="font-semibold">{statusChip.label}</span></div>
				</div>
				<div class="mt-3 flex items-start gap-2 rounded-xl bg-slate-50 px-3 py-2.5 text-[11.5px] leading-relaxed text-slate-500">
					<Info size={13} class="mt-0.5 shrink-0" />
					{tr('Batas 3×24 jam dihitung dari pesan pertama klien. Setelahnya riwayat tetap dapat dibaca.')}
				</div>
			</div>
		{/if}
	</aside>
</div>
