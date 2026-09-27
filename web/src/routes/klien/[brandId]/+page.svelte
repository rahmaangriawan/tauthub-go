<script lang="ts">
	// Beranda klien — struktur parity section overview: hero, KPI wallet,
	// AI Insights, Akses Cepat Keuangan, antrean publikasi, panduan.
	import { page } from '$app/state';
	import {
		ArrowUpRight, BookOpen, Coins, CreditCard, Eye, FileText, Lock, Megaphone,
		MousePointerClick, Receipt, ShieldCheck, Sparkles, Wallet
	} from '@lucide/svelte';
	import { apiRequest, ensureValidSession, getStoredSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { swr } from '$lib/swr-cache.svelte';

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
	const jangkauan = $derived(d?.trends?.publications?.reduce((a: number, b: number) => a + b, 0) ?? 0);
	const fmtID = (n: number) => n.toLocaleString('id-ID');

	const kpi = $derived([
		{ label: 'SALDO TERSEDIA', value: saldo, unit: 'Kredit', note: 'Siap digunakan untuk publikasi', icon: Wallet },
		{ label: 'KREDIT TERTAHAN', value: reserved, unit: 'Kredit', note: 'Mengamankan pesanan aktif', icon: Lock },
		{ label: 'ESTIMASI JANGKAUAN (BULAN INI)', value: jangkauan, unit: '', note: 'Terhitung dari publikasi bulan berjalan', icon: Eye },
		{ label: 'TOTAL KLIK (SMART LINK)', value: 0, unit: '', note: 'Konversi tautan artikel aktif', icon: MousePointerClick }
	]);

	const statusLabel: Record<string, string> = {
		pending: 'Menunggu Mitra',
		processing: 'Diproses',
		published: 'Terpublikasi',
		failed: 'Gagal',
		cancelled: 'Dibatalkan',
		queued: 'Diantrekan',
		ready: 'Siap'
	};
	const statusTone: Record<string, string> = {
		published: 'success',
		ready: 'success',
		failed: 'danger',
		cancelled: 'muted',
		pending: 'warning',
		processing: 'accent',
		queued: 'accent'
	};
</script>

<div class="mx-auto max-w-6xl">
	<section class="relative overflow-hidden rounded-xl px-8 py-7 text-white" style="background: linear-gradient(102deg, #1c3fb0 0%, var(--primary) 62%);">
		<div class="flex flex-wrap items-start justify-between gap-6">
			<div class="max-w-xl">
				<h1 class="text-2xl font-bold">Selamat datang kembali, {userName}!! 👋</h1>
				<p class="mt-2 text-sm leading-relaxed text-white/85">
					Apa target distribusi PR Anda hari ini? Jelajahi katalog media kami secara manual
					atau biarkan Agen AI Pintar menyusun strategi dan menulis rilis pers untuk Anda.
				</p>
				<div class="mt-5 flex flex-wrap gap-2.5">
					<a
						href={`/klien/${brandId}/media`}
						class="inline-flex items-center gap-2 rounded-lg bg-white px-4 py-2 text-sm font-semibold"
						style="color: var(--primary);"
					>
						<Megaphone size={15} /> Buka Katalog Media
					</a>
					<a
						href={`/klien/${brandId}/campaigns`}
						class="inline-flex items-center gap-2 rounded-lg border border-white/45 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-white/10"
					>
						<Sparkles size={15} /> Buat Kampanye
					</a>
				</div>
			</div>
			<div class="rounded-xl border border-white/25 bg-white/10 px-5 py-4 text-center">
				<div class="mx-auto flex h-9 w-9 items-center justify-center rounded-full bg-emerald-400">
					<ShieldCheck size={18} class="text-white" />
				</div>
				<p class="mt-2 text-[10px] font-semibold tracking-wide text-white/80">STATUS KEAMANAN</p>
				<p class="text-sm font-bold">100% Lolos Audit</p>
			</div>
		</div>
	</section>

	{#if overview.error}
		<p class="app-card mt-4 border-l-4 px-4 py-3 text-sm text-destructive">
			Gagal memuat ringkasan: {overview.error.message}. Periksa koneksi lalu muat ulang.
		</p>
	{/if}

	<section class="mt-5 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
		{#each kpi as card (card.label)}
			<div class="app-card p-4">
				<div class="flex items-start justify-between gap-2">
					<p class="text-[11px] font-semibold tracking-wide text-muted-foreground">{card.label}</p>
					<card.icon size={16} style="color: var(--primary);" />
				</div>
				<p class="mt-2.5 text-2xl font-bold tabular-nums">
					{fmtID(card.value)}
					{#if card.unit}<span class="ml-1 text-sm font-medium text-muted-foreground">{card.unit}</span>{/if}
				</p>
				<p class="mt-1 text-xs text-muted-foreground">{card.note}</p>
			</div>
		{/each}
	</section>

	<section class="mt-4 grid gap-4 lg:grid-cols-3">
		<div class="app-card flex items-start gap-4 p-5 lg:col-span-2">
			<div class="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl text-primary-foreground" style="background-color: var(--primary);">
				<Sparkles size={20} />
			</div>
			<div class="min-w-0">
				<p class="flex flex-wrap items-center gap-2 font-semibold">
					Tauthub AI Insights
					<span class="rounded-full px-2 py-0.5 text-[10px] font-bold" style="background-color: var(--accent); color: var(--accent-foreground);">AGENT 2 &amp; 4</span>
				</p>
				<p class="mt-1.5 text-sm leading-relaxed text-muted-foreground">
					{#if d && (d.campaigns?.length ?? 0) > 0}
						{d.campaigns.length} kampanye terdaftar dan {d.publications?.length ?? 0} publikasi tercatat.
						Gunakan tabel antrean di bawah untuk memantau status terbaru tiap media.
					{:else}
						Belum ada kampanye aktif. Mulai dari Katalog Media atau buat kampanye pertama Anda —
						Agen AI akan menyusun strategi dan menulis rilis pers secara otomatis.
					{/if}
				</p>
			</div>
		</div>

		<div class="app-card p-5">
			<div class="flex items-center justify-between">
				<p class="font-semibold">Akses Cepat Keuangan</p>
				<CreditCard size={16} style="color: var(--primary);" />
			</div>
			<p class="mt-3 text-[11px] font-semibold tracking-wide text-muted-foreground">TOTAL SALDO AKTIF</p>
			<p class="text-2xl font-bold tabular-nums">
				{fmtID(saldo)} <span class="text-sm font-medium text-muted-foreground">Kredit</span>
			</p>
			<a href={`/klien/${brandId}/credits`} class="app-btn app-btn-primary mt-3 w-full">Top Up Saldo</a>
			<div class="mt-2.5 grid grid-cols-2 gap-2 text-xs font-medium">
				<a class="app-btn app-btn-soft !text-xs" href={`/klien/${brandId}/credits`}>
					<Receipt size={13} /> Riwayat
				</a>
				<a class="app-btn app-btn-soft !text-xs" href={`/klien/${brandId}/invoices`}>
					<FileText size={13} /> Klaim PPh 23
				</a>
			</div>
		</div>
	</section>

	<section class="mt-4 grid gap-4 lg:grid-cols-3">
		<div class="app-card p-5 lg:col-span-2">
			<div class="flex items-center justify-between gap-3">
				<h2 class="font-semibold">Antrean &amp; Status Publikasi Terkini</h2>
				<a class="inline-flex items-center gap-1 text-xs font-medium" style="color: var(--primary);" href={`/klien/${brandId}/publications`}>
					Lihat semua <ArrowUpRight size={13} />
				</a>
			</div>
			<div class="mt-3 overflow-x-auto">
				<table class="w-full text-sm">
					<thead>
						<tr class="border-b text-left text-[11px] tracking-wide text-muted-foreground">
							<th class="py-2 pr-4 font-semibold">NAMA KAMPANYE / ARTIKEL</th>
							<th class="py-2 pr-4 font-semibold">MEDIA</th>
							<th class="py-2 pr-4 font-semibold">STATUS (QC)</th>
							<th class="py-2 text-right font-semibold">KREDIT</th>
						</tr>
					</thead>
					<tbody>
						{#each pubQueue as pub (pub.id)}
							{@const tone = statusTone[pub.status] ?? 'muted'}
							<tr class="border-b last:border-0">
								<td class="py-2.5 pr-4">{pub.document_title ?? '—'}</td>
								<td class="py-2.5 pr-4 text-muted-foreground">{pub.media_name ?? '—'}</td>
								<td class="py-2.5 pr-4">
									<span
										class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-semibold"
										style={tone === 'success'
											? 'background-color: var(--success-soft); color: var(--success);'
											: tone === 'warning'
												? 'background-color: var(--warning-soft); color: var(--warning);'
												: tone === 'danger'
													? 'background-color: color-mix(in srgb, var(--destructive) 12%, transparent); color: var(--destructive);'
													: tone === 'accent'
														? 'background-color: var(--accent); color: var(--accent-foreground);'
														: 'background-color: var(--muted); color: var(--muted-foreground);'}
									>
										{statusLabel[pub.status] ?? pub.status}
									</span>
								</td>
								<td class="py-2.5 text-right tabular-nums">{fmtID(pub.client_price ?? 0)}</td>
							</tr>
						{:else}
							<tr>
								<td colspan="4" class="py-8 text-center text-muted-foreground">
									Belum ada publikasi. Antrean terisi setelah kampanye berjalan.
									<a class="ml-1 underline" href={`/klien/${brandId}/campaigns`}>Buat kampanye</a>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</div>

		<div class="app-card p-5">
			<div class="flex items-center justify-between">
				<p class="font-semibold">Panduan &amp; Bantuan</p>
				<BookOpen size={16} style="color: var(--primary);" />
			</div>
			<a href="/docs" class="mt-3 flex items-start gap-3 rounded-lg border p-3 transition-colors hover:bg-secondary">
				<BookOpen size={18} class="mt-0.5 shrink-0" style="color: var(--primary);" />
				<div>
					<p class="text-sm font-semibold">Cara Kerja Tauthub</p>
					<p class="text-xs text-muted-foreground">Alur dari brief, distribusi, sampai laporan kinerja.</p>
				</div>
			</a>
			<div class="mt-2 space-y-2 text-xs text-muted-foreground">
				<p class="flex items-center gap-2"><Coins size={13} /> Kredit dipotong saat publikasi dieksekusi.</p>
				<p class="flex items-center gap-2"><ShieldCheck size={13} /> Semua artikel melewati gate moderasi.</p>
			</div>
		</div>
	</section>
</div>
