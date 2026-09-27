<script lang="ts">
	// Shell klien — struktur & metrik parity product-shell.module.css:
	// sidebar 264px (inner 16/10/12), nav item min-height 40px TANPA margin
	// antar-item, label grup 8/10/5, topbar 64px padding 0 28px,
	// konten 32/28/64.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		Bell, ChartLine, ChevronDown, Coins, House, Megaphone, Newspaper, Receipt,
		ShieldCheck, TriangleAlert, UserCircle
	} from '@lucide/svelte';
	import { apiRequest, ensureValidSession, getStoredSession, showToast, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { swr } from '$lib/swr-cache.svelte';
	import { getI18n } from '$lib/i18n/i18n.svelte';
	import { setBrandUrlId } from '$lib/brand-url.svelte';
	import AppShell from '$lib/components/app-shell.svelte';
	import AccountDropdown from '$lib/components/account-dropdown.svelte';
	import NotificationBell from '$lib/components/notification-bell.svelte';
	import TopUpModal from '$lib/components/credit/topup-modal.svelte';
	import { submitTopUp, type TopUpInvoiceData } from '$lib/credits/topup-client';

	let { children }: { children: import('svelte').Snippet } = $props();

	const brandId = $derived(page.params.brandId as string);
	const pathname = $derived(page.url.pathname);
	const { t } = getI18n();

	let user = $state<{ id: string; email: string; display_name: string; role?: string } | null>(null);

	const overview = swr<any>(
		() => (getStoredSession() ? `/v1/brands/${brandId}/overview` : null),
		async () => {
			const res = await apiRequest(`/v1/brands/${brandId}/overview`);
			if (!res.ok) throw new Error('gagal memuat overview');
			return res.json();
		}
	);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) {
				window.location.href = '/login';
				return;
			}
			user = {
				id: fresh.user.id,
				email: fresh.user.email,
				display_name: fresh.user.display_name || fresh.user.email,
				role: fresh.user.role
			};
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		// Tab checkout gateway menulis kunci ini setelah pembayaran selesai;
		// event `storage` menyegarkan saldo tanpa reload halaman.
		const onStorage = (event: StorageEvent) => {
			if (event.key === 'tauthub:topup-success' || event.key === 'tauthub:topup-cancelled') {
				topupPayment = null;
				topupError = '';
				topupOpen = false;
				topupKey = crypto.randomUUID();
				void overview.refetch();
			}
		};
		window.addEventListener('storage', onStorage);
		return () => {
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
			window.removeEventListener('storage', onStorage);
		};
	});

	const saldo = $derived(overview.data?.wallet?.available_tc ?? 0);

	// Top Up cepat dari utility bar (parity CreditPill app lama). Paket & kurs
	// diambil dari GET /credits agar selalu ikut konfigurasi platform; modal
	// memakai fallback bawaan bila endpoint belum sempat merespons.
	let topupOpen = $state(false);
	let topupBusy = $state(false);
	let topupError = $state('');
	let topupPayment = $state<{ payment_url: string | null; amount_idr: number; tc: number } | null>(null);
	let topupRate = $state(1000);
	let topupPackages = $state<Array<{ tc: number; idr: number }>>([]);
	// Satu intent = satu idempotency key; key dipertahankan saat retry dan hanya
	// di-reset setelah intent selesai/batal (parity credit-pill.tsx).
	let topupKey = '';

	async function openTopUp() {
		topupPayment = null;
		topupError = '';
		topupOpen = true;
		if (!topupKey) topupKey = crypto.randomUUID();
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/credits?limit=1`);
			if (res.ok) {
				const data = await res.json();
				if (data.tc_config?.rate_idr) topupRate = data.tc_config.rate_idr;
				if (Array.isArray(data.tc_config?.topup_packages)) topupPackages = data.tc_config.topup_packages;
			}
		} catch {
			// non-fatal: modal memakai fallback paket & kurs default
		}
	}

	async function startTopUp(tc: number, invoice?: TopUpInvoiceData | null) {
		topupBusy = true;
		topupError = '';
		if (!topupKey) topupKey = crypto.randomUUID();
		try {
			const result = await submitTopUp(`/v1/brands/${brandId}`, tc, topupKey, invoice);
			if (result.status === 'paid') {
				topupKey = crypto.randomUUID();
				topupPayment = null;
				topupOpen = false;
				showToast('Top up ini sudah terbayar. Saldo telah diperbarui.');
				void overview.refetch();
				return;
			}
			if (['expired', 'cancelled', 'failed'].includes(result.status)) {
				topupKey = crypto.randomUUID();
				topupPayment = null;
				topupError = 'Pembayaran sebelumnya batal atau kedaluwarsa. Silakan coba top up lagi.';
				return;
			}
			topupPayment = { payment_url: result.payment_url, amount_idr: result.amount_idr, tc: result.tc };
		} catch (reason) {
			topupError = reason instanceof Error ? reason.message : 'Gagal membuat pembayaran.';
		} finally {
			topupBusy = false;
		}
	}

	function resetTopUpPayment() {
		topupPayment = null;
		topupKey = crypto.randomUUID();
		void overview.refetch();
	}

	// Link internal memakai public_id (6 karakter) seperti app lama; UUID tetap
	// diterima di route sehingga deep-link lama tidak rusak.
	const brandUrlId = $derived(overview.data?.public_id ?? brandId);
	$effect(() => setBrandUrlId(brandUrlId));

	const base = $derived(`/klien/${brandUrlId}`);
	const groups = $derived([
		{
			label: 'MENU UTAMA',
			items: [{ label: t.nav.brand, href: base, icon: House, active: pathname === base }]
		},
		{
			label: 'RUANG KERJA',
			items: [
				{ label: t.nav.media_catalog, href: `${base}/media`, icon: Newspaper, active: pathname.includes('/media') },
				{ label: t.nav.campaigns, href: `${base}/campaigns`, icon: Megaphone, active: pathname.includes('/campaigns') },
				{ label: t.nav.publications, href: `${base}/publications`, icon: ShieldCheck, active: pathname.includes('/publications') },
				{ label: 'Pelacak 404', href: `${base}/link-monitor`, icon: TriangleAlert, active: pathname.includes('/link-monitor') }
			]
		},
		{
			label: 'ANALITIK & PELAPORAN',
			items: [
				{ label: t.nav.analytics, href: `${base}/analytics`, icon: ChartLine, active: pathname.includes('/analytics') },
				{ label: t.nav.audit, href: `${base}/audit-publikasi`, icon: ShieldCheck, active: pathname.includes('/audit-publikasi') }
			]
		},
		{
			label: 'KEUANGAN',
			items: [
				{ label: t.nav.credits, href: `${base}/credits`, icon: Coins, active: pathname.includes('/credits') },
				{ label: t.nav.invoices, href: `${base}/invoices`, icon: Receipt, active: pathname.includes('/invoices') }
			]
		},
		{
			label: 'PENGATURAN',
			items: [{ label: t.nav.profile, href: `${base}/profile`, icon: UserCircle, active: pathname.includes('/profile') }]
		}
	]);
</script>

	<AppShell brandHref={base} navLabel="Navigasi Tauthub" {groups}>
		{#snippet account()}
			{#if user}
				<AccountDropdown
					{user}
					roleLabel="Klien"
					profileHref={`${base}/profile`}
					workspaceSwitchHref={user.role === 'partner' ? '/mitra' : undefined}
					workspaceSwitchLabel={user.role === 'partner' ? 'Portal Mitra' : undefined}
				/>
			{/if}
		{/snippet}

	{#snippet toolbar()}
		<button class="app-btn app-btn-soft !min-h-8 !px-3" type="button" aria-label="Pilih bahasa">
			<span>🇮🇩</span> <span class="app-lang-label">Indonesia</span> <ChevronDown size={14} />
		</button>
		<button
			class="app-credit-pill app-credit-trigger"
			type="button"
			onclick={() => void openTopUp()}
			aria-label={`Saldo Tauthub Kredit: ${saldo.toLocaleString('id-ID')} Kredit. Buka top up.`}
		>
			<Coins size={15} />
			<strong>{saldo.toLocaleString('id-ID')} Kredit</strong>
		</button>
		<NotificationBell apiBase={`/v1/brands/${brandId}`} fallbackBase={base} />
	{/snippet}

	{@render children()}

	{#if topupOpen}
		<TopUpModal
			rateIdr={topupRate}
			packages={topupPackages}
			busy={topupBusy}
			error={topupError}
			payment={topupPayment}
			onSelectPackage={startTopUp}
			onResetPayment={resetTopUpPayment}
			onClose={() => (topupOpen = false)}
		/>
	{/if}
</AppShell>
