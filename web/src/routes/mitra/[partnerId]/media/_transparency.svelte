<script lang="ts">
	// Tab "Transparansi Data" di Daftar Media mitra: menampilkan metrik yang
	// SAMA dengan yang dilihat klien di katalog (read-only), supaya mitra tahu
	// bagaimana medianya tampil. Data datang dari GET /v1/partner/media yang
	// sudah dimuat halaman induk, tanpa request tambahan.
	import { tr } from '$lib/i18n/phrases';
	import MediaDetailSimple from './_detail-simple.svelte';
	import MediaFavicon from './_favicon.svelte';
	import type { MediaDetailItem } from '$lib/media/detail-types';
	import { formatTraffic } from '$lib/media/traffic';
	import { avgPublishHours, formatSla } from '$lib/media/sla';
	import { mediaDistribution } from '$lib/media/distribution';
	import { aiTrustToneStyle } from '$lib/media/ai-trust-tone';
	import { healthToneStyle } from '$lib/media/health-tone';
	import { mediaDomainLabel } from '$lib/media/domain-label';
	import { Eye } from '@lucide/svelte';

	type TransparencyMedia = MediaDetailItem & {
		total_terbit?: number | null;
		articles_7d?: number | null;
		crawl_source?: string | null;
		crawl_last_crawled_at?: string | null;
		crawl_last_status?: string | null;
		crawl_stale?: boolean;
	};

	let { items, loading = false }: { items: TransparencyMedia[]; loading?: boolean } = $props();

	let selected = $state<TransparencyMedia | null>(null);

	// Tabel transparansi dibatasi 10 baris per halaman supaya tidak jadi daftar
	// panjang saat media mitra banyak.
	const PAGE_SIZE = 10;
	let pageNo = $state(1);
	const totalPages = $derived(Math.max(1, Math.ceil(items.length / PAGE_SIZE)));
	const paged = $derived(items.slice((pageNo - 1) * PAGE_SIZE, pageNo * PAGE_SIZE));
	const rangeStart = $derived(items.length === 0 ? 0 : (pageNo - 1) * PAGE_SIZE + 1);
	const rangeEnd = $derived(Math.min(pageNo * PAGE_SIZE, items.length));

	// Jaga halaman tetap valid saat pencarian/filter memangkas daftar.
	$effect(() => {
		if (pageNo > totalPages) pageNo = totalPages;
	});

	const domainOf = (m: TransparencyMedia) =>
		(m.website_url ?? '')
			.replace(/^https?:\/\/(www\.)?/i, '')
			.replace(/\/+$/, '')
			.trim();

	const healthLabel = (score: number | null) =>
		score == null
			? tr('Belum Diskor')
			: score >= 70
				? tr('Sangat Sehat')
				: score >= 40
					? tr('Perlu Perhatian')
					: tr('Kondisi Rendah');

	const ratingText = (m: TransparencyMedia) =>
		m.rating_avg != null && m.rating_avg > 0
			? `${m.rating_avg.toFixed(1).replace('.', ',')}/5`
			: tr('Belum ada penilaian');

	const slaText = (m: TransparencyMedia) => {
		const hours = avgPublishHours(m);
		return hours == null ? tr('Belum ada riwayat') : formatSla(hours);
	};

	const distOf = (m: TransparencyMedia) =>
		mediaDistribution(m.health_breakdown, m.published_count ?? m.total_terbit ?? null);
	const distText = (m: TransparencyMedia) => {
		const d = distOf(m);
		if (!d.known) return tr('Belum ada');
		return d.published > 0 ? `${d.published} ${tr('artikel')}` : tr('Belum ada distribusi');
	};

	const crawlText = (m: TransparencyMedia) =>
		m.articles_7d != null
			? `${m.articles_7d} ${tr('artikel')}`
			: m.crawl_last_status === 'blocked'
				? tr('Diblokir situs')
				: m.crawl_last_status === 'no_source'
					? tr('Tak ada sitemap/feed')
					: tr('Belum ada data');

	const catsOf = (m: TransparencyMedia) =>
		m.categories && m.categories.length ? m.categories : m.category ? [m.category] : ['Umum'];
</script>

{#if loading}
	<p class="mt-6 text-sm text-muted-foreground">{tr('Memuat metrik media…')}</p>
{:else if items.length === 0}
	<div class="app-card mt-4 px-6 py-10 text-center">
		<p class="text-sm text-muted-foreground">{tr('Belum ada media untuk ditampilkan.')}</p>
	</div>
{:else}
	<p class="mt-3 text-xs text-muted-foreground">
		{tr('Metrik di bawah ini sama dengan yang dilihat klien di katalog Tauthub.')}
	</p>

	<!-- Desktop -->
	<div class="app-card mt-3 hidden overflow-x-auto md:block">
		<table class="w-full text-sm">
			<thead>
				<tr class="text-left text-xs text-muted-foreground">
					<th class="px-4 py-2.5 font-medium">{tr('Media')}</th>
					<th class="px-4 py-2.5 font-medium">{tr('Kategori')}</th>
					<th class="px-4 py-2.5 font-medium">{tr('Trafik')}</th>
					<th class="px-4 py-2.5 font-medium">DA</th>
					<th class="px-4 py-2.5 font-medium">{tr('AI Trust')}</th>
					<th class="px-4 py-2.5 font-medium">SLA</th>
					<th class="px-4 py-2.5 font-medium">{tr('Distribusi')}</th>
					<th class="px-4 py-2.5 font-medium">{tr('Rating')}</th>
					<th class="px-4 py-2.5 font-medium">{tr('Kesehatan')}</th>
					<th class="px-4 py-2.5 font-medium">{tr('Artikel/minggu')}</th>
					<th class="px-4 py-2.5 text-right font-medium">{tr('Aksi')}</th>
				</tr>
			</thead>
			<tbody>
				{#each paged as m (m.id)}
					{@const traffic = formatTraffic(m.sw_visits, m.traffic_estimate)}
					{@const cats = catsOf(m)}
					<tr class="border-t align-top">
						<td class="px-4 py-2.5">
							<div class="flex items-center gap-2">
								<MediaFavicon domain={domainOf(m)} size={18} />
								<span class="font-medium">{mediaDomainLabel(m.website_url, m.name)}</span>
							</div>
							{#if cats.length > 1}
								<span class="block text-xs text-muted-foreground">+{cats.length - 1} {tr('kategori lain')}</span>
							{/if}
						</td>
						<td class="px-4 py-2.5">{cats[0]}</td>
						<td class="px-4 py-2.5">
							<div class="flex flex-col">
								<span>{traffic.text}</span>
								{#if traffic.source}
									<span class="text-xs text-muted-foreground">{traffic.source}</span>
								{/if}
							</div>
						</td>
						<td class="px-4 py-2.5">{m.seo_da ?? tr('Belum diukur')}</td>
						<td class="px-4 py-2.5">
							<span
								class="inline-flex min-w-8 items-center justify-center rounded-md border px-1.5 py-0.5 text-xs font-semibold"
								style={aiTrustToneStyle(m.seo_ai_trust)}>{m.seo_ai_trust ?? tr('Belum diukur')}</span
							>
						</td>
						<td class="px-4 py-2.5">{slaText(m)}</td>
						<td class="px-4 py-2.5">
							{distOf(m).published > 0 ? `${distOf(m).published} ${tr('artikel')}` : distText(m)}
						</td>
						<td class="px-4 py-2.5">{ratingText(m)}</td>
						<td class="px-4 py-2.5">
							{#if m.health_score != null}
								<span
									class="inline-flex min-w-9 items-center justify-center rounded-md border px-2 py-0.5 text-xs font-bold tabular-nums"
									style={healthToneStyle(m.health_score)}
									aria-label={`${tr('Kesehatan')}: ${m.health_score} ${tr('dari')} 100, ${healthLabel(m.health_score)}`}
								>{m.health_score}</span>
							{:else}
								<span
									class="inline-flex items-center justify-center rounded-md border px-2 py-0.5 text-xs font-medium"
									style={healthToneStyle(null)}
									aria-label={`${tr('Kesehatan')}: ${tr('Belum Diskor')}`}
								>{tr('Belum Diskor')}</span>
							{/if}
						</td>
						<td class="px-4 py-2.5">
							<span class="font-medium">{crawlText(m)}</span>
							{#if m.crawl_stale && m.articles_7d != null}
								<span class="text-xs text-muted-foreground"> ({tr('belum diperbarui')})</span>
							{/if}
						</td>
						<td class="px-4 py-2.5 text-right">
							<button class="app-btn app-btn-soft !min-h-8 !px-2.5" type="button" onclick={() => (selected = m)}>
								<Eye size={14} /> {tr('Detail')}
							</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>

	<!-- Mobile -->
	<div class="mt-4 grid gap-2 md:hidden">
		{#each paged as m (m.id)}
			{@const traffic = formatTraffic(m.sw_visits, m.traffic_estimate)}
			{@const cats = catsOf(m)}
			<button class="app-card w-full px-4 py-3 text-left" type="button" onclick={() => (selected = m)}>
				<div class="flex items-center justify-between gap-2">
					<span class="flex min-w-0 items-center gap-2">
						<MediaFavicon domain={domainOf(m)} size={20} />
						<span class="truncate font-medium">{mediaDomainLabel(m.website_url, m.name)}</span>
					</span>
					<span class="shrink-0 text-xs text-muted-foreground">{cats[0]}</span>
				</div>
				<div class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 text-xs text-muted-foreground">
					<span>{tr('Trafik')}: {traffic.text}</span>
					<span>DA: {m.seo_da ?? tr('Belum diukur')}</span>
					<span>{tr('AI Trust')}: {m.seo_ai_trust ?? tr('Belum diukur')}</span>
					<span>SLA: {slaText(m)}</span>
					<span>{tr('Distribusi')}: {distText(m)}</span>
					<span>{tr('Rating')}: {ratingText(m)}</span>
					<span
						>{tr('Kesehatan')}:
						{#if m.health_score != null}
							<span
								class="ml-1 inline-flex min-w-7 items-center justify-center rounded border px-1 text-[11px] font-bold tabular-nums"
								style={healthToneStyle(m.health_score)}>{m.health_score}</span
							>
						{:else}
							<span class="font-medium">{tr('Belum Diskor')}</span>
						{/if}
					</span>
					<span>{tr('Artikel/minggu')}: {crawlText(m)}</span>
				</div>
			</button>
		{/each}
	</div>

	<div class="mt-3 flex flex-wrap items-center justify-between gap-2">
		<span class="text-xs text-muted-foreground">
			{tr('Menampilkan')} {rangeStart}–{rangeEnd} {tr('dari')} {items.length} {tr('media')}
		</span>
		{#if totalPages > 1}
			<div class="flex gap-2">
				<button
					class="app-btn app-btn-soft"
					type="button"
					disabled={pageNo <= 1}
					onclick={() => (pageNo = Math.max(1, pageNo - 1))}>{tr('Sebelumnya')}</button
				>
				<button
					class="app-btn app-btn-soft"
					type="button"
					disabled={pageNo >= totalPages}
					onclick={() => (pageNo = Math.min(totalPages, pageNo + 1))}>{tr('Berikutnya')}</button
				>
			</div>
		{/if}
	</div>
{/if}

{#if selected}
	<MediaDetailSimple media={selected} onclose={() => (selected = null)} />
{/if}
