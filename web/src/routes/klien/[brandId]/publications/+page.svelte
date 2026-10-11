<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Status Publikasi klien — port PublicationsSection (distribution.tsx).
	// Legacy memakai CARD LIST (bukan tabel): tiap baris = [cover][utama][sisi].
	// Layout: header (+eyebrow PUBLICATIONS +tab) → toolbar (cari + STATUS/MEDIA/
	// KATEGORI + urut) → banner error → daftar baris → footer paginasi → modal
	// (detail, batalkan, tracking) → toast "macet".
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import {
		Calendar, ChartLine, CircleAlert, CircleCheck, Coins, ExternalLink, FileText, Image as ImageIcon, LoaderCircle, PencilLine, RotateCcw, Search, X
	} from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import Select from '$lib/components/select.svelte';
	import Tooltip from '$lib/components/tooltip.svelte';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}`);

	type Pub = {
		id: string; document_id: string; media_id: string; status: string; client_price: number;
		publish_status?: string | null; publish_progress?: number | null;
		published_at: string | null; published_url: string | null; featured_image_url?: string | null;
		document_title: string | null; media_name: string | null; media_category?: string | null;
		media_categories?: string[] | null; media_status?: string | null; order_code?: string | null;
		created_at: string; cancel_reason?: string | null;
		pr_analysis?: {
			message?: { score: number | null; status?: string | null } | null;
			// sentiment berupa objek {sentiment, confidence, status} — parity
			// getPublicationPrAnalysis di app lama, bukan string.
			sentiment?: { sentiment: string; confidence: number | null; status?: string | null } | null;
			verification?: { status: string; compliance_score: number | null } | null;
		} | null;
	};
	type Detail = Pub & {
		partner_draft_edits?: unknown; timeline?: unknown;
		verification?: { status: string; compliance_score: number | null; error_message?: string | null } | null;
	};

	let rows = $state<Pub[]>([]);
	let total = $state(0);
	let pageNum = $state(1);
	const pageSize = 20;
	let query = $state('');
	let debouncedQuery = $state('');
	let statusFilter = $state('');
	let mediaFilter = $state('');
	let categoryFilter = $state('');
	let sortOrder = $state<'newest' | 'oldest' | 'price_desc' | 'price_asc'>('newest');
	let view = $state<'publications' | 'plans'>('publications');
	let loading = $state(true);
	let err = $state('');
	let rateIdr = $state(1000);
	let mediaCatalog = $state<Array<{ name: string; category: string | null }>>([]);

	// Modal
	let detail = $state<Detail | null>(null);
	let detailBusy = $state(false);
	let cancelTarget = $state<Pub | null>(null);
	let cancelReason = $state('');
	let cancelBusy = $state(false);
	let trackingTarget = $state<Pub | null>(null);
	let tracking = $state<{ tracking_path: string; destination_url: string | null; status: string } | null>(null);
	let trackingCounts = $state<Record<string, number>>({});
	let destinationUrl = $state('');
	let trackingBusy = $state(false);

	let imgError = $state<Record<string, boolean>>({});
	let confirmReset = $state<Pub | null>(null);

	// Debounce pencarian (parity 300ms).
	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	$effect(() => {
		const v = query;
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => {
			debouncedQuery = v.trim();
			pageNum = 1;
			void load();
		}, 300);
		return () => clearTimeout(searchTimer);
	});

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) {
				await goto('/login');
				return;
			}
			await Promise.all([load(), loadWallet(), loadMediaCatalog()]);
		})();
		const onExpired = () => void goto('/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		loading = true;
		err = '';
		try {
			const params = new URLSearchParams({ limit: String(pageSize), offset: String((pageNum - 1) * pageSize) });
			if (debouncedQuery) params.set('q', debouncedQuery);
			const res = await apiRequest(`${apiBase}/publications?${params}`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat publikasi.')));
			const data = (await res.json()) as { publications?: Pub[]; total?: number };
			rows = data.publications ?? [];
			total = data.total ?? 0;
		} catch (reason) {
			err = reason instanceof Error ? reason.message : tr('Gagal memuat publikasi.');
		} finally {
			loading = false;
		}
	}

	async function loadWallet() {
		try {
			const res = await apiRequest(`${apiBase}/credits?limit=1`);
			if (!res.ok) return;
			const data = (await res.json()) as { tc_config?: { rate_idr?: number } | null };
			if (data.tc_config?.rate_idr) rateIdr = data.tc_config.rate_idr;
		} catch { /* fallback rate 1000 (parity) */ }
	}

	async function loadMediaCatalog() {
		try {
			const res = await apiRequest(`${apiBase}/media?limit=100`);
			if (!res.ok) return;
			const data = (await res.json()) as { media?: Array<{ name: string; category: string | null }> };
			mediaCatalog = data.media ?? [];
		} catch { /* opsi filter tetap dari baris terlihat */ }
	}

	// ── Status (parity PUB_STATUS_CONFIG) ──────────────────────────────
	const PUB_STATUS_CONFIG: Record<string, { label: string; dot: string; text: string; bg: string }> = {
		published: { label: 'Published', dot: 'var(--success)', text: 'var(--success)', bg: 'var(--success-soft)' },
		completed: { label: 'Completed', dot: 'var(--success)', text: 'var(--success)', bg: 'var(--success-soft)' },
		processing: { label: 'Processing', dot: 'var(--warning)', text: 'var(--warning)', bg: 'var(--warning-soft)' },
		queued: { label: 'Queued', dot: 'var(--warning)', text: 'var(--warning)', bg: 'var(--warning-soft)' },
		scheduled: { label: 'Scheduled', dot: 'var(--primary)', text: 'var(--primary)', bg: 'var(--accent)' },
		failed: { label: 'Failed', dot: 'var(--destructive)', text: 'var(--destructive)', bg: 'color-mix(in srgb, var(--destructive) 12%, transparent)' },
		rejected: { label: 'Rejected', dot: 'var(--destructive)', text: 'var(--destructive)', bg: 'color-mix(in srgb, var(--destructive) 12%, transparent)' },
		draft: { label: 'Draft', dot: 'var(--muted-foreground)', text: 'var(--muted-foreground)', bg: 'var(--muted)' },
		cancelled: { label: 'Cancelled', dot: 'var(--muted-foreground)', text: 'var(--muted-foreground)', bg: 'var(--muted)' },
		revision: { label: tr('Perlu Revisi'), dot: 'var(--warning)', text: 'var(--warning)', bg: 'var(--warning-soft)' }
	};
	const STATUS_FALLBACK = { dot: 'var(--muted-foreground)', text: 'var(--muted-foreground)', bg: 'var(--muted)' };
	function statusCfg(status: string) {
		const c = PUB_STATUS_CONFIG[status.toLowerCase()];
		return c ?? { ...STATUS_FALLBACK, label: status };
	}

	// Publikasi yang sudah diterima media tetapi diterbitkan sebagai DRAF di
	// website mitra belum bisa dibuka publik. Statusnya ditulis "Disetujui"
	// (bukan "Published") dan alasannya muncul saat dihover.
	const isDraftVisibility = (row: { status: string; publish_status?: string | null }) =>
		row.status === 'published' && row.publish_status === 'draft';
	const draftVisibilityTip = $derived(
		tr(
			'Artikel sudah diterima & diterbitkan media, tetapi masih berstatus draf di website mitra — belum bisa dibuka publik. Tautan tayang muncul otomatis setelah mitra mempublikasikannya.'
		)
	);
	const draftVisibilityCfg = $derived({
		label: tr('Disetujui'),
		dot: 'var(--warning)',
		text: 'var(--warning)',
		bg: 'var(--warning-soft)'
	});

	const CANCELLABLE = new Set(['pending', 'queued', 'waiting_partner', 'partner_rejected', 'processing', 'retrying']);
	const RESETTABLE = new Set(['processing', 'failed', 'retrying', 'queued', 'cancelled']);
	const IN_FLIGHT = ['queued', 'processing', 'retrying'];

	/** Tampilan berbahasa rupiah: nilai rupiah ditampilkan apa adanya. */
	function tcLabel(rupiah: number, _rate: number): string {
		return `Rp ${Math.max(0, Math.round(Number(rupiah) || 0)).toLocaleString('id-ID')}`;
	}

	const fmtDate = (v: string | null) =>
		v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '—';
	const SENTIMENT_LABEL: Record<string, string> = { positive: 'Positif', neutral: 'Netral', negative: 'Negatif' };

	function categoriesOf(p: Pub): string[] {
		if (Array.isArray(p.media_categories) && p.media_categories.length > 0) return p.media_categories;
		return p.media_category ? [p.media_category] : [];
	}
	function progressOf(p: Pub): number {
		return Math.min(100, Math.max(5, p.publish_progress ?? 10));
	}
	function progressLabel(p: Pub): string {
		const base = p.status === 'processing' ? 'Mengirim ke media…' : p.status === 'retrying' ? 'Mencoba ulang…' : 'Mengantre…';
		return `${base} ${Math.min(100, p.publish_progress ?? 10)}%`;
	}

	// Filter & sort sisi klien pada halaman terlihat (parity: hanya q server-side).
	const filtered = $derived(
		rows.filter((r) => {
			if (statusFilter && r.status.toLowerCase() !== statusFilter.toLowerCase()) return false;
			if (mediaFilter && (r.media_name ?? '').toLowerCase() !== mediaFilter.toLowerCase()) return false;
			if (categoryFilter) {
				const cats = categoriesOf(r);
				if (!cats.some((c) => c.toLowerCase() === categoryFilter.toLowerCase())) return false;
			}
			return true;
		})
	);
	const sorted = $derived.by(() => {
		const list = [...filtered];
		const t = (v: string | null) => (v ? new Date(v).getTime() : 0);
		list.sort((a, b) => {
			if (sortOrder === 'oldest') return t(a.created_at) - t(b.created_at);
			if (sortOrder === 'price_desc') return (b.client_price || 0) - (a.client_price || 0);
			if (sortOrder === 'price_asc') return (a.client_price || 0) - (b.client_price || 0);
			return t(b.created_at) - t(a.created_at);
		});
		return list;
	});

	const mediaOptions = $derived(
		[...new Set([...rows.map((r) => r.media_name).filter((n): n is string => Boolean(n)), ...mediaCatalog.map((m) => m.name)].filter(Boolean))].sort()
	);
	const categoryOptions = $derived(
		[
			...new Set([
				...rows.flatMap((r) => categoriesOf(r)),
				...mediaCatalog.map((m) => m.category).filter((c): c is string => Boolean(c))
			])
		].sort()
	);
	const totalPages = $derived(Math.max(1, Math.ceil(total / pageSize)));

	// ── Aksi ───────────────────────────────────────────────────────────
	async function openDetail(row: Pub) {
		detailBusy = true;
		try {
			const res = await apiRequest(`${apiBase}/publications/${row.id}`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat detail publikasi.')));
			const data = (await res.json()) as { publication?: Detail } & Detail;
			detail = (data.publication ?? data) as Detail;
		} catch (reason) {
			err = reason instanceof Error ? reason.message : tr('Gagal memuat detail publikasi.');
		} finally {
			detailBusy = false;
		}
	}

	async function submitCancel() {
		if (!cancelTarget) return;
		cancelBusy = true;
		try {
			const res = await apiRequest(`${apiBase}/publications/${cancelTarget.id}/cancel`, {
				method: 'POST',
				body: JSON.stringify({ reason: cancelReason.trim() || null })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal membatalkan publikasi.')));
			cancelTarget = null;
			cancelReason = '';
			detail = null;
			await load();
		} catch (reason) {
			err = reason instanceof Error ? reason.message : tr('Gagal membatalkan publikasi.');
		} finally {
			cancelBusy = false;
		}
	}

	async function retryPublication(id: string) {
		try {
			const res = await apiRequest(`${apiBase}/publications/${id}/retry`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal mencoba ulang publikasi.')));
			await load();
		} catch (reason) {
			err = reason instanceof Error ? reason.message : tr('Gagal mencoba ulang publikasi.');
		}
	}

	async function resetToApproval(p: Pub) {
		confirmReset = null;
		try {
			const res = await apiRequest(`${apiBase}/publications/${p.id}/reset-approval`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal mengembalikan publikasi ke persetujuan.')));
			await load();
		} catch (reason) {
			err = reason instanceof Error ? reason.message : tr('Gagal mengembalikan publikasi ke persetujuan.');
		}
	}

	async function openTracking(row: Pub) {
		trackingTarget = row;
		tracking = null;
		trackingCounts = {};
		destinationUrl = '';
		try {
			const res = await apiRequest(`${apiBase}/publications/${row.id}/tracking`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat data pelacakan.')));
			const data = (await res.json()) as {
				tracking?: { tracking_path: string; destination_url: string | null; status: string } | null;
				counts?: Record<string, number>;
			};
			tracking = data.tracking ?? null;
			trackingCounts = data.counts ?? {};
			destinationUrl = data.tracking?.destination_url ?? row.published_url ?? '';
		} catch (reason) {
			err = reason instanceof Error ? reason.message : tr('Gagal memuat data pelacakan.');
		}
	}

	async function saveTracking() {
		if (!trackingTarget) return;
		trackingBusy = true;
		try {
			const res = await apiRequest(`${apiBase}/publications/${trackingTarget.id}/tracking`, {
				method: 'POST',
				body: JSON.stringify({ destination_url: destinationUrl.trim() || null })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan tujuan pelacakan.')));
			const data = (await res.json()) as { tracking?: { tracking_path: string; destination_url: string | null; status: string } };
			tracking = data.tracking ?? tracking;
		} catch (reason) {
			err = reason instanceof Error ? reason.message : tr('Gagal menyimpan tujuan pelacakan.');
		} finally {
			trackingBusy = false;
		}
	}

	const trackUrl = $derived(
		tracking ? `${typeof window !== 'undefined' ? window.location.origin : ''}${tracking.tracking_path}` : ''
	);
	const trackTotal = $derived(Object.values(trackingCounts).reduce((s, n) => s + n, 0));
</script>

<PageTitle title={tr("Publikasi")} />

<!-- Badge status publikasi. Dipakai daftar & modal detail; saat artikel masih
     draf di website mitra, badge dibungkus tooltip penjelas. -->
{#snippet statusBadge(row: Pub, cfg: { label: string; dot: string; text: string; bg: string })}
	<span
		class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-semibold capitalize"
		style="color: {cfg.text}; background-color: {cfg.bg};"
	>
		<span class="h-1.5 w-1.5 rounded-full {IN_FLIGHT.includes(row.status) ? 'animate-pulse' : ''}" style="background-color: {cfg.dot};"></span>
		{cfg.label}
	</span>
{/snippet}

<section>
	<header class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<p class="text-xs font-semibold uppercase tracking-wide" style="color: var(--primary);">PUBLICATIONS</p>
			<h1 class="mt-1 text-2xl font-semibold tracking-tight">{tr("Publikasi")}</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				{view === 'publications'
					? tr('Artikel approved yang dikirim ke media — queue worker (Phase 2) atau persetujuan partner (Phase 3).')
					: tr('Jadwal mengirim media ke Mitra sesuai pengaturan — hanya distribusi ke Mitra, waktu terbit aktual dikelola Mitra.')}
			</p>
		</div>
		<div class="inline-flex rounded-lg border p-0.5" style="background-color: var(--muted);">
			<button
				class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
				style={view === 'publications' ? 'background-color: var(--card); color: var(--foreground);' : 'color: var(--muted-foreground);'}
				type="button"
				onclick={() => (view = 'publications')}
			>{tr("Publikasi")}</button>
			<button
				class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
				style={view === 'plans' ? 'background-color: var(--card); color: var(--foreground);' : 'color: var(--muted-foreground);'}
				type="button"
				onclick={() => (view = 'plans')}
			>{tr("Jadwal Terjadwal")}</button>
		</div>
	</header>

	{#if view === 'publications'}
		<!-- Toolbar filter -->
		<div class="app-card mt-4 flex flex-wrap items-center gap-3 p-3">
			<label class="app-input flex min-w-[220px] flex-1 items-center gap-2 !py-1.5">
				<Search size={15} class="text-muted-foreground" />
				<input class="w-full bg-transparent text-sm outline-none" placeholder={tr("Cari artikel, media, atau kategori…")} bind:value={query} />
			</label>
			<Select
				bind:value={statusFilter}
				ariaLabel="Filter status"
				placeholder={tr("Semua Status")}
				minWidth={150}
				options={[
					{ value: '', label: tr('Semua Status') },
					{ value: 'waiting_partner', label: 'waiting_partner' },
					{ value: 'revision', label: tr('Perlu Revisi') },
					{ value: 'published', label: 'Published' },
					{ value: 'queued', label: 'Queued' },
					{ value: 'processing', label: 'Processing' },
					{ value: 'pending_review', label: 'pending_review' },
					{ value: 'failed', label: 'Failed' },
					{ value: 'cancelled', label: 'Cancelled' }
				]}
			/>
			<Select
				bind:value={mediaFilter}
				ariaLabel="Filter media"
				placeholder={tr("Semua Media")}
				minWidth={150}
				options={[
					{ value: '', label: tr('Semua Media') },
					...mediaOptions.map((m) => ({ value: m, label: m }))
				]}
			/>
			<Select
				bind:value={categoryFilter}
				ariaLabel="Filter kategori"
				placeholder={tr("Semua Kategori")}
				minWidth={150}
				options={[
					{ value: '', label: tr('Semua Kategori') },
					...categoryOptions.map((c) => ({ value: c, label: c }))
				]}
			/>
			<Select
				value={sortOrder}
				ariaLabel="Urutkan"
				minWidth={160}
				align="end"
				onchange={(v) => (sortOrder = v as 'newest' | 'oldest' | 'price_desc' | 'price_asc')}
				options={[
					{ value: 'newest', label: '⇅ Terbaru' },
					{ value: 'oldest', label: '⇅ Terlama' },
					{ value: 'price_desc', label: tr('⇅ Harga Tertinggi') },
					{ value: 'price_asc', label: tr('⇅ Harga Terendah') }
				]}
			/>
		</div>

		{#if err}
			<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);" role="alert">{err}</div>
		{/if}

		{#if loading && rows.length === 0}
			<div class="app-card mt-4 flex flex-col gap-3 p-4">
				{#each Array(5) as _, i (i)}
					<div class="flex flex-col gap-3 md:flex-row md:items-center md:gap-4">
						<div class="aspect-video w-full shrink-0 animate-pulse rounded-lg md:aspect-auto md:h-[88px] md:w-[140px]" style="background-color: var(--muted);"></div>
						<div class="flex-1 space-y-2">
							<div class="h-3 w-1/3 animate-pulse rounded" style="background-color: var(--muted);"></div>
							<div class="h-2.5 w-1/2 animate-pulse rounded" style="background-color: var(--muted);"></div>
						</div>
					</div>
				{/each}
			</div>
		{:else if sorted.length === 0}
			<div class="app-card mt-4 px-6 py-10 text-center text-sm text-muted-foreground">{tr("Belum ada publikasi yang cocok…")}</div>
		{:else}
			<div class="app-card mt-4 flex flex-col">
				{#each sorted as row (row.id)}
					{@const cfg = statusCfg(row.status)}
					{@const draftVis = isDraftVisibility(row)}
					{@const badge = draftVis ? draftVisibilityCfg : cfg}
					{@const cats = categoriesOf(row)}
					{@const thumb = row.featured_image_url?.startsWith('https://app.tauthub.com/uploads/')
						? row.featured_image_url.replace('https://app.tauthub.com', '')
						: row.featured_image_url}
					<div class="flex flex-col gap-4 border-b p-4 last:border-0 md:flex-row">
						<!-- Cover: mobile 16/9 selebar kartu dengan label status menempel
						     di pojok kanan atas gambar; desktop thumbnail 140x88. -->
						<div class="relative w-full shrink-0 md:h-[88px] md:w-[140px]">
							{#if thumb && !imgError[row.id]}
								<img
									src={thumb}
									alt={row.document_title ?? 'Cover artikel'}
									class="aspect-video w-full rounded-lg object-cover md:aspect-auto md:h-full md:w-full"
									loading="lazy"
									onerror={() => (imgError = { ...imgError, [row.id]: true })}
								/>
							{:else}
								<span class="flex aspect-video w-full items-center justify-center rounded-lg md:aspect-auto md:h-full md:w-full" style="background-color: var(--muted); color: var(--muted-foreground);" title={row.document_title ?? undefined}>
									<ImageIcon size={28} />
								</span>
							{/if}
							<!-- Label status: overlay di pojok kanan atas gambar (mobile). -->
							<div class="absolute right-2 top-2 md:hidden">
								{#if draftVis}
									<Tooltip content={draftVisibilityTip} maxWidth={320}>
										{@render statusBadge(row, badge)}
									</Tooltip>
								{:else}
									{@render statusBadge(row, badge)}
								{/if}
							</div>
						</div>

						<!-- Utama -->
						<div class="min-w-0 flex-1">
							<a class="pub-title font-semibold hover:underline" href={`/klien/${brandUrl}/editor/${row.document_id}`}>
								{row.document_title ?? tr('Artikel Tanpa Judul')}
							</a>

							<div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
								<span>{fmtDate(row.created_at)}</span>
								{#if row.order_code}
									<span class="rounded px-1.5 py-0.5 font-medium" style="background-color: var(--muted); color: var(--foreground);" title="Nomor order publikasi ini">{row.order_code}</span>
								{/if}
								<span>{tr("Media:")}<span style="color: var(--foreground);">{row.media_name ?? tr('Media Mitra')}</span></span>
								<span>{tcLabel(row.client_price, rateIdr)}</span>
								{#if row.published_url}
									<a class="inline-flex items-center gap-1" style="color: var(--primary);" href={row.published_url} target="_blank" rel="noreferrer">{tr("Lihat Artikel")}<ExternalLink size={13} />
									</a>
								{/if}
							</div>

							{#if cats.length > 0}
								<div class="mt-2 flex flex-wrap items-center gap-1.5">
									{#each cats.slice(0, 2) as c (c)}
										<span class="rounded px-2 py-0.5 text-xs font-medium" style="background-color: var(--muted);">{c}</span>
									{/each}
									{#if cats.length > 2}
										<span class="rounded px-2 py-0.5 text-xs font-medium" style="background-color: var(--muted);" title={`Kategori: ${cats.join(', ')}`}>+{cats.length - 2}</span>
									{/if}
								</div>
							{/if}

							{#if row.media_status === 'archived'}
								<p class="mt-2 text-xs" style="color: var(--warning);">{tr("⚠ Media mitra diarsipkan — permintaan tidak akan lanjut sampai media dipulihkan mitra.")}</p>
							{/if}

							{#if row.status === 'published' && row.pr_analysis}
								<div class="mt-2 flex flex-wrap items-center gap-1.5 text-xs">
									{#if row.pr_analysis.sentiment}
										<Tooltip content={tr('Nada pemberitaan artikel hasil analisis otomatis setelah terbit: positif, netral, atau negatif. Dipakai untuk mengukur kesan artikel yang sudah tayang di media.')} maxWidth={300}>
											<span class="rounded px-2 py-0.5" style="background-color: var(--muted);">Sentiment {SENTIMENT_LABEL[row.pr_analysis.sentiment.sentiment] ?? row.pr_analysis.sentiment.sentiment}</span>
										</Tooltip>
									{/if}
									{#if row.pr_analysis.verification}
										<Tooltip content={tr('Verifikasi otomatis tautan artikel: URL bisa diakses, judul dan brand muncul di halaman, konten ter-publish, dan domain sesuai media. Tiap check bernilai 20 poin, lulus bila skor mencapai 60. ✓ lulus, ✗ gagal, • belum bisa diperiksa.')} maxWidth={320}>
											<span
												class="rounded px-2 py-0.5"
												style={row.pr_analysis.verification.status === 'passed'
													? 'background-color: var(--success-soft); color: var(--success);'
													: row.pr_analysis.verification.status === 'failed'
														? 'background-color: color-mix(in srgb, var(--destructive) 12%, transparent); color: var(--destructive);'
														: 'background-color: var(--warning-soft); color: var(--warning);'}
											>
												URL {row.pr_analysis.verification.status === 'passed' ? '✓' : row.pr_analysis.verification.status === 'failed' ? '✗' : '•'}{row.pr_analysis.verification.compliance_score != null ? ` ${row.pr_analysis.verification.compliance_score}/100` : ''}
											</span>
										</Tooltip>
									{/if}
									{#if row.pr_analysis.message?.score != null}
										<Tooltip content={tr('Persentase pesan utama kampanye yang tersampaikan di artikel, diperiksa otomatis setelah terbit. Skor rendah berarti sebagian pesan penting belum masuk ke tulisan.')} maxWidth={300}>
											<span class="rounded px-2 py-0.5" style="background-color: var(--muted);">Message {row.pr_analysis.message.score}/100</span>
										</Tooltip>
									{/if}
								</div>
							{/if}
						</div>

						<!-- Sisi kanan -->
						<div class="flex shrink-0 flex-col items-end gap-2 md:w-[230px]">
							<!-- Label status desktop (mobile: pojok kanan atas kartu). -->
							<div class="hidden md:block">
								{#if draftVis}
									<Tooltip content={draftVisibilityTip} maxWidth={320}>
										{@render statusBadge(row, badge)}
									</Tooltip>
								{:else}
									{@render statusBadge(row, badge)}
								{/if}
							</div>

							{#if IN_FLIGHT.includes(row.status)}
								<div class="w-full">
									<div class="h-1.5 w-full overflow-hidden rounded-full" style="background-color: var(--muted);">
										<div class="h-full rounded-full" style="width: {progressOf(row)}%; background-color: var(--primary);"></div>
									</div>
									<p class="mt-1 text-right text-xs text-muted-foreground">{progressLabel(row)}</p>
								</div>
							{/if}

							<div class="flex w-full flex-wrap items-center justify-end gap-1.5 md:w-auto">
								{#if CANCELLABLE.has(row.status)}
									<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs flex-1 md:flex-none" style="color: var(--destructive);" type="button" onclick={() => { cancelTarget = row; cancelReason = ''; }}>
										Batalkan
									</button>
								{/if}
								{#if row.status === 'failed'}
									<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs flex-1 md:flex-none" type="button" onclick={() => void retryPublication(row.id)}>
										<RotateCcw size={13} /> Retry
									</button>
								{/if}
								{#if RESETTABLE.has(row.status)}
									<!-- Mobile: baris sendiri, lebar penuh, di atas Detail/Tracking. -->
									<button
										class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs w-full md:w-auto md:flex-none"
										type="button"
										title={tr("Kembalikan ke tahap persetujuan mitra (tombol setuju muncul lagi)")}
										onclick={() => (confirmReset = row)}
									>{tr("Kembalikan ke Persetujuan")}</button>
								{/if}
								<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs flex-1 md:flex-none" type="button" onclick={() => void openDetail(row)}>
									{detailBusy ? tr('Memuat…') : tr('Detail')}
								</button>
								<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs flex-1 md:flex-none" type="button" onclick={() => void openTracking(row)}>
									<ChartLine size={14} /> Tracking
								</button>
							</div>
						</div>
					</div>
				{/each}
			</div>

			<!-- Footer paginasi (custom, parity) -->
			{#if total > 0}
				<div class="mt-3 flex flex-wrap items-center justify-between gap-3">
					<span class="text-sm text-muted-foreground"> {tr("Menampilkan")} {(pageNum - 1) * pageSize + 1}–{Math.min(pageNum * pageSize, total)} {tr("dari")} {total}
					</span>
					<div class="flex items-center gap-2">
						<button class="app-btn app-btn-soft !min-h-8 !px-2.5 !text-xs" type="button" disabled={pageNum <= 1 || loading} onclick={() => { pageNum -= 1; void load(); }}>‹ Sebelumnya</button>
						<span class="text-sm text-muted-foreground">{pageNum} / {totalPages}</span>
						<button class="app-btn app-btn-soft !min-h-8 !px-2.5 !text-xs" type="button" disabled={pageNum >= totalPages || loading} onclick={() => { pageNum += 1; void load(); }}>Berikutnya ›</button>
					</div>
				</div>
			{/if}
		{/if}
	{:else}
		<!-- Jadwal Terjadwal: dikelola di halaman kampanye Taut Pintar (jadwal
		     dibuat dari alur distribusi), jadi arahkan ke sana. -->
		<div class="app-card mt-4 flex flex-col items-center gap-3 px-6 py-12 text-center">
			<p class="font-medium">{tr("Jadwal berulang dikelola dari alur kampanye")}</p>
			<p class="max-w-lg text-sm text-muted-foreground">{tr("Buat dan pantau jadwal distribusi berulang lewat The Gatekeeper pada kampanye Taut Pintar, supaya media, jadwal, dan biayanya tercatat dalam satu alur.")}</p>
			<a class="app-btn app-btn-primary" href={`/klien/${brandUrl}/campaigns`}>{tr("Buka Kampanye")}</a>
		</div>
	{/if}
</section>

<!-- Modal Detail -->
{#if detail}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);">
		<button type="button" aria-label={tr("Tutup")} tabindex="-1" style="position:absolute; inset:0; border:none; background:transparent; cursor:default;" onclick={() => (detail = null)}></button>
		<dialog open class="app-card relative z-10 max-h-[88vh] w-full max-w-xl overflow-y-auto" aria-label={tr("Detail publikasi")} onkeydown={(e) => { if (e.key === 'Escape') detail = null; }}>
			<header class="flex items-start justify-between gap-4 px-6 pt-5">
				<div class="min-w-0">
					<h2 class="line-clamp-2 text-[17px] font-semibold leading-snug">{detail.document_title ?? tr('Artikel Tanpa Judul')}</h2>
					<p class="mt-1 text-xs text-muted-foreground">{detail.media_name ?? tr('Media Mitra')} · {fmtDate(detail.created_at)}</p>
				</div>
				<button class="rounded-lg p-1.5 text-muted-foreground hover:bg-secondary" type="button" aria-label={tr("Tutup")} onclick={() => (detail = null)}><X size={18} /></button>
			</header>

			<dl class="mt-4 grid grid-cols-1 gap-x-10 gap-y-4 border-t px-6 py-4 text-sm sm:grid-cols-2">
				<div>
					<dt class="flex items-center gap-1.5 text-xs text-muted-foreground"><CircleCheck size={13} /> Status</dt>
					<dd class="mt-1.5">
						{#if isDraftVisibility(detail)}
							<Tooltip content={draftVisibilityTip} maxWidth={320}>
								{@render statusBadge(detail, draftVisibilityCfg)}
							</Tooltip>
						{:else}
							{@render statusBadge(detail, statusCfg(detail.status))}
						{/if}
					</dd>
				</div>
				<div>
					<dt class="flex items-center gap-1.5 text-xs text-muted-foreground"><Coins size={13} /> Biaya</dt>
					<dd class="mt-1.5 font-medium tabular-nums">{tcLabel(detail.client_price, rateIdr)}</dd>
				</div>
				<div>
					<dt class="flex items-center gap-1.5 text-xs text-muted-foreground"><Calendar size={13} /> {tr("Terbit")}</dt>
					<dd class="mt-1.5 font-medium">{fmtDate(detail.published_at)}</dd>
				</div>
				{#if detail.order_code}
					<div>
						<dt class="flex items-center gap-1.5 text-xs text-muted-foreground"><FileText size={13} /> Order</dt>
						<dd class="mt-1.5 font-medium">{detail.order_code}</dd>
					</div>
				{/if}
			</dl>

			{#if detail.verification}
				<p class="border-t px-6 py-3 text-xs text-muted-foreground">
					Verifikasi URL: <span class="font-medium" style="color: var(--foreground);">{detail.verification.status}</span>{detail.verification.compliance_score != null ? ` · skor ${detail.verification.compliance_score}/100` : ''}
					{#if detail.verification.error_message}<span class="mt-0.5 block">{detail.verification.error_message}</span>{/if}
				</p>
			{/if}

			{#if detail.published_url}
				<div class="border-t px-6 py-3">
					<a class="inline-flex items-center gap-1.5 text-sm font-medium hover:underline" style="color: var(--primary);" href={detail.published_url} target="_blank" rel="noreferrer">
						<ExternalLink size={14} />
						{tr("Buka artikel terbit")}
					</a>
				</div>
			{/if}

			<footer class="flex flex-wrap items-center gap-2 border-t px-6 py-4">
				{#if CANCELLABLE.has(detail.status)}
					<button class="app-btn app-btn-soft" style="color: var(--destructive);" type="button" onclick={() => { cancelTarget = detail; cancelReason = ''; }}>{tr("Batalkan Publikasi")}</button>
				{/if}
				{#if RESETTABLE.has(detail.status)}
					<button class="app-btn app-btn-soft" type="button" onclick={() => (confirmReset = detail)}>{tr("Kembalikan ke Persetujuan")}</button>
				{/if}
				{#if detail.status === 'failed'}
					<button class="app-btn app-btn-soft" type="button" onclick={() => void retryPublication(detail!.id)}>{tr("Retry Publikasi")}</button>
				{/if}
				<a class="app-btn app-btn-primary ml-auto" href={`/klien/${brandUrl}/editor/${detail.document_id}`}>
					<PencilLine size={14} />
					{tr("Buka Editor")}
				</a>
			</footer>
		</dialog>
	</div>
{/if}

<!-- Modal Batalkan -->
{#if cancelTarget}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);">
		<button type="button" aria-label={tr("Tutup")} tabindex="-1" style="position:absolute; inset:0; border:none; background:transparent; cursor:default;" onclick={() => { if (!cancelBusy) cancelTarget = null; }}></button>
		<dialog open class="app-card relative z-10 w-full max-w-md p-5" aria-label="Batalkan publikasi" onkeydown={(e) => { if (e.key === 'Escape' && !cancelBusy) cancelTarget = null; }}>
			<h2 class="text-base font-bold">Batalkan publikasi</h2>
			<p class="mt-2 text-sm text-muted-foreground">{tr("Publikasi ke")}<strong style="color: var(--foreground);">{cancelTarget.media_name ?? tr('Media Mitra')}</strong>{tr("akan dibatalkan. Saldo (bila ada) otomatis dikembalikan.")}</p>
			<label class="mt-3 flex flex-col gap-1 text-sm">
				Alasan (opsional)
				<textarea class="app-input" rows="3" bind:value={cancelReason} maxlength={500}></textarea>
			</label>
			<div class="mt-4 flex items-center justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" disabled={cancelBusy} onclick={() => (cancelTarget = null)}>{tr("Batal")}</button>
				<button class="app-btn app-btn-primary" type="button" disabled={cancelBusy} onclick={() => void submitCancel()}>
					{cancelBusy ? 'Membatalkan…' : 'Batalkan publikasi'}
				</button>
			</div>
		</dialog>
	</div>
{/if}

<!-- Konfirmasi Kembalikan ke Persetujuan -->
{#if confirmReset}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);">
		<button type="button" aria-label={tr("Tutup")} tabindex="-1" style="position:absolute; inset:0; border:none; background:transparent; cursor:default;" onclick={() => (confirmReset = null)}></button>
		<dialog open class="app-card relative z-10 w-full max-w-md p-5" aria-label={tr("Kembalikan ke persetujuan")} onkeydown={(e) => { if (e.key === 'Escape') confirmReset = null; }}>
			<h2 class="text-base font-bold">{tr("Kembalikan ke persetujuan?")}</h2>
			<p class="mt-2 text-sm text-muted-foreground">{tr("Kembalikan publikasi ini ke tahap persetujuan mitra? Tombol setuju akan muncul lagi di portal mitra.")}</p>
			<div class="mt-4 flex items-center justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (confirmReset = null)}>{tr("Batal")}</button>
				<button class="app-btn app-btn-primary" type="button" onclick={() => void resetToApproval(confirmReset!)}>{tr("Kembalikan")}</button>
			</div>
		</dialog>
	</div>
{/if}

<!-- Modal Tracking -->
{#if trackingTarget}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);">
		<button type="button" aria-label={tr("Tutup")} tabindex="-1" style="position:absolute; inset:0; border:none; background:transparent; cursor:default;" onclick={() => (trackingTarget = null)}></button>
		<dialog open class="app-card relative z-10 w-full max-w-lg p-5" aria-label="Pelacakan publikasi" onkeydown={(e) => { if (e.key === 'Escape') trackingTarget = null; }}>
			<div class="flex items-start justify-between gap-3">
				<h2 class="text-base font-bold">Tracking</h2>
				<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label={tr("Tutup")} onclick={() => (trackingTarget = null)}><X size={18} /></button>
			</div>
			<p class="mt-1 text-xs text-muted-foreground">{trackingTarget.document_title ?? tr('Artikel Tanpa Judul')}</p>

			{#if tracking}
				<div class="mt-3 rounded-lg border p-3">
					<p class="text-xs text-muted-foreground">Short link</p>
					<a class="text-sm font-medium" style="color: var(--primary);" href={trackUrl} target="_blank" rel="noreferrer">{trackUrl}</a>
					<p class="mt-2 text-xs text-muted-foreground">{tr("Total klik tercatat:")}<strong style="color: var(--foreground);" class="tabular-nums">{trackTotal.toLocaleString('id-ID')}</strong>
					</p>
					{#if Object.keys(trackingCounts).length > 0}
						<div class="mt-1 flex flex-wrap gap-2 text-xs text-muted-foreground">
							{#each Object.entries(trackingCounts) as [k, v] (k)}<span>{k}: <strong style="color: var(--foreground);">{v}</strong></span>{/each}
						</div>
					{/if}
				</div>
			{:else}
				<p class="mt-3 text-sm text-muted-foreground">{tr("Belum ada tautan pelacakan untuk publikasi ini.")}</p>
			{/if}

			<label class="mt-3 flex flex-col gap-1 text-sm">
				URL tujuan (opsional)
				<input class="app-input" bind:value={destinationUrl} placeholder="https://…" maxlength={2000} />
			</label>

			<div class="mt-4 flex items-center justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (trackingTarget = null)}>{tr("Tutup")}</button>
				<button class="app-btn app-btn-primary" type="button" disabled={trackingBusy} onclick={() => void saveTracking()}>
					{trackingBusy ? 'Menyimpan…' : tr('Simpan')}
				</button>
			</div>
		</dialog>
	</div>
{/if}

{#if detailBusy}
	<p class="fixed bottom-4 right-4 flex items-center gap-2 rounded-lg border px-3 py-2 text-sm shadow" style="background-color: var(--card);" role="status">
		<LoaderCircle size={15} class="animate-spin" />{tr("Memuat detail…")}</p>
{/if}
{#if err && rows.length > 0}
	<p class="fixed bottom-4 right-4 flex max-w-sm items-center gap-2 rounded-lg border px-3 py-2 text-sm" style="background-color: var(--card); color: var(--destructive);" role="status">
		<CircleAlert size={15} /> {err}
	</p>
{/if}

<style>
	/* Judul publikasi dibuat inline (parity <span className={pubStyles.pubTitle}>
	   app lama) supaya judul panjang membungkus ke baris berikutnya. Dengan
	   `truncate` sebagai elemen inline, ellipsis tidak pernah aktif dan teks
	   justru meluber keluar kartu. */
	.pub-title { display: inline-block; max-width: 100%; word-break: break-word; }
</style>
