// Port src/lib/credits/topup-client.ts (repo lama).
//
// Proteksi double request: saat request gagal di jalan (timeout/network),
// transaksi mungkin sudah terbuat di server — JANGAN membuat intent baru.
// Frontend memeriksa status dengan idempotency key yang sama beberapa kali
// sebelum menyerah (server mengembalikan transaksi yang sama).
import { apiRequest, readApiError } from '$lib/auth';

export type TopUpInvoiceData = { company_name: string; npwp: string; npwp_address: string; po_number?: string };
export type TopUpResult = { payment_url: string | null; amount_idr: number; tc: number; status: string };

export async function submitTopUp(
	apiBase: string,
	tc: number,
	idempotencyKey: string,
	invoice?: TopUpInvoiceData | null
): Promise<TopUpResult> {
	try {
		const res = await apiRequest(`${apiBase}/credits/topup`, {
			method: 'POST',
			body: JSON.stringify(
				invoice
					? { tc, note: JSON.stringify(invoice), idempotency_key: idempotencyKey }
					: { tc, idempotency_key: idempotencyKey }
			)
		});
		if (!res.ok) throw new Error(await readApiError(res, 'Gagal membuat pembayaran.'));
		const data = (await res.json()) as {
			payment: { payment_url: string | null; status: string };
			amount_idr: number;
			tc: number;
		};
		return {
			payment_url: data.payment.payment_url,
			amount_idr: data.amount_idr,
			tc: data.tc,
			status: data.payment.status
		};
	} catch (reason) {
		for (let attempt = 0; attempt < 3; attempt++) {
			await new Promise((resolve) => setTimeout(resolve, 1500));
			try {
				const check = await apiRequest(
					`${apiBase}/credits/topup?idempotency_key=${encodeURIComponent(idempotencyKey)}`
				);
				if (check.ok) {
					const data = (await check.json()) as {
						transaction: {
							payment_url: string | null;
							amount: number;
							purpose_ref_id: string | null;
						} | null;
					};
					if (data.transaction) {
						return {
							payment_url: data.transaction.payment_url,
							amount_idr: data.transaction.amount,
							tc: Number(data.transaction.purpose_ref_id ?? 0),
							status: 'pending'
						};
					}
				}
			} catch {
				// coba lagi sampai batas percobaan
			}
		}
		throw reason;
	}
}