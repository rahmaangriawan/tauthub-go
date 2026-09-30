import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

import { ssrGate } from '$lib/server/gate';

/**
 * SSR role gate — port admin/layout.tsx app lama: sebelum shell admin
 * terkirim, user harus context platform_admins. Non-admin diarahkan ke
 * areanya sendiri (partner → /mitra/{public_id}, lainnya → /klien);
 * sesi tidak valid → /login.
 */
export const load: LayoutServerLoad = async ({ cookies }) => {
	const gate = await ssrGate(cookies.get('refresh_token'));
	if (!gate) redirect(307, '/login');
	if (!gate.admin) {
		redirect(307, gate.partner ? `/mitra/${gate.partner.public_id}` : '/klien');
	}
	return { admin: gate.admin };
};
