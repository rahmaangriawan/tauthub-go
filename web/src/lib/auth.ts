// Port src/lib/auth.ts (repo lama) — kontrak localStorage & event dibuat identik.
// API_BASE kosong = same-origin (dev: proxy Vite; produksi: nginx) — lihat REWRITE/02 §4.

export type UserRole = "client" | "partner" | "admin";

export type AuthUser = {
	id: string;
	email: string;
	display_name: string;
	role?: UserRole;
	created_at: string;
};

export type AuthSession = {
	accessToken: string;
	refreshToken: string;
	expiresAt: number;
	refreshExpiresAt: number;
	sessionExpiresAt: number;
	user: AuthUser;
};

export type TokenResponse = {
	access_token: string;
	refresh_token: string;
	expires_in: number;
	refresh_expires_in: number;
	session_expires_at: string;
	user: AuthUser;
};

const SESSION_KEY = 'ai_writer_session';
const REQUEST_TIMEOUT_MS = 12_000;
const API_BASE = '';

// Cache SWR menyimpan data tenant; tanpa invalidasi, logout → login ulang
// di tab sama bisa menampilkan data akun sebelumnya (parity auth.ts lama).
import { invalidateSwrCache } from '$lib/swr-cache.svelte';

let refreshing: Promise<AuthSession | null> | null = null;

export function getAccessToken() {
	const session = getStoredSession();
	return session?.accessToken ?? null;
}

export function getStoredSession(): AuthSession | null {
	if (typeof window === 'undefined') return null;
	const raw = localStorage.getItem(SESSION_KEY);
	if (!raw) return null;
	try {
		const session = JSON.parse(raw) as Partial<AuthSession> & { sessionExpiresAt?: number };
		if (!session.sessionExpiresAt) {
			expireSession();
			return null;
		}
		if (session.sessionExpiresAt <= Date.now()) {
			expireSession();
			return null;
		}
		if (!session.accessToken || !session.user?.id || !session.refreshToken) {
			clearSession();
			return null;
		}
		return session as AuthSession;
	} catch {
		localStorage.removeItem(SESSION_KEY);
		return null;
	}
}

export function storeSession(result: TokenResponse, currentDeadline?: number): AuthSession {
	const now = Date.now();
	const responseDeadline = Date.parse(result.session_expires_at);
	const sessionExpiresAt =
		currentDeadline == null ? responseDeadline : Math.min(currentDeadline, responseDeadline);
	if (!Number.isFinite(sessionExpiresAt) || sessionExpiresAt <= now) {
		expireSession();
		throw new Error('Session deadline has expired');
	}
	const session: AuthSession = {
		accessToken: result.access_token,
		refreshToken: result.refresh_token,
		expiresAt: Math.min(now + result.expires_in * 1000, sessionExpiresAt),
		refreshExpiresAt: Math.min(now + result.refresh_expires_in * 1000, sessionExpiresAt),
		sessionExpiresAt,
		user: result.user
	};
	localStorage.setItem(SESSION_KEY, JSON.stringify(session));
	return session;
}

export function clearSession() {
	if (typeof window === 'undefined') return;
	const old = (() => {
		try {
			return JSON.parse(localStorage.getItem(SESSION_KEY) ?? 'null') as AuthSession | null;
		} catch {
			return null;
		}
	})();
	localStorage.removeItem(SESSION_KEY);
	// Cache SWR menyimpan data tenant; tanpa ini, logout lalu login ulang di tab yang
	// sama bisa sempat menampilkan data akun sebelumnya.
	invalidateSwrCache();
	// Best-effort server revocation via background fetch (parity).
	if (old?.refreshToken) {
		void fetch(`${API_BASE}/v1/auth/logout`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ refresh_token: old.refreshToken })
		}).catch(() => {});
	}
}

export const USER_UPDATED_EVENT = 'ai-writer:user-updated';
export const TOAST_EVENT = 'ai-writer:toast';
export const AUTH_EXPIRED_EVENT = 'ai-writer:auth-expired';
export type ToastDetail = { type: 'success' | 'error'; message: string };

function expireSession() {
	clearSession();
	if (typeof window !== 'undefined') {
		window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
	}
}

export function updateStoredUser(user: AuthUser) {
	if (typeof window === 'undefined') return;
	const session = getStoredSession();
	if (!session) return;
	session.user = user;
	localStorage.setItem(SESSION_KEY, JSON.stringify(session));
	window.dispatchEvent(new CustomEvent<AuthUser>(USER_UPDATED_EVENT, { detail: user }));
}

async function doRefresh(session: AuthSession): Promise<AuthSession | null> {
	try {
		const res = await fetch(`${API_BASE}/v1/auth/refresh`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ refresh_token: session.refreshToken })
		});
		if (!res.ok) return null;
		const data = (await res.json()) as TokenResponse;
		return storeSession(data, session.sessionExpiresAt);
	} catch {
		return null;
	}
}

export async function ensureValidSession(force = false): Promise<AuthSession | null> {
	const session = getStoredSession();
	if (!session) return null;
	if (session.sessionExpiresAt <= Date.now()) {
		expireSession();
		return null;
	}
	if (!force && session.expiresAt > Date.now() + 30_000) return session;
	if (session.refreshExpiresAt <= Date.now()) {
		expireSession();
		return null;
	}
	if (!refreshing) {
		refreshing = doRefresh(session).finally(() => {
			refreshing = null;
		});
	}
	const refreshed = await refreshing;
	if (!refreshed) expireSession();
	return refreshed;
}

export function showToast(message: string, type: ToastDetail['type'] = 'success') {
	if (typeof window !== 'undefined')
		window.dispatchEvent(new CustomEvent<ToastDetail>(TOAST_EVENT, { detail: { type, message } }));
}

function mutationMessage(method: string) {
	if (method === 'DELETE') return 'Berhasil dihapus.';
	if (method === 'PUT' || method === 'PATCH') return 'Berhasil diperbarui.';
	return 'Berhasil disimpan.';
}

export async function apiRequest(
	path: string,
	init: RequestInit = {},
	timeoutMs = REQUEST_TIMEOUT_MS
): Promise<Response> {
	// Proactive refresh if access is near expiry.
	let session = getStoredSession();
	if (session && session.sessionExpiresAt <= Date.now()) {
		expireSession();
		session = null;
	} else if (session && session.expiresAt <= Date.now() + 30_000 && session.refreshExpiresAt > Date.now()) {
		const refreshed = await ensureValidSession();
		if (refreshed) session = refreshed;
	}

	const method = (init.method ?? 'GET').toUpperCase();
	const shouldToast =
		method !== 'GET' &&
		!path.includes('/auth/') &&
		!path.includes('/notifications/') &&
		!path.includes('/crawl');
	const headers = new Headers(init.headers);
	const token = session?.accessToken ?? null;
	if (token && !headers.has('Authorization')) headers.set('Authorization', `Bearer ${token}`);
	if (init.body && !(init.body instanceof FormData) && !headers.has('Content-Type')) {
		headers.set('Content-Type', 'application/json');
	}

	const controller = new AbortController();
	const timeout = window.setTimeout(() => controller.abort(), timeoutMs);
	const doFetch = () =>
		fetch(`${API_BASE}${path}`, {
			...init,
			headers,
			credentials: 'include',
			signal: init.signal ?? controller.signal
		});

	try {
		let response = await doFetch();
		// On 401, force a token refresh once then retry (parity auth.ts).
		if (response.status === 401 && session?.refreshToken) {
			const refreshed = await ensureValidSession(true);
			if (refreshed) {
				headers.set('Authorization', `Bearer ${refreshed.accessToken}`);
				response = await doFetch();
			} else {
				clearSession();
				if (typeof window !== 'undefined') {
					window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
				}
			}
		} else if (response.status === 401) {
			clearSession();
			if (typeof window !== 'undefined') {
				window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
			}
		}
		if (shouldToast)
			showToast(
				response.ok ? mutationMessage(method) : 'Gagal menyimpan perubahan.',
				response.ok ? 'success' : 'error'
			);
		return response;
	} finally {
		window.clearTimeout(timeout);
	}
}

export async function readApiError(response: Response, fallback: string) {
	try {
		const body = (await response.json()) as { detail?: unknown };
		if (Array.isArray(body.detail)) return body.detail[0]?.msg ?? fallback;
		if (typeof body.detail === 'string') return body.detail;
		if (body.detail && typeof body.detail === 'object') {
			const detail = body.detail as { message?: string; score?: number };
			if (detail.message && typeof detail.score === 'number')
				return `${detail.message}. Readiness saat ini ${detail.score}%.`;
			if (detail.message) return detail.message;
		}
		return fallback;
	} catch {
		return fallback;
	}
}

export function authErrorMessage(reason: unknown) {
	if (reason instanceof DOMException && reason.name === 'AbortError') {
		return 'Server terlalu lama merespons. Periksa layanan API lalu coba lagi.';
	}
	if (reason instanceof TypeError) {
		return 'Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.';
	}
	if (!(reason instanceof Error)) return 'Terjadi kesalahan. Silakan coba lagi.';
	const messages: Record<string, string> = {
		'Invalid email or password': 'Email atau password tidak cocok.',
		'Email already registered': 'Email ini sudah terdaftar. Silakan masuk.',
		'Too many authentication attempts': 'Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi.',
		'Current password is incorrect': 'Password saat ini salah.',
		'String should have at least 10 characters': 'Password baru minimal 10 karakter.'
	};
	return messages[reason.message] ?? reason.message;
}
