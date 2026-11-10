<script lang="ts">
	import { safeHref } from '$lib/safe-url';
	import { page } from '$app/state';
	import { env } from '$env/dynamic/public';
	import Tooltip from '$lib/components/tooltip.svelte';
	import SiteShell from '$lib/components/public/site-shell.svelte';
	import MediaCard from '$lib/components/public/media-card.svelte';
	import { idr, linkText, tierLabel, trafficText, dateText, categorySlug } from '$lib/public/format';
	import { wordCountLabel } from '$lib/media/word-count';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const APP_ORIGIN = (env.PUBLIC_APP_ORIGIN ?? 'https://app.tauthub.com').replace(/\/+$/, '');
	const host = $derived(page.url.hostname.toLowerCase());
	const loginHref = $derived(host.startsWith('data.') ? `${APP_ORIGIN}/login` : '/login');

	const m = $derived(data.media);
	const primaryCategory = $derived(m.categories[0] ?? null);
	const traffic = $derived(trafficText(m.traffic));
</script>

<SiteShell>
	<main id="main" class="pub-main">
		<div class="pub-container">
			<nav class="pub-breadcrumb" aria-label="Breadcrumb">
				<ol>
					<li><a href="/">Katalog media</a></li>
					{#if primaryCategory}
						<li><a href={`/kategori/${categorySlug(primaryCategory)}`}>{primaryCategory}</a></li>
					{/if}
					<li aria-current="page">{m.name}</li>
				</ol>
			</nav>

			<article class="pub-detail" aria-labelledby="media-title">
				<div>
					<section class="pub-panel">
						<h1 id="media-title">{m.name}</h1>
						{#if m.domain}
							<div class="pub-detail-domain">
								{#if m.website_url}
									<a href={safeHref(m.website_url)} target="_blank" rel="nofollow noopener noreferrer">{m.domain}</a>
								{:else}
									{m.domain}
								{/if}
							</div>
						{/if}

						<div class="pub-badges">
							<span class="pub-badge">{tierLabel(m.tier)}</span>
							{#if m.trust.dewan_pers_verified}
								<Tooltip content="Media ini terdaftar sebagai perusahaan pers di Dewan Pers." side="bottom">
									<button type="button" class="pub-badge ok">Terverifikasi Dewan Pers</button>
								</Tooltip>
							{/if}
							{#if m.trust.domain_verified}
								<Tooltip content="Domain media ini sudah dipastikan milik mitra penerbit, bukan situs tiruan." side="bottom">
									<button type="button" class="pub-badge ok">Domain terverifikasi</button>
								</Tooltip>
							{/if}
							{#if (m.trust.rating_avg ?? 0) > 0}
								<span class="pub-badge">Rating {m.trust.rating_avg?.toFixed(1)}/5 ({m.trust.rating_count})</span>
							{/if}
						</div>

						<dl class="pub-metric-grid">
							<div class="pub-metric"><dt>Domain Authority</dt><dd>{m.seo.da ?? '-'}</dd></div>
							<div class="pub-metric"><dt>Page Authority</dt><dd>{m.seo.pa ?? '-'}</dd></div>
							<div class="pub-metric"><dt>Estimasi trafik</dt><dd>{traffic}</dd></div>
							<div class="pub-metric">
								<dt>{m.price.type === 'nego' ? 'Harga acuan' : 'Harga publikasi'}</dt>
								<dd>{idr(m.price.amount)}</dd>
							</div>
						</dl>
					</section>

					{#if m.description}
						<section class="pub-panel">
							<h2>Tentang {m.name}</h2>
							<p>{m.description}</p>
						</section>
					{/if}

					{#if m.about}
						<section class="pub-panel">
							<h2>Ringkasan situs</h2>
							<p class="pub-about">{m.about}</p>
						</section>
					{/if}

					<section class="pub-panel">
						<h2>Ketentuan publikasi</h2>
						<dl class="pub-spec">
							<div>
								<dt>Kebijakan tautan</dt>
								<dd>{linkText(m.link_policy)}</dd>
							</div>
							<div>
								<dt>Panjang artikel</dt>
								<dd>{wordCountLabel(m.content.word_count_mode)}</dd>
							</div>
							<div>
								<dt>Gambar wajib</dt>
								<dd>{m.content.image_required ? 'Ya' : 'Tidak wajib'}</dd>
							</div>
							<div>
								<dt>Embed media</dt>
								<dd>{m.content.allow_embed ? 'Diizinkan' : 'Tidak diizinkan'}</dd>
							</div>
							<div>
								<dt>Label sponsor</dt>
								<dd>{m.content.sponsor_label === 'ada_sponsor' ? 'Wajib mencantumkan' : 'Tidak wajib'}</dd>
							</div>
							<div>
								<dt>Jumlah artikel terbit</dt>
								<dd>{m.trust.published_count}</dd>
							</div>
							{#if m.standards}
								{#if m.standards.min_words && m.standards.max_words}
									<div>
										<dt>Rentang kata</dt>
										<dd>{m.standards.min_words}–{m.standards.max_words} kata</dd>
									</div>
								{/if}
								{#if m.standards.journalistic_style}
									<div>
										<dt>Gaya penulisan</dt>
										<dd>{m.standards.journalistic_style}</dd>
									</div>
								{/if}
								{#if m.standards.quote_required}
									<div>
										<dt>Kutipan</dt>
										<dd>Wajib{m.standards.min_quotes > 0 ? ` (minimal ${m.standards.min_quotes})` : ''}</dd>
									</div>
								{/if}
								{#if m.standards.category_required}
									<div>
										<dt>Kategori artikel</dt>
										<dd>Wajib diisi</dd>
									</div>
								{/if}
							{/if}
						</dl>
						{#if m.updated_at}
							<p style="margin-top: 14px; font-size: 12px; color: var(--muted-foreground, #667085);">
								Data diperbarui {dateText(m.updated_at)}.
							</p>
						{/if}
					</section>

					{#if m.categories.length > 1}
						<section class="pub-panel">
							<h2>Kategori</h2>
							<div class="pub-tags">
								{#each m.categories as cat (cat)}
									<a class="pub-tag" href={`/kategori/${categorySlug(cat)}`}>{cat}</a>
								{/each}
							</div>
						</section>
					{/if}
				</div>

				<aside class="pub-aside">
					<div class="pub-cta-card">
						<div class="pub-cta-price">{idr(m.price.amount)}</div>
						<div class="pub-cta-note">
							{m.price.type === 'nego'
								? 'Harga acuan. Bisa dinegosiasikan lewat dashboard Tauthub.'
								: 'Harga publikasi per artikel, sudah termasuk pajak sesuai ketentuan.'}
						</div>
						<a class="pub-btn pub-btn-primary" href={loginHref}>Pesan lewat Tauthub</a>
						{#if m.example_article_url}
							<a
								href={m.example_article_url}
								class="pub-btn pub-btn-ghost"
								target="_blank"
								rel="nofollow noopener noreferrer">Lihat contoh artikel</a
							>
						{/if}
						{#if m.website_url}
							<a
								href={safeHref(m.website_url)}
								class="pub-btn pub-btn-ghost"
								target="_blank"
								rel="nofollow noopener noreferrer">Kunjungi {m.domain || 'situs media'}</a
							>
						{/if}
					</div>
				</aside>
			</article>

			{#if data.related.length}
				<section class="pub-related" aria-labelledby="related-title">
					<header class="pub-section-head">
						<h2 id="related-title">Media lain di kategori {primaryCategory ?? 'ini'}</h2>
						{#if primaryCategory}
							<p><a href={`/kategori/${categorySlug(primaryCategory)}`}>Lihat semua →</a></p>
						{/if}
					</header>
					<ul class="pub-grid">
						{#each data.related as media (media.slug)}
							<li><MediaCard {media} /></li>
						{/each}
					</ul>
				</section>
			{/if}
		</div>
	</main>
</SiteShell>
