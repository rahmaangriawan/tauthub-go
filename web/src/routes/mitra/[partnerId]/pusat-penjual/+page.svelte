<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Pusat Penjual: tempat harian mitra. Isinya urut dari yang paling perlu
	// dikerjakan: daftar tugas, pesanan yang menunggu, penghasilan, lalu kondisi
	// tiap media. Semua angka dari GET /v1/partner/seller-center (dihitung di
	// server); data yang belum ada tampil "Belum ada data", bukan 0.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ArrowRight, CircleCheck, LoaderCircle, RefreshCw, TriangleAlert } from '@lucide/svelte';
	import { getPartnerUrlId } from '$lib/partner-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	type Todo = {
		kind: string;
		severity: 'urgent' | 'warning' | 'info';
		title: string;
		detail: string;
		route: string;
		query?: string;
		deadline?: string;
		media_id?: string;
	};
	type Waiting = { id: string; document_title: string; media_name: string; deadline: string | null; remaining: string };
	type MediaCard = {
		id: string;
		name: string;
		website_url: string | null;
		status: string;
		media_type: string;
		integration_type: string | null;
		has_connection: boolean;
		catalog_visible: boolean;
		catalog_hidden_reason: string | null;
		domain_verified: boolean;
		dewan_pers_verified: boolean;
		domain_health: string;
		price: number;
		link_rel: string;
		seo_da: number | null;
		seo_pa: number | null;
		sw_visits: number | null;
		rating_avg: number | null;
		rating_count: number;
		articles_7d: number | null;
		total_terbit: number;
		pending_publications: number;
		has_history: boolean;
		health_score: number | null;
		avg_publish_hrs: number | null;
		price_benchmark: {
			peers: number;
			basis: 'category_da' | 'category' | 'media_type';
			median: number;
			p25: number;
			p75: number;
			position: 'below' | 'within' | 'above';
			price: number;
		} | null;
		last_price_change: { from: number | null; to: number | null; at: string | null } | null;
	};
	type Data = {
		summary: { media_count: number; visible_count: number; hidden_count: number; todo_count: number };
		todos: Todo[];
		pipeline: {
			counts: Record<string, number>;
			waiting: Waiting[];
			waiting_total: number;
			expiring_24h: number;
			rejection_rate: number | null;
			avg_publish_hours: number | null;
			unresponsive_cancelled: number;
			sla_hours: number;
		};
		earnings: {
			pending: number;
			available: number;
			paid: number;
			lifetime_publication: number;
			hold_hours: number;
			last_30_days: number;
			orders_30_days: number;
			series: { day: string; amount: number; orders: number }[];
		};
		media: MediaCard[];
	};

	type GscPoint = { fetched_at: string; clicks: number; impressions: number; ctr: number; position: number };
	type TrafficMedia = {
		media_id: string;
		name: string;
		website_url: string | null;
		matched: boolean;
		site_url: string;
		gsc: GscPoint[];
		similarweb: { date: string | null; visits: number }[];
	};
	type Traffic = { connected: boolean; connections: number; period_days: number; media: TrafficMedia[] };

	const partnerId = $derived(page.params.partnerId as string);
	const base = $derived(`/mitra/${getPartnerUrlId() || partnerId}`);

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state('');
	let data = $state<Data | null>(null);
	let showAllTodos = $state(false);

	// Search Console: dimuat terpisah supaya galatnya tidak menutup halaman utama.
	let traffic = $state<Traffic | null>(null);
	let trafficError = $state('');
	let trafficNotice = $state('');
	let connecting = $state(false);
	let pullingId = $state<string | null>(null);

	async function loadTraffic() {
		try {
			const res = await apiRequest('/v1/partner/traffic');
			if (!res.ok) {
				trafficError = await readApiError(res, tr('Gagal memuat data trafik.'));
				return;
			}
			traffic = (await res.json()) as Traffic;
		} catch {
			trafficError = tr('Gagal memuat data trafik.');
		}
	}

	async function connectGsc() {
		connecting = true;
		trafficError = '';
		try {
			const res = await apiRequest('/v1/partner/gsc/connect');
			if (!res.ok) {
				trafficError = await readApiError(res, tr('Gagal memulai sambungan Search Console.'));
				return;
			}
			const j = (await res.json().catch(() => ({}))) as { url?: string };
			if (j.url) window.location.href = j.url;
			else trafficError = tr('Gagal memulai sambungan Search Console.');
		} catch {
			trafficError = tr('Gagal memulai sambungan Search Console.');
		} finally {
			connecting = false;
		}
	}

	async function pullGsc(mediaId: string) {
		pullingId = mediaId;
		trafficError = '';
		trafficNotice = '';
		try {
			const res = await apiRequest(`/v1/partner/media/${mediaId}/gsc-snapshot`, { method: 'POST' });
			if (!res.ok) {
				trafficError = await readApiError(res, tr('Gagal mengambil data Search Console.'));
				return;
			}
			trafficNotice = tr('Data Search Console diperbarui.');
			await loadTraffic();
		} catch {
			trafficError = tr('Gagal mengambil data Search Console.');
		} finally {
			pullingId = null;
		}
	}

	/** Perubahan klik dibanding pengambilan sebelumnya; kosong bila data terlalu kecil untuk dibandingkan. */
	function clickTrend(points: GscPoint[]): string {
		if (points.length < 2) return '';
		const prev = points[points.length - 2].clicks;
		const last = points[points.length - 1].clicks;
		if (prev < 10) return '';
		const pct = Math.round(((last - prev) / prev) * 100);
		if (pct === 0) return tr('Stabil dibanding pengambilan sebelumnya');
		return `${pct > 0 ? tr('Naik') : tr('Turun')} ${Math.abs(pct)}% ${tr('dibanding pengambilan sebelumnya')}`;
	}

	function sparkPath(points: GscPoint[]): string {
		if (points.length < 2) return '';
		const max = Math.max(1, ...points.map((p) => p.clicks));
		const step = 100 / (points.length - 1);
		return points.map((p, i) => `${i === 0 ? 'M' : 'L'}${(i * step).toFixed(1)},${(26 - (p.clicks / max) * 24).toFixed(1)}`).join(' ');
	}

	const fmtDate = (v: string | null | undefined) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '—');

	async function load(silent = false) {
		if (silent) refreshing = true;
		else loading = true;
		error = '';
		try {
			const res = await apiRequest('/v1/partner/seller-center');
			if (!res.ok) {
				error = tr('Gagal memuat Pusat Penjual. Coba muat ulang halaman.');
				return;
			}
			data = (await res.json()) as Data;
		} catch {
			error = tr('Gagal memuat Pusat Penjual. Coba muat ulang halaman.');
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			if (page.url.searchParams.get('gsc_connected') === '1') {
				trafficNotice = tr('Search Console tersambung. Data pertama diambil otomatis dalam beberapa menit, atau klik Ambil data.');
			}
			await Promise.all([load(), loadTraffic()]);
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}`;
	const fmt = (n: number) => (n ?? 0).toLocaleString('id-ID');
	const href = (t: Todo) => `${base}/${t.route}${t.query ? `?${t.query}` : ''}`;
	const host = (u: string | null) => (u ?? '').replace(/^https?:\/\//, '').replace(/^www\./, '').replace(/\/.*$/, '');

	const severityLabel = (s: Todo['severity']) =>
		s === 'urgent' ? tr('Mendesak') : s === 'warning' ? tr('Perlu perhatian') : tr('Saran');

	const visibleTodos = $derived(data ? (showAllTodos ? data.todos : data.todos.slice(0, 6)) : []);

	const basisText = (b: 'category_da' | 'category' | 'media_type') =>
		b === 'category_da'
			? tr('kategori dan DA serupa')
			: b === 'category'
				? tr('kategori yang sama')
				: tr('jenis media yang sama');
	const positionText = (p: 'below' | 'within' | 'above') =>
		p === 'above' ? tr('Di atas kisaran sebaya') : p === 'below' ? tr('Di bawah kisaran sebaya') : tr('Dalam kisaran sebaya');
	const positionHint = (p: 'below' | 'within' | 'above') =>
		p === 'above'
			? tr('Harga di atas kisaran bisa membuat order lebih sepi. Naikkan hanya bila situs Anda memang lebih kuat.')
			: p === 'below'
				? tr('Ada ruang untuk menaikkan harga tanpa keluar dari kisaran sebaya.')
				: tr('Harga Anda sejalan dengan media sebaya.');

	const hiddenReasonText = (r: string | null) =>
		r === 'not_connected'
			? tr('Belum tersambung ke sistem publikasi')
			: r === 'no_price'
				? tr('Belum punya harga')
				: r === 'partner_hidden'
					? tr('Anda sembunyikan')
					: r === 'suspended'
						? tr('Ditangguhkan')
						: r === 'inactive'
							? tr('Belum aktif')
							: tr('Tidak tampil di katalog');

	// Grafik batang 30 hari. Tinggi dinormalisasi ke hari tertinggi; hari tanpa
	// penghasilan hanya garis tipis supaya jelas itu nol, bukan data hilang.
	const bars = $derived.by(() => {
		const series = data?.earnings.series ?? [];
		const max = Math.max(1, ...series.map((s) => s.amount));
		const w = 300 / Math.max(series.length, 1);
		return series.map((s, i) => {
			const h = s.amount > 0 ? Math.max(3, (s.amount / max) * 56) : 1;
			return { x: i * w + 1, y: 60 - h, w: Math.max(w - 2, 1), h, s };
		});
	});
</script>

<PageTitle title={tr('Pusat Penjual')} />

<section class="sc">
	<header class="sc-head">
		<div>
			<h1 class="page-title">{tr('Pusat Penjual')}</h1>
			<p class="page-desc">{tr('Yang perlu Anda kerjakan hari ini, dan kondisi media Anda.')}</p>
		</div>
		<button class="app-btn app-btn-soft sc-refresh" type="button" disabled={loading || refreshing} onclick={() => void load(true)}>
			<RefreshCw size={15} class={refreshing ? 'animate-spin' : ''} />
			{tr('Muat ulang')}
		</button>
	</header>

	{#if error}
		<div class="app-card sc-error" role="alert">{error}</div>
	{/if}

	{#if loading}
		<div class="app-card sc-loading">{tr('Memuat Pusat Penjual…')}</div>
	{:else if data}
		<!-- Perlu dikerjakan -->
		<section class="sc-block" aria-labelledby="sc-todo-title">
			<div class="sc-block-head">
				<h2 id="sc-todo-title" class="sc-h2">{tr('Perlu dikerjakan')}</h2>
				{#if data.todos.length > 0}<span class="sc-count">{data.todos.length}</span>{/if}
			</div>
			{#if data.todos.length === 0}
				<div class="app-card sc-empty">
					<CircleCheck size={20} strokeWidth={1.5} />
					{#if data.summary.media_count === 0}
						<div>
							<p class="sc-empty-title">{tr('Belum ada media')}</p>
							<p class="sc-muted">{tr('Daftarkan media pertama Anda supaya klien bisa memesan artikel.')}</p>
							<a class="app-btn app-btn-primary sc-action" href={`${base}/media`}>{tr('Daftar Media')}<ArrowRight size={15} /></a>
						</div>
					{:else}
						<div>
							<p class="sc-empty-title">{tr('Tidak ada yang perlu dikerjakan')}</p>
							<p class="sc-muted">{tr('Media Anda siap menerima order.')}</p>
						</div>
					{/if}
				</div>
			{:else}
				<ul class="app-card sc-todos">
					{#each visibleTodos as t, i (t.kind + (t.media_id ?? '') + (t.query ?? '') + i)}
						<li class="sc-todo">
							<span class="sc-sev sc-sev-{t.severity}">{severityLabel(t.severity)}</span>
							<div class="sc-todo-body">
								<p class="sc-todo-title">{t.title}</p>
								<p class="sc-muted">{t.detail}</p>
							</div>
							<a class="app-btn app-btn-soft sc-action" href={href(t)}>{tr('Buka')}<ArrowRight size={15} /></a>
						</li>
					{/each}
				</ul>
				{#if data.todos.length > 6}
					<button class="sc-more" type="button" onclick={() => (showAllTodos = !showAllTodos)}>
						{showAllTodos ? tr('Tampilkan lebih sedikit') : `${tr('Lihat semua')} (${data.todos.length})`}
					</button>
				{/if}
			{/if}
		</section>

		<div class="sc-two">
			<!-- Pesanan menunggu -->
			<section class="app-card sc-panel" aria-labelledby="sc-orders-title">
				<h2 id="sc-orders-title" class="sc-h2">{tr('Pesanan menunggu Anda')}</h2>
				{#if data.pipeline.waiting.length === 0}
					<p class="sc-muted sc-mt">{tr('Tidak ada pesanan yang menunggu persetujuan.')}</p>
				{:else}
					<ul class="sc-orders">
						{#each data.pipeline.waiting.slice(0, 6) as o (o.id)}
							<li>
								<div class="sc-order-main">
									<p class="sc-todo-title">{o.document_title}</p>
									<p class="sc-muted">{o.media_name}</p>
								</div>
								<span class="sc-remaining" class:sc-remaining-late={o.remaining === 'sudah lewat'}>{o.remaining}</span>
								<a class="sc-order-link" href={`${base}/publications?pub=${o.id}`}>{tr('Buka')}</a>
							</li>
						{/each}
					</ul>
					{#if data.pipeline.waiting_total > 6}
						<p class="sc-muted sc-mt">{tr('Dan')} {data.pipeline.waiting_total - 6} {tr('pesanan lain.')}</p>
					{/if}
				{/if}
				<dl class="sc-facts">
					<div>
						<dt>{tr('Rata-rata waktu tayang')}</dt>
						<dd>{data.pipeline.avg_publish_hours === null ? tr('Belum ada data') : `${data.pipeline.avg_publish_hours.toFixed(1)} ${tr('jam')}`}</dd>
					</div>
					<div>
						<dt>{tr('Pesanan yang Anda tolak')}</dt>
						<dd>{data.pipeline.rejection_rate === null ? tr('Belum ada data') : `${Math.round(data.pipeline.rejection_rate * 100)}%`}</dd>
					</div>
					<div>
						<dt>{tr('Dibatalkan karena tidak direspons (30 hari)')}</dt>
						<dd>{data.pipeline.unresponsive_cancelled}</dd>
					</div>
				</dl>
				<p class="sc-note">{tr('Pesanan yang tidak disetujui dalam')} {data.pipeline.sla_hours} {tr('jam dibatalkan otomatis.')}</p>
			</section>

			<!-- Penghasilan -->
			<section class="app-card sc-panel" aria-labelledby="sc-earn-title">
				<h2 id="sc-earn-title" class="sc-h2">{tr('Penghasilan')}</h2>
				<p class="sc-big">{rp(data.earnings.available)}</p>
				<p class="sc-muted">{tr('Bisa ditarik sekarang')}</p>
				<dl class="sc-facts">
					<div>
						<dt>{tr('Menunggu masa tahan')} {data.earnings.hold_hours} {tr('jam')}</dt>
						<dd>{rp(data.earnings.pending)}</dd>
					</div>
					<div>
						<dt>{tr('30 hari terakhir')}</dt>
						<dd>{rp(data.earnings.last_30_days)} · {data.earnings.orders_30_days} {tr('artikel')}</dd>
					</div>
					<div>
						<dt>{tr('Sudah dicairkan')}</dt>
						<dd>{rp(data.earnings.paid)}</dd>
					</div>
				</dl>
				<svg class="sc-bars" viewBox="0 0 300 60" preserveAspectRatio="none" role="img"
					aria-label={tr('Penghasilan per hari, 30 hari terakhir')}>
					{#each bars as b (b.s.day)}
						<rect x={b.x} y={b.y} width={b.w} height={b.h} rx="1">
							<title>{b.s.day}: {rp(b.s.amount)}</title>
						</rect>
					{/each}
				</svg>
				<a class="sc-order-link" href={`${base}/earnings`}>{tr('Lihat rincian dan tarik saldo')}</a>
			</section>
		</div>

		<!-- Trafik dari Google (Search Console) -->
		<section class="sc-block" aria-labelledby="sc-gsc-title">
			<div class="sc-block-head">
				<h2 id="sc-gsc-title" class="sc-h2">{tr('Trafik dari Google')}</h2>
				<span class="sc-muted">{tr('Dari Google Search Console, 28 hari terakhir')}</span>
			</div>
			{#if trafficNotice}
				<div class="app-card sc-notice" role="status">{trafficNotice}</div>
			{/if}
			{#if trafficError}
				<div class="app-card sc-error" role="alert">{trafficError}</div>
			{/if}
			{#if !traffic}
				{#if !trafficError}<div class="app-card sc-loading">{tr('Memuat data trafik…')}</div>{/if}
			{:else if !traffic.connected}
				<div class="app-card sc-gsc-intro">
					<div>
						<p class="sc-todo-title">{tr('Buktikan trafik situs Anda ke klien')}</p>
						<p class="sc-muted">{tr('Hubungkan Google Search Console untuk melihat klik, tayangan, dan posisi situs Anda di Google. Akses hanya baca, dan Anda bisa mencabutnya kapan saja dari akun Google.')}</p>
					</div>
					<button class="app-btn app-btn-primary sc-action" type="button" disabled={connecting} onclick={() => void connectGsc()}>
						{#if connecting}<LoaderCircle size={15} class="animate-spin" />{/if}
						{tr('Hubungkan Search Console')}
					</button>
				</div>
			{:else}
				<ul class="app-card sc-gsc-list">
					{#each traffic.media as t (t.media_id)}
						{@const last = t.gsc.length > 0 ? t.gsc[t.gsc.length - 1] : null}
						{@const trend = clickTrend(t.gsc)}
						<li class="sc-gsc-row">
							<div class="sc-gsc-name">
								<p class="sc-todo-title">{t.name}</p>
								<p class="sc-muted">{host(t.website_url) || '—'}</p>
							</div>
							{#if !t.matched}
								<p class="sc-muted sc-gsc-msg">{tr('Situs ini belum ada di Search Console akun Anda. Tambahkan dan verifikasi situsnya di Search Console, lalu hubungkan ulang.')}</p>
							{:else if !last}
								<p class="sc-muted sc-gsc-msg">{tr('Belum ada data. Klik Ambil data untuk pengambilan pertama.')}</p>
							{:else}
								<dl class="sc-gsc-nums">
									<div><dt>{tr('Klik')}</dt><dd>{fmt(last.clicks)}</dd></div>
									<div><dt>{tr('Tayangan')}</dt><dd>{fmt(last.impressions)}</dd></div>
									<div><dt>{tr('Posisi rata-rata')}</dt><dd>{last.position.toFixed(1)}</dd></div>
								</dl>
								<div class="sc-gsc-trend">
									{#if t.gsc.length >= 2}
										<svg viewBox="0 0 100 28" preserveAspectRatio="none" class="sc-spark" role="img" aria-label={tr('Tren klik per pengambilan')}>
											<path d={sparkPath(t.gsc)} />
										</svg>
									{/if}
									<p class="sc-muted">{trend || `${tr('Diambil')} ${fmtDate(last.fetched_at)}`}</p>
								</div>
							{/if}
							{#if t.matched}
								<button class="app-btn app-btn-soft sc-action" type="button" disabled={pullingId !== null} onclick={() => void pullGsc(t.media_id)}>
									{#if pullingId === t.media_id}<LoaderCircle size={15} class="animate-spin" />{/if}
									{tr('Ambil data')}
								</button>
							{/if}
						</li>
					{/each}
				</ul>
				<p class="sc-note">{tr('Search Console hanya memberi klik, tayangan, dan posisi. Status terindeks tiap halaman tidak tersedia di sini. Data diambil otomatis seminggu sekali.')}</p>
			{/if}
		</section>

		<!-- Kesehatan media -->
		<section class="sc-block" aria-labelledby="sc-media-title">
			<div class="sc-block-head">
				<h2 id="sc-media-title" class="sc-h2">{tr('Kondisi media')}</h2>
				<span class="sc-muted">{data.summary.visible_count} {tr('tampil di katalog')} · {data.summary.hidden_count} {tr('tidak tampil')}</span>
			</div>
			{#if data.media.length === 0}
				<div class="app-card sc-empty"><p class="sc-muted">{tr('Belum ada media.')}</p></div>
			{:else}
				<div class="sc-media-grid">
					{#each data.media as m (m.id)}
						<article class="app-card sc-media">
							<header class="sc-media-head">
								<div class="sc-media-name">
									<p class="sc-todo-title">{m.name}</p>
									<p class="sc-muted">{host(m.website_url) || '—'}</p>
								</div>
								{#if m.catalog_visible}
									<span class="sc-pill sc-pill-ok">{tr('Tampil di katalog')}</span>
								{:else}
									<span class="sc-pill sc-pill-warn" title={hiddenReasonText(m.catalog_hidden_reason)}>{hiddenReasonText(m.catalog_hidden_reason)}</span>
								{/if}
							</header>
							<dl class="sc-facts sc-facts-tight">
								<div><dt>DA / PA</dt><dd>{m.seo_da === null && m.seo_pa === null ? tr('Belum ada data') : `${m.seo_da ?? '—'} / ${m.seo_pa ?? '—'}`}</dd></div>
								<div><dt>{tr('Kunjungan bulanan')}</dt><dd>{m.sw_visits === null ? tr('Belum ada data') : fmt(m.sw_visits)}</dd></div>
								<div><dt>{tr('Artikel tayang')}</dt><dd>{m.total_terbit}</dd></div>
								<div><dt>{tr('Order berjalan')}</dt><dd>{m.pending_publications}</dd></div>
								<div><dt>{tr('Rating kecepatan')}</dt><dd>{m.rating_count === 0 || m.rating_avg === null ? tr('Belum ada data') : `${m.rating_avg}/5 (${m.rating_count})`}</dd></div>
								<div><dt>{tr('Rata-rata tayang')}</dt><dd>{m.avg_publish_hrs === null ? tr('Belum ada riwayat') : `${m.avg_publish_hrs.toFixed(1)} ${tr('jam')}`}</dd></div>
							</dl>
							<div class="sc-price">
								<p class="sc-price-head">
									<span>{tr('Harga Anda')}: <strong>{m.price > 0 ? rp(m.price) : '—'}</strong></span>
									{#if m.price_benchmark}
										<span class="sc-pill sc-pill-{m.price_benchmark.position === 'within' ? 'ok' : 'warn'}">{positionText(m.price_benchmark.position)}</span>
									{/if}
								</p>
								{#if m.price_benchmark}
									<p class="sc-muted">
										{tr('Media sebaya')} ({m.price_benchmark.peers}, {basisText(m.price_benchmark.basis)}): {rp(m.price_benchmark.p25)} {tr('sampai')} {rp(m.price_benchmark.p75)}, {tr('median')} {rp(m.price_benchmark.median)}.
									</p>
									<p class="sc-muted">{positionHint(m.price_benchmark.position)}</p>
									<p class="sc-note">{tr('Pembanding harga sebaya, bukan jaminan laku.')}</p>
								{:else if m.price > 0}
									<p class="sc-muted">{tr('Data pembanding belum cukup')}</p>
								{/if}
								{#if m.last_price_change && m.last_price_change.to !== null}
									<p class="sc-note">{tr('Terakhir diubah')} {fmtDate(m.last_price_change.at)}: {m.last_price_change.from === null ? '' : `${rp(m.last_price_change.from)} → `}{rp(m.last_price_change.to)}</p>
								{/if}
							</div>
							<ul class="sc-checks">
								<li class:sc-bad={!m.domain_verified}>
									{#if m.domain_verified}<CircleCheck size={14} strokeWidth={1.5} />{:else}<TriangleAlert size={14} strokeWidth={1.5} />{/if}
									{m.domain_verified ? tr('Domain terverifikasi') : tr('Domain belum terverifikasi')}
								</li>
								<li class:sc-bad={!m.dewan_pers_verified}>
									{#if m.dewan_pers_verified}<CircleCheck size={14} strokeWidth={1.5} />{:else}<TriangleAlert size={14} strokeWidth={1.5} />{/if}
									{m.dewan_pers_verified ? tr('Terverifikasi Dewan Pers') : tr('Belum terverifikasi Dewan Pers')}
								</li>
								{#if m.integration_type && m.integration_type !== 'manual'}
									<li class:sc-bad={!m.has_connection}>
										{#if m.has_connection}<CircleCheck size={14} strokeWidth={1.5} />{:else}<TriangleAlert size={14} strokeWidth={1.5} />{/if}
										{m.has_connection ? tr('Tersambung ke Tauthub') : tr('Belum tersambung ke Tauthub')}
									</li>
								{/if}
								{#if m.domain_health === 'unreachable'}
									<li class="sc-bad"><TriangleAlert size={14} strokeWidth={1.5} />{tr('Situs tidak bisa diakses')}</li>
								{/if}
							</ul>
							<a class="sc-order-link" href={`${base}/media`}>{tr('Kelola media')}</a>
						</article>
					{/each}
				</div>
			{/if}
		</section>
	{/if}
</section>

<style>
	.sc { display: flex; flex-direction: column; gap: 20px; }
	.sc-head { display: flex; flex-wrap: wrap; align-items: flex-start; justify-content: space-between; gap: 12px; }
	.sc-refresh { min-height: 44px; }
	.sc-error { padding: 12px 16px; font-size: 14px; border-color: var(--destructive); color: var(--destructive); }
	.sc-loading { padding: 20px; text-align: center; font-size: 13.5px; color: var(--muted-foreground); }
	.sc-block { display: flex; flex-direction: column; gap: 10px; }
	.sc-block-head { display: flex; flex-wrap: wrap; align-items: baseline; gap: 10px; }
	.sc-h2 { font-size: 16px; font-weight: 700; color: var(--foreground); }
	.sc-count {
		display: inline-flex; align-items: center; justify-content: center; min-width: 22px; height: 22px;
		padding: 0 7px; border-radius: 999px; font-size: 12px; font-weight: 700;
		color: var(--primary); background: color-mix(in srgb, var(--primary) 12%, transparent);
	}
	.sc-muted { font-size: 13px; line-height: 1.5; color: var(--muted-foreground); }
	.sc-mt { margin-top: 10px; }
	.sc-note { margin-top: 12px; font-size: 12px; line-height: 1.5; color: var(--muted-foreground); }

	.sc-todos { padding: 0; list-style: none; overflow: hidden; }
	.sc-todo { display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: 14px; padding: 14px 16px; }
	.sc-todo + .sc-todo { border-top: 1px solid var(--border); }
	.sc-todo-body { min-width: 0; }
	.sc-todo-title { font-size: 14px; font-weight: 650; line-height: 1.45; color: var(--foreground); word-break: break-word; }
	.sc-sev {
		display: inline-flex; align-items: center; border-radius: 999px; padding: 3px 10px;
		font-size: 11.5px; font-weight: 650; white-space: nowrap;
	}
	.sc-sev-urgent { color: var(--destructive); background: color-mix(in srgb, var(--destructive) 12%, transparent); }
	.sc-sev-warning { color: var(--warning); background: var(--warning-soft); }
	.sc-sev-info { color: var(--muted-foreground); background: var(--muted); }
	.sc-action { min-height: 44px; display: inline-flex; align-items: center; gap: 6px; white-space: nowrap; }
	.sc-more { align-self: flex-start; min-height: 44px; padding: 0 4px; font-size: 13px; font-weight: 600; color: var(--primary); }
	.sc-empty { display: flex; align-items: flex-start; gap: 12px; padding: 18px 16px; color: var(--success); }
	.sc-empty-title { font-size: 14.5px; font-weight: 650; color: var(--foreground); }
	.sc-empty .sc-action { margin-top: 10px; }

	.sc-two { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
	.sc-panel { padding: 18px 18px 16px; display: flex; flex-direction: column; }
	.sc-orders { margin-top: 8px; padding: 0; list-style: none; }
	.sc-orders li { display: grid; grid-template-columns: 1fr auto auto; align-items: center; gap: 12px; padding: 10px 0; }
	.sc-orders li + li { border-top: 1px solid var(--border); }
	.sc-order-main { min-width: 0; }
	.sc-remaining { font-size: 12.5px; font-weight: 650; color: var(--warning); white-space: nowrap; }
	.sc-remaining-late { color: var(--destructive); }
	.sc-order-link { display: inline-flex; align-items: center; min-height: 44px; font-size: 13px; font-weight: 600; color: var(--primary); text-decoration: none; }
	.sc-order-link:hover { text-decoration: underline; }

	.sc-facts { margin-top: 12px; display: flex; flex-direction: column; }
	.sc-facts > div { display: flex; justify-content: space-between; gap: 12px; padding: 8px 0; font-size: 13px; }
	.sc-facts > div + div { border-top: 1px solid var(--border); }
	.sc-facts dt { color: var(--muted-foreground); }
	.sc-facts dd { font-weight: 600; color: var(--foreground); text-align: right; font-variant-numeric: tabular-nums; }
	.sc-facts-tight > div { padding: 6px 0; }
	.sc-big { margin-top: 6px; font-size: 28px; font-weight: 750; letter-spacing: -0.015em; color: var(--foreground); font-variant-numeric: tabular-nums; }
	.sc-bars { margin-top: 14px; width: 100%; height: 64px; }
	.sc-bars rect { fill: var(--primary); opacity: 0.85; }

	.sc-media-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 14px; }
	.sc-media { padding: 16px; display: flex; flex-direction: column; }
	.sc-media-head { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; }
	.sc-media-name { min-width: 0; }
	.sc-pill { flex: 0 0 auto; border-radius: 999px; padding: 3px 10px; font-size: 11.5px; font-weight: 650; max-width: 55%; text-align: center; }
	.sc-pill-ok { color: var(--success); background: var(--success-soft); }
	.sc-pill-warn { color: var(--warning); background: var(--warning-soft); }
	.sc-price { margin-top: 10px; padding-top: 10px; border-top: 1px solid var(--border); display: flex; flex-direction: column; gap: 4px; }
	.sc-price-head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; font-size: 13px; color: var(--foreground); }
	.sc-checks { margin: 10px 0 6px; padding: 0; list-style: none; display: flex; flex-direction: column; gap: 5px; }
	.sc-checks li { display: flex; align-items: center; gap: 7px; font-size: 12.5px; color: var(--success); }
	.sc-checks li.sc-bad { color: var(--muted-foreground); }

	.sc-notice { padding: 12px 16px; font-size: 14px; border-color: var(--success); background: var(--success-soft); color: var(--success); }
	.sc-gsc-intro { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 14px; padding: 16px; }
	.sc-gsc-intro > div { flex: 1 1 320px; min-width: 0; }
	.sc-gsc-list { padding: 0; list-style: none; overflow: hidden; }
	.sc-gsc-row { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(0, 1.6fr) minmax(0, 1fr) auto; align-items: center; gap: 16px; padding: 14px 16px; }
	.sc-gsc-row + .sc-gsc-row { border-top: 1px solid var(--border); }
	.sc-gsc-name { min-width: 0; }
	.sc-gsc-msg { grid-column: 2 / 4; }
	.sc-gsc-nums { display: flex; gap: 20px; }
	.sc-gsc-nums dt { font-size: 11.5px; color: var(--muted-foreground); }
	.sc-gsc-nums dd { font-size: 16px; font-weight: 700; color: var(--foreground); font-variant-numeric: tabular-nums; }
	.sc-gsc-trend { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
	.sc-spark { width: 100%; height: 28px; }
	.sc-spark path { fill: none; stroke: var(--primary); stroke-width: 1.8; vector-effect: non-scaling-stroke; }

	@media (max-width: 860px) {
		.sc-two { grid-template-columns: 1fr; }
		.sc-gsc-row { grid-template-columns: 1fr; gap: 10px; }
		.sc-gsc-msg { grid-column: auto; }
		.sc-gsc-row .sc-action { justify-content: center; }
	}
	@media (max-width: 640px) {
		.sc-todo { grid-template-columns: 1fr; gap: 8px; }
		.sc-todo .sc-sev { justify-self: start; }
		.sc-todo .sc-action { justify-content: center; }
		.sc-media-grid { grid-template-columns: 1fr; }
		.sc-pill { max-width: 60%; }
	}
</style>
