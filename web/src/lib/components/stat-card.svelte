<script lang="ts">
	// Kartu statistik ringkasan (KPI) — dipakai oleh dashboard mitra supaya
	// keempat kartu punya struktur, spacing, dan tinggi yang identik. Isi kartu
	// ditulis sekali sebagai snippet lama dirender ke <div> (non-interaktif) atau
	// <a> (bila ada href). Sparkline hanya digambar bila ada satu nilai > 0 —
	// tanpa grafik palsu dari data kosong.
	import Sparkline from '$lib/components/sparkline.svelte';
	import type { Component } from 'svelte';

	type Tone = 'neutral' | 'primary' | 'warning' | 'success';

	let {
		label,
		value,
		hint,
		tone = 'neutral',
		icon: Icon,
		sparkline,
		href
	}: {
		label: string;
		value: string;
		hint?: string;
		tone?: Tone;
		icon?: Component<{ size?: number }>;
		sparkline?: { values: number[]; stroke: string; title: string };
		href?: string;
	} = $props();

	type ToneStyle = { iconBox: string; label: string; value: string; hint: string };

	const toneStyles: Record<Tone, ToneStyle> = {
		neutral: {
			iconBox:
				'background-color: var(--accent); color: var(--accent-foreground); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--primary) 16%, transparent);',
			label: 'color: var(--muted-foreground);',
			value: 'color: var(--foreground);',
			hint: 'color: var(--muted-foreground);'
		},
		primary: {
			iconBox:
				'background-color: var(--accent); color: var(--primary); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--primary) 24%, transparent);',
			label: 'color: var(--primary);',
			value: 'color: var(--primary);',
			hint: 'color: var(--primary); font-weight: 500;'
		},
		warning: {
			iconBox:
				'background-color: var(--warning-soft); color: var(--warning); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--warning) 28%, transparent);',
			label: 'color: var(--warning);',
			value: 'color: var(--warning);',
			hint: 'color: var(--warning); font-weight: 500;'
		},
		success: {
			iconBox:
				'background-color: var(--card); color: var(--success); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--success) 30%, transparent);',
			label: 'color: var(--success);',
			value: 'color: var(--success);',
			hint: 'color: var(--success); font-weight: 500;'
		}
	};

	const t = $derived(toneStyles[tone]);
	// Kartu sukses punya dasar hijau-soft dengan border hijau tipis (spec §2).
	const cardStyle = $derived(
		tone === 'success'
			? 'background-color: var(--success-soft); border-color: color-mix(in srgb, var(--success) 30%, transparent);'
			: ''
	);
	const hasHistory = $derived(Boolean(sparkline?.values?.some((v) => v > 0)));
</script>

{#if href}
	<a {href} class="app-card flex h-full flex-col gap-3 p-5 outline-offset-2 transition-shadow hover:shadow-md" style={cardStyle}>
		{@render body()}
	</a>
{:else}
	<div class="app-card flex h-full flex-col gap-3 p-5" style={cardStyle}>
		{@render body()}
	</div>
{/if}

{#snippet body()}
	<div class="flex items-start justify-between gap-3">
		<span class="text-[11px] leading-[1.4] font-bold uppercase tracking-[0.06em]" style={t.label}>{label}</span>
		{#if Icon}
			<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-[10px]" style={t.iconBox}>
				<Icon size={17} />
			</div>
		{/if}
	</div>
	<div class="mt-auto">
		<div class="text-[26px] leading-[1.12] font-bold tracking-[-0.03em] tabular-nums" style={t.value}>{value}</div>
		{#if hint}
			<div class="mt-1.5 text-[12.5px] leading-[1.45]" style={t.hint}>{hint}</div>
		{/if}
	</div>
	{#if sparkline && hasHistory}
		<Sparkline
			values={sparkline.values}
			stroke={sparkline.stroke}
			title={sparkline.title}
			width={240}
			height={32}
			class="mt-1 w-full shrink-0"
		/>
	{/if}
{/snippet}
