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
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import {
		ArrowRight, ArrowUp, Bot, ChartLine, Check, ChevronDown, ClipboardList, Code2, Copy, FileText, Link2, LoaderCircle, MessageSquare,
		MoreHorizontal, PanelRight, Pencil, Pin, Plus, RotateCcw, Save, Search, ShieldAlert, Sparkle, Square, SquarePen, Trash2, Upload, UserCircle, X
	} from '@lucide/svelte';
	import PageTitle from '$lib/components/page-title.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, getAccessToken, showToast, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { renderMarkdown } from '$lib/markdown-lite';
	import { sanitizeHtml } from '$lib/sanitize-html';

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/profiler`);

	type BriefListItem = { id: string; name: string; description: string | null; short_code: string | null; pinned: boolean; updated_at: string | null };
	type FieldMeta = {
		key: string; layer: number; section: string; label: string; widget: string;
		required: boolean; sort_order: number; example_question: string; enabled: boolean;
	};
	type FieldStateView = { status: string; source: string; note: string };
	type SourceView = { kind: string; label: string };
	type ChatRow = { id?: string; role: 'user' | 'assistant'; content: string; created_at?: string };

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
		} catch { /* penyimpanan penuh/privat — pakai bawaan */ }
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

	let loading = $state(true);
	let error = $state('');

	// Tinggi kerja halaman: diukur dari posisi kontainer supaya halaman tidak
	// pernah melebihi pandangan (compact, tanpa scroll badan) — apapun tinggi
	// bar utilitas di atasnya.
	let shellEl = $state<HTMLElement | null>(null);
	let shellTop = $state(64);

	// Chat
	let profileName = $state('');
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
	let urlOpen = $state(false);
	let urlValue = $state('');
	let pasteOpen = $state(false);

	// Lencana "Beta" di pojok kanan atas judul percakapan — keterangannya
	// dibuka saat diklik, ditutup lewat backdrop atau Escape.
	let betaOpen = $state(false);

	// Tab workspace: percakapan Copilot vs ringkasan entitas (satu halaman).
	let activeTab = $state<'copilot' | 'ringkasan'>('copilot');
	let showLayer2 = $state(false);

	// Header band bisa diciutkan jadi strip judul tipis; default termuat jejak.
	let headerCollapsed = $state(true);

	// Kartu brief bisa diciutkan jadi ikon Tauthub; klik ikon membuka lagi.
	let cardOpen = $state(true);

	// Popover tindakan Copilot (pill kanan).
	let actionsOpen = $state(false);

	// Lampiran dokumen: dipasang dulu sebagai chip, baru diunggah saat Kirim.
	type PendingDoc = { file: File };
	let pendingDocs = $state<PendingDoc[]>([]);
	let uploadingNow = $state(false);
	let draggingFiles = $state(false);
	let dragDepth = 0;

	// Panel Ringkasan
	let ringkasan = $state<Record<string, string>>({});
	let saving = $state(false);

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

		let jsonPreviewOpen = $state(false);
		const previewJson = $derived(JSON.stringify(values, null, 2));

		function copyJson() {
			try {
				void navigator.clipboard.writeText(previewJson);
				showToast(tr('JSON disalin ke papan klip.'));
			} catch {
				showToast(tr('Gagal menyalin JSON.'), 'error');
			}
		}

		// Bersihkan percakapan di layar; data profil yang sudah tersimpan di
		// Ringkasan tidak ikut terhapus (server tidak disentuh).
		function restartChat() {
			openConfirm({
				title: tr('Mulai ulang obrolan?'),
				body: tr('Percakapan di layar ini dikembalikan ke awal. Data profil entitas yang sudah tersimpan di Ringkasan tetap aman.'),
				confirmLabel: tr('Mulai Ulang'),
				danger: false,
				action: () => {
					messages = [];
					streamError = '';
					streamingText = '';
					showToast(tr('Obrolan dimulai ulang.'));
				}
			});
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
			updated_at: p.updated_at ?? null
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

	// Sematkan/lepas: ditulis di server, jadi seluruh anggota brand melihat pin
	// yang sama dan urutannya otomatis menaruh yang disematkan di atas.
	async function togglePin(id: string, pinned: boolean) {
		menuId = null;
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
			try {
				for (const d of pendingDocs) names.push(await uploadBriefFile(d.file));
			} catch (e) {
				uploadingNow = false;
				// Chip dipertahankan supaya bisa mencoba lagi tanpa memilih ulang.
				streamError = e instanceof Error ? e.message : tr('Gagal mengunggah dokumen.');
				return;
			}
			uploadingNow = false;
			pendingDocs = [];
			if (!message) {
				const list = names.map((n) => `\u{22}${n}\u{22}`).join(', ');
				message =
					names.length === 1
						? tr(`Aku baru unggah dokumen ${list}. Baca dulu ya, lalu tunjukkan apa yang kamu tangkap.`)
						: tr(`Aku baru mengunggah ${names.length} dokumen: ${list}. Baca dulu ya, lalu tunjukkan apa yang kamu tangkap.`);
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
						messages = [...messages, { role: 'assistant', content: String(data.reply ?? partial), created_at: new Date().toISOString() }];
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

	async function addWebsiteUrl() {
		const url = urlValue.trim();
		if (!url) return;
		urlOpen = false;
		urlValue = '';
		working(true);
		try {
			const res = await apiRequest(`${apiBase}/profiles/${profileId}/urls`, {
				method: 'POST',
				body: JSON.stringify({ url })
			}, 60_000);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menambahkan URL.')));
			await send(`Ini website resmi brand-ku: ${url}. Baca dulu ya, lalu tunjukkan apa yang kamu tangkap.`);
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal menambahkan URL.');
			working(false);
		}
	}

	const MAX_UPLOAD_BYTES = 10 * 1024 * 1024;

	// Unggah satu dokumen; nama file dikembalikan untuk disebut di pesan.
	async function uploadBriefFile(file: File): Promise<string> {
		const form = new FormData();
		form.append('file', file);
		const res = await apiRequest(`${apiBase}/profiles/${profileId}/documents`, {
			method: 'POST',
			body: form
		}, 120_000);
		if (!res.ok) throw new Error(await readApiError(res, tr('Gagal mengunggah dokumen.')));
		return file.name;
	}

	function autoGrow(el: HTMLTextAreaElement) {
		el.style.height = 'auto';
		el.style.height = `${Math.min(el.scrollHeight, 160)}px`;
	}

	function fmtSize(bytes: number): string {
		if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
		if (bytes >= 1024) return `${Math.round(bytes / 1024)} KB`;
		return `${bytes} B`;
	}

	// Pasang file sebagai lampiran (chip). Yang dobel nama+ukuran diabaikan,
	// yang melebihi 10 MB ditolak dengan menyebut namanya.
	function stageFiles(list: FileList | File[] | null | undefined) {
		if (!list) return;
		const rejected: string[] = [];
		for (const file of Array.from(list)) {
			if (file.size > MAX_UPLOAD_BYTES) {
				rejected.push(`${file.name} (${fmtSize(file.size)})`);
				continue;
			}
			const dup = pendingDocs.some((d) => d.file.name === file.name && d.file.size === file.size);
			if (!dup) pendingDocs = [...pendingDocs, { file }];
		}
		if (rejected.length > 0) {
			streamError = `${tr('Melebihi 10 MB, tidak dilampirkan')}: ${rejected.join(', ')}`;
		}
	}

	function hasFileDrag(e: DragEvent): boolean {
		return Array.from(e.dataTransfer?.types ?? []).includes('Files');
	}
	function onChatDragEnter(e: DragEvent) {
		if (!hasFileDrag(e)) return;
		dragDepth += 1;
		draggingFiles = true;
	}
	function onChatDragLeave() {
		dragDepth = Math.max(0, dragDepth - 1);
		if (dragDepth === 0) draggingFiles = false;
	}
	function onChatDrop(e: DragEvent) {
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
				showToast(tr('Seksi ini belum punya kolom penyimpanan — periksa skema field di admin.'), 'error');
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

<PageTitle title={`Taut Brief — ${profileName || tr('Profiler')}`} />

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
						onclick={() => { sidebarOpen = false; activeTab = 'ringkasan'; }}
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
		<section class="flex-shrink-0 border-b bg-card" style="border-color: var(--border);">
			<div class="flex flex-col gap-3 px-4 pt-4 pb-3 md:px-6 xl:flex-row xl:items-start xl:justify-between">
				<div class="min-w-0">
					<div class="flex min-w-0 items-center gap-2.5">
						<h1 class="truncate text-2xl font-semibold tracking-tight">Brand Brief &amp; Entity Builder</h1>
						<span class="flex-shrink-0 rounded border px-2 py-0.5 text-[10px] font-bold" style="background: var(--accent); border-color: color-mix(in srgb, var(--primary) 25%, transparent); color: var(--accent-foreground);">GEO v2.4</span>
					</div>
					{#if !headerCollapsed}
						<p class="mt-1 max-w-3xl text-sm text-muted-foreground">
							{tr('Bangun Entity Source of Truth brand Anda melalui obrolan santai atau tempel URL. AI mengekstrak fakta kanonik secara otomatis agar siap digunakan di ratusan rilis media.')}
						</p>
					{/if}
				</div>
				<div class="brief-head-actions flex flex-shrink-0 flex-wrap items-center gap-2">
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
							class="app-btn app-btn-soft !min-h-8 !min-w-8 !p-1"
							type="button"
							aria-label={tr('Pratinjau JSON')}
							title={tr('Pratinjau JSON')}
							onclick={() => (jsonPreviewOpen = true)}
						>
							<Code2 size={14} />
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
						<button class="app-btn app-btn-soft !min-h-8 !text-xs" type="button" onclick={() => (jsonPreviewOpen = true)}>
							<Code2 size={14} />{tr('Pratinjau JSON')}
						</button>
						<button class="app-btn app-btn-primary !min-h-8 !text-xs" type="button" onclick={() => void goto(`/klien/${brandUrl}/campaigns`)}>
							{tr('Gunakan untuk Kampanye')}<ArrowRight size={14} />
						</button>
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

			<div class="flex flex-wrap items-center justify-between gap-2 border-t px-4 py-2 md:px-6" style="border-color: var(--border);">
				<div class="flex flex-wrap items-center gap-1.5" role="tablist">
					<button
						class="brief-tab"
						class:brief-tab-active={activeTab === 'copilot'}
						type="button"
						role="tab"
						aria-selected={activeTab === 'copilot'}
						onclick={() => (activeTab = 'copilot')}
					>
						<Bot size={14} />{tr('Copilot (Obrolan & Ekstraksi AI)')}
						{#if activeTab === 'copilot'}<span class="h-2 w-2 animate-pulse rounded-full" style="background: var(--primary);"></span>{/if}
					</button>
					<button
						class="brief-tab"
						class:brief-tab-active={activeTab === 'ringkasan'}
						type="button"
						role="tab"
						aria-selected={activeTab === 'ringkasan'}
						onclick={() => (activeTab = 'ringkasan')}
					>
						<FileText size={14} />{tr('Ringkasan Entitas')}
						<span
							class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-bold"
							style="background: {progressPct >= 100 ? 'var(--success-soft)' : 'var(--warning-soft)'}; color: {progressPct >= 100 ? 'var(--success)' : 'var(--warning)'};"
						>
							<span class="h-1.5 w-1.5 rounded-full" style="background: {progressPct >= 100 ? 'var(--success)' : 'var(--warning)'};"></span>{progressPct}% {tr('Siap Rilis')}
						</span>
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
					<span class="inline-flex items-center gap-1.5">
						<span class="h-2 w-2 rounded-full" style="background: {configured ? 'var(--success)' : 'var(--warning)'};"></span>{tr('Mode Interaktif GEO Aktif')}
					</span>
					<span style="color: var(--border);">|</span>
					<span>{tr('Sinkronisasi Terakhir')}: {syncText()}</span>
				</div>
			</div>
		</section>

	{#if activeTab === 'copilot'}
			<!-- Pane Copilot: lapisan melayang di atas, obrolan mengisi sisanya. -->
			<div class="relative flex min-h-0 flex-1 flex-col">
				<!-- Lapisan melayang: kartu brief kiri, pill Copilot kanan, di atas obrolan. -->
				<div class="pointer-events-none absolute inset-x-0 top-2 z-20 flex items-start justify-between gap-3 px-4 md:px-6">
					<div class="pointer-events-auto">
						{#if cardOpen}
					<div class="brief-pop brief-pop-static">
							<div class="brief-pop-id">
					<div class="flex items-start gap-3.5">
						<div class="relative">
							<span class="brief-avatar brief-avatar-ai" style="width: 36px; height: 36px;" aria-hidden="true">
								<img src="/tauthub-icon.png" alt="" />
							</span>
							<span class="absolute -right-0.5 -bottom-0.5 h-2.5 w-2.5 rounded-full border-2 border-white" style="background: {streaming ? 'var(--warning)' : 'var(--success)'};"></span>
						</div>
						<div>
							<div class="flex flex-wrap items-center gap-2">
								<h2 class="text-sm font-bold">Tauthub AI Copilot</h2>
							</div>
							<p class="mt-0.5 text-[10px] leading-snug text-muted-foreground">{tr('Ekstraktor Pengetahuan Entitas Otomatis & Sinkronisasi GEO Source of Truth')}</p>
						</div>
						<button
							class="brief-card-toggle"
							type="button"
							aria-expanded={cardOpen}
							aria-label={tr('Kecilkan')}
							title={tr('Kecilkan')}
							onclick={() => (cardOpen = false)}
						>
												<svg xmlns="http://www.w3.org/2000/svg" width="1em" height="1em" viewBox="0 0 24 24" aria-hidden="true">
													<path d="M0 0h24v24H0z" fill="none" />
													<path fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M15.55 2.73h1.8c1.9 0 2.84 0 3.42.58c.59.59.59 1.53.59 3.42v1.8M8.97 21.28H6.73c-1.9 0-2.84 0-3.42-.58c-.59-.59-.59-1.53-.59-3.42v-2.25m11.7-5.36l5.94-5.94M9.7 14.3l-5.98 5.97" />
												</svg>
						</button>
					</div>
						</div>
						<div class="brief-pop-head">
									<span class="brief-pop-title">{tr('Semua Brief')} ({briefs.length})</span>
									<span class="brief-pop-actions">
										<span class="brief-pop-count">{progressPct}%</span>
										<button class="brief-pop-icon" type="button" aria-label={tr('Brief baru')} disabled={workingState} onclick={() => void createBrief()}>
												<Plus size={14} />
										</button>
									</span>
								</div>
								<div class="brief-pop-list">
									{#if briefs.length === 0}
										<p class="brief-pop-empty">{tr('Belum ada brief.')}</p>
									{/if}
									{#each briefs as b (b.id)}
										{@const isActive = b.id === profileId}
										<a
												class="brief-pop-row"
												class:active={isActive}
												href={`/klien/${brandUrl}/profiler/${b.short_code || b.id}`}
												onclick={() => (cardOpen = false)}
										>
												<span class="min-w-0 flex-1 truncate">{b.name}</span>
												{#if b.pinned}<Pin size={12} class="shrink-0" style="color: var(--muted-foreground);" />{/if}
										</a>
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
					<button
						class="brief-picker"
						class:brief-picker-open={actionsOpen}
						type="button"
						aria-expanded={actionsOpen}
						aria-label={tr('Tindakan Copilot')}
						onclick={() => (actionsOpen = !actionsOpen)}
					>
						<span class="truncate">{tr('Copilot')}</span>
						<svg xmlns="http://www.w3.org/2000/svg" width="1em" height="1em" viewBox="0 0 24 24" aria-hidden="true">
							<path d="M0 0h24v24H0z" fill="none" />
							<path fill="none" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M15.55 2.73h1.8c1.9 0 2.84 0 3.42.58c.59.59.59 1.53.59 3.42v1.8M8.97 21.28H6.73c-1.9 0-2.84 0-3.42-.58c-.59-.59-.59-1.53-.59-3.42v-2.25m11.7-5.36l5.94-5.94M9.7 14.3l-5.98 5.97" />
						</svg>
					</button>

					{#if actionsOpen}
						<button class="brief-pop-backdrop" type="button" aria-label={tr('Tutup')} onclick={() => (actionsOpen = false)}></button>
						<div
							class="brief-pop brief-pop-right"
							role="dialog"
							aria-label={tr('Tindakan Copilot')}
							tabindex="-1"
							onkeydown={(e) => {
								if (e.key === 'Escape') actionsOpen = false;
							}}
						>
							<div class="brief-pop-head">
								<span class="brief-pop-title">{tr('Tindakan Copilot')}</span>
							</div>
							<div class="brief-pop-list">
								<button class="brief-pop-row" type="button" onclick={() => { actionsOpen = false; restartChat(); }}>
									<RotateCcw size={13} />{tr('Mulai Ulang')}
								</button>
								<button class="brief-pop-row" type="button" onclick={() => { actionsOpen = false; document.getElementById('brief-doc-input')?.click(); }}>
									<Upload size={13} />{tr('Unggah Dokumen (PDF/DOCX)')}
								</button>
							</div>
						</div>
					{/if}

				</div>
				</div>
				</div>

			{#if error}
				<p class="mx-3 mt-2 rounded-lg p-2 text-sm" style="background: var(--warning-soft); color: var(--warning);">{error}</p>
			{/if}

			{#if draggingFiles}
				<div
					class="pointer-events-none absolute inset-2 z-20 flex items-center justify-center rounded-xl border-2 border-dashed"
					style="border-color: var(--primary); background: color-mix(in srgb, var(--primary) 8%, transparent);"
				>
					<p class="text-sm font-medium" style="color: var(--primary);">{tr('Lepaskan untuk melampirkan dokumen')}</p>
				</div>
			{/if}

			<div bind:this={threadEl} class="flex min-h-0 flex-1 flex-col overflow-y-auto px-3 py-3">
				{#if !loading && messages.length === 0}
					<div class="m-auto w-full max-w-2xl px-4">
						<!-- Sapaan pembuka: judul besar + deskripsi, bukan gelembung chat. -->
						<h1 class="text-center text-2xl font-semibold tracking-tight">{tr('Bangun brief brand-mu lewat obrolan')}</h1>
						<p class="mx-auto mt-2 max-w-md text-center text-sm leading-relaxed text-muted-foreground">
							{tr('Ceritakan brand-mu, sisanya aku tanyakan satu per satu.')}
						</p>
						<!-- Empat jalan masuk, dua kolom dua baris. -->
						<div class="mt-7 grid grid-cols-2 gap-3">
							<button class="brief-cta" type="button" onclick={() => { urlOpen = true; sidebarOpen = false; }}>
								<span class="brief-cta-icon"><Link2 size={16} /></span>
								<span class="min-w-0">
									<span class="brief-cta-title">{tr('Tempel URL website')}</span>
									<span class="brief-cta-sub">{tr('Aku baca dan tangkap sendiri')}</span>
								</span>
							</button>
							<button class="brief-cta" type="button" onclick={() => document.getElementById('brief-doc-input')?.click()}>
								<span class="brief-cta-icon"><Upload size={16} /></span>
								<span class="min-w-0">
									<span class="brief-cta-title">{tr('Unggah dokumen')}</span>
									<span class="brief-cta-sub">{tr('Company profile / PDF / DOCX')}</span>
								</span>
							</button>
							<button class="brief-cta" type="button" onclick={() => { pasteOpen = true; sidebarOpen = false; }}>
								<span class="brief-cta-icon"><ClipboardList size={16} /></span>
								<span class="min-w-0">
									<span class="brief-cta-title">{tr('Tempel teks siap pakai')}</span>
									<span class="brief-cta-sub">{tr('Ringkasan atau keterangan bisnis')}</span>
								</span>
							</button>
							<button class="brief-cta" type="button" onclick={() => document.getElementById('brief-composer')?.focus()}>
								<span class="brief-cta-icon"><MessageSquare size={16} /></span>
								<span class="min-w-0">
									<span class="brief-cta-title">{tr('Ceritakan dari awal')}</span>
									<span class="brief-cta-sub">{tr('Ngobrol bebas, aku arahkan')}</span>
								</span>
							</button>
						</div>
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
									<span class="brief-avatar brief-avatar-ai" aria-hidden="true"><img src="/tauthub-icon.png" alt="" /></span>
									<div class="flex min-w-0 flex-col items-start gap-1">
										<div class="brief-markdown max-w-[92%] rounded-2xl rounded-tl-sm border px-4 py-3 text-[13px] leading-relaxed shadow-sm" style="border-color: var(--border); background: var(--card);">
											{@html sanitizeHtml(renderMarkdown(m.content))}
										</div>
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
								<span class="brief-avatar brief-avatar-ai" aria-hidden="true"><img src="/tauthub-icon.png" alt="" /></span>
								<div class="max-w-[92%] whitespace-pre-wrap rounded-2xl rounded-tl-sm border px-4 py-3 text-[13px] leading-relaxed shadow-sm" style="border-color: var(--border); background: var(--card);">
									{streamingText}<span class="inline-block animate-pulse">▍</span>
								</div>
							</div>
						{:else if streaming}
							<div class="flex items-start gap-2.5">
								<span class="brief-avatar brief-avatar-ai" aria-hidden="true"><img src="/tauthub-icon.png" alt="" /></span>
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

			<!-- Composer + layanan, menempel di bawah tanpa ikut kerek halaman. -->
			<div class="border-t px-3 py-2" style="border-color: var(--border);">
				{#if !configured}
					<p class="mx-auto mb-1.5 max-w-2xl text-xs" style="color: var(--warning);">
						{tr('AI ini belum diaktifkan admin. Isi brief tetap bisa dilihat dan dikoreksi lewat Ringkasan.')}
					</p>
				{/if}

				<div class="mx-auto max-w-2xl">
					{#if urlOpen}
						<div class="mb-1.5 flex items-center gap-1.5">
							<input class="app-input flex-1 !min-h-8 !py-1 text-sm" placeholder="https://brand-ku.com" bind:value={urlValue}
								onkeydown={(e) => { if (e.key === 'Enter') { e.preventDefault(); void addWebsiteUrl(); } }} />
							<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-sm" type="button" disabled={workingState || !urlValue.trim()} onclick={() => void addWebsiteUrl()}>
								{tr('Baca')}
							</button>
							<button class="app-btn app-btn-soft !min-h-8 !px-2" type="button" aria-label={tr('Batal')} onclick={() => { urlOpen = false; urlValue = ''; }}>
								<X size={14} />
							</button>
						</div>
					{/if}

					{#if pasteOpen}
						<div class="mb-1.5">
							<textarea class="app-input w-full !py-1 text-sm" rows="3" placeholder={tr('Tempel teks tentang brand-mu di sini…')} bind:value={composer}></textarea>
							<div class="mt-1 flex justify-end gap-1.5">
								<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={() => (pasteOpen = false)}>{tr('Tutup')}</button>
							</div>
						</div>
					{/if}

					<div class="brief-composer">
						{#if uploadingNow}
							<p class="flex items-center gap-1.5 pb-1 text-[11px] text-muted-foreground">
								<LoaderCircle size={12} class="animate-spin" />{tr('Mengunggah dokumen…')}
							</p>
						{/if}

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

						<div class="brief-composer-bar">
							<label class="brief-composer-icon cursor-pointer" aria-label={tr('Lampirkan dokumen')}>
								<Plus size={19} />
								<input
									id="brief-doc-input"
									class="hidden"
									type="file"
									multiple
									accept=".pdf,.docx,.txt,.md,.csv,application/pdf"
									onchange={(e) => {
										stageFiles((e.currentTarget as HTMLInputElement).files);
										(e.currentTarget as HTMLInputElement).value = '';
									}}
								/>
							</label>
							<button
								class="brief-composer-icon"
								class:active={urlOpen}
								type="button"
								aria-label={tr('Tempel URL website')}
								onclick={() => (urlOpen = !urlOpen)}
							>
								<Link2 size={17} />
							</button>
							<span class="flex-1"></span>
							<button
								class="brief-composer-send"
								type="button"
								aria-label={streaming ? tr('Hentikan generasi') : tr('Kirim')}
								disabled={streaming || uploadingNow ? true : !composer.trim() && pendingDocs.length === 0}
								onclick={() => (streaming ? controller?.abort() : void send())}
							>
								{#if streaming}<Square size={13} /><span>{tr('Hentikan')}</span>{:else}<ArrowUp size={16} /><span>{tr('Kirim')}</span>{/if}
							</button>
						</div>
					</div>

					<p class="mt-1 text-center text-[11px] text-muted-foreground">
						{#if streaming}
							{tr('Sedang menyusun balasan — tombol stop menghentikannya.')}
						{:else}
							{tr('Enter kirim, Shift+Enter baris baru. Seret file ke sini atau tempel (Ctrl+V) untuk melampirkan dokumen.')}{#if sources.length > 0}&nbsp;·&nbsp;{sources.length} {tr('sumber dibaca')}{/if}
						{/if}
					</p>
				</div>
			</div>
			</div>
		{:else}

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
								</p>
							</div>
							<span
								class="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-[11px] font-bold"
								style="background: {progressPct >= 100 ? 'var(--success-soft)' : 'var(--warning-soft)'}; color: {progressPct >= 100 ? 'var(--success)' : 'var(--warning)'};"
							>
								<span class="h-1.5 w-1.5 rounded-full" style="background: {progressPct >= 100 ? 'var(--success)' : 'var(--warning)'};"></span>
								{progressPct}% {tr('Siap Rilis')}
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
										<p class="text-[11px] text-muted-foreground">{tr('Data kanonik primer pengenal brand di sistem search & LLM')}</p>
									</div>
								</div>
								<div class="mt-4 space-y-3">
									{#each [['canonical_name', 'Nama kanonik entitas'], ['entity_category', 'Kategori entitas'], ['canonical_description', 'Deskripsi kanonik'], ['boilerplate', 'Boilerplate "Tentang [brand]"']] as pair (pair[0])}
										<div>
											<p class="mb-1 block text-[11px] font-bold uppercase tracking-wider text-muted-foreground">{pair[1]}</p>
											<div class="rounded-xl border p-3" style="background: color-mix(in srgb, var(--muted) 60%, white); border-color: var(--border);">
												{#if String(ringkasan[pair[0]] ?? '').trim()}
													<p class="text-xs leading-relaxed">{ringkasan[pair[0]]}</p>
												{:else}
													<p class="text-xs italic" style="color: var(--muted-foreground);">{tr('Belum diisi — lengkapi lewat Copilot.')}</p>
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
											<p class="text-xs italic" style="color: var(--muted-foreground);">{tr('Belum ada — lengkapi lewat obrolan Copilot.')}</p>
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
											<p class="text-xs italic" style="color: var(--muted-foreground);">{tr('Belum ada — lengkapi lewat obrolan Copilot.')}</p>
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
											<p class="mb-1 block text-[11px] font-bold uppercase tracking-wider text-muted-foreground">{pair[1]}</p>
											<div class="rounded-xl border p-3" style="background: color-mix(in srgb, var(--muted) 60%, white); border-color: var(--border);">
												{#if String(ringkasan[pair[0]] ?? '').trim()}
													<p class="text-xs leading-relaxed">{ringkasan[pair[0]]}</p>
												{:else}
													<p class="text-xs italic" style="color: var(--muted-foreground);">{tr('Belum diisi — lengkapi lewat Copilot.')}</p>
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
										<p class="text-[11px] text-muted-foreground">{tr('Pertanyaan yang ingin brand Anda menangkan di jawaban mesin AI')}</p>
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
											{tr('Belum ada pertanyaan target — lengkapi lewat obrolan Copilot.')}
										</p>
									{/if}
								</div>
								<div class="mt-4 flex items-center justify-between gap-2 border-t pt-3 text-[11px]" style="border-color: var(--border);">
									<span class="text-muted-foreground">{tr('Mesin Terhubung: ChatGPT 4o, Gemini 1.5, Perplexity AI')}</span>
									<button class="font-bold underline-offset-2 hover:underline" style="color: var(--primary);" type="button" onclick={() => (activeTab = 'copilot')}>
										{tr('Uji Respon LLM')} →
									</button>
								</div>
							</section>
						</div>
					</div>
				</div>
			</div>
		{/if}
	</main>

	<!-- ── Pratinjau JSON (Entity Source of Truth) ───────────────────── -->
	{#if jsonPreviewOpen}
		<div class="brief-modal-overlay" role="presentation">
			<button class="brief-modal-backdrop" type="button" aria-label={tr('Tutup')} onclick={() => (jsonPreviewOpen = false)}></button>
			<div
				class="brief-modal brief-modal-wide"
				role="dialog"
				aria-modal="true"
				aria-label={tr('Pratinjau JSON')}
				tabindex="-1"
				onkeydown={(e) => {
					if (e.key === 'Escape') jsonPreviewOpen = false;
				}}
			>
				<h2 class="brief-modal-title">{tr('Pratinjau JSON')}</h2>
				<p class="brief-modal-text">{tr('Data profil entitas yang sudah tersimpan di brief ini.')}</p>
				<pre class="brief-json">{previewJson}</pre>
				<div class="brief-modal-actions">
					<button class="app-btn app-btn-soft" type="button" onclick={() => (jsonPreviewOpen = false)}>{tr('Tutup')}</button>
					<button class="app-btn app-btn-primary" type="button" onclick={copyJson}>{tr('Salin JSON')}</button>
				</div>
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
	/* Avatar AI memakai lencana logo Tauthub (bukan ikon petir). */
	.brief-avatar-ai { background: var(--card); overflow: hidden; }
	.brief-avatar-ai img {
		display: block;
		width: 100%;
		height: 100%;
		border-radius: inherit;
		object-fit: cover;
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


	.brief-cta {
		display: flex;
		align-items: flex-start;
		gap: 12px;
		padding: 14px 16px;
		border-radius: 14px;
		border: 1px solid var(--border);
		background: var(--card);
		text-align: left;
		transition: border-color 0.15s ease, transform 0.15s ease, box-shadow 0.15s ease;
	}
	.brief-cta:hover {
		border-color: var(--primary);
		background: var(--accent);
	}
	.brief-cta-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 36px;
		height: 36px;
		border-radius: 10px;
		flex-shrink: 0;
		background: var(--secondary);
		color: var(--primary);
	}
	.brief-cta-title { display: block; font-size: 14px; font-weight: 600; }
	.brief-cta-sub { display: block; margin-top: 2px; font-size: 12px; color: var(--muted-foreground); }

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

	/* Isi dalam .brief-markdown berasal dari {@html} hasil renderMarkdown, jadi
	   penyeleksinya harus :global — Svelte tidak bisa melihat elemen yang ada
	   di dalam string HTML. */
	.brief-markdown :global(p) { margin: 0 0 0.5rem; }
	.brief-markdown :global(p:last-child) { margin-bottom: 0; }
	.brief-markdown :global(ul),
	.brief-markdown :global(ol) { margin: 0.25rem 0 0.5rem 1.1rem; }
	.brief-markdown :global(ul) { list-style: disc; }
	.brief-markdown :global(ol) { list-style: decimal; }
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

	/* Kartu composer ala referensi: lampiran di atas, kolom teks di tengah,
	   tombol lampirkan di kiri bawah, tombol kirim bulat di kanan bawah. */
	.brief-composer {
		padding: 10px 12px 8px;
		border: 1px solid var(--input);
		border-radius: 16px;
		background: var(--card);
		box-shadow: var(--shadow-sm);
		transition: border-color 0.15s ease, box-shadow 0.15s ease;
	}
	.brief-composer:focus-within {
		border-color: var(--primary);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 14%, transparent);
	}
	.brief-composer-files {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		padding-bottom: 8px;
	}
	.brief-file-chip {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		max-width: 100%;
		padding: 4px 6px 4px 8px;
		border-radius: 10px;
		border: 1px solid var(--border);
		background: var(--secondary);
		font-size: 12px;
	}
	.brief-composer-input {
		display: block;
		width: 100%;
		min-height: 26px;
		max-height: 160px;
		padding: 2px 2px 8px;
		border: none;
		outline: none;
		background: transparent;
		color: var(--foreground);
		font-size: 15px;
		line-height: 1.6;
		resize: none;
	}
	.brief-composer-input::placeholder { color: var(--muted-foreground); }
	.brief-composer-bar {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.brief-composer-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border-radius: var(--radius-button);
		color: var(--muted-foreground);
		transition: background-color 0.12s ease, color 0.12s ease;
	}
	.brief-composer-icon:hover { background: var(--secondary); color: var(--foreground); }
	.brief-composer-icon.active { background: var(--accent); color: var(--accent-foreground); }
	/* Tombol kirim berlabel, sejajar gaya tombol aksi utama di desain referensi. */
	.brief-composer-send {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 7px;
		min-height: 34px;
		padding: 0 14px;
		border-radius: var(--radius-button);
		background: var(--primary);
		color: var(--primary-foreground);
		font-size: 12.5px;
		font-weight: 700;
		box-shadow: 0 1px 2px color-mix(in srgb, var(--primary) 32%, transparent);
		transition: opacity 0.12s ease, background-color 0.12s ease;
	}
	.brief-composer-send:hover:not(:disabled) { background: color-mix(in srgb, var(--primary) 88%, black); }
	.brief-composer-send:disabled {
		background: var(--secondary);
		color: var(--muted-foreground);
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
	/* Pemilih brief di kiri area kerja: nama brief sebagai tombol buka daftar. */
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
	.brief-picker:hover { border-color: var(--primary); background: var(--accent); color: var(--accent-foreground); }

	/* Jam kirim pesan (waktu lokal browser) di bawah gelembung. */
	.brief-time { padding: 0 4px; font-size: 11px; color: var(--muted-foreground); }

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

	/* Dialog pratinjau JSON: lebih lebar + blok data yang bisa digulir. */
	.brief-modal-wide { max-width: 640px; }
	.brief-json {
		margin-top: 12px;
		max-height: 320px;
		overflow: auto;
		padding: 12px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--secondary);
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
		font-size: 11.5px;
		line-height: 1.55;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}

	/* Popover pemilih brief: kartu terang, header + hitungan + daftar. */
	.brief-picker-open { border-color: var(--primary); background: var(--accent); color: var(--accent-foreground); }
	.brief-pop-backdrop { position: fixed; inset: 0; z-index: 40; cursor: default; }
	.brief-pop {
		position: absolute;
		top: calc(100% + 6px);
		left: 16px;
		z-index: 50;
		width: 300px;
		max-height: 340px;
		overflow-y: auto;
		padding: 12px;
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
	.brief-card-toggle svg { transition: transform 0.22s ease; }

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
		width: 320px;
		max-height: none;
	}
	.brief-pop-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
	/* Identitas Copilot di dalam popover: panel lembut selebar popover. */
	.brief-pop-id {
		display: flex;
		align-items: center;
		gap: 12px;
		margin: -12px -12px 10px;
		padding: 12px;
		border-bottom: 1px solid var(--border);
		border-radius: 14px 14px 0 0;
		background: color-mix(in srgb, var(--muted) 45%, white);
	}
	.brief-pop-title { font-size: 13px; font-weight: 700; color: var(--foreground); }
	.brief-pop-actions { display: inline-flex; align-items: center; gap: 4px; }
	.brief-pop-count { font-size: 12px; font-weight: 700; color: var(--success); }
	.brief-pop-icon {
		display: inline-flex; align-items: center; justify-content: center;
		width: 26px; height: 26px; border-radius: 7px; color: var(--muted-foreground);
		transition: background-color 0.12s ease, color 0.12s ease;
	}
	.brief-pop-icon:hover:not(:disabled) { background: var(--secondary); color: var(--foreground); }
	.brief-pop-list { margin-top: 4px; display: flex; flex-direction: column; gap: 2px; }
	.brief-pop-empty { padding: 6px 4px; font-size: 12px; color: var(--muted-foreground); }
	/* Baris brief: yang tidak aktif abu-abu, yang aktif hitam dengan latar abu tipis. */
	.brief-pop-row {
		display: flex; align-items: center; gap: 9px;
		width: 100%;
		padding: 7px 9px; border-radius: 8px;
		font-size: 12.5px; color: var(--muted-foreground); text-decoration: none;
		background: none; border: none; text-align: left; cursor: pointer;
		transition: background-color 0.12s ease, color 0.12s ease;
	}
	/* Popover aksi Copilot melayang di sisi kanan. */
	.brief-pop-right { left: auto; right: 16px; }
	.brief-pop-row:hover { background: var(--secondary); color: var(--foreground); }
	.brief-pop-row.active { background: var(--secondary); color: var(--foreground); font-weight: 600; }
	@media (prefers-reduced-motion: reduce) {
		.thinking-dots span { animation: none; opacity: 0.6; }
		.brief-cta, .brief-cta:hover { transform: none; box-shadow: none; }
	}

</style>
