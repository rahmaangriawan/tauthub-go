<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Port app/mitra/[partnerId]/[[...section]]/integrations-section.tsx.
	// Halaman Integrasi mitra = 4 tab dengan gerbang Syarat & Ketentuan:
	//   terms → wordpress (Koneksi CMS) · web-custom (Publishing API) · blogger.
	// Tab bisa di-deep-link lewat hash (#wordpress / #web-custom / #blogger) —
	// itulah tujuan redirect section lama /connections, /publishing-api, /blogger
	// (lihat page.tsx app lama).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Check, Lock } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import Connections from './_connections.svelte';
	import PublishingApi from './_publishing-api.svelte';
	import Blogger from './_blogger.svelte';
	import Status from './_status.svelte';
	import PageTitle from '$lib/components/page-title.svelte';
	import './integrations.css';

	type TabKey = 'terms' | 'wordpress' | 'web-custom' | 'blogger';
	type Tone = 'ok' | 'error' | 'warn' | 'idle';
	type ChannelMeta = { count: number; tone: Tone };

	let terms = $state({ acceptedAt: null as string | null, loading: true, busy: false, error: '' });
	let tab = $state<TabKey>('terms');
	let termsChecked = $state(false);
	let confirmOpen = $state(false);

	// Deep link dari Daftar Media (ikon mata): #blogger?media=<id> menyorot media
	// itu di tab yang sesuai. `highlightTab` mencegah id dipakai tab lain.
	let highlightId = $state('');
	let highlightTab = $state<TabKey>('terms');

	// Ringkasan per kanal untuk bar tab: jumlah koneksi/media dan titik status.
	// Dimuat sekali, ringan; detailnya tetap dimuat oleh masing-masing tab.
	let meta = $state<Record<Exclude<TabKey, 'terms'>, ChannelMeta | null>>({
		wordpress: null,
		'web-custom': null,
		blogger: null
	});

	const accepted = $derived(Boolean(terms.acceptedAt));

	const TABS: Array<[TabKey, string]> = [
		['terms', 'Syarat & Ketentuan'],
		['wordpress', 'WordPress'],
		['web-custom', 'Web Custom'],
		['blogger', 'Blogspot']
	];

	function hashTab(): TabKey {
		// Hash bisa membawa query (callback OAuth Blogger: #blogger?blogger_connected=1&media=…),
		// jadi potong di '?' sebelum dibandingkan — parity app lama.
		const h = page.url.hash.replace(/^#/, '').split('?')[0];
		if (h === 'wordpress' || h === 'web-custom' || h === 'blogger') return h;
		return 'terms';
	}

	function hashMedia(): string {
		const q = page.url.hash.split('?')[1];
		return q ? (new URLSearchParams(q).get('media') ?? '') : '';
	}

	/** Dipanggil tab setelah menyorot: id dilepas dan ?media= dibersihkan dari URL. */
	function clearHighlight() {
		highlightId = '';
		const hash = highlightTab === 'terms' ? '' : `#${highlightTab}`;
		history.replaceState(null, '', `${page.url.pathname}${page.url.search}${hash}`);
	}

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([loadTerms(), loadMeta()]);
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	// Sinkronkan tab aktif dengan hash setelah status syarat diketahui — tab
	// terlarang jatuh kembali ke "terms" (parity effect app lama).
	$effect(() => {
		if (terms.loading) return;
		const requested = hashTab();
		tab = !accepted && requested !== 'terms' ? 'terms' : requested;
		const media = hashMedia();
		if (media) {
			highlightId = media;
			highlightTab = requested;
		}
	});

	async function loadTerms() {
		terms = { ...terms, loading: true, error: '' };
		const res = await apiRequest('/v1/partner/me');
		if (!res.ok) {
			terms = { acceptedAt: null, loading: false, busy: false, error: await readApiError(res, tr('Gagal memuat status integrasi.')) };
			return;
		}
		const data = await res.json();
		const acceptedAt: string | null =
			data?.integration_terms_accepted_at ?? data?.partner?.integration_terms_accepted_at ?? null;
		terms = { acceptedAt, loading: false, busy: false, error: '' };
	}

	function toneOf(statuses: string[], okValue: string): Tone {
		if (statuses.length === 0) return 'idle';
		if (statuses.includes('failed') || statuses.includes('error')) return 'error';
		if (statuses.includes('testing')) return 'warn';
		if (statuses.includes(okValue)) return 'ok';
		return 'idle';
	}

	async function loadMeta() {
		const [connRes, mediaRes] = await Promise.all([
			apiRequest('/v1/partner/cms-connections'),
			apiRequest('/v1/partner/media?limit=500')
		]);
		if (connRes.ok) {
			const rows: any[] = (await connRes.json()).connections ?? [];
			const active = rows.filter((r) => r.status === 'active');
			meta.wordpress = {
				count: active.length,
				tone: toneOf(active.map((r) => r.last_test_status ?? ''), 'ok')
			};
		}
		if (mediaRes.ok) {
			const all: any[] = (await mediaRes.json()).media ?? [];
			const custom = all.filter((m) => m.integration_type === 'custom_web' && m.status !== 'hidden');
			const blogger = all.filter((m) => m.integration_type === 'blogger' && m.status !== 'hidden');
			meta['web-custom'] = { count: custom.length, tone: toneOf(custom.map((m) => m.custom_web_status ?? ''), 'connected') };
			meta.blogger = { count: blogger.length, tone: toneOf(blogger.map((m) => m.blogger_status ?? ''), 'connected') };
		}
	}

	async function acceptTerms() {
		confirmOpen = false;
		terms = { ...terms, busy: true, error: '' };
		const res = await apiRequest('/v1/partner/me', {
			method: 'PATCH',
			body: JSON.stringify({ accept_integration_terms: true })
		});
		if (!res.ok) {
			terms = { ...terms, busy: false, error: await readApiError(res, tr('Gagal menyimpan persetujuan.')) };
			return;
		}
		await loadTerms();
		selectTab('wordpress');
	}

	function selectTab(value: TabKey) {
		if (!accepted && value !== 'terms') return;
		tab = value;
		const hash = value === 'terms' ? '' : `#${value}`;
		// Hash dipakai agar deep-link lama (/connections, /publishing-api, /blogger)
		// mendarat di tab yang benar.
		history.replaceState(null, '', `${page.url.pathname}${page.url.search}${hash}`);
	}

	const acceptedLong = $derived(
		terms.acceptedAt
			? new Date(terms.acceptedAt).toLocaleString('id-ID', { dateStyle: 'long', timeStyle: 'short' })
			: ''
	);
	const acceptedShort = $derived(
		terms.acceptedAt ? new Date(terms.acceptedAt).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : ''
	);
</script>

<PageTitle title={tr('Integrasi')} />

<section class="int-page">
	<header class="int-header">
		<div>
			<h1 class="page-title">{tr('Integrasi')}</h1>
			<p class="page-desc">{tr('Sambungkan website media Anda agar artikel klien terkirim otomatis lewat WordPress, website custom, atau Blogspot')}</p>
		</div>
		{#if !terms.loading}
			{#if accepted}
				<p class="int-terms-state" style="color: var(--muted-foreground);">
					<Check size={16} strokeWidth={1.5} aria-hidden="true" />
					{tr('Syarat disetujui')} {acceptedShort}
				</p>
			{:else}
				<p class="int-terms-state" style="color: var(--warning);">
					<Lock size={16} strokeWidth={1.5} aria-hidden="true" />
					{tr('Syarat belum disetujui')}
				</p>
			{/if}
		{/if}
	</header>

	{#if terms.error}
		<div class="app-card int-error">{terms.error}</div>
	{/if}

	{#if terms.loading}
		<p class="int-loading">{tr('Memuat…')}</p>
	{:else}
		<div class="int-tabs" role="tablist" aria-label={tr('Kanal integrasi')}>
			{#each TABS as [value, label] (value)}
				{@const locked = !accepted && value !== 'terms'}
				{@const m = value === 'terms' ? null : meta[value]}
				<button
					class="int-tab"
					class:int-tab--active={tab === value}
					class:int-tab--locked={locked}
					role="tab"
					aria-selected={tab === value}
					aria-disabled={locked}
					title={locked ? tr('Setujui Syarat & Ketentuan dulu') : undefined}
					onclick={() => selectTab(value)}
				>
					<span class="int-tab__label">{label}</span>
					{#if locked}
						<Lock size={14} strokeWidth={1.5} aria-hidden="true" />
					{:else if m}
						<span class="int-tab__meta">
							<span class="int-tab__count">{m.count}</span>
							<Status tone={m.tone} size="xs" />
						</span>
					{/if}
				</button>
			{/each}
		</div>
		{#if !accepted}
			<p class="int-tabs-hint">{tr('Tab WordPress, Web Custom, dan Blogspot terbuka setelah Syarat & Ketentuan disetujui')}</p>
		{/if}

		{#if tab === 'terms'}
			<div class="app-card int-terms">
				<h2 class="int-card-title">{tr('Syarat & Ketentuan Integrasi')}</h2>
				<div class="int-terms__body">
					<p>{tr('Dengan menyetujui, Anda memberi Tauthub izin mengirim artikel klien ke akun/website media Anda melalui kanal yang Anda hubungkan di halaman ini (WordPress, Web Custom, atau Blogspot).')}</p>
					<p>{tr('Anda bertanggung jawab memastikan kredensial yang dimasukkan valid dan akun yang terhubung milik media Anda. Tauthub tidak pernah mengubah artikel yang sudah terbit tanpa tindakan Anda.')}</p>
					<p>{tr('Persetujuan ini dicatat waktunya dan dapat dicabut dengan memutus koneksi di tab terkait.')}</p>
					<p>{tr('Akses ke media Anda di Tauthub hanya boleh diberikan kepada anggota dengan peran penulis atau maksimal editor. Peran yang lebih tinggi tidak diperbolehkan untuk mengelola media.')}</p>
				</div>

				{#if accepted}
					<div class="int-terms__foot">
						<p class="int-terms__accepted">
							<Check size={16} strokeWidth={1.5} aria-hidden="true" />
							{tr('Disetujui pada')} {acceptedLong}
						</p>
						<button class="app-btn app-btn-soft int-btn" type="button" onclick={() => selectTab('wordpress')}>{tr('Buka kanal WordPress')}</button>
					</div>
				{:else}
					<div class="int-terms__foot int-terms__foot--consent">
						<label class="int-consent">
							<input type="checkbox" bind:checked={termsChecked} />
							<span>{tr('Saya menyetujui Syarat & Ketentuan Integrasi Tauthub di atas')}</span>
						</label>
						<button class="app-btn app-btn-primary int-btn" disabled={!termsChecked || terms.busy} onclick={() => (confirmOpen = true)}>
							{terms.busy ? tr('Menyimpan…') : tr('Setujui dan lanjutkan')}
						</button>
					</div>
				{/if}
			</div>
		{:else if tab === 'wordpress'}
			<Connections onchange={loadMeta} highlight={highlightTab === 'wordpress' ? highlightId : ''} onhighlighted={clearHighlight} />
		{:else if tab === 'web-custom'}
			<PublishingApi onchange={loadMeta} highlight={highlightTab === 'web-custom' ? highlightId : ''} onhighlighted={clearHighlight} />
		{:else}
			<Blogger onchange={loadMeta} highlight={highlightTab === 'blogger' ? highlightId : ''} onhighlighted={clearHighlight} />
		{/if}
	{/if}
</section>

{#if confirmOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card w-full max-w-sm p-6" aria-label={tr('Konfirmasi persetujuan')}>
			<h2 class="text-base font-medium">{tr('Setujui Syarat & Ketentuan?')}</h2>
			<p class="mt-2 text-sm text-muted-foreground">{tr('Setelah disetujui, tab WordPress, Web Custom, dan Blogspot akan terbuka.')}</p>
			<div class="mt-6 flex justify-end gap-2">
				<button class="app-btn app-btn-soft int-btn" onclick={() => (confirmOpen = false)}>{tr('Batal')}</button>
				<button class="app-btn app-btn-primary int-btn" disabled={terms.busy} onclick={acceptTerms}>{tr('Ya, setujui')}</button>
			</div>
		</dialog>
	</div>
{/if}

<style>
	.int-page {
		max-width: 1120px;
	}
	.int-header {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px 24px;
	}
	.int-terms-state {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		margin-top: 8px;
		font-size: 13px;
		line-height: 16px;
	}
	.int-error {
		margin-top: 16px;
		padding: 12px 16px;
		font-size: 14px;
		color: var(--destructive);
	}
	.int-loading {
		margin-top: 24px;
		font-size: 14px;
		color: var(--muted-foreground);
	}

	/* Tab garis bawah: label + jumlah mono + titik status; gembok bila terkunci.
	   Di layar sentuh baris tab digeser horizontal. */
	.int-tabs {
		display: flex;
		gap: 4px;
		margin-top: 24px;
		border-bottom: 1px solid var(--border);
		overflow-x: auto;
		scrollbar-width: none;
	}
	.int-tabs::-webkit-scrollbar {
		display: none;
	}
	.int-tab {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		flex: 0 0 auto;
		height: 40px;
		margin-bottom: -1px;
		padding: 0 12px;
		border-bottom: 2px solid transparent;
		font-size: 14px;
		line-height: 20px;
		font-weight: 500;
		color: var(--muted-foreground);
		white-space: nowrap;
		transition: color 160ms cubic-bezier(0.2, 0.8, 0.2, 1), border-color 160ms cubic-bezier(0.2, 0.8, 0.2, 1);
	}
	.int-tab:hover:not(.int-tab--locked) {
		color: var(--foreground);
	}
	.int-tab:focus-visible {
		outline: none;
		box-shadow: inset 0 0 0 2px color-mix(in srgb, var(--primary) 40%, transparent);
		border-radius: 6px 6px 0 0;
	}
	.int-tab--active {
		color: var(--foreground);
		border-bottom-color: var(--primary);
	}
	.int-tab--locked {
		cursor: not-allowed;
		color: color-mix(in srgb, var(--muted-foreground) 60%, transparent);
	}
	.int-tab__meta {
		display: inline-flex;
		align-items: center;
		gap: 8px;
	}
	.int-tab__count {
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
		font-size: 12px;
		line-height: 16px;
		font-variant-numeric: tabular-nums;
		color: var(--muted-foreground);
	}
	.int-tabs-hint {
		margin-top: 8px;
		font-size: 13px;
		line-height: 16px;
		color: var(--muted-foreground);
	}
	@media (max-width: 767px) {
		.int-tab {
			height: 44px;
		}
	}

	/* Kartu Syarat & Ketentuan */
	.int-terms {
		margin-top: 24px;
		max-width: 760px;
		padding: 24px;
	}
	.int-card-title {
		font-size: 16px;
		line-height: 24px;
		font-weight: 500;
		color: var(--foreground);
	}
	.int-terms__body {
		display: flex;
		flex-direction: column;
		gap: 12px;
		margin-top: 12px;
		font-size: 14px;
		line-height: 22px;
		color: var(--muted-foreground);
	}
	.int-terms__foot {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: 12px 16px;
		margin-top: 24px;
		padding-top: 16px;
		border-top: 1px solid var(--border);
	}
	.int-terms__accepted {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		line-height: 16px;
		color: var(--success);
	}
	.int-consent {
		display: flex;
		align-items: flex-start;
		gap: 10px;
		font-size: 14px;
		line-height: 20px;
		color: var(--foreground);
		cursor: pointer;
	}
	.int-consent input {
		margin-top: 2px;
		width: 16px;
		height: 16px;
		accent-color: var(--primary);
	}
</style>
