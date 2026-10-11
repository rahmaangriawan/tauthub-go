<script lang="ts">
	// Menu utilitas header versi MOBILE. Di layar sempit, pemilih bahasa + saldo
	// kredit tidak muat berdampingan dengan lonceng, jadi keduanya dipindah ke
	// satu dropdown yang dipicu tombol ikon (titik tiga vertikal). Di desktop
	// komponen ini tidak tampil (md:hidden) — bahasa & saldo tetap sebaris.
	import { Coins } from '@lucide/svelte';
	import { tr } from '$lib/i18n/phrases';
	import { getI18n } from '$lib/i18n/i18n.svelte';
	import FlagIcon from '$lib/components/flag-icon.svelte';
	import type { Locale } from '$lib/i18n/dictionaries';

	let {
		saldoLabel = '',
		saldoHref = null,
		onTopUp = null
	}: {
		/** Teks saldo yang ditampilkan (mis. "Rp 6.284.000"). Kosong = tanpa baris saldo. */
		saldoLabel?: string;
		/** Bila diisi, baris saldo menjadi tautan (dipakai portal mitra). */
		saldoHref?: string | null;
		/** Bila diisi, baris saldo memanggil aksi ini (mis. buka modal top up klien). */
		onTopUp?: (() => void) | null;
	} = $props();

	const i18n = getI18n();
	let open = $state(false);

	const OPTIONS: Array<{ value: Locale; label: string }> = [
		{ value: 'id', label: 'Indonesia' },
		{ value: 'en', label: 'English' }
	];

	function pick(value: Locale) {
		i18n.setLocale(value);
		open = false;
	}

	function openTopUpFromMenu() {
		open = false;
		onTopUp?.();
	}
</script>

<svelte:window onkeydown={(e) => { if (e.key === 'Escape' && open) open = false; }} />

<div class="app-utility-more-wrap">
	<button
		class="app-utility-more"
		type="button"
		aria-label={tr('Menu lainnya')}
		aria-haspopup="dialog"
		aria-expanded={open}
		onclick={() => (open = !open)}
	>
		<svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 20 20" aria-hidden="true">
			<path fill="currentColor" d="M8 4.5a.5.5 0 0 0-.5-.5h-3a.5.5 0 0 0-.5.5v3a.5.5 0 0 0 .5.5h3a.5.5 0 0 0 .5-.5zm1 3A1.5 1.5 0 0 1 7.5 9h-3A1.5 1.5 0 0 1 3 7.5v-3A1.5 1.5 0 0 1 4.5 3h3A1.5 1.5 0 0 1 9 4.5zm7-3a.5.5 0 0 0-.5-.5h-3a.5.5 0 0 0-.5.5v3a.5.5 0 0 0 .5.5h3a.5.5 0 0 0 .5-.5zm1 3A1.5 1.5 0 0 1 15.5 9h-3A1.5 1.5 0 0 1 11 7.5v-3A1.5 1.5 0 0 1 12.5 3h3A1.5 1.5 0 0 1 17 4.5zm-9 5a.5.5 0 0 0-.5-.5h-3a.5.5 0 0 0-.5.5v3a.5.5 0 0 0 .5.5h3a.5.5 0 0 0 .5-.5zm1 3A1.5 1.5 0 0 1 7.5 17h-3A1.5 1.5 0 0 1 3 15.5v-3A1.5 1.5 0 0 1 4.5 11h3A1.5 1.5 0 0 1 9 12.5zm7-3a.5.5 0 0 0-.5-.5h-3a.5.5 0 0 0-.5.5v3a.5.5 0 0 0 .5.5h3a.5.5 0 0 0 .5-.5zm1 3a1.5 1.5 0 0 1-1.5 1.5h-3a1.5 1.5 0 0 1-1.5-1.5v-3a1.5 1.5 0 0 1 1.5-1.5h3a1.5 1.5 0 0 1 1.5 1.5z" />
		</svg>
	</button>

	{#if open}
		<!-- Backdrop transparan: klik di luar menu menutupnya tanpa menggelapkan. -->
		<button class="app-utility-menu-backdrop" type="button" aria-label={tr('Tutup')} onclick={() => (open = false)}></button>
		<div class="app-utility-menu" role="dialog" aria-label={tr('Menu lainnya')}>
			<!-- Bahasa: hanya bendera, sejajar kiri-kanan. -->
			<div class="app-utility-flags" role="radiogroup" aria-label={tr('Bahasa')}>
				{#each OPTIONS as option (option.value)}
					<button
						type="button"
						role="radio"
						aria-checked={i18n.locale === option.value}
						class="app-utility-flag"
						class:active={i18n.locale === option.value}
						title={option.label}
						aria-label={option.label}
						onclick={() => pick(option.value)}
					>
						<FlagIcon locale={option.value} size={18} />
					</button>
				{/each}
			</div>

			{#if saldoLabel}
				{#if onTopUp}
					<button type="button" class="app-utility-saldo" onclick={openTopUpFromMenu}>
						<span class="app-utility-saldo-left"><Coins size={16} />{saldoLabel}</span>
						<span class="app-utility-saldo-cta">{tr('Top Up')}</span>
					</button>
				{:else if saldoHref}
					<a class="app-utility-saldo" href={saldoHref}>
						<span class="app-utility-saldo-left"><Coins size={16} />{saldoLabel}</span>
						<span class="app-utility-saldo-cta">{tr('Lihat')}</span>
					</a>
				{:else}
					<div class="app-utility-saldo">
						<span class="app-utility-saldo-left"><Coins size={16} />{saldoLabel}</span>
					</div>
				{/if}
			{/if}
		</div>
	{/if}
</div>

<style>
	.app-utility-more-wrap {
		position: relative;
		display: none;
		flex: 0 0 auto;
	}
	/* Tombol pemicu: mengikuti bentuk pil saldo di header. */
	.app-utility-more {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 36px;
		height: 32px;
		padding: 0;
		border: 0;
		background: transparent;
		color: var(--foreground);
		cursor: pointer;
	}
	.app-utility-more:hover { color: var(--primary); }
	@media (max-width: 767px) {
		.app-utility-more-wrap { display: block; }
	}

	.app-utility-menu-backdrop {
		position: fixed;
		inset: 0;
		z-index: 1140;
		border: 0;
		padding: 0;
		background: transparent;
		cursor: default;
	}
	/* Dropdown menempel di bawah tombol, rata kanan — bukan sheet dari bawah. */
	.app-utility-menu {
		position: absolute;
		top: calc(100% + 8px);
		right: 0;
		z-index: 1150;
		display: flex;
		flex-direction: column;
		gap: 8px;
		min-width: 210px;
		padding: 10px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--card, #fff);
		box-shadow: var(--shadow-md, 0 12px 30px rgba(15, 23, 42, 0.18));
		animation: utilMenuIn 0.14s cubic-bezier(0.16, 1, 0.3, 1);
	}
	@keyframes utilMenuIn {
		from { opacity: 0; transform: translateY(-6px) scale(0.98); }
		to { opacity: 1; transform: translateY(0) scale(1); }
	}
	@media (prefers-reduced-motion: reduce) {
		.app-utility-menu { animation: none; }
	}

	/* Bahasa: dua bendera sejajar kiri-kanan, tanpa teks. */
	.app-utility-flags {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.app-utility-flag {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 44px;
		height: 36px;
		border: 1px solid var(--border);
		border-radius: 9px;
		background: transparent;
		cursor: pointer;
	}
	.app-utility-flag:hover { background: var(--secondary); }
	.app-utility-flag.active {
		border-color: var(--primary);
		background: var(--accent, #eef4ff);
	}

	.app-utility-saldo {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		width: 100%;
		min-height: 40px;
		padding: 6px 10px;
		border: 1px solid var(--border);
		border-radius: 9px;
		background: transparent;
		color: var(--foreground);
		font: inherit;
		font-size: 13.5px;
		text-align: left;
		text-decoration: none;
		cursor: pointer;
	}
	.app-utility-saldo:hover { background: var(--secondary); }
	.app-utility-saldo-left { display: inline-flex; align-items: center; gap: 8px; font-weight: 700; }
	.app-utility-saldo-left :global(svg) { color: var(--success); }
	.app-utility-saldo-cta { font-size: 12px; font-weight: 700; color: var(--primary); }
</style>
