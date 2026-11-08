<script lang="ts">
	// Garis-garis mengalir untuk panel kiri halaman auth. Port dari blok
	// "Login 3" Hirael (MIT, Mohammad Shehadeh), yang di sana memakai
	// motion/react; di sini animasinya CSS murni: pathLength="1" menormalkan
	// panjang tiap path jadi 1 unit, lalu stroke-dasharray/-dashoffset
	// digerakkan supaya garis tampak menjalar, menghilang, dan muncul lagi.
	// Durasi tiap garis dibedakan dengan jitter deterministik (sin), sama
	// seperti aslinya, supaya 72 garis tidak bergerak serempak.
	let { position = 1 }: { position?: 1 | -1 } = $props();

	const jitter = (i: number) => {
		const value = Math.sin(i + 1) * 10_000;
		return value - Math.floor(value);
	};

	const paths = Array.from({ length: 36 }, (_, i) => ({
		id: i,
		d: `M-${380 - i * 5 * position} -${189 + i * 6}C-${380 - i * 5 * position} -${189 + i * 6} -${
			312 - i * 5 * position
		} ${216 - i * 6} ${152 - i * 5 * position} ${343 - i * 6}C${616 - i * 5 * position} ${470 - i * 6} ${
			684 - i * 5 * position
		} ${875 - i * 6} ${684 - i * 5 * position} ${875 - i * 6}`,
		width: 0.5 + i * 0.03,
		opacity: Math.min(1, 0.1 + i * 0.03),
		duration: 20 + jitter(i) * 10
	}));
</script>

<div class="fp" aria-hidden="true">
	<svg class="fp-svg" fill="none" viewBox="0 0 696 316">
		{#each paths as p (p.id)}
			<path
				class="fp-path"
				d={p.d}
				pathLength="1"
				stroke="currentColor"
				stroke-opacity={p.opacity}
				stroke-width={p.width}
				style="animation-duration: {p.duration.toFixed(2)}s"
			/>
		{/each}
	</svg>
</div>

<style>
	.fp {
		position: absolute;
		inset: 0;
		pointer-events: none;
	}
	.fp-svg {
		display: block;
		width: 100%;
		height: 100%;
		color: var(--auth-primary, #2563eb);
		opacity: 0.6;
	}
	.fp-path {
		stroke-dasharray: 1 1;
		stroke-dashoffset: 0;
		animation-name: fpFlow;
		animation-timing-function: linear;
		animation-iteration-count: infinite;
	}
	@keyframes fpFlow {
		0% {
			stroke-dasharray: 0.3 1;
			stroke-dashoffset: 0;
		}
		50% {
			stroke-dasharray: 0.65 1;
			stroke-dashoffset: -1;
		}
		100% {
			stroke-dasharray: 1 1;
			stroke-dashoffset: 0;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.fp-path {
			animation: none;
			stroke-dasharray: 1 1;
		}
	}
</style>
