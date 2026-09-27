<script lang="ts">
	// Modal "Buat Angle Berbeda" (parity blok angleModal app lama).
	import {
		BookOpen,
		ChartBar,
		Check,
		CircleCheck,
		Compass,
		ListChecks,
		Newspaper,
		Quote,
		TrendingUp,
		Trophy,
		UserRound,
		Users,
		X
	} from '@lucide/svelte';
	import type { FlowState } from './_flow-state.svelte';

	let { flow }: { flow: FlowState } = $props();

	const ICONS: Record<string, typeof Compass> = {
		benefit: UserRound,
		news_value: Newspaper,
		data: ChartBar,
		expert: Quote,
		story: BookOpen,
		community: Users,
		how_to: ListChecks,
		trend: TrendingUp,
		solution: CircleCheck,
		milestone: Trophy
	};
	const angleIcon = (id: string) => ICONS[id] ?? Compass;

	const radius = 30;
	const circumference = 2 * Math.PI * radius;
	const dashOffset = $derived(circumference - (Math.min(100, Math.max(0, flow.angleProgress)) / 100) * circumference);
	const currentAngleItem = $derived(flow.mediaItems.find((m) => m.id === flow.angleModal));
	const statusText = $derived(
		flow.angleProgress < 25
			? 'Menganalisis sudut pemberitaan dan target media…'
			: flow.angleProgress < 60
				? `Menyusun kerangka narasi dengan sudut ${flow.submittingAngleLabel || 'baru'}…`
				: flow.angleProgress < 90
					? 'Menulis naskah varian artikel secara menyeluruh…'
					: flow.angleProgress < 100
						? 'Menyelesaikan dan menyimpan dokumen varian…'
						: 'Menyimpan ke alur kampanye…'
	);
</script>

{#if flow.angleModal}
	<div class="overlay" role="presentation">
		<dialog
			open
			class="angle-modal-content"
			aria-label="Buat Angle Berbeda"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => { if (e.key === 'Escape' && !flow.submittingAngleId) flow.angleModal = null; }}
		>
			<div class="modal-head">
				<h3 style="margin:0;font-size:14.5px;font-weight:700;">Buat Angle Berbeda</h3>
				<button
					class="modal-close"
					type="button"
					aria-label="Tutup"
					disabled={!!flow.submittingAngleId}
					onclick={() => (flow.angleModal = null)}
				>
					<X size={15} />
				</button>
			</div>
			<div class="angle-modal-body">
				{#if flow.submittingAngleId}
					<div class="angle-progress-wrapper">
						<div class="angle-circular-progress">
							<svg class="angle-progress-svg" viewBox="0 0 76 76">
								<circle cx="38" cy="38" r={radius} class="angle-progress-track" stroke-width="5" fill="transparent" />
								<circle
									cx="38"
									cy="38"
									r={radius}
									class="angle-progress-indicator"
									stroke-width="5"
									fill="transparent"
									stroke-dasharray={circumference}
									stroke-dashoffset={dashOffset}
									stroke-linecap="round"
								/>
							</svg>
							<span class="angle-progress-percent">{Math.round(flow.angleProgress)}%</span>
						</div>
						<div class="angle-progress-info">
							<h4 class="angle-progress-title">
								{flow.angleProgress >= 100
									? 'Artikel Berhasil Dibuat!'
									: `Membuat Artikel "${flow.submittingAngleLabel || 'Angle Pilihan'}"`}
							</h4>
							<p class="angle-progress-status">{statusText}</p>
						</div>
					</div>
				{:else}
					<p class="angle-modal-description">Pilih sudut pemberitaan yang berbeda untuk artikel di media ini</p>
					{#if flow.angleOptions.length > 0}
						<div class="angle-options-grid">
							{#each flow.angleOptions as opt (opt.id)}
								{@const Ico = angleIcon(opt.id)}
								<button
									type="button"
									class={currentAngleItem?.angle_label === opt.label ? 'angle-option-card angle-option-card-active' : 'angle-option-card'}
									disabled={!!flow.paraphraseBusy}
									onclick={() => flow.angleModal && void flow.submitAngle(flow.angleModal, opt.id)}
								>
									<div class="angle-option-left">
										<div class="angle-option-icon-box"><Ico size={17} /></div>
										<span class="angle-option-label">{opt.label}</span>
									</div>
									<div class="angle-option-radio">{#if currentAngleItem?.angle_label === opt.label}<Check size={11} />{/if}</div>
								</button>
							{/each}
						</div>
					{:else}
						<p class="angle-modal-description" style="color:var(--muted-foreground);">Memuat opsi angle…</p>
					{/if}
					<div class="angle-modal-footer">
						<button class="angle-modal-cancel-btn" type="button" onclick={() => (flow.angleModal = null)}>Batal</button>
					</div>
				{/if}
			</div>
		</dialog>
	</div>
{/if}