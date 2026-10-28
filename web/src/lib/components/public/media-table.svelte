<script lang="ts">
	import { ArrowRight, ArrowUpDown, Info } from '@lucide/svelte';
	import Tooltip from '$lib/components/tooltip.svelte';
	import MediaFavicon from './media-favicon.svelte';
	import type { PublicMedia } from '$lib/public/types';
	import { idr, linkText, trafficText } from '$lib/public/format';
	import { formatSla } from '$lib/media/sla';

	/**
	 * Tabel direktori media. Satu baris = satu media; semua nilai berasal dari
	 * record katalog (tidak ada angka karangan). Kolom yang bisa diurutkan
	 * mengirim `urut` ke query katalog yang sama dengan filter — bukan sorting
	 * sisi klien, supaya urutan konsisten dengan pagination.
	 */
	let {
		items,
		sort,
		sortHref
	}: {
		items: PublicMedia[];
		sort: string;
		sortHref: (key: string) => string;
	} = $props();

	type SortKey = 'nama' | 'da' | 'trafik' | 'harga_asc' | 'harga_desc';

	// Kolom yang punya padanan di API. Kolom lain tidak menampilkan ikon sortir
	// karena backend belum mendukungnya (jangan menawarkan sesuatu yang diam).
	const SORTABLE: Record<string, SortKey> = {
		nama: 'nama',
		da: 'da',
		trafik: 'trafik',
		harga: 'harga_asc'
	};

	function isSorted(col: string): boolean {
		if (col === 'harga') return sort === 'harga_asc' || sort === 'harga_desc';
		return sort === SORTABLE[col];
	}

	const GEO_TIP = 'GEO menandai apakah situs media ini pernah dikutip sebagai sumber pada jawaban AI Google.';
	const SLA_TIP = 'SLA adalah rata-rata waktu terbit nyata media ini, dihitung dari riwayat publikasi.';
	const TRAFFIC_TIP = 'Estimasi trafik bulanan. Sumber SimilarWeb dipakai bila tersedia; selain itu angka estimasi media.';
</script>

<div class="pub-table-wrap">
	<table class="pub-table">
		<caption class="sr-only">Direktori media Tauthub</caption>
		<thead>
			<tr>
				<th scope="col" class="col-name" aria-sort={sort === 'nama' ? 'ascending' : 'none'}>
					<a class="pub-th-sort" href={sortHref('nama')}>
						Nama media
						<ArrowUpDown size={12} aria-hidden="true" />
					</a>
				</th>
				<th scope="col" class="col-num" aria-sort={sort === 'da' ? 'descending' : 'none'}>
					<a class="pub-th-sort" href={sortHref('da')}>
						DA
						<ArrowUpDown size={12} aria-hidden="true" />
					</a>
				</th>
				<th scope="col" class="col-num" aria-sort={sort === 'trafik' ? 'descending' : 'none'}>
					<a class="pub-th-sort" href={sortHref('trafik')}>
						Trafik (est)
						<ArrowUpDown size={12} aria-hidden="true" />
					</a>
					<Tooltip content={TRAFFIC_TIP} side="bottom">
						<button type="button" class="pub-th-info" aria-label={TRAFFIC_TIP}>
							<Info size={12} aria-hidden="true" />
						</button>
					</Tooltip>
				</th>
				<th scope="col" class="col-geo">
					GEO
					<Tooltip content={GEO_TIP} side="bottom">
						<button type="button" class="pub-th-info" aria-label={GEO_TIP}>
							<Info size={12} aria-hidden="true" />
						</button>
					</Tooltip>
				</th>
				<th scope="col" class="col-sla">
					SLA
					<Tooltip content={SLA_TIP} side="bottom">
						<button type="button" class="pub-th-info" aria-label={SLA_TIP}>
							<Info size={12} aria-hidden="true" />
						</button>
					</Tooltip>
				</th>
				<th scope="col" class="col-cat">Kategori</th>
				<th scope="col" class="col-price" aria-sort={isSorted('harga') ? 'ascending' : 'none'}>
					<a class="pub-th-sort" href={sortHref('harga')}>
						Harga
						<ArrowUpDown size={12} aria-hidden="true" />
					</a>
				</th>
				<th scope="col" class="col-action">Aksi</th>
			</tr>
		</thead>
		<tbody>
			{#each items as media (media.slug)}
				{@const verified = media.trust.dewan_pers_verified || media.trust.domain_verified}
				{@const link = linkText(media.link_policy)}
				{@const geo = media.seo.ai_citation}
				{@const sla = formatSla(media.sla?.avg_publish_hours ?? null)}
				<tr>
					<th scope="row" class="col-name">
						<div class="pub-media">
							<MediaFavicon domain={media.domain} size={26} />
							<div class="pub-media-text">
								<span class="pub-media-name">
									{media.name}
									{#if verified}
										<Tooltip
											content={media.trust.dewan_pers_verified
												? 'Terdaftar sebagai perusahaan pers di Dewan Pers.'
												: 'Domain media ini sudah dipastikan milik mitra penerbit.'}
											side="bottom"
										>
											<button type="button" class="pub-media-check" aria-label="Media terverifikasi">
												<svg viewBox="0 0 24 24" width="13" height="13" aria-hidden="true">
													<path
														fill="currentColor"
														d="M12 2l2.4 1.8 3-.2 1 2.8 2.4 1.8-1 2.8 1 2.8-2.4 1.8-1 2.8-3-.2L12 22l-2.4-1.8-3 .2-1-2.8L3.2 15.8l1-2.8-1-2.8 2.4-1.8 1-2.8 3 .2z"
													/>
													<path
														fill="#fff"
														d="M10.6 15.2l-2.8-2.8 1.2-1.2 1.6 1.6 4-4 1.2 1.2z"
													/>
												</svg>
											</button>
										</Tooltip>
									{/if}
								</span>
								<span class="pub-media-domain">{media.domain || media.website_url || '-'}</span>
								<span class="pub-media-meta">
									<span>{media.link_policy.max_links !== 0 ? link : 'Tanpa backlink'}</span>
									{#if media.trust.published_count === 0}
										<span class="pub-media-none">Belum ada distribusi</span>
									{/if}
								</span>
							</div>
						</div>
					</th>

					<td class="col-num">
						{#if media.seo.da != null}<span class="pub-num">{media.seo.da}</span>{:else}<span class="pub-dash">—</span>{/if}
					</td>

					<td class="col-num">
						{#if media.traffic.monthly != null && media.traffic.monthly > 0}
							<span class="pub-num">{trafficText(media.traffic)}</span>
						{:else}
							<span class="pub-dash">—</span>
						{/if}
					</td>

					<td class="col-geo">
						{#if geo === true}
							<span class="pub-geo is-yes">Ya</span>
						{:else if geo === false}
							<span class="pub-geo is-no">Tidak</span>
						{:else}
							<span class="pub-dash">—</span>
						{/if}
					</td>

					<td class="col-sla">
						{#if sla === '—'}<span class="pub-dash">—</span>{:else}<span class="pub-sla">{sla}</span>{/if}
					</td>

					<td class="col-cat">
						{#if media.categories.length}
							<span class="pub-cat-list">
								{#each media.categories.slice(0, 2) as cat (cat)}
									<span class="pub-cat">{cat}</span>
								{/each}
								{#if media.categories.length > 2}
									<Tooltip content={media.categories.slice(2).join(', ')} side="bottom">
										<button type="button" class="pub-cat pub-cat-more">+{media.categories.length - 2}</button>
									</Tooltip>
								{/if}
							</span>
						{:else}
							<span class="pub-dash">—</span>
						{/if}
					</td>

					<td class="col-price">
						<span class="pub-price-label">{media.price.type === 'nego' ? 'Harga acuan' : 'Harga mulai'}</span>
						<span class="pub-price-value">{idr(media.price.amount)}</span>
					</td>

					<td class="col-action">
						<a class="pub-row-cta" href={media.url}>
							Lihat detail
							<ArrowRight size={13} aria-hidden="true" />
						</a>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
