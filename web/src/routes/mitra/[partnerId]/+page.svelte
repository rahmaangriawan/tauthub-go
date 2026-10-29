<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Ringkasan mitra — modern SaaS layout: 4 kartu KPI di atas, lalu layout
	// asimetris 2:1 berisi panel Status Publikasi (progress bar distribusi +
	// ringkasan 5 bucket status) dan panel Aktivitas Terbaru (notifikasi nyata
	// bertipe publikasi/pesan). Semua angka dari data aktual; tanpa data rekaan.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import type { Component } from 'svelte';
	import {
		Activity,
		ArrowRight,
		Check,
		CircleCheck,
		Clock,
		Layers,
		Megaphone,
		MessageCircle,
		Newspaper,
		RotateCcw,
		TriangleAlert,
		Wallet,
		X
	} from '@lucide/svelte';
	import { getPartnerUrlId } from '$lib/partner-url.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
	import StatCard from '$lib/components/stat-card.svelte';
	import { chatUnread, refreshChatUnreadNow } from '$lib/chat/chat-unread.svelte';
	import WaRequiredBanner from '$lib/components/wa-required-banner.svelte';

	const partnerId = $derived(page.params.partnerId as string);
	const base = $derived(`/mitra/${getPartnerUrlId() || partnerId}`);

	let loading = $state(true);
	let error = $state('');
	let me = $state<any>(null);
	// /v1/partner/me juga mengirim { user } — email dipakai di baris ringkas.
	let meUser = $state<{ email?: string } | null>(null);
	let earnings = $state<any>(null);
	let media = $state<any[]>([]);
	let pubError = $state(false);
	let publications = $state<any[]>([]);
	let activity = $state<any[]>([]);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			void refreshChatUnreadNow();
			const [meRes, earnRes, mediaRes, pubRes, notifRes] = await Promise.all([
				apiRequest('/v1/partner/me'),
				// Limit diperbesar khusus deret sparkline penghasilan.
				apiRequest('/v1/partner/earnings?limit=200'),
				apiRequest('/v1/partner/media?limit=500'),
				apiRequest('/v1/partner/publications?limit=200'),
				// Feed aktivitas: notifikasi umum terbaru, difilter ke event nyata.
				apiRequest('/v1/partner/notifications?limit=14&category=general')
			]);
			if (meRes.ok) {
				const meData = await meRes.json();
				me = meData.partner;
				meUser = meData.user ?? null;
			}
			if (earnRes.ok) earnings = await earnRes.json();
			if (mediaRes.ok) media = (await mediaRes.json()).media ?? [];
			pubError = !pubRes.ok;
			if (pubRes.ok) {
				const pubData = await pubRes.json();
				publications = (pubData?.publications ?? []) as any[];
				// Deret sparkline "masuk per hari" tetap dari tanggal publikasi.
				publicationTimes = publications
					.map((p) => Date.parse(p?.created_at ?? ''))
					.filter((t) => Number.isFinite(t));
			}
			if (notifRes.ok) {
				const notifData = await notifRes.json();
				activity = ((notifData?.notifications ?? []) as any[])
					.filter((n) => typeof n?.type === 'string' && (n.type.startsWith('publication.') || n.type === 'chat.message'))
					.slice(0, 2);
			}
			// Permintaan non-OK tidak boleh ditelan — tampilkan callout galat.
			const failed: string[] = [];
			if (!meRes.ok) failed.push(tr('Ringkasan mitra'));
			if (!earnRes.ok) failed.push(tr('Penghasilan'));
			if (!mediaRes.ok) failed.push(tr('Media'));
			if (!pubRes.ok) failed.push(tr('Publikasi'));
			if (!notifRes.ok) failed.push(tr('Aktivitas'));
			if (failed.length > 0)
				error = `${tr('Gagal memuat')}: ${failed.join(', ')}. ${tr('Coba muat ulang halaman.')}`;
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	let publicationTimes = $state<number[]>([]);

	const fmt = (n: number) => (n ?? 0).toLocaleString('id-ID');
	const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}`;
	// Soft tint util (dipakai callout error, banner, dan badge status).
	const soft = (token: string) => `color-mix(in srgb, var(${token}) 26%, transparent)`;

	// ── Deret sparkline: murni dari data API (created_at / amount) ───────────
	// Tidak ada seri rekaan; tiap titik dihitung dari record nyata.
	const DAY_MS = 86_400_000;
	const endOfToday = () => {
		const d = new Date();
		d.setHours(23, 59, 59, 999);
		return d.getTime();
	};
	// Kumulatif: nilai hari ke-i = total item dengan timestamp <= akhir hari itu.
	function cumulativeSeries(items: { t: number; v: number }[], days: number): number[] {
		const end = endOfToday();
		return Array.from({ length: days }, (_, i) => {
			const cutoff = end - (days - 1 - i) * DAY_MS;
			return items.reduce((sum, it) => sum + (it.t <= cutoff ? it.v : 0), 0);
		});
	}
	// Per-hari: jumlah item yang jatuh di tiap bucket hari dalam jendela.
	function dailySeries(times: number[], days: number): number[] {
		const start = endOfToday() - (days - 1) * DAY_MS;
		const buckets = new Array(days).fill(0) as number[];
		for (const t of times) {
			const idx = Math.floor((t - start) / DAY_MS);
			if (idx >= 0 && idx < days) buckets[idx] += 1;
		}
		return buckets;
	}
	const toTime = (v: unknown): number => (typeof v === 'string' ? Date.parse(v) : NaN);

	// Media: kumulatif dari media.created_at (endpoint /v1/partner/media, limit 500).
	const mediaSeries = $derived(
		cumulativeSeries(
			media.map((m) => ({ t: toTime(m?.created_at), v: 1 })).filter((it) => Number.isFinite(it.t)),
			30
		)
	);
	// Penghasilan: kumulatif dari earnings[].amount + created_at (kecuali voided).
	const earningSeries = $derived(
		cumulativeSeries(
			(earnings?.earnings ?? [])
				.filter((e: any) => e?.status !== 'voided')
				.map((e: any) => ({ t: toTime(e?.created_at), v: Number(e?.amount) || 0 }))
				.filter((it: { t: number }) => Number.isFinite(it.t)),
			30
		)
	);
	// Menunggu: permintaan artikel masuk per hari dari publications[].created_at.
	const pubSeries = $derived(dailySeries(publicationTimes, 14));

	// Total penghasilan aktual = lifetime penghasilan publikasi (net, tanpa voided)
	// dari backend; fallback ke settled + pending bila field belum tersedia.
	const totalEarnings = $derived(
		earnings?.summary?.lifetime_publication ??
			(earnings?.summary?.settled ?? 0) + (earnings?.summary?.pending ?? 0)
	);
	const pendingCount = $derived(me?.pending_count ?? 0);

	// ── Distribusi status (5 bucket dari publication_counts backend) ─────────
	type Bucket = {
		key: string;
		label: string;
		count: number;
		color: string;
		pct: number;
		tooltip?: string;
	};
	const statusBuckets = $derived.by<Bucket[]>(() => {
		const c: Record<string, any> = me?.publication_counts ?? {};
		const g = (keys: string[]) => keys.reduce((sum, k) => sum + (Number(c[k]) || 0), 0);
		const lainnyaDetail = (['pending', 'revision', 'cancelled', 'partner_rejected'] as const)
			.map((k) => `${statusName(k)} ${Number(c[k]) || 0}`)
			.filter((s) => !s.endsWith(' 0'))
			.join(' · ');
		const buckets: Omit<Bucket, 'pct'>[] = [
			{ key: 'menunggu', label: tr('Menunggu'), count: g(['waiting_partner']), color: 'var(--warning)' },
			{
				key: 'diproses',
				label: tr('Diproses'),
				count: g(['partner_approved', 'queued', 'processing', 'retrying']),
				color: 'var(--primary)'
			},
			{ key: 'terbit', label: tr('Terbit'), count: g(['published']), color: 'var(--success)' },
			{ key: 'gagal', label: tr('Gagal'), count: g(['failed']), color: 'var(--destructive)' },
			{
				key: 'lainnya',
				label: tr('Lainnya'),
				count: g(['pending', 'revision', 'cancelled', 'partner_rejected']),
				color: 'var(--muted-foreground)',
				tooltip: lainnyaDetail
			}
		];
		const total = buckets.reduce((sum, b) => sum + b.count, 0);
		return buckets.map((b) => ({ ...b, pct: total > 0 ? (b.count / total) * 100 : 0 }));
	});
	const totalStatusCount = $derived(statusBuckets.reduce((sum, b) => sum + b.count, 0));
	const statusSegments = $derived(statusBuckets.filter((b) => b.count > 0));

	function statusName(status: string): string {
		const map: Record<string, string> = {
			pending: tr('Menunggu'),
			revision: tr('Perlu Revisi'),
			cancelled: tr('Dibatalkan'),
			partner_rejected: tr('Ditolak')
		};
		return map[status] ?? status;
	}
	// "0%" hanya untuk bucket nol; bucket kecil (0<pct<0.5) tetap jelas "<1%".
	const pctText = (pct: number, count: number) =>
		count > 0 && Math.round(pct) < 1 ? tr('<1%') : `${Math.round(pct)}%`;

	// Deret datar? pembanding 30-hari = nilai kumulatif hari pertama jendela.
	// Kembali null (→ tampil "—") bila pembanding nol: tanpa klaim persentase
	// dari data yang tidak ada (spec §5).
	const monthPct = (series: number[]): number | null => {
		if (!series?.length) return null;
		const cur = series[series.length - 1];
		const prev = series[0];
		if (prev <= 0) return null;
		return ((cur - prev) / prev) * 100;
	};
	const mediaDelta = $derived(monthPct(mediaSeries));
	const earningDelta = $derived(monthPct(earningSeries));

	// ── Daftar publikasi terbaru (panel bawah) ───────────────────────────────
	const dt = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID') : '-');
	const latestPublications = $derived(publications.slice(0, 6));

	const badgeStyles: Record<string, string> = {
		published: `background-color: var(--success-soft); color: var(--success); box-shadow: inset 0 0 0 1px ${soft('--success')};`,
		waiting_partner: `background-color: var(--warning-soft); color: var(--warning); box-shadow: inset 0 0 0 1px ${soft('--warning')};`,
		revision: `background-color: var(--warning-soft); color: var(--warning); box-shadow: inset 0 0 0 1px ${soft('--warning')};`,
		partner_approved: `background-color: var(--accent); color: var(--primary); box-shadow: inset 0 0 0 1px ${soft('--primary')};`,
		partner_rejected: `background-color: ${soft('--destructive')}; color: var(--destructive); box-shadow: inset 0 0 0 1px ${soft('--destructive')};`,
		failed: `background-color: ${soft('--destructive')}; color: var(--destructive); box-shadow: inset 0 0 0 1px ${soft('--destructive')};`
	};
	const neutralBadge = 'background-color: var(--muted); color: var(--muted-foreground); box-shadow: inset 0 0 0 1px var(--border);';
	const badgeStyle = (status: string) => badgeStyles[status] ?? neutralBadge;
	const badgeLabels: Record<string, string> = $derived({
		published: tr('Terbit'),
		waiting_partner: tr('Menunggu'),
		revision: tr('Perlu Revisi'),
		partner_approved: tr('Disetujui'),
		partner_rejected: tr('Ditolak'),
		failed: tr('Gagal'),
		queued: tr('Antrean'),
		processing: tr('Diproses'),
		retrying: tr('Coba ulang'),
		cancelled: tr('Dibatalkan')
	});
	const badgeLabel = (status: string) => badgeLabels[status] ?? status;

	function pubIcon(status: string): Component<{ size?: number }> {
		switch (status) {
			case 'published':
				return CircleCheck;
			case 'waiting_partner':
				return Clock;
			case 'revision':
				return RotateCcw;
			case 'partner_approved':
				return Check;
			case 'partner_rejected':
				return X;
			case 'failed':
				return TriangleAlert;
			default:
				return Megaphone;
		}
	}
	function pubIconBoxStyle(status: string): string {
		if (status === 'published')
			return `background-color: var(--success-soft); color: var(--success); box-shadow: inset 0 0 0 1px ${soft('--success')};`;
		if (status === 'waiting_partner' || status === 'revision')
			return `background-color: var(--warning-soft); color: var(--warning); box-shadow: inset 0 0 0 1px ${soft('--warning')};`;
		if (status === 'partner_approved')
			return `background-color: var(--accent); color: var(--primary); box-shadow: inset 0 0 0 1px ${soft('--primary')};`;
		return 'background-color: var(--muted); color: var(--muted-foreground); box-shadow: inset 0 0 0 1px var(--border);';
	}

	// ── Aktivitas: notifikasi publikasi/pesan dari backend (tanpa data rekaan) ─
	function activityIcon(type: string): Component<{ size?: number }> {
		switch (type) {
			case 'publication.published':
				return CircleCheck;
			case 'publication.waiting_partner':
				return Clock;
			case 'publication.failed':
				return TriangleAlert;
			case 'publication.revision_requested':
				return RotateCcw;
			case 'publication.partner_approved':
				return Check;
			case 'publication.partner_rejected':
				return X;
			case 'chat.message':
				return MessageCircle;
			default:
				return Megaphone;
		}
	}
	function activityIconStyle(type: string): string {
		if (type === 'publication.published') return 'background: var(--success-soft); color: var(--success);';
		if (type === 'publication.waiting_partner' || type === 'publication.revision_requested')
			return 'background: var(--warning-soft); color: var(--warning);';
		if (type === 'publication.failed' || type === 'publication.partner_rejected')
			return 'background: color-mix(in srgb, var(--destructive) 12%, transparent); color: var(--destructive);';
		return 'background: var(--accent); color: var(--primary);';
	}

	function timeAgo(value: string | null | undefined): string {
		if (!value) return '—';
		const t = Date.parse(value);
		if (!Number.isFinite(t)) return '—';
		const diffMin = Math.floor((Date.now() - t) / 60_000);
		if (diffMin < 1) return tr('baru saja');
		if (diffMin < 60) return `${diffMin} ${tr('menit lalu')}`;
		const hours = Math.floor(diffMin / 60);
		if (hours < 24) return `${hours} ${tr('jam lalu')}`;
		const days = Math.floor(hours / 24);
		if (days <= 7) return `${days} ${tr('hari lalu')}`;
		return new Date(t).toLocaleDateString('id-ID', { day: 'numeric', month: 'short' });
	}
</script>

<PageTitle title={tr("Ringkasan Mitra")} />

<div class="mb-1">
	<WaRequiredBanner show={me?.wa_required === true} mediaCount={me?.media_count ?? 0} profileHref={`${base}/profile`} />
</div>

<section class="flex w-full flex-col gap-5">
	<!-- Header / sambutan -->
	<header class="flex w-full flex-wrap items-start justify-between gap-4 pt-1 pb-0.5">
		<div class="flex flex-1 flex-col gap-1">
			<!-- HP: nama di baris sendiri, lalu lencana status sejajar dengan baris
			     keterangan. Desktop tetap menaruh lencana di samping nama. -->
			<div class="flex flex-wrap items-center gap-x-3 gap-y-1.5">
				<h1 class="m-0 max-sm:w-full text-[24px] leading-[1.2] font-bold tracking-[-0.025em]">
					{tr('Halo')}, {me?.name ?? tr('Mitra')}
				</h1>
				{#if me?.status}
					<span
						class="inline-flex items-center gap-1.5 rounded-[6px] px-2 py-0.5 text-[11.5px] font-semibold capitalize"
						style="background-color: var(--success-soft); color: var(--success); border: 1px solid {soft('--success')};"
					>
						<span class="h-1.5 w-1.5 rounded-full" style="background-color: var(--success);"></span>
						<span>{me.status}</span>
					</span>
				{/if}
			</div>
			<p class="m-0 text-[13.5px] leading-normal text-muted-foreground sm:w-full">
				{me?.media_count ?? 0} {tr("media terdaftar")}{#if meUser?.email} · {meUser.email}{/if}
			</p>
		</div>
		<div class="flex gap-2">
			<a class="app-btn app-btn-soft" href={`${base}/media`}>{tr('Kelola Media')}</a>
			<a class="app-btn app-btn-primary" href={`${base}/publications`}>{tr("Lihat Publikasi")}</a>
		</div>
	</header>

	{#if error}
		<div
			class="flex flex-wrap items-center justify-between gap-3 rounded-[10px] px-4 py-3 text-[13px]"
			style="background-color: {soft('--destructive')}; color: var(--destructive); border: 1px solid {soft('--destructive')};"
		>
			<span>{error}</span>
			<button type="button" class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" onclick={() => window.location.reload()}>
				{tr('Muat ulang')}
			</button>
		</div>
	{/if}

	{#if loading}
		<!-- Skeleton: bentuk meniru layout final (4 kartu + 2 panel). -->
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
			{#each Array.from({ length: 4 }) as _}
				<div class="app-card min-h-[138px] animate-pulse" style="background-color: var(--secondary);"></div>
			{/each}
		</div>
		<div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
			<div class="app-card min-h-[280px] animate-pulse lg:col-span-2" style="background-color: var(--secondary);"></div>
			<div class="app-card min-h-[280px] animate-pulse" style="background-color: var(--secondary);"></div>
		</div>
	{:else}
		<!-- 1. Ringkasan statistik — 4 kartu -->
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
			<StatCard
				label={tr('Media Anda')}
				value={fmt(me?.media_count ?? 0)}
				hint={tr('Media aktif terdaftar')}
				icon={Newspaper}
				delta={mediaDelta}
				sparkline={{ values: mediaSeries, stroke: 'var(--primary)', title: tr('Media terdaftar kumulatif, 30 hari terakhir') }}
			/>

			<StatCard
				label={tr('Menunggu')}
				value={fmt(pendingCount)}
				hint={pendingCount > 0 ? tr('Perlu tindakan segera') : tr('Semua publikasi telah diproses')}
				tone={pendingCount > 0 ? 'warning' : 'neutral'}
				icon={Clock}
				sparkline={{ values: pubSeries, stroke: 'var(--warning)', title: tr('Permintaan artikel masuk per hari, 14 hari terakhir') }}
			/>

			<StatCard
				label={tr('Total Penghasilan')}
				value={rp(totalEarnings)}
				hint={tr('Penghasilan dari artikel terbit')}
				tone="success"
				icon={Wallet}
				delta={earningDelta}
				sparkline={{ values: earningSeries, stroke: 'var(--success)', title: tr('Penghasilan kumulatif (nilai aktual per hari), 30 hari terakhir') }}
			/>

			<StatCard
				label={tr('Pesan')}
				value={fmt(chatUnread.count)}
				hint={chatUnread.count > 0 ? tr('Pesan klien belum dibaca') : tr('Semua pesan klien sudah dibaca')}
				tone={chatUnread.count > 0 ? 'primary' : 'neutral'}
				icon={MessageCircle}
				href={`${base}/chat`}
			/>
		</div>

		<!-- 2. Area operasional — Status Publikasi (2/3) + Aktivitas Terbaru (1/3) -->
		<div class="grid grid-cols-1 gap-4 lg:grid-cols-3">
			<section class="app-card flex flex-col overflow-hidden lg:col-span-2">
				<div class="flex flex-wrap items-start justify-between gap-3 border-b px-5 pt-4 pb-4">
					<div class="flex items-start gap-3">
						<div
							class="flex h-9 w-9 shrink-0 items-center justify-center rounded-[10px]"
							style="background-color: var(--accent); color: var(--primary); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--primary) 20%, transparent);"
						>
							<Layers size={17} />
						</div>
						<div>
							<h2 class="m-0 text-[15px] font-semibold tracking-[-0.01em]">{tr('Status Publikasi')}</h2>
							<p class="m-0 mt-0.5 text-[12.5px] text-muted-foreground">{tr('Ringkasan seluruh artikel Anda saat ini.')}</p>
						</div>
					</div>
					<a
						class="inline-flex shrink-0 items-center gap-1 rounded-[6px] px-2 py-1 text-[12.5px] font-semibold transition-colors hover:bg-accent"
						style="color: var(--primary);"
						href={`${base}/publications`}
					>
						{tr('Lihat semua')}
						<ArrowRight size={13} />
					</a>
				</div>

				<div class="flex flex-1 flex-col justify-center gap-5 px-5 py-5">
					{#if totalStatusCount === 0}
						<p class="m-auto text-center text-[13px] text-muted-foreground">{tr('Belum ada publikasi')}</p>
					{:else}
						<div
							class="flex h-2.5 w-full overflow-hidden rounded-full"
							style="background-color: var(--secondary);"
							role="img"
							aria-label={statusBuckets.map((b) => `${b.label} ${b.count}`).join(', ')}
						>
							{#each statusSegments as seg (seg.key)}
								<div
									style={`width: ${seg.pct}%; background-color: ${seg.color};`}
									title={`${seg.label}: ${fmt(seg.count)} (${pctText(seg.pct, seg.count)})`}
								></div>
							{/each}
						</div>

						<div class="grid grid-cols-2 gap-x-4 gap-y-5 sm:grid-cols-3 lg:grid-cols-5">
							{#each statusBuckets as b, i (b.key)}
								<div class="flex flex-col gap-1.5 {i > 0 ? 'lg:border-l lg:border-border lg:pl-4' : ''}" title={b.tooltip}>
									<div class="flex items-baseline gap-2">
										<span class="h-2 w-2 shrink-0 translate-y-[-1px] rounded-full" style={`background-color: ${b.color};`}></span>
										<span class="text-[20px] leading-none font-bold tabular-nums">{fmt(b.count)}</span>
									</div>
									<span class="flex flex-wrap items-center gap-x-1.5 text-[12px] text-muted-foreground">
										{b.label}
										<span class="font-medium" style={`color: ${b.count > 0 ? b.color : 'var(--muted-foreground)'};`}>{pctText(b.pct, b.count)}</span>
									</span>
								</div>
							{/each}
						</div>
					{/if}
				</div>
			</section>

			<section class="app-card flex flex-col overflow-hidden">
				<div class="relative flex items-start gap-3 border-b px-5 pt-4 pb-4">
					<div
						class="flex h-9 w-9 shrink-0 items-center justify-center rounded-[10px]"
						style="background-color: var(--accent); color: var(--primary); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--primary) 20%, transparent);"
					>
						<Activity size={17} />
					</div>
					<div class="min-w-0 flex-1 pr-16">
						<h2 class="m-0 text-[15px] font-semibold tracking-[-0.01em]">{tr('Aktivitas Terbaru')}</h2>
						<p class="m-0 mt-0.5 text-[12.5px] text-muted-foreground">{tr('Pantau publikasi dan klien Anda.')}</p>
					</div>
					<a
						class="absolute right-5 top-[17px] inline-flex shrink-0 items-center gap-1 rounded-[6px] px-2 py-1 text-[12.5px] font-semibold transition-colors hover:bg-accent"
						style="color: var(--primary);"
						href={`${base}/chat`}
					>
						{tr('Lihat semua')}
						<ArrowRight size={13} />
					</a>
				</div>

				{#if activity.length === 0}
					<div class="flex flex-1 flex-col items-center gap-2.5 px-6 py-12 text-center">
						<div
							class="flex h-11 w-11 items-center justify-center rounded-full"
							style="background-color: var(--accent); color: var(--primary);"
						>
							<Activity size={19} />
						</div>
						<h3 class="m-0 text-[14px] font-semibold">{tr('Belum ada aktivitas terbaru')}</h3>
						<p class="m-0 max-w-[260px] text-[12.5px] leading-relaxed text-muted-foreground">
							{tr('Aktivitas akan muncul di sini setelah ada pembaruan pada publikasi atau pesan.')}
						</p>
					</div>
				{:else}
					<ul class="flex flex-1 flex-col divide-y divide-border">
						{#each activity as n (n.id)}
							{@const NIcon = activityIcon(n.type ?? '')}
							{@const hrefOf = (n.link as string) || `${base}/publications`}
							<li>
								<a href={hrefOf} class="flex items-start gap-3 px-5 py-3.5 transition-colors last:pb-4 hover:bg-accent">
									<div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-[9px]" style={activityIconStyle(n.type ?? '')}>
										<NIcon size={15} />
									</div>
									<div class="min-w-0 flex-1">
										<p class="truncate text-[13px] font-semibold" title={n.title}>{n.title}</p>
										{#if typeof n.body === 'string' && n.body}
											<p class="mt-0.5 truncate text-[11.5px] text-muted-foreground" title={n.body}>{n.body}</p>
										{/if}
									</div>
									<span class="shrink-0 pt-0.5 text-[11px] text-muted-foreground">{timeAgo(n.created_at)}</span>
								</a>
							</li>
						{/each}
					</ul>
				{/if}
			</section>
		</div>

		<!-- 3. Publikasi Terbaru (list, full-width) -->
		<div class="app-card overflow-hidden">
			<div class="flex flex-wrap items-center justify-between gap-3 border-b" style="padding: 16px 20px 14px;">
				<div class="flex flex-col gap-0.5">
					<h2 class="m-0 text-[15px] font-semibold tracking-[-0.01em]">{tr('Publikasi Terbaru')}</h2>
					<p class="m-0 text-[12.5px] text-muted-foreground">{tr('Riwayat distribusi artikel terbaru untuk media Anda')}</p>
				</div>
				<a
					class="inline-flex items-center gap-1 rounded-[6px] px-2 py-1 text-[12.5px] font-semibold transition-colors hover:bg-accent"
					style="color: var(--primary);"
					href={`${base}/publications`}
				>
					{tr('Lihat semua')}
					<ArrowRight size={13} />
				</a>
			</div>

			{#if pubError}
				<p class="px-5 py-8 text-center text-[13px]" style="color: var(--destructive);">{tr('Gagal memuat publikasi terbaru.')}</p>
			{:else if publications.length === 0}
				<div class="flex flex-col items-center gap-2 px-5 py-[44px] text-center text-muted-foreground">
					<Newspaper size={34} />
					<h3 class="m-0 text-[14px] font-semibold text-foreground">{tr('Belum ada publikasi')}</h3>
					<p class="m-0 text-[13px] text-muted-foreground">{tr('Publikasi dari klien Tauthub akan muncul di sini setelah tersedia.')}</p>
				</div>
			{:else}
				<div class="flex flex-col">
					{#each latestPublications as p (p.id)}
						{@const PubIcon = pubIcon(p.status)}
						{@const cover = p.thumbnail_url ?? p.featured_image_url ?? null}
						<div class="flex flex-col items-start justify-between gap-2 border-b px-5 py-[13px] transition-colors last:border-b-0 hover:bg-accent sm:flex-row sm:items-center sm:gap-4">
							<div class="flex min-w-0 items-center gap-3">
								<!-- Sampul artikel lebih informatif daripada ikon status; ikon
								     tetap dipakai bila artikel belum punya sampul. -->
								{#if cover}
									<img
										src={cover}
										alt=""
										class="h-[38px] w-[56px] shrink-0 rounded-[8px] border object-cover"
										style="border-color: var(--border); background-color: var(--muted);"
										loading="lazy"
										decoding="async"
									/>
								{:else}
									<div class="flex h-[38px] w-[38px] shrink-0 items-center justify-center rounded-[10px]" style={pubIconBoxStyle(p.status)}>
										<PubIcon size={18} />
									</div>
								{/if}
								<div class="flex min-w-0 flex-col gap-0.5">
									<div class="line-clamp-2 break-words text-[13.5px] leading-[1.35] font-semibold" title={p.document_title ?? tr('Artikel')}>
										<span>{p.document_title ?? tr('Artikel')}</span>
										{#if p.media_name}
											<span class="font-medium" style="color: var(--primary);"> · {p.media_name}</span>
										{/if}
									</div>
									<div class="flex flex-wrap items-center gap-1.5 text-[12px] text-muted-foreground">
										<span>{p.order_code ?? tr('Tanpa kode')}</span>
										<span style="color: var(--border);">·</span>
										<span>{dt(p.created_at)}</span>
									</div>
								</div>
							</div>
							<div class="flex w-full min-w-0 items-center justify-between gap-3 border-t pt-2 sm:w-auto sm:justify-end sm:border-t-0 sm:pt-0">
								<span class="inline-flex items-center gap-1.5 rounded-[6px] px-2 py-[3px] text-[12px] font-semibold" style={badgeStyle(p.status)}>
									{badgeLabel(p.status)}
								</span>
								<span class="shrink-0 text-right text-[13.5px] font-bold tabular-nums">{rp(p.partner_amount ?? 0)}</span>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</section>
