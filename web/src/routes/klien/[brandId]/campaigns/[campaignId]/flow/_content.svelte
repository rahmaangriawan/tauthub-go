<script lang="ts">
	// Tahap 1 — Content (parity blok `flowStatus === "content"` app lama).
	import { Diamond, Gauge, RefreshCw } from '@lucide/svelte';
	import type { FlowState } from './_flow-state.svelte';
	import TrailingDots from './_trailing-dots.svelte';
	import ContentEditor from './_content-editor.svelte';
	import AiAnalysis from './_ai-analysis.svelte';

	let { flow }: { flow: FlowState } = $props();

	const GEN_MODES = [
		{
			value: 'economy' as const,
			title: 'Mode Hemat',
			icon: Gauge,
			desc: 'Cepat dan hemat. Cocok untuk draf awal & artikel rutin.'
		},
		{
			value: 'premium' as const,
			title: 'Mode Premium',
			icon: Diamond,
			desc: 'Kualitas terbaik, penalaran mendalam. Untuk konten penting & analitis.'
		}
	];

	const GEN_STEPS = [
		{ label: 'Menganalisis brief', micro: 'Tauthub sedang membaca brief Anda dan menemukan informasi penting untuk artikel.' },
		{ label: 'Menyusun artikel', micro: 'Sedang mengubah bahan Anda menjadi naskah yang ringkas dan informatif.' },
		{ label: 'Memeriksa kelengkapan', micro: 'Memastikan informasi penting dari brief tidak terlewat.' },
		{ label: 'Menilai kualitas artikel', micro: 'Memeriksa struktur, relevansi, dan kualitas artikel.' },
		{ label: 'Menyiapkan hasil akhir', micro: 'Sedikit lagi. Kami sedang melakukan pemeriksaan terakhir sebelum hasil ditampilkan.' }
	];

	const genPct = $derived(Math.round(flow.genProgress));
	const genIdx = $derived(genPct <= 20 ? 0 : genPct <= 45 ? 1 : genPct <= 65 ? 2 : genPct <= 90 ? 3 : 4);
	const activeStep = $derived(GEN_STEPS[genIdx] ?? GEN_STEPS[GEN_STEPS.length - 1]);
	const hasEvaluation = $derived(
		!!flow.doc && typeof flow.doc.evaluation === 'object' && Object.keys(flow.doc.evaluation).length > 0
	);
</script>

<h2 class="card-title">Hasil Artikel</h2>

{#if !flow.doc && !flow.genBusy && !flow.intake?.job_id}
	<div style="margin-top:12px;margin-bottom:12px;">
		<div style="font-size:12px;font-weight:600;color:var(--muted-foreground);margin-bottom:8px;">
			Mode AI <span style="font-weight:400;">— pilih kualitas &amp; biaya</span>
		</div>
		<div class="mode-grid">
			{#each GEN_MODES as mode (mode.value)}
				{@const ModeIcon = mode.icon}
				<button
					type="button"
					class={flow.genMode === mode.value ? 'mode-card mode-card-active' : 'mode-card'}
					disabled={flow.genBusy}
					title={mode.desc}
					onclick={() => (flow.genMode = mode.value)}
				>
					<span style="font-weight:600;font-size:13px;display:inline-flex;align-items:center;gap:6px;">
						<ModeIcon size={16} /> {mode.title}
					</span>
					<span style="font-size:11px;color:var(--muted-foreground);line-height:1.35;">{mode.desc.slice(0, 90)}…</span>
				</button>
			{/each}
		</div>
	</div>
	<div class="actions" style="margin-top:0;">
		<button class="app-btn app-btn-primary" type="button" disabled={flow.genBusy} onclick={() => void flow.generateArticle()} title="Biaya 3 Kredit">
			{flow.genBusy ? 'Menyiapkan…' : 'Generate Article · 3 Kredit'}
		</button>
	</div>
{/if}

{#if flow.genBusy}
	<div class="gen-card">
		<div style="display:flex;justify-content:space-between;gap:12px;align-items:center;">
			<span style="display:flex;gap:10px;align-items:center;min-width:0;">
				<TrailingDots />
				<strong style="font-size:14px;font-weight:700;color:var(--foreground);">
					{activeStep.label === 'Menyiapkan hasil akhir' ? 'Tauthub sedang menyiapkan artikel Anda' : activeStep.label}
				</strong>
			</span>
			<span style="font-size:13px;font-weight:750;color:var(--accent-foreground);min-width:36px;text-align:right;flex:none;">{genPct}%</span>
		</div>
		<div class="usage-quota-bar" style="margin-top:10px;height:7px;">
			<div class="usage-quota-fill" style="width:{flow.genProgress}%;transition:width .9s ease;"></div>
		</div>
		<div class="gen-steps">
			{#each GEN_STEPS as s, i (s.label)}
				<div class={i < genIdx ? 'gen-step gen-step-done' : i === genIdx ? 'gen-step gen-step-active' : 'gen-step'}>
					<span style="width:18px;display:inline-flex;justify-content:center;flex:none;font-size:12px;">
						{#if i < genIdx}✓{:else if i === genIdx}<TrailingDots size={14} />{:else}○{/if}
					</span>
					{s.label}
				</div>
			{/each}
		</div>
		<p class="card-hint" style="margin:10px 0 0;font-size:12.5px;">{activeStep.micro}</p>
		<div style="margin-top:12px;display:flex;gap:8px;flex-wrap:wrap;">
			<button
				type="button"
				class="app-btn app-btn-soft"
				disabled={flow.busy}
				onclick={() => {
					flow.genBusy = false;
					flow.genProgress = 0;
					flow.genStep = '';
					void flow.gotoStage('intake');
				}}
			>
				← Kembali ke Draf (Content Intake)
			</button>
			<button type="button" class="app-btn app-btn-soft" onclick={() => void flow.loadFlow()}>Muat ulang hasil</button>
		</div>
	</div>
{/if}

{#if flow.doc}
	<div style="margin-top:16px;">
		{#if flow.doc.meta_title}<p class="card-hint"><strong>Meta title:</strong> {flow.doc.meta_title}</p>{/if}
		{#if flow.doc.meta_description}<p class="card-hint"><strong>Meta description:</strong> {flow.doc.meta_description}</p>{/if}
		{#if hasEvaluation}
			<div style="margin-top:12px;display:flex;gap:12px;flex-wrap:wrap;align-items:center;">
				<span style="font-size:13px;color:var(--muted-foreground);">✨ Dibuat dengan <strong>AI Tauthub</strong></span>
			</div>
		{/if}
		<div style="margin-top:14px;">
			{#key flow.editorReloadKey}
				<ContentEditor
					initialTitle={flow.doc.title}
					initialHtml={flow.doc.body_html}
					saving={flow.editorSaving}
					saveError={flow.editorError}
					onSave={(title, html, publish) => void flow.saveDocument(title, html, publish)}
				/>
			{/key}
		</div>
		<div style="margin-top:12px;display:flex;align-items:center;gap:8px;flex-wrap:wrap;">
			<button
				type="button"
				class="app-btn app-btn-soft"
				disabled={flow.regenBusy}
				onclick={() => void flow.openRegenModal()}
			>
				<RefreshCw size={15} />
				Buat Ulang
			</button>
			<span class="card-hint" style="margin:0;font-size:12px;">
				Buat ulang artikel dengan sudut pemberitaan berbeda · biaya <strong>5 TC</strong>
			</span>
		</div>
		{#if flow.campaign?.media_type}
			<AiAnalysis flowBase={flow.flowBase} />
		{/if}
	</div>
{/if}
