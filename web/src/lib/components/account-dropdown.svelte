<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Menu dropdown akun pada sidebar — port DropdownMenu accountTrigger
	// dari Next.js product-shell.tsx.
	// Menampilkan info user, link ke profil akun, alih dashboard/workspace (jika ada),
	// link platform admin (jika admin), dan tombol Keluar (logout).
	import { CircleUser, Building, Check, Settings, LogOut, ChevronDown } from '@lucide/svelte';
	import { clearSession } from '$lib/auth';

	// Satu brand di daftar pemilih. `public_id` adalah kunci URL; `id` dipakai
	// sebagai cadangan bila API lama belum mengirim public_id.
	type BrandOption = { id: string; public_id?: string | null; name: string; role?: string };

	let {
		user,
		roleLabel = 'Klien',
		profileHref = '/klien',
		workspaceSwitchHref,
		workspaceSwitchLabel,
		brands = [],
		currentBrandId
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
		/** Brand yang bisa dipilih; baris saat ini ditandai dan tidak ditautkan. */
		brands?: BrandOption[];
		/** public_id (atau UUID) brand yang sedang dibuka. */
		currentBrandId?: string;
	} = $props();

	const brandHref = (brand: BrandOption) => `/klien/${brand.public_id ?? brand.id}`;
	const isCurrent = (brand: BrandOption) => Boolean(currentBrandId) && (brand.public_id ?? brand.id) === currentBrandId;
	// Satu brand tidak perlu pemilih: tidak ada tujuan lain. Selebihnya daftar
	// pendek di atas "Beralih Workspace".
	const showBrands = $derived(brands.length > 1);

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

			{#if showBrands}
				<p class="app-account-section-label">Brand</p>
				<div class="app-account-brands">
					{#each brands as brand (brand.id)}
						{@const current = isCurrent(brand)}
						{#if current}
							<span class="app-account-menu-item app-account-brand-current" role="menuitem" aria-current="true">
								<Check size={16} />
								<span class="app-account-brand-name">{brand.name}</span>
								{#if brand.role}<span class="app-account-brand-role">{brand.role}</span>{/if}
							</span>
						{:else}
							<a href={brandHref(brand)} class="app-account-menu-item" role="menuitem">
								<Building size={16} />
								<span class="app-account-brand-name">{brand.name}</span>
								{#if brand.role}<span class="app-account-brand-role">{brand.role}</span>{/if}
							</a>
						{/if}
					{/each}
				</div>
				<div class="app-account-menu-divider"></div>
			{/if}

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

	/* Pemilih brand: daftar pendek dengan label kecil di atasnya. Bila brand
	   banyak (agensi), daftar digulir agar menu tidak melampaui viewport. */
	.app-account-section-label {
		margin: 2px 0 2px;
		padding: 2px 10px;
		font-size: 10.5px;
		font-weight: 700;
		letter-spacing: 0.08em;
		text-transform: uppercase;
		color: var(--muted-foreground);
	}

	.app-account-brands {
		display: flex;
		flex-direction: column;
		max-height: 200px;
		overflow-y: auto;
	}

	.app-account-brand-name {
		min-width: 0;
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.app-account-brand-role {
		flex-shrink: 0;
		font-size: 10.5px;
		font-weight: 600;
		text-transform: capitalize;
		color: var(--muted-foreground);
	}

	.app-account-brand-current {
		cursor: default;
		background-color: color-mix(in srgb, var(--primary) 8%, transparent);
	}

	.app-account-brand-current:hover {
		background-color: color-mix(in srgb, var(--primary) 8%, transparent);
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
