<script lang="ts">
	// Akses Crawler mitra — port section "Akses Crawler": kredensial crawler
	// per media (terbitkan/rotasi/nonaktifkan/cabut), uji koneksi ke website,
	// salinan Cloudflare Worker, dan diagnostik terakhir.
	import { onMount } from 'svelte';
	import { Copy, RefreshCw, Server, ShieldCheck, Trash2, Zap } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	let loading = $state(true);
	let media = $state<any[]>([]);
	let selectedMedia = $state('');
	let credential = $state<any>(null);
	let diagnostics = $state<any[]>([]);
	let issued = $state<any>(null);
	let consent = $state(false);
	let notice = $state('');
	let error = $state('');
	let busy = $state(false);
	let testResult = $state<any>(null);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			const res = await apiRequest('/v1/partner/media?limit=200');
			if (res.ok) media = (await res.json()).media ?? [];
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function loadCrawler() {
		if (!selectedMedia) return;
		testResult = null;
		const res = await apiRequest(`/v1/partner/crawler?media_id=${selectedMedia}`);
		if (res.ok) {
			const data = await res.json();
			credential = data.credential;
			diagnostics = data.diagnostics ?? [];
		}
	}

	async function action(kind: 'issue' | 'rotate' | 'disable' | 'revoke' | 'test') {
		if (!selectedMedia) return;
		busy = true;
		error = '';
		notice = '';
		try {
			const payload: Record<string, unknown> = { action: kind, media_id: selectedMedia };
			if (kind === 'issue') {
				if (!consent) {
					error = 'Centang persetujuan dokumen akses crawler terlebih dahulu.';
					return;
				}
				payload.consent = true;
			}
			const res = await apiRequest('/v1/partner/crawler', { method: 'POST', body: JSON.stringify(payload) });
			const data = await res.json();
			if (!res.ok) {
				error = data?.detail ?? 'Aksi crawler gagal.';
				return;
			}
			if (kind === 'issue' || kind === 'rotate') {
				issued = data;
				notice = 'Kredensial dibuat. Salin secret SEKARANG — hanya ditampilkan sekali.';
			} else if (kind === 'test') {
				testResult = data;
				notice = data.message;
			} else {
				notice = kind === 'disable' ? 'Akses crawler dinonaktifkan.' : 'Akses crawler dicabut.';
			}
			await loadCrawler();
		} finally {
			busy = false;
		}
	}

	async function copy(label: string, value: string) {
		try {
			await navigator.clipboard.writeText(value);
			notice = `${label} disalin.`;
		} catch { /* clipboard tidak tersedia */ }
	}

	const dt = (v: string | null) => (v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—');
	const verdictLabel: Record<string, string> = {
		ok: 'Normal', blocked_cloudflare: 'Diblokir Cloudflare', blocked_waf: 'Diblokir WAF',
		blocked_origin: 'Diblokir origin', rate_limited: 'Rate limited', signature_rejected: 'Signature ditolak'
	};
</script>

<section class="mx-auto max-w-4xl">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Akses Crawler</h1>
		<p class="mt-1 text-sm text-muted-foreground">
			Kredensial per website agar crawler Tauthub dapat memeriksa konten tanpa diblokir WAF.
		</p>
	</header>

	{#if notice}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}
	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat media…</p>
	{:else}
		<div class="app-card mt-4 flex flex-wrap items-center gap-2 p-3">
			<Select
				bind:value={selectedMedia}
				ariaLabel="Pilih media"
				placeholder="Pilih media…"
				minWidth={240}
				onchange={loadCrawler}
				options={[
					{ value: '', label: 'Pilih media…' },
					...media.map((m) => ({ value: m.id, label: m.name }))
				]}
			/>
			{#if selectedMedia}
				<button class="app-btn app-btn-soft" disabled={busy} onclick={() => action('test')}><Zap size={14} /> Uji Koneksi</button>
			{/if}
		</div>

		{#if selectedMedia}
			{#if testResult}
				<div class="app-card mt-3 p-4">
					<p class="text-sm font-medium">Hasil uji: {testResult.ok ? 'Berhasil' : 'Gagal'}</p>
					<p class="mt-1 text-xs text-muted-foreground">
						Verdict: {verdictLabel[testResult.verdict] ?? testResult.verdict}
						{#if testResult.signed_verdict} · signed: {verdictLabel[testResult.signed_verdict] ?? testResult.signed_verdict}{/if}
					</p>
					<p class="mt-1 text-sm">{testResult.message}</p>
					{#if testResult.hint}<p class="mt-1 text-xs" style="color: var(--warning, #a15c00);">{testResult.hint}</p>{/if}
				</div>
			{/if}

			{#if issued}
				<div class="app-card mt-3 p-4" style="border-color: var(--success, #16794b);">
					<p class="text-xs font-semibold" style="color: var(--success);">Secret hanya ditampilkan sekali — salin ke Worker Secret Cloudflare.</p>
					<div class="mt-2 space-y-2">
						<div class="flex gap-2">
							<input class="app-input w-full font-mono text-xs" readonly value={issued.crawlerId} />
							<button class="app-btn app-btn-soft shrink-0" onclick={() => copy('Site ID', issued.crawlerId)}><Copy size={13} /></button>
						</div>
						<div class="flex gap-2">
							<input class="app-input w-full font-mono text-xs" readonly value={issued.secretB64} />
							<button class="app-btn app-btn-soft shrink-0" onclick={() => copy('Secret', issued.secretB64)}><Copy size={13} /></button>
						</div>
					</div>
					{#if issued.worker_template}
						<details class="mt-3">
							<summary class="cursor-pointer text-xs font-medium">Lihat kode Cloudflare Worker</summary>
							<textarea class="app-input mt-2 w-full font-mono text-[11px]" rows="10" readonly value={issued.worker_template}></textarea>
							<button class="app-btn app-btn-soft mt-2 !min-h-7 !px-2.5 !text-xs" onclick={() => copy('Worker', issued.worker_template)}><Copy size={12} /> Salin Worker</button>
						</details>
					{/if}
					<button class="app-btn app-btn-soft mt-2 !min-h-7 !px-2.5 !text-xs" onclick={() => (issued = null)}>Tutup</button>
				</div>
			{/if}

			<div class="app-card mt-3 p-4">
				<div class="flex items-center gap-2">
					<Server size={16} />
					<h2 class="text-sm font-semibold">Kredensial Crawler</h2>
				</div>
				{#if credential}
					<dl class="mt-3 grid grid-cols-2 gap-2 text-sm">
						<div><dt class="text-xs text-muted-foreground">Site ID</dt><dd class="font-mono text-xs">{credential.crawler_id}</dd></div>
						<div><dt class="text-xs text-muted-foreground">Status</dt><dd>{credential.status}</dd></div>
						<div><dt class="text-xs text-muted-foreground">Domain</dt><dd>{credential.domain}</dd></div>
						<div><dt class="text-xs text-muted-foreground">Fingerprint</dt><dd class="font-mono text-xs">{credential.secret_fingerprint}</dd></div>
						<div><dt class="text-xs text-muted-foreground">Dibuat</dt><dd class="text-xs">{dt(credential.created_at)}</dd></div>
						<div><dt class="text-xs text-muted-foreground">Rotasi terakhir</dt><dd class="text-xs">{dt(credential.rotated_at)}</dd></div>
					</dl>
					<div class="mt-3 flex flex-wrap gap-2">
						<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" disabled={busy} onclick={() => action('rotate')}><RefreshCw size={13} /> Rotasi Secret</button>
						{#if credential.status === 'active' || credential.status === 'rotating'}
							<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" disabled={busy} onclick={() => action('disable')}>Nonaktifkan</button>
						{/if}
						<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" disabled={busy} onclick={() => action('revoke')}><Trash2 size={13} /> Cabut</button>
					</div>
				{:else}
					<div class="mt-3">
						<label class="flex items-start gap-2 text-sm">
							<input class="mt-1" type="checkbox" bind:checked={consent} />
							<span> Saya menyetujui dokumen akses crawler: Tauthub hanya mengakses halaman publik website, tidak pernah panel admin/akun, dan dapat dinonaktifkan kapan saja.</span>
						</label>
						<button class="app-btn app-btn-primary mt-2" disabled={busy || !consent} onclick={() => action('issue')}>
							<ShieldCheck size={15} /> Buat Kredensial
						</button>
					</div>
				{/if}

				{#if diagnostics.length > 0}
					<h3 class="mt-4 text-sm font-semibold">Diagnosis Terakhir</h3>
					<ul class="mt-2 space-y-1.5 text-xs">
						{#each diagnostics as d (d.id)}
							<li class="flex items-center justify-between gap-2">
								<span>{verdictLabel[d.verdict] ?? d.verdict} · HTTP {d.http_status ?? '—'}</span>
								<span class="text-muted-foreground">{dt(d.created_at)}</span>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		{/if}
	{/if}
</section>
