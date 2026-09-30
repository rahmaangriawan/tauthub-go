<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Port app/payments/success/page.tsx
	import PaymentReturn from '$lib/components/payment-return.svelte';
	import PageTitle from '$lib/components/page-title.svelte';
</script>

<PageTitle title={tr("Pembayaran Berhasil")} />

<PaymentReturn mode="success" />
