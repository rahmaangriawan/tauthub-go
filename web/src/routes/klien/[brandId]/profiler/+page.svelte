<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// /profiler adalah perutean + gerbang: klik tombol "Profiler" dulu dimintai
	// pilih cara masuk — (1) Chat Brief dengan Tubi AI, atau (2) Instan Brief
	// (isi template agensi lalu unggah, langsung sinkron ke tab Media). Nanti
	// perutean ke brief terbaru / membuat "Brief pertama" bila belum ada.
	// Pilihan Gerbang 2 dikirim sebagai ?gate=instan dan dibaca sekali oleh
	// profil workspace (tidak disimpan — deep-link langsung ke URL huruf tokoh
	// berkas tidak melintasi gerbang ini).
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { ArrowRight, FileUp, LoaderCircle, MessageSquare } from '@lucide/svelte';
	import PageTitle from '$lib/components/page-title.svelte';
	import GateTemplateList from '$lib/components/gate-template-list.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { getBrandUrlId } from '$lib/brand-url.svelte';

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);
	const apiBase = $derived(`/v1/brands/${brandId}/profiler`);

	let error = $state('');
	// Popup gerbang: tertunda sampai daftar brief selesai dimuat (butuh target),
	// lalu update dua pilihan. `routed` menutup alur antar pilihan.
	type GateKind = 'chat' | 'instan';
	let gateOpen = $state(false);
	let routing = $state(false);

	type ProfileRow = { id: string; short_code: string | null };

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await pickTarget();
			gateOpen = !error;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	// Muat/buat brief target dulu (tanpa pindah halaman); hasilnya disimpan
	// untuk tombol gerbang, supaya popup tak pernah klik ke tujuan yang
	// tidak ada.
	let targetUrl = $state<string | null>(null);

	async function pickTarget() {
		const res = await apiRequest(`${apiBase}/profiles`);
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal memuat daftar brief.'));
			return;
		}
		const data = (await res.json()) as { profiles?: Array<{ id: string; short_code: string | null }> };
		const rows = data.profiles ?? [];
		if (rows.length === 0) {
			const created = await apiRequest(`${apiBase}/profiles`, {
				method: 'POST',
				body: JSON.stringify({ name: tr('Brief pertama') })
			});
			if (!created.ok) {
				error = await readApiError(created, tr('Gagal membuat brief pertama.'));
				return;
			}
			const item = (await created.json()) as { id: string; short_code: string | null };
			targetUrl = target(item.id, item.short_code);
			return;
		}
		// Server memilah updated_at DESC, jadi baris pertama = brief terakhir disentuh.
		const last = rows[0];
		targetUrl = target(last.id, last.short_code);
	}

	function enterGate(gate: GateKind) {
		if (!targetUrl || routing) return;
		routing = true;
		void goto(gate === 'instan' ? `${targetUrl}?gate=instan` : targetUrl, { replaceState: true });
	}

	function target(id: string, shortCode: string | null) {
		return `/klien/${brandUrl}/profiler/${shortCode || id}`;
	}
</script>

<PageTitle title="Profiler" />

{#if gateOpen && targetUrl}
	<!-- Gerbang: pilih cara masuk Taut Brief -->
	<div class="gate-wrap">
		<div class="gate-card" role="dialog" aria-modal="true" aria-label={tr('Pilih cara masuk')}>
			<h1 class="gate-title">{tr('Masuk Taut Brief')}</h1>
			<p class="gate-sub">{tr('Gunakan gerbang mana untuk brand ini?')}</p>
			<div class="gate-options">
				<button class="gate-option" type="button" disabled={routing} onclick={() => enterGate('chat')}>
					<span class="gate-icon"><MessageSquare size={28} strokeWidth={1.6} /></span>
					<span class="gate-name">{tr('Chat Brief')}</span>
					<span class="gate-desc">{tr('Isi Entity Core bersama Tubi lewat percakapan bertahap hingga lengkap.')}</span>
					<span class="gate-cta">{tr('Mulai percakapan')} <ArrowRight size={14} /></span>
				</button>
				<button class="gate-option" type="button" disabled={routing} onclick={() => enterGate('instan')}>
					<span class="gate-icon"><FileUp size={28} strokeWidth={1.6} /></span>
					<span class="gate-name">{tr('Instan Brief')}</span>
					<span class="gate-desc">{tr('Unduh template, isi bersama tim, lalu unggah dokumen atau gambar untuk diringkas otomatis.')}</span>
					<span class="gate-cta">{tr('Gunakan template')} <ArrowRight size={14} /></span>
				</button>
			</div>
			<!-- Opsi sekunder: brief baru dari template yang sudah disimpan. -->
			<GateTemplateList {brandId} {brandUrl} disabled={routing} />
			{#if routing}
				<p class="gate-loading"><LoaderCircle size={14} class="animate-spin" /> {tr('Membuka brief…')}</p>
			{/if}
		</div>
	</div>
{:else}
	<div class="flex items-center gap-2 py-10 text-sm text-muted-foreground">
		{#if error}
			<span style="color: var(--destructive);">{error}</span>
			<a class="app-btn app-btn-soft ml-2" href={`/klien/${brandUrl}/campaigns`}>{tr('Kembali ke kampanye')}</a>
		{:else}
			<LoaderCircle size={18} class="animate-spin" />
			<span>{tr('Membuka brief terakhir…')}</span>
		{/if}
	</div>
{/if}

<style>
	.gate-wrap {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 60dvh;
		padding: 24px 16px;
	}
	.gate-card {
		width: 100%;
		max-width: 1080px;
		padding: 28px 28px 24px;
		border: 1px solid var(--border, #e2e8f0);
		border-radius: 16px;
		background: var(--card, #fff);
		box-shadow: 0 16px 40px rgba(15, 23, 42, 0.08);
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
	/* Dua gerbang sejajar bersebelahan (menginsa satu kolom di layar sempit). */
	.gate-options {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 12px;
	}
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
	.gate-option:disabled { opacity: 0.6; cursor: wait; }
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
</style>
