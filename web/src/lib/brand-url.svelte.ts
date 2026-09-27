/**
 * URL brand yang dipakai di semua link klien — parity app lama:
 * `brandUrlId ?? brandId` (public_id 6 karakter menang atas UUID).
 * Route tetap menerima keduanya (BrandAccess: `id = $1 OR public_id = $1`),
 * jadi deep-link lama maupun UUID tetap bekerja.
 */
let brandUrlId = $state('');

export function setBrandUrlId(value: string) {
	brandUrlId = value;
}

export function getBrandUrlId(): string {
	return brandUrlId;
}
