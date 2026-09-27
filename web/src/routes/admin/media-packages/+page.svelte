<script lang="ts">
	// Media Packages — bundel media yang bisa dipilih klien satu klik.
	// Port apps/web/src/app/admin/media-packages/page.tsx.
	import { onMount } from 'svelte';
	import { Package, Plus, Trash } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const PAGE_SIZE = 20;

	type MediaPackage = {
		id: string;
		name: string;
		slug: string;
		description: string | null;
		status: string;
		discount_percent: number;
		media_count: number;
		total_credit_cost: number;
		discounted_credit_cost: number;
		savings_credit: number;
		created_at: string;
		updated_at: string;
	};
	type MediaOption = { id: string; name: string; category: string | null };

	let rows = $state<MediaPackage[]>([]);
	let mediaOptions = $state<MediaOption[]>([]);
	let error = $state('');
	let loading = $state(true);
	let drafts = $state<Record<string, string>>({});
	let savingId = $state<string | null>(null);
	let creating = $state(false);
	let createName = $state('');
	let createDescription = $state('');
	let createDiscount = $state('0');
	let createMediaIds = $state<string[]>([]);
	let mediaQuery = $state('');
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

	// Pencarian media sisi server (debounced) — endpoint /media mendukung search.
	let debouncedMediaQuery = $state('');
	let searchTimer: ReturnType<typeof setTimeout> | null = null;
	$effect(() => {
		const value = mediaQuery.trim();
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => (debouncedMediaQuery = value), 300);
		return () => {
			if (searchTimer) clearTimeout(searchTimer);
		};
	});
	$effect(() => {
		void debouncedMediaQuery;
		void searchMedia(debouncedMediaQuery);
	});

	async function searchMedia(q: string) {
		try {
			const params = new URLSearchParams({ limit: '500' });
			if (q) params.set('search', q);
			const res = await apiRequest(`/v1/platform/media?${params}`);
			if (!res.ok) return;
			const data = await res.json();
			mediaOptions = data.media ?? [];
		} catch {
			/* keep current list */
		}
	}

	async function load() {
		loading = true;
		error = '';
		try {
			const [pkgRes, mediaRes] = await Promise.all([
				apiRequest('/v1/platform/media-packages'),
				apiRequest('/v1/platform/media?limit=500')
			]);
			if (!pkgRes.ok) {
				error = await readApiError(pkgRes, 'Permintaan gagal.');
				return;
			}
			const pkgData = await pkgRes.json();
			rows = pkgData.packages ?? [];
			drafts = Object.fromEntries(rows.map((p) => [p.id, String(p.discount_percent)]));
			if (mediaRes.ok) {
				const mediaData = await mediaRes.json();
				mediaOptions = mediaData.media ?? [];
			} else {
				mediaOptions = [];
			}
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	async function saveDiscount(pkg: MediaPackage) {
		const raw = Number(drafts[pkg.id]);
		if (!Number.isFinite(raw)) {
			error = 'Diskon harus berupa angka.';
			return;
		}
		error = '';
		savingId = pkg.id;
		try {
			const res = await apiRequest(`/v1/platform/media-packages/${pkg.id}`, {
				method: 'PUT',
				body: JSON.stringify({ discount_percent: Math.max(0, Math.min(100, Math.round(raw))) })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			await load();
		} finally {
			savingId = null;
		}
	}

	async function createPackage() {
		const name = createName.trim();
		const discount = Math.max(0, Math.min(100, Number(createDiscount) || 0));
		if (!name) {
			error = 'Nama paket wajib diisi.';
			return;
		}
		if (createMediaIds.length === 0) {
			error = 'Pilih minimal 1 media.';
			return;
		}
		error = '';
		creating = true;
		try {
			const res = await apiRequest('/v1/platform/media-packages', {
				method: 'POST',
				body: JSON.stringify({
					name,
					description: createDescription.trim() || null,
					media_ids: createMediaIds,
					discount_percent: discount
				})
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			createName = '';
			createDescription = '';
			createDiscount = '0';
			createMediaIds = [];
			mediaQuery = '';
			await load();
		} finally {
			creating = false;
		}
	}

	async function deletePackage(pkg: MediaPackage) {
		if (!window.confirm(`Hapus paket "${pkg.name}"? Paket akan hilang dari daftar klien.`)) return;
		error = '';
		savingId = pkg.id;
		try {
			const res = await apiRequest(`/v1/platform/media-packages/${pkg.id}`, { method: 'DELETE' });
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			await load();
		} finally {
			savingId = null;
		}
	}

	function toggleCreateMedia(id: string) {
		createMediaIds = createMediaIds.includes(id)
			? createMediaIds.filter((x) => x !== id)
			: [...createMediaIds, id];
	}

	const filteredMedia = $derived.by(() => {
		const q = mediaQuery.trim().toLowerCase();
		if (!q) return mediaOptions;
		return mediaOptions.filter(
			(m) => m.name.toLowerCase().includes(q) || (m.category ?? '').toLowerCase().includes(q)
		);
	});

	const visibleRows = $derived(rows.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE));
</script>

{#snippet packageCell(pkg: MediaPackage)}
	<div class="flex items-center gap-2">
		<Package size={16} />
		<div>
			<p class="font-medium">{pkg.name}</p>
			<p class="text-xs text-muted-foreground">
				{pkg.slug ?? pkg.id.slice(0, 8)}{pkg.description ? ` · ${pkg.description}` : ''}
			</p>
		</div>
	</div>
{/snippet}

{#snippet mediaCountCell(pkg: MediaPackage)}
	<Badge tone="accent" value={`${pkg.media_count} media`} />
{/snippet}

{#snippet totalCell(pkg: MediaPackage)}
	<span class="tabular-nums">{pkg.total_credit_cost.toLocaleString('id-ID')} TC</span>
{/snippet}

{#snippet discountCell(pkg: MediaPackage)}
	<input
		class="app-input"
		type="number"
		min="0"
		max="100"
		style="width: 90px;"
		value={drafts[pkg.id] ?? String(pkg.discount_percent)}
		oninput={(e) => (drafts = { ...drafts, [pkg.id]: e.currentTarget.value })}
	/>
{/snippet}

{#snippet savingsCell(pkg: MediaPackage)}
	{#if pkg.discount_percent > 0}
		<span class="font-bold" style="color: var(--success);">Hemat {pkg.savings_credit.toLocaleString('id-ID')} TC</span>
	{:else}
		<span class="text-xs text-muted-foreground">—</span>
	{/if}
{/snippet}

{#snippet packageStatusCell(pkg: MediaPackage)}
	<Badge tone={pkg.status === 'active' ? 'success' : 'neutral'} value={pkg.status} />
{/snippet}

{#snippet packageActions(pkg: MediaPackage)}
	<div class="flex justify-end gap-2">
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			disabled={savingId === pkg.id}
			onclick={() => void saveDiscount(pkg)}>{savingId === pkg.id ? 'Menyimpan…' : 'Simpan diskon'}</button
		>
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			style="color: var(--destructive);"
			disabled={savingId === pkg.id}
			aria-label={`Hapus paket ${pkg.name}`}
			onclick={() => void deletePackage(pkg)}
		>
			<Trash size={13} />
		</button>
	</div>
{/snippet}

<section>
	<PageHeader
		title="Media Packages"
		description="Bundel media yang bisa dipilih klien satu klik. Diskon persen (0–100) benar-benar dipakai saat billing TC publikasi — setiap media anggota dihitung dengan harga diskon, dan klien melihat 'Hemat X TC' di tombol paket. Paket yang dihapus otomatis hilang dari daftar klien."
	>
		<Badge tone="accent" value={`${rows.length} paket`} />
	</PageHeader>

	<Feedback {loading} {error} />

	<div class="app-card mt-4 p-4">
		<div class="flex flex-col gap-3">
			<p class="font-bold">Buat paket baru</p>
			<div class="flex flex-wrap items-end gap-3">
				<label class="block min-w-[220px] flex-1">
					<span class="mb-1 block text-xs text-muted-foreground">Nama paket</span>
					<input
						class="app-input w-full"
						placeholder="cth: Media Nasional Tier 1"
						maxlength="120"
						bind:value={createName}
					/>
				</label>
				<label class="block min-w-[180px] flex-1">
					<span class="mb-1 block text-xs text-muted-foreground">Deskripsi (opsional)</span>
					<input
						class="app-input w-full"
						placeholder="cth: Detik, Kompas, Tempo"
						maxlength="300"
						bind:value={createDescription}
					/>
				</label>
				<label class="block" style="width: 120px;">
					<span class="mb-1 block text-xs text-muted-foreground">Diskon %</span>
					<input class="app-input w-full" type="number" min="0" max="100" bind:value={createDiscount} />
				</label>
				<button class="app-btn app-btn-primary" disabled={creating} onclick={createPackage}>
					<Plus size={16} />{creating ? 'Menyimpan…' : 'Buat paket'}
				</button>
			</div>
			<div class="flex flex-wrap items-center gap-3">
				<input
					class="app-input min-w-[220px] flex-1"
					placeholder="Cari media untuk ditambahkan…"
					bind:value={mediaQuery}
				/>
				<span class="text-xs text-muted-foreground">{createMediaIds.length} media dipilih</span>
			</div>
			<div class="max-h-[180px] overflow-y-auto rounded-lg border p-2">
				{#if filteredMedia.length === 0}
					<p class="text-sm text-muted-foreground">Tidak ada media yang cocok.</p>
				{/if}
				<div class="flex flex-wrap gap-2">
					{#each filteredMedia as m (m.id)}
						<button
							type="button"
							class="app-btn !min-h-8 !px-3 !text-xs"
							class:app-btn-primary={createMediaIds.includes(m.id)}
							class:app-btn-soft={!createMediaIds.includes(m.id)}
							onclick={() => toggleCreateMedia(m.id)}>{m.name}</button
						>
					{/each}
				</div>
			</div>
		</div>
	</div>

	<DataTable
		columns={[
			{ key: 'package', label: 'Paket', render: packageCell },
			{ key: 'media', label: 'Media', render: mediaCountCell },
			{ key: 'total', label: 'Total (TC)', render: totalCell },
			{ key: 'discount', label: 'Diskon %', render: discountCell },
			{ key: 'savings', label: 'Hemat (TC)', render: savingsCell },
			{ key: 'status', label: 'Status', render: packageStatusCell },
			{ key: 'actions', label: '', align: 'right' as const, render: packageActions }
		]}
		rows={visibleRows}
		rowKey={(pkg: MediaPackage) => pkg.id}
		empty="Belum ada media package."
		minWidth="900px"
	/>

	<Pagination
		total={rows.length}
		limit={PAGE_SIZE}
		offset={(page - 1) * PAGE_SIZE}
		onChange={(next) => (page = next / PAGE_SIZE + 1)}
	/>

	<p class="mt-2 text-xs text-muted-foreground">
		Catatan: diskon dihitung per media saat reservasi TC publikasi (pembulatan ke bawah per media, minimal 1 TC bila harga
		asli &gt; 0). Transaksi lama tidak terpengaruh — hanya publikasi baru yang membawa package_id memakai diskon ini.
	</p>
</section>
