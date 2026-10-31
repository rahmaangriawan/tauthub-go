<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Halaman admin Media terpadu: tab Direktori (master data), tab Arsip
	// (media terarsip / disembunyikan dari mitra), tab Pending (menunggu review),
	// tab Intelligence (health score), tab Pelanggaran (penindakan domain),
	// tab Pengaturan.
	// Port apps/web/src/app/admin/media/page.tsx.
	//
	// Deep-link dari notifikasi admin (`/admin/media?status=pending_review#tab-pending`)
	// membuka tab Pending langsung.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import MediaDirectory from './media-directory.svelte';
	import MediaArchive from './media-archive.svelte';
	import MediaPending from './media-pending.svelte';
	import MediaRecent from './media-recent.svelte';
	import MediaIntelligenceTab from './media-intelligence-tab.svelte';
	import MediaViolationsTab from './media-violations-tab.svelte';
	import MediaSettings from './media-settings.svelte';
	import MediaLogTab from './media-log-tab.svelte';

	const PENDING_MEDIA_PATH = '/v1/platform/media?limit=1&offset=0&status=pending_review';

	let tab = $state(page.url.searchParams.get('status') === 'pending_review' ? 'pending' : 'directory');
	let pendingCount = $state<number | null>(null);

	// Query berubah saat admin mengeklik notifikasi → pindah tab tanpa remount.
	$effect(() => {
		if (page.url.searchParams.get('status') === 'pending_review') tab = 'pending';
	});

	async function loadPendingCount() {
		const res = await apiRequest(PENDING_MEDIA_PATH);
		if (res.ok) {
			const data = await res.json();
			pendingCount = Number(data.total ?? 0);
		} else {
			pendingCount = null;
		}
	}

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await loadPendingCount();
		})();
		const interval = window.setInterval(() => void loadPendingCount(), 30000);
		const onFocus = () => void loadPendingCount();
		const onRefresh = () => void loadPendingCount();
		window.addEventListener('focus', onFocus);
		window.addEventListener('tauthub:media-pending-refresh', onRefresh);
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			window.clearInterval(interval);
			window.removeEventListener('focus', onFocus);
			window.removeEventListener('tauthub:media-pending-refresh', onRefresh);
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
		};
	});

	const tabs = $derived([
		{ value: 'directory', label: tr('Direktori') },
		{ value: 'archive', label: tr('Arsip') },
		{ value: 'pending', label: 'Pending' },
		{ value: 'recent', label: tr('Baru Ditambahkan') },
		{ value: 'intelligence', label: 'Intelligence' },
		// Tab pelanggaran: daftar media yang punya pelanggaran aktif (ringan/berat)
		// beserta riwayatnya, supaya admin bisa menindak tanpa mencari satu-satu.
		{ value: 'violations', label: tr('Pelanggaran') },
		// Isi tab ini hanya pengaturan template Bulk Import, jadi namanya
		// disesuaikan dengan isinya (value tetap 'settings').
		{ value: 'settings', label: tr('Bulk Import') },
		// Riwayat job pemeriksaan media (cek DA, trafik, verifikasi domain),
		// dipindah dari panel kuning di atas tabel Direktori.
		{ value: 'log', label: 'Log' }
	]);
</script>

<section class="w-full">
	<PageHeader
		title={tr("Media")}
		description={tr("Direktori = master data (harga, kredit, tier, integrasi, partner); Arsip = media diarsipkan / disembunyikan dari mitra; Intelligence = health score media.")}
	/>

	<div class="mt-4 flex flex-wrap gap-1 border-b" role="tablist">
		{#each tabs as t (t.value)}
			<button
				type="button"
				id={t.value === 'pending' ? 'tab-pending' : undefined}
				role="tab"
				aria-selected={tab === t.value}
				class="app-btn !min-h-8 !rounded-b-none !px-3.5 !text-sm"
				class:app-btn-primary={tab === t.value}
				class:app-btn-soft={tab !== t.value}
				onclick={() => (tab = t.value)}
			>
				{t.label}
				{#if t.value === 'pending' && pendingCount != null && pendingCount > 0}
					<span
						class="ml-1.5 inline-flex min-w-[18px] items-center justify-center rounded-full px-1 text-[11px] leading-4"
						style="background-color: var(--destructive); color: white; position: relative; top: -6px;"
						title={`${pendingCount} media menunggu review`}
					>
						{pendingCount > 99 ? '99+' : pendingCount}
					</span>
				{/if}
			</button>
		{/each}
	</div>

	<div class="mt-4">
		{#if tab === 'directory'}
			<MediaDirectory onPendingRefresh={loadPendingCount} />
		{:else if tab === 'archive'}
			<MediaArchive />
		{:else if tab === 'pending'}
			<MediaPending />
		{:else if tab === 'recent'}
			<MediaRecent />
		{:else if tab === 'intelligence'}
			<MediaIntelligenceTab />
		{:else if tab === 'violations'}
			<MediaViolationsTab />
		{:else if tab === 'settings'}
			<MediaSettings />
		{:else if tab === 'log'}
			<MediaLogTab />
		{/if}
	</div>
</section>
