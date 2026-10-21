<script lang="ts">
	// Pemilih tanggal + jam untuk jadwal terbit. Modal (bukan popover) supaya
	// tidak terpotong overflow tabel/panel dan tetap nyaman di layar kecil.
	// Waktu SELALU dibentuk dalam zona waktu perangkat (new Date(y, m, d, h, m))
	// — jangan parse string lokal dengan `new Date("YYYY-MM-DDTHH:mm")` karena
	// hasilnya bisa bergeser satu hari di zona waktu tertentu.
	import { tr } from '$lib/i18n/phrases';
	import { untrack } from 'svelte';
	import { CalendarDays, ChevronLeft, ChevronRight, X } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';

	let {
		open = false,
		title = '',
		value = null,
		min = new Date(),
		onClose,
		onApply
	}: {
		open?: boolean;
		title?: string;
		value?: Date | null;
		min?: Date;
		onClose: () => void;
		onApply: (date: Date) => void;
	} = $props();

	const MONTHS = ['Januari', 'Februari', 'Maret', 'April', 'Mei', 'Juni', 'Juli', 'Agustus', 'September', 'Oktober', 'November', 'Desember'];
	const WEEKDAYS = ['Sen', 'Sel', 'Rab', 'Kam', 'Jum', 'Sab', 'Min'];
	const MINUTES = Array.from({ length: 12 }, (_, i) => i * 5);

	// Slot berikutnya: jam bulat terdekat setelah `min` (minimal +1 jam).
	function nextSlot(from: Date): Date {
		const d = new Date(from.getTime() + 60 * 60 * 1000);
		d.setMinutes(Math.ceil(d.getMinutes() / 5) * 5, 0, 0);
		return d;
	}

	function startOfDay(d: Date): number {
		return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
	}

	let viewYear = $state(0);
	let viewMonth = $state(0);
	let selectedDay = $state<number | null>(null);
	let hour = $state(9);
	let minute = $state(0);
	let error = $state('');

	$effect(() => {
		if (!open) return;
		// Hanya bergantung pada `open`: nilai awal dibaca sekali saat modal
		// dibuka. Tanpa untrack, Date baru dari parent tiap render akan
		// mereset pilihan user di tengah pemilihan.
		untrack(() => {
			const base = value ?? nextSlot(min);
			viewYear = base.getFullYear();
			viewMonth = base.getMonth();
			selectedDay = base.getDate();
			hour = base.getHours();
			minute = base.getMinutes() - (base.getMinutes() % 5);
			error = '';
		});
	});

	const monthStart = $derived(new Date(viewYear, viewMonth, 1));
	const daysInMonth = $derived(new Date(viewYear, viewMonth + 1, 0).getDate());
	// Minggu dimulai Senin: getDay() 0=Minggu → offset 6, 1=Senin → 0.
	const leadingBlanks = $derived((monthStart.getDay() + 6) % 7);
	const cells = $derived.by(() => {
		const out: Array<number | null> = Array.from({ length: leadingBlanks }, () => null);
		for (let d = 1; d <= daysInMonth; d += 1) out.push(d);
		while (out.length % 7 !== 0) out.push(null);
		return out;
	});
	const minDay = $derived(startOfDay(min));
	const minuteOptions = $derived.by(() => {
		const list = MINUTES.includes(minute) ? MINUTES : [...MINUTES, minute].sort((a, b) => a - b);
		return list.map((v) => ({ value: String(v), label: String(v).padStart(2, '0') }));
	});
	const hourOptions = Array.from({ length: 24 }, (_, i) => ({ value: String(i), label: String(i).padStart(2, '0') }));

	function dayDisabled(day: number): boolean {
		return new Date(viewYear, viewMonth, day).getTime() < minDay;
	}

	function shiftMonth(delta: number) {
		const d = new Date(viewYear, viewMonth + delta, 1);
		viewYear = d.getFullYear();
		viewMonth = d.getMonth();
	}

	/** Isi tanggal & jam dengan waktu perangkat saat ini. */
	function setNow() {
		const n = new Date();
		viewYear = n.getFullYear();
		viewMonth = n.getMonth();
		selectedDay = n.getDate();
		hour = n.getHours();
		minute = n.getMinutes();
		error = '';
	}

	function apply() {
		if (selectedDay === null) {
			error = tr('Pilih tanggal terbit dulu.');
			return;
		}
		const chosen = new Date(viewYear, viewMonth, selectedDay, hour, minute, 0, 0);
		if (chosen.getTime() < min.getTime()) {
			error = tr('Waktu terbit harus di masa depan.');
			return;
		}
		onApply(chosen);
		onClose();
	}
</script>

{#if open}
	<div
		class="fixed inset-0 z-[80] flex items-center justify-center p-4"
		style="background-color: rgba(15, 23, 42, 0.5);"
		role="presentation"
		onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}
		onkeydown={(e) => { if (e.key === 'Escape') onClose(); }}
	>
		<div class="w-full max-w-sm rounded-xl border bg-white p-4 shadow-2xl" style="border-color: var(--border);" role="dialog" aria-modal="true" aria-label={title || tr('Jadwal terbit')}>
			<div class="flex items-center justify-between gap-2">
				<p class="flex items-center gap-2 text-sm font-bold text-slate-900">
					<CalendarDays size={16} style="color: var(--primary);" />
					{title || tr('Jadwal terbit')}
				</p>
				<button class="rounded-lg p-1 text-slate-400 transition-colors hover:bg-slate-100 hover:text-slate-600" type="button" aria-label={tr('Tutup')} onclick={onClose}>
					<X size={16} />
				</button>
			</div>

			<div class="mt-3 flex items-center justify-between">
				<button class="rounded-lg p-1.5 text-slate-500 transition-colors hover:bg-slate-100" type="button" aria-label={tr('Bulan sebelumnya')} onclick={() => shiftMonth(-1)}>
					<ChevronLeft size={16} />
				</button>
				<span class="text-sm font-semibold text-slate-800">{MONTHS[viewMonth]} {viewYear}</span>
				<button class="rounded-lg p-1.5 text-slate-500 transition-colors hover:bg-slate-100" type="button" aria-label={tr('Bulan berikutnya')} onclick={() => shiftMonth(1)}>
					<ChevronRight size={16} />
				</button>
			</div>

			<div class="mt-2 grid grid-cols-7 gap-1 text-center">
				{#each WEEKDAYS as w (w)}
					<span class="py-1 text-[10.5px] font-bold uppercase tracking-wide text-slate-400">{w}</span>
				{/each}
				{#each cells as day, i (i)}
					{#if day === null}
						<span></span>
					{:else}
						<button
							type="button"
							class="aspect-square rounded-lg text-[13px] font-medium transition-colors {selectedDay === day
								? 'text-white'
								: dayDisabled(day)
									? 'cursor-not-allowed text-slate-300'
									: 'text-slate-700 hover:bg-slate-100'}"
							style={selectedDay === day ? 'background-color: var(--primary);' : ''}
							disabled={dayDisabled(day)}
							aria-pressed={selectedDay === day}
							onclick={() => { selectedDay = day; error = ''; }}
						>
							{day}
						</button>
					{/if}
				{/each}
			</div>

			<div class="mt-3 flex items-center gap-2">
				<div class="flex flex-1 items-center gap-1.5 text-xs text-slate-600">
					<span class="shrink-0">{tr('Jam')}</span>
					<div class="timeSelect">
						<Select value={String(hour)} options={hourOptions} ariaLabel={tr('Jam')} minWidth={84} onchange={(v) => (hour = Number(v))} />
					</div>
				</div>
				<div class="flex flex-1 items-center gap-1.5 text-xs text-slate-600">
					<span class="shrink-0">{tr('Menit')}</span>
					<div class="timeSelect">
						<Select value={String(minute)} options={minuteOptions} ariaLabel={tr('Menit')} minWidth={84} onchange={(v) => (minute = Number(v))} />
					</div>
				</div>
			</div>

			{#if error}
				<p class="mt-2 text-xs" style="color: var(--destructive);">{error}</p>
			{/if}

			<div class="mt-4 flex items-center justify-between gap-2">
				<button class="app-btn app-btn-soft !min-h-9 !px-3 !text-xs" type="button" onclick={setNow}>{tr('Sekarang')}</button>
				<div class="flex gap-2">
					<button class="app-btn app-btn-soft !min-h-9 !px-3.5 !text-xs" type="button" onclick={onClose}>{tr('Batal')}</button>
					<button class="app-btn app-btn-primary !min-h-9 !px-3.5 !text-xs" type="button" onclick={apply}>{tr('Terapkan')}</button>
				</div>
			</div>
		</div>
	</div>
{/if}

<style>
	/* Dropdown Jam/Menit memakai komponen Select aplikasi; lebarkan triggernya
	   agar dua kolom terbagi rata di dalam modal. */
	.timeSelect {
		flex: 1;
		min-width: 0;
	}
	.timeSelect :global(.zs-select-trigger) {
		width: 100%;
		max-width: none;
		height: 34px;
		justify-content: space-between;
		font-size: 12px;
	}
	.timeSelect :global(.zs-select-label) {
		max-width: none;
	}
</style>
