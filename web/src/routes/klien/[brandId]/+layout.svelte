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
	import { apiRequest, ensureValidSession, getStoredSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { swr } from '$lib/swr-cache.svelte';
	import { getI18n } from '$lib/i18n/i18n.svelte';
	import { setBrandUrlId } from '$lib/brand-url.svelte';

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
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	const initials = $derived(
		(user?.display_name ?? '?')
			.split(/\s+/)
			.map((w) => w[0])
			.slice(0, 2)
			.join('')
			.toUpperCase()
	);
	const saldo = $derived(overview.data?.wallet?.available_tc ?? 0);

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

<div class="app-shell">
	<aside class="app-sidebar">
		<div class="app-sidebar-inner">
			<div class="app-sidebar-brand">
				<a href={base} class="text-xl font-extrabold tracking-tight" style="color: var(--primary);">
					taut<span style="color: var(--foreground);">hub</span>
				</a>
			</div>

			<nav class="app-sidebar-nav" aria-label="Navigasi Tauthub">
				{#each groups as group, index (group.label)}
					<!-- Separator antar grup menu (parity app lama: <Separator size="4" my="3" />,
					     tinggi 4px + margin 12px, tidak dipasang sebelum grup pertama). -->
					{#if index > 0}<span class="app-sidebar-separator" aria-hidden="true"></span>{/if}
					<span class="app-sidebar-label">{group.label}</span>
					{#each group.items as item (item.href)}
						<a
							href={item.href}
							class="app-sidebar-link"
							data-active={item.active ? 'true' : 'false'}
							aria-current={item.active ? 'page' : undefined}
						>
							<item.icon size={18} strokeWidth={item.active ? 2.4 : 2} />
							<span>{item.label}</span>
						</a>
					{/each}
				{/each}
			</nav>

			{#if user}
				<div class="app-account-trigger">
					<div
						class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-xs font-bold text-primary-foreground"
						style="background-color: var(--primary);"
					>
						{initials}
					</div>
					<div class="app-account-info">
						<span class="app-account-name">{user.display_name}</span>
						<span class="app-account-email">{user.email}</span>
					</div>
					<span class="app-role-badge">Klien</span>
					<ChevronDown size={14} class="shrink-0" style="color: var(--sidebar-soft);" />
				</div>
			{/if}
		</div>
	</aside>

	<div class="flex min-w-0 flex-1 flex-col">
		<header class="app-utility-bar">
			<button class="app-btn app-btn-soft !min-h-8 !px-3" type="button" aria-label="Pilih bahasa">
				<span>🇮🇩</span> Indonesia <ChevronDown size={14} />
			</button>
			<a href={`${base}/credits`} class="app-credit-pill">
				<Coins size={15} />
				<strong>{saldo.toLocaleString('id-ID')} Kredit</strong>
			</a>
			<button class="rounded-lg p-2 transition-colors hover:bg-secondary" type="button" aria-label="Notifikasi">
				<Bell size={18} />
			</button>
		</header>
		<main class="app-content">
			{@render children()}
		</main>
	</div>
</div>
