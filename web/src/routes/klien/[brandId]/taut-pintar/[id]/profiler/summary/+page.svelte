<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Ringkasan Executive PR Brief (hasil The Profiler).
	// Port profiler/summary/client.tsx. Hanya menampilkan hasil terbaru; kalau
	// tidak ada hasil (deep-link) → redirect ke form.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { LoaderCircle } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import AgentSteps from '../../../_agent-steps.svelte';
	import BriefView from '../_brief-view.svelte';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);
	const formUrl = $derived(`/klien/${brandUrl}/taut-pintar/${campaignId}/profiler`);

	let brief = $state<any>(null);
	// Status kampanye (dari GET /profile/draft) — final = view only.
	let campaignStatus = $state<string | null>(null);
	const viewOnly = $derived(['submitted', 'scheduled', 'distributed'].includes(campaignStatus ?? ''));
	let loading = $state(true);
	let error = $state('');
	let busy = $state(false);

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
		try {
			const res = await apiRequest(`${apiBase}/profile/draft`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat hasil analisis.')));
			const data = await res.json();
			if (!data.result?.executive_brief) {
				await goto(formUrl, { replaceState: true });
				return;
			}
			campaignStatus = data.campaign?.taut_status ?? null;
			brief = data.result.executive_brief;
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal memuat hasil analisis.');
		}
	}

	async function advance() {
		if (busy) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`${apiBase}/advance`, { method: 'POST', body: JSON.stringify({ target: 'profiled' }) });
			// 422 = penanda "profiled" tidak bisa/tidak perlu diset (kampanye lama
			// atau hasil belum berstatus completed). Brief sudah tampil di layar —
			// lanjutkan ke Matchmaker, jangan buntu di error.
			if (!res.ok && res.status !== 422) {
				throw new Error(await readApiError(res, tr('Gagal mengunci hasil Profiler.')));
			}
			await goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/matchmaker`);
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal lanjut.');
			busy = false;
		}
	}

	async function backToForm() {
		if (busy) return;
		if (viewOnly) {
			error = tr('Kampanye sudah selesai — hasil analisis tidak bisa diubah lagi.');
			return;
		}
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`${apiBase}/profile/result`, { method: 'DELETE' });
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menghapus hasil analisis.')));
			await goto(formUrl, { replaceState: true });
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal menghapus hasil.');
			busy = false;
		}
	}

	async function onVerify(index: number, action: 'answered' | 'ignored', response?: string) {
		const res = await apiRequest(`${apiBase}/profile/verify`, {
			method: 'POST',
			body: JSON.stringify({ index, action, response })
		});
		if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan tanggapan verifikasi.')));
		const data = await res.json();
		brief = data.brief;
	}
</script>

<PageTitle title="Taut Pintar — Executive PR Brief" />

<main>
	{#if viewOnly}
		<div class="mb-3 flex flex-wrap items-center gap-2 rounded-xl border px-4 py-2.5 text-[13px]" style="border-color: var(--border); background: var(--muted);">
			<span>{tr('Kampanye ini sudah selesai — mode lihat saja. Brief tidak bisa diubah lagi.')}</span>
			<a class="font-semibold underline" href={`/klien/${brandUrl}/taut-pintar/${campaignId}/ringkasan`}>{tr('Buka Ringkasan Distribusi')}</a>
		</div>
	{/if}
	<AgentSteps current={1} />
	{#if loading}
		<div class="mt-6 flex items-center justify-center gap-2 text-sm text-muted-foreground">
			<LoaderCircle size={18} class="animate-spin" />{tr("Memuat Executive PR Brief…")}</div>
	{:else}
		{#if error}<p class="mt-4 text-center text-sm" style="color: var(--destructive);">{error}</p>{/if}
		{#if brief}
			<div class="mt-4">
				<BriefView {brief} advancing={busy} onAdvance={() => void advance()} onBack={() => void backToForm()} onVerify={onVerify} />
			</div>
		{/if}
	{/if}
</main>
