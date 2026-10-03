/**
 * Fisher–Yates dengan PRNG mulberry32.
 *
 * Dipakai katalog media untuk merotasi urutan tiap muat ulang halaman: seed
 * diacak sekali saat komponen dibuat, lalu urutan tetap stabil selama seed itu
 * tidak berubah — supaya sortir/paginasi reaktif tidak melompat-lompat.
 */
export function seededShuffle<T>(items: readonly T[], seed: number): T[] {
	const out = items.slice();
	let s = seed >>> 0;
	const next = () => {
		s = (s + 0x6d2b79f5) >>> 0;
		let t = s;
		t = Math.imul(t ^ (t >>> 15), t | 1);
		t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
		return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
	};
	for (let i = out.length - 1; i > 0; i--) {
		const j = Math.floor(next() * (i + 1));
		[out[i], out[j]] = [out[j], out[i]];
	}
	return out;
}

/** Seed acak untuk rotasi urutan (0..2^31-1). */
export function randomShuffleSeed(): number {
	return Math.floor(Math.random() * 2147483648);
}
