<script lang="ts">
	import Tooltip from '$lib/components/tooltip.svelte';
	import type { PublicMedia } from '$lib/public/types';
	import { idr, linkText, tierLabel, trafficText } from '$lib/public/format';

	let { media }: { media: PublicMedia } = $props();

	const da = $derived(media.seo.da);
	const traffic = $derived(trafficText(media.traffic));
	const isNego = $derived(media.price.type === 'nego');
</script>

<a
	class="pub-card"
	href={media.url}
	aria-label={`Detail media ${media.name}${media.trust.dewan_pers_verified ? '. Terdaftar di Dewan Pers' : ''}`}
>
	<div class="pub-card-top">
		<div>
			<h3 class="pub-card-name">{media.name}</h3>
			{#if media.domain}<div class="pub-card-domain">{media.domain}</div>{/if}
		</div>
		<span class="pub-tier {media.tier ?? ''}">{tierLabel(media.tier)}</span>
	</div>

	{#if media.description}
		<p class="pub-card-desc">{media.description}</p>
	{/if}

	<div class="pub-metrics">
		<span>DA <b>{da ?? '-'}</b></span>
		<span>Trafik <b>{traffic}</b></span>
		{#if media.trust.dewan_pers_verified}
			<Tooltip content="Terdaftar sebagai perusahaan pers di Dewan Pers.">
				<span><b>Dewan Pers</b></span>
			</Tooltip>
		{/if}
		{#if (media.trust.rating_avg ?? 0) > 0}<span>Rating <b>{media.trust.rating_avg?.toFixed(1)}/5</b></span>{/if}
	</div>

	{#if media.categories.length}
		<div class="pub-tags">
			{#each media.categories.slice(0, 3) as cat (cat)}
				<span class="pub-tag">{cat}</span>
			{/each}
		</div>
	{/if}

	<div class="pub-card-bottom">
		<div class="pub-price">
			<small>{isNego ? 'Harga acuan (nego)' : 'Harga mulai'}</small>
			{idr(media.price.amount)}
		</div>
		<span class="pub-card-cta">Lihat detail</span>
	</div>
	<span class="sr-only">{linkText(media.link_policy)}</span>
</a>

<style>
	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip: rect(0 0 0 0);
		white-space: nowrap;
	}
</style>
