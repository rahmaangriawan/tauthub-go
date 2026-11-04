<script lang="ts">
	// Profil media versi mitra (read-only). Tampilan mengikuti referensi
	// "Modern Media Profile": header identitas + aksen gelombang biru lembut,
	// grid metrik, lalu tabel ketentuan publikasi. Semua nilai tetap dari data
	// media yang sama; hanya presentasinya yang diubah.
	import { tr } from '$lib/i18n/phrases';
	import { mediaPlatform } from '$lib/media/platform';
	import { onMount } from 'svelte';
	import {
		Ban,
		CalendarDays,
		ChartBar,
		Circle,
		CircleCheck,
		Coins,
		ExternalLink,
		FileText,
		Gauge,
		HeartPulse,
		Layers,
		Link as LinkIcon,
		Link2,
		Newspaper,
		ShieldCheck,
		SquarePlay,
		Star,
		Tag,
		X,
		Zap
	} from '@lucide/svelte';
	import type { MediaDetailItem } from '$lib/media/detail-types';
	import { formatTraffic } from '$lib/media/traffic';
	import { avgPublishHours, formatSla } from '$lib/media/sla';
	import { mediaDistribution } from '$lib/media/distribution';
	import { healthToneStyle } from '$lib/media/health-tone';
	import { mediaDomainLabel } from '$lib/media/domain-label';
	import { wordCountLabel } from '$lib/media/word-count';
	import { MEDIA_LABEL } from '$lib/media/labels';
	import MediaFavicon from './_favicon.svelte';

	type SimpleMedia = MediaDetailItem & {
		total_terbit?: number | null;
		articles_7d?: number | null;
		crawl_last_crawled_at?: string | null;
		crawl_last_status?: string | null;
		crawl_stale?: boolean;
	};

	let { media, onclose }: { media: SimpleMedia; onclose: () => void } = $props();

	let dialogEl = $state<HTMLDivElement | null>(null);
	onMount(() => dialogEl?.focus());

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') onclose();
	}

	const domainOf = (m: SimpleMedia) =>
		(m.website_url ?? '')
			.replace(/^https?:\/\/(www\.)?/i, '')
			.replace(/\/+$/, '')
			.trim();

	const dateText = (iso: string | null) => {
		if (!iso) return '';
		const at = new Date(iso);
		if (Number.isNaN(at.getTime())) return '';
		return new Intl.DateTimeFormat('id-ID', {
			day: 'numeric',
			month: 'short',
			year: 'numeric',
			timeZone: 'Asia/Jakarta'
		}).format(at);
	};

	const domainText = $derived(mediaDomainLabel(media.website_url, media.name));
	const typeLabel = $derived(media.media_type ? (MEDIA_LABEL[media.media_type] ?? media.media_type) : '');
	const categoryText = $derived(
		Array.isArray(media.categories) && media.categories.length
			? media.categories[0]
			: (media.category ?? '')
	);

	const traffic = $derived(formatTraffic(media.sw_visits, media.traffic_estimate));
	const hours = $derived(avgPublishHours(media));
	const slaText = $derived(hours == null ? tr('Belum ada riwayat') : formatSla(hours));
	const dist = $derived(
		mediaDistribution(media.health_breakdown, media.published_count ?? media.total_terbit ?? null)
	);
	const score = $derived(media.health_score);
	const healthLabel = $derived(
		score == null
			? tr('Belum Diskor')
			: score >= 70
				? tr('Sangat Sehat')
				: score >= 40
					? tr('Perlu Perhatian')
					: tr('Kondisi Rendah')
	);
	const ratingText = $derived(
		media.rating_avg != null && media.rating_avg > 0
			? `${media.rating_avg.toFixed(1).replace('.', ',')}/5${media.rating_count ? ` (${media.rating_count})` : ''}`
			: tr('Belum ada penilaian')
	);
	const velocityText = $derived(
		media.articles_7d != null
			? `${media.articles_7d} ${tr('artikel')}`
			: media.crawl_last_status === 'blocked'
				? tr('Diblokir situs')
				: media.crawl_last_status === 'no_source'
					? tr('Tak ada sitemap/feed')
					: tr('Belum ada data')
	);
	const velocityNote = $derived(
		media.articles_7d != null && media.crawl_last_crawled_at
			? `${tr('Data per')} ${dateText(media.crawl_last_crawled_at)}${media.crawl_stale ? ` (${tr('belum diperbarui')})` : ''}`
			: ''
	);

	// Indikator pemenuhan memakai pembagi yang sama dengan mesin health
	// (fulfillment 30, retensi 20, uptime 15). Tidak ada angka baru.
	const healthB = $derived((media.health_breakdown ?? {}) as Record<string, unknown>);
	const hasHealthDetail = $derived(Boolean(media.health_breakdown));
	const fulfillmentPct = $derived(Math.round((Number(healthB.success_rate ?? 0) / 30) * 100));
	const permanencePct = $derived(Math.round((Number(healthB.rejection_rate ?? 0) / 20) * 100));
	const uptimePct = $derived(Math.round((Number(healthB.availability ?? 0) / 15) * 100));
	const statusItems = $derived([
		{ key: 'fulfillment', label: tr('Fulfillment'), pct: fulfillmentPct },
		{ key: 'permanence', label: tr('Retensi tautan'), pct: permanencePct },
		{ key: 'uptime', label: tr('Uptime'), pct: uptimePct }
	]);

	const months = $derived(media.link_duration_months);
	const durationText = $derived(
		months == null
			? tr('Belum diatur')
			: months === 0
				? tr('Permanen, tautan tidak dilepas')
				: `${months} ${tr('bulan')}`
	);
	const linksText = $derived(
		media.max_links == null
			? tr('Maks. 2 tautan')
			: media.max_links === 0
				? tr('Tanpa tautan keluar')
				: `${media.max_links} ${tr('tautan')} ${media.link_rel === 'sponsored' ? 'Sponsored' : media.link_rel === 'nofollow' || media.allow_hyperlink === false ? 'Nofollow' : 'Dofollow'}`
	);
	const embedText = $derived(media.allow_embed ? tr('Diizinkan') : tr('Tidak diizinkan'));
	// Sama persis dengan yang dilihat klien di detail media (media-detail-modal).
	const headingText = $derived(
		media.allow_heading === true
			? tr('Boleh memakai subjudul')
			: media.allow_heading === false
				? tr('Tanpa subjudul')
				: media.media_type === 'blogger'
					? tr('Boleh (bawaan media blog)')
					: tr('Tanpa subjudul (bawaan media berita)')
	);
	const platformText = $derived(mediaPlatform(media));
	const sampleUrl = $derived((media.contoh_artikel_url ?? '').trim());
	const sponsorText = $derived(
		!media.label_sponsor || media.label_sponsor === 'tidak_ada'
			? tr('Organik, tanpa label')
			: tr('Dengan label sponsor')
	);
	const topicsText = $derived(
		media.topik_terlarang && media.topik_terlarang.length
			? media.topik_terlarang.join(', ')
			: tr('Tidak ada')
	);
</script>

<svelte:window onkeydown={onKeydown} />

<div
	class="overlay"
	role="presentation"
	onclick={(e) => {
		if (e.target === e.currentTarget) onclose();
	}}
	onkeydown={(e) => {
		if (e.key === 'Escape') onclose();
	}}
>
	<div
		class="sheet"
		role="dialog"
		aria-modal="true"
		aria-labelledby="mitra-media-title"
		tabindex="-1"
		bind:this={dialogEl}
	>
		<!-- ── Header identitas ─────────────────────────────────────── -->
		<header class="head">
			<div class="wave" aria-hidden="true">
				<svg viewBox="0 0 520 160" preserveAspectRatio="none">
					<path d="M0 108 C 110 40, 200 156, 320 98 S 460 44, 520 76 L 520 160 L 0 160 Z" fill="#eaf4fc" />
					<path d="M0 132 C 130 76, 220 168, 350 116 S 470 92, 520 108 L 520 160 L 0 160 Z" fill="#d8ecfa" />
				</svg>
			</div>

			<div class="identity">
				<div class="logoBox">
					<MediaFavicon domain={domainOf(media)} size={44} />
				</div>
				<div class="headText">
					<h2 id="mitra-media-title">{media.name}</h2>
					<p class="sub">
						{#if media.website_url}
							<a class="site" href={media.website_url} target="_blank" rel="noreferrer">
								{domainText}<ExternalLink size={13} />
							</a>
						{:else}
							<span class="siteText">{domainText}</span>
						{/if}
						{#if typeLabel}
							<span class="sep" aria-hidden="true">·</span>
							<span class="typeBadge">{typeLabel}</span>
						{/if}
						{#if categoryText}
							<span class="sep" aria-hidden="true">·</span>
							<span class="catText">{categoryText}</span>
						{/if}
					</p>
				</div>
			</div>

			<button class="close" type="button" aria-label={tr('Tutup')} onclick={onclose}>
				<X size={20} />
			</button>
		</header>

		<!-- ── Isi ──────────────────────────────────────────────────── -->
		<div class="body">
			<p class="lead">{tr('Ini tampilan media Anda di katalog klien Tauthub.')}</p>

			<section>
				<h3 class="sectionTitle"><ChartBar size={20} />{tr('Ringkasan')}</h3>
				<dl class="metrics">
					<div class="metric">
						<span class="metricIcon"><Coins size={19} /></span>
						<dt>{tr('Trafik per bulan')}</dt>
						<dd>{traffic.text}</dd>
					</div>
					<div class="metric">
						<span class="metricIcon"><Gauge size={19} /></span>
						<dt>Domain Authority</dt>
						<dd>{media.seo_da ?? tr('Belum diukur')}</dd>
					</div>
					<div class="metric">
						<span class="metricIcon"><ShieldCheck size={19} /></span>
						<dt>AI Trust</dt>
						<dd>{media.seo_ai_trust ?? tr('Belum diukur')}</dd>
					</div>
					<div class="metric">
						<span class="metricIcon"><Star size={19} /></span>
						<dt>{tr('Rating klien')}</dt>
						<dd>{ratingText}</dd>
					</div>
					<div class="metric">
						<span class="metricIcon"><Zap size={19} /></span>
						<dt>{tr('SLA terbit')}</dt>
						<dd>{slaText}</dd>
					</div>
					<div class="metric">
						<span class="metricIcon"><Newspaper size={19} /></span>
						<dt>{tr('Artikel terbit')}</dt>
						<dd>{dist.published} {tr('artikel')}</dd>
					</div>
					<div class="metric metricHealth" style={healthToneStyle(score)}>
						<span class="metricIcon"><HeartPulse size={19} /></span>
						<dt>{tr('Kesehatan')}</dt>
						<dd>
							{#if score != null}
								<span class="healthScore"
									>{score}<span class="healthMax">/100</span><span class="srOnly">, {healthLabel}</span></span
								>
							{:else}
								{tr('Belum Diskor')}
							{/if}
						</dd>
					</div>
					<div class="metric">
						<span class="metricIcon"><CalendarDays size={19} /></span>
						<dt>{tr('Artikel per minggu')}</dt>
						<dd>
							{velocityText}
							{#if velocityNote}<span class="note">{velocityNote}</span>{/if}
						</dd>
					</div>
				</dl>

				{#if hasHealthDetail}
					<ul class="statusRow">
						{#each statusItems as item (item.key)}
							<li class="statusItem" class:statusOk={item.pct >= 100}>
								{#if item.pct >= 100}
									<CircleCheck size={16} />
								{:else}
									<Circle size={16} />
								{/if}
								<span>{item.label} {item.pct}%</span>
							</li>
						{/each}
					</ul>
				{/if}
			</section>

			<section>
				<h3 class="sectionTitle"><Layers size={20} />{tr('Ketentuan publikasi')}</h3>
				<dl class="terms">
					<div class="term">
						<dt><LinkIcon size={16} />{tr('Garansi tayang')}</dt>
						<dd>{durationText}</dd>
					</div>
					<div class="term">
						<dt><Link2 size={16} />{tr('Batas tautan keluar')}</dt>
						<dd>{linksText}</dd>
					</div>
					<div class="term">
						<dt><FileText size={16} />{tr('Panjang artikel')}</dt>
						<dd>{wordCountLabel(media.word_count_mode)}</dd>
					</div>
					<div class="term">
						<dt><FileText size={16} />{tr('Subjudul (H2/H3)')}</dt>
						<dd>{headingText}</dd>
					</div>
					<div class="term">
						<dt><SquarePlay size={16} />{tr('Embed video/maps')}</dt>
						<dd>{embedText}</dd>
					</div>
					<div class="term">
						<dt><Layers size={16} />{tr('Platform')}</dt>
						<dd>{platformText}</dd>
					</div>
					{#if sampleUrl}
						<div class="term">
							<dt><Link2 size={16} />{tr('Contoh artikel')}</dt>
							<dd><a href={sampleUrl} target="_blank" rel="noreferrer noopener">{tr('Lihat contoh')} ↗</a></dd>
						</div>
					{/if}
					<div class="term">
						<dt><Tag size={16} />{tr('Label sponsor')}</dt>
						<dd>{sponsorText}</dd>
					</div>
					<div class="term">
						<dt><Ban size={16} />{tr('Topik dilarang')}</dt>
						<dd>{topicsText}</dd>
					</div>
				</dl>
			</section>
		</div>

		<!-- ── Footer ───────────────────────────────────────────────── -->
		<footer class="foot">
			<button class="closeBtn" type="button" onclick={onclose}>{tr('Tutup')}</button>
		</footer>
	</div>
</div>

<style>
	.overlay {
		position: fixed;
		inset: 0;
		z-index: 70;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 16px;
		background: rgba(15, 23, 42, 0.55);
	}
	.sheet {
		/* Palet Tauthub khusus profil media ini. */
		--mt-navy: #0f1f3d;
		--mt-muted: #5c6b84;
		--mt-blue-deep: #2c5ead;
		--mt-blue: #1591dc;
		--mt-blue-light: #4bb8fa;
		--mt-blue-pale: #c4e2f5;
		--mt-panel: #f1f6fd;
		--mt-panel-border: #e1eaf5;
		--mt-divider: #e6edf7;
		--mt-green: #1e9e6a;

		position: relative;
		display: flex;
		flex-direction: column;
		width: 100%;
		max-width: 860px;
		max-height: 90vh;
		overflow: hidden;
		border-radius: 16px;
		background: #fff;
		color: var(--mt-navy);
		box-shadow: 0 24px 60px -16px rgba(15, 31, 61, 0.35);
	}
	.sheet:focus {
		outline: none;
	}

	/* ── Header ─────────────────────────────────────────────────── */
	.head {
		position: relative;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		flex-shrink: 0;
		padding: 22px 24px;
		overflow: hidden;
		border-bottom: 1px solid var(--mt-divider);
		background: #fff;
	}
	.wave {
		position: absolute;
		inset: 0;
		width: 100%;
		height: 100%;
		pointer-events: none;
	}
	.wave svg {
		display: block;
		width: 100%;
		height: 100%;
	}
	.identity {
		position: relative;
		z-index: 1;
		display: flex;
		align-items: center;
		gap: 16px;
		min-width: 0;
	}
	.logoBox {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 64px;
		height: 64px;
		flex-shrink: 0;
		border: 1px solid var(--mt-panel-border);
		border-radius: 14px;
		background: #fff;
		box-shadow: 0 2px 6px rgba(15, 31, 61, 0.08);
	}
	.headText {
		min-width: 0;
	}
	.head h2 {
		margin: 0;
		font-size: 23px;
		font-weight: 700;
		line-height: 1.2;
		letter-spacing: -0.01em;
		color: var(--mt-navy);
		word-break: break-word;
	}
	.sub {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 8px;
		margin: 6px 0 0;
		font-size: 13px;
		color: var(--mt-muted);
	}
	.site {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		color: var(--mt-blue-deep);
		font-weight: 600;
		text-decoration: none;
	}
	.site:hover {
		text-decoration: underline;
	}
	.site:focus-visible {
		outline: 2px solid var(--mt-blue);
		outline-offset: 2px;
		border-radius: 4px;
	}
	.siteText {
		font-weight: 600;
		color: var(--mt-blue-deep);
	}
	.sep {
		color: #b7c4d8;
	}
	.typeBadge {
		display: inline-flex;
		align-items: center;
		height: 20px;
		padding: 0 8px;
		border-radius: 6px;
		background: #eaf2fc;
		color: var(--mt-blue-deep);
		font-size: 11.5px;
		font-weight: 600;
	}
	.close {
		position: relative;
		z-index: 1;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 44px;
		height: 44px;
		flex-shrink: 0;
		border: 1px solid var(--mt-panel-border);
		border-radius: 12px;
		background: #fff;
		color: #4a5b76;
		cursor: pointer;
		box-shadow: 0 1px 3px rgba(15, 31, 61, 0.06);
	}
	.close:hover {
		background: #f4f8fd;
	}
	.close:focus-visible {
		outline: 2px solid var(--mt-blue);
		outline-offset: 2px;
	}

	/* ── Isi ────────────────────────────────────────────────────── */
	.body {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: 24px;
		padding: 24px;
		overflow-y: auto;
		overscroll-behavior: contain;
	}
	.lead {
		margin: 0;
		font-size: 13.5px;
		color: var(--mt-muted);
	}
	.sectionTitle {
		display: flex;
		align-items: center;
		gap: 9px;
		margin: 0 0 14px;
		font-size: 18px;
		font-weight: 700;
		color: var(--mt-navy);
	}
	.sectionTitle :global(svg) {
		flex-shrink: 0;
		color: var(--mt-blue-deep);
	}

	.metrics {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 10px;
		margin: 0;
	}
	.metric {
		display: grid;
		grid-template-columns: auto 1fr;
		grid-template-rows: auto auto;
		column-gap: 9px;
		row-gap: 7px;
		min-width: 0;
		padding: 14px;
		border: 1px solid var(--mt-panel-border);
		border-radius: 12px;
		background: var(--mt-panel);
	}
	.metricIcon {
		display: inline-flex;
		margin-top: 1px;
		color: var(--mt-blue-deep);
	}
	.metric dt {
		margin: 0;
		font-size: 12.5px;
		font-weight: 500;
		line-height: 1.3;
		color: var(--mt-muted);
	}
	.metric dd {
		grid-column: 1 / -1;
		margin: 0;
		font-size: 17px;
		font-weight: 700;
		line-height: 1.3;
		color: var(--mt-navy);
		word-break: break-word;
	}
	.metricHealth .metricIcon,
	.metricHealth dd {
		color: inherit;
	}
	.metricHealth dt {
		color: inherit;
		opacity: 0.85;
	}
	.healthMax {
		font-size: 12px;
		font-weight: 600;
		opacity: 0.8;
	}
	.healthScore {
		font-variant-numeric: tabular-nums;
	}
	.srOnly {
		position: absolute;
		width: 1px;
		height: 1px;
		padding: 0;
		margin: -1px;
		overflow: hidden;
		clip: rect(0 0 0 0);
		white-space: nowrap;
		border: 0;
	}
	.note {
		display: block;
		margin-top: 2px;
		font-size: 11px;
		font-weight: 500;
		color: var(--mt-muted);
	}

	.statusRow {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 10px 18px;
		margin: 16px 0 0;
		padding: 0;
		list-style: none;
	}
	.statusItem {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		font-size: 12.5px;
		color: var(--mt-muted);
	}
	.statusItem + .statusItem {
		padding-left: 18px;
		border-left: 1px solid var(--mt-divider);
	}
	.statusOk {
		color: var(--mt-navy);
		font-weight: 600;
	}
	.statusItem :global(svg) {
		flex-shrink: 0;
		color: #a9b6c9;
	}
	.statusOk :global(svg) {
		color: var(--mt-green);
	}

	/* ── Ketentuan publikasi ───────────────────────────────────── */
	.terms {
		margin: 0;
		overflow: hidden;
		border: 1px solid var(--mt-panel-border);
		border-radius: 12px;
	}
	.term {
		display: grid;
		grid-template-columns: 230px 1fr;
	}
	.term + .term {
		border-top: 1px solid var(--mt-divider);
	}
	.term dt {
		display: flex;
		align-items: center;
		gap: 9px;
		margin: 0;
		padding: 12px 14px;
		background: #f4f8fd;
		color: var(--mt-muted);
		font-size: 13px;
		font-weight: 600;
	}
	.term dd {
		margin: 0;
		padding: 12px 14px;
		background: #fff;
		color: var(--mt-navy);
		font-size: 13.5px;
		font-weight: 600;
		word-break: break-word;
	}
	.term dt :global(svg) {
		flex-shrink: 0;
		color: var(--mt-blue-deep);
	}

	/* ── Footer ─────────────────────────────────────────────────── */
	.foot {
		display: flex;
		justify-content: flex-end;
		flex-shrink: 0;
		padding: 14px 24px;
		border-top: 1px solid var(--mt-divider);
		background: #fff;
	}
	.closeBtn {
		height: 42px;
		padding: 0 24px;
		border: 1px solid var(--mt-blue-deep);
		border-radius: 10px;
		background: var(--mt-blue-deep);
		color: #fff;
		font: inherit;
		font-size: 14px;
		font-weight: 600;
		cursor: pointer;
		transition: background-color 0.15s ease;
	}
	.closeBtn:hover {
		background: #244d91;
	}
	.closeBtn:focus-visible {
		outline: 2px solid var(--mt-blue);
		outline-offset: 2px;
	}

	@media (max-width: 1023px) {
		.metrics {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
	}
	@media (max-width: 767px) {
		.overlay {
			padding: 10px;
		}
		.sheet {
			max-height: 92vh;
			border-radius: 14px;
		}
		.head {
			gap: 12px;
			padding: 16px 18px;
		}
		.logoBox {
			width: 54px;
			height: 54px;
			border-radius: 12px;
		}
		.head h2 {
			font-size: 19px;
		}
		.body {
			gap: 20px;
			padding: 18px;
		}
		.metric {
			padding: 11px;
		}
		.metric dt {
			font-size: 11.5px;
		}
		.metric dd {
			font-size: 15px;
		}
		.statusItem + .statusItem {
			padding-left: 0;
			border-left: 0;
		}
		.term {
			grid-template-columns: 1fr;
		}
		.term dt {
			padding: 10px 14px 4px;
		}
		.term dd {
			padding: 2px 14px 12px;
		}
		.foot {
			padding: 12px 18px;
		}
		.closeBtn {
			width: 100%;
		}
	}
</style>
