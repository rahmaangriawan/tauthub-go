<script lang="ts">
	// Konfigurasi TC — port app/admin/credits/page.tsx: konfigurasi denominasi
	// (rate + paket topup), penyesuaian manual ledger (kredit/debit), dan daftar
	// wallet Client (available/reserved/captured/total TC, topup, publikasi).
	import { onMount } from 'svelte';
	import { Coins } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtNum, fmtRp } from '$lib/admin-format';
	import { platformError } from '../_platform';

	type WalletRow = {
		brand_id: string; brand_name: string | null; client_name: string | null;
		available_tc: number; reserved_tc: number; captured_tc: number; total_tc: number;
		topup_tc: number; topup_idr: number; spent_tc: number; updated_at: string | null;
	};
	type TcConfig = { rate_idr: number; topup_packages: Array<{ tc: number; idr: number }> };

	const PAGE_SIZE = 20;

	let wallets = $state<WalletRow[]>([]);
	let config = $state<TcConfig | null>(null);
	let error = $state('');
	let notice = $state('');
	let loading = $state(true);
	let q = $state('');
	let busy = $state(false);

	let rate = $state('1000');
	let packages = $state('500:500000,1000:1000000,5000:5000000');

	let adjBrand = $state('');
	let adjTc = $state('');
	let adjDirection = $state<'credit' | 'debit'>('credit');
	let adjReason = $state('');
	let pageNum = $state(1);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	function onSearch(value: string) {
		q = value;
		void load();
	}

	async function load() {
		loading = true;
		error = '';
		const res = await apiRequest(`/v1/platform/credits?q=${encodeURIComponent(q)}`);
		if (res.ok) {
			const data = await res.json();
			wallets = data.wallets ?? [];
			config = data.tc_config;
			rate = String(data.tc_config.rate_idr);
			packages = (data.tc_config.topup_packages ?? []).map((p: { tc: number; idr: number }) => `${p.tc}:${p.idr}`).join(',');
		} else {
			error = await platformError(res);
		}
		loading = false;
	}

	async function saveConfig() {
		busy = true;
		error = '';
		notice = '';
		const rateIdr = Number(rate);
		const topupPackages = packages.split(',').map((part) => {
			const [tc, idrAmt] = part.split(':').map((s) => Number(s.trim()));
			return { tc, idr: idrAmt };
		});
		const res = await apiRequest('/v1/platform/credits', {
			method: 'PUT',
			body: JSON.stringify({ rate_idr: rateIdr, topup_packages: topupPackages })
		});
		if (!res.ok) {
			error = await platformError(res);
			busy = false;
			return;
		}
		notice = 'Konfigurasi TC tersimpan.';
		busy = false;
		await load();
	}

	async function adjust(brandId: string) {
		busy = true;
		error = '';
		notice = '';
		const res = await apiRequest('/v1/platform/credits', {
			method: 'POST',
			body: JSON.stringify({ brand_id: brandId, tc: Number(adjTc), direction: adjDirection, reason: adjReason })
		});
		if (!res.ok) {
			error = await platformError(res);
			busy = false;
			return;
		}
		notice = 'Penyesuaian ledger diterapkan.';
		adjTc = '';
		adjReason = '';
		busy = false;
		await load();
	}

	const start = $derived((pageNum - 1) * PAGE_SIZE);
	const visibleWallets = $derived(wallets.slice(start, start + PAGE_SIZE));
	const denied = $derived(Boolean(!loading && wallets.length === 0 && error.includes('Akses platform')));
</script>

<section class="mx-auto max-w-7xl">
	<PageHeader
		title="Tauthub Credit"
		description="Saldo internal Client dalam TC (1 TC = Rp1.000 default). Client melihat TC, Partner melihat Rupiah, Admin melihat keduanya. TC bukan uang yang dapat dicairkan."
	>
		<span class="rounded-full px-3 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">
			{config ? `1 TC = Rp${config.rate_idr.toLocaleString('id-ID')}` : 'Konfigurasi'}
		</span>
	</PageHeader>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke pengelolaan Tauthub Credit.</p>
		</div>
	{:else}
		<Feedback {loading} {error} {notice} loadingText="Memuat Tauthub Credit…" />

		<div class="app-card mt-4 p-5">
			<h2 class="text-sm font-semibold">Konfigurasi Denominasi</h2>
			<p class="mt-1 text-xs text-muted-foreground">
				Rate baru hanya berlaku untuk transaksi baru — semua entri ledger lama menyimpan snapshot rate saat transaksi terjadi. Paket Top Up harus memenuhi tc × rate = idr.
			</p>
			<div class="mt-3 flex flex-wrap items-end gap-3">
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Rate (1 TC = ? Rupiah)</span>
					<input class="app-input !w-[140px]" type="number" min="1" value={rate} oninput={(e) => (rate = (e.currentTarget as HTMLInputElement).value)} />
				</label>
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Paket Top Up (tc:idr, pisahkan koma — contoh 500:500000,1000:1000000)</span>
					<input class="app-input !w-[380px]" bind:value={packages} />
				</label>
				<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void saveConfig()}>Simpan konfigurasi</button>
			</div>
		</div>

		<div class="app-card mt-4 p-5">
			<h2 class="text-sm font-semibold">Penyesuaian Manual (Ledger)</h2>
			<p class="mt-1 text-xs text-muted-foreground">
				Penyesuaian tercatat di ledger + audit log dengan alasan. Debit tidak bisa membuat saldo available negatif.
			</p>
			<div class="mt-3 flex flex-wrap items-end gap-3">
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Brand (ID)</span>
					<Select
						ariaLabel="Brand (ID)"
						placeholder="Pilih brand…"
						minWidth={240}
						value={adjBrand}
						onchange={(v) => (adjBrand = v)}
						options={[
							{ value: '', label: 'Pilih brand…' },
							...wallets.map((w) => ({ value: w.brand_id, label: w.brand_name ?? w.client_name ?? w.brand_id }))
						]}
					/>
				</label>
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Jumlah TC</span>
					<input class="app-input !w-[120px]" type="number" min="1" value={adjTc} oninput={(e) => (adjTc = (e.currentTarget as HTMLInputElement).value)} />
				</label>
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Arah</span>
					<Select
						ariaLabel="Arah"
						minWidth={170}
						value={adjDirection}
						onchange={(v) => (adjDirection = v === 'debit' ? 'debit' : 'credit')}
						options={[
							{ value: 'credit', label: 'Kredit (+TC)' },
							{ value: 'debit', label: 'Debit (−TC)' }
						]}
					/>
				</label>
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Alasan</span>
					<input class="app-input !w-[260px]" placeholder="Contoh: kompensasi pelanggan" bind:value={adjReason} />
				</label>
				<button
					class="app-btn app-btn-primary"
					type="button"
					disabled={busy || !adjBrand || !adjTc || adjReason.trim().length < 3}
					onclick={() => void adjust(adjBrand)}
				>Terapkan</button>
			</div>
		</div>

		<div class="mt-4 flex flex-wrap items-center justify-between gap-2">
			<h2 class="text-sm font-semibold">Wallet Client</h2>
			<label class="app-input !w-[240px] !py-1.5">
				<input class="w-full bg-transparent text-sm outline-none" placeholder="Cari brand…" value={q} oninput={(e) => onSearch((e.currentTarget as HTMLInputElement).value)} />
			</label>
		</div>

		<div class="app-card mt-3 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 1080px;">
				<thead>
					<tr class="text-xs text-muted-foreground">
						<th class="px-4 py-2.5 text-left font-medium">Brand / Client</th>
						<th class="px-4 py-2.5 text-right font-medium">Available TC</th>
						<th class="px-4 py-2.5 text-right font-medium">Reserved TC</th>
						<th class="px-4 py-2.5 text-right font-medium">Captured TC</th>
						<th class="px-4 py-2.5 text-right font-medium">Total TC</th>
						<th class="px-4 py-2.5 text-right font-medium">≈ IDR</th>
						<th class="px-4 py-2.5 text-right font-medium">Top Up</th>
						<th class="px-4 py-2.5 text-right font-medium">Publikasi (TC)</th>
					</tr>
				</thead>
				<tbody>
					{#each visibleWallets as w (w.brand_id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<p class="font-bold">{w.brand_name ?? '—'}</p>
								<p class="text-xs text-muted-foreground">{w.client_name ?? w.brand_id}</p>
							</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{fmtNum(w.available_tc)} TC</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{fmtNum(w.reserved_tc)} TC</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{fmtNum(w.captured_tc)} TC</td>
							<td class="px-4 py-2.5 text-right font-bold tabular-nums">{fmtNum(w.total_tc)} TC</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{fmtRp(w.total_tc * (config?.rate_idr ?? 1000))}</td>
							<td class="px-4 py-2.5 text-right tabular-nums">
								{#if w.topup_tc > 0}
									{fmtNum(w.topup_tc)} TC<br /><span class="text-xs text-muted-foreground">{fmtRp(w.topup_idr)}</span>
								{:else}
									—
								{/if}
							</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{w.spent_tc > 0 ? `${fmtNum(w.spent_tc)} TC` : '—'}</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && wallets.length === 0}
				<div class="flex flex-col items-center px-4 py-12 text-center">
					<Coins size={26} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">Belum ada wallet. Wallet dibuat otomatis saat Client melakukan Top Up atau publikasi pertama.</p>
				</div>
			{/if}
			<div class="px-4 pb-4">
				<Pagination
					total={wallets.length}
					limit={PAGE_SIZE}
					offset={start}
					onChange={(next) => (pageNum = Math.floor(next / PAGE_SIZE) + 1)}
				/>
			</div>
		</div>
	{/if}
</section>
