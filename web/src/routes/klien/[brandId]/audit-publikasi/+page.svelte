<script lang="ts">
	// Audit Publikasi: rekap kepatuhan artikel yang sudah terbit. Data dari Link
	// Guardian (artikel bisa dibuka, tautan di dalamnya masih ada, tanpa noindex,
	// atribut rel sesuai) plus status garansi dan klaim.
	import { tr } from '$lib/i18n/phrases';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { CalendarDays, ChevronDown, ChevronLeft, ChevronRight, Download, RefreshCw } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	type Item = {
		id: string;
		document_title: string;
		media_name: string;
		published_at: string | null;
		published_url: string | null;
		link_state: string | null;
		problem_kind: string | null;
		last_checked_at: string | null;
		warranty_until: string | null;
		in_warranty: boolean;
		claim_status: string | null;
	};
	type Audit = {
		totals: {
			published: number;
			healthy: number;
			problem: number;
			gone: number;
			unreachable: number;
			unchecked: number;
			claims_open: number;
		};
		monthly: { month: string; total: number; healthy: number; problem: number }[];
		media: { name: string; total: number; problem: number }[];
		items: Item[];
	};

	const PROBLEM_LABEL: Record<string, string> = {
		gone: 'Artikel hilang',
		link_removed: 'Tautan dihapus',
		noindex: 'Ada noindex',
		rel_changed: 'Atribut rel berubah'
	};

	const brandId = $derived(page.params.brandId as string);

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state('');
	let data = $state<Audit | null>(null);
	// Periode: bulan terbit (YYYY-MM). Kosong = semua periode.
	let month = $state('');
	let brandName = $state('');

	// Nama file: Audit_Publikasi_<nama klien>[_<periode>]. Karakter yang tidak aman dibuang.
	function fileBase(): string {
		const safe = brandName.replace(/[\\/:*?"<>|]/g, '').trim().replace(/\s+/g, '_');
		return `Audit_Publikasi${safe ? '_' + safe : ''}${month ? '_' + month : ''}`;
	}

	async function load(silent = false) {
		if (silent) refreshing = true;
		else loading = true;
		error = '';
		try {
			const q = month ? `?month=${encodeURIComponent(month)}` : '';
			const res = await apiRequest(`/v1/brands/${brandId}/audit${q}`);
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal memuat audit publikasi.'));
				return;
			}
			data = (await res.json()) as Audit;
		} catch {
			error = tr('Gagal memuat audit publikasi.');
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			void apiRequest(`/v1/brands/${brandId}`)
				.then((r) => (r.ok ? r.json() : null))
				.then((b) => (brandName = b?.name ?? ''))
				.catch(() => {});
			await load();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	function setMonth(v: string) {
		month = v;
		void load(true);
	}

	// Pemilih periode: pola yang sama dengan date picker proyek (pemicu + popover).
	let pickerOpen = $state(false);
	let pickerYear = $state(new Date().getFullYear());
	let periodEl = $state<HTMLDivElement>();
	const NOW = new Date();
	const MAX_Y = NOW.getFullYear();
	const MAX_M = NOW.getMonth();
	const isFuture = (y: number, m: number) => y > MAX_Y || (y === MAX_Y && m > MAX_M);
	const MONTH_SHORT = Array.from({ length: 12 }, (_, i) =>
		new Date(2024, i, 1).toLocaleDateString('id-ID', { month: 'short' })
	);

	function togglePicker() {
		pickerOpen = !pickerOpen;
		pickerYear = month ? Number(month.slice(0, 4)) : MAX_Y;
	}
	function pickPeriod(v: string) {
		pickerOpen = false;
		setMonth(v);
	}
	function pickMonthIdx(m: number) {
		if (isFuture(pickerYear, m)) return;
		pickPeriod(`${pickerYear}-${String(m + 1).padStart(2, '0')}`);
	}

	const pct = (n: number, d: number) => (d > 0 ? Math.round((n / d) * 100) : 0);

	const periodLabel = $derived(
		month
			? new Date(Number(month.slice(0, 4)), Number(month.slice(5, 7)) - 1, 1).toLocaleDateString('id-ID', {
					month: 'long',
					year: 'numeric'
				})
			: ''
	);

	// Pembagian kondisi: saling lepas, jumlahnya sama dengan total artikel.
	const conditions = $derived.by(() => {
		const t = data?.totals;
		if (!t) return [];
		const other = Math.max(0, t.published - t.healthy - t.problem - t.unreachable - t.unchecked);
		return [
			{ key: 'healthy', label: tr('Sehat'), value: t.healthy, tone: 'good' },
			{ key: 'problem', label: tr('Bermasalah'), value: t.problem, tone: 'bad' },
			{ key: 'unreachable', label: tr('Tak terjangkau'), value: t.unreachable, tone: 'muted' },
			{ key: 'unchecked', label: tr('Belum dicek'), value: t.unchecked + other, tone: 'faint' }
		];
	});

	// Cincin kepatuhan: tiap kondisi satu busur. Persen di tengah = tautan sehat.
	const RING_R = 60;
	const RING_C = 2 * Math.PI * RING_R;
	const ring = $derived.by(() => {
		const total = data?.totals.published ?? 0;
		let acc = 0;
		return conditions
			.filter((c) => c.value > 0)
			.map((c) => {
				const len = (c.value / Math.max(1, total)) * RING_C;
				const seg = { key: c.key, tone: c.tone, dash: `${Math.max(0, len - 2)} ${RING_C - Math.max(0, len - 2)}`, offset: -acc };
				acc += len;
				return seg;
			});
	});
	const healthPct = $derived(pct(data?.totals.healthy ?? 0, data?.totals.published ?? 0));

	// Grafik tren 12 bulan (SVG): kolom bertumpuk, sudut atas membulat lewat clipPath.
	const CH = { w: 640, h: 264, padL: 32, padB: 28, padT: 24 };
	// 12 bulan tahun berjalan: kiri = Januari, kanan = Desember. Bulan tanpa
	// artikel tetap tampil sebagai label kosong.
	const trend = $derived.by(() => {
		const byKey = new Map((data?.monthly ?? []).map((m) => [m.month, m]));
		const year = new Date().getFullYear();
		return Array.from({ length: 12 }, (_, k) => {
			const key = `${year}-${String(k + 1).padStart(2, '0')}`;
			const m = byKey.get(key);
			return { month: key, total: m?.total ?? 0, healthy: m?.healthy ?? 0, problem: m?.problem ?? 0 };
		});
	});
	const monthlyMax = $derived(Math.max(4, ...trend.map((m) => m.total)));
	const bars = $derived.by(() => {
		const list = trend;
		const innerW = CH.w - CH.padL - 8;
		const innerH = CH.h - CH.padB - CH.padT;
		const step = innerW / Math.max(12, list.length);
		const barW = Math.min(28, step * 0.5);
		const base = CH.h - CH.padB;
		const sc = (v: number) => (v / monthlyMax) * innerH;
		return list.map((m, i) => {
			const x = CH.padL + i * step + (step - barW) / 2;
			const hh = sc(m.healthy);
			const ph = sc(m.problem);
			const oh = sc(Math.max(0, m.total - m.healthy - m.problem));
			const top = base - hh - ph - oh;
			return {
				i,
				month: m.month,
				label: new Date(Number(m.month.slice(0, 4)), Number(m.month.slice(5, 7)) - 1, 1).toLocaleDateString('id-ID', {
					month: 'short'
				}),
				x,
				barW,
				top,
				h: base - top,
				hy: base - hh,
				hh,
				py: base - hh - ph,
				ph,
				oy: top,
				oh,
				total: m.total
			};
		});
	});
	const ticks = $derived(
		[0, 0.5, 1].map((f) => ({
			v: Math.round(monthlyMax * f),
			y: CH.h - CH.padB - f * (CH.h - CH.padB - CH.padT)
		}))
	);
	const hasTrend = $derived(trend.some((m) => m.total > 0));

	const mediaMax = $derived(Math.max(1, ...(data?.media ?? []).map((m) => m.total)));

	function claimLabel(s: string | null): string {
		if (!s) return '—';
		if (s === 'submitted' || s === 'checking' || s === 'contacted') return tr('Terbuka');
		if (s === 'refunded' || s === 'replaced') return tr('Terpakai');
		return s;
	}

	// Unduh CSV dari data yang sedang tampil.
	function exportCsv() {
		const items = data?.items ?? [];
		const esc = (v: unknown) => {
			const s = v == null ? '' : String(v);
			return /[",\n;]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
		};
		const header = ['Judul', 'Media', 'Tanggal terbit', 'Cek terakhir', 'Status link', 'Masalah', 'Garansi', 'Klaim', 'URL artikel'];
		const rows = items.map((i) => [
			i.document_title,
			i.media_name,
			i.published_at ? i.published_at.slice(0, 10) : '',
			i.last_checked_at ? i.last_checked_at.slice(0, 10) : '',
			i.link_state ?? 'belum dicek',
			i.problem_kind ? (PROBLEM_LABEL[i.problem_kind] ?? i.problem_kind) : '',
			i.in_warranty ? 'Berlaku' : 'Habis',
			claimLabel(i.claim_status),
			i.published_url ?? ''
		]);
		const csv = [header, ...rows].map((r) => r.map(esc).join(',')).join('\n');
		const blob = new Blob(['﻿' + csv], { type: 'text/csv;charset=utf-8' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `${fileBase()}.csv`;
		a.click();
		URL.revokeObjectURL(url);
	}

	function statusText(i: Item): string {
		if (!i.link_state) return tr('Belum dicek');
		if (i.link_state === 'live') return i.problem_kind ? tr('Bermasalah') : tr('Hidup');
		if (i.link_state === 'gone') return tr('Hilang');
		if (i.link_state === 'unreachable') return tr('Tak terjangkau');
		return i.link_state;
	}

	// Unduh rekap PDF: ringkasan angka dan daftar artikel, dibuat di browser.
	async function exportPdf() {
		const { jsPDF } = await import('jspdf');
		const doc = new jsPDF({ unit: 'mm', format: 'a4' });
		const items = data?.items ?? [];
		const t = data?.totals;
		const L = 14;
		const R = 196;
		const INK: [number, number, number] = [17, 24, 39];
		const MUTED: [number, number, number] = [110, 118, 130];
		const LINE: [number, number, number] = [226, 230, 237];
		const BAD: [number, number, number] = [192, 57, 43];
		const today = new Date().toLocaleDateString('id-ID', { day: 'numeric', month: 'long', year: 'numeric' });

		// Potong teks sesuai lebar kolom, dengan tanda …
		const fit = (text: string, maxW: number) => {
			let str = text || '—';
			if (doc.getTextWidth(str) <= maxW) return str;
			while (str.length > 1 && doc.getTextWidth(str + '…') > maxW) str = str.slice(0, -1);
			return str + '…';
		};
		const rule = (yy: number, color: [number, number, number] = LINE) => {
			doc.setDrawColor(...color);
			doc.setLineWidth(0.2);
			doc.line(L, yy, R, yy);
		};

		let y = 22;
		doc.setFont('helvetica', 'bold');
		doc.setFontSize(18);
		doc.setTextColor(...INK);
		doc.text(tr('Audit Publikasi'), L, y);
		y += 7;
		doc.setFont('helvetica', 'normal');
		doc.setFontSize(10);
		doc.setTextColor(...MUTED);
		doc.text(`${periodLabel || tr('Semua periode')}  ·  ${today}`, L, y);
		y += 6;
		rule(y);
		y += 9;

		// Ringkasan: lima kolom sejajar, angka bermasalah dan hilang merah bila ada
		if (t) {
			const cells: [string, number, boolean][] = [
				[tr('Terbit'), t.published, false],
				[tr('Sehat'), t.healthy, false],
				[tr('Bermasalah'), t.problem, t.problem > 0],
				[tr('Hilang'), t.gone, t.gone > 0],
				[tr('Klaim terbuka'), t.claims_open, false]
			];
			const step = (R - L) / cells.length;
			cells.forEach(([label, value, bad], k) => {
				const x = L + k * step;
				doc.setFont('helvetica', 'normal');
				doc.setFontSize(9);
				doc.setTextColor(...MUTED);
				doc.text(label, x, y);
				doc.setFont('helvetica', 'bold');
				doc.setFontSize(15);
				doc.setTextColor(...(bad ? BAD : INK));
				doc.text(String(value), x, y + 8);
			});
			y += 17;
			rule(y);
			y += 9;
		}

		// Grafik: tren 12 bulan, kondisi tautan, dan media terbanyak (digambar langsung)
		const BLUE: [number, number, number] = [44, 94, 173];
		const OTHER: [number, number, number] = [205, 213, 224];
		const GREY: [number, number, number] = [150, 156, 166];
		const section = (text: string) => {
			if (y > 226) {
				doc.addPage();
				y = 22;
			}
			doc.setFont('helvetica', 'bold');
			doc.setFontSize(11);
			doc.setTextColor(...INK);
			doc.text(text, L, y);
			y += 7;
		};
		const monthShort = (k: string) =>
			new Date(Number(k.slice(0, 4)), Number(k.slice(5, 7)) - 1, 1).toLocaleDateString('id-ID', { month: 'short' });
		const legendItem = (x: number, color: [number, number, number], label: string) => {
			doc.setFillColor(...color);
			doc.rect(x, y - 2.5, 2.5, 2.5, 'F');
			doc.setFont('helvetica', 'normal');
			doc.setFontSize(8);
			doc.setTextColor(...MUTED);
			doc.text(label, x + 4, y);
		};

		// 1. Tren 12 bulan: kolom bertumpuk, Januari kiri sampai Desember kanan
		section(tr('Tren 12 bulan'));
		legendItem(L, BLUE, tr('Sehat'));
		legendItem(L + 24, [192, 57, 43], tr('Bermasalah'));
		legendItem(L + 56, OTHER, tr('Lainnya'));
		y += 6;
		const chartH = 36;
		const baseY = y + chartH;
		const tMax = Math.max(4, ...trend.map((m) => m.total));
		const stepX = (R - L) / 12;
		const bw = stepX * 0.5;
		const sc = (v: number) => (v / tMax) * chartH;
		doc.setDrawColor(...LINE);
		doc.setLineWidth(0.2);
		[0, 0.5, 1].forEach((f) => doc.line(L, baseY - f * chartH, R, baseY - f * chartH));
		trend.forEach((m, i) => {
			const x = L + i * stepX + (stepX - bw) / 2;
			const h = sc(m.healthy);
			const pr = sc(m.problem);
			const o = sc(Math.max(0, m.total - m.healthy - m.problem));
			if (h > 0) {
				doc.setFillColor(...BLUE);
				doc.rect(x, baseY - h, bw, h, 'F');
			}
			if (pr > 0) {
				doc.setFillColor(...BAD);
				doc.rect(x, baseY - h - pr, bw, pr, 'F');
			}
			if (o > 0) {
				doc.setFillColor(...OTHER);
				doc.rect(x, baseY - h - pr - o, bw, o, 'F');
			}
			doc.setFont('helvetica', 'normal');
			doc.setFontSize(7);
			if (m.total > 0) {
				doc.setTextColor(...INK);
				doc.text(String(m.total), x + bw / 2, baseY - sc(m.total) - 1.5, { align: 'center' });
			}
			doc.setTextColor(...MUTED);
			doc.text(monthShort(m.month), x + bw / 2, baseY + 4, { align: 'center' });
		});
		doc.setDrawColor(...GREY);
		doc.line(L, baseY, R, baseY);
		y = baseY + 12;

		// 2. Kondisi tautan: satu batang proporsi dengan rincian
		if (t && t.published > 0) {
			section(tr('Kondisi tautan'));
			const toneColor = (tone: string): [number, number, number] =>
				tone === 'good' ? BLUE : tone === 'bad' ? BAD : tone === 'muted' ? GREY : OTHER;
			let bx = L;
			for (const c of conditions) {
				const w = (c.value / t.published) * (R - L);
				if (w > 0) {
					doc.setFillColor(...toneColor(c.tone));
					doc.rect(bx, y, w, 5, 'F');
					bx += w;
				}
			}
			y += 11;
			const colW = (R - L) / conditions.length;
			conditions.forEach((c, k) => {
				const x = L + k * colW;
				doc.setFillColor(...toneColor(c.tone));
				doc.rect(x, y - 2.5, 2.5, 2.5, 'F');
				doc.setFont('helvetica', 'normal');
				doc.setFontSize(8);
				doc.setTextColor(...MUTED);
				doc.text(c.label, x + 4, y);
				doc.setFont('helvetica', 'bold');
				doc.setFontSize(10);
				doc.setTextColor(...INK);
				doc.text(`${c.value}  ·  ${pct(c.value, t.published)}%`, x + 4, y + 6);
			});
			y += 14;
		}

		// 3. Media terbanyak: batang horizontal, biru sehat dan merah bermasalah
		const media = (data?.media ?? []).slice(0, 7);
		if (media.length > 0) {
			section(tr('Media terbanyak'));
			const mMax = Math.max(1, ...media.map((m) => m.total));
			const barX = L + 56;
			const barW = R - 22 - barX;
			media.forEach((m) => {
				doc.setFont('helvetica', 'normal');
				doc.setFontSize(9);
				doc.setTextColor(...INK);
				doc.text(fit(m.name, 52), L, y);
				const okW = ((m.total - m.problem) / mMax) * barW;
				const badW = (m.problem / mMax) * barW;
				doc.setFillColor(...OTHER);
				doc.rect(barX, y - 3, barW, 3.2, 'F');
				if (okW > 0) {
					doc.setFillColor(...BLUE);
					doc.rect(barX, y - 3, okW, 3.2, 'F');
				}
				if (badW > 0) {
					doc.setFillColor(...BAD);
					doc.rect(barX + okW, y - 3, badW, 3.2, 'F');
				}
				doc.setFont('helvetica', 'bold');
				doc.text(String(m.total), R, y, { align: 'right' });
				y += 6.5;
			});
			y += 6;
		}

		// Tabel: kolom tetap, garis tipis antarbaris
		const col = {
			title: { x: L, w: 84 },
			media: { x: 102, w: 38 },
			date: { x: 143, w: 24 },
			status: { x: 170, w: 26 }
		};
		const header = () => {
			doc.setFont('helvetica', 'bold');
			doc.setFontSize(8.5);
			doc.setTextColor(...MUTED);
			doc.text(tr('Judul'), col.title.x, y);
			doc.text(tr('Media'), col.media.x, y);
			doc.text(tr('Terbit'), col.date.x, y);
			doc.text(tr('Status'), col.status.x, y);
			y += 3;
			rule(y, INK);
			y += 5;
		};
		header();
		for (const i of items) {
			if (y > 274) {
				doc.addPage();
				y = 22;
				header();
			}
			doc.setFont('helvetica', 'normal');
			doc.setFontSize(9);
			doc.setTextColor(...INK);
			doc.text(fit(i.document_title, col.title.w), col.title.x, y);
			doc.text(fit(i.media_name, col.media.w), col.media.x, y);
			doc.text(fmtShort(i.published_at), col.date.x, y);
			doc.setTextColor(...(i.problem_kind ? BAD : INK));
			doc.text(fit(statusText(i), col.status.w), col.status.x, y);
			y += 3.5;
			rule(y);
			y += 4;
		}
		if (items.length === 0) {
			doc.setFont('helvetica', 'normal');
			doc.setFontSize(9);
			doc.setTextColor(...MUTED);
			doc.text(tr('Tidak ada data untuk periode ini.'), L, y);
		}

		// Footer di setiap halaman
		const pages = doc.getNumberOfPages();
		for (let p = 1; p <= pages; p++) {
			doc.setPage(p);
			doc.setFont('helvetica', 'normal');
			doc.setFontSize(8);
			doc.setTextColor(...MUTED);
			doc.text('Tauthub · Audit Publikasi', L, 288);
			doc.text(`${p} / ${pages}`, R, 288, { align: 'right' });
		}

		doc.save(`${fileBase()}.pdf`);
	}

	const fmtShort = (v: string | null) =>
		v ? new Date(v).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' }) : '—';
</script>

<PageTitle title={tr('Audit Publikasi')} />
<svelte:window
	onkeydown={(e) => {
		if (e.key === 'Escape') pickerOpen = false;
	}}
	onpointerdown={(e) => {
		if (pickerOpen && periodEl && !periodEl.contains(e.target as Node)) pickerOpen = false;
	}}
/>

<section class="aud">
	<header class="aud-head">
		<div>
			<h1 class="page-title">{tr('Audit Publikasi')}</h1>
			<p class="page-desc">{tr('Rekap kepatuhan artikel terbit: link hidup, hilang, atau tidak terjangkau.')}</p>
		</div>
		<button class="aud-btn" onclick={() => load(true)} disabled={loading || refreshing} aria-label={tr('Muat ulang')}>
			<RefreshCw size={15} class={refreshing ? 'aud-spin' : ''} />
			<span>{tr('Muat ulang')}</span>
		</button>
	</header>

	<div class="aud-toolbar">
		<div class="aud-period" bind:this={periodEl}>
			<span class="aud-period-label">{tr('Periode terbit')}</span>
			<button class="aud-trigger" aria-haspopup="dialog" aria-expanded={pickerOpen} onclick={togglePicker}>
				<CalendarDays size={15} />
				<span>{periodLabel || tr('Semua periode')}</span>
				<ChevronDown size={14} />
			</button>
			{#if pickerOpen}
				<div class="aud-picker" role="dialog" aria-label={tr('Periode terbit')}>
					<div class="aud-picker-year">
						<button class="aud-step" onclick={() => pickerYear--} aria-label={tr('Sebelumnya')}>
							<ChevronLeft size={14} />
						</button>
						<strong>{pickerYear}</strong>
						<button
							class="aud-step"
							onclick={() => pickerYear++}
							disabled={pickerYear >= MAX_Y}
							aria-label={tr('Berikutnya')}
						>
							<ChevronRight size={14} />
						</button>
					</div>
					<div class="aud-picker-months">
						{#each MONTH_SHORT as m, i (i)}
							<button
								class="aud-picker-m"
								class:is-current={month === `${pickerYear}-${String(i + 1).padStart(2, '0')}`}
								disabled={isFuture(pickerYear, i)}
								onclick={() => pickMonthIdx(i)}
							>
								{m}
							</button>
						{/each}
					</div>
					<button class="aud-picker-all" onclick={() => pickPeriod('')}>{tr('Semua periode')}</button>
				</div>
			{/if}
		</div>
		<div class="aud-exports">
			<button class="aud-text-btn" onclick={exportCsv} disabled={!data || (data.items ?? []).length === 0}>
			<Download size={15} />
			<span>{tr('Unduh CSV')}</span>
		</button>
			<button class="aud-text-btn" onclick={exportPdf} disabled={!data || (data.items ?? []).length === 0}>
			<Download size={15} />
			<span>{tr('Unduh PDF')}</span>
		</button>
		</div>
	</div>

	{#if loading}
		<p class="aud-muted">{tr('Memuat audit publikasi…')}</p>
	{:else if error}
		<div class="aud-error" role="alert">
			<p>{error}</p>
			<button class="aud-btn" onclick={() => load()}>{tr('Coba lagi')}</button>
		</div>
	{:else if data}
		<!-- Baris 1: cincin kepatuhan (sempit) + tren 12 bulan (lebar) -->
		<div class="aud-row aud-row-a">
			<div class="aud-card">
				<h2 class="aud-card-title">{tr('Kondisi tautan')}</h2>
				<div class="ring-wrap">
					<svg class="ring" viewBox="0 0 160 160" role="img" aria-label={tr('Kondisi tautan')}>
						<circle class="ring-track" cx="80" cy="80" r={RING_R} />
						{#each ring as s (s.key)}
							<circle
								class="ring-seg seg-{s.tone}"
								cx="80"
								cy="80"
								r={RING_R}
								stroke-dasharray={s.dash}
								stroke-dashoffset={s.offset}
							/>
						{/each}
					</svg>
					<div class="ring-center">
						<span class="ring-num">{healthPct}<small>%</small></span>
						<span class="ring-label">{tr('Sehat')}</span>
					</div>
				</div>
				<ul class="cond-list">
					{#each conditions as c (c.key)}
						<li>
							<span class="dot dot-{c.tone}"></span>
							<span class="cond-name">{c.label}</span>
							<strong>{c.value}</strong>
							<span class="cond-pct">{pct(c.value, data.totals.published)}%</span>
						</li>
					{/each}
				</ul>
			</div>

			<div class="aud-card">
				<div class="aud-card-head">
					<h2 class="aud-card-title">{tr('Tren 12 bulan')}</h2>
					<div class="aud-legend">
						<span><i class="dot-good"></i>{tr('Sehat')}</span>
						<span><i class="dot-bad"></i>{tr('Bermasalah')}</span>
						<span><i class="dot-faint"></i>{tr('Lainnya')}</span>
					</div>
				</div>
				{#if hasTrend}
					<svg class="aud-chart" viewBox="0 0 {CH.w} {CH.h}" role="img" aria-label={tr('Tren 12 bulan')}>
						{#each ticks as t (t.y)}
							<line class="ax" x1={CH.padL} x2={CH.w - 8} y1={t.y} y2={t.y} />
							<text class="tick" x={CH.padL - 8} y={t.y + 4} text-anchor="end">{t.v}</text>
						{/each}
						<g class="bars">
							{#each bars as b (b.month)}
								<g class="bar" style="--i:{b.i}">
									<title>{b.label}: {b.total}</title>
									{#if b.total > 0}
										<clipPath id="aud-bar-{b.month}">
											<rect x={b.x} y={b.top} width={b.barW} height={b.h} rx="4" />
										</clipPath>
										<g clip-path="url(#aud-bar-{b.month})">
											<rect class="s-good" x={b.x} y={b.hy} width={b.barW} height={b.hh} />
											<rect class="s-bad" x={b.x} y={b.py} width={b.barW} height={b.ph} />
											<rect class="s-other" x={b.x} y={b.oy} width={b.barW} height={b.oh} />
										</g>
										<text class="val" x={b.x + b.barW / 2} y={b.top - 6} text-anchor="middle">{b.total}</text>
									{/if}
									<text class="tick" x={b.x + b.barW / 2} y={CH.h - 8} text-anchor="middle">{b.label}</text>
								</g>
							{/each}
						</g>
					</svg>
				{:else}
					<p class="aud-muted">{tr('Tidak ada data untuk periode ini.')}</p>
				{/if}
			</div>
		</div>

		<!-- Baris 2: peringkat media (lebar) + tiga angka (sempit) -->
		<div class="aud-row aud-row-b">
			<div class="aud-card">
				<h2 class="aud-card-title">{tr('Media terbanyak')}</h2>
				{#if (data.media ?? []).length === 0}
					<p class="aud-muted">{tr('Tidak ada data untuk periode ini.')}</p>
				{:else}
					<ol class="aud-rank">
						{#each data.media as m, i (m.name)}
							<li>
								<span class="rank-no">{i + 1}</span>
								<span class="rank-name">{m.name || '—'}</span>
								<span class="rank-val">{m.total}</span>
								<span class="rank-track">
									<i class="rank-ok" style="width:{((m.total - m.problem) / mediaMax) * 100}%"></i>
									<i class="rank-bad" style="width:{(m.problem / mediaMax) * 100}%"></i>
								</span>
							</li>
						{/each}
					</ol>
				{/if}
			</div>

			<div class="aud-tally">
				<div class="tally-item">
					<span class="tally-label">{tr('Terbit')}</span>
					<span class="tally-num">{data.totals.published}</span>
					<span class="tally-note">{periodLabel || tr('Semua periode')}</span>
				</div>
				<div class="tally-item">
					<span class="tally-label">{tr('Hilang')}</span>
					<span class="tally-num" class:is-bad={data.totals.gone > 0}>{data.totals.gone}</span>
					<span class="tally-note">{pct(data.totals.gone, data.totals.published)}% {tr('dari artikel terbit')}</span>
				</div>
				<div class="tally-item">
					<span class="tally-label">{tr('Klaim terbuka')}</span>
					<span class="tally-num" class:is-accent={data.totals.claims_open > 0}>{data.totals.claims_open}</span>
					<span class="tally-note">{tr('Klaim garansi yang sedang diproses')}</span>
				</div>
			</div>
		</div>
	{/if}
</section>

<style>
	.aud {
		--aud-line: color-mix(in srgb, currentColor 9%, transparent);
		--aud-muted: color-mix(in srgb, currentColor 58%, transparent);
		--aud-good: #2c5ead;
		--aud-bad: #c0392b;
		--aud-ok: #2e7d4f;
		--aud-other: #cdd5e0;
		display: flex;
		flex-direction: column;
		gap: 24px;
	}
	.aud-head {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 16px;
		flex-wrap: wrap;
	}
	.aud-btn {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		height: 36px;
		padding: 0 12px;
		font: inherit;
		font-size: 14px;
		color: inherit;
		background: #fff;
		border: 1px solid var(--aud-line);
		border-radius: 8px;
		cursor: pointer;
		transition: background-color 150ms ease;
	}
	.aud-btn:hover:not(:disabled) {
		background: color-mix(in srgb, currentColor 4%, #fff);
	}
	.aud-btn:disabled {
		opacity: 0.4;
		cursor: default;
	}
	.aud-text-btn {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		height: 36px;
		padding: 0 8px;
		font: inherit;
		font-size: 14px;
		color: var(--aud-muted);
		background: transparent;
		border: 0;
		border-radius: 8px;
		cursor: pointer;
		transition: background-color 150ms ease, color 150ms ease;
	}
	.aud-text-btn:hover:not(:disabled) {
		color: inherit;
		background: color-mix(in srgb, currentColor 5%, transparent);
	}
	.aud-text-btn:disabled {
		opacity: 0.4;
		cursor: default;
	}
	.aud-exports {
		display: flex;
		gap: 4px;
		margin-left: auto;
	}
	:global(.aud-spin) {
		animation: aud-spin 0.9s linear infinite;
	}
	@keyframes aud-spin {
		to {
			transform: rotate(360deg);
		}
	}
	.aud-toolbar {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 12px;
	}
	.aud-period {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		color: var(--aud-muted);
	}
	.aud-period {
		position: relative;
		display: inline-flex;
		align-items: center;
		gap: 8px;
	}
	.aud-period-label {
		font-size: 13px;
		color: var(--aud-muted);
	}
	.aud-trigger {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		height: 36px;
		padding: 0 12px;
		font: inherit;
		font-size: 14px;
		color: inherit;
		background: #fff;
		border: 1px solid var(--aud-line);
		border-radius: 8px;
		cursor: pointer;
		transition: background-color 150ms ease;
	}
	.aud-trigger:hover {
		background: color-mix(in srgb, currentColor 3%, #fff);
	}
	.aud-picker {
		position: absolute;
		top: calc(100% + 8px);
		left: 0;
		z-index: 20;
		width: 260px;
		padding: 12px;
		background: #fff;
		border: 1px solid var(--aud-line);
		border-radius: 12px;
		box-shadow: 0 8px 24px rgba(15, 23, 42, 0.1);
		animation: aud-pop 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	@keyframes aud-pop {
		from {
			opacity: 0;
			transform: translateY(-4px);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}
	.aud-picker-year {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 10px;
		font-size: 15px;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}
	.aud-step {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		background: #fff;
		border: 1px solid var(--aud-line);
		border-radius: 8px;
		color: var(--aud-muted);
		cursor: pointer;
	}
	.aud-step:hover:not(:disabled) {
		color: var(--aud-good);
	}
	.aud-step:disabled {
		opacity: 0.35;
		cursor: default;
	}
	.aud-picker-months {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 6px;
	}
	.aud-picker-m {
		height: 36px;
		font: inherit;
		font-size: 13px;
		color: inherit;
		background: transparent;
		border: 1px solid transparent;
		border-radius: 8px;
		cursor: pointer;
		transition: background-color 150ms ease;
	}
	.aud-picker-m:hover:not(:disabled) {
		background: color-mix(in srgb, currentColor 5%, transparent);
	}
	.aud-picker-m:disabled {
		opacity: 0.35;
		cursor: default;
	}
	.aud-picker-m.is-current {
		background: color-mix(in srgb, var(--aud-good) 10%, #fff);
		border-color: color-mix(in srgb, var(--aud-good) 35%, transparent);
		color: var(--aud-good);
		font-weight: 600;
	}
	.aud-picker-all {
		width: 100%;
		margin-top: 10px;
		height: 32px;
		font: inherit;
		font-size: 13px;
		color: var(--aud-muted);
		background: transparent;
		border: 0;
		border-top: 1px solid var(--aud-line);
		padding-top: 8px;
		cursor: pointer;
	}
	.aud-picker-all:hover {
		color: inherit;
	}
	.aud-link-btn {
		font: inherit;
		font-size: 13px;
		color: inherit;
		background: transparent;
		border: 0;
		text-decoration: underline;
		text-underline-offset: 3px;
		cursor: pointer;
	}
	.aud-muted {
		color: var(--aud-muted);
		margin: 8px 0;
	}
	.aud-error {
		padding: 16px;
		border-radius: 16px;
		background: #fff;
		border: 1px solid color-mix(in srgb, var(--aud-bad) 40%, transparent);
	}

	/* Kartu putih: permukaan utama halaman ini */
	.aud-card {
		background: #fff;
		border: 1px solid var(--aud-line);
		border-radius: 16px;
		padding: 24px;
		box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
		min-width: 0;
	}
	.aud-card-title {
		font-size: 15px;
		font-weight: 600;
		margin: 0;
	}
	.aud-card-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		flex-wrap: wrap;
		margin-bottom: 16px;
	}
	.aud-row {
		display: grid;
		gap: 24px;
		align-items: stretch;
	}
	.aud-row-a {
		grid-template-columns: minmax(0, 2fr) minmax(0, 3fr);
	}
	.aud-row-b {
		grid-template-columns: minmax(0, 1fr) 240px;
	}

	/* Cincin kepatuhan: tanda khas halaman ini */
	.ring-wrap {
		position: relative;
		width: 160px;
		height: 160px;
		margin: 16px auto;
	}
	.ring-wrap > .ring {
		display: block;
		width: 100%;
		height: 100%;
		transform: rotate(-90deg);
		border: 0;
		outline: 0;
		box-shadow: none;
		overflow: visible;
	}
	.ring circle {
		fill: none;
		stroke-width: 14;
	}
	.ring-track {
		stroke: color-mix(in srgb, currentColor 7%, transparent);
	}
	.ring-seg.seg-good {
		stroke: var(--aud-ok);
	}
	.ring-seg.seg-bad {
		stroke: var(--aud-bad);
	}
	.ring-seg.seg-muted {
		stroke: color-mix(in srgb, currentColor 35%, transparent);
	}
	.ring-seg.seg-faint {
		stroke: var(--aud-other);
	}
	.ring-center {
		position: absolute;
		inset: 0;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 2px;
	}
	.ring-num {
		font-size: 36px;
		line-height: 1;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}
	.ring-num small {
		font-size: 18px;
		font-weight: 500;
		color: var(--aud-muted);
	}
	.ring-label {
		font-size: 13px;
		color: var(--aud-muted);
	}
	.cond-list {
		list-style: none;
		padding: 0;
		margin: 0;
		display: flex;
		flex-direction: column;
		gap: 12px;
		font-size: 14px;
	}
	.cond-list li {
		display: grid;
		grid-template-columns: 8px minmax(0, 1fr) auto 40px;
		align-items: center;
		gap: 8px;
	}
	.cond-list strong {
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}
	.cond-pct {
		text-align: right;
		color: var(--aud-muted);
		font-size: 13px;
		font-variant-numeric: tabular-nums;
	}
	.dot {
		width: 8px;
		height: 8px;
		border-radius: 999px;
		display: inline-block;
	}
	.dot-good {
		background: var(--aud-ok);
	}
	.dot-bad {
		background: var(--aud-bad);
	}
	.dot-muted {
		background: color-mix(in srgb, currentColor 35%, transparent);
	}
	.dot-faint {
		background: var(--aud-other);
	}
	/* Legenda grafik tren memakai warna kolomnya: biru untuk sehat */
	.aud-legend .dot-good {
		background: var(--aud-good);
	}
	.aud-legend {
		display: flex;
		gap: 16px;
		font-size: 12px;
		color: var(--aud-muted);
	}
	.aud-legend span {
		display: inline-flex;
		align-items: center;
		gap: 6px;
	}

	.aud-chart {
		width: 100%;
		height: auto;
		display: block;
	}
	.aud-chart .ax {
		stroke: var(--aud-line);
		stroke-width: 1;
	}
	.aud-chart .tick {
		font-size: 11px;
		fill: var(--aud-muted);
		font-variant-numeric: tabular-nums;
	}
	.aud-chart .val {
		font-size: 11px;
		font-weight: 600;
		fill: currentColor;
		font-variant-numeric: tabular-nums;
	}
	.aud-chart .s-good {
		fill: var(--aud-good);
	}
	.aud-chart .s-bad {
		fill: var(--aud-bad);
	}
	.aud-chart .s-other {
		fill: var(--aud-other);
	}
	.aud-chart .bar {
		transition: opacity 150ms ease;
	}
	.aud-chart .bars:hover .bar:not(:hover) {
		opacity: 0.5;
	}
	/* Satu entrance saat dimuat: kolom tumbuh dari dasar */
	.aud-chart .bar > g {
		transform-box: fill-box;
		transform-origin: 50% 100%;
		animation: aud-grow 420ms cubic-bezier(0.22, 1, 0.36, 1) both;
		animation-delay: calc(var(--i) * 24ms);
	}
	@keyframes aud-grow {
		from {
			transform: scaleY(0.2);
			opacity: 0;
		}
		to {
			transform: none;
			opacity: 1;
		}
	}

	/* Peringkat media */
	.aud-rank {
		list-style: none;
		padding: 0;
		margin: 16px 0 0;
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.aud-rank li {
		display: grid;
		grid-template-columns: 20px minmax(0, 1fr) auto;
		grid-template-rows: auto auto;
		column-gap: 8px;
		row-gap: 4px;
		align-items: baseline;
		font-size: 14px;
	}
	.rank-no {
		color: var(--aud-muted);
		font-size: 13px;
		font-variant-numeric: tabular-nums;
	}
	.rank-name {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.rank-val {
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}
	.rank-track {
		grid-column: 2 / 4;
		display: flex;
		height: 4px;
		background: var(--aud-line);
		border-radius: 999px;
		overflow: hidden;
	}
	.rank-track i {
		display: block;
		height: 100%;
	}
	.rank-ok {
		background: var(--aud-good);
	}
	.rank-bad {
		background: var(--aud-bad);
	}

	/* Tiga angka: satu kartu, tiap angka punya label dan keterangan. Warna hanya bila ada arti. */
	.aud-tally {
		display: flex;
		flex-direction: column;
		background: #fff;
		border: 1px solid var(--aud-line);
		border-radius: 16px;
		padding: 0 24px;
		box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04);
	}
	.tally-item {
		flex: 1;
		display: flex;
		flex-direction: column;
		justify-content: center;
		gap: 4px;
		padding: 20px 0;
	}
	.tally-item + .tally-item {
		border-top: 1px solid var(--aud-line);
	}
	.tally-label {
		font-size: 13px;
		color: var(--aud-muted);
	}
	.tally-num {
		font-size: 32px;
		line-height: 1.1;
		font-weight: 500;
		font-variant-numeric: tabular-nums;
	}
	.tally-num.is-bad {
		color: var(--aud-bad);
	}
	.tally-num.is-accent {
		color: var(--aud-good);
	}
	.tally-note {
		font-size: 12px;
		color: var(--aud-muted);
	}

	@media (max-width: 900px) {
		.aud-row-a,
		.aud-row-b {
			grid-template-columns: minmax(0, 1fr);
		}
		.aud-card {
			padding: 16px;
		}
		.tally-num {
			font-size: 26px;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.aud-chart .bar > g {
			animation: none;
		}
		:global(.aud-spin) {
			animation: none;
		}
	}
</style>
