<script lang="ts">
	// AI gateway — port app/admin/ai/page.tsx + _model-knowledge-drawer.tsx.
	// Tab: Provider (config sealed key + template), Model Guidelines & RAG,
	// Routing, Test generation terisolasi, dan Safety Controls (kill switch global).
	import { onMount } from 'svelte';
	import { Brain, CheckCircle2, Plus, Sparkles, TriangleAlert, Zap } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtNum } from '$lib/admin-format';

	type Provider = {
		provider: string;
		configured: boolean;
		circuit: 'closed' | 'open';
		models: Record<string, string | null>;
		timeout_seconds: number;
		max_retries: number;
	};
	type Usage = { summary: { calls: number; successful_calls: number; total_tokens: number } };
	type Generation = {
		content: string;
		provider: string;
		model: string;
		request_id?: string;
		input_tokens: number;
		output_tokens: number;
		total_tokens: number;
		latency_ms: number;
	};
	type Operations = {
		generation_disabled: boolean;
		calls_24h: number;
		failed_calls_24h: number;
		alerts: string[];
	};
	type ModelTier = 'default' | 'economy' | 'premium';
	type Configuration = {
		base_url: string;
		api_key_configured: boolean;
		api_key_hint?: string | null;
		source: 'database' | 'environment' | 'none';
		default_model: string;
		economy_model: string;
		premium_model: string;
	};
	type ModelKnowledge = {
		model_id?: string;
		reasoning_level?: string | null;
		summary?: string | null;
		characteristics?: string | null;
		suitable_for?: string | null;
		unsuitable_for?: string | null;
		guidelines?: string | null;
		limitations?: string | null;
		editorial_context?: string | null;
		chunk_count?: number;
	};
	type ModelWithKnowledgeInfo = {
		model_id: string;
		model_name: string;
		provider: string;
		tier?: string;
		is_active?: boolean;
		has_knowledge: boolean;
		chunk_count: number;
		reasoning_level?: string | null;
		knowledge?: ModelKnowledge | null;
	};
	type KnowledgeChunk = { id: string; section: string; chunk_index: number; content: string; priority: number };

	type ProviderTemplateKey =
		| 'openai'
		| 'google'
		| 'openrouter'
		| 'cloudflare'
		| 'groq'
		| 'ollama'
		| 'custom';

	const CF_BASE_PREFIX = 'https://api.cloudflare.com/client/v4/accounts/';
	const CF_BASE_SUFFIX = '/ai/v1';

	function buildCfUrl(accountId: string) {
		const id = accountId.trim();
		if (!id) return `${CF_BASE_PREFIX}YOUR_ACCOUNT_ID${CF_BASE_SUFFIX}`;
		return `${CF_BASE_PREFIX}${id}${CF_BASE_SUFFIX}`;
	}

	function parseCfAccountId(url: string): string {
		const m = url.match(/\/accounts\/([a-f0-9-]{20,}|[a-z0-9-]{10,})\/ai\/v1/i);
		if (!m) return '';
		const v = m[1];
		if (v === 'YOUR_ACCOUNT_ID' || v.includes('<')) return '';
		return v;
	}

	const PROVIDER_TEMPLATES: Record<
		ProviderTemplateKey,
		{
			label: string;
			base_url: string;
			default_model: string;
			economy_model: string;
			premium_model: string;
			hint: string;
			api_key_placeholder?: string;
		}
	> = {
		openai: {
			label: 'OpenAI (GPT-4o)',
			base_url: 'https://api.openai.com/v1',
			default_model: 'gpt-4o-mini',
			economy_model: 'gpt-4o-mini',
			premium_model: 'gpt-4o',
			hint: 'OpenAI API key resmi (sk-proj-...). Akses GPT-4o, GPT-4o-mini.',
			api_key_placeholder: 'sk-proj-...'
		},
		google: {
			label: 'Google AI Studio (Gemini)',
			base_url: 'https://generativelanguage.googleapis.com/v1beta/openai',
			default_model: 'gemini-2.0-flash',
			economy_model: 'gemini-1.5-flash',
			premium_model: 'gemini-1.5-pro',
			hint: 'Google AI Studio API key (AIzaSy...). Model valid: gemini-2.0-flash, gemini-1.5-flash, gemini-1.5-pro.',
			api_key_placeholder: 'AIzaSy...'
		},
		openrouter: {
			label: 'OpenRouter (Multi-Model)',
			base_url: 'https://openrouter.ai/api/v1',
			default_model: 'google/gemini-2.0-flash-exp:free',
			economy_model: 'meta-llama/llama-3.3-70b-instruct',
			premium_model: 'anthropic/claude-3.5-sonnet',
			hint: 'OpenRouter API key (sk-or-...). Akses 200+ model AI.',
			api_key_placeholder: 'sk-or-v1-...'
		},
		cloudflare: {
			label: 'Cloudflare Workers AI',
			base_url: buildCfUrl('YOUR_ACCOUNT_ID'),
			default_model: '@cf/meta/llama-3.1-8b-instruct-fast',
			economy_model: '@cf/meta/llama-3.1-8b-instruct',
			premium_model: '@cf/meta/llama-3.1-405b-instruct',
			hint: "Cloudflare API Token dengan izin 'Workers AI: Read'. Account ID dari dashboard Cloudflare.",
			api_key_placeholder: 'Cloudflare API Token...'
		},
		groq: {
			label: 'Groq Cloud (Fast Llama-3)',
			base_url: 'https://api.groq.com/openai/v1',
			default_model: 'llama-3.3-70b-versatile',
			economy_model: 'llama-3.1-8b-instant',
			premium_model: 'llama-3.3-70b-versatile',
			hint: 'Groq API key (gsk_...). Ultra-fast inference Llama-3.',
			api_key_placeholder: 'gsk_...'
		},
		ollama: {
			label: 'Ollama (Local AI)',
			base_url: 'http://localhost:11434/v1',
			default_model: 'llama3.2',
			economy_model: 'llama3.2:1b',
			premium_model: 'qwen2.5:14b',
			hint: 'Ollama lokal atau remote. API key boleh diisi sembarang string jika tanpa auth.',
			api_key_placeholder: 'ollama (bebas)'
		},
		custom: {
			label: 'Custom Base URL',
			base_url: '',
			default_model: '',
			economy_model: '',
			premium_model: '',
			hint: 'Endpoint OpenAI-compatible kustom (vLLM, LocalAI, Azure, dll).',
			api_key_placeholder: 'API Key...'
		}
	};

	type ProviderConfigState = {
		base_url: string;
		api_key: string;
		default_model: string;
		economy_model: string;
		premium_model: string;
		cf_account_id: string;
	};

	const STORAGE_KEY = 'platform_ai_provider_saved_configs';

	function emptyState(): ProviderConfigState {
		return { base_url: '', api_key: '', default_model: '', economy_model: '', premium_model: '', cf_account_id: '' };
	}

	function defaultProviderState(): Record<ProviderTemplateKey, ProviderConfigState> {
		const keys: ProviderTemplateKey[] = ['openai', 'google', 'openrouter', 'cloudflare', 'groq', 'ollama', 'custom'];
		const out = {} as Record<ProviderTemplateKey, ProviderConfigState>;
		for (const key of keys) {
			const preset = PROVIDER_TEMPLATES[key];
			out[key] = {
				base_url: preset.base_url,
				api_key: '',
				default_model: preset.default_model,
				economy_model: preset.economy_model,
				premium_model: preset.premium_model,
				cf_account_id: ''
			};
		}
		return out;
	}

	let loading = $state(true);
	let error = $state('');
	let notice = $state('');
	let busy = $state(false);

	let provider = $state<Provider | null>(null);
	let usage = $state<Usage | null>(null);
	let operations = $state<Operations | null>(null);
	let configuration = $state<Configuration | null>(null);
	let modelsList = $state<ModelWithKnowledgeInfo[]>([]);

	let tab = $state<'provider' | 'knowledge' | 'routing' | 'test' | 'safety'>('provider');

	let template = $state<ProviderTemplateKey>('openai');
	let formBaseUrl = $state('');
	let formApiKey = $state('');
	let formDefault = $state('');
	let formEconomy = $state('');
	let formPremium = $state('');
	let cfAccountId = $state('');
	let savedConfigs = $state<Record<ProviderTemplateKey, ProviderConfigState>>(defaultProviderState());

	let result = $state<Generation | null>(null);
	let testTier = $state<ModelTier>('default');
	let testPrompt = $state('');

	let knowledgeSearch = $state('');
	let customModelInput = $state('');
	let drawerTarget = $state<ModelWithKnowledgeInfo | null>(null);
	let switchOpen = $state(false);

	// Drawer knowledge state.
	let drawerLoading = $state(false);
	let drawerError = $state('');
	let drawerNotice = $state('');
	let drawerBusy = $state(false);
	let drawerChunks = $state<KnowledgeChunk[]>([]);
	let drawerExisting = $state<ModelKnowledge | null>(null);
	let kReasoning = $state('sedang');
	let kSummary = $state('');
	let kCharacteristics = $state('');
	let kSuitableFor = $state('');
	let kUnsuitableFor = $state('');
	let kGuidelines = $state('');
	let kLimitations = $state('');
	let kEditorialContext = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await loadLocalStorage();
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function loadLocalStorage() {
		try {
			const stored = localStorage.getItem(STORAGE_KEY);
			if (stored) savedConfigs = { ...defaultProviderState(), ...JSON.parse(stored) };
		} catch {
			/* localStorage tidak tersedia */
		}
	}

	function persistSavedConfigs(next: Record<ProviderTemplateKey, ProviderConfigState>) {
		savedConfigs = next;
		try {
			localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
		} catch {
			/* localStorage tidak tersedia */
		}
	}

	function updateCurrentConfig(patch: Partial<ProviderConfigState>) {
		const current = savedConfigs[template] ?? emptyState();
		persistSavedConfigs({ ...savedConfigs, [template]: { ...current, ...patch } });
	}

	async function getJson<T>(path: string): Promise<{ ok: true; data: T } | { ok: false; error: string }> {
		try {
			const res = await apiRequest(`/v1/platform${path}`);
			if (!res.ok) {
				const detail = await readApiError(res, 'Permintaan platform gagal.');
				return {
					ok: false,
					error:
						res.status === 404 && detail === 'Resource not found'
							? 'Akses platform administrator tidak tersedia.'
							: detail
				};
			}
			return { ok: true, data: (await res.json()) as T };
		} catch {
			return { ok: false, error: 'Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.' };
		}
	}

	async function load() {
		error = '';
		const [providerRes, usageRes, operationsRes, configurationRes, modelsRes] = await Promise.all([
			getJson<Provider>('/ai/status'),
			getJson<Usage>('/ai/usage?days=30'),
			getJson<Operations>('/operations/summary'),
			getJson<Configuration>('/ai/configuration'),
			getJson<{ items: ModelWithKnowledgeInfo[] }>('/ai/models')
		]);

		if (!providerRes.ok) {
			error = providerRes.error;
			return;
		}
		provider = providerRes.data;
		if (usageRes.ok) usage = usageRes.data;
		if (operationsRes.ok) operations = operationsRes.data;

		if (configurationRes.ok) {
			const data = configurationRes.data;
			configuration = data;
			formBaseUrl = data.base_url || '';
			formDefault = data.default_model || '';
			formEconomy = data.economy_model || '';
			formPremium = data.premium_model || '';
			if (data.base_url.includes('generativelanguage.googleapis.com')) template = 'google';
			else if (data.base_url.includes('openrouter.ai')) template = 'openrouter';
			else if (data.base_url.includes('cloudflare')) {
				template = 'cloudflare';
				cfAccountId = parseCfAccountId(data.base_url);
			} else if (data.base_url.includes('groq.com')) template = 'groq';
			else if (data.base_url.includes('11434')) template = 'ollama';
			else if (data.base_url.includes('openai.com')) template = 'openai';
			else if (data.base_url) template = 'custom';
		}

		if (modelsRes.ok) modelsList = modelsRes.data.items ?? [];
	}

	function applyTemplate(value: string) {
		const nextKey = value as ProviderTemplateKey;
		if (nextKey === template) return;

		const current: ProviderConfigState = {
			base_url: formBaseUrl,
			api_key: formApiKey,
			default_model: formDefault,
			economy_model: formEconomy,
			premium_model: formPremium,
			cf_account_id: cfAccountId
		};
		const updated = { ...savedConfigs, [template]: current };
		persistSavedConfigs(updated);

		const targetSaved = updated[nextKey] ?? defaultProviderState()[nextKey];
		const preset = PROVIDER_TEMPLATES[nextKey];

		template = nextKey;
		formBaseUrl = targetSaved.base_url || preset?.base_url || '';
		formApiKey = targetSaved.api_key || '';
		formDefault = targetSaved.default_model || preset?.default_model || '';
		formEconomy = targetSaved.economy_model || preset?.economy_model || '';
		formPremium = targetSaved.premium_model || preset?.premium_model || '';
		cfAccountId =
			targetSaved.cf_account_id ||
			(nextKey === 'cloudflare' ? parseCfAccountId(targetSaved.base_url || '') : '');
	}

	async function saveConfiguration(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		notice = '';
		try {
			const finalUrl = template === 'cloudflare' ? buildCfUrl(cfAccountId) : formBaseUrl.trim();
			const res = await apiRequest('/v1/platform/ai/configuration', {
				method: 'PUT',
				body: JSON.stringify({
					base_url: finalUrl,
					api_key: formApiKey.trim() || undefined,
					default_model: formDefault.trim(),
					economy_model: formEconomy.trim() || undefined,
					premium_model: formPremium.trim() || undefined
				})
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			formApiKey = '';
			notice = 'Konfigurasi AI provider berhasil disimpan.';
			await load();
		} finally {
			busy = false;
		}
	}

	async function probe(tier: ModelTier) {
		busy = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest('/v1/platform/ai/probe', {
				method: 'POST',
				body: JSON.stringify({ tier })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			const data = (await res.json()) as Generation;
			result = data;
			notice = `Probe ${tier} berhasil melalui ${data.model}.`;
		} finally {
			busy = false;
		}
	}

	async function generate(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const res = await apiRequest('/v1/platform/ai/test-generation', {
				method: 'POST',
				body: JSON.stringify({ tier: testTier, prompt: testPrompt })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			result = (await res.json()) as Generation;
		} finally {
			busy = false;
		}
	}

	async function toggleGeneration() {
		busy = true;
		error = '';
		try {
			const res = await apiRequest('/v1/platform/operations/generation-switch', {
				method: 'PUT',
				body: JSON.stringify({ generation_disabled: !operations?.generation_disabled })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			switchOpen = false;
			await load();
		} finally {
			busy = false;
		}
	}

	// ── Knowledge drawer ─────────────────────────────────────────────────
	function openKnowledge(target: ModelWithKnowledgeInfo) {
		drawerTarget = target;
		drawerError = '';
		drawerNotice = '';
		void loadKnowledge(target.model_id);
	}

	const knowledgeDefaults: Record<string, string | null> = {};
	function resetKnowledgeForm() {
		kReasoning = 'sedang';
		kSummary = '';
		kCharacteristics = '';
		kSuitableFor = '';
		kUnsuitableFor = '';
		kGuidelines = '';
		kLimitations = '';
		kEditorialContext = '';
		void knowledgeDefaults;
	}

	async function loadKnowledge(modelId: string) {
		drawerLoading = true;
		drawerChunks = [];
		drawerExisting = null;
		try {
			const res = await apiRequest(`/v1/platform/ai/models/${encodeURIComponent(modelId)}/knowledge`);
			if (!res.ok) {
				drawerError = await readApiError(res, 'Permintaan platform gagal.');
				resetKnowledgeForm();
				return;
			}
			const data = (await res.json()) as { knowledge: ModelKnowledge | null; chunks: KnowledgeChunk[] };
			const k = data.knowledge;
			drawerExisting = k;
			drawerChunks = data.chunks ?? [];
			if (k) {
				kReasoning = (k.reasoning_level as string) || 'sedang';
				kSummary = k.summary || '';
				kCharacteristics = k.characteristics || '';
				kSuitableFor = k.suitable_for || '';
				kUnsuitableFor = k.unsuitable_for || '';
				kGuidelines = k.guidelines || '';
				kLimitations = k.limitations || '';
				kEditorialContext = k.editorial_context || '';
			} else {
				resetKnowledgeForm();
			}
		} finally {
			drawerLoading = false;
		}
	}

	async function saveKnowledge(e: SubmitEvent) {
		e.preventDefault();
		if (!drawerTarget) return;
		drawerBusy = true;
		drawerError = '';
		drawerNotice = '';
		try {
			const res = await apiRequest(
				`/v1/platform/ai/models/${encodeURIComponent(drawerTarget.model_id)}/knowledge`,
				{
					method: 'PUT',
					body: JSON.stringify({
						model_name: drawerTarget.model_name || drawerTarget.model_id,
						provider: drawerTarget.provider,
						reasoning_level: kReasoning,
						summary: kSummary.trim() || null,
						characteristics: kCharacteristics.trim() || null,
						suitable_for: kSuitableFor.trim() || null,
						unsuitable_for: kUnsuitableFor.trim() || null,
						guidelines: kGuidelines.trim() || null,
						limitations: kLimitations.trim() || null,
						editorial_context: kEditorialContext.trim() || null,
						status: 'active'
					})
				}
			);
			if (!res.ok) {
				drawerError = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			const data = await res.json();
			drawerExisting = data.knowledge;
			drawerNotice = `Knowledge tersimpan (${data.chunk_count} semantic RAG chunks terindeks).`;
			await load();
			setTimeout(() => (drawerTarget = null), 700);
		} finally {
			drawerBusy = false;
		}
	}

	async function deleteKnowledge() {
		if (!drawerTarget) return;
		if (!confirm(`Hapus knowledge untuk model ${drawerTarget.model_name || drawerTarget.model_id}? Model AI tetap dapat digunakan.`)) return;
		drawerBusy = true;
		drawerError = '';
		try {
			const res = await apiRequest(
				`/v1/platform/ai/models/${encodeURIComponent(drawerTarget.model_id)}/knowledge`,
				{ method: 'DELETE' }
			);
			if (!res.ok) {
				drawerError = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			drawerExisting = null;
			drawerChunks = [];
			drawerTarget = null;
			await load();
		} finally {
			drawerBusy = false;
		}
	}

	function handleAddCustomModelKnowledge() {
		const id = customModelInput.trim();
		if (!id) return;
		customModelInput = '';
		openKnowledge({
			model_id: id,
			model_name: id,
			provider: 'custom',
			tier: 'custom',
			is_active: false,
			has_knowledge: false,
			chunk_count: 0
		});
	}

	const filteredModels = $derived(
		modelsList.filter((m) => {
			if (!knowledgeSearch.trim()) return true;
			const q = knowledgeSearch.toLowerCase();
			return (
				m.model_id.toLowerCase().includes(q) ||
				m.model_name.toLowerCase().includes(q) ||
				m.provider.toLowerCase().includes(q) ||
				(m.knowledge?.summary || '').toLowerCase().includes(q)
			);
		})
	);

	const knowledgeCount = $derived(modelsList.filter((m) => m.has_knowledge).length);
	const tierTone = (tier?: string) =>
		tier === 'default' ? 'success' : tier === 'premium' ? 'accent' : tier === 'economy' ? 'warning' : 'neutral';
</script>

<section>
	<PageHeader
		title="AI Management"
		description="Provider health, routing model, Model Guidelines & RAG Knowledge, isolated testing, dan global safety controls."
	>
		<Badge tone={provider?.configured ? 'success' : 'danger'} label={provider?.configured ? 'Configured' : 'Not configured'} />
		<Badge tone={provider?.circuit === 'closed' ? 'success' : 'danger'} label="Circuit {provider?.circuit ?? '-'}" />
	</PageHeader>

	<Feedback {loading} {error} {notice} loadingText="Memuat status AI platform…" />

	{#if !provider && error.includes('Akses platform')}
		<div class="app-card mt-4 px-4 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke pengaturan AI platform.</p>
		</div>
	{:else if provider}
		<div class="mt-4 flex flex-wrap gap-2">
			{#each [
				{ key: 'provider', label: 'Provider' },
				{ key: 'knowledge', label: 'Model Guidelines & RAG' },
				{ key: 'routing', label: 'Routing' },
				{ key: 'test', label: 'Test' },
				{ key: 'safety', label: 'Safety Controls' }
			] as t (t.key)}
				<button
					class="app-btn {tab === t.key ? 'app-btn-primary' : 'app-btn-soft'} !min-h-8 !px-3 !text-xs"
					type="button"
					onclick={() => (tab = t.key as typeof tab)}
				>{t.label}</button>
			{/each}
		</div>

		<!-- TAB 1: PROVIDER -->
		{#if tab === 'provider'}
			<div class="mt-3 space-y-4">
				<div class="app-card p-4">
					<div class="flex flex-wrap items-start justify-between gap-3">
						<div>
							<h2 class="text-base font-semibold">AI Provider</h2>
							<p class="mt-1 text-sm text-muted-foreground">
								Pilih template lalu masukkan endpoint <code class="rounded bg-secondary px-1">/chat/completions</code>. Support OpenAI, Google Gemini, OpenRouter, Cloudflare, Groq, Ollama.
							</p>
						</div>
						<Badge tone={configuration?.api_key_configured ? 'success' : 'neutral'} label={configuration?.source ?? 'none'} />
					</div>

					<div class="mt-4 flex flex-wrap items-center gap-3">
						<label class="block">
							<span class="text-sm font-medium">Provider template</span>
							<Select
								ariaLabel="Provider template"
								minWidth={240}
								value={template}
								onchange={(v) => applyTemplate(v)}
								options={Object.entries(PROVIDER_TEMPLATES).map(([key, preset]) => ({ value: key, label: preset.label }))}
							/>
						</label>
						<div class="flex-1 rounded-lg border px-3 py-2 text-xs" style="border-color: var(--accent-foreground); color: var(--accent-foreground);">
							{PROVIDER_TEMPLATES[template]?.hint}
							{#if template === 'cloudflare'}
								<span class="ml-1 font-bold">{cfAccountId ? buildCfUrl(cfAccountId) : 'Isi Account ID di bawah'}</span>
							{/if}
						</div>
					</div>

					<form class="mt-4" onsubmit={saveConfiguration}>
						{#if template === 'cloudflare'}
							<div class="mb-4 grid gap-4 md:grid-cols-2">
								<label class="block">
									<span class="text-sm font-medium">Cloudflare Account ID <span class="text-xs text-muted-foreground">(tinggal paste ID-nya aja)</span></span>
									<input
										class="app-input mt-1 w-full"
										placeholder="e.g. f8b8f2d2a7b84f6c9e2d1a3b5c7d9e1f2"
										value={cfAccountId}
										oninput={(e) => {
											const id = (e.currentTarget as HTMLInputElement).value.trim();
											cfAccountId = id;
											const newUrl = id ? buildCfUrl(id) : PROVIDER_TEMPLATES.cloudflare.base_url;
											formBaseUrl = newUrl;
											updateCurrentConfig({ cf_account_id: id, base_url: newUrl });
										}}
									/>
								</label>
								<div>
									<span class="text-sm font-medium">Auto-built Base URL</span>
									<code class="mt-1 block break-all rounded bg-secondary px-2 py-1.5 text-xs">{formBaseUrl || 'isi Account ID dulu'}</code>
								</div>
							</div>
						{/if}
						<div class="grid gap-4 {template === 'cloudflare' ? '' : 'md:grid-cols-2'}">
							{#if template !== 'cloudflare'}
								<label class="block">
									<span class="text-sm font-medium">Base URL</span>
									<input
										class="app-input mt-1 w-full"
										type="url"
										required
										placeholder={PROVIDER_TEMPLATES[template]?.base_url || 'https://api.openai.com/v1'}
										value={formBaseUrl}
										oninput={(e) => {
											const val = (e.currentTarget as HTMLInputElement).value;
											formBaseUrl = val;
											updateCurrentConfig({ base_url: val });
										}}
									/>
								</label>
							{/if}
							<label class="block">
								<span class="text-sm font-medium">
									API Key {template === 'cloudflare' ? '(Cloudflare API Token)' : ''}
									{template === 'google' ? '(Google AI Studio Key)' : ''}
								</span>
								<input
									class="app-input mt-1 w-full"
									type="password"
									autocomplete="new-password"
									required={!configuration?.api_key_configured && !formApiKey}
									minlength={configuration?.api_key_configured ? 0 : 4}
									placeholder={formApiKey
										? 'API Key diisi'
										: configuration?.api_key_configured
											? `${configuration.api_key_hint} · (kosongkan jika tidak diganti)`
											: PROVIDER_TEMPLATES[template]?.api_key_placeholder || 'API Key...'}
									value={formApiKey}
									oninput={(e) => {
										const val = (e.currentTarget as HTMLInputElement).value;
										formApiKey = val;
										updateCurrentConfig({ api_key: val });
									}}
								/>
							</label>
						</div>

						<div class="mt-4 grid gap-4 md:grid-cols-3">
							<label class="block">
								<span class="text-sm font-medium">Default model</span>
								<input
									class="app-input mt-1 w-full"
									required
									maxlength={200}
									placeholder={PROVIDER_TEMPLATES[template]?.default_model || 'gpt-4o-mini'}
									value={formDefault}
									oninput={(e) => {
										const val = (e.currentTarget as HTMLInputElement).value;
										formDefault = val;
										updateCurrentConfig({ default_model: val });
									}}
								/>
							</label>
							<label class="block">
								<span class="text-sm font-medium">Economy model</span>
								<input
									class="app-input mt-1 w-full"
									maxlength={200}
									placeholder={PROVIDER_TEMPLATES[template]?.economy_model || 'Kosong = Default'}
									value={formEconomy}
									oninput={(e) => {
										const val = (e.currentTarget as HTMLInputElement).value;
										formEconomy = val;
										updateCurrentConfig({ economy_model: val });
									}}
								/>
							</label>
							<label class="block">
								<span class="text-sm font-medium">Premium model</span>
								<input
									class="app-input mt-1 w-full"
									maxlength={200}
									placeholder={PROVIDER_TEMPLATES[template]?.premium_model || 'Kosong = Default'}
									value={formPremium}
									oninput={(e) => {
										const val = (e.currentTarget as HTMLInputElement).value;
										formPremium = val;
										updateCurrentConfig({ premium_model: val });
									}}
								/>
							</label>
						</div>

						<div class="mt-4 flex flex-wrap items-center justify-between gap-3">
							<p class="text-xs text-muted-foreground">Economy dan Premium yang kosong otomatis memakai Default.</p>
							<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? 'Menyimpan...' : 'Simpan konfigurasi'}</button>
						</div>
					</form>
				</div>

				<div class="grid gap-4 md:grid-cols-2">
					<div class="app-card p-4">
						<div class="flex items-center justify-between">
							<h2 class="text-sm font-semibold">Provider state</h2>
							<Badge tone={provider.configured ? 'success' : 'danger'} label={provider.provider ?? 'Loading'} />
						</div>
						<dl class="mt-3 space-y-2 text-sm">
							<div class="flex gap-3"><dt class="w-[150px] text-muted-foreground">Configured</dt><dd>{provider.configured ? 'Yes' : 'No'}</dd></div>
							<div class="flex gap-3"><dt class="w-[150px] text-muted-foreground">Circuit</dt><dd>{provider.circuit ?? '-'}</dd></div>
							<div class="flex gap-3"><dt class="w-[150px] text-muted-foreground">Timeout</dt><dd>{provider.timeout_seconds ?? 0} seconds</dd></div>
							<div class="flex gap-3"><dt class="w-[150px] text-muted-foreground">Maximum retries</dt><dd>{provider.max_retries ?? 0}</dd></div>
						</dl>
						<div class="mt-4 flex flex-wrap gap-2">
							{#each ['default', 'economy', 'premium'] as tier (tier)}
								<button
									class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
									type="button"
									disabled={busy || !provider.configured}
									onclick={() => void probe(tier as ModelTier)}
								><Zap size={13} /> Probe {tier}</button>
							{/each}
						</div>
					</div>
					<div class="app-card p-4">
						<h2 class="text-sm font-semibold">Usage · 30 hari</h2>
						<dl class="mt-3 space-y-2 text-sm">
							<div class="flex gap-3"><dt class="w-[150px] text-muted-foreground">Calls</dt><dd>{fmtNum(usage?.summary.calls)}</dd></div>
							<div class="flex gap-3"><dt class="w-[150px] text-muted-foreground">Successful</dt><dd>{fmtNum(usage?.summary.successful_calls)}</dd></div>
							<div class="flex gap-3"><dt class="w-[150px] text-muted-foreground">Total tokens</dt><dd>{fmtNum(usage?.summary.total_tokens)}</dd></div>
						</dl>
					</div>
				</div>
			</div>
		{/if}

		<!-- TAB 2: MODEL GUIDELINES & RAG -->
		{#if tab === 'knowledge'}
			<div class="app-card mt-3 p-4">
				<div class="flex flex-wrap items-start justify-between gap-3">
					<div>
						<div class="flex items-center gap-2">
							<Brain size={20} style="color: var(--accent-foreground);" />
							<h2 class="text-sm font-semibold">AI Model Guidelines &amp; RAG Knowledge</h2>
						</div>
						<p class="mt-1 text-sm text-muted-foreground">
							Setiap model AI memiliki knowledge khusus (tingkat reasoning, kecocokan tugas, karakteristik, batasan, dan konteks editorial) yang diindeks sebagai semantic RAG chunks.
						</p>
					</div>
					<Badge tone="accent" label="{knowledgeCount} / {modelsList.length} Model Memiliki Knowledge" />
				</div>

				<div class="mt-4 flex flex-wrap items-center justify-between gap-3">
					<input class="app-input min-w-[260px] flex-1" placeholder="Cari model AI atau kata kunci knowledge..." bind:value={knowledgeSearch} />
					<div class="flex items-center gap-2">
						<input
							class="app-input w-[220px]"
							placeholder="Model ID kustom..."
							bind:value={customModelInput}
							onkeydown={(e) => {
								if (e.key === 'Enter') {
									e.preventDefault();
									handleAddCustomModelKnowledge();
								}
							}}
						/>
						<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={!customModelInput.trim()} onclick={handleAddCustomModelKnowledge}>
							<Plus size={13} /> + Tambah Model
						</button>
					</div>
				</div>

				<div class="mt-4 overflow-x-auto">
					<table class="w-full text-sm" style="min-width: 900px;">
						<thead>
							<tr class="text-xs text-muted-foreground">
								<th class="px-3 py-2 text-left font-medium" style="width:28%;">Model AI</th>
								<th class="px-3 py-2 text-left font-medium" style="width:12%;">Provider</th>
								<th class="px-3 py-2 text-left font-medium" style="width:15%;">Tingkat Reasoning</th>
								<th class="px-3 py-2 text-left font-medium" style="width:20%;">Status Knowledge</th>
								<th class="px-3 py-2 text-right font-medium" style="width:25%;">Aksi</th>
							</tr>
						</thead>
						<tbody>
							{#each filteredModels as m (m.model_id)}
								<tr class="border-t">
									<td class="px-3 py-2">
										<div class="flex flex-wrap items-center gap-2">
											<span class="font-medium">{m.model_name || m.model_id}</span>
											{#if m.tier && m.tier !== 'custom'}
												<Badge tone={tierTone(m.tier)} label={m.tier.toUpperCase()} />
											{/if}
										</div>
										<code class="text-xs text-muted-foreground">{m.model_id}</code>
									</td>
									<td class="px-3 py-2"><Badge tone="neutral" label={m.provider || '—'} /></td>
									<td class="px-3 py-2">
										{#if m.reasoning_level}
											<Badge tone={m.reasoning_level === 'tinggi' ? 'accent' : m.reasoning_level === 'rendah' ? 'success' : 'neutral'} label="Reasoning: {m.reasoning_level.toUpperCase()}" />
										{:else}
											<span class="text-xs text-muted-foreground">—</span>
										{/if}
									</td>
									<td class="px-3 py-2">
										{#if m.has_knowledge}
											<Badge tone="accent" label="{m.chunk_count} chunks terindeks" />
										{:else}
											<span class="text-xs text-muted-foreground">Belum tersedia</span>
										{/if}
									</td>
									<td class="px-3 py-2 text-right">
										<button
											class="app-btn {m.has_knowledge ? 'app-btn-soft' : 'app-btn-primary'} !min-h-7 !px-2.5 !text-xs"
											type="button"
											onclick={() => openKnowledge(m)}
										><Sparkles size={12} /> {m.has_knowledge ? 'Edit Knowledge' : '+ Tambah Knowledge'}</button>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
					{#if filteredModels.length === 0}
						<p class="px-4 py-8 text-center text-sm text-muted-foreground">Tidak ada model AI yang cocok dengan pencarian.</p>
					{/if}
				</div>
			</div>
		{/if}

		<!-- TAB 3: ROUTING -->
		{#if tab === 'routing'}
			<div class="app-card mt-3 p-4">
				<h2 class="text-sm font-semibold">Model routing</h2>
				<dl class="mt-3 space-y-3 text-sm">
					{#each Object.entries(provider.models ?? {}) as [tier, model] (tier)}
						{@const matching = modelsList.find((m) => m.model_id === model)}
						<div class="flex flex-wrap items-center justify-between gap-3">
							<div class="flex items-center gap-3">
								<dt class="w-[150px] text-muted-foreground">{tier}</dt>
								<dd class="flex items-center gap-2">
									<code class="rounded bg-secondary px-1.5 py-0.5 text-xs">{model ?? 'belum diatur'}</code>
									{#if matching?.reasoning_level}
										<Badge tone={matching.reasoning_level === 'tinggi' ? 'accent' : 'neutral'} label="Reasoning {matching.reasoning_level}" />
									{/if}
								</dd>
							</div>
							{#if model}
								<button
									class="app-btn {matching?.has_knowledge ? 'app-btn-soft' : 'app-btn-primary'} !min-h-7 !px-2.5 !text-xs"
									type="button"
									onclick={() =>
										openKnowledge(
											matching ?? {
												model_id: model,
												model_name: model,
												provider: provider?.provider || 'configured',
												tier,
												is_active: true,
												has_knowledge: false,
												chunk_count: 0
											}
										)}
								>
									<Sparkles size={12} />
									{matching?.has_knowledge ? `Knowledge (${matching.chunk_count} chunks)` : '+ Tambah Knowledge'}
								</button>
							{/if}
						</div>
					{/each}
				</dl>
				<div class="mt-4 rounded-lg border px-3 py-2 text-xs" style="border-color: var(--accent-foreground); color: var(--accent-foreground);">
					Routing generative dapat diubah pada tab Provider. Job yang sudah dibuat tetap memakai snapshot modelnya.
				</div>
			</div>
		{/if}

		<!-- TAB 4: TEST -->
		{#if tab === 'test'}
			<div class="mt-3 grid gap-4 md:grid-cols-2">
				<div class="app-card p-4">
					<h2 class="text-sm font-semibold">Isolated test generation</h2>
					<p class="mt-1 text-sm text-muted-foreground">
						Prompt ini dicatat sebagai admin_test_generation dan tidak masuk workflow produksi.
					</p>
					<form class="mt-4 space-y-4" onsubmit={generate}>
						<Select
							ariaLabel="Pilih tier"
							minWidth={200}
							value={testTier}
							onchange={(v) => (testTier = v as ModelTier)}
							options={[
								{ value: 'default', label: 'Default' },
								{ value: 'economy', label: 'Economy' },
								{ value: 'premium', label: 'Premium' }
							]}
						/>
						<textarea
							class="app-input w-full"
							rows="10"
							minlength={1}
							maxlength={4000}
							required
							placeholder="Tulis prompt pengujian..."
							bind:value={testPrompt}
						></textarea>
						<button class="app-btn app-btn-primary w-full" type="submit" disabled={busy || !provider.configured}>
							{busy ? 'Generating...' : 'Run isolated test'}
						</button>
					</form>
				</div>
				<div class="app-card p-4">
					<h2 class="text-sm font-semibold">Result</h2>
					{#if result}
						<dl class="mt-3 space-y-2 text-sm">
							<div class="flex gap-3"><dt class="w-[110px] text-muted-foreground">Model</dt><dd><code>{result.model}</code></dd></div>
							<div class="flex gap-3"><dt class="w-[110px] text-muted-foreground">Latency</dt><dd>{result.latency_ms} ms</dd></div>
							<div class="flex gap-3"><dt class="w-[110px] text-muted-foreground">Tokens</dt><dd>{result.input_tokens} input · {result.output_tokens} output</dd></div>
						</dl>
						<pre class="mt-3 max-h-[360px] overflow-auto whitespace-pre-wrap rounded-lg border px-3 py-2 text-xs">{result.content}</pre>
					{:else}
						<p class="mt-3 rounded-lg border px-3 py-8 text-center text-sm text-muted-foreground">Hasil test akan muncul di sini.</p>
					{/if}
				</div>
			</div>
		{/if}

		<!-- TAB 5: SAFETY -->
		{#if tab === 'safety'}
			<div class="app-card mt-3 p-4">
				<div class="flex flex-wrap items-start justify-between gap-3">
					<div>
						<div class="flex items-center gap-2">
							<h2 class="text-sm font-semibold">Global generation switch</h2>
							<Badge tone={operations?.generation_disabled ? 'danger' : 'success'} label={operations?.generation_disabled ? 'Disabled' : 'Enabled'} />
						</div>
						<p class="mt-1 text-sm text-muted-foreground">Mengontrol seluruh generation baru. Job yang sudah berjalan tidak dihapus.</p>
					</div>
					<button
						class="app-btn {operations?.generation_disabled ? 'app-btn-primary' : 'app-btn-soft'}"
						type="button"
						onclick={() => (switchOpen = true)}
					>{operations?.generation_disabled ? 'Enable generation' : 'Disable generation'}</button>
				</div>
				{#each operations?.alerts ?? [] as alert (alert)}
					<div class="mt-4 flex items-start gap-2 rounded-lg border px-3 py-2 text-sm" style="border-color: var(--warning);">
						<TriangleAlert size={15} class="mt-0.5 shrink-0" style="color: var(--warning);" />
						<p>{alert}</p>
					</div>
				{/each}
			</div>
		{/if}
	{/if}
</section>

<!-- Model knowledge modal -->
{#if drawerTarget}
	<Modal
		title="Knowledge Model"
		label="Knowledge Model"
		maxWidth="2xl"
		onClose={() => (drawerTarget = null)}
	>
		<div class="space-y-4">
			<p class="text-sm text-muted-foreground">
				Tambahkan pedoman dan informasi yang akan digunakan Tauthub saat menentukan atau menggunakan model ini.
			</p>

			<div class="grid gap-3 rounded-lg border px-4 py-3 sm:grid-cols-4">
				<div>
					<p class="text-[11px] uppercase tracking-wide text-muted-foreground">Nama Model</p>
					<p class="mt-1 break-words text-sm font-semibold">{drawerTarget.model_name || drawerTarget.model_id}</p>
				</div>
				<div>
					<p class="text-[11px] uppercase tracking-wide text-muted-foreground">Provider</p>
					<p class="mt-1 text-sm font-medium capitalize">{drawerTarget.provider || '—'}</p>
				</div>
				<div>
					<p class="text-[11px] uppercase tracking-wide text-muted-foreground">Model ID</p>
					<code class="mt-1 block break-all text-xs">{drawerTarget.model_id}</code>
				</div>
				<div>
					<p class="text-[11px] uppercase tracking-wide text-muted-foreground">Role / Tier</p>
					<div class="mt-1">
						<Badge tone={tierTone(drawerTarget.tier)} label={drawerTarget.tier ? drawerTarget.tier.toUpperCase() : 'CUSTOM'} />
					</div>
				</div>
			</div>

			{#if drawerError}
				<div class="rounded-lg border px-3 py-2 text-sm" style="color: var(--destructive);">{drawerError}</div>
			{/if}
			{#if drawerNotice}
				<div class="flex items-center gap-2 rounded-lg border px-3 py-2 text-sm" style="color: var(--success);">
					<CheckCircle2 size={15} /> {drawerNotice}
				</div>
			{/if}
			{#if drawerLoading}<p class="text-sm text-muted-foreground">Memuat knowledge model…</p>{/if}

			<form id="model-knowledge-form" class="space-y-4" onsubmit={saveKnowledge}>
				<div class="grid gap-4 sm:grid-cols-3">
					<label class="block sm:col-span-2">
						<span class="text-sm font-semibold">1. Ringkasan Model</span>
						<textarea class="app-input mt-2 w-full" rows="3" maxlength={2000} bind:value={kSummary}
							placeholder="Model dengan kemampuan reasoning rendah yang cocok untuk tugas sederhana dan cepat."></textarea>
						<span class="mt-1 block text-right text-[11px] text-muted-foreground">{kSummary.length}/2000 karakter</span>
					</label>
					<label class="block">
						<span class="text-sm font-semibold">2. Tingkat Reasoning</span>
						<Select
							ariaLabel="Tingkat Reasoning"
							minWidth={260}
							value={kReasoning}
							onchange={(v) => (kReasoning = v)}
							options={[
								{ value: 'rendah', label: 'Rendah (Fast & Simple)' },
								{ value: 'sedang', label: 'Sedang (Standard)' },
								{ value: 'tinggi', label: 'Tinggi (Deep Reasoning)' }
							]}
						/>
						<span class="mt-1 block text-[11px] text-muted-foreground">Tingkat kedalaman pemikiran logika model.</span>
					</label>
				</div>

				<label class="block">
					<span class="text-sm font-semibold">3. Karakteristik Model</span>
					<textarea class="app-input mt-2 w-full" rows="3" maxlength={3000} bind:value={kCharacteristics}
						placeholder="Karakteristik output, format penulisan, kecepatan, gaya penjelasan, daya tahan konteks..."></textarea>
					<span class="mt-1 block text-right text-[11px] text-muted-foreground">{kCharacteristics.length}/3000 karakter</span>
				</label>

				<div class="grid gap-4 sm:grid-cols-2">
					<label class="block">
						<span class="text-sm font-semibold" style="color: var(--success);">4. Cocok Untuk</span>
						<textarea class="app-input mt-2 w-full" rows="4" maxlength={3000} bind:value={kSuitableFor}
							placeholder={'- Rewriting ringan & parafrase\n- Klasifikasi berita & entitas\n- Ekstraksi data kuesioner\n- Summarization artikel'}></textarea>
						<span class="mt-1 block text-right text-[11px] text-muted-foreground">{kSuitableFor.length}/3000 karakter</span>
					</label>
					<label class="block">
						<span class="text-sm font-semibold" style="color: var(--destructive);">5. Tidak Ideal Untuk</span>
						<textarea class="app-input mt-2 w-full" rows="4" maxlength={3000} bind:value={kUnsuitableFor}
							placeholder={'- Reasoning multi-step yang rumit\n- Rantai instruksi bertingkat panjang\n- Analisis perbandingan data masif'}></textarea>
						<span class="mt-1 block text-right text-[11px] text-muted-foreground">{kUnsuitableFor.length}/3000 karakter</span>
					</label>
				</div>

				<label class="block">
					<span class="text-sm font-semibold">6. Pedoman Penggunaan AI</span>
					<textarea class="app-input mt-2 w-full" rows="4" maxlength={5000} bind:value={kGuidelines}
						placeholder="Instruksi khusus prompt engineering untuk model ini..."></textarea>
					<span class="mt-1 block text-right text-[11px] text-muted-foreground">{kGuidelines.length}/5000 karakter</span>
				</label>

				<label class="block">
					<span class="text-sm font-semibold">7. Catatan / Limitasi</span>
					<textarea class="app-input mt-2 w-full" rows="3" maxlength={5000} bind:value={kLimitations}
						placeholder="Keterbatasan teknis, context window limit, token output maksimum..."></textarea>
					<span class="mt-1 block text-right text-[11px] text-muted-foreground">{kLimitations.length}/5000 karakter</span>
				</label>

				<label class="block">
					<span class="text-sm font-semibold">8. RAG Knowledge / Editorial Context</span>
					<textarea class="app-input mt-2 w-full" rows="4" maxlength={5000} bind:value={kEditorialContext}
						placeholder="Konteks editorial atau pedoman penulisan khusus saat artikel dihasilkan oleh model ini..."></textarea>
					<span class="mt-1 block text-right text-[11px] text-muted-foreground">{kEditorialContext.length}/5000 karakter</span>
				</label>

				{#if drawerChunks.length > 0}
					<div class="rounded-lg border px-3 py-3">
						<p class="text-xs font-semibold text-muted-foreground">RAG Semantic Chunks yang Terindeks ({drawerChunks.length})</p>
						<div class="mt-2 space-y-2">
							{#each drawerChunks as c (c.id)}
								<div class="rounded-lg border px-2 py-2">
									<div class="flex items-center justify-between gap-2">
										<Badge tone="accent" label={c.section} />
										<span class="text-[11px] text-muted-foreground">Priority {c.priority}</span>
									</div>
									<p class="mt-1 line-clamp-3 whitespace-pre-wrap text-xs text-muted-foreground">{c.content}</p>
								</div>
							{/each}
						</div>
					</div>
				{/if}
			</form>
		</div>

		{#snippet footer()}
			<div class="mr-auto">
				{#if drawerExisting}
					<button class="app-btn app-btn-soft" type="button" disabled={drawerBusy} onclick={() => void deleteKnowledge()}>
						Hapus Knowledge
					</button>
				{:else}
					<span class="text-xs text-muted-foreground">Knowledge model bersifat opsional dan tidak mengubah konfigurasi runtime.</span>
				{/if}
			</div>
			<button class="app-btn app-btn-soft" type="button" disabled={drawerBusy} onclick={() => (drawerTarget = null)}>Batal</button>
			<button class="app-btn app-btn-primary" type="submit" form="model-knowledge-form" disabled={drawerBusy || drawerLoading}>
				{drawerBusy ? 'Menyimpan...' : drawerExisting ? 'Simpan Perubahan' : 'Simpan Knowledge'}
			</button>
		{/snippet}
	</Modal>
{/if}

<!-- Konfirmasi kill switch -->
{#if switchOpen}
	<Modal
		title={operations?.generation_disabled ? 'Aktifkan generation global?' : 'Hentikan generation global?'}
		label="Konfirmasi generation switch"
		maxWidth="md"
		onClose={() => (switchOpen = false)}
	>
		<p class="text-sm text-muted-foreground">
			{operations?.generation_disabled
				? 'Brand dapat mengirim generation baru setelah kontrol diaktifkan.'
				: 'Seluruh brand akan ditolak ketika mengirim generation baru sampai kontrol diaktifkan kembali.'}
		</p>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => (switchOpen = false)}>Batal</button>
			<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void toggleGeneration()}>
				{busy ? 'Menyimpan...' : operations?.generation_disabled ? 'Aktifkan' : 'Hentikan'}
			</button>
		{/snippet}
	</Modal>
{/if}
