import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

import { ssrGate } from '$lib/server/gate';

/**
 * SSR brand gate — port klien/[brandId]/gate.ts (assertBrandAccess):
 * - Partner hanya boleh masuk bila VIP aktif bertier brand_ambassador
 *   (VIP Reguler murni fee affiliate → /mitra); admin boleh masuk.
 * - Brand unknown / bukan anggota → /klien (index mendaratkan user ke
 *   brand miliknya).
 */
export const load: LayoutServerLoad = async ({ cookies, params }) => {
	const gate = await ssrGate(cookies.get('refresh_token'), params.brandId);
	if (!gate) redirect(307, '/login');

	if (gate.partner) {
		const p = gate.partner;
		const vipActive =
			p.is_vip === true &&
			(p.vip_tier || 'regular') === 'brand_ambassador' &&
			(!p.vip_expires_at || Date.parse(p.vip_expires_at) > Date.now());
		if (!vipActive) redirect(307, '/mitra');
	}

	if (!gate.brand) redirect(307, '/klien');
	return { brand: gate.brand };
};
