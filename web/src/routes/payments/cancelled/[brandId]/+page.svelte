<script lang="ts">
	// Port app/payments/cancelled/[brandId]/page.tsx
	import { page } from '$app/state';
	import PaymentReturn from '$lib/components/payment-return.svelte';

	const brandId = $derived(page.params.brandId as string);
</script>

<PaymentReturn mode="cancelled" {brandId} />
