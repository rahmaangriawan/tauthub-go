<script lang="ts">
	// Referral mitra — port section "Referral Mitra": kode & tautan rujukan
	// (mitra + klien), statistik klik, daftar rujukan, dan komisi terkumpul.
	import { onMount } from 'svelte';
	import { Check, MousePointerClick, Share2, Users } from '@lucide/svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	let loading = $state(true);
	let data = $state<any>(null);
	let copied = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			const res = await apiRequest('/v1/partner/referrals');
			if (res.ok) data = await res.json();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function copy(label: string, value: string) {
		try {
			await navigator.clipboard.writeText(value);
			copied = label;
			setTimeout(() => (copied = ''), 2000);
		} catch {
			copied = '';
		}
	}

	const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID')}`;
	const dt = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '—');
	const ratePct = $derived(((data?.rate_bps ?? 0) / 100).toFixed(2));
</script>

<section class="mx-auto max-w-5xl">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Referral Mitra</h1>
		<p class="mt-1 text-sm text-muted-foreground">Bagikan tautan rujukan Anda dan dapatkan komisi dari setiap transaksi.</p>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat data referral…</p>
	{:else if data}
		<div class="mt-4 grid gap-3 sm:grid-cols-3">
			<div class="app-card p-4">
				<div class="flex items-center gap-2 text-xs font-semibold tracking-wide text-muted-foreground"><Users size={14} /> TOTAL RUJUKAN</div>
				<p class="mt-2 text-2xl font-bold tabular-nums">{data.totalReferrals ?? 0}</p>
			</div>
			<div class="app-card p-4">
				<div class="flex items-center gap-2 text-xs font-semibold tracking-wide text-muted-foreground"><MousePointerClick size={14} /> KLIK TAUTAN</div>
				<p class="mt-2 text-2xl font-bold tabular-nums">{data.clicks?.total ?? 0}</p>
				<p class="mt-1 text-xs text-muted-foreground">{data.clicks?.unique ?? 0} pengunjung unik</p>
			</div>
			<div class="app-card p-4">
				<div class="flex items-center gap-2 text-xs font-semibold tracking-wide text-muted-foreground"><Share2 size={14} /> KOMISI TERKUMPUL</div>
				<p class="mt-2 text-2xl font-bold tabular-nums">{rp(data.referral_earning)}</p>
				<p class="mt-1 text-xs text-muted-foreground">Rate {ratePct}%</p>
			</div>
		</div>

		<div class="app-card mt-4 p-4">
			<div class="flex items-center gap-2">
				<h2 class="text-sm font-semibold">Kode Referral</h2>
				<span class="rounded-full px-2.5 py-0.5 font-mono text-sm font-bold" style="background-color: var(--accent); color: var(--accent-foreground);">{data.code}</span>
			</div>

			<div class="mt-4 space-y-3">
				<div>
					<p class="text-xs font-medium text-muted-foreground">Tautan pendaftaran mitra</p>
					<div class="mt-1 flex gap-2">
						<input class="app-input w-full font-mono text-xs" readonly value={data.link_mitra} />
						<button class="app-btn app-btn-soft shrink-0" onclick={() => copy('mitra', data.link_mitra)}>
							{#if copied === 'mitra'}<Check size={14} /> Tersalin{:else}Salin{/if}
						</button>
					</div>
				</div>
				<div>
					<p class="text-xs font-medium text-muted-foreground">Tautan pendaftaran klien</p>
					<div class="mt-1 flex gap-2">
						<input class="app-input w-full font-mono text-xs" readonly value={data.link_klien} />
						<button class="app-btn app-btn-soft shrink-0" onclick={() => copy('klien', data.link_klien)}>
							{#if copied === 'klien'}<Check size={14} /> Tersalin{:else}Salin{/if}
						</button>
					</div>
				</div>
			</div>
		</div>

		<div class="app-card mt-4 overflow-x-auto">
			<div class="border-b px-4 py-3"><h2 class="text-sm font-semibold">Daftar Rujukan</h2></div>
			{#if (data.referrals ?? []).length === 0}
				<p class="px-4 py-8 text-center text-sm text-muted-foreground">Belum ada yang mendaftar melalui tautan Anda.</p>
			{:else}
				<table class="w-full text-sm">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="px-4 py-2 font-medium">Pengguna</th>
							<th class="px-4 py-2 font-medium">Kode</th>
							<th class="px-4 py-2 font-medium">Tanggal</th>
						</tr>
					</thead>
					<tbody>
						{#each data.referrals as r (r.id)}
							<tr class="border-t">
								<td class="px-4 py-2">{r.referee_email ?? 'Pengguna baru'}</td>
								<td class="px-4 py-2 font-mono text-xs text-muted-foreground">{r.code}</td>
								<td class="px-4 py-2 text-xs text-muted-foreground">{dt(r.created_at)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>
	{/if}
</section>
