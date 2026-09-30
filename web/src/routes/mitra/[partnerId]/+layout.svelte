<script lang="ts">
	import LangSelector from '$lib/components/lang-selector.svelte';
	import { isVipTier, isVvipTier, vipTierBadge } from '$lib/partner/vip-tier';
	import { getI18n } from '$lib/i18n/i18n.svelte';

	import { tr } from '$lib/i18n/phrases';

	// Shell mitra — parity components/shells/product-shell.tsx (cabang partner)
	// dengan metrik product-shell.module.css yang sama seperti shell klien:
	// sidebar 264px (inner 16/10/12), nav item min-height 40px TANPA margin,
	// label grup 8/10/5, topbar 64px padding 0 28px, konten 32/28/64.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
		import {
			Bell, ChevronDown, Coins, Gauge, Gift, House, Link2, Newspaper, PlugZap,
			Receipt, ScrollText, Server, Settings, ShieldCheck, TriangleAlert,
			UserCircle, Wallet
		} from '@lucide/svelte';
	import { apiRequest, ensureValidSession, getStoredSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { swr } from '$lib/swr-cache.svelte';
	import { setPartnerUrlId } from '$lib/partner-url.svelte';
	import AppShell from '$lib/components/app-shell.svelte';
import AnnouncementPopup from '$lib/components/announcement-popup.svelte';
	import AccountDropdown from '$lib/components/account-dropdown.svelte';
	import NotificationBell from '$lib/components/notification-bell.svelte';

	let { children }: { children: import('svelte').Snippet } = $props();

	const partnerId = $derived(page.params.partnerId as string);
	const pathname = $derived(page.url.pathname);
	const i18n = $derived(getI18n());

	let user = $state<{ id: string; email: string; display_name: string; role?: string } | null>(null);

	// /v1/partner/me mengembalikan { user, partner } — public_id dipakai untuk URL.
	const me = swr<any>(
		() => (getStoredSession() ? '/v1/partner/me' : null),
		async () => {
			const res = await apiRequest('/v1/partner/me');
			if (!res.ok) throw new Error('gagal memuat data mitra');
			return res.json();
		}
	);
	const earnings = swr<any>(
		() => (getStoredSession() ? '/v1/partner/earnings' : null),
		async () => {
			const res = await apiRequest('/v1/partner/earnings');
			if (!res.ok) throw new Error('gagal memuat penghasilan');
			return res.json();
		}
	);
	// Status VIP mitra. Menu VIP Dashboard hanya untuk VVIP (tier
	// brand_ambassador) dan tetap tampil walau kontrak habis (hasVip) — parity
	// usePartnerVip app lama. VIP Reguler masuk lewat pill tier di header.
	const vip = swr<any>(
		() => (getStoredSession() ? '/v1/partner/me/vip' : null),
		async () => {
			const res = await apiRequest('/v1/partner/me/vip');
			if (!res.ok) throw new Error('gagal memuat status VIP');
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
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	// Link internal memakai public_id (6 karakter) seperti app lama; UUID tetap
	// diterima di route sehingga deep-link lama tidak rusak.
	const partnerUrlId = $derived(me.data?.partner?.public_id ?? partnerId);
	$effect(() => setPartnerUrlId(partnerUrlId));

	const base = $derived(`/mitra/${partnerUrlId}`);
	const isVip = $derived(Boolean(vip.data?.hasVip ?? vip.data?.isVip));
	// VVIP = tier tertinggi; VIP Reguler tidak memakai menu VIP Dashboard.
	// Sumber kebenaran sama dengan gerbang halaman VIP: tier brand_ambassador.
	// is_vip saja tidak cukup karena VIP Reguler juga is_vip=true.
	const isVvip = $derived(isVvipTier(vip.data?.tier));
	// Pill tier di samping lonceng, hanya untuk mitra keanggotaan VIP.
	const vipPill = $derived(isVip || isVipTier(vip.data?.tier) ? vipTierBadge(vip.data?.tier) : null);
	const saldo = $derived(earnings.data?.summary?.settled ?? 0);

	/**
	 * Menu aktif hanya bila path persis sama atau berada DI BAWAH href-nya.
	 * Pencocokan per segmen, bukan substring: dengan `pathname.includes`,
	 * `/vip-dashboard` ikut menyalakan menu `/vip`.
	 */
	const isActive = (href: string) => pathname === href || pathname.startsWith(`${href}/`);
	const groups = $derived([
		{
			label: i18n.t.nav_partner.group_summary,
			items: [
				{ label: i18n.t.nav_partner.overview, href: base, icon: House, active: pathname === base },
				...(isVvip ? [{ label: i18n.t.nav_partner.vip_dashboard, href: `${base}/vip`, icon: ShieldCheck, active: isActive(`${base}/vip`) }] : [])
			]
		},
		{
			label: i18n.t.nav_partner.group_media,
			items: [
				{ label: tr('Publikasi'), href: `${base}/publications`, icon: Newspaper, active: isActive(`${base}/publications`) },
				{ label: tr('Daftar Media'), href: `${base}/media`, icon: ScrollText, active: isActive(`${base}/media`) },
				{ label: i18n.t.nav_partner.link_monitor, href: `${base}/link-monitor`, icon: TriangleAlert, active: isActive(`${base}/link-monitor`) },
				{ label: i18n.t.nav_partner.crawler, href: `${base}/crawler`, icon: Server, active: isActive(`${base}/crawler`) }
			]
		},
		{
			label: i18n.t.nav_partner.group_tools,
			items: [{ label: i18n.t.nav_partner.seo_tool, href: `${base}/seo-tool`, icon: Gauge, active: isActive(`${base}/seo-tool`) }]
		},
		{
			label: i18n.t.nav_partner.group_referral,
			items: [{ label: tr('Referral Mitra'), href: `${base}/referral`, icon: Gift, active: isActive(`${base}/referral`) }]
		},
		{
			label: i18n.t.nav_partner.group_finance,
			items: [
				{ label: tr('Penghasilan'), href: `${base}/earnings`, icon: Wallet, active: isActive(`${base}/earnings`) },
				{ label: i18n.t.nav_partner.settlements, href: `${base}/settlements`, icon: Receipt, active: isActive(`${base}/settlements`) }
			]
		},
		{
			label: i18n.t.nav_partner.group_settings,
			items: [
				{ label: i18n.t.nav_partner.integrations, href: `${base}/integrations`, icon: PlugZap, active: isActive(`${base}/integrations`) },
				{ label: i18n.t.nav_partner.profile, href: `${base}/profile`, icon: Settings, active: isActive(`${base}/profile`) },
				{ label: i18n.t.nav_partner.notifications, href: `${base}/notifications`, icon: Bell, active: isActive(`${base}/notifications`) }
			]
		}
	]);
</script>

	<AppShell brandHref={base} navLabel="Navigasi portal mitra" {groups}>
		{#snippet account()}
			{#if user}
				<AccountDropdown
					user={{
						id: user.id,
						email: user.email,
						display_name: me.data?.partner?.name || user.display_name,
						role: user.role
					}}
					roleLabel="Mitra"
					profileHref={`${base}/profile`}
					workspaceSwitchHref="/klien"
					workspaceSwitchLabel="Dashboard Klien"
				/>
			{/if}
		{/snippet}

		{#snippet toolbar()}
			<LangSelector />
			<a href={`${base}/earnings`} class="app-credit-pill app-earnings-pill">
				<Coins size={15} />
				<strong>Rp{saldo.toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}</strong>
			</a>
{#if vipPill}
				<a href={isVvip ? `${base}/vip` : `${base}/referral`} class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-[11px] font-semibold"
					title={isVvip ? tr('Kelola keanggotaan VIP Anda') : tr('Lihat komisi referral Anda')}
					style="background-color: {vipPill.bg}; color: {vipPill.fg}; border: 1px solid {vipPill.border};"
				>
					<span class="h-1.5 w-1.5 rounded-full" style="background-color: {vipPill.fg};" aria-hidden="true"></span>
					{vipPill.label}
				</a>
			{/if}
			<NotificationBell apiBase="/v1/partner" fallbackBase={base} showAnnouncementTab />
		{/snippet}

	{@render children()}
	<AnnouncementPopup />

</AppShell>
