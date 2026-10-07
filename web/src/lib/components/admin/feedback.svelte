<script lang="ts">
	// Blok umpan balik admin: loading / error / notice. Dipakai semua halaman
	// supaya pesan status tampil dengan gaya yang sama.
	let {
		loading = false,
		error = '',
		notice = '',
		loadingText = 'Memuat data…',
		rows = 6,
		skeleton = 'rows'
	}: {
		loading?: boolean;
		error?: string;
		notice?: string;
		loadingText?: string;
		/** Jumlah baris skeleton saat loading; 'none' untuk teks lama. */
		rows?: number;
		skeleton?: 'rows' | 'none';
	} = $props();
</script>

{#if notice}
	<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>
{/if}
{#if error}
	<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>
{/if}
{#if loading && skeleton === 'rows' && rows > 0}
	<div class="skWrap" aria-hidden="true">
		{#each Array(rows) as _, i}
			<div class="skRow">
				<div class="skBar" style="width: {26 + ((i * 7) % 20)}%"></div>
				<div class="skBar skBarSm" style="width: 14%"></div>
				<div class="skBar skBarSm" style="width: 9%"></div>
			</div>
		{/each}
	</div>
	<span class="sr-only">{loadingText}</span>
{:else if loading}
	<p class="mt-6 text-sm text-muted-foreground">{loadingText}</p>
{/if}

<style>
	/* Placeholder skeleton: lebih terasa "sedang memuat" daripada layar kosong. */
	.skWrap { display: flex; flex-direction: column; gap: 10px; padding: 16px 2px 6px; }
	.skRow { display: flex; align-items: center; gap: 14px; }
	.skBar { height: 12px; border-radius: 6px; background: var(--muted); animation: skPulse 1.4s ease-in-out infinite; }
	.skBarSm { height: 10px; }
	@keyframes skPulse { 0%, 100% { opacity: 1 } 50% { opacity: .45 } }
</style>
