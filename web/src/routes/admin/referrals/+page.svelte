<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Referral — port app/admin/referrals/page.tsx: audit komisi referral
	// (earning → commission 1% direct) dari partner_referral_commissions.
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDate, fmtRp } from '$lib/admin-format';
	import { platformError } from '../_platform';

	type Row = {
		id: string; referrer: string; referrer_email: string | null; referee: string;
		referee_email: string; amount: number; rate_bps: number; status: string; created_at: string;
	};

	let rows = $state<Row[]>([]);
	let error = $state('');
	let loaded = $state(false);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			const res = await apiRequest('/v1/platform/referrals');
			if (res.ok) {
				const data = await res.json();
				rows = data.commissions ?? [];
			} else {
				error = await platformError(res);
			}
			loaded = true;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	const statusTone = (status: string): 'success' | 'accent' | 'danger' | 'warning' =>
		status === 'available' ? 'success' : status === 'paid' ? 'accent' : status === 'reversed' ? 'danger' : 'warning';
</script>

<section>
	<PageHeader title="Referral" description="Audit referral · earning → commission 1% direct" />
	<Feedback loading={!loaded} {error} loadingText="Memuat referral…" />

	{#if loaded}
		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">Referrer → Referral</th>
						<th class="px-4 py-2.5 font-medium">Rate</th>
						<th class="px-4 py-2.5 font-medium">Amount</th>
						<th class="px-4 py-2.5 font-medium">Status</th>
						<th class="px-4 py-2.5 font-medium">{tr("Tanggal")}</th>
					</tr>
				</thead>
				<tbody>
					{#each rows as r (r.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<p class="font-medium">{r.referrer}</p>
								<p class="text-xs text-muted-foreground">{r.referee} · {r.referee_email}</p>
							</td>
							<td class="px-4 py-2.5 tabular-nums">{(r.rate_bps / 100).toFixed(1)}%</td>
							<td class="px-4 py-2.5 tabular-nums">{fmtRp(r.amount)}</td>
							<td class="px-4 py-2.5"><Badge label={r.status} tone={statusTone(r.status)} /></td>
							<td class="px-4 py-2.5 text-muted-foreground">{r.created_at ? fmtDate(r.created_at) : '—'}</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if rows.length === 0 && !error}
				<p class="px-4 py-4 text-sm text-muted-foreground">{tr("Belum ada komisi referral.")}</p>
			{/if}
		</div>
	{/if}
</section>
