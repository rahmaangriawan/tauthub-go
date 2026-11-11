<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Konfigurasi Saldo — port app/admin/credits/page.tsx: konfigurasi denominasi
	// (rate + paket topup), penyesuaian manual ledger (kredit/debit), dan daftar
	// saldo Client (sisa/ditahan/terpakai/total, topup, publikasi).
	import { onMount } from 'svelte';
	import { Coins, Info } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import Tooltip from '$lib/components/tooltip.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { rp } from '$lib/credits/rupiah';
	import { platformError } from '../_platform';

	type WalletRow = {
		brand_id: string; brand_name: string | null; client_name: string | null;
		available_tc: number; reserved_tc: number; captured_tc: number; total_tc: number;
		topup_tc: number; topup_idr: number; pub_tc: number; updated_at: string | null;
	};
	type TcConfig = { rate_idr: number; topup_packages: Array<{ tc: number; idr: number }> };

	const PAGE_SIZE = 20;

	// Keterangan tiap kolom uang. Dipasang sebagai tooltip di header supaya
	// admin baru tidak perlu menebak arti Sisa Saldo, Ditahan, dan Terpakai.
	const TIPS = {
		available: 'Uang yang masih bisa dipakai klien untuk mengajukan publikasi atau memakai fitur AI.',
		reserved:
			'Uang yang dikunci saat publikasi diajukan tapi artikelnya belum terbit. Kalau publikasi batal, uangnya kembali ke Sisa Saldo. Kalau artikel terbit, uangnya pindah ke Terpakai.',
		captured:
			'Uang yang sudah habis dan tidak kembali ke saldo. Isinya biaya publikasi yang terbit ditambah biaya AI seperti tulis artikel, generate gambar, dan Profiler.',
		total: 'Jumlah dari Sisa Saldo, Ditahan, dan Terpakai.',
		topup: 'Total uang yang pernah disetor klien ke akunnya sejak awal.',
		pub: 'Total biaya publikasi yang sudah terbit. Biaya AI tidak dihitung di sini, biaya AI masuk ke Terpakai.'
	};

	// Dua kartu alat admin (konfigurasi denominasi + penyesuaian ledger manual)
	// disembunyikan: kurs sekarang tetap 1 dan paket top up dikelola di halaman
	// Paket. Kodenya sengaja tidak dihapus supaya bisa dimunculkan lagi dengan
	// satu baris kalau suatu saat perlu koreksi saldo manual.
	const SHOW_ADMIN_TOOLS = false;

	type PromoCode = {
		code: string; bonus_tc: number; min_topup_tc: number; max_uses: number | null;
		used_count: number; valid_until: string | null; active: boolean; note: string | null;
	};
	let promos = $state<PromoCode[]>([]);
	let promoForm = $state({ code: '', bonus_tc: '', min_topup_tc: '', max_uses: '', valid_until: '', note: '' });
	let promoBusy = $state(false);

	async function loadPromos() {
		const res = await apiRequest('/v1/platform/promo-codes');
		if (res.ok) promos = (await res.json()).codes ?? [];
	}

	async function createPromo(e: SubmitEvent) {
		e.preventDefault();
		promoBusy = true;
		error = '';
		notice = '';
		try {
			const f = promoForm;
			const res = await apiRequest('/v1/platform/promo-codes', {
				method: 'POST',
				body: JSON.stringify({
					code: f.code,
					bonus_tc: Number(f.bonus_tc),
					min_topup_tc: f.min_topup_tc ? Number(f.min_topup_tc) : undefined,
					max_uses: f.max_uses ? Number(f.max_uses) : undefined,
					valid_until: f.valid_until || undefined,
					note: f.note || undefined
				})
			});
			if (!res.ok) {
				error = await platformError(res);
				return;
			}
			notice = `Kode ${f.code.toUpperCase()} dibuat.`;
			promoForm = { code: '', bonus_tc: '', min_topup_tc: '', max_uses: '', valid_until: '', note: '' };
			await loadPromos();
		} finally {
			promoBusy = false;
		}
	}

	async function togglePromo(p: PromoCode) {
		promoBusy = true;
		try {
			const res = await apiRequest(`/v1/platform/promo-codes/${encodeURIComponent(p.code)}`, {
				method: 'PUT',
				body: JSON.stringify({ active: !p.active })
			});
			if (!res.ok) error = await platformError(res);
			await loadPromos();
		} finally {
			promoBusy = false;
		}
	}

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
			packages = (data.tc_config.topup_packages ?? [])
				.map((p: { tc: number; idr: number; bonus_tc?: number }) => `${p.tc}:${p.idr}${p.bonus_tc ? ':' + p.bonus_tc : ''}`)
				.join(',');
		} else {
			error = await platformError(res);
		}
		loading = false;
		void loadPromos();
	}

	async function saveConfig() {
		busy = true;
		error = '';
		notice = '';
		const rateIdr = Number(rate);
		const topupPackages = packages.split(',').map((part) => {
			const [tc, idrAmt, bonus] = part.split(':').map((s) => Number(s.trim()));
			return bonus ? { tc, idr: idrAmt, bonus_tc: bonus } : { tc, idr: idrAmt };
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
		notice = 'Konfigurasi saldo tersimpan.';
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

<section>
	<PageHeader
		title={tr("Saldo Klien")}
		description={tr("Saldo internal Client dalam rupiah. Client, Partner, dan Admin melihat nominal yang sama. Nilai ini bukan uang yang dapat dicairkan.")}
	>
		<span class="rounded-full px-3 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">
			{config ? `Kurs aktif: ${config.rate_idr.toLocaleString('id-ID')} × (1 unit = Rp1)` : 'Konfigurasi'}
		</span>
	</PageHeader>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke pengelolaan Tauthub Credit.")}</p>
		</div>
	{:else}
		<Feedback {loading} {error} {notice} loadingText="Memuat Tauthub Credit…" />

		<div class="app-card mt-4 p-5">
			<h2 class="text-sm font-semibold">Kode promo top up</h2>
			<p class="mt-1 text-xs text-muted-foreground">Bonus TC tetap yang masuk sebagai baris ledger terpisah setelah pembayaran lunas. Tidak mengubah nominal pembayaran atau faktur. Satu brand hanya sekali per kode.</p>
			<form class="mt-3 flex flex-wrap items-end gap-3" onsubmit={createPromo}>
				<label class="block"><span class="mb-1 block text-xs font-semibold text-muted-foreground">Kode</span>
					<input class="app-input !w-[160px]" required maxlength="40" bind:value={promoForm.code} /></label>
				<label class="block"><span class="mb-1 block text-xs font-semibold text-muted-foreground">Bonus (TC)</span>
					<input class="app-input !w-[110px]" type="number" min="1" required bind:value={promoForm.bonus_tc} /></label>
				<label class="block"><span class="mb-1 block text-xs font-semibold text-muted-foreground">Minimal top up (TC)</span>
					<input class="app-input !w-[140px]" type="number" min="1" bind:value={promoForm.min_topup_tc} /></label>
				<label class="block"><span class="mb-1 block text-xs font-semibold text-muted-foreground">Batas pakai</span>
					<input class="app-input !w-[110px]" type="number" min="1" placeholder="tanpa batas" bind:value={promoForm.max_uses} /></label>
				<label class="block"><span class="mb-1 block text-xs font-semibold text-muted-foreground">Berlaku sampai</span>
					<input class="app-input !w-[160px]" type="date" bind:value={promoForm.valid_until} /></label>
				<button class="app-btn app-btn-primary" type="submit" disabled={promoBusy || !promoForm.code || !promoForm.bonus_tc}>Buat kode</button>
			</form>
			{#if promos.length > 0}
				<table class="mt-4 w-full text-sm">
					<thead><tr class="border-b text-left text-[11px] text-muted-foreground">
						<th class="py-2 font-semibold">KODE</th><th class="py-2 font-semibold">BONUS</th>
						<th class="py-2 font-semibold">MIN. TOP UP</th><th class="py-2 font-semibold">TERPAKAI</th>
						<th class="py-2 font-semibold">BERLAKU</th><th class="py-2 text-right font-semibold">AKSI</th>
					</tr></thead>
					<tbody>
						{#each promos as p (p.code)}
							<tr class="border-b last:border-0">
								<td class="py-2 font-medium">{p.code}</td>
								<td class="py-2">{p.bonus_tc} TC</td>
								<td class="py-2">{p.min_topup_tc} TC</td>
								<td class="py-2">{p.used_count}{p.max_uses ? ` / ${p.max_uses}` : ''}</td>
								<td class="py-2 text-muted-foreground">{p.active ? (p.valid_until ? `s.d. ${new Date(p.valid_until).toLocaleDateString('id-ID')}` : 'Aktif') : 'Nonaktif'}</td>
								<td class="py-2 text-right">
									<button class="app-btn app-btn-soft" type="button" disabled={promoBusy} onclick={() => void togglePromo(p)}>{p.active ? 'Nonaktifkan' : 'Aktifkan'}</button>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			{/if}
		</div>

		{#if SHOW_ADMIN_TOOLS}
			<div class="app-card mt-4 p-5">
				<h2 class="text-sm font-semibold">Konfigurasi Denominasi</h2>
				<p class="mt-1 text-xs text-muted-foreground">{tr("Rate baru hanya berlaku untuk transaksi baru — semua entri ledger lama menyimpan snapshot rate saat transaksi terjadi. Paket Top Up harus memenuhi tc × rate = idr.")}</p>
				<div class="mt-3 flex flex-wrap items-end gap-3">
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">Kurs lama (referensi)</span>
						<input class="app-input !w-[140px]" type="number" min="1" value={rate} oninput={(e) => (rate = (e.currentTarget as HTMLInputElement).value)} />
					</label>
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">Paket Top Up (tc:idr atau tc:idr:bonus, pisahkan koma — contoh 500:500000,1000:1000000:50)</span>
						<input class="app-input !w-[380px]" bind:value={packages} />
					</label>
					<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void saveConfig()}>{tr("Simpan konfigurasi")}</button>
				</div>
			</div>

			<div class="app-card mt-4 p-5">
				<h2 class="text-sm font-semibold">Penyesuaian Manual (Ledger)</h2>
				<p class="mt-1 text-xs text-muted-foreground">{tr("Penyesuaian tercatat di ledger + audit log dengan alasan. Debit tidak bisa membuat saldo available negatif.")}</p>
				<div class="mt-3 flex flex-wrap items-end gap-3">
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">Brand (ID)</span>
						<Select
							ariaLabel="Brand (ID)"
							placeholder={tr("Pilih brand…")}
							minWidth={240}
							value={adjBrand}
							onchange={(v) => (adjBrand = v)}
							options={[
								{ value: '', label: tr('Pilih brand…') },
								...wallets.map((w) => ({ value: w.brand_id, label: w.brand_name ?? w.client_name ?? w.brand_id }))
							]}
						/>
					</label>
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Jumlah Saldo")}</span>
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
								{ value: 'credit', label: tr('Kredit (+ saldo)') },
								{ value: 'debit', label: 'Debit (−saldo)' }
							]}
						/>
					</label>
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">Alasan</span>
						<input class="app-input !w-[260px]" placeholder={tr("Contoh: kompensasi pelanggan")} bind:value={adjReason} />
					</label>
					<button
						class="app-btn app-btn-primary"
						type="button"
						disabled={busy || !adjBrand || !adjTc || adjReason.trim().length < 3}
						onclick={() => void adjust(adjBrand)}
					>Terapkan</button>
				</div>
			</div>
		{/if}

		<div class="mt-4 flex justify-end">
			<label class="app-input !w-[240px] !py-1.5">
				<input class="w-full bg-transparent text-sm outline-none" placeholder={tr("Cari nama klien...")} value={q} oninput={(e) => onSearch((e.currentTarget as HTMLInputElement).value)} />
			</label>
		</div>
		<div class="app-card mt-3 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 980px;">
				<thead>
					<tr class="text-xs text-muted-foreground">
						<th class="px-4 py-2.5 text-left font-medium">{tr("Brand / Klien")}</th>
						{#each [{ label: 'Sisa Saldo', tip: TIPS.available }, { label: 'Ditahan', tip: TIPS.reserved }, { label: 'Terpakai', tip: TIPS.captured }, { label: 'Total Saldo', tip: TIPS.total }, { label: 'Top Up', tip: TIPS.topup }, { label: 'Publikasi', tip: TIPS.pub }] as col (col.label)}
							<th class="px-4 py-2.5 text-right font-medium">
								<span class="inline-flex items-center gap-1">
									{tr(col.label)}
									<Tooltip content={tr(col.tip)} side="bottom" align="end" maxWidth={300}>
										<button type="button" class="text-slate-400 transition-colors hover:text-slate-600" aria-label={tr(col.tip)}>
											<Info size={12} aria-hidden="true" />
										</button>
									</Tooltip>
								</span>
							</th>
						{/each}
					</tr>
				</thead>
				<tbody>
					{#each visibleWallets as w (w.brand_id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<p class="font-bold">{w.brand_name ?? '—'}</p>
								<p class="text-xs text-muted-foreground">{w.client_name ?? w.brand_id}</p>
							</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{rp(w.available_tc)}</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{rp(w.reserved_tc)}</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{rp(w.captured_tc)}</td>
							<td class="px-4 py-2.5 text-right font-bold tabular-nums">{rp(w.total_tc)}</td>
							<td class="px-4 py-2.5 text-right tabular-nums">
								{#if w.topup_tc > 0}
									{rp(w.topup_tc)}
								{:else}
									—
								{/if}
							</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{w.pub_tc > 0 ? rp(w.pub_tc) : '—'}</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && wallets.length === 0}
				<div class="flex flex-col items-center px-4 py-12 text-center">
					<Coins size={26} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">{tr("Belum ada wallet. Wallet dibuat otomatis saat Client melakukan Top Up atau publikasi pertama.")}</p>
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
