import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

import { ssrGate } from '$lib/server/gate';
import { isVvipTier } from '$lib/partner/vip-tier';

/**
 * Gerbang halaman VIP Dashboard: HANYA untuk VVIP (tier brand_ambassador).
 *
 * VIP Reguler dan mitra reguler dialihkan ke ringkasan mitra. Gerbang ini
 * ditaruh di server (bukan hanya menyembunyikan menu sidebar) supaya tidak bisa
 * dilewati dengan mengetik URL atau mematikan JavaScript. Sumber kebenarannya
 * tier di partners.vip_tier, sama seperti menu sidebar, jadi menu dan akses
 * tidak pernah berbeda.
 */
export const load: PageServerLoad = async ({ cookies, params }) => {
	const gate = await ssrGate(cookies.get('refresh_token'));
	if (!gate) redirect(307, '/login');
	if (!gate.partner) redirect(307, '/klien');
	if (!isVvipTier(gate.partner.vip_tier)) redirect(307, `/mitra/${params.partnerId}`);
	return { partner: gate.partner };
};
