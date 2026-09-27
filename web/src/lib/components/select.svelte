<script lang="ts">
	// Dropdown bergaya (pengganti <select> bawaan browser) — parity Radix
	// Select/DropdownMenu di app lama: trigger 38px, radius 8px, border tipis,
	// menu mengambang dengan item terpilih (tebal + biru + centang) dan pill
	// jumlah opsional. Menu diposisikan fixed supaya tidak terpotong overflow
	// induk (tabel/kartu), persis seperti portal Radix.
	import type { Component, Snippet } from 'svelte';
	import { Check, ChevronDown } from '@lucide/svelte';

	export type SelectOption = { value: string; label: string; count?: number; disabled?: boolean };

	let {
		value = $bindable(''),
		options = [],
		ariaLabel,
		placeholder = 'Pilih',
		Icon = null,
		minWidth = 180,
		align = 'start',
		menuMaxHeight = 340,
		variant = 'default',
		disabled = false,
		onchange,
		footer
	}: {
		value?: string;
		options: SelectOption[];
		ariaLabel: string;
		placeholder?: string;
		Icon?: Component<{ size?: number; color?: string }> | null;
		minWidth?: number;
		align?: 'start' | 'end';
		menuMaxHeight?: number;
		variant?: 'default' | 'plain';
		disabled?: boolean;
		onchange?: (value: string, option: SelectOption | null) => void;
		footer?: Snippet;
	} = $props();

	let open = $state(false);
	let trigger = $state<HTMLButtonElement | null>(null);
	let menu = $state<HTMLDivElement | null>(null);
	let pos = $state({ left: 0, top: 0, width: 0 });
	let activeIndex = $state(-1);

	const selected = $derived(options.find((o) => o.value === value) ?? null);
	const hasValue = $derived(selected != null && selected.value !== '');

	function openMenu() {
		if (!trigger) return;
		const rect = trigger.getBoundingClientRect();
		const width = Math.max(minWidth, rect.width);
		const maxLeft = Math.max(8, window.innerWidth - width - 8);
		const left = align === 'end' ? Math.max(8, rect.right - width) : Math.min(rect.left, maxLeft);
		pos = { left, top: rect.bottom + 4, width };
		activeIndex = Math.max(
			0,
			options.findIndex((o) => o.value === value)
		);
		open = true;
	}

	function closeMenu() {
		open = false;
	}

	function pick(option: SelectOption) {
		if (option.disabled) return;
		value = option.value;
		onchange?.(option.value, option);
		open = false;
		trigger?.focus();
	}

	function moveActive(delta: number) {
		if (options.length === 0) return;
		let next = activeIndex;
		for (let step = 0; step < options.length; step++) {
			next = (next + delta + options.length) % options.length;
			if (!options[next].disabled) break;
		}
		activeIndex = next;
	}

	function onTriggerKeydown(event: KeyboardEvent) {
		if (event.key === 'ArrowDown' || event.key === 'ArrowUp' || event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			if (!open) openMenu();
			else if (open && (event.key === 'ArrowDown' || event.key === 'ArrowUp')) {
				moveActive(event.key === 'ArrowDown' ? 1 : -1);
			}
		}
	}

	function onMenuKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			event.preventDefault();
			closeMenu();
			trigger?.focus();
			return;
		}
		if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
			event.preventDefault();
			moveActive(event.key === 'ArrowDown' ? 1 : -1);
			return;
		}
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			const option = options[activeIndex];
			if (option) pick(option);
		}
	}

	$effect(() => {
		if (!open) return;
		menu?.focus();
		const onScroll = () => closeMenu();
		window.addEventListener('scroll', onScroll, true);
		window.addEventListener('resize', onScroll);
		return () => {
			window.removeEventListener('scroll', onScroll, true);
			window.removeEventListener('resize', onScroll);
		};
	});
</script>

<button
	bind:this={trigger}
	type="button"
	class="zs-select-trigger"
	class:zs-select-active={hasValue}
	class:zs-select-plain={variant === 'plain'}
	aria-haspopup="listbox"
	aria-expanded={open}
	aria-label={ariaLabel}
	disabled={disabled}
	onclick={() => (open ? closeMenu() : openMenu())}
	onkeydown={onTriggerKeydown}
>
	{#if Icon}
		<span class="zs-select-icon" class:zs-select-icon-active={hasValue}>
			<Icon size={16} color={hasValue ? '#2563eb' : '#64748b'} />
		</span>
	{/if}
	<span class="zs-select-label">{selected?.label ?? placeholder}</span>
	<span class="zs-select-caret" class:zs-select-caret-active={hasValue}>
		<ChevronDown size={14} />
	</span>
</button>

{#if open}
	<button type="button" class="zs-select-backdrop" aria-label="Tutup pilihan" onclick={closeMenu}></button>
	<div
		bind:this={menu}
		class="zs-select-menu"
		role="listbox"
		aria-label={ariaLabel}
		tabindex="-1"
		style="left: {pos.left}px; top: {pos.top}px; min-width: {pos.width}px; max-height: {menuMaxHeight}px;"
		onkeydown={onMenuKeydown}
	>
		{#each options as option, index (option.value)}
			<button
				type="button"
				role="option"
				aria-selected={option.value === value}
				class="zs-select-item"
				class:zs-select-item-selected={option.value === value}
				class:zs-select-item-active={index === activeIndex}
				disabled={option.disabled}
				onclick={() => pick(option)}
				onmousemove={() => (activeIndex = index)}
			>
				<span class="zs-select-item-label">{option.label}</span>
				<span class="zs-select-item-right">
					{#if option.count != null}
						<span class="zs-select-count" class:zs-select-count-selected={option.value === value}>
							{option.count}
						</span>
					{/if}
					{#if option.value === value}
						<Check size={14} />
					{/if}
				</span>
			</button>
		{/each}
		{#if footer}
			<div class="zs-select-footer">{@render footer()}</div>
		{/if}
	</div>
{/if}

<style>
	.zs-select-trigger {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		height: 38px;
		padding: 0 12px;
		max-width: 260px;
		border: 1px solid #e2e8f0;
		border-radius: 8px;
		background: var(--card, #ffffff);
		color: #334155;
		font: inherit;
		font-size: 13px;
		font-weight: 500;
		cursor: pointer;
		outline: none;
		white-space: nowrap;
		transition:
			background-color 0.15s ease,
			border-color 0.15s ease,
			color 0.15s ease;
	}
	.zs-select-trigger:hover {
		background: #f8fafc;
		border-color: #cbd5e1;
		color: #0f172a;
	}
	.zs-select-trigger:focus-visible {
		border-color: #2563eb;
		box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12);
	}
	.zs-select-trigger.zs-select-active {
		background: #eff6ff;
		border-color: #bfdbfe;
		color: #2563eb;
	}
	.zs-select-plain {
		border-color: transparent;
		background: transparent;
	}

	.zs-select-icon {
		display: inline-flex;
		align-items: center;
		flex-shrink: 0;
		color: #64748b;
	}
	.zs-select-icon-active {
		color: #2563eb;
	}
	.zs-select-label {
		max-width: 190px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.zs-select-caret {
		flex-shrink: 0;
		color: #64748b;
	}
	.zs-select-caret-active {
		color: #2563eb;
	}

	.zs-select-backdrop {
		position: fixed;
		inset: 0;
		z-index: 1090;
		border: none;
		padding: 0;
		background: transparent;
		cursor: default;
	}

	.zs-select-menu {
		position: fixed;
		z-index: 1100;
		display: flex;
		flex-direction: column;
		overflow-y: auto;
		padding: 5px;
		border: 1px solid #e2e8f0;
		border-radius: 10px;
		background: #ffffff;
		box-shadow: 0 8px 24px rgba(15, 23, 42, 0.12);
		animation: zsSelectIn 0.12s cubic-bezier(0.16, 1, 0.3, 1);
		transform-origin: top center;
	}
	.zs-select-menu:focus {
		outline: none;
	}
	@keyframes zsSelectIn {
		from {
			opacity: 0;
			transform: translateY(-4px) scale(0.98);
		}
		to {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
	}

	.zs-select-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		width: 100%;
		min-height: 34px;
		padding: 6px 10px;
		border: none;
		border-radius: 7px;
		background: transparent;
		color: #0f172a;
		font: inherit;
		font-size: 13px;
		font-weight: 400;
		text-align: left;
		cursor: pointer;
	}
	.zs-select-item:hover,
	.zs-select-item-active {
		background: #f1f5f9;
	}
	.zs-select-item-selected {
		color: #2563eb;
		font-weight: 700;
	}
	.zs-select-item:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.zs-select-item-label {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.zs-select-item-right {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		flex-shrink: 0;
		color: #2563eb;
	}
	.zs-select-count {
		font-size: 11px;
		font-weight: 600;
		padding: 1px 6px;
		border-radius: 999px;
		background: #f1f5f9;
		color: #64748b;
	}
	.zs-select-count-selected {
		background: #dbeafe;
		color: #1d4ed8;
	}
	.zs-select-footer {
		margin-top: 4px;
		padding-top: 6px;
		border-top: 1px solid #f1f5f9;
	}
</style>
