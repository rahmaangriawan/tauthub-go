<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Pengaturan brand — identitas yang dipakai di seluruh area klien: nama
	// brand, nama klien, industri, dan bahasa utama. Endpoint daftar brand
	// sudah membawa field ini, jadi halaman tidak butuh endpoint baca khusus.
	// Peran viewer/reviewer hanya melihat nilai; server tetap penentu akhir.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Building, LoaderCircle } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
	import Select from '$lib/components/select.svelte';

	const brandId = $derived(page.params.brandId as string);

	type BrandRow = {
		id: string;
		public_id?: string | null;
		name: string;
		role?: string;
		client_name?: string | null;
		industry?: string | null;
		primary_language?: string | null;
	};

	let brand = $state<BrandRow | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state('');
	let notice = $state('');

	let form = $state({ name: '', client_name: '', industry: '', primary_language: 'id-ID' });

	const canEdit = $derived(!brand?.role || !['viewer', 'reviewer'].includes(brand.role));
	const roleLabel = $derived(brand?.role ? brand.role.charAt(0).toUpperCase() + brand.role.slice(1) : '');

	// Nilai bawaan database "id-ID"; en-US disediakan sebagai pilihan kedua.
	// Nilai lain yang tersimpan tetap ditampilkan agar tidak hilang diam-diam.
	const languageOptions = $derived.by(() => {
		const base = [
			{ value: 'id-ID', label: tr('Indonesia') },
			{ value: 'en-US', label: tr('Inggris') }
		];
		if (form.primary_language && !base.some((o) => o.value === form.primary_language)) {
			return [{ value: form.primary_language, label: form.primary_language }, ...base];
		}
		return base;
	});

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		error = '';
		const res = await apiRequest('/v1/brands');
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal memuat pengaturan brand.'));
			return;
		}
		const rows = (await res.json()) as BrandRow[];
		const found = rows.find((b) => (b.public_id ?? b.id) === brandId || b.id === brandId) ?? null;
		if (!found) {
			error = tr('Brand tidak ditemukan atau Anda tidak punya akses ke brand ini.');
			return;
		}
		brand = found;
		form = {
			name: found.name ?? '',
			client_name: found.client_name ?? '',
			industry: found.industry ?? '',
			primary_language: found.primary_language || 'id-ID'
		};
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		if (!canEdit || saving) return;
		const name = form.name.trim();
		if (name.length < 2) {
			error = tr('Nama brand minimal 2 karakter.');
			return;
		}
		saving = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}`, {
				method: 'PUT',
				body: JSON.stringify({
					name,
					client_name: form.client_name.trim(),
					industry: form.industry.trim(),
					primary_language: form.primary_language
				})
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan pengaturan brand.')));
			notice = tr('Pengaturan brand tersimpan.');
			if (brand) brand = { ...brand, name };
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal menyimpan pengaturan brand.');
		} finally {
			saving = false;
		}
	}
</script>

<PageTitle title={tr('Pengaturan Brand')} />

<section>
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">{tr('Pengaturan Brand')}</h1>
		<p class="mt-1 text-sm text-muted-foreground">
			{tr('Identitas brand yang dipakai Tauthub saat menyusun konten dan laporan.')}
		</p>
	</header>

	{#if notice}<p class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</p>{/if}
	{#if error}<p class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);" role="alert">{error}</p>{/if}

	{#if loading}
		<div class="app-card mt-4 flex items-center gap-2 p-5 text-sm text-muted-foreground">
			<LoaderCircle size={16} class="animate-spin" />
			{tr('Memuat pengaturan brand…')}
		</div>
	{:else if brand}
		<form class="app-card mt-4 p-5" onsubmit={save}>
			<div class="flex flex-wrap items-center gap-3">
				<span class="grid h-9 w-9 place-items-center rounded-xl" style="background: var(--accent); color: var(--accent-foreground);">
					<Building size={18} />
				</span>
				<div class="min-w-0">
					<h2 class="truncate font-semibold">{brand.name}</h2>
					<p class="text-xs text-muted-foreground">
						{tr('Peran Anda')}: <span class="font-medium uppercase">{roleLabel || '—'}</span>
					</p>
				</div>
			</div>

			{#if !canEdit}
				<p class="mt-3 rounded-lg px-3 py-2 text-xs" style="background: var(--secondary); color: var(--muted-foreground);">
					{tr('Peran Anda hanya dapat melihat pengaturan ini. Hubungi admin brand untuk mengubahnya.')}
				</p>
			{/if}

			<div class="mt-4 grid gap-4 sm:grid-cols-2">
				<label class="flex flex-col gap-1 text-sm">
					<span class="text-xs font-medium text-muted-foreground">{tr('Nama brand')}</span>
					<input
						class="app-input"
						bind:value={form.name}
						disabled={!canEdit || saving}
						required
						minlength={2}
						maxlength={160}
						placeholder={tr('mis. Giovani Barbershop')}
					/>
					<span class="text-[11.5px] text-muted-foreground">{tr('Nama yang tampil di dashboard dan laporan.')}</span>
				</label>

				<label class="flex flex-col gap-1 text-sm">
					<span class="text-xs font-medium text-muted-foreground">{tr('Nama klien')}</span>
					<input
						class="app-input"
						bind:value={form.client_name}
						disabled={!canEdit || saving}
						maxlength={160}
						placeholder={tr('mis. PT Giovani Indonesia')}
					/>
					<span class="text-[11.5px] text-muted-foreground">{tr('Nama badan usaha atau pihak yang diwakili brand ini.')}</span>
				</label>

				<label class="flex flex-col gap-1 text-sm">
					<span class="text-xs font-medium text-muted-foreground">{tr('Industri')}</span>
					<input
						class="app-input"
						bind:value={form.industry}
						disabled={!canEdit || saving}
						maxlength={120}
						placeholder={tr('mis. jasa perawatan pria')}
					/>
					<span class="text-[11.5px] text-muted-foreground">{tr('Bidang usaha utama. Dipakai sebagai konteks penulisan.')}</span>
				</label>

				<div class="flex flex-col gap-1 text-sm">
					<span class="text-xs font-medium text-muted-foreground">{tr('Bahasa utama')}</span>
					<Select
						bind:value={form.primary_language}
						options={languageOptions}
						ariaLabel={tr('Bahasa utama brand')}
						disabled={!canEdit || saving}
					/>
					<span class="text-[11.5px] text-muted-foreground">{tr('Bahasa konten yang ditulis untuk brand ini.')}</span>
				</div>
			</div>

			{#if canEdit}
				<button class="app-btn app-btn-primary mt-4" type="submit" disabled={saving}>
					{#if saving}<LoaderCircle size={14} class="animate-spin" />{/if}
					{saving ? tr('Menyimpan…') : tr('Simpan pengaturan')}
				</button>
			{/if}
		</form>
	{/if}
</section>
