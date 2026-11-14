<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Invoice & Pajak (read-only; pembuatan invoice dari admin).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { ChevronDown, CircleAlert, Download, LoaderCircle, Receipt } from '@lucide/svelte';
	import { downloadInvoicePdf, formatNpwp } from '$lib/invoice-pdf';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import Select from '$lib/components/select.svelte';
	import PageTitle from '$lib/components/page-title.svelte';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);
	type Invoice = {
		id: string; number: string; status: string; amount: number; campaign_name: string | null;
		issued_at: string | null; due_at: string | null; paid_at: string | null; notes: string | null;
		po_number: string | null;
		bill_to: { company_name?: string; npwp?: string; npwp_address?: string } | null;
	};

	// Profil penagihan: disimpan sekali, dipakai mengisi form top-up berfaktur.
	let profile = $state({ company_name: '', npwp: '', npwp_address: '', billing_email: '' });
	let profileVisible = $state(true);
	let profileSaving = $state(false);
	let profileError = $state('');
	let profileNotice = $state('');
	let pdfBusy = $state<string | null>(null);

	// Galat per kolom, dengan aturan yang sama seperti server
	// (api/internal/domains/brandflow/billing_profile.go) supaya tidak ada isian
	// yang lolos ke server lalu baru ditolak: nama perusahaan 2–200 karakter,
	// alamat 5–500 karakter, NPWP 15 atau 16 digit, email opsional tapi harus
	// berbentuk email. Pesannya pun sama, jadi pengguna melihat satu kalimat
	// yang sama di mana pun galatnya muncul.
	type FieldName = 'company_name' | 'npwp' | 'npwp_address' | 'billing_email';
	const NPWP_DROP_RE = /[^\d.\-\s]/g;
	const EMAIL_RE = /^[^@\s]+@[^@\s]+\.[^@\s]+$/;

	let fieldErrors = $state<Partial<Record<FieldName, string>>>({});
	let companyEl = $state<HTMLInputElement | null>(null);
	let npwpEl = $state<HTMLInputElement | null>(null);
	let addressEl = $state<HTMLTextAreaElement | null>(null);
	let emailEl = $state<HTMLInputElement | null>(null);

	const runeLen = (v: string) => [...v].length;
	const npwpDigits = (v: string) => v.replace(/\D/g, '');

	function validateProfile(): Partial<Record<FieldName, string>> {
		const errors: Partial<Record<FieldName, string>> = {};
		const company = profile.company_name.trim();
		const address = profile.npwp_address.trim();
		const digits = npwpDigits(profile.npwp);
		const email = profile.billing_email.trim();

		if (runeLen(company) < 2 || runeLen(company) > 200)
			errors.company_name = tr('Nama perusahaan wajib diisi (2 sampai 200 karakter).');
		if (digits.length !== 15 && digits.length !== 16)
			errors.npwp = tr('NPWP harus 15 atau 16 digit (titik dan tanda hubung boleh dipakai).');
		if (runeLen(address) < 5 || runeLen(address) > 500)
			errors.npwp_address = tr('Alamat sesuai NPWP wajib diisi (5 sampai 500 karakter).');
		if (email !== '' && (email.length > 254 || !EMAIL_RE.test(email)))
			errors.billing_email = tr('Email tagihan tidak valid.');

		return errors;
	}

	function showFieldError(name: FieldName) {
		fieldErrors = { ...fieldErrors, [name]: validateProfile()[name] ?? '' };
	}

	function setField(name: FieldName, value: string) {
		profile[name] = value;
		// Galat yang sudah tampil hilang begitu kolomnya benar.
		if (fieldErrors[name]) showFieldError(name);
	}

	function onFieldBlur(name: FieldName) {
		if (name === 'npwp') profile.npwp = formatNpwp(profile.npwp);
		else profile[name] = profile[name].trim();
		showFieldError(name);
	}

	function focusFirstInvalid(errors: Partial<Record<FieldName, string>>) {
		const order: [FieldName, HTMLElement | null][] = [
			['company_name', companyEl],
			['npwp', npwpEl],
			['npwp_address', addressEl],
			['billing_email', emailEl]
		];
		const first = order.find(([name]) => errors[name])?.[1];
		if (!first) return;
		first.focus();
		first.scrollIntoView({ block: 'nearest' });
	}

	async function loadProfile() {
		const res = await apiRequest(`/v1/brands/${brandId}/billing-profile`);
		if (res.status === 403) {
			profileVisible = false;
			return;
		}
		if (!res.ok) return;
		const j = await res.json();
		if (j.profile) {
			profile = {
				company_name: j.profile.company_name ?? '', npwp: formatNpwp(j.profile.npwp),
				npwp_address: j.profile.npwp_address ?? '', billing_email: j.profile.billing_email ?? ''
			};
		}
	}

	async function saveProfile() {
		profileError = '';
		profileNotice = '';
		// Gerbang terakhir sebelum request: satu pun kolom tidak valid, tidak ada
		// yang dikirim, dan fokus pindah ke kolom pertama yang salah.
		const errors = validateProfile();
		fieldErrors = errors;
		if (Object.keys(errors).length > 0) {
			focusFirstInvalid(errors);
			return;
		}
		profileSaving = true;
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/billing-profile`, {
				method: 'PUT',
				body: JSON.stringify({
					company_name: profile.company_name.trim(),
					npwp: npwpDigits(profile.npwp),
					npwp_address: profile.npwp_address.trim(),
					billing_email: profile.billing_email.trim()
				})
			});
			if (!res.ok) {
				profileError = await readApiError(res, tr('Gagal menyimpan data penagihan.'));
				return;
			}
			fieldErrors = {};
			profileNotice = tr('Data penagihan disimpan.');
			await loadProfile();
		} finally {
			profileSaving = false;
		}
	}

	async function pdf(inv: Invoice) {
		pdfBusy = inv.id;
		try {
			await downloadInvoicePdf(inv);
		} finally {
			pdfBusy = null;
		}
	}

	let invoices = $state<Invoice[]>([]);
	let total = $state(0);
	let status = $state('');
	let loading = $state(true);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([load(), loadProfile()]);
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		const params = new URLSearchParams({ limit: '100' });
		if (status) params.set('status', status);
		const res = await apiRequest(`/v1/brands/${brandId}/invoices?${params}`);
		if (res.ok) {
			const data = await res.json();
			invoices = data.invoices ?? [];
			total = data.total ?? 0;
		}
	}

	const statusLabel: Record<string, string> = $derived({ draft: 'Draf', issued: tr('Diterbitkan'), paid: tr('Lunas'), void: tr('Dibatalkan') });
	const statusTone = (s: string) => (s === 'paid' ? 'good' : s === 'issued' ? 'warn' : 'muted');
	const fmt = (n: number) => n.toLocaleString('id-ID', { maximumFractionDigits: 0 });
	const fmtDate = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' }) : '—');
</script>

<PageTitle title="Invoice & Pajak" />

<section class="iv">
	<header class="iv-head">
		<div>
			<h1 class="page-title">Invoice & Pajak</h1>
			<p class="page-desc">{total} {tr("invoice tercatat untuk brand ini.")}</p>
		</div>
		<Select
			bind:value={status}
			ariaLabel="Status invoice"
			placeholder={tr("Semua status")}
			align="end"
			onchange={() => load()}
			options={[
				{ value: '', label: tr('Semua status') },
				{ value: 'draft', label: 'Draf' },
				{ value: 'issued', label: tr('Diterbitkan') },
				{ value: 'paid', label: tr('Lunas') },
				{ value: 'void', label: tr('Dibatalkan') }
			]}
		/>
	</header>

	{#if profileVisible}
		<!-- Data penagihan: akordion. Tertutup secara bawaan; ringkasannya
		     menunjukkan apakah datanya sudah terisi. -->
		<details class="app-card iv-panel">
			<summary class="iv-summary">
				<h2 class="iv-h2">{tr('Data penagihan')}</h2>
				<span class="iv-summary-state">{profile.company_name || tr('Belum diisi')}</span>
				<ChevronDown size={16} strokeWidth={1.5} class="iv-chevron" />
			</summary>

			<div class="iv-body">
				<p class="iv-desc">{tr('Disimpan sekali dan mengisi otomatis form top up berfaktur. NPWP hanya terlihat oleh anggota yang boleh mengisi saldo.')}</p>

				<div class="iv-fields">
					<div class="iv-field">
						<label for="iv-company">{tr('Nama perusahaan (sesuai NPWP)')} <span class="iv-req" aria-hidden="true">*</span></label>
						<input
							id="iv-company"
							class="app-input"
							bind:this={companyEl}
							value={profile.company_name}
							maxlength="200"
							autocomplete="organization"
							aria-invalid={fieldErrors.company_name ? 'true' : undefined}
							aria-describedby={fieldErrors.company_name ? 'iv-err-company' : undefined}
							oninput={(e) => setField('company_name', e.currentTarget.value)}
							onblur={() => onFieldBlur('company_name')}
						/>
						{#if fieldErrors.company_name}
							<p class="iv-field-error" id="iv-err-company">
								<CircleAlert size={13} strokeWidth={1.5} />
								{fieldErrors.company_name}
							</p>
						{/if}
					</div>

					<div class="iv-field">
						<label for="iv-npwp">{tr('Nomor NPWP')} <span class="iv-req" aria-hidden="true">*</span></label>
						<input
							id="iv-npwp"
							class="app-input"
							bind:this={npwpEl}
							value={profile.npwp}
							inputmode="numeric"
							maxlength="24"
							placeholder="00.000.000.0-000.000"
							aria-invalid={fieldErrors.npwp ? 'true' : undefined}
							aria-describedby={fieldErrors.npwp ? 'iv-err-npwp' : undefined}
							oninput={(e) => {
								// Hanya angka, titik, tanda hubung, dan spasi yang masuk ke
								// kolom ini; sisa ketikan dibuang di depan pengguna.
								const el = e.currentTarget;
								const next = el.value.replace(NPWP_DROP_RE, '');
								if (next !== el.value) el.value = next;
								setField('npwp', next);
							}}
							onblur={() => onFieldBlur('npwp')}
						/>
						{#if fieldErrors.npwp}
							<p class="iv-field-error" id="iv-err-npwp">
								<CircleAlert size={13} strokeWidth={1.5} />
								{fieldErrors.npwp}
							</p>
						{/if}
					</div>

					<div class="iv-field iv-wide">
						<label for="iv-address">{tr('Alamat sesuai NPWP')} <span class="iv-req" aria-hidden="true">*</span></label>
						<textarea
							id="iv-address"
							class="app-input"
							bind:this={addressEl}
							value={profile.npwp_address}
							rows="2"
							maxlength="500"
							aria-invalid={fieldErrors.npwp_address ? 'true' : undefined}
							aria-describedby={fieldErrors.npwp_address ? 'iv-err-address' : undefined}
							oninput={(e) => setField('npwp_address', e.currentTarget.value)}
							onblur={() => onFieldBlur('npwp_address')}
						></textarea>
						{#if fieldErrors.npwp_address}
							<p class="iv-field-error" id="iv-err-address">
								<CircleAlert size={13} strokeWidth={1.5} />
								{fieldErrors.npwp_address}
							</p>
						{/if}
					</div>

					<div class="iv-field iv-wide">
						<label for="iv-email">{tr('Email tagihan (opsional)')}</label>
						<input
							id="iv-email"
							class="app-input"
							bind:this={emailEl}
							value={profile.billing_email}
							type="email"
							maxlength="254"
							autocomplete="email"
							aria-invalid={fieldErrors.billing_email ? 'true' : undefined}
							aria-describedby={fieldErrors.billing_email ? 'iv-err-email' : undefined}
							oninput={(e) => setField('billing_email', e.currentTarget.value)}
							onblur={() => onFieldBlur('billing_email')}
						/>
						{#if fieldErrors.billing_email}
							<p class="iv-field-error" id="iv-err-email">
								<CircleAlert size={13} strokeWidth={1.5} />
								{fieldErrors.billing_email}
							</p>
						{/if}
					</div>
				</div>

				{#if profileError}<p class="iv-msg iv-msg-bad" role="alert">{profileError}</p>{/if}
				{#if profileNotice}<p class="iv-msg iv-msg-good">{profileNotice}</p>{/if}

				<button class="app-btn app-btn-primary iv-submit" type="button" disabled={profileSaving} onclick={() => void saveProfile()}>
					{#if profileSaving}<LoaderCircle size={15} strokeWidth={1.5} class="animate-spin" />{/if}
					{tr('Simpan data penagihan')}
				</button>
			</div>
		</details>
	{/if}

	{#if loading}
		<p class="iv-loading">{tr('Memuat invoice...')}</p>
	{:else if invoices.length === 0}
		<div class="app-card iv-empty">
			<span class="iv-empty-icon"><Receipt size={20} strokeWidth={1.5} /></span>
			<p class="iv-empty-title">{status ? tr('Tidak ada invoice dengan status ini') : tr('Belum ada invoice')}</p>
			<p class="iv-empty-desc">{tr("Invoice diterbitkan admin setelah kampanye atau pembelian paket.")}</p>
		</div>
	{:else}
		<div class="app-card iv-card">
			<div class="iv-cols iv-headrow" aria-hidden="true">
				<span>{tr('Nomor Invoice')}</span>
				<span>{tr('Kampanye')}</span>
				<span>{tr('Status')}</span>
				<span class="iv-right">{tr('Jumlah')}</span>
				<span class="iv-opt-po">{tr('Nomor PO')}</span>
				<span class="iv-opt-due">{tr('Jatuh tempo')}</span>
				<span></span>
			</div>

			<ul class="iv-list">
				{#each invoices as inv (inv.id)}
					<li class="iv-cols iv-row">
						<span class="iv-number">{inv.number}</span>
						<span class="iv-campaign">{inv.campaign_name ?? '—'}</span>
						<span class="iv-state" class:is-good={statusTone(inv.status) === 'good'} class:is-warn={statusTone(inv.status) === 'warn'}>
							<span class="iv-dot" class:is-good={statusTone(inv.status) === 'good'} class:is-warn={statusTone(inv.status) === 'warn'}></span>
							{statusLabel[inv.status] ?? inv.status}
						</span>
						<span class="iv-amount">Rp {fmt(inv.amount)}</span>
						<span class="iv-muted iv-opt-po">{inv.po_number ?? '—'}</span>
						<span class="iv-muted iv-opt-due">{fmtDate(inv.due_at)}</span>
						<span class="iv-action">
							<button class="app-btn app-btn-soft iv-pdf" type="button" disabled={pdfBusy === inv.id} onclick={() => void pdf(inv)} title={`PDF ${inv.number}`}>
								{#if pdfBusy === inv.id}
									<LoaderCircle size={14} strokeWidth={1.5} class="animate-spin" />
								{:else}
									<Download size={14} strokeWidth={1.5} />
								{/if}
								PDF
							</button>
						</span>
					</li>
				{/each}
			</ul>
		</div>
	{/if}
</section>

<style>
	.iv {
		display: flex;
		flex-direction: column;
		gap: 24px;
	}

	/* ── Kepala halaman ── */
	.iv-head {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		justify-content: space-between;
		gap: 16px;
	}

	/* ── Data penagihan: akordion ── */
	.iv-panel {
		overflow: hidden;
	}
	.iv-summary {
		display: flex;
		align-items: baseline;
		gap: 10px;
		padding: 16px 20px;
		cursor: pointer;
		list-style: none;
		transition: background-color 120ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.iv-summary::-webkit-details-marker {
		display: none;
	}
	.iv-summary:hover {
		background-color: color-mix(in srgb, var(--secondary) 55%, transparent);
	}
	.iv-summary:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: -2px;
	}
	.iv-summary-state {
		flex: 1 1 auto;
		overflow: hidden;
		font-size: 12.5px;
		line-height: 18px;
		color: var(--muted-foreground);
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	:global(.iv-chevron) {
		flex: 0 0 auto;
		align-self: center;
		margin-left: auto;
		color: var(--muted-foreground);
		transition: transform 160ms cubic-bezier(0.22, 1, 0.36, 1);
	}
	.iv-panel[open] :global(.iv-chevron) {
		transform: rotate(180deg);
	}
	.iv-body {
		padding: 4px 20px 20px;
		border-top: 1px solid var(--border);
	}
	.iv-h2 {
		font-size: 15px;
		line-height: 20px;
		font-weight: 500;
		color: var(--foreground);
	}
	.iv-desc {
		margin-top: 4px;
		font-size: 13px;
		line-height: 19px;
		color: var(--muted-foreground);
		max-width: 72ch;
	}
	.iv-fields {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 16px;
		margin-top: 16px;
	}
	.iv-field {
		display: flex;
		flex-direction: column;
		gap: 6px;
		font-size: 13px;
		line-height: 18px;
		font-weight: 500;
		color: var(--foreground);
	}
	.iv-field label {
		cursor: text;
	}
	.iv-field :global(.app-input) {
		font-weight: 400;
		min-height: 40px;
	}
	.iv-field :global(.app-input[aria-invalid='true']) {
		border-color: var(--destructive);
	}
	.iv-field :global(.app-input[aria-invalid='true']:focus-visible) {
		border-color: var(--destructive);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--destructive) 15%, transparent);
	}
	.iv-req {
		color: var(--destructive);
	}
	.iv-field-error {
		display: flex;
		align-items: center;
		gap: 5px;
		font-size: 12.5px;
		line-height: 16px;
		font-weight: 400;
		color: var(--destructive);
	}
	.iv-field-error :global(svg) {
		flex: 0 0 auto;
	}
	.iv-wide {
		grid-column: 1 / -1;
	}
	.iv-msg {
		margin-top: 12px;
		font-size: 13px;
		line-height: 18px;
	}
	.iv-msg-bad {
		color: var(--destructive);
	}
	.iv-msg-good {
		color: var(--success);
	}
	.iv-submit {
		align-self: flex-start;
		margin-top: 20px;
		min-height: 40px;
	}

	/* ── Daftar invoice ── */
	.iv-loading {
		font-size: 13px;
		color: var(--muted-foreground);
	}
	.iv-card {
		display: flex;
		flex-direction: column;
		overflow: hidden;
	}
	.iv-cols {
		display: grid;
		grid-template-columns: 132px minmax(0, 1fr) 116px 128px 104px 120px 88px;
		align-items: center;
		gap: 16px;
	}
	.iv-headrow {
		padding: 10px 20px;
		border-bottom: 1px solid var(--border);
		font-size: 12px;
		line-height: 16px;
		color: var(--muted-foreground);
	}
	.iv-right {
		text-align: right;
	}
	.iv-list {
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.iv-list li + li {
		border-top: 1px solid var(--border);
	}
	.iv-row {
		padding: 12px 20px;
	}
	.iv-number {
		font-size: 13.5px;
		font-weight: 500;
		color: var(--foreground);
		font-variant-numeric: tabular-nums;
		word-break: break-word;
	}
	.iv-campaign {
		font-size: 13px;
		line-height: 18px;
		color: var(--muted-foreground);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.iv-state {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		color: var(--muted-foreground);
	}
	.iv-dot {
		flex: 0 0 6px;
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background-color: var(--muted-foreground);
	}
	.iv-dot.is-good,
	.iv-state.is-good .iv-dot.is-good {
		background-color: var(--success);
	}
	.iv-dot.is-warn {
		background-color: var(--warning);
	}
	.iv-state.is-good {
		color: var(--success);
	}
	.iv-state.is-warn {
		color: var(--warning);
	}
	.iv-amount {
		font-size: 13.5px;
		text-align: right;
		color: var(--foreground);
		font-variant-numeric: tabular-nums;
	}
	.iv-muted {
		font-size: 13px;
		color: var(--muted-foreground);
	}
	.iv-action {
		display: flex;
		justify-content: flex-end;
	}
	.iv-pdf {
		min-height: 32px;
		padding: 0 10px;
		font-size: 12.5px;
		font-weight: 500;
	}

	/* ── Kosong ── */
	.iv-empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 4px;
		padding: 40px 24px;
		text-align: center;
	}
	.iv-empty-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 44px;
		height: 44px;
		margin-bottom: 10px;
		border-radius: 12px;
		background-color: var(--secondary);
		color: var(--muted-foreground);
	}
	.iv-empty-title {
		font-size: 14px;
		line-height: 20px;
		font-weight: 500;
		color: var(--foreground);
	}
	.iv-empty-desc {
		font-size: 13px;
		line-height: 18px;
		color: var(--muted-foreground);
	}

	/* ── Layar sempit: kolom opsional dilepas, baris tetap satu garis ── */
	@media (max-width: 1180px) {
		.iv-cols {
			grid-template-columns: 120px minmax(0, 1fr) 112px 120px 88px;
		}
		.iv-opt-po {
			display: none;
		}
	}
	@media (max-width: 900px) {
		.iv-fields {
			grid-template-columns: minmax(0, 1fr);
		}
		.iv-cols {
			grid-template-columns: minmax(0, 1fr) 108px 88px;
		}
		.iv-opt-due,
		.iv-campaign {
			display: none;
		}
		.iv-amount {
			display: block;
		}
	}
	@media (max-width: 560px) {
		.iv-headrow {
			display: none;
		}
		.iv-cols {
			grid-template-columns: minmax(0, 1fr) 88px;
			gap: 8px 12px;
		}
		.iv-state,
		.iv-amount {
			grid-column: 1;
		}
		.iv-action {
			grid-row: 1 / span 3;
			grid-column: 2;
			align-items: start;
		}
		.iv-amount {
			font-size: 13px;
		}
	}
</style>
