<script lang="ts">
	// Shell klien — parity components/shells/product-shell.tsx: sidebar putih,
	// grup nav berkaps, ikon terisi saat aktif, topbar (bahasa/kredit/bell),
	// user chip. Kelas `.app-*` dari app.css (token asli app lama).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		Bell, ChartLine, ChevronDown, Coins, House, Newspaper, Megaphone, Receipt,
		ShieldCheck, TriangleAlert, UserCircle
	} from '@lucide/svelte';
	import { apiRequest, ensureValidSession, getStoredSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { swr } from '$lib/swr-cache.svelte';
	import { getI18n } from '$lib/i18n/i18n.svelte';

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

	const base = $derived(`/klien/${brandId}`);
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
				{ label: t.nav.audit, href: `${base}/audit-publikasi`, icon: ShieldCheck, active: pathname.includes('/audit') }
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

<div class="flex min-h-screen bg-background">
	<aside
		class="sticky top-0 flex h-screen w-60 shrink-0 flex-col border-r"
		style="background-color: var(--sidebar); border-color: var(--sidebar-border);"
	>
		<div class="px-5 pt-5 pb-1">
			<a href={base} class="text-xl font-extrabold tracking-tight" style="color: var(--primary);">
				taut<span style="color: var(--foreground);">hub</span>
			</a>
		</div>

		<nav class="mt-2 flex-1 overflow-y-auto px-3 pb-3">
			{#each groups as group (group.label)}
				<p class="px-3 pt-4 pb-1 text-[11px] font-semibold tracking-wide" style="color: var(--sidebar-soft);">
					{group.label}
				</p>
				{#each group.items as item (item.href)}
					<a
						href={item.href}
						class="app-sidebar-link mt-0.5"
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
			<div class="m-3 rounded-xl border p-2.5" style="border-color: var(--sidebar-border); background-color: var(--sidebar-hover);">
				<div class="flex items-center gap-2.5">
					<div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-xs font-bold text-primary-foreground" style="background-color: var(--primary);">
						{initials}
					</div>
					<div class="min-w-0 flex-1">
						<p class="truncate text-sm font-semibold">{user.display_name}</p>
						<p class="truncate text-xs" style="color: var(--sidebar-soft);">{user.email}</p>
					</div>
					<span class="shrink-0 rounded-full px-2 py-0.5 text-[10px] font-semibold" style="background-color: var(--accent); color: var(--accent-foreground);">
						Klien
					</span>
					<ChevronDown size={14} class="shrink-0" style="color: var(--sidebar-soft);" />
				</div>
			</div>
		{/if}
	</aside>

	<div class="flex min-w-0 flex-1 flex-col">
		<header class="sticky top-0 z-10 flex items-center justify-end gap-2 border-b bg-card px-6 py-3">
			<button class="app-btn app-btn-soft !py-1.5" type="button" aria-label="Pilih bahasa">
				<span>🇮🇩</span> Indonesia <ChevronDown size={14} />
			</button>
			<a
				href={`${base}/credits`}
				class="flex items-center gap-1.5 rounded-full px-3 py-1.5 text-sm font-semibold"
				style="background-color: var(--warning-soft); color: var(--warning);"
			>
				<Coins size={15} />
				{saldo.toLocaleString('id-ID')} Kredit
			</a>
			<button class="rounded-lg p-2 transition-colors hover:bg-secondary" type="button" aria-label="Notifikasi">
				<Bell size={18} />
			</button>
		</header>
		<main class="min-w-0 flex-1 px-6 py-6">
			{@render children()}
		</main>
	</div>
</div>
