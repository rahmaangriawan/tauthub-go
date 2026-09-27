<script lang="ts">
	// RSS Feeds — channel RSS pull per partner (Phase 4). Media feed wajib milik
	// partner; token = otentikasi feed publik.
	// Port apps/web/src/app/admin/rss-feeds/page.tsx.
	import { onMount } from 'svelte';
	import { Radio } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const PAGE_SIZE = 20;

	type Feed = {
		id: string;
		partner_id: string;
		media_id: string | null;
		media_name: string | null;
		name: string;
		slug: string | null;
		token: string;
		status: string;
		item_status: string;
		install_status: string;
		install_error: string | null;
		installed_at: string | null;
		feed_path: string;
		created_at: string;
	};

	const INSTALL_TONE: Record<string, 'neutral' | 'success' | 'warning' | 'danger'> = {
		verified: 'success',
		pending: 'warning',
		failed: 'danger'
	};
	const INSTALL_LABEL: Record<string, string> = {
		verified: 'Terpasang',
		pending: 'Belum dipasang',
		failed: 'Gagal'
	};
	type Partner = { id: string; name: string | null; email: string; status: string };
	type Media = { id: string; name: string; partner_id: string | null; status: string };

	let rows = $state<Feed[]>([]);
	let total = $state(0);
	let partners = $state<Partner[]>([]);
	let media = $state<Media[]>([]);
	let error = $state('');
	let loading = $state(true);
	let createOpen = $state(false);
	let busy = $state(false);
	let form = $state({ partner_id: '', media_id: '', name: '', item_status: 'published' });
	let page = $state(1);

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
		loading = true;
		error = '';
		try {
			const [feedRes, partnerRes, mediaRes] = await Promise.all([
				apiRequest(`/v1/platform/rss-feeds?limit=20&offset=${(page - 1) * PAGE_SIZE}`),
				apiRequest('/v1/platform/partners?limit=500'),
				apiRequest('/v1/platform/media?limit=500')
			]);
			if (!feedRes.ok) {
				error = await readApiError(feedRes, 'Permintaan gagal.');
				return;
			}
			const feedData = await feedRes.json();
			rows = feedData.feeds ?? [];
			total = feedData.total ?? 0;
			if (partnerRes.ok) {
				const partnerData = await partnerRes.json();
				partners = (partnerData.partners ?? []).filter((p: Partner) => p.status === 'active');
			}
			if (mediaRes.ok) {
				const mediaData = await mediaRes.json();
				media = mediaData.media ?? [];
			}
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	async function create(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		error = '';
		try {
			const res = await apiRequest('/v1/platform/rss-feeds', {
				method: 'POST',
				body: JSON.stringify({
					partner_id: form.partner_id,
					media_id: form.media_id || null,
					name: form.name.trim(),
					item_status: form.item_status
				})
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			createOpen = false;
			await load();
		} finally {
			busy = false;
		}
	}

	async function rotate(feed: Feed) {
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/rss-feeds/${feed.id}`, { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			await load();
		} catch {
			error = 'Permintaan gagal.';
		}
	}

	async function revoke(feed: Feed) {
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/rss-feeds/${feed.id}`, { method: 'DELETE' });
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			await load();
		} catch {
			error = 'Permintaan gagal.';
		}
	}

	const partnerMedia = $derived(form.partner_id ? media.filter((m) => m.partner_id === form.partner_id) : []);
</script>

{#snippet feedCell(row: Feed)}
	<div class="flex items-center gap-2">
		<Radio size={16} />
		<div>
			<p class="font-medium">{row.name}</p>
			<p class="text-xs text-muted-foreground">{row.slug ?? row.id.slice(0, 8)}</p>
		</div>
	</div>
{/snippet}

{#snippet mediaCell(row: Feed)}
	<span>{row.media_name ?? '—'}</span>
{/snippet}

{#snippet installCell(row: Feed)}
	<div>
		<Badge tone={INSTALL_TONE[row.install_status] ?? 'neutral'} label={INSTALL_LABEL[row.install_status] ?? row.install_status} />
		{#if row.install_error}
			<p class="mt-1 text-xs" style="color: var(--destructive);">{row.install_error}</p>
		{/if}
	</div>
{/snippet}

{#snippet itemStatusCell(row: Feed)}
	<Badge tone="neutral" value={row.item_status} />
{/snippet}

{#snippet feedPathCell(row: Feed)}
	<code class="text-xs">{row.feed_path}</code>
{/snippet}

{#snippet feedStatusCell(row: Feed)}
	<Badge tone={row.status === 'active' ? 'success' : 'neutral'} value={row.status} />
{/snippet}

{#snippet feedActions(row: Feed)}
	<div class="flex gap-2">
		{#if row.status === 'active'}
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--warning);"
				onclick={() => void rotate(row)}>Rotasi token</button
			>
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--destructive);"
				onclick={() => void revoke(row)}>Revoke</button
			>
		{/if}
	</div>
{/snippet}

<section class="mx-auto max-w-7xl">
	<PageHeader
		title="RSS Feeds"
		description="Channel RSS pull per partner (Phase 4). Media feed wajib milik partner; token = otentikasi feed publik."
	>
		<Badge tone="accent" value={`${total} feed`} />
		<button class="app-btn app-btn-primary" onclick={() => (createOpen = true)}><Radio size={16} />Buat feed</button>
	</PageHeader>

	<Feedback {loading} {error} />

	{#if rows.length === 0 && error.includes('Akses platform')}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke RSS feeds.</p>
		</div>
	{:else}
		<DataTable
			columns={[
				{ key: 'feed', label: 'Feed', render: feedCell },
				{ key: 'media', label: 'Media', render: mediaCell },
				{ key: 'install', label: 'Pemasangan', render: installCell },
				{ key: 'item', label: 'Item status', render: itemStatusCell },
				{ key: 'path', label: 'Feed path', render: feedPathCell },
				{ key: 'status', label: 'Status', render: feedStatusCell },
				{ key: 'actions', label: '', render: feedActions }
			]}
			rows={rows}
			rowKey={(row: Feed) => row.id}
			{loading}
			empty="Belum ada RSS feed."
			minWidth="1100px"
		/>

		<Pagination
			total={total}
			limit={PAGE_SIZE}
			offset={(page - 1) * PAGE_SIZE}
			onChange={(next) => {
				page = next / PAGE_SIZE + 1;
				void load();
			}}
		/>
	{/if}
</section>

{#if createOpen}
	<Modal title="Buat RSS feed" maxWidth="md" onClose={() => (createOpen = false)}>
		<form class="space-y-3" onsubmit={create}>
			<label class="block">
				<span class="text-xs font-bold text-muted-foreground">Partner</span>
				<select
					class="app-input mt-1 w-full"
					value={form.partner_id}
					onchange={(e) => {
						form.partner_id = e.currentTarget.value;
						form.media_id = '';
					}}
				>
					<option value="">Pilih partner</option>
					{#each partners as p (p.id)}
						<option value={p.id}>{p.name ?? p.email}</option>
					{/each}
				</select>
			</label>
			<label class="block">
				<span class="text-xs font-bold text-muted-foreground">Media partner (opsional)</span>
				<select class="app-input mt-1 w-full" bind:value={form.media_id}>
					<option value="">Semua media partner</option>
					{#each partnerMedia as m (m.id)}
						<option value={m.id}>{m.name}</option>
					{/each}
				</select>
			</label>
			<label class="block">
				<span class="text-xs font-bold text-muted-foreground">Nama feed</span>
				<input class="app-input mt-1 w-full" required placeholder="Feed MediaKita" bind:value={form.name} />
			</label>
			<label class="block">
				<span class="text-xs font-bold text-muted-foreground">Item status</span>
				<select class="app-input mt-1 w-full" bind:value={form.item_status}>
					<option value="published">published</option>
					<option value="draft">draft (perlu approve → queued)</option>
				</select>
			</label>
			{#if error}
				<p class="text-sm" style="color: var(--destructive);">{error}</p>
			{/if}
			<div class="flex justify-end gap-2 border-t pt-4">
				<button type="button" class="app-btn app-btn-soft" onclick={() => (createOpen = false)}>Batal</button>
				<button type="submit" class="app-btn app-btn-primary" disabled={busy}>{busy ? 'Menyimpan…' : 'Buat feed'}</button>
			</div>
		</form>
	</Modal>
{/if}
