<script lang="ts">
	// Ringkasan pasca-Gatekeeper — laporan distribusi kampanye Taut Pintar.
	// Port ringkasan/{client,revision-dialog}.tsx.
	//
	// Catatan port: app lama membuka dialog revisi dengan rich editor
	// (ContentEditor) + unggah screenshot. Di sini editor dirender sebagai
	// textarea HTML sederhana memakai endpoint dokumen yang sama — perilaku
	// endpoint identik, hanya kontrol editnya lebih ringkas.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { BadgeCheck, CircleCheck, ClipboardList, Clock, Database, LoaderCircle, Megaphone, Paperclip, RefreshCw, RotateCcw } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import AgentSteps from '../../_agent-steps.svelte';
	import { MANUAL_PROFILER_REASON, fmtIdr, rpCurrency, toJakartaDisplay } from '../../_shared';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	const campaignId = $derived(page.params.id as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);

	type CheckStatus = 'pending' | 'running' | 'passed' | 'failed' | 'review_required';
	type MediaItem = {
		media_id: string;
		name: string;
		media_type: string | null;
		price_idr: number;
		da: number | null;
		ss: number | null;
		est_traffic: number | null;
		link_capacity: number | null;
		scheduled_at: string | null;
		article: { title: string; updated_at: string | null; links: { count: number; rels: string[] } };
		audit: { status: CheckStatus };
		publish: { status: string | null; url: string | null };
		publication_id: string | null;
		document_id: string | null;
		client_approved_at: string | null;
	};
	type AuditsData = {
		campaign: { id: string; taut_status: string | null; entry_mode?: 'ai' | 'manual' | null };
		media: MediaItem[];
		totalCostIdr: number;
		totalCostTc: number;
		tokens?: { used: number };
	};

	const DONE_STATUSES = new Set(['submitted', 'scheduled', 'distributed']);

	let data = $state<AuditsData | null>(null);
	let error = $state('');
	let manualFlow = $state(false);
	let loading = $state(true);
	let acceptingId = $state<string | null>(null);
	let actionNotice = $state('');

	// Dialog revisi.
	let revisionTarget = $state<{ publication_id: string | null; document_id: string | null; name: string } | null>(null);
	let revTitle = $state('');
	let revHtml = $state('');
	let revComment = $state('');
	let revAttachment = $state<{ url: string; name: string } | null>(null);
	let revLoading = $state(false);
	let revUploading = $state(false);
	let revSaving = $state(false);
	let revError = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await loadData();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function loadData() {
		loading = true;
		error = '';
		try {
			const res = await apiRequest(`${apiBase}/gatekeeper/audits`);
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal memuat ringkasan distribusi.'));
			const audits: AuditsData = await res.json();
			data = audits;
			manualFlow = audits.campaign.entry_mode === 'manual';
		} catch {
			error = 'Gagal memuat ringkasan distribusi.';
		} finally {
			loading = false;
		}
	}

	async function acceptArticle(item: MediaItem) {
		if (!item.publication_id) return;
		acceptingId = item.publication_id;
		actionNotice = '';
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/publications/${item.publication_id}/accept`, { method: 'POST' });
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal menyetujui artikel.'));
			if (data)
				data = {
					...data,
					media: data.media.map((m) =>
						m.publication_id === item.publication_id ? { ...m, client_approved_at: new Date().toISOString() } : m
					)
				};
			actionNotice = `Artikel di ${item.name} sudah Anda setujui. Penerbitan tetap menunggu ACC mitra.`;
		} catch (reason) {
			error = reason instanceof Error ? reason.message : 'Gagal menyetujui artikel.';
		} finally {
			acceptingId = null;
		}
	}

	/* ─── Dialog revisi ─── */

	async function openRevision(target: { publication_id: string | null; document_id: string | null; name: string }) {
		revisionTarget = target;
		revComment = '';
		revAttachment = null;
		revError = '';
		revHtml = '';
		revTitle = '';
		if (!target.document_id) return;
		revLoading = true;
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/documents/${target.document_id}/html`);
			if (!res.ok) throw new Error('Gagal memuat isi artikel.');
			const d = await res.json();
			revHtml = d.content ?? '';
			revTitle = d.title ?? '';
		} catch (reason) {
			revError = reason instanceof Error ? reason.message : 'Gagal memuat isi artikel.';
		} finally {
			revLoading = false;
		}
	}

	async function uploadScreenshot(file: File) {
		revUploading = true;
		revError = '';
		try {
			const form = new FormData();
			form.append('file', file);
			const res = await apiRequest(`/v1/brands/${brandId}/assets`, { method: 'POST', body: form });
			const d = await res.json().catch(() => null);
			if (!res.ok || !d?.url) throw new Error(d?.detail ?? 'Gagal mengunggah screenshot.');
			revAttachment = { url: d.url, name: file.name };
		} catch (reason) {
			revError = reason instanceof Error ? reason.message : 'Gagal mengunggah screenshot.';
		} finally {
			revUploading = false;
		}
	}

	async function submitRevision() {
		const target = revisionTarget;
		if (!target?.publication_id || revSaving) return;
		const trimmed = revComment.trim();
		if (trimmed.length < 5) {
			revError = 'Tuliskan komentar minimal 5 karakter supaya blogger tahu apa yang perlu diubah.';
			return;
		}
		revSaving = true;
		revError = '';
		try {
			// 1) Simpan hasil edit sebagai versi artikel baru (endpoint dokumen).
			if (target.document_id) {
				const saveRes = await apiRequest(`/v1/brands/${brandId}/documents/${target.document_id}`, {
					method: 'PUT',
					body: JSON.stringify({
						content: revHtml,
						title: revTitle,
						content_format: 'html',
						change_note: `Revisi klien: ${trimmed}`.slice(0, 255)
					})
				});
				if (!saveRes.ok) throw new Error(await readApiError(saveRes, 'Gagal menyimpan revisi artikel.'));
			}
			// 2) Catat komentar + lampiran, lalu teruskan ke blogger.
			const res = await apiRequest(`/v1/brands/${brandId}/publications/${target.publication_id}/request-revision`, {
				method: 'POST',
				body: JSON.stringify({
					comment: trimmed,
					attachmentUrl: revAttachment?.url ?? null,
					attachmentName: revAttachment?.name ?? null
				})
			});
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal mengirim revisi.'));
			actionNotice = 'Revisi terkirim ke pemilik domain.';
			revisionTarget = null;
			await loadData();
		} catch (reason) {
			revError = reason instanceof Error ? reason.message : 'Gagal mengirim revisi.';
		} finally {
			revSaving = false;
		}
	}

	/* ─── Turunan tampilan ─── */

	const mediaList = $derived(data?.media ?? []);
	const done = $derived(DONE_STATUSES.has(data?.campaign.taut_status ?? ''));
	const publishedCount = $derived(mediaList.filter((m) => m.publish.status === 'published').length);
	const allPublished = $derived(mediaList.length > 0 && publishedCount === mediaList.length);
	const bannerState = $derived(!done ? 'idle' : allPublished ? 'published' : 'waiting');
	const waitingCount = $derived(mediaList.length - publishedCount);
	const progressPercent = $derived(mediaList.length > 0 ? Math.round((publishedCount / mediaList.length) * 100) : 0);
	const statusSub = $derived(
		!done
			? 'Belum didistribusikan — submit lewat The Gatekeeper.'
			: waitingCount > 0
				? `${waitingCount} media menunggu persetujuan mitra`
				: 'Semua media terbit'
	);
	const resume = $derived.by(() => {
		const s = data?.campaign.taut_status ?? '';
		switch (s) {
			case 'draft':
			case 'profiling':
				return { slug: 'profiler', label: 'Lengkapi The Profiler' };
			case 'profiled':
				return { slug: 'matchmaker', label: 'Lanjut ke The Matchmaker' };
			case 'placement':
				return { slug: 'writer', label: 'Lanjut ke The Writer' };
			case 'submitted':
			case 'scheduled':
				return { slug: 'gatekeeper', label: 'Kembali ke The Gatekeeper' };
			default:
				return null;
		}
	});

	const initialOf = (name: string | null | undefined) => (name ?? '').trim().charAt(0).toUpperCase() || '?';
	const avatarTone = (name: string | null | undefined) => {
		const s = (name ?? '').trim().toLowerCase();
		let h = 0;
		for (let i = 0; i < s.length; i += 1) h = (h * 31 + s.charCodeAt(i)) % 6;
		return h;
	};
	const AVA = ['var(--accent)', 'var(--success-soft)', 'var(--warning-soft)', 'var(--muted)', 'var(--accent)', 'var(--success-soft)'];
</script>

<PageTitle title="Taut Pintar — Ringkasan Distribusi" />

<main>
	<AgentSteps current={4} tone={bannerState === 'published' ? 'published' : bannerState === 'waiting' ? 'waiting' : undefined} extra="ringkasan" lockedSteps={manualFlow ? [1] : undefined} lockedReason={MANUAL_PROFILER_REASON} />

	{#if loading}
		<div class="app-card mt-4 flex items-center justify-between gap-3 p-4">
			<span class="flex items-center gap-2 text-sm font-bold"><LoaderCircle size={18} class="animate-spin" /> Memuat ringkasan…</span>
			<button class="app-btn app-btn-soft" type="button" onclick={loadData}><RotateCcw size={14} /> Segarkan</button>
		</div>
		<p class="mt-2 text-sm text-muted-foreground">Mengambil status distribusi terbaru dari server.</p>
	{:else if !data}
		<div class="app-card mt-4 p-4">
			<p class="font-bold">Ringkasan Tidak Tersedia</p>
			<p class="mt-1 text-sm text-muted-foreground">{error || 'Data kampanye belum tersedia.'}</p>
		</div>
	{:else}
		<div
			class="app-card mt-4 p-4"
			style="background-color: {bannerState === 'published' ? 'var(--success-soft)' : 'var(--accent)'}; color: {bannerState === 'published' ? 'var(--success)' : 'var(--accent-foreground)'};"
		>
			<div class="flex items-center gap-2">
				{#if bannerState === 'published'}<CircleCheck size={18} />{:else}<LoaderCircle size={18} />{/if}
				<p class="font-bold">
					{bannerState === 'published'
						? 'Distribusi Selesai — Semua Artikel Terbit'
						: bannerState === 'waiting'
							? `Distribusi Berhasil — Menunggu ACC Mitra (${publishedCount}/${mediaList.length} Terbit)`
							: 'Kampanye Belum Terdistribusi'}
				</p>
			</div>
			<p class="mt-1 text-sm" style="opacity: .85;">
				{bannerState === 'published'
					? 'Semua artikel sudah terbit di media masing-masing. Bukti tayang ada di kolom Status Terbit.'
					: bannerState === 'waiting'
						? 'Kampanye sudah disubmit. Sebagian artikel masih menunggu persetujuan mitra sebelum terbit.'
						: 'Submit kampanye lewat The Gatekeeper dulu untuk melihat laporan distribusi.'}
			</p>
		</div>

		{#if resume}
			<div class="mt-3 flex justify-end">
				<a class="app-btn app-btn-primary" href={`/klien/${brandUrl}/taut-pintar/${campaignId}/${resume.slug}`}>
					<RotateCcw size={14} /> {resume.label}
				</a>
			</div>
		{/if}

		<div class="mt-3 grid gap-3 md:grid-cols-2">
			<div class="app-card p-4">
				<div class="flex items-center gap-3">
					<span class="flex h-9 w-9 items-center justify-center rounded-lg" style="background-color: var(--accent); color: var(--accent-foreground);"><Database size={20} /></span>
					<p class="text-sm text-muted-foreground">Kredit Distribusi Media</p>
				</div>
				<p class="mt-2 text-lg font-bold">{fmtIdr(data.totalCostTc)} Kredit</p>
				<p class="text-xs text-muted-foreground">Setara {rpCurrency(data.totalCostIdr)}</p>
			</div>
			<div class="app-card p-4">
				<div class="flex items-center gap-3">
					<span class="flex h-9 w-9 items-center justify-center rounded-lg" style="background-color: var(--warning-soft); color: var(--warning);"><ClipboardList size={20} /></span>
					<p class="text-sm text-muted-foreground">Status Kampanye</p>
				</div>
				<p class="mt-2 text-lg font-bold">{publishedCount} / {mediaList.length} Terbit</p>
				<div class="mt-2 h-2 w-full overflow-hidden rounded-full" style="background-color: var(--muted);" role="progressbar" aria-valuenow={progressPercent} aria-valuemin="0" aria-valuemax="100">
					<div class="h-full" style="width: {progressPercent}%; background-color: var(--primary);"></div>
				</div>
				<p class="mt-1 text-xs text-muted-foreground">{statusSub}</p>
			</div>
		</div>

		{#if error}<p class="mt-2 text-sm" style="color: var(--destructive);">{error}</p>{/if}

		<section class="app-card mt-3 p-4">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<div class="flex items-center gap-2">
					<span class="flex h-8 w-8 items-center justify-center rounded-lg" style="background-color: var(--accent); color: var(--accent-foreground);"><Megaphone size={18} /></span>
					<div>
						<p class="font-bold">Media Terpilih ({mediaList.length})</p>
						<p class="text-xs text-muted-foreground">Daftar media untuk distribusi konten kampanye ini.</p>
					</div>
				</div>
				<div class="flex gap-2">
					<a class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" href={`/klien/${brandUrl}/taut-pintar`}>Kembali ke Daftar Kampanye</a>
					<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={loadData}><RefreshCw size={14} /> Segarkan</button>
				</div>
			</div>

			{#if mediaList.length > 0}
				<div class="mt-3 overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="text-left text-[11px] uppercase tracking-wide text-muted-foreground">
								<th class="px-3 py-2 font-semibold">Media</th>
								<th class="px-3 py-2 font-semibold">Tipe</th>
								<th class="px-3 py-2 font-semibold">DA / SS</th>
								<th class="px-3 py-2 font-semibold">Link Artikel</th>
								<th class="px-3 py-2 font-semibold">Jadwal Terbit</th>
								<th class="px-3 py-2 font-semibold">Judul Artikel</th>
								<th class="px-3 py-2 font-semibold">Status Terbit</th>
								<th class="px-3 py-2 font-semibold">Aksi</th>
							</tr>
						</thead>
						<tbody>
							{#each mediaList as m (m.media_id)}
								<tr class="border-t">
									<td class="px-3 py-2">
										<div class="flex items-center gap-2">
											<span class="flex h-7 w-7 items-center justify-center rounded-full text-xs font-bold" style="background-color: {AVA[avatarTone(m.name)]};">{initialOf(m.name)}</span>
											<span class="font-medium">{m.name || '—'}</span>
										</div>
									</td>
									<td class="px-3 py-2">
										{#if m.media_type}<span class="rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--accent); color: var(--accent-foreground);">{m.media_type}</span>{:else}—{/if}
									</td>
									<td class="px-3 py-2">{m.da != null ? m.da : '—'}{m.ss != null ? ` / ${m.ss}%` : ''}</td>
									<td class="px-3 py-2">
										{#if m.article.links.count > 0}
											<span class="text-xs">{m.article.links.count} link{m.article.links.rels.length > 0 ? ` · ${m.article.links.rels.join(', ')}` : ' · tanpa rel'}</span>
										{:else}
											<span class="text-xs text-muted-foreground">Tidak ada link</span>
										{/if}
									</td>
									<td class="px-3 py-2">
										<span class="inline-flex items-center gap-1 text-xs" style="color: {m.publish.status === 'published' ? 'var(--success)' : 'var(--muted-foreground)'};">
											<Clock size={12} />{toJakartaDisplay(m.scheduled_at) ?? 'Segera'}
										</span>
									</td>
									<td class="max-w-[260px] px-3 py-2">
										{#if m.article.title}
											<span class="block truncate text-xs">{m.article.title}</span>
										{:else}
											<span class="text-xs text-muted-foreground">Belum ada artikel</span>
										{/if}
									</td>
									<td class="px-3 py-2">
										{#if m.publish.status === 'published' && m.publish.url}
											<a href={m.publish.url} target="_blank" rel="noreferrer" class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--success-soft); color: var(--success);">
												<CircleCheck size={11} /> Terbit - Lihat
											</a>
										{:else if m.publish.status === 'failed'}
											<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--destructive); color: #fff;">Gagal</span>
										{:else if m.publish.status === 'processing' || m.publish.status === 'queued' || m.publish.status === 'retrying'}
											<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">Diproses</span>
										{:else if m.publish.status === 'waiting_partner' || m.publish.status === 'partner_approved' || m.publish.status === 'pending'}
											<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--warning-soft); color: var(--warning);"><Clock size={11} /> Menunggu Mitra</span>
										{:else}
											<span class="text-xs text-muted-foreground">—</span>
										{/if}
									</td>
									<td class="px-3 py-2">
										<div class="flex items-center gap-2">
											<button
												class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs"
												type="button"
												disabled={!m.publication_id || !m.document_id}
												title={!m.document_id ? 'Artikel untuk media ini belum tersedia' : undefined}
												onclick={() => openRevision({ publication_id: m.publication_id, document_id: m.document_id, name: m.name })}
											>Revisi</button>
											{#if m.client_approved_at}
												<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--success-soft); color: var(--success);"><BadgeCheck size={11} /> Disetujui klien</span>
											{:else}
												<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" disabled={!m.publication_id || acceptingId === m.publication_id} onclick={() => acceptArticle(m)}>
													{acceptingId === m.publication_id ? 'Menyimpan…' : 'Accept'}
												</button>
											{/if}
										</div>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				{#if actionNotice}<p class="mt-3 text-sm" style="color: var(--success);">{actionNotice}</p>{/if}
			{:else}
				<p class="mt-3 text-sm text-muted-foreground">Belum ada media terpilih.</p>
			{/if}
		</section>
	{/if}
</main>

{#if revisionTarget}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card my-6 w-full max-w-3xl p-5" aria-label="Revisi Artikel">
			<p class="text-lg font-bold">Revisi Artikel — {revisionTarget.name ?? 'media'}</p>
			<p class="mt-1 text-sm text-muted-foreground">
				Edit artikelnya langsung, jelaskan yang perlu diubah, lalu kirim ke pemilik domain. Revisi boleh berkali-kali.
			</p>

			{#if revError}<p class="mt-3 text-sm" style="color: var(--destructive);">{revError}</p>{/if}

			{#if revLoading}
				<div class="mt-4 flex items-center justify-center gap-2 text-sm text-muted-foreground"><LoaderCircle size={18} class="animate-spin" /> Memuat artikel…</div>
			{:else}
				<div class="mt-3 space-y-3">
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Judul artikel</span>
						<input class="app-input mt-1 w-full" maxlength="300" bind:value={revTitle} />
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Isi Artikel (HTML)</span>
						<textarea class="app-input mt-1 w-full font-mono text-xs" rows="10" bind:value={revHtml}></textarea>
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Komentar untuk pemilik domain</span>
						<textarea class="app-input mt-1 w-full" rows="3" placeholder="mis. Paragraf kedua tolong ganti nama produk menjadi versi terbaru." bind:value={revComment}></textarea>
					</label>
					<div>
						<span class="text-xs font-medium text-muted-foreground">Screenshot (opsional)</span>
						<div class="mt-1 flex flex-wrap items-center gap-3">
							<input type="file" accept="image/png,image/jpeg,image/webp,image/gif" disabled={revUploading} onchange={(e) => { const f = (e.currentTarget as HTMLInputElement).files?.[0]; if (f) void uploadScreenshot(f); }} />
							{#if revUploading}<LoaderCircle size={14} class="animate-spin" />{/if}
							{#if revAttachment}
								<span class="flex items-center gap-2 text-xs text-muted-foreground">
									<Paperclip size={14} />{revAttachment.name}
									<button class="underline" type="button" onclick={() => (revAttachment = null)}>Hapus</button>
								</span>
							{/if}
						</div>
					</div>
				</div>
			{/if}

			<div class="mt-4 flex items-center justify-end gap-3">
				<button class="app-btn app-btn-soft" type="button" disabled={revSaving || revUploading} onclick={() => (revisionTarget = null)}>Batal</button>
				<button class="app-btn app-btn-primary" type="button" disabled={revSaving || revUploading || revLoading} onclick={submitRevision}>
					{#if revSaving}<LoaderCircle size={14} class="animate-spin" />{/if}
					Kirim Revisi
				</button>
			</div>
		</dialog>
	</div>
{/if}
