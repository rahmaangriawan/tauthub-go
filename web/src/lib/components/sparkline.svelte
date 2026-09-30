<script lang="ts">
	// Sparkline mini untuk kartu KPI: garis + area isian tipis dari deret angka
	// nyata (bukan dekorasi, bukan smoothstep dari satu nilai). Tanpa axis, grid,
	// glow, atau gradient. Tooltip native lewat elemen <title> berisi deskripsi
	// persis apa yang digambar dan jendela waktunya.
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

	const linePath = $derived(
		points.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x.toFixed(2)},${p.y.toFixed(2)}`).join(' ')
	);
	const areaPath = $derived(
		`${linePath} L${points[points.length - 1].x.toFixed(2)},${H} L${points[0].x.toFixed(2)},${H} Z`
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
	<path d={areaPath} fill={stroke} fill-opacity="0.12" />
	<path
		d={linePath}
		fill="none"
		stroke={stroke}
		stroke-width="1.5"
		stroke-linecap="round"
		stroke-linejoin="round"
		vector-effect="non-scaling-stroke"
	/>
</svg>
