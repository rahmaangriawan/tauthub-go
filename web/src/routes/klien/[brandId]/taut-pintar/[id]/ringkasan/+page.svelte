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
	import { Check, CircleCheck, ClipboardList, Clock, Database, Ellipsis, ExternalLink, Eye, Link as LinkIcon, Loader, LoaderCircle, Paperclip, RefreshCw, RotateCcw } from '@lucide/svelte';
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
		website_url: string | null;
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

		let revisionTarget = $state<{ publication_id: string | null; document_id: string | null; media_id: string; name: string; featured_image_url: string | null } | null>(null);
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
			media_id: m.media_id,
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

	// Favicon media di tabel: domain dari website_url, fallback ke inisial nama.
	const cleanDomain = (url: string | null | undefined, name: string): string =>
		((url ?? '').replace(/^https?:\/\/(www\.)?/i, '').replace(/\/+$/, '').trim() || name.trim()).toLowerCase();
	let faviconFailed = $state<Record<string, boolean>>({});
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
			class="app-card mt-4 flex items-center justify-between gap-3 p-4"
			style="background-color: {bannerState === 'published' ? 'var(--success-soft)' : 'var(--accent)'}; color: {bannerState === 'published' ? 'var(--success)' : 'var(--accent-foreground)'};"
		>
			<div class="min-w-0">
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
				<a class="app-btn app-btn-primary shrink-0" href={`/klien/${brandUrl}/taut-pintar/${campaignId}/${resume.slug}`}>
					<RotateCcw size={14} /> {resume.label}
				</a>
			{/if}
		</div>

		<div class="mt-3 grid gap-3 md:grid-cols-2">
			<div
				class="app-card relative overflow-hidden p-4 sm:p-5"
				style="background: linear-gradient(115deg, #ffffff 0%, #f5f9ff 45%, #e4eefe 100%);"
			>
				<img
					src="/images/wallet-distribusi.webp"
					alt=""
					aria-hidden="true"
					class="pointer-events-none absolute right-0 bottom-0 h-[78%] w-auto translate-x-[10%] translate-y-[18%] object-contain opacity-25 select-none"
					draggable="false"
				/>
				<div class="relative z-10">
					<div class="flex items-center gap-3">
						<span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-blue-100 text-blue-600"><Database size={20} /></span>
						<div class="min-w-0">
							<p class="text-sm font-semibold text-slate-700">{tr("Biaya Distribusi Media")}</p>
							<p class="text-xs text-muted-foreground mt-0.5">{tr("Total biaya untuk distribusi ke seluruh media terpilih.")}</p>
						</div>
					</div>
					<p class="mt-4 text-3xl font-extrabold tracking-tight text-slate-900">{rpCurrency(data.totalCostIdr)}</p>
				</div>
			</div>
			<div
				class="app-card relative overflow-hidden p-4 sm:p-5"
				style="background: {allPublished
					? 'linear-gradient(115deg, #ffffff 0%, #f6fdf9 45%, #e6f9ee 100%)'
					: 'linear-gradient(115deg, #ffffff 0%, #fefaef 45%, #fcf1d6 100%)'};"
			>
				<div class="relative z-10">
					<div class="flex items-start justify-between gap-3">
						<div class="flex items-center gap-3">
							{#if allPublished}
								<span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl" style="background-color: #d9f4e4; color: #16a34a;"><Check size={20} strokeWidth={2.8} /></span>
							{:else}
								<span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl" style="background-color: var(--warning-soft); color: var(--warning);"><ClipboardList size={20} /></span>
							{/if}
							<div class="min-w-0">
								<p class="text-sm font-semibold text-slate-700">{tr("Status Kampanye")}</p>
								<p class="text-xs text-muted-foreground mt-0.5">{tr("Progres penerbitan artikel ke media terpilih.")}</p>
							</div>
						</div>
					</div>
					<p class="mt-4 text-3xl font-extrabold tracking-tight text-slate-900">{publishedCount} / {mediaList.length} {tr("Terbit")}</p>
					<div class="mt-3 flex items-center gap-3">
						<div
							class="h-2 flex-1 overflow-hidden rounded-full"
							style="background-color: {allPublished ? '#d9f4e4' : 'var(--muted)'};"
							role="progressbar"
							aria-valuenow={progressPercent}
							aria-valuemin="0"
							aria-valuemax="100"
						>
							<div class="h-full rounded-full transition-all" style="width: {progressPercent}%; background-color: {allPublished ? '#22c55e' : 'var(--primary)'};"></div>
						</div>
						<span class="text-xs font-bold" style="color: {allPublished ? '#16a34a' : 'var(--muted-foreground)'};">{progressPercent}%</span>
					</div>
					<p class="mt-2 flex items-center gap-1.5 text-xs font-medium" style="color: {allPublished ? '#16a34a' : 'var(--muted-foreground)'};">
						{#if allPublished}<CircleCheck size={13} />{/if}
						{statusSub}
					</p>
				</div>
			</div>
		</div>

		{#if error}<p class="mt-2 text-sm" style="color: var(--destructive);">{error}</p>{/if}

		<section class="app-card mt-3 p-4 sm:p-5">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div class="flex items-center gap-3">
					<span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-blue-50 text-blue-600 shadow-2xs">
						<svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 16 16" aria-hidden="true">
							<g fill="currentColor">
								<path d="M15.964.686a.5.5 0 0 0-.65-.65L.767 5.855a.75.75 0 0 0-.124 1.329l4.995 3.178l1.531 2.406a.5.5 0 0 0 .844-.536L6.637 10.07l7.494-7.494l-1.895 4.738a.5.5 0 1 0 .928.372zm-2.54 1.183L5.93 9.363L1.591 6.602z" />
								<path d="M16 12.5a3.5 3.5 0 1 1-7 0a3.5 3.5 0 0 1 7 0m-1.993-1.679a.5.5 0 0 0-.686.172l-1.17 1.95l-.547-.547a.5.5 0 0 0-.708.708l.774.773a.75.75 0 0 0 1.174-.144l1.335-2.226a.5.5 0 0 0-.172-.686" />
							</g>
						</svg>
					</span>
					<div>
						<p class="font-bold text-base text-slate-900">{tr("Media Terpilih (")}{mediaList.length})</p>
						<p class="text-xs text-slate-500 mt-0.5">{tr("Daftar media untuk distribusi konten kampanye ini.")}</p>
					</div>
				</div>
				<div class="flex items-center gap-2">
					<button
						class="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 shadow-2xs hover:bg-slate-50 transition-colors"
						type="button"
						onclick={loadData}
					>
						<RefreshCw size={13} class="text-blue-600" />
						Segarkan
					</button>
				</div>
			</div>

			{#if mediaList.length > 0}
				<div class="mt-4 overflow-x-auto rounded-lg border border-slate-100">
					<table class="w-full text-sm">
						<thead>
							<tr class="bg-slate-50/80 border-b border-slate-100 text-left text-[11px] font-bold uppercase tracking-wider text-slate-500">
								<th class="px-4 py-3">{tr("Media")}</th>
								<th class="px-4 py-3">Tipe</th>
								<th class="px-4 py-3">DA</th>
								<th class="px-4 py-3">Link Artikel</th>
								<th class="px-4 py-3">{tr("Jadwal Terbit")}</th>
								<th class="px-4 py-3">Judul Artikel</th>
								<th class="px-4 py-3">{tr("Status Terbit")}</th>
								<th class="px-4 py-3 text-right">Aksi</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-slate-100 bg-white">
							{#each mediaList as m (m.media_id)}
								{@const domain = cleanDomain(m.website_url, m.name)}
								<tr class="hover:bg-slate-50/50 transition-colors">
									<td class="px-4 py-3">
										<div class="flex items-center gap-3">
											<span class="flex h-9 w-9 shrink-0 items-center justify-center overflow-hidden rounded-full text-xs font-bold border border-slate-200/60" style="background-color: {AVA[avatarTone(m.name)]};">
												{#if domain && !faviconFailed[m.media_id]}
													<img
														src={`https://www.google.com/s2/favicons?domain=${encodeURIComponent(domain)}&sz=128`}
														alt=""
														class="h-full w-full object-cover"
														loading="lazy"
														decoding="async"
														onerror={() => (faviconFailed = { ...faviconFailed, [m.media_id]: true })}
													/>
												{:else}
													{initialOf(m.name)}
												{/if}
											</span>
											<div class="min-w-0">
												<span class="block font-bold text-slate-900 text-[13.5px] leading-tight truncate">{m.name || '—'}</span>
												{#if domain}
													<span class="block text-[11px] text-slate-400 font-normal mt-0.5 truncate">{domain}</span>
												{/if}
											</div>
										</div>
									</td>
									<td class="px-4 py-3 whitespace-nowrap">
										{#if m.media_type}
											<span class="rounded-full border border-blue-200/80 bg-blue-50 px-3 py-0.5 text-[11px] font-semibold text-blue-600">
												{m.media_type}
											</span>
										{:else}
											—
										{/if}
									</td>
									<td class="px-4 py-3 font-semibold text-slate-800">{m.da != null ? m.da : '—'}</td>
									<td class="px-4 py-3 whitespace-nowrap">
										{#if m.article.links.count > 0}
											<span class="inline-flex items-center gap-1.5 text-xs font-medium text-slate-700">
												<LinkIcon size={12} class="text-slate-400 shrink-0" />
												{m.article.links.count} link{m.article.links.rels.length > 0 ? ` · ${m.article.links.rels.join(', ')}` : ' dofollow'}
											</span>
										{:else}
											<span class="text-xs text-muted-foreground">{tr("Tidak ada link")}</span>
										{/if}
									</td>
									<td class="px-4 py-3 whitespace-nowrap">
										<span class="inline-flex items-center gap-1.5 text-xs font-medium text-emerald-600">
											<Clock size={12} class="text-emerald-500 shrink-0" />
											{toJakartaDisplay(m.scheduled_at) ?? 'Segera'}
										</span>
									</td>
									<td class="max-w-[280px] px-4 py-3">
										{#if m.article.title}
											<span class="block truncate text-xs text-slate-700" title={m.article.title}>{m.article.title}</span>
										{:else}
											<span class="text-xs text-muted-foreground">{tr("Belum ada artikel")}</span>
										{/if}
									</td>
									<td class="px-4 py-3 whitespace-nowrap">
										{#if m.publish.status === 'published'}
											<span class="inline-flex items-center gap-1.5">
												<span class="inline-flex items-center gap-1 rounded-full border border-emerald-200/80 bg-emerald-50 px-2.5 py-0.5 text-[11px] font-semibold text-emerald-600">
													<CircleCheck size={11} class="text-emerald-500" />
													{tr("Terbit")}
												</span>
												{#if m.publish.url}
													<a
														href={m.publish.url}
														target="_blank"
														rel="noreferrer"
														class="inline-flex items-center gap-1 rounded-lg border border-blue-200 bg-blue-50/60 px-2 py-0.5 text-[11px] font-semibold text-blue-600 hover:bg-blue-100/70 transition-colors"
														title={tr('Buka artikel di situs mitra')}
													>
														<ExternalLink size={11} />
														{tr("Lihat")}
													</a>
												{/if}
											</span>
										{:else if m.publish.status === 'failed'}
											<span class="rounded-full px-2.5 py-0.5 text-[11px] font-semibold bg-rose-50 border border-rose-200 text-rose-600">{tr("Gagal")}</span>
										{:else if m.publish.status === 'processing' || m.publish.status === 'queued' || m.publish.status === 'retrying'}
											<span class="rounded-full px-2.5 py-0.5 text-[11px] font-semibold bg-blue-50 border border-blue-200 text-blue-600">Diproses</span>
										{:else if m.publish.status === 'revision'}
											<span class="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-[11px] font-semibold bg-blue-50 border border-blue-200 text-blue-600">
												<RotateCcw size={11} />{tr("Revisi - Menunggu Mitra")}
											</span>
										{:else if m.publish.status === 'waiting_partner' || m.publish.status === 'partner_approved' || m.publish.status === 'pending'}
											<span class="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-[11px] font-semibold bg-amber-50 border border-amber-200 text-amber-700">
												<Clock size={11} />{tr("Menunggu Mitra")}
											</span>
										{:else}
											<span class="text-xs text-muted-foreground">—</span>
										{/if}
									</td>
									<td class="px-4 py-3 text-right">
										<div class="relative inline-flex items-center">
											<button
												type="button"
												class="inline-flex h-8 w-8 items-center justify-center rounded-full border border-slate-200 bg-white text-slate-500 shadow-2xs hover:bg-slate-50 hover:text-slate-800 transition-colors"
												aria-haspopup="menu"
												aria-expanded={activeMenuId === m.media_id}
												aria-label={`${tr('Aksi untuk')} ${m.name}`}
												onclick={(e) => {
													e.stopPropagation();
													toggleRowMenu(m.media_id, e.currentTarget as HTMLElement);
												}}
												>
													<Ellipsis size={14} />
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
				coverAi={revisionTarget ? { brandId, campaignId, mediaId: revisionTarget.media_id, articleId: null } : null}
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
