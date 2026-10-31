<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Tab "Pelanggaran" di /admin/media.
	//
	// Menampilkan media yang punya pelanggaran AKTIF (ringan/berat) beserta
	// riwayat lengkapnya, plus media yang sedang disuspend. Tujuannya supaya
	// admin bisa menindak dari satu tempat tanpa mencari media satu per satu di
	// direktori.
	//
	// Sumber data: GET /v1/platform/media?scope=all (semua status kecuali hidden)
	// lalu disaring di klien berdasarkan violation_level dan is_suspended. Untuk
	// jumlah media yang wajar (ratusan) satu request sudah cukup; kalau nanti
	// perlu filter server-side, endpoint daftar bisa diberi parameter
	// violation=ringan|berat.
	import { onMount } from 'svelte';
	import { Search, ShieldAlert } from '@lucide/svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import { errMessage, isAccessError, mountWithSession, platformRequest, type Media } from './_shared';

	type ViolationRow = {
		id: string;
		level: string;
		note: string | null;
		created_at: string;
		revoked_at: string | null;
		actor_email: string | null;
	};

	let rows = $state<Media[]>([]);
	let loading = $state(true);
	let error = $state('');
	let search = $state('');
	/** Filter tingkat: '' = semua yang punya pelanggaran, 'ringan' | 'berat'. */
	let levelFilter = $state<'' | 'ringan' | 'berat'>('');
	/** Riwayat pelanggaran per media (dimuat saat baris dibuka). */
	let history = $state<Record<string, ViolationRow[]>>({});
	let openId = $state<string | null>(null);
	let historyLoading = $state<string | null>(null);

	let ready = $state(false);
	onMount(() => mountWithSession(async () => (ready = true)));

	$effect(() => {
		if (!ready) return;
		void load();
	});

	async function load() {
		error = '';
		loading = true;
		try {
			const data = await platformRequest<{ media?: Media[]; items?: Media[] }>('/media?scope=all&limit=500');
			rows = data.media ?? data.items ?? [];
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			loading = false;
		}
	}

	/** Media yang perlu perhatian: punya pelanggaran aktif atau disuspend. */
	const flagged = $derived(
		rows.filter((m) => (m.violation_level ?? 'none') !== 'none' || m.is_suspended)
	);

	const filtered = $derived(
		flagged.filter((m) => {
			if (levelFilter && (m.violation_level ?? 'none') !== levelFilter) return false;
			if (!search.trim()) return true;
			const q = search.trim().toLowerCase();
			return (
				m.name.toLowerCase().includes(q) ||
				(m.website_url ?? '').toLowerCase().includes(q) ||
				(m.partner_name ?? '').toLowerCase().includes(q)
			);
		})
	);

	async function toggleHistory(m: Media) {
		if (openId === m.id) {
			openId = null;
			return;
		}
		openId = m.id;
		if (history[m.id]) return;
		historyLoading = m.id;
		try {
			const data = await platformRequest<{ items: ViolationRow[] }>(`/media/${m.id}/violations`);
			history = { ...history, [m.id]: data.items ?? [] };
		} catch {
			history = { ...history, [m.id]: [] };
		} finally {
			historyLoading = null;
		}
	}

	const levelTone = (level: string) => (level === 'berat' ? 'danger' : 'warning');
	const levelLabel = (level: string) => (level === 'berat' ? tr('Berat') : level === 'ringan' ? tr('Ringan') : tr('Tidak ada'));
</script>

{#if error && !isAccessError(error)}
	<div class="app-card mt-4 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>
{/if}

<div class="mt-4 flex flex-wrap items-center gap-2">
	<label class="app-input flex min-w-[240px] flex-1 items-center gap-2 !py-1.5">
		<Search size={15} class="text-muted-foreground" />
		<input
			class="w-full bg-transparent text-sm outline-none"
			placeholder={tr('Cari nama media, domain, atau mitra…')}
			aria-label={tr('Cari media bermasalah')}
			bind:value={search}
		/>
	</label>
	<div class="flex gap-1">
		<button
			type="button"
			class="app-btn !min-h-8 !px-3 !text-xs"
			class:app-btn-primary={levelFilter === ''}
			class:app-btn-soft={levelFilter !== ''}
			onclick={() => (levelFilter = '')}>{tr('Semua')}</button
		>
		<button
			type="button"
			class="app-btn !min-h-8 !px-3 !text-xs"
			class:app-btn-primary={levelFilter === 'ringan'}
			class:app-btn-soft={levelFilter !== 'ringan'}
			onclick={() => (levelFilter = 'ringan')}>{tr('Ringan')}</button
		>
		<button
			type="button"
			class="app-btn !min-h-8 !px-3 !text-xs"
			class:app-btn-primary={levelFilter === 'berat'}
			class:app-btn-soft={levelFilter !== 'berat'}
			onclick={() => (levelFilter = 'berat')}>{tr('Berat')}</button
		>
	</div>
</div>

{#if loading}
	<p class="mt-6 text-sm text-muted-foreground">{tr('Memuat data pelanggaran…')}</p>
{:else if flagged.length === 0}
	<div class="app-card mt-4 flex flex-col items-center gap-2 px-6 py-12 text-center">
		<div class="flex h-11 w-11 items-center justify-center rounded-xl" style="background-color: var(--success-soft, #eaf8f1); color: var(--success, #16794b);">
			<ShieldAlert size={22} />
		</div>
		<p class="text-sm font-medium">{tr('Tidak ada media bermasalah')}</p>
		<p class="max-w-md text-xs text-muted-foreground">
			{tr('Belum ada media dengan pelanggaran aktif atau penangguhan domain. Catat pelanggaran lewat tombol Edit pada baris media di tab Direktori.')}
		</p>
	</div>
{:else if filtered.length === 0}
	<p class="mt-6 text-sm text-muted-foreground">{tr('Tidak ada media yang cocok dengan filter ini.')}</p>
{:else}
	<div class="app-card mt-4 overflow-x-auto">
		<table class="w-full text-sm" style="min-width: 900px;">
			<thead>
				<tr class="text-left text-xs text-muted-foreground">
					<th class="px-4 py-2.5 font-medium min-w-[240px]">{tr('Media')}</th>
					<th class="px-4 py-2.5 font-medium">{tr('Mitra')}</th>
					<th class="px-4 py-2.5 font-medium">{tr('Pelanggaran')}</th>
					<th class="px-4 py-2.5 font-medium">{tr('Status domain')}</th>
					<th class="px-4 py-2.5 font-medium">{tr('Artikel terbit')}</th>
					<th class="px-4 py-2.5 font-medium">{tr('Riwayat')}</th>
				</tr>
			</thead>
			<tbody>
				{#each filtered as m (m.id)}
					<tr class="border-t">
						<td class="px-4 py-3">
							<p class="font-medium">{m.name}</p>
							<p class="mt-0.5 text-xs text-muted-foreground">{m.website_url ?? '—'}</p>
						</td>
						<td class="px-4 py-3 text-xs">{m.partner_name ?? '—'}</td>
						<td class="px-4 py-3">
							<Badge tone={levelTone(m.violation_level ?? 'none')} label={levelLabel(m.violation_level ?? 'none')} />
						</td>
						<td class="px-4 py-3">
							{#if m.is_suspended}
								<span title={m.suspended_reason ?? undefined}>
									<Badge tone="danger" label={tr('Disuspend')} />
								</span>
							{:else}
								<Badge tone="success" label={tr('Aktif')} />
							{/if}
						</td>
						<td class="px-4 py-3 tabular-nums">{m.published_count ?? 0}</td>
						<td class="px-4 py-3">
							<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void toggleHistory(m)}>
								{openId === m.id ? tr('Tutup') : tr('Lihat riwayat')}
							</button>
						</td>
					</tr>
					{#if openId === m.id}
						<tr class="border-t" style="background-color: var(--secondary);">
							<td colspan="6" class="px-4 py-3">
								{#if historyLoading === m.id}
									<p class="text-xs text-muted-foreground">{tr('Memuat riwayat…')}</p>
								{:else if (history[m.id] ?? []).length === 0}
									<p class="text-xs text-muted-foreground">{tr('Belum ada catatan pelanggaran.')}</p>
								{:else}
									<div class="space-y-1.5">
										{#each history[m.id] as v (v.id)}
											<div class="rounded-lg border px-2.5 py-1.5 text-xs" style="border-color: var(--border); background: var(--card);">
												<div class="flex items-center justify-between gap-2">
													<Badge tone={levelTone(v.level)} label={levelLabel(v.level)} />
													<span class="text-muted-foreground">{fmtDateTime(v.created_at)}</span>
												</div>
												{#if v.note}<p class="mt-1 leading-snug">{v.note}</p>{/if}
												<p class="mt-0.5 text-[11px] text-muted-foreground">
													{v.actor_email ?? '—'}{#if v.revoked_at} · {tr('dicabut')} {fmtDateTime(v.revoked_at)}{/if}
												</p>
											</div>
										{/each}
									</div>
								{/if}
								{#if m.is_suspended && m.suspended_reason}
									<p class="mt-2 text-xs" style="color: var(--warning, #a15c00);">
										<strong>{tr('Alasan suspend')}:</strong> {m.suspended_reason}
									</p>
								{/if}
							</td>
						</tr>
					{/if}
				{/each}
			</tbody>
		</table>
	</div>
	<p class="mt-2 text-xs text-muted-foreground">
		{filtered.length} {tr('dari')} {flagged.length} {tr('media bermasalah')}
	</p>
{/if}
