<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Port app/berita-acara/[token]/page.tsx — dokumen berita acara kampanye
	// publik (tanpa login), diakses lewat token share. Data dari
	// /v1/campaign-reports/{token} (sumber sama dengan halaman lama).
	import { onMount } from 'svelte';
	import { page } from '$app/state';

	const token = $derived(page.params.token as string);

	let loading = $state(true);
	let report = $state<any>(null);
	let error = $state('');

	onMount(() => {
		void (async () => {
			const res = await fetch(`/v1/campaign-reports/${encodeURIComponent(token)}`, { cache: 'no-store' });
			if (res.ok) {
				report = await res.json();
			} else {
				error = tr('Berita acara tidak ditemukan atau tautannya sudah dicabut.');
			}
			loading = false;
		})();
	});

	const fmtDate = (v: unknown) =>
		v ? new Date(v as string).toLocaleDateString('id-ID', { dateStyle: 'long' }) : '—';
	const fmtDateTime = (v: unknown) =>
		v ? new Date(v as string).toLocaleString('id-ID', { dateStyle: 'long', timeStyle: 'short' }) : '—';

	const campaignTone = (status: string) =>
		status === 'active' ? 'var(--success)' : status === 'completed' ? 'var(--primary)' : 'var(--destructive)';
	const pubTone = (status: string) =>
		status === 'published' || status === 'completed'
			? 'var(--success)'
			: status === 'failed' || status === 'rejected'
				? 'var(--destructive)'
				: 'var(--warning)';

	function print() {
		window.print();
	}
</script>

<svelte:head>
	<title>{tr("Berita Acara Kampanye")}</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<main class="min-h-screen bg-[linear-gradient(160deg,#eef2f7,#f8fafc)] px-4 py-10">
	<div class="mx-auto w-full max-w-3xl">
		{#if loading}
			<p class="text-center text-sm text-muted-foreground">{tr("Memuat berita acara…")}</p>
		{:else if error || !report}
			<div class="app-card p-8 text-center">
				<h1 class="text-xl font-semibold">Tautan tidak tersedia</h1>
				<p class="mt-2 text-sm text-muted-foreground">{tr("Berita acara ini sudah dicabut atau tautannya salah. Minta tautan terbaru kepada pemilik kampanye.")}</p>
			</div>
		{:else}
			<article class="app-card p-7">
				<div class="flex items-center justify-between gap-3">
					<img
						src="/tauthub-logo.webp"
						alt="Tauthub"
						width="140"
						height="35"
						style="display:block; width:auto; height:26px; object-fit:contain;"
					/>
					<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={print}>{tr("Cetak / Simpan PDF")}</button>
				</div>

				<header class="mt-6">
					<p class="text-xs font-semibold uppercase tracking-[.08em] text-muted-foreground">{report.brand_name}</p>
					<h1 class="mt-1 text-2xl font-semibold tracking-tight">{report.campaign.name}</h1>
					<div class="mt-2 flex flex-wrap items-center gap-2 text-xs">
						<span
							class="rounded-full px-2 py-0.5 font-medium"
							style="background: var(--accent); color: {campaignTone(report.campaign.status)};"
						>{report.campaign.status}</span>
						<span class="text-muted-foreground">{tr("· Dibuat")} {fmtDate(report.campaign.created_at)}</span>
					</div>
					{#if report.campaign.description}
						<p class="mt-3 text-sm text-muted-foreground">{report.campaign.description}</p>
					{/if}
				</header>

				<div class="mt-6 grid gap-3 sm:grid-cols-4">
					<div class="rounded-lg border p-3">
						<p class="text-xs text-muted-foreground">Artikel dibuat</p>
						<p class="mt-1 text-xl font-bold tabular-nums">{report.summary.articles_created}</p>
					</div>
					<div class="rounded-lg border p-3">
						<p class="text-xs text-muted-foreground">Artikel terbit</p>
						<p class="mt-1 text-xl font-bold tabular-nums">{report.summary.published_count}</p>
					</div>
					<div class="rounded-lg border p-3">
						<p class="text-xs text-muted-foreground">{tr("Media terbit")}</p>
						<p class="mt-1 text-xl font-bold tabular-nums">{report.summary.media_published_count}</p>
					</div>
					<div class="rounded-lg border p-3">
						<p class="text-xs text-muted-foreground">TC terpakai</p>
						<p class="mt-1 text-xl font-bold tabular-nums">{report.summary.tc_used.toLocaleString('id-ID')}</p>
					</div>
				</div>

				<section class="mt-6">
					<h2 class="text-sm font-semibold">{tr("Media yang menerbitkan")}</h2>
					{#if report.summary.published_media_names}
						<p class="mt-1.5 text-sm text-muted-foreground">
							{report.summary.published_media_names.split(' · ').join(', ')}
						</p>
					{:else}
						<p class="mt-1.5 text-sm text-muted-foreground">{tr("Belum ada artikel yang terbit.")}</p>
					{/if}
				</section>

				<section class="mt-6">
					<h2 class="text-sm font-semibold">{tr("Daftar publikasi")}</h2>
					{#if report.publications.length === 0}
						<p class="mt-1.5 text-sm text-muted-foreground">{tr("Belum ada publikasi dalam kampanye ini.")}</p>
					{:else}
						<ul class="mt-2 divide-y">
							{#each report.publications as pub (pub.id)}
								<li class="flex flex-wrap items-center justify-between gap-3 py-2.5">
									<div class="min-w-0">
										<p class="text-sm font-medium">
											{pub.document_title ?? 'Artikel'} → {pub.media_name ?? tr('Media')}
										</p>
										<p class="text-xs text-muted-foreground">
											{pub.published_at
												? `Terbit ${fmtDateTime(pub.published_at)}`
												: `Dibuat ${fmtDate(report.campaign.created_at)}`}{pub.media_category ? ` · ${pub.media_category}` : ''}
										</p>
									</div>
									<div class="flex flex-wrap items-center gap-2.5">
										{#if pub.published_url}
											<a class="text-xs font-medium" style="color: var(--primary);" href={pub.published_url} target="_blank" rel="noreferrer">{tr("Lihat terbit")}</a>
										{/if}
										<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background: var(--accent); color: {pubTone(pub.status)};">
											{pub.status}
										</span>
									</div>
								</li>
							{/each}
						</ul>
					{/if}
				</section>

				<p class="mt-6 border-t pt-4 text-xs text-muted-foreground"> {tr("Dokumen berita acara digenerate otomatis dari data kampanye Tauthub pada")} {fmtDateTime(report.generated_at)} WIB.
					TC terpakai mencakup biaya pembuatan dan distribusi konten kampanye ini.
				</p>
			</article>
		{/if}
	</div>
</main>
