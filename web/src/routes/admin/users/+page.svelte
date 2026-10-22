<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Kelola user (klien & mitra) — port app/admin/users/page.tsx:
	// setujui pendaftaran, suspend/reactivate, kuota token harian, jatah
	// generate Writer, tambah TC manual, set password (super admin),
	// ubah peran, dan tambah pengguna manual.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		ArrowLeftRight, Coins, Ellipsis, Key, Search, UserRound, Plus, TriangleAlert,
		CircleCheck
	} from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, ensureValidSession, getStoredSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDate, fmtNum } from '$lib/admin-format';
	import { platformError } from '../_platform';
	import { swrPeek, swrPut } from '$lib/swr';

	type GenQuota = { user_id: string; writer_generate_quota: number | null; effective: number };
	type User = {
		id: string; email: string; display_name: string; role?: string;
		is_active: boolean; is_approved?: boolean; brand_count: number; created_at: string;
		writer_generate_quota?: number | null;
		available_tc?: number | null;
	};
	type BrandOption = {
		id: string; name: string; public_id: string | null; role: string;
		available_tc: number; reserved_tc: number; captured_tc: number;
	};

	type StatusKind = 'pending' | 'active' | 'suspended';
	const statusMeta: Record<StatusKind, { label: string; tone: 'warning' | 'success' | 'danger' }> = $derived({
		pending: { label: tr('Menunggu'), tone: 'warning' },
		active: { label: tr('Aktif'), tone: 'success' },
		suspended: { label: 'Ditangguhkan', tone: 'danger' }
	});

	const PAGE_SIZE = 20;

	let rows = $state<User[]>([]);
	let loading = $state(true);
	let error = $state('');
	let busy = $state(false);
	let saving = $state<string | null>(null);
	let pageNum = $state(1);
	let currentUserId = $state('');
	let isSuperAdmin = $state(false);

	let searchText = $state('');
	let roleFilter = $state('client');

	// Dialog status (approve/suspend/reactivate)
	let statusTarget = $state<User | null>(null);
	// Dialog tambah TC manual
	let tcUser = $state<User | null>(null);
	let tcBrands = $state<BrandOption[]>([]);
	let tcBrandId = $state('');
	let tcAmount = $state('');
	// Batas server: POST /credit-adjustment menolak > 10.000.000 TC.
	const TC_MAX = 10_000_000;
	let tcReason = $state('');
	let tcError = $state('');
	let tcBusy = $state(false);
	let tcNotice = $state('');
	// Dialog tambah pengguna manual
	let createOpen = $state(false);
	let createEmail = $state('');
	let createName = $state('');
	let createRole = $state<'client' | 'partner'>('client');
	let createPassword = $state('');
	let createBusy = $state(false);
	let createError = $state('');
	let createNotice = $state('');
	// Dialog set password (super admin)
	let pwUser = $state<User | null>(null);
	let pwValue = $state('');
	let pwConfirm = $state('');
	let pwBusy = $state(false);
	let pwError = $state('');
	let pwNotice = $state('');
	// Dialog ubah peran
	let roleTarget = $state<User | null>(null);
	let roleNext = $state<'client' | 'partner'>('partner');
	let roleKeepBrands = $state(false);
	let roleBusy = $state(false);
	let roleError = $state('');
	let roleNotice = $state('');
	// Menu aksi per baris
	let openMenu = $state<string | null>(null);
	let menuPos = $state({ top: 0, right: 0 });

	const statusKind = (user: User): StatusKind =>
		user.is_approved === false ? 'pending' : user.is_active ? 'active' : 'suspended';
	const initials = (user: User) =>
		(user.display_name || user.email)
			.split(/\s+/)
			.slice(0, 2)
			.map((part) => part[0])
			.join('')
			.toUpperCase();

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			currentUserId = getStoredSession()?.user.id ?? '';
			// Deep-link ?role=client|partner (menu sidebar per role).
			const roleParam = page.url.searchParams.get('role');
			if (roleParam === 'client') roleFilter = roleParam;
			await load();
			loading = false;
			const me = await apiRequest('/v1/platform/admins/me');
			if (me.ok) {
				const data = await me.json();
				isSuperAdmin = data.role === 'super_admin';
			}
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load(search = '', role = roleFilter) {
		error = '';
		const params = new URLSearchParams();
		if (search) params.set('search', search);
		if (role) params.set('role', role);
		const query = params.toString();
		const listUrl = `/v1/platform/operations/users${query ? `?${query}` : ''}`;
		// Data lama (<=10 menit) tampil seketika, refresh di latar.
		const cached = swrPeek<User[]>(listUrl);
		if (cached) {
			rows = cached;
			loading = false;
		} else {
			loading = true;
		}
		const res = await apiRequest(listUrl);
		if (res.ok) {
			const data = (await res.json()) as User[];
			rows = data;
			swrPut(listUrl, data);
		} else {
			error = await platformError(res);
		}
		loading = false;
	}

	// Deep-link & navigasi sidebar (?role=client / ?role=partner). Navigasi
	// client-side antar menu tidak me-remount halaman, jadi perubahan query
	// harus didengarkan agar filter ikut berganti.
	let roleParamSeen = page.url.searchParams.get('role') ?? '';
	$effect(() => {
		const roleParam = page.url.searchParams.get('role');
		if ((roleParam === 'client') && roleParam !== roleParamSeen) {
			roleParamSeen = roleParam;
			if (roleParam !== roleFilter) {
				roleFilter = roleParam;
				pageNum = 1;
				void load(searchText.trim(), roleParam);
			}
		}
	});

	function submitSearch(e: SubmitEvent) {
		e.preventDefault();
		pageNum = 1;
		void load(searchText.trim());
	}


	async function updateStatus() {
		if (!statusTarget) return;
		busy = true;
		error = '';
		const kind = statusKind(statusTarget);
		const body =
			kind === 'pending'
				? { is_active: true, approve: true }
				: { is_active: kind === 'active' ? false : true };
		const res = await apiRequest(`/v1/platform/operations/users/${statusTarget?.id}/status`, {
			method: 'PUT',
			body: JSON.stringify(body)
		});
		if (!res.ok) {
			error = await platformError(res);
			busy = false;
			return;
		}
		const updated: User = await res.json();
		rows = rows.map((item) => (item.id === updated.id ? updated : item));
		statusTarget = null;
		busy = false;
	}

	async function updateGenQuota(userId: string, value: string) {
		const raw = value.trim();
		const parsed = raw === '' ? null : parseInt(raw, 10);
		if (raw !== '' && (isNaN(parsed as number) || (parsed as number) < 1 || (parsed as number) > 999)) return;
		saving = userId;
		error = '';
		const res = await apiRequest(`/v1/platform/operations/users/${userId}/generate-quota`, {
			method: 'PUT',
			body: JSON.stringify({ writer_generate_quota: parsed })
		});
		if (res.ok) {
			const updated: GenQuota = await res.json();
			rows = rows.map((item) =>
				item.id === updated.user_id ? { ...item, writer_generate_quota: updated.writer_generate_quota } : item
			);
		} else {
			error = await platformError(res);
		}
		saving = null;
	}

	async function loadGenQuota(userId: string) {
		const res = await apiRequest(`/v1/platform/operations/users/${userId}/generate-quota`);
		if (!res.ok) return;
		const quota: GenQuota = await res.json();
		rows = rows.map((item) =>
			item.id === quota.user_id ? { ...item, writer_generate_quota: quota.writer_generate_quota } : item
		);
	}

	function openCreate() {
		createError = '';
		createNotice = '';
		createEmail = '';
		createName = '';
		createRole = 'client';
		createPassword = '';
		createOpen = true;
	}

	async function submitCreateUser() {
		if (createEmail.trim().length < 5 || createName.trim().length < 2) {
			createError = 'Email valid dan nama (min 2 karakter) wajib diisi.';
			return;
		}
		const password = createPassword.trim();
		if (password && password.length < 10) {
			createError = 'Password minimal 10 karakter.';
			return;
		}
		createBusy = true;
		createError = '';
		createNotice = '';
		const res = await apiRequest('/v1/platform/operations/users', {
			method: 'POST',
			body: JSON.stringify({
				email: createEmail.trim().toLowerCase(),
				display_name: createName.trim(),
				role: createRole,
				...(password ? { password } : {})
			})
		});
		if (!res.ok) {
			createError = await platformError(res);
			createBusy = false;
			return;
		}
		const result = await res.json();
		createNotice = result.password_set
			? tr('User dibuat dengan password yang Anda tetapkan — akun sudah bisa login sekarang.')
			: `User dibuat.${result.activation_sent ? ' Email aktivasi terkirim — user mengatur password lewat link tersebut.' : ' Email aktivasi GAGAL dikirim — buat token aktivasi ulang atau bagikan kredensial manual.'}`;
		createEmail = '';
		createName = '';
		createRole = 'client';
		createPassword = '';
		createBusy = false;
		await load();
	}

	function openPasswordDialog(user: User) {
		pwUser = user;
		pwValue = '';
		pwConfirm = '';
		pwError = '';
		pwNotice = '';
	}

	async function submitSetPassword() {
		if (!pwUser) return;
		if (pwValue.trim().length < 10) {
			pwError = 'Password minimal 10 karakter.';
			return;
		}
		if (pwValue !== pwConfirm) {
			pwError = 'Konfirmasi password tidak cocok.';
			return;
		}
		pwBusy = true;
		pwError = '';
		pwNotice = '';
		const res = await apiRequest(`/v1/platform/operations/users/${pwUser.id}/password`, {
			method: 'PUT',
			body: JSON.stringify({ new_password: pwValue })
		});
		if (!res.ok) {
			pwError = await platformError(res);
			pwBusy = false;
			return;
		}
		pwNotice = `Password ${pwUser.email} berhasil diperbarui. Sesi lama otomatis keluar.`;
		pwValue = '';
		pwConfirm = '';
		pwBusy = false;
	}

	function openRoleDialog(user: User) {
		roleTarget = user;
		roleNext = user.role === 'partner' ? 'client' : 'partner';
		roleKeepBrands = false;
		roleError = '';
		roleNotice = '';
	}

	async function submitRoleChange() {
		if (!roleTarget) return;
		roleBusy = true;
		roleError = '';
		roleNotice = '';
		const res = await apiRequest(`/v1/platform/operations/users/${roleTarget.id}/role`, {
			method: 'PUT',
			body: JSON.stringify({ role: roleNext, keep_brands: roleKeepBrands })
		});
		if (!res.ok) {
			roleError = await platformError(res);
			roleBusy = false;
			return;
		}
		const result = await res.json();
		// Daftar ini difilter per peran: kalau peran barunya tidak lagi cocok,
		// baris dibuang sekarang (langsung hilang) — bukan sekadar ganti label.
		if (roleFilter && result.role !== roleFilter) {
			rows = rows.filter((item) => item.id !== result.id);
			void load(searchText, roleFilter);
		} else {
			rows = rows.map((item) => (item.id === result.id ? { ...item, role: result.role } : item));
		}
		const kept = result.retained_brands.length > 0 ? ` Brand yang dipertahankan: ${result.retained_brands.join(', ')}.` : '';
		roleNotice =
			`${roleTarget.display_name} sekarang berperan sebagai ${result.role === 'partner' ? tr('Mitra') : tr('Klien')}.` +
			(result.removed_brands > 0 ? ` ${result.removed_brands} brand kosong dihapus.` : '') +
			kept +
			(result.note ? ` ${result.note}` : '');
		roleTarget = null;
		roleBusy = false;
	}

	async function openTcDialog(user: User) {
		tcUser = user;
		tcBrandId = '';
		tcAmount = '';
		tcReason = '';
		tcError = '';
		tcNotice = '';
		const res = await apiRequest(`/v1/platform/operations/users/${user.id}/brands`);
		if (!res.ok) {
			tcError = await platformError(res);
			return;
		}
		const data = await res.json();
		tcBrands = data.brands ?? [];
		if (tcBrands.length === 1) tcBrandId = tcBrands[0].id;
	}

	async function submitTcAdjustment() {
		if (!tcUser || !tcBrandId) {
			tcError = tr('Pilih brand dulu.');
			return;
		}
		const tc = parseInt(tcAmount, 10);
		if (isNaN(tc) || tc <= 0) {
			tcError = tr('Jumlah TC harus angka positif.');
			return;
		}
		if (tc > TC_MAX) {
			tcError = tr('Maksimal 10.000.000 TC per penyesuaian (1 TC = Rp1.000).');
			return;
		}
		if (tcReason.trim().length < 3) {
			tcError = 'Alasan minimal 3 karakter.';
			return;
		}
		tcBusy = true;
		tcError = '';
		tcNotice = '';
		const res = await apiRequest('/v1/platform/operations/credit-adjustment', {
			method: 'POST',
			body: JSON.stringify({ brand_id: tcBrandId, tc, reason: tcReason.trim() })
		});
		if (!res.ok) {
			tcError = await platformError(res);
			tcBusy = false;
			return;
		}
		const result = await res.json();
		tcNotice = `${fmtNum(result.tc_added)} TC ditambahkan. Saldo available sekarang ${fmtNum(result.wallet.available_tc)} TC.`;
		// Kolom "Jumlah Saldo" ikut bertambah tanpa menunggu muat ulang daftar.
		const targetId = tcUser.id;
		rows = rows.map((item) =>
			item.id === targetId ? { ...item, available_tc: (item.available_tc ?? 0) + result.tc_added } : item
		);
		tcAmount = '';
		tcReason = '';
		tcBusy = false;
	}

	const start = $derived((pageNum - 1) * PAGE_SIZE);
	const visibleRows = $derived(rows.slice(start, start + PAGE_SIZE));
	const denied = $derived(Boolean(!loading && rows.length === 0 && error.includes('Akses platform')));
	const dialogMeta = $derived(statusTarget ? statusMeta[statusKind(statusTarget)] : null);
</script>

<section>
	<PageHeader title={tr("Daftar Klien")} description={tr("Setujui pendaftaran klien baru dan kelola status akun serta batas pemakaian harian.")}>
		<button class="app-btn app-btn-soft" type="button" onclick={openCreate}>{tr("Tambah Pengguna")}</button>
		<span class="rounded-full px-3 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{rows.length} users</span>
	</PageHeader>

	<Feedback {loading} {error} loadingText="Memuat direktori user…" />

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke direktori user platform.")}</p>
		</div>
	{:else}
		<div class="app-card mt-4 flex flex-wrap items-center gap-3 p-3">
			<form class="flex flex-1 items-center gap-2" role="search" onsubmit={submitSearch}>
				<label class="app-input flex min-w-[220px] flex-1 items-center gap-2 !py-1.5">
					<Search size={15} class="text-muted-foreground" />
					<input
						class="w-full bg-transparent text-sm outline-none"
						name="search"
						aria-label={tr("Cari user")}
						placeholder={tr("Cari nama atau email...")}
						bind:value={searchText}
					/>
				</label>
				<button class="app-btn app-btn-soft" type="submit" disabled={loading}>{tr("Cari")}</button>
			</form>
		</div>

		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 980px;">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">User</th>
						<th class="px-4 py-2.5 font-medium">Status</th>
						<th class="px-4 py-2.5 font-medium">Brands</th>
						<th class="px-4 py-2.5 font-medium">{tr("Jumlah Saldo")}</th>
						<th class="px-4 py-2.5 font-medium">Jatah Generate</th>
						<th class="px-4 py-2.5 font-medium">Created</th>
						<th class="px-4 py-2.5"></th>
					</tr>
				</thead>
				<tbody>
					{#each visibleRows as user (user.id)}
						{@const meta = statusMeta[statusKind(user)]}
						<tr class="border-t">
							<td class="px-4 py-2.5">
								<div class="flex items-center gap-3">
									<span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-xs font-bold" style="background-color: var(--secondary); color: var(--muted-foreground);">
										{initials(user)}
									</span>
									<div>
										<p class="flex items-center gap-2 font-medium">
											{user.display_name}
											{#if user.id === currentUserId}<Badge label="Anda" tone="neutral" />{/if}
										</p>
										<p class="text-xs text-muted-foreground">{user.email}</p>
										{#if user.role === 'partner'}
											<Badge label={tr("Mitra")} tone="accent" />
										{:else if user.role === 'client'}
											<span class="mt-1 inline-block rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{tr("Klien")}</span>
										{/if}
									</div>
								</div>
							</td>
							<td class="px-4 py-2.5"><Badge label={meta.label} tone={meta.tone} /></td>
							<td class="px-4 py-2.5 tabular-nums">{user.brand_count}</td>
							<td class="px-4 py-2.5">
								<!-- Saldo TC akun klien: total available di semua brand milik user. -->
								<span class="font-semibold tabular-nums">{fmtNum(user.available_tc ?? 0)}</span>
								<span class="ml-1 text-xs text-muted-foreground">TC</span>
							</td>
							<td class="px-4 py-2.5">
								<div class="flex items-center gap-2">
									<input
										class="app-input !w-[90px] !py-1 !text-xs"
										inputmode="numeric"
										value={user.writer_generate_quota ?? ''}
										placeholder="3"
										disabled={saving === user.id}
										title={user.writer_generate_quota == null ? 'Default 3 (belum diatur): 1x generate pertama + 2x regenerasi' : `Jatah diatur manual: ${user.writer_generate_quota}x`}
										onblur={(e) => {
											const v = (e.currentTarget as HTMLInputElement).value.trim();
											const cur = user.writer_generate_quota;
											const next = v === '' ? null : parseInt(v, 10);
											const same = v === '' ? cur == null : Number.isFinite(next) && next === (cur ?? 3);
											if (!same) void updateGenQuota(user.id, v);
										}}
									/>
									<button
										class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs"
										type="button"
										disabled={saving === user.id}
										onclick={() => {
											const next = prompt(tr('Jatah generate per artikel (1-999, kosongkan = default 3):'), String(user.writer_generate_quota ?? ''));
											void updateGenQuota(user.id, next ?? '');
										}}
									>{saving === user.id ? '...' : 'Set'}</button>
									<Badge
										label={`${(user.writer_generate_quota ?? 3)}x`}
										tone={(user.writer_generate_quota ?? 0) >= 99 ? 'accent' : 'neutral'}
									/>
								</div>
							</td>
							<td class="px-4 py-2.5 text-muted-foreground">{fmtDate(user.created_at)}</td>
							<td class="px-4 py-2.5">
								{#if user.id !== currentUserId}
									{#if statusKind(user) === 'pending'}
										<button class="app-btn !min-h-7 !px-2.5 !text-xs" style="background-color: var(--success-soft); color: var(--success);" type="button" onclick={() => (statusTarget = user)}>Setujui</button>
									{:else}
										<button
											class="rounded-lg p-1.5 hover:bg-secondary"
											type="button"
											aria-label={`Aksi ${user.display_name}`}
											onclick={(e) => {
												const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
												menuPos = { top: rect.bottom + 4, right: window.innerWidth - rect.right };
												openMenu = openMenu === user.id ? null : user.id;
											}}
										><Ellipsis size={19} /></button>
										{#if openMenu === user.id}
											<div class="fixed inset-0 z-40" role="presentation" onclick={() => (openMenu = null)}></div>
											<div
												class="app-card fixed z-50 min-w-[210px] p-1.5 text-sm shadow-md"
												style="top: {menuPos.top}px; right: {menuPos.right}px;"
											>
												{#if user.role === 'client' || user.role === 'partner'}
													<button class="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left hover:bg-secondary" type="button" onclick={() => { openMenu = null; openRoleDialog(user); }}>
														<ArrowLeftRight size={15} /> Ubah jadi {user.role === 'partner' ? tr('Klien') : tr('Mitra')}
													</button>
												{/if}
												<button class="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left hover:bg-secondary" type="button" onclick={() => { openMenu = null; void openTcDialog(user); }}>
													<Coins size={15} />{tr("Tambah TC")}</button>
												{#if isSuperAdmin}
													<button class="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left hover:bg-secondary" type="button" onclick={() => { openMenu = null; openPasswordDialog(user); }}>
														<Key size={15} /> Set password
													</button>
												{/if}
												<button
													class="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left hover:bg-secondary"
													style={user.is_active ? 'color: var(--destructive);' : undefined}
													type="button"
													onclick={() => { openMenu = null; statusTarget = user; }}
												>{user.is_active ? 'Suspend user' : 'Reactivate user'}</button>
											</div>
										{/if}
									{/if}
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && rows.length === 0}
				<div class="flex flex-col items-center px-4 py-12 text-center">
					<UserRound size={26} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">{tr("Tidak ada user yang cocok.")}</p>
				</div>
			{/if}
			<div class="px-4 pb-4">
				<Pagination
					total={rows.length}
					limit={PAGE_SIZE}
					offset={start}
					onChange={(next) => (pageNum = Math.floor(next / PAGE_SIZE) + 1)}
				/>
			</div>
		</div>
	{/if}
</section>

{#if statusTarget}
	<Modal
		title={dialogMeta?.label === tr('Menunggu') ? 'Setujui pendaftaran?' : statusTarget?.is_active ? 'Suspend user?' : 'Reactivate user?'}
		label="Konfirmasi status user"
		maxWidth="md"
		onClose={() => (statusTarget = null)}
	>
		<p class="text-sm text-muted-foreground">
			{dialogMeta?.label === tr('Menunggu')
				? `${statusTarget?.display_name} akan langsung bisa login dan mengakses platform.`
				: statusTarget?.is_active
					? `${statusTarget?.display_name} akan langsung ditolak saat memakai token atau login kembali.`
					: `${statusTarget?.display_name} akan kembali dapat login dan mengakses brand miliknya.`}
		</p>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => (statusTarget = null)}>{tr("Batal")}</button>
			<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void updateStatus()}>
				{busy ? 'Menyimpan...' : dialogMeta?.label === tr('Menunggu') ? 'Setujui' : statusTarget?.is_active ? 'Suspend' : 'Reactivate'}
			</button>
		{/snippet}
	</Modal>
{/if}

{#if createOpen}
	<Modal title={tr("Tambah Pengguna")} label={tr("Tambah pengguna")} maxWidth="md" onClose={() => (createOpen = false)}>
		<p class="text-sm text-muted-foreground">{tr("Buat akun langsung tanpa pendaftaran umum. Akun langsung aktif; user mengatur password lewat link aktivasi yang dikirim ke emailnya (berlaku 1 jam).")}</p>
		<div class="mt-4 flex flex-col gap-3">
			{#if createError}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{createError}</div>
			{/if}
			{#if createNotice}
				<div class="flex items-start gap-2 text-sm" style="color: var(--success);"><CircleCheck size={16} class="mt-0.5 shrink-0" />{createNotice}</div>
			{/if}
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Email")}</span>
				<input class="app-input w-full" type="email" placeholder={tr("nama@perusahaan.com")} bind:value={createEmail} />
			</label>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Nama lengkap / nama perusahaan")}</span>
				<input class="app-input w-full" placeholder={tr("Nama Anda")} bind:value={createName} />
			</label>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Peran")}</span>
				<Select
					ariaLabel={tr("Peran")}
					minWidth={260}
					value={createRole}
					onchange={(v) => (createRole = v === 'partner' ? 'partner' : 'client')}
					options={[
						{ value: 'client', label: tr('Klien — kelola brand & konten') },
						{ value: 'partner', label: tr('Mitra — media partner') }
					]}
				/>
			</label>
			{#if isSuperAdmin}
				<label class="block">
					<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Password awal (opsional)")}</span>
					<input class="app-input w-full" type="password" placeholder={tr("Kosongkan untuk kirim link aktivasi via email")} bind:value={createPassword} />
					<span class="mt-1 block text-xs text-muted-foreground">{tr("Diisi (min 10 karakter) = akun langsung bisa login tanpa email. Kosong = pakai link aktivasi.")}</span>
				</label>
			{/if}
		</div>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => (createOpen = false)}>{tr("Tutup")}</button>
			<button
				class="app-btn app-btn-primary"
				type="button"
				disabled={createBusy || createEmail.trim().length < 5 || createName.trim().length < 2}
				onclick={() => void submitCreateUser()}
			>{createBusy ? 'Membuat...' : 'Buat Pengguna'}</button>
		{/snippet}
	</Modal>
{/if}

{#if tcUser}
	<Modal title={`Tambah TC — ${tcUser.display_name}`} label={tr("Tambah TC")} maxWidth="lg" onClose={() => (tcUser = null)}>
		<p class="text-sm text-muted-foreground">{tr("Tambahkan Tauthub Credit (TC) manual ke wallet brand milik user ini. Gunakan untuk kompensasi topup gagal atau penyesuaian.")}</p>
		<div class="mt-4 flex flex-col gap-3">
			{#if tcError}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{tcError}</div>
			{/if}
			{#if tcNotice}
				<div class="flex items-start gap-2 text-sm" style="color: var(--success);"><Coins size={16} class="mt-0.5 shrink-0" />{tcNotice}</div>
			{/if}
			<div>
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Brand tujuan</span>
				{#if tcBrands.length === 0}
					<p class="text-xs text-muted-foreground">User ini tidak memiliki brand.</p>
				{:else}
					<Select
						ariaLabel="Brand tujuan"
						placeholder={tr("Pilih brand…")}
						minWidth={260}
						value={tcBrandId}
						onchange={(v) => (tcBrandId = v)}
						options={[
							{ value: '', label: tr('Pilih brand…') },
							...tcBrands.map((b) => ({ value: b.id, label: `${b.name} — ${fmtNum(b.available_tc)} TC tersedia` }))
						]}
					/>
				{/if}
			</div>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Jumlah TC")}</span>
				<input class="app-input w-full" type="number" min="1" max={TC_MAX} placeholder={tr("cth: 1000")} value={tcAmount} oninput={(e) => (tcAmount = (e.currentTarget as HTMLInputElement).value)} />
				<span class="mt-1 block text-xs text-muted-foreground">{tr('Maksimal 10.000.000 TC per penyesuaian, 1 TC = Rp1.000.')}</span>
			</label>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Alasan (wajib)</span>
				<textarea
					class="app-input w-full"
					rows={3}
					placeholder={tr("cth: Kompensasi topup gagal — pembayaran sukses tapi saldo tidak masuk")}
					bind:value={tcReason}
				></textarea>
				<span class="mt-1 block text-xs text-muted-foreground">Akan tercatat di ledger & audit log brand.</span>
			</label>
		</div>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => (tcUser = null)}>{tr("Batal")}</button>
			<button class="app-btn app-btn-primary" type="button" disabled={tcBusy || tcBrands.length === 0} onclick={() => void submitTcAdjustment()}>
				<Coins size={15} />{tcBusy ? 'Menambah…' : tr('Tambah TC')}
			</button>
		{/snippet}
	</Modal>
{/if}

{#if pwUser}
	<Modal title={`Set password — ${pwUser.display_name}`} label="Set password" maxWidth="md" onClose={() => (pwUser = null)}>
		<p class="text-sm text-muted-foreground"> {tr("Tetapkan password baru untuk")} {pwUser.email} tanpa link email. Sesi login lama user otomatis dikeluarkan, dan pemilik akun diberi tahu lewat email.
		</p>
		<div class="mt-4 flex flex-col gap-3">
			{#if pwError}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{pwError}</div>
			{/if}
			{#if pwNotice}
				<div class="flex items-start gap-2 text-sm" style="color: var(--success);"><CircleCheck size={16} class="mt-0.5 shrink-0" />{pwNotice}</div>
			{/if}
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Password baru")}</span>
				<input class="app-input w-full" type="password" placeholder={tr("Minimal 10 karakter")} bind:value={pwValue} />
			</label>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Ulangi password</span>
				<input class="app-input w-full" type="password" placeholder={tr("Ketik ulang password")} bind:value={pwConfirm} />
			</label>
		</div>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => (pwUser = null)}>{tr("Tutup")}</button>
			<button
				class="app-btn app-btn-primary"
				type="button"
				disabled={pwBusy || pwValue.trim().length < 10 || pwValue !== pwConfirm}
				onclick={() => void submitSetPassword()}
			>{pwBusy ? 'Menyimpan...' : 'Set password'}</button>
		{/snippet}
	</Modal>
{/if}

{#if roleTarget}
	<Modal title={`Ubah peran — ${roleTarget.display_name}`} label={tr("Ubah peran")} maxWidth="md" onClose={() => (roleTarget = null)}>
		<p class="text-sm text-muted-foreground">
			{roleNext === 'partner'
				? tr('Akun ini akan menjadi Mitra: baris mitra aktif dibuat sehingga bisa mengelola media dan menerima publikasi.')
				: tr('Akun ini akan menjadi Klien: brand default dan profilnya dibuat otomatis.')}
		</p>
		<div class="mt-4 flex flex-col gap-3">
			{#if roleError}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{roleError}</div>
			{/if}
			{#if roleNotice}
				<div class="flex items-start gap-2 text-sm" style="color: var(--success);"><CircleCheck size={16} class="mt-0.5 shrink-0" />{roleNotice}</div>
			{/if}
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Peran baru")}</span>
				<Select
					ariaLabel={tr("Peran baru")}
					minWidth={260}
					value={roleNext}
					onchange={(v) => (roleNext = v === 'partner' ? 'partner' : 'client')}
					options={[
						{ value: 'partner', label: tr('Mitra — media partner') },
						{ value: 'client', label: tr('Klien — kelola brand & konten') }
					]}
				/>
			</label>
			{#if roleNext === 'partner'}
				<label class="flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={roleKeepBrands} />
					Pertahankan brand klien (jangan hapus brand kosong)
				</label>
			{/if}
			<p class="text-xs text-muted-foreground">
				{roleNext === 'partner'
					? tr('Brand kosong tanpa publikasi/transaksi dihapus otomatis. Brand yang sudah dipakai tetap dipertahankan.')
					: 'Pengguna perlu login ulang agar sesi memakai peran baru.'}
			</p>
		</div>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => (roleTarget = null)}>{tr("Tutup")}</button>
			<button class="app-btn app-btn-primary" type="button" disabled={roleBusy} onclick={() => void submitRoleChange()}>
				{roleBusy ? 'Menyimpan...' : `Ubah jadi ${roleNext === 'partner' ? tr('Mitra') : tr('Klien')}`}
			</button>
		{/snippet}
	</Modal>
{/if}
