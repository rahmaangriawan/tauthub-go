<script lang="ts">
	import { ChartColumn, ShieldCheck, Wallet } from '@lucide/svelte';
	import SiteShell from '$lib/components/public/site-shell.svelte';
	import CatalogFilters from '$lib/components/public/catalog-filters.svelte';
	import HeroMediaPreview from '$lib/components/public/hero-media-preview.svelte';
	import MediaTable from '$lib/components/public/media-table.svelte';
	import Pagination from '$lib/components/public/pagination.svelte';
	import { number } from '$lib/public/format';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const activeCategory = $derived(data.filters.kategori);

	function hrefFor(p: number): string {
		const qs = new URLSearchParams(data.query);
		if (p > 1) qs.set('page', String(p));
		const s = qs.toString();
		return s ? `/?${s}` : '/';
	}

	/**
	 * Tautan judul kolom tabel. Mengikuti pola yang sudah dipakai filter:
	 * mengubah `urut` di query string, halaman kembali ke 1, filter lain tetap.
	 */
	function sortHref(key: string): string {
		const qs = new URLSearchParams(data.query);
		qs.delete('page');
		const current = data.filters.urut || 'nama';
		const next =
			key === 'harga'
				? current === 'harga_asc'
					? 'harga_desc'
					: 'harga_asc'
				: key;
		if (next === 'nama') qs.delete('urut');
		else qs.set('urut', next);
		const s = qs.toString();
		return s ? `/?${s}` : '/';
	}
</script>

<SiteShell>
	<main id="main" class="pub-main">
		<section class="pub-hero" aria-labelledby="hero-title">
			<div class="pub-container">
				<div class="pub-hero-grid">
					<div class="pub-hero-copy">
						<p class="pub-eyebrow">Direktori Media Tauthub</p>
						<h1 id="hero-title">Katalog Media Online Indonesia: DA, Trafik, dan Harga Publikasi</h1>
						<p class="pub-lead">
							<strong>{number(data.stats.total)} media aktif</strong> dari mitra penerbit. Tiap halaman memuat
							DA, estimasi trafik bulanan, harga per artikel, dan aturan backlink, jadi Anda bisa menilai
							medianya dulu sebelum memesan.
						</p>

						{#if data.popular.length}
							<nav class="pub-popular" aria-labelledby="popular-label">
								<p id="popular-label">Pencarian populer:</p>
								<ul>
									{#each data.popular as item}
										<li><a class="pub-chip" href={item.href}>{item.label}</a></li>
									{/each}
								</ul>
							</nav>
						{/if}
					</div>

					<aside class="pub-hero-preview" aria-labelledby="hero-preview-label">
						<div class="pub-hero-panel" aria-hidden="true"></div>
						{#if data.heroMedia.length}
							<ul class="pub-hero-cards">
								{#each data.heroMedia as media (media.slug)}
									<li><HeroMediaPreview {media} /></li>
								{/each}
							</ul>
						{:else}
							<p class="pub-hero-empty" role="status">Katalog sedang dimuat. Muat ulang sebentar lagi.</p>
						{/if}
						<p class="pub-hero-caption" id="hero-preview-label">Contoh media dari katalog Tauthub</p>
					</aside>
				</div>
			</div>
		</section>

		<section class="pub-catalog" aria-labelledby="catalog-title">
			<div class="pub-container">
				<CatalogFilters
					action="/"
					values={data.filters}
					categories={data.categories}
					lockedCategory={null}
				/>

				<header class="pub-section-head">
					<h2 id="catalog-title">{activeCategory ? 'Hasil kategori' : 'Semua media'}</h2>
					<p>{number(data.list.total)} media ditemukan</p>
				</header>

				{#if data.list.items.length}
					<MediaTable items={data.list.items} sort={data.filters.urut || 'nama'} {sortHref} />
					<Pagination pageNo={data.list.page} pages={data.list.pages} {hrefFor} />
				{:else}
					<div class="pub-empty" role="status">
						<p>Tidak ada media yang cocok dengan filter ini.</p>
						<p><a href="/">Reset filter</a></p>
					</div>
				{/if}

				<section class="pub-sources" aria-labelledby="sources-title">
					<div class="pub-sources-intro">
						<p class="pub-sources-eyebrow">Informasi</p>
						<h2 id="sources-title">Dari mana angka di halaman ini?</h2>
						<p>Berikut penjelasan singkat sumber data dan alur pemesanan media di Tauthub.</p>
					</div>

					<ul class="pub-sources-grid">
						<li>
							<span class="pub-sources-icon" aria-hidden="true"><ShieldCheck size={20} /></span>
							<h3>Verifikasi media</h3>
							<p>
								Setiap media dikelola mitra penerbit. Badge centang hanya muncul pada media yang domainnya
								sudah terverifikasi atau terdaftar di Dewan Pers.
							</p>
						</li>
						<li>
							<span class="pub-sources-icon" aria-hidden="true"><ChartColumn size={20} /></span>
							<h3>DA dan estimasi trafik</h3>
							<p>
								Nilai DA dan estimasi trafik diambil dari pemeriksaan berkala terhadap situs media, bukan dari
								klaim mitra.
							</p>
						</li>
						<li>
							<span class="pub-sources-icon" aria-hidden="true"><Wallet size={20} /></span>
							<h3>Harga publikasi</h3>
							<p>
								Harga yang tampil adalah harga publikasi per artikel untuk klien Tauthub, sesuai ketentuan
								masing-masing media.
							</p>
						</li>
					</ul>

					<p class="pub-sources-note">
						Untuk memesan, masuk ke dashboard Tauthub: pilih media, siapkan artikel, lalu pantau proses terbit
						sampai tautan tayang. Aturan backlink tiap media ada di halaman detailnya.
					</p>
				</section>
			</div>
		</section>
	</main>
</SiteShell>
