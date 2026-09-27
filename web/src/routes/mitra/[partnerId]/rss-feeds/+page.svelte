<script lang="ts">
	// Kelola RSS feed mitra — port section "RSS Feeds" (RssFeedsSection):
	// daftar feed, salin URL, test feed.json, verifikasi pemasangan, rotasi
	// token, cabut feed, plus panduan RSS Feed & JSON Pull.
	import { onMount } from 'svelte';
	import { Check, Copy, Rss, RefreshCw, Trash2, TriangleAlert } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type Feed = {
		id: string; media_id: string | null; media_name: string | null; media_website_url: string | null;
		name: string; slug: string | null; item_status: string; install_status: string;
		install_error: string | null; installed_at: string | null; feed_path: string; feed_url: string;
		created_at: string | null;
	};
	type TestResult = { id: string; ok: boolean; status: number; items: number | null; error: string | null };

	let loading = $state(true);
	let feeds = $state<Feed[]>([]);
	let error = $state('');
	let copied = $state('');
	let verifyBusy = $state<string | null>(null);
	let testBusy = $state<string | null>(null);
	let testResult = $state<TestResult | null>(null);
	let revokeTarget = $state<Feed | null>(null);
	let busy = $state(false);

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
		const res = await apiRequest('/v1/partner/rss-feeds');
		if (res.ok) feeds = (await res.json()).feeds ?? [];
	}

	async function copy(feed: Feed) {
		try {
			await navigator.clipboard.writeText(feed.feed_url);
			copied = feed.id;
			setTimeout(() => (copied = ''), 1500);
		} catch {
			error = 'Tidak dapat menyalin ke clipboard.';
		}
	}

	/** Klien klik "Sudah dipasang" → Tauthub memverifikasi URL feed ada di website media. */
	async function verify(feed: Feed) {
		verifyBusy = feed.id;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/rss-feeds/${feed.id}/verify`, { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, 'Verifikasi gagal.');
				return;
			}
			await load();
		} finally {
			verifyBusy = null;
		}
	}

	/** Test Connection: uji endpoint JSON Pull (feed.json) — terpisah dari verifikasi pemasangan di website. */
	async function testConnection(feed: Feed) {
		testBusy = feed.id;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/rss-feeds/${feed.id}/test`, { method: 'POST' });
			const data = await res.json().catch(() => null);
			if (!res.ok) {
				error = data?.detail ?? 'Uji feed gagal.';
				return;
			}
			testResult = { id: feed.id, ok: Boolean(data?.ok), status: Number(data?.status ?? 0), items: data?.items_count ?? null, error: data?.error ?? null };
		} finally {
			testBusy = null;
		}
	}

	/** Rotasi token feed — token lama langsung mati; pemasangan perlu diulang. */
	async function rotate(feed: Feed) {
		if (!confirm('Rotasi token feed? Tautan lama langsung tidak berlaku dan pemasangan perlu diulang.')) return;
		verifyBusy = feed.id;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/rss-feeds/${feed.id}/rotate`, { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, 'Gagal merotasi token.');
				return;
			}
			await load();
		} finally {
			verifyBusy = null;
		}
	}

	/** Cabut feed — token berhenti berfungsi; baris dipertahankan untuk audit. */
	async function revoke() {
		if (!revokeTarget) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/rss-feeds/${revokeTarget.id}`, { method: 'DELETE' });
			if (!res.ok) {
				error = await readApiError(res, 'Gagal mencabut feed.');
				return;
			}
			revokeTarget = null;
			await load();
		} finally {
			busy = false;
		}
	}

	const INSTALL_LABEL: Record<string, string> = { verified: 'Terpasang', pending: 'Belum dipasang', failed: 'Gagal verifikasi' };
	const installColor = (s: string) => (s === 'verified' ? 'var(--success)' : s === 'failed' ? 'var(--destructive)' : 'var(--warning)');
	const dt = (v: string | null) => (v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—');
</script>

<section>
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">RSS Feeds</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				Pasang URL feed di website Anda, lalu klik 'Sudah dipasang' agar Tauthub memverifikasi. Setelah terverifikasi, artikel klien otomatis muncul di feed Anda.
			</p>
		</div>
		<span class="rounded-full px-2.5 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{feeds.length} feed</span>
	</header>

	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	<div class="app-card mt-3 px-4 py-2.5 text-xs text-muted-foreground">
		Token feed ini juga dipakai untuk <strong>JSON Pull</strong> (website custom): <code class="font-mono">GET /v1/publishing/feed.json?token=...</code> — artikel terstruktur (content_html + markdown + meta) untuk dirender sendiri oleh script/CI Anda.
	</div>

	<div class="app-card mt-4 overflow-x-auto">
		<table class="w-full text-sm">
			<thead>
				<tr class="text-left text-xs text-muted-foreground">
					<th class="px-4 py-2.5 font-medium">Feed</th>
					<th class="px-4 py-2.5 font-medium">Media</th>
					<th class="px-4 py-2.5 font-medium">Pemasangan</th>
					<th class="px-4 py-2.5 font-medium">Feed URL</th>
					<th class="px-4 py-2.5 font-medium"></th>
				</tr>
			</thead>
			<tbody>
				{#each feeds as row (row.id)}
					<tr class="border-t">
						<td class="px-4 py-2.5">
							<div class="flex items-center gap-2">
								<Rss size={16} />
								<div>
									<p class="font-medium">{row.name}</p>
									{#if row.media_website_url}<p class="text-xs text-muted-foreground">{row.media_website_url}</p>{/if}
								</div>
							</div>
						</td>
						<td class="px-4 py-2.5">{row.media_name ?? 'Semua media'}</td>
						<td class="px-4 py-2.5">
							<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--secondary); color: {installColor(row.install_status)};">
								{INSTALL_LABEL[row.install_status] ?? row.install_status}
							</span>
							{#if row.install_error}<p class="mt-1 text-xs" style="color: var(--destructive);">{row.install_error}</p>{/if}
							{#if row.installed_at}<p class="mt-1 text-xs text-muted-foreground">Terverifikasi {dt(row.installed_at)}</p>{/if}
						</td>
						<td class="px-4 py-2.5">
							<p class="font-mono text-xs">{row.feed_path}</p>
							<p class="mt-1 text-xs text-muted-foreground">Cara pasang: tambahkan di &lt;head&gt; situs Anda — <code class="font-mono">&lt;link rel="alternate" type="application/rss+xml" href="{row.feed_url}" /&gt;</code></p>
						</td>
						<td class="px-4 py-2.5">
							<div class="flex flex-wrap gap-2">
								<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" onclick={() => copy(row)}>
									{#if copied === row.id}<Check size={13} /> Tersalin{:else}<Copy size={13} /> Salin URL{/if}
								</button>
								<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" disabled={testBusy !== null} onclick={() => testConnection(row)}>
									<RefreshCw size={13} /> {testBusy === row.id ? 'Menguji…' : 'Test Connection'}
								</button>
								<button
									class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
									style="color: {row.install_status === 'verified' ? 'var(--success)' : 'var(--primary)'};"
									disabled={verifyBusy !== null}
									onclick={() => verify(row)}
								>
									{verifyBusy === row.id ? 'Memverifikasi…' : 'Sudah dipasang'}
								</button>
								<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" disabled={verifyBusy !== null} onclick={() => rotate(row)}><RefreshCw size={13} /> Rotasi token</button>
								<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" style="color: var(--destructive);" disabled={verifyBusy !== null} onclick={() => (revokeTarget = row)}><Trash2 size={13} /> Cabut</button>
							</div>
							{#if testResult && testResult.id === row.id}
								{#if testResult.ok}
									<p class="mt-1 text-xs" style="color: var(--success);"><Check size={12} class="inline align-[-2px]" /> Feed OK — {testResult.items ?? 0} artikel siap ditarik (HTTP {testResult.status})</p>
								{:else}
									<p class="mt-1 text-xs" style="color: var(--destructive);"><TriangleAlert size={12} class="inline align-[-2px]" /> Feed gagal — {testResult.error ?? `HTTP ${testResult.status}`}</p>
								{/if}
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
		{#if !loading && feeds.length === 0}
			<div class="flex flex-col items-center gap-2 px-4 py-12 text-center">
				<Rss size={26} style="color: var(--muted-foreground);" />
				<p class="text-sm text-muted-foreground">Belum ada RSS feed untuk partner ini.</p>
			</div>
		{/if}
	</div>

	<div class="app-card mt-4 p-4">
		<div class="mb-2 flex items-center justify-between gap-2">
			<h2 class="text-base font-semibold">Panduan — cara kerja RSS Feed &amp; JSON Pull</h2>
			<span class="rounded-full px-2 py-0.5 text-[11px] font-medium text-muted-foreground" style="background-color: var(--secondary);">Untuk mitra baru</span>
		</div>
		<div class="space-y-2 text-sm text-muted-foreground">
			<p><strong>1. Buat feed</strong> — klik <strong>+ Buat feed</strong> untuk media Anda. Tauthub membuat URL unik (contoh <code class="font-mono">https://app.tauthub.com/feed/partner/{'{token}'}</code>) beserta versi JSON (<code class="font-mono">/v1/publishing/feed.json?token={'{token}'}</code>) — satu token, dua format.</p>
			<p><strong>2. Pasang di website</strong> — salin URL feed lalu pasang <strong>tag link</strong> di bagian <code class="font-mono">&lt;head&gt;</code> website media Anda:</p>
		</div>
		<pre class="mt-3 overflow-x-auto rounded-lg border p-3 font-mono text-xs leading-relaxed" style="background-color: var(--secondary);">{`<!-- RSS biasa -->
<link rel="alternate" type="application/rss+xml" title="Nama Media" href="https://app.tauthub.com/feed/partner/TOKEN_ANDA" />

<!-- atau untuk website custom yang memakai JSON Pull -->
<link rel="alternate" type="application/json" title="Nama Media" href="https://app.tauthub.com/v1/publishing/feed.json?token=TOKEN_ANDA" />`}</pre>
		<div class="mt-3 space-y-2 text-sm text-muted-foreground">
			<p><strong>3. Verifikasi</strong> — setelah tag terpasang, klik <strong>Sudah dipasang</strong>. Tauthub memeriksa apakah URL feed (RSS atau JSON) benar-benar ada di website Anda, lalu menandai feed sebagai <strong>terverifikasi</strong>. Tanpa verifikasi, artikel klien belum didistribusikan.</p>
			<p><strong>4. Test Connection</strong> — tombol ini menarik feed.json langsung dari server Tauthub dan menampilkan jumlah artikel siap ditarik. Gunakan untuk memastikan feed sehat sebelum pemasangan.</p>
			<p><strong>5. Selesai</strong> — setiap publikasi yang disetujui otomatis muncul di feed. Website/CMS Anda (atau cron) membaca feed sesuai jadwalnya sendiri. Jika ingin distribusi langsung (push) tanpa menunggu pull, gunakan <strong>Tauthub Publishing API</strong> — lihat menu Publishing API.</p>
		</div>
	</div>

	{#if revokeTarget}
		<div class="fixed inset-0 z-50 flex items-center justify-center p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
			<dialog open class="app-card w-full max-w-md p-5" aria-label="Cabut RSS feed">
				<h2 class="text-lg font-semibold">Cabut RSS feed</h2>
				<p class="mt-2 text-sm text-muted-foreground">
					Feed <strong>{revokeTarget.name}</strong> akan dicabut — token berhenti berfungsi seketika dan artikel tidak lagi muncul di feed tersebut. Tindakan ini tidak menghapus publikasi yang sudah terbit.
				</p>
				<div class="mt-4 flex justify-end gap-2">
					<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => (revokeTarget = null)}>Batal</button>
					<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={revoke}>{busy ? 'Mencabut…' : 'Cabut feed'}</button>
				</div>
			</dialog>
		</div>
	{/if}
</section>

<style>
	/* URL feed bisa sangat panjang; di layar sempit biarkan membungkus supaya
	   tidak mendorong lebar halaman (blok <pre> tetap punya scroll sendiri). */
	code.font-mono { overflow-wrap: anywhere; }
</style>
