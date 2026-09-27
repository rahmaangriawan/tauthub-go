<script lang="ts">
	// Agent 1: The Profiler — form Executive PR Brief.
	// Port app/klien/[brandId]/taut-pintar/[id]/profiler/{page,client}.tsx.
	//
	// Redirect berbasis data: app lama me-redirect ke /profiler/summary bila sudah
	// ada hasil analisis (server redirect). Di SvelteKit redirect itu dilakukan di
	// onMount setelah fetch (lihat `load()`).
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Activity, LoaderCircle } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import AgentSteps from '../../_agent-steps.svelte';
	import ProfilerSections from './_sections.svelte';
	import { emptyDraft, draftFromResponse, validateProfilerDraft } from '../../_shared';
	import type { ProfilerDraft } from '../../_shared';

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);
	const creditsHref = $derived(`/klien/${brandUrl}/credits`);

	type ChannelOption = {
		id: string;
		name: string;
		slug: string;
		description: string;
		angles: Array<{ id: string; name: string; slug: string; description: string }>;
	};
	type DynamicQuestion = { key: string; label: string; suggested_answer: string };
	type ExtractResult = {
		business_name?: string;
		industry?: string;
		products_services?: string;
		positioning?: string;
		value_proposition?: string;
		tone_of_voice?: string;
		objective?: string;
		audience?: string[];
		guardrails?: string[];
		spokepersons?: Array<{ name?: string; role?: string; quote?: string }>;
		channels?: string[];
		backlink?: { enabled: boolean; target_url: string; anchor_text: string };
		recommended_angles?: Record<string, string>;
		dynamic_questions?: Record<string, DynamicQuestion[]>;
		dynamic_pr_answers?: Record<string, Record<string, string>>;
		budget_recommendation?: { mode: 'efficient' | 'optimal'; amount: number; reason: string };
	};

	let draft = $state<ProfilerDraft>(emptyDraft(''));
	let options = $state<ChannelOption[]>([]);
	let dynamicQuestions = $state<Record<string, DynamicQuestion[]>>({});
	let walletAvailable = $state(0);
	let loading = $state(true);
	let error = $state('');
	let crawlError = $state('');
	let crawling = $state(false);
	let crawlProgress = $state(0);
	let saveStatus = $state<'idle' | 'dirty' | 'saving' | 'saved' | 'error'>('idle');

	let saveTimer: number | null = null;
	let saveSeq = 0;
	// Draft yang dimuat sudah punya hasil crawl → auto-crawl dimatikan.
	let hasSavedCrawl = false;
	let autoCrawledFor: string | null = null;
	let hydrated = false;

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			if (saveTimer) window.clearTimeout(saveTimer);
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
		};
	});

	async function load() {
		try {
			const [draftRes, optionsRes] = await Promise.all([
				apiRequest(`${apiBase}/profile/draft`),
				apiRequest(`${apiBase}/profiler/options`)
			]);
			if (!draftRes.ok || !optionsRes.ok) throw new Error('Gagal memuat data Profiler.');
			const draftData = await draftRes.json();
			const optionsData = await optionsRes.json();
			// Sudah ada hasil analisis → form menampilkan summary; sekarang URL-nya
			// terpisah. Jalur edit ulang lewat tombol "Edit ulang" di summary.
			if (draftData.result) {
				await goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/profiler/summary`, { replaceState: true });
				return;
			}
			draft = draftFromResponse(draftData.draft, draftData.campaign?.name ?? '');
			hasSavedCrawl = Boolean(draftData.draft?.crawl_result);
			dynamicQuestions = draftData.draft?.crawl_result?.dynamic_questions ?? {};
			options = optionsData.channels ?? [];
			walletAvailable = draftData.wallet?.available ?? 0;
			saveStatus = 'idle';
			hydrated = true;
			loading = false;
			// Rentang anggaran dipaksa 5.000-100.000 saat load.
			const a = draft.budget?.amount ?? 0;
			if (a > 0 && (a < 5000 || a > 100000)) {
				patch({ budget: { ...draft.budget, amount: Math.min(100000, Math.max(5000, a)) } });
			}
		} catch (reason) {
			error = reason instanceof Error ? reason.message : 'Gagal memuat data.';
			loading = false;
		}
	}

	function patch(p: Partial<ProfilerDraft>) {
		const next: Partial<ProfilerDraft> = { ...p };
		// Sinkronkan narasumber: UI pakai spokepersons (list), validasi legacy
		// juga membaca spokesperson (tunggal); jaga keduanya konsisten.
		if (p.spokepersons) {
			const first = p.spokepersons[0];
			next.spokesperson = first
				? { name: first.name ?? '', role: first.role ?? '', quote: first.quote ?? '' }
				: { name: '', role: '', quote: '' };
		} else if (p.spokesperson) {
			next.spokepersons = [p.spokesperson];
		}
		draft = { ...draft, ...next };
		if (hydrated) saveStatus = 'dirty';
	}

	// Autosave debounced 800ms, stale-safe.
	$effect(() => {
		if (!hydrated || saveStatus !== 'dirty') return;
		const payload = draft;
		if (saveTimer) window.clearTimeout(saveTimer);
		const seq = ++saveSeq;
		saveTimer = window.setTimeout(() => {
			saveStatus = 'saving';
			void (async () => {
				try {
					const res = await apiRequest(`${apiBase}/profile/draft`, { method: 'PUT', body: JSON.stringify(payload) });
					if (!res.ok) throw new Error('Gagal menyimpan draft.');
					if (seq === saveSeq) saveStatus = 'saved';
				} catch {
					if (seq === saveSeq) saveStatus = 'error';
				}
			})();
		}, 800);
	});

	// Auto-crawl background: begitu draft tersimpan dan website terisi, tarik data
	// sekali per URL — saat user menekan "Analisis", fakta sudah siap. Hanya untuk
	// draft baru (draft dengan crawl_result sudah punya pilihan user).
	$effect(() => {
		const site = draft.business_profile?.website?.trim();
		if (!site || crawling || saveStatus !== 'saved') return;
		if (hasSavedCrawl) return;
		if (autoCrawledFor === site) return;
		autoCrawledFor = site;
		void crawl(site);
	});

	async function crawl(url: string) {
		crawling = true;
		crawlError = '';
		crawlProgress = 0;
		const timer = window.setInterval(() => (crawlProgress += 1), 1000);
		try {
			const res = await apiRequest(`${apiBase}/crawl`, { method: 'POST', body: JSON.stringify({ url }) });
			if (!res.ok) {
				crawlError = await readApiError(res, 'Gagal menarik data website.');
				return;
			}
			const data = await res.json();
			const ex: ExtractResult = data.extracted;
			const bp = {
				...draft.business_profile,
				business: ex.business_name || draft.business_profile.business,
				industry: ex.industry || draft.business_profile.industry,
				usp: ex.value_proposition || ex.positioning || draft.business_profile.usp,
				website: url
			};
			const backlinkActive = ex.backlink?.enabled ?? !draft.backlink?.enabled;
			const hasChannelSelection = (draft.channels ?? []).length > 0;
			const aiChannels = ex.channels?.length
				? ex.channels.map((c) => ({ channel: c as 'mainstream' | 'blogger', angle_slug: ex.recommended_angles?.[c] ?? '' }))
				: null;
			// Angle: hanya untuk channel yang belum punya pilihan angle dari user.
			const aiAnglePatch: Record<string, string> = ex.channels?.length
				? Object.fromEntries(
						ex.channels
							.map((c) => {
								const recommended = ex.recommended_angles?.[c];
								const userPicked = (draft.channels ?? []).find((x) => x.channel === c)?.angle_slug?.trim();
								return userPicked ? null : ([c, recommended ?? ''] as const);
							})
							.filter((x): x is readonly [string, string] => x !== null)
					)
				: {};

			// Dynamic PR answers per angle — nilai user menang hanya bila tidak kosong.
			const existingDynamic = (draft.materials?.dynamic ?? {}) as Record<string, unknown>;
			const mergedDynamic: Record<string, unknown> = { ...existingDynamic };
			for (const [angleSlug, answers] of Object.entries(ex.dynamic_pr_answers ?? {})) {
				const existingAnswers = (existingDynamic[angleSlug] ?? {}) as Record<string, string>;
				const merged: Record<string, string> = { ...answers };
				for (const [k, v] of Object.entries(existingAnswers)) {
					if (String(v ?? '').trim()) merged[k] = v;
				}
				mergedDynamic[angleSlug] = merged;
			}
			const dq = ex.dynamic_questions ?? {};
			for (const [angleSlug, questions] of Object.entries(dq)) {
				const answers: Record<string, string> = { ...((mergedDynamic[angleSlug] ?? {}) as Record<string, string>) };
				const staticAnswers = (ex.dynamic_pr_answers?.[angleSlug] ?? {}) as Record<string, string>;
				for (const q of questions) {
					if (String(answers[q.key] ?? '').trim()) continue;
					if (q.suggested_answer?.trim()) answers[q.key] = q.suggested_answer.trim();
					else if (staticAnswers[q.key]?.trim()) answers[q.key] = staticAnswers[q.key].trim();
				}
				mergedDynamic[angleSlug] = answers;
			}

			const budgetRec = ex.budget_recommendation;
			const budgetPatch =
				budgetRec && budgetRec.amount > 0 && (draft.budget?.amount ?? 0) <= 0
					? { budget: { mode: budgetRec.mode, amount: budgetRec.amount } }
					: {};

			const aiSpokepersons =
				(draft.spokepersons ?? []).length === 0
					? (ex.spokepersons ?? [])
							.map((s) => ({ name: (s.name ?? '').trim(), role: (s.role ?? '').trim(), quote: (s.quote ?? '').trim() }))
							.filter((s) => s.name)
					: [];

			patch({
				business_profile: bp,
				...(ex.objective ? { objective: ex.objective } : {}),
				...(ex.audience?.length ? { audience: { selected: ex.audience, custom: draft.audience?.custom ?? '' } } : {}),
				...(ex.guardrails?.length ? { guardrails: { rules: ex.guardrails } } : {}),
				...(aiSpokepersons.length ? { spokepersons: aiSpokepersons } : {}),
				...(aiChannels
					? { channels: aiChannels.map((c) => ({ channel: c.channel, angle_slug: aiAnglePatch[c.channel] ?? c.angle_slug })) }
					: hasChannelSelection
						? { channels: (draft.channels ?? []).map((c) => ({ ...c, angle_slug: aiAnglePatch[c.channel] ?? c.angle_slug })) }
						: { channels: [{ channel: 'mainstream' as const, angle_slug: '' }] }),
				...(Object.keys(mergedDynamic).length ? { materials: { ...draft.materials, mode: 'dynamic' as const, dynamic: mergedDynamic } } : {}),
				...budgetPatch,
				backlink: {
					...draft.backlink,
					enabled: backlinkActive,
					target_url: ex.backlink?.target_url || url,
					anchor_text: ex.backlink?.anchor_text || draft.backlink.anchor_text || '',
					smart_variations: draft.backlink.smart_variations ?? true
				}
			});
			if (Object.keys(dq).length) dynamicQuestions = dq;
		} catch {
			crawlError = 'Website belum dapat dibaca otomatis. Anda tetap dapat melanjutkan dengan mengisi informasi secara manual.';
		} finally {
			window.clearInterval(timer);
			crawling = false;
			crawlProgress = 0;
		}
	}

	const saveLabel = $derived(
		saveStatus === 'saving' ? 'Menyimpan...' : saveStatus === 'saved' ? 'Tersimpan' : saveStatus === 'error' ? 'Gagal' : 'Tersimpan otomatis'
	);

	// Validasi pakai schema yang sama dengan server; saldo dicek terpisah.
	const blockReason = $derived.by(() => {
		const v = validateProfilerDraft(draft);
		if (v) return v;
		const budget = draft.budget;
		if (budget?.mode === 'efficient' && budget.amount > walletAvailable)
			return 'Kredit Saldo tidak mencukupi. Top Up dulu sebelum menjalankan analisis.';
		return null;
	});

	// Flush autosave lalu pindah ke route proses (POST /profile/analyze).
	async function startAnalysis() {
		try {
			await apiRequest(`${apiBase}/profile/draft`, { method: 'PUT', body: JSON.stringify(draft) });
		} catch {
			// Draft tidak tersimpan bukan penghalang: analyze memakai body sendiri.
		}
		await goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/profiler/analyzing`, { replaceState: true });
	}
</script>

<main>
	{#if crawling || crawlProgress > 0}
		<div class="app-card mb-3 flex items-center justify-between gap-3 px-4 py-2.5" role="status" aria-live="polite">
			<span class="flex items-center gap-2 text-sm font-medium">
				<LoaderCircle size={16} class="animate-spin" /> Menarik &amp; mengekstrak fakta website… (rata-rata 30–45 dtk)
			</span>
			<span class="text-sm font-bold" style="color: var(--primary);">{crawlProgress} dtk</span>
		</div>
	{/if}

	<header class="flex flex-wrap items-center justify-between gap-3">
		<div class="flex items-center gap-3">
			<div class="flex h-10 w-10 items-center justify-center rounded-xl" style="background-color: var(--primary);">
				<Activity size={22} style="color: #fff;" />
			</div>
			<div>
				<h1 class="font-bold leading-tight">Agent 1: The Profiler</h1>
				<p class="text-sm leading-tight text-muted-foreground">
					Susun Executive PR Brief dari konteks bisnis, tujuan kampanye, target media, dan anggaran Anda.
				</p>
			</div>
		</div>
		<div class="flex flex-col items-end gap-1">
			<div class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium">
				{walletAvailable.toLocaleString('id-ID')} Kredit
			</div>
			<div class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
				<span class="h-2 w-2 rounded-full" style="background-color: var(--success);"></span>{saveLabel}
			</div>
		</div>
	</header>

	<div class="mt-3">
		<AgentSteps current={1} />
	</div>

	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}
	{#if crawlError}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--warning);">{crawlError}</div>{/if}

	{#if loading}
		<div class="mt-6 flex items-center justify-center gap-2 text-sm text-muted-foreground">
			<LoaderCircle size={18} class="animate-spin" /> Memuat The Profiler…
		</div>
	{:else}
		<div class="mt-4">
			<ProfilerSections
				bind:draft
				{apiBase}
				{creditsHref}
				{options}
				{walletAvailable}
				{crawling}
				{dynamicQuestions}
				onPatch={patch}
				onCrawl={(url) => void crawl(url)}
			/>
		</div>

		<div class="app-card mt-4 flex flex-wrap items-center justify-between gap-3 p-4">
			<p class="text-xs" style="color: {blockReason ? 'var(--warning)' : 'var(--muted-foreground)'};">
				{blockReason ?? 'Semua perubahan disimpan otomatis sebagai draft.'}
			</p>
			<button class="app-btn app-btn-primary" type="button" disabled={Boolean(blockReason)} onclick={startAnalysis}>
				Jalankan Analisis Profiler
			</button>
		</div>
	{/if}
</main>
