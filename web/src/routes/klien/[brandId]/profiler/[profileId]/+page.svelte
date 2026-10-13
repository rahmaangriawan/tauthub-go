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
		ArrowUp, Check, ClipboardList, Copy, FileText, Link2, LoaderCircle, MessageSquare,
		MoreHorizontal, PanelRight, Pencil, Pin, Plus, Search, Sparkle, Square, SquarePen, Trash2, Upload, X
	} from '@lucide/svelte';
	import PageTitle from '$lib/components/page-title.svelte';
	import SlideOver from '$lib/components/slide-over.svelte';
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
	type ChatRow = { id?: string; role: 'user' | 'assistant'; content: string };

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

	// Lampiran dokumen: dipasang dulu sebagai chip, baru diunggah saat Kirim.
	type PendingDoc = { file: File };
	let pendingDocs = $state<PendingDoc[]>([]);
	let uploadingNow = $state(false);
	let draggingFiles = $state(false);
	let dragDepth = 0;

	// Panel Ringkasan
	let summaryOpen = $state(false);
	let ringkasan = $state<Record<string, string>>({});
	let saving = $state(false);
	let showLayer2 = $state(false);
	let summaryNotice = $state('');

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

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
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

	$effect(() => {
		if (threadEl) threadEl.scrollTop = threadEl.scrollHeight;
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

		messages = [...messages, { role: 'user', content: message }];
		streaming = true;
		streamingText = '';
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
					} else if (ev === 'done') {
						finished = true;
						messages = [...messages, { role: 'assistant', content: String(data.reply ?? partial) }];
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
				if (streamingText) messages = [...messages, { role: 'assistant', content: streamingText }];
				streamError = tr('Generasi dihentikan.');
			} else {
				streamError = e instanceof Error ? e.message : tr('Gagal mengirim pesan.');
			}
		} finally {
			working(false);
			generationCount++;
			streaming = false;
			streamingText = '';
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
			summaryNotice = tr('Belum ada perubahan.');
			return;
		}
		saving = true;
		summaryNotice = '';
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
				summaryNotice = tr('Seksi ini belum punya kolom penyimpanan — periksa skema field di admin.');
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
			summaryNotice = tr('Koreksi tersimpan.');
			await hydrate();
		} catch (e) {
			summaryNotice = e instanceof Error ? e.message : tr('Gagal menyimpan koreksi.');
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
	class="flex overflow-hidden"
	style="height: calc(100dvh - {shellTop}px); margin: -32px -28px -64px;"
>

	<!-- ── Sidebar riwayat brief ─────────────────────────────────────── -->
	<aside
		bind:this={railEl}
		class="brief-rail fixed inset-y-0 left-0 z-40 shrink-0 border-r transition-transform lg:static lg:translate-x-0"
		class:dragging-rail={draggingRail}
		style="border-color: var(--border); top: 64px; height: calc(100dvh - 64px); width: {sidebarWidth}px;"
		class:translate-x-0={sidebarOpen}
		class:-translate-x-full={!sidebarOpen}
	>
		<div class="flex h-full flex-col">
			<!-- Baris atas: ciri di kiri; cari + Ringkasan di kanan. -->
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
					onclick={() => { summaryOpen = !summaryOpen; sidebarOpen = false; }}
				>
					<span class="flex items-center justify-between text-[13px] font-medium">
						{tr('Ringkasan')}
						<PanelRight size={14} />
					</span>
					<span class="mt-1.5 block h-1 w-full overflow-hidden rounded-full" style="background: rgba(15,23,42,.12);">
						<span class="block h-full" style="width: {progress.total ? (progress.filled / progress.total) * 100 : 0}%; background-color: var(--primary);"></span>
					</span>
					<span class="mt-1 block text-[11px] opacity-80">
						{progress.filled} / {progress.total} {tr('field wajib terisi')}
					</span>
				</button>
			</div>
		</div>

		<!-- Pegangan lebar: tarik untuk mengatur; klik dua kali kembali ke bawaan. -->
		<div
			class="brief-rail-handle"
			class:dragging={draggingRail}
			role="separator"
			aria-orientation="vertical"
			onpointerdown={(e) => startRailDrag(e)}
			ondblclick={(e) => { e.preventDefault(); sidebarWidth = 270; try { localStorage.setItem(SIDEBAR_WIDTH_KEY, '270'); } catch { /* abaikan */ } }}
		></div>
	</aside>

	{#if sidebarOpen}
		<!-- Pola backdrop yang sama dengan modal katalog media. -->
		<div
			class="fixed inset-0 z-30 lg:hidden"
			style="background-color: rgba(15,23,42,.45);"
			role="presentation"
			onclick={() => (sidebarOpen = false)}
		></div>
	{/if}

	{#if menuId}
		<!-- Klik di mana pun menutup menu aksi; item pembukanya dinaikkan z-nya
		     supaya menunya tetap di atas backdrop ini dan bisa diklik. -->
		<div class="fixed inset-0 z-20" role="presentation" onclick={() => (menuId = null)}></div>
	{/if}

	<!-- ── Chat ──────────────────────────────────────────────────────── -->
	<main
		class="relative flex min-w-0 flex-1 flex-col"
		ondragenter={onChatDragEnter}
		ondragleave={onChatDragLeave}
		ondragover={(e) => {
			if (hasFileDrag(e)) e.preventDefault();
		}}
		ondrop={onChatDrop}
	>
			<header class="flex items-center justify-between gap-2 border-b px-3 py-2" style="border-color: var(--border);">
				<div class="flex min-w-0 items-center gap-2">
					<button class="app-btn app-btn-soft !min-h-7 !px-2 lg:hidden" type="button" aria-label={tr('Buka daftar brief')} onclick={() => (sidebarOpen = true)}>
						<Plus size={15} />
					</button>
					<p class="truncate text-sm font-semibold">{profileName || tr('Brief')}</p>
					<span class="hidden rounded-full px-2 py-0.5 text-[11px] sm:inline" style="background: var(--secondary); color: var(--secondary-foreground);">
						{phase}
					</span>
				</div>

				<div class="relative flex shrink-0 items-center">
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
			</header>

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
					<h1 class="text-center text-2xl font-bold tracking-tight">{tr('Bangun brief brand-mu lewat ngobrol')}</h1>
					<p class="mx-auto mt-2 max-w-md text-center text-sm leading-relaxed text-muted-foreground">
						{tr('Cerita saja soal brand-mu. Aku yang tanya sisanya — satu hal dulu, santai.')}
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
							<div class="flex justify-end">
								<p class="max-w-[85%] whitespace-pre-wrap rounded-2xl rounded-br-sm px-3.5 py-2 text-[15px] leading-relaxed" style="background: var(--primary); color: var(--primary-foreground);">{m.content}</p>
							</div>
						{:else}
							<!-- Sisi lawan chat: balasan asisten selalu di kiri, dengan baris
							     aksi (salin) di bawah gelembungnya. -->
							<div class="brief-msg flex flex-col items-start gap-1">
								<!-- renderMarkdown escape HTML lebih dulu; sanitizeHtml menyaring sisanya -->
								<div class="brief-markdown max-w-[92%] rounded-2xl rounded-bl-sm border px-3.5 py-2.5 text-[15px] leading-relaxed" style="border-color: var(--border);">
									{@html sanitizeHtml(renderMarkdown(m.content))}
								</div>
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
						{/if}
					{/each}

					{#if streamingText}
						<div class="flex justify-start">
							<div class="max-w-[92%] whitespace-pre-wrap rounded-2xl rounded-bl-sm border px-3.5 py-2.5 text-[15px] leading-relaxed" style="border-color: var(--border);">
								{streamingText}<span class="inline-block animate-pulse">▍</span>
							</div>
						</div>
					{:else if streaming}
						<div class="flex justify-start">
							<div class="brief-thinking">
								<span class="thinking-dots" aria-hidden="true"><span></span><span></span><span></span></span>
								<span>{tr('Sedang berpikir…')}</span>
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
				{#if showWidget && activeField}
					<div class="mb-1.5 rounded-lg border p-2" style="border-color: var(--primary);">
						<p class="text-xs font-semibold">{widgetLabel(activeField)}</p>
						{#if showWidget === 'enum'}
							<div class="mt-1.5">
								<Select
									ariaLabel={widgetLabel(activeField)}
									placeholder={tr('Pilih…')}
									options={[{ value: '', label: tr('Pilih…') }, ...ENUM_CHOICES[activeField.key].map((v: string) => ({ value: v, label: v }))]}
									onchange={(value: string) => (widgetValue = value)}
								/>
								<button class="app-btn app-btn-primary mt-1.5 !min-h-7 !text-xs" type="button" disabled={!widgetValue || streaming} onclick={() => activeField && void commitWidget(activeField.key, widgetValue)}>
									{tr('Pakai ini')}
								</button>
							</div>
						{:else if showWidget === 'bool'}
							<div class="mt-1.5 flex flex-wrap gap-1.5">
								{#each ['ya', 'tidak'] as opt (opt)}
									<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={() => commitWidget(activeField.key, opt)}>{opt}</button>
								{/each}
							</div>
						{/if}
					</div>
				{/if}

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

				<!-- Kartu composer: lampiran di dalam kartu (1), kolom teks (2),
				     tombol lampirkan "+" (3), tombol kirim bulat (4). -->
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
							{#if streaming}<Square size={14} />{:else}<ArrowUp size={17} />{/if}
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
	</main>

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

	<!-- ── Panel Ringkasan ───────────────────────────────────────────── -->
	<SlideOver open={summaryOpen} title={tr('Ringkasan')} subtitle={profileName} onClose={() => (summaryOpen = false)}>
		<div class="space-y-4">
			<div>
				<div class="h-1.5 w-full overflow-hidden rounded-full" style="background: var(--secondary);">
					<div class="h-full" style="width: {progress.total ? (progress.filled / progress.total) * 100 : 0}%; background-color: var(--primary);"></div>
				</div>
				<p class="mt-1.5 text-xs text-muted-foreground">
					{progress.filled} / {progress.total} {tr('field wajib terisi')} · {sources.length} {tr('sumber dibaca')}
				</p>
				<p class="mt-2 text-[11px] leading-relaxed" style="color: var(--muted-foreground);">
					{tr('Field di sini yang diisi AI saat kamu ngobrol — nggak perlu diisi manual. Panel ini untuk meninjau: kalau tangkapan AI kurang pas, koreksi langsung di sini dan itu tersimpan sebagai konfirmasimu.')}
				</p>
			</div>

			{#if summaryNotice}
				<p class="rounded-lg p-2 text-xs" style="background: var(--secondary);">{summaryNotice}</p>
			{/if}

			{#each bySection(1) as group (group.section)}
				<section>
					<h3 class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{group.section}</h3>
					<div class="mt-2 space-y-2">
						{#each group.items as f (f.key)}
							{@const st = fieldStates[f.key]}
							<label class="block">
								<span class="flex items-center justify-between gap-2 text-[11px]">
									<span class="font-medium">{f.label || f.key}</span>
									<span class="text-muted-foreground">{stateLabel(f.key)}</span>
								</span>
								<textarea
									class="app-input mt-0.5 w-full text-xs transition-opacity"
									rows={(f.widget === 'longtext' || (ringkasan[f.key] ?? '').length > 80) ? 3 : 1}
									style="opacity: {isFilled(f.key) ? 1 : 0.55};"
									bind:value={ringkasan[f.key]}
								></textarea>
							</label>
						{/each}
					</div>
				</section>
			{/each}

			{#if lapis2.length > 0}
				<section class="border-t pt-3" style="border-color: var(--border);">
					<button class="flex w-full items-center justify-between gap-2 text-left" type="button" onclick={() => (showLayer2 = !showLayer2)}>
						<h3 class="text-xs font-bold uppercase tracking-wide text-muted-foreground">
							{tr('Konteks Brand (lapis 2)')} — {tr('otomatis dari website & dokumen')}
						</h3>
						<span class="text-xs text-muted-foreground">{showLayer2 ? '−' : '+'}</span>
					</button>

					{#if showLayer2}
						{#each lapis2 as group (group.section)}
							<div class="mt-3">
								<p class="text-[11px] font-semibold" style="color: var(--muted-foreground);">{group.section}</p>
								<div class="mt-1.5 space-y-2">
									{#each group.items as f (f.key)}
										<label class="block">
											<span class="text-[11px] text-muted-foreground">{f.label || f.key}</span>
											<textarea
												class="app-input mt-0.5 w-full text-xs"
												rows={(ringkasan[f.key] ?? '').length > 60 ? 3 : 1}
												bind:value={ringkasan[f.key]}
											></textarea>
										</label>
									{/each}
								</div>
							</div>
						{/each}
					{/if}
				</section>
			{/if}

			<p class="text-[11px] text-muted-foreground">{tr('Koreksi di panel ini langsung tersimpan di brief dan ditandai sebagai konfirmasimu.')}</p>
		</div>

		{#snippet footer()}
			<button class="app-btn app-btn-primary w-full" type="button" disabled={saving} onclick={() => void saveRingkasan()}>
				{saving ? tr('Menyimpan…') : tr('Simpan koreksi')}
			</button>
		{/snippet}
	</SlideOver>
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
	.brief-rail-handle:hover::after,
	.brief-rail-handle.dragging::after { background: var(--primary); }
	/* Di layar sempit sidebar berperan drawer: lebarnya tetap, pegangan disembunyikan. */
	@media (max-width: 1023px) {
		.brief-rail { width: 270px !important; }
		.brief-rail-handle { display: none; }
	}

	.brief-mark {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 26px;
		height: 26px;
		border-radius: 8px;
		background: var(--secondary);
		color: var(--primary);
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

	/* Lencana Beta: penanda fitur prarilis, warna peringatan supaya tidak
	   terbaca sebagai status produk yang sudah matang. */
	.brief-beta-chip {
		display: inline-flex;
		align-items: center;
		padding: 2px 9px;
		border: 1px solid color-mix(in srgb, var(--warning) 28%, transparent);
		border-radius: 999px;
		background: var(--warning-soft);
		color: var(--warning);
		font-size: 11px;
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
		border-radius: 12px;
		background: var(--card);
		font-size: 14px;
		font-weight: 600;
		color: var(--foreground);
		transition: background-color 0.12s ease, box-shadow 0.12s ease;
	}
	.brief-primary:hover { box-shadow: 0 1px 0 rgba(15, 23, 42, 0.06); background: var(--card); }

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
	.brief-item:hover { background: var(--secondary); }
	.brief-item[aria-current='page'] {
		background: var(--card);
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
	.brief-pin:hover { background: var(--secondary); }
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
	.brief-more:hover { background: var(--secondary); color: var(--foreground); }

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
		transform: translateY(-2px);
		box-shadow: 0 8px 20px rgba(15, 23, 42, 0.08);
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
	.brief-modal-actions .app-btn:not(.brief-modal-danger) {
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
		border: 1px solid var(--border);
		border-radius: 20px;
		background: var(--card);
		transition: border-color 0.15s ease, box-shadow 0.15s ease;
	}
	.brief-composer:focus-within {
		border-color: var(--primary);
		box-shadow: 0 0 0 3px rgba(43, 103, 236, 0.12);
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
		border-radius: 50%;
		color: var(--muted-foreground);
		transition: background-color 0.12s ease, color 0.12s ease;
	}
	.brief-composer-icon:hover { background: var(--secondary); color: var(--foreground); }
	.brief-composer-icon.active { background: var(--secondary); color: var(--primary); }
	.brief-composer-send {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 34px;
		height: 34px;
		border-radius: 50%;
		background: var(--primary);
		color: var(--primary-foreground);
		transition: opacity 0.12s ease, background-color 0.12s ease;
	}
	.brief-composer-send:disabled {
		background: var(--secondary);
		color: var(--muted-foreground);
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
		/* Kartu composer ala referensi: lampiran di atas, kolom teks di tengah,
	   tombol lampirkan di kiri bawah, tombol kirim bulat di kanan bawah. */
	.brief-composer {
		padding: 10px 12px 8px;
		border: 1px solid var(--border);
		border-radius: 20px;
		background: var(--card);
		transition: border-color 0.15s ease, box-shadow 0.15s ease;
	}
	.brief-composer:focus-within {
		border-color: var(--primary);
		box-shadow: 0 0 0 3px rgba(43, 103, 236, 0.12);
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
		border-radius: 50%;
		color: var(--muted-foreground);
		transition: background-color 0.12s ease, color 0.12s ease;
	}
	.brief-composer-icon:hover { background: var(--secondary); color: var(--foreground); }
	.brief-composer-icon.active { background: var(--secondary); color: var(--primary); }
	.brief-composer-send {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 34px;
		height: 34px;
		border-radius: 50%;
		background: var(--primary);
		color: var(--primary-foreground);
		transition: opacity 0.12s ease, background-color 0.12s ease;
	}
	.brief-composer-send:disabled {
		background: var(--secondary);
		color: var(--muted-foreground);
		cursor: not-allowed;
	}

	.brief-msg-action { opacity: 0.75; }
	}

	.brief-thinking {
		display: inline-flex;
		align-items: center;
		gap: 10px;
		padding: 10px 14px;
		border-radius: 16px 16px 4px 16px;
		border: 1px solid var(--border);
		background: var(--card);
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
	@media (prefers-reduced-motion: reduce) {
		.thinking-dots span { animation: none; opacity: 0.6; }
		.brief-cta, .brief-cta:hover { transform: none; box-shadow: none; }
	}
</style>
