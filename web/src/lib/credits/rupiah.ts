// Konversi kredit ↔ rupiah untuk seluruh tampilan klien.
//
// Kurs aktif datang dari API (`tc_config.rate_idr`, dikirim di payload saldo /
// biaya). Fallback Rp 1.000 sama dengan modal top up supaya layar tidak pernah
// menampilkan angka yang salah saat kurs belum terbaca.
export const DEFAULT_TC_RATE_IDR = 1000;

export function tcRate(rate?: number | null): number {
	const value = Number(rate);
	return Number.isFinite(value) && value > 0 ? value : DEFAULT_TC_RATE_IDR;
}

const rpFmt = new Intl.NumberFormat('id-ID', {
	style: 'currency',
	currency: 'IDR',
	maximumFractionDigits: 0,
	roundingMode: 'trunc'
});

/** Format rupiah bertanda "Rp". */
export const rp = (amount: number) => rpFmt.format(Math.round(amount || 0));

/** Kredit → rupiah memakai kurs aktif. */
export const tcToRp = (tc: number, rate?: number | null) => rp((tc || 0) * tcRate(rate));

/**
 * Rupiah → kredit. Dibulatkan ke ATAS supaya nominal yang diketik klien selalu
 * tercukupi oleh saldo kredit (tidak pernah menagih kurang dari yang tertulis).
 */
export const rpToTc = (amountIdr: number, rate?: number | null) =>
	Math.ceil((amountIdr || 0) / tcRate(rate));
