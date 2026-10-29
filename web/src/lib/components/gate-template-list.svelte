<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Opsi ketiga gerbang Taut Brief: mulai dari template brief yang pernah
	// disimpan dari brand ini (atau brand workspace). Sengaja sekunder — daftar
	// ringkas di bawah dua kartu utama. Daftar kosong berarti tidak ada yang
	// ditampilkan sama sekali.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { ArrowRight, LoaderCircle } from '@lucide/svelte';
	import { apiRequest, readApiError } from '$lib/auth';

	type Template = {
		id: string;
		name: string;
		description?: string | null;
		source_brand_name?: string | null;
		created_at?: string | null;
	};

	let {
		brandId,
		brandUrl,
		disabled = false
	}: {
		brandId: string;
		brandUrl: string;
		disabled?: boolean;
	} = $props();

	let templates = $state<Template[]>([]);
	let applying = $state<string | null>(null);
	let error = $state('');

	onMount(() => {
		void load();
	});

	async function load() {
		try {
			const res = await apiRequest(`/v1/templates?brandId=${encodeURIComponent(brandId)}`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat template.')));
			const data = await res.json();
			templates = Array.isArray(data) ? data : [];
		} catch {
			// Opsi sekunder: kegagalan memuat cukup menyembunyikan daftar.
			templates = [];
		}
	}

	async function apply(template: Template) {
		if (applying || disabled) return;
		applying = template.id;
		error = '';
		try {
			const res = await apiRequest(`/v1/templates/${template.id}/apply`, {
				method: 'POST',
				body: JSON.stringify({ brandId, name: template.name })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memakai template.')));
			const created = (await res.json()) as { id?: string; short_code?: string | null };
			const key = created.short_code || created.id;
			if (!key) throw new Error(tr('Template diterapkan, tetapi brief baru tidak dapat dibuka.'));
			await goto(`/klien/${brandUrl}/profiler/${key}?gate=instan`, { replaceState: true });
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal memakai template.');
			applying = null;
		}
	}
</script>

{#if templates.length > 0}
	<div class="tpl-section">
		<p class="tpl-label">{tr('Atau mulai dari template')}</p>
		<div class="tpl-list">
			{#each templates as tpl (tpl.id)}
				<button
					class="tpl-item"
					type="button"
					disabled={disabled || Boolean(applying)}
					onclick={() => void apply(tpl)}
				>
					<span class="tpl-main">
						<span class="tpl-name">{tpl.name}</span>
						{#if tpl.description}<span class="tpl-desc">{tpl.description}</span>{/if}
						{#if tpl.source_brand_name}<span class="tpl-source">{tr('Dari')} {tpl.source_brand_name}</span>{/if}
					</span>
					{#if applying === tpl.id}
						<LoaderCircle size={14} class="animate-spin" style="flex-shrink: 0; color: var(--primary, #2b67ec);" />
					{:else}
						<ArrowRight size={14} style="flex-shrink: 0; color: var(--primary, #2b67ec);" />
					{/if}
				</button>
			{/each}
		</div>
		{#if error}<p class="tpl-error">{error}</p>{/if}
	</div>
{/if}

<style>
	.tpl-section {
		margin-top: 16px;
		padding-top: 14px;
		border-top: 1px solid var(--border, #e2e8f0);
	}

	.tpl-label {
		margin: 0 0 8px;
		font-size: 11.5px;
		font-weight: 600;
		letter-spacing: 0.02em;
		color: var(--muted-foreground, #667085);
	}

	.tpl-list {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
		gap: 8px;
	}

	.tpl-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding: 10px 12px;
		text-align: left;
		border: 1px solid var(--border, #e2e8f0);
		border-radius: 10px;
		background: var(--card, #fff);
		cursor: pointer;
		transition: border-color 0.12s ease, background-color 0.12s ease;
	}

	.tpl-item:hover:not(:disabled) {
		border-color: color-mix(in srgb, var(--primary, #2b67ec) 40%, var(--border, #e2e8f0));
		background: color-mix(in srgb, var(--primary, #2b67ec) 4%, var(--card, #fff));
	}

	.tpl-item:disabled {
		opacity: 0.6;
		cursor: progress;
	}

	.tpl-main {
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
	}

	.tpl-name {
		font-size: 13px;
		font-weight: 600;
		color: var(--foreground, #111827);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.tpl-desc {
		font-size: 11.5px;
		line-height: 1.45;
		color: var(--muted-foreground, #667085);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.tpl-source {
		font-size: 10.5px;
		color: var(--muted-foreground, #667085);
	}

	.tpl-error {
		margin: 8px 0 0;
		font-size: 12px;
		color: var(--destructive, #dc2626);
	}
</style>
