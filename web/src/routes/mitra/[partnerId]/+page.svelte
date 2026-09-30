<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Ringkasan mitra - port setia section "Ringkasan" portal mitra versi lama:
	// header + pil status, 3 kartu ringkasan (media / menunggu / total penghasilan),
	// strip 4 statistik publikasi, dan panel Publikasi Terbaru. Blok tambahan yang
	// sudah ada (saldo, media terbaru, notifikasi, tabel penghasilan) dipertahankan
	// dengan sistem visual yang sama.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		ArrowRight,
		Check,
		CircleCheck,
		Clock,
		Megaphone,
		Newspaper,
		RotateCcw,
		TriangleAlert,
		Wallet,
		X
	} from '@lucide/svelte';
	import { getPartnerUrlId } from '$lib/partner-url.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
		import Sparkline from '$lib/components/sparkline.svelte';

	const partnerId = $derived(page.params.partnerId as string);
	const base = $derived(`/mitra/${getPartnerUrlId() || partnerId}`);

	let loading = $state(true);
	let error = $state('');
	let pubError = $state(false);
	let me = $state<any>(null);
	let earnings = $state<any>(null);
	let media = $state<any[]>([]);
	let publications = $state<any[]>([]);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			const [meRes, earnRes, mediaRes, pubRes] = await Promise.all([
				apiRequest('/v1/partner/me'),
				// Limit diperbesar khusus untuk deret sparkline; tabel tetap 5 baris (slice).
				apiRequest('/v1/partner/earnings?limit=200'),
				apiRequest('/v1/partner/media?limit=500'),
				apiRequest('/v1/partner/publications?limit=200')
			]);
			if (meRes.ok) me = (await meRes.json()).partner;
			if (earnRes.ok) earnings = await earnRes.json();
			if (mediaRes.ok) media = (await mediaRes.json()).media ?? [];
			if (pubRes.ok) publications = (await pubRes.json()).publications ?? [];
			// Permintaan non-OK tidak boleh ditelan - tampilkan callout galat.
			const failed: string[] = [];
			if (!meRes.ok) failed.push(tr('Ringkasan mitra'));
			if (!earnRes.ok) failed.push(tr('Penghasilan'));
			if (!mediaRes.ok) failed.push(tr('Media'));
			if (!pubRes.ok) failed.push(tr('Publikasi'));
			pubError = !pubRes.ok;
			if (failed.length > 0)
				error = `${tr('Gagal memuat')}: ${failed.join(', ')}. ${tr('Coba muat ulang halaman.')}`;
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	const stats = $derived(me?.publication_stats ?? {});
	const fmt = (n: number) => (n ?? 0).toLocaleString('id-ID');
	const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}`;
	// Total penghasilan = yang sudah settle + yang masih menunggu (parity kartu lama).
	const totalEarnings = $derived((earnings?.summary?.settled ?? 0) + (earnings?.summary?.pending ?? 0));
	const dt = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID') : '-');

	// ── Deret sparkline: murni dari data API (created_at / amount) ───────────
	// Tidak ada seri rekaan/smoothstep; tiap titik dihitung dari record nyata.
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
	const pubSeries = $derived(
		dailySeries(
			publications.map((p) => toTime(p?.created_at)).filter((t) => Number.isFinite(t)),
			14
		)
	);

	// Panel tetap menampilkan baris terbaru saja walau limit fetch diperbesar.
	const latestPublications = $derived(publications.slice(0, 6));

	// ── Token visual (dipetakan dari mitra-overview.module.css) ──────────────
	// gray-a5/gray-10/gray-12 → border / muted-foreground / foreground;
	// green → success(+soft); amber → warning(+soft); red → destructive; accent → primary.
	type Tone = 'neutral' | 'pending' | 'earning';
	const soft = (token: string) => `color-mix(in srgb, var(${token}) 26%, transparent)`;

	const cardStyle = (tone: Tone) =>
		tone === 'earning'
			? `border-color: ${soft('--success')}; background-color: var(--success-soft);`
			: tone === 'pending'
				? `border-color: ${soft('--warning')}; background-color: var(--warning-soft);`
				: '';
	const iconBoxStyle = (tone: Tone) =>
		tone === 'earning'
			? `background-color: var(--success-soft); color: var(--success); border: 1px solid ${soft('--success')};`
			: tone === 'pending'
				? `background-color: var(--warning-soft); color: var(--warning); border: 1px solid ${soft('--warning')};`
				: `background-color: var(--accent); color: var(--accent-foreground); border: 1px solid ${soft('--primary')};`;
	const labelStyle = (tone: Tone) =>
		tone === 'earning' ? 'color: var(--success);' : tone === 'pending' ? 'color: var(--warning);' : '';
	const valueStyle = (tone: Tone) => (tone === 'earning' ? 'color: var(--success);' : '');
	const hintStyle = (tone: Tone) =>
		tone === 'earning'
			? 'color: var(--success); font-weight: 500;'
			: tone === 'pending'
				? 'color: var(--warning); font-weight: 500;'
				: '';

	// ── Status publikasi (badge + ikon) ──────────────────────────────────────
	const badgeStyles: Record<string, string> = {
		published: `background-color: var(--success-soft); color: var(--success); border: 1px solid ${soft('--success')};`,
		waiting_partner: `background-color: var(--warning-soft); color: var(--warning); border: 1px solid ${soft('--warning')};`,
		revision: `background-color: var(--warning-soft); color: var(--warning); border: 1px solid ${soft('--warning')};`,
		partner_approved: `background-color: var(--accent); color: var(--accent-foreground); border: 1px solid ${soft('--primary')};`,
		partner_rejected: `background-color: ${soft('--destructive')}; color: var(--destructive); border: 1px solid ${soft('--destructive')};`,
		rejected: `background-color: ${soft('--destructive')}; color: var(--destructive); border: 1px solid ${soft('--destructive')};`,
		failed: `background-color: ${soft('--destructive')}; color: var(--destructive); border: 1px solid ${soft('--destructive')};`
	};
	const neutralBadge = `background-color: var(--muted); color: var(--muted-foreground); border: 1px solid var(--border);`;
	const badgeStyle = (status: string) => badgeStyles[status] ?? neutralBadge;

	const badgeLabels: Record<string, string> = $derived({
		published: tr('Terbit'),
		waiting_partner: tr('Menunggu'),
		revision: tr('Perlu Revisi'),
		partner_approved: tr('Disetujui'),
		partner_rejected: tr('Ditolak'),
		rejected: tr('Ditolak'),
		failed: tr('Gagal'),
		queued: tr('Antrean'),
		processing: tr('Diproses'),
		retrying: tr('Coba ulang'),
		cancelled: tr('Dibatalkan')
	});
	const badgeLabel = (status: string) => badgeLabels[status] ?? status;

	function pubIcon(status: string) {
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
			case 'rejected':
				return X;
			case 'failed':
				return TriangleAlert;
			default:
				return Megaphone;
		}
	}
	function pubIconBoxStyle(status: string) {
		if (status === 'published')
			return `background-color: var(--success-soft); color: var(--success); border: 1px solid ${soft('--success')};`;
		if (status === 'waiting_partner')
			return `background-color: var(--warning-soft); color: var(--warning); border: 1px solid ${soft('--warning')};`;
		if (status === 'revision')
			return `background-color: var(--warning-soft); color: var(--warning); border: 1px solid ${soft('--warning')};`;
		if (status === 'partner_approved')
			return `background-color: var(--accent); color: var(--accent-foreground); border: 1px solid ${soft('--primary')};`;
		return `background-color: var(--muted); color: var(--muted-foreground); border: 1px solid var(--border);`;
	}
</script>

<PageTitle title={tr("Ringkasan Mitra")} />

<section class="flex w-full flex-col gap-5">
	<!-- 1. Header / sambutan (metrik dari mitra-overview.module.css) -->
	<header class="flex w-full flex-wrap items-start justify-between gap-4 pt-1 pb-0.5">
		<div class="flex flex-1 flex-col gap-1">
			<div class="flex flex-wrap items-center gap-3">
				<h1 class="m-0 text-[24px] leading-[1.2] font-bold tracking-[-0.025em]">
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
			<p class="m-0 text-[13.5px] leading-normal text-muted-foreground">
				{me?.media_count ?? 0} {tr("media terdaftar")}{#if me?.public_id} · ID mitra {me.public_id}{/if}
			</p>
		</div>
		<div class="flex gap-2">
			<a class="app-btn app-btn-soft" href={`${base}/media`}>{tr('Kelola Media')}</a>
			<a class="app-btn app-btn-primary" href={`${base}/publications`}>{tr("Lihat Publikasi")}</a>
		</div>
	</header>

	{#if error}
		<div
			class="rounded-[10px] px-4 py-3 text-[13px]"
			style="background-color: {soft('--destructive')}; color: var(--destructive); border: 1px solid {soft('--destructive')};"
		>
			{error}
		</div>
	{/if}

	{#if loading}
		<p class="text-sm text-muted-foreground">{tr("Memuat ringkasan…")}</p>
	{:else}
		<!-- 2. Tiga kartu ringkasan -->
		<div class="grid grid-cols-1 gap-3.5 md:grid-cols-3">
			<!-- Media Anda -->
			<div class="app-card flex flex-col justify-between gap-2.5" style="padding: 16px 18px;">
				<div class="flex items-center justify-between gap-2">
					<span class="text-[11px] font-bold uppercase tracking-[0.05em] text-muted-foreground">{tr("Media Anda")}</span>
					<div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-[8px]" style={iconBoxStyle('neutral')}>
						<Newspaper size={17} />
					</div>
				</div>
				<div>
					<div class="my-0.5 text-[24px] leading-[1.15] font-bold tracking-[-0.03em] tabular-nums" style={valueStyle('neutral')}>{fmt(me?.media_count ?? 0)}</div>
					<div class="text-[12px] leading-[1.35] text-muted-foreground" style={hintStyle('neutral')}>{tr("Media aktif terdaftar")}</div>
				</div>
				<Sparkline
					values={mediaSeries}
					stroke="var(--primary)"
					title={tr('Media terdaftar kumulatif, 30 hari terakhir')}
					class="mt-0.5"
				/>
			</div>

			<!-- Menunggu (alert bila ada tindakan) -->
			{#if (me?.pending_count ?? 0) > 0}
				<div class="app-card flex flex-col justify-between gap-2.5" style="padding: 16px 18px; {cardStyle('pending')}">
					<div class="flex items-center justify-between gap-2">
						<span class="text-[11px] font-bold uppercase tracking-[0.05em]" style={labelStyle('pending')}>{tr("Menunggu")}</span>
						<div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-[8px]" style={iconBoxStyle('pending')}>
							<Clock size={17} />
						</div>
					</div>
					<div>
						<div class="my-0.5 text-[24px] leading-[1.15] font-bold tracking-[-0.03em] tabular-nums" style={valueStyle('pending')}>{fmt(me?.pending_count ?? 0)}</div>
						<div class="text-[12px] leading-[1.35]" style={hintStyle('pending')}>{tr("Perlu tindakan segera")}</div>
					</div>
					<Sparkline
						values={pubSeries}
						stroke="var(--warning)"
						title={tr('Permintaan artikel masuk per hari, 14 hari terakhir')}
						class="mt-0.5"
					/>
				</div>
			{:else}
				<div class="app-card flex flex-col justify-between gap-2.5" style="padding: 16px 18px;">
					<div class="flex items-center justify-between gap-2">
						<span class="text-[11px] font-bold uppercase tracking-[0.05em] text-muted-foreground">{tr("Menunggu")}</span>
						<div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-[8px]" style={iconBoxStyle('neutral')}>
							<Clock size={17} />
						</div>
					</div>
					<div>
						<div class="my-0.5 text-[24px] leading-[1.15] font-bold tracking-[-0.03em] tabular-nums text-muted-foreground">0</div>
						<div class="text-[12px] leading-[1.35] text-muted-foreground">{tr("Semua publikasi telah diproses")}</div>
					</div>
					<Sparkline
						values={pubSeries}
						stroke="var(--warning)"
						title={tr('Permintaan artikel masuk per hari, 14 hari terakhir')}
						class="mt-0.5"
					/>
				</div>
			{/if}

			<!-- Total Penghasilan (spotlight) -->
				<div class="app-card flex flex-col justify-between gap-3.5" style="padding: 18px 20px; {cardStyle('earning')}">
					<div class="flex items-center justify-between gap-2">
						<span class="text-[11px] font-bold uppercase tracking-[0.05em]" style={labelStyle('earning')}>{tr("Total Penghasilan")}</span>
					<div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-[8px]" style={iconBoxStyle('earning')}>
						<Wallet size={17} />
					</div>
				</div>
				<div>
					<div class="my-0.5 text-[24px] leading-[1.15] font-bold tracking-[-0.03em] tabular-nums" style={valueStyle('earning')}>{rp(totalEarnings)}</div>
						<div class="text-[12px] leading-[1.35]" style={hintStyle('earning')}>{tr("Penghasilan dari artikel terbit")}</div>
					</div>
					<Sparkline
						values={earningSeries}
						stroke="var(--success)"
						title={tr('Penghasilan kumulatif (settled + pending), 30 hari terakhir')}
						class="mt-0.5"
					/>
				</div>
		</div>

		<!-- 3. Statistik publikasi -->
		{#if me?.publication_stats}
			<div class="grid grid-cols-2 gap-3.5 md:grid-cols-4">
				<div class="flex flex-col gap-1 rounded-lg border bg-card" style="padding: 14px 16px;">
					<span class="text-[20px] leading-[1.1] font-bold tabular-nums">{fmt((stats.waiting ?? 0) + (stats.draft_delivered ?? 0))}</span>
					<span class="text-[12px] text-muted-foreground">{tr("Draf / menunggu tindakan")}</span>
				</div>
				<div class="flex flex-col gap-1 rounded-lg border bg-card" style="padding: 14px 16px;">
					<span class="text-[20px] leading-[1.1] font-bold tabular-nums">{fmt(stats.auto_published ?? 0)}</span>
					<span class="text-[12px] text-muted-foreground">{tr("Terbit otomatis")}</span>
				</div>
				<div class="flex flex-col gap-1 rounded-lg border bg-card" style="padding: 14px 16px;">
					<span class="text-[20px] leading-[1.1] font-bold tabular-nums">{fmt(stats.failed ?? 0)}</span>
					<span class="text-[12px] text-muted-foreground">{tr("Gagal")}</span>
				</div>
				<div class="flex flex-col gap-1 rounded-lg border bg-card" style="padding: 14px 16px;">
					<span class="text-[20px] leading-[1.1] font-bold tabular-nums">{fmt(stats.waiting ?? 0)}</span>
					<span class="text-[12px] text-muted-foreground">{tr("Menunggu")}</span>
				</div>
			</div>
		{/if}

		<!-- 4. Publikasi Terbaru -->
		<div class="app-card overflow-hidden">
			<div class="flex flex-wrap items-center justify-between gap-3 border-b" style="padding: 16px 20px 14px;">
				<div class="flex flex-col gap-0.5">
					<h2 class="m-0 text-[15px] font-semibold tracking-[-0.015em]">{tr("Publikasi Terbaru")}</h2>
					<p class="m-0 text-[12.5px] text-muted-foreground">{tr("Riwayat distribusi artikel terbaru untuk media Anda")}</p>
				</div>
				<a class="inline-flex items-center gap-1 rounded-[6px] px-2 py-1 text-[12.5px] font-semibold transition-colors hover:bg-accent" style="color: var(--primary);" href={`${base}/publications`}>
					<span>{tr("Lihat semua")}</span>
					<ArrowRight size={13} />
				</a>
			</div>

			{#if pubError}
				<p class="px-5 py-8 text-center text-[13px]" style="color: var(--destructive);">{tr("Gagal memuat publikasi terbaru.")}</p>
			{:else if publications.length === 0}
				<div class="flex flex-col items-center gap-2 px-5 py-[44px] text-center text-muted-foreground">
					<Newspaper size={36} />
					<h3 class="m-0 text-[14px] font-semibold text-foreground">{tr("Belum ada publikasi")}</h3>
					<p class="m-0 text-[13px] text-muted-foreground">{tr("Publikasi dari klien Tauthub akan muncul di sini setelah tersedia.")}</p>
				</div>
			{:else}
				<div class="flex flex-col">
					{#each latestPublications as p (p.id)}
						{@const PubIcon = pubIcon(p.status)}
						<div class="flex flex-col items-start justify-between gap-2 border-b px-5 py-[13px] transition-colors last:border-b-0 hover:bg-accent sm:flex-row sm:items-center sm:gap-4">
							<div class="flex min-w-0 items-center gap-3">
								<div class="flex h-[34px] w-[34px] shrink-0 items-center justify-center rounded-[8px]" style={pubIconBoxStyle(p.status)}>
									<PubIcon size={18} />
								</div>
								<div class="flex min-w-0 flex-col gap-0.5">
									<div class="truncate text-[13.5px] leading-[1.35] font-semibold" title={p.document_title ?? tr('Artikel')}>
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
							<div class="flex w-full items-center justify-between gap-3.5 border-t border-dashed pt-1.5 sm:w-auto sm:justify-end sm:border-t-0 sm:pt-0">
								<span class="inline-flex items-center gap-1.5 rounded-[6px] px-2 py-[3px] text-[12px] font-semibold capitalize" style={badgeStyle(p.status)}>
									{badgeLabel(p.status)}
								</span>
								<span class="min-w-[90px] text-right text-[13.5px] font-bold tabular-nums">{rp(p.partner_amount ?? 0)}</span>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>

	{/if}
</section>
