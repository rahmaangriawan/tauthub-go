<script lang="ts">
	// Router kampanye Taut Pintar — "lanjutkan dari lokasi terakhir".
	// Port app/klien/[brandId]/taut-pintar/[id]/page.tsx (server redirect) ke
	// redirect sisi klien: SvelteKit tidak punya server redirect per-data di
	// sini, jadi tujuan diputuskan setelah fetch status kampanye.
	//
	//   - profil selesai (profiled, hasil terverifikasi) → The Matchmaker
	//   - sudah ada publikasi / status terminal           → Ringkasan
	//   - produksi                                        → The Writer
	//   - review                                          → The Gatekeeper
	//   - placement (media terkunci)                      → The Writer
	//   - sisanya (draft/profiling/status wizard lama)    → The Profiler
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { LoaderCircle } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

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
			error = 'Kampanye tidak ditemukan.';
			return;
		}
		const campaign = await res.json();
		const status: string | null = campaign?.taut_status ?? null;

		// Penanda "kampanye SELESAI" yang andal: sudah ada baris publikasi.
		// Dipakai lebih dulu — status string kampanye lama bisa legacy dan tidak
		// bisa dipercaya. Cek lewat matriks Gatekeeper (publication_id/publish).
		const hasPublication = await checkPublication();
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

		// Artikel sudah diproduksi → The Writer; QC/review → The Gatekeeper.
		if (status === 'production') {
			await goto(`${base}/writer`, { replaceState: true });
			return;
		}
		if (status === 'review') {
			await goto(`${base}/gatekeeper`, { replaceState: true });
			return;
		}
		// Media sudah terkunci (order dari Katalog Media) → langsung The Writer.
		if (status === 'placement') {
			await goto(`${base}/writer`, { replaceState: true });
			return;
		}

		// Status wizard lama (materials/budget/...) dan tanpa status → The Profiler.
		await goto(`${base}/profiler`, { replaceState: true });
	}

	async function checkPublication(): Promise<boolean> {
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/taut/campaigns/${campaignId}/gatekeeper/audits`);
			if (!res.ok) return false;
			const data = await res.json();
			const media: Array<{ publication_id?: string | null; publish?: { status?: string | null } }> = data?.media ?? [];
			return media.some((m) => Boolean(m.publication_id) || Boolean(m.publish?.status));
		} catch {
			return false;
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

<div class="flex items-center gap-2 py-10 text-sm text-muted-foreground">
	<LoaderCircle size={18} class="animate-spin" />
	{#if error}
		<span style="color: var(--destructive);">{error}</span>
		<a class="app-btn app-btn-soft ml-2" href={`/klien/${getBrandUrlId() || brandId}/taut-pintar`}>Kembali ke daftar</a>
	{:else}
		<span>Mengarahkan ke langkah berikutnya…</span>
	{/if}
</div>
