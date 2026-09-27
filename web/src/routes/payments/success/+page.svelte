<script lang="ts">
	// Port app/payments/success/page.tsx
	import PaymentReturn from '$lib/components/payment-return.svelte';
	import PageTitle from '$lib/components/page-title.svelte';
</script>

<PageTitle title="Pembayaran Berhasil" />

<PaymentReturn mode="success" />
