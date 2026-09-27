<script lang="ts">
	// Revenue share — port app/admin/revenue-shares/page.tsx: pembagian MARKUP
	// Tauthub (client − harga mitra). Prioritas: media+partner → media → partner
	// → default 75/25. Tab kedua: Markup Harga Klien (pricing_config).
	import { onMount } from 'svelte';
	import { CircleCheck, Coins, Percent, TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtRp, fmtNum } from '$lib/admin-format';
	import { platformError, isAccessDenied } from '../_platform';

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
	let tab = $state<'rules' | 'finance'>('rules');
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

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
			await loadPricing();
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

<section class="mx-auto max-w-7xl">
	<PageHeader
		title="Revenue Shares"
		description="Pembagian MARKUP Tauthub (client − harga mitra). Mitra selalu dapat harga penuhnya + share markup. Prioritas: media+partner → media → partner → default 75/25."
	>
		<span class="rounded-full px-3 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{fmtNum(total)} rule</span>
		<button class="app-btn app-btn-primary" type="button" onclick={() => (createOpen = true)}><Percent size={16} /> Buat rule</button>
	</PageHeader>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke pengelolaan revenue share.</p>
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
			><Coins size={15} /> Markup Harga Klien</button>
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
						<p class="mt-2 text-sm text-muted-foreground">Belum ada rule revenue share.</p>
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
		{:else}
			<div class="app-card mt-3 p-5">
				{#if markupLoading}
					<p class="text-sm text-muted-foreground">Memuat konfigurasi finance…</p>
				{:else}
					<p class="text-sm text-muted-foreground">
						Harga klien = harga mitra × (1 + markup%). Nilai disimpan sekali di database dan dibaca
						lewat cache server (bukan query berat tiap halaman) — katalog klien, kredit TC, dan
						checkout langsung memakai nilai baru begitu disimpan. Publikasi yang sudah dibuat tidak
						berubah (harganya ter-snapshot saat dibuat).
					</p>
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
						<p class="text-sm text-muted-foreground">
							Contoh: media {fmtRp(markupPreviewBase)} → klien <strong>{markupPreview != null ? fmtRp(markupPreview) : '—'}</strong>
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
								{markupBusy ? 'Menyimpan…' : 'Simpan Markup'}
							</button>
							{#if markup != null && markup !== defaultMarkup}
								<button class="app-btn app-btn-soft" type="button" disabled={markupBusy} onclick={() => (markup = defaultMarkup)}>
									Kembalikan ke default ({defaultMarkup}%)
								</button>
							{/if}
						</div>
					</form>
				{/if}
			</div>
		{/if}
	{/if}
</section>

{#if createOpen}
	<Modal title="Buat rule revenue share" label="Buat rule revenue share" maxWidth="lg" onClose={() => (createOpen = false)}>
		<form class="flex flex-col gap-3" onsubmit={create}>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Nama rule</span>
				<input class="app-input w-full" required placeholder="Default 75/25" bind:value={form.name} />
			</label>
			<div class="grid gap-3 sm:grid-cols-2">
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Mitra (opsional)</span>
					<select class="app-input w-full" bind:value={form.partner_id}>
						<option value="">Semua mitra</option>
						{#each partners.filter((p) => p.status === 'active') as p (p.id)}
							<option value={p.id}>{p.name ?? p.email}</option>
						{/each}
					</select>
				</label>
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Media (opsional)</span>
					<select class="app-input w-full" bind:value={form.media_id}>
						<option value="">Semua media</option>
						{#each media.filter((m) => m.status === 'active') as m (m.id)}
							<option value={m.id}>{m.name}</option>
						{/each}
					</select>
				</label>
			</div>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Tipe</span>
				<select class="app-input w-full" bind:value={form.share_type}>
					<option value="percentage">Percentage</option>
					<option value="fixed">Fixed</option>
				</select>
			</label>
			{#if form.share_type === 'percentage'}
				<div class="grid gap-3 sm:grid-cols-2">
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">Tauthub %</span>
						<input class="app-input w-full" type="number" min="0" max="100" value={form.tulisio_percent} oninput={(e) => (form = { ...form, tulisio_percent: (e.currentTarget as HTMLInputElement).value })} />
					</label>
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">Mitra %</span>
						<input class="app-input w-full" type="number" min="0" max="100" value={form.partner_percent} oninput={(e) => (form = { ...form, partner_percent: (e.currentTarget as HTMLInputElement).value })} />
					</label>
				</div>
			{:else}
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Mitra (fixed, IDR) — di-cap di harga client</span>
					<input
						class="app-input w-full"
						type="text"
						inputmode="numeric"
						value={form.partner_fixed ? Number(form.partner_fixed).toLocaleString('id-ID') : ''}
						oninput={(e) => (form = { ...form, partner_fixed: (e.currentTarget as HTMLInputElement).value.replace(/\D/g, '') })}
					/>
				</label>
			{/if}
			{#if error}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{error}</div>
			{/if}
			<div class="mt-1 flex justify-end gap-2 border-t pt-4">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (createOpen = false)}>Batal</button>
				<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? 'Menyimpan…' : 'Buat rule'}</button>
			</div>
		</form>
	</Modal>
{/if}
