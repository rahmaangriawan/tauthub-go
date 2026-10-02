<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Port IntegrationsPanel (brand-workspace.tsx) — koneksi WordPress REST API
	// (cms-connections) + kirim artikel via ?document=&title= (deep link Preview).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	const documentId = $derived(page.url.searchParams.get('document'));
	const documentTitle = $derived(page.url.searchParams.get('title') ?? tr('Artikel terpilih'));

	type Connection = { id: string; name: string; base_url: string; status: string };

	let connections = $state<Connection[]>([]);
	let form = $state({ provider: 'wordpress' as 'wordpress' | 'emdash', name: '', base_url: '', username: '', application_password: '', access_token: '' });
	let busy = $state(false);
	let publishStatus = $state<'draft' | 'publish'>('draft');
	let message = $state('');
	let messageOk = $state(false);

	const activeConnection = $derived(connections.find((c) => c.status === 'active'));

	async function load() {
		const res = await apiRequest(`/v1/brands/${brandId}/cms-connections`);
		if (res.ok) connections = (await res.json()) as Connection[];
	}

	async function create(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		message = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/cms-connections`, {
				method: 'POST',
				body: JSON.stringify(
					form.provider === 'emdash'
						? { provider: form.provider, name: form.name, base_url: form.base_url, access_token: form.access_token }
						: { provider: form.provider, name: form.name, base_url: form.base_url, username: form.username, application_password: form.application_password }
				)
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menyimpan koneksi.')));
			form = { provider: form.provider, name: '', base_url: '', username: '', application_password: '', access_token: '' };
			message = tr('Koneksi tersimpan.');
			messageOk = true;
			await load();
		} catch (error) {
			message = error instanceof Error ? error.message : tr('Gagal menyimpan koneksi.');
			messageOk = false;
		} finally {
			busy = false;
		}
	}

	async function test(id: string) {
		const res = await apiRequest(`/v1/brands/${brandId}/cms-connections/${id}/test`, { method: 'POST' }, 30_000);
		const result = (await res.json().catch(() => null)) as { message?: string; detail?: string } | null;
		message = result?.message ?? result?.detail ?? (res.ok ? tr('Koneksi berhasil diuji.') : tr('Koneksi gagal.'));
		messageOk = res.ok;
		await load();
	}

	async function revoke(id: string) {
		if (!window.confirm(tr('Cabut koneksi WordPress ini?'))) return;
		const res = await apiRequest(`/v1/brands/${brandId}/cms-connections/${id}`, { method: 'DELETE' });
		message = res.ok ? tr('Koneksi WordPress berhasil dicabut.') : await readApiError(res, tr('Gagal mencabut koneksi.'));
		messageOk = res.ok;
		await load();
	}

	async function publishToWordPress() {
		if (!documentId || !activeConnection) return;
		busy = true;
		message = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/documents/${documentId}/publish`, {
				method: 'POST',
				body: JSON.stringify({ connection_id: String(activeConnection.id), publish_status: publishStatus })
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal mengirim artikel ke WordPress.')));
			const publication = (await res.json()) as { status: string; error_message?: string | null };
			if (publication.status === 'failed') throw new Error(publication.error_message || tr('WordPress menolak publikasi.'));
			if (publication.status !== 'completed')
				throw new Error(`${tr('Publikasi belum selesai (status')}: ${publication.status}).`);
			message = tr('Artikel berhasil dikirim sebagai') + ` ${publishStatus === 'draft' ? 'draft' : 'publish'}.`;
			messageOk = true;
		} catch (error) {
			message = error instanceof Error ? error.message : tr('Gagal mengirim artikel.');
			messageOk = false;
		} finally {
			busy = false;
		}
	}

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});
</script>

<PageTitle title={tr('Integrations')} />

<section>
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Integrations</h1>
		<p class="mt-1 text-sm text-muted-foreground">WordPress REST API — {tr('Gunakan Application Password WordPress, bukan password login biasa.')}</p>
	</header>

	{#if documentId}
		<div class="app-card mt-4 p-5">
			<h2 class="font-semibold">{tr('Integrasikan Artikel ke WordPress')}</h2>
			<p class="mt-1 text-sm text-muted-foreground">{tr('Artikel yang akan dikirim')}: <strong>{documentTitle}</strong></p>
			<p class="mt-1 text-sm text-muted-foreground">
				{#if activeConnection}
					{tr('Koneksi aktif')}: <strong>{activeConnection.name}</strong>
				{:else}
					{tr('Belum ada koneksi WordPress aktif.')}
				{/if}
			</p>
			<div class="mt-3 flex flex-wrap items-center gap-4">
				<fieldset class="flex gap-4 text-sm">
					<label class="flex items-center gap-1.5">
						<input type="radio" name="publish-status" checked={publishStatus === 'draft'} onchange={() => (publishStatus = 'draft')} />
						{tr('Simpan sebagai Draft')}
					</label>
					<label class="flex items-center gap-1.5">
						<input type="radio" name="publish-status" checked={publishStatus === 'publish'} onchange={() => (publishStatus = 'publish')} />
						{tr('Publish Artikel')}
					</label>
				</fieldset>
				<button class="app-btn app-btn-primary" type="button" disabled={busy || !activeConnection} onclick={() => void publishToWordPress()}>
					{busy ? tr('Mengirim ke WordPress') : tr('Kirim ke WordPress')}
				</button>
			</div>
		</div>
	{/if}

	{#if message}
		<p
			class="mt-4 rounded-md px-3 py-2 text-sm"
			style="background: {messageOk ? 'var(--success-soft)' : 'color-mix(in srgb, var(--destructive) 8%, transparent)'}; color: {messageOk ? 'var(--success)' : 'var(--destructive)'};"
		>
			{message}
		</p>
	{/if}

	<div class="mt-4 grid gap-4 lg:grid-cols-3">
		<div class="app-card p-5 lg:col-span-2">
			{#if !connections.some((c) => c.status === 'active')}
				<form class="mt-3 space-y-3" onsubmit={create}>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">{tr('Jenis Integrasi')}</span>
						<select class="app-input mt-1 w-full" bind:value={form.provider}>
							<option value="wordpress">WordPress (REST API)</option>
							<option value="emdash">EmDash CMS</option>
						</select>
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">{tr('Nama Koneksi')}</span>
						<input class="app-input mt-1 w-full" required placeholder={tr('Contoh: Situs Utama')} bind:value={form.name} />
					</label>
					<label class="block">
						<span class="text-xs font-medium text-muted-foreground">{tr('Base URL')}</span>
						<input class="app-input mt-1 w-full" required placeholder="https://situs-anda.com" bind:value={form.base_url} />
					</label>
					{#if form.provider === 'wordpress'}
						<label class="block">
							<span class="text-xs font-medium text-muted-foreground">{tr('Username WordPress')}</span>
							<input class="app-input mt-1 w-full" required bind:value={form.username} />
						</label>
						<label class="block">
							<span class="text-xs font-medium text-muted-foreground">{tr('Application Password')}</span>
							<input class="app-input mt-1 w-full" required type="password" bind:value={form.application_password} />
						</label>
					{:else}
						<label class="block">
							<span class="text-xs font-medium text-muted-foreground">Access Token EmDash</span>
							<input class="app-input mt-1 w-full" required type="password" placeholder="ec_pat_…" bind:value={form.access_token} />
							<span class="mt-1 block text-[11px] text-muted-foreground">{tr('Personal access token dari EmDash (menu Integrations → API).')}</span>
						</label>
					{/if}
					<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? tr('Menyimpan…') : tr('Simpan Koneksi')}</button>
				</form>
			{:else}
				<p class="text-sm text-muted-foreground">
					{tr('Satu brand hanya dapat memiliki satu koneksi WordPress aktif. Cabut koneksi saat ini untuk menggantinya.')}
				</p>
			{/if}
		</div>

		<div class="app-card p-5">
			<h2 class="font-semibold">{tr('Koneksi tersimpan')}</h2>
			<div class="mt-3 flex flex-col gap-2">
				{#if connections.length === 0}
					<p class="text-sm text-muted-foreground">{tr('Belum ada koneksi WordPress.')}</p>
				{:else}
					{#each connections as connection (connection.id)}
						<div class="flex items-center justify-between gap-3 rounded-lg border px-3 py-2" style="border-color: var(--border);">
							<div class="min-w-0">
								<span class="block truncate text-sm font-medium">{connection.name}</span>
								<span class="block truncate text-xs text-muted-foreground">
									{connection.base_url} ·
									<span style="color: {connection.status === 'active' ? 'var(--success)' : 'var(--muted-foreground)'}; font-weight: 600;">{connection.status}</span>
								</span>
							</div>
							<div class="flex flex-none gap-1.5">
								<button class="app-btn app-btn-soft" type="button" onclick={() => void test(connection.id)}>Test</button>
								<button class="app-btn app-btn-soft" type="button" onclick={() => void revoke(connection.id)}>{tr('Cabut')}</button>
							</div>
						</div>
					{/each}
				{/if}
			</div>
		</div>
	</div>
</section>
