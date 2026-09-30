<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Tab WordPress (Koneksi CMS) — port ConnectionsSection app lama
	// (app/mitra/[partnerId]/[[...section]]/sections.tsx). Provider terkunci ke
	// WordPress REST API; daftar media diambil dari media mitra ber-website_url.
	import { onMount } from 'svelte';
	import { Info, Plug, Plus, Trash2 } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, authErrorMessage, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const APP_PASSWORD_STEPS = [
		'Login ke wp-admin situs Anda, contoh: https://situs-media.example/wp-admin',
		'Buka menu Pengguna → Profil',
		'Scroll ke bawah ke bagian Application Passwords',
		'Isi nama aplikasi (contoh: "Tauthub"), klik Add New Application Password',
		'Salin password yang muncul (format xxxx xxxx xxxx xxxx xxxx xxxx), hanya ditampilkan sekali',
		'Tempel di kolom Application Password di atas'
	];

	// GET /v1/partner/cms-connections mengembalikan SELURUH koneksi dalam satu
	// respons (tanpa limit/offset server), jadi paginasi dilakukan di klien
	// (parity usePagination app lama, 20 baris per halaman).
	const PAGE_SIZE = 20;

	let rows = $state<any[]>([]);
	let pageNumber = $state(1);
	let mediaOptions = $state<any[]>([]);
	let loading = $state(true);
	let error = $state('');
	let busy = $state(false);
	let testingId = $state<string | null>(null);
	let createOpen = $state(false);
	let form = $state({ website_id: '', name: '', base_url: '', username: '', application_password: '' });

	const totalPages = $derived(Math.max(1, Math.ceil(rows.length / PAGE_SIZE)));
	const visibleRows = $derived(rows.slice((pageNumber - 1) * PAGE_SIZE, pageNumber * PAGE_SIZE));

	function goToPage(next: number) {
		pageNumber = Math.min(Math.max(1, next), totalPages);
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

	async function load() {
		loading = true;
		error = '';
		try {
			const [connRes, mediaRes] = await Promise.all([
				apiRequest('/v1/partner/cms-connections'),
				apiRequest('/v1/partner/media?limit=500&offset=0')
			]);
			if (connRes.ok) {
				rows = (await connRes.json()).connections ?? [];
				goToPage(pageNumber);
			} else {
				error = await readApiError(connRes, tr('Gagal memuat koneksi.'));
			}
			if (mediaRes.ok) {
				const media = (await mediaRes.json()).media ?? [];
				mediaOptions = media.filter((item: any) => Boolean(item.website_url));
			}
		} finally {
			loading = false;
		}
	}

	function pickMedia(id: string) {
		const media = mediaOptions.find((item) => item.id === id);
		form.website_id = id;
		if (media?.website_url) form.base_url = media.website_url;
		if (!form.name.trim() && media) form.name = `${media.name} WordPress`;
	}

	async function create(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			// Provider terkunci: koneksi media mitra hanya mendukung WordPress.
			const body: Record<string, unknown> = {
				provider: 'wordpress',
				website_id: form.website_id || null,
				name: form.name.trim(),
				base_url: form.base_url.trim(),
				username: form.username.trim() || null,
				application_password: form.application_password
			};
			const res = await apiRequest('/v1/partner/cms-connections', { method: 'POST', body: JSON.stringify(body) });
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menyimpan koneksi.'));
				return;
			}
			createOpen = false;
			form = { website_id: '', name: '', base_url: '', username: '', application_password: '' };
			await load();
		} finally {
			busy = false;
		}
	}

	async function test(row: any) {
		error = '';
		testingId = row.id;
		try {
			const res = await apiRequest(`/v1/partner/cms-connections/${row.id}/test`, { method: 'POST' }, 30_000);
			if (!res.ok) error = await readApiError(res, 'Uji koneksi gagal.');
			await load();
		} finally {
			testingId = null;
		}
	}

	async function revoke(row: any) {
		error = '';
		const res = await apiRequest(`/v1/partner/cms-connections/${row.id}`, { method: 'DELETE' });
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal mencabut koneksi.'));
			return;
		}
		await load();
	}

	async function removeConnection(row: any) {
		if (!confirm(`Hapus permanen koneksi "${row.name}"? Tindakan ini tidak bisa dibatalkan.`)) return;
		error = '';
		const res = await apiRequest(`/v1/partner/cms-connections/${row.id}?mode=delete`, { method: 'DELETE' });
		if (!res.ok) {
			error = await readApiError(res, tr('Gagal menghapus koneksi.'));
			return;
		}
		await load();
	}
</script>

<div class="app-card mt-4 overflow-x-auto">
	<div class="flex flex-wrap items-center justify-between gap-2 border-b px-4 py-3">
		<div class="flex items-center gap-2">
			<Plug size={16} />
			<h2 class="text-sm font-semibold">Koneksi ({rows.length})</h2>
		</div>
		<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" onclick={() => (createOpen = true)}><Plus size={13} />{tr("Tambah koneksi")}</button>
	</div>

	<p class="px-4 pt-3 text-xs text-muted-foreground">{tr("Koneksi WordPress / Web Custom milik partner. Publikasi ke media Anda yang ber-integrasi WordPress dikirim lewat koneksi ini, bukan koneksi brand klien.")}</p>

	{#if error}<p class="px-4 pt-2 text-sm" style="color: var(--destructive);">{error}</p>{/if}

	{#if loading && rows.length === 0}
		<p class="px-4 py-8 text-center text-sm text-muted-foreground">{tr("Memuat koneksi…")}</p>
	{:else if rows.length === 0}
		<p class="px-4 py-8 text-center text-sm text-muted-foreground">{tr("Belum ada koneksi. Tambahkan koneksi WordPress untuk media ber-integrasi WordPress.")}</p>
	{:else}
		<table class="w-full text-sm">
			<thead>
				<tr class="text-left text-xs text-muted-foreground">
					<th class="px-4 py-2.5 font-medium">{tr('Koneksi')}</th>
					<th class="px-4 py-2.5 font-medium">Provider</th>
					<th class="px-4 py-2.5 font-medium">Endpoint</th>
					<th class="px-4 py-2.5 font-medium">Status</th>
					<th class="px-4 py-2.5 font-medium">{tr('Test terakhir')}</th>
					<th class="px-4 py-2.5 text-right font-medium">{tr('Aksi')}</th>
				</tr>
			</thead>
			<tbody>
				{#each visibleRows as row (row.id)}
					<tr class="border-t">
						<td class="px-4 py-2.5">
							<p class="font-medium">{row.name}</p>
							<p class="text-xs text-muted-foreground">{row.username ?? String(row.id).slice(0, 8)}</p>
						</td>
						<td class="px-4 py-2.5">
							<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--secondary); color: var(--muted-foreground);">{row.provider}</span>
						</td>
						<td class="max-w-[220px] break-words px-4 py-2.5 font-mono text-[11px] text-muted-foreground">{row.base_url}</td>
						<td class="px-4 py-2.5">
							<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: {row.status === 'active' ? 'var(--success-soft)' : 'var(--secondary)'}; color: {row.status === 'active' ? 'var(--success)' : 'var(--muted-foreground)'};">{row.status}</span>
						</td>
						<td class="px-4 py-2.5 text-xs" style="color: {row.last_test_status === 'ok' ? 'var(--success)' : row.last_test_status === 'error' ? 'var(--destructive)' : 'var(--muted-foreground)'};">
							{row.last_test_message ?? tr('Belum diuji')}
						</td>
						<td class="px-4 py-2.5">
							<div class="flex justify-end gap-2">
								{#if row.status === 'active'}
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" disabled={testingId !== null} onclick={() => test(row)}>
										{testingId === row.id ? 'Menguji…' : 'Test'}
									</button>
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" style="color: var(--destructive);" onclick={() => revoke(row)}>{tr('Cabut')}</button>
								{:else}
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" style="color: var(--destructive);" onclick={() => removeConnection(row)}>
										<Trash2 size={12} />{tr("Hapus")}</button>
								{/if}
							</div>
						</td>
					</tr>
				{/each}
				</tbody>
			</table>
	{/if}

	{#if rows.length > PAGE_SIZE}
		<div class="flex flex-wrap items-center justify-between gap-3 border-t px-4 py-3">
			<p class="text-xs text-muted-foreground">
				{tr("Menampilkan")} {(pageNumber - 1) * PAGE_SIZE + 1}–{Math.min(pageNumber * PAGE_SIZE, rows.length)} {tr("dari")} {rows.length}
			</p>
			<div class="flex items-center gap-2">
				<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" disabled={pageNumber <= 1} onclick={() => goToPage(pageNumber - 1)}>‹ {tr("Sebelumnya")}</button>
				<p class="text-xs text-muted-foreground">{tr("Halaman")} {pageNumber} {tr("dari")} {totalPages}</p>
				<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" disabled={pageNumber >= totalPages} onclick={() => goToPage(pageNumber + 1)}>{tr("Berikutnya")} ›</button>
			</div>
		</div>
	{/if}
</div>

{#if createOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto p-4" style="background-color: rgba(0,0,0,.45);" role="presentation">
		<dialog open class="app-card w-full max-w-lg p-5" aria-label={tr("Tambah koneksi")}>
			<h2 class="text-lg font-semibold">{tr("Tambah koneksi")}</h2>
			{#if error}<p class="mt-2 text-sm" style="color: var(--destructive);">{error}</p>{/if}
			<form class="mt-4 space-y-3" onsubmit={create}>
				<div>
					<span class="text-xs font-medium text-muted-foreground">Provider</span>
					<div class="mt-1 flex items-center gap-2 rounded-lg border px-3 py-2">
						<Plug size={15} />
						<span class="text-sm font-medium">WordPress REST API</span>
						<span class="ml-auto rounded-full px-2 py-0.5 text-[11px]" style="background: var(--secondary); color: var(--muted-foreground);">{tr('Terkunci')}</span>
					</div>
					<p class="mt-1 text-[11px] text-muted-foreground">{tr("Provider terkunci ke WordPress REST API, koneksi media mitra hanya mendukung WordPress.")}</p>
				</div>

				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">{tr("Pilih media")}</span>
					<Select
						ariaLabel={tr("Pilih media")}
						placeholder={mediaOptions.length > 0 ? tr('Pilih media yang akan dikoneksikan') : tr('Belum ada media dengan website URL')}
						minWidth={260}
						value={form.website_id}
						onchange={(v) => pickMedia(v)}
						options={[
							{
								value: '',
								label: mediaOptions.length > 0 ? tr('Pilih media yang akan dikoneksikan') : tr('Belum ada media dengan website URL'),
								disabled: true
							},
							...mediaOptions.map((media) => ({ value: media.id, label: `${media.name} (${media.website_url})` }))
						]}
					/>
					<p class="mt-1 text-[11px] text-muted-foreground">{tr("Daftar diambil dari Media Anda. URL WordPress terisi otomatis.")}</p>
				</label>

				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">{tr("Nama koneksi")}</span>
					<input class="app-input mt-1 w-full" required placeholder={tr("Nama koneksi")} bind:value={form.name} />
				</label>
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">{tr('URL WordPress')}</span>
					<input class="app-input mt-1 w-full" required readonly placeholder={tr("Pilih media terlebih dahulu")} bind:value={form.base_url} />
				</label>
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">{tr('Username / email WordPress')}</span>
					<input class="app-input mt-1 w-full" required bind:value={form.username} />
				</label>
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">{tr("Application Password")}</span>
					<input
						class="app-input mt-1 w-full"
						required
						type="password"
						placeholder="xxxx xxxx xxxx xxxx xxxx xxxx"
						value={form.application_password}
						oninput={(e) => (form.application_password = e.currentTarget.value.replace(/\s+/g, ''))}
					/>
					<div class="mt-2 rounded-lg border p-3">
						<p class="flex items-center gap-1 text-[11px] font-medium"><Info size={13} />{tr("Cara Ambil Application Password")}</p>
						<ol class="mt-2 space-y-1.5">
							{#each APP_PASSWORD_STEPS as step, i (i)}
								<li class="flex gap-2 text-[11px] leading-relaxed">
									<span class="mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full text-[9px] font-bold text-white" style="background: var(--primary);">{i + 1}</span>
									<span>{step}</span>
								</li>
							{/each}
						</ol>
					</div>
				</label>

				<div class="flex justify-end gap-2 border-t pt-3">
					<button class="app-btn app-btn-soft" type="button" onclick={() => (createOpen = false)}>{tr("Batal")}</button>
					<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{tr("Simpan koneksi")}</button>
				</div>
			</form>
		</dialog>
	</div>
{/if}
