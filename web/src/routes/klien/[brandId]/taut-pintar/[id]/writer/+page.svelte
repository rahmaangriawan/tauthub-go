<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Agent 3: The Writer — dapur produksi artikel per media terpilih.
	// Port app/klien/[brandId]/taut-pintar/[id]/writer/client.tsx.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { CalendarClock, CircleAlert, CircleCheck, Clock, Eye, FileText, Image, Link, Link2, ListChecks, LoaderCircle, Pencil, PencilLine, Send, ShieldCheck, Sparkles, Trash2, TriangleAlert, Upload, Wallet, X } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, getAccessToken, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import AgentSteps from '../../_agent-steps.svelte';
	import TautBottomBar from '../../_bottom-bar.svelte';
	import NaskahModal from './_naskah-modal.svelte';
	import QcPanel from './_qc-panel.svelte';
	import { MANUAL_PROFILER_REASON, buildTrackedUrl, stripScheme, withScheme, textToHtml, toLocalInput, isCampaignDone } from '../../_shared';
	import { tcToRp } from '$lib/credits/rupiah';
	import { ARTICLE_DEFAULT_COVER } from '$lib/media/cover';
	import { mediaDomainLabel } from '$lib/media/domain-label';
	import PageTitle from '$lib/components/page-title.svelte';
	import SlideOver from '$lib/components/slide-over.svelte';

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
	};
	type LinksForm = {
		tracking_mode: 'direct' | 'tauthub';
		primary_url: string;
		primary_anchor: string;
		backup_url: string;
		backup_anchor: string;
	};
	type LinkCounts = { dofollow: number; nofollow: number; sponsored: number };
	type ArticleStats = { words: number; images: number; links: number; cover: string | null };

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
	type WriterAngle = { slug: string; name: string; description?: string | null; has_prompt: boolean };
	type WriterChannel = { slug: string; name: string; angles: WriterAngle[] };
	let angleChannels = $state<WriterChannel[]>([]);
	let anglesLoading = $state(true);
	let anglesError = $state('');
	// Jumlah analisis QC per media (badge di tombol "Analisis QC").
	let qcCounts = $state<Record<string, number>>({});
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
	// Tarif penulisan dari server (platform_settings) — satu angka untuk semua aksi.
	let writerCostCredits = $state(2);
	// Tarif per gambar dari konfigurasi AI — dipakai bar dan panel Biaya Gambar.
	let imageCreditCost = $state(2);
	let trackingOn = $state(false);
	let genState = $state<Record<string, string>>({});
	let bulkRunning = $state(false);
	let bulkProgress = $state<{ done: number; total: number } | null>(null);
	let uploadBusyId = $state<string | null>(null);
	let checklistFor = $state<string | null>(null);
	let coverChecking = $state(false);
	let agentStatus = $state<{ configured: boolean; reason: string | null } | null>(null);
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
	} | null>(null);
	let editorSaving = $state(false);
	let editorError = $state('');
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
				articleStats = { ...articleStats, [item.media_id]: { words, images: imgs.length, links: tags.length, cover: d.featured_image_url ?? null } };
				if (d.title?.trim()) articleTitles = { ...articleTitles, [item.media_id]: d.title.trim() };
			} catch {
				/* fallback badge */
			}
		}
	}

	function articleStatusOf(item: PlacementItem): string {
		return genState[item.media_id] ?? item.article_status;
	}
	function hasDoc(item: PlacementItem): boolean {
		const s = articleStatusOf(item);
		return s !== 'none' && s !== 'generating' && s !== 'error' && s !== 'queued';
	}

	/** Rincian rel link untuk badge "N Dofollow - N Sponsored · N Nofollow". */
	function linkStatsTail(links: LinkCounts): string {
		return [
			...(links.sponsored > 0 ? [`${links.sponsored} Sponsored`] : []),
			...(links.nofollow > 0 || links.sponsored === 0 ? [`${links.nofollow} Nofollow`] : [])
		].join(' · ');
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
				linkLimit
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
				linkLimit
			};
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal membuka artikel.');
		}
	}

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
			linkLimit: upLimit
		};
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
			linkLimit: genLimit
		};
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
			judul = lines[0];
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
	async function generateNaskah(
		mediaId: string,
		topic: string,
		links: { anchor_text: string; url: string }[] = [],
		angleSlug = ''
	): Promise<{ title: string; html: string; imagesGenerated?: number; imagesCostCredits?: number }> {
		const mediaType = placement.find((p) => p.media_id === mediaId)?.media_type;
		const angle = angleChannels.find((ch) => ch.slug === mediaType)?.angles.find((a) => a.slug === angleSlug && a.has_prompt);
		if (anglesLoading || anglesError || !angle) throw new Error(tr('Pilih angle artikel yang tersedia untuk media ini.'));
		const res = await apiRequest(`${apiBase}/produce/${mediaId}`, {
			method: 'POST',
			body: JSON.stringify({ topic, links, angle_slug: angleSlug })
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
			await load();
			return true;
		} catch (reason) {
			editorError = reason instanceof Error ? reason.message : tr('Gagal menyimpan artikel.');
			return false;
		}
	}

	async function saveEditor(title: string, html: string, coverUrl: string | null, category: string, metaDescription: string) {
		if (viewOnly) { error = tr(VIEW_ONLY_MSG); return; }
		if (!editor) return;
		editorSaving = true;
		const saved = await persistEditor(title, html, coverUrl, category, metaDescription);
		editorSaving = false;
		if (saved) {
			editor = null;
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
		// Penulisan ditagih PER generate dengan tarif rata dari server: artikel
		// baru 1× tarif dan tiap regenerasi menambah 1× lagi (ledger backend:
		// taut-writer / taut-writer-regen). Jadi biaya penulisan = jumlah
		// penulisan NYATA; media yang belum punya artikel diproyeksikan 1×.
		const writingUnits = selected.reduce((s, m) => s + (hasDoc(m) ? 1 + regenCountOf(m) : 1), 0);
		const regens = selected.reduce((s, m) => s + (hasDoc(m) ? regenCountOf(m) : 0), 0);
		const writing = writingUnits * writerCostCredits;
		// Biaya media memakai harga penempatan (dikonversi server), bukan
		// credit_cost yang bukan sumber penagihan — dulu itu sebab bar menulis
		// Rp 22.000 padahal yang ditagih Rp 20.000.
		const media = selected.reduce((s, m) => s + (m.cost_tc ?? 0), 0);
		const imageCount = selected.reduce((s, m) => s + (m.images_count ?? 0), 0);
		const images = imageCount * imageCreditCost;
		return { count: selected.length, writing, writingUnits, regens, media, images, imageCount, total: writing + media + images };
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

	const editorMedia = $derived.by(() => {
		const current = editor;
		if (!current) return null;
		return selected.find((m) => m.media_id === current.mediaId) ?? null;
	});

	async function publish() {
		if (viewOnly) { error = tr(VIEW_ONLY_MSG); return; }
		if (coverChecking || distributing) return;
		// QC belum lulus → buka panel QC agar alasan terlihat (halaman Gatekeeper dihapus).
		if (!distributable) {
			openQcPanel();
			return;
		}
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
	async function useDefaultCovers() {
		if (missingCovers.length === 0 || coverBusy) return;
		coverBusy = true;
		publishError = '';
		try {
			for (const item of missingCovers) {
				const res = await apiRequest(`${apiBase}/articles/${item.mediaId}`, {
					method: 'PUT',
					body: JSON.stringify({ featured_image_url: ARTICLE_DEFAULT_COVER })
				});
				if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan cover default.')));
			}
			missingCovers = [];
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
	const checkMediaType = $derived(checklistItem ? ((checklistItem as unknown as { media_type?: string }).media_type ?? null) : null);

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
								: tr('MENUNGGU ARTIKEL');
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

<main>
	{#if viewOnly}
		<div class="mb-3 flex flex-wrap items-center gap-2 rounded-xl border px-4 py-2.5 text-[13px]" style="border-color: var(--border); background: var(--muted);">
			<span>{tr('Kampanye ini sudah selesai — mode lihat saja. Isinya tidak bisa diubah lagi.')}</span>
			<a class="font-semibold underline" href={`/klien/${brandUrl}/taut-pintar/${campaignId}/ringkasan`}>{tr('Buka Ringkasan')}</a>
		</div>
	{/if}
	<div class="flex flex-wrap items-center justify-between gap-3">
		<div class="flex items-center gap-3">
			<div class="flex h-10 w-10 items-center justify-center rounded-xl" style="background-color: var(--primary);">
				<Pencil size={22} style="color: #fff;" />
			</div>
			<div>
				<p class="font-bold leading-tight">Agent 3: Penyedia Konten</p>
				<p class="text-sm leading-tight text-muted-foreground">{manualFlow ? tr('Dapur Produksi') : tr('Dapur Produksi AI')} ({cost.count} {tr('media terpilih')})</p>
			</div>
		</div>
		<div class="flex flex-wrap items-center justify-end gap-3">
			{#if campaignName}
				<span class="inline-flex max-w-[260px] items-center gap-1.5 truncate rounded-full border px-3.5 py-1.5 text-[12.5px] font-semibold" style="background-color: var(--muted);" title={campaignName}><FileText size={13} />{campaignName}</span>
			{/if}
			<span class="inline-flex items-center gap-1.5 rounded-full px-3.5 py-1.5 text-[13px] font-bold" style="background-color: var(--success-soft); color: var(--success);" title={tr('Saldo Anda')}>{toRp(wallet)}</span>
		</div>
	</div>

	<div class="mt-3">
		<AgentSteps current={2} />
	</div>

	{#if spokepersons.length > 0}
		<div class="mt-3 rounded-[10px] border p-3" style="background-color: var(--muted);">
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
						<span class="flex h-7 w-7 items-center justify-center rounded-lg" style="background-color: var(--accent); color: var(--accent-foreground);"><Link size={18} /></span>
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
						class="rounded-xl border p-3 text-left"
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
							class="rounded-xl border p-3 text-left"
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
					<div class="rounded-xl border p-3" style="border-color: var(--primary);">
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

					<div class="rounded-xl border p-3">
						<div class="flex items-center justify-between">
							<p class="flex items-center gap-2 text-sm font-bold text-muted-foreground"><Link2 size={16} /> {tr('Tautan Cadangan (Prioritas 2)')}</p>
							<span class="rounded-full px-2 py-0.5 text-[10px] font-semibold" style={badge('muted')}>{tr("Opsional")}</span>
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

		<section class="app-card mt-3 p-4" aria-label={tr('Dapur produksi AI')}>
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div class="flex items-center gap-2">
					<span class="flex h-7 w-7 items-center justify-center rounded-lg" style="background-color: var(--accent); color: var(--accent-foreground);"><FileText size={16} /></span>
					<div>
						<p class="font-bold">{manualFlow ? tr('Dapur Produksi') : tr('Dapur Produksi AI')} ({cost.count} {tr('Media Terpilih')})</p>
						<p class="text-xs text-muted-foreground">
							{#if manualFlow}
								{tr('Unggah artikel tulisanmu sendiri, atau buat dengan AI lewat tombol di tiap media.')}
							{:else}
								{tr('Biaya penulisan konten organik oleh The Writer:')} <strong>{toRp(writerCostCredits)}</strong> {tr('/ artikel — baru maupun tulis ulang.')}
							{/if}
						</p>
					</div>
				</div>
			</div>

			{#if selected.length === 0}
				<div class="mt-3 rounded-lg border px-4 py-8 text-center text-sm text-muted-foreground">{tr("Belum ada media terpilih. Kembali ke Matchmaker untuk memilih media.")}</div>
			{:else}
				<div class="mt-3 space-y-2">
					{#each selected as m (m.media_id)}
						{@const st = articleStatusOf(m)}
						{@const statusKind = st === 'error' ? 'none' : st}
						{@const generating = st === 'generating'}
						{@const queued = st === 'queued'}
						{@const links2 = linkStats[m.media_id] ?? null}
						<div class="flex flex-wrap items-center justify-between gap-4 rounded-xl border px-5 py-4">
							<div class="flex min-w-0 items-center gap-3">
								<div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full" style="background-color: var(--muted);">
									<FileText size={18} class="text-muted-foreground" />
								</div>
								<div class="min-w-0">
									<div class="flex min-w-0 flex-wrap items-center gap-1.5">
										<p class="font-bold leading-tight">{mediaDomainLabel(m.website_url, m.name)}</p>
										{#if m.media_type}
											<span
												class="shrink-0 rounded-full px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wide"
												style="background: {m.media_type === 'blogger' ? 'var(--warning-soft)' : 'var(--accent)'}; color: {m.media_type === 'blogger' ? 'var(--warning)' : 'var(--primary)'};"
											>{m.media_type === 'blogger' ? tr('Blogger') : tr('Mainstream')}</span>
										{/if}
									</div>
									<div class="mt-1 flex flex-wrap items-center gap-2">
										<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style={badge(statusTone(statusKind))}>
											{#if statusKind === 'generating'}<LoaderCircle size={11} class="animate-spin" />
											{:else if statusKind === 'queued'}<Clock size={11} />
											{:else if statusKind === 'draft'}<FileText size={11} />
											{:else if statusKind === 'ready'}<CircleCheck size={11} />
											{:else if statusKind === 'blocked'}<X size={11} />
											{:else}<Clock size={11} />{/if}
											{statusText(statusKind)}
										</span>
										{#if !mediaAllowsLink(m)}
											<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style={badge('warning')}>{tr("Tidak Tersedia Backlink")}</span>
										{:else if links2}
											<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style={badge('warning')}>
												<PencilLine size={11} /> {links2.dofollow} Dofollow - {linkStatsTail(links2)}
											</span>
										{:else}
											<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style={badge('warning')}>
												<PencilLine size={11} /> {m.label_sponsor === 'ada_sponsor' ? 'Sponsored' : 'Dofollow'} · Maks. {m.max_links != null && m.max_links > 0 ? m.max_links : 1} Link
											</span>
										{/if}
									</div>
								</div>
							</div>
							{#if articleTitles[m.media_id]}
								<div class="min-w-0 flex-1 basis-[220px]">
									<p class="text-[11px] text-muted-foreground">{tr('Judul Artikel')}</p>
									<p class="line-clamp-2 text-[13px] font-semibold leading-snug" title={articleTitles[m.media_id]}>{articleTitles[m.media_id]}</p>
								</div>
							{/if}
							<p class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
								<Image size={12} class="shrink-0" />
								{#if hasDoc(m) && articleStats[m.media_id]}
									<span style="color: {articleStats[m.media_id].cover ? 'var(--success)' : 'var(--destructive)'}; font-weight: 600;">{tr('Cover')} {articleStats[m.media_id].cover ? tr('ada') : tr('tidak ada')}</span>
									<span>·</span><span>{articleStats[m.media_id].images} {tr('gambar di artikel')}</span>
									<span>·</span><span>{articleStats[m.media_id].words.toLocaleString('id-ID')} {tr('kata')}</span>
								{:else}
									<span>{tr('Gambar artikel: tidak ada')}</span>
								{/if}
							</p>
							<div class="flex flex-wrap gap-2">
									<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={bulkRunning || viewOnly} onclick={() => (checklistFor = m.media_id)}><ListChecks size={14} /> {tr('Cek Validasi')}</button>
									{#if removeTarget && removeTarget.media_id === m.media_id}
										<div class="fixed inset-0 z-[70] flex items-center justify-center p-4" style="background-color: rgba(15, 23, 42, 0.55);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget && !removeBusy) removeTarget = null; }}>
											<div class="flex w-full max-w-md flex-col overflow-hidden rounded-2xl border shadow-2xl" style="border-color: var(--border); background-color: var(--card, #fff);" role="dialog" aria-modal="true" tabindex="-1" aria-label={tr('Hapus Media dari Kampanye')} onclick={(e) => e.stopPropagation()} onkeydown={(e) => { if (e.key === 'Escape' && !removeBusy) removeTarget = null; }}>
												<div class="flex items-start justify-between gap-3 border-b px-5 py-4" style="border-color: var(--border);">
													<div class="flex items-center gap-2.5 text-sm font-bold" style="color: var(--destructive);">
														<Trash2 size={18} />
														<span>{tr('Hapus Media dari Kampanye?')}</span>
													</div>
													<button class="rounded-lg p-1.5 text-muted-foreground hover:bg-secondary" type="button" aria-label={tr('Tutup')} disabled={removeBusy} onclick={() => (removeTarget = null)}>
														<X size={16} />
													</button>
												</div>
												<div class="space-y-3 p-5">
													<p class="text-sm font-semibold">{m.name}</p>
													<p class="text-xs leading-relaxed text-muted-foreground">
														{tr('Media ini akan dilepas dari kampanye (tidak dihapus permanen). Artikel dan datanya tetap tersimpan, jadi bisa dipakai lagi nanti.')}
													</p>
													<p class="rounded-lg p-2.5 text-xs leading-relaxed" style="background-color: var(--muted, #f8fafc); color: var(--foreground);">
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
															const remaining = placement.filter((p) => p.media_id !== removeTarget?.media_id).map((p) => p.media_id);
															const res = await apiRequest(`${apiBase}/placement`, { method: 'PUT', body: JSON.stringify({ selected: remaining }) });
															if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menghapus media dari kampanye.')));
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
									<!-- Generate AI ditampilkan juga di mode manual: generate lewat
									     Pengelola Naskah memang sudah bisa, jadi tombolnya tidak
									     disembunyikan (kampanye lama/katalog sering ber-mode manual). -->
									{#if !hasDoc(m)}
										<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={generating || bulkRunning || viewOnly} onclick={() => openNaskahGenerate(m)}>
											{#if generating}<LoaderCircle size={14} class="animate-spin" />{:else}<Sparkles size={14} />{/if}
											{generating ? tr('Membuat…') : queued ? tr('Mengantre…') : tr('Generate Konten')}
										</button>
									{/if}
								{#if !hasDoc(m)}
									<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={bulkRunning || viewOnly} onclick={() => openNaskahUpload(m)}>
										<Upload size={14} /> {tr('Unggah Artikel')}
									</button>
								{/if}
								{#if hasDoc(m)}
									<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={bulkRunning} onclick={() => void openPreview(m)}><Eye size={14} /> {tr('Edit Artikel')}</button>
										{@const qcCount = qcCounts[m.media_id] ?? 0}
										<button
											class="app-btn !min-h-8 !px-3 !text-xs"
											type="button"
											disabled={bulkRunning}
											style={qcCount > 0 ? 'background-color: var(--accent); color: var(--accent-foreground);' : ''}
											onclick={() => (qcFor = m.media_id)}
										>
											<ShieldCheck size={14} /> {tr('Analisis QC')}
											<span
												class="rounded-full px-1.5 py-0.5 text-[10px] font-bold"
												style={qcCount > 0 ? 'background-color: var(--primary); color: #fff;' : 'background-color: var(--muted); color: var(--muted-foreground);'}
											>{qcCount > 0 ? qcCount : tr('Belum ada')}</span>
										</button>
									{/if}
									<button
										class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border text-muted-foreground transition-colors hover:border-destructive hover:bg-destructive hover:text-white focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-1"
										style="border-color: var(--border);"
										type="button"
										disabled={bulkRunning || removeBusy || viewOnly}
										aria-label={tr('Hapus media dari kampanye')}
										title={tr('Hapus media dari kampanye')}
										onclick={() => { removeError = ''; removeTarget = m; }}
									>
										<Trash2 size={14} />
									</button>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</section>

			<TautBottomBar fullWidth>
				<div class="selectedBarLeftCol">
					<!-- Bar hanya menampilkan TOTAL. Rinciannya muncul sebagai popover
					     ke atas saat di-hover atau diklik. -->
					<span class="relative inline-flex">
						<button
							class="selectedStatItem selectedStatItemTotal"
							type="button"
							aria-expanded={costPopoverOpen}
							onmouseenter={() => (costPopoverOpen = true)}
							onmouseleave={() => { if (!costPinned) costPopoverOpen = false; }}
							onfocus={() => (costPopoverOpen = true)}
							onblur={() => { if (!costPinned) costPopoverOpen = false; }}
							onclick={() => { costPinned = !costPinned; costPopoverOpen = costPinned; }}
						>
							<span class="selectedStatLabel flex items-center gap-1">
								{tr('TOTAL TAGIHAN AKHIR')}
								<Wallet size={12} class="opacity-70" />
							</span>
							<span class="selectedStatPriceVal selectedStatTotalVal">{toRp(cost.total)}</span>
						</button>

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
										<span class="text-muted-foreground">({cost.writingUnits - cost.regens} {tr('artikel')}{cost.regens > 0 ? ` + ${cost.regens} ${tr('tulis ulang')}` : ''})</span>
										<span class="costPopoverRowNote">{cost.writingUnits} × {toRp(writerCostCredits)}</span>
									</span>
									<span class="font-medium">{toRp(cost.writing)}</span>
								</div>

								<div class="costPopoverRow">
									<span>
										{tr('Biaya gambar')} <span class="text-muted-foreground">({cost.imageCount} {tr('gambar')})</span>
										<span class="costPopoverRowNote">{cost.imageCount} × {toRp(imageCreditCost)}</span>
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
					</span>
				</div>
				<div class="selectedBarRight">
					{#if distributable}
						<!-- Lebarnya dibatasi supaya blok tombol tidak melebar sampai memaksa
						     bar melipat dan tombolnya turun ke baris kedua. -->
						<p class="writerQcNotice text-muted-foreground">{tr('QC sudah lulus — tombol ini langsung menerbitkan kampanye dan membuka Ringkasan.')}</p>
					{:else if distributable === false}
						<p class="writerQcNotice" style="color: var(--warning, #b45309);">{tr('QC belum lulus. Jalankan Cek Validasi sekarang')}</p>
					{/if}
					<div class="writerActionBtns">
						<button class="selectedCancelBtn" type="button" onclick={() => goto(`/klien/${brandUrl}/taut-pintar`)}>{tr('Kembali')}</button>
						<button class="selectedCreateBtn" type="button" disabled={coverChecking || distributing || viewOnly} title={viewOnly ? tr(VIEW_ONLY_MSG) : undefined} onclick={publish}>
							{#if coverChecking || distributing}<LoaderCircle size={15} class="animate-spin" />{:else}<Send size={15} />{/if}
							<span>{coverChecking ? tr('Memeriksa cover…') : distributing ? tr('Menerbitkan…') : tr('Lanjut Tinjau & Terbit')}</span>
						</button>
					</div>
				</div>
			</TautBottomBar>
	{/if}
</main>

{#if checklistItem}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<div class="app-card w-full max-w-md p-5">
			<p class="flex items-center gap-2 font-bold" style="color: var(--primary);"><ListChecks size={18} /> {tr('Cek Validasi Artikel')}</p>
			<p class="mt-1 text-sm text-muted-foreground">{tr("Ringkasan indikator artikel untuk media")} {checklistItem.name}.</p>
			<div class="mt-3 space-y-2 text-sm">
				<div class="flex items-start justify-between gap-3"><span class="text-muted-foreground">{tr("Status artikel")}</span><span class="text-right font-medium">{articleStatusOf(checklistItem)}</span></div>
				<div class="flex items-start justify-between gap-3"><span class="text-muted-foreground">{tr("Jumlah kata")}</span><span class="text-right font-medium">{checkStats ? `${checkStats.words.toLocaleString('id-ID')} ${tr('kata')}` : tr('Tidak ada')}</span></div>
				<div class="flex items-start justify-between gap-3"><span class="text-muted-foreground">{tr('Cover artikel')}</span><span class="text-right font-medium">{checkStats ? (checkStats.cover ? tr('Ada') : tr('Tidak ada')) : tr('Tidak ada')}</span></div>
				<div class="flex items-start justify-between gap-3"><span class="text-muted-foreground">{tr('Gambar di artikel')}</span><span class="text-right font-medium">{checkStats ? `${checkStats.images} ${tr('gambar')}` : tr('Tidak ada')}</span></div>
				<div class="flex items-start justify-between gap-3"><span class="text-muted-foreground">{tr('Tautan aktif')}</span><span class="text-right font-medium">{checkStats ? `${checkStats.links} ${tr('tautan')}` : tr('Tidak ada')}</span></div>
				<div class="flex items-start justify-between gap-3">
					<span class="text-muted-foreground">{tr("Kesesuaian media tujuan")}</span>
					<span class="text-right font-medium">{checkMediaType ? (checkMediaType === 'mainstream' ? 'Mainstream' : checkMediaType === 'blogger' ? 'Blogger' : checkMediaType) : tr('Tidak ada indikator')}</span>
				</div>
			</div>
			{#if !checkStats}<p class="mt-2 text-xs text-muted-foreground">{tr("Artikel belum dibuat untuk media ini — indikator ditulis \"Tidak ada\".")}</p>{/if}
			<div class="mt-4 flex justify-end">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (checklistFor = null)}>{tr("Tutup")}</button>
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
			initialTab={ed.tab}
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
			linkLimit={ed.linkLimit}
			sponsored={targetMedia?.label_sponsor === 'ada_sponsor'}
			saving={editorSaving}
			saveError={editorError}
			onClose={() => { if (!editorSaving) editor = null; }}
			onSave={(title, html, coverUrl, category, metaDescription) => void saveEditor(title, html, coverUrl, category, metaDescription)}
			onUploadCover={uploadCover}
			onUploadDoc={uploadDocNaskah}
			onGenerate={(topic, links, angleSlug) => generateNaskah(ed.mediaId, topic, links, angleSlug)}
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
			<div class="rounded-xl border p-3">
				<p class="text-[11px] font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('Waktu Terbit per Media')}</p>
				<p class="mt-1 text-[11px] text-muted-foreground">{tr('Atur satu per satu: biarkan "Sekarang" untuk terbit langsung, atau pilih tanggal untuk menjadwalkan media itu saja.')}</p>
				<!-- Daftar ini punya area gulir sendiri: dengan puluhan media, bloknya tidak
				     memanjang sampai mendorong isi lain keluar panel. -->
				<div class="mt-2 max-h-[42vh] space-y-2 overflow-y-auto pr-1">
					{#each selected as m (m.media_id)}
						{@const at = scheduleByMedia[m.media_id] ?? ''}
						<div class="rounded-lg border p-2">
							<div class="flex items-center justify-between gap-2">
								<span class="min-w-0 flex-1 truncate text-[13px] font-semibold">{m.name}</span>
								<span class="shrink-0 text-[11px] font-medium" style="color: {at ? 'var(--primary)' : 'var(--success)'};">{at ? tr('Terjadwal') : tr('Langsung')}</span>
							</div>
							<div class="mt-1.5 flex flex-wrap items-center gap-1.5">
								<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-[11px]" type="button" style={quickBtnStyle(quickPick[m.media_id] === 'now')} onclick={() => setMediaSchedule(m.media_id, 'now')}>{tr('Sekarang')}</button>
								<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-[11px]" type="button" style={quickBtnStyle(quickPick[m.media_id] === '1h')} onclick={() => setMediaSchedule(m.media_id, '1h')}>+1 jam</button>
								<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-[11px]" type="button" style={quickBtnStyle(quickPick[m.media_id] === '24h')} onclick={() => setMediaSchedule(m.media_id, '24h')}>+24 jam</button>
								<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-[11px]" type="button" style={quickBtnStyle(quickPick[m.media_id] === '3d')} onclick={() => setMediaSchedule(m.media_id, '3d')}>+3 hari</button>
							</div>
							<input
								class="app-input mt-1.5 w-full !text-xs"
								type="datetime-local"
								aria-label={`${tr('Jadwal terbit')} ${m.name}`}
								value={at}
								oninput={(e) => setMediaSchedule(m.media_id, (e.currentTarget as HTMLInputElement).value)}
							/>
						</div>
					{/each}
				</div>
			</div>

		</div>

		{#snippet footer()}
			<!-- Pemeriksaan cover + ringkasan biaya ditaruh di kaki panel: bagian ini
			     tidak ikut tergulir, jadi selalu terlihat tepat di atas tombol. -->
			<div class="w-full space-y-2">
				<div class="rounded-xl border p-3">
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

				<div class="rounded-xl border p-3 text-sm">
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
			border-radius: 14px;
			border: 1px solid var(--border);
			background: var(--card);
			box-shadow: 0 16px 40px rgba(15, 23, 42, 0.18);
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

		.selectedStatItemTotal {
			padding: 10px 16px;
			border-radius: 11px;
			background: var(--success-soft, #e8f5ee);
			/* Tombol mewarisi text-align: center bawaan browser — dipaksa rata
			   kiri supaya label dan angkanya sejajar. */
			text-align: left;
		}
		.selectedStatItemTotal .selectedStatLabel { color: var(--success); }
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
			border-radius: 8px;
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
			border-radius: 8px;
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
