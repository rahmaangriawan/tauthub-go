<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Agent 3: The Writer — dapur produksi artikel per media terpilih.
	// Port app/klien/[brandId]/taut-pintar/[id]/writer/client.tsx.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { CalendarClock, Check, CircleAlert, CircleCheck, Clock, Eye, FileText, Image, Images, Info, Link, Link2, ListChecks, LoaderCircle, Pencil, PencilLine, Send, ShieldCheck, Sparkles, Target, Trash2, TriangleAlert, Type, Upload, Wallet, Wrench, X } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, getAccessToken, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import AgentSteps from '../../_agent-steps.svelte';
	import TautBottomBar from '../../_bottom-bar.svelte';
	import NaskahModal from './_naskah-modal.svelte';
	import QcPanel, { QC_CHECK_COUNT } from './_qc-panel.svelte';
	import { MANUAL_PROFILER_REASON, buildTrackedUrl, stripScheme, withScheme, textToHtml, cleanPastedText, toJakartaDisplay, toLocalInput, isCampaignDone } from '../../_shared';
	import { tcToRp } from '$lib/credits/rupiah';
	import { ARTICLE_DEFAULT_COVER, sameOriginAssetUrl } from '$lib/media/cover';
	import { mediaDomainLabel } from '$lib/media/domain-label';
	import PageTitle from '$lib/components/page-title.svelte';
	import SlideOver from '$lib/components/slide-over.svelte';
	import DateTimePicker from '$lib/components/date-time-picker.svelte';

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);
	// Status kampanye dari GET /profile/draft. Final = view only.
	let campaignStatus = $state<string | null>(null);
	const viewOnly = $derived(isCampaignDone(campaignStatus));
	const VIEW_ONLY_MSG = 'Kampanye sudah selesai — mode lihat saja. Hubungi admin kalau perlu revisi.';

	type PlacementItem = {
		media_id: string;
		name: string;
		score: number | null;
		status: string;
		article_status: string;
		reason: string | null;
		credit_cost: number | null;
		allow_hyperlink: boolean | null;
		max_links: number | null;
		allow_embed: boolean;
		price?: number | null;
		/** Kredit penempatan hasil konversi harga media — SAMA dengan yang ditagih. */
		cost_tc?: number | null;
		/** Jumlah gambar di artikel tersimpan (untuk panel Biaya Gambar). */
		images_count?: number | null;
		regen_count: number | null;
		categories: string[];
		label_sponsor?: string;
		media_type?: string | null;
		angle_slug?: string | null;
		website_url?: string | null;
		/** Standar media mewajibkan kategori saat terbit. */
		category_required?: boolean;
		/** 'ai' bila artikel pernah dibuat The Writer; 'manual' bila diunggah. */
		article_source?: 'ai' | 'manual' | null;
		/** Aturan panjang artikel media (mis. kata_300_700 / custom_500_1000). */
		word_count_mode?: string | null;
	};
	type LinksForm = {
		tracking_mode: 'direct' | 'tauthub';
		primary_url: string;
		primary_anchor: string;
		backup_url: string;
		backup_anchor: string;
	};
	type LinkCounts = { dofollow: number; nofollow: number; sponsored: number };
	type ArticleStats = { words: number; images: number; links: number; cover: string | null; category: string | null };

	const EMPTY_LINKS: LinksForm = { tracking_mode: 'direct', primary_url: '', primary_anchor: '', backup_url: '', backup_anchor: '' };

	/** Izin backlink media: keputusan mitra via max_links (0 = tidak boleh);
	 *  allow_hyperlink hanya fallback baris legacy yang max_links-nya kosong. */
	function mediaAllowsLink(item: Pick<PlacementItem, 'allow_hyperlink' | 'max_links'>): boolean {
		return item.max_links != null ? item.max_links > 0 : item.allow_hyperlink !== false;
	}

	let campaignName = $state(tr('Kampanye'));
	let wallet = $state(0);
	let rateIdr = $state(0);
	let spokepersons = $state<Array<{ name: string; role: string; quote: string }>>([]);
	let linkStats = $state<Record<string, LinkCounts>>({});
	let articleStats = $state<Record<string, ArticleStats>>({});
	let articleTitles = $state<Record<string, string>>({});
	let placement = $state<PlacementItem[]>([]);
	type WriterAngle = { slug: string; name: string; description?: string | null; has_prompt: boolean; pick_count?: number };
	type WriterChannel = { slug: string; name: string; angles: WriterAngle[] };
	let angleChannels = $state<WriterChannel[]>([]);
	let anglesLoading = $state(true);
	let anglesError = $state('');
	// Jumlah analisis QC per media (badge di tombol "Analisis QC").
	let qcCounts = $state<Record<string, number>>({});
	// QC otomatis setelah "Simpan Artikel": ikon tombol berputar selama
	// pemindaian berjalan; setelah selesai angka badge menjadi HIJAU (tanda
	// QC selesai) — aturan pemilik produk.
	let qcRunning = $state<Record<string, boolean>>({});
	// Konfirmasi hapus media dari kampanye (soft delete, data artikel tetap aman).
	let removeTarget = $state<PlacementItem | null>(null);
	let removeBusy = $state(false);
	let removeError = $state('');
	let links = $state<LinksForm>(EMPTY_LINKS);
	let linksStatus = $state<'idle' | 'dirty' | 'saving' | 'saved' | 'error'>('idle');
	let loading = $state(true);
	let error = $state('');
	let backlinkEnabled = $state(false);
	let entryMode = $state<'ai' | 'manual' | null>(null);
	let regenMax = $state(2);
	// Tarif dari server (platform_settings / konfigurasi AI): setiap generate
	// ditagih 1× tarif ini, dan biaya bar memakai tarif sekarang untuk semua
	// penulisan/gambar AI milik kampanye.
	let writerCostCredits = $state(2);
	let imageCreditCost = $state(2);
	// Jumlah pemakaian AI kampanye ini (agregat ledger server): penulisan AI
	// (artikel baru + setiap regenerasi) dan gambar AI. Supaya klien yang
	// menulis/unggah artikelnya sendiri tidak melihat biaya AI; harganya
	// mengikuti tarif sekarang di atas — bukan nominal tertagih lampau.
	let writerChargeCount = $state(0);
	let writerRegenCount = $state(0);
	let imageChargeCount = $state(0);
	let trackingOn = $state(false);
	let genState = $state<Record<string, string>>({});
	let bulkRunning = $state(false);
	let bulkProgress = $state<{ done: number; total: number } | null>(null);
	let uploadBusyId = $state<string | null>(null);
	let checklistFor = $state<string | null>(null);
	let coverChecking = $state(false);
	let agentStatus = $state<{ configured: boolean; reason: string | null } | null>(null);
	let avatarFailed = $state<Record<string, boolean>>({});

	function cleanDomain(url: string | null | undefined, name: string): string {
		const raw = (url ?? '').replace(/^https?:\/\/(www\.)?/i, '').replace(/\/+$/, '').trim() || name.trim();
		return raw.toLowerCase();
	}

	const AVATAR_PALETTES = [
		{ bg: '#b91c1c', text: '#ffffff' },
		{ bg: '#334155', text: '#ffffff' },
		{ bg: '#0284c7', text: '#ffffff' },
		{ bg: '#d97706', text: '#ffffff' },
		{ bg: '#059669', text: '#ffffff' },
		{ bg: '#7c3aed', text: '#ffffff' },
		{ bg: '#0f172a', text: '#ffffff' },
		{ bg: '#ea580c', text: '#ffffff' },
		{ bg: '#dc2626', text: '#ffffff' },
		{ bg: '#0891b2', text: '#ffffff' }
	];

	function avatarStyle(name: string | null | undefined) {
		const s = (name ?? '').trim().toLowerCase();
		let h = 0;
		for (let i = 0; i < s.length; i += 1) h = (h * 31 + s.charCodeAt(i)) % AVATAR_PALETTES.length;
		return AVATAR_PALETTES[h];
	}

	const initialOf = (name: string | null | undefined) => (name ?? '').trim().charAt(0).toUpperCase() || '?';
	// Panel analisis QC (slide-over kanan) untuk satu artikel.
	let qcFor = $state<string | null>(null);
	let qcToken = $state(0);
	// Media yang hasil QC-nya menunggu dibuka: panel ditunda sampai modal naskah
	// tertutup supaya dua overlay tidak menumpuk dan kunci scroll tidak bentrok.
	let gatekeeperAgent = $state<{ configured: boolean; reason: string | null } | null>(null);
	// Distribusi langsung dari The Writer ketika QC sudah lulus (Gatekeeper dilewati).
	let distributable = $state<boolean | null>(null);
	let distributing = $state(false);
	let distributeFailed = $state(false);
	let topup = $state<{ needed: number; available: number } | null>(null);
	// Panel geser penerbitan: jadwal diatur PER MEDIA (boleh campur: satu terbit
	// langsung, satu terjadwal), dan cover tiap artikel dipastikan dulu.
	let publishOpen = $state(false);
	// Popover rincian biaya di bar bawah: terbuka saat hover, tetap terbuka saat diklik.
	let costPopoverOpen = $state(false);
	let costPinned = $state(false);
	// Popover kesiapan terbit: daftar prasyarat yang belum terpenuhi + aksi
	// "Cek Semua QC". Menggantikan pembukaan panel QC otomatis dari tombol terbit.
	let readinessOpen = $state(false);
	let bulkQcRunning = $state(false);
	// media_id -> nilai datetime-local; tidak ada entri = terbit langsung.
	let scheduleByMedia = $state<Record<string, string>>({});
	// Preset cepat yang sedang berlaku per media, untuk sorotan tombol. Kosong
	// berarti waktunya diketik manual sehingga tidak ada preset yang aktif.
	let quickPick = $state<Record<string, 'now' | '1h' | '24h' | '3d'>>({});
	let missingCovers = $state<{ mediaId: string; name: string }[]>([]);
	let coverBusy = $state(false);
	let publishError = $state('');
	let shortLinkDomain = $state<string | null>(null);
	let primaryCode = $state<string | null>(null);
	let backupCode = $state<string | null>(null);

	let editor = $state<{
		mediaId: string;
		mediaName: string;
		tab: 'tulis' | 'unggah' | 'generate';
		title: string;
		html: string;
		cover: string | null;
		category: string | null;
		metaDescription: string | null;
		linkLimit: number;
		/** Asal artikel tersimpan — menentukan tab yang tampil saat Edit Artikel. */
		source?: 'ai' | 'manual' | null;
		/** Aturan panjang artikel media (ditampilkan di Pengelola Naskah). */
		wordCountMode?: string | null;
	} | null>(null);
	let editorSaving = $state(false);
	let editorError = $state('');
	// Snapshot artikel dari BACKEND saat "Edit Artikel" dibuka — dipakai untuk
	// cek "ada perubahan?" saat Simpan: kalau sama, QC otomatis TIDAK dipicu
	// (hemat token) karena hasil pemindaian tetap valid. Upload/generate mulai
	// dari kosong → baseline null → QC selalu jalan.
	let editorBaseline = $state<{ title: string; html: string } | null>(null);
	// Pratinjau streaming: html parsial dari SSE saat AI menulis; null = idle.
	let streamHtml = $state<string | null>(null);

	let saveLinksTimer: number | null = null;
	let saveLinksSeq = 0;
	let hydrated = false;
	let linksDirty = false;

	const REASON_LABEL: Record<string, string> = $derived({
		'dinonaktifkan oleh admin': tr('AI Writer sedang dinonaktifkan oleh admin'),
		'base_url kosong': tr('AI Writer belum dikonfigurasi (server AI belum diatur admin)'),
		'api key belum diisi atau kedaluwarsa': tr('AI Writer belum bisa dipakai — API key belum diisi atau sudah kedaluwarsa. Hubungi admin.'),
		'model belum dipilih': tr('AI Writer belum dikonfigurasi (model AI belum dipilih admin)')
	});

	const manualMode = $derived(page.url.searchParams.get('mode') === 'manual');
	const manualFlow = $derived(entryMode ? entryMode === 'manual' : manualMode);
	const selected = $derived(placement.filter((p) => p.status === 'selected'));
	// ── Seleksi Dapur Produksi (aturan pemilik produk): centang per media,
	//    bar aksi muncul di bawah header; "Generate Semua" masih placeholder
	//    popup "Dalam Pengerjaan" — mesin bulk menyusul.
	let pickedIds = $state<Record<string, boolean>>({});
	let bulkPopupOpen = $state(false);
	const pickedList = $derived(
		Object.keys(pickedIds).filter((id) => pickedIds[id] && selected.some((m) => m.media_id === id))
	);
	const allPicked = $derived(selected.length > 0 && pickedList.length === selected.length);
	function togglePicked(id: string) {
		pickedIds = { ...pickedIds, [id]: !pickedIds[id] };
	}
	function pickAll() {
		if (allPicked) {
			pickedIds = {};
			return;
		}
		pickedIds = Object.fromEntries(selected.map((m) => [m.media_id, true]));
	}

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			void loadAgentStatus();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		const onPageHide = () => flushLinks();
		window.addEventListener('pagehide', onPageHide);
		return () => {
			if (saveLinksTimer) window.clearTimeout(saveLinksTimer);
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
			window.removeEventListener('pagehide', onPageHide);
		};
	});

	async function loadAgentStatus() {
		try {
			// Satu respons memuat keempat agen, jadi status gatekeeper untuk
			// panel QC didapat tanpa request tambahan.
			const res = await apiRequest(`/v1/brands/${brandId}/taut/agent-status?agent=writer`);
			if (res.ok) {
				const d = await res.json();
				const w = d.agents?.find((a: { agent: string }) => a.agent === 'writer');
				if (w) agentStatus = { configured: w.configured, reason: w.reason };
				const g = d.agents?.find((a: { agent: string }) => a.agent === 'gatekeeper');
				if (g) gatekeeperAgent = { configured: g.configured, reason: g.reason };
			}
		} catch {
			/* status opsional */
		}
	}

	async function loadAngles() {
		anglesLoading = true;
		anglesError = '';
		try {
			const res = await apiRequest(`${apiBase}/profiler/options`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat pilihan angle.')));
			const data = await res.json();
			if (!Array.isArray(data?.channels)) throw new Error(tr('Gagal memuat pilihan angle.'));
			if (data.channels.some((channel: WriterChannel) => !Array.isArray(channel.angles) || channel.angles.some((angle) => typeof angle.has_prompt !== 'boolean'))) {
				throw new Error(tr('Pilihan angle belum tersedia di server. Muat ulang setelah API diperbarui.'));
			}
			angleChannels = data.channels;
		} catch (reason) {
			angleChannels = [];
			anglesError = reason instanceof Error ? reason.message : tr('Gagal memuat pilihan angle.');
		} finally {
			anglesLoading = false;
		}
	}

	async function load() {
		try {
			const [placeRes, linksRes, draftRes, auditsRes] = await Promise.all([
				apiRequest(`${apiBase}/placement`),
				apiRequest(`${apiBase}/writer-links`),
				apiRequest(`${apiBase}/profile/draft`),
				apiRequest(`${apiBase}/gatekeeper/audits`),
				loadAngles()
			]);
			if (placeRes.ok) {
				const d = await placeRes.json();
				placement = d.media ?? [];
				const rm = Number(d.regen_max ?? 2);
				if (Number.isFinite(rm) && rm >= 0) regenMax = rm;
				const wc = Number(d.writer_cost_credits ?? 2);
				if (Number.isFinite(wc) && wc > 0) writerCostCredits = wc;
				const ic = Number(d.image_credit_cost ?? 0);
				if (Number.isFinite(ic) && ic >= 0) imageCreditCost = ic;
				const wcCount = Number(d.writer_charge_count ?? 0);
				if (Number.isFinite(wcCount) && wcCount >= 0) writerChargeCount = wcCount;
				const wr = Number(d.writer_regen_count ?? 0);
				if (Number.isFinite(wr) && wr >= 0) writerRegenCount = wr;
				const icCount = Number(d.image_charge_count ?? 0);
				if (Number.isFinite(icCount) && icCount >= 0) imageChargeCount = icCount;
			}
			// null = belum diketahui / gagal dibaca → tombol kembali ke jalur aman
			// (mampir ke halaman Gatekeeper).
			const auditsData = auditsRes.ok ? await auditsRes.json().catch(() => null) : null;
			distributable = auditsData?.canDistribute === true;
			const counts: Record<string, number> = {};
			for (const item of auditsData?.media ?? []) {
				const id = String(item?.media_id ?? '');
				if (id) counts[id] = Number(item?.audit?.count ?? 0);
			}
			qcCounts = counts;
			const linksData = linksRes.ok ? await linksRes.json() : null;
			const draftData = draftRes.ok ? await draftRes.json() : null;
			const bl = draftData?.profilerBacklink ?? draftData?.draft?.backlink ?? draftData?.crawl_result?.backlink;
			backlinkEnabled = Boolean(draftData?.profilerBacklink?.enabled ?? draftData?.draft?.backlink?.enabled);
			const hasBl = Boolean(bl?.target_url && bl?.anchor_text);

			if (linksData) {
				const filled: LinksForm = {
					tracking_mode: linksData.tracking_mode === 'tauthub' ? 'tauthub' : 'direct',
					primary_url: String(linksData.primary_url ?? ''),
					primary_anchor: linksData.primary_anchor ?? '',
					backup_url: String(linksData.backup_url ?? ''),
					backup_anchor: linksData.backup_anchor ?? ''
				};
				if (hasBl) {
					if (!filled.primary_url.trim() || !filled.primary_anchor.trim()) {
						filled.primary_url = filled.primary_url.trim() || stripScheme(bl.target_url);
						filled.primary_anchor = filled.primary_anchor.trim() || bl.anchor_text;
					} else if (!filled.backup_url.trim() || !filled.backup_anchor.trim()) {
						filled.backup_url = filled.backup_url.trim() || stripScheme(bl.target_url);
						filled.backup_anchor = filled.backup_anchor.trim() || bl.anchor_text;
					}
				}
				// Form yang masih dirty tidak boleh ditimpa balik oleh respons server.
				if (!linksDirty) {
					links = { ...filled, primary_url: stripScheme(filled.primary_url), backup_url: stripScheme(filled.backup_url) };
					shortLinkDomain = linksData.short_domain ?? null;
					trackingOn = linksData.tracking_mode === 'tauthub';
					primaryCode = linksData.primary_code ?? null;
					backupCode = linksData.backup_code ?? null;
					linksStatus = filled.primary_url ? 'saved' : 'idle';
				}
			}
			if (draftData) {
				campaignName = draftData.campaign?.name ?? tr('Kampanye');
				campaignStatus = draftData.campaign?.taut_status ?? null;
				wallet = draftData.wallet?.available ?? 0;
				rateIdr = draftData.wallet?.rate_idr ?? 0;
				if (draftData.entry_mode === 'ai' || draftData.entry_mode === 'manual') entryMode = draftData.entry_mode;
				spokepersons = (draftData.campaign?.spokepersons ?? [])
					.filter((s: { name?: string }) => s?.name?.trim())
					.map((s: { name?: string; role?: string; quote?: string }) => ({
						name: String(s.name ?? '').trim(),
						role: String(s.role ?? '').trim(),
						quote: String(s.quote ?? '').trim()
					}));
			}
		} catch {
			error = tr('Gagal memuat data Writer.');
		} finally {
			loading = false;
			// Apa pun hasil GET, form tetap harus bisa diedit dan disimpan.
			hydrated = true;
			void loadArticleInfo();
		}
	}

	// Hitung link & statistik tiap artikel (badge + Cek Validasi). Non-blocking.
	async function loadArticleInfo() {
		for (const item of placement) {
			if (!hasDoc(item)) continue;
			try {
				const res = await apiRequest(`${apiBase}/articles/${item.media_id}`);
				if (!res.ok) continue;
				const d = await res.json();
				const html = d.content_html ?? '';
				const tags = html.match(/<a\b[^>]*>/gi) ?? [];
				const plain = html.replace(/<[^>]+>/g, ' ');
				const words = plain.trim() ? plain.trim().split(/\s+/).filter(Boolean).length : 0;
				const imgs = (html.match(/<img\b[^>]*>/gi) ?? []).filter((t: string) => !/src=["']data:/i.test(t));
				let nofollow = 0;
				let sponsored = 0;
				for (const t of tags) {
					const m = (t as string).match(/\brel\s*=\s*["']([^"']*)["']/i);
					if (!m) continue;
					const tokens = m[1].toLowerCase().split(/\s+/);
					if (tokens.includes('sponsored')) sponsored++;
					else if (tokens.includes('nofollow')) nofollow++;
				}
				linkStats = { ...linkStats, [item.media_id]: { dofollow: tags.length - nofollow - sponsored, nofollow, sponsored } };
				articleStats = { ...articleStats, [item.media_id]: { words, images: imgs.length, links: tags.length, cover: d.featured_image_url ?? null, category: d.category ?? null } };
				if (d.title?.trim()) articleTitles = { ...articleTitles, [item.media_id]: d.title.trim() };
			} catch {
				/* fallback badge */
			}
		}
	}

	// genState hanya menang untuk status SEMENTARA (proses generate). Status
	// 'draft' yang tersimpan di memori tidak boleh menutupi hasil server: setelah
	// QC selesai, server sudah menulis article_status = 'ready', sedangkan
	// genState masih 'draft' dari proses generate — popover prasyarat lalu terus
	// menyuruh "Cek QC" padahal badge artikel sudah hijau. 'draft' lokal hanya
	// dipakai selama server belum sempat melaporkan dokumennya (anti-kedip).
	function articleStatusOf(item: PlacementItem): string {
		const local = genState[item.media_id];
		if (local === 'generating' || local === 'queued' || local === 'error') return local;
		if (local === 'draft' && item.article_status === 'none') return 'draft';
		return item.article_status;
	}
	function hasDoc(item: PlacementItem): boolean {
		const s = articleStatusOf(item);
		return s !== 'none' && s !== 'generating' && s !== 'error' && s !== 'queued';
	}

	// ---------- Kesiapan terbit (popover prasyarat) ----------
	type ReadinessGroup = { key: string; label: string; items: { mediaId: string; name: string }[] };

	const readinessGroups = $derived.by((): ReadinessGroup[] => {
		const groups: ReadinessGroup[] = [];
		const docs = selected.filter((m) => hasDoc(m));
		const toItems = (list: PlacementItem[]) => list.map((m) => ({ mediaId: m.media_id, name: m.name }));
		const add = (key: string, label: string, list: PlacementItem[]) => {
			if (list.length > 0) groups.push({ key, label: tr(label), items: toItems(list) });
		};

		add('article', 'Artikel belum dibuat', selected.filter((m) => !hasDoc(m)));

		// QC dianggap belum dicek bila belum ada audit untuk media itu; gagal bila
		// sudah dipindai tetapi artikel belum berstatus "ready".
		add('qc', 'QC belum dicek', docs.filter((m) => (qcCounts[m.media_id] ?? 0) === 0));
		add('qc-fail', 'QC belum lulus', docs.filter((m) => (qcCounts[m.media_id] ?? 0) > 0 && articleStatusOf(m) !== 'ready'));

		add('cover', 'Cover belum ada', docs.filter((m) => articleStats[m.media_id] && !articleStats[m.media_id].cover));

		add(
			'category',
			'Kategori belum dipilih',
			docs.filter((m) => m.category_required === true && articleStats[m.media_id] && !articleStats[m.media_id].category)
		);
		return groups;
	});

	/** Media yang masih perlu dipindai: belum punya audit atau belum lulus. */
	const qcTargets = $derived(
		selected.filter((m) => hasDoc(m) && ((qcCounts[m.media_id] ?? 0) === 0 || articleStatusOf(m) !== 'ready'))
	);
	const readyToPublish = $derived(distributable === true && readinessGroups.length === 0);

	/** Rincian rel link: "1 Dofollow · 2 Nofollow · 1 Sponsored", atau "0 tautan". */
	function linkStatsText(links: LinkCounts): string {
		const parts = [
			...(links.dofollow > 0 ? [`${links.dofollow} Dofollow`] : []),
			...(links.nofollow > 0 ? [`${links.nofollow} Nofollow`] : []),
			...(links.sponsored > 0 ? [`${links.sponsored} Sponsored`] : [])
		];
		return parts.length > 0 ? parts.join(' · ') : '0 tautan';
	}

	function patchLinks(patch: Partial<LinksForm>) {
		links = { ...links, ...patch };
		if (hydrated) {
			linksStatus = 'dirty';
			linksDirty = true;
		}
	}

	function linksComplete(): boolean {
		return Boolean(links.primary_url.trim() && links.primary_anchor.trim());
	}
	function linksPayload() {
		return {
			tracking_mode: links.tracking_mode,
			primary_url: withScheme(links.primary_url),
			primary_anchor: links.primary_anchor.trim(),
			backup_url: links.backup_url.trim() ? withScheme(links.backup_url) : null,
			backup_anchor: links.backup_anchor.trim() || null
		};
	}
	async function persistLinks() {
		if (viewOnly) return; // kampanye final: tautan tidak boleh diubah
		try {
			const res = await apiRequest(`${apiBase}/writer-links`, { method: 'PUT', body: JSON.stringify(linksPayload()) });
			if (!res.ok) return null;
			return await res.json().catch(() => null);
		} catch {
			return null;
		}
	}
	function applyLinksResult(rd: { primary_code?: string | null; backup_code?: string | null; short_domain?: string | null } | null) {
		if (!rd) return;
		primaryCode = rd.primary_code ?? null;
		backupCode = rd.backup_code ?? null;
		shortLinkDomain = rd.short_domain ?? null;
	}

	function linksStatusLabel(): string {
		if (!linksComplete()) return tr('Belum lengkap');
		switch (linksStatus) {
			case 'saved': return tr('Tersimpan otomatis');
			case 'saving': return 'Menyimpan…';
			case 'error': return tr('Gagal tersimpan');
			default: return tr('Belum tersimpan');
		}
	}

	// Autosave debounced 800ms untuk mode pelacakan + tautan.
	$effect(() => {
		if (!hydrated || linksStatus !== 'dirty') return;
		const payload = links;
		if (saveLinksTimer) window.clearTimeout(saveLinksTimer);
		const seq = ++saveLinksSeq;
		saveLinksTimer = window.setTimeout(() => {
			if (!(payload.primary_url.trim() && payload.primary_anchor.trim())) return;
			linksStatus = 'saving';
			void (async () => {
				const rd = await persistLinks();
				if (seq !== saveLinksSeq) return;
				if (rd) {
					applyLinksResult(rd);
					linksStatus = 'saved';
					linksDirty = false;
				} else {
					linksStatus = 'error';
				}
			})();
		}, 800);
	});

	// Flush saat halaman ditutup/tab disembunyikan (keepalive).
	function flushLinks() {
		if (!hydrated || linksStatus !== 'dirty' || !linksComplete()) return;
		void fetch(`${apiBase}/writer-links`, {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${getAccessToken() ?? ''}` },
			body: JSON.stringify(linksPayload()),
			keepalive: true
		}).catch(() => {});
	}

	async function generateOne(item: PlacementItem, openAfter = true) {
		if (viewOnly) { error = tr(VIEW_ONLY_MSG); return; }
		genState = { ...genState, [item.media_id]: 'generating' };
		error = '';
		try {
			if (linksStatus === 'dirty' && linksComplete()) {
				const rd = await persistLinks();
				if (rd) {
					applyLinksResult(rd);
					linksStatus = 'saved';
					linksDirty = false;
				}
			}
			// Streaming (SSE): editor dibuka segera dan potongan artikel tampil
			// bertahap saat LLM menulis. Hasil akhir tetap diambil dari server
			// (sudah difinalisasi + backlink) lewat openPreview.
			const linkLimit = mediaAllowsLink(item) ? (item.max_links && item.max_links > 0 ? item.max_links : 2) : 0;
			editor = {
				mediaId: item.media_id,
				mediaName: item.name,
				tab: 'tulis',
				title: '',
				html: '',
				cover: null,
				category: null,
				metaDescription: null,
				linkLimit,
				source: 'ai',
				wordCountMode: item.word_count_mode ?? null
			};
			streamHtml = '';
			const token = getAccessToken();
			const controller = new AbortController();
			const abortTimer = window.setTimeout(() => controller.abort(), 300_000);
			const res = await fetch(`${apiBase}/produce/${item.media_id}/stream`, {
				method: 'POST',
				headers: {
					...(token ? { Authorization: `Bearer ${token}` } : {}),
					'Content-Type': 'application/json'
				},
				credentials: 'include',
				signal: controller.signal
			});
			if (!res.ok || !res.body) {
				throw new Error(await readApiError(res, tr('Gagal generate artikel.')));
			}
			const reader = res.body.getReader();
			const dec = new TextDecoder();
			let buf = '';
			let finished = false;
			let streamErr = '';
			while (!finished) {
				const { value, done: fin } = await reader.read();
				if (fin) break;
				buf += dec.decode(value, { stream: true });
				let idx;
				while ((idx = buf.indexOf('\n\n')) >= 0) {
					const block = buf.slice(0, idx);
					buf = buf.slice(idx + 2);
					let ev = 'message';
					let data = '';
					for (const line of block.split('\n')) {
						if (line.startsWith('event:')) ev = line.slice(6).trim();
						else if (line.startsWith('data:')) data += line.slice(5).trim();
					}
					if (!data) continue;
					let payload: { append?: string; message?: string } | null = null;
					try {
						payload = JSON.parse(data);
					} catch {
						continue;
					}
					if (ev === 'delta' && typeof payload?.append === 'string') {
						streamHtml += payload.append;
					} else if (ev === 'done') {
						finished = true;
					} else if (ev === 'error') {
						streamErr = String(payload?.message ?? tr('Gagal generate artikel.'));
						finished = true;
					}
				}
			}
			window.clearTimeout(abortTimer);
			if (streamErr) throw new Error(streamErr);
			if (!finished) throw new Error(tr('Koneksi streaming terputus.'));
			genState = { ...genState, [item.media_id]: 'draft' };
			qcToken += 1;
			// Penulisan/gambar AI di atas mengubah tagihan — segarkan data biaya
			// supaya bar Total Tagihan Akhir berubah begitu generate selesai.
			void load();
			// Modal masih terbuka dan user belum menutupnya sendiri: ganti
			// pratinjau streaming dengan artikel final dari server.
			const wasOpen = editor?.mediaId === item.media_id && streamHtml !== null;
			streamHtml = null;
			if (openAfter && wasOpen) {
				editor = null;
				await openPreview(item);
			}
		} catch (reason) {
			streamHtml = null;
			editor = null;
			genState = { ...genState, [item.media_id]: 'error' };
			error = reason instanceof Error ? reason.message : tr('Gagal generate artikel.');
		}
	}

	async function generateAll() {
		if (viewOnly) { error = tr(VIEW_ONLY_MSG); return; }
		if (bulkRunning) return;
		const targets = selected.filter((m) => !hasDoc(m) || canRegen(m));
		if (targets.length === 0) return;
		if (linksStatus === 'dirty' && linksComplete()) {
			const rd = await persistLinks();
			if (rd) {
				applyLinksResult(rd);
				linksStatus = 'saved';
				linksDirty = false;
			}
		}
		bulkRunning = true;
		error = '';
		bulkProgress = { done: 0, total: targets.length };
		const queued: Record<string, string> = { ...genState };
		for (const m of targets) queued[m.media_id] = 'queued';
		genState = queued;
		let done = 0;
		let stop = false;
		// Paralel terbatas: satu generate = panggilan LLM 20-35 dtk, serial
		// membuat 4 media menunggu hampir 2 menit. Tiga sekaligus memangkas
		// total waktu bulk tanpa membanjiri gateway.
		const CONCURRENCY = 3;
		let next = 0;
		async function bulkWorker() {
			while (!stop) {
				const idx = next++;
				if (idx >= targets.length) return;
				const m = targets[idx];
				const isRegen = hasDoc(m);
				genState = { ...genState, [m.media_id]: 'generating' };
				try {
					// Sama seperti generate satu artikel: ini panggilan LLM.
					const res = await apiRequest(`${apiBase}/produce/${m.media_id}`, { method: 'POST' }, 240_000);
					if (!res.ok) throw new Error(await readApiError(res, isRegen ? 'Gagal generate ulang artikel.': tr('Gagal generate artikel.')));
					genState = { ...genState, [m.media_id]: 'draft' };
				} catch (reason) {
					const msg = reason instanceof Error ? reason.message : tr('Gagal generate artikel.');
					genState = { ...genState, [m.media_id]: 'error' };
					error = `${m.name}: ${msg}`;
					if (/kredit|saldo|402/i.test(msg)) stop = true;
				}
				done += 1;
				bulkProgress = { done, total: targets.length };
			}
		}
		await Promise.all(Array.from({ length: Math.min(CONCURRENCY, targets.length) }, () => bulkWorker()));
		bulkRunning = false;
		bulkProgress = null;
		// Sisa "queued" yang tak sempat diproses kembali ke status aslinya.
		const cleaned: Record<string, string> = {};
		for (const [k, v] of Object.entries(genState)) if (v !== 'queued') cleaned[k] = v;
		genState = cleaned;
		qcToken += 1;
		await load();
	}

	async function openPreview(item: PlacementItem) {
		error = '';
		try {
			const res = await apiRequest(`${apiBase}/articles/${item.media_id}`);
			if (!res.ok) throw new Error(await readApiError(res, 'Artikel belum tersedia.'));
			const d = await res.json();
			const linkLimit = mediaAllowsLink(item) ? (item.max_links && item.max_links > 0 ? item.max_links : 2) : 0;
			editorError = '';
			editor = {
				mediaId: item.media_id,
				mediaName: item.name,
				tab: 'tulis',
				title: d.title ?? '',
				html: d.content_html ?? '',
				cover: d.featured_image_url ?? null,
				category: d.category ?? null,
				metaDescription: d.meta_description ?? null,
				linkLimit,
				source: item.article_source ?? null,
				wordCountMode: item.word_count_mode ?? null
			};
			editorBaseline = { title: editor.title, html: editor.html };
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal membuka artikel.');
		}
	}

	/** Domain dari website_url media (untuk favicon kecil di header modal). */
	const mediaDomainOf = (m: PlacementItem | null | undefined): string =>
		((m?.website_url ?? '').replace(/^https?:\/\/(www\.)?/i, '').replace(/\/+$/, '').trim() || (m?.name ?? '').trim()).toLowerCase();

	function openNaskahUpload(item: PlacementItem) {
		error = '';
		const upLimit = mediaAllowsLink(item) ? (item.max_links && item.max_links > 0 ? item.max_links : 2) : 0;
		editorError = '';
		editor = {
			mediaId: item.media_id,
			mediaName: item.name,
			tab: 'unggah',
			title: '',
			html: '',
			cover: null,
			category: null,
			metaDescription: null,
			linkLimit: upLimit,
			source: 'manual',
			wordCountMode: item.word_count_mode ?? null
		};
		editorBaseline = null;
	}

	/** Tombol "Generate Konten": buka Pengelola Naskah langsung di tab
	 *  "Generate dengan AI", jadi user mengisi topik/cover/kategori dulu sebelum
	 *  The Writer menulis — bukan generate tanpa konteks lalu mengedit belakangan. */
	function openNaskahGenerate(item: PlacementItem) {
		error = '';
		const genLimit = mediaAllowsLink(item) ? (item.max_links && item.max_links > 0 ? item.max_links : 2) : 0;
		editorError = '';
		editor = {
			mediaId: item.media_id,
			mediaName: item.name,
			tab: 'generate',
			title: '',
			html: '',
			cover: null,
			category: null,
			metaDescription: null,
			linkLimit: genLimit,
			source: 'ai',
			wordCountMode: item.word_count_mode ?? null
		};
		editorBaseline = null;
	}

	async function uploadDocNaskah(
		file: File,
		onPhase?: (phase: 'extract' | 'clean') => void,
		links: { anchor_text: string; url: string }[] = []
	): Promise<{ title: string; html: string; warning?: string }> {
		const form = new FormData();
		form.append('file', file);
		onPhase?.('extract');
		const upRes = await apiRequest(`${apiBase}/files`, { method: 'POST', body: form });
		const upData = await upRes.json().catch(() => null);
		const text = upData?.file && typeof upData.file.extracted_text === 'string' ? String(upData.file.extracted_text) : '';
		if (!upRes.ok || !text.trim()) throw new Error(upData?.detail ?? 'File gagal dibaca. Gunakan DOCX, PDF, atau TXT.');
		// Pisahkan judul otomatis: baris pertama pendek (< 120 karakter) dianggap
		// judul; selain itu pakai nama file.
		const lines = text.split('\n').map((l) => l.trim()).filter(Boolean);
		let judul = file.name.replace(/\.[^.]+$/, '');
		let isi = text;
		if (lines.length > 1 && lines[0].length <= 120) {
			// Bersihkan judul dari artefak copy-paste (zero-width, NBSP).
			judul = cleanPastedText(lines[0]).replace(/\n+/g, ' ');
			isi = lines.slice(1).join('\n');
		}
		// DOCX bergambar: server sudah memindahkan gambarnya ke Tauthub dan
		// mengembalikan HTML teks + gambar. Pakai apa adanya (tanpa AI merapikan)
		// supaya dokumen pindah sama persis dan gambarnya tidak hilang.
		const richHtml = upData?.file && typeof upData.file.extracted_html === 'string' ? String(upData.file.extracted_html) : '';
		if (richHtml.trim()) {
			return { title: judul.slice(0, 200), html: richHtml };
		}
		const rawHtml = textToHtml(isi);
		// Rapikan dengan AI sebelum masuk editor. Kegagalan apa pun tidak
		// menghalangi: teks asli tetap dipakai, alasannya dikembalikan sebagai
		// peringatan agar modal menampilkannya.
		onPhase?.('clean');
		try {
			const cleanRes = await apiRequest(
				`${apiBase}/documents/cleanup`,
				{ method: 'POST', body: JSON.stringify({ text: isi, title: judul, links }) },
				180_000
			);
			if (!cleanRes.ok) {
				throw new Error(await readApiError(cleanRes, tr('AI gagal merapikan dokumen.')));
			}
			const cleanData = await cleanRes.json().catch(() => null);
			const cleaned = cleanData?.html ? String(cleanData.html) : '';
			if (cleanData?.cleaned === false || !cleaned.trim()) {
				const reason = String(cleanData?.reason ?? '').trim() || tr('AI mengembalikan hasil kosong.');
				return { title: judul.slice(0, 200), html: rawHtml, warning: `${tr('AI gagal merapikan dokumen:')} ${reason} ${tr('Teks asli tetap dipakai.')}` };
			}
			return { title: judul.slice(0, 200), html: cleaned };
		} catch (reason) {
			const msg = reason instanceof Error ? reason.message : tr('AI gagal merapikan dokumen.');
			return { title: judul.slice(0, 200), html: rawHtml, warning: `${tr('AI gagal merapikan dokumen:')} ${msg} ${tr('Teks asli tetap dipakai.')}` };
		}
	}

	// Generate dengan arahan topik dari modal Pengelola Naskah. Penulisan
	// ditagih 1× tarif per generate: artikel baru maupun setiap regenerasi
	// (tarif & penagihan ada di backend).
	/** Isian mode "Yang Proper" dari modal Pengelola Naskah — strict grounding. */
	type ProperGenPayload = { fakta: string; dateline: string; narasumber: string; kutipan: string; quotes: Array<{ dateline: string; narasumber: string; kutipan: string }>; fokus: string; brief_id: string };

	async function generateNaskah(
		mediaId: string,
		topic: string,
		links: { anchor_text: string; url: string }[] = [],
		angleSlug = '',
		proper: ProperGenPayload | null = null
	): Promise<{ title: string; html: string; category?: string; imagesGenerated?: number; imagesCostCredits?: number }> {
		const mediaType = placement.find((p) => p.media_id === mediaId)?.media_type;
		const angle = angleChannels.find((ch) => ch.slug === mediaType)?.angles.find((a) => a.slug === angleSlug && a.has_prompt);
		if (anglesLoading || anglesError || !angle) throw new Error(tr('Pilih angle artikel yang tersedia untuk media ini.'));
		// Mode Proper membawa isian fakta terkunci + Brief Copilot: backend
		// memakainya untuk prompt strict grounding & verifikasi Gatekeeper.
		const payload = proper
			? { topic, links, angle_slug: angleSlug, gaya: 'proper', proper }
			: { topic, links, angle_slug: angleSlug };
		const res = await apiRequest(`${apiBase}/produce/${mediaId}`, {
			method: 'POST',
			body: JSON.stringify(payload)
		}, 240_000);
		const data = await res.json().catch(() => null);
		if (!res.ok) throw new Error(data?.detail ?? tr('Gagal membuat draf AI.'));
		placement = placement.map((p) => p.media_id === mediaId ? { ...p, angle_slug: angleSlug } : p);
		genState = { ...genState, [mediaId]: 'draft' };
		qcToken += 1;
		// Setiap generate menambah tagihan (regenerasi ditagih lagi) — segarkan
		// data kampanye supaya regen_count, gambar, dan Total Tagihan Akhir di
		// bar bawah ikut naik tanpa reload halaman.
		await load();
		return {
			title: String(data?.title ?? ''),
			html: String(data?.content_html ?? ''),
			// Rubrik pilihan AI (server memvalidasinya ke kategori media; kosong
			// hanya bila media memang tidak punya daftar kategori).
			category: String(data?.category ?? ''),
			imagesGenerated: Number(data?.images_generated ?? 0) || 0,
			imagesCostCredits: Number(data?.images_cost_credits ?? 0) || 0
		};
	}

	// Simpan isi editor: PUT bila dokumen sudah ada, POST upload bila belum.
	// Pemeriksaan QC tidak lagi dijalankan dari sini — pemindaian tetap tersedia
	// lewat tombol "Analisis QC" per media di daftar.
	async function persistEditor(title: string, html: string, coverUrl: string | null, category: string, metaDescription: string): Promise<boolean> {
		if (viewOnly) { error = tr(VIEW_ONLY_MSG); return false; }
		const current = editor;
		if (!current) return false;
		const mediaId = current.mediaId;
		editorError = '';
		try {
			// Ada dokumen → PUT; belum ada (hasil upload) → POST upload.
			const checkRes = await apiRequest(`${apiBase}/articles/${mediaId}`);
			if (checkRes.ok) {
				const putRes = await apiRequest(`${apiBase}/articles/${mediaId}`, {
					method: 'PUT',
					body: JSON.stringify({ title, content_html: html, featured_image_url: coverUrl ?? undefined, category, meta_description: metaDescription })
				});
				if (!putRes.ok) throw new Error(await readApiError(putRes, tr('Gagal menyimpan artikel.')));
			} else {
				const postRes = await apiRequest(`${apiBase}/articles/${mediaId}/upload`, {
					method: 'POST',
					body: JSON.stringify({ title, content_html: html, featured_image_url: coverUrl ?? undefined, category, meta_description: metaDescription })
				});
				if (!postRes.ok) throw new Error(await readApiError(postRes, tr('Gagal menyimpan artikel.')));
				genState = { ...genState, [mediaId]: 'draft' };
			}
			qcToken += 1;
			// Jangan tunggu reload: kalau load lambat/tertahan, modal tetap
			// terlihat "menyimpan" dan spinner QC tidak pernah sempat muncul.
			void load();
			return true;
		} catch (reason) {
			editorError = reason instanceof Error ? reason.message : tr('Gagal menyimpan artikel.');
			return false;
		}
	}

	// QC otomatis pasca simpan: pemindaian gatekeeper untuk artikel yang baru
	// disimpan, level-artikel (sama dengan tombol pemindaian ulang di panel QC).
	// Setiap selesai, qcToken dinaikkan supaya panel & badge muat data baru.
	//
	// qcQueued: simpan yang datang saat QC media itu masih berjalan. Setelah
	// pemindaian selesai, satu putaran ulang dijalankan supaya hasil QC selalu
	// mencerminkan versi artikel terakhir — dulu simpan seperti itu diabaikan
	// diam-diam oleh guard.
	let qcQueued = $state<Record<string, boolean>>({});

	async function runQcAfterSave(mediaId: string, alreadyBusy = false) {
		if (!alreadyBusy) {
			if (qcRunning[mediaId]) {
				qcQueued = { ...qcQueued, [mediaId]: true };
				return;
			}
			qcRunning = { ...qcRunning, [mediaId]: true };
		}
		try {
			const res = await apiRequest(`${apiBase}/gatekeeper/audit-all`, {
				method: 'POST',
				body: JSON.stringify({ mediaIds: [mediaId] })
			}, 900_000);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menjalankan pemindaian QC.')));
		} catch {
			// Senyap: QC manual tetap tersedia lewat tombol/panel bila otomatis gagal.
		} finally {
			// Spinner dimatikan SEGERA begitu request selesai. Dulu status baru
			// dibersihkan setelah `await load()` — dengan banyak media, load bisa
			// lambat/tertahan sehingga spinner terlihat tidak pernah hilang dan
			// simpan berikutnya ikut terblokir guard.
			qcRunning = { ...qcRunning, [mediaId]: false };
			qcToken += 1;
			void load();
			if (qcQueued[mediaId]) {
				const next = { ...qcQueued };
				delete next[mediaId];
				qcQueued = next;
				void runQcAfterSave(mediaId);
			}
		}
	}

	async function saveEditor(title: string, html: string, coverUrl: string | null, category: string, metaDescription: string) {
		if (viewOnly) { error = tr(VIEW_ONLY_MSG); return; }
		if (!editor) return;
		const mediaId = editor.mediaId;
		// QC otomatis hanya saat ada PERUBAHAN vs backend: judul/isi sama
		// persis (mis. klien hanya ganti cover/meta tanpa menulis ulang)
		// → pemindaian dipindai hasil lama cukup, jangan bakar token AI.
		const kontenBerubah = !editorBaseline
			|| editorBaseline.title !== (title ?? '')
			|| editorBaseline.html !== (html ?? '');
		// Nyalakan spinner sejak Simpan ditekan, bukan setelah artikel tersimpan.
		// QC baru bisa mulai setelah PUT/POST selesai; tanpa ini ada jeda kosong
		// sehingga saat menyimpan satu per satu spinner terlihat hilang/berkedip.
		const spinnerStartedHere = kontenBerubah && qcRunning[mediaId] !== true;
		if (spinnerStartedHere) qcRunning = { ...qcRunning, [mediaId]: true };
		editorSaving = true;
		const saved = await persistEditor(title, html, coverUrl, category, metaDescription);
		editorSaving = false;
		if (saved) {
			editor = null;
			if (kontenBerubah) void runQcAfterSave(mediaId, spinnerStartedHere);
		} else if (spinnerStartedHere) {
			// Penyimpanan gagal → tidak ada QC yang berjalan; jangan tinggalkan spinner.
			qcRunning = { ...qcRunning, [mediaId]: false };
		}
	}

	/** Buka panel QC untuk media terpilih pertama (pengganti halaman Gatekeeper). */
	function openQcPanel() {
		const target = selected[0]?.media_id ?? placement[0]?.media_id ?? null;
		if (target) qcFor = target;
	}

	async function uploadCover(file: File): Promise<string> {
		if (viewOnly) { error = tr(VIEW_ONLY_MSG); return ''; }
		const form = new FormData();
		form.append('file', file);
		form.append('for', 'cover');
		const res = await apiRequest(`/v1/brands/${brandId}/assets`, { method: 'POST', body: form });
		const data = await res.json().catch(() => null);
		if (!res.ok || !data?.url) throw new Error(data?.detail ?? tr('Gagal mengunggah cover.'));
		return data.url;
	}

	/* ── Biaya (murni kredit, tanpa rupiah di perhitungan) ── */
	const regenCountOf = (m: PlacementItem) => m.regen_count ?? 0;
	const canRegen = (m: PlacementItem) => hasDoc(m) && regenCountOf(m) < regenMax;

	const cost = $derived.by(() => {
		// Setiap generate ditagih 1× tarif sekarang (artikel baru maupun
		// regenerasi), dan yang dihitung HANYA artikel/gambar yang benar-benar
		// dibuat AI di kampanye ini (jumlah dari ledger server) — klien yang
		// menulis atau mengunggah artikelnya sendiri tidak melihat biaya AI.
		// Biaya media memakai harga penempatan (dikonversi server), bukan
		// credit_cost yang bukan sumber penagihan — dulu itu sebab bar menulis
		// Rp 22.000 padahal yang ditagih Rp 20.000. Ini biaya saat terbit.
		const writingUnits = writerChargeCount + writerRegenCount;
		const writing = writingUnits * writerCostCredits;
		const images = imageChargeCount * imageCreditCost;
		const media = selected.reduce((s, m) => s + (m.cost_tc ?? 0), 0);
		return {
			count: selected.length,
			writing,
			writingUnits,
			media,
			images,
			imageCount: imageChargeCount,
			total: writing + media + images
		};
	});
	const pendingCount = $derived(selected.filter((m) => !hasDoc(m) || canRegen(m)).length);
	// Biaya ditampilkan dalam rupiah memakai kurs aktif (fallback Rp 1.000).
	const toRp = (credits: number) => tcToRp(credits, rateIdr);

	// Rincian biaya penulisan dalam bahasa manusia, tampil di bar bawah supaya
	// angka tidak muncul tanpa penjelasan. Angka tooltip = biaya tombol
	// "Generate Semua Artikel", yaitu satu penulisan untuk tiap media yang
	// belum punya artikel atau masih boleh diregenerasi.
	const costDetail = $derived.by(() => {
		if (pendingCount === 0) return tr('Semua media sudah punya artikel — tidak ada biaya penulisan tertunda.');
		return `${pendingCount} ${tr('artikel')} × ${toRp(writerCostCredits)} = ${toRp(pendingCount * writerCostCredits)}`;
	});


	// Tooltip rincian biaya tombol "Generate Semua Artikel" (parity Tooltip app lama).
	const generateTooltip = $derived.by(() => {
		if (pendingCount === 0) return tr('Semua media sudah memiliki artikel — generate tidak diperlukan.');
		return `${tr('Rincian:')} ${costDetail}`;
	});

	// Umpan balik perubahan tagihan: begitu total berubah, chip delta mengapung
	// di atas angkanya lalu naik dan memudar. Hijau = naik, merah = turun.
	// Selama data awal masih dimuat, total pindah 0 → nilai penuh tanpa aksi
	// apa pun dan TIDAK boleh dianimasikan: karena itu baseline diambil dari
	// snapshot pertama setelah data siap (hydrated), baru selisih dihitung.
	// prevTotalShown sengaja non-reaktif: hanya buku besar pembanding, bukan
	// tampilan, agar tidak memicu ulang efeknya sendiri.
	let billDelta = $state<{ id: number; delta: number } | null>(null);
	let billDeltaSeq = 0;
	let deltaBaselineSeen = false;
	let prevTotalShown: number | null = null;
	$effect(() => {
		const now = cost.total;
		if (!hydrated || !Number.isFinite(now)) return;
		const prev = prevTotalShown;
		if (!deltaBaselineSeen) {
			deltaBaselineSeen = true;
			prevTotalShown = now;
			return;
		}
		if (prev === null || prev === now) {
			prevTotalShown = now;
			return;
		}
		prevTotalShown = now;
		billDeltaSeq += 1;
		billDelta = { id: billDeltaSeq, delta: now - prev };
	});

	const editorMedia = $derived.by(() => {
		const current = editor;
		if (!current) return null;
		return selected.find((m) => m.media_id === current.mediaId) ?? null;
	});

	async function publish() {
		if (viewOnly) { error = tr(VIEW_ONLY_MSG); return; }
		if (coverChecking || distributing) return;
		// Prasyarat belum beres → tampilkan popover penjelas.
		if (!readyToPublish) {
			readinessOpen = true;
			void loadArticleInfo();
			return;
		}
		readinessOpen = false;
		await openPublishPanel();
	}

	async function openPublishPanel() {
		coverChecking = true;
		error = '';
		publishError = '';
		try {
			// Cover diperiksa SEBELUM panel dibuka, supaya panel bisa langsung
			// menampilkan artikel mana yang belum punya cover.
			missingCovers = await articlesWithoutCover();
			// Media tanpa waktu terbit dianggap memakai preset "Sekarang".
			quickPick = Object.fromEntries(selected.filter((m) => !scheduleByMedia[m.media_id]).map((m) => [m.media_id, 'now']));
			publishOpen = true;
		} finally {
			coverChecking = false;
		}
	}

	/** Cek QC sekaligus dari popover kesiapan terbit. `ids` diisi bila dipanggil
	 *  dari tombol grup (hanya media di daftar grup itu); tanpa `ids` dipakai
	 *  seluruh target QC kampanye. */
	async function runQcAll(ids?: string[]) {
		if (bulkQcRunning || viewOnly) return;
		const fallback = qcTargets.length > 0 ? qcTargets : selected.filter((m) => hasDoc(m));
		const targets = ids && ids.length > 0 ? ids : fallback.map((m) => m.media_id);
		if (targets.length === 0) return;
		bulkQcRunning = true;
		error = '';
		const running: Record<string, boolean> = { ...qcRunning };
		for (const id of targets) running[id] = true;
		qcRunning = running;
		try {
			// Satu panggilan untuk semua media: server memindai paralel dan baru
			// membalas setelah selesai, jadi progress di UI bersifat animasi.
			const res = await apiRequest(
				`${apiBase}/gatekeeper/audit-all`,
				{ method: 'POST', body: JSON.stringify({ mediaIds: targets }) },
				900_000
			);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menjalankan pemindaian QC.')));
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal menjalankan pemindaian QC.');
		} finally {
			const next = { ...qcRunning };
			for (const id of targets) delete next[id];
			qcRunning = next;
			bulkQcRunning = false;
			qcToken += 1;
			await load();
			// Semua prasyarat beres → popover ditutup dan lanjut ke Ringkasan.
			if (readyToPublish) {
				readinessOpen = false;
				await publish();
			}
		}
	}

	/** Artikel terpilih yang sudah punya dokumen tetapi belum punya cover. */
	async function articlesWithoutCover(): Promise<{ mediaId: string; name: string }[]> {
		const missing: { mediaId: string; name: string }[] = [];
		for (const m of selected) {
			if (!hasDoc(m)) continue;
			try {
				const res = await apiRequest(`${apiBase}/articles/${m.media_id}`);
				if (!res.ok) continue;
				const d = await res.json();
				if (!d.featured_image_url?.trim()) missing.push({ mediaId: m.media_id, name: m.name });
			} catch {
				/* tak diketahui — tidak memblokir */
			}
		}
		return missing;
	}

	/** Pakai thumbnail default Tauthub untuk artikel yang belum punya cover. */
	async function useDefaultCovers(list: { mediaId: string; name: string }[] = missingCovers) {
		if (list.length === 0 || coverBusy) return;
		coverBusy = true;
		publishError = '';
		try {
			for (const item of list) {
				const res = await apiRequest(`${apiBase}/articles/${item.mediaId}`, {
					method: 'PUT',
					body: JSON.stringify({ featured_image_url: ARTICLE_DEFAULT_COVER })
				});
				if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan cover default.')));
			}
			if (list === missingCovers) missingCovers = [];
			await loadArticleInfo();
		} catch (reason) {
			publishError = reason instanceof Error ? reason.message : tr('Gagal menyimpan cover default.');
		} finally {
			coverBusy = false;
		}
	}

	/** Setel jadwal satu media. 'now'/'' = terbit langsung; '1h'|'24h'|'3d' = preset. */
	function setMediaSchedule(mediaId: string, value: string) {
		const next = { ...scheduleByMedia };
		const picks = { ...quickPick };
		if (value === '' || value === 'now') {
			delete next[mediaId];
			picks[mediaId] = 'now';
		} else if (value === '1h' || value === '24h' || value === '3d') {
			const ms = value === '1h' ? 3_600_000 : value === '24h' ? 86_400_000 : 3 * 86_400_000;
			next[mediaId] = toLocalInput(new Date(Date.now() + ms).toISOString());
			picks[mediaId] = value;
		} else {
			// Waktu yang diketik manual tidak lagi mewakili preset mana pun.
			next[mediaId] = value;
			delete picks[mediaId];
		}
		scheduleByMedia = next;
		quickPick = picks;
	}

	/** Sorotan untuk tombol preset yang sedang berlaku. */
	const quickBtnStyle = (isActive: boolean) =>
		isActive ? 'background-color: var(--primary); color: var(--primary-foreground);' : '';

	// Pemilih tanggal untuk opsi "terbit belakangan".
	let schedulePickerFor = $state<PlacementItem | null>(null);
	let schedulePickerInitial = $state<Date | null>(null);

	function openSchedulePicker(m: PlacementItem) {
		schedulePickerFor = m;
		const at = scheduleByMedia[m.media_id];
		schedulePickerInitial = at ? new Date(at) : null;
	}

	function applySchedulePicker(date: Date) {
		const target = schedulePickerFor;
		if (!target) return;
		schedulePickerFor = null;
		// Waktu yang sudah lewat/±1 menit diperlakukan sebagai "Sekarang".
		if (date.getTime() <= Date.now() + 60_000) {
			setMediaSchedule(target.media_id, 'now');
			return;
		}
		scheduleByMedia = { ...scheduleByMedia, [target.media_id]: date.toISOString() };
		// Waktu manual tidak mewakili preset mana pun.
		const picks = { ...quickPick };
		delete picks[target.media_id];
		quickPick = picks;
	}

	// Jumlah media yang dijadwalkan, untuk label tombol dan ringkasan panel.
	const scheduledCount = $derived(selected.filter((m) => Boolean(scheduleByMedia[m.media_id])).length);

	/**
	 * Payload jadwal: hanya media yang punya waktu. Backend memperlakukan media
	 * yang tidak disebut sebagai "terbit sekarang" (`timelineFromBody`), jadi
	 * daftar parsial memang berarti campuran langsung + terjadwal.
	 */
	function schedulePayload(): { media_id: string; scheduled_at: string }[] {
		return Object.entries(scheduleByMedia)
			.filter(([mediaId, at]) => Boolean(at) && selected.some((m) => m.media_id === mediaId))
			.map(([media_id, at]) => ({ media_id, scheduled_at: new Date(at).toISOString() }));
	}

	/** Kirim terbit dari panel, lalu buka Ringkasan. Cover wajib sudah beres. */
	async function confirmPublish() {
		if (viewOnly) { error = tr(VIEW_ONLY_MSG); return; }
		if (distributing) return;
		if (missingCovers.length > 0) {
			publishError = tr('Selesaikan cover dulu: tambahkan lewat Edit Artikel, atau pakai thumbnail default Tauthub.');
			return;
		}
		if (schedulePayload().some((s) => new Date(s.scheduled_at).getTime() <= Date.now())) {
			publishError = tr('Ada jadwal yang sudah lewat — pilih waktu di masa depan.');
			return;
		}
		publishOpen = false;
		await distributeNow(schedulePayload());
	}

	async function distributeNow(schedule: { media_id: string; scheduled_at: string }[] = []) {
		if (viewOnly) { error = tr(VIEW_ONLY_MSG); return; }
		distributing = true;
		try {
			// schedule kosong = terbit serentak sekarang; berisi jadwal = terbit terjadwal.
			const res = await apiRequest(`${apiBase}/gatekeeper/distribute`, { method: 'POST', body: JSON.stringify({ schedule }) }, 120_000);
			const d = await res.json().catch(() => ({}));
			if (res.status === 402 && d.code === 'insufficient_credits') {
				topup = { needed: Number(d.needed ?? 0), available: Number(d.available ?? 0) };
				return;
			}
			if (!res.ok) throw new Error(d.detail ?? tr('Gagal menerbitkan kampanye.'));
			await goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/ringkasan`);
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal menerbitkan kampanye.');
			distributeFailed = true;
		} finally {
			distributing = false;
		}
	}

	const showLinksCard = $derived(backlinkEnabled || Boolean(links.primary_url.trim()));
	const finalUrlPreview = $derived(links.primary_url.trim() ? buildTrackedUrl(withScheme(links.primary_url), { mode: links.tracking_mode, medium: 'taut', campaign: campaignId.slice(0, 8) }) : '');
	const backupUrlPreview = $derived(links.backup_url.trim() ? buildTrackedUrl(withScheme(links.backup_url), { mode: links.tracking_mode, medium: 'taut', campaign: campaignId.slice(0, 8) }) : '');
	const shortBase = $derived(typeof window !== 'undefined' ? (shortLinkDomain ? `https://${shortLinkDomain}` : window.location.origin) : '');
	const primaryShortLink = $derived(links.tracking_mode === 'tauthub' && primaryCode ? `${shortBase}/t/${primaryCode}` : '');
	const backupShortLink = $derived(links.tracking_mode === 'tauthub' && backupCode ? `${shortBase}/t/${backupCode}` : '');

	const checklistItem = $derived(checklistFor ? (placement.find((p) => p.media_id === checklistFor) ?? null) : null);
	const checkStats = $derived(checklistItem ? (articleStats[checklistItem.media_id] ?? null) : null);
	const checkLinks = $derived(checklistItem ? (linkStats[checklistItem.media_id] ?? null) : null);
	const checkMediaType = $derived(checklistItem ? ((checklistItem as unknown as { media_type?: string }).media_type ?? null) : null);

	// Baris indikator untuk popup Cek Validasi: ikon + nilai + warna chip.
	// pos = hijau (bagus), warn = merah (perlu perhatian), num = netral.
	type CheckRow = {
		Icon: typeof FileText;
		label: string;
		sub: string;
		value: string;
		tone: 'pos' | 'warn' | 'num';
	};
	const checkRows = $derived.by<CheckRow[]>(() => {
		if (!checklistItem) return [];
		const st = articleStatusOf(checklistItem);
		return [
			{
				Icon: FileText,
				label: tr('Status artikel'),
				sub: tr('Status terkini artikel di sistem.'),
				value: st === 'draft' ? tr('Tersedia') : statusText(st),
				tone: st === 'ready' ? 'pos' : st === 'blocked' ? 'warn' : 'num'
			},
			{
				Icon: Type,
				label: tr('Jumlah kata'),
				sub: tr('Total kata dalam artikel.'),
				value: checkStats ? `${checkStats.words.toLocaleString('id-ID')} ${tr('kata')}` : tr('Tidak ada'),
				tone: 'num'
			},
			{
				Icon: Image,
				label: tr('Cover artikel'),
				sub: tr('Ketersediaan gambar cover.'),
				value: checkStats ? (checkStats.cover ? tr('Ada') : tr('Tidak ada')) : tr('Tidak ada'),
				tone: checkStats?.cover ? 'pos' : 'num'
			},
			{
				Icon: Images,
				label: tr('Gambar di artikel'),
				sub: tr('Jumlah gambar yang digunakan.'),
				value: checkStats ? `${checkStats.images} ${tr('gambar')}` : tr('Tidak ada'),
				tone: 'num'
			},
			{
				Icon: Link2,
				label: tr('Tautan aktif'),
				sub: tr('Jumlah tautan yang berfungsi.'),
				value: checkLinks ? linkStatsText(checkLinks) : checkStats ? `${checkStats.links} ${tr('tautan')}` : tr('Tidak ada'),
				tone: 'num'
			},
			{
				Icon: Target,
				label: tr('Kesesuaian media tujuan'),
				sub: tr('Kecocokan konten dengan karakter media.'),
				value: checkMediaType
					? checkMediaType === 'mainstream'
						? 'Mainstream'
						: checkMediaType === 'blogger'
							? 'Blogger'
							: checkMediaType
					: tr('Tidak ada indikator'),
				tone: checkMediaType ? 'pos' : 'num'
			}
		];
	});

	const qcItem = $derived(qcFor ? (placement.find((p) => p.media_id === qcFor) ?? null) : null);
	const qcStats = $derived(qcItem ? (articleStats[qcItem.media_id] ?? null) : null);

	const statusTone = (s: string) =>
		s === 'generating' || s === 'queued'
			? 'warning'
			: s === 'draft'
				? 'accent'
				: s === 'ready'
					? 'success'
					: s === 'blocked'
						? 'destructive'
						: 'muted';
	const statusText = (s: string) =>
		s === 'generating'
			? tr('Generasi AI…')
			: s === 'queued'
				? tr('Dalam Antrean…')
				: s === 'draft'
					? tr('Draft Tersedia')
						: s === 'ready'
							? tr('Siap Terbit')
								: s === 'blocked'
									? tr('Diblokir')
									: tr('Menunggu Artikel');
	const badge = (tone: string) =>
		tone === 'success'
			? 'background-color: var(--success-soft); color: var(--success);'
			: tone === 'warning'
				? 'background-color: var(--warning-soft); color: var(--warning);'
				: tone === 'destructive'
						? 'background-color: color-mix(in srgb, var(--destructive) 10%, var(--card)); color: var(--destructive);'
					: tone === 'accent'
						? 'background-color: var(--accent); color: var(--accent-foreground);'
						: 'background-color: var(--muted); color: var(--muted-foreground);';
</script>

	<PageTitle title="Taut Pintar — Penyedia Konten" />

	<main class="w-full pb-32">
		{#if viewOnly}
			<div class="mb-4 flex flex-wrap items-center gap-2 rounded-[5px] border border-slate-200 bg-slate-100/70 px-4 py-2.5 text-xs text-slate-700">
				<span>{tr('Kampanye ini sudah selesai — mode lihat saja. Isinya tidak bisa diubah lagi.')}</span>
				<a class="font-semibold text-blue-600 underline" href={`/klien/${brandUrl}/taut-pintar/${campaignId}/ringkasan`}>{tr('Buka Ringkasan')}</a>
			</div>
		{/if}

		<!-- Header Halaman dihapus: informasi judul sudah diwakili banner produksi. -->

		<div class="mt-3">
			<AgentSteps current={2} />
		</div>

	{#if spokepersons.length > 0}
		<div class="mt-3 rounded-[5px] border p-3" style="background-color: var(--muted);">
			<p class="text-xs font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr("Kutipan Narasumber (dari The Profiler)")}</p>
			<div class="mt-1 space-y-2">
				{#each spokepersons as s, i (s.name + '-' + i)}
					<div>
						<p class="text-sm"><span class="font-bold">{s.name}</span>{#if s.role}<span class="text-muted-foreground">, {s.role}</span>{/if}</p>
						{#if s.quote}<p class="text-sm text-muted-foreground">{tr('Kutipan:')} “{s.quote}”</p>{/if}
					</div>
				{/each}
			</div>
			<p class="mt-2 text-xs text-muted-foreground">{tr("Kutipan ini otomatis dipakai AI saat membuat artikel untuk semua media terpilih (minimal satu kutipan, dengan atribusi nama dan jabatan).")}</p>
		</div>
	{/if}

	{#if agentStatus && !agentStatus.configured}
		<div class="app-card mt-3 flex items-start gap-2 px-4 py-2.5 text-sm" style="background-color: color-mix(in srgb, var(--destructive) 7%, var(--card)); color: var(--destructive);" role="alert">
			<TriangleAlert size={16} class="mt-0.5 shrink-0" />
			<span>{REASON_LABEL[agentStatus.reason ?? ''] ?? tr('AI Writer belum dikonfigurasi oleh admin.')} {tr("Generate artikel tidak akan berjalan sampai admin mengaktifkannya.")}</span>
		</div>
	{/if}

	{#if error}
		<div class="app-card mt-3 flex items-start gap-2 px-4 py-2.5 text-sm" style="background-color: color-mix(in srgb, var(--destructive) 7%, var(--card)); color: var(--destructive);">
			<span class="flex-1">{error}</span>
			<button class="rounded p-0.5 hover:bg-secondary" type="button" aria-label={tr("Tutup error")} onclick={() => (error = '')}><X size={12} /></button>
		</div>
	{/if}

	{#if topup}
		<div class="app-card mt-3 px-4 py-3 text-sm" style="background-color: var(--warning-soft); color: var(--warning);" role="alert">
			<p class="font-bold">{tr('Saldo Tidak Cukup')}</p>
			<p class="mt-1">Distribusi butuh <strong>{toRp(topup.needed)}</strong>, saldo tersedia <strong>{toRp(topup.available)}</strong>. Silakan top up terlebih dahulu.</p>
			<div class="mt-2 flex flex-wrap gap-2">
				<a class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" href={`/klien/${brandUrl}/credits`}>{tr('Top Up Saldo')}</a>
				<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={() => (topup = null)}>{tr('Tutup')}</button>
			</div>
		</div>
	{/if}

	{#if distributeFailed}
		<div class="app-card mt-3 flex flex-wrap items-center gap-3 px-4 py-3 text-sm">
			<span class="flex-1 text-muted-foreground">{tr('Penerbitan gagal. Periksa panel Analisis QC lalu coba lagi.')}</span>
			<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={openQcPanel}><ShieldCheck size={14} /> {tr('Buka Panel QC')}</button>
		</div>
	{/if}

	{#if loading}
		<div class="mt-6 flex items-center gap-2 text-sm text-muted-foreground"><LoaderCircle size={18} class="animate-spin" />{tr("Memuat The Writer…")}</div>
	{:else}
		{#if showLinksCard}
			<section class="app-card mt-3 p-4" aria-label={tr("Mode pelacakan dan anchor text")}>
				<div class="flex flex-wrap items-center justify-between gap-2">
					<div class="flex items-center gap-2">
						<span class="flex h-7 w-7 items-center justify-center rounded-[5px]" style="background-color: var(--accent); color: var(--accent-foreground);"><Link size={18} /></span>
						<p class="font-bold">{tr("Mode Pelacakan & Anchor Text")}</p>
					</div>
					<p class="text-xs" style="color: {linksStatus === 'error' ? 'var(--destructive)' : linksStatus === 'saving' ? 'var(--primary)' : linksStatus === 'dirty' ? 'var(--warning)' : 'var(--muted-foreground)'};">{linksStatusLabel()}</p>
				</div>
				<p class="mt-1 text-sm text-muted-foreground">{tr("Pilih bagaimana Tauthub menyuntikkan tautan, lalu atur peluru URL dan kata kuncinya.")}</p>

				<div class="mt-3 flex items-center gap-2">
					<input
						type="checkbox"
						aria-label={tr('Tauthub Tracking Link')}
						checked={trackingOn}
						disabled={linksStatus === 'saving'}
						onchange={(e) => { trackingOn = (e.currentTarget as HTMLInputElement).checked; patchLinks({ tracking_mode: trackingOn ? 'tauthub' : 'direct' }); }}
					/>
					<span class="text-xs font-medium">{tr('Tauthub Tracking Link')}</span>
					<span class="text-xs text-muted-foreground">{tr('(short link + statistik klik — mati = Pure SEO)')}</span>
				</div>

				<div class="mt-3 grid gap-3 sm:grid-cols-2" role="radiogroup" aria-label={tr('Mode pelacakan')}>
					<button
						type="button"
						class="rounded-[5px] border p-3 text-left"
						style="border-color: {links.tracking_mode === 'direct' ? 'var(--primary)' : 'var(--border)'}; background-color: {links.tracking_mode === 'direct' ? 'var(--accent)' : 'transparent'};"
						role="radio" aria-checked={links.tracking_mode === 'direct'}
						onclick={() => { trackingOn = false; patchLinks({ tracking_mode: 'direct' }); }}
					>
						<div class="flex items-center justify-between">
							<span class="inline-flex items-center gap-2 font-medium"><Link2 size={16} /> {tr('Direct URL (Pure SEO)')}</span>
							{#if links.tracking_mode === 'direct'}<CircleCheck size={18} style="color: var(--primary);" />{/if}
						</div>
						<p class="mt-2 text-xs text-muted-foreground">{tr("URL ditanam mentah tanpa redirect untuk 100% aliran Link Juice — default.")}</p>
					</button>
					{#if trackingOn}
						<button
							type="button"
							class="rounded-[5px] border p-3 text-left"
							style="border-color: {links.tracking_mode === 'tauthub' ? 'var(--primary)' : 'var(--border)'}; background-color: {links.tracking_mode === 'tauthub' ? 'var(--accent)' : 'transparent'};"
							role="radio" aria-checked={links.tracking_mode === 'tauthub'}
							onclick={() => patchLinks({ tracking_mode: 'tauthub' })}
						>
							<div class="flex items-center justify-between">
								<span class="inline-flex items-center gap-2 font-medium"><Link size={16} /> {tr('Tauthub Tracking Link')}</span>
								{#if links.tracking_mode === 'tauthub'}<CircleCheck size={18} style="color: var(--primary);" />{/if}
							</div>
							<p class="mt-2 text-xs text-muted-foreground">{tr('Menggunakan format parameter')} <code>?utm_source=tauthub</code> {tr('untuk merekam data CTR dan Analytics. Cocok untuk kampanye PR & Promosi.')}</p>
						</button>
					{/if}
				</div>

				<div class="mt-4 space-y-4">
					<div class="rounded-[5px] border p-3" style="border-color: var(--primary);">
						<p class="flex items-center gap-2 text-sm font-bold" style="color: var(--primary);"><Link2 size={16} /> {tr('Tautan Utama (Prioritas 1)')}</p>
						<div class="mt-3 grid gap-3 sm:grid-cols-2">
							<label class="block">
								<span class="text-xs font-medium">{tr('Target URL')} <span style="color: var(--destructive);">*</span></span>
								<div class="app-input mt-1 flex items-center gap-1">
									<span class="text-xs text-muted-foreground">https://</span>
									<input class="w-full bg-transparent text-sm outline-none" placeholder="tauthub.com/promo" value={links.primary_url} oninput={(e) => patchLinks({ primary_url: stripScheme((e.currentTarget as HTMLInputElement).value) })} />
								</div>
							</label>
							<label class="block">
								<span class="text-xs font-medium">{tr('Anchor Text Induk')} <span style="color: var(--destructive);">*</span></span>
								<input class="app-input mt-1 w-full" maxlength="120" placeholder="TautHub PR Platform" value={links.primary_anchor} oninput={(e) => patchLinks({ primary_anchor: (e.currentTarget as HTMLInputElement).value })} />
							</label>
						</div>
						{#if primaryShortLink}
							<p class="mt-2 break-all text-xs" style="color: var(--primary);">{tr('Preview tautan utama:')} <code>{primaryShortLink}</code></p>
						{:else if finalUrlPreview}
							<p class="mt-2 break-all text-xs" style="color: var(--primary);">{tr('Preview URL final:')} <code>{finalUrlPreview}</code></p>
						{/if}
					</div>

					<div class="rounded-[5px] border p-3">
						<div class="flex items-center justify-between">
							<p class="flex items-center gap-2 text-sm font-bold text-muted-foreground"><Link2 size={16} /> {tr('Tautan Cadangan (Prioritas 2)')}</p>
							<span class="rounded-[5px] px-2 py-0.5 text-[10px] font-semibold" style={badge('muted')}>{tr("Opsional")}</span>
						</div>
						<p class="mt-1 text-xs text-muted-foreground">{tr("Hanya akan digunakan secara dinamis oleh AI pada media mitra yang mengizinkan penanaman")} <strong>{tr('Maksimal 2 Link')}</strong>.</p>
						<div class="mt-3 grid gap-3 sm:grid-cols-2">
							<label class="block">
								<span class="text-xs font-medium">{tr('Target URL Cadangan')}</span>
								<div class="app-input mt-1 flex items-center gap-1">
									<span class="text-xs text-muted-foreground">https://</span>
									<input class="w-full bg-transparent text-sm outline-none" placeholder="tauthub.com/about" value={links.backup_url} oninput={(e) => patchLinks({ backup_url: stripScheme((e.currentTarget as HTMLInputElement).value) })} />
								</div>
							</label>
							<label class="block">
								<span class="text-xs font-medium">{tr('Anchor Text Cadangan')}</span>
								<input class="app-input mt-1 w-full" maxlength="120" placeholder={tr("Platform PR Terbaik Indonesia")} value={links.backup_anchor} oninput={(e) => patchLinks({ backup_anchor: (e.currentTarget as HTMLInputElement).value })} />
							</label>
						</div>
						{#if backupShortLink}
							<p class="mt-2 break-all text-xs" style="color: var(--primary);">{tr('Preview tautan cadangan:')} <code>{backupShortLink}</code></p>
						{:else if backupUrlPreview}
							<p class="mt-2 break-all text-xs" style="color: var(--primary);">{tr('Preview URL cadangan:')} <code>{backupUrlPreview}</code></p>
						{/if}
					</div>
				</div>
			</section>
		{/if}

				<!-- Banner Informasi Dapur Produksi -->
				<div class="relative mt-4 overflow-hidden rounded-[5px] border border-blue-100/80 bg-gradient-to-r from-blue-50/70 via-slate-50/40 to-blue-50/40 py-5 pl-5 pr-3.5 shadow-2xs">
				<div class="pointer-events-none absolute -right-2 -top-2 h-28 w-44 opacity-60">
					<svg viewBox="0 0 160 120" fill="none" class="h-full w-full">
						<path d="M110 20h30a6 6 0 0 1 6 6v70a6 6 0 0 1-6 6h-30a6 6 0 0 1-6-6V26a6 6 0 0 1 6-6z" fill="#93c5fd" fill-opacity="0.25" stroke="#60a5fa" stroke-width="1.5" stroke-dasharray="3 3"/>
						<path d="M120 12h32a6 6 0 0 1 6 6v70a6 6 0 0 1-6 6h-32a6 6 0 0 1-6-6V18a6 6 0 0 1 6-6z" fill="#ffffff" fill-opacity="0.8" stroke="#3b82f6" stroke-width="1.5"/>
						<line x1="128" y1="28" x2="150" y2="28" stroke="#93c5fd" stroke-width="2" stroke-linecap="round"/>
						<line x1="128" y1="38" x2="146" y2="38" stroke="#cbd5e1" stroke-width="2" stroke-linecap="round"/>
						<line x1="128" y1="48" x2="142" y2="48" stroke="#cbd5e1" stroke-width="2" stroke-linecap="round"/>
						<path d="M102 36l2 4 4 2-4 2-2 4-2-4-4-2 4-2 2-4z" fill="#3b82f6"/>
						<path d="M96 68l1.5 3 3 1.5-3 1.5-1.5 3-1.5-3-3-1.5 3-1.5 1.5-3z" fill="#60a5fa"/>
						<path d="M154 84l1.5 3 3 1.5-3 1.5-1.5 3-1.5-3-3-1.5 3-1.5 1.5-3z" fill="#93c5fd"/>
					</svg>
				</div>
				<div class="relative z-10 flex flex-wrap items-center justify-between gap-2.5">
					<div class="flex items-center gap-3.5">
						<div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-[5px] border border-blue-200/70 bg-white text-blue-600 shadow-2xs">
							<FileText size={20} class="stroke-[2.2]" />
						</div>
						<div>
							<h3 class="text-base font-bold text-slate-900 leading-tight">
								{manualFlow ? tr('Dapur Produksi') : tr('Dapur Produksi AI')} ({cost.count} {tr('Media Terpilih')})
							</h3>
							<p class="mt-1 text-xs text-slate-500">
								{#if manualFlow}
									{tr('Unggah artikel tulisanmu sendiri, atau buat dengan AI lewat tombol di tiap media.')}
								{:else}
									{tr('Biaya penulisan konten organik oleh The Writer:')} <strong class="text-slate-700">{toRp(writerCostCredits)}</strong> {tr('/ artikel — baru maupun tulis ulang.')}
								{/if}
							</p>
						</div>
					</div>
					<div class="flex shrink-0 items-center gap-2 lg:mr-15">
						<!-- Generate Semua di header: klik membuka popup yang sama dengan
						     bar seleksi (masih placeholder "Dalam Pengerjaan");
						     grup digeser ke kiri ~40px agar tidak tembus ke tepi. -->
						<button
							class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs disabled:opacity-50 disabled:cursor-not-allowed"
							type="button"
							disabled={pickedList.length < 2}
							title={pickedList.length < 2 ? tr('Pilih minimal 2 media untuk Generate Semua.') : tr('Generate artikel untuk semua media terpilih')}
							onclick={() => (bulkPopupOpen = true)}
						>
							<Sparkles size={13} /> {tr('Generate Semua')}
						</button>
						{#if campaignName}
							<span class="inline-flex max-w-[260px] cursor-help items-center gap-1.5 truncate rounded-[5px] border border-slate-200 bg-white px-3.5 py-1.5 text-xs font-semibold text-slate-700 shadow-2xs" title={`${tr('Ini adalah nama kampanye Anda')}: ${campaignName}`}>
								<FileText size={13} class="shrink-0 text-slate-400" />
								<span class="truncate">{campaignName}</span>
							</span>
						{/if}
	
					</div>
				</div>
			</div>

			{#if pickedList.length > 0}
				<!-- Bar aksi seleksi: muncul tepat di bawah header Dapur Produksi. -->
				<div class="sticky top-0 z-10 mt-3 flex flex-wrap items-center gap-2 rounded-[5px] border border-blue-200/80 bg-blue-50/90 px-3.5 py-2 shadow-2xs backdrop-blur-sm">
					<span class="text-xs font-bold text-blue-700">{pickedList.length} {tr('media dipilih')}</span>
					<div class="flex flex-1 flex-wrap items-center justify-end gap-2">
						<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={pickAll}>
							{allPicked ? tr('Batal Pilih Semua') : tr('Pilih Semua')}
						</button>
					</div>
				</div>
			{/if}

			<!-- Daftar Media Terpilih -->
			{#if selected.length === 0}
				<div class="mt-4 rounded-[5px] border border-slate-200 bg-white px-4 py-12 text-center text-sm text-slate-500 shadow-2xs">
					{tr("Belum ada media terpilih. Kembali ke Matchmaker untuk memilih media.")}
				</div>
			{:else}
				<div class="mt-4 space-y-3">
					{#each selected as m (m.media_id)}
						{@const st = articleStatusOf(m)}
						{@const statusKind = st === 'error' ? 'none' : st}
						{@const generating = st === 'generating'}
						{@const queued = st === 'queued'}
						{@const links2 = linkStats[m.media_id] ?? null}
						{@const domain = cleanDomain(m.website_url, m.name)}
						{@const av = avatarStyle(m.name)}
						<div
							class="rounded-[5px] border bg-white p-4.5 sm:p-5 shadow-2xs transition-all duration-200 {pickedIds[m.media_id]
								? 'border-blue-400 bg-blue-50/20 hover:border-blue-400'
								: 'border-slate-200/80 hover:border-slate-300 hover:shadow-xs'}"
						>
							<div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
								<!-- Kolom 1: Identitas Media -->
								<div class="flex min-w-0 items-center gap-3.5 lg:w-[280px] shrink-0">
									<input
										type="checkbox"
										class="h-4 w-4 shrink-0 cursor-pointer rounded-[4px] accent-blue-600"
										checked={!!pickedIds[m.media_id]}
										title={tr('Pilih media ini')}
										aria-label={`${tr('Pilih media ini')}: ${m.name}`}
										onclick={(e) => { e.stopPropagation(); togglePicked(m.media_id); }}
									/>
									<div
										class="flex h-11 w-11 shrink-0 items-center justify-center rounded-[5px] overflow-hidden border border-slate-200/60 font-bold text-base"
										style={domain && !avatarFailed[m.media_id]
											? 'background-color: transparent;'
											: `background-color: ${av.bg}; color: ${av.text};`}
									>
										{#if domain && !avatarFailed[m.media_id]}
											<img
												src={`https://www.google.com/s2/favicons?domain=${encodeURIComponent(domain)}&sz=128`}
												alt={m.name}
												loading="lazy"
												decoding="async"
												class="h-full w-full object-cover"
												onerror={() => { avatarFailed = { ...avatarFailed, [m.media_id]: true }; }}
											/>
										{:else}
											<span>{initialOf(m.name)}</span>
										{/if}
									</div>

									<div class="min-w-0 flex-1">
										<div class="flex items-center gap-2">
											<p class="font-bold text-[14.5px] leading-tight text-slate-900 truncate" title={m.name}>
												{mediaDomainLabel(m.website_url, m.name)}
											</p>
											{#if m.media_type}
												<span
													class="shrink-0 rounded-[5px] border px-2 py-0.5 text-[7.5px] font-bold tracking-wider uppercase {m.media_type === 'blogger' ? 'border-amber-200/80 bg-amber-50 text-amber-700' : 'border-blue-200/80 bg-blue-50 text-blue-600'}"
												>
													{m.media_type === 'blogger' ? tr('Blogger') : tr('Mainstream')}
												</span>
											{/if}
										</div>

										<div class="mt-1.5 flex flex-wrap items-center gap-1.5">
											{#if statusKind === 'ready'}
												<span class="inline-flex items-center gap-1 rounded-[5px] border border-emerald-200/80 bg-emerald-50 px-2.5 py-0.5 text-[9px] font-medium text-emerald-700">
													<CircleCheck size={11} strokeWidth={2.2} />
													<span>{tr('Siap Terbit')}</span>
												</span>
											{:else if statusKind === 'draft'}
												<span class="inline-flex items-center gap-1 rounded-[5px] border border-blue-200/80 bg-blue-50 px-2.5 py-0.5 text-[9px] font-medium text-blue-700">
													<FileText size={11} />
													<span>{tr('Draft Tersedia')}</span>
												</span>
											{:else if statusKind === 'generating'}
												<span class="inline-flex items-center gap-1 rounded-[5px] border border-blue-200/80 bg-blue-50 px-2.5 py-0.5 text-[9px] font-medium text-blue-600">
													<LoaderCircle size={11} class="animate-spin" />
													<span>{tr('Generasi AI…')}</span>
												</span>
											{:else if statusKind === 'queued'}
												<span class="inline-flex items-center gap-1 rounded-[5px] border border-slate-200 bg-slate-100/80 px-2.5 py-0.5 text-[9px] font-medium text-slate-600">
													<Clock size={11} />
													<span>{tr('Dalam Antrean…')}</span>
												</span>
											{:else if statusKind === 'blocked'}
												<span class="inline-flex items-center gap-1 rounded-[5px] border border-rose-200 bg-rose-50 px-2.5 py-0.5 text-[9px] font-medium text-rose-700">
													<X size={11} />
													<span>{tr('Diblokir')}</span>
												</span>
											{:else}
												<span class="inline-flex items-center gap-1 rounded-[5px] border border-slate-200/70 bg-slate-100/80 px-2.5 py-0.5 text-[9px] font-medium text-slate-600">
													<Clock size={11} />
													<span>{tr('Menunggu Artikel')}</span>
												</span>
											{/if}

											{#if !mediaAllowsLink(m)}
												<span class="rounded-[5px] border border-slate-200 bg-slate-100 px-2.5 py-0.5 text-[9px] font-medium text-slate-500">
													{tr('Tidak Tersedia Backlink')}
												</span>
											{:else if links2}
												{#if links2.sponsored > 0}
													<span class="inline-flex items-center gap-1 rounded-[5px] border border-amber-200/80 bg-amber-50 px-2 py-0.5 text-[9px] font-medium text-amber-800">
														<PencilLine size={10.5} />
														<span>{links2.sponsored} Sponsored</span>
													</span>
												{/if}
												{#if links2.dofollow > 0}
													<span class="inline-flex items-center gap-1 rounded-[5px] border border-amber-200/80 bg-amber-50 px-2 py-0.5 text-[9px] font-medium text-amber-800">
														<PencilLine size={10.5} />
														<span>{links2.dofollow} Dofollow</span>
													</span>
												{/if}
												<!-- Lapor hanya jenis link yang benar-benar ada; badge berangka 0
												     (mis. "0 Nofollow" di samping "1 Dofollow") tidak berguna. -->
												{#if links2.nofollow > 0 || links2.nofollow + links2.dofollow + links2.sponsored === 0}
													<span class="inline-flex items-center gap-1 rounded-[5px] border border-slate-200 bg-slate-100/70 px-2 py-0.5 text-[9px] font-medium text-slate-600">
														<span class="h-1.5 w-1.5 rounded-full border border-slate-400"></span>
														<span>{links2.nofollow > 0 ? `${links2.nofollow} Nofollow` : '0 Tautan'}</span>
													</span>
												{/if}
											{:else}
												<span class="inline-flex items-center gap-1 rounded-[5px] border border-amber-200/80 bg-amber-50 px-2.5 py-0.5 text-[9px] font-medium text-amber-800">
													<PencilLine size={10.5} />
													<span>{m.label_sponsor === 'ada_sponsor' ? 'Sponsored' : 'Dofollow'} (Maks. {m.max_links != null && m.max_links > 0 ? m.max_links : 1})</span>
												</span>
											{/if}
										</div>
									</div>
								</div>

								<!-- Kolom 2: Informasi Artikel (mengisi sisa lebar, tanpa batas atas) -->
								<div class="min-w-0 flex-1">
									{#if articleTitles[m.media_id]}
										<div>
											<p class="text-[11px] font-medium text-slate-400 mb-0.5">{tr('Judul Artikel')}</p>
											<p class="line-clamp-2 text-[13.5px] font-semibold text-slate-900 leading-snug" title={articleTitles[m.media_id]}>
												{articleTitles[m.media_id]}
											</p>
										</div>
									{:else}
										<div>
											<p class="text-sm font-semibold text-slate-400 leading-none">-</p>
											<p class="mt-1.5 inline-flex items-center gap-1.5 text-xs text-slate-400 font-normal">
												<Image size={13} class="shrink-0 text-slate-300" />
												<span>{tr('Gambar artikel: tidak ada')}</span>
											</p>
										</div>
									{/if}
								</div>

									<!-- Kolom 3: Cover & Metadata -->
									<div class="flex items-center gap-3 shrink-0">
										<!-- Thumbnail cover: di mobile & tablet diperbesar supaya gambarnya
										     terbaca jelas; ukuran desktop tetap seperti desain awal. -->
										<div class="h-24 w-36 sm:h-30 sm:w-44 lg:h-13 lg:w-18 shrink-0 overflow-hidden rounded-[5px] border border-slate-200/80 bg-slate-100/70 flex items-center justify-center">
											{#if hasDoc(m) && articleStats[m.media_id]?.cover}
												<img src={sameOriginAssetUrl(articleStats[m.media_id].cover) ?? ''} alt="Cover" class="h-full w-full object-cover" />
											{:else}
												<Image size={20} class="text-slate-400 stroke-[1.5] h-8 w-8 sm:h-9 sm:w-9 lg:h-5 lg:w-5" />
											{/if}
										</div>
									<div class="flex flex-col gap-0.5">
										{#if hasDoc(m) && articleStats[m.media_id]}
											{#if articleStats[m.media_id].cover}
												<span class="inline-flex items-center gap-1 text-xs font-semibold text-emerald-600">
													<Check size={12} strokeWidth={2.4} />
													<span>{tr('Cover ada')}</span>
												</span>
											{:else}
												<span class="inline-flex items-center gap-1 text-xs font-semibold text-rose-600">
													<CircleAlert size={12} strokeWidth={2.2} />
													<span>{tr('Cover tidak ada')}</span>
												</span>
											{/if}
											<span class="text-xs text-slate-500 font-normal">
												{articleStats[m.media_id].words.toLocaleString('id-ID')} {tr('kata')}
											</span>
										{:else}
											<span class="inline-flex items-center gap-1 text-xs font-medium text-slate-500">
												<FileText size={12} class="text-slate-400" />
												<span>{tr('Cover belum ada')}</span>
											</span>
											<span class="text-xs text-slate-400 font-normal">0 {tr('kata')}</span>
										{/if}
									</div>
								</div>

								<!-- Kolom 4: Area Aksi (2x2) -->
								<!-- Aksi: satu baris ikon; tooltip muncul di atas tombol
								     saat hover dengan animasi halus. -->
									<div class="w-full lg:w-auto shrink-0 flex flex-wrap items-center justify-start lg:justify-end gap-2 overflow-visible">
										<button
											type="button"
											class={`expAction h-8 min-w-8 rounded-[5px] inline-flex items-center justify-center text-xs font-semibold transition-colors ${
												hasDoc(m)
													? 'bg-blue-600 hover:bg-blue-700 text-white shadow-2xs'
													: 'bg-blue-50/90 hover:bg-blue-100 text-blue-600 border border-blue-200/80'
											}`}
											aria-label={tr('Cek Validasi')}
											disabled={bulkRunning || viewOnly}
											onclick={() => (checklistFor = m.media_id)}
										>
											<ShieldCheck size={14} strokeWidth={2.2} class="shrink-0" />
											<span class="expTip">{tr('Cek Validasi')}</span>
										</button>

										{#if hasDoc(m)}
											<button
												type="button"
												class="expAction h-8 min-w-8 inline-flex items-center justify-center rounded-[5px] border border-slate-200 bg-white hover:bg-slate-50 hover:border-slate-300 text-slate-700 text-xs font-semibold transition-colors"
												aria-label={tr('Edit Artikel')}
												disabled={bulkRunning}
												onclick={() => void openPreview(m)}
											>
												<Pencil size={12} strokeWidth={2} class="shrink-0" />
												<span class="expTip">{tr('Edit Artikel')}</span>
											</button>

											{@const qcCount = qcCounts[m.media_id] ?? 0}
											{@const qcBusy = qcRunning[m.media_id] === true}
											<!-- Badge menampilkan JUMLAH PEMERIKSA panel (satu ceklis),
											     bukan jumlah riwayat pemindaian — angkanya selalu
											     sinkron dengan daftar kartu pada panel Analisis QC. -->
											{@const qcBadge = qcCount > 0 ? QC_CHECK_COUNT : 0}
											<!-- HIJAU = hasil QC terakhir lulus (artikel Siap Terbit):
											     dibaca dari status artikel, bukan memori sesi, supaya
											     tetap hijau setelah refresh halaman. -->
											{@const qcSelesai = !qcBusy && m.article_status === 'ready' && qcCount > 0}
											<button
												type="button"
												class="expAction relative h-8 min-w-8 inline-flex items-center justify-center rounded-[5px] border transition-colors {qcSelesai
													? 'border-emerald-200 bg-emerald-50 hover:bg-emerald-100 text-emerald-700'
													: 'border-blue-200/70 bg-blue-50/70 hover:bg-blue-100/80 text-blue-700'}"
												aria-label={qcBusy
													? tr('QC sedang memeriksa artikel…')
													: qcCount > 0 ? `${tr('Analisis QC')} (${QC_CHECK_COUNT})` : tr('Analisis QC')}
												disabled={bulkRunning || qcBusy}
												onclick={() => (qcFor = m.media_id)}
											>
												{#if qcBusy}
													<LoaderCircle size={13} strokeWidth={2} class="shrink-0 animate-spin" />
												{:else}
													<ListChecks size={13} strokeWidth={2} class="shrink-0" />
												{/if}
												<span class="expTip">{qcBusy ? tr('QC sedang memeriksa artikel…') : qcCount > 0 ? `${tr('Analisis QC')} (${QC_CHECK_COUNT})` : tr('Analisis QC')}</span>
												<!-- Angka QC nempel di pojok kanan atas pill, tetap tampil
												     meski tombol sedang menciut jadi ikon saja; hijau = QC selesai. -->
												<span
													class="pointer-events-none absolute -right-1.5 -top-1.5 inline-flex h-4 min-w-4 items-center justify-center rounded-[5px] px-1 text-[10px] font-bold leading-none ring-2 ring-white"
													style={qcSelesai
														? 'background-color: var(--success, #16794b); color: #fff;'
														: qcCount > 0 ? 'background-color: #2563eb; color: #fff;' : 'background-color: #e2e8f0; color: #64748b;'}
												>
													{qcBadge}
												</span>
											</button>
										{:else}
											<button
												type="button"
												class="expAction h-8 min-w-8 inline-flex items-center justify-center rounded-[5px] border border-slate-200 bg-white hover:bg-slate-50 hover:border-slate-300 text-slate-700 text-xs font-semibold transition-colors"
												aria-label={generating ? tr('Membuat…') : queued ? tr('Mengantre…') : tr('Generate Konten')}
												disabled={generating || bulkRunning || viewOnly}
												onclick={() => openNaskahGenerate(m)}
											>
												{#if generating}
													<LoaderCircle size={12} class="animate-spin text-blue-600 shrink-0" />
													<span class="expTip">{tr('Membuat…')}</span>
												{:else if queued}
													<Clock size={12} class="text-slate-400 shrink-0" />
													<span class="expTip">{tr('Mengantre…')}</span>
												{:else}
													<Sparkles size={12} class="text-slate-500 shrink-0" />
													<span class="expTip">{tr('Generate Konten')}</span>
												{/if}
											</button>

												<button
													type="button"
													class="expAction h-8 min-w-8 inline-flex items-center justify-center rounded-[5px] border border-slate-200 bg-white hover:bg-slate-50 hover:border-slate-300 text-slate-700 text-xs font-semibold transition-colors"
													aria-label={tr('Tulis Sendiri')}
													disabled={bulkRunning || viewOnly}
													onclick={() => openNaskahUpload(m)}
												>
													<PencilLine size={12} strokeWidth={2} class="text-slate-500 shrink-0" />
													<span class="expTip">{tr('Tulis Sendiri')}</span>
												</button>
										{/if}

										<!-- Hapus: ikon saja (tidak ikut auto expand), nama tetap lewat
										     aria-label + tooltip; hover cukup mengubah warna tombol. -->
										<button
											type="button"
											class="expAction inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-[5px] border border-rose-200 bg-rose-50 text-rose-600 hover:bg-rose-100 hover:border-rose-300 hover:text-rose-700 transition-colors"
											disabled={bulkRunning || removeBusy || viewOnly}
											aria-label={tr('Hapus media dari kampanye')}
											onclick={() => { removeError = ''; removeTarget = m; }}
										>
											<Trash2 size={14} class="shrink-0" />
											<span class="expTip">{tr('Hapus media dari kampanye')}</span>
										</button>
									</div>
							</div>
						</div>
					{/each}
				</div>
			{/if}

			{#if removeTarget}
				<div class="fixed inset-0 z-[70] flex items-center justify-center p-4" style="background-color: rgba(15, 23, 42, 0.55);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget && !removeBusy) removeTarget = null; }}>
					<div class="flex w-full max-w-md flex-col overflow-hidden rounded-[5px] border shadow-2xl" style="border-color: var(--border); background-color: var(--card, #fff);" role="dialog" aria-modal="true" tabindex="-1" aria-label={tr('Hapus Media dari Kampanye')} onclick={(e) => e.stopPropagation()} onkeydown={(e) => { if (e.key === 'Escape' && !removeBusy) removeTarget = null; }}>
						<div class="flex items-start justify-between gap-3 border-b px-5 py-4" style="border-color: var(--border);">
							<div class="flex items-center gap-2.5 text-sm font-bold" style="color: var(--destructive);">
								<Trash2 size={18} />
								<span>{tr('Hapus Media dari Kampanye?')}</span>
							</div>
							<button class="rounded-[5px] p-1.5 text-muted-foreground hover:bg-secondary" type="button" aria-label={tr('Tutup')} disabled={removeBusy} onclick={() => (removeTarget = null)}>
								<X size={16} />
							</button>
						</div>
						<div class="space-y-3 p-5">
							<p class="text-sm font-semibold">{removeTarget.name}</p>
							<p class="text-xs leading-relaxed text-muted-foreground">
								{tr('Media ini akan dilepas dari kampanye (tidak dihapus permanen). Artikel dan datanya tetap tersimpan, jadi bisa dipakai lagi nanti.')}
							</p>
							<p class="rounded-[5px] p-2.5 text-xs leading-relaxed" style="background-color: var(--muted, #f8fafc); color: var(--foreground);">
								{tr('Untuk mengaktifkan kembali media ini, buka The Matchmaker dan pilih ulang medianya.')}
							</p>
							{#if removeError}
								<p class="flex items-center gap-1.5 text-xs" style="color: var(--destructive);"><CircleAlert size={14} /> {removeError}</p>
							{/if}
						</div>
						<div class="flex items-center justify-end gap-2.5 border-t px-5 py-3.5" style="border-color: var(--border); background-color: var(--muted, #f8fafc);">
							<button class="app-btn app-btn-soft text-xs" type="button" disabled={removeBusy} onclick={() => (removeTarget = null)}>{tr('Batal')}</button>
							<button class="app-btn text-xs font-semibold" style="background-color: var(--destructive); color: #fff;" type="button" disabled={removeBusy} onclick={async () => {
								if (!removeTarget) return;
								removeBusy = true;
								removeError = '';
								try {
									const removedId = removeTarget.media_id;
									const remaining = placement
										.filter((p) => p.status === 'selected' && p.media_id !== removedId)
										.map((p) => p.media_id);
									const res = await apiRequest(`${apiBase}/placement`, { method: 'PUT', body: JSON.stringify({ selected: remaining }) });
									if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menghapus media dari kampanye.')));
									placement = placement.map((p) => (p.media_id === removedId ? { ...p, status: 'removed' } : p));
									removeTarget = null;
									await load();
								} catch (err) {
									removeError = err instanceof Error ? err.message : tr('Gagal menghapus media dari kampanye.');
								} finally {
									removeBusy = false;
								}
							}}>
								{#if removeBusy}<LoaderCircle size={14} class="animate-spin" /> <span>{tr('Melepas…')}</span>{:else}<Trash2 size={13} /> <span>{tr('Ya, Lepas Media')}</span>{/if}
							</button>
						</div>
					</div>
				</div>
			{/if}

				<TautBottomBar fullWidth>
					<div class="flex w-full flex-wrap items-center justify-between gap-4 py-1">
						<!-- Kolom Kiri: Total Tagihan Akhir -->
						<div class="relative inline-flex items-center">
							<button
								class="flex items-center gap-3 rounded-[5px] border border-emerald-200/90 bg-emerald-50/70 hover:bg-emerald-50/90 px-4 py-2 text-left transition-colors cursor-pointer"
								type="button"
								aria-expanded={costPopoverOpen}
								onmouseenter={() => (costPopoverOpen = true)}
								onmouseleave={() => { if (!costPinned) costPopoverOpen = false; }}
								onfocus={() => (costPopoverOpen = true)}
								onblur={() => { if (!costPinned) costPopoverOpen = false; }}
								onclick={() => { costPinned = !costPinned; costPopoverOpen = costPinned; }}
							>
								<div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-[5px] bg-emerald-100/90 text-emerald-700">
									<Wallet size={18} strokeWidth={2.2} />
								</div>
								<div>
									<span class="flex items-center gap-1 text-[10px] font-bold uppercase tracking-wider text-slate-500">
										{tr('TOTAL TAGIHAN AKHIR')}
										<Info size={11} class="opacity-70" />
									</span>
									<span class="selectedStatPriceVal selectedStatTotalVal text-lg sm:text-xl font-extrabold text-emerald-700 leading-tight">
										{toRp(cost.total)}
									</span>
								</div>
							</button>

							{#if billDelta}
								{#key billDelta.id}
									<span class="billDeltaWrap" aria-hidden="true">
										<span class="billDelta" class:down={billDelta.delta < 0}>
											{billDelta.delta > 0 ? '+' : '-'}{toRp(Math.abs(billDelta.delta))}
										</span>
									</span>
								{/key}
							{/if}

							{#if costPopoverOpen}
								{#if costPinned}
									<button
										class="fixed inset-0 z-30 cursor-default"
										type="button"
										aria-label={tr('Tutup rincian biaya')}
										onclick={() => { costPinned = false; costPopoverOpen = false; }}
									></button>
								{/if}
								<div class="costPopover" role="dialog" tabindex="-1" aria-label={tr('Rincian biaya')}
									onkeydown={(e) => { if (e.key === 'Escape') { costPinned = false; costPopoverOpen = false; } }}
								>
									<p class="costPopoverTitle">{tr('Rincian biaya')}</p>

									<div class="costPopoverRow">
										<span>
											{tr('Biaya media')} <span class="text-muted-foreground">({cost.count} {tr('media')})</span>
										</span>
										<span class="font-medium">{toRp(cost.media)}</span>
									</div>

									<div class="costPopoverRow">
										<span>
											{tr('Biaya penulisan AI')}
											{#if cost.writingUnits > 0}
												<span class="text-muted-foreground">({cost.writingUnits} {tr('artikel')}{writerRegenCount > 0 ? ` + ${writerRegenCount} ${tr('tulis ulang')}` : ''})</span>
												<span class="costPopoverRowNote">{cost.writingUnits} × {toRp(writerCostCredits)}</span>
											{:else}
												<span class="text-muted-foreground">({tr('belum memakai AI')})</span>
											{/if}
										</span>
										<span class="font-medium">{toRp(cost.writing)}</span>
									</div>

									<div class="costPopoverRow">
										<span>
											{tr('Biaya gambar AI')} <span class="text-muted-foreground">({cost.imageCount} {tr('gambar')})</span>
											{#if cost.imageCount > 0}<span class="costPopoverRowNote">{cost.imageCount} × {toRp(imageCreditCost)}</span>{/if}
										</span>
										<span class="font-medium">{toRp(cost.images)}</span>
									</div>

									<div class="costPopoverDivider"></div>

									<div class="costPopoverTotal">
										<span>{tr('TOTAL TAGIHAN AKHIR')}</span>
										<span>{toRp(cost.total)}</span>
									</div>

								</div>
							{/if}
						</div>

						<!-- Kolom Tengah: Pesan Status QC -->
						<div class="hidden md:flex ml-auto items-center gap-2 max-w-sm text-xs leading-snug">
							{#if distributable}
								<CircleCheck size={16} class="text-emerald-600 shrink-0" />
								<span class="text-slate-600 font-medium">{tr('QC sudah lulus')}</span>
							{:else if distributable === false}
								<TriangleAlert size={16} class="text-amber-600 shrink-0" />
								<span class="text-amber-800 font-medium">{tr('QC belum lulus — klik Lanjut Tinjau & Terbit untuk melihat prasyarat.')}</span>
							{/if}
						</div>

						<!-- Kolom Kanan: Tombol Aksi -->
						<div class="flex items-center gap-2.5">
							<!-- Kembali mengarah ke langkah sebelumnya: Katalog Media
							     (klien memilih/menambah media), dengan konteks kampanye
							     dibawa agar centang pilihan tetap terjaga. -->
								<button class="h-10 px-4.5 rounded-lg border border-slate-200 bg-white hover:bg-slate-50 hover:border-slate-300 text-slate-700 text-[13px] font-medium transition-all whitespace-nowrap" type="button" onclick={() => goto(`/klien/${brandUrl}/media?campaign=${campaignId}`)}>{tr('Kembali')}</button>
								<div class="relative">
									<button class="h-10 px-5 rounded-lg bg-slate-900 hover:bg-slate-800 hover:-translate-y-px text-white text-[13px] font-semibold inline-flex items-center gap-1.5 transition-all whitespace-nowrap disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:translate-y-0" type="button" disabled={coverChecking || distributing || viewOnly} title={viewOnly ? tr(VIEW_ONLY_MSG) : undefined} aria-haspopup="dialog" aria-expanded={readinessOpen} onclick={publish}>
										{#if coverChecking || distributing}<LoaderCircle size={14} class="animate-spin" />{:else}<Send size={14} />{/if}
										<span>{coverChecking ? tr('Memeriksa cover…') : distributing ? tr('Menerbitkan…') : tr('Lanjut Tinjau & Terbit')}</span>
									</button>

									{#if readinessOpen}
										<button class="fixed inset-0 z-30 cursor-default" type="button" aria-label={tr('Tutup')} onclick={() => (readinessOpen = false)}></button>
										<div
											class="readyPopover"
											role="dialog"
											aria-modal="true"
											aria-label={tr('Prasyarat terbit')}
											tabindex="-1"
											onkeydown={(e) => { if (e.key === 'Escape') readinessOpen = false; }}
										>
											<div class="flex items-start justify-between gap-3">
												<div class="flex items-center gap-2">
													<span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full" style="background-color: #fff7ed; color: #c2410c;">
														<TriangleAlert size={16} />
													</span>
													<div>
														<p class="text-[13.5px] font-bold text-slate-900">{tr('Belum siap terbit')}</p>
														<p class="text-[11.5px] leading-snug text-muted-foreground">{tr('Selesaikan prasyarat berikut dulu.')}</p>
													</div>
												</div>
												<button class="rounded-lg p-1 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600" type="button" aria-label={tr('Tutup')} onclick={() => (readinessOpen = false)}>
													<X size={15} />
												</button>
											</div>

											<div class="mt-3 space-y-2">
												{#if readinessGroups.length === 0}
													<p class="text-[12px] leading-snug text-slate-600">{tr('QC belum bisa diverifikasi. Muat ulang halaman lalu coba lagi.')}</p>
												{/if}
												{#each readinessGroups as g (g.key)}
													<div class="rounded-lg border border-slate-200 bg-white p-2.5">
														<div class="flex items-center justify-between gap-2">
															<span class="inline-flex items-center gap-1.5 text-[12px] font-semibold text-slate-800">
																{#if g.key === 'qc' || g.key === 'qc-fail'}<ShieldCheck size={13} />{:else if g.key === 'cover'}<Image size={13} />{:else if g.key === 'category'}<FileText size={13} />{:else}<Sparkles size={13} />{/if}
																{g.label}
															</span>
															<span class="rounded-full bg-slate-100 px-2 py-0.5 text-[10.5px] font-bold text-slate-600">{g.items.length}</span>
														</div>
														<div class="mt-1.5 flex flex-wrap gap-1.5">
															{#each g.items as item (item.mediaId)}
																<span class="readyChip">{item.name}</span>
															{/each}
														</div>
															{#if g.key === 'article'}
																<p class="mt-1.5 text-[11px] leading-snug text-slate-500">{tr('Buat artikel lewat Generate Konten atau Tulis Sendiri.')}</p>
													{:else if g.key === 'cover'}
														<button class="app-btn app-btn-soft mt-2 !min-h-7 !px-2.5 !text-[11px]" type="button" disabled={coverBusy} onclick={() => void useDefaultCovers(g.items)}>
															{#if coverBusy}<LoaderCircle size={12} class="animate-spin" />{:else}<Image size={12} />{/if}
															{coverBusy ? tr('Menerapkan…') : tr('Pakai cover default')}
														</button>
													{:else if g.key === 'qc' || g.key === 'qc-fail'}
														<button
															class="app-btn app-btn-soft mt-2 !min-h-7 !px-2.5 !text-[11px]"
															type="button"
															disabled={bulkQcRunning || viewOnly}
															title={tr('Pindai QC semua media di daftar ini sekaligus')}
															onclick={() => void runQcAll(g.items.map((i) => i.mediaId))}
														>
															{#if bulkQcRunning}<LoaderCircle size={12} class="animate-spin" />{:else}<ShieldCheck size={12} />{/if}
															{bulkQcRunning ? tr('Memeriksa…') : tr('Cek QC Sekarang')}
														</button>
													{/if}
													</div>
												{/each}
											</div>

											<div class="mt-3">
												{#if bulkQcRunning}
													<div class="readyProgress" aria-hidden="true"><span></span></div>
													<p class="mt-1.5 flex items-center gap-1.5 text-[11.5px] text-slate-600">
														<LoaderCircle size={13} class="animate-spin" />
														{tr('Memeriksa')} {qcTargets.length || selected.length} {tr('artikel')}…
													</p>
												{:else if qcTargets.length > 0}
													<button class="app-btn app-btn-primary inline-flex w-full items-center justify-center gap-1.5 !min-h-9 !px-3 !text-xs" type="button" disabled={viewOnly} onclick={() => void runQcAll()}>
														<ShieldCheck size={14} /> {tr('Cek Semua QC')} ({qcTargets.length})
													</button>
												{:else}
													<button class="app-btn app-btn-soft w-full !min-h-9 !px-3 !text-xs" type="button" onclick={() => (readinessOpen = false)}>{tr('Tutup')}</button>
												{/if}
											</div>
										</div>
									{/if}
								</div>
							</div>
						</div>
				</TautBottomBar>
	{/if}
</main>

{#if checklistItem}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<div class="app-card w-full max-w-2xl max-h-[92dvh] overflow-y-auto">
			<!-- Kepala: ikon + judul, dan tombol tutup X -->
			<div class="flex items-start gap-3 border-b border-slate-100 px-5 pb-4 pt-5">
				<span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl" style="background-color: color-mix(in srgb, var(--primary) 10%, transparent); color: var(--primary);">
					<ListChecks size={21} />
				</span>
				<div class="min-w-0 flex-1">
					<p class="text-lg leading-tight font-bold text-slate-900">{tr('Cek Validasi Artikel')}</p>
					<p class="mt-1 text-[13.5px] text-muted-foreground">{tr("Ringkasan indikator artikel untuk media")} {checklistItem.name}.</p>
				</div>
				<button class="rounded-lg p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600" type="button" aria-label={tr('Tutup')} onclick={() => (checklistFor = null)}>
					<X size={18} />
				</button>
			</div>

			<!-- Indikator: kartu ikon + keterangan + chip nilai -->
			<div class="grid grid-cols-1 gap-2.5 p-5 sm:grid-cols-2">
				{#each checkRows as row (row.label)}
					<div class="flex items-start gap-3 rounded-2xl border border-slate-200 bg-white p-3.5">
						<span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full" style="background-color: color-mix(in srgb, var(--primary) 10%, transparent); color: var(--primary);">
							<row.Icon size={17} />
						</span>
						<div class="min-w-0 flex-1">
							<p class="text-[13.5px] font-semibold text-slate-900">{row.label}</p>
							<p class="mt-0.5 text-xs leading-relaxed text-muted-foreground">{row.sub}</p>
						</div>
						<span class="shrink-0 rounded-full px-3 py-1.5 text-xs font-bold whitespace-nowrap {row.tone === 'pos' ? 'bg-emerald-50 text-emerald-700' : row.tone === 'warn' ? 'bg-rose-50 text-rose-700' : 'bg-slate-100 text-slate-600'}">
							{row.value}
						</span>
					</div>
				{/each}
			</div>

			<!-- Kaki: catatan khusus + banner saran + tombol tutup -->
			<div class="px-5 pb-5">
				{#if !checkStats}
					<p class="mb-3 rounded-xl bg-amber-50 px-3.5 py-2.5 text-[12.5px] text-amber-800">{tr("Artikel belum dibuat untuk media ini — indikator ditulis \"Tidak ada\".")}</p>
				{/if}
				<div class="flex items-center gap-2.5 rounded-xl px-3.5 py-3" style="background-color: color-mix(in srgb, var(--primary) 7%, transparent);">
					<Info size={16} class="shrink-0" style="color: var(--primary);" />
					<p class="text-[13px] text-slate-700">{tr('Pastikan semua indikator sesuai sebelum melanjutkan publikasi.')}</p>
				</div>
				<button class="app-btn app-btn-primary mt-4 h-11 w-full rounded-[5px] text-[13.5px] font-semibold" type="button" onclick={() => (checklistFor = null)}>{tr('Tutup')}</button>
			</div>
		</div>
	</div>
{/if}

{#if editor}
	{@const ed = editor}
	{@const targetMedia = placement.find((p) => p.media_id === ed.mediaId)}
	{@const angleChannel = angleChannels.find((ch) => ch.slug === targetMedia?.media_type)}
	{#key ed.mediaId}
		<NaskahModal
			{brandId}
			{campaignId}
			mediaId={ed.mediaId}
			mediaName={ed.mediaName}
			mediaDomain={mediaDomainOf(targetMedia)}
			initialTab={ed.tab}
			initialSource={ed.source ?? null}
			initialTitle={ed.title}
			initialHtml={ed.html}
			initialCoverUrl={ed.cover}
			initialCategory={ed.category}
			initialMetaDescription={ed.metaDescription}
			categoryOptions={targetMedia?.categories ?? []}
			angleOptions={angleChannel?.angles.filter((a) => a.has_prompt === true) ?? []}
			angleChannelName={angleChannel?.name ?? ''}
			initialAngleSlug={targetMedia?.angle_slug ?? ''}
			{anglesLoading}
			{anglesError}
			onRetryAngles={() => void loadAngles()}
			mediaType={targetMedia?.media_type ?? ''}
			wordCountMode={targetMedia?.word_count_mode ?? ed.wordCountMode ?? ''}
			embedAllowed={targetMedia?.allow_embed ?? true}
			linkLimit={ed.linkLimit}
			sponsored={targetMedia?.label_sponsor === 'ada_sponsor'}
			saving={editorSaving}
			saveError={editorError}
			onClose={() => { if (!editorSaving) editor = null; }}
			onRefresh={() => void load()}
			onSave={(title, html, coverUrl, category, metaDescription) => void saveEditor(title, html, coverUrl, category, metaDescription)}
			onUploadCover={uploadCover}
			onUploadDoc={uploadDocNaskah}
			onGenerate={(topic, links, angleSlug, proper) => generateNaskah(ed.mediaId, topic, links, angleSlug, proper ?? null)}
			streamingHtml={streamHtml}
		/>
	{/key}
{/if}

{#if publishOpen}
	<SlideOver
		open={publishOpen}
		title={tr('Terbitkan Kampanye')}
		subtitle={tr('Pilih terbit langsung atau jadwalkan, lalu kirim ke media terpilih.')}
		closeLabel={tr('Tutup')}
		onClose={() => (publishOpen = false)}
	>
		<div class="space-y-3">
			<div class="rounded-[5px] border p-3">
				<p class="text-[11px] font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('Waktu Terbit per Media')}</p>
				<p class="mt-1 text-[11px] text-muted-foreground">{tr('Atur satu per satu: biarkan "Sekarang" untuk terbit langsung, atau pilih tanggal untuk menjadwalkan media itu saja.')}</p>
				<p class="mt-1 text-[11px] text-muted-foreground">{tr('Media yang dijadwalkan terbit belakangan bisa diedit dan diubah jadwalnya dari halaman Ringkasan.')}</p>
				<!-- Daftar ini punya area gulir sendiri: dengan puluhan media, bloknya tidak
				     memanjang sampai mendorong isi lain keluar panel. -->
				<div class="mt-2 max-h-[42vh] space-y-2 overflow-y-auto pr-1">
					{#each selected as m (m.media_id)}
						{@const explicit = scheduleByMedia[m.media_id] ?? ''}
						<div class="rounded-[5px] border p-2">
							<div class="flex items-center justify-between gap-2">
								<span class="min-w-0 flex-1 truncate text-[13px] font-semibold">{m.name}</span>
								<span class="shrink-0 text-[11px] font-medium" style="color: {explicit ? 'var(--primary)' : 'var(--success)'};">{explicit ? tr('Terjadwal') : tr('Langsung')}</span>
							</div>
							<div class="mt-1.5 flex flex-wrap items-center gap-1.5">
								<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-[11px]" type="button" style={quickBtnStyle(quickPick[m.media_id] === 'now')} onclick={() => setMediaSchedule(m.media_id, 'now')}>{tr('Sekarang')}</button>
								<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-[11px]" type="button" style={quickBtnStyle(quickPick[m.media_id] === '1h')} onclick={() => setMediaSchedule(m.media_id, '1h')}>+1 jam</button>
								<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-[11px]" type="button" style={quickBtnStyle(quickPick[m.media_id] === '24h')} onclick={() => setMediaSchedule(m.media_id, '24h')}>+24 jam</button>
								<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-[11px]" type="button" style={quickBtnStyle(quickPick[m.media_id] === '3d')} onclick={() => setMediaSchedule(m.media_id, '3d')}>+3 hari</button>
							</div>
							<button
								type="button"
								class="mt-1.5 inline-flex w-full items-center justify-between gap-1.5 rounded-lg border px-2.5 py-1.5 text-[11.5px] font-semibold transition-colors {explicit
									? 'border-indigo-200 bg-indigo-50 text-indigo-700 hover:bg-indigo-100'
									: 'border-slate-200 bg-white text-slate-600 hover:bg-slate-50'}"
								aria-label={`${tr('Pilih tanggal terbit')} ${m.name}`}
								onclick={() => openSchedulePicker(m)}
							>
								<span class="inline-flex min-w-0 items-center gap-1.5">
									<CalendarClock size={12} class="shrink-0" />
									<span class="truncate">{explicit ? toJakartaDisplay(explicit) : tr('Pilih tanggal terbit…')}</span>
								</span>
								{#if explicit}
									<span
										role="button"
										tabindex="0"
										class="shrink-0 rounded p-0.5 text-slate-400 transition-colors hover:bg-white/70 hover:text-slate-600"
										aria-label={tr('Batalkan jadwal')}
										title={tr('Batalkan jadwal — terbit sekarang')}
										onclick={(e) => { e.stopPropagation(); setMediaSchedule(m.media_id, 'now'); }}
										onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); e.stopPropagation(); setMediaSchedule(m.media_id, 'now'); } }}
									>
										<X size={11} />
									</span>
								{/if}
							</button>
						</div>
					{/each}
				</div>
			</div>

		</div>

		{#snippet footer()}
			<!-- Pemeriksaan cover + ringkasan biaya ditaruh di kaki panel: bagian ini
			     tidak ikut tergulir, jadi selalu terlihat tepat di atas tombol. -->
			<div class="w-full space-y-2">
				<div class="rounded-[5px] border p-3">
					<p class="text-[11px] font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('Pemeriksaan Cover')}</p>
					{#if missingCovers.length === 0}
						<p class="mt-1.5 flex items-center gap-1.5 text-sm" style="color: var(--success);"><CircleCheck size={14} /> {tr('Semua artikel sudah punya cover.')}</p>
					{:else}
						<p class="mt-1.5 text-sm" style="color: var(--warning);">{missingCovers.length} {tr('artikel belum punya cover')}: {missingCovers.map((m) => m.name).join(', ')}.</p>
						<p class="mt-1 text-[11px] text-muted-foreground">{tr('Tambahkan cover lewat Edit Artikel, atau pakai thumbnail default Tauthub.')}</p>
						<button class="app-btn app-btn-soft mt-2 !min-h-8 !px-3 !text-xs" type="button" disabled={coverBusy} onclick={() => void useDefaultCovers()}>
							{#if coverBusy}<LoaderCircle size={14} class="animate-spin" />{:else}<Image size={14} />{/if}
							{coverBusy ? tr('Menerapkan…') : tr('Pakai thumbnail default Tauthub')}
						</button>
					{/if}
				</div>

				<div class="rounded-[5px] border p-3 text-sm">
					<div class="flex items-center justify-between gap-2"><span class="text-muted-foreground">{tr('Media terpilih')}</span><span class="font-semibold">{cost.count} {tr('Media')}</span></div>
					<div class="mt-1 flex items-center justify-between gap-2"><span class="text-muted-foreground">{tr('Waktu terbit')}</span><span class="font-medium">{cost.count - scheduledCount} {tr('langsung')} · {scheduledCount} {tr('terjadwal')}</span></div>
					<div class="mt-1 flex items-center justify-between gap-2"><span class="text-muted-foreground">{tr('Total tagihan')}</span><span class="font-bold">{toRp(cost.total)}</span></div>
				</div>

				{#if publishError}<p class="text-sm" style="color: var(--destructive);">{publishError}</p>{/if}

				<div class="flex flex-wrap justify-end gap-2">
					<button class="app-btn app-btn-soft !min-h-9 !px-3.5 !text-xs" type="button" onclick={() => (publishOpen = false)}>{tr('Batal')}</button>
					<button class="app-btn !min-h-9 !px-3.5 !text-xs" type="button" style="background-color: var(--success); color: #fff;" disabled={distributing || missingCovers.length > 0 || coverBusy} onclick={() => void confirmPublish()}>
						{#if distributing}<LoaderCircle size={14} class="animate-spin" />{:else if scheduledCount > 0}<CalendarClock size={14} />{:else}<Send size={14} />{/if}
						{distributing ? tr('Menerbitkan…') : scheduledCount > 0 ? tr('Terbitkan & Jadwalkan') : tr('Terbitkan Sekarang')}
					</button>
				</div>
			</div>
		{/snippet}
	</SlideOver>

{/if}

{#if qcItem}
	{@const qc = qcItem}
	{#key qc.media_id}
		<QcPanel
			{apiBase}
			mediaId={qc.media_id}
			mediaName={qc.name}
			articleStatus={articleStatusOf(qc)}
			words={qcStats?.words ?? null}
			regenLabel={`${tr('Generate Ulang')} (${toRp(writerCostCredits)})`}
			canRegen={canRegen(qc) && !bulkRunning}
			generating={articleStatusOf(qc) === 'generating'}
			gatekeeperConfigured={gatekeeperAgent ? gatekeeperAgent.configured : null}
			reloadToken={qcToken}
			onClose={() => (qcFor = null)}
			onEdit={() => void openPreview(qc)}
			onRegenerate={() => void generateOne(qc, false)}
				onScanned={() => {
					qcToken += 1;
					// Setelah pemindaian selesai: bila seluruh gerbang keras kini lulus,
					// panel QC ditutup dan panel penerbitan langsung terbuka — tidak
					// perlu menutup manual lalu menekan tombol di bar bawah.
					void (async () => {
						await load();
						if (!viewOnly && !editor && distributable) {
							qcFor = null;
							void publish();
						}
					})();
				}}
			/>
		{/key}
	{/if}

	<!-- Popup placeholder untuk "Generate Semua" (bulk): mesinnya menyusul. -->
	{#if bulkPopupOpen}
		<div class="fixed inset-0 z-[70] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation" onclick={() => (bulkPopupOpen = false)} onkeydown={(e) => { if (e.key === 'Escape') bulkPopupOpen = false; }}>
			<dialog open class="app-card w-full max-w-sm p-6 text-center" aria-label={tr('Generate Semua')} onclick={(e) => e.stopPropagation()}>
				<div class="mx-auto flex h-14 w-14 items-center justify-center rounded-full border" style="background: var(--warning-soft, #fdf3e3); border-color: color-mix(in srgb, var(--warning, #d97706) 35%, transparent); color: var(--warning, #b45309);">
					<Wrench size={24} strokeWidth={2} />
				</div>
				<h3 class="mt-3 text-base font-bold">{tr('Generate Semua')}</h3>
				<p class="mt-1.5 text-sm text-muted-foreground">{tr('Fitur generate artikel di semua media terpilih masih dalam pengerjaan.')}</p>
				<button class="app-btn app-btn-soft mt-4 !min-h-9 !px-4" type="button" onclick={() => (bulkPopupOpen = false)}>{tr('Tutup')}</button>
			</dialog>
		</div>
	{/if}

	{#if schedulePickerFor}
		<DateTimePicker
			open
			title={`${tr('Jadwal terbit')} — ${schedulePickerFor.name}`}
			value={schedulePickerInitial}
			min={new Date()}
			onClose={() => (schedulePickerFor = null)}
			onApply={applySchedulePicker}
		/>
	{/if}

	<style>
		/* Bar biaya bawah (referensi style dari katalog media):
		   Stat item rapi dengan pembatas vertikal tipis, tombol hitam untuk aksi utama,
		   dan membentang penuh (full-width) mengikuti area konten. */

		.selectedBarLeftCol {
			display: flex;
			flex-direction: column;
			gap: 3px;
			min-width: 0;
		}

		.selectedStatItem {
			display: flex;
			flex-direction: column;
			gap: 3px;
		}
		.selectedStatLabel {
			font-size: 10.5px;
			font-weight: 700;
			text-transform: uppercase;
			letter-spacing: 0.05em;
			color: #94a3b8;
			white-space: nowrap;
		}
		.selectedStatPriceVal {
			font-size: 17.5px;
			font-weight: 750;
			color: #0f172a;
			letter-spacing: -0.01em;
			line-height: 1.2;
			font-variant-numeric: tabular-nums;
		}
		/* Total adalah angka yang harus terbaca pertama: diberi blok hijau. */
		.costPopover {
			position: absolute;
			bottom: calc(100% + 10px);
			left: 0;
			z-index: 40;
			width: 300px;
			padding: 12px 14px;
			border-radius: 5px;
			border: 1px solid var(--border);
			background: var(--card);
			box-shadow: 0 16px 40px rgba(15, 23, 42, 0.18);
		}
		/* Popover prasyarat terbit: menempel di atas tombol "Lanjut Tinjau & Terbit". */
		.readyPopover {
			position: absolute;
			bottom: calc(100% + 10px);
			right: 0;
			z-index: 40;
			width: min(380px, calc(100vw - 28px));
			padding: 14px;
			border-radius: 10px;
			border: 1px solid var(--border);
			background: var(--card);
			box-shadow: 0 18px 45px rgba(15, 23, 42, 0.2);
			animation: readyPopIn 180ms ease-out;
		}
		.readyChip {
			display: inline-flex;
			align-items: center;
			max-width: 100%;
			padding: 2px 8px;
			border-radius: 999px;
			background: #f1f5f9;
			color: #475569;
			font-size: 11px;
			font-weight: 600;
			white-space: nowrap;
			overflow: hidden;
			text-overflow: ellipsis;
		}
		.readyProgress {
			height: 6px;
			border-radius: 999px;
			background: #e2e8f0;
			overflow: hidden;
		}
		.readyProgress span {
			display: block;
			width: 40%;
			height: 100%;
			border-radius: 999px;
			background: linear-gradient(90deg, #2563eb, #93c5fd, #2563eb);
			animation: readySweep 1.1s linear infinite;
		}
		@keyframes readyPopIn {
			from { opacity: 0; transform: translateY(8px) scale(0.98); }
			to { opacity: 1; transform: translateY(0) scale(1); }
		}
		@keyframes readySweep {
			from { transform: translateX(-110%); }
			to { transform: translateX(300%); }
		}
		@media (prefers-reduced-motion: reduce) {
			.readyPopover { animation: none; }
			.readyProgress span { animation-duration: 3s; }
		}
		.costPopoverTitle {
			font-size: 10.5px;
			font-weight: 700;
			text-transform: uppercase;
			letter-spacing: 0.05em;
			color: var(--muted-foreground);
		}
		.costPopoverRow {
			display: flex;
			align-items: baseline;
			justify-content: space-between;
			gap: 10px;
			margin-top: 9px;
			font-size: 12.5px;
		}
		.costPopoverRowNote {
			display: block;
			font-size: 10.5px;
			color: var(--muted-foreground);
		}
		.costPopoverDivider {
			height: 1px;
			margin-top: 11px;
			background: var(--border);
		}
		.costPopoverTotal {
			display: flex;
			align-items: baseline;
			justify-content: space-between;
			gap: 10px;
			margin-top: 10px;
			font-size: 13.5px;
			font-weight: 700;
			color: var(--success);
		}

		/* Chip delta tagihan: memunculkan selisih (+ hijau / - merah) yang naik
		   dari bawah angka total lalu memudar. Tujuan animasinya: memancing mata
		   ke angka yang baru saja berubah setelah suatu aksi (R-19). */

		/* Tombol aksi card: selalu ikon (32px), tidak mengembang. Saat hover/
		   fokus, tooltip muncul di atas tombol dengan animasi halus. */
		.expAction {
			box-sizing: border-box;
			position: relative;
		}
		.expTip {
			position: absolute;
			bottom: calc(100% + 8px);
			/* Anchor ke kanan tombol: tooltip memanjang ke kiri, jadi tidak
			   menambah lebar dokumen saat tombol berada di tepi kanan. */
			right: 0;
			left: auto;
			transform: translateY(4px) scale(0.96);
			transform-origin: right bottom;
			padding: 5px 9px;
			border-radius: 5px;
			background: #0f172a;
			color: #ffffff;
			font-size: 11px;
			font-weight: 600;
			line-height: 1.4;
			white-space: nowrap;
			opacity: 0;
			pointer-events: none;
			z-index: 20;
			transition: opacity 0.18s ease, transform 0.18s ease;
		}
		.expTip::after {
			content: '';
			position: absolute;
			top: 100%;
			right: 10px;
			transform: none;
			border: 4px solid transparent;
			border-top-color: #0f172a;
		}
		.expAction:hover .expTip,
		.expAction:focus-visible .expTip {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
		@media (prefers-reduced-motion: reduce) {
			.expTip { transition: none; }
		}

		.billDeltaWrap {
			position: absolute;
			left: 50%;
			bottom: calc(100% + 10px);
			transform: translateX(-50%);
			pointer-events: none;
			z-index: 5;
		}
		.billDelta {
			display: inline-flex;
			padding: 3px 9px;
			border-radius: 5px;
			font-size: 12.5px;
			font-weight: 800;
			white-space: nowrap;
			color: var(--success);
			background-color: var(--success-soft, #e8f5ee);
			box-shadow: 0 6px 18px rgba(15, 23, 42, 0.14);
			animation: bill-delta-rise 1.6s ease-out forwards;
		}
		.billDelta.down {
			color: var(--destructive);
			background-color: var(--destructive-soft, #fdecee);
		}
		@keyframes bill-delta-rise {
			0% { opacity: 0; transform: translateY(12px); }
			18% { opacity: 1; transform: translateY(0); }
			65% { opacity: 1; transform: translateY(-7px); }
			100% { opacity: 0; transform: translateY(-16px); }
		}
		@media (prefers-reduced-motion: reduce) {
			/* Tanpa gerakan: chip tetap tampil-teruntai khas animasi langsung
			   naik-turun, cukup memudar pelan di tempat. */
			.billDelta { animation-name: bill-delta-fade; }
			@keyframes bill-delta-fade {
				0% { opacity: 0; }
				20% { opacity: 1; }
				70% { opacity: 1; }
				100% { opacity: 0; }
			}
		}
		.selectedStatTotalVal {
			font-size: 19px;
			font-weight: 800;
			color: var(--success);
		}
		.selectedBarRight {
			display: flex;
			flex-direction: column;
			align-items: flex-end;
			gap: 6px;
		}
		.writerQcNotice {
			max-width: 380px;
			text-align: right;
			font-size: 11px;
			line-height: 1.35;
		}
		.writerActionBtns {
			display: flex;
			align-items: center;
			gap: 10px;
		}
		.selectedCancelBtn {
			background: #ffffff;
			border: 1px solid #e2e8f0;
			color: #334155;
			font-size: 13px;
			font-weight: 500;
			padding: 8px 18px;
			min-height: 40px;
			border-radius: 5px;
			cursor: pointer;
			outline: none;
			transition: all 0.12s ease;
			white-space: nowrap;
			display: inline-flex;
			align-items: center;
			justify-content: center;
		}
		.selectedCancelBtn:hover {
			background: #f8fafc;
			border-color: #cbd5e1;
			color: #0f172a;
		}
		.selectedCreateBtn {
			background: #0f172a;
			color: #ffffff;
			font-size: 13px;
			font-weight: 600;
			padding: 9px 20px;
			min-height: 40px;
			border-radius: 5px;
			border: none;
			cursor: pointer;
			outline: none;
			transition: all 0.15s ease;
			display: inline-flex;
			align-items: center;
			gap: 6px;
			white-space: nowrap;
		}
		.selectedCreateBtn:hover:not(:disabled) {
			background: #1e293b;
			transform: translateY(-1px);
		}
		.selectedCreateBtn:disabled {
			opacity: 0.5;
			cursor: not-allowed;
			transform: none;
		}

		/* Kolom kiri bar: total tagihan + popover rinciannya. */
		.selectedBarLeftCol {
			display: flex;
			flex-direction: column;
			gap: 3px;
			min-width: 0;
		}

		@media (max-width: 767px) {
			.selectedBarRight {
				width: 100%;
				align-items: stretch;
			}
			.writerQcNotice {
				text-align: left;
				max-width: none;
			}
			.writerActionBtns {
				width: 100%;
			}
			.selectedCancelBtn {
				flex: 1;
				min-height: 44px;
			}
			.selectedCreateBtn {
				flex: 2;
				justify-content: center;
				min-height: 44px;
			}
		}
	</style>
