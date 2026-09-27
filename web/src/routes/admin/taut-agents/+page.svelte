<script lang="ts">
	// Port app/admin/taut-agents/page.tsx — konfigurasi per-agent LLM Taut Pintar
	// (base_url, api key, model, system prompt, rate limit) + preferensi pengecek
	// Gatekeeper + aktivitas audit + log/training AI. Hard stop: agent yang belum
	// dikonfigurasi membuat wizard klien berhenti (pipeline tidak fallback).
	import { onMount } from 'svelte';
	import { Bot, CheckCircle2, RefreshCw, ShieldCheck, TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import AgentCard from './agent-card.svelte';
	import AgentLogsPanel from './agent-logs-panel.svelte';
	import { apiRequest, readApiError, authErrorMessage, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';

	type AgentConfig = {
		agent: string;
		base_url: string | null;
		model: string | null;
		temperature: number | null;
		max_tokens: number | null;
		timeout_ms: number | null;
		system_prompt: string | null;
		api_key_configured: boolean;
		enabled: boolean;
		rate_limit_per_min: number;
		updated_at: string | null;
		configured: boolean;
	};

	const AGENT_META: Record<string, { label: string; role: string; hint: string }> = {
		profiler: { label: 'Profiler', role: 'Strategis', hint: 'Menerjemahkan profil bisnis + anggaran menjadi strategy_brief.' },
		matchmaker: { label: 'Matchmaker', role: 'Placement', hint: 'AI ranking media berdasarkan brief Profiler — wajib dikonfigurasi, tanpa fallback formula.' },
		writer: { label: 'Writer', role: 'Kreatif', hint: 'Satu artikel unik per media sesuai trigger.' },
		gatekeeper: { label: 'Gatekeeper', role: 'Editor & Compliance', hint: 'Tanam hyperlink natural + editorial + pre-check; hard gate tetap moderateContent.' }
	};

	const GATEKEEPER_CATEGORIES = [
		'sexual_content', 'child_safety', 'gambling', 'fraud_scam', 'violence', 'extortion',
		'hate_speech', 'violent_extremism', 'harmful_propaganda', 'dangerous_activity',
		'illegal_activity_facilitation', 'human_exploitation', 'self_harm', 'other_illegal_content'
	];

	let tab = $state<'agents' | 'gatekeeper' | 'logs'>('agents');
	let loading = $state(true);
	let error = $state('');
	let agents = $state<AgentConfig[]>([]);

	let logs = $state<any[]>([]);
	let logsLoading = $state(false);

	let gk = $state<any>(null);
	let gkNotice = $state('');
	let gkError = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([loadAgents(), loadGatekeeper(), loadLogs()]);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function loadAgents() {
		const res = await apiRequest('/v1/platform/taut-agents');
		if (res.ok) agents = (await res.json()).agents ?? [];
		else error = await readApiError(res, 'Gagal memuat daftar agent.');
	}

	async function loadGatekeeper() {
		const res = await apiRequest('/v1/platform/taut-agents/gatekeeper');
		if (!res.ok) return;
		const data = await res.json();
		const raw = data.settings ?? {};
		const base = {
			plagiarism: { enabled: true, language: 'id', threshold_warning: 10, threshold_fail: 30, max_chars: 8000 },
			contentSafety: {
				enabled: true,
				active_categories: ['sexual_content', 'child_safety', 'gambling', 'fraud_scam', 'violence', 'hate_speech'],
				block_levels: ['high'],
				review_levels: ['medium']
			},
			linkHealth: { enabled: true, ai_classify: true },
			brandVoice: { enabled: true, ai_assist: true }
		};
		gk = {
			plagiarism: { ...base.plagiarism, ...(raw.plagiarism ?? {}) },
			contentSafety: { ...base.contentSafety, ...(raw.contentSafety ?? {}) },
			linkHealth: { ...base.linkHealth, ...(raw.linkHealth ?? {}) },
			brandVoice: { ...base.brandVoice, ...(raw.brandVoice ?? {}) }
		};
	}

	async function loadLogs() {
		logsLoading = true;
		const res = await apiRequest('/v1/platform/gatekeeper/logs?pageSize=20');
		if (res.ok) logs = (await res.json()).logs ?? [];
		logsLoading = false;
	}

	async function saveGatekeeper() {
		gkNotice = '';
		gkError = '';
		const res = await apiRequest('/v1/platform/taut-agents/gatekeeper', {
			method: 'PUT',
			body: JSON.stringify({ settings: gk })
		});
		if (!res.ok) {
			gkError = await readApiError(res, 'Gagal menyimpan pengaturan Gatekeeper.');
			return;
		}
		gkNotice = 'Pengaturan Gatekeeper tersimpan.';
	}

	async function toggleCategory(key: string) {
		const list: string[] = gk.contentSafety.active_categories;
		gk.contentSafety.active_categories = list.includes(key) ? list.filter((c) => c !== key) : [...list, key];
	}

	async function saveAgent(agent: string, form: Record<string, unknown>) {
		const res = await apiRequest(`/v1/platform/taut-agents/${agent}`, { method: 'PUT', body: JSON.stringify(form) });
		if (!res.ok) throw new Error(await readApiError(res, 'Gagal menyimpan agent.'));
		await loadAgents();
	}

	async function testAgent(agent: string) {
		const res = await apiRequest(`/v1/platform/taut-agents/${agent}/test`, { method: 'POST' });
		const data = await res.json().catch(() => null);
		if (!res.ok) throw new Error(data?.detail ?? 'Uji agent gagal.');
		return data as { ok: boolean; message: string; latencyMs?: number };
	}

	const verdictLabel = (v: string) =>
		v === 'failed' ? 'Gagal' : v === 'review_required' ? 'Review' : 'Lulus';
	const verdictTone = (v: string) =>
		v === 'failed' ? 'var(--destructive)' : v === 'review_required' ? 'var(--warning)' : 'var(--success)';
</script>

<section class="mx-auto max-w-6xl">
	<PageHeader
		title="Taut Pintar — Agent"
		description="Per-agent LLM (base_url, api key, model, system prompt). Hanya super admin."
	/>

	<Feedback {loading} {error} />

	<div class="app-card mt-4 px-4 py-3 text-sm" style="border-color: var(--primary);">
		<strong>Hard stop:</strong> bila agent belum dikonfigurasi/disabled, wizard Klien berhenti dengan pesan
		<code>Agent [X] belum dikonfigurasi — hubungi super admin</code>. Pipeline tidak fallback.
	</div>

	<div class="mt-4 flex gap-2">
		{#each [['agents', 'Agent'], ['gatekeeper', 'Pengecekan Gatekeeper'], ['logs', 'Log & Training AI']] as [value, label] (value)}
			<button
				class="app-btn !min-h-8 !px-3 !text-xs"
				class:app-btn-primary={tab === value}
				class:app-btn-soft={tab !== value}
				onclick={() => (tab = value as 'agents' | 'gatekeeper' | 'logs')}
			>{label}</button>
		{/each}
	</div>

	{#if !loading}
		{#if tab === 'agents'}
			<div class="mt-4 flex flex-col gap-3">
				{#each agents as cfg (cfg.agent)}
					{@const meta = AGENT_META[cfg.agent] ?? { label: cfg.agent, role: cfg.agent, hint: '' }}
					<AgentCard config={cfg} {meta} onSave={saveAgent} onTest={testAgent} />
				{/each}
			</div>
		{:else if tab === 'gatekeeper'}
			<div class="mt-4 flex flex-col gap-4">
				{#if gk}
					<div class="app-card p-4">
						<div class="flex items-center gap-2">
							<ShieldCheck size={18} />
							<h2 class="text-sm font-semibold">Gatekeeper Settings</h2>
						</div>
						<Feedback error={gkError} notice={gkNotice} />

						<div class="mt-3 grid gap-3 sm:grid-cols-2">
							<label class="flex items-center gap-2 text-sm">
								<input type="checkbox" bind:checked={gk.plagiarism.enabled} />
								Cek plagiarisme
							</label>
							<label class="block">
								<span class="text-xs font-medium text-muted-foreground">Bahasa</span>
								<input class="app-input mt-1 w-full" bind:value={gk.plagiarism.language} />
							</label>
							<label class="block">
								<span class="text-xs font-medium text-muted-foreground">Ambang peringatan (%)</span>
								<input class="app-input mt-1 w-full" type="number" bind:value={gk.plagiarism.threshold_warning} />
							</label>
							<label class="block">
								<span class="text-xs font-medium text-muted-foreground">Ambang gagal (%)</span>
								<input class="app-input mt-1 w-full" type="number" bind:value={gk.plagiarism.threshold_fail} />
							</label>
							<label class="block">
								<span class="text-xs font-medium text-muted-foreground">Maks karakter</span>
								<input class="app-input mt-1 w-full" type="number" bind:value={gk.plagiarism.max_chars} />
							</label>
						</div>

						<div class="mt-4">
							<p class="text-xs font-medium text-muted-foreground">Kategori keamanan konten aktif</p>
							<div class="mt-1.5 flex flex-wrap gap-1.5">
								{#each GATEKEEPER_CATEGORIES as cat (cat)}
									<button
										type="button"
										class="app-btn !min-h-7 !px-2.5 !text-[11px]"
										class:app-btn-primary={gk.contentSafety.active_categories.includes(cat)}
										class:app-btn-soft={!gk.contentSafety.active_categories.includes(cat)}
										onclick={() => toggleCategory(cat)}
									>{cat}</button>
								{/each}
							</div>
						</div>

						<div class="mt-4 grid gap-3 sm:grid-cols-2">
							<label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={gk.contentSafety.enabled} /> Keamanan konten</label>
							<label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={gk.linkHealth.enabled} /> Kesehatan tautan</label>
							<label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={gk.linkHealth.ai_classify} /> Klasifikasi tautan AI</label>
							<label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={gk.brandVoice.enabled} /> Brand voice</label>
							<label class="flex items-center gap-2 text-sm"><input type="checkbox" bind:checked={gk.brandVoice.ai_assist} /> Bantuan AI brand voice</label>
						</div>

						<div class="mt-4 flex justify-end">
							<button class="app-btn app-btn-primary" onclick={saveGatekeeper}>Simpan Pengaturan</button>
						</div>
					</div>
				{/if}

				<div class="app-card overflow-x-auto p-4">
					<div class="flex items-center justify-between gap-2">
						<h2 class="text-sm font-semibold">Aktivitas Gatekeeper (20 terakhir)</h2>
						<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" disabled={logsLoading} onclick={loadLogs}>
							<RefreshCw size={12} /> {logsLoading ? 'Memuat…' : 'Refresh'}
						</button>
					</div>
					<table class="mt-3 w-full text-sm">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="py-1.5 pr-3 font-medium">Waktu</th>
								<th class="py-1.5 pr-3 font-medium">Kampanye</th>
								<th class="py-1.5 pr-3 font-medium">Media</th>
								<th class="py-1.5 pr-3 font-medium">Verdict</th>
								<th class="py-1.5 font-medium">Status</th>
							</tr>
						</thead>
						<tbody>
							{#each logs as l (l.id)}
								<tr class="border-t">
									<td class="py-1.5 pr-3 text-xs">{fmtDateTime(l.createdAt)}</td>
									<td class="max-w-[220px] break-words py-1.5 pr-3">{l.campaignName ?? l.campaignId}</td>
									<td class="max-w-[180px] break-words py-1.5 pr-3">{l.mediaName ?? l.mediaId}</td>
									<td class="py-1.5 pr-3">
										<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--accent); color: {verdictTone(l.overallVerdict)};">
											{verdictLabel(l.overallVerdict)}
										</span>
									</td>
									<td class="py-1.5 text-xs text-muted-foreground">{l.status}</td>
								</tr>
							{/each}
							{#if logs.length === 0}
								<tr><td class="py-6 text-center text-sm text-muted-foreground" colspan="5">Belum ada audit Gatekeeper.</td></tr>
							{/if}
						</tbody>
					</table>
				</div>
			</div>
		{:else}
			<AgentLogsPanel />
		{/if}
	{/if}
</section>
