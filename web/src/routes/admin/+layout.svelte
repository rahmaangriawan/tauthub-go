<script lang="ts">
	import LangSelector from '$lib/components/lang-selector.svelte';
	import MobileUtilityMenu from '$lib/components/mobile-utility-menu.svelte';
	import { getI18n } from '$lib/i18n/i18n.svelte';

	import { tr } from '$lib/i18n/phrases';

	// Shell admin — port components/shells/product-shell.tsx (cabang platform).
	// Metrik & kelas `.app-*` sama dengan shell klien/mitra (token asli app lama).
	// Nav mengikuti grup di app lama + gating permission (super_admin melihat semua).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		Bell, Binoculars, BookOpenText, Bug, ChartBar, ChevronDown, ClipboardList, Coins,
		Gauge, Gift, Headset, House, Megaphone, MessageCircle, Newspaper, PenTool, Percent, Receipt, ScrollText,
		Settings, ShieldCheck, Sparkle, TriangleAlert, Users, Wallet, Zap
	} from '@lucide/svelte';
	import FinanceReportIcon from '$lib/components/icons/finance-report-icon.svelte';
	import SettlementIcon from '$lib/components/icons/settlement-icon.svelte';
	import { apiRequest, ensureValidSession, getStoredSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import AppShell from '$lib/components/app-shell.svelte';
import AnnouncementPopup from '$lib/components/announcement-popup.svelte';
	import AccountDropdown from '$lib/components/account-dropdown.svelte';
	import NotificationBell from '$lib/components/notification-bell.svelte';

	let { children }: { children: import('svelte').Snippet } = $props();

	const pathname = $derived(page.url.pathname);
	const i18n = $derived(getI18n());

	let user = $state<{ id: string; email: string; display_name: string } | null>(null);
	// Badge merah di menu Dukungan: jumlah tiket dengan pesan mitra belum dibaca.
	let supportUnread = $state(0);
	// Badge merah di submenu Direktori: jumlah media yang menunggu review.
	// Dipakai supaya admin tahu ada pengajuan baru tanpa membuka tab Pending.
	let pendingMedia = $state(0);
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
		// Angka tiket berubah saat admin membalas; cukup segarkan berkala dan
		// setiap kali pindah halaman, bukan polling cepat.
		const supportTimer = setInterval(() => {
			if (document.visibilityState === 'visible') void loadSupportUnread();
		}, 30_000);
		supportCleanup = () => clearInterval(supportTimer);
		// Jumlah media menunggu review: segarkan saat tab kembali aktif dan saat
		// admin menyetujui/menolak dari halaman Media (event di bawah).
		const pendingTimer = setInterval(() => {
			if (document.visibilityState === 'visible') void loadPendingMedia();
		}, 30_000);
		pendingCleanup = () => clearInterval(pendingTimer);
		const onPendingRefresh = () => void loadPendingMedia();
		window.addEventListener('tauthub:media-pending-refresh', onPendingRefresh);
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
			window.removeEventListener('tauthub:media-pending-refresh', onPendingRefresh);
			supportCleanup?.();
			pendingCleanup?.();
		};
	});

	let supportCleanup: (() => void) | null = null;
	let pendingCleanup: (() => void) | null = null;

	async function loadSupportUnread() {
		try {
			const res = await apiRequest('/v1/platform/tickets/unread-count');
			if (!res.ok) return;
			const data = await res.json();
			supportUnread = Number(data?.count ?? 0);
		} catch {
			/* badge bersifat informasi; gagal memuat tidak mengganggu navigasi */
		}
	}

	// Jumlah media menunggu review. Memakai daftar media dengan limit=1 dan
	// membaca field total — tidak ada endpoint hitung khusus, dan ini sudah
	// dipakai tab Pending sehingga angkanya pasti sama.
	async function loadPendingMedia() {
		try {
			const res = await apiRequest('/v1/platform/media?limit=1&offset=0&status=pending_review');
			if (!res.ok) return;
			const data = await res.json();
			pendingMedia = Number(data?.total ?? 0);
		} catch {
			/* badge bersifat informasi; gagal memuat tidak mengganggu navigasi */
		}
	}

	// Segarkan badge setiap kali rute berubah (mis. kembali dari halaman tiket).
	$effect(() => {
		pathname;
		if (user) {
			void loadSupportUnread();
			void loadPendingMedia();
		}
	});

	const isSuper = $derived(admin?.role === 'super_admin');
	const can = (key: string) => isSuper || admin?.permissions?.[key] === true;

	// Grup nav parity platformGroups di product-shell.tsx app lama — termasuk
	// item bertingkat (children) seperti Media → Direktori/Media Otomatis/…
	// dan kotak pencarian menu. `permission` kosong berarti selalu tampil.
	const groups = $derived([
		{
			label: i18n.t.nav_admin.group_summary,
			items: [{ label: i18n.t.nav_admin.overview, href: '/admin', icon: House, active: pathname === '/admin' }]
		},
		{
			label: i18n.t.nav_admin.group_taut_ai,
			items: [
				...(isSuper
					? [
							{
								// Taut Pintar tanpa submenu: semua sub-halaman (Monitoring,
								// AI Gambar, Agent Config, Racik Agent, dua agent
								// Matchmaker, dan folder Taut Brief) ditampilkan sebagai
								// folder di halaman explorer /admin/taut-pintar — menu
								// tetap menyala saat user berada di salah satu folder
								// itu. Prompt Angle bukan agent: menu terpisah di bawahnya.
								label: i18n.t.nav_admin.taut_pintar,
								href: '/admin/taut-pintar',
								icon: Sparkle,
								active: pathname.startsWith('/admin/taut-pintar') || pathname.startsWith('/admin/taut-agents') || pathname.startsWith('/admin/taut-brief')
							},
							{
								label: 'Prompt Angle',
								href: '/admin/prompt-angle',
								icon: PenTool,
								active: pathname.startsWith('/admin/prompt-angle')
							},
							{
								label: i18n.t.nav_admin.ai_gateway,
								href: '/admin/ai',
								icon: Zap,
								active: pathname.startsWith('/admin/ai') && !pathname.startsWith('/admin/ai-usage'),
								children: [{ label: 'AI Token Usage', href: '/admin/ai-usage', active: pathname.startsWith('/admin/ai-usage') }]
							}
							// Menu Taut Brief dihapus dari sidebar: sekarang menjadi
							// folder di dalam explorer Taut Pintar (isinya Konfigurasi,
							// Agent Entitas, Agent File). Log-nya tetap tabel
							// taut_brief_* sendiri; dua agent AI Media Matchmaker
							// tetap folder langsung di Taut Pintar.
						]
					: [])
			]
		},
		{
			label: i18n.t.nav_admin.group_media,
			items: [
				...(can('media')
					? [
							{
								label: i18n.t.nav_admin.media,
								href: '/admin/media',
								icon: Newspaper,
								active:
									(pathname.startsWith('/admin/media') &&
										!pathname.startsWith('/admin/media-packages') &&
										!pathname.startsWith('/admin/media-standards') &&
										!pathname.startsWith('/admin/media-otomatis') &&
										!pathname.startsWith('/admin/media-intelligence') &&
										!pathname.startsWith('/admin/media-summaries')) ||
									pathname.startsWith('/admin/link-monitor'),
								children: [
									{
										label: tr('Direktori'),
										href: '/admin/media',
										// Badge merah: ada domain/media menunggu review.
										badge: pendingMedia,
										active:
											pathname.startsWith('/admin/media') &&
											!pathname.startsWith('/admin/media-packages') &&
											!pathname.startsWith('/admin/media-standards') &&
											!pathname.startsWith('/admin/media-otomatis') &&
											!pathname.startsWith('/admin/media-intelligence')
									},
									{ label: i18n.t.nav_admin.media_otomatis, href: '/admin/media-otomatis', active: pathname.startsWith('/admin/media-otomatis') },
									// "Media Intelligence" tidak lagi jadi submenu: isinya sudah
									// menjadi tab Intelligence di /admin/media, dan rutenya
									// hanya redirect ke sana. Menu terpisah jadi jalan buntu.
									{ label: i18n.t.nav_admin.media_packages, href: '/admin/media-packages', active: pathname.startsWith('/admin/media-packages') },
									{ label: i18n.t.nav_admin.media_standards, href: '/admin/media-standards', active: pathname.startsWith('/admin/media-standards') },
									{ label: i18n.t.nav_admin.link_monitor, href: '/admin/link-monitor', active: pathname.startsWith('/admin/link-monitor') },
									{ label: i18n.t.nav_admin.domain_diagnostics, href: '/admin/domain-diagnostics', active: pathname.startsWith('/admin/domain-diagnostics') }
								]
							}
						]
					: []),
					...(can('content')
						? [
								{ label: i18n.t.nav_admin.knowledge_base, href: '/admin/knowledge-bases', icon: BookOpenText, active: pathname.startsWith('/admin/knowledge-bases') }
							]
						: []),
				...(can('publications')
					? [
							{
								// Publikasi = folder explorer: anak-anaknya tampil sebagai
								// kartu folder di /admin/publications (ala Taut Pintar),
								// bukan submenu sidebar.
								label: tr('Publikasi'),
								href: '/admin/publications',
								icon: Megaphone,
								active:
									pathname.startsWith('/admin/publications') ||
									pathname.startsWith('/admin/variants') ||
									pathname.startsWith('/admin/auto-replace') ||
									pathname.startsWith('/admin/rss-feeds') ||
									pathname.startsWith('/admin/orders')
							}
						]
					: []),
				...(can('pr')
					? [
							{
								label: tr('PR & Analitik'),
								href: '/admin/pr-calculator',
								icon: ChartBar,
								active:
									pathname.startsWith('/admin/pr-calculator') ||
									pathname.startsWith('/admin/pr-brand-standards') ||
									pathname.startsWith('/admin/pr-analysis-history'),
								children: [
									{ label: i18n.t.nav_admin.pr_calculator, href: '/admin/pr-calculator', active: pathname.startsWith('/admin/pr-calculator') },
									{ label: i18n.t.nav_admin.pr_brand_standards, href: '/admin/pr-brand-standards', active: pathname.startsWith('/admin/pr-brand-standards') },
									{ label: tr('Riwayat Analisis PR'), href: '/admin/pr-analysis-history', active: pathname.startsWith('/admin/pr-analysis-history') }
								]
							}
						]
						: [])
				]
			},
			// Standar & moderasi konten: satu kelompok sendiri supaya tidak
			// tercampur dengan menu media dan publikasi di atasnya. Grup hanya
			// dirender bila ada isinya, karena label grup selalu tampil.
			...(can('content')
				? [
						{
							label: i18n.t.nav_admin.group_standards,
							items: [
								{ label: i18n.t.nav_admin.content_standards, href: '/admin/content-standards', icon: ClipboardList, active: pathname.startsWith('/admin/content-standards') },
								{ label: i18n.t.nav_admin.dewan_pers, href: '/admin/dewan-pers-reviews', icon: ShieldCheck, active: pathname.startsWith('/admin/dewan-pers-reviews') },
								{ label: i18n.t.nav_admin.content_settings, href: '/admin/content-settings', icon: ClipboardList, active: pathname.startsWith('/admin/content-settings') },
								{ label: i18n.t.nav_admin.moderations, href: '/admin/moderations', icon: ShieldCheck, active: pathname.startsWith('/admin/moderations') }
							]
						}
					]
				: []),
			{
				label: i18n.t.nav_admin.group_users,
			items: [
				...(can('users')
					? [{ label: tr('Klien'), href: '/admin/users', icon: Users, active: pathname.startsWith('/admin/users') }]
					: []),
				...(can('partners')
					? [
							{
								label: i18n.t.nav_admin.partners,
								href: '/admin/partners',
								icon: Users,
								active:
									pathname.startsWith('/admin/partners') ||
									pathname.startsWith('/admin/sindikasi-konten') ||
									pathname.startsWith('/admin/integrations'),
								children: [
									{ label: i18n.t.nav_admin.partners, href: '/admin/partners', active: pathname.startsWith('/admin/partners') },
									{ label: i18n.t.nav_admin.sindikasi, href: '/admin/sindikasi-konten', active: pathname.startsWith('/admin/sindikasi-konten') },
									{ label: tr('Listing Media Mitra'), href: '/admin/integrations', active: pathname.startsWith('/admin/integrations') }
								]
							}
						]
					: []),
				...(can('partners')
					? [
							{
								label: i18n.t.nav_admin.referral,
								href: '/admin/referrals',
								icon: Gift,
								active:
									pathname.startsWith('/admin/referrals') ||
									pathname.startsWith('/admin/referral-audit') ||
									pathname.startsWith('/admin/referred-domains') ||
									pathname.startsWith('/admin/vip-partners'),
								children: [
									{ label: i18n.t.nav_admin.referral, href: '/admin/referrals', active: pathname.startsWith('/admin/referrals') },
									{
										label: tr('Monitoring Non VIP'),
										href: '/admin/referral-audit',
										active: pathname.startsWith('/admin/referral-audit') || pathname.startsWith('/admin/referred-domains')
									},
									{ label: tr('Monitoring VIP'), href: '/admin/vip-partners', active: pathname.startsWith('/admin/vip-partners') }
								]
							}
						]
					: []),
				// Pengumuman, notifikasi, obrolan, dan dukungan dikelompokkan sendiri
				// sebagai KOMUNIKASI supaya tidak tercampur dengan daftar pengguna.
				{ label: tr('Changelog'), href: '/admin/changelog', icon: ScrollText, active: pathname.startsWith('/admin/changelog') }
			]
		},
		{
			label: i18n.t.nav_admin.group_communication,
			items: [
				// Pengumuman terbuka untuk semua platform admin (guard backend sama).
				{ label: tr('Kotak Pengumuman'), href: '/admin/pengumuman', icon: Megaphone, active: pathname.startsWith('/admin/pengumuman') },
				{ label: tr('Notifikasi'), href: '/admin/notifications', icon: Bell, active: pathname.startsWith('/admin/notifications') },
				{ label: tr('Log Obrolan'), href: '/admin/obrolan', icon: MessageCircle, active: pathname.startsWith('/admin/obrolan') },
				{ label: tr('Dukungan'), href: '/admin/dukungan', icon: Headset, active: pathname.startsWith('/admin/dukungan'), badge: supportUnread }
			]
		},
		{
			label: i18n.t.nav_admin.group_monetization,
			items: [
				...(can('payments')
					// Ikon uang (bukan grafik batang): ChartBar sudah dipakai menu
					// "PR & Analitik" di grup lain, jadi laporan keuangan tampak
					// kembar dengan menu itu di sidebar.
					? [{ label: i18n.t.nav_admin.finance_report, href: '/admin/finance-report', icon: FinanceReportIcon, active: pathname.startsWith('/admin/finance-report') }]
					: []),
				// Payments & Invoice dan Packages khusus Super Admin: isinya
				// kredensial payment gateway dan harga paket kredit. Menu lain di
				// grup ini (Keuangan, Revenue Shares, Saldo, Settlements) tetap
				// mengikuti permission masing-masing.
				...(isSuper
					? [
							{
								label: tr('Payments & Invoice'),
								href: '/admin/payments',
								icon: Wallet,
								active: pathname.startsWith('/admin/payments') || pathname.startsWith('/admin/invoices'),
								children: [
									{ label: i18n.t.nav_admin.payments, href: '/admin/payments', active: pathname.startsWith('/admin/payments') },
									{ label: i18n.t.nav_admin.invoices, href: '/admin/invoices', active: pathname.startsWith('/admin/invoices') }
								]
							}
						]
					: []),
				...(can('payments')
					? [{ label: i18n.t.nav_admin.revenue_shares, href: '/admin/revenue-shares', icon: Percent, active: pathname.startsWith('/admin/revenue-shares') }]
					: []),
				...(isSuper
					? [{ label: i18n.t.nav_admin.packages, href: '/admin/packages', icon: Coins, active: pathname.startsWith('/admin/packages') }]
					: []),
				...(can('topup')
					? [
							{
								label: i18n.t.nav_admin.credits,
								href: '/admin/credits',
								icon: Coins,
								active: pathname.startsWith('/admin/credits') || pathname.startsWith('/admin/credit-reports'),
								children: [
									{ label: i18n.t.nav_admin.credits_wallet, href: '/admin/credits', active: pathname.startsWith('/admin/credits') },
									{ label: i18n.t.nav_admin.credit_reports, href: '/admin/credit-reports', active: pathname.startsWith('/admin/credit-reports') }
								]
							}
						]
					: []),
				...(can('settlements')
					? [{ label: i18n.t.nav_admin.settlements, href: '/admin/settlements', icon: SettlementIcon, active: pathname.startsWith('/admin/settlements') }]
					: [])
			]
		},
			{
				label: i18n.t.nav_admin.group_ops,
				items: [
					...(isSuper
						? [
								// Integrasi GSC, GA4 Config, dan WA Gateway pindah ke tab
								// di /admin/konfigurasi; route lamanya tetap hidup sebagai
								// deep link, hanya menu sidebarnya yang dihapus.
								{ label: i18n.t.nav_admin.configuration, href: '/admin/konfigurasi', icon: Settings, active: pathname.startsWith('/admin/konfigurasi') || pathname.startsWith('/admin/integrasi-gsc') || pathname.startsWith('/admin/ga4-configs') || pathname.startsWith('/admin/wa-gateway') }
							]
						: [])
				]
			},
			{
				// Monitoring, laporan, dan audit: satu kelompok sendiri supaya
				// tidak tercampur dengan menu Konfigurasi di atasnya.
				label: i18n.t.nav_admin.group_monitoring,
				items: [
					...(can('errors')
						? [{ label: i18n.t.nav_admin.errors, href: '/admin/errors', icon: TriangleAlert, active: pathname.startsWith('/admin/errors') }]
						: []),
					// Monitoring Order pindah ke folder Publikasi (group MEDIA & KONTEN),
					// jadi tidak lagi berdiri sendiri di bagian Operasional.
					{ label: i18n.t.nav_admin.media_summaries, href: '/admin/media-summaries', icon: Binoculars, active: pathname.startsWith('/admin/media-summaries') },
					...(isSuper
						? [
								{ label: i18n.t.nav_admin.audit_log, href: '/admin/audit-log', icon: ClipboardList, active: pathname.startsWith('/admin/audit-log') },
								{ label: i18n.t.nav_admin.bug_reports, href: '/admin/bug-reports', icon: Bug, active: pathname.startsWith('/admin/bug-reports') },
								{ label: 'Alamat Email Mati', href: '/admin/email-suppressions', icon: TriangleAlert, active: pathname.startsWith('/admin/email-suppressions') },
								{ label: 'Aksi Agent', href: '/admin/agent-actions', icon: ClipboardList, active: pathname.startsWith('/admin/agent-actions') },
								{ label: 'Konfigurasi MCP', href: '/admin/konfigurasi/mcp', icon: ShieldCheck, active: pathname.startsWith('/admin/konfigurasi/mcp') },
								{ label: i18n.t.nav_admin.operations, href: '/admin/operations', icon: Gauge, active: pathname.startsWith('/admin/operations') }
							]
						: [])
				]
			},
		...(isSuper
			? [
					{
						label: i18n.t.nav_admin.group_access,
						items: [
							{ label: i18n.t.nav_admin.roles, href: '/admin/roles', icon: ShieldCheck, active: pathname.startsWith('/admin/roles') },
							{ label: i18n.t.nav_admin.admins, href: '/admin/admins', icon: ShieldCheck, active: pathname.startsWith('/admin/admins') }
						]
					}
				]
			: [])
	]);
</script>

<AppShell brandHref="/admin" navLabel="Navigasi admin" {groups} searchable>
	{#snippet account()}
		{#if user}
			<AccountDropdown
				user={{ ...user, role: 'admin' }}
				roleLabel={isSuper ? 'Super Admin' : 'Admin'}
				profileHref="/admin/konfigurasi"
			/>
		{/if}
	{/snippet}

	{#snippet toolbar()}
		<div class="hidden items-center gap-2 md:flex">
			<LangSelector />
		</div>
		<MobileUtilityMenu />
		<NotificationBell apiBase="/v1/platform/operations" markReadApi="/v1/platform/notifications" fallbackBase="/admin" readable={false} showAnnouncementTab serverCategory={false} />
	{/snippet}

	{@render children()}
	<AnnouncementPopup />

</AppShell>
