<script lang="ts">
	// GA4 Configs — port app/admin/ga4-configs/page.tsx.
	// Integrasi Google Analytics 4 per brand: Measurement ID + API Secret (secret
	// tidak pernah dikembalikan), consent mode, event mapping JSON.
	import { onMount } from 'svelte';
	import { ChartLine, TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type Config = {
		id: string;
		brand_id: string;
		brand_name: string | null;
		measurement_id: string;
		has_api_secret: boolean;
		enabled: boolean;
		consent_mode: string;
		event_mapping: unknown;
		updated_at: string;
	};
	type Brand = { id: string; name: string; slug: string };

	const PAGE_SIZE = 20;

	let loading = $state(true);
	let error = $state('');
	let rows = $state<Config[]>([]);
	let total = $state(0);
	let brands = $state<Brand[]>([]);
	let page = $state(1);
	let createOpen = $state(false);
	let busy = $state(false);
	let form = $state({
		brand_id: '',
		measurement_id: '',
		api_secret: '',
		consent_mode: 'required',
		enabled: 'false',
		event_mapping: ''
	});

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load(page);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load(targetPage: number) {
		loading = true;
		error = '';
		try {
			const [cfgRes, brandRes] = await Promise.all([
				apiRequest(`/v1/platform/ga4-configs?limit=${PAGE_SIZE}&offset=${(targetPage - 1) * PAGE_SIZE}`),
				apiRequest('/v1/platform/operations/brands')
			]);
			if (!cfgRes.ok) {
				const msg = await readApiError(cfgRes, 'Permintaan platform gagal.');
				error = msg === 'Resource not found' ? 'Akses platform administrator tidak tersedia.' : msg;
				return;
			}
			const cfgData = await cfgRes.json();
			rows = cfgData.ga4_configs ?? [];
			total = cfgData.total ?? 0;
			if (brandRes.ok) {
				brands = (await brandRes.json()) ?? [];
				form = { ...form, brand_id: form.brand_id || brands[0]?.id || '' };
			}
			page = targetPage;
		} catch {
			error = 'Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.';
		} finally {
			loading = false;
		}
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			let eventMapping: unknown = null;
			if (form.event_mapping.trim()) {
				try {
					eventMapping = JSON.parse(form.event_mapping);
				} catch {
					error = 'event_mapping bukan JSON yang valid.';
					return;
				}
			}
			const res = await apiRequest('/v1/platform/ga4-configs', {
				method: 'POST',
				body: JSON.stringify({
					brand_id: form.brand_id,
					measurement_id: form.measurement_id.trim(),
					api_secret: form.api_secret.trim() || null,
					consent_mode: form.consent_mode,
					enabled: form.enabled === 'true',
					event_mapping: eventMapping
				})
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			createOpen = false;
			await load(page);
		} finally {
			busy = false;
		}
	}

	async function updateStatus(config: Config, enabled: boolean) {
		error = '';
		const res = await apiRequest(`/v1/platform/ga4-configs/${config.id}`, {
			method: 'PUT',
			body: JSON.stringify({ enabled })
		});
		if (!res.ok) {
			error = await readApiError(res, 'Permintaan platform gagal.');
			return;
		}
		await load(page);
	}

	async function remove(config: Config) {
		if (!confirm('Hapus konfigurasi GA4 ini?')) return;
		error = '';
		const res = await apiRequest(`/v1/platform/ga4-configs/${config.id}`, { method: 'DELETE' });
		if (!res.ok) {
			error = await readApiError(res, 'Permintaan platform gagal.');
			return;
		}
		await load(page);
	}
</script>

<section class="mx-auto max-w-7xl">
	<PageHeader
		title="GA4 Configs"
		description="Integrasi Google Analytics 4 per brand — Measurement ID + API Secret (Phase 8). Secret tidak pernah ditampilkan."
	>
		<Badge tone="accent" label="{total} config" />
		<button class="app-btn app-btn-primary" type="button" onclick={() => (createOpen = true)}><ChartLine size={15} /> Buat config</button>
	</PageHeader>

	<Feedback {loading} {error} loadingText="Memuat konfigurasi GA4…" />

	{#if rows.length === 0 && error.includes('Akses platform')}
		<div class="app-card mt-4 px-4 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke konfigurasi GA4.</p>
		</div>
	{:else}
		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 1000px;">
				<thead>
					<tr class="text-xs text-muted-foreground">
						<th class="px-4 py-2.5 text-left font-medium">Brand</th>
						<th class="px-4 py-2.5 text-left font-medium">Measurement ID</th>
						<th class="px-4 py-2.5 text-left font-medium">API Secret</th>
						<th class="px-4 py-2.5 text-left font-medium">Consent</th>
						<th class="px-4 py-2.5 text-left font-medium">Status</th>
						<th class="px-4 py-2.5 text-right font-medium"></th>
					</tr>
				</thead>
				<tbody>
					{#each rows as row (row.id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<div class="flex items-center gap-2">
									<ChartLine size={15} />
									<div>
										<p class="font-medium">{row.brand_name ?? row.brand_id.slice(0, 8)}</p>
										<p class="text-xs text-muted-foreground">{row.id.slice(0, 8)}</p>
									</div>
								</div>
							</td>
							<td class="px-4 py-2.5"><code class="text-xs">{row.measurement_id}</code></td>
							<td class="px-4 py-2.5">{row.has_api_secret ? '✓ tersimpan' : '—'}</td>
							<td class="px-4 py-2.5"><Badge tone="neutral" label={row.consent_mode} /></td>
							<td class="px-4 py-2.5"><Badge tone={row.enabled ? 'success' : 'neutral'} label={row.enabled ? 'enabled' : 'disabled'} /></td>
							<td class="px-4 py-2.5">
								<div class="flex justify-end gap-2">
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void updateStatus(row, !row.enabled)}>
										{row.enabled ? 'Nonaktifkan' : 'Aktifkan'}
									</button>
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void remove(row)}>Hapus</button>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && rows.length === 0}
				<p class="px-4 py-12 text-center text-sm text-muted-foreground">Belum ada konfigurasi GA4.</p>
			{/if}
		</div>
		<Pagination total={total} limit={PAGE_SIZE} offset={(page - 1) * PAGE_SIZE} onChange={(next) => void load(next / PAGE_SIZE + 1)} />
	{/if}
</section>

{#if createOpen}
	<Modal title="Buat konfigurasi GA4" label="Buat konfigurasi GA4" maxWidth="lg" onClose={() => (createOpen = false)}>
		<form id="ga4-create-form" class="space-y-3" onsubmit={save}>
			<label class="block">
				<span class="text-sm font-medium">Brand</span>
				<Select
					ariaLabel="Brand"
					minWidth={240}
					placeholder="Pilih brand"
					value={form.brand_id}
					onchange={(v) => (form.brand_id = v)}
					options={brands.map((b) => ({ value: b.id, label: b.name }))}
				/>
			</label>
			<label class="block">
				<span class="text-sm font-medium">Measurement ID</span>
				<input class="app-input mt-1 w-full" required placeholder="G-XXXXXXX" bind:value={form.measurement_id} />
			</label>
			<label class="block">
				<span class="text-sm font-medium">API Secret (server-side, tidak pernah dikembalikan)</span>
				<input class="app-input mt-1 w-full" type="password" placeholder="Dari GA4 → Data Streams → MP API secrets" bind:value={form.api_secret} />
			</label>
			<div class="grid gap-3 sm:grid-cols-2">
				<label class="block">
					<span class="text-sm font-medium">Consent mode</span>
					<Select
						ariaLabel="Consent mode"
						minWidth={200}
						value={form.consent_mode}
						onchange={(v) => (form.consent_mode = v)}
						options={[
							{ value: 'required', label: 'required' },
							{ value: 'none', label: 'none' }
						]}
					/>
				</label>
				<label class="block">
					<span class="text-sm font-medium">Status</span>
					<Select
						ariaLabel="Status"
						minWidth={200}
						value={form.enabled}
						onchange={(v) => (form.enabled = v)}
						options={[
							{ value: 'true', label: 'enabled' },
							{ value: 'false', label: 'disabled' }
						]}
					/>
				</label>
			</div>
			<label class="block">
				<span class="text-sm font-medium">Event mapping (JSON, opsional)</span>
				<span class="block text-xs text-muted-foreground">Contoh: {'{"article_view": "article_view"}'}</span>
				<textarea class="app-input mt-1 w-full font-mono text-xs" rows="3" bind:value={form.event_mapping}></textarea>
			</label>
			{#if error}
				<div class="flex items-center gap-2 text-sm" style="color: var(--destructive);">
					<TriangleAlert size={15} /> {error}
				</div>
			{/if}
		</form>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => (createOpen = false)}>Batal</button>
			<button class="app-btn app-btn-primary" type="submit" form="ga4-create-form" disabled={busy}>{busy ? 'Menyimpan…' : 'Buat config'}</button>
		{/snippet}
	</Modal>
{/if}
