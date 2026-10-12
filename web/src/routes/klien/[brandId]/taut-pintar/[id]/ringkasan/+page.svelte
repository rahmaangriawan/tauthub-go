<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Ringkasan pasca-Gatekeeper — laporan distribusi kampanye Taut Pintar.
	// Port ringkasan/{client,revision-dialog}.tsx.
	//
	// Catatan port: app lama membuka dialog revisi dengan rich editor
	// (ContentEditor) + unggah screenshot. Di sini editor dirender sebagai
	// textarea HTML sederhana memakai endpoint dokumen yang sama — perilaku
	// endpoint identik, hanya kontrol editnya lebih ringkas.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { CircleCheck, ClipboardList, Clock, Database, Ellipsis, ExternalLink, Eye, Loader, LoaderCircle, Megaphone, Paperclip, RefreshCw, RotateCcw } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import AgentSteps from '../../_agent-steps.svelte';
	import EditorModal from '../writer/_editor-modal.svelte';
	import { MANUAL_PROFILER_REASON, rpCurrency, toJakartaDisplay } from '../../_shared';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);

	type CheckStatus = 'pending' | 'running' | 'passed' | 'failed' | 'review_required';
	type MediaItem = {
		media_id: string;
		name: string;
		media_type: string | null;
		featured_image_url: string | null;
		price_idr: number;
		da: number | null;
		ss: number | null;
		est_traffic: number | null;
		link_capacity: number | null;
		scheduled_at: string | null;
		article: { title: string; updated_at: string | null; links: { count: number; rels: string[] } };
		audit: { status: CheckStatus };
		publish: { status: string | null; url: string | null };
		publication_id: string | null;
		document_id: string | null;
	};
	type AuditsData = {
		campaign: { id: string; name?: string; taut_status: string | null; entry_mode?: 'ai' | 'manual' | null };
		media: MediaItem[];
		totalCostIdr: number;
		totalCostTc: number;
		tokens?: { used: number };
	};

	const DONE_STATUSES = new Set(['submitted', 'scheduled', 'distributed']);

	let data = $state<AuditsData | null>(null);
	let campaignName = $state('');
	let error = $state('');
	let manualFlow = $state(false);
	let loading = $state(true);
	let actionNotice = $state('');

		// Dialog revisi & editor naskah.
		type EditorMode = 'view' | 'edit_revision' | 'request_revision';
		let editorMode = $state<EditorMode>('view');

		// Menu aksi titik tiga per baris media
		let activeMenuId = $state<string | null>(null);
		const activeMenuMedia = $derived(data?.media.find((m) => m.media_id === activeMenuId) ?? null);
		let menuRight = $state(0);
		let menuTop = $state(0);
		let menuBottom = $state<number | null>(null);
		const menuPositionStyle = $derived(
			menuBottom != null
				? `right: ${menuRight}px; bottom: ${menuBottom}px;`
				: `right: ${menuRight}px; top: ${menuTop}px;`
		);

		function toggleRowMenu(id: string, trigger: HTMLElement) {
			if (activeMenuId === id) {
				activeMenuId = null;
				return;
			}
			const rect = trigger.getBoundingClientRect();
			menuRight = Math.max(8, window.innerWidth - rect.right);
			const spaceBelow = window.innerHeight - rect.bottom;
			const spaceAbove = rect.top;
			if (spaceBelow < 110 && spaceAbove > spaceBelow) {
				menuBottom = Math.max(8, window.innerHeight - rect.top + 4);
				menuTop = 0;
			} else {
				menuBottom = null;
				menuTop = rect.bottom + 4;
			}
			activeMenuId = id;
		}

		function closeRowMenu() {
			activeMenuId = null;
		}

		let revisionTarget = $state<{ publication_id: string | null; document_id: string | null; name: string; featured_image_url: string | null } | null>(null);
		let revTitle = $state('');
		let revHtml = $state('');
		let revCover = $state<string | null>(null);
		let revComment = $state('');
		let revAttachment = $state<{ url: string; name: string } | null>(null);
		let revLoading = $state(false);
		let revUploading = $state(false);
		let revSaving = $state(false);
		let revError = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await loadData();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function loadData() {
		loading = true;
		error = '';
		try {
			const res = await apiRequest(`${apiBase}/gatekeeper/audits`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat ringkasan distribusi.')));
			const audits: AuditsData = await res.json();
			data = audits;
			manualFlow = audits.campaign.entry_mode === 'manual';
			campaignName = audits.campaign?.name ?? '';
			if (!campaignName) {
				const draftRes = await apiRequest(`${apiBase}/profile/draft`).catch(() => null);
				if (draftRes?.ok) {
					const d = await draftRes.json().catch(() => null);
					campaignName = d?.campaign?.name ?? '';
				}
			}
		} catch {
			error = tr('Gagal memuat ringkasan distribusi.');
		} finally {
			loading = false;
		}
	}

	/* ─── Dialog revisi & pratinjau editor ─── */

	async function openArticleModal(m: MediaItem, mode: EditorMode) {
		editorMode = mode;
		revisionTarget = {
			publication_id: m.publication_id,
			document_id: m.document_id,
			name: m.name,
			featured_image_url: m.featured_image_url
		};
		revComment = '';
		revAttachment = null;
		revError = '';
		revHtml = '';
		revTitle = m.article.title;
		revCover = m.featured_image_url;
		if (!m.document_id) return;
		revLoading = true;
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/documents/${m.document_id}/html`);
			if (!res.ok) throw new Error(tr('Gagal memuat isi artikel.'));
			const d = await res.json();
			revHtml = d.content ?? '';
			if (!revTitle && typeof d.title === 'string') revTitle = d.title;
		} catch (reason) {
			revError = reason instanceof Error ? reason.message : tr('Gagal memuat isi artikel.');
		} finally {
			revLoading = false;
		}
	}

	async function saveDocumentOnly() {
		const target = revisionTarget;
		if (!target?.document_id || revSaving) return;
		revSaving = true;
		revError = '';
		try {
			const saveRes = await apiRequest(`/v1/brands/${brandId}/documents/${target.document_id}`, {
				method: 'PUT',
				body: JSON.stringify({
					title: revTitle,
					content: revHtml,
					featured_image_url: revCover ?? undefined
				})
			});
			if (!saveRes.ok) throw new Error(await readApiError(saveRes, tr('Gagal menyimpan perubahan artikel.')));
			revisionTarget = null;
			actionNotice = tr('Perubahan artikel berhasil disimpan.');
			await loadData();
		} catch (reason) {
			revError = reason instanceof Error ? reason.message : tr('Gagal menyimpan perubahan artikel.');
		} finally {
			revSaving = false;
		}
	}

	async function uploadScreenshot(file: File) {
		revUploading = true;
		revError = '';
		try {
			const form = new FormData();
			form.append('file', file);
			const res = await apiRequest(`/v1/brands/${brandId}/assets`, { method: 'POST', body: form });
			const d = await res.json().catch(() => null);
			if (!res.ok || !d?.url) throw new Error(d?.detail ?? tr('Gagal mengunggah screenshot.'));
			revAttachment = { url: d.url, name: file.name };
		} catch (reason) {
			revError = reason instanceof Error ? reason.message : tr('Gagal mengunggah screenshot.');
		} finally {
			revUploading = false;
		}
	}

	// Unggah cover baru dari editor revisi; editor memakai nilai kembaliannya.
	async function uploadRevisionCover(file: File): Promise<string> {
		const form = new FormData();
		form.append('file', file);
		const res = await apiRequest(`/v1/brands/${brandId}/assets`, { method: 'POST', body: form });
		const d = await res.json().catch(() => null);
		if (!res.ok || !d?.url) throw new Error(d?.detail ?? tr('Gagal mengunggah cover.'));
		return d.url as string;
	}

	async function submitRevision() {
		const target = revisionTarget;
		if (!target?.publication_id || revSaving) return;
		const trimmed = revComment.trim();
		if (trimmed.length < 5) {
			revError = tr('Tuliskan komentar minimal 5 karakter supaya blogger tahu apa yang perlu diubah.');
			return;
		}
		revSaving = true;
		revError = '';
		try {
			// 1) Simpan hasil edit sebagai versi artikel baru (endpoint dokumen).
			if (target.document_id) {
				const saveRes = await apiRequest(`/v1/brands/${brandId}/documents/${target.document_id}`, {
					method: 'PUT',
					body: JSON.stringify({
						content: revHtml,
						title: revTitle,
						content_format: 'html',
						change_note: `Revisi klien: ${trimmed}`.slice(0, 255)
					})
				});
				if (!saveRes.ok) throw new Error(await readApiError(saveRes, tr('Gagal menyimpan revisi artikel.')));
			}
			// 2) Catat komentar + lampiran, lalu teruskan ke blogger.
			const res = await apiRequest(`/v1/brands/${brandId}/publications/${target.publication_id}/request-revision`, {
				method: 'POST',
				body: JSON.stringify({
					comment: trimmed,
					attachmentUrl: revAttachment?.url ?? null,
					attachmentName: revAttachment?.name ?? null,
					featuredImageUrl: revCover ?? null
				})
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal mengirim revisi.')));
			actionNotice = 'Revisi terkirim ke pemilik domain.';
			revisionTarget = null;
			await loadData();
		} catch (reason) {
			revError = reason instanceof Error ? reason.message : tr('Gagal mengirim revisi.');
		} finally {
			revSaving = false;
		}
	}

	/* ─── Turunan tampilan ─── */

	const mediaList = $derived(data?.media ?? []);
	const done = $derived(DONE_STATUSES.has(data?.campaign.taut_status ?? ''));
	const publishedCount = $derived(mediaList.filter((m) => m.publish.status === 'published').length);
	const allPublished = $derived(mediaList.length > 0 && publishedCount === mediaList.length);
	const bannerState = $derived(!done ? 'idle' : allPublished ? 'published' : 'waiting');
	const waitingCount = $derived(mediaList.length - publishedCount);
	const progressPercent = $derived(mediaList.length > 0 ? Math.round((publishedCount / mediaList.length) * 100) : 0);
	const statusSub = $derived(
		!done
			? tr('Belum didistribusikan. Terbitkan dari halaman The Writer.')
			: waitingCount > 0
				? `${waitingCount} media menunggu persetujuan mitra`
				: 'Semua media terbit'
	);
	const resume = $derived.by(() => {
		const s = data?.campaign.taut_status ?? '';
		switch (s) {
			case 'draft':
			case 'profiling':
				return { slug: 'profiler', label: 'Lengkapi The Profiler' };
			case 'profiled':
				return { slug: 'matchmaker', label: 'Lanjut ke The Matchmaker' };
			case 'placement':
				return { slug: 'writer', label: 'Lanjut ke The Writer' };
			case 'submitted':
			case 'scheduled':
				return { slug: 'writer', label: tr('Kembali ke The Writer') };
			default:
				return null;
		}
	});

	const initialOf = (name: string | null | undefined) => (name ?? '').trim().charAt(0).toUpperCase() || '?';
	const avatarTone = (name: string | null | undefined) => {
		const s = (name ?? '').trim().toLowerCase();
		let h = 0;
		for (let i = 0; i < s.length; i += 1) h = (h * 31 + s.charCodeAt(i)) % 6;
		return h;
	};
	const AVA = ['var(--accent)', 'var(--success-soft)', 'var(--warning-soft)', 'var(--muted)', 'var(--accent)', 'var(--success-soft)'];
</script>

<PageTitle title="Taut Pintar — Ringkasan Distribusi" />

<main>
	<AgentSteps current={3} tone={bannerState === 'published' ? 'published' : bannerState === 'waiting' ? 'waiting' : undefined} extra="ringkasan" />

	{#if loading}
		<div class="app-card mt-4 flex items-center justify-between gap-3 p-4">
			<span class="flex items-center gap-2 text-sm font-bold"><LoaderCircle size={18} class="animate-spin" />{tr("Memuat ringkasan…")}</span>
			<button class="app-btn app-btn-soft" type="button" onclick={loadData}><RotateCcw size={14} /> Segarkan</button>
		</div>
		<p class="mt-2 text-sm text-muted-foreground">{tr("Mengambil status distribusi terbaru dari server.")}</p>
	{:else if !data}
		<div class="app-card mt-4 p-4">
			<p class="font-bold">{tr("Ringkasan Tidak Tersedia")}</p>
			<p class="mt-1 text-sm text-muted-foreground">{error || 'Data kampanye belum tersedia.'}</p>
		</div>
	{:else}
		<div
			class="app-card mt-4 p-4"
			style="background-color: {bannerState === 'published' ? 'var(--success-soft)' : 'var(--accent)'}; color: {bannerState === 'published' ? 'var(--success)' : 'var(--accent-foreground)'};"
		>
				<div class="flex items-center gap-2">
					{#if bannerState === 'published'}
						<CircleCheck size={18} class="shrink-0" />
					{:else}
						<span class="bannerLoader inline-flex shrink-0 items-center justify-center" aria-hidden="true">
							<Loader size={18} />
						</span>
					{/if}
						<p class="font-bold">
							{#if bannerState === 'published'}
								{tr('Distribusi Selesai — Semua Artikel Terbit')}
							{:else if bannerState === 'waiting'}
								{`Distribusi Berhasil — Menunggu ACC Mitra (${publishedCount}/${mediaList.length} Terbit)`}
							{:else if campaignName}
								<span>{tr('Kampanye')} <span class="underline underline-offset-4 decoration-current/70">{campaignName}</span> {tr('Belum Terdistribusi')}</span>
							{:else}
								{tr('Kampanye Belum Terdistribusi')}
							{/if}
						</p>
					</div>
				<p class="mt-1 text-sm" style="opacity: .85;">
					{#if bannerState === 'published'}
						{tr('Semua artikel sudah terbit di media masing-masing. Bukti tayang ada di kolom Status Terbit.')}
					{:else if bannerState === 'waiting'}
						{tr('Kampanye sudah disubmit. Sebagian artikel masih menunggu persetujuan mitra sebelum terbit.')}
					{:else}
						{tr('Terbitkan kampanye dari halaman The Writer untuk mendistribusikan artikel ke media terpilih dan melihat laporan distribusi.')}
					{/if}
				</p>
		</div>

		{#if resume}
			<div class="mt-3 flex justify-end">
				<a class="app-btn app-btn-primary" href={`/klien/${brandUrl}/taut-pintar/${campaignId}/${resume.slug}`}>
					<RotateCcw size={14} /> {resume.label}
				</a>
			</div>
		{/if}

		<div class="mt-3 grid gap-3 md:grid-cols-2">
			<div class="app-card p-4">
				<div class="flex items-center gap-3">
					<span class="flex h-9 w-9 items-center justify-center rounded-lg" style="background-color: var(--accent); color: var(--accent-foreground);"><Database size={20} /></span>
					<p class="text-sm text-muted-foreground">{tr("Biaya Distribusi Media")}</p>
				</div>
				<p class="mt-2 text-lg font-bold">{rpCurrency(data.totalCostIdr)}</p>
			</div>
			<div class="app-card p-4">
				<div class="flex items-center gap-3">
					<span class="flex h-9 w-9 items-center justify-center rounded-lg" style="background-color: var(--warning-soft); color: var(--warning);"><ClipboardList size={20} /></span>
					<p class="text-sm text-muted-foreground">{tr("Status Kampanye")}</p>
				</div>
				<p class="mt-2 text-lg font-bold">{publishedCount} / {mediaList.length} {tr("Terbit")}</p>
				<div class="mt-2 h-2 w-full overflow-hidden rounded-full" style="background-color: var(--muted);" role="progressbar" aria-valuenow={progressPercent} aria-valuemin="0" aria-valuemax="100">
					<div class="h-full" style="width: {progressPercent}%; background-color: var(--primary);"></div>
				</div>
				<p class="mt-1 text-xs text-muted-foreground">{statusSub}</p>
			</div>
		</div>

		{#if error}<p class="mt-2 text-sm" style="color: var(--destructive);">{error}</p>{/if}

		<section class="app-card mt-3 p-4">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<div class="flex items-center gap-2">
					<span class="flex h-8 w-8 items-center justify-center rounded-lg" style="background-color: var(--accent); color: var(--accent-foreground);"><Megaphone size={18} /></span>
					<div>
						<p class="font-bold">{tr("Media Terpilih (")}{mediaList.length})</p>
						<p class="text-xs text-muted-foreground">{tr("Daftar media untuk distribusi konten kampanye ini.")}</p>
					</div>
				</div>
				<div class="flex gap-2">
					<a class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" href={`/klien/${brandUrl}/taut-pintar`}>{tr("Kembali ke Daftar Kampanye")}</a>
					<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={loadData}><RefreshCw size={14} /> Segarkan</button>
				</div>
			</div>

			{#if mediaList.length > 0}
				<div class="mt-3 overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="text-left text-[11px] uppercase tracking-wide text-muted-foreground">
								<th class="px-3 py-2 font-semibold">{tr("Media")}</th>
								<th class="px-3 py-2 font-semibold">Tipe</th>
								<th class="px-3 py-2 font-semibold">DA / SS</th>
								<th class="px-3 py-2 font-semibold">Link Artikel</th>
								<th class="px-3 py-2 font-semibold">{tr("Jadwal Terbit")}</th>
								<th class="px-3 py-2 font-semibold">Judul Artikel</th>
								<th class="px-3 py-2 font-semibold">{tr("Status Terbit")}</th>
								<th class="px-3 py-2 font-semibold">Aksi</th>
							</tr>
						</thead>
						<tbody>
							{#each mediaList as m (m.media_id)}
								<tr class="border-t">
									<td class="px-3 py-2">
										<div class="flex items-center gap-2">
											<span class="flex h-7 w-7 items-center justify-center rounded-full text-xs font-bold" style="background-color: {AVA[avatarTone(m.name)]};">{initialOf(m.name)}</span>
											<span class="font-medium">{m.name || '—'}</span>
										</div>
									</td>
									<td class="px-3 py-2">
										{#if m.media_type}<span class="rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--accent); color: var(--accent-foreground);">{m.media_type}</span>{:else}—{/if}
									</td>
									<td class="px-3 py-2">{m.da != null ? m.da : '—'}{m.ss != null ? ` / ${m.ss}%` : ''}</td>
									<td class="px-3 py-2">
										{#if m.article.links.count > 0}
											<span class="text-xs">{m.article.links.count} link{m.article.links.rels.length > 0 ? ` · ${m.article.links.rels.join(', ')}` : ' · tanpa rel'}</span>
										{:else}
											<span class="text-xs text-muted-foreground">{tr("Tidak ada link")}</span>
										{/if}
									</td>
									<td class="px-3 py-2">
										<span class="inline-flex items-center gap-1 text-xs" style="color: {m.publish.status === 'published' ? 'var(--success)' : 'var(--muted-foreground)'};">
											<Clock size={12} />{toJakartaDisplay(m.scheduled_at) ?? 'Segera'}
										</span>
									</td>
									<td class="max-w-[260px] px-3 py-2">
										{#if m.article.title}
											<span class="block truncate text-xs">{m.article.title}</span>
										{:else}
											<span class="text-xs text-muted-foreground">{tr("Belum ada artikel")}</span>
										{/if}
									</td>
									<td class="px-3 py-2">
										{#if m.publish.status === 'published'}
											<span class="inline-flex items-center gap-1.5">
												<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--success-soft); color: var(--success);">
													<CircleCheck size={11} />{tr("Terbit")}</span>
												{#if m.publish.url}
													<a
														href={m.publish.url}
														target="_blank"
														rel="noreferrer"
														class="inline-flex items-center gap-1 rounded-lg border px-1.5 py-0.5 text-[11px] font-medium hover:underline"
														style="border-color: var(--accent-foreground); color: var(--accent-foreground); background-color: var(--accent);"
														title={tr('Buka artikel di situs mitra')}
													>
														<ExternalLink size={11} />{tr("Lihat")}
													</a>
												{/if}
											</span>
										{:else if m.publish.status === 'failed'}
											<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--destructive); color: #fff;">{tr("Gagal")}</span>
										{:else if m.publish.status === 'processing' || m.publish.status === 'queued' || m.publish.status === 'retrying'}
											<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">Diproses</span>
										{:else if m.publish.status === 'revision'}
											<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);"><RotateCcw size={11} />{tr("Revisi - Menunggu Mitra")}</span>
										{:else if m.publish.status === 'waiting_partner' || m.publish.status === 'partner_approved' || m.publish.status === 'pending'}
											<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--warning-soft); color: var(--warning);"><Clock size={11} />{tr("Menunggu Mitra")}</span>
										{:else}
											<span class="text-xs text-muted-foreground">—</span>
										{/if}
									</td>
										<td class="px-3 py-2">
											<div class="relative flex items-center">
												<button
													type="button"
													class="inline-flex h-7 w-7 items-center justify-center rounded-lg border text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
													style="border-color: var(--border); background: var(--card, #fff);"
													aria-haspopup="menu"
													aria-expanded={activeMenuId === m.media_id}
													aria-label={`${tr('Aksi untuk')} ${m.name}`}
													onclick={(e) => {
														e.stopPropagation();
														toggleRowMenu(m.media_id, e.currentTarget as HTMLElement);
													}}
												>
													<Ellipsis size={15} />
												</button>
											</div>
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>

						{#if activeMenuMedia}
							{@const m = activeMenuMedia}
							{@const isRevisionRequested = m?.publish?.status === 'revision'}
							<button
								type="button"
								class="fixed inset-0 z-40 bg-transparent cursor-default"
								aria-label={tr("Tutup menu")}
								onclick={closeRowMenu}
							></button>
							<div
								class="fixed z-50 w-44 rounded-xl border py-1 shadow-lg"
								style="border-color: var(--border); background: var(--card, #fff); {menuPositionStyle}"
								role="menu"
								aria-label={tr('Menu aksi')}
							>
								<button
									type="button"
									role="menuitem"
									class="flex w-full items-center gap-2 px-3 py-2 text-left text-xs font-medium transition-colors hover:bg-secondary disabled:cursor-not-allowed disabled:opacity-50"
									disabled={!m.document_id}
									title={!m.document_id ? tr('Artikel belum tersedia') : undefined}
									onclick={() => {
										const targetMedia = m;
										closeRowMenu();
										void openArticleModal(targetMedia, isRevisionRequested ? 'edit_revision' : 'view');
									}}
								>
									<Eye size={13} class="shrink-0 text-muted-foreground" />
									<span>{isRevisionRequested ? tr('Edit Artikel') : tr('Lihat / Edit')}</span>
								</button>

								<button
									type="button"
									role="menuitem"
									class="flex w-full items-center gap-2 px-3 py-2 text-left text-xs font-medium transition-colors hover:bg-secondary disabled:cursor-not-allowed disabled:opacity-50"
									disabled={!m.publication_id || !m.document_id}
									title={!m.document_id ? tr('Artikel belum tersedia') : !m.publication_id ? tr('Publikasi belum diajukan') : undefined}
									onclick={() => {
										const targetMedia = m;
										closeRowMenu();
										void openArticleModal(targetMedia, 'request_revision');
									}}
								>
									<RotateCcw size={13} class="shrink-0 text-muted-foreground" />
									<span>{tr('Ajukan Revisi')}</span>
								</button>
							</div>
						{/if}

				{#if actionNotice}<p class="mt-3 text-sm" style="color: var(--success);">{actionNotice}</p>{/if}
			{:else}
				<p class="mt-3 text-sm text-muted-foreground">{tr("Belum ada media terpilih.")}</p>
			{/if}
		</section>
	{/if}
</main>

{#if revisionTarget}
	{#if revLoading}
		<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(15, 23, 42, 0.45);" role="dialog" aria-modal="true" aria-label={tr('Revisi Artikel')}>
			<div class="app-card flex items-center gap-2 p-5 text-sm text-muted-foreground">
				<LoaderCircle size={18} class="animate-spin" />{tr("Memuat artikel…")}
			</div>
		</div>
	{:else}
		<EditorModal
			mediaName={revisionTarget.name ?? 'media'}
			initialTitle={revTitle}
			initialHtml={revHtml}
			initialCoverUrl={revCover}
			linkLimit={9999}
			requireCover={false}
			enforceLinkLimit={false}
				saving={revSaving}
				saveError={revError || null}
				subtitle={editorMode === 'request_revision'
					? tr("Edit artikelnya langsung, jelaskan yang perlu diubah, lalu kirim ke pemilik domain. Revisi boleh berkali-kali.")
					: editorMode === 'edit_revision'
						? tr("Perbaiki artikel sesuai pengajuan revisi dari mitra lalu simpan.")
						: tr("Pratinjau naskah artikel (mode baca).")}
				saveLabel={editorMode === 'request_revision' ? tr("Kirim Revisi") : tr("Simpan Revisi")}
				showSaveButton={editorMode !== 'view'}
				readOnly={editorMode === 'view'}
				onClose={() => (revisionTarget = null)}
				onUploadCover={uploadRevisionCover}
				onSave={(title, html, cover) => {
					revTitle = title;
					revHtml = html;
					revCover = cover;
					if (editorMode === 'request_revision') {
						void submitRevision();
					} else if (editorMode === 'edit_revision') {
						void saveDocumentOnly();
					}
				}}
			>
				{#snippet extraFields()}
					{#if editorMode === 'request_revision'}
						<label class="block">
							<span class="text-xs font-medium text-muted-foreground">{tr("Komentar untuk pemilik domain")}</span>
							<textarea class="app-input mt-1 w-full" rows="3" placeholder={tr("mis. Paragraf kedua tolong ganti nama produk menjadi versi terbaru.")} bind:value={revComment}></textarea>
						</label>
						<div>
							<span class="text-xs font-medium text-muted-foreground">{tr("Screenshot (opsional)")}</span>
							<div class="mt-1 flex flex-wrap items-center gap-3">
								<input type="file" accept="image/png,image/jpeg,image/webp,image/gif" disabled={revUploading} onchange={(e) => { const f = (e.currentTarget as HTMLInputElement).files?.[0]; if (f) void uploadScreenshot(f); }} />
								{#if revUploading}<LoaderCircle size={14} class="animate-spin" />{/if}
								{#if revAttachment}
									<span class="flex items-center gap-2 text-xs text-muted-foreground">
										<Paperclip size={14} />{revAttachment.name}
										<button class="underline" type="button" onclick={() => (revAttachment = null)}>{tr("Hapus")}</button>
									</span>
								{/if}
							</div>
						</div>
					{/if}
				{/snippet}
			</EditorModal>
	{/if}
{/if}

<style>
	.bannerLoader {
		animation: spin 1.4s linear infinite;
		transform-origin: center;
		transition: animation-duration 0.2s ease;
	}
	.bannerLoader:hover {
		animation-duration: 0.7s;
	}
	@keyframes spin {
		from {
			transform: rotate(0deg);
		}
		to {
			transform: rotate(360deg);
		}
	}
</style>
