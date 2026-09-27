<script lang="ts">
	// Dashboard admin — port app/admin/page.tsx (Executive Overview):
	// KPI ekosistem, feed publikasi terkini, aktivitas agen AI, kesehatan
	// AI engine, queue worker, dan pintasan navigasi.
	import { onMount } from 'svelte';
	import {
		ArrowRight, RotateCw, Cpu, Globe, Newspaper, ShieldCheck, Users,
		CircleCheck, TriangleAlert, Zap
	} from '@lucide/svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime, fmtNum } from '$lib/admin-format';
	import { platformError, isAccessDenied } from './_platform';

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
		if (res.ok) summary = await res.json();
		else error = await platformError(res);
		if (isManual) refreshing = false;
	}

	const eco = $derived(summary?.ecosystem);
	const recentPubs = $derived(summary?.recent_publications ?? []);
	const recentAgents = $derived(summary?.recent_agent_runs ?? []);
	const denied = $derived(!loaded ? false : Boolean(error && !summary && isAccessDenied(error)));

	const tierLabel = (agent: string) => (agent ? agent.charAt(0).toUpperCase() + agent.slice(1) : agent);
	const shortModel = (model: string | null) => (model ? model.split('/').pop() : 'default');
</script>

<section>
	<header class="flex flex-wrap items-end justify-between gap-4 border-b pb-5">
		<div>
			<p class="text-xs font-bold tracking-wide text-muted-foreground">TAUTHUB PLATFORM CONTROL</p>
			<h1 class="mt-1 text-2xl font-semibold tracking-tight">Executive Overview</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				Status operasional real-time, aktivitas publikasi media, dan performa orkestrasi AI.
			</p>
		</div>
		<div class="flex items-center gap-3">
			<button
				class="rounded-lg border p-2 hover:bg-secondary disabled:opacity-50"
				type="button"
				disabled={refreshing}
				title="Muat ulang data real-time"
				aria-label="Muat ulang data real-time"
				onclick={() => void load(true)}
			>
				<RotateCw size={16} />
			</button>
			<span class="rounded-full px-2.5 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">
				Admin Console
			</span>
		</div>
	</header>

	{#if !loaded}
		<p class="mt-6 text-sm text-muted-foreground">Memuat platform overview…</p>
	{:else if denied}
		<div class="app-card mt-4 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke control plane platform.</p>
		</div>
	{:else}
		{#if error}
			<div class="app-card mt-4 flex items-start gap-2 px-4 py-3 text-sm" style="color: var(--destructive);">
				<TriangleAlert size={18} class="mt-0.5 shrink-0" />
				<span>{error}</span>
			</div>
		{/if}
		{#if summary?.generation_disabled}
			<div class="app-card mt-3 flex items-start gap-2 px-4 py-3 text-sm" style="color: var(--destructive);">
				<TriangleAlert size={18} class="mt-0.5 shrink-0" />
				<span><strong>Safety Circuit Active:</strong> Generation global sedang dihentikan oleh safety control. Semua agen AI diblokir sementara.</span>
			</div>
		{/if}
		{#each summary?.alerts ?? [] as alert (alert)}
			<div class="app-card mt-3 flex items-start gap-2 px-4 py-3 text-sm" style="color: var(--warning);">
				<TriangleAlert size={18} class="mt-0.5 shrink-0" />
				<span>{alert}</span>
			</div>
		{/each}

		<!-- KPI Utama -->
		{#if eco}
			<div class="mt-5 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
				<div class="app-card flex min-h-[124px] flex-col p-5">
					<div class="flex items-center justify-between">
						<span class="flex items-center gap-2 text-sm font-medium text-muted-foreground"><Users size={18} /> Pengguna</span>
						<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">
							{fmtNum(eco.partner_users)} Mitra
						</span>
					</div>
					<p class="mt-2.5 text-3xl font-bold tracking-tight">{fmtNum(eco.total_users)}</p>
					<div class="mt-auto flex items-center justify-between pt-2">
						<span class="text-xs text-muted-foreground">{fmtNum(eco.client_users)} Klien terdaftar</span>
						<a class="text-xs" style="color: var(--primary);" href="/admin/users">Lihat →</a>
					</div>
				</div>

				<div class="app-card flex min-h-[124px] flex-col p-5">
					<div class="flex items-center justify-between">
						<span class="flex items-center gap-2 text-sm font-medium text-muted-foreground"><Globe size={18} /> Katalog Media</span>
						<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--success-soft); color: var(--success);">
							{fmtNum(eco.active_media)} Aktif
						</span>
					</div>
					<p class="mt-2.5 text-3xl font-bold tracking-tight">{fmtNum(eco.total_media)}</p>
					<div class="mt-auto flex items-center justify-between pt-2">
						<span class="text-xs text-muted-foreground">{fmtNum(eco.total_media - eco.active_media)} Draft / Non-aktif</span>
						<a class="text-xs" style="color: var(--primary);" href="/admin/media">Kelola →</a>
					</div>
				</div>

				<div class="app-card flex min-h-[124px] flex-col p-5">
					<div class="flex items-center justify-between">
						<span class="flex items-center gap-2 text-sm font-medium text-muted-foreground"><Newspaper size={18} /> Total Publikasi</span>
						<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">
							100% Live
						</span>
					</div>
					<p class="mt-2.5 text-3xl font-bold tracking-tight">{fmtNum(eco.total_publications)}</p>
					<div class="mt-auto flex items-center justify-between pt-2">
						<span class="text-xs text-muted-foreground">{fmtNum(eco.published_publications)} Terbit di media partner</span>
						<a class="text-xs" style="color: var(--primary);" href="/admin/publications">Riwayat →</a>
					</div>
				</div>

				<div class="app-card flex min-h-[124px] flex-col p-5">
					<div class="flex items-center justify-between">
						<span class="flex items-center gap-2 text-sm font-medium text-muted-foreground"><ShieldCheck size={18} /> Taut Agents Run</span>
						<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">
							{fmtNum(eco.agent_runs_done)} Sukses
						</span>
					</div>
					<p class="mt-2.5 text-3xl font-bold tracking-tight">{fmtNum(eco.total_agent_runs)}</p>
					<div class="mt-auto flex items-center justify-between pt-2">
						<span class="text-xs text-muted-foreground">Gatekeeper pass: {fmtNum(eco.gatekeeper_passed)}/{fmtNum(eco.total_gatekeeper)}</span>
						<a class="text-xs" style="color: var(--primary);" href="/admin/taut-agents">Monitor →</a>
					</div>
				</div>
			</div>
		{:else}
			<div class="mt-5 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
				{#each [0, 1, 2, 3] as i (i)}
					<div class="app-card min-h-[120px] animate-pulse" style="background-color: var(--secondary);"></div>
				{/each}
			</div>
		{/if}

		<!-- Feed publikasi & aktivitas agen -->
		<div class="mt-5 grid gap-4 lg:grid-cols-2">
			<div class="app-card p-5">
				<div class="flex items-center justify-between">
					<span class="flex items-center gap-2">
						<Newspaper size={20} class="text-muted-foreground" />
						<h2 class="text-base font-semibold">Publikasi Media Terkini</h2>
					</span>
					<a class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" href="/admin/publications">Lihat Semua</a>
				</div>
				<p class="mt-2 text-sm text-muted-foreground">
					Rilis artikel terbaru yang berhasil didistribusikan ke jaringan media rekanan.
				</p>

				{#if recentPubs.length > 0}
					<div class="mt-3 overflow-x-auto">
						<table class="w-full text-sm">
							<thead>
								<tr class="text-left text-xs text-muted-foreground">
									<th class="py-2 pr-3 font-medium">Media / Brand</th>
									<th class="py-2 pr-3 font-medium">Status</th>
									<th class="py-2 text-right font-medium">Tautan</th>
								</tr>
							</thead>
							<tbody>
								{#each recentPubs as pub (pub.id)}
									<tr class="border-t">
										<td class="py-2 pr-3">
											<p class="font-medium">{pub.media_name || 'Media'}</p>
											<p class="text-xs text-muted-foreground">
												{pub.brand_name || 'Brand'} • {pub.published_at ? fmtDateTime(pub.published_at) : 'Baru'}
											</p>
										</td>
										<td class="py-2 pr-3">
											<span
												class="rounded-full px-2 py-0.5 text-[11px] font-medium"
												style="background-color: {pub.status === 'published' ? 'var(--success-soft)' : 'var(--accent)'}; color: {pub.status === 'published' ? 'var(--success)' : 'var(--accent-foreground)'};"
											>
												{pub.status === 'published' ? 'Live' : pub.status}
											</span>
										</td>
										<td class="py-2 text-right">
											{#if pub.published_url}
												<a class="text-xs" style="color: var(--primary);" href={pub.published_url} target="_blank" rel="noopener noreferrer">Buka URL ↗</a>
											{:else}
												<span class="text-xs text-muted-foreground">—</span>
											{/if}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{:else}
					<div class="mt-3 rounded-lg border border-dashed p-6 text-center">
						<p class="text-sm text-muted-foreground">Belum ada publikasi artikel terbaru yang tercatat.</p>
					</div>
				{/if}
			</div>

			<div class="app-card p-5">
				<div class="flex items-center justify-between">
					<span class="flex items-center gap-2">
						<Cpu size={20} class="text-muted-foreground" />
						<h2 class="text-base font-semibold">Aktivitas Taut AI &amp; Gateway</h2>
					</span>
					<a class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" href="/admin/taut-agents">Detail Run</a>
				</div>
				<p class="mt-2 text-sm text-muted-foreground">
					Riwayat pemanggilan agen pencocokan, sudut pandang, dan verifikasi otomatis.
				</p>

				{#if recentAgents.length > 0}
					<div class="mt-3 overflow-x-auto">
						<table class="w-full text-sm">
							<thead>
								<tr class="text-left text-xs text-muted-foreground">
									<th class="py-2 pr-3 font-medium">Agent / Kampanye</th>
									<th class="py-2 pr-3 font-medium">Model</th>
									<th class="py-2 pr-3 font-medium">Durasi</th>
									<th class="py-2 text-right font-medium">Status</th>
								</tr>
							</thead>
							<tbody>
								{#each recentAgents as run (run.id)}
									<tr class="border-t">
										<td class="py-2 pr-3">
											<p class="font-medium capitalize">{tierLabel(run.agent)}</p>
											<p class="text-xs text-muted-foreground">{run.campaign_name || 'Kampanye'}</p>
										</td>
										<td class="py-2 pr-3 text-xs text-muted-foreground">{shortModel(run.model)}</td>
										<td class="py-2 pr-3 text-xs text-muted-foreground">{run.duration_ms ? `${Math.round(run.duration_ms / 1000)}s` : '—'}</td>
										<td class="py-2 text-right">
											<span
												class="rounded-full px-2 py-0.5 text-[11px] font-medium"
												style="background-color: {run.status === 'done' ? 'var(--success-soft)' : run.status === 'failed' ? 'var(--destructive-soft, #fdecee)' : 'var(--warning-soft)'}; color: {run.status === 'done' ? 'var(--success)' : run.status === 'failed' ? 'var(--destructive)' : 'var(--warning)'};"
											>
												{run.status === 'done' ? 'Selesai' : run.status === 'failed' ? 'Gagal' : run.status}
											</span>
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{:else}
					<div class="mt-3 rounded-lg border border-dashed p-6 text-center">
						<p class="text-sm text-muted-foreground">Belum ada aktivitas agen AI terbaru.</p>
					</div>
				{/if}
			</div>
		</div>

		<!-- Diagnostik & pintasan -->
		<div class="mt-5 grid gap-4 md:grid-cols-3">
			<div class="app-card flex flex-col p-5">
				<span class="flex items-center gap-2">
					<Zap size={20} class="text-muted-foreground" />
					<h3 class="text-base font-semibold">Kesehatan AI Engine</h3>
				</span>
				<p class="mt-2 text-sm text-muted-foreground">Infrastruktur provider LLM dan throughput panggilan 24 jam.</p>

				<div class="mt-3 flex flex-col">
					<div class="flex items-center justify-between border-b py-1.5 text-sm">
						<span class="text-muted-foreground">Provider Utama</span>
						<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: {summary?.provider_configured ? 'var(--success-soft)' : 'var(--warning-soft)'}; color: {summary?.provider_configured ? 'var(--success)' : 'var(--warning)'};">
							{summary?.provider || 'openai-compatible'}
						</span>
					</div>
					<div class="flex items-center justify-between border-b py-1.5 text-sm">
						<span class="text-muted-foreground">Circuit Breaker</span>
						<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--success-soft); color: var(--success);">Normal ({summary?.circuit || 'closed'})</span>
					</div>
					<div class="flex items-center justify-between border-b py-1.5 text-sm">
						<span class="text-muted-foreground">Panggilan 24 Jam</span>
						<span class="font-medium">
							{fmtNum(summary?.calls_24h ?? 0)}
							{#if summary?.failed_calls_24h}
								<span class="ml-1 text-xs" style="color: var(--destructive);">({fmtNum(summary.failed_calls_24h)} gagal)</span>
							{:else}
								<span class="ml-1 text-xs" style="color: var(--success);">(100% OK)</span>
							{/if}
						</span>
					</div>
					<div class="flex items-center justify-between py-1.5 text-sm">
						<span class="text-muted-foreground">Token 30 Hari</span>
						<span class="font-medium">
							{((summary?.tokens_30d ?? 0) / 1000).toLocaleString('id-ID', { maximumFractionDigits: 1 })}k
						</span>
					</div>
				</div>

				<a class="app-btn app-btn-soft mt-4 w-full" href="/admin/ai">
					Buka AI Gateway <ArrowRight size={14} />
				</a>
			</div>

			<div class="app-card flex flex-col p-5">
				<span class="flex items-center gap-2">
					<CircleCheck size={20} class="text-muted-foreground" />
					<h3 class="text-base font-semibold">Background Worker &amp; Queue</h3>
				</span>
				<p class="mt-2 text-sm text-muted-foreground">Pemantauan antrean tugas, batch konten, dan dead letter alert.</p>

				<div class="mt-3 flex flex-col">
					<div class="flex items-center justify-between border-b py-1.5 text-sm">
						<span class="text-muted-foreground">Running Batches</span>
						<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{summary?.running_batches ?? 0} batch aktif</span>
					</div>
					<div class="flex items-center justify-between border-b py-1.5 text-sm">
						<span class="text-muted-foreground">Dead Letters</span>
						<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: {summary?.dead_letter_items ? 'var(--destructive-soft, #fdecee)' : 'var(--success-soft)'}; color: {summary?.dead_letter_items ? 'var(--destructive)' : 'var(--success)'};">
							{summary?.dead_letter_items ? `${summary.dead_letter_items} perlu aksi` : '0 (Bersih)'}
						</span>
					</div>
					<div class="flex items-center justify-between border-b py-1.5 text-sm">
						<span class="text-muted-foreground">Status Scheduler</span>
						<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--success-soft); color: var(--success);">Aktif (Cron Worker)</span>
					</div>
					<div class="flex items-center justify-between py-1.5 text-sm">
						<span class="text-muted-foreground">Audit Log</span>
						<a class="text-xs" style="color: var(--primary);" href="/admin/audit-log">Buka Log →</a>
					</div>
				</div>

				<a class="app-btn app-btn-soft mt-4 w-full" href="/admin/operations">
					Kelola Antrean <ArrowRight size={14} />
				</a>
			</div>

			<div class="app-card flex flex-col p-5">
				<span class="flex items-center gap-2">
					<Globe size={20} class="text-muted-foreground" />
					<h3 class="text-base font-semibold">Pusat Navigasi Penting</h3>
				</span>
				<p class="mt-2 text-sm text-muted-foreground">Pintasan cepat ke modul konfigurasi dan manajemen operasional utama.</p>

				<div class="mt-3 flex flex-col gap-2">
					{#each [
						{ href: '/admin/revenue-shares', label: 'Markup Harga & Bagi Hasil' },
						{ href: '/admin/konfigurasi', label: 'Pengaturan Platform & Integrasi' },
						{ href: '/admin/media-summaries', label: 'Kesimpulan & Crawler Media' },
						{ href: '/admin/taut-pintar', label: 'Kampanye Taut Pintar' }
					] as link (link.href)}
						<a
							class="flex items-center justify-between rounded-lg border px-3 py-2.5 text-sm font-medium transition-colors hover:bg-secondary"
							style="border-color: var(--border);"
							href={link.href}
						>
							<span>{link.label}</span>
							<ArrowRight size={14} class="text-muted-foreground" />
						</a>
					{/each}
				</div>
			</div>
		</div>
	{/if}
</section>
