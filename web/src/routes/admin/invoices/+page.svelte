<script lang="ts">
	// Invoice — port app/admin/invoices/page.tsx: tagihan ke brand (manual atau
	// otomatis dari campaign = Σ harga publikasi published). Aksi: Issue,
	// Tandai bayar, Void. Form buat invoice memuat campaign brand terpilih.
	import { onMount } from 'svelte';
	import { Receipt, TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDate, fmtRp, fmtNum } from '$lib/admin-format';
	import { platformError, isAccessDenied } from '../_platform';

	type Invoice = {
		id: string; brand_id: string; brand_name: string | null; campaign_id: string | null;
		number: string; status: string; amount: number; issued_at: string | null;
		due_at: string | null; paid_at: string | null; notes: string | null; created_at: string;
	};
	type Brand = { id: string; name: string; slug: string };
	type Campaign = { id: string; name: string; status: string; publication_count: number };

	const PAGE_SIZE = 20;
	const STATUS_TONE: Record<string, 'neutral' | 'warning' | 'success' | 'danger'> = {
		draft: 'neutral', issued: 'warning', paid: 'success', void: 'danger'
	};

	let rows = $state<Invoice[]>([]);
	let total = $state(0);
	let brands = $state<Brand[]>([]);
	let campaigns = $state<Campaign[]>([]);
	let error = $state('');
	let loading = $state(true);
	let pageNum = $state(1);
	let createOpen = $state(false);
	let busy = $state(false);
	let form = $state({
		brand_id: '', campaign_id: '', amount: '', status: 'draft',
		issued_at: '', due_at: '', notes: ''
	});

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

	async function load() {
		loading = true;
		error = '';
		const offset = (pageNum - 1) * PAGE_SIZE;
		const [invRes, brandRes] = await Promise.all([
			apiRequest(`/v1/platform/invoices?limit=${PAGE_SIZE}&offset=${offset}`),
			apiRequest('/v1/platform/operations/brands')
		]);
		if (invRes.ok) {
			const data = await invRes.json();
			rows = data.invoices ?? [];
			total = data.total ?? 0;
		} else {
			error = await platformError(invRes);
		}
		if (brandRes.ok) {
			brands = await brandRes.json();
			form = { ...form, brand_id: form.brand_id || brands[0]?.id || '' };
		}
		loading = false;
	}

	async function loadCampaigns(brandId: string) {
		form = { ...form, brand_id: brandId, campaign_id: '' };
		const res = await apiRequest(`/v1/platform/operations/brands/${brandId}/campaigns`);
		if (res.ok) {
			const data = await res.json();
			campaigns = data.campaigns ?? [];
		} else {
			campaigns = [];
		}
	}

	async function create(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		const payload: Record<string, unknown> = {
			brand_id: form.brand_id,
			campaign_id: form.campaign_id || null,
			status: form.status,
			notes: form.notes.trim() || null,
			issued_at: form.issued_at ? new Date(`${form.issued_at}T00:00:00.000Z`).toISOString() : null,
			due_at: form.due_at ? new Date(`${form.due_at}T00:00:00.000Z`).toISOString() : null
		};
		if (!form.campaign_id) payload.amount = Number(form.amount) || 0;
		const res = await apiRequest('/v1/platform/invoices', { method: 'POST', body: JSON.stringify(payload) });
		if (!res.ok) {
			error = await platformError(res);
			busy = false;
			return;
		}
		createOpen = false;
		busy = false;
		await load();
	}

	async function updateStatus(invoice: Invoice, status: string) {
		error = '';
		const res = await apiRequest(`/v1/platform/invoices/${invoice.id}`, {
			method: 'PUT',
			body: JSON.stringify({ status, paid_at: status === 'paid' ? new Date().toISOString() : null })
		});
		if (!res.ok) {
			error = await platformError(res);
			return;
		}
		await load();
	}

	const denied = $derived(Boolean(!loading && rows.length === 0 && isAccessDenied(error)));
</script>

<section class="mx-auto max-w-7xl">
	<PageHeader
		title="Invoices"
		description="Tagihan ke brand — manual atau otomatis dari campaign (jumlah = Σ harga publikasi published)."
	>
		<span class="rounded-full px-3 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{fmtNum(total)} invoice</span>
		<button class="app-btn app-btn-primary" type="button" onclick={() => (createOpen = true)}><Receipt size={16} /> Buat invoice</button>
	</PageHeader>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke invoice.</p>
		</div>
	{:else}
		<Feedback {loading} {error} loadingText="Memuat invoice…" />

		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 980px;">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">Nomor</th>
						<th class="px-4 py-2.5 font-medium">Brand</th>
						<th class="px-4 py-2.5 font-medium">Campaign</th>
						<th class="px-4 py-2.5 font-medium">Jumlah</th>
						<th class="px-4 py-2.5 font-medium">Status</th>
						<th class="px-4 py-2.5 font-medium">Jatuh tempo</th>
						<th class="px-4 py-2.5"></th>
					</tr>
				</thead>
				<tbody>
					{#each rows as row (row.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<div class="flex items-center gap-2">
									<Receipt size={16} class="shrink-0 text-muted-foreground" />
									<div>
										<p class="font-medium">{row.number}</p>
										<p class="text-xs text-muted-foreground">{row.id.slice(0, 8)}</p>
									</div>
								</div>
							</td>
							<td class="px-4 py-2.5">{row.brand_name ?? row.brand_id.slice(0, 8)}</td>
							<td class="px-4 py-2.5">{row.campaign_id ? 'Ya' : '—'}</td>
							<td class="px-4 py-2.5 tabular-nums">{fmtRp(row.amount)}</td>
							<td class="px-4 py-2.5"><Badge label={row.status} tone={STATUS_TONE[row.status] ?? 'neutral'} /></td>
							<td class="px-4 py-2.5 text-muted-foreground">{row.due_at ? fmtDate(row.due_at) : '—'}</td>
							<td class="px-4 py-2.5">
								<div class="flex gap-2">
									{#if row.status === 'draft'}
										<button class="app-btn !min-h-7 !px-2.5 !text-xs" style="background-color: var(--warning-soft); color: var(--warning);" type="button" onclick={() => void updateStatus(row, 'issued')}>Issue</button>
									{/if}
									{#if row.status === 'issued'}
										<button class="app-btn !min-h-7 !px-2.5 !text-xs" style="background-color: var(--success-soft); color: var(--success);" type="button" onclick={() => void updateStatus(row, 'paid')}>Tandai bayar</button>
									{/if}
									{#if ['draft', 'issued'].includes(row.status)}
										<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void updateStatus(row, 'void')}>Void</button>
									{/if}
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && rows.length === 0}
				<div class="flex flex-col items-center px-4 py-12 text-center">
					<Receipt size={26} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">Belum ada invoice.</p>
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
	{/if}
</section>

{#if createOpen}
	<Modal title="Buat invoice" label="Buat invoice" maxWidth="lg" onClose={() => (createOpen = false)}>
		<form class="flex flex-col gap-3" onsubmit={create}>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Brand</span>
				<Select
					ariaLabel="Brand"
					minWidth={240}
					placeholder="Pilih brand"
					value={form.brand_id}
					onchange={(v) => void loadCampaigns(v)}
					options={brands.map((b) => ({ value: b.id, label: b.name }))}
				/>
			</label>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Campaign (opsional — jumlah otomatis)</span>
				<Select
					ariaLabel="Campaign"
					minWidth={260}
					value={form.campaign_id}
					onchange={(v) => (form = { ...form, campaign_id: v })}
					options={[
						{ value: '', label: 'Manual (isi jumlah)' },
						...campaigns.map((c) => ({ value: c.id, label: `${c.name} (${c.publication_count} publikasi)` }))
					]}
				/>
			</label>
			{#if !form.campaign_id}
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Jumlah (IDR)</span>
					<input class="app-input w-full" type="number" min="0" required value={form.amount} oninput={(e) => (form = { ...form, amount: (e.currentTarget as HTMLInputElement).value })} />
				</label>
			{/if}
			<div class="grid gap-3 sm:grid-cols-2">
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Status</span>
					<Select
						ariaLabel="Status"
						minWidth={160}
						value={form.status}
						onchange={(v) => (form = { ...form, status: v })}
						options={['draft', 'issued', 'paid', 'void'].map((s) => ({ value: s, label: s }))}
					/>
				</label>
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Jatuh tempo</span>
					<input class="app-input w-full" type="date" bind:value={form.due_at} />
				</label>
			</div>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Catatan (opsional)</span>
				<input class="app-input w-full" bind:value={form.notes} />
			</label>
			{#if error}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{error}</div>
			{/if}
			<div class="mt-1 flex justify-end gap-2 border-t pt-4">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (createOpen = false)}>Batal</button>
				<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? 'Menyimpan…' : 'Buat invoice'}</button>
			</div>
		</form>
	</Modal>
{/if}
