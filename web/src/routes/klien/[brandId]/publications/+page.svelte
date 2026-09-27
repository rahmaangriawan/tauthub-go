<script lang="ts">
	// Status Publikasi klien — port PublicationsSection (distribution.tsx).
	// Legacy memakai CARD LIST (bukan tabel): tiap baris = [cover][utama][sisi].
	// Layout: header (+eyebrow PUBLICATIONS +tab) → toolbar (cari + STATUS/MEDIA/
	// KATEGORI + urut) → banner error → daftar baris → footer paginasi → modal
	// (detail, batalkan, tracking) → toast "macet".
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import {
		ChartLine, CircleAlert, ExternalLink, Image as ImageIcon, LoaderCircle, RotateCcw, Search, X
	} from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

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
			message?: { score: number | null } | null;
			sentiment?: string | null;
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
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal memuat publikasi.'));
			const data = (await res.json()) as { publications?: Pub[]; total?: number };
			rows = data.publications ?? [];
			total = data.total ?? 0;
		} catch (reason) {
			err = reason instanceof Error ? reason.message : 'Gagal memuat publikasi.';
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
		cancelled: { label: 'Cancelled', dot: 'var(--muted-foreground)', text: 'var(--muted-foreground)', bg: 'var(--muted)' }
	};
	const STATUS_FALLBACK = { dot: 'var(--muted-foreground)', text: 'var(--muted-foreground)', bg: 'var(--muted)' };
	function statusCfg(status: string) {
		const c = PUB_STATUS_CONFIG[status.toLowerCase()];
		return c ?? { ...STATUS_FALLBACK, label: status };
	}

	const CANCELLABLE = new Set(['pending', 'queued', 'waiting_partner', 'partner_rejected', 'processing', 'retrying']);
	const RESETTABLE = new Set(['processing', 'failed', 'retrying', 'queued', 'cancelled']);
	const IN_FLIGHT = ['queued', 'processing', 'retrying'];

	/** tcLabel — konversi rupiah → TC dengan rate aktif (parity). */
	function tcLabel(rupiah: number, rate: number): string {
		const tc = Math.max(0, Math.round(Number(rupiah) / Math.max(1, rate)));
		return `${tc.toLocaleString('id-ID')} TC`;
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
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal memuat detail publikasi.'));
			const data = (await res.json()) as { publication?: Detail } & Detail;
			detail = (data.publication ?? data) as Detail;
		} catch (reason) {
			err = reason instanceof Error ? reason.message : 'Gagal memuat detail publikasi.';
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
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal membatalkan publikasi.'));
			cancelTarget = null;
			cancelReason = '';
			detail = null;
			await load();
		} catch (reason) {
			err = reason instanceof Error ? reason.message : 'Gagal membatalkan publikasi.';
		} finally {
			cancelBusy = false;
		}
	}

	async function retryPublication(id: string) {
		try {
			const res = await apiRequest(`${apiBase}/publications/${id}/retry`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal mencoba ulang publikasi.'));
			await load();
		} catch (reason) {
			err = reason instanceof Error ? reason.message : 'Gagal mencoba ulang publikasi.';
		}
	}

	async function resetToApproval(p: Pub) {
		confirmReset = null;
		try {
			const res = await apiRequest(`${apiBase}/publications/${p.id}/reset-approval`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal mengembalikan publikasi ke persetujuan.'));
			await load();
		} catch (reason) {
			err = reason instanceof Error ? reason.message : 'Gagal mengembalikan publikasi ke persetujuan.';
		}
	}

	async function openTracking(row: Pub) {
		trackingTarget = row;
		tracking = null;
		trackingCounts = {};
		destinationUrl = '';
		try {
			const res = await apiRequest(`${apiBase}/publications/${row.id}/tracking`);
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal memuat data pelacakan.'));
			const data = (await res.json()) as {
				tracking?: { tracking_path: string; destination_url: string | null; status: string } | null;
				counts?: Record<string, number>;
			};
			tracking = data.tracking ?? null;
			trackingCounts = data.counts ?? {};
			destinationUrl = data.tracking?.destination_url ?? row.published_url ?? '';
		} catch (reason) {
			err = reason instanceof Error ? reason.message : 'Gagal memuat data pelacakan.';
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
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal menyimpan tujuan pelacakan.'));
			const data = (await res.json()) as { tracking?: { tracking_path: string; destination_url: string | null; status: string } };
			tracking = data.tracking ?? tracking;
		} catch (reason) {
			err = reason instanceof Error ? reason.message : 'Gagal menyimpan tujuan pelacakan.';
		} finally {
			trackingBusy = false;
		}
	}

	const trackUrl = $derived(
		tracking ? `${typeof window !== 'undefined' ? window.location.origin : ''}${tracking.tracking_path}` : ''
	);
	const trackTotal = $derived(Object.values(trackingCounts).reduce((s, n) => s + n, 0));
</script>

<section class="mx-auto max-w-6xl">
	<header class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<p class="text-xs font-semibold uppercase tracking-wide" style="color: var(--primary);">PUBLICATIONS</p>
			<h1 class="mt-1 text-2xl font-semibold tracking-tight">Publikasi</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				{view === 'publications'
					? 'Artikel approved yang dikirim ke media — queue worker (Phase 2) atau persetujuan partner (Phase 3).'
					: 'Jadwal mengirim media ke Mitra sesuai pengaturan — hanya distribusi ke Mitra, waktu terbit aktual dikelola Mitra.'}
			</p>
		</div>
		<div class="inline-flex rounded-lg border p-0.5" style="background-color: var(--muted);">
			<button
				class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
				style={view === 'publications' ? 'background-color: var(--card); color: var(--foreground);' : 'color: var(--muted-foreground);'}
				type="button"
				onclick={() => (view = 'publications')}
			>Publikasi</button>
			<button
				class="rounded-md px-3 py-1.5 text-sm font-medium transition-colors"
				style={view === 'plans' ? 'background-color: var(--card); color: var(--foreground);' : 'color: var(--muted-foreground);'}
				type="button"
				onclick={() => (view = 'plans')}
			>Jadwal Terjadwal</button>
		</div>
	</header>

	{#if view === 'publications'}
		<!-- Toolbar filter -->
		<div class="app-card mt-4 flex flex-wrap items-center gap-3 p-3">
			<label class="app-input flex min-w-[220px] flex-1 items-center gap-2 !py-1.5">
				<Search size={15} class="text-muted-foreground" />
				<input class="w-full bg-transparent text-sm outline-none" placeholder="Cari artikel, media, atau kategori…" bind:value={query} />
			</label>
			<select class="app-input !w-auto min-w-[150px]" bind:value={statusFilter}>
				<option value="">Semua Status</option>
				<option value="waiting_partner">waiting_partner</option>
				<option value="published">Published</option>
				<option value="queued">Queued</option>
				<option value="processing">Processing</option>
				<option value="pending_review">pending_review</option>
				<option value="failed">Failed</option>
				<option value="cancelled">Cancelled</option>
			</select>
			<select class="app-input !w-auto min-w-[150px]" bind:value={mediaFilter}>
				<option value="">Semua Media</option>
				{#each mediaOptions as m (m)}<option value={m}>{m}</option>{/each}
			</select>
			<select class="app-input !w-auto min-w-[150px]" bind:value={categoryFilter}>
				<option value="">Semua Kategori</option>
				{#each categoryOptions as c (c)}<option value={c}>{c}</option>{/each}
			</select>
			<select class="app-input !w-auto min-w-[160px]" bind:value={sortOrder}>
				<option value="newest">⇅ Terbaru</option>
				<option value="oldest">⇅ Terlama</option>
				<option value="price_desc">⇅ Harga Tertinggi</option>
				<option value="price_asc">⇅ Harga Terendah</option>
			</select>
		</div>

		{#if err}
			<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);" role="alert">{err}</div>
		{/if}

		{#if loading && rows.length === 0}
			<div class="app-card mt-4 flex flex-col gap-3 p-4">
				{#each Array(5) as _, i (i)}
					<div class="flex items-center gap-4">
						<div class="h-[88px] w-[140px] shrink-0 animate-pulse rounded-lg" style="background-color: var(--muted);"></div>
						<div class="flex-1 space-y-2">
							<div class="h-3 w-1/3 animate-pulse rounded" style="background-color: var(--muted);"></div>
							<div class="h-2.5 w-1/2 animate-pulse rounded" style="background-color: var(--muted);"></div>
						</div>
					</div>
				{/each}
			</div>
		{:else if sorted.length === 0}
			<div class="app-card mt-4 px-6 py-10 text-center text-sm text-muted-foreground">
				Belum ada publikasi yang cocok…
			</div>
		{:else}
			<div class="app-card mt-4 flex flex-col">
				{#each sorted as row (row.id)}
					{@const cfg = statusCfg(row.status)}
					{@const cats = categoriesOf(row)}
					{@const thumb = row.featured_image_url?.startsWith('https://app.tauthub.com/uploads/')
						? row.featured_image_url.replace('https://app.tauthub.com', '')
						: row.featured_image_url}
					<div class="flex flex-col gap-4 border-b p-4 last:border-0 md:flex-row">
						<!-- Cover -->
						{#if thumb && !imgError[row.id]}
							<img
								src={thumb}
								alt={row.document_title ?? 'Cover artikel'}
								class="h-[88px] w-[140px] shrink-0 rounded-lg object-cover"
								loading="lazy"
								onerror={() => (imgError = { ...imgError, [row.id]: true })}
							/>
						{:else}
							<span class="flex h-[88px] w-[140px] shrink-0 items-center justify-center rounded-lg" style="background-color: var(--muted); color: var(--muted-foreground);" title={row.document_title ?? undefined}>
								<ImageIcon size={28} />
							</span>
						{/if}

						<!-- Utama -->
						<div class="min-w-0 flex-1">
							<a class="pub-title font-semibold hover:underline" href={`/klien/${brandUrl}/editor/${row.document_id}`}>
								{row.document_title ?? 'Artikel Tanpa Judul'}
							</a>

							<div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
								<span>{fmtDate(row.created_at)}</span>
								{#if row.order_code}
									<span class="rounded px-1.5 py-0.5 font-medium" style="background-color: var(--muted); color: var(--foreground);" title="Nomor order publikasi ini">{row.order_code}</span>
								{/if}
								<span>Media: <span style="color: var(--foreground);">{row.media_name ?? 'Media Mitra'}</span></span>
								<span>{tcLabel(row.client_price, rateIdr)}</span>
								{#if row.published_url}
									<a class="inline-flex items-center gap-1" style="color: var(--primary);" href={row.published_url} target="_blank" rel="noreferrer">
										Lihat terbit <ExternalLink size={13} />
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
								<p class="mt-2 text-xs" style="color: var(--warning);">
									⚠ Media mitra diarsipkan — permintaan tidak akan lanjut sampai media dipulihkan mitra.
								</p>
							{/if}

							{#if row.status === 'published' && row.pr_analysis}
								<div class="mt-2 flex flex-wrap items-center gap-1.5 text-xs">
									{#if row.pr_analysis.sentiment}
										<span class="rounded px-2 py-0.5" style="background-color: var(--muted);">Sentiment {SENTIMENT_LABEL[row.pr_analysis.sentiment] ?? row.pr_analysis.sentiment}</span>
									{/if}
									{#if row.pr_analysis.verification}
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
									{/if}
									{#if row.pr_analysis.message?.score != null}
										<span class="rounded px-2 py-0.5" style="background-color: var(--muted);">Message {row.pr_analysis.message.score}/100</span>
									{/if}
								</div>
							{/if}
						</div>

						<!-- Sisi kanan -->
						<div class="flex shrink-0 flex-col items-end gap-2 md:w-[230px]">
							<span
								class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-semibold capitalize"
								style="color: {cfg.text}; background-color: {cfg.bg};"
							>
								<span class="h-1.5 w-1.5 rounded-full {IN_FLIGHT.includes(row.status) ? 'animate-pulse' : ''}" style="background-color: {cfg.dot};"></span>
								{cfg.label}
							</span>

							{#if IN_FLIGHT.includes(row.status)}
								<div class="w-full">
									<div class="h-1.5 w-full overflow-hidden rounded-full" style="background-color: var(--muted);">
										<div class="h-full rounded-full" style="width: {progressOf(row)}%; background-color: var(--primary);"></div>
									</div>
									<p class="mt-1 text-right text-xs text-muted-foreground">{progressLabel(row)}</p>
								</div>
							{/if}

							<div class="flex flex-wrap items-center justify-end gap-1.5">
								{#if CANCELLABLE.has(row.status)}
									<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" style="color: var(--destructive);" type="button" onclick={() => { cancelTarget = row; cancelReason = ''; }}>
										Batalkan
									</button>
								{/if}
								{#if row.status === 'failed'}
									<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={() => void retryPublication(row.id)}>
										<RotateCcw size={13} /> Retry
									</button>
								{/if}
								{#if RESETTABLE.has(row.status)}
									<button
										class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs"
										type="button"
										title="Kembalikan ke tahap persetujuan mitra (tombol setuju muncul lagi)"
										onclick={() => (confirmReset = row)}
									>Kembalikan ke Persetujuan</button>
								{/if}
								<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={() => void openDetail(row)}>
									{detailBusy ? 'Memuat…' : 'Detail'}
								</button>
								<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={() => void openTracking(row)}>
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
					<span class="text-sm text-muted-foreground">
						Menampilkan {(pageNum - 1) * pageSize + 1}–{Math.min(pageNum * pageSize, total)} dari {total}
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
			<p class="font-medium">Jadwal berulang dikelola dari alur kampanye</p>
			<p class="max-w-lg text-sm text-muted-foreground">
				Buat dan pantau jadwal distribusi berulang lewat The Gatekeeper pada kampanye Taut Pintar, supaya
				media, jadwal, dan kreditnya tercatat dalam satu alur.
			</p>
			<a class="app-btn app-btn-primary" href={`/klien/${brandUrl}/campaigns`}>Buka Kampanye</a>
		</div>
	{/if}
</section>

<!-- Modal Detail -->
{#if detail}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);">
		<button type="button" aria-label="Tutup" tabindex="-1" style="position:absolute; inset:0; border:none; background:transparent; cursor:default;" onclick={() => (detail = null)}></button>
		<dialog open class="app-card relative z-10 max-h-[88vh] w-full max-w-2xl overflow-y-auto p-5" aria-label="Detail publikasi" onkeydown={(e) => { if (e.key === 'Escape') detail = null; }}>
			<div class="flex items-start justify-between gap-3">
				<div>
					<h2 class="text-base font-bold">{detail.document_title ?? 'Artikel Tanpa Judul'}</h2>
					<p class="mt-0.5 text-xs text-muted-foreground">{detail.media_name ?? 'Media Mitra'} · {fmtDate(detail.created_at)}</p>
				</div>
				<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label="Tutup" onclick={() => (detail = null)}><X size={18} /></button>
			</div>

			<dl class="mt-4 grid grid-cols-2 gap-3 text-sm">
				<div><dt class="text-xs text-muted-foreground">Status</dt><dd class="font-semibold capitalize">{statusCfg(detail.status).label}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Biaya</dt><dd class="font-semibold tabular-nums">{tcLabel(detail.client_price, rateIdr)}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Terbit</dt><dd>{fmtDate(detail.published_at)}</dd></div>
				{#if detail.order_code}<div><dt class="text-xs text-muted-foreground">Order</dt><dd>{detail.order_code}</dd></div>{/if}
			</dl>

			{#if detail.verification}
				<p class="mt-3 rounded-lg px-3 py-2 text-xs" style="background-color: var(--muted);">
					Verifikasi URL: <strong>{detail.verification.status}</strong>{detail.verification.compliance_score != null ? ` · skor ${detail.verification.compliance_score}/100` : ''}
					{#if detail.verification.error_message}<span class="block text-muted-foreground">{detail.verification.error_message}</span>{/if}
				</p>
			{/if}

			{#if detail.published_url}
				<a class="mt-3 inline-flex items-center gap-1 text-sm" style="color: var(--primary);" href={detail.published_url} target="_blank" rel="noreferrer">
					Buka artikel terbit <ExternalLink size={13} />
				</a>
			{/if}

			<div class="mt-5 flex flex-wrap items-center justify-end gap-2 border-t pt-4">
				{#if CANCELLABLE.has(detail.status)}
					<button class="app-btn app-btn-soft" style="color: var(--destructive);" type="button" onclick={() => { cancelTarget = detail; cancelReason = ''; }}>Batalkan Publikasi</button>
				{/if}
				{#if RESETTABLE.has(detail.status)}
					<button class="app-btn app-btn-soft" type="button" onclick={() => (confirmReset = detail)}>Kembalikan ke Persetujuan</button>
				{/if}
				{#if detail.status === 'failed'}
					<button class="app-btn app-btn-soft" type="button" onclick={() => void retryPublication(detail!.id)}>Retry Publikasi</button>
				{/if}
				<a class="app-btn app-btn-primary" href={`/klien/${brandUrl}/editor/${detail.document_id}`}>Buka Editor</a>
			</div>
		</dialog>
	</div>
{/if}

<!-- Modal Batalkan -->
{#if cancelTarget}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);">
		<button type="button" aria-label="Tutup" tabindex="-1" style="position:absolute; inset:0; border:none; background:transparent; cursor:default;" onclick={() => { if (!cancelBusy) cancelTarget = null; }}></button>
		<dialog open class="app-card relative z-10 w-full max-w-md p-5" aria-label="Batalkan publikasi" onkeydown={(e) => { if (e.key === 'Escape' && !cancelBusy) cancelTarget = null; }}>
			<h2 class="text-base font-bold">Batalkan publikasi</h2>
			<p class="mt-2 text-sm text-muted-foreground">
				Publikasi ke <strong style="color: var(--foreground);">{cancelTarget.media_name ?? 'Media Mitra'}</strong> akan dibatalkan. Kredit (bila ada) otomatis dikembalikan.
			</p>
			<label class="mt-3 flex flex-col gap-1 text-sm">
				Alasan (opsional)
				<textarea class="app-input" rows="3" bind:value={cancelReason} maxlength={500}></textarea>
			</label>
			<div class="mt-4 flex items-center justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" disabled={cancelBusy} onclick={() => (cancelTarget = null)}>Batal</button>
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
		<button type="button" aria-label="Tutup" tabindex="-1" style="position:absolute; inset:0; border:none; background:transparent; cursor:default;" onclick={() => (confirmReset = null)}></button>
		<dialog open class="app-card relative z-10 w-full max-w-md p-5" aria-label="Kembalikan ke persetujuan" onkeydown={(e) => { if (e.key === 'Escape') confirmReset = null; }}>
			<h2 class="text-base font-bold">Kembalikan ke persetujuan?</h2>
			<p class="mt-2 text-sm text-muted-foreground">
				Kembalikan publikasi ini ke tahap persetujuan mitra? Tombol setuju akan muncul lagi di portal mitra.
			</p>
			<div class="mt-4 flex items-center justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (confirmReset = null)}>Batal</button>
				<button class="app-btn app-btn-primary" type="button" onclick={() => void resetToApproval(confirmReset!)}>Kembalikan</button>
			</div>
		</dialog>
	</div>
{/if}

<!-- Modal Tracking -->
{#if trackingTarget}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);">
		<button type="button" aria-label="Tutup" tabindex="-1" style="position:absolute; inset:0; border:none; background:transparent; cursor:default;" onclick={() => (trackingTarget = null)}></button>
		<dialog open class="app-card relative z-10 w-full max-w-lg p-5" aria-label="Pelacakan publikasi" onkeydown={(e) => { if (e.key === 'Escape') trackingTarget = null; }}>
			<div class="flex items-start justify-between gap-3">
				<h2 class="text-base font-bold">Tracking</h2>
				<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label="Tutup" onclick={() => (trackingTarget = null)}><X size={18} /></button>
			</div>
			<p class="mt-1 text-xs text-muted-foreground">{trackingTarget.document_title ?? 'Artikel Tanpa Judul'}</p>

			{#if tracking}
				<div class="mt-3 rounded-lg border p-3">
					<p class="text-xs text-muted-foreground">Short link</p>
					<a class="text-sm font-medium" style="color: var(--primary);" href={trackUrl} target="_blank" rel="noreferrer">{trackUrl}</a>
					<p class="mt-2 text-xs text-muted-foreground">
						Total klik tercatat: <strong style="color: var(--foreground);" class="tabular-nums">{trackTotal.toLocaleString('id-ID')}</strong>
					</p>
					{#if Object.keys(trackingCounts).length > 0}
						<div class="mt-1 flex flex-wrap gap-2 text-xs text-muted-foreground">
							{#each Object.entries(trackingCounts) as [k, v] (k)}<span>{k}: <strong style="color: var(--foreground);">{v}</strong></span>{/each}
						</div>
					{/if}
				</div>
			{:else}
				<p class="mt-3 text-sm text-muted-foreground">Belum ada tautan pelacakan untuk publikasi ini.</p>
			{/if}

			<label class="mt-3 flex flex-col gap-1 text-sm">
				URL tujuan (opsional)
				<input class="app-input" bind:value={destinationUrl} placeholder="https://…" maxlength={2000} />
			</label>

			<div class="mt-4 flex items-center justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (trackingTarget = null)}>Tutup</button>
				<button class="app-btn app-btn-primary" type="button" disabled={trackingBusy} onclick={() => void saveTracking()}>
					{trackingBusy ? 'Menyimpan…' : 'Simpan'}
				</button>
			</div>
		</dialog>
	</div>
{/if}

{#if detailBusy}
	<p class="fixed bottom-4 right-4 flex items-center gap-2 rounded-lg border px-3 py-2 text-sm shadow" style="background-color: var(--card);" role="status">
		<LoaderCircle size={15} class="animate-spin" /> Memuat detail…
	</p>
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
