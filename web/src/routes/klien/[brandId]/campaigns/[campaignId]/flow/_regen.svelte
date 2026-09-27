<script lang="ts">
	// Modal "Buat Ulang Artikel" + toast sukses (parity blok regenOpen/regenToast
	// app lama; tinggi & animasi disederhanakan memakai CSS modal bersama).
	import { CircleCheck, X } from '@lucide/svelte';
	import type { FlowState } from './_flow-state.svelte';

	let { flow }: { flow: FlowState } = $props();

	const radius = 40;
	const circumference = 2 * Math.PI * radius;
	const dashOffset = $derived(circumference * (1 - flow.regenProgress / 100));
	const selectedLabel = $derived(flow.regenAngleOptions.find((o) => o.id === flow.regenAngleId)?.label ?? '');
</script>

{#if flow.regenToast}
	{@const toast = flow.regenToast}
	<div class="flow-toast" role="status">
		<span class="flow-toast-icon"><CircleCheck size={17} /></span>
		<span class="flow-toast-text">
			Naskah <strong>{toast.title.slice(0, 60)}</strong> berhasil dibuat ulang, silakan cek.
		</span>
		<button type="button" class="flow-toast-cta" onclick={() => void flow.checkRegenToast()}>Cek</button>
		<button type="button" class="flow-toast-close" aria-label="Tutup" onclick={() => (flow.regenToast = null)}>
			<X size={14} />
		</button>
	</div>
{/if}

{#if flow.regenOpen}
	<div class="overlay" role="presentation">
		<dialog
			open
			class="modal-card"
			style="width:min(460px,100%);"
			aria-label="Buat Ulang Artikel"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => { if (e.key === 'Escape' && !flow.regenBusy) flow.regenOpen = false; }}
		>
			<div class="modal-head">
				<span>{flow.regenBusy ? 'Membuat ulang artikel…' : 'Buat Ulang Artikel'}</span>
				<button type="button" class="modal-close" onclick={() => (flow.regenOpen = false)} disabled={flow.regenBusy} aria-label="Tutup">
					<X size={16} />
				</button>
			</div>

			{#if flow.regenBusy}
				<div class="circle-progress">
					<svg width="92" height="92" viewBox="0 0 92 92">
						<circle cx="46" cy="46" r={radius} fill="none" stroke="var(--border)" stroke-width="7" />
						<circle
							cx="46"
							cy="46"
							r={radius}
							fill="none"
							stroke="var(--primary)"
							stroke-width="7"
							stroke-linecap="round"
							stroke-dasharray={`${circumference}`}
							stroke-dashoffset={`${dashOffset}`}
							transform="rotate(-90 46 46)"
							style="transition: stroke-dashoffset .25s linear;"
						/>
						<text x="46" y="52" text-anchor="middle" font-size="19" font-weight="700" fill="var(--foreground)">
							{Math.round(flow.regenProgress)}%
						</text>
					</svg>
					<p>AI Tauthub sedang menulis ulang artikel dengan sudut pemberitaan baru sesuai gaya jurnalistik…</p>
				</div>
			{:else if flow.regenConfirm}
				<div style="padding:18px 18px 4px;display:grid;gap:4px;">
					<p style="margin:0;font-size:13.5px;color:var(--foreground);font-weight:600;">
						Buat ulang dengan sudut <em>{selectedLabel}</em>?
					</p>
					<p style="margin:0;font-size:13px;color:var(--muted-foreground);line-height:1.6;">
						Buat ulang akan memakan <strong style="color:var(--accent-foreground);">5 TC</strong> dari saldo Anda dan menghasilkan artikel baru dengan sudut pemberitaan yang berbeda.
					</p>
				</div>
			{:else}
				<div style="padding:16px 18px;display:grid;gap:8px;">
					<p style="margin:0;font-size:12.5px;color:var(--muted-foreground);">Pilih sudut pemberitaan baru:</p>
					{#if flow.regenAngleOptions.length === 0}
						<p style="margin:0;font-size:13px;color:var(--muted-foreground);">Memuat opsi…</p>
					{:else}
						{#each flow.regenAngleOptions as opt (opt.id)}
							{@const active = flow.regenAngleId === opt.id}
							<button
								type="button"
								class={active ? 'angle-choice angle-choice-active' : 'angle-choice'}
								onclick={() => (flow.regenAngleId = opt.id)}
							>
								<span class="angle-choice-radio{active ? ' angle-choice-radio-active' : ''}"></span>
								{opt.label}
							</button>
						{/each}
					{/if}
				</div>
			{/if}

			<div class="modal-foot">
				{#if !flow.regenBusy}
					<button
						type="button"
						class="app-btn app-btn-soft"
						onclick={() => (flow.regenConfirm ? (flow.regenConfirm = false) : (flow.regenOpen = false))}
					>
						Kembali
					</button>
					<button
						type="button"
						class="app-btn app-btn-primary"
						disabled={!flow.regenAngleId}
						onclick={() => (flow.regenConfirm ? void flow.submitRegen() : (flow.regenConfirm = true))}
					>
						{flow.regenConfirm ? 'Ya, Buat Ulang (5 Kredit)' : 'Lanjut'}
					</button>
				{/if}
			</div>
		</dialog>
	</div>
{/if}