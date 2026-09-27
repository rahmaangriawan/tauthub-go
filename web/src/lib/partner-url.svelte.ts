/**
 * URL mitra yang dipakai di semua link portal mitra — parity app lama:
 * `partnerUrlId ?? partnerId` (public_id 6 karakter menang atas UUID).
 * Route tetap menerima keduanya (partner.me mengembalikan public_id), jadi
 * deep-link lama maupun UUID tetap bekerja.
 */
let partnerUrlId = $state('');

export function setPartnerUrlId(value: string) {
	partnerUrlId = value;
}

export function getPartnerUrlId(): string {
	return partnerUrlId;
}
