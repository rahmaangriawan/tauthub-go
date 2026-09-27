<script lang="ts">
	// Kualitas Konten Brand — atur voice, tone, aturan editorial, dan profil per
	// brand. Dikelola admin, bukan klien.
	// Port apps/web/src/app/admin/content-settings/page.tsx.
	import { onMount } from 'svelte';
	import { Save } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type Brand = { id: string; name: string; slug: string; owner_email: string };
	type Config = {
		profile: Record<string, unknown> | null;
		voice: Record<string, unknown> | null;
		editorial_rules: Record<string, unknown> | null;
		prompts: Array<{ id: string; version: number; content: string; status: string }>;
	};

	let brands = $state<Brand[]>([]);
	let selectedId = $state('');
	let config = $state<Config | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state('');
	let saveMsg = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await loadBrands();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function loadBrands() {
		try {
			const res = await apiRequest('/v1/platform/operations/brands');
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			brands = await res.json();
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	let lastLoadedId = '';
	$effect(() => {
		if (selectedId && selectedId !== lastLoadedId) {
			lastLoadedId = selectedId;
			void loadConfig(selectedId);
		}
	});

	async function loadConfig(brandId: string) {
		if (!brandId) return;
		loading = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/operations/brands/${brandId}/content-settings`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			config = await res.json();
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	function toArray(v: unknown): string[] {
		if (Array.isArray(v)) return v.map((x) => String(x));
		if (typeof v === 'string')
			return v
				.split(',')
				.map((s) => s.trim())
				.filter(Boolean);
		return [];
	}

	function setVoice(key: string, value: unknown) {
		if (!config) return;
		config = { ...config, voice: { ...(config.voice ?? {}), [key]: value } };
	}
	function setRules(key: string, value: unknown) {
		if (!config) return;
		config = { ...config, editorial_rules: { ...(config.editorial_rules ?? {}), [key]: value } };
	}
	function setProfile(key: string, value: unknown) {
		if (!config) return;
		config = { ...config, profile: { ...(config.profile ?? {}), [key]: value } };
	}

	async function save() {
		if (!selectedId || !config) return;
		saving = true;
		saveMsg = '';
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/operations/brands/${selectedId}/content-settings`, {
				method: 'PUT',
				body: JSON.stringify({
					voice: {
						personality: toArray(config.voice?.personality),
						formality: Number(config.voice?.formality ?? 3),
						enthusiasm: Number(config.voice?.enthusiasm ?? 3),
						directness: Number(config.voice?.directness ?? 3),
						humor: Number(config.voice?.humor ?? 1),
						point_of_view: String(config.voice?.point_of_view ?? 'second_person'),
						preferred_terms: toArray(config.voice?.preferred_terms),
						banned_terms: toArray(config.voice?.banned_terms),
						cta_style: String(config.voice?.cta_style ?? '')
					},
					editorial_rules: {
						required_rules: toArray(config.editorial_rules?.required_rules),
						forbidden_rules: toArray(config.editorial_rules?.forbidden_rules),
						required_disclaimer: String(config.editorial_rules?.required_disclaimer ?? ''),
						citation_policy: String(config.editorial_rules?.citation_policy ?? '')
					},
					profile: {
						positioning: String(config.profile?.positioning ?? ''),
						value_proposition: String(config.profile?.value_proposition ?? ''),
						target_audience: String(config.profile?.target_audience ?? ''),
						products_services: String(config.profile?.products_services ?? ''),
						differentiation: String(config.profile?.differentiation ?? ''),
						default_cta: String(config.profile?.default_cta ?? ''),
						default_target_words:
							config.profile?.default_target_words != null ? Number(config.profile.default_target_words) : 1000
					}
				})
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			saveMsg = 'Pengaturan konten tersimpan.';
		} finally {
			saving = false;
		}
	}

	const sliderKeys = ['formality', 'enthusiasm', 'directness', 'humor'] as const;
</script>

<section class="mx-auto max-w-5xl">
	<PageHeader
		title="Kualitas Konten Brand"
		description="Atur voice, tone, aturan editorial, dan profil per brand — dikelola admin, bukan klien."
	>
		<Badge tone="accent" value={`${brands.length} brand`} />
	</PageHeader>

	<Feedback {loading} {error} notice={saveMsg} />

	<div class="app-card mt-4 p-4">
		<div class="flex flex-col gap-3">
			<label class="block">
				<span class="text-sm font-medium">Pilih Brand</span>
				<Select
					ariaLabel="Pilih brand"
					placeholder="Pilih brand…"
					minWidth={280}
					value={selectedId}
					onchange={(v) => (selectedId = v)}
					options={[
						{ value: '', label: 'Pilih brand…' },
						...brands.map((b) => ({ value: b.id, label: `${b.name} (${b.slug})` }))
					]}
				/>
			</label>
		</div>
	</div>

	{#if selectedId && config}
		<div class="mt-3 flex flex-col gap-3">
			<div class="app-card p-4">
				<h2 class="mb-3 text-base font-semibold">Voice &amp; Tone</h2>
				<div class="flex flex-col gap-2">
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Personality (koma)</span>
						<input
							class="app-input mt-1 w-full"
							value={String(config.voice?.personality ?? '')}
							oninput={(e) => setVoice('personality', e.currentTarget.value)}
						/>
					</label>
					{#each sliderKeys as k (k)}
						<div class="flex items-center gap-2">
							<span class="w-[90px] text-sm capitalize">{k}:</span>
							<input
								type="range"
								min="1"
								max="5"
								class="flex-1"
								value={Number(config.voice?.[k] ?? 3)}
								oninput={(e) => setVoice(k, Number(e.currentTarget.value))}
							/>
							<span class="text-sm">{Number(config.voice?.[k] ?? 3)}/5</span>
						</div>
					{/each}
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">CTA Style</span>
						<input
							class="app-input mt-1 w-full"
							value={String(config.voice?.cta_style ?? '')}
							oninput={(e) => setVoice('cta_style', e.currentTarget.value)}
						/>
					</label>
				</div>
			</div>

			<div class="app-card p-4">
				<h2 class="mb-3 text-base font-semibold">Aturan Editorial</h2>
				<div class="flex flex-col gap-2">
					<textarea
						class="app-input w-full"
						placeholder="Wajib — satu per baris"
						value={toArray(config.editorial_rules?.required_rules).join('\n')}
						oninput={(e) => setRules('required_rules', e.currentTarget.value.split('\n'))}
					></textarea>
					<textarea
						class="app-input w-full"
						placeholder="Dilarang — satu per baris"
						value={toArray(config.editorial_rules?.forbidden_rules).join('\n')}
						oninput={(e) => setRules('forbidden_rules', e.currentTarget.value.split('\n'))}
					></textarea>
					<textarea
						class="app-input w-full"
						placeholder="Disclaimer wajib"
						value={String(config.editorial_rules?.required_disclaimer ?? '')}
						oninput={(e) => setRules('required_disclaimer', e.currentTarget.value)}
					></textarea>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Citation policy</span>
						<input
							class="app-input mt-1 w-full"
							value={String(config.editorial_rules?.citation_policy ?? '')}
							oninput={(e) => setRules('citation_policy', e.currentTarget.value)}
						/>
					</label>
				</div>
			</div>

			<div class="app-card p-4">
				<h2 class="mb-3 text-base font-semibold">Profil Brand</h2>
				<div class="flex flex-col gap-2">
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Positioning</span>
						<input
							class="app-input mt-1 w-full"
							value={String(config.profile?.positioning ?? '')}
							oninput={(e) => setProfile('positioning', e.currentTarget.value)}
						/>
					</label>
					<textarea
						class="app-input w-full"
						placeholder="Value proposition"
						value={String(config.profile?.value_proposition ?? '')}
						oninput={(e) => setProfile('value_proposition', e.currentTarget.value)}
					></textarea>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Target audience</span>
						<input
							class="app-input mt-1 w-full"
							value={String(config.profile?.target_audience ?? '')}
							oninput={(e) => setProfile('target_audience', e.currentTarget.value)}
						/>
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Default CTA</span>
						<input
							class="app-input mt-1 w-full"
							value={String(config.profile?.default_cta ?? '')}
							oninput={(e) => setProfile('default_cta', e.currentTarget.value)}
						/>
					</label>
					<div class="flex items-center gap-2">
						<span class="w-[160px] text-sm">Jumlah kata (admin)</span>
						<input
							class="app-input flex-1"
							type="number"
							min="300"
							max="10000"
							step="50"
							value={String(config.profile?.default_target_words ?? 1000)}
							oninput={(e) => setProfile('default_target_words', Number(e.currentTarget.value) || 1000)}
						/>
						<span class="text-sm">300–10.000</span>
					</div>
					<p class="text-xs text-muted-foreground">
						Panjang artikel default per brand — klien tidak bisa ubah (diatur admin di sini).
					</p>
				</div>
			</div>

			<div>
				<button class="app-btn app-btn-primary" disabled={saving} onclick={save}>
					<Save size={16} />{saving ? 'Menyimpan…' : 'Simpan Pengaturan'}
				</button>
			</div>
		</div>
	{:else if brands.length === 0 && error.includes('Akses platform')}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses pengaturan konten platform.</p>
		</div>
	{/if}
</section>
