import type { PageServerLoad } from './$types';
import { requireSuperAdmin } from '$lib/server/require-super';

// Halaman khusus Super Admin: admin biasa dialihkan ke /admin.
export const load: PageServerLoad = async ({ parent }) => {
	await requireSuperAdmin(parent);
};
