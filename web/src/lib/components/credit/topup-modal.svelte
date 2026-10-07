<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Modal Top Up Kredit — port components/credit/topup-modal.tsx.
	// Struktur & perilaku sama: preset paket + nominal lain, opsi faktur PPN 11%,
	// ringkasan tagihan, lalu CTA membuat pembayaran lewat gateway aktif.
	import { X, ExternalLink, CheckCircle, Info, TriangleAlert, ChevronDown } from '@lucide/svelte';
	import type { TopUpInvoiceData } from '$lib/credits/topup-client';

	let {
		rateIdr = 1000,
		packages = [],
		busy = false,
		error = '',
		payment = null,
		onSelectPackage,
		onResetPayment,
		onClose
	}: {
		rateIdr?: number;
		packages?: Array<{ tc: number; idr: number }>;
		busy?: boolean;
		error?: string;
		payment?: { payment_url: string | null; amount_idr: number; tc: number } | null;
		onSelectPackage: (tc: number, invoice?: TopUpInvoiceData | null) => void;
		onResetPayment: () => void;
		onClose?: () => void;
	} = $props();

	// Fallback sama dengan app lama bila admin belum mengisi paket di tc_config.
	const DEFAULT_PACKAGES = [
		{ tc: 10, idr: 10000 },
		{ tc: 100, idr: 100000 },
		{ tc: 1000, idr: 1000000 }
	];
	const effectivePackages = $derived(packages.length > 0 ? packages : DEFAULT_PACKAGES);

	const fmtIDR = (n: number) => `Rp ${n.toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}`;

	// Minimum top up Rp 10.000 — dihitung dari rate aktif (1 Kredit = rateIdr) supaya
	// tetap benar kalau admin mengubah rate.
	const MIN_TOPUP_IDR = 10_000;
	const minTc = $derived(Math.max(1, Math.ceil(MIN_TOPUP_IDR / (rateIdr || 1000))));
	const minLabel = $derived(fmtIDR(minTc * (rateIdr || 1000)));

	let customTcInput = $state('');
	let customError = $state('');
	let selectedPreset = $state<number | null>(null);
	let needInvoice = $state(false);
	let invoice = $state<TopUpInvoiceData>({ company_name: '', npwp: '', npwp_address: '' });
	let invoiceError = $state('');
	let showCustomInput = $state(false);
	let customInputEl = $state<HTMLInputElement | null>(null);

	const parsedCustomTc = $derived.by(() => {
		const raw = customTcInput.replace(/\D/g, '');
		return raw ? parseInt(raw, 10) : 0;
	});
	const effectiveTc = $derived(parsedCustomTc > 0 ? parsedCustomTc : (selectedPreset ?? 0));
	const subtotal = $derived(effectiveTc * rateIdr);
	const ppn = $derived(needInvoice ? Math.round(subtotal * 0.11) : 0);
	const total = $derived(subtotal + ppn);
	const estimatedCustomPrice = $derived(parsedCustomTc * rateIdr);
	const isCustomSelected = $derived(showCustomInput || parsedCustomTc > 0);

	function handleCustomTcChange(event: Event) {
		const rawDigits = (event.currentTarget as HTMLInputElement).value.replace(/\D/g, '');
		if (!rawDigits) {
			customTcInput = '';
			customError = '';
			return;
		}
		const num = parseInt(rawDigits, 10);
		customTcInput = (num > 10_000_000 ? 10_000_000 : num).toLocaleString('id-ID');
		customError = '';
		if (num > 0) selectedPreset = null;
	}

	function handlePresetClick(tc: number) {
		selectedPreset = tc;
		customTcInput = '';
		customError = '';
		showCustomInput = false;
	}

	function handleNominalLain() {
		selectedPreset = null;
		showCustomInput = true;
		window.setTimeout(() => customInputEl?.focus(), 50);
	}

	function validateAndSubmit(tc: number) {
		if (needInvoice) {
			if (!invoice.company_name.trim() || !invoice.npwp.trim() || !invoice.npwp_address.trim()) {
				invoiceError = 'Lengkapi Nama Perusahaan, Nomor NPWP, dan Alamat NPWP.';
				return;
			}
			invoiceError = '';
			onSelectPackage(tc, invoice);
			return;
		}
		onSelectPackage(tc, null);
	}

	function handleCta() {
		if (effectiveTc < minTc) {
			customError = `Minimal ${minLabel} (${minTc} Kredit).`;
			return;
		}
		validateAndSubmit(effectiveTc);
	}

	function onKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && onClose) onClose();
	}
</script>

<svelte:window onkeydown={onKeydown} />

<div class="tu-overlay" role="presentation" onclick={onClose}>
	<dialog
		open
		class="tu-dialog"
		aria-label={tr("Top Up Saldo (Kredit)")}
		onclick={(e) => e.stopPropagation()}
	>
		<div class="tu-header">
			<div class="tu-headline">
				<span class="tu-wallet" aria-hidden="true">
					<svg viewBox="0 0 56 56" width="20" height="20" fill="currentColor" aria-hidden="true">
						<path d="M6.081 16.001h40.97l.174.005A6 6 0 0 1 53 22v24a6 6 0 0 1-6 6H9a6 6 0 0 1-6-6V22l.003-.205l.01-.18A6.01 6.01 0 0 1 6.081 16M37 30a4 4 0 1 0 0 8a4 4 0 0 0 0-8m9.847-21.55a6 6 0 0 1 .205 1.55l-.001 3H14.424l25.075-8.795a6 6 0 0 1 7.348 4.243" />
					</svg>
				</span>
				<div>
					<h3 class="tu-title">{tr("Top Up Saldo (Kredit)")}</h3>
					<p class="tu-subtitle">1 Kredit = Rp {rateIdr.toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })}</p>
				</div>
			</div>
			{#if onClose}
				<button type="button" class="tu-close" onclick={onClose} aria-label={tr("Tutup")}>
					<X size={18} />
				</button>
			{/if}
		</div>

		{#if error}
			<div class="tu-error">{error}</div>
		{/if}

		{#if payment}
			<div class="tu-payment">
				<div>
					<p class="tu-payment-title">
						Pesanan dibuat — {payment.tc.toLocaleString('id-ID')} Kredit · {fmtIDR(payment.amount_idr)}
					</p>
					<p class="tu-payment-meta">{tr("Menunggu pembayaran diverifikasi payment gateway. Kredit masuk otomatis ke saldo setelah pembayaran lunas.")}</p>
				</div>
				<div class="tu-payment-actions">
					{#if payment.payment_url}
						<a class="tu-btn-primary" href={payment.payment_url} target="_blank" rel="noreferrer">
							Bayar Sekarang ({fmtIDR(payment.amount_idr)}) <ExternalLink size={15} />
						</a>
					{/if}
					<button type="button" class="tu-btn-secondary" onclick={onResetPayment}>{tr("Saya sudah bayar — Muat ulang saldo")}</button>
				</div>
			</div>
		{:else}
			<div class="tu-section">
				<span class="tu-label">{tr("Pilih Nominal Kredit")}</span>
				<div class="tu-presets">
					{#each effectivePackages as pkg (pkg.tc)}
						{@const selected = selectedPreset === pkg.tc && !isCustomSelected}
						<button
							type="button"
							class="tu-preset"
							class:tu-preset-selected={selected}
							disabled={busy}
							aria-pressed={selected}
							onclick={() => handlePresetClick(pkg.tc)}
						>
							{#if selected}<span class="tu-preset-check"><CheckCircle size={14} /></span>{/if}
							<img
								src={selected ? '/images/coin-selected.webp' : '/images/coin-unselected.webp'}
								alt=""
								width="18"
								height="18"
								class="tu-coin"
								aria-hidden="true"
							/>
							<span class="tu-preset-tc">{pkg.tc.toLocaleString('id-ID')} {tr("Kredit")}</span>
							<span class="tu-preset-price">{fmtIDR(pkg.idr)}</span>
						</button>
					{/each}
					<button
						type="button"
						class="tu-preset"
						class:tu-preset-selected={isCustomSelected}
						disabled={busy}
						aria-pressed={isCustomSelected}
						onclick={handleNominalLain}
					>
						{#if isCustomSelected}<span class="tu-preset-check"><CheckCircle size={14} /></span>{/if}
						<img
							src={isCustomSelected ? '/images/coin-selected.webp' : '/images/coin-unselected.webp'}
							alt=""
							width="18"
							height="18"
							class="tu-coin"
							aria-hidden="true"
						/>
						<span class="tu-preset-tc">{tr("Nominal Lain")}</span>
						<span class="tu-preset-price">Min. {minTc} {tr("Kredit")}</span>
					</button>
				</div>
			</div>

			{#if showCustomInput}
				<div class="tu-custom">
					<label class="tu-label" for="tu-custom-input">{tr("Atau masukkan nominal lainnya")}</label>
					<div>
						<div class="tu-custom-row">
							<div class="tu-custom-wrap">
								<input
									id="tu-custom-input"
									bind:this={customInputEl}
									type="text"
									inputmode="numeric"
									class="tu-input"
									placeholder={`Minimal ${minTc} ${tr('Kredit')}`}
									value={customTcInput}
									oninput={handleCustomTcChange}
									disabled={busy}
								/>
								<span class="tu-custom-suffix">{tr("Kredit")}</span>
							</div>
						</div>
						<div class="tu-custom-meta">
							{#if parsedCustomTc > 0}
								<span>Estimasi: <strong class="tu-custom-calc">{fmtIDR(estimatedCustomPrice)}</strong></span>
							{:else}
								<span>1 Kredit = Rp{rateIdr.toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })} · Minimal {minLabel}</span>
							{/if}
							{#if parsedCustomTc > 0 && parsedCustomTc < minTc}
								<span class="tu-invalid">Minimal {minLabel} ({minTc} {tr("Kredit")}).</span>
							{/if}
							{#if customError}<span class="tu-invalid">{customError}</span>{/if}
						</div>
					</div>
				</div>
			{/if}

			<div class="tu-invoice" class:tu-invoice-selected={needInvoice}>
				<label class="tu-invoice-header">
					<input
						type="checkbox"
						checked={needInvoice}
						onchange={(e) => (needInvoice = e.currentTarget.checked)}
					/>
					<img src="/images/invoice.webp" alt="" width="32" height="32" class="tu-invoice-icon" aria-hidden="true" />
					<span class="tu-invoice-text">
						<span class="tu-invoice-title">{tr("Saya butuh Faktur Pajak (PPN 11%)")}</span>
						<span class="tu-invoice-desc">{tr("Tagihan akan diterbitkan atas nama PT Tauthub dengan tambahan PPN 11%.")}</span>
					</span>
					<span class="tu-caret" class:tu-caret-open={needInvoice}><ChevronDown size={14} /></span>
				</label>
				{#if needInvoice}
					<div class="tu-invoice-fields">
						<label class="tu-field-label" for="tu-inv-company">{tr("Nama Perusahaan (Sesuai NPWP) *")}</label>
						<input
							id="tu-inv-company"
							class="tu-input"
							placeholder={tr("PT Contoh Sejahtera")}
							bind:value={invoice.company_name}
						/>
						<label class="tu-field-label" for="tu-inv-npwp">Nomor NPWP *</label>
						<input
							id="tu-inv-npwp"
							class="tu-input"
							placeholder="00.000.000.0-000.000"
							inputmode="numeric"
							bind:value={invoice.npwp}
						/>
						<label class="tu-field-label" for="tu-inv-address">Alamat NPWP *</label>
						<textarea
							id="tu-inv-address"
							class="tu-input tu-textarea"
							rows="2"
							placeholder={tr("Jl. Contoh No. 123, Jakarta")}
							bind:value={invoice.npwp_address}
						></textarea>
						{#if invoiceError}<span class="tu-invalid">{invoiceError}</span>{/if}
						<p class="tu-hint">{tr("Data faktur akan diteruskan ke tim Finance untuk penerbitan faktur pajak.")}</p>
					</div>
				{/if}
			</div>

			<div class="tu-billing">
				<div class="tu-billing-left">
					<span class="tu-label">{tr("Ringkasan Tagihan")}</span>
					<div class="tu-row"><span>{tr("Subtotal")}</span><span>{fmtIDR(subtotal)}</span></div>
					<div class="tu-row"><span>PPN 11%</span><span>{fmtIDR(ppn)}</span></div>
					<div class="tu-divider"></div>
					<div class="tu-row tu-row-total"><span>{tr("Total Pembayaran")}</span><span>{fmtIDR(total)}</span></div>
					<p class="tu-hint">1 Kredit = Rp{rateIdr.toLocaleString('id-ID', { maximumFractionDigits: 0, roundingMode: 'trunc' })} · {tr("belum termasuk biaya gateway")}</p>
				</div>
				<img src="/images/billing-summary.webp" alt="" class="tu-billing-image" aria-hidden="true" />
			</div>

			<div class="tu-info">
				<span class="tu-info-icon"><Info size={16} /></span>
				<div>
					<p class="tu-info-title">Punya Bukti Potong PPh 23?</p>
					<p class="tu-info-desc">{tr("Klien korporasi dapat mengklaim bukti potong PPh 23 menjadi tambahan saldo Kredit melalui halaman Invoice setelah pembayaran berhasil.")}</p>
				</div>
			</div>

			<button
				type="button"
				class="tu-cta"
				disabled={busy || effectiveTc < minTc}
				onclick={handleCta}
			>
				{busy ? 'Memproses…' : 'Lanjutkan ke Pembayaran →'}
			</button>

			<div class="tu-disclaimer">
				<span class="tu-disclaimer-icon"><TriangleAlert size={15} /></span>
				<p>
					{tr("Kredit hasil Top Up")} <strong>{tr("tidak dapat di-refund / dikembalikan menjadi uang")}</strong>{tr("— hanya untuk layanan Tauthub.")}</p>
			</div>
		{/if}
	</dialog>
</div>

<style>
	.tu-overlay {
		position: fixed;
		inset: 0;
		z-index: 60;
		display: flex;
		align-items: flex-start;
		justify-content: center;
		overflow-y: auto;
		padding: 24px 16px;
		background-color: rgba(15, 23, 42, 0.45);
	}
	.tu-dialog {
		position: relative;
		display: flex;
		flex-direction: column;
		gap: 18px;
		width: 100%;
		max-width: 640px;
		margin: auto;
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 20px;
		background-color: var(--card);
		color: var(--foreground);
		box-shadow: var(--shadow-md);
	}
	.tu-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px;
	}
	.tu-headline { display: flex; align-items: center; gap: 10px; min-width: 0; }
	.tu-wallet {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 44px;
		height: 44px;
		flex-shrink: 0;
		border-radius: 12px;
		background-color: color-mix(in srgb, var(--primary) 12%, transparent);
		color: var(--primary);
	}
	.tu-title { margin: 0; font-size: 18px; font-weight: 750; letter-spacing: -0.02em; }
	.tu-subtitle { margin: 2px 0 0; font-size: 12.5px; color: var(--muted-foreground); }
	.tu-close {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 30px;
		height: 30px;
		flex-shrink: 0;
		border-radius: 8px;
		color: var(--muted-foreground);
	}
	.tu-close:hover { background-color: var(--secondary); color: var(--foreground); }

	.tu-label {
		display: block;
		font-size: 11px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--muted-foreground);
	}
	.tu-section { display: flex; flex-direction: column; gap: 12px; }

	.tu-presets { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; }
	.tu-preset {
		position: relative;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 4px;
		min-height: 76px;
		padding: 18px 12px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background-color: var(--card);
		text-align: center;
		transition: border-color 0.15s ease, background-color 0.15s ease, box-shadow 0.15s ease;
	}
	.tu-preset:hover:not(:disabled) {
		border-color: color-mix(in srgb, var(--primary) 45%, var(--border));
		background-color: color-mix(in srgb, var(--primary) 5%, var(--card));
	}
	.tu-preset:disabled { opacity: 0.55; cursor: not-allowed; }
	.tu-preset-selected {
		border: 1.5px solid var(--primary) !important;
		background-color: color-mix(in srgb, var(--primary) 8%, var(--card)) !important;
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 18%, transparent);
	}
	.tu-preset-check { position: absolute; top: 7px; right: 7px; color: var(--primary); line-height: 0; }
	.tu-coin { display: block; width: 18px; height: 18px; flex-shrink: 0; }
	.tu-preset-tc { margin-top: 6px; font-size: 13.5px; font-weight: 700; letter-spacing: -0.015em; }
	.tu-preset-price { margin-top: 4px; font-size: 12px; font-weight: 500; color: var(--muted-foreground); font-variant-numeric: tabular-nums; }

	.tu-custom {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 12px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background-color: var(--muted);
	}
	.tu-custom-row { display: flex; align-items: center; gap: 8px; }
	.tu-custom-wrap { position: relative; display: flex; align-items: center; flex: 1; min-width: 0; }
	.tu-input {
		width: 100%;
		padding: 10px 12px;
		border: 1px solid var(--input);
		border-radius: 10px;
		background-color: var(--card);
		color: var(--foreground);
		font-size: 13.5px;
		font-weight: 600;
		outline: none;
	}
	.tu-input:focus { border-color: var(--primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 18%, transparent); }
	.tu-custom-wrap .tu-input { padding-right: 54px; }
	.tu-custom-suffix {
		position: absolute;
		right: 12px;
		font-size: 12px;
		font-weight: 700;
		color: var(--muted-foreground);
		pointer-events: none;
	}
	.tu-custom-meta {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 8px;
		min-height: 16px;
		margin-top: 6px;
		font-size: 11.5px;
		color: var(--muted-foreground);
		flex-wrap: wrap;
	}
	.tu-custom-calc { color: var(--primary); font-variant-numeric: tabular-nums; }
	.tu-invalid { color: var(--destructive); font-size: 11.5px; font-weight: 500; }

	.tu-invoice {
		display: flex;
		flex-direction: column;
		gap: 12px;
		padding: 16px;
		border: 1px solid var(--border);
		border-radius: 12px;
		background-color: var(--card);
		transition: border-color 0.15s ease, box-shadow 0.15s ease;
	}
	.tu-invoice:hover { border-color: color-mix(in srgb, var(--foreground) 22%, var(--border)); }
	.tu-invoice-selected {
		border-color: var(--primary) !important;
		background-color: color-mix(in srgb, var(--primary) 5%, var(--card)) !important;
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 18%, transparent);
	}
	.tu-invoice-header { display: flex; align-items: flex-start; gap: 12px; width: 100%; cursor: pointer; }
	.tu-invoice-header input[type='checkbox'] { margin-top: 4px; width: 16px; height: 16px; accent-color: var(--primary); flex-shrink: 0; }
	.tu-invoice-icon { width: 32px; height: 32px; border-radius: 6px; object-fit: contain; flex-shrink: 0; }
	.tu-invoice-text { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
	.tu-invoice-title { font-size: 13px; font-weight: 650; line-height: 1.35; }
	.tu-invoice-desc { font-size: 12px; color: var(--muted-foreground); line-height: 1.45; }
	.tu-caret { margin-left: auto; align-self: center; flex-shrink: 0; color: var(--muted-foreground); transition: transform 0.2s ease; }
	.tu-caret-open { transform: rotate(180deg); }
	.tu-invoice-fields {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding-top: 12px;
		border-top: 1px solid var(--border);
	}
	.tu-field-label { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: var(--muted-foreground); }
	.tu-textarea { resize: vertical; min-height: 56px; font-weight: 400; }
	.tu-hint { margin: 2px 0 0; font-size: 11.5px; color: var(--muted-foreground); line-height: 1.4; }

	.tu-billing {
		display: flex;
		gap: 16px;
		align-items: stretch;
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 20px;
		background-color: var(--card);
	}
	.tu-billing-left { display: flex; flex: 1; min-width: 0; flex-direction: column; gap: 6px; }
	.tu-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; font-size: 13px; color: var(--muted-foreground); }
	.tu-row span:last-child { font-weight: 600; color: var(--foreground); font-variant-numeric: tabular-nums; }
	.tu-divider { height: 1px; margin: 4px 0; background-color: var(--border); }
	.tu-row-total { font-size: 14px; font-weight: 750; }
	.tu-row-total span:last-child { color: var(--primary); font-size: 15px; }
	.tu-billing-image { width: 112px; max-height: 88px; align-self: center; flex-shrink: 0; object-fit: contain; border-radius: 10px; }

	.tu-info {
		display: flex;
		align-items: flex-start;
		gap: 10px;
		padding: 12px 14px;
		border: 1px solid color-mix(in srgb, var(--primary) 25%, var(--border));
		border-radius: 12px;
		background-color: color-mix(in srgb, var(--primary) 6%, var(--card));
	}
	.tu-info-icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		flex-shrink: 0;
		border: 1px solid color-mix(in srgb, var(--primary) 25%, var(--border));
		border-radius: 8px;
		background-color: var(--card);
		color: var(--primary);
	}
	.tu-info-title { margin: 0 0 2px; font-size: 12.5px; font-weight: 700; }
	.tu-info-desc { margin: 0; font-size: 13px; color: var(--muted-foreground); line-height: 1.5; }

	.tu-btn-primary {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 6px;
		padding: 10px 14px;
		border: 1px solid color-mix(in srgb, var(--primary) 80%, black);
		border-radius: 10px;
		background-color: var(--primary);
		color: var(--primary-foreground);
		font-size: 13px;
		font-weight: 650;
		text-decoration: none;
		transition: background-color 0.15s ease;
	}
	.tu-btn-primary:hover:not(:disabled) { background-color: color-mix(in srgb, var(--primary) 88%, black); }
	.tu-btn-primary:disabled { opacity: 0.45; cursor: not-allowed; }

	.tu-cta {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 100%;
		padding: 12px 16px;
		border: 1px solid color-mix(in srgb, var(--primary) 80%, black);
		border-radius: 12px;
		background-color: var(--primary);
		color: var(--primary-foreground);
		font-size: 14px;
		font-weight: 700;
		box-shadow: 0 1px 2px color-mix(in srgb, var(--primary) 20%, transparent);
		transition: background-color 0.15s ease, transform 0.15s ease, box-shadow 0.15s ease;
	}
	.tu-cta:hover:not(:disabled) {
		background-color: color-mix(in srgb, var(--primary) 88%, black);
		transform: translateY(-1px);
		box-shadow: 0 4px 12px color-mix(in srgb, var(--primary) 28%, transparent);
	}
	.tu-cta:active:not(:disabled) { transform: none; box-shadow: none; }
	.tu-cta:disabled { opacity: 0.45; cursor: not-allowed; transform: none; box-shadow: none; }

	.tu-disclaimer {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		padding: 10px 12px;
		border: 1px solid color-mix(in srgb, var(--warning) 35%, var(--border));
		border-radius: 10px;
		background-color: color-mix(in srgb, var(--warning) 10%, var(--card));
		color: color-mix(in srgb, var(--warning) 85%, black);
		font-size: 12.5px;
		line-height: 1.45;
	}
	.tu-disclaimer p { margin: 0; }
	.tu-disclaimer-icon { flex-shrink: 0; margin-top: 1px; }

	.tu-error {
		border: 1px solid color-mix(in srgb, var(--destructive) 35%, var(--border));
		border-radius: 10px;
		padding: 9px 12px;
		background-color: color-mix(in srgb, var(--destructive) 8%, var(--card));
		color: var(--destructive);
		font-size: 12.5px;
		line-height: 1.4;
	}

	.tu-payment {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 14px 16px;
		border: 1px solid color-mix(in srgb, var(--primary) 30%, var(--border));
		border-radius: 12px;
		background-color: color-mix(in srgb, var(--primary) 6%, var(--card));
	}
	.tu-payment-title { margin: 0; font-size: 14px; font-weight: 700; letter-spacing: -0.01em; }
	.tu-payment-meta { margin: 4px 0 0; font-size: 12px; color: var(--muted-foreground); line-height: 1.45; }
	.tu-payment-actions { display: flex; flex-direction: column; gap: 7px; }
	.tu-btn-secondary {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 9px 12px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background-color: var(--card);
		color: var(--foreground);
		font-size: 12px;
		font-weight: 550;
	}
	.tu-btn-secondary:hover { background-color: var(--secondary); }

	@media (max-width: 560px) {
		.tu-presets { grid-template-columns: repeat(2, minmax(0, 1fr)); }
		.tu-billing { flex-direction: column; }
		.tu-billing-image { width: 96px; align-self: flex-start; }
	}
	@media (max-width: 380px) {
		.tu-custom-row { flex-direction: column; align-items: stretch; }
	}
</style>