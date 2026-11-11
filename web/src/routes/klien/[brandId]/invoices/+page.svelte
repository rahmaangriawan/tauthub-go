<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Invoice & Pajak (read-only; pembuatan invoice dari admin).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Download, LoaderCircle, Receipt } from '@lucide/svelte';
	import { downloadInvoicePdf, formatNpwp } from '$lib/invoice-pdf';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import Select from '$lib/components/select.svelte';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);
	type Invoice = {
		id: string; number: string; status: string; amount: number; campaign_name: string | null;
		issued_at: string | null; due_at: string | null; paid_at: string | null; notes: string | null;
		po_number: string | null;
		bill_to: { company_name?: string; npwp?: string; npwp_address?: string } | null;
	};

	// Profil penagihan: disimpan sekali, dipakai mengisi form top-up berfaktur.
	let profile = $state({ company_name: '', npwp: '', npwp_address: '', billing_email: '' });
	let profileVisible = $state(true);
	let profileSaving = $state(false);
	let profileError = $state('');
	let profileNotice = $state('');
	let pdfBusy = $state<string | null>(null);

	async function loadProfile() {
		const res = await apiRequest(`/v1/brands/${brandId}/billing-profile`);
		if (res.status === 403) {
			profileVisible = false;
			return;
		}
		if (!res.ok) return;
		const j = await res.json();
		if (j.profile) {
			profile = {
				company_name: j.profile.company_name ?? '', npwp: formatNpwp(j.profile.npwp),
				npwp_address: j.profile.npwp_address ?? '', billing_email: j.profile.billing_email ?? ''
			};
		}
	}

	async function saveProfile() {
		profileSaving = true;
		profileError = '';
		profileNotice = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/billing-profile`, { method: 'PUT', body: JSON.stringify(profile) });
			if (!res.ok) {
				profileError = await readApiError(res, tr('Gagal menyimpan data penagihan.'));
				return;
			}
			profileNotice = tr('Data penagihan disimpan.');
			await loadProfile();
		} finally {
			profileSaving = false;
		}
	}

	async function pdf(inv: Invoice) {
		pdfBusy = inv.id;
		try {
			await downloadInvoicePdf(inv);
		} finally {
			pdfBusy = null;
		}
	}

	let invoices = $state<Invoice[]>([]);
	let total = $state(0);
	let status = $state('');
	let loading = $state(true);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([load(), loadProfile()]);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		const params = new URLSearchParams({ limit: '100' });
		if (status) params.set('status', status);
		const res = await apiRequest(`/v1/brands/${brandId}/invoices?${params}`);
		if (res.ok) {
			const data = await res.json();
			invoices = data.invoices ?? [];
			total = data.total ?? 0;
		}
	}

	const statusLabel: Record<string, string> = $derived({ draft: 'Draf', issued: 'Diterbitkan', paid: 'Lunas', void: tr('Dibatalkan') });
	const stateStyle = (s: string) =>
		s === 'paid' ? 'background-color: var(--success-soft); color: var(--success);'
		: s === 'issued' ? 'background-color: var(--warning-soft); color: var(--warning);'
		: 'background-color: var(--muted); color: var(--muted-foreground);';
	const fmt = (n: number) => n.toLocaleString('id-ID', { maximumFractionDigits: 0 });
	const fmtDate = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID') : '—');
</script>

<PageTitle title="Invoice & Pajak" />

<section>
	<header class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<h1 class="page-title">Invoice & Pajak</h1>
			<p class="page-desc">{total} {tr("invoice tercatat untuk brand ini.")}</p>
		</div>
		<Select
			bind:value={status}
			ariaLabel="Status invoice"
			placeholder={tr("Semua status")}
			align="end"
			onchange={() => load()}
			options={[
				{ value: '', label: tr('Semua status') },
				{ value: 'draft', label: 'Draf' },
				{ value: 'issued', label: 'Diterbitkan' },
				{ value: 'paid', label: 'Lunas' },
				{ value: 'void', label: tr('Dibatalkan') }
			]}
		/>
	</header>

	{#if profileVisible}
		<div class="app-card mt-4 p-5">
			<h2 class="text-base font-semibold">{tr('Data penagihan')}</h2>
			<p class="mt-1 text-sm text-muted-foreground">{tr('Disimpan sekali dan mengisi otomatis form top up berfaktur. NPWP hanya terlihat oleh anggota yang boleh mengisi saldo.')}</p>
			<div class="mt-4 grid gap-3 sm:grid-cols-2">
				<label class="grid gap-1 text-sm">{tr('Nama perusahaan (sesuai NPWP)')}
					<input class="app-input" bind:value={profile.company_name} maxlength="200" /></label>
				<label class="grid gap-1 text-sm">{tr('Nomor NPWP')}
					<input class="app-input" inputmode="numeric" bind:value={profile.npwp} placeholder="00.000.000.0-000.000" /></label>
				<label class="grid gap-1 text-sm sm:col-span-2">{tr('Alamat sesuai NPWP')}
					<textarea class="app-input" rows="2" bind:value={profile.npwp_address} maxlength="500"></textarea></label>
				<label class="grid gap-1 text-sm sm:col-span-2">{tr('Email tagihan (opsional)')}
					<input class="app-input" type="email" bind:value={profile.billing_email} maxlength="254" /></label>
			</div>
			{#if profileError}<p class="mt-3 text-sm" style="color: var(--destructive);">{profileError}</p>{/if}
			{#if profileNotice}<p class="mt-3 text-sm" style="color: var(--success);">{profileNotice}</p>{/if}
			<button class="app-btn app-btn-primary mt-4 min-h-11" type="button" disabled={profileSaving} onclick={() => void saveProfile()}>
				{#if profileSaving}<LoaderCircle size={15} class="animate-spin" />{/if}
				{tr('Simpan data penagihan')}
			</button>
		</div>
	{/if}

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat invoice…")}</p>
	{:else if invoices.length === 0}
		<div class="app-card mt-6 flex flex-col items-center px-6 py-12 text-center">
			<Receipt size={22} style="color: var(--primary);" />
			<p class="mt-2 text-sm font-medium">{status ? tr('Tidak ada invoice dengan status ini.') : tr('Belum ada invoice.')}</p>
			<p class="mt-1 text-sm text-muted-foreground">{tr("Invoice diterbitkan admin setelah kampanye atau pembelian paket.")}</p>
		</div>
	{:else}
		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="border-b text-left text-[11px] tracking-wide text-muted-foreground">
						<th class="px-4 py-2.5 font-semibold">NOMOR</th>
						<th class="px-4 py-2.5 font-semibold">KAMPANYE</th>
						<th class="px-4 py-2.5 font-semibold">STATUS</th>
						<th class="px-4 py-2.5 text-right font-semibold">JUMLAH</th>
						<th class="px-4 py-2.5 font-semibold">NOMOR PO</th>
						<th class="px-4 py-2.5 font-semibold">JATUH TEMPO</th>
						<th class="px-4 py-2.5"></th>
					</tr>
				</thead>
				<tbody>
					{#each invoices as inv (inv.id)}
						<tr class="border-b last:border-0">
							<td class="px-4 py-2.5 font-medium">{inv.number}</td>
							<td class="px-4 py-2.5 text-muted-foreground">{inv.campaign_name ?? '—'}</td>
							<td class="px-4 py-2.5">
								<span class="rounded-full px-2 py-0.5 text-xs font-semibold" style={stateStyle(inv.status)}>
									{statusLabel[inv.status] ?? inv.status}
								</span>
							</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{fmt(inv.amount)}</td>
							<td class="px-4 py-2.5 text-muted-foreground">{inv.po_number ?? '-'}</td>
							<td class="px-4 py-2.5 text-muted-foreground">{fmtDate(inv.due_at)}</td>
							<td class="px-4 py-2.5 text-right">
								<button class="app-btn app-btn-soft min-h-11" type="button" disabled={pdfBusy === inv.id} onclick={() => void pdf(inv)}>
									{#if pdfBusy === inv.id}<LoaderCircle size={14} class="animate-spin" />{:else}<Download size={14} />{/if}
									PDF
								</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>
