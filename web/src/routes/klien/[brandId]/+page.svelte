<script lang="ts">
	// Beranda Klien — port 1:1 dari overview-dashboard.tsx & overview-dashboard.module.css
	// app Next.js (bukan client-overview.tsx lama).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import {
		ArrowRight, BookOpen, CircleCheck, Coins, FileText, Globe,
		Lightbulb, Lock, ShieldCheck, Sparkles, TrendingUp, X
	} from '@lucide/svelte';
	import { apiRequest, getStoredSession, showToast, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { getI18n } from '$lib/i18n/i18n.svelte';
	import { submitTopUp, type TopUpInvoiceData } from '$lib/credits/topup-client';
	import TopUpModal from '$lib/components/credit/topup-modal.svelte';

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const fallbackBase = $derived(`/klien/${brandUrl}`);
	const { t, locale } = getI18n();

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

	function tcLabel(rupiah: number, rate: number): string {
		return Math.max(0, Math.round(Number(rupiah) / Math.max(1, rate))).toLocaleString('id-ID');
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
	let wallet = $state<Wallet | null>(null);
	let tcConfig = $state<{ rate_idr: number; topup_packages: Pkg[] } | null>(null);
	let reports = $state<{ reach: number; clicks: number } | null>(null);
	let prevReports = $state<{ reach: number; clicks: number } | null>(null);
	let pubs = $state<PubRow[]>([]);
	let pubsTotal = $state(0);
	let loading = $state(true);

	// Popup "Buat Kampanye"
	let createOpen = $state(false);
	let createBusy = $state(false);
	let createErr = $state('');
	let createName = $state('');
	let createMode = $state<'ai' | 'manual' | null>(null);

	// Popup Top Up
	let topUpOpen = $state(false);
	let topUpBusy = $state(false);
	let topUpPayment = $state<{ payment_url: string | null; amount_idr: number; tc: number } | null>(null);
	let topUpError = $state('');
	let topUpKey = '';

	async function load() {
		try {
			const [creditsRes, pubsRes, reportsRes, prevRes] = await Promise.all([
				apiRequest(`/v1/brands/${brandId}/credits?limit=1`),
				apiRequest(`/v1/brands/${brandId}/publications?limit=6`),
				apiRequest(
					`/v1/brands/${brandId}/reports?from=${startOfMonth().toISOString()}&to=${new Date().toISOString()}`
				),
				apiRequest(
					`/v1/brands/${brandId}/reports?from=${lastMonthRange().from.toISOString()}&to=${lastMonthRange().to.toISOString()}`
				)
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

	const reachDelta = $derived.by(() => {
		if (prevReach <= 0) return null;
		const pct = Math.round(((reach - prevReach) / prevReach) * 100);
		if (pct > 0) return `up:${pct}`;
		if (pct < 0) return `down:${Math.abs(pct)}`;
		return 'flat';
	});

	const fmt = (n: number) => n.toLocaleString(locale);

	async function createTautCampaign(mode: 'ai' | 'manual') {
		const campaignName = createName.trim();
		if (campaignName.length < 2) {
			createErr = 'Nama kampanye minimal 2 karakter.';
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
					entry_mode: mode
				})
			});
			if (!res.ok) throw new Error('Gagal membuat kampanye.');
			const data = (await res.json()) as { id: string };
			await goto(
				mode === 'ai'
					? `${fallbackBase}/taut-pintar/${data.id}/profiler`
					: `${fallbackBase}/taut-pintar/${data.id}/matchmaker?mode=manual`
			);
		} catch (reason) {
			createErr = reason instanceof Error ? reason.message : 'Gagal membuat kampanye.';
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
				showToast('Top Up berhasil! Saldo telah diperbarui.', 'success');
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
	const insightMessages = [
		'Kinerja kampanye Anda menunjukkan tren positif. Bulan ini tercatat 13 publikasi aktif yang terus memperluas jangkauan brand ke audiens yang relevan.',
		'Berdasarkan profil Anda, AI Matchmaker menemukan 3 media mainstream baru dengan audiens yang sangat cocok untuk produk Anda. Selain itu, artikel terakhir Anda berhasil lolos Gatekeeper tanpa revisi.'
	];

	$effect(() => {
		const timer = setInterval(() => {
			insightIndex = (insightIndex + 1) % insightMessages.length;
		}, 2600);
		return () => clearInterval(timer);
	});
</script>

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
	<div class="flex items-center justify-center p-12 text-sm text-muted-foreground" aria-busy="true">
		Memuat overview…
	</div>
{:else}
	<div class="wrap">
		<!-- ── HERO ─────────────────────────────────────────────────────── -->
		<section class="hero">
			<div class="heroContent">
				<h1 class="heroTitle">
					{t.dashboard.welcome.replace('{name}', (userName || 'User') + '!')}
				</h1>
				<p class="heroSubtitle">{t.dashboard.subtitle}</p>
				<div class="heroBtns">
					<button class="heroCTA" type="button" onclick={() => goto(`${fallbackBase}/media`)}>
						<Globe size={18} strokeWidth={2.4} />
						<span>{t.dashboard.open_media_catalog}</span>
					</button>
					<button
						class="heroCTAGhost"
						type="button"
						onclick={() => {
							createErr = '';
							createName = '';
							createOpen = true;
						}}
					>
						<Sparkles size={18} strokeWidth={2.4} />
						<span>{t.dashboard.create_campaign}</span>
					</button>
				</div>
			</div>
			<div class="heroVisual">
				<div class="heroSecurityCard">
					<div class="securityShieldCircle">
						<ShieldCheck size={24} strokeWidth={2.4} color="#34d399" />
					</div>
					<span class="securityTitle">{t.dashboard.security_title}</span>
					<span class="securityValue">100% {t.dashboard.security_passed}</span>
				</div>
			</div>
		</section>

		<!-- ── METRIC CARDS ─────────────────────────────────────────────── -->
		<div class="metricGrid">
			<div class="metricCard">
				<div class="metricHeader">
					<span class="metricTitle">{t.dashboard.available_balance}</span>
					<div class="metricIcon iconBlue"><Coins size={16} strokeWidth={2.4} /></div>
				</div>
				<div class="metricValueRow">
					<span class="metricValue">{fmt(available)}</span>
					<span class="metricUnit">{t.common.credits}</span>
				</div>
				<span class="metricDesc">{t.dashboard.balance_desc}</span>
			</div>

			<div class="metricCard">
				<div class="metricHeader">
					<span class="metricTitle">{t.dashboard.reserved_credit}</span>
					<div class="metricIcon iconBlue"><Lock size={16} strokeWidth={2.4} /></div>
				</div>
				<div class="metricValueRow">
					<span class="metricValue">{fmt(reserved)}</span>
					<span class="metricUnit">{t.common.credits}</span>
				</div>
				<span class="metricDesc">{t.dashboard.reserved_desc}</span>
			</div>

			<div class="metricCard">
				<div class="metricHeader">
					<span class="metricTitle">{t.dashboard.estimated_reach}</span>
					<div class="metricIcon iconBlue"><TrendingUp size={16} strokeWidth={2.4} /></div>
				</div>
				<div class="metricValueRow">
					<span class="metricValue">{fmt(reach)}</span>
				</div>
				<span class="metricDescReach">
					{reachDelta === null
						? '—'
						: reachDelta === 'flat'
							? t.dashboard.reach_delta_flat
							: reachDelta.startsWith('up')
								? t.dashboard.reach_delta_up.replace('{n}', reachDelta.split(':')[1])
								: t.dashboard.reach_delta_down.replace('{n}', reachDelta.split(':')[1])}
				</span>
			</div>

			<div class="metricCard">
				<div class="metricHeader">
					<span class="metricTitle">{t.dashboard.total_clicks}</span>
					<div class="metricIcon iconBlue"><FileText size={16} strokeWidth={2.4} /></div>
				</div>
				<div class="metricValueRow">
					<span class="metricValue">{fmt(clicks)}</span>
				</div>
				<span class="metricDesc">{t.dashboard.clicks_desc}</span>
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
						<h3 class="insightsTitle">{t.dashboard.insights_title}</h3>
						<span class="insightsBadge">{t.dashboard.insights_badge}</span>
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
					<h3 class="sectionTitle" style="margin: 0;">{t.dashboard.quick_finance_title}</h3>
					<div class="financeHeaderIcon">
						<Sparkles size={14} strokeWidth={2.4} />
					</div>
				</div>
				<div class="financeSubheader">
					<Coins size={18} strokeWidth={2.4} style="color: var(--amber-9, #d97706);" />
					<span class="financeSubLabel">{t.dashboard.total_active_balance}</span>
				</div>
				<div class="financeValueRow">
					<span class="financeValue">{fmt(available)}</span>
					<span class="financeUnit">{t.common.credits}</span>
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
					{t.dashboard.topup}
				</button>
				<div class="financeDivider"></div>
				<div class="financeLinks">
					<a class="ghostLink" href={`${fallbackBase}/credits`}>
						<FileText size={14} strokeWidth={2.4} />
						<span>{t.dashboard.tx_history}</span>
					</a>
					<a class="ghostLink" href={`${fallbackBase}/invoices`}>
						<FileText size={14} strokeWidth={2.4} />
						<span>{t.dashboard.claim_pph}</span>
					</a>
				</div>
			</div>
		</div>

		<!-- ── 2-COLUMN (QUEUES & HELP) ─────────────────────────────────── -->
		<div class="twoCol">
			<!-- LEFT: PUBLICATION QUEUE -->
			<section>
				<h2 class="sectionTitle">{t.dashboard.queue_title}</h2>
				{#if pubs.length === 0}
					<div class="emptyCard">
						<p>{t.dashboard.queue_empty}</p>
					</div>
				{:else}
					<div class="queueTable">
						<div class="queueHeader">
							<span>{t.dashboard.queue_col_campaign}</span>
							<span>{t.dashboard.queue_col_media}</span>
							<span>{t.dashboard.queue_col_status}</span>
							<span>Kredit</span>
						</div>
						{#each pubs as r (r.id)}
							<div class="queueRow">
								<div class="queueCell font-medium">
									<span class="truncate block">{r.document_title || '—'}</span>
								</div>
								<div class="queueCell text-muted-foreground">
									<span class="truncate block">{r.media_name || '—'}</span>
								</div>
								<div class="queueCell">
									{@render statusBadge(r.status, (t.status as Record<string, string>)[r.status] ?? r.status)}
								</div>
								<div class="queueCell font-medium">
									<span>{tcLabel(r.client_price ?? 0, tcConfig?.rate_idr ?? wallet?.rate_idr ?? 1000)}</span>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</section>

			<!-- RIGHT: HELP -->
			<div class="helpCard">
				<div class="helpHeader">
					<h3 class="sectionTitle" style="margin: 0;">{t.dashboard.help_title}</h3>
					<div class="helpHeaderIcon">
						<BookOpen size={16} strokeWidth={2.4} />
					</div>
				</div>
				<div class="helpList">
					<a class="ghostLinkHelp" href="/docs">
						<div class="helpItemIcon">
							<BookOpen size={18} strokeWidth={2.4} />
						</div>
						<div class="helpItemContent">
							<div class="helpItemTitle">{t.dashboard.help_how_title}</div>
							<div class="helpItemDesc">{t.dashboard.help_how_desc}</div>
						</div>
						<ArrowRight size={14} class="helpArrow" />
					</a>
					<a class="ghostLinkHelp" href="/docs">
						<div class="helpItemIcon">
							<ShieldCheck size={18} strokeWidth={2.4} />
						</div>
						<div class="helpItemContent">
							<div class="helpItemTitle">{t.dashboard.help_gate_title}</div>
							<div class="helpItemDesc">{t.dashboard.help_gate_desc}</div>
						</div>
						<ArrowRight size={14} class="helpArrow" />
					</a>
				</div>
			</div>
		</div>
	</div>
{/if}

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
			aria-label="Buat Kampanye"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => {
				if (e.key === 'Escape' && !createBusy) {
					createOpen = false;
					createMode = null;
				}
			}}
		>
			<div class="dialogHeader">
				<h2 class="dialogTitle">Buat Kampanye</h2>
				<button
					type="button"
					class="dialogClose"
					aria-label="Tutup"
					disabled={createBusy}
					onclick={() => {
						createOpen = false;
						createMode = null;
					}}
				>
					<X size={16} />
				</button>
			</div>
			<p class="dialogDesc">Pilih cara kamu membuat kampanye distribusi konten.</p>

			<div class="dialogField">
				<label class="dialogLabel" for="new-campaign-name">
					Nama Kampanye <span class="requiredStar">*</span>
				</label>
				<input
					id="new-campaign-name"
					class="app-input"
					placeholder="Contoh: Launching Produk Q3 2026"
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
					onclick={() => (createMode = 'ai')}
					disabled={createBusy}
					class="modeCard"
					class:modeCardActive={createMode === 'ai'}
				>
					<div class="modeCardTitle">
						<Sparkles size={16} strokeWidth={2.4} color="var(--primary)" />
						<strong>AI Pintar</strong>
					</div>
					<p class="modeCardDesc">
						Profiler: AI menyusun strategi & brief, lalu Matchmaker pilih media. Butuh 10 Kredit.
					</p>
				</button>
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
					<p class="modeCardDesc">
						The Writer tanpa AI: siapkan media & unggah artikel tulisanmu sendiri.
					</p>
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
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 24px;
		overflow: hidden;
		border-radius: 18px;
		padding: 32px 36px;
		background-image: url('/images/dashboard-hero.webp');
		background-size: cover;
		background-position: center right;
		border: 1px solid var(--border);
		box-shadow: 0 4px 20px rgba(15, 23, 42, 0.05);
	}
	.heroContent {
		max-width: 680px;
		position: relative;
		z-index: 2;
	}
	.heroTitle {
		font-size: clamp(20px, 2.2vw, 28px);
		font-weight: 750;
		line-height: 1.2;
		letter-spacing: -0.02em;
		color: #ffffff;
		margin: 0 0 8px;
		text-shadow: 0 1px 2px rgba(0, 0, 0, 0.25);
		white-space: nowrap;
	}
	.heroSubtitle {
		font-size: 14px;
		line-height: 1.6;
		color: rgba(255, 255, 255, 0.92);
		margin: 0 0 20px;
		max-width: 500px;
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
		font-size: 14px;
		font-weight: 700;
		background: #ffffff;
		color: #1b67fe;
		border: none;
		cursor: pointer;
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
		font-size: 14px;
		font-weight: 700;
		background: rgba(255, 255, 255, 0.18);
		color: #ffffff;
		border: 1px solid rgba(255, 255, 255, 0.3);
		backdrop-filter: blur(8px);
		cursor: pointer;
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
		flex: 0 0 auto;
		display: flex;
		align-items: center;
		justify-content: center;
	}
	.heroSecurityCard {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		text-align: center;
		width: 170px;
		padding: 22px 16px;
		background: rgba(255, 255, 255, 0.15);
		backdrop-filter: blur(16px);
		-webkit-backdrop-filter: blur(16px);
		border: 1px solid rgba(255, 255, 255, 0.28);
		border-radius: 20px;
		box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);
	}
	.securityShieldCircle {
		width: 52px;
		height: 52px;
		border-radius: 50%;
		background: rgba(255, 255, 255, 0.18);
		border: 1px solid rgba(255, 255, 255, 0.3);
		display: flex;
		align-items: center;
		justify-content: center;
		margin-bottom: 12px;
		backdrop-filter: blur(8px);
		-webkit-backdrop-filter: blur(8px);
	}
	.securityTitle {
		color: rgba(255, 255, 255, 0.85);
		letter-spacing: 0.08em;
		text-transform: uppercase;
		font-size: 11px;
		font-weight: 700;
		margin-bottom: 4px;
	}
	.securityValue {
		color: #ffffff;
		font-size: 15px;
		font-weight: 700;
		line-height: 1.2;
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
	.metricCard::after {
		content: '';
		position: absolute;
		top: 0;
		right: 0;
		bottom: 0;
		width: 4px;
		background: #f59e0b;
		opacity: 0;
		transform: scaleY(0.4);
		transition:
			opacity 0.18s ease,
			transform 0.18s ease;
	}
	.metricCard:hover {
		transform: translateY(-2px);
		box-shadow: 0 8px 24px rgba(15, 23, 42, 0.08);
	}
	.metricCard:hover::after {
		opacity: 1;
		transform: scaleY(1);
	}
	.metricHeader {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		margin-bottom: 8px;
	}
	.metricTitle {
		color: var(--muted-foreground);
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.04em;
		text-transform: uppercase;
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
		background: #eef4ff;
		color: #1b67fe;
	}
	.metricValueRow {
		display: flex;
		align-items: baseline;
		gap: 8px;
	}
	.metricValue {
		font-size: 24px;
		font-weight: 750;
		line-height: 1.2;
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
	.metricDescReach {
		margin-top: 8px;
		display: block;
		font-size: 12px;
		line-height: 1.45;
		color: #059669;
		font-weight: 600;
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
		border-radius: 20px;
		border: 1px solid var(--border);
		background: radial-gradient(700px 260px at 92% 0%, rgba(124, 58, 237, 0.06), transparent 60%),
			var(--card);
		padding: 22px 28px;
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
		background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%);
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
		font-size: 15px;
		font-weight: 700;
	}
	.insightsBadge {
		display: inline-flex;
		align-items: center;
		border-radius: 999px;
		padding: 1px 8px;
		font-size: 10px;
		font-weight: 700;
		background: #faf5ff;
		color: #7e22ce;
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
		padding: 28px 26px;
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
		background: #eef4ff;
		color: #1b67fe;
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
		font-size: 24px;
		font-weight: 750;
		line-height: 1.2;
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
		font-size: 15px;
		font-weight: 700;
		color: var(--foreground);
		margin: 0 0 12px;
	}
	.queueTable {
		border: 1px solid var(--border);
		border-radius: 14px;
		overflow: hidden;
		background: var(--card);
	}
	.queueHeader,
	.queueRow {
		display: grid;
		grid-template-columns: 2fr 1.4fr 1.2fr 0.8fr;
		gap: 12px;
		align-items: center;
		padding: 12px 16px;
		font-size: 13px;
	}
	.queueHeader {
		background: var(--muted);
		border-bottom: 1px solid var(--border);
		font-size: 11px;
		font-weight: 700;
		color: var(--muted-foreground);
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}
	.queueRow + .queueRow {
		border-top: 1px solid var(--border);
	}
	.queueCell {
		min-width: 0;
	}
	.emptyCard {
		border: 1px dashed var(--border);
		border-radius: 14px;
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
		font-size: 11.5px;
		font-weight: 600;
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
		padding: 28px 26px;
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
		background: #eef4ff;
		color: #1b67fe;
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
		border-radius: 12px;
		color: var(--foreground);
		text-align: left;
		text-decoration: none;
		transition: background-color 0.15s ease;
	}
	.ghostLinkHelp:hover {
		background-color: var(--secondary);
	}
	.ghostLinkHelp:hover :global(.helpArrow) {
		transform: translateX(3px);
		color: #1b67fe;
	}
	.helpItemIcon {
		width: 36px;
		height: 36px;
		border-radius: 10px;
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		background: #eef4ff;
		color: #1b67fe;
	}
	.helpItemContent {
		flex: 1;
		min-width: 0;
	}
	.helpItemTitle {
		font-size: 14px;
		font-weight: 600;
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
		max-width: 520px;
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
		.hero {
			flex-direction: column;
			align-items: flex-start;
			padding: 28px 22px;
			gap: 20px;
		}
		.heroVisual {
			width: 100%;
		}
		.metricGrid {
			grid-template-columns: 1fr;
		}
		.insights {
			flex-direction: column;
			align-items: flex-start;
			padding: 22px;
		}
		.aiAgentImgDirect {
			width: 96px;
			height: 96px;
		}
		.queueHeader {
			display: none;
		}
		.queueRow {
			grid-template-columns: 1fr;
			gap: 6px;
			padding: 12px 16px;
		}
		.modeGrid {
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
		.heroCTAGhost {
			transition: none;
		}
	}
</style>
