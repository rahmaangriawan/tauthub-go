<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Detail satu order kampanye (CAM-) di Monitoring Order: daftar media
	// terorder, judul artikel, status terbit, dan rupiahnya — Rencana semua
	// media terorder, Terpakai = publikasi non-cancelled (client_price),
	// Sisa budget bila campaigns.budget diisi.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ArrowLeft, ExternalLink } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import { fmtDateTime, fmtRp, fmtNum } from '$lib/admin-format';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { platformError } from '../../../_platform';

	type MediaRow = {
		media_id: string; media_name: string; website_url: string; category: string;
		price: number; document_id: string | null; article_title: string | null;
		article_status: string; publication_id: string | null; pub_status: string;
		published_url: string | null; published_at: string | null; scheduled_at: string | null;
	};
	type Detail = {
		campaign: {
			id: string; order_code: string | null; name: string; taut_status: string | null;
			campaign_status: string | null; brand_name: string | null; brand_public_id: string | null;
			budget: number; starts_at: string | null; ends_at: string | null; created_at: string;
		};
		mode: string;
		media: MediaRow[];
		totals: {
			planned: number; realized: number; remaining: number; budget: number;
			media_count: number; publication_count: number; published_count: number;
		};
	};

	const PUB_STATUS_TONE: Record<string, 'success' | 'neutral' | 'warning' | 'accent' | 'danger'> = {
		published: 'success', pending: 'warning', waiting_partner: 'warning',
		partner_approved: 'accent', partner_rejected: 'danger', queued: 'accent',
		processing: 'accent', failed: 'danger', retrying: 'warning', cancelled: 'neutral'
	};
	const PUB_STATUS_LABEL: Record<string, string> = $derived({
		pending: 'Menunggu', waiting_partner: 'Menunggu mitra', partner_approved: 'Disetujui mitra',
		partner_rejected: 'Ditolak mitra', queued: 'Dalam antrean', processing: 'Diproses',
		published: 'Terbit', failed: 'Gagal', retrying: 'Dicoba ulang', cancelled: 'Dibatalkan',
		submitted: 'Levelling', scheduled: 'Terjadwal', distributed: 'Tersebar', production: 'Produksi',
		review: 'Review', placement: 'Media terkunci', profiled: 'Profil selesai', draft: 'Draf'
	});

	const campaignId = $derived(page.params.campaignId as string);

	let data = $state<Detail | null>(null);
	let error = $state('');
	let loading = $state(true);

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
			const res = await apiRequest(`/v1/platform/orders/campaigns/${campaignId}`);
			if (!res.ok) {
				error = await platformError(res);
				return;
			}
			data = await res.json();
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	const camp = $derived(data?.campaign);
	const totals = $derived(data?.totals);
	const st = $derived(camp?.taut_status ?? camp?.campaign_status ?? '');
</script>

<section>
	<ExplorerFrame parents={[{ label: tr('Publikasi'), href: '/admin/publications' }, { label: tr('Monitoring Order'), href: '/admin/orders' }]} crumb={tr('Detail kampanye')} path={`/admin/orders/kampanye/${campaignId}`}>
	<PageHeader
		title={camp?.name ?? tr('Detail kampanye')}
		description={`${camp?.order_code ?? campaignId}${camp?.brand_name ? ` · ${camp.brand_name}` : ''}`}
	>
		{#if st}<Badge label={PUB_STATUS_LABEL[st] ?? st} tone={PUB_STATUS_TONE[st] ?? 'neutral'} />{/if}
		<a
			href="/admin/orders?tab=kampanye"
			class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
			style="text-decoration: none;"
		>
			<ArrowLeft size={13} />{tr("Kembali ke daftar")}</a>
	</PageHeader>

	{#if loading && !data && !error}
		<p class="mt-6 text-sm text-muted-foreground">Memuat detail kampanye…</p>
	{:else if error}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{error}</p>
		</div>
	{:else if camp && totals}
		<Feedback {error} />

		<div class="mt-4 grid grid-cols-2 gap-3 lg:grid-cols-4">
			<div class="app-card px-4 py-3">
				<p class="text-xs text-muted-foreground">Rencana</p>
				<p class="mt-1 text-xl font-bold tabular-nums">{fmtRp(totals.planned)}</p>
				<p class="mt-1 text-xs text-muted-foreground">{fmtNum(totals.media_count)} media terorder</p>
			</div>
			<div class="app-card px-4 py-3">
				<p class="text-xs text-muted-foreground">Terpakai</p>
				<p class="mt-1 text-xl font-bold tabular-nums">{fmtRp(totals.realized)}</p>
				<p class="mt-1 text-xs text-muted-foreground">{fmtNum(totals.publication_count)} publikasi non-cancelled</p>
			</div>
			<div class="app-card px-4 py-3">
				<p class="text-xs text-muted-foreground">{totals.budget > 0 ? 'Sisa budget' : 'Sisa rencana'}</p>
				<p class="mt-1 text-xl font-bold tabular-nums">{fmtRp(totals.remaining)}</p>
				{#if totals.budget > 0}
					<p class="mt-1 text-xs text-muted-foreground">budget {fmtRp(totals.budget)}</p>
				{/if}
			</div>
			<div class="app-card px-4 py-3">
				<p class="text-xs text-muted-foreground">Terbit</p>
				<p class="mt-1 text-xl font-bold tabular-nums">{fmtNum(totals.published_count)}</p>
				<p class="mt-1 text-xs text-muted-foreground">dari {fmtNum(totals.media_count)} media</p>
			</div>
		</div>

		{#if data?.mode === 'publications_legacy'}
			<p class="mt-3 text-xs text-muted-foreground">Kampanye ini dibuat dari jalur lama — daftar media diambil dari publikasinya.</p>
		{/if}

		<div class="app-card mt-3 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 1100px;">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">{tr("Media")}</th>
						<th class="px-4 py-2.5 font-medium">Kategori</th>
						<th class="px-4 py-2.5 font-medium">Artikel</th>
						<th class="px-4 py-2.5 font-medium">{tr("Harga")}</th>
						<th class="px-4 py-2.5 font-medium">Status terbit</th>
						<th class="px-4 py-2.5 font-medium">Waktu</th>
					</tr>
				</thead>
				<tbody>
					{#each data?.media ?? [] as row (row.media_id + (row.publication_id ?? ''))}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<p>{row.media_name ?? '—'}</p>
								{#if row.website_url}
									<p class="text-xs text-muted-foreground">{row.website_url}</p>
								{/if}
							</td>
							<td class="px-4 py-2.5">{row.category || '—'}</td>
							<td class="max-w-[240px] px-4 py-2.5">
								{#if row.publication_id}
									<a class="font-medium" style="color: var(--primary);" href={`/admin/publications/${row.publication_id}`}>
										{row.article_title ?? row.publication_id.slice(0, 8)}
									</a>
								{:else}
									{row.article_title ?? 'Belum ada artikel'}
								{/if}
							</td>
							<td class="px-4 py-2.5 tabular-nums">{fmtRp(row.price)}</td>
							<td class="px-4 py-2.5">
								{#if row.pub_status}
									<Badge label={PUB_STATUS_LABEL[row.pub_status] ?? row.pub_status} tone={PUB_STATUS_TONE[row.pub_status] ?? 'neutral'} />
								{:else}
									<span class="text-xs text-muted-foreground">Belum dibuat</span>
								{/if}
								{#if row.published_url}
									<a class="mt-1 flex items-center gap-1 text-xs" style="color: var(--success);" href={row.published_url} target="_blank" rel="noopener noreferrer">
										<ExternalLink size={12} />{tr("URL terbit")}
									</a>
								{/if}
							</td>
							<td class="px-4 py-2.5 text-xs text-muted-foreground">
								{#if row.published_at}{fmtDateTime(row.published_at)}
								{:else if row.scheduled_at}{fmtDateTime(row.scheduled_at)}
								{:else}—{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && (data?.media.length ?? 0) === 0}
				<div class="flex flex-col items-center px-4 py-12 text-center">
					<p class="text-sm text-muted-foreground">Tidak ada media pada kampanye ini.</p>
				</div>
			{/if}
		</div>
	{/if}
	</ExplorerFrame>
</section>
