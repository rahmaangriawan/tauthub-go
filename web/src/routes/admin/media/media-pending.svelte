<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Tab Pending /admin/media — media baru menunggu review Admin (pending_review).
	// Port MediaPendingSection di apps/web/src/app/admin/media/_sections.tsx.
	import { Search, MessageSquare } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import DataTable from '$lib/components/admin/data-table.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { fmtDateTime } from '$lib/admin-format';
	import {
		errMessage,
		isAccessError,
		idr,
		mountWithSession,
		platformRequest,
		categoryText,
		type Media,
		type Partner
	} from './_shared';

	const PAGE_SIZE = 20;

	let rows = $state<Media[]>([]);
	let total = $state(0);
	let partners = $state<Partner[]>([]);
	let error = $state('');
	let loading = $state(true);
	let search = $state('');
	let page = $state(1);
	let reviewTarget = $state<Media | null>(null);
	// Media milik mitra tanpa nomor WA: ACC butuh konfirmasi eksplisit.
	let waConfirm = $state<Media | null>(null);
	let rejectReason = $state('');
	let reviewBusy = $state(false);

	// Chat mitra pengaju: admin memulai percakapan lewat tiket dukungan yang
	// sudah ada (POST /v1/platform/tickets). Tiket itu yang muncul di menu
	// Dukungan milik mitra; mitra menerima lonceng + email + WA dari jalur
	// balasan admin yang sama.
	let chatTarget = $state<Media | null>(null);
	let chatSubject = $state('');
	let chatBody = $state('');
	let chatBusy = $state(false);
	let chatError = $state('');
	let chatNotice = $state('');

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
		void debouncedSearch;
		void page;
		void load();
	});

	async function load() {
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams({
				limit: String(PAGE_SIZE),
				offset: String((page - 1) * PAGE_SIZE),
				status: 'pending_review'
			});
			if (debouncedSearch) params.set('search', debouncedSearch);
			const [mediaData, partnerData] = await Promise.all([
				platformRequest<{ media: Media[]; total: number }>(`/media?${params}`),
				platformRequest<{ partners: Partner[] }>('/partners')
			]);
			rows = mediaData.media ?? [];
			total = mediaData.total ?? 0;
			partners = partnerData.partners ?? [];
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			loading = false;
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
			// Badge merah di sidebar & tab "Baru Ditambahkan" ikut turun tanpa
			// menunggu polling 30 detik.
			window.dispatchEvent(new CustomEvent('tauthub:media-pending-refresh'));
			await load();
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
			window.dispatchEvent(new CustomEvent('tauthub:media-pending-refresh'));
			reviewTarget = null;
			rejectReason = '';
			await load();
		} catch (reason) {
			error = errMessage(reason);
		} finally {
			reviewBusy = false;
		}
	}

	/** Info pemilik media untuk verifikasi sebelum Setujui/Tolak: nama diambil dari
	 *  baris API (partner_name) dan email dari daftar mitra. */
	const partnerInfo = (row: Media): { id: string | null; name: string; email: string | null } => {
		const id = row.partner_id ?? null;
		const fromList = id ? partners.find((p) => p.id === id) : undefined;
		return {
			id,
			name: row.partner_name?.trim() || fromList?.name?.trim() || fromList?.email || '—',
			email: fromList?.email ?? null
		};
	};

	/** Tautan ke halaman Mitra — halaman itu membaca ?q= dan memfilter daftarnya. */
	const partnerHref = (info: { name: string; email: string | null }) =>
		`/admin/partners?q=${encodeURIComponent(info.email || info.name)}`;

	/** Buka modal chat untuk mitra pemilik media ini. */
	function openChat(row: Media) {
		chatTarget = row;
		chatSubject = `Media ${row.name}`;
		chatBody = '';
		chatError = '';
		chatNotice = '';
	}

	/**
	 * Kirim pesan pembuka ke mitra lewat tiket dukungan.
	 * Endpoint yang sama dengan tombol "Chat partner" di halaman Mitra, jadi
	 * percakapan langsung muncul di menu Dukungan mitra dan di /admin/dukungan.
	 */
	async function sendChat() {
		const target = chatTarget;
		if (!target) return;
		const info = partnerInfo(target);
		if (!info.id) {
			chatError = tr('Media ini belum terhubung ke mitra, jadi tidak ada yang bisa dihubungi.');
			return;
		}
		const subject = chatSubject.trim();
		const body = chatBody.trim();
		chatError = '';
		chatNotice = '';
		if (!subject) {
			chatError = tr('Subjek wajib diisi.');
			return;
		}
		if (!body) {
			chatError = tr('Pesan wajib diisi.');
			return;
		}
		chatBusy = true;
		try {
			await platformRequest('/tickets', {
				method: 'POST',
				body: JSON.stringify({ partner_id: info.id, subject, body })
			});
			chatTarget = null;
			chatSubject = '';
			chatBody = '';
			chatNotice = tr('Pesan terkirim ke menu Dukungan mitra.');
		} catch (reason) {
			chatError = errMessage(reason);
		} finally {
			chatBusy = false;
		}
	}

	// Daftar perubahan mitra yang menunggu keputusan admin (migrasi 159).
	const pendingChanges = (row: Media) => (Array.isArray(row.pending_changes) ? row.pending_changes : []);
</script>

{#snippet mediaCell(row: Media)}
	<div class="flex items-center gap-2">
		<div>
			<p class="font-medium">{row.name}</p>
			<p class="text-xs text-muted-foreground">{row.slug ?? row.id.slice(0, 8)}</p>
		</div>
	</div>
{/snippet}

{#snippet changesCell(row: Media)}
	{#if pendingChanges(row).length === 0}
		<p class="text-xs text-muted-foreground">Pengajuan baru — tanpa perubahan data.</p>
	{:else}
		<div style="min-width: 200px; max-width: 320px;">
			{#each pendingChanges(row) as c (c.field)}
				<p class="mb-1 text-xs leading-snug">
					<span class="font-medium">{c.label}:</span>
					<span class="text-muted-foreground">{c.from ?? '(kosong)'}</span>
					{' → '}
					<span style="color: var(--primary);">{c.to ?? '(kosong)'}</span>
				</p>
			{/each}
		</div>
	{/if}
{/snippet}

{#snippet websiteCell(row: Media)}
	<p class="max-w-[220px] truncate text-xs text-muted-foreground">{row.website_url ?? '—'}</p>
{/snippet}

{#snippet priceCell(row: Media)}
	<span class="tabular-nums">{idr(row.price ?? 0)}</span>
{/snippet}

{#snippet partnerCell(row: Media)}
	{@const owner = partnerInfo(row)}
	<div>
		{#if owner.id}
			<a
				class="text-sm font-medium underline decoration-dotted"
				style="color: var(--primary);"
				href={partnerHref(owner)}
				title={tr('Buka halaman Mitra untuk memeriksa pengaju ini')}
			>{owner.name}</a>
		{:else}
			<p class="text-sm">{owner.name}</p>
		{/if}
		{#if owner.email && owner.email !== owner.name}
			<p class="break-all text-xs text-muted-foreground">{owner.email}</p>
		{/if}
		{#if row.partner_wa_required}
			<span class="mt-1 inline-block rounded-full px-2 py-0.5 text-[10px] font-semibold" style="background-color: var(--warning-soft); color: var(--warning);">{tr('WA mitra belum diisi')}</span>
		{/if}
		<div class="mt-1">
			{#if row.referred_by_partner_id}
				<Badge
					tone="accent"
					label={`Referral: ${row.referrer_name ?? row.referred_by_partner_id.slice(0, 8)}`}
				/>
			{:else}
				<Badge tone="neutral" label="Non-referral" />
			{/if}
		</div>
	</div>
{/snippet}

{#snippet updatedCell(row: Media)}
	<span class="text-xs text-muted-foreground">{fmtDateTime(row.updated_at ?? row.created_at)}</span>
{/snippet}

{#snippet categoryCell(row: Media)}
	<span class="text-muted-foreground">{categoryText(row)}</span>
{/snippet}

{#snippet statusCell()}
	<Badge tone="warning" label={tr("🟡 Menunggu Review")} />
{/snippet}

{#snippet pendingActions(row: Media)}
	<div class="flex flex-wrap gap-2">
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			style="color: var(--success);"
			disabled={reviewBusy}
			onclick={() => (row.partner_wa_required ? (waConfirm = row) : void approveMedia(row))}>Setujui</button
		>
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			style="color: var(--destructive);"
			disabled={reviewBusy}
			onclick={() => {
				reviewTarget = row;
				rejectReason = '';
				error = '';
			}}>Tolak</button
		>
		<button
			class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
			title={tr('Kirim pesan ke mitra lewat tiket dukungan')}
			disabled={chatBusy || !partnerInfo(row).id}
			onclick={() => openChat(row)}><MessageSquare size={13} /> {tr('Chat')}</button
		>
	</div>
{/snippet}

{#if loading && rows.length === 0 && !error}
	<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat media pending…")}</p>
{:else if rows.length === 0 && isAccessError(error)}
	<div class="app-card mt-6 px-6 py-10 text-center">
		<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke media pending.")}</p>
	</div>
{:else}
	<div class="mt-4 flex flex-wrap items-center gap-2">
		<label class="app-input flex min-w-[240px] flex-1 items-center gap-2 !py-1.5">
			<Search size={15} class="text-muted-foreground" />
			<input
				class="w-full bg-transparent text-sm outline-none"
				placeholder={tr("Cari nama, slug, kategori, atau email mitra…")}
				aria-label={tr("Cari media pending")}
				bind:value={search}
				oninput={() => (page = 1)}
			/>
		</label>
		{#if debouncedSearch}
			<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" onclick={() => (search = '')}
				>Bersihkan pencarian</button
			>
		{/if}
		<div class="flex-1"></div>
		<Badge tone="warning" value={`${total} menunggu review`} />
		<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" onclick={load}>Muat ulang</button>
	</div>

	{#if error && !isAccessError(error)}
		<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>
	{/if}
	{#if chatNotice}
		<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{chatNotice}</div>
	{/if}

	<DataTable
		columns={[
			{ key: 'name', label: tr('Media'), render: mediaCell },
			{ key: 'changes', label: 'Perubahan', render: changesCell },
			{ key: 'category', label: 'Kategori', render: categoryCell },
			{ key: 'website', label: 'Website', render: websiteCell },
			{ key: 'price', label: tr('Harga'), render: priceCell },
			{ key: 'partner', label: tr('Mitra'), render: partnerCell },
			{ key: 'updated', label: 'Diperbarui', render: updatedCell },
			{ key: 'status', label: 'Status', render: statusCell },
			{ key: 'actions', label: '', render: pendingActions }
		]}
		rows={rows}
		rowKey={(row: Media) => row.id}
		{loading}
		empty={tr("Tidak ada media menunggu review.")}
		minWidth="1200px"
	/>

	<Pagination
		total={total}
		limit={PAGE_SIZE}
		offset={(page - 1) * PAGE_SIZE}
		onChange={(next) => (page = next / PAGE_SIZE + 1)}
	/>
{/if}

{#if reviewTarget}
	<Modal title={`Review media — ${reviewTarget.name}`} maxWidth="md" onClose={() => (reviewTarget = null)}>		<p class="text-sm">
			{tr('Mitra pengaju:')}
			<strong>{partnerInfo(reviewTarget).name}</strong>{#if partnerInfo(reviewTarget).email} · <span class="text-muted-foreground">{partnerInfo(reviewTarget).email}</span>{/if}
		</p>
		<p class="mt-1 text-sm text-muted-foreground">{tr("Tolak media ini. Alasan akan dikirim ke mitra. (Negosiasi harga dinonaktifkan — harga ditentukan mitra.)")}</p>
		<label class="mt-3 block">
			<span class="text-xs font-medium text-muted-foreground">Alasan / Catatan</span>
			<textarea
				class="app-input mt-1 w-full"
				rows="4"
				bind:value={rejectReason}
				placeholder={tr("Contoh: Kualitas konten belum memenuhi standar...")}
			></textarea>
		</label>
		<div class="mt-3 flex justify-end gap-2">
			<button class="app-btn app-btn-soft" disabled={reviewBusy} onclick={() => (reviewTarget = null)}>{tr("Batal")}</button>
			<button
				class="app-btn app-btn-primary"
				style="background-color: var(--destructive);"
				disabled={reviewBusy}
				onclick={rejectMedia}>{reviewBusy ? 'Menyimpan…' : 'Tolak Media'}</button
					>
				</div>
			</Modal>
		{/if}

	{#if chatTarget}
		<Modal
			title={`Chat — ${partnerInfo(chatTarget).name}`}
			label={`Chat — ${partnerInfo(chatTarget).name}`}
			maxWidth="md"
			onClose={() => (chatTarget = null)}
		>
			<p class="text-sm text-muted-foreground">
				{tr(
					'Pesan ini masuk ke menu Dukungan mitra sebagai tiket. Mitra menerima notifikasi lonceng, email, dan WhatsApp — balasannya muncul di sana.'
				)}
			</p>
			<p class="mt-2 text-xs text-muted-foreground">
				{tr('Terkait media:')} <strong>{chatTarget.name}</strong>
			</p>
			<div class="mt-3 grid gap-1.5">
				<span class="text-xs font-medium text-muted-foreground">{tr('Subjek')}</span>
				<input
					class="app-input text-sm"
					maxlength="200"
					bind:value={chatSubject}
					placeholder={tr('Mis. Pertanyaan tentang media Anda')}
					aria-label={tr('Subjek pesan')}
				/>
			</div>
			<div class="mt-3 grid gap-1.5">
				<span class="text-xs font-medium text-muted-foreground">{tr('Pesan')}</span>
				<textarea
					class="app-input min-h-[120px] text-sm"
					maxlength="5000"
					bind:value={chatBody}
					placeholder={tr('Tulis pesan untuk mitra…')}
					aria-label={tr('Isi pesan')}
				></textarea>
			</div>
			{#if chatError}
				<div class="mt-3 text-sm" style="color: var(--destructive);">{chatError}</div>
			{/if}
			{#snippet footer()}
				<button class="app-btn app-btn-soft" type="button" disabled={chatBusy} onclick={() => (chatTarget = null)}>{tr('Batal')}</button>
				<button class="app-btn app-btn-primary" type="button" disabled={chatBusy} onclick={() => void sendChat()}>
					{chatBusy ? tr('Mengirim…') : tr('Kirim pesan')}
				</button>
			{/snippet}
		</Modal>
	{/if}

	{#if waConfirm}
		<Modal title={tr('Mitra belum mengisi nomor WhatsApp')} maxWidth="md" onClose={() => (waConfirm = null)}>
			<p class="text-sm">
				<strong>{waConfirm.name}</strong> {tr('milik mitra yang punya lebih dari satu media tetapi belum mengisi nomor WhatsApp. Kalau ada pesanan, mitranya sulit dikonfirmasi.')}
			</p>
			<p class="mt-2 text-sm text-muted-foreground">{tr('Kamu tetap bisa menyetujui — pertimbangkan kirim pengingat dulu dari halaman Mitra.')}</p>
			{#snippet footer()}
				<button class="app-btn app-btn-soft" type="button" onclick={() => (waConfirm = null)}>{tr('Batal')}</button>
				<button
					class="app-btn app-btn-primary"
					type="button"
					onclick={() => { const target = waConfirm; waConfirm = null; if (target) void approveMedia(target); }}
				>{tr('Tetap setujui')}</button>
			{/snippet}
		</Modal>
	{/if}
