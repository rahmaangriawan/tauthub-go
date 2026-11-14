<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Konfigurasi platform — port app/admin/konfigurasi/page.tsx (+ crawlhub-tab, server-tab).
	// Tab: SEO Metrics (RapidAPI + SimilarWeb + SERPens), API Pagespeed, Email/SMTP,
	// Turnstile CF, Google Sign-In, Short Link, Crawlhub, Server.
	// Semua secret tidak pernah dikembalikan server — hanya flag "sudah dikonfigurasi".
	import { onMount } from 'svelte';
	import { tick } from 'svelte';
	import { CheckCircle2, Gauge, Globe, KeyRound, Mail, RefreshCw, ShieldCheck, Sparkles, TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';
	// Panel integrasi yang dipindah ke tab: file yang sama dipakai route
	// /admin/integrasi-gsc, /admin/ga4-configs, dan /admin/wa-gateway.
	import GscPanel from '../integrasi-gsc/gsc-panel.svelte';
	import Ga4Panel from '../ga4-configs/ga4-panel.svelte';
	import WaGatewayPanel from '../wa-gateway/wa-gateway-panel.svelte';
	import EmailTemplatesPanel from './email-templates-panel.svelte';
	import DomainBlocksPanel from './domain-blocks-panel.svelte';
	import EmailLogPanel from './email-log-panel.svelte';

	type SeoConfig = {
		api_key_configured: boolean;
		api_key_hint?: string | null;
		host?: string | null;
		provider_label?: string | null;
		provider_url?: string | null;
		last_da_error?: string | null;
		last_da_error_at?: string | null;
		similarweb_api_key_configured?: boolean;
		similarweb_provider_url?: string | null;
		similarweb_host?: string | null;
		serpens_api_key_configured?: boolean;
	};
	type SmtpConfig = {
		smtp_configured: boolean;
		host: string | null;
		port: number;
		secure: boolean;
		username: string | null;
		password_configured: boolean;
		from_email: string;
		from_name: string;
		enabled: boolean;
		last_test_at: string | null;
		last_test_ok: boolean | null;
		last_test_message: string | null;
	};
	type TurnstileConfig = {
		enabled: boolean;
		site_key: string | null;
		secret_configured: boolean;
		updated_at: string | null;
	};

	type ApiEndpoint = { method: string; path: string; body?: string; purpose: string };
	type CrawlhubConfig = {
		base_url: string;
		hmac_secret_configured: boolean;
		ingest_secret_configured: boolean;
		configured: boolean;
		api: {
			signature: { headers: string[]; format: string; maxSkewSeconds: number };
			outbound: ApiEndpoint[];
			inbound: ApiEndpoint[];
		};
	};
	type CrawlTestResult = {
		status: string;
		articles7d: number | null;
		confidence: string;
		source: string | null;
		errorMessage: string | null;
		durationMs?: number;
		methodDetail?: { requests?: number; statuses?: number[]; notes?: string[]; blockedReason?: string };
	};
	type CrawlTarget = {
		media_id: string;
		website_url: string;
		last_status: string | null;
		last_crawled_at: string | null;
		next_run_at: string;
		consecutive_failures: number;
		last_run: {
			articles_7d: number | null;
			confidence: string | null;
			source: string | null;
			blocked_reason: string | null;
			error_message: string | null;
		} | null;
	};
	type CrawlRow = {
		media_id: string;
		name: string;
		website_url: string;
		articles_7d: number | null;
		confidence: string | null;
		source: string | null;
		last_crawled_at: string | null;
		last_status: string | null;
		consecutive_failures: number;
		last_error: string | null;
		registered: boolean;
		stale?: boolean;
	};
	type MonitorData = {
		summary: { total: number; registered: number; failed: number; unregistered: number; pending: number };
		targets: CrawlRow[];
		total: number;
		offset: number;
	};

	const CRAWL_STATUS_LABEL: Record<string, string> = {
		ok: 'BISA DIHITUNG',
		blocked: 'DIBLOKIR',
		no_source: 'TIDAK ADA SUMBER',
		error: 'ERROR'
	};
	const CRAWL_SOURCE_LABEL: Record<string, string> = {
		sitemap: 'sitemap (tanggal modifikasi)',
		sitemap_news: 'news sitemap (cakupan ~48 jam)',
		feed: 'feed RSS/Atom (tanggal terbit)',
		html_sample: 'sampel halaman listing',
		wordpress_api: 'WordPress REST API (tanggal terbit)',
		google_news: 'Google News RSS (indeks)',
		wayback: 'Wayback CDX (perkiraan arsip)'
	};
	const CRAWL_CONFIDENCE_LABEL: Record<string, string> = {
		exact: 'utuh',
		lower_bound: 'batas bawah (≥ N)',
		estimate: 'perkiraan',
		unknown: 'tidak diketahui'
	};

	function crawlEmptyReason(row: { last_crawled_at: string | null; last_status: string | null; last_error: string | null }) {
		// Status percobaan terakhir lebih informatif daripada "belum pernah",
		// terutama untuk situs yang selalu diblokir WAF sejak awal.
		switch (row.last_status) {
			case 'no_source':
				return 'Tak ada sitemap/feed di situsnya';
			case 'blocked':
				return 'Diblokir situs (Cloudflare/robots)';
			case 'error':
				return row.last_error ? `Gagal: ${row.last_error}` : tr('Gagal dicrawl');
			case 'ok':
				return 'Terbaca, jumlah artikel belum tersedia';
		}
		if (!row.last_crawled_at) return tr('Belum pernah dicrawl');
		return tr('Belum ada angka dari crawler');
	}

	const TABS = [
		{ key: 'seo', label: 'SEO Metrics' },
		{ key: 'pagespeed', label: 'API Pagespeed' },
		{ key: 'smtp', label: 'Email / SMTP' },
		{ key: 'gsc', label: 'Integrasi GSC' },
		{ key: 'ga4', label: 'GA4 Config' },
		{ key: 'wa', label: 'WA Gateway' },
		{ key: 'domain', label: 'Domain Email' },
		{ key: 'turnstile', label: 'Turnstile CF' },
		{ key: 'google', label: 'Google Sign-In' },
		{ key: 'shortlink', label: 'Short Link' },
		{ key: 'crawlhub', label: 'Crawlhub' },
		{ key: 'server', label: 'Server' }
	] as const;

	type TabKey = (typeof TABS)[number]['key'];

	let tab = $state<TabKey>('seo');

	// scrollToTab: setelah tab dirender, gulir wadah tab ke bawah header sticky
	// supaya anchor (#smtp, #gsc, …) mendarat di judul bagian, bukan tertutup.
	async function scrollToTab(key: string) {
		await tick();
		const el = document.getElementById(key);
		if (el) el.scrollIntoView({ block: 'start' });
	}

	// Deep link juga mengenali sub-tab Email/SMTP: #smtp-config dan
	// #smtp-template membuka tab SMTP sekaligus bagian yang dimaksud.
	const HASH_TO_VIEW: Record<string, { tab: TabKey; smtp?: 'config' | 'templates' | 'log' }> = {
		'smtp-config': { tab: 'smtp', smtp: 'config' },
		'smtp-template': { tab: 'smtp', smtp: 'templates' },
		'smtp-log': { tab: 'smtp', smtp: 'log' }
	};

	function selectTab(key: TabKey) {
		tab = key;
		// Ganti hash tanpa menumpuk riwayat: URL bisa dibagikan dan tombol
		// Back tetap kembali ke halaman sebelumnya, bukan antar tab.
		// Sengaja TIDAK scroll: memindah viewport tiba-tiba saat ganti tab
		// terasa seperti halaman "melompat". Scroll hanya untuk deep link.
		if (typeof history !== 'undefined') {
			history.replaceState(history.state, '', `#${key}`);
		}
	}

	function selectSmtpView(view: 'config' | 'templates' | 'log') {
		smtpView = view;
		const hash = view === 'templates' ? 'smtp-template' : view === 'log' ? 'smtp-log' : 'smtp-config';
		if (typeof history !== 'undefined') {
			history.replaceState(history.state, '', `#${hash}`);
		}
	}

	// `fallback` per aksi supaya pesan errornya spesifik seperti app lama
	// (mis. "Gagal menguji domain."), bukan generik untuk semua tab.
	async function request<T>(path: string, init: RequestInit = {}, fallback = 'Permintaan platform gagal.'): Promise<{ ok: true; data: T } | { ok: false; error: string }> {
		try {
			const res = await apiRequest(`/v1/platform${path}`, init);
			if (!res.ok) {
				const detail = await readApiError(res, fallback);
				return {
					ok: false,
					error: res.status === 404 && detail === 'Resource not found' ? 'Akses platform administrator tidak tersedia.' : detail
				};
			}
			if (res.status === 204) return { ok: true, data: undefined as T };
			return { ok: true, data: (await res.json()) as T };
		} catch {
			return { ok: false, error: tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.') };
		}
	}

	// ── SEO Metrics ────────────────────────────────────────────────────────
	let seoConfig = $state<SeoConfig | null>(null);
	let seoLoading = $state(true);
	let seoError = $state('');
	let seoKey = $state('');
	let seoMsg = $state({ text: '', ok: false });
	let seoBusy = $state(false);
	let similarwebKey = $state('');
	let similarwebMsg = $state({ text: '', ok: false });
	let similarwebBusy = $state(false);
	type SwDiag = {
		ok: boolean;
		provider_status: number | null;
		ket?: string | null;
		detail?: string | null;
		payload_raw?: string | null;
		domain?: string | null;
		visits?: number | null;
		snapshot_date?: string | null;
	};
	const SW_PROVIDER_URL = 'https://rapidapi.com/belchiorarkad-FqvHs2EDOtP/api/similar-web-api';

	// Tutorial integrasi Similar Web API (Go CLI). Kode disimpan sebagai string
	// supaya kurung kurawal tidak dibaca sebagai ekspresi Svelte.
	const SW_TUT_INIT = `mkdir go-similarweb
cd go-similarweb
go mod init go-similarweb`;
	const SW_TUT_MODEL = `package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Struktur untuk menangkap response JSON
type ApiResponse struct {
	DomainAnalytics interface{} \`json:"domain_analytics"\`
}

// Konfigurasi API
const (
	BaseURL = "https://similar-web-api.p.rapidapi.com"
	ApiKey  = "YOUR_API_KEY_DISINI" // Ganti dengan API Key Anda
	ApiHost = "similar-web-api.p.rapidapi.com"
)`;
	const SW_TUT_REQUEST = `func getWebsiteAnalytics(targetDomain string) {
	// 1. Encode URL parameter untuk keamanan
	endpoint := fmt.Sprintf("%s/analyticsv1?url=%s", BaseURL, url.QueryEscape(targetDomain))

	// 2. Buat request
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\\n", err)
		return
	}

	// 3. Tambahkan header wajib
	req.Header.Add("x-rapidapi-key", ApiKey)
	req.Header.Add("x-rapidapi-host", ApiHost)

	// 4. Eksekusi request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error executing request: %v\\n", err)
		return
	}
	defer res.Body.Close()

	// 5. Baca body response
	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("Error reading body: %v\\n", err)
		return
	}

	// 6. Cek status code
	if res.StatusCode != 200 {
		fmt.Printf("API Error: %s - %s\\n", res.Status, string(body))
		return
	}

	// 7. Parse JSON ke struct atau cetak langsung
	var data ApiResponse
	if err := json.Unmarshal(body, &data); err != nil {
		fmt.Printf("Error parsing JSON: %v\\n", err)
		return
	}

	fmt.Printf("Berhasil mengambil data untuk: %s\\n", targetDomain)
	fmt.Println("Hasil (Raw JSON):", string(body))
}`;
	const SW_TUT_MAIN = `func main() {
	var domain string
	fmt.Print("Masukkan nama domain (contoh: amazon.com): ")
	fmt.Scanln(&domain)

	if domain == "" {
		fmt.Println("Domain tidak boleh kosong")
		return
	}

	getWebsiteAnalytics(domain)
}`;
	const SW_TUT_OTHER = `// Contoh untuk Traffic
endpoint := fmt.Sprintf("%s/domain/traffic?url=%s", BaseURL, targetDomain)`;
	let swTestBusy = $state(false);
	let swTest = $state<SwDiag | null>(null);
	let serpensKey = $state('');
	let serpensMsg = $state({ text: '', ok: false });
	let serpensBusy = $state(false);

	async function loadSeo() {
		const res = await request<SeoConfig>('/seo/config');
		if (res.ok) seoConfig = res.data;
		else seoError = res.error;
		seoLoading = false;
	}

	async function saveSeo(e: SubmitEvent) {
		e.preventDefault();
		seoBusy = true;
		seoMsg = { text: '', ok: false };
		const res = await request<SeoConfig>('/seo/config', {
			method: 'PUT',
			body: JSON.stringify({ api_key: seoKey.trim() || null })
		});
		if (!res.ok) seoMsg = { text: res.error, ok: false };
		else {
			seoKey = '';
			seoMsg = { text: 'RapidAPI SEO berhasil disimpan.', ok: true };
			await loadSeo();
		}
		seoBusy = false;
	}

	// ── Tes koneksi DA/PA (diagnosa provider, tanpa menyentuh media) ──────
	let seoDiagDomain = $state('app.tauthub.com');
	let seoDiagBusy = $state(false);
	let seoDiag = $state<{ text: string; ok: boolean; raw: string } | null>(null);
	async function testSeoProvider() {
		seoDiagBusy = true;
		seoDiag = null;
		try {
			const res = await request<{
				ok?: boolean; detail?: string; provider_status?: number; da?: number; pa?: number;
				payload_raw?: string; endpoint?: string; missing?: string[];
			}>(`/seo/diagnose?domain=${encodeURIComponent(seoDiagDomain.trim() || 'app.tauthub.com')}`);
			if (!res.ok) seoDiag = { text: res.error, ok: false, raw: '' };
			else {
				const d = res.data ?? {};
				const ok = Boolean(d.ok);
				const text = ok
					? `Provider menjawab. DA ${d.da ?? '—'} · PA ${d.pa ?? '—'}${d.missing?.length ? ` · tidak ditemukan: ${d.missing.join(', ')}` : ''}`
					: d.detail || 'Provider menolak permintaan.';
				seoDiag = { text, ok, raw: d.payload_raw ?? '' };
			}
		} finally {
			seoDiagBusy = false;
		}
	}

	async function saveSimilarWeb(e: SubmitEvent) {
		e.preventDefault();
		similarwebBusy = true;
		similarwebMsg = { text: '', ok: false };
		const res = await request<SeoConfig>('/seo/config', {
			method: 'PUT',
			body: JSON.stringify({ similarweb_api_key: similarwebKey.trim() || null })
		});
		if (!res.ok) similarwebMsg = { text: res.error, ok: false };
		else {
			similarwebKey = '';
			similarwebMsg = { text: 'Key Similar Web API berhasil disimpan.', ok: true };
			await loadSeo();
		}
		similarwebBusy = false;
	}

	async function testSimilarWeb() {
		swTestBusy = true;
		swTest = null;
		const res = await request<SwDiag>('/seo/similarweb-diagnose', {}, 'Gagal menguji SimilarWeb.');
		if (!res.ok) swTest = { ok: false, provider_status: null, ket: res.error };
		else swTest = res.data;
		swTestBusy = false;
	}

	async function saveSerpens(e: SubmitEvent) {
		e.preventDefault();
		serpensBusy = true;
		serpensMsg = { text: '', ok: false };
		const res = await request<SeoConfig>('/seo/config', {
			method: 'PUT',
			body: JSON.stringify({ serpens_api_key: serpensKey.trim() || null })
		});
		if (!res.ok) serpensMsg = { text: res.error, ok: false };
		else {
			serpensKey = '';
			serpensMsg = { text: 'SERPens API key berhasil disimpan.', ok: true };
			await loadSeo();
		}
		serpensBusy = false;
	}

	// ── API Pagespeed ──────────────────────────────────────────────────────
	let psConfig = $state<{ api_key_configured: boolean; api_key_hint?: string | null } | null>(null);
	let psLoading = $state(true);
	let psKey = $state('');
	let psBusy = $state(false);
	let psMsg = $state({ text: '', ok: false });
	let psTestBusy = $state(false);
	let psTestMsg = $state({ text: '', ok: false });
	let psTestLatency = $state<number | null>(null);

	async function loadPagespeed() {
		const res = await request<{ api_key_configured: boolean; api_key_hint?: string | null }>('/settings/pagespeed');
		if (res.ok) psConfig = res.data;
		else psMsg = { text: res.error, ok: false };
		psLoading = false;
	}

	async function savePagespeed(e: SubmitEvent) {
		e.preventDefault();
		psBusy = true;
		psMsg = { text: '', ok: false };
		const res = await request('/settings/pagespeed', {
			method: 'PUT',
			body: JSON.stringify({ api_key: psKey.trim() || null })
		});
		if (!res.ok) psMsg = { text: res.error, ok: false };
		else {
			psKey = '';
			psMsg = { text: 'API Pagespeed berhasil disimpan.', ok: true };
			await loadPagespeed();
		}
		psBusy = false;
	}

	async function testPagespeed() {
		psTestBusy = true;
		psTestMsg = { text: '', ok: false };
		psTestLatency = null;
		const res = await request<{ ok: boolean; message: string; latency_ms?: number }>('/settings/pagespeed/test', {
			method: 'POST',
			body: JSON.stringify({ url: 'https://example.com' })
		});
		if (!res.ok) psTestMsg = { text: res.error, ok: false };
		else {
			psTestMsg = { text: res.data.message, ok: res.data.ok };
			psTestLatency = res.data.latency_ms ?? null;
		}
		psTestBusy = false;
	}

	// ── Email / SMTP ───────────────────────────────────────────────────────
	let smtp = $state<SmtpConfig | null>(null);
	// Sub-navigasi tab Email/SMTP: form koneksi atau panel template.
	let smtpView = $state<'config' | 'templates' | 'log'>('config');
	let smtpLoading = $state(true);
	let smtpError = $state('');
	let smtpBusy = $state(false);
	let smtpMsg = $state({ text: '', ok: false });
	let smtpHost = $state('');
	let smtpPort = $state('587');
	let smtpSecure = $state(false);
	let smtpUsername = $state('');
	let smtpPassword = $state('');
	let smtpFromEmail = $state('no-reply@tauthub.com');
	let smtpFromName = $state('Tauthub');
	let smtpEnabled = $state(false);
	let smtpTestBusy = $state(false);
	let smtpTestMsg = $state({ text: '', ok: false });
	let smtpSendBusy = $state(false);
	let smtpSendMsg = $state({ text: '', ok: false });
	let testEmail = $state('');

	// Webhook bounce/komplain Kirim.id: secret disimpan ter-seal, tidak pernah
	// dikembalikan ke UI; hanya status dan URL yang didaftarkan di Kirim.id.
	let hookInfo = $state<{ configured: boolean; source: string; url: string } | null>(null);
	let hookSecret = $state('');
	let hookBusy = $state(false);
	let hookMsg = $state({ text: '', ok: false });

	async function loadWebhook() {
		const res = await request<{ configured: boolean; source: string; url: string }>('/settings/email/webhook');
		if (res.ok) hookInfo = res.data;
	}

	async function saveWebhook(e: SubmitEvent) {
		e.preventDefault();
		if (!hookSecret.trim() && hookInfo?.configured && !confirm('Kolom kosong akan menghapus secret webhook yang tersimpan. Lanjutkan?')) return;
		hookBusy = true;
		hookMsg = { text: '', ok: false };
		const res = await request('/settings/email/webhook', {
			method: 'PUT',
			body: JSON.stringify({ secret: hookSecret.trim() })
		});
		if (!res.ok) hookMsg = { text: res.error, ok: false };
		else {
			hookMsg = { text: hookSecret.trim() ? 'Secret webhook disimpan.' : 'Secret webhook dihapus.', ok: true };
			hookSecret = '';
			await loadWebhook();
		}
		hookBusy = false;
	}

	async function loadSmtp() {
		void loadWebhook();
		const res = await request<SmtpConfig>('/settings/email');
		if (!res.ok) {
			smtpError = res.error;
		} else {
			const data = res.data;
			smtp = data;
			smtpHost = data.host ?? '';
			smtpPort = String(data.port ?? 587);
			smtpSecure = data.secure ?? false;
			smtpUsername = data.username ?? '';
			smtpFromEmail = data.from_email ?? 'no-reply@tauthub.com';
			smtpFromName = data.from_name ?? 'Tauthub';
			smtpEnabled = data.enabled ?? false;
		}
		smtpLoading = false;
	}

	async function saveSmtp(e: SubmitEvent) {
		e.preventDefault();
		smtpBusy = true;
		smtpMsg = { text: '', ok: false };
		const res = await request('/settings/email', {
			method: 'PUT',
			body: JSON.stringify({
				host: smtpHost.trim() || null,
				port: Number(smtpPort) || 587,
				secure: smtpSecure,
				username: smtpUsername.trim() || null,
				password: smtpPassword.trim() || null,
				from_email: smtpFromEmail.trim(),
				from_name: smtpFromName.trim(),
				enabled: smtpEnabled
			})
		});
		if (!res.ok) smtpMsg = { text: res.error, ok: false };
		else {
			smtpPassword = '';
			smtpMsg = { text: tr('SMTP berhasil disimpan — email berikutnya langsung memakai konfigurasi ini.'), ok: true };
			await loadSmtp();
		}
		smtpBusy = false;
	}

	async function testSmtp() {
		smtpTestBusy = true;
		smtpTestMsg = { text: '', ok: false };
		const res = await request<{ ok: boolean; message: string; latency_ms?: number }>('/settings/email/test-connection', {
			method: 'POST'
		});
		if (!res.ok) smtpTestMsg = { text: res.error, ok: false };
		else smtpTestMsg = { text: `${res.data.message}${res.data.latency_ms ? ` (${res.data.latency_ms}ms)` : ''}`, ok: res.data.ok };
		await loadSmtp();
		smtpTestBusy = false;
	}

	async function sendTestEmail() {
		if (!testEmail.trim()) {
			smtpSendMsg = { text: 'Masukkan alamat email tujuan.', ok: false };
			return;
		}
		smtpSendBusy = true;
		smtpSendMsg = { text: '', ok: false };
		const res = await request('/settings/email/test-send', {
			method: 'POST',
			body: JSON.stringify({ recipient_email: testEmail.trim() })
		});
		smtpSendMsg = res.ok
			? { text: 'Test email berhasil dikirim.', ok: true }
			: { text: res.error, ok: false };
		smtpSendBusy = false;
	}

	// ── Turnstile ──────────────────────────────────────────────────────────
	let turnstile = $state<TurnstileConfig | null>(null);
	let tsLoading = $state(true);
	let tsError = $state('');
	let tsSiteKey = $state('');
	let tsSecretKey = $state('');
	let tsEnabled = $state(false);
	let tsBusy = $state(false);
	let tsMsg = $state({ text: '', ok: false });
	let tsTestBusy = $state(false);
	let tsTestMsg = $state({ text: '', ok: false });

	async function loadTurnstile() {
		const res = await request<TurnstileConfig>('/settings/turnstile');
		if (!res.ok) tsError = res.error;
		else {
			turnstile = res.data;
			tsSiteKey = res.data.site_key ?? '';
			tsEnabled = res.data.enabled ?? false;
		}
		tsLoading = false;
	}

	async function saveTurnstile(e: SubmitEvent) {
		e.preventDefault();
		tsBusy = true;
		tsMsg = { text: '', ok: false };
		const res = await request('/settings/turnstile', {
			method: 'PUT',
			body: JSON.stringify({
				...(tsSiteKey.trim() && { site_key: tsSiteKey.trim() }),
				...(tsSecretKey.trim() && { secret_key: tsSecretKey.trim() }),
				enabled: tsEnabled
			})
		});
		if (!res.ok) tsMsg = { text: res.error, ok: false };
		else {
			tsSecretKey = '';
			tsMsg = { text: 'Konfigurasi Turnstile berhasil disimpan.', ok: true };
			await loadTurnstile();
		}
		tsBusy = false;
	}

	async function testTurnstile() {
		tsTestBusy = true;
		tsTestMsg = { text: '', ok: false };
		const res = await request<{ ok: boolean; message: string }>('/settings/turnstile/test', { method: 'POST' });
		tsTestMsg = res.ok ? { text: res.data.message, ok: res.data.ok } : { text: res.error, ok: false };
		tsTestBusy = false;
	}

	// ── Google Sign-In ─────────────────────────────────────────────────────
	type GoogleConfig = {
		client_id: string | null;
		redirect_uri: string | null;
		callback_url: string;
		// Diagnostik admin-only (tidak pernah tampil di halaman publik).
		configured?: boolean;
		config_source?: string;
		config_error?: string;
		config_error_detail?: string;
		missing_fields?: string[];
	};
	let google = $state<GoogleConfig | null>(null);
	let googleClientId = $state('');
	let googleClientSecret = $state('');
	let googleRedirectUri = $state('');
	let googleBusy = $state(false);
	let googleMsg = $state({ text: '', ok: false });

	async function loadGoogle() {
		const res = await request<GoogleConfig>('/oauth/google');
		if (!res.ok) googleMsg = { text: res.error, ok: false };
		else {
			google = res.data;
			googleClientId = res.data.client_id ?? '';
			googleRedirectUri = res.data.redirect_uri ?? '';
		}
	}

	async function saveGoogle(e: SubmitEvent) {
		e.preventDefault();
		googleBusy = true;
		googleMsg = { text: '', ok: false };
		const res = await request('/oauth/google', {
			method: 'PUT',
			body: JSON.stringify({
				client_id: googleClientId.trim(),
				client_secret: googleClientSecret.trim() || null,
				redirect_uri: googleRedirectUri.trim()
			})
		});
		if (!res.ok) googleMsg = { text: res.error, ok: false };
		else {
			googleClientSecret = '';
			googleMsg = { text: 'Konfigurasi Google Sign-In berhasil disimpan.', ok: true };
			await loadGoogle();
		}
		googleBusy = false;
	}

	// ── Short Link ─────────────────────────────────────────────────────────
	let shortDomain = $state<string | null>(null);
	let shortInput = $state('');
	let shortLoading = $state(true);
	let shortBusy = $state(false);
	let shortMsg = $state({ text: '', ok: false });

	async function loadShortLink() {
		const res = await request<{ domain: string | null }>('/short-link-config');
		if (!res.ok) shortMsg = { text: res.error, ok: false };
		else {
			shortDomain = res.data.domain ?? null;
			shortInput = res.data.domain ?? '';
		}
		shortLoading = false;
	}

	async function saveShortLink(e: SubmitEvent) {
		e.preventDefault();
		shortBusy = true;
		shortMsg = { text: '', ok: false };
		const res = await request<{ domain: string | null }>('/short-link-config', {
			method: 'PUT',
			body: JSON.stringify({ domain: shortInput.trim() || null })
		});
		if (!res.ok) shortMsg = { text: res.error, ok: false };
		else {
			shortDomain = res.data.domain ?? null;
			shortMsg = {
				text: res.data.domain ? 'Domain short link tersimpan.' : tr('Kembali ke fallback domain utama.'),
				ok: true
			};
			await loadShortLink();
		}
		shortBusy = false;
	}

	// ── Crawlhub ───────────────────────────────────────────────────────────
	let crawlTab = $state<'endpoint' | 'monitoring'>('endpoint');
	let chConfig = $state<CrawlhubConfig | null>(null);
	let chLoading = $state(true);
	let chLoadError = $state('');
	let chBaseUrl = $state('');
	let chHmac = $state('');
	let chIngest = $state('');
	let chSaving = $state(false);
	let chNotice = $state('');
	let chSaveError = $state('');
	let chDomain = $state('');
	let chTesting = $state(false);
	let chTestResult = $state<CrawlTestResult | null>(null);
	let chTestError = $state('');
	let chMediaId = $state('');
	let chTarget = $state<CrawlTarget | null>(null);
	let chChecking = $state(false);
	let chTargetError = $state('');
	let monitor = $state<MonitorData | null>(null);
	let monitorFilter = $state<'all' | 'registered' | 'problem' | 'stale' | 'unregistered'>('all');
	let monitorLoading = $state(false);
	let monitorError = $state('');
	let monitorOffset = $state(0);
	let syncBusy = $state(false);
	let refreshingId = $state<string | null>(null);

	async function loadCrawlhub() {
		chLoading = true;
		chLoadError = '';
		const res = await request<CrawlhubConfig>('/crawlhub', {}, 'Gagal memuat konfigurasi Crawlhub.');
		if (res.ok) {
			chConfig = res.data;
			chBaseUrl = res.data.base_url;
		} else {
			chLoadError = res.error;
		}
		chLoading = false;
	}

	async function loadMonitor() {
		monitorLoading = true;
		monitorError = '';
		const res = await request<MonitorData>(`/crawlhub/targets?filter=${monitorFilter}&limit=20&offset=${monitorOffset}`, {}, 'Gagal memuat status crawl media.');
		if (res.ok) {
			const cutoff = Date.now() - 7 * 24 * 3600 * 1000;
			monitor = {
				...res.data,
				targets: res.data.targets.map((t) => ({
					...t,
					stale: !t.registered || !t.last_crawled_at || new Date(t.last_crawled_at).getTime() < cutoff
				}))
			};
		} else {
			monitorError = res.error;
		}
		monitorLoading = false;
	}

	async function syncCrawlhub() {
		syncBusy = true;
		monitorError = '';
		const res = await request<{ ok: boolean; enqueued: number }>(
			'/crawlhub/targets',
			{ method: 'POST' },
			'Gagal menjalankan sinkronisasi.'
		);
		if (res.ok) {
			const n = res.data.enqueued;
			chNotice =
				n === 0
					? 'Tidak ada media yang perlu disinkronkan — semua sudah punya angka segar.'
					: `${n} media diantrekan (belum pernah crawl / lama tidak crawl / error). Worker memproses di latar belakang — angka muncul bertahap.`;
			await loadMonitor();
			// Worker memproses antrean di latar belakang; segarkan bertahap.
			setTimeout(() => void loadMonitor(), 8000);
			setTimeout(() => void loadMonitor(), 30000);
			setTimeout(() => void loadMonitor(), 60000);
		} else {
			monitorError = res.error;
		}
		syncBusy = false;
	}

	async function recrawlRow(row: CrawlRow) {
		refreshingId = row.media_id;
		monitorError = '';
		const res = await request<{ ok: boolean; articles_7d?: number; source?: string }>(
			'/crawlhub',
			{
				method: 'POST',
				body: JSON.stringify({ action: 'crawl_now', mediaId: row.media_id })
			},
			'Gagal meminta crawl ulang.'
		);
		if (res.ok) {
			await loadMonitor();
			const measured = typeof res.data?.articles_7d === 'number';
			chNotice = measured
				? `Crawl ulang ${row.name} selesai — terdeteksi ${res.data.articles_7d} artikel 7 hari terakhir via ${CRAWL_SOURCE_LABEL[res.data.source ?? ''] ?? res.data.source ?? 'fallback'}.`
				: `Permintaan crawl ulang terkirim untuk ${row.name}. Crawlhub sedang bekerja; angka muncul dalam beberapa detik sampai satu menit.`;
			setTimeout(() => void loadMonitor(), 8000);
			setTimeout(() => void loadMonitor(), 30000);
		} else {
			monitorError = res.error;
		}
		refreshingId = null;
	}

	async function saveCrawlhub(e: SubmitEvent) {
		e.preventDefault();
		chSaving = true;
		chSaveError = '';
		chNotice = '';
		const res = await request<CrawlhubConfig>(
			'/crawlhub',
			{
				method: 'PUT',
				body: JSON.stringify({
					base_url: chBaseUrl.trim(),
					hmac_secret: chHmac.trim() === '' ? null : chHmac.trim(),
					ingest_secret: chIngest.trim() === '' ? null : chIngest.trim()
				})
			},
			'Gagal menyimpan konfigurasi.'
		);
		if (res.ok) {
			chConfig = res.data;
			chBaseUrl = res.data.base_url;
			chHmac = '';
			chIngest = '';
			chNotice = 'Konfigurasi tersimpan.';
		} else {
			chSaveError = res.error;
		}
		chSaving = false;
	}

	async function runCrawlTest(e: SubmitEvent) {
		e.preventDefault();
		const value = chDomain.trim();
		if (!value) return;
		chTesting = true;
		chTestError = '';
		chTestResult = null;
		const res = await request<{ ok: boolean; result: CrawlTestResult }>(
			'/crawlhub',
			{
				method: 'POST',
				body: JSON.stringify({ action: 'test', domain: value })
			},
			'Gagal menguji domain.'
		);
		if (res.ok) chTestResult = res.data.result;
		else chTestError = res.error;
		chTesting = false;
	}

	async function checkCrawlTarget(e: SubmitEvent) {
		e.preventDefault();
		const value = chMediaId.trim();
		if (!value) return;
		chChecking = true;
		chTargetError = '';
		chTarget = null;
		const res = await request<{ ok: boolean; target: CrawlTarget }>(
			'/crawlhub',
			{
				method: 'POST',
				body: JSON.stringify({ action: 'status', mediaId: value })
			},
			'Gagal memeriksa status media.'
		);
		if (res.ok) chTarget = res.data.target;
		else chTargetError = res.error;
		chChecking = false;
	}

	// ── Server config ──────────────────────────────────────────────────────
	let serverCfg = $state<{ ip: string | null; source: 'dashboard' | 'env' | 'none' } | null>(null);
	let serverIp = $state('');
	let serverLoading = $state(true);
	let serverBusy = $state(false);
	let serverMsg = $state({ text: '', ok: false });
	let serverError = $state('');

	async function loadServer() {
		const res = await request<{ ip: string | null; source: 'dashboard' | 'env' | 'none' }>('/server-config');
		if (res.ok) {
			serverCfg = res.data;
			serverIp = res.data.ip ?? '';
		} else {
			serverError = res.error;
		}
		serverLoading = false;
	}

	async function saveServer(e: SubmitEvent) {
		e.preventDefault();
		serverBusy = true;
		serverMsg = { text: '', ok: false };
		serverError = '';
		const res = await request<{ ip: string | null; source: 'dashboard' | 'env' | 'none' }>('/server-config', {
			method: 'PUT',
			body: JSON.stringify({ public_ip: serverIp.trim() || null })
		});
		if (res.ok) {
			serverCfg = res.data;
			serverIp = res.data.ip ?? '';
			serverMsg = {
				text: res.data.ip ? 'IP publik server disimpan.' : 'Nilai dihapus — sistem kembali mendeteksi otomatis.',
				ok: true
			};
		} else {
			serverError = res.error;
		}
		serverBusy = false;
	}

	onMount(() => {
		// Deep link: /admin/konfigurasi#smtp membuka tab itu. Gulir hanya di
		// sini (hash diubah dari luar/halaman dimuat), bukan saat klik tab,
		// supaya pindah tab tidak membuat viewport melompat tiba-tiba.
		const applyHash = () => {
			const key = window.location.hash.replace(/^#/, '').trim();
			if (!key) return;
			const sub = HASH_TO_VIEW[key];
			if (sub) {
				tab = sub.tab;
				if (sub.smtp) smtpView = sub.smtp;
				scrollToTab(sub.tab);
				return;
			}
			if (TABS.some((t) => t.key === key)) {
				tab = key as TabKey;
				scrollToTab(key);
			}
		};
		applyHash();
		window.addEventListener('hashchange', applyHash);
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([
				loadSeo(),
				loadPagespeed(),
				loadSmtp(),
				loadTurnstile(),
				loadGoogle(),
				loadShortLink(),
				loadCrawlhub(),
				loadMonitor(),
				loadServer()
			]);
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => {
			window.removeEventListener('hashchange', applyHash);
			window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
		};
	});

	const serverSourceLabel = $derived(
		serverCfg?.source === 'dashboard'
			? tr('Diatur dari dashboard ini')
			: serverCfg?.source === 'env'
				? tr('Dari env SERVER_PUBLIC_IP (belum diubah dari dashboard)')
				: tr('Belum diatur — sistem mendeteksi otomatis saat dibutuhkan')
	);
</script>

<section>
	<PageHeader
		title="Konfigurasi"
		description="Kredensial layanan pihak ketiga — key disegel server-side dan tidak pernah tampil plaintext."
	/>

	<div class="mt-4 flex flex-wrap gap-2">
		{#each TABS as t (t.key)}
			<!-- Tiap tab punya anchor sendiri: #seo, #smtp, #gsc, dan seterusnya,
			     supaya tautan langsung ke satu bagian bisa dibagikan. -->
			<a
				href={`#${t.key}`}
				id={`tab-${t.key}`}
				class="app-btn {tab === t.key ? 'app-btn-primary' : 'app-btn-soft'} !min-h-8 !px-3 !text-xs"
				style="text-decoration: none;"
				aria-current={tab === t.key ? 'page' : undefined}
				onclick={(e) => {
					e.preventDefault();
					selectTab(t.key);
				}}
			>{t.label}</a>
		{/each}
	</div>

	<!-- Wadah isi tab: jadi target anchor (#seo, #smtp, …) dengan offset
	     scroll supaya judul tidak tertutup header sticky. -->
	<div id={tab} class="scroll-mt-24">

	<!-- SEO Metrics -->
	{#if tab === 'seo'}
		<div class="app-card mt-3 p-4">
			{#if seoLoading && !seoConfig && !seoError}
				<p class="text-sm text-muted-foreground">{tr("Memuat konfigurasi…")}</p>
			{:else}
				<div class="space-y-4">
					<Feedback error={seoError} />
					<div class="flex flex-wrap items-center gap-2">
						<KeyRound size={18} />
						<h2 class="text-base font-semibold">SEO Metrics API (RapidAPI)</h2>
						<Badge tone={seoConfig?.api_key_configured ? 'success' : 'warning'} label={seoConfig?.api_key_configured ? 'Terhubung' : tr('Belum Dikonfigurasi')} />
					</div>
					<p class="text-sm text-muted-foreground">{tr("API server-side untuk fitur")}<strong>Cek DA/PA</strong>{tr("di dashboard Mitra (Domain & Page Authority media). Tanpa key ini, mitra yang klik Cek DA/PA akan melihat pesan \"RapidAPI SEO belum dikonfigurasi oleh Admin\". Key tidak pernah ditampilkan atau dikirim ke browser.")}</p>
					<!-- Kredit provider (khusus internal): nama developer RapidAPI yang
					     dipakai, dibuka di tab baru dan nofollow. -->
					<p class="text-xs text-muted-foreground">
						{tr('Provider')}:
						<a
							class="font-medium underline underline-offset-2"
							style="color: var(--primary);"
							href={seoConfig?.provider_url ?? 'https://rapidapi.com/ksdeveloper/api/bulk-da-pa-checker-semrush'}
							target="_blank"
							rel="noopener noreferrer nofollow"
						>{seoConfig?.provider_label ?? 'KSDeveloper'}</a>
						<span>· Bulk DA PA Checker (Semrush)</span>
					</p>
					<div class="rounded-lg border px-3 py-2 text-xs text-muted-foreground">
						Host: <code class="rounded bg-secondary px-1">{seoConfig?.host ?? 'bulk-da-pa-checker-semrush.p.rapidapi.com'}</code>
						{seoConfig?.api_key_hint ? ` · Key ${seoConfig.api_key_hint}` : ''}
					</div>
					{#if seoConfig?.last_da_error}
						<div class="rounded-lg border px-3 py-2 text-xs" style="border-color: var(--warning); background-color: var(--warning-soft); color: var(--warning);">
							<p class="font-semibold">{tr('Percobaan terakhir ditolak provider')}{seoConfig.last_da_error_at ? ` · ${fmtDateTime(seoConfig.last_da_error_at)}` : ''}</p>
							<p class="mt-0.5">{seoConfig.last_da_error}</p>
						</div>
					{/if}
					<div class="rounded-lg border p-3" style="border-color: var(--border);">
						<div class="flex flex-wrap items-end gap-2">
							<label class="min-w-[200px] flex-1">
								<span class="text-xs font-medium">{tr('Domain uji')}</span>
								<input class="app-input mt-1 w-full" bind:value={seoDiagDomain} placeholder="contoh.com" />
							</label>
							<button class="app-btn app-btn-soft !min-h-9" type="button" disabled={seoDiagBusy} onclick={() => void testSeoProvider()}>
								{seoDiagBusy ? 'Menguji…' : tr('Tes koneksi provider')}
							</button>
						</div>
						{#if seoDiag}
							<p class="mt-2 text-xs font-medium" style="color: {seoDiag.ok ? 'var(--success)' : 'var(--destructive)'};">{seoDiag.text}</p>
							{#if seoDiag.raw}
								<pre class="mt-1 max-h-40 overflow-auto whitespace-pre-wrap break-all rounded bg-secondary p-2 text-[11px] text-muted-foreground">{seoDiag.raw}</pre>
							{/if}
						{/if}
					</div>
					<form class="space-y-3" onsubmit={saveSeo}>
						<label class="block">
							<span class="text-sm font-medium">RapidAPI Key</span>
							<input
								class="app-input mt-1 w-full"
								type="password"
								required={!seoConfig?.api_key_configured}
								minlength={seoConfig?.api_key_configured ? 0 : 8}
								placeholder={seoConfig?.api_key_configured ? tr('Kosongkan jika tidak diubah') : 'Masukkan RapidAPI key'}
								bind:value={seoKey}
							/>
						</label>
						{#if seoMsg.text}
							<div class="flex items-center gap-2 text-sm" style="color: {seoMsg.ok ? 'var(--success)' : 'var(--destructive)'};">
								{#if seoMsg.ok}<CheckCircle2 size={15} />{:else}<TriangleAlert size={15} />{/if}
								{seoMsg.text}
							</div>
						{/if}
						<button class="app-btn app-btn-primary" type="submit" disabled={seoBusy}>{seoBusy ? 'Menyimpan…' : tr('Simpan RapidAPI Key')}</button>
					</form>
					{#if !seoConfig?.api_key_configured}
						<p class="rounded-lg border px-3 py-6 text-center text-sm text-muted-foreground">{tr("Belum ada key tersimpan — fitur Cek DA/PA nonaktif sampai key diisi.")}</p>
					{/if}

					<hr class="border-t" />

					<!-- SimilarWeb -->
					<div class="space-y-3">
						<div class="flex flex-wrap items-center gap-2">
							<Globe size={18} />
							<h2 class="text-base font-semibold">Similar Web API (RapidAPI)</h2>
							<Badge tone={seoConfig?.similarweb_api_key_configured ? 'success' : 'warning'} label={seoConfig?.similarweb_api_key_configured ? 'Key terhubung' : tr('Belum Dikonfigurasi')} />
						</div>
						<p class="text-sm text-muted-foreground">{tr("API untuk angka kunjungan bulanan")}<strong>SimilarWeb</strong>{tr("di Direktori Media, Katalog, dan matchmaker. Cukup satu key RapidAPI; key tidak pernah ditampilkan atau dikirim ke browser.")}</p>
						<!-- Kredit provider: listing RapidAPI yang benar-benar dipanggil backend. -->
						<p class="text-xs text-muted-foreground">
							{tr('Data SimilarWeb di Tauthub diambil lewat')}
							<a
								class="font-medium underline underline-offset-2"
								style="color: var(--primary);"
								href={seoConfig?.similarweb_provider_url ?? SW_PROVIDER_URL}
								target="_blank"
								rel="noopener noreferrer nofollow"
							>Similar Web API</a>
							{tr('di RapidAPI, dikembangkan oleh')} <strong>belchiorarkad</strong>.
							{tr('Akun RapidAPI pemilik key harus Subscribe ke listing tersebut.')}
						</p>
						<div class="rounded-lg border px-3 py-2 text-xs text-muted-foreground">
							Host: <code class="rounded bg-secondary px-1">{seoConfig?.similarweb_host ?? 'similar-web-api.p.rapidapi.com'}</code>
							· Endpoint: <code class="rounded bg-secondary px-1">/analyticsv1?url=</code>
						</div>
						<!-- Tutorial integrasi (Go CLI), tertutup secara default supaya kartu tetap ringkas. -->
						<details class="rounded-lg border px-3 py-2 text-xs">
							<summary class="cursor-pointer font-medium">{tr('Tutorial: Integrasi Similar Web API dengan Go')}</summary>
							<div class="mt-3 space-y-3 text-muted-foreground">
								<p>{tr('Tutorial ini memandu membuat aplikasi CLI sederhana untuk mengambil data analitik sebuah website.')}</p>

								<p class="font-semibold" style="color: var(--foreground);">1. {tr('Persiapan')}</p>
								<ul class="list-disc space-y-0.5 pl-5">
									<li>API Key: {tr('dapatkan dari')} <a class="underline underline-offset-2" style="color: var(--primary);" href={SW_PROVIDER_URL} target="_blank" rel="noopener noreferrer nofollow">RapidAPI Similar Web API</a>.</li>
									<li>{tr('Go terpasang di komputer')} (<code class="rounded bg-secondary px-1">go version</code>).</li>
								</ul>

								<p class="font-semibold" style="color: var(--foreground);">2. {tr('Inisialisasi project')}</p>
								<pre class="overflow-x-auto rounded bg-secondary p-2"><code>{SW_TUT_INIT}</code></pre>

								<p class="font-semibold" style="color: var(--foreground);">3. {tr('Membuat struktur data (models)')}</p>
								<p>{tr('API mengembalikan JSON, jadi buat struct agar mudah diolah. Contoh untuk endpoint')} <code class="rounded bg-secondary px-1">/analyticsv1</code>. {tr('Buat file')} <code class="rounded bg-secondary px-1">main.go</code>:</p>
								<pre class="overflow-x-auto rounded bg-secondary p-2"><code>{SW_TUT_MODEL}</code></pre>

								<p class="font-semibold" style="color: var(--foreground);">4. {tr('Membuat fungsi request')}</p>
								<pre class="overflow-x-auto rounded bg-secondary p-2"><code>{SW_TUT_REQUEST}</code></pre>

								<p class="font-semibold" style="color: var(--foreground);">5. {tr('Fungsi main')}</p>
								<pre class="overflow-x-auto rounded bg-secondary p-2"><code>{SW_TUT_MAIN}</code></pre>

								<p class="font-semibold" style="color: var(--foreground);">6. {tr('Cara menjalankan')}</p>
								<ul class="list-disc space-y-0.5 pl-5">
									<li>{tr('Ganti')} <code class="rounded bg-secondary px-1">YOUR_API_KEY_DISINI</code> {tr('dengan key asli dari RapidAPI, lalu simpan.')}</li>
									<li>{tr('Jalankan')} <code class="rounded bg-secondary px-1">go run main.go</code></li>
								</ul>

								<p class="font-semibold" style="color: var(--foreground);">{tr('Endpoint lain')}</p>
								<p>{tr('Cukup ganti bagian endpoint di dalam kode:')}</p>
								<pre class="overflow-x-auto rounded bg-secondary p-2"><code>{SW_TUT_OTHER}</code></pre>
								<ul class="list-disc space-y-0.5 pl-5">
									<li><code class="rounded bg-secondary px-1">/analyticsv1</code>: {tr('overview lengkap (Global Rank, Country Rank, Category, Traffic).')}</li>
									<li><code class="rounded bg-secondary px-1">/domain/traffic</code>: {tr('data trafik bulanan.')}</li>
									<li><code class="rounded bg-secondary px-1">/domain/competitors</code>: {tr('daftar website kompetitor yang serupa.')}</li>
									<li><code class="rounded bg-secondary px-1">/domain/topkeyword</code>: {tr('kata kunci SEO yang paling banyak mendatangkan trafik.')}</li>
									<li><code class="rounded bg-secondary px-1">/domain/engagement</code>: {tr('rata-rata durasi kunjungan dan bounce rate.')}</li>
								</ul>

								<p class="font-semibold" style="color: var(--foreground);">{tr('Error handling')}</p>
								<ul class="list-disc space-y-0.5 pl-5">
									<li><strong>401</strong>: {tr('API key salah atau tidak aktif.')}</li>
									<li><strong>429</strong>: {tr('batas paket terlewati (rate limit).')}</li>
									<li><strong>404</strong>: {tr('data domain belum tersedia di database SimilarWeb.')}</li>
								</ul>
							</div>
						</details>
						<form class="space-y-3" onsubmit={saveSimilarWeb}>
							<label class="block">
								<span class="text-sm font-medium">Similar Web API — RapidAPI Key</span>
								<input
									class="app-input mt-1 w-full"
									type="password"
									required={!seoConfig?.similarweb_api_key_configured}
									minlength={seoConfig?.similarweb_api_key_configured ? 0 : 8}
									placeholder={seoConfig?.similarweb_api_key_configured ? tr('Kosongkan jika tidak diubah') : 'Masukkan RapidAPI key untuk Similar Web API'}
									bind:value={similarwebKey}
								/>
							</label>
							<p class="text-xs text-muted-foreground">{tr('Ambil key di Dashboard RapidAPI (header')} <code class="rounded bg-secondary px-1">x-rapidapi-key</code>{tr(') setelah Subscribe ke listing Similar Web API.')}</p>
							{#if similarwebMsg.text}
								<div class="flex items-center gap-2 text-sm" style="color: {similarwebMsg.ok ? 'var(--success)' : 'var(--destructive)'};">
									{#if similarwebMsg.ok}<CheckCircle2 size={15} />{:else}<TriangleAlert size={15} />{/if}
									{similarwebMsg.text}
								</div>
							{/if}
							<div class="flex flex-wrap gap-2">
								<button class="app-btn app-btn-primary" type="submit" disabled={similarwebBusy}>{similarwebBusy ? 'Menyimpan…' : tr('Simpan Key')}</button>
								<button class="app-btn app-btn-soft" type="button" onclick={testSimilarWeb} disabled={swTestBusy}>{swTestBusy ? 'Menguji…' : 'Tes koneksi provider'}</button>
							</div>
							{#if swTest}
								<div class="space-y-1 rounded-lg border px-3 py-2 text-xs">
									<div class="flex items-center gap-2 font-medium" style="color: {swTest.ok ? 'var(--success)' : 'var(--destructive)'};">
										{#if swTest.ok}<CheckCircle2 size={14} />{:else}<TriangleAlert size={14} />{/if}
										{#if swTest.provider_status === null}
											{swTest.ket ?? swTest.detail ?? ''}
										{:else}
											Provider {swTest.provider_status} — {swTest.ket ?? swTest.detail ?? ''}
										{/if}
									</div>
									{#if swTest.domain}
										<p class="text-muted-foreground">
											{tr('Domain uji')}: <code class="rounded bg-secondary px-1">{swTest.domain}</code>
											{#if swTest.visits != null}· {tr('Kunjungan terbaca')}: <strong>{Math.round(swTest.visits ?? 0).toLocaleString('id-ID')}</strong>{/if}
											{#if swTest.snapshot_date}· Snapshot {swTest.snapshot_date}{/if}
										</p>
									{/if}
									{#if swTest.payload_raw}
										<div class="text-muted-foreground">
											<p>{tr('Jawaban provider (mentah)')}:</p>
											<!-- JSON penuh bisa sangat panjang; dibatasi tingginya supaya
											     kartu tidak memanjang dan tombol lain tetap terjangkau. -->
											<pre class="mt-1 max-h-52 overflow-auto rounded bg-secondary p-2"><code class="break-all whitespace-pre-wrap">{swTest.payload_raw}</code></pre>
										</div>
									{/if}
								</div>
							{/if}
						</form>
					</div>

					<hr class="border-t" />

					<!-- SERPens -->
					<div class="space-y-3">
						<div class="flex flex-wrap items-center gap-2">
							<Sparkles size={18} />
							<h2 class="text-base font-semibold">SERPens — AI Citation (RapidAPI)</h2>
							<Badge tone={seoConfig?.serpens_api_key_configured ? 'success' : 'warning'} label={seoConfig?.serpens_api_key_configured ? 'Terhubung' : tr('Belum Dikonfigurasi')} />
						</div>
						<p class="text-sm text-muted-foreground">{tr("API untuk indikator")}<strong>AI Citation</strong>{tr("di Katalog Media: mendeteksi apakah domain media terdeteksi sebagai rujukan Google AI Overview. Opsional — indikator nonaktif sampai key diisi.")}</p>
						<form class="space-y-3" onsubmit={saveSerpens}>
							<label class="block">
								<span class="text-sm font-medium">SERPens RapidAPI Key</span>
								<input
									class="app-input mt-1 w-full"
									type="password"
									required={!seoConfig?.serpens_api_key_configured}
									minlength={seoConfig?.serpens_api_key_configured ? 0 : 8}
									placeholder={seoConfig?.serpens_api_key_configured ? tr('Kosongkan jika tidak diubah') : 'Masukkan SERPens RapidAPI key'}
									bind:value={serpensKey}
								/>
							</label>
							{#if serpensMsg.text}
								<div class="flex items-center gap-2 text-sm" style="color: {serpensMsg.ok ? 'var(--success)' : 'var(--destructive)'};">
									{#if serpensMsg.ok}<CheckCircle2 size={15} />{:else}<TriangleAlert size={15} />{/if}
									{serpensMsg.text}
								</div>
							{/if}
							<button class="app-btn app-btn-primary" type="submit" disabled={serpensBusy}>{serpensBusy ? 'Menyimpan…' : tr('Simpan SERPens Key')}</button>
						</form>
					</div>
				</div>
			{/if}
		</div>
	{/if}

	<!-- API Pagespeed -->
	{#if tab === 'pagespeed'}
		<div class="app-card mt-3 p-4">
			{#if psLoading && !psConfig}
				<p class="text-sm text-muted-foreground">{tr("Memuat konfigurasi Pagespeed…")}</p>
			{:else}
				<div class="flex flex-wrap items-center gap-2">
					<Gauge size={18} />
					<h2 class="text-base font-semibold">API Pagespeed (Google)</h2>
					<Badge tone={psConfig?.api_key_configured ? 'success' : 'warning'} label={psConfig?.api_key_configured ? 'Terhubung' : tr('Belum Dikonfigurasi')} />
				</div>
				<p class="mt-2 text-sm text-muted-foreground">{tr("Key Google PageSpeed Insights untuk tab")}<strong>Kecepatan</strong>{tr("di Tool SEO Mitra. Disimpan terenkripsi server-side, tidak pernah tampil plaintext.")}</p>
				<div class="mt-3 rounded-lg border px-3 py-2 text-xs text-muted-foreground">
					Endpoint: <code class="rounded bg-secondary px-1">https://www.googleapis.com/pagespeedonline/v5/runPagespeed</code>
					{psConfig?.api_key_hint ? ` · Key ${psConfig.api_key_hint}` : ''}
				</div>
				<form class="mt-4 space-y-3" onsubmit={savePagespeed}>
					<label class="block">
						<span class="text-sm font-medium">Google PageSpeed API Key</span>
						<span class="block text-xs text-muted-foreground">
							{psConfig?.api_key_configured ? tr('Terkonfigurasi ✓ — kosongkan jika tidak diubah') : tr('Belum diisi — dapatkan di Google Cloud Console (PageSpeed Insights API)')}
						</span>
						<input
							class="app-input mt-1 w-full"
							type="password"
							placeholder={psConfig?.api_key_configured ? tr('Kosongkan jika tidak diubah') : 'Masukkan API key'}
							bind:value={psKey}
						/>
					</label>
					{#if psMsg.text}
						<div class="flex items-center gap-2 text-sm" style="color: {psMsg.ok ? 'var(--success)' : 'var(--destructive)'};">
							{#if psMsg.ok}<CheckCircle2 size={15} />{:else}<TriangleAlert size={15} />{/if}
							{psMsg.text}
						</div>
					{/if}
					<button class="app-btn app-btn-primary" type="submit" disabled={psBusy}>{psBusy ? 'Menyimpan…' : tr('Simpan API Key')}</button>
				</form>
				<hr class="my-4 border-t" />
				<h3 class="text-sm font-semibold">Test Koneksi</h3>
				<p class="mt-1 text-sm text-muted-foreground">{tr("Uji koneksi ke Google PageSpeed dengan URL https://example.com. Pastikan key valid dan quota tersedia.")}</p>
				<div class="mt-2 flex flex-wrap items-center gap-3">
					<button class="app-btn app-btn-soft" type="button" disabled={psTestBusy} onclick={() => void testPagespeed()}>
						{psTestBusy ? 'Menguji…' : 'Test Koneksi API'}
					</button>
					{#if psTestMsg.text}
						<span class="text-sm" style="color: {psTestMsg.ok ? 'var(--success)' : 'var(--destructive)'};">
							{psTestMsg.text}{psTestLatency != null ? ` (${psTestLatency}ms)` : ''}
						</span>
					{/if}
				</div>
				{#if !psConfig?.api_key_configured}
					<p class="mt-3 rounded-lg border px-3 py-6 text-center text-sm text-muted-foreground">{tr("Belum ada key tersimpan — fitur Kecepatan nonaktif sampai key diisi.")}</p>
				{/if}
			{/if}
		</div>
	{/if}

	<!-- Email / SMTP -->
	{#if tab === 'smtp'}
		<div class="mt-3 grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_220px]">
			<div>
				{#if smtpView === 'config'}
					<div class="app-card p-4">
			{#if smtpLoading && !smtp}
				<p class="text-sm text-muted-foreground">{tr("Memuat konfigurasi SMTP…")}</p>
			{:else}
				<Feedback error={smtpError} />
				<div class="flex flex-wrap items-center gap-2">
					<Mail size={18} />
					<h2 class="text-base font-semibold">{tr("Email & SMTP")}</h2>
					<Badge tone={smtp?.enabled ? 'success' : 'warning'} label={smtp?.enabled ? tr('Aktif') : tr('Nonaktif')} />
					{#if smtp?.smtp_configured}<Badge tone="success" label="Terkonfigurasi" />{/if}
				</div>
				<p class="mt-2 text-sm text-muted-foreground">{tr("Konfigurasi SMTP relay Kirim.id untuk email transaksional. Password disimpan terenkripsi (AES-256-GCM) dan tidak pernah ditampilkan kembali.")}</p>

				<details class="mt-3 rounded-lg border px-3 py-2 text-sm">
					<summary class="cursor-pointer font-medium">{tr("Setelan Kirim.id")}</summary>
					<ul class="mt-2 list-disc space-y-1 pl-5 text-xs text-muted-foreground">
						<li>Host <code>mail.kirim.id</code> — port <code>587</code> (STARTTLS) atau <code>465</code> (TLS, centang Secure).</li>
						<li>Username <code>kir</code> + 6 angka dan password 32 karakter dari Settings → Kredensial SMTP.</li>
						<li>{tr("Domain alamat From wajib sudah berstatus aktif di akun Kirim.id.")}</li>
						<li>{tr("Batas: 50 penerima dan 10 MB per email; kecepatan mengikuti paket akun.")}</li>
					</ul>
				</details>

				{#if smtp?.last_test_at}
					<div class="mt-3 rounded-lg border px-3 py-2 text-sm" style="border-color: {smtp.last_test_ok ? 'var(--success)' : 'var(--warning)'};">
						Test terakhir: {smtp.last_test_ok ? tr('Berhasil') : tr('Gagal')} — {smtp.last_test_message ?? '-'}
						<span class="text-xs text-muted-foreground">({fmtDateTime(smtp.last_test_at)})</span>
					</div>
				{/if}

				<hr class="my-3 border-t" />

				<form class="space-y-3" onsubmit={saveSmtp}>
					<div class="flex flex-wrap gap-3">
						<label class="block min-w-[220px] flex-1">
							<span class="text-sm font-medium">SMTP Host</span>
							<span class="block text-xs text-muted-foreground">mail.kirim.id</span>
							<input class="app-input mt-1 w-full" placeholder="mail.kirim.id" bind:value={smtpHost} />
						</label>
						<label class="block w-[120px]">
							<span class="text-sm font-medium">Port</span>
							<span class="block text-xs text-muted-foreground">{tr("587 (STARTTLS) atau 465 (TLS)")}</span>
							<input class="app-input mt-1 w-full" type="number" placeholder="587" bind:value={smtpPort} />
						</label>
						<label class="block">
							<span class="text-sm font-medium">Secure (TLS)</span>
							<span class="mt-2 flex items-center gap-2">
								<input type="checkbox" bind:checked={smtpSecure} />
								<span class="text-sm text-muted-foreground">{smtpSecure ? 'Ya (port 465)' : tr('Tidak (port 587 STARTTLS)')}</span>
							</span>
						</label>
					</div>
					<div class="flex flex-wrap gap-3">
						<label class="block min-w-[220px] flex-1">
							<span class="text-sm font-medium">Username / Login</span>
							<span class="block text-xs text-muted-foreground">kir + 6 angka</span>
							<input class="app-input mt-1 w-full" placeholder="kir123456" autocomplete="off" bind:value={smtpUsername} />
						</label>
						<label class="block min-w-[220px] flex-1">
							<span class="text-sm font-medium">{tr("SMTP Password / Key")}</span>
							<span class="block text-xs text-muted-foreground">
								{smtp?.password_configured ? tr('Terkonfigurasi ✓ — kosongkan jika tidak diubah') : tr('Belum diisi')}
							</span>
							<input
								class="app-input mt-1 w-full"
								type="password"
								autocomplete="new-password"
								placeholder={smtp?.password_configured ? tr('Kosongkan jika tidak diubah') : tr('32 karakter dari dashboard Kirim.id')}
								bind:value={smtpPassword}
							/>
						</label>
					</div>
					<div class="flex flex-wrap gap-3">
						<label class="block min-w-[220px] flex-1">
							<span class="text-sm font-medium">From Name</span>
							<input class="app-input mt-1 w-full" placeholder="Tauthub" bind:value={smtpFromName} />
						</label>
						<label class="block min-w-[220px] flex-1">
							<span class="text-sm font-medium">{tr("From Email")}</span>
							<input class="app-input mt-1 w-full" type="email" placeholder="no-reply@tauthub.com" bind:value={smtpFromEmail} />
						</label>
					</div>
					<hr class="border-t" />
					<div class="flex flex-wrap items-center justify-between gap-3">
						<div>
							<p class="text-sm font-medium">{tr("Transactional Email")}</p>
							<p class="text-xs text-muted-foreground">{tr("Matikan hanya untuk maintenance — security email tetap terkirim.")}</p>
						</div>
						<input type="checkbox" bind:checked={smtpEnabled} />
					</div>
					{#if smtpMsg.text}
						<div class="flex items-center gap-2 text-sm" style="color: {smtpMsg.ok ? 'var(--success)' : 'var(--destructive)'};">
							{#if smtpMsg.ok}<CheckCircle2 size={15} />{:else}<TriangleAlert size={15} />{/if}
							{smtpMsg.text}
						</div>
					{/if}
					<button class="app-btn app-btn-primary" type="submit" disabled={smtpBusy}>{smtpBusy ? 'Menyimpan…' : tr('Simpan Konfigurasi')}</button>
				</form>

				<hr class="my-4 border-t" />
				<h3 class="text-sm font-semibold">Webhook bounce Kirim.id</h3>
				<p class="mt-1 text-xs text-muted-foreground">
					Mencatat alamat email yang memantul permanen supaya tidak dikirimi lagi. Daftarkan URL di bawah di Kirim.id
					(event bounce dan complaint), lalu tempel secret yang diberikan Kirim.id di sini.
				</p>
				<form class="mt-3 space-y-3" onsubmit={saveWebhook}>
					<label class="block">
						<span class="text-sm font-medium">URL webhook</span>
						<input class="app-input mt-1 w-full" readonly value={hookInfo?.url ?? ''} onfocus={(e) => e.currentTarget.select()} />
					</label>
					<label class="block">
						<span class="text-sm font-medium">Secret webhook</span>
						<span class="block text-xs text-muted-foreground">
							{#if hookInfo?.configured}
								{hookInfo.source === 'env' ? 'Terkonfigurasi dari environment server' : 'Terkonfigurasi'} ✓ — kosongkan dan simpan untuk menghapus
							{:else}
								Belum diisi — webhook menolak semua kiriman
							{/if}
						</span>
						<input
							class="app-input mt-1 w-full"
							type="password"
							autocomplete="new-password"
							placeholder={hookInfo?.configured ? 'Isi untuk mengganti secret' : 'Tempel secret dari Kirim.id'}
							bind:value={hookSecret}
						/>
					</label>
					{#if hookMsg.text}
						<div class="flex items-center gap-2 text-sm" style="color: {hookMsg.ok ? 'var(--success)' : 'var(--destructive)'};">
							{#if hookMsg.ok}<CheckCircle2 size={15} />{:else}<TriangleAlert size={15} />{/if}
							{hookMsg.text}
						</div>
					{/if}
					<button class="app-btn app-btn-primary" type="submit" disabled={hookBusy}>{hookBusy ? 'Menyimpan…' : 'Simpan Secret'}</button>
				</form>

				<hr class="my-4 border-t" />
				<h3 class="text-sm font-semibold">Test SMTP</h3>
				<div class="mt-2 flex flex-wrap items-center gap-3">
					<button class="app-btn app-btn-soft" type="button" disabled={smtpTestBusy} onclick={() => void testSmtp()}>
						{smtpTestBusy ? 'Menguji…' : 'Test Connection'}
					</button>
					{#if smtpTestMsg.text}
						<span class="text-sm" style="color: {smtpTestMsg.ok ? 'var(--success)' : 'var(--destructive)'};">{smtpTestMsg.text}</span>
					{/if}
				</div>
				<div class="mt-3 flex flex-wrap items-center gap-3">
					<input class="app-input min-w-[200px] flex-1" type="email" placeholder="admin@example.com" bind:value={testEmail} />
					<button class="app-btn app-btn-soft" type="button" disabled={smtpSendBusy} onclick={() => void sendTestEmail()}>
						{smtpSendBusy ? 'Mengirim…' : 'Send Test Email'}
					</button>
				</div>
				{#if smtpSendMsg.text}
					<p class="mt-1 text-sm" style="color: {smtpSendMsg.ok ? 'var(--success)' : 'var(--destructive)'};">{smtpSendMsg.text}</p>
				{/if}
				{#if !smtp?.smtp_configured}
					<p class="mt-3 rounded-lg border px-3 py-6 text-center text-sm text-muted-foreground">{tr("SMTP belum dikonfigurasi — Tauthub belum dapat mengirim email transactional.")}</p>
				{/if}
			{/if}
					</div>
				{:else if smtpView === 'log'}
					<EmailLogPanel />
				{:else}
					<EmailTemplatesPanel />
				{/if}
			</div>
			<aside class="app-card p-2 lg:sticky lg:top-20">
				<nav class="flex gap-1 lg:flex-col" aria-label={tr('Bagian Email & SMTP')}>
					<button
						type="button"
						class="app-btn !min-h-8 !justify-start !px-3 !text-xs"
						class:app-btn-primary={smtpView === 'config'}
						class:app-btn-soft={smtpView !== 'config'}
						onclick={() => selectSmtpView('config')}>{tr('Konfigurasi')}</button
					>
					<button
						type="button"
						class="app-btn !min-h-8 !justify-start !px-3 !text-xs"
						class:app-btn-primary={smtpView === 'templates'}
						class:app-btn-soft={smtpView !== 'templates'}
						onclick={() => selectSmtpView('templates')}>{tr('Template')}</button
					>
					<button
						type="button"
						class="app-btn !min-h-8 !justify-start !px-3 !text-xs"
						class:app-btn-primary={smtpView === 'log'}
						class:app-btn-soft={smtpView !== 'log'}
						onclick={() => selectSmtpView('log')}>{tr('Log Email')}</button
					>
				</nav>
			</aside>
		</div>
	{/if}

	<!-- Integrasi GSC (panel yang sama dipakai route /admin/integrasi-gsc) -->
	{#if tab === 'gsc'}
		<GscPanel />
	{/if}

	<!-- GA4 Config (panel yang sama dipakai route /admin/ga4-configs) -->
	{#if tab === 'ga4'}
		<Ga4Panel />
	{/if}

	<!-- WA Gateway (panel yang sama dipakai route /admin/wa-gateway) -->
	{#if tab === 'wa'}
		<WaGatewayPanel />
	{/if}

	<!-- Turnstile -->
	{#if tab === 'domain'}
		<div class="mt-4"><DomainBlocksPanel /></div>
	{/if}

	{#if tab === 'turnstile'}
		<div class="app-card mt-3 p-4">
			{#if tsLoading && !turnstile}
				<p class="text-sm text-muted-foreground">{tr("Memuat konfigurasi Turnstile…")}</p>
			{:else}
				<Feedback error={tsError} />
				<div class="flex flex-wrap items-center gap-2">
					<ShieldCheck size={18} />
					<h2 class="text-base font-semibold">Cloudflare Turnstile</h2>
					<Badge tone={tsEnabled ? 'success' : 'warning'} label={tsEnabled ? tr('Aktif') : tr('Nonaktif')} />
					{#if turnstile?.secret_configured}<Badge tone="success" label="Secret Terkonfigurasi" />{/if}
				</div>
				<p class="mt-2 text-sm text-muted-foreground">{tr("Proteksi anti-bot untuk")}<strong>{tr("Login, Daftar, dan Lupa Password")}</strong>{tr("di Tauthub. Saat aktif, user wajib menyelesaikan verifikasi Turnstile sebelum submit. Secret key disegel server-side dan tidak pernah tampil.")}</p>
				<div class="mt-3 rounded-lg border px-3 py-2 text-xs text-muted-foreground">
					Setup: dashboard Cloudflare → <strong>Turnstile</strong> → buat site → salin <strong>Site Key</strong> &
					<strong>Secret Key</strong>. Di <strong>Allowed Domains</strong> tambahkan <code>app.tauthub.com</code>{tr("(dan")}<code>localhost:3000</code>{tr("untuk dev).")}</div>
				<form class="mt-4 space-y-3" onsubmit={saveTurnstile}>
					<label class="block">
						<span class="text-sm font-medium">Site Key</span>
						<span class="block text-xs text-muted-foreground">Publik — aman ditampilkan ke browser (diperlukan widget).</span>
						<input class="app-input mt-1 w-full" placeholder="0x4AAAAAAA..." bind:value={tsSiteKey} />
					</label>
					<label class="block">
						<span class="text-sm font-medium">Secret Key</span>
						<span class="block text-xs text-muted-foreground">
							{turnstile?.secret_configured ? tr('Terkonfigurasi ✓ — kosongkan jika tidak diubah') : tr('Belum diisi — wajib saat pertama kali.')}
						</span>
						<input
							class="app-input mt-1 w-full"
							type="password"
							placeholder={turnstile?.secret_configured ? tr('Kosongkan jika tidak diubah') : 'Masukkan secret key'}
							bind:value={tsSecretKey}
						/>
					</label>
					<label class="flex items-center gap-2">
						<input type="checkbox" bind:checked={tsEnabled} />
						<span class="text-sm">Aktifkan Turnstile (wajib di login, daftar, & lupa password)</span>
					</label>
					{#if tsMsg.text}
						<div class="flex items-center gap-2 text-sm" style="color: {tsMsg.ok ? 'var(--success)' : 'var(--destructive)'};">
							{#if tsMsg.ok}<CheckCircle2 size={15} />{:else}<TriangleAlert size={15} />{/if}
							{tsMsg.text}
						</div>
					{/if}
					<div class="flex flex-wrap items-center gap-3">
						<button class="app-btn app-btn-primary" type="submit" disabled={tsBusy}>{tsBusy ? 'Menyimpan…' : tr('Simpan Konfigurasi')}</button>
						<button class="app-btn app-btn-soft" type="button" disabled={tsTestBusy} onclick={() => void testTurnstile()}>
							{tsTestBusy ? 'Menguji…' : 'Test Koneksi'}
						</button>
						{#if tsTestMsg.text}
							<span class="text-sm" style="color: {tsTestMsg.ok ? 'var(--success)' : 'var(--destructive)'};">{tsTestMsg.text}</span>
						{/if}
					</div>
				</form>
			{/if}
		</div>
	{/if}

	<!-- Google Sign-In -->
	{#if tab === 'google'}
		<div class="app-card mt-3 p-4">
			<div class="flex flex-wrap items-center gap-2">
				<h2 class="text-base font-semibold">Google Sign-In</h2>
				<Badge tone={google?.client_id ? 'success' : 'warning'} label={google?.client_id ? 'Terhubung' : tr('Belum Dikonfigurasi')} />
			</div>
			<p class="mt-2 text-sm text-muted-foreground">
				Callback: <code class="text-xs">{google?.callback_url ?? 'https://app.tauthub.com/v1/auth/google/callback'}</code>
			</p>
			<div class="mt-3 flex items-start gap-2 rounded-lg border px-3 py-2 text-xs" style="border-color: var(--warning);">
				<TriangleAlert size={15} class="mt-0.5 shrink-0" style="color: var(--warning);" />
				<p>{tr("Daftarkan callback di Google Cloud Console → OAuth 2.0 Client → Authorized redirect URIs. Jangan pakai /api/gsc/callback (itu untuk Search Console).")}</p>
			</div>
			<!-- Diagnostik admin-only: alasan rinci dari server, tidak pernah tampil di halaman publik. -->
			{#if google && google.configured === false}
				<div class="mt-3 rounded-lg border px-3 py-2 text-xs" style="border-color: var(--destructive); color: var(--destructive);">
					<p class="font-medium">{tr("Konfigurasi Google belum lengkap.")}</p>
					{#if google.config_error_detail}<p class="mt-1">{google.config_error_detail}</p>{/if}
					{#if google.missing_fields?.length}
						<p class="mt-1">{tr("Field kurang:")} <code>{google.missing_fields.join(', ')}</code></p>
					{/if}
					{#if google.config_error}
						<p class="mt-1">{tr("Kode:")} <code>{google.config_error}</code> · {tr("sumber:")} <code>{google.config_source}</code></p>
					{/if}
				</div>
			{/if}
			<form class="mt-4 space-y-3" onsubmit={saveGoogle}>
				<div class="grid gap-3 md:grid-cols-2">
					<label class="block">
						<span class="text-sm font-medium">Client ID</span>
						<input class="app-input mt-1 w-full" required placeholder="xxxxx.apps.googleusercontent.com" bind:value={googleClientId} />
					</label>
					<label class="block">
						<span class="text-sm font-medium">Client Secret</span>
						<input class="app-input mt-1 w-full" type="password" placeholder={tr("Kosongkan jika tidak diubah")} bind:value={googleClientSecret} />
					</label>
				</div>
				<label class="block">
					<span class="text-sm font-medium">Redirect URI</span>
					<input class="app-input mt-1 w-full" required type="url" placeholder="https://app.tauthub.com/v1/auth/google/callback" bind:value={googleRedirectUri} />
				</label>
				{#if googleMsg.text}
					<div class="flex items-center gap-2 text-sm" style="color: {googleMsg.ok ? 'var(--success)' : 'var(--destructive)'};">
						{#if googleMsg.ok}<CheckCircle2 size={15} />{:else}<TriangleAlert size={15} />{/if}
						{googleMsg.text}
					</div>
				{/if}
				<button class="app-btn app-btn-primary" type="submit" disabled={googleBusy}>{googleBusy ? 'Menyimpan…' : tr('Simpan Konfigurasi')}</button>
			</form>
		</div>
	{/if}

	<!-- Short Link -->
	{#if tab === 'shortlink'}
		<div class="app-card mt-3 p-4">
			{#if shortLoading && !shortDomain && !shortMsg.text}
				<p class="text-sm text-muted-foreground">{tr("Memuat konfigurasi short link…")}</p>
			{:else}
				<div class="flex flex-wrap items-center gap-2">
					<Globe size={18} />
					<h2 class="text-base font-semibold">Domain Short Link (The Writer)</h2>
					<Badge tone={shortDomain ? 'success' : 'warning'} label={shortDomain ? `Aktif — ${shortDomain}` : 'Fallback: domain utama (/t/[kode])'} />
				</div>
				<p class="mt-2 text-sm text-muted-foreground">
					Saat klien memilih mode <strong>Tauthub Tracking Link</strong>, artikel menampilkan{' '}
					<strong>{shortDomain ? `${shortDomain}/[kode]` : '[domain-utama]/t/[kode]'}</strong>{tr("dengan kode maks 5 karakter. Klik tercatat di platform lalu pembaca diarahkan ke URL asli yang bersih (tanpa UTM).")}</p>
				<div class="mt-3 rounded-lg border px-3 py-2 text-xs text-muted-foreground">
					<p class="font-semibold">Tutorial pemasangan domain:</p>
					<p>1. Beli domain (mis. tautan.link) di registrar mana pun.</p>
					<p>2. DNS: A record <code>@</code>{tr("dan")}<code>www</code> → IP VPS TautHub (sama dengan domain utama).</p>
					<p>3. CloudPanel → Sites → Add Site → <strong>Reverse Proxy</strong> → isi domain tersebut → target <code>http://127.0.0.1:3000</code> (port app produksi).</p>
					<p>4. Aktifkan SSL Let&apos;s Encrypt (centang www + redirect http ke https).</p>
					<p>5. Edit vHost site tersebut, tambahkan di blok location: <code>rewrite ^/(.+)$ /t/$1 last;</code>{tr("— agar")}<code>domain/[kode]</code> langsung memakai route redirect existing tanpa /t/.</p>
					<p>6. Isi domain di form bawah ini — semua short link langsung memakainya, tanpa deploy ulang.</p>
				</div>
				<form class="mt-4 space-y-3" onsubmit={saveShortLink}>
					<label class="block">
						<span class="text-sm font-medium">Domain Short Link</span>
						<span class="block text-xs text-muted-foreground">{tr("Hostname saja tanpa https:// — kosongkan untuk fallback domain utama")}</span>
						<input class="app-input mt-1 w-full" placeholder="tautan.link" bind:value={shortInput} />
					</label>
					{#if shortMsg.text}
						<div class="flex items-center gap-2 text-sm" style="color: {shortMsg.ok ? 'var(--success)' : 'var(--destructive)'};">
							{#if shortMsg.ok}<CheckCircle2 size={15} />{:else}<TriangleAlert size={15} />{/if}
							{shortMsg.text}
						</div>
					{/if}
					<button class="app-btn app-btn-primary" type="submit" disabled={shortBusy}>{shortBusy ? 'Menyimpan…' : tr('Simpan Domain')}</button>
				</form>
				{#if !shortDomain}
					<p class="mt-3 rounded-lg border px-3 py-6 text-center text-sm text-muted-foreground">{tr("Belum ada domain custom — short link memakai domain utama via /t/[kode]. Semua fitur tetap berjalan.")}</p>
				{/if}
			{/if}
		</div>
	{/if}

	<!-- Crawlhub -->
	{#if tab === 'crawlhub'}
		<div class="space-y-4">
			<div class="flex flex-wrap gap-2">
				<button class="app-btn {crawlTab === 'endpoint' ? 'app-btn-primary' : 'app-btn-soft'} !min-h-8 !px-3 !text-xs" type="button" onclick={() => (crawlTab = 'endpoint')}>Endpoint & Uji Crawl</button>
				<button
					class="app-btn {crawlTab === 'monitoring' ? 'app-btn-primary' : 'app-btn-soft'} !min-h-8 !px-3 !text-xs"
					type="button"
					onclick={() => {
						crawlTab = 'monitoring';
						void loadMonitor();
					}}
				>{tr("Monitoring Velocity")}</button>
			</div>

			{#if crawlTab === 'endpoint'}
				<div class="app-card p-4">
					<h2 class="text-base font-semibold">Endpoint Crawlhub</h2>
					<p class="mt-1 text-sm text-muted-foreground">
						Domain service crawler. Bisa diganti kapan saja tanpa deploy; env <code>CRAWLHUB_BASE_URL</code>{tr("hanya dipakai sebagai nilai awal sebelum disimpan di sini.")}</p>
					{#if chLoading}
						<p class="mt-3 text-sm text-muted-foreground">{tr("Memuat konfigurasi…")}</p>
					{:else if chLoadError}
						<div class="mt-3 rounded-lg border px-3 py-2 text-sm" style="color: var(--destructive);">{chLoadError}</div>
					{:else}
						<form class="mt-3 space-y-3" onsubmit={saveCrawlhub}>
							<label class="block">
								<span class="text-sm font-medium">Base URL</span>
								<input class="app-input mt-1 w-full" placeholder="https://crawl.tauthub.com" bind:value={chBaseUrl} />
							</label>
							<div class="grid gap-3 sm:grid-cols-2">
								<label class="block">
									<span class="text-sm font-medium">HMAC secret {chConfig?.hmac_secret_configured ? '(terisi)' : ''}</span>
									<input
										class="app-input mt-1 w-full"
										type="password"
										placeholder={chConfig?.hmac_secret_configured ? 'biarkan kosong bila tidak diganti' : 'wajib diisi'}
										bind:value={chHmac}
									/>
								</label>
								<label class="block">
									<span class="text-sm font-medium">Ingest secret {chConfig?.ingest_secret_configured ? '(terisi)' : ''}</span>
									<input class="app-input mt-1 w-full" type="password" placeholder={tr("dipakai route penerima hasil di Writer")} bind:value={chIngest} />
								</label>
							</div>
							<div class="flex flex-wrap items-center gap-3">
								<button class="app-btn app-btn-primary" type="submit" disabled={chSaving}>{chSaving ? 'Menyimpan…' : tr('Simpan')}</button>
								<Badge tone={chConfig?.configured ? 'success' : 'warning'} label={chConfig?.configured ? 'siap dipakai' : 'belum lengkap'} />
								<span class="text-xs text-muted-foreground">Secret disegel server-side, tidak pernah dikembalikan ke browser.</span>
							</div>
						</form>
					{/if}
					{#if chSaveError}<div class="mt-3 text-sm" style="color: var(--destructive);">{chSaveError}</div>{/if}
					{#if chNotice}<div class="mt-3 text-sm" style="color: var(--success);">{chNotice}</div>{/if}
				</div>

				<div class="app-card p-4">
					<h2 class="text-base font-semibold">Uji Crawl</h2>
					<p class="mt-1 text-sm text-muted-foreground">{tr("Uji satu domain tanpa menyentuh database dan tanpa mendaftarkannya sebagai target. Cocok untuk memastikan situs mitra bisa dibaca sebelum masuk katalog.")}</p>
					<form class="mt-3 flex flex-wrap items-end gap-3" onsubmit={runCrawlTest}>
						<label class="block min-w-[280px] flex-1">
							<span class="text-sm font-medium">Domain</span>
							<input class="app-input mt-1 w-full" placeholder={tr("https://contoh-media.id")} bind:value={chDomain} />
						</label>
						<button class="app-btn app-btn-primary" type="submit" disabled={chTesting || !chConfig?.configured}>
							{chTesting ? 'Mencrawl…' : 'Tes Crawl'}
						</button>
					</form>
					{#if chTestError}<div class="mt-3 text-sm" style="color: var(--destructive);">{chTestError}</div>{/if}
					{#if chTestResult}
						<div class="mt-3 space-y-3">
							<div class="flex flex-wrap items-center gap-3">
								<Badge
									tone={chTestResult.status === 'ok' ? 'success' : chTestResult.status === 'blocked' ? 'danger' : 'warning'}
									label={CRAWL_STATUS_LABEL[chTestResult.status] ?? chTestResult.status}
								/>
								{#if chTestResult.durationMs !== undefined}
									<span class="text-xs text-muted-foreground">{chTestResult.durationMs} ms</span>
								{/if}
							</div>
							<div class="grid gap-3 sm:grid-cols-2">
								<div>
									<p class="text-xs text-muted-foreground">Artikel / 7 hari</p>
									<p class="text-sm">{chTestResult.articles7d === null ? 'tidak terukur' : chTestResult.articles7d}</p>
								</div>
								<div>
									<p class="text-xs text-muted-foreground">Keyakinan</p>
									<p class="text-sm">{CRAWL_CONFIDENCE_LABEL[chTestResult.confidence] ?? chTestResult.confidence}</p>
								</div>
								<div>
									<p class="text-xs text-muted-foreground">Sumber</p>
									<p class="text-sm">{chTestResult.source ? (CRAWL_SOURCE_LABEL[chTestResult.source] ?? chTestResult.source) : '—'}</p>
								</div>
								<div>
									<p class="text-xs text-muted-foreground">{tr("Jumlah request")}</p>
									<p class="text-sm">{chTestResult.methodDetail?.requests ?? '—'}</p>
								</div>
							</div>
							{#if chTestResult.errorMessage}
								<div class="rounded-lg border px-3 py-2 text-sm" style="color: {chTestResult.status === 'blocked' ? 'var(--destructive)' : 'var(--muted-foreground)'};">
									{chTestResult.errorMessage}
									{chTestResult.methodDetail?.blockedReason === 'cloudflare_challenge'
										? ' Minta pemilik situs menambahkan WAF custom rule dengan aksi Skip untuk User-Agent CrawlhubBot dan IP egress crawler.'
										: ''}
								</div>
							{/if}
							{#if chTestResult.methodDetail?.notes?.length}
								<div>
									<p class="text-xs text-muted-foreground">Log langkah</p>
									<pre class="mt-1 whitespace-pre-wrap text-xs">{chTestResult.methodDetail.notes.join('\n')}</pre>
								</div>
							{/if}
						</div>
					{/if}
				</div>

				<div class="app-card p-4">
					<h2 class="text-base font-semibold">{tr("Status Media")}</h2>
					<p class="mt-1 text-sm text-muted-foreground">{tr("Lihat hasil crawl terakhir untuk satu media yang sudah terdaftar sebagai target.")}</p>
					<form class="mt-3 flex flex-wrap items-end gap-3" onsubmit={checkCrawlTarget}>
						<label class="block min-w-[280px] flex-1">
							<span class="text-sm font-medium">{tr("ID media")}</span>
							<input class="app-input mt-1 w-full" placeholder={tr("uuid media dari tabel media")} bind:value={chMediaId} />
						</label>
						<button class="app-btn app-btn-soft" type="submit" disabled={chChecking || !chConfig?.configured}>
							{chChecking ? 'Memeriksa…' : 'Cek status'}
						</button>
					</form>
					{#if chTargetError}<div class="mt-3 text-sm" style="color: var(--destructive);">{chTargetError}</div>{/if}
					{#if chTarget}
						<div class="mt-3 grid gap-3 sm:grid-cols-2">
							<div>
								<p class="text-xs text-muted-foreground">Website</p>
								<p class="text-sm">{chTarget.website_url}</p>
							</div>
							<div>
								<p class="text-xs text-muted-foreground">{tr("Status terakhir")}</p>
								<p class="text-sm">{chTarget.last_status ?? 'belum pernah'}</p>
							</div>
							<div>
								<p class="text-xs text-muted-foreground">Artikel / 7 hari</p>
								<p class="text-sm">{chTarget.last_run?.articles_7d == null ? 'belum terukur' : chTarget.last_run.articles_7d}</p>
							</div>
							<div>
								<p class="text-xs text-muted-foreground">Terakhir dicrawl</p>
								<p class="text-sm">{chTarget.last_crawled_at ? fmtDateTime(chTarget.last_crawled_at) : '—'}</p>
							</div>
							{#if chTarget.last_run?.blocked_reason === 'cloudflare_challenge'}
								<div class="sm:col-span-2 rounded-lg border px-3 py-2 text-sm" style="border-color: var(--warning);">{tr("Diblokir Cloudflare, bukan gagal biasa. Media ini tetap tercatat “belum terukur”, tidak dilabeli tidak produktif.")}</div>
							{/if}
						</div>
					{/if}
				</div>

				<div class="app-card p-4">
					<h2 class="text-base font-semibold">API</h2>
					<p class="mt-1 text-sm text-muted-foreground">{tr("Kontrak yang dipakai Writer untuk berbicara dengan Crawlhub. Ini referensi, bukan yang perlu diisi.")}</p>
					{#if chConfig?.api}
						<div class="mt-3 space-y-4 text-sm">
							<div>
								<p class="font-semibold">Tanda tangan</p>
								<p>Header <code>{chConfig.api.signature.headers.join(' + ')}</code></p>
								<p class="text-muted-foreground">{chConfig.api.signature.format}, toleransi {chConfig.api.signature.maxSkewSeconds} detik.</p>
							</div>
							<hr class="border-t" />
							<div>
								<p class="font-semibold">Writer → Crawlhub</p>
								<div class="mt-2 space-y-2">
									{#each chConfig.api.outbound as endpoint (`${endpoint.method}-${endpoint.path}`)}
										<div>
											<p><Badge tone="neutral" label={endpoint.method} /> <code>{endpoint.path}</code></p>
											<p class="text-xs text-muted-foreground">{endpoint.purpose}{endpoint.body ? ` Body: ${endpoint.body}` : ''}</p>
										</div>
									{/each}
								</div>
							</div>
							<hr class="border-t" />
							<div>
								<p class="font-semibold">Crawlhub → Writer</p>
								<div class="mt-2 space-y-2">
									{#each chConfig.api.inbound as endpoint (`${endpoint.method}-${endpoint.path}`)}
										<div>
											<p><Badge tone="neutral" label={endpoint.method} /> <code>{endpoint.path}</code></p>
											<p class="text-xs text-muted-foreground">{endpoint.purpose}</p>
										</div>
									{/each}
								</div>
							</div>
						</div>
					{:else}
						<p class="mt-3 text-sm text-muted-foreground">{tr("Memuat referensi API…")}</p>
					{/if}
				</div>
			{:else}
				<div class="app-card p-4">
					<div class="flex flex-wrap items-center justify-between gap-3">
						<div>
							<h2 class="text-base font-semibold">{tr("Monitoring Velocity")}</h2>
							<p class="mt-1 text-sm text-muted-foreground">{tr("Hasil crawl otomatis per media mitra aktif. Scheduler mengantre sinkronisasi tiap menit; hasil disegarkan tiap 7 hari (CRAWLHUB_STALE_DAYS), media yang belum punya angka dicek ulang tiap 24 jam, dan gagal diulang setelah 24 jam (maks 2x).")}</p>
						</div>
						<button class="app-btn app-btn-primary" type="button" disabled={syncBusy || monitorLoading} onclick={() => void syncCrawlhub()}>
							{syncBusy ? 'Menyinkronkan…' : 'Sinkronkan Sekarang'}
						</button>
					</div>

					{#if chNotice}<p class="mt-2 text-sm" style="color: var(--success);">{chNotice}</p>{/if}

					{#if monitor}
						<div class="mt-3 flex flex-wrap gap-3">
							{#each [
								{ label: 'Media Dipantau', value: monitor.summary.total },
								{ label: 'Terdaftar', value: monitor.summary.registered },
								{ label: tr('Menunggu Antrean'), value: monitor.summary.pending },
								{ label: tr('Gagal'), value: monitor.summary.failed },
								{ label: tr('Belum Terdaftar'), value: monitor.summary.unregistered }
							] as s (s.label)}
								<div class="app-card min-w-[130px] flex-1 p-3">
									<p class="text-xs text-muted-foreground">{s.label}</p>
									<p class="text-xl font-bold">{s.value}</p>
								</div>
							{/each}
						</div>
					{/if}

					{#if monitorError}<p class="mt-2 text-sm" style="color: var(--destructive);">{monitorError}</p>{/if}

					<div class="mt-4 flex flex-wrap items-center gap-2">
						{#each [
							['all', tr('Semua')],
							['registered', 'Sudah Terdaftar'],
							['problem', 'Bermasalah'],
							['stale', tr('Kadaluarsa / Belum Dicek')],
							['unregistered', tr('Belum Terdaftar')]
						] as [value, label] (value)}
							<button
								class="app-btn {monitorFilter === value ? 'app-btn-primary' : 'app-btn-soft'} !min-h-7 !px-2.5 !text-xs"
								type="button"
								onclick={() => {
									monitorFilter = value as typeof monitorFilter;
									monitorOffset = 0;
									void loadMonitor();
								}}
							>{label}</button>
						{/each}
						<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" disabled={monitorLoading} onclick={() => void loadMonitor()}>
							<RefreshCw size={12} /> Segarkan
						</button>
					</div>

					{#if monitorLoading && !monitor}
						<p class="mt-3 text-sm text-muted-foreground">{tr("Memuat status crawl…")}</p>
					{:else if monitor && monitor.targets.length === 0}
						<p class="mt-3 text-sm text-muted-foreground">{tr("Tidak ada media pada filter ini. Tekan \"Sinkronkan Sekarang\" untuk mendaftarkan media mitra aktif ke Crawlhub.")}</p>
					{:else if monitor && monitor.targets.length > 0}
						<div class="mt-3 overflow-x-auto">
							<table class="w-full text-sm" style="min-width: 1080px;">
								<thead>
									<tr class="text-xs text-muted-foreground">
										<th class="px-3 py-2 text-left font-medium">{tr("Media")}</th>
										<th class="px-3 py-2 text-left font-medium">Artikel / 7 Hari</th>
										<th class="px-3 py-2 text-left font-medium">Sumber</th>
										<th class="px-3 py-2 text-left font-medium">Terakhir Berhasil</th>
										<th class="px-3 py-2 text-left font-medium">{tr("Gagal Berturut")}</th>
										<th class="px-3 py-2 text-left font-medium">Error Terakhir</th>
										<th class="px-3 py-2 text-left font-medium">Aksi</th>
									</tr>
								</thead>
								<tbody>
									{#each monitor.targets as row (row.media_id)}
										<tr class="border-t">
											<td class="px-3 py-2">
												<p class="font-medium">{row.name}</p>
												<p class="text-xs text-muted-foreground">{row.website_url}</p>
											</td>
											<td class="px-3 py-2">
												{#if row.articles_7d == null}
													<span class="text-xs" style="color: {row.last_status === 'error' || row.consecutive_failures > 0 ? 'var(--destructive)' : 'var(--muted-foreground)'};">
														{crawlEmptyReason(row)}
													</span>
												{:else}
													<span class="font-bold">{row.articles_7d}</span>
													{#if row.stale}
														<span class="text-xs text-muted-foreground">({tr('basi')})</span>
													{/if}
												{/if}
											</td>
											<td class="px-3 py-2 text-xs">{row.source ? (CRAWL_SOURCE_LABEL[row.source] ?? row.source) : '—'}</td>
											<td class="px-3 py-2 text-xs" style="color: {row.stale ? 'var(--warning)' : 'var(--muted-foreground)'};">
												{row.last_crawled_at ? fmtDateTime(row.last_crawled_at) : '—'}
											</td>
											<td class="px-3 py-2">
												{#if row.consecutive_failures > 0}
													<Badge tone="danger" label="{row.consecutive_failures}x" />
												{:else}
													<span class="text-xs text-muted-foreground">0</span>
												{/if}
											</td>
											<td class="px-3 py-2">
												<span class="block max-w-[220px] truncate text-xs text-muted-foreground" title={row.last_error ?? undefined}>{row.last_error ?? '—'}</span>
											</td>
											<td class="px-3 py-2">
												<button
													class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs"
													type="button"
													disabled={refreshingId === row.media_id || syncBusy}
													onclick={() => void recrawlRow(row)}
												>{refreshingId === row.media_id ? 'Meminta…' : 'Crawl Ulang'}</button>
											</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
						<Pagination
							total={monitor.total}
							limit={20}
							offset={monitorOffset}
							onChange={(next) => {
								monitorOffset = next;
								void loadMonitor();
							}}
						/>
					{/if}
				</div>
			{/if}
		</div>
	{/if}

	<!-- Server -->
	{#if tab === 'server'}
		<div class="app-card mt-3 p-4">
			{#if serverLoading && !serverCfg}
				<p class="text-sm text-muted-foreground">{tr("Memuat konfigurasi server…")}</p>
			{:else}
				<form class="space-y-3" onsubmit={saveServer}>
					<div>
						<h2 class="text-base font-semibold">IP Publik Server</h2>
						<p class="mt-1 text-sm text-muted-foreground">{tr("IP ini ditampilkan ke mitra ketika Cloudflare/WAF mereka memblokir pengecekan domain, supaya mitra bisa memasukkannya ke daftar izin. Disimpan terenkripsi di database.")}</p>
					</div>
					<label class="block">
						<span class="text-sm font-medium">IP publik (IPv4 / IPv6)</span>
						<span class="block text-xs text-muted-foreground">{tr("Kosongkan untuk menghapus dan membiarkan sistem mendeteksi otomatis.")}</span>
						<input class="app-input mt-1 w-full font-mono" placeholder="43.156.237.97" bind:value={serverIp} />
					</label>
					<div class="flex flex-wrap items-center gap-3">
						<button class="app-btn app-btn-primary" type="submit" disabled={serverBusy}>{serverBusy ? 'Menyimpan…' : tr('Simpan')}</button>
						<Badge
							tone={serverCfg?.source === 'dashboard' ? 'success' : serverCfg?.source === 'env' ? 'warning' : 'neutral'}
							label={serverSourceLabel}
						/>
						{#if serverCfg?.ip}<span class="text-xs text-muted-foreground">Nilai aktif: <code>{serverCfg.ip}</code></span>{/if}
					</div>
					{#if serverMsg.text}<p class="text-sm" style="color: var(--success);">{serverMsg.text}</p>{/if}
					{#if serverError && !serverError.includes('Akses platform')}<p class="text-sm" style="color: var(--destructive);">{serverError}</p>{/if}
					<div class="rounded-lg border px-3 py-2 text-xs text-muted-foreground">{tr("Isi dengan IP publik VPS tempat aplikasi ini berjalan. Kalau dikosongkan, sistem mendeteksi lewat layanan echo IP saat ada mitra yang diblokir — tetapi nilai manual lebih dapat diandalkan karena tidak bergantung layanan luar.")}</div>
					<div class="rounded-lg border px-3 py-2 text-xs" style="border-color: var(--warning); color: var(--warning);">{tr("Perubahan berlaku langsung pada pengecekan berikutnya. IP yang salah membuat mitra mengizinkan alamat yang tidak berguna, jadi pastikan angkanya sesuai VPS Anda.")}</div>
				</form>
			{/if}
		</div>
	{/if}
	</div>
</section>
