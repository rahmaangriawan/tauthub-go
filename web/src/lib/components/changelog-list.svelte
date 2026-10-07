<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Daftar "Apa yang Baru" — dipakai dashboard klien dan mitra. Isi entri
	// berasal dari GET /v1/changelog (hasil impor admin dari
	// web/static/changelog.md) dan ditandai sudah dibaca saat halaman dibuka.
	import { onMount } from 'svelte';
	import { LoaderCircle, Megaphone } from '@lucide/svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { getI18n } from '$lib/i18n/i18n.svelte';
	import { sanitizeHtml } from '$lib/sanitize-html';
	import PageTitle from '$lib/components/page-title.svelte';

	type Entry = {
		id: string;
		version: string;
		released_on: string;
		category: string;
		title: string;
		summary: string;
		body: string;
		read_at: string | null;
	};

	const CATEGORY_STYLE: Record<string, string> = {
		fitur: 'background-color: var(--accent); color: var(--primary);',
		perbaikan: 'background-color: var(--warning-soft); color: var(--warning);',
		performa: 'background-color: var(--success-soft); color: var(--success);',
		lainnya: 'background-color: var(--muted); color: var(--muted-foreground);'
	};

	const CATEGORY_LABEL: Record<string, string> = {
		fitur: 'Fitur',
		perbaikan: 'Perbaikan',
		performa: 'Performa',
		lainnya: 'Lainnya'
	};

	const i18n = $derived(getI18n());
	// Isi entri diterjemahkan otomatis oleh API saat antarmuka Inggris;
	// daftar dimuat ulang begitu bahasanya berganti.
	const lang = $derived(i18n.locale === 'en' ? 'en' : 'id');
	let loadedLang = $state('');
	let ready = $state(false);

	let items = $state<Entry[]>([]);
	let unread = $state(0);
	let loading = $state(true);
	let error = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			ready = true;
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	// Bahasa antarmuka berganti → muat ulang daftar dengan terjemahan bahasa itu.
	$effect(() => {
		if (!ready || loadedLang === lang) return;
		void load();
	});

	async function load() {
		error = '';
		loadedLang = lang;
		try {
			// Terjemahan otomatis bisa memakan puluhan detik pada permintaan pertama
			// (sesudahnya hasilnya di-cache di server) — timeout dinaikkan.
			const res = await apiRequest(`/v1/changelog?lang=${lang}`, {}, 120_000);
			if (!res.ok) throw new Error(tr('Gagal memuat catatan rilis.'));
			const data = (await res.json()) as { items?: Entry[]; unread?: number };
			items = data.items ?? [];
			unread = data.unread ?? 0;
			await markRead(items.filter((e) => !e.read_at));
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal memuat catatan rilis.');
		}
	}

	/** Tandai entri sudah dibaca (senyap — kegagalan tidak mengganggu bacaan). */
	async function markRead(targets: Entry[]) {
		if (targets.length === 0) return;
		const now = new Date().toISOString();
		await Promise.all(
			targets.map((e) =>
				apiRequest(`/v1/changelog/${e.id}/read`, { method: 'POST' }).catch(() => undefined)
			)
		);
		items = items.map((e) => (e.read_at ? e : { ...e, read_at: now }));
		unread = 0;
	}

	function fmtDate(value: string): string {
		const parsed = new Date(`${value}T00:00:00`);
		if (Number.isNaN(parsed.getTime())) return value;
		return parsed.toLocaleDateString('id-ID', { day: 'numeric', month: 'long', year: 'numeric' });
	}
</script>

<PageTitle title={tr('Apa yang Baru')} />

<section class="mx-auto w-full max-w-3xl">
	<header class="flex flex-wrap items-center justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{tr('Apa yang Baru')}</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				{tr('Ringkasan perubahan Tauthub — fitur baru, perbaikan, dan peningkatan performa.')}
			</p>
		</div>
		{#if unread > 0}
			<span class="rounded-full px-3 py-1 text-xs font-semibold" style="background-color: var(--accent); color: var(--primary);">
				{unread} {tr('belum dibaca')}
			</span>
		{/if}
	</header>

	{#if loading}
		<div class="mt-6 flex items-center justify-center gap-2 text-sm text-muted-foreground">
			<LoaderCircle size={18} class="animate-spin" />{tr('Memuat catatan rilis…')}
		</div>
	{:else if error}
		<div class="app-card mt-4 px-4 py-3 text-sm" style="color: var(--destructive);" role="alert">{error}</div>
	{:else if items.length === 0}
		<div class="app-card mt-4 px-6 py-10 text-center">
			<Megaphone size={22} class="mx-auto text-muted-foreground" />
			<p class="mt-2 text-sm text-muted-foreground">{tr('Belum ada catatan rilis.')}</p>
		</div>
	{:else}
		<ol class="mt-4 grid gap-3">
			{#each items as entry (entry.id)}
				<li class="app-card p-5" style={entry.read_at ? '' : 'border-left: 3px solid var(--primary);'}>
					<div class="flex flex-wrap items-center gap-2">
						<span class="rounded-full px-2.5 py-0.5 text-[11px] font-bold" style="background-color: var(--muted);">{entry.version}</span>
						<span class="rounded-full px-2.5 py-0.5 text-[11px] font-semibold" style={CATEGORY_STYLE[entry.category] ?? CATEGORY_STYLE.lainnya}>
							{tr(CATEGORY_LABEL[entry.category] ?? entry.category)}
						</span>
						<span class="text-xs text-muted-foreground">{fmtDate(entry.released_on)}</span>
						{#if !entry.read_at}
							<span class="rounded-full px-2 py-0.5 text-[10px] font-bold" style="background-color: var(--primary); color: var(--primary-foreground);">{tr('BARU')}</span>
						{/if}
					</div>
					<h2 class="mt-2 font-semibold">{entry.title}</h2>
					<div class="prose-sm mt-2 text-sm leading-relaxed text-foreground [&_li]:ml-4 [&_li]:list-disc [&_p]:mt-2 [&_strong]:font-semibold">
						{@html sanitizeHtml(entry.body)}
					</div>
				</li>
			{/each}
		</ol>
	{/if}
</section>
