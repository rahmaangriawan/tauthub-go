<script lang="ts">
	// Port components/shells/product-shell.tsx + app-shell.tsx (fase UI klien):
	// sidebar grup nav, topbar (bahasa, kredit, bell), user chip. Nav pakai key
	// i18n dictionaries (parity label).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		Bell, ChevronDown, CreditCard, Flag, Globe, Home, LineChart,
		Receipt, ShieldCheck, Wallet
	} from '@lucide/svelte';
	import {
		apiRequest, ensureValidSession, getStoredSession, AUTH_EXPIRED_EVENT
	} from '$lib/auth';
	import { swr } from '$lib/swr-cache.svelte';
	import { getI18n } from '$lib/i18n/i18n.svelte';

	let { children }: { children: import('svelte').Snippet } = $props();

	const brandId = $derived(page.params.brandId as string);
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

	const groups = $derived([
		{
			label: 'MENU UTAMA',
			items: [{ label: 'Beranda', icon: Home, href: `/klien/${brandId}`, key: 'brand' }]
		},
		{
			label: 'RUANG KERJA',
			items: [
				{ label: 'Katalog Media', icon: Globe, href: `/klien/${brandId}/media`, key: 'media_catalog' },
				{ label: 'Kampanye', icon: Flag, href: `/klien/${brandId}/campaigns`, key: 'campaigns' },
				{ label: 'Status Publikasi', icon: ShieldCheck, href: `/klien/${brandId}/publications`, key: 'publications' },
				{ label: 'Pelacak 404', icon: LineChart, href: `/klien/${brandId}/link-monitor`, key: 'audit' }
			]
		},
		{
			label: 'ANALITIK & PELAPORAN',
			items: [
				{ label: 'Analisis Kinerja', icon: LineChart, href: `/klien/${brandId}/analytics`, key: 'analytics' },
				{ label: 'Audit Publikasi', icon: ShieldCheck, href: `/klien/${brandId}/audit`, key: 'audit' }
			]
		},
		{
			label: 'KEUANGAN',
			items: [
				{ label: 'Kredit Saldo', icon: Wallet, href: `/klien/${brandId}/credits`, key: 'credits' },
				{ label: 'Invoice & Pajak', icon: Receipt, href: `/klien/${brandId}/invoices`, key: 'invoices' }
			]
		},
		{
			label: 'PENGATURAN',
			items: [{ label: 'Profil & Akun', icon: CreditCard, href: `/klien/${brandId}/profile`, key: 'profile' }]
		}
	]);

</script>

<div class="flex min-h-screen bg-[#f6f8fb]">
	<!-- Sidebar -->
	<aside class="sticky top-0 flex h-screen w-60 shrink-0 flex-col border-r border-border bg-white">
		<div class="px-5 pt-5">
			<a href={`/klien/${brandId}`} class="text-xl font-extrabold tracking-tight text-[#1d4ed8]">
				taut<span class="text-[#0f172a]">hub</span>
			</a>
		</div>
		<nav class="mt-4 flex-1 overflow-y-auto px-3 pb-4">
			{#each groups as group (group.label)}
				<p class="px-2 pt-4 pb-1 text-[11px] font-semibold tracking-wide text-slate-400">{group.label}</p>
				{#each group.items as item (item.key)}
					{@const active = page.url.pathname === item.href}
					<a
						href={item.href}
						class="mt-0.5 flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm font-medium transition-colors {active
							? 'bg-[#1d4ed8] text-white'
							: 'text-slate-600 hover:bg-slate-100'}"
					>
						<item.icon size={17} />
						{item.label}
					</a>
				{/each}
			{/each}
		</nav>
		{#if user}
			<div class="mx-3 mb-4 flex items-center gap-2.5 rounded-xl border border-border p-2.5">
				<div class="flex h-9 w-9 items-center justify-center rounded-full bg-[#1d4ed8] text-xs font-bold text-white">
					{initials}
				</div>
				<div class="min-w-0 flex-1">
					<p class="truncate text-sm font-semibold">{user.display_name}</p>
					<p class="truncate text-xs text-muted-foreground">{user.email}</p>
				</div>
				<span class="rounded-full bg-blue-100 px-2 py-0.5 text-[10px] font-semibold text-[#1d4ed8]">Klien</span>
				<ChevronDown size={14} class="text-slate-400" />
			</div>
		{/if}
	</aside>

	<!-- Main + topbar -->
	<div class="flex min-w-0 flex-1 flex-col">
		<header class="sticky top-0 z-10 flex items-center justify-end gap-4 border-b border-border bg-white px-6 py-3">
			<button class="flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-sm font-medium hover:bg-slate-100">
				<span>🇮🇩</span> Indonesia <ChevronDown size={14} />
			</button>
			<a
				href={`/klien/${brandId}/credits`}
				class="flex items-center gap-1.5 rounded-full bg-amber-50 px-3 py-1.5 text-sm font-semibold text-amber-700 ring-1 ring-amber-200"
			>
				<CreditCard size={15} />
				{saldo.toLocaleString('id-ID')} Kredit
			</a>
			<button class="rounded-lg p-2 hover:bg-slate-100" aria-label="Notifikasi">
				<Bell size={18} />
			</button>
		</header>
		<main class="min-w-0 flex-1 px-6 py-6">
			{@render children()}
		</main>
	</div>
</div>
