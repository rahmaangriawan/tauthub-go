<script lang="ts">
	// Paket kredit — port app/admin/packages/page.tsx: paket kredit per brand;
	// grant langsung tercatat di ledger credit_transactions. Aksi aktif/revoke.
	import { onMount } from 'svelte';
	import { Coins, TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtRp, fmtNum } from '$lib/admin-format';
	import { platformError, isAccessDenied } from '../_platform';

	type Package = {
		id: string; brand_id: string; brand_name: string | null; name: string;
		credit_total: number; credit_remaining: number; price: number; status: string; created_at: string;
	};
	type Brand = { id: string; name: string; slug: string };

	const PAGE_SIZE = 20;

	let rows = $state<Package[]>([]);
	let brands = $state<Brand[]>([]);
	let error = $state('');
	let loading = $state(true);
	let createOpen = $state(false);
	let busy = $state(false);
	let form = $state({ brand_id: '', name: '', credit_total: '100', price: '0' });
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

	async function load() {
		loading = true;
		error = '';
		const [pkgRes, brandRes] = await Promise.all([
			apiRequest('/v1/platform/packages'),
			apiRequest('/v1/platform/operations/brands')
		]);
		if (pkgRes.ok) {
			const data = await pkgRes.json();
			rows = data.packages ?? [];
		} else {
			error = await platformError(pkgRes);
		}
		if (brandRes.ok) {
			brands = await brandRes.json();
			form = { ...form, brand_id: form.brand_id || brands[0]?.id || '' };
		}
		loading = false;
	}

	async function create(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		const res = await apiRequest('/v1/platform/packages', {
			method: 'POST',
			body: JSON.stringify({
				brand_id: form.brand_id,
				name: form.name.trim(),
				credit_total: Number(form.credit_total),
				price: Number(form.price) || 0
			})
		});
		if (!res.ok) {
			error = await platformError(res);
			busy = false;
			return;
		}
		createOpen = false;
		form = { ...form, name: '', credit_total: '100', price: '0' };
		busy = false;
		await load();
	}

	async function setStatus(pkg: Package, status: string) {
		error = '';
		const res = await apiRequest(`/v1/platform/packages/${pkg.id}`, {
			method: 'PUT',
			body: JSON.stringify({ status })
		});
		if (!res.ok) {
			error = await platformError(res);
			return;
		}
		await load();
	}

	const start = $derived((pageNum - 1) * PAGE_SIZE);
	const visibleRows = $derived(rows.slice(start, start + PAGE_SIZE));
	const denied = $derived(Boolean(!loading && rows.length === 0 && isAccessDenied(error)));
</script>

<section class="mx-auto max-w-7xl">
	<PageHeader
		title="Packages"
		description="Paket kredit per brand; grant langsung tercatat di ledger credit_transactions (Phase 5)."
	>
		<span class="rounded-full px-3 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{fmtNum(rows.length)} paket</span>
		<button class="app-btn app-btn-primary" type="button" onclick={() => (createOpen = true)}><Coins size={16} /> Buat paket</button>
	</PageHeader>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke pengelolaan paket.</p>
		</div>
	{:else}
		<Feedback {loading} {error} loadingText="Memuat paket kredit…" />

		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 900px;">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">Paket</th>
						<th class="px-4 py-2.5 font-medium">Brand</th>
						<th class="px-4 py-2.5 font-medium">Kredit</th>
						<th class="px-4 py-2.5 font-medium">Harga</th>
						<th class="px-4 py-2.5 font-medium">Status</th>
						<th class="px-4 py-2.5"></th>
					</tr>
				</thead>
				<tbody>
					{#each visibleRows as row (row.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<div class="flex items-center gap-2">
									<Coins size={16} class="shrink-0 text-muted-foreground" />
									<div>
										<p class="font-medium">{row.name}</p>
										<p class="text-xs text-muted-foreground">{row.id.slice(0, 8)}</p>
									</div>
								</div>
							</td>
							<td class="px-4 py-2.5">{row.brand_name ?? row.brand_id.slice(0, 8)}</td>
							<td class="px-4 py-2.5 tabular-nums">{fmtNum(row.credit_remaining)} / {fmtNum(row.credit_total)}</td>
							<td class="px-4 py-2.5 tabular-nums">{fmtRp(row.price)}</td>
							<td class="px-4 py-2.5"><Badge label={row.status} tone={row.status === 'active' ? 'success' : 'neutral'} /></td>
							<td class="px-4 py-2.5">
								{#if row.status === 'active'}
									<button class="app-btn !min-h-7 !px-2.5 !text-xs" style="background-color: var(--destructive-soft, #fdecee); color: var(--destructive);" type="button" onclick={() => void setStatus(row, 'revoked')}>Revoke</button>
								{:else}
									<button class="app-btn !min-h-7 !px-2.5 !text-xs" style="background-color: var(--success-soft); color: var(--success);" type="button" onclick={() => void setStatus(row, 'active')}>Aktifkan</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && rows.length === 0}
				<div class="flex flex-col items-center px-4 py-12 text-center">
					<Coins size={26} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">Belum ada paket kredit.</p>
				</div>
			{/if}
			<div class="px-4 pb-4">
				<Pagination
					total={rows.length}
					limit={PAGE_SIZE}
					offset={start}
					onChange={(next) => (pageNum = Math.floor(next / PAGE_SIZE) + 1)}
				/>
			</div>
		</div>
	{/if}
</section>

{#if createOpen}
	<Modal title="Buat paket kredit" label="Buat paket kredit" maxWidth="lg" onClose={() => (createOpen = false)}>
		<form class="flex flex-col gap-3" onsubmit={create}>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Brand</span>
				<select class="app-input w-full" bind:value={form.brand_id}>
					{#each brands as b (b.id)}
						<option value={b.id}>{b.name}</option>
					{/each}
				</select>
			</label>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Nama paket</span>
				<input class="app-input w-full" required placeholder="Paket Starter" bind:value={form.name} />
			</label>
			<div class="grid gap-3 sm:grid-cols-2">
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Jumlah kredit</span>
					<input class="app-input w-full" type="number" min="1" required value={form.credit_total} oninput={(e) => (form = { ...form, credit_total: (e.currentTarget as HTMLInputElement).value })} />
				</label>
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Harga (IDR)</span>
					<input
						class="app-input w-full"
						type="text"
						inputmode="numeric"
						value={form.price ? Number(form.price).toLocaleString('id-ID') : ''}
						oninput={(e) => (form = { ...form, price: (e.currentTarget as HTMLInputElement).value.replace(/\D/g, '') })}
					/>
				</label>
			</div>
			{#if error}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{error}</div>
			{/if}
			<div class="mt-1 flex justify-end gap-2 border-t pt-4">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (createOpen = false)}>Batal</button>
				<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? 'Menyimpan…' : 'Buat paket'}</button>
			</div>
		</form>
	</Modal>
{/if}
