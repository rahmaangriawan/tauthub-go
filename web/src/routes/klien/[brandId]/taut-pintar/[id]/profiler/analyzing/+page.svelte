<script lang="ts">
	// Halaman proses analisis Profiler — pemilik tunggal POST /profile/analyze.
	// Port profiler/analyzing/client.tsx.
	//   - Datang dari form: status submission masih draft → jalankan analyze.
	//   - Reload/deep-link: status analyzing → polling; sudah ada hasil → summary;
	//     failed/tanpa submission → mulai ulang.
	//   - Sukses → progress 100% sesaat → redirect /summary.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { ArrowLeft, Check } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import AgentSteps from '../../../_agent-steps.svelte';

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);
	const formUrl = $derived(`/klien/${brandUrl}/taut-pintar/${campaignId}/profiler`);
	const summaryUrl = $derived(`/klien/${brandUrl}/taut-pintar/${campaignId}/profiler/summary`);

	const ANALYSIS_STEPS = [
		'Sedang memproses data profiler Anda…',
		'Membaca profil bisnis…',
		'Memetakan objektif kampanye…',
		'Menganalisis target audiens…',
		'Menyelaraskan guardrails…',
		'Menganalisis angle konten…',
		'Memproses bahan referensi…',
		'Menyusun strategi media…',
		'Menyiapkan Executive PR Brief…',
		'Executive PR Brief selesai!'
	];
	const POLL_INTERVAL_MS = 4000;
	/** Submission masih analyzing setelah durasi ini → anggap stuck, balik ke form. */
	const PROBE_TIMEOUT_MS = 10 * 60 * 1000;

	let step = $state(0);
	let progress = $state(3);
	let error = $state('');
	let stalled = $state(false);

	let stepTimer: number | null = null;
	let progressTimer: number | null = null;
	let pollTimer: number | null = null;
	let stepRef = 0;
	let started = false;
	let mounted = true;

	function clearTimers() {
		if (stepTimer) window.clearInterval(stepTimer);
		if (progressTimer) window.clearInterval(progressTimer);
		if (pollTimer) window.clearInterval(pollTimer);
		stepTimer = null;
		progressTimer = null;
		pollTimer = null;
	}

	function startTicker() {
		stepTimer = window.setInterval(() => {
			stepRef = Math.min(stepRef + 1, ANALYSIS_STEPS.length - 1);
			step = stepRef;
		}, 1400);
		progressTimer = window.setInterval(() => {
			if (progress >= 98) return;
			const target = Math.min(98, Math.round(((stepRef + 1) / ANALYSIS_STEPS.length) * 100));
			if (progress < target) progress += 1;
		}, 150);
	}

	onMount(() => {
		mounted = true;
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			try {
				const res = await apiRequest(`${apiBase}/profile/draft`);
				if (res.ok) {
					const data = await res.json();
					if (data.result) {
						await goto(summaryUrl, { replaceState: true });
						return;
					}
					if (data.draft?.status === 'analyzing') {
						pollUntilOutcome();
						return;
					}
				}
			} catch {
				// Gagal baca status → tetap jalankan (draft kosong/idempoten).
			}
			void runAnalyze();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			mounted = false;
			clearTimers();
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
		};
	});

	// Analisis baru: draft diambil dari DB (sudah di-flush form), lalu dikirim
	// utuh sebagai body POST /profile/analyze.
	async function runAnalyze() {
		if (started) return;
		started = true;
		startTicker();

		let payload: Record<string, unknown> | null = null;
		try {
			const draftRes = await apiRequest(`${apiBase}/profile/draft`);
			if (!draftRes.ok) throw new Error('Gagal memuat draft.');
			const draftData = await draftRes.json();
			payload = draftData.draft ?? null;
		} catch {
			error = 'Draft tidak ditemukan. Kembali ke form untuk mengulang.';
			clearTimers();
			return;
		}
		if (!payload) {
			error = 'Draft tidak ditemukan. Kembali ke form untuk mengulang.';
			clearTimers();
			return;
		}

		try {
			const res = await apiRequest(`${apiBase}/profile/analyze`, { method: 'POST', body: JSON.stringify(payload) }, 900_000);
			if (!res.ok) {
				const data = await res.json().catch(() => null);
				if (data?.code === 'insufficient_credits') {
					error = 'Kredit Saldo tidak mencukupi untuk menjalankan kampanye ini. Silakan Top Up Kredit Saldo terlebih dahulu.';
				} else {
					error = data?.detail ?? 'Analisis AI mengalami kendala sementara. Draft Anda tetap tersimpan.';
				}
				clearTimers();
				return;
			}
			clearTimers();
			stepRef = ANALYSIS_STEPS.length - 1;
			step = stepRef;
			progress = 100;
			await new Promise((r) => window.setTimeout(r, 900));
			if (mounted) await goto(summaryUrl, { replaceState: true });
		} catch {
			clearTimers();
			error = 'Analisis AI mengalami kendala sementara. Draft Anda tetap tersimpan.';
		}
	}

	// Reload di tengah analisis yang masih jalan: poll GET sampai muncul hasil,
	// atau timeout dianggap stuck.
	function pollUntilOutcome() {
		const startedAt = Date.now();
		pollTimer = window.setInterval(async () => {
			if (!mounted) return;
			try {
				const res = await apiRequest(`${apiBase}/profile/draft`);
				if (!res.ok) return;
				const data = await res.json();
				if (data.result) {
					clearTimers();
					await goto(summaryUrl, { replaceState: true });
					return;
				}
				if (Date.now() - startedAt > PROBE_TIMEOUT_MS) {
					clearTimers();
					stalled = true;
				}
			} catch {
				// Kesalahan polling sesaat diabaikan; interval berjalan terus.
			}
		}, POLL_INTERVAL_MS);
	}
</script>

<main class="mx-auto max-w-2xl">
	<AgentSteps current={1} />

	<div class="mt-6 flex flex-col items-center gap-4 text-center">
		{#if !error && !stalled}
			<div class="relative flex h-28 w-28 items-center justify-center">
				{#if progress < 100}
					<div class="absolute inset-0 animate-spin rounded-full border-2 border-transparent" style="border-top-color: var(--primary); border-right-color: var(--primary);"></div>
				{/if}
				{#if progress >= 100}
					<div class="flex h-20 w-20 items-center justify-center rounded-full" style="background-color: var(--success-soft); color: var(--success);">
						<Check size={26} strokeWidth={3} />
					</div>
				{:else}
					<span class="text-xl font-bold">{progress}%</span>
				{/if}
			</div>
		{/if}

		<div class="app-card w-full p-5">
			<p class="text-base font-bold">
				{error ? 'Analisis gagal' : stalled ? 'Koneksi terputus' : 'Profiler Sedang Bekerja'}
			</p>
			<p class="mt-1 text-sm text-muted-foreground">{error || stalled ? '' : ANALYSIS_STEPS[step]}</p>

			{#if error || stalled}
				<div class="mt-3 flex flex-col items-center gap-2">
					<p class="text-xs text-muted-foreground">
						{error || 'Analisis sudah berjalan lama. Periksa koneksi lalu coba lagi.'}
					</p>
					<a class="app-btn app-btn-soft" href={formUrl}>
						<ArrowLeft size={14} /> Kembali ke form
					</a>
				</div>
			{:else}
				<div class="mt-4 h-2 w-full overflow-hidden rounded-full" style="background-color: var(--muted);">
					<div class="h-full transition-[width] duration-200" style="width: {progress}%; background-color: var(--primary);"></div>
				</div>
			{/if}
		</div>

		{#if !error && !stalled}
			<p class="text-xs text-muted-foreground">Dokumen Executive PR Brief sedang disusun oleh AI secara otomatis.</p>
		{/if}
	</div>
</main>
