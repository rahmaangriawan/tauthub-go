<script lang="ts">
	// Shell mitra — parity components/shells/product-shell.tsx (cabang partner)
	// dengan metrik product-shell.module.css yang sama seperti shell klien:
	// sidebar 264px (inner 16/10/12), nav item min-height 40px TANPA margin,
	// label grup 8/10/5, topbar 64px padding 0 28px, konten 32/28/64.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		Bell, ChevronDown, Coins, Gauge, House, Link2, Newspaper, PlugZap,
		Receipt, ScrollText, Server, Settings, Share2, ShieldCheck, TriangleAlert,
		UserCircle, Wallet
	} from '@lucide/svelte';
	import { apiRequest, ensureValidSession, getStoredSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { swr } from '$lib/swr-cache.svelte';
	import { setPartnerUrlId } from '$lib/partner-url.svelte';
	import AppShell from '$lib/components/app-shell.svelte';
	import AccountDropdown from '$lib/components/account-dropdown.svelte';

	let { children }: { children: import('svelte').Snippet } = $props();

	const partnerId = $derived(page.params.partnerId as string);
	const pathname = $derived(page.url.pathname);

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
	// Menu VIP Dashboard tetap tampil selama mitra terdaftar VIP (hasVip),
	// walau masa kontrak sudah habis — parity usePartnerVip di app lama.
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
	const saldo = $derived(earnings.data?.summary?.settled ?? 0);

	const groups = $derived([
		{
			label: 'RINGKASAN',
			items: [
				{ label: 'Ringkasan', href: base, icon: House, active: pathname === base },
				...(isVip ? [{ label: 'VIP Dashboard', href: `${base}/vip`, icon: ShieldCheck, active: pathname.includes('/vip') }] : [])
			]
		},
		{
			label: 'MEDIA & PUBLIKASI',
			items: [
				{ label: 'Publikasi', href: `${base}/publications`, icon: Newspaper, active: pathname.includes('/publications') },
				{ label: 'Daftar Media', href: `${base}/media`, icon: ScrollText, active: pathname.includes('/media') },
				{ label: 'Pelacak 404', href: `${base}/link-monitor`, icon: TriangleAlert, active: pathname.includes('/link-monitor') },
				{ label: 'Akses Crawler', href: `${base}/crawler`, icon: Server, active: pathname.includes('/crawler') }
			]
		},
		{
			label: 'PERALATAN',
			items: [{ label: 'SEO & Performa', href: `${base}/seo-tool`, icon: Gauge, active: pathname.includes('/seo-tool') }]
		},
		{
			label: 'REFERRAL',
			items: [{ label: 'Referral Mitra', href: `${base}/referral`, icon: Share2, active: pathname.includes('/referral') }]
		},
		{
			label: 'KEUANGAN',
			items: [
				{ label: 'Penghasilan', href: `${base}/earnings`, icon: Wallet, active: pathname.includes('/earnings') },
				{ label: 'Invoice', href: `${base}/settlements`, icon: Receipt, active: pathname.includes('/settlements') }
			]
		},
		{
			label: 'PENGATURAN',
			items: [
				{ label: 'Integrasi', href: `${base}/integrations`, icon: PlugZap, active: pathname.includes('/integrations') },
				{ label: 'Konfigurasi', href: `${base}/profile`, icon: Settings, active: pathname.includes('/profile') },
				{ label: 'Notifikasi', href: `${base}/notifications`, icon: Bell, active: pathname.includes('/notifications') }
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
		<button class="app-btn app-btn-soft !min-h-8 !px-3" type="button" aria-label="Pilih bahasa">
			<span>🇮🇩</span> <span class="app-lang-label">Indonesia</span> <ChevronDown size={14} />
		</button>
		<a href={`${base}/earnings`} class="app-credit-pill">
			<Coins size={15} />
			<strong>{saldo.toLocaleString('id-ID')} Saldo</strong>
		</a>
		<a href={`${base}/notifications`} class="rounded-lg p-2 transition-colors hover:bg-secondary" aria-label="Notifikasi">
			<Bell size={18} />
		</a>
	{/snippet}

	{@render children()}
</AppShell>
