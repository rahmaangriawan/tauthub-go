<script lang="ts">
	// Teks multi-baris (isi pengumuman) dirender sebagai paragraf yang
	// benar-benar terpisah: satu enter = satu paragraf dengan jarak vertikal.
	// Sebelumnya `whitespace-pre-line` hanya memberi ganti baris tanpa jarak,
	// sehingga paragraf tampak menyatu dan admin harus mengetik enter dua kali.
	type Props = {
		text: string | null | undefined;
		/** Class wrapper — ukuran/warna teks diwarisi semua paragraf. */
		class?: string;
	};

	let { text, class: className = '' }: Props = $props();

	const paragraphs = $derived(
		(text ?? '')
			.split(/\r?\n+/)
			.map((line) => line.trim())
			.filter((line) => line.length > 0)
	);
</script>

<div class="{className} space-y-2">
	{#each paragraphs as paragraph}
		<p>{paragraph}</p>
	{/each}
</div>
