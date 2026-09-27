<script lang="ts">
	// Auto-Replace — saat partner menolak publikasi, Tauthub otomatis membuat
	// publikasi pengganti ke media rekomendasi terbaik. Atur aktif/nonaktif global
	// atau per brand, dan batas penolakan beruntun.
	// Port apps/web/src/app/admin/auto-replace/page.tsx.
	import { onMount } from 'svelte';
	import { Shuffle } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const PAGE_SIZE = 20;

	type ConfigRow = {
		id: string;
		brand_id: string | null;
		brand_name: string | null;
		enabled: boolean;
		max_replaces: number;
		updated_at: string | null;
	};
	type Brand = { id: string; name: string; slug: string; owner_email: string };

	let global = $state<ConfigRow | null>(null);
	let perBrand = $state<ConfigRow[]>([]);
	let brands = $state<Brand[]>([]);
	let error = $state('');
	let loading = $state(true);
	let busy = $state(false);
	let brandFilter = $state('');
	let brandEnabled = $state(true);
	let brandMax = $state('3');
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
			const [configRes, brandRes] = await Promise.all([
				apiRequest('/v1/platform/auto-replace'),
				apiRequest('/v1/platform/operations/brands')
			]);
			if (!configRes.ok) {
				error = await readApiError(configRes, 'Permintaan gagal.');
				return;
			}
			const config = await configRes.json();
			global = config.global;
			perBrand = config.per_brand ?? [];
			if (brandRes.ok) {
				const brandData = await brandRes.json();
				brands = Array.isArray(brandData) ? brandData : [];
			}
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	async function saveGlobal() {
		if (!global) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest('/v1/platform/auto-replace', {
				method: 'PUT',
				body: JSON.stringify({ enabled: global.enabled, max_replaces: global.max_replaces })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			error = 'Konfigurasi global disimpan.';
			void load();
		} finally {
			busy = false;
		}
	}

	async function saveBrand() {
		if (!brandFilter) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/auto-replace?brandId=${brandFilter}`, {
				method: 'PUT',
				body: JSON.stringify({ enabled: brandEnabled, max_replaces: Number(brandMax) || 3 })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			error = 'Konfigurasi brand disimpan.';
			brandFilter = '';
			void load();
		} finally {
			busy = false;
		}
	}

	async function toggleBrand(row: ConfigRow) {
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/auto-replace?brandId=${row.brand_id}`, {
				method: 'PUT',
				body: JSON.stringify({ enabled: !row.enabled, max_replaces: row.max_replaces })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			void load();
		} finally {
			busy = false;
		}
	}

	const visibleBrands = $derived(perBrand.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE));
	const isNotice = $derived(error.includes('disimpan'));
</script>

<section>
	<PageHeader
		title="Auto-Replace"
		description="Saat partner menolak publikasi, Tauthub otomatis membuat publikasi pengganti ke media rekomendasi terbaik. Atur aktif/nonaktif global atau per brand, dan batas penolakan beruntun."
	>
		<Badge
			tone={global?.enabled === false ? 'danger' : 'success'}
			label={global?.enabled === false ? 'Nonaktif (global)' : 'Aktif (global)'}
		/>
	</PageHeader>

	<Feedback {loading} error={isNotice ? '' : error} notice={isNotice ? error : ''} />

	<div class="app-card mt-4 p-4">
		<div class="flex flex-wrap items-center justify-between gap-4">
			<div>
				<p class="font-medium">Pengaturan global</p>
				<p class="text-xs text-muted-foreground">
					Berlaku untuk semua brand yang tidak punya override sendiri. Default: aktif, maksimal 3 penggantian beruntun per
					publikasi.
				</p>
			</div>
			{#if global}
				<div class="flex flex-wrap items-center gap-4">
					<label class="flex items-center gap-2">
						<input
							type="checkbox"
							checked={global.enabled}
							onchange={(e) => {
								if (global) global = { ...global, enabled: e.currentTarget.checked };
							}}
						/>
						<span class="text-sm">Aktif</span>
					</label>
					<label class="block">
						<span class="text-xs font-bold text-muted-foreground">Maks. penggantian beruntun</span>
						<input
							class="app-input mt-1"
							type="number"
							min="1"
							max="10"
							style="width: 90px;"
							value={String(global.max_replaces)}
							oninput={(e) => {
								if (global)
									global = {
										...global,
										max_replaces: Math.min(10, Math.max(1, Number(e.currentTarget.value) || 1))
									};
							}}
						/>
					</label>
					<button class="app-btn app-btn-primary" onclick={saveGlobal} disabled={busy}>Simpan global</button>
				</div>
			{/if}
		</div>
	</div>

	<div class="app-card mt-3 p-4">
		<p class="mb-3 font-medium">Override per brand</p>
		<div class="flex flex-wrap items-end gap-3">
			<label class="block min-w-[260px] flex-1">
				<span class="text-xs font-bold text-muted-foreground">Brand</span>
				<Select
					ariaLabel="Brand"
					placeholder="Pilih brand…"
					minWidth={260}
					value={brandFilter}
					onchange={(v) => (brandFilter = v)}
					options={[
						{ value: '', label: 'Pilih brand…' },
						...brands.filter((b) => b.name).map((b) => ({ value: b.id, label: b.name }))
					]}
				/>
			</label>
			<label class="flex items-center gap-2 pb-3">
				<input type="checkbox" bind:checked={brandEnabled} />
				<span class="text-sm">Aktif</span>
			</label>
			<label class="block">
				<span class="text-xs font-bold text-muted-foreground">Maks. beruntun</span>
				<input class="app-input mt-1" type="number" min="1" max="10" style="width: 90px;" bind:value={brandMax} />
			</label>
			<button class="app-btn app-btn-primary" onclick={saveBrand} disabled={busy || !brandFilter}>Tambah override</button>
		</div>

		<div class="mt-4 overflow-x-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="text-xs text-muted-foreground">
						<th class="px-4 py-2.5 text-left font-medium">Brand</th>
						<th class="px-4 py-2.5 text-left font-medium">Status</th>
						<th class="px-4 py-2.5 text-left font-medium">Maks. beruntun</th>
						<th class="px-4 py-2.5 text-left font-medium">Diperbarui</th>
						<th class="px-4 py-2.5 text-left font-medium"></th>
					</tr>
				</thead>
				<tbody>
					{#if perBrand.length === 0}
						<tr
							><td colspan="5" class="px-4 py-2.5 text-sm text-muted-foreground"
								>Belum ada override per brand — semua brand memakai pengaturan global.</td
							></tr
						>
					{/if}
					{#each visibleBrands as row (row.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5 font-medium">{row.brand_name ?? row.brand_id}</td>
							<td class="px-4 py-2.5">
								<Badge tone={row.enabled ? 'success' : 'danger'} label={row.enabled ? 'Aktif' : 'Nonaktif'} />
							</td>
							<td class="px-4 py-2.5 tabular-nums">{row.max_replaces}</td>
							<td class="px-4 py-2.5 text-xs">{row.updated_at ? fmtDateTime(row.updated_at) : '—'}</td>
							<td class="px-4 py-2.5">
								<input
									type="checkbox"
									checked={row.enabled}
									aria-label={`Aktifkan override ${row.brand_name ?? row.brand_id}`}
									onchange={() => void toggleBrand(row)}
								/>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
		<Pagination
			total={perBrand.length}
			limit={PAGE_SIZE}
			offset={(page - 1) * PAGE_SIZE}
			onChange={(next) => (page = next / PAGE_SIZE + 1)}
		/>
	</div>

	<div class="app-card mt-3 p-4 text-xs text-muted-foreground">
		<Shuffle size={14} class="mr-1 inline align-[-2px]" />
		Alur: partner menolak → Tauthub mencari media alternatif (media yang sudah dicoba untuk artikel yang sama dieksklusi,
		standard check tetap dijalankan) → publikasi pengganti dibuat otomatis → klien diberi notifikasi. Jika melebihi batas
		beruntun atau tidak ada kandidat, publikasi tetap ditolak tanpa penggantian.
	</div>
</section>
