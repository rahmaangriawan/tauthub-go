<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Detail publikasi admin — riwayat attempts, job delivery, payload, callback
	// adaptor, analisis PR, auto-replacement, dan variant.
	// Port apps/web/src/app/admin/publications/[publicationId]/page.tsx.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ArrowLeft, Ban, ExternalLink, ImagePlus, RefreshCw } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type Attempt = {
		attempt_number: number;
		status: string;
		error_code: string | null;
		error_message: string | null;
		started_at: string | null;
		completed_at: string | null;
	};
	type Job = {
		id: string;
		status: string;
		publish_status: string;
		remote_post_id: string | null;
		remote_url: string | null;
		error_code: string | null;
		error_message: string | null;
		warning_message: string | null;
		attempts: number;
		started_at: string | null;
		completed_at: string | null;
		payload_snapshot: Record<string, unknown> | null;
	};
	type Callback = {
		status_code: number | null;
		accepted: boolean | null;
		error_message: string | null;
		ip_address: string | null;
		user_agent: string | null;
		created_at: string;
	};
	type Detail = {
		publication: {
			id: string;
			status: string;
			client_price: number;
			partner_price: number | null;
			markup_percent: number | null;
			publish_status: string;
			connection_id: string | null;
			scheduled_at: string | null;
			published_at: string | null;
			published_url: string | null;
			notes: string | null;
			cancel_reason: string | null;
			rejection_reason: string | null;
			adapter_url: string | null;
			brand_name: string | null;
			brand_public_id: string | null;
			media_name: string | null;
			media_integration_type: string | null;
			partner_name: string | null;
			partner_public_id: string | null;
			document_id: string;
			document_title: string | null;
			document_slug: string | null;
			current_version: number;
			campaign_name: string | null;
			error_code: string | null;
			error_message: string | null;
			created_by_email: string | null;
			created_at: string;
			updated_at: string;
		};
		attempts: Attempt[];
		job: Job | null;
		callbacks: Callback[];
		variant: {
			angle_id: string;
			angle_label: string;
			media_id: string;
			variant_status: string;
			batch_id: string;
			mode: string;
			seed: number;
			batch_created_at: string;
		} | null;
		replaces: {
			id: string;
			status: string;
			media_name: string | null;
			document_title: string | null;
			created_at: string;
			brand_public_id: string | null;
		} | null;
		replaced_by: {
			id: string;
			status: string;
			media_name: string | null;
			document_title: string | null;
			published_at: string | null;
			published_url: string | null;
			created_at: string;
			brand_public_id: string | null;
		} | null;
		pr_analysis: {
			message: {
				consistency_score: number | null;
				results: Array<{ message: string; found: boolean; evidence: string | null }>;
				status: string;
				error_message: string | null;
				analyzed_at: string | null;
			} | null;
			sentiment: {
				sentiment: string;
				confidence: number | null;
				status: string;
				error_message: string | null;
				analyzed_at: string | null;
			} | null;
			verification: {
				status: string;
				compliance_score: number | null;
				checks: Array<{ key: string; label: string; ok: boolean; detail: string | null }>;
				error_message: string | null;
				verified_at: string | null;
			} | null;
		} | null;
	};

	const VERIFICATION_LABEL: Record<string, string> = {
		passed: 'URL terverifikasi',
		failed: 'URL bermasalah',
		unavailable: 'Verifikasi tidak tersedia'
	};
	const SENTIMENT_LABEL: Record<string, string> = { positive: 'Positif', neutral: 'Netral', negative: 'Negatif' };
	const STATUS_TONE: Record<string, 'neutral' | 'accent' | 'success' | 'warning' | 'danger'> = {
		published: 'success',
		pending: 'warning',
		waiting_partner: 'warning',
		partner_approved: 'accent',
		partner_rejected: 'danger',
		queued: 'accent',
		processing: 'accent',
		failed: 'danger',
		retrying: 'warning',
		cancelled: 'neutral'
	};
	const ATTEMPT_LABEL: Record<string, string> = $derived({
		published: tr('Terbit'),
		retrying: 'Mencoba ulang',
		failed: tr('Gagal'),
		queued: 'Diantrekan',
		processing: 'Diproses',
		partner_approved: tr('Disetujui mitra'),
		waiting_partner: tr('Menunggu persetujuan')
	});
	// Label status publikasi untuk badge: bahasa manusia, bukan nilai mentah.
	const PUB_STATUS_LABEL: Record<string, string> = $derived({
		pending: tr('Menunggu diproses'),
		waiting_partner: tr('Menunggu persetujuan media'),
		partner_approved: tr('Disetujui media'),
		partner_rejected: tr('Ditolak media'),
		queued: tr('Antrean'),
		processing: tr('Diproses'),
		published: tr('Terbit'),
		failed: tr('Gagal'),
		retrying: tr('Mencoba ulang'),
		cancelled: tr('Dibatalkan'),
		revision: tr('Perlu revisi')
	});

	const publicationId = $derived(page.params.publicationId as string);

	let data = $state<Detail | null>(null);
	let error = $state('');
	let loading = $state(true);
	let retrying = $state(false);
	let retryError = $state('');
	let retryDone = $state('');
	let repairing = $state(false);
	let repairError = $state('');
	let repairDone = $state('');
	let syncing = $state(false);
	let syncError = $state('');
	let syncDone = $state('');
	let cancelOpen = $state(false);
	let cancelReason = $state('');
	let cancelBusy = $state(false);
	let cancelError = $state('');
	let cancelDone = $state('');

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
			const res = await apiRequest(`/v1/platform/operations/publications/${publicationId}`);
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

	async function retryNow() {
		retrying = true;
		retryError = '';
		retryDone = '';
		try {
			const res = await apiRequest(`/v1/platform/operations/publications/${publicationId}/retry`, {
				method: 'POST'
			});
			if (!res.ok) {
				retryError = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			retryDone = 'Publikasi diantrekan ulang. Status berubah menjadi queued dan job delivery baru dibuat.';
			void load();
		} finally {
			retrying = false;
		}
	}

	async function repairImages() {
		repairing = true;
		repairError = '';
		repairDone = '';
		try {
			const res = await apiRequest(
				`/v1/platform/operations/publications/${publicationId}/repair-images`,
				{ method: 'POST' }
			);
			if (!res.ok) {
				repairError = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			repairDone =
				'Perbaikan diantrekan. Gambar artikel akan diunggah ulang ke media mitra dan dipasang ke post yang sama (tidak membuat post baru).';
			void load();
		} finally {
			repairing = false;
		}
	}

	const idr = (n: unknown) =>
		new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0, roundingMode: 'trunc' }).format(
			Number(n ?? 0)
		);
	// Sinkron ulang untuk WordPress: mengirim ulang gambar utama + kategori ke
	// post yang sudah ada (kategori = taxonomy, gambar utama = featured_media).
	async function resyncWordpress() {
		if (
			!confirm(
				'Sinkronkan ulang gambar utama & kategori ke WordPress?\n\n' +
					'Post di WordPress akan DIPERBARUI (judul dan isi ikut dikirim ulang), bukan dibuat baru. ' +
					'Pakai ini kalau artikel terbit tanpa gambar utama atau tanpa kategori.'
			)
		)
			return;
		syncing = true;
		syncError = '';
		syncDone = '';
		try {
			const res = await apiRequest(
				`/v1/platform/operations/publications/${publicationId}/resync-wordpress`,
				{ method: 'POST' }
			);
			if (!res.ok) {
				syncError = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			syncDone =
				'Sinkronisasi diantrekan. Gambar utama dan kategori akan dikirim ulang ke post WordPress yang sama (post tidak digandakan). Muat ulang halaman ini beberapa saat lagi untuk melihat hasilnya.';
			void load();
		} finally {
			syncing = false;
		}
	}
	async function cancelPublication() {
		cancelBusy = true;
		cancelError = '';
		cancelDone = '';
		try {
			const res = await apiRequest(`/v1/platform/operations/publications/${publicationId}/cancel`, {
				method: 'POST',
				body: JSON.stringify({ reason: cancelReason.trim() })
			});
			if (!res.ok) {
				cancelError = await readApiError(res, 'Gagal membatalkan publikasi.');
				return;
			}
			cancelOpen = false;
			cancelDone =
				'Publikasi dibatalkan. Kredit reservasi dikembalikan ke Saldo Akun klien, dan klien serta mitra menerima notifikasi.';
			await load();
		} finally {
			cancelBusy = false;
		}
	}
	const pub = $derived(data?.publication);
	const isWordpress = $derived(pub?.media_integration_type === 'wordpress');
</script>

<section>
	<ExplorerFrame parents={[{ label: tr('Publikasi'), href: '/admin/publications' }, { label: tr('Daftar Publikasi'), href: '/admin/publications/daftar' }]} crumb={tr('Detail publikasi')} path={`/admin/publications/${publicationId}`}>
	<PageHeader
		title={pub?.document_title ?? tr('Detail publikasi')}
		description={`Publication ${publicationId}${pub?.campaign_name ? ` · Campaign: ${pub.campaign_name}` : ''}`}
	>
		{#if pub}<Badge tone={STATUS_TONE[pub.status] ?? 'neutral'} value={PUB_STATUS_LABEL[pub.status] ?? pub.status} />{/if}
		<a
			href="/admin/publications/daftar"
			class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
			style="text-decoration: none;"
		>
			<ArrowLeft size={13} />{tr("Kembali ke daftar")}</a>
	</PageHeader>

	{#if loading && !data && !error}
		<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat detail publikasi…")}</p>
	{:else if error.includes('Akses platform') || error.includes('tidak ditemukan')}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{error}</p>
		</div>
	{:else}
		<Feedback {error} />

		{#if pub}
			{#if pub.error_code}
				<div
					class="app-card mt-4 px-4 py-3 text-sm"
					style="color: var(--destructive);"
				>
					<strong>{pub.error_code}</strong> — {pub.error_message ?? 'tidak ada detail'}<br /> {tr("Gagal pada")} {pub.updated_at ? fmtDateTime(pub.updated_at) : '—'}
				</div>
			{/if}

			{#if pub.status === 'failed' || pub.status === 'retrying'}
				<div class="mt-4 flex flex-wrap items-center gap-2">
					<button class="app-btn app-btn-primary" onclick={retryNow} disabled={retrying}>
						<RefreshCw size={15} />{retrying ? 'Mengantrekan…' : 'Retry manual'}
					</button>
					<span class="text-xs text-muted-foreground">
						Mengantrekan ulang publikasi ini sebagai job delivery baru (state machine: {pub.status} → queued).
					</span>
				</div>
			{/if}
			{#if retryError}
				<p class="mt-3 text-sm" style="color: var(--destructive);">{retryError}</p>
			{/if}
			{#if retryDone}
				<p class="mt-3 text-sm" style="color: var(--success);">{retryDone}</p>
			{/if}

			{#if pub.status !== 'published' && pub.status !== 'cancelled'}
				<div class="mt-4 flex flex-wrap items-center gap-2">
					<button
						class="app-btn app-btn-soft"
						style="color: var(--destructive);"
						type="button"
						disabled={cancelBusy}
						onclick={() => {
							cancelReason = '';
							cancelError = '';
							cancelDone = '';
							cancelOpen = true;
						}}
					>
						<Ban size={15} />{tr('Batalkan publikasi')}
					</button>
					<span class="text-xs text-muted-foreground">
						{tr('Membatalkan pesanan dari panel operasi. Kredit reservasi dikembalikan ke Saldo Akun klien, dan klien serta mitra menerima notifikasi.')}
					</span>
				</div>
			{/if}
			{#if cancelError}
				<p class="mt-3 text-sm" style="color: var(--destructive);">{cancelError}</p>
			{/if}
			{#if cancelDone}
				<p class="mt-3 text-sm" style="color: var(--success);">{cancelDone}</p>
			{/if}

			{#if data?.job?.warning_message}
				<div class="app-card mt-4 px-4 py-3 text-sm" style="color: var(--warning);">
					<strong>{tr('Peringatan pengiriman')}</strong> — {data.job.warning_message}
				</div>
			{/if}

			{#if pub.status === 'published' && isWordpress}
				<div class="mt-4 flex flex-wrap items-center gap-2">
					<button class="app-btn app-btn-soft" onclick={resyncWordpress} disabled={syncing}>
						<ImagePlus size={15} />{syncing ? tr('Mengantrekan…') : tr('Sinkron ulang gambar & kategori')}
					</button>
					<span class="text-xs text-muted-foreground">
						{tr('Kirim ulang gambar utama dan kategori ke post WordPress yang sudah terbit. Pakai ini kalau artikel tampil tanpa gambar utama atau masuk kategori “Uncategorized”. Post mitra diperbarui, bukan dibuat baru.')}
					</span>
				</div>
			{:else if pub.status === 'published' && data?.job?.remote_post_id}
				<div class="mt-4 flex flex-wrap items-center gap-2">
					<button class="app-btn app-btn-soft" onclick={repairImages} disabled={repairing}>
						<ImagePlus size={15} />{repairing ? tr('Mengantrekan…') : tr('Perbaiki gambar')}
					</button>
					<span class="text-xs text-muted-foreground">
						{tr('Unggah ulang gambar artikel ke media mitra dan pasang gambar utama pada post yang sudah terbit (post mitra diperbarui, bukan dibuat baru).')}
					</span>
				</div>
			{/if}
			{#if repairError}
				<p class="mt-3 text-sm" style="color: var(--destructive);">{repairError}</p>
			{/if}
			{#if repairDone}
				<p class="mt-3 text-sm" style="color: var(--success);">{repairDone}</p>
			{/if}
			{#if syncError}
				<p class="mt-3 text-sm" style="color: var(--destructive);">{syncError}</p>
			{/if}
			{#if syncDone}
				<p class="mt-3 text-sm" style="color: var(--success);">{syncDone}</p>
			{/if}

			<div class="app-card mt-4 p-4">
				<div class="grid gap-3 sm:grid-cols-3">
					<div>
						<p class="text-xs text-muted-foreground">{tr("Media")}</p>
						<p class="text-sm font-medium">{pub.media_name ?? '—'}</p>
						<p class="text-xs text-muted-foreground">
							{pub.media_integration_type ?? '—'}{pub.adapter_url ? ` · ${pub.adapter_url}` : ''}
						</p>
					</div>
					<div>
						<p class="text-xs text-muted-foreground">{tr("Brand / Klien")}</p>
						<p class="text-sm font-medium">{pub.brand_name ?? '—'}</p>
						{#if pub.brand_public_id}<p class="text-xs text-muted-foreground">{pub.brand_public_id}</p>{/if}
					</div>
					<div>
						<p class="text-xs text-muted-foreground">{tr('Mitra')}</p>
						<p class="text-sm font-medium">{pub.partner_name ?? '—'}</p>
						{#if pub.partner_public_id}<p class="text-xs text-muted-foreground">{pub.partner_public_id}</p>{/if}
					</div>
					<div>
						<p class="mb-1 text-xs text-muted-foreground">Price breakdown</p>
						<div class="flex justify-between gap-3 text-xs">
							<span class="text-muted-foreground">{tr("Harga Mitra")}</span>
							<span>{idr(pub.partner_price ?? pub.client_price)}</span>
						</div>
						<div class="flex justify-between gap-3 text-xs">
							<span class="text-muted-foreground">Markup Tauthub</span>
							<span>{pub.markup_percent != null ? `${pub.markup_percent}%` : '—'}</span>
						</div>
						<div class="flex justify-between gap-3 text-xs">
							<span class="text-muted-foreground">Nilai Markup</span>
							<span>{idr(Math.max(0, pub.client_price - (pub.partner_price ?? pub.client_price)))}</span>
						</div>
						<div class="mt-1 flex justify-between gap-3 border-t pt-1 text-xs font-bold">
							<span>{tr("Harga Klien")}</span><span>{idr(pub.client_price)}</span>
						</div>
					</div>
					<div>
						<p class="text-xs text-muted-foreground">Mode terbit</p>
						<p class="text-sm font-medium">{pub.publish_status}</p>
					</div>
					<div>
						<p class="text-xs text-muted-foreground">{tr("Dibuat oleh")}</p>
						<p class="text-sm font-medium">{pub.created_by_email ?? '—'}</p>
					</div>
					{#if pub.scheduled_at}
						<div>
							<p class="text-xs text-muted-foreground">Terjadwal</p>
							<p class="text-sm font-medium">{fmtDateTime(pub.scheduled_at)}</p>
						</div>
					{/if}
					{#if pub.published_at}
						<div>
							<p class="text-xs text-muted-foreground">{tr("Terbit")}</p>
							<p class="text-sm font-medium">{fmtDateTime(pub.published_at)}</p>
						</div>
					{/if}
					<div>
						<p class="text-xs text-muted-foreground">Koneksi</p>
						<code class="text-xs">{pub.connection_id ?? '—'}</code>
					</div>
				</div>
				{#if pub.published_url}
					<div class="mt-3">
						<p class="text-xs text-muted-foreground">URL terbit</p>
						<a
							href={pub.published_url}
							target="_blank"
							rel="noreferrer"
							class="text-xs font-semibold"
							style="color: var(--primary);"
							>{pub.published_url} <ExternalLink size={13} class="inline" /></a
						>
					</div>
				{/if}
				{#if pub.rejection_reason}
					<p class="mt-3 rounded-lg px-3 py-2 text-xs" style="background-color: var(--destructive-soft, #fdecee); color: var(--destructive);"> {tr("Ditolak mitra:")} {pub.rejection_reason}
					</p>
				{/if}
				{#if pub.notes}
					<div class="mt-3">
						<p class="text-xs text-muted-foreground">Catatan</p>
						<p class="text-sm">{pub.notes}</p>
					</div>
				{/if}
				{#if pub.cancel_reason}
					<div class="mt-3">
						<p class="text-xs text-muted-foreground">Alasan batal</p>
						<p class="text-sm">{pub.cancel_reason}</p>
					</div>
				{/if}
			</div>

			<div class="app-card mt-4 p-4">
				<h2 class="mb-2 text-base font-semibold">{tr("Riwayat attempts")}</h2>
				{#if data!.attempts.length === 0}
					<p class="text-sm text-muted-foreground">{tr("Belum ada percobaan pengiriman tercatat.")}</p>
				{/if}
				{#each data!.attempts as attempt (attempt.attempt_number)}
					<div class="flex items-start justify-between gap-3 border-b py-2 last:border-0">
						<div>
							<p class="text-sm font-medium">
								#{attempt.attempt_number} {ATTEMPT_LABEL[attempt.status] ?? attempt.status}
							</p>
							{#if attempt.error_code}<Badge tone="danger" value={attempt.error_code} />{/if}
							{#if attempt.error_message}
								<p class="mt-1 text-xs" style="color: var(--destructive);">{attempt.error_message}</p>
							{/if}
						</div>
						<span class="shrink-0 text-xs text-muted-foreground">
							{attempt.completed_at
								? fmtDateTime(attempt.completed_at)
								: attempt.started_at
									? fmtDateTime(attempt.started_at)
									: '—'}
						</span>
					</div>
				{/each}
			</div>

			{#if data!.publication.status === 'published' && data!.pr_analysis && (data!.pr_analysis.message || data!.pr_analysis.sentiment || data!.pr_analysis.verification)}
				<div class="app-card mt-4 p-4">
					<h2 class="mb-2 text-base font-semibold">Ringkasan Analisis PR</h2>
					<p class="mb-3 text-xs text-muted-foreground">{tr("Analisis otomatis atas artikel yang terbit (message consistency + tone + verifikasi URL) — bukan AVE, bukan jaminan dampak bisnis.")}</p>
					{#if data!.pr_analysis.message}
						<div class="mb-3">
							<div class="mb-1 flex items-center gap-2">
								<p class="text-sm font-medium">Message Consistency</p>
								<Badge
									tone={data!.pr_analysis.message.consistency_score != null &&
									data!.pr_analysis.message.consistency_score >= 70
										? 'success'
										: data!.pr_analysis.message.consistency_score != null
											? 'warning'
											: 'neutral'}
									label={data!.pr_analysis.message.consistency_score != null
										? `${data!.pr_analysis.message.consistency_score}/100`
										: '—'}
								/>
								{#if data!.pr_analysis.message.status === 'failed'}<Badge tone="danger" label="gagal" />{/if}
							</div>
							{#if data!.pr_analysis.message.error_message}
								<p class="mb-1 text-xs" style="color: var(--destructive);">
									{data!.pr_analysis.message.error_message}
								</p>
							{/if}
							{#each data!.pr_analysis.message.results as r, index (index)}
								<div class="flex items-start gap-2 border-b py-1 last:border-0">
									<div class="min-w-0 flex-1">
										<p class="text-sm">{r.message}</p>
										{#if r.evidence}<p class="text-xs text-muted-foreground">{r.evidence}</p>{/if}
									</div>
									<Badge tone={r.found ? 'success' : 'danger'} label={r.found ? '✓' : '✗'} />
								</div>
							{/each}
						</div>
					{/if}
					{#if data!.pr_analysis.sentiment}
						<div class="mb-3">
							<p class="text-sm font-medium">Sentiment / Tone</p>
							<div class="mt-1 flex items-center gap-2">
								<Badge
									tone={data!.pr_analysis.sentiment.sentiment === 'positive'
										? 'success'
										: data!.pr_analysis.sentiment.sentiment === 'negative'
											? 'danger'
											: 'neutral'}
									label={SENTIMENT_LABEL[data!.pr_analysis.sentiment.sentiment] ??
										data!.pr_analysis.sentiment.sentiment}
								/>
								{#if data!.pr_analysis.sentiment.confidence != null}
									<span class="text-xs text-muted-foreground">
										confidence {Math.round(data!.pr_analysis.sentiment.confidence * 100)}%
									</span>
								{/if}
								{#if data!.pr_analysis.sentiment.error_message}
									<span class="text-xs" style="color: var(--destructive);">
										{data!.pr_analysis.sentiment.error_message}
									</span>
								{/if}
							</div>
						</div>
					{/if}
					{#if data!.pr_analysis.verification}
						<div>
							<div class="mb-1 flex items-center gap-2">
								<p class="text-sm font-medium">Verifikasi URL</p>
								<Badge
									tone={data!.pr_analysis.verification.status === 'passed'
										? 'success'
										: data!.pr_analysis.verification.status === 'failed'
											? 'danger'
											: 'warning'}
									label={`${VERIFICATION_LABEL[data!.pr_analysis.verification.status] ??
										data!.pr_analysis.verification.status}${data!.pr_analysis.verification.compliance_score != null
										? ` · ${data!.pr_analysis.verification.compliance_score}/100`
										: ''}`}
								/>
							</div>
							{#if data!.pr_analysis.verification.error_message}
								<p class="mb-1 text-xs" style="color: var(--warning);">
									{data!.pr_analysis.verification.error_message}
								</p>
							{/if}
							{#each data!.pr_analysis.verification.checks as c (c.key)}
								<div class="flex items-start gap-2 border-b py-1 last:border-0">
									<div class="min-w-0 flex-1">
										<p class="text-sm">{c.label}</p>
										{#if c.detail}<p class="text-xs text-muted-foreground">{c.detail}</p>{/if}
									</div>
									<span class="text-xs" style="color: {c.ok ? 'var(--success)' : 'var(--destructive)'};">
										{c.ok ? '✓' : '✗'}
									</span>
								</div>
							{/each}
						</div>
					{/if}
				</div>
			{/if}

			{#if data!.replaces || data!.replaced_by}
				<div class="app-card mt-4 p-4">
					<h2 class="mb-2 text-base font-semibold">{tr("Riwayat auto-replacement")}</h2>
					{#if data!.replaces}
						<div class="flex items-start justify-between gap-3 border-b py-2">
							<div>
								<p class="text-sm font-medium">{tr("Pengganti dari:")} {data!.replaces.media_name ?? '—'}</p>
								<p class="mt-1 text-xs text-muted-foreground">{tr("Publikasi ini dibuat otomatis setelah publikasi di media tersebut ditolak (status asli:")}<Badge tone={STATUS_TONE[data!.replaces.status] ?? 'neutral'} value={data!.replaces.status} />).
								</p>
								{#if data!.replaces.brand_public_id}
									<a
										href={`/klien/${data!.replaces.brand_public_id}/distribution`}
										target="_blank"
										rel="noreferrer"
										class="text-xs font-semibold"
										style="color: var(--primary);"
										>{tr("Buka di dashboard klien")}<ExternalLink size={13} class="inline" /></a
									>
								{/if}
							</div>
							<span class="shrink-0 text-xs text-muted-foreground">{fmtDateTime(data!.replaces.created_at)}</span>
						</div>
					{/if}
					{#if data!.replaced_by}
						<div class="flex items-start justify-between gap-3 border-b py-2 last:border-0">
							<div>
								<p class="text-sm font-medium">{tr("Digantikan oleh:")} {data!.replaced_by.media_name ?? '—'}</p>
								<p class="mt-1 text-xs text-muted-foreground">
									Setelah ditolak, artikel otomatis dialihkan ke media tersebut (status terbaru:
									<Badge tone={STATUS_TONE[data!.replaced_by.status] ?? 'neutral'} value={data!.replaced_by.status} />
									{data!.replaced_by.published_at ? ` · terbit ${fmtDateTime(data!.replaced_by.published_at)}` : ''}).
								</p>
								{#if data!.replaced_by.published_url}
									<p class="mt-1 text-xs">
										<a
											href={data!.replaced_by.published_url}
											target="_blank"
											rel="noreferrer"
											style="color: var(--primary);">{data!.replaced_by.published_url}
											<ExternalLink size={13} class="inline" /></a
										>
									</p>
								{/if}
								{#if data!.replaced_by.brand_public_id}
									<a
										href={`/klien/${data!.replaced_by.brand_public_id}/distribution`}
										target="_blank"
										rel="noreferrer"
										class="text-xs font-semibold"
										style="color: var(--primary);"
										>{tr("Buka di dashboard klien")}<ExternalLink size={13} class="inline" /></a
									>
								{/if}
							</div>
							<span class="shrink-0 text-xs text-muted-foreground">{fmtDateTime(data!.replaced_by.created_at)}</span>
						</div>
					{/if}
				</div>
			{/if}

			{#if data!.variant}
				<div class="app-card mt-4 p-4">
					<h2 class="mb-2 text-base font-semibold">Variant Tauthub (multi-artikel)</h2>
					<div class="flex flex-wrap items-center gap-3">
						<div>
							<p class="text-xs text-muted-foreground">Sudut pandang</p>
							<p class="text-sm font-medium">
								{data!.variant.angle_label} <Badge tone="accent" value={data!.variant.angle_id} />
							</p>
						</div>
						<div>
							<p class="text-xs text-muted-foreground">Mode</p>
							<p class="text-sm font-medium">
								{data!.variant.mode === 'generate' ? tr('Dibuatkan dari bahan') : 'Parafrase'} · seed {data!.variant.seed}
							</p>
						</div>
						<div>
							<p class="text-xs text-muted-foreground">{tr("Status variant")}</p>
							<p class="text-sm font-medium">{data!.variant.variant_status}</p>
						</div>
						<div>
							<p class="text-xs text-muted-foreground">Batch</p>
							<code class="text-xs">{data!.variant.batch_id}</code>
						</div>
					</div>
					<p class="mt-2 text-xs text-muted-foreground">{tr("Artikel ini dihasilkan dari satu sumber bersama artikel lain, masing-masing dengan sudut pandang berbeda (diacak per batch).")}</p>
				</div>
			{/if}

			<div class="app-card mt-4 p-4">
				<h2 class="mb-2 text-base font-semibold">{tr("Riwayat callback adaptor")}</h2>
				{#if data!.callbacks.length === 0}
					<p class="text-sm text-muted-foreground">{tr("Belum ada callback dari adaptor mitra untuk publikasi ini.")}</p>
				{/if}
				{#each data!.callbacks as callback, index (index)}
					<div
						class="flex items-start justify-between gap-3 py-2"
						style={index < data!.callbacks.length - 1 ? 'border-bottom: 1px solid var(--border);' : ''}
					>
						<div class="min-w-0">
							<div class="mb-1 flex items-center gap-2">
								<Badge
									tone={callback.accepted ? 'success' : callback.status_code === 200 ? 'success' : 'danger'}
									label={`HTTP ${callback.status_code ?? '—'}`}
								/>
								<Badge tone={callback.accepted ? 'success' : 'danger'} label={callback.accepted ? 'accepted' : 'ditolak'} />
							</div>
							{#if callback.error_message}
								<p class="mb-1 text-xs" style="color: var(--destructive);">{callback.error_message}</p>
							{/if}
							<p class="break-words text-xs text-muted-foreground">
								{callback.ip_address ?? '—'}{callback.user_agent ? ` · ${callback.user_agent}` : ''}
							</p>
						</div>
						<span class="shrink-0 text-xs text-muted-foreground">{fmtDateTime(callback.created_at)}</span>
					</div>
				{/each}
			</div>

			{#if data!.job}
				<div class="app-card mt-4 p-4">
					<h2 class="mb-2 text-base font-semibold">Job delivery</h2>
					<div class="mb-3 grid gap-3 sm:grid-cols-3">
						<div>
							<p class="text-xs text-muted-foreground">{tr("Status job")}</p>
							<p class="text-sm font-medium">{data!.job.status}</p>
						</div>
						<div>
							<p class="text-xs text-muted-foreground">Attempts</p>
							<p class="text-sm font-medium">{data!.job.attempts}</p>
						</div>
						<div>
							<p class="text-xs text-muted-foreground">Remote ID</p>
							<code class="text-xs">{data!.job.remote_post_id ?? '—'}</code>
						</div>
					</div>
					{#if data!.job.error_message}
						<p class="rounded-lg px-3 py-2 text-xs" style="background-color: var(--destructive-soft, #fdecee); color: var(--destructive);">
							{data!.job.error_code ? `[${data!.job.error_code}] ` : ''}{data!.job.error_message}
						</p>
					{/if}
					{#if data!.job.payload_snapshot}
						<div class="mt-3">
							<p class="mb-1 text-xs text-muted-foreground">Payload snapshot</p>
							<pre
								class="max-h-[300px] overflow-auto rounded-lg p-3 text-xs"
								style="background-color: var(--secondary);">{JSON.stringify(data!.job.payload_snapshot, null, 2)}</pre>
						</div>
					{/if}
				</div>
			{/if}

			<div class="app-card mt-4 p-4">
				<h2 class="mb-2 text-base font-semibold">Kontekstual</h2>
				<p class="text-xs text-muted-foreground">Dokumen</p>
				<code class="text-xs">{pub.document_id} · v{pub.current_version} · {pub.document_slug ?? '—'}</code>
				<div class="mt-3 flex flex-wrap gap-3">
					{#if pub.brand_public_id}
						<a
							href={`/klien/${pub.brand_public_id}/distribution`}
							target="_blank"
							rel="noreferrer"
							class="text-xs font-semibold"
							style="color: var(--primary);">{tr("Buka di dashboard klien")}<ExternalLink size={13} class="inline" /></a
						>
					{/if}
					{#if pub.partner_public_id}
						<a
							href={`/mitra/${pub.partner_public_id}/publications`}
							target="_blank"
							rel="noreferrer"
							class="text-xs font-semibold"
							style="color: var(--primary);">{tr("Buka di portal mitra")}<ExternalLink size={13} class="inline" /></a
						>
					{/if}
				</div>
			</div>
		{/if}
	{/if}
	</ExplorerFrame>
</section>

{#if cancelOpen}
	<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);">
		<button
			type="button"
			aria-label={tr('Tutup')}
			tabindex="-1"
			style="position:absolute; inset:0; border:none; background:transparent; cursor:default;"
			onclick={() => {
				if (!cancelBusy) cancelOpen = false;
			}}
		></button>
		<dialog
			open
			class="app-card relative z-10 w-full max-w-md p-5"
			aria-label={tr('Batalkan publikasi')}
			onkeydown={(e) => {
				if (e.key === 'Escape' && !cancelBusy) cancelOpen = false;
			}}
		>
			<h2 class="text-base font-bold">{tr('Batalkan publikasi')}</h2>
			<p class="mt-2 text-sm text-muted-foreground">
				{tr('Publikasi ini akan dibatalkan dan kredit reservasinya dikembalikan ke Saldo Akun klien. Klien dan mitra menerima notifikasi berisi alasan di bawah.')}
			</p>
			<label class="mt-3 flex flex-col gap-1 text-sm">
				{tr('Alasan (opsional)')}
				<textarea
					class="app-input"
					rows="3"
					bind:value={cancelReason}
					maxlength={500}
					placeholder={tr('Permintaan klien, duplikat pesanan, atau kendala operasional.')}
				></textarea>
			</label>
			{#if cancelError}
				<p class="mt-2 text-sm" style="color: var(--destructive);">{cancelError}</p>
			{/if}
			<div class="mt-4 flex justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" disabled={cancelBusy} onclick={() => (cancelOpen = false)}>
					{tr('Batal')}
				</button>
				<button
					class="app-btn"
					style="background-color: var(--destructive); color: #fff;"
					type="button"
					disabled={cancelBusy}
					onclick={() => void cancelPublication()}
				>
					{cancelBusy ? tr('Membatalkan…') : tr('Batalkan publikasi')}
				</button>
			</div>
		</dialog>
	</div>
{/if}
