<script lang="ts">
	// Step pill Taut Pintar — penanda posisi user di alur 4 agent.
	// Port agent-steps.tsx; versi portal ke utility bar tidak bisa dipakai di
	// sini (bar dimiliki +layout), jadi pill dirender inline di kepala halaman.
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Check } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { AGENT_STEPS, AGENT_STEP_ROUTES, MANUAL_PROFILER_REASON } from './_shared';

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

	const steps = $derived(extra === 'ringkasan' ? [...AGENT_STEPS, 'Ringkasan'] : [...AGENT_STEPS]);
	const cur = $derived(extra === 'ringkasan' ? steps.length : current);

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

	const clickable = (n: number, locked: boolean) => n <= cur && !locked;

	function select(n: number, locked: boolean) {
		if (!clickable(n, locked)) return;
		const slug = AGENT_STEP_ROUTES[n - 1];
		if (slug) void goto(`${base}/${slug}`);
	}
</script>

<ol class="taut-steps" aria-label="Tahapan Taut Pintar">
	{#each steps as label, i (label)}
		{@const n = i + 1}
		{@const locked = Boolean(lockedSteps?.includes(n))}
		{@const canClick = clickable(n, locked)}
		{@const cls = pillClass(n, locked, canClick)}
		{@const tip = locked ? (lockedReason ?? `${label} tidak dipakai pada kampanye ini.`) : canClick ? `Buka ${label} (langkah tersimpan)` : undefined}
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
						     bukan centang, supaya tidak terbaca "sudah dikerjakan". -->
						{#if n < cur && !locked}<Check size={13} strokeWidth={3} />{:else}{n}{/if}
					</span>
					<span class="taut-step-label">{label}</span>
				</span>
			</li>
		{/if}
	{/each}
</ol>

{#if lockedSteps?.includes(1) && extra !== 'ringkasan'}
	<p class="mt-2 text-[11px] text-muted-foreground">{MANUAL_PROFILER_REASON}</p>
{/if}

<style>
	.taut-steps {
		display: flex;
		align-items: center;
		justify-content: flex-end;
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
