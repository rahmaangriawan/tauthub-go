<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Kelola mitra — port app/admin/partners/page.tsx: direktori mitra dengan
	// filter praktis (status/kontak/media/mode otomatis), ekspor kontak CSV,
	// popup media & pengajuan harga, review dokumen Dewan Pers, pengaturan VIP
	// (VVIP & VIP Reguler), gerbang izin mode otomatis, dan hapus partner.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		ArrowUpRight, Check, Copy, Download, Ellipsis, FileText, Image as ImageIcon,
		MessageCircle, Newspaper, Search, ShieldCheck, Trash, TriangleAlert, UsersRound
	} from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDate, fmtNum } from '$lib/admin-format';
	import Tooltip from '$lib/components/tooltip.svelte';
	import { platformError } from '../_platform';
	import { swrPeek, swrPut } from '$lib/swr';

	type Partner = {
		id: string; user_id: string; email: string; name: string | null; status: string;
		media_count: number; document_status: 'pending' | 'verified' | 'revision' | null;
		document_count: number; created_at: string; is_vip?: boolean; vip_tier?: string;
		vip_contract_months?: number | null; vip_contract_days?: number | null;
		vip_expires_at?: string | null; email_alt1?: string | null; email_alt2?: string | null;
		wa_number?: string | null; wa_consent?: boolean; secure_direct_status?: string | null; price_min?: number | null;
		price_max?: number | null; auto_publish_enabled?: boolean;
	};
	type PartnerDocument = {
		id: string; partner_id: string; name: string; type: 'pdf' | 'image';
		size_bytes: number; url: string; status: 'pending' | 'verified' | 'revision';
		note: string | null; uploaded_at: string; reviewed_at: string | null;
	};
	type PartnerMedia = {
		id: string; name: string; slug: string | null; category: string | null;
		website_url: string | null; price: number | null; credit_cost: number | null;
		status: string; integration_type: string; media_tier: string | null;
		pricing_type: string | null; price_status: string | null;
		negotiated_price: number | null; dewan_pers_verified?: boolean;
	};

	const PARTNER_PAGE_SIZE = 20;
	const MEDIA_DIALOG_PAGE_SIZE = 10;

	const DOC_STATUS_LABEL: Record<string, string> = $derived({
		pending: tr('Menunggu'), verified: 'Terverifikasi', revision: 'Perlu Revisi'
	});
	const DOC_STATUS_TONE: Record<string, 'warning' | 'success' | 'danger'> = {
		pending: 'warning', verified: 'success', revision: 'danger'
	};
	const PRICE_STATUS_LABEL: Record<string, string> = $derived({
		menunggu_review: tr('Menunggu Review'), disetujui: tr('Disetujui'), negosiasi: 'Negosiasi',
		ditolak: tr('Ditolak'), final: 'Final'
	});
	const MEDIA_STATUS_LABEL: Record<string, string> = $derived({
		active: tr('Aktif'), inactive: tr('Nonaktif'), draft: 'Draf', pending_review: 'Pending',
		rejected: tr('Ditolak'), archived: tr('Arsip'), hidden: 'Disembunyikan'
	});
	const idr = (n: unknown) => `Rp ${Number(n ?? 0).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}`;

	let rows = $state<Partner[]>([]);
	let total = $state(0);
	let error = $state('');
	let loading = $state(true);
	let busy = $state(false);

	// Deep link notifikasi pengajuan Secure Direct: ?q=<email mitra>.
	const initialQuery = page.url.searchParams.get('q') ?? '';
	let search = $state(initialQuery);
	let debouncedSearch = $state(initialQuery);
	let filterStatus = $state('');
	let filterContact = $state('');
	let filterHasMedia = $state('');
	let filterAutoPublish = $state('');
	// Tab khusus pengajuan Secure Direct: filter status pengajuan terakhir mitra.
	let sdTab = $state<'all' | 'pending'>('all');
	let sdPendingTotal = $state(0);
	let pageNum = $state(1);
	let exporting = $state(false);
	let busyId = $state<string | null>(null);
	let waNotice = $state('');

	// Tambah partner
	let createOpen = $state(false);
	let form = $state({ email: '', name: '', status: 'active' });
	// Hapus partner
	let deleting = $state<Partner | null>(null);
	// Aksi baris
	let openMenu = $state<string | null>(null);
	let menuPos = $state({ top: 0, right: 0 });
	// Ubah peran (Klien ⇄ Mitra) — endpoint sama dengan /admin/users:
	// PUT /v1/platform/operations/users/{userId}/role.
	let roleTarget = $state<Partner | null>(null);
	let roleNext = $state<'partner' | 'client'>('client');
	let roleKeepBrands = $state(false);
	let roleBusy = $state(false);
	let roleError = $state('');
	let roleNotice = $state('');
	// Dokumen
	let docsPartner = $state<Partner | null>(null);
	let docs = $state<PartnerDocument[]>([]);
	let docsLoading = $state(false);
	let docsError = $state('');
	let reviewDoc = $state<PartnerDocument | null>(null);
	let reviewNote = $state('');
	let reviewError = $state('');
	// Media mitra
	let mediaPartner = $state<Partner | null>(null);
	let partnerMedia = $state<PartnerMedia[]>([]);
	let mediaReportedTotal = $state(0);
	let mediaLoading = $state(false);
	let mediaError = $state('');
	let mediaPage = $state(1);
	let copyFeedback = $state<{ key: string; ok: boolean } | null>(null);
	// Gerbang mode otomatis
	let autoPublishTarget = $state<Partner | null>(null);
	// Pengaturan VIP
	let vipTarget = $state<Partner | null>(null);
	let vipSettingsLoading = $state(false);
	let vipSettingsSaving = $state(false);
	let vipSettingsError = $state('');
	let vipSettings = $state({
		vip_tier: 'brand_ambassador' as 'brand_ambassador' | 'vip_reguler',
		commission_rate_bp: 500, cpa_rate: 15000, cpa_tier_threshold: 1000,
		cpa_tier1_rate: 10000, cpa_tier2_rate: 5000,
		contract_months: null as number | null, contract_days: null as number | null,
		entitlements: { client_workspace: true, ai_premium: true, seo_tools: true, keyword_research: true }
	});

	let debounceTimer: ReturnType<typeof setTimeout> | null = null;

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			void loadPendingTotal();
			// Kolom Mitra di /admin/media menautkan ke sini dengan ?q=<email|nama>.
			const q = new URLSearchParams(window.location.search).get('q')?.trim() ?? '';
			if (q) {
				search = q;
				debouncedSearch = q;
			}
			await load();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	function onSearchInput(value: string) {
		search = value;
		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			debouncedSearch = value.trim();
			pageNum = 1;
			void load();
		}, 300);
	}

	function applyFilters() {
		pageNum = 1;
		void load();
	}

	/** Jumlah mitra yang pengajuan Secure Direct-nya masih menunggu. */
	async function loadPendingTotal() {
		try {
			const res = await apiRequest('/v1/platform/partners?secure_direct=pending&limit=1');
			if (!res.ok) return;
			const data = await res.json();
			sdPendingTotal = Number(data?.total ?? 0);
		} catch {
			/* angka tab bersifat informasi — gagal memuat tidak menghalangi daftar */
		}
	}

	function setSdTab(next: 'all' | 'pending') {
		if (sdTab === next) return;
		sdTab = next;
		pageNum = 1;
		void load();
	}

	async function load() {
		error = '';
		const params = new URLSearchParams({
			limit: String(PARTNER_PAGE_SIZE),
			offset: String((pageNum - 1) * PARTNER_PAGE_SIZE)
		});
		if (debouncedSearch) params.set('search', debouncedSearch);
		if (filterStatus) params.set('status', filterStatus);
		if (filterContact) params.set('contact', filterContact);
		if (filterHasMedia) params.set('has_media', filterHasMedia);
		if (filterAutoPublish) params.set('auto_publish', filterAutoPublish);
		if (sdTab === 'pending') params.set('secure_direct', 'pending');
		const listUrl = `/v1/platform/partners?${params.toString()}`;
		// Data lama (<=10 menit) tampil seketika, refresh di latar.
		const cached = swrPeek<{ partners?: typeof rows; total?: number }>(listUrl);
		if (cached) {
			rows = cached.partners ?? [];
			total = cached.total ?? 0;
			loading = false;
		} else {
			loading = true;
		}
		const res = await apiRequest(listUrl);
		if (res.ok) {
			const data = await res.json();
			rows = data.partners ?? [];
			total = data.total ?? 0;
			swrPut(listUrl, data);
		} else {
			error = await platformError(res);
		}
		loading = false;
	}

	function resetFilters() {
		search = '';
		debouncedSearch = '';
		filterStatus = '';
		filterContact = '';
		filterHasMedia = '';
		filterAutoPublish = '';
		sdTab = 'all';
		applyFilters();
	}

	function changePage(nextOffset: number) {
		pageNum = Math.floor(nextOffset / PARTNER_PAGE_SIZE) + 1;
		void load();
	}

	async function exportCsv() {
		exporting = true;
		error = '';
		const params = new URLSearchParams({ format: 'csv' });
		if (debouncedSearch) params.set('search', debouncedSearch);
		const res = await apiRequest(`/v1/platform/partners?${params.toString()}`);
		if (!res.ok) {
			error = await platformError(res);
			exporting = false;
			return;
		}
		const blob = await res.blob();
		const a = document.createElement('a');
		a.href = URL.createObjectURL(blob);
		a.download = `kontak-mitra-${new Date().toISOString().slice(0, 10)}.csv`;
		a.click();
		URL.revokeObjectURL(a.href);
		exporting = false;
	}

	async function setAutoPublish(row: Partner, enabled: boolean) {
		busyId = row.id;
		error = '';
		const res = await apiRequest(`/v1/platform/partners/${row.id}`, {
			method: 'PUT',
			body: JSON.stringify({ auto_publish_enabled: enabled })
		});
		if (!res.ok) {
			error = await platformError(res);
			busyId = null;
			return;
		}
		rows = rows.map((p) => (p.id === row.id ? { ...p, auto_publish_enabled: enabled, secure_direct_status: enabled ? 'approved' : p.secure_direct_status } : p));
		if (enabled) void loadPendingTotal();
		autoPublishTarget = null;
		busyId = null;
	}

	async function create(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		const res = await apiRequest('/v1/platform/partners', {
			method: 'POST',
			body: JSON.stringify({ email: form.email.trim(), name: form.name.trim() || null, status: form.status })
		});
		if (!res.ok) {
			error = await platformError(res);
			busy = false;
			return;
		}
		createOpen = false;
		form = { email: '', name: '', status: 'active' };
		busy = false;
		await load();
	}

	/** Kirim pengingat isi nomor WA (lonceng + email, maksimal 1x per 30 hari). */
	async function sendWaReminder(partner: Partner) {
		error = '';
		waNotice = '';
		try {
			const res = await apiRequest(`/v1/platform/partners/${partner.id}/wa-reminder`, { method: 'POST' });
			const data = (await res.json().catch(() => ({}))) as { detail?: string; sent?: boolean };
			if (!res.ok) throw new Error(await platformError(res));
			waNotice = data.detail ?? (data.sent ? tr('Pengingat dikirim.') : tr('Tidak ada yang dikirim.'));
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal mengirim pengingat.');
		}
	}

	async function setStatus(partner: Partner, status: string) {
		error = '';
		const res = await apiRequest(`/v1/platform/partners/${partner.id}`, {
			method: 'PUT',
			body: JSON.stringify({ status })
		});
		if (!res.ok) {
			error = await platformError(res);
			return;
		}
		await load();
	}

	/** Buka dialog ubah peran. Halaman ini daftar mitra, jadi arah default: mitra → klien. */
	function openRoleDialog(row: Partner) {
		roleTarget = row;
		roleNext = 'client';
		roleKeepBrands = false;
		roleError = '';
		roleNotice = '';
	}

	async function submitRoleChange() {
		if (!roleTarget || roleBusy) return;
		// Disimpan sebelum await: TS tidak lagi menganggap roleTarget non-null
		// setelah beberapa await di dalam blok try.
		const targetId = roleTarget.id;
		roleBusy = true;
		roleError = '';
		roleNotice = '';
		try {
			const res = await apiRequest(`/v1/platform/operations/users/${roleTarget.user_id}/role`, {
				method: 'PUT',
				body: JSON.stringify({ role: roleNext, keep_brands: roleKeepBrands })
			});
			if (!res.ok) {
				roleError = await platformError(res);
				return;
			}
			const data = (await res.json().catch(() => ({}))) as { detail?: string };
			roleNotice = data.detail ?? (roleNext === 'partner' ? tr('Peran diubah menjadi Mitra.') : tr('Peran diubah menjadi Klien.'));
			// Mitra yang turun jadi klien tidak lagi bagian daftar mitra aktif:
			// buang barisnya sekarang, lalu fokuskan tampilan ke mitra aktif dan
			// sinkronkan ulang dengan server.
			if (roleNext === 'client') {
				rows = rows.filter((item) => item.id !== targetId);
				filterStatus = 'active';
				pageNum = 1;
			}
			await load();
		} catch (reason) {
			roleError = reason instanceof Error ? reason.message : tr('Gagal mengubah peran.');
		} finally {
			roleBusy = false;
		}
	}

	async function setVip(partner: Partner, isVip: boolean, tier?: 'brand_ambassador' | 'vip_reguler') {
		error = '';
		const body: Record<string, unknown> = { is_vip: isVip };
		if (isVip && tier) {
			body.vip_tier = tier;
			if (tier === 'vip_reguler') body.contract_days = 60;
		}
		const res = await apiRequest(`/v1/platform/partners/${partner.id}/vip`, {
			method: 'PUT',
			body: JSON.stringify(body)
		});
		if (!res.ok) {
			error = await platformError(res);
			return;
		}
		await load();
	}

	async function openVipSettings(partner: Partner) {
		vipTarget = partner;
		vipSettingsError = '';
		vipSettingsLoading = true;
		const res = await apiRequest(`/v1/platform/partners/${partner.id}/vip`);
		if (res.ok) {
			const data = await res.json();
			vipSettings = {
				vip_tier: data.vip.tier === 'vip_reguler' ? 'vip_reguler' : 'brand_ambassador',
				commission_rate_bp: data.vip.commissionRateBp,
				cpa_rate: data.vip.cpaRate,
				cpa_tier_threshold: data.vip.cpaTierThreshold,
				cpa_tier1_rate: data.vip.cpaTier1Rate,
				cpa_tier2_rate: data.vip.cpaTier2Rate,
				contract_months: data.vip.vipContractMonths ?? null,
				contract_days: data.vip.vipContractDays ?? (partner.vip_tier === 'vip_reguler' ? 60 : null),
				entitlements: data.vip.entitlements
			};
		}
		vipSettingsLoading = false;
	}

	async function saveVipSettings() {
		if (!vipTarget) return;
		vipSettingsSaving = true;
		vipSettingsError = '';
		const res = await apiRequest(`/v1/platform/partners/${vipTarget.id}/vip`, {
			method: 'PUT',
			body: JSON.stringify({ is_vip: true, ...vipSettings })
		});
		if (!res.ok) {
			vipSettingsError = await platformError(res);
			vipSettingsSaving = false;
			return;
		}
		vipTarget = null;
		vipSettingsSaving = false;
		await load();
	}

	async function confirmDelete() {
		if (!deleting) return;
		busy = true;
		error = '';
		const res = await apiRequest(`/v1/platform/partners/${deleting.id}`, { method: 'DELETE' });
		if (!res.ok) {
			error = await platformError(res);
			busy = false;
			return;
		}
		rows = rows.filter((r) => r.id !== deleting!.id);
		total = Math.max(0, total - 1);
		deleting = null;
		busy = false;
	}

	async function openMedia(partner: Partner) {
		mediaPartner = partner;
		partnerMedia = [];
		mediaReportedTotal = 0;
		mediaError = '';
		mediaPage = 1;
		mediaLoading = true;
		const res = await apiRequest(`/v1/platform/media?partner_id=${encodeURIComponent(partner.id)}&scope=all&limit=500`);
		if (res.ok) {
			const data = await res.json();
			partnerMedia = data.media ?? [];
			mediaReportedTotal = data.total ?? 0;
		} else {
			mediaError = await platformError(res);
		}
		mediaLoading = false;
	}

	/** Domain telanjang dari URL media — tanpa skema, www, dan garis miring akhir. */
	function domainOf(m: PartnerMedia) {
		return (m.website_url ?? '').replace(/^https?:\/\/(www\.)?/, '').replace(/\/$/, '');
	}

	function downloadDomains() {
		const cell = (v: string) => (/[",\n]/.test(v) ? `"${v.replace(/"/g, '""')}"` : v);
		const rows = partnerMedia.map((m) => `${cell(m.name)},${cell(domainOf(m))}`);
		const blob = new Blob([['Media,Domain', ...rows].join('\n')], { type: 'text/csv;charset=utf-8' });
		const a = document.createElement('a');
		a.href = URL.createObjectURL(blob);
		a.download = `domain-media-${(mediaPartner?.name ?? 'mitra').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')}-${new Date().toISOString().slice(0, 10)}.csv`;
		a.click();
		URL.revokeObjectURL(a.href);
	}

	async function copyContact(key: string, value: string) {
		try {
			await navigator.clipboard.writeText(value);
			copyFeedback = { key, ok: true };
		} catch {
			copyFeedback = { key, ok: false };
		}
		setTimeout(() => {
			if (copyFeedback?.key === key) copyFeedback = null;
		}, 2000);
	}

	async function openDocs(partner: Partner) {
		docsPartner = partner;
		docs = [];
		docsError = '';
		docsLoading = true;
		const res = await apiRequest(`/v1/platform/partners/${partner.id}/documents`);
		if (res.ok) {
			const data = await res.json();
			docs = data.documents ?? [];
		} else {
			docsError = await platformError(res);
		}
		docsLoading = false;
	}

	function openReview(doc: PartnerDocument) {
		reviewDoc = doc;
		reviewNote = doc.note ?? '';
		reviewError = '';
	}

	async function submitReview(status: PartnerDocument['status']) {
		if (!reviewDoc || !docsPartner) return;
		if (status === 'revision' && reviewNote.trim().length < 5) {
			reviewError = 'Alasan penolakan wajib diisi (minimal 5 karakter).';
			return;
		}
		busy = true;
		reviewError = '';
		docsError = '';
		const partnerId = docsPartner.id;
		const res = await apiRequest(`/v1/platform/partners/${partnerId}/documents/${reviewDoc.id}`, {
			method: 'PUT',
			body: JSON.stringify({ status, note: reviewNote.trim() || null })
		});
		if (!res.ok) {
			reviewError = await platformError(res);
			busy = false;
			return;
		}
		reviewDoc = null;
		reviewNote = '';
		busy = false;
		await openDocs(docsPartner!);
		await load();
	}

	const mediaPaged = $derived(
		partnerMedia.slice((mediaPage - 1) * MEDIA_DIALOG_PAGE_SIZE, mediaPage * MEDIA_DIALOG_PAGE_SIZE)
	);
	const denied = $derived(Boolean(!loading && rows.length === 0 && error.includes('Akses platform')));
	const hasFilter = $derived(Boolean(debouncedSearch || filterStatus || filterContact || filterHasMedia || filterAutoPublish));
</script>

<section>
	<PageHeader
		title="Partners"
		description={tr("Mitra media (1 akun user = 1 partner). Media milik partner aktif melewati gate persetujuan pada publikasi. Verifikasi Dewan Pers dinilai per domain/media, bukan per akun.")}
	>
		<span class="rounded-full px-3 py-1 text-xs font-medium" style="background-color: var(--accent); color: var(--accent-foreground);">{fmtNum(total)} partner</span>
		<button class="app-btn app-btn-primary" type="button" onclick={() => (createOpen = true)}><UsersRound size={16} />{tr("Tambah partner")}</button>
	</PageHeader>

	<!-- Tab khusus: mitra yang mengajukan Secure Direct dan masih menunggu ACC. -->
	<div class="mt-4 flex flex-wrap items-center gap-1 rounded-lg border p-1" style="border-color: var(--border); background: var(--card); width: fit-content;" role="tablist" aria-label={tr('Kelompok mitra')}>
		<button
			type="button"
			role="tab"
			aria-selected={sdTab === 'all'}
			class="inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-semibold transition-colors {sdTab === 'all' ? 'text-white' : 'text-muted-foreground hover:bg-secondary'}"
			style={sdTab === 'all' ? 'background-color: var(--primary);' : ''}
			onclick={() => setSdTab('all')}
		>
			{tr('Semua mitra')}
		</button>
		<button
			type="button"
			role="tab"
			aria-selected={sdTab === 'pending'}
			class="inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-semibold transition-colors {sdTab === 'pending' ? 'text-white' : 'text-muted-foreground hover:bg-secondary'}"
			style={sdTab === 'pending' ? 'background-color: var(--primary);' : ''}
			onclick={() => setSdTab('pending')}
		>
			<ShieldCheck size={13} />
			{tr('Pengajuan Secure Direct')}
			{#if sdPendingTotal > 0}
				<span class="rounded-full px-1.5 text-[10.5px] font-bold {sdTab === 'pending' ? 'bg-white/20 text-white' : 'bg-secondary text-muted-foreground'}">{sdPendingTotal}</span>
			{/if}
		</button>
	</div>

	<div class="mt-3 flex flex-wrap items-center gap-2">
		<label class="app-input flex min-w-[220px] flex-1 items-center gap-2 !py-1.5">
			<Search size={15} class="text-muted-foreground" />
			<input class="w-full bg-transparent text-sm outline-none" placeholder={tr("Cari nama, email, atau nomor WA…")} aria-label={tr("Cari partner")} value={search} oninput={(e) => onSearchInput((e.currentTarget as HTMLInputElement).value)} />
		</label>
		<Select
			ariaLabel="Filter status"
			minWidth={160}
			value={filterStatus}
			onchange={(v) => {
				filterStatus = v;
				applyFilters();
			}}
			options={[
				{ value: '', label: tr('Semua status') },
				{ value: 'active', label: tr('Aktif') },
				{ value: 'inactive', label: tr('Nonaktif') }
			]}
		/>
		<Select
			ariaLabel="Filter kontak"
			minWidth={190}
			value={filterContact}
			onchange={(v) => {
				filterContact = v;
				applyFilters();
			}}
			options={[
				{ value: '', label: tr('Semua kontak') },
				{ value: 'wa', label: 'Punya WA' },
				{ value: 'alt', label: 'Punya email alternatif' },
				{ value: 'incomplete', label: 'Kontak belum lengkap' }
			]}
		/>
		<Select
			ariaLabel="Filter media"
			minWidth={170}
			value={filterHasMedia}
			onchange={(v) => {
				filterHasMedia = v;
				applyFilters();
			}}
			options={[
				{ value: '', label: tr('Semua media') },
				{ value: '1', label: tr('Sudah ada media') },
				{ value: '0', label: tr('Belum ada media') }
			]}
		/>
		<Select
			ariaLabel="Filter mode otomatis"
			minWidth={190}
			value={filterAutoPublish}
			onchange={(v) => {
				filterAutoPublish = v;
				applyFilters();
			}}
			options={[
				{ value: '', label: tr('Semua mode otomatis') },
				{ value: '1', label: 'Diizinkan otomatis' },
				{ value: '0', label: tr('Belum diizinkan') }
			]}
		/>
		{#if hasFilter}
			<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={resetFilters}>Reset filter</button>
		{/if}
		<span class="flex-1"></span>
		<button class="app-btn app-btn-soft" type="button" disabled={exporting} onclick={() => void exportCsv()}>
			{exporting ? 'Mengekspor…' : 'Ekspor kontak (CSV)'}
		</button>
	</div>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke pengelolaan partner.")}</p>
		</div>
	{:else}
		<Feedback {loading} {error} notice={waNotice} loadingText="Memuat partner…" />

		<div class="app-card mt-4 overflow-x-auto">
			<table class="w-full text-sm" style="min-width: 1280px;">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-4 py-2.5 font-medium">Partner</th>
						<th class="px-4 py-2.5 font-medium">{tr("Email")}</th>
						<th class="px-4 py-2.5 font-medium">WA</th>
						<th class="px-4 py-2.5 font-medium">{tr("Email Alternatif")}</th>
						<th class="px-4 py-2.5 font-medium">{tr("Harga Media")}</th>
						<th class="px-4 py-2.5 font-medium min-w-[220px]">{tr("Media")}</th>
						<th class="px-4 py-2.5 font-medium">Dewan Pers</th>
						<th class="px-4 py-2.5 font-medium">VIP</th>
						<th class="px-4 py-2.5 font-medium">Status</th>
						<th class="px-4 py-2.5 font-medium">Aksi</th>
					</tr>
				</thead>
				<tbody>
					{#each rows as row (row.id)}
						<tr class="border-t">
							<td class="px-4 py-3">
								<div class="flex items-center gap-2">
									<UsersRound size={16} class="shrink-0 text-muted-foreground" />
									<div>
										{#if row.secure_direct_status === 'pending'}
											<Tooltip content={tr('Mengajukan Secure Direct — gunakan menu titik-tiga untuk mengizinkan mode otomatis.')}>
												<Badge label={tr('Pengajuan Secure Direct')} tone="warning" />
											</Tooltip>
										{/if}
										<p class="font-medium">{row.name ?? row.email}</p>
										<p class="mt-0.5 flex items-center gap-1 text-xs text-muted-foreground" title={tr('ID mitra ini bisa dicari di katalog media.')}>
											<span>{tr('ID Mitra')}:</span>
											<code class="font-mono">{row.id.slice(0, 8)}</code>
											<button
												class="rounded p-0.5 hover:bg-secondary"
												type="button"
												title={tr('Salin ID mitra')}
												aria-label={tr('Salin ID mitra')}
												onclick={(e) => { e.stopPropagation(); void copyContact(`pid-${row.id}`, row.id.slice(0, 8)); }}
											>
												{#if copyFeedback?.key === `pid-${row.id}` && copyFeedback?.ok}<Check size={12} style="color: var(--success);" />{:else}<Copy size={12} />{/if}
											</button>
										</p>
									</div>
								</div>
							</td>
							<td class="whitespace-nowrap px-4 py-3">
								<a style="color: var(--primary);" href={`mailto:${row.email}`}>{row.email}</a>
								<div class="mt-0.5">
									<Tooltip content={tr('Tanggal mitra bergabung ke platform Tauthub.')}>
										<span class="text-xs text-muted-foreground">{tr('Bergabung')} {fmtDate(row.created_at)}</span>
									</Tooltip>
								</div>
							</td>
							<td class="px-4 py-3">
								{#if row.wa_number}
									<div class="flex flex-col gap-1">
										<a style="color: var(--primary);" href={`https://wa.me/${row.wa_number.replace(/[^0-9]/g, '')}`} target="_blank" rel="noreferrer">{row.wa_number}</a>
										{#if !row.wa_consent}<Badge label="tanpa izin WA" tone="neutral" />{/if}
										</div>
							{:else}
										<span class="text-xs text-muted-foreground">—</span>
										{#if (row.media_count ?? 0) > 2}
											<Badge label={tr('WA belum diisi')} tone="warning" />
										{/if}
							{/if}
							</td>
							<td class="px-4 py-3">
								{#if row.email_alt1 || row.email_alt2}
									<div class="flex flex-col gap-1 text-xs">
										{#if row.email_alt1}<span>{row.email_alt1}</span>{/if}
										{#if row.email_alt2}<span>{row.email_alt2}</span>{/if}
									</div>
								{:else}
									<span class="text-xs text-muted-foreground">—</span>
								{/if}
							</td>
							<td class="px-4 py-3">
								{#if row.price_min != null || row.price_max != null}
									<span class="whitespace-nowrap text-xs">
										{row.price_min === row.price_max
											? idr(row.price_min ?? 0)
											: `${idr(row.price_min ?? 0)} – ${idr(row.price_max ?? 0)}`}
									</span>
								{:else}
									<span class="text-xs text-muted-foreground">—</span>
								{/if}
							</td>
							<td class="px-4 py-3">
								{#if row.media_count > 0}
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" title={tr("Lihat media & pengajuan harganya")} onclick={() => void openMedia(row)}>
										{row.media_count} media
									</button>
								{:else}
									<span class="text-xs text-muted-foreground">—</span>
								{/if}
							</td>
							<td class="px-4 py-3">
								{#if row.document_count === 0}
									<Badge label={tr("Belum upload")} tone="neutral" />
								{:else}
									<Badge
										label={`${row.document_status ? DOC_STATUS_LABEL[row.document_status] : '—'} (${row.document_count})`}
										tone={DOC_STATUS_TONE[row.document_status ?? 'pending'] ?? 'neutral'}
									/>
								{/if}
							</td>
							<td class="px-4 py-3">
								{#if row.is_vip}
									<div class="flex flex-col items-start gap-1">
										<Badge label={row.vip_tier === 'vip_reguler' ? 'VIP Reguler' : 'VVIP'} tone="accent" />
										{#if row.vip_contract_months != null}<span class="text-xs text-muted-foreground">Kontrak {row.vip_contract_months} bln</span>{/if}
										{#if row.vip_expires_at}<span class="text-xs text-muted-foreground">Sampai {fmtDate(row.vip_expires_at)}</span>{/if}
									</div>
								{:else}
									<Badge label="-" tone="neutral" />
								{/if}
							</td>
							<td class="px-4 py-3"><Badge label={row.status} tone={row.status === 'active' ? 'success' : 'neutral'} /></td>
							<td class="px-4 py-3">
								<button
									class="rounded-lg p-1.5 hover:bg-secondary"
									type="button"
									aria-label={`Aksi untuk ${row.name ?? row.email}`}
									onclick={(e) => {
										const rect = (e.currentTarget as HTMLElement).getBoundingClientRect();
										menuPos = { top: rect.bottom + 4, right: window.innerWidth - rect.right };
										openMenu = openMenu === row.id ? null : row.id;
									}}
								><Ellipsis size={19} /></button>
								{#if openMenu === row.id}
									<div class="fixed inset-0 z-40" role="presentation" onclick={() => (openMenu = null)}></div>
									<div
										class="app-card fixed z-50 min-w-[230px] p-1.5 text-sm shadow-md"
										style="top: {menuPos.top}px; right: {menuPos.right}px;"
									>
											<button class="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left hover:bg-secondary" type="button" onclick={() => { openMenu = null; void openDocs(row); }}>
												<ShieldCheck size={14} /> Dokumen
											</button>
											{#if (row.media_count ?? 0) > 2 && (!row.wa_number || !row.wa_consent)}
												<button class="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left hover:bg-secondary" type="button" onclick={() => { openMenu = null; void sendWaReminder(row); }}>
													<MessageCircle size={14} /> {tr('Kirim pengingat isi WA')}
												</button>
											{/if}
											<button class="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left hover:bg-secondary" type="button" onclick={() => { openMenu = null; openRoleDialog(row); }}>
												<UsersRound size={14} /> {tr('Ubah peran (Klien/Mitra)')}
											</button>
											<button class="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left hover:bg-secondary" type="button" onclick={() => { openMenu = null; void openVipSettings(row); }}>
												{row.is_vip ? tr('Pengaturan VIP') : 'Jadikan VIP'}
											</button>
											{#if !row.is_vip}
												<button class="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left hover:bg-secondary" type="button" onclick={() => { openMenu = null; void setVip(row, true, 'vip_reguler'); }}>
													Jadikan VIP Reguler (60 hari)
												</button>
											{/if}
											<button class="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left hover:bg-secondary" type="button" onclick={() => { openMenu = null; autoPublishTarget = row; }}>
												{row.auto_publish_enabled ? 'Cabut izin mode otomatis' : 'Izinkan mode otomatis'}
											</button>
											{#if row.status === 'active'}
												<button class="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left hover:bg-secondary" type="button" onclick={() => { openMenu = null; void setStatus(row, 'inactive'); }}>Nonaktifkan</button>
											{:else}
												<button class="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left hover:bg-secondary" type="button" onclick={() => { openMenu = null; void setStatus(row, 'active'); }}>Aktifkan</button>
											{/if}
											<div class="my-1 border-t"></div>
											<button
												class="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left hover:bg-secondary"
												style="color: var(--destructive);"
												type="button"
												onclick={() => { openMenu = null; deleting = row; }}
											><Trash size={14} />{tr("Hapus")}</button>
										</div>
									{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if !loading && rows.length === 0}
				<div class="flex flex-col items-center px-4 py-12 text-center">
					<UsersRound size={26} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">{tr("Belum ada partner. Tambahkan partner pertama.")}</p>
				</div>
			{/if}
			<div class="px-4 pb-4">
				<Pagination
					total={total}
					limit={PARTNER_PAGE_SIZE}
					offset={(pageNum - 1) * PARTNER_PAGE_SIZE}
					onChange={changePage}
				/>
			</div>
		</div>
	{/if}
</section>

{#if createOpen}
	<Modal title={tr("Tambah partner")} label={tr("Tambah partner")} maxWidth="lg" onClose={() => (createOpen = false)}>
		<p class="text-sm text-muted-foreground">{tr("User dengan email ini akan menjadi mitra media Tauthub.")}</p>
		<form class="mt-4 flex flex-col gap-3" onsubmit={create}>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Email user")}</span>
				<input class="app-input w-full" type="email" required placeholder={tr("mitra@media.com")} bind:value={form.email} />
			</label>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Nama (opsional)")}</span>
				<input class="app-input w-full" placeholder={tr("Nama media partner")} bind:value={form.name} />
			</label>
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">Status</span>
				<Select
					ariaLabel="Status"
					minWidth={200}
					value={form.status}
					onchange={(v) => (form.status = v)}
					options={[
						{ value: 'active', label: 'active' },
						{ value: 'inactive', label: 'inactive' }
					]}
				/>
			</label>
			<div class="mt-1 flex justify-end gap-2 border-t pt-4">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (createOpen = false)}>{tr("Batal")}</button>
				<button class="app-btn app-btn-primary" type="submit" disabled={busy}>{busy ? 'Menyimpan…' : tr('Tambah partner')}</button>
			</div>
		</form>
	</Modal>
{/if}

{#if mediaPartner}
	<Modal title={`Media mitra — ${mediaPartner.name ?? mediaPartner.email}`} label={tr("Media mitra")} maxWidth="4xl" onClose={() => (mediaPartner = null)}>
		<p class="text-sm text-muted-foreground">{tr("Media yang terdeteksi milik mitra ini beserta harga yang mereka ajukan.")}</p>

		{#if mediaPartner.email || mediaPartner.wa_number || mediaPartner.email_alt1 || mediaPartner.email_alt2}
			<div class="mt-3 flex flex-wrap gap-5 rounded-lg border p-3">
				{#if mediaPartner.email}
					<div class="flex flex-col gap-1">
						<span class="text-xs font-semibold text-muted-foreground">{tr("Email")}</span>
						<div class="flex items-center gap-1">
							<a class="text-xs" style="color: var(--primary);" href={`mailto:${mediaPartner.email}`}>{mediaPartner.email}</a>
							<button class="rounded p-1 hover:bg-secondary" type="button" aria-label={`Salin email ${mediaPartner.email}`} onclick={() => void copyContact('email', mediaPartner?.email ?? '')}>
								{#if copyFeedback?.key === 'email' && copyFeedback?.ok}<Check size={14} style="color: var(--success);" />{:else}<Copy size={14} />{/if}
							</button>
						</div>
					</div>
				{/if}
				{#if mediaPartner.wa_number}
					<div class="flex flex-col gap-1">
						<span class="text-xs font-semibold text-muted-foreground">WhatsApp</span>
						<div class="flex items-center gap-2">
							<span class="text-xs">{mediaPartner.wa_number}</span>
							<a
								class="inline-flex h-[22px] w-[22px] items-center justify-center rounded"
								style="color: var(--primary);"
								href={`https://wa.me/${mediaPartner.wa_number.replace(/[^0-9]/g, '')}`}
								target="_blank"
								rel="noreferrer"
								aria-label={`Buka chat WhatsApp ${mediaPartner.wa_number}`}
								title={tr("Buka chat WhatsApp")}
							><ArrowUpRight size={14} /></a>
							<button class="rounded p-1 hover:bg-secondary" type="button" aria-label={`Salin WhatsApp ${mediaPartner.wa_number}`} title="Salin nomor" onclick={() => void copyContact('wa', mediaPartner?.wa_number ?? '')}>
								{#if copyFeedback?.key === 'wa' && copyFeedback?.ok}<Check size={14} style="color: var(--success);" />{:else}<Copy size={14} />{/if}
							</button>
						</div>
					</div>
				{/if}
				{#if mediaPartner.email_alt1 || mediaPartner.email_alt2}
					<div class="flex flex-col gap-1">
						<span class="text-xs font-semibold text-muted-foreground">{tr("Email alternatif")}</span>
						<div class="flex flex-col gap-1">
							{#each [['alt1', mediaPartner.email_alt1], ['alt2', mediaPartner.email_alt2]] as [key, value] (key)}
								{#if value}
									<div class="flex items-center gap-1">
										<a class="text-xs" style="color: var(--primary);" href={`mailto:${value}`}>{value}</a>
										<button class="rounded p-1 hover:bg-secondary" type="button" aria-label={`Salin email alternatif ${value}`} onclick={() => void copyContact(String(key), String(value ?? ''))}>
											{#if copyFeedback?.key === key && copyFeedback?.ok}<Check size={14} style="color: var(--success);" />{:else}<Copy size={14} />{/if}
										</button>
									</div>
								{/if}
							{/each}
						</div>
					</div>
				{/if}
				{#if copyFeedback && !copyFeedback?.ok}
					<span class="self-center text-xs" style="color: var(--warning);">{tr("Gagal menyalin — salin manual dari teks di atas.")}</span>
				{/if}
				{#if mediaPartner.wa_number && !mediaPartner.wa_consent}
					<span class="self-center text-xs text-muted-foreground">{tr("Mitra belum menyetujui kontak via WhatsApp.")}</span>
				{/if}
			</div>
		{/if}

		{#if mediaLoading}
			<p class="mt-4 text-sm text-muted-foreground">{tr("Memuat daftar media mitra…")}</p>
		{:else if mediaError}
			<div class="mt-4 flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{mediaError}</div>
		{:else if partnerMedia.length === 0}
			<div class="mt-4 flex flex-col items-center rounded-lg border border-dashed p-6 text-center">
				<Newspaper size={26} class="text-muted-foreground" />
				<p class="mt-2 text-sm text-muted-foreground">{tr("Mitra ini belum punya media terdaftar. Media muncul di sini setelah mitra melisting lewat portal mitra.")}</p>
			</div>
		{:else}
			<div class="mt-3 max-h-[420px] overflow-auto">
				<table class="w-full text-sm" style="min-width: 760px;">
					<thead>
						<tr class="text-left text-xs text-muted-foreground">
							<th class="py-2 pr-3 font-medium">{tr("Media")}</th>
							<th class="py-2 pr-3 font-medium">Kategori</th>
							<th class="py-2 pr-3 font-medium">{tr("Harga Diajukan")}</th>
							<th class="py-2 pr-3 font-medium">{tr("Status Harga")}</th>
							<th class="py-2 font-medium">{tr("Status Media")}</th>
						</tr>
					</thead>
					<tbody>
						{#each mediaPaged as m (m.id)}
							<tr class="border-t">
								<td class="py-2 pr-3">
									<p class="font-medium">{m.name}</p>
									{#if m.website_url}
										<div class="flex items-center gap-1">
											<p class="text-xs text-muted-foreground">{domainOf(m)}</p>
											<button
												class="rounded p-0.5 hover:bg-secondary"
												type="button"
												title={tr("Salin domain")}
												aria-label={`Salin domain ${domainOf(m)}`}
												onclick={() => void copyContact(`dom-${m.id}`, domainOf(m))}
											>
												{#if copyFeedback?.key === `dom-${m.id}` && copyFeedback?.ok}<Check size={12} style="color: var(--success);" />{:else}<Copy size={12} />{/if}
											</button>
										</div>
									{/if}
								</td>
								<td class="py-2 pr-3">{m.category ?? '—'}</td>
								<td class="py-2 pr-3">
									{m.price != null ? idr(m.price) : '—'}
									{#if m.pricing_type === 'nego'}<span class="text-xs text-muted-foreground"> · nego</span>{/if}
								</td>
								<td class="py-2 pr-3">
									<Badge label={PRICE_STATUS_LABEL[m.price_status ?? ''] ?? m.price_status ?? '—'} tone="neutral" />
								</td>
								<td class="py-2">
									<Badge label={MEDIA_STATUS_LABEL[m.status] ?? m.status} tone="neutral" />
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
			<Pagination
				total={partnerMedia.length}
				limit={MEDIA_DIALOG_PAGE_SIZE}
				offset={(mediaPage - 1) * MEDIA_DIALOG_PAGE_SIZE}
				onChange={(next) => (mediaPage = Math.floor(next / MEDIA_DIALOG_PAGE_SIZE) + 1)}
			/>
			{#if mediaReportedTotal > partnerMedia.length}
				<p class="mt-2 text-xs" style="color: var(--warning);">
					Server melaporkan {mediaReportedTotal} media, {partnerMedia.length} {tr("termuat. Sisanya bisa dilihat di halaman Media.")} </p>
			{/if}
		{/if}

		{#snippet footer()}
			<button
				class="app-btn app-btn-soft"
				type="button"
				disabled={mediaLoading || partnerMedia.length === 0}
				title={tr("Unduh semua domain media sebagai CSV")}
				onclick={downloadDomains}
			><Download size={14} />{tr("Unduh domain (CSV)")}</button>
			<button class="app-btn app-btn-soft" type="button" onclick={() => (mediaPartner = null)}>{tr("Tutup")}</button>
		{/snippet}
	</Modal>
{/if}

{#if docsPartner}
	<Modal title={`Dokumen Dewan Pers — ${docsPartner.name ?? tr('Mitra')}`} label="Dokumen mitra" maxWidth="2xl" onClose={() => { docsPartner = null; reviewDoc = null; }}>
		<p class="text-sm text-muted-foreground">{tr("Dokumen verifikasi yang diunggah mitra. Tandai terverifikasi agar mitra dapat menambahkan media.")}</p>
		<div class="mt-4">
			{#if docsError}
				<div class="mb-3 flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{docsError}</div>
			{/if}
			{#if docsLoading}
				<p class="text-sm text-muted-foreground">{tr("Memuat dokumen…")}</p>
			{:else if docs.length === 0}
				<div class="flex flex-col items-center rounded-lg border border-dashed p-6 text-center">
					<ShieldCheck size={26} class="text-muted-foreground" />
					<p class="mt-2 text-sm text-muted-foreground">{tr("Mitra ini belum mengunggah dokumen.")}</p>
				</div>
			{:else}
				<div class="flex flex-col gap-2">
					{#each docs as doc (doc.id)}
						<div class="flex items-center justify-between gap-3 rounded-lg border p-3">
							<div class="flex min-w-0 items-center gap-2">
								{#if doc.type === 'pdf'}<FileText size={20} style="color: var(--primary);" />{:else}<ImageIcon size={20} style="color: var(--success);" />{/if}
								<div class="min-w-0">
									<p class="truncate text-sm font-medium">{doc.name}</p>
									<p class="text-xs text-muted-foreground">
										{doc.size_bytes > 0 ? `${(doc.size_bytes / (1024 * 1024)).toFixed(1)} MB · ` : ''}Diunggah {fmtDate(doc.uploaded_at)}
									</p>
									{#if doc.note}<p class="text-xs" style="color: var(--destructive);">{doc.note}</p>{/if}
								</div>
							</div>
							<div class="flex shrink-0 items-center gap-2">
								<Badge label={DOC_STATUS_LABEL[doc.status]} tone={DOC_STATUS_TONE[doc.status]} />
								<a class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" href={doc.url} target="_blank" rel="noreferrer">{tr("Lihat")}</a>
								<button class="app-btn !min-h-7 !px-2.5 !text-xs" style="background-color: var(--accent); color: var(--accent-foreground);" type="button" onclick={() => openReview(doc)}>Review</button>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	</Modal>
{/if}

{#if reviewDoc}
	<Modal title="Review Dokumen" label="Review dokumen" maxWidth="md" onClose={() => { reviewDoc = null; reviewError = ''; }}>
		<p class="text-sm text-muted-foreground">{reviewDoc.name}</p>
		<div class="mt-4 flex flex-col gap-3">
			{#if reviewError}
				<div class="flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{reviewError}</div>
			{/if}
			<label class="block">
				<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Alasan (wajib saat menolak, opsional saat menyetujui)")}</span>
				<textarea
					class="app-input w-full"
					rows={3}
					placeholder={tr("Contoh: sertifikat tidak terbaca, mohon unggah ulang dengan resolusi lebih baik.")}
					bind:value={reviewNote}
					oninput={() => (reviewError = '')}
				></textarea>
				<span class="mt-1 block text-xs text-muted-foreground">{tr("Alasan ini akan ditampilkan ke mitra di halaman Profil.")}</span>
			</label>
		</div>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => { reviewDoc = null; reviewError = ''; }}>{tr("Batal")}</button>
			<button class="app-btn app-btn-soft" style="color: var(--destructive);" type="button" disabled={busy} onclick={() => void submitReview('revision')}>
				{busy ? 'Menyimpan…' : 'Tolak'}
			</button>
			<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void submitReview('verified')}>
				{busy ? 'Menyimpan…' : 'Setujui'}
			</button>
		{/snippet}
	</Modal>
{/if}

{#if vipTarget}
	<Modal title={vipTarget.is_vip ? tr('Pengaturan VIP') : 'Jadikan VIP'} label={tr("Pengaturan VIP")} maxWidth="lg" onClose={() => { vipTarget = null; vipSettingsError = ''; }}>
		<p class="text-sm text-muted-foreground">{tr("Konfigurasi tier VIP untuk")} {vipTarget.name ?? vipTarget.email}.</p>
		<div class="mt-4 flex flex-col gap-3">
			{#if vipSettingsLoading}
				<p class="text-sm text-muted-foreground">{tr("Memuat pengaturan…")}</p>
			{:else}
				<div class="flex items-center justify-between gap-3">
					<div>
						<p class="text-sm font-medium">{tr("Status VIP")}</p>
						<p class="text-xs text-muted-foreground">Aktifkan/nonaktifkan status VIP mitra ini</p>
					</div>
					<input
						type="checkbox"
						checked={vipTarget.is_vip ?? false}
						onchange={(e) => {
							const checked = (e.currentTarget as HTMLInputElement).checked;
							if (!checked) {
								const target = vipTarget!;
								vipTarget = null;
								void setVip(target, false);
							} else {
								vipTarget = { ...vipTarget!, is_vip: true };
							}
						}}
					/>
				</div>
				<div class="border-t"></div>
				<p class="text-sm font-medium">Tipe VIP</p>
				<div class="flex gap-2">
					<button
						class="app-btn !min-h-8 !px-3 !text-xs"
						class:app-btn-primary={vipSettings.vip_tier === 'brand_ambassador'}
						class:app-btn-soft={vipSettings.vip_tier !== 'brand_ambassador'}
						type="button"
						onclick={() => (vipSettings = { ...vipSettings, vip_tier: 'brand_ambassador' })}
					>VVIP</button>
					<button
						class="app-btn !min-h-8 !px-3 !text-xs"
						class:app-btn-primary={vipSettings.vip_tier === 'vip_reguler'}
						class:app-btn-soft={vipSettings.vip_tier !== 'vip_reguler'}
						type="button"
						onclick={() => (vipSettings = { ...vipSettings, vip_tier: 'vip_reguler' })}
					>VIP Reguler</button>
				</div>
				<p class="text-xs text-muted-foreground">
					{vipSettings.vip_tier === 'vip_reguler'
						? tr('Fee affiliate Rp10.000/domain untuk 1000 domain pertama, lalu Rp5.000/domain. Komisi transaksi tetap 1% dan tidak dibatasi kontrak. Tanpa fasilitas Dashboard Klien/AI Premium/SEO Tools.')
						: 'Komisi transaksi 5% + CPA Rp15.000/domain + fasilitas Dashboard Klien, AI Premium, SEO Tools.'}
				</p>
				<div class="border-t"></div>

				{#if vipSettings.vip_tier === 'brand_ambassador'}
					<p class="text-sm font-medium">Komisi</p>
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">Transaction Commission Rate (basis point)</span>
						<input class="app-input w-full" type="number" min="0" max="10000" value={vipSettings.commission_rate_bp} oninput={(e) => (vipSettings = { ...vipSettings, commission_rate_bp: Number((e.currentTarget as HTMLInputElement).value) })} />
						<span class="mt-1 block text-xs text-muted-foreground">{(vipSettings.commission_rate_bp / 100).toFixed(1)}{tr("% dari nilai transaksi. Default VIP: 500 (5%).")}</span>
					</label>
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">CPA Rate (Rp)</span>
						<input class="app-input w-full" type="number" min="0" value={vipSettings.cpa_rate} oninput={(e) => (vipSettings = { ...vipSettings, cpa_rate: Number((e.currentTarget as HTMLInputElement).value) })} />
						<span class="mt-1 block text-xs text-muted-foreground">CPA per domain approved. Default VIP: 15000.</span>
					</label>
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">Masa Kontrak (bulan)</span>
						<input
							class="app-input w-full"
							type="number"
							min="0"
							max="120"
							value={vipSettings.contract_months == null ? '' : String(vipSettings.contract_months)}
							placeholder="mis. 12"
							oninput={(e) => {
								const v = (e.currentTarget as HTMLInputElement).value;
								vipSettings = { ...vipSettings, contract_months: v === '' ? null : Number(v) };
							}}
						/>
						<span class="mt-1 block text-xs text-muted-foreground">{tr("Kosongkan (0) untuk tanpa batas. Saat masa kontrak habis, akun mitra otomatis kembali ke status biasa & hak VIP nonaktif.")}</span>
					</label>
					<div class="border-t"></div>
					<p class="text-sm font-medium">Entitlements</p>
					{#each [
						['client_workspace', tr('Dashboard Klien')],
						['ai_premium', 'AI Premium'],
						['seo_tools', 'SEO Tools'],
						['keyword_research', 'Keyword Research']
					] as [key, label] (key)}
						<div class="flex items-center justify-between gap-3">
							<span class="text-sm">{label}</span>
							<input
								type="checkbox"
								checked={vipSettings.entitlements[key as keyof typeof vipSettings.entitlements]}
								onchange={(e) => {
									const checked = (e.currentTarget as HTMLInputElement).checked;
									vipSettings = {
										...vipSettings,
										entitlements: { ...vipSettings.entitlements, [key]: checked }
									};
								}}
							/>
						</div>
					{/each}
				{:else}
					<p class="text-sm font-medium">Fee per Domain (bertingkat)</p>
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">Batas kuota domain</span>
						<input class="app-input w-full" type="number" min="0" value={vipSettings.cpa_tier_threshold} oninput={(e) => (vipSettings = { ...vipSettings, cpa_tier_threshold: Number((e.currentTarget as HTMLInputElement).value) })} />
						<span class="mt-1 block text-xs text-muted-foreground">{tr("Jumlah domain yang mendapat fee tier-1 sejak aktivasi.")}</span>
					</label>
					<div class="grid gap-3 sm:grid-cols-2">
						<label class="block">
							<span class="mb-1 block text-xs font-semibold text-muted-foreground">Fee domain ke-1..N (Rp)</span>
							<input class="app-input w-full" type="number" min="0" value={vipSettings.cpa_tier1_rate} oninput={(e) => (vipSettings = { ...vipSettings, cpa_tier1_rate: Number((e.currentTarget as HTMLInputElement).value) })} />
						</label>
						<label class="block">
							<span class="mb-1 block text-xs font-semibold text-muted-foreground">{tr("Fee setelah kuota (Rp)")}</span>
							<input class="app-input w-full" type="number" min="0" value={vipSettings.cpa_tier2_rate} oninput={(e) => (vipSettings = { ...vipSettings, cpa_tier2_rate: Number((e.currentTarget as HTMLInputElement).value) })} />
						</label>
					</div>
					<label class="block">
						<span class="mb-1 block text-xs font-semibold text-muted-foreground">Masa Kontrak (hari)</span>
						<input
							class="app-input w-full"
							type="number"
							min="0"
							max="3650"
							value={vipSettings.contract_days == null ? '' : String(vipSettings.contract_days)}
							placeholder="mis. 60"
							oninput={(e) => {
								const v = (e.currentTarget as HTMLInputElement).value;
								vipSettings = { ...vipSettings, contract_days: v === '' ? null : Number(v) };
							}}
						/>
						<span class="mt-1 block text-xs text-muted-foreground">{tr("Default 60 hari. Setelah kontrak berakhir, hak VIP Reguler (fee per-domain) nonaktif. Domain yang masuk sebelum berakhir tetap dibayar.")}</span>
					</label>
				{/if}

				{#if vipSettingsError}
					<div class="flex items-start gap-2 text-xs" style="color: var(--destructive);">{vipSettingsError}</div>
				{/if}
				<div class="flex justify-end gap-2 border-t pt-4">
					<button class="app-btn app-btn-soft" type="button" onclick={() => { vipTarget = null; vipSettingsError = ''; }}>{tr("Batal")}</button>
					<button class="app-btn app-btn-primary" type="button" disabled={vipSettingsSaving} onclick={() => void saveVipSettings()}>
						{vipSettingsSaving ? 'Menyimpan…' : vipTarget.is_vip ? tr('Simpan') : 'Aktifkan VIP'}
					</button>
				</div>
			{/if}
		</div>
	</Modal>
{/if}

{#if autoPublishTarget}
	<Modal
		title={autoPublishTarget?.auto_publish_enabled ? 'Cabut izin mode otomatis?' : 'Izinkan mode otomatis?'}
		label="Konfirmasi izin mode otomatis"
		maxWidth="md"
		onClose={() => { if (busyId === null) autoPublishTarget = null; }}
	>
		<p class="text-sm text-muted-foreground">
			{#if autoPublishTarget?.auto_publish_enabled} {tr("Mitra")} <strong>{autoPublishTarget.name ?? autoPublishTarget.email}</strong> tidak lagi bisa menyalakan mode otomatis di medianya. Publikasi berikutnya kembali menunggu ACC mitra.
			{:else} {tr("Mitra")} <strong>{autoPublishTarget.name ?? autoPublishTarget.email}</strong> {tr("boleh menyalakan mode otomatis pada medianya sendiri. Artikel ke media itu akan terbit tanpa ACC mitra.")} {/if}
		</p>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" disabled={busyId !== null} onclick={() => (autoPublishTarget = null)}>{tr("Batal")}</button>
			<button
				class="app-btn app-btn-primary"
				type="button"
				disabled={busyId !== null}
				onclick={() => { if (autoPublishTarget) void setAutoPublish(autoPublishTarget, !autoPublishTarget?.auto_publish_enabled); }}
			>{autoPublishTarget?.auto_publish_enabled ? 'Cabut izin' : 'Izinkan'}</button>
		{/snippet}
	</Modal>
{/if}

{#if deleting}
	<Modal title={tr("Hapus partner?")} label={tr("Hapus partner")} maxWidth="md" onClose={() => (deleting = null)}>
		<p class="text-sm text-muted-foreground">
			Yakin mau hapus <strong>{deleting.name ?? deleting.email}</strong>{tr("? Tindakan ini menghapus permanen partner beserta seluruh data turunannya (media, dokumen, publikasi, penarikan, dan lainnya) dan tidak dapat dibatalkan.")}</p>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => (deleting = null)}>{tr("Batal")}</button>
			<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void confirmDelete()}>
				{busy ? 'Menghapus…' : 'Ya, hapus partner'}
			</button>
		{/snippet}
	</Modal>
{/if}

{#if roleTarget}
	<Modal title={`Ubah peran — ${roleTarget.name ?? roleTarget.email}`} label={tr("Ubah peran")} maxWidth="md" onClose={() => (roleTarget = null)}>
		<p class="text-sm text-muted-foreground">
			{tr('Peran menentukan dasbor yang dipakai akun ini:')}
			<strong>{tr('Mitra')}</strong> {tr('mengelola media dan menerima publikasi;')}
			<strong>{tr('Klien')}</strong> {tr('mengelola brand dan kontennya.')}
		</p>
		<div class="mt-3 grid gap-1.5">
			<span class="text-xs font-medium text-muted-foreground">{tr('Peran baru')}</span>
			<Select
				value={roleNext}
				ariaLabel={tr('Peran baru')}
				minWidth={260}
				onchange={(v) => (roleNext = v === 'partner' ? 'partner' : 'client')}
				options={[
					{ value: 'client', label: tr('Klien — kelola brand & konten') },
					{ value: 'partner', label: tr('Mitra — media partner') }
				]}
			/>
		</div>
		{#if roleNext === 'partner'}
			<label class="mt-3 flex items-start gap-2 text-sm">
				<input type="checkbox" class="mt-0.5" bind:checked={roleKeepBrands} />
				<span>{tr('Pertahankan brand miliknya — kalau tidak dicentang, brand dilepas saat jadi Mitra.')}</span>
			</label>
		{/if}
		{#if roleError}
			<div class="mt-3 flex items-start gap-2 text-sm" style="color: var(--destructive);"><TriangleAlert size={16} class="mt-0.5 shrink-0" />{roleError}</div>
		{/if}
		{#if roleNotice}
			<div class="mt-3 flex items-start gap-2 text-sm" style="color: var(--success);"><Check size={16} class="mt-0.5 shrink-0" />{roleNotice}</div>
		{/if}
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" disabled={roleBusy} onclick={() => (roleTarget = null)}>{tr("Tutup")}</button>
			<button class="app-btn app-btn-primary" type="button" disabled={roleBusy} onclick={() => void submitRoleChange()}>
				{roleBusy ? tr('Menyimpan…') : tr('Simpan peran')}
			</button>
		{/snippet}
	</Modal>
{/if}
