<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Kotak Pengumuman (admin): tulis pengumuman broadcast ke pengguna klien
	// dan/atau mitra, lalu pantau berapa yang sudah membacanya. Endpoint:
	// GET/POST/DELETE /v1/platform/announcements (guard platform admin).
	import { onMount } from 'svelte';
	import { Megaphone, Trash2 } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
	import Select from '$lib/components/select.svelte';

	type Announcement = {
		id: string;
		title: string;
		body: string;
		audience: string;
		author_name: string | null;
		read_count: number;
		recipient_count: number;
		created_at: string | null;
	};

	let items = $state<Announcement[]>([]);
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');

	let title = $state('');
	let body = $state('');
	let audience = $state('all');
	let confirmDeleteId = $state<string | null>(null);

	const fmtDate = (v: string | null) =>
		v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—';

	const AUDIENCE_OPTIONS = $derived([
		{ value: 'all', label: tr('Semua pengguna (klien + mitra)') },
		{ value: 'client', label: tr('Hanya klien') },
		{ value: 'partner', label: tr('Hanya mitra') }
	]);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		loading = true;
		error = '';
		try {
			const res = await apiRequest('/v1/platform/announcements');
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat pengumuman.')));
			const data = await res.json();
			items = data.items ?? [];
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal memuat pengumuman.');
		} finally {
			loading = false;
		}
	}

	async function send(e: SubmitEvent) {
		e.preventDefault();
		if (busy) return;
		busy = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest('/v1/platform/announcements', {
				method: 'POST',
				body: JSON.stringify({ title: title.trim(), body: body.trim(), audience })
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal mengirim pengumuman.'));
				return;
			}
			const data = await res.json();
			const created = (data.announcement ?? data) as Announcement;
			items = [created, ...items];
			title = '';
			body = '';
			audience = 'all';
			notice = tr('Pengumuman terkirim ke kotak pengumuman pengguna.');
		} finally {
			busy = false;
		}
	}

	async function remove(item: Announcement) {
		if (confirmDeleteId !== item.id) {
			confirmDeleteId = item.id;
			return;
		}
		confirmDeleteId = null;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/announcements/${item.id}`, { method: 'DELETE' });
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menghapus pengumuman.'));
				return;
			}
			items = items.filter((i) => i.id !== item.id);
		} finally {
			busy = false;
		}
	}

	const audienceLabel: Record<string, string> = $derived({
		all: tr('Semua pengguna'),
		client: tr('Klien'),
		partner: tr('Mitra')
	});
</script>

<PageTitle title={tr("Kotak Pengumuman")} />

<section>
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">{tr("Kotak Pengumuman")}</h1>
		<p class="mt-1 text-sm text-muted-foreground">{tr("Tulis pengumuman yang tampil di kotak pengumuman seluruh pengguna klien dan mitra.")}</p>
	</header>

	{#if notice}<p class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</p>{/if}
	{#if error}<p class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</p>{/if}

	<form class="app-card mt-4 p-5" onsubmit={send}>
		<div class="flex items-center gap-3">
			<Megaphone size={18} style="color: var(--primary);" />
			<h2 class="font-semibold">{tr("Tulis Pengumuman")}</h2>
		</div>
		<div class="mt-3 grid gap-3">
			<label class="flex flex-col gap-1 text-sm">
				{tr('Judul pengumuman')}
				<input class="app-input" bind:value={title} maxlength={120} required minlength={5} placeholder={tr('mis. Maintenance sistem 30 Sep malam')} />
			</label>
			<label class="flex flex-col gap-1 text-sm">
				{tr('Isi pengumuman')}
				<textarea class="app-input" bind:value={body} rows={4} maxlength={4000} required minlength={5} placeholder={tr('Tulis isi pengumuman yang jelas dan ringkas.')}></textarea>
			</label>
			<div class="flex flex-wrap items-end gap-3">
				<div class="flex flex-col gap-1 text-sm">
					<span class="text-xs text-muted-foreground">{tr('Penerima')}</span>
					<Select bind:value={audience} ariaLabel={tr('Penerima')} minWidth={240} options={AUDIENCE_OPTIONS} />
				</div>
				<button class="app-btn app-btn-primary" type="submit" disabled={busy || title.trim().length < 5 || body.trim().length < 5}>
					{busy ? tr('Mengirim…') : tr('Kirim Pengumuman')}
				</button>
			</div>
		</div>
	</form>

	<div class="app-card mt-4 p-5">
		<h2 class="font-semibold">{tr('Pengumuman Terkirim')}</h2>
		{#if loading}
			<p class="mt-3 text-sm text-muted-foreground">{tr('Memuat…')}</p>
		{:else if items.length === 0}
			<p class="mt-3 text-sm text-muted-foreground">{tr('Belum ada pengumuman yang dikirim.')}</p>
		{:else}
			<ul class="mt-3 divide-y" style="border-color: var(--border);">
				{#each items as item (item.id)}
					{@const deleting = confirmDeleteId === item.id}
					<li class="py-3 first:pt-0 last:pb-0">
						<div class="flex flex-wrap items-start justify-between gap-2">
							<div class="min-w-0">
								<p class="text-sm font-semibold">{item.title}</p>
								<p class="mt-0.5 flex flex-wrap items-center gap-x-2 text-xs text-muted-foreground">
									<span>{fmtDate(item.created_at)}</span>
									<span>·</span>
									<span>{audienceLabel[item.audience] ?? item.audience}</span>
									{#if item.author_name}<span>· {item.author_name}</span>{/if}
								</p>
							</div>
							<div class="flex shrink-0 items-center gap-2">
								<span class="rounded-full px-2 py-0.5 text-[11px] font-semibold" style="background-color: var(--muted); color: var(--muted-foreground);">
									{tr('Dibaca')} {item.read_count}/{item.recipient_count}
								</span>
								<button
									class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs"
									type="button"
									style={deleting ? 'color: var(--destructive);' : ''}
									disabled={busy}
									onclick={() => void remove(item)}
								>
									<Trash2 size={12} />{deleting ? tr('Yakin hapus?') : tr('Hapus')}
								</button>
							</div>
						</div>
						<p class="mt-1.5 text-sm whitespace-pre-line">{item.body}</p>
					</li>
				{/each}
			</ul>
		{/if}
	</div>
</section>
