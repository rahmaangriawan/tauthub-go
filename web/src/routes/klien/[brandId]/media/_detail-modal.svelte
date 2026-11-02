<script lang="ts">
	// Wrapper klien: memakai modal detail bersama di $lib supaya angka & tampilan
	// mitra (tab Transparansi) identik. Bagian klien-saja tetap aktif di sini.
	import { page } from '$app/state';
	import MediaDetailModal from '$lib/components/media-detail-modal.svelte';
	import type { MediaDetailItem } from '$lib/media/detail-types';

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
	const crawlStatusUrl = $derived(
		brandId && media.id ? `/v1/brands/${brandId}/media/${media.id}/crawl-status` : null
	);
</script>

<MediaDetailModal {media} {onclose} {onProceed} audience="client" {crawlStatusUrl} />
