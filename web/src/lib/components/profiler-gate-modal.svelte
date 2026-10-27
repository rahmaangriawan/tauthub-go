<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Popup gerbang Taut Brief — dipakai tombol "Profiler AI" di halaman
	// Kampanye. Menentukan target brief (yang terakhir disentuh, atau buat
	// "Brief pertama" kalau brand belum punya), lalu menawarkan dua cara
	// masuk: Chat Brief (alur chat) atau Instan Brief (template + unggah,
	// ?gate=instan). Halaman perutean /profiler memakai gaya kartu yang sama
	// untuk akses lewat URL langsung.
	import { ArrowRight, FileUp, LoaderCircle, MessageSquare, } from '@lucide/svelte';
	import { goto } from '$app/navigation';
	import { apiRequest, readApiError, ensureValidSession } from '$lib/auth';

	type Props = {
		open: boolean;
		brandId: string;
		brandUrl: string;
		onClose: () => void;
	};
	let { open, brandId, brandUrl, onClose }: Props = $props();

	const apiBase = $derived(`/v1/brands/${brandId}/profiler`);

	let error = $state('');
	/** 'idle' hanya sebentar; target null = belum siap masuk. */
	let targetUrl = $state<string | null>(null);
	let loading = $state(false);
	let routing = $state(false);
	let loadedFor = $state('');

	$effect(() => {
		// Muat target tiap kali popup dibuka untuk brand ini (sekali per buka).
		if (!open) {
			loadedFor = '';
			return;
		}
		if (loadedFor === brandId) return;
		loadedFor = brandId;
		void load();
	});

	async function load() {
		loading = true;
		error = '';
		targetUrl = null;
		try {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			const res = await apiRequest(`${apiBase}/profiles`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat daftar brief.')));
			const data = (await res.json()) as { profiles?: Array<{ id: string; short_code: string | null }> };
			const rows = data.profiles ?? [];
			if (rows.length === 0) {
				const created = await apiRequest(`${apiBase}/profiles`, {
					method: 'POST',
					body: JSON.stringify({ name: tr('Brief pertama') })
				});
				if (!created.ok) throw new Error(await readApiError(created, tr('Gagal membuat brief pertama.')));
				const item = (await created.json()) as { id: string; short_code: string | null };
				targetUrl = target(item.id, item.short_code);
			} else {
				// Server memilah updated_at DESC → baris pertama = terakhir disentuh.
				targetUrl = target(rows[0].id, rows[0].short_code);
			}
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal memuat daftar brief.');
		} finally {
			loading = false;
		}
	}

	function target(id: string, shortCode: string | null) {
		return `/klien/${brandUrl}/profiler/${shortCode || id}`;
	}

	function enterGate(gate: 'chat' | 'instan') {
		if (!targetUrl || routing) return;
		routing = true;
		void goto(gate === 'instan' ? `${targetUrl}?gate=instan` : targetUrl, { replaceState: true });
	}

	function retry() {
		loadedFor = '';
		void load();
		loadedFor = brandId;
	}
</script>

{#if open}
	<div class="gate-overlay" role="presentation">
		<button class="gate-backdrop" type="button" aria-label={tr('Tutup')} onclick={onClose}></button>
		<div class="gate-card" role="dialog" aria-modal="true" aria-label={tr('Pilih cara masuk')}>
			<h1 class="gate-title">{tr('Masuk Taut Brief')}</h1>
			<p class="gate-sub">{tr('Gunakan gerbang mana untuk brand ini?')}</p>

			{#if error}
				<div class="gate-error">
					<p>{error}</p>
					<div class="gate-error-actions">
						<button class="app-btn app-btn-soft !min-h-8 !text-xs" type="button" onclick={retry}>{tr('Coba lagi')}</button>
						<button class="app-btn app-btn-soft !min-h-8 !text-xs" type="button" onclick={onClose}>{tr('Tutup')}</button>
					</div>
				</div>
			{:else}
				<div class="gate-options" class:gate-options-loading={loading}>
					<button class="gate-option" type="button" disabled={loading || routing || !targetUrl} onclick={() => enterGate('chat')}>
						<span class="gate-icon"><MessageSquare size={28} strokeWidth={1.6} /></span>
						<span class="gate-name">{tr('Chat Brief')}</span>
						<span class="gate-desc">{tr('Isi Entity Core bersama Tubi lewat percakapan bertahap hingga lengkap.')}</span>
						<span class="gate-cta">{tr('Mulai percakapan')} <ArrowRight size={14} /></span>
					</button>
					<button class="gate-option" type="button" disabled={loading || routing || !targetUrl} onclick={() => enterGate('instan')}>
						<span class="gate-icon"><FileUp size={28} strokeWidth={1.6} /></span>
						<span class="gate-name">{tr('Instan Brief')}</span>
						<span class="gate-desc">{tr('Unduh template, isi bersama tim, lalu unggah dokumen atau gambar untuk diringkas otomatis.')}</span>
						<span class="gate-cta">{tr('Gunakan template')} <ArrowRight size={14} /></span>
					</button>
				</div>
				{#if loading || routing}
					<p class="gate-loading"><LoaderCircle size={14} class="animate-spin" /> {loading ? tr('Menyiapkan brief…') : tr('Membuka brief…')}</p>
				{/if}
			{/if}
		</div>
	</div>
{/if}

<style>
	.gate-overlay {
		position: fixed;
		inset: 0;
		z-index: 80;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 20px;
	}
	.gate-backdrop {
		position: absolute;
		inset: 0;
		background: rgba(15, 23, 42, 0.45);
		cursor: default;
	}
	.gate-card {
		position: relative;
		z-index: 1;
		width: 100%;
		max-width: 1080px;
		padding: 28px 28px 24px;
		border: 1px solid var(--border, #e2e8f0);
		border-radius: 18px;
		background: var(--card, #fff);
		box-shadow: 0 24px 60px rgba(15, 23, 42, 0.22);
	}
	.gate-title {
		margin: 0;
		text-align: center;
		font-size: 22px;
		font-weight: 700;
		letter-spacing: -0.01em;
		color: var(--foreground, #111827);
	}
	.gate-sub {
		margin: 6px 0 20px;
		text-align: center;
		font-size: 13.5px;
		color: var(--muted-foreground, #667085);
	}
	/* Dua gerbang sejajar; turun satu kolom di layar sempit. */
	.gate-options {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 12px;
	}
	.gate-options-loading { opacity: 0.75; }
	@media (max-width: 720px) {
		.gate-options { grid-template-columns: 1fr; }
	}
	.gate-option {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 8px;
		padding: 24px 26px 22px;
		text-align: left;
		border: 1px solid var(--border, #e2e8f0);
		border-radius: 14px;
		background: var(--card, #fff);
		cursor: pointer;
		transition: border-color 0.12s ease, background-color 0.12s ease, box-shadow 0.12s ease;
	}
	.gate-option:hover:not(:disabled) {
		border-color: color-mix(in srgb, var(--primary, #2b67ec) 45%, var(--border, #e2e8f0));
		background: color-mix(in srgb, var(--primary, #2b67ec) 4%, var(--card, #fff));
		box-shadow: 0 8px 20px rgba(43, 103, 236, 0.08);
	}
	.gate-option:disabled { opacity: 0.65; cursor: progress; }
	.gate-icon {
		display: inline-flex;
		align-items: center;
		color: var(--primary, #2b67ec);
		margin-bottom: 2px;
	}
	.gate-name {
		font-size: 18px;
		font-weight: 700;
		letter-spacing: -0.01em;
		color: var(--foreground, #111827);
	}
	.gate-desc {
		max-width: 460px;
		font-size: 13.5px;
		line-height: 1.6;
		color: var(--muted-foreground, #667085);
	}
	/* CTA di dasar kartu, rata kiri seperti referensi. */
	.gate-cta {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		margin-top: auto;
		padding-top: 10px;
		font-size: 13.5px;
		font-weight: 600;
		color: var(--primary, #2b67ec);
	}
	.gate-option:hover:not(:disabled) .gate-cta { text-decoration: underline; }
	.gate-loading {
		margin: 14px 0 0;
		display: inline-flex;
		align-items: center;
		gap: 6px;
		font-size: 12px;
		color: var(--muted-foreground, #667085);
	}
	.gate-error p {
		margin: 0;
		font-size: 13px;
		color: var(--destructive, #dc2626);
	}
	.gate-error-actions {
		margin-top: 10px;
		display: flex;
		gap: 8px;
	}
</style>
