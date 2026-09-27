<script lang="ts">
	// Indikator proses berputar (port TrailingDots app lama, tanpa `motion`).
	let { size = 18 }: { size?: number } = $props();
	const dots = [0, 1, 2, 3, 4];
</script>

<span
	class="trailing-dots"
	style="width:{size}px;height:{size}px;"
	aria-hidden="true"
>
	{#each dots as i (i)}
		<span class="td-ring" style="animation-delay:{i * 0.08}s;">
			<span class="td-dot" style="background:{i === 0 ? 'var(--foreground)' : 'var(--muted-foreground)'};opacity:{1 - i * 0.18};"></span>
		</span>
	{/each}
</span>

<style>
	.trailing-dots {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		position: relative;
		flex: none;
	}
	.td-ring {
		position: absolute;
		inset: 0;
		animation: td-rotate 1.5s ease-in-out infinite;
	}
	.td-dot {
		position: absolute;
		top: 0;
		left: 50%;
		width: 4px;
		height: 4px;
		border-radius: 999px;
		margin-left: -2px;
	}
	@keyframes td-rotate {
		from { transform: rotate(0deg); }
		to { transform: rotate(360deg); }
	}
	@media (prefers-reduced-motion: reduce) {
		.td-ring { animation: none; }
	}
</style>
