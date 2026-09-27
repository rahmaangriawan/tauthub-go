<script lang="ts">
	// Wizard alur kampanye klien — port `campaign-flow-module.tsx` (3.749 baris)
	// app Next.js lama. State + pemanggilan API ada di `_flow-state.svelte.ts`;
	// tiap tahap jadi komponen sendiri: _intake / _content / _media / _review /
	// _distributing, dengan modal bersama _regen / _angle-modal / _post-editor.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Check } from '@lucide/svelte';
	import { ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { STAGES, createFlowState } from './_flow-state.svelte';
	import Intake from './_intake.svelte';
	import Content from './_content.svelte';
	import Media from './_media.svelte';
	import Review from './_review.svelte';
	import Distributing from './_distributing.svelte';
	import Regen from './_regen.svelte';
	import './flow.css';

	const segment = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.campaignId as string);
	const flow = createFlowState(
		() => segment,
		() => campaignId
	);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return; // layout yang mengarahkan ke /login
			await flow.boot();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	// Polling distribusi hanya saat tahap 5.
	$effect(() => {
		if (flow.flowStatus === 'distributing') flow.startDistributionPolling();
		else flow.stopDistributionPolling();
		return () => flow.stopDistributionPolling();
	});

	function stageClickable(i: number) {
		return (
			i < flow.stageIndex &&
			!!flow.campaign &&
			flow.campaign.campaign_flow_status !== 'checkout' &&
			flow.campaign.campaign_flow_status !== 'distributing'
		);
	}
</script>

<main>
	<div class="flow-content">
		{#if flow.loading}
			<div class="skeleton-list">
				{#each [0, 1, 2] as i (i)}<div class="skeleton-row"></div>{/each}
			</div>
		{:else if !flow.flowData}
			<div class="flow-err">{flow.err || 'Flow tidak dapat dimuat.'}</div>
		{:else}
			<!-- Stepper -->
			<div class="flow-stepper">
				{#each STAGES as s, i (s.key)}
					{@const clickable = stageClickable(i)}
					{@const isDone = i < flow.stageIndex || (i === 4 && flow.isStage5Completed)}
					{@const isActive = i === flow.stageIndex && !flow.isStage5Completed}
					<div style="display:flex;align-items:center;gap:6px;flex:{i < STAGES.length - 1 ? 1 : 'none'};">
						<button
							type="button"
							class="stepper-item{isDone ? ' stepper-item-done' : ''}{isActive ? ' stepper-item-active' : ''}{clickable
								? ' stepper-item-clickable'
								: ''}"
							disabled={!clickable}
							title={clickable ? `Kembali ke tahap ${s.label}` : undefined}
							onclick={clickable ? () => void flow.gotoStage(s.key) : undefined}
						>
							<span class="stepper-number">{#if isDone}<Check size={11} />{:else}{i + 1}{/if}</span>
							<span>{s.label}</span>
						</button>
						{#if i < STAGES.length - 1}
							<div class="stepper-connector{isDone ? ' stepper-connector-done' : ''}"></div>
						{/if}
					</div>
				{/each}
			</div>

			{#if flow.err}<div class="flow-err">{flow.err}</div>{/if}

			{#if flow.campaign && flow.flowStatus !== 'final_review'}
				<div class="eyebrow">Kampanye: {flow.campaign.name}</div>
			{/if}

			{#if flow.flowStatus === 'intake'}
				<Intake {flow} />
			{:else if flow.flowStatus === 'content'}
				<Content {flow} />
			{:else if flow.flowStatus === 'media_selection'}
				<Media {flow} />
			{:else if flow.flowStatus === 'final_review'}
				<Review {flow} />
			{:else if flow.flowStatus === 'distributing'}
				<Distributing {flow} />
			{/if}
		{/if}
	</div>

	<Regen {flow} />
</main>
