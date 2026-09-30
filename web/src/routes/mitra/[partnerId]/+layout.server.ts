import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

import { ssrGate } from '$lib/server/gate';

/**
 * SSR role gate — port mitra/layout.tsx app lama: shell partner hanya untuk
 * partner aktif; non-partner (termasuk admin tanpa partner record) → /klien.
 * Kepemilikan [partnerId] ditegakkan API per-endpoint (parity app lama).
 */
export const load: LayoutServerLoad = async ({ cookies }) => {
	const gate = await ssrGate(cookies.get('refresh_token'));
	if (!gate) redirect(307, '/login');
	if (!gate.partner) redirect(307, '/klien');
	return { partner: gate.partner };
};
