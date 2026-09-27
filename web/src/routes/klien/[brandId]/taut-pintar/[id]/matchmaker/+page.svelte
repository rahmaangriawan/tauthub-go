<script lang="ts">
	// Agent 2: The Matchmaker — kurasi media & alokasi anggaran.
	// Port app/klien/[brandId]/taut-pintar/[id]/matchmaker/client.tsx.
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Bot, ChevronDown, Clock, Coins, Hand, Search, SlidersHorizontal, Sparkles, Target, X, Zap } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import AgentSteps from '../../_agent-steps.svelte';
	import Select from '$lib/components/select.svelte';
	import { MANUAL_PROFILER_REASON, fmtIdr, rpCurrency, humanizeReason, fmtTrafficShort, formatTraffic } from '../../_shared';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);

	type PlacementRow = { media_id: string; name: string; score: number | null; status: string; reason: string | null };
	type MediaRow = {
		id: string;
		name: string;
		category: string | null;
		categories?: string[];
		media_type: string | null;
		description: string | null;
		website_url: string | null;
		integration_type: string | null;
		credit_cost: number | null;
		client_price: number | null;
		price: number | null;
		traffic_estimate: number | null;
		sw_visits?: number | null;
		media_tier: string | null;
		media_quality_score: number | null;
		effective_quality: number | null;
		health_score: number | null;
		dewan_pers_verified: boolean | null;
		domain_verified?: boolean | null;
		contoh_artikel_url?: string | null;
		seo_da: number | null;
		seo_pa: number | null;
		seo_spam_score: number | null;
		seo_ai_trust?: number | null;
		seo_ai_citation?: boolean | null;
		status: string;
		created_at: string | null;
	};
	type DraftInfo = {
		campaignName: string;
		budgetMode: string | null;
		budgetAmount: number;
		walletAvailable: number;
		rateIdr: number;
	};

	let info = $state<DraftInfo | null>(null);
	let placement = $state<PlacementRow[]>([]);
	let library = $state<MediaRow[]>([]);
	let selected = $state<Set<string>>(new Set());
	let manualPicks = $state<Set<string>>(new Set());
	let entryMode = $state<'ai' | 'manual' | null>(null);
	let tabChoice = $state<string | null>(null);
	let loading = $state(true);
	let running = $state(false);
	let saving = $state(false);
	let error = $state('');

	let q = $state('');
	let aiSectionOpen = $state(true);
	let typeFilter = $state('all');
	let sort = $state('recommended');
	let daMin = $state('');
	let spamMax = $state('');
	let priceMax = $state('');
	let showAdvanced = $state(false);
	let detail = $state<MediaRow | null>(null);

	// ?mode=manual dari popup Buat Kampanye.
	const manualMode = $derived(page.url.searchParams.get('mode') === 'manual');
	const manualFlow = $derived(entryMode ? entryMode === 'manual' : manualMode);
	const tab = $derived(tabChoice ?? (manualFlow ? 'custom' : 'ai'));
	const activePicks = $derived(tab === 'custom' ? manualPicks : selected);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			// Picks katalog sudah dikonsumsi oleh initializer manualPicks — hapus
			// agar kampanye manual berikutnya tidak mewarisi basi.
			try {
				sessionStorage.removeItem('taut_manual_picks');
			} catch {
				/* sessionStorage tidak tersedia */
			}
			setLoading(true);
			// Jalur manual ditentukan lebih dulu supaya Matchmaker AI tidak pernah
			// dijalankan — dan kredit tidak terpotong — untuk kampanye manual.
			const mode = manualMode ? 'manual' : await loadInfo();
			const manualPick = mode === 'manual';
			await Promise.all([loadPlacement(!manualPick, manualPick), loadLibrary()]);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	function setLoading(v: boolean) {
		loading = v;
	}

	async function loadInfo(): Promise<'ai' | 'manual' | null> {
		try {
			const res = await apiRequest(`${apiBase}/profile/draft`);
			if (res.ok) {
				const data = await res.json();
				info = {
					campaignName: data.campaign?.name ?? 'Kampanye',
					budgetMode: data.draft?.budget?.mode ?? null,
					budgetAmount: data.draft?.budget?.amount ?? 0,
					walletAvailable: data.wallet?.available ?? 0,
					rateIdr: data.wallet?.rate_idr ?? 0
				};
				if (data.entry_mode === 'ai' || data.entry_mode === 'manual') entryMode = data.entry_mode;
				return data.entry_mode ?? null;
			}
		} catch {
			/* non-blocking */
		}
		return null;
	}

	async function loadPlacement(autoRun: boolean, manualPick = false) {
		running = autoRun;
		try {
			const res = await apiRequest(`${apiBase}/placement`);
			let data: { media: PlacementRow[] } = { media: [] };
			if (res.ok) data = await res.json();
			if (autoRun && data.media.length === 0) {
				const runRes = await apiRequest(`${apiBase}/placement`, { method: 'POST' });
				if (runRes.ok) {
					const runData = await runRes.json();
					data.media = runData.media ?? [];
				} else {
					error = await readApiError(runRes, 'AI Matchmaker gagal dijalankan. Coba lagi.');
				}
			}
			placement = data.media;
			const serverSelected = new Set(data.media.filter((m) => m.status === 'selected').map((m) => m.media_id));
			if (manualPick) {
				// Kampanye manual: daftar pilihan dibangun di tab Manual; `selected`
				// tidak dipakai sebagai sumber centang — `manualPicks` yang dipakai.
				manualPicks = new Set([...manualPicks, ...serverSelected]);
				selected = new Set();
			} else {
				selected = serverSelected;
			}
		} catch (reason) {
			error = reason instanceof Error ? reason.message : 'Gagal memuat rekomendasi.';
		} finally {
			running = false;
		}
	}

	async function loadLibrary() {
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/media?limit=250`);
			if (res.ok) {
				const data = await res.json();
				library = data.media ?? data.items ?? [];
			}
		} catch {
			/* non-blocking */
		}
	}

	const mediaById = $derived.by(() => {
		const map = new Map<string, MediaRow>();
		for (const m of library) map.set(m.id, m);
		return map;
	});

	function toggleMedia(id: string) {
		const next = new Set(selected);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		selected = next;
	}

	function toggleManual(id: string) {
		const next = new Set(manualPicks);
		if (next.has(id)) next.delete(id);
		else next.add(id);
		manualPicks = next;
	}

	function placeholderMedia(p: PlacementRow): MediaRow {
		return {
			id: p.media_id, name: p.name, category: null, media_type: null,
			description: null, website_url: null, integration_type: 'manual',
			credit_cost: 0, client_price: 0, price: 0, traffic_estimate: 0,
			media_tier: null, media_quality_score: null, effective_quality: null,
			health_score: null, dewan_pers_verified: null,
			seo_da: null, seo_pa: null, seo_spam_score: null,
			status: 'active', created_at: null
		};
	}

	const recommendedCards = $derived.by(() => {
		let rows = placement.map((p) => ({ placement: p, media: mediaById.get(p.media_id) ?? placeholderMedia(p) }));
		if (q.trim()) {
			const needle = q.trim().toLowerCase();
			rows = rows.filter(
				(r) => r.media.name.toLowerCase().includes(needle) || (r.media.website_url ?? '').toLowerCase().includes(needle)
			);
		}
		if (typeFilter !== 'all') rows = rows.filter((r) => (r.media.media_type ?? '') === typeFilter);
		rows.sort((a, b) => {
			const selA = selected.has(a.placement.media_id) ? 0 : 1;
			const selB = selected.has(b.placement.media_id) ? 0 : 1;
			if (selA !== selB) return selA - selB;
			const remA = a.placement.status === 'removed' ? 1 : 0;
			const remB = b.placement.status === 'removed' ? 1 : 0;
			if (remA !== remB) return remA - remB;
			if (sort === 'price') return (a.media.credit_cost ?? 0) - (b.media.credit_cost ?? 0);
			if (sort === 'reach') return (b.media.traffic_estimate ?? 0) - (a.media.traffic_estimate ?? 0);
			return (b.placement.score ?? -1) - (a.placement.score ?? -1);
		});
		return rows;
	});

	const customCards = $derived.by(() => {
		const placementIds = new Set(placement.map((p) => p.media_id));
		// Hanya media lolos review admin (status "active") yang boleh dipilih.
		let rows = library.filter((m) => m.status === 'active');
		if (q.trim()) {
			const needle = q.trim().toLowerCase();
			rows = rows.filter(
				(m) =>
					m.name.toLowerCase().includes(needle) ||
					(m.category ?? '').toLowerCase().includes(needle) ||
					(m.website_url ?? '').toLowerCase().includes(needle)
			);
		}
		if (typeFilter !== 'all') rows = rows.filter((m) => (m.media_type ?? '') === typeFilter);
		if (daMin) rows = rows.filter((m) => (m.seo_da ?? 0) >= Number(daMin));
		if (spamMax) rows = rows.filter((m) => (m.seo_spam_score ?? 100) <= Number(spamMax));
		if (priceMax) rows = rows.filter((m) => (m.credit_cost ?? 0) <= Number(priceMax));
		rows.sort((a, b) => {
			const selA = manualPicks.has(a.id) ? 0 : 1;
			const selB = manualPicks.has(b.id) ? 0 : 1;
			if (selA !== selB) return selA - selB;
			if (sort === 'price') return (a.credit_cost ?? 0) - (b.credit_cost ?? 0);
			if (sort === 'reach') return (b.traffic_estimate ?? 0) - (a.traffic_estimate ?? 0);
			if (sort === 'recommended') return (placementIds.has(a.id) ? 0 : 1) - (placementIds.has(b.id) ? 0 : 1);
			return (b.health_score ?? 0) - (a.health_score ?? 0);
		});
		return rows;
	});

	const summary = $derived.by(() => {
		let reach = 0;
		let cost = 0;
		for (const id of activePicks) {
			const m = mediaById.get(id);
			if (m) {
				reach += m.traffic_estimate ?? 0;
				cost += m.credit_cost ?? 0;
			}
		}
		return { count: activePicks.size, reach, cost };
	});

	async function finalize() {
		const picks = tab === 'custom' ? manualPicks : selected;
		if (picks.size === 0) return;
		saving = true;
		error = '';
		try {
			// Tab Manual: media dari katalog belum punya baris di taut_campaign_media,
			// jadi harus di-ADD dulu. API membatasi 20 id per request → bertahap.
			if (tab === 'custom') {
				const ids = Array.from(picks);
				for (let i = 0; i < ids.length; i += 20) {
					const addRes = await apiRequest(`${apiBase}/placement`, {
						method: 'PUT',
						body: JSON.stringify({ add: ids.slice(i, i + 20) })
					});
					if (!addRes.ok) throw new Error(await readApiError(addRes, 'Gagal menambahkan media manual.'));
				}
			}
			const res = await apiRequest(`${apiBase}/placement`, {
				method: 'PUT',
				body: JSON.stringify({ selected: Array.from(picks) })
			});
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal menyimpan seleksi media.'));
			const advRes = await apiRequest(`${apiBase}/advance`, { method: 'POST', body: JSON.stringify({ target: 'placement' }) });
			if (!advRes.ok) throw new Error(await readApiError(advRes, 'Gagal mengunci placement.'));
			await goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/writer${manualFlow ? '?mode=manual' : ''}`);
		} catch (reason) {
			error = reason instanceof Error ? reason.message : 'Gagal melanjutkan.';
			saving = false;
		}
	}

	function getReleaseSpeed(integrationType: string | null) {
		const autoTypes = ['wordpress', 'blogger', 'tulisio_api', 'custom_web', 'webhook'];
		if (integrationType && autoTypes.includes(integrationType))
			return { label: 'Super Cepat', sub: '< 3 Jam', isSuper: true, tone: 'success' };
		if (integrationType === 'rss') return { label: 'Cepat', sub: '< 24 Jam', isSuper: false, tone: 'accent' };
		return { label: 'Standar', sub: '1 - 2 Hari', isSuper: false, tone: 'muted' };
	}
	function getDa(m: MediaRow): string | number {
		return m.seo_da != null && m.seo_da > 0
			? m.seo_da
			: m.media_quality_score
				? Math.max(1, Math.round(m.media_quality_score * 0.45))
				: '—';
	}
	function getAiTrust(m: MediaRow): string | number {
		if (m.seo_ai_trust != null && m.seo_ai_trust > 0) return m.seo_ai_trust;
		const quality = m.effective_quality ?? m.media_quality_score;
		return quality ? Math.max(1, Math.round(quality * 0.9)) : '—';
	}
	const cleanDomain = (m: MediaRow) =>
		m.website_url ? m.website_url.replace(/^https?:\/\/(www\.)?/, '').replace(/\/$/, '') : m.id.slice(0, 8);

	const badge = (tone: string) =>
		tone === 'success'
			? 'background-color: var(--success-soft); color: var(--success);'
			: tone === 'accent'
				? 'background-color: var(--accent); color: var(--accent-foreground);'
				: 'background-color: var(--muted); color: var(--muted-foreground);';
</script>

<PageTitle title="Taut Pintar — The Matchmaker" />

<main>
	<div class="flex flex-wrap items-center justify-between gap-3">
		<div class="flex items-center gap-3">
			<div class="flex h-10 w-10 items-center justify-center rounded-xl" style="background-color: var(--primary);">
				<Target size={22} style="color: #fff;" />
			</div>
			<div>
				<p class="font-bold leading-tight">Agent 2: The Matchmaker</p>
				<p class="text-sm leading-tight text-muted-foreground">Kurasi Media &amp; Alokasi Anggaran</p>
			</div>
		</div>
		<div class="inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-medium">
			<span>{fmtIdr(info?.walletAvailable ?? 0)} Kredit</span>
		</div>
	</div>

	<div class="mt-3">
		<AgentSteps current={2} lockedSteps={manualFlow ? [1] : undefined} lockedReason={MANUAL_PROFILER_REASON} />
	</div>

	{#if error}
		<div class="app-card mt-3 flex items-start gap-2 px-4 py-2.5 text-sm" style="color: var(--destructive);">
			<span class="flex-1">{error}</span>
			<button class="rounded p-0.5 hover:bg-secondary" type="button" aria-label="Tutup error" onclick={() => (error = '')}><X size={12} /></button>
		</div>
	{/if}

	<!-- Tabs -->
	<div class="mt-4 flex gap-1 border-b">
		<button
			class="inline-flex items-center gap-1.5 border-b-2 px-3 py-2 text-sm font-medium"
			style="border-color: {tab === 'ai' ? 'var(--primary)' : 'transparent'}; color: {tab === 'ai' ? 'var(--primary)' : 'inherit'}; {manualFlow ? 'opacity:.42; cursor:not-allowed;' : ''}"
			type="button"
			disabled={manualFlow}
			title={manualFlow ? 'Rekomendasi AI dinonaktifkan pada kampanye pemilihan manual' : undefined}
			onclick={() => { if (!manualFlow || tab === 'custom') tabChoice = 'ai'; }}
		>
			<Sparkles size={13} /> Rekomendasi AI Pintar
		</button>
		<button
			class="inline-flex items-center gap-1.5 border-b-2 px-3 py-2 text-sm font-medium"
			style="border-color: {tab === 'custom' ? 'var(--primary)' : 'transparent'}; color: {tab === 'custom' ? 'var(--primary)' : 'inherit'};"
			type="button"
			onclick={() => (tabChoice = 'custom')}
		>
			<Hand size={13} /> Manual
		</button>
	</div>

	<div class="app-card mt-3 flex flex-wrap items-center gap-4 p-3 text-sm">
		<span class="inline-flex items-center gap-1 text-muted-foreground">
			<Target size={14} /> Kampanye: <strong style="color: var(--foreground);">{info?.campaignName ?? '—'}</strong>
		</span>
		{#if tab === 'ai'}
			<span class="inline-flex items-center gap-1 text-muted-foreground">
				<Coins size={14} style="color: var(--success);" />
				Mode:
				<strong style="color: var(--foreground);">
					{info?.budgetMode === 'optimal'
						? 'Optimal (Strategis)'
						: info?.budgetMode === 'efficient' && (info?.budgetAmount ?? 0) > 0
							? `Efisien (Max ${fmtIdr(info.budgetAmount)} Kredit)`
							: 'Efisien'}
				</strong>
			</span>
		{/if}
	</div>

	{#if tab === 'ai'}
		<div class="app-card mt-3 flex flex-wrap items-center gap-2 p-3">
			<label class="app-input flex min-w-[200px] flex-1 items-center gap-2 !py-1.5">
				<Search size={14} class="text-muted-foreground" />
				<input class="w-full bg-transparent text-sm outline-none" placeholder="Cari domain media atau niche…" aria-label="Cari domain media atau niche" bind:value={q} />
			</label>
			<Select
				bind:value={typeFilter}
				ariaLabel="Filter tipe media"
				placeholder="Semua Tipe Media"
				minWidth={170}
				align="end"
				options={[
					{ value: 'all', label: 'Semua Tipe Media' },
					{ value: 'mainstream', label: 'Media Online' },
					{ value: 'blogger', label: 'Blog' }
				]}
			/>
			<Select
				bind:value={sort}
				ariaLabel="Urutkan"
				placeholder="Urutkan"
				minWidth={170}
				align="end"
				options={[
					{ value: 'recommended', label: 'AI Match Score' },
					{ value: 'match', label: 'Match % Tertinggi' },
					{ value: 'price', label: 'Harga Terendah' },
					{ value: 'reach', label: 'Est. Reach Tertinggi' }
				]}
			/>
		</div>

		{#if loading || running}
			<div class="app-card mt-3 flex flex-col items-center gap-2 px-4 py-10 text-center text-sm text-muted-foreground">
				<div class="h-6 w-6 animate-spin rounded-full border-2 border-current border-t-transparent"></div>
				<p>{running ? 'AI Matchmaker sedang mencocokkan media dengan hasil The Profiler…' : 'Memuat rekomendasi…'}</p>
			</div>
		{:else if recommendedCards.length === 0}
			<div class="app-card mt-3 px-4 py-10 text-center text-sm text-muted-foreground">
				Belum ada rekomendasi.
				<button class="ml-1 font-medium" style="color: var(--primary);" type="button" onclick={() => void loadPlacement(true)}>Jalankan ulang Matchmaker</button>
			</div>
		{:else}
			<div class="mt-3">
				<button class="app-card flex w-full items-center justify-between gap-2 p-3 text-sm" type="button" aria-expanded={aiSectionOpen} onclick={() => (aiSectionOpen = !aiSectionOpen)}>
					<span class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-semibold" style={badge('accent')}>
						<Sparkles size={13} /> Pilihan AI
					</span>
					<span class="inline-flex items-center gap-2 text-muted-foreground">
						{recommendedCards.filter((r) => r.placement.status !== 'removed').length} rekomendasi
						<ChevronDown size={16} style="transform: {aiSectionOpen ? 'rotate(180deg)' : 'none'}; transition: transform .15s;" />
					</span>
				</button>
				{#if aiSectionOpen}
					<div class="app-card mt-2 overflow-x-auto">
						<table class="w-full text-sm">
							<thead>
								<tr class="text-left text-[11px] uppercase tracking-wide text-muted-foreground">
									<th class="px-3 py-2 font-semibold">Pilih</th>
									<th class="px-3 py-2 font-semibold">Nama Media</th>
									<th class="px-3 py-2 font-semibold">Kategori</th>
									<th class="px-3 py-2 font-semibold">Trafik (Est)</th>
									<th class="px-3 py-2 text-center font-semibold">DA</th>
									<th class="px-3 py-2 text-center font-semibold">AI Trust</th>
									<th class="px-3 py-2 text-center font-semibold">AI Cited</th>
									<th class="px-3 py-2 font-semibold">Waktu Rilis</th>
									<th class="px-3 py-2 font-semibold">Harga</th>
									<th class="px-3 py-2 text-right font-semibold">Aksi</th>
								</tr>
							</thead>
							<tbody>
								{#each recommendedCards.filter((r) => r.placement.status !== 'removed') as { media: m, placement: p } (m.id)}
									{@const traffic = formatTraffic(m.sw_visits, m.traffic_estimate)}
									{@const speed = getReleaseSpeed(m.integration_type)}
									{@const isSelected = selected.has(m.id)}
									<tr class="border-t" style="background: {isSelected ? 'var(--accent)' : 'transparent'};" onclick={() => toggleMedia(m.id)}>
										<td class="px-3 py-2" onclick={(e) => e.stopPropagation()}>
											<input type="checkbox" checked={isSelected} aria-label={`Pilih media ${m.name}`} onchange={() => toggleMedia(m.id)} />
										</td>
										<td class="px-3 py-2">
											<div class="flex items-center gap-1.5">
												<span class="font-medium">{m.name}</span>
												{#if p.score != null && p.score > 0}
													<span class="rounded-full px-1.5 py-0.5 text-[10px] font-semibold" style={badge('accent')} title={humanizeReason(p.reason) || 'Rekomendasi AI'}>
														★ {Math.round(p.score)}% Match
													</span>
												{/if}
											</div>
											{#if m.website_url}
												<a class="text-xs text-muted-foreground" href={m.website_url.startsWith('http') ? m.website_url : `https://${m.website_url}`} target="_blank" rel="noreferrer noopener" onclick={(e) => e.stopPropagation()}>{cleanDomain(m)}</a>
											{:else}
												<span class="text-xs text-muted-foreground">{cleanDomain(m)}</span>
											{/if}
										</td>
										<td class="px-3 py-2">
											<span class="rounded-full px-2 py-0.5 text-[11px]" style={badge('muted')}>
												{Array.isArray(m.categories) && m.categories.length > 0 ? m.categories[0] : (m.category ?? 'Umum')}
											</span>
										</td>
										<td class="px-3 py-2">
											<p>{traffic.text}</p>
											<p class="text-xs text-muted-foreground">{traffic.source}</p>
										</td>
										<td class="px-3 py-2 text-center"><span class="rounded px-1.5 py-0.5" style={badge('muted')}>{getDa(m)}</span></td>
										<td class="px-3 py-2 text-center"><span class="rounded px-1.5 py-0.5" style={badge('muted')}>{getAiTrust(m)}</span></td>
										<td class="px-3 py-2 text-center">
											{#if m.seo_ai_citation === true}
												<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px]" style={badge('accent')}><Bot size={11} /> Sumber AI</span>
											{:else}—{/if}
										</td>
										<td class="px-3 py-2">
											<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px]" style={badge(speed.tone)}>
												{#if speed.isSuper}<Zap size={12} />{:else}<Clock size={12} />{/if}{speed.label}
											</span>
											<p class="text-xs text-muted-foreground">{speed.sub}</p>
										</td>
										<td class="px-3 py-2 font-semibold">Rp {fmtIdr(m.client_price ?? m.price ?? 0)}</td>
										<td class="px-3 py-2 text-right" onclick={(e) => e.stopPropagation()}>
											<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={() => (detail = m)}>Detail</button>
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</div>

			{#if !running && recommendedCards.some((r) => r.placement.status === 'removed')}
				<div class="mt-4">
					<p class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Tidak direkomendasikan AI</p>
					<div class="app-card mt-2 overflow-x-auto">
						<table class="w-full text-sm">
							<tbody>
								{#each recommendedCards.filter((r) => r.placement.status === 'removed') as { media: m, placement: p } (m.id)}
									<tr class="border-t" onclick={() => toggleMedia(m.id)}>
										<td class="px-3 py-2" onclick={(e) => e.stopPropagation()}>
											<input type="checkbox" checked={selected.has(m.id)} aria-label={`Pilih media ${m.name}`} onchange={() => toggleMedia(m.id)} />
										</td>
										<td class="px-3 py-2 font-medium">{m.name}</td>
										<td class="px-3 py-2 text-xs text-muted-foreground">{humanizeReason(p.reason) || 'Tidak direkomendasikan.'}</td>
										<td class="px-3 py-2 text-right" onclick={(e) => e.stopPropagation()}>
											<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={() => (detail = m)}>Detail</button>
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				</div>
			{/if}
		{/if}
	{:else}
		<div class="app-card mt-3 flex flex-wrap items-center gap-2 p-3">
			<label class="app-input flex min-w-[200px] flex-1 items-center gap-2 !py-1.5">
				<Search size={14} class="text-muted-foreground" />
				<input class="w-full bg-transparent text-sm outline-none" placeholder="Cari domain media atau niche…" aria-label="Cari domain media atau niche" bind:value={q} />
			</label>
			<Select
				bind:value={typeFilter}
				ariaLabel="Filter tipe media"
				placeholder="Semua Tipe Media"
				minWidth={170}
				align="end"
				options={[
					{ value: 'all', label: 'Semua Tipe Media' },
					{ value: 'mainstream', label: 'Media Online' },
					{ value: 'blogger', label: 'Blog' }
				]}
			/>
			<Select
				bind:value={sort}
				ariaLabel="Urutkan"
				placeholder="Urutkan"
				minWidth={170}
				align="end"
				options={[
					{ value: 'recommended', label: 'AI Match Score' },
					{ value: 'match', label: 'Match % Tertinggi' },
					{ value: 'price', label: 'Harga Terendah' },
					{ value: 'reach', label: 'Est. Reach Tertinggi' }
				]}
			/>
			<button class="app-btn app-btn-soft" type="button" onclick={() => (showAdvanced = !showAdvanced)}>
				<SlidersHorizontal size={14} /> Filter Lanjutan
			</button>
			{#if showAdvanced}
				<div class="w-full rounded-lg border p-3">
					<p class="text-xs font-bold">Filter Lanjutan</p>
					<div class="mt-2 grid gap-3 sm:grid-cols-3">
						<label class="block"><span class="text-xs text-muted-foreground">DA Score (Minimal)</span><input class="app-input mt-1 w-full" type="number" min="0" max="100" placeholder="0" bind:value={daMin} /></label>
						<label class="block"><span class="text-xs text-muted-foreground">Spam Score (Maksimal)</span><input class="app-input mt-1 w-full" type="number" min="0" max="100" placeholder="15" bind:value={spamMax} /></label>
						<label class="block"><span class="text-xs text-muted-foreground">Harga (Kredit, Maksimal)</span><input class="app-input mt-1 w-full" type="number" min="0" placeholder="Tanpa batas" bind:value={priceMax} /></label>
					</div>
					<button class="app-btn app-btn-soft mt-2 !min-h-7 !px-2 !text-xs" type="button" onclick={() => { daMin = ''; spamMax = ''; priceMax = ''; }}>Reset Filter</button>
				</div>
			{/if}
		</div>

		{#if customCards.length === 0}
			<div class="app-card mt-3 px-4 py-10 text-center text-sm text-muted-foreground">Tidak ada media yang cocok dengan filter.</div>
		{:else}
			<div class="app-card mt-3 overflow-x-auto">
				<table class="w-full text-sm">
					<thead>
						<tr class="text-left text-[11px] uppercase tracking-wide text-muted-foreground">
							<th class="px-3 py-2 font-semibold">Pilih</th>
							<th class="px-3 py-2 font-semibold">Nama Media</th>
							<th class="px-3 py-2 font-semibold">Kategori</th>
							<th class="px-3 py-2 font-semibold">Trafik (Est)</th>
							<th class="px-3 py-2 text-center font-semibold">DA</th>
							<th class="px-3 py-2 font-semibold">Harga</th>
							<th class="px-3 py-2 text-right font-semibold">Aksi</th>
						</tr>
					</thead>
					<tbody>
						{#each customCards as m (m.id)}
							{@const traffic = formatTraffic(m.sw_visits, m.traffic_estimate)}
							{@const isPicked = manualPicks.has(m.id)}
							<tr class="border-t" style="background: {isPicked ? 'var(--accent)' : 'transparent'};" onclick={() => toggleManual(m.id)}>
								<td class="px-3 py-2" onclick={(e) => e.stopPropagation()}>
									<input type="checkbox" checked={isPicked} aria-label={`Pilih media ${m.name}`} onchange={() => toggleManual(m.id)} />
								</td>
								<td class="px-3 py-2">
									<p class="font-medium">{m.name}</p>
									{#if m.website_url}<p class="text-xs text-muted-foreground">{cleanDomain(m)}</p>{/if}
								</td>
								<td class="px-3 py-2"><span class="rounded-full px-2 py-0.5 text-[11px]" style={badge('muted')}>{m.category ?? 'Umum'}</span></td>
								<td class="px-3 py-2"><p>{traffic.text}</p><p class="text-xs text-muted-foreground">{traffic.source}</p></td>
								<td class="px-3 py-2 text-center"><span class="rounded px-1.5 py-0.5" style={badge('muted')}>{getDa(m)}</span></td>
								<td class="px-3 py-2 font-semibold">Rp {fmtIdr(m.client_price ?? m.price ?? 0)}</td>
								<td class="px-3 py-2 text-right" onclick={(e) => e.stopPropagation()}>
									<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={() => (detail = m)}>Detail</button>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	{/if}

	<section class="app-card mt-4 p-4">
		<div class="flex flex-wrap items-end justify-between gap-4">
			<div class="flex flex-wrap gap-6">
				<div>
					<p class="text-xs text-muted-foreground">Media Terpilih</p>
					<p class="font-bold">{summary.count} Domain</p>
				</div>
				<div>
					<p class="text-xs text-muted-foreground">Est. Reach</p>
					<p class="font-bold" style="color: var(--primary);">{fmtTrafficShort(summary.reach)}+</p>
				</div>
				<div>
					<p class="text-xs text-muted-foreground">Total Biaya Kampanye</p>
					<p class="text-xl font-bold">{(info?.rateIdr ?? 0) > 0 ? rpCurrency(summary.cost * (info?.rateIdr ?? 0)) : '—'}</p>
					<p class="text-xs" style="color: var(--warning);">Jumlah Kredit: {fmtIdr(summary.cost)}</p>
				</div>
			</div>
			<div class="flex items-center gap-3">
				<a class="app-btn app-btn-soft" href={`/klien/${brandUrl}/taut-pintar/${campaignId}/profiler`}>Kembali</a>
				<button class="app-btn app-btn-primary" type="button" disabled={saving || summary.count === 0} onclick={finalize}>
					{#if saving}<div class="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent"></div>{/if}
					{saving ? 'Menyimpan…' : 'Kunci & Lanjutkan Penulisan →'}
				</button>
			</div>
		</div>
	</section>
</main>

{#if detail}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card my-6 w-full max-w-lg p-5" aria-label={`Detail media ${detail.name}`}>
			<div class="flex items-start justify-between gap-3">
				<div class="min-w-0">
					<h2 class="text-lg font-semibold">{detail.name}</h2>
					{#if detail.website_url}
						<a class="mt-0.5 inline-flex items-center gap-1 text-xs" style="color: var(--primary);" href={detail.website_url} target="_blank" rel="noreferrer">
							{detail.website_url}
						</a>
					{/if}
				</div>
				<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label="Tutup detail media" onclick={() => (detail = null)}><X size={18} /></button>
			</div>
			{#if detail.description}<p class="mt-3 text-sm text-muted-foreground">{detail.description}</p>{/if}
			<dl class="mt-4 grid grid-cols-2 gap-3 text-sm">
				<div><dt class="text-xs text-muted-foreground">Harga klien</dt><dd class="font-semibold">Rp {fmtIdr(detail.client_price ?? detail.price ?? 0)}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Biaya kredit</dt><dd class="font-semibold">{detail.credit_cost ?? 0} TC</dd></div>
				<div><dt class="text-xs text-muted-foreground">DA / PA</dt><dd>{getDa(detail)} / {detail.seo_pa ?? '—'}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Jangkauan</dt><dd>{fmtIdr(detail.traffic_estimate ?? 0)}</dd></div>
			</dl>
			<div class="mt-4 flex justify-end">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (detail = null)}>Tutup</button>
			</div>
		</dialog>
	</div>
{/if}
