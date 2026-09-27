<script lang="ts">
	// Konfigurasi mitra — port section "Konfigurasi": profil mitra (nama,
	// rekening pencairan, persetujuan syarat integrasi), dokumen verifikasi,
	// dan sindikasi RSS (pengaktifan hanya oleh admin).
	import { onMount } from 'svelte';
	import { FileText, Save, Trash2, Upload } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	let loading = $state(true);
	let partner = $state<any>(null);
	let documents = $state<any[]>([]);
	let notice = $state('');
	let error = $state('');
	let busy = $state(false);
	let form = $state({
		name: '', payout_type: 'bank', bank_name: '', bank_account_number: '',
		bank_account_name: '', rss_enabled: false
	});

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([load(), loadDocuments()]);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		const res = await apiRequest('/v1/partner/me');
		if (res.ok) {
			const data = await res.json();
			partner = data.partner;
			if (partner) {
				form = {
					name: partner.name ?? '',
					payout_type: partner.payout_type ?? 'bank',
					bank_name: partner.bank_name ?? '',
					bank_account_number: partner.bank_account_number ?? '',
					bank_account_name: partner.bank_account_name ?? '',
					rss_enabled: Boolean(partner.rss_enabled)
				};
			}
		}
	}

	async function loadDocuments() {
		const res = await apiRequest('/v1/partner/documents');
		if (res.ok) documents = (await res.json()).documents ?? [];
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const res = await apiRequest('/v1/partner/me', {
				method: 'PATCH',
				body: JSON.stringify({
					name: form.name.trim(),
					payout_type: form.payout_type,
					bank_name: form.bank_name.trim(),
					bank_account_number: form.bank_account_number.trim(),
					bank_account_name: form.bank_account_name.trim(),
					accept_integration_terms: !partner?.integration_terms_accepted_at
				})
			});
			if (!res.ok) {
				error = await readApiError(res, "Permintaan gagal.");
				return;
			}
			notice = 'Profil mitra tersimpan.';
			await load();
		} finally {
			busy = false;
		}
	}

	async function disableRSS() {
		if (!confirm('Matikan sindikasi konten (RSS)? Pengaktifan hanya bisa dilakukan admin.')) return;
		const res = await apiRequest('/v1/partner/me', {
			method: 'PATCH',
			body: JSON.stringify({ rss_enabled: false })
		});
		const data = await res.json().catch(() => null);
		if (!res.ok) {
			error = data?.detail ?? 'Gagal mematikan RSS.';
			return;
		}
		notice = 'Sindikasi RSS dimatikan.';
		await load();
	}

	async function uploadDocument(ev: Event) {
		const input = ev.target as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		error = '';
		const body = new FormData();
		body.append('file', file);
		const res = await apiRequest('/v1/partner/documents', { method: 'POST', body });
		if (!res.ok) {
			error = await readApiError(res, "Permintaan gagal.");
			input.value = '';
			return;
		}
		notice = 'Dokumen diunggah, menunggu review admin.';
		input.value = '';
		await loadDocuments();
	}

	async function deleteDocument(id: string) {
		if (!confirm('Hapus dokumen ini?')) return;
		const res = await apiRequest(`/v1/partner/documents/${id}`, { method: 'DELETE' });
		if (!res.ok) {
			error = await readApiError(res, "Permintaan gagal.");
			return;
		}
		notice = 'Dokumen dihapus.';
		await loadDocuments();
	}

	const dt = (v: string | null) => (v ? new Date(v).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '—');
	const docStatus: Record<string, string> = { pending: 'Menunggu review', verified: 'Terverifikasi', revision: 'Perlu revisi' };
</script>

<section class="mx-auto max-w-3xl">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Konfigurasi</h1>
		<p class="mt-1 text-sm text-muted-foreground">Profil mitra, rekening pencairan, dan dokumen verifikasi.</p>
	</header>

	{#if notice}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}
	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat konfigurasi…</p>
	{:else}
		{#if partner && !partner.integration_terms_accepted_at}
			<div class="app-card mt-4 p-4" style="border-color: var(--warning, #a15c00);">
				<h2 class="text-sm font-semibold">Syarat Integrasi Belum Disetujui</h2>
				<p class="mt-1 text-xs text-muted-foreground">
					Dengan menyimpan profil ini, Anda menyatakan menyetujui syarat integrasi publikasi Tauthub.
				</p>
			</div>
		{/if}

		<form class="app-card mt-4 p-4" onsubmit={save}>
			<h2 class="text-sm font-semibold">Profil Mitra</h2>
			<div class="mt-3 grid gap-3 sm:grid-cols-2">
				<label class="block sm:col-span-2">
					<span class="text-xs font-medium text-muted-foreground">Nama Mitra *</span>
					<input class="app-input mt-1 w-full" required bind:value={form.name} />
				</label>
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">Metode Pencairan</span>
					<Select
						bind:value={form.payout_type}
						ariaLabel="Metode Pencairan"
						placeholder="Transfer Bank"
						minWidth={200}
						options={[
							{ value: 'bank', label: 'Transfer Bank' },
							{ value: 'ewallet', label: 'E-Wallet' },
							{ value: 'manual', label: 'Manual' }
						]}
					/>
				</label>
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">Nama Bank / E-Wallet</span>
					<input class="app-input mt-1 w-full" bind:value={form.bank_name} />
				</label>
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">Nomor Rekening / HP</span>
					<input class="app-input mt-1 w-full" inputmode="numeric" bind:value={form.bank_account_number} />
				</label>
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">Nama Pemilik Rekening</span>
					<input class="app-input mt-1 w-full" bind:value={form.bank_account_name} />
				</label>
			</div>
			<p class="mt-2 text-xs text-muted-foreground">
				Nomor rekening wajib diisi sebelum menarik saldo. Nomor hanya boleh berisi angka.
			</p>
			<div class="mt-3 flex justify-end">
				<button class="app-btn app-btn-primary" type="submit" disabled={busy}>
					<Save size={15} /> {busy ? 'Menyimpan…' : 'Simpan Profil'}
				</button>
			</div>
		</form>

		<div class="app-card mt-4 p-4">
			<h2 class="text-sm font-semibold">Sindikasi Konten (RSS)</h2>
			<p class="mt-1 text-xs text-muted-foreground">
				Status saat ini: <strong>{partner?.rss_enabled ? 'Aktif' : 'Nonaktif'}</strong>.
				Pengaktifan hanya dapat dilakukan admin Tauthub; mitra dapat mematikannya.
			</p>
			{#if partner?.rss_enabled}
				<button class="app-btn app-btn-soft mt-2" onclick={disableRSS}>Matikan RSS</button>
			{/if}
		</div>

		<div class="app-card mt-4 p-4">
			<div class="flex items-center justify-between gap-3">
				<div>
					<h2 class="text-sm font-semibold">Dokumen Verifikasi</h2>
					<p class="mt-0.5 text-xs text-muted-foreground">Sertifikat Dewan Pers, legalitas media (PDF/JPG/PNG, maks. 10 MB).</p>
				</div>
				<label class="app-btn app-btn-soft cursor-pointer">
					<Upload size={15} /> Unggah
					<input class="hidden" type="file" accept="application/pdf,image/*" onchange={uploadDocument} />
				</label>
			</div>

			{#if documents.length === 0}
				<p class="mt-3 text-sm text-muted-foreground">Belum ada dokumen.</p>
			{:else}
				<ul class="mt-3 divide-y">
					{#each documents as doc (doc.id)}
						<li class="flex items-center justify-between gap-3 py-2.5">
							<div class="flex min-w-0 items-center gap-2">
								<FileText size={16} class="shrink-0 text-muted-foreground" />
								<div class="min-w-0">
									<a class="block truncate text-sm font-medium" style="color: var(--primary);" href={doc.url} target="_blank" rel="noreferrer">{doc.name}</a>
									<p class="text-[11px] text-muted-foreground">
										{Math.round((doc.size_bytes ?? 0) / 1024)} KB · {docStatus[doc.status] ?? doc.status} · {dt(doc.uploaded_at)}
									</p>
									{#if doc.note}<p class="text-[11px]" style="color: var(--warning, #a15c00);">{doc.note}</p>{/if}
								</div>
							</div>
							<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label={`Hapus ${doc.name}`} onclick={() => deleteDocument(doc.id)}><Trash2 size={15} /></button>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}
</section>
