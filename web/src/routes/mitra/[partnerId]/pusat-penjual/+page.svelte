<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Pusat Penjual: tempat harian mitra. Urutannya: apa yang paling perlu
	// dikerjakan sekarang, uang yang masuk, pesanan yang menunggu, trafik dari
	// Google, lalu kondisi tiap media. Semua angka dari GET /v1/partner/seller-center
	// (dihitung di server); data yang belum ada tampil kosong, bukan 0.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ArrowRight, CircleCheck, LoaderCircle, RefreshCw, TriangleAlert } from '@lucide/svelte';
	import { getPartnerUrlId } from '$lib/partner-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	type Todo = {
		kind: string;
		severity: 'urgent' | 'warning' | 'info' | 'waiting';
		title: string;
		detail: string;
		route: string;
		query?: string;
		deadline?: string;
		media_id?: string;
	};
	type Waiting = { id: string; document_title: string; media_name: string; deadline: string | null; remaining: string };
	type Interest = { picked: number; brands: number; ordered: number; cancelled: number; picked_total: number };
	type MediaCard = {
		id: string;
		name: string;
		website_url: string | null;
		status: string;
		catalog_visible: boolean;
		catalog_hidden_reason: string | null;
		interest: Interest;
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

	// Tugas yang sejenis (mis. dua media sama-sama belum tersambung) digabung
	// jadi satu baris supaya daftar tidak terbaca seperti pengulangan.
	type Group = {
		key: string;
		severity: Todo['severity'];
		title: string;
		detail: string;
		route: string;
		query?: string;
		names: string[];
	};

	const partnerId = $derived(page.params.partnerId as string);
	const base = $derived(`/mitra/${getPartnerUrlId() || partnerId}`);

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state('');
	let data = $state<Data | null>(null);
	let showAllRest = $state(false);
	// Favicon situs lewat layanan favicon publik Google, seperti di halaman Penghasilan.
	// Gagal dimuat atau domain kosong: tampil huruf pertama nama media.
	let faviconFailed = $state<Record<string, boolean>>({});

	// Search Console: dimuat terpisah supaya galatnya tidak menutup halaman utama.
	let traffic = $state<Traffic | null>(null);
	let trafficError = $state('');
	let trafficNotice = $state('');
	let connecting = $state(false);
	let pullingId = $state<string | null>(null);

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
	const fmtDate = (v: string | null | undefined) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '-');
	const host = (u: string | null) => (u ?? '').replace(/^https?:\/\//, '').replace(/^www\./, '').replace(/\/.*$/, '');
	const faviconSrc = (u: string | null) => {
		const h = host(u);
		return h ? `https://www.google.com/s2/favicons?domain=${encodeURIComponent(h)}&sz=64` : '';
	};
	const go = (g: { route: string; query?: string }) => `${base}/${g.route}${g.query ? `?${g.query}` : ''}`;

	const severityLabel = (s: Todo['severity']) =>
		s === 'urgent' ? tr('Mendesak') : s === 'warning' ? tr('Perlu perhatian') : s === 'waiting' ? tr('Sedang ditinjau') : tr('Saran');

	const groups = $derived.by<Group[]>(() => {
		if (!data) return [];
		const names = new Map(data.media.map((m) => [m.id, m.name]));
		const out: Group[] = [];
		const index = new Map<string, Group & { suffix: string }>();
		for (const t of data.todos) {
			const name = t.media_id ? names.get(t.media_id) : undefined;
			if (!name || !t.title.startsWith(name)) {
				out.push({ key: `${t.kind}|${t.title}|${t.query ?? ''}`, severity: t.severity, title: t.title, detail: t.detail, route: t.route, query: t.query, names: [] });
				continue;
			}
			const suffix = t.title.slice(name.length).trim();
			const key = `${t.kind}|${t.route}|${suffix}`;
			let g = index.get(key);
			if (!g) {
				g = { key, severity: t.severity, title: '', detail: t.detail, route: t.route, query: t.query, names: [], suffix };
				index.set(key, g);
				out.push(g);
			}
			g.names.push(name);
		}
		for (const g of index.values()) {
			g.title = g.names.length === 1 ? `${g.names[0]} ${g.suffix}` : `${g.names.length} media ${g.suffix}`;
		}
		return out;
	});

	// "Sedang ditinjau" bukan tugas mitra (menunggu tim Tauthub), jadi tidak ikut
	// hitungan "perlu dikerjakan" dan tidak pernah jadi sorotan utama.
	const actionable = $derived(groups.filter((g) => g.severity !== 'waiting'));
	const waitingGroups = $derived(groups.filter((g) => g.severity === 'waiting'));
	const focus = $derived(actionable[0] ?? null);
	const rest = $derived([...actionable.slice(1), ...waitingGroups]);
	const shownRest = $derived(showAllRest ? rest : rest.slice(0, 4));
	const urgentCount = $derived(groups.filter((g) => g.severity === 'urgent').length);

	const headline = $derived.by(() => {
		if (!data) return '';
		if (data.summary.media_count === 0) return tr('Belum ada media');
		if (actionable.length === 0) return tr('Semua beres');
		return urgentCount > 0 ? `${urgentCount} ${tr('hal mendesak')}` : `${actionable.length} ${tr('hal perlu dikerjakan')}`;
	});
	const subline = $derived.by(() => {
		if (!data) return '';
		if (data.summary.media_count === 0) return tr('Daftarkan media pertama Anda agar klien bisa memesan artikel.');
		if (actionable.length === 0) return waitingGroups.length > 0 ? tr('Tinggal menunggu tinjauan tim Tauthub.') : tr('Semua media siap menerima pesanan.');
		return urgentCount > 0 ? tr('Kerjakan ini dulu, sebelum batas waktunya lewat.') : tr('Tidak ada yang mendesak.');
	});

	const hiddenReasonText = (r: string | null) =>
		r === 'not_connected'
			? tr('Belum tersambung')
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
</script>

<PageTitle title={tr('Pusat Penjual')} />

<section class="sc">
	<header class="sc-top">
		<div>
			<h1 class="page-title">{tr('Pusat Penjual')}</h1>
			<p class="page-desc">{tr('Tugas hari ini, pesanan, penghasilan, dan kondisi media Anda.')}</p>
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
		<!-- Hari ini: satu hal terpenting di kiri, uang di kanan -->
		<div class="sc-hero">
			<section class="sc-today" aria-labelledby="sc-today-title">
				<p class="sc-kicker">{tr('Hari ini')}</p>
				<h2 id="sc-today-title" class="sc-headline">{headline}</h2>
				<p class="sc-sub">{subline}</p>

				{#if focus}
					<div class="sc-focus">
						<span class="sc-sev sc-sev-{focus.severity}">{severityLabel(focus.severity)}</span>
						<p class="sc-focus-title">{focus.title}</p>
						<p class="sc-focus-detail">{focus.detail}</p>
						{#if focus.names.length > 1}
							<ul class="sc-chips">
								{#each focus.names.slice(0, 4) as n (n)}<li>{n}</li>{/each}
								{#if focus.names.length > 4}<li>+{focus.names.length - 4}</li>{/if}
							</ul>
						{/if}
						<a class="app-btn app-btn-primary sc-cta" href={go(focus)}>{tr('Kerjakan sekarang')}<ArrowRight size={16} /></a>
					</div>
				{:else if data.summary.media_count === 0}
					<a class="app-btn app-btn-primary sc-cta sc-cta-solo" href={`${base}/media`}>{tr('Daftar Media')}<ArrowRight size={16} /></a>
				{/if}

				{#if data.summary.media_count > 0}
					<div class="sc-ready">
						<div class="sc-ready-bar" role="img" aria-label={`${data.summary.visible_count} ${tr('dari')} ${data.summary.media_count} ${tr('media tampil di katalog klien')}`}>
							{#each data.media as m (m.id)}
								<span class:on={m.catalog_visible} title={m.name}></span>
							{/each}
						</div>
						<p class="sc-muted">
							<strong>{data.summary.visible_count}</strong> {tr('dari')} {data.summary.media_count} {tr('media tampil di katalog klien')}
						</p>
					</div>
				{/if}
			</section>

			<section class="sc-money" aria-labelledby="sc-earn-title">
				<h2 id="sc-earn-title" class="sc-money-title">{tr('Bisa ditarik sekarang')}</h2>
				<p class="sc-money-big">{rp(data.earnings.available)}</p>
				<svg class="sc-bars" viewBox="0 0 300 60" preserveAspectRatio="none" role="img"
					aria-label={tr('Penghasilan per hari, 30 hari terakhir')}>
					{#each bars as b (b.s.day)}
						<rect x={b.x} y={b.y} width={b.w} height={b.h} rx="1">
							<title>{b.s.day}: {rp(b.s.amount)}</title>
						</rect>
					{/each}
				</svg>
				<dl class="sc-money-facts">
					<div><dt>{tr('30 hari terakhir')}</dt><dd>{rp(data.earnings.last_30_days)}</dd></div>
					<div><dt>{tr('Masa tahan')} {data.earnings.hold_hours} {tr('jam')}</dt><dd>{rp(data.earnings.pending)}</dd></div>
					<div><dt>{tr('Sudah dicairkan')}</dt><dd>{rp(data.earnings.paid)}</dd></div>
				</dl>
				<a class="sc-money-link" href={`${base}/earnings`}>{tr('Rincian dan tarik saldo')}<ArrowRight size={15} /></a>
			</section>
		</div>

		<!-- Berikutnya -->
		{#if rest.length > 0}
			<section class="sc-block" aria-labelledby="sc-next-title">
				<div class="sc-block-head">
					<h2 id="sc-next-title" class="sc-h2">{tr('Berikutnya')}</h2>
					<span class="sc-count">{rest.length}</span>
				</div>
				<ul class="app-card sc-rest">
					{#each shownRest as g (g.key)}
						<li>
							<a class="sc-rest-row" href={go(g)}>
								<span class="sc-sev sc-sev-{g.severity}">{severityLabel(g.severity)}</span>
								<span class="sc-rest-body">
									<span class="sc-rest-title">{g.title}</span>
									<span class="sc-muted">{g.detail}</span>
									{#if g.names.length > 1}
										<span class="sc-chips">
											{#each g.names.slice(0, 4) as n (n)}<span class="sc-chip">{n}</span>{/each}
											{#if g.names.length > 4}<span class="sc-chip">+{g.names.length - 4}</span>{/if}
										</span>
									{/if}
								</span>
								<ArrowRight size={16} class="sc-rest-arrow" />
							</a>
						</li>
					{/each}
				</ul>
				{#if rest.length > 4}
					<button class="sc-more" type="button" onclick={() => (showAllRest = !showAllRest)}>
						{showAllRest ? tr('Tampilkan lebih sedikit') : `${tr('Lihat semua')} (${rest.length})`}
					</button>
				{/if}
			</section>
		{/if}

		<!-- Pesanan menunggu -->
		<section class="sc-block" aria-labelledby="sc-orders-title">
			<div class="sc-block-head">
				<h2 id="sc-orders-title" class="sc-h2">{tr('Pesanan menunggu persetujuan')}</h2>
				{#if data.pipeline.waiting_total > 0}<span class="sc-count">{data.pipeline.waiting_total}</span>{/if}
			</div>
			<div class="app-card sc-orders-card">
				{#if data.pipeline.waiting.length === 0}
					<p class="sc-calm"><CircleCheck size={18} strokeWidth={1.5} />{tr('Tidak ada pesanan yang menunggu')}</p>
				{:else}
					<ul class="sc-orders">
						{#each data.pipeline.waiting.slice(0, 6) as o (o.id)}
							<li>
								<a class="sc-order-row" href={`${base}/publications?pub=${o.id}`}>
									<span class="sc-order-main">
										<span class="sc-rest-title">{o.document_title}</span>
										<span class="sc-muted">{o.media_name}</span>
									</span>
									<span class="sc-remaining" class:sc-remaining-late={o.remaining === 'sudah lewat'}>{o.remaining}</span>
								</a>
							</li>
						{/each}
					</ul>
					{#if data.pipeline.waiting_total > 6}
						<p class="sc-muted sc-pad">{tr('Dan')} {data.pipeline.waiting_total - 6} {tr('pesanan lain.')}</p>
					{/if}
				{/if}
				<p class="sc-note sc-pad">{tr('Pesanan yang tidak disetujui dalam')} {data.pipeline.sla_hours} {tr('jam dibatalkan otomatis.')}</p>
			</div>
			<dl class="sc-tiles">
				<div class="app-card sc-tile">
					<dt>{tr('Rata-rata waktu tayang')}</dt>
					<dd class:sc-nil={data.pipeline.avg_publish_hours === null}>
						{data.pipeline.avg_publish_hours === null ? '-' : data.pipeline.avg_publish_hours.toFixed(1)}
						<small>{data.pipeline.avg_publish_hours === null ? tr('Belum ada data') : tr('jam')}</small>
					</dd>
				</div>
				<div class="app-card sc-tile">
					<dt>{tr('Pesanan yang Anda tolak')}</dt>
					<dd class:sc-nil={data.pipeline.rejection_rate === null}>
						{data.pipeline.rejection_rate === null ? '-' : `${Math.round(data.pipeline.rejection_rate * 100)}%`}
						{#if data.pipeline.rejection_rate === null}<small>{tr('Belum ada data')}</small>{/if}
					</dd>
				</div>
				<div class="app-card sc-tile">
					<dt>{tr('Dibatalkan karena tidak dibalas (30 hari)')}</dt>
					<dd class:sc-bad-num={data.pipeline.unresponsive_cancelled > 0}>{data.pipeline.unresponsive_cancelled}</dd>
				</div>
			</dl>
		</section>

		<!-- Trafik dari Google (Search Console) -->
		<section class="sc-block" aria-labelledby="sc-gsc-title">
			<div class="sc-block-head">
				<h2 id="sc-gsc-title" class="sc-h2">{tr('Trafik dari Google')}</h2>
				<span class="sc-muted">{tr('Search Console, 28 hari terakhir')}</span>
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
						<p class="sc-rest-title">{tr('Tunjukkan trafik situs Anda ke klien')}</p>
						<p class="sc-muted">{tr('Hubungkan Google Search Console untuk melihat klik, tayangan, dan posisi situs di Google. Aksesnya hanya baca dan bisa Anda cabut kapan saja dari akun Google.')}</p>
					</div>
					<button class="app-btn app-btn-soft sc-action" type="button" disabled={connecting} onclick={() => void connectGsc()}>
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
								{#if faviconSrc(t.website_url) && !faviconFailed[t.media_id]}
									<img class="sc-fav" src={faviconSrc(t.website_url)} alt="" width="32" height="32" loading="lazy"
										onerror={() => (faviconFailed = { ...faviconFailed, [t.media_id]: true })} />
								{:else}
									<span class="sc-fav sc-fav-letter" aria-hidden="true">{t.name.charAt(0).toUpperCase()}</span>
								{/if}
								<div class="sc-id-text">
									<p class="sc-rest-title">{t.name}</p>
									<p class="sc-muted">{host(t.website_url) || '-'}</p>
								</div>
							</div>
							{#if !t.matched}
								<p class="sc-muted sc-gsc-msg">{tr('Situs ini belum terdaftar di Search Console akun Anda. Tambahkan dan verifikasi di sana, lalu hubungkan ulang.')}</p>
							{:else if !last}
								<p class="sc-muted sc-gsc-msg">{tr('Belum ada data. Klik Ambil data untuk mengambil yang pertama.')}</p>
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
				<p class="sc-note">{tr('Search Console hanya memberi klik, tayangan, dan posisi, bukan status indeks tiap halaman. Data diambil otomatis seminggu sekali.')}</p>
			{/if}
		</section>

		<!-- Minat klien -->
		<section class="sc-block" aria-labelledby="sc-media-title">
			<div class="sc-block-head">
				<h2 id="sc-media-title" class="sc-h2">{tr('Minat klien')}</h2>
				<span class="sc-muted">{tr('Dihitung dari kampanye klien, 30 hari terakhir')}</span>
			</div>
			{#if data.media.length === 0}
				<div class="app-card sc-empty"><p class="sc-muted">{tr('Belum ada media.')}</p></div>
			{:else}
				<ul class="app-card sc-interest">
					{#each data.media as m (m.id)}
						{@const it = m.interest}
						<li class="sc-interest-row">
							<div class="sc-gsc-name">
								{#if faviconSrc(m.website_url) && !faviconFailed[m.id]}
									<img class="sc-fav" src={faviconSrc(m.website_url)} alt="" width="32" height="32" loading="lazy"
										onerror={() => (faviconFailed = { ...faviconFailed, [m.id]: true })} />
								{:else}
									<span class="sc-fav sc-fav-letter" aria-hidden="true">{m.name.charAt(0).toUpperCase()}</span>
								{/if}
								<div class="sc-id-text">
									<p class="sc-rest-title">{m.name}</p>
									<p class="sc-muted">{host(m.website_url) || '-'}</p>
								</div>
							</div>

							{#if !m.catalog_visible}
								<p class="sc-muted sc-interest-msg">
									<span class="sc-pill sc-pill-warn">{hiddenReasonText(m.catalog_hidden_reason)}</span>
									{tr('Klien belum bisa memilih media ini.')}
								</p>
							{:else if it.picked === 0}
								<p class="sc-muted sc-interest-msg">
									{tr('Belum ada klien yang memilih media ini dalam 30 hari terakhir.')}
									{#if it.picked_total > 0}{tr('Sepanjang waktu')}: {it.picked_total}.{/if}
								</p>
							{:else}
								<dl class="sc-funnel">
									<div><dt>{tr('Dipilih klien')}</dt><dd>{it.picked}<small>{it.brands} {tr('brand')}</small></dd></div>
									<div><dt>{tr('Jadi pesanan')}</dt><dd>{it.ordered}</dd></div>
									<div><dt>{tr('Dibatalkan')}</dt><dd class:sc-nil={it.cancelled === 0}>{it.cancelled}</dd></div>
								</dl>
								<div class="sc-conv" role="img" aria-label={`${it.ordered} ${tr('dari')} ${it.picked} ${tr('jadi pesanan')}`}>
									<span style="width: {Math.min(100, (it.ordered / it.picked) * 100)}%"></span>
								</div>
							{/if}

							<a class="sc-link" href={`${base}/media`}>{tr('Kelola media')}<ArrowRight size={15} /></a>
						</li>
					{/each}
				</ul>
				<p class="sc-note">{tr('Jumlah tayang dan penanda bookmark di katalog belum tercatat, jadi tidak ditampilkan di sini.')}</p>
			{/if}
		</section>
	{/if}
</section>

<style>
	.sc { display: flex; flex-direction: column; gap: 32px; }
	.sc-top { display: flex; flex-wrap: wrap; align-items: flex-start; justify-content: space-between; gap: 12px; }
	.sc-refresh { min-height: 44px; }
	.sc-error { padding: 12px 16px; font-size: 14px; border-color: var(--destructive); color: var(--destructive); }
	.sc-notice { padding: 12px 16px; font-size: 14px; border-color: var(--success); background: var(--success-soft); color: var(--success); }
	.sc-loading { padding: 20px; text-align: center; font-size: 14px; color: var(--muted-foreground); }
	.sc-muted { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.sc-note { font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	.sc-pad { padding: 0 20px 16px; }
	.sc-block { display: flex; flex-direction: column; gap: 12px; }
	.sc-block-head { display: flex; flex-wrap: wrap; align-items: baseline; gap: 4px 12px; }
	.sc-h2 { font-size: 18px; line-height: 24px; font-weight: 600; color: var(--foreground); }
	.sc-count {
		display: inline-flex; align-items: center; justify-content: center; min-width: 24px; height: 24px;
		padding: 0 8px; border-radius: 999px; font-size: 12px; font-weight: 600;
		color: var(--primary); background: color-mix(in srgb, var(--primary) 12%, transparent);
	}
	.sc-sev {
		display: inline-flex; align-items: center; align-self: flex-start; border-radius: 999px; padding: 4px 12px;
		font-size: 12px; line-height: 16px; font-weight: 500; white-space: nowrap;
	}
	.sc-sev-urgent { color: var(--destructive); background: color-mix(in srgb, var(--destructive) 12%, transparent); }
	.sc-sev-warning { color: var(--warning); background: var(--warning-soft); }
	.sc-sev-info { color: var(--muted-foreground); background: var(--muted); }
	.sc-sev-waiting { color: var(--primary); background: color-mix(in srgb, var(--primary) 12%, transparent); }
	.sc-action { min-height: 44px; display: inline-flex; align-items: center; justify-content: center; gap: 8px; white-space: nowrap; }
	.sc-link { display: inline-flex; align-items: center; gap: 8px; min-height: 44px; margin-top: auto; font-size: 13px; font-weight: 500; color: var(--primary); text-decoration: none; }
	.sc-link:hover { text-decoration: underline; }
	.sc-calm { display: flex; align-items: center; gap: 8px; padding: 16px 20px; font-size: 14px; font-weight: 500; color: var(--success); }
	.sc-chips { display: flex; flex-wrap: wrap; gap: 8px; padding: 0; margin: 8px 0 0; list-style: none; }
	.sc-chips li, .sc-chip {
		display: inline-flex; align-items: center; border-radius: 8px; padding: 4px 8px; font-size: 12px; line-height: 16px; font-weight: 500;
		color: var(--foreground); background: var(--muted);
	}
	.sc-nil { color: var(--muted-foreground) !important; font-weight: 400 !important; }

	/* ---- Hari ini ---- */
	.sc-hero { display: grid; grid-template-columns: minmax(0, 1.5fr) minmax(0, 1fr); gap: 24px; align-items: stretch; }
	.sc-today { display: flex; flex-direction: column; gap: 4px; padding: 4px 0 0; }
	.sc-kicker { font-size: 13px; line-height: 20px; font-weight: 500; color: var(--muted-foreground); }
	.sc-headline { font-size: clamp(32px, 4vw, 44px); font-weight: 600; letter-spacing: -0.02em; line-height: 1.1; color: var(--foreground); }
	.sc-sub { font-size: 15px; line-height: 24px; color: var(--muted-foreground); }
	.sc-focus {
		display: flex; flex-direction: column; gap: 8px; margin-top: 20px; padding: 20px 24px 24px;
		border: 1px solid var(--border); border-radius: 16px; background: var(--card, #fff);
	}
	.sc-focus-title { font-size: 18px; font-weight: 600; line-height: 26px; color: var(--foreground); word-break: break-word; }
	.sc-focus-detail { font-size: 14px; line-height: 22px; color: var(--muted-foreground); }
	.sc-cta { align-self: flex-start; min-height: 48px; margin-top: 8px; display: inline-flex; align-items: center; gap: 8px; padding: 0 20px; font-weight: 500; }
	.sc-cta-solo { margin-top: 16px; }
	.sc-ready { display: flex; flex-direction: column; gap: 8px; margin-top: 24px; }
	.sc-ready-bar { display: flex; gap: 4px; height: 8px; }
	.sc-ready-bar span { flex: 1 1 0; min-width: 8px; max-width: 64px; border-radius: 999px; background: var(--border); }
	.sc-ready-bar span.on { background: var(--success); }

	.sc-money {
		display: flex; flex-direction: column; padding: 24px; border-radius: 20px;
		color: var(--primary-foreground, #fff); background: var(--primary);
	}
	.sc-money-title { font-size: 13px; line-height: 20px; font-weight: 500; opacity: 0.85; }
	.sc-money-big { margin-top: 4px; font-size: clamp(32px, 3.6vw, 40px); font-weight: 600; letter-spacing: -0.02em; line-height: 1.15; font-variant-numeric: tabular-nums; }
	.sc-bars { margin-top: 16px; width: 100%; height: 64px; }
	.sc-bars rect { fill: currentColor; opacity: 0.8; }
	.sc-money-facts { margin-top: 16px; display: flex; flex-direction: column; }
	.sc-money-facts > div { display: flex; justify-content: space-between; gap: 12px; padding: 8px 0; font-size: 13px; line-height: 20px; border-top: 1px solid color-mix(in srgb, currentColor 22%, transparent); }
	.sc-money-facts dt { opacity: 0.8; }
	.sc-money-facts dd { font-weight: 500; font-variant-numeric: tabular-nums; text-align: right; }
	.sc-money-link { display: inline-flex; align-items: center; gap: 8px; min-height: 44px; margin-top: 8px; font-size: 13px; font-weight: 500; color: inherit; text-decoration: none; }
	.sc-money-link:hover { text-decoration: underline; }

	/* ---- Berikutnya ---- */
	.sc-rest { padding: 0; list-style: none; overflow: hidden; }
	.sc-rest li + li { border-top: 1px solid var(--border); }
	.sc-rest-row { display: grid; grid-template-columns: 112px minmax(0, 1fr) auto; align-items: start; gap: 16px; padding: 16px 20px; color: inherit; text-decoration: none; transition: background-color 140ms ease; }
	.sc-rest-row:hover { background: color-mix(in srgb, var(--secondary) 60%, transparent); }
	.sc-rest-body { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
	.sc-rest-title { font-size: 14px; font-weight: 600; line-height: 22px; color: var(--foreground); word-break: break-word; }
	:global(.sc-rest-arrow) { margin-top: 4px; color: var(--muted-foreground); transition: transform 140ms ease; }
	.sc-rest-row:hover :global(.sc-rest-arrow) { transform: translateX(4px); }
	.sc-more { align-self: flex-start; min-height: 44px; padding: 0 4px; font-size: 13px; font-weight: 500; color: var(--primary); }

	/* ---- Pesanan ---- */
	.sc-orders-card { padding: 0; overflow: hidden; }
	.sc-orders { padding: 0; list-style: none; }
	.sc-orders li + li { border-top: 1px solid var(--border); }
	.sc-order-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 16px; padding: 16px 20px; color: inherit; text-decoration: none; transition: background-color 140ms ease; }
	.sc-order-row:hover { background: color-mix(in srgb, var(--secondary) 60%, transparent); }
	.sc-order-main { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
	.sc-remaining { padding: 4px 12px; border-radius: 999px; font-size: 12px; line-height: 16px; font-weight: 500; white-space: nowrap; color: var(--warning); background: var(--warning-soft); }
	.sc-remaining-late { color: var(--destructive); background: color-mix(in srgb, var(--destructive) 12%, transparent); }
	.sc-tiles { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
	.sc-tile { padding: 16px 20px; display: flex; flex-direction: column; gap: 4px; }
	.sc-tile dt { font-size: 13px; line-height: 20px; color: var(--muted-foreground); }
	.sc-tile dd { display: flex; align-items: baseline; gap: 8px; font-size: 28px; line-height: 36px; font-weight: 600; letter-spacing: -0.02em; color: var(--foreground); font-variant-numeric: tabular-nums; }
	.sc-tile dd small { font-size: 12px; font-weight: 400; letter-spacing: 0; color: var(--muted-foreground); }
	.sc-bad-num { color: var(--destructive) !important; }

	/* ---- Trafik ---- */
	.sc-gsc-intro { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 16px; padding: 20px; }
	.sc-gsc-intro > div { flex: 1 1 320px; min-width: 0; }
	.sc-gsc-list { padding: 0; list-style: none; overflow: hidden; }
	.sc-gsc-row { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(0, 1.6fr) minmax(0, 1fr) auto; align-items: center; gap: 16px; padding: 16px 20px; }
	.sc-gsc-row + .sc-gsc-row { border-top: 1px solid var(--border); }
	.sc-gsc-name { display: flex; align-items: center; gap: 12px; min-width: 0; }
	.sc-gsc-msg { grid-column: 2 / 4; }
	.sc-gsc-nums { display: flex; gap: 24px; }
	.sc-gsc-nums dt { font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	.sc-gsc-nums dd { font-size: 20px; line-height: 28px; font-weight: 600; letter-spacing: -0.01em; color: var(--foreground); font-variant-numeric: tabular-nums; }
	.sc-gsc-trend { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
	.sc-spark { width: 100%; height: 28px; }
	.sc-spark path { fill: none; stroke: var(--primary); stroke-width: 1.8; vector-effect: non-scaling-stroke; }

	/* ---- Minat klien ---- */
	.sc-interest { padding: 0; list-style: none; overflow: hidden; }
	.sc-interest-row { display: grid; grid-template-columns: minmax(0, 1.2fr) minmax(0, 2fr) auto; align-items: center; gap: 16px; padding: 16px 20px; }
	.sc-interest-row + .sc-interest-row { border-top: 1px solid var(--border); }
	.sc-interest-row > .sc-link { margin-top: 0; }
	.sc-interest-msg { grid-column: 2 / 3; display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
	.sc-id-text { min-width: 0; }
	.sc-fav {
		flex: 0 0 auto; width: 32px; height: 32px; border-radius: 8px; object-fit: contain;
		border: 1px solid var(--border); background: var(--card, #fff); padding: 4px;
	}
	.sc-fav-letter {
		display: inline-flex; align-items: center; justify-content: center; padding: 0;
		font-size: 14px; font-weight: 600; color: var(--muted-foreground); background: var(--muted);
	}
	.sc-pill { flex: 0 0 auto; border-radius: 999px; padding: 4px 12px; font-size: 12px; line-height: 16px; font-weight: 500; text-align: center; }
	.sc-pill-warn { color: var(--warning); background: var(--warning-soft); }
	.sc-funnel { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
	.sc-funnel dt { font-size: 12px; line-height: 16px; color: var(--muted-foreground); }
	.sc-funnel dd { display: flex; align-items: baseline; gap: 8px; margin-top: 2px; font-size: 22px; line-height: 28px; font-weight: 600; letter-spacing: -0.01em; color: var(--foreground); font-variant-numeric: tabular-nums; }
	.sc-funnel dd small { font-size: 12px; font-weight: 400; letter-spacing: 0; color: var(--muted-foreground); }
	.sc-conv { grid-column: 2 / 3; height: 4px; margin-top: -8px; border-radius: 999px; background: var(--muted); overflow: hidden; }
	.sc-conv span { display: block; height: 100%; border-radius: 999px; background: var(--success); }
	.sc-empty { padding: 20px; }

	@media (max-width: 960px) {
		.sc-hero { grid-template-columns: 1fr; }
		.sc-gsc-row { grid-template-columns: 1fr; gap: 12px; }
		.sc-gsc-msg { grid-column: auto; }
		.sc-gsc-row .sc-action { width: 100%; }
		.sc-interest-row { grid-template-columns: 1fr; gap: 12px; }
		.sc-interest-msg, .sc-conv { grid-column: auto; }
		.sc-conv { margin-top: 0; }
	}
	@media (max-width: 720px) {
		.sc { gap: 28px; }
		.sc-tiles { grid-template-columns: 1fr; gap: 12px; }
		.sc-cta { align-self: stretch; justify-content: center; }
		.sc-rest-row { grid-template-columns: minmax(0, 1fr) auto; gap: 8px 12px; }
		.sc-rest-row .sc-sev { grid-column: 1 / -1; }
		.sc-money { padding: 20px; }
		.sc-focus { padding: 16px 20px 20px; }
	}

	/* Masuk sekali saat halaman dibuka; tidak ada gerak karena gulir. */
	@media (prefers-reduced-motion: no-preference) {
		.sc-today, .sc-money { animation: sc-in 420ms cubic-bezier(0.16, 1, 0.3, 1) both; }
		.sc-money { animation-delay: 70ms; }
		@keyframes sc-in { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: none; } }
	}
	@media (prefers-reduced-motion: reduce) {
		.sc-rest-row, .sc-order-row, :global(.sc-rest-arrow) { transition: none; }
	}
</style>
