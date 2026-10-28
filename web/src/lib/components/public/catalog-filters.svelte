<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Search, SlidersHorizontal } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import type { PublicCategory } from '$lib/public/types';

	/**
	 * Toolbar katalog: search box + tombol Filter yang membuka popover.
	 *
	 * Satu sumber state: `desired`. Setiap kontrol menulis ke situ, lalu URL
	 * dibangun dari `desired` — bukan dari `values` yang masih basi. Ini penting
	 * saat pengguna mengubah beberapa filter beruntun: tanpa itu perubahan
	 * pertama hilang karena request berikutnya dibangun dari URL lama.
	 *
	 * `values` (query string) tetap sumber kebenaran untuk data yang dimuat
	 * +page.server.ts. Saat URL berubah dari luar (tombol Back, chip kategori),
	 * `desired` disinkronkan lagi — kecuali ada perubahan kita yang masih
	 * berjalan.
	 *
	 * Tanpa JS, form GET tetap mengirim q + filter aktif sebagai hidden input.
	 */
	let {
		action,
		values,
		categories,
		lockedCategory = null
	}: {
		action: string;
		values: { q: string; kategori: string; tier: string; da: string; harga: string; urut: string };
		categories: PublicCategory[];
		lockedCategory?: string | null;
	} = $props();

	const SORTS = [
		{ value: 'nama', label: 'Nama A–Z' },
		{ value: 'da', label: 'DA tertinggi' },
		{ value: 'trafik', label: 'Trafik tertinggi' },
		{ value: 'harga_asc', label: 'Harga terendah' },
		{ value: 'harga_desc', label: 'Harga tertinggi' },
		{ value: 'baru', label: 'Terbaru' }
	];
	const DEFAULT_SORT = 'nama';

	const tierOptions = [
		{ value: '', label: 'Semua tier' },
		{ value: 'tier1', label: 'Tier 1' },
		{ value: 'tier2', label: 'Tier 2' },
		{ value: 'tier3', label: 'Tier 3' }
	];
	const sortOptions = SORTS.map((s) => ({ value: s.value, label: s.label }));

	type Filters = { q: string; kategori: string; tier: string; da: string; harga: string; urut: string };

	const fromValues = (v: typeof values): Filters => ({
		q: v.q,
		kategori: v.kategori,
		tier: v.tier,
		da: v.da,
		harga: v.harga,
		urut: v.urut || DEFAULT_SORT
	});

	// Nilai awal sengaja diambil sekali (untrack); sinkronisasi berikutnya
	// diurus $effect di bawah saat URL berubah dari luar.
	let desired = $state<Filters>(untrack(() => fromValues(values)));

	// URL terakhir yang kita minta. Dipakai untuk (a) mencegah request ganda
	// untuk nilai yang sama, dan (b) membedakan navigasi kita sendiri dari
	// navigasi luar (Back/Forward, chip kategori).
	let lastRequested = $state('');
	let pending = $state(0);

	let open = $state(false);
	let wrapEl = $state<HTMLDivElement | null>(null);
	let buttonEl = $state<HTMLButtonElement | null>(null);

	$effect(() => {
		const current = `${page.url.pathname}${page.url.search}`;
		// Perubahan kita masih berjalan: jangan timpa input pengguna dengan
		// nilai URL yang belum tentu terbaru.
		if (pending > 0) return;
		if (current === lastRequested) return;
		desired = fromValues(values);
		lastRequested = current;
	});

	const activeCount = $derived(
		(lockedCategory ? 0 : desired.kategori ? 1 : 0) +
			(desired.tier ? 1 : 0) +
			(desired.da.trim() ? 1 : 0) +
			(desired.harga.trim() ? 1 : 0) +
			(desired.urut !== DEFAULT_SORT ? 1 : 0)
	);

	function buildUrl(f: Filters): string {
		const params = new URLSearchParams();
		if (f.q.trim()) params.set('q', f.q.trim());
		if (!lockedCategory && f.kategori) params.set('kategori', f.kategori);
		if (f.tier) params.set('tier', f.tier);
		if (f.da.trim()) params.set('da', f.da.trim());
		if (f.harga.trim()) params.set('harga', f.harga.trim());
		if (f.urut && f.urut !== DEFAULT_SORT) params.set('urut', f.urut);
		const s = params.toString();
		return s ? `${action}?${s}` : action;
	}

	/** Navigasi hanya bila URL berubah; tidak ada request untuk nilai yang sama. */
	async function apply() {
		const target = buildUrl(desired);
		if (target === lastRequested) return;
		const current = `${page.url.pathname}${page.url.search}`;
		if (target === current) {
			lastRequested = target;
			return;
		}
		lastRequested = target;
		pending++;
		try {
			await goto(target, { replaceState: true, noScroll: true, keepFocus: true });
		} finally {
			pending--;
		}
	}

	function set<K extends keyof Filters>(key: K, value: Filters[K]) {
		desired = { ...desired, [key]: value };
	}

	// Input numerik: debounce 350ms supaya tidak memicu request per ketikan.
	let debounceTimer: ReturnType<typeof setTimeout> | null = null;
	function scheduleNumeric() {
		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			debounceTimer = null;
			void apply();
		}, 350);
	}
	function flushNumeric() {
		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = null;
		void apply();
	}

	function resetAll() {
		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = null;
		// Reset hanya menyentuh filter popover; kata kunci pencarian tetap.
		desired = { ...desired, kategori: '', tier: '', da: '', harga: '', urut: DEFAULT_SORT };
		void apply();
	}

	onMount(() => {
		const onPointerDown = (event: PointerEvent) => {
			if (!open) return;
			const target = event.target as Node | null;
			if (!target) return;
			if (wrapEl?.contains(target)) return;
			// Dropdown Select merender menu + backdrop lewat portal di body,
			// jadi secara DOM berada "di luar" popover. Klik di sana berarti
			// pengguna sedang memilih opsi, bukan menutup panel.
			const el = target instanceof Element ? target : target.parentElement;
			if (el?.closest('.zs-select-menu, .zs-select-backdrop')) return;
			open = false;
		};
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape' && open) {
				open = false;
				buttonEl?.focus();
			}
		};
		document.addEventListener('pointerdown', onPointerDown, true);
		document.addEventListener('keydown', onKey);
		return () => {
			document.removeEventListener('pointerdown', onPointerDown, true);
			document.removeEventListener('keydown', onKey);
			if (debounceTimer) clearTimeout(debounceTimer);
		};
	});
</script>

<div class="pub-toolbar">
	<form class="pub-toolbar-search" method="GET" action={action} role="search">
		<Search size={17} class="pub-toolbar-icon" aria-hidden="true" />
		<input
			type="search"
			name="q"
			value={desired.q}
			placeholder="Cari nama media atau domain, mis. kompas.com"
			aria-label="Cari media"
			oninput={(e) => set('q', e.currentTarget.value)}
			onkeydown={(e) => {
				if (e.key === 'Enter' && !e.isComposing) {
					e.preventDefault();
					void apply();
				}
			}}
		/>
		<!-- Filter aktif ikut terkirim saat form di-submit tanpa JS. -->
		{#if !lockedCategory && desired.kategori}<input type="hidden" name="kategori" value={desired.kategori} />{/if}
		{#if desired.tier}<input type="hidden" name="tier" value={desired.tier} />{/if}
		{#if desired.da}<input type="hidden" name="da" value={desired.da} />{/if}
		{#if desired.harga}<input type="hidden" name="harga" value={desired.harga} />{/if}
		{#if desired.urut !== DEFAULT_SORT}<input type="hidden" name="urut" value={desired.urut} />{/if}
	</form>

	<div class="pub-filter-wrap" bind:this={wrapEl}>
		<button
			type="button"
			class="pub-filter-btn"
			class:is-active={activeCount > 0}
			bind:this={buttonEl}
			aria-expanded={open}
			aria-controls="pub-filter-popover"
			onclick={() => (open = !open)}
		>
			<SlidersHorizontal size={16} aria-hidden="true" />
			<span>Filter</span>
			{#if activeCount > 0}
				<span class="pub-filter-badge">{activeCount}</span>
			{/if}
		</button>

		{#if open}
			<div class="pub-popover" id="pub-filter-popover" role="dialog" aria-label="Filter pencarian">
				<div class="pub-popover-head">
					<strong>Filter pencarian</strong>
					<button
						type="button"
						class="pub-reset"
						class:is-idle={activeCount === 0}
						disabled={activeCount === 0}
						onclick={resetAll}>Reset semua</button
					>
				</div>

				<div class="pub-popover-grid">
					{#if !lockedCategory}
						<div class="pub-popover-field">
							<span id="pf-cat-label">Kategori</span>
							<Select
								ariaLabel="Kategori"
								value={desired.kategori}
								options={[{ value: '', label: 'Semua kategori' }, ...categories.map((c) => ({ value: c.slug, label: c.name, count: c.count }))]}
								minWidth={0}
								searchable
								searchPlaceholder="Cari kategori"
								onchange={(v) => {
									set('kategori', v);
									void apply();
								}}
							/>
						</div>
					{/if}

					<div class="pub-popover-field">
						<span id="pf-tier-label">Tier</span>
						<Select
							ariaLabel="Tier"
							value={desired.tier}
							options={tierOptions}
							minWidth={0}
							onchange={(v) => {
								set('tier', v);
								void apply();
							}}
						/>
					</div>

					<label class="pub-popover-field">
						<span>DA minimal</span>
						<input
							type="number"
							min="0"
							max="100"
							inputmode="numeric"
							placeholder="Min. DA"
							value={desired.da}
							oninput={(e) => {
								set('da', e.currentTarget.value);
								scheduleNumeric();
							}}
							onkeydown={(e) => {
								if (e.key === 'Enter') {
									e.preventDefault();
									flushNumeric();
								}
							}}
						/>
					</label>

					<label class="pub-popover-field">
						<span>Harga maks. (Rp)</span>
						<input
							type="number"
							min="0"
							step="1000"
							inputmode="numeric"
							placeholder="Harga maksimal"
							value={desired.harga}
							oninput={(e) => {
								set('harga', e.currentTarget.value);
								scheduleNumeric();
							}}
							onkeydown={(e) => {
								if (e.key === 'Enter') {
									e.preventDefault();
									flushNumeric();
								}
							}}
						/>
					</label>

					<div class="pub-popover-field pub-popover-wide pub-popover-sort">
						<span id="pf-sort-label">Urutkan</span>
						<Select
							ariaLabel="Urutkan"
							value={desired.urut}
							options={sortOptions}
							minWidth={0}
							onchange={(v) => {
								set('urut', v);
								void apply();
							}}
						/>
					</div>
				</div>
			</div>
		{/if}
	</div>
</div>
