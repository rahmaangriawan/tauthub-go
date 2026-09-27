<script lang="ts">
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

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);
	const formUrl = $derived(`/klien/${brandUrl}/taut-pintar/${campaignId}/profiler`);

	let brief = $state<any>(null);
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
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal memuat hasil analisis.'));
			const data = await res.json();
			if (!data.result?.executive_brief) {
				await goto(formUrl, { replaceState: true });
				return;
			}
			brief = data.result.executive_brief;
		} catch (reason) {
			error = reason instanceof Error ? reason.message : 'Gagal memuat hasil analisis.';
		}
	}

	async function advance() {
		if (busy) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`${apiBase}/advance`, { method: 'POST', body: JSON.stringify({ target: 'profiled' }) });
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal mengunci hasil Profiler.'));
			await goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/matchmaker`);
		} catch (reason) {
			error = reason instanceof Error ? reason.message : 'Gagal lanjut.';
			busy = false;
		}
	}

	async function backToForm() {
		if (busy) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`${apiBase}/profile/result`, { method: 'DELETE' });
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal menghapus hasil analisis.'));
			await goto(formUrl, { replaceState: true });
		} catch (reason) {
			error = reason instanceof Error ? reason.message : 'Gagal menghapus hasil.';
			busy = false;
		}
	}

	async function onVerify(index: number, action: 'answered' | 'ignored', response?: string) {
		const res = await apiRequest(`${apiBase}/profile/verify`, {
			method: 'POST',
			body: JSON.stringify({ index, action, response })
		});
		if (!res.ok) throw new Error(await readApiError(res, 'Gagal menyimpan tanggapan verifikasi.'));
		const data = await res.json();
		brief = data.brief;
	}
</script>

<main>
	<AgentSteps current={1} />
	{#if loading}
		<div class="mt-6 flex items-center justify-center gap-2 text-sm text-muted-foreground">
			<LoaderCircle size={18} class="animate-spin" /> Memuat Executive PR Brief…
		</div>
	{:else}
		{#if error}<p class="mt-4 text-center text-sm" style="color: var(--destructive);">{error}</p>{/if}
		{#if brief}
			<div class="mt-4">
				<BriefView {brief} advancing={busy} onAdvance={() => void advance()} onBack={() => void backToForm()} onVerify={onVerify} />
			</div>
		{/if}
	{/if}
</main>
