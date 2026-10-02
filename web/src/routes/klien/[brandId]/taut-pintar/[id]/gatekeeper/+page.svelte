<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Agent 4: The Gatekeeper — QC & pembayaran distribusi.
	// Port app/klien/[brandId]/taut-pintar/[id]/gatekeeper/client.tsx.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { CircleAlert, Clock, LoaderCircle, ShieldAlert, ShieldCheck, TriangleAlert, Wallet, X } from '@lucide/svelte';
	import { Tooltip } from 'bits-ui';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { sanitizeHtml } from '$lib/sanitize-html';
	import AgentSteps from '../../_agent-steps.svelte';
	import TautBottomBar from '../../_bottom-bar.svelte';
	import { MANUAL_PROFILER_REASON, fmtIdr, fmtTrafficFull, toJakartaDisplay, toLocalInput, dt } from '../../_shared';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);
	const backUrl = $derived(`/klien/${brandUrl}/taut-pintar/${campaignId}/writer`);

	type CheckStatus = 'pending' | 'running' | 'passed' | 'failed' | 'review_required';
	type Check = {
		status: CheckStatus;
		percent?: number;
		riskLevel?: string;
		categories?: string[];
		unsafeUrls?: string[];
		level?: string;
	};
	type Checks = { plagiarism: Check; contentSafety: Check; linkHealth: Check; brandVoice: Check };
	type MediaItem = {
		media_id: string;
		name: string;
		media_type: string | null;
		price_idr: number;
		da: number | null;
		ss: number | null;
		est_traffic: number | null;
		link_capacity: number | null;
		scheduled_at: string | null;
		article: { title: string; updated_at: string | null };
		audit: { status: CheckStatus; checks: Checks };
	};
	type AuditsData = {
		campaign: { id: string; taut_status: string | null; entry_mode?: 'ai' | 'manual' | null };
		media: MediaItem[];
		canDistribute: boolean;
		totalCostIdr: number;
		totalCostTc: number;
		rateIdr: number;
	};

	const CHECK_CARDS: Array<{ key: keyof Checks; title: string; gate: boolean }> = [
		{ key: 'plagiarism', title: 'Plagiarism Checker', gate: false },
		{ key: 'contentSafety', title: 'Illegal Content Guard', gate: true },
		{ key: 'linkHealth', title: 'Link Health Validator', gate: true },
		{ key: 'brandVoice', title: 'Brand Voice Limits', gate: false }
	];
	const DEFAULT_DESC = $derived(tr('Menunggu pemindaian…'));

	let data = $state<AuditsData | null>(null);
	let wallet = $state(0);
	let loading = $state(true);
	let error = $state('');
	let agentStatus = $state<{ configured: boolean; reason: string | null } | null>(null);
	let auditing = $state(false);
	let distributing = $state(false);
	let topup = $state<{ needed: number; available: number } | null>(null);
	let manualFlow = $state(false);
	let preview = $state<{
		mediaId: string;
		mediaName: string;
		title: string;
		html: string;
		updatedAt: string | null;
		cover: string | null;
		category: string | null;
		description: string;
		stats: { imageCount: number; images: string[]; words: number };
	} | null>(null);
	// Jadwal per media: nilai tersimpan dari server digabung dengan perubahan user.
	let scheduleEdits = $state<Record<string, string | null>>({});
	let publishModeChoice = $state<'now' | 'scheduled' | null>(null);
	// Dropdown jadwal: menempel pada tombol yang diklik, diposisikan fixed
	// supaya tidak mendorong baris tabel (tanpa pergeseran layout).
	let scheduleTarget = $state<MediaItem | null>(null);
	let schedulePos = $state({ left: 0, top: 0 });
	const SCHEDULE_PANEL_WIDTH = 300;
	const SCHEDULE_PANEL_HEIGHT = 268;

	function openSchedule(item: MediaItem, trigger: HTMLElement) {
		const rect = trigger.getBoundingClientRect();
		const left = Math.min(
			Math.max(8, rect.right - SCHEDULE_PANEL_WIDTH),
			Math.max(8, window.innerWidth - SCHEDULE_PANEL_WIDTH - 8)
		);
		const spaceBelow = window.innerHeight - rect.bottom - 8;
		const placeUp = spaceBelow < SCHEDULE_PANEL_HEIGHT && rect.top > spaceBelow;
		const top = placeUp
			? Math.max(8, rect.top - SCHEDULE_PANEL_HEIGHT - 6)
			: rect.bottom + 6;
		schedulePos = { left, top };
		scheduleTarget = item;
	}

	$effect(() => {
		if (!scheduleTarget) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === 'Escape') scheduleTarget = null;
		};
		// Menutup saat halaman digulir/diubah ukurannya, seperti dropdown lain.
		const onScroll = (event: Event) => {
			const target = event.target as Node | null;
			const panel = document.getElementById('schedule-dropdown');
			if (target && panel && (target === panel || panel.contains(target))) return;
			scheduleTarget = null;
		};
		const onResize = () => (scheduleTarget = null);
		window.addEventListener('keydown', onKey);
		window.addEventListener('scroll', onScroll, true);
		window.addEventListener('resize', onResize);
		return () => {
			window.removeEventListener('keydown', onKey);
			window.removeEventListener('scroll', onScroll, true);
			window.removeEventListener('resize', onResize);
		};
	});

	let pollTimer: number | null = null;
	const savedSchedule = $derived(
		Object.fromEntries(
			(data?.media ?? []).filter((m) => m.scheduled_at != null).map((m) => [m.media_id, m.scheduled_at as string])
		) as Record<string, string | null>
	);
	const scheduleByMedia = $derived({ ...savedSchedule, ...scheduleEdits });
	const publishMode = $derived(publishModeChoice ?? (Object.keys(savedSchedule).length > 0 ? 'scheduled' : 'now'));
	const scheduleError = $derived(
		Object.values(scheduleByMedia).some((v) => v && new Date(v).getTime() <= Date.now())
			? tr('Ada jadwal yang sudah lewat — pilih waktu di masa depan.')
			: null
	);
	const modeError = $derived(
		publishMode !== 'scheduled'
			? null
			: Object.values(scheduleByMedia).some((v) => v != null)
				? null
				: tr('Pilih jadwal minimal satu media, atau ubah ke Terbitkan Langsung.')
	);
	const blockingError = $derived(scheduleError ?? modeError);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			void loadAgentStatus();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			if (pollTimer) window.clearInterval(pollTimer);
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
		};
	});

	let loadedOnce = false;

	async function load() {
		try {
			const [auditsRes, draftRes] = await Promise.all([
				apiRequest(`${apiBase}/gatekeeper/audits`),
				apiRequest(`${apiBase}/profile/draft`)
			]);
			if (!auditsRes.ok) throw new Error(await readApiError(auditsRes, tr('Gagal memuat hasil audit.')));
			const d: AuditsData = await auditsRes.json();
			data = d;
			if (!loadedOnce) {
				loadedOnce = true;
				error = '';
			}
			manualFlow = d.campaign.entry_mode === 'manual';
			const needsPoll = d.media.some(
				(m) => m.audit.status === 'running' || Object.values(m.audit.checks).some((c) => c?.status === 'running')
			);
			setPolling(needsPoll);
			if (draftRes.ok) {
				const dd = await draftRes.json().catch(() => null);
				wallet = dd?.wallet?.available ?? 0;
			}
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal memuat data Gatekeeper.');
		} finally {
			loading = false;
		}
	}

	function setPolling(on: boolean) {
		if (on && !pollTimer) {
			pollTimer = window.setInterval(() => void load(), 4000);
		} else if (!on && pollTimer) {
			window.clearInterval(pollTimer);
			pollTimer = null;
		}
	}

	async function loadAgentStatus() {
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/taut/agent-status?agent=gatekeeper`);
			if (res.ok) {
				const d = await res.json();
				const g = d.agents?.find((a: { agent: string }) => a.agent === 'gatekeeper');
				if (g) agentStatus = { configured: g.configured, reason: g.reason };
			}
		} catch {
			/* status opsional */
		}
	}

	function applyTopup(d: { code?: string; needed?: number; available?: number }): boolean {
		if (d.code === 'insufficient_credits' && typeof d.needed === 'number') {
			topup = { needed: d.needed, available: d.available ?? 0 };
			return true;
		}
		return false;
	}

	async function runAudit() {
		if (auditing) return;
		auditing = true;
		error = '';
		topup = null;
		try {
			// audit-all sinkron dan memindai SEMUA media terpilih, jadi durasinya
			// jauh di atas timeout bawaan apiRequest (12 detik). Tanpa timeout
			// panjang, koneksi diputus dan konteks request server ikut batal
			// sehingga loop audit berhenti di tengah jalan.
			const res = await apiRequest(`${apiBase}/gatekeeper/audit-all`, { method: 'POST', body: JSON.stringify({}) }, 900_000);
			const d = await res.json().catch(() => ({}));
			if (res.status === 402 && applyTopup(d)) return;
			if (!res.ok) throw new Error(d.detail ?? tr('Gagal menjalankan audit.'));
			await load();
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal menjalankan audit.');
		} finally {
			auditing = false;
		}
	}

	async function distribute() {
		if (distributing || !data?.canDistribute) return;
		distributing = true;
		error = '';
		topup = null;
		try {
			// Mode "langsung": jangan kirim jadwal sama sekali, sehingga backend
			// memakai perilaku serentak (semua scheduled_at = sekarang).
			const schedule =
				publishMode === 'now'
					? []
					: Object.entries(scheduleByMedia)
							.filter(([, v]) => v != null)
							.map(([media_id, scheduled_at]) => ({ media_id, scheduled_at }));
			const res = await apiRequest(`${apiBase}/gatekeeper/distribute`, {
				method: 'POST',
				body: JSON.stringify({ schedule })
			});
			const d = await res.json().catch(() => ({}));
			if (res.status === 402 && applyTopup(d)) return;
			if (!res.ok) throw new Error(d.detail ?? tr('Gagal mendistribusikan kampanye.'));
			await goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/ringkasan`);
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal mendistribusikan kampanye.');
			distributing = false;
		}
	}

	function setSchedule(mediaId: string, value: string | null) {
		scheduleEdits = { ...scheduleByMedia, [mediaId]: value };
	}

	async function openMockup(m: MediaItem) {
		error = '';
		try {
			const res = await apiRequest(`${apiBase}/articles/${m.media_id}`);
			if (!res.ok) throw new Error(await readApiError(res, 'Artikel belum tersedia.'));
			const d = await res.json();
			const html = d.content_html ?? '';
			const plain = html
				.replace(/<(script|style)[^>]*>[\s\S]*?<\/\1>/gi, ' ')
				.replace(/<[^>]+>/g, ' ');
			const words = plain.split(/\s+/).filter(Boolean).length;
			const srcs: string[] = [];
			const imgRe = /<img[^>]*\ssrc=["']([^"']+)["']/gi;
			let imgMatch: RegExpExecArray | null;
			while ((imgMatch = imgRe.exec(String(html))) !== null) srcs.push(imgMatch[1]);
			const description = (d.meta_description ?? '').trim() || (plain.length > 180 ? `${plain.slice(0, 180).trimEnd()}…` : plain);
			preview = {
				mediaId: m.media_id,
				mediaName: m.name,
				title: d.title ?? m.article.title,
				html,
				updatedAt: m.article.updated_at,
				cover: (d.featured_image_url ?? '').trim() || null,
				category: (d.category ?? '').trim() || null,
				description,
				stats: { imageCount: srcs.length, images: srcs, words }
			};
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal membuka mockup.');
		}
	}

	const previewHtml = $derived(preview ? sanitizeHtml(preview.html) : '');

	const cardStatus = $derived.by(() => {
		const out: Record<keyof Checks, CheckStatus> = {
			plagiarism: 'pending',
			contentSafety: 'pending',
			linkHealth: 'pending',
			brandVoice: 'pending'
		};
		if (!data || data.media.length === 0) return out;
		for (const card of CHECK_CARDS) {
			const statuses = data.media.map((m) => m.audit.checks[card.key].status);
			if (statuses.includes('running')) out[card.key] = 'running';
			else if (statuses.includes('failed')) out[card.key] = 'failed';
			else if (statuses.includes('review_required')) out[card.key] = 'review_required';
			else if (statuses.every((s) => s === 'passed')) out[card.key] = 'passed';
			else out[card.key] = 'pending';
		}
		return out;
	});

	function summaryOf(key: keyof Checks, check: Check): string {
		switch (key) {
			case 'plagiarism':
				if (check.status === 'passed') return `Kemiripan ${check.percent ?? '?'}%`;
				if (check.status === 'failed') return `Kemiripan ${check.percent ?? '?'}% (di atas ambang)`;
				if (check.status === 'review_required') return `Perlu review, kemiripan ${check.percent ?? '?'}%`;
				return DEFAULT_DESC;
			case 'contentSafety':
				if (check.status === 'passed') return `Risiko ${check.riskLevel ?? 'rendah'}`;
				if (check.status === 'failed') return `Diblokir: ${check.categories?.join(', ') ?? 'konten ilegal'}`;
				if (check.status === 'review_required') return `Perlu review (risiko ${check.riskLevel ?? '?'})`;
				return DEFAULT_DESC;
			case 'linkHealth': {
				const n = check.unsafeUrls?.length ?? 0;
				if (check.status === 'passed') return 'Semua link aman';
				if (check.status === 'failed') return `${n} URL tidak aman`;
				if (check.status === 'review_required') return `${n} URL butuh review`;
				return DEFAULT_DESC;
			}
			case 'brandVoice':
				if (check.status === 'passed' && check.level === 'minor_drift') return tr('Penyimpangan kecil dari suara brand');
				if (check.status === 'passed') return 'Konsisten dengan suara brand';
				if (check.status === 'failed') return tr('Melenceng dari suara brand');
				return DEFAULT_DESC;
		}
	}

	const statusTone = (s: CheckStatus) =>
		s === 'running'
			? 'warning'
			: s === 'passed'
				? 'success'
				: s === 'failed'
					? 'destructive'
					: s === 'review_required'
						? 'warning'
						: 'muted';
	const statusLabel = (s: CheckStatus) =>
		s === 'running' ? 'Memindai…' : s === 'passed' ? 'Lulus' : s === 'failed' ? tr('Gagal') : s === 'review_required' ? 'Perlu Review' : 'Pending';
	const badgeStyle = (tone: string) =>
		tone === 'success'
			? 'background-color: var(--success-soft); color: var(--success);'
			: tone === 'warning'
				? 'background-color: var(--warning-soft); color: var(--warning);'
				: tone === 'destructive'
					? 'background-color: var(--destructive-foreground, #fdecee); color: var(--destructive);'
					: 'background-color: var(--muted); color: var(--muted-foreground);';
</script>

<PageTitle title="Taut Pintar — The Gatekeeper" />

<main>
	<div class="flex flex-wrap items-center justify-between gap-3">
		<div class="flex items-center gap-3">
			<div class="flex h-10 w-10 items-center justify-center rounded-xl" style="background-color: var(--primary);">
				<ShieldCheck size={22} style="color: #fff;" />
			</div>
			<div>
				<p class="font-bold leading-tight">Agent 4: The Gatekeeper</p>
				<p class="text-sm leading-tight text-muted-foreground">Quality Control & Pembayaran Distribusi</p>
			</div>
		</div>
		<div class="flex flex-wrap items-center justify-end gap-3">
			<div class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium">
				<Wallet size={13} /><span>{fmtIdr(wallet)} {tr("Kredit")}</span>
			</div>
		</div>
	</div>

	<div class="mt-3">
		<AgentSteps current={4} lockedSteps={manualFlow ? [1] : undefined} lockedReason={MANUAL_PROFILER_REASON} />
	</div>

	{#if agentStatus && !agentStatus.configured}
		<div class="app-card mt-3 flex items-start gap-2 px-4 py-2.5 text-sm" style="color: var(--warning);" role="alert">
			<TriangleAlert size={16} class="mt-0.5 shrink-0" />
			<span>{tr("AI Gatekeeper belum dikonfigurasi oleh admin. Audit tidak akan berjalan sampai admin mengaktifkannya.")}</span>
		</div>
	{/if}

	{#if error}
		<div class="app-card mt-3 flex items-start gap-2 px-4 py-2.5 text-sm" style="color: var(--destructive);">
			<TriangleAlert size={16} class="mt-0.5 shrink-0" />
			<span class="flex-1">{error}</span>
			<button class="rounded p-0.5 hover:bg-secondary" type="button" aria-label={tr("Tutup error")} onclick={() => (error = '')}><X size={12} /></button>
		</div>
	{/if}

	{#if topup}
		<section class="app-card mt-3 p-4">
			<div class="flex items-center gap-2">
				<TriangleAlert size={16} style="color: var(--warning);" />
				<p class="text-sm font-bold">{tr("Saldo Kredit Tidak Cukup")}</p>
			</div>
			<p class="mt-2 text-sm text-muted-foreground">
				Distribusi butuh <strong>{fmtIdr(topup.needed)} {tr("Kredit")}</strong>, saldo tersedia <strong>{fmtIdr(topup.available)} {tr("Kredit")}</strong>. Silakan top up terlebih dahulu.
			</p>
			<div class="mt-3 flex gap-3">
				<a class="app-btn app-btn-primary" href={`/klien/${brandUrl}/credits`}>{tr("Top Up Kredit")}</a>
				<button class="app-btn app-btn-soft" type="button" onclick={() => (topup = null)}>{tr("Tutup")}</button>
			</div>
		</section>
	{/if}

	<section class="app-card mt-3 flex items-start gap-3 p-4" aria-label="Kenapa butuh Audit Gatekeeper">
		<div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg" style="background-color: var(--accent); color: var(--accent-foreground);">
			<ShieldCheck size={18} />
		</div>
		<div>
			<p class="font-semibold">Kenapa butuh Audit Gatekeeper?</p>
			<p class="mt-0.5 text-sm text-muted-foreground">{tr("Sistem mendeteksi adanya perubahan manual pada artikel di Agent 3. Gatekeeper harus memindai ulang setiap artikel secara individual untuk mencegah penyisipan konten ilegal (Judi, SARA) yang dapat merugikan reputasi media mitra.")}</p>
		</div>
	</section>

	{#if loading}
		<div class="mt-6 flex items-center gap-2 text-sm text-muted-foreground"><LoaderCircle size={18} class="animate-spin" />{tr("Memuat The Gatekeeper…")}</div>
	{:else}
		<section class="app-card mt-3 p-4" aria-label={tr("Sistem Deep Scan dan QC")}>
			<div class="mb-3 flex flex-wrap items-center justify-between gap-3">
				<div>
					<p class="font-semibold">Sistem Deep Scan & QC</p>
					<p class="mt-1 text-sm text-muted-foreground">{tr("Empat pengecek dijalankan per media. Illegal Content & Link Health adalah gerbang wajib.")}</p>
				</div>
				<button class="app-btn app-btn-primary" type="button" disabled={auditing || (data?.media.length ?? 0) === 0} onclick={runAudit}>
					{#if auditing}<LoaderCircle size={16} class="animate-spin" />{:else}<ShieldCheck size={16} />{/if}
					{auditing ? 'Memindai…' : 'Jalankan Audit Individual'}
				</button>
			</div>

			<div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
				{#each CHECK_CARDS as card (card.key)}
					{@const status = cardStatus[card.key]}
					{@const match = data?.media.find((m) => m.audit.checks[card.key].status === status)}
					<div class="rounded-xl border p-3" style="border-color: {status === 'passed' ? 'var(--success)' : 'var(--border)'};">
						<div class="flex items-center gap-2">
							<div class="flex h-7 w-7 items-center justify-center rounded-lg" style="background-color: var(--muted);">
								{#if card.key === 'plagiarism'}<span class="text-xs font-bold">¶</span>
								{:else if card.key === 'contentSafety'}<ShieldAlert size={16} />
								{:else if card.key === 'linkHealth'}<span class="text-xs font-bold">⛓</span>
								{:else}<span class="text-xs font-bold">~</span>{/if}
							</div>
							<p class="text-sm font-semibold">{card.title}</p>
							{#if card.gate}<span class="rounded-full px-2 py-0.5 text-[10px] font-semibold" style={badgeStyle('warning')}>Hard Gate</span>{/if}
						</div>
						<p class="mt-2 text-xs text-muted-foreground">{status !== 'pending' && match ? summaryOf(card.key, match.audit.checks[card.key]) : DEFAULT_DESC}</p>
						<div class="mt-2">
							<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style={badgeStyle(statusTone(status))}>
								{#if status === 'running'}<LoaderCircle size={11} class="animate-spin" />{/if}
								{statusLabel(status)}
							</span>
						</div>
					</div>
				{/each}
			</div>
		</section>

		<section class="app-card mt-3 p-4" aria-label={tr("Matriks media")}>
			<p class="font-semibold">{tr("Matriks Media Terpilih (")}{data?.media.length ?? 0})</p>
			{#if (data?.media.length ?? 0) === 0}
				<div class="mt-3 rounded-lg border px-4 py-8 text-center text-sm text-muted-foreground">{tr("Belum ada media untuk diaudit. Kembali ke Matchmaker untuk memilih media.")}<div class="mt-3">
						<a class="app-btn app-btn-soft" href={`/klien/${brandUrl}/taut-pintar/${campaignId}/matchmaker`}>{tr("Kembali ke Matchmaker")}</a>
					</div>
				</div>
			{:else}
				<div class="mt-3 overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="text-left text-[11px] uppercase tracking-wide text-muted-foreground">
								<th class="px-3 py-2 font-semibold">{tr("Media")}</th>
								<th class="px-3 py-2 font-semibold">Tipe</th>
								<th class="px-3 py-2 font-semibold">{tr("Status Audit")}</th>
								<th class="px-3 py-2 font-semibold">{tr("Kredit")}</th>
								<th class="px-3 py-2 font-semibold">Otoritas</th>
								<th class="px-3 py-2 font-semibold">Est. Trafik</th>
								<th class="px-3 py-2 font-semibold">Link Capacity</th>
								<th class="px-3 py-2 font-semibold">{tr("Jadwal Distribusi")}</th>
								<th class="px-3 py-2 font-semibold"></th>
							</tr>
						</thead>
						<tbody>
							{#each data?.media ?? [] as m (m.media_id)}
								<tr class="border-t">
									<td class="px-3 py-2">
										<p class="font-medium">{m.name}</p>
										<p class="text-xs text-muted-foreground">
											{m.article.title ? `“${m.article.title.slice(0, 60)}${m.article.title.length > 60 ? '…' : ''}”` : tr('Belum ada artikel')}
										</p>
									</td>
									<td class="px-3 py-2">
										<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style={m.media_type === 'blogger' ? 'background-color: var(--accent); color: var(--accent-foreground);' : 'background-color: var(--muted); color: var(--muted-foreground);'}>
											{m.media_type === 'blogger' ? 'BLOGGER' : 'MAINSTREAM'}
										</span>
									</td>
									<td class="px-3 py-2">
										<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style={badgeStyle(statusTone(m.audit.status))}>
											{#if m.audit.status === 'running'}<LoaderCircle size={11} class="animate-spin" />{/if}
											{statusLabel(m.audit.status)}
										</span>
									</td>
									<td class="px-3 py-2 font-semibold">{Math.round(m.price_idr / Math.max(1, data?.rateIdr ?? 1000))} {tr("Kredit")}</td>
									<td class="px-3 py-2">
										<p class="font-semibold">DA {m.da ?? '—'}</p>
										<p class="text-xs text-muted-foreground">{m.ss != null ? `SS ${m.ss}%` : 'SS —'}</p>
									</td>
									<td class="px-3 py-2">{fmtTrafficFull(m.est_traffic)} / Bln</td>
									<td class="px-3 py-2">{m.link_capacity != null && m.link_capacity > 0 ? `Maks. ${m.link_capacity} Link` : '—'}</td>
									<td class="px-3 py-2">
										<button
											class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs"
											type="button"
											onclick={(e) => openSchedule(m, e.currentTarget as HTMLElement)}
											aria-label={`Atur jadwal untuk ${m.name}`}
										>
											<Clock size={12} />
											<span>{toJakartaDisplay(scheduleByMedia[m.media_id] ?? null) ?? tr('Sekarang')}</span>
										</button>
									</td>
									<td class="px-3 py-2">
										<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={() => void openMockup(m)}>Tinjau Artikel</button>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</section>

		<Tooltip.Provider delayDuration={0} skipDelayDuration={300}>
			<TautBottomBar>
				<!-- Kiri: Status & Kredit ringkas sejajar -->
				<div class="flex flex-wrap items-center gap-4 sm:gap-6">
					<div>
						<p class="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">{tr('Status Kampanye')}</p>
						<div class="flex items-center gap-1.5 font-bold" style="color: {data?.canDistribute ? 'var(--success)' : 'var(--warning)'};">
							<span>{data?.canDistribute ? tr('Siap Distribusi') : tr('Terkunci (Butuh QC)')}</span>
							<Tooltip.Root>
								<Tooltip.Trigger>
									{#snippet child({ props })}
										<button
											{...props}
											type="button"
											class="inline-flex cursor-pointer text-muted-foreground transition-colors hover:text-foreground"
											aria-label={tr('Keterangan status')}
										>
											<CircleAlert size={14} />
										</button>
									{/snippet}
								</Tooltip.Trigger>
								<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={6}>
									<Tooltip.Arrow class="uiTipArrow" />
									{data?.canDistribute
										? tr('Semua hard gate lulus (Illegal Content + Link Health).')
										: tr('Illegal Content & Link Health harus Lulus untuk semua media.')}
								</Tooltip.Content>
							</Tooltip.Root>
						</div>
					</div>

					<div class="hidden h-7 w-px bg-border sm:block" aria-hidden="true"></div>

					<div>
						<p class="flex items-center gap-1 text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
							{tr('Kredit Total')} <Wallet size={11} />
						</p>
						<div class="flex items-center gap-1.5 text-base font-bold">
							<span>{fmtIdr(data?.totalCostTc ?? 0)} {tr("Kredit")}</span>
							<Tooltip.Root>
								<Tooltip.Trigger>
									{#snippet child({ props })}
										<button
											{...props}
											type="button"
											class="inline-flex cursor-pointer text-muted-foreground transition-colors hover:text-foreground"
											aria-label={tr('Keterangan nilai rupiah')}
										>
											<CircleAlert size={14} />
										</button>
									{/snippet}
								</Tooltip.Trigger>
								<Tooltip.Content class="uiTip" strategy="fixed" sideOffset={6}>
									<Tooltip.Arrow class="uiTipArrow" />
									{tr('Setara Rp')} {fmtIdr(data?.totalCostIdr ?? 0)}
								</Tooltip.Content>
							</Tooltip.Root>
						</div>
					</div>
				</div>

				<!-- Kanan: Aksi distribusi (sejajar) -->
				<div class="flex flex-wrap items-center gap-2 sm:gap-3">
					<div class="flex items-center gap-1.5" role="radiogroup" aria-label={tr('Mode distribusi')}>
						<button
							class={publishMode === 'now' ? 'app-btn app-btn-primary !min-h-9 !px-3 !text-xs' : 'app-btn app-btn-soft !min-h-9 !px-3 !text-xs'}
							type="button"
							aria-pressed={publishMode === 'now'}
							onclick={() => (publishModeChoice = 'now')}
						>{tr('Terbitkan Langsung')}</button>
						<button
							class={publishMode === 'scheduled' ? 'app-btn app-btn-primary !min-h-9 !px-3 !text-xs' : 'app-btn app-btn-soft !min-h-9 !px-3 !text-xs'}
							type="button"
							aria-pressed={publishMode === 'scheduled'}
							onclick={() => (publishModeChoice = 'scheduled')}
						>{tr('Jadwalkan')}</button>
					</div>
					{#if blockingError}<p class="text-xs" style="color: var(--destructive);">{blockingError}</p>{/if}
					<a class="app-btn app-btn-soft !min-h-9 !px-3 !text-xs" href={backUrl}>{tr("Kembali (Revisi)")}</a>
					<button class="app-btn app-btn-primary !min-h-9 !px-4 !text-xs" type="button" disabled={!data?.canDistribute || distributing || Boolean(blockingError)} onclick={distribute}>
						{#if distributing}<LoaderCircle size={15} class="animate-spin" />{:else}<ShieldCheck size={15} />{/if}
						{distributing ? tr('Memproses…') : tr('Bayar & Distribusikan')}
					</button>
				</div>
			</TautBottomBar>
		</Tooltip.Provider>
	{/if}
</main>

{#if scheduleTarget}
	{@const m = scheduleTarget}
	<!-- Latar transparan: klik di luar menutup dropdown. -->
	<button class="fixed inset-0 z-30 cursor-default" type="button" aria-label={tr('Tutup jadwal')} onclick={() => (scheduleTarget = null)}></button>
	<div
		id="schedule-dropdown"
		class="fixed z-40 rounded-xl border p-3.5 shadow-xl"
		style="left: {schedulePos.left}px; top: {schedulePos.top}px; width: {SCHEDULE_PANEL_WIDTH}px; border-color: var(--border); background-color: var(--card, #fff);"
		role="dialog"
		aria-label={tr('Atur Jadwal Distribusi')}
	>
		<div class="flex items-start justify-between gap-2">
			<div class="min-w-0">
				<p class="text-xs font-bold">{tr('Atur Jadwal Distribusi')}</p>
				<p class="mt-0.5 truncate text-[11px] text-muted-foreground">{m.name}</p>
			</div>
			<button class="rounded-md p-1 hover:bg-secondary" type="button" aria-label={tr('Tutup')} onclick={() => (scheduleTarget = null)}>
				<X size={14} />
			</button>
		</div>

		<label class="mt-3 block text-[11px] font-semibold uppercase tracking-wide text-muted-foreground" for="schedule-datetime">
			{tr('Tanggal & Waktu')}
		</label>
		<input
			id="schedule-datetime"
			type="datetime-local"
			class="app-input mt-1.5 w-full !text-xs"
			aria-label={`Jadwal distribusi ${m.name}`}
			value={scheduleByMedia[m.media_id] ? toLocalInput(scheduleByMedia[m.media_id] as string) : ''}
			onchange={(e) => {
				const v = (e.currentTarget as HTMLInputElement).value;
				setSchedule(m.media_id, v ? new Date(v).toISOString() : null);
			}}
		/>

		<div class="mt-2.5 flex flex-wrap gap-1.5">
			<button
				class={!scheduleByMedia[m.media_id] ? 'app-btn app-btn-primary !min-h-7 !px-2 !text-xs' : 'app-btn app-btn-soft !min-h-7 !px-2 !text-xs'}
				type="button"
				onclick={() => setSchedule(m.media_id, null)}
			><Clock size={11} /> {tr('Sekarang')}</button>
			{#each [['+1 Jam', 60], ['+24 Jam', 1440], ['+3 Hari', 4320]] as [label, mins] (label)}
				<button
					class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs"
					type="button"
					onclick={() => setSchedule(m.media_id, new Date(Date.now() + (mins as number) * 60000).toISOString())}
				>{label}</button>
			{/each}
		</div>

		<p class="mt-2.5 text-[11px] leading-relaxed text-muted-foreground">
			{tr('Dipakai pada mode Jadwalkan. Kosong = terbit segera setelah pembayaran.')}
		</p>
	</div>
{/if}



{#if preview}
	<div class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="dialog" aria-modal="true" aria-label={`Mockup artikel ${preview.mediaName}`}>
		<div class="app-card my-auto w-full max-w-2xl p-5">
			<div class="flex items-start justify-between gap-3">
				<div class="min-w-0">
					<p class="break-words font-bold">{preview.title || 'Tanpa judul'}</p>
					<p class="mt-1 text-xs text-muted-foreground">
						{preview.mediaName}{preview.updatedAt ? ` · Diperbarui ${dt(preview.updatedAt)}` : ''}
					</p>
				</div>
				<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label={tr("Tutup mockup")} onclick={() => (preview = null)}><X size={16} /></button>
			</div>
			<img
				class="mt-3 max-h-72 w-full rounded-lg object-cover"
				src={preview.cover ?? '/images/article-fallback.png'}
				alt={`Ilustrasi artikel ${preview.title || preview.mediaName}`}
			/>
			{#if preview.category || preview.description}
				<div class="mt-3">
					{#if preview.category}<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{preview.category}</span>{/if}
					{#if preview.description}<p class="mt-1 text-xs text-muted-foreground">{preview.description}</p>{/if}
				</div>
			{/if}
			<div class="prose prose-sm mt-3 max-h-[50vh] overflow-y-auto text-sm">{@html previewHtml}</div>
			<div class="mt-3 flex items-center justify-between border-t pt-3">
				<p class="text-xs text-muted-foreground">
					{preview.stats.words.toLocaleString('id-ID')} kata ·{' '}
					{preview.stats.imageCount > 0 ? `${preview.stats.imageCount} gambar` : 'tanpa gambar'}
				</p>
				<button class="app-btn app-btn-soft" type="button" onclick={() => (preview = null)}>{tr("Tutup")}</button>
			</div>
		</div>
	</div>
{/if}
