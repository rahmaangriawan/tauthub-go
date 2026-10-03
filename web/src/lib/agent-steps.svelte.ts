/**
 * Status pill langkah Taut Pintar yang dirender ke utility bar atas.
 *
 * App lama memakai `createPortal` ke `[class*="utilityBar"] > div:first-child`
 * (lihat agent-steps.tsx) — pill muncul di sisi KIRI bar atas, sebaris dengan
 * tombol hamburger, bukan di badan halaman. SvelteKit tidak punya portal, jadi
 * halaman menulis state ini lewat `setAgentSteps` dan shell klien merendernya
 * di slot `toolbarLeft` (posisi & perilaku tetap parity).
 *
 * `token` mencegah balapan saat pindah halaman: halaman lama yang unmount
 * setelah halaman baru mount tidak boleh menghapus state milik halaman baru.
 */

export type AgentStepState = {
	current: 1 | 2 | 3 | 4;
	tone?: 'waiting' | 'published';
	extra?: 'ringkasan';
	lockedSteps?: number[];
	lockedReason?: string;
};

import { apiRequest } from '$lib/auth';

let state = $state<AgentStepState | null>(null);
let owner: symbol | null = null;

export function setAgentSteps(next: AgentStepState, token: symbol) {
	state = next;
	owner = token;
}

export function clearAgentSteps(token: symbol) {
	if (owner !== token) return;
	state = null;
	owner = null;
}

export function getAgentSteps(): AgentStepState | null {
	return state;
}

/**
 * Langkah terjauh yang sudah dicapai sebuah kampanye selama sesi berjalan.
 *
 * Pil langkah hanya mengaktifkan `n <= langkah saat ini`, sehingga kembali ke
 * langkah sebelumnya (mis. dari The Writer ke The Profiler) membuat pil langkah
 * berikutnya mati dan pengguna tidak bisa maju lagi. Nilai terjauh ini dipakai
 * sebagai batas atas supaya langkah yang sudah pernah dicapai tetap bisa dibuka.
 *
 * Disimpan per kampanye agar kampanye lain tidak ikut terbuka. Sifatnya sesi:
 * setelah muat ulang halaman, batasnya kembali ke langkah halaman yang dibuka.
 */
let furthest = $state<{ campaignId: string; step: number }>({ campaignId: '', step: 0 });

export function noteReachedStep(campaignId: string, step: number) {
	if (!campaignId || step <= 0) return;
	if (furthest.campaignId !== campaignId) {
		furthest = { campaignId, step };
		return;
	}
	if (step > furthest.step) furthest = { campaignId, step };
}

export function furthestStep(campaignId: string): number {
	return furthest.campaignId === campaignId ? furthest.step : 0;
}

/**
 * Langkah yang tersirat dari `taut_status` kampanye.
 *
 * Sebelumnya pil "Ringkasan" hanya muncul di halaman Ringkasan itu sendiri,
 * sehingga setelah pengguna bisa maju kembali dari langkah sebelumnya, tidak ada
 * jalan pulang ke Ringkasan dari halaman lain. Status kampanye dipakai untuk
 * mengetahui tahap terjauh yang benar-benar sudah ditempuh (termasuk setelah
 * halaman dimuat ulang, karena datanya dari server).
 */
const STEP_BY_STATUS: Record<string, number> = {
	draft: 1,
	profiling: 1,
	profiled: 2,
	materials: 2,
	budget: 2,
	placement: 3,
	production: 3,
	review: 3,
	submitted: 4,
	scheduled: 4,
	distributed: 4
};

let statusSteps = $state<Record<string, number>>({});
const statusFetching: Record<string, boolean> = {};

/** Ambil status kampanye sekali per kampanye, lalu simpan langkahnya. */
export async function ensureCampaignStep(brandId: string, campaignId: string) {
	if (!brandId || !campaignId) return;
	if (statusSteps[campaignId] !== undefined || statusFetching[campaignId]) return;
	statusFetching[campaignId] = true;
	try {
		const res = await apiRequest(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);
		if (!res.ok) return;
		const d = await res.json().catch(() => null);
		const status = String(d?.taut_status ?? '');
		statusSteps = { ...statusSteps, [campaignId]: STEP_BY_STATUS[status] ?? 0 };
	} catch {
		/* tanpa data status, pil tetap bisa dipakai untuk langkah yang sudah dilewati */
	} finally {
		statusFetching[campaignId] = false;
	}
}

export function statusStep(campaignId: string): number {
	return statusSteps[campaignId] ?? 0;
}
