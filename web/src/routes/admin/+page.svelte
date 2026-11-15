<script lang="ts">
	import { safeHref } from '$lib/safe-url';
	import { tr } from '$lib/i18n/phrases';

	// Dashboard admin — control plane: status operasional, feed publikasi media,
	// dan aktivitas agen AI. Data: GET /v1/platform/operations/summary.
	import { onMount } from 'svelte';
	import {
		ArrowRight, RotateCw,
		CircleCheck, TriangleAlert, ExternalLink, ChevronLeft
	} from '@lucide/svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime, fmtNum } from '$lib/admin-format';
	import { platformError, isAccessDenied } from './_platform';
	import PageTitle from '$lib/components/page-title.svelte';

	type EcosystemStats = {
		total_users: number; client_users: number; partner_users: number;
		total_brands: number; total_media: number; active_media: number;
		total_publications: number; published_publications: number;
		total_agent_runs: number; agent_runs_done: number;
		gatekeeper_passed: number; total_gatekeeper: number;
	};
	type RecentPublication = {
		id: string; media_name: string | null; brand_name: string | null; status: string;
		publish_status: string | null; published_url: string | null; published_at: string | null;
	};
	type RecentAgentRun = {
		id: string; agent: string; status: string; model: string | null;
		duration_ms: number | null; created_at: string; campaign_name: string | null;
	};
	type Summary = {
		generation_disabled: boolean; provider: string; provider_configured: boolean;
		circuit: string; calls_24h: number; failed_calls_24h: number; tokens_30d: number;
		running_batches: number; dead_letter_items: number; alerts: string[];
		ecosystem?: EcosystemStats;
		recent_publications?: RecentPublication[];
		recent_agent_runs?: RecentAgentRun[];
	};

	let summary = $state<Summary | null>(null);
	let error = $state('');
	let loaded = $state(false);
	let refreshing = $state(false);
	// Waktu muat terakhir — dibaca klien saja, jadi tidak memicu hydration mismatch.
	let updatedAt = $state('');
	let feedLimit = $state(5);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loaded = true;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load(isManual = false) {
		if (isManual) refreshing = true;
		error = '';
		const res = await apiRequest('/v1/platform/operations/summary');
		if (res.ok) {
			summary = await res.json();
			updatedAt = new Date().toLocaleTimeString('id-ID', { timeZone: 'Asia/Jakarta', hour: '2-digit', minute: '2-digit' });
		} else {
			error = await platformError(res);
		}
		if (isManual) refreshing = false;
	}

	const eco = $derived(summary?.ecosystem);
	const recentPubs = $derived(summary?.recent_publications ?? []);
	const recentAgents = $derived(summary?.recent_agent_runs ?? []);
	const denied = $derived(!loaded ? false : Boolean(error && !summary && isAccessDenied(error)));

	// Satu slot pengumuman: safety circuit dulu, lalu alert API (bisa beberapa).
	const notices = $derived.by(() => {
		const out: Array<{ text: string; tone: 'danger' | 'warning' }> = [];
		if (summary?.generation_disabled) {
			out.push({
				text: tr('Generation global sedang dihentikan oleh safety control. Semua agen AI diblokir sementara.'),
				tone: 'danger'
			});
		}
		for (const text of summary?.alerts ?? []) out.push({ text, tone: 'warning' });
		return out;
	});

	const pct = (a: number, b: number) => (b > 0 ? Math.min(100, (a / b) * 100) : 0);
	const partnerPct = $derived(eco ? pct(eco.partner_users, eco.total_users) : 0);
	const rates = $derived(
		eco
			? [
					{ label: tr('Media aktif'), part: eco.active_media, total: eco.total_media, pct: pct(eco.active_media, eco.total_media) },
					{ label: tr('Publikasi terbit'), part: eco.published_publications, total: eco.total_publications, pct: pct(eco.published_publications, eco.total_publications) },
					{ label: tr('Run agen selesai'), part: eco.agent_runs_done, total: eco.total_agent_runs, pct: pct(eco.agent_runs_done, eco.total_agent_runs) },
					{ label: tr('Gatekeeper lolos'), part: eco.gatekeeper_passed, total: eco.total_gatekeeper, pct: pct(eco.gatekeeper_passed, eco.total_gatekeeper) }
				]
			: []
	);

	// Geometri grafik (SVG).
	const DR = 54;
	const DC = 2 * Math.PI * DR;
	const BH = 150; // tinggi area batang
	const BY = (pct: number) => 20 + BH - (pct / 100) * BH;

	const runSec = (r: RecentAgentRun) => Math.round((r.duration_ms ?? 0) / 1000);
	const runMax = $derived(Math.max(1, ...recentAgents.map(runSec)));
	const runH = $derived(Math.max(1, recentAgents.length) * 44);
	const queueRows = $derived([
		{ label: tr('Batch berjalan'), value: summary?.running_batches ?? 0, color: 'var(--primary)' },
		{ label: tr('Dead letter'), value: summary?.dead_letter_items ?? 0, color: 'var(--destructive)' }
	]);
	const queueMax = $derived(Math.max(5, ...queueRows.map((q) => q.value)));

	const tierLabel = (agent: string) => (agent ? agent.charAt(0).toUpperCase() + agent.slice(1) : agent);
	const shortModel = (model: string | null) => (model ? model.split('/').pop() : 'default');
	const hostOf = (url: string | null) => {
		if (!url) return '';
		try {
			return new URL(url).hostname.replace(/^www\./, '');
		} catch {
			return url;
		}
	};
</script>

<PageTitle title="Dasbor Admin" />

<section>
	<div class="flex items-start justify-between gap-4">
		<div>
			<h1 class="page-title">Ringkasan platform</h1>
			{#if loaded && summary}
				<p class="page-desc flex items-center gap-2">
					<span class="live-dot" aria-hidden="true"></span>
					<span>{tr('Terhubung')}</span>
					<span aria-hidden="true">·</span>
					<span>{tr('diperbarui')} {updatedAt || '—'} WIB</span>
				</p>
			{:else if !loaded}
				<p class="mt-1.5 text-xs text-muted-foreground">{tr('Memuat data platform…')}</p>
			{/if}
		</div>
		<button
			class="app-icon-btn"
			type="button"
			disabled={refreshing}
			title="Muat ulang data"
			aria-label="Muat ulang data"
			onclick={() => void load(true)}
		>
			<RotateCw size={16} strokeWidth={1.5} class={refreshing ? 'animate-spin' : ''} />
		</button>
	</div>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr('Akun ini tidak memiliki akses ke control plane platform.')}</p>
		</div>
	{:else}
		{#if error}
			<div class="notice notice-danger mt-5">
				<TriangleAlert size={16} strokeWidth={1.5} class="shrink-0" />
				<span>{error}</span>
			</div>
		{/if}
		{#each notices as notice, i (notice.text + i)}
			<div class="notice mt-5 {notice.tone === 'danger' ? 'notice-danger' : 'notice-warning'}">
				<TriangleAlert size={16} strokeWidth={1.5} class="shrink-0" />
				<span>{notice.text}</span>
				<a class="notice-action" href="/admin/operations">{tr('Buka antrean')}</a>
			</div>
		{/each}

		<!-- Angka ekosistem -->
		<div class="mt-8 grid items-stretch gap-4 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)]">
			<div class="app-card flex flex-col p-6">
				<h2 class="text-base font-medium">{tr('Pengguna')}</h2>
				<p class="mt-1 text-sm text-muted-foreground">{tr('Komposisi mitra dan klien')}</p>
				<div class="mt-6 flex flex-1 items-center gap-6">
					<svg viewBox="0 0 140 140" class="donut" role="img" aria-label="{eco ? `${fmtNum(eco.partner_users)} ${tr('mitra')}, ${fmtNum(eco.client_users)} ${tr('klien')}` : ''}">
						<circle cx="70" cy="70" r={DR} class="donut-track" />
						{#if eco && eco.total_users > 0}
							<circle
								cx="70" cy="70" r={DR} class="donut-seg"
								stroke-dasharray="{(partnerPct / 100) * DC} {DC}"
								transform="rotate(-90 70 70)"
							/>
						{/if}
						<text x="70" y="68" text-anchor="middle" class="donut-num">{eco ? fmtNum(eco.total_users) : '—'}</text>
						<text x="70" y="86" text-anchor="middle" class="donut-lbl">{tr('pengguna')}</text>
					</svg>
					{#if eco}
						<dl class="flex flex-col gap-4 text-xs">
							<div>
								<dt class="flex items-center gap-2 text-muted-foreground"><span class="key key-a" aria-hidden="true"></span>{tr('Mitra')}</dt>
								<dd class="mt-1 text-sm font-medium tabular-nums">{fmtNum(eco.partner_users)} <span class="font-normal text-muted-foreground">· {Math.round(partnerPct)}%</span></dd>
							</div>
							<div>
								<dt class="flex items-center gap-2 text-muted-foreground"><span class="key key-b" aria-hidden="true"></span>{tr('Klien')}</dt>
								<dd class="mt-1 text-sm font-medium tabular-nums">{fmtNum(eco.client_users)} <span class="font-normal text-muted-foreground">· {Math.round(100 - partnerPct)}%</span></dd>
							</div>
						</dl>
					{/if}
				</div>
			</div>

			<div class="app-card p-6">
				<h2 class="text-base font-medium">{tr('Kesehatan ekosistem')}</h2>
				<p class="mt-1 text-sm text-muted-foreground">{tr('Persentase yang sudah aktif, terbit, selesai, atau lolos')}</p>
				<svg viewBox="0 0 480 230" class="bars mt-4" role="img" aria-label={rates.map((r) => `${r.label} ${Math.round(r.pct)}%`).join(', ')}>
					{#each [0, 50, 100] as t (t)}
						<line x1="44" x2="476" y1={BY(t)} y2={BY(t)} class="grid-line" />
						<text x="38" y={BY(t) + 4} text-anchor="end" class="axis">{t}%</text>
					{/each}
					{#each rates as r, i (r.label)}
						{@const x = 64 + i * 104}
						<rect {x} y={BY(100)} width="64" height={BH} rx="6" class="bar-bg" />
						<rect {x} y={BY(r.pct)} width="64" height={(r.pct / 100) * BH} rx="6" class="bar-fg" />
						<text x={x + 32} y={BY(r.pct) + 22} text-anchor="middle" class="bar-val">{Math.round(r.pct)}%</text>
						<text x={x + 32} y="222" text-anchor="middle" class="axis">{r.label}</text>
					{/each}
				</svg>
			</div>
		</div>

		<div class="mt-8 grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
			<!-- Feed publikasi -->
			<div class="app-card flex flex-col p-6">
				<div class="flex items-start justify-between gap-4">
					<div>
						<h2 class="text-base font-medium">Publikasi terbaru</h2>
						<p class="mt-1 text-sm text-muted-foreground">
							{tr('Artikel yang sudah terbit di media rekanan')}
						</p>
					</div>
					{#if !loaded}
						<span class="shimmer-line mt-1 h-8 w-24 rounded-md"></span>
					{:else if eco && recentPubs.length > 0}
						<a class="app-link-sm" href="/admin/publications/daftar">
							{tr('Semua publikasi')}
							<ArrowRight size={14} strokeWidth={1.5} />
						</a>
					{/if}
				</div>

				{#if !loaded}
					<div class="mt-4 flex flex-col">
						{#each [0, 1, 2, 3, 4] as i (i)}
							<div class="feed-row">
								<div class="min-w-0 flex-1">
									<span class="shimmer-line h-4 w-40 rounded"></span>
									<span class="shimmer-line mt-2 h-3 w-56 rounded"></span>
								</div>
							</div>
						{/each}
					</div>
				{:else if recentPubs.length > 0}
					<ol class="mt-4">
						{#each recentPubs.slice(0, feedLimit) as pub (pub.id)}
							<li class="feed-row">
								<span class="feed-rail" aria-hidden="true"></span>
								<div class="min-w-0 flex-1">
									<p class="truncate text-sm font-medium">{pub.media_name || tr('Media')}</p>
									<p class="mt-1 text-xs text-muted-foreground">
										{pub.brand_name || tr('Brand')}
										<span aria-hidden="true"> · {pub.published_at ? fmtDateTime(pub.published_at) : tr('Baru dipublikasi')}</span>
									</p>
								</div>
								{#if pub.published_url}
									<a
										class="app-link-sm shrink-0"
										href={safeHref(pub.published_url)}
										target="_blank"
										rel="noopener noreferrer"
										title={hostOf(pub.published_url)}
									>
										{hostOf(pub.published_url)}
										<ExternalLink size={12} strokeWidth={1.5} />
									</a>
								{/if}
							</li>
						{/each}
					</ol>
					{#if recentPubs.length > 5}
						<button class="app-link-sm mt-4 self-start" type="button" onclick={() => (feedLimit = feedLimit === 5 ? recentPubs.length : 5)}>
							{#if feedLimit === 5}
								{tr('Tampilkan semua')}
								<ArrowRight size={14} strokeWidth={1.5} />
							{:else}
								<ChevronLeft size={14} strokeWidth={1.5} />
								{tr('Ringkas')}
							{/if}
						</button>
					{/if}
				{:else}
					<div class="mt-4 rounded-lg border border-dashed px-6 py-10 text-center">
						<p class="text-sm text-muted-foreground">{tr('Belum ada publikasi yang terbit.')}</p>
					</div>
				{/if}
			</div>

			<!-- Kinerja AI -->
			<div class="app-card flex flex-col p-6">
				<h2 class="text-base font-medium">Kinerja AI</h2>
				<p class="mt-1 text-sm text-muted-foreground">
					{tr('Gateway provider dan pemakaian token')}
				</p>

				<div class="mt-6">
					<p class="text-xs text-muted-foreground">{tr('Token 30 hari terakhir')}</p>
					<p class="mt-1 text-[28px] font-medium leading-none tabular-nums tracking-tight">
						{#if loaded}{fmtNum(summary?.tokens_30d ?? 0)}{:else}—{/if}
					</p>
				</div>

				<dl class="spec-list mt-6">
					<div class="spec-row">
						<dt>{tr('Panggilan AI 24 jam')}</dt>
						<dd class="tabular-nums">
							{#if loaded}
								{fmtNum(summary?.calls_24h ?? 0)}
								{#if summary?.failed_calls_24h}
									<span class="spec-note spec-danger">{fmtNum(summary.failed_calls_24h)} {tr('gagal')}</span>
								{:else}
									<span class="spec-note">0 {tr('gagal')}</span>
								{/if}
							{:else}—{/if}
						</dd>
					</div>
					<div class="spec-row">
						<dt>{tr('Provider')}</dt>
						<dd>
							<span class="status-dot" data-tone={summary?.provider_configured ? 'ok' : 'warn'} aria-hidden="true"></span>
							{summary?.provider ?? 'openai-compatible'}
						</dd>
					</div>
					<div class="spec-row">
						<dt>{tr('Circuit breaker')}</dt>
						<dd class="capitalize">{summary?.circuit ?? 'closed'}</dd>
					</div>
				</dl>

				<a class="app-link-sm mt-auto pt-6" href="/admin/ai">
					{tr('AI Gateway')}
					<ArrowRight size={14} strokeWidth={1.5} />
				</a>
			</div>
		</div>

		<div class="mt-8 grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
			<!-- Run agen -->
			<div class="app-card p-6">
				<div class="flex items-start justify-between gap-4">
					<div>
						<h2 class="text-base font-medium">{tr('Run agen AI terakhir')}</h2>
						<p class="mt-1 text-sm text-muted-foreground">{tr('Writer dan gatekeeper per kampanye')}</p>
					</div>
					<a class="app-link-sm shrink-0" href="/admin/taut-agents">
						{tr('Semua run')}
						<ArrowRight size={14} strokeWidth={1.5} />
					</a>
				</div>

				{#if !loaded}
					<div class="mt-4">
						{#each [0, 1, 2, 3] as i (i)}
							<div class="feed-row"><span class="shimmer-line h-4 w-48 rounded"></span></div>
						{/each}
					</div>
				{:else if recentAgents.length > 0}
					<svg viewBox="0 0 480 {runH}" class="bars mt-4" role="img" aria-label={recentAgents.map((r) => `${tierLabel(r.agent)} ${runSec(r)}s`).join(', ')}>
						{#each recentAgents as run, i (run.id)}
							{@const y = i * 44}
							<text x="0" y={y + 17} class="row-name">{tierLabel(run.agent).slice(0, 18)}</text>
							<text x="0" y={y + 33} class="axis">{(run.campaign_name || tr('Kampanye')).slice(0, 22)}</text>
							<rect x="160" y={y + 8} width="280" height="22" rx="5" class="bar-bg" />
							<rect x="160" y={y + 8} width={Math.max(4, (runSec(run) / runMax) * 280)} height="22" rx="5" fill={run.status === 'done' ? 'var(--primary)' : run.status === 'failed' ? 'var(--destructive)' : 'var(--warning)'}>
								<title>{shortModel(run.model)} · {run.status}</title>
							</rect>
							<text x="476" y={y + 24} text-anchor="end" class="bar-val-out">{run.duration_ms ? `${runSec(run)}s` : '—'}</text>
						{/each}
					</svg>
				{:else}
					<div class="mt-4 rounded-lg border border-dashed px-6 py-10 text-center">
						<p class="text-sm text-muted-foreground">{tr('Belum ada run agen.')}</p>
					</div>
				{/if}
			</div>

			<!-- Worker & queue -->
			<div class="app-card flex flex-col p-6">
				<div class="flex items-start justify-between gap-4">
					<div>
						<h2 class="text-base font-medium">{tr('Worker dan antrean')}</h2>
						<p class="mt-1 text-sm text-muted-foreground">{tr('Batch konten dan job yang gagal permanen')}</p>
					</div>
					<a class="app-link-sm shrink-0" href="/admin/operations">
						{tr('Kelola')}
						<ArrowRight size={14} strokeWidth={1.5} />
					</a>
				</div>

				<svg viewBox="0 0 480 120" class="bars mt-6" role="img" aria-label="{tr('Batch berjalan')} {fmtNum(summary?.running_batches ?? 0)}, {tr('Dead letter')} {fmtNum(summary?.dead_letter_items ?? 0)}">
					{#each queueRows as q, i (q.label)}
						{@const y = i * 52}
						<text x="0" y={y + 14} class="row-name">{q.label}</text>
						<rect x="0" y={y + 24} width="440" height="16" rx="5" class="bar-bg" />
						{#if q.value > 0}
							<rect x="0" y={y + 24} width={Math.max(6, (q.value / queueMax) * 440)} height="16" rx="5" fill={q.color} />
						{/if}
						<text x="476" y={y + 37} text-anchor="end" class="bar-val-out">{fmtNum(q.value)}</text>
					{/each}
				</svg>
				<p class="mt-4 flex items-center gap-2 text-xs text-muted-foreground">
					<span class="status-dot" data-tone="ok" aria-hidden="true"></span>
					{tr('Scheduler')} {tr('Aktif')}
				</p>

				<a class="app-link-sm mt-auto pt-6" href="/admin/audit-log">
					{tr('Audit log')}
					<ArrowRight size={14} strokeWidth={1.5} />
				</a>
			</div>
		</div>

		<!-- Pintasan -->
		<div class="mt-8">
			<h2 class="text-xs text-muted-foreground">{tr('Pintasan')}</h2>
			<div class="mt-2 grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
				{#each [
					{ href: '/admin/revenue-shares', label: tr('Markup harga & bagi hasil') },
					{ href: '/admin/konfigurasi', label: tr('Pengaturan platform & integrasi') },
					{ href: '/admin/media-summaries', label: tr('Kesimpulan & crawler media') },
					{ href: '/admin/taut-pintar/monitoring', label: tr('Kampanye Taut Pintar') }
				] as link (link.href)}
					<a class="shortcut" href={link.href}>
						<span class="truncate">{link.label}</span>
						<ArrowRight size={14} strokeWidth={1.5} class="shrink-0 text-muted-foreground" />
					</a>
				{/each}
			</div>
		</div>
	{/if}
</section>

<style>
	/* ── Signature: satu titik status hidup, dipakai di header dan tiap status ── */
	.live-dot {
		position: relative;
		display: inline-block;
		width: 6px;
		height: 6px;
		border-radius: 999px;
		background-color: var(--success);
	}
	.live-dot::after {
		content: '';
		position: absolute;
		inset: -3px;
		border-radius: 999px;
		border: 1px solid color-mix(in srgb, var(--success) 35%, transparent);
	}

	.status-dot {
		display: inline-block;
		width: 6px;
		height: 6px;
		border-radius: 999px;
		margin-right: 6px;
		vertical-align: 1px;
		background-color: var(--muted-foreground);
	}
	.status-dot[data-tone='ok'] { background-color: var(--success); }
	.status-dot[data-tone='warn'] { background-color: var(--warning); }

	/* ── Tombol ikon ── */
	.app-icon-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background-color: var(--card);
		color: var(--muted-foreground);
		transition: background-color 160ms cubic-bezier(0.22, 1, 0.36, 1), color 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.app-icon-btn:hover:not(:disabled) { background-color: var(--secondary); color: var(--foreground); }
	.app-icon-btn:disabled { opacity: 0.5; cursor: not-allowed; }
	.app-icon-btn:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; }

	/* ── Tautan kecil ── */
	.app-link-sm {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		color: var(--primary);
		font-size: 12px;
		font-weight: 500;
		line-height: 20px;
		white-space: nowrap;
		text-decoration: none;
	}
	.app-link-sm:hover { text-decoration: underline; text-underline-offset: 2px; }

	/* ── Pemberitahuan ── */
	.notice {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		gap: 8px;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 12px 16px;
		background-color: var(--card);
		font-size: 13px;
		line-height: 20px;
	}
	.notice-warning { color: var(--warning); }
	.notice-danger { color: var(--destructive); }
	.notice-action {
		margin-left: auto;
		color: inherit;
		font-weight: 500;
		white-space: nowrap;
		text-decoration: underline;
		text-underline-offset: 2px;
	}

	/* ── Feed publikasi: garis waktu tipis, bukan tabel ── */
	.feed-row {
		position: relative;
		display: flex;
		align-items: flex-start;
		gap: 12px;
		min-height: 56px;
		padding: 10px 0 10px 18px;
	}
	.feed-row + .feed-row { border-top: 1px solid color-mix(in srgb, var(--foreground) 6%, transparent); }
	.feed-rail {
		position: absolute;
		left: 3px;
		top: 16px;
		width: 6px;
		height: 6px;
		border-radius: 999px;
		background-color: var(--card);
		box-shadow: inset 0 0 0 1.5px var(--primary);
	}
	.feed-row::before {
		content: '';
		position: absolute;
		left: 5.5px;
		top: 0;
		bottom: 0;
		width: 1px;
		background-color: color-mix(in srgb, var(--foreground) 10%, transparent);
	}
	.feed-row:first-child::before { top: 16px; }
	.feed-row:last-child::before { bottom: calc(100% - 22px); }

	/* ── Daftar spesifikasi label/nilai ── */
	.spec-list { display: flex; flex-direction: column; }
	.spec-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		min-height: 36px;
		font-size: 13px;
	}
	.spec-row + .spec-row { border-top: 1px solid color-mix(in srgb, var(--foreground) 6%, transparent); }
	.spec-row dt { color: var(--muted-foreground); }
	.spec-row dd { margin: 0; font-weight: 500; }
	.spec-note { margin-left: 6px; font-size: 11px; color: var(--muted-foreground); }
	.spec-danger { color: var(--destructive); }

	/* ── Meter publikasi ── */
	.donut { width: 140px; height: 140px; flex: none; }
	.donut-track, .donut-seg { fill: none; stroke-width: 14; }
	.donut-track { stroke: color-mix(in srgb, var(--foreground) 14%, transparent); }
	.donut-seg { stroke: var(--primary); stroke-linecap: butt; }
	.donut-num { fill: var(--foreground); font-size: 26px; font-weight: 500; font-variant-numeric: tabular-nums; }
	.donut-lbl { fill: var(--muted-foreground); font-size: 11px; }
	.key { width: 8px; height: 8px; border-radius: 999px; }
	.key-a { background-color: var(--primary); }
	.key-b { background-color: color-mix(in srgb, var(--foreground) 14%, transparent); }
	.bars { width: 100%; height: auto; }
	.grid-line { stroke: color-mix(in srgb, var(--foreground) 10%, transparent); stroke-width: 1; }
	.axis { fill: var(--muted-foreground); font-size: 11px; }
	.bar-bg { fill: color-mix(in srgb, var(--foreground) 6%, transparent); }
	.bar-fg { fill: var(--primary); }
	.row-name { fill: var(--foreground); font-size: 13px; font-weight: 500; }
	.bar-val-out { fill: var(--muted-foreground); font-size: 12px; font-variant-numeric: tabular-nums; }
	.bar-val { fill: var(--primary-foreground); font-size: 12px; font-weight: 500; font-variant-numeric: tabular-nums; }

	/* ── Pintasan ── */
	.shortcut {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		min-height: 44px;
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 0 12px;
		background-color: var(--card);
		font-size: 13px;
		font-weight: 500;
		color: var(--foreground);
		text-decoration: none;
		transition: background-color 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.shortcut:hover { background-color: var(--secondary); }

	/* ── Kerangka muat: statis, tanpa denyut ── */
	.shimmer-line {
		display: block;
		background-color: var(--secondary);
	}

	@media (prefers-reduced-motion: no-preference) {
		section {
			animation: unslop-rise 320ms cubic-bezier(0.22, 1, 0.36, 1) both;
		}
		@keyframes unslop-rise {
			from { opacity: 0; transform: translateY(6px); }
			to { opacity: 1; transform: none; }
		}
	}
</style>
