<script lang="ts">
	// Panel "Coming Soon" untuk fitur yang belum dirilis — parity project lama
	// (brand-workspace.tsx / distribution.tsx: 🚧 + border dashed + latar abu).
	//
	// Beda dengan WorkInProgress (banner tipis di atas konten): komponen ini
	// MENGGANTIKAN isi area, jadi tidak ada data setengah-jadi yang terlihat.
	import { tr } from '$lib/i18n/phrases';

	let {
		/** Kalimat penjelas, mis. "sedang kami siapkan dan akan segera tersedia". */
		message,
		/** Tinggi padding; halaman penuh pakai 'lg'. */
		size = 'md'
	}: { message: string; size?: 'md' | 'lg' } = $props();
</script>

<div class="coming-soon" data-size={size} role="status">
	<span class="cs-emoji" aria-hidden="true">🚧</span>
	<strong class="cs-title">Coming Soon</strong>
	<span class="cs-message">{tr(message)}</span>
</div>

<style>
	.coming-soon {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 10px;
		padding: 48px 24px;
		border: 1px dashed var(--border);
		border-radius: 14px;
		background: var(--muted);
		text-align: center;
	}
	.coming-soon[data-size='lg'] {
		padding: 56px 24px;
	}
	.cs-emoji {
		font-size: 36px;
		line-height: 1;
	}
	.coming-soon[data-size='lg'] .cs-emoji {
		font-size: 40px;
	}
	.cs-title {
		font-size: 18px;
		letter-spacing: 0.02em;
		color: var(--foreground);
	}
	.coming-soon[data-size='lg'] .cs-title {
		font-size: 20px;
	}
	.cs-message {
		max-width: 460px;
		font-size: 14px;
		line-height: 1.55;
		color: var(--muted-foreground);
	}
</style>
