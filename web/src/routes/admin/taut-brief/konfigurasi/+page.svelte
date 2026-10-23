<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Taut Brief — console asisten Profiler brand. Sengaja BERDIRI SENDIRI,
	// terpisah dari /admin/taut-agents: konfigurasi, prompt, skema field, dan
	// log-nya memakai tabel taut_brief_* sendiri (lihat migrasi 212).
	//
	// Lima tab: Koneksi (transport + probe kapabilitas model), Prompt (versi
	// Brief Engine), Skema Field (katalog field yang boleh diisi AI), Uji
	// (jalankan satu giliran tanpa menyentuh Profiler siapa pun), Log.
	import { onMount } from 'svelte';
	import { CheckCircle2, Play, Plus, RefreshCw, Save, Trash2, TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import ExplorerFrame from '$lib/components/admin/explorer-frame.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';

	const BASE = '/v1/platform/taut-brief';

	type Capabilities = {
		json_mode?: boolean;
		stream_json?: boolean;
		probed_at?: string;
		json_note?: string;
		stream_note?: string;
		model?: string;
		latency_ms?: { json?: number; stream?: number };
	};

	type ConfigResponse = {
		base_url: string;
		model: string;
		temperature: number;
		max_tokens: number | null;
		timeout_ms: number;
		rate_limit_per_min: number;
		enabled: boolean;
		api_key_configured: boolean;
		capabilities: Capabilities | null;
		configured: boolean;
	};

	type VersionMeta = {
		id: string;
		version: number;
		status: string;
		change_note: string | null;
		created_at: string;
		published_at: string | null;
	};

	type Field = {
		key: string;
		layer: number;
		section: string;
		column?: string;
		label: string;
		widget: string;
		required: boolean;
		sort_order: number;
		example_question: string;
		enabled: boolean;
	};

	type FieldUpdate = {
		key: string;
		value: string;
		status: string;
		source: string;
		note: string;
	};

	type RunLog = {
		id: string;
		profile_name: string;
		brand_name: string;
		agent: string | null;
		status: string;
		model: string | null;
		token_input: number | null;
		token_output: number | null;
		duration_ms: number | null;
		error: string | null;
		created_at: string;
	};

	let tab = $state<'koneksi' | 'prompt' | 'fields' | 'uji' | 'log'>('koneksi');
	let loading = $state(true);
	let error = $state('');
	let notice = $state('');
	let busy = $state(false);

	// Koneksi
	let config = $state<ConfigResponse | null>(null);
	let form = $state({ base_url: '', api_key: '', model: '', temperature: 0.3, max_tokens: '', timeout_ms: 60000, rate_limit_per_min: 12, enabled: false });
	let probeMessage = $state('');
	let probeOk = $state(false);

	// Prompt
	let prompts = $state<VersionMeta[]>([]);
	let promptText = $state('');
	let promptNote = $state('');
	let activePromptId = $state('');

	// Skema field
	let fieldVersions = $state<VersionMeta[]>([]);
	let fields = $state<Field[]>([]);
	let fieldsNote = $state('');
	let activeFieldVersionId = $state('');

	// Uji
	let scenario = $state('');
	let material = $state('');
	let testResult = $state<any>(null);
	let testError = $state('');

	// Log
	let logs = $state<RunLog[]>([]);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([loadConfig(), loadPrompts(), loadFields(), loadLogs()]);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function loadConfig() {
		const res = await apiRequest(`${BASE}/config`);
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal memuat konfigurasi Taut Brief.'));
			return;
		}
		config = await res.json();
		form = {
			base_url: config?.base_url ?? '',
			api_key: '',
			model: config?.model ?? '',
			temperature: config?.temperature ?? 0.3,
			max_tokens: config?.max_tokens ? String(config.max_tokens) : '',
			timeout_ms: config?.timeout_ms ?? 60000,
			rate_limit_per_min: config?.rate_limit_per_min ?? 12,
			enabled: config?.enabled ?? false
		};
	}

	async function saveConfig() {
		busy = true;
		error = '';
		notice = '';
		try {
			const payload: Record<string, unknown> = {
				base_url: form.base_url,
				model: form.model,
				temperature: Number(form.temperature),
				timeout_ms: Number(form.timeout_ms),
				rate_limit_per_min: Number(form.rate_limit_per_min),
				enabled: form.enabled
			};
			if (form.api_key.trim()) payload.api_key = form.api_key.trim();
			if (String(form.max_tokens).trim()) payload.max_tokens = Number(form.max_tokens);
			const res = await apiRequest(`${BASE}/config`, { method: 'PUT', body: JSON.stringify(payload) });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan konfigurasi.')));
			notice = tr('Konfigurasi tersimpan.');
			await loadConfig();
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal menyimpan konfigurasi.');
		} finally {
			busy = false;
		}
	}

	async function probe() {
		busy = true;
		probeMessage = '';
		try {
			const res = await apiRequest(`${BASE}/config/test`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, tr('Uji koneksi gagal.')));
			const data = await res.json();
			probeOk = Boolean(data?.ok);
			probeMessage = String(data?.message ?? '');
			await loadConfig();
		} catch (e) {
			probeOk = false;
			probeMessage = e instanceof Error ? e.message : tr('Uji koneksi gagal.');
		} finally {
			busy = false;
		}
	}

	async function loadPrompts() {
		const res = await apiRequest(`${BASE}/prompts`);
		if (!res.ok) return;
		const data = await res.json();
		prompts = data?.items ?? [];
		const active = prompts.find((v) => v.status === 'active') ?? prompts[0];
		if (active) {
			activePromptId = active.id;
			await loadPromptDetail(active.id);
		}
	}

	async function loadPromptDetail(id: string) {
		const res = await apiRequest(`${BASE}/prompts/${id}`);
		if (!res.ok) return;
		const data = await res.json();
		promptText = data?.version?.prompt ?? '';
		promptNote = data?.version?.change_note ?? '';
	}

	async function savePrompt() {
		if (!activePromptId) return;
		busy = true;
		try {
			const res = await apiRequest(`${BASE}/prompts/${activePromptId}`, {
				method: 'PATCH',
				body: JSON.stringify({ prompt: promptText, change_note: promptNote })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan prompt.')));
			notice = tr('Prompt tersimpan.');
			await loadPrompts();
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal menyimpan prompt.');
		} finally {
			busy = false;
		}
	}

	async function newPromptVersion() {
		busy = true;
		try {
			const res = await apiRequest(`${BASE}/prompts`, {
				method: 'POST',
				body: JSON.stringify({ clone_active: true, change_note: promptNote })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal membuat versi prompt.')));
			notice = tr('Versi prompt baru dibuat dan langsung aktif.');
			await loadPrompts();
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal membuat versi prompt.');
		} finally {
			busy = false;
		}
	}

	async function activatePrompt(id: string) {
		busy = true;
		try {
			const res = await apiRequest(`${BASE}/prompts/${id}/activate`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal mengaktifkan versi.')));
			await loadPrompts();
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal mengaktifkan versi.');
		} finally {
			busy = false;
		}
	}

	async function deletePrompt(id: string) {
		if (!confirm(tr('Hapus versi prompt ini?'))) return;
		busy = true;
		try {
			const res = await apiRequest(`${BASE}/prompts/${id}`, { method: 'DELETE' });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menghapus versi.')));
			await loadPrompts();
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal menghapus versi.');
		} finally {
			busy = false;
		}
	}

	async function loadFields() {
		const res = await apiRequest(`${BASE}/fields`);
		if (!res.ok) return;
		const data = await res.json();
		fieldVersions = data?.items ?? [];
		const active = fieldVersions.find((v) => v.status === 'active') ?? fieldVersions[0];
		if (active) {
			activeFieldVersionId = active.id;
			await loadFieldDetail(active.id);
		}
	}

	async function loadFieldDetail(id: string) {
		const res = await apiRequest(`${BASE}/fields/${id}`);
		if (!res.ok) return;
		const data = await res.json();
		fields = [...(data?.version?.fields ?? [])].sort((a, b) => a.sort_order - b.sort_order);
		fieldsNote = data?.version?.change_note ?? '';
	}

	async function saveFieldVersion() {
		busy = true;
		try {
			const payload = { fields: renumbered(), change_note: fieldsNote };
			const res = await apiRequest(`${BASE}/fields`, { method: 'POST', body: JSON.stringify(payload) });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan skema field.')));
			notice = tr('Skema field disimpan sebagai versi baru dan langsung aktif.');
			await loadFields();
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal menyimpan skema field.');
		} finally {
			busy = false;
		}
	}

	function renumbered(): Field[] {
		return [...fields]
			.sort((a, b) => a.sort_order - b.sort_order)
			.map((f, i) => ({ ...f, sort_order: (f.layer === 1 ? 1 : f.layer === 2 ? 100 : 900) + i }));
	}

	function moveField(index: number, dir: -1 | 1) {
		const sorted = renumbered();
		const target = index + dir;
		if (target < 0 || target >= sorted.length) return;
		[sorted[index], sorted[target]] = [sorted[target], sorted[index]];
		fields = sorted.map((f, i) => ({ ...f, sort_order: i + 1 }));
	}

	function addField(layer: number) {
		fields = [
			...fields,
			{
				key: '',
				layer,
				section: layer === 1 ? 'Entity Core' : 'Identitas Brand',
				label: '',
				widget: 'text',
				required: layer === 1,
				sort_order: fields.length + 1,
				example_question: '',
				enabled: true
			}
		];
	}

	async function runTest() {
		busy = true;
		testResult = null;
		testError = '';
		try {
			const res = await apiRequest(`${BASE}/test`, {
				method: 'POST',
				body: JSON.stringify({ scenario, material, fields: renumbered() })
			}, 300_000);
			if (!res.ok) throw new Error(await readApiError(res, tr('Uji prompt gagal.')));
			testResult = await res.json();
		} catch (e) {
			testError = e instanceof Error ? e.message : tr('Uji prompt gagal.');
		} finally {
			busy = false;
		}
	}

	async function loadLogs() {
		const res = await apiRequest(`${BASE}/logs?limit=100`);
		if (!res.ok) return;
		const data = await res.json();
		logs = data?.items ?? [];
	}

	const layerLabel = (n: number) => (n === 1 ? 'Lapis 1 — Entity Core' : n === 2 ? 'Lapis 2 — Brand Context' : 'Lapis 3 — Campaign Memory');
	const caps = $derived(config?.capabilities ?? null);
	const streamFlag = $derived(caps?.stream_json === true);
</script>

<section>
	<ExplorerFrame parents={[{ label: tr('Taut Pintar'), href: '/admin/taut-pintar' }, { label: tr('Taut Brief'), href: '/admin/taut-brief' }]} crumb={tr('Konfigurasi')} path="/admin/taut-brief/konfigurasi">
	<PageHeader
		title="Taut Brief"
		description={tr('Asisten Profiler brand: percakapan yang menyusun brief lewat chat. Terpisah dari agent Taut Pintar.')}
	>
		<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => void probe()}>
			<RefreshCw size={14} />{tr('Uji koneksi')}
		</button>
	</PageHeader>

	<div class="mt-4 flex flex-wrap gap-1">
		{#each [['koneksi', 'Koneksi'], ['prompt', 'Prompt'], ['fields', 'Skema Field'], ['uji', 'Uji'], ['log', 'Log']] as [value, label] (value)}
			<button
				type="button"
				class="app-btn !min-h-8 !px-3 !text-xs"
				class:app-btn-primary={tab === value}
				class:app-btn-soft={tab !== value}
				onclick={() => (tab = value as typeof tab)}
			>{tr(label)}</button>
		{/each}
	</div>

	<Feedback {loading} {error} {notice} />

	{#if !loading}
		<!-- ── KONEKSI ───────────────────────────────────────────── -->
		{#if tab === 'koneksi'}
			<section class="app-card mt-4 p-5">
				<h2 class="text-sm font-semibold">{tr('Transport & Model')}</h2>
				<p class="mt-1 text-xs text-muted-foreground">
					{tr('Base URL harus OpenAI-compatible. API key disegel sebelum disimpan dan tidak pernah dikirim ke browser.')}
				</p>

				<div class="mt-4 grid gap-3 sm:grid-cols-2">
					<label class="block sm:col-span-2">
						<span class="text-xs font-medium text-muted-foreground">Base URL</span>
						<input class="app-input mt-1 w-full" placeholder="https://openrouter.ai/api/v1" bind:value={form.base_url} />
					</label>
					<label class="block sm:col-span-2">
						<span class="text-xs font-medium text-muted-foreground">API Key</span>
						<input
							class="app-input mt-1 w-full"
							type="password"
							placeholder={config?.api_key_configured ? tr('Kosongkan jika tidak diubah') : 'sk-…'}
							bind:value={form.api_key}
						/>
					</label>
					<label class="block sm:col-span-2">
						<span class="text-xs font-medium text-muted-foreground">{tr('Model')}</span>
						<input class="app-input mt-1 w-full" placeholder="deepseek/deepseek-v4.1" bind:value={form.model} />
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Temperature</span>
						<input class="app-input mt-1 w-full" type="number" step="0.1" min="0" max="2" bind:value={form.temperature} />
						<span class="text-[11px] text-muted-foreground">{tr('Brief engine pakai nilai rendah (0.2-0.4).')}</span>
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Max tokens</span>
						<input class="app-input mt-1 w-full" type="number" placeholder={tr('opsional')} bind:value={form.max_tokens} />
						<span class="text-[11px] text-muted-foreground">{tr('Kosongkan untuk bawaan provider. Maksimal 200000.')}</span>
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Timeout (ms)</span>
						<input class="app-input mt-1 w-full" type="number" bind:value={form.timeout_ms} />
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Rate limit / menit</span>
						<input class="app-input mt-1 w-full" type="number" min="0" max="600" bind:value={form.rate_limit_per_min} />
						<span class="text-[11px] text-muted-foreground">{tr('0 = tanpa batas.')}</span>
					</label>
				</div>

				<label class="mt-4 flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={form.enabled} />
					{tr('Aktifkan fitur Taut Brief untuk klien')}
				</label>

				{#if !form.enabled}
					<p class="mt-3 flex items-start gap-2 text-xs" style="color: var(--warning);">
						<TriangleAlert size={14} class="mt-0.5 shrink-0" />
						{tr('Selama nonaktif, halaman Profiler klien menampilkan pesan bahwa fitur belum dikonfigurasi admin.')}
					</p>
				{/if}

				<div class="mt-4 flex items-center gap-2">
					<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void saveConfig()}>
						<Save size={14} />{busy ? tr('Menyimpan…') : tr('Simpan')}
					</button>
					<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => void probe()}>
						<Play size={14} />{tr('Uji koneksi')}
					</button>
				</div>

				{#if probeMessage}
					<p class="mt-3 text-sm" style="color: {probeOk ? 'var(--success)' : 'var(--destructive)'};">
						{probeMessage}
					</p>
				{/if}

				{#if caps}
					<div class="mt-4 rounded-lg border p-3 text-xs" style="border-color: var(--border);">
						<p class="font-semibold">{tr('Kapabilitas model terakhir diuji')}</p>
						<ul class="mt-2 space-y-1 text-muted-foreground">
							<li>
								JSON mode: <strong style="color: {caps.json_mode ? 'var(--success)' : 'var(--destructive)'};">{caps.json_mode ? 'OK' : 'GAGAL'}</strong>
								{#if caps.json_note}<span> · {caps.json_note}</span>{/if}
							</li>
							<li>
								Streaming JSON: <strong style="color: {streamFlag ? 'var(--success)' : 'var(--warning)'};">{streamFlag ? 'didukung' : 'tidak didukung'}</strong>
								{#if caps.stream_note}<span> · {caps.stream_note}</span>{/if}
							</li>
							{#if caps.model}<li>{tr('Model')}: {caps.model}</li>{/if}
							{#if caps.probed_at}<li>{tr('Diuji')}: {new Date(caps.probed_at).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' })}</li>{/if}
						</ul>
						{#if !streamFlag && caps.probed_at}
							<p class="mt-2" style="color: var(--warning);">
								{tr('Balasan akan tampil utuh setelah selesai, bukan mengetik bertahap. Jalur non-streaming dipakai otomatis.')}
							</p>
						{/if}
					</div>
				{/if}
			</section>
		{/if}

		<!-- ── PROMPT ────────────────────────────────────────────── -->
		{#if tab === 'prompt'}
			<section class="app-card mt-4 p-5">
				<h2 class="text-sm font-semibold">{tr('Prompt Brief Engine')}</h2>
				<p class="mt-1 text-xs text-muted-foreground">
					{tr('Prompt ini statis. Variabel per giliran (state brief, daftar field, dokumen) dikirim backend lewat pesan user, bukan disisipkan di sini.')}
				</p>

				<div class="mt-3 flex flex-wrap gap-1">
					{#each prompts as v (v.id)}
						<button
							type="button"
							class="app-btn !min-h-7 !px-2 !text-xs"
							class:app-btn-primary={v.id === activePromptId}
							class:app-btn-soft={v.id !== activePromptId}
							onclick={() => { activePromptId = v.id; void loadPromptDetail(v.id); }}
						>
							v{v.version} · {v.status}
						</button>
					{/each}
				</div>

				<textarea
					class="app-input mt-3 w-full font-mono text-xs"
					rows="18"
					bind:value={promptText}
					placeholder={tr('Instruksi Brief Engine…')}
				></textarea>

				<div class="mt-3 grid gap-3 sm:grid-cols-2">
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">{tr('Catatan perubahan')}</span>
						<input class="app-input mt-1 w-full" maxlength={255} bind:value={promptNote} />
					</label>
				</div>

				<div class="mt-4 flex flex-wrap items-center gap-2">
					<button class="app-btn app-btn-primary" type="button" disabled={busy || !activePromptId} onclick={() => void savePrompt()}>
						<Save size={14} />{tr('Simpan')}
					</button>
					<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => void newPromptVersion()}>
						<Plus size={14} />{tr('Versi baru (salin aktif)')}
					</button>
					{#if activePromptId}
						{@const active = prompts.find((v) => v.id === activePromptId)}
						{#if active && active.status !== 'active'}
							<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => void activatePrompt(active.id)}>
								<CheckCircle2 size={14} />{tr('Aktifkan versi ini')}
							</button>
						{/if}
						<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => void deletePrompt(activePromptId)}>
							<Trash2 size={14} />{tr('Hapus')}
						</button>
					{/if}
				</div>
			</section>

			<section class="app-card mt-4 p-4">
				<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{tr('Riwayat versi')}</p>
				<ul class="mt-2 space-y-1 text-xs text-muted-foreground">
					{#each prompts as v (v.id)}
						<li>v{v.version} · {v.status} · {fmtDateTime(v.created_at)}{v.change_note ? ` — ${v.change_note}` : ''}</li>
					{/each}
				</ul>
			</section>
		{/if}

		<!-- ── SKEMA FIELD ───────────────────────────────────────── -->
		{#if tab === 'fields'}
			<section class="app-card mt-4 p-5">
				<h2 class="text-sm font-semibold">{tr('Skema Field')}</h2>
				<p class="mt-1 text-xs text-muted-foreground">
					{tr('Urutan di sini adalah prioritas pertanyaan. Lapis 1 wajib dan dijaga ketat; lapis 2 kebanyakan terisi otomatis dari website/dokumen; lapis 3 diisi sistem dan tidak pernah ditanyakan ke AI.')}
				</p>

				<div class="mt-3 flex flex-wrap gap-1">
					{#each fieldVersions as v (v.id)}
						<button
							type="button"
							class="app-btn !min-h-7 !px-2 !text-xs"
							class:app-btn-primary={v.id === activeFieldVersionId}
							class:app-btn-soft={v.id !== activeFieldVersionId}
							onclick={() => { activeFieldVersionId = v.id; void loadFieldDetail(v.id); }}
						>
							v{v.version} · {v.status}
						</button>
					{/each}
				</div>

				<div class="mt-4 space-y-6">
					{#each [1, 2, 3] as layer (layer)}
						{@const rows = fields.filter((f) => f.layer === layer)}
						{#if layer !== 3 || rows.length > 0}
							<div>
								<div class="flex items-center justify-between">
									<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{layerLabel(layer)}</p>
									{#if layer !== 3}
										<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={() => addField(layer)}>
											<Plus size={12} />{tr('Tambah')}
										</button>
									{/if}
								</div>

								{#if layer === 3}
									<p class="mt-2 text-xs text-muted-foreground">
										{tr('Diisi sistem dari data pemakaian (kampanye, media, anchor). Tidak bisa diedit dan tidak pernah diminta ke AI.')}
									</p>
								{:else}
									<div class="mt-2 space-y-2">
										{#each rows as f (f.key + f.sort_order)}
											{@const idx = fields.indexOf(f)}
											<div class="grid items-center gap-2 rounded-lg border p-2 sm:grid-cols-[auto_1fr_1fr_auto_auto]" style="border-color: var(--border);">
												<div class="flex gap-1">
													<button class="app-btn app-btn-soft !min-h-7 !px-1.5 !text-xs" type="button" onclick={() => moveField(idx, -1)} aria-label={tr('Naikkan')}>↑</button>
													<button class="app-btn app-btn-soft !min-h-7 !px-1.5 !text-xs" type="button" onclick={() => moveField(idx, 1)} aria-label={tr('Turunkan')}>↓</button>
												</div>
												<label class="block">
													<span class="text-[11px] text-muted-foreground">key</span>
													<input class="app-input mt-0.5 w-full font-mono text-xs" bind:value={f.key} />
												</label>
												<label class="block">
													<span class="text-[11px] text-muted-foreground">{tr('Label')}</span>
													<input class="app-input mt-0.5 w-full text-xs" bind:value={f.label} />
												</label>
												<label class="block">
													<span class="text-[11px] text-muted-foreground">{tr('Seksi')}</span>
													<input class="app-input mt-0.5 w-full text-xs" bind:value={f.section} />
												</label>
												<div class="flex flex-col gap-1 text-xs">
													<label class="flex items-center gap-1">
														<input type="checkbox" bind:checked={f.required} />{tr('wajib')}
													</label>
													<label class="flex items-center gap-1">
														<input type="checkbox" bind:checked={f.enabled} />{tr('aktif')}
													</label>
												</div>
												<div class="sm:col-span-5">
													<label class="block">
														<span class="text-[11px] text-muted-foreground">{tr('Contoh pertanyaan (inspirasi untuk AI, tidak dibaca persis)')}</span>
														<input class="app-input mt-0.5 w-full text-xs" bind:value={f.example_question} />
													</label>
												</div>
											</div>
										{/each}
									</div>
								{/if}
							</div>
						{/if}
					{/each}
				</div>

				<label class="mt-4 block">
					<span class="text-xs font-medium text-muted-foreground">{tr('Catatan perubahan')}</span>
					<input class="app-input mt-1 w-full" maxlength={255} bind:value={fieldsNote} />
				</label>

				<div class="mt-4">
					<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void saveFieldVersion()}>
						<Save size={14} />{tr('Simpan sebagai versi baru')}
					</button>
				</div>
			</section>
		{/if}

		<!-- ── UJI ───────────────────────────────────────────────── -->
		{#if tab === 'uji'}
			<section class="app-card mt-4 p-5">
				<h2 class="text-sm font-semibold">{tr('Uji prompt & skema')}</h2>
				<p class="mt-1 text-xs text-muted-foreground">
					{tr('Menjalankan satu giliran terhadap state kosong dengan prompt dan skema yang terlihat di atas. Tidak menyentuh Profiler klien mana pun.')}
				</p>

				<label class="mt-3 block">
					<span class="text-xs font-medium text-muted-foreground">{tr('Skenario (pesan user)')}</span>
					<textarea class="app-input mt-1 w-full text-sm" rows="4" bind:value={scenario}
						placeholder={tr('Contoh: Tauthub itu platform PR, mirip seedbacklink. Aku mau brand-ku muncul di jawaban ChatGPT.')}></textarea>
				</label>

				<label class="mt-3 block">
					<span class="text-xs font-medium text-muted-foreground">{tr('Bahan (opsional) — teks website atau dokumen')}</span>
					<textarea class="app-input mt-1 w-full text-xs" rows="5" bind:value={material}
						placeholder={tr('Tempel isi halaman di sini untuk mensimulasikan material yang sudah dibaca sistem.')}></textarea>
				</label>

				<div class="mt-4">
					<button class="app-btn app-btn-primary" type="button" disabled={busy || !scenario.trim()} onclick={() => void runTest()}>
						<Play size={14} />{busy ? tr('Menjalankan…') : tr('Jalankan uji')}
					</button>
				</div>

				{#if testError}
					<p class="mt-3 text-sm" style="color: var(--destructive);">{testError}</p>
				{/if}
			</section>

			{#if testResult}
				{@const usage = testResult.usage ?? {}}
				<section class="app-card mt-4 p-5">
					<div class="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
						<span>fase: <strong>{testResult.phase}</strong></span>
						<span>asked_field: <strong>{testResult.asked_field ?? '—'}</strong></span>
						<span>ready: <strong>{testResult.ready_for_confirmation ? 'ya' : 'belum'}</strong></span>
						<span>{usage.model ?? ''} · in {usage.input ?? 0} / out {usage.output ?? 0} · {usage.durationMs ?? 0} ms</span>
						<span>{usage.streamed ? tr('streaming') : tr('non-streaming')}</span>
					</div>

					<div class="mt-4 rounded-lg border p-3" style="border-color: var(--border);">
						<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{tr('Balasan ke user')}</p>
						<p class="mt-2 whitespace-pre-wrap text-sm">{testResult.reply}</p>
					</div>

					{#if (testResult.field_updates ?? []).length > 0}
						<div class="mt-3">
							<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{tr('Yang akan disimpan')}</p>
							<ul class="mt-2 space-y-1 text-xs">
								{#each testResult.field_updates as u (u.key)}
									<li><code>{u.key}</code> = <span>{u.value}</span> <span class="text-muted-foreground">({u.status} · {u.source})</span></li>
								{/each}
							</ul>
						</div>
					{/if}

					{#if (testResult.rejected_updates ?? []).length > 0}
						<div class="mt-3">
							<p class="text-xs font-bold uppercase tracking-wide" style="color: var(--warning);">{tr('Ditolak validasi')}</p>
							<ul class="mt-2 space-y-1 text-xs text-muted-foreground">
								{#each testResult.rejected_updates as r (r)}<li>{r}</li>{/each}
							</ul>
						</div>
					{/if}

					{#if (testResult.suggested_values ?? []).length > 0}
						<div class="mt-3">
							<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{tr('Draft yang ditawarkan (belum disimpan)')}</p>
							<ul class="mt-2 space-y-1 text-xs text-muted-foreground">
								{#each testResult.suggested_values as s (s.key)}
									<li><code>{s.key}</code> — {s.value}</li>
								{/each}
							</ul>
						</div>
					{/if}
				</section>
			{/if}
		{/if}

		<!-- ── LOG ───────────────────────────────────────────────── -->
		{#if tab === 'log'}
			<section class="app-card mt-4 p-5">
				<div class="flex items-center justify-between">
					<h2 class="text-sm font-semibold">{tr('Log pemakaian')}</h2>
					<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={() => void loadLogs()}>
						<RefreshCw size={13} />{tr('Muat ulang')}
					</button>
				</div>

				{#if logs.length === 0}
					<p class="mt-3 text-sm text-muted-foreground">{tr('Belum ada pemakaian.')}</p>
				{:else}
					<div class="mt-3 overflow-x-auto">
						<table class="w-full text-left text-xs">
							<thead class="text-muted-foreground">
								<tr>
									<th class="py-1 pr-3">Waktu</th>
									<th class="py-1 pr-3">Brand</th>
									<th class="py-1 pr-3">Profiler</th>
									<th class="py-1 pr-3">Agent</th>
									<th class="py-1 pr-3">Status</th>
									<th class="py-1 pr-3">Model</th>
									<th class="py-1 pr-3">Token</th>
									<th class="py-1 pr-3">Durasi</th>
								</tr>
							</thead>
							<tbody>
								{#each logs as r (r.id)}
									<tr class="border-t" style="border-color: var(--border);">
										<td class="py-1.5 pr-3 whitespace-nowrap">{fmtDateTime(r.created_at)}</td>
										<td class="py-1.5 pr-3">{r.brand_name || '—'}</td>
										<td class="py-1.5 pr-3">{r.profile_name || '—'}</td>
										<td class="py-1.5 pr-3">{r.agent || 'chat'}</td>
										<td class="py-1.5 pr-3">
											<span style="color: {r.status === 'done' ? 'var(--success)' : r.status === 'failed' ? 'var(--destructive)' : 'var(--muted-foreground)'};">{r.status}</span>
											{#if r.error}<span class="ml-1 text-muted-foreground" title={r.error}>·</span>{/if}
										</td>
										<td class="py-1.5 pr-3">{r.model ?? '—'}</td>
										<td class="py-1.5 pr-3">{r.token_input ?? 0} / {r.token_output ?? 0}</td>
										<td class="py-1.5 pr-3">{r.duration_ms ?? 0} ms</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</section>
		{/if}
	{/if}
	</ExplorerFrame>
</section>
