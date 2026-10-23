<script lang="ts">
	// Bingkai "file manager" untuk halaman-halaman di dalam folder Taut Pintar:
	// bar tiga titik + breadcrumb Admin › Taut Pintar › <folder> di atas, status
	// bar Lokasi di bawah. Dipakai halaman explorer /admin/taut-pintar dan
	// seluruh halaman yang diajukan sebagai "folder" di sana.
	import type { Snippet } from 'svelte';
	import { tr } from '$lib/i18n/phrases';
	import { House, ChevronRight } from '@lucide/svelte';

	let {
		crumb,
		path,
		meta = '',
		// Segmen breadcrumb yang bisa diklik, dari luar ke dalam (mis.
		// [{ label: 'Taut Pintar', href: '/admin/taut-pintar' }]). Kosongkan
		// untuk halaman yang berdiri sendiri.
		parents = [],
		children
	}: {
		crumb: string;
		path: string;
		meta?: string;
		parents?: Array<{ label: string; href: string }>;
		children: Snippet;
	} = $props();
</script>

<div class="win">
	<div class="winBar">
		<span class="winDots" aria-hidden="true"><i></i><i></i><i></i></span>
		<nav class="crumbs" aria-label={tr('Lokasi')}>
			<a class="crumb crumbLink" href="/admin"><House size={13} /> Admin</a>
			{#each parents as p (p.href)}
				<span class="crumbSep"><ChevronRight size={13} /></span>
				<a class="crumb crumbLink" href={p.href}>{p.label}</a>
			{/each}
			<span class="crumbSep"><ChevronRight size={13} /></span>
			<span class="crumb crumbOn">{crumb}</span>
		</nav>
		{#if meta}<span class="winMeta">{meta}</span>{/if}
	</div>

	<div class="winBody">
		{@render children()}
	</div>

	<div class="statusBar">
		<span>{tr('Lokasi')}: {path}</span>
		<span>{meta}</span>
	</div>
</div>

<style>
	.win {
		border: 1px solid var(--border, #dfe5ef);
		border-radius: 14px;
		background: var(--card, #fff);
		box-shadow: 0 10px 30px rgba(15, 23, 42, 0.06);
		overflow: hidden;
	}
	.winBar {
		display: flex;
		align-items: center;
		gap: 14px;
		padding: 10px 14px;
		border-bottom: 1px solid var(--border, #dfe5ef);
		background: linear-gradient(180deg, #f8fafc, #f1f5f9);
	}
	.winDots {
		display: inline-flex;
		gap: 6px;
		flex: none;
	}
	.winDots i {
		width: 10px;
		height: 10px;
		border-radius: 999px;
		background: #e2e8f0;
	}
	.winDots i:nth-child(1) { background: #fca5a5; }
	.winDots i:nth-child(2) { background: #fcd34d; }
	.winDots i:nth-child(3) { background: #86efac; }
	.crumbs {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		min-width: 0;
		flex-wrap: wrap;
		font-size: 12.5px;
		color: var(--muted-foreground, #667085);
	}
	.crumb {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		padding: 4px 9px;
		border-radius: 8px;
		white-space: nowrap;
	}
	/* Segmen breadcrumb bisa diklik — hover memberi isyarat link. */
	a.crumbLink {
		text-decoration: none;
		color: inherit;
		cursor: pointer;
	}
	a.crumbLink:hover {
		background: #eef2f7;
		color: var(--primary, #2b67ec);
	}
	.crumbOn {
		background: #eef4ff;
		color: var(--primary, #2b67ec);
		font-weight: 600;
	}
	.crumbSep {
		display: inline-flex;
		color: #cbd5e1;
		flex: none;
	}
	.winMeta {
		margin-left: auto;
		font-size: 12px;
		color: var(--muted-foreground, #667085);
		white-space: nowrap;
	}
	.winBody {
		padding: 20px 22px 24px;
	}
	.statusBar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		padding: 8px 14px;
		border-top: 1px solid var(--border, #dfe5ef);
		background: #f8fafc;
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
		font-size: 11.5px;
		color: var(--muted-foreground, #667085);
	}
	@media (max-width: 640px) {
		.winBody {
			padding: 14px;
		}
	}
</style>
