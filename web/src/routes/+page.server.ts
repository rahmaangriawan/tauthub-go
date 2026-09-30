import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

/**
 * Frontpage tanpa landing page: pengunjung langsung diarahkan ke layar MASUK
 * (tab default "Masuk"). Alur pendaftaran tetap punya URL sendiri:
 * /register-role, /register/klien, /register/mitra, dan /login?mode=register.
 * Redirect di server supaya tetap bekerja walau JavaScript mati.
 */
export const load: PageServerLoad = () => {
	throw redirect(307, '/login');
};
