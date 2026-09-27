<script lang="ts">
	// Kelola member brand — daftar anggota, tambah lewat email, ubah role, hapus.
	// Endpoint: /v1/brands/{brandId}/members (list/create/update/delete).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Check, Plus, Trash2, UserPlus, Users } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import Select from '$lib/components/select.svelte';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);

	type Member = {
		id: string; user_id: string; role: string; created_at: string; email: string; display_name: string;
	};

	const ROLES: Array<{ value: string; label: string }> = [
		{ value: 'admin', label: 'Admin' },
		{ value: 'strategist', label: 'Strategist' },
		{ value: 'writer', label: 'Writer' },
		{ value: 'reviewer', label: 'Reviewer' },
		{ value: 'viewer', label: 'Viewer' }
	];
	const roleLabel = (role: string) => ROLES.find((r) => r.value === role)?.label ?? role;

	let members = $state<Member[]>([]);
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');
	let showAdd = $state(false);
	let form = $state({ email: '', role: 'viewer' });
	const brandUrlId = $derived(brandUrl);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		error = '';
		const res = await apiRequest(`/v1/brands/${brandId}/members`);
		if (!res.ok) {
			error = await readApiError(res, 'Gagal memuat daftar member.');
			return;
		}
		members = await res.json();
	}

	async function addMember(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/members`, {
				method: 'POST',
				body: JSON.stringify({ email: form.email.trim(), role: form.role })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal menambah member.');
				return;
			}
			const created = (await res.json()) as Member;
			notice = `${created.display_name || created.email} ditambahkan sebagai ${roleLabel(created.role)}.`;
			form = { email: '', role: 'viewer' };
			showAdd = false;
			await load();
		} finally {
			busy = false;
		}
	}

	async function updateRole(member: Member, role: string) {
		if (role === member.role) return;
		busy = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/members/${member.id}`, {
				method: 'PUT',
				body: JSON.stringify({ role })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal mengubah role member.');
				return;
			}
			notice = `Role ${member.display_name || member.email} diubah menjadi ${roleLabel(role)}.`;
			await load();
		} finally {
			busy = false;
		}
	}

	async function removeMember(member: Member) {
		if (!confirm(`Hapus ${member.display_name || member.email} dari brand ini?`)) return;
		busy = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/members/${member.id}`, { method: 'DELETE' });
			if (!res.ok) {
				error = await readApiError(res, 'Gagal menghapus member.');
				return;
			}
			notice = `${member.display_name || member.email} dihapus dari brand.`;
			await load();
		} finally {
			busy = false;
		}
	}

	const dt = (v: string | null) =>
		v ? new Date(v).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' }) : '—';
</script>

<section class="mx-auto max-w-4xl">
	<header class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">Member Brand</h1>
			<p class="mt-1 text-sm text-muted-foreground">
				Kelola siapa saja yang punya akses ke brand ini beserta perannya.
			</p>
		</div>
		<button class="app-btn app-btn-primary" type="button" onclick={() => { showAdd = !showAdd; error = ''; }}>
			<UserPlus size={15} /> Tambah Member
		</button>
	</header>

	{#if notice}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}
	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	{#if showAdd}
		<form class="app-card mt-4 p-4" onsubmit={addMember}>
			<h2 class="text-sm font-semibold">Tambah member</h2>
			<p class="mt-0.5 text-xs text-muted-foreground">
				Masukkan email akun Tauthub yang sudah terdaftar. Role menentukan hak akses di workspace brand ini.
			</p>
			<div class="mt-3 grid gap-3 sm:grid-cols-2">
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">Email pengguna</span>
					<input class="app-input mt-1 w-full" type="email" required placeholder="nama@perusahaan.com" bind:value={form.email} />
				</label>
				<label class="block">
					<span class="text-xs font-medium text-muted-foreground">Role</span>
					<div class="mt-1">
						<Select
							bind:value={form.role}
							ariaLabel="Role member"
							minWidth={200}
							options={ROLES.map((r) => ({ value: r.value, label: r.label }))}
						/>
					</div>
				</label>
			</div>
			<div class="mt-3 flex justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (showAdd = false)}>Batal</button>
				<button class="app-btn app-btn-primary" type="submit" disabled={busy || !form.email.trim()}>
					{busy ? 'Menyimpan…' : 'Tambah'}
				</button>
			</div>
		</form>
	{/if}

	<div class="app-card mt-4 overflow-x-auto">
		{#if loading}
			<p class="px-4 py-8 text-center text-sm text-muted-foreground">Memuat member…</p>
		{:else if members.length === 0}
			<div class="flex flex-col items-center px-4 py-10 text-center">
				<Users size={24} style="color: var(--primary);" />
				<p class="mt-2 text-sm font-medium">Belum ada member.</p>
				<p class="mt-1 text-sm text-muted-foreground">Tambahkan rekan tim agar mereka bisa mengelola brand ini.</p>
			</div>
		{:else}
			<table class="w-full text-sm">
				<thead>
					<tr class="border-b text-left text-[11px] tracking-wide text-muted-foreground">
						<th class="px-4 py-2.5 font-semibold">NAMA</th>
						<th class="memb-email px-4 py-2.5 font-semibold">EMAIL</th>
						<th class="px-4 py-2.5 font-semibold">ROLE</th>
						<th class="memb-added px-4 py-2.5 font-semibold">DITAMBAHKAN</th>
						<th class="px-4 py-2.5 font-semibold text-right">AKSI</th>
					</tr>
				</thead>
				<tbody>
					{#each members as m (m.id)}
						<tr class="border-b last:border-0">
							<td class="px-4 py-2.5 font-medium">{m.display_name || '—'}</td>
							<td class="memb-email px-4 py-2.5 text-muted-foreground">{m.email}</td>
							<td class="px-4 py-2.5">
								<div
									class={busy || m.role === 'owner' ? 'pointer-events-none opacity-60' : ''}
									inert={busy || m.role === 'owner'}
								>
									<Select
										value={m.role}
										ariaLabel={`Role ${m.display_name || m.email}`}
										minWidth={150}
										onchange={(v) => void updateRole(m, v)}
										options={[
											...(m.role === 'owner' ? [{ value: 'owner', label: 'Owner' }] : []),
											...ROLES.map((r) => ({ value: r.value, label: r.label }))
										]}
									/>
								</div>
							</td>
							<td class="memb-added px-4 py-2.5 text-muted-foreground">{dt(m.created_at)}</td>
							<td class="px-4 py-2.5 text-right">
								{#if m.role === 'owner'}
									<span class="text-[11px] text-muted-foreground">Pemilik brand</span>
								{:else}
									<button
										class="rounded-lg p-1.5 transition-colors hover:bg-secondary"
										aria-label={`Hapus ${m.display_name || m.email}`}
										disabled={busy}
										onclick={() => void removeMember(m)}
									>
										<Trash2 size={15} style="color: var(--destructive);" />
									</button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</div>

	<p class="mt-3 flex items-center gap-1.5 text-xs text-muted-foreground">
		<Check size={13} style="color: var(--success);" />
		Hanya admin brand yang dapat mengubah role dan menghapus member.
	</p>
</section>

<style>
	/* Di layar sempit kolom email dan tanggal membungkus supaya tabel 5 kolom
	   ini tidak memaksa scroll horizontal di dalam kartu. */
	@media (max-width: 767px) {
		.memb-email,
		.memb-added { word-break: break-word; white-space: normal; }
	}
</style>
