// Bantuan untuk menyalin isi log/laporan sebagai Markdown, supaya mudah
// ditempel ke chat, tiket, atau catatan debug.

/** Salin teks ke clipboard. Balikan false bila diblokir browser. */
export async function copyText(text: string): Promise<boolean> {
	try {
		await navigator.clipboard.writeText(text);
		return true;
	} catch {
		// Cadangan untuk konteks tanpa Clipboard API (mis. http biasa, iframe).
		try {
			const el = document.createElement('textarea');
			el.value = text;
			el.setAttribute('readonly', '');
			el.style.position = 'fixed';
			el.style.opacity = '0';
			document.body.appendChild(el);
			el.select();
			const ok = document.execCommand('copy');
			document.body.removeChild(el);
			return ok;
		} catch {
			return false;
		}
	}
}

/** Kosongkan nilai null/undefined dan rapikan jadi satu baris aman untuk sel tabel. */
export function mdCell(value: unknown): string {
	if (value === null || value === undefined || value === '') return '-';
	return String(value).replace(/\r?\n+/g, ' ').replace(/\|/g, '\\|').trim() || '-';
}

/** Tabel Markdown dari header dan baris. Tanpa baris: "_Tidak ada data._". */
export function mdTable(headers: string[], rows: unknown[][]): string {
	if (rows.length === 0) return '_Tidak ada data._';
	const head = `| ${headers.map(mdCell).join(' | ')} |`;
	const sep = `| ${headers.map(() => '---').join(' | ')} |`;
	const body = rows.map((r) => `| ${r.map(mdCell).join(' | ')} |`);
	return [head, sep, ...body].join('\n');
}

/** Blok kode berpagar; pagar diperpanjang bila isinya memuat ```. */
export function mdCode(text: string, lang = ''): string {
	let fence = '```';
	while (text.includes(fence)) fence += '`';
	return `${fence}${lang}\n${text}\n${fence}`;
}

/** Daftar "- **label:** nilai" untuk pasangan kunci-nilai. */
export function mdList(pairs: [string, unknown][]): string {
	return pairs.map(([k, v]) => `- **${k}:** ${v === null || v === undefined || v === '' ? '-' : String(v).replace(/\r?\n+/g, ' ')}`).join('\n');
}

/** Judul laporan standar: judul, waktu salin, dan alamat halaman. */
export function mdReport(title: string, ...sections: string[]): string {
	const stamp = new Date().toLocaleString('id-ID', { timeZone: 'Asia/Jakarta', hour12: false });
	const url = typeof location !== 'undefined' ? location.href : '';
	return [`# ${title}`, `_Disalin ${stamp} WIB${url ? ` dari ${url}` : ''}_`, ...sections.filter(Boolean)].join('\n\n') + '\n';
}
