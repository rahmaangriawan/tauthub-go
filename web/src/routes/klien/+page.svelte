<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Port app/klien/page.tsx: satu brand → langsung ke dashboard brand itu.
	// Lebih dari satu brand → tampilkan pemilih, karena mengarahkan diam-diam
	// ke brand pertama membuat brand kedua tidak pernah terjangkau. Tanpa brand
	// → tawarkan brand VIP; gagal → state error.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { Building } from '@lucide/svelte';
	import { apiRequest, ensureValidSession, resolveSessionRole, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	type Brand = {
		id: string;
		public_id?: string | null;
		name?: string;
		role?: string;
		workspace_id?: string | null;
		status?: string;
	};

	type PickerState = 'loading' | 'empty' | 'error' | 'picker';

	// Petunjuk brand terakhir; kunci yang sama ditulis shell brand saat dibuka.
	const LAST_BRAND_KEY = 'tauthub:last-brand';
	const ROLE_LABELS: Record<string, string> = {
		owner: 'Pemilik',
		admin: 'Admin',
		strategist: 'Strategist',
		writer: 'Writer',
		reviewer: 'Reviewer',
		viewer: 'Viewer'
	};

	let view = $state('loading' as PickerState);
	let error = $state('');
	let busy = $state(false);
	let brands = $state<Brand[]>([]);
	let lastBrandId = $state('');

	const brandUrlId = (brand: Brand) => brand.public_id ?? brand.id;
	const roleLabel = (role?: string) => (role ? ROLE_LABELS[role] ?? role : '');

	async function provisionVipBrand(navigate: boolean): Promise<string | null> {
		try {
			const res = await apiRequest('/v1/partner/vip/brand', { method: 'POST' });
			if (!res.ok) return null;
			const data = (await res.json()) as { id?: string; public_id?: string | null };
			const pid = data.public_id ?? data.id ?? null;
			if (navigate && pid) await goto(`/klien/${pid}`, { replaceState: true });
			return pid;
		} catch {
			return null;
		}
	}

	onMount(() => {
		try {
			lastBrandId = localStorage.getItem(LAST_BRAND_KEY) ?? '';
		} catch {
			// penyimpanan diblokir: tanpa petunjuk, pemilih tetap berfungsi
		}
		let cancelled = false;
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) {
				await goto('/login');
				return;
			}
			// Gerbang area: admin punya shell sendiri; shell klien bukan tempatnya.
			const role = fresh.user.role ?? (await resolveSessionRole());
			if (role === 'admin') {
				await goto('/admin', { replaceState: true });
				return;
			}
			try {
				const res = await apiRequest('/v1/brands');
				if (!res.ok) {
					if (!cancelled) view = 'error';
					return;
				}
				const rows = (await res.json()) as Brand[];
				if (cancelled) return;
				if (rows.length === 0) {
					const pid = await provisionVipBrand(false);
					if (pid) {
						if (!cancelled) await goto(`/klien/${pid}`, { replaceState: true });
						return;
					}
					// Mitra tanpa workspace (bukan VVIP) tidak boleh manteng area
					// klien: balik ke shell mitra. VVIP sudah tercukupi provisi di
					// atas; klien tanpa brand tetap melihat tawaran brand VIP.
					if (role === 'partner') {
						await goto('/mitra', { replaceState: true });
						return;
					}
					if (!cancelled) view = 'empty';
					return;
				}
				if (rows.length === 1) {
					await goto(`/klien/${brandUrlId(rows[0])}`, { replaceState: true });
					return;
				}
				// Lebih dari satu brand: biarkan pengguna memilih. ID brand
				// terakhir hanya jadi penanda, bukan tujuan otomatis.
				brands = rows;
				view = 'picker';
			} catch {
				if (!cancelled) view = 'error';
			}
		})();

		const onExpired = () => void goto('/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			cancelled = true;
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
		};
	});

	async function createVip() {
		busy = true;
		error = '';
		const pid = await provisionVipBrand(true);
		if (!pid) {
			error = tr('Gagal membuat brand. Pastikan akun VIP sudah aktif.');
			busy = false;
			return;
		}
		busy = false;
	}
</script>

<PageTitle title={tr("Klien")} />

<main class="min-h-screen bg-background px-10 py-10 pb-16">
	<div>
		<header class="mb-8">
			<p class="text-[11px] font-bold uppercase tracking-[0.12em] text-primary">Tauthub</p>
			<h1 class="my-2 text-4xl font-bold tracking-tight">{tr("Dashboard Klien")}</h1>
			{#if view === 'picker'}
				<p class="max-w-[620px] text-[15px] leading-relaxed text-muted-foreground">{tr("Pilih brand yang ingin dibuka. Daftar ini memuat brand pribadi dan brand dari workspace Anda.")}</p>
			{:else}
				<p class="max-w-[620px] text-[15px] leading-relaxed text-muted-foreground">{tr("Mengarahkan Anda ke dashboard brand…")}</p>
			{/if}
		</header>

		{#if view === 'loading'}
			<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
				{#each Array(3) as _, i (i)}
					<div class="grid h-[78px] content-center gap-2 rounded-xl border border-border bg-card p-3.5">
						<div class="h-2.5 w-[34%] animate-pulse rounded-md bg-muted"></div>
						<div class="h-4 w-[62%] animate-pulse rounded-md bg-muted"></div>
						<div class="h-2.5 w-[48%] animate-pulse rounded-md bg-muted"></div>
					</div>
				{/each}
			</div>
		{:else if view === 'picker'}
			<ul class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
				{#each brands as brand (brand.id)}
					{@const pid = brandUrlId(brand)}
					<li>
						<a
							href={`/klien/${pid}`}
							class="flex h-full flex-col gap-2.5 rounded-xl border border-border bg-card p-3.5 transition-colors hover:border-primary/40 hover:bg-secondary"
						>
							<div class="flex items-center justify-between gap-2">
								<span class="grid h-9 w-9 place-items-center rounded-lg bg-secondary text-primary"><Building size={16} /></span>
								{#if brand.role}
									<span class="rounded-full bg-secondary px-2 py-0.5 text-[10.5px] font-semibold text-muted-foreground">{roleLabel(brand.role)}</span>
								{/if}
							</div>
							<span class="truncate text-[15px] font-semibold text-foreground">{brand.name ?? pid}</span>
							{#if pid === lastBrandId}
								<span class="text-[11.5px] font-medium text-primary">{tr("Terakhir dibuka")}</span>
							{:else}
								<span class="text-[11.5px] text-muted-foreground">{tr("Buka dashboard brand")}</span>
							{/if}
						</a>
					</li>
				{/each}
			</ul>
		{:else if view === 'empty'}
			<div class="rounded-[20px] border border-dashed border-border px-6 py-16 text-center text-muted-foreground">
				<Building class="mx-auto size-[30px] opacity-60" />
				<p class="mt-3 text-lg font-semibold text-foreground">Brand VIP belum dibuat</p>
				<span class="mb-2 mt-1 inline-block rounded-full bg-violet-100 px-2 py-0.5 text-xs font-semibold text-violet-700">
					VIP Partner
				</span>
				<p class="mt-2">{tr("Anda dapat membuat brand sendiri untuk mulai menggunakan Dashboard Klien.")}</p>
				{#if error}<p class="mt-2 text-sm text-destructive">{error}</p>{/if}
				<button
					class="app-btn app-btn-primary mt-3"
					disabled={busy}
					onclick={() => void createVip()}
				>
					{busy ? 'Membuat…' : 'Buat Brand VIP'}
				</button>
			</div>
		{:else}
			<div class="rounded-xl border border-destructive/40 bg-destructive/5 px-3.5 py-3 text-sm text-destructive">
				{error || tr('Gagal memuat brand. Silakan muat ulang halaman atau coba lagi nanti.')}
			</div>
		{/if}
	</div>
</main>
