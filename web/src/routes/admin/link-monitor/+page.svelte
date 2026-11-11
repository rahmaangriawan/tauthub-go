<script lang="ts">
	import { safeHref } from '$lib/safe-url';
	import { tr } from '$lib/i18n/phrases';

	// Pelacak 404 — menjaga garansi tayang 12 bulan: memeriksa apakah artikel yang
	// sudah terbit di situs media mitra masih bisa dibuka.
	// Port apps/web/src/app/admin/link-monitor/page.tsx.
	import { onMount } from 'svelte';
	import { RefreshCw, Search, Unlink, Info, ExternalLink, Check, X } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import Tooltip from '$lib/components/tooltip.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const PAGE_SIZE = 20;

	interface Summary {
		watched: number;
		live: number;
		gone: number;
		unreachable: number;
		no_url: number;
		domains_down: number;
		domains_unknown: number;
	}
	interface PublicationItem {
		id: string;
		document_title: string | null;
		brand_name: string | null;
		media_name: string | null;
		partner_name: string | null;
		published_url: string | null;
		published_at: string | null;
		state: string | null;
		http_status: number | null;
		consecutive_failures: number | null;
		first_detected_gone_at: string | null;
		last_checked_at: string | null;
		error_message: string | null;
	}
	interface DomainItem {
		media_id: string;
		name: string | null;
		website_url: string | null;
		partner_name: string | null;
		domain_health: string;
		domain_last_checked_at: string | null;
		domain_check_fail_count: number | null;
		domain_verified: boolean | number;
	}
	interface Page<T> {
		items: T[];
		total: number;
		limit: number;
		offset: number;
	}

	/** Status link artikel — merah hanya untuk hilang nyata (404/410). */
	function linkMeta(state: string | null): {
		label: string;
		tone: 'neutral' | 'success' | 'warning' | 'danger';
		title: string;
	} {
		switch (state) {
			case 'gone':
				return {
					label: 'Hilang (404)',
					tone: 'danger',
					title:
						'Halaman artikel sudah tidak ada di situs media (404/410). Tekan Ganti artikel untuk membuat pengganti di media lain.'
				};
			case 'unreachable':
				return {
					label: 'Tak terjangkau',
					tone: 'warning',
					title:
						'Situs media menolak akses atau sedang error. Belum tentu hilang, jadi tidak dihitung sebagai artikel hilang.'
				};
			case 'live':
				return { label: 'Tayang', tone: 'success', title: tr('Halaman artikel masih bisa dibuka di situs media.') };
			case 'no_url':
				return {
					label: 'Tanpa URL',
					tone: 'neutral',
					title: tr('Publikasi ini belum punya URL, jadi halamannya belum bisa diperiksa.')
				};
			default:
				return { label: tr('Belum dicek'), tone: 'neutral', title: tr('Halaman artikel belum pernah diperiksa.') };
		}
	}

	/** Status kesehatan domain mitra (media.domain_health). */
	function domainMeta(health: string): { label: string; tone: 'neutral' | 'success' | 'warning' | 'danger'; title: string } {
		switch (health) {
			case 'unreachable':
				return {
					label: 'Mati',
					tone: 'danger',
					title: 'Situs mitra tidak bisa dihubungi, sehingga artikel yang terbit di sana ikut tidak bisa dibuka.'
				};
			case 'token_missing':
				return {
					label: 'Token hilang',
					tone: 'warning',
					title: 'Situs mitra hidup, tetapi penanda verifikasi domain tidak ditemukan lagi di halamannya.'
				};
			case 'live':
				return { label: 'Hidup', tone: 'success', title: tr('Situs mitra bisa dihubungi dan penanda verifikasi domain ditemukan.') };
			default:
				return { label: tr('Belum dicek'), tone: 'neutral', title: 'Situs mitra ini belum pernah diperiksa.' };
		}
	}

	let summary = $state<Summary | null>(null);
	let pubs = $state<Page<PublicationItem>>({ items: [], total: 0, limit: 500, offset: 0 });
	let domains = $state<Page<DomainItem>>({ items: [], total: 0, limit: 500, offset: 0 });
	let loading = $state(true);
	let checking = $state(false);
	let busyId = $state<string | null>(null);
	let error = $state('');
	let notice = $state('');
	let stateFilter = $state('');
	let healthFilter = $state('');
	let search = $state('');
	let tab = $state('articles');
	let pubsPage = $state(1);
	let domainsPage = $state(1);

	// ── Klaim garansi (Link Guardian SLA) ──────────────────────────────────
	// Baris klaim dibuat dari sisi klien; di sini admin memprosesnya: cek
	// kondisi domain, hubungi pemilik domain, lalu kembalikan saldo bila tidak
	// bisa diselesaikan. Kelayakan sudah dihitung server, jadi UI hanya
	// menampilkan alasannya saat tombol pengembalian tidak boleh ditekan.
	type ClaimRow = {
		id: string; brand_name: string | null; media_name: string | null; published_url: string | null;
		published_at: string | null; status: string; detection_state: string | null;
		anchor_expected: string | null; domain_check_note: string | null; owner_contact_note: string | null;
		refund_idr: number | null; refund_tc: number | null;
		eligible: boolean; ineligible_reason: string | null;
		link_state: string | null; in_article_state: string | null;
	};
	let claims = $state<ClaimRow[]>([]);
	let openClaim = $state<string | null>(null);
	let highlightClaim = $state<string | null>(null);
	let claimBusy = $state<string | null>(null);
	let noteDomain = $state('');
	let noteOwner = $state('');
	let rejectReason = $state('');
	// Konfirmasi pengembalian saldo: panel di dalam halaman, bukan confirm().
	let refundPanelFor = $state<string | null>(null);

	const claimDone = (st: string) => st === 'refunded' || st === 'rejected' || st === 'replaced';
	const openClaimCount = $derived(claims.filter((c) => !claimDone(c.status)).length);

	const CLAIM_STATUS_LABEL: Record<string, string> = $derived({
		submitted: tr('Menunggu diperiksa'), checking: tr('Sedang diperiksa'),
		contacted: tr('Menghubungi pemilik domain'), refunded: tr('Dana dikembalikan'),
		rejected: tr('Ditolak'), replaced: tr('Artikel pengganti dibuat')
	});
	const claimTone = (s: string): 'success' | 'warning' | 'danger' | 'neutral' =>
		s === 'refunded' || s === 'replaced' ? 'success' : s === 'rejected' ? 'neutral' : 'warning';
	const fmtDay = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'medium' }) : '—');
	const fmtRp = (n: number | null) => `Rp ${(n ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0 })}`;	const detectionLabel = (c: ClaimRow) =>
		c.detection_state === 'link_removed' ? tr('Tautan dihapus')
		: c.detection_state === 'gone' ? tr('Hilang (404)')
		: c.detection_state === 'noindex' ? tr('Noindex')
		: c.detection_state === 'rel_changed' ? tr('Atribut tautan berubah')
		: c.link_state ?? '—';

	function toggleClaim(c: ClaimRow) {
		if (openClaim === c.id) {
			openClaim = null;
			refundPanelFor = null;
			return;
		}
		openClaim = c.id;
		refundPanelFor = null;
		noteDomain = c.domain_check_note ?? '';
		noteOwner = c.owner_contact_note ?? '';
		rejectReason = '';
	}

	async function saveClaimStep(c: ClaimRow) {
		claimBusy = c.id;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/platform/link-guardian/claims/${c.id}/step`, {
				method: 'POST',
				body: JSON.stringify({
					status: noteOwner.trim() ? 'contacted' : 'checking',
					domain_check_note: noteDomain,
					owner_contact_note: noteOwner
				})
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			notice = 'Progres klaim tersimpan.';
			await load();
		} finally {
			claimBusy = null;
		}
	}

	async function refundClaim(c: ClaimRow) {
		claimBusy = c.id;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/platform/link-guardian/claims/${c.id}/refund`, {
				method: 'POST',
				body: JSON.stringify({ resolution_note: noteDomain })
			});
			const data = await res.json().catch(() => null);
			if (!res.ok) {
				error = data?.detail ?? (await readApiError(res, 'Permintaan gagal.'));
				return;
			}
			notice = `Saldo dikembalikan ${fmtRp(data?.refund_idr ?? 0)} ke akun klien.`;
			refundPanelFor = null;
			await load();
		} finally {
			claimBusy = null;
		}
	}

	async function rejectClaim(c: ClaimRow) {
		if (rejectReason.trim().length < 5) {
			error = 'Alasan penolakan minimal 5 karakter.';
			return;
		}
		claimBusy = c.id;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/platform/link-guardian/claims/${c.id}/reject`, {
				method: 'POST',
				body: JSON.stringify({ reason: rejectReason })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			notice = 'Klaim ditolak dan alasannya tercatat.';
			rejectReason = '';
			await load();
		} finally {
			claimBusy = null;
		}
	}

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
			// Notifikasi klaim menautkan ?claim=<id>: buka tab Klaim dan sorot
			// barisnya supaya admin mendarat tepat di klaim yang dimaksud.
			const wanted = new URL(window.location.href).searchParams.get('claim');
			if (wanted) {
				tab = 'claims';
				highlightClaim = wanted;
				const target = claims.find((c) => c.id === wanted);
				if (target) toggleClaim(target);
			}
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		loading = true;
		error = '';
		const pubQuery = new URLSearchParams({ view: 'publications', limit: '500' });
		if (stateFilter) pubQuery.set('state', stateFilter);
		if (search.trim()) pubQuery.set('q', search.trim());
		const domQuery = new URLSearchParams({ view: 'domains', limit: '500' });
		if (healthFilter) domQuery.set('health', healthFilter);
		try {
			const [sumRes, pubsRes, domsRes, claimsRes] = await Promise.all([
				apiRequest('/v1/platform/link-monitor?view=summary'),
				apiRequest(`/v1/platform/link-monitor?${pubQuery}`),
				apiRequest(`/v1/platform/link-monitor?${domQuery}`),
				apiRequest('/v1/platform/link-guardian/claims')
			]);
			if (!sumRes.ok) {
				error = await readApiError(sumRes, 'Permintaan gagal.');
				return;
			}
			summary = await sumRes.json();
			pubs = pubsRes.ok ? await pubsRes.json() : { items: [], total: 0, limit: 500, offset: 0 };
			domains = domsRes.ok ? await domsRes.json() : { items: [], total: 0, limit: 500, offset: 0 };
			if (claimsRes.ok) {
				const data = (await claimsRes.json()) as { claims?: ClaimRow[] };
				claims = data.claims ?? [];
			}
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	async function checkNow() {
		checking = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest('/v1/platform/link-monitor/check', { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const result = await res.json();
			if (result.total === 0) {
				notice =
					'Tidak ada pengecekan baru yang diantrekan — semua artikel dan domain masih dalam masa berlaku cek 7 hari.';
			} else {
				notice = `Antrean pengecekan dibuat: ${result.queued_publications} artikel dan ${result.queued_domains} domain.${result.capped ? ' Dibuat maksimal 200 per kategori; sisanya menyusul pada siklus berikutnya.' : ''} Hasil muncul setelah worker selesai.`;
			}
			await load();
		} finally {
			checking = false;
		}
	}

	async function replaceArticle(publicationId: string) {
		busyId = publicationId;
		error = '';
		notice = '';
		try {
			const res = await apiRequest('/v1/platform/link-monitor/replace', {
				method: 'POST',
				body: JSON.stringify({ publicationId })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			notice = 'Artikel digantikan ke media alternatif. Publikasi pengganti sudah dibuat dan masuk antrean proses.';
			await load();
		} finally {
			busyId = null;
		}
	}

	// Empat angka ringkasan: dua tentang artikel, dua tentang domain.
	const articleStats = $derived([
		{ label: tr('Dipantau'), value: summary?.watched ?? 0, tone: 'neutral' as const },
		{ label: tr('Tayang'), value: summary?.live ?? 0, tone: 'ok' as const },
		{ label: tr('Hilang'), value: summary?.gone ?? 0, tone: 'danger' as const },
		{ label: tr('Tak terjangkau'), value: summary?.unreachable ?? 0, tone: 'warn' as const }
	]);
	const domainStats = $derived([
		{ label: tr('Domain mati'), value: summary?.domains_down ?? 0, tone: 'danger' as const },
		{ label: tr('Belum dicek'), value: summary?.domains_unknown ?? 0, tone: 'neutral' as const }
	]);

	const visiblePubs = $derived(pubs.items.slice((pubsPage - 1) * PAGE_SIZE, pubsPage * PAGE_SIZE));
	const visibleDomains = $derived(domains.items.slice((domainsPage - 1) * PAGE_SIZE, domainsPage * PAGE_SIZE));
	const websiteHref = (url: string | null) =>
		url ? (/^[a-z][a-z0-9+.-]*:\/\//i.test(url) ? url : `https://${url}`) : '';
	const hostOf = (url: string | null) => {
		if (!url) return '';
		try {
			return new URL(url).hostname.replace(/^www\./, '');
		} catch {
			return url;
		}
	};
	const hasFilter = $derived(Boolean(stateFilter || search.trim() || healthFilter));
</script>

<section>
	<PageHeader title="Link Guardian">
		{#if (summary?.gone ?? 0) > 0}
			<span class="alert-chip">
				<span class="dot" aria-hidden="true"></span>
				{summary?.gone} {tr('artikel hilang')}
			</span>
		{/if}
		<button class="app-btn app-btn-primary" onclick={checkNow} disabled={checking}>
			<RefreshCw size={15} strokeWidth={1.5} />
			{checking ? tr('Mengantrekan…') : tr('Cek sekarang')}
		</button>
	</PageHeader>

	<div class="mt-2">
		<Tooltip
			content={tr('Pengecekan ulang berjalan otomatis tiap 7 hari. Artikel dipantau selama masa garansi medianya: 12 bulan sejak terbit, atau tanpa batas bila medianya menjamin tautan permanen.')}
			side="bottom"
			align="start"
			maxWidth={340}
		>
			<button class="info-line" type="button">
				<Info size={13} strokeWidth={1.5} />
				<span>{tr('Cara kerja pemeriksaan')}</span>
			</button>
		</Tooltip>
	</div>

	<Feedback {loading} {error} {notice} />

	<!-- Angka ringkas: artikel di kiri, domain di kanan -->
	<div class="stats mt-5">
		<div class="stat-group">
			<span class="stat-heading">{tr('Artikel')}</span>
			<div class="stat-row">
				{#each articleStats as s (s.label)}
					<div class="stat">
						<span class="stat-label">{s.label}</span>
						<span class="stat-value" data-tone={s.value === 0 ? 'zero' : s.tone}>{s.value}</span>
					</div>
				{/each}
			</div>
		</div>
		<div class="stat-group">
			<span class="stat-heading">{tr('Domain mitra')}</span>
			<div class="stat-row">
				{#each domainStats as s (s.label)}
					<div class="stat">
						<span class="stat-label">{s.label}</span>
						<span class="stat-value" data-tone={s.value === 0 ? 'zero' : s.tone}>{s.value}</span>
					</div>
				{/each}
			</div>
		</div>
	</div>

	<div class="-mx-1 mt-6 flex gap-1 overflow-x-auto px-1 pb-1" role="tablist">
		{#each [
			['articles', tr('Artikel bermasalah'), pubs.items.length],
			['domains', tr('Domain mitra'), domains.items.length],
			['claims', tr('Klaim garansi'), openClaimCount]
		] as [key, label, count] (key)}
			<button
				class="tab"
				data-active={tab === key}
				role="tab"
				aria-selected={tab === key}
				type="button"
				onclick={() => (tab = key as typeof tab)}
			>
				{label}
				<span class="tab-count tabular-nums">{count}</span>
			</button>
		{/each}
	</div>

	<div class="pt-4">
		{#if tab === 'claims'}
			{#if claims.length === 0}
				<div class="empty">
					<p class="text-sm text-muted-foreground">{tr('Belum ada klaim masuk.')}</p>
				</div>
			{:else}
				<p class="mb-3 text-xs text-muted-foreground">
					{tr('Kelayakan sudah diperiksa server: masih dalam masa garansi media, media terverifikasi, dan kerusakan 404 atau tautan dicabut.')}
				</p>
				<div class="flex flex-col gap-2">
					{#each claims as c (c.id)}
						<div class="app-card p-4" data-highlight={highlightClaim === c.id}>
							<div class="flex flex-wrap items-start justify-between gap-3">
								<div class="min-w-0">
									<p class="truncate text-sm font-medium">
										{c.brand_name ?? tr('Brand')} <span class="text-muted-foreground">·</span> {c.media_name ?? tr('Media')}
									</p>
									{#if c.published_url}
										<a
											class="link-sm max-w-full break-all"
											href={safeHref(c.published_url)}
											target="_blank"
											rel="noreferrer">{hostOf(c.published_url)} <ExternalLink size={11} strokeWidth={1.5} /></a
										>
									{/if}
									<p class="meta-line">
										<span>{tr('Tayang')} {fmtDay(c.published_at)}</span>
										<span aria-hidden="true">·</span>
										<span>{detectionLabel(c)}</span>
										{#if c.anchor_expected}<span aria-hidden="true">·</span><span class="truncate">anchor {c.anchor_expected}</span>{/if}
										{#if c.refund_tc}<span aria-hidden="true">·</span><span>{fmtRp(c.refund_idr)}</span>{/if}
									</p>
								</div>
								<div class="flex flex-none items-center gap-2">
									<span class="status-pill" data-tone={claimTone(c.status)}>{CLAIM_STATUS_LABEL[c.status] ?? c.status}</span>
									<button class="btn-quiet" type="button" onclick={() => toggleClaim(c)}>
										{openClaim === c.id ? tr('Tutup') : tr('Proses')}
									</button>
								</div>
							</div>

							{#if !c.eligible && !claimDone(c.status)}
								<p class="mt-2 text-xs" style="color: var(--warning);">{tr('Tidak bisa diklaim:')} {c.ineligible_reason}</p>
							{/if}

							{#if openClaim === c.id}
								<div class="mt-3 grid gap-3 sm:grid-cols-2">
									<label class="block">
										<span class="field-label">{tr('Hasil cek kondisi domain')}</span>
										<textarea class="app-input w-full" rows="2" bind:value={noteDomain}></textarea>
									</label>
									<label class="block">
										<span class="field-label">{tr('Catatan menghubungi pemilik domain')}</span>
										<textarea class="app-input w-full" rows="2" bind:value={noteOwner}></textarea>
									</label>
								</div>
								{#if !claimDone(c.status)}
									<label class="mt-3 block">
										<span class="field-label">{tr('Alasan penolakan, wajib bila klaim ditolak')}</span>
										<input class="app-input w-full" bind:value={rejectReason} />
									</label>
									<div class="mt-3 flex flex-wrap justify-end gap-2">
										<button class="app-btn app-btn-soft" type="button" disabled={claimBusy === c.id} onclick={() => void saveClaimStep(c)}>
											{tr('Simpan langkah')}
										</button>
										<button class="btn-quiet" style="color: var(--destructive);" type="button" disabled={claimBusy === c.id} onclick={() => void rejectClaim(c)}>
											{tr('Tolak klaim')}
										</button>
										<button
											class="app-btn app-btn-primary"
											type="button"
											disabled={claimBusy === c.id || !c.eligible}
											title={c.eligible ? tr('Kembalikan saldo senilai pembelian artikel ini') : (c.ineligible_reason ?? '')}
											onclick={() => (refundPanelFor = refundPanelFor === c.id ? null : c.id)}
										>
											{tr('Kembalikan saldo')}
										</button>
									</div>

									<!-- Konfirmasi di dalam halaman; tidak bisa dibatalkan, jadi
									     nominal dan tujuan ditulis lengkap sebelum admin menekan. -->
									{#if refundPanelFor === c.id}
										<div class="refund-panel mt-3">
											<div class="min-w-0 flex-1">
												<p class="text-sm font-medium">{tr('Kembalikan saldo klien?')}</p>
												<p class="mt-1 text-xs text-muted-foreground">
													{c.brand_name ?? tr('Brand')} · {c.media_name ?? tr('Media')}
													{#if c.refund_idr}<span class="tabular-nums"> · {fmtRp(c.refund_idr)}</span>{/if}
												</p>
												<p class="mt-1 text-xs text-muted-foreground">
													{tr('Tindakan ini mengurangi pemasukan Tauthub dan tidak bisa dibatalkan.')}
												</p>
											</div>
											<div class="flex flex-none items-center gap-2">
												<button class="btn-quiet" type="button" onclick={() => (refundPanelFor = null)}>{tr('Batal')}</button>
												<button
													class="app-btn app-btn-primary"
													type="button"
													disabled={claimBusy === c.id}
													onclick={() => void refundClaim(c)}
												>
													{claimBusy === c.id ? tr('Memproses…') : tr('Ya, kembalikan')}
												</button>
											</div>
										</div>
									{/if}
								{/if}
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		{:else if tab === 'articles'}
			<form class="mb-4 flex flex-wrap items-center gap-2" onsubmit={(e) => e.preventDefault()}>
				<label class="search-box">
					<Search size={15} strokeWidth={1.5} class="shrink-0 text-muted-foreground" />
					<input
						class="w-full bg-transparent text-sm outline-none"
						name="q"
						aria-label={tr('Cari artikel')}
						placeholder={tr('Cari judul, media, mitra, atau URL')}
						bind:value={search}
					/>
				</label>
				<Select
					ariaLabel={tr('Filter status artikel')}
					minWidth={260}
					value={stateFilter}
					onchange={(v) => {
						stateFilter = v;
						notice = '';
						pubsPage = 1;
						void load();
					}}
					options={[
						{ value: '', label: tr('Bermasalah: hilang dan tak terjangkau') },
						{ value: 'gone', label: tr('Hilang (404)') },
						{ value: 'unreachable', label: tr('Tak terjangkau') },
						{ value: 'live', label: tr('Tayang') },
						{ value: 'no_url', label: tr('Tanpa URL') }
					]}
				/>
				{#if hasFilter}
					<button
						type="button"
						class="btn-quiet"
						onclick={() => {
							search = '';
							stateFilter = '';
							notice = '';
							pubsPage = 1;
							void load();
						}}>{tr('Hapus filter')}</button
					>
				{/if}
			</form>

			{#if loading}
				<p class="text-sm text-muted-foreground">{tr('Memuat data…')}</p>
			{:else if pubs.items.length === 0}
				<div class="empty">
					<Unlink size={18} strokeWidth={1.5} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">
						{stateFilter || search.trim()
							? tr('Tidak ada artikel yang cocok dengan filter ini.')
							: tr('Tidak ada artikel hilang atau tak terjangkau.')}
					</p>
				</div>
			{:else}
				<!-- Tabel desktop -->
				<div class="app-card hidden overflow-x-auto lg:block">
					<table class="data">
						<thead>
							<tr>
								<th class="c-art">{tr('Artikel')}</th>
								<th class="c-media">{tr('Media')}</th>
								<th class="c-url">URL</th>
								<th class="c-status">{tr('Status')}</th>
								<th class="c-fail">{tr('Gagal')}</th>
								<th class="c-checked">{tr('Terakhir dicek')}</th>
								<th class="c-act"></th>
							</tr>
						</thead>
						<tbody>
							{#each visiblePubs as item (item.id)}
								{@const meta = linkMeta(item.state)}
								<tr>
									<td class="c-art">
										<span class="block text-sm font-medium" title={item.document_title ?? ''}>{item.document_title ?? tr('Tanpa judul')}</span>
										<span class="block truncate text-xs text-muted-foreground">{item.brand_name ?? '—'}</span>
									</td>
									<td class="c-media">
										<span class="block truncate text-sm">{item.media_name ?? '—'}</span>
										<span class="block truncate text-xs text-muted-foreground">{item.partner_name ?? tr('Tanpa mitra')}</span>
									</td>
									<td class="c-url">
										{#if item.published_url}
											<a class="link-sm max-w-full" href={safeHref(item.published_url)} target="_blank" rel="noreferrer noopener" title={item.published_url}>
												<span class="truncate">{hostOf(item.published_url)}</span>
												<ExternalLink size={11} strokeWidth={1.5} />
											</a>
										{:else}
											<span class="text-xs text-muted-foreground">—</span>
										{/if}
										{#if item.http_status != null}
											<span class="block text-xs tabular-nums text-muted-foreground">HTTP {item.http_status}</span>
										{/if}
									</td>
									<td class="c-status">
										<span class="status-pill" data-tone={meta.tone} title={meta.title}>{meta.label}</span>
										{#if item.error_message}
											<span class="mt-1 block truncate text-xs text-muted-foreground" title={item.error_message}>{item.error_message}</span>
										{/if}
									</td>
									<td class="c-fail tabular-nums" data-dim={(item.consecutive_failures ?? 0) === 0}>{item.consecutive_failures ?? 0}x</td>
									<td class="c-checked">
										<span class="block text-xs leading-5">{fmtDateTime(item.last_checked_at)}</span>
										{#if item.first_detected_gone_at}
											<span class="mt-0.5 block text-xs text-muted-foreground">{tr('sejak')} {fmtDay(item.first_detected_gone_at)}</span>
										{/if}
									</td>
									<td class="c-act">
										<button
											class="btn-replace"
											disabled={busyId !== null || item.state !== 'gone'}
											title={item.state === 'gone'
												? tr('Buat publikasi pengganti ke media alternatif')
												: tr('Hanya artikel berstatus hilang (404) yang bisa diganti')}
											onclick={() => void replaceArticle(item.id)}
										>
											{busyId === item.id ? tr('Memproses…') : tr('Ganti')}
										</button>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				<!-- Kartu untuk layar kecil -->
				<div class="grid gap-3 lg:hidden">
					{#each visiblePubs as item (item.id)}
						{@const m = linkMeta(item.state)}
						<div class="app-card p-4">
							<div class="flex items-start justify-between gap-3">
								<p class="text-sm font-medium">{item.document_title ?? tr('Tanpa judul')}</p>
								<span class="status-pill shrink-0" data-tone={m.tone}>{m.label}</span>
							</div>
							<p class="mt-1 text-xs text-muted-foreground">
								{item.media_name ?? '—'} · {item.brand_name ?? '—'}
							</p>
							{#if item.published_url}
								<a class="link-sm mt-3 block break-all" href={safeHref(item.published_url)} target="_blank" rel="noreferrer noopener">{item.published_url}</a>
							{/if}
							<div class="meta-line mt-2">
								{#if item.http_status != null}<span class="tabular-nums">HTTP {item.http_status}</span><span aria-hidden="true">·</span>{/if}
								<span class="tabular-nums">{item.consecutive_failures ?? 0}x {tr('gagal')}</span>
								<span aria-hidden="true">·</span>
								<span>{fmtDateTime(item.last_checked_at)}</span>
							</div>
							<button
								class="btn-replace mt-3"
								disabled={busyId !== null || item.state !== 'gone'}
								onclick={() => void replaceArticle(item.id)}
							>
								{busyId === item.id ? tr('Memproses…') : tr('Ganti artikel')}
							</button>
						</div>
					{/each}
				</div>

				<Pagination
					total={pubs.items.length}
					limit={PAGE_SIZE}
					offset={(pubsPage - 1) * PAGE_SIZE}
					onChange={(next) => (pubsPage = next / PAGE_SIZE + 1)}
				/>
				{#if pubs.total > pubs.items.length}
					<p class="mt-2 text-xs text-muted-foreground">
						{tr('Menampilkan')} {pubs.items.length} {tr('dari total')} {pubs.total} {tr('artikel. Persempit dengan filter untuk melihat sisanya.')}
					</p>
				{/if}
			{/if}
		{:else}
			<form class="mb-4 flex flex-wrap items-center gap-2" onsubmit={(e) => e.preventDefault()}>
				<Select
					ariaLabel={tr('Filter kesehatan domain')}
					minWidth={260}
					value={healthFilter}
					onchange={(v) => {
						healthFilter = v;
						notice = '';
						domainsPage = 1;
						void load();
					}}
					options={[
						{ value: '', label: tr('Perlu perhatian: mati dan belum dicek') },
						{ value: 'unreachable', label: tr('Domain mati') },
						{ value: 'unknown', label: tr('Belum dicek') },
						{ value: 'token_missing', label: tr('Token verifikasi hilang') },
						{ value: 'live', label: tr('Hidup') }
					]}
				/>
				{#if healthFilter}
					<button
						type="button"
						class="btn-quiet"
						onclick={() => {
							healthFilter = '';
							notice = '';
							domainsPage = 1;
							void load();
						}}>{tr('Hapus filter')}</button
					>
				{/if}
			</form>

			{#if loading}
				<p class="text-sm text-muted-foreground">{tr('Memuat data…')}</p>
			{:else if domains.items.length === 0}
				<div class="empty">
					<Unlink size={18} strokeWidth={1.5} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">
						{healthFilter
							? tr('Tidak ada domain dengan status kesehatan yang dipilih.')
							: tr('Tidak ada domain mitra yang mati atau belum dicek.')}
					</p>
				</div>
			{:else}
				<div class="app-card hidden overflow-x-auto lg:block">
					<table class="data">
						<thead>
							<tr>
								<th class="c-media">{tr('Media')}</th>
								<th class="c-partner">{tr('Mitra')}</th>
								<th class="c-url">{tr('Website')}</th>
								<th class="c-status">{tr('Status')}</th>
								<th class="c-checked">{tr('Terakhir dicek')}</th>
								<th class="c-fail">{tr('Gagal')}</th>
							</tr>
						</thead>
						<tbody>
							{#each visibleDomains as item (item.media_id)}
								{@const meta = domainMeta(item.domain_health)}
								<tr>
									<td class="c-media">
										<span class="block truncate text-sm font-medium">{item.name ?? tr('Tanpa nama')}</span>
										<span class="block text-xs text-muted-foreground">
											{item.domain_verified === true || item.domain_verified === 1 ? tr('Terverifikasi') : tr('Belum terverifikasi')}
										</span>
									</td>
									<td class="c-partner">
										<span class="block truncate text-sm">{item.partner_name ?? '—'}</span>
									</td>
									<td class="c-url">
										{#if item.website_url}
											<a class="link-sm max-w-full" href={websiteHref(item.website_url)} target="_blank" rel="noreferrer noopener" title={item.website_url}>
												<span class="truncate">{hostOf(item.website_url)}</span>
												<ExternalLink size={11} strokeWidth={1.5} />
											</a>
										{:else}
											<span class="text-xs text-muted-foreground">{tr('Belum diisi')}</span>
										{/if}
									</td>
									<td class="c-status">
										<span class="status-pill" data-tone={meta.tone} title={meta.title}>{meta.label}</span>
									</td>
									<td class="c-checked"><span class="text-xs">{fmtDateTime(item.domain_last_checked_at)}</span></td>
									<td class="c-fail tabular-nums" data-dim={(item.domain_check_fail_count ?? 0) === 0}>{item.domain_check_fail_count ?? 0}x</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				<div class="grid gap-3 lg:hidden">
					{#each visibleDomains as item (item.media_id)}
						{@const m = domainMeta(item.domain_health)}
						<div class="app-card p-4">
							<div class="flex items-start justify-between gap-3">
								<p class="text-sm font-medium">{item.name ?? tr('Tanpa nama')}</p>
								<span class="status-pill shrink-0" data-tone={m.tone}>{m.label}</span>
							</div>
							<p class="mt-1 text-xs text-muted-foreground">{item.partner_name ?? '—'}</p>
							{#if item.website_url}
								<a class="link-sm mt-3 block break-all" href={websiteHref(item.website_url)} target="_blank" rel="noreferrer noopener">{item.website_url}</a>
							{/if}
							<div class="meta-line mt-2">
								<span class="tabular-nums">{item.domain_check_fail_count ?? 0}x {tr('gagal')}</span>
								<span aria-hidden="true">·</span>
								<span>{fmtDateTime(item.domain_last_checked_at)}</span>
							</div>
						</div>
					{/each}
				</div>

				<Pagination
					total={domains.items.length}
					limit={PAGE_SIZE}
					offset={(domainsPage - 1) * PAGE_SIZE}
					onChange={(next) => (domainsPage = next / PAGE_SIZE + 1)}
				/>
				{#if domains.total > domains.items.length}
					<p class="mt-2 text-xs text-muted-foreground">
						{tr('Menampilkan')} {domains.items.length} {tr('dari total')} {domains.total} {tr('domain. Persempit dengan filter untuk melihat sisanya.')}
					</p>
				{/if}
			{/if}
		{/if}
	</div>
</section>

<style>
	/* ── Chip peringatan di header ── */
	.alert-chip {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		border-radius: 6px;
		padding: 2px 8px;
		background-color: var(--destructive-soft, #fdecee);
		color: var(--destructive);
		font-size: 12px;
		font-weight: 500;
		white-space: nowrap;
	}
	.dot {
		display: inline-block;
		width: 6px;
		height: 6px;
		border-radius: 999px;
		background-color: currentColor;
	}

	/* ── Baris info ── */
	.info-line {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		color: var(--muted-foreground);
		font-size: 12px;
		line-height: 18px;
		transition: color 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.info-line:hover { color: var(--foreground); }
	.info-line:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; border-radius: 4px; }

	/* ── Angka ringkas ── */
	.stats {
		display: grid;
		gap: 16px;
	}
	@media (min-width: 900px) {
		.stats { grid-template-columns: minmax(0, 2fr) minmax(0, 1fr); }
	}
	.stat-group {
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background-color: var(--card);
		padding: 12px 16px 14px;
		min-width: 0;
	}
	.stat-heading {
		display: block;
		color: var(--muted-foreground);
		font-size: 11px;
		line-height: 16px;
	}
	.stat-row {
		display: flex;
		flex-wrap: wrap;
		gap: 8px 28px;
		margin-top: 6px;
	}
	.stat { display: flex; align-items: baseline; gap: 8px; min-width: 0; }
	.stat-label { color: var(--muted-foreground); font-size: 12px; white-space: nowrap; }
	.stat-value {
		color: var(--foreground);
		font-size: 20px;
		font-weight: 500;
		line-height: 24px;
		font-variant-numeric: tabular-nums;
	}
	.stat-value[data-tone='zero'] { color: var(--muted-foreground); }
	.stat-value[data-tone='ok'] { color: var(--success); }
	.stat-value[data-tone='danger'] { color: var(--destructive); }
	.stat-value[data-tone='warn'] { color: var(--warning); }

	/* ── Tab ── */
	.tab {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		flex: 0 0 auto;
		min-height: 32px;
		border-radius: 8px;
		padding: 0 12px;
		color: var(--muted-foreground);
		font-size: 13px;
		font-weight: 500;
		white-space: nowrap;
		transition: background-color 160ms cubic-bezier(0.22, 1, 0.36, 1), color 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	/* Tab aktif: biru aksen dengan teks putih — satu-satunya warna di baris ini */
	.tab:hover { background-color: var(--secondary); color: var(--foreground); }
	.tab[data-active='true'] {
		background-color: var(--primary);
		color: var(--primary-foreground);
		font-weight: 500;
	}
	.tab[data-active='true']:hover { background-color: var(--primary); color: var(--primary-foreground); }
	.tab:focus-visible { outline: 2px solid var(--ring); outline-offset: 2px; }
	.tab-count {
		border-radius: 4px;
		padding: 0 5px;
		background-color: color-mix(in srgb, var(--foreground) 7%, transparent);
		color: var(--muted-foreground);
		font-size: 11px;
		line-height: 16px;
	}
	.tab[data-active='true'] .tab-count {
		background-color: color-mix(in srgb, var(--primary-foreground) 22%, transparent);
		color: var(--primary-foreground);
	}

	/* ── Pencarian ── */
	.search-box {
		display: flex;
		flex: 1;
		align-items: center;
		gap: 8px;
		min-width: 220px;
		max-width: 380px;
		min-height: 34px;
		border: 1px solid var(--input);
		border-radius: var(--radius-control);
		padding: 0 12px;
		background-color: var(--card);
	}
	.search-box:focus-within {
		border-color: var(--ring);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--ring) 15%, transparent);
	}

	/* ── Tabel ── */
	.data {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
		table-layout: fixed;
	}
	.data thead th {
		padding: 10px 12px;
		border-bottom: 1px solid var(--border);
		color: var(--muted-foreground);
		font-size: 11px;
		font-weight: 500;
		text-align: left;
		white-space: nowrap;
	}
	.data tbody td {
		padding: 10px 12px;
		vertical-align: top;
		min-width: 0;
		overflow: hidden;
	}
	.data tbody tr + tr td { border-top: 1px solid color-mix(in srgb, var(--foreground) 6%, transparent); }
	.data tbody tr:hover td { background-color: color-mix(in srgb, var(--secondary) 55%, transparent); }

	.c-art { width: 29%; }
	.c-media { width: 17%; }
	.c-partner { width: 16%; }
	.c-url { width: 16%; }
	.c-status { width: 108px; }
	.c-fail { width: 60px; text-align: right !important; }
	.c-checked { width: 140px; }
	.c-act { width: 88px; text-align: right !important; }

	.data thead th.c-fail,
	.data thead th.c-act { text-align: right; }
	.data td[data-dim='true'] { color: var(--muted-foreground); }

	.meta-line {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 4px 6px;
		margin-top: 4px;
		color: var(--muted-foreground);
		font-size: 11px;
		line-height: 16px;
	}

	/* ── Chip status ── */
	.status-pill {
		display: inline-flex;
		align-items: center;
		border-radius: 5px;
		padding: 1px 7px;
		font-size: 11px;
		font-weight: 500;
		line-height: 18px;
		white-space: nowrap;
	}
	.status-pill[data-tone='success'] { background-color: var(--success-soft); color: var(--success); }
	.status-pill[data-tone='warning'] { background-color: var(--warning-soft); color: var(--warning); }
	.status-pill[data-tone='danger'] { background-color: var(--destructive-soft, #fdecee); color: var(--destructive); }
	.status-pill[data-tone='neutral'] { background-color: var(--secondary); color: var(--muted-foreground); }

	/* ── Tautan ── */
	.link-sm {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		color: var(--primary);
		font-size: 12px;
		line-height: 18px;
		text-decoration: none;
		overflow: hidden;
	}
	.link-sm:hover { text-decoration: underline; text-underline-offset: 2px; }

	/* ── Tombol kecil ── */
	.btn-quiet {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-height: 28px;
		border: 1px solid var(--border);
		border-radius: 7px;
		padding: 0 10px;
		background-color: var(--card);
		color: var(--foreground);
		font-size: 12px;
		font-weight: 500;
		white-space: nowrap;
		transition: background-color 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.btn-quiet:hover { background-color: var(--secondary); }
	.btn-quiet:disabled { opacity: 0.5; cursor: not-allowed; }

	.btn-replace {
		min-height: 28px;
		border: 1px solid var(--border);
		border-radius: 7px;
		padding: 0 10px;
		background-color: var(--card);
		color: var(--destructive);
		font-size: 12px;
		font-weight: 500;
		white-space: nowrap;
		transition: background-color 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.btn-replace:hover:not(:disabled) { background-color: var(--destructive-soft, #fdecee); }
	.btn-replace:disabled { color: var(--muted-foreground); opacity: 0.5; cursor: not-allowed; }

	/* ── Penanda & kosong ── */
	.app-card[data-highlight='true'] {
		border-color: var(--primary);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 12%, transparent);
	}
	.empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		border: 1px dashed var(--border);
		border-radius: var(--radius);
		padding: 40px 16px;
		text-align: center;
	}

	/* ── Panel konfirmasi refund ── */
	.refund-panel {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 12px;
		border: 1px solid color-mix(in srgb, var(--destructive) 24%, var(--border));
		border-left: 3px solid var(--destructive);
		border-radius: 8px;
		padding: 12px 14px;
		background-color: color-mix(in srgb, var(--destructive-soft, #fdecee) 45%, var(--card));
	}

	/* ── Field ── */
	.field-label {
		display: block;
		margin-bottom: 4px;
		color: var(--muted-foreground);
		font-size: 12px;
		font-weight: 500;
	}
</style>
