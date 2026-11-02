import { redirect } from '@sveltejs/kit';

/**
 * Penjaga route khusus Super Admin. Menu untuk halaman ini memang hanya tampil
 * bagi Super Admin, tapi URL-nya tetap bisa dibuka langsung, jadi route harus
 * memeriksa peran sendiri. Admin biasa dialihkan ke beranda admin.
 *
 * Dipakai di `+page.server.ts` route terkait: `await requireSuperAdmin(parent)`.
 * Data `admin` datang dari `admin/+layout.server.ts` (ssrGate).
 *
 * Ini hanya soal kenyamanan; penegakan sesungguhnya ada di backend lewat
 * middleware `SuperAdminOnly`, yang menolak API-nya dengan 403.
 */
export async function requireSuperAdmin(
	parent: () => Promise<{ admin: { role: string } | null }>
): Promise<void> {
	const { admin } = await parent();
	if (admin?.role !== 'super_admin') redirect(307, '/admin');
}
