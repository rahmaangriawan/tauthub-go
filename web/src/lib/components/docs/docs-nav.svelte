<script lang="ts">
	import DocsNav from './docs-nav.svelte';

	type Node = { title: string; url: string; children: Node[] };
	let { nodes, current, depth = 0 }: { nodes: Node[]; current: string; depth?: number } = $props();

	const within = (n: Node) => current === n.url || current.startsWith(`${n.url}/`);
</script>

<ul class="dk-nav-list" class:dk-nav-sub={depth > 0}>
	{#each nodes as n (n.url)}
		<li>
			{#if n.children.length}
				<details open={within(n)}>
					<summary class:active={current === n.url}>
						<a href={n.url} aria-current={current === n.url ? 'page' : undefined}>{n.title}</a>
						<span class="dk-chev" aria-hidden="true"></span>
					</summary>
					<DocsNav nodes={n.children} {current} depth={depth + 1} />
				</details>
			{:else}
				<a
					class="dk-nav-link"
					class:active={current === n.url}
					href={n.url}
					aria-current={current === n.url ? 'page' : undefined}>{n.title}</a
				>
			{/if}
		</li>
	{/each}
</ul>
