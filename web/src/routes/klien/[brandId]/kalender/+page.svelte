<script lang="ts">
	// Kalender editorial: penempatan artikel per hari dalam satu bulan. Read-only.
	// Tanggal penempatan = tanggal terbit bila sudah terbit, selain itu jadwal.
	// Panel rincian hanya muncul setelah sebuah tanggal diklik.
	import { tr } from '$lib/i18n/phrases';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ChevronDown, ChevronLeft, ChevronRight, ExternalLink, X } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
	import type { TransitionConfig } from 'svelte/transition';

	type Placement = {
		id: string;
		status: string;
		publish_status: string;
		document_title: string;
		media_name: string;
		scheduled_at: string | null;
		published_at: string | null;
		event_at: string | null;
		published_url: string | null;
		campaign_id: string | null;
	};
	type Calendar = { month: string; items: Placement[] };

	const brandId = $derived(page.params.brandId as string);

	// Di sel: maksimal 2 label, sisanya "+N lainnya". Di layar kecil: titik saja.
	const MAX_LABELS = 2;
	const MAX_DOTS = 4;

	const localKey = (d: Date) =>
		`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;

	const today = new Date();
	let viewYear = $state(today.getFullYear());
	let viewMonth = $state(today.getMonth()); // 0-11
	let loading = $state(true);
	let error = $state('');
	let items = $state<Placement[]>([]);
	// null = panel tertutup. Hanya terisi setelah pengguna mengklik sebuah tanggal.
	let selectedKey = $state<string | null>(null);

	const monthLabel = $derived(new Date(viewYear, viewMonth, 1).toLocaleDateString('id-ID', { month: 'long', year: 'numeric' }));

	// Header hari, Senin dulu. 2024-01-01 adalah hari Senin.
	const WEEKDAYS = Array.from({ length: 7 }, (_, i) =>
		new Date(2024, 0, 1 + i).toLocaleDateString('id-ID', { weekday: 'short' })
	);

	async function load() {
		loading = true;
		error = '';
		try {
			const ym = `${viewYear}-${String(viewMonth + 1).padStart(2, '0')}`;
			const res = await apiRequest(`/v1/brands/${brandId}/calendar?month=${ym}`);
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal memuat kalender.'));
				return;
			}
			const body = (await res.json()) as Calendar;
			items = body.items ?? [];
		} catch {
			error = tr('Gagal memuat kalender.');
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	// Ganti bulan: ubah tampilan lalu muat ulang data bulan itu. Panel ditutup.
	// Batas atas: bulan berjalan. Masa depan belum boleh dilihat.
	const MAX_YEAR = today.getFullYear();
	const MAX_MONTH = today.getMonth();
	const isFutureMonth = (y: number, m: number) => y > MAX_YEAR || (y === MAX_YEAR && m > MAX_MONTH);
	const atMaxMonth = $derived(viewYear === MAX_YEAR && viewMonth >= MAX_MONTH);

	function shiftMonth(delta: number) {
		if (delta > 0 && isFutureMonth(viewYear, viewMonth + 1)) return;
		const d = new Date(viewYear, viewMonth + delta, 1);
		viewYear = d.getFullYear();
		viewMonth = d.getMonth();
		selectedKey = null;
		void load();
	}

	function goToday() {
		const t = new Date();
		viewYear = t.getFullYear();
		viewMonth = t.getMonth();
		selectedKey = null;
		void load();
	}

	// Pemilih cepat bulan dan tahun: klik nama bulan.
	let pickerOpen = $state(false);
	let monthWrapEl = $state<HTMLDivElement>();
	let pickerYear = $state(today.getFullYear());
	const MONTH_SHORT = Array.from({ length: 12 }, (_, i) =>
		new Date(2024, i, 1).toLocaleDateString('id-ID', { month: 'short' })
	);
	function togglePicker() {
		pickerOpen = !pickerOpen;
		pickerYear = viewYear;
	}
	function pickMonth(m: number) {
		if (isFutureMonth(pickerYear, m)) return;
		viewYear = pickerYear;
		viewMonth = m;
		selectedKey = null;
		pickerOpen = false;
		void load();
	}

	function pickDay(key: string) {
		// Klik tanggal yang sama lagi menutup panel.
		selectedKey = selectedKey === key ? null : key;
		panelPage = 1;
	}

	// Penempatan dikelompokkan per tanggal lokal.
	const byDay = $derived.by(() => {
		const map = new Map<string, Placement[]>();
		for (const it of items) {
			if (!it.event_at) continue;
			const k = localKey(new Date(it.event_at));
			const list = map.get(k) ?? [];
			list.push(it);
			map.set(k, list);
		}
		return map;
	});

	// Sel kalender: minggu dimulai Senin; sel kosong sebelum tanggal 1 dan sesudah akhir bulan.
	const cells = $derived.by(() => {
		const first = new Date(viewYear, viewMonth, 1);
		const offset = (first.getDay() + 6) % 7;
		const days = new Date(viewYear, viewMonth + 1, 0).getDate();
		const out: ({ key: string; day: number; weekend: boolean } | null)[] = [];
		for (let i = 0; i < offset; i++) out.push(null);
		for (let d = 1; d <= days; d++) {
			const date = new Date(viewYear, viewMonth, d);
			const dow = (date.getDay() + 6) % 7; // 0 = Senin, 6 = Minggu
			out.push({ key: localKey(date), day: d, weekend: dow >= 5 });
		}
		while (out.length % 7 !== 0) out.push(null);
		return out;
	});

	const selectedItems = $derived(selectedKey ? (byDay.get(selectedKey) ?? []) : []);

	// Paginasi panel: 3 publikasi per halaman.
	const PANEL_PAGE_SIZE = 3;
	let panelPage = $state(1);
	const panelPages = $derived(Math.max(1, Math.ceil(selectedItems.length / PANEL_PAGE_SIZE)));
	const curPanelPage = $derived(Math.min(panelPage, panelPages));
	const pagedItems = $derived(
		selectedItems.slice((curPanelPage - 1) * PANEL_PAGE_SIZE, curPanelPage * PANEL_PAGE_SIZE)
	);
	const todayKey = localKey(today);

	function dotTone(status: string): 'good' | 'bad' | 'muted' {
		if (status === 'published') return 'good';
		if (status === 'failed') return 'bad';
		return 'muted';
	}

	function statusLabel(status: string): string {
		if (status === 'published') return tr('Terbit');
		if (status === 'failed') return tr('Gagal');
		if (status === 'cancelled') return tr('Dibatalkan');
		return tr('Jadwal terbit');
	}

	const fmtDate = (v: string | null) =>
		v ? new Date(v).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' }) : '—';

	// ---- Animasi ----
	// Kurva cubic-bezier(0.22, 1, 0.36, 1) dihitung langsung, supaya sama persis dengan spesifikasi.
	function cubicBezier(x1: number, y1: number, x2: number, y2: number) {
		const cx = 3 * x1, bx = 3 * (x2 - x1) - cx, ax = 1 - cx - bx;
		const cy = 3 * y1, by = 3 * (y2 - y1) - cy, ay = 1 - cy - by;
		const sx = (t: number) => ((ax * t + bx) * t + cx) * t;
		const sy = (t: number) => ((ay * t + by) * t + cy) * t;
		const dx = (t: number) => (3 * ax * t + 2 * bx) * t + cx;
		return (x: number) => {
			let t = x;
			for (let i = 0; i < 8; i++) {
				const e = sx(t) - x;
				const d = dx(t);
				if (Math.abs(e) < 1e-6 || Math.abs(d) < 1e-6) break;
				t -= e / d;
			}
			return sy(t);
		};
	}
	const EASE = cubicBezier(0.22, 1, 0.36, 1);

	const reducedMotion = () => typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches;
	const isMobileSheet = () => typeof matchMedia !== 'undefined' && matchMedia('(max-width: 900px)').matches;

	// Panel masuk: dari kanan (desktop) atau naik dari bawah (mobile, bottom sheet).
	function panelIn(_node: Element, _params?: unknown): TransitionConfig {
		if (reducedMotion()) return { duration: 0 };
		if (isMobileSheet())
			return { duration: 260, easing: EASE, css: (t) => `opacity:${t};transform:translateY(${(1 - t) * 24}px)` };
		return { duration: 260, easing: EASE, css: (t) => `opacity:${t};transform:translateX(${(1 - t) * 20}px)` };
	}

	// Panel keluar: pudar dan sedikit bergeser ke kanan (atau turun, di mobile).
	function panelOut(_node: Element, _params?: unknown): TransitionConfig {
		if (reducedMotion()) return { duration: 0 };
		if (isMobileSheet())
			return { duration: 200, easing: EASE, css: (t) => `opacity:${t};transform:translateY(${(1 - t) * 24}px)` };
		return { duration: 180, easing: EASE, css: (t) => `opacity:${t};transform:translateX(${(1 - t) * 12}px)` };
	}

	// Konten masuk: naik 4px sambil pudar. Dipakai juga untuk stagger daftar.
	function swapIn(_node: Element, params: { delay?: number } = {}): TransitionConfig {
		if (reducedMotion()) return { duration: 0 };
		return {
			delay: params.delay ?? 0,
			duration: 180,
			easing: EASE,
			css: (t) => `opacity:${t};transform:translateY(${(1 - t) * 4}px)`
		};
	}

	// Konten lama memudar cepat saat tanggal lain dipilih (crossfade).
	function fadeOut(_node: Element, _params?: unknown): TransitionConfig {
		if (reducedMotion()) return { duration: 0 };
		return { duration: 120, easing: EASE, css: (t) => `opacity:${t}` };
	}
</script>

<PageTitle title={tr('Kalender editorial')} />
<svelte:window
	onkeydown={(e) => {
		if (e.key === 'Escape') pickerOpen = false;
	}}
	onpointerdown={(e) => {
		// Klik di luar pemilih menutupnya.
		if (pickerOpen && monthWrapEl && !monthWrapEl.contains(e.target as Node)) pickerOpen = false;
	}}
/>

<section class="kal">
	<header class="kal-head">
		<h1 class="page-title">{tr('Kalender editorial')}</h1>
		<p class="page-desc">
			{tr('Sebaran waktu terbit artikel dalam sebulan. Klik tanggal untuk melihat penempatannya.')}
		</p>
	</header>

	<div class="kal-bar">
		<div class="kal-steps">
			<button class="kal-step" onclick={() => shiftMonth(-1)} aria-label={tr('Sebelumnya')}>
				<ChevronLeft size={16} />
			</button>
			<button class="kal-step" onclick={() => shiftMonth(1)} disabled={atMaxMonth} aria-label={tr('Berikutnya')}>
				<ChevronRight size={16} />
			</button>
		</div>
		<div class="kal-month-wrap" bind:this={monthWrapEl}>
			<button
				class="kal-month"
				aria-haspopup="dialog"
				aria-expanded={pickerOpen}
				onclick={togglePicker}
			>
				<span>{monthLabel}</span>
				<ChevronDown size={14} />
			</button>
			{#if pickerOpen}
				<div class="kal-picker" role="dialog" aria-label="Pilih bulan dan tahun">
					<div class="kal-picker-year">
						<button class="kal-pager-btn" onclick={() => pickerYear--} aria-label="Tahun sebelumnya">
							<ChevronLeft size={14} />
						</button>
						<strong class="kal-picker-yeartext">{pickerYear}</strong>
						<button
							class="kal-pager-btn"
							onclick={() => pickerYear++}
							disabled={pickerYear >= MAX_YEAR}
							aria-label="Tahun berikutnya"
						>
							<ChevronRight size={14} />
						</button>
					</div>
					<div class="kal-picker-months">
						{#each MONTH_SHORT as m, i (i)}
							<button
								class="kal-picker-m"
								class:is-current={i === viewMonth && pickerYear === viewYear}
								disabled={isFutureMonth(pickerYear, i)}
								onclick={() => pickMonth(i)}
							>
								{m}
							</button>
						{/each}
					</div>
				</div>
			{/if}
		</div>
		<button class="kal-today" onclick={goToday}>{tr('Hari ini')}</button>
	</div>

	{#if error}
		<div class="kal-error" role="alert">
			<p>{error}</p>
			<button onclick={load}>{tr('Coba lagi')}</button>
		</div>
	{:else if loading}
		<p class="kal-muted">{tr('Memuat kalender…')}</p>
	{:else}
		<div class="kal-body" class:is-open={selectedKey !== null}>
			<div class="kal-grid" role="grid" aria-label={monthLabel}>
				{#each WEEKDAYS as wd (wd)}
					<div class="kal-wd" role="columnheader">{wd}</div>
				{/each}
				{#each cells as cell, idx (idx)}
					{#if cell}
						{@const list = byDay.get(cell.key) ?? []}
						<button
							class="kal-cell"
							class:is-weekend={cell.weekend}
							class:is-today={cell.key === todayKey}
							class:is-selected={cell.key === selectedKey}
							onclick={() => pickDay(cell.key)}
							aria-pressed={cell.key === selectedKey}
							aria-label={`${cell.day} ${monthLabel}, ${list.length} ${tr('Penempatan')}`}
						>
							<span class="kal-day">{cell.day}</span>
							{#if list.length > 0}
								<span class="kal-events">
									{#each list.slice(0, MAX_LABELS) as it (it.id)}
										<span class="kal-ev kal-ev-{dotTone(it.status)}" title={it.document_title}>
											{statusLabel(it.status)}
										</span>
									{/each}
									{#if list.length > MAX_LABELS}
										<span class="kal-more">+{list.length - MAX_LABELS} {tr('lainnya')}</span>
									{/if}
								</span>
								<span class="kal-dots" aria-hidden="true">
									{#each list.slice(0, MAX_DOTS) as it (it.id)}
										<i class="kal-dot kal-dot-{dotTone(it.status)}"></i>
									{/each}
									{#if list.length > MAX_DOTS}
										<span class="kal-more">+{list.length - MAX_DOTS}</span>
									{/if}
								</span>
							{/if}
						</button>
					{:else}
						<div class="kal-cell kal-empty" aria-hidden="true"></div>
					{/if}
				{/each}
			</div>

			<div class="kal-side">
				{#if selectedKey}
					<aside class="kal-panel" in:panelIn out:panelOut>
						{#key selectedKey}
							<div class="kal-panel-inner" in:swapIn out:fadeOut>
								<header class="kal-panel-head">
									<h2 class="kal-panel-title">{fmtDate(selectedKey)}</h2>
									<button class="kal-close" onclick={() => (selectedKey = null)} aria-label={tr('Tutup')}>
										<X size={16} />
									</button>
								</header>
								{#if selectedItems.length === 0}
									<p class="kal-muted">{tr('Tidak ada penempatan di hari ini.')}</p>
								{:else}
									<ul class="kal-list">
										{#each pagedItems as it, i (it.id)}
											<li class="kal-item" in:swapIn={{ delay: 40 + i * 40 }}>
												<span class="kal-dot kal-dot-{dotTone(it.status)}"></span>
												<div class="kal-item-main">
													<strong class="kal-item-title">{it.document_title || '—'}</strong>
													<span class="kal-item-media">{it.media_name || '—'}</span>
													<div class="kal-item-end">
														<span class="kal-status">{statusLabel(it.status)}</span>
														{#if it.published_url}
															<a class="kal-link" href={it.published_url} target="_blank" rel="noopener noreferrer">
																<ExternalLink size={13} />
																<span>{tr('Buka artikel')}</span>
															</a>
														{/if}
													</div>
												</div>
											</li>
										{/each}
									</ul>
									{#if panelPages > 1}
										<nav class="kal-pager" aria-label="Paginasi penempatan">
											<button
												class="kal-pager-btn"
												onclick={() => (panelPage = curPanelPage - 1)}
												disabled={curPanelPage <= 1}
												aria-label={tr('Sebelumnya')}
											>
												<ChevronLeft size={14} />
											</button>
											<span class="kal-pager-info">{curPanelPage} / {panelPages}</span>
											<button
												class="kal-pager-btn"
												onclick={() => (panelPage = curPanelPage + 1)}
												disabled={curPanelPage >= panelPages}
												aria-label={tr('Berikutnya')}
											>
												<ChevronRight size={14} />
											</button>
										</nav>
									{/if}
								{/if}
							</div>
						{/key}
					</aside>
				{/if}
			</div>
		</div>
	{/if}
</section>

<style>
	/* Palet Tauthub */
	.kal {
		--kal-primary: #2c5ead;
		--kal-soft: #eaf3ff;
		--kal-border: #dce4ee;
		--kal-muted: #6b7a90;
		--kal-weekend: #fafbfd;
		--kal-hover: #f4f8fd;
		display: flex;
		flex-direction: column;
		gap: 24px;
	}
	.kal-head {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.kal-bar {
		display: flex;
		align-items: center;
		gap: 12px;
		min-height: 40px;
	}
	.kal-steps {
		display: inline-flex;
		border: 1px solid var(--kal-border);
		border-radius: 8px;
		overflow: hidden;
		background: #fff;
	}
	.kal-step {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		background: transparent;
		border: 0;
		color: var(--kal-muted);
		cursor: pointer;
		transition: background-color 150ms ease, color 150ms ease;
	}
	.kal-step + .kal-step {
		border-left: 1px solid var(--kal-border);
	}
	.kal-step:hover {
		background: var(--kal-hover);
		color: var(--kal-primary);
	}
	.kal-month-wrap {
		position: relative;
		display: inline-flex;
		align-items: center;
	}
	.kal-month-wrap > .kal-month {
		display: inline-flex;
		flex-direction: row;
		align-items: center;
		gap: 6px;
		width: auto;
		min-width: 0;
		padding: 4px 8px;
		margin: 0 0 0 -8px;
		white-space: nowrap;
		font: inherit;
		font-size: 18px;
		font-weight: 600;
		line-height: 1.3;
		font-variant-numeric: tabular-nums;
		color: inherit;
		background: transparent;
		border: 0;
		border-radius: 8px;
		cursor: pointer;
		transition: background-color 150ms ease;
	}
	.kal-month-wrap > .kal-month:hover,
	.kal-month-wrap > .kal-month[aria-expanded='true'] {
		background: var(--kal-hover);
	}
	.kal-picker {
		position: absolute;
		top: calc(100% + 8px);
		left: 0;
		z-index: 20;
		width: 260px;
		padding: 12px;
		background: #fff;
		border: 1px solid var(--kal-border);
		border-radius: 12px;
		box-shadow: 0 8px 24px rgba(15, 23, 42, 0.1);
		animation: kal-pop 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	@keyframes kal-pop {
		from {
			opacity: 0;
			transform: translateY(-4px);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}
	.kal-picker-year {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 10px;
	}
	.kal-picker-yeartext {
		font-size: 15px;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}
	.kal-picker-months {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 6px;
	}
	.kal-picker-m {
		height: 34px;
		font: inherit;
		font-size: 13px;
		color: #1f2a3d;
		background: transparent;
		border: 1px solid transparent;
		border-radius: 8px;
		cursor: pointer;
		transition: background-color 150ms ease, color 150ms ease;
	}
	.kal-picker-m:hover:not(:disabled) {
		background: var(--kal-hover);
	}
	.kal-picker-m:disabled,
	.kal-step:disabled {
		opacity: 0.35;
		cursor: default;
	}
	.kal-picker-m:disabled:hover,
	.kal-step:disabled:hover {
		background: transparent;
	}
	.kal-picker-m.is-current {
		background: var(--kal-soft);
		border-color: color-mix(in srgb, var(--kal-primary) 35%, transparent);
		color: var(--kal-primary);
		font-weight: 600;
	}
	.kal-today {
		margin-left: auto;
		height: 32px;
		padding: 0 14px;
		font: inherit;
		font-size: 14px;
		font-weight: 500;
		color: var(--kal-primary);
		background: #fff;
		border: 1px solid #b9d0ee;
		border-radius: 8px;
		cursor: pointer;
		transition: background-color 150ms ease;
	}
	.kal-today:hover {
		background: var(--kal-soft);
	}
	.kal-muted {
		color: var(--kal-muted);
		margin: 8px 0;
	}
	.kal-error {
		padding: 16px;
		border-radius: 12px;
		border: 1px solid color-mix(in srgb, #c0392b 40%, transparent);
	}
	.kal-error button {
		font: inherit;
		margin-top: 8px;
		background: transparent;
		color: inherit;
		border: 1px solid currentColor;
		border-radius: 8px;
		padding: 4px 10px;
		cursor: pointer;
	}

	/* Kolom panel selalu ada; lebarnya 0 saat tertutup dan 300px saat terbuka,
	   sehingga kalender tidak melompat, hanya menyempit dengan halus. */
	.kal-body {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 0px;
		gap: 0;
		align-items: start;
		transition: grid-template-columns 240ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.kal-body.is-open {
		grid-template-columns: minmax(0, 1fr) 300px;
		gap: 24px;
	}
	.kal-side {
		min-width: 0;
		overflow: hidden;
	}
	.kal-grid {
		display: grid;
		grid-template-columns: repeat(7, minmax(0, 1fr));
		gap: 4px;
		min-width: 0;
	}
	.kal-wd {
		font-size: 13px;
		font-weight: 500;
		color: var(--kal-muted);
		text-align: center;
		height: 36px;
		display: flex;
		align-items: center;
		justify-content: center;
		background: #f7f9fc;
		border-bottom: 1px solid var(--kal-border);
		border-radius: 6px 6px 0 0;
		margin-bottom: 4px;
	}
	.kal-cell {
		min-height: 96px;
		display: flex;
		flex-direction: column;
		align-items: stretch;
		gap: 6px;
		padding: 8px;
		background: #fff;
		border: 1px solid var(--kal-border);
		border-radius: 8px;
		color: inherit;
		font: inherit;
		text-align: left;
		cursor: pointer;
		min-width: 0;
		overflow: hidden;
		transition: background-color 180ms ease, border-color 180ms ease, transform 180ms ease;
	}
	.kal-cell:hover {
		background: var(--kal-hover);
	}
	.kal-cell:active {
		transform: scale(0.98);
	}
	.kal-cell.is-weekend {
		background: var(--kal-weekend);
	}
	.kal-cell.kal-empty {
		background: transparent;
		border-color: transparent;
		cursor: default;
	}
	.kal-day {
		align-self: flex-start;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 24px;
		height: 24px;
		padding: 0 6px;
		font-size: 13px;
		font-variant-numeric: tabular-nums;
		border-radius: 6px;
		color: #1f2a3d;
		transition: color 180ms ease;
	}
	/* Hari ini: biru muda + angka biru Tauthub. */
	.kal-cell.is-today {
		background: var(--kal-soft);
		border-color: color-mix(in srgb, var(--kal-primary) 35%, transparent);
	}
	.kal-cell.is-today .kal-day {
		color: var(--kal-primary);
		font-weight: 600;
	}
	/* Terpilih: biru muda, border primary, angka biru semibold, micro-scale 0.97 → 1. */
	.kal-cell.is-selected {
		background: var(--kal-soft);
		border-color: var(--kal-primary);
		animation: kal-select 200ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.kal-cell.is-selected .kal-day {
		color: var(--kal-primary);
		font-weight: 600;
	}
	@keyframes kal-select {
		from {
			transform: scale(0.97);
		}
		to {
			transform: scale(1);
		}
	}

	.kal-events {
		display: flex;
		flex-direction: column;
		gap: 3px;
		min-width: 0;
	}
	.kal-ev {
		display: block;
		align-self: flex-start;
		max-width: 100%;
		font-size: 11px;
		line-height: 16px;
		padding: 0 6px;
		border-radius: 4px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		background: #f1f5f9;
		color: #4a5a72;
	}
	.kal-ev-good {
		background: var(--kal-soft);
		color: var(--kal-primary);
	}
	.kal-ev-bad {
		background: #fdecec;
		color: #b42318;
	}
	.kal-more {
		font-size: 11px;
		color: var(--kal-muted);
		padding-left: 2px;
	}
	/* Titik hanya tampil di layar kecil. */
	.kal-dots {
		display: none;
		flex-wrap: wrap;
		gap: 4px;
		align-items: center;
	}
	.kal-dot {
		width: 6px;
		height: 6px;
		border-radius: 999px;
		background: #9aa8bb;
		display: inline-block;
		flex: none;
	}
	.kal-dot-good {
		background: var(--kal-primary);
	}
	.kal-dot-bad {
		background: #b42318;
	}

	/* Panel detail (desktop): lebar tetap 300px di dalam kolom yang melebar. */
	.kal-panel {
		width: 300px;
		padding: 16px;
		border: 1px solid var(--kal-border);
		border-radius: 12px;
		background: #fff;
		will-change: transform, opacity;
	}
	.kal-panel-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		margin-bottom: 12px;
	}
	.kal-panel-title {
		font-size: 15px;
		font-weight: 600;
		margin: 0;
	}
	.kal-close {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		background: transparent;
		border: 0;
		border-radius: 8px;
		color: var(--kal-muted);
		cursor: pointer;
		transition: background-color 150ms ease, color 150ms ease;
	}
	.kal-close:hover {
		color: var(--kal-primary);
		background: var(--kal-hover);
	}
	.kal-list {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
	}
	.kal-item {
		display: flex;
		gap: 10px;
		padding: 12px 0;
		border-top: 1px solid var(--kal-border);
	}
	.kal-item:first-child {
		border-top: 0;
		padding-top: 0;
	}
	.kal-item .kal-dot {
		margin-top: 7px;
	}
	.kal-item-main {
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
	}
	.kal-item-title {
		font-weight: 500;
		font-size: 14px;
		overflow-wrap: anywhere;
	}
	.kal-item-media {
		font-size: 13px;
		color: var(--kal-muted);
	}
	.kal-item-end {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		margin-top: 4px;
		flex-wrap: wrap;
	}
	.kal-status {
		font-size: 12px;
		color: var(--kal-muted);
	}
	.kal-link {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		font-size: 13px;
		color: var(--kal-primary);
		text-underline-offset: 3px;
	}
	.kal-link:hover {
		text-decoration: underline;
	}
	.kal-pager {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 10px;
		padding-top: 12px;
		border-top: 1px solid var(--kal-border);
	}
	.kal-pager-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		background: #fff;
		border: 1px solid var(--kal-border);
		border-radius: 8px;
		color: var(--kal-muted);
		cursor: pointer;
		transition: background-color 150ms ease, color 150ms ease;
	}
	.kal-pager-btn:hover:not(:disabled) {
		color: var(--kal-primary);
		background: var(--kal-hover);
	}
	.kal-pager-btn:disabled {
		opacity: 0.4;
		cursor: default;
	}
	.kal-pager-info {
		font-size: 13px;
		font-variant-numeric: tabular-nums;
		color: var(--kal-muted);
		min-width: 40px;
		text-align: center;
	}

	/* Layar ≤900px: panel menjadi bottom sheet dari bawah, dengan scroll internal. */
	@media (max-width: 900px) {
		.kal-body,
		.kal-body.is-open {
			grid-template-columns: minmax(0, 1fr);
			gap: 0;
		}
		.kal-side {
			position: fixed;
			left: 0;
			right: 0;
			bottom: 0;
			z-index: 50;
			max-height: 70vh;
			overflow-x: hidden;
			overflow-y: auto;
			padding-bottom: env(safe-area-inset-bottom);
		}
		.kal-panel {
			width: auto;
			border-radius: 16px 16px 0 0;
			border-bottom: 0;
			box-shadow: 0 -8px 24px rgba(15, 23, 42, 0.12);
			padding: 16px 16px calc(16px + env(safe-area-inset-bottom));
		}
	}
	/* Layar kecil: sel lebih pendek, label diganti titik. */
	@media (max-width: 640px) {
		.kal-cell {
			min-height: 64px;
			padding: 6px;
		}
		.kal-events {
			display: none;
		}
		.kal-dots {
			display: flex;
		}
		.kal-month {
			min-width: 0;
			font-size: 16px;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.kal-body {
			transition: none;
		}
		.kal-cell,
		.kal-cell:active {
			transition: none;
			transform: none;
		}
		.kal-cell.is-selected {
			animation: none;
		}
	}
</style>
