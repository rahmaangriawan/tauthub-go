<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Agent 2: The Matchmaker — kurasi media & alokasi anggaran.
	// Port app/klien/[brandId]/taut-pintar/[id]/matchmaker/client.tsx.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import {
		Check, ChevronDown, Coins, ExternalLink, FileText, Hand, Info,
		Search, SlidersHorizontal, Sparkles, Star, Target, X
	} from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import AgentSteps from '../../_agent-steps.svelte';
	import Select from '$lib/components/select.svelte';
	import Tooltip from '$lib/components/tooltip.svelte';
	import MediaDetailModal from '../../../media/_detail-modal.svelte';
	import TautBottomBar from '../../_bottom-bar.svelte';
	import { MANUAL_PROFILER_REASON, fmtIdr, rpCurrency, humanizeReason, fmtTrafficShort, formatTraffic, isCampaignDone } from '../../_shared';
	import { mediaDomainLabel } from '$lib/media/domain-label';
	import { mediaDistribution } from '$lib/media/distribution';
	import { getCategoryTone } from '$lib/media/category-tone';
	import { avgPublishHours, formatSla, slaDurationText } from '$lib/media/sla';
	import PageTitle from '$lib/components/page-title.svelte';
	// Kelas tabel bersama (port media-intelligence.module.css) — dipakai persis
	// seperti app lama yang mengimpor modul CSS Katalog Media.
	import '../../../media/media.css';

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);
	// Status kampanye dari GET /profile/draft. Final (terkirim/terjadwal/
	// terdistribusi) → halaman ini hanya boleh dilihat, bukan diubah.
	let campaignStatus = $state<string | null>(null);
	const viewOnly = $derived(isCampaignDone(campaignStatus));

	type PlacementRow = { media_id: string; name: string; score: number | null; status: string; reason: string | null };
	type MediaRow = {
		id: string;
		name: string;
		category: string | null;
		categories?: string[];
		media_type?: string | null;
		description: string | null;
		website_url: string | null;
		contoh_artikel_url?: string | null;
		bahasa_artikel?: string[] | null;
		integration_type: string;
		credit_cost: number;
		client_price: number;
		price: number;
		markup_percent?: number | null;
		pricing_type?: string | null;
		traffic_estimate: number;
		sw_visits?: number | null;
		media_tier: string | null;
		media_quality_score: number | null;
		effective_quality: number | null;
		quality_source: 'media' | 'tier_default' | null;
		health_score: number | null;
		health_breakdown?: Record<string, unknown> | null;
		health_computed_at?: string | null;
		published_count?: number | null;
		rating_avg?: number | null;
		rating_count?: number | null;
		dewan_pers_verified: boolean | null;
		domain_verified?: boolean | null;
		seo_da: number | null;
		seo_pa: number | null;
		seo_spam_score: number | null;
		seo_checked_at?: string | null;
		seo_ai_trust?: number | null;
		seo_ai_citation?: boolean | null;
		status: string;
		label_sponsor?: string | null;
		topik_terlarang?: string[] | null;
		max_links?: number | null;
		allow_hyperlink?: boolean | null;
		image_required?: boolean | null;
		publication_mode?: string | null;
		word_count_mode?: string | null;
		allow_embed?: boolean | null;
		auto_publish?: boolean | null;
		price_status?: string | null;
		created_at: string | null;
	};
	type MediaTableEntry = { media: MediaRow; matchPercent?: number | null; reason?: string | null };
	type DraftInfo = {
		campaignName: string;
		budgetMode: string | null;
		budgetAmount: number;
		walletAvailable: number;
		rateIdr: number;
	};

	let info = $state<DraftInfo | null>(null);
	let placement = $state<PlacementRow[]>([]);
	let library = $state<MediaRow[]>([]);
	// Lazyload direktori media: halaman pertama 20 baris, sisanya ditarik saat
	// sentinel di ujung tabel masuk viewport. Server tidak pernah mengirim
	// seluruh katalog sekaligus, dan DOM tidak menahan ratusan baris.
	const LIBRARY_PAGE = 20;
	let libraryTotal = $state(0);
	let libraryLoading = $state(false);
	let librarySentinel = $state<HTMLElement | null>(null);
	const libraryHasMore = $derived(library.length < libraryTotal);
	let selected = $state<Set<string>>(new Set());
	let manualPicks = $state<Set<string>>(new Set());
	let entryMode = $state<'ai' | 'manual' | null>(null);
	let tabChoice = $state<string | null>(null);
	let loading = $state(true);
	let running = $state(false);
	let saving = $state(false);
	let error = $state('');

	let q = $state('');
	let aiSectionOpen = $state(true);
	let typeFilter = $state('all');
	let sort = $state('recommended');
	let daMin = $state('');
	let spamMax = $state('');
	let priceMax = $state('');
	let detail = $state<MediaRow | null>(null);

	// Ada filter/pencarian aktif? Dipakai untuk memutuskan apakah halaman
	// direktori berikutnya perlu ikut ditarik supaya hasil tidak kosong palsu.
	const filterActive = $derived(
		Boolean(q.trim()) || typeFilter !== 'all' || Boolean(daMin) || Boolean(spamMax) || Boolean(priceMax)
	);

	// Popover kategori (+N) — fixed seperti portal Radix di app lama agar tidak
	// terpotong overflow kartu/tabel.
	let catPopoverId = $state<string | null>(null);
	let catPopoverPos = $state<{ left: number; top: number }>({ left: 0, top: 0 });
	let catPopoverCats = $state<string[]>([]);

	// Popover "Filter Lanjutan" (tab Manual) — mengambang, bukan panel inline.
	let advOpen = $state(false);
	let advBtn = $state<HTMLButtonElement | null>(null);
	let advPos = $state<{ left: number; top: number }>({ left: 0, top: 0 });

	// ?mode=manual dari popup Buat Kampanye.
	const manualMode = $derived(page.url.searchParams.get('mode') === 'manual');
	const manualFlow = $derived(entryMode ? entryMode === 'manual' : manualMode);
	const tab = $derived(tabChoice ?? (manualFlow ? 'custom' : 'ai'));
	const activePicks = $derived(tab === 'custom' ? manualPicks : selected);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			// Picks katalog sudah dikonsumsi oleh initializer manualPicks — hapus
			// agar kampanye manual berikutnya tidak mewarisi basi.
			try {
				sessionStorage.removeItem('taut_manual_picks');
			} catch {
				/* sessionStorage tidak tersedia */
			}
			setLoading(true);
			// Jalur manual ditentukan lebih dulu supaya Matchmaker AI tidak pernah
			// dijalankan — dan kredit tidak terpotong — untuk kampanye manual.
			const mode = manualMode ? 'manual' : await loadInfo();
			const manualPick = mode === 'manual';
			await Promise.all([loadPlacement(!manualPick, manualPick), loadLibrary()]);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	function setLoading(v: boolean) {
		loading = v;
	}

	async function loadInfo(): Promise<'ai' | 'manual' | null> {
		try {
			const res = await apiRequest(`${apiBase}/profile/draft`);
			if (res.ok) {
				const data = await res.json();
				info = {
					campaignName: data.campaign?.name ?? tr('Kampanye'),
					budgetMode: data.draft?.budget?.mode ?? null,
					budgetAmount: data.draft?.budget?.amount ?? 0,
					walletAvailable: data.wallet?.available ?? 0,
					rateIdr: data.wallet?.rate_idr ?? 0
				};
				campaignStatus = data.campaign?.taut_status ?? null;
				if (data.entry_mode === 'ai' || data.entry_mode === 'manual') entryMode = data.entry_mode;
				return data.entry_mode ?? null;
			}
		} catch {
			/* non-blocking */
		}
		return null;
	}

	async function loadPlacement(autoRun: boolean, manualPick = false) {
		running = autoRun;
		try {
			const res = await apiRequest(`${apiBase}/placement`);
			let data: { media: PlacementRow[] } = { media: [] };
			if (res.ok) data = await res.json();
			if (autoRun && data.media.length === 0) {
				// Matchmaker memanggil LLM (puluhan detik); timeout bawaan 12 dtk
				// memutus request dan konteks server batal di tengah jalan.
				const runRes = await apiRequest(`${apiBase}/placement`, { method: 'POST' }, 240_000);
				if (runRes.ok) {
					const runData = await runRes.json();
					data.media = runData.media ?? [];
				} else {
					error = await readApiError(runRes, 'AI Matchmaker gagal dijalankan. Coba lagi.');
				}
			}
			placement = data.media;
			const serverSelected = new Set(data.media.filter((m) => m.status === 'selected').map((m) => m.media_id));
			if (manualPick) {
				// Kampanye manual: daftar pilihan dibangun di tab Manual; `selected`
				// tidak dipakai sebagai sumber centang — `manualPicks` yang dipakai.
				manualPicks = new Set([...manualPicks, ...serverSelected]);
				selected = new Set();
			} else {
				selected = serverSelected;
			}
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal memuat rekomendasi.');
		} finally {
			running = false;
		}
	}

	async function loadLibrary(reset = false) {
		if (libraryLoading) return;
		if (!reset && library.length > 0 && !libraryHasMore) return;
		libraryLoading = true;
		const offset = reset ? 0 : library.length;
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/media?limit=${LIBRARY_PAGE}&offset=${offset}`);
			if (res.ok) {
				const data = await res.json();
				const rows: MediaRow[] = data.media ?? data.items ?? [];
				library = reset ? rows : [...library, ...rows];
				if (typeof data.total === 'number') libraryTotal = data.total;
			}
		} catch {
			/* non-blocking */
		} finally {
			libraryLoading = false;
		}
	}

	const mediaById = $derived.by(() => {
		const map = new Map<string, MediaRow>();
		for (const m of library) map.set(m.id, m);
		return map;
	});

	function toggleMedia(id: string) {
		if (viewOnly) return; // kampanye final: mode lihat saja
		const next = new Set(selected);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		selected = next;
	}

	function toggleManual(id: string) {
		if (viewOnly) return; // kampanye final: mode lihat saja
		const next = new Set(manualPicks);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		manualPicks = next;
	}

	function placeholderMedia(p: PlacementRow): MediaRow {
		return {
			id: p.media_id, name: p.name, category: null, media_type: null,
			description: null, website_url: null, integration_type: 'manual',
			credit_cost: 0, client_price: 0, price: 0, traffic_estimate: 0,
			media_tier: null, media_quality_score: null, effective_quality: null, quality_source: null,
			health_score: null, dewan_pers_verified: null,
			seo_da: null, seo_pa: null, seo_spam_score: null,
			status: 'active', created_at: null
		};
	}

	const recommendedCards = $derived.by(() => {
		let rows = placement.map((p) => ({ placement: p, media: mediaById.get(p.media_id) ?? placeholderMedia(p) }));
		if (q.trim()) {
			const needle = q.trim().toLowerCase();
			rows = rows.filter(
				(r) => r.media.name.toLowerCase().includes(needle) || (r.media.website_url ?? '').toLowerCase().includes(needle)
			);
		}
		if (typeFilter !== 'all') rows = rows.filter((r) => (r.media.media_type ?? '') === typeFilter);
		rows.sort((a, b) => {
			const selA = selected.has(a.placement.media_id) ? 0 : 1;
			const selB = selected.has(b.placement.media_id) ? 0 : 1;
			if (selA !== selB) return selA - selB;
			const remA = a.placement.status === 'removed' ? 1 : 0;
			const remB = b.placement.status === 'removed' ? 1 : 0;
			if (remA !== remB) return remA - remB;
			if (sort === 'price') return (a.media.credit_cost ?? 0) - (b.media.credit_cost ?? 0);
			if (sort === 'reach') return (b.media.traffic_estimate ?? 0) - (a.media.traffic_estimate ?? 0);
			return (b.placement.score ?? -1) - (a.placement.score ?? -1);
		});
		return rows;
	});

	const customCards = $derived.by(() => {
		const placementIds = new Set(placement.map((p) => p.media_id));
		// Hanya media lolos review admin (status "active") yang boleh dipilih.
		let rows = library.filter((m) => m.status === 'active');
		if (q.trim()) {
			const needle = q.trim().toLowerCase();
			rows = rows.filter(
				(m) =>
					m.name.toLowerCase().includes(needle) ||
					(m.category ?? '').toLowerCase().includes(needle) ||
					(m.website_url ?? '').toLowerCase().includes(needle)
			);
		}
		if (typeFilter !== 'all') rows = rows.filter((m) => (m.media_type ?? '') === typeFilter);
		if (daMin) rows = rows.filter((m) => (m.seo_da ?? 0) >= Number(daMin));
		if (spamMax) rows = rows.filter((m) => (m.seo_spam_score ?? 100) <= Number(spamMax));
		if (priceMax) rows = rows.filter((m) => (m.credit_cost ?? 0) <= Number(priceMax));
		rows.sort((a, b) => {
			const selA = manualPicks.has(a.id) ? 0 : 1;
			const selB = manualPicks.has(b.id) ? 0 : 1;
			if (selA !== selB) return selA - selB;
			if (sort === 'price') return (a.credit_cost ?? 0) - (b.credit_cost ?? 0);
			if (sort === 'reach') return (b.traffic_estimate ?? 0) - (a.traffic_estimate ?? 0);
			if (sort === 'recommended') return (placementIds.has(a.id) ? 0 : 1) - (placementIds.has(b.id) ? 0 : 1);
			return (b.health_score ?? 0) - (a.health_score ?? 0);
		});
		return rows;
	});

	// Baris tabel per bagian (parity MediaDirectoryTable: entri sama untuk AI,
	// removed, dan Manual — termasuk kolom AI Trust / AI Cited / Waktu Rilis).
	const recommendedEntries = $derived<MediaTableEntry[]>(
		recommendedCards
			.filter((r) => r.placement.status !== 'removed')
			.map((r) => ({ media: r.media, matchPercent: r.placement.score, reason: r.placement.reason }))
	);
	const removedEntries = $derived<MediaTableEntry[]>(
		recommendedCards
			.filter((r) => r.placement.status === 'removed')
			.map((r) => ({ media: r.media, matchPercent: r.placement.score, reason: r.placement.reason }))
	);
	const customEntries = $derived<MediaTableEntry[]>(customCards.map((m) => ({ media: m })));

	// Sentinel ujung tabel: tarik halaman berikutnya begitu terlihat. `library.length`
	// dibaca agar observer dipasang ulang setelah tiap halaman — kalau viewport
	// masih tinggi, sentinel tetap terlihat dan halaman berikutnya ikut ditarik
	// sampai viewport penuh atau katalog habis.
	$effect(() => {
		const el = librarySentinel;
		const loaded = library.length;
		if (!el || !libraryHasMore || loading) return;
		if (loaded === 0) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries.some((e) => e.isIntersecting)) void loadLibrary();
			},
			{ rootMargin: '240px' }
		);
		observer.observe(el);
		return () => observer.disconnect();
	});

	// Filter/pencarian aktif tapi hasil yang sudah dimuat masih di bawah satu
	// halaman → teruskan menarik halaman berikutnya (dengan jeda ketik) sampai
	// ketemu atau katalog habis, supaya pencarian tidak memberi hasil kosong palsu.
	$effect(() => {
		const matched = customCards.length;
		if (!filterActive || libraryLoading || !libraryHasMore || loading) return;
		if (matched >= LIBRARY_PAGE) return;
		const timer = setTimeout(() => void loadLibrary(), 250);
		return () => clearTimeout(timer);
	});

	// Media rekomendasi AI datang dari `placement`, barisnya dicocokkan ke
	// direktori. Kalau halamannya belum dimuat, barisnya jatuh ke placeholder
	// (DA/trafik/harga kosong) dan total biaya & reach di bar bawah jadi salah.
	// Jadi tarik halaman berikutnya sampai semua media placement ketemu.
	$effect(() => {
		if (loading || libraryLoading || !libraryHasMore) return;
		if (placement.length === 0) return;
		const missing = placement.some((p) => !mediaById.has(p.media_id));
		if (!missing) return;
		const timer = setTimeout(() => void loadLibrary(), 150);
		return () => clearTimeout(timer);
	});

	const summary = $derived.by(() => {
		let reach = 0;
		let cost = 0;
		for (const id of activePicks) {
			const m = mediaById.get(id);
			if (m) {
				reach += m.traffic_estimate ?? 0;
				cost += m.credit_cost ?? 0;
			}
		}
		return { count: activePicks.size, reach, cost };
	});

	async function finalize() {
		if (viewOnly) {
			await goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/ringkasan`);
			return;
		}
		const picks = tab === 'custom' ? manualPicks : selected;
		if (picks.size === 0) return;
		saving = true;
		error = '';
		try {
			// Tab Manual: media dari katalog belum punya baris di taut_campaign_media,
			// jadi harus di-ADD dulu. API membatasi 20 id per request → bertahap.
			if (tab === 'custom') {
				const ids = Array.from(picks);
				for (let i = 0; i < ids.length; i += 20) {
					const addRes = await apiRequest(`${apiBase}/placement`, {
						method: 'PUT',
						body: JSON.stringify({ add: ids.slice(i, i + 20) })
					});
					if (!addRes.ok) throw new Error(await readApiError(addRes, tr('Gagal menambahkan media manual.')));
				}
			}
			const res = await apiRequest(`${apiBase}/placement`, {
				method: 'PUT',
				body: JSON.stringify({ selected: Array.from(picks) })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan seleksi media.')));
			const advRes = await apiRequest(`${apiBase}/advance`, { method: 'POST', body: JSON.stringify({ target: 'placement' }) });
			if (!advRes.ok) {
				// 422 = kampanye sudah melewati tahap ini (mis. sudah terdistribusi).
				// Itu bukan kesalahan user — antar ke Ringkasan (mode lihat).
				if (advRes.status === 422) {
					await goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/ringkasan`);
					return;
				}
				throw new Error(await readApiError(advRes, tr('Gagal mengunci placement.')));
			}
			await goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/writer${manualFlow ? '?mode=manual' : ''}`);
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal melanjutkan.');
			saving = false;
		}
	}

	// SLA waktu terbit = rata-rata waktu penerbitan NYATA media
	// (health_breakdown.avg_publish_hours), bukan taksiran dari tipe integrasi.
	function getSla(m: MediaRow): string {
		return formatSla(avgPublishHours(m));
	}
	function slaTip(m: MediaRow): string {
		const hours = avgPublishHours(m);
		if (hours == null) {
			return tr('Belum ada riwayat penerbitan yang cukup untuk menghitung SLA media ini.');
		}
		return tr('SLA ± {waktu} — rata-rata waktu terbit nyata media ini: artikel tayang sekitar {waktu} setelah dikirim.')
			.replace('{waktu}', slaDurationText(hours));
	}
	/** Rating kecepatan terbit dari klien (bintang di dialog "Hubungi" pada
	 *  ringkasan), dirata-ratakan per media. Tanpa penilaian = tidak ditampilkan. */
	function mediaRating(m: MediaRow): { avg: number; count: number } | null {
		const avg = m.rating_avg;
		if (typeof avg !== 'number' || !(avg > 0)) return null;
		return { avg, count: m.rating_count ?? 0 };
	}
	function ratingTip(r: { avg: number; count: number }): string {
		const base = tr('Rating kecepatan terbit {avg}/5 dari klien yang menghubungi mitra.')
			.replace('{avg}', r.avg.toFixed(1));
		if (r.count <= 0) return base;
		return base + ' ' + tr('Berdasarkan {n} penilaian.').replace('{n}', String(r.count));
	}
	// DA & AI Trust hanya menampilkan hasil pemeriksaan nyata; jangan menurunkan
	// angka dari skor kualitas media karena itu bukan hasil pemeriksaan apa pun.
	function getDa(m: MediaRow): string | number {
		return m.seo_da != null ? m.seo_da : '—';
	}
	function getAiTrust(m: MediaRow): string | number {
		return m.seo_ai_trust != null ? m.seo_ai_trust : '—';
	}
	// Nama tampil = domain (bukan nama brand), huruf pertama dibesarkan.
	const domainLabel = (m: MediaRow) => mediaDomainLabel(m.website_url, m.name);
	// Rekam jejak distribusi media (jumlah artikel yang sudah terbit lewat media ini).
	const distribution = (m: MediaRow) => mediaDistribution(m.health_breakdown, m.published_count);

	function openCategoryPopover(event: MouseEvent, id: string, cats: string[]) {
		event.stopPropagation();
		if (catPopoverId === id) {
			catPopoverId = null;
			return;
		}
		const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
		const width = 280;
		const left = Math.min(Math.max(8, rect.left), Math.max(8, window.innerWidth - width - 8));
		catPopoverPos = { left, top: rect.bottom + 4 };
		catPopoverCats = cats;
		catPopoverId = id;
	}

	function openAdvFilter() {
		if (advOpen) {
			advOpen = false;
			return;
		}
		const rect = advBtn?.getBoundingClientRect();
		const width = 440;
		const left = rect
			? Math.min(Math.max(8, rect.right - width), Math.max(8, window.innerWidth - width - 8))
			: 8;
		advPos = { left, top: (rect?.bottom ?? 0) + 8 };
		advOpen = true;
	}

	$effect(() => {
		if (!catPopoverId) return;
		const close = () => (catPopoverId = null);
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape') close();
		};
		window.addEventListener('scroll', close, true);
		window.addEventListener('resize', close);
		window.addEventListener('keydown', onKey);
		return () => {
			window.removeEventListener('scroll', close, true);
			window.removeEventListener('resize', close);
			window.removeEventListener('keydown', onKey);
		};
	});

	$effect(() => {
		if (!advOpen) return;
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape') advOpen = false;
		};
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});
</script>

{#snippet mediaTable(entries: MediaTableEntry[], selectedIds: Set<string>, onToggle: (id: string) => void)}
	<div class="tableWrap">
		<table class="modernTable">
			<thead>
				<tr>
					<th class="modernTh" style="width:36px;padding-right:0">{tr('Pilih')}</th>
					<th class="modernTh">{tr('Nama Media')}</th>
					<th class="modernTh">{tr('Kategori')}</th>
					<th class="modernTh">{tr('Trafik (Est)')}</th>
					<th class="modernTh" style="text-align:center">DA</th>
						<th class="modernTh" style="text-align:center">
							<span class="thWithInfo">GEO / AI TRUST
								<Tooltip label={tr('Keterangan GEO dan AI Trust')} maxWidth={260}>
									{#snippet rich()}
										<strong>GEO</strong> — {tr('Centang biru menandai situs media ini disebut sebagai salah satu sumber pada jawaban AI Google.')}
										<br /><strong>AI Trust Score</strong> {tr('— Nilai 0-100: seberapa siap situs media ini dibaca dan dipercaya oleh mesin AI, dilihat dari kecepatan halaman dan kelengkapan data situsnya. Tanda — berarti kecepatannya belum bisa diukur.')}
									{/snippet}
									<button type="button" class="infoDot" aria-label={tr('Keterangan GEO dan AI Trust')} onclick={(e) => e.stopPropagation()}><Info size={14} /></button>
								</Tooltip>
							</span>
						</th>
					<th class="modernTh">
						<span class="thWithInfo">{tr('Waktu Rilis & SLA')}
							<Tooltip label={tr('Keterangan SLA waktu terbit')} maxWidth={280}>
								{#snippet rich()}
									<span class="tipStack">
										<strong>{tr('SLA Waktu Terbit')}</strong>
										<span>{tr('Rata-rata waktu terbit NYATA media ini, dihitung dari riwayat publikasinya: dari artikel dikirim sampai tayang. Media yang biasanya terbit 1 jam setelah dikirim akan tampil "SLA ± 1 jam".')}</span>
										<span>{tr('Tanda — berarti belum ada riwayat penerbitan yang cukup untuk menghitung SLA-nya.')}</span>
									</span>
								{/snippet}
								<button type="button" class="infoDot" aria-label={tr('Keterangan SLA waktu terbit')} onclick={(e) => e.stopPropagation()}><Info size={14} /></button>
							</Tooltip>
						</span>
					</th>
					<th class="modernTh">{tr('Harga')}</th>
					<th class="modernTh" style="text-align:right">{tr('Aksi')}</th>
				</tr>
			</thead>
			<tbody>
				{#each entries as entry (entry.media.id)}
					{@const m = entry.media}
					{@const traffic = formatTraffic(m.sw_visits, m.traffic_estimate)}
					{@const sla = getSla(m)}
					{@const rating = mediaRating(m)}
					{@const cats = Array.isArray(m.categories) && m.categories.length > 0 ? m.categories : m.category ? [m.category] : ['Umum']}
						{@const isSel = selectedIds.has(m.id)}
						{@const dist = distribution(m)}
					<tr class="modernTr" style="background: {isSel ? '#f8faff' : 'transparent'}; cursor:pointer;" onclick={() => onToggle(m.id)}>
						<td class="modernTd" style="width:36px;padding-right:0" onclick={(e) => e.stopPropagation()}>
							<input type="checkbox" class="modernCheckbox" checked={isSel} disabled={viewOnly} onchange={() => onToggle(m.id)} aria-label={`${tr('Pilih media')} ${m.name}`} />
						</td>
						<td class="modernTd">
							<div class="mediaRowCol">
								<div class="mediaTitleLine">
									{#if m.website_url}
										<a
											class="mediaHeading mediaHeadingLink"
											href={m.website_url.startsWith('http') ? m.website_url : `https://${m.website_url}`}
											target="_blank"
											rel="noreferrer noopener"
											title={`Buka ${domainLabel(m)}`}
											onclick={(e) => e.stopPropagation()}
										>{domainLabel(m)} <ExternalLink size={12} /></a>
									{:else}
										<span class="mediaHeading">{domainLabel(m)}</span>
									{/if}
									{#if m.domain_verified}
										<span class="verifiedTick" title={tr('Domain terverifikasi Tauthub')} aria-label={tr('Domain Terverifikasi')}>
											<svg width="15" height="15" viewBox="0 0 22 22" fill="none" aria-hidden="true">
												<path d="M20.396 11c-.018-.646-.215-1.275-.57-1.816-.354-.54-.852-.972-1.438-1.246.223-.607.27-1.264.14-1.897-.131-.634-.437-1.218-.882-1.687-.47-.445-1.053-.75-1.687-.882-.633-.13-1.29-.083-1.897.14-.273-.587-.704-1.086-1.245-1.44S11.647 1.62 11 1.604c-.646.017-1.273.213-1.813.568s-.969.854-1.24 1.44c-.608-.223-1.267-.272-1.902-.14-.635.13-1.22.436-1.69.882-.445.47-.749 1.055-.878 1.688-.13.633-.08 1.29.144 1.896-.587.274-1.087.705-1.443 1.245-.356.54-.555 1.17-.574 1.817.02.647.218 1.276.574 1.817.356.54.856.972 1.443 1.245-.224.606-.274 1.263-.144 1.896.13.634.433 1.218.877 1.688.47.443 1.054.747 1.687.878.633.132 1.29.084 1.897-.136.274.586.705 1.084 1.246 1.439.54.354 1.17.551 1.816.569.647-.016 1.276-.213 1.817-.567s.972-.854 1.245-1.44c.604.239 1.266.296 1.903.164.636-.132 1.22-.447 1.68-.907.46-.46.776-1.044.908-1.681s.075-1.299-.165-1.903c.586-.274 1.084-.705 1.439-1.246.354-.54.551-1.17.569-1.816z" fill="#1D9BF0"/>
												<path d="M9.662 14.85l-3.429-3.428 1.293-1.302 2.072 2.072 4.4-4.794 1.347 1.246z" fill="#FFFFFF"/>
											</svg>
										</span>
									{/if}
									{#if entry.matchPercent != null && entry.matchPercent > 0}
										<span class="badgeMatch" title={humanizeReason(entry.reason) || tr('Rekomendasi AI')}>★ {Math.round(entry.matchPercent)}% Match</span>
									{/if}
								</div>
								{#if m.contoh_artikel_url || dist.known}
									<div class="mediaMetaLine">
										{#if m.contoh_artikel_url}
											<a class="sampleDocLink" href={m.contoh_artikel_url} target="_blank" rel="noreferrer nofollow" onclick={(e) => e.stopPropagation()}><FileText size={13} /><span>Sample</span></a>
										{/if}
										{#if dist.known}
											<span
												class="distBadge"
												class:distBadgeMuted={dist.published === 0}
												title={tr('Jumlah artikel yang sudah terbit lewat media ini di seluruh platform Tauthub.')}
											><span>{dist.published > 0 ? `${dist.published} ${tr('artikel terdistribusi')}` : tr('Belum ada distribusi')}</span></span>
										{/if}
									</div>
								{/if}
								{#if m.dewan_pers_verified}
									<div class="dewanPersBadge"><Check size={13} /><span>{tr('Dewan Pers')}</span></div>
								{/if}
							</div>
						</td>
							<td class="modernTd" onclick={(e) => e.stopPropagation()}>
								<div class="categoryBadgeGroup">
									<span class="categoryPill categoryPill--{getCategoryTone(cats[0])}">{cats[0]}</span>
									{#if cats.length > 1}
									<button
										type="button"
										class="categoryPlusTag"
										title={`${tr('Kategori')}: ${cats.join(', ')}`}
										aria-label={`${cats.length} ${tr('kategori')}: ${cats.join(', ')}`}
										aria-expanded={catPopoverId === m.id}
										onclick={(e) => openCategoryPopover(e, m.id, cats)}
									>+{cats.length - 1}</button>
								{/if}
							</div>
						</td>
						<td class="modernTd">
							<div class="trafficStack">
								<div>
									<!-- Tanpa spasi antar-span: JSX di app lama membuang whitespace
									     antar-elemen, jadi jarak panah tren murni dari
									     margin-left 6px, bukan dari spasi teks. -->
									<span class="trafficMainVal">{traffic.text}</span>{#if traffic.trend}<span class={traffic.trend.isUp ? 'trafficTrendGreen' : 'trafficTrendRed'}>{traffic.trend.isUp ? `↑ ${traffic.trend.val}%` : `↓ ${traffic.trend.val}%`}</span>{/if}
								</div>
								<span class="trafficSourceText">{traffic.source}</span>
							</div>
						</td>
						<td class="modernTd" style="text-align:center"><span class="scoreBoxPill">{getDa(m)}</span></td>
						<td class="modernTd" style="text-align:center">
							<div class="flex flex-col items-center gap-1">
								<span class="scoreBoxPill">{getAiTrust(m)}</span>
								<!-- Indikator GEO = centang AI Cited; disembunyikan bila tidak ada. -->
								{#if m.seo_ai_citation === true}
									<span class="aiCitationBadge" title={tr('Sumber AI')} aria-label={tr('Sumber AI')}><Check size={11} /></span>
								{/if}
							</div>
						</td>
						<td class="modernTd">
							<!-- SLA = rata-rata waktu terbit nyata media (bukan label taksiran). -->
							<div class="flex flex-col items-start gap-1">
								<Tooltip label={slaTip(m)} maxWidth={280}>
									<span class="slaPill">{sla}</span>
								</Tooltip>
								{#if rating}
									<span class="ratingPill" title={ratingTip(rating)}>
										<Star size={11} fill="currentColor" strokeWidth={0} />
										{rating.avg.toFixed(1)}/5
									</span>
								{/if}
							</div>
						</td>
						<td class="modernTd"><span class="priceBoldText">Rp {fmtIdr(m.client_price ?? m.price ?? 0)}</span></td>
						<td class="modernTd" style="text-align:right" onclick={(e) => e.stopPropagation()}>
							<button class="viewDetailBtn" type="button" onclick={(e) => { e.stopPropagation(); detail = m; }}>{tr('Detail')}</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/snippet}

<PageTitle title={tr('Taut Pintar — The Matchmaker')} />

<main>
	{#if viewOnly}
		<div class="mb-3 flex flex-wrap items-center gap-2 rounded-xl border px-4 py-2.5 text-[13px]" style="border-color: var(--border); background: var(--muted);">
			<span>{tr('Kampanye ini sudah selesai — mode lihat saja. Isinya tidak bisa diubah lagi.')}</span>
			<a class="font-semibold underline" href={`/klien/${brandUrl}/taut-pintar/${campaignId}/ringkasan`}>{tr('Buka Ringkasan')}</a>
		</div>
	{/if}
	<div class="flex flex-wrap items-center justify-between gap-3">
		<div class="flex items-center gap-3">
			<div class="flex h-11 w-11 items-center justify-center rounded-xl" style="background-color: var(--primary);">
				<Target size={22} style="color: #fff;" />
			</div>
			<div>
				<p class="font-bold leading-tight">Agent 2: The Matchmaker</p>
				<p class="text-sm leading-tight text-muted-foreground">{tr('Kurasi Media & Alokasi Anggaran')}</p>
			</div>
		</div>
		<div class="inline-flex items-center gap-1.5 rounded-full px-3.5 py-1.5 text-xs font-bold" style="background-color: var(--success-soft); color: var(--success);">
			<span>{rpCurrency((info?.walletAvailable ?? 0) * (info?.rateIdr ?? 0))}</span>
		</div>
	</div>

	<div class="mt-3">
		<AgentSteps current={1} />
	</div>

	{#if error}
		<div class="errorBanner mt-3 flex items-start gap-2">
			<span class="flex-1">{error}</span>
			<button class="rounded p-0.5" type="button" aria-label={tr('Tutup error')} onclick={() => (error = '')}><X size={12} /></button>
		</div>
	{/if}

	<!-- Tabs -->
	<div class="mt-4 flex gap-1 border-b">
		<button
			class="inline-flex items-center gap-1.5 border-b-2 px-3 py-2 text-sm font-medium"
			style="border-color: {tab === 'ai' ? 'var(--primary)' : 'transparent'}; color: {tab === 'ai' ? 'var(--primary)' : 'inherit'}; {manualFlow ? 'opacity:.42; cursor:not-allowed; filter:grayscale(100%);' : ''}"
			type="button"
			disabled={manualFlow}
			title={manualFlow ? tr('Rekomendasi AI dinonaktifkan pada kampanye pemilihan manual') : undefined}
			onclick={() => { if (!manualFlow || tab === 'custom') tabChoice = 'ai'; }}
		>
			<Sparkles size={13} /> {tr('Rekomendasi AI Pintar')}
		</button>
		<button
			class="inline-flex items-center gap-1.5 border-b-2 px-3 py-2 text-sm font-medium"
			style="border-color: {tab === 'custom' ? 'var(--primary)' : 'transparent'}; color: {tab === 'custom' ? 'var(--primary)' : 'inherit'};"
			type="button"
			onclick={() => (tabChoice = 'custom')}
		>
			<Hand size={13} /> {tr('Manual')}
		</button>
	</div>

	<div class="mt-2 mb-2.5 flex flex-wrap items-center gap-4 text-sm">
		<span class="inline-flex items-center gap-1 text-muted-foreground">
			<Target size={14} />{tr('Kampanye:')}<strong style="color: var(--foreground);">{info?.campaignName ?? '—'}</strong>
		</span>
		{#if tab === 'ai'}
			<span class="inline-flex items-center gap-1 text-muted-foreground">
				<Coins size={14} style="color: var(--success);" />
				{tr('Mode:')}
				<strong style="color: var(--foreground);">
					{info?.budgetMode === 'optimal'
						? tr('Optimal (Strategis)')
						: info?.budgetMode === 'efficient' && (info?.budgetAmount ?? 0) > 0
							? `${tr('Efisien')} (${tr('Max')} ${rpCurrency(info.budgetAmount * (info?.rateIdr ?? 0))})`
							: tr('Efisien')}
				</strong>
			</span>
		{/if}
	</div>

	{#if tab === 'ai'}
		<div class="app-card mt-3 flex flex-wrap items-center gap-2 p-3">
			<label class="app-input flex min-w-[200px] flex-1 items-center gap-2 !py-1.5">
				<Search size={14} class="text-muted-foreground" />
				<input class="w-full bg-transparent text-sm outline-none" placeholder={tr('Cari domain media atau niche…')} aria-label={tr('Cari domain media atau niche')} bind:value={q} />
			</label>
			<Select
				bind:value={typeFilter}
				ariaLabel={tr('Filter tipe media')}
				placeholder={tr('Semua Tipe Media')}
				minWidth={170}
				align="end"
				options={[
					{ value: 'all', label: tr('Semua Tipe Media') },
					{ value: 'mainstream', label: tr('Media Online') },
					{ value: 'blogger', label: 'Blog' }
				]}
			/>
			<Select
				bind:value={sort}
				ariaLabel={tr('Urutkan')}
				placeholder={tr('Urutkan')}
				minWidth={170}
				align="end"
				options={[
					{ value: 'recommended', label: 'AI Match Score' },
					{ value: 'match', label: tr('Match % Tertinggi') },
					{ value: 'price', label: tr('Harga Terendah') },
					{ value: 'reach', label: tr('Est. Reach Tertinggi') }
				]}
			/>
		</div>

		{#if loading || running}
			<div class="app-card mt-3 flex flex-col items-center gap-2 px-4 py-10 text-center text-sm text-muted-foreground">
				<div class="h-6 w-6 animate-spin rounded-full border-2 border-current border-t-transparent"></div>
				<p>{running ? tr('AI Matchmaker sedang mencocokkan media dengan kebutuhan kampanye…') : tr('Memuat rekomendasi…')}</p>
			</div>
		{:else if recommendedCards.length === 0}
			<div class="app-card mt-3 px-4 py-10 text-center text-sm text-muted-foreground">{tr('Belum ada rekomendasi.')}{#if !viewOnly}<button class="ml-1 font-medium" style="color: var(--primary);" type="button" onclick={() => void loadPlacement(true)}>{tr('Jalankan ulang Matchmaker')}</button>{/if}
			</div>
		{:else}
			<div class="aiSectionGroup mt-3">
				<button class="aiSectionHeader" type="button" aria-expanded={aiSectionOpen} onclick={() => (aiSectionOpen = !aiSectionOpen)}>
					<span class="aiSectionBadge">
						<Sparkles size={13} />
						{tr('Pilihan AI')}
					</span>
					<span class="aiSectionMeta">
						{recommendedEntries.length} {tr('rekomendasi')}
						<ChevronDown size={16} style="transform: {aiSectionOpen ? 'rotate(180deg)' : 'none'}; transition: transform .15s;" />
					</span>
				</button>
				{#if aiSectionOpen}
					{@render mediaTable(recommendedEntries, selected, toggleMedia)}
				{/if}
			</div>

			<!-- Media yang ditolak AI (removed) — di LUAR wrapper buka-tutup -->
			{#if !running && removedEntries.length > 0}
				<div class="removedSection">
					<span class="removedLabel">{tr('Tidak direkomendasikan AI')}</span>
					{@render mediaTable(removedEntries, selected, toggleMedia)}
				</div>
			{/if}
		{/if}
	{:else}
		<div class="app-card mt-3 flex flex-wrap items-center gap-2 p-3">
			<label class="app-input flex min-w-[200px] flex-1 items-center gap-2 !py-1.5">
				<Search size={14} class="text-muted-foreground" />
				<input class="w-full bg-transparent text-sm outline-none" placeholder={tr('Cari domain media atau niche…')} aria-label={tr('Cari domain media atau niche')} bind:value={q} />
			</label>
			<Select
				bind:value={typeFilter}
				ariaLabel={tr('Filter tipe media')}
				placeholder={tr('Semua Tipe Media')}
				minWidth={170}
				align="end"
				options={[
					{ value: 'all', label: tr('Semua Tipe Media') },
					{ value: 'mainstream', label: tr('Media Online') },
					{ value: 'blogger', label: 'Blog' }
				]}
			/>
			<Select
				bind:value={sort}
				ariaLabel={tr('Urutkan')}
				placeholder={tr('Urutkan')}
				minWidth={170}
				align="end"
				options={[
					{ value: 'recommended', label: 'AI Match Score' },
					{ value: 'match', label: tr('Match % Tertinggi') },
					{ value: 'price', label: tr('Harga Terendah') },
					{ value: 'reach', label: tr('Est. Reach Tertinggi') }
				]}
			/>
			<button bind:this={advBtn} class="app-btn app-btn-soft" type="button" aria-expanded={advOpen} onclick={openAdvFilter}>
				<SlidersHorizontal size={14} /> {tr('Filter Lanjutan')}
			</button>
		</div>

		{#if advOpen}
			<button type="button" class="advFilterBackdrop" aria-label={tr('Tutup filter')} onclick={() => (advOpen = false)}></button>
			<div class="advFilterPanel" style="left: {advPos.left}px; top: {advPos.top}px;">
				<div class="advFilterCard">
					<p class="text-xs font-bold">{tr('Filter Lanjutan')}</p>
					<label class="advFilterField">
						<span>{tr('DA Score (Minimal)')}</span>
						<input class="app-input w-full" type="number" min="0" max="100" placeholder="0" bind:value={daMin} />
					</label>
					<label class="advFilterField">
						<span>{tr('Spam Score (Maksimal)')}</span>
						<input class="app-input w-full" type="number" min="0" max="100" placeholder="15" bind:value={spamMax} />
					</label>
					<label class="advFilterField">
						<span>{tr('Harga (Maksimal)')}</span>
						<input class="app-input w-full" type="number" min="0" placeholder={tr('Tanpa batas')} bind:value={priceMax} />
					</label>
					<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={() => { daMin = ''; spamMax = ''; priceMax = ''; }}>{tr('Reset Filter')}</button>
				</div>
			</div>
		{/if}

		{#if customCards.length === 0}
			<div class="app-card mt-3 px-4 py-10 text-center text-sm text-muted-foreground">
				{libraryHasMore ? tr('Memuat media…') : tr('Tidak ada media yang cocok dengan filter.')}
			</div>
		{:else}
			{@render mediaTable(customEntries, manualPicks, toggleManual)}
			{#if libraryHasMore}
				<div
					bind:this={librarySentinel}
					class="px-4 py-4 text-center text-xs text-muted-foreground"
					role="status"
					aria-live="polite"
				>
					{#if libraryLoading}{tr('Memuat media…')}{/if}
				</div>
			{/if}
		{/if}
	{/if}

	<TautBottomBar>
		<div class="bottomStats">
			<div class="bottomStat">
				<span class="bottomStatLabel">{tr('Media Terpilih')}</span>
				<span class="bottomStatValue">{summary.count} {tr('Domain')}</span>
			</div>
			<div class="bottomStat">
				<span class="bottomStatLabel">Est. Reach</span>
				<span class="bottomStatValue" style="color: var(--primary);">{fmtTrafficShort(summary.reach)}+</span>
			</div>
			<div class="bottomStat">
				<span class="bottomStatLabel">{tr('Total Biaya Kampanye')}</span>
				<span class="bottomStatValue" style="font-size:20px;">{(info?.rateIdr ?? 0) > 0 ? rpCurrency(summary.cost * (info?.rateIdr ?? 0)) : '—'}</span>
			</div>
		</div>
		<div class="bottomActions">
			<a class="app-btn app-btn-soft" href={`/klien/${brandUrl}/campaigns`}>{tr('Kembali ke daftar kampanye')}</a>
			{#if viewOnly}
				<a class="app-btn app-btn-primary" href={`/klien/${brandUrl}/taut-pintar/${campaignId}/ringkasan`}>{tr('Lihat Ringkasan →')}</a>
			{:else}
				<button class="app-btn app-btn-primary" type="button" disabled={saving || summary.count === 0} onclick={finalize}>
					{#if saving}<div class="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent"></div>{/if}
					{saving ? tr('Menyimpan…') : tr('Kunci & Lanjutkan Penulisan →')}
				</button>
			{/if}
		</div>
	</TautBottomBar>
</main>

	{#if detail}
		{@const d = detail}
		<MediaDetailModal
			media={d}
			onclose={() => (detail = null)}
			onProceed={() => {
				if (tab === 'custom') {
					manualPicks = new Set([...manualPicks, d.id]);
				} else {
					selected = new Set([...selected, d.id]);
				}
				detail = null;
				void finalize();
			}}
		/>
	{/if}

<!-- Popover kategori (+N) — fixed agar tidak terpotong kartu/wrapper tabel. -->
{#if catPopoverId}
	<button type="button" class="categoryPopoverBackdrop" aria-label={tr('Tutup daftar kategori')} onclick={() => (catPopoverId = null)}></button>
	<div class="categoryPopover" role="tooltip" style="left: {catPopoverPos.left}px; top: {catPopoverPos.top}px;">
		<div class="categoryPopoverTitle">{tr('Semua Kategori')}</div>
			<div class="categoryPopoverList">
				{#each catPopoverCats as c (c)}<span class="categoryPill categoryPill--{getCategoryTone(c)}">{c}</span>{/each}
			</div>
	</div>
{/if}

<style>
	/* Isi bottom bar tetap (posisi/spacer disediakan TautBottomBar). */
	.bottomStats {
		display: flex;
		align-items: center;
		gap: 36px;
		flex-wrap: wrap;
	}
	.bottomStat {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.bottomStatLabel {
		font-size: 10.5px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--muted-foreground);
	}
	.bottomStatValue {
		font-size: 18px;
		font-weight: 800;
		color: var(--foreground);
		font-variant-numeric: tabular-nums;
	}
	.bottomStatSub {
		font-size: 11.5px;
		font-weight: 600;
		color: var(--warning);
	}
	.bottomActions {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	@media (max-width: 767px) {
		.bottomStats {
			justify-content: space-between;
			width: 100%;
		}
		.bottomActions {
			flex-direction: column;
			align-items: stretch;
			width: 100%;
		}
		.bottomActions :global(.app-btn) {
			width: 100%;
		}
	}

	/* Badge "% Match" (parity .badgeMatch matchmaker.module.css). */
	.badgeMatch {
		font-size: 11px;
		font-weight: 700;
		color: var(--primary);
		padding: 2px 8px;
		background: var(--accent);
		border: 1px solid var(--border);
		border-radius: 6px;
		white-space: nowrap;
	}

	/* Section "Pilihan AI" — kotak biru bisa buka-tutup (parity .aiSectionGroup). */
	.aiSectionGroup {
		border: 2px solid var(--primary);
		border-radius: 14px;
		background: var(--accent);
		overflow: hidden;
	}
	.aiSectionHeader {
		width: 100%;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 12px 16px;
		background: color-mix(in srgb, var(--primary) 10%, var(--card));
		border: none;
		cursor: pointer;
		font-family: inherit;
	}
	.aiSectionHeader:hover {
		background: color-mix(in srgb, var(--primary) 16%, var(--card));
	}
	.aiSectionBadge {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 13px;
		font-weight: 750;
		color: var(--primary);
	}
	.aiSectionMeta {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: 12.5px;
		color: var(--muted-foreground);
	}

	/* Media yang ditolak AI — di luar wrapper, tampil redup. */
	.removedSection {
		margin-top: 14px;
	}
	.removedLabel {
		display: inline-flex;
		align-items: center;
		font-size: 12.5px;
		font-weight: 700;
		color: var(--muted-foreground);
		letter-spacing: 0.02em;
		text-transform: uppercase;
		margin-bottom: 8px;
	}

	/* Field di popover Filter Lanjutan. */
	.advFilterField {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: 12.5px;
		font-weight: 600;
		color: var(--muted-foreground);
	}

</style>
