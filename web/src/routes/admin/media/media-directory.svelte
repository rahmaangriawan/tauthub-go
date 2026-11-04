<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Tab Direktori /admin/media — master data media (harga, kredit, tier, integrasi, partner).
	// Port MediaDirectorySection di apps/web/src/app/admin/media/_sections.tsx.
	import { onDestroy, onMount } from 'svelte';
	import { CircleAlert, CircleCheck, ExternalLink, Info, Pencil, Plus, RefreshCw, Search, ShieldCheck, Users } from '@lucide/svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { LINK_DURATION_OPTIONS, normalizeLinkDurationMonths } from '$lib/media/link-duration';
	import { fmtDateTime, statusLabel } from '$lib/admin-format';
	import { rp } from '$lib/credits/rupiah';
	import {
		compactNumber,
		errMessage,
		idr,
		isAccessError,
		mountWithSession,
		platformRequest,
		scoreTone,
		MEDIA_INTEGRATION_TYPES_UI,
		MEDIA_STATUSES,
		MEDIA_TIERS,
		categoryText,
		AUDIT_LABEL,
		humanizeAuditAction,
		auditTone,
		type AuditItem,
		type Media,
		type Partner
	} from './_shared';

	let { onPendingRefresh }: { onPendingRefresh?: () => void } = $props();

	const PAGE_SIZE = 20;

	let rows = $state<Media[]>([]);
	let total = $state(0);
	let partners = $state<Partner[]>([]);
	let error = $state('');
	let loading = $state(true);
	let createOpen = $state(false);
	let editing = $state<Media | null>(null);
	let daEditing = $state(false);
	let daInput = $state('');
	// Judul dialog edit: domain saja + favicon (pola sama dengan media-detail-modal);
	// favicon gagal muat → jatuh ke inisial.
	const editDomain = $derived(
		(editing?.website_url ?? '').replace(/^https?:\/\/(www\.)?/i, '').replace(/\/+$/, '').trim().toLowerCase()
	);
	let editFaviconFailed = $state(false);
	$effect(() => {
		void editDomain;
		editFaviconFailed = false;
	});
	let busy = $state(false);
	let selectedIds = $state<Set<string>>(new Set());
	let bulkBusy = $state(false);
	let bulkDeleteOpen = $state(false);
	let bulkResult = $state<{ archived: number; blocked: Array<{ name: string; reason: string }> } | null>(null);
	// Ubah jenis tautan massal: hanya Super Admin (server juga menegakkan).
	let isSuperAdmin = $state(false);
	let bulkLinkOpen = $state(false);
	let bulkLinkKind = $state<'dofollow' | 'nofollow' | 'sponsored'>('dofollow');
	let bulkLinkResult = $state<{ updated: number; blocked: Array<{ name: string; reason: string }> } | null>(null);
	let tcRate = $state(1000);
	let markupPercent = $state(15);
	let search = $state('');
	let statusFilter = $state('');
	// Urutan skor (0-100). Filter minimum DA/GEO sudah dihapus.
	let sortBy = $state('');
	let page = $state(1);
	let detailAudit = $state<AuditItem[] | null>(null);
	let detailLoading = $state(false);
	// Panel pelanggaran + suspend di modal edit media.
	let violations = $state<Array<{ id: string; level: string; note: string | null; created_at: string; actor_email: string | null; revoked_at: string | null }>>([]);
	let violationsLoading = $state(false);
	let violationOpen = $state(false);
	let violationDraft = $state<{ level: 'ringan' | 'berat'; note: string }>({ level: 'ringan', note: '' });
	let violationBusy = $state(false);
	let suspendOpen = $state(false);
	let suspendReason = $state('');
	let suspendBusy = $state(false);
	/** Baris yang sedang mengganti visibilitas (Publik ⇄ Hidden). */
	let visBusyId = $state<string | null>(null);
	let recomputing = $state<string | null>(null);
	let syncing = $state(false);
	let swBusyId = $state<string | null>(null);
	let swBulkBusy = $state(false);
	let swSummary = $state('');
	let swStatus = $state<{
		pending_jobs: number;
		queued_jobs: number;
		running_jobs: number;
		failed_last_hour: number;
		stale: number;
		last_error: string | null;
		/** Kesiapan penyedia — dipakai chip "Cek otomatis". */
		auto_ready?: boolean;
		traffic_ready?: boolean;
		traffic_blocked_by?: string;
		seo_ready?: boolean;
		seo_blocked_by?: string;
	} | null>(null);
	let swStatusTimer: ReturnType<typeof setTimeout> | undefined;
	let swVisits = $state<Record<string, number | null>>({});
	let aiBusyId = $state<string | null>(null);
	let geoBusyId = $state<string | null>(null);
	let daBulkBusy = $state(false);
	let daSummary = $state('');
	// Indikator antrean pemeriksaan DA/AI (job media_seo_refresh).
	let daStatus = $state<{
		pending_jobs: number;
		queued_jobs: number;
		running_jobs: number;
		stale: number;
		failed_last_hour: number;
		last_error: string | null;
		da_error: string | null;
		da_error_at: string | null;
		/** Domain yang gagal diperiksa dalam sejam terakhir (dari payload job). */
		failed_items?: Array<{ media_name: string; website_url: string; last_error: string; at: string | null }>;
	} | null>(null);
	let daStatusTimer: ReturnType<typeof setTimeout> | null = null;
	let daCancelBusy = $state(false);
	let reviewTarget = $state<Media | null>(null);

	let rejectReason = $state('');
	let reviewBusy = $state(false);
	let negoPrice = $state('');
	let priceConfirmOpen = $state(false);
	let priceNote = $state('');

	let form = $state({
		name: '',
		category: '',
		website_url: '',
		price: '0',
		status: 'active',
		integration_type: 'manual',
		partner_id: '',
		publish_defaults: '',
		traffic_estimate: '0',
		media_tier: 'tier3',
		media_quality_score: '',
		media_quality_notes: '',
		seo_da: '',
		link_duration_months: '12',
		max_links: '1',
		label_sponsor: 'tidak_ada',
		allow_hyperlink: true,
		pricing_type: 'nett' as 'nett' | 'nego',
		// Katalog publik (data.tauthub.com) — hanya admin yang boleh mengubah.
		public_listed: true,
		public_slug: ''
	});

	// Jenis tautan = kombinasi label_sponsor + allow_hyperlink; jumlah backlink
	// (max_links) diatur terpisah. Nofollow = tetap boleh pasang tautan, tapi
	// sistem menambahkan rel nofollow otomatis (artikel tidak ditolak).
	type LinkKind = 'dofollow' | 'nofollow' | 'sponsored';
	const LINK_KINDS: { value: LinkKind; label: string }[] = [
		{ value: 'dofollow', label: 'Dofollow' },
		{ value: 'nofollow', label: 'Nofollow' },
		{ value: 'sponsored', label: 'Sponsored' }
	];
	const linkKind = $derived<LinkKind>(
		form.label_sponsor === 'ada_sponsor' ? 'sponsored' : form.allow_hyperlink ? 'dofollow' : 'nofollow'
	);
	function setLinkKind(kind: LinkKind) {
		form.label_sponsor = kind === 'sponsored' ? 'ada_sponsor' : 'tidak_ada';
		form.allow_hyperlink = kind !== 'nofollow';
		if (Number(form.max_links) < 1) form.max_links = '1';
	}

	// Pencarian sisi server (debounced) — cari nama/slug/kategori media.
	let debouncedSearch = $state('');
	let searchTimer: ReturnType<typeof setTimeout> | null = null;
	$effect(() => {
		const value = search.trim();
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => (debouncedSearch = value), 300);
		return () => {
			if (searchTimer) clearTimeout(searchTimer);
		};
	});

	let ready = $state(false);
	onMount(() => mountWithSession(async () => (ready = true)));
	$effect(() => {
		if (!ready) return;
		void loadDaStatus();
	});
	$effect(() => {
		if (!ready) return;
		void (async () => {
			try {
				const me = await platformRequest<{ role?: string }>('/admins/me');
				isSuperAdmin = me?.role === 'super_admin';
			} catch {
				isSuperAdmin = false;
			}
		})();
	});
	onDestroy(() => {
		if (daStatusTimer) clearTimeout(daStatusTimer);
	});

	// Layout HP (< 768px): daftar jadi kartu, alat pemeriksaan dilipat.
	let isMobile = $state(false);
	let toolsOpen = $state(false);
	$effect(() => {
		const mq = window.matchMedia('(max-width: 767px)');
		const sync = () => (isMobile = mq.matches);
		sync();
		mq.addEventListener('change', sync);
		return () => mq.removeEventListener('change', sync);
	});

	$effect(() => {
		if (!ready) return;
		void debouncedSearch;
		void statusFilter;
		void sortBy;
		void page;
		void load();
	});

	// Setelah deploy, halaman yang sudah terbuka bisa masih memegang respons
	// lama tanpa kolom verifikasi/pemilik. Ambil ulang sekali supaya ikon
	// Verifikasi tidak tampil "-" untuk semua media hanya karena data basi.
	let fieldsBackfill = false;
	$effect(() => {
		if (!fieldsBackfill && rows.length > 0 && !('domain_verified' in rows[0])) {
			fieldsBackfill = true;
			void load();
		}
	});

	async function load() {
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams({
				limit: String(PAGE_SIZE),
				offset: String((page - 1) * PAGE_SIZE)
			});
			if (debouncedSearch) params.set('search', debouncedSearch);
		if (statusFilter) params.set('status', statusFilter);
		if (sortBy) params.set('sort', sortBy);
			const [mediaData, partnerData, creditsData, pricingData] = await Promise.all([
				platformRequest<{ media: Media[]; total: number }>(`/media?${params}`),
				platformRequest<{ partners: Partner[] }>('/partners'),
				platformRequest<{ tc_config?: { rate_idr?: number } }>('/credits'),
				platformRequest<{ markup_percent?: number }>('/pricing').catch(() => null)
			]);
			rows = mediaData.media ?? [];
			total = mediaData.total ?? 0;
			partners = partnerData.partners ?? [];
			const rate = creditsData.tc_config?.rate_idr;
			if (rate) tcRate = rate;
			if (pricingData?.markup_percent != null) markupPercent = pricingData.markup_percent;
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			loading = false;
		}
	}

	// Pricing engine (server = sumber kebenaran; preview ini hanya bantuan UX):
	// client_price = harga mitra × (1 + markup/100); kredit dari CLIENT price.
	const clientPriceFor = (base: number) => Math.round(base * (1 + (Number.isFinite(markupPercent) ? markupPercent : 0) / 100));
	const creditForPrice = (price: number) => {
		const client = clientPriceFor(price);
		return client > 0 ? Math.max(1, Math.ceil(client / (tcRate || 1000))) : 0;
	};

	function openCreate() {
		editing = null;
		detailAudit = null;
		form = {
			name: '',
			category: '',
			website_url: '',
			price: '0',
			status: 'active',
			integration_type: 'manual',
			partner_id: '',
			publish_defaults: '',
			traffic_estimate: '0',
			media_tier: 'tier3',
			media_quality_score: '',
			media_quality_notes: '',
			seo_da: '',
			link_duration_months: '12',
			max_links: '1',
			label_sponsor: 'tidak_ada',
			allow_hyperlink: true,
			pricing_type: 'nett',
			public_listed: true,
			public_slug: ''
		};
		createOpen = true;
	}

	function openEdit(row: Media) {
		editing = row;
		form = {
			name: row.name,
			category: row.category ?? '',
			website_url: row.website_url ?? '',
			price: String(row.price ?? 0),
			status: row.status,
			integration_type: row.integration_type,
			partner_id: row.partner_id ?? '',
			publish_defaults: row.publish_defaults ? JSON.stringify(row.publish_defaults) : '',
			traffic_estimate: String(row.traffic_estimate ?? 0),
			media_tier: row.media_tier,
			media_quality_score: row.media_quality_score != null ? String(row.media_quality_score) : '',
			media_quality_notes: row.media_quality_notes ?? '',
			seo_da: row.seo_da != null ? String(row.seo_da) : '',
			link_duration_months: String(normalizeLinkDurationMonths(row.link_duration_months)),
			max_links: ['0', '1', '2', '3'].includes(String(row.max_links ?? 1)) ? String(row.max_links ?? 1) : '1',
			label_sponsor: row.label_sponsor === 'ada_sponsor' ? 'ada_sponsor' : 'tidak_ada',
			allow_hyperlink: row.allow_hyperlink ?? Number(row.max_links ?? 1) > 0,
			pricing_type: row.pricing_type ?? 'nett',
			public_listed: row.public_listed !== false,
			public_slug: row.public_slug ?? ''
		};
		detailAudit = null;
		detailLoading = true;
		createOpen = true;
		void platformRequest<{ items: AuditItem[] }>(`/media/${row.id}/audit`)
			.then((data) => (detailAudit = data.items ?? []))
			.catch(() => (detailAudit = []))
			.finally(() => (detailLoading = false));
		void loadViolations(row.id);
	}

	/** Riwayat pelanggaran media (panel detail admin). */
	async function loadViolations(mediaID: string) {
		violationsLoading = true;
		try {
			const data = await platformRequest<{ items: typeof violations }>(`/media/${mediaID}/violations`);
			violations = data.items ?? [];
		} catch {
			violations = [];
		} finally {
			violationsLoading = false;
		}
	}

	/** Catat pelanggaran (ringan/berat) dengan keterangan wajib. */
	async function saveViolation() {
		if (!editing) return;
		if (!violationDraft.note.trim()) {
			error = tr('Keterangan pelanggaran wajib diisi.');
			return;
		}
		violationBusy = true;
		error = '';
		try {
			await platformRequest(`/media/${editing.id}/violations`, {
				method: 'POST',
				body: JSON.stringify({ level: violationDraft.level, note: violationDraft.note.trim() })
			});
			violationOpen = false;
			violationDraft = { level: 'ringan', note: '' };
			await loadViolations(editing.id);
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			violationBusy = false;
		}
	}

	/** Cabut pelanggaran aktif — riwayatnya tetap tersimpan. */
	async function revokeViolation() {
		if (!editing) return;
		if (!confirm(tr('Cabut status pelanggaran untuk media ini? Riwayatnya tetap tersimpan.'))) return;
		violationBusy = true;
		try {
			await platformRequest(`/media/${editing.id}/violations`, {
				method: 'POST',
				body: JSON.stringify({ level: 'none' })
			});
			await loadViolations(editing.id);
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			violationBusy = false;
		}
	}

	/**
	 * Ganti visibilitas katalog: Publik (tampil di katalog klien) ⇄ Hidden.
	 *
	 * Memakai PUT /media/{id} dengan dua field sekaligus:
	 *   - status: 'active' | 'hidden'  → menentukan tampil/tidak di katalog klien
	 *   - public_listed: true | false   → menentukan tampil/tidak di katalog publik
	 * Media yang baru masuk direktori selalu Publik (default kolom DB = true).
	 */
	async function toggleVisibility(row: Media, makePublic: boolean) {
		visBusyId = row.id;
		error = '';
		try {
			await platformRequest(`/media/${row.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					status: makePublic ? 'active' : 'hidden',
					public_listed: makePublic
				})
			});
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			visBusyId = null;
		}
	}

	/** Suspend domain: sembunyikan dari katalog + tolak publikasi baru. */	async function doSuspend() {
		if (!editing) return;
		if (!suspendReason.trim()) {
			error = tr('Alasan penangguhan wajib diisi.');
			return;
		}
		suspendBusy = true;
		error = '';
		try {
			await platformRequest(`/media/${editing.id}/suspend`, {
				method: 'POST',
				body: JSON.stringify({ reason: suspendReason.trim() })
			});
			suspendOpen = false;
			suspendReason = '';
			await load();
			// Baris di tabel sudah diperbarui; sinkronkan objek yang sedang diedit.
			if (editing) {
				const fresh = rows.find((r) => r.id === editing!.id);
				if (fresh) editing = fresh;
			}
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			suspendBusy = false;
		}
	}

	async function doUnsuspend() {
		if (!editing) return;
		if (!confirm(tr('Cabut penangguhan domain ini? Media akan tampil lagi di katalog.'))) return;
		suspendBusy = true;
		try {
			await platformRequest(`/media/${editing.id}/unsuspend`, { method: 'POST' });
			await load();
			const fresh = rows.find((r) => r.id === editing!.id);
			if (fresh) editing = fresh;
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			suspendBusy = false;
		}
	}

	async function saveDaManual() {
		if (!editing) return;
		const val = daInput.trim();
		const da = val === '' ? null : Number(val);
		if (val !== '' && (isNaN(da!) || da! < 0 || da! > 100)) return;
		await platformRequest(`/media/${editing.id}`, {
			method: 'PUT',
			body: JSON.stringify({ seo_da: da })
		});
		editing = { ...editing, seo_da: da };
		daEditing = false;
		void load();
	}

	function closeForm() {
		createOpen = false;
		editing = null;
		daEditing = false;
		priceConfirmOpen = false;
		priceNote = '';
		violationOpen = false;
		violationDraft = { level: 'ringan', note: '' };
		suspendOpen = false;
		suspendReason = '';
		violations = [];
	}

	async function doSave() {
		busy = true;
		error = '';
		try {
			let publishDefaults: unknown = null;
			if (form.publish_defaults.trim()) {
				try {
					publishDefaults = JSON.parse(form.publish_defaults);
				} catch {
					error = tr('publish_defaults bukan JSON yang valid.');
					return;
				}
			}
			const price = Number(form.price) || 0;
			const payload: Record<string, unknown> = {
				name: form.name.trim(),
				category: form.category.trim() || null,
				website_url: form.website_url.trim() || null,
				price,
				credit_cost: creditForPrice(price),
				status: form.status,
				integration_type: form.integration_type,
				partner_id: form.partner_id || null,
				publish_defaults: publishDefaults,
				traffic_estimate: Number(form.traffic_estimate) || 0,
				media_tier: form.media_tier,
					media_quality_score: form.media_quality_score === '' ? null : Number(form.media_quality_score),
					media_quality_notes: form.media_quality_notes.trim() || null,
					// DA manual hanya dikirim saat mengedit (media harus sudah ada).
					// Kosong = hapus nilai DA (dikirim null).
					seo_da: !editing ? undefined : form.seo_da.trim() === '' ? null : Number(form.seo_da),
					link_duration_months: Number(form.link_duration_months),
					max_links: Number(form.max_links),
					label_sponsor: form.label_sponsor,
					allow_hyperlink: form.allow_hyperlink,
					pricing_type: form.pricing_type,
					public_listed: form.public_listed,
					// Slug publik hanya dikirim bila diisi; rename media tidak boleh
					// memutus URL yang sudah terindeks.
					public_slug: form.public_slug.trim() || undefined,
				price_change_note:
					form.pricing_type !== (editing?.pricing_type ?? 'nett') ? priceNote.trim() || null : undefined
			};
			if (editing) {
				await platformRequest(`/media/${editing.id}`, { method: 'PUT', body: JSON.stringify(payload) });
			} else {
				await platformRequest('/media', { method: 'POST', body: JSON.stringify(payload) });
			}
			closeForm();
			await load();
			onPendingRefresh?.();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			busy = false;
		}
	}

	function onSubmit(event: SubmitEvent) {
		event.preventDefault();
		// Media terbit + harga berubah nego↔nett → konfirmasi dulu dengan catatan.
		if (editing && form.pricing_type !== (editing.pricing_type ?? 'nett')) {
			priceNote = '';
			priceConfirmOpen = true;
			return;
		}
		void doSave();
	}

	async function updateStatus(media: Media, status: string) {
		error = '';
		try {
			await platformRequest(`/media/${media.id}`, { method: 'PUT', body: JSON.stringify({ status }) });
			await load();
		} catch (reason) {
			error = errMessage(reason);
		}
	}

	/**
	 * Buka dialog suspend untuk satu baris tabel (dari kolom Aksi).
	 *
	 * Dulu kolom ini berisi tombol "Nonaktifkan" yang mengubah status menjadi
	 * 'inactive' — status yang tidak dipakai di mana pun: tidak ada media
	 * berstatus itu, dan efeknya sama saja dengan menyembunyikan media tanpa
	 * mencatat alasan. Sekarang diganti suspend yang alasannya wajib diisi dan
	 * bisa dicabut.
	 */
	function openSuspend(row: Media) {
		editing = row;
		suspendReason = '';
		suspendOpen = true;
		error = '';
	}

	/** Cabut suspend langsung dari tabel (tanpa membuka modal edit). */
	async function unsuspendRow(row: Media) {
		if (!confirm(`${tr('Cabut penangguhan')} "${row.name}"? ${tr('Media akan tampil lagi di katalog.')}`)) return;
		suspendBusy = true;
		error = '';
		try {
			await platformRequest(`/media/${row.id}/unsuspend`, { method: 'POST' });
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			suspendBusy = false;
		}
	}

	async function approveMedia(media: Media) {
		reviewBusy = true;
		error = '';
		try {
			await platformRequest(`/media/${media.id}/review`, {
				method: 'POST',
				body: JSON.stringify({ decision: 'approve' })
			});
			await load();
			onPendingRefresh?.();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			reviewBusy = false;
		}
	}

	async function rejectMedia() {
		if (!reviewTarget) return;
		const reason = rejectReason.trim();
		if (!reason) {
			error = 'Alasan penolakan wajib diisi.';
			return;
		}
		reviewBusy = true;
		error = '';
		try {
			await platformRequest(`/media/${reviewTarget.id}/review`, {
				method: 'POST',
				body: JSON.stringify({ decision: 'reject', reason })
			});
			reviewTarget = null;
			rejectReason = '';
			await load();
			onPendingRefresh?.();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			reviewBusy = false;
		}
	}

	async function negotiateMedia() {
		if (!reviewTarget) return;
		const price = Number(negoPrice.replace(/\D/g, ''));
		if (!price) {
			error = tr('Harga negosiasi wajib diisi.');
			return;
		}
		reviewBusy = true;
		error = '';
		try {
			await platformRequest(`/media/${reviewTarget.id}/review`, {
				method: 'POST',
				body: JSON.stringify({
					decision: 'negotiate',
					reason: rejectReason.trim() || 'Negosiasi harga',
					negotiated_price: price
				})
			});
			reviewTarget = null;
			rejectReason = '';
			negoPrice = '';
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			reviewBusy = false;
		}
	}

	async function bulkArchive() {
		bulkBusy = true;
		error = '';
		bulkResult = null;
		try {
			const ids = [...selectedIds];
			const result = await platformRequest<{
				archived: number;
				blocked: Array<{ id: string; name: string; reason: string }>;
			}>('/media/bulk', { method: 'POST', body: JSON.stringify({ ids }) });
			bulkResult = {
				archived: result.archived,
				blocked: (result.blocked ?? []).map((b) => ({ name: b.name, reason: b.reason }))
			};
			selectedIds = new Set();
			bulkDeleteOpen = false;
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			bulkBusy = false;
		}
	}

	async function applyBulkLinkKind() {
		bulkBusy = true;
		error = '';
		bulkLinkResult = null;
		try {
			const ids = [...selectedIds];
			const result = await platformRequest<{
				updated: number;
				blocked: Array<{ id: string; name: string; reason: string }>;
			}>('/media/bulk-link-kind', { method: 'POST', body: JSON.stringify({ ids, kind: bulkLinkKind }) });
			bulkLinkResult = {
				updated: result.updated,
				blocked: (result.blocked ?? []).map((b) => ({ name: b.name, reason: b.reason }))
			};
			selectedIds = new Set();
			bulkLinkOpen = false;
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			bulkBusy = false;
		}
	}

	async function syncCredits() {
		syncing = true;
		error = '';
		try {
			const result = await platformRequest<{ synced: number; rate_idr: number }>('/media/sync-credit-cost', {
				method: 'POST'
			});
			error = `Sinkronisasi kredit selesai: ${result.synced} media diperbarui.`;
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			syncing = false;
		}
	}

	async function recomputeHealth(media: Media) {
		error = '';
		recomputing = media.id;
		try {
			await platformRequest(`/media/${media.id}/recompute-health`, { method: 'POST' });
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			recomputing = null;
		}
	}

	async function checkSimilarWeb(media: Media) {
		error = '';
		swBusyId = media.id;
		try {
			const r = await platformRequest<{ visits: number | null; cached: boolean }>(
				`/media/${media.id}/similarweb`,
				{ method: 'POST' }
			);
			swVisits = { ...swVisits, [media.id]: r.visits };
			if (r.visits != null) {
				rows = rows.map((row) => (row.id === media.id ? { ...row, traffic_estimate: r.visits as number } : row));
			}
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			swBusyId = null;
		}
	}

	/**
	 * Pemeriksaan trafik TIDAK dijalankan langsung — dulu satu klik memanggil
	 * penyedia untuk hingga 25 media berurutan (tiap media bisa 15 detik) dan
	 * selalu berakhir timeout proxy. Sekarang semua media kandidat masuk
	 * antrean (job media_similarweb_refresh) dan scheduler/worker yang
	 * melakukan panggilannya satu per satu; tabel menyusul lewat indikator.
	 */
	async function bulkSimilarWeb() {
		error = '';
		swBulkBusy = true;
		swSummary = 'Memasukkan antrian…';
		try {
			const r = await platformRequest<{
				enqueued: number;
				skipped: number;
				candidates: number;
				total_stale: number;
			}>('/media/similarweb-bulk', { method: 'POST', body: JSON.stringify({ limit: 25 }) });
			const sisa = r.total_stale - r.enqueued;
			swSummary =
				r.enqueued === 0
					? 'Tidak ada media yang perlu diisi trafiknya — semua media aktif sudah punya data. Media berdata tidak diperiksa ulang oleh tombol ini.'
					: `Antrean dibuat: ${r.enqueued} media tanpa data trafik dijadwalkan (sisa ${Math.max(0, sisa)}${r.skipped ? `, ${r.skipped} gagal antre` : ''}). Media yang sudah punya angka dilewati — scheduler memproses sisanya otomatis.`;
			await loadSwStatus();
			void load();
		} catch (reason) {
			error = errMessage(reason);
			swSummary = '';
		} finally {
			swBulkBusy = false;
		}
	}

	/** Status antrean cek trafik — indikator + auto-refresh tabel 15 dtk. */
	async function loadSwStatus() {
		try {
			swStatus = await platformRequest<{
				pending_jobs: number;
				queued_jobs: number;
				running_jobs: number;
				failed_last_hour: number;
				stale: number;
				last_error: string | null;
				auto_ready?: boolean;
				traffic_ready?: boolean;
				traffic_blocked_by?: string;
				seo_ready?: boolean;
				seo_blocked_by?: string;
			}>('/media/similarweb-status');
			if ((swStatus?.pending_jobs ?? 0) > 0) {
				if (swStatusTimer) clearTimeout(swStatusTimer);
				swStatusTimer = setTimeout(() => {
					void load();
					void loadSwStatus();
				}, 15_000);
			}
		} catch {
			/* indikator opsional — jangan ganggu daftar media */
		}
	}

	$effect(() => {
		if (swStatus === null) void loadSwStatus();
		return () => {
			if (swStatusTimer) clearTimeout(swStatusTimer);
		};
	});

	/**
	 * Pemeriksaan satu media TIDAK dijalankan langsung. Satu kali periksa butuh
	 * ~25-30 detik (kecepatan situs + penyedia DA + SERPens) sedangkan klien
	 * memutus koneksi di 12 detik, jadi dulu tombol ini selalu berakhir
	 * "Server terlalu lama merespons". Sekarang pekerjaannya diantrekan dan
	 * tabel menyusul lewat indikator antrean.
	 */
	async function checkGeo(media: Media) {
		error = '';
		geoBusyId = media.id;
		try {
			const r = await platformRequest<{ queued: boolean }>(`/media/${media.id}/geo-refresh`, {
				method: 'POST'
			});
			daSummary = r.queued
				? `Pemeriksaan GEO "${media.name}" masuk antrean — DA tidak ikut dipanggil.`
				: `"${media.name}" sudah ada di antrean pemeriksaan GEO.`;
			await loadDaStatus();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			geoBusyId = null;
		}
	}

	async function checkAiSignals(media: Media) {
		error = '';
		aiBusyId = media.id;
		try {
			const r = await platformRequest<{ queued: boolean }>(`/media/${media.id}/seo-refresh`, {
				method: 'POST'
			});
			daSummary = r.queued
				? `Pemeriksaan "${media.name}" masuk antrean — hasilnya muncul di tabel begitu selesai.`
				: `"${media.name}" sudah ada di antrean pemeriksaan.`;
			await loadDaStatus();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			aiBusyId = null;
		}
	}

	/** Status antrean cek DA — dipakai indikator dan auto-refresh tabel. */
	async function loadDaStatus() {
		try {
			daStatus = await platformRequest<{
				pending_jobs: number;
				queued_jobs: number;
				running_jobs: number;
				stale: number;
				failed_last_hour: number;
				last_error: string | null;
				da_error: string | null;
				da_error_at: string | null;
			}>('/media/seo-refresh-status');
			if ((daStatus?.pending_jobs ?? 0) > 0) {
				// Selama masih ada job menunggu/diproses, tabel + indikator
				// disegarkan sendiri tiap 15 dtk sampai antrean benar-benar kosong.
				if (daStatusTimer) clearTimeout(daStatusTimer);
				daStatusTimer = setTimeout(() => {
					void load();
					void loadDaStatus();
				}, 15_000);
			}
		} catch {
			/* indikator opsional — jangan ganggu daftar media */
		}
	}

	/** Hentikan antrean pemeriksaan yang sedang menunggu. */

	/**
	 * Antrekan pemeriksaan DA + AI Trust — satu klik untuk seluruh media yang
	 * relevan, bukan 25 per klik. Pemeriksaan berjalan di latar lewat job
	 * `media_seo_refresh`, jadi halaman tidak ikut menunggu.
	 *   - daOnly = true  → hanya media yang DA-nya belum ada (tombol "Cek DA semua")
	 *   - force  = true  → semua media aktif, termasuk yang sudah berskor
	 * Satu pemeriksaan mengisi DA dan AI Trust sekaligus, jadi ketiga tombol
	 * memakai job yang sama; yang berbeda hanya media mana yang dipilih.
	 */
	async function enqueueSeoRefresh(force = false, daOnly = false) {
		const pertanyaan = force
			? tr('Hitung ulang DA dan AI Trust untuk SEMUA media aktif, termasuk yang sudah punya skor?')
			: daOnly
				? tr('Cek DA untuk semua media yang nilai DA-nya belum ada? Media yang sudah punya DA dilewati.')
				: tr('Cek DA dan AI Trust untuk semua media yang belum punya skor? Media yang sudah dicek dilewati.');
		if (!confirm(pertanyaan)) return;
		error = '';
		daBulkBusy = true;
		daSummary = 'Mengantrekan pemeriksaan…';
		try {
			const r = await platformRequest<{ enqueued: number; total_stale: number; skipped: number }>(
				'/media/seo-refresh-all',
				{ method: 'POST', body: JSON.stringify({ force, da_only: daOnly }) }
			);
			if (r.enqueued === 0) {
				daSummary = force
					? 'Tidak ada media aktif yang bisa diperiksa (butuh website_url terisi).'
					: daOnly
						? 'Semua media yang punya website sudah punya nilai DA — tidak ada yang perlu dicek.'
						: tr('Semua media sudah punya skor — tidak ada yang perlu dicek. Pakai "Hitung ulang semua" kalau ingin memeriksa ulang.');
			} else {
				const apa = force ? 'dihitung ulang' : daOnly ? 'tanpa DA' : 'belum berskor';
				daSummary = `${r.enqueued} media ${apa} masuk antrean (dari ${r.total_stale} kandidat). Hasil muncul bertahap di tabel.`;
			}
			await loadDaStatus();
		} catch (reason) {
			error = errMessage(reason);
			daSummary = '';
		} finally {
			daBulkBusy = false;
		}
	}

	function toggleSelected(id: string, checked: boolean) {
		const next = new Set(selectedIds);
		if (checked) next.add(id);
		else next.delete(id);
		selectedIds = next;
	}

	function toggleSelectAll(checked: boolean) {
		const next = new Set(selectedIds);
		for (const row of rows) {
			if (checked) next.add(row.id);
			else next.delete(row.id);
		}
		selectedIds = next;
	}

	const allSelected = $derived(rows.length > 0 && rows.every((row) => selectedIds.has(row.id)));
	const showAccessState = $derived(rows.length === 0 && isAccessError(error));

	function domainOf(url: string): string {
		try { return new URL(url).hostname; } catch { return url; }
	}
</script>

<svelte:window onclick={() => (toolsOpen = false)} />

{#snippet selectCell(row: Media)}
	<input
		type="checkbox"
		checked={selectedIds.has(row.id)}
		aria-label={`Pilih ${row.name}`}
		onclick={(e) => e.stopPropagation()}
		onchange={(e) => toggleSelected(row.id, e.currentTarget.checked)}
	/>
{/snippet}

{#snippet selectAllHeader()}
	<input
		type="checkbox"
		checked={allSelected}
		aria-label={tr('Pilih halaman ini')}
		title={tr('Pilih halaman ini')}
		onchange={(e) => toggleSelectAll(e.currentTarget.checked)}
	/>
{/snippet}

{#snippet mediaCell(row: Media)}
	<div class="flex items-center gap-2">
		{#if row.website_url}
			<img
				src="https://www.google.com/s2/favicons?domain={encodeURIComponent(row.website_url)}&sz=16"
				alt=""
				class="h-4 w-4 shrink-0 rounded-[2px]"
				loading="lazy"
			/>
		{:else}
			<span class="h-4 w-4 shrink-0 rounded-[2px] bg-muted"></span>
		{/if}
		<div class="flex min-w-0 flex-col gap-0.5">
			<div class="flex items-center gap-1">
				<span class="truncate text-sm font-medium">{row.website_url ? domainOf(row.website_url) : (row.slug ?? row.id.slice(0, 8))}</span>
				{#if row.partner_id}<Badge tone="accent" value="Mitra" />{/if}
			</div>
			{#if row.partner_id}
				<!-- Tautan "Kontak" diberi border tipis supaya terbaca sebagai tombol
				     kecil, bukan teks biasa yang menempel di bawah domain. -->
				<a
					href="/admin/partners?partner_id={row.partner_id}"
					class="w-fit rounded border px-1.5 py-px text-[11px] font-medium leading-4 transition-colors hover:bg-[color-mix(in_srgb,var(--primary)_10%,transparent)]"
					style="color: var(--primary); border-color: color-mix(in srgb, var(--primary) 35%, transparent);"
					onclick={(e) => e.stopPropagation()}
				>Kontak</a>
			{/if}
		</div>
	</div>
{/snippet}

{#snippet integrationModeCell(row: Media)}
	<div class="flex flex-col items-start gap-1">
		{#if (row.publication_mode ?? 'draft') === 'publish'}
			<Badge tone="success" label="API" />
		{:else}
			<Badge tone="warning" label="Manual" />
		{/if}
		{#if row.auto_publish}
			<span class="text-[10px] text-muted-foreground leading-none">Secure Direct</span>
		{/if}
	</div>
{/snippet}

{#snippet dewanPersCell(row: Media)}
	{#if row.dewan_pers_verified}<Badge tone="success" label="Ya" />
	{:else}<Badge tone="neutral" label={tr("Tidak")} />{/if}
{/snippet}

{#snippet priceCell(row: Media)}
	<div class="tabular-nums">
		<p>{idr(row.price ?? 0)}</p>
		{#if row.price > 0}
			<p class="text-xs text-muted-foreground">klien {idr(row.client_price ?? 0)} · {row.markup_percent ?? 15}%</p>
		{/if}
	</div>
{/snippet}

{#snippet verifyCell(row: Media)}
	{#if row.domain_verified}
		<span
			class="inline-flex items-center"
			style="color: var(--success);"
			title={row.domain_verified_at
				? `Kode verifikasi domain ditemukan (${fmtDateTime(row.domain_verified_at)})`
				: tr('Kode verifikasi domain ditemukan di halaman media')}
			aria-label={tr('Domain terverifikasi')}
		>
			<ShieldCheck size={16} />
		</span>
	{:else}
		<span
			class="text-sm text-muted-foreground"
			title={tr('Kode verifikasi belum ditemukan — meta tag tauthub-verification atau TXT DNS belum ada')}
			aria-label={tr('Domain belum terverifikasi')}
		>-</span>
	{/if}
{/snippet}

{#snippet actionsCell(row: Media)}
	<div class="flex flex-wrap justify-center gap-1">
		{#if row.status === 'pending_review'}
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--success);"
				disabled={reviewBusy}
				onclick={(e) => {
					e.stopPropagation();
					void approveMedia(row);
				}}>Setujui</button
			>
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--destructive);"
				disabled={reviewBusy}
				onclick={(e) => {
					e.stopPropagation();
					reviewTarget = row;
					rejectReason = '';
					error = '';
				}}>Tolak</button
			>
		{/if}
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			onclick={(e) => {
				e.stopPropagation();
				openEdit(row);
			}}>Edit</button
		>
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			disabled={recomputing === row.id}
			onclick={(e) => {
				e.stopPropagation();
				void recomputeHealth(row);
			}}>{recomputing === row.id ? 'Menghitung…' : 'Hitung health'}</button
		>
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			disabled={aiBusyId === row.id}
			onclick={(e) => {
				e.stopPropagation();
				void checkAiSignals(row);
			}}>{aiBusyId === row.id ? 'Mengantrekan…' : tr('Cek DA')}</button
		>
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			disabled={geoBusyId === row.id}
			title={tr('Cek GEO/AI Trust saja — tidak memanggil penyedia DA/PA, jadi tidak memakai kuota berbayar')}
			onclick={(e) => {
				e.stopPropagation();
				void checkGeo(row);
			}}>{geoBusyId === row.id ? 'Mengantrekan…' : tr('Cek GEO')}</button
		>
		{#if row.status === 'active' && !row.is_suspended}
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--destructive);"
				title={tr('Sembunyikan dari katalog + tolak publikasi baru, dengan alasan tercatat. Bisa dicabut kapan saja.')}
				onclick={(e) => {
					e.stopPropagation();
					openSuspend(row);
				}}>{tr('Suspend')}</button
			>
		{:else if row.is_suspended}
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--success);"
				title={row.suspended_reason ?? undefined}
				onclick={(e) => {
					e.stopPropagation();
					void unsuspendRow(row);
				}}>{tr('Cabut suspend')}</button
			>
		{:else if row.status !== 'pending_review' && row.status !== 'rejected'}
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--success);"
				onclick={(e) => {
					e.stopPropagation();
					void updateStatus(row, 'active');
				}}>Aktifkan</button
			>
		{/if}
		{#if row.status !== 'archived'}
			<button
				class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
				style="color: var(--destructive);"
				onclick={(e) => {
					e.stopPropagation();
					void updateStatus(row, 'archived');
				}}>Arsipkan</button
			>
		{/if}
	</div>
{/snippet}

{#snippet visibilityCell(row: Media)}
	{@const isPublic = row.public_listed !== false && row.status !== 'hidden'}
	<!-- Satu tombol bergeser (toggle) untuk Publik/Hidden. Saat diklik, knob
	     bergeser dengan animasi dan label ikut berganti, jadi tidak perlu dua
	     tombol berdampingan di kolom sempit. -->
	<button
		type="button"
		class="visToggle"
		class:visToggleOn={isPublic}
		role="switch"
		aria-checked={isPublic}
		aria-label={`${tr('Visibilitas katalog')}: ${isPublic ? tr('Publik') : 'Hidden'}`}
		title={isPublic
			? tr('Tampil di katalog media klien. Klik untuk menyembunyikan (hidden).')
			: tr('Disembunyikan dari katalog klien. Klik untuk menampilkan (publik).')}
		disabled={visBusyId === row.id}
		onclick={(e) => {
			e.stopPropagation();
			void toggleVisibility(row, !isPublic);
		}}
	>
		<span class="visToggleTrack" aria-hidden="true">
			<span class="visToggleKnob"></span>
		</span>
		<span class="visToggleLabel">{isPublic ? tr('Publik') : 'Hidden'}</span>
	</button>
{/snippet}

{#snippet integrationCell(row: Media)}
	<Badge tone="neutral" value={row.integration_type} />
{/snippet}

{#if loading && rows.length === 0 && !error}
	<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat direktori media…")}</p>
{:else if showAccessState}
	<div class="app-card mt-6 px-6 py-10 text-center">
		<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke direktori media.")}</p>
	</div>
{:else}
	<div class="mdir-toolbar mt-4 flex flex-wrap items-center gap-2">
		<label class="app-input flex w-full items-center gap-2 !py-1.5 sm:min-w-[240px] sm:flex-1 sm:w-auto">
			<Search size={15} class="text-muted-foreground" />
			<input
				class="w-full bg-transparent text-sm outline-none"
					placeholder={tr("Cari media, ID mitra, nama mitra, atau email mitra…")}
				aria-label={tr("Cari media")}
				bind:value={search}
				oninput={() => (page = 1)}
			/>
		</label>
		{#if debouncedSearch}
			<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" onclick={() => (search = '')}
				>Bersihkan pencarian</button
			>
		{/if}
		<Select
			ariaLabel={tr("Filter status media")}
			minWidth={isMobile ? 0 : 170}
			value={statusFilter}
			onchange={(v) => {
				statusFilter = v;
				page = 1;
			}}
			options={[
				{ value: '', label: tr('Semua status') },
				{ value: 'active', label: tr('🟢 Aktif') },
				{ value: 'inactive', label: tr('Nonaktif') },
				{ value: 'draft', label: 'Draf' },
				{ value: 'archived', label: tr('Arsip') }
			]}
		/>
		<Select
			ariaLabel={tr('Urutkan media')}
			minWidth={isMobile ? 0 : 210}
			value={sortBy}
			onchange={(v) => {
				sortBy = v;
				page = 1;
			}}
			options={[
				{ value: '', label: tr('Urut: Terbaru') },
				{ value: 'da_desc', label: tr('DA: terbesar → terkecil') },
				{ value: 'da_asc', label: tr('DA: terkecil → terbesar') },
				{ value: 'geo_desc', label: tr('GEO: terbesar → terkecil') },
				{ value: 'geo_asc', label: tr('GEO: terkecil → terbesar') },
				{ value: 'traffic_desc', label: tr('Trafik: terbesar → terkecil') },
				{ value: 'traffic_asc', label: tr('Trafik: terkecil → terbesar') },
				{ value: 'name_asc', label: tr('Nama A-Z') }
			]}
		/>
		<div class="hidden flex-1 sm:block"></div>
		<Badge tone="accent" value={`${total} media`} />
		<label class="mdir-pick flex items-center gap-1.5 text-xs text-muted-foreground">
			<input type="checkbox" checked={allSelected} onchange={(e) => toggleSelectAll(e.currentTarget.checked)} />{tr("Pilih halaman ini")}</label>
	</div>

	<div class="mdir-actionbar mt-2 flex flex-wrap items-center gap-2">
		{#if selectedIds.size > 0}
			<span class="text-xs font-medium">{selectedIds.size} {tr('dipilih')}</span>
			<button
				class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
				disabled={bulkBusy}
				onclick={() => (bulkDeleteOpen = true)}>Arsipkan {selectedIds.size} media</button
			>
			{#if isSuperAdmin}
				<button
					class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
					disabled={bulkBusy}
					onclick={() => (bulkLinkOpen = true)}>{tr('Ubah jenis tautan')} ({selectedIds.size})</button
				>
			{/if}
		{:else}
			<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" onclick={openCreate}>
				<Plus size={15} />{tr("Tambah media")}</button>
			<div class="mdir-maint relative">
				<button
					class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
					aria-haspopup="menu"
					aria-expanded={toolsOpen}
					onclick={(e) => { e.stopPropagation(); toolsOpen = !toolsOpen; }}
				>
					<RefreshCw size={15} />{tr('Pemeliharaan')}
					<span aria-hidden="true">{toolsOpen ? '▲' : '▼'}</span>
				</button>
				{#if toolsOpen}
					<div class="mdir-menu" role="menu" tabindex="-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.key === 'Escape' && (toolsOpen = false)}>
						<button role="menuitem" disabled={syncing} onclick={() => { toolsOpen = false; void syncCredits(); }}>
							{tr('Sinkronkan kredit semua')}
						</button>
						<button role="menuitem" disabled={swBulkBusy} onclick={() => { toolsOpen = false; void bulkSimilarWeb(); }} title="Antrekan pemeriksaan trafik HANYA untuk media yang belum pernah diperiksa SimilarWeb (belum punya snapshot). Media yang sudah pernah diperiksa — termasuk hasil 0 kunjungan — dilewati; scheduler yang memproses antreannya.">
							{swBulkBusy ? 'Mengantrekan…' : swStatus && swStatus.pending_jobs > 0 ? `Antrean Trafik (${swStatus.pending_jobs})` : 'Antrekan Cek Trafik Semua'}
						</button>
						<button role="menuitem" disabled={daBulkBusy} title={tr('Cek DA dan AI Trust untuk seluruh media yang belum punya skor; yang sudah dicek dilewati')} onclick={() => { toolsOpen = false; void enqueueSeoRefresh(); }}>
							{daBulkBusy ? 'Mengantrekan…' : tr('Cek AI & SEO Semua')}
						</button>
						<button role="menuitem" disabled={daBulkBusy} title={tr('Cek DA untuk semua media yang nilai DA-nya belum ada (DA dan AI Trust diambil pada pemeriksaan yang sama)')} onclick={() => { toolsOpen = false; void enqueueSeoRefresh(false, true); }}>
							{tr('Cek DA semua')}
						</button>
						<button role="menuitem" disabled={daBulkBusy} title={tr('Hitung ulang DA dan AI Trust untuk seluruh media aktif, termasuk yang sudah punya skor')} onclick={() => { toolsOpen = false; void enqueueSeoRefresh(true); }}>
							{tr('Hitung ulang semua')}
						</button>
					</div>
				{/if}
			</div>
		{/if}
		<div class="flex-1"></div>
		<!-- Chip "Cek otomatis": memberi tahu apakah DA & trafik diperiksa sendiri oleh
		     sistem, atau berhenti karena kredensial penyedia belum lengkap. -->
		{#if swStatus}
			{@const autoReady = swStatus.auto_ready !== false}
			{#if autoReady}
				<span class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs" style="background-color: var(--success-soft, #eaf8f1); color: var(--success, #16794b);">
					<CircleCheck size={13} aria-hidden="true" />
					{tr('Cek otomatis aktif')}
					{#if (swStatus.pending_jobs ?? 0) > 0}<span>· {tr('trafik')} {swStatus.pending_jobs} {tr('antre')}</span>{/if}
					{#if (daStatus?.pending_jobs ?? 0) > 0}<span>· {tr('DA')} {daStatus?.pending_jobs} {tr('antre')}</span>{/if}
				</span>
			{:else}
				<span class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs" style="background-color: var(--warning-soft, #fff6df); color: var(--warning, #a15c00);">
					<CircleAlert size={13} aria-hidden="true" />
					{tr('Cek otomatis berhenti')} — {(swStatus.traffic_blocked_by || swStatus.seo_blocked_by || tr('kredensial penyedia belum lengkap')).replace(/\.$/, '')}
					<a href="/admin/konfigurasi" style="color: var(--primary); text-decoration: underline;">{tr('Lengkapi di Konfigurasi')}</a>
				</span>
			{/if}
		{/if}
	</div>

	<!-- Status antrean trafik, antrean DA, dan sebab kegagalan sepenuhnya ada di
	     tab Log. Direktori sengaja tidak menampilkan penunjuk apa pun supaya
	     halaman kerja tetap bersih. -->
	{#if swSummary}<p class="mt-2 text-xs text-muted-foreground">{swSummary}</p>{/if}
	{#if daSummary}<p class="mt-2 text-xs text-muted-foreground">{daSummary}</p>{/if}
	{#if error && !isAccessError(error)}
		<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>
	{/if}

	{#if isMobile}
		<!-- HP: tiap media jadi satu kartu (tabel 9 kolom tidak muat di layar sempit). -->
		<div class="mdir-cards mt-4 space-y-3">
			{#if loading}
				<p class="app-card px-4 py-10 text-center text-sm text-muted-foreground">{tr("Memuat data…")}</p>
			{:else if rows.length === 0}
				<p class="app-card px-4 py-10 text-center text-sm text-muted-foreground">{tr("Belum ada media. Tambahkan media pertama.")}</p>
			{:else}
				{#each rows as row (row.id)}
					<article class="mdir-card app-card p-3.5">
						<div class="flex items-start gap-2">
							<label class="mdir-check">{@render selectCell(row)}</label>
							<div class="min-w-0 flex-1">{@render mediaCell(row)}</div>
							<div class="shrink-0">{@render visibilityCell(row)}</div>
						</div>
						<dl class="mt-3 grid grid-cols-2 gap-x-3 gap-y-2.5 text-sm">
							<div class="min-w-0">
								<dt class="text-xs text-muted-foreground">{tr('Harga')}</dt>
								<dd>{@render priceCell(row)}</dd>
							</div>
							<div class="min-w-0">
								<dt class="text-xs text-muted-foreground">Integrasi</dt>
								<dd>{@render integrationCell(row)}</dd>
							</div>
							<div class="min-w-0">
								<dt class="text-xs text-muted-foreground">{tr('Mode Integrasi')}</dt>
								<dd>{@render integrationModeCell(row)}</dd>
							</div>
							<div class="min-w-0">
								<dt class="text-xs text-muted-foreground">Dewan Pers</dt>
								<dd>{@render dewanPersCell(row)}</dd>
							</div>
							<div class="min-w-0">
								<dt class="text-xs text-muted-foreground">{tr('Verifikasi')}</dt>
								<dd>{@render verifyCell(row)}</dd>
							</div>
						</dl>
						<div class="mdir-actions mt-3 border-t pt-3">{@render actionsCell(row)}</div>
					</article>
				{/each}
			{/if}
		</div>
	{:else}
	<DataTable
		columns={[
			{ key: 'select', label: '', width: '36px', header: selectAllHeader, render: selectCell },
			{ key: 'name', label: tr('Media'), render: mediaCell },
			{ key: 'domain_verified', label: tr('Verifikasi'), align: 'center' as const, width: '96px', render: verifyCell },
			{ key: 'publication_mode', label: tr('Mode Integrasi'), width: '110px', render: integrationModeCell },
			// Kolom "Jumlah Kata" dilepas atas permintaan pemilik produk: isinya
			// (Sesuai Permintaan Klien / rentang kata) jarang dipakai saat memindai
			// tabel, dan masih bisa dilihat di modal edit.
			{ key: 'dewan_pers', label: 'Dewan Pers', render: dewanPersCell },
			{ key: 'price', label: tr('Harga'), render: priceCell },
			// Kategori, Trafik, Health, GEO/AI Trust, dan DA dipindah ke popup
			// detail supaya tabel cukup untuk memindai identitas & status.
			{ key: 'integration', label: 'Integrasi', render: integrationCell },
			// Status visibilitas: Publik (tampil di katalog klien) / Hidden.
			// Satu tombol bergeser (toggle) supaya tidak dua tombol berdampingan.
			{ key: 'visibility', label: 'Status', align: 'center' as const, width: '120px', render: visibilityCell },
			{ key: 'actions', label: 'Aksi', align: 'center' as const, width: '300px', render: actionsCell }
		]}
		rows={rows}
		rowKey={(row: Media) => row.id}
		{loading}
		empty={tr("Belum ada media. Tambahkan media pertama.")}
		minWidth="1150px"
		onRowClick={(row: Media) => openEdit(row)}
	/>
	{/if}

	<Pagination
		total={total}
		limit={PAGE_SIZE}
		offset={(page - 1) * PAGE_SIZE}
		onChange={(next) => (page = next / PAGE_SIZE + 1)}
	/>

	{#if bulkLinkResult}
		<div
			class="app-card mt-3 px-4 py-2.5 text-sm"
			style="color: {bulkLinkResult.blocked.length ? 'var(--warning)' : 'var(--success)'};"
		>
			{bulkLinkResult.updated} media {tr('jenis tautannya diubah.')}
			{bulkLinkResult.blocked.length > 0
				? ` Dilewati ${bulkLinkResult.blocked.length} media: ${bulkLinkResult.blocked
						.slice(0, 3)
						.map((b) => `${b.name} (${b.reason})`)
						.join(', ')}${bulkLinkResult.blocked.length > 3 ? ', …' : ''}.`
				: ''}
		</div>
	{/if}

	{#if bulkResult}
		<div
			class="app-card mt-3 px-4 py-2.5 text-sm"
			style="color: {bulkResult.blocked.length ? 'var(--warning)' : 'var(--success)'};"
		>
			{bulkResult.archived} media diarsipkan.
			{bulkResult.blocked.length > 0
				? ` Dilewati ${bulkResult.blocked.length} media: ${bulkResult.blocked
						.slice(0, 3)
						.map((b) => b.name)
						.join(', ')}${bulkResult.blocked.length > 3 ? ', …' : ''}.`
				: ''}
		</div>
	{/if}
{/if}

{#snippet catalogSettings()}
<!-- Katalog publik (data.tauthub.com). Flag ini hanya untuk admin;
     mitra tidak bisa mengubahnya. -->
<section class={editing ? 'mw-panel space-y-3' : 'space-y-3 border-t pt-4'}>
	<h3 class="mw-ptitle">Katalog publik</h3>
	<label class="mw-switch-row">
		<input type="checkbox" role="switch" class="mw-switch" bind:checked={form.public_listed} />
		<span>
			<strong class="mw-ink">{tr('Tampilkan di katalog publik')}</strong>
			<span class="mw-hint !mt-0.5">
				{tr('Media ini tampil di data.tauthub.com lengkap dengan harga publik, DA, trafik, dan kebijakan tautan. Matikan untuk menyembunyikannya tanpa mengubah status.')}
			</span>
		</span>
	</label>
	{#if editing}
		<label class="block">
			<span class="mw-label">{tr('Slug URL publik')}</span>
			<span class="mw-slug">
				<span class="mw-slug-prefix">data.tauthub.com/media/</span>
				<input
					class="mw-slug-input font-mono text-xs"
					bind:value={form.public_slug}
					placeholder={editing.public_slug ?? ''}
				/>
			</span>
			<span class="mw-hint first-letter:uppercase"
				>{tr('mengubahnya memutus tautan lama yang sudah terindeks mesin pencari.')}</span
			>
		</label>
	{/if}
</section>
{/snippet}

{#snippet partnerSettings()}
<section class={editing ? 'mw-panel space-y-3' : 'space-y-3 border-t pt-4'}>
	<h3 class="mw-ptitle">Mitra &amp; publish</h3>
	<label class="block">
		<span class="mw-label">{tr("Mitra (opsional)")}</span>
		<Select
			ariaLabel={tr("Mitra (opsional)")}
			minWidth={220}
			value={form.partner_id}
			onchange={(v) => (form.partner_id = v)}
			options={[
				{ value: '', label: 'Tanpa partner' },
				...partners.filter((p) => p.status === 'active').map((p) => ({ value: p.id, label: p.name ?? p.email }))
			]}
		/>
	</label>

	<label class="block">
		<span class="mw-label">Publish defaults (JSON, opsional)</span>
		<textarea class="app-input mt-1 w-full font-mono text-xs" rows="3" bind:value={form.publish_defaults}
		></textarea>
		<span class="mw-hint"
			>{'Contoh: {"author_id": 1, "categories": ["Teknologi"], "featured_image_url": "https://…"}'}</span
		>
	</label>
</section>
{/snippet}

{#if createOpen}
	<Modal
		title={editing ? editDomain || editing.name : tr('Tambah media')}
		label={editing ? `${tr('Edit media')} ${editDomain || editing.name}` : undefined}
		maxWidth={editing ? 'workspace' : '2xl'}
		onClose={closeForm}
	>
		{#snippet titleLead()}
			{#if editing}
				{#if editDomain && !editFaviconFailed}
					<img
						class="h-8 w-8 shrink-0 rounded-lg border bg-white object-contain p-1"
						src={`https://www.google.com/s2/favicons?domain=${encodeURIComponent(editDomain)}&sz=64`}
						alt=""
						width="32"
						height="32"
						onerror={() => (editFaviconFailed = true)}
					/>
				{:else}
					<span
						class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-sm font-semibold"
						style="background-color: #eef6fd; color: #2c5ead;"
						>{(editing.name || 'M').trim().charAt(0).toUpperCase()}</span
					>
				{/if}
			{/if}
		{/snippet}

		{#snippet titleExtra()}
			{#if editing}
				{#if editing.status === 'pending_review'}
					<Badge tone="warning" label={tr("🟡 Menunggu Review")} />
				{:else if editing.status === 'rejected'}
					<span title={editing.rejection_reason ?? undefined}
						><Badge tone="danger" label={tr("🔴 Ditolak")} /></span
					>
				{:else}
					<Badge
						tone={editing.status === 'active' ? 'success' : editing.status === 'draft' ? 'warning' : 'neutral'}
						value={statusLabel(editing.status)}
					/>
				{/if}
			{/if}
		{/snippet}

		{#snippet footer()}
			<button type="button" class="app-btn app-btn-soft !min-h-10 !rounded-[10px] !px-5" onclick={closeForm}>{tr("Batal")}</button>
			<button type="submit" form="media-form" class="app-btn app-btn-primary !min-h-10 !rounded-[10px] !px-5" disabled={busy}>
				{busy ? 'Menyimpan…' : editing ? tr('Simpan perubahan') : tr('Tambah media')}
			</button>
		{/snippet}


		{#if error && !isAccessError(error)}
			<p class="mb-3 text-sm" style="color: var(--destructive);">{error}</p>
		{/if}

		{#if editing}
			{#if editing.partner_id}
				{@const partnerName =
					partners.find((p) => p.id === editing?.partner_id)?.name ??
					partners.find((p) => p.id === editing?.partner_id)?.email ??
					'—'}
				<div class="mw-sync mb-4">
					<span class="flex items-center gap-2 whitespace-nowrap">
						<Users size={15} />
						<span class="text-muted-foreground">{tr("Mitra")}</span>
						<strong class="mw-ink">{partnerName}</strong>
					</span>
					<span class="mw-sync-sep" aria-hidden="true"></span>
					<span>{tr("Perubahan di sini tersinkron otomatis ke dashboard mitra, dan sebaliknya.")}</span>
				</div>
			{/if}
			{#if editing.status === 'rejected' && editing.rejection_reason}
				<p class="mb-3 text-xs" style="color: var(--destructive);">Alasan: {editing.rejection_reason}</p>
			{/if}
		{/if}

		<div class={editing ? 'grid items-start gap-5 lg:grid-cols-2' : ''}>
			<!-- ───────────── Kolom kiri: pengaturan media ───────────── -->
			<form id="media-form" class={editing ? 'mw-panel space-y-4' : 'space-y-4'} onsubmit={onSubmit}>
				<section class="space-y-3">
					<h3 class="mw-ptitle">Informasi media</h3>
					<label class="block">
						<span class="mw-label">{tr("Nama media")} <span style="color: var(--destructive);">*</span></span>
						<input class="app-input mt-1 w-full" required placeholder={tr("Contoh: MediaKita")} bind:value={form.name} />
					</label>
					<div class="grid gap-3 sm:grid-cols-2">
						<label class="block">
							<span class="mw-label">Kategori</span>
							<input class="app-input mt-1 w-full" placeholder={tr("Teknologi")} bind:value={form.category} />
						</label>
						<label class="block">
							<span class="mw-label">Website URL</span>
							<input class="app-input mt-1 w-full" placeholder="https://media.com" bind:value={form.website_url} />
						</label>
					</div>
				</section>

				<section class="space-y-3 border-t pt-4">
					<h3 class="mw-ptitle">Harga</h3>
					<div class="grid gap-3 sm:grid-cols-2">
						<label class="block">
							<span class="mw-label">{tr("Harga mitra (IDR)")}</span>
							<input
								class="app-input mt-1 w-full"
								type="text"
								inputmode="numeric"
								value={form.price ? Number(form.price).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' }) : ''}
								oninput={(e) => (form.price = e.currentTarget.value.replace(/\D/g, ''))}
							/>
							<span class="mw-hint"
								>{tr("Base price — nominal yang menjadi hak mitra. Harga klien dihitung otomatis oleh pricing engine (ditambah markup Tauthub).")}</span
							>
						</label>
						<label class="block">
							<span class="mw-label">{tr("Harga klien (IDR)")}</span>
							<input
								class="app-input mt-1 w-full"
								readonly
								value={clientPriceFor(Number(form.price) || 0).toLocaleString('id-ID')}
							/>
							<span class="mw-hint"
								>{tr("Harga mitra + markup")} {markupPercent}{tr("% (pricing engine) — kredit dihitung dari harga klien.")}</span
							>
						</label>
					</div>

					{#if Number(form.price) > 0}
						<div class="mw-breakdown">
							<p class="mw-label mb-2">Price breakdown</p>
							<div class="grid grid-cols-2 gap-y-3 sm:grid-cols-4">
								<div class="mw-bd-cell">
									<span class="mw-bd-k">{tr("Harga Mitra")}</span>
									<span class="mw-bd-v">{idr(Number(form.price) || 0)}</span>
								</div>
								<div class="mw-bd-cell">
									<span class="mw-bd-k">Markup Tauthub</span>
									<span class="mw-bd-v">{markupPercent}%</span>
								</div>
								<div class="mw-bd-cell">
									<span class="mw-bd-k">Nilai Markup</span>
									<span class="mw-bd-v"
										>{idr(Math.max(0, clientPriceFor(Number(form.price) || 0) - (Number(form.price) || 0)))}</span
									>
								</div>
								<div class="mw-bd-cell">
									<span class="mw-bd-k" style="color: var(--primary);">{tr("Harga Klien")}</span>
									<span class="mw-bd-v" style="color: var(--primary);">{idr(clientPriceFor(Number(form.price) || 0))}</span>
								</div>
							</div>
						</div>
					{/if}

					<div>
						<span class="mw-label">{tr("Model Harga")}</span>
						<div class="mw-seg mt-1">
							<button
								type="button"
								class="mw-seg-btn"
								class:mw-seg-on={form.pricing_type === 'nett'}
								onclick={() => (form.pricing_type = 'nett')}>Nett</button
							>
							<button
								type="button"
								class="mw-seg-btn"
								class:mw-seg-on={form.pricing_type === 'nego'}
								onclick={() => (form.pricing_type = 'nego')}>Nego</button
							>
						</div>
						<p class="mw-hint">
							{editing && form.pricing_type !== (editing.pricing_type ?? 'nett')
								? tr('Perubahan model harga pada media yang sudah terbit akan dikonfirmasi dengan catatan ke mitra saat disimpan.')
								: tr('Nett = harga tetap; Nego = harga bisa dinegosiasikan dengan klien.')}
						</p>
					</div>

					<div class="grid gap-3 sm:grid-cols-2">
						<label class="block">
							<span class="mw-label">Biaya kredit (Rp)</span>
							<input class="app-input mt-1 w-full" readonly value={creditForPrice(Number(form.price) || 0).toLocaleString('id-ID')} />
							<span class="mw-hint"
								>{tr("Otomatis dari harga klien ÷ kurs saldo")}{tcRate ? ' (kurs aktif ×' + tcRate.toLocaleString('id-ID') + ')' : ''} — tidak diisi manual;
								server menghitung ulang saat disimpan.</span
							>
						</label>
						<label class="block">
							<span class="mw-label">{tr("Estimasi trafik bulanan (untuk estimated reach)")}</span>
							<input class="app-input mt-1 w-full" type="number" min="0" bind:value={form.traffic_estimate} />
						</label>
					</div>
				</section>

				<section class="space-y-3 border-t pt-4">
					<h3 class="mw-ptitle">Kualitas &amp; SEO</h3>
					<div class="grid gap-3 sm:grid-cols-3">
						<label class="block">
							<span class="mw-label">{tr("Media tier")}</span>
							<Select
								ariaLabel={tr("Media tier")}
								minWidth={140}
								value={form.media_tier}
								onchange={(v) => (form.media_tier = v)}
								options={MEDIA_TIERS.map((t) => ({ value: t, label: t }))}
							/>
						</label>
						<label class="block">
							<span class="mw-label">{tr("Media quality score (0-100)")}</span>
							<input
								class="app-input mt-1 w-full"
								type="number"
								min="0"
								max="100"
								placeholder={tr("kosong = tidak diset")}
								bind:value={form.media_quality_score}
							/>
						</label>
						<label class="block">
							<span class="mw-label">{tr("Domain Authority (DA)")}</span>
							<input
								class="app-input mt-1 w-full"
								type="text"
								inputmode="numeric"
								placeholder={tr("kosong = tidak diset")}
								value={form.seo_da}
								oninput={(e) => {
									const digits = e.currentTarget.value.replace(/\D/g, '').slice(0, 3);
									form.seo_da = digits === '' ? '' : String(Math.min(100, Number(digits)));
								}}
							/>
						</label>
					</div>
					<p class="mw-hint !mt-0"
						>{tr("Isi manual bila deteksi otomatis belum menemukan DA (angka 0-100). Kosongkan untuk menghapus.")}</p
					>
					<label class="block">
						<span class="mw-label">Catatan kualitas (opsional)</span>
						<textarea
							class="app-input mt-1 w-full"
							rows="2"
							placeholder={tr("Contoh: otoritas tinggi di sektor teknologi")}
							bind:value={form.media_quality_notes}
						></textarea>
					</label>
				</section>

				<section class="space-y-3 border-t pt-4">
					<h3 class="mw-ptitle">Ketentuan tautan &amp; integrasi</h3>
					<label class="block">
						<span class="mw-label">{tr("Masa berlaku tautan (backlink)")}</span>
						<Select
							bind:value={form.link_duration_months}
							ariaLabel={tr("Masa berlaku tautan (backlink)")}
							minWidth={220}
							options={LINK_DURATION_OPTIONS.map((o) => ({ value: String(o.value), label: tr(o.label) }))}
						/>
						<span class="mw-hint"
							>{tr("Permanen bila tautan tidak pernah dilepas; atau jumlah bulan jaminan tayang. Nilai ini menentukan masa pemantauan 404 dan masa garansi klaim klien.")}</span
						>
					</label>

					<div>
						<span class="mw-label">{tr('Jenis tautan')}</span>
						<div class="mw-seg mt-1" role="group" aria-label={tr('Jenis tautan')}>
							{#each LINK_KINDS as k (k.value)}
								<button
									type="button"
									class="mw-seg-btn"
									class:mw-seg-on={linkKind === k.value}
									aria-pressed={linkKind === k.value}
									onclick={() => setLinkKind(k.value)}>{k.label}</button
								>
							{/each}
						</div>
						<p class="mw-hint">
							{linkKind === 'dofollow'
								? tr('Dofollow: tautan biasa ke website klien. Google ikut menghitungnya untuk SEO. Artikel boleh memuat tautan sampai batas di bawah.')
								: linkKind === 'sponsored'
									? tr('Sponsored: tautan diberi tanda iklan atau sponsor. Artikel boleh memuat tautan sampai batas di bawah.')
									: tr('Nofollow: tautan ke website klien tetap boleh dipasang, tapi sistem memberinya tanda nofollow otomatis. Google tidak menghitungnya untuk SEO. Artikel tidak ditolak.')}
							{tr('Pengaturan ini sama dengan yang diatur mitra di halaman Media mitra.')}
						</p>
					</div>

					<div class="grid gap-3 sm:grid-cols-3">
						<label class="block">
							<span class="mw-label">{tr('Jumlah backlink maksimal')}</span>
							<Select
								ariaLabel={tr('Jumlah backlink maksimal')}
								minWidth={140}
								value={form.max_links}
								onchange={(v) => (form.max_links = v)}
								options={[
									// 0 hanya muncul untuk media lama yang diatur mitra "tanpa backlink".
									...(form.max_links === '0' ? [{ value: '0', label: tr('0 — Tanpa backlink') }] : []),
									{ value: '1', label: tr('1 Link (default)') },
									{ value: '2', label: tr('2 Link') },
									{ value: '3', label: tr('3 Link') }
								]}
							/>
						</label>
						<label class="block">
							<span class="mw-label">Tipe integrasi</span>
							<Select
								ariaLabel="Tipe integrasi"
								minWidth={140}
								value={form.integration_type}
								onchange={(v) => (form.integration_type = v)}
								options={MEDIA_INTEGRATION_TYPES_UI.map((t) => ({ value: t, label: t }))}
							/>
						</label>
						<label class="block">
							<span class="mw-label">Status</span>
							<Select
								ariaLabel="Status"
								minWidth={140}
								value={form.status}
								onchange={(v) => (form.status = v)}
								options={MEDIA_STATUSES.map((s) => ({ value: s, label: statusLabel(s) }))}
							/>
						</label>
					</div>
				</section>

				{#if !editing}
					{@render catalogSettings()}
					{@render partnerSettings()}
				{/if}

				{#if error && !isAccessError(error)}
					<p class="text-sm" style="color: var(--destructive);">{error}</p>
				{/if}
			</form>

			<!-- ───────────── Kolom kanan: monitoring & administrasi ───────────── -->
			{#if editing}
				<div class="space-y-3.5 lg:row-span-2">
					{@render catalogSettings()}
					{@render partnerSettings()}

					<div class="mw-panel mw-soft">
						<h3 class="mw-ptitle mb-3">{tr("Ringkasan media")}</h3>
						<div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
							<div class="mw-tile">
								<p class="mw-tile-k">Kategori</p>
								<p class="mw-tile-v">{categoryText(editing)}</p>
							</div>
							<div class="mw-tile">
								<p class="mw-tile-k">Trafik</p>
								<div class="flex items-center gap-1.5">
									<p class="mw-tile-v">{compactNumber(editing.traffic_estimate ?? 0)}</p>
									<button
										class="rounded-lg p-1 hover:bg-secondary"
										type="button"
										title="Cek trafik SimilarWeb"
										aria-label={`Cek trafik ${editing.name}`}
										disabled={swBusyId === editing.id || swBulkBusy}
										onclick={() => editing && void checkSimilarWeb(editing)}
									>
										<RefreshCw size={12} />
									</button>
								</div>
								{#if swVisits[editing.id] != null}
									<p class="text-[11px]" style="color: var(--primary);">
										SW: {(swVisits[editing.id] ?? 0).toLocaleString('id-ID')}/bln
									</p>
								{/if}
							</div>
							<div class="mw-tile">
								<p class="mw-tile-k">GEO / AI Trust</p>
								{#if editing.seo_ai_trust != null}
									<Badge tone={scoreTone(editing.seo_ai_trust)} value={`${editing.seo_ai_trust}/100`} />
								{:else}
									<Badge tone="neutral" label="belum diskor" />
								{/if}
							</div>
							<div class="mw-tile">
								<p class="mw-tile-k flex items-center gap-1">DA
									{#if !daEditing}
										<button
											type="button"
											class="text-muted-foreground hover:text-foreground"
											onclick={() => { daInput = editing?.seo_da != null ? String(editing.seo_da) : ''; daEditing = true; }}
											title="Edit DA manual"
										><Pencil size={11} /></button>
									{/if}
								</p>
								{#if daEditing}
									<div class="flex items-center gap-1">
										<input
											type="number"
											min="0"
											max="100"
											class="h-6 w-14 rounded border px-1.5 text-xs"
											bind:value={daInput}
											onkeydown={(e) => { if (e.key === 'Enter') void saveDaManual(); if (e.key === 'Escape') daEditing = false; }}
											onblur={() => void saveDaManual()}
											autofocus
										/>
									</div>
								{:else if editing.seo_da != null}
									<Badge tone={scoreTone(editing.seo_da)} value={`${editing.seo_da}`} />
								{:else}
									<Badge tone="neutral" label="belum ada" />
								{/if}
							</div>
							<div class="mw-tile">
								<p class="mw-tile-k">{tr("Verifikasi domain")}</p>
								{#if editing.domain_verified}
									<p class="flex items-center gap-1.5 text-sm font-semibold" style="color: var(--success);">
										<ShieldCheck size={15} />{tr("Terverifikasi")}
									</p>
									{#if editing.domain_verified_at}
										<p class="text-[11px] text-muted-foreground">{fmtDateTime(editing.domain_verified_at)}</p>
									{/if}
								{:else}
									<p class="mw-tile-v text-muted-foreground">-</p>
									<p class="text-[11px] leading-snug text-muted-foreground">{tr("Meta tag / TXT DNS belum ditemukan")}</p>
								{/if}
							</div>
							<div class="mw-tile">
								<p class="mw-tile-k">Diperiksa</p>
								<p class="mw-tile-v">{editing.seo_checked_at ? fmtDateTime(editing.seo_checked_at) : '-'}</p>
							</div>
						</div>
					</div>

					<div class="mw-panel">
						<div class="mb-3 flex flex-wrap items-baseline justify-between gap-2">
							<h3 class="mw-ptitle">{tr("Health media")}</h3>
							{#if editing.health_computed_at}
								<p class="text-[11px] text-muted-foreground">Dihitung {fmtDateTime(editing.health_computed_at)}</p>
							{/if}
						</div>
						<div class="flex flex-wrap items-stretch gap-x-4 gap-y-3">
							<div class="mw-health-score">
								{#if editing.health_score != null}
									<span class="text-3xl font-bold tabular-nums" style="color: var(--success);">{editing.health_score}</span
									><span class="text-sm text-muted-foreground">/100</span>
								{:else}
									<Badge tone="neutral" label="belum diskor" />
								{/if}
							</div>
							{#if editing.health_breakdown}
								{#each [['Availability', editing.health_breakdown.availability, 15], ['Success rate', editing.health_breakdown.success_rate, 30], ['Publish time', editing.health_breakdown.publish_time, 35], ['Rejection', editing.health_breakdown.rejection_rate, 20]] as [label, value, max] (label)}
									<div class="mw-health-cell">
										<p class="text-[11px] text-muted-foreground">{label}</p>
										<p class="text-base font-bold tabular-nums">{Number(value ?? 0)}/{Number(max)}</p>
									</div>
								{/each}
							{/if}
						</div>
						{#if !editing.health_breakdown}
							<p class="mt-2 text-xs text-muted-foreground">{tr("Belum ada breakdown — klik Hitung health di tabel untuk menghitung skor.")}</p>
						{/if}
					</div>

					<div class="mw-panel">
						<div class="mb-1 flex items-center justify-between gap-2">
							<h3 class="mw-ptitle">{tr("Mode Integrasi")}</h3>
							{#if (editing.publication_mode ?? 'draft') === 'publish'}
								<Badge tone="success" label={tr("Terbit")} />
							{:else}
								<Badge tone="warning" label="Draf" />
							{/if}
						</div>
						<p class="text-xs leading-relaxed text-muted-foreground">
							{(editing.publication_mode ?? 'draft') === 'publish'
								? tr('Artikel dari klien langsung diterbitkan ke media tanpa persetujuan manual Mitra.')
								: tr('Artikel dari klien masuk sebagai draft dan menunggu tindakan Mitra. Dikelola mitra lewat halaman Media mitra.')}
						</p>
					</div>

					<!-- ── Ringkasan operasional: artikel terbit, pelanggaran, suspend ── -->
					<div class="mw-panel">
						<h3 class="mw-ptitle mb-3">{tr('Ringkasan & penindakan')}</h3>
						<div class="mb-3 grid grid-cols-3 gap-2">
							<div class="mw-tile">
								<p class="mw-tile-k">{tr('Total artikel terbit')}</p>
								<p class="text-lg font-bold tabular-nums">{editing.published_count ?? 0}</p>
							</div>
							<div class="mw-tile">
								<p class="mw-tile-k">{tr('Pelanggaran')}</p>
								<p
									class="text-lg font-bold"
									style={(editing.violation_level ?? 'none') === 'berat'
										? 'color: var(--destructive);'
										: (editing.violation_level ?? 'none') === 'ringan'
											? 'color: var(--warning);'
											: 'color: var(--success);'}
								>{(editing.violation_level ?? 'none') === 'berat'
										? tr('Berat')
										: (editing.violation_level ?? 'none') === 'ringan'
											? tr('Ringan')
											: tr('Tidak ada')}</p>
							</div>
							<div class="mw-tile">
								<p class="mw-tile-k">{tr('Status domain')}</p>
								<p class="text-lg font-bold" style={editing.is_suspended ? 'color: var(--destructive);' : 'color: var(--success);'}>
									{editing.is_suspended ? tr('Disuspend') : tr('Aktif')}
								</p>
							</div>
						</div>

						{#if editing.is_suspended}
							<div class="mb-3 rounded-lg px-3 py-2 text-xs" style="background-color: var(--warning-soft, #fff6df); color: var(--warning, #a15c00);">
								<strong>{tr('Domain ditangguhkan')}</strong> — {editing.suspended_reason ?? tr('tanpa alasan tercatat')}.
								{tr('Media tidak tampil di katalog dan publikasi baru ditolak.')}
							</div>
						{/if}

						<div class="flex flex-wrap gap-2">
							<button class="app-btn app-btn-soft !min-h-9 !px-3 !text-xs" type="button" onclick={() => (violationOpen = true)}>
								{tr('Catat pelanggaran')}
							</button>
							{#if (editing.violation_level ?? 'none') !== 'none'}
								<button
									class="app-btn app-btn-soft !min-h-9 !px-3 !text-xs"
									type="button"
									disabled={violationBusy}
									onclick={() => void revokeViolation()}
								>{violationBusy ? tr('Memproses…') : tr('Cabut pelanggaran')}</button>
							{/if}
							{#if editing.is_suspended}
								<button
									class="app-btn app-btn-primary !min-h-9 !px-3 !text-xs"
									type="button"
									disabled={suspendBusy}
									onclick={() => void doUnsuspend()}
								>{suspendBusy ? tr('Memproses…') : tr('Cabut suspend')}</button>
							{:else}
								<button
									class="app-btn !min-h-9 !px-3 !text-xs"
									type="button"
									style="background-color: var(--destructive); color: #fff;"
									onclick={() => (suspendOpen = true)}
								>{tr('Suspend domain')}</button>
							{/if}
						</div>
					</div>

					<!-- Riwayat pelanggaran -->
					<div class="mw-panel">
						<h3 class="mw-ptitle mb-2">{tr('Riwayat pelanggaran')}</h3>
						{#if violationsLoading}
							<p class="text-xs text-muted-foreground">{tr('Memuat…')}</p>
						{:else if violations.length === 0}
							<p class="mw-empty">
								<Info size={14} />{tr('Belum ada pelanggaran tercatat.')}
							</p>
						{:else}
							<div class="max-h-48 space-y-1.5 overflow-y-auto pr-1">
								{#each violations as v (v.id)}
									<div class="rounded-lg border px-2.5 py-1.5 text-xs" style="border-color: var(--border);">
										<div class="flex items-center justify-between gap-2">
											<Badge
												tone={v.level === 'berat' ? 'danger' : 'warning'}
												label={v.level === 'berat' ? tr('Berat') : tr('Ringan')}
											/>
											<span class="text-muted-foreground">{fmtDateTime(v.created_at)}</span>
										</div>
										{#if v.note}<p class="mt-1 leading-snug">{v.note}</p>{/if}
										<p class="mt-0.5 text-[11px] text-muted-foreground">
											{v.actor_email ?? '—'}{#if v.revoked_at} · {tr('dicabut')} {fmtDateTime(v.revoked_at)}{/if}
										</p>
									</div>
								{/each}
							</div>
						{/if}
					</div>

					<div class="mw-panel">
						<div class="mb-1 flex items-center justify-between gap-2">
							<h3 class="mw-ptitle">Verifikasi Dewan Pers</h3>
							{#if editing.dewan_pers_verified}<Badge tone="success" label="Ya" />
							{:else}<Badge tone="neutral" label={tr("Tidak")} />{/if}
						</div>
						{#if editing.dewan_pers_verified}
							<div class="space-y-1">
								{#if editing.dewan_pers_note}<p class="text-xs">{editing.dewan_pers_note}</p>{/if}
								{#if editing.dewan_pers_cert_no}
									<p class="text-xs text-muted-foreground">Sertifikat: {editing.dewan_pers_cert_no}</p>
								{/if}
								{#if editing.dewan_pers_doc_key}
									<a
										class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
										href={`/uploads/${editing.dewan_pers_doc_key}`}
										target="_blank"
										rel="noreferrer">{tr("Lihat dokumen")}</a
									>
								{:else}
									<p class="text-xs text-muted-foreground">Tanpa dokumen bukti.</p>
								{/if}
							</div>
						{:else}
							<p class="text-xs leading-relaxed text-muted-foreground">{tr("Belum diverifikasi. Status dikelola mitra lewat halaman Media mitra (per media/domain).")}</p>
						{/if}
					</div>

				</div>

				<!-- Riwayat audit: kolom kiri, tepat di bawah form. -->
				<div class="mw-panel lg:col-start-1 lg:row-start-2">
					<h3 class="mw-ptitle mb-2">{tr("Riwayat audit")}</h3>
					{#if detailLoading}
						<p class="text-xs text-muted-foreground">{tr("Memuat…")}</p>
					{:else if detailAudit === null}
						<p class="text-xs text-muted-foreground">—</p>
					{:else if detailAudit.length === 0}
						<p class="text-xs text-muted-foreground">{tr("Belum ada aktivitas untuk media ini.")}</p>
					{:else}
						<div class="max-h-80 space-y-1 overflow-y-auto pr-1">
							{#each detailAudit as a (a.id)}
								<div class="mw-audit-row">
									<div class="flex items-center justify-between gap-3">
										<Badge tone={auditTone(a.action)} label={AUDIT_LABEL[a.action] ?? humanizeAuditAction(a.action)} />
										<span class="shrink-0 text-[11px] text-muted-foreground">{fmtDateTime(a.created_at)}</span>
									</div>
									{#if a.detail?.label}
										<p class="mt-1 text-xs leading-snug">
											<span class="font-medium">{a.detail.label}:</span>
											<span class="text-muted-foreground">{a.detail.from || tr('belum diisi')}</span>
											<span style="color: var(--primary);"> → {a.detail.to || tr('belum diisi')}</span>
										</p>
									{/if}
									{#if a.actor_email}
										<p class="mt-0.5 text-[11px] text-muted-foreground">{tr('oleh')} {a.actor_email}</p>
									{/if}
								</div>
							{/each}
						</div>
					{/if}
				</div>
			{/if}
		</div>
	</Modal>
{/if}

{#if violationOpen}
	<Modal title={tr('Catat pelanggaran')} maxWidth="md" onClose={() => (violationOpen = false)}>
		<p class="text-sm text-muted-foreground">
			{tr('Catat pelanggaran untuk')}<strong>{editing?.name}</strong>. {tr('Riwayatnya tersimpan dan bisa dicabut tanpa menghapus catatan.')}
		</p>
		<div class="mt-3">
			<span class="text-xs font-medium text-muted-foreground">{tr('Tingkat')}</span>
			<div class="mt-1 flex gap-2">
				<button
					type="button"
					class="app-btn !min-h-8 !px-3 !text-xs"
					class:app-btn-primary={violationDraft.level === 'ringan'}
					class:app-btn-soft={violationDraft.level !== 'ringan'}
					onclick={() => (violationDraft.level = 'ringan')}>{tr('Ringan')}</button
				>
				<button
					type="button"
					class="app-btn !min-h-8 !px-3 !text-xs"
					class:app-btn-primary={violationDraft.level === 'berat'}
					class:app-btn-soft={violationDraft.level !== 'berat'}
					onclick={() => (violationDraft.level = 'berat')}>{tr('Berat')}</button
				>
			</div>
		</div>
		<label class="mt-3 block">
			<span class="text-xs font-medium text-muted-foreground">{tr('Keterangan')} *</span>
			<textarea
				class="app-input mt-1 w-full"
				rows="3"
				bind:value={violationDraft.note}
				placeholder={tr('Contoh: Menayangkan konten judi online pada 12 Okt 2026.')}
			></textarea>
		</label>
		<div class="mt-3 flex justify-end gap-2">
			<button class="app-btn app-btn-soft" type="button" onclick={() => (violationOpen = false)}>{tr('Batal')}</button>
			<button class="app-btn app-btn-primary" type="button" disabled={violationBusy} onclick={() => void saveViolation()}>
				{violationBusy ? tr('Menyimpan…') : tr('Simpan')}
			</button>
		</div>
	</Modal>
{/if}

{#if suspendOpen}
	<Modal title={tr('Suspend domain')} maxWidth="md" onClose={() => (suspendOpen = false)}>
		<p class="text-sm text-muted-foreground">
			{tr('Media')}<strong>{editing?.name}</strong>{tr('akan disembunyikan dari katalog klien & mitra, dan publikasi baru ditolak. Data, riwayat, dan harga tidak dihapus — penangguhan bisa dicabut kapan saja.')}
		</p>
		<label class="mt-3 block">
			<span class="text-xs font-medium text-muted-foreground">{tr('Alasan penangguhan')} *</span>
			<textarea
				class="app-input mt-1 w-full"
				rows="3"
				bind:value={suspendReason}
				placeholder={tr('Contoh: Domain terindikasi menjual tautan judi. Menunggu klarifikasi mitra.')}
			></textarea>
		</label>
		<div class="mt-3 flex justify-end gap-2">
			<button class="app-btn app-btn-soft" type="button" onclick={() => (suspendOpen = false)}>{tr('Batal')}</button>
			<button
				class="app-btn"
				type="button"
				style="background-color: var(--destructive); color: #fff;"
				disabled={suspendBusy}
				onclick={() => void doSuspend()}
			>{suspendBusy ? tr('Memproses…') : tr('Suspend domain')}</button>
		</div>
	</Modal>
{/if}

{#if priceConfirmOpen}
	<Modal title="Konfirmasi perubahan harga" maxWidth="md" onClose={() => (priceConfirmOpen = false)}>
		<p class="text-sm text-muted-foreground">{tr("Model harga media ini berubah dari")}<strong>{editing?.pricing_type === 'nego' ? 'Nego' : 'Nett'}</strong> menjadi
			<strong>{form.pricing_type === 'nego' ? 'Nego' : 'Nett'}</strong>{tr(". Perubahan berlaku untuk media yang sudah terbit dan akan tersinkron ke dashboard mitra.")}</p>
		<label class="mt-3 block">
			<span class="text-xs font-medium text-muted-foreground">{tr("Catatan untuk mitra (opsional)")}</span>
			<textarea
				class="app-input mt-1 w-full"
				rows="3"
				placeholder={tr("Contoh: Harga dikonfirmasi menjadi nett setelah kesepakatan.")}
				bind:value={priceNote}
			></textarea>
		</label>
		<div class="mt-3 flex justify-end gap-2">
			<button class="app-btn app-btn-soft" onclick={() => (priceConfirmOpen = false)}>{tr("Batal")}</button>
			<button
				class="app-btn app-btn-primary"
				onclick={() => {
					priceConfirmOpen = false;
					void doSave();
				}}>Konfirmasi Perubahan</button
			>
		</div>
	</Modal>
{/if}

{#if reviewTarget}
	<Modal title={tr("Media Ditolak")} maxWidth="md" onClose={() => (reviewTarget = null)}>
		<p class="text-sm text-muted-foreground">{tr("Tolak media")}<strong>{reviewTarget.name}</strong>{tr("? Media tidak dihapus — akan kembali ke mitra dengan alasan ini.")}</p>
		<label class="mt-3 block">
			<span class="text-xs font-medium text-muted-foreground">Alasan Penolakan *</span>
			<textarea
				class="app-input mt-1 w-full"
				rows="4"
				bind:value={rejectReason}
				placeholder={tr("Contoh: Harga yang diajukan belum sesuai dengan kualitas dan traffic media. Silakan lakukan penyesuaian harga dan ajukan kembali.")}
			></textarea>
		</label>
		<div class="mt-3 flex flex-wrap items-end justify-end gap-2">
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">{tr("Harga negosiasi (IDR)")}</span>
				<input
					class="app-input mt-1 w-full"
					type="text"
					inputmode="numeric"
					placeholder="45.000"
					value={negoPrice ? Number(negoPrice).toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' }) : ''}
					oninput={(e) => (negoPrice = e.currentTarget.value.replace(/\D/g, ''))}
				/>
			</label>
			<button
				class="app-btn app-btn-soft"
				style="color: var(--warning);"
				disabled={reviewBusy}
				onclick={negotiateMedia}>{reviewBusy ? 'Menyimpan…' : tr('Negosiasi Harga')}</button
			>
			<button class="app-btn app-btn-soft" disabled={reviewBusy} onclick={() => (reviewTarget = null)}>Batalkan</button>
			<button
				class="app-btn app-btn-primary"
				style="background-color: var(--destructive);"
				disabled={reviewBusy}
				onclick={rejectMedia}>{reviewBusy ? 'Menyimpan…' : 'Tolak Media'}</button
			>
		</div>
	</Modal>
{/if}

{#if bulkLinkOpen}
	<Modal title={`${tr('Ubah jenis tautan')} ${selectedIds.size} media`} maxWidth="md" onClose={() => (bulkLinkOpen = false)}>
		<p class="text-sm text-muted-foreground">
			{tr('Pilih jenis tautan untuk semua media yang kamu centang. Pilihan ini mengubah dua hal yang juga diatur mitra: jumlah backlink (tautan ke website klien) dan label sponsor.')}
		</p>
		<div class="mw-seg mt-3" role="group" aria-label={tr('Jenis tautan')}>
			{#each LINK_KINDS as k (k.value)}
				<button
					type="button"
					class="mw-seg-btn"
					class:mw-seg-on={bulkLinkKind === k.value}
					aria-pressed={bulkLinkKind === k.value}
					onclick={() => (bulkLinkKind = k.value)}>{k.label}</button
				>
			{/each}
		</div>
		<div class="mt-4 flex justify-end gap-3">
			<button class="app-btn app-btn-soft" disabled={bulkBusy} onclick={() => (bulkLinkOpen = false)}>{tr("Batal")}</button>
			<button class="app-btn app-btn-primary" disabled={bulkBusy} onclick={applyBulkLinkKind}>
				{bulkBusy ? 'Menyimpan…' : tr('Terapkan')}
			</button>
		</div>
	</Modal>
{/if}

{#if bulkDeleteOpen}
	<Modal title={`Arsipkan ${selectedIds.size} media?`} maxWidth="md" onClose={() => (bulkDeleteOpen = false)}>
		<p class="text-sm">{tr("Media terpilih akan")}<strong>dipindahkan ke arsip</strong>{tr("(soft delete) — disembunyikan dari portal mitra dan klien. Media bisa dipulihkan kapan saja dari tab Arsip. Tidak ada data yang dihapus permanen.")}</p>
		<div class="mt-4 flex justify-end gap-3">
			<button class="app-btn app-btn-soft" disabled={bulkBusy} onclick={() => (bulkDeleteOpen = false)}>{tr("Batal")}</button>
			<button
				class="app-btn app-btn-primary"
				style="background-color: var(--destructive);"
				disabled={bulkBusy}
				onclick={bulkArchive}>{bulkBusy ? 'Mengarsipkan…' : 'Ya, arsipkan'}</button
			>
		</div>
	</Modal>
{/if}

<style>
	/* Toggle Publik ⇄ Hidden. Satu kontrol bergeser (bukan dua tombol) supaya
	   kolom Status tetap sempit, dengan transisi halus saat berpindah. */
	.visToggle {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		border: 1px solid var(--border);
		border-radius: 999px;
		padding: 3px 9px 3px 4px;
		background-color: var(--muted);
		color: var(--muted-foreground);
		font-size: 11px;
		font-weight: 600;
		line-height: 1;
		cursor: pointer;
		transition:
			background-color 200ms ease,
			border-color 200ms ease,
			color 200ms ease;
	}
	.visToggle:hover:not(:disabled) {
		border-color: var(--primary);
	}
	.visToggle:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
	}
	.visToggle:disabled {
		opacity: 0.6;
		cursor: progress;
	}
	.visToggleOn {
		background-color: var(--success-soft, #eaf8f1);
		border-color: color-mix(in srgb, var(--success, #16794b) 35%, transparent);
		color: var(--success, #16794b);
	}
	.visToggleTrack {
		position: relative;
		display: block;
		width: 26px;
		height: 14px;
		flex: 0 0 26px;
		border-radius: 999px;
		background-color: color-mix(in srgb, var(--foreground) 22%, transparent);
		transition: background-color 200ms ease;
	}
	.visToggleOn .visToggleTrack {
		background-color: var(--success, #16794b);
	}
	.visToggleKnob {
		position: absolute;
		top: 2px;
		left: 2px;
		width: 10px;
		height: 10px;
		border-radius: 999px;
		background-color: #fff;
		box-shadow: 0 1px 2px rgba(0, 0, 0, 0.25);
		transition: transform 220ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.visToggleOn .visToggleKnob {
		transform: translateX(12px);
	}
	.visToggleLabel {
		white-space: nowrap;
	}
	@media (prefers-reduced-motion: reduce) {
		.visToggle,
		.visToggleTrack,
		.visToggleKnob {
			transition: none;
		}
	}

	/* ── Edit media: workspace dua kolom ── */
	.mw-ink {
		color: #14233f;
	}
	.mw-panel {
		border: 1px solid var(--border);
		border-radius: 12px;
		padding: 14px 16px;
		background-color: #fff;
	}
	.mw-soft {
		background-color: #f6faff;
	}
	.mw-ptitle {
		font-size: 15px;
		font-weight: 600;
		line-height: 1.3;
		color: #14233f;
	}
	.mw-label {
		display: block;
		font-size: 12px;
		font-weight: 500;
		color: var(--muted-foreground, #5b6b82);
	}
	.mw-hint {
		display: block;
		margin-top: 4px;
		font-size: 11.5px;
		line-height: 1.4;
		color: var(--muted-foreground, #5b6b82);
	}
	.mw-sync {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 6px 14px;
		padding: 10px 12px;
		font-size: 12.5px;
		color: #2c5ead;
		background-color: #eef6fd;
		border: 1px solid #c4e2f5;
		border-radius: 10px;
	}
	.mw-sync-sep {
		align-self: stretch;
		width: 1px;
		background-color: #c4e2f5;
	}
	@media (max-width: 640px) {
		.mw-sync-sep {
			display: none;
		}
	}
	.mw-breakdown {
		padding: 12px 14px;
		background-color: #f1f8fe;
		border: 1px solid #c4e2f5;
		border-radius: 10px;
	}
	.mw-bd-cell {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 0 12px;
		border-left: 1px solid #c4e2f5;
	}
	.mw-bd-cell:first-child {
		padding-left: 0;
		border-left: 0;
	}
	@media (max-width: 639px) {
		.mw-bd-cell:nth-child(odd) {
			padding-left: 0;
			border-left: 0;
		}
	}
	.mw-bd-k {
		font-size: 11.5px;
		color: var(--muted-foreground, #5b6b82);
	}
	.mw-bd-v {
		font-size: 15px;
		font-weight: 700;
		font-variant-numeric: tabular-nums;
		color: #14233f;
	}
	.mw-seg {
		display: inline-flex;
		padding: 3px;
		gap: 2px;
		background-color: var(--secondary);
		border: 1px solid var(--border);
		border-radius: 10px;
	}
	.mw-seg-btn {
		min-width: 76px;
		padding: 6px 16px;
		font-size: 13px;
		font-weight: 500;
		color: var(--muted-foreground, #5b6b82);
		border-radius: 8px;
		transition:
			background-color 150ms ease,
			color 150ms ease;
	}
	.mw-seg-btn:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 1px;
	}
	.mw-seg-on {
		color: #fff;
		background-color: #2c5ead;
	}
	.mw-switch-row {
		display: flex;
		align-items: flex-start;
		gap: 12px;
		padding: 12px 14px;
		background-color: #f6faff;
		border: 1px solid var(--border);
		border-radius: 10px;
		cursor: pointer;
		font-size: 13.5px;
	}
	.mw-switch {
		appearance: none;
		position: relative;
		flex: 0 0 36px;
		width: 36px;
		height: 20px;
		margin-top: 1px;
		border-radius: 999px;
		background-color: color-mix(in srgb, var(--foreground, #111827) 22%, transparent);
		cursor: pointer;
		transition: background-color 180ms ease;
	}
	.mw-switch::after {
		content: '';
		position: absolute;
		top: 2px;
		left: 2px;
		width: 16px;
		height: 16px;
		border-radius: 999px;
		background-color: #fff;
		box-shadow: 0 1px 2px rgba(0, 0, 0, 0.25);
		transition: transform 180ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.mw-switch:checked {
		background-color: #2c5ead;
	}
	.mw-switch:checked::after {
		transform: translateX(16px);
	}
	.mw-switch:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
	}
	.mw-slug {
		display: flex;
		align-items: stretch;
		margin-top: 4px;
		overflow: hidden;
		border: 1px solid var(--border);
		border-radius: 10px;
		background-color: #fff;
	}
	.mw-slug:focus-within {
		border-color: var(--primary);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 18%, transparent);
	}
	.mw-slug-prefix {
		display: flex;
		align-items: center;
		padding: 0 10px;
		font-size: 12px;
		white-space: nowrap;
		color: var(--muted-foreground, #5b6b82);
		background-color: var(--secondary);
		border-right: 1px solid var(--border);
	}
	.mw-slug-input {
		flex: 1;
		min-width: 0;
		padding: 9px 10px;
		background: transparent;
		outline: none;
	}
	.mw-tile {
		min-width: 0;
		padding: 9px 11px;
		background-color: #fff;
		border: 1px solid #e3ecf7;
		border-radius: 10px;
	}
	.mw-tile-k {
		font-size: 11.5px;
		color: var(--muted-foreground, #5b6b82);
	}
	.mw-tile-v {
		font-size: 15px;
		font-weight: 600;
		color: #14233f;
	}
	.mw-health-score {
		display: flex;
		align-items: baseline;
		gap: 2px;
		align-self: center;
		padding-right: 16px;
		border-right: 1px solid var(--border);
	}
	.mw-health-cell {
		padding-right: 16px;
		border-right: 1px solid var(--border);
	}
	.mw-health-cell:last-child {
		border-right: 0;
	}
	@media (max-width: 480px) {
		.mw-health-score,
		.mw-health-cell {
			border-right: 0;
		}
	}
	.mw-empty {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 10px 12px;
		font-size: 12.5px;
		color: #2c5ead;
		background-color: #eef6fd;
		border-radius: 10px;
	}
	.mw-audit-row {
		padding: 7px 0;
		border-bottom: 1px solid #edf1f7;
	}
	.mw-audit-row:last-child {
		border-bottom: 0;
	}

	/* ── Daftar media di HP ── */
	.mdir-check {
		display: flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 44px;
		width: 44px;
		height: 44px;
		margin: -8px 0 0 -10px;
		cursor: pointer;
	}
	.mdir-check :global(input) {
		width: 20px;
		height: 20px;
	}
	/* Toggle Publik/Hidden di kartu: tampilan ringkas (32px) tapi area sentuh
	   tetap 44px lewat pseudo-element, jadi tidak tampak gemuk. */
	.mdir-card :global(.visToggle) {
		position: relative;
		min-height: 32px;
		gap: 8px;
		padding: 4px 12px 4px 5px;
		font-size: 12px;
		line-height: 1;
	}
	.mdir-card :global(.visToggle)::after {
		content: '';
		position: absolute;
		inset: -6px -4px;
	}
	.mdir-card :global(.visToggleTrack) {
		width: 34px;
		height: 20px;
		flex-basis: 34px;
	}
	.mdir-card :global(.visToggleKnob) {
		width: 16px;
		height: 16px;
	}
	.mdir-card :global(.visToggleOn .visToggleKnob) {
		transform: translateX(14px);
	}
	.mdir-actions :global(.flex-wrap) {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 8px;
	}
	.mdir-actions :global(.app-btn) {
		width: 100%;
		min-height: 44px !important;
		justify-content: center;
		text-align: center;
	}
	.mdir-menu {
		position: absolute;
		top: calc(100% + 4px);
		left: 0;
		z-index: 30;
		min-width: 240px;
		padding: 4px;
		display: flex;
		flex-direction: column;
		background: var(--card, #fff);
		border: 1px solid var(--border, #dfe5ef);
		border-radius: 10px;
		box-shadow: 0 8px 24px rgba(15, 23, 42, 0.12);
	}
	.mdir-menu button {
		text-align: left;
		padding: 8px 12px;
		font-size: 13px;
		border-radius: 6px;
		color: var(--foreground);
	}
	.mdir-menu button:hover:not(:disabled) {
		background: var(--muted, #f1f4f9);
	}
	.mdir-menu button:disabled {
		opacity: 0.5;
	}
	@media (max-width: 767px) {
		.mdir-toolbar :global(.app-btn),
		.mdir-actionbar :global(.app-btn) {
			min-height: 44px !important;
		}
		.mdir-pick {
			min-height: 44px;
		}
		.mdir-pick :global(input) {
			width: 20px;
			height: 20px;
		}
	}
</style>
