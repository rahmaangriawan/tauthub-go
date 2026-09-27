<script lang="ts">
	// Media Standards — profil editorial per media (panjang artikel, gaya jurnalistik,
	// kutipan, hard-selling, forbidden content). Final check dijalankan setelah klien
	// memilih media, sebelum publish.
	// Port apps/web/src/app/admin/media-standards/page.tsx.
	import { onMount } from 'svelte';
	import { Newspaper, Plus } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const PAGE_SIZE = 20;

	type Standard = {
		id: string;
		media_id: string;
		media_name: string | null;
		min_words: number;
		max_words: number;
		journalistic_style: string;
		hard_selling_tolerance: string;
		quote_required: boolean;
		min_quotes: number;
		image_required: boolean;
		category_required: boolean;
		required_info: string[];
		forbidden_content: string[];
		status: string;
	};
	type Media = { id: string; name: string; status: string };

	function emptyForm(mediaId: string) {
		return {
			media_id: mediaId,
			min_words: '400',
			max_words: '1500',
			journalistic_style: 'news',
			hard_selling_tolerance: 'low',
			quote_required: false,
			min_quotes: '0',
			image_required: false,
			category_required: false,
			required_info: '',
			forbidden_content: ''
		};
	}

	let rows = $state<Standard[]>([]);
	let mediaOptions = $state<Media[]>([]);
	let mediaSearch = $state('');
	let error = $state('');
	let loading = $state(true);
	let dialogOpen = $state(false);
	let editing = $state<Standard | null>(null);
	let busy = $state(false);
	let form = $state(emptyForm(''));
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
			const [standardRes, mediaRes] = await Promise.all([
				apiRequest('/v1/platform/media-standards'),
				apiRequest('/v1/platform/media?limit=500')
			]);
			if (!standardRes.ok) {
				error = await readApiError(standardRes, 'Permintaan gagal.');
				return;
			}
			rows = await standardRes.json();
			if (mediaRes.ok) {
				const mediaData = await mediaRes.json();
				const media: Media[] = mediaData.media ?? [];
				mediaOptions = media.filter((m) => m.status !== 'archived');
				form = form.media_id ? form : emptyForm(media[0]?.id ?? '');
			}
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	// Pencarian media sisi server (debounced) — cari seluruh katalog media.
	let mediaSearchTimer: ReturnType<typeof setTimeout> | null = null;
	$effect(() => {
		const q = mediaSearch.trim();
		if (mediaSearchTimer) clearTimeout(mediaSearchTimer);
		if (!q) return;
		mediaSearchTimer = setTimeout(async () => {
			const res = await apiRequest(`/v1/platform/media?search=${encodeURIComponent(q)}&limit=100`);
			if (!res.ok) return;
			const data = await res.json();
			mediaOptions = (data.media ?? []).filter((m: Media) => m.status !== 'archived');
		}, 300);
		return () => {
			if (mediaSearchTimer) clearTimeout(mediaSearchTimer);
		};
	});

	function openCreate() {
		editing = null;
		mediaSearch = '';
		const withStandard = new Set(rows.map((r) => r.media_id));
		const firstFree = mediaOptions.find((m) => !withStandard.has(m.id)) ?? mediaOptions[0];
		form = emptyForm(firstFree?.id ?? '');
		dialogOpen = true;
	}

	function openEdit(target: Standard) {
		editing = target;
		mediaSearch = '';
		form = {
			media_id: target.media_id,
			min_words: String(target.min_words),
			max_words: String(target.max_words),
			journalistic_style: target.journalistic_style,
			hard_selling_tolerance: target.hard_selling_tolerance,
			quote_required: target.quote_required,
			min_quotes: String(target.min_quotes),
			image_required: target.image_required,
			category_required: target.category_required,
			required_info: target.required_info.join('\n'),
			forbidden_content: target.forbidden_content.join('\n')
		};
		dialogOpen = true;
	}

	function payload() {
		return {
			media_id: form.media_id,
			min_words: Number(form.min_words) || 0,
			max_words: Number(form.max_words) || 0,
			journalistic_style: form.journalistic_style,
			hard_selling_tolerance: form.hard_selling_tolerance,
			quote_required: form.quote_required,
			min_quotes: Number(form.min_quotes) || 0,
			image_required: form.image_required,
			category_required: form.category_required,
			required_info: form.required_info
				.split('\n')
				.map((s) => s.trim())
				.filter(Boolean),
			forbidden_content: form.forbidden_content
				.split('\n')
				.map((s) => s.trim())
				.filter(Boolean)
		};
	}

	async function save(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		error = '';
		try {
			const res = editing
				? await apiRequest(`/v1/platform/media-standards/${editing.media_id}`, {
						method: 'PUT',
						body: JSON.stringify(payload())
					})
				: await apiRequest('/v1/platform/media-standards', {
						method: 'POST',
						body: JSON.stringify(payload())
					});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			dialogOpen = false;
			await load();
		} finally {
			busy = false;
		}
	}

	async function remove(target: Standard) {
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/media-standards/${target.media_id}`, { method: 'DELETE' });
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			await load();
		} catch {
			error = 'Permintaan gagal.';
		}
	}

	const withStandard = $derived(new Set(rows.map((r) => r.media_id)));
	const visibleRows = $derived(rows.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE));
	const selectOptions = $derived.by(() => {
		if (editing && !mediaOptions.some((m) => m.id === editing?.media_id)) {
			return [
				{ id: editing.media_id, name: editing.media_name ?? editing.media_id.slice(0, 8) },
				...mediaOptions
			];
		}
		return mediaOptions;
	});
</script>

{#snippet standardMediaCell(row: Standard)}
	<div class="flex items-center gap-2">
		<Newspaper size={16} />
		<span class="font-medium">{row.media_name ?? row.media_id.slice(0, 8)}</span>
	</div>
{/snippet}

{#snippet lengthCell(row: Standard)}
	<span>{row.min_words}–{row.max_words} kata</span>
{/snippet}

{#snippet styleCell(row: Standard)}
	<Badge tone="accent" label={row.journalistic_style} />
{/snippet}

{#snippet toleranceCell(row: Standard)}
	<span>{row.hard_selling_tolerance}</span>
{/snippet}

{#snippet quoteCell(row: Standard)}
	<span>{row.quote_required ? `${row.min_quotes} kutipan` : 'opsional'}</span>
{/snippet}

{#snippet rulesCell(row: Standard)}
	<div class="flex flex-wrap gap-1">
		{#if row.forbidden_content.length > 0}
			<Badge tone="danger" label={`forbidden ×${row.forbidden_content.length}`} />
		{/if}
		{#if row.required_info.length > 0}
			<Badge tone="warning" label={`info wajib ×${row.required_info.length}`} />
		{/if}
	</div>
{/snippet}

{#snippet standardStatusCell(row: Standard)}
	<Badge tone={row.status === 'active' ? 'success' : 'neutral'} value={row.status} />
{/snippet}

{#snippet standardActions(row: Standard)}
	<div class="flex gap-2">
		<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" onclick={() => openEdit(row)}>Edit</button>
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			style="color: var(--destructive);"
			onclick={() => void remove(row)}>Hapus</button
		>
	</div>
{/snippet}

<section>
	<PageHeader
		title="Media Standards"
		description="Profil editorial per media (panjang artikel, gaya jurnalistik, kutipan, hard-selling, forbidden content). Final check dijalankan setelah klien memilih media, sebelum publish."
	>
		<Badge tone="accent" value={`${rows.length} standard`} />
		<button class="app-btn app-btn-primary" onclick={openCreate}><Plus size={16} />Buat standard</button>
	</PageHeader>

	<Feedback {loading} {error} />

	<DataTable
		columns={[
			{ key: 'media', label: 'Media', render: standardMediaCell },
			{ key: 'length', label: 'Panjang', render: lengthCell },
			{ key: 'style', label: 'Gaya', render: styleCell },
			{ key: 'tolerance', label: 'Toleransi promosi', render: toleranceCell },
			{ key: 'quote', label: 'Kutipan', render: quoteCell },
			{ key: 'rules', label: 'Aturan', render: rulesCell },
			{ key: 'status', label: 'Status', render: standardStatusCell },
			{ key: 'actions', label: '', render: standardActions }
		]}
		rows={visibleRows}
		rowKey={(row: Standard) => row.id}
		{loading}
		empty="Belum ada standard media. Buat yang pertama untuk mengaktifkan final check."
		minWidth="1000px"
	/>

	<Pagination
		total={rows.length}
		limit={PAGE_SIZE}
		offset={(page - 1) * PAGE_SIZE}
		onChange={(next) => (page = next / PAGE_SIZE + 1)}
	/>
</section>

{#if dialogOpen}
	<Modal
		title={editing ? `Edit standard — ${editing.media_name ?? ''}` : 'Buat standard media'}
		maxWidth="2xl"
		onClose={() => (dialogOpen = false)}
	>
		<form class="space-y-3" onsubmit={save}>
			<label class="block">
				<span class="text-xs font-bold text-muted-foreground">Cari media</span>
				<input
					class="app-input mt-1 w-full"
					placeholder="Cari media…"
					disabled={!!editing}
					bind:value={mediaSearch}
				/>
				<span class="mt-1 block text-xs text-muted-foreground"
					>Ketik nama media — pencarian berjalan di seluruh katalog (bukan hanya 100 media pertama).</span
				>
			</label>
			<label class="block">
				<span class="text-xs font-bold text-muted-foreground">Media</span>
				<div class="mt-1">
					<Select
						ariaLabel="Media"
						minWidth={300}
						disabled={!!editing}
						value={form.media_id}
						onchange={(v) => (form.media_id = v)}
						options={selectOptions.map((m) => ({
							value: m.id,
							label: `${m.name}${withStandard.has(m.id) ? ' (sudah ada standard)' : ''}`
						}))}
					/>
				</div>
				{#if !editing}
					<span class="mt-1 block text-xs text-muted-foreground"
						>Media yang sudah punya standard tetap tampil dan ditandai — membuat standard baru untuk media yang sama
						akan menambah baris baru (bukan menimpa).</span
					>
				{/if}
			</label>
			<div class="grid gap-3 sm:grid-cols-2">
				<label class="block">
					<span class="text-xs font-bold text-muted-foreground">Min kata</span>
					<input class="app-input mt-1 w-full" type="number" min="0" bind:value={form.min_words} />
				</label>
				<label class="block">
					<span class="text-xs font-bold text-muted-foreground">Max kata</span>
					<input class="app-input mt-1 w-full" type="number" min="0" bind:value={form.max_words} />
				</label>
				<label class="block">
					<span class="text-xs font-bold text-muted-foreground">Gaya jurnalistik</span>
					<Select
						ariaLabel="Gaya jurnalistik"
						minWidth={280}
						value={form.journalistic_style}
						onchange={(v) => (form.journalistic_style = v)}
						options={[
							{ value: 'news', label: 'News — keras, 5W+1H' },
							{ value: 'feature', label: 'Feature — human interest' },
							{ value: 'magazine', label: 'Magazine — panjang & analitis' },
							{ value: 'blog', label: 'Blog — santai' }
						]}
					/>
				</label>
				<label class="block">
					<span class="text-xs font-bold text-muted-foreground">Toleransi promosi (hard-selling)</span>
					<Select
						ariaLabel="Toleransi promosi (hard-selling)"
						minWidth={240}
						value={form.hard_selling_tolerance}
						onchange={(v) => (form.hard_selling_tolerance = v)}
						options={[
							{ value: 'none', label: 'None — tanpa promosi' },
							{ value: 'low', label: 'Low' },
							{ value: 'medium', label: 'Medium' },
							{ value: 'high', label: 'High' }
						]}
					/>
				</label>
				<div>
					<span class="text-xs font-bold text-muted-foreground">Kutipan narasumber</span>
					<div class="mt-1 flex items-center gap-3">
						<input type="checkbox" bind:checked={form.quote_required} />
						<input class="app-input" type="number" min="0" style="width: 90px;" bind:value={form.min_quotes} />
					</div>
				</div>
				<div>
					<span class="text-xs font-bold text-muted-foreground">Persyaratan lainnya</span>
					<div class="mt-1 flex flex-wrap items-center gap-4">
						<label class="flex items-center gap-1.5 text-xs">
							<input type="checkbox" bind:checked={form.image_required} /> Gambar
						</label>
						<label class="flex items-center gap-1.5 text-xs">
							<input type="checkbox" bind:checked={form.category_required} /> Kategori
						</label>
					</div>
				</div>
			</div>
			<label class="block">
				<span class="text-xs font-bold text-muted-foreground">Informasi wajib (satu per baris)</span>
				<textarea
					class="app-input mt-1 w-full"
					rows="3"
					placeholder={'nama perusahaan\nlokasi\nnama produk'}
					bind:value={form.required_info}
				></textarea>
			</label>
			<label class="block">
				<span class="text-xs font-bold text-muted-foreground">Konten terlarang (satu per baris)</span>
				<textarea
					class="app-input mt-1 w-full"
					rows="3"
					placeholder={'klaim medis\ninvestasi dijamin'}
					bind:value={form.forbidden_content}
				></textarea>
			</label>
			{#if error}
				<p class="text-sm" style="color: var(--destructive);">{error}</p>
			{/if}
			<div class="flex justify-end gap-2 border-t pt-4">
				<button type="button" class="app-btn app-btn-soft" onclick={() => (dialogOpen = false)}>Batal</button>
				<button type="submit" class="app-btn app-btn-primary" disabled={busy}
					>{busy ? 'Menyimpan…' : editing ? 'Simpan' : 'Buat standard'}</button
				>
			</div>
		</form>
	</Modal>
{/if}
