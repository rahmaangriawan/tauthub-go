<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Daftar & tambah admin platform — memakai endpoint yang sama dengan
	// app lama `/admin/roles` (GET/POST /v1/platform/admins). App lama tidak
	// punya route `/admin/admins`; menu baru di shell memisahkan daftar admin
	// dari matriks role, jadi halaman ini fokus pada direktori + tambah admin.
	import { onMount } from 'svelte';
	import { MessageCircle, Plus, ShieldCheck, Trash, TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import { apiRequest, ensureValidSession, getStoredSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDate } from '$lib/admin-format';
	import { platformError } from '../_platform';

	type AdminRole = 'super_admin' | 'admin';
	type AdminRow = {
		user_id: string; email: string; display_name: string; is_active: boolean;
		role: AdminRole; permissions: Record<string, boolean>; created_at: string;
		wa_number?: string; wa_consent?: boolean;
		telegram_user_id?: number | null;
	};

	const PERMISSION_KEYS = [
		'media', 'publications', 'settlements', 'payments', 'topup', 'errors',
		'users', 'partners', 'brands', 'content', 'pr', 'integrations'
	] as const;

	const PERMISSION_LABELS: Record<string, string> = $derived({
		media: 'Approve & kelola media (domain mitra)',
		publications: tr('Publikasi'),
		settlements: 'Penarikan penghasilan mitra',
		payments: 'Pembayaran & gateway',
		topup: 'Error saldo top-up',
		errors: 'Error logs & operasional',
		users: tr('Kelola user & persetujuan'),
		partners: tr('Mitra'),
		brands: 'Brand & konten',
		content: 'Konten & kualitas',
		pr: 'PR & kalkulator',
		integrations: 'Integrasi & koneksi'
	});

	function emptyPermissions(): Record<string, boolean> {
		const out: Record<string, boolean> = {};
		for (const key of PERMISSION_KEYS) out[key] = false;
		return out;
	}

	let rows = $state<AdminRow[]>([]);
	let error = $state('');
	let loading = $state(true);
	let busy = $state(false);
	let dialogOpen = $state(false);
	let draftEmail = $state('');
	let draftRole = $state<AdminRole>('admin');
	let draftPerms = $state<Record<string, boolean>>(emptyPermissions());
	let deleting = $state<AdminRow | null>(null);
	let waTarget = $state<AdminRow | null>(null);
	let waDraft = $state('');
	let waError = $state('');

	function openWa(admin: AdminRow) {
		waTarget = admin;
		waDraft = admin.wa_number ?? '';
		waError = '';
	}

	async function saveWa(e: SubmitEvent) {
		e.preventDefault();
		if (!waTarget) return;
		busy = true;
		waError = '';
		const res = await apiRequest(`/v1/platform/admins/${waTarget.user_id}/wa-number`, {
			method: 'PUT',
			body: JSON.stringify({ wa_number: waDraft })
		});
		if (!res.ok) {
			waError = await platformError(res);
			busy = false;
			return;
		}
		const updated: AdminRow = await res.json();
		rows = rows.map((item) => (item.user_id === updated.user_id ? updated : item));
		waTarget = null;
		busy = false;
	}
	let currentUserId = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			currentUserId = getStoredSession()?.user.id ?? '';
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		loading = true;
		error = '';
		const res = await apiRequest('/v1/platform/admins');
		if (res.ok) rows = await res.json();
		else error = await platformError(res);
		loading = false;
	}

	function openCreate() {
		draftEmail = '';
		draftRole = 'admin';
		draftPerms = emptyPermissions();
		dialogOpen = true;
	}

	async function submitCreate(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		const res = await apiRequest('/v1/platform/admins', {
			method: 'POST',
			body: JSON.stringify({ email: draftEmail, role: draftRole, permissions: draftPerms })
		});
		if (!res.ok) {
			error = await platformError(res);
			busy = false;
			return;
		}
		const created: AdminRow = await res.json();
		rows = [...rows, created];
		dialogOpen = false;
		busy = false;
	}

	async function confirmDelete() {
		if (!deleting) return;
		busy = true;
		error = '';
		const res = await apiRequest(`/v1/platform/admins/${deleting.user_id}`, { method: 'DELETE' });
		if (!res.ok) {
			error = await platformError(res);
			busy = false;
			return;
		}
		rows = rows.filter((item) => item.user_id !== deleting!.user_id);
		deleting = null;
		busy = false;
	}

	const denied = $derived(Boolean(!loading && rows.length === 0 && error.includes('Akses platform')));

	// Edit Telegram ID untuk bot Persetujuan MCP. Dipakai saat admin menekan
	// tombol Setujui di Telegram — bot memetakan from.id ke baris admin ini.
	async function editTelegramId(admin: AdminRow) {
		const current = admin.telegram_user_id ? String(admin.telegram_user_id) : '';
		const raw = window.prompt(
			`Telegram User ID untuk ${admin.email}.\n` +
				`Cari lewat bot @userinfobot di Telegram.\n` +
				`Kosongkan lalu OK untuk menghapus kaitan.`,
			current
		);
		if (raw === null) return;
		const trimmed = raw.trim();
		if (trimmed && !/^\d+$/.test(trimmed)) {
			error = 'Telegram ID harus angka bulat.';
			return;
		}
		const payload: Record<string, unknown> = { telegram_user_id: trimmed === '' ? null : trimmed };
		busy = true;
		const res = await apiRequest(`/v1/platform/admins/${admin.user_id}`, {
			method: 'PATCH',
			body: JSON.stringify(payload)
		});
		busy = false;
		if (!res.ok) {
			error = await platformError(res);
			return;
		}
		await load();
	}
</script>

<section>
	<PageHeader title="Admins" description={tr("Daftar admin platform dan tambah admin baru dari email user yang sudah terdaftar.")}>
		<button class="app-btn app-btn-primary" type="button" onclick={openCreate}><Plus size={16} />{tr("Tambah admin")}</button>
	</PageHeader>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<h2 class="text-base font-semibold">Khusus Super Admin</h2>
			<p class="mt-1 text-sm text-muted-foreground">{tr("Halaman ini hanya dapat diakses oleh Super Admin.")}</p>
			<a class="app-btn app-btn-soft mt-4" href="/admin">Ke Overview</a>
		</div>
	{:else}
		<Feedback {loading} {error} loadingText="Memuat daftar admin…" />
		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 960px;">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">Admin</th>
						<th class="px-4 py-2.5 font-medium">Role</th>
						<th class="px-4 py-2.5 font-medium">Hak Akses</th>
						<th class="px-4 py-2.5 font-medium">WhatsApp</th>
						<th class="px-4 py-2.5 font-medium">Telegram</th>
						<th class="px-4 py-2.5 font-medium">{tr("Dibuat")}</th>
						<th class="px-4 py-2.5"></th>
					</tr>
				</thead>
				<tbody>
					{#each rows as admin (admin.user_id)}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<p class="flex items-center gap-2 font-medium">
									{admin.display_name || admin.email}
									{#if admin.user_id === currentUserId}<Badge label="Anda" tone="neutral" />{/if}
								</p>
								<p class="text-xs text-muted-foreground">{admin.email}</p>
								{#if !admin.is_active}<Badge label={tr("Akun nonaktif")} tone="danger" />{/if}
							</td>
							<td class="px-4 py-2.5">
								<span
									class="rounded-full px-2 py-0.5 text-[11px] font-medium"
									style="background-color: {admin.role === 'super_admin' ? 'var(--accent)' : 'var(--secondary)'}; color: {admin.role === 'super_admin' ? 'var(--accent-foreground)' : 'var(--muted-foreground)'};"
								>{admin.role === 'super_admin' ? 'Super Admin' : 'Admin'}</span>
							</td>
							<td class="px-4 py-2.5">
								{#if admin.role === 'super_admin'}
									<span class="flex items-center gap-2 text-sm text-muted-foreground"><ShieldCheck size={15} /> Akses penuh</span>
								{:else}
									<p class="max-w-[420px] text-xs text-muted-foreground">
										{PERMISSION_KEYS.filter((key) => admin.permissions[key]).map((key) => PERMISSION_LABELS[key]).join(', ') || tr('Belum ada hak akses.')}
									</p>
									<a class="text-xs" style="color: var(--primary);" href="/admin/roles">Atur di Role & Akses →</a>
								{/if}
							</td>
							<td class="px-4 py-2.5">
								{#if admin.wa_number}
									<p class="text-xs tabular-nums">+{admin.wa_number}</p>
									<button class="text-xs" style="color: var(--primary);" type="button" onclick={() => openWa(admin)}>{tr('Ubah nomor')}</button>
								{:else}
									<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={() => openWa(admin)}><MessageCircle size={14} />{tr('Tambahkan nomor WA')}</button>
								{/if}
							</td>
							<td class="px-4 py-2.5 text-xs">
								{#if admin.telegram_user_id}
									<span class="font-mono">{admin.telegram_user_id}</span>
								{:else}
									<span class="text-muted-foreground">—</span>
								{/if}
								<button
									class="text-xs underline"
									style="color: var(--primary); display: block; margin-top: 2px;"
									type="button"
									onclick={() => editTelegramId(admin)}
									disabled={busy}
								>Ubah</button>
							</td>
							<td class="px-4 py-2.5 text-xs text-muted-foreground">{fmtDate(admin.created_at)}</td>
							<td class="px-4 py-2.5">
								{#if admin.user_id !== currentUserId}
									<button
										class="rounded-lg p-1.5 hover:bg-secondary"
										style="color: var(--destructive);"
										type="button"
										aria-label={`Hapus admin ${admin.email}`}
										onclick={() => (deleting = admin)}
									><Trash size={14} /></button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && rows.length === 0 && !error}
				<div class="px-4 py-12 text-center text-sm text-muted-foreground">{tr("Belum ada admin yang terdaftar.")}</div>
			{/if}
		</div>
	{/if}
</section>

{#if dialogOpen}
	<Modal title={tr("Tambah admin")} label={tr("Tambah admin")} maxWidth="lg" onClose={() => (dialogOpen = false)}>
		<p class="text-sm text-muted-foreground">{tr("Cari user berdasarkan email yang sudah terdaftar, lalu beri role dan hak akses menu.")}</p>
		<form class="mt-4 flex flex-col gap-3" onsubmit={submitCreate}>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Email user")}</span>
				<input class="app-input w-full" type="email" required placeholder="pengelola@gmail.com" bind:value={draftEmail} />
			</label>
			<div>
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Role</span>
				<div class="flex gap-4 text-sm">
					<label class="flex items-center gap-2">
						<input type="radio" name="role" value="admin" checked={draftRole === 'admin'} onchange={() => (draftRole = 'admin')} /> Admin biasa
					</label>
					<label class="flex items-center gap-2">
						<input type="radio" name="role" value="super_admin" checked={draftRole === 'super_admin'} onchange={() => (draftRole = 'super_admin')} /> Super Admin
					</label>
				</div>
			</div>
			{#if draftRole === 'admin'}
				<div>
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">Hak akses menu</span>
					<div class="flex flex-col gap-2">
						{#each PERMISSION_KEYS as key (key)}
							<label class="flex items-center gap-2 text-sm">
								<input
									type="checkbox"
									checked={Boolean(draftPerms[key])}
									onchange={(e) => (draftPerms = { ...draftPerms, [key]: (e.currentTarget as HTMLInputElement).checked })}
								/>
								{PERMISSION_LABELS[key]}
							</label>
						{/each}
					</div>
				</div>
			{/if}
			{#if error}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{error}</div>
			{/if}
			<div class="mt-1 flex justify-end gap-2 border-t pt-4">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (dialogOpen = false)}>{tr("Batal")}</button>
				<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? 'Menyimpan…' : tr('Tambah admin')}</button>
			</div>
		</form>
	</Modal>
{/if}

{#if waTarget}
	<Modal title={tr("Nomor WhatsApp admin")} label={tr("Nomor WhatsApp admin")} maxWidth="md" onClose={() => (waTarget = null)}>
		<p class="text-sm text-muted-foreground">
			{tr('Nomor ini dipakai khusus untuk mengirim informasi login ke WhatsApp')} {waTarget.display_name || waTarget.email}. {tr('Email tidak dikirim.')}
		</p>
		<form class="mt-4 flex flex-col gap-3" onsubmit={saveWa}>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr('Nomor WhatsApp')}</span>
				<input class="app-input w-full" type="tel" inputmode="tel" placeholder="0812 3456 7890" bind:value={waDraft} />
				<span class="mt-1 block text-xs text-muted-foreground">{tr('Kosongkan lalu simpan untuk menghapus nomor.')}</span>
			</label>
			{#if waError}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{waError}</div>
			{/if}
			<div class="mt-1 flex justify-end gap-2 border-t pt-4">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (waTarget = null)}>{tr('Batal')}</button>
				<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? 'Menyimpan…' : tr('Simpan')}</button>
			</div>
		</form>
	</Modal>
{/if}

{#if deleting}
	<Modal title={tr("Hapus role admin?")} label={tr("Hapus role admin")} maxWidth="md" onClose={() => (deleting = null)}>
		<p class="text-sm text-muted-foreground">
			{deleting.display_name || deleting.email} {tr("akan kehilangan akses dashboard admin. Akun user itu sendiri tetap ada.")} </p>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => (deleting = null)}>{tr("Batal")}</button>
			<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void confirmDelete()}>
				{busy ? 'Menghapus…' : tr('Hapus')}
			</button>
		{/snippet}
	</Modal>
{/if}
