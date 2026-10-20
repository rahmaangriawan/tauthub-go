<script lang="ts">
	// Sparkline mini untuk kartu KPI: kurva halus + isian gradient dari deret
	// angka nyata (bukan dekorasi, bukan smoothstep dari satu nilai). Tanpa
	// axis/grid. Tooltip native lewat elemen <title> berisi deskripsi persis apa
	// yang digambar dan jendela waktunya.
	let {
		values,
		stroke,
		title,
		width = 68,
		height = 26,
		class: className = ''
	}: {
		values: number[];
		stroke: string;
		title: string;
		width?: number;
		height?: number;
		class?: string;
	} = $props();

	// Ruang gambar internal; SVG direntang ke width/height final.
	const W = 100;
	const H = 32;
	const PAD = 3;

	// Id gradient unik per instance supaya beberapa sparkline di satu halaman
	// tidak saling menimpa definisi <defs>.
	const gid = `sparkgrad-${Math.random().toString(36).slice(2, 9)}`;

	const points = $derived.by(() => {
		const vals = values.length > 0 ? values : [0];
		const min = Math.min(...vals);
		const max = Math.max(...vals);
		const range = max - min;
		return vals.map((v, i) => ({
			x: vals.length === 1 ? 0 : (i / (vals.length - 1)) * W,
			// Deret datar (termasuk semua nol) digambar rata di dasar.
			y: range === 0 ? H - PAD : H - PAD - ((v - min) / range) * (H - PAD * 2)
		}));
	});

	// Catmull-Rom → cubic bezier: titik tetap dari data nyata, hanya kontrol
	// interpolasi antar titik yang melengkung (tension 1/6, kurva stabil).
	const linePath = $derived.by(() => {
		const pts = points;
		if (pts.length === 0) return '';
		if (pts.length === 1) return `M0,${pts[0].y}`;
		let d = `M${pts[0].x.toFixed(2)},${pts[0].y.toFixed(2)}`;
		for (let i = 1; i < pts.length; i++) {
			const p0 = pts[i - 1];
			const p1 = pts[i];
			const prev = pts[i - 2] ?? p0;
			const next = pts[i + 1] ?? p1;
			const c1x = p0.x + (p1.x - prev.x) / 6;
			const c1y = p0.y + (p1.y - prev.y) / 6;
			const c2x = p1.x - (next.x - p0.x) / 6;
			const c2y = p1.y - (next.y - p0.y) / 6;
			d += ` C${c1x.toFixed(2)},${c1y.toFixed(2)} ${c2x.toFixed(2)},${c2y.toFixed(2)} ${p1.x.toFixed(2)},${p1.y.toFixed(2)}`;
		}
		return d;
	});
	const areaPath = $derived(
		`${linePath} L${points[points.length - 1].x.toFixed(2)},${H} L0,${H} Z`
	);
</script>

<svg
	viewBox={`0 0 ${W} ${H}`}
	preserveAspectRatio="none"
	{width}
	{height}
	class={className}
	role="img"
	aria-label={title}
>
	<title>{title}</title>
	<defs>
		<linearGradient id={gid} x1="0" y1="0" x2="0" y2="1">
			<stop offset="0%" stop-color={stroke} stop-opacity="0.26" />
			<stop offset="100%" stop-color={stroke} stop-opacity="0.04" />
		</linearGradient>
	</defs>
	<path d={areaPath} fill={`url(#${gid})`} />
	<path
		d={linePath}
		fill="none"
		stroke={stroke}
		stroke-width="2"
		stroke-linecap="round"
		stroke-linejoin="round"
		vector-effect="non-scaling-stroke"
	/>
</svg>
