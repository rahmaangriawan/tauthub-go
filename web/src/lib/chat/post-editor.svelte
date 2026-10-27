<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Editor artikel dari dalam obrolan klien. Klien memperbaiki artikel sesuai
	// permintaan mitra, melihat riwayat permintaan (review_comments), lalu bila
	// publikasi berstatus revision bisa langsung mengirim ulang ke mitra.
	import { onMount } from 'svelte';
	import { LoaderCircle, RotateCcw } from '@lucide/svelte';
	import EditorModal from '$lib/components/article-editor-modal.svelte';
	import { apiRequest, readApiError } from '$lib/auth';

	type ThreadLite = {
		id: string;
		campaign_id: string | null;
		document_id: string | null;
		media_id: string;
		media_name: string;
		publication_id: string | null;
		pub_status: string | null;
		topic: string | null;
		topic_note?: string;
	};

	type RevisionComment = {
		id: string;
		body: string;
		resolved: boolean;
		created_at: string;
	};

	let {
		apiBase,
		thread,
		onClose
	}: { apiBase: string; thread: ThreadLite; onClose: () => void } = $props();

	let loading = $state(true);
	let loadError = $state('');
	let saving = $state(false);
	let saveError = $state('');
	let notice = $state('');
	let resending = $state(false);

	let title = $state('');
	let html = $state('');
	let cover = $state<string | null>(null);
	let category = $state('');
	let meta = $state('');
	let comments = $state<RevisionComment[]>([]);

	const topicLabel = (topic: string | null) =>
		topic === 'confirm' ? tr('Konfirmasi') : topic === 'revision' ? tr('Revisi') : topic === 'quote' ? tr('Butuh Kutipan') : '';

	const fmtDateTime = (iso: string) =>
		iso
			? new Date(iso).toLocaleString('id-ID', {
					day: 'numeric',
					month: 'short',
					year: 'numeric',
					hour: '2-digit',
					minute: '2-digit'
				})
			: '—';

	onMount(() => {
		void load();
	});

	async function load() {
		loading = true;
		loadError = '';
		try {
			const [artRes, docRes] = await Promise.all([
				apiRequest(`${apiBase}/taut/campaigns/${thread.campaign_id}/articles/${thread.media_id}`),
				thread.document_id ? apiRequest(`${apiBase}/documents/${thread.document_id}`) : Promise.resolve(null)
			]);
			if (!artRes.ok) {
				loadError = await readApiError(artRes, tr('Gagal memuat artikel.'));
				return;
			}
			const art = (await artRes.json().catch(() => null)) as Record<string, unknown> | null;
			title = String(art?.title ?? '');
			html = String(art?.content_html ?? '');
			cover = (art?.featured_image_url as string | null) ?? null;
			category = String(art?.category ?? '');
			meta = String(art?.meta_description ?? '');

			if (docRes?.ok) {
				const doc = (await docRes.json().catch(() => null)) as { comments?: unknown } | null;
				const list = Array.isArray(doc?.comments) ? doc.comments : [];
				comments = list
					.map((c) => c as Record<string, unknown>)
					.filter((c) => typeof c?.body === 'string' && String(c.body).trim() !== '')
					.map((c) => ({
						id: String(c.id ?? ''),
						body: String(c.body ?? ''),
						resolved: Boolean(c.resolved),
						created_at: String(c.created_at ?? '')
					}));
			}
		} catch (reason) {
			loadError = reason instanceof Error ? reason.message : tr('Gagal memuat artikel.');
		} finally {
			loading = false;
		}
	}

	async function uploadCover(file: File): Promise<string> {
		const form = new FormData();
		form.append('file', file);
		const res = await apiRequest(`${apiBase}/assets`, { method: 'POST', body: form });
		const data = await res.json().catch(() => null);
		if (!res.ok || !data?.url) throw new Error(data?.detail ?? tr('Gagal mengunggah cover.'));
		return String(data.url);
	}

	async function save(
		nextTitle: string,
		nextHtml: string,
		nextCover: string | null,
		nextCategory: string,
		nextMeta: string
	) {
		if (saving) return;
		saving = true;
		saveError = '';
		notice = '';
		try {
			const res = await apiRequest(`${apiBase}/taut/campaigns/${thread.campaign_id}/articles/${thread.media_id}`, {
				method: 'PUT',
				body: JSON.stringify({
					title: nextTitle,
					content_html: nextHtml,
					featured_image_url: nextCover ?? undefined,
					category: nextCategory,
					meta_description: nextMeta
				})
			});
			if (!res.ok) {
				saveError = await readApiError(res, tr('Gagal menyimpan artikel.'));
				return;
			}
			title = nextTitle;
			html = nextHtml;
			cover = nextCover;
			category = nextCategory;
			meta = nextMeta;
			notice = tr('Artikel tersimpan.');
		} finally {
			saving = false;
		}
	}

	async function resend() {
		if (!thread.publication_id || resending) return;
		resending = true;
		saveError = '';
		try {
			const res = await apiRequest(`${apiBase}/publications/${thread.publication_id}/retry`, { method: 'POST' });
			if (!res.ok) {
				saveError = await readApiError(res, tr('Gagal mengirim ulang artikel.'));
				return;
			}
			notice = tr('Artikel dikirim ulang ke mitra.');
			setTimeout(onClose, 900);
		} finally {
			resending = false;
		}
	}
</script>

{#snippet historyPanel()}
	{#if thread.topic}
		<div>
			<p class="text-[11px] font-bold uppercase tracking-wide text-slate-400">{tr('Alasan mitra menghubungi')}</p>
			<span class="mt-1.5 inline-flex rounded-full border border-amber-200 bg-amber-50 px-2.5 py-0.5 text-[11px] font-semibold text-amber-700">
				{topicLabel(thread.topic)}
			</span>
			{#if thread.topic_note}
				<p class="mt-1.5 text-[12px] leading-relaxed text-slate-600">{thread.topic_note}</p>
			{/if}
		</div>
	{/if}

	<div>
		<p class="text-[11px] font-bold uppercase tracking-wide text-slate-400">{tr('Riwayat permintaan mitra')}</p>
		{#if comments.length === 0}
			<p class="mt-1.5 text-[12px] text-muted-foreground">{tr('Belum ada permintaan revisi dari mitra.')}</p>
		{:else}
			<ul class="mt-1.5 space-y-2">
				{#each comments as c (c.id)}
					<li class="rounded-lg border p-2.5" style="border-color: var(--border);">
						<div class="flex items-center justify-between gap-2">
							<span class="text-[10.5px] text-muted-foreground">{fmtDateTime(c.created_at)}</span>
							<span
								class="rounded-full px-1.5 py-0.5 text-[10px] font-semibold"
								style={c.resolved
									? 'background: var(--success-soft); color: var(--success);'
									: 'background: var(--warning-soft); color: var(--warning);'}
							>
								{c.resolved ? tr('Selesai') : tr('Belum selesai')}
							</span>
						</div>
						<p class="mt-1 whitespace-pre-wrap text-[12px] leading-relaxed text-slate-700">{c.body}</p>
					</li>
				{/each}
			</ul>
		{/if}
	</div>

	{#if notice}
		<p class="text-[12px] font-medium" style="color: var(--success);">{notice}</p>
	{/if}
	{#if thread.pub_status === 'revision' && notice}
		<button
			type="button"
			class="app-btn app-btn-primary inline-flex w-full items-center justify-center gap-1.5"
			disabled={resending}
			onclick={() => void resend()}
		>
			{#if resending}<LoaderCircle size={13} class="animate-spin" />{:else}<RotateCcw size={13} />{/if}
			{resending ? tr('Mengirim ulang…') : tr('Kirim ulang ke mitra')}
		</button>
	{/if}
{/snippet}

{#if loading}
	<div class="fixed inset-0 z-[70] flex items-center justify-center p-4" style="background-color: rgba(15, 23, 42, 0.45);" role="presentation">
		<div class="app-card flex items-center gap-2 px-5 py-4 text-sm font-semibold">
			<LoaderCircle size={17} class="animate-spin" /> {tr('Memuat artikel…')}
		</div>
	</div>
{:else if loadError}
	<div class="fixed inset-0 z-[70] flex items-center justify-center p-4" style="background-color: rgba(15, 23, 42, 0.45);" role="presentation" onclick={(e) => { if (e.target === e.currentTarget) onClose(); }}>
		<dialog open class="app-card w-full max-w-[420px] p-5" aria-label={tr('Edit Post')} onclick={(e) => e.stopPropagation()}>
			<h2 class="text-lg font-semibold">{tr('Edit Post')}</h2>
			<p class="mt-2 text-sm" style="color: var(--destructive);">{loadError}</p>
			<div class="mt-4 flex justify-end">
				<button class="app-btn app-btn-soft" type="button" onclick={onClose}>{tr('Tutup')}</button>
			</div>
		</dialog>
	</div>
{:else}
	<EditorModal
		mediaName={thread.media_name}
		initialTitle={title}
		initialHtml={html}
		initialCoverUrl={cover}
		initialCategory={category}
		initialMetaDescription={meta}
		categoryOptions={category ? [category] : []}
		linkLimit={2}
		enforceLinkLimit={false}
		requireCover={false}
		saving={saving}
		saveError={saveError}
		subtitle={tr('Perbaiki artikel sesuai permintaan mitra, lalu simpan.')}
		onClose={() => { if (!saving) onClose(); }}
		onSave={(nextTitle, nextHtml, nextCover, nextCategory, nextMeta) =>
			void save(nextTitle, nextHtml, nextCover, nextCategory, nextMeta)}
		onUploadCover={uploadCover}
		asideExtra={historyPanel}
	/>
{/if}
