<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// WA Gateway — port app/admin/wa-gateway/page.tsx.
	// Tab: Template WA (per event, dengan pratinjau variabel), Setup API (token Fonnte),
	// dan Monitoring (ringkasan + per event + riwayat pengiriman + detail).
	import { onMount } from 'svelte';
	import { RefreshCw, Save, TriangleAlert, Zap } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';

	type WaConfig = {
		configured: boolean;
		token_hint: string | null;
		sender_name: string | null;
		country_code: string;
		enabled: boolean;
		updated_at: string | null;
	};
	type WaTemplate = {
		event: string;
		label: string;
		body: string;
		default_body: string;
		enabled: boolean;
		customized: boolean;
		preview: string | null;
	};
	type DeviceStatus = {
		connected: boolean;
		device: string | null;
		deviceStatus: string | null;
		quota: string | null;
		package: string | null;
		expired: string | null;
		name: string | null;
	};
	type DeliveryItem = {
		id: string;
		event: string;
		label: string;
		target: string;
		status: 'queued' | 'sent' | 'failed' | string;
		attempts: number;
		last_error: string | null;
		reference_id: string | null;
		body: string;
		created_at: string | null;
		sent_at: string | null;
	};
	type MonitoringData = {
		summary: { total: number; sent: number; failed: number; queued: number; success_rate: number | null };
		by_event: Array<{ event: string; label: string; total: number; sent: number; failed: number }>;
		top_errors: Array<{ error: string; total: number }>;
		items: DeliveryItem[];
		total_filtered: number;
		page: number;
		page_size: number;
		days: number;
	};

	const PERIOD_OPTIONS = $derived([
		{ value: '1', label: '24 jam terakhir' },
		{ value: '7', label: '7 hari terakhir' },
		{ value: '30', label: '30 hari terakhir' },
		{ value: '0', label: 'Seluruh riwayat' }
	]);
	const STATUS_TONE: Record<string, 'success' | 'danger' | 'neutral'> = {
		sent: 'success',
		failed: 'danger',
		queued: 'neutral'
	};
	const STATUS_LABEL: Record<string, string> = $derived({
		sent: 'Terkirim',
		failed: tr('Gagal'),
		queued: tr('Menunggu')
	});

	let tab = $state<'templates' | 'setup' | 'monitoring'>('templates');
	let loading = $state(true);
	let error = $state('');
	let config = $state<WaConfig | null>(null);
	let templates = $state<WaTemplate[]>([]);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		error = '';
		try {
			const [cfgRes, tplRes] = await Promise.all([
				apiRequest('/v1/platform/wa/config'),
				apiRequest('/v1/platform/wa/templates')
			]);
			if (!cfgRes.ok) {
				const msg = await readApiError(cfgRes, 'Permintaan platform gagal.');
				error = msg === 'Resource not found' ? 'Akses platform administrator tidak tersedia.' : msg;
				return;
			}
			config = await cfgRes.json();
			setupSenderName = config?.sender_name ?? '';
			setupEnabled = Boolean(config?.enabled);
			if (tplRes.ok) templates = (await tplRes.json()).items ?? [];
		} catch {
			error = tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		}
	}

	// ── Template tab ───────────────────────────────────────────────────────
	let editing = $state<string | null>(null);
	let draft = $state('');
	let tplBusy = $state(false);
	let tplMsg = $state({ text: '', ok: false });

	function renderPreview(text: string) {
		return text
			.replace(/\{nama\}/g, 'Budi')
			.replace(/\{judul\}/g, 'Contoh Artikel Tauthub')
			.replace(/\{status\}/g, 'contoh keterangan')
			.replace(/\{jumlah\}/g, 'Rp 150.000')
			.replace(/\{link\}/g, 'https://app.tauthub.com/mitra/mitra-contoh/publications?pub=pub-contoh');
	}

	async function saveTemplate(event: string, enabled: boolean, body: string) {
		tplBusy = true;
		tplMsg = { text: '', ok: false };
		try {
			const res = await apiRequest('/v1/platform/wa/templates', {
				method: 'PUT',
				body: JSON.stringify({ event, body, enabled })
			});
			if (!res.ok) {
				tplMsg = { text: await readApiError(res, 'Permintaan platform gagal.'), ok: false };
				return;
			}
			tplMsg = { text: 'Template disimpan.', ok: true };
			editing = null;
			await load();
		} finally {
			tplBusy = false;
		}
	}

	async function toggleTemplate(t: WaTemplate) {
		tplBusy = true;
		tplMsg = { text: '', ok: false };
		try {
			const res = await apiRequest('/v1/platform/wa/templates', {
				method: 'PUT',
				body: JSON.stringify({ event: t.event, body: t.body, enabled: !t.enabled })
			});
			if (!res.ok) {
				tplMsg = { text: await readApiError(res, 'Permintaan platform gagal.'), ok: false };
				return;
			}
			await load();
		} finally {
			tplBusy = false;
		}
	}

	async function resetTemplate(t: WaTemplate) {
		if (!confirm(`Kembalikan template ${t.label} ke default?`)) return;
		tplBusy = true;
		tplMsg = { text: '', ok: false };
		try {
			const res = await apiRequest(`/v1/platform/wa/templates?event=${encodeURIComponent(t.event)}`, { method: 'DELETE' });
			if (!res.ok) {
				tplMsg = { text: await readApiError(res, 'Permintaan platform gagal.'), ok: false };
				return;
			}
			tplMsg = { text: tr('Template dikembalikan ke default.'), ok: true };
			editing = null;
			await load();
		} finally {
			tplBusy = false;
		}
	}

	// ── Setup tab ──────────────────────────────────────────────────────────
	let setupToken = $state('');
	let setupSenderName = $state('');
	let setupEnabled = $state(false);
	let setupBusy = $state(false);
	let setupMsg = $state({ text: '', ok: false });
	let testTarget = $state('');
	let testing = $state(false);
	let testMsg = $state({ text: '', ok: false });
	let device = $state<DeviceStatus | null>(null);

	async function saveSetup(e: SubmitEvent) {
		e.preventDefault();
		setupBusy = true;
		setupMsg = { text: '', ok: false };
		try {
			const res = await apiRequest('/v1/platform/wa/config', {
				method: 'PUT',
				body: JSON.stringify({
					token: setupToken.trim() || null,
					sender_name: setupSenderName.trim() || null,
					enabled: setupEnabled
				})
			});
			if (!res.ok) {
				setupMsg = { text: await readApiError(res, 'Permintaan platform gagal.'), ok: false };
				return;
			}
			setupToken = '';
			setupMsg = { text: 'Konfigurasi WA Gateway tersimpan.', ok: true };
			await load();
		} finally {
			setupBusy = false;
		}
	}

	async function runTest() {
		testing = true;
		testMsg = { text: '', ok: false };
		device = null;
		try {
			const res = await apiRequest('/v1/platform/wa/test', {
				method: 'POST',
				body: JSON.stringify({ target: testTarget.trim() || null })
			});
			if (!res.ok) {
				testMsg = { text: await readApiError(res, 'Permintaan platform gagal.'), ok: false };
				return;
			}
			const data = await res.json();
			testMsg = { text: data.detail ?? (data.ok ? 'Koneksi berhasil.' : 'Koneksi gagal.'), ok: Boolean(data.ok) };
			if (data.device) device = data.device;
		} finally {
			testing = false;
		}
	}

	// ── Monitoring tab ─────────────────────────────────────────────────────
	let monitor = $state<MonitoringData | null>(null);
	let monitorLoading = $state(false);
	let monitorError = $state('');
	let monStatus = $state('');
	let monEvent = $state('');
	let monSearch = $state('');
	// Pencarian live ala app lama: 300 ms setelah berhenti mengetik.
	let monSearchTimer: ReturnType<typeof setTimeout> | null = null;
	function onMonitorSearchInput() {
		if (monSearchTimer) clearTimeout(monSearchTimer);
		monSearchTimer = setTimeout(() => changeMonitorFilter(() => {}), 300);
	}
	let monDays = $state('7');
	let monPage = $state(1);
	let detail = $state<DeliveryItem | null>(null);
	let monitorLoaded = $state(false);

	async function loadMonitor() {
		monitorLoading = true;
		monitorError = '';
		try {
			const params = new URLSearchParams({ days: monDays, page: String(monPage) });
			if (monStatus) params.set('status', monStatus);
			if (monEvent) params.set('event', monEvent);
			if (monSearch.trim()) params.set('q', monSearch.trim());
			const res = await apiRequest(`/v1/platform/wa/deliveries?${params.toString()}`);
			if (!res.ok) {
				monitorError = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			monitor = await res.json();
		} catch {
			monitorError = tr('Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.');
		} finally {
			monitorLoading = false;
			monitorLoaded = true;
		}
	}

	function openMonitoring() {
		tab = 'monitoring';
		if (!monitorLoaded) void loadMonitor();
	}

	function changeMonitorFilter(apply: () => void) {
		monPage = 1;
		apply();
		void loadMonitor();
	}

	const truncate = (value: string, max: number) => (value.length > max ? `${value.slice(0, max)}…` : value);
</script>

<section>
	<PageHeader
		title="WA Gateway"
		description={tr("Notifikasi WhatsApp untuk mitra dan klien lewat Fonnte: atur template pesan per event dan koneksi API-nya.")}
	/>

	<Feedback {loading} {error} loadingText="Memuat WA Gateway…" />

	{#if !config && error.includes('Akses platform')}
		<div class="app-card mt-4 px-4 py-10 text-center">
			<p class="text-sm text-muted-foreground">{tr("Akun ini tidak memiliki akses ke pengaturan WA Gateway.")}</p>
		</div>
	{:else}
		<div class="mt-4 flex flex-wrap gap-2">
			<button class="app-btn {tab === 'templates' ? 'app-btn-primary' : 'app-btn-soft'} !min-h-8 !px-3 !text-xs" type="button" onclick={() => (tab = 'templates')}>Template WA</button>
			<button class="app-btn {tab === 'setup' ? 'app-btn-primary' : 'app-btn-soft'} !min-h-8 !px-3 !text-xs" type="button" onclick={() => (tab = 'setup')}>Setup API</button>
			<button class="app-btn {tab === 'monitoring' ? 'app-btn-primary' : 'app-btn-soft'} !min-h-8 !px-3 !text-xs" type="button" onclick={openMonitoring}>Monitoring</button>
		</div>

		<!-- Template WA -->
		{#if tab === 'templates'}
			<div class="mt-3 space-y-3">
				{#if templates.length === 0}
					<p class="app-card px-4 py-12 text-center text-sm text-muted-foreground">{tr("Belum ada event yang dipetakan ke WhatsApp.")}</p>
				{:else}
					{#if tplMsg.text}
						<div class="app-card px-4 py-2.5 text-sm" style="color: {tplMsg.ok ? 'var(--success)' : 'var(--destructive)'};">{tplMsg.text}</div>
					{/if}
					<div class="app-card px-4 py-2.5 text-xs" style="color: var(--muted-foreground);">{tr("Variabel yang bisa dipakai:")}<strong>{'{nama}'}</strong> nama penerima, <strong>{'{judul}'}</strong> judul artikel/domain,
						<strong>{'{status}'}</strong> keterangan/alasan, <strong>{'{jumlah}'}</strong> nominal rupiah,
						<strong>{'{link}'}</strong> tautan dashboard.
					</div>
					<div class="app-card px-4 py-2.5 text-xs" style="color: var(--accent-foreground);">
						Event publikasi dikirim ke <strong>{tr("mitra dan klien")}</strong>, masing-masing dengan tautan ke portalnya sendiri
						(mitra ke <strong>/mitra/…/publications</strong>, klien ke <strong>/klien/…/publications</strong>{tr(") yang langsung membuka detail artikel. Event media, pencairan, dan fee referral hanya relevan untuk mitra.")}</div>

					{#each templates as t (t.event)}
						<div class="app-card p-4">
							<div class="flex flex-wrap items-center justify-between gap-3">
								<div>
									<p class="text-sm font-medium">{t.label}</p>
									<p class="text-xs text-muted-foreground">{t.event}</p>
								</div>
								<div class="flex items-center gap-2">
									{#if t.customized}<Badge tone="accent" label="Dikustom" />{/if}
									<Badge tone={t.enabled ? 'success' : 'neutral'} label={t.enabled ? tr('Aktif') : tr('Nonaktif')} />
									<input type="checkbox" checked={t.enabled} disabled={tplBusy} onchange={() => void toggleTemplate(t)} aria-label="Aktifkan template" />
								</div>
							</div>

							{#if editing === t.event}
								<div class="mt-2 space-y-2">
									<textarea class="app-input w-full" rows="5" placeholder={tr("Tulis isi pesan WhatsApp…")} bind:value={draft}></textarea>
									<p class="text-xs text-muted-foreground">Pratinjau hasil render:</p>
									<div class="whitespace-pre-wrap rounded-lg px-3 py-2 text-xs" style="background-color: var(--secondary);">{renderPreview(draft)}</div>
									<div class="flex flex-wrap justify-end gap-2">
										<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => (editing = null)}>{tr("Batal")}</button>
										<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" disabled={tplBusy} onclick={() => void resetTemplate(t)}>Reset ke default</button>
										<button class="app-btn app-btn-primary !min-h-7 !px-2.5 !text-xs" type="button" disabled={tplBusy} onclick={() => void saveTemplate(t.event, t.enabled, draft)}>
											<Save size={12} /> {tplBusy ? 'Menyimpan…' : tr('Simpan')}
										</button>
									</div>
								</div>
							{:else}
								<div class="mt-2">
									<div class="whitespace-pre-wrap rounded-lg px-3 py-2 text-xs" style="background-color: var(--secondary);">{t.preview ?? t.body}</div>
									<div class="mt-2 flex justify-end">
										<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => { editing = t.event; draft = t.body; }}>Edit template</button>
									</div>
								</div>
							{/if}
						</div>
					{/each}
				{/if}
			</div>
		{/if}

		<!-- Setup API -->
		{#if tab === 'setup' && config}
			<div class="mt-3 max-w-[720px] space-y-3">
				<div class="app-card p-4">
					<div class="flex flex-wrap items-center gap-2">
						<h2 class="text-base font-semibold">Koneksi Fonnte</h2>
						<Badge tone={config?.configured ? 'success' : 'warning'} label={config?.configured ? 'Token tersimpan' : tr('Belum dikonfigurasi')} />
						<Badge tone={config?.enabled ? 'success' : 'neutral'} label={config?.enabled ? tr('Aktif') : tr('Nonaktif')} />
					</div>
					<form class="mt-3 space-y-3" onsubmit={saveSetup}>
						<label class="block">
							<span class="text-sm font-medium">Token Fonnte</span>
							<span class="block text-xs text-muted-foreground">
								{config?.token_hint ? `Tersimpan: ${config?.token_hint}. Kosongkan bila tidak diubah.` : tr('Ambil dari dashboard Fonnte → Device → Token.')}
							</span>
							<input class="app-input mt-1 w-full" type="password" placeholder={config?.configured ? tr('Kosongkan jika tidak diubah') : 'Masukkan token device Fonnte'} bind:value={setupToken} />
						</label>
						<label class="block">
							<span class="text-sm font-medium">{tr("Nama pengirim")}</span>
							<span class="block text-xs text-muted-foreground">{tr("Opsional. Muncul sebagai identitas bila device mendukung.")}</span>
							<input class="app-input mt-1 w-full" placeholder={tr("mis. Tauthub")} bind:value={setupSenderName} />
						</label>
						<p class="text-xs text-muted-foreground">{tr("Kode negara otomatis “62” — nomor 08xx dikonversi ke 628xx saat pengiriman.")}</p>
						<div class="flex flex-wrap items-center justify-between gap-3">
							<div>
								<p class="text-sm font-medium">Aktifkan pengiriman WA</p>
								<p class="text-xs text-muted-foreground">{tr("Bila nonaktif, tidak ada pesan yang dikirim walau template aktif.")}</p>
							</div>
							<input type="checkbox" bind:checked={setupEnabled} />
						</div>
						{#if setupMsg.text}
							<p class="text-sm" style="color: {setupMsg.ok ? 'var(--success)' : 'var(--destructive)'};">{setupMsg.text}</p>
						{/if}
						<div class="flex justify-end">
							<button class="app-btn app-btn-primary" type="submit" disabled={setupBusy}><Save size={14} /> {setupBusy ? 'Menyimpan…' : tr('Simpan konfigurasi')}</button>
						</div>
					</form>
				</div>

				<div class="app-card p-4">
					<div class="flex items-center gap-2">
						<Zap size={16} />
						<h2 class="text-base font-semibold">Test Koneksi</h2>
					</div>
					<p class="mt-1 text-xs text-muted-foreground">{tr("Mengecek status device dan sisa kuota Fonnte. Isi nomor di bawah bila ingin sekalian mengirim pesan uji.")}</p>
					<label class="mt-3 block">
						<span class="text-sm font-medium">Nomor tujuan uji (opsional)</span>
						<input class="app-input mt-1 w-full" placeholder="08123456789" bind:value={testTarget} />
					</label>
					<div class="mt-2 flex justify-end">
						<button class="app-btn app-btn-soft" type="button" disabled={testing} onclick={() => void runTest()}>
							{testing ? 'Menguji…' : 'Test koneksi'}
						</button>
					</div>
					{#if testMsg.text}
						<div class="mt-2 flex items-center gap-2 rounded-lg border px-3 py-2 text-sm" style="color: {testMsg.ok ? 'var(--success)' : 'var(--destructive)'};">
							{#if testMsg.ok}<Zap size={14} />{:else}<TriangleAlert size={14} />{/if}
							{testMsg.text}
						</div>
					{/if}
					{#if device}
						<div class="app-card mt-3 p-3">
							<div class="flex flex-wrap items-center gap-2">
								<Badge tone={device.connected ? 'success' : 'danger'} label={device.connected ? 'Terhubung' : 'Terputus'} />
								{#if device.deviceStatus}<span class="text-xs text-muted-foreground">status: {device.deviceStatus}</span>{/if}
							</div>
							<div class="mt-2 space-y-1 text-sm">
								{#if device.device}<p>Device: <strong>{device.device}</strong></p>{/if}
								{#if device.name}<p>{tr("Nama:")} {device.name}</p>{/if}
								{#if device.package}<p>Paket: {device.package}</p>{/if}
								{#if device.quota}<p>Sisa kuota: <strong>{device.quota}</strong> pesan</p>{/if}
								{#if device.expired}<p>Berlaku sampai: {device.expired}</p>{/if}
							</div>
						</div>
					{/if}
				</div>
			</div>
		{/if}

		<!-- Monitoring -->
		{#if tab === 'monitoring'}
			<div class="mt-3 space-y-3">
				{#if monitorError}<p class="text-sm" style="color: var(--destructive);">{monitorError}</p>{/if}

				<div class="flex flex-wrap gap-3">
					{#each [
						{ label: tr('Total'), value: monitor?.summary.total ?? 0 },
						{ label: 'Terkirim', value: monitor?.summary.sent ?? 0 },
						{ label: tr('Gagal'), value: monitor?.summary.failed ?? 0 },
						{ label: tr('Menunggu'), value: monitor?.summary.queued ?? 0 },
						{ label: 'Tingkat sukses', value: monitor?.summary.success_rate == null ? '—' : `${monitor.summary.success_rate}%` }
					] as s (s.label)}
						<div class="app-card min-w-[130px] flex-1 p-3">
							<p class="text-xs text-muted-foreground">{s.label}</p>
							<p class="text-xl font-bold">{s.value}</p>
						</div>
					{/each}
				</div>
				<p class="text-xs text-muted-foreground">{tr("Tingkat sukses dihitung dari pesan yang sudah selesai (terkirim + gagal); pesan yang masih menunggu belum punya hasil.")}</p>

				<div class="flex flex-wrap items-end gap-3">
					<label class="block min-w-[165px]">
						<span class="text-xs text-muted-foreground">Periode</span>
						<Select
							ariaLabel="Filter periode"
							minWidth={180}
							value={monDays}
							onchange={(v) => changeMonitorFilter(() => (monDays = v))}
							options={PERIOD_OPTIONS.map((o) => ({ value: o.value, label: o.label }))}
						/>
					</label>
					<label class="block min-w-[165px]">
						<span class="text-xs text-muted-foreground">Status</span>
						<Select
							ariaLabel="Filter status"
							minWidth={180}
							value={monStatus}
							onchange={(v) => changeMonitorFilter(() => (monStatus = v))}
							options={[
								{ value: '', label: tr('Semua status') },
								{ value: 'sent', label: 'Terkirim' },
								{ value: 'failed', label: tr('Gagal') },
								{ value: 'queued', label: tr('Menunggu') }
							]}
						/>
					</label>
					<label class="block min-w-[200px]">
						<span class="text-xs text-muted-foreground">Event</span>
						<Select
							ariaLabel="Filter event"
							minWidth={200}
							value={monEvent}
							onchange={(v) => changeMonitorFilter(() => (monEvent = v))}
							options={[
								{ value: '', label: tr('Semua event') },
								...(monitor?.by_event ?? []).map((ev) => ({ value: ev.event, label: ev.label })),
								...(monEvent && !(monitor?.by_event ?? []).some((ev) => ev.event === monEvent)
									? [{ value: monEvent, label: monEvent }]
									: [])
							]}
						/>
					</label>
					<label class="block min-w-[220px] flex-1">
						<span class="text-xs text-muted-foreground">{tr("Cari")}</span>
						<input class="app-input mt-1 w-full" placeholder={tr("Nomor tujuan, isi pesan, atau error")} bind:value={monSearch} oninput={onMonitorSearchInput} />
					</label>
					<button class="app-btn app-btn-soft" type="button" disabled={monitorLoading} onclick={() => void loadMonitor()}><RefreshCw size={14} /> Muat ulang</button>
				</div>

				{#if monitor && monitor.top_errors.length > 0}
					<div class="app-card p-4">
						<h3 class="text-sm font-medium">Error tersering</h3>
						<div class="mt-2 space-y-2">
							{#each monitor.top_errors as e (e.error)}
								<div class="flex items-start justify-between gap-3">
									<span class="break-words font-mono text-xs">{e.error}</span>
									<Badge tone="danger" label="{e.total}×" />
								</div>
							{/each}
						</div>
					</div>
				{/if}

				{#if monitor && monitor.by_event.length > 0}
					<div class="app-card p-4">
						<h3 class="text-sm font-medium">Per event</h3>
						<div class="mt-2 overflow-x-auto">
							<table class="w-full text-sm" style="min-width: 620px;">
								<thead>
									<tr class="text-xs text-muted-foreground">
										<th class="px-3 py-2 text-left font-medium">Event</th>
										<th class="px-3 py-2 text-left font-medium">{tr("Total")}</th>
										<th class="px-3 py-2 text-left font-medium">Terkirim</th>
										<th class="px-3 py-2 text-left font-medium">{tr("Gagal")}</th>
									</tr>
								</thead>
								<tbody>
									{#each monitor.by_event as e (e.event)}
										<tr class="border-t">
											<td class="px-3 py-2">{e.label}</td>
											<td class="px-3 py-2">{e.total}</td>
											<td class="px-3 py-2" style="color: var(--success);">{e.sent}</td>
											<td class="px-3 py-2" style="color: {e.failed > 0 ? 'var(--destructive)' : 'inherit'};">{e.failed}</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					</div>
				{/if}

				<div class="app-card p-4">
					<div class="flex flex-wrap items-center justify-between gap-3">
						<h3 class="text-sm font-medium">{tr("Riwayat pengiriman")}</h3>
						<span class="text-xs text-muted-foreground">{monitor?.total_filtered ?? 0} entri</span>
					</div>

					{#if !monitor || monitor.items.length === 0}
						<p class="mt-3 text-sm text-muted-foreground">{monitorLoading ? tr('Memuat…') : tr('Belum ada pengiriman pada filter ini.')}</p>
					{:else}
						<div class="mt-2 overflow-x-auto">
							<table class="w-full text-sm" style="min-width: 900px;">
								<thead>
									<tr class="text-xs text-muted-foreground">
										<th class="px-3 py-2 text-left font-medium">Waktu</th>
										<th class="px-3 py-2 text-left font-medium">Status</th>
										<th class="px-3 py-2 text-left font-medium">Event</th>
										<th class="px-3 py-2 text-left font-medium">Tujuan</th>
										<th class="px-3 py-2 text-left font-medium">Percobaan</th>
										<th class="px-3 py-2 text-left font-medium">Keterangan</th>
									</tr>
								</thead>
								<tbody>
									{#each monitor.items as item (item.id)}
										<tr class="cursor-pointer border-t hover:bg-secondary" onclick={() => (detail = item)}>
											<td class="px-3 py-2 text-xs">{fmtDateTime(item.created_at)}</td>
											<td class="px-3 py-2"><Badge tone={STATUS_TONE[item.status] ?? 'neutral'} label={STATUS_LABEL[item.status] ?? item.status} /></td>
											<td class="px-3 py-2 text-xs">{item.label}</td>
											<td class="px-3 py-2 text-xs">{item.target}</td>
											<td class="px-3 py-2 text-xs">{item.attempts}</td>
											<td class="px-3 py-2 text-xs" style="color: {item.last_error ? 'var(--destructive)' : 'var(--muted-foreground)'};">
												{item.last_error ? truncate(item.last_error, 60) : item.sent_at ? `Terkirim ${fmtDateTime(item.sent_at)}` : '—'}
											</td>
										</tr>
									{/each}
								</tbody>
							</table>
						</div>
					{/if}

					<Pagination
						total={monitor?.total_filtered ?? 0}
						limit={monitor?.page_size ?? 20}
						offset={(monPage - 1) * (monitor?.page_size ?? 20)}
						onChange={(next) => {
							monPage = next / (monitor?.page_size ?? 20) + 1;
							void loadMonitor();
						}}
					/>
				</div>
			</div>
		{/if}
	{/if}
</section>

{#if detail}
	<Modal title={tr("Detail pengiriman")} label={tr("Detail pengiriman WA")} maxWidth="xl" onClose={() => (detail = null)}>
		<div class="space-y-3">
			<p class="text-sm text-muted-foreground">{detail.label} · {detail.target}</p>
			<div class="flex flex-wrap items-center gap-3">
				<Badge tone={STATUS_TONE[detail.status] ?? 'neutral'} label={STATUS_LABEL[detail.status] ?? detail.status} />
				<span class="text-xs text-muted-foreground">Percobaan: {detail.attempts}</span>
				<span class="text-xs text-muted-foreground">{tr("Dibuat:")} {fmtDateTime(detail.created_at)}</span>
				{#if detail.sent_at}<span class="text-xs text-muted-foreground">Terkirim: {fmtDateTime(detail.sent_at)}</span>{/if}
			</div>
			{#if detail.reference_id}<p class="text-xs text-muted-foreground">Referensi: {detail.reference_id}</p>{/if}
			{#if detail.last_error}
				<div class="flex items-start gap-2 rounded-lg border px-3 py-2 text-sm" style="border-color: var(--destructive); color: var(--destructive);">
					<TriangleAlert size={15} class="mt-0.5 shrink-0" /> {detail.last_error}
				</div>
			{/if}
			<div>
				<p class="text-xs text-muted-foreground">Isi pesan</p>
				<div class="mt-1 whitespace-pre-wrap rounded-lg px-3 py-2 text-xs" style="background-color: var(--secondary);">{detail.body}</div>
			</div>
		</div>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" onclick={() => (detail = null)}>{tr("Tutup")}</button>
		{/snippet}
	</Modal>
{/if}
