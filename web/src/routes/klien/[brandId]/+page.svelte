<script lang="ts">
	// Port section "Beranda" (overview) — hero, KPI wallet, AI Insights,
	// Akses Cepat Keuangan, Antrean publikasi, Panduan. Data dari endpoint
	// /overview (port read-only fase 6/7/8).
	import { apiRequest, ensureValidSession, getStoredSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { swr } from '$lib/swr-cache.svelte';
	import { page } from '$app/state';
	import {
		BookOpen, CreditCard, FileText, Flag, Globe, LineChart, Receipt, ShieldCheck, Wallet
	} from '@lucide/svelte';

	const brandId = $derived(page.params.brandId as string);

	const overview = swr<any>(
		() => (getStoredSession() ? `/v1/brands/${brandId}/overview` : null),
		async () => {
			const res = await apiRequest(`/v1/brands/${brandId}/overview`);
			if (!res.ok) throw new Error('gagal memuat overview');
			return res.json();
		}
	);

	$effect(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) window.location.href = '/login';
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	const d = $derived(overview.data);
	const saldo = $derived(d?.wallet?.available_tc ?? 0);
	const reserved = $derived(d?.wallet?.reserved_tc ?? 0);
	const userName = $derived(d?.user?.name ?? d?.user?.email?.split('@')[0] ?? '');
	const pubQueue = $derived((d?.publications ?? []).slice(0, 5));
	const hasPubQueue = $derived(pubQueue.length > 0);
	const fmtID = (n: number) => n.toLocaleString('id-ID');

	const statusLabel: Record<string, string> = {
		pending: 'Menunggu Mitra',
		processing: 'Diproses',
		published: 'Terpublikasi',
		failed: 'Gagal',
		cancelled: 'Dibatalkan',
		queued: 'Diantrekan',
		ready: 'Siap'
	};
</script>

<div class="mx-auto max-w-6xl">
	<!-- Hero -->
	<section class="relative overflow-hidden rounded-2xl bg-gradient-to-r from-[#1e3fae] via-[#2547c9] to-[#5b6cf0] px-8 py-8 text-white">
		<div class="absolute -right-10 -top-16 h-56 w-56 rounded-full bg-white/10 blur-2xl"></div>
		<div class="absolute right-24 top-6 text-white/70">✦</div>
		<div class="relative flex items-start justify-between gap-6">
			<div class="max-w-xl">
				<h1 class="text-2xl font-bold">Selamat datang kembali, {userName}!! 👋</h1>
				<p class="mt-2 text-sm leading-relaxed text-white/85">
					Apa target distribusi PR Anda hari ini? Jelajahi katalog media kami secara
					manual atau biarkan Agen AI Pintar menyusun strategi dan menulis rilis pers
					untuk Anda.
				</p>
				<div class="mt-4 flex gap-3">
					<a
						href={`/klien/${brandId}/media`}
						class="flex items-center gap-2 rounded-lg bg-white px-4 py-2 text-sm font-semibold text-[#1d4ed8]"
					>
						<Globe size={15} /> Buka Katalog Media
					</a>
					<a
						href={`/klien/${brandId}/campaigns`}
						class="flex items-center gap-2 rounded-lg border border-white/40 px-4 py-2 text-sm font-semibold text-white hover:bg-white/10"
					>
						<Flag size={15} /> Buat Kampanye
					</a>
				</div>
			</div>
			<div class="rounded-xl bg-white/15 px-5 py-4 text-center backdrop-blur">
				<div class="mx-auto flex h-9 w-9 items-center justify-center rounded-full bg-emerald-400/90">
					<ShieldCheck size={18} class="text-white" />
				</div>
				<p class="mt-2 text-[10px] font-semibold tracking-wide text-white/80">STATUS KEAMANAN</p>
				<p class="text-sm font-bold">100% Lolos Audit</p>
			</div>
		</div>
	</section>

	{#if overview.error}
		<p class="mt-4 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{overview.error.message}</p>
	{/if}

	<!-- KPI cards -->
	<section class="mt-5 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
		<div class="rounded-xl border border-border bg-card p-4">
			<div class="flex items-center justify-between">
				<p class="text-xs font-semibold tracking-wide text-slate-400">SALDO TERSEDIA</p>
				<Wallet size={16} class="text-[#1d4ed8]" />
			</div>
			<p class="mt-2 text-2xl font-bold">{fmtID(saldo)} <span class="text-sm font-medium text-muted-foreground">Kredit</span></p>
			<p class="mt-1 text-xs text-muted-foreground">Siap digunakan untuk publikasi</p>
		</div>
		<div class="rounded-xl border border-border bg-card p-4">
			<div class="flex items-center justify-between">
				<p class="text-xs font-semibold tracking-wide text-slate-400">KREDIT TERTAHAN</p>
				<ShieldCheck size={16} class="text-[#1d4ed8]" />
			</div>
			<p class="mt-2 text-2xl font-bold">{fmtID(reserved)} <span class="text-sm font-medium text-muted-foreground">Kredit</span></p>
			<p class="mt-1 text-xs text-muted-foreground">Mengamankan pesanan aktif</p>
		</div>
		<div class="rounded-xl border border-border bg-card p-4">
			<div class="flex items-center justify-between">
				<p class="text-xs font-semibold tracking-wide text-slate-400">ESTIMASI JANGKAUAN (BULAN INI)</p>
				<LineChart size={16} class="text-[#1d4ed8]" />
			</div>
			<p class="mt-2 text-2xl font-bold">0</p>
			<p class="mt-1 text-xs text-muted-foreground">—</p>
		</div>
		<div class="rounded-xl border border-border bg-card p-4">
			<div class="flex items-center justify-between">
				<p class="text-xs font-semibold tracking-wide text-slate-400">TOTAL KLIK (SMART LINK)</p>
				<FileText size={16} class="text-[#1d4ed8]" />
			</div>
			<p class="mt-2 text-2xl font-bold">0</p>
			<p class="mt-1 text-xs text-muted-foreground">Konversi tautan artikel aktif</p>
		</div>
	</section>

	<!-- Insights + Keuangan -->
	<section class="mt-5 grid gap-4 lg:grid-cols-3">
		<div class="relative overflow-hidden rounded-xl border border-border bg-card p-5 lg:col-span-2">
			<div class="absolute -right-8 -top-8 h-32 w-32 rounded-full bg-blue-100/60 blur-xl"></div>
			<div class="relative flex items-start gap-4">
				<div class="flex h-11 w-11 items-center justify-center rounded-xl bg-[#1d4ed8] text-white">
					<LineChart size={20} />
				</div>
				<div class="flex-1">
					<p class="font-semibold">
						Tauthub AI Insights
						<span class="ml-2 rounded-full bg-violet-100 px-2 py-0.5 text-[10px] font-bold text-violet-700">AGENT 2 &amp; 4</span>
					</p>
					<p class="mt-1 text-sm text-muted-foreground">
						{#if d && (d.campaigns?.length ?? 0) > 0}
							Kinerja kampanye Anda memunjukkan {d.campaigns.length} kampanye terdaftar. Publikasi
							aktif tercatat {d.publications?.length ?? 0} item — pantau progresnya di tabel bawah.
						{:else}
							Belum ada kampanye aktif. Mulai dari Katalog Media atau buat kampanye pertama Anda —
							Agen AI akan menyusun strategi dan menulis rilis pers secara otomatis.
						{/if}
					</p>
				</div>
			</div>
		</div>

		<div class="rounded-xl border border-border bg-card p-5">
			<div class="flex items-center justify-between">
				<p class="font-semibold">Akses Cepat Keuangan</p>
				<CreditCard size={16} class="text-[#1d4ed8]" />
			</div>
			<p class="mt-3 text-xs font-semibold tracking-wide text-slate-400">TOTAL SALDO AKTIF</p>
			<p class="text-2xl font-bold">{fmtID(saldo)} <span class="text-sm font-medium text-muted-foreground">Kredit</span></p>
			<button class="mt-3 w-full rounded-lg bg-[#1d4ed8] py-2 text-sm font-semibold text-white hover:opacity-90">
				Top Up Saldo
			</button>
			<div class="mt-3 grid grid-cols-2 gap-2 text-xs font-medium">
				<a class="flex items-center gap-1.5 rounded-lg border border-border px-2 py-2 hover:bg-accent" href={`/klien/${brandId}/credits`}>
					<Receipt size={13} /> Riwayat Transaksi
				</a>
				<a class="flex items-center gap-1.5 rounded-lg border border-border px-2 py-2 hover:bg-accent" href={`/klien/${brandId}/invoices`}>
					<FileText size={13} /> Klaim PPh 23
				</a>
			</div>
		</div>
	</section>

	<!-- Antrean publikasi + Panduan -->
	<section class="mt-5 grid gap-4 lg:grid-cols-3">
		<div class="rounded-xl border border-border bg-card p-5 lg:col-span-2">
			<h2 class="font-semibold">Antrean &amp; Status Publikasi Terkini</h2>
			<div class="mt-3 overflow-x-auto">
				<table class="w-full text-sm">
					<thead>
						<tr class="border-b border-border text-left text-xs text-muted-foreground">
							<th class="py-2 pr-4 font-medium">Nama Kampanye / Artikel</th>
							<th class="py-2 pr-4 font-medium">Media</th>
							<th class="py-2 pr-4 font-medium">Status (QC)</th>
							<th class="py-2 font-medium">Kredit</th>
						</tr>
					</thead>
					<tbody>
						{#each pubQueue as pub (pub.id)}
							<tr class="border-b border-border/50 last:border-0">
								<td class="py-2.5 pr-4">{pub.document_title ?? '—'}</td>
								<td class="py-2.5 pr-4">{pub.media_name ?? '—'}</td>
								<td class="py-2.5 pr-4">
									<span class="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2 py-0.5 text-xs font-semibold text-amber-700">
										● {statusLabel[pub.status] ?? pub.status}
									</span>
								</td>
								<td class="py-2.5">{fmtID(pub.client_price ?? 0)}</td>
							</tr>
						{:else}
							<tr><td colspan="4" class="py-6 text-center text-muted-foreground">
								Belum ada publikasi — antrean akan terisi setelah kampanye berjalan.
							</td></tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>

		<div class="rounded-xl border border-border bg-card p-5">
			<div class="flex items-center justify-between">
				<p class="font-semibold">Panduan &amp; Bantuan</p>
				<BookOpen size={16} class="text-[#1d4ed8]" />
			</div>
			<a href="/docs" class="mt-3 flex items-start gap-3 rounded-lg border border-border p-3 hover:bg-accent">
				<BookOpen size={18} class="mt-0.5 text-[#1d4ed8]" />
				<div>
					<p class="text-sm font-semibold">Cara Kerja Tauthub</p>
					<p class="text-xs text-muted-foreground">Pelajari alur kerja dari A hingga Z.</p>
				</div>
			</a>
		</div>
	</section>
</div>
