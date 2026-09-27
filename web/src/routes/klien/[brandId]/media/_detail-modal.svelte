<script lang="ts">
	// Modal detail media — port media-detail-modal.tsx.
	// Layout: header (avatar + badge + aksi) → strip metrik → 2 kolom
	// (jalur publikasi + kinerja | parameter AI & gatekeeper) → footer biaya.
	import { X, ExternalLink, FileText, Check, Zap, Clock, Info, ArrowRight, Bot, Coins } from '@lucide/svelte';

	type Media = {
		id: string; name: string; category: string | null; categories?: string[];
		description: string | null; website_url: string | null; contoh_artikel_url?: string | null;
		bahasa_artikel?: string[] | null; price: number; client_price: number; credit_cost: number;
		integration_type: string; traffic_estimate: number; media_tier: string | null;
		media_quality_score: number | null; effective_quality: number | null;
		quality_source: 'media' | 'tier_default' | null; health_score: number | null;
		health_breakdown: Record<string, unknown> | null; health_computed_at?: string | null;
		dewan_pers_verified?: boolean | null; domain_verified?: boolean | null;
		seo_da?: number | null; seo_pa?: number | null; seo_spam_score?: number | null;
		seo_checked_at?: string | null; seo_ai_trust?: number | null; seo_ai_citation?: boolean | null;
		sw_visits?: number | null; sw_snapshot_date?: string | null; gsc_connected?: boolean;
		image_required?: boolean | null; label_sponsor?: string | null; topik_terlarang?: string[];
		max_links?: number | null; allow_hyperlink?: boolean | null;
		publication_mode?: string | null; word_count_mode?: string | null; allow_embed?: boolean;
	};

	let { media, onclose }: { media: Media; onclose: () => void } = $props();

	// INTEGRATION_INFO — salinan persis media-detail-modal.tsx.
	const INTEGRATION_INFO: Record<string, { label: string; auto: boolean; tooltip: string }> = {
		manual: { label: 'Posting Manual', auto: false, tooltip: 'Diterbitkan langsung oleh tim media secara manual (di luar sistem Tauthub), menunggu proses manusia, umumnya 2-3 hari kerja.' },
		wordpress: { label: 'Posting Otomatis · WordPress', auto: true, tooltip: 'Artikel dikirim otomatis (push) ke WordPress media begitu antrean distribusi Tauthub memprosesnya, bisa terbit di hari yang sama.' },
		blogger: { label: 'Posting Otomatis · Blogger', auto: true, tooltip: 'Artikel dikirim otomatis (push) ke blog Blogger media begitu antrean distribusi Tauthub memprosesnya, bisa terbit di hari yang sama.' },
		tulisio_api: { label: 'Posting Otomatis · Tauthub API', auto: true, tooltip: 'Artikel dikirim otomatis (push) ke website custom media melalui sistem distribusi Tauthub, bisa terbit di hari yang sama.' },
		custom_web: { label: 'Posting Otomatis · Custom Web', auto: true, tooltip: 'Artikel dikirim otomatis (push) ke website custom media melalui sistem distribusi Tauthub.' },
		rss: { label: 'Posting Otomatis · RSS', auto: true, tooltip: 'Artikel muncul di feed RSS Tauthub milik media. Website media menarik (pull) sesuai jadwalnya sendiri.' },
		webhook: { label: 'Posting Otomatis · Webhook', auto: true, tooltip: 'Artikel dikirim otomatis ke website media sesuai pengaturan koneksi milik media.' }
	};
	const INTEGRATION_FALLBACK = { label: 'Posting Manual', auto: false, tooltip: 'Metode penerbitan belum dikenal, dianggap manual oleh Tauthub.' };
	const integration = $derived(INTEGRATION_INFO[media.integration_type] ?? INTEGRATION_FALLBACK);
	const tierLabel = (tier: string | null) => (tier ? `Tier ${tier.replace(/^tier/i, '').trim() || '—'}` : '—');

	const fmt = (n: number | null | undefined) => (n ?? 0).toLocaleString('id-ID');

	/** "Hari ini, 08:30 WIB" / "Kemarin, …" / "17 Sep 2026, 08:30 WIB" (Asia/Jakarta). */
	function jakartaStamp(iso: string | null | undefined): string {
		if (!iso) return '—';
		const at = new Date(iso);
		if (Number.isNaN(at.getTime())) return '—';
		const time = at.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Jakarta' }).replace(':', '.');
		const today = new Date();
		const sameDay = (a: Date, b: Date) =>
			a.toLocaleDateString('id-ID', { timeZone: 'Asia/Jakarta' }) === b.toLocaleDateString('id-ID', { timeZone: 'Asia/Jakarta' });
		const yesterday = new Date(today.getTime() - 86_400_000);
		if (sameDay(at, today)) return `Hari ini, ${time} WIB`;
		if (sameDay(at, yesterday)) return `Kemarin, ${time} WIB`;
		const date = at.toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'Asia/Jakarta' });
		return `${date}, ${time} WIB`;
	}

	function trafficText(): { text: string; source: string } {
		const val = media.sw_visits && media.sw_visits > 0 ? media.sw_visits : media.traffic_estimate > 0 ? media.traffic_estimate : 0;
		if (!val) return { text: 'Estimasi', source: 'Via Estimasi Media' };
		if (val >= 1_000_000) return { text: `± ${(val / 1_000_000).toFixed(1).replace(/\.0$/, '')}M / bulan`, source: 'Via Similarweb' };
		if (val >= 1_000) return { text: `± ${Math.round(val / 1_000)} rb / bulan`, source: 'Via Similarweb' };
		return { text: `± ${val.toLocaleString('id-ID')} / bulan`, source: 'Via Similarweb' };
	}
	const traffic = $derived(trafficText());

	const da = $derived(media.seo_da != null && media.seo_da > 0 ? media.seo_da : '—');
	const aiTrust = $derived(
		media.seo_ai_trust != null && media.seo_ai_trust > 0
			? media.seo_ai_trust
			: media.effective_quality
				? Math.max(1, Math.round(media.effective_quality * 0.9))
				: '—'
	);

	const breakdown = $derived((media.health_breakdown ?? {}) as Record<string, number | null>);
	const perfCards = $derived([
		{ key: 'availability', label: 'Ketersediaan', value: breakdown.availability },
		{ key: 'success_rate', label: 'Tingkat Sukses', value: breakdown.success_rate },
		{ key: 'publish_time', label: 'Waktu Terbit', value: breakdown.publish_time },
		{ key: 'rejection_rate', label: 'Tingkat Penolakan', value: breakdown.rejection_rate }
	]);

	const topik = $derived(media.topik_terlarang ?? []);
	const bahasa = $derived(media.bahasa_artikel ?? []);
	const par = $derived(`Rp ${fmt(media.client_price ?? media.price)}`);

	function copyLink() {
		if (media.website_url) void navigator.clipboard?.writeText(media.website_url);
	}
</script>

<div class="modalOverlay" role="presentation" onclick={onclose}>
	<dialog
		open
		class="detailCard"
		aria-label={`Detail media ${media.name}`}
		onclick={(e) => e.stopPropagation()}
		onkeydown={(e) => { if (e.key === 'Escape') onclose(); }}
	>
		<!-- Header -->
		<div class="detailHeader">
			<div class="detailIdentity">
				<span class="detailAvatar">{media.name.slice(0, 1).toUpperCase()}</span>
				<div>
					<div class="detailNameLine">
						<h2 class="detailName">{media.name}</h2>
						{#if media.domain_verified}<span class="verifiedTick" title="Domain terverifikasi"><Check size={14} /></span>{/if}
					</div>
					<div class="detailMetaLine">
						{#if media.website_url}
							<a class="domainLink" href={media.website_url} target="_blank" rel="noreferrer" onclick={(e) => e.stopPropagation()}>
								{media.website_url.replace(/^https?:\/\//, '')} <ExternalLink size={12} />
							</a>
						{/if}
						<span class="detailTier">{tierLabel(media.media_tier)}</span>
						{#if media.dewan_pers_verified}<span class="dewanPersBadge"><Check size={12} />Dewan Pers</span>{/if}
					</div>
				</div>
			</div>
			<div class="detailHeaderActions">
				<button class="advFilterResetBtn" type="button" onclick={copyLink}>Bagikan</button>
				<button class="advFilterResetBtn" type="button" onclick={() => window.print()}>
					<FileText size={14} /> Ekspor Info
				</button>
				<button class="modalClose" type="button" aria-label="Tutup detail" onclick={onclose}><X size={18} /></button>
			</div>
		</div>

		<!-- Strip metrik -->
		<div class="detailMetrics">
			<div class="detailMetric">
				<span class="detailMetricLabel">TRAFIK</span>
				<span class="detailMetricValue">{traffic.text}</span>
				<span class="detailMetricSub">{traffic.source}</span>
			</div>
			<div class="detailMetric">
				<span class="detailMetricLabel">DOMAIN AUTH</span>
				<span class="detailMetricValue">{da}</span>
				<span class="detailMetricSub">DA {media.seo_da ?? '—'} · PA {media.seo_pa ?? '—'}</span>
			</div>
			<div class="detailMetric">
				<span class="detailMetricLabel">AI TRUST</span>
				<span class="detailMetricValue">{aiTrust}</span>
				<span class="detailMetricSub">{media.seo_ai_citation ? 'Terdeteksi sumber AI' : 'Belum terdeteksi AI'}</span>
			</div>
		</div>

		<div class="detailGrid">
			<!-- Kolom kiri -->
			<div class="detailCol">
				<div class="detailHero">
					<span class="detailHeroEyebrow">JALUR PUBLIKASI</span>
					<p class="detailHeroTitle">{integration.label}</p>
					<p class="detailHeroDesc">{integration.tooltip}</p>
					<div class="detailHeroChips">
						<span class="detailHeroChip">
							{#if integration.auto}<Zap size={12} />{:else}<Clock size={12} />{/if}
							{integration.auto ? 'Otomatis' : 'Manual'}
						</span>
						{#if media.gsc_connected}<span class="detailHeroChip">GSC terhubung</span>{/if}
					</div>
				</div>

				<div class="detailPanel">
					<div class="detailPanelHead">
						<span class="detailPanelTitle">Indikator Kinerja</span>
						<span class="detailPanelSub">Diperbarui {jakartaStamp(media.health_computed_at)}</span>
					</div>
					<div class="perfGrid">
						{#each perfCards as c (c.key)}
							<div class="perfCard">
								<span class="perfLabel">{c.label}</span>
								<span class="perfValue">{c.value != null ? `${c.value}%` : '—'}</span>
							</div>
						{/each}
					</div>
					<div class="detailPanelFoot">
						<span>Skor kinerja keseluruhan</span>
						<strong>{media.health_score != null ? `${media.health_score}/100` : '—'}</strong>
					</div>
				</div>
			</div>

			<!-- Kolom kanan -->
			<div class="detailCol">
				<div class="detailPanel">
					<div class="detailPanelHead"><span class="detailPanelTitle">Parameter AI &amp; Redaksi</span></div>
					<dl class="paramList">
						<div class="paramRow"><dt>Kategori</dt><dd>{media.category ?? 'Umum'}</dd></div>
						<div class="paramRow"><dt>Kualitas efektif</dt><dd>{media.effective_quality ?? '—'}{#if media.quality_source === 'tier_default'} <span class="paramNote">(baseline tier)</span>{/if}</dd></div>
						<div class="paramRow"><dt>Bahasa artikel</dt><dd>{bahasa.length ? bahasa.join(', ') : '—'}</dd></div>
						<div class="paramRow"><dt>Panjang artikel</dt><dd>{media.word_count_mode ?? '—'}</dd></div>
						<div class="paramRow"><dt>Maks. tautan</dt><dd>{media.allow_hyperlink === false ? 'Tidak menerima tautan' : (media.max_links ?? '—')}</dd></div>
						<div class="paramRow"><dt>Label sponsor</dt><dd>{media.label_sponsor === 'ada_sponsor' ? 'Bersponsor (rel=sponsored)' : 'Tanpa sponsor'}</dd></div>
						<div class="paramRow"><dt>Embed video/peta</dt><dd>{media.allow_embed ? 'Diizinkan' : 'Tidak diizinkan'}</dd></div>
						<div class="paramRow"><dt>Mode publikasi</dt><dd>{media.publication_mode ?? '—'}</dd></div>
					</dl>
				</div>

				<div class="detailPanel">
					<div class="detailPanelHead">
						<span class="detailPanelTitle">Pembatasan Gatekeeper</span>
						<span class="infoDot" title="Batasan ini diterapkan otomatis saat artikel dihasilkan dan diperiksa Gatekeeper."><Info size={14} /></span>
					</div>
					{#if topik.length}
						<div class="topikWrap">
							{#each topik as t (t)}<span class="topikChip">{t}</span>{/each}
						</div>
					{:else}
						<p class="detailPanelEmpty">Tidak ada topik terlarang untuk media ini.</p>
					{/if}
					{#if media.image_required}
						<p class="gatekeeperNote">Wajib menyertakan gambar/visual header pada artikel.</p>
					{/if}
				</div>

				{#if media.description}
					<div class="detailPanel">
						<div class="detailPanelHead"><span class="detailPanelTitle">Tentang Media</span></div>
						<p class="detailDesc">{media.description}</p>
					</div>
				{/if}
			</div>
		</div>

		<!-- Footer -->
		<div class="detailFooter">
			<div class="detailCost">
				<span class="detailCostIcon"><Coins size={16} /></span>
				<div>
					<span class="detailCostLabel">BIAYA PUBLIKASI</span>
					<div class="detailCostValues">
						<strong class="detailCostPrice">{par}</strong>
						<span class="detailCostCredit">≈ {fmt(media.credit_cost)} Kredit</span>
					</div>
				</div>
			</div>
			<div class="detailFooterActions">
				{#if media.contoh_artikel_url}
					<a class="advFilterResetBtn" href={media.contoh_artikel_url} target="_blank" rel="noreferrer nofollow">
						<FileText size={14} /> Lihat Contoh Artikel
					</a>
				{/if}
				<button class="selectedCreateBtn" type="button" onclick={onclose}>
					<span>Pilih Media Ini</span><ArrowRight size={14} />
				</button>
			</div>
		</div>

		{#if media.seo_ai_citation}
			<p class="detailAiNote"><Bot size={13} /> Media ini terdeteksi sebagai rujukan mesin AI.</p>
		{/if}
	</dialog>
</div>
