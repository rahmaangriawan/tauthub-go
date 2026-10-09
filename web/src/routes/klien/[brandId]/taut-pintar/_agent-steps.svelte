<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Step pill Taut Pintar — penanda posisi user di alur 4 agent.
	// Port agent-steps.tsx: sama seperti app lama, pil DIPINDAHKAN ke sisi kiri
	// utility bar (app lama memakai createPortal ke `utilityBar > div:first-child`;
	// Svelte tidak punya portal, jadi node-nya dipindahkan ke elemen anchor yang
	// dibagikan layout lewat getToolbarSlot). Di luar layout klien — atau sebelum
	// bar ter-mount — pil tetap tampil inline sebagai fallback.
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Check } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { getToolbarSlot } from '$lib/components/toolbar-slot.svelte';
	import { ensureCampaignStep, furthestStep, noteReachedStep, statusStep } from '$lib/agent-steps.svelte';
	import { AGENT_STEPS, AGENT_STEP_ROUTES } from './_shared';

	let {
		current,
		tone,
		extra,
		lockedSteps,
		lockedReason
	}: {
		current: 1 | 2 | 3 | 4;
		tone?: 'waiting' | 'published';
		extra?: 'ringkasan';
		lockedSteps?: number[];
		lockedReason?: string;
	} = $props();

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const base = $derived(`/klien/${getBrandUrlId() || brandId}/taut-pintar/${campaignId}`);

	// Langkah saat ini dihitung TANPA membaca panjang daftar pil, supaya tidak
	// terjadi lingkaran dependensi: halaman Ringkasan selalu langkah kelima.
	const cur = $derived(extra === 'ringkasan' ? AGENT_STEPS.length + 1 : current);

	// Catat langkah terjauh yang sudah dicapai, dan kenali tahap kampanye dari
	// statusnya. Status dipakai supaya pil Ringkasan tetap tersedia dari halaman
	// lain setelah kampanye selesai — termasuk setelah halaman dimuat ulang.
	$effect(() => {
		noteReachedStep(campaignId, cur);
		void ensureCampaignStep(brandId, campaignId);
	});
	const maxReached = $derived(Math.max(cur, furthestStep(campaignId), statusStep(campaignId)));
	const steps = $derived(maxReached > AGENT_STEPS.length ? [...AGENT_STEPS, 'Ringkasan'] : [...AGENT_STEPS]);

	function pillClass(n: number, locked: boolean, clickable: boolean): string {
		const active = n === cur;
		const done = n < cur;
		const parts = ['taut-step'];
		if (active) parts.push('taut-step-active');
		if (done) parts.push('taut-step-done');
		if (active && tone === 'waiting') parts.push('taut-step-waiting');
		if (active && tone === 'published') parts.push('taut-step-published');
		if (clickable) parts.push('taut-step-clickable');
		if (locked) parts.push('taut-step-locked');
		return parts.join(' ');
	}

	const clickable = (n: number, locked: boolean) => n <= maxReached && !locked;

	function select(n: number, locked: boolean) {
		if (!clickable(n, locked)) return;
		const slug = AGENT_STEP_ROUTES[n - 1];
		if (!slug) return;
		// The Matchmaker kini mengarah ke Katalog Media: klien memilih media di
		// sana (katalog), bukan di halaman matchmaker.
		if (slug === 'matchmaker') {
			void goto(`/klien/${getBrandUrlId() || brandId}/media`);
			return;
		}
		void goto(`${base}/${slug}`);
	}

	const toolbarSlot = getToolbarSlot();

	/** Action: pindahkan node ke slot utility bar selama slot-nya tersedia. */
	function portalToToolbar(node: HTMLElement) {
		const stop = $effect.root(() => {
			$effect(() => {
				const target = toolbarSlot?.el ?? null;
				if (!target) return;
				if (node.parentElement !== target) target.appendChild(node);
			});
		});
		return {
			destroy() {
				stop();
				node.remove();
			}
		};
	}
</script>

<ol class="taut-steps" aria-label="Tahapan Taut Pintar" use:portalToToolbar>
	{#each steps as label, i (label)}
		{@const n = i + 1}
		{@const locked = Boolean(lockedSteps?.includes(n))}
		{@const canClick = clickable(n, locked)}
		{@const cls = pillClass(n, locked, canClick)}
		{@const tip = n === cur
			? tr('Anda berada di sini')
			: locked
				? (lockedReason ?? `${label} tidak dipakai pada kampanye ini.`)
				: canClick
					? `Buka ${label} (langkah tersimpan)`
					: undefined}
		{#if canClick}
			<li>
				<button
					type="button"
					class={cls}
					aria-current={n === cur ? 'step' : undefined}
					title={tip}
					onclick={() => select(n, locked)}
				>
					<span class="taut-step-num" aria-hidden="true">
						{#if n < cur && !locked}<Check size={13} strokeWidth={3} />{:else}{n}{/if}
					</span>
					<span class="taut-step-label">{label}</span>
				</button>
			</li>
		{:else}
			<li>
				<span class={cls} aria-current={n === cur ? 'step' : undefined} aria-disabled={locked || undefined} title={tip}>
					<span class="taut-step-num" aria-hidden="true">
						<!-- Langkah yang di-bypass tidak pernah selesai — tampilkan nomornya,
						     bukan centang, supaya tidak terbaca tr("sudah dikerjakan"). -->
						{#if n < cur && !locked}<Check size={13} strokeWidth={3} />{:else}{n}{/if}
					</span>
					<span class="taut-step-label">{label}</span>
				</span>
			</li>
		{/if}
	{/each}
</ol>

<style>
	.taut-steps {
		display: flex;
		align-items: center;
		justify-content: flex-start;
		width: fit-content;
		gap: 8px;
		flex-wrap: wrap;
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.taut-steps > li {
		display: inline-flex;
		list-style: none;
	}
	.taut-step {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		padding: 5px 14px 5px 5px;
		border: 1px solid var(--border);
		border-radius: 999px;
		background: var(--card);
		font-size: 12.5px;
		color: var(--muted-foreground);
		white-space: nowrap;
		font-family: inherit;
		text-align: left;
	}
	.taut-step-num {
		width: 22px;
		height: 22px;
		border-radius: 50%;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		background: var(--muted);
		color: var(--muted-foreground);
		font-size: 12px;
		font-weight: 700;
		flex-shrink: 0;
	}
	.taut-step-active {
		border-color: var(--primary);
		color: var(--foreground);
		font-weight: 700;
	}
	.taut-step-active .taut-step-num {
		background: var(--primary);
		color: var(--primary-foreground);
	}
	.taut-step-done .taut-step-num {
		background: var(--success);
		color: #ffffff;
	}
	.taut-step-waiting {
		border-color: var(--warning);
	}
	.taut-step-waiting .taut-step-num {
		background: var(--warning);
		color: #ffffff;
	}
	.taut-step-published {
		border-color: var(--success);
	}
	.taut-step-published .taut-step-num {
		background: var(--success);
		color: #ffffff;
	}
	.taut-step-clickable {
		cursor: pointer;
		transition: border-color 0.15s ease, box-shadow 0.15s ease;
	}
	.taut-step-clickable:hover {
		border-color: var(--primary);
		box-shadow: 0 2px 6px rgba(43, 103, 236, 0.18);
	}
	.taut-step-clickable:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
	}
	.taut-step-locked {
		opacity: 0.5;
		cursor: default;
		background: var(--muted);
		border-color: var(--border);
		color: var(--muted-foreground);
	}
	.taut-step-locked .taut-step-num {
		background: var(--muted);
		color: var(--muted-foreground);
	}
	@media (max-width: 640px) {
		.taut-step-label {
			display: none;
		}
		.taut-step {
			padding: 5px;
		}
	}
</style>
