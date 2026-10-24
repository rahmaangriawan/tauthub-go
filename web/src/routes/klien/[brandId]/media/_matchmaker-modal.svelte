<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// AI Media Matchmaker — dua tahap:
	// 1. Brief bebas diinterpretasi AI (agent berversi Taut Brief) menjadi
	//    ringkasan terstruktur yang bisa dikoreksi.
	// 2. Server mencocokkan kriteria itu dengan katalog NYATA (harga, DA,
	//    traffic, aturan editorial) dan menyusun paket.
	// Modal ini tidak membuat kampanye/pesanan; "Terapkan" hanya mencentang
	// media di katalog.
	import {
		ArrowRight, BarChart3, Check, CircleAlert, CircleDollarSign, ClipboardList, Clock, FileText, Info, Layers,
		LoaderCircle, Pencil, Plus, ShieldCheck, Sparkles, Target, Wallet, X, Paperclip, TriangleAlert
	} from '@lucide/svelte';
	import { apiRequest, readApiError } from '$lib/auth';
	import { mediaDomainLabel } from '$lib/media/domain-label';
	import Select from '$lib/components/select.svelte';

	type Criteria = {
		objective: string;
		audience: string;
		categories: string[];
		media_types: string[];
		count: number;
		budget_total: number;
		budget_min: number;
		budget_max: number;
		min_da: number;
		min_ai_trust: number;
		min_traffic: number;
		min_words: number;
		min_backlinks: number;
		require_hyperlink: boolean;
		require_permanent_link: boolean;
		languages: string[];
		exclude_topics: string[];
		notes: string;
	};

	type Interpretation = {
		summary: string;
		provenance: Record<string, string>;
		assumptions: string[];
		unknowns: string[];
	};

	type Candidate = {
		id: string;
		name: string;
		website_url: string | null;
		category: string | null;
		media_type: string | null;
		price: number;
		client_price: number;
		credit_cost: number;
		media_tier: string | null;
		health_score: number | null;
		seo_da: number | null;
		seo_ai_trust: number | null;
		traffic_estimate: number;
		link_duration_months: number;
		allow_hyperlink: boolean;
		max_links: number | null;
		dewan_pers_verified: boolean;
		published_count: number;
		score: number;
		match_label: string;
		reasons: string[];
		warnings: string[];
		in_package: boolean;
	};

	type CompositionItem = { key: string; label: string; count: number; percent: number };

	type Result = {
		interpretation: Interpretation | null;
		criteria: Criteria;
		candidates: Candidate[];
		composition: { total: number; items: CompositionItem[] };
		// agent = paket dipilih agent kedua (media_matchmaker_select),
		// auto = paket disusun skor deterministik server.
		selector: 'agent' | 'auto';
		package: {
			count: number;
			total: number;
			budget: number;
			remaining: number;
			summary: string;
			by_category: Record<string, number>;
			by_type: Record<string, number>;
			unmet: string[];
		};
		pool_size: number;
		warnings: string[];
	};

	type Props = {
		apiBase: string;
		categories: string[];
		detailIds: Set<string>;
		hasSelection: boolean;
		// Modal detail dirender di atas modal ini; Escape di sana hanya boleh
		// menutup detail, bukan keduanya.
		escapeDisabled: boolean;
		onClose: () => void;
		onShowDetail: (id: string) => void;
		onApply: (items: Array<{ id: string; name: string; price: number; credits: number }>) => void;
	};

	let { apiBase, categories, detailIds, hasSelection, escapeDisabled, onClose, onShowDetail, onApply }: Props = $props();

	const BRIEF_MAX = 2000;
	const MIN_ANALYZE_CHARS = 15;
	// Interpretasi brief = panggilan LLM; bisa 30-60 detik. Timeout default
	// apiRequest (12 dtk) selalu memutus panggilan ini lebih dulu.
	const ANALYZE_TIMEOUT_MS = 120_000;
	const SEARCH_TIMEOUT_MS = 60_000;

	let brief = $state('');
	let step = $state<1 | 2>(1);
	let analyzing = $state(false);
	let analyzeError = $state('');
	let interpretation = $state<Interpretation | null>(null);
	let criteria = $state<Criteria | null>(null);
	let result = $state<Result | null>(null);
	let editing = $state(false);
	let searching = $state(false);
	let searchError = $state('');
	let selected = $state<Set<string>>(new Set());
	let lastAnalyzed = $state('');
	let briefEl = $state<HTMLTextAreaElement | null>(null);
	// Hanya respons analisis terbaru yang boleh diterapkan — panggilan lama yang
	// pulang belakangan (user sempat mengetik lagi) tidak menimpa state baru.
	let analyzeSeq = 0;

	const errText = (reason: unknown, fallback: string) =>
		reason instanceof Error && reason.message ? reason.message : fallback;

	const money = (n: number) => 'Rp' + Math.round(n || 0).toLocaleString('id-ID');
	const moneyShort = (n: number) => {
		if (!n) return 'Rp0';
		if (n >= 1_000_000_000) return 'Rp' + (n / 1_000_000_000).toFixed(1).replace('.0', '') + ' M';
		if (n >= 1_000_000) return 'Rp' + (n / 1_000_000).toFixed(1).replace('.0', '') + ' juta';
		if (n >= 1_000) return 'Rp' + Math.round(n / 1_000) + ' rb';
		return money(n);
	};
	const trafficFmt = (n: number) => {
		if (!n || n <= 0) return '—';
		if (n >= 1_000_000) return (n / 1_000_000).toFixed(1).replace('.0', '') + ' jt';
		if (n >= 1_000) return Math.round(n / 1_000) + ' rb';
		return n.toLocaleString('id-ID');
	};

	// Anggaran punya dua makna berbeda: total kampanye (budget_total) dan
	// preferensi harga per media (budget_min/max). Ditampilkan terpisah supaya
	// rentang per-media tidak terbaca sebagai budget kampanye.
	const budgetTotalLabel = $derived.by(() => {
		if (!criteria || criteria.budget_total <= 0) return '';
		return moneyShort(criteria.budget_total);
	});

	const perMediaLabel = $derived.by(() => {
		if (!criteria) return '';
		const mn = criteria.budget_min;
		const mx = criteria.budget_max;
		if (mn > 0 && mx > 0) return `${moneyShort(mn)} – ${moneyShort(mx)} /media`;
		if (mn > 0) return `≥ ${moneyShort(mn)} /media`;
		if (mx > 0) return `≤ ${moneyShort(mx)} /media`;
		return '';
	});

	// Prompt Matchmaker meminta model menuliskan NAMA FIELD ke `unknowns` untuk
	// data yang tidak disebut brief (lihat migrasi 238). Tanpa peta ini daftar
	// itu tampil sebagai key JSON mentah di kartu ringkasan.
	const articleRules = $derived.by(() => {
		if (!criteria) return [] as string[];
		const out: string[] = [];
		if (criteria.min_words > 0) out.push(tr('Minimal') + ' ' + criteria.min_words.toLocaleString('id-ID') + ' ' + tr('kata'));
		if (criteria.min_backlinks > 0) out.push(tr('Minimal') + ' ' + criteria.min_backlinks + ' ' + tr('backlink'));
		return out;
	});

	// Status ringkasan. Daftar unknowns sengaja tidak memicu "Perlu
	// klarifikasi" — daftar itu sudah dihilangkan dari UI klien (terasa
	// seperti PR); detailnya tetap tersimpan untuk admin di folder Log.
	const summaryState = $derived.by<'idle' | 'loading' | 'error' | 'clarify' | 'ok'>(() => {
		if (analyzing) return 'loading';
		if (analyzeError) return 'error';
		if (interpretation) {
			if ((criteria?.count ?? 0) <= 0) return 'clarify';
			return 'ok';
		}
		return 'idle';
	});

	// Tier dengan porsi terbesar — ditampilkan di kanan kepala komposisi.
	const compositionTop = $derived.by(() => {
		const items = result?.composition.items ?? [];
		if (items.length === 0) return null;
		return items.reduce((a, b) => (b.percent > a.percent ? b : a));
	});

	const tierColor = (key: string) => (key === 'tier1' ? '#2b67ec' : key === 'tier2' ? '#9db9ff' : key === 'tier3' ? '#f3b45f' : '#cbd5e1');

	// ── Filter + urut daftar kandidat (tahap 2) ─────────────────────────
	// Daftar panjang (hingga 40 kandidat) dipangkas secara visual lewat chip
	// filter; tidak ada data yang disembunyikan permanen.
	type CandFilter = 'semua' | 'terpilih' | 'rekomendasi' | 'tier1' | 'tier2' | 'tier3';
	type CandSort = 'skor' | 'harga' | 'da';
	let candFilter = $state<CandFilter>('semua');
	let candSort = $state<CandSort>('skor');

	const tierKey = (t: string | null) => (t ?? '').trim().toLowerCase();

	const filterCounts = $derived.by(() => {
		const list = result?.candidates ?? [];
		return {
			semua: list.length,
			terpilih: list.filter((c) => selected.has(c.id)).length,
			rekomendasi: list.filter((c) => c.in_package).length,
			tier1: list.filter((c) => tierKey(c.media_tier) === 'tier1').length,
			tier2: list.filter((c) => tierKey(c.media_tier) === 'tier2').length,
			tier3: list.filter((c) => tierKey(c.media_tier) === 'tier3').length
		};
	});

	const visibleCandidates = $derived.by(() => {
		const list = result?.candidates ?? [];
		let out = list;
		if (candFilter === 'terpilih') out = list.filter((c) => selected.has(c.id));
		else if (candFilter === 'rekomendasi') out = list.filter((c) => c.in_package);
		else if (candFilter.startsWith('tier')) out = list.filter((c) => tierKey(c.media_tier) === candFilter);
		const sorted = [...out];
		if (candSort === 'harga') sorted.sort((a, b) => (a.client_price ?? 0) - (b.client_price ?? 0));
		else if (candSort === 'da') sorted.sort((a, b) => (b.seo_da ?? 0) - (a.seo_da ?? 0));
		// 'skor' = urutan server (paket dulu, lalu skor) — biarkan apa adanya.
		return sorted;
	});

	const selectedItems = $derived((result?.candidates ?? []).filter((c) => selected.has(c.id)));
	const selectedTotal = $derived(selectedItems.reduce((sum, c) => sum + (c.client_price ?? 0), 0));
	const overBudget = $derived(Boolean(result && result.package.budget > 0 && selectedTotal > result.package.budget));

	// Ringkasan TIDAK dibuat otomatis saat mengetik — user menekan "Ringkas
	// Brief". Effect ini hanya membersihkan hasil lama ketika brief dikosongkan
	// di bawah ambang minimal, supaya ringkasan basi tidak tertinggal.
	$effect(() => {
		if (brief.trim().length >= MIN_ANALYZE_CHARS) return;
		criteria = null;
		interpretation = null;
		result = null;
		analyzeError = '';
		lastAnalyzed = '';
		analyzeSeq++;
	});

	async function analyze(text: string) {
		const seq = ++analyzeSeq;
		analyzing = true;
		analyzeError = '';
		// Kosongkan hasil lama di awal supaya panel animasi pemrosesan tampil
		// selama meringkas — bukan spinner di bawah kartu yang basi.
		criteria = null;
		interpretation = null;
		try {
			const res = await apiRequest(
				`${apiBase}/media/matchmaker`,
				{ method: 'POST', body: JSON.stringify({ brief: text }) },
				ANALYZE_TIMEOUT_MS
			);
			if (seq !== analyzeSeq) return;
			if (!res.ok) {
				analyzeError = await readApiError(res, tr('AI Matchmaker gagal memilih media.'));
				criteria = null;
				interpretation = null;
				return;
			}
			const data = (await res.json()) as Result;
			criteria = data.criteria;
			interpretation = data.interpretation;
			result = data;
			selected = new Set(data.candidates.filter((c) => c.in_package).map((c) => c.id));
			lastAnalyzed = text;
			step = 1;
		} catch (reason) {
			if (seq !== analyzeSeq) return;
			analyzeError = errText(reason, tr('AI Matchmaker gagal memilih media.'));
			criteria = null;
			interpretation = null;
		} finally {
			if (seq === analyzeSeq) analyzing = false;
		}
	}

	async function search() {
		searchError = '';
		// Tanpa ringkasan tidak ada kriteria yang bisa dicari; tombolnya pun
		// disabled. Tidak ada panggilan AI implisit di sini.
		if (!criteria) return;
		searching = true;
		try {
			const res = await apiRequest(
				`${apiBase}/media/matchmaker`,
				// summary = ringkasan dari agent pertama; jadi bahan konteks
				// agent pemilih media di server.
				{ method: 'POST', body: JSON.stringify({ criteria, summary: interpretation?.summary ?? '' }) },
				SEARCH_TIMEOUT_MS
			);
			if (!res.ok) {
				searchError = await readApiError(res, tr('AI Matchmaker gagal memilih media.'));
				return;
			}
			const data = (await res.json()) as Result;
			result = data;
			selected = new Set(data.candidates.filter((c) => c.in_package).map((c) => c.id));
			step = 2;
		} catch (reason) {
			searchError = errText(reason, tr('AI Matchmaker gagal memilih media.'));
		} finally {
			searching = false;
		}
	}

	function toggle(id: string) {
		selected = new Set(selected);
		if (selected.has(id)) selected.delete(id);
		else selected.add(id);
	}

	function toggleEdit() {
		// Ringkasan tidak lagi diringkas otomatis, jadi koreksi user tidak bisa
		// tertimpa; lastAnalyzed hanya penanda "brief ini yang terakhir diringkas".
		editing = !editing;
	}

	// Contoh brief yang menutup semua field kriteria (tujuan, audiens, jumlah,
	// kategori yang ada di katalog, anggaran total + per media, DA, AI Trust,
	// trafik, syarat artikel, hyperlink/permanen, bahasa, exclude topics)
	// supaya daftar "Belum disebut di brief" kosong saat diringkas.
	// Dirotasi tiap klik: dasar-murah-sedikit → lengkap → dst.
	const EXAMPLE_BRIEFS = [
		`Kami UMKM katering sehat yang baru buka, mau memperkenalkan brand lewat publikasi artikel sederhana di beberapa media berita. Cukup 3 media dalam bahasa Indonesia dari kategori Berita atau Media Nasional.

Total anggaran kami Rp900.000, dengan harga per media sekitar Rp250.000 - Rp350.000. Syaratnya tidak rumit: DA minimal 20, AI Trust minimal 60, dan estimasi trafik minimal 5.000 kunjungan per bulan.

Setiap artikel minimal 500 kata dengan 1 backlink dofollow permanen. Hindari judul clickbait dan klaim yang berlebihan. Isinya perkenalan layanan katering sehat kami dan manfaat makan sehat setiap hari.`,
		`Kami brand skincare pria Ravencare ingin meningkatkan brand awareness dan SEO lewat artikel rekomendasi dan perbandingan produk, menyasar pria usia 20-35 tahun yang tertarik perawatan wajah. Susunkan 10 media dalam bahasa Indonesia dari kategori Bisnis, Lifestyle, dan Media Nasional.

Anggaran total kampanye Rp7.500.000. Harga per media yang kami inginkan Rp250.000 - Rp750.000. Pilih media dengan DA minimal 25, AI Trust minimal 70, dan estimasi trafik minimal 10.000 kunjungan per bulan.

Setiap artikel minimal 1.000 kata dan wajib mendapat 2 backlink dofollow permanen. Hindari klaim medis yang tidak terbukti, klaim memutihkan secara instan, dan judul yang menjanjikan hasil berlebihan.

Produk yang dibahas: Ravencare Men Facial Wash Daily Clean, Oil Control Moisturizer, dan Sunscreen SPF 50 PA++++, dibandingkan dengan produk sejenis dari Kahf dan Nivea Men.`
	];
	let exampleIdx = $state(0);

	function fillExample() {
		// Rotasi: setiap klik mengganti ke contoh berikutnya, bukan yang sama.
		brief = EXAMPLE_BRIEFS[exampleIdx % EXAMPLE_BRIEFS.length];
		exampleIdx++;
		// Fokus + kursor di akhir supaya user bisa langsung menyesuaikan.
		setTimeout(() => {
			briefEl?.focus();
			briefEl?.setSelectionRange(brief.length, brief.length);
		}, 0);
	}

	// Avatar huruf dengan warna stabil per domain — fallback bila favicon
	// gagal dimuat atau media tidak punya website_url.
	const AVATAR_COLORS = ['#2563eb', '#059669', '#d97706', '#dc2626', '#0f766e', '#475569'];
	function avatarColor(seed: string) {
		let h = 7;
		for (const ch of seed) h = (h * 31 + ch.charCodeAt(0)) % 9973;
		return AVATAR_COLORS[h % AVATAR_COLORS.length];
	}

	// Tahapan tampilan selama meringkas — dekoratif, tanpa progres palsu:
	// tidak ada persentase, dan selesai HANYA ketika response benar-benar datang.
	const PROCESS_STAGES = [
		'Membaca brief kampanye',
		'Mengenali tujuan dan anggaran',
		'Mengekstrak persyaratan media',
		'Menyusun ringkasan kebutuhan'
	];
	let stageIdx = $state(0);
	$effect(() => {
		if (!analyzing) {
			stageIdx = 0;
			return;
		}
		// Hormati prefers-reduced-motion: teks tahap tetap statis.
		if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
		const t = setInterval(() => {
			stageIdx = (stageIdx + 1) % PROCESS_STAGES.length;
		}, 2400);
		return () => clearInterval(t);
	});
	const stageLabel = $derived(PROCESS_STAGES[stageIdx]);

	// Favicon asli dari web media via layanan favicon Google; kalau gagal
	// dimuat (domain mati/tanpa favicon), fallback ke avatar huruf.
	let faviconFailed = $state<Set<string>>(new Set());
	function faviconSrc(url: string | null) {
		if (!url) return '';
		try {
			const host = new URL(url.includes('://') ? url : `https://${url}`).hostname;
			return `https://www.google.com/s2/favicons?domain=${host}&sz=64`;
		} catch {
			return '';
		}
	}

	function addDetail() {
		const suffix = '\n\nDetail tambahan: ';
		if (brief.length + suffix.length > BRIEF_MAX) brief = brief.slice(0, BRIEF_MAX - suffix.length);
		brief = brief + suffix;
		briefEl?.focus();
		briefEl?.setSelectionRange(brief.length, brief.length);
	}

	function apply() {
		onApply(
			selectedItems.map((c) => ({
				id: c.id,
				name: mediaDomainLabel(c.website_url, c.name),
				price: c.client_price ?? c.price ?? 0,
				credits: c.credit_cost ?? 0
			}))
		);
	}

	function close() {
		onClose();
	}
</script>

<svelte:window onkeydown={(e) => { if (e.key === 'Escape' && !escapeDisabled) close(); }} />

<div class="mmOverlay" role="presentation" onclick={close}>
	<dialog open class="mmDialog" aria-label={tr('AI Media Matchmaker')} onclick={(e) => e.stopPropagation()}>
		<!-- Header -->
		<div class="mmHeader">
			<div class="mmHeaderIcon"><img src="/matchmaker-icon.webp" alt="" draggable="false" /></div>
			<div class="mmHeaderText">
				<h2>{tr('AI Media Matchmaker')}</h2>
				<p>{tr('Terjemahkan brief kampanye menjadi rekomendasi media yang relevan')}</p>
			</div>
			<button class="mmClose" type="button" aria-label={tr('Tutup')} onclick={close}><X size={20} /></button>
		</div>

		<!-- Stepper -->
		<div class="mmStepper" aria-label={tr('Langkah')}>
			<div class="mmStep" class:mmStepOn={step === 1}>
				<span class="mmStepDot">1</span>
				<span class="mmStepLabel">{tr('Pahami kebutuhan')}</span>
			</div>
			<span class="mmStepLine" class:mmStepLineOn={step === 2}></span>
			<div class="mmStep" class:mmStepOn={step === 2}>
				<span class="mmStepDot">2</span>
				<span class="mmStepLabel">{tr('Pilih media')}</span>
			</div>
		</div>

		{#if step === 1}
			<div class="mmBody">
				<!-- Kiri: brief -->
				<section class="mmCol">
					<div class="mmColHead">
						<h3 class="mmColTitle">{tr('Ceritakan kebutuhan kampanye')}</h3>
						<button class="mmExampleBtn" type="button" onclick={fillExample} title={tr('Isi textarea dengan contoh brief — klik lagi untuk contoh lain')}>
							<ClipboardList size={14} />{tr('Contoh')}
						</button>
					</div>
					<p class="mmColSub">{tr('Tulis seperti biasa. AI akan merangkum brief kamu sebelum memilih media.')}</p>

					<div class="mmTextWrap">
						<textarea
							bind:this={briefEl}
							class="mmTextarea"
							maxlength={BRIEF_MAX}
							bind:value={brief}
							placeholder={tr('Tempel instruksi dari WhatsApp atau email, atau tulis kebutuhan kampanyemu di sini…')}
						></textarea>
						<span class="mmCounter">{brief.length}/{BRIEF_MAX}</span>
					</div>

					<div class="mmBriefTools">
						<span class="mmAttachIcons" aria-hidden="true"><Paperclip size={15} /></span>
						<span class="mmAttachHint">{tr('Kamu bisa menempelkan brief dari WhatsApp atau email.')}</span>
						<button class="mmLinkBtn" type="button" onclick={addDetail}>
							<Plus size={15} />{tr('Tambah detail')}
						</button>
					</div>

					<!-- Ringkas hanya saat user menekan tombol — tidak otomatis
					     saat mengetik, supaya brief panjang tidak terpotong
					     ringkasannya di tengah jalan. -->
					<div class="mmBriefActions">
						<button
							class="mmBtnPrimary mmSummarize"
							type="button"
							disabled={analyzing || brief.trim().length < MIN_ANALYZE_CHARS}
							onclick={() => void analyze(brief.trim())}
						>
							{#if analyzing}<span class="mmSpin"><LoaderCircle size={15} /></span>{:else}<Sparkles size={15} />{/if}
							{analyzing ? tr('Meringkas…') : interpretation ? tr('Ringkas ulang') : tr('Ringkas Brief')}
						</button>
						{#if interpretation && brief.trim() !== lastAnalyzed}
							<span class="mmStaleHint">{tr('Brief berubah — ringkas ulang untuk memperbarui.')}</span>
						{/if}
					</div>
				</section>

				<!-- Kanan: ringkasan -->
				<section class="mmCol mmColRight">
					<div class="mmRightHead">
						<h3 class="mmColTitle">{tr('Ringkasan kebutuhan')}</h3>
						<span class="mmBadge" class:mmBadgeOk={summaryState === 'ok'} class:mmBadgeWarn={summaryState === 'clarify' || summaryState === 'error'} class:mmBadgeMuted={summaryState === 'idle' || summaryState === 'loading'}>
							{#if summaryState === 'loading'}
								<span class="mmSpin"><LoaderCircle size={13} /></span> {tr('Menganalisis…')}
							{:else if summaryState === 'ok'}
								<Check size={13} /> {tr('Brief terdeteksi')}
							{:else if summaryState === 'clarify'}
								<TriangleAlert size={13} /> {tr('Perlu klarifikasi')}
							{:else if summaryState === 'error'}
								<CircleAlert size={13} /> {tr('Gagal')}
							{:else}
								{tr('Menunggu brief')}
							{/if}
						</span>
						{#if criteria}
							<button class="mmLinkBtn mmEditBtn" type="button" onclick={toggleEdit}>
								<Pencil size={15} />{editing ? tr('Selesai') : tr('Edit ringkasan')}
							</button>
						{/if}
					</div>

					{#if analyzeError}
						<div class="mmErrorBox"><CircleAlert size={15} /> {analyzeError}</div>
					{/if}

					{#if criteria}
						{#key lastAnalyzed}
							<div class="mmCards mmReveal">
								<!-- Jumlah media -->
								<div class="mmCardRow">
									<span class="mmCardIcon"><Layers size={18} /></span>
									<span class="mmCardLabel">{tr('Jumlah media')}</span>
									{#if editing}
										<input class="mmInlineInput" type="number" min="1" max="30" bind:value={criteria.count} />
									{:else}
										<strong class="mmCardValue">{criteria.count} {tr('media')}</strong>
									{/if}
								</div>

							<!-- Anggaran: total kampanye dan harga per media ditampilkan
							     terpisah — brief sering menyebut keduanya sekaligus. -->
							<div class="mmCardRow">
								<span class="mmCardIcon"><Wallet size={18} /></span>
								<span class="mmCardLabel">{tr('Anggaran')}</span>
								{#if editing}
									<span class="mmBudgetEdit">
										<label>
											<span>{tr('Total')}</span>
											<input class="mmInlineInput" type="number" min="0" step="100000" bind:value={criteria.budget_total} placeholder={tr('Total')} />
										</label>
										<label>
											<span>{tr('Per media min')}</span>
											<input class="mmInlineInput" type="number" min="0" step="50000" bind:value={criteria.budget_min} placeholder={tr('Min')} />
										</label>
										<label>
											<span>{tr('Per media maks')}</span>
											<input class="mmInlineInput" type="number" min="0" step="50000" bind:value={criteria.budget_max} placeholder={tr('Maks')} />
										</label>
									</span>
								{:else}
									<span class="mmBudgetCol">
										<strong class="mmCardValue">{budgetTotalLabel || perMediaLabel || tr('Belum disebutkan')}</strong>
										{#if budgetTotalLabel && perMediaLabel}<span class="mmBudgetSub">{perMediaLabel}</span>{/if}
									</span>
								{/if}
							</div>

							<!-- Tujuan publikasi -->
							<div class="mmCardRow">
								<span class="mmCardIcon"><Target size={18} /></span>
								<span class="mmCardLabel">{tr('Tujuan publikasi')}</span>
								{#if editing}
									<input class="mmInlineInput mmInlineWide" type="text" bind:value={criteria.objective} placeholder={tr('Tujuan kampanye')} />
								{:else}
									<strong class="mmCardValue">{criteria.objective || tr('Belum disebutkan')}</strong>
								{/if}
							</div>

							<!-- Komposisi media sengaja TIDAK ada di ringkasan: itu hasil
							     pencocokan katalog di tahap "Pilih media", bukan bagian
							     kebutuhan yang dirangkum AI. -->


							<!-- Syarat artikel -->
							<div class="mmCardBlock">
								<div class="mmCardHead">
									<span class="mmCardIcon"><FileText size={18} /></span>
									<span class="mmCardLabel">{tr('Syarat artikel')}</span>
								</div>
								{#if editing}
									<div class="mmRuleEdit">
										<label>
											<span>{tr('Minimal kata')}</span>
											<input class="mmInlineInput" type="number" min="0" step="100" bind:value={criteria.min_words} />
										</label>
										<label>
											<span>{tr('Minimal backlink')}</span>
											<input class="mmInlineInput" type="number" min="0" max="20" bind:value={criteria.min_backlinks} />
										</label>
									</div>
								{:else if articleRules.length > 0}
									<ul class="mmRules">
										{#each articleRules as rule (rule)}<li>{rule}</li>{/each}
									</ul>
								{:else}
									<p class="mmMuted">{tr('Belum disebutkan dalam brief.')}</p>
								{/if}
							</div>
							</div>
						{/key}

						<div class="mmInfoBox">
							<Info size={16} />
							<span>{tr('Anggaran dan syarat artikel bisa kamu koreksi sebelum pencarian media dimulai.')}</span>
						</div>
					{:else if analyzing}
						<!-- Animasi pemrosesan premium: maskot melayang + skeleton
						     yang meniru struktur kartu hasil + scan line tipis.
						     Mengikuti state `analyzing` yang sebenarnya. -->
						<div class="mmProc">
							<img class="mmMascot" src="/brief-empty-mascot.webp" alt="" draggable="false" />
							<p class="mmProcStatus">
								{tr(PROCESS_STAGES[stageIdx])}<span class="mmProcDots" aria-hidden="true"><i></i><i></i><i></i></span>
							</p>
							<div class="mmSkList" aria-hidden="true">
								{#each [44, 58, 70, 50] as w, i (i)}
									<div class="mmSkRow">
										<span class="mmSkIcon"></span>
										<span class="mmSkBar" style={`width:${w}%`}></span>
										<span class="mmSkVal" style={`width:${18 + i * 4}%`}></span>
									</div>
								{/each}
							</div>
							<span class="mmScan" aria-hidden="true"></span>
						</div>
					{:else}
						<div class="mmEmpty">
							{#if summaryState === 'error'}
								<p>{tr('Ringkasan belum bisa dibuat. Perbaiki brief lalu coba lagi.')}</p>
							{:else}
								<p>{tr('Ringkasan kebutuhan akan muncul di sini setelah kamu menekan Ringkas Brief.')}</p>
							{/if}
						</div>
						<div class="mmInfoBox">
							<Info size={16} />
							<span>{tr('Komposisi dan syarat bisa kamu koreksi sebelum pencarian media dimulai.')}</span>
						</div>
					{/if}

					{#if searchError}<div class="mmErrorBox"><CircleAlert size={15} /> {searchError}</div>{/if}
				</section>
			</div>
		{:else}
			<!-- Tahap 2: hasil pencarian — daftar kandidat (kiri) + ringkasan paket (kanan). -->
			<div class="mmBody mmBodyResults">
				<section class="mmCol mmColLeft">
					{#if result && result.candidates.length > 0}
						<!-- Chip filter + urut: memangkas daftar panjang tanpa
						     menyembunyikan data — semua kandidat tetap satu klik jauh. -->
						<div class="mmFilterBar">
							<div class="mmChips" role="group" aria-label={tr('Filter kandidat')}>
								<button class="mmChip" class:mmChipOn={candFilter === 'semua'} type="button" onclick={() => (candFilter = 'semua')}>
									{tr('Semua')}<em>{filterCounts.semua}</em>
								</button>
								<button class="mmChip" class:mmChipOn={candFilter === 'terpilih'} type="button" onclick={() => (candFilter = 'terpilih')}>
									{tr('Terpilih')}<em>{filterCounts.terpilih}</em>
								</button>
								<button class="mmChip" class:mmChipOn={candFilter === 'rekomendasi'} type="button" onclick={() => (candFilter = 'rekomendasi')}>
									{tr('Rekomendasi')}<em>{filterCounts.rekomendasi}</em>
								</button>
								{#if filterCounts.tier1 > 0}
									<button class="mmChip" class:mmChipOn={candFilter === 'tier1'} type="button" onclick={() => (candFilter = 'tier1')}>
										Tier 1<em>{filterCounts.tier1}</em>
									</button>
								{/if}
								{#if filterCounts.tier2 > 0}
									<button class="mmChip" class:mmChipOn={candFilter === 'tier2'} type="button" onclick={() => (candFilter = 'tier2')}>
										Tier 2<em>{filterCounts.tier2}</em>
									</button>
								{/if}
								{#if filterCounts.tier3 > 0}
									<button class="mmChip" class:mmChipOn={candFilter === 'tier3'} type="button" onclick={() => (candFilter = 'tier3')}>
										Tier 3<em>{filterCounts.tier3}</em>
									</button>
								{/if}
							</div>
							<div class="mmSortWrap">
								<Select
									value={candSort}
									onchange={(v) => (candSort = v as CandSort)}
									options={[
										{ value: 'skor', label: tr('Skor tertinggi') },
										{ value: 'harga', label: tr('Harga termurah') },
										{ value: 'da', label: tr('DA tertinggi') }
									]}
									ariaLabel={tr('Urut kandidat')}
									minWidth={0}
									align="end"
									fitContent
								/>
							</div>
						</div>

						{#if visibleCandidates.length === 0}
							<p class="mmMuted">{tr('Tidak ada kandidat pada filter ini.')}</p>
						{:else}
						<ul class="mmCards">
							{#each visibleCandidates as c (c.id)}
								{@const cardName = mediaDomainLabel(c.website_url, c.name)}
								<li class="mmCard2" class:mmCard2On={selected.has(c.id)}>
									<div class="mmCard2Main">
										<input
											class="mmCheck"
											type="checkbox"
											checked={selected.has(c.id)}
											onchange={() => toggle(c.id)}
											aria-label={cardName}
										/>
										{#if !faviconFailed.has(c.id) && faviconSrc(c.website_url)}
											<img
												class="mmAvatar mmAvatarImg"
												src={faviconSrc(c.website_url)}
												alt=""
												loading="lazy"
												onerror={() => (faviconFailed = new Set(faviconFailed).add(c.id))}
											/>
										{:else}
											<span class="mmAvatar" style={`background:${avatarColor(cardName)}`}>{cardName.charAt(0).toUpperCase()}</span>
										{/if}
										<div class="mmCard2Info">
											<div class="mmCard2Top">
												<strong class="mmCard2Name">{cardName}</strong>
												<span class="mmScoreChip">{Math.round(c.score)} · {tr(c.match_label)}</span>
												{#if c.in_package}<span class="mmRecBadge">{tr('Rekomendasi paket')}</span>{/if}
											</div>
											<div class="mmMeta2">
												<span>{c.category || tr('Tanpa kategori')}</span>
												<span>DA {c.seo_da ?? '—'}</span>
												<span>AI Trust {c.seo_ai_trust ?? '—'}</span>
												<span>{trafficFmt(c.traffic_estimate)} / bln</span>
												<span>{c.link_duration_months === 0 ? tr('Tautan permanen') : `${c.link_duration_months} bln`}</span>
											</div>
										</div>
										<div class="mmCard2Side">
											<strong class="mmPrice">{money(c.client_price)}</strong>
											{#if detailIds.has(c.id)}
												<button class="mmDetailBtn" type="button" onclick={() => onShowDetail(c.id)}>
													{tr('Detail')} <ArrowRight size={13} />
												</button>
											{/if}
										</div>
									</div>
									{#if c.reasons.length > 0}<p class="mmReason2">{c.reasons.join(' ')}</p>{/if}
									{#if c.warnings.length > 0}<p class="mmItemWarn2"><CircleAlert size={13} /> {c.warnings.join(' ')}</p>{/if}
								</li>
							{/each}
						</ul>
						{/if}
					{:else}
						<div class="mmEmpty">
							<p>{tr('Tidak ada media yang memenuhi kriteria ini. Longgarkan budget, DA, atau kategori lalu cari lagi.')}</p>
						</div>
					{/if}
				</section>

				<section class="mmCol mmColRight">
					<div class="mmResHead">
						<span class="mmResIcon mmResIconPlain"><BarChart3 size={22} /></span>
						<div class="mmResHeadText">
							<div class="mmResTitleRow">
								<h3 class="mmResTitle">{tr('Ringkasan paket')}</h3>
							</div>
							<p class="mmResSub">
								{result?.selector === 'agent'
									? tr('Paket dipilih AI dari kandidat katalog nyata.')
									: tr('Paket disusun skor otomatis dari kandidat katalog nyata.')}
							</p>
						</div>
					</div>

					<div class="mmStatGrid">
						<div class="mmStatCard">
							<span class="mmStatIcon" style="background:#eff6ff;color:#2563eb"><Layers size={16} /></span>
							<span class="mmStatLabel">{tr('Media terpilih')}</span>
							<strong class="mmStatValue">{selected.size}</strong>
							<span class="mmStatSub">{result?.candidates.length ?? 0} {tr('kandidat')}</span>
						</div>
						<div class="mmStatCard">
							<span class="mmStatIcon" style="background:#ecfdf5;color:#059669"><CircleDollarSign size={16} /></span>
							<span class="mmStatLabel">{tr('Total harga')}</span>
							<strong class="mmStatValue">{money(selectedTotal)}</strong>
						</div>
						<div class="mmStatCard">
							<span class="mmStatIcon" style="background:#fff7ed;color:#ea580c"><Wallet size={16} /></span>
							<span class="mmStatLabel">{tr('Anggaran')}</span>
							<strong class="mmStatValue">{result && result.package.budget > 0 ? money(result.package.budget) : '—'}</strong>
						</div>
						<div class="mmStatCard">
							<span class="mmStatIcon" style="background:#eff6ff;color:#2563eb"><Clock size={16} /></span>
							<span class="mmStatLabel">{tr('Sisa anggaran')}</span>
							<strong class="mmStatValue">{result && result.package.budget > 0 ? money(Math.max(0, result.package.budget - selectedTotal)) : '—'}</strong>
						</div>
					</div>

					{#if result && result.package.summary}<p class="mmPkgSummary">{result.package.summary}</p>{/if}

					{#if result && result.composition.items.length > 0}
						<div class="mmComp">
							<div class="mmCompHead">
								<span class="mmCompLabel"><FileText size={15} /> {tr('Komposisi media')}</span>
								{#if compositionTop}<span class="mmCompMeta">{compositionTop.label} | {compositionTop.percent}%</span>{/if}
							</div>
							<div class="mmBar">
								{#each result.composition.items as item (item.key)}
									<span class="mmBarSeg" style={`width:${item.percent}%;background:${tierColor(item.key)}`}></span>
								{/each}
							</div>
							<div class="mmLegend">
								{#each result.composition.items as item (item.key)}
									<span class="mmLegendItem">
										<span class="mmDot" style={`background:${tierColor(item.key)}`}></span>
										{item.label} {item.percent}%
									</span>
								{/each}
							</div>
						</div>
					{/if}

					{#if result && (result.package.unmet.length > 0 || result.warnings.length > 0)}
						<div class="mmWarnBox">
							<strong><TriangleAlert size={14} /> {tr('Catatan')}</strong>
							<ul>
								{#each [...result.package.unmet, ...result.warnings] as w (w)}<li>{w}</li>{/each}
							</ul>
						</div>
					{/if}

					{#if overBudget}
						<div class="mmErrorBox"><CircleAlert size={15} /> {tr('Total pilihan melebihi anggaran yang diisi.')}</div>
					{/if}

					<div class="mmInfoBox">
						<Info size={16} />
						<span>{tr('Rekomendasi ini belum membuat pesanan. Periksa pilihanmu, lalu terapkan ke katalog bila sudah cocok.')}</span>
					</div>
				</section>
			</div>
		{/if}

		<!-- Footer -->
		<footer class="mmFooter">
			<div class="mmFootNote">
				<ShieldCheck size={18} />
				<span>{tr('Media akan dicocokkan berdasarkan data katalog, kategori, metrik, harga, dan ketersediaan.')}</span>
			</div>
			<div class="mmFootActions">
				{#if step === 1}
					<button class="mmBtn" type="button" onclick={close}>{tr('Batal')}</button>
					<button class="mmBtnPrimary" type="button" disabled={searching || analyzing || !criteria} onclick={() => void search()}>
						{searching ? tr('Mencari media…') : tr('Cari Media yang Cocok')}
						{#if searching}<span class="mmSpin"><LoaderCircle size={16} /></span>{:else}<ArrowRight size={16} />{/if}
					</button>
				{:else}
					<button class="mmBtn" type="button" onclick={() => (step = 1)}>{tr('Kembali')}</button>
					<button class="mmBtnPrimary" type="button" disabled={selected.size === 0} onclick={apply}>
						<Check size={16} />
						{hasSelection ? tr('Terapkan & Ganti Pilihan') : tr('Terapkan ke Katalog')}
					</button>
				{/if}
			</div>
		</footer>
	</dialog>
</div>

<style>
	.mmOverlay {
		position: fixed;
		inset: 0;
		z-index: 100;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 20px;
		background: rgba(15, 23, 42, 0.5);
		backdrop-filter: blur(1.5px);
	}

	.mmDialog {
		display: flex;
		flex-direction: column;
		width: 100%;
		max-width: 1180px;
		max-height: 92vh;
		background: var(--card, #fff);
		border: 1px solid var(--border, #dfe5ef);
		border-radius: 16px;
		box-shadow: 0 24px 60px rgba(15, 23, 42, 0.18);
		color: #0f172a;
		overflow: hidden;
		padding: 0;
	}

	/* ── Header ─────────────────────────────────────────────── */
	.mmHeader {
		display: flex;
		align-items: flex-start;
		gap: 16px;
		padding: 24px 26px 12px;
	}
	.mmHeaderIcon {
		flex: none;
		width: 58px;
		height: 58px;
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.mmHeaderIcon img {
		max-width: 100%;
		max-height: 100%;
		user-select: none;
	}
	.mmHeaderText {
		flex: 1;
		min-width: 0;
	}
	.mmHeaderText h2 {
		margin: 2px 0 2px;
		font-size: 23px;
		line-height: 1.25;
		font-weight: 700;
		letter-spacing: -0.01em;
	}
	.mmHeaderText p {
		margin: 0;
		font-size: 13.5px;
		color: var(--muted-foreground, #667085);
	}
	.mmClose {
		flex: none;
		border: none;
		background: transparent;
		color: #94a3b8;
		cursor: pointer;
		padding: 6px;
		border-radius: 8px;
		display: inline-flex;
	}
	.mmClose:hover {
		background: #f1f5f9;
		color: #0f172a;
	}

	/* ── Stepper ────────────────────────────────────────────── */
	.mmStepper {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 14px;
		padding: 10px 26px 18px;
	}
	.mmStep {
		display: inline-flex;
		align-items: center;
		gap: 9px;
		color: #94a3b8;
		font-size: 13.5px;
		font-weight: 500;
	}
	.mmStepDot {
		width: 27px;
		height: 27px;
		border-radius: 999px;
		background: #eef2f8;
		color: #94a3b8;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		font-size: 12.5px;
		font-weight: 700;
	}
	.mmStepOn {
		color: var(--primary, #2b67ec);
		font-weight: 650;
	}
	.mmStepOn .mmStepDot {
		background: var(--primary, #2b67ec);
		color: #fff;
	}
	.mmStepLine {
		width: min(340px, 34%);
		height: 1px;
		background: #e2e8f0;
	}
	.mmStepLineOn {
		background: #c7d2fe;
	}

	/* ── Body dua kolom ─────────────────────────────────────── */
	.mmBody {
		flex: 1;
		min-height: 0;
		display: grid;
		grid-template-columns: 1fr 1fr;
		overflow: hidden;
		border-top: 1px solid var(--border, #dfe5ef);
	}
	.mmCol {
		padding: 20px 26px 22px;
		overflow-y: auto;
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-height: 0;
		/* Scrollbar custom — tipis, bulat, abu lembut (bukan default browser). */
		scrollbar-width: thin;
		scrollbar-color: #cbd5e1 transparent;
	}
	.mmCol::-webkit-scrollbar {
		width: 6px;
	}
	.mmCol::-webkit-scrollbar-track {
		background: transparent;
	}
	.mmCol::-webkit-scrollbar-thumb {
		background: #cbd5e1;
		border-radius: 999px;
	}
	.mmCol:hover::-webkit-scrollbar-thumb {
		background: #94a3b8;
	}
	.mmColRight {
		border-left: 1px solid var(--border, #dfe5ef);
		background: #fcfdff;
	}
	.mmColTitle {
		margin: 0;
		font-size: 19px;
		font-weight: 700;
		letter-spacing: -0.01em;
	}
	.mmColHead {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
	}
	.mmExampleBtn {
		flex: none;
		display: inline-flex;
		align-items: center;
		gap: 6px;
		border: 1px solid #c7dbff;
		background: #eff6ff;
		color: var(--primary, #2b67ec);
		font-size: 12px;
		font-weight: 650;
		padding: 6px 11px;
		border-radius: 8px;
		cursor: pointer;
		transition: background 0.15s ease, border-color 0.15s ease;
	}
	.mmExampleBtn:hover {
		background: #dbeafe;
		border-color: #a5c8fc;
	}
	.mmColSub {
		margin: -6px 0 2px;
		font-size: 13px;
		color: var(--muted-foreground, #667085);
	}
	.mmRightHead {
		display: flex;
		align-items: center;
		gap: 10px;
		flex-wrap: wrap;
	}
	.mmEditBtn {
		margin-left: auto;
	}

	/* ── Textarea ───────────────────────────────────────────── */
	.mmTextWrap {
		position: relative;
		display: flex;
		flex: 1;
		min-height: 240px;
	}
	.mmTextarea {
		width: 100%;
		resize: none;
		border: 1px solid #dbe2ee;
		border-radius: 8px;
		padding: 14px 15px 34px;
		font-family: inherit;
		font-size: 14px;
		line-height: 1.6;
		color: #0f172a;
		background: #fff;
		outline: none;
	}
	.mmTextarea::placeholder {
		color: #9aa7bb;
	}
	.mmTextarea:focus {
		border-color: var(--primary, #2b67ec);
		box-shadow: 0 0 0 3px rgba(43, 103, 236, 0.12);
	}
	.mmCounter {
		position: absolute;
		right: 12px;
		bottom: 10px;
		font-size: 11.5px;
		color: #94a3b8;
		pointer-events: none;
	}
	.mmBriefTools {
		display: flex;
		align-items: center;
		gap: 7px;
		font-size: 12.5px;
		color: var(--muted-foreground, #667085);
		flex-wrap: wrap;
	}
	.mmBriefActions {
		display: flex;
		align-items: center;
		gap: 10px;
		flex-wrap: wrap;
	}
	.mmSummarize {
		align-self: flex-start;
	}
	.mmStaleHint {
		font-size: 12px;
		color: #a15c00;
	}
	.mmAttachIcons {
		display: inline-flex;
		color: #94a3b8;
	}
	.mmAttachHint {
		flex: 1;
		min-width: 140px;
	}
	.mmLinkBtn {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		background: none;
		border: none;
		padding: 0;
		color: var(--primary, #2b67ec);
		font-size: 12.5px;
		font-weight: 600;
		cursor: pointer;
	}
	.mmLinkBtn:hover {
		text-decoration: underline;
	}

	/* ── Ringkasan ──────────────────────────────────────────── */
	.mmBadge {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		font-size: 11.5px;
		font-weight: 600;
		padding: 4px 9px;
		border-radius: 999px;
		border: 1px solid transparent;
	}
	.mmBadgeOk {
		background: var(--success-soft, #eaf8f1);
		color: var(--success, #16794b);
		border-color: #c9eeda;
	}
	.mmBadgeWarn {
		background: var(--warning-soft, #fff6df);
		color: #a15c00;
		border-color: #f6e2b0;
	}
	.mmBadgeMuted {
		background: #f1f5f9;
		color: #64748b;
	}
	.mmCards {
		display: flex;
		flex-direction: column;
		gap: 9px;
	}
	.mmCardRow,
	.mmCardBlock {
		border: 1px solid var(--border, #dfe5ef);
		border-radius: 10px;
		background: #fff;
	}
	.mmCardRow {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 12px 14px;
	}
	.mmCardBlock {
		padding: 12px 14px;
		display: flex;
		flex-direction: column;
		gap: 9px;
	}
	.mmCardHead {
		display: flex;
		align-items: center;
		gap: 10px;
	}
	.mmCardIcon {
		display: inline-flex;
		color: #7c8aa0;
		flex: none;
	}
	.mmCardLabel {
		font-size: 13px;
		color: var(--muted-foreground, #667085);
	}
	.mmCardValue {
		margin-left: auto;
		font-size: 13.5px;
		font-weight: 650;
		text-align: right;
		color: #0f172a;
	}
	.mmInlineInput {
		margin-left: auto;
		width: 96px;
		height: 30px;
		border: 1px solid #dbe2ee;
		border-radius: 7px;
		padding: 0 9px;
		font-size: 12.5px;
		font-family: inherit;
		text-align: right;
		outline: none;
		background: #fff;
	}
	.mmInlineInput:focus {
		border-color: var(--primary, #2b67ec);
		box-shadow: 0 0 0 3px rgba(43, 103, 236, 0.1);
	}
	.mmInlineWide {
		flex: 1;
		width: auto;
		text-align: left;
	}
	.mmBudgetEdit {
		margin-left: auto;
		display: inline-flex;
		flex-wrap: wrap;
		gap: 8px;
	}
	.mmBudgetEdit label {
		display: inline-flex;
		flex-direction: column;
		gap: 3px;
		font-size: 10.5px;
		color: var(--muted-foreground, #667085);
	}
	.mmBudgetEdit .mmInlineInput {
		margin-left: 0;
		width: 110px;
	}
	.mmBudgetCol {
		margin-left: auto;
		display: inline-flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 2px;
	}
	.mmBudgetSub {
		font-size: 11.5px;
		color: var(--muted-foreground, #667085);
	}
	.mmRuleEdit {
		display: flex;
		flex-wrap: wrap;
		gap: 14px;
	}
	.mmRuleEdit label {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: 12px;
		color: var(--muted-foreground, #667085);
	}
	.mmRuleEdit .mmInlineInput {
		margin-left: 0;
		width: 78px;
	}
	.mmRules {
		margin: 0;
		padding-left: 18px;
		/* Preflight Tailwind mematikan bullet list — aktifkan lagi. */
		list-style: disc;
		font-size: 12.5px;
		line-height: 1.7;
		color: #334155;
	}
	.mmRules li::marker {
		color: #94a3b8;
	}
	.mmMuted {
		margin: 0;
		font-size: 12.5px;
		color: #94a3b8;
	}
	.mmBar {
		display: flex;
		height: 9px;
		border-radius: 999px;
		overflow: hidden;
		background: #eef2f8;
	}
	.mmBarSeg {
		height: 100%;
	}
	.mmLegend {
		display: flex;
		flex-wrap: wrap;
		gap: 12px;
		margin-top: 9px;
		font-size: 11.5px;
		color: #475569;
	}
	.mmLegendItem {
		display: inline-flex;
		align-items: center;
		gap: 5px;
	}
	.mmDot {
		width: 8px;
		height: 8px;
		border-radius: 999px;
		display: inline-block;
	}
	.mmInfoBox {
		display: flex;
		align-items: flex-start;
		gap: 9px;
		background: #eef4ff;
		border-radius: 10px;
		padding: 11px 13px;
		font-size: 12.5px;
		line-height: 1.55;
		color: #2952a3;
	}
	.mmInfoBox :global(svg) {
		flex: none;
		margin-top: 1px;
	}
	.mmEmpty {
		flex: 1;
		min-height: 180px;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 10px;
		border: 1px dashed #dbe2ee;
		border-radius: 10px;
		color: #94a3b8;
		font-size: 13px;
		text-align: center;
		padding: 20px;
	}
	.mmEmpty p {
		margin: 0;
		max-width: 320px;
		line-height: 1.55;
	}

	/* ── Animasi pemrosesan premium (kolom ringkasan) ───────── */
	.mmProc {
		position: relative;
		overflow: hidden;
		flex: 1;
		min-height: 260px;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 12px;
		border: 1px solid #d8e4fb;
		border-radius: 12px;
		background: linear-gradient(180deg, #f7faff, #eef4ff);
		padding: 22px 20px 26px;
	}
	.mmMascot {
		width: 84px;
		height: 84px;
		object-fit: contain;
		animation: mmFloat 3s ease-in-out infinite;
	}
	@keyframes mmFloat {
		0%,
		100% {
			transform: translateY(0);
		}
		50% {
			transform: translateY(-7px);
		}
	}
	.mmProcStatus {
		margin: 0;
		font-size: 13.5px;
		font-weight: 650;
		color: var(--primary, #2b67ec);
		display: inline-flex;
		align-items: baseline;
	}
	.mmProcDots i {
		display: inline-block;
		width: 3px;
		height: 3px;
		margin-left: 3px;
		border-radius: 999px;
		background: var(--primary, #2b67ec);
		animation: mmDot 1.2s ease-in-out infinite;
	}
	.mmProcDots i:nth-child(2) {
		animation-delay: 0.2s;
	}
	.mmProcDots i:nth-child(3) {
		animation-delay: 0.4s;
	}
	@keyframes mmDot {
		0%,
		100% {
			opacity: 0.25;
		}
		50% {
			opacity: 1;
		}
	}

	/* Skeleton meniru struktur kartu hasil (ikon + label + nilai). */
	.mmSkList {
		width: 100%;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.mmSkRow {
		display: flex;
		align-items: center;
		gap: 10px;
		border: 1px solid #e2e9f8;
		border-radius: 10px;
		background: #fff;
		padding: 10px 12px;
	}
	.mmSkIcon {
		flex: none;
		width: 26px;
		height: 26px;
		border-radius: 8px;
		background: #e4ecfb;
	}
	.mmSkBar,
	.mmSkVal {
		height: 9px;
		border-radius: 999px;
		background: linear-gradient(90deg, #e6edf9 25%, #f2f6fd 50%, #e6edf9 75%);
		background-size: 200% 100%;
		animation: mmShimmer 1.6s ease-in-out infinite;
	}
	.mmSkVal {
		margin-left: auto;
	}
	@keyframes mmShimmer {
		0% {
			background-position: 200% 0;
		}
		100% {
			background-position: -200% 0;
		}
	}

	/* Garis pemindaian horizontal di dalam panel ringkasan. */
	.mmScan {
		position: absolute;
		left: 6%;
		right: 6%;
		height: 2px;
		border-radius: 999px;
		background: linear-gradient(90deg, transparent, rgba(43, 103, 236, 0.45), transparent);
		animation: mmScanMove 3.2s ease-in-out infinite;
		pointer-events: none;
	}
	@keyframes mmScanMove {
		0%,
		100% {
			top: 12%;
			opacity: 0;
		}
		15%,
		85% {
			opacity: 1;
		}
		50% {
			top: 88%;
		}
	}

	/* Hasil muncul bertahap: fade + geser vertikal pendek, stagger 90ms. */
	.mmReveal > * {
		animation: mmFadeUp 240ms ease both;
	}
	.mmReveal > *:nth-child(2) {
		animation-delay: 90ms;
	}
	.mmReveal > *:nth-child(3) {
		animation-delay: 180ms;
	}
	.mmReveal > *:nth-child(4) {
		animation-delay: 270ms;
	}
	@keyframes mmFadeUp {
		from {
			opacity: 0;
			transform: translateY(6px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.mmMascot,
		.mmProcDots i,
		.mmSkBar,
		.mmSkVal,
		.mmReveal > * {
			animation: none;
		}
		.mmScan {
			display: none;
		}
	}
	.mmErrorBox {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		background: #fef2f2;
		border: 1px solid #fecaca;
		color: #b91c1c;
		border-radius: 9px;
		padding: 10px 12px;
		font-size: 12.5px;
		line-height: 1.5;
	}
	.mmWarnBox {
		border: 1px solid #f6e2b0;
		background: var(--warning-soft, #fff6df);
		color: #8a5200;
		border-radius: 10px;
		padding: 10px 12px;
		font-size: 12px;
		line-height: 1.55;
	}
	.mmWarnBox strong {
		display: inline-flex;
		align-items: center;
		gap: 5px;
	}
	.mmWarnBox ul {
		margin: 5px 0 0;
		padding-left: 18px;
		list-style: disc;
	}
	.mmWarnBox li {
		margin-bottom: 3px;
	}
	.mmWarnBox li::marker {
		color: #d97706;
	}

	/* ── Tahap 2 ────────────────────────────────────────────── */
	/* ── Tahap 2: hasil pencarian ───────────────────────────── */
	.mmBodyResults {
		grid-template-columns: minmax(0, 3fr) minmax(0, 2fr);
		gap: 20px;
	}
	.mmBodyResults .mmCol {
		padding: 20px 22px 24px;
		gap: 20px;
	}
	.mmBodyResults .mmColRight {
		background: #fff;
	}
	.mmResHead {
		display: flex;
		align-items: flex-start;
		gap: 12px;
	}
	.mmResIcon {
		flex: none;
		width: 44px;
		height: 44px;
		border-radius: 12px;
		background: #eff6ff;
		color: #2563eb;
		display: inline-flex;
		align-items: center;
		justify-content: center;
	}
	.mmResIconPlain {
		background: #f1f5f9;
		color: #334155;
	}
	.mmResHeadText {
		flex: 1;
		min-width: 0;
	}
	.mmResTitleRow {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}
	.mmResTitle {
		margin: 0;
		font-size: 18px;
		font-weight: 700;
		letter-spacing: -0.01em;
	}
	.mmResSub {
		margin: 3px 0 0;
		font-size: 12.5px;
		color: var(--muted-foreground, #667085);
	}

	/* Chip filter + urut daftar kandidat */
	.mmFilterBar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		flex-wrap: wrap;
	}
	.mmChips {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
	.mmChip {
		display: inline-flex;
		align-items: center;
		gap: 5px;
		border: 1px solid var(--border, #dfe5ef);
		background: #fff;
		border-radius: 999px;
		padding: 5px 11px;
		font-size: 12px;
		font-weight: 600;
		color: #475569;
		cursor: pointer;
		transition: background 0.15s ease, border-color 0.15s ease, color 0.15s ease;
	}
	.mmChip em {
		font-style: normal;
		font-size: 11px;
		color: #94a3b8;
	}
	.mmChip:hover {
		border-color: #b9cdf5;
	}
	.mmChipOn {
		background: #eff6ff;
		border-color: #c7dbff;
		color: var(--primary, #2b67ec);
	}
	.mmChipOn em {
		color: var(--primary, #2b67ec);
	}
	.mmSortWrap :global(.zs-select-trigger) {
		height: 30px;
		padding: 0 10px;
		font-size: 12px;
		font-weight: 600;
		color: #475569;
		border-radius: 999px;
	}

	/* Kartu kandidat */
	.mmCards {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.mmCard2 {
		border: 1px solid var(--border, #dfe5ef);
		border-radius: 12px;
		padding: 12px 14px;
		background: #fff;
		transition: border-color 0.15s ease, background 0.15s ease;
	}
	.mmCard2On {
		border-color: var(--primary, #2b67ec);
		background: #f5f9ff;
	}
	.mmCard2Main {
		display: flex;
		align-items: flex-start;
		gap: 10px;
	}
	.mmCheck {
		margin-top: 10px;
		flex: none;
		width: 15px;
		height: 15px;
		accent-color: var(--primary, #2b67ec);
	}
	.mmAvatar {
		flex: none;
		width: 34px;
		height: 34px;
		border-radius: 9px;
		color: #fff;
		font-size: 15px;
		font-weight: 700;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		margin-top: 1px;
	}
	.mmAvatarImg {
		object-fit: cover;
		background: #fff;
		border: 1px solid var(--border, #dfe5ef);
	}
	.mmCard2Info {
		flex: 1;
		min-width: 0;
	}
	.mmCard2Top {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 7px;
	}
	.mmCard2Name {
		font-size: 13.5px;
		font-weight: 700;
		overflow-wrap: anywhere;
	}
	.mmScoreChip {
		font-size: 10.5px;
		font-weight: 650;
		background: #f1f5f9;
		color: #475569;
		padding: 2px 7px;
		border-radius: 999px;
	}
	.mmRecBadge {
		font-size: 10.5px;
		font-weight: 650;
		background: var(--success-soft, #eaf8f1);
		color: var(--success, #16794b);
		padding: 2px 8px;
		border-radius: 999px;
	}
	.mmMeta2 {
		display: flex;
		flex-wrap: wrap;
		margin-top: 5px;
		font-size: 11.5px;
		color: #7c8aa0;
	}
	.mmMeta2 > span {
		display: inline-flex;
		align-items: center;
	}
	.mmMeta2 > span + span::before {
		content: '';
		display: inline-block;
		width: 1px;
		height: 10px;
		background: #e2e8f0;
		margin: 0 8px;
	}
	.mmCard2Side {
		flex: none;
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 6px;
		margin-left: 8px;
	}
	.mmPrice {
		font-size: 13.5px;
		font-weight: 700;
		white-space: nowrap;
	}
	.mmDetailBtn {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		border: none;
		background: #eff6ff;
		color: var(--primary, #2b67ec);
		font-size: 11.5px;
		font-weight: 650;
		padding: 5px 10px;
		border-radius: 8px;
		cursor: pointer;
		transition: background 0.15s ease;
	}
	.mmDetailBtn:hover {
		background: #dbeafe;
	}
	.mmReason2 {
		margin: 9px 0 0 59px;
		font-size: 11.5px;
		line-height: 1.5;
		color: #475569;
	}
	.mmItemWarn2 {
		display: flex;
		align-items: flex-start;
		gap: 5px;
		margin: 5px 0 0 59px;
		font-size: 11.5px;
		color: #a15c00;
	}

	/* Ringkasan paket */
	.mmPkgSummary {
		margin: 0;
		font-size: 13px;
		line-height: 1.55;
		color: #0f172a;
	}
	.mmStatGrid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 10px;
	}
	.mmStatCard {
		border: 1px solid var(--border, #dfe5ef);
		border-radius: 12px;
		background: #fff;
		padding: 12px 13px;
		display: flex;
		flex-direction: column;
		gap: 3px;
	}
	.mmStatIcon {
		width: 30px;
		height: 30px;
		border-radius: 9px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		margin-bottom: 3px;
	}
	.mmStatLabel {
		font-size: 11.5px;
		color: var(--muted-foreground, #667085);
	}
	.mmStatValue {
		font-size: 16.5px;
		font-weight: 750;
		letter-spacing: -0.01em;
		overflow-wrap: anywhere;
	}

	/* Komposisi */
	.mmComp {
		margin: 0;
	}
	.mmCompHead {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		margin-bottom: 10px;
	}
	.mmCompLabel {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		font-size: 13px;
		font-weight: 700;
		color: #0f172a;
	}
	.mmCompMeta {
		font-size: 12px;
		color: var(--muted-foreground, #667085);
	}

	/* ── Footer ─────────────────────────────────────────────── */
	.mmFooter {
		flex: none;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 18px;
		padding: 14px 26px;
		border-top: 1px solid var(--border, #dfe5ef);
		background: #fff;
	}
	.mmFootNote {
		display: flex;
		align-items: center;
		gap: 9px;
		font-size: 12.5px;
		color: var(--muted-foreground, #667085);
		min-width: 0;
	}
	.mmFootNote :global(svg) {
		flex: none;
		color: var(--primary, #2b67ec);
	}
	.mmFootActions {
		display: flex;
		align-items: center;
		gap: 10px;
		flex: none;
	}
	.mmBtn {
		height: 42px;
		padding: 0 20px;
		border: 1px solid var(--border, #dfe5ef);
		border-radius: 8px;
		background: #fff;
		color: #334155;
		font-size: 13.5px;
		font-weight: 600;
		font-family: inherit;
		cursor: pointer;
	}
	.mmBtn:hover {
		background: #f8fafc;
	}
	.mmBtnPrimary {
		height: 42px;
		padding: 0 20px;
		display: inline-flex;
		align-items: center;
		gap: 8px;
		border: none;
		border-radius: 8px;
		background: var(--primary, #2b67ec);
		color: #fff;
		font-size: 13.5px;
		font-weight: 650;
		font-family: inherit;
		cursor: pointer;
		box-shadow: 0 2px 8px rgba(43, 103, 236, 0.22);
	}
	.mmBtnPrimary:hover:not(:disabled) {
		background: #1f56d4;
	}
	.mmBtnPrimary:disabled {
		opacity: 0.55;
		cursor: not-allowed;
		box-shadow: none;
	}
	.mmSpin {
		display: inline-flex;
		animation: mmSpin 1s linear infinite;
	}
	@keyframes mmSpin {
		to {
			transform: rotate(360deg);
		}
	}

	/* ── Responsif ──────────────────────────────────────────── */
	@media (max-width: 980px) {
		.mmBody {
			grid-template-columns: 1fr;
			overflow-y: auto;
		}
		.mmCol {
			overflow: visible;
		}
		.mmColRight {
			border-left: none;
			border-top: 1px solid var(--border, #dfe5ef);
		}
		.mmTextWrap {
			min-height: 200px;
		}
		.mmStepLine {
			width: 16%;
		}
	}
	@media (max-width: 640px) {
		.mmOverlay {
			padding: 0;
			align-items: stretch;
		}
		.mmDialog {
			max-height: 100vh;
			height: 100vh;
			border-radius: 0;
			border: none;
		}
		.mmHeader {
			padding: 18px 16px 10px;
			gap: 12px;
		}
		.mmHeaderIcon {
			width: 46px;
			height: 46px;
		}
		.mmHeaderText h2 {
			font-size: 19px;
		}
		.mmStepper {
			padding: 6px 16px 14px;
			gap: 8px;
		}
		.mmStepLabel {
			font-size: 12px;
		}
		.mmStepLine {
			width: 12%;
		}
		.mmCol {
			padding: 16px;
		}
		.mmFooter {
			flex-direction: column;
			align-items: stretch;
			gap: 10px;
			padding: 12px 16px;
		}
		.mmFootActions {
			justify-content: stretch;
		}
		.mmFootActions > * {
			flex: 1;
			justify-content: center;
		}
		.mmCardRow {
			flex-wrap: wrap;
		}
		.mmCardValue {
			flex-basis: 100%;
			text-align: left;
			margin-left: 28px;
		}
		.mmStatGrid {
			grid-template-columns: 1fr;
		}
		.mmReason2,
		.mmItemWarn2 {
			margin-left: 0;
		}
	}
</style>
