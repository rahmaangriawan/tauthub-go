<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Alamat email mati (hard bounce) dan komplain spam. Alamat bertipe "mati"
	// tidak dikirimi email lagi; admin bisa menghapusnya bila pemilik sudah
	// memperbaiki kotak masuknya.
	import { onMount } from 'svelte';
	import { Search } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';

	type Item = { email: string; reason: string; detail: string; created_at: string };
	type Page = { total: number; items: Item[] };

	const PAGE_SIZE = 20;

	let loading = $state(true);
	let error = $state('');
	let notice = $state('');
	let removing = $state('');
	let data = $state<Page>({ total: 0, items: [] });
	let offset = $state(0);
	let searchInput = $state('');
	let search = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load(0, '');
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load(nextOffset: number, q: string) {
		loading = true;
		error = '';
		const params = new URLSearchParams({ offset: String(nextOffset), limit: String(PAGE_SIZE) });
		if (q) params.set('search', q);
		try {
			const res = await apiRequest(`/v1/platform/operations/email-suppressions?${params.toString()}`);
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			data = await res.json();
			offset = nextOffset;
		} catch {
			error = tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		} finally {
			loading = false;
		}
	}

	function applySearch() {
		search = searchInput.trim();
		void load(0, search);
	}

	function reasonLabel(reason: string): string {
		if (reason === 'hard_bounce') return 'Alamat mati';
		if (reason === 'complaint') return 'Komplain spam';
		return reason;
	}

	async function remove(email: string) {
		if (!confirm(`Hapus ${email} dari daftar? Alamat ini akan boleh dikirimi email lagi.`)) return;
		removing = email;
		error = '';
		notice = '';
		try {
			const res = await apiRequest('/v1/platform/operations/email-suppressions', {
				method: 'DELETE',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ email })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal menghapus alamat.');
				return;
			}
			notice = `${email} dihapus dari daftar.`;
			await load(offset, search);
		} catch {
			error = tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		} finally {
			removing = '';
		}
	}
</script>

<section>
	<PageHeader
		title="Alamat Email Mati"
		description="Alamat yang memantul permanen atau mengomplain spam. Alamat mati tidak dikirimi email lagi supaya reputasi pengiriman terjaga."
	>
		<Badge tone="accent" label="{data.total.toLocaleString('id-ID')} alamat" />
	</PageHeader>

	<Feedback {loading} {error} loadingText="Memuat daftar…" />
	{#if notice}
		<p class="app-card mt-4 px-4 py-3 text-sm" role="status">{notice}</p>
	{/if}

	<form
		class="app-card mt-4 flex flex-wrap items-center gap-3 p-3"
		onsubmit={(e) => {
			e.preventDefault();
			applySearch();
		}}
	>
		<div class="relative min-w-[240px] flex-1">
			<Search size={14} class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
			<input class="app-input w-full pl-9" placeholder={tr('Cari alamat email...')} aria-label={tr('Cari alamat email')} bind:value={searchInput} />
		</div>
		<button class="app-btn app-btn-soft" type="submit" disabled={loading}>Cari</button>
		{#if search}
			<button
				class="app-btn app-btn-soft"
				type="button"
				disabled={loading}
				onclick={() => {
					searchInput = '';
					search = '';
					void load(0, '');
				}}>Reset</button
			>
		{/if}
	</form>

	<div class="app-card mt-4 overflow-x-auto">
		<table class="w-full text-sm" style="min-width: 760px;">
			<thead>
				<tr class="text-xs text-muted-foreground">
					<th class="px-4 py-2.5 text-left font-medium">Alamat</th>
					<th class="px-4 py-2.5 text-left font-medium">Jenis</th>
					<th class="px-4 py-2.5 text-left font-medium">Keterangan</th>
					<th class="px-4 py-2.5 text-left font-medium">Dicatat</th>
					<th class="px-4 py-2.5 text-right font-medium">Aksi</th>
				</tr>
			</thead>
			<tbody>
				{#each data.items as item (item.email)}
					<tr class="border-t">
						<td class="px-4 py-2.5">{item.email}</td>
						<td class="px-4 py-2.5">
							<Badge tone={item.reason === 'hard_bounce' ? 'danger' : 'neutral'} label={reasonLabel(item.reason)} />
						</td>
						<td class="px-4 py-2.5 text-muted-foreground">{item.detail || '-'}</td>
						<td class="px-4 py-2.5">{fmtDateTime(item.created_at)}</td>
						<td class="px-4 py-2.5 text-right">
							<button class="app-btn app-btn-soft" type="button" disabled={removing === item.email} onclick={() => remove(item.email)}>
								Hapus
							</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
		{#if !loading && data.items.length === 0}
			<p class="px-4 py-12 text-center text-sm text-muted-foreground">{tr('Belum ada alamat yang tercatat.')}</p>
		{/if}
	</div>

	<div class="mt-4 flex flex-wrap items-center justify-between gap-3">
		<p class="text-sm text-muted-foreground">
			{data.total ? `${offset + 1}–${Math.min(offset + PAGE_SIZE, data.total)} dari ${data.total} alamat` : '0 alamat'}
		</p>
		<Pagination total={data.total} limit={PAGE_SIZE} {offset} onChange={(next) => void load(next, search)} />
	</div>
</section>
