<script lang="ts">
	// Kartu statistik ringkasan (KPI) — dipakai oleh dashboard mitra supaya
	// keempat kartu punya struktur, spacing, dan tinggi yang identik: baris atas
	// (ikon kiri + label + pill delta "dari bulan lalu"), angka utama, hint,
	// lalu sparkline lebar di dasar kartu. Pill delta hanya ditampilkan dari
	// data nyata (prop `delta`); tanpa pembanding valid dirender tanda "—".
	import Sparkline from '$lib/components/sparkline.svelte';
	import { tr } from '$lib/i18n/phrases';
	import type { Component } from 'svelte';
	import { ArrowDown, ArrowUp } from '@lucide/svelte';

	type Tone = 'neutral' | 'primary' | 'warning' | 'success';
	type CardPalette = {
		cardStyle?: string;
		iconBoxStyle?: string;
		valueStyle?: string;
		pillStyle?: string;
	};

	let {
		label,
		value,
		hint,
		tone = 'neutral',
		palette,
		icon: Icon,
		sparkline,
		href,
		delta
	}: {
		label: string;
		value: string;
		hint?: string;
		tone?: Tone;
		palette?: CardPalette;
		icon?: Component<{ size?: number }>;
		sparkline?: { values: number[]; stroke: string; fillColor?: string; fillOpacity?: number; title: string };
		href?: string;
		delta?: number | null;
	} = $props();

	type ToneStyle = { iconBox: string; value: string; hint: string };

	const toneStyles: Record<Tone, ToneStyle> = {
		neutral: {
			iconBox:
				'background-color: var(--accent); color: var(--primary); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--primary) 16%, transparent);',
			value: 'color: var(--foreground);',
			hint: 'color: var(--muted-foreground);'
		},
		primary: {
			iconBox:
				'background-color: var(--accent); color: var(--primary); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--primary) 24%, transparent);',
			value: 'color: var(--foreground);',
			hint: 'color: var(--muted-foreground);'
		},
		warning: {
			iconBox:
				'background-color: var(--warning-soft); color: var(--warning); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--warning) 28%, transparent);',
			value: 'color: var(--foreground);',
			hint: 'color: var(--muted-foreground);'
		},
		success: {
			iconBox:
				'background-color: var(--card); color: var(--success); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--success) 30%, transparent);',
			value: 'color: var(--success);',
			hint: 'color: var(--muted-foreground);'
		}
	};

	const t = $derived(toneStyles[tone]);
	const iconBoxStyle = $derived(palette?.iconBoxStyle ?? t.iconBox);
	const valueStyle = $derived(palette?.valueStyle ?? t.value);
	// Kartu sukses punya dasar hijau-soft dengan border hijau tipis (spec §2),
	// kecuali palette kustom diberikan.
	const cardStyle = $derived(
		palette?.cardStyle ??
			(tone === 'success'
				? 'background-color: var(--success-soft); border-color: color-mix(in srgb, var(--success) 30%, transparent);'
				: '')
	);
	const deltaSign = $derived(
		delta == null ? '' : delta > 0.5 ? 'up' : delta < -0.5 ? 'down' : 'flat'
	);
	const deltaText = $derived(
		delta == null ? '—' : `${delta > 0 ? '+' : delta < 0 ? '-' : ''}${Math.abs(Math.round(delta))}%`
	);
	// Pill hijau = naik; merah = turun; "—"/datar = netral (tanpa klaim angka).
	const deltaStyle = $derived.by(() => {
		if (palette?.pillStyle) return palette.pillStyle;
		if (delta == null || deltaSign === 'flat')
			return 'background-color: var(--secondary); color: var(--muted-foreground);';
		if (deltaSign === 'down')
			return 'background-color: color-mix(in srgb, var(--destructive) 10%, transparent); color: var(--destructive);';
		return 'background-color: var(--success-soft); color: var(--success);';
	});
</script>

{#if href}
	<a {href} class="app-card flex h-full flex-col gap-3.5 p-5 outline-offset-2 transition-shadow hover:shadow-md" style={cardStyle}>
		{@render body()}
	</a>
{:else}
	<div class="app-card flex h-full flex-col gap-3.5 p-5" style={cardStyle}>
		{@render body()}
	</div>
{/if}

{#snippet body()}
	<div class="flex items-center justify-between gap-3">
		<div class="flex min-w-0 items-center gap-3">
			{#if Icon}
				<div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-[12px]" style={iconBoxStyle}>
					<Icon size={19} />
				</div>
			{/if}
			<span class="truncate text-[12px] font-semibold uppercase tracking-[0.05em]" style="color: var(--foreground);">{label}</span>
		</div>
		{#if delta !== undefined}
			<div class="flex shrink-0 flex-col items-end gap-0.5">
				<span
					class="inline-flex items-center gap-0.5 rounded-full px-2 py-0.5 text-[11px] font-semibold tabular-nums"
					style={deltaStyle}
				>
					{#if deltaSign === 'up'}<ArrowUp size={10} />{:else if deltaSign === 'down'}<ArrowDown size={10} />{/if}
					{deltaText}
				</span>
				<span class="text-[10.5px] text-muted-foreground">{tr('dari bulan lalu')}</span>
			</div>
		{/if}
	</div>
	<div class="mt-auto">
		<div class="text-[30px] leading-[1.1] font-bold tracking-[-0.03em] tabular-nums" style={valueStyle}>{value}</div>
		{#if hint}
			<div class="mt-1.5 text-[13px] leading-[1.45]" style={t.hint}>{hint}</div>
		{/if}
	</div>
	{#if sparkline}
		<Sparkline
			values={sparkline.values}
			stroke={sparkline.stroke}
			fillColor={sparkline.fillColor}
			fillOpacity={sparkline.fillOpacity}
			title={sparkline.title}
			width={240}
			height={44}
			class="mt-2 w-full shrink-0"
		/>
	{/if}
{/snippet}
