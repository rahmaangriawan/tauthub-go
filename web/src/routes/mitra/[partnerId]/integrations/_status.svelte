<script lang="ts">
	// Titik status koneksi — satu bahasa status untuk seluruh halaman Integrasi
	// (tab, header kartu, baris): hijau terhubung, merah gagal, kuning sedang
	// diuji, abu belum terhubung. Tanpa pill; warna hanya di titiknya.
	type Tone = 'ok' | 'error' | 'warn' | 'idle';

	let { tone = 'idle', label = '', size = 'sm' }: { tone?: Tone; label?: string; size?: 'sm' | 'xs' } = $props();

	const color: Record<Tone, string> = {
		ok: 'var(--success)',
		error: 'var(--destructive)',
		warn: 'var(--warning)',
		idle: 'var(--muted-foreground)'
	};
</script>

<span class="int-status" class:int-status--xs={size === 'xs'}>
	<span class="int-status__dot" style="background: {color[tone]};"></span>
	{#if label}<span class="int-status__label" style="color: {tone === 'idle' ? 'var(--muted-foreground)' : color[tone]};">{label}</span>{/if}
</span>

<style>
	.int-status {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		line-height: 16px;
		white-space: nowrap;
	}
	.int-status--xs {
		gap: 6px;
		font-size: 12px;
	}
	.int-status__dot {
		display: inline-block;
		width: 8px;
		height: 8px;
		border-radius: 4px;
		flex: 0 0 auto;
	}
	.int-status--xs .int-status__dot {
		width: 6px;
		height: 6px;
		border-radius: 3px;
	}
	.int-status__label {
		font-weight: 500;
	}
</style>
