// Warna per tier loyalty, dipakai bersama oleh badge klien (account-dropdown)
// dan preview di panel admin. Nama default Bronze/Silver/Gold/Platinum; tier
// custom jatuh ke warna aksen.
export function loyaltyTierColor(name: string | null | undefined): string {
	const n = (name ?? '').toLowerCase();
	if (n.includes('platinum')) return '#7c6fd6';
	if (n.includes('gold')) return '#e4891a';
	if (n.includes('silver')) return '#8795a8';
	if (n.includes('bronze')) return '#b07b4f';
	return 'var(--primary)';
}
