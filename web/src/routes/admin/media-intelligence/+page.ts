// Halaman "Media Intelligence" lama sudah digabung ke /admin/media (tab
// Intelligence) — parity redirect server app/admin/media-intelligence/page.tsx.
// Redirect server (bukan client goto) menjaga bookmark lama tanpa kedip.
import { redirect } from '@sveltejs/kit';

export function load() {
	redirect(307, '/admin/media');
}
