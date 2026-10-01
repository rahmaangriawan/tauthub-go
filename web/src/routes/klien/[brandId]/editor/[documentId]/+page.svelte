<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Fase 4: editor dokumen (port section Editor minimal) — versi, autosave,
	// status review. Diff/komentar/restore menyusul bersama fase review.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
	import RichTextEditor from '$lib/components/rich-text-editor.svelte';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const documentId = $derived(page.params.documentId as string);

	type Version = {
		id: string;
		version: number;
		html: string;
		attribution: string;
		change_note: string | null;
		created_at: string;
	};

	let doc = $state<any>(null);
	let html = $state('');
	let loading = $state(true);
	let busy = $state(false);
	let msg = $state('');
	let error = $state('');

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
		const res = await apiRequest(`/v1/brands/${brandId}/documents/${documentId}`);
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal memuat dokumen.'));
			return;
		}
		doc = await res.json();
		html = doc.content?.html ?? '';
	}

	async function save() {
		if (!doc) return;
		busy = true;
		error = '';
		msg = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/documents/${documentId}`, {
				method: 'PUT',
				body: JSON.stringify({
					html,
					change_note: 'Edit manual',
					base_version: doc.current_version
				})
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menyimpan.'));
				return;
			}
			doc = await res.json();
			html = doc.content?.html ?? '';
			msg = `Versi ${doc.current_version} tersimpan.`;
		} finally {
			busy = false;
		}
	}

	async function restoreVersion(v: Version) {
		busy = true;
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/documents/${documentId}`, {
				method: 'PUT',
				body: JSON.stringify({ html: v.html, change_note: `Pulihkan v${v.version}`, base_version: doc?.current_version })
			});
			if (res.ok) {
				doc = await res.json();
				html = doc.content?.html ?? '';
				msg = `Dokumen dipulihkan dari versi ${v.version}.`;
			} else {
				error = await readApiError(res, tr('Gagal memulihkan versi.'));
			}
		} finally {
			busy = false;
		}
	}
</script>

<PageTitle title={tr("Editor Artikel")} />

<section>
	<a class="text-sm text-muted-foreground underline" href={`/klien/${brandUrl}/generate`}>← Generate</a>
	{#if loading}
		<p class="mt-4 text-sm text-muted-foreground">{tr("Memuat dokumen…")}</p>
	{:else if doc}
		<div class="mt-2 flex flex-wrap items-center justify-between gap-3">
			<div>
				<h1 class="text-xl font-semibold tracking-tight">{doc.title}</h1>
				<p class="text-xs text-muted-foreground">
					/{doc.slug} · status <strong>{doc.status}</strong> · versi aktif v{doc.current_version}
				</p>
			</div>
			<div class="flex items-center gap-2">
				{#if msg}<span class="text-xs text-emerald-600">{msg}</span>{/if}
				{#if error}<span class="text-xs text-destructive">{error}</span>{/if}
				<button class="app-btn app-btn-primary" disabled={busy} onclick={save}>
					{busy ? 'Menyimpan…' : tr('Simpan versi baru')}
				</button>
			</div>
		</div>

		<div class="mt-4 grid gap-4 lg:grid-cols-[2fr_1fr]">
			<section class="app-card p-4">
				<div class="flex items-center justify-between">
					<h2 class="text-sm font-semibold">Konten</h2>
				</div>
				<RichTextEditor bind:value={html} {brandId} disabled={busy} ariaLabel={tr('Isi artikel')} />
			</section>

			<div class="grid gap-4">

				<section class="app-card p-4">
					<h2 class="text-sm font-semibold">{tr("Riwayat versi (")}{(doc.versions ?? []).length})</h2>
					<ul class="mt-2 space-y-2 text-xs">
						{#each doc.versions ?? [] as v (v.id)}
							<li class="flex items-start justify-between gap-2 border-b border-border/50 pb-2 last:border-0">
								<div>
									<p class="font-medium">v{v.version} · {v.attribution}</p>
									<p class="text-muted-foreground">{v.change_note ?? '—'}</p>
								</div>
								{#if v.version !== doc.current_version}
									<button class="shrink-0 underline" disabled={busy} onclick={() => restoreVersion(v)}>Pulihkan</button>
								{:else}
									<span class="shrink-0 text-emerald-600">aktif</span>
								{/if}
							</li>
						{/each}
					</ul>
				</section>
			</div>
		</div>
	{:else}
		<p class="mt-4 text-sm text-destructive">{error || 'Dokumen tidak ditemukan.'}</p>
	{/if}
</section>
