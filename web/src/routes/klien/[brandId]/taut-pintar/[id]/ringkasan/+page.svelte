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
	import { ArrowRight, CalendarClock, Check, CircleCheck, ClipboardList, Clock, Database, ExternalLink, FileText, Info, Link as LinkIcon, Loader, LoaderCircle, MessageCircle, Pencil, RefreshCw, RotateCcw, Send, Star, X } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import AgentSteps from '../../_agent-steps.svelte';
	import { MANUAL_PROFILER_REASON, rpCurrency, toJakartaDisplay } from '../../_shared';
	import PageTitle from '$lib/components/page-title.svelte';
	import DateTimePicker from '$lib/components/date-time-picker.svelte';
	import EditorModal from '$lib/components/article-editor-modal.svelte';

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);

	type CheckStatus = 'pending' | 'running' | 'passed' | 'failed' | 'review_required';

	/** Kunci per-artikel: media satu artikel tetap memakai media_id. */
	const pkey = (m: { media_id: string; slot?: number | null }) => {
		const slot = m.slot ?? 1;
		return slot > 1 ? `${m.media_id}:${slot}` : m.media_id;
	};
	const slotQuery = (slot?: number | null) => ((slot ?? 1) > 1 ? `?slot=${slot}` : '');
	type MediaItem = {
		media_id: string;
		/** Nomor artikel pada media ini (1 = artikel pertama). */
		slot?: number;
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
		canDistribute?: boolean;
		tokens?: { used: number };
	};

	const DONE_STATUSES = new Set(['submitted', 'scheduled', 'distributed']);

	/** Forum obrolan mitra. `topic` hanya diisi saat MITRA yang menginisiasi
	 *  (konfirmasi/revisi/kutipan); `last_message_by` menandai siapa bicara
	 *  terakhir — dua-duanya dipakai agar balasan mitra di forum yang dibuka
	 *  klien tetap terbaca sebagai "mitra sudah memberi kabar". */
	type PartnerThread = {
		publication_id?: string | null;
		topic?: string | null;
		last_message_by?: string | null;
		last_message?: { author_role?: string | null } | null;
	};
	const PARTNER_TOPICS = new Set(['confirm', 'revision', 'quote']);
	// null = belum/gagal dimuat → jangan memblokir edit karena masalah jaringan.
	let partnerAskedIds = $state<Set<string> | null>(null);

	let data = $state<AuditsData | null>(null);
	let campaignName = $state('');
	let error = $state('');
	let manualFlow = $state(false);
	let loading = $state(true);

	// Klik "Hubungi" tidak langsung pindah halaman: tampilkan pernyataan dulu
	// dan wajib dicentang sebelum tombol lanjut aktif.
	let contactTarget = $state<MediaItem | null>(null);
	let contactAck = $state(false);
	// Rating kecepatan terbit: hanya state tampilan (belum ada endpoint
	// penyimpanan) — interaksi bintang tetap hidup seperti desain.
	let contactRating = $state(0);
	let contactHover = $state(0);

	function openContact(m: MediaItem) {
		contactAck = false;
		contactRating = 0;
		contactHover = 0;
		contactTarget = m;
	}

	function confirmContact() {
		if (!contactTarget?.publication_id || !contactAck) return;
		const id = contactTarget.publication_id;
		const rating = contactRating;
		contactTarget = null;
		// Rating bintang dikirim bersama pembukaan forum (endpoint idempoten),
		// jadi nilainya tersimpan sebagai rating media dan muncul di kolom SLA
		// katalog. Kegagalan simpan tidak boleh menghalangi klien menghubungi mitra.
		if (rating > 0) {
			void apiRequest(`/v1/brands/${brandId}/chat/threads`, {
				method: 'POST',
				body: JSON.stringify({ publication_id: id, rating })
			}).catch(() => {});
		}
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
			const [res, threadsRes] = await Promise.all([
				apiRequest(`${apiBase}/gatekeeper/audits`),
				// Forum obrolan dipakai menentukan publikasi mana yang sudah
				// dihubungi mitra (konfirmasi/revisi/kutipan). Gagal memuat
				// bukan alasan memblokir edit — set-nya dibiarkan null.
				apiRequest(`/v1/brands/${brandId}/chat/threads`).catch(() => null)
			]);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat ringkasan distribusi.')));
			const audits: AuditsData = await res.json();
			data = audits;
			manualFlow = audits.campaign.entry_mode === 'manual';
			campaignName = audits.campaign?.name ?? '';
			if (threadsRes?.ok) {
				const t = (await threadsRes.json().catch(() => null)) as { threads?: PartnerThread[] } | null;
				if (t?.threads) partnerAskedIds = new Set(
					t.threads
						.filter((th) =>
							th.publication_id &&
							(PARTNER_TOPICS.has(th.topic ?? '') ||
								th.last_message_by === 'partner' ||
								th.last_message?.author_role === 'partner')
						)
						.map((th) => th.publication_id as string)
				);
			}
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
	/** Jumlah artikel per domain — dipakai menandai "Artikel 2/2" agar dua kartu
	 *  dengan domain sama tidak terbaca sebagai media ganda. */
	const slotTotals = $derived.by(() => {
		const counts: Record<string, number> = {};
		for (const m of mediaList) counts[m.media_id] = (counts[m.media_id] ?? 0) + 1;
		return counts;
	});
	/** Jumlah domain, bukan jumlah kartu: satu domain bisa memuat beberapa artikel. */
	const mediaCount = $derived(Object.keys(slotTotals).length);
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
				// Kampanye yang sudah membawa media (jalur Katalog Media/Kilat)
				// lanjut ke produksi artikel; yang belum punya media hanya bisa
				// dilanjutkan di The Profiler jalur lama.
				return mediaList.length > 0
					? { slug: 'writer', label: tr('Lanjut ke The Writer') }
					: { slug: 'profiler', label: tr('Lanjutkan kampanye lama (The Profiler)') };
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
	const capFirst = (v: string): string => (v ? v.charAt(0).toUpperCase() + v.slice(1) : v);
	let faviconFailed = $state<Record<string, boolean>>({});

	/* ─── Jadwal terbit publikasi (hanya media yang terbit belakangan) ─── */
	let pickerFor = $state<MediaItem | null>(null);
	let pickerInitial = $state<Date | null>(null);
	let afterSaveFor = $state<MediaItem | null>(null);

	// Editor artikel.
	let editorFor = $state<MediaItem | null>(null);
	let edTitle = $state('');
	let edHtml = $state('');
	let edCover = $state<string | null>(null);
	let edCategory = $state('');
	let edMeta = $state('');
	let edLoading = $state(false);
	let edSaving = $state(false);
	let edError = $state('');

	// QC ulang setelah artikel diedit: gerbang keras (konten & tautan) wajib
	// masih lulus sebelum kampanye boleh diterbitkan.
	let qcRunning = $state<Record<string, boolean>>({});

	async function runQc(mediaId: string, slot: number) {
		const key = pkey({ media_id: mediaId, slot });
		if (qcRunning[key]) return;
		qcRunning = { ...qcRunning, [key]: true };
		try {
			// slots membatasi audit ke artikel ini; tanpa itu satu artikel memicu
			// audit untuk semua artikel di domain yang sama (biaya AI berlipat).
			await apiRequest(`${apiBase}/gatekeeper/audit-all`, {
				method: 'POST',
				body: JSON.stringify({ mediaIds: [mediaId], slots: [slot] })
			}, 900_000);
		} catch {
			/* senyap — status QC tampil dari data audits */
		} finally {
			qcRunning = { ...qcRunning, [key]: false };
			await loadData();
		}
	}

	const scheduleFor = (m: MediaItem): string => m.scheduled_at ?? '';
	const fmtSchedule = (m: MediaItem): string => toJakartaDisplay(scheduleFor(m) || null) ?? 'Segera';
	/**
	 * Hanya publikasi yang MASIH MENUNGGU (terjadwal/menunggu mitra) yang boleh
	 * diedit & dijadwalkan ulang — bukan yang sedang diproses atau sudah terbit.
	 */
	const canEdit = (m: MediaItem): boolean => {
		if (!m.publication_id || !m.document_id) return false;
		const s = m.publish.status ?? '';
		if (s === 'published' || s === 'processing' || s === 'queued') return false;
		if (s === 'waiting_partner' || s === 'partner_approved' || s === 'pending') return true;
		return m.scheduled_at ? new Date(m.scheduled_at).getTime() > Date.now() : false;
	};

	/** Mitra sudah mengirim konfirmasi/revisi/kutipan untuk publikasi ini. */
	const partnerAsked = (m: MediaItem): boolean => {
		if (!partnerAskedIds) return true; // tidak diketahui → jangan blokir
		return Boolean(m.publication_id && partnerAskedIds.has(m.publication_id));
	};

	/**
	 * Boleh diedit sekarang? Selama kampanye belum selesai, aturan lama berlaku.
	 * Setelah kampanye selesai, edit hanya dibuka bila mitra sudah lebih dulu
	 * menghubungi (konfirmasi/revisi/kutipan) atau artikel sudah berstatus
	 * revisi — supaya artikel yang masih ditinjau tidak diubah diam-diam.
	 */
	const editableNow = (m: MediaItem): boolean =>
		canEdit(m) && (!done || partnerAsked(m) || m.publish.status === 'revision');

	const editorLocked = $derived(done && !(editorFor ? editableNow(editorFor) : false));

	function openPicker(m: MediaItem) {
		pickerFor = m;
		const at = scheduleFor(m);
		pickerInitial = at ? new Date(at) : null;
	}

	async function reschedule(m: MediaItem, iso: string | null) {
		if (!m.publication_id) return;
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/publications/${m.publication_id}/schedule`, {
				method: 'POST',
				body: JSON.stringify({ scheduled_at: iso })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal mengubah jadwal terbit.')));
			await loadData();
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal mengubah jadwal terbit.');
		}
	}

	function applySchedule(date: Date) {
		const target = pickerFor;
		if (!target) return;
		pickerFor = null;
		// "Sekarang" = terbit pada pemrosesan berikutnya (bukan timestamp beku).
		const iso = date.getTime() <= Date.now() + 60_000 ? null : date.toISOString();
		void reschedule(target, iso);
	}

	/** Popover pasca simpan: pilih terbit sekarang atau jadwalkan. */
	function choosePublishNow() {
		const target = afterSaveFor;
		afterSaveFor = null;
		if (target) void reschedule(target, null);
	}

	function choosePublishSchedule() {
		const target = afterSaveFor;
		afterSaveFor = null;
		if (target) openPicker(target);
	}

	/* ─── Editor artikel ─── */

	async function openEditor(m: MediaItem) {
		if (!m.document_id) return;
		editorFor = m;
		edLoading = true;
		edError = '';
		edTitle = m.article.title ?? '';
		edHtml = '';
		edCover = m.featured_image_url;
		edCategory = '';
		edMeta = '';
		try {
			const res = await apiRequest(`${apiBase}/articles/${m.media_id}${slotQuery(m.slot)}`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat artikel.')));
			const d = await res.json();
			edTitle = d.title ?? edTitle;
			edHtml = d.content_html ?? '';
			edCover = d.featured_image_url ?? edCover;
			edCategory = d.category ?? '';
			edMeta = d.meta_description ?? '';
		} catch (reason) {
			edError = reason instanceof Error ? reason.message : tr('Gagal memuat artikel.');
		} finally {
			edLoading = false;
		}
	}

	async function uploadCover(file: File): Promise<string> {
		const form = new FormData();
		form.append('file', file);
		const res = await apiRequest(`/v1/brands/${brandId}/assets`, { method: 'POST', body: form });
		const d = await res.json().catch(() => null);
		if (!res.ok || !d?.url) throw new Error(d?.detail ?? tr('Gagal mengunggah cover.'));
		return d.url as string;
	}

	async function saveArticle(title: string, html: string, coverUrl: string | null, category: string, metaDescription: string) {
		const target = editorFor;
		if (!target || edSaving) return;
		if (editorLocked) return; // kampanye selesai & artikel sudah terbit/diproses
		edSaving = true;
		edError = '';
		try {
			const res = await apiRequest(`${apiBase}/articles/${target.media_id}${slotQuery(target.slot)}`, {
				method: 'PUT',
				body: JSON.stringify({ title, content_html: html, featured_image_url: coverUrl ?? undefined, category, meta_description: metaDescription })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan artikel.')));
			editorFor = null;
			afterSaveFor = target;
			void runQc(target.media_id, target.slot ?? 1);
			await loadData();
		} catch (reason) {
			edError = reason instanceof Error ? reason.message : tr('Gagal menyimpan artikel.');
		} finally {
			edSaving = false;
		}
	}

</script>

<PageTitle title="Taut Pintar — Ringkasan Distribusi" />

<main>
	<!-- Kampanye selesai: langkah pemilihan media & profil ikut terkunci supaya
	     tidak ada jalan menambah media ke kampanye yang sudah ditutup. -->
	<AgentSteps
		current={3}
		tone={bannerState === 'published' ? 'published' : bannerState === 'waiting' ? 'waiting' : undefined}
		extra="ringkasan"
		lockedSteps={done ? [1, 2] : []}
		lockedReason={tr('Kampanye sudah selesai — mode lihat saja.')}
	/>

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

		{#snippet publishStatus(m: MediaItem)}
			{#if m.publish.status === 'published'}
				<span class="inline-flex items-center gap-1 text-[11.5px] font-semibold text-emerald-600">
					{tr("Terbit")}
					{#if m.publish.url}
						<a
							href={m.publish.url}
							target="_blank"
							rel="noreferrer"
							class="text-emerald-600 transition-colors hover:text-emerald-700"
							title={tr('Buka artikel di situs mitra')}
							aria-label={tr('Buka artikel di situs mitra')}
						>
							<ExternalLink size={11} />
						</a>
					{/if}
				</span>
			{:else if m.publish.status === 'failed'}
				<span class="text-[11.5px] font-semibold text-rose-600">{tr("Gagal")}</span>
			{:else if m.publish.status === 'processing' || m.publish.status === 'queued' || m.publish.status === 'retrying'}
				<span class="text-[11.5px] font-semibold text-blue-600">{tr("Diproses")}</span>
			{:else if m.publish.status === 'revision'}
				<span class="text-[11.5px] font-semibold text-blue-600">{tr("Revisi")}</span>
			{:else if m.publish.status === 'waiting_partner' || m.publish.status === 'partner_approved' || m.publish.status === 'pending'}
				<span class="text-[11.5px] font-semibold text-amber-600">{tr("Menunggu mitra")}</span>
			{:else}
				<span class="text-[11.5px] font-medium text-slate-500">{tr("Belum terbit")}</span>
			{/if}
		{/snippet}

		<section class="app-card mt-3 p-4 sm:p-5">
			<!-- Judul dan tombol segarkan satu baris; teksnya yang menyusut, bukan tombolnya. -->
			<div class="flex items-start gap-3">
				<span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-blue-50 text-blue-600 shadow-2xs">
					<svg xmlns="http://www.w3.org/2000/svg" width="22" height="22" viewBox="0 0 24 24" aria-hidden="true">
						<path d="M0 0h24v24H0z" fill="none" />
						<path fill="currentColor" d="M21.04 2.96c-.35-.35-.855-.46-1.315-.29l-16.9 6.225c-.47.17-.79.605-.815 1.105c-.03.5.245.965.69 1.19l6.035 3.02l4.735-4.735l1.06 1.06l-4.735 4.735l3.02 6.035c.21.425.645.69 1.115.69H14c.5-.03.93-.35 1.105-.815L21.33 4.275c.17-.46.06-.965-.29-1.315" />
					</svg>
				</span>
				<div class="min-w-0 flex-1">
					<div class="flex flex-wrap items-center gap-2">
						<p class="font-bold text-base text-slate-900">{tr("Media Terpilih (")}{mediaCount})</p>
						{#if mediaList.length !== mediaCount}
							<span class="rounded-full bg-slate-100 px-2.5 py-0.5 text-[10.5px] font-semibold text-slate-500">
								{mediaList.length} {tr('Artikel')}
							</span>
						{/if}
					</div>
					<p class="mt-0.5 text-xs text-slate-500">{tr("Daftar media untuk distribusi konten kampanye ini.")}</p>
				</div>
				<button
					class="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-700 shadow-2xs transition-colors hover:bg-slate-50"
					type="button"
					aria-label={tr('Segarkan')}
					title={tr('Segarkan')}
					onclick={loadData}
				>
					<RefreshCw size={15} class="text-blue-600" />
				</button>
			</div>

			{#if mediaList.length > 0}
				<!-- Satu kartu per artikel. Tabel delapan kolom harus digeser ke samping
				     di HP, jadi isinya dibaca sebagai kartu; di desktop kartu yang sama
				     mengalir dua–tiga kolom. -->
				<div class="mt-4 grid gap-3 md:hidden">
					{#each mediaList as m (pkey(m))}
						{@const domain = cleanDomain(m.website_url, m.name)}
						{@const total = slotTotals[m.media_id] ?? 1}
						{@const scheduleText = canEdit(m) ? (scheduleFor(m) ? fmtSchedule(m) : tr('Segera')) : (toJakartaDisplay(m.scheduled_at) ?? tr('Segera'))}
						<article class="rounded-xl border border-slate-200/70 bg-white p-4 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
							<!-- Kepala: favicon, domain, tipe media, dan tombol hubungi mitra. -->
							<div class="flex items-center gap-3">
								<span
									class="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-full border border-slate-200/70 text-sm font-bold"
									style="background-color: {AVA[avatarTone(m.name)]};"
								>
									{#if domain && !faviconFailed[pkey(m)]}
										<img
											src={`https://www.google.com/s2/favicons?domain=${encodeURIComponent(domain)}&sz=128`}
											alt=""
											class="h-full w-full object-cover"
											loading="lazy"
											decoding="async"
											onerror={() => (faviconFailed = { ...faviconFailed, [pkey(m)]: true })}
										/>
									{:else}
										{initialOf(m.name)}
									{/if}
								</span>
								<div class="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1">
									<span class="min-w-0 max-w-full truncate text-[15px] font-bold leading-tight text-slate-900">{domain || m.name || '—'}</span>
									{#if m.media_type}
										<span class="shrink-0 rounded-full bg-blue-50 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.05em] text-blue-600">{m.media_type}</span>
									{/if}
									{#if total > 1}
										<span class="shrink-0 rounded-full bg-slate-100 px-2.5 py-1 text-[10px] font-semibold text-slate-500">{tr('Artikel')} {m.slot ?? 1}/{total}</span>
									{/if}
								</div>
								<button
									type="button"
									class="inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-xl border border-slate-200 bg-white text-slate-700 transition-colors hover:border-slate-300 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40"
									disabled={!m.publication_id}
									aria-label={tr('Hubungi')}
									title={!m.publication_id ? tr('Publikasi belum diajukan') : tr('Hubungi mitra tentang artikel ini')}
									onclick={() => openContact(m)}
								>
									<MessageCircle size={16} />
								</button>
							</div>

							<!-- Isi artikel. Judul yang boleh diedit sekaligus jadi tombol edit,
							     supaya tidak perlu tombol tambahan di kartu. -->
							<div class="mt-3 border-t border-slate-100 pt-3">
								{#if m.article.title}
									{#if editableNow(m)}
										<button
											type="button"
											class="-m-1 flex w-full items-start gap-2 rounded-lg p-1 text-left transition-colors hover:bg-slate-50"
											title={tr('Edit artikel lalu atur waktu terbitnya')}
											onclick={() => void openEditor(m)}
										>
											<span class="line-clamp-2 text-[14.5px] font-semibold leading-snug text-slate-900">{m.article.title}</span>
											<Pencil size={13} class="mt-1 shrink-0 text-slate-400" />
										</button>
									{:else}
										<p class="line-clamp-2 text-[14.5px] font-semibold leading-snug text-slate-900">{m.article.title}</p>
									{/if}
								{:else}
									<div class="flex items-center gap-3">
										<span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-slate-100 text-slate-400">
											<FileText size={18} />
										</span>
										<div class="min-w-0">
											<p class="text-[13.5px] font-bold text-slate-900">{tr('Belum ada artikel')}</p>
											<p class="mt-0.5 text-[11.5px] leading-snug text-slate-500">{tr('Tambahkan artikel untuk media ini.')}</p>
										</div>
									</div>
								{/if}
							</div>

							<!-- Empat keterangan: DA, jumlah link, jadwal, status terbit. -->
							<div class="mt-3 flex flex-wrap items-start justify-between gap-x-4 gap-y-2.5 border-t border-slate-100 pt-3">
								<div class="min-w-0">
									<p class="text-[10.5px] font-medium text-slate-400">DA</p>
									<p class="mt-0.5 text-[13.5px] font-bold leading-none text-slate-900">{m.da != null ? m.da : '—'}</p>
								</div>
								<div class="min-w-0">
									<p class="text-[10.5px] font-medium text-slate-400">{tr('Tautan')}</p>
									<div class="mt-0.5 flex items-center gap-1.5">
										<LinkIcon size={12} class="shrink-0 text-slate-400" />
										<span
											class="whitespace-nowrap text-[11.5px] font-medium text-slate-600"
											title={m.article.links.count > 0 && m.article.links.rels.length > 0 ? m.article.links.rels.join(', ') : undefined}
										>
											{#if m.article.links.count > 0}{m.article.links.count} link{:else}{tr('Tidak ada link')}{/if}
										</span>
									</div>
								</div>
								<div class="min-w-0">
									<p class="text-[10.5px] font-medium text-slate-400">{tr('Tayang')}</p>
									<div class="mt-0.5 flex items-center gap-1.5">
										{#if canEdit(m)}
											<!-- Jadwal sekaligus jadi tombol pengatur waktu terbit. -->
											<button
												type="button"
												class="-m-1 inline-flex items-center gap-1.5 rounded-md p-1 text-[11.5px] font-semibold transition-colors hover:bg-slate-50 {scheduleFor(m) ? 'text-indigo-600' : 'text-emerald-600'}"
												title={tr('Atur waktu terbit media ini')}
												onclick={() => openPicker(m)}
											>
												<Clock size={12} class="shrink-0 {scheduleFor(m) ? 'text-indigo-500' : 'text-emerald-500'}" />
												<span class="whitespace-nowrap">{scheduleText}</span>
											</button>
										{:else}
											<Clock size={12} class="shrink-0 text-emerald-500" />
											<span class="whitespace-nowrap text-[11.5px] font-semibold text-emerald-600">{scheduleText}</span>
										{/if}
									</div>
								</div>
								<div>
									<p class="text-[10.5px] font-medium text-slate-400">{tr('Status terbit')}</p>
									<div class="mt-0.5">{@render publishStatus(m)}</div>
								</div>
							</div>

							{#if canEdit(m) && scheduleFor(m)}
								<div class="mt-2">
									<button
										type="button"
										class="inline-flex items-center gap-1 text-[11px] font-medium text-slate-500 transition-colors hover:text-slate-700"
										title={tr('Batalkan jadwal — terbit sekarang')}
										onclick={() => void reschedule(m, null)}
									>
										<X size={11} /> {tr('Batalkan jadwal')}
									</button>
								</div>
							{/if}
						</article>
					{/each}
				</div>

				<div class="mt-4 hidden overflow-x-auto rounded-xl border border-slate-200/70 md:block">
					<table class="w-full text-sm">
						<thead>
							<tr class="border-b border-slate-100 bg-white text-left text-[10.5px] font-bold uppercase tracking-[0.07em] text-slate-400">
								<th class="px-3.5 py-3.5">{tr("Media")}</th>
								<th class="px-3.5 py-3.5">Tipe</th>
								<th class="px-3.5 py-3.5">DA</th>
								<th class="px-3.5 py-3.5">Link Artikel</th>
								<th class="px-3.5 py-3.5">{tr("Jadwal Terbit")}</th>
								<th class="px-3.5 py-3.5">Judul Artikel</th>
								<th class="px-3.5 py-3.5">{tr("Status Terbit")}</th>
								<th class="px-3.5 py-3.5 text-right">{tr("Aksi")}</th>
							</tr>
						</thead>
						<tbody class="divide-y divide-slate-100 bg-white">
							{#each mediaList as m (pkey(m))}
								{@const domain = cleanDomain(m.website_url, m.name)}
								<tr class="transition-colors hover:bg-slate-50/60">
									<td class="px-3.5 py-4">
										<div class="flex items-center gap-3">
											<span class="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full border border-slate-200/70 text-xs font-bold ring-2 ring-white" style="background-color: {AVA[avatarTone(m.name)]};">
												{#if domain && !faviconFailed[pkey(m)]}
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
													<span class="block text-[11.5px] text-slate-400 font-normal mt-0.5 truncate">{domain}</span>
												{/if}
											</div>
										</div>
									</td>
									<td class="px-3.5 py-4 whitespace-nowrap">
										{#if m.media_type}
											<span class="rounded-full border border-blue-200/70 bg-blue-50 px-3 py-1 text-[11px] font-semibold text-blue-700">
												{m.media_type}
											</span>
										{:else}
											—
										{/if}
									</td>
									<td class="px-3.5 py-4 text-[13px] font-bold text-slate-900">{m.da != null ? m.da : '—'}</td>
									<td class="px-3.5 py-4 whitespace-nowrap">
										{#if m.article.links.count > 0}
											<span class="inline-flex items-center gap-1.5 text-[12px] font-medium text-slate-700">
												<LinkIcon size={12} class="text-slate-400 shrink-0" />
												{m.article.links.count} link{m.article.links.rels.length > 0 ? ` · ${m.article.links.rels.join(', ')}` : ' dofollow'}
											</span>
										{:else}
											<span class="text-xs text-muted-foreground">{tr("Tidak ada link")}</span>
										{/if}
									</td>
									<td class="px-3.5 py-4 whitespace-nowrap">
										{#if canEdit(m)}
											<div class="flex items-center gap-1">
												<button
													type="button"
													class="inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1 text-[11.5px] font-semibold transition-colors {scheduleFor(m)
														? 'border-indigo-200 bg-indigo-50 text-indigo-700 hover:bg-indigo-100'
														: 'border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100'}"
													title={tr('Atur waktu terbit media ini')}
													onclick={() => openPicker(m)}
												>
													<CalendarClock size={12} />
													{scheduleFor(m) ? fmtSchedule(m) : tr('Segera')}
												</button>
												{#if scheduleFor(m)}
													<button
														type="button"
														class="rounded-md p-1 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600"
														aria-label={tr('Batalkan jadwal')}
														title={tr('Batalkan jadwal — terbit sekarang')}
														onclick={() => void reschedule(m, null)}
													>
														<X size={12} />
													</button>
												{/if}
											</div>
										{:else}
											<span class="inline-flex items-center gap-1.5 text-[12px] font-semibold text-emerald-600">
												<Clock size={12} class="text-emerald-500 shrink-0" />
												{toJakartaDisplay(m.scheduled_at) ?? 'Segera'}
											</span>
										{/if}
									</td>
									<td class="max-w-[240px] px-3.5 py-4">
										{#if m.article.title}
											<span class="block truncate text-[12.5px] text-slate-700" title={m.article.title}>{m.article.title}</span>
										{:else}
											<span class="text-xs text-muted-foreground">{tr("Belum ada artikel")}</span>
										{/if}
									</td>
									<td class="px-3.5 py-4 whitespace-nowrap">{@render publishStatus(m)}</td>
									<td class="px-3.5 py-4">
										<div class="flex flex-wrap items-center justify-end gap-1.5">
											{#if editableNow(m)}
												<button
													type="button"
													class="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-700 shadow-2xs transition-colors hover:bg-slate-50 hover:border-slate-300"
													aria-label={tr('Edit Artikel')}
													title={tr('Edit artikel lalu atur waktu terbitnya')}
													onclick={() => void openEditor(m)}
												>
													<Pencil size={13} />
												</button>
											{/if}
											<!-- Hubungi mitra tentang artikel ini; publikasi belum ada
											     (belum diajukan) belum punya forum. -->
											<button
												type="button"
												class="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-slate-200 bg-white text-slate-700 shadow-2xs transition-colors hover:bg-slate-50 hover:border-slate-300 disabled:cursor-not-allowed disabled:opacity-50"
												disabled={!m.publication_id}
												aria-label={tr('Hubungi')}
												title={!m.publication_id ? tr('Publikasi belum diajukan') : tr('Hubungi mitra tentang artikel ini')}
												onclick={() => openContact(m)}
											>
												<MessageCircle size={13} />
											</button>
										</div>
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

<!-- Editor artikel: dipakai untuk memperbaiki artikel sebelum terbit. -->
{#if editorFor}
	{#if edLoading}
		<div class="fixed inset-0 z-[60] flex items-center justify-center p-4" style="background-color: rgba(15, 23, 42, 0.45);" role="presentation">
			<div class="app-card flex items-center gap-2 px-5 py-4 text-sm font-semibold">
				<LoaderCircle size={17} class="animate-spin" /> {tr('Memuat artikel…')}
			</div>
		</div>
	{:else}
		<EditorModal
			mediaName={editorFor.name}
			{brandId}
			{campaignId}
			mediaId={editorFor.media_id}
			initialTitle={edTitle}
			initialHtml={edHtml}
			initialCoverUrl={edCover}
			initialCategory={edCategory}
			initialMetaDescription={edMeta}
			categoryOptions={edCategory ? [edCategory] : []}
			linkLimit={editorFor.link_capacity && editorFor.link_capacity > 0 ? editorFor.link_capacity : 2}
			enforceLinkLimit={false}
			requireCover={false}
			saving={edSaving}
			saveError={edError}
			readOnly={editorLocked}
			showSaveButton={!editorLocked}
			subtitle={tr('Edit artikel, simpan, lalu pilih kapan artikel ini terbit.')}
			onClose={() => { if (!edSaving) editorFor = null; }}
			onSave={(title, html, coverUrl, category, metaDescription) => void saveArticle(title, html, coverUrl, category, metaDescription)}
			onUploadCover={uploadCover}
		/>
	{/if}
{/if}

<!-- Popover pasca simpan: terbit sekarang atau jadwalkan. -->
{#if afterSaveFor}
	<div class="fixed inset-0 z-[70] flex items-center justify-center p-4" style="background-color: rgba(15, 23, 42, 0.5);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) afterSaveFor = null; }}>
		<div class="app-card w-full max-w-sm overflow-hidden" role="dialog" aria-modal="true" aria-label={tr('Artikel tersimpan')} tabindex="-1" onkeydown={(e) => { if (e.key === 'Escape') afterSaveFor = null; }}>
			<div class="flex items-start gap-3 px-5 pb-2 pt-5">
				<span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl" style="background-color: var(--success-soft); color: var(--success);"><CircleCheck size={20} /></span>
				<div class="min-w-0 flex-1">
					<p class="text-base leading-tight font-bold text-slate-900">{tr('Artikel tersimpan')}</p>
					<p class="mt-1 text-[13px] leading-relaxed text-muted-foreground">{tr('Mau artikel ini terbit sekarang atau dijadwalkan dulu?')}</p>
					<p class="mt-1 truncate text-[12px] font-semibold text-slate-700">{afterSaveFor.name}</p>
					<p class="mt-1 text-[11px] leading-snug text-muted-foreground">{tr('Perubahan jadwal langsung diterapkan ke publikasi media ini.')}</p>
				</div>
			</div>
			<div class="grid gap-2 px-5 pb-3 pt-3">
				<button class="app-btn app-btn-primary inline-flex w-full items-center justify-center gap-1.5 !min-h-10 !px-3 !text-xs" type="button" onclick={choosePublishNow}>
					<Send size={14} /> {tr('Terbit sekarang')}
				</button>
				<button class="app-btn app-btn-soft inline-flex w-full items-center justify-center gap-1.5 !min-h-10 !px-3 !text-xs" type="button" onclick={choosePublishSchedule}>
					<CalendarClock size={14} /> {tr('Jadwalkan dulu')}
				</button>
			</div>
			<div class="border-t px-5 py-3" style="border-color: var(--border);">
				<button class="text-xs font-medium text-muted-foreground transition-colors hover:text-slate-700" type="button" onclick={() => (afterSaveFor = null)}>{tr('Nanti saja')}</button>
			</div>
		</div>
	</div>
{/if}

{#if pickerFor}
	<DateTimePicker
		open
		title={pickerFor ? `${tr('Jadwal terbit')} — ${pickerFor.name}` : tr('Jadwal terbit')}
		value={pickerInitial}
		min={new Date()}
		onClose={() => { pickerFor = null; }}
		onApply={applySchedule}
	/>
{/if}

{#if contactTarget}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4"
		style="background-color: rgba(0, 0, 0, 0.45);"
		role="presentation"
		onclick={(e) => { if (e.target === e.currentTarget) contactTarget = null; }}
	>
		<div
			class="app-card flex max-h-[92dvh] w-full max-w-lg flex-col overflow-hidden"
			role="dialog"
			aria-modal="true"
			aria-label={tr('Konfirmasi Sesi Obrolan')}
			tabindex="-1"
			onkeydown={(e) => { if (e.key === 'Escape') contactTarget = null; }}
			onclick={(e) => e.stopPropagation()}
		>
			<!-- Kepala -->
			<div class="flex items-start gap-3 px-5 pb-3 pt-5">
				<span class="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-blue-50 text-blue-600">
					<MessageCircle size={21} />
				</span>
				<div class="min-w-0 flex-1">
					<p class="text-lg leading-tight font-bold text-slate-900">{tr('Konfirmasi Sesi Obrolan')}</p>
					<p class="mt-1 truncate text-[13px] text-muted-foreground">{tr('Mitra:')} {capFirst(cleanDomain(contactTarget.website_url, contactTarget.name))}</p>
				</div>
				<button class="shrink-0 rounded-lg p-1.5 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600" type="button" aria-label={tr('Tutup')} onclick={() => (contactTarget = null)}>
					<X size={18} />
				</button>
			</div>

			<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-4">
				<!-- Rating kecepatan terbit -->
				<div class="rounded-2xl border border-slate-200 bg-white px-4 py-4 text-center">
					<p class="text-[13.5px] font-bold text-slate-900">{tr('Beri Rating Kecepatan Terbit')}</p>
					<p class="mt-1 text-[12px] leading-relaxed text-slate-500">{tr('Seberapa cepat mitra memproses dan menayangkan artikel Anda?')}</p>
					<div class="mt-3 flex items-center justify-center gap-2" role="radiogroup" aria-label={tr('Beri Rating Kecepatan Terbit')}>
						{#each [1, 2, 3, 4, 5] as n (n)}
							<button
								type="button"
								role="radio"
								aria-checked={contactRating === n}
								aria-label={`${n} / 5`}
								class="flex h-9 w-9 items-center justify-center rounded-lg transition-colors {n <= (contactHover || contactRating) ? 'text-amber-400' : 'text-slate-300 hover:text-amber-300'}"
								onmouseenter={() => (contactHover = n)}
								onmouseleave={() => (contactHover = 0)}
								onclick={() => (contactRating = n)}
							>
								<Star size={22} fill={n <= (contactHover || contactRating) ? 'currentColor' : 'none'} strokeWidth={1.8} />
							</button>
						{/each}
					</div>
					<div class="mt-1.5 flex items-center justify-between px-1">
						<span class="text-[10.5px] text-slate-400">{tr('Sangat Lambat')}</span>
						<span class="text-[10.5px] text-slate-400">{tr('Sangat Cepat')}</span>
					</div>
				</div>

				<!-- Ketentuan ruang obrolan -->
				<div class="mt-3.5 rounded-xl border border-blue-100 bg-blue-50/60 px-4 py-3.5">
					<p class="flex items-center gap-2 text-[12.5px] font-bold text-blue-900">
						<Info size={14} class="shrink-0 text-blue-600" />
						{tr('Ketentuan Ruang Obrolan')}
					</p>
					<ol class="mt-2.5 list-decimal space-y-1.5 pl-4 text-[12px] leading-relaxed text-slate-600">
						<li>{tr('Dengan melanjutkan, kamu membuka sesi obrolan dengan mitra untuk artikel ini. Sesi berlaku 3×24 jam sejak pesan pertama dan riwayatnya dapat dibaca kedua pihak.')}</li>
						<li>{tr('Gunakan untuk mengajukan revisi, penambahan atau perbaikan backlink, dan pertanyaan lain seputar artikel.')}</li>
						<li>{tr('Setelah sesi berakhir, riwayat tetap dapat dibaca tetapi pesan baru tidak bisa dikirim.')}</li>
						<li>{tr('Pesan pada ruang ini dapat ditinjau tim admin Tauthub untuk keperluan operasional dan penyelesaian masalah.')}</li>
					</ol>
				</div>

				<!-- Persetujuan wajib -->
				<label class="mt-3.5 flex cursor-pointer items-start gap-2.5 rounded-xl border border-slate-200 px-3.5 py-3 transition-colors hover:border-slate-300">
					<input type="checkbox" bind:checked={contactAck} class="mt-0.5 h-4 w-4 shrink-0" style="accent-color: var(--primary);" />
					<span class="text-[12.5px] leading-relaxed text-slate-700">
						{tr('Saya mengerti dan ingin melanjutkan menghubungi mitra untuk artikel ini.')}
					</span>
				</label>
			</div>

			<!-- Kaki -->
			<div class="flex flex-col-reverse gap-2 border-t border-slate-100 px-5 py-4 sm:flex-row sm:justify-end">
				<button class="app-btn app-btn-soft w-full justify-center sm:w-auto" type="button" onclick={() => (contactTarget = null)}>{tr('Batal')}</button>
				<button class="app-btn app-btn-primary inline-flex w-full items-center justify-center gap-1.5 sm:w-auto" type="button" disabled={!contactAck} onclick={confirmContact}>
					{tr('Lanjut ke Ruang Obrolan')}
					<ArrowRight size={14} />
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
