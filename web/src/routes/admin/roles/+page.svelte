<script lang="ts">
	// Role & akses — port app/admin/roles/page.tsx: daftar admin platform,
	// matriks hak akses menu (toggle per permission), ubah role, tambah admin
	// dari email user terdaftar, dan hapus akses admin. Khusus super admin.
	import { onMount } from 'svelte';
	import { Crown, Plus, ShieldCheck, Trash } from '@lucide/svelte';
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
	};

	const PERMISSION_KEYS = [
		'media', 'publications', 'settlements', 'payments', 'topup', 'errors',
		'users', 'partners', 'brands', 'content', 'pr', 'integrations'
	] as const;

	const PERMISSION_LABELS: Record<string, string> = {
		media: 'Approve & kelola media (domain mitra)',
		publications: 'Publikasi',
		settlements: 'Penarikan penghasilan mitra',
		payments: 'Pembayaran & gateway',
		topup: 'Error saldo top-up',
		errors: 'Error logs & operasional',
		users: 'Kelola user & persetujuan',
		partners: 'Mitra',
		brands: 'Brand & konten',
		content: 'Konten & kualitas',
		pr: 'PR & kalkulator',
		integrations: 'Integrasi & koneksi'
	};

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

	async function updatePerm(user: AdminRow, key: string, value: boolean) {
		rows = rows.map((item) =>
			item.user_id === user.user_id
				? { ...item, permissions: { ...item.permissions, [key]: value } }
				: item
		);
		const res = await apiRequest(`/v1/platform/admins/${user.user_id}`, {
			method: 'PATCH',
			body: JSON.stringify({ permissions: { [key]: value } })
		});
		if (res.ok) {
			const updated: AdminRow = await res.json();
			rows = rows.map((item) => (item.user_id === updated.user_id ? updated : item));
		} else {
			error = await platformError(res);
			await load();
		}
	}

	async function updateRole(user: AdminRow, role: AdminRole) {
		rows = rows.map((item) => (item.user_id === user.user_id ? { ...item, role } : item));
		const res = await apiRequest(`/v1/platform/admins/${user.user_id}`, {
			method: 'PATCH',
			body: JSON.stringify({ role })
		});
		if (res.ok) {
			const updated: AdminRow = await res.json();
			rows = rows.map((item) => (item.user_id === updated.user_id ? updated : item));
		} else {
			error = await platformError(res);
			await load();
		}
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
	const emptyList = $derived(Boolean(!loading && rows.length === 0 && !error));
</script>

<section>
	<PageHeader title="Role Admin" description="Kelola super admin dan admin biasa beserta hak akses menu-nya.">
		<span class="inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">
			<Crown size={14} /> {rows.length} admin
		</span>
	</PageHeader>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<h2 class="text-base font-semibold">Khusus Super Admin</h2>
			<p class="mt-1 text-sm text-muted-foreground">Halaman ini hanya dapat diakses oleh Super Admin.</p>
			<a class="app-btn app-btn-soft mt-4" href="/admin">Ke Overview</a>
		</div>
	{:else if emptyList}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<h2 class="text-base font-semibold">Role tidak ditemukan</h2>
			<p class="mt-1 text-sm text-muted-foreground">Belum ada admin yang terdaftar.</p>
		</div>
	{:else}
		<Feedback {loading} {error} loadingText="Memuat daftar role admin…" />

		<div class="mt-4 flex justify-end">
			<button class="app-btn app-btn-primary" type="button" onclick={openCreate}><Plus size={16} /> Tambah admin</button>
		</div>

		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 980px;">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">Admin</th>
						<th class="px-4 py-2.5 font-medium">Role</th>
						<th class="px-4 py-2.5 font-medium">Hak Akses Menu</th>
						<th class="px-4 py-2.5 font-medium">Dibuat</th>
						<th class="px-4 py-2.5"></th>
					</tr>
				</thead>
				<tbody>
					{#each rows as admin (admin.user_id)}
						<tr class="border-t align-top">
							<td class="px-4 py-2.5">
								<div class="flex flex-col gap-1">
									<p class="flex items-center gap-2 font-medium">
										{admin.display_name || admin.email}
										{#if admin.user_id === currentUserId}<Badge label="Anda" tone="neutral" />{/if}
									</p>
									<p class="text-xs text-muted-foreground">{admin.email}</p>
									{#if !admin.is_active}<Badge label="Akun nonaktif" tone="danger" />{/if}
								</div>
							</td>
							<td class="px-4 py-2.5">
								<div class="flex items-center gap-2">
									<span
										class="rounded-full px-2 py-0.5 text-[11px] font-medium"
										style="background-color: {admin.role === 'super_admin' ? 'var(--accent)' : 'var(--secondary)'}; color: {admin.role === 'super_admin' ? 'var(--accent-foreground)' : 'var(--muted-foreground)'};"
									>{admin.role === 'super_admin' ? 'Super Admin' : 'Admin'}</span>
									{#if admin.user_id !== currentUserId}
										<button
											class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs"
											type="button"
											onclick={() => void updateRole(admin, admin.role === 'super_admin' ? 'admin' : 'super_admin')}
										>Ubah</button>
									{/if}
								</div>
							</td>
							<td class="px-4 py-2.5">
								{#if admin.role === 'super_admin'}
									<span class="flex items-center gap-2 text-sm text-muted-foreground"><ShieldCheck size={15} /> Akses penuh</span>
								{:else}
									<div class="flex max-w-[560px] flex-wrap gap-3">
										{#each PERMISSION_KEYS as key (key)}
											<label class="flex items-center gap-2 text-xs text-muted-foreground">
												<input
													type="checkbox"
													checked={Boolean(admin.permissions[key])}
													onchange={(e) => void updatePerm(admin, key, (e.currentTarget as HTMLInputElement).checked)}
												/>
												{PERMISSION_LABELS[key]}
											</label>
										{/each}
									</div>
								{/if}
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
		</div>
	{/if}
</section>

{#if dialogOpen}
	<Modal title="Tambah admin" label="Tambah admin" maxWidth="lg" onClose={() => (dialogOpen = false)}>
		<p class="text-sm text-muted-foreground">
			Cari user berdasarkan email yang sudah terdaftar, lalu beri role dan hak akses menu.
		</p>
		<form class="mt-4 flex flex-col gap-3" onsubmit={submitCreate}>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Email user</span>
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
			<div class="mt-1 flex justify-end gap-2 border-t pt-4">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (dialogOpen = false)}>Batal</button>
				<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? 'Menyimpan…' : 'Tambah admin'}</button>
			</div>
		</form>
	</Modal>
{/if}

{#if deleting}
	<Modal title="Hapus role admin?" label="Hapus role admin" maxWidth="md" onClose={() => (deleting = null)}>
		<p class="text-sm text-muted-foreground">
			{deleting.display_name || deleting.email} akan kehilangan akses dashboard admin. Akun user itu sendiri tetap ada.
		</p>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => (deleting = null)}>Batal</button>
			<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void confirmDelete()}>
				{busy ? 'Menghapus…' : 'Hapus'}
			</button>
		{/snippet}
	</Modal>
{/if}
