<script lang="ts">
	import { ArrowRight, CircleCheck } from '@lucide/svelte';
	import Tooltip from '$lib/components/tooltip.svelte';
	import MediaFavicon from './media-favicon.svelte';
	import type { PublicMedia } from '$lib/public/types';
	import { trafficText } from '$lib/public/format';

	/**
	 * Kartu ringkas untuk preview hero. Semua nilai berasal dari record media
	 * yang dikirim +page.server.ts (katalog nyata), tidak ada angka karangan.
	 * Metrik sengaja hanya dua (DA + trafik) supaya kartu tetap padat.
	 */
	let { media }: { media: PublicMedia } = $props();

	const label = $derived(media.domain || media.name);
	const category = $derived(media.categories[0] ?? media.media_type ?? null);
	const traffic = $derived(media.traffic.monthly != null ? trafficText(media.traffic) : null);
	const verified = $derived(media.trust.domain_verified || media.trust.dewan_pers_verified);

	// Teks tooltip mengikuti alasan verifikasinya, bukan label generik.
	const verifyLabel = $derived.by(() => {
		const parts: string[] = [];
		if (media.trust.domain_verified) parts.push('Domain sudah diverifikasi Tauthub');
		if (media.trust.dewan_pers_verified) parts.push('Terdaftar di Dewan Pers');
		return parts.join('. ') || 'Belum diverifikasi';
	});
</script>

<a class="hero-media" href={media.url} aria-label={`Lihat detail ${media.name}${verified ? `. ${verifyLabel}` : ''}`}>
	<MediaFavicon domain={media.domain} />

	<div class="hero-media-body">
		<div class="hero-media-name">
			<span>{label}</span>
			{#if verified}
				<Tooltip content={verifyLabel} label={verifyLabel}>
					<span class="hero-media-check" role="img" aria-label={verifyLabel}>
						<CircleCheck size={14} aria-hidden="true" />
					</span>
				</Tooltip>
			{/if}
		</div>
		{#if category}
			<div class="hero-media-cat">{category}</div>
		{/if}
	</div>

	{#if media.seo.da != null}
		<div class="hero-media-metric">
			<span>DA</span>
			<strong>{media.seo.da}</strong>
		</div>
	{/if}
	{#if traffic}
		<div class="hero-media-metric">
			<span>Trafik</span>
			<strong>{traffic}</strong>
		</div>
	{/if}

	<ArrowRight size={16} class="hero-media-arrow" aria-hidden="true" />
</a>
