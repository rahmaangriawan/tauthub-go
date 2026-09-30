<script lang="ts" generics="Row = Record<string, unknown>">
	import { tr } from '$lib/i18n/phrases';
	// Tabel admin generik: kolom + baris + status loading/kosong + klik baris.
	// Menjaga tampilan tabel seragam di semua halaman /admin.
	import type { Snippet } from 'svelte';

	type Column = {
		key: string;
		label: string;
		align?: 'left' | 'right' | 'center';
		width?: string;
		render?: Snippet<[Row]>;
	};

	let {
		columns,
		rows,
		rowKey,
		loading = false,
		empty = 'Belum ada data.',
		minWidth = '760px',
		onRowClick
	}: {
		columns: Column[];
		rows: Row[];
		rowKey: (row: Row, index: number) => string;
		loading?: boolean;
		empty?: string;
		minWidth?: string;
		onRowClick?: (row: Row) => void;
	} = $props();

	const alignClass = (align?: string) =>
		align === 'right' ? 'text-right' : align === 'center' ? 'text-center' : 'text-left';
</script>

<div class="app-card mt-4 overflow-x-auto">
	{#if loading}
		<p class="px-4 py-10 text-center text-sm text-muted-foreground">{tr("Memuat data…")}</p>
	{:else if rows.length === 0}
		<p class="px-4 py-10 text-center text-sm text-muted-foreground">{empty}</p>
	{:else}
		<table class="admin-table w-full text-sm" style="--admin-table-min: {minWidth};">
			<thead>
				<tr class="text-xs text-muted-foreground">
					{#each columns as col (col.key)}
						<th class="px-4 py-2.5 font-medium {alignClass(col.align)}" style={col.width ? `width:${col.width};` : undefined}>
							{col.label}
						</th>
					{/each}
				</tr>
			</thead>
			<tbody>
				{#each rows as row, index (rowKey(row, index))}
					<tr
						class="border-t {onRowClick ? 'cursor-pointer hover:bg-secondary' : ''}"
						onclick={onRowClick ? () => onRowClick(row) : undefined}
					>
						{#each columns as col (col.key)}
							<td class="px-4 py-2.5 {alignClass(col.align)}">
								{#if col.render}
									{@render col.render(row)}
								{:else}
									{String((row as Record<string, unknown>)[col.key] ?? '—')}
								{/if}
							</td>
						{/each}
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</div>

<style>
	/* Lebar minimum hanya berlaku di layar lapang. Di bawah 768px tabel
	   mengikuti lebar kartu dan sel membungkus, seperti tabel Radix di app
	   lama, supaya tidak muncul scroll horizontal di dalam kartu. */
	.admin-table { min-width: var(--admin-table-min, 760px); }
	@media (max-width: 767px) {
		.admin-table { min-width: 0; }
		.admin-table :global(th),
		.admin-table :global(td) { word-break: break-word; }
	}
</style>
