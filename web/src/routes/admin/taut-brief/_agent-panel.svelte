<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Panel konfigurasi satu agent ringkasan Brief: koneksi/model + prompt
	// berversi. Dipakai dua kali (agent entitas & agent file) supaya bentuk UI
	// dan aturannya persis sama.
	import { onMount } from 'svelte';
	import { CheckCircle2, Play, Plus, Save, Trash2, TriangleAlert } from '@lucide/svelte';
	import { apiRequest, readApiError } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';

	type Props = {
		agent: string;
		title: string;
		description: string;
		disabledHint?: string;
	};
	let { agent, title, description, disabledHint }: Props = $props();

	const BASE = '/v1/platform/taut-brief';

	type AgentConfig = {
		base_url: string;
		model: string;
		temperature: number;
		max_tokens: number | null;
		timeout_ms: number;
		enabled: boolean;
		api_key_configured: boolean;
		effective: { base_url: string; model: string; configured: boolean; inherited: boolean };
	};

	type VersionMeta = {
		id: string;
		version: number;
		status: string;
		change_note: string | null;
		created_at: string;
		published_at: string | null;
	};

	let config = $state<AgentConfig | null>(null);
	let form = $state({
		base_url: '',
		api_key: '',
		model: '',
		temperature: 0.2,
		max_tokens: '',
		timeout_ms: 120000,
		enabled: true
	});

	let prompts = $state<VersionMeta[]>([]);
	let promptId = $state('');
	let promptText = $state('');
	let promptNote = $state('');

	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');
	let probeOk = $state(false);
	let probeMessage = $state('');

	const base = $derived(`${BASE}/agents/${agent}`);

	onMount(() => {
		void loadAll();
	});

	async function loadAll() {
		loading = true;
		error = '';
		try {
			await Promise.all([loadConfig(), loadPrompts()]);
		} finally {
			loading = false;
		}
	}

	async function loadConfig() {
		const res = await apiRequest(`${base}/config`);
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal memuat konfigurasi agent.'));
			return;
		}
		const data: AgentConfig = await res.json();
		config = data;
		form = {
			base_url: data?.base_url ?? '',
			api_key: '',
			model: data?.model ?? '',
			temperature: data?.temperature ?? 0.2,
			max_tokens: data?.max_tokens ? String(data.max_tokens) : '',
			timeout_ms: data?.timeout_ms ?? 120000,
			enabled: data?.enabled ?? true
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
				enabled: form.enabled
			};
			if (form.api_key.trim()) payload.api_key = form.api_key.trim();
			if (String(form.max_tokens).trim()) payload.max_tokens = Number(form.max_tokens);
			const res = await apiRequest(`${base}/config`, { method: 'PUT', body: JSON.stringify(payload) });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan konfigurasi agent.')));
			notice = tr('Konfigurasi agent tersimpan.');
			await loadConfig();
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal menyimpan konfigurasi agent.');
		} finally {
			busy = false;
		}
	}

	async function probe() {
		busy = true;
		probeMessage = '';
		try {
			const res = await apiRequest(`${base}/config/test`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, tr('Uji koneksi gagal.')));
			const data = await res.json();
			probeOk = Boolean(data?.ok);
			probeMessage = String(data?.message ?? '');
		} catch (e) {
			probeOk = false;
			probeMessage = e instanceof Error ? e.message : tr('Uji koneksi gagal.');
		} finally {
			busy = false;
		}
	}

	async function loadPrompts() {
		const res = await apiRequest(`${base}/prompts`);
		if (!res.ok) return;
		const data = await res.json();
		prompts = data?.items ?? [];
		const active = prompts.find((v) => v.status === 'active') ?? prompts[0];
		if (active) {
			promptId = active.id;
			await loadPromptDetail(active.id);
		}
	}

	async function loadPromptDetail(id: string) {
		const res = await apiRequest(`${base}/prompts/${id}`);
		if (!res.ok) return;
		const data = await res.json();
		promptText = data?.version?.prompt ?? '';
		promptNote = data?.version?.change_note ?? '';
	}

	async function savePrompt() {
		if (!promptId) return;
		busy = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`${base}/prompts/${promptId}`, {
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

	async function newVersion() {
		busy = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`${base}/prompts`, {
				method: 'POST',
				body: JSON.stringify({ clone_active: true, change_note: promptNote })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal membuat versi prompt.')));
			notice = tr('Versi prompt baru dibuat & langsung aktif.');
			await loadPrompts();
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal membuat versi prompt.');
		} finally {
			busy = false;
		}
	}

	async function activatePrompt(id: string) {
		busy = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`${base}/prompts/${id}/activate`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal mengaktifkan prompt.')));
			notice = tr('Versi prompt diaktifkan.');
			await loadPrompts();
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal mengaktifkan prompt.');
		} finally {
			busy = false;
		}
	}

	async function deletePrompt(id: string) {
		busy = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`${base}/prompts/${id}`, { method: 'DELETE' });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menghapus prompt.')));
			notice = tr('Versi prompt dihapus.');
			await loadPrompts();
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal menghapus prompt.');
		} finally {
			busy = false;
		}
	}
</script>

{#if loading}
	<p class="mt-4 text-sm text-muted-foreground">{tr('Memuat konfigurasi agent…')}</p>
{:else}
	{#if error}
		<p class="mt-4 rounded-lg p-2 text-sm" style="background: color-mix(in srgb, var(--destructive) 8%, white); color: var(--destructive);">{error}</p>
	{/if}
	{#if notice}
		<p class="mt-4 rounded-lg p-2 text-sm" style="background: var(--success-soft); color: var(--success);">{notice}</p>
	{/if}

	<section class="app-card mt-4 p-5">
		<h2 class="text-sm font-semibold">{title}</h2>
		<p class="mt-1 text-xs text-muted-foreground">{description}</p>

		{#if config?.effective?.inherited}
			<p class="mt-3 rounded-lg p-2 text-xs" style="background: var(--secondary); color: var(--muted-foreground);">
				{tr('Sebagian koneksi masih mewarisi config Taut Brief')}: {config.effective.base_url || '—'} · {config.effective.model || '—'}
			</p>
		{/if}

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
				<span class="text-[11px] text-muted-foreground">{tr('Hasil sebaiknya deterministik (0.1-0.3).')}</span>
			</label>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Max tokens</span>
				<input class="app-input mt-1 w-full" type="number" placeholder={tr('opsional')} bind:value={form.max_tokens} />
				<span class="text-[11px] text-muted-foreground">{tr('Batas panjang hasil. Kosongkan untuk bawaan provider.')}</span>
			</label>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Timeout (ms)</span>
				<input class="app-input mt-1 w-full" type="number" bind:value={form.timeout_ms} />
			</label>
		</div>

		<label class="mt-4 flex items-center gap-2 text-sm">
			<input type="checkbox" bind:checked={form.enabled} />
			{tr('Aktifkan agent ini')}
		</label>

		{#if !form.enabled}
			<p class="mt-3 flex items-start gap-2 text-xs" style="color: var(--warning);">
				<TriangleAlert size={14} class="mt-0.5 shrink-0" />
				{disabledHint ?? tr('Selama nonaktif, ringkasan tidak diperbarui dan Writer mode proper akan menolak brief ini.')}
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
			<p class="mt-3 text-sm" style="color: {probeOk ? 'var(--success)' : 'var(--destructive)'};">{probeMessage}</p>
		{/if}
	</section>

	<section class="app-card mt-4 p-5">
		<h2 class="text-sm font-semibold">{tr('Prompt agent')}</h2>
		<p class="mt-1 text-xs text-muted-foreground">
			{tr('Prompt statis. Bahan dikirim backend sebagai pesan user setiap kali agent dipanggil.')}
		</p>

		<div class="mt-3 flex flex-wrap gap-1">
			{#each prompts as v (v.id)}
				<button
					type="button"
					class="app-btn !min-h-7 !px-2 !text-xs"
					class:app-btn-primary={v.id === promptId}
					class:app-btn-soft={v.id !== promptId}
					onclick={() => {
						promptId = v.id;
						void loadPromptDetail(v.id);
					}}
				>
					v{v.version} · {v.status}
				</button>
			{/each}
		</div>

		<textarea
			class="app-input mt-3 w-full font-mono text-xs"
			rows="16"
			bind:value={promptText}
				placeholder={tr('Instruksi agent…')}
		></textarea>

		<div class="mt-3 grid gap-3 sm:grid-cols-2">
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">{tr('Catatan perubahan')}</span>
				<input class="app-input mt-1 w-full" maxlength={255} bind:value={promptNote} />
			</label>
		</div>

		<div class="mt-4 flex flex-wrap items-center gap-2">
			<button class="app-btn app-btn-primary" type="button" disabled={busy || !promptId} onclick={() => void savePrompt()}>
				<Save size={14} />{tr('Simpan')}
			</button>
			<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => void newVersion()}>
				<Plus size={14} />{tr('Versi baru (salin aktif)')}
			</button>
			{#if promptId}
				{@const current = prompts.find((v) => v.id === promptId)}
				{#if current && current.status !== 'active'}
					<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => void activatePrompt(current.id)}>
						<CheckCircle2 size={14} />{tr('Aktifkan versi ini')}
					</button>
				{/if}
				<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => void deletePrompt(promptId)}>
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
