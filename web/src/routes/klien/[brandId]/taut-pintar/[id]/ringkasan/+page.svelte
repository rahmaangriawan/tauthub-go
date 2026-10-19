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
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Check, CircleCheck, ClipboardList, Clock, Database, ExternalLink, Link as LinkIcon, Loader, LoaderCircle, MessageCircle, RefreshCw, RotateCcw, X } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import AgentSteps from '../../_agent-steps.svelte';
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

	// Klik "Hubungi" tidak langsung pindah halaman: tampilkan pernyataan dulu
	// dan wajib dicentang sebelum tombol lanjut aktif.
	let contactTarget = $state<MediaItem | null>(null);
	let contactAck = $state(false);

	function openContact(m: MediaItem) {
		contactAck = false;
		contactTarget = m;
	}

	function confirmContact() {
		if (!contactTarget?.publication_id || !contactAck) return;
		const id = contactTarget.publication_id;
		contactTarget = null;
		goto(`/klien/${brandUrl}/chat?pub=${id}`);
	}

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
								<th class="px-4 py-3 text-right">{tr("Aksi")}</th>
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
										<!-- Hubungi mitra tentang artikel ini; publikasi belum ada
										     (belum diajukan) belum punya forum. -->
										<button
											type="button"
											class="app-btn app-btn-soft inline-flex items-center gap-1.5 !min-h-7 !px-2.5 !text-[11.5px]"
											disabled={!m.publication_id}
											title={!m.publication_id ? tr('Publikasi belum diajukan') : tr('Hubungi mitra tentang artikel ini')}
											onclick={() => openContact(m)}
										>
											<MessageCircle size={12} />{tr('Hubungi')}
										</button>
									</td>
																		</tr>
								{/each}
							</tbody>
						</table>
					</div>

			{:else}
				<p class="mt-3 text-sm text-muted-foreground">{tr("Belum ada media terpilih.")}</p>
			{/if}
		</section>
	{/if}
</main>

{#if contactTarget}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4"
		style="background-color: rgba(0, 0, 0, 0.45);"
		role="presentation"
		onclick={(e) => { if (e.target === e.currentTarget) contactTarget = null; }}
	>
		<div
			class="app-card w-full max-w-lg"
			role="dialog"
			aria-modal="true"
			aria-label={tr('Ajukan Revisi lewat Obrolan')}
			tabindex="-1"
			onkeydown={(e) => { if (e.key === 'Escape') contactTarget = null; }}
			onclick={(e) => e.stopPropagation()}
		>
			<div class="flex items-start gap-3 border-b border-slate-100 px-5 pb-4 pt-5">
				<span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl" style="background-color: color-mix(in srgb, var(--primary) 10%, transparent); color: var(--primary);">
					<MessageCircle size={21} />
				</span>
				<div class="min-w-0 flex-1">
					<p class="text-lg leading-tight font-bold text-slate-900">{tr('Ajukan Revisi lewat Obrolan')}</p>
					<p class="mt-1 text-[13px] text-muted-foreground">{contactTarget.name}</p>
				</div>
				<button class="rounded-lg p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600" type="button" aria-label={tr('Tutup')} onclick={() => (contactTarget = null)}>
					<X size={18} />
				</button>
			</div>

			<div class="px-5 py-4">
				<p class="text-[13px] leading-relaxed text-slate-700">
					{tr('Dengan melanjutkan, kamu membuka sesi obrolan dengan mitra untuk artikel ini. Sesi berlaku 3×24 jam sejak pesan pertama dan riwayatnya dapat dibaca kedua pihak.')}
				</p>
				<ul class="mt-3 space-y-1.5 text-[12.5px] leading-relaxed text-slate-600">
					<li>• {tr('Gunakan untuk mengajukan revisi, penambahan atau perbaikan backlink, dan pertanyaan lain seputar artikel.')}</li>
					<li>• {tr('Setelah sesi berakhir, riwayat tetap dapat dibaca tetapi pesan baru tidak bisa dikirim.')}</li>
				</ul>

				<label class="mt-4 flex cursor-pointer items-start gap-2.5 rounded-xl border border-slate-200 px-3.5 py-3 transition-colors hover:border-slate-300">
					<input type="checkbox" bind:checked={contactAck} class="mt-0.5 h-4 w-4 shrink-0" style="accent-color: var(--primary);" />
					<span class="text-[12.5px] leading-relaxed text-slate-700">
						{tr('Saya mengerti dan ingin melanjutkan menghubungi mitra untuk artikel ini.')}
					</span>
				</label>
			</div>

			<div class="flex justify-end gap-2 border-t border-slate-100 px-5 py-4">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (contactTarget = null)}>{tr('Batal')}</button>
				<button class="app-btn app-btn-primary" type="button" disabled={!contactAck} onclick={confirmContact}>
					{tr('Lanjut ke Obrolan')}
				</button>
			</div>
		</div>
	</div>
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
