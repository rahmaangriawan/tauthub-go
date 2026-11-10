// safeHref: hanya URL http(s) yang boleh jadi tautan. Skema lain
// (javascript:, data:, dll.) menghasilkan undefined sehingga atribut href
// tidak dirender. URL relatif juga ditolak karena nilai ini berasal dari
// input pihak ketiga (published_url, website_url, contoh_artikel_url).
export function safeHref(raw: unknown): string | undefined {
	if (typeof raw !== 'string') return undefined;
	const s = raw.trim();
	if (!s || s.length > 2000) return undefined;
	// eslint-disable-next-line no-control-regex
	if (/[\u0000-\u001f\u007f]/.test(s)) return undefined;
	try {
		const u = new URL(s);
		return u.protocol === 'http:' || u.protocol === 'https:' ? s : undefined;
	} catch {
		return undefined;
	}
}
