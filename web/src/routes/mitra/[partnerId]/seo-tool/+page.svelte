<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Port section "Tool SEO" app lama (SeoToolSection): dua tab SEO (cek
	// DA/PA/spam domain, cache 24 jam) dan Kecepatan (audit PageSpeed mobile +
	// desktop paralel, Core Web Vitals, riwayat audit). Struktur & label
	// mengikuti sections.tsx lama; token warna memakai design token app ini.
	import { onMount } from 'svelte';
	import { CheckCircle2, Gauge, Globe2, Search, TriangleAlert } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	type SeoResult = {
		domain: string;
		da: number | null;
		pa: number | null;
		spam_score: number | null;
		checked_at: string;
		cached?: boolean;
	};
	type SeoHistoryItem = { domain: string; da: number | null; pa: number | null; spam_score: number | null; checked_at: string };
	type SpeedAudit = {
		id: string;
		url: string;
		normalized_url: string;
		domain: string;
		strategy: string;
		performance_score: number | null;
		accessibility_score: number | null;
		best_practices_score: number | null;
		seo_score: number | null;
		lcp_ms: number | null;
		inp_ms: number | null;
		cls: number | null;
		audits: Array<{ id: string; title: string; score: number | null; displayValue?: string }>;
		created_at: string;
		cached?: boolean;
	};
	type SpeedHistoryItem = {
		id: string;
		url: string;
		domain: string;
		strategy: string;
		performance_score: number | null;
		lcp_ms: number | null;
		cls: number | null;
		created_at: string;
	};

	let tab = $state<'seo' | 'kecepatan'>('seo');

	// ---- Tab SEO ----
	let seoLoading = $state(true);
	let seoHistory = $state<SeoHistoryItem[]>([]);
	let domain = $state('');
	let seoBusy = $state(false);
	let seoError = $state('');
	let seoResult = $state<SeoResult | null>(null);

	// ---- Tab Kecepatan ----
	let mediaList = $state<Array<{ id: string; name: string; website_url: string | null }>>([]);
	let selectedUrl = $state('');
	let customUrl = $state('');
	let speedBusy = $state(false);
	let speedError = $state('');
	let speedResults = $state<{ mobile?: SpeedAudit; desktop?: SpeedAudit }>({});
	let activeStrategy = $state<'mobile' | 'desktop'>('mobile');
	let speedHistory = $state<SpeedHistoryItem[]>([]);
	let speedLoadedOnce = $state(false);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await loadSeoHistory();
			seoLoading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	function switchTab(next: 'seo' | 'kecepatan') {
		tab = next;
		if (next === 'kecepatan' && !speedLoadedOnce) {
			speedLoadedOnce = true;
			void loadSpeedBootstrap();
		}
	}

	async function loadSeoHistory() {
		const res = await apiRequest('/v1/partner/seo/check');
		if (res.ok) seoHistory = (await res.json()).history ?? [];
	}

	async function checkDomain(e: SubmitEvent) {
		e.preventDefault();
		if (!domain.trim()) {
			seoError = tr('Masukkan domain terlebih dahulu.');
			return;
		}
		seoBusy = true;
		seoError = '';
		seoResult = null;
		try {
			const res = await apiRequest('/v1/partner/seo/check', {
				method: 'POST',
				body: JSON.stringify({ domain: domain.trim() })
			});
			const data = await res.json();
			if (!res.ok) {
				seoError = await readApiError(res, tr('Pemeriksaan domain gagal. Coba lagi atau hubungi admin Tauthub.'));
				return;
			}
			seoResult = data;
			await loadSeoHistory();
		} finally {
			seoBusy = false;
		}
	}

	async function loadSpeedBootstrap() {
		const mediaRes = await apiRequest('/v1/partner/media?limit=50');
		if (mediaRes.ok) {
			const media = (await mediaRes.json()).media ?? [];
			mediaList = media.filter((item: { website_url?: string | null }) => Boolean(item.website_url));
		}
		await loadSpeedHistory();
	}

	async function loadSpeedHistory() {
		const res = await apiRequest('/v1/partner/seo/speed-audit?limit=6');
		if (res.ok) speedHistory = (await res.json()).items ?? [];
	}

	async function analyzeSpeed(force = false) {
		const url = customUrl.trim() || selectedUrl.trim();
		if (!url) {
			speedError = tr('Pilih media atau masukkan URL terlebih dahulu.');
			return;
		}
		speedBusy = true;
		speedError = '';
		speedResults = {};
		const payload = { url, force };
		// Analisis mobile + desktop dijalankan paralel (parity app lama).
		const settled = await Promise.allSettled([
			apiRequest('/v1/partner/seo/speed-audit', {
				method: 'POST',
				body: JSON.stringify({ ...payload, strategy: 'mobile' })
			}),
			apiRequest('/v1/partner/seo/speed-audit', {
				method: 'POST',
				body: JSON.stringify({ ...payload, strategy: 'desktop' })
			})
		]);
		const mobile = settled[0].status === 'fulfilled' && settled[0].value.ok ? ((await settled[0].value.json()) as SpeedAudit) : undefined;
		const desktop = settled[1].status === 'fulfilled' && settled[1].value.ok ? ((await settled[1].value.json()) as SpeedAudit) : undefined;
		if (!mobile && !desktop) {
			let message = tr('Analisis gagal.');
			const failed = settled.find((s) => s.status === 'rejected') as PromiseRejectedResult | undefined;
			if (failed) message = authErrorMessageText(failed.reason);
			else {
				const resp = (settled[0].status === 'fulfilled' ? settled[0].value : settled[1].status === 'fulfilled' ? settled[1].value : null);
				if (resp) message = await readApiError(resp, tr('Analisis gagal.'));
			}
			speedError = message;
		} else {
			speedResults = { mobile, desktop };
			activeStrategy = mobile ? 'mobile' : 'desktop';
			await loadSpeedHistory();
		}
		speedBusy = false;
	}

	function authErrorMessageText(reason: unknown) {
		if (reason instanceof TypeError) return tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		if (reason instanceof Error) return reason.message;
		return tr('Analisis gagal.');
	}

	async function viewHistoryItem(item: SpeedHistoryItem) {
		speedBusy = true;
		speedError = '';
		try {
			const other = item.strategy === 'mobile' ? 'desktop' : 'mobile';
			const enc = encodeURIComponent(item.url);
			const mainRes = await apiRequest(`/v1/partner/seo/speed-audit?url=${enc}&strategy=${item.strategy}`);
			const altRes = await apiRequest(`/v1/partner/seo/speed-audit?url=${enc}&strategy=${other}`);
			const main = mainRes.ok ? ((await mainRes.json()) as SpeedAudit) : undefined;
			const alt = altRes.ok ? ((await altRes.json()) as SpeedAudit) : undefined;
			if (!main) {
				speedError = tr('Belum ada audit untuk URL ini.');
				return;
			}
			if (item.strategy === 'desktop') speedResults = { desktop: main, mobile: alt };
			else speedResults = { mobile: main, desktop: alt };
			activeStrategy = item.strategy === 'desktop' ? 'desktop' : 'mobile';
		} finally {
			speedBusy = false;
		}
	}

	function resetSpeed() {
		speedResults = {};
		speedError = '';
	}

	// formatWib*: WIB eksplisit (Asia/Jakarta) — parity app lama.
	function wibParts(iso: string) {
		const date = new Date(iso);
		if (Number.isNaN(date.getTime())) return null;
		const parts = new Intl.DateTimeFormat('id-ID', {
			timeZone: 'Asia/Jakarta',
			day: 'numeric',
			month: 'long',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			hourCycle: 'h23'
		}).formatToParts(date);
		const get = (type: string) => parts.find((p) => p.type === type)?.value ?? '';
		return { day: get('day'), month: get('month'), year: get('year'), hour: get('hour'), minute: get('minute') };
	}
	function formatWibFull(iso: string) {
		const p = wibParts(iso);
		return p ? `${p.day} ${p.month} ${p.year}, ${p.hour}:${p.minute}` : '—';
	}
	function formatWibShort(iso: string) {
		const date = new Date(iso);
		if (Number.isNaN(date.getTime())) return '—';
		return new Intl.DateTimeFormat('id-ID', {
			timeZone: 'Asia/Jakarta',
			day: '2-digit',
			month: 'short',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			hourCycle: 'h23'
		}).format(date);
	}

	function seoScoreColor(v: number | null, invert = false) {
		if (v == null) return 'var(--muted-foreground)';
		if (invert) return v >= 60 ? 'var(--success)' : v >= 40 ? 'var(--warning)' : 'var(--destructive)';
		return v >= 50 ? 'var(--success)' : v >= 25 ? 'var(--warning)' : 'var(--destructive)';
	}
	function seoScoreLabel(v: number | null, invert = false) {
		if (v == null) return tr('Belum tersedia');
		if (invert) return v >= 60 ? tr('Aman') : v >= 40 ? tr('Perlu perhatian') : tr('Tinggi');
		return v >= 50 ? tr('Baik') : v >= 25 ? tr('Cukup') : tr('Rendah');
	}
	function speedScoreColor(v: number | null) {
		if (v == null) return 'var(--muted-foreground)';
		if (v >= 90) return 'var(--success)';
		if (v >= 50) return 'var(--warning)';
		return 'var(--destructive)';
	}
	function speedScoreLabel(v: number | null) {
		if (v == null) return '—';
		if (v >= 90) return tr('Baik');
		if (v >= 50) return tr('Perlu perhatian');
		return tr('Buruk');
	}
	function cwvStatus(v: number | null, type: 'lcp' | 'inp' | 'cls'): { label: string; color: string } {
		if (v == null) return { label: '—', color: 'var(--muted-foreground)' };
		if (type === 'lcp') {
			if (v <= 2500) return { label: tr('Baik'), color: 'var(--success)' };
			if (v <= 4000) return { label: tr('Perlu perhatian'), color: 'var(--warning)' };
			return { label: tr('Buruk'), color: 'var(--destructive)' };
		}
		if (type === 'inp') {
			if (v <= 200) return { label: tr('Baik'), color: 'var(--success)' };
			if (v <= 500) return { label: tr('Perlu perhatian'), color: 'var(--warning)' };
			return { label: tr('Buruk'), color: 'var(--destructive)' };
		}
		if (v <= 0.1) return { label: tr('Baik'), color: 'var(--success)' };
		if (v <= 0.25) return { label: tr('Perlu perhatian'), color: 'var(--warning)' };
		return { label: tr('Buruk'), color: 'var(--destructive)' };
	}

	const displayUrl = $derived(customUrl.trim() || selectedUrl.trim() || '');
	const activeAudit = $derived(speedResults[activeStrategy] ?? speedResults.mobile ?? speedResults.desktop);
	const hasMobile = $derived(Boolean(speedResults.mobile));
	const hasDesktop = $derived(Boolean(speedResults.desktop));
	const failedAudits = $derived(activeAudit ? (activeAudit.audits ?? []).filter((a) => a.score != null && a.score < 0.5) : []);
	const improvementAudits = $derived(activeAudit ? (activeAudit.audits ?? []).filter((a) => a.score == null || a.score >= 0.5) : []);
	const perfCopy = $derived.by(() => {
		const score = activeAudit?.performance_score;
		if (score == null) return tr('Skor performa belum tersedia.');
		if (score >= 90) return tr('Performa halaman sangat baik.');
		if (score >= 50) return tr('Performa halaman masih memiliki beberapa area yang dapat dioptimalkan.');
		return tr('Performa halaman perlu perbaikan signifikan.');
	});

	const dtSeo = (v: string | null) =>
		v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—';
</script>

<PageTitle title={tr('Tool SEO')} />

<section>
	<header>
		<p class="text-xs font-semibold tracking-wide text-muted-foreground">{tr("PARTNER PORTAL · SEO")}</p>
		<h1 class="text-2xl font-semibold tracking-tight">{tr('Tool SEO')}</h1>
		<p class="mt-1 text-sm text-muted-foreground">{tr("Analisis kualitas dan performa website media Anda.")}</p>
	</header>

	<div class="mt-4 flex gap-2 border-b" style="border-color: var(--border);">
		{#each [['seo', 'SEO'], ['kecepatan', tr('Kecepatan')]] as [value, label] (value)}
			<button
				type="button"
				class="-mb-px border-b-2 px-4 py-2 text-sm font-medium transition-colors"
				class:font-semibold={tab === value}
				style="border-color: {tab === value ? 'var(--primary)' : 'transparent'}; color: {tab === value ? 'var(--primary)' : 'var(--muted-foreground)'};"
				onclick={() => switchTab(value as 'seo' | 'kecepatan')}
			>
				{label}
			</button>
		{/each}
	</div>

	{#if tab === 'seo'}
		<div class="mt-4">
			<h2 class="text-base font-semibold">{tr("Periksa Kualitas Domain")}</h2>
			<p class="mt-1 text-sm text-muted-foreground">{tr("Masukkan domain untuk melihat DA, PA, dan Spam Score.")}</p>

			{#if seoError}
				<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{seoError}</div>
			{/if}

			<form class="mt-3 flex flex-wrap items-center gap-2" onsubmit={checkDomain}>
				<label class="block w-full max-w-md flex-1">
					<span class="sr-only">{tr("Masukkan domain untuk melihat DA, PA, dan Spam Score.")}</span>
					<input class="app-input w-full" required placeholder={tr("contoh: tauthub.com")} bind:value={domain} />
				</label>
				<button class="app-btn app-btn-primary" type="submit" disabled={seoBusy}>
					<Globe2 size={15} />
					{seoBusy ? tr('Memeriksa…') : tr('Cek Domain')}
				</button>
			</form>

			{#if seoResult}
				<div class="app-card mt-4 p-4">
					<div class="flex flex-wrap items-center gap-2">
						<Globe2 size={17} />
						<h3 class="text-base font-bold">{seoResult.domain}</h3>
						<span class="text-xs text-muted-foreground">· {tr('dicek')} {dtSeo(seoResult.checked_at)}</span>
						{#if seoResult.cached}
							<span class="rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--accent); color: var(--accent-foreground);">{tr('dari cache (24 jam)')}</span>
						{/if}
					</div>
					<div class="mt-3 grid gap-3 sm:grid-cols-3">
						<div class="rounded-xl border p-3" style="border-color: var(--border);">
							<p class="text-xs text-muted-foreground">{tr("Domain Authority (DA)")}</p>
							<p class="mt-1 text-3xl font-bold tabular-nums" style="color: {seoResult.da == null ? 'inherit' : 'var(--accent-foreground)'};">{seoResult.da ?? '—'}</p>
							<span class="mt-1 inline-block rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--secondary); color: {seoScoreColor(seoResult.da)};">{seoScoreLabel(seoResult.da)}</span>
						</div>
						<div class="rounded-xl border p-3" style="border-color: var(--border);">
							<p class="text-xs text-muted-foreground">{tr("Page Authority (PA)")}</p>
							<p class="mt-1 text-3xl font-bold tabular-nums" style="color: {seoResult.pa == null ? 'inherit' : 'var(--accent-foreground)'};">{seoResult.pa ?? '—'}</p>
							<span class="mt-1 inline-block rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--secondary); color: {seoScoreColor(seoResult.pa)};">{seoScoreLabel(seoResult.pa)}</span>
						</div>
						<div class="rounded-xl border p-3" style="border-color: var(--border);">
							<p class="text-xs text-muted-foreground">{tr("Spam Score")}</p>
							<p class="mt-1 text-3xl font-bold tabular-nums" style="color: {seoResult.spam_score == null ? 'inherit' : 'var(--accent-foreground)'};">{seoResult.spam_score ?? '—'}</p>
							<span class="mt-1 inline-block rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--secondary); color: {seoScoreColor(seoResult.spam_score, true)};">{seoScoreLabel(seoResult.spam_score, true)}</span>
						</div>
					</div>
					<p class="mt-3 text-xs text-muted-foreground">{tr("Skor dari provider pihak ketiga (Moz metrics via RapidAPI). Bukan jaminan kualitas media, gunakan sebagai referensi.")}</p>
				</div>
			{/if}

			<div class="app-card mt-4 overflow-x-auto">
				<div class="border-b px-4 py-3"><h3 class="text-sm font-semibold">{tr("Riwayat Pengecekan")}</h3></div>
				{#if seoLoading}
					<p class="px-4 py-6 text-center text-sm text-muted-foreground">{tr("Memuat riwayat…")}</p>
				{:else if seoHistory.length === 0}
					<p class="px-4 py-6 text-center text-sm text-muted-foreground">{tr("Belum ada riwayat pemeriksaan domain.")}</p>
				{:else}
					<table class="w-full text-sm">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="px-4 py-2 font-medium">{tr("Domain")}</th>
								<th class="px-4 py-2 text-right font-medium">DA</th>
								<th class="px-4 py-2 text-right font-medium">PA</th>
								<th class="px-4 py-2 text-right font-medium">{tr("Spam")}</th>
								<th class="px-4 py-2 font-medium">{tr("Tanggal")}</th>
							</tr>
						</thead>
						<tbody>
							{#each seoHistory as h (h.domain + h.checked_at)}
								<tr class="border-t">
									<td class="px-4 py-2">{h.domain}</td>
									<td class="px-4 py-2 text-right tabular-nums">{h.da ?? '—'}</td>
									<td class="px-4 py-2 text-right tabular-nums">{h.pa ?? '—'}</td>
									<td class="px-4 py-2 text-right tabular-nums">{h.spam_score ?? '—'}</td>
									<td class="px-4 py-2 text-xs text-muted-foreground">{dtSeo(h.checked_at)}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</div>
		</div>
	{:else}
		<div class="mt-4">
			{#if speedError}
				<div class="app-card mb-4 p-4" style="border-color: var(--destructive);">
					<div class="flex items-start gap-2">
						<TriangleAlert size={17} class="mt-0.5 shrink-0" style="color: var(--destructive);" />
						<div class="min-w-0 flex-1">
							<p class="text-sm font-bold">{tr("Analisis belum dapat dilakukan")}</p>
							<p class="mt-1 text-sm">{speedError}</p>
							<p class="mt-1 text-xs text-muted-foreground">{tr("Pastikan website dapat diakses secara publik dan coba lagi.")}</p>
							{#if displayUrl}
								<button class="app-btn app-btn-soft mt-2 !min-h-7 !px-2.5 !text-xs" style="color: var(--destructive);" disabled={speedBusy} onclick={() => void analyzeSpeed(false)}>
									{tr("Coba Lagi")}
								</button>
							{/if}
						</div>
					</div>
				</div>
			{/if}

			{#if !activeAudit && !speedBusy}
				<div class="app-card mb-4 p-4">
					<h2 class="text-base font-semibold">{tr("Analisis Kecepatan Website")}</h2>
					<p class="mt-1 text-sm text-muted-foreground">{tr("Periksa performa, Core Web Vitals, dan masalah teknis yang dapat memengaruhi pengalaman pengguna website. Hasil akan mencakup audit Mobile dan Desktop.")}</p>
					<div class="mt-4">
						<p class="text-sm font-bold">{tr("Website yang akan dianalisis")}</p>
						<div class="mt-2 max-w-md">
							<Select
								value={selectedUrl}
								ariaLabel={tr("Website yang akan dianalisis")}
								placeholder={tr("Pilih website")}
								minWidth={420}
								onchange={(v) => { selectedUrl = v; if (v) customUrl = ''; }}
								options={[
									{ value: '', label: tr('Pilih website'), disabled: true },
									...mediaList.map((m) => {
										const host = (m.website_url ?? '').replace(/^https?:\/\//i, '').replace(/\/+$/, '');
										return { value: m.website_url ?? '', label: host ? `${m.name} (${host})` : m.name };
									})
								]}
							/>
						</div>
						{#if selectedUrl}
							<p class="mt-1 max-w-md break-all text-xs text-muted-foreground">{selectedUrl}</p>
						{/if}
						<div class="my-3 flex items-center gap-2">
							<div class="h-px flex-1" style="background-color: var(--border);"></div>
							<span class="whitespace-nowrap text-xs text-muted-foreground">{tr("atau masukkan URL lain")}</span>
							<div class="h-px flex-1" style="background-color: var(--border);"></div>
						</div>
						<label class="block max-w-md">
							<span class="sr-only">{tr("atau masukkan URL lain")}</span>
							<input
								class="app-input w-full"
								placeholder={tr("https://example.com")}
								bind:value={customUrl}
								oninput={() => { if (customUrl) selectedUrl = ''; }}
							/>
						</label>
					</div>
					<div class="mt-4">
						<button class="app-btn app-btn-primary" disabled={speedBusy || !displayUrl} onclick={() => void analyzeSpeed(false)}>
							<Search size={15} />
							{speedBusy ? tr('Menganalisis…') : tr('Analisis Kecepatan')}
						</button>
					</div>
				</div>

				{#if speedHistory.length === 0}
					<div class="py-6 text-center">
						<p class="text-sm text-muted-foreground">{tr("Belum ada hasil analisis.")}</p>
						<p class="mt-1 text-sm text-muted-foreground">{tr("Jalankan analisis untuk melihat performa dan Core Web Vitals website.")}</p>
					</div>
				{/if}
			{/if}

			{#if speedBusy}
				<div class="app-card mb-4 p-10 text-center">
					<div class="mx-auto h-8 w-8 animate-spin rounded-full border-2 border-t-transparent" style="border-color: var(--primary); border-top-color: transparent;"></div>
					<p class="mt-3 font-bold">{tr("Sedang menganalisis website...")}</p>
					<p class="mt-1 text-sm text-muted-foreground">{tr("Proses ini membutuhkan beberapa detik.")}</p>
				</div>
			{/if}

			{#if activeAudit && !speedBusy}
				<div class="mb-4 flex flex-wrap items-start justify-between gap-3">
					<div class="min-w-0">
						<p class="text-xs font-medium tracking-wide text-muted-foreground">{tr("Kecepatan Website")}</p>
						<h2 class="mt-1 break-all text-xl font-semibold">{activeAudit.domain}</h2>
						<p class="mt-2 break-all text-sm text-muted-foreground">{tr("Website")}: {activeAudit.url}</p>
						<p class="mt-1 text-xs text-muted-foreground">
							{tr("Audit terakhir")}: {formatWibFull(activeAudit.created_at)} · {activeAudit.strategy === 'desktop' ? tr('Desktop') : tr('Mobile')}{activeAudit.cached ? ` · ${tr('dari cache')}` : ''}
						</p>
					</div>
					<div class="flex flex-wrap items-center gap-2">
						{#if hasMobile && hasDesktop}
							<div class="flex overflow-hidden rounded-lg border" style="border-color: var(--border);">
								<button
									type="button"
									class="px-3 py-1.5 text-xs font-medium"
									style="background-color: {activeStrategy === 'mobile' ? 'var(--primary)' : 'transparent'}; color: {activeStrategy === 'mobile' ? 'var(--primary-foreground)' : 'var(--muted-foreground)'};"
									onclick={() => (activeStrategy = 'mobile')}
								>{tr("Mobile")}</button>
								<button
									type="button"
									class="px-3 py-1.5 text-xs font-medium"
									style="background-color: {activeStrategy === 'desktop' ? 'var(--primary)' : 'transparent'}; color: {activeStrategy === 'desktop' ? 'var(--primary-foreground)' : 'var(--muted-foreground)'};"
									onclick={() => (activeStrategy = 'desktop')}
								>{tr("Desktop")}</button>
							</div>
						{:else}
							<span class="rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--secondary); color: var(--muted-foreground);">{hasDesktop ? tr('Desktop') : tr('Mobile')}</span>
						{/if}
						<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" disabled={speedBusy} onclick={() => void analyzeSpeed(true)}>
							<Gauge size={13} />
							{tr("Analisis Ulang")}
						</button>
						<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" onclick={resetSpeed}>{tr("Analisis URL lain")}</button>
					</div>
				</div>

				<div class="mb-4">
					<h3 class="text-base font-semibold">{tr("Performance Overview")}</h3>
					<div class="mt-3 grid gap-3 md:grid-cols-2">
						<div class="app-card p-4">
							<div class="flex items-center justify-between gap-2">
								<p class="text-sm font-bold">{tr("Performance")}</p>
								<span class="rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--secondary); color: {speedScoreColor(activeAudit.performance_score)};">{speedScoreLabel(activeAudit.performance_score)}</span>
							</div>
							<div class="mt-2 flex items-baseline gap-2">
								<span class="text-4xl font-bold tabular-nums" style="color: {activeAudit.performance_score == null ? 'inherit' : 'var(--accent-foreground)'};">{activeAudit.performance_score ?? '—'}</span>
								<span class="text-sm text-muted-foreground">/ 100</span>
							</div>
							<div class="mt-3 h-2.5 w-full overflow-hidden rounded-full" style="background-color: var(--secondary);">
								<div class="h-full rounded-full" style="width: {activeAudit.performance_score ?? 0}%; background-color: {speedScoreColor(activeAudit.performance_score)};"></div>
							</div>
							<p class="mt-3 text-sm text-muted-foreground">{perfCopy}</p>
						</div>
						<div class="grid gap-3 sm:grid-cols-3">
							{#each [
								{ label: 'Accessibility', value: activeAudit.accessibility_score },
								{ label: 'Best Practices', value: activeAudit.best_practices_score },
								{ label: 'SEO', value: activeAudit.seo_score }
							] as sub (sub.label)}
								<div class="app-card p-3">
									<p class="text-xs text-muted-foreground">{sub.label}</p>
									<div class="mt-2 flex items-baseline gap-1">
										<span class="text-2xl font-bold tabular-nums">{sub.value ?? '—'}</span>
										<span class="text-xs text-muted-foreground">/ 100</span>
									</div>
									<span class="mt-2 inline-block rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--secondary); color: {speedScoreColor(sub.value)};">{speedScoreLabel(sub.value)}</span>
								</div>
							{/each}
						</div>
					</div>
				</div>

				<div class="mb-4">
					<h3 class="text-base font-semibold">{tr("Core Web Vitals")}</h3>
					<p class="mt-1 text-sm text-muted-foreground">{tr("Metrik utama yang menggambarkan pengalaman pengguna saat halaman dimuat dan digunakan.")}</p>
					<div class="mt-3 grid gap-3 sm:grid-cols-3">
						{#each [
							{ label: 'LCP', value: activeAudit.lcp_ms, display: activeAudit.lcp_ms != null ? `${(activeAudit.lcp_ms / 1000).toFixed(1)} s` : '—', type: 'lcp' as const, desc: tr('Waktu hingga konten utama halaman tampil.') },
							{ label: 'INP', value: activeAudit.inp_ms, display: activeAudit.inp_ms != null ? `${activeAudit.inp_ms} ms` : '—', type: 'inp' as const, desc: tr('Respons halaman terhadap interaksi pengguna.') },
							{ label: 'CLS', value: activeAudit.cls, display: activeAudit.cls != null ? String(activeAudit.cls) : '—', type: 'cls' as const, desc: tr('Stabilitas posisi elemen saat halaman dimuat.') }
						] as metric (metric.label)}
							{@const st = cwvStatus(metric.value, metric.type)}
							<div class="app-card p-4">
								<div class="flex items-center justify-between gap-2">
									<p class="text-sm font-bold">{metric.label}</p>
									<span class="rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--secondary); color: {st.color};">{st.label}</span>
								</div>
								<p class="mt-3 text-2xl font-bold tabular-nums">{metric.display}</p>
								<p class="mt-2 text-xs text-muted-foreground">{metric.desc}</p>
							</div>
						{/each}
					</div>
				</div>

				<div class="mb-4">
					<h3 class="text-base font-semibold">{tr("Yang Perlu Diperhatikan")}</h3>
					{#if failedAudits.length === 0}
						<div class="mt-3 flex items-center gap-2 rounded-xl p-3" style="background-color: var(--success-soft);">
							<CheckCircle2 size={18} style="color: var(--success);" />
							<p class="text-sm">{tr("Tidak ada masalah performance utama yang ditemukan.")}</p>
						</div>
					{:else}
						<div class="mt-3 space-y-2">
							{#each failedAudits.slice(0, 6) as a (a.id)}
								<div class="flex items-start gap-3 rounded-xl p-3" style="background-color: var(--warning-soft);">
									<TriangleAlert size={18} class="mt-0.5 shrink-0" style="color: var(--warning);" />
									<div class="min-w-0">
										<p class="text-sm font-medium">{a.title}</p>
										{#if a.displayValue}<p class="mt-1 text-xs text-muted-foreground">{a.displayValue}</p>{/if}
									</div>
								</div>
							{/each}
						</div>
					{/if}
				</div>

				{#if improvementAudits.length > 0}
					<div class="mb-4">
						<h3 class="text-base font-semibold">{tr("Rekomendasi Perbaikan")}</h3>
						<div class="mt-3 space-y-2">
							{#each improvementAudits.slice(0, 6) as a (a.id)}
								<div class="flex items-start justify-between gap-3 rounded-xl p-3" style="background-color: var(--muted);">
									<div class="min-w-0">
										<p class="text-sm font-medium">{a.title}</p>
										{#if a.displayValue}<p class="mt-1 break-all text-xs text-muted-foreground">{a.displayValue}</p>{/if}
									</div>
									<span class="shrink-0 whitespace-nowrap rounded-full px-2 py-0.5 text-[11px]" style="background-color: {a.displayValue ? 'var(--accent)' : 'var(--secondary)'}; color: {a.displayValue ? 'var(--accent-foreground)' : 'var(--muted-foreground)'};">
										{a.displayValue ? tr('Potensi penghematan') : tr('Penyempurnaan')}
									</span>
								</div>
							{/each}
						</div>
					</div>
				{/if}
			{/if}

			{#if speedHistory.length > 0}
				<div>
					<h3 class="text-base font-semibold">{tr("Riwayat Audit")}</h3>
					<p class="mt-1 text-sm text-muted-foreground">{tr("Pantau hasil analisis performa website sebelumnya.")}</p>
					<div class="app-card mt-3 overflow-x-auto">
						<table class="w-full text-sm">
							<thead>
								<tr class="text-left text-xs text-muted-foreground">
									<th class="px-4 py-2.5 font-medium">{tr("Website")}</th>
									<th class="px-4 py-2.5 font-medium">{tr("Strategi")}</th>
									<th class="px-4 py-2.5 font-medium">{tr("Tanggal")}</th>
									<th class="px-4 py-2.5 text-right font-medium">{tr("Performance")}</th>
									<th class="px-4 py-2.5 text-right font-medium">LCP</th>
									<th class="px-4 py-2.5 text-right font-medium">CLS</th>
									<th class="px-4 py-2.5 text-right font-medium">{tr("Aksi")}</th>
								</tr>
							</thead>
							<tbody>
								{#each speedHistory as h (h.id)}
									<tr class="border-t">
										<td class="max-w-[220px] break-all px-4 py-2.5">{h.domain}</td>
										<td class="px-4 py-2.5">
											<span class="rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--secondary); color: var(--muted-foreground);">{h.strategy === 'desktop' ? tr('Desktop') : tr('Mobile')}</span>
										</td>
										<td class="whitespace-nowrap px-4 py-2.5 text-xs text-muted-foreground">{formatWibShort(h.created_at)}</td>
										<td class="px-4 py-2.5 text-right tabular-nums" style="color: {speedScoreColor(h.performance_score)};">{h.performance_score ?? '—'}</td>
										<td class="px-4 py-2.5 text-right tabular-nums">{h.lcp_ms != null ? `${(h.lcp_ms / 1000).toFixed(1)}s` : '—'}</td>
										<td class="px-4 py-2.5 text-right tabular-nums">{h.cls ?? '—'}</td>
										<td class="px-4 py-2.5 text-right">
											<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" disabled={speedBusy} onclick={() => void viewHistoryItem(h)}>{tr("Lihat")}</button>
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				</div>
			{/if}
		</div>
	{/if}
</section>
