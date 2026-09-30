<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Menu dropdown akun pada sidebar — port DropdownMenu accountTrigger
	// dari Next.js product-shell.tsx.
	// Menampilkan info user, link ke profil akun, alih dashboard/workspace (jika ada),
	// link platform admin (jika admin), dan tombol Keluar (logout).
	import { CircleUser, Building, Settings, LogOut, ChevronDown } from '@lucide/svelte';
	import { clearSession } from '$lib/auth';

	let {
		user,
		roleLabel = 'Klien',
		profileHref = '/klien',
		workspaceSwitchHref,
		workspaceSwitchLabel
	}: {
		user: {
			id: string;
			email: string;
			display_name: string;
			role?: string;
		};
		roleLabel?: string;
		profileHref?: string;
		workspaceSwitchHref?: string;
		workspaceSwitchLabel?: string;
	} = $props();

	let open = $state(false);
	let wrapper = $state<HTMLDivElement | null>(null);

	const initials = $derived(
		(user.display_name || user.email || '?')
			.split(/\s+/)
			.map((w) => w[0])
			.slice(0, 2)
			.join('')
			.toUpperCase()
	);

	function logout() {
		clearSession();
		window.location.href = '/login';
	}

	function onClickOutside(event: MouseEvent) {
		if (open && wrapper && !wrapper.contains(event.target as Node)) {
			open = false;
		}
	}

	function onKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && open) {
			open = false;
		}
	}
</script>

<svelte:window onclick={onClickOutside} onkeydown={onKeydown} />

<div class="app-account-wrapper" bind:this={wrapper}>
	<button
		type="button"
		class="app-account-trigger"
		onclick={() => (open = !open)}
		aria-expanded={open}
		aria-label={tr("Buka menu akun")}
	>
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
		<span class="app-role-badge">{roleLabel}</span>
		<ChevronDown
			size={14}
			class={open ? 'shrink-0 transition-transform duration-200 rotate-180' : 'shrink-0 transition-transform duration-200'}
			style="color: var(--sidebar-soft);"
		/>
	</button>

	{#if open}
		<div class="app-account-menu" role="menu" aria-label="Menu akun">
			<div class="app-account-menu-header">
				<span class="app-account-menu-email">{user.email}</span>
				<span class="app-role-badge">{roleLabel}</span>
			</div>

			<div class="app-account-menu-divider"></div>

			{#if workspaceSwitchHref}
				<a href={workspaceSwitchHref} class="app-account-menu-item" role="menuitem">
					<Building size={16} />
					<span>{workspaceSwitchLabel ?? 'Beralih Workspace'}</span>
				</a>
			{/if}

			<a href={profileHref} class="app-account-menu-item" role="menuitem">
				<CircleUser size={16} />
				<span>Profil akun</span>
			</a>

			{#if user.role === 'admin' && !profileHref.startsWith('/admin')}
				<a href="/admin" class="app-account-menu-item" role="menuitem">
					<Settings size={16} />
					<span>Platform admin</span>
				</a>
			{/if}

			<div class="app-account-menu-divider"></div>

			<button
				type="button"
				class="app-account-menu-item app-account-menu-logout"
				role="menuitem"
				onclick={logout}
			>
				<LogOut size={16} />
				<span>{tr("Keluar")}</span>
			</button>
		</div>
	{/if}
</div>

<style>
	.app-account-wrapper {
		position: relative;
		width: 100%;
		margin-top: auto;
	}

	button.app-account-trigger {
		display: flex;
		align-items: center;
		gap: 10px;
		width: 100%;
		min-height: 52px;
		border: 1px solid var(--sidebar-border);
		border-radius: 10px;
		padding: 8px 10px;
		background-color: var(--sidebar-hover);
		color: var(--sidebar-foreground);
		cursor: pointer;
		text-align: left;
		font: inherit;
		transition:
			background-color 160ms cubic-bezier(0.2, 0.8, 0.2, 1),
			border-color 160ms cubic-bezier(0.2, 0.8, 0.2, 1);
		touch-action: manipulation;
	}

	button.app-account-trigger:hover {
		border-color: var(--sidebar-border);
		background-color: color-mix(in srgb, var(--sidebar-hover) 80%, white);
	}

	button.app-account-trigger:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
	}

	.app-account-menu {
		position: absolute;
		bottom: calc(100% + 8px);
		left: 0;
		right: 0;
		z-index: 70;
		display: flex;
		flex-direction: column;
		padding: 6px;
		background-color: var(--popover, #ffffff);
		color: var(--popover-foreground, #111827);
		border: 1px solid var(--border);
		border-radius: 12px;
		box-shadow:
			0 10px 25px rgba(0, 0, 0, 0.12),
			0 2px 6px rgba(0, 0, 0, 0.04);
		animation: accountMenuSlide 0.15s cubic-bezier(0.16, 1, 0.3, 1);
	}

	@keyframes accountMenuSlide {
		from {
			opacity: 0;
			transform: translateY(6px) scale(0.98);
		}
		to {
			opacity: 1;
			transform: translateY(0) scale(1);
		}
	}

	.app-account-menu-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		padding: 8px 10px 6px;
	}

	.app-account-menu-email {
		font-size: 11.5px;
		font-weight: 500;
		color: var(--muted-foreground);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		max-width: 180px;
	}

	.app-account-menu-divider {
		height: 1px;
		margin: 4px 0;
		background-color: var(--border);
	}

	.app-account-menu-item {
		display: flex;
		align-items: center;
		gap: 10px;
		width: 100%;
		padding: 9px 10px;
		min-height: 40px;
		border: none;
		border-radius: 8px;
		background: transparent;
		color: var(--foreground);
		font-size: 13px;
		font-weight: 550;
		text-decoration: none;
		text-align: left;
		cursor: pointer;
		transition:
			background-color 0.12s ease,
			color 0.12s ease;
		touch-action: manipulation;
	}

	.app-account-menu-item:hover,
	.app-account-menu-item:focus-visible {
		background-color: var(--secondary);
		outline: none;
	}

	.app-account-menu-logout {
		color: var(--destructive);
	}

	.app-account-menu-logout:hover,
	.app-account-menu-logout:focus-visible {
		background-color: color-mix(in srgb, var(--destructive) 10%, transparent);
		color: var(--destructive);
	}
</style>
