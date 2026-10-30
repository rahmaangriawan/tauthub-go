<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Modal detail media — port 1:1 dari media-detail-modal.tsx & media-intelligence.module.css.
	// Layout: header (avatar inisial, domain, verified, dewan pers, tombol aksi)
	// → strip metrik (estimasi trafik, DA, AI Trust Score)
	// → 2 kolom:
	//    Kiri : Hero Card (Tauthub Secure Direct / Editorial Review + ilustrasi 3D)
	//           + Indikator Kinerja Mitra (4 grid box + Content Velocity Crawlhub)
	//    Kanan: Parameter AI & Redaksi + Pembatasan Gatekeeper (chip tag)
	// → footer: Biaya Publikasi + tombol CTA "Lanjut ke Dapur Produksi".
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		Calendar, ChartBar, Check, CircleCheck, Clock, Download, FileText,
		Settings, Server, Image as ImageIcon, Info, Link as LinkIcon, Share2,
		CircleAlert, X, CircleX, Tag
	} from '@lucide/svelte';
	import { apiRequest } from '$lib/auth';
	import { wordCountLabel, normalizeWordCountMode } from '$lib/media/word-count';
	import { normalizeLinkDurationMonths } from '$lib/media/link-duration';
	import { mediaDistribution } from '$lib/media/distribution';
	import { getCategoryTone } from '$lib/media/category-tone';
	import { MEDIA_LABEL } from '../taut-pintar/_shared';

	export type MediaDetailItem = {
		id: string;
		name: string;
		category: string | null;
		categories?: string[];
		description: string | null;
		website_url: string | null;
		contoh_artikel_url?: string | null;
		bahasa_artikel?: string[] | null;
		price: number;
		pricing_type?: string | null;
		client_price: number;
		markup_percent?: number | null;
		credit_cost: number;
		integration_type: string;
		traffic_estimate: number;
		media_tier: string | null;
		media_type?: string | null;
		media_quality_score: number | null;
		effective_quality: number | null;
		quality_source: 'media' | 'tier_default' | null;
		health_score: number | null;
		health_computed_at?: string | null;
		health_breakdown?: Record<string, unknown> | null;
		published_count?: number | null;
		dewan_pers_verified?: boolean | null;
		domain_verified?: boolean | null;
		seo_da?: number | null;
		seo_pa?: number | null;
		seo_spam_score?: number | null;
		seo_checked_at?: string | null;
		seo_ai_trust?: number | null;
		seo_ai_citation?: boolean | null;
		gsc_connected?: boolean;
		sw_visits?: number | null;
		sw_snapshot_date?: string | null;
		rating_avg?: number | null;
		rating_count?: number | null;
		partner_id?: string | null;
		partner_public_id?: string | null;
		partner_name?: string | null;
		status?: string | null;
		label_sponsor?: string | null;
		topik_terlarang?: string[] | null;
		max_links?: number | null;
		allow_hyperlink?: boolean | null;
		link_duration_months?: number | null;
		image_required?: boolean | null;
		publication_mode?: string | null;
		word_count_mode?: string | null;
		allow_embed?: boolean | null;
		auto_publish?: boolean | null;
		price_status?: string | null;
		created_at?: string | null;
	};

	let {
		media,
		onclose,
		onProceed
	}: {
		media: MediaDetailItem;
		onclose: () => void;
		onProceed?: () => void;
	} = $props();

	const brandId = $derived(page.params.brandId as string);

	const fmtCompact = (value: number) =>
		value > 0 ? new Intl.NumberFormat('id-ID', { notation: 'compact', maximumFractionDigits: 1 }).format(value) : null;
	const idr = (value: number) =>
		new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0, roundingMode: 'trunc' }).format(value);

	const autoPath = $derived(Boolean(media.auto_publish));
	const trafficSw = $derived(media.sw_visits != null && media.sw_visits > 0 ? fmtCompact(media.sw_visits) : null);
	const trafficEst = $derived(
		media.traffic_estimate != null && media.traffic_estimate > 0
			? `± ${new Intl.NumberFormat('id-ID').format(media.traffic_estimate)} / bln`
			: null
	);
	const trafficText = $derived(trafficSw ? `${trafficSw} / bln` : trafficEst ?? '—');
	const daText = $derived(media.seo_da != null ? String(media.seo_da) : '—');
	const aiTrustText = $derived(media.seo_ai_trust != null ? String(media.seo_ai_trust) : '—');
	const ratingText = $derived(
		media.rating_avg != null && media.rating_avg > 0
			? `${media.rating_avg.toFixed(1).replace('.', ',')}/5`
			: '—'
	);
	// Rekam jejak distribusi: jumlah artikel yang sudah terbit lewat media ini
	// di seluruh platform. Di tabel katalog ini tampil sebagai baris kecil di
	// bawah nama media; di modal perlu ikut tampil supaya informasinya tidak
	// hilang di layar kecil.
	const dist = $derived(mediaDistribution(media.health_breakdown, media.published_count));
	const distText = $derived(
		!dist.known ? '—' : dist.published > 0 ? `${dist.published} ${tr('artikel')}` : tr('Belum ada distribusi')
	);
	const distTitle = $derived(
		dist.published > 0
			? `${dist.published} ${tr('artikel terdistribusi')} ${tr('— jumlah artikel yang sudah terbit lewat media ini di seluruh platform Tauthub.')}`
			: tr('Belum ada distribusi')
	);
	const ratingTitle = $derived.by(() => {		const avg = media.rating_avg;
		if (avg == null || !(avg > 0)) return tr('Belum ada penilaian dari klien.');
		const base = tr('Rating kecepatan terbit {avg}/5 dari klien yang menghubungi mitra.').replace(
			'{avg}',
			avg.toFixed(1)
		);
		const count = media.rating_count ?? 0;
		return count > 0 ? base + ' ' + tr('Berdasarkan {n} penilaian.').replace('{n}', String(count)) : base;
	});

	let crawlArticles = $state<number | null>(null);
	let crawlAt = $state<string | null>(null);
	let crawlStale = $state(false);
	let copiedShare = $state(false);
	let catPopoverOpen = $state(false);
	let infoTipOpen = $state(false);
	let exportingInfo = $state(false);
	let infoExportFailed = $state(false);

	// Content velocity dari crawlhub API
	onMount(() => {
		if (!brandId || !media.id) return;
		let cancelled = false;
		void (async () => {
			try {
				const res = await apiRequest(`/v1/brands/${brandId}/media/${media.id}/crawl-status`);
				if (!res.ok || cancelled) return;
				const data = (await res.json()) as {
					articles_7d?: number | null;
					last_crawled_at?: string | null;
					stale?: boolean;
				};
				if (!cancelled) {
					crawlArticles = data.articles_7d ?? null;
					crawlAt = data.last_crawled_at ?? null;
					crawlStale = data.stale === true;
				}
			} catch {
				if (!cancelled) crawlArticles = null;
			}
		})();
		return () => {
			cancelled = true;
		};
	});

	const velocityBadge = $derived(
		crawlArticles == null
			? null
			: crawlArticles >= 1
				? { label: 'PRODUKTIF', isProductive: true }
				: { label: 'STAGNAN', isProductive: false }
	);

	const score = $derived(media.health_score);
	const healthLabel = $derived(
		score == null ? tr('Belum Diskor') : score >= 70 ? 'Sangat Sehat' : score >= 40 ? 'Perlu Perhatian' : 'Kondisi Rendah'
	);

		const b = $derived((media.health_breakdown ?? {}) as Record<string, unknown>);
		const published = $derived(Number(b.published ?? 0));
		const rejected = $derived(Number(b.rejected ?? 0));
		const failed = $derived(Number(b.failed ?? 0));
		const totalTx = $derived(published + rejected + failed);
		const hasHistory = $derived(totalTx > 0);
		const avgPublishHours = $derived(b.avg_publish_hours != null ? Number(b.avg_publish_hours) : null);

		const METRIC_MAX = { fulfillment: 30, sla: 35, permanence: 20, uptime: 15 };
		const fulfillmentPoints = $derived(Number(b.success_rate ?? 0));
		const slaPoints = $derived(Number(b.publish_time ?? 0));
		const permanencePoints = $derived(Number(b.rejection_rate ?? 0));
		const uptimePoints = $derived(Number(b.availability ?? 0));

	const pctOf = (points: number, max: number) => `${Math.round((points / max) * 1000) / 10}%`;
	const fulfillmentPct = $derived(pctOf(fulfillmentPoints, METRIC_MAX.fulfillment));
	const permanencePct = $derived(pctOf(permanencePoints, METRIC_MAX.permanence));
	const uptimePct = $derived(pctOf(uptimePoints, METRIC_MAX.uptime));

	const slaText = $derived(
		hasHistory && avgPublishHours != null
			? `${avgPublishHours.toLocaleString('id-ID', { maximumFractionDigits: 1 })} jam`
			: '—'
	);

	const maxLinksCount = $derived(media.max_links);
	// Masa berlaku tautan mengikuti kebijakan media (0 = permanen).
	const linkDurationMonths = $derived(normalizeLinkDurationMonths(media.link_duration_months));
	const permanentLink = $derived(linkDurationMonths === 0);
	const linkDurationText = $derived(
		permanentLink ? tr('Permanen — tautan tidak pernah dilepas') : `${linkDurationMonths} ${tr('Bulan')} · ${tr('kebijakan media')}`
	);
	const linkPolicy = $derived(
		maxLinksCount == null
			? 'Maks. 2 Dofollow'
			: maxLinksCount === 0
				? tr('Tidak Ada (Tanpa Link)')
				: `Maks. ${maxLinksCount} ${media.allow_hyperlink ? 'Dofollow' : 'Nofollow'}`
	);

	const wordCountNote = $derived.by(() => {
		if (normalizeWordCountMode(media.word_count_mode) !== 'sesuai_permintaan') return null;
		return media.media_type === 'blogger'
			? tr('≈ 500–5.000 kata (mengikuti permintaan klien)')
			: tr('≈ mulai 300 kata (mengikuti permintaan klien)');
	});

	const GENERAL_RESTRICTIONS = $derived(['SARA', 'Judi / Kasino', 'Dewasa (Adult)', 'Kripto / Forex']);
	const partnerTopics = $derived((media.topik_terlarang ?? []).map((s) => s.trim()).filter(Boolean));
	const gatekeeperList = $derived(Array.from(new Set([...GENERAL_RESTRICTIONS, ...partnerTopics])));

	const isOrganic = $derived(media.label_sponsor !== 'ada_sponsor');
	const sponsorText = $derived(isOrganic ? tr('✓ Tidak Ada (Organik)') : 'Ada Label Sponsor / Adv');
	const isDirectPublish = $derived(media.publication_mode === 'publish');
	const initialLetter = $derived((media.name || 'M').trim().charAt(0).toUpperCase());
	// Favicon langsung dari website media (layanan favicon publik Google);
	// gagal muat → jatuh ke inisial huruf.
	const faviconDomain = $derived(
		(media.website_url ?? '').replace(/^https?:\/\/(www\.)?/i, '').replace(/\/+$/, '').trim()
	);
	let faviconFailed = $state(false);

	const cats = $derived(
		Array.isArray(media.categories) && media.categories.length > 0
			? media.categories
			: media.category
				? [media.category]
				: ['Umum']
	);
	const firstCategory = $derived(cats[0]);
	const extraCategories = $derived(cats.slice(1));
	// Jenis media (Blogger / Mainstream) ditampilkan di baris identitas supaya
	// klien tahu tipe medianya sebelum memesan; kosong bila data tidak ada.
	const mediaTypeLabel = $derived(media.media_type ? (MEDIA_LABEL[media.media_type] ?? media.media_type) : '');

	function jakartaStamp(iso: string | null | undefined): string {
		if (!iso) return '—';
		const at = new Date(iso);
		if (Number.isNaN(at.getTime())) return '—';
		const dayFmt = new Intl.DateTimeFormat('id-ID', {
			timeZone: 'Asia/Jakarta',
			day: 'numeric',
			month: 'short',
			year: 'numeric'
		});
		const time = new Intl.DateTimeFormat('id-ID', {
			timeZone: 'Asia/Jakarta',
			hour: '2-digit',
			minute: '2-digit',
			hour12: false
		}).format(at);
		const day = dayFmt.format(at);
		if (day === dayFmt.format(new Date())) return `Hari ini, ${time} WIB`;
		if (day === dayFmt.format(new Date(Date.now() - 86_400_000))) return `Kemarin, ${time} WIB`;
		return `${day}, ${time} WIB`;
	}

	function handleShare() {
		if (typeof window !== 'undefined') {
			const url = media.website_url ?? window.location.href;
			navigator.clipboard
				.writeText(url)
				.then(() => {
					copiedShare = true;
					setTimeout(() => (copiedShare = false), 2000);
				})
				.catch(() => {});
		}
	}

	/** Unduh ringkasan info media sebagai laporan PDF bermerek Tauthub. */
	async function exportInfo() {
		if (exportingInfo) return;
		exportingInfo = true;
		infoExportFailed = false;
		try {
			// jspdf relatif berat, jadi baru diunduh saat tombol ini dipakai.
			const { buildMediaInfoPdf, mediaInfoPdfFileName } = await import('$lib/media/media-info-pdf');
			const doc = buildMediaInfoPdf({
				name: media.name,
				domainLabel: media.website_url
					? media.website_url.replace(/^https?:\/\/(www\.)?/, '').replace(/\/$/, '')
					: '—',
				websiteUrl: media.website_url,
				categories: cats,
				mediaTypeLabel,
				verified: media.domain_verified !== false,
				dewanPers: Boolean(media.dewan_pers_verified),
				trafficText,
				daText,
				aiTrustText,
				autoPath,
				healthScore: score,
				healthLabel,
				healthMax: 100,
				metrics: [
					{
						label: tr('Fulfillment Rate'),
						value: fulfillmentPct,
						pct: (fulfillmentPoints / METRIC_MAX.fulfillment) * 100
					},
					{
						label: tr('SLA Responsiveness'),
						value: slaText === '—' ? tr('Belum ada riwayat') : slaText,
						pct: (slaPoints / METRIC_MAX.sla) * 100
					},
					{
						label: tr('Link Permanence'),
						value: permanencePct,
						pct: (permanencePoints / METRIC_MAX.permanence) * 100
					},
					{
						label: tr('Server Uptime'),
						value: uptimePct,
						pct: (uptimePoints / METRIC_MAX.uptime) * 100
					}
				],
				historyLine: hasHistory
					? `${tr('Riwayat')}: ${published} ${tr('terbit')}, ${rejected} ${tr('ditolak')}, ${failed} ${tr('gagal')}.`
					: tr('Belum ada riwayat publikasi (seluruh komponen bernilai 0 sesuai perhitungan engine).'),
				parameters: [
					{
						label: tr('Garansi Tayang (SLA)'),
						value: permanentLink
							? tr('Permanen — tautan tidak pernah dilepas')
							: `${linkDurationMonths} ${tr('Bulan')} (${tr('mengikuti kebijakan media')})`
					},
					{
						label: tr('Label Sponsor / Adv'),
						value: media.label_sponsor === 'ada_sponsor' ? tr('Ada Label Sponsor / Adv') : tr('Tidak Ada (Organik)')
					},
					{ label: tr('Batas Tautan Keluar'), value: linkPolicy },
					{ label: tr('Panjang Artikel'), value: wordCountLabel(media.word_count_mode) },
					{
						label: tr('Support Embed (Video/Maps)'),
						value: media.allow_embed ? tr('Diizinkan') : tr('Tidak Diizinkan')
					},
					{
						label: tr('Mode Pengiriman'),
						value: isDirectPublish
							? tr('Langsung Terbit (otomatis, tanpa langkah terbit manual di media)')
							: tr('Draf (diterbitkan oleh media)')
					}
				],
				gatekeeper: gatekeeperList,
				priceText: idr(media.client_price ?? media.price),
				generatedAt: jakartaStamp(new Date().toISOString()),
				reference: `REF ${media.id.slice(0, 8).toUpperCase()}`,
				labels: {
					reportTitle: 'Media Information Report',
					generatedAt: tr('Dibuat'),
					traffic: tr('Estimasi Trafik'),
					da: tr('Domain Authority'),
					aiTrust: tr('AI Trust Score'),
					publicationPath: tr('Jalur Publikasi'),
					partnerKpi: tr('Indikator Kinerja Mitra'),
					parameters: tr('Parameter AI & Redaksi'),
					gatekeeper: tr('Pembatasan Gatekeeper'),
					publicationCost: tr('Biaya Publikasi'),
					directPathTitle: 'Tauthub Secure Direct™',
					directPathDesc: tr(
						'Artikel langsung terbit ke media target setelah submit pesanan tanpa perlu persetujuan manual, riwayat tayang otomatis akan terpantau di ringkasan kampanye.'
					),
					reviewPathTitle: 'Editorial Review (SLA Tracked)',
					reviewPathDesc: tr(
						'Naskah akan ditinjau secara manual oleh redaksi media, seluruh proses antrian penerbitan, revisi dan waktu respon dipantau pada halaman ringkasan kampanye.'
					),
					restrictedTopics: tr(
						'Naskah yang mengandung topik berikut akan diblokir otomatis oleh AI Validator sebelum saldo Anda terpotong:'
					),
					footerNote: `${tr('Dibuat otomatis oleh Tauthub')} · tauthub.com`,
					page: tr('Halaman')
				}
			});
			doc.save(mediaInfoPdfFileName(media.name));
		} catch {
			infoExportFailed = true;
			setTimeout(() => (infoExportFailed = false), 2500);
		} finally {
			exportingInfo = false;
		}
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') onclose();
	}
</script>

<svelte:window onkeydown={onKeydown} />

<div class="detailOverlay" role="presentation" onclick={onclose}>
	<div class="detailModalWrapper">
		<dialog open class="detailModal" aria-label={media.name} onclick={(e) => e.stopPropagation()}>
			<!-- ── Header Modal ──────────────────────────────────────── -->
			<div class="detailHeader">
				<div class="detailHeaderLeft">
					<div class="detailHeaderAvatar">
						{#if faviconDomain && !faviconFailed}
							<img
								src={`https://www.google.com/s2/favicons?domain=${encodeURIComponent(faviconDomain)}&sz=128`}
								alt={media.name}
								loading="lazy"
								decoding="async"
								onerror={() => (faviconFailed = true)}
							/>
						{:else}
							<span class="detailHeaderAvatarFallback">{initialLetter}</span>
						{/if}
					</div>

					<div class="detailHeaderInfo">
						<div class="detailHeaderTitleRow">
							<h3 class="detailTitle">{media.name}</h3>
							{#if media.domain_verified !== false}
								<span class="verifiedBadge" title="Domain terverifikasi">
									<CircleCheck size={14} />
									<span>VERIFIED</span>
								</span>
							{/if}
							{#if media.dewan_pers_verified}
								<span class="dewanPersPill" title="Terverifikasi Dewan Pers">
									DEWAN PERS
								</span>
							{/if}
						</div>
						<div class="detailCategoryRow">
							{#if media.website_url}
								<a
									class="detailDomainText"
									href={media.website_url}
									target="_blank"
									rel="noreferrer"
									title={`Buka ${media.website_url}`}
								>
									{media.website_url.replace(/^https?:\/\/(www\.)?/, '').replace(/\/$/, '')}
								</a>
							{:else}
								<span class="detailDomainText">—</span>
							{/if}
							<span class="detailDotSeparator">·</span>
							{#if mediaTypeLabel}
								<span class="detailCategoryText">{tr('Tipe')}: {mediaTypeLabel}</span>
								<span class="detailDotSeparator">·</span>
							{/if}
							<span class="detailCategoryText">
								Kategori: {firstCategory}
								{#if extraCategories.length > 0}
									<span class="categoryPopoverAnchor">
										<button
											type="button"
											class="categoryPlusTag"
											aria-label={`${cats.length} kategori: ${cats.join(', ')}`}
											onclick={() => (catPopoverOpen = !catPopoverOpen)}
										>
											+{extraCategories.length}
										</button>
										{#if catPopoverOpen}
											<div class="categoryPopover">
												<div class="categoryPopoverTitle">{tr("Semua Kategori")}</div>
													<div class="categoryPopoverList">
														{#each cats as c (c)}
															<span class="categoryPill categoryPill--{getCategoryTone(c)}">{c}</span>
														{/each}
													</div>
											</div>
										{/if}
									</span>
								{/if}
							</span>
						</div>
					</div>
				</div>

				<!-- Aksi Kanan Atas: Bagikan, Ekspor Info, Tutup -->
				<div class="detailHeaderActions">
					<button
						class="detailSharePillBtn"
						type="button"
						onclick={handleShare}
						title={copiedShare ? 'Tautan disalin!' : 'Bagikan tautan'}
					>
						{#if copiedShare}
							<Check size={16} color="#16a34a" />
							<span style="color: #16a34a;">Disalin!</span>
						{:else}
							<Share2 size={16} />
							<span>Bagikan</span>
						{/if}
					</button>

					<button
						class="detailExportPillBtn"
						type="button"
						onclick={exportInfo}
						disabled={exportingInfo}
						title={tr('Unduh informasi media (PDF)')}
					>
						<Download size={16} />
						<span>{exportingInfo ? tr('Menyiapkan…') : infoExportFailed ? tr('Gagal, coba lagi') : 'Ekspor Info'}</span>
					</button>

					<button class="detailCloseBtn" type="button" aria-label={tr("Tutup")} onclick={onclose}>
						<X size={18} />
					</button>
				</div>
			</div>

			<!-- ── Baris Metrik Dinamis (Trafik | DA | AI Trust) ────── -->
			<div class="detailMetricsRow">
				<div class="detailMetricItem">
					<span class="detailMetricLabel">Estimasi Trafik:</span>
					<span class="detailMetricValue">{trafficText}</span>
				</div>
				<div class="detailMetricItem">
					<span class="detailMetricLabel">Domain Auth:</span>
					<span class="detailMetricValue">{daText}</span>
				</div>
				<div class="detailMetricItem">
					<span class="detailMetricLabel">AI Trust Score:</span>
					<span class="detailMetricValueGood">{aiTrustText}</span>
				</div>
				<div class="detailMetricItem" title={ratingTitle}>
					<span class="detailMetricLabel">Rating:</span>
					<span class="detailMetricValue">{ratingText}</span>
				</div>
				<div class="detailMetricItem detailMetricDistribution" title={distTitle}>
					<span class="detailMetricLabel">{tr('Distribusi')}:</span>
					<span class="detailMetricValue" class:detailMetricMuted={dist.published === 0}>{distText}</span>
				</div>
			</div>

			<!-- ── Modal Body: Dua Kolom ─────────────────────────────── -->
			<div class="detailBody">
				<div class="detailColumns">
					<!-- Kolom Kiri -->
					<div class="detailColumn">
						<!-- Kartu Hero Jalur Publikasi -->
						<div class={autoPath ? 'heroCardAuto' : 'heroCardReview'}>
							<div class="heroCardReviewContent">
								<span class="heroCardEyebrow">JALUR PUBLIKASI</span>
								{#if autoPath}
									<h4 class="heroCardTitle">Tauthub Secure Direct™</h4>
									<p class="heroCardDesc">{tr("Artikel langsung terbit ke media target setelah submit pesanan tanpa perlu persetujuan manual, riwayat tayang otomatis akan terpantau di ringkasan kampanye.")}</p>
								{:else}
									<h4 class="heroCardTitle">Editorial Review (SLA Tracked)</h4>
									<p class="heroCardDesc">{tr("Naskah akan ditinjau secara manual oleh redaksi media, seluruh proses antrian penerbitan, revisi dan waktu respon dipantau pada halaman ringkasan kampanye.")}</p>
								{/if}
							</div>

							<div class="heroIllustrationWrapper">
								<img
									src={autoPath
										? '/images/secure-direct-illustration.webp'
										: '/images/manual-review-illustration.webp'}
									alt={autoPath ? 'Ilustrasi Tauthub Secure Direct' : 'Ilustrasi Editorial Review'}
									width="140"
									height="140"
									class="heroIllustrationImg"
								/>
							</div>
						</div>

						<!-- Kartu Indikator Kinerja Mitra -->
						<div class="perfCard">
							<div class="perfCardHead">
								<div class="perfCardTitleGroup">
									<ChartBar size={20} color="#2563eb" />
									<span class="perfCardTitle">{tr("Indikator Kinerja Mitra")}</span>
									<span class="infoTipAnchor">
										<button
											type="button"
											class="infoTipBtn"
											aria-label="Cara data kinerja ini dipantau"
											onclick={() => (infoTipOpen = !infoTipOpen)}
										>
											<Info size={14} />
										</button>
										{#if infoTipOpen}
											<div class="infoTipTooltip" role="tooltip">
												Data dipantau otomatis berdasarkan riwayat {totalTx} transaksi terakhir.
												<br />Update terakhir: {jakartaStamp(media.health_computed_at)}
											</div>
										{/if}
									</span>
								</div>
								<span class="perfHealthBadge">
									<span class="perfHealthDot"></span>
									<span>HEALTH: {healthLabel.toUpperCase()}</span>
								</span>
							</div>

							<!-- 4 Kotak Sub-Card (Grid 2x2) -->
							<div class="perfGrid">
								<div class="perfSubCard">
									<div class="perfSubIconBox">
										<FileText size={20} />
									</div>
									<div class="perfSubContent">
										<span class="perfSubLabel">Fulfillment Rate</span>
										<div class="perfSubValue">{fulfillmentPct}</div>
										<span class="perfSubSub">Rasio order sukses diterbitkan</span>
									</div>
								</div>

								<div class="perfSubCard">
									<div class="perfSubIconBox">
										<Clock size={20} />
									</div>
									<div class="perfSubContent">
										<span class="perfSubLabel">SLA Responsiveness</span>
										<div class="perfSubValue">{slaText}</div>
										<span class="perfSubSub">Rata-rata waktu tayang</span>
									</div>
								</div>

								<div class="perfSubCard">
									<div class="perfSubIconBox">
										<LinkIcon size={20} />
									</div>
									<div class="perfSubContent">
										<span class="perfSubLabel">Link Permanence</span>
										<div class="perfSubValue">{permanencePct}</div>
										<span class="perfSubSub">Integritas retensi artikel</span>
									</div>
								</div>

								<div class="perfSubCard">
									<div class="perfSubIconBox">
										<Server size={20} />
									</div>
									<div class="perfSubContent">
										<span class="perfSubLabel">Server Uptime</span>
										<div class="perfSubValue">{uptimePct}</div>
										<span class="perfSubSub">Stabilitas akses web (30 hari)</span>
									</div>
								</div>
							</div>

							<!-- Content Velocity dari Crawlhub -->
							<div class="velocityCard">
								<div class="velocityIconBox">
									<FileText size={18} />
								</div>
								<div class="velocityContent">
									<span class="velocityTitle">Content Velocity</span>
									<span class="velocitySub">Rata-rata artikel terbit / minggu</span>
									{#if crawlAt}
										<span class="velocitySub">
											Data per {jakartaStamp(crawlAt)}{crawlStale ? ' (belum diperbarui)' : ''}
										</span>
									{/if}
								</div>
								<div class="velocityRight">
									{#if crawlArticles == null}
										<span class="velocityEmpty">{tr("Belum ada data crawl")}</span>
									{:else}
										<div class="velocityValue">
											{crawlArticles} <span class="velocityValueUnit">artikel</span>
										</div>
										{#if velocityBadge && !crawlStale}
											<span
												class={velocityBadge.isProductive
													? 'velocityBadge'
													: 'velocityBadgeStagnant'}
											>
												{velocityBadge.label}
											</span>
										{/if}
									{/if}
								</div>
							</div>
						</div>
					</div>

					<!-- Kolom Kanan -->
					<div class="detailColumn">
						<!-- Parameter AI & Redaksi -->
						<div class="paramCard">
							<div class="paramCardHead">
								<Settings size={20} color="#2563eb" />
								<span class="paramCardTitle">Parameter AI & Redaksi</span>
							</div>

							<div class="paramRows">
									<div class="paramItemRow">
										<div class="paramItemLeftGroup">
											<Calendar size={18} class="paramRowIcon" />
											<span>Garansi Tayang (SLA)</span>
										</div>
										<span
											class="paramBadgePill"
											class:paramBadgePillPermanent={permanentLink}
										>{linkDurationText}</span>
									</div>

								<div class="paramItemRow">
									<div class="paramItemLeftGroup">
										<Tag size={18} class="paramRowIcon" />
										<span>Label Sponsor / Adv</span>
									</div>
									<span class="paramBadgePillGreen">{sponsorText}</span>
								</div>

								<div class="paramItemRow">
									<div class="paramItemLeftGroup">
										<LinkIcon size={18} class="paramRowIcon" />
										<span>{tr("Batas Tautan Keluar")}</span>
									</div>
									<span class="paramValueBold">{linkPolicy}</span>
								</div>

								<div class="paramItemRow">
									<div class="paramItemLeftGroup">
										<FileText size={18} class="paramRowIcon" />
										<span>{tr("Panjang Artikel")}</span>
									</div>
									<div style="text-align: right;">
										<span class="paramValueBold">{wordCountLabel(media.word_count_mode)}</span>
										{#if wordCountNote}
											<div class="paramValueMuted" style="font-size: 11px;">{wordCountNote}</div>
										{/if}
									</div>
								</div>

								<div class="paramItemRow">
									<div class="paramItemLeftGroup">
										<ImageIcon size={18} class="paramRowIcon" />
										<span>Support Embed (Video/Maps)</span>
									</div>
									{#if media.allow_embed}
										<span class="paramBadgePillGreen">
											<Check size={13} aria-hidden="true" />
											Diizinkan
										</span>
									{:else}
										<span class="paramValueMuted">{tr("Tidak Diizinkan")}</span>
									{/if}
								</div>

								<!-- Rekam jejak distribusi. Di HP baris ini pindah dari
								     strip metrik ke sini supaya strip tidak penuh digeser. -->
								<div class="paramItemRow paramRowDistribution" title={distTitle}>
									<div class="paramItemLeftGroup">
										<ChartBar size={18} class="paramRowIcon" />
										<span>{tr('Distribusi')}</span>
									</div>
									<span class={dist.published === 0 ? 'paramValueMuted' : 'paramValueBold'}>{distText}</span>
								</div>
							</div>
						</div>

						<!-- Pembatasan Gatekeeper -->
						<div class="gatekeeperCard">
							<div class="gatekeeperHead">
								<CircleAlert size={20} class="gatekeeperIcon" />
								<span class="gatekeeperTitle">Pembatasan Gatekeeper</span>
							</div>

							<p class="gatekeeperDesc">{tr("Naskah yang mengandung topik berikut akan diblokir otomatis oleh AI Validator sebelum saldo Anda terpotong:")}</p>

							<div class="gatekeeperChips">
								{#each gatekeeperList as tag (tag)}
									<span class="gatekeeperChip">
										<CircleX size={14} />
										<span>{tag}</span>
									</span>
								{/each}
							</div>
						</div>
					</div>
				</div>
			</div>

			<!-- ── Footer Modal ──────────────────────────────────────── -->
			<div class="detailFooter">
				<div class="detailFooterLeft">
					<div class="detailFooterCoinIcon">
						<!-- Ikon koin kustom (pengganti Lucide Coins) untuk label biaya. -->
						<svg xmlns="http://www.w3.org/2000/svg" width="26" height="26" viewBox="0 0 24 24" aria-hidden="true" style="color: #2563eb;">
							<path d="M0 0h24v24H0z" fill="none" />
							<g fill="none">
								<path fill="currentColor" d="m4.23 3.16l-3 4.39h18.6l-3.1-4.22z" opacity=".2" />
								<rect width="12.66" height="20.39" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" rx="3" transform="matrix(0 -1 -1 0 22.29 20.21)" />
								<path stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M1.9 12.07v-2.5c0-2.83 0-4.24.88-5.12s2.3-.88 5.12-.88h6.16c1.74 0 2.61 0 3.32.43c.71.42 1.13 1.19 1.96 2.72l.17.33" />
								<path fill="currentColor" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M18.44 13.88a.7.7 0 1 1-1.4 0a.7.7 0 0 1 1.4 0" />
							</g>
						</svg>
					</div>
					<div class="detailFooterAmountCol">
						<span class="detailFooterLabel">BIAYA PUBLIKASI</span>
						<div class="detailFooterAmountGroup">
							<span class="detailFooterPriceBig">
								{idr(media.client_price ?? media.price)}
							</span>
						</div>
					</div>
				</div>

				<button
					class="detailCtaBtnBlue"
					type="button"
					onclick={() => {
						if (onProceed) onProceed();
						else onclose();
					}}
				>
					<span>Lanjut ke Dapur Produksi</span>
					<svg
						xmlns="http://www.w3.org/2000/svg"
						width="18"
						height="18"
						viewBox="0 0 24 24"
						aria-hidden="true"
						style="flex-shrink: 0;"
					>
						<path d="M0 0h24v24H0z" fill="none" />
						<path
							fill="currentColor"
							d="M21.928 11.607c-.202-.488-.635-.605-.928-.633V8c0-1.103-.897-2-2-2h-6V4.61c.305-.274.5-.668.5-1.11a1.5 1.5 0 0 0-3 0c0 .442.195.836.5 1.11V6H5c-1.103 0-2 .897-2 2v2.997l-.082.006A1 1 0 0 0 1.99 12v2a1 1 0 0 0 1 1H3v5c0 1.103.897 2 2 2h14c1.103 0 2-.897 2-2v-5a1 1 0 0 0 1-1v-1.938a1 1 0 0 0-.072-.455M5 20V8h14l.001 3.996L19 12v2l.001.005l.001 5.995z"
						/>
						<ellipse cx="8.5" cy="12" fill="currentColor" rx="1.5" ry="2" />
						<ellipse cx="15.5" cy="12" fill="currentColor" rx="1.5" ry="2" />
						<path fill="currentColor" d="M8 16h8v2H8z" />
					</svg>
				</button>
			</div>
		</dialog>
	</div>
</div>

<style>
	.detailOverlay {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.45);
		backdrop-filter: blur(2px);
		display: flex;
		align-items: center;
		justify-content: center;
		z-index: 1000;
		padding: 16px;
	}

	.detailModalWrapper {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 14px;
		width: 100%;
		max-width: 1200px;
		max-height: 94vh;
	}

	.detailModal {
		width: 100%;
		max-width: 1200px;
		max-height: 92vh;
		background: #ffffff;
		border: 1px solid #e2e8f0;
		border-radius: 16px;
		box-shadow:
			0 20px 48px rgba(0, 0, 0, 0.16),
			0 2px 8px rgba(0, 0, 0, 0.04);
		display: flex;
		flex-direction: column;
		overflow: hidden;
		animation: modalFadeIn 0.15s cubic-bezier(0.16, 1, 0.3, 1);
		color: #0f172a;
	}

	@keyframes modalFadeIn {
		from {
			opacity: 0;
			transform: scale(0.97) translateY(4px);
		}
		to {
			opacity: 1;
			transform: scale(1) translateY(0);
		}
	}

	/* Header */
	.detailHeader {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		padding: 20px 32px;
		border-bottom: 1px solid #f1f5f9;
		background: #ffffff;
	}
	.detailHeaderLeft {
		display: flex;
		align-items: center;
		gap: 14px;
		min-width: 0;
	}
	.detailHeaderAvatar {
		width: 48px;
		height: 48px;
		border-radius: 50%;
		background: transparent;
		border: none;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 26px;
		font-weight: 800;
		color: #2563eb;
		flex-shrink: 0;
		overflow: hidden;
	}
	.detailHeaderAvatar img {
		width: 100%;
		height: 100%;
		object-fit: cover;
		display: block;
	}
	/* Fallback inisial tetap butuh latar + cincin agar hurufnya terbaca. */
	.detailHeaderAvatarFallback {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 100%;
		height: 100%;
		border-radius: 50%;
		background: #ffffff;
		border: 1px solid #e2e8f0;
	}
	.detailHeaderInfo {
		display: flex;
		flex-direction: column;
		gap: 3px;
		min-width: 0;
	}
	.detailHeaderTitleRow {
		display: flex;
		align-items: center;
		gap: 10px;
		flex-wrap: wrap;
	}
	.detailTitle {
		margin: 0;
		font-size: 18px;
		font-weight: 750;
		color: #0f172a;
		letter-spacing: -0.01em;
	}
	.verifiedBadge {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 3px 10px;
		border-radius: 6px;
		border: 1px solid #bfdbfe;
		background: #eff6ff;
		color: #2563eb;
		font-size: 11.5px;
		font-weight: 800;
		letter-spacing: 0.04em;
	}
	.dewanPersPill {
		display: inline-flex;
		align-items: center;
		padding: 3px 10px;
		border-radius: 6px;
		border: 1px solid #bbf7d0;
		background: #f0fdf4;
		color: #16a34a;
		font-size: 11.5px;
		font-weight: 800;
		letter-spacing: 0.04em;
	}
	.detailCategoryRow {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		flex-wrap: wrap;
	}
	.detailDomainText {
		font-size: 13px;
		font-weight: 500;
		color: #64748b;
		text-decoration: none;
	}
	.detailDomainText:hover {
		text-decoration: underline;
		color: #0f172a;
	}
	.detailDotSeparator {
		color: #cbd5e1;
		font-size: 13px;
	}
	.detailCategoryText {
		color: #64748b;
	}
	.categoryPopoverAnchor {
		position: relative;
		display: inline-block;
	}
	.categoryPlusTag {
		display: inline-block;
		padding: 2px 6px;
		background: #e2e8f0;
		color: #475569;
		border-radius: 6px;
		font-size: 11px;
		font-weight: 600;
		border: none;
		cursor: pointer;
		margin-left: 6px;
		transition:
			background 0.12s ease,
			color 0.12s ease;
	}
	.categoryPlusTag:hover {
		background: #cbd5e1;
		color: #0f172a;
	}
	.categoryPopover {
		position: absolute;
		left: 0;
		top: calc(100% + 4px);
		z-index: 50;
		background: #ffffff;
		border: 1px solid #e2e8f0;
		border-radius: 10px;
		padding: 10px 12px;
		box-shadow: 0 10px 25px rgba(0, 0, 0, 0.1);
		min-width: 180px;
	}
	.categoryPopoverTitle {
		font-size: 11px;
		font-weight: 700;
		color: #64748b;
		text-transform: uppercase;
		margin-bottom: 6px;
	}
	.categoryPopoverList {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
		.categoryPill {
			display: inline-flex;
			align-items: center;
			padding: 3px 8px;
			border-radius: 6px;
			font-size: 11.5px;
			font-weight: 600;
			line-height: 1.25;
			border: 1px solid #cbd5e1;
			background: #f1f5f9;
			color: #1e293b;
		}
		.categoryPill--charcoal { background: #f1f5f9; border-color: #cbd5e1; color: #1e293b; }
		.categoryPill--slate { background: #f8fafc; border-color: #cbd5e1; color: #334155; }
		.categoryPill--stone { background: #fafaf9; border-color: #e7e5e4; color: #57534e; }
		.categoryPill--navy { background: #f0f4f9; border-color: #cbd5e1; color: #0f2744; }
		.categoryPill--cobalt { background: #eff6ff; border-color: #bfdbfe; color: #1d4ed8; }
		.categoryPill--blue { background: #eef6ff; border-color: #cfe4fe; color: #1e40af; }
		.categoryPill--sky { background: #f0f9ff; border-color: #bae6fd; color: #0369a1; }
		.categoryPill--cyan { background: #ecfeff; border-color: #a5f3fc; color: #0e7490; }
		.categoryPill--teal { background: #f0fdfa; border-color: #99f6e4; color: #0f766e; }
		.categoryPill--emerald { background: #ecfdf5; border-color: #a7f3d0; color: #047857; }
		.categoryPill--green { background: #f0fdf4; border-color: #bbf7d0; color: #15803d; }
		.categoryPill--lime { background: #f7fee7; border-color: #d9f99d; color: #4d7c0f; }
		.categoryPill--gold { background: #fefce8; border-color: #fef08a; color: #a16207; }
		.categoryPill--amber { background: #fffbeb; border-color: #fde68a; color: #92400e; }
		.categoryPill--orange { background: #fff7ed; border-color: #fed7aa; color: #c2410c; }
		.categoryPill--coral { background: #fff4f2; border-color: #ffd5cc; color: #b93815; }
		.categoryPill--red { background: #fef2f2; border-color: #fecaca; color: #b91c1c; }
		.categoryPill--burgundy { background: #fff1f2; border-color: #fecdd3; color: #9f1239; }
		.categoryPill--pink { background: #fdf2f8; border-color: #fbcfe8; color: #be185d; }
		.categoryPill--rose { background: #fff1f2; border-color: #fecdd3; color: #be123c; }
		.categoryPill--fuchsia { background: #fdf4ff; border-color: #f5d0fe; color: #a21caf; }
		.categoryPill--purple { background: #faf5ff; border-color: #e9d5ff; color: #7e22ce; }
		.categoryPill--violet { background: #f5f3ff; border-color: #ddd6fe; color: #6d28d9; }
		.categoryPill--indigo { background: #eef2ff; border-color: #c7d2fe; color: #4338ca; }

	.detailHeaderActions {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-shrink: 0;
	}
	.detailSharePillBtn,
	.detailExportPillBtn {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		height: 36px;
		padding: 0 16px;
		border-radius: 8px;
		border: 1px solid #e2e8f0;
		background: #ffffff;
		color: #1e293b;
		font-size: 13px;
		font-weight: 600;
		cursor: pointer;
		transition: all 0.15s ease;
		touch-action: manipulation;
	}
	.detailSharePillBtn:hover,
	.detailExportPillBtn:hover {
		background: #f8fafc;
		border-color: #cbd5e1;
	}
	.detailExportPillBtn:disabled {
		opacity: 0.7;
		cursor: progress;
	}
	.detailExportPillBtn:disabled:hover {
		background: #ffffff;
		border-color: #e2e8f0;
	}
	.detailCloseBtn {
		width: 34px;
		height: 34px;
		border-radius: 8px;
		border: 1px solid transparent;
		background: transparent;
		color: #64748b;
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
		transition: all 0.15s ease;
		padding: 0;
		flex-shrink: 0;
		touch-action: manipulation;
	}
	.detailCloseBtn:hover {
		background: #f1f5f9;
		color: #0f172a;
	}

	/* Sub-Header Metrics Strip */
	.detailMetricsRow {
		display: flex;
		align-items: center;
		gap: 40px;
		padding: 14px 32px;
		border-top: 1px solid #f1f5f9;
		border-bottom: 1px solid #f1f5f9;
		background: #fcfdfe;
		flex-wrap: wrap;
	}
	.detailMetricItem {
		display: flex;
		align-items: baseline;
		gap: 8px;
	}
	.detailMetricLabel {
		font-size: 13px;
		font-weight: 600;
		color: #64748b;
	}
	.detailMetricValue {
		font-size: 14px;
		font-weight: 700;
		color: #0f172a;
		font-variant-numeric: tabular-nums;
	}
	.detailMetricValueGood {
		font-size: 14px;
		font-weight: 700;
		color: #16a34a;
		font-variant-numeric: tabular-nums;
	}
	/* Nilai kosong/nol (mis. "Belum ada distribusi") dibedakan dari angka nyata
	   supaya tidak terbaca sebagai data. */
	.detailMetricMuted {
		font-weight: 500;
		color: #64748b;
	}

	/* Body & Columns */
	.detailBody {
		padding: 22px 32px 28px;
		display: flex;
		flex-direction: column;
		gap: 18px;
		overflow-y: auto;
		background: #f8fafc;
	}
	.detailColumns {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 22px;
		align-items: stretch;
	}
	.detailColumn {
		display: flex;
		flex-direction: column;
		gap: 20px;
		min-width: 0;
	}

	/* Hero Cards (Review & Auto) */
	.heroCardReview {
		position: relative;
		overflow: hidden;
		padding: 22px 24px;
		border-radius: 16px;
		background: linear-gradient(135deg, #9a3412 0%, #c2410c 35%, #d97707 70%, #f59e0b 100%);
		border: 1px solid rgba(245, 158, 11, 0.25);
		color: #ffffff;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
	}
	.heroCardAuto {
		position: relative;
		overflow: hidden;
		padding: 22px 24px;
		border-radius: 16px;
		background: radial-gradient(ellipse 70% 60% at 90% 40%, rgba(30, 90, 210, 0.28) 0%, transparent 70%),
			radial-gradient(circle at 10% 15%, rgba(45, 80, 140, 0.22) 0%, transparent 60%),
			linear-gradient(135deg, #111d33 0%, #0a1324 50%, #060d1b 100%);
		border: 1px solid rgba(67, 195, 252, 0.18);
		color: #ffffff;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
	}
	.heroCardReviewContent {
		position: relative;
		z-index: 2;
		flex: 1;
		min-width: 0;
	}
	.heroCardEyebrow {
		font-size: 10px;
		font-weight: 800;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: rgba(255, 255, 255, 0.75);
		display: block;
		margin-bottom: 4px;
	}
	.heroCardTitle {
		font-size: 19px;
		font-weight: 800;
		letter-spacing: -0.01em;
		color: #ffffff;
		margin: 0;
		line-height: 1.25;
	}
	.heroCardDesc {
		font-size: 12.5px;
		line-height: 1.55;
		color: rgba(255, 255, 255, 0.88);
		margin: 6px 0 0 0;
	}
	.heroIllustrationWrapper {
		width: 140px;
		height: 140px;
		flex-shrink: 0;
		position: relative;
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.heroIllustrationImg {
		width: 100%;
		height: 100%;
		object-fit: contain;
		filter: drop-shadow(0 6px 16px rgba(0, 0, 0, 0.16));
	}

	/* Kartu Indikator Kinerja Mitra */
	.perfCard {
		border-radius: 14px;
		border: 1px solid #e2e8f0;
		background: #ffffff;
		padding: 18px 20px;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	.perfCardHead {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		flex-wrap: wrap;
	}
	.perfCardTitleGroup {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.perfCardTitle {
		font-size: 14px;
		font-weight: 700;
		color: #0f172a;
	}
	.infoTipAnchor {
		position: relative;
		display: inline-flex;
		align-items: center;
	}
	.infoTipBtn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 18px;
		height: 18px;
		padding: 0;
		border: none;
		background: transparent;
		color: #94a3b8;
		cursor: help;
	}
	.infoTipTooltip {
		position: absolute;
		right: 0;
		top: calc(100% + 6px);
		z-index: 30;
		width: 240px;
		padding: 8px 10px;
		border-radius: 8px;
		background: #0f172a;
		color: #f8fafc;
		font-size: 11.5px;
		line-height: 1.5;
		white-space: pre-line;
		text-align: left;
		box-shadow: 0 6px 20px rgba(15, 23, 42, 0.25);
	}
	.perfHealthBadge {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 4px 10px;
		border-radius: 999px;
		background: #ecfdf5;
		border: 1px solid #a7f3d0;
		color: #059669;
		font-size: 11px;
		font-weight: 800;
		letter-spacing: 0.04em;
	}
	.perfHealthDot {
		width: 7px;
		height: 7px;
		border-radius: 50%;
		background-color: #10b981;
	}
	.perfGrid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 12px;
	}
	.perfSubCard {
		background: #f8fafc;
		border: 1px solid #f1f5f9;
		border-radius: 12px;
		padding: 12px 14px;
		display: flex;
		align-items: flex-start;
		gap: 12px;
	}
	.perfSubIconBox {
		width: 38px;
		height: 38px;
		border-radius: 10px;
		background: #eff6ff;
		color: #2563eb;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}
	.perfSubContent {
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
	}
	.perfSubLabel {
		font-size: 11.5px;
		font-weight: 600;
		color: #475569;
	}
	.perfSubValue {
		font-size: 20px;
		font-weight: 800;
		letter-spacing: -0.02em;
		color: #0f172a;
		line-height: 1.2;
		font-variant-numeric: tabular-nums;
	}
	.perfSubSub {
		font-size: 10.5px;
		color: #94a3b8;
		margin-top: 1px;
	}

	/* Content Velocity */
	.velocityCard {
		display: flex;
		align-items: center;
		gap: 12px;
		margin-top: 12px;
		padding: 14px 16px;
		border: 1px solid #e2e8f0;
		border-radius: 10px;
		background: #f8fafc;
	}
	.velocityIconBox {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 34px;
		height: 34px;
		flex: none;
		border-radius: 8px;
		background: #eff6ff;
		color: #1d4ed8;
	}
	.velocityContent {
		display: flex;
		flex-direction: column;
		gap: 2px;
		flex: 1 1 auto;
		min-width: 0;
	}
	.velocityTitle {
		font-size: 13px;
		font-weight: 700;
		color: #2563eb;
	}
	.velocitySub {
		font-size: 11.5px;
		color: #64748b;
	}
	.velocityRight {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 4px;
		flex: none;
	}
	.velocityValue {
		font-size: 20px;
		font-weight: 700;
		color: #0f172a;
		line-height: 1;
		font-variant-numeric: tabular-nums;
	}
	.velocityValueUnit {
		font-size: 12px;
		font-weight: 500;
		color: #64748b;
	}
	.velocityBadge {
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.06em;
		padding: 2px 8px;
		border-radius: 999px;
		background: #dbeafe;
		color: #1d4ed8;
	}
	.velocityBadgeStagnant {
		font-size: 10px;
		font-weight: 700;
		letter-spacing: 0.06em;
		padding: 2px 8px;
		border-radius: 999px;
		background: #f1f5f9;
		color: #64748b;
	}
	.velocityEmpty {
		font-size: 11.5px;
		color: #64748b;
	}

	/* Kartu Parameter AI & Redaksi */
	.paramCard {
		border-radius: 14px;
		border: 1px solid #e2e8f0;
		background: #ffffff;
		padding: 18px 20px;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.paramCardHead {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.paramCardTitle {
		font-size: 14px;
		font-weight: 700;
		color: #0f172a;
	}
	.paramRows {
		display: flex;
		flex-direction: column;
	}
	.paramItemRow {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 11px 0;
		border-bottom: 1px solid #f8fafc;
		font-size: 13px;
	}
	.paramItemRow:last-child {
		border-bottom: none;
	}
	/* Distribusi hanya tampil di kartu Parameter saat layar HP; di desktop
	   sudah ada di strip metrik. */
	.paramRowDistribution {
		display: none;
	}
	.paramItemLeftGroup {
		display: flex;
		align-items: center;
		gap: 8px;
		color: #475569;
		font-weight: 500;
	}
	:global(.paramRowIcon) {
		color: #2563eb;
		flex-shrink: 0;
	}
	.paramBadgePill {
		display: inline-flex;
		align-items: center;
		padding: 4px 12px;
		border-radius: 6px;
		background: #eff6ff;
		color: #1d4ed8;
		font-size: 12px;
		font-weight: 700;
	}
	/* Tautan permanen = kabar baik: hijau, bukan biru. */
	.paramBadgePillPermanent {
		background: #ecfdf5;
		color: #047857;
	}
	.paramBadgePillGreen {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 4px 12px;
		border-radius: 6px;
		background: #ecfdf5;
		color: #047857;
		font-size: 12px;
		font-weight: 700;
	}
	.paramValueMuted {
		color: #64748b;
		font-weight: 600;
	}
	.paramValueBold {
		color: #0f172a;
		font-weight: 800;
	}

	/* Pembatasan Gatekeeper */
	.gatekeeperCard {
		border-radius: 14px;
		border: 1px solid #fecaca;
		background: #fff5f5;
		padding: 18px 20px;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}
	.gatekeeperHead {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	:global(.gatekeeperIcon) {
		color: #dc2626;
		flex-shrink: 0;
	}
	.gatekeeperTitle {
		font-size: 14px;
		font-weight: 700;
		color: #991b1b;
	}
	.gatekeeperDesc {
		font-size: 12px;
		line-height: 1.5;
		color: #dc2626;
		margin: 2px 0 4px 0;
	}
	.gatekeeperChips {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		margin-top: 4px;
	}
	.gatekeeperChip {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 12px;
		border-radius: 8px;
		background: #ffffff;
		border: 1px solid #fca5a5;
		color: #991b1b;
		font-size: 12px;
		font-weight: 700;
		box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03);
	}

	/* Footer Modal */
	.detailFooter {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		padding: 16px 32px;
		border-top: 1px solid #f1f5f9;
		background: #ffffff;
		flex-shrink: 0;
	}
	.detailFooterLeft {
		display: flex;
		align-items: center;
		gap: 14px;
	}
	.detailFooterCoinIcon {
		width: 42px;
		height: 42px;
		border-radius: 10px;
		background: #eff6ff;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}
	.detailFooterAmountCol {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.detailFooterLabel {
		font-size: 10.5px;
		font-weight: 800;
		letter-spacing: 0.08em;
		color: #64748b;
		text-transform: uppercase;
	}
	.detailFooterAmountGroup {
		display: flex;
		align-items: baseline;
		gap: 8px;
	}
	.detailFooterPriceBig {
		font-size: 24px;
		font-weight: 900;
		letter-spacing: -0.02em;
		color: #0f172a;
	}

	.detailCtaBtnBlue {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 12px 24px;
		border-radius: 12px;
		border: none;
		background: #2563eb;
		color: #ffffff;
		font-size: 14px;
		font-weight: 700;
		cursor: pointer;
		transition: background 0.15s ease;
		line-height: 1;
		touch-action: manipulation;
	}
	.detailCtaBtnBlue:hover {
		background: #1d4ed8;
	}
	.detailCtaBtnBlue:active {
		background: #1e40af;
	}

	/* Responsive per antislop-layoutmobile */
	@media (max-width: 768px) {
		/* Di HP modal jadi sheet penuh: menempel tepi kiri-kanan dan dasar
		   layar. Overlay yang diberi padding 16px membuat modal mengambang
		   sempit dengan dua lapis bingkai, padahal isinya sudah panjang. */
		.detailOverlay {
			padding: 0;
			align-items: flex-end;
		}
		.detailModalWrapper {
			max-height: 100dvh;
			gap: 0;
		}
		.detailModal {
			max-height: 96dvh;
			border-radius: 16px 16px 0 0;
			border-bottom: 0;
		}
		.detailHeader {
			padding: 16px 18px;
			flex-direction: column;
			align-items: flex-start;
			gap: 12px;
		}
		.detailHeaderActions {
			width: 100%;
			/* Bagikan dan Ekspor Info menempel di kiri sebagai satu kelompok,
			   tombol tutup didorong mentok ke kanan (margin-left: auto di
			   bawah). space-between menyebar ketiganya rata, sehingga tombol
			   tutup berhenti di tengah dan tidak lagi di pojok. */
			justify-content: flex-start;
		}
		.detailCloseBtn {
			margin-left: auto;
		}
		/* Deretan metrik jadi slider mendatar: empat metrik tidak cukup lebar
		   untuk satu baris di HP, dan membungkusnya membuat strip ini setinggi
		   85px sehingga isi utama terdorong ke bawah. Digeser mendatar dengan
		   scrollbar tipis sebagai penanda masih ada metrik lain di kanan. */
		.detailMetricsRow {
			padding: 12px 18px 8px;
			gap: 16px;
			flex-wrap: nowrap;
			overflow-x: auto;
			overflow-y: hidden;
			overscroll-behavior-x: contain;
			-webkit-overflow-scrolling: touch;
			scrollbar-width: thin;
			scrollbar-color: #cbd5e1 transparent;
		}
		.detailMetricsRow::-webkit-scrollbar {
			height: 6px;
		}
		.detailMetricsRow::-webkit-scrollbar-track {
			background: transparent;
		}
		.detailMetricsRow::-webkit-scrollbar-thumb {
			background: #cbd5e1;
			border-radius: 999px;
		}
		.detailMetricsRow::-webkit-scrollbar-thumb:hover {
			background: #94a3b8;
		}
		/* Tiap metrik selebar isinya sendiri, tidak boleh mengempis. */
		.detailMetricItem {
			flex: 0 0 auto;
		}
		/* Distribusi pindah ke kartu Parameter AI & Redaksi di HP: strip metrik
		   cukup 4 item dan tidak perlu digeser untuk melihatnya. */
		.detailMetricDistribution {
			display: none;
		}
		.paramRowDistribution {
			display: flex;
		}
		.detailBody {
			padding: 16px 18px;
		}
		.detailColumns {
			grid-template-columns: 1fr;
			gap: 16px;
		}
		.heroCardReview,
		.heroCardAuto {
			flex-direction: column;
			align-items: flex-start;
			padding: 18px 16px;
		}
		.heroIllustrationWrapper {
			align-self: center;
			width: 110px;
			height: 110px;
			margin-top: 8px;
		}
		.perfGrid {
			grid-template-columns: 1fr;
		}
		.detailFooter {
			flex-direction: column;
			align-items: stretch;
			padding: 16px 18px calc(16px + env(safe-area-inset-bottom, 0px));
			gap: 12px;
		}
		.detailCtaBtnBlue {
			width: 100%;
			min-height: 44px;
		}
	}
</style>
