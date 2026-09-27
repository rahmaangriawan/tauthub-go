<script lang="ts">
	// Berita acara kampanye Taut Pintar — dokumen siap cetak.
	// Port app/klien/[brandId]/taut-pintar/[id]/berita-acara/page.tsx.
	//
	// App lama membaca langsung dari DB (rows/queries tanpa endpoint). Backend Go
	// tidak menyediakan endpoint khusus berita acara, jadi data disusun dari
	// endpoint yang ada: brand + campaign detail + matriks Gatekeeper. Timestamp
	// kampanye (created_at/updated_at) tidak diekspos endpoint detail campaign —
	// ditampilkan "-" apa adanya, tanpa mengarang nilai.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Printer } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { MEDIA_LABEL, BUDGET_LABEL, STAGE_LABEL, fmtDateWib } from '../../_shared';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);

	type PlacementRow = {
		media_id: string;
		media_name: string;
		title: string | null;
		article_status: string | null;
		pub_status: string | null;
		published_url: string | null;
		published_at: string | null;
	};

	let loading = $state(true);
	let notFound = $state(false);
	let campaignName = $state('');
	let brandName = $state('');
	let businessProfile = $state<Record<string, unknown> | null>(null);
	let budgetConfig = $state<unknown>(null);
	let mediaType = $state<string | null>(null);
	let budgetStrategy = $state<string | null>(null);
	let tautStatus = $state<string | null>(null);
	let stats = $state<{ articles: number; published_media: number; tc_used: number } | null>(null);
	let placements = $state<PlacementRow[]>([]);

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
		const [campRes, brandRes, auditRes] = await Promise.all([
			apiRequest(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`),
			apiRequest(`/v1/brands/${brandId}`),
			apiRequest(`/v1/brands/${brandId}/taut/campaigns/${campaignId}/gatekeeper/audits`)
		]);
		if (!campRes.ok) {
			notFound = true;
			return;
		}
		const c = await campRes.json();
		campaignName = c.name ?? '';
		businessProfile = typeof c.business_profile === 'object' ? c.business_profile : null;
		budgetConfig = typeof c.budget_config === 'object' ? c.budget_config : null;
		mediaType = c.media_type ?? null;
		budgetStrategy = c.budget_strategy ?? null;
		tautStatus = c.taut_status ?? null;
		stats = c.stats ?? null;

		if (brandRes.ok) {
			const b = await brandRes.json();
			brandName = b?.name ?? '';
		}

		if (auditRes.ok) {
			const a = await auditRes.json();
			const media: Array<{
				media_id: string;
				name: string;
				article?: { title?: string | null };
				publish?: { status?: string | null; url?: string | null };
				scheduled_at?: string | null;
			}> = a?.media ?? [];
			placements = media.map((m) => ({
				media_id: m.media_id,
				media_name: m.name,
				title: m.article?.title ?? null,
				article_status: null,
				pub_status: m.publish?.status ?? null,
				published_url: m.publish?.url ?? null,
				published_at: m.publish?.status === 'published' ? (m.scheduled_at ?? null) : null
			}));
		}
	}

	const businessName = $derived(typeof businessProfile?.business === 'string' ? (businessProfile.business as string) : '-');
	const businessIndustry = $derived(typeof businessProfile?.industry === 'string' ? (businessProfile.industry as string) : '-');
	const budgetCfgText = $derived(budgetConfig ? JSON.stringify(budgetConfig) : '-');
</script>

<PageTitle title="Berita Acara Kampanye" />

<main class="py-4">
	<div class="no-print mb-4 flex items-center justify-between gap-3">
		<a class="app-btn app-btn-soft" href={`/klien/${brandUrl}/taut-pintar/${campaignId}`}>Kembali ke wizard</a>
		<button class="app-btn app-btn-soft" type="button" onclick={() => window.print()}>
			<Printer size={15} /> Cetak
		</button>
	</div>

	{#if loading}
		<p class="text-sm text-muted-foreground">Memuat berita acara…</p>
	{:else if notFound}
		<div class="app-card px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">Kampanye tidak ditemukan untuk brand ini.</p>
			<a class="mt-2 inline-block text-sm" style="color: var(--primary);" href={`/klien/${brandUrl}/taut-pintar`}>Kembali ke daftar kampanye</a>
		</div>
	{:else}
		<div class="app-card p-8">
			<h1 class="text-center text-xl font-bold">Berita Acara Kampanye Distribusi Konten</h1>
			<p class="mt-1 text-center text-sm text-muted-foreground">
				Taut Pintar · diterbitkan otomatis dari data kampanye saat halaman dibuka
			</p>

			<div class="mt-6 grid grid-cols-[minmax(160px,220px)_1fr] gap-x-4 gap-y-2 border-y py-4 text-sm">
				<span class="font-medium text-muted-foreground">Nama kampanye</span><span>{campaignName}</span>
				<span class="font-medium text-muted-foreground">Brand / klien</span><span>{brandName}</span>
				<span class="font-medium text-muted-foreground">Profil bisnis</span><span>{businessName}</span>
				<span class="font-medium text-muted-foreground">Industri</span><span>{businessIndustry}</span>
				<span class="font-medium text-muted-foreground">Jenis media target</span><span>{MEDIA_LABEL[String(mediaType ?? '')] ?? '-'}</span>
				<span class="font-medium text-muted-foreground">Strategi anggaran</span><span>{BUDGET_LABEL[String(budgetStrategy ?? '')] ?? '-'}</span>
				<span class="font-medium text-muted-foreground">Nominal / konfigurasi</span><span class="break-all">{budgetCfgText}</span>
				<span class="font-medium text-muted-foreground">Tahap saat ini</span><span>{STAGE_LABEL[String(tautStatus ?? 'draft')] ?? 'Draft'}</span>
				<span class="font-medium text-muted-foreground">Dibuat</span><span>{fmtDateWib(null)} WIB</span>
				<span class="font-medium text-muted-foreground">Diperbarui</span><span>{fmtDateWib(null)} WIB</span>
			</div>

			<div class="mt-5 grid grid-cols-3 gap-3">
				<div class="rounded-lg border p-3 text-center">
					<p class="text-xs text-muted-foreground">Artikel diproduksi</p>
					<p class="text-lg font-bold">{stats?.articles ?? 0}</p>
				</div>
				<div class="rounded-lg border p-3 text-center">
					<p class="text-xs text-muted-foreground">Media terbit</p>
					<p class="text-lg font-bold">{stats?.published_media ?? 0}</p>
				</div>
				<div class="rounded-lg border p-3 text-center">
					<p class="text-xs text-muted-foreground">TC terpakai</p>
					<p class="text-lg font-bold">{stats?.tc_used ?? 0} TC</p>
				</div>
			</div>

			<div class="mt-6 overflow-x-auto">
				<table class="w-full border-collapse text-sm">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="border-b px-2 py-2 font-medium">Media</th>
							<th class="border-b px-2 py-2 font-medium">Judul artikel</th>
							<th class="border-b px-2 py-2 font-medium">Status publikasi</th>
							<th class="border-b px-2 py-2 font-medium">Tayang / URL</th>
						</tr>
					</thead>
					<tbody>
						{#if placements.length === 0}
							<tr><td class="border-b px-2 py-2 text-muted-foreground" colspan="4">Belum ada media terpilih untuk kampanye ini.</td></tr>
						{:else}
							{#each placements as r (r.media_id)}
								<tr>
									<td class="border-b px-2 py-2">{r.media_name}</td>
									<td class="border-b px-2 py-2">{r.title ?? '-'}</td>
									<td class="border-b px-2 py-2">{r.pub_status ?? '-'}</td>
									<td class="border-b px-2 py-2 break-all">
										{r.published_at ? `${fmtDateWib(r.published_at)} WIB · ` : ''}
										{#if r.published_url}
											<a style="color: var(--primary);" href={r.published_url}>{r.published_url}</a>
										{:else}-{/if}
									</td>
								</tr>
							{/each}
						{/if}
					</tbody>
				</table>
			</div>

			<p class="mt-4 text-[13px] text-muted-foreground">
				Dokumen ini dihasilkan dari data sistem pada saat dicetak. Angka artikel, media terbit, dan TC terpakai
				adalah hasil penjumlahan agregat basis data (publikasi terbit, ledger debit TC) milik kampanye ini.
			</p>

			<div class="mt-10 flex justify-around gap-6 text-sm">
				<div class="min-w-0 flex-1 text-center sm:min-w-[200px] sm:flex-none">
					<div class="h-16"></div>
					<div class="border-t pt-2">Pihak klien</div>
				</div>
				<div class="min-w-0 flex-1 text-center sm:min-w-[200px] sm:flex-none">
					<div class="h-16"></div>
					<div class="border-t pt-2">Tauthub</div>
				</div>
			</div>
		</div>
	{/if}
</main>

<style>
	@media print {
		.no-print {
			display: none !important;
		}
	}
</style>
