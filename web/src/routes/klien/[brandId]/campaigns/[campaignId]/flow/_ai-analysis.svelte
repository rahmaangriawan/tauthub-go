<script lang="ts">
	// Port ai-analysis-section.tsx — analisis & optimasi konten setelah artikel ada.
	import { onMount } from 'svelte';
	import { Check, ChevronDown, Sparkle, X } from '@lucide/svelte';
	import { apiRequest } from '$lib/auth';
	import { readError } from './_flow-state.svelte';

	let { flowBase }: { flowBase: string } = $props();

	type AnalysisData = {
		editorial_score: number | null;
		diagnosis: string | null;
		strengths: string[];
		violations: string[];
		content_type_detected: string | null;
		editorial_fit_score: number | null;
		readability: number | null;
		clarity: number | null;
		search_intent: string | null;
		structure_score: number | null;
		potential_value: string | null;
		headline_suggestions: string[];
		revised_draft: string | null;
		standard_source: string | null;
		standard_version: string | null;
		fallback_type: string | null;
	};

	let analysis = $state<AnalysisData | null>(null);
	let busy = $state(false);
	let error = $state('');
	let expanded = $state(true);
	let draftExpanded = $state(false);
	let selectedHeadline = $state<string | null>(null);

	async function load() {
		try {
			const res = await apiRequest(`${flowBase}/flow/analyze`, { method: 'POST', body: JSON.stringify({}) });
			if (!res.ok) {
				error = await readError(res, 'Analisis gagal.');
				return;
			}
			const data = (await res.json()) as { analysis: AnalysisData };
			analysis = data.analysis;
			if (data.analysis?.headline_suggestions?.length) selectedHeadline = data.analysis.headline_suggestions[0] ?? null;
		} catch {
			error = 'Analisis gagal.';
		}
	}

	onMount(() => {
		void load();
	});

	function scoreColor(value: number | null): string {
		if (value == null) return 'var(--muted-foreground)';
		if (value >= 80) return 'var(--success)';
		if (value >= 60) return 'var(--warning)';
		return 'var(--destructive)';
	}

	function standardLabel(source: string | null): string {
		if (source === 'media_specific') return 'Standard khusus media';
		if (source === 'mainstream_fallback') return 'Standard Jurnalistik Mainstream (fallback)';
		if (source === 'blogger_fallback') return 'Standard Blog / Evergreen (fallback)';
		return 'Standard editorial global';
	}

	const scoreBadges = $derived(
		analysis
			? [
					{ label: 'Skor Kelayakan Editorial', value: analysis.editorial_score },
					{ label: 'Editorial Fit', value: analysis.editorial_fit_score },
					{ label: 'Readability', value: analysis.readability },
					{ label: 'Clarity', value: analysis.clarity },
					{ label: 'Structure', value: analysis.structure_score }
				].filter((b) => b.value != null)
			: []
	);
</script>

<div class="ai-analysis">
	<button type="button" class="ai-analysis-head" onclick={() => (expanded = !expanded)}>
		<span class="ai-analysis-title"><Sparkle size={15} /> Analisis &amp; Optimasi Konten</span>
		<span style="display:flex;gap:10px;align-items:center;">
			{#if analysis?.editorial_score != null}
				<span style="font-size:12px;font-weight:700;color:{scoreColor(analysis.editorial_score)};">
					Skor {analysis.editorial_score}/100
				</span>
			{/if}
			<span style="display:inline-flex;transition:transform 150ms;transform:{expanded ? 'rotate(180deg)' : 'none'};">
				<ChevronDown size={14} />
			</span>
		</span>
	</button>

	{#if expanded}
		<div class="ai-analysis-body">
			{#if error}<div style="font-size:12px;color:var(--destructive);">{error}</div>{/if}

			{#if !analysis && !busy && !error}
				<button
					type="button"
					disabled={busy}
					class="app-btn app-btn-soft"
					style="width:fit-content;"
					onclick={async () => {
						busy = true;
						error = '';
						try {
							await load();
						} finally {
							busy = false;
						}
					}}
				>
					{busy ? 'Menganalisis…' : 'Jalankan Analisis AI'}
				</button>
			{/if}

			{#if busy}<div style="font-size:13px;color:var(--muted-foreground);">AI sedang menganalisis naskah berdasarkan standard media…</div>{/if}

			{#if analysis}
				{#if analysis.standard_source}
					<div style="font-size:11px;color:var(--muted-foreground);">
						Dioptimasi berdasarkan <strong>{standardLabel(analysis.standard_source)}</strong>{analysis.standard_version
							? ` (${analysis.standard_version})`
							: ''}
					</div>
				{/if}

				<div class="ai-score-grid">
					{#each scoreBadges as badge (badge.label)}
						<div class="ai-score-badge">
							<div class="ai-score-label">{badge.label}</div>
							<div class="ai-score-value" style="color:{scoreColor(badge.value)};">{badge.value ?? '—'}</div>
						</div>
					{/each}
				</div>

				{#if analysis.content_type_detected}
					<div>
						<span style="font-size:12px;font-weight:600;color:var(--muted-foreground);">Content Type: </span>
						<span style="font-size:13px;font-weight:700;color:var(--primary);">{analysis.content_type_detected}</span>
					</div>
				{/if}

				{#if analysis.search_intent}
					<div style="font-size:12px;color:var(--muted-foreground);">Search Intent: {analysis.search_intent}</div>
				{/if}

				{#if analysis.potential_value}
					<div style="font-size:12px;color:var(--muted-foreground);">Nilai Potensial: {analysis.potential_value}</div>
				{/if}

				{#if analysis.diagnosis}
					<div style="font-size:13px;color:var(--foreground);line-height:1.5;">
						<strong style="display:block;font-size:12px;color:var(--muted-foreground);margin-bottom:2px;">Diagnosis / AI Assessment</strong>
						{analysis.diagnosis}
					</div>
				{/if}

				{#if analysis.strengths.length > 0}
					<div>
						<strong style="font-size:12px;color:var(--success);display:block;margin-bottom:4px;">DNA Sesuai</strong>
						<div style="display:flex;flex-direction:column;gap:3px;">
							{#each analysis.strengths as s, i (i)}
								<div class="ai-list-item">
									<span style="color:var(--success);margin-top:1px;flex:none;"><Check size={13} /></span>
									<span>{s}</span>
								</div>
							{/each}
						</div>
					</div>
				{/if}

				{#if analysis.violations.length > 0}
					<div>
						<strong style="font-size:12px;color:var(--destructive);display:block;margin-bottom:4px;">Pelanggaran / Bias</strong>
						<div style="display:flex;flex-direction:column;gap:3px;">
							{#each analysis.violations as v, i (i)}
								<div class="ai-list-item">
									<span style="color:var(--destructive);margin-top:1px;flex:none;"><X size={13} /></span>
									<span>{v}</span>
								</div>
							{/each}
						</div>
					</div>
				{/if}

				{#if analysis.headline_suggestions.length > 0}
					<div>
						<strong style="font-size:12px;color:var(--muted-foreground);display:block;margin-bottom:4px;">Rekomendasi Judul</strong>
						<div style="display:flex;flex-direction:column;gap:4px;">
							{#each analysis.headline_suggestions as h, i (i)}
								<button
									type="button"
									class={selectedHeadline === h ? 'ai-headline-btn ai-headline-btn-active' : 'ai-headline-btn'}
									onclick={() => (selectedHeadline = h)}
								>
									{h}
								</button>
							{/each}
						</div>
					</div>
				{/if}

				{#if analysis.revised_draft}
					<div style="border:1px solid var(--border);border-radius:var(--radius-control);">
						<button
							type="button"
							style="width:100%;display:flex;align-items:center;justify-content:space-between;padding:9px 12px;border:0;background:transparent;cursor:pointer;font-weight:600;font-size:13px;color:var(--foreground);"
							onclick={() => (draftExpanded = !draftExpanded)}
						>
							<span>Draf Hasil Revisi</span>
							<span style="display:inline-flex;transition:transform 150ms;transform:{draftExpanded ? 'rotate(180deg)' : 'none'};">
								<ChevronDown size={13} />
							</span>
						</button>
						{#if draftExpanded}
							<div style="padding:0 12px 12px;font-size:13px;color:var(--muted-foreground);line-height:1.55;white-space:pre-wrap;max-height:320px;overflow-y:auto;">
								{analysis.revised_draft}
							</div>
						{/if}
					</div>
				{/if}
			{/if}
		</div>
	{/if}
</div>
