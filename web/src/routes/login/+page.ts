// Halaman /login membaca ?mode=register di klien (tanpa reload) dan meneruskan
// peran terkunci bila datang dari /register/klien atau /register/mitra.
import type { PageLoad } from './$types';

export const ssr = false;

export const load: PageLoad = ({ url }) => {
	return {
		initialMode: url.searchParams.get('mode') === 'register' ? 'register' : 'login',
		lockedRole: null
	};
};
