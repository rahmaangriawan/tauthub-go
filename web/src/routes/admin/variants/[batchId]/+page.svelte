<script lang="ts">
	// Detail batch variant — audit sudut pandang per media + riwayat job generasi.
	// Port apps/web/src/app/admin/variants/[batchId]/page.tsx.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ArrowLeft, ChevronDown, ChevronRight, ExternalLink } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type BatchJob = {
		id: string;
		status: string;
		attempts: number;
		max_attempts: number;
		last_error: string | null;
		run_at: string | null;
		created_at: string | null;
	};
	type VariantDetail = {
		id: string;
		media_id: string;
		media_name: string | null;
		media_category: string | null;
		media_integration_type: string | null;
		angle_id: string;
		angle_label: string;
		status: string;
		document_id: string | null;
		document_title: string | null;
		publication_id: string | null;
		publication_status: string | null;
		published_url: string | null;
		error_message: string | null;
		created_at: string;
		updated_at: string;
		jobs: BatchJob[];
	};
	type Detail = {
		batch: {
			batch_id: string;
			brand_id: string;
			brand_name: string | null;
			brand_public_id: string | null;
			user_email: string | null;
			mode: string;
			seed: number;
			status: string;
			source_document_id: string | null;
			source_material: string | null;
			created_at: string;
			completed_at: string | null;
			variant_counts: { total: number; done: number; failed: number };
		};
		variants: VariantDetail[];
	};

	const BATCH_STATUS_META: Record<
		string,
		{ label: string; tone: 'neutral' | 'accent' | 'success' | 'warning' | 'danger' }
	> = {
		queued: { label: 'Diantrekan', tone: 'accent' },
		processing: { label: 'Diproses', tone: 'warning' },
		completed: { label: 'Selesai', tone: 'success' },
		partial: { label: 'Sebagian gagal', tone: 'warning' },
		failed: { label: 'Gagal', tone: 'danger' }
	};
	const VARIANT_STATUS_TONE: Record<string, 'neutral' | 'success' | 'warning' | 'danger'> = {
		queued: 'neutral',
		processing: 'warning',
		completed: 'success',
		failed: 'danger'
	};
	const JOB_STATUS_TONE: Record<string, 'neutral' | 'success' | 'warning' | 'danger'> = {
		completed: 'success',
		running: 'warning',
		pending: 'neutral',
		failed: 'danger'
	};

	const batchId = $derived(page.params.batchId as string);

	let data = $state<Detail | null>(null);
	let error = $state('');
	let loading = $state(true);
	let expanded = $state<Set<string>>(new Set());
	let showSource = $state(false);

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
			const res = await apiRequest(`/v1/platform/operations/variant-batches/${batchId}`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			data = await res.json();
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	function toggle(id: string) {
		const next = new Set(expanded);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		expanded = next;
	}

	const batch = $derived(data?.batch);
	const meta = $derived(batch ? (BATCH_STATUS_META[batch.status] ?? { label: batch.status, tone: 'neutral' as const }) : null);
</script>

<section class="mx-auto max-w-6xl">
	<PageHeader
		title="Detail batch variant"
		description={batch
			? `Batch ${batch.batch_id} · ${batch.brand_name ?? '—'} · mode ${batch.mode === 'generate' ? 'dibuatkan dari bahan' : 'parafrase'} · seed ${batch.seed}`
			: `Batch ${batchId}`}
	>
		{#if meta}<Badge tone={meta.tone} label={meta.label} />{/if}
		<a href="/admin/variants" class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" style="text-decoration: none;">
			<ArrowLeft size={13} /> Kembali ke daftar
		</a>
	</PageHeader>

	{#if loading && !data && !error}
		<p class="mt-6 text-sm text-muted-foreground">Memuat detail batch variant…</p>
	{:else if error.includes('Akses platform') || error.includes('tidak ditemukan')}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{error}</p>
		</div>
	{:else if batch}
		<Feedback {error} />

		<div class="app-card mt-4 p-4">
			<div class="grid gap-3 sm:grid-cols-3">
				<div>
					<p class="text-xs text-muted-foreground">Brand / Klien</p>
					<p class="text-sm font-medium">{batch.brand_name ?? '—'}</p>
					{#if batch.brand_public_id}<p class="text-xs text-muted-foreground">{batch.brand_public_id}</p>{/if}
				</div>
				<div>
					<p class="text-xs text-muted-foreground">Pemohon</p>
					<p class="text-sm font-medium">{batch.user_email ?? '—'}</p>
				</div>
				<div>
					<p class="text-xs text-muted-foreground">Varian</p>
					<p class="text-sm font-medium">
						{batch.variant_counts.done}/{batch.variant_counts.total} jadi{batch.variant_counts.failed > 0
							? ` · ${batch.variant_counts.failed} gagal`
							: ''}
					</p>
				</div>
				<div>
					<p class="text-xs text-muted-foreground">Mode</p>
					<p class="text-sm font-medium">{batch.mode === 'generate' ? 'Dibuatkan dari bahan' : 'Parafrase'}</p>
				</div>
				<div>
					<p class="text-xs text-muted-foreground">Seed (PRNG sudut)</p>
					<code class="text-xs">{batch.seed}</code>
				</div>
				<div>
					<p class="text-xs text-muted-foreground">Dibuat</p>
					<p class="text-sm font-medium">{fmtDateTime(batch.created_at)}</p>
					{#if batch.completed_at}
						<p class="text-xs text-muted-foreground">selesai {fmtDateTime(batch.completed_at)}</p>
					{/if}
				</div>
			</div>
			{#if batch.source_document_id}
				<div class="mt-3">
					<p class="text-xs text-muted-foreground">Artikel sumber (parafrase)</p>
					<code class="text-xs">{batch.source_document_id}</code>
				</div>
			{/if}
			{#if batch.source_material}
				<div class="mt-3">
					<p class="mb-1 text-xs text-muted-foreground">Bahan sumber (generate)</p>
					<div class="overflow-auto rounded-lg" style="max-height: {showSource ? '420px' : '140px'};">
						<pre class="whitespace-pre-wrap p-3 text-xs" style="background-color: var(--secondary);">
{batch.source_material.length > 600 && !showSource
								? `${batch.source_material.slice(0, 600)}…`
								: batch.source_material}</pre
						>
					</div>
					{#if batch.source_material.length > 600}
						<button
							class="app-btn app-btn-soft mt-2 !min-h-7 !px-2.5 !text-xs"
							onclick={() => (showSource = !showSource)}>{showSource ? 'Tutup bahan' : 'Lihat bahan lengkap'}</button
						>
					{/if}
				</div>
			{/if}
		</div>

		<div class="app-card mt-4 p-4">
			<h2 class="mb-2 text-base font-semibold">Audit sudut pandang per media ({batch.variant_counts.total})</h2>
			<div class="overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 1100px;">
					<thead>
						<tr class="text-xs text-muted-foreground">
							<th class="px-4 py-2.5 text-left font-medium">Media</th>
							<th class="px-4 py-2.5 text-left font-medium">Sudut pandang</th>
							<th class="px-4 py-2.5 text-left font-medium">Artikel</th>
							<th class="px-4 py-2.5 text-left font-medium">Publikasi</th>
							<th class="px-4 py-2.5 text-left font-medium">Status</th>
							<th class="px-4 py-2.5 text-left font-medium">Waktu</th>
							<th class="px-4 py-2.5 text-left font-medium"></th>
						</tr>
					</thead>
					<tbody>
						{#if data!.variants.length === 0}
							<tr
								><td colspan="7" class="px-4 py-2.5 text-xs text-muted-foreground">Belum ada varian tercatat.</td
								></tr
							>
						{/if}
						{#each data!.variants as v (v.id)}
							{@const open = expanded.has(v.id)}
							<tr class="border-t">
								<td class="px-4 py-2.5">
									<button
										type="button"
										class="text-left"
										style="background: none; border: none; padding: 0; cursor: pointer; color: inherit; font: inherit;"
										onclick={() => toggle(v.id)}
									>
										<span class="flex items-center gap-2">
											{#if open}<ChevronDown size={14} class="text-muted-foreground" />
											{:else}<ChevronRight size={14} class="text-muted-foreground" />{/if}
											<span class="min-w-0">
												<span class="block font-medium">{v.media_name ?? '—'}</span>
												{#if v.media_category}
													<span class="block text-xs text-muted-foreground">{v.media_category}</span>
												{/if}
											</span>
										</span>
									</button>
								</td>
								<td class="px-4 py-2.5">
									<p>{v.angle_label}</p>
									<p class="text-xs text-muted-foreground">{v.angle_id}</p>
								</td>
								<td class="px-4 py-2.5 text-xs">{v.document_title ?? '—'}</td>
								<td class="px-4 py-2.5">
									{#if v.publication_id}
										<a
											href={`/admin/publications/${v.publication_id}`}
											class="whitespace-nowrap text-xs font-semibold"
											style="color: var(--primary);"
											>{v.publication_status ?? 'lihat detail'} <ExternalLink size={13} class="inline" /></a
										>
									{:else}
										<span class="text-xs text-muted-foreground">—</span>
									{/if}
								</td>
								<td class="px-4 py-2.5">
									<Badge tone={VARIANT_STATUS_TONE[v.status] ?? 'neutral'} value={v.status} />
									{#if v.error_message}
										<p class="mt-1 text-xs" style="color: var(--destructive);">{v.error_message}</p>
									{/if}
								</td>
								<td class="px-4 py-2.5 text-xs text-muted-foreground">{fmtDateTime(v.updated_at)}</td>
								<td class="px-4 py-2.5">
									{#if v.jobs.length > 0}
										<Badge
											tone={v.jobs.some((j) => j.status === 'failed') ? 'danger' : 'neutral'}
											label={`${v.jobs.length} job`}
										/>
									{:else}
										<span class="text-xs text-muted-foreground">—</span>
									{/if}
								</td>
							</tr>
							{#if open}
								<tr>
									<td colspan="7" class="px-4 py-3" style="background-color: var(--secondary);">
										{#if v.jobs.length === 0}
											<p class="text-xs text-muted-foreground">Belum ada job generasi tercatat.</p>
										{/if}
										{#each v.jobs as j (j.id)}
											<div class="flex flex-wrap items-start justify-between gap-3 border-b py-2 last:border-0">
												<div class="min-w-0">
													<div class="mb-1 flex flex-wrap items-center gap-2">
														<Badge tone={JOB_STATUS_TONE[j.status] ?? 'neutral'} value={j.status} />
														<Badge tone="neutral" label={`attempt ${j.attempts}/${j.max_attempts}`} />
														<code class="text-xs text-muted-foreground">{j.id.slice(0, 8)}</code>
													</div>
													{#if j.last_error}
														<p class="text-xs" style="color: var(--destructive);">{j.last_error}</p>
													{/if}
												</div>
												<span class="shrink-0 text-xs text-muted-foreground">
													{j.run_at ? `run ${fmtDateTime(j.run_at)}` : fmtDateTime(j.created_at)}
												</span>
											</div>
										{/each}
									</td>
								</tr>
							{/if}
						{/each}
					</tbody>
				</table>
			</div>
			<p class="mt-3 text-xs text-muted-foreground">
				Satu batch = satu sumber → satu artikel per media dengan sudut pandang berbeda (PRNG seeded). Klik varian untuk
				melihat riwayat job generasi di queue.
			</p>
		</div>
	{/if}
</section>
