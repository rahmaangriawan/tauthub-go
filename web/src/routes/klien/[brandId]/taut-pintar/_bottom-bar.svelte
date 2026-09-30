<script lang="ts">
	// Bar aksi bawah halaman Taut Pintar — parity `.bottomBar` (writer,
	// matchmaker, gatekeeper) dan `.stickyFooter` (profiler) di app lama:
	// fixed di bawah, mulai dari tepi kanan sidebar (264px) sampai tepi kanan
	// viewport, dengan garis atas + bayangan ke atas. Spacer di aliran normal
	// diukur dari tinggi bar asli supaya konten terakhir tidak tertutup —
	// tinggi bar berbeda-beda per langkah dan bisa membungkus di layar sempit.
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	let barEl = $state<HTMLElement | null>(null);
	let barHeight = $state(84);

	$effect(() => {
		const el = barEl;
		if (!el) return;
		const measure = () => (barHeight = el.offsetHeight);
		measure();
		const observer = new ResizeObserver(measure);
		observer.observe(el);
		return () => observer.disconnect();
	});
</script>

<div class="taut-bottom-spacer" style="height: {barHeight}px" aria-hidden="true"></div>
<div class="taut-bottom-bar" bind:this={barEl}>
	<div class="taut-bottom-inner">{@render children()}</div>
</div>

<style>
	.taut-bottom-bar {
		position: fixed;
		left: 264px;
		right: 0;
		bottom: 0;
		z-index: 30;
		background: var(--card);
		border-top: 1px solid var(--border);
		box-shadow: 0 -4px 16px rgba(15, 23, 42, 0.05);
	}
	.taut-bottom-inner {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		max-width: 1280px;
		margin: 0 auto;
		padding: 12px 32px;
	}
	/* Sidebar disembunyikan di bawah 768px (app.css) — bar ikut melebar penuh. */
	@media (max-width: 767px) {
		.taut-bottom-bar {
			left: 0;
		}
		.taut-bottom-inner {
			padding: 10px 16px;
		}
	}
</style>
