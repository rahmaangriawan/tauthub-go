<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Drawer Pedoman Editorial AI — 15 standar penulisan per jenis konten,
	// diindeks sebagai RAG knowledge engine.
	// Port apps/web/src/app/admin/content-standards/_editorial-drawer.tsx.
	import { Check, Clock, Save, ShieldCheck, Sparkle, Target, PenTool, Quote, FileText, X } from '@lucide/svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import { apiRequest, readApiError } from '$lib/auth';
	import {
		DEFAULT_STANDARDS_BY_TYPE,
		EDITORIAL_GROUPS,
		EDITORIAL_SECTIONS_META,
		getEmptyEditorialSections,
		type ContentTypeStandardDetail,
		type EditorialSectionKey,
		type EditorialSectionsData,
		type SectionGroupCategory
	} from './_editorial';

	let {
		contentTypeCode,
		contentTypeName,
		open,
		headerBadge,
		onClose,
		onSaved
	}: {
		contentTypeCode: string | null;
		contentTypeName: string;
		open: boolean;
		headerBadge?: string;
		onClose: () => void;
		onSaved: () => void;
	} = $props();

	const groupIcons: Record<SectionGroupCategory, typeof Target> = {
		purpose_angle: Target,
		style_structure: PenTool,
		fact_quotes: Quote,
		info_sources: FileText,
		guardrails_quality: ShieldCheck
	};

	let detail = $state<ContentTypeStandardDetail | null>(null);
	let loading = $state(false);
	let saving = $state(false);
	let error = $state('');
	let successMsg = $state('');
	let activeGroup = $state<SectionGroupCategory>('purpose_angle');
	let sections = $state<EditorialSectionsData>(getEmptyEditorialSections());
	let changeNote = $state('');

	$effect(() => {
		if (open && contentTypeCode) {
			void loadStandard(contentTypeCode);
		}
	});

	async function loadStandard(code: string) {
		loading = true;
		error = '';
		successMsg = '';
		try {
			const res = await apiRequest(`/v1/platform/content-intake-types/${code}/standards`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			const data = await res.json();
			detail = data.standard;
			if (data.standard?.active_standard?.sections) {
				sections = data.standard.active_standard.sections;
			} else {
				// Jika belum ada, gunakan default preset jika tersedia, atau kosong.
				const preset = DEFAULT_STANDARDS_BY_TYPE[code];
				sections = preset ? { ...preset.sections } : getEmptyEditorialSections();
			}
			changeNote = '';
		} catch {
			error = 'Permintaan gagal.';
		} finally {
			loading = false;
		}
	}

	function handleSectionChange(key: EditorialSectionKey, value: string) {
		sections = { ...sections, [key]: value };
	}

	function applyPreset() {
		if (!contentTypeCode) return;
		const preset = DEFAULT_STANDARDS_BY_TYPE[contentTypeCode];
		if (preset) {
			sections = { ...preset.sections };
			changeNote = tr('Memuat standar rekomendasi bawaan Tauthub');
			successMsg = 'Standar rekomendasi berhasil dimuat ke formulir.';
		}
	}

	async function handleSave(statusToSet: 'active' | 'draft') {
		if (!contentTypeCode) return;
		saving = true;
		error = '';
		successMsg = '';
		try {
			const res = await apiRequest(`/v1/platform/content-intake-types/${contentTypeCode}/standards`, {
				method: 'PUT',
				body: JSON.stringify({
					sections,
					status: statusToSet,
					change_note: changeNote.trim() || undefined
				})
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			successMsg =
				statusToSet === 'active'
					? 'Standar AI berhasil disimpan dan diaktifkan (RAG chunks diindeks)!'
					: 'Draft standar AI berhasil disimpan.';
			await loadStandard(contentTypeCode);
			onSaved();
		} finally {
			saving = false;
		}
	}

	async function handleActivateVersion(versionId: string) {
		if (!contentTypeCode) return;
		saving = true;
		error = '';
		try {
			const res = await apiRequest(
				`/v1/platform/content-intake-types/${contentTypeCode}/standards/versions/${versionId}/activate`,
				{ method: 'POST' }
			);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan gagal.');
				return;
			}
			successMsg = 'Versi berhasil diaktifkan!';
			await loadStandard(contentTypeCode);
			onSaved();
		} finally {
			saving = false;
		}
	}

	const sectionsInCurrentGroup = $derived(EDITORIAL_SECTIONS_META.filter((m) => m.group === activeGroup));
</script>

{#if open}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4"
		style="background-color: rgba(0,0,0,.45);"
		role="presentation"
		onclick={onClose}
	>
		<dialog
			open
			class="app-card w-full max-w-5xl p-0"
			style="max-height: 90vh; overflow: hidden;"
			aria-label={contentTypeName || 'Editorial Standard'}
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => {
				if (e.key === 'Escape') onClose();
			}}
		>
			<!-- Header Drawer -->
			<div class="border-b p-4" style="background-color: var(--secondary);">
				<div class="flex items-start justify-between gap-3">
					<div>
						<div class="flex flex-wrap items-center gap-2">
							<h2 class="text-lg font-semibold">{contentTypeName || 'Editorial Standard'}</h2>
							<Badge tone="neutral" label={`Kode: ${contentTypeCode}`} />
							{#if detail?.active_version}
								<Badge tone="success" label={`Standar AI Aktif (v${detail.active_version})`} />
							{:else}
								<Badge tone="warning" label={tr("Belum Ada Standar Aktif")} />
							{/if}
							{#if detail?.chunk_count}
								<Badge tone="accent" label={`${detail.chunk_count} RAG Chunks`} />
							{/if}
							{#if headerBadge}
								<Badge tone="neutral" label={headerBadge} />
							{/if}
						</div>
						<p class="mt-1 text-sm text-muted-foreground">{tr("Pedoman penulisan khusus (Editorial Knowledge) yang diindeks ke AI Engine Tauthub untuk menentukan")}<strong>HOW TO WRITE</strong> jenis konten ini.
						</p>
					</div>
					<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label={tr("Tutup")} onclick={onClose}>
						<X size={18} />
					</button>
				</div>
			</div>

			{#if error}
				<div class="px-4 pt-3 text-sm" style="color: var(--destructive);">{error}</div>
			{/if}
			{#if successMsg}
				<div class="px-4 pt-3 text-sm" style="color: var(--success);">
					<Check size={14} class="mr-1 inline" />{successMsg}
				</div>
			{/if}

			<!-- Main Content Area -->
			<div class="flex flex-col p-4" style="height: calc(90vh - 200px); overflow: hidden;">
				<div class="flex flex-wrap gap-1 border-b pb-2">
					{#each EDITORIAL_GROUPS as g (g.id)}
						{@const IconComponent = groupIcons[g.id] ?? Target}
						<button
							type="button"
							class="app-btn !min-h-8 !whitespace-nowrap !px-3 !text-xs"
							class:app-btn-primary={activeGroup === g.id}
							class:app-btn-soft={activeGroup !== g.id}
							onclick={() => (activeGroup = g.id)}
						>
							<IconComponent size={15} /> {g.label}
						</button>
					{/each}
				</div>

				<div class="mt-4 flex-1 overflow-y-auto pr-2">
					{#if loading}
						<p class="text-sm text-muted-foreground">{tr("Memuat standar…")}</p>
					{:else}
						<div class="space-y-4">
							{#each sectionsInCurrentGroup as meta (meta.key)}
								{@const value = sections[meta.key] ?? ''}
								<div class="app-card p-3">
									<div class="mb-2 flex items-start justify-between gap-3">
										<div>
											<p class="font-bold">{meta.label}</p>
											<p class="mt-1 text-xs text-muted-foreground">{meta.description}</p>
										</div>
										<span class="shrink-0 text-xs tabular-nums text-muted-foreground"
											>{value.length} karakter</span
										>
									</div>
									<textarea
										class="app-input w-full text-xs leading-relaxed"
										rows="4"
										placeholder={meta.placeholder}
										value={value}
										oninput={(e) => handleSectionChange(meta.key, e.currentTarget.value)}
									></textarea>
								</div>
							{/each}

							<!-- Version & Change Note Section -->
							<div class="border-t pt-3">
								<p class="mb-1 text-xs font-bold text-muted-foreground">CATATAN PERUBAHAN VERSI (OPSIONAL)</p>
								<input
									class="app-input w-full"
									placeholder={tr("Contoh: Menyesuaikan aturan lead dan atribusi klaim...")}
									bind:value={changeNote}
								/>
							</div>

							<!-- Version History Quick List -->
							{#if detail?.versions && detail.versions.length > 1}
								<div class="app-card p-3">
									<div class="mb-2 flex items-center gap-1 text-xs font-bold text-muted-foreground">
										<Clock size={14} /> RIWAYAT VERSI TERDAFTAR
									</div>
									<div class="flex flex-wrap gap-2">
										{#each detail.versions as v (v.id)}
											<div
												class="flex items-center gap-2 rounded-md border px-2 py-1"
												style="background-color: {v.status === 'active'
													? 'var(--success-soft)'
													: 'var(--secondary)'};"
											>
												<span class="text-xs font-medium">v{v.version} ({v.status})</span>
												{#if v.change_note}
													<span class="text-xs text-muted-foreground">· {v.change_note}</span>
												{/if}
												{#if v.status !== 'active'}
													<button
														class="app-btn app-btn-soft !min-h-6 !px-2 !text-[11px]"
														style="color: var(--primary);"
														onclick={() => void handleActivateVersion(v.id)}>Aktifkan</button
													>
												{/if}
											</div>
										{/each}
									</div>
								</div>
							{/if}
						</div>
					{/if}
				</div>
			</div>

			<!-- Footer Actions -->
			<div class="border-t p-4" style="background-color: var(--secondary);">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<button class="app-btn app-btn-soft" onclick={applyPreset}>
						<Sparkle size={15} /> Muat Contoh Rekomendasi Tauthub
					</button>
					<div class="flex items-center gap-2">
						<button class="app-btn app-btn-soft" onclick={onClose}>{tr("Tutup")}</button>
						<button
							class="app-btn app-btn-soft"
							disabled={saving || loading}
							onclick={() => void handleSave('draft')}>{tr("Simpan Draft")}</button
						>
						<button
							class="app-btn app-btn-primary"
							disabled={saving || loading}
							onclick={() => void handleSave('active')}
						>
							<Save size={16} /> {saving ? 'Menyimpan & Mengindeks RAG…' : tr('Simpan & Aktifkan AI Standard')}
						</button>
					</div>
				</div>
			</div>
		</dialog>
	</div>
{/if}
