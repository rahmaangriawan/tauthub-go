<script lang="ts">
	import LangSelector from '$lib/components/lang-selector.svelte';

	import { tr } from '$lib/i18n/phrases';

	// Shell klien — struktur & metrik parity product-shell.module.css:
	// sidebar 264px (inner 16/10/12), nav item min-height 40px TANPA margin
	// antar-item, label grup 8/10/5, topbar 64px padding 0 28px,
	// konten 32/28/64.
	import { onMount, setContext } from 'svelte';
	import { page } from '$app/state';
	import {
		Bell, ChartLine, ChevronDown, Coins, House, Megaphone, MessageCircle, Newspaper, Receipt,
		ScrollText, ShieldCheck, TriangleAlert, UserCircle
	} from '@lucide/svelte';
	import { apiRequest, ensureValidSession, getStoredSession, resolveSessionRole, showToast, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { swr } from '$lib/swr-cache.svelte';
	import { getI18n } from '$lib/i18n/i18n.svelte';
	import { chatUnread, startChatUnreadPolling, stopChatUnreadPolling } from '$lib/chat/chat-unread.svelte';
	import { setBrandUrlId } from '$lib/brand-url.svelte';
	import AppShell from '$lib/components/app-shell.svelte';
import AnnouncementPopup from '$lib/components/announcement-popup.svelte';
	import AccountDropdown from '$lib/components/account-dropdown.svelte';
	import NotificationBell from '$lib/components/notification-bell.svelte';
	import MobileUtilityMenu from '$lib/components/mobile-utility-menu.svelte';
	import TopUpModal from '$lib/components/credit/topup-modal.svelte';
	import { setToolbarSlot } from '$lib/components/toolbar-slot.svelte';
	import { submitTopUp, type TopUpInvoiceData } from '$lib/credits/topup-client';

	let { children }: { children: import('svelte').Snippet } = $props();

	// Slot sisi kiri utility bar. Pil tahapan Taut Pintar dipindahkan ke sini
	// oleh halamannya (parity agent-steps.tsx yang mem-portal AgentStepsHeader
	// ke utilityBar app lama). Halaman tetap pemilik komponennya, jadi props
	// langkah/tone/locked tidak perlu disalurkan lewat layout.
	const toolbarSlot = setToolbarSlot();
	let toolbarLeftEl = $state<HTMLElement | null>(null);
	$effect(() => {
		toolbarSlot.el = toolbarLeftEl;
	});

	const brandId = $derived(page.params.brandId as string);
	const pathname = $derived(page.url.pathname);
	// Objek i18n disimpan utuh; `t` dibaca lewat getter agar reaktif.
	const i18n = $derived(getI18n());
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
			// Gerbang tenancy shell: data API sudah menolak non-member per brand,
			// tapi shell-nya tidak boleh bisa dibuka oleh role yang salah.
			// Admin langsung ditolak; mitra hanya boleh lanjut bila benar-benar
			// member brand ini (Client Workspace VVIP via brand VIP-nya).
			const role = fresh.user.role ?? (await resolveSessionRole());
			if (role === 'admin') {
				window.location.href = '/admin';
				return;
			}
			if (role === 'partner') {
				const access = await apiRequest(`/v1/brands/${brandId}/overview`);
				if (!access.ok) {
					window.location.href = '/mitra';
					return;
				}
			}
			user = {
				id: fresh.user.id,
				email: fresh.user.email,
				display_name: fresh.user.display_name || fresh.user.email,
				role: fresh.user.role
			};
		})();
		// Badge menu "Pesan": poll ringan jumlah pesan belum dibaca.
		const brandIdNow = page.params.brandId as string;
		startChatUnreadPolling(`/v1/brands/${brandIdNow}`);
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
			stopChatUnreadPolling();
		};
	});

	const saldo = $derived(overview.data?.wallet?.available_tc ?? 0);
	// Tampilan klien berbahasa rupiah: saldo kredit dikonversi memakai kurs aktif
	// dari data saldo (fallback ke kurs modal top up, lalu Rp 1.000).
	const saldoRate = $derived(
		Number(overview.data?.wallet?.rate_idr ?? overview.data?.tc_config?.rate_idr) || 1000
	);
	const fmtIDRPill = (tc: number) => `Rp ${(tc * saldoRate).toLocaleString('id-ID', { maximumFractionDigits: 0 })}`;

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

	// Jembatan untuk halaman anak (katalog media): membuka modal top up yang sama
	// tanpa menyalin alur pembayarannya. onClosed dipanggil saat modal tertutup
	// atau setelah pembayaran berhasil, supaya halaman pemanggil bisa menyegarkan
	// saldo yang ditampilkannya.
	let topupClosedHook: (() => void) | null = null;

	setContext('brandTopUp', {
		open(onClosed?: () => void) {
			topupClosedHook = onClosed ?? null;
			void openTopUp();
		}
	});

	function closeTopUp() {
		topupOpen = false;
		const hook = topupClosedHook;
		topupClosedHook = null;
		if (hook) hook();
	}

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
				closeTopUp();
				showToast(tr('Top up ini sudah terbayar. Saldo telah diperbarui.'));
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
			topupError = reason instanceof Error ? reason.message : tr('Gagal membuat pembayaran.');
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
	/**
	 * Menu aktif hanya bila path persis sama atau berada DI BAWAH href-nya.
	 * Pencocokan per segmen, bukan substring: dengan `pathname.includes`, halaman
	 * `/taut-pintar/.../profiler` ikut menyalakan menu `/profile` karena kata
	 * "profiler" memuat "profile".
	 */
	const isActive = (href: string) => pathname === href || pathname.startsWith(`${href}/`);
	const groups = $derived([
		{
			label: i18n.t.nav.group_main,
			items: [{ label: i18n.t.nav.brand, href: base, icon: House, active: pathname === base }]
		},
		{
			label: i18n.t.nav.group_workspace,
			items: [
				{ label: i18n.t.nav.media_catalog, href: `${base}/media`, icon: Newspaper, active: isActive(`${base}/media`) },
				{
					label: i18n.t.nav.campaigns,
					href: `${base}/campaigns`,
					icon: Megaphone,
					active: isActive(`${base}/campaigns`) || isActive(`${base}/profiler`)
				},
				{ label: i18n.t.nav.publications, href: `${base}/publications`, icon: ShieldCheck, active: isActive(`${base}/publications`) },
				{ label: i18n.t.nav.link_monitor, href: `${base}/link-guardian`, icon: TriangleAlert, active: isActive(`${base}/link-guardian`) }
			]
		},
		{
			label: i18n.t.nav.group_analytics,
			items: [
				{ label: i18n.t.nav.analytics, href: `${base}/analytics`, icon: ChartLine, active: isActive(`${base}/analytics`) },
				{ label: i18n.t.nav.audit, href: `${base}/audit-publikasi`, icon: ShieldCheck, active: isActive(`${base}/audit-publikasi`) }
			]
		},
		{
			label: i18n.t.nav.group_finance,
			items: [
				{ label: i18n.t.nav.credits, href: `${base}/credits`, icon: Coins, active: isActive(`${base}/credits`) },
				{ label: i18n.t.nav.invoices, href: `${base}/invoices`, icon: Receipt, active: isActive(`${base}/invoices`) }
			]
		},
		{
			label: i18n.t.nav.group_settings,
			items: [
				{ label: i18n.t.nav.profile, href: `${base}/profile`, icon: UserCircle, active: isActive(`${base}/profile`) },
				{ label: i18n.t.nav.chat, href: `${base}/chat`, icon: MessageCircle, active: isActive(`${base}/chat`), badge: chatUnread.count },
				{ label: i18n.t.nav.changelog, href: `${base}/changelog`, icon: ScrollText, active: isActive(`${base}/changelog`) }
			]
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
					workspaceSwitchLabel={user.role === 'partner' ? tr('Portal Mitra') : undefined}
				/>
			{/if}
		{/snippet}

	{#snippet toolbarLeft()}
		<div bind:this={toolbarLeftEl} class="flex min-w-0 flex-wrap items-center gap-2"></div>
	{/snippet}

	{#snippet toolbar()}
		<!-- Desktop: bahasa + saldo sebaris. Mobile: keduanya masuk menu ikon. -->
		<div class="hidden items-center gap-2 md:flex">
			<LangSelector />
			<button
				class="app-credit-pill app-credit-trigger"
				type="button"
				onclick={() => void openTopUp()}
				aria-label={`Saldo Tauthub: ${fmtIDRPill(saldo)}. Buka top up.`}
			>
				<Coins size={15} />
				<strong>{fmtIDRPill(saldo)}</strong>
			</button>
		</div>
		<MobileUtilityMenu saldoLabel={fmtIDRPill(saldo)} onTopUp={() => void openTopUp()} />
		<NotificationBell apiBase={`/v1/brands/${brandId}`} fallbackBase={base} showAnnouncementTab />
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
			onClose={closeTopUp}
		/>
	{/if}
	<AnnouncementPopup />

</AppShell>
