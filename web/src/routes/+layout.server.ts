// Locale pilihan pengguna paling awal: server hanya bisa membacanya dari
// cookie (localStorage tertutup untuk SSR). Nilai ini dipakai layout akar
// untuk menyetel bahasa SEBELUM render, sehingga halaman pertama sudah
// berbahasa yang benar — tidak ada kilatan Bahasa Indonesia pada user EN.
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = ({ cookies }) => {
	const stored = cookies.get('tauthub-locale');
	return { locale: stored === 'en' ? 'en' : 'id' };
};
