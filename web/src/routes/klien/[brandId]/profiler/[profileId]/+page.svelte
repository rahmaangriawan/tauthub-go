<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Taut Brief — workspace Profiler brand.
	//
	// Form enam tab diganti percakapan. Susunannya:
	//   [ nav Tauthub ] [ Sidebar Brief 280px ] [ Chat flex-1 ] [ Ringkasan ]
	//
	// Sidebar Brief menggantikan halaman daftar Profiler ("+ Brief baru",
	// pencarian, daftar per waktu). Tombol "Ringkasan" di dasar sidebar membuka
	// panel kanan: Entity Core penuh dan bisa dikoreksi, lalu Lapis 2 terlipat.
	// Koreksi disimpan lewat PUT Profiler yang sudah ada — bukan jalur baru.
	//
	// Streaming memakai fetch + pembaca SSE manual (pola yang sama dengan
	// halaman The Writer), bukan apiRequest yang timeout bawaannya 12 detik.
	import { onDestroy, onMount, tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import {
		ArrowLeft, ArrowRight, ArrowUp, Bot, ChartLine, Check, ChevronDown, Clipboard, Copy, ExternalLink, FileDown, FileText, ImagePlus, Images, LoaderCircle,
		Clock, FileCheck, FileSearch,
		Mic, MoreHorizontal, Newspaper, PanelRight, Pencil, Pin, Plus, RotateCcw, Save, Search, Send, ShieldAlert, ShieldCheck, Sparkle, Square, SquarePen, Store, Trash2, Upload, UserCircle, X,
		History
	} from '@lucide/svelte';
	import PageTitle from '$lib/components/page-title.svelte';
	import Select from '$lib/components/select.svelte';
	import BriefFieldInput from '$lib/components/brief-field-input.svelte';
	import { apiRequest, readApiError, ensureValidSession, getAccessToken, showToast, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { renderMarkdown } from '$lib/markdown-lite';
	import { sanitizeHtml } from '$lib/sanitize-html';
	import BriefSummaryView from './_brief-summary-view.svelte';

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/profiler`);
	const brandApi = $derived(`/v1/brands/${brandId}`);

	type BriefListItem = {
		id: string; name: string; description: string | null; short_code: string | null; pinned: boolean; updated_at: string | null;
		// Progres field wajib per brief dari GET /profiles; null bila API lama.
		progress: { filled: number; total: number; pct: number } | null
	};
	type FieldMeta = {
		key: string; layer: number; section: string; label: string; widget: string;
		required: boolean; sort_order: number; example_question: string; enabled: boolean;
	};
	type FieldStateView = { status: string; source: string; note: string };
	type SourceView = { kind: string; label: string };
	// Aksi yang diusulkan AI untuk sebuah balasan. Backend sudah menyaringnya
	// ke allowlist, jadi frontend hanya perlu tahu bentuknya.
	type ReplyAction = {
		kind: 'timeline_post';
		label: string;
		caption?: string;
		hashtags?: string[];
		occurred_on?: string;
		reason?: string;
	};
	type ChatRow = {
		id?: string;
		role: 'user' | 'assistant';
		content: string;
		created_at?: string;
		actions?: ReplyAction[];
		actions_used?: string[];
	};

	// Sidebar brief = riwayat brief milik brand ini. Nantinya diisi onMount.
	let briefs = $state<BriefListItem[]>([]);
	let briefSearch = $state('');
	let sidebarOpen = $state(false);
	let searchOpen = $state(false);
	let menuId = $state<string | null>(null);
	let sidebarWidth = $state(270);
	let draggingRail = $state(false);
	let railEl = $state<HTMLElement | null>(null);

	const SIDEBAR_WIDTH_KEY = 'tauthub_brief_sidebar';

	// Lebar sidebar bisa ditarik; min 220 supaya tombol aksi tak terjepit,
	// maks 480 supaya area chat tak habis.
	function loadSidebarWidth(): number {
		try {
			const raw = Number(localStorage.getItem(SIDEBAR_WIDTH_KEY));
			if (Number.isFinite(raw) && raw >= 220 && raw <= 480) return Math.round(raw);
		} catch { /* penyimpanan penuh/privat, pakai bawaan */ }
		return 270;
	}

	function startRailDrag(e: PointerEvent) {
		if (!railEl) return;
		e.preventDefault();
		draggingRail = true;
		const left = railEl.getBoundingClientRect().left;
		document.documentElement.style.userSelect = 'none';
		document.documentElement.style.cursor = 'col-resize';
		const onMove = (ev: PointerEvent) => {
			sidebarWidth = Math.min(480, Math.max(220, Math.round(ev.clientX - left)));
		};
		const onUp = () => {
			draggingRail = false;
			document.documentElement.style.userSelect = '';
			document.documentElement.style.cursor = '';
			window.removeEventListener('pointermove', onMove);
			window.removeEventListener('pointerup', onUp);
			try {
				localStorage.setItem(SIDEBAR_WIDTH_KEY, String(sidebarWidth));
			} catch { /* abaikan */ }
		};
		window.addEventListener('pointermove', onMove);
		window.addEventListener('pointerup', onUp);
	}
	let renameId = $state<string | null>(null);
	let renameValue = $state('');
	// Aksi di panel brief melayang (terpisah dari sidebar supaya dua menu tidak
	// terbuka bersamaan).
	let popMenuId = $state<string | null>(null);
	// Menu kebab memakai position:fixed di ujung tombolnya — kalau tetap
	// absolute di dalam baris, menu ini terpotong overflow daftar brief saat
	// barisnya berada di tepi atas/bawah panel (kotaknya tak terlihat).
	let popMenuPos = $state<{ left: number; top: number; placeUp: boolean } | null>(null);
	let popRenameId = $state<string | null>(null);
	let popRenameValue = $state('');

	// URL memakai kode pendek (maks 5 karakter, migrasi 213). Param bisa berupa
	// kode ATAU id 36 karakter lama, jadi diterjemahkan dulu ke id internal —
	// seluruh panggilan API memakai id, hanya URL yang ditampilkan ringkas.
	const routeParam = $derived(page.params.profileId as string);
	const profileId = $derived.by(() => {
		if (!routeParam) return '';
		const exact = briefs.find((b) => b.short_code === routeParam || b.id === routeParam);
		if (exact) return exact.id;
		return routeParam.length === 36 ? routeParam : '';
	});
	const activeShortCode = $derived(briefs.find((b) => b.id === profileId)?.short_code ?? routeParam);
	const briefBase = $derived(profileId ? `${apiBase}/profiles/${profileId}/brief` : '');

	// Gerbang 2 (Instan Brief): masuk lewat /profiler?gate=instan (halaman
	// perutean). Murni dari URL — deep-link/refresh tanpa param kembali ke
	// mode chat, dan popup gerbang tidak muncul di dalam workspace ini.
	let gateMode = $state<'chat' | 'instan'>(page.url.searchParams.get('gate') === 'instan' ? 'instan' : 'chat');

	let loading = $state(true);
	let error = $state('');

	// Tinggi kerja halaman: diukur dari posisi kontainer supaya halaman tidak
	// pernah melebihi pandangan (compact, tanpa scroll badan) — apapun tinggi
	// bar utilitas di atasnya.
	let shellEl = $state<HTMLElement | null>(null);
	let shellTop = $state(64);

	// Chat
	let profileName = $state('');
	// Nama brand untuk judul tab. Nama brief internal ("Brief 7") tidak dipakai
	// di judul karena itu nomor urut pembuatan, bukan identitas halaman.
	let brandName = $state('');
	let messages = $state<ChatRow[]>([]);
	let composer = $state('');
	let streaming = $state(false);
	let streamingText = $state('');
	let streamError = $state('');
	// Status singkat dari backend selagi AI menyiapkan giliran (mis.
	// "Membaca tautan example.com…") — tampil di indikator berpikir.
	let streamStatus = $state('');
	let phase = $state('intake');
	let progress = $state({ filled: 0, total: 0 });
	let fields = $state<FieldMeta[]>([]);
	let fieldStates = $state<Record<string, FieldStateView>>({});
	let values = $state<Record<string, any>>({});
	let sources = $state<SourceView[]>([]);
	let configured = $state(true);
	let askedField = $state<string | null>(null);
	let generationCount = $state(0);

	// Widget
	let widgetValue = $state('');

	// Layanan pendukung percakapan
	let uploading = $state(false);

	// Lencana "Beta" di pojok kanan atas judul percakapan — keterangannya
	// dibuka saat diklik, ditutup lewat backdrop atau Escape.
	let betaOpen = $state(false);

	// Tab workspace: Copilot, Entitas, Timeline, Media, dan Ringkasan AI.
	type BriefTab = 'copilot' | 'entitas' | 'timeline' | 'media' | 'ringkasan';
	const BRIEF_TABS: readonly BriefTab[] = ['copilot', 'entitas', 'timeline', 'media', 'ringkasan'];
	let activeTab = $state<BriefTab>('copilot');
	let showLayer2 = $state(false);

	// Header band bisa diciutkan jadi strip judul tipis; default termuat jejak.
	let headerCollapsed = $state(true);

	// Kartu brief bisa diciutkan jadi ikon Tauthub; klik ikon membuka lagi.
	let cardOpen = $state(true);

	// Popover lampiran berkas di composer.
	let attachOpen = $state(false);

	// Lampiran dokumen: dipasang dulu sebagai chip, baru diunggah saat Kirim.
	type PendingDoc = { file: File };
	let pendingDocs = $state<PendingDoc[]>([]);
	let uploadingNow = $state(false);
	let draggingFiles = $state(false);
	let dragDepth = 0;

	// Panel Ringkasan
	let ringkasan = $state<Record<string, string>>({});
	let saving = $state(false);

	// Panel Media: dokumen & gambar milik brief ini. Sumbernya sama dengan yang
	// dipakai Copilot, jadi berkas yang diunggah lewat obrolan ikut tampil.
	type MediaDoc = {
		id: string;
		original_filename: string;
		mime_type: string | null;
		size_bytes: number | null;
		parse_status: string;
		parse_error: string | null;
		file_deleted_at: string | null;
		created_at: string | null;
		url: string;
	};
	type MediaImage = {
		id: string;
		url: string;
		file_name: string | null;
		mime_type: string | null;
		size_bytes: number | null;
		note: string | null;
		created_at: string | null;
	};
	let mediaDocs = $state<MediaDoc[]>([]);
	let mediaImages = $state<MediaImage[]>([]);
	let mediaLoading = $state(false);
	let mediaError = $state('');
	let mediaUploading = $state(false);
	let mediaDragging = $state(false);
	let mediaDragDepth = 0;

	// ---------- Timeline (feed kabar terbaru brand) ----------
	// Post kedaluwarsa otomatis 3 bulan; gambar memakai jalur unggah Media.
	type TimelineLink = { url: string; title: string; description: string; image: string; site: string };
	type TimelinePost = {
		id: string;
		body: string;
		hashtags: string[];
		image_url: string | null;
		image_id: string | null;
		occurred_on: string | null;
		created_at: string | null;
		author_name: string;
		link_url: string | null;
		link_title: string | null;
		link_description: string | null;
		link_image: string | null;
		link_site: string | null;
	};
	let timeline = $state<TimelinePost[]>([]);
	let timelineLoading = $state(false);
	let timelineError = $state('');
	let timelinePostError = $state('');
	let timelinePosting = $state(false);
	let timelineBody = $state('');
	let timelineBrandName = $state('');
	let timelineBrandSlug = $state('');
	// Composer rich dibuka sebagai modal ("Utas baru"), seperti feed sosial.
	let timelineComposerOpen = $state(false);
	let timelineImageFile = $state<File | null>(null);
	let timelineImagePreview = $state('');
	// Unggahan gambar berjalan di belakang layar sejak dipasang; Kirim hanya
	// melanjutkan/menunggu hasilnya. Nomor urut membatalkan hasil upload lama
	// saat gambar diganti atau dilepas.
	let timelineImageId = $state('');
	let timelineImageUrl = $state('');
	let timelineUploading = $state(false);
	let timelineUploadFailed = $state(false);
	let timelineUploadSeq = 0;
	let timelineUploadPromise: Promise<string> | null = null;
	let timelineLink = $state<TimelineLink | null>(null);
	let timelineLinkLoading = $state(false);
	let timelineDragging = $state(false);
	// Draf disimpan di browser per brief: bertahan saat refresh, tanpa antrean
	// server. Gambar yang sudah terunggah ikut disimpan sebagai URL.
	type TimelineDraft = {
		id: string;
		body: string;
		link: TimelineLink | null;
		imageId: string;
		imageUrl: string;
		savedAt: number;
	};
	let timelineDrafts = $state<TimelineDraft[]>([]);
	let timelineDraftsOpen = $state(false);
	let timelineDiscardAsk = $state(false);
	// Menu titik tiga per post: hanya satu yang terbuka pada satu waktu.
	let timelineMenuFor = $state<string | null>(null);
	const TIMELINE_BODY_MAX = 2000;
	// Nomor urut permintaan: jawaban lama tidak boleh menimpa brief yang baru.
	let mediaLoadSeq = 0;

	// Panel Ringkasan (AI): dua ringkasan yang dibaca Writer mode proper.
	type SummaryKind = 'entity' | 'file';
	type SummaryItem = {
		kind: SummaryKind;
		status: 'idle' | 'queued' | 'running' | 'ready' | 'failed';
		content: string;
		source_chars: number;
		token_input: number;
		token_output: number;
		model: string;
		error: string;
		generated_at: string | null;
		updated_at: string | null;
		stale: boolean;
	};
	let summaryTab = $state<SummaryKind>('entity');
	let summaries = $state<SummaryItem[]>([]);
	let summaryLoading = $state(false);
	let summaryError = $state('');
	let summaryBusy = $state<SummaryKind | null>(null);
	let summaryTimer: ReturnType<typeof setTimeout> | null = null;
	let summarySeq = 0;
	const summaryCurrent = $derived(summaryOf(summaryTab));

	// ---------- popup "entitas 100% → ringkas" ----------
	// Fase: 'idle' (tak tampil) → 'invite' (ajakan) → 'working' (animasi) →
	// 'done' (berhasil) atau 'error'. 'edit' menandai user meneruskan obrolan
	// dengan niat mengedit, supaya pesan berikutnya diarahkan ke tanya-jawab
	// entitas alih-alih disangka sinyal siap rilis.
	type SummaryPromptPhase = 'idle' | 'invite' | 'working' | 'done' | 'error';
	let summaryPrompt = $state<SummaryPromptPhase>('idle');
	let summaryPromptError = $state('');
	// Penanda "sudah pernah ditawari utk progres 100% ini" agar popup tidak
	// muncul berulang tiap render.
	let summaryInvitedFor = $state('');
	let editModeIntent = $state(false);

	const entitySummaryReady = $derived(summaryOf('entity').status === 'ready');
	const entitySummaryBusy = $derived(
		summaryOf('entity').status === 'queued' || summaryOf('entity').status === 'running'
	);

	// Undangan yang sudah ditutup disimpan di localStorage per brief, supaya
	// refresh halaman TIDAK memunculkannya lagi ("jangan tampil lagi dulu").
	// Kunci = progres + tahap, jadi bila datanya berubah (progres turun lalu
	// naik lagi ke 100%) undangan boleh muncul kembali.
	function inviteSignature(): string {
		return `${progress.filled}/${progress.total}:${entitySummaryReady ? 'edit' : 'pre'}`;
	}
	function inviteStorageKey(sig: string): string {
		return `brief.summaryInvite.${profileId}.${sig}`;
	}
	function inviteDismissed(sig: string): boolean {
		try {
			return localStorage.getItem(inviteStorageKey(sig)) === '1';
		} catch {
			return false;
		}
	}
	function dismissInvite() {
		try {
			localStorage.setItem(inviteStorageKey(inviteSignature()), '1');
		} catch {
			// Mode privat / storage penuh: cukup tutup untuk sesi ini.
		}
		summaryPrompt = 'idle';
	}

	// Kunci undangan per kombinasi progres+ringkasan: begitu ringkasan jadi,
	// kuncinya berubah sehingga undangan berikutnya (setelah data berubah lagi)
	// tetap bisa muncul.
	$effect(() => {
		if (progressPct < 100) return;
		// WAJIB memakai inviteSignature() yang sama dengan dismissInvite():
		// sebelumnya di sini dipakai boolean mentah (`:false`), sedangkan yang
		// disimpan `:pre`, sehingga penanda penolakan tidak pernah cocok dan
		// popup muncul lagi setiap refresh.
		const key = inviteSignature();
		if (summaryInvitedFor === key) return;
		summaryInvitedFor = key;
		if (entitySummaryReady || summaryPrompt !== 'idle') return;
		if (inviteDismissed(key)) return;
		summaryPrompt = 'invite';
	});
	const summaryBadge = $derived(summaryStatus(summaryCurrent));
	const summaryActionLabel = $derived(
		summaryCurrent.status === 'idle'
			? tr('Buat ringkasan')
			: summaryCurrent.status === 'failed'
				? tr('Coba lagi')
				: tr('Perbarui sekarang')
	);

	let controller: AbortController | null = null;
	let threadEl: HTMLElement | null = $state<HTMLElement | null>(null);

	// Tombol salin di bawah balasan; indeks pesan yang baru saja disalin diberi
	// centang singkat sebagai umpan balik.
	let copiedIndex = $state<number | null>(null);
	let copiedTimer: number | null = null;

	// Dialog konfirmasi milik aplikasi (bukan confirm() bawaan browser), dipakai
	// bersama oleh hapus brief dan mulai percakapan baru.
	type ConfirmRequest = {
		title: string;
		body: string;
		confirmLabel: string;
		danger: boolean;
		action: () => void | Promise<void>;
	};
	let confirmReq = $state<ConfirmRequest | null>(null);
	let confirmEl = $state<HTMLElement | null>(null);

	$effect(() => {
		if (confirmReq && confirmEl) confirmEl.focus();
	});

	function openConfirm(req: ConfirmRequest) {
		menuId = null;
		confirmReq = req;
	}

	async function copyReply(index: number, text: string) {
		try {
			await navigator.clipboard.writeText(text);
			copiedIndex = index;
			showToast(tr('Balasan disalin.'));
			if (copiedTimer) window.clearTimeout(copiedTimer);
			copiedTimer = window.setTimeout(() => (copiedIndex = null), 1600);
		} catch {
			showToast(tr('Gagal menyalin. Salin manual ya.'), 'error');
		}
	}

	// ---------- Aksi usulan AI pada balasan ----------
	// Aksi disimpan di meta pesan oleh backend, jadi tombolnya bertahan setelah
	// reload. Setelah dijalankan, kind-nya ditandai lewat endpoint consume agar
	// tidak bisa diklik dua kali dan membuat post duplikat.
	let actionBusy = $state<string | null>(null);

	function actionUsed(m: ChatRow, kind: string): boolean {
		return (m.actions_used ?? []).includes(kind);
	}

	async function runTimelineAction(m: ChatRow, action: ReplyAction) {
		if (actionBusy) return;
		if (!m.id) {
			// Sebelumnya kondisi ini menyebabkan tombol tampak mati tanpa
			// penjelasan. Sekarang user diberi tahu apa yang harus dilakukan.
			showToast(tr('Muat ulang halaman dulu, lalu coba lagi.'), 'error');
			return;
		}
		const caption = (action.caption ?? '').trim();
		if (!caption) {
			showToast(tr('Usulan ini tidak punya isi yang bisa disimpan.'), 'error');
			return;
		}
		actionBusy = m.id;
		try {
			const res = await apiRequest(`${apiBase}/profiles/${profileId}/timeline`, {
				method: 'POST',
				body: JSON.stringify({
					body: caption,
					hashtags: action.hashtags ?? [],
					occurred_on: action.occurred_on || ''
				})
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menambahkan ke Timeline.')));
			// Tandai dipakai. Kalau penandaan gagal, post tetap sudah dibuat;
			// yang hilang hanya status tombol, jadi tidak perlu menggagalkan.
			await apiRequest(`${briefBase}/actions/consume`, {
				method: 'POST',
				body: JSON.stringify({ message_id: m.id, kind: action.kind })
			}).catch(() => null);
			messages = messages.map((row) =>
				row.id === m.id
					? { ...row, actions_used: [...(row.actions_used ?? []), action.kind] }
					: row
			);
			showToast(tr('Ditambahkan ke Timeline.'));
			void loadTimeline();
		} catch (e) {
			showToast(e instanceof Error ? e.message : tr('Gagal menambahkan ke Timeline.'), 'error');
		} finally {
			actionBusy = null;
		}
	}

		let userName = $state('');
		const userInitials = $derived.by(() => {
			const parts = userName.trim().split(/\s+/);
			if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase();
			return userName.slice(0, 2).toUpperCase() || 'ST';
		});

		// Waktu hidrasi/giliran AI terakhir — dipakai strip status "Sinkronisasi
		// Terakhir" di baris tab (data nyata, bukan teks tetap).
		let lastSyncAt = $state<number | null>(null);
		function syncText(): string {
			if (!lastSyncAt) return tr('Menunggu data');
			const secs = Math.max(0, Math.round((Date.now() - lastSyncAt) / 1000));
			if (secs < 60) return tr('Baru saja');
			if (secs < 3600) return `${Math.round(secs / 60)} ${tr('menit lalu')}`;
			return `${Math.round(secs / 3600)} ${tr('jam lalu')}`;
		}

		/**
	 * Marquee saat hover untuk teks yang terpotong: bila teks memang lebih
	 * lebar dari wadahnya, kursor masuk memicu geser bolak-balik supaya nama
	 * panjang bisa terbaca utuh tanpa memperlebar panel. Nama pendek tidak
	 * ikut bergerak. Interaksi dipasang pakai action, bukan handler inline,
	 * supaya elemen statis tidak butuh role a11y.
	 */
	function marqueeOnHover(node: HTMLElement) {
		const text = node.firstElementChild as HTMLElement | null;
		if (!text) return {};
		const start = () => {
			const overflow = text.scrollWidth - node.clientWidth;
			if (overflow <= 2) return;
			text.style.setProperty('--marquee-shift', `-${overflow + 6}px`);
			text.style.setProperty('--marquee-duration', `${Math.min(14, Math.max(2.5, overflow / 28))}s`);
			text.classList.add('brief-marquee-run');
		};
		const stop = () => text.classList.remove('brief-marquee-run');
		node.addEventListener('mouseenter', start);
		node.addEventListener('mouseleave', stop);
		node.addEventListener('focusin', start);
		node.addEventListener('focusout', stop);
		return {
			destroy() {
				node.removeEventListener('mouseenter', start);
				node.removeEventListener('mouseleave', stop);
				node.removeEventListener('focusin', start);
				node.removeEventListener('focusout', stop);
			}
		};
	}

	/** Pecah jawaban longtext jadi butir untuk kartu guardrails/target AI. */
	function items(key: string): string[] {
		const raw = String(ringkasan[key] ?? '').trim();
		if (!raw) return [];
		return raw
			.split(/\r?\n/)
			.map((line) => line.replace(/^\s*(?:[-•*]|\d+[.)])\s*/, '').trim())
			.filter(Boolean);
	}

	/** Jam lokal browser dari created_at API (UTC ISO). Kosong bila tak ada. */
	function timeLabel(iso?: string): string {
		if (!iso) return '';
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return '';
		return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
	}

	onMount(() => {
			void (async () => {
				const fresh = await ensureValidSession();
				if (!fresh) return;
				userName = fresh.user?.display_name || fresh.user?.email || '';
				// Briefs dulu: URL memakai kode pendek, jadi id internal baru ketahuan
				// setelah daftar dimuat.
			await loadBriefs();
			sidebarWidth = loadSidebarWidth();
			void loadBrandName();
			if (profileId) await hydrate();
			loading = false;
			measureWorkspace();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	function measureWorkspace() {
		if (!shellEl) return;
		const top = shellEl.getBoundingClientRect().top;
		if (top > 0) shellTop = Math.round(top);
	}

	$effect(() => {
		// Berganti brief lewat sidebar tanpa me-reload halaman → hidrasi ulang.
		if (profileId && !loading) void hydrate();
	});

	// Tab terakhir per brief disimpan di browser: reload tetap di tab yang sama,
	// dan tiap brief punya tab-nya sendiri saat berpindah lewat sidebar.
	let tabRestoredFor = '';
	$effect(() => {
		if (!profileId || tabRestoredFor === profileId) return;
		tabRestoredFor = profileId;
		try {
			const saved = localStorage.getItem(`brief.tab.${profileId}`);
			if (saved && (BRIEF_TABS as readonly string[]).includes(saved)) activeTab = saved as BriefTab;
		} catch {
			// localStorage diblokir → tetap di tab default.
		}
	});

	$effect(() => {
		if (!profileId || tabRestoredFor !== profileId) return;
		try {
			localStorage.setItem(`brief.tab.${profileId}`, activeTab);
		} catch {
			// Penyimpanan penuh/diblokir bukan alasan mengganggu UI.
		}
	});

	$effect(() => {
		// Tab Media memuat daftar berkas saat dibuka dan saat brief berganti.
		if (activeTab === 'media' && profileId) void loadMedia();
		// Pane Instan Brief (gerbang 2) juga menampilkan daftar berkas.
		if (gateMode === 'instan' && activeTab === 'copilot' && profileId) void loadMedia();
		if (activeTab === 'timeline' && profileId) void loadTimeline();
		// Menu titik tiga tidak boleh ikut terbawa ke brief/tab lain.
		timelineMenuFor = null;
		// Draf terikat ke brief: muat ulang saat brief berganti.
		if (profileId) loadTimelineDrafts();
	});

	$effect(() => {
		// Tab Ringkasan memuat hasil agent saat dibuka / brief berganti.
		if (activeTab === 'ringkasan' && profileId) void loadSummaries();
	});

	onDestroy(() => {
		if (summaryTimer) clearTimeout(summaryTimer);
		try { recognition?.stop(); } catch { /* sudah berhenti */ }
	});

	// Gulir ke pesan terbaru setiap daftar/teks bertambah. Dibaca eksplisit di
	// dalam efek supaya efek berjalan ulang saat pesan baru masuk atau AI menulis.
	$effect(() => {
		void messages.length;
		void streamingText;
		void loading;
		const el = threadEl;
		if (el) el.scrollTop = el.scrollHeight;
	});

	$effect(() => {
		// Baca state di sini supaya efek berjalan ulang saat data masuk maupun
		// saat berpindah brief — posisi kontainer bisa ikut bergeser.
		const layoutSignal = `${loading}|${profileId}`;
		void layoutSignal;
		measureWorkspace();
		window.addEventListener('resize', measureWorkspace);
		return () => window.removeEventListener('resize', measureWorkspace);
	});

	// ---------- data ----------

	/** Nama brand untuk judul tab. Gagal di sini tidak boleh menghalangi brief. */
	async function loadBrandName() {
		try {
			const res = await apiRequest(brandApi);
			if (!res.ok) return;
			const data = await res.json();
			brandName = String(data?.name ?? '');
		} catch {
			// Judul jatuh ke teks bawaan; bukan alasan menampilkan error.
		}
	}

	async function loadBriefs() {
		const res = await apiRequest(`${apiBase}/profiles`);
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal memuat daftar brief.'));
			return;
		}
		const data = await res.json();
		briefs = (data?.profiles ?? []).map((p: any) => ({
			id: p.id,
			name: p.name,
			description: p.description ?? null,
			short_code: p.short_code ?? null,
			pinned: Boolean(p.pinned),
			updated_at: p.updated_at ?? null,
			progress: p.progress ?? null
		}));
	}

	async function hydrate() {
		const res = await apiRequest(`${briefBase}/chat`);
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal memuat percakapan.'));
			return;
		}
		const data = await res.json();
		profileName = data?.profile?.name ?? '';
		messages = data?.messages ?? [];
		phase = data?.phase ?? 'intake';
		progress = data?.progress ?? { filled: 0, total: 0 };
		fields = (data?.fields ?? []).filter((f: FieldMeta) => f.enabled && f.layer !== 3);
		fieldStates = data?.field_states ?? {};
		values = data?.values ?? {};
		sources = data?.sources ?? [];
		configured = Boolean(data?.configured);
		streamError = '';
		askedField = null;
		ringkasan = ringkasanCopy();
		fillInstanForm();
		lastSyncAt = Date.now();
	}

	function ringkasanCopy(): Record<string, string> {
		const copy: Record<string, string> = {};
		for (const f of fields) {
			const v = values[f.key];
			copy[f.key] = Array.isArray(v) ? v.join('; ') : typeof v === 'string' ? v : v == null ? '' : String(v);
		}
		return copy;
	}

	async function createBrief() {
		working(true);
		try {
			const res = await apiRequest(`${apiBase}/profiles`, {
				method: 'POST',
				body: JSON.stringify({ name: `${tr('Brief')} ${(briefs?.length ?? 0) + 1}` })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal membuat brief baru.')));
			const data = await res.json();
			await loadBriefs();
			// Navigasi ke brief baru memakai route yang sama (hanya param berubah),
			// jadi komponen TIDAK di-mount ulang. Tanpa reset di sini, tombol
			// "Brief baru" tetap disabled setelah brief pertama dibuat — terlihat
			// seolah jumlah brief dibatasi.
			working(false);
			await goto(`/klien/${brandUrl}/profiler/${data.short_code || data.id}`);
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal membuat brief baru.');
			working(false);
		}
	}

	// ---------- CRUD brief (menu aksi di panel) ----------

	function toggleMenu(id: string) {
		renameId = null;
		menuId = menuId === id ? null : id;
	}

	function startRename(id: string, current: string) {
		menuId = null;
		renameId = id;
		renameValue = current;
	}

	async function saveRename(id: string) {
		const name = renameValue.trim();
		if (!name) {
			renameId = null;
			return;
		}
		try {
			const res = await apiRequest(`${apiBase}/profiles/${id}`, {
				method: 'PUT',
				body: JSON.stringify({ name })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan nama.')));
			await loadBriefs();
			if (id === profileId) profileName = name;
			renameId = null;
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal menyimpan nama.');
		}
	}

	// ---------- Aksi di panel brief melayang ----------

	function togglePopMenu(id: string, ev?: MouseEvent) {
		popRenameId = null;
		popMenuPos = null;
		if (popMenuId === id) {
			popMenuId = null;
			return;
		}
		popMenuId = id;
		if (ev?.currentTarget instanceof HTMLElement) {
			const rect = ev.currentTarget.getBoundingClientRect();
			const width = 150;
			const height = 70; // dua tombol menu + padding
			const gap = 6;
			const left = Math.max(8, Math.min(rect.right - width, window.innerWidth - width - 8));
			// Default membuka ke bawah; bila di bawah tidak cukup ruang dan di
			// atasnya masih ada, bar menu terbuka ke ATAS supaya kotaknya tampil.
			const placeUp = window.innerHeight - rect.bottom < height + gap && rect.top >= height + gap;
			popMenuPos = { left, top: placeUp ? rect.top - gap - height : rect.bottom + gap, placeUp };
		}
	}

	function startPopRename(id: string, current: string) {
		popMenuId = null;
		popMenuPos = null;
		popRenameId = id;
		popRenameValue = current;
	}

	async function savePopRename(id: string) {
		const name = popRenameValue.trim();
		if (!name) {
			popRenameId = null;
			return;
		}
		try {
			const res = await apiRequest(`${apiBase}/profiles/${id}`, {
				method: 'PUT',
				body: JSON.stringify({ name })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan nama.')));
			await loadBriefs();
			if (id === profileId) profileName = name;
			popRenameId = null;
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal menyimpan nama.');
		}
	}

	// Sematkan/lepas: ditulis di server, jadi seluruh anggota brand melihat pin
	// yang sama dan urutannya otomatis menaruh yang disematkan di atas.
	async function togglePin(id: string, pinned: boolean) {
		menuId = null;
		popMenuId = null;
		popMenuPos = null;
		try {
			const res = await apiRequest(`${apiBase}/profiles/${id}`, {
				method: 'PUT',
				body: JSON.stringify({ pinned: !pinned })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyematkan brief.')));
			await loadBriefs();
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal menyematkan brief.');
		}
	}

	async function duplicateBrief(id: string) {
		working(true);
		menuId = null;
		try {
			const res = await apiRequest(`${apiBase}/profiles/${id}/duplicate`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menduplikasi brief.')));
			const data = await res.json();
			await loadBriefs();
			await goto(`/klien/${brandUrl}/profiler/${data.short_code || data.id}`);
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal menduplikasi brief.');
			working(false);
		}
	}

	// Simpan isi sebuah brief sebagai template: nama + deskripsi diminta lewat
	// modal kecil, isinya disalin oleh server. Template lalu muncul sebagai opsi
	// ketiga di gerbang Taut Brief.
	let templateSaveFor = $state<BriefListItem | null>(null);
	let templateSaveName = $state('');
	let templateSaveDesc = $state('');
	let templateSaveBusy = $state(false);
	let templateSaveError = $state('');

	function startSaveTemplate(item: BriefListItem) {
		popMenuId = null;
		popMenuPos = null;
		templateSaveFor = item;
		templateSaveName = item.name;
		templateSaveDesc = item.description ?? '';
		templateSaveError = '';
	}

	async function saveAsTemplate(e: SubmitEvent) {
		e.preventDefault();
		const item = templateSaveFor;
		if (!item || templateSaveBusy) return;
		const name = templateSaveName.trim();
		if (name.length < 2) {
			templateSaveError = tr('Nama template minimal 2 karakter.');
			return;
		}
		templateSaveBusy = true;
		templateSaveError = '';
		try {
			const res = await apiRequest(`${apiBase}/profiles/${item.id}/save-as-template`, {
				method: 'POST',
				body: JSON.stringify({ name, description: templateSaveDesc.trim() })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan template.')));
			templateSaveFor = null;
			showToast(tr('Template tersimpan. Template ini muncul saat membuka brief baru.'));
		} catch (reason) {
			templateSaveError = reason instanceof Error ? reason.message : tr('Gagal menyimpan template.');
		} finally {
			templateSaveBusy = false;
		}
	}

	function requestDelete(item: BriefListItem) {
		openConfirm({
			title: tr('Hapus brief ini?'),
			body: `${tr('Brief')} "${item.name}" ${tr('akan dihapus dari daftar. Kampanye yang sudah memakainya tidak terpengaruh, karena datanya sudah tersimpan di kampanye tersebut.')}`,
			confirmLabel: tr('Hapus'),
			danger: true,
			action: () => deleteBrief(item.id)
		});
	}

	async function deleteBrief(id: string) {
		working(true);
		menuId = null;
		popMenuId = null;
		popMenuPos = null;
		try {
			const res = await apiRequest(`${apiBase}/profiles/${id}`, { method: 'DELETE' });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menghapus brief.')));
			await loadBriefs();
			// Brief aktif terhapus → pindah ke brief terbaru yang tersisa; kalau
			// kosong, /profiler otomatis membuat yang baru.
			if (id === profileId) {
				const next = briefs[0];
				await goto(next
					? `/klien/${brandUrl}/profiler/${next.short_code || next.id}`
					: `/klien/${brandUrl}/profiler`);
			} else {
				working(false);
			}
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal menghapus brief.');
			working(false);
		}
	}

	// Riwayat brief diperlakukan seperti riwayat chat: satu daftar polos,
	// terbaru dulu, berapapun lamanya — tidak dikelompokkan waktu, tidak dibuang.
	function visibleBriefs() {
		const q = briefSearch.trim().toLowerCase();
		if (!q) return briefs;
		return briefs.filter((b) => `${b.name} ${b.description ?? ''}`.toLowerCase().includes(q));
	}

	// ---------- percakapan ----------

	async function send(preset?: string) {
		const text = (preset ?? composer).trim();
		if ((!text && pendingDocs.length === 0) || streaming || uploadingNow) return;

		// Entitas sudah 100% & sudah diringkas, lalu user mau mengetik lagi:
		// tawarkan dulu "lanjut edit?" — kecuali pesan ini memang berasal dari
		// alur edit (continueToEdit) agar tidak memicu popup berulang.
		if (!editModeIntent && summaryPrompt === 'idle' && progressPct >= 100 && entitySummaryReady) {
			// Sekali ditutup ("Nanti"), jangan menahan pengiriman lagi untuk
			// progres yang sama — termasuk setelah refresh.
			const sig = `${progress.filled}/${progress.total}:edit`;
			if (!inviteDismissed(sig)) {
				summaryInvitedFor = '';
				summaryPrompt = 'invite';
				return;
			}
		}
		editModeIntent = false;

		streamError = '';
		composer = '';
		const composerEl = document.getElementById('brief-composer') as HTMLTextAreaElement | null;
		if (composerEl) composerEl.style.height = '';

		// Lampiran diunggah DULU supaya dokumen sudah masuk sebelum giliran AI.
		// Kalau user cuma kirim file tanpa teks, susun pesan penyebut namanya
		// supaya AI tahu harus membaca apa.
		let message = text;
		if (pendingDocs.length > 0) {
			uploadingNow = true;
			const names: string[] = [];
			const imageNames: string[] = [];
			try {
				for (const d of pendingDocs) {
					const up = await uploadBriefFile(d.file);
					if (up.kind === 'image') imageNames.push(up.name);
					else names.push(up.name);
				}
			} catch (e) {
				uploadingNow = false;
				// Chip dipertahankan supaya bisa mencoba lagi tanpa memilih ulang.
				streamError = e instanceof Error ? e.message : tr('Gagal mengunggah dokumen.');
				return;
			}
			uploadingNow = false;
			pendingDocs = [];
			if (!message) {
				const parts: string[] = [];
				if (names.length > 0) {
					const list = names.map((n) => `\u{22}${n}\u{22}`).join(', ');
					parts.push(
						names.length === 1
							? tr(`Aku baru unggah dokumen ${list}. Baca dulu ya, lalu tunjukkan apa yang kamu tangkap.`)
							: tr(`Aku baru mengunggah ${names.length} dokumen: ${list}. Baca dulu ya, lalu tunjukkan apa yang kamu tangkap.`)
					);
				}
				if (imageNames.length > 0) {
					const list = imageNames.map((n) => `\u{22}${n}\u{22}`).join(', ');
					parts.push(
						imageNames.length === 1
							? tr(`Aku juga unggah gambar ${list} sebagai aset brand.`)
							: tr(`Aku juga mengunggah ${imageNames.length} gambar: ${list} sebagai aset brand.`)
					);
				}
				message = parts.join(' ');
			}
		}

		messages = [...messages, { role: 'user', content: message, created_at: new Date().toISOString() }];
		streaming = true;
		streamingText = '';
		streamStatus = '';
		controller = new AbortController();

		const token = getAccessToken();
		try {
			const res = await fetch(`${briefBase}/chat/stream`, {
				method: 'POST',
				headers: {
					...(token ? { Authorization: `Bearer ${token}` } : {}),
					'Content-Type': 'application/json'
				},
				credentials: 'include',
				body: JSON.stringify({ message }),
				signal: controller.signal
			});
			if (!res.ok || !res.body) throw new Error(await readApiError(res, tr('Gagal mengirim pesan.')));

			const reader = res.body.getReader();
			const dec = new TextDecoder();
			let buf = '';
			let finished = false;
			let partial = '';

			while (!finished) {
				const { value, done: fin } = await reader.read();
				if (fin) break;
				buf += dec.decode(value, { stream: true });
				let idx: number;
				while ((idx = buf.indexOf('\n\n')) >= 0) {
					const block = buf.slice(0, idx);
					buf = buf.slice(idx + 2);
					let ev = '';
					let payload = '';
					for (const line of block.split('\n')) {
						if (line.startsWith('event:')) ev = line.slice(6).trim();
						else if (line.startsWith('data:')) payload += line.slice(5).trim();
					}
					if (!payload) continue;
					const data = JSON.parse(payload);
					if (ev === 'delta' && typeof data.append === 'string') {
						partial += data.append;
						streamingText = partial;
						streamStatus = '';
					} else if (ev === 'status') {
						// Progres backend sebelum balasan mengalir (mis. "Membaca tautan …").
						streamStatus = String(data.message ?? '');
					} else if (ev === 'done') {
						finished = true;
						messages = [...messages, {
							role: 'assistant',
							// id dari server: wajib ada supaya tombol aksi bisa
							// menandai dirinya terpakai. Tanpa id, klik tidak
							// mengirim apa pun dan tombol tampak mati.
							id: typeof data.message_id === 'string' ? data.message_id : undefined,
							content: String(data.reply ?? partial),
							created_at: new Date().toISOString(),
							// Aksi usulan ikut di respons "done", supaya tombolnya
							// muncul tanpa menunggu reload.
							actions: Array.isArray(data.actions) ? data.actions : undefined,
							actions_used: []
						}];
						applyDone(data);
					} else if (ev === 'error') {
						streamError = String(data.message ?? tr('Generasi gagal.'));
					}
				}
			}
			if (!finished && partial) {
				// Koneksi putus di tengah: yang sudah tampil tetap dipertahankan,
				// tapi tidak ditandai selesai supaya user tahu itu belum lengkap.
				streamError = tr('Koneksi terputus saat AI menulis. Balasan ini belum tuntas, coba kirim ulang untuk melanjutkan.');
			}
		} catch (e) {
			if ((e as Error)?.name === 'AbortError') {
				if (streamingText) messages = [...messages, { role: 'assistant', content: streamingText, created_at: new Date().toISOString() }];
				streamError = tr('Generasi dihentikan.');
			} else {
				streamError = e instanceof Error ? e.message : tr('Gagal mengirim pesan.');
			}
		} finally {
			working(false);
			generationCount++;
			streaming = false;
			streamingText = '';
			streamStatus = '';
			controller = null;
			// askedField dari giliran ini dikelola applyDone — jangan direset di sini.
		}
	}

	function applyDone(data: any) {
		phase = String(data?.phase ?? phase);
		progress = data?.progress ?? progress;
		for (const u of data?.field_updates ?? []) {
			fieldStates = { ...fieldStates, [u.key]: { status: u.status, source: u.source, note: u.note ?? '' } };
			ringkasan[u.key] = String(u.value ?? '');
		}
		askedField = data?.asked_field ?? null;
		lastSyncAt = Date.now();
		void loadSources();
	}

	async function loadSources() {
		const res = await apiRequest(`${briefBase}/chat`);
		if (!res.ok) return;
		const data = await res.json();
		sources = data?.sources ?? [];
	}

	// ---------- layanan pendukung: URL & dokumen ----------


	const MAX_UPLOAD_BYTES = 10 * 1024 * 1024;

	// Gambar disimpan sebagai aset brand (profiler_images), dokumen masuk ke
	// profiler_documents supaya teksnya bisa dibaca AI.
	const IMAGE_FILE = /^image\/(jpeg|png|webp|gif)$/i;

	function isImageFile(file: File): boolean {
		return IMAGE_FILE.test(file.type) || /\.(jpe?g|png|webp|gif)$/i.test(file.name);
	}

	// Unggah satu berkas; jenisnya dikembalikan untuk menyusun pesan.
	// source 'chat' (composer Copilot) memakan kuota percakapan (maks 5 berkas
	// / 25 MB per brief); 'library' (tab Media) hanya kena kuota harian user.
	async function uploadBriefFile(file: File, source: 'chat' | 'library' = 'chat'): Promise<{ name: string; kind: 'doc' | 'image' }> {
		const image = isImageFile(file);
		const form = new FormData();
		form.append('file', file);
		form.append('source', source);
		const res = await apiRequest(`${apiBase}/profiles/${profileId}/${image ? 'images' : 'documents'}`, {
			method: 'POST',
			body: form
		}, 120_000);
		if (!res.ok) {
			throw new Error(
				await readApiError(res, image ? tr('Gagal mengunggah gambar.') : tr('Gagal mengunggah dokumen.'))
			);
		}
		return { name: file.name, kind: image ? 'image' : 'doc' };
	}

	async function loadMedia() {
		if (!profileId) return;
		const seq = ++mediaLoadSeq;
		mediaLoading = true;
		mediaError = '';
		try {
			const res = await apiRequest(`${apiBase}/profiles/${profileId}`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat media.')));
			const data = await res.json();
			if (seq !== mediaLoadSeq) return;
			mediaDocs = Array.isArray(data?.documents) ? data.documents : [];
			mediaImages = Array.isArray(data?.images) ? data.images : [];
		} catch (e) {
			if (seq === mediaLoadSeq) mediaError = e instanceof Error ? e.message : tr('Gagal memuat media.');
		} finally {
			if (seq === mediaLoadSeq) mediaLoading = false;
		}
	}

	// Unggah dari tab Media: satu jalur dengan lampiran Copilot, langsung segar
	// daftarnya setelah selesai.
	async function uploadMediaFiles(list: FileList | File[] | null | undefined) {
		if (!list || mediaUploading) return;
		const files = Array.from(list);
		if (files.length === 0) return;
		const tooBig = files.filter((f) => f.size > MAX_UPLOAD_BYTES).map((f) => `${f.name} (${fmtSize(f.size)})`);
		const accepted = files.filter((f) => f.size <= MAX_UPLOAD_BYTES);
		mediaError = tooBig.length > 0 ? `${tr('Melebihi 10 MB, tidak dilampirkan')}: ${tooBig.join(', ')}` : '';
		if (accepted.length === 0) return;
		mediaUploading = true;
		try {
			for (const file of accepted) await uploadBriefFile(file, 'library');
			await loadMedia();
		} catch (e) {
			mediaError = e instanceof Error ? e.message : tr('Gagal mengunggah media.');
		} finally {
			mediaUploading = false;
		}
	}

	function removeMediaDoc(doc: MediaDoc) {
		openConfirm({
			title: tr('Hapus dokumen?'),
			body: tr(`Dokumen "${doc.original_filename}" akan dihapus dari brief ini dan tidak dipakai AI sebagai referensi lagi.`),
			confirmLabel: tr('Hapus'),
			danger: true,
			action: async () => {
				const res = await apiRequest(`${apiBase}/documents/${doc.id}`, { method: 'DELETE' });
				if (!res.ok) {
					mediaError = await readApiError(res, tr('Gagal menghapus dokumen.'));
					return;
				}
				mediaDocs = mediaDocs.filter((d) => d.id !== doc.id);
			}
		});
	}

	function removeMediaImage(img: MediaImage) {
		const label = img.file_name || tr('gambar');
		openConfirm({
			title: tr('Hapus gambar?'),
			body: tr(`Gambar "${label}" akan dihapus dari brief ini.`),
			confirmLabel: tr('Hapus'),
			danger: true,
			action: async () => {
				const res = await apiRequest(`${apiBase}/images/${img.id}`, { method: 'DELETE' });
				if (!res.ok) {
					mediaError = await readApiError(res, tr('Gagal menghapus gambar.'));
					return;
				}
				mediaImages = mediaImages.filter((i) => i.id !== img.id);
			}
		});
	}

	function mediaDate(iso?: string | null): string {
		if (!iso) return '';
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return '';
		return d.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' });
	}

	// ---------- timeline ----------

	async function loadTimeline() {
		if (!profileId) return;
		timelineLoading = true;
		timelineError = '';
		try {
			// Identitas brand hanya pelengkap tampilan: gagal di sini tidak boleh
			// membuat feed ikut kosong.
			const [brandRes, timelineRes] = await Promise.all([
				apiRequest(brandApi).catch(() => null),
				apiRequest(`${apiBase}/profiles/${profileId}/timeline`)
			]);
			if (!timelineRes.ok) throw new Error(await readApiError(timelineRes, tr('Gagal memuat timeline.')));
			const timelineData = await timelineRes.json();
			timeline = Array.isArray(timelineData?.items) ? timelineData.items : [];
			if (brandRes?.ok) {
				const brandData = await brandRes.json().catch(() => null);
				timelineBrandName = String(brandData?.name || profileName || tr('Brand'));
				timelineBrandSlug = String(brandData?.slug || brandData?.public_id || '');
			} else {
				timelineBrandName = String(profileName || tr('Brand'));
			}
		} catch (e) {
			timelineError = e instanceof Error ? e.message : tr('Gagal memuat timeline.');
		} finally {
			timelineLoading = false;
		}
	}

	async function openTimelineComposer() {
		timelinePostError = '';
		timelineDraftsOpen = false;
		timelineComposerOpen = true;
		await tick();
		growTimelineInput();
		// Kursor langsung siap menulis begitu modal terbuka.
		timelineInputEl?.focus();
	}

	// Kembali dari daftar draf ke kolom tulis, kursor ikut aktif lagi.
	async function backToTimelineComposer() {
		timelineDraftsOpen = false;
		await tick();
		growTimelineInput();
		const el = timelineInputEl;
		if (el) {
			el.focus();
			el.setSelectionRange(el.value.length, el.value.length);
		}
	}

	// Textarea modal ringkas saat kosong, tumbuh mengikuti isi sampai batas 40vh.
	let timelineInputEl = $state<HTMLTextAreaElement | null>(null);
	function growTimelineInput() {
		const el = timelineInputEl;
		if (!el) return;
		el.style.height = 'auto';
		const max = Math.round(window.innerHeight * 0.4);
		el.style.height = `${Math.min(el.scrollHeight, max)}px`;
	}

	// Menutup composer dengan isi → tawarkan simpan sebagai draf dulu.
	// `intent` menentukan tujuan setelah jawaban: tutup modal atau buka daftar draf.
	let timelineDiscardIntent = $state<'close' | 'drafts'>('close');

	function timelineHasDraftContent(): boolean {
		return Boolean(timelineBody.trim() || timelineImageFile || timelineImageId || timelineLink);
	}

	function requestCloseTimelineComposer() {
		if (timelineHasDraftContent()) {
			timelineDiscardIntent = 'close';
			timelineDiscardAsk = true;
			return;
		}
		closeTimelineComposer();
	}

	// Ikon draf: kalau ada isi yang belum dikirim, simpan dulu (bukan langsung
	// pindah layar dan meninggalkan tulisan).
	function openTimelineDrafts() {
		if (timelineHasDraftContent()) {
			timelineDiscardIntent = 'drafts';
			timelineDiscardAsk = true;
			return;
		}
		timelineDraftsOpen = true;
	}

	function closeTimelineComposer() {
		timelineComposerOpen = false;
		timelineDragging = false;
		timelineDiscardAsk = false;
		timelineDraftsOpen = false;
	}

	// ---------- draf timeline (localStorage per brief) ----------

	function timelineDraftsKey(): string {
		return `timeline.drafts.${profileId}`;
	}

	function loadTimelineDrafts() {
		try {
			const raw = localStorage.getItem(timelineDraftsKey());
			const parsed = raw ? JSON.parse(raw) : [];
			timelineDrafts = Array.isArray(parsed) ? parsed : [];
		} catch {
			timelineDrafts = [];
		}
	}

	function persistTimelineDrafts() {
		try {
			localStorage.setItem(timelineDraftsKey(), JSON.stringify(timelineDrafts));
		} catch {
			// Penyimpanan penuh/diblokir bukan alasan menggagalkan penulisan.
		}
	}

	function resetTimelineComposer() {
		timelineBody = '';
		clearTimelineImage();
		clearTimelineLink();
		timelinePostError = '';
	}

	async function saveTimelineDraft() {
		// Gambar yang masih diunggah ditunggu sebentar supaya draf tidak
		// kehilangan gambarnya; kalau gagal, draf tetap disimpan tanpa gambar.
		let imageId = timelineImageId;
		let imageUrl = timelineImageUrl;
		if (!imageId && timelineImageFile) {
			try {
				imageId = await (timelineUploadPromise ?? startTimelineUpload(timelineImageFile));
				imageUrl = timelineImageUrl;
			} catch {
				imageId = '';
				imageUrl = '';
			}
		}
		timelineDrafts = [
			{
				id: crypto.randomUUID(),
				body: timelineBody,
				link: timelineLink,
				imageId,
				imageUrl,
				savedAt: Date.now()
			},
			...timelineDrafts
		].slice(0, 20);
		persistTimelineDrafts();
		resetTimelineComposer();
		timelineDiscardAsk = false;
		if (timelineDiscardIntent === 'drafts') {
			timelineDraftsOpen = true;
		} else {
			closeTimelineComposer();
		}
	}

	function discardTimelineDraft() {
		resetTimelineComposer();
		timelineDiscardAsk = false;
		if (timelineDiscardIntent === 'drafts') {
			timelineDraftsOpen = true;
		} else {
			closeTimelineComposer();
		}
	}

	async function resumeTimelineDraft(draft: TimelineDraft) {
		resetTimelineComposer();
		timelineBody = draft.body;
		timelineLink = draft.link;
		if (draft.imageId) {
			timelineImageId = draft.imageId;
			timelineImageUrl = draft.imageUrl;
			timelineImagePreview = draft.imageUrl;
		}
		timelineDraftsOpen = false;
		timelineComposerOpen = true;
		await tick();
		growTimelineInput();
		const el = timelineInputEl;
		if (el) {
			el.focus();
			el.setSelectionRange(el.value.length, el.value.length);
		}
	}

	function deleteTimelineDraft(id: string) {
		timelineDrafts = timelineDrafts.filter((d) => d.id !== id);
		persistTimelineDrafts();
	}

	// Gambar: berkas dipasang lokal untuk pratinjau, unggahannya langsung jalan
	// di latar belakang supaya saat Kirim tinggal menunggu/melanjutkan.
	function attachTimelineImage(file: File | null | undefined) {
		if (!file) return;
		if (!isImageFile(file)) {
			timelinePostError = tr('Hanya gambar (JPG, PNG, WebP, GIF) yang bisa dilampirkan.');
			return;
		}
		if (file.size > MAX_UPLOAD_BYTES) {
			timelinePostError = `${tr('Gambar melebihi 10 MB')}: ${file.name} (${fmtSize(file.size)})`;
			return;
		}
		timelinePostError = '';
		if (timelineImagePreview) URL.revokeObjectURL(timelineImagePreview);
		timelineImageFile = file;
		timelineImagePreview = URL.createObjectURL(file);
		timelineImageId = '';
		void startTimelineUpload(file);
	}

	// startTimelineUpload: mulai (atau ulangi) unggahan latar belakang.
	function startTimelineUpload(file: File): Promise<string> {
		const seq = ++timelineUploadSeq;
		timelineUploading = true;
		timelineUploadFailed = false;
		const p = uploadTimelineImage(file)
			.then((up) => {
				if (seq === timelineUploadSeq) {
					timelineImageId = up.id;
					timelineImageUrl = up.url;
					timelineUploading = false;
					timelineUploadFailed = false;
				}
				return up.id;
			})
			.catch((e) => {
				if (seq === timelineUploadSeq) {
					timelineUploading = false;
					timelineUploadFailed = true;
				}
				throw e;
			});
		// Tangani penolakan tanpa menelan hasilnya: promise aslinya tetap
		// di-await oleh submitTimelinePost.
		void p.catch(() => {});
		timelineUploadPromise = p;
		return p;
	}

	function clearTimelineImage() {
		// Naikkan nomor urut supaya hasil upload yang masih jalan diabaikan.
		timelineUploadSeq++;
		if (timelineImagePreview) URL.revokeObjectURL(timelineImagePreview);
		timelineImagePreview = '';
		timelineImageFile = null;
		timelineImageId = '';
		timelineImageUrl = '';
		timelineUploading = false;
		timelineUploadFailed = false;
		timelineUploadPromise = null;
	}

	function clearTimelineLink() {
		timelineLink = null;
		timelineLinkLoading = false;
	}

	// Ambil metadata OG lewat API (server yang fetch, jadi tidak kena CORS).
	async function loadTimelineLink(raw: string) {
		const url = raw.trim();
		if (!url) return;
		if (timelineLink?.url === url) return;
		timelineLink = { url, title: '', description: '', image: '', site: '' };
		timelineLinkLoading = true;
		try {
			const res = await apiRequest(`${apiBase}/link-preview?url=${encodeURIComponent(url)}`);
			if (!res.ok) return;
			const data = await res.json();
			if (!data?.ok) return;
			timelineLink = {
				url: String(data.url || url),
				title: String(data.title || ''),
				description: String(data.description || ''),
				image: String(data.image || ''),
				site: String(data.site || '')
			};
		} catch {
			// Pratinjau gagal bukan alasan menggagalkan posting: tautan tetap dikirim.
		} finally {
			timelineLinkLoading = false;
		}
	}

	function firstURLIn(text: string): string {
		const m = text.match(/https?:\/\/[^\s<>"]+/i);
		return m ? m[0].replace(/[),.;]+$/, '') : '';
	}

	function handleTimelinePaste(e: ClipboardEvent) {
		const files = e.clipboardData?.files;
		if (files && files.length > 0) {
			const image = Array.from(files).find((f) => isImageFile(f));
			if (image) {
				e.preventDefault();
				attachTimelineImage(image);
				return;
			}
		}
		const text = e.clipboardData?.getData('text') ?? '';
		const url = firstURLIn(text);
		if (url && !timelineLink) void loadTimelineLink(url);
	}

	function handleTimelineDrop(e: DragEvent) {
		e.preventDefault();
		timelineDragging = false;
		const file = Array.from(e.dataTransfer?.files ?? []).find((f) => isImageFile(f));
		if (file) attachTimelineImage(file);
	}

	// Tagar yang diketik langsung (#promo) ikut dikirim sebagai bahan AI.
	function timelineHashtagsOf(body: string): string[] {
		const out = Array.from(body.matchAll(/#([\p{L}\p{N}_]{1,40})/gu)).map((m) => m[1]);
		return Array.from(new Set(out)).slice(0, 10);
	}

	// Pecah caption jadi potongan teks + tagar agar tagar bisa diwarnai
	// tanpa menjadi tautan.
	type BodyPart = { text: string; tag: boolean };
	function timelineBodyParts(body: string): BodyPart[] {
		const parts: BodyPart[] = [];
		const re = /#[\p{L}\p{N}_]{1,40}/gu;
		let last = 0;
		for (const m of body.matchAll(re)) {
			const at = m.index ?? 0;
			if (at > last) parts.push({ text: body.slice(last, at), tag: false });
			parts.push({ text: m[0], tag: true });
			last = at + m[0].length;
		}
		if (last < body.length) parts.push({ text: body.slice(last), tag: false });
		return parts;
	}

	async function uploadTimelineImage(file: File): Promise<{ id: string; url: string }> {
		const form = new FormData();
		form.append('file', file);
		form.append('source', 'library');
		const res = await apiRequest(`${apiBase}/profiles/${profileId}/images`, { method: 'POST', body: form }, 120_000);
		if (!res.ok) throw new Error(await readApiError(res, tr('Gagal mengunggah gambar timeline.')));
		return await res.json();
	}

	async function submitTimelinePost() {
		const body = timelineBody.trim();
		if ((!body && !timelineImageFile && !timelineLink) || timelinePosting) return;
		timelinePosting = true;
		timelinePostError = '';
		try {
			let imageId = timelineImageId;
			if (timelineImageFile && !imageId) {
				// Lanjutkan unggahan yang masih berjalan; kalau sebelumnya gagal,
				// ulangi sekarang (jenis berkas & ukuran sudah divalidasi saat pasang).
				const pending = !timelineUploadPromise || timelineUploadFailed
					? startTimelineUpload(timelineImageFile)
					: timelineUploadPromise;
				imageId = await pending;
			}
			const res = await apiRequest(`${apiBase}/profiles/${profileId}/timeline`, {
				method: 'POST',
				body: JSON.stringify({
					body,
					hashtags: timelineHashtagsOf(body),
					image_id: imageId || undefined,
					link_url: timelineLink?.url || undefined
				})
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memposting timeline.')));
			timelineBody = '';
			clearTimelineImage();
			clearTimelineLink();
			timelineComposerOpen = false;
			await loadTimeline();
		} catch (e) {
			timelinePostError = e instanceof Error ? e.message : tr('Gagal memposting timeline.');
		} finally {
			timelinePosting = false;
		}
	}

	async function deleteTimelinePost(post: TimelinePost) {
		openConfirm({
			title: tr('Hapus post timeline?'),
			body: tr('Post ini akan dihapus dari timeline dan tidak dipakai lagi sebagai bahan artikel.'),
			confirmLabel: tr('Hapus'),
			danger: true,
			action: async () => {
				const res = await apiRequest(`${apiBase}/timeline/${post.id}`, { method: 'DELETE' });
				if (!res.ok) {
					timelinePostError = await readApiError(res, tr('Gagal menghapus post.'));
					return;
				}
				timeline = timeline.filter((p) => p.id !== post.id);
			}
		});
	}

	function timelineRelativeTime(iso?: string | null): string {
		if (!iso) return '';
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return '';
		const seconds = Math.max(0, Math.floor((Date.now() - d.getTime()) / 1000));
		if (seconds < 60) return tr('Baru saja');
		if (seconds < 3600) return `${Math.floor(seconds / 60)} ${tr('menit lalu')}`;
		if (seconds < 86400) return `${Math.floor(seconds / 3600)} ${tr('jam lalu')}`;
		if (seconds < 30 * 86400) return `${Math.floor(seconds / 86400)} ${tr('hari lalu')}`;
		return mediaDate(iso);
	}

	function timelineInitials(name: string): string {
		const words = name.trim().split(/\s+/).filter(Boolean);
		if (words.length > 1) return (words[0][0] + words[1][0]).toLocaleUpperCase('id-ID');
		return (words[0]?.slice(0, 2) || 'TH').toLocaleUpperCase('id-ID');
	}

	// Identitas penulis = akun yang login (sama dengan halaman Profil & Akun),
	// bukan nama brand. Brand tetap jadi konteks halaman ("Timeline Brand").
	function timelineAuthorName(): string {
		return String(userName || timelineBrandName || profileName || tr('Brand'));
	}

	function timelinePostAuthor(post: TimelinePost): string {
		return String(post.author_name || timelineAuthorName());
	}

	function copyTimelineLink(post: TimelinePost) {
		const url = `${window.location.origin}/klien/${brandUrl}/profiler/${activeShortCode}#timeline-${post.id}`;
		void navigator.clipboard.writeText(url).then(() => showToast(tr('Tautan post disalin.'))).catch(() => showToast(tr('Gagal menyalin tautan.'), 'error'));
	}

	// ---------- ringkasan AI (entitas & file) ----------

	function summaryOf(kind: SummaryKind): SummaryItem {
		return (
			summaries.find((s) => s.kind === kind) ?? {
				kind,
				status: 'idle',
				content: '',
				source_chars: 0,
				token_input: 0,
				token_output: 0,
				model: '',
				error: '',
				generated_at: null,
				updated_at: null,
				stale: false
			}
		);
	}

	// ---------- riwayat perubahan brief ----------
	// Dimuat malas saat panel dibuka, tanpa polling. Filter field dikirim ke
	// server (?field=) supaya daftar panjang tidak perlu disaring di browser.

	type HistoryEntry = {
		field_key: string;
		old_value: string | null;
		new_value: string | null;
		changed_by_name: string | null;
		changed_at: string | null;
	};

	let historyOpen = $state(false);
	let historyFilter = $state('');
	let historyEntries = $state<HistoryEntry[]>([]);
	let historyLoading = $state(false);
	let historyError = $state('');
	// Kunci brief+filter yang datanya sudah dimuat; kosong = belum pernah.
	let historyLoadedFor = $state('');
	let historyPanelFor = $state('');

	function historyFieldLabel(key: string): string {
		return fields.find((f) => f.key === key)?.label ?? key.replace(/_/g, ' ');
	}

	const historyOptions = $derived.by(() => {
		const opts = [{ value: '', label: tr('Semua field') }];
		const seen = new Set<string>();
		for (const f of fields) {
			if (seen.has(f.key)) continue;
			seen.add(f.key);
			opts.push({ value: f.key, label: f.label });
		}
		// Kunci lama yang tidak lagi ada di skema tetap bisa disaring.
		for (const entry of historyEntries) {
			if (seen.has(entry.field_key)) continue;
			seen.add(entry.field_key);
			opts.push({ value: entry.field_key, label: historyFieldLabel(entry.field_key) });
		}
		return opts;
	});

	function truncateHistoryValue(value: string | null, max = 80): string {
		const text = (value ?? '').replace(/\s+/g, ' ').trim();
		if (!text) return '—';
		return text.length > max ? `${text.slice(0, max - 1)}…` : text;
	}

	async function loadHistory(field = historyFilter) {
		if (!profileId) return;
		historyLoading = true;
		historyError = '';
		try {
			const query = field ? `?field=${encodeURIComponent(field)}` : '';
			const res = await apiRequest(`${apiBase}/profiles/${profileId}/history${query}`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat riwayat perubahan.')));
			const data = await res.json();
			historyEntries = Array.isArray(data) ? data : Array.isArray(data?.items) ? data.items : [];
			historyLoadedFor = `${profileId}|${field}`;
		} catch (e) {
			historyError = e instanceof Error ? e.message : tr('Gagal memuat riwayat perubahan.');
		} finally {
			historyLoading = false;
		}
	}

	function toggleHistory(open: boolean) {
		historyOpen = open;
		if (!open) return;
		// Brief berganti: buang sisa riwayat brief sebelumnya sebelum memuat.
		if (!historyLoadedFor.startsWith(`${profileId}|`)) {
			historyEntries = [];
			historyError = '';
		}
		if (historyLoadedFor !== `${profileId}|${historyFilter}`) void loadHistory();
	}

	$effect(() => {
		// Reset saat brief berganti; panel yang sedang terbuka langsung memuat
		// riwayat brief yang baru.
		if (historyPanelFor === profileId) return;
		historyPanelFor = profileId;
		historyEntries = [];
		historyError = '';
		historyLoadedFor = '';
		if (historyOpen && profileId) void loadHistory();
	});

	async function loadSummaries(silent = false, statusOnly = false) {
		if (!profileId) return;
		const seq = ++summarySeq;
		if (!silent) summaryLoading = true;
		let transitioned = false;
		try {
			const res = await apiRequest(`${briefBase}/summaries${statusOnly ? '?status=1' : ''}`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat ringkasan.')));
			const data = await res.json();
			if (seq !== summarySeq) return;
			summaries = Array.isArray(data?.items) ? data.items : [];
			summaryError = '';
			// Polling status: begitu tidak ada job berjalan, ambil versi lengkap
			// (termasuk penanda kedaluwarsa).
			transitioned = statusOnly && !summaries.some((s) => s.status === 'queued' || s.status === 'running');
		} catch (e) {
			if (seq === summarySeq) summaryError = e instanceof Error ? e.message : tr('Gagal memuat ringkasan.');
		}
		if (seq !== summarySeq) return;
		summaryLoading = false;
		if (transitioned) {
			await loadSummaries(true, false);
			return;
		}
		scheduleSummaryPoll();
	}

	// Polling hanya selama ada job berjalan; berhenti sendiri saat selesai.
	// Jitter 4–7 detik mencegah ratusan tab memukul API serentak.
	function scheduleSummaryPoll() {
		if (summaryTimer) {
			clearTimeout(summaryTimer);
			summaryTimer = null;
		}
		if (!summaries.some((s) => s.status === 'queued' || s.status === 'running')) return;
		summaryTimer = setTimeout(() => void loadSummaries(true, true), 4000 + Math.random() * 3000);
	}

	async function regenerateSummary(kind: SummaryKind) {
		if (summaryBusy) return;
		summaryBusy = kind;
		summaryError = '';
		try {
			const res = await apiRequest(`${briefBase}/summaries/${kind}/regenerate`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menjadwalkan ringkasan.')));
			await loadSummaries(true);
		} catch (e) {
			summaryError = e instanceof Error ? e.message : tr('Gagal menjadwalkan ringkasan.');
		} finally {
			summaryBusy = null;
		}
	}

	// Popup: user setuju diringkas. Animasi 'working' ditahan minimal ~1,6 detik
	// supaya transisi terasa disengaja walau job-nya cepat, lalu menunggu job
	// benar-benar selesai (status ready/failed).
	async function runSummaryFromPrompt() {
		// Tandai sudah ditawari sebelum mulai: bila admin refresh saat proses
		// berjalan, popup ajakan tidak muncul lagi.
		try {
			localStorage.setItem(inviteStorageKey(inviteSignature()), '1');
		} catch {
			// abaikan mode privat
		}
		summaryPrompt = 'working';
		summaryPromptError = '';
		const startedAt = Date.now();
		await regenerateSummary('entity');
		if (summaryOf('entity').status === 'failed') {
			summaryPromptError = summaryOf('entity').error || tr('Ringkasan gagal dibuat. Coba lagi sebentar lagi.');
			summaryPrompt = 'error';
			return;
		}
		for (let i = 0; i < 60; i++) {
			if (summaryOf('entity').status === 'failed') {
				summaryPromptError = summaryOf('entity').error || tr('Ringkasan gagal dibuat.');
				summaryPrompt = 'error';
				return;
			}
			if (summaryOf('entity').status === 'ready') break;
			await new Promise((r) => setTimeout(r, 800));
			await loadSummaries(true, true);
		}
		const elapsed = Date.now() - startedAt;
		if (elapsed < 1600) await new Promise((r) => setTimeout(r, 1600 - elapsed));
		summaryPrompt = summaryOf('entity').status === 'ready' ? 'done' : 'error';
		if (summaryPrompt === 'error' && !summaryPromptError) {
			summaryPromptError = tr('Ringkasan belum selesai. Tunggu sebentar, lalu periksa lagi.');
		}
	}

	// Field opsional (USP, fakta penting, dsb.) tidak dihitung progres dan tidak
	// ditanyakan otomatis oleh agent. Tombol ini meminta Copilot menawarkan
	// isiannya — tetap LLM yang menyusun, dari state & sumber yang sudah ada.
	async function askCopilotForOptionals() {
		if (streaming) return;
		const missing = optionalEmpty.slice(0, 10).map((f) => f.label);
		if (missing.length === 0) return;
		editModeIntent = true;
		summaryPrompt = 'idle';
		activeTab = 'copilot';
		await send(
			tr(
				`Semua field wajib sudah terisi. Sekarang tawarkan isian untuk field opsional yang masih kosong ini: ${missing.join(', ')}. ` +
					'Untuk tiap field, ajukan satu nilai berdasar fakta yang sudah ada di brief (source "ai_suggested"), tampilkan sebagai draft yang bisa kujawab singkat: pakai / ubah / lewati. Mulai dari Keunggulan (USP) dan Fakta penting yang wajib dipertahankan.'
			)
		);
	}

	// Lanjut obrolan setelah ringkasan jadi. Klien TIDAK menyusun daftar data:
	// konteks giliran (stateBlock) sudah memuat setiap field beserta status dan
	// nilainya, jadi LLM sendirilah yang membaca dan menyajikan pilihannya.
	// Klien hanya menyampaikan niat user.
	async function continueToEdit() {
		summaryPrompt = 'idle';
		editModeIntent = true;
		activeTab = 'copilot';
		const typed = composer.trim();
		const intent = tr(
			'Entitas sudah lengkap dan ringkasannya sudah dibuat. Aku ingin meninjau dan mengedit entitas ini.'
		);
		await send(typed ? `${intent}\n\n${typed}` : intent);
	}

	async function copySummary(item: SummaryItem) {
		try {
			await navigator.clipboard.writeText(item.content);
			showToast(tr('Ringkasan disalin.'));
		} catch {
			showToast(tr('Gagal menyalin.'), 'error');
		}
	}

	function summaryDate(iso?: string | null): string {
		if (!iso) return '';
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return '';
		return d.toLocaleString('id-ID', { day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' });
	}

	function summaryStatus(item: SummaryItem): { label: string; bg: string; color: string } {
		if (item.status === 'queued' || item.status === 'running') {
			return { label: tr('Sedang dibuat…'), bg: 'var(--warning-soft)', color: 'var(--warning)' };
		}
		if (item.status === 'failed') {
			return { label: tr('Gagal'), bg: 'color-mix(in srgb, var(--destructive) 10%, white)', color: 'var(--destructive)' };
		}
		if (item.status === 'ready' && item.stale) {
			return { label: tr('Kedaluwarsa'), bg: 'var(--warning-soft)', color: 'var(--warning)' };
		}
		if (item.status === 'ready') {
			return { label: tr('Siap'), bg: 'var(--success-soft)', color: 'var(--success)' };
		}
		return { label: tr('Belum dibuat'), bg: 'var(--secondary)', color: 'var(--muted-foreground)' };
	}

	function onMediaDragEnter(e: DragEvent) {
		if (!hasFileDrag(e)) return;
		e.preventDefault();
		e.stopPropagation();
		mediaDragDepth += 1;
		mediaDragging = true;
	}
	function onMediaDragLeave(e: DragEvent) {
		e.stopPropagation();
		mediaDragDepth = Math.max(0, mediaDragDepth - 1);
		if (mediaDragDepth === 0) mediaDragging = false;
	}
	function onMediaDrop(e: DragEvent) {
		e.preventDefault();
		e.stopPropagation();
		mediaDragDepth = 0;
		mediaDragging = false;
		void uploadMediaFiles(e.dataTransfer?.files);
	}

	// Textarea tumbuh mengikuti isi sampai 50vh. Composer selalu dua area
	// (teks di atas, toolbar satu baris di bawah) — tanpa mode ganda, jadi
	// cukup ukur sekali: height auto dulu agar bisa menyusut.
	function autoGrow(el: HTMLTextAreaElement) {
		el.style.height = 'auto';
		el.style.overflowY = 'hidden';
		const max = Math.round(window.innerHeight * 0.5);
		el.style.height = `${Math.min(el.scrollHeight, max)}px`;
		if (el.scrollHeight > max) el.style.overflowY = 'auto';
	}

	// Dikte suara (Web Speech API). Browser tanpa dukungan tidak menampilkan
	// tombol mikrofon sama sekali. Hasil final ditambahkan ke composer.
	const speechSupported = typeof window !== 'undefined' && ('SpeechRecognition' in window || 'webkitSpeechRecognition' in window);
	let listening = $state(false);
	let recognition: any = null;
	function toggleMic() {
		if (!speechSupported) return;
		if (listening) {
			recognition?.stop();
			return;
		}
		const Ctor = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
		if (!Ctor) return;
		recognition = new Ctor();
		recognition.lang = 'id-ID';
		recognition.continuous = true;
		recognition.interimResults = false;
		recognition.onresult = (ev: any) => {
			let text = '';
			for (let i = ev.resultIndex; i < ev.results.length; i++) {
				if (ev.results[i].isFinal) text += ev.results[i][0].transcript;
			}
			if (text.trim()) {
				composer = composer ? composer.trimEnd() + ' ' + text.trim() : text.trim();
				// Sama seperti template: DOM menghorus setelah tick, ukur di sana.
				void tick().then(() => {
					const el = document.getElementById('brief-composer') as HTMLTextAreaElement | null;
					if (el) autoGrow(el);
				});
			}
		};
		recognition.onend = () => (listening = false);
		recognition.onerror = () => (listening = false);
		recognition.start();
		listening = true;
	}

	function fmtSize(bytes: number): string {
		if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
		if (bytes >= 1024) return `${Math.round(bytes / 1024)} KB`;
		return `${bytes} B`;
	}

	// Template brief siap pakai untuk klien yang mau satu set langsung.
	// Data yang AI TIDAK boleh mengarang (nama, angka, tahun, budget, URL)
	// ditanda [KURUNG SIKU] — user tinggal menggantinya dengan data asli
	// sebelum mengirim; teks lain sudah cukup untuk mengisi Entity Core.
	type BriefTemplate = { title: string; desc: string; text: string };

	const BRIEF_TEMPLATES: BriefTemplate[] = [
		{
			title: 'Agensi: satu set',
			desc: 'Brief lengkap untuk klien agensi. Identitas, budget, dan syarat publikasi tinggal diisi.',
			text: [
				'Brief gaya agensi. Target: profil entitas lengkap dalam satu sesi.',
				'',
				'IDENTITAS (WAJIB GANTI DENGAN DATA ASLI)',
				'Nama brand: [NAMA BRAND, ejaan resmi]',
				'Kategori: [PLATFORM / AGENSI / TOKO / LAINNYA]',
				'Deskripsi 1 kalimat: [apa yang membantu siapa untuk apa]',
				'Fakta resmi: berdiri [TAHUN] di [KOTA], [ANGKA MITRA/NASABAH/KLIENT] sampai hari ini. Angka ini tidak boleh dikarang.',
				'Website resmi: [URL]. Silakan baca dulu semua isinya.',
				'',
				'ANGGARAN & MEDIA',
				'Budget: maksimal Rp[NOMINAL] per media. Angka ini plafon, bukan harga minimum.',
				'Jumlah media: [JUMLAH]',
				'Minimum DA: [ANGKA], minimum AI Trust: [ANGKA]',
				'Syarat artikel: minimal [ANGKA] kata, [ANGKA] backlink per artikel, tautan [permanen / tidak permanen]',
				'Jenis media: Media Mainstream dan Media Blogger',
				'',
				'ATURAN & TUJUAN',
				'Kata/frasa terlarang: [misal "nomor 1", "terbaik", "terjangkau"]',
				'Klaim yang boleh dikutip (dengan buktinya): [DAFTAR KLAIM]',
				'Pertanyaan yang ingin dimenangkan di ChatGPT/Perplexity: [2-3 pertanyaan]'
			].join('\n')
		},
		{
			title: 'Personal brand',
			desc: 'Untuk tokoh dan praktisi. Posisi, kredibilitas, dan pertanyaan AI yang ingin dimenangkan.',
			text: [
				'Brief personal brand.',
				'',
				'IDENTITAS (WAJIB GANTI DENGAN DATA ASLI)',
				'Nama: [NAMA LENGKAP + GELAR]',
				'Profesi: [JABATAN, mis. founder / dokter / dosen / praktisi]',
				'Deskripsi 1 kalimat: [siapa, membantu siapa, untuk apa]',
				'Fakta resmi: pengalaman [TAHUN] di [BIDANG], afiliasi [PERUSAHAAN/INSTITUSI], prestasi [YANG BISA DIBUKTIKAN]. Kalau datanya tidak ada, biarkan kosong.',
				'',
				'POSISI DIGITAL',
				'Audiens: [SEGMENTASI KLIEN, mis. korporat, pemula, rekan sebaya]',
				'Pertanyaan yang ingin dimenangkan di ChatGPT/Perplexity: [2-3 pertanyaan]',
				'Keyword prioritas: [KEYWORD]. Target tautan: [URL]',
				'Kata/frasa terlarang: [KATA YANG DILARANG]',
				'',
				'GAYA & MEDIA',
				'Gaya bahasa: [PROFESIONAL / SANTAI PROFESIONAL]',
				'Budget: maksimal Rp[NOMINAL] per media, dengan [JUMLAH] media.',
				'Syarat artikel: minimal [ANGKA] kata, [ANGKA] backlink, tautan [permanen / tidak permanen]'
			].join('\n')
		},
		{
			title: 'Peluncuran produk',
			desc: 'Produk baru. Cerita produk, target pasar, dan rencana sebar publikasinya.',
			text: [
				'Brief peluncuran produk.',
				'',
				'PRODUK (WAJIB GANTI DENGAN DATA ASLI)',
				'Nama brand/produk: [NAMA]',
				'Kategori: [PRODUK DIGITAL / PRODUK FISIK / JASA]',
				'Deskripsi 2 kalimat: [apa yang dijual, siapa targetnya, apa bedanya]',
				'Fakta resmi: diluncurkan [TANGGAL/TAHUN] oleh [NAMA PERUSAHAAN], harga mulai Rp[ANGKA]',
				'Keunggulan utama (USP): [FITUR YANG BENAR-BENAR ADA, jangan dibesar-besarkan]',
				'',
				'DISTRIBUSI',
				'Budget: maksimal Rp[NOMINAL] per media. Angka ini plafon, bukan harga minimum.',
				'Jumlah media: [JUMLAH]',
				'Minimum DA: [ANGKA], minimum AI Trust: [ANGKA]',
				'Syarat artikel: minimal [ANGKA] kata, [ANGKA] backlink per artikel, tautan [permanen / tidak permanen]',
				'Jenis media: Media Mainstream dan Media Blogger',
				'',
				'TUJUAN',
				'Pertanyaan yang ingin dimenangkan di ChatGPT/Perplexity: [2-3 pertanyaan]',
				'Kata/frasa terlarang: [KATA YANG DILARANG]'
			].join('\n')
		}
	];

	function applyBriefTemplate(tpl: BriefTemplate) {
		composer = tpl.text;
		showToast(tr('Template dimuat. Tinggal ganti bagian [KURUNG SIKU] dengan data asli.'));
		document.getElementById('brief-composer')?.scrollIntoView({ behavior: 'smooth', block: 'center' });
		// autoGrow mengukur dari el.value — nilai state itu baru menghorus di
		// DOM setelah tick. Tanpa menunggu tick, pengukurannya memakai isi lama
		// (masih kosong) sehingga textarea tetap satu baris dan isinya tak terlihat.
		void tick().then(() => {
			const el = document.getElementById('brief-composer') as HTMLTextAreaElement | null;
			if (el) autoGrow(el);
			el?.focus();
		});
	}

	// ---------- Gerbang 2: Instan Brief ----------
	// Form Profil Brand: data permanen brand (identitas, fakta, kanal resmi,
	// aturan penulisan). Disimpan langsung ke field brief terstruktur lewat
	// POST fields/batch — deterministik, tanpa LLM dan tanpa berkas.
	// Kebutuhan kampanye (angle, keyword, budget) bukan bagian profil brand;
	// itu diatur terpisah saat membuat kampanye.
	let instanFormBusy = $state(false);
	// Saat tab "Isi field langsung"/"Template DOCX" menempel di atas, bar tab
	// utama (Instan Brief/Entitas/Media/Timeline/Ringkasan) disembunyikan agar
	// tidak ada dua baris tab. Bar itu muncul lagi saat digulir ke atas.
	let instanScrollEl = $state<HTMLElement | null>(null);
	let instanSwitchEl = $state<HTMLElement | null>(null);
	let briefHeadHidden = $state(false);
	let instanLastScrollTop = 0;
	// Animasi tinggi bar mengubah scrollTop di dekat dasar halaman; event yang
	// lahir dari perubahan itu diabaikan sebentar supaya tidak bolak-balik.
	let instanScrollLockUntil = 0;

	function setBriefHeadHidden(hidden: boolean) {
		if (briefHeadHidden === hidden) return;
		briefHeadHidden = hidden;
		// Popover Beta tidak boleh tertinggal terbuka saat bar menyingkir.
		if (hidden) betaOpen = false;
		instanScrollLockUntil = Date.now() + 260;
	}
	// Dua cara mengisi Profil Brand: langsung di sini, atau lewat template DOCX.
	// Default ke form karena itu jalur utama; pilihan disimpan selama halaman terbuka.
	let instanPath = $state<'field' | 'template'>('field');
	let instanErrors = $state<Partial<Record<'nama' | 'alias' | 'kategori' | 'kategoriLain' | 'deskripsi' | 'boilerplate' | 'fakta' | 'kanal' | 'kataTerlarang' | 'aturanKlaim' | 'klaim' | 'pertanyaan' | 'keywordUrl', string>>>({});
	let instanForm = $state({
		nama: '', alias: '', kategori: '', kategoriLain: '', deskripsi: '', boilerplate: '',
		fakta: '', kanal: '', kataTerlarang: '', aturanKlaim: '', klaim: '', pertanyaan: '', keywordUrl: ''
	});

	const INSTAN_CATEGORIES = ['Perusahaan', 'Produk', 'Layanan', 'Organisasi', 'Personal brand', 'Lainnya'];

	// Kategori yang tersimpan dari obrolan bisa di luar daftar (mis. "Agensi").
	// Nilai itu tetap ditampilkan sebagai opsi supaya konteks brand tidak hilang.
	const instanCategoryOptions = $derived(
		instanForm.kategori && !INSTAN_CATEGORIES.includes(instanForm.kategori)
			? [instanForm.kategori, ...INSTAN_CATEGORIES]
			: INSTAN_CATEGORIES
	);

	type InstanFieldSpec = {
		form: keyof typeof instanForm;
		key: string;
		label: string;
		hint: string;
		placeholder?: string;
		required?: boolean;
		widget?: 'text' | 'longtext' | 'select';
	};
	// Hanya dua field yang benar-benar wajib: nama resmi dan kategori. Sisanya
	// opsional walau skema brief menandainya required untuk hitungan progres.
	const INSTAN_FIELD_MAP: InstanFieldSpec[] = [
		{ form: 'nama', key: 'canonical_name', label: 'Nama resmi brand', hint: 'Gunakan nama resmi perusahaan, organisasi, produk, atau brand sesuai dokumen legal. Jangan digabungkan dengan alias.', placeholder: 'Ejaan resmi', required: true },
		{ form: 'alias', key: 'preferred_terms', label: 'Nama alternatif atau alias', hint: 'Cantumkan sebutan lain yang sah, seperti singkatan, nama populer, atau variasi ejaan. Pisahkan dengan titik koma.', placeholder: 'mis. GB; Giovani; Barbershop Giovani' },
		{ form: 'kategori', key: 'entity_category', label: 'Kategori brand', hint: 'Pilih satu kategori yang paling tepat mewakili entitas ini.', required: true, widget: 'select' },
		{ form: 'deskripsi', key: 'canonical_description', label: 'Deskripsi resmi', hint: 'Tuliskan satu kalimat inti, kemudian deskripsi lengkap sebagai rujukan penulisan konten.', placeholder: 'Siapa yang dibantu dan untuk apa', widget: 'longtext' },
		{ form: 'boilerplate', key: 'boilerplate', label: 'Boilerplate "Tentang brand"', hint: 'Tulis paragraf singkat siap tempel yang merangkum brand. Paragraf ini dipakai apa adanya di bagian penutup konten.', placeholder: 'mis. Giovani Barbershop melayani perawatan pria di Bandung sejak 2018.', widget: 'longtext' },
		{ form: 'fakta', key: 'official_facts', label: 'Fakta resmi', hint: 'Cantumkan tahun berdiri, lokasi, jumlah cabang, atau sertifikasi. Sertakan sumber bila tersedia dan hanya masukkan data yang dapat diverifikasi.', placeholder: 'Berdiri 2018 di Bandung, 12 cabang. Sumber: situs resmi', widget: 'longtext' },
		{ form: 'kanal', key: 'canonical_links', label: 'Kanal resmi', hint: 'Tuliskan satu URL per baris, termasuk situs web, Instagram, LinkedIn, atau marketplace. Setiap URL harus diawali http:// atau https://.', placeholder: 'https://giovani.example\nhttps://instagram.com/giovani', widget: 'longtext' },
		{ form: 'kataTerlarang', key: 'forbidden_words', label: 'Kata atau frasa yang harus dihindari', hint: 'Cantumkan istilah atau penyebutan yang tidak boleh muncul dalam artikel. Pisahkan dengan titik koma.', placeholder: 'mis. nomor 1, terbaik, termurah' },
		{ form: 'aturanKlaim', key: 'forbidden_claims', label: 'Aturan klaim dan batasan penulisan', hint: 'Tetapkan batasan agar artikel tidak memuat klaim tanpa bukti. Contoh: jangan menyebut produk sebagai nomor satu tanpa data resmi.', placeholder: 'Jangan mengklaim nomor satu tanpa data resmi', widget: 'longtext' },
		{ form: 'klaim', key: 'allowed_claims', label: 'Klaim yang boleh dipakai', hint: 'Cantumkan klaim yang sudah terbukti dan boleh ditulis, beserta buktinya. Pisahkan dengan titik koma.', placeholder: 'mis. 12 cabang di Bandung; bersertifikat halal', widget: 'longtext' },
		{ form: 'pertanyaan', key: 'target_questions', label: 'Target pertanyaan di mesin pencari AI', hint: 'Tuliskan pertanyaan yang ingin dijawab konten tentang brand ini. Ini target editorial, bukan jaminan brand muncul dalam jawaban AI.', placeholder: 'Apa rekomendasi barbershop di Bandung untuk pria?', widget: 'longtext' },
		{ form: 'keywordUrl', key: 'target_keywords_urls', label: 'Keyword prioritas dan URL target', hint: 'Tulis satu pasangan per baris: keyword, lalu URL halaman yang dituju. Dipakai untuk mengarahkan tautan di dalam konten.', placeholder: 'barbershop bandung https://giovani.example/bandung', widget: 'longtext' }
	];

	function splitUrlTokens(value: string): string[] {
		return value
			.split(/[\n\r;]+/)
			.map((v) => v.trim())
			.filter(Boolean);
	}

	function isHttpUrl(value: string): boolean {
		try {
			const u = new URL(value);
			return (u.protocol === 'http:' || u.protocol === 'https:') && Boolean(u.host);
		} catch {
			return false;
		}
	}

	// Isi form dari nilai yang sudah tersimpan, supaya reload tidak mengosongkan
	// data yang baru disimpan.
	function fillInstanForm() {
		const read = (key: string): string => {
			const v = values[key];
			if (v == null) return '';
			if (Array.isArray(v)) return v.join('; ');
			return String(v);
		};
		for (const m of INSTAN_FIELD_MAP) {
			if (m.form !== 'kategori') instanForm[m.form] = read(m.key);
		}
		const category = read('entity_category');
		if (/^lainnya\s*:/i.test(category)) {
			instanForm.kategori = 'Lainnya';
			instanForm.kategoriLain = category.slice(category.indexOf(':') + 1).trim();
		} else {
			instanForm.kategori = category;
			instanForm.kategoriLain = '';
		}
	}

	function validateInstan(): boolean {
		const next: typeof instanErrors = {};
		if (!instanForm.nama.trim()) next.nama = tr('Nama resmi brand wajib diisi.');
		if (!instanForm.kategori) next.kategori = tr('Pilih kategori brand terlebih dahulu.');
		if (instanForm.kategori === 'Lainnya' && !instanForm.kategoriLain.trim()) {
			next.kategoriLain = tr('Sebutkan kategori brand tersebut.');
		}
		const badLink = splitUrlTokens(instanForm.kanal).find((u) => !isHttpUrl(u));
		if (badLink) next.kanal = `${tr('Bukan URL yang diawali http:// atau https://')}: ${badLink}`;
		instanErrors = next;
		return Object.keys(next).length === 0;
	}

	function instanPayload(): Array<{ key: string; value: string }> {
		const known = new Set(fields.filter((f) => f.enabled).map((f) => f.key));
		const out: Array<{ key: string; value: string }> = [];
		for (const m of INSTAN_FIELD_MAP) {
			let value = instanForm[m.form].trim();
			if (m.form === 'kategori' && value === 'Lainnya' && instanForm.kategoriLain.trim()) {
				value = `Lainnya: ${instanForm.kategoriLain.trim()}`;
			}
			if (value && known.has(m.key)) out.push({ key: m.key, value });
		}
		return out;
	}

	// Simpan seluruh isian sekali jalan. Backend memvalidasi semua field dulu,
	// lalu menulisnya dalam satu transaksi (tidak ada simpan separuh).
	async function saveInstanBrief() {
		if (instanFormBusy) return;
		if (!validateInstan()) {
			showToast(tr('Periksa kolom yang ditandai.'), 'error');
			return;
		}
		const payload = instanPayload();
		if (payload.length === 0) {
			showToast(tr('Skema brief belum punya field yang cocok. Isi dulu lewat obrolan.'), 'error');
			return;
		}
		instanFormBusy = true;
		try {
			const res = await apiRequest(`${briefBase}/fields/batch`, {
				method: 'POST',
				body: JSON.stringify({ fields: payload })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan Profil Brand.')));
			await hydrate();
			showToast(tr('Profil Brand tersimpan.'));
		} catch (e) {
			// Isian tidak dikosongkan: user bisa memperbaiki dan mencoba lagi.
			showToast(e instanceof Error ? e.message : tr('Gagal menyimpan Profil Brand.'), 'error');
		} finally {
			instanFormBusy = false;
		}
	}

	// Sembunyikan bar tab utama hanya selama tab Profil Brand menempel, dan
	// munculkan lagi saat pengguna menggulir ke atas. Ambang kecil mencegah
	// kedip karena menyembunyikan bar mengubah tinggi area gulir.
	function onInstanScroll(e: Event) {
		const el = e.currentTarget as HTMLElement;
		const top = el.scrollTop;
		if (Date.now() < instanScrollLockUntil) {
			instanLastScrollTop = top;
			return;
		}
		if (top <= 8) {
			setBriefHeadHidden(false);
			instanLastScrollTop = top;
			return;
		}
		const stuck = (instanSwitchEl?.getBoundingClientRect().top ?? Infinity) <= el.getBoundingClientRect().top + 1;
		if (!stuck) {
			setBriefHeadHidden(false);
		} else if (top > instanLastScrollTop + 2) {
			setBriefHeadHidden(true);
		} else if (top < instanLastScrollTop - 6) {
			setBriefHeadHidden(false);
		}
		instanLastScrollTop = top;
	}

	$effect(() => {
		// Bar tab utama tidak boleh tetap tersembunyi saat pindah tab.
		if (activeTab !== 'copilot' || gateMode !== 'instan') {
			briefHeadHidden = false;
			instanLastScrollTop = 0;
		}
	});



	// Pasang file sebagai lampiran (chip). Yang dobel nama+ukuran diabaikan,
	// yang melebihi 10 MB ditolak dengan menyebut namanya. `kind` membatasi
	// jalur: 'image' (popover Gambar) hanya menerima gambar, 'doc' (popover
	// Dokumen) hanya pdf/docx/txt/md/csv — picker bisa dipaksa "All Files",
	// validasi ini yang menjamin pemisahannya.
	//
	// Guard kuota percakapan (backend tetap penentu akhir): maks 5 berkas dan
	// 25 MB per brief untuk lampiran chat.
	const CHAT_MAX_FILES = 5;
	const CHAT_MAX_BYTES = 25 * 1024 * 1024;
	const DOC_EXTS = new Set(['pdf', 'docx', 'txt', 'md', 'csv']);
	function isDocFile(f: File): boolean {
		if (f.type === 'application/pdf') return true;
		const ext = f.name.split('.').pop()?.toLowerCase() ?? '';
		return DOC_EXTS.has(ext);
	}

	function stageFiles(list: FileList | File[] | null | undefined, kind?: 'image' | 'doc') {
		if (!list) return;
		const rejected: string[] = [];
		const wrongKind: string[] = [];
		const acceptedFiles: File[] = [];
		for (const file of Array.from(list)) {
			if (file.size > MAX_UPLOAD_BYTES) {
				rejected.push(`${file.name} (${fmtSize(file.size)})`);
				continue;
			}
			if (kind === 'image' && !isImageFile(file)) {
				wrongKind.push(file.name);
				continue;
			}
			if (kind === 'doc' && !isDocFile(file)) {
				wrongKind.push(file.name);
				continue;
			}
			acceptedFiles.push(file);
		}
		// Sisa slot & sisa porsi ukuran percakapan.
		const slot = Math.max(0, CHAT_MAX_FILES - pendingDocs.length);
		let stagedBytes = pendingDocs.reduce((n, d) => n + d.file.size, 0);
		const overflow: string[] = [];
		const staged: File[] = [];
		for (const file of acceptedFiles) {
			const dup = pendingDocs.some((d) => d.file.name === file.name && d.file.size === file.size);
			if (dup) continue;
			if (staged.length >= slot || stagedBytes + file.size > CHAT_MAX_BYTES) {
				overflow.push(file.name);
				continue;
			}
			staged.push(file);
			stagedBytes += file.size;
		}
		if (staged.length > 0) pendingDocs = [...pendingDocs, ...staged.map((file) => ({ file }))];
		const problems: string[] = [];
		if (rejected.length > 0) problems.push(`${tr('Melebihi 10 MB, tidak dilampirkan')}: ${rejected.join(', ')}`);
		if (wrongKind.length > 0) {
			const label = kind === 'image' ? tr('Bukan gambar, tidak dilampirkan') : tr('Bukan dokumen (pdf, docx, txt, md), tidak dilampirkan');
			problems.push(`${label}: ${wrongKind.join(', ')}`);
		}
		if (overflow.length > 0) {
			problems.push(
				`${tr(`Maksimal ${CHAT_MAX_FILES} berkas / 25 MB lampiran per brief, tidak dilampirkan`)}: ${overflow.join(', ')}`
			);
		}
		if (problems.length > 0) streamError = problems.join(' · ');
	}

	function hasFileDrag(e: DragEvent): boolean {
		return Array.from(e.dataTransfer?.types ?? []).includes('Files');
	}
	function onChatDragEnter(e: DragEvent) {
		// Gerbang 2 (instan) punya dropzone sendiri di panenya — jangan
		// memicu overlay/penanganan composer chat yang tak tampil.
		if (activeTab !== 'copilot' || gateMode !== 'chat' || !hasFileDrag(e)) return;
		dragDepth += 1;
		draggingFiles = true;
	}
	function onChatDragLeave() {
		if (activeTab !== 'copilot') return;
		dragDepth = Math.max(0, dragDepth - 1);
		if (dragDepth === 0) draggingFiles = false;
	}
	function onChatDrop(e: DragEvent) {
		if (activeTab !== 'copilot') return;
		e.preventDefault();
		dragDepth = 0;
		draggingFiles = false;
		stageFiles(e.dataTransfer?.files);
	}

	// ---------- widget ----------

	// Pilihan baku untuk field tertentu. Sisanya cukup dijawab lewat composer.
	const ENUM_CHOICES: Record<string, string[]> = {
		tone: ['formal', 'santai', 'teknis', 'persuasif', 'jurnalistik'],
		formality: ['sangat formal', 'formal', 'netral', 'santai'],
		greeting: ['kamu', 'Anda', 'Kak', 'Sobat'],
		point_of_view: ['orang pertama (kami)', 'orang pertama (aku)', 'orang ketiga'],
		language: ['Indonesia', 'English'],
		search_intent: ['informational', 'commercial', 'transactional', 'navigational']
	};
	const LIST_KEYS = new Set(['preferred_terms', 'forbidden_terms']);
	const BOOL_KEYS = new Set(['use_tables', 'use_lists', 'use_faq', 'use_examples']);

	const activeField = $derived.by(() => {
		if (!askedField) return null;
		return fields.find((f) => f.key === askedField) ?? null;
	});

	const lapis2 = $derived(bySection(2));

	// Persentase ringkas untuk lencana di rel Ringkasan dan panelnya.
	const progressPct = $derived(progress.total ? Math.round((progress.filled / progress.total) * 100) : 0);

	// Persen progres per brief: brief aktif memakai angka hidup (progressPct,
	// ikut naik saat chat mengisi field), brief lain memakai angka dari daftar.
	const briefPct = (b: BriefListItem) =>
		b.id === profileId ? progressPct : (b.progress?.pct ?? 0);

	// Field opsional (layer 2) yang masih kosong. Progres "field wajib" hanya
	// menghitung field WAJIB, jadi 100% bukan berarti semua yang tampak di tab
	// Entitas sudah terisi — mis. Keunggulan/USP dan Fakta penting. Angka ini
	// dipakai untuk memberi tahu jujur + jalan pintas melengkapinya.
	const optionalEmpty = $derived(
		fields.filter((f) => f.enabled && !f.required && f.layer !== 3 && !isFilled(f.key))
	);

	// Widget hanya bila field-nya memang butuh kontrol khusus. Pertanyaan biasa
	// cukup dijawab di composer, jadi tidak ada kartu yang menganggu.
	const showWidget = $derived.by(() => {
		const f = activeField;
		if (!f) return null as null | 'enum' | 'list' | 'bool';
		if (ENUM_CHOICES[f.key]) return 'enum';
		if (LIST_KEYS.has(f.key)) return 'list';
		if (BOOL_KEYS.has(f.key)) return 'bool';
		return null;
	});

	function widgetLabel(f: FieldMeta) {
		return f.label || f.key;
	}

	async function commitWidget(key: string, value: string) {
		if (!value.trim()) return;
		const f = fields.find((x) => x.key === key);
		widgetValue = '';
		try {
			const res = await apiRequest(`${briefBase}/fields`, {
				method: 'POST',
				body: JSON.stringify({ key, value, status: 'confirmed', source: 'user' })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan pilihan.')));
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal menyimpan pilihan.');
			return;
		}
		await send(`${widgetLabel(f ?? ({ key } as FieldMeta))}: ${value}`);
	}

	// ---------- panel Ringkasan ----------

	// Seksi → kolom penyimpanan. Fungsi kedua untuk seksi baru: isi "column"
	// di skema field (lihat Skema Field di /admin/taut-brief).
	const SECTION_COLUMN: Record<string, string> = {
		'Entity Core': 'entity_core',
		'Identitas Brand': 'identity',
		'Gaya Penulisan': 'style',
		'Pedoman Konten & SEO': 'seo'
	};

	function bySection(layer: number) {
		const groups: Array<{ section: string; items: FieldMeta[] }> = [];
		for (const f of fields.filter((x) => x.layer === layer)) {
			const g = groups.find((x) => x.section === f.section);
			if (g) g.items.push(f);
			else groups.push({ section: f.section, items: [f] });
		}
		return groups;
	}

	// Status tiap field untuk panel: yang kosong dibaca "diisi AI saat dibahas"
	// — bukan "kosong", karena pengisian memang kerja AI lewat percakapan.
	function isFilled(key: string): boolean {
		const st = fieldStates[key];
		return Boolean(st && (st.status === 'filled' || st.status === 'confirmed'));
	}

	function sourceLabel(src: string): string {
		switch (src) {
			case 'url': return tr('dari website');
			case 'document': return tr('dari dokumen');
			case 'ai_suggested': return tr('usulan AI');
			default: return tr('darimu');
		}
	}

	function stateLabel(key: string): string {
		const st = fieldStates[key];
		if (!st || st.status === 'empty') return tr('diisi AI saat dibahas');
		const src = sourceLabel(st.source);
		return st.status === 'confirmed' ? `${tr('dikonfirmasi')} · ${src}` : `${tr('terisi')} · ${src}`;
	}

	async function saveRingkasan() {
		const changed = fields.filter((f) => (ringkasan[f.key] ?? '') !== '' && (ringkasan[f.key] ?? '') !== (values[f.key] ?? ''));
		// Field yang dikosongkan juga ikut disimpan (pengguna boleh menghapus isi).
		const touched = fields.filter((f: FieldMeta) => (ringkasan[f.key] ?? '') !== (values[f.key] ?? ''));
		void changed;
		if (touched.length === 0) {
			showToast(tr('Belum ada perubahan.'));
			return;
		}
		saving = true;
		try {
			const payload: Record<string, unknown> = {};
			const confirmedKeys: Array<{ key: string; value: string }> = [];
			for (const f of touched) {
				const col = SECTION_COLUMN[f.section];
				if (!col) continue;
				const v = (ringkasan[f.key] ?? '').trim();
				if (!payload[col]) payload[col] = { ...(values[col] as any) };
				(payload[col] as Record<string, unknown>)[f.key] = v;
				confirmedKeys.push({ key: f.key, value: v });
			}
			if (Object.keys(payload).length === 0) {
				showToast(tr('Bagian ini belum bisa disimpan. Coba lagi nanti.'), 'error');
				return;
			}
			const res = await apiRequest(`${apiBase}/profiles/${profileId}`, {
				method: 'PUT',
				body: JSON.stringify(payload)
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan koreksi.')));
			for (const item of confirmedKeys) {
				if (!item.value) continue;
				void apiRequest(`${briefBase}/fields`, {
					method: 'POST',
					body: JSON.stringify({ key: item.key, value: item.value, status: 'confirmed', source: 'user' })
				});
			}
			showToast(tr('Koreksi tersimpan.'));
			await hydrate();
		} catch (e) {
			showToast(e instanceof Error ? e.message : tr('Gagal menyimpan koreksi.'), 'error');
		} finally {
			saving = false;
		}
	}

	let workingState = $state(false);
	function working(v: boolean) {
		workingState = v;
	}

</script>

<!-- Judul tab memakai nama brand, bukan nama brief: nama brief hanyalah
     "Brief {n}" hasil penomoran saat dibuat, jadi tidak bermakna bagi pembaca. -->
<PageTitle title={`${brandName || tr('Profiler')} · Taut Brief`} />

<!-- Blok kerja diukur tingginya (lihat measureWorkspace) supaya mengisi tepat
	ruang antara bar atas dan dasar jendela: compact, tanpa scroll badan. -->
<div
	bind:this={shellEl}
	class="relative flex overflow-hidden"
	style="height: calc(100dvh - {shellTop}px); margin: -32px -28px -64px;"
>

	{#if sidebarOpen}
		<!-- Backdrop drawer brief; pola yang sama dengan modal katalog media. -->
		<div
			class="fixed inset-0 z-30"
			style="background-color: rgba(15,23,42,.45);"
			role="presentation"
			onclick={() => (sidebarOpen = false)}
		></div>
	{/if}

	<!-- ── Drawer riwayat brief (fungsi lama, kini panel geser) ─────── -->
	<aside
		bind:this={railEl}
		class="brief-rail fixed inset-y-0 left-0 z-40 flex flex-col border-r shadow-xl transition-transform"
		class:dragging-rail={draggingRail}
		style="border-color: var(--border); top: 64px; height: calc(100dvh - 64px); width: {sidebarWidth}px;"
		class:translate-x-0={sidebarOpen}
		class:-translate-x-full={!sidebarOpen}
	>
		<div class="flex h-full flex-col">
			<!-- Baris atas: ciri di kiri; cari di kanan. -->
			<div class="flex items-center justify-between px-3 pt-3 pb-1">
				<span class="brief-mark" aria-hidden="true"><Sparkle size={15} /></span>
				<div class="flex items-center gap-1">
					<button
						class="brief-icon-btn"
						class:active={searchOpen}
						type="button"
						aria-label={tr('Cari brief')}
						onclick={() => (searchOpen = !searchOpen)}
					>
						<Search size={16} />
					</button>
				</div>
			</div>

			{#if searchOpen}
				<div class="px-3 pb-1">
					<label class="flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs" style="border-color: var(--border); background: var(--card);">
						<Search size={12} class="text-muted-foreground" />
						<input class="w-full bg-transparent outline-none" placeholder={tr('Cari brief…')} bind:value={briefSearch} />
					</label>
				</div>
			{/if}

			<!-- Aksi utama, menonjol seperti baris "Obrolan" di referensi. -->
			<div class="px-3 pt-2">
				<button class="brief-primary" type="button" disabled={workingState} onclick={() => void createBrief()}>
					<SquarePen size={16} />
					<span>{tr('Brief baru')}</span>
				</button>
			</div>

			<p class="px-4 pt-4 pb-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">{tr('Brief')}</p>

			<!-- Riwayat: satu daftar polos, terbaru dulu, berapapun lamanya. -->
			<nav class="min-h-0 flex-1 overflow-y-auto">
				{#if visibleBriefs().length === 0}
					<p class="px-3 py-3 text-xs text-muted-foreground">{tr('Belum ada brief.')}</p>
				{/if}
				{#each visibleBriefs() as b (b.id)}
					<div
						class="brief-item"
						class:brief-item-open={menuId === b.id}
						aria-current={b.id === profileId ? 'page' : undefined}
					>
						{#if renameId === b.id}
							<input
								class="app-input min-h-6 min-w-0 flex-1 !px-1.5 !py-0.5 text-[13px]"
								bind:value={renameValue}
								onkeydown={(e) => {
									if (e.key === 'Enter') { e.preventDefault(); void saveRename(b.id); }
									if (e.key === 'Escape') renameId = null;
								}}
							/>
							<button class="brief-more" type="button" aria-label={tr('Simpan nama')} onclick={() => void saveRename(b.id)}>✓</button>
						{:else}
							<a class="min-w-0 flex-1 truncate" href={`/klien/${brandUrl}/profiler/${b.short_code || b.id}`}>
								<span class="brief-item-title block truncate text-[13px] font-medium">{b.name}</span>
								{#if b.description}
									<span class="block truncate text-[11px] text-muted-foreground">{b.description}</span>
								{/if}
							</a>
							<span class="brief-pct">{briefPct(b)}%</span>
							<button
								class="brief-pin"
								class:pinned={b.pinned}
								type="button"
								aria-pressed={b.pinned}
								aria-label={b.pinned ? tr('Lepas sematan') : tr('Sematkan brief')}
								onclick={(e) => { e.preventDefault(); void togglePin(b.id, b.pinned); }}
							>
								<Pin size={13} />
							</button>
							<button
								class="brief-more"
								type="button"
								aria-label={tr('Aksi brief')}
								onclick={(e) => { e.preventDefault(); toggleMenu(b.id); }}
							>
								<MoreHorizontal size={14} />
							</button>
						{/if}

						{#if menuId === b.id}
							<div class="brief-menu">
								<button type="button" onclick={() => startRename(b.id, b.name)}>
									<Pencil size={13} />{tr('Ubah nama')}
								</button>
								<button type="button" onclick={() => void duplicateBrief(b.id)}>
									<Copy size={13} />{tr('Duplikat')}
								</button>
								<button type="button" class="brief-menu-danger" onclick={() => requestDelete(b)}>
									<Trash2 size={13} />{tr('Hapus')}
								</button>
							</div>
						{/if}
					</div>
				{/each}
				</nav>

				<div class="border-t px-3 py-3" style="border-color: var(--border);">
					<button
						class="brief-summary w-full"
						type="button"
						aria-label={tr('Buka panel Ringkasan')}
						onclick={() => { sidebarOpen = false; activeTab = 'entitas'; }}
					>
						<span class="flex items-center justify-between gap-2 text-[13px] font-medium">
							{tr('Ringkasan')}
							<PanelRight size={14} />
						</span>
						<span class="mt-1.5 block h-1 w-full overflow-hidden rounded-full" style="background: color-mix(in srgb, var(--foreground) 10%, transparent);">
							<span class="block h-full" style="width: {progressPct}%; background-color: var(--primary);"></span>
						</span>
						<span class="mt-1 block text-[11px] opacity-80">
							{progress.filled} / {progress.total} {tr('field wajib terisi')}
						</span>
					</button>
				</div>
			</div>

			<div
				class="brief-rail-handle"
				role="separator"
				aria-orientation="vertical"
				onpointerdown={(e) => startRailDrag(e)}
			></div>
		</aside>

		{#if sidebarOpen}
			<div class="fixed inset-0 z-30" style="background-color: rgba(15,23,42,.45);" role="presentation" onclick={() => (sidebarOpen = false)}></div>
		{/if}

		{#if menuId}
			<div class="fixed inset-0 z-20" role="presentation" onclick={() => (menuId = null)}></div>
		{/if}

		<main
		class="relative flex min-w-0 flex-1 flex-col"
		ondragenter={onChatDragEnter}
		ondragleave={onChatDragLeave}
		ondragover={(e) => {
			if (hasFileDrag(e)) e.preventDefault();
		}}
		ondrop={onChatDrop}
	>
		<!-- Header band (desain GEO): judul + aksi, lalu baris tab + status. -->
		<section
			class="flex-shrink-0 border-b bg-card"
			style="border-color: {briefHeadHidden ? 'transparent' : 'var(--border)'};"
		>
			<!-- Seluruh kepala workspace (judul + bar tab) ikut melipat saat tab
			     Profil Brand menempel, supaya tidak memakan tinggi layar. -->
			<div
				class="brief-tabs-collapse"
				class:is-collapsed={briefHeadHidden}
				class:tabs-pop-open={betaOpen}
			>
			<div class="brief-head-block">
			<div class="flex flex-col gap-3 px-4 pt-4 pb-3 md:px-6 xl:flex-row xl:items-start xl:justify-between">
				<div class="min-w-0">
					<div class="flex min-w-0 items-center gap-2.5">
						<h1 class="truncate text-2xl font-semibold tracking-tight">{tr('Brand Brief & Entity Builder')}</h1>
						<span class="flex-shrink-0 rounded border px-2 py-0.5 text-[10px] font-bold" style="background: var(--accent); border-color: color-mix(in srgb, var(--primary) 25%, transparent); color: var(--accent-foreground);">Beta</span>
					</div>
					{#if !headerCollapsed}
						<p class="mt-1 max-w-3xl text-sm text-muted-foreground">
							{tr('Kumpulkan identitas dan fakta resmi brand-mu lewat obrolan, atau tempel URL situsnya. AI membaca sumber itu lalu menyusunnya jadi data yang dipakai saat menulis artikel.')}
						</p>
					{/if}
				</div>
				<div class="brief-head-actions flex flex-shrink-0 flex-wrap items-center gap-2">
					{#if activeTab !== 'media' && activeTab !== 'ringkasan'}
					{#if headerCollapsed}
						<button
							class="app-btn app-btn-soft !min-h-8 !min-w-8 !p-1"
							type="button"
							aria-label={tr('Simpan Draf')}
							title={tr('Simpan Draf')}
							onclick={() => void saveRingkasan()}
						>
							<Save size={14} />
						</button>
						<button
							class="app-btn app-btn-primary !min-h-8 !min-w-8 !p-1"
							type="button"
							aria-label={tr('Gunakan untuk Kampanye')}
							title={tr('Gunakan untuk Kampanye')}
							onclick={() => void goto(`/klien/${brandUrl}/campaigns`)}
						>
							<svg xmlns="http://www.w3.org/2000/svg" width="1em" height="1em" viewBox="0 0 16 16" aria-hidden="true">
								<path d="M0 0h16v16H0z" fill="none" />
								<path fill="currentColor" d="M13 2.5a1.5 1.5 0 0 1 3 0v11a1.5 1.5 0 0 1-3 0v-.214c-2.162-1.241-4.49-1.843-6.912-2.083l.405 2.712A1 1 0 0 1 5.51 15.1h-.548a1 1 0 0 1-.916-.599l-1.85-3.49l-.202-.003A2.014 2.014 0 0 1 0 9V7a2.02 2.02 0 0 1 1.992-2.013a75 75 0 0 0 2.483-.075c3.043-.154 6.148-.849 8.525-2.199zm1 0v11a.5.5 0 0 0 1 0v-11a.5.5 0 0 0-1 0m-1 1.35c-2.344 1.205-5.209 1.842-8 2.033v4.233q.27.015.537.036c2.568.189 5.093.744 7.463 1.993zm-9 6.215v-4.13a95 95 0 0 1-1.992.052A1.02 1.02 0 0 0 1 7v2c0 .55.448 1.002 1.006 1.009A61 61 0 0 1 4 10.065m-.657.975l1.609 3.037l.01.024h.548l-.002-.014l-.443-2.966a68 68 0 0 0-1.722-.082z" />
							</svg>
						</button>
					{:else}
						<button class="app-btn app-btn-soft !min-h-8 !text-xs" type="button" onclick={() => void saveRingkasan()}>{tr('Simpan Draf')}</button>
						<button class="app-btn app-btn-primary !min-h-8 !text-xs" type="button" onclick={() => void goto(`/klien/${brandUrl}/campaigns`)}>
							{tr('Gunakan untuk Kampanye')}<ArrowRight size={14} />
						</button>
					{/if}
					{/if}
					<button
						class="app-btn app-btn-soft !min-h-8 !min-w-8 !p-1"
						type="button"
						aria-label={headerCollapsed ? tr('Perluas header') : tr('Ciutkan header')}
						aria-expanded={!headerCollapsed}
						title={headerCollapsed ? tr('Perluas header') : tr('Ciutkan header')}
						onclick={() => (headerCollapsed = !headerCollapsed)}
					>
						<svg xmlns="http://www.w3.org/2000/svg" width="1em" height="1em" viewBox="0 0 16 16" aria-hidden="true">
							<path d="M0 0h16v16H0z" fill="none" />
							<path fill="currentColor" fill-rule="evenodd" d="M3.646 9.146a.5.5 0 0 1 .708 0L8 12.793l3.646-3.647a.5.5 0 0 1 .708.708l-4 4a.5.5 0 0 1-.708 0l-4-4a.5.5 0 0 1 0-.708m0-2.292a.5.5 0 0 0 .708 0L8 3.207l3.646 3.647a.5.5 0 0 0 .708-.708l-4-4a.5.5 0 0 0-.708 0l-4 4a.5.5 0 0 0 0 .708" />
						</svg>
					</button>
				</div>
			</div>

				<div
					class="brief-tabs-row flex flex-wrap items-center justify-between gap-2 border-t px-4 py-2 md:px-6"
					style="border-color: var(--border);"
				>
				<div class="flex flex-wrap items-center gap-1.5" role="tablist">
					<button
						class="brief-tab"
						class:brief-tab-active={activeTab === 'copilot'}
						type="button"
						role="tab"
						aria-selected={activeTab === 'copilot'}
						onclick={() => (activeTab = 'copilot')}
					>
						{#if gateMode === 'instan'}
							<Upload size={14} />{tr('Instan Brief')}
						{:else}
							<Bot size={14} />{tr('Copilot Chat')}
						{/if}
						{#if activeTab === 'copilot'}<span class="h-2 w-2 animate-pulse rounded-full" style="background: var(--primary);"></span>{/if}
					</button>
					<button
						class="brief-tab"
						class:brief-tab-active={activeTab === 'entitas'}
						type="button"
						role="tab"
						aria-selected={activeTab === 'entitas'}
						onclick={() => (activeTab = 'entitas')}
					>
						<FileText size={14} />{tr('Entitas')}
						<span
							class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold"
							style="background: {progressPct >= 100 ? 'var(--success-soft)' : 'var(--warning-soft)'}; color: {progressPct >= 100 ? 'var(--success)' : 'var(--warning)'};"
						>
							<span class="h-1.5 w-1.5 rounded-full" style="background: {progressPct >= 100 ? 'var(--success)' : 'var(--warning)'};"></span>{progressPct}%
						</span>
					</button>
					<button
						class="brief-tab"
						class:brief-tab-active={activeTab === 'media'}
						type="button"
						role="tab"
						aria-selected={activeTab === 'media'}
						onclick={() => (activeTab = 'media')}
					>
						<Images size={14} />{tr('Media')}
						{#if mediaImages.length + mediaDocs.length > 0}
							<span class="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-bold" style="background: var(--secondary); color: var(--muted-foreground);">
								{mediaImages.length + mediaDocs.length}
							</span>
						{/if}
					</button>
					<button
						class="brief-tab"
						class:brief-tab-active={activeTab === 'timeline'}
						type="button"
						role="tab"
						aria-selected={activeTab === 'timeline'}
						onclick={() => (activeTab = 'timeline')}
					>
						<Newspaper size={14} />{tr('Timeline')}
						{#if timeline.length > 0}
							<span class="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-bold" style="background: var(--secondary); color: var(--muted-foreground);">
								{timeline.length}
							</span>
						{/if}
					</button>
					<button
						class="brief-tab"
						class:brief-tab-active={activeTab === 'ringkasan'}
						type="button"
						role="tab"
						aria-selected={activeTab === 'ringkasan'}
						onclick={() => (activeTab = 'ringkasan')}
					>
						<Sparkle size={14} />{tr('Ringkasan')}
						{#if summaries.some((s) => s.status === 'queued' || s.status === 'running')}
							<span class="h-1.5 w-1.5 animate-pulse rounded-full" style="background: var(--warning);"></span>
						{/if}
					</button>
					</div>
					<div class="flex items-center gap-3 text-[11px] text-muted-foreground">
							<div class="relative flex items-center">
							<button
								class="brief-beta-chip"
								class:active={betaOpen}
								type="button"
								aria-expanded={betaOpen}
								aria-label={tr('Tentang status Beta')}
								onclick={() => (betaOpen = !betaOpen)}
							>
								{tr('Beta')}
							</button>
						{#if betaOpen}
							<button class="brief-beta-backdrop" type="button" aria-label={tr('Tutup')} onclick={() => (betaOpen = false)}></button>
							<div
								class="brief-beta-pop"
								role="dialog"
								aria-label={tr('Tentang fitur Beta')}
								tabindex="-1"
								onkeydown={(e) => {
									if (e.key === 'Escape') betaOpen = false;
								}}
							>
								<p class="brief-beta-pop-title">{tr('Fitur Beta')}</p>
								<p class="brief-beta-pop-text">
									{tr('Fitur ini masih dalam tahap pengembangan tim Tauthub. Isi dan tampilannya bisa berubah, dan mungkin masih ada bagian yang belum sempurna.')}
								</p>
							</div>
						{/if}
						</div>
						<span>{tr('Sinkronisasi Terakhir')}: {syncText()}</span>
					</div>
				</div>
			</div>
			</div>
			</section>

	{#snippet instanPane()}
			<!-- Gerbang 2: isi template → unggah berkas → analisis. Reuse
			     uploadMediaFiles/loadMedia (tab Media) supaya sinkron otomatis. -->
			<div
				class="instan-scroll min-h-0 flex-1 overflow-y-auto"
				bind:this={instanScrollEl}
				onscroll={onInstanScroll}
			>
				<div class="instan-page">
					<header class="instan-head">
						<h2>{tr('Template Brief')}</h2>
						<p class="instan-head-desc">{tr('Lengkapi informasi brand pada formulir di bawah ini, atau unduh template untuk diisi secara offline. Data yang tersimpan menjadi acuan penulisan artikel.')}</p>
					</header>

					<div class="instan-grid">
						<div class="instan-main">
							<section class="instan-card">
								<div class="instan-card-head">
									<div class="min-w-0">
										<h3><span class="instan-step">1</span>{tr('Profil Brand')}</h3>
										<p class="instan-lead">{tr('Informasi resmi yang menjadi acuan Tauthub saat menyusun konten untuk brand ini.')}</p>
									</div>
								</div>

								<div class="instan-switch-sticky" bind:this={instanSwitchEl}>
								<div class="instan-switch" role="tablist" aria-label={tr('Pilih cara mengisi profil brand')}>
									<button
										type="button"
										role="tab"
										id="instan-tab-field"
										aria-selected={instanPath === 'field'}
										aria-controls="instan-panel-field"
										onclick={() => (instanPath = 'field')}
									>{tr('Isi field langsung')}</button>
									<button
										type="button"
										role="tab"
										id="instan-tab-template"
										aria-selected={instanPath === 'template'}
										aria-controls="instan-panel-template"
										onclick={() => (instanPath = 'template')}
									>{tr('Template DOCX')}</button>
								</div>
								</div>

								{#if instanPath === 'field'}
								<div id="instan-panel-field" role="tabpanel" aria-labelledby="instan-tab-field">
								<form
									class="instan-form"
									onsubmit={(e) => { e.preventDefault(); void saveInstanBrief(); }}
								>
									{#each INSTAN_FIELD_MAP as m (m.key)}
										<BriefFieldInput
											keyName={m.key}
											label={tr(m.label)}
											hint={tr(m.hint)}
											widget={m.widget ?? 'text'}
											options={m.form === 'kategori' ? instanCategoryOptions : []}
											required={Boolean(m.required)}
											placeholder={m.placeholder ? tr(m.placeholder) : ''}
											error={instanErrors[m.form]}
											bind:value={instanForm[m.form]}
										/>
										{#if m.form === 'kategori' && instanForm.kategori === 'Lainnya'}
											<BriefFieldInput
												keyName="entity_category_custom"
												label={tr('Kategori lainnya')}
												hint={tr('Sebutkan kategori brand tersebut.')}
												placeholder={tr('mis. komunitas, yayasan, koperasi')}
												required
												error={instanErrors.kategoriLain}
												bind:value={instanForm.kategoriLain}
											/>
										{/if}
									{/each}
									<div class="instan-form-actions">
										<button class="app-btn app-btn-primary instan-btn" type="submit" disabled={instanFormBusy}>
											{instanFormBusy ? tr('Menyimpan…') : tr('Simpan ke Brief')}
										</button>
									</div>
									<p class="instan-hint">{tr('Kolom yang dikosongkan akan dilewati. Data disimpan langsung ke profil brand tanpa AI dan tanpa berkas.')}</p>
								</form>
								</div>
								{:else}
								<div
									class="instan-template"
									id="instan-panel-template"
									role="tabpanel"
									aria-labelledby="instan-tab-template"
								>
									<p class="instan-template-lead">{tr('Unduh template, lengkapi di Word, lalu unggah kembali pada langkah 2. Isi template dibaca tanpa AI.')}</p>
									<p class="instan-hint">{tr('Template mencakup nama brand, alias, kategori, deskripsi, fakta resmi, kanal resmi, istilah yang dihindari, aturan klaim, dan target pertanyaan.')}</p>
									<a class="app-btn app-btn-primary instan-btn" href="/templates/brief-brand.docx" download>
										<FileDown size={14} />{tr('Unduh Template DOCX')}
									</a>
								</div>
								{/if}

								{#if instanPath === 'template'}
								<p class="instan-hint">
									{tr('Template lain:')}
									<a class="instan-link" href="/templates/brief-produk.docx" download>{tr('Produk & Layanan')}</a>
									<span class="instan-sep">·</span>
									<a class="instan-link" href="/templates/brief-kampanye.docx" download>{tr('Kebutuhan Kampanye')}</a>
									</p>
								{/if}
								</section>

								{#if instanPath === 'template'}
								<section class="instan-card">
										<h3><span class="instan-step">2</span>{tr('Unggah berkas brand di sini')}</h3>
									<div
										class="instan-drop"
										class:instan-drag={mediaDragging}
										role="button"
										tabindex="0"
										aria-label={tr('Unggah berkas brand')}
										onclick={() => document.getElementById('brief-instan-input')?.click()}
										onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); document.getElementById('brief-instan-input')?.click(); } }}
										ondragover={(e) => onMediaDragEnter(e)}
										ondragenter={(e) => onMediaDragEnter(e)}
										ondragleave={(e) => onMediaDragLeave(e)}
										ondrop={(e) => onMediaDrop(e)}
									>
										<span class="instan-drop-ico"><Upload size={18} /></span>
										<p>{mediaUploading ? tr('Mengunggah…') : tr('Seret berkas ke sini atau klik untuk mengunggah')}</p>
										<p class="instan-drop-note">
											{tr('PDF, DOCX, TXT, MD, CSV · Gambar JPG, PNG, WebP · maksimal 10 MB per berkas')}
										</p>
										<input
											id="brief-instan-input"
											class="hidden"
											type="file"
											multiple
											accept=".pdf,.docx,.txt,.md,.csv,application/pdf,image/png,image/jpeg,image/webp,image/gif"
											onclick={(e) => e.stopPropagation()}
											onchange={(e) => {
												const input = e.currentTarget as HTMLInputElement;
												void uploadMediaFiles(input.files);
												input.value = '';
											}}
										/>
									</div>
									{#if mediaError}
										<p class="mt-3 rounded-lg p-2 text-xs" style="background: var(--warning-soft); color: var(--warning);">{mediaError}</p>
									{/if}

									<h4 class="instan-uploads-title">
										{tr('Berkas yang diunggah')} ({mediaDocs.length + mediaImages.length})
									</h4>
									<div class="instan-uploads">
										{#if mediaDocs.length + mediaImages.length === 0}
											<div class="instan-empty">
												<span class="instan-ico instan-ico-muted"><FileText size={16} /></span>
												<div class="min-w-0">
													<p class="instan-empty-title">{tr('Belum ada berkas unggahan.')}</p>
													<p class="instan-empty-note">{tr('Semua berkas yang diunggah di sini akan tampil juga di tab Media.')}</p>
												</div>
											</div>
										{:else}
											{#each mediaDocs as doc (doc.id)}
												<a class="instan-file" href={doc.url} target="_blank" rel="noreferrer">
													<FileText size={13} />
													<span class="truncate">{doc.original_filename}</span>
													{#if doc.size_bytes}<span class="instan-file-size">{fmtSize(doc.size_bytes)}</span>{/if}
													{#if doc.parse_status === 'ready'}
														<span class="instan-badge" style="background: var(--success-soft); color: var(--success);">{tr('Terbaca AI')}</span>
													{:else if doc.parse_status === 'failed'}
														<span class="instan-badge" style="background: var(--warning-soft); color: var(--warning);" title={doc.parse_error ?? ''}>{tr('Gagal dibaca')}</span>
													{/if}
													{#if doc.file_deleted_at}
														<span class="instan-badge" style="background: var(--secondary); color: var(--muted-foreground);">{tr('Berkas dibersihkan')}</span>
													{/if}
												</a>
											{/each}
											{#each mediaImages as im (im.id)}
												<a class="instan-file" href={im.url} target="_blank" rel="noreferrer">
													<Images size={13} />
													<span class="truncate">{im.file_name || tr('Gambar')}</span>
													{#if im.size_bytes}<span class="instan-file-size">{fmtSize(im.size_bytes)}</span>{/if}
												</a>
											{/each}
										{/if}
									</div>
									{#if mediaDocs.length + mediaImages.length > 0}
										<p class="instan-hint">
											{tr('Daftar lengkap + hapus berkas ada di tab')}
											<button class="instan-link" type="button" onclick={() => (activeTab = 'media')}>{tr('Media')}</button>.
										</p>
										{/if}
								</section>
								{/if}

								<section class="instan-card">
									<h3><span class="instan-step">{instanPath === 'field' ? '2' : '3'}</span>{tr('Analisis & ringkasan')}</h3>
									<p class="instan-lead">
										{instanPath === 'field'
											? tr('Field yang tersimpan langsung dapat dipakai Ringkasan dan konteks artikel.')
											: tr('Setelah berkas bertanda "Terbaca AI", Ringkasan mengubahnya menjadi data brand yang dapat Anda koreksi. Dokumen dan gambar ini juga menjadi bahan artikel saat Copilot aktif.')}
									</p>
								<div class="instan-cta">
									<button class="app-btn app-btn-primary instan-btn" type="button" onclick={() => (activeTab = 'ringkasan')}>
										{tr('Lanjut ke Ringkasan')}<ArrowRight size={14} />
									</button>
									<button class="app-btn app-btn-soft instan-btn" type="button" onclick={() => (activeTab = 'entitas')}>
										{tr('Lihat data brand')}
									</button>
								</div>
								{#if sources.length > 0}
									<p class="instan-sources">{tr('Sumber yang sudah terbaca')}: {sources.length}</p>
								{/if}
							</section>
						</div>

						<aside class="instan-side">
							<!-- Disclosure: apa yang terjadi dengan data yang diunggah/diisi.
							     Ditulis apa adanya, termasuk keterbatasan (log/backup). -->
							<section class="instan-card">
								<h3 class="instan-side-title"><ShieldCheck size={15} />{tr('Penanganan data Anda')}</h3>
								<div class="instan-facts">
									<div class="instan-fact">
										<span class="instan-ico instan-ico-sm"><FileCheck size={14} /></span>
										<div class="min-w-0">
											<p class="instan-fact-title">{tr('Isian yang Anda simpan')}</p>
											<p class="instan-fact-desc">{tr('Isian disimpan apa adanya ke data brand. Langkah ini tidak memakai AI dan tidak membuat berkas.')}</p>
										</div>
									</div>
									{#if instanPath === 'template'}
									<div class="instan-fact">
										<span class="instan-ico instan-ico-sm"><FileSearch size={14} /></span>
										<div class="min-w-0">
											<p class="instan-fact-title">{tr('Membaca dokumen')}</p>
											<p class="instan-fact-desc">{tr('Tauthub membaca sendiri isi dokumen PDF, DOCX, dan TXT. Proses ini tidak memakai AI.')}</p>
										</div>
									</div>
									{/if}
									<div class="instan-fact">
										<span class="instan-ico instan-ico-sm"><FileCheck size={14} /></span>
										<div class="min-w-0">
											<p class="instan-fact-title">{tr('Membuat ringkasan')}</p>
											<p class="instan-fact-desc">{tr('Ringkasan hanya dibuat saat Anda menekan tombolnya. Hanya langkah itu yang mengirim teks ke AI, sesuai pengaturan platform.')}</p>
										</div>
									</div>
									{#if instanPath === 'template'}
									<div class="instan-fact">
										<span class="instan-ico instan-ico-sm"><Clock size={14} /></span>
										<div class="min-w-0">
											<p class="instan-fact-title">{tr('Masa simpan berkas')}</p>
											<p class="instan-fact-desc">{tr('Berkas yang Anda unggah dihapus otomatis setelah masa simpannya berakhir. Teks dokumen yang sudah terbaca tetap tersimpan selama brief ini ada.')}</p>
										</div>
									</div>
									<div class="instan-fact">
										<span class="instan-ico instan-ico-sm"><Trash2 size={14} /></span>
										<div class="min-w-0">
											<p class="instan-fact-title">{tr('Kalau dokumen dihapus')}</p>
											<p class="instan-fact-desc">{tr('Jika dokumen dihapus, teksnya ikut terhapus. Ringkasan dan catatan yang sudah dibuat masih dapat memuat kutipannya, dan hal itu tidak dapat dibatalkan.')}</p>
										</div>
									</div>
									{/if}
								</div>
							</section>

						</aside>
					</div>
				</div>
			</div>
	{/snippet}

	{#if activeTab === 'copilot'}
		{#if gateMode === 'instan'}
			<!-- Gerbang 2: pane Instan Brief menggantikan area chat. -->
			{@render instanPane()}
		{:else}
			<!-- Pane Copilot: lapisan melayang di atas, obrolan mengisi sisanya. -->
			<div class="relative flex min-h-0 flex-1 flex-col">
				<!-- Lapisan melayang: kartu brief kiri, pill Copilot kanan, di atas obrolan. -->
				<div class="pointer-events-none absolute inset-x-0 top-2 z-20 flex items-start justify-between gap-3 px-4 md:px-6">
					<div class="pointer-events-auto">
						{#if cardOpen}
					<div class="brief-pop brief-pop-static">
							<div class="brief-pop-id">
						<!-- Header simpel ala toolbar: judul produk kiri, tombol kecilkan kanan. -->
						<h2 class="brief-pop-brand">Tubi AI</h2>
						<button
							class="brief-card-toggle"
							type="button"
							aria-expanded={cardOpen}
							aria-label={tr('Kecilkan')}
							title={tr('Kecilkan')}
							onclick={() => (cardOpen = false)}
						>
							<ChevronDown size={14} />
						</button>
							</div>
							<div class="brief-pop-head">
									<!-- Tombol "Obrolan Baru" saja di header; nama & progres
									     brief tampil di baris daftar. -->
									<button class="brief-newchat-sm" type="button" disabled={workingState} onclick={() => void createBrief()}>
										<SquarePen size={14} />
										<span>{tr('Obrolan Baru')}</span>
									</button>
								</div>
								<div
									class="brief-pop-list"
									onscroll={() => { if (popMenuId) { popMenuId = null; popMenuPos = null; } }}
								>
									{#if briefs.length === 0}
										<p class="brief-pop-empty">{tr('Belum ada brief.')}</p>
									{/if}
									{#each briefs as b (b.id)}
										{@const isActive = b.id === profileId}
										<div class="brief-pop-item">
											{#if popRenameId === b.id}
												<input
													class="app-input min-h-7 min-w-0 flex-1 !px-1.5 !py-0.5 text-[12.5px]"
													bind:value={popRenameValue}
													aria-label={tr('Ubah nama')}
													onkeydown={(e) => {
														if (e.key === 'Enter') { e.preventDefault(); void savePopRename(b.id); }
														if (e.key === 'Escape') popRenameId = null;
													}}
												/>
												<button class="brief-pop-icon brief-pop-save" type="button" aria-label={tr('Simpan nama')} onclick={() => void savePopRename(b.id)}>✓</button>
											{:else}
												<a
														class="brief-pop-row"
														class:active={isActive}
														href={`/klien/${brandUrl}/profiler/${b.short_code || b.id}`}
												>
														<span class="brief-marquee" use:marqueeOnHover title={b.name}><span>{b.name}</span></span>
														<span class="brief-pct">{briefPct(b)}%</span>
												</a>
												<button
													class="brief-pop-icon brief-pop-pin"
													class:pinned={b.pinned}
													type="button"
													aria-pressed={b.pinned}
													aria-label={b.pinned ? tr('Lepas sematan') : tr('Sematkan brief')}
													title={b.pinned ? tr('Lepas sematan') : tr('Sematkan brief')}
													onclick={(e) => { e.preventDefault(); void togglePin(b.id, b.pinned); }}
												>
													<Pin size={12} />
												</button>
												<button
													class="brief-pop-icon brief-pop-kebab"
													type="button"
													aria-label={tr('Aksi brief')}
													title={tr('Aksi brief')}
													onclick={(e) => { e.preventDefault(); togglePopMenu(b.id, e); }}
												>
													<MoreHorizontal size={14} />
												</button>
											{/if}
											{#if popMenuId === b.id}
												<button
													class="brief-pop-backdrop"
													type="button"
													aria-label={tr('Tutup')}
													onclick={() => { popMenuId = null; popMenuPos = null; }}
													onkeydown={(e) => { if (e.key === 'Escape') { popMenuId = null; popMenuPos = null; } }}
												></button>
												<div
													class="brief-menu brief-pop-menu"
													style="position: fixed; left: {popMenuPos?.left ?? 8}px; top: {popMenuPos?.top ?? 8}px; z-index: {popMenuPos?.placeUp ? 260 : 250};"
													role="menu"
												>
													<button type="button" role="menuitem" onclick={() => startPopRename(b.id, b.name)}>
														<Pencil size={13} />{tr('Ubah nama')}
													</button>
													<button type="button" role="menuitem" onclick={() => { popMenuId = null; popMenuPos = null; void duplicateBrief(b.id); }}>
														<Copy size={13} />{tr('Duplikat')}
													</button>
													<button type="button" role="menuitem" onclick={() => startSaveTemplate(b)}>
														<FileText size={13} />{tr('Simpan sebagai template')}
													</button>
													<button type="button" class="brief-menu-danger" role="menuitem" onclick={() => { popMenuId = null; popMenuPos = null; requestDelete(b); }}>
														<Trash2 size={13} />{tr('Hapus')}
													</button>
												</div>
											{/if}
										</div>
									{/each}
								</div>
					</div>
						{:else}
							<button
								class="brief-card-icon"
								type="button"
								aria-label={tr('Buka kartu brief')}
								title={tr('Buka kartu brief')}
								onclick={() => (cardOpen = true)}
							>
								<img src="/tauthub-icon.png" alt="" />
							</button>
						{/if}
					</div>
					<div class="pointer-events-auto">
						<div class="relative flex flex-shrink-0 items-center">
					<!-- Obrolan baru = jalur yang sama dengan "+ Brief baru": brief
					     lama tetap tersimpan (masih ada di daftar), layar pindah ke
					     obrolan yang benar-benar baru. -->
					<button
						class="brief-picker"
						type="button"
						disabled={streaming || workingState}
						aria-label={tr('Brief baru')}
						title={tr('Brief baru')}
						onclick={() => void createBrief()}
					>
						<SquarePen size={13} />
						<span class="truncate">{tr('Obrolan Baru')}</span>
					</button>

				</div>
				</div>
				</div>

			{#if error}
				<p class="mx-3 mt-2 rounded-lg p-2 text-sm" style="background: var(--warning-soft); color: var(--warning);">{error}</p>
			{/if}

			{#if draggingFiles && activeTab === 'copilot'}
				<div
					class="pointer-events-none absolute inset-2 z-20 flex items-center justify-center rounded-xl border-2 border-dashed"
					style="border-color: var(--primary); background: color-mix(in srgb, var(--primary) 8%, transparent);"
				>
					<p class="text-sm font-medium" style="color: var(--primary);">{tr('Lepaskan untuk melampirkan berkas')}</p>
				</div>
			{/if}

				<div bind:this={threadEl} class="flex min-h-0 flex-1 flex-col overflow-y-auto px-3 py-3">
					{#if !loading && messages.length === 0}
						<div class="m-auto w-full max-w-2xl px-4">
							<!-- Sapaan pembuka: judul besar + deskripsi, bukan gelembung chat. -->
							<h1 class="text-center text-2xl font-semibold tracking-tight">{tr('Bangun brief brand-mu lewat obrolan')}</h1>
							<p class="mx-auto mt-3 max-w-md text-center text-sm leading-relaxed text-muted-foreground">
								{tr('Ceritakan brand-mu, sisanya aku tanyakan satu per satu.')}
							</p>

							<!-- Template siap pakai: klien sat-set tinggal klik lalu
							     mengganti bagian [KURUNG SIKU] dengan data aslinya. -->
							<p class="brief-tpl-label">{tr('Atau mulai dari template')}</p>
							<div class="brief-templates">
								{#each BRIEF_TEMPLATES as tpl (tpl.title)}
									<button class="brief-tpl" type="button" onclick={() => applyBriefTemplate(tpl)}>
										<span class="brief-tpl-title">{tr(tpl.title)}</span>
										<span class="brief-tpl-desc">{tr(tpl.desc)}</span>
									</button>
								{/each}
							</div>
							<p class="brief-tpl-note">{tr('Semua bagian [KURUNG SIKU] wajib diganti data asli. Aku tidak mengarang angka yang tidak kamu berikan.')}</p>
						</div>
					{:else}
					<div class="mx-auto max-w-2xl space-y-2.5">
						{#each messages as m, i (m.id ?? `msg-${i}`)}
							{#if m.role === 'user'}
								<div class="flex items-start justify-end gap-2.5">
									<div class="flex max-w-[80%] min-w-0 flex-col items-end gap-1">
										<p class="max-w-full whitespace-pre-wrap rounded-2xl rounded-tr-sm px-4 py-2.5 text-[13px] leading-relaxed shadow-sm" style="background: var(--primary); color: var(--primary-foreground);">{m.content}</p>
										{#if timeLabel(m.created_at)}<span class="brief-time">{timeLabel(m.created_at)}</span>{/if}
									</div>
									<span class="brief-avatar brief-avatar-user" aria-hidden="true" title={userName}>{userInitials}</span>
								</div>
								{:else}
									<div class="brief-msg flex items-start gap-2.5">
										<span class="brief-avatar brief-avatar-ai" aria-hidden="true"><img src="/brief-chat-ai.webp" alt="" /></span>
										<div class="flex min-w-0 flex-col items-start gap-1">
											<div class="brief-markdown max-w-[92%] rounded-2xl rounded-tl-sm border px-4 py-3 text-[13px] leading-relaxed shadow-sm" style="border-color: var(--border); background: var(--card);">
												{@html sanitizeHtml(renderMarkdown(m.content))}
											</div>
											{#if m.actions?.length}
												<div class="brief-actions">
													{#each m.actions as action (action.kind)}
														{@const used = actionUsed(m, action.kind)}
														<button
															class="brief-action-btn"
															class:is-used={used}
															type="button"
															disabled={used || actionBusy === m.id}
															title={action.reason ?? ''}
															onclick={() => {
																if (action.kind === 'timeline_post') void runTimelineAction(m, action);
															}}
														>
															{#if used}
																<Check size={14} />{tr('Sudah ditambahkan')}
															{:else if actionBusy === m.id}
																<LoaderCircle size={14} class="animate-spin" />{tr('Menambahkan…')}
															{:else}
																<Newspaper size={14} />{tr(action.label)}
															{/if}
														</button>
														{#if !used && action.caption}
															<p class="brief-action-preview">{action.caption}</p>
														{/if}
													{/each}
												</div>
											{/if}
											<div class="flex items-center gap-1">
												{#if timeLabel(m.created_at)}<span class="brief-time">{timeLabel(m.created_at)}</span>{/if}
												<button
													class="brief-msg-action"
													type="button"
													onclick={() => void copyReply(i, m.content)}
												>
													{#if copiedIndex === i}
														<Check size={13} />{tr('Tersalin')}
													{:else}
														<Copy size={13} />{tr('Salin')}
													{/if}
												</button>
											</div>
										</div>
									</div>
							{/if}
				{/each}

						{#if streamingText}
							<div class="flex items-start gap-2.5">
								<span class="brief-avatar brief-avatar-ai" aria-hidden="true"><img src="/brief-chat-ai.webp" alt="" /></span>
								<div class="max-w-[92%] whitespace-pre-wrap rounded-2xl rounded-tl-sm border px-4 py-3 text-[13px] leading-relaxed shadow-sm" style="border-color: var(--border); background: var(--card);">
									{streamingText}<span class="inline-block animate-pulse">▍</span>
								</div>
							</div>
						{:else if streaming}
							<div class="flex items-start gap-2.5">
								<span class="brief-avatar brief-avatar-ai" aria-hidden="true"><img src="/brief-chat-ai.webp" alt="" /></span>
								<div class="brief-thinking">
									<span class="thinking-dots" aria-hidden="true"><span></span><span></span><span></span></span>
									<span>{streamStatus || tr('Sedang berpikir…')}</span>
								</div>
							</div>
						{/if}

						{#if streamError}
							<p class="rounded-lg p-2 text-xs" style="background: var(--warning-soft); color: var(--warning);">{streamError}</p>
						{/if}
					</div>
				{/if}
			</div>

			<!-- Popup ajakan ringkas: muncul saat entitas 100% (sekali per progres)
			     dan saat user hendak mengirim pesan setelah ringkasan jadi. -->
			{#if summaryPrompt !== 'idle'}
				<div class="brief-summary-nudge-wrap">
					<div class="brief-summary-nudge" role="status" aria-live="polite">
						{#if summaryPrompt === 'invite'}
							<span class="brief-nudge-icon" aria-hidden="true"><Sparkle size={16} /></span>
							<div class="min-w-0 flex-1">
								<p class="text-[13px] font-bold">{tr('Field wajib sudah lengkap. Ringkas sekarang?')}</p>
								<p class="mt-0.5 text-[11.5px] text-muted-foreground">
									{entitySummaryReady
										? tr('Mau lanjut mengedit entitas yang sudah ada?')
										: tr('Satukan fakta brand-mu jadi satu ringkasan siap pakai.')}
								</p>
							</div>
							<div class="flex shrink-0 items-center gap-1.5">
								{#if entitySummaryReady}
									<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" type="button" onclick={() => void continueToEdit()}>
										{tr('Ya, lanjut edit')}
									</button>
									<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={dismissInvite}>
										{tr('Nanti')}
									</button>
								{:else}
									<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" type="button" onclick={() => void runSummaryFromPrompt()}>
										{tr('Ya, Ringkas')}
									</button>
									<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={dismissInvite}>
										{tr('Nanti')}
									</button>
								{/if}
							</div>
						{:else if summaryPrompt === 'working'}
							<span class="brief-nudge-icon brief-nudge-spin" aria-hidden="true"><LoaderCircle size={16} /></span>
							<div class="min-w-0 flex-1">
								<p class="text-[13px] font-bold">{tr('Menyusun ringkasan…')}</p>
								<div class="brief-nudge-bar mt-1.5"><span></span></div>
								<p class="mt-1 text-[11.5px] text-muted-foreground">{tr('Membaca identitas dan fakta resmi dari brief-mu.')}</p>
							</div>
						{:else if summaryPrompt === 'done'}
							<span class="brief-nudge-icon brief-nudge-ok" aria-hidden="true"><Check size={16} /></span>
							<div class="min-w-0 flex-1">
								<p class="text-[13px] font-bold">{tr('Ringkasan siap!')}</p>
								<p class="mt-0.5 text-[11.5px] text-muted-foreground">
									{#if summaryOf('entity').source_chars > 0 && summaryOf('entity').content}
										{tr('Hemat')} {Math.max(0, Math.round((1 - summaryOf('entity').content.length / summaryOf('entity').source_chars) * 100))}% {tr('dibanding bahan aslinya.')}
									{:else}
										{tr('Ringkasan entitas sudah dibuat.')}
									{/if}
								</p>
							</div>
							<div class="flex shrink-0 items-center gap-1.5">
								<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={() => (activeTab = 'ringkasan')}>
									{tr('Lihat')}
								</button>
								<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" type="button" onclick={() => void continueToEdit()}>
									{tr('Lanjut edit')}
								</button>
							</div>
						{:else if summaryPrompt === 'error'}
							<span class="brief-nudge-icon brief-nudge-err" aria-hidden="true"><ShieldAlert size={16} /></span>
							<div class="min-w-0 flex-1">
								<p class="text-[13px] font-bold">{tr('Ringkasan gagal dibuat')}</p>
								<p class="mt-0.5 line-clamp-2 text-[11.5px]" style="color: var(--destructive);">{summaryPromptError}</p>
							</div>
							<button class="app-btn app-btn-soft !min-h-8 !shrink-0 !px-3 !text-xs" type="button" onclick={() => (summaryPrompt = 'idle')}>
								{tr('Tutup')}
							</button>
						{/if}
					</div>
				</div>
			{/if}

			<!-- Composer + layanan, menempel di bawah tanpa ikut kerek halaman. -->
			<div class="border-t px-3 py-2" style="border-color: var(--border);">
				{#if !configured}
					<p class="mx-auto mb-1.5 max-w-2xl text-xs" style="color: var(--warning);">
						{tr('Fitur AI ini belum aktif di platform. Isi brief tetap bisa dilihat dan dikoreksi lewat Ringkasan.')}
					</p>
				{/if}

				<div class="mx-auto max-w-2xl">

					<div class="brief-composer">
						{#if pendingDocs.length > 0}
							<div class="brief-composer-files">
								{#each pendingDocs as d, i (d.file.name + String(d.file.size) + i)}
									<span class="brief-file-chip">
										<FileText size={13} class="shrink-0" style="color: var(--primary);" />
										<span class="max-w-[190px] truncate font-medium">{d.file.name}</span>
										<button
											type="button"
											aria-label={tr('Lepaskan lampiran')}
											class="rounded p-0.5 hover:bg-secondary"
											onclick={() => (pendingDocs = pendingDocs.filter((_, xi) => xi !== i))}
										>
											<X size={12} />
										</button>
									</span>
								{/each}
								<span class="self-center text-[10.5px] font-semibold text-muted-foreground">{pendingDocs.length}/{CHAT_MAX_FILES}</span>
							</div>
						{/if}

						<textarea
							id="brief-composer"
							class="brief-composer-input"
							rows="1"
							placeholder={tr('Tulis balasan…')}
							bind:value={composer}
							disabled={streaming}
							oninput={(e) => autoGrow(e.currentTarget as HTMLTextAreaElement)}
							onpaste={(e: ClipboardEvent) => {
								const files = e.clipboardData?.files;
								if (files && files.length > 0) {
									e.preventDefault();
									stageFiles(files);
								}
							}}
							onkeydown={(e) => {
								if (e.key === 'Enter' && !(e as KeyboardEvent).shiftKey) {
									e.preventDefault();
									void send();
								}
							}}
						></textarea>

						<div class="brief-composer-toolbar">
							<div class="brief-composer-tools">
								<div class="brief-attach">
									<button
										class="brief-composer-icon"
										type="button"
										aria-label={tr('Lampirkan berkas')}
										aria-expanded={attachOpen}
										onclick={() => (attachOpen = !attachOpen)}
									>
										<Plus size={18} />
									</button>

									{#if attachOpen}
										<button class="brief-pop-backdrop" type="button" aria-label={tr('Tutup')} onclick={() => (attachOpen = false)}></button>
										<div
											class="brief-attach-pop"
											role="menu"
											tabindex="-1"
											onkeydown={(e) => {
												if (e.key === 'Escape') attachOpen = false;
											}}
										>
											<button
												class="brief-attach-item"
												type="button"
												role="menuitem"
												onclick={() => { attachOpen = false; document.getElementById('brief-image-input')?.click(); }}
											>
												<span class="brief-attach-icon" aria-hidden="true">
													<svg width="21" height="21" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
														<path d="M25.6 0H6.4C2.86538 0 0 2.86538 0 6.4V25.6C0 29.1346 2.86538 32 6.4 32H25.6C29.1346 32 32 29.1346 32 25.6V6.4C32 2.86538 29.1346 0 25.6 0Z" fill="url(#brief-attach-img-grad)" />
														<path d="M5.9577 24.8845C5.42578 25.9483 6.19937 27.2 7.38878 27.2H18.2111C19.4005 27.2 20.1741 25.9483 19.6422 24.8845L14.231 14.0622C13.6414 12.8829 11.9585 12.8829 11.3688 14.0622L5.9577 24.8845Z" fill="white" />
														<path d="M15.5577 24.8845C15.0258 25.9483 15.7994 27.2 16.9888 27.2H24.6111C25.8005 27.2 26.5741 25.9483 26.0422 24.8845L22.231 17.2622C21.6414 16.0829 19.9585 16.0829 19.3688 17.2622L15.5577 24.8845Z" fill="white" fill-opacity="0.6" />
														<path d="M24.0002 11.2C25.7675 11.2 27.2002 9.76726 27.2002 7.99995C27.2002 6.23264 25.7675 4.79995 24.0002 4.79995C22.2329 4.79995 20.8002 6.23264 20.8002 7.99995C20.8002 9.76726 22.2329 11.2 24.0002 11.2Z" fill="white" />
														<defs>
															<linearGradient id="brief-attach-img-grad" x1="16" y1="0" x2="16" y2="32" gradientUnits="userSpaceOnUse">
																<stop stop-color="#00E676" />
																<stop offset="1" stop-color="#00C853" />
															</linearGradient>
														</defs>
													</svg>
												</span>
												{tr('Gambar')}
											</button>
											<button
												class="brief-attach-item"
												type="button"
												role="menuitem"
												onclick={() => { attachOpen = false; document.getElementById('brief-doc-input')?.click(); }}
											>
												<span class="brief-attach-icon" aria-hidden="true">
													<svg width="21" height="21" viewBox="0 0 512 512" xmlns="http://www.w3.org/2000/svg">
														<polygon fill="#FFB74D" points="406.941,98.68 406.941,473.339 39.241,473.339 39.241,0 308.793,0 406.794,98.534" />
														<polygon fill="#FFA91E" points="406.941,98.68 406.941,473.339 223.091,473.339 223.091,0 308.793,0 406.794,98.534" />
														<g fill="#FFFFFF">
															<rect x="98.858" y="257.526" width="137.927" height="31.347" />
															<rect x="97.865" y="130.675" width="137.927" height="31.347" />
															<rect x="97.865" y="194.414" width="250.775" height="31.347" />
														</g>
														<polygon fill="#FF8900" points="406.794,98.532 308.794,98.532 308.794,0" />
														<path fill="#67BFFF" d="M472.759,406.601C472.759,464.812,425.571,512,367.36,512c-0.251,0-0.502,0-0.752-0.01c-28.473-0.188-54.251-11.682-73.101-30.208c-16.927-16.635-28.264-38.933-30.929-63.843c-0.397-3.72-0.616-7.502-0.616-11.337c0-31.587,13.908-59.925,35.913-79.245c0.24-0.209,0.481-0.418,0.721-0.616c8.579-7.398,18.369-13.427,29.017-17.774c12.048-4.901,25.203-7.659,38.996-7.743c0.251-0.01,0.502-0.01,0.752-0.01C425.571,301.213,472.759,348.39,472.759,406.601z" />
														<path fill="#0088FF" d="M472.759,406.601C472.759,464.812,425.571,512,367.36,512c-0.251,0-0.502,0-0.752-0.01V301.223c0.251-0.01,0.502-0.01,0.752-0.01C425.571,301.213,472.759,348.39,472.759,406.601z" />
														<polygon fill="#FFFFFF" points="422.823,391.952 422.823,423.299 383.033,423.299 383.033,465.157 351.687,465.157 351.687,423.299 311.897,423.299 311.897,391.952 351.687,391.952 351.687,350.103 383.033,350.103 383.033,391.952" />
														<g fill="#D9D8DB">
															<polygon points="422.823,391.952 422.823,423.299 383.033,423.299 383.033,465.157 366.608,465.157 366.608,350.103 383.033,350.103 383.033,391.952" />
															<rect x="223.086" y="130.675" width="12.696" height="31.347" />
															<rect x="223.086" y="257.526" width="13.688" height="31.347" />
															<rect x="223.086" y="194.414" width="125.544" height="31.347" />
														</g>
													</svg>
												</span>
												{tr('Dokumen')}
											</button>
										</div>
									{/if}

									<input
										id="brief-image-input"
										class="hidden"
										type="file"
										multiple
										accept="image/png,image/jpeg,image/webp,image/gif"
										onchange={(e) => {
											stageFiles((e.currentTarget as HTMLInputElement).files, 'image');
											(e.currentTarget as HTMLInputElement).value = '';
										}}
									/>
									<input
										id="brief-doc-input"
										class="hidden"
										type="file"
										multiple
										accept=".pdf,.docx,.txt,.md,.csv,application/pdf"
										onchange={(e) => {
											stageFiles((e.currentTarget as HTMLInputElement).files, 'doc');
											(e.currentTarget as HTMLInputElement).value = '';
										}}
									/>
								</div>

								{#if uploadingNow}
									<span class="brief-composer-uploading">
										<LoaderCircle size={12} class="animate-spin" />{tr('Mengunggah…')}
									</span>
								{/if}
							</div>

							<div class="brief-composer-actions">
								{#if streaming}
									<span class="brief-composer-thinking">
										<LoaderCircle size={12} class="animate-spin" />
										{tr('Berpikir')}
									</span>
								{/if}
								{#if speechSupported}
									<button
										class="brief-composer-icon"
										class:brief-mic-active={listening}
										type="button"
										aria-label={listening ? tr('Berhenti mendikte') : tr('Dikte suara')}
										onclick={toggleMic}
									>
										<Mic size={17} />
									</button>
								{/if}
								<button
									class="brief-composer-send"
									class:brief-composer-stop={streaming}
									type="button"
									aria-label={streaming ? tr('Hentikan generasi') : tr('Kirim')}
									disabled={streaming || uploadingNow ? false : !composer.trim() && pendingDocs.length === 0}
									onclick={() => (streaming ? controller?.abort() : void send())}
								>
									{#if streaming}<Square size={13} />{:else}<ArrowUp size={16} />{/if}
								</button>
							</div>
						</div>
					</div>

					<p class="mt-1 text-center text-[11px] text-muted-foreground">
						{#if streaming}
							{tr('Sedang menyusun balasan. Tombol stop menghentikannya.')}
						{:else}
							{tr('Enter kirim, Shift+Enter baris baru. Seret berkas ke sini atau tempel (Ctrl+V) untuk melampirkan.')}{#if sources.length > 0}&nbsp;·&nbsp;{sources.length} {tr('sumber dibaca')}{/if}
						{/if}
					</p>
					</div>
				</div>
			</div>
		{/if}
							{:else if activeTab === 'entitas'}

				<!-- ── Panel tab Ringkasan Entitas (desain GEO) ─────────────── -->
			<div class="min-h-0 flex-1 overflow-y-auto px-4 py-4 md:px-6">
				<div class="w-full">
					<section class="app-card p-4">
						<div class="flex flex-wrap items-center justify-between gap-3">
							<div class="min-w-0 flex-1">
								<div class="h-1.5 w-full overflow-hidden rounded-full" style="background: var(--secondary);">
									<div class="h-full" style="width: {progressPct}%; background-color: var(--primary);"></div>
								</div>
								<p class="mt-1.5 text-xs text-muted-foreground">
									{progress.filled} / {progress.total} {tr('field wajib terisi')} · {sources.length} {tr('sumber dibaca')}
									{#if progressPct >= 100 && optionalEmpty.length > 0}
										&nbsp;·&nbsp;{optionalEmpty.length} {tr('field opsional masih kosong')}
									{/if}
								</p>
								{#if progressPct >= 100 && optionalEmpty.length > 0}
									<button
										class="mt-2 inline-flex items-center gap-1.5 text-[11px] font-bold underline-offset-2 hover:underline"
										style="color: var(--primary);"
										type="button"
										onclick={() => void askCopilotForOptionals()}
									>
										{tr('Lengkapi field opsional lewat Copilot')} →
									</button>
								{/if}
							</div>
							<span
								class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-[11px] font-bold"
								style="background: {progressPct >= 100 ? 'var(--success-soft)' : 'var(--warning-soft)'}; color: {progressPct >= 100 ? 'var(--success)' : 'var(--warning)'};"
							>
								<span class="h-1.5 w-1.5 rounded-full" style="background: {progressPct >= 100 ? 'var(--success)' : 'var(--warning)'};"></span>
								{progressPct}%
							</span>
						</div>
					</section>

					<div class="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-2">
						<div class="space-y-4">
							<section class="app-card p-5">
								<div class="flex items-center gap-3 border-b pb-4" style="border-color: var(--border);">
									<span class="flex h-9 w-9 items-center justify-center rounded-xl" style="background: var(--accent); color: var(--accent-foreground);">
										<UserCircle size={18} />
									</span>
									<div>
										<h3 class="text-sm font-bold">{tr('Identitas Inti Entitas (Core Entity)')}</h3>
										<p class="text-[11px] text-muted-foreground">{tr('Data resmi yang dipakai sistem pencarian dan LLM untuk mengenali brand')}</p>
									</div>
								</div>
								<div class="mt-4 space-y-3">
									{#each [['canonical_name', 'Nama resmi entitas'], ['entity_category', 'Kategori entitas'], ['canonical_description', 'Deskripsi resmi'], ['boilerplate', 'Teks "Tentang [brand]"']] as pair (pair[0])}
										<div>
											<p class="mb-1 block text-[11px] font-bold uppercase tracking-wider text-muted-foreground">{tr(pair[1])}</p>
											<div class="rounded-xl border p-3" style="background: color-mix(in srgb, var(--muted) 60%, white); border-color: var(--border);">
												{#if String(ringkasan[pair[0]] ?? '').trim()}
													<p class="text-xs leading-relaxed">{ringkasan[pair[0]]}</p>
												{:else}
													<p class="text-xs italic" style="color: var(--muted-foreground);">{tr('Belum diisi. Lengkapi lewat obrolan Copilot.')}</p>
												{/if}
											</div>
										</div>
									{/each}
								</div>
							</section>

							<section class="app-card p-5">
								<div class="flex items-center gap-3 border-b pb-4" style="border-color: var(--border);">
									<span class="flex h-9 w-9 items-center justify-center rounded-xl" style="background: var(--accent); color: var(--accent-foreground);">
										<ShieldAlert size={18} />
									</span>
									<div>
										<h3 class="text-sm font-bold">{tr('Batasan & Panduan Narasi Media (Guardrails Advertorial)')}</h3>
										<p class="text-[11px] text-muted-foreground">{tr('Instruksi otomatis untuk jurnal & generator konten PR')}</p>
									</div>
								</div>
								<div class="mt-4 space-y-4">
									<div class="rounded-xl border p-4" style="border-color: color-mix(in srgb, var(--success) 28%, transparent); background: var(--success-soft);">
										<div class="mb-2.5 flex items-center gap-2">
											<span class="flex h-5 w-5 items-center justify-center rounded-full text-xs font-bold text-white" style="background: var(--success);"><Check size={12} strokeWidth={3} /></span>
											<span class="text-xs font-bold uppercase tracking-wide" style="color: var(--success);">{tr('PILAR NARASI UTAMA (DIANJURKAN)')}</span>
										</div>
										{#if items('allowed_claims').length > 0}
											<ul class="list-disc space-y-2 pl-7 text-xs leading-relaxed">
												{#each items('allowed_claims') as it (it)}
													<li>{it}</li>
												{/each}
											</ul>
										{:else}
											<p class="text-xs italic" style="color: var(--muted-foreground);">{tr('Belum ada. Lengkapi lewat obrolan Copilot.')}</p>
										{/if}
									</div>
									<div class="rounded-xl border p-4" style="border-color: color-mix(in srgb, var(--destructive) 30%, transparent); background: color-mix(in srgb, var(--destructive) 7%, white);">
										<div class="mb-2.5 flex items-center gap-2">
											<span class="flex h-5 w-5 items-center justify-center rounded-full text-xs font-bold text-white" style="background: var(--destructive);"><X size={12} strokeWidth={3} /></span>
											<span class="text-xs font-bold uppercase tracking-wide" style="color: var(--destructive);">{tr('LARANGAN KERAS (NEGATIVE CONSTRAINTS)')}</span>
										</div>
										{#if items('forbidden_claims').length > 0 || items('forbidden_words').length > 0 || items('forbidden_terms').length > 0}
											<ul class="list-disc space-y-2 pl-7 text-xs leading-relaxed">
												{#each [...items('forbidden_claims'), ...items('forbidden_words'), ...items('forbidden_terms')] as it, xi (it + String(xi))}
													<li>{it}</li>
												{/each}
											</ul>
										{:else}
											<p class="text-xs italic" style="color: var(--muted-foreground);">{tr('Belum ada. Lengkapi lewat obrolan Copilot.')}</p>
										{/if}
									</div>
								</div>
							</section>
						</div>

						<div class="space-y-4">
							<section class="app-card p-5">
								<div class="flex items-center gap-3 border-b pb-4" style="border-color: var(--border);">
									<span class="flex h-9 w-9 items-center justify-center rounded-xl" style="background: var(--accent); color: var(--accent-foreground);">
										<ChartLine size={18} />
									</span>
									<div>
										<h3 class="text-sm font-bold">{tr('Fakta Resmi & Sinyal Entitas (Entity Facts)')}</h3>
										<p class="text-[11px] text-muted-foreground">{tr('Atribut spesifik yang dijadikan rujukan oleh Knowledge Graph')}</p>
									</div>
								</div>
								<div class="mt-4 space-y-3">
									{#each [['official_facts', 'Fakta resmi (tahun, jumlah, lokasi)'], ['usp', 'Keunggulan unik (USP)'], ['key_facts', 'Fakta penting yang wajib dipertahankan'], ['region', 'Wilayah operasional'], ['website', 'Website resmi (canonical domain)']] as pair (pair[0])}
										<div>
											<p class="mb-1 block text-[11px] font-bold uppercase tracking-wider text-muted-foreground">{tr(pair[1])}</p>
											<div class="rounded-xl border p-3" style="background: color-mix(in srgb, var(--muted) 60%, white); border-color: var(--border);">
												{#if String(ringkasan[pair[0]] ?? '').trim()}
													<p class="text-xs leading-relaxed">{ringkasan[pair[0]]}</p>
												{:else}
													<p class="text-xs italic" style="color: var(--muted-foreground);">{tr('Belum diisi. Lengkapi lewat obrolan Copilot.')}</p>
												{/if}
											</div>
										</div>
									{/each}
								</div>
							</section>

							<section class="app-card p-5">
								<div class="flex items-center gap-3 border-b pb-4" style="border-color: var(--border);">
									<span class="flex h-9 w-9 items-center justify-center rounded-xl" style="background: var(--accent); color: var(--accent-foreground);">
										<Search size={18} />
									</span>
									<div>
										<h3 class="text-sm font-bold">{tr('Target Pertanyaan AI (GEO Target Query)')}</h3>
										<p class="text-[11px] text-muted-foreground">{tr('Pertanyaan yang ingin brand-mu menangkan di jawaban mesin AI')}</p>
									</div>
								</div>
								<div class="mt-4 space-y-2.5">
									{#if items('target_questions').length > 0}
										{#each items('target_questions') as q (q)}
											<div class="flex items-center justify-between gap-3 rounded-xl border p-3" style="border-color: var(--border); background: color-mix(in srgb, var(--muted) 60%, white);">
												<span class="flex min-w-0 items-center gap-2.5">
													<span class="flex-shrink-0 text-xs font-bold" style="color: var(--primary);">›</span>
													<span class="truncate text-xs font-medium">{q}</span>
												</span>
												<span
													class="inline-flex flex-shrink-0 items-center gap-1 rounded px-2 py-0.5 text-[10px] font-bold"
													style="background: {isFilled('target_questions') ? 'var(--success-soft)' : 'var(--secondary)'}; color: {isFilled('target_questions') ? 'var(--success)' : 'var(--muted-foreground)'};"
												>
													<span class="h-1.5 w-1.5 rounded-full" style="background: {isFilled('target_questions') ? 'var(--success)' : 'var(--muted-foreground)'};"></span>
													{isFilled('target_questions') ? tr('Sinyal Siap') : tr('Belum terverifikasi')}
												</span>
											</div>
										{/each}
									{:else}
										<p class="rounded-xl border px-3.5 py-3 text-xs italic" style="border-color: var(--border); color: var(--muted-foreground);">
											{tr('Belum ada pertanyaan target. Lengkapi lewat obrolan Copilot.')}
										</p>
									{/if}
								</div>
							</section>
						</div>
					</div>
				</div>
			</div>
{:else if activeTab === 'timeline'}
				<!-- Timeline Brand: feed gaya Threads — kartu putih, pemisah garis. -->
				<div class="min-h-0 flex-1 overflow-y-auto timeline-scroll bg-[var(--muted)] px-4 py-5 md:px-6">
					<div class="mx-auto w-full max-w-2xl space-y-4">
						<header class="flex items-center gap-3 px-1">
							<span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full" style="background: color-mix(in srgb, var(--primary) 10%, white); color: var(--primary);">
								<Store size={18} />
							</span>
							<div class="min-w-0">
								<h2 class="text-lg font-bold tracking-tight">{tr('Timeline Brand')}</h2>
								<p class="text-[12.5px] text-muted-foreground">
									{tr('Bagikan kabar terbaru brand-mu kepada audiens.')}
									<span class="ml-1 text-[11px] opacity-80">{tr('Kedaluwarsa otomatis 3 bulan.')}</span>
								</p>
							</div>
						</header>

						<section class="timeline-card">
							<div
								class="timeline-compose-row"
								role="button"
								tabindex="0"
								aria-label={tr('Tulis post baru')}
								onclick={openTimelineComposer}
								onkeydown={(e) => {
									if (e.key === 'Enter' || e.key === ' ') {
										e.preventDefault();
										openTimelineComposer();
									}
								}}
							>
								<span class="timeline-avatar" aria-hidden="true">{timelineInitials(timelineAuthorName())}</span>
								<span class="timeline-trigger-text" class:timeline-trigger-empty={!timelineBody.trim()}>
									{timelineBody.trim() || tr('Apa yang baru?')}
								</span>
								<button class="timeline-kirim" type="button" onclick={openTimelineComposer}>{tr('Kirim')}</button>
							</div>

							{#if timelinePostError}
								<div class="timeline-error" role="alert">
									<span class="min-w-0 flex-1">{timelinePostError}</span>
								</div>
							{/if}
						</section>

						{#if timelineLoading}
							<div class="timeline-card timeline-empty"><LoaderCircle size={18} class="animate-spin" /><span>{tr('Memuat timeline…')}</span></div>
						{:else if timelineError}
							<div class="timeline-card timeline-empty timeline-empty-error">
								<p>{timelineError}</p>
								<button class="timeline-error-retry" type="button" onclick={() => void loadTimeline()}>{tr('Coba lagi')}</button>
							</div>
						{:else if timeline.length === 0}
							<div class="timeline-card timeline-empty">
								<span class="timeline-empty-icon"><Newspaper size={18} /></span>
								<strong>{tr('Belum ada kabar terbaru')}</strong>
								<p>{tr('Bagikan informasi, promo, atau pembaruan brand-mu agar tampil di Timeline Brand.')}</p>
							</div>
						{:else}
							<section class="timeline-card timeline-feed">
								{#each timeline as post (post.id)}
									<article id={`timeline-${post.id}`} class="timeline-post">
										<span class="timeline-avatar" aria-hidden="true">{timelineInitials(timelinePostAuthor(post))}</span>
										<div class="timeline-post-main min-w-0 flex-1">
											<div class="timeline-post-meta">
												<span class="timeline-post-name">{timelinePostAuthor(post)}</span>
												<span class="timeline-post-time">{timelineRelativeTime(post.created_at)}</span>
												<button
													class="timeline-more"
													type="button"
													aria-label={tr('Menu post')}
													aria-expanded={timelineMenuFor === post.id}
													title={tr('Menu post')}
													onclick={() => (timelineMenuFor = timelineMenuFor === post.id ? null : post.id)}
												>
													<MoreHorizontal size={17} />
												</button>
											</div>
											{#if post.body}
												<p class="timeline-post-body">
													{#each timelineBodyParts(post.body) as part, i (`b-${i}`)}{#if part.tag}<span class="timeline-body-tag">{part.text}</span>{:else}{part.text}{/if}{/each}
												</p>
											{/if}
											{#if post.link_url}
												<a class="timeline-link-card" href={post.link_url} target="_blank" rel="noopener noreferrer">
													<span class="timeline-link-body">
														<span class="timeline-link-site">{post.link_site || post.link_url}</span>
														<strong class="timeline-link-title">{post.link_title || post.link_url}</strong>
														{#if post.link_description}<span class="timeline-link-desc">{post.link_description}</span>{/if}
													</span>
													{#if post.link_image}<img class="timeline-link-thumb" src={post.link_image} alt="" loading="lazy" />{/if}
												</a>
											{/if}
											{#if post.image_url}<img src={post.image_url} alt={tr('Gambar postingan') + ' ' + (timelineBrandName || profileName)} class="timeline-post-image" loading="lazy" />{/if}
											{#if timelineMenuFor === post.id}
												<button class="timeline-menu-backdrop" type="button" aria-label={tr('Tutup menu')} onclick={() => (timelineMenuFor = null)}></button>
												<div class="timeline-menu" role="menu">
													<button
														type="button"
														role="menuitem"
														onclick={() => {
															timelineMenuFor = null;
															void copyTimelineLink(post);
														}}
													>
														<Clipboard size={15} />{tr('Salin tautan')}
													</button>
													<button
														class="timeline-menu-danger"
														type="button"
														role="menuitem"
														onclick={() => {
															timelineMenuFor = null;
															void deleteTimelinePost(post);
														}}
													>
														<Trash2 size={15} />{tr('Hapus post')}
													</button>
												</div>
											{/if}
										</div>
									</article>
								{/each}
							</section>
						{/if}
					</div>
				</div>

				{#if timelineComposerOpen}
					<!-- Modal composer: rich field (tautan, gambar, tagar) sebelum dikirim. -->
					<div class="timeline-modal-backdrop" role="presentation" onclick={requestCloseTimelineComposer}></div>
					<div class="timeline-modal" role="dialog" aria-modal="true" aria-label={tr('Utas baru')}>
						{#if timelineDraftsOpen}
							<header class="timeline-modal-head">
								<button class="timeline-modal-icon-btn timeline-modal-back" type="button" aria-label={tr('Kembali')} onclick={() => void backToTimelineComposer()}>
									<ArrowLeft size={18} />
								</button>
								<span class="timeline-modal-title">{tr('Draf')}</span>
								<span class="timeline-modal-spacer"></span>
							</header>
							<div class="timeline-drafts-body">
								{#if timelineDrafts.length === 0}
									<div class="timeline-drafts-empty">
										<FileText size={46} strokeWidth={1.4} />
										<strong>{tr('Belum ada draf')}</strong>
										<p>{tr('Draf yang kamu buat akan tampil di sini.')}</p>
									</div>
								{:else}
									{#each timelineDrafts as draft (draft.id)}
										<div class="timeline-draft-row">
											<button class="timeline-draft-open" type="button" onclick={() => resumeTimelineDraft(draft)}>
												<span class="timeline-draft-text">{draft.body.trim() || tr('(tanpa teks)')}</span>
												<span class="timeline-draft-meta">
													{new Date(draft.savedAt).toLocaleString('id-ID', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}
												</span>
											</button>
											{#if draft.imageUrl}
												<img src={draft.imageUrl} alt="" class="timeline-draft-thumb" loading="lazy" />
											{/if}
											<button class="timeline-more" type="button" aria-label={tr('Hapus draf')} title={tr('Hapus draf')} onclick={() => deleteTimelineDraft(draft.id)}>
												<Trash2 size={16} />
											</button>
										</div>
									{/each}
								{/if}
							</div>
						{:else}
						<header class="timeline-modal-head">
							<button class="timeline-modal-cancel" type="button" onclick={requestCloseTimelineComposer}>{tr('Batal')}</button>
							<span class="timeline-modal-title">{tr('Utas baru')}</span>
							<button class="timeline-modal-icon-btn" type="button" aria-label={tr('Draf')} title={tr('Draf')} onclick={openTimelineDrafts}>
								<FileText size={18} />
							</button>
						</header>

						<div
							class="timeline-modal-body"
							class:timeline-modal-dragging={timelineDragging}
							role="group"
							aria-label={tr('Isi post')}
							ondragover={(e) => {
								e.preventDefault();
								timelineDragging = true;
							}}
							ondragleave={() => (timelineDragging = false)}
							ondrop={handleTimelineDrop}
						>
							<div class="timeline-compose-top">
								<div class="timeline-compose-rail">
									<span class="timeline-avatar timeline-avatar-sm" aria-hidden="true">{timelineInitials(timelineAuthorName())}</span>
									<span class="timeline-compose-line" aria-hidden="true"></span>
								</div>
								<div class="timeline-compose-content">
									<p class="timeline-modal-name">
										<span>{timelineAuthorName()}</span>
										<svg class="timeline-verified" width="14" height="14" viewBox="0 0 16 16" fill="none" role="img" aria-label={tr('Terverifikasi')}>
											<path
												d="M16 8.375C16 8.93437 15.8656 9.45312 15.5969 9.92813C15.3281 10.4031 14.9688 10.775 14.5156 11.0344C14.5281 11.1188 14.5344 11.25 14.5344 11.4281C14.5344 12.275 14.25 12.9937 13.6875 13.5875C13.1219 14.1844 12.4406 14.4812 11.6438 14.4812C11.2875 14.4812 10.9469 14.4156 10.625 14.2844C10.375 14.7969 10.0156 15.2094 9.54375 15.525C9.075 15.8438 8.55937 16 8 16C7.42812 16 6.90938 15.8469 6.44688 15.5344C5.98125 15.225 5.625 14.8094 5.375 14.2844C5.05312 14.4156 4.71562 14.4812 4.35625 14.4812C3.55937 14.4812 2.875 14.1844 2.30312 13.5875C1.73125 12.9937 1.44687 12.2719 1.44687 11.4281C1.44687 11.3344 1.45938 11.2031 1.48125 11.0344C1.02813 10.7719 0.66875 10.4031 0.4 9.92813C0.134375 9.45312 0 8.93437 0 8.375C0 7.78125 0.15 7.23438 0.446875 6.74062C0.74375 6.24687 1.14375 5.88125 1.64375 5.64375C1.5125 5.2875 1.44687 4.92812 1.44687 4.57188C1.44687 3.72813 1.73125 3.00625 2.30312 2.4125C2.875 1.81875 3.55937 1.51875 4.35625 1.51875C4.7125 1.51875 5.05312 1.58438 5.375 1.71563C5.625 1.20312 5.98438 0.790625 6.45625 0.475C6.925 0.159375 7.44063 0 8 0C8.55937 0 9.075 0.159375 9.54375 0.471875C10.0125 0.7875 10.375 1.2 10.625 1.7125C10.9469 1.58125 11.2844 1.51562 11.6438 1.51562C12.4406 1.51562 13.1219 1.8125 13.6875 2.40937C14.2531 3.00625 14.5344 3.725 14.5344 4.56875C14.5344 4.9625 14.475 5.31875 14.3562 5.64062C14.8562 5.87813 15.2563 6.24375 15.5531 6.7375C15.85 7.23438 16 7.78125 16 8.375ZM7.65938 10.7844L10.9625 5.8375C11.0469 5.70625 11.0719 5.5625 11.0437 5.40938C11.0125 5.25625 10.9344 5.13438 10.8031 5.05312C10.6719 4.96875 10.5281 4.94063 10.375 4.9625C10.2188 4.9875 10.0938 5.0625 10 5.19375L7.09062 9.56875L5.75 8.23125C5.63125 8.1125 5.49375 8.05625 5.34062 8.0625C5.18437 8.06875 5.05 8.125 4.93125 8.23125C4.825 8.3375 4.77187 8.47187 4.77187 8.63437C4.77187 8.79375 4.825 8.92813 4.93125 9.0375L6.77187 10.8781L6.8625 10.95C6.96875 11.0219 7.07812 11.0562 7.18437 11.0562C7.39375 11.0531 7.55313 10.9656 7.65938 10.7844Z"
												fill="#3897F0"
											/>
										</svg>
									</p>

									<textarea
										class="timeline-modal-input"
										rows="1"
										maxlength={TIMELINE_BODY_MAX}
										placeholder={tr('Apa yang baru?')}
										bind:value={timelineBody}
										bind:this={timelineInputEl}
										oninput={growTimelineInput}
										onpaste={handleTimelinePaste}
									></textarea>

							{#if timelineImagePreview}
								<div class="timeline-preview-wrap">
									<img src={timelineImagePreview} alt={tr('Pratinjau gambar postingan')} class="timeline-preview-image" />
									<button class="timeline-remove-image" type="button" aria-label={tr('Lepaskan gambar')} onclick={clearTimelineImage}>
										<X size={15} />
									</button>
									<span
										class="timeline-upload-badge"
										class:timeline-upload-badge-error={timelineUploadFailed}
									>
										{#if timelineUploading}
											<LoaderCircle size={12} class="animate-spin" />{tr('Mengunggah…')}
										{:else if timelineUploadFailed}
											{tr('Gagal, dicoba lagi saat Kirim')}
										{:else}
											<Check size={12} />{tr('Siap dikirim')}
										{/if}
									</span>
								</div>
							{/if}

								{#if timelineLink}
									<div class="timeline-link-card timeline-link-card-edit">
										<span class="timeline-link-body">
											<span class="timeline-link-site">{timelineLinkLoading ? tr('Memuat pratinjau…') : timelineLink.site || timelineLink.url}</span>
											<strong class="timeline-link-title">{timelineLink.title || timelineLink.url}</strong>
											{#if timelineLink.description}<span class="timeline-link-desc">{timelineLink.description}</span>{/if}
										</span>
										{#if timelineLink.image && !timelineLinkLoading}<img class="timeline-link-thumb" src={timelineLink.image} alt="" loading="lazy" />{/if}
										<button class="timeline-link-remove" type="button" aria-label={tr('Lepaskan tautan')} onclick={clearTimelineLink}>
											<X size={13} />
										</button>
									</div>
								{/if}
								</div>
							</div>

							<div class="timeline-modal-tools">
								<label class="timeline-tool timeline-file-tool" title={tr('Gambar')}>
									<ImagePlus size={17} />
									<span>{tr('Gambar')}</span>
									<input class="hidden" type="file" accept="image/png,image/jpeg,image/webp,image/gif" onchange={(e) => {
										attachTimelineImage((e.currentTarget as HTMLInputElement).files?.[0]);
										(e.currentTarget as HTMLInputElement).value = '';
									}} />
								</label>
								<span class="timeline-modal-hint">{tr('Tempel tautan atau seret gambar.')}</span>
							</div>

							{#if timelinePostError}
								<div class="timeline-error timeline-error-modal" role="alert">
									<span class="min-w-0 flex-1">{timelinePostError}</span>
								</div>
							{/if}
						</div>

						<footer class="timeline-modal-foot">
							<span class="timeline-char-count" class:timeline-char-limit={timelineBody.length >= TIMELINE_BODY_MAX}>{timelineBody.length}/{TIMELINE_BODY_MAX}</span>
							<button
								class="timeline-kirim"
								class:timeline-kirim-ready={Boolean(timelineBody.trim() || timelineImageFile || timelineLink)}
								type="button"
								disabled={timelinePosting || (!timelineBody.trim() && !timelineImageFile && !timelineLink)}
								onclick={() => void submitTimelinePost()}
							>
								{#if timelinePosting}<LoaderCircle size={14} class="animate-spin" />{/if}{tr('Kirim')}
							</button>
						</footer>
						{/if}
					</div>

					{#if timelineDiscardAsk}
						<!-- Konfirmasi sebelum isi yang belum terkirim hilang. -->
						<div class="timeline-confirm-backdrop" role="presentation" onclick={() => (timelineDiscardAsk = false)}></div>
						<div class="timeline-confirm" role="dialog" aria-modal="true" aria-label={tr('Simpan ke draf?')}>
							<strong class="timeline-confirm-title">{tr('Simpan ke draf?')}</strong>
							<p class="timeline-confirm-text">{tr('Simpan konsep untuk diedit dan diposting di lain waktu.')}</p>
							<button class="timeline-confirm-btn" type="button" onclick={() => void saveTimelineDraft()}>{tr('Simpan')}</button>
							<button class="timeline-confirm-btn timeline-confirm-danger" type="button" onclick={discardTimelineDraft}>{tr('Jangan simpan')}</button>
							<button class="timeline-confirm-btn" type="button" onclick={() => (timelineDiscardAsk = false)}>{tr('Batal')}</button>
						</div>
					{/if}
				{/if}
			{:else if activeTab === 'media'}
				<!-- ── Panel tab Media: dokumen & gambar brief ─────────────── -->
				<div class="min-h-0 flex-1 overflow-y-auto px-4 py-4 md:px-6">
					<div class="w-full space-y-4">
						<!-- Kotak unggah: jalur yang sama dengan lampiran Copilot. -->
						<section class="app-card p-4">
							<div
								class="rounded-xl border-2 border-dashed px-4 py-7 text-center transition-colors"
								class:cursor-pointer={!mediaUploading}
								style="border-color: {mediaDragging ? 'var(--primary)' : 'var(--border)'}; background: {mediaDragging ? 'color-mix(in srgb, var(--primary) 8%, transparent)' : 'color-mix(in srgb, var(--muted) 45%, white)'};"
								role="button"
								tabindex="0"
								aria-label={tr('Unggah berkas ke brief')}
								onclick={() => {
									if (!mediaUploading) document.getElementById('brief-media-input')?.click();
								}}
								onkeydown={(e) => {
									if (e.key === 'Enter' || e.key === ' ') {
										e.preventDefault();
										if (!mediaUploading) document.getElementById('brief-media-input')?.click();
									}
								}}
								ondragenter={onMediaDragEnter}
								ondragover={(e) => {
									if (hasFileDrag(e)) {
										e.preventDefault();
										e.stopPropagation();
									}
								}}
								ondragleave={onMediaDragLeave}
								ondrop={onMediaDrop}
							>
								<span class="mx-auto flex h-11 w-11 items-center justify-center rounded-xl" style="background: var(--accent); color: var(--accent-foreground);">
									{#if mediaUploading}<LoaderCircle size={20} class="animate-spin" />{:else}<Upload size={20} />{/if}
								</span>
								<p class="mt-3 text-sm font-semibold">
									{mediaUploading ? tr('Mengunggah…') : tr('Seret berkas ke sini atau klik untuk mengunggah')}
								</p>
								<p class="mt-1 text-[11px] text-muted-foreground">
									{tr('Dokumen PDF, DOCX, TXT, MD, CSV · Gambar JPG, PNG, WebP · maksimal 10 MB per berkas')}
								</p>
								<input
									id="brief-media-input"
									class="hidden"
									type="file"
									multiple
									accept=".pdf,.docx,.txt,.md,.csv,application/pdf,image/png,image/jpeg,image/webp,image/gif"
									onclick={(e) => e.stopPropagation()}
									onchange={(e) => {
										const input = e.currentTarget as HTMLInputElement;
										void uploadMediaFiles(input.files);
										input.value = '';
									}}
								/>
							</div>
							{#if mediaError}
								<p class="mt-3 rounded-lg p-2 text-xs" style="background: var(--warning-soft); color: var(--warning);">{mediaError}</p>
							{/if}
						</section>

						{#if mediaLoading && mediaImages.length + mediaDocs.length === 0}
							<p class="flex items-center gap-2 text-xs text-muted-foreground">
								<LoaderCircle size={14} class="animate-spin" />{tr('Memuat media…')}
							</p>
						{/if}

						<section class="app-card p-5">
							<div class="flex flex-wrap items-center justify-between gap-2 border-b pb-4" style="border-color: var(--border);">
								<div class="flex items-center gap-3">
									<span class="flex h-9 w-9 items-center justify-center rounded-xl" style="background: var(--accent); color: var(--accent-foreground);">
										<Images size={18} />
									</span>
									<div>
										<h3 class="text-sm font-bold">{tr('Gambar')}</h3>
										<p class="text-[11px] text-muted-foreground">{tr('Aset visual brand dari Copilot atau tab ini.')}</p>
									</div>
								</div>
								<span class="rounded-full px-2.5 py-0.5 text-[11px] font-bold" style="background: var(--secondary); color: var(--muted-foreground);">{mediaImages.length}</span>
							</div>
							{#if mediaImages.length > 0}
								<div class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-4">
									{#each mediaImages as im (im.id)}
										<figure class="group relative overflow-hidden rounded-xl border" style="border-color: var(--border);">
											<a href={im.url} target="_blank" rel="noreferrer" class="block">
												<img src={im.url} alt={im.file_name ?? ''} class="h-32 w-full object-cover" loading="lazy" />
											</a>
											<button
												class="absolute right-1.5 top-1.5 rounded-lg p-1.5 opacity-0 shadow-sm transition-opacity group-hover:opacity-100 focus-visible:opacity-100"
												style="background: var(--card); color: var(--destructive);"
												type="button"
												aria-label={tr('Hapus gambar')}
												onclick={() => removeMediaImage(im)}
											>
												<Trash2 size={13} />
											</button>
											<figcaption class="px-2.5 py-2">
												<p class="truncate text-[11px] font-medium" title={im.file_name ?? ''}>{im.file_name ?? tr('Gambar')}</p>
												<p class="mt-0.5 text-[10px] text-muted-foreground">{im.size_bytes ? fmtSize(im.size_bytes) : '-'} · {mediaDate(im.created_at)}</p>
											</figcaption>
										</figure>
									{/each}
								</div>
							{:else}
								<p class="mt-4 rounded-xl border px-3.5 py-3 text-xs italic" style="border-color: var(--border); color: var(--muted-foreground);">
									{tr('Belum ada gambar. Unggah lewat kotak di atas atau lampirkan saat mengobrol dengan Copilot.')}
								</p>
							{/if}
						</section>

						<section class="app-card p-5">
							<div class="flex flex-wrap items-center justify-between gap-2 border-b pb-4" style="border-color: var(--border);">
								<div class="flex items-center gap-3">
									<span class="flex h-9 w-9 items-center justify-center rounded-xl" style="background: var(--accent); color: var(--accent-foreground);">
										<FileText size={18} />
									</span>
									<div>
										<h3 class="text-sm font-bold">{tr('Dokumen')}</h3>
										<p class="text-[11px] text-muted-foreground">{tr('Referensi yang dibaca AI saat menyusun brief.')}</p>
									</div>
								</div>
								<span class="rounded-full px-2.5 py-0.5 text-[11px] font-bold" style="background: var(--secondary); color: var(--muted-foreground);">{mediaDocs.length}</span>
							</div>
							{#if mediaDocs.length > 0}
								<ul class="mt-4 space-y-2">
									{#each mediaDocs as doc (doc.id)}
										<li class="flex items-center gap-3 rounded-xl border p-3" style="border-color: var(--border); background: color-mix(in srgb, var(--muted) 55%, white);">
											<span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg" style="background: var(--accent); color: var(--accent-foreground);">
												<FileText size={16} />
											</span>
											<div class="min-w-0 flex-1">
												<p class="truncate text-xs font-semibold" title={doc.original_filename}>{doc.original_filename}</p>
												<p class="mt-0.5 flex flex-wrap items-center gap-1.5 text-[10px] text-muted-foreground">
													<span>{doc.size_bytes ? fmtSize(doc.size_bytes) : '-'}</span>
													<span>·</span>
													<span>{mediaDate(doc.created_at)}</span>
												{#if doc.parse_status === 'ready'}
													<span class="rounded px-1.5 py-0.5 font-bold" style="background: var(--success-soft); color: var(--success);">{tr('Terbaca AI')}</span>
												{:else if doc.parse_status === 'failed'}
													<span class="rounded px-1.5 py-0.5 font-bold" style="background: var(--warning-soft); color: var(--warning);" title={doc.parse_error ?? ''}>{tr('Gagal dibaca')}</span>
												{/if}
												{#if doc.file_deleted_at}
													<span class="rounded px-1.5 py-0.5 font-bold" style="background: var(--secondary); color: var(--muted-foreground);" title={tr('Berkas fisik dibersihkan otomatis; isinya tetap diingat AI.')}>{tr('Berkas dibersihkan')}</span>
												{/if}
											</p>
										</div>
										{#if doc.url}
											<a class="brief-icon-btn" href={doc.url} target="_blank" rel="noreferrer" aria-label={tr('Buka dokumen')} title={tr('Buka dokumen')}>
												<ExternalLink size={15} />
											</a>
										{/if}
											<button class="brief-icon-btn" type="button" aria-label={tr('Hapus dokumen')} title={tr('Hapus dokumen')} onclick={() => removeMediaDoc(doc)}>
												<Trash2 size={15} />
											</button>
										</li>
									{/each}
								</ul>
							{:else}
								<p class="mt-4 rounded-xl border px-3.5 py-3 text-xs italic" style="border-color: var(--border); color: var(--muted-foreground);">
									{tr('Belum ada dokumen. Unggah lewat kotak di atas atau lampirkan saat mengobrol dengan Copilot.')}
								</p>
							{/if}
						</section>
					</div>
			</div>
			{:else}
				<!-- ── Panel tab Ringkasan: hasil agent entitas & file ─────── -->
				<div class="min-h-0 flex-1 overflow-y-auto px-4 py-4 md:px-6">
					<div class="w-full space-y-4">
						<section class="app-card p-4">
							<div class="flex items-center gap-3">
								<span class="flex h-9 w-9 items-center justify-center rounded-xl" style="background: var(--accent); color: var(--accent-foreground);">
									<Sparkle size={18} />
								</span>
								<div>
									<h3 class="text-sm font-bold">{tr('Ringkasan Brief')}</h3>
									<p class="text-[11px] text-muted-foreground">{tr('Fakta brand yang dipakai AI Writer saat menulis artikel.')}</p>
								</div>
							</div>
						</section>

						<div class="flex flex-wrap items-center gap-1.5" role="tablist">
							<button
								class="brief-tab"
								class:brief-tab-active={summaryTab === 'entity'}
								type="button"
								role="tab"
								aria-selected={summaryTab === 'entity'}
								onclick={() => (summaryTab = 'entity')}
							>
								<UserCircle size={14} />{tr('Ringkasan Entitas')}
							</button>
							<button
								class="brief-tab"
								class:brief-tab-active={summaryTab === 'file'}
								type="button"
								role="tab"
								aria-selected={summaryTab === 'file'}
								onclick={() => (summaryTab = 'file')}
							>
								<FileText size={14} />{tr('Ringkasan File')}
							</button>
						</div>

						{#if summaryError}
							<p class="rounded-lg p-2 text-xs" style="background: var(--warning-soft); color: var(--warning);">{summaryError}</p>
						{/if}

						<section class="app-card p-5">
							<div class="flex flex-wrap items-start justify-between gap-3 border-b pb-4" style="border-color: var(--border);">
								<div class="min-w-0">
									<div class="flex flex-wrap items-center gap-2">
										<h3 class="text-sm font-bold">{summaryTab === 'entity' ? tr('Ringkasan Entitas') : tr('Ringkasan File')}</h3>
										<span class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-[10px] font-bold" style="background: {summaryBadge.bg}; color: {summaryBadge.color};">
											<span class="h-1.5 w-1.5 rounded-full" style="background: {summaryBadge.color};"></span>{summaryBadge.label}
										</span>
									</div>
									<p class="mt-1 text-[11px] text-muted-foreground">
										{summaryTab === 'entity'
											? tr('Identitas dan fakta resmi dari profil brief.')
											: tr('Isi dokumen, tautan, dan catatan gambar brief.')}
									</p>
								</div>
								<div class="flex flex-wrap items-center gap-2">
									<button class="app-btn app-btn-soft !min-h-8 !text-xs" type="button" onclick={() => void copySummary(summaryCurrent)} disabled={!summaryCurrent.content}>
										<Copy size={14} />{tr('Salin')}
									</button>
									{#if summaryCurrent.status !== 'queued' && summaryCurrent.status !== 'running'}
										<button
											class="app-btn app-btn-primary !min-h-8 !text-xs"
											type="button"
											onclick={() => void regenerateSummary(summaryTab)}
											disabled={summaryBusy !== null}
										>
											{#if summaryBusy === summaryTab}
												<LoaderCircle size={14} class="animate-spin" />
											{:else}
												<RotateCcw size={14} />
											{/if}
											{summaryActionLabel}
										</button>
									{/if}
								</div>
							</div>

							{#if summaryCurrent.status === 'queued' || summaryCurrent.status === 'running'}
								<div class="mt-4 flex items-center gap-2 rounded-xl border px-4 py-3 text-xs" style="border-color: var(--border); background: color-mix(in srgb, var(--muted) 55%, white);">
									<LoaderCircle size={15} class="animate-spin" style="color: var(--primary);" />
									{tr('Agent sedang menyusun ringkasan dari data brief. Hasil muncul otomatis di sini.')}
								</div>
							{:else if summaryCurrent.status === 'failed'}
								<div class="mt-4 rounded-xl border p-4 text-xs" style="border-color: color-mix(in srgb, var(--destructive) 30%, transparent); background: color-mix(in srgb, var(--destructive) 7%, white);">
									<p class="font-bold" style="color: var(--destructive);">{tr('Ringkasan gagal dibuat')}</p>
									<p class="mt-1 leading-relaxed" style="color: var(--destructive);">{summaryCurrent.error || tr('Tidak ada pesan error.')}</p>
									<p class="mt-1 text-muted-foreground">{tr('Coba lagi sebentar lagi.')}</p>
								</div>
							{:else if summaryCurrent.content}
								<div class="brief-summary-body mt-4 text-sm leading-relaxed">
									{#if summaryTab === 'entity'}
										<!-- Ringkasan entitas = kumpulan field kanonik. Dirender
										     sebagai bagian berjudul + baris label/nilai, bukan dump
										     teks mentah berisi nama field snake_case. -->
										<BriefSummaryView content={summaryCurrent.content} />
									{:else}
										<div class="brief-markdown">
											{@html sanitizeHtml(renderMarkdown(summaryCurrent.content))}
										</div>
									{/if}
								</div>
							{:else}
								<p class="mt-4 rounded-xl border px-3.5 py-3 text-xs italic" style="border-color: var(--border); color: var(--muted-foreground);">
									{tr('Belum ada ringkasan. Klik "Buat ringkasan" untuk menyusunnya dari data brief terbaru.')}
								</p>
							{/if}

							<div class="mt-4 flex flex-wrap items-center gap-x-4 gap-y-1 border-t pt-3 text-[11px] text-muted-foreground" style="border-color: var(--border);">
								{#if summaryCurrent.generated_at}<span>{tr('Dibuat')}: {summaryDate(summaryCurrent.generated_at)}</span>{/if}
								{#if summaryCurrent.source_chars > 0}
									<span>{summaryCurrent.source_chars.toLocaleString('id-ID')} {tr('karakter bahan')} → {summaryCurrent.content.length.toLocaleString('id-ID')} {tr('karakter ringkasan')}</span>
								{/if}
								{#if summaryCurrent.source_chars > 0 && summaryCurrent.content}
									<span style="color: var(--success); font-weight: 650;">
										{tr('Hemat')} {Math.max(0, Math.round((1 - summaryCurrent.content.length / summaryCurrent.source_chars) * 100))}%
									</span>
								{/if}
								{#if summaryCurrent.stale && summaryCurrent.status === 'ready'}
									<span style="color: var(--warning);">{tr('Data brief berubah sejak ringkasan ini dibuat.')}</span>
								{/if}
							</div>
						</section>

						<!-- Riwayat perubahan: terlipat, dimuat saat dibuka. -->
						<section class="app-card">
							<button
								class="history-toggle"
								type="button"
								aria-expanded={historyOpen}
								onclick={() => toggleHistory(!historyOpen)}
							>
								<span class="flex items-center gap-2">
									<History size={15} style="color: var(--primary);" />
									<span class="text-sm font-bold">{tr('Riwayat perubahan')}</span>
								</span>
								<span class={historyOpen ? 'history-chevron open' : 'history-chevron'}>
									<ChevronDown size={15} />
								</span>
							</button>

							{#if historyOpen}
								<div class="border-t px-4 pb-4 pt-3" style="border-color: var(--border);">
									<div class="flex flex-wrap items-center justify-between gap-2">
										<p class="text-[11.5px] text-muted-foreground">{tr('Catatan perubahan field brief, terbaru dulu.')}</p>
										<Select
											bind:value={historyFilter}
											options={historyOptions}
											ariaLabel={tr('Saring riwayat berdasarkan field')}
											minWidth={220}
											onchange={(value) => { if (historyOpen) void loadHistory(value); }}
										/>
									</div>

									{#if historyLoading}
										<p class="mt-3 flex items-center gap-2 text-xs text-muted-foreground">
											<LoaderCircle size={14} class="animate-spin" /> {tr('Memuat riwayat…')}
										</p>
									{:else if historyError}
										<div class="mt-3 flex flex-wrap items-center gap-2 text-xs" style="color: var(--destructive);">
											<span>{historyError}</span>
											<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-[11.5px]" type="button" onclick={() => void loadHistory()}>{tr('Coba lagi')}</button>
										</div>
									{:else if historyEntries.length === 0}
										<p class="mt-3 text-xs italic text-muted-foreground">{tr('Belum ada perubahan tercatat untuk filter ini.')}</p>
									{:else}
										<ul class="mt-2 divide-y" style="border-color: var(--border);">
											{#each historyEntries as entry, i (`${entry.field_key}-${entry.changed_at}-${i}`)}
												<li class="py-2.5">
													<div class="flex flex-wrap items-baseline justify-between gap-2">
														<span class="text-[12.5px] font-semibold">{historyFieldLabel(entry.field_key)}</span>
														<span class="text-[11px] text-muted-foreground">{timelineRelativeTime(entry.changed_at)}</span>
													</div>
													<p class="mt-1 text-[12px] leading-relaxed">
														<span class="text-muted-foreground line-through">{truncateHistoryValue(entry.old_value)}</span>
														<span class="mx-1.5 text-muted-foreground">→</span>
														<span style="color: var(--foreground);">{truncateHistoryValue(entry.new_value)}</span>
													</p>
													{#if entry.changed_by_name}
														<p class="mt-0.5 text-[11px] text-muted-foreground">{tr('oleh')} {entry.changed_by_name}</p>
													{/if}
												</li>
											{/each}
										</ul>
									{/if}
								</div>
							{/if}
						</section>
					</div>
				</div>
			{/if}
	</main>

	<!-- ── Modal simpan sebagai template (dari menu kartu brief) ─────── -->
	{#if templateSaveFor}
		<div class="brief-modal-overlay" role="presentation">
			<button
				class="brief-modal-backdrop"
				type="button"
				aria-label={tr('Tutup')}
				onclick={() => (templateSaveFor = null)}
			></button>
			<div
				class="brief-modal"
				role="dialog"
				aria-modal="true"
				aria-labelledby="brief-template-title"
			>
				<form onsubmit={saveAsTemplate}>
					<h2 id="brief-template-title" class="brief-modal-title">{tr('Simpan sebagai template')}</h2>
					<p class="brief-modal-text">
						{tr('Isi brief ini disalin menjadi template. Template bisa dipakai saat membuka brief baru untuk brand ini.')}
					</p>
					<div class="mt-3 grid gap-3">
						<label class="flex flex-col gap-1 text-[12.5px]">
							<span class="font-medium" style="color: var(--muted-foreground);">{tr('Nama template')}</span>
							<input class="app-input" bind:value={templateSaveName} required minlength={2} maxlength={160} />
						</label>
						<label class="flex flex-col gap-1 text-[12.5px]">
							<span class="font-medium" style="color: var(--muted-foreground);">{tr('Deskripsi (opsional)')}</span>
							<textarea
								class="app-input"
								rows="2"
								maxlength={300}
								bind:value={templateSaveDesc}
								style="min-height: 60px; resize: vertical;"
							></textarea>
						</label>
					</div>
					{#if templateSaveError}
						<p class="mt-2 text-[12px]" style="color: var(--destructive);" role="alert">{templateSaveError}</p>
					{/if}
					<div class="brief-modal-actions">
						<button class="app-btn app-btn-soft" type="button" onclick={() => (templateSaveFor = null)}>
							{tr('Batal')}
						</button>
						<button class="app-btn" type="submit" disabled={templateSaveBusy}>
							{templateSaveBusy ? tr('Menyimpan…') : tr('Simpan template')}
						</button>
					</div>
				</form>
			</div>
		</div>
	{/if}

	<!-- ── Dialog konfirmasi (hapus brief / percakapan baru) ─────────── -->
	{#if confirmReq}
		<div class="brief-modal-overlay" role="presentation">
			<!-- Backdrop berupa tombol, pola yang sama dengan katalog media; kartu
			     dialog diletakkan di atasnya. -->
			<button
				class="brief-modal-backdrop"
				type="button"
				aria-label={tr('Tutup')}
				onclick={() => (confirmReq = null)}
			></button>

			<div
				class="brief-modal"
				role="dialog"
				aria-modal="true"
				aria-labelledby="brief-confirm-title"
				tabindex="-1"
				bind:this={confirmEl}
				onkeydown={(e) => {
					if (e.key === 'Escape') confirmReq = null;
				}}
			>
				<h2 id="brief-confirm-title" class="brief-modal-title">{confirmReq.title}</h2>
				<p class="brief-modal-text">{confirmReq.body}</p>
				<div class="brief-modal-actions">
					<button class="app-btn app-btn-soft" type="button" onclick={() => (confirmReq = null)}>
						{tr('Batal')}
					</button>
					<button
						class="app-btn"
						class:brief-modal-danger={confirmReq.danger}
						type="button"
						onclick={() => {
							const action = confirmReq?.action;
							confirmReq = null;
							if (action) void action();
						}}
					>
						{confirmReq.confirmLabel}
					</button>
				</div>
			</div>
		</div>
	{/if}

</div>

<style>
	/* Sidebar riwayat: permukaan lembut, jejak aksen, ringan — sengaja berbeda
	   dari sidebar navigasi aplikasi yang putih dan berat. */
	.brief-rail {
		background: var(--muted);
	}

	.brief-rail-handle {
		position: absolute;
		right: 0;
		top: 0;
		bottom: 0;
		width: 8px;
		cursor: col-resize;
		z-index: 5;
	}
	.brief-rail-handle::after {
		content: '';
		position: absolute;
		right: 0;
		top: 0;
		bottom: 0;
		width: 3px;
		background: transparent;
		transition: background 0.12s ease;
	}
	.brief-rail-handle:hover::after { background: var(--primary); }
	/* Di layar sempit sidebar berperan drawer: lebarnya tetap, pegangan disembunyikan. */
	@media (max-width: 1023px) {
		.brief-rail { width: 270px !important; }
		.brief-rail-handle { display: none; }
	}

	/* Tanda brand di baris atas rel — kotak aksen padat, padanan lencana logo
	   di desain referensi. */
	.brief-mark {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border-radius: 10px;
		background: var(--primary);
		color: #fff;
		box-shadow: 0 1px 2px rgba(23, 43, 77, 0.18);
	}

	/* Avatar kotak di sisi gelembung percakapan (padanan desain referensi):
	   AI bergradasi aksen, pengguna gelap netral. */
	.brief-avatar {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		flex-shrink: 0;
		border-radius: 10px;
		box-shadow: 0 1px 2px rgba(23, 43, 77, 0.12);
	}
	/* Avatar AI memakai maskot robot transparan; gambar aslinya lebih lebar
	   dari kotak avatar, jadi object-fit contain supaya tidak kepangkas. */
	.brief-avatar-ai { background: transparent; overflow: hidden; }
	.brief-avatar-ai img {
		display: block;
		width: 100%;
		height: 100%;
		border-radius: inherit;
		object-fit: contain;
	}
	/* Avatar pengguna: inisial nama dari sesi (app ini tidak menyimpan foto profil). */
	.brief-avatar-user {
		background: var(--accent);
		border: 1px solid color-mix(in srgb, var(--primary) 18%, transparent);
		color: var(--accent-foreground);
		font-size: 11.5px;
		font-weight: 700;
		letter-spacing: 0.02em;
	}

	.brief-icon-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 30px;
		height: 30px;
		border-radius: 50%;
		color: var(--muted-foreground);
		transition: background-color 0.12s ease, color 0.12s ease;
	}
	.brief-icon-btn:hover { background: var(--card); color: var(--foreground); }
	.brief-icon-btn.active { background: var(--card); color: var(--primary); }

	/* Popover info Beta: penjelasan prarilis, warna peringatan supaya tidak
	   terbaca sebagai status produk yang sudah matang. */
	/* Lencana Beta di baris status: penanda fitur prarilis, warna peringatan. */
	.brief-beta-chip {
		display: inline-flex;
		align-items: center;
		padding: 1px 8px;
		border: 1px solid color-mix(in srgb, var(--warning) 28%, transparent);
		border-radius: 999px;
		background: var(--warning-soft);
		color: var(--warning);
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.03em;
		transition: background-color 0.12s ease, border-color 0.12s ease;
	}
	.brief-beta-chip:hover,
	.brief-beta-chip.active {
		border-color: color-mix(in srgb, var(--warning) 45%, transparent);
		background: color-mix(in srgb, var(--warning) 16%, transparent);
	}
	.brief-beta-backdrop {
		position: fixed;
		inset: 0;
		z-index: 40;
		cursor: default;
	}
	.brief-beta-pop {
		position: absolute;
		top: calc(100% + 8px);
		right: 0;
		z-index: 50;
		width: 272px;
		padding: 10px 12px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--card, #fff);
		box-shadow: 0 12px 28px rgba(15, 23, 42, 0.14);
		text-align: left;
	}
	.brief-beta-pop-title {
		font-size: 12px;
		font-weight: 700;
		color: var(--foreground);
	}
	.brief-beta-pop-text {
		margin-top: 4px;
		font-size: 12px;
		line-height: 1.5;
		color: var(--muted-foreground);
	}

	.brief-primary {
		display: flex;
		align-items: center;
		gap: 10px;
		width: 100%;
		padding: 9px 12px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--card);
		font-size: 14px;
		font-weight: 600;
		color: var(--foreground);
		box-shadow: var(--shadow-sm);
		transition: border-color 0.12s ease, background-color 0.12s ease;
	}
	.brief-primary :global(svg) { color: var(--primary); }
	.brief-primary:hover { border-color: color-mix(in srgb, var(--primary) 45%, var(--border)); }

	.brief-summary {
		width: 100%;
		padding: 9px 11px;
		border-radius: 12px;
		border: 1px solid var(--border);
		background: var(--card);
		text-align: left;
		transition: border-color 0.12s ease, box-shadow 0.12s ease;
	}
	.brief-summary:hover {
		border-color: var(--primary);
		box-shadow: 0 4px 14px rgba(15, 23, 42, 0.07);
	}

	.brief-item {
		position: relative;
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 8px;
		margin: 1px 6px;
		border-radius: 8px;
		color: var(--foreground);
	}
	/* Rel berlatar lembut: var(--secondary) sama persis dengan latar rel sehingga
	   hover lama tak terlihat — dipakai lapisan gelap tipis sebagai gantinya.
	   Item aktif naik jadi kartu putih bergaris rambut. */
	.brief-item:hover { background: color-mix(in srgb, var(--foreground) 6%, transparent); }
	.brief-item[aria-current='page'] {
		background: var(--card);
		box-shadow: var(--shadow-sm);
	}
	.brief-item[aria-current='page'] .brief-item-title { font-weight: 600; }
	.brief-item-open { z-index: 40; }

	.brief-pin {
		flex-shrink: 0;
		padding: 3px;
		border-radius: 6px;
		color: var(--muted-foreground);
		opacity: 0;
		transition: opacity 0.12s ease, color 0.12s ease;
	}
	.brief-item:hover .brief-pin,
	.brief-item:focus-within .brief-pin,
	.brief-item-open .brief-pin { opacity: 1; }
	.brief-pin.pinned { opacity: 1; color: var(--primary); }
	.brief-pin:hover { background: color-mix(in srgb, var(--foreground) 8%, transparent); }
	@media (hover: none) {
		.brief-pin { opacity: 0.75; }
	}

	.brief-more {
		flex-shrink: 0;
		padding: 3px;
		border-radius: 6px;
		color: var(--muted-foreground);
		opacity: 0;
		transition: opacity 0.12s ease;
	}
	.brief-item:hover .brief-more,
	.brief-item:focus-within .brief-more,
	.brief-item-open .brief-more { opacity: 1; }
	.brief-more:hover { background: color-mix(in srgb, var(--foreground) 8%, transparent); color: var(--foreground); }

	.brief-menu {
		position: absolute;
		right: 6px;
		top: calc(100% - 2px);
		z-index: 50;
		min-width: 150px;
		padding: 4px;
		border-radius: 10px;
		border: 1px solid var(--border);
		background: var(--card);
		box-shadow: 0 10px 26px rgba(15, 23, 42, 0.14);
	}
	.brief-menu button {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		padding: 6px 8px;
		border-radius: 6px;
		font-size: 12px;
		text-align: left;
	}
	.brief-menu button:hover { background: var(--secondary); }
	.brief-menu-danger { color: var(--destructive); }



	/* Riwayat perubahan: baris pembuka datar selebar kartu, isi di bawahnya. */
	.history-toggle {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		width: 100%;
		padding: 12px 16px;
		border: none;
		border-radius: 12px;
		background: transparent;
		color: var(--foreground);
		text-align: left;
		cursor: pointer;
	}
	.history-toggle:hover {
		background: color-mix(in srgb, var(--secondary) 60%, transparent);
	}
	.history-chevron {
		display: inline-flex;
		color: var(--muted-foreground);
		transition: transform 160ms ease;
	}
	.history-chevron.open {
		transform: rotate(180deg);
	}

	.brief-modal-overlay {
		position: fixed;
		inset: 0;
		z-index: 60;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 20px;
		background: rgba(15, 23, 42, 0.45);
	}
	.brief-modal-backdrop {
		position: absolute;
		inset: 0;
		cursor: default;
	}
	.brief-modal {
		position: relative;
		width: 100%;
		max-width: 390px;
		padding: 18px 18px 14px;
		border-radius: 16px;
		border: 1px solid var(--border);
		background: var(--card);
		box-shadow: 0 20px 48px rgba(15, 23, 42, 0.22);
		outline: none;
	}
	.brief-modal-title { font-size: 15px; font-weight: 600; }
	.brief-modal-text {
		margin-top: 6px;
		font-size: 13px;
		line-height: 1.55;
		color: var(--muted-foreground);
	}
	.brief-modal-actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		margin-top: 16px;
	}
	.brief-modal-danger {
		background: var(--destructive);
		color: #fff;
	}
	/* Tombol aksi utama modal biru; tombol "Tutup" (soft) tetap netral abu. */
	.brief-modal-actions .app-btn:not(.brief-modal-danger):not(.app-btn-soft) {
		background: var(--primary);
		color: var(--primary-foreground);
	}
	.brief-modal-danger:hover { opacity: 0.92; }

	/* Wadah hasil ringkasan: kartu lembut dengan batas tipis supaya isinya
	   terbaca sebagai dokumen, bukan teks polos menumpuk. */
	.brief-summary-body {
		padding: 14px 16px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: color-mix(in srgb, var(--muted) 45%, white);
		max-height: 62vh;
		overflow-y: auto;
	}

	/* Isi dalam .brief-markdown berasal dari {@html} hasil renderMarkdown, jadi
	   penyeleksinya harus :global — Svelte tidak bisa melihat elemen yang ada
	   di dalam string HTML. */
	.brief-markdown :global(p) { margin: 0 0 0.5rem; }
	.brief-markdown :global(p:last-child) { margin-bottom: 0; }
	/* Tailwind Preflight mematikan list-style & padding bawaan daftar. Tanpa
	   padding-left penandanya menempel/terpotong — ini yang membuat ringkasan
	   berpoin tampak rata kiri. */
	.brief-markdown :global(ul),
	.brief-markdown :global(ol) {
		margin: 0.35rem 0 0.65rem;
		padding-left: 1.5rem;
	}
	.brief-markdown :global(ul) { list-style: disc; }
	.brief-markdown :global(ol) { list-style: decimal; }
	.brief-markdown :global(ul ul) { list-style: circle; margin: 0.2rem 0; }
	.brief-markdown :global(li) { margin: 0.25rem 0; padding-left: 0.25rem; }
	.brief-markdown :global(li)::marker { color: var(--muted-foreground); }
	.brief-markdown :global(li > p) { margin: 0 0 0.25rem; }
	/* Judul kecil di dalam ringkasan jadi pemisah bagian yang jelas. */
	.brief-markdown :global(h3),
	.brief-markdown :global(h4) { color: var(--foreground); margin-bottom: 0.35rem; }
	.brief-markdown :global(code) {
		padding: 0.05rem 0.3rem;
		border-radius: 6px;
		background: var(--secondary);
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
		font-size: 0.85em;
	}
	.brief-markdown :global(a) {
		text-decoration: underline;
		text-underline-offset: 2px;
	}
	.brief-markdown :global(strong) { font-weight: 650; }

	/* Kutipan: garis aksen kiri + latar lembut, untuk kutipan narasumber. */
	.brief-markdown :global(blockquote) {
		margin: 0.55rem 0;
		padding: 8px 12px;
		border-left: 3px solid var(--primary);
		border-radius: 0 8px 8px 0;
		background: var(--secondary);
	}

	/* Tabel: untuk ringkasan field yang lebih jernih dibanding paragraf. */
	.brief-markdown :global(table) {
		width: 100%;
		margin: 0.6rem 0;
		border-collapse: collapse;
		font-size: 0.94em;
	}
	.brief-markdown :global(th),
	.brief-markdown :global(td) {
		padding: 6px 10px;
		border: 1px solid var(--border);
		text-align: left;
		vertical-align: top;
	}
	.brief-markdown :global(thead th) {
		background: var(--secondary);
		font-weight: 600;
	}

	/* Blok kode berpagar. */
	.brief-markdown :global(pre) {
		margin: 0.6rem 0;
		padding: 10px 12px;
		border-radius: 10px;
		background: var(--secondary);
		overflow-x: auto;
	}
	.brief-markdown :global(pre code) {
		padding: 0;
		background: transparent;
		font-size: 0.86em;
		white-space: pre;
	}

	.brief-markdown :global(hr) {
		margin: 0.85rem 0;
		border: none;
		border-top: 1px solid var(--border);
	}
	.brief-markdown :global(h3) {
		margin: 0.7rem 0 0.35rem;
		font-size: 1.02em;
		font-weight: 700;
	}
	.brief-markdown :global(h4) {
		margin: 0.6rem 0 0.3rem;
		font-size: 0.97em;
		font-weight: 650;
	}
	.brief-markdown :global(del) { opacity: 0.65; }
	.brief-markdown :global(li) { margin: 0.15rem 0; }

	/* Popup ajakan ringkas di atas composer Copilot. */
	.brief-summary-nudge-wrap {
		padding: 8px 12px 0;
		animation: brief-nudge-in 0.26s cubic-bezier(0.16, 1, 0.3, 1);
	}
	.brief-summary-nudge {
		display: flex;
		align-items: center;
		gap: 10px;
		max-width: 42rem;
		margin: 0 auto;
		padding: 10px 12px;
		border: 1px solid color-mix(in srgb, var(--primary) 22%, var(--border));
		border-radius: 14px;
		background: color-mix(in srgb, var(--primary) 6%, white);
		box-shadow: 0 6px 18px rgba(15, 23, 42, 0.06);
	}
	.brief-nudge-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 auto;
		width: 32px;
		height: 32px;
		border-radius: 999px;
		background: color-mix(in srgb, var(--primary) 12%, white);
		color: var(--primary);
	}
	.brief-nudge-ok {
		background: var(--success-soft);
		color: var(--success);
		animation: brief-nudge-pop 0.4s cubic-bezier(0.34, 1.56, 0.64, 1);
	}
	.brief-nudge-err {
		background: var(--warning-soft);
		color: var(--warning);
	}
	.brief-nudge-spin :global(svg) { animation: brief-nudge-spin 0.9s linear infinite; }
	/* Bar progres tak-tentu: memberi kesan pekerjaan berjalan tanpa angka palsu. */
	.brief-nudge-bar {
		position: relative;
		height: 3px;
		overflow: hidden;
		border-radius: 999px;
		background: color-mix(in srgb, var(--primary) 15%, white);
	}
	.brief-nudge-bar span {
		position: absolute;
		inset-block: 0;
		width: 40%;
		border-radius: 999px;
		background: var(--primary);
		animation: brief-nudge-slide 1.1s ease-in-out infinite;
	}
	@keyframes brief-nudge-in {
		from { opacity: 0; transform: translateY(6px); }
		to { opacity: 1; transform: translateY(0); }
	}
	@keyframes brief-nudge-pop {
		0% { transform: scale(0.6); }
		60% { transform: scale(1.12); }
		100% { transform: scale(1); }
	}
	@keyframes brief-nudge-spin { to { transform: rotate(360deg); } }
	@keyframes brief-nudge-slide {
		0% { left: -40%; }
		100% { left: 100%; }
	}
	@media (prefers-reduced-motion: reduce) {
		.brief-summary-nudge-wrap,
		.brief-nudge-ok,
		.brief-nudge-spin :global(svg) { animation: none; }
		.brief-nudge-bar span { animation: none; left: 0; width: 100%; }
	}

	/* Composer dua area: teks di atas, toolbar SATU baris di bawah.
	   Tinggi satu baris: 16+14 padding + 2 border + 21 teks + 8 jarak + 36 toolbar ≈ 97px. */
	.brief-composer {
		display: flex;
		flex-direction: column;
		padding: 16px 14px 14px;
		border: 1px solid var(--border, #e2e8f0);
		border-radius: 20px;
		background: #ffffff;
		box-shadow: 0 1px 3px rgba(15, 23, 42, 0.04);
		transition: border-color 0.15s ease, box-shadow 0.15s ease;
	}
	.brief-composer:focus-within {
		border-color: var(--primary, #2b67ec);
		box-shadow: 0 0 0 3px rgba(43, 103, 236, 0.1);
	}
	.brief-composer-files {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		margin-bottom: 8px;
	}
	.brief-file-chip {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		max-width: 100%;
		padding: 4px 6px 4px 8px;
		border-radius: 10px;
		border: 1px solid var(--border, #e2e8f0);
		background: var(--secondary, #f8fafc);
		font-size: 12px;
	}
	.brief-composer-input {
		display: block;
		box-sizing: border-box;
		width: 100%;
		margin: 0;
		padding: 0 2px;
		min-height: 21px;
		max-height: 50vh;
		border: none;
		outline: none;
		background: transparent;
		color: var(--foreground, #0f172a);
		font: inherit;
		font-size: 14.5px;
		line-height: 21px;
		resize: none;
		overflow-y: hidden;
		transition: height 0.18s ease;
	}
	.brief-composer-input::placeholder { color: var(--muted-foreground, #94a3b8); }
	/* Toolbar tunggal di dasar composer: + dan status di kiri,
	   berpikir/mikrofon/kirim di kanan. */
	.brief-composer-toolbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		margin-top: 8px;
	}
	.brief-composer-tools {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
	}
	.brief-composer-uploading {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 11.5px;
		color: var(--muted-foreground, #64748b);
	}
	.brief-composer-actions {
		display: flex;
		align-items: center;
		gap: 4px;
	}
	.brief-composer-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 auto;
		width: 36px;
		height: 36px;
		padding: 0;
		border-radius: 999px;
		color: var(--muted-foreground, #64748b);
		background: transparent;
		border: none;
		transition: background-color 0.12s ease, color 0.12s ease;
	}
	/* Popover lampiran: mengapung ke atas dari tombol tambah. */
	.brief-attach {
		position: relative;
		flex: 0 0 auto;
	}
	.brief-attach-pop {
		position: absolute;
		left: -6px;
		bottom: calc(100% + 12px);
		z-index: 50;
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 176px;
		padding: 6px;
		border: 1px solid var(--border, #e2e8f0);
		border-radius: 14px;
		background: var(--card, #ffffff);
		box-shadow: 0 12px 32px rgba(15, 23, 42, 0.14), 0 2px 6px rgba(15, 23, 42, 0.06);
		animation: brief-attach-in 0.14s ease;
	}
	.brief-attach-pop::after {
		content: '';
		position: absolute;
		left: 20px;
		bottom: -5px;
		width: 10px;
		height: 10px;
		border-right: 1px solid var(--border, #e2e8f0);
		border-bottom: 1px solid var(--border, #e2e8f0);
		background: var(--card, #ffffff);
		transform: rotate(45deg);
	}
	.brief-attach-item {
		display: flex;
		align-items: center;
		gap: 10px;
		width: 100%;
		padding: 7px 8px;
		border: none;
		border-radius: 9px;
		background: transparent;
		color: var(--foreground, #0f172a);
		font-size: 13.5px;
		font-weight: 550;
		text-align: left;
		transition: background-color 0.12s ease;
	}
	.brief-attach-item:hover {
		background: var(--secondary, #f1f5f9);
	}
	.brief-attach-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 auto;
		width: 26px;
		height: 26px;
	}
	@keyframes brief-attach-in {
		from { opacity: 0; transform: translateY(4px) scale(0.98); }
		to { opacity: 1; transform: translateY(0) scale(1); }
	}
	@media (prefers-reduced-motion: reduce) {
		.brief-attach-pop { animation: none; }
	}
	.brief-composer-icon:hover {
		background: var(--secondary, #f1f5f9);
		color: var(--foreground, #0f172a);
	}
	.brief-composer-icon[aria-expanded='true'] {
		background: var(--secondary, #f1f5f9);
		color: var(--foreground, #0f172a);
	}
	.brief-composer-icon.brief-mic-active {
		background: #fee2e2;
		color: #dc2626;
	}
	.brief-composer-thinking {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 4px 10px;
		border-radius: 999px;
		background: #eff6ff;
		color: var(--primary, #2b67ec);
		font-size: 11.5px;
		font-weight: 650;
	}
	/* Tombol kirim: lingkaran biru Tauthub, seukuran kontrol lain. */
	.brief-composer-send {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 auto;
		width: 36px;
		height: 36px;
		padding: 0;
		border-radius: 999px;
		background: var(--primary, #2b67ec);
		color: #ffffff;
		border: none;
		box-shadow: 0 1px 2px rgba(43, 103, 236, 0.25);
		cursor: pointer;
		transition: opacity 0.12s ease, background-color 0.12s ease, transform 0.08s ease;
	}
	.brief-composer-send:hover:not(:disabled) {
		background: #2053c7;
	}
	.brief-composer-send:active:not(:disabled) {
		transform: scale(0.96);
	}
	.brief-composer-send.brief-composer-stop {
		background: #dc2626;
		box-shadow: 0 1px 2px rgba(220, 38, 38, 0.25);
	}
	.brief-composer-send:disabled {
		background: #e2e8f0;
		color: #94a3b8;
		box-shadow: none;
		cursor: not-allowed;
	}

	.brief-msg-action {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		margin-left: 4px;
		padding: 3px 7px;
		border-radius: 7px;
		font-size: 11px;
		color: var(--muted-foreground);
		opacity: 0;
		transition: opacity 0.12s ease, background-color 0.12s ease;
	}
	.brief-msg:hover .brief-msg-action,
	.brief-msg-action:focus-visible { opacity: 1; }
	.brief-msg-action:hover { background: var(--secondary); color: var(--foreground); }
	@media (hover: none) {
	.brief-msg-action { opacity: 0.75; }
	}

	/* Aksi usulan AI: tombol menempel di bawah balasan, dengan cuplikan caption
	   supaya user tahu apa yang akan ditambahkan sebelum menekan. */
	.brief-actions {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 6px;
		max-width: 92%;
		margin-top: 2px;
	}
	.brief-action-btn {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		padding: 7px 12px;
		border: 1px solid color-mix(in srgb, var(--primary) 35%, var(--border));
		border-radius: 8px;
		background: color-mix(in srgb, var(--primary) 7%, var(--card));
		font-size: 12.5px;
		font-weight: 600;
		color: var(--primary);
		cursor: pointer;
		transition: background-color 0.12s ease, border-color 0.12s ease;
	}
	.brief-action-btn:hover:not(:disabled) {
		background: color-mix(in srgb, var(--primary) 12%, var(--card));
		border-color: color-mix(in srgb, var(--primary) 55%, var(--border));
	}
	.brief-action-btn:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
	}
	.brief-action-btn.is-used {
		border-color: var(--border);
		background: var(--secondary);
		color: var(--muted-foreground);
		cursor: default;
	}
	.brief-action-preview {
		margin: 0;
		padding: 0 2px;
		font-size: 11.5px;
		line-height: 1.55;
		color: var(--muted-foreground);
		display: -webkit-box;
		line-clamp: 2;
		-webkit-line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}

	.brief-thinking {
		display: inline-flex;
		align-items: center;
		gap: 10px;
		padding: 10px 14px;
		border-radius: 16px 16px 16px 4px;
		border: 1px solid var(--border);
		background: var(--card);
		box-shadow: var(--shadow-sm);
		font-size: 14px;
		color: var(--muted-foreground);
	}
	.thinking-dots {
		display: inline-flex;
		align-items: center;
		gap: 4px;
	}
	.thinking-dots span {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--primary);
		animation: brief-bounce 1.2s infinite ease-in-out;
	}
	.thinking-dots span:nth-child(2) { animation-delay: 0.15s; }
	.thinking-dots span:nth-child(3) { animation-delay: 0.3s; }
	@keyframes brief-bounce {
		0%, 60%, 100% { transform: translateY(0); opacity: 0.45; }
		30% { transform: translateY(-4px); opacity: 1; }
	}
	/* Tombol "Obrolan Baru" di kanan area kerja: satu klik, tanpa popover. */
	.brief-picker {
		display: inline-flex;
		min-width: 0;
		max-width: 220px;
		align-items: center;
		gap: 6px;
		padding: 5px 11px;
		border: 1px solid var(--border);
		border-radius: 999px;
		background: var(--card);
		color: var(--foreground);
		font-size: 13px;
		font-weight: 600;
		transition: border-color 0.12s ease, color 0.12s ease, background-color 0.12s ease;
	}
	.brief-picker:hover:not(:disabled) { border-color: var(--primary); background: var(--accent); color: var(--accent-foreground); }
	.brief-picker:disabled { opacity: 0.5; cursor: not-allowed; }

	/* Jam kirim pesan (waktu lokal browser) di bawah gelembung. */
	.brief-time { padding: 0 4px; font-size: 11px; color: var(--muted-foreground); }

	/* ── Gerbang 2: pane Instan Brief ─────────────────────────────────── */
	.instan-page {
		/* Main content Instan Brief memakai lebar penuh area konten; dulu dibatasi
		   1180px sehingga menyisakan ruang kosong di layar lebar. */
		max-width: none;
		margin: 0 auto;
		padding: 32px 32px 56px;
	}
	.instan-head h2 {
		margin: 0;
		font-size: 27px;
		font-weight: 700;
		letter-spacing: -0.02em;
		line-height: 1.22;
		color: var(--foreground, #111827);
	}
	/* Dua kolom: langkah 1-3 di kiri (68%), pengelolaan data di kanan (32%). */
	.instan-grid {
		display: grid;
		grid-template-columns: minmax(0, 1.36fr) minmax(0, 0.64fr);
		gap: 28px;
		margin-top: 32px;
		align-items: start;
	}
	.instan-main,
	.instan-side {
		display: flex;
		flex-direction: column;
		gap: 24px;
		min-width: 0;
	}
	/* Kolom kanan mengikuti gulir: kartu penjelasan data tetap terlihat selama
	   form panjang di kiri digulir. Hanya di layar lebar, karena di bawah 1024px
	   grid jadi satu kolom dan sticky hanya akan menutupi konten. */
	@media (min-width: 1025px) {
		.instan-side {
			position: sticky;
			top: 20px;
			align-self: start;
		}
	}
	.instan-card {
		padding: 28px;
		border: 1px solid var(--border, #e2e8f0);
		border-radius: 12px;
		background: var(--card, #fff);
	}
	.instan-card h3 {
		margin: 0 0 12px;
		display: flex;
		align-items: center;
		gap: 9px;
		font-size: 17px;
		font-weight: 600;
		color: var(--foreground, #111827);
	}
	.instan-side-title {
		font-size: 15px !important;
		gap: 7px !important;
		margin-bottom: 18px !important;
	}
	.instan-side-title :global(svg) { color: var(--primary, #2b67ec); flex-shrink: 0; }
	.instan-step {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 22px;
		height: 22px;
		flex-shrink: 0;
		border-radius: 999px;
		background: color-mix(in srgb, var(--primary, #2b67ec) 10%, var(--card, #fff));
		color: var(--primary, #2b67ec);
		font-size: 12px;
		font-weight: 700;
	}
	.instan-lead {
		margin: 0 0 16px;
		font-size: 14px;
		line-height: 1.65;
		color: var(--muted-foreground);
	}

	/* Kotak ikon: satu ukuran, satu warna aksen, dipakai ulang di semua kartu. */
	.instan-ico {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		flex-shrink: 0;
		border-radius: 9px;
		background: color-mix(in srgb, var(--primary, #2b67ec) 8%, var(--card, #fff));
		color: var(--primary, #2b67ec);
	}
	.instan-ico-sm { width: 26px; height: 26px; border-radius: 8px; margin-top: 1px; }
	.instan-ico-muted { background: var(--secondary); color: var(--muted-foreground); }

	/* Kepala halaman: judul + satu kalimat penjelas. */
	.instan-head-desc {
		margin: 10px 0 0;
		max-width: 780px;
		font-size: 14.5px;
		line-height: 1.7;
		color: var(--muted-foreground);
	}

	/* Kepala kartu: judul seksi kiri, aksi kanan bila ada. */
	.instan-card-head {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 16px;
		flex-wrap: wrap;
	}
	.instan-card-head .instan-lead { margin-bottom: 0; }

	/* Pemilih dua jalur (segmented): pola yang sama dengan switch Masuk/Daftar
	   di halaman auth, memakai token warna halaman ini. Bungkusnya sticky supaya
	   tab tetap terjangkau saat form panjang digulir; latar selebar kartu
	   menutup isi yang lewat di belakangnya. */
	.instan-switch-sticky {
		position: sticky;
		top: 0;
		z-index: 6;
		margin: 20px -28px 0;
		padding: 12px 28px;
		background: var(--card, #fff);
		border-bottom: 1px solid var(--border, #e2e8f0);
	}
	.instan-switch {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 4px;
		margin: 0;
		padding: 5px;
		border: 1px solid var(--border, #e2e8f0);
		border-radius: 10px;
		background: color-mix(in srgb, var(--muted) 45%, var(--card, #fff));
	}
	.instan-switch button {
		height: 38px;
		border: 1px solid transparent;
		border-radius: 7px;
		background: transparent;
		color: var(--muted-foreground);
		font: inherit;
		font-size: 14px;
		font-weight: 500;
		cursor: pointer;
		transition: background-color 120ms ease, color 120ms ease;
	}
	.instan-switch button:hover { color: var(--foreground, #111827); }
	.instan-switch button[aria-selected='true'] {
		background: var(--card, #fff);
		border-color: var(--border, #e2e8f0);
		color: var(--foreground, #111827);
		font-weight: 600;
	}
	.instan-switch button:focus-visible {
		outline: 2px solid var(--primary, #2b67ec);
		outline-offset: 1px;
	}

	.instan-template {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 14px;
		margin-top: 24px;
		padding: 22px;
		border: 1px dashed var(--border, #e2e8f0);
		border-radius: 10px;
		background: color-mix(in srgb, var(--muted) 22%, var(--card, #fff));
	}
	.instan-template-lead {
		margin: 0;
		font-size: 14px;
		line-height: 1.65;
		color: var(--foreground, #111827);
	}
	.instan-template .instan-hint { margin: 0; }

	/* Satu ukuran tombol untuk seluruh halaman Instan Brief, mengikuti ukuran
	   tombol standar aplikasi (padding .5rem .875rem, teks 14px) dengan tinggi
	   minimum 40px agar target klik nyaman. */
	.instan-btn {
		min-height: 40px !important;
		padding: 0 16px !important;
		font-size: 14px !important;
	}

	.instan-form {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 24px 28px;
		margin-top: 28px;
	}
	.instan-form-actions {
		display: flex;
		justify-content: flex-end;
		gap: 10px;
		grid-column: 1 / -1;
		margin-top: 8px;
	}
	.instan-form > .instan-hint {
		grid-column: 1 / -1;
		margin-top: 0;
	}
	.instan-hint {
		margin: 16px 0 0;
		font-size: 13px;
		line-height: 1.65;
		color: var(--muted-foreground);
	}
	.instan-sep { margin: 0 8px; color: var(--muted-foreground); }

	.instan-drop {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 12px;
		padding: 32px 20px;
		border: 1.5px dashed color-mix(in srgb, var(--primary, #2b67ec) 35%, var(--border, #e2e8f0));
		border-radius: 10px;
		background: color-mix(in srgb, var(--primary, #2b67ec) 3%, var(--card, #fff));
		cursor: pointer;
		text-align: center;
		outline: none;
		transition: background-color 0.12s ease, border-color 0.12s ease;
	}
	.instan-drop-ico {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 34px;
		height: 34px;
		border-radius: 999px;
		background: color-mix(in srgb, var(--primary, #2b67ec) 9%, var(--card, #fff));
		color: var(--primary, #2b67ec);
	}
	.instan-drop p { margin: 0; font-size: 13.5px; font-weight: 600; color: var(--foreground, #111827); }
	.instan-drop-note { font-weight: 400 !important; font-size: 12px !important; color: var(--muted-foreground); }
	.instan-drop:hover,
	.instan-drop:focus-visible,
	.instan-drag {
		background: color-mix(in srgb, var(--primary, #2b67ec) 7%, var(--card, #fff));
		border-color: color-mix(in srgb, var(--primary, #2b67ec) 55%, var(--border, #e2e8f0));
	}

	.instan-uploads-title {
		margin: 24px 0 12px;
		font-size: 13px;
		font-weight: 600;
		color: var(--muted-foreground);
	}
	.instan-uploads { display: flex; flex-direction: column; gap: 8px; }
	.instan-empty {
		display: flex;
		align-items: flex-start;
		gap: 14px;
		padding: 18px;
		border: 1px dashed var(--border, #e2e8f0);
		border-radius: 10px;
	}
	.instan-empty-title { margin: 0; font-size: 13.5px; font-weight: 600; color: var(--foreground, #111827); }
	.instan-empty-note { margin: 2px 0 0; font-size: 12.5px; line-height: 1.55; color: var(--muted-foreground); }
	.instan-file {
		display: flex;
		align-items: center;
		gap: 10px;
		min-width: 0;
		padding: 10px 12px;
		border: 1px solid var(--border, #e2e8f0);
		border-radius: 8px;
		font-size: 12.5px;
		color: var(--foreground, #111827);
		text-decoration: none;
		background: var(--card, #fff);
	}
	.instan-file:hover { border-color: color-mix(in srgb, var(--primary) 35%, var(--border, #e2e8f0)); }
	.instan-file > :global(svg) { flex-shrink: 0; color: var(--muted-foreground); }
	.instan-file > span:first-of-type { flex: 1 1 auto; min-width: 0; }
	.instan-file-size { flex-shrink: 0; font-size: 11px; color: var(--muted-foreground); }
	.instan-badge {
		flex-shrink: 0;
		border-radius: 6px;
		padding: 1px 7px;
		font-size: 10px;
		font-weight: 700;
	}
	.instan-link {
		padding: 0;
		border: none;
		background: none;
		font: inherit;
		font-size: 12.5px;
		font-weight: 600;
		color: var(--primary, #2b67ec);
		cursor: pointer;
	}
	.instan-link:hover { text-decoration: underline; }

	.instan-cta { display: flex; flex-wrap: wrap; align-items: center; gap: 10px; margin-top: 18px; }

	/* Daftar kebijakan data: satu card, item berupa baris ikon + teks. */
	.instan-facts { display: flex; flex-direction: column; gap: 22px; }
	.instan-fact { display: flex; align-items: flex-start; gap: 14px; }
	.instan-fact-title { margin: 0; font-size: 13.5px; line-height: 1.4; font-weight: 600; color: var(--foreground, #111827); }
	.instan-fact-desc { margin: 6px 0 0; font-size: 13px; line-height: 1.65; color: var(--muted-foreground); }

	@media (max-width: 1024px) {
		.instan-grid { grid-template-columns: 1fr; }
	}
	@media (max-width: 720px) {
		.instan-page { padding: 20px 16px 40px; }
		.instan-head h2 { font-size: 21px; }
		.instan-form { grid-template-columns: 1fr; }
	}


	/* Kartu template di layar sapaan: klik = isi composer, ganti [KURUNG
	   SIKU] saja. Grid 3 kolom, 1 kolom di layar sempit. Jarak mengikuti tiga
	   tingkat: judul ke subjudul rapat, blok template terpisah jelas, catatan
	   menutup dengan jeda sedang. */
	.brief-tpl-label {
		margin: 34px auto 0;
		font-size: 11.5px;
		font-weight: 600;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		color: var(--muted-foreground);
		text-align: center;
	}
	.brief-templates {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 12px;
		margin: 14px auto 0;
		max-width: 700px;
	}
	.brief-tpl {
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding: 16px;
		text-align: left;
		border: 1px solid var(--border, #e2e8f0);
		border-radius: 12px;
		background: var(--card, #fff);
		cursor: pointer;
		transition: border-color 0.12s ease, background-color 0.12s ease;
	}
	.brief-tpl:hover {
		border-color: color-mix(in srgb, var(--primary) 45%, var(--border, #e2e8f0));
		background: color-mix(in srgb, var(--primary) 5%, var(--card, #fff));
	}
	.brief-tpl:disabled { opacity: 0.55; cursor: not-allowed; }
	.brief-tpl-title {
		font-size: 13.5px;
		font-weight: 700;
		line-height: 1.4;
		color: var(--foreground, #111827);
	}
	.brief-tpl-desc {
		font-size: 12.5px;
		line-height: 1.6;
		color: var(--muted-foreground, #667085);
	}
	.brief-tpl-note {
		margin: 18px auto 0;
		max-width: 520px;
		text-align: center;
		font-size: 12.5px;
		line-height: 1.6;
		color: var(--muted-foreground);
	}
	@media (max-width: 720px) {
		.brief-templates { grid-template-columns: 1fr; }
	}

	/* Bar tab utama disembunyikan selama tab Profil Brand menempel. Tingginya
	   dianimasikan lewat track grid (1fr ke 0fr) supaya tinggi otomatis, termasuk
	   saat tab membungkus di layar sempit, tetap terukur tanpa nilai tebakan. */
	.brief-tabs-collapse {
		display: grid;
		grid-template-rows: 1fr;
		overflow: hidden;
		opacity: 1;
		transition: grid-template-rows 220ms ease, opacity 180ms ease;
	}
	.brief-tabs-collapse.is-collapsed {
		grid-template-rows: 0fr;
		opacity: 0;
	}
	/* Popover Beta keluar dari kotak bar; jangan dipotong saat terbuka. */
	.brief-tabs-collapse.tabs-pop-open { overflow: visible; }
	.brief-head-block,
	.brief-tabs-row {
		min-height: 0;
	}
	@media (prefers-reduced-motion: reduce) {
		.brief-tabs-collapse { transition: none; }
	}

	/* Tab workspace (desain GEO): pil lembut berikon; tab aktif berlatar aksen. */
	.brief-tab {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 7px 14px;
		border: 1px solid transparent;
		border-radius: var(--radius-button);
		font-size: 12px;
		font-weight: 600;
		color: var(--muted-foreground);
		transition: background-color 0.12s ease, color 0.12s ease, border-color 0.12s ease;
	}
	.brief-tab:hover { background: var(--secondary); color: var(--foreground); }
	.brief-tab-active {
		background: var(--accent);
		border-color: color-mix(in srgb, var(--primary) 35%, transparent);
		color: var(--accent-foreground);
		font-weight: 700;
	}

	/* Popover lampiran: kartu terang, header + hitungan + daftar. */
	.brief-pop-backdrop { position: fixed; inset: 0; z-index: 40; cursor: default; }
	.brief-pop {
		position: absolute;
		top: calc(100% + 6px);
		left: 16px;
		z-index: 50;
		width: 320px;
		max-height: 360px;
		overflow-y: auto;
		padding: 14px;
		border: 1px solid var(--border);
		border-radius: 14px;
		background: var(--card);
		box-shadow: var(--shadow-md);
	}
	/* Tombol aksi header: terangkat halus saat hover, mengecil saat ditekan. */
	.brief-head-actions button {
		transition: transform 0.16s ease, box-shadow 0.16s ease, background-color 0.16s ease;
	}
	.brief-head-actions button:hover {
		transform: translateY(-1.5px);
		box-shadow: 0 4px 12px rgba(15, 23, 42, 0.12);
	}
	.brief-head-actions button:active {
		transform: translateY(0) scale(0.94);
		box-shadow: none;
	}
	@media (prefers-reduced-motion: reduce) {
		.brief-head-actions button:hover,
		.brief-head-actions button:active { transform: none; box-shadow: none; }
	}

	/* Tombol aksi header muncul dengan animasi halus saat header dibuka/diciutkan. */
	.brief-head-actions > * {
		animation: brief-head-pop 0.22s ease both;
	}
	.brief-head-actions > *:nth-child(2) { animation-delay: 0.04s; }
	.brief-head-actions > *:nth-child(3) { animation-delay: 0.08s; }
	.brief-head-actions > *:nth-child(4) { animation-delay: 0.12s; }
	@keyframes brief-head-pop {
		from { opacity: 0; transform: translateY(-4px) scale(0.96); }
		to { opacity: 1; transform: none; }
	}
	@media (prefers-reduced-motion: reduce) {
		.brief-head-actions > * { animation: none; }
	}

	/* Tombol buka/tutup daftar brief: selalu tampak di ujung kanan kartu. */
	.brief-card-toggle {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		width: 26px;
		height: 26px;
		margin-left: auto;
		border-radius: 7px;
		color: var(--muted-foreground);
		transition: background-color 0.15s ease, color 0.15s ease, transform 0.18s ease;
	}
	.brief-card-toggle:hover { background: var(--secondary); color: var(--foreground); }
	.brief-card-toggle:active { transform: scale(0.88); }
	/* Ikon toggle kini lucide komponen (ChevronDown), jadi perlu :global. */
	.brief-card-toggle :global(svg) { transition: transform 0.22s ease; display: block; }

	/* Kartu ciut: hanya ikon Tauthub bulat yang melayang; klik untuk membuka lagi. */
	.brief-card-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 44px;
		height: 44px;
		padding: 6px;
		border: 1px solid color-mix(in srgb, var(--primary) 18%, transparent);
		border-radius: 14px;
		background: var(--card);
		box-shadow: var(--shadow-md);
		animation: brief-card-in 0.2s ease;
		transition: transform 0.16s ease, box-shadow 0.16s ease, border-color 0.16s ease;
	}
	.brief-card-icon img { width: 100%; height: 100%; border-radius: 10px; }
	.brief-card-icon:hover { transform: translateY(-1px) scale(1.05); border-color: var(--primary); }
	.brief-card-icon:active { transform: scale(0.92); }
	@keyframes brief-card-in {
		from { opacity: 0; transform: scale(0.94) translateY(-3px); }
		to { opacity: 1; transform: none; }
	}
	.brief-pop-id, .brief-pop-head, .brief-pop-list { animation: brief-card-in 0.2s ease; }
	@media (prefers-reduced-motion: reduce) {
		.brief-card-icon, .brief-pop-id, .brief-pop-head, .brief-pop-list { animation: none; }
		.brief-card-icon:hover, .brief-card-icon:active { transform: none; }
	}

	/* Panel daftar brief: tampil tetap (bukan popover mengapung). */
	.brief-pop-static {
		position: static;
		top: auto;
		left: auto;
		width: 280px;
		/* Batasi tinggi supaya panel tidak keluar viewport; hanya daftar brief
		   yang menggulir, header tetap terlihat. */
		max-height: calc(100dvh - 200px);
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}
	.brief-pop-static > .brief-pop-id,
	.brief-pop-static > .brief-pop-head { flex: 0 0 auto; }
	/* Header panel: tombol "Obrolan Baru" kiri, nama brief mengikuti;
	   persen progres ada di baris daftar, bukan di kepala panel. */
	.brief-pop-head { display: flex; align-items: center; gap: 8px; }
	/* "Obrolan Baru": baris menu polos berikon — bukan pill berwarna. */
	.brief-newchat-sm {
		display: inline-flex;
		align-items: center;
		gap: 9px;
		padding: 8px 10px;
		border: none;
		border-radius: 8px;
		background: none;
		font-size: 13px;
		font-weight: 500;
		color: var(--muted-foreground);
		cursor: pointer;
		transition: background-color 0.12s ease, color 0.12s ease;
	}
	.brief-newchat-sm :global(svg) { color: var(--muted-foreground); }
	.brief-newchat-sm:hover:not(:disabled) { background: var(--secondary); color: var(--foreground); }
	.brief-newchat-sm:disabled { opacity: 0.5; cursor: not-allowed; }
	/* Header panel: judul produk ("Tubi AI") kiri, tombol kecilkan kanan. */
	.brief-pop-id {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin: -14px -14px 12px;
		padding: 12px 14px;
		border-bottom: 1px solid var(--border);
		border-radius: 14px 14px 0 0;
		background: color-mix(in srgb, var(--muted) 45%, white);
	}
	.brief-pop-brand {
		font-size: 15px;
		font-weight: 700;
		letter-spacing: -0.01em;
		color: var(--foreground);
	}
	/* Nama brief di baris daftar: terpotong dengan ellipsis, saat dihover
	   bergeser (marquee, dipasang action) supaya nama panjang terbaca utuh.
	   Tidak di-grow supaya chip persen menempel di ajir nama, bukan
	   terdorong ke tepi kanan baris. */
	.brief-marquee {
		display: block;
		min-width: 0;
		flex: 0 1 auto;
		overflow: hidden;
	}
	.brief-marquee > span {
		display: block;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	/* Kelas brief-marquee-run dipasang action saat hover (runtime), jadi
	   harus lewat :global agar svelte-check tidak menandainya unused. */
	.brief-marquee > :global(.brief-marquee-run) {
		max-width: none;
		overflow: visible;
		text-overflow: clip;
		will-change: transform;
		animation: brief-name-marquee var(--marquee-duration, 4s) ease-in-out infinite alternate;
	}
	@keyframes brief-name-marquee {
		from { transform: translateX(0); }
		to { transform: translateX(var(--marquee-shift, 0px)); }
	}
	@media (prefers-reduced-motion: reduce) {
		.brief-marquee > :global(.brief-marquee-run) { animation: none; }
	}
	/* Persen progres field wajib di tiap baris brief (panel melayang + rail kiri). */
	.brief-pct {
		flex-shrink: 0;
		font-size: 11.5px;
		font-weight: 600;
		color: var(--muted-foreground);
	}
	.brief-pop-icon {
		display: inline-flex; align-items: center; justify-content: center;
		width: 28px; height: 28px; border-radius: 7px; color: var(--muted-foreground);
		transition: background-color 0.12s ease, color 0.12s ease;
	}
	.brief-pop-icon:hover:not(:disabled) { background: var(--secondary); color: var(--foreground); }
	.brief-pop-list {
		/* Baris "Obrolan Baru" di head butuh napas sebelum daftar dimulai. */
		margin-top: 12px;
		display: flex;
		flex-direction: column;
		gap: 4px;
		min-height: 0;
		overflow-y: auto;
		/* Tampilkan maksimal 10 baris; sisanya lewat scroll internal. */
		max-height: calc(38px * 10 + 4px * 9);
		padding-right: 2px;
		scrollbar-width: thin;
		scrollbar-color: color-mix(in srgb, var(--foreground) 22%, transparent) transparent;
	}
	.brief-pop-list::-webkit-scrollbar { width: 6px; }
	.brief-pop-list::-webkit-scrollbar-track { background: transparent; }
	.brief-pop-list::-webkit-scrollbar-thumb {
		border-radius: 999px;
		background: color-mix(in srgb, var(--foreground) 22%, transparent);
	}
	.brief-pop-list::-webkit-scrollbar-thumb:hover {
		background: color-mix(in srgb, var(--foreground) 35%, transparent);
	}
	/* Baris brief: aksi menumpuk DI DALAM baris (absolute), jadi lebar baris
	   tidak bertambah dan nama tetap terpotong rapi di dalam panel. */
	.brief-pop-item {
		position: relative;
		border-radius: 7px;
	}
	.brief-pop-item .brief-pop-row { width: 100%; padding-right: 64px; }
	.brief-pop-item > .app-input { width: 100%; padding-right: 34px; }
	.brief-pop-pin, .brief-pop-kebab, .brief-pop-save {
		position: absolute;
		top: 50%;
		transform: translateY(-50%);
		z-index: 2;
	}
	.brief-pop-pin { right: 34px; }
	.brief-pop-kebab, .brief-pop-save { right: 5px; }
	.brief-pop-pin, .brief-pop-kebab { opacity: 0; }
	.brief-pop-item:hover .brief-pop-pin,
	.brief-pop-item:hover .brief-pop-kebab,
	.brief-pop-item:focus-within .brief-pop-pin,
	.brief-pop-item:focus-within .brief-pop-kebab,
	.brief-pop-item:has(.brief-menu) .brief-pop-pin,
	.brief-pop-item:has(.brief-menu) .brief-pop-kebab { opacity: 1; }
	.brief-pop-pin.pinned { opacity: 1; color: var(--primary); }
	.brief-pop-menu { right: 4px; }
	/* Baris paling bawah: buka menu ke atas supaya tidak terpotong panel. */
	.brief-pop-item:nth-last-child(-n + 3) .brief-pop-menu { top: auto; bottom: calc(100% - 2px); }
	.brief-pop-empty { padding: 6px 4px; font-size: 12px; color: var(--muted-foreground); }
	/* Baris brief: yang tidak aktif abu-abu, yang aktif hitam dengan latar abu tipis. */
	.brief-pop-row {
		display: flex; align-items: center; gap: 10px;
		width: 100%;
		padding: 10px 10px; border-radius: 8px;
		font-size: 13px; color: var(--muted-foreground); text-decoration: none;
		background: none; border: none; text-align: left; cursor: pointer;
		transition: background-color 0.12s ease, color 0.12s ease;
	}
	.brief-pop-row:hover { background: var(--secondary); color: var(--foreground); }
	.brief-pop-row.active { background: var(--secondary); color: var(--foreground); font-weight: 600; }

	/* ── Timeline Brand: feed gaya Threads — kartu putih, pemisah garis ── */
	.timeline-scroll { background: var(--muted); }
	.timeline-card {
		background: var(--card);
		border: 1px solid var(--border);
		border-radius: 16px;
		box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
	}
	.timeline-avatar {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 auto;
		width: 40px;
		height: 40px;
		border-radius: 9999px;
		color: var(--primary-foreground);
		background: var(--primary);
		font-size: 13px;
		font-weight: 700;
		letter-spacing: 0.02em;
	}
	/* Composer ringkas: avatar + teks draf + Kirim. Klik di mana pun membuka modal. */
	.timeline-compose-row {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 14px 16px;
		cursor: text;
	}
	.timeline-trigger-text {
		flex: 1 1 auto;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 15px;
		line-height: 1.5;
		color: var(--foreground);
	}
	.timeline-trigger-empty { color: var(--muted-foreground); }
	.timeline-kirim {
		flex: 0 0 auto;
		display: inline-flex;
		align-items: center;
		gap: 6px;
		margin-top: 2px;
		padding: 9px 18px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--card);
		font-size: 13.5px;
		font-weight: 700;
		color: var(--foreground);
		transition: background-color 0.12s ease, border-color 0.12s ease;
	}
	.timeline-kirim:hover:not(:disabled) {
		background: var(--secondary);
		border-color: color-mix(in srgb, var(--border) 55%, var(--foreground));
	}
	/* Terisi → tombol jadi utama: biru padat, teks putih. */
	.timeline-kirim-ready,
	.timeline-kirim-ready:hover:not(:disabled) {
		background: var(--primary);
		border-color: var(--primary);
		color: var(--primary-foreground);
	}
	.timeline-kirim-ready:hover:not(:disabled) {
		background: color-mix(in srgb, var(--primary) 88%, black);
	}
	.timeline-kirim:disabled { opacity: 0.45; cursor: not-allowed; }
	.timeline-error {
		display: flex;
		align-items: center;
		gap: 10px;
		margin: 0 16px 12px;
		border-radius: 10px;
		padding: 8px 12px;
		font-size: 12px;
		background: color-mix(in srgb, var(--destructive) 8%, white);
		color: var(--destructive);
	}
	.timeline-error-retry {
		border: none;
		background: transparent;
		font-size: 12px;
		font-weight: 700;
		text-decoration: underline;
		text-underline-offset: 3px;
		color: var(--destructive);
	}
	/* Feed: satu kartu, tiap post dipisah garis tipis (bukan kartu terpisah). */
	.timeline-post { display: flex; gap: 12px; padding: 16px; }
	.timeline-post + .timeline-post { border-top: 1px solid var(--border); }
	.timeline-post-meta { display: flex; align-items: baseline; gap: 6px; min-width: 0; }
	.timeline-post-name {
		font-size: 14.5px;
		font-weight: 700;
		color: var(--foreground);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.timeline-post-time { font-size: 13px; color: var(--muted-foreground); white-space: nowrap; }
	.timeline-post-body {
		margin-top: 2px;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
		font-size: 15px;
		line-height: 1.55;
		color: var(--foreground);
	}
	.timeline-post-image {
		display: block;
		width: 100%;
		margin-top: 10px;
		border-radius: 12px;
		border: 1px solid var(--border);
		object-fit: contain;
		max-height: 520px;
		background: var(--muted);
	}
	/* Titik tiga di kanan atas post → popover aksi. */
	.timeline-post-main { position: relative; }
	.timeline-more {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 auto;
		align-self: center;
		width: 28px;
		height: 28px;
		margin: -4px -6px 0 auto;
		border-radius: 9999px;
		color: var(--muted-foreground);
		transition: background-color 0.12s ease, color 0.12s ease;
	}
	.timeline-more:hover { background: var(--secondary); color: var(--foreground); }
	.timeline-menu-backdrop {
		position: fixed;
		inset: 0;
		z-index: 30;
		background: transparent;
		border: none;
		cursor: default;
	}
	.timeline-menu {
		position: absolute;
		right: 0;
		top: 28px;
		z-index: 40;
		min-width: 178px;
		padding: 5px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--card);
		box-shadow: 0 12px 28px rgba(15, 23, 42, 0.14);
	}
	.timeline-menu button {
		display: flex;
		align-items: center;
		gap: 9px;
		width: 100%;
		padding: 8px 10px;
		border-radius: 8px;
		font-size: 13px;
		font-weight: 600;
		color: var(--foreground);
		text-align: left;
	}
	.timeline-menu button:hover { background: var(--secondary); }
	.timeline-menu .timeline-menu-danger { color: var(--destructive); }

	/* Tagar di dalam caption: berwarna, sengaja bukan tautan. */
	.timeline-body-tag { color: var(--primary); font-weight: 600; }

	/* Kartu pratinjau tautan (Open Graph). */
	.timeline-link-card {
		display: flex;
		align-items: stretch;
		gap: 12px;
		margin-top: 10px;
		padding: 12px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: color-mix(in srgb, var(--muted) 45%, white);
		text-decoration: none;
		color: inherit;
	}
	a.timeline-link-card:hover { border-color: color-mix(in srgb, var(--border) 45%, var(--foreground)); }
	.timeline-link-body { display: flex; flex-direction: column; gap: 3px; min-width: 0; flex: 1 1 auto; }
	.timeline-link-site { font-size: 11.5px; color: var(--muted-foreground); }
	.timeline-link-title { font-size: 13.5px; font-weight: 700; line-height: 1.4; overflow-wrap: anywhere; }
	.timeline-link-desc {
		display: -webkit-box;
		line-clamp: 2;
		-webkit-line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		font-size: 12.5px;
		line-height: 1.45;
		color: var(--muted-foreground);
	}
	.timeline-link-thumb {
		flex: 0 0 auto;
		width: 96px;
		height: 96px;
		border-radius: 8px;
		object-fit: cover;
		background: var(--muted);
	}
	.timeline-link-card-edit { position: relative; margin-top: 0; }
	.timeline-link-remove {
		position: absolute;
		right: 6px;
		top: 6px;
		display: inline-flex;
		padding: 5px;
		border-radius: 9999px;
		background: rgba(15, 23, 42, 0.65);
		color: #fff;
	}

	/* ── Modal composer ("Utas baru") ───────────────────────────────────── */
	.timeline-modal-backdrop {
		position: fixed;
		inset: 0;
		z-index: 60;
		background: rgba(15, 23, 42, 0.45);
	}
	/* Modal dipusatkan ke area konten (main), bukan viewport: nav kiri 264px
	   membuat pusat viewport meleset ke kiri dari kolom timeline. */
	.timeline-modal {
		position: absolute;
		z-index: 61;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		display: flex;
		flex-direction: column;
		width: min(620px, calc(100vw - 32px));
		max-height: min(82vh, 720px);
		border: 1px solid var(--border);
		border-radius: 18px;
		background: var(--card);
		box-shadow: 0 24px 64px rgba(15, 23, 42, 0.24);
	}
	.timeline-modal-head {
		display: grid;
		grid-template-columns: 1fr auto 1fr;
		align-items: center;
		gap: 8px;
		padding: 10px 16px;
		border-bottom: 1px solid var(--border);
	}
	.timeline-modal-cancel { justify-self: start; font-size: 13.5px; font-weight: 600; color: var(--foreground); }
	.timeline-modal-icon-btn {
		justify-self: end;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border-radius: 9999px;
		color: var(--foreground);
		transition: background-color 0.12s ease;
	}
	.timeline-modal-icon-btn:hover { background: var(--secondary); }
	/* Tombol kembali di tampilan Draf menempel di ujung kiri. */
	.timeline-modal-back { justify-self: start; margin-left: -6px; }
	.timeline-modal-title { font-size: 14.5px; font-weight: 700; }
	.timeline-modal-spacer { justify-self: end; width: 44px; }
	.timeline-modal-body {
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 10px 16px 12px;
		overflow-y: auto;
		transition: background-color 0.12s ease;
	}
	.timeline-modal-dragging { background: color-mix(in srgb, var(--primary) 6%, transparent); }
	/* Rel kiri: avatar + garis penghubung turun ke baris tool. */
	.timeline-compose-top { display: flex; gap: 10px; }
	.timeline-compose-rail { display: flex; flex-direction: column; align-items: center; flex: 0 0 auto; width: 34px; }
	.timeline-compose-line { flex: 1 1 auto; width: 1.5px; min-height: 10px; margin-top: 6px; border-radius: 2px; background: var(--border); }
	.timeline-compose-content { flex: 1 1 auto; min-width: 0; }
	.timeline-avatar-sm { width: 34px; height: 34px; font-size: 11.5px; }
	.timeline-modal-name {
		display: flex;
		align-items: center;
		gap: 4px;
		min-width: 0;
		font-size: 13.5px;
		font-weight: 700;
		line-height: 1.3;
	}
	.timeline-modal-name > span {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.timeline-verified { flex: 0 0 auto; }
	.timeline-modal-input {
		width: 100%;
		min-height: 28px;
		max-height: 40vh;
		padding: 2px 0;
		overflow-y: auto;
		resize: none;
		border: none;
		outline: none;
		background: transparent;
		font-size: 15px;
		line-height: 1.5;
		color: var(--foreground);
	}
	.timeline-modal-input::placeholder { color: var(--muted-foreground); font-size: 13.5px; }
	/* Baris tool di luar kolom konten; padding tombol (9px) menaruh ikon tepat
	   di sumbu avatar/garis di atasnya. */
	.timeline-modal-tools { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; margin-left: 0; }
	.timeline-modal-hint { font-size: 11.5px; color: var(--muted-foreground); }
	.timeline-modal-foot {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding: 10px 16px;
		border-top: 1px solid var(--border);
	}
	.timeline-error-modal { margin: 0; }

	/* Daftar draf di dalam modal. */
	.timeline-drafts-body { flex: 1 1 auto; min-height: 320px; padding: 12px 16px 16px; overflow-y: auto; }
	.timeline-drafts-empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 8px;
		height: 100%;
		min-height: 280px;
		text-align: center;
		color: var(--muted-foreground);
	}
	.timeline-drafts-empty strong { font-size: 14px; color: var(--foreground); }
	.timeline-drafts-empty p { font-size: 13px; }
	.timeline-draft-row {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 10px;
		border-radius: 12px;
		transition: background-color 0.12s ease;
	}
	.timeline-draft-row:hover { background: var(--secondary); }
	.timeline-draft-open {
		display: flex;
		flex-direction: column;
		gap: 3px;
		flex: 1 1 auto;
		min-width: 0;
		text-align: left;
	}
	.timeline-draft-text {
		display: -webkit-box;
		line-clamp: 2;
		-webkit-line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		font-size: 13.5px;
		line-height: 1.45;
		color: var(--foreground);
	}
	.timeline-draft-meta { font-size: 11.5px; color: var(--muted-foreground); }
	.timeline-draft-thumb { flex: 0 0 auto; width: 44px; height: 44px; border-radius: 8px; object-fit: cover; background: var(--muted); }

	/* Konfirmasi "Simpan ke draf?" — lembar kecil di atas modal. */
	.timeline-confirm-backdrop {
		position: fixed;
		inset: 0;
		z-index: 70;
		background: rgba(15, 23, 42, 0.35);
	}
	.timeline-confirm {
		position: absolute;
		z-index: 71;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		display: flex;
		flex-direction: column;
		width: min(320px, calc(100vw - 40px));
		padding: 18px 0 0;
		overflow: hidden;
		border: 1px solid var(--border);
		border-radius: 16px;
		background: var(--card);
		box-shadow: 0 20px 48px rgba(15, 23, 42, 0.22);
		text-align: center;
	}
	.timeline-confirm-title { padding: 0 18px; font-size: 14.5px; font-weight: 700; }
	.timeline-confirm-text { margin: 6px 0 14px; padding: 0 18px; font-size: 12.5px; line-height: 1.5; color: var(--muted-foreground); }
	.timeline-confirm-btn {
		width: 100%;
		padding: 11px 0;
		border-top: 1px solid var(--border);
		font-size: 13.5px;
		font-weight: 600;
		color: var(--foreground);
	}
	.timeline-confirm-btn:hover { background: var(--secondary); }
	.timeline-confirm-danger { color: var(--destructive); }
	.timeline-preview-wrap { position: relative; max-width: 420px; }
	.timeline-preview-image {
		display: block;
		width: 100%;
		max-height: 260px;
		object-fit: contain;
		border-radius: 12px;
		border: 1px solid var(--border);
	}
	.timeline-remove-image {
		position: absolute;
		right: 8px;
		top: 8px;
		display: inline-flex;
		padding: 6px;
		border-radius: 9999px;
		background: rgba(15, 23, 42, 0.65);
		color: white;
	}
	/* Status unggahan latar belakang di bawah pratinjau. */
	.timeline-upload-badge {
		position: absolute;
		left: 8px;
		bottom: 8px;
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 4px 9px;
		border-radius: 9999px;
		background: rgba(15, 23, 42, 0.72);
		color: #fff;
		font-size: 11px;
		font-weight: 600;
	}
	.timeline-upload-badge-error { background: color-mix(in srgb, var(--destructive) 88%, black); }
	.timeline-tool {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 7px 9px;
		border: none;
		border-radius: 8px;
		background: transparent;
		color: var(--muted-foreground);
		font-size: 12.5px;
		font-weight: 600;
	}
	.timeline-tool:hover { background: var(--secondary); color: var(--foreground); }
	.timeline-file-tool { cursor: pointer; }
	.timeline-char-count { padding-right: 6px; font-size: 11.5px; color: var(--muted-foreground); font-variant-numeric: tabular-nums; }
	.timeline-char-limit { color: var(--destructive); font-weight: 700; }
	.timeline-empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 8px;
		padding: 40px 20px;
		text-align: center;
		border-style: dashed;
	}
	.timeline-empty strong { font-size: 14.5px; color: var(--foreground); }
	.timeline-empty p { max-width: 380px; font-size: 13px; color: var(--muted-foreground); }
	.timeline-empty-icon {
		display: inline-flex;
		padding: 12px;
		border-radius: 9999px;
		background: color-mix(in srgb, var(--primary) 10%, white);
		color: var(--primary);
	}
	.timeline-empty-error { border-style: solid; gap: 6px; }
	@media (max-width: 640px) {
		.timeline-compose-row { padding: 12px; gap: 10px; }
		.timeline-kirim { padding: 8px 14px; }
		.timeline-post { padding: 14px 12px; }
		.timeline-error { margin-left: 12px; margin-right: 12px; }
		.timeline-modal { max-height: 88vh; }
		.timeline-link-thumb { width: 72px; height: 72px; }
	}
	@media (prefers-reduced-motion: reduce) {
		.thinking-dots span { animation: none; opacity: 0.6; }
	}

</style>
