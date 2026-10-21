<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Kampanye Kilat — jalur ringkas untuk klien yang tidak mau mengisi form
	// panjang. Dua isian saja (URL website + anggaran Rupiah), lalu The
	// Matchmaker memilih medianya dan klien langsung menuju The Writer. Jalur
	// The Profiler sudah tidak ditawarkan dari sini (hanya untuk kampanye lama).
	import { onDestroy, onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { CircleCheck, LoaderCircle, Sparkles, TriangleAlert, Wallet, X, Zap } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import TautBottomBar from '../../_bottom-bar.svelte';
	import { rpCurrency, isCampaignDone } from '../../_shared';
	import { tcToRp } from '$lib/credits/rupiah';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);
	// Status kampanye dari GET /profile/draft. Final = view only.
	let campaignStatus = $state<string | null>(null);
	const viewOnly = $derived(isCampaignDone(campaignStatus));

	type CrawlExtraction = {
		business_name?: string;
		industry?: string;
		products_services?: string;
		positioning?: string;
		value_proposition?: string;
		tone_of_voice?: string;
		keywords?: string[];
		facts?: string[];
		uncertain?: string[];
		objective?: string;
		audience?: string[];
		guardrails?: string[];
		spokepersons?: Array<{ name?: string; role?: string; quote?: string }>;
		channels?: string[];
		budget_recommendation?: { mode?: string; amount?: number; reason?: string };
	};
	type PlacementRow = {
		media_id: string;
		name: string;
		score: number | null;
		status: string;
		reason: string | null;
		credit_cost: number | null;
	};

	const OBJECTIVE_LABEL: Record<string, string> = {
		brand_awareness: tr('Kesadaran Merek'),
		seo_dominance: tr('Dominasi SEO'),
		conversion: tr('Konversi Penjualan'),
		reputation_management: tr('Manajemen Reputasi')
	};

	let campaignName = $state('');
	let wallet = $state(0);
	let rateIdr = $state(0);
	let siteUrl = $state('');
	let budgetRp = $state('');
	let extracted = $state<CrawlExtraction | null>(null);
	let crawlNote = $state('');
	let media = $state<PlacementRow[]>([]);
	let picked = $state<Set<string>>(new Set());
	let busy = $state<'idle' | 'crawl' | 'place'>('idle');
	let savingSelection = $state(false);
	let loading = $state(true);
	let error = $state('');
	// Validasi: pesan baru muncul setelah kolom disentuh atau tombol ditekan,
	// supaya form tidak merah sejak halaman dibuka.
	let urlEl = $state<HTMLInputElement | null>(null);
	let budgetEl = $state<HTMLInputElement | null>(null);
	let touchedUrl = $state(false);
	let touchedBudget = $state(false);
	let attempted = $state(false);
	// Peringatan URL muncul begitu pengetikan berhenti sejenak (400 ms), jadi
	// pengguna dapat peringatan merah langsung tanpa kedip tiap ketukan.
	let urlSettled = $state(false);
	let urlSettleTimer: number | null = null;
	function onUrlInput() {
		urlSettled = false;
		if (urlSettleTimer) window.clearTimeout(urlSettleTimer);
		urlSettleTimer = window.setTimeout(() => (urlSettled = true), 400);
	}
	onDestroy(() => {
		if (urlSettleTimer) window.clearTimeout(urlSettleTimer);
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
		try {
			const [draftRes, placeRes, campaignRes] = await Promise.all([
				apiRequest(`${apiBase}/profile/draft`),
				apiRequest(`${apiBase}/placement`),
				apiRequest(`${apiBase}`)
			]);
			if (draftRes.ok) {
				const d = await draftRes.json();
				campaignStatus = d.campaign?.taut_status ?? null;
				campaignName = d.campaign?.name ?? '';
				wallet = d.wallet?.available ?? 0;
				rateIdr = Number(d.wallet?.rate_idr) || 1000;
			}
			if (placeRes.ok) {
				const d = await placeRes.json();
				media = d.media ?? [];
				picked = new Set(media.filter((m) => m.status === 'selected').map((m) => m.media_id));
			}
			// Anggaran dipulihkan dari kampanye supaya "Sisa Anggaran" tidak hilang
			// saat halaman dimuat ulang.
			if (campaignRes.ok) {
				const c = await campaignRes.json().catch(() => null);
				const nominal = Number((c?.campaign?.budget_config ?? c?.budget_config)?.nominal ?? 0);
				if (nominal > 0) budgetRp = fmtRibuan(String(Math.round(nominal)));
			}
			await trimToBudget();
		} catch {
			error = tr('Gagal memuat data kampanye.');
		}
	}

	/** Anggaran Rupiah → kredit internal (dipakai pemilihan media AI, bukan tampilan). */
	const toTc = (rp: number) => (rateIdr > 0 ? Math.ceil(rp / rateIdr) : 0);
	const budgetValue = $derived(Number(budgetRp.replace(/[^\d]/g, '')) || 0);
	const estimatedTc = $derived(toTc(budgetValue));

	/** Angka mentah → format ribuan Indonesia (2.500.000) sambil diketik. */
	function fmtRibuan(raw: string): string {
		const digits = raw.replace(/[^\d]/g, '').replace(/^0+(?=\d)/, '').slice(0, 15);
		return digits ? Number(digits).toLocaleString('id-ID') : '';
	}
	function onBudgetInput(event: Event) {
		budgetRp = fmtRibuan((event.currentTarget as HTMLInputElement).value);
	}
	/** Browser bisa menyisipkan nilai tanpa memicu `input` (autofill/restore
	 *  sesi/paste), jadi kolom angka dibersihkan ulang saat ditinggalkan. */
	function onBudgetBlur() {
		budgetRp = fmtRibuan(budgetRp);
		touchedBudget = true;
	}

	/** "namabrand.com" → "https://namabrand.com" (pola sama dengan The Profiler). */
	function normalizeUrl(raw: string): string {
		const trimmed = raw.trim();
		return /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`;
	}

	/** Host lokal/internal ditolak di klien agar pengguna dapat pesan langsung;
	 *  server tetap punya penjaga SSRF-nya sendiri (yang jadi batas sebenarnya). */
	function isPrivateHost(host: string): boolean {
		const h = host.toLowerCase();
		if (h === 'localhost' || h.endsWith('.localhost') || h.endsWith('.local') || h.endsWith('.internal')) return true;
		const m = h.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/);
		if (!m) return false;
		const [a, b] = [Number(m[1]), Number(m[2])];
		if (a === 127 || a === 10 || a === 0) return true;
		if (a === 169 && b === 254) return true;
		if (a === 192 && b === 168) return true;
		if (a === 172 && b >= 16 && b <= 31) return true;
		return false;
	}

	const urlError = $derived.by(() => {
		const raw = siteUrl.trim();
		if (!raw) return tr('URL website wajib diisi.');
		if (raw.length > 500) return tr('URL terlalu panjang (maksimal 500 karakter).');
		if (/[\s<>"'`]/.test(raw)) return tr('URL tidak boleh mengandung spasi, tanda kutip, atau tanda kurung siku.');
		// "javascript:..." / "ftp://..." ditolak sebagai skema, tapi "host:3000"
		// bukan skema — bagian setelah titik dua yang murni angka dianggap port.
		const scheme = raw.match(/^([a-z][a-z0-9+.-]*):(.*)$/i);
		if (scheme && !/^https?$/i.test(scheme[1]) && !/^\d+([/?#].*)?$/.test(scheme[2])) {
			return tr('Hanya URL http:// atau https:// yang diterima.');
		}
		let parsed: URL;
		try {
			parsed = new URL(normalizeUrl(raw));
		} catch {
			return tr('Format URL tidak dikenali. Contoh: https://namabrand.com');
		}
		if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
			return tr('Hanya URL http:// atau https:// yang diterima.');
		}
		if (isPrivateHost(parsed.hostname)) return tr('Alamat lokal atau jaringan internal tidak diizinkan.');
		if (!parsed.hostname.includes('.') || parsed.hostname.endsWith('.')) return tr('Alamat domain tidak valid.');
		return null;
	});

	const budgetError = $derived.by(() => {
		if (!budgetRp.trim()) return tr('Anggaran kampanye wajib diisi.');
		if (budgetValue <= 0) return tr('Anggaran harus lebih dari 0.');
		if (budgetValue > 1_000_000_000) return tr('Anggaran maksimal Rp 1.000.000.000.');
		return null;
	});

	const showUrlError = $derived(Boolean(urlError) && (urlSettled || touchedUrl || attempted));
	const showBudgetError = $derived(Boolean(budgetError) && (touchedBudget || attempted));
	const selectedRows = $derived(media.filter((m) => picked.has(m.media_id)));
	const totalTc = $derived(selectedRows.reduce((sum, m) => sum + (m.credit_cost ?? 0), 0));
	const remainingRp = $derived(budgetValue - totalTc * rateIdr);
	const alternatives = $derived(media.filter((m) => !picked.has(m.media_id)));
	const ready = $derived(media.length > 0);

	async function racik() {
		if (viewOnly) {
			await goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/ringkasan`);
			return;
		}
		if (busy !== 'idle') return;
		attempted = true;
		error = '';
		crawlNote = '';
		if (urlError || budgetError) {
			// Fokuskan kolom pertama yang bermasalah supaya pengguna langsung tahu.
			(urlError ? urlEl : budgetEl)?.focus();
			return;
		}
		// URL dinormalkan (skema https ditambahkan bila belum ada) sebelum dikirim.
		const url = normalizeUrl(siteUrl);
		busy = 'crawl';
		try {
			// 1. Baca website: 1 fetch + 1 LLM di server (timeout LLM 3 menit).
			const crawlRes = await apiRequest(`${apiBase}/crawl`, { method: 'POST', body: JSON.stringify({ url }) }, 240_000);
			const crawlData = await crawlRes.json().catch(() => null);
			if (!crawlRes.ok) throw new Error(crawlData?.detail ?? (await readApiError(crawlRes, tr('Gagal membaca website.'))));
			extracted = crawlData?.extracted ?? null;
			if (!extracted) throw new Error(tr('Website tidak bisa dibaca. Coba URL halaman utama, atau pakai wizard lengkap.'));
			if (extracted.uncertain?.length) crawlNote = extracted.uncertain.join(' · ');

			// 2. Tulis business_profile ke kampanye: inilah yang dibaca Matchmaker
			//    dan The Writer saat menyusun artikel (draft Profiler tidak dipakai di sini).
			const profileRes = await apiRequest(`${apiBase}/profile`, {
				method: 'PUT',
				body: JSON.stringify({
					business_profile: {
						business: extracted.business_name ?? '',
						industry: extracted.industry ?? '',
						audience: (extracted.audience ?? []).join(', '),
						usp: extracted.value_proposition || extracted.positioning || '',
						website: url
					},
					spokepersons: (extracted.spokepersons ?? []).filter((s) => s?.name?.trim())
				})
			});
			if (!profileRes.ok) throw new Error(await readApiError(profileRes, tr('Gagal menyimpan ringkasan strategi.')));

			// 3. Anggaran dalam Rupiah ditulis ke budget_config supaya Matchmaker
			//    mengutamakan media yang harganya masuk anggaran (screenScore).
			const budgetRes = await apiRequest(`${apiBase}`, {
				method: 'PUT',
				body: JSON.stringify({ budget_strategy: 'efisien', budget_config: { nominal: budgetValue } })
			});
			if (!budgetRes.ok) throw new Error(await readApiError(budgetRes, tr('Gagal menyimpan anggaran.')));

			// 4. Status dikunci ke placement, lalu Matchmaker memilih medianya.
			const advRes = await apiRequest(`${apiBase}/advance`, { method: 'POST', body: JSON.stringify({ target: 'placement' }) });
			if (!advRes.ok) throw new Error(await readApiError(advRes, tr('Gagal membuka tahap pemilihan media.')));

			busy = 'place';
			const placeRes = await apiRequest(`${apiBase}/placement`, { method: 'POST' }, 240_000);
			if (!placeRes.ok) {
				const d = await placeRes.json().catch(() => null);
				throw new Error(d?.detail ?? (await readApiError(placeRes, tr('AI Matchmaker gagal memilih media.'))));
			}
			// Respons POST hanya memuat id/nama/skor/alasan — tanpa harga. Ambil
			// ulang daftar lengkap, lalu centang otomatis yang muat anggaran.
			const listRes = await apiRequest(`${apiBase}/placement`);
			const listData = listRes.ok ? await listRes.json() : null;
			media = listData?.media ?? [];
			if (media.length === 0) {
				error = tr('Belum ada media yang cocok. Coba ubah anggaran atau buka wizard lengkap.');
				return;
			}
			picked = pickWithinBudget(media);
			try {
				await saveSelection(picked);
			} catch {
				/* pilihan tetap bisa disimpan lewat tombol centang */
			}
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal meracik kampanye kilat.');
		} finally {
			busy = 'idle';
		}
	}

	/** Centang otomatis media yang muat anggaran, diurutkan skor AI (maks 20). */
	function pickWithinBudget(rows: PlacementRow[]): Set<string> {
		// Utamakan pilihan AI (status 'selected'). Baris 'removed' hanya dipakai
		// sebagai cadangan saat AI tidak menandai satu pun, supaya sisa baris dari
		// percobaan sebelumnya tidak tampil seolah rekomendasi AI.
		const aiPicked = rows.filter((r) => r.status === 'selected');
		const ranked = (aiPicked.length > 0 ? aiPicked : rows).slice().sort((a, b) => (b.score ?? -1) - (a.score ?? -1));
		const out = new Set<string>();
		let spent = 0;
		for (const row of ranked) {
			if (out.size >= 20) break;
			const price = row.credit_cost ?? 0;
			if (estimatedTc > 0 && spent + price > estimatedTc) continue;
			out.add(row.media_id);
			spent += price;
		}
		// Jangan biarkan kampanye kosong kalau tak satu pun muat anggaran.
		if (out.size === 0 && ranked.length > 0) out.add(ranked[0].media_id);
		return out;
	}

	/** `selected` mengganti seluruh pilihan kampanye (batas server 20 id). */
	async function saveSelection(set: Set<string>) {
		const res = await apiRequest(`${apiBase}/placement`, { method: 'PUT', body: JSON.stringify({ selected: [...set] }) });
		if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan pilihan media.')));
	}

	/**
	 * Pilihan yang tersimpan bisa melebihi anggaran: Matchmaker menandai SEMUA
	 * kandidat yang masuk ranking sebagai 'selected', bukan daftar pendek. Rapikan
	 * hanya di dalam pilihan yang ada, jadi media yang sudah dilepas pengguna tidak
	 * ikut tercentang lagi.
	 */
	async function trimToBudget() {
		if (budgetValue <= 0 || picked.size === 0) return;
		const chosen = media.filter((m) => picked.has(m.media_id));
		const spent = chosen.reduce((sum, m) => sum + (m.credit_cost ?? 0), 0);
		if (spent <= estimatedTc) return;
		const next = pickWithinBudget(chosen);
		if (next.size === picked.size) return;
		picked = next;
		try {
			await saveSelection(picked);
		} catch {
			/* gagal simpan bukan hal fatal — pengguna masih bisa mengubah manual */
		}
	}

	let selectionTimer: number | null = null;
	function toggle(mediaId: string) {
		const next = new Set(picked);
		if (next.has(mediaId)) next.delete(mediaId);
		else next.add(mediaId);
		if (next.size > 20) {
			error = tr('Maksimal 20 media per kampanye.');
			return;
		}
		picked = next;
		if (selectionTimer) window.clearTimeout(selectionTimer);
		savingSelection = true;
		selectionTimer = window.setTimeout(() => {
			void (async () => {
				try {
					await saveSelection(next);
					error = '';
				} catch (reason) {
					error = reason instanceof Error ? reason.message : tr('Gagal menyimpan pilihan media.');
				} finally {
					savingSelection = false;
				}
			})();
		}, 600);
	}

	async function toWriter() {
		await goto(`/klien/${brandUrl}/taut-pintar/${campaignId}/writer`);
	}
</script>

<PageTitle title={tr('Taut Pintar — Kampanye Kilat')} />

<main>
	<div class="flex flex-wrap items-center justify-between gap-3">
		<div class="flex items-center gap-3">
			<div class="flex h-10 w-10 items-center justify-center rounded-xl" style="background-color: var(--primary);">
				<Zap size={22} style="color: #fff;" />
			</div>
			<div>
				<p class="font-bold leading-tight">{tr('Kampanye Kilat')}</p>
				<p class="text-sm leading-tight text-muted-foreground">{tr('Dua isian, AI yang menyusun sisanya')}</p>
			</div>
		</div>
		<div class="flex flex-wrap items-center justify-end gap-3">
			{#if campaignName}
				<span class="inline-flex max-w-[260px] items-center gap-1.5 truncate rounded-full border px-3.5 py-1.5 text-[12.5px] font-semibold" style="background-color: var(--muted);" title={campaignName}>{campaignName}</span>
			{/if}
			<span class="inline-flex items-center gap-1.5 rounded-full px-3.5 py-1.5 text-[13px] font-bold" style="background-color: var(--success-soft); color: var(--success);" title={tr('Saldo Anda')}><Wallet size={13} />{tcToRp(wallet, rateIdr)}</span>
		</div>
	</div>

	{#if error}
		<div class="app-card mt-3 flex items-start gap-2 px-4 py-2.5 text-sm" style="background-color: color-mix(in srgb, var(--destructive) 7%, var(--card)); color: var(--destructive);" role="alert">
			<TriangleAlert size={16} class="mt-0.5 shrink-0" />
			<span class="flex-1">{error}</span>
			<button class="rounded p-0.5 hover:bg-secondary" type="button" aria-label={tr('Tutup error')} onclick={() => (error = '')}><X size={12} /></button>
		</div>
	{/if}

	{#if loading}
		<div class="mt-6 flex items-center gap-2 text-sm text-muted-foreground"><LoaderCircle size={18} class="animate-spin" />{tr('Memuat Kampanye Kilat…')}</div>
	{:else}
		<section class="app-card mt-3 p-4" aria-label={tr('Isian kampanye kilat')}>
			<div class="grid gap-3 sm:grid-cols-2">
				<label class="block">
					<span class="text-xs font-medium">{tr('URL Website atau Produk')} <span style="color: var(--destructive);">*</span></span>
					<input
						class="app-input mt-1 w-full"
						inputmode="url"
						placeholder="https://namabrand.com"
						bind:this={urlEl}
						bind:value={siteUrl}
						oninput={onUrlInput}
						style={showUrlError ? 'border-color: var(--destructive);' : ''}
						autocomplete="off"
						spellcheck="false"
						maxlength="500"
						aria-invalid={showUrlError}
						aria-describedby={showUrlError ? 'kilat-url-error' : undefined}
						onblur={() => (touchedUrl = true)}
						disabled={busy !== 'idle'}
					/>
					{#if showUrlError}
						<span id="kilat-url-error" class="mt-1 block text-[11px]" style="color: var(--destructive);">{urlError}</span>
					{:else}
						<span class="mt-1 block text-[11px] text-muted-foreground">{tr('Halaman utama website brand, contoh https://namabrand.com')}</span>
					{/if}
				</label>
				<label class="block">
					<span class="text-xs font-medium">{tr('Anggaran Kampanye (Rupiah)')} <span style="color: var(--destructive);">*</span></span>
					<input
						class="app-input mt-1 w-full"
						inputmode="numeric"
						placeholder="2.500.000"
						bind:this={budgetEl}
						value={budgetRp}
						oninput={onBudgetInput}
						onchange={onBudgetBlur}
						onblur={onBudgetBlur}
						style={showBudgetError ? 'border-color: var(--destructive);' : ''}
						autocomplete="off"
						maxlength="19"
						aria-invalid={showBudgetError}
						aria-describedby={showBudgetError ? 'kilat-budget-error' : undefined}
						disabled={busy !== 'idle'}
					/>
					{#if showBudgetError}
						<span id="kilat-budget-error" class="mt-1 block text-[11px]" style="color: var(--destructive);">{budgetError}</span>
					{:else if budgetValue > 0}
						<span class="mt-1 block text-[11px] text-muted-foreground">{tr('Anggaran ini dipakai AI untuk memilih media yang muat.')}</span>
					{/if}
				</label>
			</div>
			{#if viewOnly}
				<div class="mb-1 flex flex-wrap items-center gap-2 rounded-xl border px-4 py-2.5 text-[13px]" style="border-color: var(--border); background: var(--muted);">
					<span>{tr('Kampanye ini sudah selesai — mode lihat saja.')}</span>
					<a class="font-semibold underline" href={`/klien/${brandUrl}/taut-pintar/${campaignId}/ringkasan`}>{tr('Buka Ringkasan')}</a>
				</div>
			{/if}
			<div class="mt-3 flex flex-wrap items-center gap-3">
				<button class="app-btn app-btn-primary" type="button" disabled={busy !== 'idle' || viewOnly} onclick={() => void racik()}>
					{#if busy !== 'idle'}<LoaderCircle size={16} class="animate-spin" />{:else}<Sparkles size={16} />{/if}
					{busy === 'crawl' ? tr('Membaca website…') : busy === 'place' ? tr('AI memilih media…') : ready ? tr('Racik Ulang Kampanye Kilat') : tr('Racik Kampanye Kilat')}
				</button>
			</div>
			<p class="mt-2 text-[11px] text-muted-foreground">
				{tr('Tautan backlink dan anchor text diatur di The Writer setelah media dipilih.')}
			</p>
		</section>

		{#if extracted || ready}
			<div class="mt-3 grid gap-3 lg:grid-cols-2">
				<section class="app-card p-4" aria-label={tr('Ringkasan strategi')}>
					<div class="flex items-center justify-between gap-2">
						<p class="qcTitle">{tr('Ringkasan Strategi AI')}</p>
						{#if extracted?.objective}<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{OBJECTIVE_LABEL[extracted.objective] ?? extracted.objective}</span>{/if}
					</div>
					{#if extracted}
						<div class="mt-2 space-y-2 text-sm">
							<div>
								<p class="text-[11px] text-muted-foreground">{tr('Brand')}</p>
								<p class="font-semibold">{extracted.business_name ?? '—'}</p>
							</div>
							{#if extracted.industry}
								<div>
									<p class="text-[11px] text-muted-foreground">{tr('Industri')}</p>
									<p>{extracted.industry}</p>
								</div>
							{/if}
							{#if extracted.value_proposition || extracted.positioning}
								<div>
									<p class="text-[11px] text-muted-foreground">{tr('Sudut Berita / Nilai Jual')}</p>
									<p>{extracted.value_proposition || extracted.positioning}</p>
								</div>
							{/if}
							{#if extracted.keywords?.length}
								<div>
									<p class="text-[11px] text-muted-foreground">{tr('Kata Kunci')}</p>
									<div class="mt-1 flex flex-wrap gap-1">
										{#each extracted.keywords.slice(0, 8) as kw (kw)}<span class="rounded-full px-2 py-0.5 text-[10px] font-medium" style="background-color: var(--muted);">{kw}</span>{/each}
									</div>
								</div>
							{/if}
							{#if extracted.audience?.length}
								<div>
									<p class="text-[11px] text-muted-foreground">{tr('Target Audiens')}</p>
									<p>{extracted.audience.join(', ')}</p>
								</div>
							{/if}
							{#if crawlNote}
								<p class="text-[11px]" style="color: var(--warning);">{tr('Perlu dikonfirmasi:')} {crawlNote}</p>
							{/if}
						</div>
					{:else}
						<p class="mt-2 text-sm text-muted-foreground">{tr('Ringkasan strategi muncul setelah website dibaca.')}</p>
					{/if}
				</section>

				<section class="app-card p-4" aria-label={tr('Rekomendasi media')}>
					<div class="flex items-center justify-between gap-2">
						<p class="qcTitle">{tr('Rekomendasi Media AI')}</p>
						<p class="text-[11px] text-muted-foreground">{savingSelection ? tr('Menyimpan pilihan…') : `${picked.size} ${tr('dipilih')}`}</p>
					</div>

					{#if media.length === 0}
						<p class="mt-2 text-sm text-muted-foreground">{tr('Belum ada rekomendasi. Jalankan Racik Kampanye Kilat dulu.')}</p>
					{:else}
						<div class="kilatList">
							{#each [...selectedRows, ...alternatives] as m (m.media_id)}
								<label class="kilatRow">
									<input type="checkbox" checked={picked.has(m.media_id)} onchange={() => toggle(m.media_id)} />
									<span class="min-w-0 flex-1">
										<span class="block truncate text-[13px] font-semibold">{m.name}</span>
										{#if m.reason}<span class="block truncate text-[11px] text-muted-foreground">{m.reason}</span>{/if}
									</span>
									<span class="shrink-0 text-right">
										<span class="block text-[12px] font-semibold">{tcToRp(m.credit_cost ?? 0, rateIdr)}</span>
									</span>
								</label>
							{/each}
						</div>
						<p class="mt-2 text-[11px] text-muted-foreground">{tr('AI sudah mencentang media teratas yang muat anggaranmu. Tambah atau kurangi sesuai kebutuhan; detail lengkap tiap media ada di The Matchmaker.')}</p>
					{/if}
				</section>
			</div>
		{/if}

		<TautBottomBar>
			<div class="flex flex-wrap items-start gap-5">
				<div class="flex flex-col gap-1">
					<p class="text-[11px] font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('Media Terpilih')}</p>
					<p class="text-[17px] font-bold leading-tight">{picked.size} {tr('Domain')}</p>
				</div>
				<span class="mt-3.5 self-start text-lg font-semibold" style="color: var(--border);">+</span>
				<div class="flex flex-col gap-1">
					<p class="text-[11px] font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('Estimasi Biaya Media')}</p>
					<p class="text-[17px] font-bold leading-tight">{tcToRp(totalTc, rateIdr)}</p>
				</div>
				{#if budgetValue > 0}
					<div class="flex flex-col gap-1">
						<p class="text-[11px] font-bold uppercase tracking-[0.04em] text-muted-foreground">{tr('Sisa Anggaran')}</p>
						<p class="text-[19px] font-extrabold leading-tight" style="color: {remainingRp < 0 ? 'var(--destructive)' : 'var(--success)'};">{rpCurrency(remainingRp)}</p>
					</div>
				{/if}
			</div>
			<div class="flex flex-wrap items-center gap-3">
				<button class="app-btn app-btn-soft" type="button" onclick={() => goto(`/klien/${brandUrl}/taut-pintar`)}>{tr('Kembali')}</button>
				<button class="app-btn" type="button" style="background-color: var(--success); color: #fff;" disabled={picked.size === 0} onclick={() => void toWriter()}>
					<CircleCheck size={16} /> {tr('Buat Draf Artikel')}
				</button>
			</div>
		</TautBottomBar>
	{/if}
</main>

<style>
	.qcTitle {
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--muted-foreground);
	}
	.kilatList {
		margin-top: 8px;
		max-height: 46vh;
		overflow-y: auto;
		border: 1px solid var(--border);
		border-radius: var(--radius);
	}
	.kilatRow {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 9px 11px;
		cursor: pointer;
	}
	.kilatRow + .kilatRow {
		border-top: 1px solid var(--border);
	}
	.kilatRow:hover {
		background-color: var(--muted);
	}
</style>
