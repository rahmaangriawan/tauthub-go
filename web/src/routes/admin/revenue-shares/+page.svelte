<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Revenue share — port app/admin/revenue-shares/page.tsx: pembagian MARKUP
	// Tauthub (client − harga mitra). Prioritas: media+partner → media → partner
	// → default 75/25. Tab kedua: Markup Harga Klien (pricing_config).
	import { onMount } from 'svelte';
	import { Award, CircleCheck, Coins, Percent, TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtRp, fmtNum } from '$lib/admin-format';
	import { platformError, isAccessDenied } from '../_platform';
	import { loyaltyTierColor } from '$lib/loyalty';

	type Rule = {
		id: string; name: string; partner_id: string | null; partner_name: string | null;
		media_id: string | null; media_name: string | null; share_type: string;
		tulisio_percent: number; partner_percent: number; partner_fixed: number;
		status: string; created_at: string;
	};
	type Partner = { id: string; name: string | null; email: string; status: string };
	type Media = { id: string; name: string; status: string };
	type PricingConfig = { markup_percent: number; default_markup_percent: number };

	const PAGE_SIZE = 20;

	let rows = $state<Rule[]>([]);
	let total = $state(0);
	let partners = $state<Partner[]>([]);
	let media = $state<Media[]>([]);
	let error = $state('');
	let loading = $state(true);
	let pageNum = $state(1);
	let tab = $state<'rules' | 'finance' | 'loyalty'>('rules');
	let createOpen = $state(false);
	let busy = $state(false);
	let form = $state({
		name: '', partner_id: '', media_id: '', share_type: 'percentage',
		tulisio_percent: '75', partner_percent: '25', partner_fixed: '0'
	});

	// Tab markup
	let markup = $state<number | null>(null);
	let defaultMarkup = $state(15);
	let markupLoading = $state(true);
	let markupBusy = $state(false);
	let markupMsg = $state('');

	// Tab loyalty tiers
	type Tier = { name: string; min_spend_tc: number; discount_percent: number; sort_order: number };
	let tiers = $state<Tier[]>([]);
	let tiersLoading = $state(true);
	let tierBusy = $state(false);
	let tierMsg = $state('');
	let tierForm = $state({ name: '', min_spend_tc: '', discount_percent: '', sort_order: '' });

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
			await loadPricing();
			await loadTiers();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		loading = true;
		error = '';
		const offset = (pageNum - 1) * PAGE_SIZE;
		const [ruleRes, partnerRes, mediaRes] = await Promise.all([
			apiRequest(`/v1/platform/revenue-shares?limit=${PAGE_SIZE}&offset=${offset}`),
			apiRequest('/v1/platform/partners?limit=500'),
			apiRequest('/v1/platform/media?limit=500')
		]);
		if (ruleRes.ok) {
			const data = await ruleRes.json();
			rows = data.revenue_shares ?? [];
			total = data.total ?? 0;
		} else {
			error = await platformError(ruleRes);
		}
		if (partnerRes.ok) {
			const data = await partnerRes.json();
			partners = data.partners ?? [];
		}
		if (mediaRes.ok) {
			const data = await mediaRes.json();
			media = data.media ?? [];
		}
		loading = false;
	}

	async function loadPricing() {
		markupLoading = true;
		const res = await apiRequest('/v1/platform/pricing');
		if (res.ok) {
			const data: PricingConfig = await res.json();
			markup = data.markup_percent;
			defaultMarkup = data.default_markup_percent ?? 15;
		} else {
			markupMsg = await platformError(res);
		}
		markupLoading = false;
	}

	async function create(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		const res = await apiRequest('/v1/platform/revenue-shares', {
			method: 'POST',
			body: JSON.stringify({
				name: form.name.trim(),
				partner_id: form.partner_id || null,
				media_id: form.media_id || null,
				share_type: form.share_type,
				tulisio_percent: Number(form.tulisio_percent) || 0,
				partner_percent: Number(form.partner_percent) || 0,
				partner_fixed: Number(form.partner_fixed) || 0
			})
		});
		if (!res.ok) {
			error = await platformError(res);
			busy = false;
			return;
		}
		createOpen = false;
		form = {
			name: '', partner_id: '', media_id: '', share_type: 'percentage',
			tulisio_percent: '75', partner_percent: '25', partner_fixed: '0'
		};
		busy = false;
		await load();
	}

	async function archive(rule: Rule) {
		error = '';
		const res = await apiRequest(`/v1/platform/revenue-shares/${rule.id}`, {
			method: 'PUT',
			body: JSON.stringify({ status: 'archived' })
		});
		if (!res.ok) {
			error = await platformError(res);
			return;
		}
		await load();
	}

	async function saveMarkup(e: SubmitEvent) {
		e.preventDefault();
		if (markup == null) return;
		markupBusy = true;
		markupMsg = '';
		const res = await apiRequest('/v1/platform/pricing', {
			method: 'PUT',
			body: JSON.stringify({ markup_percent: Number(markup) })
		});
		if (!res.ok) {
			markupMsg = await platformError(res);
			markupBusy = false;
			return;
		}
		const r = await res.json();
		markup = r.markup_percent;
		markupMsg = 'Persentase markup berhasil disimpan — harga klien & kredit di semua halaman langsung memakai nilai baru.';
		markupBusy = false;
	}

	async function loadTiers() {
		tiersLoading = true;
		const res = await apiRequest('/v1/platform/loyalty-tiers');
		if (res.ok) {
			const data = await res.json();
			tiers = data.loyalty_tiers ?? [];
		} else {
			tierMsg = await platformError(res);
		}
		tiersLoading = false;
	}

	async function upsertTier(t: { name: string; min_spend_tc: number | string; discount_percent: number | string; sort_order: number | string }) {
		tierBusy = true;
		tierMsg = '';
		const res = await apiRequest('/v1/platform/loyalty-tiers', {
			method: 'PUT',
			body: JSON.stringify({
				name: String(t.name).trim(),
				min_spend_tc: Number(t.min_spend_tc) || 0,
				discount_percent: Number(t.discount_percent) || 0,
				sort_order: Number(t.sort_order) || 0
			})
		});
		if (!res.ok) {
			tierMsg = await platformError(res);
			tierBusy = false;
			return false;
		}
		tierBusy = false;
		await loadTiers();
		return true;
	}

	async function saveTier(t: Tier) {
		if (await upsertTier(t)) tierMsg = `Tier ${t.name} tersimpan.`;
	}

	async function addTier(e: SubmitEvent) {
		e.preventDefault();
		if (!tierForm.name.trim()) {
			tierMsg = 'Nama tier wajib diisi.';
			return;
		}
		if (await upsertTier(tierForm)) {
			tierMsg = `Tier ${tierForm.name.trim()} tersimpan.`;
			tierForm = { name: '', min_spend_tc: '', discount_percent: '', sort_order: '' };
		}
	}

	async function deleteTier(name: string) {
		if (!confirm(`Hapus tier ${name}?`)) return;
		tierMsg = '';
		const res = await apiRequest(`/v1/platform/loyalty-tiers/${encodeURIComponent(name)}`, { method: 'DELETE' });
		if (!res.ok) {
			tierMsg = await platformError(res);
			return;
		}
		await loadTiers();
	}

	const markupPreviewBase = 50000;
	const markupPreview = $derived(
		markup != null && Number.isFinite(markup) && markup >= 0
			? Math.round(markupPreviewBase * (1 + markup / 100))
			: null
	);
	const denied = $derived(Boolean(!loading && rows.length === 0 && isAccessDenied(error)));
	const scopeLabel = (row: Rule) =>
		row.media_name ? `Media: ${row.media_name}` : row.partner_name ? `Mitra: ${row.partner_name}` : 'Platform (default)';
</script>

<section>
	<PageHeader
		title="Revenue Shares"
		description={tr("Pembagian MARKUP Tauthub (client − harga mitra). Mitra selalu dapat harga penuhnya + share markup. Prioritas: media+partner → media → partner → default 75/25.")}
	>
		<span class="rounded-full px-3 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{fmtNum(total)} rule</span>
		<button class="app-btn app-btn-primary" type="button" onclick={() => (createOpen = true)}><Percent size={16} />{tr("Buat rule")}</button>
	</PageHeader>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke pengelolaan revenue share.")}</p>
		</div>
	{:else}
		<Feedback {loading} {error} loadingText="Memuat revenue share…" />

		<div class="mt-4 flex flex-wrap gap-1 border-b">
			<button
				class="flex items-center gap-2 border-b-2 px-3 py-2 text-sm font-medium"
				style="border-color: {tab === 'rules' ? 'var(--primary)' : 'transparent'}; color: {tab === 'rules' ? 'var(--primary)' : 'var(--muted-foreground)'};"
				type="button"
				onclick={() => (tab = 'rules')}
			><Percent size={15} /> Rule Pembagian</button>
			<button
				class="flex items-center gap-2 border-b-2 px-3 py-2 text-sm font-medium"
				style="border-color: {tab === 'finance' ? 'var(--primary)' : 'transparent'}; color: {tab === 'finance' ? 'var(--primary)' : 'var(--muted-foreground)'};"
				type="button"
				onclick={() => (tab = 'finance')}
			><Coins size={15} />{tr("Markup Harga Klien")}</button>
			<button
				class="flex items-center gap-2 border-b-2 px-3 py-2 text-sm font-medium"
				style="border-color: {tab === 'loyalty' ? 'var(--primary)' : 'transparent'}; color: {tab === 'loyalty' ? 'var(--primary)' : 'var(--muted-foreground)'};"
				type="button"
				onclick={() => (tab = 'loyalty')}
			><Award size={15} />{tr("Loyalty Tier")}</button>
		</div>

		{#if tab === 'rules'}
			<div class="app-card mt-3 overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 1000px;">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="px-4 py-2.5 font-medium">Rule</th>
							<th class="px-4 py-2.5 font-medium">Skope</th>
							<th class="px-4 py-2.5 font-medium">Tipe</th>
							<th class="px-4 py-2.5 font-medium">Split</th>
							<th class="px-4 py-2.5 font-medium">Status</th>
							<th class="px-4 py-2.5 font-medium">Aksi</th>
						</tr>
					</thead>
					<tbody>
						{#each rows as row (row.id)}
							<tr class="border-t">
								<td class="px-4 py-2.5">
									<div class="flex items-center gap-2">
										<Percent size={16} class="shrink-0 text-muted-foreground" />
										<div>
											<p class="font-medium">{row.name}</p>
											<p class="text-xs text-muted-foreground">{row.id.slice(0, 8)}</p>
										</div>
									</div>
								</td>
								<td class="px-4 py-2.5">{scopeLabel(row)}</td>
								<td class="px-4 py-2.5"><Badge label={row.share_type} tone="neutral" /></td>
								<td class="px-4 py-2.5">
									{row.share_type === 'percentage'
										? `Tauthub ${row.tulisio_percent}% / Mitra ${row.partner_percent}% (dari markup)`
										: `Mitra +${fmtRp(row.partner_fixed)} (dari markup)`}
								</td>
								<td class="px-4 py-2.5"><Badge label={row.status} tone={row.status === 'active' ? 'success' : 'neutral'} /></td>
								<td class="px-4 py-2.5">
									{#if row.status === 'active'}
										<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void archive(row)}>Arsipkan</button>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
				{#if !loading && rows.length === 0}
					<div class="flex flex-col items-center px-4 py-12 text-center">
						<Percent size={26} class="text-muted-foreground" />
						<p class="mt-2 text-sm text-muted-foreground">{tr("Belum ada rule revenue share.")}</p>
					</div>
				{/if}
				<div class="px-4 pb-4">
					<Pagination
						total={total}
						limit={PAGE_SIZE}
						offset={(pageNum - 1) * PAGE_SIZE}
						onChange={(next) => { pageNum = Math.floor(next / PAGE_SIZE) + 1; void load(); }}
					/>
				</div>
			</div>
		{:else if tab === 'finance'}
			<div class="app-card mt-3 p-5">
				{#if markupLoading}
					<p class="text-sm text-muted-foreground">{tr("Memuat konfigurasi finance…")}</p>
				{:else}
					<p class="text-sm text-muted-foreground">{tr("Harga klien = harga mitra × (1 + markup%). Nilai disimpan sekali di database dan dibaca lewat cache server (bukan query berat tiap halaman) — katalog klien, kredit saldo, dan checkout langsung memakai nilai baru begitu disimpan. Publikasi yang sudah dibuat tidak berubah (harganya ter-snapshot saat dibuat).")}</p>
					<div class="my-4 border-t"></div>
					<form class="flex max-w-[420px] flex-col gap-3" onsubmit={saveMarkup}>
						<label class="block">
							<span class="mb-1 block text-xs font-semibold text-muted-foreground">Persentase markup (%)</span>
							<input
								class="app-input w-full"
								type="number"
								min="0"
								step="0.01"
								required
								placeholder={String(defaultMarkup)}
								value={markup ?? ''}
								oninput={(e) => {
									const v = (e.currentTarget as HTMLInputElement).value;
									markup = v === '' ? null : Number(v);
								}}
							/>
						</label>
						<p class="text-sm text-muted-foreground"> {tr("Contoh: media")} {fmtRp(markupPreviewBase)} → klien <strong>{markupPreview != null ? fmtRp(markupPreview) : '—'}</strong>
							{markup != null && markup > 0 ? ` (markup ${markup}%)` : ' (tanpa markup)'}
						</p>
						{#if markupMsg}
							<div class="flex items-start gap-2 text-sm" style="color: {markupMsg.includes('berhasil') ? 'var(--success)' : 'var(--destructive)'};">
								{#if markupMsg.includes('berhasil')}<CircleCheck size={16} class="mt-0.5 shrink-0" />{:else}<TriangleAlert size={16} class="mt-0.5 shrink-0" />{/if}
								{markupMsg}
							</div>
						{/if}
						<div class="flex items-center gap-2">
							<button class="app-btn app-btn-primary" type="submit" disabled={markupBusy || markup == null}>
								{markupBusy ? 'Menyimpan…' : tr('Simpan Markup')}
							</button>
							{#if markup != null && markup !== defaultMarkup}
								<button class="app-btn app-btn-soft" type="button" disabled={markupBusy} onclick={() => (markup = defaultMarkup)}> {tr("Kembalikan ke default (")}{defaultMarkup}%)
								</button>
							{/if}
						</div>
					</form>
				{/if}
			</div>
		{:else if tab === 'loyalty'}
			<div class="app-card mt-3 p-5">
				<p class="text-sm text-muted-foreground">{tr("Diskon loyalitas berdasarkan total belanja seumur hidup brand (TC ≈ rupiah). Diskon memotong markup Tauthub; harga partner tetap. Hanya tampil di checkout & saat charge, bukan di harga katalog.")}</p>
				<div class="my-4 border-t"></div>
				{#if tiersLoading}
					<p class="text-sm text-muted-foreground">{tr("Memuat tier…")}</p>
				{:else}
					<div class="overflow-x-auto">
						<table class="w-full text-sm" style="min-width: 640px;">
							<thead>
								<tr class="text-left text-xs text-muted-foreground">
									<th class="px-3 py-2 font-medium">{tr("Tier")}</th>
									<th class="px-3 py-2 font-medium">{tr("Min. belanja (TC)")}</th>
									<th class="px-3 py-2 font-medium">{tr("Diskon (%)")}</th>
									<th class="px-3 py-2 font-medium">{tr("Urutan")}</th>
									<th class="px-3 py-2 font-medium">{tr("Aksi")}</th>
								</tr>
							</thead>
							<tbody>
								{#each tiers as t (t.name)}
									<tr class="border-t">
										<td class="px-3 py-2">
											<span
												class="inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs font-bold"
												style="color: {loyaltyTierColor(t.name)}; background-color: color-mix(in srgb, {loyaltyTierColor(t.name)} 15%, transparent);"
											>
												<Award size={12} />{t.name}{#if Number(t.discount_percent) > 0} · −{t.discount_percent}%{/if}
											</span>
										</td>
										<td class="px-3 py-2"><input class="app-input w-36" type="number" min="0" step="1" bind:value={t.min_spend_tc} /></td>
										<td class="px-3 py-2"><input class="app-input w-24" type="number" min="0" max="100" step="0.01" bind:value={t.discount_percent} /></td>
										<td class="px-3 py-2"><input class="app-input w-20" type="number" step="1" bind:value={t.sort_order} /></td>
										<td class="px-3 py-2">
											<div class="flex gap-2">
												<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" disabled={tierBusy} onclick={() => void saveTier(t)}>{tr("Simpan")}</button>
												<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void deleteTier(t.name)}>{tr("Hapus")}</button>
											</div>
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
					{#if tiers.length === 0}
						<p class="mt-3 text-sm text-muted-foreground">{tr("Belum ada tier.")}</p>
					{/if}

					<div class="my-4 border-t"></div>
					<p class="mb-2 text-xs font-semibold text-muted-foreground">{tr("Tambah / perbarui tier")}</p>
					<form class="flex flex-wrap items-end gap-3" onsubmit={addTier}>
						<label class="block"><span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Nama tier")}</span><input class="app-input w-40" bind:value={tierForm.name} placeholder="Gold" maxlength="40" /></label>
						<label class="block"><span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Min. belanja (TC)")}</span><input class="app-input w-40" type="number" min="0" step="1" bind:value={tierForm.min_spend_tc} placeholder="50000000" /></label>
						<label class="block"><span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Diskon (%)")}</span><input class="app-input w-28" type="number" min="0" max="100" step="0.01" bind:value={tierForm.discount_percent} placeholder="6" /></label>
						<label class="block"><span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Urutan")}</span><input class="app-input w-20" type="number" step="1" bind:value={tierForm.sort_order} placeholder="3" /></label>
						<button class="app-btn app-btn-primary" type="submit" disabled={tierBusy}>{tierBusy ? tr('Menyimpan…') : tr('Simpan Tier')}</button>
					</form>
					{#if tierMsg}
						<div class="mt-3 flex items-start gap-2 text-sm" style="color: {tierMsg.includes('tersimpan') ? 'var(--success)' : 'var(--destructive)'};">
							{#if tierMsg.includes('tersimpan')}<CircleCheck size={16} class="mt-0.5 shrink-0" />{:else}<TriangleAlert size={16} class="mt-0.5 shrink-0" />{/if}
							{tierMsg}
						</div>
					{/if}
				{/if}
			</div>
		{/if}
	{/if}
</section>

{#if createOpen}
	<Modal title={tr("Buat rule revenue share")} label={tr("Buat rule revenue share")} maxWidth="lg" onClose={() => (createOpen = false)}>
		<form class="flex flex-col gap-3" onsubmit={create}>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Nama rule")}</span>
				<input class="app-input w-full" required placeholder="Default 75/25" bind:value={form.name} />
			</label>
			<div class="grid gap-3 sm:grid-cols-2">
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Mitra (opsional)")}</span>
					<Select
						ariaLabel={tr("Mitra")}
						minWidth={240}
						value={form.partner_id}
						onchange={(v) => (form.partner_id = v)}
						options={[
							{ value: '', label: tr('Semua mitra') },
							...partners.filter((p) => p.status === 'active').map((p) => ({ value: p.id, label: p.name ?? p.email }))
						]}
					/>
				</label>
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Media (opsional)")}</span>
					<Select
						ariaLabel={tr("Media")}
						minWidth={240}
						value={form.media_id}
						onchange={(v) => (form.media_id = v)}
						options={[
							{ value: '', label: tr('Semua media') },
							...media.filter((m) => m.status === 'active').map((m) => ({ value: m.id, label: m.name }))
						]}
					/>
				</label>
			</div>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Tipe</span>
				<Select
					ariaLabel="Tipe"
					minWidth={200}
					value={form.share_type}
					onchange={(v) => (form.share_type = v)}
					options={[
						{ value: 'percentage', label: 'Percentage' },
						{ value: 'fixed', label: 'Fixed' }
					]}
				/>
			</label>
			{#if form.share_type === 'percentage'}
				<div class="grid gap-3 sm:grid-cols-2">
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">Tauthub %</span>
						<input class="app-input w-full" type="number" min="0" max="100" value={form.tulisio_percent} oninput={(e) => (form = { ...form, tulisio_percent: (e.currentTarget as HTMLInputElement).value })} />
					</label>
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Mitra %")}</span>
						<input class="app-input w-full" type="number" min="0" max="100" value={form.partner_percent} oninput={(e) => (form = { ...form, partner_percent: (e.currentTarget as HTMLInputElement).value })} />
					</label>
				</div>
			{:else}
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Mitra (fixed, IDR) — di-cap di harga client")}</span>
					<input
						class="app-input w-full"
						type="text"
						inputmode="numeric"
						value={form.partner_fixed ? Number(form.partner_fixed).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' }) : ''}
						oninput={(e) => (form = { ...form, partner_fixed: (e.currentTarget as HTMLInputElement).value.replace(/\D/g, '') })}
					/>
				</label>
			{/if}
			{#if error}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{error}</div>
			{/if}
			<div class="mt-1 flex justify-end gap-2 border-t pt-4">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (createOpen = false)}>{tr("Batal")}</button>
				<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? 'Menyimpan…' : tr('Buat rule')}</button>
			</div>
		</form>
	</Modal>
{/if}
