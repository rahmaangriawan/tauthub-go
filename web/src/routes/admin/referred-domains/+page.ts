// Halaman "Referred Domains" lama sudah digabung ke "Monitoring Non VIP"
// (/admin/referral-audit) sebagai tab "Domain Referral" — parity redirect
// app/admin/referred-domains/page.tsx. Redirect ini menjaga bookmark lama.
import { redirect } from '@sveltejs/kit';

export function load() {
	redirect(307, '/admin/referral-audit');
}
