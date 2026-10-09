<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

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
		writer_mainstream: { label: 'Media Mainstream', role: 'Subagent The Writer', hint: 'Artikel gaya berita 5W+1H tanpa heading — untuk media mainstream.' },
		writer_blogger: { label: 'Blogger', role: 'Subagent The Writer', hint: 'Artikel dengan subjudul H2/H3 + daftar — untuk media blogger/niche.' },
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

	// Wrapper The Writer: satu kartu dengan tab per subagent (Blogger/Media
	// Mainstream). Agent `writer` generik tetap ada di DB sebagai cadangan mesin
	// writer, tapi tidak ditampilkan karena sudah diwakili dua subagent ini.
	let writerTab = $state<'writer_blogger' | 'writer_mainstream'>('writer_blogger');
	const writerCard = $derived(agents.find((a) => a.agent === writerTab) ?? null);
	const WRITER_TABS: Array<{ agent: 'writer_blogger' | 'writer_mainstream'; label: string }> = [
		{ agent: 'writer_blogger', label: 'Blogger' },
		{ agent: 'writer_mainstream', label: 'Media Mainstream' }
	];

	// Prompt per jenis artikel (editorial/advertorial/opini) untuk The Writer.
	// Base URL / API key / model tetap milik agent (dipakai bersama); yang berbeda
	// hanya system prompt-nya. Dikelola di sini karena menyatu dengan kartu agent.
	type ArticleType = {
		id: string;
		code: string;
		name: string;
		active: boolean;
		active_version?: number | null;
		active_version_id?: string | null;
	};
	let articleTypes = $state<ArticleType[]>([]);
	let atTab = $state(''); // code jenis artikel terpilih
	let atPrompt = $state('');
	let atVersionId = $state<string | null>(null);
	let atBusy = $state(false);
	let atMsg = $state('');
	let atMsgOk = $state(false);
	const atTypeName = $derived(articleTypes.find((t) => t.code === atTab)?.name ?? '');

	async function loadArticleTypes() {
		const res = await apiRequest('/v1/platform/taut/article-types');
		if (!res.ok) return;
		articleTypes = (await res.json()).items ?? [];
		const first = articleTypes.find((t) => t.code === 'editorial') ?? articleTypes[0] ?? null;
		if (first) await selectArticleTypeTab(first.code);
	}

	async function selectArticleTypeTab(code: string) {
		atTab = code;
		atMsg = '';
		const type = articleTypes.find((t) => t.code === code);
		if (!type) return;
		const res = await apiRequest(`/v1/platform/taut/article-types/${type.id}/versions`);
		if (!res.ok) return;
		const versions = (await res.json()).items ?? [];
		const active = versions.find((v: { status: string }) => v.status === 'active') ?? versions[0] ?? null;
		if (!active) {
			atVersionId = null;
			atPrompt = '';
			return;
		}
		atVersionId = active.id;
		const detail = await apiRequest(`/v1/platform/taut/article-types/${type.id}/versions/${active.id}`);
		atPrompt = detail.ok ? ((await detail.json()).version?.prompt ?? '') : '';
	}

	async function saveArticleTypePrompt() {
		const type = articleTypes.find((t) => t.code === atTab);
		if (!type || !atVersionId) return;
		atBusy = true;
		atMsg = '';
		try {
			const res = await apiRequest(`/v1/platform/taut/article-types/${type.id}/versions/${atVersionId}`, {
				method: 'PATCH',
				body: JSON.stringify({ prompt: atPrompt })
			});
			if (!res.ok) {
				atMsg = await readApiError(res, tr('Gagal menyimpan prompt.'));
				atMsgOk = false;
				return;
			}
			atMsg = tr('Prompt tersimpan.');
			atMsgOk = true;
		} finally {
			atBusy = false;
		}
	}

	let gk = $state<any>(null);
	let gkNotice = $state('');
	let gkError = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([loadAgents(), loadGatekeeper(), loadLogs(), loadArticleTypes()]);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function loadAgents() {
		const res = await apiRequest('/v1/platform/taut-agents');
		if (res.ok) agents = (await res.json()).agents ?? [];
		else error = await readApiError(res, tr('Gagal memuat daftar agent.'));
	}

	async function loadGatekeeper() {
		const res = await apiRequest('/v1/platform/taut-agents/gatekeeper');
		if (!res.ok) return;
		const data = await res.json();
		const raw = data.settings ?? {};
		const base = {
			aiOverview: { enabled: true, pass_total: 32, pass_min_indicator: 7, max_chars: 8000 },
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
			aiOverview: { ...base.aiOverview, ...(raw.aiOverview ?? {}) },
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
			gkError = await readApiError(res, tr('Gagal menyimpan pengaturan Gatekeeper.'));
			return;
		}
		gkNotice = tr('Pengaturan Gatekeeper tersimpan.');
	}

	async function toggleCategory(key: string) {
		const list: string[] = gk.contentSafety.active_categories;
		gk.contentSafety.active_categories = list.includes(key) ? list.filter((c) => c !== key) : [...list, key];
	}

	async function saveAgent(agent: string, form: Record<string, unknown>) {
		const res = await apiRequest(`/v1/platform/taut-agents/${agent}`, { method: 'PUT', body: JSON.stringify(form) });
		if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan agent.')));
		await loadAgents();
	}

	async function testAgent(agent: string) {
		const res = await apiRequest(`/v1/platform/taut-agents/${agent}/test`, { method: 'POST' });
		const data = await res.json().catch(() => null);
		if (!res.ok) throw new Error(data?.detail ?? 'Uji agent gagal.');
		return data as { ok: boolean; message: string; latencyMs?: number };
	}

	const verdictLabel = (v: string) =>
		v === 'failed' ? tr('Gagal') : v === 'review_required' ? 'Review' : 'Lulus';
	const verdictTone = (v: string) =>
		v === 'failed' ? 'var(--destructive)' : v === 'review_required' ? 'var(--warning)' : 'var(--success)';
</script>

<section>
	<PageHeader
		title="Taut Pintar — Agent"
		description="Per-agent LLM (base_url, api key, model, system prompt). Hanya super admin. Kecepatan generate mengikuti model: model opus-class ±70 token/detik (artikel 20-35 detik), model flash-class (mis. via OpenRouter, base URL https://openrouter.ai/api/v1) bisa 5x lebih cepat. Perubahan langsung aktif tanpa restart."
	/>

	<Feedback {loading} {error} />

	<div class="app-card mt-4 px-4 py-3 text-sm" style="border-color: var(--primary);">
		<strong>Hard stop:</strong>{tr("bila agent belum dikonfigurasi/disabled, wizard Klien berhenti dengan pesan")}<code>Agent [X] belum dikonfigurasi — hubungi super admin</code>. Pipeline tidak fallback.
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
				<!-- Wrapper The Writer: dua subagent (Blogger & Media Mainstream) dalam
				     satu kartu bertab, supaya tidak menumpuk tiga kartu writer. -->
				<section class="app-card p-4">
					<div class="flex flex-wrap items-start justify-between gap-3">
						<div>
							<p class="text-sm font-semibold">The Writer</p>
							<p class="mt-0.5 text-xs text-muted-foreground">
								{tr("Dua subagent — koneksi (base URL, API key, model) bisa berbeda per tipe media. Media Blogger memakai System Prompt-nya sendiri; Media Mainstream memakai prompt per jenis artikel (editorial/advertorial/opini) di bawah.")}
							</p>
						</div>
						<div class="flex gap-1 rounded-lg p-1" style="background-color: var(--muted);" role="tablist" aria-label={tr("Subagent The Writer")}>
							{#each WRITER_TABS as t (t.agent)}
								<button
									class="app-btn !min-h-8 !px-3 !text-xs"
									class:app-btn-primary={writerTab === t.agent}
									class:app-btn-soft={writerTab !== t.agent}
									type="button"
									role="tab"
									aria-selected={writerTab === t.agent}
									onclick={() => (writerTab = t.agent)}
								>{t.label}</button>
							{/each}
						</div>
					</div>
					{#if writerCard}
						<div class="mt-3">
							<AgentCard
								bare
								hideSystemPrompt={writerTab === 'writer_mainstream'}
								config={writerCard}
								meta={AGENT_META[writerTab] ?? { label: 'The Writer', role: 'Kreatif', hint: '' }}
								onSave={saveAgent}
								onTest={testAgent}
							/>

							{#if writerTab === 'writer_mainstream'}
								<!-- Prompt per jenis artikel: base_url/api_key/model di atas
								     dipakai bersama; yang berbeda hanya system prompt.
								     Hanya untuk The Writer media MAINSTREAM — media blogger
								     terisolasi dengan system prompt-nya sendiri di kartu atas. -->
								<div class="mt-4 border-t pt-4" style="border-color: var(--border);">
								<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{tr("Prompt per Jenis Artikel")}</p>
								<p class="mt-1 text-[11px] text-muted-foreground">
									{tr("Base URL, API key, dan model di atas dipakai bersama untuk semua jenis artikel. Yang berbeda hanya system prompt (knowledge)-nya — dipilih klien lewat toggle di modal Pengelola Naskah. Berlaku untuk The Writer di semua tipe media; kosongkan untuk memakai prompt dasar.")}
								</p>
								{#if articleTypes.length > 0}
									<div class="mt-2 flex flex-wrap gap-1.5" role="tablist" aria-label={tr("Jenis artikel")}>
										{#each articleTypes as t (t.id)}
											<button
												type="button"
												role="tab"
												aria-selected={atTab === t.code}
												class="app-btn !min-h-8 !px-3 !text-xs"
												class:app-btn-primary={atTab === t.code}
												class:app-btn-soft={atTab !== t.code}
												onclick={() => selectArticleTypeTab(t.code)}
											>{t.name}</button>
										{/each}
									</div>
									{#if atVersionId}
										<label class="mt-3 block">
											<span class="text-xs font-medium text-muted-foreground">{tr("System Prompt (knowledge)")} · {atTypeName}</span>
											<textarea
												class="app-input mt-1 w-full font-mono text-xs"
												rows="10"
												placeholder={tr("Tulis persona & pedoman gaya jenis artikel ini…")}
												bind:value={atPrompt}
											></textarea>
										</label>
										<div class="mt-2 flex items-center gap-2">
											<button type="button" class="app-btn app-btn-primary" disabled={atBusy} onclick={saveArticleTypePrompt}>
												{atBusy ? tr('Menyimpan…') : tr('Simpan Prompt')}
											</button>
											{#if atMsg}
												<span class="text-sm" style="color: {atMsgOk ? 'var(--success)' : 'var(--destructive)'};">{atMsg}</span>
											{/if}
										</div>
									{:else}
										<p class="mt-3 text-xs text-muted-foreground">{tr("Jenis artikel ini belum punya versi prompt aktif.")}</p>
									{/if}
								{:else}
									<p class="mt-2 text-xs text-muted-foreground">{tr("Belum ada jenis artikel.")}</p>
								{/if}
							</div>
							{/if}
						</div>
					{:else}
						<p class="mt-3 text-xs text-muted-foreground">{tr("Konfigurasi subagent belum termuat.")}</p>
					{/if}
				</section>

				{#each agents as cfg (cfg.agent)}
					{#if !cfg.agent.startsWith('writer')}
						{@const meta = AGENT_META[cfg.agent] ?? { label: cfg.agent, role: cfg.agent, hint: '' }}
						<AgentCard config={cfg} {meta} onSave={saveAgent} onTest={testAgent} />
					{/if}
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
								<input type="checkbox" bind:checked={gk.aiOverview.enabled} />
								Cek AI Overview (kesiapan artikel untuk Google AI Overviews)
							</label>
							<label class="block">
								<span class="text-xs font-medium text-muted-foreground">Ambang total lulus (0-40)</span>
								<input class="app-input mt-1 w-full" type="number" min="0" max="40" bind:value={gk.aiOverview.pass_total} />
							</label>
							<label class="block">
								<span class="text-xs font-medium text-muted-foreground">Skor minimum per indikator (0-10)</span>
								<input class="app-input mt-1 w-full" type="number" min="0" max="10" bind:value={gk.aiOverview.pass_min_indicator} />
							</label>
							<label class="block">
								<span class="text-xs font-medium text-muted-foreground">Maks karakter isi</span>
								<input class="app-input mt-1 w-full" type="number" bind:value={gk.aiOverview.max_chars} />
							</label>
						</div>
						<p class="mt-2 text-[11px] text-muted-foreground">
							Lulus bila total skor &ge; ambang total DAN tidak ada indikator di bawah skor minimum. Non-blocking: skor rendah hanya memunculkan catatan revisi, tidak menghalangi distribusi. Artikel buatan AI Tauthub dinilai ulang sekali setelah pass revisi.
						</p>

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
							<button class="app-btn app-btn-primary" onclick={saveGatekeeper}>{tr("Simpan Pengaturan")}</button>
						</div>
					</div>
				{/if}

				<div class="app-card overflow-x-auto p-4">
					<div class="flex items-center justify-between gap-2">
						<h2 class="text-sm font-semibold">Aktivitas Gatekeeper (20 terakhir)</h2>
						<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" disabled={logsLoading} onclick={loadLogs}>
							<RefreshCw size={12} /> {logsLoading ? tr('Memuat…') : 'Refresh'}
						</button>
					</div>
					<table class="mt-3 w-full text-sm">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="py-1.5 pr-3 font-medium">Waktu</th>
								<th class="py-1.5 pr-3 font-medium">{tr("Kampanye")}</th>
								<th class="py-1.5 pr-3 font-medium">{tr("Media")}</th>
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
								<tr><td class="py-6 text-center text-sm text-muted-foreground" colspan="5">{tr("Belum ada audit Gatekeeper.")}</td></tr>
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
