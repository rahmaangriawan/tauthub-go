<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Modal pemilihan jenis akun sebelum OAuth Google (brief §6-§8).
	// Penting: JANGAN membuat akun di sini. Modal hanya memilih `role`, lalu
	// pemanggil mengarahkan ke /v1/auth/google/url?mode=register&role=...
	import { ShieldCheck, ArrowRight } from '@lucide/svelte';

	let {
		open,
		role,
		busy = false,
		onSelect,
		onClose,
		onContinue
	}: {
		open: boolean;
		role: 'client' | 'partner' | null;
		busy?: boolean;
		onSelect: (role: 'client' | 'partner') => void;
		onClose: () => void;
		onContinue: () => void;
	} = $props();

	// Overlay memakai <button> transparan sebagai lapisan luar: klik di luar
	// modal menutup, dan elemennya tetap dapat dijangkau keyboard lewat Escape
	// pada <svelte:window> (brief §9).
	let dialogEl: HTMLDivElement | undefined = $state();
	let overlayEl: HTMLButtonElement | undefined = $state();

	$effect(() => {
		if (!open) return;
		// Fokuskan dialog saat dibuka agar Tab mulai dari dalam modal.
		dialogEl?.focus();
	});

	function onKeydown(e: KeyboardEvent) {
		if (!open) return;
		if (e.key === 'Escape') {
			e.preventDefault();
			onClose();
		}
	}
</script>

<svelte:window onkeydown={onKeydown} />

{#if open}
	<div class="auth-modal-overlay">
		<!-- Lapisan klik-di-luar: elemen interaktif asli (bukan div ber-onclick). -->
		<button
			bind:this={overlayEl}
			type="button"
			class="auth-modal-backdrop"
			aria-label={tr("Tutup dialog")}
			tabindex="-1"
			onclick={onClose}
		></button>
		<div
			bind:this={dialogEl}
			class="auth-modal"
			role="dialog"
			aria-modal="true"
			aria-labelledby="auth-role-title"
			tabindex="-1"
		>
			<span class="auth-modal-badge">{tr("DAFTAR AKUN")}</span>
			<h2 class="auth-modal-title" id="auth-role-title">{tr("Daftar sebagai apa?")}</h2>
			<p class="auth-modal-sub">{tr("Pilih jenis akun yang akan Anda buat dengan Google.")}</p>

			<div class="auth-rolecards" role="radiogroup" aria-label={tr("Pilih jenis akun Google")}>
				<button
					type="button"
					class="auth-rolecard"
					role="radio"
					aria-checked={role === 'client'}
					onclick={() => onSelect('client')}
				>
					<span class="auth-radiodot" aria-hidden="true"></span>
					<span class="auth-rolecard-art" aria-hidden="true">
						<img src="/images/role-client-v2.webp" alt="" />
					</span>
					<span class="auth-rolecard-title">{tr("Klien")}</span>
					<span class="auth-rolecard-desc">{tr("Untuk perusahaan, tim, atau individu yang ingin menggunakan layanan Tauthub.")}</span>
				</button>

				<button
					type="button"
					class="auth-rolecard"
					role="radio"
					aria-checked={role === 'partner'}
					onclick={() => onSelect('partner')}
				>
					<span class="auth-radiodot" aria-hidden="true"></span>
					<span class="auth-rolecard-art" aria-hidden="true">
						<img src="/images/role-partner-v2.webp" alt="" />
					</span>
					<span class="auth-rolecard-title">{tr("Mitra")}</span>
					<span class="auth-rolecard-desc">{tr("Untuk media partner, penerbit, atau pemilik media yang ingin bekerja sama dengan Tauthub.")}</span>
				</button>
			</div>

			<div class="auth-modal-footer">
				<div class="auth-secure">
					<ShieldCheck size={16} strokeWidth={2} />
					<div>{tr("Akun Anda aman dengan login Google.")}<small>{tr("Kami tidak akan pernah menyalahgunakan data Anda.")}</small>
					</div>
				</div>
				<div class="auth-modal-actions">
					<button type="button" class="auth-btn-ghost" onclick={onClose} disabled={busy}>{tr("Batal")}</button>
					<!-- Disabled sampai peran dipilih (brief §8). -->
					<button
						type="button"
						class="auth-btn-primary"
						disabled={!role || busy}
						onclick={onContinue}
					>
						{busy ? tr('Menghubungkan…') : tr('Lanjut ke Google')}
						<ArrowRight size={14} strokeWidth={2.4} />
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
