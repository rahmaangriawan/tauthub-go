<script lang="ts">
	// Agent 3: The Writer — dapur produksi artikel per media terpilih.
	// Port app/klien/[brandId]/taut-pintar/[id]/writer/client.tsx.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { CircleCheck, Clock, Eye, FileText, Image, Link, Link2, ListChecks, LoaderCircle, Pencil, PenLine, Send, Sparkles, TriangleAlert, Upload, X } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, getAccessToken, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import AgentSteps from '../../_agent-steps.svelte';
	import EditorModal from './_editor-modal.svelte';
	import { MANUAL_PROFILER_REASON, fmtIdr, rpCurrency, buildTrackedUrl, stripScheme, withScheme, textToHtml } from '../../_shared';

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

	let campaignName = $state('Kampanye');
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
	let shortLinkDomain = $state<string | null>(null);
	let primaryCode = $state<string | null>(null);
	let backupCode = $state<string | null>(null);

	let editor = $state<{
		mediaId: string;
		mediaName: string;
		title: string;
		html: string;
		cover: string | null;
		category: string | null;
		metaDescription: string | null;
		linkLimit: number;
		allowEmbed: boolean;
	} | null>(null);
	let editorSaving = $state(false);
	let editorError = $state('');

	let saveLinksTimer: number | null = null;
	let saveLinksSeq = 0;
	let hydrated = false;
	let linksDirty = false;

	const REASON_LABEL: Record<string, string> = {
		'dinonaktifkan oleh admin': 'AI Writer sedang dinonaktifkan oleh admin',
		'base_url kosong': 'AI Writer belum dikonfigurasi (server AI belum diatur admin)',
		'api key belum diisi atau kedaluwarsa': 'AI Writer belum bisa dipakai — API key belum diisi atau sudah kedaluwarsa. Hubungi admin.',
		'model belum dipilih': 'AI Writer belum dikonfigurasi (model AI belum dipilih admin)'
	};

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
			const res = await apiRequest(`/v1/brands/${brandId}/taut/agent-status?agent=writer`);
			if (res.ok) {
				const d = await res.json();
				const w = d.agents?.find((a: { agent: string }) => a.agent === 'writer');
				if (w) agentStatus = { configured: w.configured, reason: w.reason };
			}
		} catch {
			/* status opsional */
		}
	}

	async function load() {
		try {
			const [placeRes, linksRes, draftRes] = await Promise.all([
				apiRequest(`${apiBase}/placement`),
				apiRequest(`${apiBase}/writer-links`),
				apiRequest(`${apiBase}/profile/draft`)
			]);
			if (placeRes.ok) {
				const d = await placeRes.json();
				placement = d.media ?? [];
				const rm = Number(d.regen_max ?? 2);
				if (Number.isFinite(rm) && rm >= 0) regenMax = rm;
			}
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
				campaignName = draftData.campaign?.name ?? 'Kampanye';
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
			error = 'Gagal memuat data Writer.';
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
		if (!linksComplete()) return 'Belum lengkap';
		switch (linksStatus) {
			case 'saved': return 'Tersimpan otomatis';
			case 'saving': return 'Menyimpan…';
			case 'error': return 'Gagal tersimpan';
			default: return 'Belum tersimpan';
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
			const res = await apiRequest(`${apiBase}/produce/${item.media_id}`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal generate artikel.'));
			genState = { ...genState, [item.media_id]: 'draft' };
			if (openAfter) await openPreview(item);
		} catch (reason) {
			genState = { ...genState, [item.media_id]: 'error' };
			error = reason instanceof Error ? reason.message : 'Gagal generate artikel.';
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
				const res = await apiRequest(`${apiBase}/produce/${m.media_id}`, { method: 'POST' });
				if (!res.ok) throw new Error(await readApiError(res, isRegen ? 'Gagal generate ulang artikel.' : 'Gagal generate artikel.'));
				genState = { ...genState, [m.media_id]: 'draft' };
			} catch (reason) {
				const msg = reason instanceof Error ? reason.message : 'Gagal generate artikel.';
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
				title: d.title ?? '',
				html: d.content_html ?? '',
				cover: d.featured_image_url ?? null,
				category: d.category ?? null,
				metaDescription: d.meta_description ?? null,
				linkLimit,
				allowEmbed: item.allow_embed
			};
		} catch (reason) {
			error = reason instanceof Error ? reason.message : 'Gagal membuka artikel.';
		}
	}

	async function uploadFor(item: PlacementItem, file: File) {
		uploadBusyId = item.media_id;
		error = '';
		try {
			const form = new FormData();
			form.append('file', file);
			const upRes = await apiRequest(`${apiBase}/files`, { method: 'POST', body: form });
			const upData = await upRes.json().catch(() => null);
			const text = upData?.file && typeof upData.file.extracted_text === 'string' ? String(upData.file.extracted_text) : '';
			if (!upRes.ok || !text.trim()) throw new Error(upData?.detail ?? 'File gagal dibaca. Gunakan PDF, DOCX, atau TXT.');
			const base = file.name.replace(/\.[^.]+$/, '');
			const upLimit = mediaAllowsLink(item) ? (item.max_links && item.max_links > 0 ? item.max_links : 2) : 0;
			editorError = '';
			editor = {
				mediaId: item.media_id,
				mediaName: item.name,
				title: base.slice(0, 200),
				html: textToHtml(text),
				cover: null,
				category: null,
				metaDescription: null,
				linkLimit: upLimit,
				allowEmbed: item.allow_embed
			};
		} catch (reason) {
			error = reason instanceof Error ? reason.message : 'Gagal mengunggah file.';
		} finally {
			uploadBusyId = null;
		}
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
				if (!putRes.ok) throw new Error(await readApiError(putRes, 'Gagal menyimpan artikel.'));
			} else {
				const postRes = await apiRequest(`${apiBase}/articles/${mediaId}/upload`, {
					method: 'POST',
					body: JSON.stringify({ title, content_html: html })
				});
				if (!postRes.ok) throw new Error(await readApiError(postRes, 'Gagal menyimpan artikel.'));
				genState = { ...genState, [mediaId]: 'draft' };
			}
			editor = null;
			await load();
		} catch (reason) {
			editorError = reason instanceof Error ? reason.message : 'Gagal menyimpan artikel.';
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
		if (!res.ok || !data?.url) throw new Error(data?.detail ?? 'Gagal mengunggah cover.');
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
	const toRp = (credits: number) => (rateIdr > 0 ? rpCurrency(credits * rateIdr) : `${credits} Kredit`);

	const editorMedia = $derived.by(() => {
		const current = editor;
		if (!current) return null;
		return selected.find((m) => m.media_id === current.mediaId) ?? null;
	});

	async function publish() {
		if (coverChecking) return;
		coverChecking = true;
		error = '';
		try {
			// Gerbang cover: artikel tanpa cover menghalangi lanjut ke Gatekeeper.
			const missing: string[] = [];
			for (const m of selected) {
				if (!hasDoc(m)) continue;
				try {
					const res = await apiRequest(`${apiBase}/articles/${m.media_id}`);
					if (!res.ok) continue;
					const d = await res.json();
					if (!d.featured_image_url?.trim()) missing.push(m.name);
				} catch {
					/* tak diketahui — tidak memblokir */
				}
			}
			if (missing.length) {
				error = `Cover belum diisi untuk ${missing.length} artikel (${missing.join(', ')}). Buka Edit Artikel, unggah cover atau pilih 'pakai logo Tauthub' saat menyimpan.`;
				return;
			}
			await goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/gatekeeper`);
		} finally {
			coverChecking = false;
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
			? 'Generasi AI…'
			: s === 'queued'
				? 'Dalam Antrean…'
				: s === 'draft'
					? 'Draft Tersedia'
					: s === 'ready'
						? 'Siap Terbit'
						: s === 'blocked'
							? 'Diblokir'
							: 'MENUNGGU AI';
	const badge = (tone: string) =>
		tone === 'success'
			? 'background-color: var(--success-soft); color: var(--success);'
			: tone === 'warning'
				? 'background-color: var(--warning-soft); color: var(--warning);'
				: tone === 'destructive'
					? 'background-color: var(--destructive-foreground, #fdecee); color: var(--destructive);'
					: tone === 'accent'
						? 'background-color: var(--accent); color: var(--accent-foreground);'
						: 'background-color: var(--muted); color: var(--muted-foreground);';
</script>

<main class="mx-auto max-w-6xl">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<div class="flex items-center gap-3">
			<div class="flex h-10 w-10 items-center justify-center rounded-xl" style="background-color: var(--primary);">
				<PenLine size={22} style="color: #fff;" />
			</div>
			<div>
				<p class="font-bold leading-tight">Agent 3: The Writer</p>
				<p class="text-sm leading-tight text-muted-foreground">Dapur Produksi{manualFlow ? '' : ' AI'} ({cost.count} media terpilih)</p>
			</div>
		</div>
		<div class="flex flex-wrap items-center justify-end gap-3">
			{#if campaignName}
				<span class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium"><FileText size={13} />{campaignName}</span>
			{/if}
			<span class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium">{fmtIdr(wallet)} Kredit</span>
		</div>
	</div>

	<div class="mt-3">
		<AgentSteps current={3} lockedSteps={manualFlow ? [1] : undefined} lockedReason={MANUAL_PROFILER_REASON} />
	</div>

	{#if spokepersons.length > 0}
		<div class="app-card mt-3 p-4">
			<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">Kutipan Narasumber (dari The Profiler)</p>
			<div class="mt-1 space-y-2">
				{#each spokepersons as s, i (s.name + '-' + i)}
					<div>
						<p class="text-sm font-bold">{s.name}{s.role ? `, ${s.role}` : ''}</p>
						{#if s.quote}<p class="text-sm text-muted-foreground">Kutipan: &ldquo;{s.quote}&rdquo;</p>{/if}
					</div>
				{/each}
			</div>
			<p class="mt-2 text-xs text-muted-foreground">Kutipan ini otomatis dipakai AI saat membuat artikel untuk semua media terpilih (minimal satu kutipan, dengan atribusi nama dan jabatan).</p>
		</div>
	{/if}

	{#if entryMode === 'manual'}
		<div class="app-card mt-3 flex flex-wrap items-center gap-3 p-4">
			<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg" style="background-color: var(--primary);"><Sparkles size={18} style="color: #fff;" /></div>
			<div class="flex-1">
				<p class="font-bold">Mau AI yang mengerjakan sisanya?</p>
				<p class="text-sm text-muted-foreground">
					Kampanye ini berjalan tanpa AI — kamu memilih media dan menyiapkan artikelnya sendiri.
					Kalau ingin AI menyusun brief, memilih media, lalu menulis artikelnya, mulai dari The Profiler.
					Kampanye ini tetap dipakai, jadi tidak perlu membuat kampanye baru.
				</p>
			</div>
			<a class="app-btn app-btn-primary" href={`/klien/${brandUrl}/taut-pintar/${campaignId}/profiler`}>Mulai dari The Profiler</a>
		</div>
	{/if}

	{#if agentStatus && !agentStatus.configured}
		<div class="app-card mt-3 flex items-start gap-2 px-4 py-2.5 text-sm" style="color: var(--warning);" role="alert">
			<TriangleAlert size={16} class="mt-0.5 shrink-0" />
			<span>{REASON_LABEL[agentStatus.reason ?? ''] ?? 'AI Writer belum dikonfigurasi oleh admin.'} Generate artikel tidak akan berjalan sampai admin mengaktifkannya.</span>
		</div>
	{/if}

	{#if error}
		<div class="app-card mt-3 flex items-start gap-2 px-4 py-2.5 text-sm" style="color: var(--destructive);">
			<span class="flex-1">{error}</span>
			<button class="rounded p-0.5 hover:bg-secondary" type="button" aria-label="Tutup error" onclick={() => (error = '')}><X size={12} /></button>
		</div>
	{/if}

	{#if loading}
		<div class="mt-6 flex items-center gap-2 text-sm text-muted-foreground"><LoaderCircle size={18} class="animate-spin" /> Memuat The Writer…</div>
	{:else}
		{#if showLinksCard}
			<section class="app-card mt-3 p-4" aria-label="Mode pelacakan dan anchor text">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<div class="flex items-center gap-2">
						<span class="flex h-7 w-7 items-center justify-center rounded-lg" style="background-color: var(--accent); color: var(--accent-foreground);"><Link size={18} /></span>
						<p class="font-bold">Mode Pelacakan &amp; Anchor Text</p>
					</div>
					<p class="text-xs" style="color: {linksStatus === 'error' ? 'var(--destructive)' : linksStatus === 'saving' ? 'var(--primary)' : linksStatus === 'dirty' ? 'var(--warning)' : 'var(--muted-foreground)'};">{linksStatusLabel()}</p>
				</div>
				<p class="mt-1 text-sm text-muted-foreground">Pilih bagaimana Tauthub menyuntikkan tautan, lalu atur peluru URL dan kata kuncinya.</p>

				<div class="mt-3 flex items-center gap-2">
					<input
						type="checkbox"
						aria-label="Tauthub Tracking Link"
						checked={trackingOn}
						disabled={linksStatus === 'saving'}
						onchange={(e) => { trackingOn = (e.currentTarget as HTMLInputElement).checked; patchLinks({ tracking_mode: trackingOn ? 'tauthub' : 'direct' }); }}
					/>
					<span class="text-xs font-medium">Tauthub Tracking Link</span>
					<span class="text-xs text-muted-foreground">(short link + statistik klik — mati = Pure SEO)</span>
				</div>

				<div class="mt-3 grid gap-3 sm:grid-cols-2" role="radiogroup" aria-label="Mode pelacakan">
					<button
						type="button"
						class="rounded-xl border p-3 text-left"
						style="border-color: {links.tracking_mode === 'direct' ? 'var(--primary)' : 'var(--border)'}; background-color: {links.tracking_mode === 'direct' ? 'var(--accent)' : 'transparent'};"
						role="radio" aria-checked={links.tracking_mode === 'direct'}
						onclick={() => { trackingOn = false; patchLinks({ tracking_mode: 'direct' }); }}
					>
						<div class="flex items-center justify-between">
							<span class="inline-flex items-center gap-2 font-medium"><Link2 size={16} /> Direct URL (Pure SEO)</span>
							{#if links.tracking_mode === 'direct'}<CircleCheck size={18} style="color: var(--primary);" />{/if}
						</div>
						<p class="mt-2 text-xs text-muted-foreground">URL ditanam mentah tanpa redirect untuk 100% aliran Link Juice — default.</p>
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
								<span class="inline-flex items-center gap-2 font-medium"><Link size={16} /> Tauthub Tracking Link</span>
								{#if links.tracking_mode === 'tauthub'}<CircleCheck size={18} style="color: var(--primary);" />{/if}
							</div>
							<p class="mt-2 text-xs text-muted-foreground">Menggunakan format parameter <code>?utm_source=tauthub</code> untuk merekam data CTR dan Analytics. Cocok untuk kampanye PR &amp; Promosi.</p>
						</button>
					{/if}
				</div>

				<div class="mt-4 space-y-4">
					<div class="rounded-xl border p-3" style="border-color: var(--primary);">
						<p class="flex items-center gap-2 text-sm font-bold" style="color: var(--primary);"><Link2 size={16} /> Tautan Utama (Prioritas 1)</p>
						<div class="mt-3 grid gap-3 sm:grid-cols-2">
							<label class="block">
								<span class="text-xs font-medium">Target URL <span style="color: var(--destructive);">*</span></span>
								<div class="app-input mt-1 flex items-center gap-1">
									<span class="text-xs text-muted-foreground">https://</span>
									<input class="w-full bg-transparent text-sm outline-none" placeholder="tauthub.com/promo" value={links.primary_url} oninput={(e) => patchLinks({ primary_url: stripScheme((e.currentTarget as HTMLInputElement).value) })} />
								</div>
							</label>
							<label class="block">
								<span class="text-xs font-medium">Anchor Text Induk <span style="color: var(--destructive);">*</span></span>
								<input class="app-input mt-1 w-full" maxlength="120" placeholder="TautHub PR Platform" value={links.primary_anchor} oninput={(e) => patchLinks({ primary_anchor: (e.currentTarget as HTMLInputElement).value })} />
							</label>
						</div>
						{#if primaryShortLink}
							<p class="mt-2 break-all text-xs" style="color: var(--primary);">Preview tautan utama: <code>{primaryShortLink}</code></p>
						{:else if finalUrlPreview}
							<p class="mt-2 break-all text-xs" style="color: var(--primary);">Preview URL final: <code>{finalUrlPreview}</code></p>
						{/if}
					</div>

					<div class="rounded-xl border p-3">
						<div class="flex items-center justify-between">
							<p class="flex items-center gap-2 text-sm font-bold text-muted-foreground"><Link2 size={16} /> Tautan Cadangan (Prioritas 2)</p>
							<span class="rounded-full px-2 py-0.5 text-[10px] font-semibold" style={badge('muted')}>Opsional</span>
						</div>
						<p class="mt-1 text-xs text-muted-foreground">Hanya akan digunakan secara dinamis oleh AI pada media mitra yang mengizinkan penanaman <strong>Maksimal 2 Link</strong>.</p>
						<div class="mt-3 grid gap-3 sm:grid-cols-2">
							<label class="block">
								<span class="text-xs font-medium">Target URL Cadangan</span>
								<div class="app-input mt-1 flex items-center gap-1">
									<span class="text-xs text-muted-foreground">https://</span>
									<input class="w-full bg-transparent text-sm outline-none" placeholder="tauthub.com/about" value={links.backup_url} oninput={(e) => patchLinks({ backup_url: stripScheme((e.currentTarget as HTMLInputElement).value) })} />
								</div>
							</label>
							<label class="block">
								<span class="text-xs font-medium">Anchor Text Cadangan</span>
								<input class="app-input mt-1 w-full" maxlength="120" placeholder="Platform PR Terbaik Indonesia" value={links.backup_anchor} oninput={(e) => patchLinks({ backup_anchor: (e.currentTarget as HTMLInputElement).value })} />
							</label>
						</div>
						{#if backupShortLink}
							<p class="mt-2 break-all text-xs" style="color: var(--primary);">Preview tautan cadangan: <code>{backupShortLink}</code></p>
						{:else if backupUrlPreview}
							<p class="mt-2 break-all text-xs" style="color: var(--primary);">Preview URL cadangan: <code>{backupUrlPreview}</code></p>
						{/if}
					</div>
				</div>
			</section>
		{/if}

		<section class="app-card mt-3 p-4" aria-label="Dapur produksi AI">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div class="flex items-center gap-2">
					<span class="flex h-7 w-7 items-center justify-center rounded-lg" style="background-color: var(--accent); color: var(--accent-foreground);"><FileText size={16} /></span>
					<div>
						<p class="font-bold">{manualFlow ? `Dapur Produksi (${cost.count} Media Terpilih)` : `Dapur Produksi AI (${cost.count} Media Terpilih)`}</p>
						<p class="text-xs text-muted-foreground">
							{manualFlow
								? 'Mode manual — unggah artikel tulisanmu sendiri untuk tiap media.'
								: `Biaya penulisan konten organik oleh The Writer adalah ${toRp(1)} / Artikel.`}
						</p>
					</div>
				</div>
				{#if !manualFlow}
					<button class="app-btn app-btn-primary" type="button" disabled={bulkRunning || pendingCount === 0} onclick={generateAll}>
						{#if bulkRunning}<LoaderCircle size={16} class="animate-spin" />{:else}<Sparkles size={16} />{/if}
						{bulkRunning
							? `Membuat ${bulkProgress ? `${bulkProgress.done}/${bulkProgress.total}` : ''}…`
							: cost.writing > 0
								? `Generate Semua Artikel · ${toRp(cost.writing)}`
								: 'Generate Semua Artikel'}
					</button>
				{/if}
			</div>

			{#if selected.length === 0}
				<div class="mt-3 rounded-lg border px-4 py-8 text-center text-sm text-muted-foreground">Belum ada media terpilih. Kembali ke Matchmaker untuk memilih media.</div>
			{:else}
				<div class="mt-3 space-y-2">
					{#each selected as m (m.media_id)}
						{@const st = articleStatusOf(m)}
						{@const generating = st === 'generating'}
						{@const links2 = linkStats[m.media_id] ?? null}
						<div class="rounded-xl border p-3">
							<div class="flex flex-wrap items-start justify-between gap-3">
								<div class="min-w-0">
									<p class="font-medium">{m.name}</p>
									<div class="mt-1 flex flex-wrap items-center gap-2">
										<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style={badge(statusTone(st === 'error' ? 'none' : st))}>
											{#if st === 'generating'}<LoaderCircle size={11} class="animate-spin" />{:else if st === 'queued'}<Clock size={11} />{/if}
											{statusText(st === 'error' ? 'none' : st)}
										</span>
										{#if !mediaAllowsLink(m)}
											<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style={badge('warning')}>Tidak Tersedia Backlink</span>
										{:else if links2}
											<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style={badge('warning')}>
												<Pencil size={11} /> {links2.dofollow} Dofollow - {linkStatsTail(links2)}
											</span>
										{:else}
											<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style={badge('warning')}>
												<Pencil size={11} /> {m.label_sponsor === 'ada_sponsor' ? 'Sponsored' : 'Dofollow'} · Maks. {m.max_links != null && m.max_links > 0 ? m.max_links : 1} Link
											</span>
										{/if}
									</div>
								</div>
								<div class="text-right">
									{#if articleTitles[m.media_id]}
										<p class="text-[10px] uppercase tracking-wide text-muted-foreground">Judul Artikel</p>
										<p class="max-w-[280px] truncate text-sm" title={articleTitles[m.media_id]}>{articleTitles[m.media_id]}</p>
									{/if}
									<p class="mt-1 inline-flex items-center gap-1.5 text-xs text-muted-foreground">
										<Image size={12} />
										{#if hasDoc(m) && articleStats[m.media_id]}
											<span style="color: {articleStats[m.media_id].cover ? 'var(--success)' : 'var(--destructive)'}; font-weight: 600;">Cover {articleStats[m.media_id].cover ? 'ada' : 'tidak ada'}</span>
											<span>·</span><span>{articleStats[m.media_id].images} gambar di artikel</span>
											<span>·</span><span>{articleStats[m.media_id].words.toLocaleString('id-ID')} kata</span>
										{:else}
											<span>Gambar artikel: tidak ada</span>
										{/if}
									</p>
								</div>
							</div>
							<div class="mt-3 flex flex-wrap gap-2">
								<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={bulkRunning} onclick={() => (checklistFor = m.media_id)}><ListChecks size={14} /> Cek Validasi</button>
								{#if !manualFlow && !hasDoc(m)}
									<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={generating || bulkRunning} onclick={() => void generateOne(m)}>
										{#if generating}<LoaderCircle size={14} class="animate-spin" />{:else}<Sparkles size={14} />{/if}
										{generating ? 'Membuat…' : st === 'queued' ? 'Mengantre…' : 'Generate Konten'}
									</button>
								{/if}
								{#if !hasDoc(m)}
									<label class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs cursor-pointer">
										{#if uploadBusyId === m.media_id}<LoaderCircle size={14} class="animate-spin" />{:else}<Upload size={14} />{/if}
										Unggah Artikel
										<input type="file" accept=".pdf,.docx,.txt" class="hidden" onchange={(e) => { const f = (e.currentTarget as HTMLInputElement).files?.[0]; if (f) void uploadFor(m, f); }} />
									</label>
								{/if}
								{#if hasDoc(m)}
									<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={bulkRunning} onclick={() => void openPreview(m)}><Eye size={14} /> Edit Artikel</button>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</section>

		<section class="app-card mt-4 p-4">
			<div class="flex flex-wrap items-end justify-between gap-4">
				<div class="flex flex-wrap gap-6">
					<div>
						<p class="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">BIAYA MEDIA ({cost.count} Media)</p>
						<p class="text-lg font-bold">{toRp(cost.media)}</p>
					</div>
					<span class="text-lg text-muted-foreground">+</span>
					<div>
						<p class="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">BIAYA PENULISAN AI</p>
						<p class="text-lg font-bold">{toRp(cost.writing)}</p>
					</div>
					<div>
						<p class="flex items-center gap-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">TOTAL TAGIHAN AKHIR</p>
						<p class="text-xl font-bold">{toRp(cost.total)}</p>
					</div>
				</div>
				<div class="flex flex-wrap items-center gap-3">
					<a class="app-btn app-btn-soft" href={`/klien/${brandUrl}/taut-pintar`}>Kembali</a>
					<button class="app-btn app-btn-primary" type="button" disabled={coverChecking} onclick={publish}>
						{#if coverChecking}<LoaderCircle size={16} class="animate-spin" />{:else}<Send size={16} />{/if}
						{coverChecking ? 'Memeriksa cover…' : 'Lanjut Tinjau & Terbit'}
					</button>
				</div>
			</div>
		</section>
	{/if}
</main>

{#if checklistItem}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<div class="app-card w-full max-w-md p-5">
			<p class="flex items-center gap-2 font-bold" style="color: var(--primary);"><ListChecks size={18} /> Cek Validasi Artikel</p>
			<p class="mt-1 text-sm text-muted-foreground">Ringkasan indikator artikel untuk media {checklistItem.name}.</p>
			<div class="mt-3 space-y-2 text-sm">
				<div class="flex items-start justify-between gap-3"><span class="text-muted-foreground">Status artikel</span><span class="text-right font-medium">{articleStatusOf(checklistItem)}</span></div>
				<div class="flex items-start justify-between gap-3"><span class="text-muted-foreground">Jumlah kata</span><span class="text-right font-medium">{checkStats ? `${checkStats.words.toLocaleString('id-ID')} kata` : 'Tidak ada'}</span></div>
				<div class="flex items-start justify-between gap-3"><span class="text-muted-foreground">Cover artikel</span><span class="text-right font-medium">{checkStats ? (checkStats.cover ? 'Ada' : 'Tidak ada') : 'Tidak ada'}</span></div>
				<div class="flex items-start justify-between gap-3"><span class="text-muted-foreground">Gambar di artikel</span><span class="text-right font-medium">{checkStats ? `${checkStats.images} gambar` : 'Tidak ada'}</span></div>
				<div class="flex items-start justify-between gap-3"><span class="text-muted-foreground">Tautan aktif</span><span class="text-right font-medium">{checkStats ? `${checkStats.links} tautan` : 'Tidak ada'}</span></div>
				<div class="flex items-start justify-between gap-3">
					<span class="text-muted-foreground">Kesesuaian media tujuan</span>
					<span class="text-right font-medium">{checkMediaType ? (checkMediaType === 'mainstream' ? 'Mainstream' : checkMediaType === 'blogger' ? 'Blogger' : checkMediaType) : 'Tidak ada indikator'}</span>
				</div>
			</div>
			{#if !checkStats}<p class="mt-2 text-xs text-muted-foreground">Artikel belum dibuat untuk media ini — indikator ditulis "Tidak ada".</p>{/if}
			<div class="mt-4 flex justify-end">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (checklistFor = null)}>Tutup</button>
			</div>
		</div>
	</div>
{/if}

{#if editor}
	{@const ed = editor}
	{#key ed.mediaId}
		<EditorModal
			mediaName={ed.mediaName}
			initialTitle={ed.title}
			initialHtml={ed.html}
			initialCoverUrl={ed.cover}
			initialCategory={ed.category}
			initialMetaDescription={ed.metaDescription}
			categoryOptions={placement.find((p) => p.media_id === ed.mediaId)?.categories ?? []}
			linkLimit={ed.linkLimit}
			allowEmbed={ed.allowEmbed}
			saving={editorSaving}
			saveError={editorError}
			onClose={() => { if (!editorSaving) editor = null; }}
			onSave={(title, html, coverUrl, category, metaDescription) => void saveEditor(title, html, coverUrl, category, metaDescription)}
			onUploadCover={uploadCover}
			regenDisabled={!editorMedia || bulkRunning || !canRegen(editorMedia)}
			regenTitle={editorMedia && hasDoc(editorMedia) && !canRegen(editorMedia) ? `Batas regenerasi tercapai (maks ${regenMax}x).` : undefined}
			onRegenerate={!manualFlow && editorMedia ? () => { const em = editorMedia; editor = null; void generateOne(em); } : undefined}
		/>
	{/key}
{/if}
