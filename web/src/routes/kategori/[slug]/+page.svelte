<script lang="ts">
	import SiteShell from '$lib/components/public/site-shell.svelte';
	import CatalogFilters from '$lib/components/public/catalog-filters.svelte';
	import MediaTable from '$lib/components/public/media-table.svelte';
	import Pagination from '$lib/components/public/pagination.svelte';
	import { number } from '$lib/public/format';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	function hrefFor(p: number): string {
		const qs = new URLSearchParams(data.query);
		if (p > 1) qs.set('page', String(p));
		const s = qs.toString();
		const base = `/kategori/${encodeURIComponent(data.category.slug)}`;
		return s ? `${base}?${s}` : base;
	}

	/** Sorting kolom tabel, memakai query `urut` yang sama dengan filter. */
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
		const base = `/kategori/${encodeURIComponent(data.category.slug)}`;
		return s ? `${base}?${s}` : base;
	}
</script>

<SiteShell>
	<main id="main" class="pub-main">
		<section class="pub-hero" aria-labelledby="category-title">
			<div class="pub-container">
				<nav class="pub-breadcrumb" aria-label="Breadcrumb">
					<ol>
						<li><a href="/">Katalog media</a></li>
						<li aria-current="page">{data.category.name}</li>
					</ol>
				</nav>
				<p class="pub-eyebrow">Kategori media</p>
				<h1 id="category-title">Media {data.category.name}</h1>
				<p class="pub-lead">
					{number(data.category.count)} media di kategori <strong>{data.category.name}</strong> menerima artikel
					lewat Tauthub. Tiap halaman media menampilkan DA, estimasi trafik, harga per artikel, dan aturan
					backlink-nya.
				</p>
			</div>
		</section>

		<section class="pub-catalog" aria-labelledby="category-list-title">
			<div class="pub-container">
				<nav class="pub-chips" aria-label="Kategori lain">
					<ul>
						<li><a class="pub-chip" href="/">Semua kategori</a></li>
						{#each data.categories.slice(0, 12) as cat}
							<li>
								<a
									class="pub-chip"
									class:is-active={cat.slug === data.category.slug}
									href={`/kategori/${encodeURIComponent(cat.slug)}`}
									aria-current={cat.slug === data.category.slug ? 'page' : undefined}
								>
									{cat.name} <em>{cat.count}</em>
								</a>
							</li>
						{/each}
					</ul>
				</nav>

				<CatalogFilters
					action={`/kategori/${encodeURIComponent(data.category.slug)}`}
					values={data.filters}
					categories={data.categories}
					lockedCategory={data.category.slug}
				/>

				<header class="pub-section-head">
					<h2 id="category-list-title">{number(data.list.total)} media di kategori ini</h2>
					<p>Halaman {data.list.page} dari {Math.max(1, data.list.pages)}</p>
				</header>

				{#if data.list.items.length}
					<MediaTable items={data.list.items} sort={data.filters.urut || 'nama'} {sortHref} />
					<Pagination pageNo={data.list.page} pages={data.list.pages} {hrefFor} />
				{:else}
					<div class="pub-empty" role="status">
						<p>Belum ada media yang cocok di kategori ini dengan filter tersebut.</p>
						<p><a href={`/kategori/${encodeURIComponent(data.category.slug)}`}>Reset filter</a></p>
					</div>
				{/if}
			</div>
		</section>
	</main>
</SiteShell>
