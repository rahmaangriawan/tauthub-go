/**
 * Cakupan domain media: root vs subdomain.
 *
 * Dipakai katalog klien dan modal detail untuk menandai apakah artikel akan
 * tayang di halaman utama domain atau di subdomain (audiens bisa berbeda).
 *
 * Suffix dua-label (co.id, or.id, dll.) dihitung satu kesatuan, supaya
 * "conference.co.id" benar terbaca ROOT, bukan subdomain dari "co.id".
 */

const MULTI_LABEL_SUFFIX = new Set([
	'co.id', 'or.id', 'web.id', 'my.id', 'biz.id', 'sch.id', 'ac.id', 'go.id',
	'net.id', 'mil.id', 'desa.id', 'co.uk', 'com.au', 'co.nz', 'com.br',
	'co.jp', 'co.kr', 'com.tr', 'co.in', 'co.za'
]);

export type DomainScope = 'root' | 'subdomain';

export function domainScope(website: string | null | undefined): DomainScope | null {
	const host = (website ?? '')
		.replace(/^https?:\/\//i, '')
		.replace(/\/.*$/, '')
		.replace(/^www\./i, '')
		.toLowerCase()
		.trim();
	if (!host) return null;
	const parts = host.split('.');
	if (parts.length < 2) return null;
	const twoLabel = parts.slice(-2).join('.');
	const root = MULTI_LABEL_SUFFIX.has(twoLabel)
		? (parts.length >= 3 ? parts.slice(-3).join('.') : host)
		: twoLabel;
	return host === root ? 'root' : 'subdomain';
}
