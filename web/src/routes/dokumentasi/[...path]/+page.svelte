<script lang="ts">
	import { tick } from 'svelte';
	import { ChevronLeft, ChevronRight } from '@lucide/svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	let activeId = $state('');
	let article = $state<HTMLElement | null>(null);

	// Tandai bagian yang sedang dibaca di daftar isi; dipasang ulang tiap halaman.
	$effect(() => {
		void data.html;
		activeId = '';
		let io: IntersectionObserver | undefined;
		void tick().then(() => {
			const heads = article?.querySelectorAll<HTMLElement>('h2[id], h3[id]');
			if (!heads || heads.length === 0) return;
			io = new IntersectionObserver(
				(entries) => {
					const visible = entries.filter((e) => e.isIntersecting);
					if (visible.length) activeId = (visible[0].target as HTMLElement).id;
				},
				{ rootMargin: '-72px 0px -70% 0px' }
			);
			heads.forEach((h) => io?.observe(h));
		});
		return () => io?.disconnect();
	});
</script>

<div class="dk-doc">
	<main class="dk-main">
		<nav class="dk-crumbs" aria-label="Jejak">
			<a href="/dokumentasi">Dokumentasi</a>
			<span aria-hidden="true">/</span>
			<a href={`/dokumentasi/${data.doc.section}`}>{data.doc.sectionLabel}</a>
			{#each data.crumbs.filter((c) => c.url !== `/dokumentasi/${data.doc.section}`) as c (c.url)}
				<span aria-hidden="true">/</span>
				<a href={c.url}>{c.title}</a>
			{/each}
		</nav>

		<article class="dk-article" bind:this={article}>
			<h1>{data.doc.title}</h1>
			{#if data.lead}
				<div class="dk-lede">{@html data.lead}</div>
			{/if}
			<div class="dk-prose">{@html data.html}</div>
		</article>

		<nav class="dk-pager" aria-label="Halaman berikutnya">
			{#if data.prev}
				<a class="dk-pager-prev" href={data.prev.url}>
					<span><ChevronLeft size={14} strokeWidth={1.5} /> Sebelumnya</span>
					<strong>{data.prev.title}</strong>
				</a>
			{:else}<span></span>{/if}
			{#if data.next}
				<a class="dk-pager-next" href={data.next.url}>
					<span>Berikutnya <ChevronRight size={14} strokeWidth={1.5} /></span>
					<strong>{data.next.title}</strong>
				</a>
			{/if}
		</nav>
	</main>

	{#if data.toc.length > 1}
		<aside class="dk-toc" aria-label="Di halaman ini">
			<p>Di halaman ini</p>
			<ul>
				{#each data.toc as t (t.id)}
					<li class:sub={t.depth === 3}>
						<a href={`#${t.id}`} class:active={activeId === t.id}>{t.text}</a>
					</li>
				{/each}
			</ul>
		</aside>
	{/if}
</div>
