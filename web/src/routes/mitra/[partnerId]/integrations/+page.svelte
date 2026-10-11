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
	import { CheckCircle2, Lock, Puzzle } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import Connections from './_connections.svelte';
	import PublishingApi from './_publishing-api.svelte';
	import Blogger from './_blogger.svelte';
	import PageTitle from '$lib/components/page-title.svelte';

	type TabKey = 'terms' | 'wordpress' | 'web-custom' | 'blogger';

	let terms = $state({ acceptedAt: null as string | null, loading: true, busy: false, error: '' });
	let tab = $state<TabKey>('terms');
	let termsChecked = $state(false);
	let confirmOpen = $state(false);

	const accepted = $derived(Boolean(terms.acceptedAt));

	function hashTab(): TabKey {
		// Hash bisa membawa query (callback OAuth Blogger: #blogger?blogger_connected=1&media=…),
		// jadi potong di '?' sebelum dibandingkan — parity app lama.
		const h = page.url.hash.replace(/^#/, '').split('?')[0];
		if (h === 'wordpress' || h === 'web-custom' || h === 'blogger') return h;
		return 'terms';
	}

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await loadTerms();
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
</script>

<PageTitle title={tr('Integrasi')} />

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{tr('Integrasi')}</h1>
			<p class="mt-1 text-sm text-muted-foreground">{tr("Kelola koneksi WordPress, Web Custom, dan Blogger Anda dalam satu tempat.")}</p>
		</div>
		{#if !terms.loading}
			{#if accepted}
				<span class="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-[11px] font-medium" style="background: var(--success-soft); color: var(--success);">
					<CheckCircle2 size={12} /> Terverifikasi
				</span>
			{:else}
				<span class="inline-flex items-center gap-1 rounded-full px-2.5 py-0.5 text-[11px] font-medium" style="background: var(--warning-soft); color: var(--warning);">
					<Lock size={12} />{tr("Belum setuju")}</span>
			{/if}
		{/if}
	</header>

	{#if terms.error}
		<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{terms.error}</div>
	{/if}

	{#if terms.loading}
		<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat…")}</p>
	{:else}
		<!-- Mobile: tab digeser horizontal bila tidak muat; desktop tetap wrap. -->
		<div class="mt-4 flex max-md:flex-nowrap max-md:overflow-x-auto md:flex-wrap gap-2">
			{#each [['terms', 'Syarat & Ketentuan'], ['wordpress', 'WordPress'], ['web-custom', 'Web Custom'], ['blogger', 'Blogger']] as item (item[0])}
				{@const value = item[0] as TabKey}
				{@const disabled = !accepted && value !== 'terms'}
				<button
					class="integration-tab app-btn !px-3 !text-xs max-md:shrink-0"
					class:app-btn-primary={tab === value}
					class:app-btn-soft={tab !== value}
					{disabled}
					style={disabled ? 'opacity:.5;' : undefined}
					onclick={() => selectTab(value)}
				>{item[1]}</button>
			{/each}
		</div>

		{#if tab === 'terms'}
			<div class="app-card mt-4 p-5">
				<div class="flex items-center gap-2">
					<Puzzle size={18} />
					<h2 class="text-sm font-semibold">{tr('Syarat & Ketentuan Integrasi')}</h2>
				</div>
				<div class="mt-3 space-y-3 text-sm text-muted-foreground">
					<p>{tr("Dengan menyetujui, Anda memberi Tauthub izin mengirim artikel klien ke akun/website media Anda melalui kanal yang Anda hubungkan di halaman ini (WordPress, Web Custom, atau Blogger).")}</p>
					<p>{tr("Anda bertanggung jawab memastikan kredensial yang dimasukkan valid dan akun yang terhubung milik media Anda. Tauthub tidak pernah mengubah artikel yang sudah terbit tanpa tindakan Anda.")}</p>
					<p>{tr("Persetujuan ini dicatat waktunya dan dapat dicabut dengan memutus koneksi di tab terkait.")}</p>
				</div>

				{#if accepted}
					<p class="mt-4 text-xs" style="color: var(--success);"> {tr("Disetujui pada")} { new Date(terms.acceptedAt as string).toLocaleString('id-ID', { dateStyle: 'long', timeStyle: 'short' })}.
					</p>
				{:else}
					<label class="mt-4 flex items-start gap-2 text-sm">
						<input class="mt-0.5" type="checkbox" bind:checked={termsChecked} />
						<span>{tr('Saya menyetujui Syarat & Ketentuan Integrasi Tauthub di atas.')}</span>
					</label>
					<div class="mt-3 flex justify-end">
						<button class="app-btn app-btn-primary" disabled={!termsChecked || terms.busy} onclick={() => (confirmOpen = true)}>
							{terms.busy ? 'Menyimpan…' : 'Setujui & Lanjutkan'}
						</button>
					</div>
				{/if}
			</div>
		{:else if tab === 'wordpress'}
			<Connections />
		{:else if tab === 'web-custom'}
			<PublishingApi />
		{:else}
			<Blogger />
		{/if}
	{/if}
</section>

{#if confirmOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card w-full max-w-sm p-5" aria-label={tr('Konfirmasi persetujuan')}>
			<h2 class="text-lg font-semibold">{tr('Setujui Syarat & Ketentuan?')}</h2>
			<p class="mt-2 text-sm text-muted-foreground">{tr("Setelah disetujui, tab WordPress, Web Custom, dan Blogger akan terbuka.")}</p>
			<div class="mt-4 flex justify-end gap-2">
				<button class="app-btn app-btn-soft" onclick={() => (confirmOpen = false)}>{tr("Batal")}</button>
				<button class="app-btn app-btn-primary" disabled={terms.busy} onclick={acceptTerms}>{tr('Ya, Setujui')}</button>
			</div>
		</dialog>
	</div>
{/if}

<style>
	/* Tab integrasi dipakai di desktop dan mobile; di layar sentuh (<768px)
		target ketuknya diperbesar. Desktop tetap 32px seperti sebelumnya. */
	.integration-tab {
		min-height: 32px;
	}
	@media (max-width: 767px) {
		.integration-tab {
			min-height: 44px;
		}
	}
</style>
