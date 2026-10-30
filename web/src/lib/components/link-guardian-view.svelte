<script lang="ts">
	import { tr } from '$lib/i18n/phrases';
	import type { Snippet } from 'svelte';
	import { ArrowUp, ExternalLink, Image as ImageIcon, Search } from '@lucide/svelte';
	import Tooltip from '$lib/components/tooltip.svelte';

	// Tampilan bersama Link Guardian (portal klien dan portal mitra).
	//
	// Sengaja presentasional: tidak tahu API mana pun. Masing-masing halaman
	// menyiapkan datanya sendiri (baris sudah dipetakan ke bentuk di bawah),
	// memilih kolom yang ingin ditampilkan, dan menitipkan kontrol toolbar serta
	// aksi per baris lewat snippet. Dengan begitu tampilannya satu sumber, tapi
	// isi dan tindakannya tetap sesuai peran halaman.

	let {
		eyebrow = 'LINK GUARDIAN',
		title = tr('Link Guardian'),
		subtitle = '',
		cards = [],
		rows = [],
		showAnchor = false,
		showVerified = true,
		headerRight,
		toolbar,
		rowAction,
		emptyTitle = tr('Tidak ada tautan bermasalah. Semua artikel sehat.'),
		emptyHint = ''
	}: {
		eyebrow?: string;
		title?: string;
		subtitle?: string;
		cards?: Array<{ label: string; value: number; hint?: string; icon?: any; tone?: string }>;
		rows?: Array<{
			id: string; url?: string | null; title?: string | null; thumb?: string | null;
			publisher?: string | null; publisherSub?: string | null; verified?: boolean | null;
			detection: { label: string; text: string; bg: string; dot: string; hint?: string };
			checkedAt?: string | null; anchor?: string | null;
		}>;
		showAnchor?: boolean;
		showVerified?: boolean;
		headerRight?: Snippet;
		toolbar?: Snippet;
		rowAction?: Snippet<[{ id: string }]>;
		emptyTitle?: string;
		emptyHint?: string;
	} = $props();

	// Kolom dibangun dari konfigurasi supaya halaman yang tidak punya datanya
	// (mis. mitra tidak punya anchor) tidak menampilkan kolom kosong.
	const cols = $derived.by(() => {
		const list: Array<{ key: string; label: string; align: 'left' | 'right'; weight: number }> = [
			{ key: 'article', label: tr('Informasi Artikel & Tautan'), align: 'left', weight: 30 },
			{ key: 'publisher', label: tr('Publisher'), align: 'left', weight: 15 }
		];
		if (showVerified) list.push({ key: 'verified', label: tr('Status Verifikasi'), align: 'left', weight: 13 });
		list.push({ key: 'detection', label: tr('Status Deteksi'), align: 'left', weight: 14 });
		list.push({ key: 'checked', label: tr('Waktu Cek'), align: 'left', weight: 13 });
		if (rowAction) list.push({ key: 'action', label: tr('Aksi'), align: 'right', weight: 15 });
		const total = list.reduce((sum, c) => sum + c.weight, 0);
		return list.map((c) => ({ ...c, width: `${((c.weight / total) * 100).toFixed(1)}%` }));
	});

	const fmtDay = (v: string | null | undefined) =>
		v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—';

	let imgBroken = $state<Record<string, boolean>>({});
	const PALETTE = [
		{
			stroke: '#2C5EAD', gradTop: '#2C5EAD', gradBottom: '#C4E2F5',
			cardBg: 'linear-gradient(180deg, rgba(44, 94, 173, 0.045) 0%, rgba(196, 226, 245, 0.14) 100%)',
			cardBorder: 'rgba(44, 94, 173, 0.16)', iconBg: 'rgba(44, 94, 173, 0.12)', iconColor: '#2C5EAD',
			pillBg: 'rgba(44, 94, 173, 0.12)', pillColor: '#2C5EAD',
			waveLine: 'M 4 34 Q 20 26, 36 27 T 70 31 T 104 25 T 138 29 T 172 22 T 206 14 T 236 18',
			waveArea: 'M 4 34 Q 20 26, 36 27 T 70 31 T 104 25 T 138 29 T 172 22 T 206 14 T 236 18 L 236 44 L 4 44 Z'
		},
		{
			stroke: '#1591DC', gradTop: '#1591DC', gradBottom: '#C4E2F5',
			cardBg: 'linear-gradient(180deg, rgba(21, 145, 220, 0.05) 0%, rgba(196, 226, 245, 0.18) 100%)',
			cardBorder: 'rgba(21, 145, 220, 0.18)', iconBg: 'rgba(21, 145, 220, 0.13)', iconColor: '#1591DC',
			pillBg: 'rgba(21, 145, 220, 0.14)', pillColor: '#1591DC',
			waveLine: 'M 4 35 Q 22 25, 38 29 T 74 26 T 110 31 T 146 21 T 182 23 T 212 16 T 236 13',
			waveArea: 'M 4 35 Q 22 25, 38 29 T 74 26 T 110 31 T 146 21 T 182 23 T 212 16 T 236 13 L 236 44 L 4 44 Z'
		},
		{
			stroke: '#4BB8FA', gradTop: '#4BB8FA', gradBottom: '#C4E2F5',
			cardBg: 'linear-gradient(180deg, rgba(75, 184, 250, 0.065) 0%, rgba(196, 226, 245, 0.22) 100%)',
			cardBorder: 'rgba(75, 184, 250, 0.26)', iconBg: 'rgba(75, 184, 250, 0.18)', iconColor: '#1591DC',
			pillBg: 'rgba(75, 184, 250, 0.22)', pillColor: '#2C5EAD',
			waveLine: 'M 4 36 Q 22 27, 40 28 T 76 32 T 112 24 T 148 27 T 184 17 T 214 19 T 236 12',
			waveArea: 'M 4 36 Q 22 27, 40 28 T 76 32 T 112 24 T 148 27 T 184 17 T 214 19 T 236 12 L 236 44 L 4 44 Z'
		},
		{
			stroke: '#2C5EAD', gradTop: '#4BB8FA', gradBottom: '#C4E2F5',
			cardBg: 'linear-gradient(180deg, rgba(196, 226, 245, 0.28) 0%, rgba(196, 226, 245, 0.10) 100%)',
			cardBorder: '#C4E2F5', iconBg: '#C4E2F5', iconColor: '#2C5EAD',
			pillBg: '#C4E2F5', pillColor: '#2C5EAD',
			waveLine: 'M 4 34 Q 20 26, 38 28 T 74 25 T 110 30 T 146 22 T 182 25 T 212 15 T 236 19',
			waveArea: 'M 4 34 Q 20 26, 38 28 T 74 25 T 110 30 T 146 22 T 182 25 T 212 15 T 236 19 L 236 44 L 4 44 Z'
		}
	];
</script>

<section class="lg-root">
	<div class="app-card lg-overview-panel">
		<header class="lg-header">
			<div class="min-w-0">
				<h1 class="text-2xl font-semibold tracking-tight">{title}</h1>
				{#if subtitle}
					<p class="lg-sub mt-1 text-sm text-muted-foreground">{subtitle}</p>
				{/if}
			</div>
			{#if headerRight}
				<div class="lg-sync">{@render headerRight()}</div>
			{/if}
		</header>

		{#if cards.length > 0}
			{@const baseTotal = cards[0]?.value ?? 0}
			<div class="lg-cards">
				{#each cards as c, idx (c.label)}
					{@const p = PALETTE[idx % PALETTE.length]}
					{@const pct = idx === 0 ? (baseTotal > 0 ? '100%' : '0%') : baseTotal > 0 ? `${Math.round((c.value / baseTotal) * 100)}%` : '0%'}
					<div class="lg-card" style="--card-bg: {p.cardBg}; --card-border: {p.cardBorder};">
						<div class="lg-card-top">
							{#if c.icon}
								<div class="lg-card-icon" style="background-color: {p.iconBg}; color: {p.iconColor};">
									<c.icon size={20} strokeWidth={2.1} />
								</div>
							{/if}
							<div class="lg-card-body">
								<p class="lg-card-label"><span>{c.label}</span></p>
								<div class="lg-card-metric">
									<p class="lg-card-value">{c.value}</p>
									<span class="lg-card-pill" style="background-color: {p.pillBg}; color: {p.pillColor};">
										<ArrowUp size={10} strokeWidth={2.6} />
										{pct}
									</span>
								</div>
								{#if c.hint}<p class="lg-card-hint">{c.hint}</p>{/if}
							</div>
						</div>
						<div class="lg-card-chart" aria-hidden="true">
							<svg viewBox="0 0 240 46" preserveAspectRatio="none" class="lg-spark-svg">
								<defs>
									<linearGradient id={`lgv-grad-${idx}`} x1="0" y1="0" x2="0" y2="1">
										<stop offset="0%" stop-color={p.gradTop} stop-opacity="0.28" />
										<stop offset="100%" stop-color={p.gradBottom} stop-opacity="0.03" />
									</linearGradient>
								</defs>
								<path d={p.waveArea} fill={`url(#lgv-grad-${idx})`} />
								<path d={p.waveLine} fill="none" stroke={p.stroke} stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" />
							</svg>
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>

	{#if toolbar}
		<div class="app-card lg-toolbar">{@render toolbar()}</div>
	{/if}

	<div class="app-card mt-3">
		{#if rows.length === 0}
			<div class="px-6 py-12 text-center">
				<p class="text-sm font-medium">{emptyTitle}</p>
				{#if emptyHint}<p class="mx-auto mt-1 max-w-lg text-sm text-muted-foreground">{emptyHint}</p>{/if}
			</div>
		{:else}
			<table class="lg-table w-full text-sm">
				<colgroup>
					{#each cols as col (col.key)}<col style="width:{col.width}" />{/each}
				</colgroup>
				<thead>
					<tr>
						{#each cols as col (col.key)}
							<th class="lg-th" class:lg-th-right={col.align === 'right'}>{col.label}</th>
						{/each}
					</tr>
				</thead>
				<tbody>
					{#each rows as row (row.id)}
						<tr class="lg-row">
							<td class="lg-cell lg-cell-article">
								<div class="lg-article">
									{#if row.thumb && !imgBroken[row.id]}
										<img
											src={row.thumb}
											alt=""
											class="lg-thumb"
											loading="lazy"
											onerror={() => (imgBroken = { ...imgBroken, [row.id]: true })}
										/>
									{:else}
										<span class="lg-thumb lg-thumb-empty" title={row.title ?? undefined}><ImageIcon size={15} /></span>
									{/if}
									<div class="min-w-0">
										{#if row.url}
											<a class="lg-url" href={row.url} target="_blank" rel="noreferrer" title={row.url}>
												<span class="truncate">{row.url}</span><ExternalLink size={11} />
											</a>
										{/if}
										{#if row.title}<p class="lg-doctitle">{row.title}</p>{/if}
										{#if showAnchor && row.anchor}
											<p class="lg-anchor">{tr('Anchor:')} <strong>{row.anchor}</strong></p>
										{/if}
									</div>
								</div>
							</td>
							<td class="lg-cell" data-label={tr('Publisher')}>
								<p class="lg-media">{row.publisher ?? '—'}</p>
								{#if row.publisherSub}<p class="lg-media-sub">{row.publisherSub}</p>{/if}
							</td>
							{#if showVerified}
								<td class="lg-cell" data-label={tr('Status Verifikasi')}>
									{#if row.verified === true}
										<span class="lg-verified">{tr('VERIFIED')}</span>
									{:else if row.verified === false}
										<span class="lg-unverified">{tr('UNVERIFIED')}</span>
									{:else}
										<span class="lg-noaction">—</span>
									{/if}
								</td>
							{/if}
							<td class="lg-cell" data-label={tr('Status Deteksi')}>
								{#snippet detBadge()}
									<span class="lg-badge" style="color: {row.detection.text}; background-color: {row.detection.bg};">
										<span class="lg-badge-dot" style="background-color: {row.detection.dot};"></span>
										<span>{row.detection.label}</span>
									</span>
								{/snippet}
								{#if row.detection.hint}
									<Tooltip content={row.detection.hint} label={row.detection.hint} maxWidth={300}>
										{@render detBadge()}
									</Tooltip>
								{:else}
									{@render detBadge()}
								{/if}
							</td>
							<td class="lg-cell" data-label={tr('Waktu Cek')}>
								<p class="lg-time">{fmtDay(row.checkedAt)}</p>
							</td>
							{#if rowAction}
								<td class="lg-cell lg-cell-action" data-label={tr('Aksi')}>
									{@render rowAction({ id: row.id })}
								</td>
							{/if}
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</div>
</section>

<style>
	/* ── Header ─────────────────────────────────────────────────────────── */
	.lg-overview-panel {
		padding: 22px 24px;
		border-radius: 20px;
		border: 1px solid color-mix(in srgb, #C4E2F5 70%, var(--border));
		background: var(--card, #ffffff);
		box-shadow: 0 6px 28px -10px rgba(44, 94, 173, 0.08);
	}
	.lg-header { display: flex; flex-wrap: wrap; align-items: flex-start; justify-content: space-between; gap: 16px; }
	.lg-sub { max-width: 66ch; }
	.lg-sync { display: flex; flex-direction: column; align-items: flex-end; gap: 8px; }

	/* ── Kartu ringkasan ────────────────────────────────────────────────── */
	.lg-cards { margin-top: 20px; display: grid; gap: 14px; grid-template-columns: repeat(auto-fit, minmax(210px, 1fr)); }
	.lg-card {
		display: flex; flex-direction: column; justify-content: space-between;
		padding: 16px 18px 12px; border-radius: 16px;
		border: 1px solid var(--card-border, #C4E2F5);
		background: var(--card-bg, rgba(196, 226, 245, 0.14));
	}
	.lg-card-top { display: flex; align-items: flex-start; gap: 14px; min-width: 0; }
	.lg-card-icon {
		width: 44px; height: 44px; flex: 0 0 44px; border-radius: 12px;
		display: flex; align-items: center; justify-content: center;
	}
	.lg-card-body { min-width: 0; flex: 1 1 auto; }
	.lg-card-label {
		display: inline-flex; align-items: center; gap: 5px;
		font-size: 13px; font-weight: 600; color: var(--foreground);
	}
	.lg-card-metric { margin-top: 6px; display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
	.lg-card-value { font-size: 28px; font-weight: 800; line-height: 1.1; letter-spacing: -0.025em; font-variant-numeric: tabular-nums; }
	.lg-card-pill {
		display: inline-flex; align-items: center; gap: 3px;
		padding: 2.5px 8px; border-radius: 999px; font-size: 11.5px; font-weight: 700;
	}
	.lg-card-hint { margin-top: 6px; font-size: 12px; line-height: 1.35; color: var(--muted-foreground); }
	.lg-card-chart { margin-top: 12px; width: 100%; height: 42px; }
	.lg-spark-svg { width: 100%; height: 100%; display: block; overflow: visible; }

	/* ── Toolbar (kontrol disediakan halaman pemakainya) ────────────────── */
	.lg-toolbar { margin-top: 12px; display: flex; flex-wrap: wrap; align-items: center; gap: 6px; padding: 8px 12px; }
	.lg-toolbar :global(.zs-select-trigger) { border: 0; background: transparent; padding-left: 6px; padding-right: 6px; }
	.lg-toolbar :global(input[type='text']),
	.lg-toolbar :global(input:not([type])) { min-width: 0; }

	/* ── Tabel ──────────────────────────────────────────────────────────── */
	.lg-table { table-layout: fixed; }
	.lg-th {
		padding: 9px 16px; font-size: 10.5px; font-weight: 700; letter-spacing: 0.06em;
		text-transform: uppercase; color: var(--muted-foreground); text-align: left;
		border-bottom: 1px solid var(--border);
	}
	.lg-th-right { text-align: right; }
	.lg-row { border-bottom: 1px solid var(--border); transition: background-color 120ms ease; }
	.lg-row:last-child { border-bottom: 0; }
	.lg-row:hover { background-color: color-mix(in srgb, var(--secondary) 55%, transparent); }
	.lg-cell { padding: 11px 16px; vertical-align: top; }
	.lg-cell-action { text-align: right; }

	.lg-article { display: flex; align-items: center; gap: 10px; min-width: 0; }
	.lg-thumb { width: 56px; height: 38px; flex: none; border-radius: 6px; object-fit: cover; }
	.lg-thumb-empty { display: flex; align-items: center; justify-content: center; background-color: var(--muted); color: var(--muted-foreground); }
	.lg-url { display: inline-flex; align-items: center; gap: 4px; max-width: 100%; font-size: 12.5px; font-weight: 500; color: var(--primary); }
	.lg-url span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; }
	.lg-url:hover span { text-decoration: underline; }
	.lg-doctitle {
		margin-top: 2px; font-size: 11.5px; line-height: 1.35; color: var(--muted-foreground);
		display: -webkit-box; -webkit-line-clamp: 2; line-clamp: 2; -webkit-box-orient: vertical;
		overflow: hidden; word-break: break-word;
	}
	.lg-anchor { margin-top: 2px; font-size: 11.5px; color: var(--muted-foreground); }
	.lg-anchor strong { color: var(--foreground); font-weight: 600; }

	.lg-media { font-size: 13.5px; font-weight: 500; }
	.lg-media-sub { margin-top: 1px; font-size: 11.5px; color: var(--muted-foreground); }

	.lg-badge { display: inline-flex; align-items: center; gap: 5px; max-width: 100%; padding: 2px 8px; border-radius: 999px; font-size: 11px; font-weight: 600; white-space: nowrap; }
	.lg-badge-dot { width: 6px; height: 6px; flex: none; border-radius: 999px; }
	.lg-verified, .lg-unverified {
		display: inline-flex; align-items: center; gap: 4px; padding: 2px 8px; border-radius: 6px;
		font-size: 10.5px; font-weight: 700; letter-spacing: 0.04em; border: 1px solid transparent;
	}
	.lg-verified { color: var(--primary); border-color: color-mix(in srgb, var(--primary) 35%, transparent); }
	.lg-unverified { color: var(--muted-foreground); border-color: var(--border); }
	.lg-time { font-size: 12.5px; }
	.lg-noaction { font-size: 12.5px; color: var(--muted-foreground); }

	/* Di layar sempit kolom tidak bisa dimuat: baris jadi kartu bertumpuk dan
	   setiap sel menampilkan labelnya sendiri lewat data-label. */
	@media (max-width: 760px) {
		.lg-table, .lg-table tbody, .lg-row, .lg-cell { display: block; width: 100%; }
		.lg-table colgroup, .lg-table thead { display: none; }
		.lg-row { padding: 12px 14px; }
		.lg-row:hover { background-color: transparent; }
		.lg-cell { padding: 0; text-align: left !important; }
		.lg-cell + .lg-cell { margin-top: 9px; }
		.lg-cell-article { margin-bottom: 3px; }
		.lg-cell[data-label]::before {
			content: attr(data-label); display: block; margin-bottom: 3px;
			color: var(--muted-foreground); font-size: 10px; font-weight: 700;
			letter-spacing: 0.06em; text-transform: uppercase;
		}
	}
</style>
