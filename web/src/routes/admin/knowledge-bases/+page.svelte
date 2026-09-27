<script lang="ts">
	// Knowledge Base — aturan editorial & gaya penulisan yang diinjeksikan ke AI
	// Engine saat content generation. Dikelola Admin, berversi, aktif/nonaktif.
	// Port apps/web/src/app/admin/knowledge-bases/page.tsx.
	import { onMount } from 'svelte';
	import { ArrowLeft, BookOpenText, Plus, Save, Search } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const PAGE_SIZE = 20;

	type Rule = {
		id: string;
		category: string;
		title: string;
		instruction: string;
		examples: string[];
		forbidden_examples: string[];
		tags: string[];
		audience: string[];
		content_types: string[];
		priority: number;
		status: string;
	};

	type Version = {
		id: string;
		knowledge_base_id: string;
		version: number;
		status: string;
		change_note: string | null;
		created_by: string;
		created_at: string;
		rule_count?: number;
	};

	type KbListItem = {
		id: string;
		code: string;
		name: string;
		description: string | null;
		scope: string;
		scope_ref: string | null;
		status: string;
		active_version: number | null;
		rule_count: number;
		created_at: string;
		updated_at: string;
	};

	type KbDetail = KbListItem & {
		versions: Version[];
		sources: Array<{
			id: string;
			source_name: string;
			source_url: string | null;
			source_type: string;
			notes: string | null;
			accessed_at: string | null;
		}>;
	};

	const PRIORITY_LABELS: Record<number, string> = {
		1: '1 · Keselamatan/Etika/Akurasi',
		2: '2 · Fakta & data user',
		3: '3 · Verifikasi & atribusi',
		4: '4 · Struktur jurnalistik',
		5: '5 · Kejelasan & keterbacaan',
		6: '6 · Gaya bahasa',
		7: '7 · SEO',
		8: '8 · Engagement/persuasi'
	};

	const CATEGORY_OPTIONS = [
		'Editorial Principles',
		'Journalistic Structure',
		'Lead',
		'Headline',
		'5W+1H',
		'Language Style',
		'Paragraph',
		'Fact & Opinion',
		'Attribution',
		'Quote',
		'Data & Numbers',
		'Press Release',
		'News Article',
		'SEO',
		'Anti Clickbait',
		'Anti Hallucination',
		'Ethics',
		'Privacy',
		'AI Governance',
		'Quality Control',
		'Corporate Language',
		'Names & Titles',
		'Foreign Terms',
		'Digital Style'
	];

	const CONTENT_TYPE_OPTIONS = [
		'press_release',
		'event',
		'csr',
		'product_launch',
		'personal_branding',
		'news',
		'article',
		'feature',
		'opinion',
		'seo_article',
		'product_content'
	];

	let rows = $state<KbListItem[]>([]);
	let total = $state(0);
	let error = $state('');
	let loading = $state(true);
	let createOpen = $state(false);
	let busy = $state(false);
	let form = $state({ code: '', name: '', description: '' });
	let detail = $state<KbDetail | null>(null);
	let detailLoading = $state(false);
	let search = $state('');
	let debouncedSearch = $state('');
	let page = $state(1);
	let tab = $state<'rules' | 'versions' | 'sources'>('rules');

	// Detail state
	let activeVersionId = $state<string | null>(null);
	let versionRules = $state<Rule[]>([]);
	let rulesLoading = $state(false);
	let creatingVersion = $state(false);
	let editingRules = $state(false);
	let rulesDraft = $state<Rule[]>([]);

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
			const res = await apiRequest(`/v1/platform/knowledge-bases?${params}`);
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

	async function openDetail(id: string) {
		detailLoading = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/knowledge-bases/${id}`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const data = await res.json();
			detail = data;
			tab = 'rules';
			const activeV = data.versions?.find((v: Version) => v.status === 'active');
			const first = activeV ?? data.versions?.[0];
			if (first) {
				activeVersionId = first.id;
				await loadRules(first.id);
			}
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			detailLoading = false;
		}
	}

	async function loadRules(versionId: string) {
		if (!detail) return;
		rulesLoading = true;
		try {
			const res = await apiRequest(`/v1/platform/knowledge-bases/${detail.id}/versions/${versionId}`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const data = await res.json();
			versionRules = data.rules ?? [];
			rulesDraft = versionRules.map((r) => ({
				...r,
				examples: [...r.examples],
				forbidden_examples: [...r.forbidden_examples],
				content_types: [...r.content_types]
			}));
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			rulesLoading = false;
		}
	}

	async function create(event: SubmitEvent) {
		event.preventDefault();
		busy = true;
		error = '';
		try {
			const res = await apiRequest('/v1/platform/knowledge-bases', {
				method: 'POST',
				body: JSON.stringify({
					code: form.code.trim(),
					name: form.name.trim(),
					description: form.description.trim() || null
				})
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			createOpen = false;
			form = { code: '', name: '', description: '' };
			await load(page);
		} finally {
			busy = false;
		}
	}

	async function toggleKbStatus(kb: KbListItem) {
		error = '';
		try {
			const res = await apiRequest(`/v1/platform/knowledge-bases/${kb.id}`, {
				method: 'PATCH',
				body: JSON.stringify({ status: kb.status === 'active' ? 'inactive' : 'active' })
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

	async function refreshDetail() {
		if (!detail) return;
		try {
			const res = await apiRequest(`/v1/platform/knowledge-bases/${detail.id}`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			detail = await res.json();
		} catch {
			error = 'Permintaan gagal.';
		}
	}

	async function createVersion() {
		if (!detail) return;
		creatingVersion = true;
		try {
			const res = await apiRequest(`/v1/platform/knowledge-bases/${detail.id}/versions`, {
				method: 'POST',
				body: JSON.stringify({ change_note: `Draft v${detail.versions.length + 1}` })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const created = await res.json();
			activeVersionId = created.id;
			await refreshDetail();
			await loadRules(created.id);
		} finally {
			creatingVersion = false;
		}
	}

	async function activateVersion(versionId: string) {
		if (!detail) return;
		try {
			const res = await apiRequest(
				`/v1/platform/knowledge-bases/${detail.id}/versions/${versionId}/activate`,
				{ method: 'POST' }
			);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			await refreshDetail();
		} catch {
			error = 'Permintaan gagal.';
		}
	}

	async function deleteDraft(versionId: string) {
		if (!detail) return;
		try {
			const res = await apiRequest(`/v1/platform/knowledge-bases/${detail.id}/versions/${versionId}`, {
				method: 'DELETE'
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			await refreshDetail();
		} catch {
			error = 'Permintaan gagal.';
		}
	}

	async function saveRules() {
		if (!detail || !selectedVersion) return;
		editingRules = true;
		try {
			const clean = rulesDraft
				.filter((r) => r.instruction.trim().length >= 10)
				.map((r) => ({
					...r,
					examples: r.examples.filter(Boolean),
					forbidden_examples: r.forbidden_examples.filter(Boolean),
					content_types: r.content_types
				}));
			const res = await apiRequest(
				`/v1/platform/knowledge-bases/${detail.id}/versions/${selectedVersion.id}/rules`,
				{ method: 'POST', body: JSON.stringify({ rules: clean }) }
			);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			await loadRules(selectedVersion.id);
		} finally {
			editingRules = false;
		}
	}

	function patchRule(index: number, patch: Partial<Rule>) {
		rulesDraft = rulesDraft.map((r, i) => (i === index ? { ...r, ...patch } : r));
	}

	const selectedVersion = $derived(
		detail?.versions.find((v) => v.id === activeVersionId) ??
			detail?.versions.find((v) => v.status === 'active') ??
			detail?.versions[0] ??
			null
	);

	const grouped = $derived.by(() => {
		const map = new Map<string, Rule[]>();
		for (const rule of versionRules) {
			const list = map.get(rule.category) ?? [];
			list.push(rule);
			map.set(rule.category, list);
		}
		return [...map.entries()].sort((a, b) => a[0].localeCompare(b[0]));
	});

	function draftFor(rule: Rule): Rule {
		return rulesDraft.find((r) => r.id === rule.id) ?? rule;
	}

	function draftIndex(rule: Rule): number {
		return rulesDraft.findIndex((r) => r.id === rule.id);
	}

	function backToList() {
		detail = null;
		versionRules = [];
		rulesDraft = [];
		void load(page);
	}
</script>

{#snippet kbCell(row: KbListItem)}
	<div class="flex items-center gap-2">
		<BookOpenText size={16} />
		<div>
			<p class="font-medium">{row.name}</p>
			<p class="text-xs text-muted-foreground">{row.description || '—'}</p>
		</div>
	</div>
{/snippet}

{#snippet codeCell(row: KbListItem)}
	<code class="text-xs">{row.code}</code>
{/snippet}

{#snippet scopeCell(row: KbListItem)}
	<Badge tone="accent" value={row.scope} />
{/snippet}

{#snippet activeVersionCell(row: KbListItem)}
	<span>{row.active_version != null ? `v${row.active_version}` : '—'}</span>
{/snippet}

{#snippet ruleCountCell(row: KbListItem)}
	<span class="tabular-nums">{row.rule_count}</span>
{/snippet}

{#snippet kbStatusCell(row: KbListItem)}
	<Badge tone={row.status === 'active' ? 'success' : 'neutral'} value={row.status} />
{/snippet}

{#snippet kbActions(row: KbListItem)}
	<div class="flex items-center gap-2">
		<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" onclick={() => void openDetail(row.id)}>Kelola</button>
		{#if row.status === 'active'}
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--destructive);"
				onclick={() => void toggleKbStatus(row)}>Nonaktifkan</button
			>
		{:else}
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--success);"
				onclick={() => void toggleKbStatus(row)}>Aktifkan</button
			>
		{/if}
	</div>
{/snippet}

{#snippet ruleEditor(rule: Rule, editable: boolean, index: number)}
	<div class="rounded-lg border p-3">
		<div class="mb-2 flex flex-wrap items-center gap-2">
			<Select
				ariaLabel="Kategori"
				minWidth={160}
				disabled={!editable}
				value={rule.category}
				onchange={(v) => patchRule(index, { category: v })}
				options={CATEGORY_OPTIONS.map((c) => ({ value: c, label: c }))}
			/>
			<input
				class="app-input min-w-[160px] flex-1"
				disabled={!editable}
				placeholder="Judul rule"
				value={rule.title}
				oninput={(e) => patchRule(index, { title: e.currentTarget.value })}
			/>
			<Select
				ariaLabel="Prioritas"
				minWidth={150}
				disabled={!editable}
				value={String(rule.priority)}
				onchange={(v) => patchRule(index, { priority: Number(v) })}
				options={[1, 2, 3, 4, 5, 6, 7, 8].map((p) => ({ value: String(p), label: PRIORITY_LABELS[p] }))}
			/>
			<Select
				ariaLabel="Status rule"
				minWidth={130}
				disabled={!editable}
				value={rule.status}
				onchange={(v) => patchRule(index, { status: v })}
				options={[
					{ value: 'active', label: 'active' },
					{ value: 'inactive', label: 'inactive' }
				]}
			/>
		</div>
		<textarea
			class="app-input w-full"
			rows="3"
			disabled={!editable}
			placeholder="Instruksi rule (wajib, min 10 karakter)…"
			value={rule.instruction}
			oninput={(e) => patchRule(index, { instruction: e.currentTarget.value })}
		></textarea>
		<div class="mt-2 grid gap-2 md:grid-cols-2">
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Contoh baik</span>
				<textarea
					class="app-input mt-1 w-full"
					rows="2"
					disabled={!editable}
					placeholder="Contoh yang benar…"
					value={rule.examples.join('\n---\n')}
					oninput={(e) =>
						patchRule(index, { examples: e.currentTarget.value.split('\n---\n').filter(Boolean) })}
				></textarea>
			</label>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Contoh yang dihindari</span>
				<textarea
					class="app-input mt-1 w-full"
					rows="2"
					disabled={!editable}
					placeholder="Contoh yang dilarang…"
					value={rule.forbidden_examples.join('\n---\n')}
					oninput={(e) =>
						patchRule(index, { forbidden_examples: e.currentTarget.value.split('\n---\n').filter(Boolean) })}
				></textarea>
			</label>
		</div>
		<div class="mt-2 flex flex-wrap items-center gap-2">
			<span class="text-xs text-muted-foreground">Content types (kosong = semua):</span>
			<div class="flex flex-wrap gap-1">
				{#each CONTENT_TYPE_OPTIONS as ct (ct)}
					<button
						type="button"
						class="app-btn !min-h-7 !px-2 !text-[11px]"
						class:app-btn-primary={rule.content_types.includes(ct)}
						class:app-btn-soft={!rule.content_types.includes(ct)}
						disabled={!editable}
						onclick={() =>
							patchRule(index, {
								content_types: rule.content_types.includes(ct)
									? rule.content_types.filter((x) => x !== ct)
									: [...rule.content_types, ct]
							})}>{ct}</button
					>
				{/each}
			</div>
		</div>
	</div>
{/snippet}

<section class="mx-auto max-w-7xl">
	<PageHeader
		title={detail ? detail.name : 'Knowledge Base'}
		description="Aturan editorial & gaya penulisan yang diinjeksi ke AI Engine saat content generation. Dikelola Admin, berversi, aktif/nonaktif."
	>
		<Badge tone="accent" value={`${total} KB`} />
		{#if !detail}
			<button class="app-btn app-btn-primary" onclick={() => (createOpen = true)}>
				<Plus size={16} />Buat Knowledge Base
			</button>
		{/if}
	</PageHeader>

	<Feedback {loading} {error} />

	{#if detail}
		<!-- Detail view -->
		<div class="mt-4">
			<div class="mb-3 flex flex-wrap items-center gap-2">
				<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" onclick={backToList}>
					<ArrowLeft size={14} />Kembali
				</button>
				<p class="text-lg font-bold">{detail.name}</p>
				<Badge tone={detail.status === 'active' ? 'success' : 'neutral'} value={detail.status} />
				<Badge tone="accent" label={`${detail.scope}${detail.scope_ref ? ` · ${detail.scope_ref}` : ''}`} />
				<div class="flex-1"></div>
				<code class="text-xs text-muted-foreground">{detail.code}</code>
			</div>

			{#if detail.description}
				<div class="app-card mb-3 px-4 py-2.5 text-sm">{detail.description}</div>
			{/if}

			<div class="flex flex-wrap gap-1 border-b" role="tablist">
				<button
					type="button"
					class="app-btn !min-h-8 !rounded-b-none !px-3.5 !text-sm"
					class:app-btn-primary={tab === 'rules'}
					class:app-btn-soft={tab !== 'rules'}
					onclick={() => (tab = 'rules')}>Aturan ({versionRules.length})</button
				>
				<button
					type="button"
					class="app-btn !min-h-8 !rounded-b-none !px-3.5 !text-sm"
					class:app-btn-primary={tab === 'versions'}
					class:app-btn-soft={tab !== 'versions'}
					onclick={() => (tab = 'versions')}>Versi ({detail.versions.length})</button
				>
				<button
					type="button"
					class="app-btn !min-h-8 !rounded-b-none !px-3.5 !text-sm"
					class:app-btn-primary={tab === 'sources'}
					class:app-btn-soft={tab !== 'sources'}
					onclick={() => (tab = 'sources')}>Referensi ({detail.sources.length})</button
				>
			</div>

			{#if tab === 'rules'}
				<div class="mt-4">
					<div class="mb-3 flex flex-wrap items-center justify-between gap-2">
						<div class="flex flex-wrap items-center gap-2">
							<Select
								ariaLabel="Versi knowledge base"
								minWidth={220}
								value={selectedVersion?.id ?? ''}
								onchange={(v) => {
									activeVersionId = v;
									void loadRules(v);
								}}
								options={detail.versions.map((v) => ({
									value: v.id,
									label: `v${v.version} · ${v.status}${v.status === 'active' ? ' (dipakai engine)' : ''}`
								}))}
							/>
							{#if selectedVersion?.status !== 'draft'}
								<Badge tone="warning" label="Hanya versi draft yang bisa diedit — buat versi baru dulu." />
							{/if}
						</div>
						<div class="flex gap-2">
							{#if selectedVersion?.status === 'draft'}
								<button
									class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
									style="color: var(--success);"
									disabled={editingRules}
									onclick={saveRules}><Save size={14} />Simpan aturan</button
								>
								<button
									class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
									style="color: var(--primary);"
									disabled={editingRules}
									onclick={() => void activateVersion(selectedVersion.id)}
									>Publikasikan v{selectedVersion.version}</button
								>
							{/if}
							<button
								class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
								disabled={creatingVersion}
								onclick={() => void createVersion()}>+ Versi draft baru</button
							>
						</div>
					</div>

					{#if rulesLoading}
						<p class="text-sm text-muted-foreground">Memuat aturan…</p>
					{:else if grouped.length === 0}
						<div class="app-card px-6 py-10 text-center">
							<p class="text-sm text-muted-foreground">Belum ada aturan pada versi ini.</p>
						</div>
					{:else}
						<div class="max-h-[560px] overflow-auto">
							<div class="flex flex-col gap-3">
								{#each grouped as [category, rules] (category)}
									<div class="app-card p-4">
										<p class="mb-2 font-bold" style="color: var(--primary);">{category}</p>
										<div class="flex flex-col gap-2">
											{#each rules as rule (rule.id)}
												{@render ruleEditor(
													draftFor(rule),
													selectedVersion?.status === 'draft',
													draftIndex(rule)
												)}
											{/each}
										</div>
									</div>
								{/each}
							</div>
						</div>
					{/if}
				</div>
			{:else if tab === 'versions'}
				<div class="app-card mt-4 overflow-x-auto">
					<table class="w-full text-sm" style="min-width: 820px;">
						<thead>
							<tr class="text-xs text-muted-foreground">
								<th class="px-4 py-2.5 text-left font-medium">Versi</th>
								<th class="px-4 py-2.5 text-left font-medium">Status</th>
								<th class="px-4 py-2.5 text-left font-medium">Rules</th>
								<th class="px-4 py-2.5 text-left font-medium">Catatan</th>
								<th class="px-4 py-2.5 text-left font-medium">Dibuat</th>
								<th class="px-4 py-2.5 text-left font-medium"></th>
							</tr>
						</thead>
						<tbody>
							{#each detail.versions as v (v.id)}
								<tr class="border-t">
									<td class="px-4 py-2.5">v{v.version}</td>
									<td class="px-4 py-2.5">
										<Badge
											tone={v.status === 'active' ? 'success' : v.status === 'draft' ? 'warning' : 'neutral'}
											value={v.status}
										/>
									</td>
									<td class="px-4 py-2.5 tabular-nums">{v.rule_count ?? 0}</td>
									<td class="px-4 py-2.5">{v.change_note || '—'}</td>
									<td class="px-4 py-2.5">{new Date(v.created_at).toLocaleString('id-ID')}</td>
									<td class="px-4 py-2.5">
										<div class="flex gap-2">
											{#if v.status === 'draft'}
												<button
													class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
													style="color: var(--success);"
													onclick={() => void activateVersion(v.id)}>Publikasikan</button
												>
											{/if}
											{#if v.status === 'active'}
												<Badge tone="success" label="dipakai engine" />
											{/if}
											{#if v.status === 'draft'}
												<button
													class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
													style="color: var(--destructive);"
													onclick={() => void deleteDraft(v.id)}>Hapus</button
												>
											{/if}
										</div>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{:else}
				<div class="app-card mt-4 overflow-x-auto">
					<table class="w-full text-sm" style="min-width: 760px;">
						<thead>
							<tr class="text-xs text-muted-foreground">
								<th class="px-4 py-2.5 text-left font-medium">Referensi</th>
								<th class="px-4 py-2.5 text-left font-medium">Tipe</th>
								<th class="px-4 py-2.5 text-left font-medium">URL</th>
								<th class="px-4 py-2.5 text-left font-medium">Catatan</th>
							</tr>
						</thead>
						<tbody>
							{#each detail.sources as s (s.id)}
								<tr class="border-t">
									<td class="px-4 py-2.5">{s.source_name}</td>
									<td class="px-4 py-2.5"><Badge tone="accent" value={s.source_type} /></td>
									<td class="px-4 py-2.5 text-xs">{s.source_url ?? '—'}</td>
									<td class="px-4 py-2.5">{s.notes || '—'}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>
	{:else}
		<div class="mt-4 flex flex-wrap items-center gap-2">
			<label class="app-input flex min-w-[240px] flex-1 items-center gap-2 !py-1.5">
				<Search size={15} class="text-muted-foreground" />
				<input
					class="w-full bg-transparent text-sm outline-none"
					placeholder="Cari nama, kode, atau deskripsi…"
					aria-label="Cari knowledge base"
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
				{ key: 'name', label: 'Knowledge Base', render: kbCell },
				{ key: 'code', label: 'Kode', render: codeCell },
				{ key: 'scope', label: 'Scope', render: scopeCell },
				{ key: 'version', label: 'Versi aktif', render: activeVersionCell },
				{ key: 'rules', label: 'Rules', render: ruleCountCell },
				{ key: 'status', label: 'Status', render: kbStatusCell },
				{ key: 'actions', label: '', render: kbActions }
			]}
			rows={rows}
			rowKey={(row: KbListItem) => row.id}
			{loading}
			empty="Belum ada knowledge base. Buat yang pertama."
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
	{/if}
</section>

{#if createOpen}
	<Modal title="Buat Knowledge Base" maxWidth="md" onClose={() => (createOpen = false)}>
		<p class="mb-3 text-sm text-muted-foreground">
			Knowledge base baru dibuat dengan versi 1.0 aktif. Aturan diisi dari halaman kelola.
		</p>
		<form class="space-y-3" onsubmit={create}>
			<label class="block">
				<span class="text-xs font-bold text-muted-foreground">Kode (slug)</span>
				<input class="app-input mt-1 w-full" required placeholder="journalistic-writing-id" bind:value={form.code} />
			</label>
			<label class="block">
				<span class="text-xs font-bold text-muted-foreground">Nama</span>
				<input
					class="app-input mt-1 w-full"
					required
					placeholder="Standar Penulisan Jurnalistik Indonesia"
					bind:value={form.name}
				/>
			</label>
			<label class="block">
				<span class="text-xs font-bold text-muted-foreground">Deskripsi</span>
				<textarea
					class="app-input mt-1 w-full"
					placeholder="Tujuan &amp; cakupan knowledge base…"
					bind:value={form.description}
				></textarea>
			</label>
			{#if error}
				<p class="text-sm" style="color: var(--destructive);">{error}</p>
			{/if}
			<div class="flex justify-end gap-2 border-t pt-4">
				<button type="button" class="app-btn app-btn-soft" onclick={() => (createOpen = false)}>Batal</button>
				<button type="submit" class="app-btn app-btn-primary" disabled={busy}>{busy ? 'Menyimpan…' : 'Buat KB'}</button>
			</div>
		</form>
	</Modal>
{/if}
