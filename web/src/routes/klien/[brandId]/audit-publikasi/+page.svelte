<script lang="ts">
	// Audit Publikasi — ringkasan kesehatan link artikel terbit (12 bulan garansi).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ShieldCheck } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);
	type Row = {
		id: string; document_title: string | null; media_name: string | null; published_url: string;
		published_at: string | null; state: string; http_status: number | null;
		consecutive_failures: number | null; last_checked_at: string | null; error_message: string | null;
	};

	let rows = $state<Row[]>([]);
	let summary = $state<{ watched: number; live: number; gone: number; unreachable: number; no_url: number } | null>(null);
	let loading = $state(true);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			const [s, l] = await Promise.all([
				apiRequest(`/v1/brands/${brandId}/link-monitor?view=summary`),
				apiRequest(`/v1/brands/${brandId}/link-monitor?limit=200`)
			]);
			if (s.ok) summary = await s.json();
			if (l.ok) rows = (await l.json()).items ?? [];
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	const audited = $derived(rows.filter((r) => r.last_checked_at !== null));
	const healthy = $derived(audited.filter((r) => r.state === 'live').length);
	const issues = $derived(rows.filter((r) => r.state === 'gone' || r.state === 'unreachable'));
	const compliance = $derived(audited.length === 0 ? null : Math.round((healthy / audited.length) * 100));
	const fmtDate = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID') : '—');
</script>

<PageTitle title="Audit Publikasi" />

<section>
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Audit Publikasi</h1>
		<p class="mt-1 text-sm text-muted-foreground">
			Rekap kepatuhan artikel terbit: link hidup, hilang, atau tidak terjangkau.
		</p>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat audit…</p>
	{:else}
		<div class="mt-4 grid gap-3 sm:grid-cols-3">
			<div class="app-card p-4">
				<p class="text-[11px] font-semibold tracking-wide text-muted-foreground">SKOR KEPATUHAN</p>
				<p class="mt-1.5 text-2xl font-bold tabular-nums">
					{compliance === null ? '—' : compliance + '%'}
				</p>
				<p class="mt-1 text-xs text-muted-foreground">
					{compliance === null ? 'Belum ada pemeriksaan link' : `${healthy} dari ${audited.length} link diperiksa`}
				</p>
			</div>
			<div class="app-card p-4">
				<p class="text-[11px] font-semibold tracking-wide text-muted-foreground">ARTIKEL DIPANTAU</p>
				<p class="mt-1.5 text-2xl font-bold tabular-nums">{summary?.watched ?? 0}</p>
				<p class="mt-1 text-xs text-muted-foreground">Masa garansi 12 bulan</p>
			</div>
			<div class="app-card p-4">
				<p class="text-[11px] font-semibold tracking-wide text-muted-foreground">PERLU TINDAKAN</p>
				<p class="mt-1.5 text-2xl font-bold tabular-nums" style={issues.length > 0 ? 'color: var(--destructive);' : ''}>{issues.length}</p>
				<p class="mt-1 text-xs text-muted-foreground">Link hilang / tidak terjangkau</p>
			</div>
		</div>

		{#if issues.length === 0}
			<div class="app-card mt-4 flex items-center gap-3 px-5 py-6">
				<ShieldCheck size={20} style="color: var(--success);" />
				<p class="text-sm">
					{rows.length === 0
						? 'Belum ada artikel terbit untuk diaudit.'
						: 'Tidak ada masalah link terdeteksi pada artikel yang dipantau.'}
				</p>
			</div>
		{:else}
			<div class="app-card mt-4 overflow-x-auto">
				<table class="w-full text-sm">
					<thead>
						<tr class="border-b text-left text-[11px] tracking-wide text-muted-foreground">
							<th class="px-4 py-2.5 font-semibold">ARTIKEL</th>
							<th class="px-4 py-2.5 font-semibold">MEDIA</th>
							<th class="px-4 py-2.5 font-semibold">MASALAH</th>
							<th class="px-4 py-2.5 font-semibold">DIPERIKSA</th>
						</tr>
					</thead>
					<tbody>
						{#each issues as row (row.id)}
							<tr class="border-b last:border-0">
								<td class="px-4 py-2.5">
									<a class="font-medium hover:underline" href={row.published_url} target="_blank" rel="noreferrer">{row.document_title ?? '—'}</a>
								</td>
								<td class="px-4 py-2.5 text-muted-foreground">{row.media_name ?? '—'}</td>
								<td class="px-4 py-2.5 text-xs" style="color: var(--destructive);">
									{row.state === 'gone' ? 'Hilang (404)' : 'Tidak terjangkau'}
									{#if row.http_status}<span class="ml-1">HTTP {row.http_status}</span>{/if}
									{#if (row.consecutive_failures ?? 0) > 0}<span class="ml-1">({row.consecutive_failures}× berturut)</span>{/if}
								</td>
								<td class="px-4 py-2.5 text-muted-foreground">{fmtDate(row.last_checked_at)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	{/if}
</section>
