<script lang="ts">
	// Renderer input field brief berbasis skema (FieldMeta.widget).
	//
	// Satu komponen untuk semua tipe: text, longtext, number, url, select,
	// multiselect, boolean. Dipakai form Instan Brief (dan bisa dipakai ulang
	// di panel Entitas bila nanti mode edit dibuka penuh). Tidak memanggil API
	// sendiri — nilai dikendalikan parent lewat bind:value.
	import { tr } from '$lib/i18n/phrases';

	type Props = {
		keyName: string;
		label: string;
		widget?: string | null;
		value: string;
		placeholder?: string;
		required?: boolean;
		options?: string[];
		wide?: boolean;
		disabled?: boolean;
	};
	let {
		keyName,
		label,
		widget = 'text',
		value = $bindable(''),
		placeholder = '',
		required = false,
		options = [],
		wide = false,
		disabled = false
	}: Props = $props();

	const kind = $derived(
		['text', 'longtext', 'number', 'url', 'select', 'multiselect', 'boolean'].includes(String(widget))
			? String(widget)
			: 'text'
	);

	// Multiselect disimpan sebagai satu string berpemisah ";" — sama dengan
	// yang dipakai API (field value selalu string).
	const selected = $derived(
		value
			.split(';')
			.map((v) => v.trim())
			.filter(Boolean)
	);

	function toggleOption(opt: string) {
		const next = selected.includes(opt) ? selected.filter((v) => v !== opt) : [...selected, opt];
		value = next.join('; ');
	}
</script>

<label class="brief-field" class:brief-field-wide={wide || kind === 'longtext'}>
	<span class="brief-field-label">
		{label}
		{#if required}<em>*</em>{/if}
	</span>

	{#if kind === 'longtext'}
		<textarea
			class="app-input brief-field-input"
			rows={3}
			{placeholder}
			{disabled}
			bind:value
			aria-label={label}
		></textarea>
	{:else if kind === 'select'}
		<select class="app-input brief-field-input" {disabled} bind:value aria-label={label}>
			<option value="">{tr('Pilih…')}</option>
			{#each options as opt (opt)}
				<option value={opt}>{opt}</option>
			{/each}
		</select>
	{:else if kind === 'multiselect'}
		<div class="brief-field-chips" role="group" aria-label={label}>
			{#each options as opt (opt)}
				{@const on = selected.includes(opt)}
				<button
					class="brief-field-chip"
					class:on
					type="button"
					{disabled}
					aria-pressed={on}
					onclick={() => toggleOption(opt)}
				>{opt}</button>
			{/each}
		</div>
	{:else if kind === 'boolean'}
		<label class="brief-field-switch">
			<input type="checkbox" checked={value === 'true'} {disabled} onchange={(e) => (value = (e.currentTarget as HTMLInputElement).checked ? 'true' : 'false')} />
			<span>{value === 'true' ? tr('Ya') : tr('Tidak')}</span>
		</label>
	{:else}
		<input
			class="app-input brief-field-input"
			type={kind === 'number' ? 'number' : kind === 'url' ? 'url' : 'text'}
			{placeholder}
			{disabled}
			bind:value
			aria-label={label}
			data-field={keyName}
		/>
	{/if}
</label>

<style>
	.brief-field {
		display: flex;
		flex-direction: column;
		gap: 4px;
		min-width: 0;
	}
	.brief-field-wide { grid-column: 1 / -1; }
	.brief-field-label {
		font-size: 11.5px;
		font-weight: 600;
		color: var(--muted-foreground);
	}
	.brief-field-label em {
		font-style: normal;
		color: var(--destructive, #dc2626);
	}
	.brief-field-input { width: 100%; }
	.brief-field-chips {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
	.brief-field-chip {
		padding: 4px 10px;
		border: 1px solid var(--border, #e2e8f0);
		border-radius: 999px;
		background: var(--card, #fff);
		font-size: 11.5px;
		color: var(--muted-foreground);
		cursor: pointer;
	}
	.brief-field-chip.on {
		border-color: color-mix(in srgb, var(--primary) 45%, var(--border, #e2e8f0));
		background: color-mix(in srgb, var(--primary) 10%, white);
		color: var(--primary);
		font-weight: 600;
	}
	.brief-field-switch {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		font-size: 12.5px;
	}
</style>
