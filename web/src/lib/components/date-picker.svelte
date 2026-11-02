<script lang="ts">
	import { Calendar, Popover } from 'bits-ui';
	import {
		getLocalTimeZone,
		parseDate,
		today,
		type DateValue
	} from '@internationalized/date';
	import { CalendarDays, ChevronDown, ChevronLeft, ChevronRight } from '@lucide/svelte';
	import { tr } from '$lib/i18n/phrases';

	// Date picker ala shadcn-svelte: trigger 40px + popover kalender bits-ui.
	// Nilai tetap string ISO `YYYY-MM-DD` supaya kontrak API tidak berubah.
	// CSS memakai :global(.zdate-*) karena Popover.Content di-portal ke body
	// sehingga selector ter-scope milik komponen tidak akan menjangkaunya.
	let {
		value = $bindable(''),
		ariaLabel,
		min = '',
		max = '',
		placeholder = tr('Pilih tanggal'),
		onchange
	}: {
		value?: string;
		ariaLabel: string;
		min?: string;
		max?: string;
		placeholder?: string;
		onchange?: () => void;
	} = $props();

	const tz = getLocalTimeZone();
	let open = $state(false);

	function parseOrNull(raw: string): DateValue | undefined {
		if (!raw) return undefined;
		try {
			return parseDate(raw.slice(0, 10));
		} catch {
			return undefined;
		}
	}

	const selected = $derived(parseOrNull(value));
	const minValue = $derived(parseOrNull(min));
	const maxValue = $derived(parseOrNull(max));

	const display = $derived(
		selected
			? new Date(selected.year, selected.month - 1, selected.day).toLocaleDateString('id-ID', {
					day: 'numeric',
					month: 'short',
					year: 'numeric'
				})
			: ''
	);

	function pick(next: DateValue | undefined) {
		if (!next) return;
		value = `${next.year}-${String(next.month).padStart(2, '0')}-${String(next.day).padStart(2, '0')}`;
		open = false;
		onchange?.();
	}

	function pickToday() {
		const current = today(tz).toString();
		if (min && current < min.slice(0, 10)) return;
		if (max && current > max.slice(0, 10)) return;
		pick(today(tz));
	}
</script>

<Popover.Root bind:open>
	<Popover.Trigger class="zdate-trigger" aria-label={ariaLabel}>
		<CalendarDays size={14} class="zdate-trigger-icon" />
		<span class="zdate-text" class:zdate-placeholder={!display}>{display || placeholder}</span>
		<ChevronDown size={13} class="zdate-trigger-icon" />
	</Popover.Trigger>
	<Popover.Portal>
		<Popover.Content class="zdate-content" sideOffset={6} align="end" collisionPadding={12}>
			<Calendar.Root
				type="single"
				value={selected}
				onValueChange={pick}
				placeholder={selected ?? today(tz)}
				{minValue}
				{maxValue}
				weekStartsOn={1}
				weekdayFormat="short"
				locale="id-ID"
				preventDeselect
				class="zdate-calendar"
			>
				{#snippet children({ months, weekdays })}
					<Calendar.Header class="zdate-header">
						<Calendar.PrevButton class="zdate-nav">
							<ChevronLeft size={15} />
						</Calendar.PrevButton>
						<Calendar.Heading class="zdate-heading" />
						<Calendar.NextButton class="zdate-nav">
							<ChevronRight size={15} />
						</Calendar.NextButton>
					</Calendar.Header>

					<Calendar.Grid class="zdate-grid">
						<Calendar.GridHead>
							<Calendar.GridRow>
								{#each weekdays as weekday}
									<Calendar.HeadCell class="zdate-weekday">{weekday}</Calendar.HeadCell>
								{/each}
							</Calendar.GridRow>
						</Calendar.GridHead>
						<Calendar.GridBody>
							{#each months as month}
								{#each month.weeks as week}
									<Calendar.GridRow class="zdate-row">
										{#each week as day}
											<Calendar.Cell date={day} month={month.value} class="zdate-cell">
												<Calendar.Day class="zdate-day" />
											</Calendar.Cell>
										{/each}
									</Calendar.GridRow>
								{/each}
							{/each}
						</Calendar.GridBody>
					</Calendar.Grid>
				{/snippet}
			</Calendar.Root>

			<div class="zdate-footer">
				<button type="button" class="zdate-today" onclick={pickToday}>{tr('Hari ini')}</button>
			</div>
		</Popover.Content>
	</Popover.Portal>
</Popover.Root>

<style>
	:global(.zdate-trigger) {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		height: 40px;
		padding: 0 10px;
		border: 1px solid var(--input);
		border-radius: var(--radius-control);
		background: var(--card);
		color: var(--foreground);
		font: inherit;
		font-size: 13px;
		text-align: left;
		cursor: pointer;
		transition: border-color 0.16s ease, box-shadow 0.16s ease;
	}
	:global(.zdate-trigger:hover) {
		border-color: color-mix(in srgb, var(--primary) 42%, var(--input));
	}
	:global(.zdate-trigger:focus-visible) {
		outline: none;
		border-color: var(--primary);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 14%, transparent);
	}
	:global(.zdate-trigger-icon) {
		flex: 0 0 auto;
		color: var(--muted-foreground);
	}
	:global(.zdate-text) {
		flex: 1 1 auto;
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
		font-variant-numeric: tabular-nums;
	}
	:global(.zdate-placeholder) {
		color: var(--muted-foreground);
	}

	:global(.zdate-content) {
		z-index: 50;
		width: 276px;
		padding: 12px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background: var(--popover);
		color: var(--popover-foreground);
		box-shadow: 0 12px 32px rgba(16, 24, 40, 0.12);
	}

	:global(.zdate-calendar) {
		width: 252px;
	}
	:global(.zdate-header) {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		margin-bottom: 8px;
	}
	:global(.zdate-heading) {
		font-size: 13px;
		font-weight: 600;
		color: var(--foreground);
	}
	:global(.zdate-nav) {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border: 0;
		border-radius: 7px;
		background: transparent;
		color: var(--muted-foreground);
		cursor: pointer;
		transition: background-color 0.16s ease, color 0.16s ease;
	}
	:global(.zdate-nav:hover) {
		background: var(--accent);
		color: var(--primary);
	}
	:global(.zdate-nav:focus-visible) {
		outline: 2px solid var(--primary);
		outline-offset: 1px;
	}

	:global(.zdate-grid) {
		width: 100%;
		table-layout: fixed;
		border-collapse: collapse;
	}
	:global(.zdate-cell) {
		padding: 1px;
		text-align: center;
	}
	:global(.zdate-weekday) {
		padding: 0 0 6px;
		font-size: 10.5px;
		font-weight: 600;
		color: var(--muted-foreground);
		text-align: center;
	}
	:global(.zdate-day) {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border-radius: 8px;
		color: var(--foreground);
		font-size: 12.5px;
		font-variant-numeric: tabular-nums;
		cursor: pointer;
		user-select: none;
		transition: background-color 0.14s ease, color 0.14s ease;
	}
	:global(.zdate-day:hover) {
		background: var(--accent);
		color: var(--primary);
	}
	:global(.zdate-day[data-selected]) {
		background: var(--primary);
		color: var(--primary-foreground);
		font-weight: 600;
	}
	:global(.zdate-day[data-today]:not([data-selected])) {
		color: var(--primary);
		font-weight: 600;
	}
	:global(.zdate-day[data-outside-month]) {
		color: color-mix(in srgb, var(--muted-foreground) 55%, transparent);
	}
	:global(.zdate-day[data-disabled]) {
		opacity: 0.35;
		cursor: not-allowed;
	}
	:global(.zdate-day[data-unavailable]) {
		text-decoration: line-through;
	}
	:global(.zdate-day:focus-visible) {
		outline: 2px solid var(--primary);
		outline-offset: 1px;
	}

	:global(.zdate-footer) {
		margin-top: 8px;
		padding-top: 8px;
		border-top: 1px solid color-mix(in srgb, var(--border) 75%, transparent);
		display: flex;
		justify-content: flex-end;
	}
	:global(.zdate-today) {
		border: 0;
		background: transparent;
		color: var(--primary);
		font: inherit;
		font-size: 12px;
		font-weight: 600;
		padding: 4px 6px;
		border-radius: 6px;
		cursor: pointer;
	}
	:global(.zdate-today:hover) {
		background: var(--accent);
	}
	:global(.zdate-today:focus-visible) {
		outline: 2px solid var(--primary);
		outline-offset: 1px;
	}
</style>
