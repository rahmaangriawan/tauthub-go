<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Beranda Klien — port 1:1 dari overview-dashboard.tsx & overview-dashboard.module.css
	// app Next.js (bukan client-overview.tsx lama).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import {
		ArrowRight, BookOpen, CircleCheck, Coins, FileText, Globe,
		Lightbulb, Lock, ShieldCheck, Sparkles, TrendingUp, X, Zap
	} from '@lucide/svelte';
	import { apiRequest, getStoredSession, showToast, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { campaignUrlId } from '$lib/campaign-url';
	import { getI18n } from '$lib/i18n/i18n.svelte';
	import { submitTopUp, type TopUpInvoiceData } from '$lib/credits/topup-client';
	import TopUpModal from '$lib/components/credit/topup-modal.svelte';
	import PageTitle from '$lib/components/page-title.svelte';
	import HelpModal from '$lib/components/help-modal.svelte';
	import GradientWave from '$lib/components/gradient-wave.svelte';
	import { STAGE_LABEL, STAGE_TONE, DONE_STATUSES } from './taut-pintar/_shared';

	// Palet brand untuk latar hero, dijaga di nada menengah supaya teks putih
	// tetap terbaca di mana pun gelombangnya lewat: navy, royal, biru brand, biru langit.
	const HERO_WAVE = ['#0b1b4a', '#1d4ed8', '#2b67ec', '#6aa5ff'];
		
	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const fallbackBase = $derived(`/klien/${brandUrl}`);
	const i18n = $derived(getI18n());

	type Wallet = { available_tc: number; reserved_tc: number; captured_tc: number; rate_idr: number };
	type Pkg = { tc: number; idr: number };
	type PubRow = {
		id: string;
		document_title: string;
		media_name: string;
		status: string;
		client_price: number;
	};

	const STATUS_COLORS: Record<string, string> = {
		published: 'green',
		completed: 'green',
		processing: 'blue',
		queued: 'gray',
		scheduled: 'blue',
		failed: 'red',
		rejected: 'red',
		draft: 'gray',
		cancelled: 'gray',
		canceled: 'gray',
		waiting_partner: 'amber',
		partner_approved: 'green',
		partner_rejected: 'red',
		pending: 'amber',
		retrying: 'amber'
	};

	function tcLabel(rupiah: number, _rate: number): string {
		return `Rp ${Math.max(0, Math.round(Number(rupiah) || 0)).toLocaleString('id-ID')}`;
	}

	function startOfMonth(d = new Date()) {
		return new Date(d.getFullYear(), d.getMonth(), 1);
	}
	function lastMonthRange() {
		const now = new Date();
		const from = new Date(now.getFullYear(), now.getMonth() - 1, 1);
		const to = new Date(now.getFullYear(), now.getMonth(), 0, 23, 59, 59);
		return { from, to };
	}

	let userName = $state('');

	// Popup panduan: kartu "Cara Kerja" dan "Standar Gatekeeper" membuka dialog
	// yang sama dengan tab awal berbeda.
	let helpOpen = $state(false);
	let helpTab = $state<'how' | 'gate'>('how');
	function openHelp(tab: 'how' | 'gate') {
		helpTab = tab;
		helpOpen = true;
	}

	// Emoji sapaan dipisah dari teksnya agar bisa dianimasikan (lihat .heroWave);
	// teksnya sendiri tetap berasal dari kamus i18n.
	const WAVE_EMOJI = '👋';
	const welcomeText = $derived(
		i18n.t.dashboard.welcome.replace('{name}', userName || 'User').replace(WAVE_EMOJI, '')
	);
	// Kata terakhir dan emoji dibungkus satu kelompok tanpa putus baris, supaya
	// emoji tidak jatuh sendirian di baris kedua saat sapaan membungkus.
	const welcomeParts = $derived.by(() => {
		const text = welcomeText.trimEnd();
		const cut = text.lastIndexOf(' ');
		return cut === -1
			? { head: '', tail: text }
			: { head: text.slice(0, cut + 1), tail: text.slice(cut + 1) };
	});
	let wallet = $state<Wallet | null>(null);
	let tcConfig = $state<{ rate_idr: number; topup_packages: Pkg[] } | null>(null);
	let reports = $state<{ reach: number; clicks: number } | null>(null);
	let prevReports = $state<{ reach: number; clicks: number } | null>(null);
	let pubs = $state<PubRow[]>([]);
	let pubsTotal = $state(0);
	// Kampanye Taut Pintar: sumber yang sama dengan menu Kampanye, jadi angkanya
	// selalu cocok dengan daftar di sana.
	type HeroCampaign = {
		id: string;
		public_id: string | null;
		name: string;
		taut_status: string | null;
		stats?: { articles: number; published_media: number; tc_used: number } | null;
	};
	let campaigns = $state<HeroCampaign[]>([]);
	let loading = $state(true);

	// Popup "Buat Kampanye"
	let createOpen = $state(false);
	let createBusy = $state(false);
	let createErr = $state('');
	let createName = $state('');
	let createMode = $state<'kilat' | 'manual' | null>('kilat');

	// Popup Top Up
	let topUpOpen = $state(false);
	let topUpBusy = $state(false);
	let topUpPayment = $state<{ payment_url: string | null; amount_idr: number; tc: number } | null>(null);
	let topUpError = $state('');
	let topUpKey = '';

	async function load() {
		try {
			const [creditsRes, pubsRes, reportsRes, prevRes, campRes] = await Promise.all([
				apiRequest(`/v1/brands/${brandId}/credits?limit=1`),
				apiRequest(`/v1/brands/${brandId}/publications?limit=6`),
				apiRequest(
					`/v1/brands/${brandId}/reports?from=${startOfMonth().toISOString()}&to=${new Date().toISOString()}`
				),
				apiRequest(
					`/v1/brands/${brandId}/reports?from=${lastMonthRange().from.toISOString()}&to=${lastMonthRange().to.toISOString()}`
				),
				apiRequest(`/v1/brands/${brandId}/taut/campaigns?page=1`)
			]);
			if (creditsRes.ok) {
				const cd = await creditsRes.json();
				wallet = cd.wallet ?? null;
				tcConfig = cd.tc_config ?? null;
			}
			if (pubsRes.ok) {
				const pd = await pubsRes.json();
				pubs = pd.publications ?? [];
				pubsTotal = pd.total ?? 0;
			}
			if (reportsRes.ok) {
				const rd = await reportsRes.json();
				reports = { reach: rd.summary?.estimated_reach ?? 0, clicks: rd.summary?.outbound_clicks ?? 0 };
			}
			if (prevRes.ok) {
				const pd = await prevRes.json();
				prevReports = {
					reach: pd.summary?.estimated_reach ?? 0,
					clicks: pd.summary?.outbound_clicks ?? 0
				};
			}
			if (campRes.ok) {
				const cd = await campRes.json();
				campaigns = cd.campaigns ?? [];
			}
			const session = getStoredSession();
			if (session?.user?.display_name) userName = session.user.display_name;
		} catch {
			/* silent */
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void load();
		function onSuccess() {
			void load();
		}
		window.addEventListener('tauthub:topup-success', onSuccess);
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			window.removeEventListener('tauthub:topup-success', onSuccess);
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
		};
	});

	const available = $derived(wallet?.available_tc ?? 0);
	const reserved = $derived(wallet?.reserved_tc ?? 0);
	const reach = $derived(reports?.reach ?? 0);
	const clicks = $derived(reports?.clicks ?? 0);
	const prevReach = $derived(prevReports?.reach ?? 0);
	// Kartu hero: kampanye yang masih dikerjakan; kalau semuanya sudah final,
	// kampanye terbaru. Progres = media terbit dari total media (rumus daftar Kampanye).
	const heroCampaign = $derived(
		campaigns.find((c) => !DONE_STATUSES.has(c.taut_status ?? 'draft')) ?? campaigns[0] ?? null
	);
	const heroRunning = $derived(!!heroCampaign && !DONE_STATUSES.has(heroCampaign.taut_status ?? 'draft'));
	const heroPublished = $derived(heroCampaign?.stats?.published_media ?? 0);
	const heroTotal = $derived(
		Math.max(heroCampaign?.stats?.articles ?? 0, heroCampaign?.stats?.published_media ?? 0)
	);
	const heroPct = $derived(heroTotal > 0 ? Math.min(100, Math.round((heroPublished / heroTotal) * 100)) : 0);
	const heroStage = $derived(heroCampaign?.taut_status ?? 'draft');
	const heroTone = $derived(heroPublished > 0 ? 'success' : (STAGE_TONE[heroStage] ?? 'muted'));

	const reachDelta = $derived.by(() => {
		if (prevReach <= 0) return null;
		const pct = Math.round(((reach - prevReach) / prevReach) * 100);
		if (pct > 0) return `up:${pct}`;
		if (pct < 0) return `down:${Math.abs(pct)}`;
		return 'flat';
	});

	const fmt = (n: number) => n.toLocaleString(i18n.locale);

	async function createTautCampaign(mode: 'manual' | 'kilat') {
		const campaignName = createName.trim();
		if (campaignName.length < 2) {
			createErr = tr('Nama kampanye minimal 2 karakter.');
			return;
		}
		createBusy = true;
		createErr = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/taut/campaigns`, {
				method: 'POST',
				body: JSON.stringify({
					name: campaignName,
					starts_at: new Date().toISOString(),
					// 'kilat' dicatat di kampanye supaya router tahu halaman ini milik
					// jalur Kilat, bukan The Profiler (lihat migrasi 169).
					entry_mode: mode
				})
			});
			if (!res.ok) throw new Error(tr('Gagal membuat kampanye.'));
			const data = (await res.json()) as { id: string; public_id?: string | null };
			// URL memakai public_id 6 karakter (migrasi 170); API menerima keduanya.
			const target = campaignUrlId(data);
			await goto(
				mode === 'kilat'
					? `${fallbackBase}/taut-pintar/${target}/kilat`
					: `${fallbackBase}/taut-pintar/${target}/matchmaker?mode=manual`
			);
		} catch (reason) {
			createErr = reason instanceof Error ? reason.message : tr('Gagal membuat kampanye.');
			createBusy = false;
		}
	}

	async function handleTopup(tc: number, invoice?: TopUpInvoiceData | null) {
		topUpBusy = true;
		topUpError = '';
		topUpPayment = null;
		const key = topUpKey || (topUpKey = crypto.randomUUID());
		try {
			const result = await submitTopUp(`/v1/brands/${brandId}`, tc, key, invoice);
			if (result.status === 'paid') {
				topUpKey = crypto.randomUUID();
				showToast(tr('Top Up berhasil! Saldo telah diperbarui.'), 'success');
				topUpOpen = false;
				window.dispatchEvent(new CustomEvent('tauthub:topup-success'));
				return;
			}
			if (result.status === 'expired' || result.status === 'cancelled' || result.status === 'failed') {
				topUpKey = crypto.randomUUID();
				topUpError = 'Pembayaran sebelumnya batal atau kedaluwarsa. Silakan coba lagi.';
				return;
			}
			topUpPayment = { payment_url: result.payment_url, amount_idr: result.amount_idr, tc: result.tc };
			if (!result.payment_url) {
				showToast('Top Up berhasil!', 'success');
				topUpOpen = false;
				window.dispatchEvent(new CustomEvent('tauthub:topup-success'));
				topUpKey = crypto.randomUUID();
			}
		} catch {
			topUpError = 'Terjadi kesalahan.';
		} finally {
			topUpBusy = false;
		}
	}

	let insightIndex = $state(0);
	const insightMessages = $derived([
		tr('Kinerja kampanye Anda menunjukkan tren positif. Bulan ini tercatat 13 publikasi aktif yang terus memperluas jangkauan brand ke audiens yang relevan.'),
		tr('Berdasarkan profil Anda, AI Matchmaker menemukan 3 media mainstream baru dengan audiens yang sangat cocok untuk produk Anda. Selain itu, artikel terakhir Anda berhasil lolos Gatekeeper tanpa revisi.')
	]);

	$effect(() => {
		const timer = setInterval(() => {
			insightIndex = (insightIndex + 1) % insightMessages.length;
		}, 2600);
		return () => clearInterval(timer);
	});
</script>

<PageTitle title={tr("Beranda Klien")} />

{#snippet statusBadge(status: string, label: string)}
	{@const color = STATUS_COLORS[status] ?? 'gray'}
	{@const isLive = ['processing', 'queued', 'waiting_partner', 'retrying', 'pending'].includes(status)}
	{@const isDone = ['published', 'completed', 'partner_approved'].includes(status)}
	<span class="statusBadge badge-{color}">
		{#if isLive}
			<span class="statusPulseDot" aria-hidden="true"></span>
		{:else if isDone}
			<CircleCheck size={12} strokeWidth={2.4} />
		{:else}
			<span class="statusStaticDot" aria-hidden="true"></span>
		{/if}
		<span>{label}</span>
	</span>
{/snippet}

{#if loading}
	<div class="flex items-center justify-center p-12 text-sm text-muted-foreground" aria-busy="true">{tr("Memuat overview…")}</div>
{:else}
	<div class="wrap">
		<!-- ── HERO ─────────────────────────────────────────────────────── -->
		<section class="hero heroBlend">
			<GradientWave colors={HERO_WAVE} />
			<div class="heroScrim" aria-hidden="true"></div>
			<div class="heroContent">
					<h1 class="heroTitle">
						{welcomeParts.head}<span class="heroNoWrap">{welcomeParts.tail} <span class="heroWave">{WAVE_EMOJI}</span></span>
					</h1>
					<p class="heroSubtitle">{i18n.t.dashboard.subtitle}</p>
					<div class="heroBtns">
						<button class="heroCTA" type="button" onclick={() => goto(`${fallbackBase}/media`)}>
							<Globe size={18} strokeWidth={2.4} />
							<span>{i18n.t.dashboard.open_media_catalog}</span>
						</button>
						<!-- CTA ini mengarah ke daftar Kampanye (sebelumnya membuka modal
						     create di halaman ini; modalnya masih ada di bawah). -->
						<button
							class="heroCTAGhost"
							type="button"
							onclick={() => goto(`${fallbackBase}/campaigns`)}
						>
							<Sparkles size={18} strokeWidth={2.4} />
							<span>{i18n.t.dashboard.create_campaign}</span>
						</button>
					</div>
				</div>
				<div class="heroVisual">
					{#if heroCampaign}
						<a class="heroCampaign" href={`${fallbackBase}/taut-pintar/${campaignUrlId(heroCampaign)}`}>
							<span class="heroCampaignTop">
								<span class="heroCampaignLabel">{heroRunning ? tr('Kampanye berjalan') : tr('Kampanye terakhir')}</span>
								<span class="heroCampaignStage">
									<i class="heroCampaignDot heroCampaignDot-{heroTone}"></i>{STAGE_LABEL[heroStage] ?? heroStage}
								</span>
							</span>
							<span class="heroCampaignName" title={heroCampaign.name}>{heroCampaign.name}</span>
							{#if heroTotal > 0}
								<span class="heroCampaignNumbers">
									<span class="heroCampaignFigure">{heroPublished}<span class="heroCampaignOf"> / {heroTotal}</span></span>
									<span class="heroCampaignCaption">{tr('media terbit')}</span>
								</span>
								<span
									class="heroCampaignBar"
									role="img"
									aria-label="{heroPublished} {tr('Terbit')} / {heroTotal} {tr('Total')}"
								>
									<span class="heroCampaignBarFill" class:heroCampaignBarFillDone={heroPct === 100} style="width: {heroPct}%"></span>
								</span>
							{:else}
								<span class="heroCampaignCaption">{tr('Belum ada media dipilih')}</span>
							{/if}
							<span class="heroCampaignOpen">{tr('Buka Kampanye')}<ArrowRight size={14} strokeWidth={2} /></span>
						</a>
					{:else}
						<div class="heroCampaign heroCampaignEmpty">
							<span class="heroCampaignLabel">{tr('Kampanye')}</span>
							<span class="heroCampaignName">{tr('Belum ada kampanye')}</span>
							<span class="heroCampaignCaption">{tr('Kampanye yang Anda buat tampil di sini, lengkap dengan progres medianya.')}</span>
						</div>
					{/if}
				</div>
		</section>

		<!-- ── METRIC CARDS ─────────────────────────────────────────────── -->
		<div class="metricGrid">
			<div class="metricCard">
				<div class="metricHeader">
					<span class="metricTitle">{i18n.t.dashboard.available_balance}</span>
					<div class="metricIcon iconBlue"><Coins size={16} strokeWidth={2.4} /></div>
				</div>
				<div class="metricValueRow">
					<span class="metricValue">{fmt(available)}</span>
					<span class="metricUnit">{i18n.t.common.credits}</span>
				</div>
				<span class="metricDesc">{i18n.t.dashboard.balance_desc}</span>
			</div>

			<div class="metricCard">
				<div class="metricHeader">
					<span class="metricTitle">{i18n.t.dashboard.reserved_credit}</span>
					<div class="metricIcon iconBlue"><Lock size={16} strokeWidth={2.4} /></div>
				</div>
				<div class="metricValueRow">
					<span class="metricValue">{fmt(reserved)}</span>
					<span class="metricUnit">{i18n.t.common.credits}</span>
				</div>
				<span class="metricDesc">{i18n.t.dashboard.reserved_desc}</span>
			</div>

			<div class="metricCard">
				<div class="metricHeader">
					<span class="metricTitle">{i18n.t.dashboard.estimated_reach}</span>
					<div class="metricIcon iconBlue"><TrendingUp size={16} strokeWidth={2.4} /></div>
				</div>
				<div class="metricValueRow">
					<span class="metricValue">{fmt(reach)}</span>
				</div>
				<span
					class="metricDesc metricDelta"
					class:metricDeltaUp={reachDelta?.startsWith('up')}
					class:metricDeltaDown={reachDelta?.startsWith('down')}
				>
					{reachDelta === null
						? '—'
						: reachDelta === 'flat'
							? i18n.t.dashboard.reach_delta_flat
							: reachDelta.startsWith('up')
								? i18n.t.dashboard.reach_delta_up.replace('{n}', reachDelta.split(':')[1])
								: i18n.t.dashboard.reach_delta_down.replace('{n}', reachDelta.split(':')[1])}
				</span>
			</div>

			<div class="metricCard">
				<div class="metricHeader">
					<span class="metricTitle">{i18n.t.dashboard.total_clicks}</span>
					<div class="metricIcon iconBlue"><FileText size={16} strokeWidth={2.4} /></div>
				</div>
				<div class="metricValueRow">
					<span class="metricValue">{fmt(clicks)}</span>
				</div>
				<span class="metricDesc">{i18n.t.dashboard.clicks_desc}</span>
			</div>
		</div>

		<!-- ── INSIGHTS + FINANCE ───────────────────────────────────────── -->
		<div class="insightsFinanceRow">
			<section class="insights">
				<div class="insightsIconBox">
					<Lightbulb size={24} strokeWidth={2.4} color="#ffffff" />
				</div>
				<div class="insightsContent">
					<div class="insightsTitleRow">
						<h3 class="insightsTitle">{i18n.t.dashboard.insights_title}</h3>
						<span class="insightsBadge">{i18n.t.dashboard.insights_badge}</span>
					</div>
					<div class="insightsRolling">
						{#each insightMessages as msg, idx (idx)}
							<span
								class="insightsRollingItem"
								class:insightsRollingItemActive={idx === insightIndex}
								class:insightsRollingItemExit={idx ===
									(insightIndex - 1 + insightMessages.length) % insightMessages.length}
							>
								{msg}
							</span>
						{/each}
					</div>
				</div>
				<img src="/images/ai-agent.webp" alt="" class="aiAgentImgDirect" />
			</section>

			<div class="financeCard">
				<div class="financeHeader">
					<h3 class="sectionTitle" style="margin: 0;">{i18n.t.dashboard.quick_finance_title}</h3>
					<div class="financeHeaderIcon">
						<Sparkles size={14} strokeWidth={2.4} />
					</div>
				</div>
				<div class="financeSubheader">
					<Coins size={18} strokeWidth={2.4} style="color: var(--amber-9, #d97706);" />
					<span class="financeSubLabel">{i18n.t.dashboard.total_active_balance}</span>
				</div>
				<div class="financeValueRow">
					<span class="financeValue">{fmt(available)}</span>
					<span class="financeUnit">{i18n.t.common.credits}</span>
				</div>
				<button
					type="button"
					class="app-btn app-btn-primary w-full mt-4"
					onclick={() => {
						topUpPayment = null;
						topUpError = '';
						topUpOpen = true;
					}}
				>
					{i18n.t.dashboard.topup}
				</button>
				<div class="financeDivider"></div>
				<div class="financeLinks">
					<a class="ghostLink" href={`${fallbackBase}/credits`}>
						<FileText size={14} strokeWidth={2.4} />
						<span>{i18n.t.dashboard.tx_history}</span>
					</a>
					<a class="ghostLink" href={`${fallbackBase}/invoices`}>
						<FileText size={14} strokeWidth={2.4} />
						<span>{i18n.t.dashboard.claim_pph}</span>
					</a>
				</div>
			</div>
		</div>

		<!-- ── 2-COLUMN (QUEUES & HELP) ─────────────────────────────────── -->
		<div class="twoCol">
			<!-- LEFT: PUBLICATION QUEUE -->
			<section>
				<h2 class="sectionTitle">{i18n.t.dashboard.queue_title}</h2>
				{#if pubs.length === 0}
					<div class="emptyCard">
						<p>{i18n.t.dashboard.queue_empty}</p>
					</div>
				{:else}
					<div class="queueTable">
						<div class="queueHeader">
							<span>{i18n.t.dashboard.queue_col_campaign}</span>
							<span>{i18n.t.dashboard.queue_col_media}</span>
							<span>{i18n.t.dashboard.queue_col_status}</span>
							<span>{tr("Biaya")}</span>
						</div>
							{#each pubs as r (r.id)}
								<a class="queueRow" href={`${fallbackBase}/publications?pub=${r.id}`}>
									<div class="queueCell queueCellTitle font-medium">
										<span class="truncate block">{r.document_title || '—'}</span>
									</div>
									<div class="queueCell queueCellMedia text-muted-foreground">
										<span class="truncate block">{r.media_name || '—'}</span>
									</div>
									<div class="queueCell queueCellStatus">
										{@render statusBadge(r.status, (i18n.t.status as Record<string, string>)[r.status] ?? r.status)}
									</div>
									<div class="queueCell queueCellPrice font-medium">
										<Coins size={13} class="queueCoinIcon" />
										<span>{tcLabel(r.client_price ?? 0, tcConfig?.rate_idr ?? wallet?.rate_idr ?? 1000)}</span>
									</div>
								</a>
							{/each}
					</div>
				{/if}
			</section>

			<!-- RIGHT: HELP -->
			<div class="helpCard">
				<div class="helpHeader">
					<h3 class="sectionTitle" style="margin: 0;">{i18n.t.dashboard.help_title}</h3>
					<div class="helpHeaderIcon">
						<BookOpen size={16} strokeWidth={2.4} />
					</div>
				</div>
				<div class="helpList">
					<button class="ghostLinkHelp" type="button" onclick={() => openHelp('how')}>
						<div class="helpItemIcon">
							<BookOpen size={18} strokeWidth={2.4} />
						</div>
						<div class="helpItemContent">
							<div class="helpItemTitle">{i18n.t.dashboard.help_how_title}</div>
							<div class="helpItemDesc">{i18n.t.dashboard.help_how_desc}</div>
						</div>
						<ArrowRight size={14} class="helpArrow" />
					</button>
					<button class="ghostLinkHelp" type="button" onclick={() => openHelp('gate')}>
						<div class="helpItemIcon">
							<ShieldCheck size={18} strokeWidth={2.4} />
						</div>
						<div class="helpItemContent">
							<div class="helpItemTitle">{i18n.t.dashboard.help_gate_title}</div>
							<div class="helpItemDesc">{i18n.t.dashboard.help_gate_desc}</div>
						</div>
						<ArrowRight size={14} class="helpArrow" />
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}

<HelpModal bind:open={helpOpen} tab={helpTab} />

<!-- ── DIALOG BUAT KAMPANYE ──────────────────────────────────────── -->
{#if createOpen}
	<div
		class="dialogOverlay"
		role="presentation"
		onclick={() => {
			if (!createBusy) {
				createOpen = false;
				createMode = null;
			}
		}}
	>
		<dialog
			open
			class="dialogBox"
			aria-label={tr("Buat Kampanye")}
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => {
				if (e.key === 'Escape' && !createBusy) {
					createOpen = false;
					createMode = null;
				}
			}}
		>
			<div class="dialogHeader">
				<h2 class="dialogTitle">{tr("Buat Kampanye")}</h2>
				<button
					type="button"
					class="dialogClose"
					aria-label={tr("Tutup")}
					disabled={createBusy}
					onclick={() => {
						createOpen = false;
						createMode = null;
					}}
				>
					<X size={16} />
				</button>
			</div>
			<p class="dialogDesc">{tr("Pilih cara kamu membuat kampanye distribusi konten.")}</p>

			<div class="dialogField">
				<label class="dialogLabel" for="new-campaign-name">{tr("Nama Kampanye")}<span class="requiredStar">*</span>
				</label>
				<input
					id="new-campaign-name"
					class="app-input"
					placeholder={tr("Contoh: Launching Produk Q3 2026")}
					bind:value={createName}
					oninput={() => {
						if (createErr) createErr = '';
					}}
				/>
			</div>

			{#if createErr}
				<p class="dialogErr">{createErr}</p>
			{/if}

			<div class="modeGrid">
				<button
					type="button"
					onclick={() => (createMode = 'manual')}
					disabled={createBusy}
					class="modeCard"
					class:modeCardActive={createMode === 'manual'}
				>
					<div class="modeCardTitle">
						<FileText size={16} strokeWidth={2.4} color="var(--muted-foreground)" />
						<strong>Manual</strong>
					</div>
					<p class="modeCardDesc">{tr("The Writer tanpa AI: siapkan media & unggah artikel tulisanmu sendiri.")}</p>
				</button>
				<button
					type="button"
					onclick={() => (createMode = 'kilat')}
					disabled={createBusy}
					class="modeCard"
					class:modeCardActive={createMode === 'kilat'}
				>
					<div class="modeCardTitle">
						<Zap size={16} strokeWidth={2.4} color="var(--primary)" />
						<strong>Kampanye Kilat</strong>
					</div>
					<p class="modeCardDesc">{tr("Cukup URL website + anggaran. AI menyusun ringkasan & memilih medianya, lalu kamu langsung ke produksi artikel.")}</p>
				</button>
			</div>

			<div class="dialogActions">
				<button
					type="button"
					class="app-btn app-btn-primary"
					disabled={createBusy || !createMode}
					onclick={() => createMode && void createTautCampaign(createMode)}
				>
					{createBusy ? 'Memproses…' : 'Mulai Sekarang'}
				</button>
			</div>
		</dialog>
	</div>
{/if}

<!-- ── DIALOG TOP UP SALDO ────────────────────────────────────────── -->
{#if topUpOpen}
	<TopUpModal
		rateIdr={tcConfig?.rate_idr ?? wallet?.rate_idr ?? 1000}
		packages={tcConfig?.topup_packages ?? []}
		busy={topUpBusy}
		error={topUpError}
		payment={topUpPayment}
		onSelectPackage={(tc, invoice) => void handleTopup(tc, invoice)}
		onResetPayment={() => (topUpPayment = null)}
		onClose={() => (topUpOpen = false)}
	/>
{/if}

<style>
	.wrap {
		display: flex;
		flex-direction: column;
		gap: 24px;
		padding-bottom: 8px;
	}

	/* Hero */
	.hero {
	position: relative;
	display: grid;
	grid-template-columns: minmax(0, 1fr) auto;
	align-items: center;
	gap: 32px;
	min-height: 248px;
	overflow: hidden;
	border-radius: 16px;
	padding: 36px 40px;
	animation: heroIn 420ms cubic-bezier(0.2, 0.8, 0.2, 1) both;
		border: 1px solid var(--border);
		box-shadow: 0 4px 20px rgba(15, 23, 42, 0.05);
	}
	.heroContent {
	max-width: 620px;
	position: relative;
	z-index: 2;
	}
	@keyframes heroIn {
		from {
			opacity: 0;
			transform: translateY(8px);
		}
		to {
			opacity: 1;
			transform: none;
		}
	}
		.heroTitle {
			font-size: 30px;
			font-weight: 600;
			line-height: 38px;
			letter-spacing: -0.01em;
			color: #ffffff;
			margin: 0 0 8px;
			text-shadow: 0 1px 2px rgba(0, 0, 0, 0.25);
			word-break: break-word;
		}
		/* Lambaian tangan: hanya emoji yang bergerak, dengan jeda di antara
		   lambaian supaya tidak terus bergoyang. */
		.heroNoWrap {
			white-space: nowrap;
		}
		.heroWave {
			display: inline-block;
			transform-origin: 72% 82%;
			animation: heroWave 3.2s ease-in-out infinite;
		}
		@keyframes heroWave {
			0%, 52% { transform: rotate(0deg); }
			58% { transform: rotate(16deg); }
			64% { transform: rotate(-7deg); }
			70% { transform: rotate(13deg); }
			76% { transform: rotate(-4deg); }
			82% { transform: rotate(8deg); }
			88%, 100% { transform: rotate(0deg); }
		}
		@media (prefers-reduced-motion: reduce) {
			.heroWave { animation: none; }
		}
		.heroSubtitle {
			font-size: 14px;
			line-height: 22px;
			color: rgba(255, 255, 255, 0.88);
			margin: 0 0 24px;
			max-width: 520px;
			text-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
		}
		.heroBtns {
			display: flex;
			flex-wrap: wrap;
			gap: 12px;
		}
		.heroCTA {
			display: inline-flex;
			align-items: center;
			gap: 8px;
			border-radius: 9px;
			padding: 10px 16px;
			min-height: 44px;
			font-size: 14px;
			font-weight: 600;
			background: #ffffff;
			color: var(--primary);
			border: none;
			cursor: pointer;
			touch-action: manipulation;
			transition:
				transform 0.18s cubic-bezier(0.2, 0.8, 0.2, 1),
				box-shadow 0.18s ease,
				background-color 0.18s ease;
		}
		.heroCTA:hover {
			background: #eef2ff;
			transform: translateY(-2px);
			box-shadow: 0 8px 20px rgba(27, 103, 254, 0.22);
		}
		.heroCTA:active {
			transform: none;
			box-shadow: none;
		}
		.heroCTAGhost {
			display: inline-flex;
			align-items: center;
			gap: 8px;
			border-radius: 9px;
			padding: 10px 16px;
			min-height: 44px;
			font-size: 14px;
			font-weight: 600;
			background: rgba(255, 255, 255, 0.18);
			color: #ffffff;
			border: 1px solid rgba(255, 255, 255, 0.3);
			backdrop-filter: blur(8px);
			cursor: pointer;
			touch-action: manipulation;
		transition:
			transform 0.18s cubic-bezier(0.2, 0.8, 0.2, 1),
			box-shadow 0.18s ease,
			background-color 0.18s ease,
			border-color 0.18s ease;
	}
	.heroCTAGhost:hover {
		background: rgba(255, 255, 255, 0.28);
		transform: translateY(-2px);
		border-color: rgba(255, 255, 255, 0.55);
		box-shadow: 0 8px 20px rgba(16, 24, 40, 0.18);
	}
	.heroCTAGhost:active {
		transform: none;
		box-shadow: none;
	}

	.heroVisual {
		position: relative;
		z-index: 2;
		display: flex;
	}
	/* Kartu kaca di kanan hero: progres kampanye. */
	.heroCampaign {
		display: flex;
		flex-direction: column;
		gap: 8px;
		width: 264px;
		padding: 16px 18px;
		border-radius: 16px;
		background: rgba(240, 246, 255, 0.78);
		border: 1px solid rgba(255, 255, 255, 0.9);
		backdrop-filter: blur(16px);
		-webkit-backdrop-filter: blur(16px);
		box-shadow:
			inset 0 1px 0 rgba(255, 255, 255, 0.8),
			0 12px 32px rgba(11, 27, 74, 0.22);
		color: #0b1b4a;
		text-decoration: none;
		transition:
			transform 0.18s cubic-bezier(0.2, 0.8, 0.2, 1),
			box-shadow 0.18s ease;
	}
	a.heroCampaign:hover {
		transform: translateY(-2px);
		box-shadow:
			inset 0 1px 0 rgba(255, 255, 255, 0.8),
			0 16px 36px rgba(11, 27, 74, 0.3);
	}
	a.heroCampaign:focus-visible {
		outline: 2px solid #ffffff;
		outline-offset: 2px;
	}
	.heroCampaignTop {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
	}
	.heroCampaignLabel {
		font-size: 13px;
		line-height: 20px;
		font-weight: 500;
		color: rgba(11, 27, 74, 0.68);
	}
	.heroCampaignStage {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		line-height: 18px;
		font-weight: 500;
		color: rgba(11, 27, 74, 0.78);
		white-space: nowrap;
	}
	.heroCampaignDot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: var(--muted-foreground);
	}
	.heroCampaignDot-accent {
		background: var(--primary);
	}
	.heroCampaignDot-success {
		background: var(--success);
	}
	.heroCampaignDot-warning {
		background: var(--warning);
	}
	.heroCampaignName {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		line-clamp: 2;
		-webkit-box-orient: vertical;
		overflow: hidden;
		font-size: 15px;
		line-height: 20px;
		font-weight: 600;
		color: #0b1b4a;
	}
	.heroCampaignNumbers {
		display: flex;
		align-items: baseline;
		gap: 8px;
		margin-top: 4px;
	}
	.heroCampaignFigure {
		font-size: 28px;
		line-height: 32px;
		font-weight: 600;
		letter-spacing: -0.02em;
		font-variant-numeric: tabular-nums;
		color: #0b1b4a;
	}
	.heroCampaignOf {
		font-size: 16px;
		font-weight: 500;
		letter-spacing: 0;
		color: rgba(11, 27, 74, 0.55);
	}
	.heroCampaignCaption {
		font-size: 13px;
		line-height: 20px;
		color: rgba(11, 27, 74, 0.68);
	}
	.heroCampaignBar {
		display: block;
		height: 6px;
		border-radius: 999px;
		background: rgba(11, 27, 74, 0.1);
		overflow: hidden;
	}
	.heroCampaignBarFill {
		display: block;
		height: 100%;
		border-radius: inherit;
		background: var(--primary);
		transition: width 0.5s cubic-bezier(0.2, 0.8, 0.2, 1);
	}
	.heroCampaignBarFillDone {
		background: var(--success);
	}
	.heroCampaignOpen {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		margin-top: 2px;
		font-size: 13px;
		line-height: 20px;
		font-weight: 500;
		color: var(--primary);
	}
	.heroCampaignEmpty {
		gap: 4px;
	}

	/* Lapisan gelap di kiri: teks putih tetap terbaca di atas gelombang terang. */
	.heroScrim {
		position: absolute;
		inset: 0;
		z-index: 1;
		pointer-events: none;
		background: linear-gradient(
			to right,
			rgba(11, 27, 74, 0.5) 0%,
			rgba(11, 27, 74, 0.3) 45%,
			rgba(11, 27, 74, 0) 72%
		);
	}

	/* Hero varian blend: 4 warna FeralUI (indigo gelap kiri untuk teks putih,
	   lagoon/misted sky kanan untuk kartu kaca terang). */
	.heroBlend {
		background-color: #0b1b4a;
		border-color: rgba(11, 27, 74, 0.18);
		box-shadow: 0 4px 20px rgba(11, 27, 74, 0.14);
	}
	.heroBlend .heroTitle {
		color: #ffffff;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.18);
	}
	.heroBlend .heroSubtitle {
		color: rgba(255, 255, 255, 0.86);
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.14);
	}
	.heroBlend .heroCTA {
		background: #ffffff;
		color: #0b1b4a;
	}
	.heroBlend .heroCTA:hover {
		background: #eef2ff;
		box-shadow: 0 8px 20px rgba(11, 27, 74, 0.28);
	}
	.heroBlend .heroCTAGhost {
		background: rgba(255, 255, 255, 0.16);
		color: #ffffff;
		border-color: rgba(255, 255, 255, 0.32);
		backdrop-filter: blur(8px);
		-webkit-backdrop-filter: blur(8px);
	}
	.heroBlend .heroCTAGhost:hover {
		background: rgba(255, 255, 255, 0.26);
		border-color: rgba(255, 255, 255, 0.55);
		box-shadow: 0 8px 20px rgba(16, 24, 40, 0.18);
	}

	/* Metric Grid */
	.metricGrid {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 16px;
	}
	.metricCard {
		position: relative;
		border: 1px solid var(--border);
		border-radius: 16px;
		background: var(--card);
		padding: 20px;
		box-shadow: 0 1px 3px rgba(15, 23, 42, 0.04);
		transition:
			transform 0.18s cubic-bezier(0.2, 0.8, 0.2, 1),
			box-shadow 0.18s ease;
		overflow: hidden;
	}
	.metricCard:hover {
		transform: translateY(-2px);
		box-shadow: 0 8px 24px rgba(15, 23, 42, 0.08);
	}
	.metricHeader {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		margin-bottom: 8px;
	}
	.metricTitle {
		color: var(--muted-foreground);
		font-size: 13px;
		font-weight: 500;
		line-height: 20px;
		}
		.metricIcon {
		width: 30px;
		height: 30px;
		border-radius: 999px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}
	.iconBlue {
		background: var(--accent);
		color: var(--primary);
	}
	.metricValueRow {
		display: flex;
		align-items: baseline;
		gap: 8px;
	}
	.metricValue {
	font-size: 26px;
	font-weight: 600;
	letter-spacing: -0.01em;
	line-height: 32px;
		font-variant-numeric: tabular-nums;
	}
	.metricUnit {
		font-size: 12px;
		color: var(--muted-foreground);
	}
	.metricDesc {
		margin-top: 8px;
		display: block;
		font-size: 12px;
		line-height: 1.45;
		color: var(--muted-foreground);
	}
	.metricDeltaUp {
	color: var(--success);
	}
	.metricDeltaDown {
	color: var(--destructive);
	}

	/* Insights + Finance */
	.insightsFinanceRow {
		display: grid;
		grid-template-columns: 2fr 1fr;
		gap: 24px;
		align-items: stretch;
	}
	.insights {
		position: relative;
		display: flex;
		align-items: center;
		gap: 20px;
		border-radius: 16px;
		border: 1px solid var(--border);
		background: var(--card);
		padding: 24px 28px;
		overflow: hidden;
	}
	.insights::before {
		content: '';
		position: absolute;
		inset: 0;
		border-radius: inherit;
		background: url('/images/insights-bg.webp') center/cover no-repeat;
		opacity: 0.15;
		pointer-events: none;
		z-index: 0;
	}
	.insights > * {
		position: relative;
		z-index: 1;
	}
	.insightsIconBox {
		width: 44px;
		height: 44px;
		border-radius: 12px;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		background: var(--primary);
	}
	.insightsContent {
		flex: 1;
		min-width: 0;
		overflow: hidden;
	}
	.insightsTitleRow {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}
	.insightsTitle {
		margin: 0;
		font-size: 16px;
		font-weight: 600;
		}
		.insightsBadge {
		display: inline-flex;
		align-items: center;
		border-radius: 999px;
		padding: 1px 8px;
		font-size: 11px;
		font-weight: 500;
		background: var(--accent);
		color: var(--accent-foreground);
		}
	.insightsRolling {
		position: relative;
		min-height: 44px;
		overflow: hidden;
		margin-top: 8px;
	}
	.insightsRollingItem {
		position: absolute;
		inset: 0;
		display: block;
		font-size: 14px;
		line-height: 1.6;
		color: var(--muted-foreground);
		transform: translateY(100%);
		opacity: 0;
		transition:
			transform 0.6s cubic-bezier(0.34, 1.2, 0.64, 1),
			opacity 0.45s ease;
	}
	.insightsRollingItemActive {
		transform: translateY(0);
		opacity: 1;
	}
	.insightsRollingItemExit {
		transform: translateY(-100%);
		opacity: 0;
	}
	.aiAgentImgDirect {
		flex-shrink: 0;
		width: 160px;
		max-height: 140px;
		height: auto;
		object-fit: contain;
	}

	.financeCard {
		border: 1px solid var(--border);
		border-radius: 16px;
		box-shadow: 0 1px 3px rgba(15, 23, 42, 0.04);
		padding: 24px;
		background: var(--card);
		display: flex;
		flex-direction: column;
	}
	.financeHeader {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 12px;
	}
	.financeHeaderIcon {
		width: 28px;
		height: 28px;
		border-radius: 8px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		background: var(--accent);
		color: var(--primary);
	}
	.financeSubheader {
		display: flex;
		align-items: center;
		gap: 8px;
		margin-bottom: 4px;
	}
	.financeSubLabel {
		font-size: 12px;
		color: var(--muted-foreground);
	}
	.financeValueRow {
		display: flex;
		align-items: baseline;
		gap: 8px;
	}
	.financeValue {
	font-size: 26px;
	font-weight: 600;
	letter-spacing: -0.01em;
	line-height: 32px;
		font-variant-numeric: tabular-nums;
	}
	.financeUnit {
		font-size: 13px;
		color: var(--muted-foreground);
	}
	.financeDivider {
		height: 1px;
		background: var(--border);
		margin: 20px 0 16px;
	}
	.financeLinks {
		display: flex;
		justify-content: space-between;
		gap: 8px;
		margin-top: auto;
	}
	.ghostLink {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 8px;
		border-radius: 8px;
		font-size: 12px;
		font-weight: 550;
		color: var(--foreground);
		text-decoration: none;
		transition: background-color 0.15s ease;
	}
	.ghostLink:hover {
		background-color: var(--secondary);
	}

	/* Two Column (Queues & Help) */
	.twoCol {
		display: grid;
		grid-template-columns: 2fr 1fr;
		gap: 24px;
		align-items: start;
	}
	.sectionTitle {
	font-size: 16px;
	line-height: 24px;
	font-weight: 600;
		color: var(--foreground);
		margin: 0 0 12px;
	}
	.queueTable {
	border: 1px solid var(--border);
	border-radius: 16px;
		overflow: hidden;
		background: var(--card);
	}
	.queueHeader,
	.queueRow {
		display: grid;
		grid-template-columns: 2fr 1.4fr 1.2fr 0.8fr;
		gap: 16px;
		align-items: center;
		padding: 12px 20px;
		font-size: 14px;
		}
		.queueHeader {
		background: var(--muted);
		border-bottom: 1px solid var(--border);
		font-size: 12px;
		font-weight: 500;
		color: var(--muted-foreground);
		}
		.queueRow {
			min-height: 52px;
			color: var(--foreground);
			text-decoration: none;
			transition: background-color 0.16s ease;
		}
		.queueRow:hover {
			background: var(--sidebar-hover, #f6f8fc);
		}
		.queueRow:focus-visible {
			outline: 2px solid var(--ring);
			outline-offset: -2px;
		}
		.queueRow + .queueRow {
		border-top: 1px solid var(--border);
	}
		.queueCell {
			min-width: 0;
		}
		.queueCellPrice {
			display: inline-flex;
			align-items: center;
			gap: 4px;
		}
		:global(.queueCoinIcon) {
			color: var(--amber-9, #d97706);
			flex-shrink: 0;
		}
		.emptyCard {
		border: 1px dashed var(--border);
		border-radius: 16px;
		padding: 24px;
		text-align: center;
		color: var(--muted-foreground);
		font-size: 13px;
		background: var(--card);
	}

	/* Status Badge */
	.statusBadge {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		border-radius: 999px;
		padding: 2px 8px;
		font-size: 12px;
		line-height: 18px;
		font-weight: 500;
		white-space: nowrap;
	}
	.badge-green {
		background: #ecfdf5;
		color: #047857;
	}
	.badge-blue {
		background: #eff6ff;
		color: #1d4ed8;
	}
	.badge-amber {
		background: #fffbeb;
		color: #b45309;
	}
	.badge-red {
		background: #fef2f2;
		color: #b91c1c;
	}
	.badge-gray {
		background: #f3f4f6;
		color: #4b5563;
	}
	.statusPulseDot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: currentColor;
		box-shadow: 0 0 0 0 currentColor;
		animation: statusPulse 1.6s infinite cubic-bezier(0.4, 0, 0.6, 1);
	}
	.statusStaticDot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: currentColor;
		opacity: 0.7;
	}
	@keyframes statusPulse {
		0% {
			transform: scale(0.95);
			opacity: 1;
		}
		50% {
			transform: scale(1.35);
			opacity: 0.5;
		}
		100% {
			transform: scale(0.95);
			opacity: 1;
		}
	}

	/* Help Card */
	.helpCard {
		border: 1px solid var(--border);
		border-radius: 16px;
		box-shadow: 0 1px 3px rgba(15, 23, 42, 0.04);
		padding: 24px;
		background: var(--card);
	}
	.helpHeader {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 12px;
	}
	.helpHeaderIcon {
		width: 28px;
		height: 28px;
		border-radius: 8px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		background: var(--accent);
		color: var(--primary);
	}
	.helpList {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.ghostLinkHelp {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 14px;
		width: 100%;
		padding: 12px 14px;
		border: none;
		background: transparent;
		font: inherit;
		border-radius: 12px;
		color: var(--foreground);
		text-align: left;
		text-decoration: none;
		cursor: pointer;
		transition: background-color 0.15s ease;
	}
	.ghostLinkHelp:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
	}
	.ghostLinkHelp:hover {
		background-color: var(--secondary);
	}
	.ghostLinkHelp:hover :global(.helpArrow) {
		transform: translateX(2px);
		color: var(--primary);
	}
	.helpItemIcon {
		width: 36px;
		height: 36px;
		border-radius: 10px;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		background: var(--accent);
		color: var(--primary);
	}
	.helpItemContent {
		flex: 1;
		min-width: 0;
	}
	.helpItemTitle {
	font-size: 14px;
	font-weight: 500;
		color: var(--foreground);
		line-height: 1.3;
	}
	.helpItemDesc {
		font-size: 12px;
		color: var(--muted-foreground);
		line-height: 1.4;
	}
	:global(.helpArrow) {
		transition:
			transform 0.15s ease,
			color 0.15s ease;
		flex-shrink: 0;
		color: var(--muted-foreground);
	}

	/* Dialog Buat Kampanye */
	.dialogOverlay {
		position: fixed;
		inset: 0;
		z-index: 60;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 16px;
		background: rgba(15, 23, 42, 0.45);
	}
	.dialogBox {
		position: relative;
		width: 100%;
		/* Dua kartu mode (Manual / Kampanye Kilat) butuh ruang supaya
		   deskripsinya tidak terpotong jadi satu kata per baris. */
		max-width: 768px;
		border-radius: 16px;
		border: 1px solid var(--border);
		background: var(--card);
		padding: 24px;
		box-shadow: var(--shadow-md);
		color: var(--foreground);
	}
	.dialogHeader {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}
	.dialogTitle {
		margin: 0;
		font-size: 18px;
		font-weight: 700;
	}
	.dialogClose {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border-radius: 8px;
		color: var(--muted-foreground);
	}
	.dialogClose:hover {
		background: var(--secondary);
		color: var(--foreground);
	}
	.dialogDesc {
		margin: 4px 0 16px;
		font-size: 13px;
		color: var(--muted-foreground);
	}
	.dialogField {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.dialogLabel {
		font-size: 13px;
		font-weight: 600;
	}
	.requiredStar {
		color: var(--destructive);
	}
	.dialogErr {
		margin-top: 8px;
		font-size: 12px;
		color: var(--destructive);
	}
	.modeGrid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 12px;
		margin-top: 16px;
	}
	.modeCard {
		text-align: left;
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 14px 16px;
		background: var(--muted);
		cursor: pointer;
		display: flex;
		flex-direction: column;
		gap: 8px;
		align-items: flex-start;
		transition:
			border-color 120ms ease,
			background-color 120ms ease,
			box-shadow 120ms ease;
	}
	.modeCard:hover:not(:disabled) {
		border-color: color-mix(in srgb, var(--primary) 40%, var(--border));
		background: color-mix(in srgb, var(--primary) 4%, var(--muted));
	}
	.modeCardActive {
		border-color: var(--primary) !important;
		background: color-mix(in srgb, var(--primary) 8%, var(--card)) !important;
		box-shadow: inset 0 0 0 1px var(--primary);
	}
	.modeCardTitle {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 14px;
	}
	.modeCardDesc {
		margin: 0;
		font-size: 11.5px;
		line-height: 1.45;
		color: var(--muted-foreground);
	}
	.dialogActions {
		display: flex;
		justify-content: flex-end;
		margin-top: 20px;
	}

		/* Responsive */
		@media (max-width: 1024px) {
		.hero {
			grid-template-columns: 1fr;
			min-height: 0;
			padding: 28px;
			/* Satu kolom: sisi indigo gelap di balik teks, bukan lagoon terang. */
			background-position: left center;
		}
		.heroVisual {
			justify-content: flex-start;
		}
		.metricGrid {
				grid-template-columns: repeat(2, minmax(0, 1fr));
			}
			.twoCol {
				grid-template-columns: 1fr;
			}
			.insightsFinanceRow {
				grid-template-columns: 1fr;
			}
		}
		@media (max-width: 767px) {
			.wrap {
				gap: 16px;
			}
			.hero {
			padding: 20px 16px;
			gap: 16px;
			}
			.heroScrim {
				background: linear-gradient(to bottom, rgba(11, 27, 74, 0.45) 0%, rgba(11, 27, 74, 0.3) 100%);
			}
			.heroTitle {
			font-size: clamp(22px, 6vw, 26px);
				line-height: 1.25;
				margin-bottom: 6px;
			}
			.heroSubtitle {
				font-size: 13px;
				line-height: 1.5;
				margin-bottom: 16px;
			}
			.heroBtns {
				flex-direction: column;
				width: 100%;
				gap: 10px;
			}
			.heroCTA,
			.heroCTAGhost {
				width: 100%;
				min-height: 44px;
				justify-content: center;
			}
			.heroVisual {
				display: block;
			}
			.heroCampaign {
				width: 100%;
				box-sizing: border-box;
				padding: 14px 16px;
				border-radius: 14px;
			}
			.heroCampaignFigure {
				font-size: 24px;
				line-height: 28px;
			}

			.metricGrid {
				grid-template-columns: repeat(2, minmax(0, 1fr));
				gap: 10px;
			}
			.metricCard {
				padding: 14px 12px;
				border-radius: 14px;
			}
			.metricHeader {
				margin-bottom: 6px;
			}
			.metricTitle {
				font-size: 12px;
			}
			.metricIcon {
				width: 26px;
				height: 26px;
			}
			.metricIcon :global(svg) {
				width: 14px;
				height: 14px;
			}
			.metricValue {
				font-size: 20px;
			}
			.metricUnit {
				font-size: 11px;
			}
			.metricDesc {
				font-size: 11px;
				margin-top: 4px;
				line-height: 1.35;
			}

			.insights {
				flex-direction: column;
				align-items: flex-start;
				padding: 18px 16px;
				border-radius: 16px;
				gap: 12px;
			}
			.insightsIconBox {
				width: 36px;
				height: 36px;
				border-radius: 10px;
			}
			.insightsIconBox :global(svg) {
				width: 20px;
				height: 20px;
			}
			.insightsTitle {
				font-size: 14px;
			}
			.insightsRolling {
				min-height: 58px;
				margin-top: 6px;
			}
			.insightsRollingItem {
				font-size: 13px;
				line-height: 1.5;
			}
			.aiAgentImgDirect {
				display: none;
			}

			.financeCard {
				padding: 18px 16px;
				border-radius: 16px;
			}
			.financeValue {
				font-size: 22px;
			}
			.financeDivider {
				margin: 16px 0 12px;
			}
			.financeLinks {
				flex-direction: column;
				gap: 8px;
			}
			.ghostLink {
				width: 100%;
				min-height: 44px;
				padding: 10px 14px;
				border: 1px solid var(--border);
				border-radius: 10px;
				justify-content: center;
				background: var(--muted);
				touch-action: manipulation;
			}
			.ghostLink:active {
				background: var(--secondary);
			}

			.queueHeader {
				display: none;
			}
			.queueRow {
				display: grid;
				grid-template-columns: 1fr auto;
				grid-template-areas:
					'title title'
					'media media'
					'status price';
				gap: 6px 12px;
				padding: 12px 14px;
			}
			.queueCellTitle {
				grid-area: title;
				font-size: 13px;
				font-weight: 600;
				line-height: 1.35;
			}
			.queueCellMedia {
				grid-area: media;
				font-size: 11.5px;
			}
			.queueCellStatus {
				grid-area: status;
				justify-self: start;
			}
			.queueCellPrice {
				grid-area: price;
				justify-self: end;
				font-size: 12px;
				color: var(--primary);
			}

			.helpCard {
				padding: 18px 16px;
				border-radius: 16px;
			}
			.ghostLinkHelp {
				min-height: 44px;
				padding: 10px 12px;
				border-radius: 10px;
				touch-action: manipulation;
			}
			.ghostLinkHelp:active {
				background: var(--border);
			}
			.helpItemIcon {
				width: 32px;
				height: 32px;
				border-radius: 8px;
			}
			.helpItemIcon :global(svg) {
				width: 16px;
				height: 16px;
			}
			.helpItemTitle {
				font-size: 13px;
			}
			.helpItemDesc {
				font-size: 11px;
			}

			.dialogBox {
				max-width: calc(100vw - 32px);
				padding: 20px 16px;
				border-radius: 16px;
			}
			.dialogClose {
				width: 44px;
				height: 44px;
				touch-action: manipulation;
			}
			.modeGrid {
				grid-template-columns: 1fr;
				gap: 10px;
			}
			.modeCard {
				padding: 12px 14px;
			}
			.dialogActions button {
				width: 100%;
				min-height: 44px;
			}
		}
		@media (max-width: 340px) {
			.metricGrid {
				grid-template-columns: 1fr;
			}
		}
	@media (prefers-reduced-motion: reduce) {
		.insightsRollingItem {
			transition: none !important;
		}
		.statusPulseDot {
			animation: none;
		}
		.heroCTA,
		.heroCTAGhost,
		.heroCampaign,
		.heroCampaignBarFill {
		transition: none;
		}
		.hero {
		animation: none;
		}
	}
</style>
