<script lang="ts">
	// Pagination katalog: tautan biasa (bukan tombol JS) supaya crawler bisa
	// mengikuti setiap halaman.
	let {
		pageNo,
		pages,
		hrefFor
	}: { pageNo: number; pages: number; hrefFor: (p: number) => string } = $props();

	const windowPages = $derived.by(() => {
		const out: number[] = [];
		const from = Math.max(1, pageNo - 2);
		const to = Math.min(pages, pageNo + 2);
		for (let p = from; p <= to; p++) out.push(p);
		return out;
	});
</script>

{#if pages > 1}
	<nav class="pub-pagination" aria-label="Halaman katalog">
		<a
			class="pub-page-btn"
			class:is-disabled={pageNo <= 1}
			href={hrefFor(Math.max(1, pageNo - 1))}
			rel="prev"
			aria-disabled={pageNo <= 1}>← Sebelumnya</a
		>
		{#if windowPages[0] > 1}
			<a class="pub-page-btn" href={hrefFor(1)}>1</a>
			{#if windowPages[0] > 2}<span class="pub-page-btn is-disabled">…</span>{/if}
		{/if}
		{#each windowPages as p (p)}
			<a class="pub-page-btn" class:is-active={p === pageNo} href={hrefFor(p)} aria-current={p === pageNo ? 'page' : undefined}
				>{p}</a
			>
		{/each}
		{#if windowPages[windowPages.length - 1] < pages}
			{#if windowPages[windowPages.length - 1] < pages - 1}<span class="pub-page-btn is-disabled">…</span>{/if}
			<a class="pub-page-btn" href={hrefFor(pages)}>{pages}</a>
		{/if}
		<a
			class="pub-page-btn"
			class:is-disabled={pageNo >= pages}
			href={hrefFor(Math.min(pages, pageNo + 1))}
			rel="next"
			aria-disabled={pageNo >= pages}>Berikutnya →</a
		>
	</nav>
{/if}
