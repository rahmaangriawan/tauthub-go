<script lang="ts">
	// Popup pengumuman: saat halaman dibuka dan ada pengumuman yang belum
	// dibaca, pengumuman terbaru muncul sekali. Menutupnya (X, Escape, backdrop,
	// atau tombol "Mengerti") menandainya dibaca, jadi tidak muncul lagi.
	// Riwayat lengkap tetap ada di tab "Pengumuman" pada lonceng/riwayat notifikasi.
	import { onMount } from 'svelte';
	import { Megaphone, X } from '@lucide/svelte';
	import { apiRequest } from '$lib/auth';
	import { tr } from '$lib/i18n/phrases';

	type Item = {
		id: string;
		title: string;
		body: string;
		audience: string;
		created_at: string | null;
		read_at: string | null;
	};

	let open = $state(false);
	let item = $state<Item | null>(null);
	let panel = $state<HTMLDivElement | null>(null);
	let closing = $state(false);

	const fmtDate = (v: string | null) =>
		v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '';

	onMount(() => {
		// Delay kecil supaya popup tidak berebut render dengan halaman itu sendiri.
		const timer = window.setTimeout(async () => {
			try {
				const res = await apiRequest('/v1/announcements');
				if (!res.ok) return;
				const data = (await res.json()) as { items?: Item[] };
				const unread = (data.items ?? []).filter((i) => !i.read_at);
				if (unread.length === 0) return;
				item = unread[0];
				open = true;
				panel?.focus();
			} catch {
				// Popup hanya penyapa; kegagalan jaringan diabaikan diam-diam.
			}
		}, 800);
		return () => window.clearTimeout(timer);
	});

	$effect(() => {
		if (!open) return;
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape') void close();
		};
		window.addEventListener('keydown', onKey);
		const previous = document.body.style.overflow;
		document.body.style.overflow = 'hidden';
		return () => {
			window.removeEventListener('keydown', onKey);
			document.body.style.overflow = previous;
		};
	});

	async function close() {
		if (!item || closing) return;
		closing = true;
		try {
			await apiRequest(`/v1/announcements/${item.id}/read`, { method: 'POST' });
		} catch {
			// Penandaan gagal: popup boleh muncul lagi di muat berikutnya.
		}
		open = false;
		item = null;
		closing = false;
	}
</script>

{#if open && item}
	<div class="fixed inset-0 z-[1200] flex items-center justify-center p-4" style="background-color: rgba(15, 23, 42, 0.45);" role="presentation" onclick={() => void close()} onkeydown={(e) => { if (e.key === 'Escape' || e.key === 'Enter' || e.key === ' ') void close(); else e.stopPropagation(); }}>
		<div
			bind:this={panel}
			class="app-card w-full max-w-md p-5"
			role="dialog"
			aria-modal="true"
			aria-label={tr('Pengumuman')}
			tabindex="-1"
			onclick={(e) => e.stopPropagation()}
		>
			<div class="flex items-start justify-between gap-3">
				<div class="flex items-center gap-3">
					<Megaphone size={18} style="color: var(--primary);" />
					<p class="text-xs font-bold uppercase tracking-[0.4px] text-muted-foreground">{tr('Pengumuman')}</p>
				</div>
				<button
					class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-muted-foreground hover:bg-secondary"
					type="button"
					aria-label={tr('Tutup')}
					onclick={() => void close()}
				>
					<X size={18} />
				</button>
			</div>
			<h2 class="mt-2 text-lg font-semibold leading-snug">{item.title}</h2>
			<p class="mt-1 text-xs text-muted-foreground">{fmtDate(item.created_at)}</p>
			<p class="mt-3 text-sm whitespace-pre-line">{item.body}</p>
			<div class="mt-4 flex justify-end">
				<button class="app-btn app-btn-primary" type="button" disabled={closing} onclick={() => void close()}>{tr('Mengerti')}</button>
			</div>
		</div>
	</div>
{/if}
