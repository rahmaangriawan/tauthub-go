import { env } from '$env/dynamic/private';

/**
 * Gate SSR — pemanggil POST /v1/auth/ssr-gate (Go, non-rotasi).
 *
 * Parity gate server-side app lama: layout admin cek role platform_admins,
 * layout mitra cek partners aktif, klien/[brandId]/gate.ts cek partner VIP +
 * membership brand. JWT refresh tidak memuat role dan /v1/auth/refresh
 * merotasi sesi (mencabut token browser), jadi SSR memakai endpoint khusus
 * ini — lihat api/internal/domains/auth/ssr_gate.go.
 *
 * API_URL opsional untuk produksi (container terpisah); default
 * http://localhost:8080 = target proxy Vite dev dan nginx (REWRITE/02 §9).
 */
const API_URL = env.API_URL ?? 'http://localhost:8080';

export interface SSRGate {
	user: { id: string; email: string; display_name?: string | null; role?: string };
	admin: { role: string; permissions: Record<string, boolean> } | null;
	partner: {
		id: string;
		public_id: string;
		is_vip: boolean;
		vip_tier: string;
		vip_expires_at: string | null;
	} | null;
	brand: { id: string; public_id: string; role: string } | null;
}

export async function ssrGate(refreshToken: string | undefined, brandId?: string): Promise<SSRGate | null> {
	if (!refreshToken) return null;
	try {
		const res = await fetch(`${API_URL}/v1/auth/ssr-gate`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ refresh_token: refreshToken, ...(brandId ? { brand_id: brandId } : {}) }),
			signal: AbortSignal.timeout(6_000)
		});
		if (!res.ok) return null;
		return (await res.json()) as SSRGate;
	} catch {
		// API tidak terjangkau: gagalkan gate (caller redirect /login), jangan
		// biarkan halaman terkirim tanpa pemeriksaan role/membership.
		return null;
	}
}
