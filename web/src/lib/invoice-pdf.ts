/**
 * Invoice PDF (A4) dari baris invoice klien. Isinya hanya yang tercatat:
 * nomor, tanggal, status, data penagihan (nama perusahaan, NPWP, alamat, PO),
 * deskripsi, dan jumlah. Tidak ada data penjual atau rincian PPN yang dikarang:
 * faktur pajak resmi tetap diterbitkan tim Finance.
 */
export type InvoicePdfInput = {
	number: string;
	status: string;
	amount: number;
	notes: string | null;
	issued_at: string | null;
	paid_at: string | null;
	po_number: string | null;
	bill_to: { company_name?: string; npwp?: string; npwp_address?: string } | null;
};

const STATUS_LABEL: Record<string, string> = { draft: 'Draf', issued: 'Diterbitkan', paid: 'Lunas', void: 'Dibatalkan' };

/** 15 digit: 00.000.000.0-000.000. 16 digit: dikelompokkan empat-empat. */
export function formatNpwp(raw: string | undefined): string {
	const d = (raw ?? '').replace(/\D/g, '');
	if (d.length === 15) return `${d.slice(0, 2)}.${d.slice(2, 5)}.${d.slice(5, 8)}.${d.slice(8, 9)}-${d.slice(9, 12)}.${d.slice(12)}`;
	if (d.length === 16) return d.replace(/(\d{4})(?=\d)/g, '$1 ');
	return raw ?? '';
}

const rp = (n: number) => `Rp ${Math.round(n || 0).toLocaleString('id-ID')}`;
const date = (v: string | null) => (v ? new Date(v).toLocaleDateString('id-ID', { dateStyle: 'long' }) : '-');

export async function downloadInvoicePdf(inv: InvoicePdfInput): Promise<void> {
	const { jsPDF } = await import('jspdf');
	const doc = new jsPDF({ unit: 'pt', format: 'a4' });
	const W = doc.internal.pageSize.getWidth();
	const M = 48;
	let y = 56;

	doc.setFont('helvetica', 'bold');
	doc.setFontSize(20);
	doc.text('Invoice', M, y);
	doc.setFontSize(11);
	doc.setFont('helvetica', 'normal');
	doc.setTextColor(90);
	doc.text('Tauthub', W - M, y, { align: 'right' });
	y += 28;

	doc.setTextColor(20);
	const meta: [string, string][] = [
		['Nomor', inv.number],
		['Tanggal terbit', date(inv.issued_at)],
		['Status', STATUS_LABEL[inv.status] ?? inv.status]
	];
	if (inv.paid_at) meta.push(['Dibayar', date(inv.paid_at)]);
	if (inv.po_number) meta.push(['Nomor PO', inv.po_number]);
	for (const [k, v] of meta) {
		doc.setTextColor(110);
		doc.text(k, M, y);
		doc.setTextColor(20);
		doc.text(v, M + 110, y);
		y += 18;
	}

	y += 14;
	doc.setFont('helvetica', 'bold');
	doc.text('Ditagihkan kepada', M, y);
	doc.setFont('helvetica', 'normal');
	y += 18;
	if (inv.bill_to?.company_name) {
		doc.text(inv.bill_to.company_name, M, y);
		y += 16;
		if (inv.bill_to.npwp) {
			doc.setTextColor(110);
			doc.text(`NPWP ${formatNpwp(inv.bill_to.npwp)}`, M, y);
			y += 16;
		}
		if (inv.bill_to.npwp_address) {
			doc.setTextColor(110);
			const lines = doc.splitTextToSize(inv.bill_to.npwp_address, W - M * 2) as string[];
			doc.text(lines, M, y);
			y += lines.length * 14;
		}
		doc.setTextColor(20);
	} else {
		doc.setTextColor(110);
		doc.text('Data penagihan tidak dicantumkan.', M, y);
		doc.setTextColor(20);
		y += 16;
	}

	y += 24;
	doc.setDrawColor(210);
	doc.line(M, y, W - M, y);
	y += 20;
	doc.setFont('helvetica', 'bold');
	doc.text('Deskripsi', M, y);
	doc.text('Jumlah', W - M, y, { align: 'right' });
	y += 10;
	doc.line(M, y, W - M, y);
	y += 20;
	doc.setFont('helvetica', 'normal');
	const desc = doc.splitTextToSize(inv.notes || 'Pembelian saldo Tauthub', W - M * 2 - 120) as string[];
	doc.text(desc, M, y);
	doc.text(rp(inv.amount), W - M, y, { align: 'right' });
	y += Math.max(desc.length, 1) * 14 + 14;
	doc.line(M, y, W - M, y);
	y += 22;
	doc.setFont('helvetica', 'bold');
	doc.setFontSize(13);
	doc.text('Total', M, y);
	doc.text(rp(inv.amount), W - M, y, { align: 'right' });

	doc.setFont('helvetica', 'normal');
	doc.setFontSize(9);
	doc.setTextColor(120);
	doc.text('Dokumen ini dibuat otomatis oleh Tauthub. Faktur pajak resmi diterbitkan tim Finance.', M, doc.internal.pageSize.getHeight() - 40);

	doc.save(`${inv.number}.pdf`);
}
