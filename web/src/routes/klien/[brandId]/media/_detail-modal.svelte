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
		Calendar, ChartBar, Check, CircleCheck, Clock, Coins, Download, FileText,
		Settings, Server, Image as ImageIcon, Info, Link as LinkIcon, Share2,
		CircleAlert, X, CircleX, Tag
	} from '@lucide/svelte';
	import { apiRequest } from '$lib/auth';
	import { wordCountLabel, normalizeWordCountMode } from '$lib/media/word-count';
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
		status?: string | null;
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

	let crawlArticles = $state<number | null>(null);
	let copiedShare = $state(false);
	let catPopoverOpen = $state(false);
	let infoTipOpen = $state(false);

	// Content velocity dari crawlhub API
	onMount(() => {
		if (!brandId || !media.id) return;
		let cancelled = false;
		void (async () => {
			try {
				const res = await apiRequest(`/v1/brands/${brandId}/media/${media.id}/crawl-status`);
				if (!res.ok || cancelled) return;
				const data = (await res.json()) as { articles_7d?: number | null };
				if (!cancelled) crawlArticles = data.articles_7d ?? null;
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

	function exportInfo() {
		const lines = [
			`INFORMASI MEDIA: ${media.name.toUpperCase()}`,
			`Website: ${media.website_url ?? '-'}`,
			`Kategori: ${media.category ?? 'Umum'}`,
			`Dewan Pers: ${media.dewan_pers_verified ? 'Terverifikasi' : tr('Belum Terverifikasi')}`,
			'',
			'JALUR PUBLIKASI',
			autoPath
				? 'Tauthub Secure Direct™ — artikel terbit otomatis ke website media tanpa antre persetujuan mitra.'
				: 'Editorial Review (SLA Tracked) — naskah ditinjau dan diterbitkan langsung oleh tim redaksi mitra.',
			'',
			'INDIKATOR KINERJA MITRA (skala engine health Tauthub)',
			`Health Score: ${score != null ? `${score} / 100` : 'belum diskor'} (${healthLabel})`,
			`Fulfillment Rate: ${fulfillmentPoints} / ${METRIC_MAX.fulfillment} (${fulfillmentPct})`,
			`SLA Responsiveness: ${slaText === '—' ? 'belum ada riwayat' : slaText} — ${slaPoints} / ${METRIC_MAX.sla}`,
			`Link Permanence: ${permanencePoints} / ${METRIC_MAX.permanence} (${permanencePct})`,
			`Server Uptime: ${uptimePoints} / ${METRIC_MAX.uptime} (${uptimePct})`,
			hasHistory
				? `Riwayat: ${published} terbit, ${rejected} ditolak, ${failed} gagal`
				: 'Belum ada riwayat publikasi — seluruh komponen bernilai 0 sesuai perhitungan engine.',
			'',
			'PARAMETER AI & REDAKSI',
			`Garansi Tayang (SLA): 12 Bulan (jaminan retensi standar platform Tauthub)`,
			`Label Sponsor / Adv: ${media.label_sponsor === 'ada_sponsor' ? 'Ada Sponsor / Adv' : tr('Tidak Ada (Organik)')}`,
			`Batas Tautan Keluar: ${linkPolicy}`,
			`Panjang Artikel: ${wordCountLabel(media.word_count_mode)}`,
			`Support Embed (Video/Maps): ${media.allow_embed ? 'Diizinkan' : tr('Tidak Diizinkan')}`,
			`Mode Pengiriman: ${isDirectPublish ? tr('Langsung Terbit (otomatis, tanpa langkah terbit manual di media)') : tr('Draf (diterbitkan oleh media)')}`,
			'',
			'PEMBATASAN GATEKEEPER',
			`Topik Dilarang: ${gatekeeperList.join(', ')}`,
			'',
			`BIAYA PUBLIKASI: ${idr(media.client_price ?? media.price)} (~ ${media.credit_cost} Kredit)`
		];;
		const blob = new Blob([lines.join('\n')], { type: 'text/plain;charset=utf-8' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `Media_Info_${media.name.replace(/[^\w-]+/g, '_')}.txt`;
		a.click();
		URL.revokeObjectURL(url);
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
						{initialLetter}
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
														<span class="categoryPill">{c}</span>
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

					<button class="detailExportPillBtn" type="button" onclick={exportInfo}>
						<Download size={16} />
						<span>Ekspor Info</span>
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
								</div>
								<div class="velocityRight">
									{#if crawlArticles == null}
										<span class="velocityEmpty">{tr("Belum ada data crawl")}</span>
									{:else}
										<div class="velocityValue">
											{crawlArticles} <span class="velocityValueUnit">artikel</span>
										</div>
										{#if velocityBadge}
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
									<span class="paramBadgePill">12 Bulan · platform</span>
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
							</div>
						</div>

						<!-- Pembatasan Gatekeeper -->
						<div class="gatekeeperCard">
							<div class="gatekeeperHead">
								<CircleAlert size={20} class="gatekeeperIcon" />
								<span class="gatekeeperTitle">Pembatasan Gatekeeper</span>
							</div>

							<p class="gatekeeperDesc">{tr("Naskah yang mengandung topik berikut akan diblokir otomatis oleh AI Validator sebelum saldo kredit Anda terpotong:")}</p>

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
						<Coins size={26} color="#2563eb" />
					</div>
					<div class="detailFooterAmountCol">
						<span class="detailFooterLabel">BIAYA PUBLIKASI</span>
						<div class="detailFooterAmountGroup">
							<span class="detailFooterPriceBig">
								{idr(media.client_price ?? media.price)}
							</span>
							<span class="detailFooterCredits">
								~ {media.credit_cost.toLocaleString('id-ID')} {tr("Kredit")} </span>
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
		background: #ffffff;
		border: 1px solid #e2e8f0;
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 26px;
		font-weight: 800;
		color: #2563eb;
		flex-shrink: 0;
		box-shadow: 0 2px 6px rgba(0, 0, 0, 0.04);
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
		display: inline-block;
		padding: 2px 8px;
		border-radius: 6px;
		background: #f1f5f9;
		font-size: 11.5px;
		color: #334155;
	}

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
	.detailFooterCredits {
		font-size: 14px;
		font-weight: 700;
		color: #2563eb;
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
		.detailHeader {
			padding: 16px 18px;
			flex-direction: column;
			align-items: flex-start;
			gap: 12px;
		}
		.detailHeaderActions {
			width: 100%;
			justify-content: space-between;
		}
		.detailMetricsRow {
			padding: 12px 18px;
			gap: 16px;
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
			padding: 16px 18px;
			gap: 12px;
		}
		.detailCtaBtnBlue {
			width: 100%;
			min-height: 44px;
		}
	}
</style>
