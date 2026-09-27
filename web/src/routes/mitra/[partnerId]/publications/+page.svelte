<script lang="ts">
	// Publikasi mitra — port section "Publikasi": daftar permintaan artikel per
	// media, filter status, pratinjau/edit draf, setujui, tolak, kirim URL,
	// dan rewrite AI (berbayar dari saldo mitra).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Check, ExternalLink, Link2, Search, Sparkles, X, XCircle } from '@lucide/svelte';
	import { getPartnerUrlId } from '$lib/partner-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const partnerId = $derived(page.params.partnerId as string);
	const base = $derived(`/mitra/${getPartnerUrlId() || partnerId}`);

	type Pub = {
		id: string; order_code: string | null; status: string; client_name: string | null;
		document_title: string | null; document_id: string; media_name: string | null;
		media_category: string | null; media_integration_type: string | null;
		media_publication_mode: string | null; client_price: number; partner_amount: number;
		publish_status: string | null; scheduled_at: string | null; published_at: string | null;
		published_url: string | null; rejection_reason: string | null; notes: string | null;
		verification_status: string | null; verification_compliance_score: number | null;
		pr_sentiment: string | null; pr_message_score: number | null;
		thumbnail_url: string | null; partner_draft_edits: Record<string, unknown> | null;
		created_at: string;
	};

	let items = $state<Pub[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');
	let statusFilter = $state('');
	let q = $state('');

	let selected = $state<Pub | null>(null);
	let detail = $state<any>(null);
	let draft = $state<any>(null);
	let rejectReason = $state('');
	let publishedUrl = $state('');
	let showReject = $state(false);
	let showURL = $state(false);

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
		const params = new URLSearchParams({ limit: '100' });
		if (statusFilter) params.set('status', statusFilter);
		const res = await apiRequest(`/v1/partner/publications?${params}`);
		if (res.ok) {
			const data = await res.json();
			items = data.publications ?? [];
			total = data.total ?? 0;
		}
	}

	const filtered = $derived(
		items.filter((p) => {
			if (!q.trim()) return true;
			const term = q.trim().toLowerCase();
			return (
				(p.document_title ?? '').toLowerCase().includes(term) ||
				(p.media_name ?? '').toLowerCase().includes(term) ||
				(p.order_code ?? '').toLowerCase().includes(term)
			);
		})
	);

	async function openDetail(p: Pub) {
		selected = p;
		detail = null;
		draft = null;
		publishedUrl = p.published_url ?? '';
		rejectReason = '';
		const [dRes, drRes] = await Promise.all([
			apiRequest(`/v1/partner/publications/${p.id}`),
			apiRequest(`/v1/partner/publications/${p.id}/draft`)
		]);
		if (dRes.ok) detail = await dRes.json();
		if (drRes.ok) draft = await drRes.json();
		else draft = { error: await readApiError(drRes, "Permintaan gagal.") };
	}

	async function closeDetail() {
		selected = null;
		detail = null;
		draft = null;
		showReject = false;
		showURL = false;
	}

	async function approve() {
		if (!selected) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/publications/${selected.id}/approve`, {
				method: 'POST',
				body: JSON.stringify({})
			});
			if (!res.ok) {
				error = await readApiError(res, "Permintaan gagal.");
				return;
			}
			const updated = await res.json();
			notice = 'Artikel disetujui.';
			await load();
			selected = updated;
			if (updated.status === 'partner_approved' && updated.media_publication_mode !== 'publish') {
				showURL = true;
			}
		} finally {
			busy = false;
		}
	}

	async function reject() {
		if (!selected) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/publications/${selected.id}/reject`, {
				method: 'POST',
				body: JSON.stringify({ reason: rejectReason.trim() })
			});
			if (!res.ok) {
				error = await readApiError(res, "Permintaan gagal.");
				return;
			}
			notice = 'Artikel ditolak.';
			showReject = false;
			await load();
			await closeDetail();
		} finally {
			busy = false;
		}
	}

	async function submitURL() {
		if (!selected) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/publications/${selected.id}/url`, {
				method: 'POST',
				body: JSON.stringify({ published_url: publishedUrl.trim() })
			});
			if (!res.ok) {
				error = await readApiError(res, "Permintaan gagal.");
				return;
			}
			notice = 'URL artikel tersimpan — artikel ditandai terbit.';
			showURL = false;
			await load();
			await openDetail(await res.json());
		} finally {
			busy = false;
		}
	}

	async function saveDraft(e: SubmitEvent) {
		e.preventDefault();
		if (!selected || !draft) return;
		busy = true;
		error = '';
		try {
			const payload: Record<string, unknown> = {
				title: draft.title,
				content_html: draft.content_html,
				meta_description: draft.meta_description ?? '',
				featured_image_url: draft.featured_image_url || null
			};
			const res = await apiRequest(`/v1/partner/publications/${selected.id}/draft`, {
				method: 'PUT',
				body: JSON.stringify(payload)
			});
			if (!res.ok) {
				error = await readApiError(res, "Permintaan gagal.");
				return;
			}
			notice = 'Perubahan draf tersimpan.';
			await load();
		} finally {
			busy = false;
		}
	}

	async function rewrite() {
		if (!selected || !draft) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/partner/publications/${selected.id}/rewrite`, {
				method: 'POST',
				body: JSON.stringify({ title: draft.title, content_html: draft.content_html })
			});
			const data = await res.json();
			if (!res.ok) {
				error = data?.detail ?? 'Rewrite gagal.';
				return;
			}
			draft.title = data.title;
			draft.content_html = data.content_html;
			notice = `Rewrite selesai (biaya Rp ${(data.amount_charged_idr ?? 0).toLocaleString('id-ID')}). Tinjau lalu simpan draf.`;
		} finally {
			busy = false;
		}
	}

	const statusLabel: Record<string, string> = {
		waiting_partner: 'Menunggu persetujuan', partner_approved: 'Disetujui', partner_rejected: 'Ditolak',
		queued: 'Antrean', processing: 'Diproses', published: 'Terbit', failed: 'Gagal',
		pending: 'Menunggu', cancelled: 'Dibatalkan', retrying: 'Coba ulang'
	};
	const statusTone: Record<string, string> = {
		waiting_partner: 'warning', partner_approved: 'accent', partner_rejected: 'destructive',
		published: 'success', failed: 'destructive', processing: 'accent', queued: 'accent'
	};
	const rp = (n: number) => `Rp ${(n ?? 0).toLocaleString('id-ID')}`;
	const dt = (v: string | null) => (v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—');
</script>

<section class="mx-auto max-w-6xl">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Publikasi</h1>
		<p class="mt-1 text-sm text-muted-foreground">{total} permintaan artikel pada media Anda.</p>
	</header>

	{#if notice}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}
	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	<form class="app-card mt-4 flex flex-wrap items-center gap-2 p-3" onsubmit={(e) => e.preventDefault()}>
		<label class="app-input flex min-w-[220px] flex-1 items-center gap-2 !py-1.5">
			<Search size={15} class="text-muted-foreground" />
			<input class="w-full bg-transparent text-sm outline-none" placeholder="Cari judul, media, atau nomor order…" bind:value={q} />
		</label>
		<select class="app-input !w-auto" bind:value={statusFilter} onchange={load}>
			<option value="">Semua status aktif</option>
			<option value="waiting_partner">Menunggu persetujuan</option>
			<option value="partner_approved">Disetujui</option>
			<option value="partner_rejected">Ditolak</option>
			<option value="queued">Antrean</option>
			<option value="processing">Diproses</option>
			<option value="published">Terbit</option>
			<option value="failed">Gagal</option>
		</select>
	</form>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat publikasi…</p>
	{:else if filtered.length === 0}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">
				{q || statusFilter ? 'Tidak ada publikasi yang cocok dengan filter ini.' : 'Belum ada permintaan publikasi.'}
			</p>
		</div>
	{:else}
		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">Artikel</th>
						<th class="px-4 py-2.5 font-medium">Klien</th>
						<th class="px-4 py-2.5 font-medium">Media</th>
						<th class="px-4 py-2.5 text-right font-medium">Penghasilan</th>
						<th class="px-4 py-2.5 font-medium">Status</th>
						<th class="px-4 py-2.5 font-medium">Dibuat</th>
					</tr>
				</thead>
				<tbody>
					{#each filtered as p (p.id)}
						<tr class="cursor-pointer border-t hover:bg-secondary" onclick={() => openDetail(p)}>
							<td class="max-w-[300px] px-4 py-2.5">
								<p class="truncate font-medium">{p.document_title ?? 'Tanpa judul'}</p>
								{#if p.order_code}<p class="text-xs text-muted-foreground">{p.order_code}</p>{/if}
							</td>
							<td class="px-4 py-2.5 text-muted-foreground">{p.client_name ?? '—'}</td>
							<td class="px-4 py-2.5">
								<p>{p.media_name ?? '—'}</p>
								<p class="text-xs text-muted-foreground">
									{p.media_category ?? '—'}{p.media_publication_mode === 'publish' ? ' · otomatis' : p.media_publication_mode === 'draft' ? ' · manual' : ''}
								</p>
							</td>
							<td class="px-4 py-2.5 text-right tabular-nums">{rp(p.partner_amount)}</td>
							<td class="px-4 py-2.5">
								<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--{statusTone[p.status] ?? 'accent'}-soft, var(--accent)); color: var(--{statusTone[p.status] ?? 'accent'});">
									{statusLabel[p.status] ?? p.status}
								</span>
							</td>
							<td class="px-4 py-2.5 text-xs text-muted-foreground">{dt(p.created_at)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</section>

{#if selected}
	<div class="fixed inset-0 z-50 flex items-start justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card my-6 w-full max-w-3xl p-5" aria-label="Detail publikasi">
			<div class="flex items-start justify-between gap-3">
				<div class="min-w-0">
					<h2 class="truncate text-lg font-semibold">{selected.document_title ?? 'Detail Publikasi'}</h2>
					<p class="mt-0.5 text-xs text-muted-foreground">
						{selected.media_name} · {statusLabel[selected.status] ?? selected.status}
						{#if selected.order_code} · {selected.order_code}{/if}
					</p>
				</div>
				<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label="Tutup detail" onclick={closeDetail}><X size={18} /></button>
			</div>

			{#if error}<p class="mt-3 text-sm" style="color: var(--destructive);">{error}</p>{/if}

			<dl class="mt-4 grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
				<div><dt class="text-xs text-muted-foreground">Penghasilan mitra</dt><dd class="font-semibold tabular-nums">{rp(selected.partner_amount)}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Harga klien</dt><dd class="tabular-nums">{rp(selected.client_price)}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Mode media</dt><dd>{selected.media_publication_mode === 'publish' ? 'Otomatis' : 'Manual'}</dd></div>
				<div><dt class="text-xs text-muted-foreground">Status kirim</dt><dd>{selected.publish_status ?? '—'}</dd></div>
			</dl>

			{#if selected.rejection_reason}
				<p class="mt-3 rounded-lg px-3 py-2 text-sm" style="background-color: var(--destructive-soft, #fdecee); color: var(--destructive);">
					Ditolak: {selected.rejection_reason}
				</p>
			{/if}

			<!-- Tombol aksi sesuai status -->
			<div class="mt-4 flex flex-wrap gap-2 border-y py-3">
				{#if selected.status === 'waiting_partner'}
					<button class="app-btn app-btn-primary" disabled={busy} onclick={approve}><Check size={15} /> Setujui</button>
					<button class="app-btn app-btn-soft" disabled={busy} onclick={() => (showReject = !showReject)}><XCircle size={15} /> Tolak</button>
				{/if}
				{#if selected.status === 'partner_approved'}
					<button class="app-btn app-btn-primary" disabled={busy} onclick={() => (showURL = !showURL)}><Link2 size={15} /> Kirim URL Artikel</button>
				{/if}
				{#if selected.status === 'published'}
					<button class="app-btn app-btn-soft" disabled={busy} onclick={() => (showURL = !showURL)}><Link2 size={15} /> Koreksi URL</button>
					{#if selected.published_url}
						<a class="app-btn app-btn-soft" href={selected.published_url} target="_blank" rel="noreferrer"><ExternalLink size={15} /> Buka Artikel</a>
					{/if}
				{/if}
			</div>

			{#if showReject}
				<div class="mt-3 rounded-lg border p-3">
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">Alasan penolakan (min. 5 karakter)</span>
						<textarea class="app-input mt-1 w-full" rows="2" bind:value={rejectReason}></textarea>
					</label>
					<div class="mt-2 flex justify-end gap-2">
						<button class="app-btn app-btn-soft" onclick={() => (showReject = false)}>Batal</button>
						<button class="app-btn app-btn-primary" disabled={busy || rejectReason.trim().length < 5} onclick={reject}>Kirim Penolakan</button>
					</div>
				</div>
			{/if}

			{#if showURL}
				<div class="mt-3 rounded-lg border p-3">
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">URL artikel yang sudah terbit</span>
						<input class="app-input mt-1 w-full" placeholder="https://media.example/artikel/…" bind:value={publishedUrl} />
					</label>
					<div class="mt-2 flex justify-end gap-2">
						<button class="app-btn app-btn-soft" onclick={() => (showURL = false)}>Batal</button>
						<button class="app-btn app-btn-primary" disabled={busy || !publishedUrl.trim()} onclick={submitURL}>Simpan URL</button>
					</div>
					<p class="mt-2 text-xs text-muted-foreground">
						Pada mode manual, mengirim URL adalah penanda artikel sudah terbit.
					</p>
				</div>
			{/if}

			<!-- Pratinjau / edit draf -->
			<div class="mt-4">
				<div class="flex items-center justify-between">
					<h3 class="text-sm font-semibold">Isi Artikel</h3>
					{#if draft?.error}
						<span class="text-xs" style="color: var(--destructive);">{draft.error}</span>
					{:else if draft}
						<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" disabled={busy} onclick={rewrite}>
							<Sparkles size={13} /> Rewrite AI (Rp 1.000)
						</button>
					{/if}
				</div>

				{#if draft && !draft.error}
					<form class="mt-2 space-y-2" onsubmit={saveDraft}>
						<label class="block">
							<span class="text-xs font-medium text-muted-foreground">Judul</span>
							<input class="app-input mt-1 w-full" maxlength="200" bind:value={draft.title} />
						</label>
						<label class="block">
							<span class="text-xs font-medium text-muted-foreground">Meta Deskripsi</span>
							<input class="app-input mt-1 w-full" maxlength="500" bind:value={draft.meta_description} />
						</label>
						<label class="block">
							<span class="text-xs font-medium text-muted-foreground">Gambar Utama (URL)</span>
							<input class="app-input mt-1 w-full" bind:value={draft.featured_image_url} />
						</label>
						<label class="block">
							<span class="text-xs font-medium text-muted-foreground">Isi Artikel (HTML)</span>
							<textarea class="app-input mt-1 w-full font-mono text-xs" rows="10" bind:value={draft.content_html}></textarea>
						</label>
						<div class="flex items-center justify-between">
							<p class="text-xs text-muted-foreground">
								Sumber: {draft.source === 'wp_draft' ? 'draf WordPress' : 'dokumen klien'}
								{#if draft.edited_at} · diedit {dt(draft.edited_at)}{/if}
							</p>
							<div class="flex gap-2">
								<button class="app-btn app-btn-primary" type="submit" disabled={busy}>Simpan Draf</button>
							</div>
						</div>
					</form>
				{/if}
			</div>

			{#if detail?.attempts?.length}
				<div class="mt-4">
					<h3 class="text-sm font-semibold">Riwayat Pengiriman</h3>
					<ul class="mt-2 space-y-1.5 text-xs">
						{#each detail.attempts as a (a.attempt_number)}
							<li class="flex items-center justify-between gap-2">
								<span>Percobaan {a.attempt_number} · {a.status}{#if a.error_code} [{a.error_code}]{/if}</span>
								<span class="text-muted-foreground">{dt(a.completed_at ?? a.started_at)}</span>
							</li>
						{/each}
					</ul>
				</div>
			{/if}
		</dialog>
	</div>
{/if}
