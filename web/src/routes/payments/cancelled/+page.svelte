<script lang="ts">
	// Port app/payments/cancelled/page.tsx
	import PaymentReturn from '$lib/components/payment-return.svelte';
	import PageTitle from '$lib/components/page-title.svelte';
</script>

<PageTitle title="Pembayaran Dibatalkan" />

<PaymentReturn mode="cancelled" />
