<script lang="ts">
	// Fallback klien: bila load server tidak sempat redirect (navigasi murni
	// klien), arahkan ke tab Domain Referral di Monitoring Non VIP.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import PageTitle from '$lib/components/page-title.svelte';

	onMount(() => {
		void goto('/admin/referral-audit', { replaceState: true });
	});
</script>

<PageTitle title="Monitoring Non VIP" />

<p class="text-sm text-muted-foreground">Mengalihkan ke Monitoring Non VIP…</p>
