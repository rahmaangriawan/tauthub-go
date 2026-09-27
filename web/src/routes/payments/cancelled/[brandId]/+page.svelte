<script lang="ts">
	// Port app/payments/cancelled/[brandId]/page.tsx
	import { page } from '$app/state';
	import PaymentReturn from '$lib/components/payment-return.svelte';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
</script>

<PageTitle title="Pembayaran Dibatalkan" />

<PaymentReturn mode="cancelled" {brandId} />
