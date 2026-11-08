<script lang="ts">
	// Latar gradasi bergelombang (WebGL) untuk hero dan kartu. Isinya ada di
	// gradient-wave-engine.ts; komponen ini hanya menyiapkan kontainer dan
	// memulai ulang engine saat opsinya berubah.
	import { GradientWaveEngine, type GradientWaveOptions } from './gradient-wave-engine';

	type Props = Partial<GradientWaveOptions> & { class?: string };

	let {
		colors = ['#0b1b4a', '#2b67ec', '#7fb6ff', '#eaf4ff'],
		isPlaying = true,
		shadowPower = 8,
		darkenTop = false,
		noiseSpeed = 0.00001,
		noiseFrequency = [0.0001, 0.0009],
		deform = { incline: 0.5, noiseAmp: 250, noiseFlow: 5 },
		class: className = ''
	}: Props = $props();

	let container: HTMLDivElement | undefined = $state();

	$effect(() => {
		if (!container) return;
		let engine: GradientWaveEngine;
		try {
			engine = new GradientWaveEngine(container, {
				colors,
				isPlaying,
				shadowPower,
				darkenTop,
				noiseSpeed,
				noiseFrequency,
				deform
			});
		} catch (err) {
			// Tanpa WebGL latar tidak tampil, tetapi halaman tetap berfungsi.
			console.error('Gagal memulai gradient wave:', err);
			return;
		}
		return () => engine.destroy();
	});
</script>

<div bind:this={container} class="gw {className}" aria-hidden="true"></div>

<style>
	.gw {
		position: absolute;
		inset: 0;
		z-index: 0;
		width: 100%;
		height: 100%;
		overflow: hidden;
		pointer-events: none;
	}
</style>
