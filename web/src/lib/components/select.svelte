<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Dropdown bergaya (pengganti <select> bawaan browser) — parity Radix
	// Select/DropdownMenu di app lama: trigger 38px, radius 8px, border tipis,
	// menu mengambang dengan item terpilih (tebal + biru + centang) dan pill
	// jumlah opsional. Menu diposisikan fixed supaya tidak terpotong overflow
	// induk (tabel/kartu), persis seperti portal Radix.
	import type { Component, Snippet } from 'svelte';
	import { Check, ChevronDown } from '@lucide/svelte';

	export type SelectOption = { value: string; label: string; description?: string; count?: number; disabled?: boolean };

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
		searchable = false,
		searchPlaceholder = 'Cari…',
		onchange,
		footer,
		popularCount = 0,
		popularLabel = 'Paling populer',
		showAllLabel = 'Lihat semua',
		showLessLabel = 'Tampilkan lebih sedikit',
			card = false,
			allOptionsLabel = '',
			fitContent = false
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
			/** Tampilkan kolom pencarian di dalam menu (untuk daftar panjang). */
			searchable?: boolean;
			searchPlaceholder?: string;
			onchange?: (value: string, option: SelectOption | null) => void;
			footer?: Snippet;
			/** >0: tampilkan hanya N opsi teratas (urut populer) plus tombol "Lihat semua". */
			popularCount?: number;
			popularLabel?: string;
			showAllLabel?: string;
			showLessLabel?: string;
			/** true: opsi mode populer dirender sebagai kartu (judul + deskripsi). */
			card?: boolean;
			allOptionsLabel?: string;
			/** true: tombol trigger dan label teks menyesuaikan lebar konten (fit-content) */
			fitContent?: boolean;
		} = $props();

	let open = $state(false);
	let trigger = $state<HTMLButtonElement | null>(null);
	let menu = $state<HTMLDivElement | null>(null);
	let searchInput = $state<HTMLInputElement | null>(null);
	let pos = $state({ left: 0, width: 0 });
	let menuTop = $state(0);
	let placement = $state<'down' | 'up'>('down');
	let menuHeight = $state(0);
	let activeIndex = $state(-1);
	let query = $state('');
	// Mode "populer": default hanya menampilkan N opsi teratas; tombol di
	// bawah menu membuka seluruh daftar. Pencarian selalu mencari ke SEMUA opsi.
	let showAll = $state(false);

	const selected = $derived(options.find((o) => o.value === value) ?? null);
	const hasValue = $derived(selected != null && selected.value !== '');
	const searching = $derived(searchable && query.trim() !== '');
	// Daftar yang ditampilkan: seluruh opsi, atau hasil filter kolom pencarian.
	const matches = $derived(
		searching ? options.filter((o) => o.label.toLowerCase().includes(query.trim().toLowerCase())) : options
	);
	const popularMode = $derived(popularCount > 0 && !searching);
	const visible = $derived(popularMode && !showAll ? matches.slice(0, popularCount) : matches);
	// Kartu butuh ruang jauh lebih tinggi daripada baris biasa, jadi batasnya
	// mengikuti tinggi layar. Ruang di atas/bawah trigger tetap jadi pembatas
	// akhir, sehingga menu tidak pernah melewati tepi layar.
	let heightLimit = $state(340);

	function openMenu() {
		if (!trigger) return;
		heightLimit = card ? window.innerHeight - 16 : menuMaxHeight;
		const rect = trigger.getBoundingClientRect();
		const width = minWidth > 0 ? Math.max(minWidth, rect.width) : rect.width;
		const maxLeft = Math.max(8, window.innerWidth - width - 8);
		const left = align === 'end' ? Math.max(8, rect.right - width) : Math.min(rect.left, maxLeft);
		// Perkirakan tinggi menu dulu (dibatasi heightLimit) supaya tahu apakah
		// muat ke bawah. Bila tidak muat dan ruang di atas lebih lapang, menu
		// dibuka KE ATAS (drop-up) agar bagian bawah tidak terpotong layar.
		const gap = 4;
		const searchHeight = searchable ? 44 : 0;
		const listLen = popularCount > 0 ? Math.min(popularCount, options.length) : options.length;
		const rowHeight = card ? 90 : 34;
		const estimated = Math.min(listLen * rowHeight + 12 + searchHeight + (popularCount > 0 ? 40 : 0), heightLimit);
		const spaceBelow = window.innerHeight - rect.bottom - gap - 8;
		const spaceAbove = rect.top - gap - 8;
		const placeUp = spaceBelow < estimated && spaceAbove > spaceBelow;
		const available = Math.max(120, placeUp ? spaceAbove : spaceBelow);
		placement = placeUp ? 'up' : 'down';
		menuHeight = Math.min(estimated, available);
		pos = { left, width };
		menuTop = placeUp ? rect.top - gap - menuHeight : rect.bottom + gap;
		query = '';
		showAll = false;
		activeIndex = Math.max(
			0,
			visible.findIndex((o) => o.value === value)
		);
		open = true;
	}

	function closeMenu() {
		open = false;
	}

	// Tinggi alami isi menu, dihitung dari anak-anaknya. `scrollHeight` tidak
	// bisa dipakai: saat isi lebih pendek dari menu ia ikut tinggi menu, jadi
	// menu tidak pernah menyusut. `getBoundingClientRect` juga tidak akurat
	// untuk footer sticky, karena posisinya bergeser dari alur normal.
	function contentHeight(el: HTMLElement) {
		const pad = parseFloat(getComputedStyle(el).paddingTop) + parseFloat(getComputedStyle(el).paddingBottom);
		let total = pad;
		for (const child of el.children) {
			const s = getComputedStyle(child);
			total += (child as HTMLElement).offsetHeight + parseFloat(s.marginTop) + parseFloat(s.marginBottom);
		}
		return total;
	}

	// Tinggi dan arah buka dihitung ulang setiap isi menu berubah (buka/tutup
	// daftar penuh, pencarian). Dengan begitu "Lihat semua" memanjang mengikuti
	// jumlah kartu — atau membuka ke atas bila ruang di bawah tidak cukup —
	// bukan mempertahankan tinggi daftar populer yang pendek.
	$effect(() => {
		if (!open || !menu || !trigger) return;
		// Dependensi reaktif: isi menu berubah.
		void showAll;
		void query;
		void visible.length;
		const rect = trigger.getBoundingClientRect();
		const gap = 4;
		const wanted = Math.min(contentHeight(menu), heightLimit);
		const spaceBelow = window.innerHeight - rect.bottom - gap - 8;
		const spaceAbove = rect.top - gap - 8;
		const placeUp = spaceBelow < wanted && spaceAbove > spaceBelow;
		const available = Math.max(120, placeUp ? spaceAbove : spaceBelow);
		const height = Math.min(wanted, available);
		placement = placeUp ? 'up' : 'down';
		menuHeight = height;
		menuTop = placeUp ? rect.top - gap - height : rect.bottom + gap;
	});

	function pick(option: SelectOption) {
		if (option.disabled) return;
		value = option.value;
		onchange?.(option.value, option);
		open = false;
		trigger?.focus();
	}

	function moveActive(delta: number) {
		const list = visible;
		if (list.length === 0) return;
		let next = activeIndex;
		for (let step = 0; step < list.length; step++) {
			next = (next + delta + list.length) % list.length;
			if (!list[next].disabled) break;
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
			const option = visible[activeIndex];
			if (option) pick(option);
		}
	}

	/** Kolom pencarian: mengetik memfilter, panah/Enter tetap menavigasi daftar. */
	function onSearchKeydown(event: KeyboardEvent) {
		if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
			event.preventDefault();
			moveActive(event.key === 'ArrowDown' ? 1 : -1);
			return;
		}
		if (event.key === 'Enter') {
			event.preventDefault();
			const option = visible[activeIndex];
			if (option) pick(option);
			return;
		}
		onMenuKeydown(event);
	}

	function onSearchInput() {
		// Hasil filter berubah: sorotan kembali ke item pertama yang terlihat.
		activeIndex = visible.findIndex((o) => !o.disabled);
	}

	$effect(() => {
		if (!open) return;
		// Dengan kolom pencarian, fokus langsung ke sana supaya user bisa
		// mengetik tanpa klik; tanpa pencarian, fokus ke menunya (navigasi panah).
		if (searchable) searchInput?.focus();
		else menu?.focus();
		// Tutup hanya bila halaman/induk yang di-scroll. Scroll DI DALAM menu
		// harus diabaikan — kalau tidak, daftar panjang tak bisa digulir karena
		// menu menutup dirinya sendiri (listener capture ikut menangkap event
		// dari menu sendiri).
		const onScroll = (event: Event) => {
			const target = event.target as Node | null;
			if (target && menu && (target === menu || menu.contains(target))) return;
			closeMenu();
		};
		const onResize = () => closeMenu();
		window.addEventListener('scroll', onScroll, true);
		window.addEventListener('resize', onResize);
		return () => {
			window.removeEventListener('scroll', onScroll, true);
			window.removeEventListener('resize', onResize);
		};
	});
</script>

	<button
		bind:this={trigger}
		type="button"
		class="zs-select-trigger"
		class:zs-select-active={hasValue}
		class:zs-select-plain={variant === 'plain'}
		class:zs-select-fit={fitContent}
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
	<button type="button" class="zs-select-backdrop" aria-label={tr("Tutup pilihan")} onclick={closeMenu}></button>
	<div
		bind:this={menu}
		class="zs-select-menu"
		class:zs-select-menu-up={placement === 'up'}
		role="listbox"
		aria-label={ariaLabel}
		tabindex="-1"
		style="left: {pos.left}px; top: {menuTop}px; min-width: {pos.width}px; width: {minWidth === 0 ? 'max-content' : 'auto'}; max-width: min(92vw, {card ? '1200px' : '360px'}, calc(100vw - {pos.left}px - 8px)); max-height: {menuHeight > 0 ? `${menuHeight}px` : `${heightLimit}px`}; {placement === 'up' && menuHeight > 0 ? `height: ${menuHeight}px;` : ''}"
		onkeydown={onMenuKeydown}
	>
		{#if searchable}
			<div class="zs-select-search">
				<input
					bind:this={searchInput}
					class="zs-select-search-input"
					type="text"
					autocomplete="off"
					spellcheck="false"
					placeholder={searchPlaceholder}
					aria-label={searchPlaceholder}
					bind:value={query}
					oninput={onSearchInput}
					onkeydown={onSearchKeydown}
				/>
			</div>
		{/if}
		{#if visible.length === 0}
			<p class="zs-select-empty">{tr("Tidak ada hasil")}</p>
		{/if}
		{#if popularMode && !showAll && visible.length > 0}
			<p class="zs-select-section">{popularLabel}</p>
		{:else if popularMode && showAll && card && !searching && visible.length > popularCount}
			<p class="zs-select-section">{allOptionsLabel || tr("Semua angle")}</p>
		{/if}
		{#if popularMode && card}
			<div class="zs-select-cards">
				{#each visible as option, index (option.value)}
					<button
						type="button"
						role="option"
						aria-selected={option.value === value}
						class="zs-card"
						class:zs-card-selected={option.value === value}
						class:zs-card-active={index === activeIndex}
						disabled={option.disabled}
						onclick={() => pick(option)}
						onmousemove={() => (activeIndex = index)}
					>
						<span class="zs-card-title">{option.label}</span>
					{#if option.description}
						<span class="zs-card-desc">{option.description}</span>
					{/if}
						{#if option.value === value}
							<span class="zs-card-check"><Check size={11} /></span>
						{/if}
					</button>
				{/each}
			</div>
		{:else}
			{#each visible as option, index (option.value)}
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
		{/if}
		{#if popularCount > 0 && !searching}
			<div class="zs-select-more-wrap">
				{#if !showAll}
					<button type="button" class="zs-select-more" onclick={() => (showAll = true)}>
						{showAllLabel} ({matches.length}) →
					</button>
				{:else if card}
					<button type="button" class="zs-select-more" onclick={() => (showAll = false)}>
						← {showLessLabel}
					</button>
				{/if}
			</div>
		{/if}
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
		.zs-select-trigger.zs-select-fit {
			width: fit-content;
			min-width: 0;
			max-width: 100%;
		}
		.zs-select-trigger.zs-select-fit .zs-select-label {
			max-width: none;
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
	/* Drop-up: dibuka ke atas saat ruang di bawah tidak cukup. Animasi dan
	   titik asal transform dibalik supaya arah geraknya konsisten. */
	/* Menu adalah flex-column yang bisa digulir. Tanpa flex-shrink:0, anak-anak
	   menu ikut mengempis saat isinya melebihi tinggi menu — kartu terlihat
	   terpotong walau seharusnya cukup digulir. */
	.zs-select-menu > * {
		flex: 0 0 auto;
	}
	.zs-select-menu-up {
		transform-origin: bottom center;
		animation-name: zsSelectInUp;
	}
	@keyframes zsSelectInUp {
		from {
			opacity: 0;
			transform: translateY(4px) scale(0.98);
		}
		to {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
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

	.zs-select-search {
		position: sticky;
		top: -5px;
		z-index: 1;
		margin: -5px -5px 4px;
		padding: 5px 5px 6px;
		background: #ffffff;
		border-bottom: 1px solid #f1f5f9;
	}
	.zs-select-search-input {
		width: 100%;
		height: 32px;
		padding: 0 10px;
		border: 1px solid #e2e8f0;
		border-radius: 7px;
		background: #f8fafc;
		color: #0f172a;
		font: inherit;
		font-size: 13px;
		outline: none;
	}
	.zs-select-search-input::placeholder {
		color: #94a3b8;
	}
	.zs-select-search-input:focus {
		border-color: #2563eb;
		background: #ffffff;
		box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.12);
	}
	.zs-select-empty {
		margin: 4px 6px 6px;
		color: #64748b;
		font-size: 12.5px;
	}
	/* Label kecil di atas daftar mode populer. */
	.zs-select-section {
		margin: 2px 6px 3px;
		color: #94a3b8;
		font-size: 10.5px;
		font-weight: 700;
		letter-spacing: 0.04em;
		text-transform: uppercase;
	}
	/* Tombol "Lihat semua" menempel di kaki menu: tetap terlihat walau daftar
	   kartu memanjang dan isi menu digulir. */
	.zs-select-more-wrap {
		position: sticky;
		bottom: -5px;
		z-index: 1;
		margin: 6px -5px -5px;
		padding: 0 5px 5px;
		background: #ffffff;
		border-top: 1px solid #f1f5f9;
	}
	.zs-select-more {
		display: block;
		width: 100%;
		margin: 0;
		padding: 7px 10px;
		border: none;
		border-radius: 7px;
		background: transparent;
		color: #2563eb;
		font: inherit;
		font-size: 12.5px;
		font-weight: 600;
		text-align: left;
		cursor: pointer;
	}
	.zs-select-more:hover {
		background: #eff6ff;
	}
	/* Mode kartu (angle populer): grid kartu judul + deskripsi. Padding memberi
	   ruang untuk centang terpilih yang sedikit keluar dari sudut kartu. */
	.zs-select-cards {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
		gap: 8px;
		padding: 8px 8px 6px;
	}
	.zs-card {
		position: relative;
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 3px;
		padding: 10px 11px;
		border: 1px solid #e2e8f0;
		border-radius: 10px;
		background: #ffffff;
		color: #0f172a;
		font: inherit;
		text-align: left;
		cursor: pointer;
		transition:
			border-color 0.15s ease,
			background-color 0.15s ease;
	}
	.zs-card:hover {
		background: #f8fafc;
	}
	.zs-card.zs-card-active {
		background: #f1f5f9;
	}
	.zs-card.zs-card-selected {
		background: #eff6ff;
		border-color: #93c5fd;
	}
	.zs-card:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}
	.zs-card-title {
		font-size: 12.5px;
		font-weight: 700;
		line-height: 1.3;
		color: inherit;
	}
	.zs-card.zs-card-selected .zs-card-title {
		color: #1d4ed8;
	}
	.zs-card-desc {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		font-size: 11px;
		line-height: 1.45;
		color: #64748b;
	}
	/* Centang menumpuk di sudut kanan atas, sedikit keluar dari kartu. Cincin
	   putih memisahkannya dari border kartu di bawahnya. */
	.zs-card-check {
		position: absolute;
		top: -7px;
		right: -7px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 18px;
		height: 18px;
		border-radius: 999px;
		border: 2px solid #ffffff;
		background: #2563eb;
		color: #ffffff;
		box-shadow: 0 1px 2px rgba(15, 23, 42, 0.18);
	}
</style>
