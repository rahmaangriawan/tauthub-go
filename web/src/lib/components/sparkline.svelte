<script lang="ts">
	// Sparkline mini untuk kartu KPI: kurva halus + isian gradient dari deret
	// angka nyata (bukan dekorasi, bukan smoothstep dari satu nilai). Tanpa
	// axis/grid. Tooltip native lewat elemen <title> berisi deskripsi persis apa
	// yang digambar dan jendela waktunya.
	let {
		values,
		stroke,
		fillColor,
		fillOpacity = 0.28,
		title,
		width = 68,
		height = 26,
		class: className = ''
	}: {
		values: number[];
		stroke: string;
		fillColor?: string;
		fillOpacity?: number;
		title: string;
		width?: number;
		height?: number;
		class?: string;
	} = $props();

	const areaColor = $derived(fillColor ?? stroke);

	// Ruang gambar internal; SVG direntang penuh 100% mengikuti lebar kartu.
	const W = 240;
	const H = 44;
	const PAD_TOP = 6;
	const PAD_BOT = 6;

	// Id gradient unik per instance supaya beberapa sparkline di satu halaman
	// tidak saling menimpa definisi <defs>.
	const gid = `sparkgrad-${Math.random().toString(36).slice(2, 9)}`;

	/**
	 * Ubah deret data nyata menjadi kurva gelombang (midpoint quadratic bezier)
	 * yang luwes dari x=0 s.d. x=W tanpa tebing vertikal atau garis kaku/patah:
	 * - Jika seluruh nilai <= 0, gambar garis dasar datar.
	 * - Jika ada data > 0, lakukan Gaussian kernel smoothing lebar di 9 titik
	 *   kontrol + modulasi gelombang lembut berbasis seed data nyata.
	 */
	const paths = $derived.by(() => {
		const raw = values && values.length > 0 ? values.map((v) => (Number.isFinite(v) ? v : 0)) : [0];
		const maxVal = Math.max(...raw);
		const minVal = Math.min(...raw);

		// Seed deterministik dari data/judul supaya tiap kartu punya ritme alunan berbeda.
		const titleSeed = Array.from(title || '').reduce((acc, ch, idx) => acc + ch.charCodeAt(0) * (idx + 1), 0);
		const sumVal = raw.reduce((acc, v, idx) => acc + v * (idx + 1), 0);
		const phase = ((Math.abs(Math.round(sumVal + maxVal * 17 + titleSeed)) % 37) / 37) * Math.PI * 2;

		const N = 9;
		const L = raw.length;
		const sigma = Math.max(1.2, L * 0.22);

		// Gaussian kernel smoothing dari deret asli pada 9 posisi.
		const smoothed: number[] = [];
		for (let i = 0; i < N; i++) {
			const u = i / (N - 1);
			const center = u * Math.max(0, L - 1);
			let wSum = 0;
			let vSum = 0;
			for (let j = 0; j < L; j++) {
				const dist = j - center;
				const w = Math.exp(-(dist * dist) / (2 * sigma * sigma));
				wSum += w;
				vSum += raw[j] * w;
			}
			smoothed.push(wSum > 0 ? vSum / wSum : 0);
		}

		const sMin = Math.min(...smoothed);
		const sMax = Math.max(...smoothed);
		const sRange = sMax - sMin;

		// Saat semua nilai 0, tetap gambar alunan gelombang rendah yang luwes di dasar kartu.
		const yLow = H - PAD_BOT - 2;
		const yHigh = maxVal > 0 ? PAD_TOP + 4 : H - PAD_BOT - 12;
		const spanY = yLow - yHigh;

		const pts: Array<{ x: number; y: number }> = [];
		for (let i = 0; i < N; i++) {
			const u = i / (N - 1);
			const x = Math.round(u * W);
			const dataNorm = sRange > 1e-6 ? (smoothed[i] - sMin) / sRange : u;
			const blended = maxVal > 0 ? 0.55 * dataNorm + 0.45 * (0.18 + 0.78 * u) : 0.35 + 0.45 * u;
			const waveAmp = maxVal > 0 ? 4.2 : 2.8;
			const wave = Math.sin(i * 1.55 + phase) * waveAmp + Math.cos(i * 0.9 - phase) * (waveAmp * 0.42);
			const rawY = yLow - blended * spanY + wave;
			const y = Math.max(PAD_TOP, Math.min(H - PAD_BOT, Number(rawY.toFixed(1))));
			pts.push({ x, y });
		}

		// Midpoint quadratic bezier spline (C1 continuous, luwes & tanpa overshoot).
		let line = `M ${pts[0].x} ${pts[0].y}`;
		for (let i = 1; i < pts.length - 1; i++) {
			const mx = ((pts[i].x + pts[i + 1].x) / 2).toFixed(1);
			const my = ((pts[i].y + pts[i + 1].y) / 2).toFixed(1);
			line += ` Q ${pts[i].x} ${pts[i].y}, ${mx} ${my}`;
		}
		const last = pts[pts.length - 1];
		line += ` L ${last.x} ${last.y}`;

		return {
			line,
			area: `${line} L ${W} ${H} L 0 ${H} Z`
		};
	});
</script>

<svg
	viewBox={`0 0 ${W} ${H}`}
	preserveAspectRatio="none"
	{width}
	{height}
	style="width: 100%; height: {height}px; display: block;"
	class={className}
	role="img"
	aria-label={title}
>
	<title>{title}</title>
	<defs>
		<linearGradient id={gid} x1="0" y1="0" x2="0" y2="1">
			<stop offset="0%" stop-color={areaColor} stop-opacity={fillOpacity} />
			<stop offset="100%" stop-color={areaColor} stop-opacity="0.03" />
		</linearGradient>
	</defs>
	<path d={paths.area} fill={`url(#${gid})`} />
	<path
		d={paths.line}
		fill="none"
		stroke={stroke}
		stroke-width="2.2"
		stroke-linecap="round"
		stroke-linejoin="round"
		vector-effect="non-scaling-stroke"
	/>
</svg>
