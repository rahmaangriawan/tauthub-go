<script lang="ts">
	// Content Standards — kuesioner readiness & pedoman editorial AI (15 standar
	// penulisan) per jenis konten.
	// Port apps/web/src/app/admin/content-standards/page.tsx.
	import { onMount } from 'svelte';
	import { ClipboardList, Plus, Sparkle, Trash } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { Search } from '@lucide/svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import EditorialDrawer from './editorial-drawer.svelte';

	const PAGE_SIZE = 20;
	const QUESTION_TYPES = [
		'text',
		'textarea',
		'number',
		'date',
		'url',
		'select',
		'multi_select',
		'radio',
		'checkbox'
	];

	type Question = {
		key: string;
		label: string;
		type: string;
		required: boolean;
		weight: number;
		helper?: string | null;
		placeholder?: string | null;
		options?: string[];
	};

	type IntakeType = {
		id: string;
		code: string;
		name: string;
		description: string | null;
		questions: Question[];
		status: string;
		standard_version?: number | null;
		standard_status?: string | null;
		standard_chunk_count?: number;
		created_at: string;
		updated_at: string;
	};

	function emptyQuestion(): Question {
		return { key: '', label: '', type: 'text', required: true, weight: 0, helper: '', placeholder: '', options: [] };
	}

	let rows = $state<IntakeType[]>([]);
	let total = $state(0);
	let error = $state('');
	let loading = $state(true);
	let createOpen = $state(false);
	let editTarget = $state<IntakeType | null>(null);
	let editorialTarget = $state<IntakeType | null>(null);
	let busy = $state(false);
	let form = $state({ code: '', name: '', description: '' });
	let questions = $state<Question[]>([emptyQuestion()]);
	let search = $state('');
	let debouncedSearch = $state('');
	let page = $state(1);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load(1);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	let searchTimer: ReturnType<typeof setTimeout> | null = null;
	$effect(() => {
		const value = search.trim();
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => {
			debouncedSearch = value;
			page = 1;
			void load(1);
		}, 300);
		return () => {
			if (searchTimer) clearTimeout(searchTimer);
		};
	});

	async function load(pageNumber = page) {
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams({
				limit: String(PAGE_SIZE),
				offset: String((pageNumber - 1) * PAGE_SIZE)
			});
			if (debouncedSearch) params.set('q', debouncedSearch);
			const res = await apiRequest(`/v1/platform/content-intake-types?${params}`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const data = await res.json();
			rows = data.items ?? [];
			total = data.total ?? 0;
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	function setQuestion(index: number, patch: Partial<Question>) {
		questions = questions.map((q, i) => (i === index ? { ...q, ...patch } : q));
	}

	function openCreate() {
		form = { code: '', name: '', description: '' };
		questions = [emptyQuestion()];
		createOpen = true;
	}

	function openEdit(target: IntakeType) {
		editTarget = target;
		form = { code: target.code, name: target.name, description: target.description ?? '' };
		questions =
			target.questions.length > 0
				? target.questions.map((q) => ({
						...q,
						helper: q.helper ?? '',
						placeholder: q.placeholder ?? '',
						options: q.options ?? []
					}))
				: [emptyQuestion()];
	}

	function buildPayload() {
		return {
			code: form.code.trim(),
			name: form.name.trim(),
			description: form.description.trim() || null,
			questions: questions
				.filter((q) => q.key.trim() && q.label.trim())
				.map((q) => ({
					key: q.key
						.trim()
						.toLowerCase()
						.replace(/[^a-z0-9_]+/g, '_'),
					label: q.label.trim(),
					type: q.type,
					required: q.required,
					weight: Number(q.weight) || 0,
					helper: q.helper?.trim() || null,
					placeholder: q.placeholder?.trim() || null,
					options:
						q.type === 'select' || q.type === 'multi_select' || q.type === 'radio' || q.type === 'checkbox'
							? (q.options ?? []).map((o) => o.trim()).filter(Boolean)
							: undefined
				}))
		};
	}

	async function create(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		error = '';
		try {
			const res = await apiRequest('/v1/platform/content-intake-types', {
				method: 'POST',
				body: JSON.stringify(buildPayload())
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			createOpen = false;
			await load(page);
		} finally {
			busy = false;
		}
	}

	async function saveEdit(event: SubmitEvent) {
		event.preventDefault();
		if (!editTarget) return;
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/content-intake-types/${editTarget.code}`, {
				method: 'PUT',
				body: JSON.stringify(buildPayload())
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			editTarget = null;
			await load(page);
		} finally {
			busy = false;
		}
	}

	async function setStatus(target: IntakeType, status: string) {
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/content-intake-types/${target.code}`, {
				method: 'PUT',
				body: JSON.stringify({ status })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			await load(page);
		} catch {
			error = 'Permintaan gagal.';
		}
	}

	const totalWeight = (q: Question[]) => q.reduce((sum, item) => sum + (Number(item.weight) || 0), 0);
</script>

{#snippet intakeTypeCell(row: IntakeType)}
	<div class="flex items-center gap-2">
		<ClipboardList size={16} />
		<div>
			<p class="font-medium">{row.name}</p>
			<p class="text-xs text-muted-foreground">{row.description || '—'}</p>
		</div>
	</div>
{/snippet}

{#snippet codeCell(row: IntakeType)}
	<code class="text-xs">{row.code}</code>
{/snippet}

{#snippet standardAiCell(row: IntakeType)}
	<div class="flex items-center gap-2">
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			style={row.standard_version ? 'color: var(--primary);' : undefined}
			title="Buka detail 15 pedoman editorial AI untuk jenis konten ini"
			onclick={() => (editorialTarget = row)}
		>
			<Sparkle size={13} />
			{row.standard_version ? `Standar AI (v${row.standard_version})` : '+ Standar AI'}
		</button>
		{#if row.standard_chunk_count && row.standard_chunk_count > 0}
			<span class="text-xs text-muted-foreground">{row.standard_chunk_count} chunks</span>
		{/if}
	</div>
{/snippet}

{#snippet questionsCell(row: IntakeType)}
	<span>{row.questions.length} pertanyaan</span>
{/snippet}

{#snippet weightCell(row: IntakeType)}
	<Badge tone={totalWeight(row.questions) === 100 ? 'success' : 'warning'} label={`${totalWeight(row.questions)}/100`} />
{/snippet}

{#snippet intakeStatusCell(row: IntakeType)}
	<Badge tone={row.status === 'active' ? 'success' : 'neutral'} value={row.status} />
{/snippet}

{#snippet intakeActions(row: IntakeType)}
	<div class="flex justify-end gap-2">
		<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" onclick={() => openEdit(row)}>Edit Kuesioner</button>
		{#if row.status === 'active'}
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--destructive);"
				onclick={() => void setStatus(row, 'archived')}>Arsipkan</button
			>
		{:else}
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--success);"
				onclick={() => void setStatus(row, 'active')}>Aktifkan</button
			>
		{/if}
	</div>
{/snippet}

{#snippet intakeTypeFields(codeDisabled: boolean)}
	<label class="block">
		<span class="text-xs font-bold text-muted-foreground">Kode (slug)</span>
		<input
			class="app-input mt-1 w-full"
			required
			disabled={codeDisabled}
			placeholder="press_release"
			bind:value={form.code}
		/>
	</label>
	<label class="block">
		<span class="text-xs font-bold text-muted-foreground">Nama</span>
		<input class="app-input mt-1 w-full" required placeholder="Siaran Pers Umum" bind:value={form.name} />
	</label>
	<label class="block">
		<span class="text-xs font-bold text-muted-foreground">Deskripsi</span>
		<input
			class="app-input mt-1 w-full"
			placeholder="Kapan jenis ini dipakai klien…"
			bind:value={form.description}
		/>
	</label>
{/snippet}

{#snippet questionEditor()}
	<div>
		<p class="mb-1 text-xs font-bold text-muted-foreground">
			Pertanyaan kuesioner
			<Badge tone={totalWeight(questions) === 100 ? 'success' : 'warning'} label={`${totalWeight(questions)}/100`} />
		</p>
		<div class="space-y-2">
			{#each questions as q, index (index)}
				<div class="app-card p-3">
					<div class="flex flex-col gap-2">
						<div class="flex flex-wrap items-center gap-2">
							<input
								class="app-input flex-[2] min-w-[120px]"
								placeholder="key (topic)"
								value={q.key}
								oninput={(e) => setQuestion(index, { key: e.currentTarget.value })}
							/>
							<Select
								ariaLabel="Tipe pertanyaan"
								minWidth={160}
								value={q.type}
								onchange={(v) => setQuestion(index, { type: v })}
								options={QUESTION_TYPES.map((t) => ({ value: t, label: t }))}
							/>
							<label class="flex items-center gap-1.5 whitespace-nowrap text-xs">
								<input
									type="checkbox"
									checked={q.required}
									onchange={(e) => setQuestion(index, { required: e.currentTarget.checked })}
								/>
								Wajib
							</label>
							<input
								class="app-input"
								type="number"
								min="0"
								max="100"
								style="width: 72px;"
								placeholder="bobot"
								value={q.weight === 0 ? '' : String(q.weight)}
								oninput={(e) => setQuestion(index, { weight: Number(e.currentTarget.value) })}
							/>
							<button
								type="button"
								class="app-btn app-btn-soft !min-h-8 !px-2.5 !text-xs"
								style="color: var(--destructive);"
								aria-label="Hapus pertanyaan"
								onclick={() => (questions = questions.filter((_, i) => i !== index))}
							>
								<Trash size={14} />
							</button>
						</div>
						<input
							class="app-input w-full"
							placeholder="Label pertanyaan (tampil ke klien)"
							value={q.label}
							oninput={(e) => setQuestion(index, { label: e.currentTarget.value })}
						/>
						<div class="flex flex-col gap-2 sm:flex-row">
							<input
								class="app-input flex-1"
								placeholder="Helper text (opsional)"
								value={q.helper ?? ''}
								oninput={(e) => setQuestion(index, { helper: e.currentTarget.value })}
							/>
							<input
								class="app-input flex-1"
								placeholder="Placeholder (opsional)"
								value={q.placeholder ?? ''}
								oninput={(e) => setQuestion(index, { placeholder: e.currentTarget.value })}
							/>
						</div>
						{#if ['select', 'multi_select', 'radio', 'checkbox'].includes(q.type)}
							<input
								class="app-input w-full"
								placeholder="Opsi, pisahkan dengan koma (mis. A, B, C)"
								value={(q.options ?? []).join(', ')}
								oninput={(e) =>
									setQuestion(index, {
										options: e.currentTarget.value
											.split(',')
											.map((o) => o.trim())
											.filter(Boolean)
									})}
							/>
						{/if}
					</div>
				</div>
			{/each}
		</div>
		<button
			type="button"
			class="app-btn app-btn-soft mt-2 !min-h-8 !px-3 !text-xs"
			onclick={() => (questions = [...questions, emptyQuestion()])}
		>
			<Plus size={14} /> Tambah pertanyaan
		</button>
	</div>
{/snippet}

<section class="mx-auto max-w-7xl">
	<PageHeader
		title="Content Standards"
		description="Kuesioner readiness & pedoman editorial AI (15 standar penulisan) per jenis konten — diindeks sebagai RAG knowledge engine Tauthub."
	>
		<Badge tone="accent" value={`${total} jenis`} />
		<button class="app-btn app-btn-primary" onclick={openCreate}><Plus size={16} />Buat jenis konten</button>
	</PageHeader>

	<Feedback {loading} {error} />

	<div class="mt-4 flex flex-wrap items-center gap-2">
		<label class="app-input flex min-w-[240px] flex-1 items-center gap-2 !py-1.5">
			<Search size={15} class="text-muted-foreground" />
			<input
				class="w-full bg-transparent text-sm outline-none"
				placeholder="Cari nama, kode, atau deskripsi…"
				aria-label="Cari jenis konten"
				bind:value={search}
			/>
		</label>
		{#if debouncedSearch}
			<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" onclick={() => (search = '')}
				>Bersihkan pencarian</button
			>
		{/if}
	</div>

	<DataTable
		columns={[
			{ key: 'name', label: 'Jenis konten', width: '26%', render: intakeTypeCell },
			{ key: 'code', label: 'Kode', width: '12%', render: codeCell },
			{ key: 'standard', label: 'Standar AI (Editorial RAG)', width: '22%', render: standardAiCell },
			{ key: 'questions', label: 'Kuesioner', width: '10%', render: questionsCell },
			{ key: 'weight', label: 'Bobot', width: '10%', render: weightCell },
			{ key: 'status', label: 'Status', width: '8%', render: intakeStatusCell },
			{ key: 'actions', label: 'Aksi', width: '12%', align: 'right' as const, render: intakeActions }
		]}
		rows={rows}
		rowKey={(row: IntakeType) => row.id}
		{loading}
		empty="Belum ada jenis konten. Buat yang pertama."
		minWidth="1000px"
	/>

	<Pagination
		total={total}
		limit={PAGE_SIZE}
		offset={(page - 1) * PAGE_SIZE}
		onChange={(next) => {
			page = next / PAGE_SIZE + 1;
			void load(page);
		}}
	/>
</section>

<EditorialDrawer
	contentTypeCode={editorialTarget?.code ?? null}
	contentTypeName={editorialTarget?.name ?? ''}
	headerBadge={editorialTarget?.standard_version ? `Standar AI (v${editorialTarget.standard_version})` : undefined}
	open={editorialTarget !== null}
	onClose={() => (editorialTarget = null)}
	onSaved={() => void load(page)}
/>

{#if createOpen}
	<Modal title="Buat jenis konten" maxWidth="2xl" onClose={() => (createOpen = false)}>
		<p class="mb-3 text-sm text-muted-foreground">
			Definisikan kuesioner &amp; bobot readiness. Total bobot disarankan 100.
		</p>
		<form class="space-y-3" onsubmit={create}>
			{@render intakeTypeFields(false)}
			{@render questionEditor()}
			{#if error}
				<p class="text-sm" style="color: var(--destructive);">{error}</p>
			{/if}
			<div class="flex justify-end gap-2 border-t pt-4">
				<button type="button" class="app-btn app-btn-soft" onclick={() => (createOpen = false)}>Batal</button>
				<button type="submit" class="app-btn app-btn-primary" disabled={busy}
					>{busy ? 'Menyimpan…' : 'Buat jenis'}</button
				>
			</div>
		</form>
	</Modal>
{/if}

{#if editTarget}
	<Modal title={`Edit ${editTarget.name}`} maxWidth="2xl" onClose={() => (editTarget = null)}>
		<form class="space-y-3" onsubmit={saveEdit}>
			{@render intakeTypeFields(true)}
			{@render questionEditor()}
			{#if error}
				<p class="text-sm" style="color: var(--destructive);">{error}</p>
			{/if}
			<div class="flex justify-end gap-2 border-t pt-4">
				<button type="button" class="app-btn app-btn-soft" onclick={() => (editTarget = null)}>Batal</button>
				<button type="submit" class="app-btn app-btn-primary" disabled={busy}
					>{busy ? 'Menyimpan…' : 'Simpan Kuesioner'}</button
				>
			</div>
		</form>
	</Modal>
{/if}
