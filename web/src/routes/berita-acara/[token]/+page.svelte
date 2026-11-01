<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Port app/berita-acara/[token]/page.tsx — dokumen berita acara kampanye
	// publik (tanpa login), diakses lewat token share. Data dari
	// /v1/campaign-reports/{token} (sumber sama dengan halaman lama).
	// Dokumen tampil sebagai kartu modal di atas backdrop dashboard palsu
	// (_backdrop-stage) — parity backdrop-stage.tsx app lama.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { rp } from '$lib/credits/rupiah';
	import { getI18n } from '$lib/i18n/i18n.svelte';
	import { sameOriginAssetUrl } from '$lib/media/cover';
	import { Image as ImageIcon } from '@lucide/svelte';
	import BackdropStage from './_backdrop-stage.svelte';

	const token = $derived(page.params.token as string);
	const i18n = getI18n();

	// Slug status mesin (waiting_partner, published, dst) tidak pernah tampil
	// apa adanya ke pembaca dokumen publik — lewat kamus status dulu.
	const statusLabel = (s: string) => (i18n.t.status as Record<string, string>)[s] ?? s;

	// Thumbnail persis seperti halaman Publikasi klien: URL /uploads/ absolut
	// dibalik ke origin yang sedang dibuka; gagal muat → tile abu dengan ikon.
	let imgError = $state<Record<string, boolean>>({});
	const thumbUrl = (u: unknown) => sameOriginAssetUrl(typeof u === 'string' ? u : null);

	let loading = $state(true);
	let report = $state<any>(null);
	let error = $state('');

	onMount(() => {
		void (async () => {
			const res = await fetch(`/v1/campaign-reports/${encodeURIComponent(token)}`, { cache: 'no-store' });
			if (res.ok) {
				report = await res.json();
			} else {
				error = tr('Berita acara tidak ditemukan atau tautannya sudah dicabut.');
			}
			loading = false;
		})();
	});

	const fmtDate = (v: unknown) =>
		v ? new Date(v as string).toLocaleDateString('id-ID', { dateStyle: 'long' }) : '—';
	const fmtDateTime = (v: unknown) =>
		v ? new Date(v as string).toLocaleString('id-ID', { dateStyle: 'long', timeStyle: 'short' }) : '—';

	const campaignTone = (status: string) =>
		status === 'active' ? 'var(--success)' : status === 'completed' ? 'var(--primary)' : 'var(--destructive)';
	const pubTone = (status: string) =>
		status === 'published' || status === 'completed'
			? 'var(--success)'
			: status === 'failed' || status === 'rejected'
				? 'var(--destructive)'
				: 'var(--warning)';

	function print() {
		window.print();
	}
</script>

<svelte:head>
	<title>{tr("Berita Acara Kampanye")}</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<BackdropStage>
	<div class="docWrap">
		{#if loading}
			<div class="app-card docLoading">{tr("Memuat berita acara…")}</div>
		{:else if error || !report}
			<div class="app-card docError">
				<h1 class="text-xl font-semibold">Tautan tidak tersedia</h1>
				<p class="mt-2 text-sm text-muted-foreground">{tr("Berita acara ini sudah dicabut atau tautannya salah. Minta tautan terbaru kepada pemilik kampanye.")}</p>
			</div>
		{:else}
			<article class="docCard">
				<div class="docTopRow">
					<img
						src="/tauthub-logo.webp"
						alt="Tauthub"
						width="140"
						height="35"
						class="docLogo"
					/>
					<button class="app-btn app-btn-soft docPrintBtn" type="button" onclick={print}>{tr("Cetak / Simpan PDF")}</button>
				</div>

				<div class="docDivider"></div>

				<header>
					<p class="docBrand">{report.brand_name}</p>
					<h1 class="docTitle">{report.campaign.name}</h1>
					<div class="docMetaRow">
						<span
							class="docBadge"
							style="background: color-mix(in srgb, {campaignTone(report.campaign.status)} 12%, transparent); color: {campaignTone(report.campaign.status)};"
						>{statusLabel(report.campaign.status)}</span>
						<span class="docMetaText">{tr("· Dibuat")} {fmtDate(report.campaign.created_at)}</span>
					</div>
					{#if report.campaign.description}
						<p class="docDesc">{report.campaign.description}</p>
					{/if}
				</header>

				<div class="docStats">
					<div>
						<span class="docStatLabel">Artikel dibuat</span>
						<span class="docStatValue">{report.summary.articles_created}</span>
					</div>
					<div>
						<span class="docStatLabel">Artikel terbit</span>
						<span class="docStatValue">{report.summary.published_count}</span>
					</div>
					<div>
						<span class="docStatLabel">{tr("Media terbit")}</span>
						<span class="docStatValue">{report.summary.media_published_count}</span>
					</div>
					<div>
						<span class="docStatLabel">Saldo terpakai</span>
						<span class="docStatValue">{rp(report.summary.tc_used)}</span>
					</div>
				</div>

				<section class="docSection">
					<h2 class="docSectionTitle">{tr("Media yang menerbitkan")}</h2>
					{#if report.summary.published_media_names}
						<p class="docSectionBody">{report.summary.published_media_names.split(' · ').join(', ')}</p>
					{:else}
						<p class="docSectionBody">{tr("Belum ada artikel yang terbit.")}</p>
					{/if}
				</section>

				<section class="docSection docPubSection">
					<h2 class="docSectionTitle">{tr("Daftar publikasi")}</h2>
					{#if report.publications.length === 0}
						<p class="docSectionBody">{tr("Belum ada publikasi dalam kampanye ini.")}</p>
					{:else}
						<ul class="docPubs">
							{#each report.publications as pub (pub.id)}
								<li class="docPub">
									<div class="docPubMain">
										{#if thumbUrl(pub.featured_image_url) && !imgError[pub.id]}
											<img
												class="docPubThumb"
												src={thumbUrl(pub.featured_image_url)}
												alt={pub.document_title ?? 'Cover artikel'}
												loading="lazy"
												onerror={() => (imgError = { ...imgError, [pub.id]: true })}
											/>
										{:else}
											<span class="docPubThumb docPubThumbEmpty" title={pub.document_title ?? undefined}>
												<ImageIcon size={22} />
											</span>
										{/if}
										<div class="docPubContent">
											<p class="docPubTitle">
												{pub.document_title ?? 'Artikel'} → {pub.media_name ?? tr('Media')}
											</p>
											<p class="docPubMeta">
												{pub.published_at
													? `Terbit ${fmtDateTime(pub.published_at)}`
													: `Dibuat ${fmtDate(report.campaign.created_at)}`}{pub.media_category ? ` · ${pub.media_category}` : ''}
											</p>
											<div class="docPubActions">
												{#if pub.published_url}
													<a class="docPubLink" href={pub.published_url} target="_blank" rel="noreferrer">{tr("Lihat terbit")}</a>
												{/if}
												<span
													class="docBadge"
													style="background: color-mix(in srgb, {pubTone(pub.status)} 12%, transparent); color: {pubTone(pub.status)};"
												>{statusLabel(pub.status)}</span>
											</div>
										</div>
									</div>
								</li>
							{/each}
						</ul>
					{/if}
				</section>

				<p class="docFootnote">{tr("Dokumen berita acara digenerate otomatis dari data kampanye Tauthub pada")} {fmtDateTime(report.generated_at)} WIB.
					Saldo terpakai mencakup biaya pembuatan dan distribusi konten kampanye ini (rupiah).
				</p>
			</article>
		{/if}
	</div>
</BackdropStage>

<style>
	.docWrap {
		width: 100%;
		max-width: 768px;
		/* Di atas dimmer (z1) supaya kartu tidak ikut blur/tergelapkan. */
		position: relative;
		z-index: 2;
	}
	.docLoading {
		padding: 20px;
		text-align: center;
		font-size: 13.5px;
		color: var(--muted-foreground);
	}
	.docError {
		padding: 32px;
		text-align: center;
	}
	/* Kartu dokumen: satu-satunya elemen terangkat di atas backdrop.
	   Kolom flex dengan max-height: saat konten melebihi viewport, hanya
	   daftar publikasi yang mengecil dan menggulir — sisanya tetap utuh. */
	.docCard {
		display: flex;
		flex-direction: column;
		width: 100%;
		max-height: calc(100dvh - 72px);
		padding: 28px 30px;
		border: 1px solid var(--border);
		border-radius: 16px;
		background: #ffffff;
		box-shadow: 0 24px 64px rgba(15, 23, 42, 0.3);
		animation: docIn 0.18s cubic-bezier(0.16, 1, 0.3, 1);
	}
	.docCard > * {
		flex-shrink: 0;
	}
	@keyframes docIn {
		from { opacity: 0; transform: translateY(8px) scale(0.99); }
		to { opacity: 1; transform: none; }
	}
	@media (prefers-reduced-motion: reduce) {
		.docCard { animation: none; }
	}
	.docTopRow {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}
	.docLogo {
		display: block;
		width: auto;
		height: 28px;
		object-fit: contain;
	}
	.docPrintBtn {
		min-height: 36px;
		padding: 0 14px;
		font-size: 12.5px;
	}
	.docDivider {
		height: 1px;
		margin-top: 18px;
		background: var(--border);
	}
	.docBrand {
		margin-top: 20px;
		font-size: 11.5px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--muted-foreground);
	}
	.docTitle {
		margin-top: 4px;
		font-size: 26px;
		font-weight: 750;
		letter-spacing: -0.015em;
		color: var(--foreground);
		word-break: break-word;
	}
	.docMetaRow {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 8px;
		margin-top: 10px;
	}
	.docBadge {
		display: inline-flex;
		align-items: center;
		border-radius: 999px;
		padding: 4px 12px;
		font-size: 11.5px;
		font-weight: 650;
		line-height: 1.4;
		white-space: nowrap;
	}
	.docMetaText {
		font-size: 12.5px;
		color: var(--muted-foreground);
	}
	.docDesc {
		margin-top: 12px;
		font-size: 13.5px;
		line-height: 1.55;
		color: var(--muted-foreground);
	}
	/* Statistik satu panel biru lembut dengan pemisah vertikal — menggantikan
	   empat kartu kembar. Angka dari data laporan yang sama. */
	.docStats {
		display: flex;
		margin-top: 20px;
		padding: 18px 22px;
		background: #f1f6fd;
		border-radius: 12px;
	}
	.docStats > div {
		flex: 1;
		min-width: 0;
	}
	.docStats > div:first-child {
		padding-right: 6px;
	}
	.docStats > div + div {
		border-left: 1px solid color-mix(in srgb, var(--primary) 14%, transparent);
		padding-left: 20px;
	}
	.docStatLabel {
		display: block;
		font-size: 10.5px;
		font-weight: 700;
		letter-spacing: 0.07em;
		text-transform: uppercase;
		color: var(--muted-foreground);
	}
	.docStatValue {
		display: block;
		margin-top: 5px;
		font-size: 25px;
		font-weight: 750;
		color: var(--foreground);
		font-variant-numeric: tabular-nums;
	}
	.docSection {
		margin-top: 24px;
	}
	.docSectionTitle {
		font-size: 16px;
		font-weight: 700;
		color: var(--foreground);
	}
	.docSectionBody {
		margin-top: 6px;
		font-size: 13.5px;
		color: var(--muted-foreground);
	}
	/* Daftar publikasi mengisi sisa tinggi kartu dan menggulir di situ saja. */
	.docPubSection {
		flex: 0 1 auto;
		min-height: 0;
		display: flex;
		flex-direction: column;
	}
	.docPubSection :global(h2.docSectionTitle) {
		flex-shrink: 0;
	}
	/* Daftar panjang digulir di dalam kartu, bukan memanjangkan dokumen.
	   Scrollbar tipis mengikuti pola .app-sidebar-nav di app. */
	.docPubs {
		flex: 1 1 auto;
		min-height: 0;
		margin-top: 4px;
		overflow-y: auto;
		padding-right: 4px;
		scrollbar-width: thin;
		scrollbar-color: color-mix(in srgb, var(--foreground) 22%, transparent) transparent;
	}
	.docPubs::-webkit-scrollbar {
		width: 8px;
	}
	.docPubs::-webkit-scrollbar-track {
		background: transparent;
	}
	.docPubs::-webkit-scrollbar-thumb {
		border-radius: 999px;
		background: color-mix(in srgb, var(--foreground) 20%, transparent);
	}
	.docPubs::-webkit-scrollbar-thumb:hover {
		background: color-mix(in srgb, var(--foreground) 36%, transparent);
	}
	.docPub {
		padding: 13px 0;
	}
	.docPubMain {
		display: flex;
		align-items: flex-start;
		gap: 14px;
	}
	.docPubThumb {
		width: 48px;
		height: 48px;
		flex: 0 0 auto;
		border-radius: 10px;
		object-fit: cover;
		border: 1px solid var(--border);
		background: var(--muted);
	}
	.docPubThumbEmpty {
		display: grid;
		place-items: center;
		color: var(--muted-foreground);
		background: var(--muted);
	}
	.docPubContent {
		min-width: 0;
	}
	.docPub + .docPub {
		border-top: 1px solid var(--border);
	}
	.docPubTitle {
		font-size: 14.5px;
		font-weight: 650;
		line-height: 1.45;
		color: var(--foreground);
	}
	.docPubMeta {
		margin-top: 3px;
		font-size: 12.5px;
		color: var(--muted-foreground);
	}
	.docPubActions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 10px;
		margin-top: 8px;
	}
	.docPubLink {
		font-size: 12.5px;
		font-weight: 600;
		color: var(--primary);
		text-decoration: none;
	}
	.docPubLink:hover {
		text-decoration: underline;
	}
	.docFootnote {
		margin-top: 24px;
		padding-top: 16px;
		border-top: 1px solid var(--border);
		font-size: 12px;
		line-height: 1.55;
		color: var(--muted-foreground);
	}
	@media (max-width: 640px) {
		.docCard {
			max-height: calc(100dvh - 56px);
			padding: 18px 16px;
			border-radius: 14px;
		}
		.docTitle {
			font-size: 22px;
		}
		.docStats {
			display: grid;
			grid-template-columns: 1fr 1fr;
			gap: 14px;
			padding: 16px;
		}
		.docStats > div:first-child {
			padding-right: 0;
		}
		.docStats > div + div {
			border-left: none;
			padding-left: 0;
		}
		.docStatValue {
			font-size: 21px;
		}
	}
	@media print {
		.docCard {
			max-height: none;
			display: block;
			box-shadow: none;
			animation: none;
		}
		/* Cetak/PDF harus mendapat seluruh daftar, bukan potongan viewport. */
		.docPubSection {
			display: block;
		}
		.docPubs {
			overflow: visible;
			padding-right: 0;
		}
	}
</style>
