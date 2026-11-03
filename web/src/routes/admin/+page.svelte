<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Dashboard admin — control plane: status operasional, feed publikasi media,
	// dan aktivitas agen AI. Data: GET /v1/platform/operations/summary.
	import { onMount } from 'svelte';
	import {
		ArrowRight, RotateCw, Globe, ShieldCheck, Users,
		CircleCheck, TriangleAlert, Zap, ExternalLink, ChevronLeft
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

	const publishedPct = $derived(
		eco && eco.total_publications > 0 ? (eco.published_publications / eco.total_publications) * 100 : 0
	);

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
										href={pub.published_url}
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

		<!-- Angka ekosistem -->
		<div class="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
			<div class="app-card p-5">
				<p class="flex items-center gap-2 text-xs text-muted-foreground"><Users size={14} strokeWidth={1.5} /> {tr('Pengguna')}</p>
				<p class="mt-3 text-[28px] font-medium leading-none tabular-nums tracking-tight">{eco ? fmtNum(eco.total_users) : '—'}</p>
				<p class="mt-2 text-xs text-muted-foreground">
					{eco ? `${fmtNum(eco.partner_users)} ${tr('mitra')} · ${fmtNum(eco.client_users)} ${tr('klien')}` : ''}
				</p>
			</div>
			<div class="app-card p-5">
				<p class="flex items-center gap-2 text-xs text-muted-foreground"><Globe size={14} strokeWidth={1.5} /> {tr('Katalog media')}</p>
				<p class="mt-3 text-[28px] font-medium leading-none tabular-nums tracking-tight">{eco ? fmtNum(eco.total_media) : '—'}</p>
				<p class="mt-2 text-xs text-muted-foreground">
					{eco ? `${fmtNum(eco.active_media)} ${tr('aktif')} · ${fmtNum(eco.total_media - eco.active_media)} ${tr('draft atau non-aktif')}` : ''}
				</p>
			</div>
			<div class="app-card p-5">
				<p class="flex items-center gap-2 text-xs text-muted-foreground"><ShieldCheck size={14} strokeWidth={1.5} /> {tr('Publikasi')}</p>
				<p class="mt-3 text-[28px] font-medium leading-none tabular-nums tracking-tight">{eco ? fmtNum(eco.total_publications) : '—'}</p>
				<div class="mt-3 flex items-center gap-2">
					<span class="meter" aria-hidden="true"><span style="width: {publishedPct}%"></span></span>
					<span class="shrink-0 whitespace-nowrap text-xs tabular-nums text-muted-foreground">
						{eco ? `${fmtNum(eco.published_publications)}/${fmtNum(eco.total_publications)} ${tr('terbit')}` : ''}
					</span>
				</div>
			</div>
			<div class="app-card p-5">
				<p class="flex items-center gap-2 text-xs text-muted-foreground"><Zap size={14} strokeWidth={1.5} /> {tr('Taut Agents')}</p>
				<p class="mt-3 text-[28px] font-medium leading-none tabular-nums tracking-tight">{eco ? fmtNum(eco.total_agent_runs) : '—'}</p>
				<p class="mt-2 text-xs text-muted-foreground">
					{eco ? `${fmtNum(eco.agent_runs_done)} ${tr('selesai')} · gatekeeper ${fmtNum(eco.gatekeeper_passed)}/${fmtNum(eco.total_gatekeeper)}` : ''}
				</p>
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
					<ol class="mt-4">
						{#each recentAgents as run (run.id)}
							<li class="run-row">
								<div class="min-w-0 flex-1">
									<p class="truncate text-[13px] font-medium">{tierLabel(run.agent)}</p>
									<p class="truncate text-xs text-muted-foreground">{run.campaign_name || tr('Kampanye')}</p>
								</div>
								<span class="run-meta">{shortModel(run.model)}</span>
								<span class="run-meta tabular-nums">{run.duration_ms ? `${Math.round(run.duration_ms / 1000)}s` : '—'}</span>
								<span class="run-status" data-tone={run.status === 'done' ? 'ok' : run.status === 'failed' ? 'danger' : 'warn'}>
									<span class="status-dot" data-tone={run.status === 'done' ? 'ok' : run.status === 'failed' ? 'danger' : 'warn'} aria-hidden="true"></span>
									{run.status === 'done' ? tr('Selesai') : run.status === 'failed' ? tr('Gagal') : run.status}
								</span>
							</li>
						{/each}
					</ol>
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

				<dl class="spec-list mt-4">
					<div class="spec-row">
						<dt>{tr('Batch berjalan')}</dt>
						<dd class="tabular-nums">{fmtNum(summary?.running_batches ?? 0)}</dd>
					</div>
					<div class="spec-row">
						<dt>{tr('Dead letter')}</dt>
						<dd>
							<span class="status-dot" data-tone={summary?.dead_letter_items ? 'danger' : 'ok'} aria-hidden="true"></span>
							{summary?.dead_letter_items ? `${fmtNum(summary.dead_letter_items)} ${tr('perlu aksi')}` : tr('Bersih')}
						</dd>
					</div>
					<div class="spec-row">
						<dt>{tr('Scheduler')}</dt>
						<dd>
							<span class="status-dot" data-tone="ok" aria-hidden="true"></span>
							{tr('Aktif')}
						</dd>
					</div>
				</dl>

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
	.status-dot[data-tone='danger'] { background-color: var(--destructive); }

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

	/* ── Run agen: satu baris, kolom sejajar ── */
	.run-row {
		display: flex;
		align-items: center;
		gap: 12px;
		min-height: 48px;
	}
	.run-row + .run-row { border-top: 1px solid color-mix(in srgb, var(--foreground) 6%, transparent); }
	.run-meta {
		flex: 0 0 auto;
		font-family: var(--font-mono);
		font-size: 11px;
		line-height: 16px;
		color: var(--muted-foreground);
		white-space: nowrap;
	}
	.run-byline { display: none; }
	.run-status {
		flex: 0 0 auto;
		display: inline-flex;
		align-items: center;
		font-size: 12px;
		color: var(--muted-foreground);
	}
	.run-status[data-tone='danger'] { color: var(--destructive); }
	.run-status[data-tone='warn'] { color: var(--warning); }

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
	.meter {
		position: relative;
		display: block;
		flex: 1;
		height: 4px;
		border-radius: 999px;
		background-color: color-mix(in srgb, var(--foreground) 8%, transparent);
		overflow: hidden;
	}
	.meter > span {
		position: absolute;
		inset: 0 auto 0 0;
		border-radius: 999px;
		background-color: var(--primary);
	}

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
