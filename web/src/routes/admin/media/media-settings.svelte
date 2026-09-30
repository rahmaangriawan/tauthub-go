<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Tab Pengaturan /admin/media — konfigurasi media platform (template URL bulk import).
	// Port MediaSettingsSection di apps/web/src/app/admin/media/_sections.tsx.
	import { Newspaper } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import { errMessage, mountWithSession, platformRequest } from './_shared';

	let templateUrl = $state('');
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');

	onMount(() =>
		mountWithSession(async () => {
			try {
				const data = await platformRequest<{ url: string | null }>('/config/template-url');
				templateUrl = data.url ?? '';
			} catch {
				/* ignore */
			}
			loading = false;
		})
	);

	async function save() {
		busy = true;
		error = '';
		notice = '';
		try {
			const url = templateUrl.trim();
			const data = await platformRequest<{ ok: boolean; url: string | null }>('/config/template-url', {
				method: 'PUT',
				body: JSON.stringify({ url })
			});
			templateUrl = data.url ?? '';
			notice = data.url ? 'Template URL tersimpan.' : 'Template URL dihapus.';
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			busy = false;
		}
	}
</script>

{#if loading}
	<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat pengaturan…")}</p>
{:else}
	<div class="app-card mt-4 p-4">
		<div class="flex flex-col gap-3">
			<div class="flex items-center gap-2">
				<Newspaper size={18} />
				<p class="font-bold">{tr("Pengaturan Bulk Import Media")}</p>
			</div>
			<p class="text-sm text-muted-foreground">{tr("URL template Google Spreadsheet yang digunakan mitra untuk Bulk Import Media. Kosongkan jika belum tersedia.")}</p>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Template URL (Google Spreadsheet)</span>
				<input
					class="app-input mt-1 w-full"
					placeholder="https://docs.google.com/spreadsheets/d/..."
					bind:value={templateUrl}
				/>
			</label>
			<p class="text-xs text-muted-foreground">{tr("Mitra akan melihat tombol \"Buka Template Spreadsheet\" di dialog Bulk Import. Jika kosong, mitra akan melihat pesan bahwa template belum diatur.")}</p>
			{#if error}
				<div class="text-sm" style="color: var(--destructive);">{error}</div>
			{/if}
			{#if notice}
				<div class="text-sm" style="color: var(--success);">{notice}</div>
			{/if}
			<div class="flex justify-end gap-2">
				<button class="app-btn app-btn-primary" disabled={busy} onclick={save}
					>{busy ? 'Menyimpan…' : tr('Simpan')}</button
				>
			</div>
		</div>
	</div>
{/if}
