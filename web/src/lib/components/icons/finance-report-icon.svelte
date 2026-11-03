<script lang="ts">
	// Ikon menu "Keuangan Tauthub" di sidebar admin: uang dalam lingkaran.
	//
	// Digambar sendiri, bukan dari @lucide/svelte, karena ikon grafik batang
	// (ChartBar) yang dipakai sebelumnya juga dipakai menu "PR & Analitik" di
	// grup lain — di sidebar keduanya tampak kembar. Varian uang-lingkaran
	// dengan bentuk ini tidak ada di paket lucide yang terpasang.
	//
	// Menerima `size`, `strokeWidth`, dan `class` supaya bisa dipasang lewat
	// jalur ikon nav yang sama dengan ikon lucide (app-shell meneruskan
	// keduanya) maupun di tempat lain yang butuh kelas warna/ukuran.
	let {
		size = 18,
		strokeWidth = 2,
		class: className = ''
	}: { size?: number; strokeWidth?: number; class?: string } = $props();
</script>

<svg
	xmlns="http://www.w3.org/2000/svg"
	width={size}
	height={size}
	viewBox="0 0 24 24"
	fill="none"
	stroke="currentColor"
	stroke-width={strokeWidth}
	stroke-linecap="round"
	stroke-linejoin="round"
	class={className}
	aria-hidden="true"
>
	<path d="M12 22c5.523 0 10-4.477 10-10S17.523 2 12 2S2 6.477 2 12s4.477 10 10 10" />
	<path
		d="M15 8.5c-.685-.685-1.891-1.161-3-1.191M9 15c.644.86 1.843 1.35 3 1.391m0-9.082c-1.32-.036-2.5.561-2.5 2.191c0 3 5.5 1.5 5.5 4.5c0 1.711-1.464 2.446-3 2.391m0-9.082V5.5m0 10.891V18.5"
	/>
</svg>
