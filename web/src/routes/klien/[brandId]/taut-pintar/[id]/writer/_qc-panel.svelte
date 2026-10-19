<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Panel analisis QC untuk SATU artikel, dibuka dari The Writer. Isi dan
	// bahasanya mengikuti Agent 4 (The Gatekeeper), termasuk badge "Hard Gate"
	// hanya pada Illegal Content Guard & Link Health Validator — dua pemeriksa
	// itulah yang benar-benar menentukan canDistribute di server.
	import { onDestroy } from 'svelte';
	import { CheckCircle2, CircleCheck, Clock, Eye, Send, LoaderCircle, ShieldAlert, ShieldCheck, Sparkles, TriangleAlert, X, XCircle } from '@lucide/svelte';
	import { apiRequest, readApiError } from '$lib/auth';
	import SlideOver from '$lib/components/slide-over.svelte';

	type CheckStatus = 'pending' | 'running' | 'passed' | 'failed' | 'review_required';
	type Check = {
		status: CheckStatus;
		percent?: number;
		riskLevel?: string;
		categories?: string[];
		confidence?: number;
		reasons?: string[];
		unsafeUrls?: string[];
		links?: Array<{ url: string; status?: string; httpStatus?: number | null; category?: string | null; reason?: string }>;
		level?: string;
		notes?: string[];
		error?: string;
		// AI Overview (kesiapan artikel untuk Google AI Overviews).
		totalScore?: number;
		maxScore?: number;
		verdict?: 'lulus' | 'ditolak';
		indicators?: Array<{ key: string; title: string; score: number; summary: string }>;
		revisionNotes?: string[];
		unsourcedEntities?: string[];
		briefAvailable?: boolean;
		info?: string;
	};
	type Checks = { aiOverview: Check; contentSafety: Check; linkHealth: Check; brandVoice: Check };
	type MediaEntry = {
		media_id: string;
		name: string;
		article: { title: string; updated_at: string | null; links: { count: number; rels: string[] } };
		audit: { status: CheckStatus; checks: Checks };
	};
	type AuditsData = {
		media: MediaEntry[];
		canDistribute: boolean;
	};

	// AI Overview Checker dihapus dari panel (aturan pemilik produk) — pemindaian
	// backendo tetap menyimpan datanya, hanya layar tiga pemeriksa ini.
	const CHECK_CARDS: Array<{ key: keyof Checks; title: string; gate: boolean }> = [
		{ key: 'contentSafety', title: 'Illegal Content Guard', gate: true },
		{ key: 'linkHealth', title: 'Link Health Validator', gate: true },
		{ key: 'brandVoice', title: 'Brand Voice Limits', gate: false }
	];

	let {
		apiBase,
		mediaId,
		mediaName,
		articleStatus,
		words,
		regenLabel,
		canRegen,
		generating,
		gatekeeperConfigured,
		reloadToken,
		onClose,
		onEdit,
		onRegenerate,
		onScanned,
		onPublish
	}: {
		apiBase: string;
		mediaId: string;
		mediaName: string;
		articleStatus: string;
		words: number | null;
		regenLabel: string;
		canRegen: boolean;
		generating: boolean;
		gatekeeperConfigured: boolean | null;
		reloadToken: number;
		onClose: () => void;
		onEdit: () => void;
		onRegenerate: () => void;
		onScanned?: () => void;
		onPublish?: () => void;
	} = $props();

	let data = $state<AuditsData | null>(null);
	let loading = $state(true);
	let error = $state('');
	let notice = $state('');
	let scanning = $state(false);
	let scanSeconds = $state(0);
	let scanTimer: number | null = null;
	let openCheck = $state<string | null>(null);
	let pollTimer: number | null = null;

	const media = $derived(data?.media.find((m) => m.media_id === mediaId) ?? null);
	const checks = $derived<Checks | null>(media?.audit.checks ?? null);
	const totalMedia = $derived(data?.media.length ?? 0);
	const scannedCount = $derived((data?.media ?? []).filter((m) => m.audit.status !== 'pending').length);
	const blocking = $derived(
		(data?.media ?? []).filter(
			(m) => m.audit.status === 'pending' || m.audit.checks.contentSafety?.status !== 'passed' || m.audit.checks.linkHealth?.status !== 'passed'
		)
	);
	const articleLinks = $derived(media?.article.links ?? null);

	async function loadAudits() {
		try {
			const res = await apiRequest(`${apiBase}/gatekeeper/audits`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat hasil pemeriksaan QC.')));
			data = await res.json();
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal memuat hasil pemeriksaan QC.');
		} finally {
			loading = false;
		}
	}

	// reloadToken sengaja dibaca di sini supaya panel menyegarkan hasilnya
	// setelah artikel diedit atau di-generate ulang dari The Writer.
	$effect(() => {
		reloadToken;
		void loadAudits();
	});

	function startPolling() {
		if (pollTimer) return;
		// audit-all menahan koneksi sampai semua media selesai dan tidak pernah
		// menulis status "running", jadi hasil per media hanya bisa dilihat
		// dengan membaca ulang daftar audit selama proses berjalan.
		pollTimer = window.setInterval(() => void loadAudits(), 4000);
	}
	function stopPolling() {
		if (!pollTimer) return;
		window.clearInterval(pollTimer);
		pollTimer = null;
	}
	onDestroy(() => {
		stopPolling();
		if (scanTimer) window.clearInterval(scanTimer);
	});

	async function scan() {
		if (scanning) return;
		scanning = true;
		scanSeconds = 0;
		if (scanTimer) window.clearInterval(scanTimer);
		scanTimer = window.setInterval(() => (scanSeconds += 1), 1000);
		error = '';
		notice = '';
		startPolling();
		try {
			// Panel ini level-artikel: kirim mediaIds supaya server memindai
			// ULANG artikel ini apa pun statusnya (termasuk yang sudah 'ready'),
			// tanpa kena jalur idempoten audit-all. audit-all sinkron: request
			// menahan koneksi sampai selesai, jadi timeout-nya harus panjang.
			const res = await apiRequest(
				`${apiBase}/gatekeeper/audit-all`,
				{ method: 'POST', body: JSON.stringify({ mediaIds: [mediaId] }) },
				900_000
			);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menjalankan pemindaian QC.')));
			const result = await res.json().catch(() => null);
			if (result?.alreadyAudited) notice = tr('Semua artikel sudah punya hasil pemindaian — tidak ada yang dipindai ulang.');
			else if ((result?.mediaCount ?? 0) === 0) notice = tr('Tidak ada artikel yang dipindai.');
		} catch (reason) {
			if (reason instanceof DOMException && reason.name === 'AbortError') {
				notice = tr('Pemindaian terputus karena batas waktu koneksi. Sebagian media mungkin sudah tersimpan hasilnya — panel ini menampilkan hasil terbaru.');
			} else {
				error = reason instanceof Error ? reason.message : tr('Gagal menjalankan pemindaian QC.');
			}
		} finally {
			await loadAudits();
			stopPolling();
			if (scanTimer) {
				window.clearInterval(scanTimer);
				scanTimer = null;
			}
			scanning = false;
			// Pemindaian mengubah article_status tiap media, jadi halaman induk
			// harus memuat ulang kalau tidak badge barisnya jadi basi.
			onScanned?.();
		}
	}

	function summaryOf(key: keyof Checks, check: Check): string {
		switch (key) {
			case 'aiOverview': {
				if (check.info) return check.info;
				if (check.error) return check.error;
				const skor = `${check.totalScore ?? '?'}/${check.maxScore ?? 40}`;
				const tanpaDasar = check.unsourcedEntities?.length ? ` · ${check.unsourcedEntities.length} klaim tanpa dasar` : '';
				if (check.status === 'passed') return `Layak AI Overview · skor ${skor}${tanpaDasar}`;
				if (check.status === 'review_required') return `Skor ${skor} — perlu revisi${tanpaDasar}`;
				return tr('Menunggu pemindaian…');
			}
			case 'contentSafety':
				if (check.status === 'passed') return `Risiko ${check.riskLevel ?? 'rendah'}`;
				if (check.status === 'failed') return `Diblokir: ${check.categories?.join(', ') ?? tr('konten ilegal')}`;
				if (check.status === 'review_required') return `Perlu review (risiko ${check.riskLevel ?? '?'})`;
				return tr('Menunggu pemindaian…');
			case 'linkHealth': {
				const n = check.unsafeUrls?.length ?? check.links?.filter((l) => l.status === 'unsafe' || l.status === 'suspicious').length ?? 0;
				if (check.status === 'passed') return tr('Semua link aman');
				if (check.status === 'failed') return `${n} URL tidak aman`;
				if (check.status === 'review_required') return `${n} URL butuh review`;
				return tr('Menunggu pemindaian…');
			}
			case 'brandVoice':
				if (check.status === 'passed' && check.level === 'minor_drift') return tr('Penyimpangan kecil dari suara brand');
				if (check.status === 'passed') return tr('Konsisten dengan suara brand');
				if (check.status === 'failed') return tr('Melenceng dari suara brand');
				return tr('Menunggu pemindaian…');
		}
	}

	const statusTone = (s: CheckStatus) =>
		s === 'running' || s === 'review_required' ? 'warning' : s === 'passed' ? 'success' : s === 'failed' ? 'destructive' : 'muted';
	const statusLabel = (s: CheckStatus) =>
		s === 'running' ? tr('Memindai…') : s === 'passed' ? tr('Lulus') : s === 'failed' ? tr('Gagal') : s === 'review_required' ? tr('Perlu Review') : 'Pending';
	const badge = (tone: string) =>
		tone === 'success'
			? 'background-color: var(--success-soft); color: var(--success);'
			: tone === 'warning'
				? 'background-color: var(--warning-soft); color: var(--warning);'
				: tone === 'destructive'
					? 'background-color: color-mix(in srgb, var(--destructive) 10%, var(--card)); color: var(--destructive);'
					: 'background-color: var(--muted); color: var(--muted-foreground);';

	const articleTone = (s: string) =>
		s === 'ready' ? 'success' : s === 'blocked' ? 'destructive' : s === 'revision' ? 'warning' : 'muted';
	const articleLabel = (s: string) =>
		s === 'ready'
			? tr('Siap Terbit')
			: s === 'blocked'
				? tr('Diblokir')
				: s === 'revision'
					? tr('Perlu Revisi')
					: s === 'draft'
						? tr('Draft Tersedia')
						: s;

	const linkTone = (status?: string) =>
		status === 'ok' ? 'success' : status === 'unreachable' ? 'muted' : status === 'suspicious' ? 'warning' : status === 'unsafe' ? 'destructive' : 'muted';
	const linkLabel = (link: { status?: string; httpStatus?: number | null }) =>
		link.status === 'ok'
			? `OK${link.httpStatus ? ` · ${link.httpStatus}` : ''}`
			: link.status === 'unreachable'
				? tr('Tidak terjangkau')
				: link.status === 'suspicious'
					? tr('Perlu review')
					: link.status === 'unsafe'
						? tr('Tidak aman')
						: (link.status ?? '—');

	const hasDetail = (key: keyof Checks, check: Check) =>
		key === 'linkHealth'
			? (check.links?.length ?? 0) > 0
			: key === 'contentSafety'
				? Boolean(check.categories?.length || check.reasons?.length)
				: key === 'brandVoice'
					? Boolean(check.notes?.length)
					: key === 'aiOverview'
						? Boolean(check.indicators?.length || check.revisionNotes?.length || check.unsourcedEntities?.length || check.error)
						: false;

	/** Skor indikator AI Overview → tone badge (hijau bila >= 7). */
	const indicatorTone = (score: number) => (score >= 7 ? 'success' : 'warning');
</script>

<SlideOver title={tr('Analisis QC Artikel')} subtitle={mediaName} closeLabel={tr('Tutup')} onClose={onClose}>
	{#if gatekeeperConfigured === false}
		<div class="qcNote" style="background-color: var(--warning-soft); color: var(--warning);" role="alert">
			<TriangleAlert size={15} class="mt-0.5 shrink-0" />
			<span>{tr('AI Gatekeeper belum dikonfigurasi oleh admin. Pemindaian tidak akan berjalan sampai admin mengaktifkannya.')}</span>
		</div>
	{/if}

	{#if error}
		<div class="qcNote" style="background-color: color-mix(in srgb, var(--destructive) 9%, var(--card)); color: var(--destructive);" role="alert">
			<TriangleAlert size={15} class="mt-0.5 shrink-0" />
			<span class="flex-1">{error}</span>
			<button class="rounded p-0.5" type="button" aria-label={tr('Tutup error')} onclick={() => (error = '')}><X size={12} /></button>
		</div>
	{/if}
	{#if notice}
		<div class="qcNote" style="background-color: var(--muted); color: var(--muted-foreground);">
			<CircleCheck size={15} class="mt-0.5 shrink-0" />
			<span>{notice}</span>
		</div>
	{/if}

	{#if loading}
		<div class="flex items-center gap-2 text-sm text-muted-foreground"><LoaderCircle size={16} class="animate-spin" />{tr('Memuat hasil pemeriksaan…')}</div>
	{:else}
		<section class="qcBlock" aria-label={tr('Konteks artikel')}>
			<div class="flex items-center justify-between gap-2">
				<p class="qcBlockTitle">{tr('Konteks Artikel')}</p>
				<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style={badge(articleTone(articleStatus))}>{articleLabel(articleStatus)}</span>
			</div>
			{#if media?.article.title}
				<p class="mt-1.5 text-[13px] font-semibold leading-snug">{media.article.title}</p>
			{:else}
				<p class="mt-1.5 text-[13px] text-muted-foreground">{tr('Judul artikel belum tersedia.')}</p>
			{/if}
			<div class="mt-2 grid grid-cols-2 gap-2 text-[12px]">
				<div>
					<p class="text-muted-foreground">{tr('Jumlah kata')}</p>
					<p class="font-semibold">{words != null ? `${words.toLocaleString('id-ID')} ${tr('kata')}` : tr('Tidak ada')}</p>
				</div>
				<div>
					<p class="text-muted-foreground">{tr('Tautan aktif')}</p>
					<p class="font-semibold">{articleLinks ? `${articleLinks.count} ${tr('tautan')}` : tr('Tidak ada')}</p>
				</div>
			</div>
			{#if articleLinks?.rels?.length}
				<p class="mt-2 text-[11px] text-muted-foreground">rel: <code>{articleLinks.rels.join(', ')}</code></p>
			{/if}
			{#if media?.article.updated_at}
				<p class="mt-1 text-[11px] text-muted-foreground">{tr('Terakhir diubah')}: {new Date(media.article.updated_at).toLocaleString('id-ID')}</p>
			{/if}
		</section>

		{#if scanning}
			<div class="qcBlock" aria-live="polite">
				<div class="flex items-center gap-2 text-sm font-semibold" style="color: var(--primary);">
					<LoaderCircle size={16} class="animate-spin" />{tr('Memindai artikel…')} <span class="font-mono text-[12px]">{Math.floor(scanSeconds / 60)}:{String(scanSeconds % 60).padStart(2, '0')}</span>
				</div>
				<p class="mt-1 text-[12px] text-muted-foreground">
					{scannedCount}/{totalMedia} {tr('artikel sudah punya hasil. Hasil per media muncul bertahap; panel ini memperbarui sendiri setiap 4 detik.')}
				</p>
				<div class="qcProgressTrack"><div class="qcProgressFill" style="width: {totalMedia > 0 ? Math.round((scannedCount / totalMedia) * 100) : 0}%;"></div></div>
			</div>
		{/if}

		<section class="qcBlock" aria-label={tr('Pemeriksaan QC')}>
			<p class="qcBlockTitle">{tr('Pemeriksa Artikel')}</p>
			<p class="mt-1 text-[11.5px] text-muted-foreground">{tr('Illegal Content & Link Health adalah gerbang wajib untuk bisa distribusi.')}</p>
			<div class="mt-2.5 space-y-2">
				{#each CHECK_CARDS as card (card.key)}
					{@const check = checks?.[card.key] ?? null}
					{@const status = check?.status ?? 'pending'}
					{@const detailOpen = openCheck === card.key}
					<div class="qcCheck" style="border-color: {status === 'passed' ? 'var(--success)' : status === 'failed' ? 'var(--destructive)' : 'var(--border)'};">
						<div class="flex items-start gap-2">
							<div class="qcCheckIcon">
								{#if card.key === 'aiOverview'}<Sparkles size={15} />
								{:else if card.key === 'contentSafety'}<ShieldAlert size={15} />
								{:else if card.key === 'linkHealth'}<span class="text-xs font-bold">⛓</span>
								{:else}<span class="text-xs font-bold">~</span>{/if}
							</div>
							<div class="min-w-0 flex-1">
								<div class="flex flex-wrap items-center gap-1.5">
									<p class="text-[13px] font-semibold">{card.title}</p>
									{#if card.gate}<span class="rounded-full px-2 py-0.5 text-[10px] font-semibold" style={badge('warning')}>Hard Gate</span>{/if}
								</div>
								<p class="mt-1 text-[11.5px] text-muted-foreground">{check ? summaryOf(card.key, check) : tr('Belum ada hasil pemindaian.')}</p>
							</div>
							<span class="inline-flex shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style={badge(statusTone(status))}>
								{#if status === 'running'}<LoaderCircle size={11} class="animate-spin" />{/if}
								{statusLabel(status)}
							</span>
						</div>

						{#if check && hasDetail(card.key, check)}
							<button class="qcDetailToggle" type="button" aria-expanded={detailOpen} onclick={() => (openCheck = detailOpen ? null : card.key)}>
								{detailOpen ? tr('Sembunyikan detail') : tr('Lihat detail')}
							</button>
							{#if detailOpen}
								<div class="qcDetail">
										{#if card.key === 'aiOverview'}
											<div class="space-y-2 text-[11.5px]">
												<p>
													{tr('Total skor')}: <strong>{check.totalScore ?? '—'}/{check.maxScore ?? 40}</strong>
													{#if check.verdict}{' · '}{check.verdict === 'lulus' ? tr('lulus') : tr('perlu revisi')}{/if}
												</p>
												{#if check.indicators?.length}
													<ul class="space-y-1.5">
														{#each check.indicators as ind (ind.key)}
															<li class="flex items-start gap-2">
																<span class="mt-0.5 inline-flex shrink-0 items-center rounded-full px-1.5 py-0.5 text-[10px] font-bold" style={badge(indicatorTone(ind.score))}>{ind.score}</span>
																<div class="min-w-0 flex-1">
																	<p class="font-semibold">{ind.title}</p>
																	{#if ind.summary}<p class="text-muted-foreground">{ind.summary}</p>{/if}
																</div>
															</li>
														{/each}
													</ul>
												{/if}
												{#if check.unsourcedEntities?.length}
													<div>
														<p class="font-semibold" style="color: var(--warning);">{tr('Klaim tanpa dasar di bahan (indikasi karangan model)')}</p>
														<ul class="list-disc space-y-1 pl-4" style="color: var(--warning);">
															{#each check.unsourcedEntities as ent (ent)}<li>{ent}</li>{/each}
														</ul>
													</div>
												{/if}
												{#if check.briefAvailable === false}
													<p class="text-muted-foreground">
														{tr('Bahan/brief kosong saat pengecekan — klaim spesifik di artikel tidak bisa diverifikasi ke sumber.')}
													</p>
												{/if}
												{#if check.revisionNotes?.length}
													<div>
														<p class="font-semibold">{tr('Catatan revisi')}</p>
														<ul class="list-disc space-y-1 pl-4 text-muted-foreground">
															{#each check.revisionNotes as note (note)}<li>{note}</li>{/each}
														</ul>
													</div>
												{/if}
											</div>
										{:else if card.key === 'linkHealth'}
										<ul class="space-y-1.5">
											{#each check.links ?? [] as link (link.url)}
												<li class="flex items-start gap-2">
													<span class="mt-1 inline-block h-1.5 w-1.5 shrink-0 rounded-full" style="background-color: var(--{linkTone(link.status) === 'muted' ? 'muted-foreground' : linkTone(link.status)});"></span>
													<div class="min-w-0 flex-1">
														<p class="break-all text-[11.5px]">{link.url}</p>
														<p class="text-[11px] text-muted-foreground">
															{linkLabel(link)}{#if link.category && link.category !== 'safe'}{' · ' + link.category}{/if}{#if link.reason}{' · ' + link.reason}{/if}
														</p>
													</div>
												</li>
											{/each}
										</ul>
									{:else if card.key === 'contentSafety'}
										<div class="space-y-1.5 text-[11.5px]">
											<p>{tr('Tingkat risiko')}: <strong>{check.riskLevel ?? '—'}</strong>{#if check.confidence != null}{' · '}{tr('Keyakinan')}{' '}{Math.round(check.confidence * 100)}%{/if}</p>
											{#if check.categories?.length}
												<div class="flex flex-wrap gap-1">
													{#each check.categories as cat (cat)}<span class="rounded-full px-2 py-0.5 text-[10px] font-medium" style={badge('destructive')}>{cat}</span>{/each}
												</div>
											{/if}
											{#if check.reasons?.length}
												<ul class="list-disc space-y-1 pl-4 text-muted-foreground">
													{#each check.reasons as reason (reason)}<li>{reason}</li>{/each}
												</ul>
											{/if}
										</div>
									{:else if card.key === 'brandVoice'}
										<div class="space-y-1.5 text-[11.5px]">
											<p>{tr('Tingkat')}: <strong>{check.level ?? '—'}</strong></p>
											{#if check.notes?.length}
												<ul class="list-disc space-y-1 pl-4 text-muted-foreground">
													{#each check.notes as note (note)}<li>{note}</li>{/each}
												</ul>
											{/if}
										</div>
									{/if}
								</div>
							{/if}
						{/if}
					</div>
				{/each}
			</div>
		</section>

		<section class="qcBlock" aria-label={tr('Kesiapan distribusi kampanye')}>
			<div class="flex items-center justify-between gap-2">
				<p class="qcBlockTitle">{tr('Kesiapan Distribusi Kampanye')}</p>
				<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style={badge(data?.canDistribute ? 'success' : 'warning')}>
					{#if data?.canDistribute}<CircleCheck size={11} />{:else}<Clock size={11} />{/if}
					{data?.canDistribute ? tr('Siap Distribusi') : tr('Terkunci (Butuh QC)')}
				</span>
			</div>
			{#if data?.canDistribute}
				<p class="mt-1.5 text-[11.5px] text-muted-foreground">{tr('Semua hard gate lulus untuk seluruh media terpilih.')}</p>
			{:else if blocking.length > 0}
				<p class="mt-1.5 text-[11.5px] text-muted-foreground">{tr('Media berikut belum lulus hard gate:')}</p>
				<ul class="mt-1.5 space-y-1">
					{#each blocking as m (m.media_id)}
						<li class="flex items-center gap-2 text-[11.5px]">
							<XCircle size={12} class="shrink-0" style="color: var(--warning);" />
							<span class="min-w-0 flex-1 truncate">{m.name}</span>
							<span class="shrink-0 text-muted-foreground">{m.audit.status === 'pending' ? tr('Belum dipindai') : statusLabel(m.audit.status)}</span>
						</li>
					{/each}
				</ul>
			{:else}
				<p class="mt-1.5 text-[11.5px] text-muted-foreground">{tr('Belum ada media terpilih pada kampanye ini.')}</p>
			{/if}
		</section>

		{#if articleStatus === 'ready'}
			<p class="mt-3 text-[11px] text-muted-foreground">
				{tr('Artikel ini berstatus Siap Terbit, jadi pemindaian ulang akan melewatinya — hasil di atas berasal dari pemindaian terakhir. Perubahan manual pada artikel tidak otomatis memicu pemindaian baru; pakai Generate Ulang agar versi terbaru ikut dipindai.')}
			</p>
		{:else if articleStatus === 'blocked'}
			<p class="mt-3 text-[11px] text-muted-foreground">
				{tr('Artikel ini diblokir oleh moderasi konten. Perbaiki isinya lewat Edit Artikel, atau buat ulang dengan Generate Ulang, lalu jalankan pemindaian lagi.')}
			</p>
		{/if}
	{/if}

	{#snippet footer()}
		<!-- Artikel yang sudah lulus QC (Siap Terbit) tak perlu aksi: cukup tombol
		     Ok untuk menutup panel — aturan pemilik produk. -->
		{#if articleStatus === 'ready'}
			<!-- Kembalikan ke OKE sederhana di sebelah KIRI — alur terbit sudah
			     ditangani tombol bar bawah "Lanjut Tinjau & Terbit". -->
			<button class="app-btn app-btn-primary !min-h-9 !px-3.5 !text-xs !mr-auto" type="button" onclick={onClose}>
				<CheckCircle2 size={14} /> {tr('OKE')}
			</button>
		{:else}
			<button class="app-btn app-btn-primary !min-h-9 !px-3.5 !text-xs" type="button" disabled={scanning || gatekeeperConfigured === false} onclick={() => void scan()}>
				{#if scanning}<LoaderCircle size={14} class="animate-spin" />{:else}<ShieldCheck size={14} />{/if}
				{scanning ? tr('Memindai…') : tr('Scan Ulang Artikel Ini')}
			</button>
			<button class="app-btn app-btn-soft !min-h-9 !px-3.5 !text-xs" type="button" onclick={onEdit}><Eye size={14} /> {tr('Edit Artikel')}</button>
			{#if canRegen}
				<button class="app-btn app-btn-soft !min-h-9 !px-3.5 !text-xs" type="button" disabled={generating} onclick={onRegenerate}>
					{#if generating}<LoaderCircle size={14} class="animate-spin" />{:else}<Sparkles size={14} />{/if}
					{generating ? tr('Membuat…') : regenLabel}
				</button>
			{/if}
		{/if}
	{/snippet}
</SlideOver>

<style>
	.qcNote {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		margin-bottom: 12px;
		border-radius: 10px;
		padding: 10px 12px;
		font-size: 12.5px;
	}
	.qcBlock {
		border: 1px solid var(--border);
		border-radius: var(--radius);
		padding: 12px;
	}
	.qcBlock + .qcBlock {
		margin-top: 10px;
	}
	.qcBlockTitle {
		font-size: 11px;
		font-weight: 700;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--muted-foreground);
	}
	.qcCheck {
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 10px;
	}
	.qcCheckIcon {
		display: flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 auto;
		width: 26px;
		height: 26px;
		border-radius: 8px;
		background-color: var(--muted);
	}
	.qcDetailToggle {
		margin-top: 8px;
		font-size: 11px;
		font-weight: 600;
		color: var(--primary);
	}
	.qcDetail {
		margin-top: 6px;
		border-top: 1px dashed var(--border);
		padding-top: 8px;
	}
	.qcProgressTrack {
		margin-top: 8px;
		height: 6px;
		border-radius: 999px;
		background-color: var(--muted);
		overflow: hidden;
	}
	.qcProgressFill {
		height: 100%;
		border-radius: 999px;
		background-color: var(--primary);
		transition: width 300ms ease-out;
	}
</style>
