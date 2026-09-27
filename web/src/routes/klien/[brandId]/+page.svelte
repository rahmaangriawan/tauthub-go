<script lang="ts">
	// Beranda klien — port section `overview` (client-overview.tsx) app lama:
	// header sambutan, 4 kartu KPI + sparkline 7 hari, Lanjutkan Pekerjaan,
	// Kampanye Aktif, Status Publikasi, Mulai dengan Cepat, Aktivitas Terbaru.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		ArrowRight, CheckCircle, Clock, Coins, History, Megaphone, Newspaper, Plus,
		Rocket, ScrollText, Target
	} from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, ensureValidSession, getStoredSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { getSwrCache, setSwrCache } from '$lib/swr-cache.svelte';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);

	type WalletSummary = { available_tc: number; reserved_tc: number; captured_tc: number; total_tc: number };
	type OverviewTrends = { campaigns: number[]; content: number[]; publications: number[]; credit: number[] };
	type Campaign = {
		id: string; name: string; status: string; article_quota: number; budget: number;
		publication_count: number; content_intake_id?: string | null; taut_status?: string | null;
		created_at: string; updated_at?: string;
	};
	type PublicationItem = {
		id: string; status: string; document_title?: string | null; media_name?: string | null; created_at: string;
	};
	type IntakeItem = {
		id: string; mode: string; content_type_code: string | null; topic: string | null; status: string;
		readiness_score: number | null; created_at: string; updated_at: string;
	};
	type ContentJobItem = { id: string; topic?: string; status: string; current_step: string; progress: number; created_at: string };
	type AuditActivity = { id: string; action: string; target_type: string; target_id: string; created_at: string };
	type DashboardStats = {
		content_in_progress: number; waiting_review: number; failed_jobs: number;
		monthly_articles: { used: number; limit: number; percent: number }; recent_activity: AuditActivity[];
	};
	type OverviewPayload = {
		brand_id: string; public_id?: string; user?: { display_name?: string; name?: string; email?: string };
		wallet?: WalletSummary | null; campaigns?: Campaign[]; intakes?: IntakeItem[]; jobs?: ContentJobItem[];
		publications?: PublicationItem[]; dashboardStats?: DashboardStats | null; trends?: OverviewTrends | null;
	};

	let data = $state<OverviewPayload | null>(null);
	let loading = $state(true);
	let error = $state('');

	const cacheKey = $derived(`overview:${brandId}`);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			const cached = getSwrCache<OverviewPayload>(cacheKey);
			if (cached) {
				data = cached;
				loading = false;
			}
			await load();
			loading = false;
		})();

		// Revalidate saat tab kembali fokus (parity OVERVIEW_CACHE + focus listener).
		const onFocus = () => void load();
		window.addEventListener('focus', onFocus);
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			window.removeEventListener('focus', onFocus);
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
		};
	});

	async function load() {
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/overview`);
			if (res.ok) {
				const payload = (await res.json()) as OverviewPayload;
				data = payload;
				setSwrCache(cacheKey, payload, 60_000);
				return;
			}
			error = 'Gagal memuat ringkasan.';
		} catch {
			error = 'Gagal memuat ringkasan.';
		}
	}

	/* ─── Derivasi (parity client-overview) ─── */
	const campaigns = $derived(data?.campaigns ?? []);
	const intakes = $derived(data?.intakes ?? []);
	const jobs = $derived(data?.jobs ?? []);
	const publications = $derived(data?.publications ?? []);
	const wallet = $derived(data?.wallet ?? null);
	const trends = $derived(data?.trends ?? null);
	const dashboardStats = $derived(data?.dashboardStats ?? null);

	const userName = $derived(
		data?.user?.display_name?.trim() || data?.user?.name?.trim() || getStoredSession()?.user?.display_name?.trim() || ''
	);

	const activeCampaigns = $derived(campaigns.filter((c) => c.status === 'active'));
	const isBrandEmpty = $derived(
		!loading && campaigns.length === 0 && intakes.length === 0 && jobs.length === 0 && publications.length === 0
	);

	const totalContentCount = $derived(dashboardStats?.monthly_articles?.used ?? jobs.length + intakes.length);

	const publicationCounts = $derived.by(() => {
		let published = 0;
		let pending = 0;
		let failed = 0;
		for (const pub of publications) {
			if (pub.status === 'published') published += 1;
			else if (['draft', 'ready', 'scheduled', 'in_review', 'submitting', 'queued'].includes(pub.status)) pending += 1;
			else if (['failed', 'rejected', 'cancelled'].includes(pub.status)) failed += 1;
		}
		return { published, pending, failed, total: publications.length };
	});

	type ContinueItem = {
		id: string; title: string; type: string; statusLabel: string;
		readiness?: number | null; date: string; actionUrl: string; actionText: string; sortTime: number;
	};

	const continueItems = $derived.by(() => {
		const items: ContinueItem[] = [];
		const intakeIdsCoveredByCampaigns = new Set<string>();

		for (const campaign of campaigns) {
			const hasDraftIntake = Boolean(campaign.content_intake_id);
			const isDraftCampaign = campaign.status === 'draft';
			// Alur baru (Taut Pintar): kampanye dianggap berjalan bila taut_status
			// belum selesai. Nilai terminal: submitted/scheduled/distributed.
			const tautStatus = (campaign.taut_status ?? '').toLowerCase();
			const hasActiveFlow = !['submitted', 'scheduled', 'distributed'].includes(tautStatus);
			if (hasDraftIntake || isDraftCampaign || hasActiveFlow) {
				if (campaign.content_intake_id) intakeIdsCoveredByCampaigns.add(campaign.content_intake_id);
				const dateStr = campaign.updated_at || campaign.created_at;
				items.push({
					id: `campaign-draft-${campaign.id}`,
					title: campaign.name,
					type: 'Draf Kampanye',
					statusLabel: 'Edit Draf',
					date: dateStr,
					actionUrl: `/klien/${brandUrl}/taut-pintar/${campaign.id}`,
					actionText: 'Lanjutkan Draf',
					sortTime: dateStr ? new Date(dateStr).getTime() : 0
				});
			}
		}

		for (const job of jobs) {
			if (job.status === 'waiting_outline_approval') {
				items.push({
					id: `job-${job.id}`, title: job.topic || 'Pembuatan Konten AI', type: 'Artikel AI',
					statusLabel: 'Perlu Persetujuan Outline', date: job.created_at,
					actionUrl: `/klien/${brandUrl}/generate`, actionText: 'Tinjau Outline',
					sortTime: new Date(job.created_at).getTime()
				});
			} else if (['queued', 'running'].includes(job.status)) {
				items.push({
					id: `job-${job.id}`, title: job.topic || 'Pembuatan Konten AI', type: 'Artikel AI',
					statusLabel: `Sedang Diproses (${job.progress}%)`, date: job.created_at,
					actionUrl: `/klien/${brandUrl}/generate`, actionText: 'Lihat Progress',
					sortTime: new Date(job.created_at).getTime()
				});
			}
		}

		for (const intake of intakes) {
			if (intakeIdsCoveredByCampaigns.has(intake.id)) continue;
			if (intake.status === 'draft' || !intake.status) {
				const dateStr = intake.updated_at || intake.created_at;
				items.push({
					id: `intake-${intake.id}`, title: intake.topic || 'Draft Konten Baru',
					type: intake.content_type_code?.toUpperCase() || 'Content Intake', statusLabel: 'Draft',
					readiness: intake.readiness_score, date: dateStr,
					actionUrl: `/klien/${brandUrl}/dashboard`, actionText: 'Lanjutkan',
					sortTime: dateStr ? new Date(dateStr).getTime() : 0
				});
			}
		}

		items.sort((a, b) => b.sortTime - a.sortTime);
		return items.slice(0, 4);
	});

	/* ─── Helpers format (parity) ─── */
	function formatTc(value: number | null | undefined): string {
		if (value === null || value === undefined || Number.isNaN(value)) return '—';
		return `${value.toLocaleString('id-ID')} Kredit`;
	}

	function formatRelativeTime(dateString: string | null | undefined): string {
		if (!dateString) return '';
		const date = new Date(dateString);
		const diffSec = Math.floor((Date.now() - date.getTime()) / 1000);
		if (diffSec < 60) return 'Baru saja';
		const diffMin = Math.floor(diffSec / 60);
		if (diffMin < 60) return `${diffMin} menit lalu`;
		const diffHours = Math.floor(diffMin / 60);
		if (diffHours < 24) return `${diffHours} jam lalu`;
		const diffDays = Math.floor(diffHours / 24);
		if (diffDays === 1) return 'Kemarin';
		if (diffDays < 7) return `${diffDays} hari lalu`;
		return date.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' });
	}

	function describeAction(action: string): string {
		switch (action) {
			case 'content.intake_created':
				return 'Draft intake konten baru dibuat';
			case 'content.job_created':
				return 'Pembuatan draf artikel AI dimulai';
			case 'content.job_completed':
				return 'Artikel AI selesai dibuat';
			case 'publication.created':
				return 'Pengajuan publikasi media berhasil dikirim';
			case 'publication.published':
				return 'Artikel berhasil diterbitkan di media mitra';
			case 'campaign.created':
				return 'Kampanye baru berhasil dibuat';
			case 'credits.topup':
				return 'Top-up Kredit Saldo berhasil';
			default:
				return action.replace(/[_.]/g, ' ');
		}
	}

	/** Sparkline 7 titik — polyline SVG (pengganti recharts MiniSpark). */
	function sparkPoints(values: number[] | undefined, width = 80, height = 36): string {
		const list = values && values.length > 0 ? values : [0, 0, 0, 0, 0, 0, 0];
		const max = Math.max(...list, 1);
		const stepX = list.length > 1 ? width / (list.length - 1) : width;
		return list
			.map((v, i) => `${(i * stepX).toFixed(1)},${(height - 3 - (v / max) * (height - 8)).toFixed(1)}`)
			.join(' ');
	}

	const kpiCards = $derived([
		{
			label: 'Kampanye Aktif', value: loading ? '—' : String(activeCampaigns.length),
			spark: trends?.campaigns, color: '#C27A2E', bg: 'color-mix(in srgb, #C27A2E 12%, transparent)', icon: Target
		},
		{
			label: 'Konten', value: loading ? '—' : String(totalContentCount),
			spark: trends?.content, color: '#3D6A3A', bg: 'color-mix(in srgb, #3D6A3A 12%, transparent)', icon: ScrollText
		},
		{
			label: 'Publikasi', value: loading ? '—' : String(publicationCounts.published),
			spark: trends?.publications, color: '#2F6F6A', bg: 'color-mix(in srgb, #2F6F6A 12%, transparent)', icon: Megaphone
		},
		{
			label: 'Kredit Saldo', value: loading ? '—' : formatTc(wallet?.available_tc),
			spark: trends?.credit, color: '#2563EB', bg: 'color-mix(in srgb, #2563EB 12%, transparent)', icon: Coins
		}
	]);

	const quickActions = $derived([
		{ title: 'Buat Konten', desc: 'Buat artikel atau press release baru dengan AI.', href: `/klien/${brandUrl}/campaigns`, icon: Plus, color: '#C27A2E' },
		{ title: 'Lihat Publikasi', desc: 'Kelola dan pantau status tayang media Anda.', href: `/klien/${brandUrl}/publications`, icon: Megaphone, color: '#2F6F6A' },
		{ title: 'Jelajahi Media', desc: 'Lihat ratusan portal berita & media partner.', href: `/klien/${brandUrl}/media`, icon: Newspaper, color: '#6D5BA6' },
		{ title: 'Kelola Kampanye', desc: 'Pantau target kuota dan budget kampanye.', href: `/klien/${brandUrl}/campaigns`, icon: Target, color: '#2563EB' }
	]);
</script>

<section class="mx-auto max-w-6xl">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">
			{#if userName}Selamat datang kembali, {userName} 👋{:else}Selamat datang kembali 👋{/if}
		</h1>
		<p class="mt-1 text-sm text-muted-foreground">
			Kelola konten, distribusi publikasi, dan performa kampanye Anda dari satu tempat.
		</p>
	</header>

	{#if error}
		<p class="app-card mt-4 border-l-4 px-4 py-3 text-sm" style="color: var(--destructive);">{error}</p>
	{/if}

	{#if isBrandEmpty}
		<div class="app-card mt-4 flex flex-col items-center px-6 py-8 text-center">
			<Rocket size={32} style="color: var(--primary);" />
			<h2 class="mt-2 text-lg font-semibold">Selamat datang di Tauthub 👋</h2>
			<p class="mt-1 max-w-md text-sm text-muted-foreground">
				Mulai perjalanan konten Anda dengan membuat artikel pertama dan distribusikan ke media partner pilihan.
			</p>
			<a class="app-btn app-btn-primary mt-4" href={`/klien/${brandUrl}/campaigns`}>
				<Plus size={16} /> Buat Konten Pertama
			</a>
		</div>
	{/if}

	<!-- 4 kartu KPI + sparkline -->
	<section class="mt-5 grid gap-4 sm:grid-cols-2 xl:grid-cols-4" aria-label="Ringkasan cepat">
		{#each kpiCards as card (card.label)}
			<div class="app-card p-4">
				<div class="flex items-center justify-between gap-2">
					<div class="flex items-center gap-2">
						<span class="flex h-8 w-8 items-center justify-center rounded-lg" style="background-color: {card.bg}; color: {card.color};">
							<card.icon size={18} />
						</span>
						<span class="text-xs font-semibold text-muted-foreground">{card.label}</span>
					</div>
				</div>
				<div class="mt-2.5 flex items-end justify-between gap-2">
					<span class="text-2xl font-bold tabular-nums">{card.value}</span>
					<svg width="80" height="36" viewBox="0 0 80 36" aria-hidden="true" class="shrink-0">
						<polyline
							points={sparkPoints(card.spark)}
							fill="none"
							stroke={card.color}
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
						/>
					</svg>
				</div>
			</div>
		{/each}
	</section>

	<div class="mt-5 grid gap-4 lg:grid-cols-3">
		<!-- Kolom kiri -->
		<div class="grid gap-4 lg:col-span-2">
			<!-- Lanjutkan Pekerjaan -->
			<section class="app-card p-5">
				<div class="flex flex-wrap items-start justify-between gap-2">
					<div>
						<h2 class="font-semibold">Lanjutkan Pekerjaan</h2>
						<p class="mt-0.5 text-xs text-muted-foreground">Item konten atau draf terbaru yang memerlukan tindak lanjut Anda.</p>
					</div>
					{#if continueItems.length > 0}
						<a class="inline-flex items-center gap-1 text-xs font-medium" style="color: var(--primary);" href={`/klien/${brandUrl}/campaigns`}>
							Buka Kampanye &amp; Draf <ArrowRight size={14} />
						</a>
					{/if}
				</div>

				{#if loading && !data}
					<div class="mt-4 space-y-2">
						<div class="h-13 animate-pulse rounded-lg" style="background-color: var(--muted);"></div>
						<div class="h-13 animate-pulse rounded-lg" style="background-color: var(--muted);"></div>
					</div>
				{:else if continueItems.length > 0}
					<ul class="mt-3 divide-y">
						{#each continueItems as item (item.id)}
							<li class="flex items-center justify-between gap-3 py-3">
								<div class="min-w-0">
									<div class="flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground">
										<span class="rounded-full px-2 py-0.5 font-semibold" style="background-color: var(--accent); color: var(--accent-foreground);">{item.type}</span>
										{#if item.readiness !== undefined && item.readiness !== null}
											<span
												class="rounded-full px-2 py-0.5 font-semibold"
												style={item.readiness >= 75
													? 'background-color: var(--success-soft); color: var(--success);'
													: item.readiness >= 40
														? 'background-color: var(--warning-soft); color: var(--warning);'
														: 'background-color: color-mix(in srgb, var(--destructive) 12%, transparent); color: var(--destructive);'}
											>Readiness {item.readiness}%</span>
										{:else}
											<span class="rounded-full px-2 py-0.5 font-medium" style="background-color: var(--muted);">{item.statusLabel}</span>
										{/if}
										<span>· {formatRelativeTime(item.date)}</span>
									</div>
									<p class="mt-1 truncate text-sm font-semibold" title={item.title}>{item.title}</p>
								</div>
								<a class="app-btn app-btn-soft !min-h-8 shrink-0 !text-xs" href={item.actionUrl} aria-label="{item.actionText}: {item.title}">
									{item.actionText} <ArrowRight size={13} />
								</a>
							</li>
						{/each}
					</ul>
				{:else}
					<div class="mt-3 flex flex-col items-center rounded-lg border border-dashed px-6 py-8 text-center">
						<CheckCircle size={28} style="color: var(--success);" />
						<p class="mt-2 text-sm font-medium">Belum ada pekerjaan yang perlu dilanjutkan.</p>
						<p class="mt-1 text-xs text-muted-foreground">Semua tugas selesai. Mulai perjalanan konten Anda dengan membuat konten baru.</p>
						<a class="app-btn app-btn-soft mt-3 !text-xs" href={`/klien/${brandUrl}/campaigns`}><Plus size={14} /> Buat Konten Baru</a>
					</div>
				{/if}
			</section>

			<!-- Kampanye Aktif -->
			<section class="app-card p-5">
				<div class="flex flex-wrap items-start justify-between gap-2">
					<div>
						<h2 class="font-semibold">Kampanye Aktif</h2>
						<p class="mt-0.5 text-xs text-muted-foreground">Kampanye publikasi yang sedang berjalan saat ini.</p>
					</div>
					<a class="inline-flex items-center gap-1 text-xs font-medium" style="color: var(--primary);" href={`/klien/${brandUrl}/campaigns`}>
						Lihat semua kampanye <ArrowRight size={14} />
					</a>
				</div>

				{#if loading && !data}
					<div class="mt-4 space-y-2">
						<div class="h-24 animate-pulse rounded-lg" style="background-color: var(--muted);"></div>
						<div class="h-24 animate-pulse rounded-lg" style="background-color: var(--muted);"></div>
					</div>
				{:else if activeCampaigns.length > 0}
					<div class="mt-3 grid gap-3">
						{#each activeCampaigns.slice(0, 3) as campaign (campaign.id)}
							{@const quota = campaign.article_quota || 1}
							{@const count = campaign.publication_count || 0}
							{@const pct = Math.min(100, Math.round((count / quota) * 100))}
							<article class="rounded-xl border p-4">
								<div class="flex items-start justify-between gap-2">
									<div class="flex min-w-0 items-center gap-2">
										<span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg" style="background-color: var(--accent); color: var(--accent-foreground);">
											<Target size={16} />
										</span>
										<a class="truncate text-sm font-semibold hover:underline" href={`/klien/${brandUrl}/taut-pintar/${campaign.id}`}>{campaign.name}</a>
									</div>
									<span class="inline-flex shrink-0 items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-semibold" style="background-color: var(--success-soft); color: var(--success);">
										<span class="h-1.5 w-1.5 rounded-full" style="background-color: var(--success);"></span> Active
									</span>
								</div>

								<div class="mt-3">
									<div class="flex flex-wrap items-center justify-between gap-2 text-xs">
										<span class="text-muted-foreground">
											<strong class="text-foreground">{count}</strong> dari {campaign.article_quota > 0 ? `${campaign.article_quota} artikel` : 'artikel terbit'} ({pct}%)
										</span>
										{#if campaign.budget > 0}
											<span class="rounded-full px-2 py-0.5 font-medium" style="background-color: var(--muted);">Budget: {campaign.budget.toLocaleString('id-ID')} Kredit</span>
										{/if}
									</div>
									<div class="mt-2 h-2 overflow-hidden rounded-full" style="background-color: var(--muted);">
										<div class="h-full rounded-full" style="width: {Math.max(8, pct)}%; background-color: var(--primary);"></div>
									</div>
								</div>

								<div class="mt-3 flex items-center justify-between gap-2">
									<a class="app-btn app-btn-soft !min-h-8 !text-xs" href={`/klien/${brandUrl}/taut-pintar/${campaign.id}`}>
										{campaign.content_intake_id ? 'Edit Draf' : 'Buat Konten'} <ArrowRight size={13} />
									</a>
									<a class="text-xs text-muted-foreground underline" href={`/klien/${brandUrl}/campaigns`}>Detail Kampanye</a>
								</div>
							</article>
						{/each}
					</div>
				{:else}
					<div class="mt-3 flex flex-col items-center rounded-lg border border-dashed px-6 py-8 text-center">
						<Target size={24} style="color: var(--primary);" />
						<p class="mt-2 text-sm font-medium">Belum ada kampanye aktif.</p>
						<p class="mt-1 text-xs text-muted-foreground">Buat kampanye untuk mengatur alur publikasi dan distribusi media Anda.</p>
						<a class="app-btn app-btn-soft mt-3 !text-xs" href={`/klien/${brandUrl}/campaigns`}><Plus size={14} /> Buat Kampanye</a>
					</div>
				{/if}
			</section>
		</div>

		<!-- Kolom kanan -->
		<div class="grid content-start gap-4">
			<!-- Status Publikasi -->
			<section class="app-card p-5">
				<div class="flex flex-wrap items-start justify-between gap-2">
					<div>
						<h2 class="font-semibold">Status Publikasi</h2>
						<p class="mt-0.5 text-xs text-muted-foreground">Distribusi konten ke jaringan media partner.</p>
					</div>
					<a class="inline-flex items-center gap-1 text-xs font-medium" style="color: var(--primary);" href={`/klien/${brandUrl}/publications`}>
						Lihat Publikasi <ArrowRight size={14} />
					</a>
				</div>

				{#if loading && !data}
					<div class="mt-3 h-18 animate-pulse rounded-lg" style="background-color: var(--muted);"></div>
				{:else if publicationCounts.total > 0}
					<div class="mt-3 space-y-2">
						<div class="flex items-center justify-between">
							<span class="rounded-full px-2 py-0.5 text-xs font-semibold" style="background-color: var(--success-soft); color: var(--success);">Published</span>
							<span class="text-sm font-semibold tabular-nums">{publicationCounts.published}</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="rounded-full px-2 py-0.5 text-xs font-semibold" style="background-color: var(--warning-soft); color: var(--warning);">Pending</span>
							<span class="text-sm font-semibold tabular-nums">{publicationCounts.pending}</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="rounded-full px-2 py-0.5 text-xs font-semibold" style="background-color: color-mix(in srgb, var(--destructive) 12%, transparent); color: var(--destructive);">Failed</span>
							<span class="text-sm font-semibold tabular-nums">{publicationCounts.failed}</span>
						</div>
					</div>
					<div class="mt-3 flex h-2 overflow-hidden rounded-full" style="background-color: var(--muted);">
						<span style="width: {(publicationCounts.published / publicationCounts.total) * 100}%; background-color: var(--success);"></span>
						<span style="width: {(publicationCounts.pending / publicationCounts.total) * 100}%; background-color: var(--warning);"></span>
						<span style="width: {(publicationCounts.failed / publicationCounts.total) * 100}%; background-color: var(--destructive);"></span>
					</div>
				{:else}
					<div class="mt-3 flex flex-col items-center rounded-lg border border-dashed px-4 py-6 text-center">
						<Megaphone size={20} style="color: var(--primary);" />
						<p class="mt-1.5 text-xs text-muted-foreground">Belum ada data publikasi.</p>
					</div>
				{/if}
			</section>

			<!-- Mulai dengan Cepat -->
			<section class="app-card p-5">
				<h2 class="font-semibold">Mulai dengan Cepat</h2>
				<p class="mt-0.5 text-xs text-muted-foreground">Aksi cepat untuk alur kerja konten dan publikasi Anda.</p>
				<div class="mt-3 grid gap-2 sm:grid-cols-2">
					{#each quickActions as action (action.title)}
						<a class="flex items-start gap-3 rounded-lg border p-3 transition-colors hover:bg-secondary" href={action.href}>
							<span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg" style="background-color: color-mix(in srgb, {action.color} 14%, transparent); color: {action.color};">
								<action.icon size={16} />
							</span>
							<span class="min-w-0">
								<span class="block text-xs font-semibold">{action.title}</span>
								<span class="mt-0.5 block text-[11px] leading-snug text-muted-foreground">{action.desc}</span>
							</span>
						</a>
					{/each}
				</div>
			</section>

			<!-- Aktivitas Terbaru -->
			<section class="app-card p-5">
				<div class="flex flex-wrap items-start justify-between gap-2">
					<div>
						<h2 class="font-semibold">Aktivitas Terbaru</h2>
						<p class="mt-0.5 text-xs text-muted-foreground">Pembaruan terkini dari sistem dan draf konten Anda.</p>
					</div>
					<a class="inline-flex items-center gap-1 text-xs font-medium" style="color: var(--primary);" href={`/klien/${brandUrl}/campaigns`} aria-label="Lihat semua aktivitas">
						Lihat semua <ArrowRight size={14} />
					</a>
				</div>

				{#if loading && !data}
					<div class="mt-3 space-y-2">
						<div class="h-8 animate-pulse rounded" style="background-color: var(--muted);"></div>
						<div class="h-8 animate-pulse rounded" style="background-color: var(--muted);"></div>
						<div class="h-8 animate-pulse rounded" style="background-color: var(--muted);"></div>
					</div>
				{:else if dashboardStats?.recent_activity && dashboardStats.recent_activity.length > 0}
					<ul class="mt-3 space-y-2.5">
						{#each dashboardStats.recent_activity.slice(0, 4) as act (act.id)}
							<li class="flex items-start gap-2.5">
								<span class="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full" style="background-color: var(--muted);">
									<History size={13} />
								</span>
								<span class="min-w-0">
									<span class="block text-xs">{describeAction(act.action)}</span>
									<span class="block text-[11px] text-muted-foreground">{formatRelativeTime(act.created_at)}</span>
								</span>
							</li>
						{/each}
					</ul>
				{:else}
					<div class="mt-3 flex flex-col items-center rounded-lg border border-dashed px-4 py-6 text-center">
						<Clock size={20} style="color: var(--primary);" />
						<p class="mt-1.5 text-xs text-muted-foreground">Belum ada aktivitas terbaru.</p>
					</div>
				{/if}
			</section>
		</div>
	</div>
</section>
