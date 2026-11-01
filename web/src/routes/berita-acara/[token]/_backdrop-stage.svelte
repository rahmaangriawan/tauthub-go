<script lang="ts">
	// Latar halaman publik Berita Acara — parity backdrop-stage.tsx app lama:
	// dashboard Klien Tauthub palsu (sidebar + topbar + panel) di belakang kartu
	// modal, ditimpa dimmer blur. Klik area backdrop (di luar kartu) → /login?mode=register.
	//
	// Semua elemen di sini `aria-hidden` dan `pointer-events: none` — murni
	// dekorasi latar, bukan navigasi yang bisa dipakai.
	import { goto } from '$app/navigation';
	import { tr } from '$lib/i18n/phrases';
	import { getI18n } from '$lib/i18n/i18n.svelte';
	import type { LucideIcon } from '@lucide/svelte';
	import {
		ChartLine, Coins, House, Megaphone, Newspaper, Receipt, ShieldCheck
	} from '@lucide/svelte';

	let { children }: { children: import('svelte').Snippet } = $props();

	// Label nav mengikuti kamus dashboard klien yang sebenarnya supaya backdrop
	// terbaca sebagai dashboard yang sama, bukan dashboard generik.
	const i18n = getI18n();

	type StageItem = { label: string; icon: LucideIcon; active?: boolean };
	type StageGroup = { label: string; items: StageItem[] };

	const GROUPS = $derived<StageGroup[]>([
		{
			label: i18n.t.nav.group_main,
			items: [{ label: i18n.t.nav.brand, icon: House }]
		},
		{
			label: i18n.t.nav.group_workspace,
			items: [
				{ label: i18n.t.nav.media_catalog, icon: Newspaper },
				{ label: i18n.t.nav.campaigns, icon: Megaphone, active: true },
				{ label: i18n.t.nav.publications, icon: ShieldCheck }
			]
		},
		{
			label: i18n.t.nav.group_analytics,
			items: [{ label: i18n.t.nav.analytics, icon: ChartLine }]
		},
		{
			label: i18n.t.nav.group_finance,
			items: [
				{ label: i18n.t.nav.credits, icon: Coins },
				{ label: i18n.t.nav.invoices, icon: Receipt }
			]
		}
	]);

	function onStageClick(event: MouseEvent) {
		// Hanya klik tepat di area stage (bukan anaknya) yang memicu redirect —
		// parity `e.target !== e.currentTarget` app lama.
		if (event.target !== event.currentTarget) return;
		void goto('/login?mode=register');
	}
</script>

<div class="stage" onclick={onStageClick} role="presentation">
	<div class="dash" aria-hidden="true">
		<aside class="dash-sidebar">
			<div class="dash-brand">
				<img src="/tauthub-logo.webp" alt="" width="140" height="35" />
			</div>
			<div class="dash-nav">
				{#each GROUPS as group (group.label)}
					<div class="dash-group">
						<span class="dash-nav-label">{group.label}</span>
						{#each group.items as item (item.label)}
							<div class="dash-nav-item" class:dash-nav-item-active={item.active}>
								<item.icon size={18} />
								<span>{item.label}</span>
							</div>
						{/each}
					</div>
				{/each}
			</div>
			<div class="dash-account">
				<span class="dash-avatar-lg">T</span>
				<div class="dash-account-info">
					<span class="dash-acc-name">{tr("Akun Tauthub")}</span>
					<span class="dash-acc-role">{tr("Klien")}</span>
				</div>
			</div>
		</aside>
		<div class="dash-main">
			<div class="dash-top">
				<span class="dash-search"></span>
				<span class="dash-avatar"></span>
			</div>
			<div class="dash-stats">
				<span class="dash-stat"></span>
				<span class="dash-stat"></span>
				<span class="dash-stat"></span>
				<span class="dash-stat"></span>
			</div>
			<div class="dash-grid">
				<span class="dash-card"></span>
				<span class="dash-card"></span>
				<span class="dash-card"></span>
			</div>
		</div>
	</div>
	<div class="dimmer" aria-hidden="true"></div>
	{@render children()}
</div>

<style>
	.stage {
		position: fixed;
		inset: 0;
		z-index: 0;
		overflow-y: auto;
		display: grid;
		padding: 24px 16px 48px;
		background: var(--background);
	}
	/* Centering via margin auto (bukan place-items center): saat kartu lebih
	   tinggi dari viewport, bagian atasnya tetap bisa di-scroll dan tidak
	   terpotong. */
	.stage > :global(*) {
		margin: auto;
	}
	/* Dashboard palsu: selalu di belakang, tidak menerima klik. */
	.dash {
		position: absolute;
		inset: 0;
		z-index: 0;
		pointer-events: none;
		background: var(--background);
	}
	.dash-sidebar {
		position: fixed;
		top: 0;
		bottom: 0;
		left: 0;
		width: 264px;
		display: flex;
		flex-direction: column;
		padding: 14px 10px 12px;
		border-right: 1px solid var(--sidebar-border);
		background: var(--sidebar);
		color: var(--sidebar-foreground);
	}
	.dash-brand {
		min-height: 44px;
		padding: 0 8px;
		display: flex;
		align-items: center;
	}
	.dash-brand img {
		display: block;
		width: auto;
		height: 30px;
		object-fit: contain;
	}
	.dash-nav {
		min-height: 0;
		flex: 1;
		margin-block: 10px;
		display: grid;
		gap: 12px;
		align-content: start;
		overflow: hidden;
	}
	.dash-group {
		display: grid;
	}
	.dash-nav-label {
		display: block;
		padding: 8px 10px 5px;
		color: var(--sidebar-soft);
		font-size: 11px;
		font-weight: 600;
		letter-spacing: 0.1em;
	}
	.dash-nav-item {
		display: flex;
		align-items: center;
		min-height: 40px;
		gap: 11px;
		padding: 0 10px;
		border-radius: 9px;
		color: var(--sidebar-soft);
		font-size: 13px;
		font-weight: 550;
	}
	.dash-nav-item-active {
		background: var(--sidebar-primary);
		color: var(--sidebar-primary-foreground);
		box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.08);
	}
	.dash-account {
		display: flex;
		align-items: center;
		gap: 10px;
		min-height: 52px;
		margin-top: auto;
		padding: 8px 10px;
		border: 1px solid var(--sidebar-border);
		border-radius: 10px;
		background: var(--sidebar-hover);
	}
	.dash-avatar-lg {
		display: grid;
		place-items: center;
		width: 28px;
		height: 28px;
		flex: none;
		border-radius: 50%;
		background: var(--primary);
		color: var(--primary-foreground);
		font-size: 12px;
		font-weight: 700;
	}
	.dash-account-info {
		display: grid;
		min-width: 0;
		flex: 1;
		gap: 2px;
		line-height: 1.2;
	}
	.dash-acc-name {
		overflow: hidden;
		color: var(--sidebar-foreground);
		font-size: 12px;
		font-weight: 600;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.dash-acc-role {
		overflow: hidden;
		color: var(--sidebar-soft);
		font-size: 10.5px;
		font-weight: 500;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.dash-main {
		margin-left: 264px;
		padding: 20px;
		display: grid;
		gap: 20px;
		align-content: start;
	}
	.dash-top {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		padding: 14px 18px;
		border: 1px solid var(--border);
		border-radius: 14px;
		background: var(--card);
	}
	.dash-search {
		display: block;
		width: 40%;
		height: 26px;
		border-radius: 8px;
		background: var(--muted);
	}
	.dash-avatar {
		display: block;
		width: 34px;
		height: 34px;
		border-radius: 50%;
		background: var(--accent);
	}
	.dash-stats {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 14px;
	}
	.dash-stat {
		display: block;
		height: 92px;
		border-radius: 14px;
		background: var(--card);
		border: 1px solid var(--border);
		box-shadow: var(--shadow-sm);
	}
	.dash-grid {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 16px;
	}
	.dash-card {
		display: block;
		height: 200px;
		border-radius: 16px;
		background: var(--card);
		border: 1px solid var(--border);
		box-shadow: var(--shadow-sm);
	}

	.dimmer {
		position: absolute;
		inset: 0;
		z-index: 1;
		pointer-events: none;
		background: rgba(15, 23, 42, 0.32);
		backdrop-filter: blur(4px);
		-webkit-backdrop-filter: blur(4px);
	}

	@media (max-width: 640px) {
		.stage {
			padding: 16px 12px 40px;
		}
		.dash-sidebar {
			display: none;
		}
		.dash-main {
			margin-left: 0;
			padding: 12px;
		}
		.dash-stats {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
		.dash-grid {
			grid-template-columns: 1fr 1fr;
		}
	}

	@media print {
		.stage {
			position: static;
			overflow: visible;
			padding: 0;
			background: #fff;
		}
		.dash,
		.dimmer {
			display: none;
		}
	}
</style>
