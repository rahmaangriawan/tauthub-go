<script lang="ts">
	// SEO & Performa mitra — port section "SEO & Performa": cek DA/PA/spam
	// domain (cache 24 jam) dan audit kecepatan PageSpeed per URL media.
	import { onMount } from 'svelte';
	import { Gauge, Search } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	let loading = $state(true);
	let history = $state<any[]>([]);
	let domain = $state('');
	let da = $state<any>(null);
	let error = $state('');
	let busy = $state(false);
	let audit = $state<any>(null);
	let auditURL = $state('');
	let auditStrategy = $state<'mobile' | 'desktop'>('mobile');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await loadHistory();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function loadHistory() {
		const res = await apiRequest('/v1/partner/seo/check');
		if (res.ok) history = (await res.json()).history ?? [];
	}

	async function checkDomain(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const res = await apiRequest('/v1/partner/seo/check', {
				method: 'POST',
				body: JSON.stringify({ domain: domain.trim() })
			});
			const data = await res.json();
			if (!res.ok) {
				error = data?.detail ?? 'Pemeriksaan domain gagal.';
				return;
			}
			da = data;
			await loadHistory();
		} finally {
			busy = false;
		}
	}

	async function runAudit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const res = await apiRequest('/v1/partner/seo/speed-audit', {
				method: 'POST',
				body: JSON.stringify({ url: auditURL.trim(), strategy: auditStrategy })
			});
			const data = await res.json();
			if (!res.ok) {
				error = data?.detail ?? 'Audit kecepatan gagal.';
				return;
			}
			audit = data;
		} finally {
			busy = false;
		}
	}

	const dt = (v: string | null) => (v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—');
	function scoreColor(score: number | null) {
		if (score == null) return 'var(--muted-foreground)';
		if (score >= 90) return 'var(--success, #16794b)';
		if (score >= 50) return 'var(--warning, #a15c00)';
		return 'var(--destructive, #c52b36)';
	}
</script>

<section class="mx-auto max-w-4xl">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">SEO & Performa</h1>
		<p class="mt-1 text-sm text-muted-foreground">Periksa otoritas domain dan kecepatan website media Anda.</p>
	</header>

	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	<form class="app-card mt-4 flex flex-wrap items-end gap-2 p-4" onsubmit={checkDomain}>
		<label class="block min-w-[240px] flex-1">
			<span class="text-xs font-medium text-muted-foreground">Domain untuk diperiksa (DA / PA / spam)</span>
			<input class="app-input mt-1 w-full" required placeholder="media.example" bind:value={domain} />
		</label>
		<button class="app-btn app-btn-primary" type="submit" disabled={busy}>
			<Search size={15} /> {busy ? 'Memeriksa…' : 'Periksa Domain'}
		</button>
	</form>

	{#if da}
		<div class="app-card mt-3 p-4">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<h2 class="text-sm font-semibold">Hasil: {da.domain}</h2>
				<span class="text-xs text-muted-foreground">{da.cached ? 'dari cache' : 'baru diperiksa'} · {dt(da.checked_at)}</span>
			</div>
			<div class="mt-3 grid grid-cols-3 gap-3">
				<div><p class="text-xs text-muted-foreground">Domain Authority</p><p class="text-2xl font-bold tabular-nums">{da.da ?? '—'}</p></div>
				<div><p class="text-xs text-muted-foreground">Page Authority</p><p class="text-2xl font-bold tabular-nums">{da.pa ?? '—'}</p></div>
				<div><p class="text-xs text-muted-foreground">Spam Score</p><p class="text-2xl font-bold tabular-nums">{da.spam_score ?? '—'}</p></div>
			</div>
		</div>
	{/if}

	{#if !loading && history.length > 0}
		<div class="app-card mt-3 overflow-x-auto">
			<div class="border-b px-4 py-3"><h2 class="text-sm font-semibold">Riwayat Pemeriksaan</h2></div>
			<table class="w-full text-sm">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2 font-medium">Domain</th>
						<th class="px-4 py-2 text-right font-medium">DA</th>
						<th class="px-4 py-2 text-right font-medium">PA</th>
						<th class="px-4 py-2 text-right font-medium">Spam</th>
						<th class="px-4 py-2 font-medium">Tanggal</th>
					</tr>
				</thead>
				<tbody>
					{#each history as h (h.domain + h.checked_at)}
						<tr class="border-t">
							<td class="px-4 py-2">{h.domain}</td>
							<td class="px-4 py-2 text-right tabular-nums">{h.da ?? '—'}</td>
							<td class="px-4 py-2 text-right tabular-nums">{h.pa ?? '—'}</td>
							<td class="px-4 py-2 text-right tabular-nums">{h.spam_score ?? '—'}</td>
							<td class="px-4 py-2 text-xs text-muted-foreground">{dt(h.checked_at)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}

	<form class="app-card mt-4 p-4" onsubmit={runAudit}>
		<div class="flex items-center gap-2">
			<Gauge size={16} />
			<h2 class="text-sm font-semibold">Audit Kecepatan</h2>
		</div>
		<p class="mt-0.5 text-xs text-muted-foreground">Hanya URL dari media milik Anda yang dapat diaudit.</p>
		<div class="mt-3 flex flex-wrap items-end gap-2">
			<label class="block min-w-[240px] flex-1">
				<span class="text-xs font-medium text-muted-foreground">URL halaman</span>
				<input class="app-input mt-1 w-full" required placeholder="https://media.example/artikel" bind:value={auditURL} />
			</label>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Perangkat</span>
				<select class="app-input mt-1" bind:value={auditStrategy}>
					<option value="mobile">Mobile</option>
					<option value="desktop">Desktop</option>
				</select>
			</label>
			<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? 'Menganalisis…' : 'Jalankan Audit'}</button>
		</div>
	</form>

	{#if audit}
		<div class="app-card mt-3 p-4">
			<div class="flex flex-wrap items-center justify-between gap-2">
				<h2 class="text-sm font-semibold">{audit.normalized_url ?? audit.url}</h2>
				<span class="text-xs text-muted-foreground">{audit.strategy} · {audit.cached ? 'dari cache' : 'baru'} · {dt(audit.created_at)}</span>
			</div>
			<div class="mt-3 grid grid-cols-2 gap-3 sm:grid-cols-4">
				{#each [['Performance', audit.performance_score], ['Accessibility', audit.accessibility_score], ['Best Practices', audit.best_practices_score], ['SEO', audit.seo_score]] as [label, score] (label)}
					<div>
						<p class="text-xs text-muted-foreground">{label}</p>
						<p class="text-2xl font-bold tabular-nums" style="color: {scoreColor(score)};">{score ?? '—'}</p>
					</div>
				{/each}
			</div>
			<div class="mt-3 grid grid-cols-3 gap-3 text-sm">
				<div><p class="text-xs text-muted-foreground">LCP</p><p class="tabular-nums">{audit.lcp_ms != null ? `${audit.lcp_ms} ms` : '—'}</p></div>
				<div><p class="text-xs text-muted-foreground">INP</p><p class="tabular-nums">{audit.inp_ms != null ? `${audit.inp_ms} ms` : '—'}</p></div>
				<div><p class="text-xs text-muted-foreground">CLS</p><p class="tabular-nums">{audit.cls ?? '—'}</p></div>
			</div>

			{#if Array.isArray(audit.audits) && audit.audits.length > 0}
				<h3 class="mt-4 text-sm font-semibold">Rekomendasi Perbaikan</h3>
				<ul class="mt-2 space-y-2 text-sm">
					{#each audit.audits as a (a.id)}
						<li>
							<p class="font-medium">{a.title}</p>
							{#if a.displayValue}<p class="text-xs text-muted-foreground">{a.displayValue}</p>{/if}
							{#if a.description}<p class="text-xs text-muted-foreground">{a.description}</p>{/if}
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}
</section>
