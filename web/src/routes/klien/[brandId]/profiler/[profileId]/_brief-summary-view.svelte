<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Perender ringkasan Brief: mengubah teks ringkasan model (dump field
	// teknis) menjadi bagian berjudul dengan baris label/nilai yang terbaca.
	// Parsing ada di $lib/brief-summary.ts; komponen ini hanya tampilan.
	import { parseBriefSummary, splitFactPairs, linkify, type BriefBlock, type BriefField } from '$lib/brief-summary';

	let { content }: { content: string } = $props();

	const doc = $derived(parseBriefSummary(content ?? ''));

	/**
	 * Bagian tanpa field (mis. "USP") berisi beberapa kalimat berdiri sendiri.
	 * Daftar lebih enak dibaca daripada paragraf menumpuk, jadi paragraf
	 * berurutan digabung jadi satu daftar butir.
	 */
	const sections = $derived(
		doc.sections.map((s) => {
			if (s.blocks.some((b) => b.type === 'fields')) return s;
			const blocks: BriefBlock[] = [];
			let run: string[] = [];
			const flush = () => {
				if (run.length === 0) return;
				blocks.push(run.length >= 2 ? { type: 'ul', items: run } : { type: 'p', text: run[0] });
				run = [];
			};
			for (const b of s.blocks) {
				if (b.type === 'p') {
					run.push(b.text);
					continue;
				}
				flush();
				blocks.push(b);
			}
			flush();
			return { ...s, blocks };
		})
	);

	/** Nilai panjang (deskripsi) dibaca sebagai paragraf, bukan baris ringkas. */
	function isLongValue(v: string | undefined): boolean {
		return (v ?? '').length > 90;
	}

	/** `Tahun berdiri: 2018. Kapasitas: 1.000 orang.` → pasangan label/nilai. */
	function pairsOf(f: BriefField): Array<{ label: string; value: string }> {
		if (!f.value || isLongValue(f.value)) return [];
		return splitFactPairs(f.value);
	}
</script>

{#snippet rich(text: string)}
	{#each linkify(text) as part, i (i)}{#if part.href}<a href={part.href} target="_blank" rel="noopener noreferrer">{part.text}</a>{:else}{part.text}{/if}{/each}
{/snippet}

<div class="bsv">
	{#each sections as section, si (section.title + '-' + si)}
		<section class="bsv-section">
			{#if section.title}
				<h4 class="bsv-title">{section.title}</h4>
			{/if}
			<div class="bsv-body">
				{#each section.blocks as block, bi (bi)}
					{#if block.type === 'p'}
						<p class="bsv-p">{@render rich(block.text)}</p>
					{:else if block.type === 'ul'}
						<ul class="bsv-ul">
							{#each block.items as item, ii (ii)}
								<li>{@render rich(item)}</li>
							{/each}
						</ul>
					{:else}
						<dl class="bsv-fields">
							{#each block.rows as f, fi (f.label + '-' + fi)}
								{@const pairs = pairsOf(f)}
								<!-- Label yang sama dengan judul bagian tidak diulang; nilainya
								     langsung di bawah judul. -->
								{@const labelRedundant = f.label.trim().toLowerCase() === section.title.trim().toLowerCase()}
								<div class="bsv-field" class:bsv-field-solo={labelRedundant}>
									{#if !labelRedundant}
										<dt class="bsv-label">{f.label}</dt>
									{/if}
									<dd class="bsv-value">
										{#if f.items && f.items.length > 0}
											<ul class="bsv-ul">
												{#each f.items as item, ii (ii)}
													<li>{@render rich(item)}</li>
												{/each}
											</ul>
										{:else if pairs.length > 0}
											<ul class="bsv-pairs">
												{#each pairs as pair, pi (pi)}
													<li><span class="bsv-pair-label">{pair.label}</span><span>{@render rich(pair.value)}</span></li>
												{/each}
											</ul>
										{:else if isLongValue(f.value)}
											<p class="bsv-p">{@render rich(f.value ?? '')}</p>
										{:else if f.value}
											<span>{@render rich(f.value)}</span>
										{:else}
											<span class="bsv-empty">{tr('—')}</span>
										{/if}
									</dd>
								</div>
							{/each}
						</dl>
					{/if}
				{/each}
			</div>
		</section>
	{/each}
</div>

<style>
	.bsv {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.bsv-section {
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--card);
		overflow: hidden;
	}
	.bsv-title {
		margin: 0;
		padding: 8px 13px;
		border-bottom: 1px solid var(--border);
		background: color-mix(in srgb, var(--muted) 55%, white);
		font-size: 11.5px;
		font-weight: 700;
		letter-spacing: 0.05em;
		text-transform: uppercase;
		color: var(--muted-foreground);
	}
	.bsv-body {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 11px 13px;
	}
	.bsv-p {
		margin: 0;
		font-size: 13.5px;
		line-height: 1.65;
		color: var(--foreground);
	}
	.bsv-ul {
		margin: 0;
		padding-left: 1.1rem;
		list-style: disc;
		font-size: 13.5px;
		line-height: 1.6;
	}
	.bsv-ul :global(li)::marker {
		color: var(--muted-foreground);
	}
	.bsv-ul :global(li + li) {
		margin-top: 3px;
	}
	.bsv-fields {
		display: flex;
		flex-direction: column;
		gap: 9px;
		margin: 0;
	}
	.bsv-field {
		display: grid;
		grid-template-columns: minmax(112px, 178px) minmax(0, 1fr);
		gap: 2px 14px;
		align-items: start;
	}
	.bsv-field-solo {
		grid-template-columns: minmax(0, 1fr);
	}
	.bsv-label {
		font-size: 12px;
		font-weight: 600;
		color: var(--muted-foreground);
		padding-top: 1px;
	}
	.bsv-value {
		margin: 0;
		min-width: 0;
		font-size: 13.5px;
		line-height: 1.6;
		color: var(--foreground);
		overflow-wrap: anywhere;
	}
	.bsv-pairs {
		margin: 0;
		padding: 0;
		list-style: none;
		display: flex;
		flex-direction: column;
		gap: 3px;
	}
	.bsv-pairs :global(li) {
		display: flex;
		flex-wrap: wrap;
		gap: 5px;
	}
	.bsv-pair-label {
		color: var(--muted-foreground);
	}
	.bsv-pair-label::after {
		content: ':';
	}
	.bsv-value :global(a),
	.bsv-p :global(a),
	.bsv-ul :global(a) {
		color: var(--primary);
		text-decoration: underline;
		text-underline-offset: 2px;
		overflow-wrap: anywhere;
	}
	.bsv-empty {
		color: var(--muted-foreground);
	}
	@media (max-width: 640px) {
		.bsv-field {
			grid-template-columns: minmax(0, 1fr);
			gap: 1px;
		}
		.bsv-label {
			font-size: 11px;
			text-transform: uppercase;
			letter-spacing: 0.03em;
		}
	}
</style>
