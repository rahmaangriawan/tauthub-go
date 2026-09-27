<script lang="ts">
	// Meta title per halaman. Root layout sudah memasang title default
	// "Tauthub"; komponen ini menimpanya dengan judul spesifik halaman memakai
	// template yang sama seperti app lama (`%s — Tauthub`).
	// `title` boleh reaktif (mis. memuat nama brand) sehingga judul ikut berubah
	// setelah data termuat, tanpa mengubah struktur halaman.
	let { title, suffix = 'Tauthub' }: { title: string; suffix?: string } = $props();
</script>

<svelte:head>
	<title>{title ? `${title} — ${suffix}` : suffix}</title>
</svelte:head>
