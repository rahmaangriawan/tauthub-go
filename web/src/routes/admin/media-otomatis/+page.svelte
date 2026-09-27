<script lang="ts">
	// Media Otomatis — daftar media mitra yang mode otomatisnya sedang menyala.
	// Port apps/web/src/app/admin/media-otomatis/page.tsx.
	//
	// Mode otomatis berarti artikel terbit TANPA persetujuan mitra. Karena itu
	// halaman ini hanya menampilkan dan mematikan; untuk menyalakannya, mitra yang
	// bersangkutan harus sudah diizinkan admin lalu menyalakan sendiri di portalnya.
	import { onMount } from 'svelte';
	import { RefreshCw, Unlink } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const PAGE_SIZE = 20;

	type MediaRow = {
		id: string;
		name: string;
		website_url: string | null;
		status: string;
		integration_type: string;
		partner_id: string | null;
		partner_name: string | null;
		auto_publish: boolean;
	};

	const STATUS_LABEL: Record<string, string> = {
		active: 'Aktif',
		inactive: 'Nonaktif',
		draft: 'Draf',
		pending_review: 'Pending',
		rejected: 'Ditolak',
		archived: 'Arsip',
		hidden: 'Disembunyikan'
	};

	const INTEGRATION_LABEL: Record<string, string> = {
		manual: 'Manual',
		wordpress: 'WordPress',
		blogger: 'Blogger',
		custom_web: 'Custom Web',
		webhook: 'Webhook',
		rss: 'RSS'
	};

	let rows = $state<MediaRow[]>([]);
	let error = $state('');
	let loading = $state(true);
	let filterStatus = $state('');
	let busyId = $state<string | null>(null);
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
			const params = new URLSearchParams({ auto_publish: '1', scope: 'all', limit: '500' });
			if (filterStatus) params.set('status', filterStatus);
			const res = await apiRequest(`/v1/platform/media?${params.toString()}`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const data = await res.json();
			rows = data.media ?? [];
			page = 1;
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	/** Mematikan mode otomatis: artikel berikutnya kembali butuh ACC mitra. */
	async function turnOff(row: MediaRow) {
		busyId = row.id;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/media/${row.id}`, {
				method: 'PUT',
				body: JSON.stringify({ auto_publish: false })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			await load();
		} finally {
			busyId = null;
		}
	}

	const pageRows = $derived(rows.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE));
</script>

{#snippet mediaCell(row: MediaRow)}
	<div>
		<p class="font-medium">{row.name}</p>
		<p class="text-xs text-muted-foreground">{row.website_url ?? '—'}</p>
	</div>
{/snippet}

{#snippet partnerCell(row: MediaRow)}
	{#if row.partner_name}
		<span>{row.partner_name}</span>
	{:else}
		<span title="Tanpa mitra: artikel langsung terbit tanpa ACC"><Badge tone="warning" label="Tanpa mitra" /></span>
	{/if}
{/snippet}

{#snippet integrationCell(row: MediaRow)}
	<span>{INTEGRATION_LABEL[row.integration_type] ?? row.integration_type}</span>
{/snippet}

{#snippet statusCell(row: MediaRow)}
	<Badge tone={row.status === 'active' ? 'success' : 'neutral'} label={STATUS_LABEL[row.status] ?? row.status} />
{/snippet}

{#snippet autoCell(row: MediaRow)}
	<div class="flex items-center gap-2">
		<input
			type="checkbox"
			checked={row.auto_publish}
			disabled={busyId === row.id}
			aria-label={`Mode otomatis ${row.name}`}
			onchange={(e) => {
				if (!e.currentTarget.checked) void turnOff(row);
			}}
		/>
		<span class="text-xs text-muted-foreground">{row.auto_publish ? 'Menyala' : 'Mati'}</span>
	</div>
{/snippet}

<section class="w-full">
	<PageHeader
		title="Media Otomatis"
		description="Media mitra yang artikelnya terbit tanpa persetujuan mitra. Menyalakan mode ini hanya bisa dilakukan mitra yang sudah Anda izinkan, dari portal mitra. Di sini Anda bisa memantau dan mematikannya."
	>
		<button class="app-btn app-btn-soft" onclick={load} disabled={loading}>
			<RefreshCw size={15} /> Segarkan
		</button>
	</PageHeader>

	<Feedback {loading} {error} />

	<div class="mt-4 flex flex-wrap items-end gap-3">
		<label class="block" style="min-width: 180px;">
			<span class="mb-1 block text-xs text-muted-foreground">Status media</span>
			<select
				class="app-input w-full"
				value={filterStatus || 'all'}
				onchange={(e) => {
					const v = e.currentTarget.value;
					filterStatus = v === 'all' ? '' : v;
					page = 1;
					void load();
				}}
			>
				<option value="all">Semua status</option>
				<option value="active">Aktif</option>
				<option value="pending_review">Pending</option>
				<option value="inactive">Nonaktif</option>
			</select>
		</label>
	</div>

	{#if loading && rows.length === 0}
		<p class="mt-6 text-sm text-muted-foreground">Memuat media otomatis…</p>
	{:else if rows.length === 0}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">
				Belum ada media dengan mode otomatis. Izinkan mitra lewat kolom 'Izinkan Mode Otomatis' di halaman Mitra, lalu
				mitra menyalakannya sendiri pada media yang dia pilih. Media itu akan muncul di sini.
			</p>
		</div>
	{:else}
		<DataTable
			columns={[
				{ key: 'name', label: 'Media', render: mediaCell },
				{ key: 'partner', label: 'Mitra', render: partnerCell },
				{ key: 'integration', label: 'Platform', render: integrationCell },
				{ key: 'status', label: 'Status', render: statusCell },
				{ key: 'auto', label: 'Mode Otomatis', render: autoCell }
			]}
			rows={pageRows}
			rowKey={(row: MediaRow) => row.id}
			empty="Belum ada media dengan mode otomatis."
			minWidth="900px"
		/>
		<Pagination
			total={rows.length}
			limit={PAGE_SIZE}
			offset={(page - 1) * PAGE_SIZE}
			onChange={(next) => (page = next / PAGE_SIZE + 1)}
		/>
	{/if}

	<p class="mt-3 flex items-start gap-1.5 text-xs text-muted-foreground">
		<Unlink size={13} class="mt-0.5 shrink-0" />
		Mematikan mode otomatis tidak mengubah artikel yang sudah terbit; hanya publikasi berikutnya yang kembali menunggu ACC
		mitra.
	</p>
</section>
