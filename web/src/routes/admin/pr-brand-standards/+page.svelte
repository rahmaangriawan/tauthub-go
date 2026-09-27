<script lang="ts">
	// Standar PR per brand — port app/admin/pr-brand-standards/page.tsx.
	// Override rate per 1.000 estimasi reach khusus brand; kosong = pakai standar global Tauthub.
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';

	type BrandStandard = {
		id: string;
		name: string;
		public_id: string | null;
		status: string;
		standard: {
			rate_per_1k_reach: number | null;
			rate_tier1: number | null;
			rate_tier2: number | null;
			rate_tier3: number | null;
			updated_at: string | null;
			updated_by: { id: string; name: string | null; email: string | null } | null;
		} | null;
	};

	const PAGE_SIZE = 20;

	let loading = $state(true);
	let error = $state('');
	let notice = $state('');
	let busy = $state(false);
	let brands = $state<BrandStandard[]>([]);
	let selected = $state('');
	let formRate = $state('');
	let page = $state(1);

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
		try {
			const res = await apiRequest('/v1/platform/pr/brand-standards');
			if (!res.ok) {
				const msg = await readApiError(res, 'Permintaan platform gagal.');
				error = msg === 'Resource not found' ? 'Akses platform administrator tidak tersedia.' : msg;
				return;
			}
			brands = (await res.json()).brands ?? [];
		} catch {
			error = 'Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.';
		} finally {
			loading = false;
		}
	}

	function pickBrand(id: string) {
		selected = id;
		const row = brands.find((b) => b.id === id);
		const std = row?.standard;
		formRate = std?.rate_per_1k_reach != null ? String(std.rate_per_1k_reach) : '';
	}

	async function save() {
		if (!selected) return;
		busy = true;
		error = '';
		notice = '';
		try {
			const raw = formRate.trim();
			const res = await apiRequest(`/v1/platform/pr/brand-standards/${selected}`, {
				method: 'PUT',
				body: JSON.stringify({ rate_per_1k_reach: raw === '' ? null : Number(raw) })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			notice = 'Standar PR brand disimpan.';
			selected = '';
			await load();
		} finally {
			busy = false;
		}
	}

	async function clearBrand(id: string) {
		if (!confirm('Hapus override brand ini? Brand kembali memakai standar global.')) return;
		busy = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/platform/pr/brand-standards/${id}`, {
				method: 'PUT',
				body: JSON.stringify({ rate_per_1k_reach: null, rate_tier1: null, rate_tier2: null, rate_tier3: null })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			notice = 'Override brand dihapus — brand kembali memakai standar global.';
			await load();
		} finally {
			busy = false;
		}
	}

	const visibleBrands = $derived(brands.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE));
	const fmtNumber = (value: number | null) => (value != null ? value.toLocaleString('id-ID') : '—');
	const fmtBy = (row: BrandStandard) => {
		const by = row.standard?.updated_by;
		const at = row.standard?.updated_at;
		const who = by ? by.name || by.email || '—' : '—';
		const time = at ? fmtDateTime(at) : '—';
		return `${who} · ${time}`;
	};
</script>

<section>
	<PageHeader
		title="Standar PR per Brand"
		description="Standarisasi PR adalah engine internal Tauthub — klien tidak mengatur ini. Override per brand: rate per 1.000 estimasi reach khusus untuk brand ini. Kosong = pakai standar global Tauthub (diatur di menu PR Calculator)."
	>
		<Badge tone="accent" label="{brands.length} brand" />
	</PageHeader>

	<Feedback {loading} {error} {notice} loadingText="Memuat standar PR per brand…" />

	{#if brands.length === 0 && error.includes('Akses platform')}
		<div class="app-card mt-4 px-4 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke standar PR.</p>
		</div>
	{:else}
		<div class="app-card mt-4 p-4">
			<h2 class="text-sm font-semibold">Atur / ubah standar sebuah brand</h2>
			<div class="mt-3 flex flex-wrap items-end gap-3">
				<label class="block min-w-[280px] flex-1">
					<span class="text-sm font-medium">Brand</span>
					<Select
						ariaLabel="Brand"
						placeholder="Pilih brand…"
						minWidth={260}
						value={selected}
						onchange={(v) => pickBrand(v)}
						options={[
							{ value: '', label: 'Pilih brand…' },
							...brands.map((b) => ({ value: b.id, label: `${b.name}${b.standard ? ' (sudah diatur)' : ''}` }))
						]}
					/>
				</label>
				<label class="block w-[170px]">
					<span class="text-sm font-medium">Rate global (Rp / 1k reach)</span>
					<input class="app-input mt-1 w-full" type="number" min={0} max={10000000} placeholder="kosong = global" bind:value={formRate} />
				</label>
				<button class="app-btn app-btn-primary" type="button" disabled={busy || !selected} onclick={() => void save()}>Simpan standar brand</button>
			</div>
			<p class="mt-2 text-xs text-muted-foreground">
				Catatan: menyimpan standar brand akan mengembalikan rate tier brand ke global Tauthub. Perubahan tidak mengubah angka historis yang sudah dihitung.
			</p>
		</div>

		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 880px;">
				<thead>
					<tr class="text-xs text-muted-foreground">
						<th class="px-4 py-2.5 text-left font-medium">Brand</th>
						<th class="px-4 py-2.5 text-left font-medium">Rate global</th>
						<th class="px-4 py-2.5 text-left font-medium">Terakhir diubah oleh</th>
						<th class="px-4 py-2.5 text-left font-medium"></th>
					</tr>
				</thead>
				<tbody>
					{#each visibleBrands as row (row.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<span class="font-medium">{row.name}</span>
								{#if row.status !== 'active'}<span class="ml-2"><Badge tone="neutral" label={row.status} /></span>{/if}
							</td>
							<td class="px-4 py-2.5">{fmtNumber(row.standard?.rate_per_1k_reach ?? null)}</td>
							<td class="px-4 py-2.5">
								{#if row.standard}
									{fmtBy(row)}
								{:else}
									<span class="text-muted-foreground">pakai global Tauthub</span>
								{/if}
							</td>
							<td class="px-4 py-2.5">
								{#if row.standard}
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" disabled={busy} onclick={() => void clearBrand(row.id)}>Hapus override</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if brands.length === 0}
				<p class="px-4 py-12 text-center text-sm text-muted-foreground">Belum ada brand.</p>
			{/if}
		</div>
		<Pagination total={brands.length} limit={PAGE_SIZE} offset={(page - 1) * PAGE_SIZE} onChange={(next) => (page = next / PAGE_SIZE + 1)} />

		<div class="app-card mt-3 p-4">
			<p class="text-xs text-muted-foreground">
				Estimated PR value = nilai media + Σ(reach ÷ 1.000 × rate). Rate tier global (Tier 1/2/3) dikelola di menu PR Calculator.
				Skor bukan AVE dan bukan jaminan dampak bisnis.
			</p>
		</div>
	{/if}
</section>
