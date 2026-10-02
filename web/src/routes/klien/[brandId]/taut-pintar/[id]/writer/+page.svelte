<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Agent 3: The Writer — dapur produksi artikel per media terpilih.
	// Port app/klien/[brandId]/taut-pintar/[id]/writer/client.tsx.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { CalendarClock, CircleCheck, Clock, Eye, FileText, Image, Link, Link2, ListChecks, LoaderCircle, Pencil, PencilLine, Send, ShieldCheck, Sparkles, TriangleAlert, Upload, Wallet, X } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, getAccessToken, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import AgentSteps from '../../_agent-steps.svelte';
	import TautBottomBar from '../../_bottom-bar.svelte';
	import NaskahModal from './_naskah-modal.svelte';
	import QcPanel from './_qc-panel.svelte';
	import { MANUAL_PROFILER_REASON, fmtIdr, rpCurrency, buildTrackedUrl, stripScheme, withScheme, textToHtml, toLocalInput } from '../../_shared';
	import { ARTICLE_DEFAULT_COVER } from '$lib/media/cover';
	import PageTitle from '$lib/components/page-title.svelte';
	import SlideOver from '$lib/components/slide-over.svelte';

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);

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
		regen_count: number | null;
		categories: string[];
		label_sponsor?: string;
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
	let links = $state<LinksForm>(EMPTY_LINKS);
	let linksStatus = $state<'idle' | 'dirty' | 'saving' | 'saved' | 'error'>('idle');
	let loading = $state(true);
	let error = $state('');
	let backlinkEnabled = $state(false);
	let entryMode = $state<'ai' | 'manual' | null>(null);
	let regenMax = $state(2);
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
	let gatekeeperAgent = $state<{ configured: boolean; reason: string | null } | null>(null);
	// Distribusi langsung dari The Writer ketika QC sudah lulus (Gatekeeper dilewati).
	let distributable = $state<boolean | null>(null);
	let distributing = $state(false);
	let distributeFailed = $state(false);
	let topup = $state<{ needed: number; available: number } | null>(null);
	// Panel geser penerbitan: jadwal diatur PER MEDIA (boleh campur: satu terbit
	// langsung, satu terjadwal), dan cover tiap artikel dipastikan dulu.
	let publishOpen = $state(false);
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

	async function load() {
		try {
			const [placeRes, linksRes, draftRes, auditsRes] = await Promise.all([
				apiRequest(`${apiBase}/placement`),
				apiRequest(`${apiBase}/writer-links`),
				apiRequest(`${apiBase}/profile/draft`),
				apiRequest(`${apiBase}/gatekeeper/audits`)
			]);
			if (placeRes.ok) {
				const d = await placeRes.json();
				placement = d.media ?? [];
				const rm = Number(d.regen_max ?? 2);
				if (Number.isFinite(rm) && rm >= 0) regenMax = rm;
			}
			// null = belum diketahui / gagal dibaca → tombol kembali ke jalur aman
			// (mampir ke halaman Gatekeeper).
			distributable = auditsRes.ok ? (await auditsRes.json().catch(() => null))?.canDistribute === true : null;
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
			// Menulis artikel memakai LLM (puluhan detik); timeout bawaan 12 dtk
			// akan memutus request dan membatalkan pekerjaan di server.
			const res = await apiRequest(`${apiBase}/produce/${item.media_id}`, { method: 'POST' }, 240_000);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal generate artikel.')));
			genState = { ...genState, [item.media_id]: 'draft' };
			qcToken += 1;
			if (openAfter) await openPreview(item);
		} catch (reason) {
			genState = { ...genState, [item.media_id]: 'error' };
			error = reason instanceof Error ? reason.message : tr('Gagal generate artikel.');
		}
	}

	async function generateAll() {
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
		for (const m of targets) {
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
				if (/kredit|saldo|402/i.test(msg)) {
					done += 1;
					bulkProgress = { done, total: targets.length };
					break;
				}
			}
			done += 1;
			bulkProgress = { done, total: targets.length };
		}
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

	async function uploadDocNaskah(file: File): Promise<{ title: string; html: string }> {
		const form = new FormData();
		form.append('file', file);
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
		return { title: judul.slice(0, 200), html: textToHtml(isi) };
	}

	// Generate dengan arahan topik dari modal Pengelola Naskah (1 kredit bila
	// artikel belum ada; regenerasi 2 kredit sesuai aturan backend).
	async function generateNaskah(mediaId: string, topic: string): Promise<{ title: string; html: string }> {
		const res = await apiRequest(`${apiBase}/produce/${mediaId}`, {
			method: 'POST',
			body: JSON.stringify({ topic })
		}, 240_000);
		const data = await res.json().catch(() => null);
		if (!res.ok) throw new Error(data?.detail ?? tr('Gagal membuat draf AI.'));
		genState = { ...genState, [mediaId]: 'draft' };
		qcToken += 1;
		return { title: String(data?.title ?? ''), html: String(data?.content_html ?? '') };
	}

	async function saveEditor(title: string, html: string, coverUrl: string | null, category: string, metaDescription: string) {
		const current = editor;
		if (!current) return;
		const mediaId = current.mediaId;
		editorSaving = true;
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
					body: JSON.stringify({ title, content_html: html })
				});
				if (!postRes.ok) throw new Error(await readApiError(postRes, tr('Gagal menyimpan artikel.')));
				genState = { ...genState, [mediaId]: 'draft' };
			}
			editor = null;
			qcToken += 1;
			await load();
		} catch (reason) {
			editorError = reason instanceof Error ? reason.message : tr('Gagal menyimpan artikel.');
		} finally {
			editorSaving = false;
		}
	}

	async function uploadCover(file: File): Promise<string> {
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
		const writing = selected.reduce((s, m) => s + (!hasDoc(m) ? 1 : canRegen(m) ? 2 : 0), 0);
		const media = selected.reduce((s, m) => s + (m.credit_cost ?? 0), 0);
		return { count: selected.length, writing, media, total: writing + media };
	});
	const pendingCount = $derived(selected.filter((m) => !hasDoc(m) || canRegen(m)).length);
	const toRp = (credits: number) => (rateIdr > 0 ? rpCurrency(credits * rateIdr) : `${credits} ${tr('Kredit')}`);

	// Tooltip rincian biaya tombol "Generate Semua Artikel" (parity Tooltip app lama).
	const generateTooltip = $derived.by(() => {
		if (cost.writing <= 0) return tr('Semua media sudah memiliki artikel — generate tidak diperlukan.');
		const baru = selected.filter((m) => !hasDoc(m)).length;
		const regen = selected.filter((m) => hasDoc(m) && canRegen(m)).length;
		return `${tr('Rincian:')} ${baru} ${tr('artikel baru')} × ${toRp(1)}${regen ? ` + ${regen} ${tr('regenerasi')} × ${toRp(2)}` : ''} = ${tr('total')} ${toRp(cost.writing)}`;
	});

	const editorMedia = $derived.by(() => {
		const current = editor;
		if (!current) return null;
		return selected.find((m) => m.media_id === current.mediaId) ?? null;
	});

	async function publish() {
		if (coverChecking || distributing) return;
		// QC belum lulus → halaman Gatekeeper yang menangani audit + penjadwalan.
		if (!distributable) {
			await goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/gatekeeper`);
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
							: tr('MENUNGGU AI');
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

<PageTitle title="Taut Pintar — The Writer" />

<main>
	<div class="flex flex-wrap items-center justify-between gap-3">
		<div class="flex items-center gap-3">
			<div class="flex h-10 w-10 items-center justify-center rounded-xl" style="background-color: var(--primary);">
				<Pencil size={22} style="color: #fff;" />
			</div>
			<div>
				<p class="font-bold leading-tight">Agent 3: The Writer</p>
				<p class="text-sm leading-tight text-muted-foreground">{manualFlow ? tr('Dapur Produksi') : tr('Dapur Produksi AI')} ({cost.count} {tr('media terpilih')})</p>
			</div>
		</div>
		<div class="flex flex-wrap items-center justify-end gap-3">
			{#if campaignName}
				<span class="inline-flex max-w-[260px] items-center gap-1.5 truncate rounded-full border px-3.5 py-1.5 text-[12.5px] font-semibold" style="background-color: var(--muted);" title={campaignName}><FileText size={13} />{campaignName}</span>
			{/if}
			<span class="inline-flex items-center gap-1.5 rounded-full px-3.5 py-1.5 text-[13px] font-bold" style="background-color: var(--success-soft); color: var(--success);">{fmtIdr(wallet)} {tr("Kredit")}</span>
		</div>
	</div>

	<div class="mt-3">
		<AgentSteps current={3} lockedSteps={manualFlow ? [1] : undefined} lockedReason={MANUAL_PROFILER_REASON} />
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

	{#if entryMode === 'manual'}
		<div class="mt-3 flex flex-wrap items-center gap-3.5 rounded-xl border p-4" style="background-color: var(--accent);">
			<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg" style="background-color: var(--primary);"><Sparkles size={18} style="color: #fff;" /></div>
			<div class="flex-1">
				<p class="font-bold">{tr("Mau AI yang mengerjakan sisanya?")}</p>
				<p class="text-sm text-muted-foreground">{tr("Kampanye ini berjalan tanpa AI — kamu memilih media dan menyiapkan artikelnya sendiri. Kalau ingin AI menyusun brief, memilih media, lalu menulis artikelnya, mulai dari The Profiler. Kampanye ini tetap dipakai, jadi tidak perlu membuat kampanye baru.")}</p>
			</div>
			<button class="app-btn app-btn-primary shrink-0" type="button" onclick={() => goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/profiler`)}>{tr("Mulai dari The Profiler")}</button>
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
			<p class="font-bold">{tr('Saldo Kredit Tidak Cukup')}</p>
			<p class="mt-1">Distribusi butuh <strong>{fmtIdr(topup.needed)} {tr("Kredit")}</strong>, saldo tersedia <strong>{fmtIdr(topup.available)} {tr("Kredit")}</strong>. Silakan top up terlebih dahulu.</p>
			<div class="mt-2 flex flex-wrap gap-2">
				<a class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" href={`/klien/${brandUrl}/credits`}>{tr('Top Up Kredit')}</a>
				<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={() => (topup = null)}>{tr('Tutup')}</button>
			</div>
		</div>
	{/if}

	{#if distributeFailed}
		<div class="app-card mt-3 flex flex-wrap items-center gap-3 px-4 py-3 text-sm">
			<span class="flex-1 text-muted-foreground">{tr('Penerbitan langsung gagal. Kamu masih bisa menyelesaikannya lewat halaman The Gatekeeper.')}</span>
			<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={() => goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/gatekeeper`)}><ShieldCheck size={14} /> {tr('Buka Gatekeeper')}</button>
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
								{tr('Mode manual — unggah artikel tulisanmu sendiri untuk tiap media.')}
							{:else}
								{tr('Biaya penulisan konten organik oleh The Writer adalah')} <strong>{toRp(1)}</strong> {tr('/ Artikel.')}
							{/if}
						</p>
					</div>
				</div>
				{#if !manualFlow}
					<button class="app-btn app-btn-primary" type="button" disabled={bulkRunning || pendingCount === 0} title={generateTooltip} onclick={generateAll}>
						{#if bulkRunning}<LoaderCircle size={16} class="animate-spin" />{:else}<Sparkles size={16} />{/if}
						{bulkRunning
							? `${tr('Membuat')} ${bulkProgress ? `${bulkProgress.done}/${bulkProgress.total}` : ''}…`
							: cost.writing > 0
								? `${tr('Generate Semua Artikel')} · ${toRp(cost.writing)}`
								: tr('Generate Semua Artikel')}
					</button>
				{/if}
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
									<p class="font-bold leading-tight">{m.name}</p>
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
								<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={bulkRunning} onclick={() => (checklistFor = m.media_id)}><ListChecks size={14} /> {tr('Cek Validasi')}</button>
								{#if !manualFlow && !hasDoc(m)}
									<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={generating || bulkRunning} onclick={() => void generateOne(m)}>
										{#if generating}<LoaderCircle size={14} class="animate-spin" />{:else}<Sparkles size={14} />{/if}
										{generating ? tr('Membuat…') : queued ? tr('Mengantre…') : tr('Generate Konten')}
									</button>
								{/if}
								{#if !hasDoc(m)}
									<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={bulkRunning} onclick={() => openNaskahUpload(m)}>
										<Upload size={14} /> {tr('Unggah Artikel')}
									</button>
								{/if}
								{#if hasDoc(m)}
									<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={bulkRunning} onclick={() => void openPreview(m)}><Eye size={14} /> {tr('Edit Artikel')}</button>
									<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={bulkRunning} onclick={() => (qcFor = m.media_id)}><ShieldCheck size={14} /> {tr('Analisis QC')}</button>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</section>

		<TautBottomBar>
			<div class="flex flex-wrap items-start gap-5">
				<div class="flex flex-col gap-1">
					<p class="flex h-4 items-center text-[11px] font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('BIAYA MEDIA')} ({cost.count} {tr('Media')})</p>
					<p class="text-[17px] font-bold leading-tight">{toRp(cost.media)}</p>
				</div>
				<span class="mt-3.5 self-start text-lg font-semibold" style="color: var(--border);">+</span>
				<div class="flex flex-col gap-1">
					<p class="flex h-4 items-center text-[11px] font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('BIAYA PENULISAN AI')}</p>
					<p class="text-[17px] font-bold leading-tight">{toRp(cost.writing)}</p>
				</div>
				<div class="flex flex-col gap-1">
					<p class="flex h-4 items-center gap-1 text-[11px] font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('TOTAL TAGIHAN AKHIR')} <Wallet size={13} /></p>
					<p class="text-[19px] font-extrabold leading-tight">{toRp(cost.total)}</p>
				</div>
			</div>
			<div class="flex flex-col items-end gap-1.5">
				{#if distributable}
					<!-- Lebarnya dibatasi supaya blok tombol tidak melebar sampai memaksa
					     bar melipat dan tombolnya turun ke baris kedua. -->
					<p class="max-w-[330px] text-right text-[11px] leading-snug text-muted-foreground">{tr('QC sudah lulus — tombol ini langsung menerbitkan kampanye dan membuka Ringkasan.')}</p>
				{/if}
				<div class="flex flex-wrap items-center gap-3">
					<button class="app-btn app-btn-soft" type="button" onclick={() => goto(`/klien/${brandUrl}/taut-pintar`)}>{tr('Kembali')}</button>
					<button class="app-btn" type="button" style="background-color: var(--success); color: #fff;" disabled={coverChecking || distributing} onclick={publish}>
						{#if coverChecking || distributing}<LoaderCircle size={16} class="animate-spin" />{:else}<Send size={16} />{/if}
						{coverChecking ? tr('Memeriksa cover…') : distributing ? tr('Menerbitkan…') : tr('Lanjut Tinjau & Terbit')}
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
	{#key ed.mediaId}
		<NaskahModal
			{brandId}
			mediaName={ed.mediaName}
			initialTab={ed.tab}
			initialTitle={ed.title}
			initialHtml={ed.html}
			initialCoverUrl={ed.cover}
			initialCategory={ed.category}
			initialMetaDescription={ed.metaDescription}
			categoryOptions={placement.find((p) => p.media_id === ed.mediaId)?.categories ?? []}
			linkLimit={ed.linkLimit}
			saving={editorSaving}
			saveError={editorError}
			onClose={() => { if (!editorSaving) editor = null; }}
			onSave={(title, html, coverUrl, category, metaDescription) => void saveEditor(title, html, coverUrl, category, metaDescription)}
			onUploadCover={uploadCover}
			onUploadDoc={uploadDocNaskah}
			onGenerate={(topic) => generateNaskah(ed.mediaId, topic)}
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
			regenLabel={tr('Generate Ulang (2 Kredit)')}
			canRegen={canRegen(qc) && !bulkRunning}
			{manualFlow}
			generating={articleStatusOf(qc) === 'generating'}
			gatekeeperConfigured={gatekeeperAgent ? gatekeeperAgent.configured : null}
			reloadToken={qcToken}
			onClose={() => (qcFor = null)}
			onEdit={() => void openPreview(qc)}
			onRegenerate={() => void generateOne(qc, false)}
			onOpenGatekeeper={() => goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/gatekeeper`)}
			onScanned={() => { qcToken += 1; void load(); }}
		/>
	{/key}
{/if}
