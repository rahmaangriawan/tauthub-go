<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Router kampanye Taut Pintar — "lanjutkan dari lokasi terakhir".
	// Port app/klien/[brandId]/taut-pintar/[id]/page.tsx (server redirect) ke
	// redirect sisi klien: SvelteKit tidak punya server redirect per-data di
	// sini, jadi tujuan diputuskan setelah fetch status kampanye.
	//
	//   - profil selesai (profiled, hasil terverifikasi) → The Matchmaker
	//   - sudah ada publikasi / status terminal           → Ringkasan
	//   - produksi                                        → The Writer
	//   - review (QC selesai, menunggu terbit)            → The Writer
	//   - placement (media terkunci)                      → The Writer
	//   - draft/profiling TANPA media                     → The Profiler (jalur lama)
	//   - draft/profiling/media lain dengan media terpasang → The Writer
	//
	// Penanda jalur lama: kampanye wizard lama mengumpulkan medianya SETELAH
	// The Profiler, jadi kampanye berstatus draft dan tanpa media hanya bisa
	// dilanjutkan di halaman itu. Kampanye dari Katalog Media (dan Kilat) sudah
	// membawa media sejak dibuat, jadi lanjutannya adalah produksi artikel —
	// bukan The Profiler yang sudah tidak ditawarkan lagi.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { LoaderCircle } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const base = $derived(`/klien/${getBrandUrlId() || brandId}/taut-pintar/${campaignId}`);

	let error = $state('');

	// Status terminal setelah Gatekeeper submit/distribusi → tampilkan ringkasan.
	const DONE_STATUSES = new Set(['submitted', 'scheduled', 'distributed']);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await route();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function route() {
		const res = await apiRequest(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);
		if (!res.ok) {
			error = tr('Kampanye tidak ditemukan.');
			return;
		}
		const campaign = await res.json();
		const status: string | null = campaign?.taut_status ?? null;

		// Penanda "kampanye SELESAI" yang andal: sudah ada baris publikasi.
		// Dipakai lebih dulu — status string kampanye lama bisa legacy dan tidak
		// bisa dipercaya. Cek lewat matriks Gatekeeper (publication_id/publish).
		const media = await loadAuditMedia();
		const hasPublication = media.some((m) => Boolean(m.publication_id) || Boolean(m.publish?.status));
		if (hasPublication || DONE_STATUSES.has(status ?? '')) {
			await goto(`${base}/ringkasan`, { replaceState: true });
			return;
		}

		// Status 'profiled' diverifikasi datanya: tanpa hasil profiler, matchmaker
		// pasti kosong — kembalikan ke profiler (status bisa salah set dari flow lama).
		if (status === 'profiled') {
			const profiled = await checkProfilerResult();
			await goto(profiled ? `${base}/matchmaker` : `${base}/profiler`, { replaceState: true });
			return;
		}

		// Artikel sudah diproduksi atau QC selesai → The Writer (QC kini di sana).
		if (status === 'production' || status === 'review') {
			await goto(`${base}/writer`, { replaceState: true });
			return;
		}
		// Media sudah terkunci (order dari Katalog Media) → langsung The Writer.
		if (status === 'placement') {
			await goto(`${base}/writer`, { replaceState: true });
			return;
		}

		// Kampanye dari jalur Kilat kembali ke halamannya sendiri — statusnya
		// 'draft' sama seperti kampanye AI Pintar yang belum diisi, jadi penanda
		// entry_mode yang membedakan (migrasi 169).
		if (campaign?.entry_mode === 'kilat') {
			await goto(`${base}/kilat`, { replaceState: true });
			return;
		}

		// Status wizard lama (materials/budget/...) dan tanpa status tanpa media →
		// The Profiler (jalur lama, masih dipakai kampanye yang dibuat sebelum
		// Katalog Media). Kalau medianya sudah terpasang, lanjutannya produksi
		// artikel di The Writer.
		await goto(media.length > 0 ? `${base}/writer` : `${base}/profiler`, { replaceState: true });
	}

	type AuditMedia = { publication_id?: string | null; publish?: { status?: string | null } };

	async function loadAuditMedia(): Promise<AuditMedia[]> {
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/taut/campaigns/${campaignId}/gatekeeper/audits`);
			if (!res.ok) return [];
			const data = await res.json();
			return (data?.media ?? []) as AuditMedia[];
		} catch {
			return [];
		}
	}

	async function checkProfilerResult(): Promise<boolean> {
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/taut/campaigns/${campaignId}/profile/draft`);
			if (!res.ok) return false;
			const data = await res.json();
			return Boolean(data?.result);
		} catch {
			return false;
		}
	}
</script>

<PageTitle title={tr("Taut Pintar — Kampanye")} />

<div class="flex items-center gap-2 py-10 text-sm text-muted-foreground">
	<LoaderCircle size={18} class="animate-spin" />
	{#if error}
		<span style="color: var(--destructive);">{error}</span>
		<a class="app-btn app-btn-soft ml-2" href={`/klien/${getBrandUrlId() || brandId}/taut-pintar`}>{tr("Kembali ke daftar")}</a>
	{:else}
		<span>Mengarahkan ke langkah berikutnya…</span>
	{/if}
</div>
