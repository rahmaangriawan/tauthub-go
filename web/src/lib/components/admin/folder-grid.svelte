<script lang="ts">
	// Grid tile folder ala file manager — dipakai bersama explorer Taut Pintar
	// dan folder Taut Brief di dalamnya. Tambah sub-halaman = tambah entri
	// di array folders milik halaman pemanggil.
	import { Folder, ChevronRight } from '@lucide/svelte';
	import { tr } from '$lib/i18n/phrases';

	type FolderEntry = {
		label: string;
		href: string;
		description: string;
		// Nama "berkas" ala explorer yang menggambarkan isi folder.
		tag: string;
	};

	let { folders }: { folders: FolderEntry[] } = $props();
</script>

<div class="grid">
	{#each folders as f (f.href)}
		<a class="tile" href={f.href}>
			<span class="folderIcon" aria-hidden="true"><Folder size={40} strokeWidth={1.6} /></span>
			<span class="tileName">{f.label}</span>
			<span class="tileTag">{f.tag}</span>
			<span class="tileDesc">{f.description}</span>
			<span class="tileOpen">{tr('Buka folder')} <ChevronRight size={13} /></span>
		</a>
	{/each}
</div>

<style>
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
		gap: 12px;
		margin-top: 20px;
	}
	.tile {
		position: relative;
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 6px;
		padding: 16px 16px 14px;
		border: 1px solid var(--border, #dfe5ef);
		border-radius: 12px;
		background: #fff;
		text-decoration: none;
		color: inherit;
		transition: border-color 0.15s ease, box-shadow 0.15s ease, transform 0.15s ease;
	}
	.tile:hover {
		border-color: #b9cdf5;
		box-shadow: 0 8px 22px rgba(43, 103, 236, 0.1);
		transform: translateY(-2px);
	}
	.tile:hover .folderIcon :global(svg) {
		fill: #fcd34d;
	}
	.folderIcon {
		color: #f59e0b;
		fill: #fde68a;
		transition: fill 0.15s ease;
		margin-bottom: 2px;
	}
	.tileName {
		font-size: 14.5px;
		font-weight: 700;
		letter-spacing: -0.01em;
	}
	.tileTag {
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
		font-size: 11px;
		color: var(--muted-foreground, #667085);
		background: #f4f6fa;
		border: 1px solid var(--border, #dfe5ef);
		border-radius: 6px;
		padding: 2px 7px;
	}
	.tileDesc {
		font-size: 12.5px;
		line-height: 1.5;
		color: var(--muted-foreground, #667085);
		display: -webkit-box;
		-webkit-line-clamp: 3;
		line-clamp: 3;
		-webkit-box-orient: vertical;
		overflow: hidden;
	}
	.tileOpen {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		margin-top: auto;
		padding-top: 6px;
		font-size: 12px;
		font-weight: 600;
		color: var(--primary, #2b67ec);
		opacity: 0;
		transition: opacity 0.15s ease;
	}
	.tile:hover .tileOpen {
		opacity: 1;
	}

	@media (max-width: 640px) {
		.grid {
			grid-template-columns: 1fr 1fr;
		}
		.tile {
			padding: 12px;
		}
		.tileDesc {
			-webkit-line-clamp: 2;
			line-clamp: 2;
		}
	}
</style>
