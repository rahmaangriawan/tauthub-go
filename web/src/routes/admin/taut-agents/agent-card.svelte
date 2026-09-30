<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Kartu konfigurasi satu agent Taut Pintar: base URL, API key (sekali tulis),
	// model, temperature/max tokens/timeout/rate limit, system prompt, enabled,
	// plus uji koneksi. Port AgentCard di app/admin/taut-agents/page.tsx.
	import { Bot, CheckCircle2, TriangleAlert } from '@lucide/svelte';

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

	let {
		config,
		meta,
		onSave,
		onTest
	}: {
		config: AgentConfig;
		meta: { label: string; role: string; hint: string };
		onSave: (agent: string, form: Record<string, unknown>) => Promise<void>;
		onTest: (agent: string) => Promise<{ ok: boolean; message: string; latencyMs?: number }>;
	} = $props();

	// Form dikendalikan state lokal. Nilai awal disalin dari prop config lewat
	// closure (bukan dibaca langsung di initializer) supaya tidak memicu
	// peringatan state_referenced_locally.
	let baseUrl = $state('');
	let apiKey = $state('');
	let model = $state('');
	let temperature = $state('');
	let maxTokens = $state('');
	let timeoutMs = $state('');
	let systemPrompt = $state('');
	let enabled = $state(false);
	let rateLimit = $state('');

	$effect(() => {
		baseUrl = config.base_url ?? '';
		model = config.model ?? '';
		temperature = String(config.temperature ?? 0.7);
		maxTokens = config.max_tokens != null ? String(config.max_tokens) : '';
		timeoutMs = String(config.timeout_ms ?? 60000);
		systemPrompt = config.system_prompt ?? '';
		enabled = config.enabled;
		rateLimit = String(config.rate_limit_per_min ?? 3);
	});

	let busy = $state(false);
	let msg = $state('');
	let msgOk = $state(false);
	let testBusy = $state(false);
	let testMsg = $state('');
	let testOk = $state(false);

	async function save(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		msg = '';
		try {
			const payload: Record<string, unknown> = { enabled };
			if (baseUrl.trim()) payload.base_url = baseUrl.trim();
			if (apiKey.trim()) payload.api_key = apiKey.trim();
			if (model.trim()) payload.model = model.trim();
			if (temperature.trim()) payload.temperature = Number(temperature);
			if (maxTokens.trim()) payload.max_tokens = Number(maxTokens);
			if (timeoutMs.trim()) payload.timeout_ms = Number(timeoutMs);
			if (systemPrompt.trim()) payload.system_prompt = systemPrompt.trim();
			if (rateLimit.trim()) payload.rate_limit_per_min = Number(rateLimit);
			await onSave(config.agent, payload);
			apiKey = '';
			msg = tr('Tersimpan.');
			msgOk = true;
		} catch (reason) {
			msg = reason instanceof Error ? reason.message : tr('Gagal menyimpan.');
			msgOk = false;
		} finally {
			busy = false;
		}
	}

	async function test() {
		testBusy = true;
		testMsg = '';
		try {
			const res = await onTest(config.agent);
			testOk = res.ok;
			testMsg = res.ok
				? `✓ ${res.message}${res.latencyMs ? ` (${res.latencyMs}ms)` : ''}`
				: `✕ ${res.message}`;
		} catch (reason) {
			testOk = false;
			testMsg = `✕ ${reason instanceof Error ? reason.message : 'Uji gagal.'}`;
		} finally {
			testBusy = false;
		}
	}
</script>

<div class="app-card p-4">
	<div class="flex flex-wrap items-center gap-2">
		<Bot size={20} />
		<h3 class="text-sm font-semibold">{meta.label}</h3>
		<span class="text-xs text-muted-foreground">({meta.role})</span>
		<span
			class="rounded-full px-2 py-0.5 text-[11px] font-medium"
			style="background: {config.configured ? 'var(--success-soft)' : 'var(--destructive-soft, #fdecee)'}; color: {config.configured ? 'var(--success)' : 'var(--destructive)'};"
		>{config.configured ? 'Configured' : 'Hard stop'}</span>
		{#if !config.enabled}
			<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--secondary); color: var(--muted-foreground);">Disabled</span>
		{/if}
	</div>
	<p class="mt-1.5 text-xs text-muted-foreground">{meta.hint}</p>

	<form class="mt-3 flex flex-col gap-3" onsubmit={save}>
		<label class="block">
			<span class="text-xs font-medium text-muted-foreground">Base URL</span>
			<input class="app-input mt-1 w-full" placeholder="https://api.openai.com/v1" bind:value={baseUrl} />
		</label>
		<label class="block">
			<span class="text-xs font-medium text-muted-foreground">API Key</span>
			<input
				class="app-input mt-1 w-full"
				type="password"
				placeholder={config.api_key_configured ? tr('Kosongkan jika tidak diubah') : 'sk-…'}
				bind:value={apiKey}
			/>
		</label>
		<label class="block">
			<span class="text-xs font-medium text-muted-foreground">Model</span>
			<input class="app-input mt-1 w-full" placeholder="gpt-4o" bind:value={model} />
		</label>

		<div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Temperature</span>
				<input class="app-input mt-1 w-full" type="number" step="0.1" min="0" max="2" bind:value={temperature} />
			</label>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Max tokens</span>
				<input class="app-input mt-1 w-full" type="number" placeholder={tr("opsional")} bind:value={maxTokens} />
			</label>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Timeout (ms)</span>
				<input class="app-input mt-1 w-full" type="number" placeholder="60000" bind:value={timeoutMs} />
			</label>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Rate limit (/min)</span>
				<input class="app-input mt-1 w-full" type="number" min="0" max="600" placeholder="3" bind:value={rateLimit} />
				<span class="text-[11px] text-muted-foreground">{tr("0-600 permintaan per menit. 0 = tanpa batas.")}</span>
			</label>
		</div>

		<label class="block">
			<span class="text-xs font-medium text-muted-foreground">System Prompt (knowledge)</span>
			<textarea class="app-input mt-1 w-full font-mono text-xs" rows="10" placeholder={tr("Instruksi agent...")} bind:value={systemPrompt}></textarea>
		</label>

		<label class="flex items-center gap-2 text-sm">
			<input type="checkbox" bind:checked={enabled} />
			Enabled
		</label>

		{#if msg}
			<p class="text-sm" style="color: {msgOk ? 'var(--success)' : 'var(--destructive)'};">{msg}</p>
		{/if}

		<div class="flex flex-wrap items-center gap-2">
			<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? 'Menyimpan…' : tr('Simpan')}</button>
			<button class="app-btn app-btn-soft" type="button" disabled={testBusy} onclick={test}>
				{testBusy ? 'Menguji…' : 'Test koneksi'}
			</button>
			{#if testMsg}
				<span class="text-sm" style="color: {testOk ? 'var(--success)' : 'var(--destructive)'};">{testMsg}</span>
			{/if}
		</div>
	</form>

	{#if !config.configured}
		<p class="mt-3 flex items-center gap-2 rounded-lg px-3 py-2 text-xs" style="background: var(--destructive-soft, #fdecee); color: var(--destructive);">
			<TriangleAlert size={14} />{tr("Belum dikonfigurasi — kampanye akan hard stop saat mencapai agent ini.")}</p>
	{/if}
	{#if msgOk}
		<p class="mt-2 flex items-center gap-1 text-xs" style="color: var(--success);"><CheckCircle2 size={12} /> Perubahan tersimpan.</p>
	{/if}
</div>
