<script lang="ts">
	// Port app/payments/success/[brandId]/page.tsx — brandId dari URL dipakai
	// untuk mengarahkan kembali ke dashboard brand yang benar.
	import { page } from '$app/state';
	import PaymentReturn from '$lib/components/payment-return.svelte';

	const brandId = $derived(page.params.brandId as string);
</script>

<PaymentReturn mode="success" {brandId} />
