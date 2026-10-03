<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Konfigurasi provider AI untuk generate gambar Taut Pintar Writer.
	// Endpoint sama dengan halaman AI Gateway (/v1/platform/ai/configuration),
	// tapi form di sini hanya mengurus kolom gambar. Field chat (base_url +
	// default_model) tetap dikirim apa adanya karena wajib oleh API.
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
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
		} catch {
			testResult = { ok: false, message: 'Tidak dapat terhubung ke server API.' };
		} finally {
			testing = false;
		}
	}
</script>

<section>
	<PageHeader
		title="AI Gambar — Konfigurasi"
		description="Atur provider AI untuk generate gambar (cover & gambar per H2) di Taut Pintar Writer."
	/>

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
							Base: {testResult.base_url ?? '—'} · Model: {testResult.model ?? '—'}
						</p>
					</div>
				{/if}
			</form>
		</div>
	{/if}
</section>
