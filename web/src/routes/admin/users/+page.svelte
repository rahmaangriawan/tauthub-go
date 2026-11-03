<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Kelola user (klien & mitra) — port app/admin/users/page.tsx:
	// setujui pendaftaran, suspend/reactivate, kuota token harian, jatah
	// generate Writer, sesuaikan saldo manual, set password (super admin),
	// ubah peran, dan tambah pengguna manual.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		ArrowLeftRight, Coins, Ellipsis, Key, Search, UserRound, Plus, TriangleAlert,
		CircleCheck, Ban, Check
	} from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, ensureValidSession, getStoredSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDate, fmtNum } from '$lib/admin-format';
	import { rp } from '$lib/credits/rupiah';
	import { platformError } from '../_platform';
	import { swrPeek, swrPut } from '$lib/swr';

	type GenQuota = { user_id: string; writer_generate_quota: number | null; effective: number };
	type User = {
		id: string; email: string; display_name: string; role?: string; wa_number?: string | null;
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
	// Total klien/mitra sebenarnya. GET /users tidak mengirim total (array
	// mentah, maks 200 baris), jadi angkanya diambil dari /operations/summary.
	let totals = $state<{ client: number; partner: number } | null>(null);

	// Dialog status (approve/suspend/reactivate)
	let statusTarget = $state<User | null>(null);
	// Dialog penyesuaian saldo manual
	let tcUser = $state<User | null>(null);
	let tcBrands = $state<BrandOption[]>([]);
	let tcBrandId = $state('');
	let tcAmount = $state('');
	// Mode penyesuaian: tambah atau kurangi saldo brand tujuan.
	let tcMode = $state<'add' | 'deduct'>('add');
	// Batas server: POST /credit-adjustment menolak nominal > Rp10 miliar.
	// Satuan saldo = rupiah penuh (bawaan konversi satuan dari TC, 1 TC lama = Rp1.000).
	const SALDO_MAX = 10_000_000_000;
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
			void loadTotals();
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

	// Total per peran untuk header (bukan jumlah baris yang tampil).
	async function loadTotals() {
		const res = await apiRequest('/v1/platform/operations/summary');
		if (!res.ok) return;
		const data = await res.json();
		const eco = data?.ecosystem;
		if (eco) totals = { client: Number(eco.client_users) || 0, partner: Number(eco.partner_users) || 0 };
	}

	async function load(search = '', role = roleFilter) {		error = '';
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
		tcMode = 'add';
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
		const amount = parseInt(tcAmount, 10);
		if (isNaN(amount) || amount <= 0) {
			tcError = tr('Jumlah saldo harus angka positif.');
			return;
		}
		if (amount > SALDO_MAX) {
			tcError = tr('Nominal maksimal Rp10.000.000.000 per penyesuaian.');
			return;
		}
		if (tcMode === 'deduct') {
			const brand = tcBrands.find((b) => b.id === tcBrandId);
			if (brand && amount > brand.available_tc) {
				tcError = tr('Saldo tidak cukup: tersedia Rp {n}.').replace('{n}', fmtNum(brand.available_tc));
				return;
			}
		}
		if (tcReason.trim().length < 3) {
			tcError = 'Alasan minimal 3 karakter.';
			return;
		}
		// Nilai negatif = kurangi saldo; server yang menyimpan sebagai satu entri
		// ledger bertanda, jadi riwayatnya tetap satu baris per penyesuaian.
		const tc = tcMode === 'deduct' ? -amount : amount;
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
		tcNotice =
			tcMode === 'deduct'
				? `${rp(Math.abs(result.tc_added))} dikurangi. Saldo tersisa ${rp(result.wallet.available_tc)}.`
				: `${rp(result.tc_added)} ditambahkan. Saldo tersisa ${rp(result.wallet.available_tc)}.`;
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
	// Antrean persetujuan: hanya yang benar-benar menunggu, bukan seluruh daftar.
	const pendingRows = $derived(rows.filter((u) => statusKind(u) === 'pending'));
</script>

<section>
	<PageHeader title={tr("Daftar Klien")} description={tr("Setujui pendaftaran klien baru dan kelola status akun serta batas pemakaian harian.")}>
		{#if totals}
			<span class="total-clients">
				<span class="tabular-nums">{fmtNum(totals.client)}</span>
				{tr('klien')}
			</span>
		{/if}
		<button class="app-btn app-btn-soft" type="button" onclick={openCreate}>{tr("Tambah Pengguna")}</button>
	</PageHeader>

	<Feedback {loading} {error} loadingText="Memuat direktori user…" />

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke direktori user platform.")}</p>
		</div>
	{:else}
		<!-- Antrean persetujuan: tampil hanya bila ada yang menunggu -->
		{#if !loading && pendingRows.length > 0}
			<div class="queue mt-5">
				<div class="flex items-baseline justify-between gap-4">
					<h2 class="text-sm font-medium">
						{pendingRows.length} {tr('pendaftaran menunggu persetujuan')}
					</h2>
					<span class="text-xs text-muted-foreground">{tr('Akun belum bisa login')}</span>
				</div>
				<ul class="mt-3">
					{#each pendingRows.slice(0, 5) as user (user.id)}
						<li class="flex items-center gap-3 py-2.5">
							<span class="min-w-0 flex-1">
								<span class="block truncate text-sm font-medium">{user.display_name}</span>
								<span class="block truncate text-xs text-muted-foreground">{user.email}</span>
							</span>
							<span class="shrink-0 text-xs tabular-nums text-muted-foreground">{fmtDate(user.created_at)}</span>
							<button class="queue-approve" type="button" onclick={() => (statusTarget = user)}>
								<Check size={14} strokeWidth={1.5} />
								{tr('Setujui')}
							</button>
						</li>
					{/each}
				</ul>
				{#if pendingRows.length > 5}
					<p class="mt-2 text-xs text-muted-foreground">{tr('dan')} {pendingRows.length - 5} {tr('lainnya di tabel di bawah')}</p>
				{/if}
			</div>
		{/if}

		<!-- Pencarian -->
		<div class="mt-5 flex flex-wrap items-center gap-2">
			<form class="flex flex-1 items-center gap-2" role="search" onsubmit={submitSearch}>
				<label class="search-box">
					<Search size={15} strokeWidth={1.5} class="shrink-0 text-muted-foreground" />
					<input
						class="w-full bg-transparent text-sm outline-none"
						name="search"
						aria-label={tr("Cari user")}
						placeholder={tr("Cari nama, email, atau nomor WA")}
						bind:value={searchText}
					/>
				</label>
				<button class="app-btn app-btn-soft" type="submit" disabled={loading}>{tr("Cari")}</button>
			</form>
			{#if !loading && searchText.trim()}
				<span class="text-xs tabular-nums text-muted-foreground">
					{visibleRows.length} {tr('hasil')}
				</span>
			{/if}
		</div>

		<div class="app-card mt-4 overflow-x-auto">
			<table class="ledger">
				<thead>
					<tr>
						<th class="col-user">{tr('Akun')}</th>
						<th class="col-status">{tr('Status')}</th>
						<th class="col-num">{tr('Brand')}</th>
						<th class="col-money">{tr('Saldo')}</th>
						<th class="col-quota">{tr('Jatah generate')}</th>
						<th class="col-date">{tr('Terdaftar')}</th>
						<th class="col-act"></th>
					</tr>
				</thead>
				<tbody>
					{#each visibleRows as user (user.id)}
						{@const kind = statusKind(user)}
						{@const meta = statusMeta[kind]}
						<tr class="ledger-row" data-status={kind}>
							<td class="col-user">
								<span class="row-spine" aria-hidden="true"></span>
								<span class="mb-stack">
									<span class="flex min-w-0 items-center gap-2">
										<span class="truncate text-sm font-medium">{user.display_name}</span>
										{#if user.id === currentUserId}<span class="you-tag">{tr('Anda')}</span>{/if}
									</span>
									<span class="meta-line">
										<span class="truncate">{user.email}</span>
										<span aria-hidden="true">·</span>
										<span class="role-badge" data-role={user.role === 'partner' ? 'partner' : 'client'}>{user.role === 'partner' ? tr('Mitra') : tr('Klien')}</span>
									</span>
									<span class="block truncate text-xs text-muted-foreground">
										{#if user.wa_number}
											<a class="hover:underline" href={`https://wa.me/${user.wa_number.replace(/[^0-9]/g, '')}`} target="_blank" rel="noreferrer">{user.wa_number}</a>
										{:else}
											{tr('Belum ada nomor WA')}
										{/if}
									</span>
								</span>
							</td>
							<td class="col-status">
								<span class="status-text" data-tone={meta.tone}>
									<span class="status-dot" data-tone={meta.tone} aria-hidden="true"></span>
									{meta.label}
								</span>
							</td>
							<td class="col-num tabular-nums">{user.brand_count}</td>
							<td class="col-money tabular-nums">
								<span class="money-cell" data-funded={(user.available_tc ?? 0) > 0}>
									<span>{rp(user.available_tc ?? 0)}</span>
									{#if user.id !== currentUserId}
										<button
											class="money-adjust"
											type="button"
											title={tr('Tambah atau kurangi saldo')}
											aria-label={`${tr('Sesuaikan saldo')} ${user.display_name}`}
											onclick={() => void openTcDialog(user)}
										>
											<Plus size={13} strokeWidth={2} />
										</button>
									{/if}
								</span>
							</td>
							<td class="col-quota">
								<div class="flex items-center gap-2">
									<input
										class="app-input quota-input"
										inputmode="numeric"
										value={user.writer_generate_quota ?? ''}
										placeholder="3"
										disabled={saving === user.id}
										aria-label={`${tr('Jatah generate')} ${user.display_name}`}
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
										class="app-btn app-btn-soft quota-set"
										type="button"
										disabled={saving === user.id}
										onclick={() => {
											const next = prompt(tr('Jatah generate per artikel (1-999, kosongkan = default 3):'), String(user.writer_generate_quota ?? ''));
											void updateGenQuota(user.id, next ?? '');
										}}
									>{saving === user.id ? 'Menyimpan' : 'Set'}</button>
									<span class="quota-mark" data-default={user.writer_generate_quota == null}>
										{(user.writer_generate_quota ?? 3)}x
									</span>
								</div>
							</td>
							<td class="col-date tabular-nums text-muted-foreground">{fmtDate(user.created_at)}</td>
							<td class="col-act">
								{#if user.id !== currentUserId}
									{#if kind === 'pending'}
										<button class="queue-approve" type="button" onclick={() => (statusTarget = user)}>
											<Check size={14} strokeWidth={1.5} />
											{tr('Setujui')}
										</button>
									{:else}
										<button
											class="row-menu-btn"
											type="button"
											aria-label={`Aksi ${user.display_name}`}
											onclick={(e) => {
												const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
												menuPos = { top: rect.bottom + 4, right: window.innerWidth - rect.right };
												openMenu = openMenu === user.id ? null : user.id;
											}}
										><Ellipsis size={16} strokeWidth={1.5} /></button>
										{#if openMenu === user.id}
											<div class="fixed inset-0 z-40" role="presentation" onclick={() => (openMenu = null)}></div>
											<div
												class="app-card row-menu fixed z-50"
												style="top: {menuPos.top}px; right: {menuPos.right}px;"
											>
												{#if user.role === 'client' || user.role === 'partner'}
													<button class="row-menu-item" type="button" onclick={() => { openMenu = null; openRoleDialog(user); }}>
														<ArrowLeftRight size={15} strokeWidth={1.5} /> {tr('Ubah jadi')} {user.role === 'partner' ? tr('Klien') : tr('Mitra')}
													</button>
												{/if}
												<button class="row-menu-item" type="button" onclick={() => { openMenu = null; void openTcDialog(user); }}>
													<Coins size={15} strokeWidth={1.5} />{tr("Sesuaikan Saldo")}
												</button>
												{#if isSuperAdmin}
													<button class="row-menu-item" type="button" onclick={() => { openMenu = null; openPasswordDialog(user); }}>
														<Key size={15} strokeWidth={1.5} /> {tr('Set password')}
													</button>
												{/if}
												<button
													class="row-menu-item"
													data-danger={user.is_active ? 'true' : undefined}
													type="button"
													onclick={() => { openMenu = null; statusTarget = user; }}
												>
													{#if user.is_active}
														<Ban size={15} strokeWidth={1.5} /> {tr('Suspend user')}
													{:else}
														<Check size={15} strokeWidth={1.5} /> {tr('Reactivate user')}
													{/if}
												</button>
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
					<UserRound size={20} strokeWidth={1.5} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">{tr("Tidak ada user yang cocok.")}</p>
					{#if searchText.trim()}
						<button class="mt-3 text-xs font-medium text-primary hover:underline" type="button" onclick={() => { searchText = ''; pageNum = 1; void load(''); }}>
							{tr('Hapus pencarian')}
						</button>
					{/if}
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
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} strokeWidth={1.5} class="mt-0.5 shrink-0" />{createError}</div>
			{/if}
			{#if createNotice}
				<div class="flex items-start gap-2 text-sm" style="color: var(--success);"><CircleCheck size={16} strokeWidth={1.5} class="mt-0.5 shrink-0" />{createNotice}</div>
			{/if}
			<label class="block">
				<span class="field-label">{tr("Email")}</span>
				<input class="app-input w-full" type="email" placeholder={tr("nama@perusahaan.com")} bind:value={createEmail} />
			</label>
			<label class="block">
				<span class="field-label">{tr("Nama atau perusahaan")}</span>
				<input class="app-input w-full" placeholder={tr("Budi Santoso atau PT Maju Jaya")} bind:value={createName} />
			</label>
			<label class="block">
				<span class="field-label">{tr("Peran")}</span>
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
					<span class="field-label">{tr("Password awal (opsional)")}</span>
					<input class="app-input w-full" type="password" placeholder={tr("Kosongkan untuk kirim link aktivasi via email")} bind:value={createPassword} />
					<span class="field-hint">{tr("Diisi (min 10 karakter) = akun langsung bisa login tanpa email. Kosong = pakai link aktivasi.")}</span>
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
	<Modal title={`Sesuaikan Saldo — ${tcUser.display_name}`} label={tr("Sesuaikan Saldo")} maxWidth="lg" onClose={() => (tcUser = null)}>
		<p class="text-sm text-muted-foreground">{tr("Tambahkan atau kurangi saldo (rupiah) pada wallet brand milik user ini. Gunakan untuk kompensasi topup gagal, koreksi saldo, atau penyesuaian lain.")}</p>
		<div class="mt-4 flex flex-col gap-3">
			{#if tcError}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} strokeWidth={1.5} class="mt-0.5 shrink-0" />{tcError}</div>
			{/if}
			{#if tcNotice}
				<div class="flex items-start gap-2 text-sm" style="color: var(--success);"><Coins size={16} strokeWidth={1.5} class="mt-0.5 shrink-0" />{tcNotice}</div>
			{/if}
			<div>
				<span class="field-label">{tr('Brand tujuan')}</span>
				{#if tcBrands.length === 0}
					<p class="text-xs text-muted-foreground">{tr('User ini tidak memiliki brand.')}</p>
				{:else}
					<Select
						ariaLabel="Brand tujuan"
						placeholder={tr("Pilih brand…")}
						minWidth={260}
						value={tcBrandId}
						onchange={(v) => (tcBrandId = v)}
						options={[
							{ value: '', label: tr('Pilih brand…') },
							...tcBrands.map((b) => ({ value: b.id, label: `${b.name} — saldo ${rp(b.available_tc)}` }))
						]}
					/>
				{/if}
			</div>
			<div>
				<span class="field-label">{tr('Jenis penyesuaian')}</span>
				<div class="segment" role="group" aria-label={tr('Jenis penyesuaian')}>
					<button
						type="button"
						class="segment-btn"
						data-active={tcMode === 'add' ? 'true' : 'false'}
						aria-pressed={tcMode === 'add'}
						onclick={() => (tcMode = 'add')}
					>{tr('Tambah saldo')}</button>
					<button
						type="button"
						class="segment-btn"
						data-active={tcMode === 'deduct' ? 'true' : 'false'}
						aria-pressed={tcMode === 'deduct'}
						onclick={() => (tcMode = 'deduct')}
					>{tr('Kurangi saldo')}</button>
				</div>
			</div>
			<label class="block">
				<span class="field-label">{tr("Jumlah saldo")}</span>
				<input class="app-input w-full" type="number" min="1" max={SALDO_MAX} placeholder={tr("cth: 1000000")} value={tcAmount} oninput={(e) => (tcAmount = (e.currentTarget as HTMLInputElement).value)} />
				<span class="field-hint">
					{tcMode === 'deduct'
						? tr('Masukkan nominal rupiah penuh — saldo brand akan dikurangi sebesar angka ini, maksimal Rp10.000.000.000.')
						: tr('Nominal rupiah penuh, maksimal Rp10.000.000.000 per penyesuaian.')}
				</span>
			</label>
			<label class="block">
				<span class="field-label">{tr('Alasan (wajib)')}</span>
				<textarea
					class="app-input w-full"
					rows={3}
					placeholder={tr("cth: Kompensasi topup gagal — pembayaran sukses tapi saldo tidak masuk")}
					bind:value={tcReason}
				></textarea>
				<span class="field-hint">{tr('Akan tercatat di ledger & audit log brand.')}</span>
			</label>
		</div>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => (tcUser = null)}>{tr("Batal")}</button>
			<button class="app-btn app-btn-primary" type="button" disabled={tcBusy || tcBrands.length === 0} onclick={() => void submitTcAdjustment()}>
				<Coins size={15} strokeWidth={1.5} />
				{tcBusy
					? (tcMode === 'deduct' ? tr('Mengurangi…') : tr('Menambah…'))
					: (tcMode === 'deduct' ? tr('Kurangi saldo') : tr('Tambah saldo'))}
			</button>
		{/snippet}
	</Modal>
{/if}

{#if pwUser}
	<Modal title={`Set password — ${pwUser.display_name}`} label="Set password" maxWidth="md" onClose={() => (pwUser = null)}>
		<p class="text-sm text-muted-foreground">
			{tr("Tetapkan password baru untuk")} {pwUser.email} {tr("tanpa link email. Sesi login lama user otomatis dikeluarkan, dan pemilik akun diberi tahu lewat email.")}
		</p>
		<div class="mt-4 flex flex-col gap-3">
			{#if pwError}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} strokeWidth={1.5} class="mt-0.5 shrink-0" />{pwError}</div>
			{/if}
			{#if pwNotice}
				<div class="flex items-start gap-2 text-sm" style="color: var(--success);"><CircleCheck size={16} strokeWidth={1.5} class="mt-0.5 shrink-0" />{pwNotice}</div>
			{/if}
			<label class="block">
				<span class="field-label">{tr("Password baru")}</span>
				<input class="app-input w-full" type="password" placeholder={tr("Minimal 10 karakter")} bind:value={pwValue} />
			</label>
			<label class="block">
				<span class="field-label">{tr('Ulangi password')}</span>
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
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} strokeWidth={1.5} class="mt-0.5 shrink-0" />{roleError}</div>
			{/if}
			{#if roleNotice}
				<div class="flex items-start gap-2 text-sm" style="color: var(--success);"><CircleCheck size={16} strokeWidth={1.5} class="mt-0.5 shrink-0" />{roleNotice}</div>
			{/if}
			<label class="block">
				<span class="field-label">{tr("Peran baru")}</span>
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
					{tr('Pertahankan brand klien (jangan hapus brand kosong)')}
				</label>
			{/if}
			<p class="text-xs text-muted-foreground">
				{roleNext === 'partner'
					? tr('Brand kosong tanpa publikasi/transaksi dihapus otomatis. Brand yang sudah dipakai tetap dipertahankan.')
					: tr('Pengguna perlu login ulang agar sesi memakai peran baru.')}
			</p>
		</div>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => (roleTarget = null)}>{tr("Tutup")}</button>
			<button class="app-btn app-btn-primary" type="button" disabled={roleBusy} onclick={() => void submitRoleChange()}>
				{roleBusy ? 'Menyimpan...' : `${tr('Ubah jadi')} ${roleNext === 'partner' ? tr('Mitra') : tr('Klien')}`}
			</button>
		{/snippet}
	</Modal>
{/if}

<style>
	.money-cell {
		display: inline-flex;
		align-items: center;
		justify-content: flex-end;
		gap: 6px;
		color: var(--muted-foreground);
		font-weight: 500;
	}
	.money-cell[data-funded='true'] { color: var(--foreground); }
	.money-adjust {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 22px;
		height: 22px;
		border: 1px solid var(--border);
		border-radius: 6px;
		color: var(--muted-foreground);
		opacity: 0.45;
		transition:
			opacity 120ms cubic-bezier(0.22, 1, 0.36, 1),
			background-color 120ms cubic-bezier(0.22, 1, 0.36, 1),
			color 120ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.ledger-row:hover .money-adjust,
	.money-adjust:focus-visible {
		opacity: 1;
	}
	.money-adjust:hover {
		background-color: var(--secondary);
		color: var(--foreground);
	}
	.money-adjust:focus-visible { outline: 2px solid var(--ring); outline-offset: 1px; }

	/* ── Total klien di header ── */
	.total-clients {
		display: inline-flex;
		align-items: baseline;
		gap: 4px;
		color: var(--muted-foreground);
		font-size: 12px;
	}
	.total-clients > span {
		color: var(--foreground);
		font-size: 15px;
		font-weight: 500;
	}

	/* ── Antrean persetujuan ── */
	.queue {
		border: 1px solid color-mix(in srgb, var(--warning) 22%, var(--border));
		border-left: 3px solid var(--warning);
		border-radius: var(--radius);
		padding: 16px 20px;
		background-color: var(--card);
	}
	.queue ul { display: flex; flex-direction: column; }
	.queue li + li { border-top: 1px solid color-mix(in srgb, var(--foreground) 6%, transparent); }
	.queue-approve {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		flex: 0 0 auto;
		min-height: 28px;
		border: 1px solid color-mix(in srgb, var(--success) 30%, var(--border));
		border-radius: 7px;
		padding: 0 10px;
		background-color: var(--card);
		color: var(--success);
		font-size: 12px;
		font-weight: 500;
		transition: background-color 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.queue-approve:hover { background-color: var(--success-soft); }

	/* ── Pencarian ── */
	.search-box {
		display: flex;
		flex: 1;
		align-items: center;
		gap: 8px;
		min-width: 220px;
		min-height: 36px;
		border: 1px solid var(--input);
		border-radius: var(--radius-control);
		padding: 0 12px;
		background-color: var(--card);
	}
	.search-box:focus-within {
		border-color: var(--ring);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--ring) 15%, transparent);
	}

	/* ── Ledger: satu baris satu akun, kolom angka rata kanan ── */
	.ledger {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
	}
	.ledger thead th {
		padding: 10px 16px;
		border-bottom: 1px solid var(--border);
		color: var(--muted-foreground);
		font-size: 11px;
		font-weight: 500;
		letter-spacing: 0.01em;
		text-align: left;
		white-space: nowrap;
	}
	/* Header angka ikut rata kanan supaya sejajar dengan isi kolomnya */
	.ledger thead th.col-num,
	.ledger thead th.col-money,
	.ledger thead th.col-act { text-align: right; }
	/* Kolom jatah berisi kontrol sejajar kiri — header juga, 16px = padding sel */
	.ledger thead th.col-quota { padding-left: 16px; }
	.ledger tbody td {
		padding: 12px 16px;
		vertical-align: middle;
	}
	.ledger tbody tr + tr td { border-top: 1px solid color-mix(in srgb, var(--foreground) 6%, transparent); }
	.ledger tbody tr:hover td { background-color: color-mix(in srgb, var(--secondary) 55%, transparent); }

	.col-user { position: relative; width: 40%; min-width: 240px; }
	.col-status { width: 96px; }
	.col-num { width: 64px; text-align: right !important; }
	.col-money { width: 148px; text-align: right !important; font-variant-numeric: tabular-nums; }
	.col-quota { width: 212px; }
	.col-date { width: 124px; white-space: nowrap; }
	.col-act { width: 96px; text-align: right !important; }

	/* Signature: spine status 3px di tepi kiri tiap baris */
	.row-spine {
		position: absolute;
		left: 0;
		top: 8px;
		bottom: 8px;
		width: 3px;
		border-radius: 0 2px 2px 0;
		background-color: transparent;
	}
	.ledger-row[data-status='active'] .row-spine { background-color: color-mix(in srgb, var(--success) 70%, transparent); }
	.ledger-row[data-status='pending'] .row-spine { background-color: var(--warning); }
	.ledger-row[data-status='suspended'] .row-spine { background-color: var(--destructive); }

	.mb-stack { display: block; min-width: 0; }
	.meta-line {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
		color: var(--muted-foreground);
		font-size: 12px;
		line-height: 18px;
	}
	/* Badge peran: biru untuk Klien, ungu-mitra untuk Mitra */
	.role-badge {
		flex: 0 0 auto;
		border-radius: 5px;
		padding: 1px 6px;
		font-size: 11px;
		font-weight: 500;
		line-height: 16px;
		white-space: nowrap;
	}
	.role-badge[data-role='client'] {
		background-color: var(--accent);
		color: var(--accent-foreground);
	}
	.role-badge[data-role='partner'] {
		background-color: var(--vvip-soft);
		color: var(--vvip);
	}
	.you-tag {
		border: 1px solid var(--border);
		border-radius: 4px;
		padding: 0 4px;
		color: var(--muted-foreground);
		font-size: 10px;
		line-height: 15px;
	}

	.status-text {
		display: inline-flex;
		align-items: center;
		border-radius: 5px;
		padding: 1px 7px 1px 6px;
		background-color: var(--success-soft);
		color: var(--success);
		font-size: 12px;
		font-weight: 500;
		white-space: nowrap;
	}
	.status-text[data-tone='warning'] {
		background-color: var(--warning-soft);
		color: var(--warning);
	}
	.status-text[data-tone='danger'] {
		background-color: var(--destructive-soft, #fdecee);
		color: var(--destructive);
	}
	.status-dot {
		display: inline-block;
		width: 6px;
		height: 6px;
		border-radius: 999px;
		margin-right: 6px;
		background-color: currentColor;
	}

	.quota-input {
		width: 64px;
		min-height: 30px;
		padding: 0 8px;
		font-size: 12px;
		text-align: right;
		font-variant-numeric: tabular-nums;
	}
	.quota-set {
		min-height: 30px;
		padding: 0 10px;
		font-size: 12px;
		white-space: nowrap;
	}
	.quota-mark {
		min-width: 28px;
		color: var(--muted-foreground);
		font-family: var(--font-mono);
		font-size: 11px;
		text-align: right;
	}
	.quota-mark[data-default='true'] { opacity: 0.55; }

	.row-menu-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border-radius: 7px;
		color: var(--muted-foreground);
		transition: background-color 160ms cubic-bezier(0.22, 1, 0.36, 1), color 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.row-menu-btn:hover { background-color: var(--secondary); color: var(--foreground); }
	.row-menu-btn:focus-visible { outline: 2px solid var(--ring); outline-offset: 1px; }

	.row-menu {
		min-width: 210px;
		padding: 4px;
		box-shadow: 0 8px 24px rgba(15, 23, 42, 0.08);
	}
	.row-menu-item {
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		min-height: 34px;
		border-radius: 6px;
		padding: 0 8px;
		font-size: 13px;
		text-align: left;
		color: var(--foreground);
		transition: background-color 120ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.row-menu-item:hover { background-color: var(--secondary); }
	.row-menu-item[data-danger='true'] { color: var(--destructive); }
	.row-menu-item :global(svg) { flex: 0 0 auto; color: var(--muted-foreground); }
	.row-menu-item[data-danger='true'] :global(svg) { color: var(--destructive); }

	/* ── Field di dialog ── */
	.field-label {
		display: block;
		margin-bottom: 4px;
		color: var(--muted-foreground);
		font-size: 12px;
		font-weight: 500;
	}
	.field-hint {
		display: block;
		margin-top: 4px;
		color: var(--muted-foreground);
		font-size: 12px;
		line-height: 18px;
	}

	/* ── Segmented control ── */
	.segment {
		display: inline-flex;
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 2px;
		background-color: var(--secondary);
	}
	.segment-btn {
		min-height: 28px;
		border-radius: 6px;
		padding: 0 12px;
		color: var(--muted-foreground);
		font-size: 12px;
		font-weight: 500;
		transition: background-color 160ms cubic-bezier(0.22, 1, 0.36, 1), color 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.segment-btn[data-active='true'] {
		background-color: var(--card);
		color: var(--foreground);
		box-shadow: 0 1px 2px rgba(15, 23, 42, 0.06);
	}

	@media (max-width: 1180px) {
		.col-date { display: none; }
	}
</style>
