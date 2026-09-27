<script lang="ts">
	// Shell admin — port components/shells/product-shell.tsx (cabang platform).
	// Metrik & kelas `.app-*` sama dengan shell klien/mitra (token asli app lama).
	// Nav mengikuti grup di app lama + gating permission (super_admin melihat semua).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		Bell, Binoculars, BookOpenText, Bug, ChartBar, ChevronDown, ClipboardList, Coins,
		Gauge, Gift, House, Megaphone, Newspaper, Percent, Receipt, ScrollText,
		Settings, ShieldCheck, Sparkle, TriangleAlert, Users, Wallet, Zap
	} from '@lucide/svelte';
	import { apiRequest, ensureValidSession, getStoredSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	let { children }: { children: import('svelte').Snippet } = $props();

	const pathname = $derived(page.url.pathname);

	let user = $state<{ id: string; email: string; display_name: string } | null>(null);
	let admin = $state<{ role: string; permissions: Record<string, boolean> } | null>(null);

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
				display_name: fresh.user.display_name || fresh.user.email
			};
			const res = await apiRequest('/v1/platform/admins/me');
			if (res.ok) {
				const data = await res.json();
				admin = data.admin ?? data;
			}
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	const isSuper = $derived(admin?.role === 'super_admin');
	const can = (key: string) => isSuper || admin?.permissions?.[key] === true;
	const initials = $derived(
		(user?.display_name ?? '?')
			.split(/\s+/)
			.map((w) => w[0])
			.slice(0, 2)
			.join('')
			.toUpperCase()
	);

	// Grup nav parity platformGroups di product-shell.tsx. `permission` kosong
	// berarti selalu tampil untuk admin mana pun.
	const groups = $derived([
		{
			label: 'RINGKASAN',
			items: [{ label: 'Overview', href: '/admin', icon: House, active: pathname === '/admin' }]
		},
		{
			label: 'TAUT & AI',
			items: [
				...(isSuper
					? [
							{ label: 'Taut Pintar', href: '/admin/taut-pintar', icon: Sparkle, active: pathname.startsWith('/admin/taut') },
							{ label: 'AI Gateway', href: '/admin/ai', icon: Zap, active: pathname.startsWith('/admin/ai') }
						]
					: [])
			]
		},
		{
			label: 'MEDIA & KONTEN',
			items: [
				...(can('media')
					? [
							{ label: 'Media', href: '/admin/media', icon: Newspaper, active: pathname.startsWith('/admin/media') && !pathname.startsWith('/admin/media-packages') && !pathname.startsWith('/admin/media-standards') && !pathname.startsWith('/admin/media-otomatis') && !pathname.startsWith('/admin/media-intelligence') && !pathname.startsWith('/admin/media-summaries') },
							{ label: 'Media Otomatis', href: '/admin/media-otomatis', icon: ScrollText, active: pathname.startsWith('/admin/media-otomatis') },
							{ label: 'Media Intelligence', href: '/admin/media-intelligence', icon: Gauge, active: pathname.startsWith('/admin/media-intelligence') },
							{ label: 'Media Packages', href: '/admin/media-packages', icon: Coins, active: pathname.startsWith('/admin/media-packages') },
							{ label: 'Media Standards', href: '/admin/media-standards', icon: ClipboardList, active: pathname.startsWith('/admin/media-standards') },
							{ label: 'Pelacak 404', href: '/admin/link-monitor', icon: TriangleAlert, active: pathname.startsWith('/admin/link-monitor') },
							{ label: 'Diagnostik Domain', href: '/admin/domain-diagnostics', icon: ScrollText, active: pathname.startsWith('/admin/domain-diagnostics') }
						]
					: []),
				...(can('content')
					? [
							{ label: 'Content Standards', href: '/admin/content-standards', icon: ClipboardList, active: pathname.startsWith('/admin/content-standards') },
							{ label: 'Review Dewan Pers', href: '/admin/dewan-pers-reviews', icon: ShieldCheck, active: pathname.startsWith('/admin/dewan-pers-reviews') },
							{ label: 'Knowledge Base', href: '/admin/knowledge-bases', icon: BookOpenText, active: pathname.startsWith('/admin/knowledge-bases') },
							{ label: 'Content Settings', href: '/admin/content-settings', icon: ClipboardList, active: pathname.startsWith('/admin/content-settings') },
							{ label: 'Moderasi', href: '/admin/moderations', icon: ShieldCheck, active: pathname.startsWith('/admin/moderations') }
						]
					: []),
				...(can('publications')
					? [
							{ label: 'Publikasi', href: '/admin/publications', icon: Megaphone, active: pathname.startsWith('/admin/publications') },
							{ label: 'Variant Batches', href: '/admin/variants', icon: Megaphone, active: pathname.startsWith('/admin/variants') },
							{ label: 'Auto-Replace', href: '/admin/auto-replace', icon: Megaphone, active: pathname.startsWith('/admin/auto-replace') },
							{ label: 'RSS Feeds', href: '/admin/rss-feeds', icon: ScrollText, active: pathname.startsWith('/admin/rss-feeds') },
							{ label: 'Sindikasi Konten', href: '/admin/sindikasi-konten', icon: ScrollText, active: pathname.startsWith('/admin/sindikasi-konten') }
						]
					: []),
				...(can('pr')
					? [
							{ label: 'PR Calculator', href: '/admin/pr-calculator', icon: ChartBar, active: pathname.startsWith('/admin/pr-calculator') },
							{ label: 'Standar PR per Brand', href: '/admin/pr-brand-standards', icon: ChartBar, active: pathname.startsWith('/admin/pr-brand-standards') },
							{ label: 'Riwayat Analisis PR', href: '/admin/pr-analysis-history', icon: ChartBar, active: pathname.startsWith('/admin/pr-analysis-history') }
						]
					: [])
			]
		},
		{
			label: 'MITRA & PENGGUNA',
			items: [
				...(can('users')
					? [{ label: 'Klien', href: '/admin/users', icon: Users, active: pathname.startsWith('/admin/users') }]
					: []),
				...(can('partners')
					? [
							{ label: 'Partners', href: '/admin/partners', icon: Users, active: pathname.startsWith('/admin/partners') },
							{ label: 'Listing Media Mitra', href: '/admin/integrations', icon: Users, active: pathname.startsWith('/admin/integrations') }
						]
					: []),
				...(can('brands')
					? [{ label: 'Brands', href: '/admin/brands', icon: Users, active: pathname.startsWith('/admin/brands') }]
					: []),
				...(can('partners')
					? [
							{ label: 'Referral', href: '/admin/referrals', icon: Gift, active: pathname.startsWith('/admin/referrals') },
							{ label: 'Monitoring Non VIP', href: '/admin/referral-audit', icon: Gift, active: pathname.startsWith('/admin/referral-audit') },
							{ label: 'Monitoring VIP', href: '/admin/vip-partners', icon: Gift, active: pathname.startsWith('/admin/vip-partners') },
							{ label: 'Domain Referral', href: '/admin/referred-domains', icon: Gift, active: pathname.startsWith('/admin/referred-domains') }
						]
					: [])
			]
		},
		{
			label: 'MONETISASI',
			items: [
				...(can('payments')
					? [
							{ label: 'Payments', href: '/admin/payments', icon: Wallet, active: pathname.startsWith('/admin/payments') },
							{ label: 'Invoices', href: '/admin/invoices', icon: Receipt, active: pathname.startsWith('/admin/invoices') },
							{ label: 'Revenue Shares', href: '/admin/revenue-shares', icon: Percent, active: pathname.startsWith('/admin/revenue-shares') },
							{ label: 'Packages', href: '/admin/packages', icon: Coins, active: pathname.startsWith('/admin/packages') }
						]
					: []),
				...(can('topup')
					? [
							{ label: 'Tauthub Credit', href: '/admin/credits', icon: Coins, active: pathname.startsWith('/admin/credits') },
							{ label: 'Laporan Topup', href: '/admin/credit-reports', icon: Coins, active: pathname.startsWith('/admin/credit-reports') }
						]
					: []),
				...(can('settlements')
					? [{ label: 'Settlements', href: '/admin/settlements', icon: Wallet, active: pathname.startsWith('/admin/settlements') }]
					: [])
			]
		},
		{
			label: 'OPERASI',
			items: [
				...(isSuper
					? [
							{ label: 'Konfigurasi', href: '/admin/konfigurasi', icon: Settings, active: pathname.startsWith('/admin/konfigurasi') },
							{ label: 'Integrasi GSC', href: '/admin/integrasi-gsc', icon: Gauge, active: pathname.startsWith('/admin/integrasi-gsc') },
							{ label: 'GA4 Config', href: '/admin/ga4-configs', icon: Gauge, active: pathname.startsWith('/admin/ga4-configs') },
							{ label: 'WA Gateway', href: '/admin/wa-gateway', icon: ScrollText, active: pathname.startsWith('/admin/wa-gateway') }
						]
					: []),
				...(can('errors')
					? [{ label: 'Error Reports', href: '/admin/errors', icon: TriangleAlert, active: pathname.startsWith('/admin/errors') }]
					: []),
				...(can('publications')
					? [{ label: 'Monitoring Order', href: '/admin/orders', icon: Receipt, active: pathname.startsWith('/admin/orders') }]
					: []),
				{ label: 'Monitoring Kesimpulan', href: '/admin/media-summaries', icon: Binoculars, active: pathname.startsWith('/admin/media-summaries') },
				...(isSuper
					? [
							{ label: 'Audit Log', href: '/admin/audit-log', icon: ClipboardList, active: pathname.startsWith('/admin/audit-log') },
							{ label: 'Bug Reports', href: '/admin/bug-reports', icon: Bug, active: pathname.startsWith('/admin/bug-reports') },
							{ label: 'Operations', href: '/admin/operations', icon: Gauge, active: pathname.startsWith('/admin/operations') }
						]
					: [])
			]
		},
		...(isSuper
			? [
					{
						label: 'AKSES',
						items: [
							{ label: 'Role', href: '/admin/roles', icon: ShieldCheck, active: pathname.startsWith('/admin/roles') },
							{ label: 'Admins', href: '/admin/admins', icon: ShieldCheck, active: pathname.startsWith('/admin/admins') }
						]
					}
				]
			: [])
	]);
</script>

<div class="app-shell">
	<aside class="app-sidebar">
		<div class="app-sidebar-inner">
			<div class="app-sidebar-brand">
				<a href="/admin" class="text-xl font-extrabold tracking-tight" style="color: var(--primary);">
					taut<span style="color: var(--foreground);">hub</span>
				</a>
			</div>

			<nav class="app-sidebar-nav" aria-label="Navigasi admin">
				{#each groups as group (group.label)}
					<span class="app-sidebar-label">{group.label}</span>
					{#each group.items as item (item.href + item.label)}
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
					<span class="app-role-badge">{isSuper ? 'Super Admin' : 'Admin'}</span>
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
			<a href="/admin/errors" class="rounded-lg p-2 transition-colors hover:bg-secondary" aria-label="Peringatan sistem">
				<Bell size={18} />
			</a>
		</header>
		<main class="app-content">
			{@render children()}
		</main>
	</div>
</div>
