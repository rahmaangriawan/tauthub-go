<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Konfigurasi provider AI untuk generate gambar Taut Pintar Writer, plus tab
	// Log & Monitoring (hasil generate + sebab error/fallback placeholder).
	// Konfigurasi memakai endpoint AI Gateway (/v1/platform/ai/configuration),
	// form di sini hanya mengurus kolom gambar. Field chat (base_url +
	// default_model) tetap dikirim apa adanya karena wajib oleh API.
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type AiConfig = {
		base_url: string;
		default_model: string;
		economy_model: string;
		premium_model: string;
		image_base_url: string;
		image_model: string;
		image_api_key_configured: boolean;
		image_api_key_hint?: string | null;
		image_credit_cost: number;
	};

	type LogEvent = {
		id: string;
		event_kind: string;
		status: string;
		provider: string | null;
		model: string | null;
		base_url: string | null;
		error_detail: string | null;
		prompt: string | null;
		role: string | null;
		brand_name: string | null;
		campaign_name: string | null;
		media_name: string | null;
		width: number;
		height: number;
		credits_charged: number;
		elapsed_ms: number;
		image_url: string | null;
		created_at: string | null;
	};

	type LogsResponse = {
		summary: { total: number; succeeded: number; fallback: number; failed: number; hours: number };
		top_errors: Array<{ detail: string; total: number; last_at: string | null }>;
		events: LogEvent[];
	};

	let tab = $state<'config' | 'logs'>('config');

	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');
	let config = $state<AiConfig | null>(null);

	let formBaseUrl = $state('');
	let formModel = $state('');
	let formApiKey = $state('');
	let apiKeyTouched = $state(false);

	// Nilai chat yang tersimpan — dikirim balik saat PUT supaya tidak tertimpa.
	let chat = $state({ base_url: '', default_model: '', economy_model: '', premium_model: '' });

	// Uji koneksi provider gambar (debugging) — menampilkan error asli provider.
	let testing = $state(false);
	let testResult = $state<{
		ok: boolean;
		configured?: boolean;
		message: string;
		base_url?: string;
		model?: string;
		width?: number;
		height?: number;
		bytes?: number;
		elapsed_ms?: number;
	} | null>(null);

	// Tab Log & Monitoring.
	let logs = $state<LogsResponse | null>(null);
	let logsLoading = $state(false);
	let logsError = $state('');
	let logsLoaded = $state(false);
	let logStatus = $state('');
	let logHours = $state('168');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) {
				window.location.href = '/login';
				return;
			}
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		error = '';
		try {
			const res = await apiRequest('/v1/platform/ai/configuration');
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			const data = (await res.json()) as AiConfig;
			config = data;
			chat = {
				base_url: data.base_url ?? '',
				default_model: data.default_model ?? '',
				economy_model: data.economy_model ?? '',
				premium_model: data.premium_model ?? ''
			};
			formBaseUrl = data.image_base_url ?? '';
			formModel = data.image_model ?? '';
			formApiKey = '';
			apiKeyTouched = false;
		} catch {
			error = tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		}
	}

	async function loadLogs() {
		logsLoading = true;
		logsError = '';
		try {
			const qs = new URLSearchParams({ hours: logHours, limit: '200' });
			if (logStatus) qs.set('status', logStatus);
			const res = await apiRequest(`/v1/platform/ai/image-logs?${qs.toString()}`);
			if (!res.ok) {
				logsError = await readApiError(res, 'Gagal memuat log generate gambar.');
				return;
			}
			logs = (await res.json()) as LogsResponse;
			logsLoaded = true;
		} catch {
			logsError = 'Tidak dapat terhubung ke server API.';
		} finally {
			logsLoading = false;
		}
	}

	function selectTab(next: 'config' | 'logs') {
		tab = next;
		if (next === 'logs' && !logsLoaded) void loadLogs();
	}

	function reloadLogs() {
		void loadLogs();
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		if (busy) return;
		if (!chat.base_url.trim()) {
			error = tr('Konfigurasikan AI chat dulu (Base URL + Model) di menu AI Gateway sebelum mengatur gambar.');
			return;
		}
		busy = true;
		notice = '';
		error = '';
		try {
			const res = await apiRequest('/v1/platform/ai/configuration', {
				method: 'PUT',
				body: JSON.stringify({
					base_url: chat.base_url,
					default_model: chat.default_model,
					economy_model: chat.economy_model || undefined,
					premium_model: chat.premium_model || undefined,
					image_base_url: formBaseUrl.trim(),
					image_model: formModel.trim(),
					image_api_key: apiKeyTouched ? formApiKey.trim() : undefined
				})
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal menyimpan konfigurasi.');
				return;
			}
			notice = tr('Konfigurasi generate gambar berhasil disimpan.');
			await load();
		} catch {
			error = 'Gagal menyimpan konfigurasi.';
		} finally {
			busy = false;
		}
	}

	/** Uji koneksi provider gambar tanpa fallback placeholder. */
	async function testConnection() {
		if (testing) return;
		testing = true;
		testResult = null;
		try {
			// Simpan dulu perubahan yang sedang diketik supaya tes memakai nilai terbaru.
			if (formBaseUrl.trim() !== (config?.image_base_url ?? '') || formModel.trim() !== (config?.image_model ?? '') || apiKeyTouched) {
				await save(new Event('submit') as unknown as SubmitEvent);
			}
			const res = await apiRequest('/v1/platform/ai/image-test', { method: 'POST' }, 180_000);
			const data = await res.json().catch(() => null);
			if (!res.ok) {
				testResult = { ok: false, message: await readApiError(res, 'Gagal menghubungi provider gambar.') };
				return;
			}
			testResult = data;
			// Uji koneksi ikut tercatat sebagai event; segarkan log bila tab pernah dibuka.
			if (logsLoaded) void loadLogs();
		} catch {
			testResult = { ok: false, message: 'Tidak dapat terhubung ke server API.' };
		} finally {
			testing = false;
		}
	}

	const statusTone = (s: string): 'success' | 'warning' | 'danger' =>
		s === 'succeeded' ? 'success' : s === 'fallback' ? 'warning' : 'danger';
	const statusLabel = (s: string) => (s === 'succeeded' ? 'Sukses' : s === 'fallback' ? 'Placeholder' : s === 'failed' ? 'Gagal' : s);
	const fmtDateTime = (v: string | null) => (v ? new Date(v).toLocaleString('id-ID') : '-');
	const scopeOf = (e: LogEvent) =>
		[e.brand_name, e.campaign_name, e.media_name].filter(Boolean).join(' / ') || '-';
	const fallbackRate = $derived.by(() => {
		const s = logs?.summary;
		if (!s || s.total <= 0) return 0;
		return Math.round(((s.fallback + s.failed) / s.total) * 100);
	});
</script>

{#snippet timeCell(item: LogEvent)}
	<p class="whitespace-nowrap text-xs">{fmtDateTime(item.created_at)}</p>
	{#if item.event_kind === 'connection_test'}<p class="text-[11px] text-muted-foreground">Tes koneksi</p>{/if}
{/snippet}

{#snippet statusCell(item: LogEvent)}
	<Badge tone={statusTone(item.status)} label={statusLabel(item.status)} />
{/snippet}

{#snippet scopeCell(item: LogEvent)}
	<p class="text-xs">{scopeOf(item)}</p>
	{#if item.role}<p class="text-[11px] text-muted-foreground">{item.role === 'cover' ? 'Cover' : 'Inline'}</p>{/if}
{/snippet}

{#snippet sourceCell(item: LogEvent)}
	<p class="text-xs">{item.model ?? '-'}</p>
	<p class="text-[11px] text-muted-foreground">{item.provider ?? '-'}{item.base_url ? ` · ${item.base_url}` : ''}</p>
{/snippet}

{#snippet resultCell(item: LogEvent)}
	{#if item.image_url}
		<img class="h-9 w-16 rounded object-cover" src={item.image_url} alt="" loading="lazy" />
	{:else}
		<span class="text-xs text-muted-foreground">-</span>
	{/if}
{/snippet}

{#snippet detailCell(item: LogEvent)}
	<div class="max-w-[340px]">
		{#if item.error_detail}
			<p class="text-xs" style="color: var(--destructive);" title={item.error_detail}>{item.error_detail}</p>
		{:else}
			<p class="truncate text-xs text-muted-foreground" title={item.prompt ?? ''}>{item.prompt ?? '-'}</p>
		{/if}
	</div>
{/snippet}

{#snippet metricsCell(item: LogEvent)}
	<p class="text-xs tabular-nums">{item.width}×{item.height}px</p>
	<p class="text-[11px] tabular-nums text-muted-foreground">{item.credits_charged} kredit · {item.elapsed_ms} ms</p>
{/snippet}

<section>
	<PageHeader
		title="AI Gambar"
		description="Konfigurasi provider generate gambar (cover & gambar per H2) Taut Pintar Writer, plus log hasil dan monitoring error."
	/>

	<div class="mt-4 flex flex-wrap gap-1 border-b" role="tablist">
		<button
			type="button"
			class="app-btn !min-h-8 !rounded-b-none !px-3.5 !text-sm"
			class:app-btn-primary={tab === 'config'}
			class:app-btn-soft={tab !== 'config'}
			onclick={() => selectTab('config')}>Konfigurasi</button
		>
		<button
			type="button"
			class="app-btn !min-h-8 !rounded-b-none !px-3.5 !text-sm"
			class:app-btn-primary={tab === 'logs'}
			class:app-btn-soft={tab !== 'logs'}
			onclick={() => selectTab('logs')}>{tr('Log & Monitoring')}</button
		>
	</div>

	<div class="pt-4">
		{#if tab === 'config'}
			<Feedback {loading} {error} loadingText="Memuat konfigurasi AI gambar…" />

			{#if notice}
				<div class="app-card mt-4 p-3" style="border-color: var(--success);">
					<p class="text-sm" style="color: var(--success);">{notice}</p>
				</div>
			{/if}

			{#if !loading}
				<div class="app-card mt-4 p-4">
					<div class="flex flex-wrap items-start justify-between gap-3">
						<div>
							<h2 class="text-base font-semibold">Provider Generate Gambar</h2>
							<p class="mt-1 max-w-2xl text-sm text-muted-foreground">
								Mendukung endpoint OpenAI-compatible <code class="rounded bg-secondary px-1">POST /images/generations</code> (mis. OpenAI),
								Pollinations (gratis, tanpa key), dan OpenRouter <code class="rounded bg-secondary px-1">POST /api/v1/images</code>.
								Contoh OpenRouter + Seedream: Base URL <code class="rounded bg-secondary px-1">https://openrouter.ai/api/v1</code>,
								Model <code class="rounded bg-secondary px-1">bytedance-seed/seedream-5-0-flash</code>, API key <code class="rounded bg-secondary px-1">sk-or-...</code>.
								Kolom yang dikosongkan berarti mengikuti konfigurasi AI chat.
								<strong>Model gambar wajib diisi</strong> agar gambar AI asli ter-generate — kalau kosong sistem memakai placeholder lokal.
							</p>
						</div>
						<div class="flex items-center gap-2">
							<Badge tone={config?.image_model ? 'success' : 'neutral'} label={config?.image_model ? 'Image siap' : 'Placeholder'} />
							<Badge tone="accent" label="{config?.image_credit_cost ?? 2} Kredit / gambar" />
						</div>
					</div>

					<form class="mt-4" onsubmit={save}>
						<div class="grid gap-4 md:grid-cols-3">
							<label class="block">
								<span class="text-sm font-medium">Base URL gambar</span>
								<input
									class="app-input mt-1 w-full"
									type="url"
									placeholder="https://openrouter.ai/api/v1"
									value={formBaseUrl}
									oninput={(e) => (formBaseUrl = (e.currentTarget as HTMLInputElement).value)}
								/>
							</label>
							<label class="block">
								<span class="text-sm font-medium">API Key gambar</span>
								<input
									class="app-input mt-1 w-full"
									type="password"
									autocomplete="new-password"
									placeholder={config?.image_api_key_configured
										? `${config.image_api_key_hint ?? '••••'} · (kosongkan jika tidak diganti)`
										: 'Biarkan kosong = pakai API key AI chat'}
									value={formApiKey}
									oninput={(e) => {
										formApiKey = (e.currentTarget as HTMLInputElement).value;
										apiKeyTouched = true;
									}}
								/>
							</label>
							<label class="block">
								<span class="text-sm font-medium">Model gambar</span>
								<input
									class="app-input mt-1 w-full"
									maxlength={200}
									placeholder="bytedance-seed/seedream-5-0-flash"
									value={formModel}
									oninput={(e) => (formModel = (e.currentTarget as HTMLInputElement).value)}
								/>
							</label>
						</div>

						<p class="mt-3 text-xs text-muted-foreground">
							Biaya {config?.image_credit_cost ?? 2} Kredit per gambar dipotong dari saldo brand (top-up klien). Ubah tarif lewat
							env <code class="rounded bg-secondary px-1">AI_IMAGE_CREDIT_COST</code> bila perlu.
						</p>

						<div class="mt-4 flex flex-wrap items-center justify-between gap-3">
							<p class="text-xs text-muted-foreground">
								Kosong = ikut AI chat. Base URL tanpa isian memakai Base URL AI chat.
							</p>
							<div class="flex items-center gap-2">
								<button class="app-btn app-btn-soft" type="button" disabled={testing || busy} onclick={testConnection}>
									{testing ? 'Menguji…' : tr('Tes Koneksi')}
								</button>
								<button class="app-btn app-btn-primary" type="submit" disabled={busy}>
									{busy ? 'Menyimpan…' : tr('Simpan konfigurasi')}
								</button>
							</div>
						</div>

						{#if testResult}
							<div
								class="mt-3 rounded-xl border p-3 text-xs"
								style="border-color: {testResult.ok ? 'var(--success)' : 'var(--destructive)'}; background-color: {testResult.ok ? 'var(--success-soft, #eafaf1)' : 'var(--destructive-soft, #fef2f2)'};"
							>
								<p class="flex items-center gap-1.5 font-bold" style="color: {testResult.ok ? 'var(--success)' : 'var(--destructive)'};">
									{testResult.ok ? '✓ Koneksi OK' : '✕ Gagal'}
								</p>
								<p class="mt-1" style="color: var(--foreground);">{testResult.message}</p>
								{#if testResult.ok}
									<p class="mt-1 text-muted-foreground">
										{testResult.width}×{testResult.height}px · {Math.round((testResult.bytes ?? 0) / 1024)} KB · {testResult.elapsed_ms} ms
									</p>
								{/if}
								<p class="mt-1 text-muted-foreground">
									Base: {testResult.base_url ?? '-'} · Model: {testResult.model ?? '-'}
								</p>
							</div>
						{/if}
					</form>
				</div>
			{/if}
		{:else}
			<div class="mb-3 flex flex-wrap items-end gap-3">
				<label class="block">
					<span class="text-xs text-muted-foreground">Status</span>
					<select
						class="app-input mt-1"
						value={logStatus}
						onchange={(e) => {
							logStatus = (e.currentTarget as HTMLSelectElement).value;
							reloadLogs();
						}}
					>
						<option value="">Semua</option>
						<option value="succeeded">Sukses</option>
						<option value="fallback">Placeholder (fallback)</option>
						<option value="failed">Gagal</option>
					</select>
				</label>
				<label class="block">
					<span class="text-xs text-muted-foreground">Rentang</span>
					<select
						class="app-input mt-1"
						value={logHours}
						onchange={(e) => {
							logHours = (e.currentTarget as HTMLSelectElement).value;
							reloadLogs();
						}}
					>
						<option value="24">24 jam</option>
						<option value="168">7 hari</option>
						<option value="720">30 hari</option>
					</select>
				</label>
				<button class="app-btn app-btn-soft !min-h-9 !px-3.5 !text-xs" type="button" disabled={logsLoading} onclick={reloadLogs}>
					{logsLoading ? 'Memuat…' : tr('Muat ulang')}
				</button>
			</div>

			{#if logsError}
				<div class="app-card mb-3 p-3 text-sm" style="border-color: var(--destructive); color: var(--destructive);">{logsError}</div>
			{/if}

			<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
				<div class="app-card p-3">
					<p class="text-xs text-muted-foreground">Total percobaan</p>
					<p class="text-xl font-semibold tabular-nums">{logs?.summary.total ?? 0}</p>
					<p class="text-[11px] text-muted-foreground">{logHours === '24' ? '24 jam' : logHours === '720' ? '30 hari' : '7 hari'} terakhir</p>
				</div>
				<div class="app-card p-3">
					<p class="text-xs text-muted-foreground">Sukses</p>
					<p class="text-xl font-semibold tabular-nums" style="color: var(--success);">{logs?.summary.succeeded ?? 0}</p>
				</div>
				<div class="app-card p-3">
					<p class="text-xs text-muted-foreground">Placeholder (fallback)</p>
					<p class="text-xl font-semibold tabular-nums" style="color: var(--warning);">{logs?.summary.fallback ?? 0}</p>
				</div>
				<div class="app-card p-3">
					<p class="text-xs text-muted-foreground">Gagal</p>
					<p class="text-xl font-semibold tabular-nums" style="color: {logs && logs.summary.failed > 0 ? 'var(--destructive)' : 'inherit'};">{logs?.summary.failed ?? 0}</p>
					<p class="text-[11px] text-muted-foreground">{fallbackRate}% tidak memakai gambar AI asli</p>
				</div>
			</div>

			<div class="app-card mt-4 p-4">
				<h3 class="text-sm font-semibold">Monitoring error</h3>
				<p class="mt-1 text-[11.5px] text-muted-foreground">Penyebab paling sering saat gambar gagal atau jatuh ke placeholder.</p>
				{#if logs?.top_errors?.length}
					<ul class="mt-3 space-y-2">
						{#each logs?.top_errors ?? [] as item, i (i)}
							<li class="flex items-start gap-2 text-xs">
								<span class="mt-0.5 inline-flex shrink-0 items-center rounded-full px-2 py-0.5 text-[10px] font-bold" style="background-color: var(--destructive-soft, #fdecee); color: var(--destructive);">{item.total}×</span>
								<div class="min-w-0 flex-1">
									<p class="break-words" style="color: var(--foreground);">{item.detail}</p>
									<p class="text-[11px] text-muted-foreground">Terakhir: {fmtDateTime(item.last_at)}</p>
								</div>
							</li>
						{/each}
					</ul>
				{:else}
					<p class="mt-2 text-xs text-muted-foreground">Belum ada error pada rentang ini.</p>
				{/if}
			</div>

			<DataTable
				columns={[
					{ key: 'created_at', label: 'Waktu', render: timeCell },
					{ key: 'status', label: 'Status', render: statusCell },
					{ key: 'scope', label: 'Brand / Kampanye / Media', render: scopeCell },
					{ key: 'source', label: 'Peran & Model', render: sourceCell },
					{ key: 'result', label: 'Hasil', render: resultCell },
					{ key: 'detail', label: 'Keterangan (apa & kenapa)', render: detailCell },
					{ key: 'metrics', label: 'Biaya & Durasi', align: 'right' as const, render: metricsCell }
				]}
				rows={logs?.events ?? []}
				rowKey={(item: LogEvent) => item.id}
				loading={logsLoading}
				empty={tr('Belum ada log generate gambar pada rentang ini.')}
				minWidth="1100px"
			/>
		{/if}
	</div>
</section>
