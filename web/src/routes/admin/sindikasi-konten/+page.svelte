<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Sindikasi Konten (RSS) — kelola mitra yang diizinkan menjalankan Sindikasi
	// Konten. Aktifkan hanya untuk mitra yang telah menyetujui ketentuan RSS feed.
	// Port apps/web/src/app/admin/sindikasi-konten/page.tsx.
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type PartnerRow = {
		id: string;
		name: string | null;
		email: string | null;
		status: string;
		rss_enabled: boolean;
		media_count: number;
	};

	let rows = $state<PartnerRow[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let error = $state('');
	let busyId = $state<string | null>(null);
	let notice = $state('');

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
			const res = await apiRequest('/v1/platform/partners?limit=500&offset=0');
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal memuat daftar mitra.'));
				return;
			}
			const data = await res.json();
			rows = data.partners ?? [];
			total = data.total ?? 0;
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal memuat daftar mitra.');
		} finally {
			loading = false;
		}
	}

	async function toggleRss(partner: PartnerRow) {
		busyId = partner.id;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/platform/partners/${partner.id}`, {
				method: 'PUT',
				body: JSON.stringify({ rss_enabled: !partner.rss_enabled })
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal mengubah status RSS.'));
				return;
			}
			notice = !partner.rss_enabled
				? `Sindikasi Konten diaktifkan untuk ${partner.name ?? partner.email}.`
				: `Sindikasi Konten dinonaktifkan untuk ${partner.name ?? partner.email}.`;
			await load();
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal mengubah status RSS.');
		} finally {
			busyId = null;
		}
	}
</script>

{#snippet partnerCell(row: PartnerRow)}
	<div class="flex flex-col gap-1">
		<span class="font-medium">{row.name ?? '—'}</span>
		<span class="text-xs text-muted-foreground">{row.email ?? ''}</span>
	</div>
{/snippet}

{#snippet mediaCountCell(row: PartnerRow)}
	<span class="tabular-nums">{row.media_count}</span>
{/snippet}

{#snippet accountStatusCell(row: PartnerRow)}
	<Badge tone={row.status === 'active' ? 'success' : 'neutral'} label={row.status === 'active' ? tr('Aktif') : tr('Nonaktif')} />
{/snippet}

{#snippet rssCell(row: PartnerRow)}
	<div class="flex items-center justify-end gap-2">
		<Badge tone={row.rss_enabled ? 'success' : 'neutral'} label={row.rss_enabled ? tr('Aktif') : tr('Nonaktif')} />
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			style="color: {row.rss_enabled ? 'var(--destructive)' : 'var(--success)'};"
			disabled={busyId === row.id}
			onclick={() => void toggleRss(row)}
			>{busyId === row.id ? 'Memproses…' : row.rss_enabled ? 'Nonaktifkan' : 'Aktifkan'}</button
		>
	</div>
{/snippet}

<section>
	<PageHeader
		title="Sindikasi Konten (RSS)"
		description={tr("Kelola mitra yang diizinkan menjalankan Sindikasi Konten. Aktifkan hanya untuk mitra yang telah menyetujui ketentuan RSS feed.")}
	/>

	<Feedback {loading} {error} {notice} />

	<DataTable
		columns={[
			{ key: 'partner', label: tr('Mitra'), render: partnerCell },
			{ key: 'media', label: tr('Media'), render: mediaCountCell },
			{ key: 'status', label: tr('Status Akun'), render: accountStatusCell },
			{ key: 'rss', label: 'Sindikasi Konten', align: 'right' as const, render: rssCell }
		]}
		rows={rows}
		rowKey={(row: PartnerRow) => row.id}
		{loading}
		empty={tr("Belum ada mitra terdaftar.")}
		minWidth="760px"
	/>

	<p class="mt-3 text-xs text-muted-foreground">{total} mitra</p>
</section>
