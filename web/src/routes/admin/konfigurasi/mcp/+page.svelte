<script lang="ts">
	// Konfigurasi MCP admin (Hermes Agent + Claude Code).
	// Nilai disimpan terenkripsi di tabel mcp_config (bukan .env). Service
	// key dan bot token tidak pernah dikembalikan server — hanya hint
	// (4 karakter terakhir) + flag "sudah diisi".
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime } from '$lib/admin-format';

	type McpConfig = {
		service_key_set: boolean;
		service_key_hint: string;
		telegram_bot_token_set: boolean;
		telegram_bot_hint: string;
		telegram_chat_ids: number[];
		writes_enabled: boolean;
		max_credit_adjust_tc: number;
		updated_at?: string | null;
	};

	let loading = $state(true);
	let saving = $state(false);
	let error = $state('');
	let notice = $state('');
	let cfg = $state<McpConfig | null>(null);

	// Form state yang DIKIRIM. Secret hanya dikirim bila diisi (atau dikosongkan
	// dengan tombol Hapus) — kita tidak mengisi ulang hint ke field.
	let serviceKey = $state('');
	let serviceKeyAction = $state<'keep' | 'replace' | 'clear'>('keep');
	let botToken = $state('');
	let botTokenAction = $state<'keep' | 'replace' | 'clear'>('keep');
	let chatIdsText = $state('');
	let writesEnabled = $state(true);
	let maxAdjust = $state(50_000_000);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		loading = true;
		error = '';
		try {
			const res = await apiRequest('/v1/platform/mcp-config');
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			cfg = await res.json();
			chatIdsText = (cfg?.telegram_chat_ids ?? []).join('\n');
			writesEnabled = Boolean(cfg?.writes_enabled);
			maxAdjust = Number(cfg?.max_credit_adjust_tc ?? 50_000_000);
			serviceKey = '';
			botToken = '';
			serviceKeyAction = 'keep';
			botTokenAction = 'keep';
		} catch {
			error = 'Tidak dapat terhubung ke server API.';
		} finally {
			loading = false;
		}
	}

	function parseChatIds(text: string): number[] | string {
		const lines = text
			.split(/[\n,\s]+/)
			.map((s) => s.trim())
			.filter(Boolean);
		const out: number[] = [];
		for (const raw of lines) {
			if (!/^-?\d+$/.test(raw)) return `Entri "${raw}" bukan angka.`;
			out.push(Number(raw));
		}
		return out;
	}

	async function save() {
		notice = '';
		error = '';
		const ids = parseChatIds(chatIdsText);
		if (typeof ids === 'string') {
			error = ids;
			return;
		}
		const body: Record<string, unknown> = {
			telegram_chat_ids: ids,
			writes_enabled: writesEnabled,
			max_credit_adjust_tc: maxAdjust
		};
		if (serviceKeyAction === 'replace') {
			if (serviceKey.trim().length < 32) {
				error = 'Service key minimal 32 karakter. Gunakan tombol "Generate" untuk nilai acak yang aman.';
				return;
			}
			body.service_key = serviceKey.trim();
		} else if (serviceKeyAction === 'clear') {
			body.service_key = '';
		}
		if (botTokenAction === 'replace') {
			if (!botToken.trim()) {
				error = 'Bot token tidak boleh kosong bila mode Replace dipilih.';
				return;
			}
			body.telegram_bot_token = botToken.trim();
		} else if (botTokenAction === 'clear') {
			body.telegram_bot_token = '';
		}

		saving = true;
		try {
			const res = await apiRequest('/v1/platform/mcp-config', {
				method: 'PUT',
				body: JSON.stringify(body)
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal menyimpan konfigurasi MCP.');
				return;
			}
			cfg = await res.json();
			notice = 'Tersimpan. Perubahan key & flag langsung berlaku; penggantian bot token butuh systemctl restart tauthub-mcp.';
			serviceKey = '';
			botToken = '';
			serviceKeyAction = 'keep';
			botTokenAction = 'keep';
		} catch {
			error = 'Tidak dapat terhubung ke server API.';
		} finally {
			saving = false;
		}
	}

	function generateServiceKey() {
		const bytes = new Uint8Array(32);
		crypto.getRandomValues(bytes);
		serviceKey = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
		serviceKeyAction = 'replace';
	}
</script>

<section>
	<PageHeader
		title="Konfigurasi MCP"
		description="Service key, bot Telegram, allowlist, dan kill switch untuk server MCP admin (Hermes / Claude Code). Secret disimpan terenkripsi di database — tidak di .env."
	/>

	<Feedback {loading} {error} loadingText="Memuat konfigurasi MCP…" />

	{#if !loading && cfg}
		{#if notice}
			<div
				class="app-card mt-4 border-l-4 px-4 py-3 text-sm"
				style="border-color: var(--success);"
			>
				{notice}
			</div>
		{/if}

		<form
			class="mt-4 flex flex-col gap-4"
			onsubmit={(e) => {
				e.preventDefault();
				void save();
			}}
		>
			<div class="app-card p-4">
				<h2 class="text-base font-semibold">Service key</h2>
				<p class="text-muted-foreground mt-1 text-xs">
					Dipakai cmd/mcp (dan admin yang mengetes via curl) sebagai Bearer saat memanggil
					/v1/platform/* via loopback. Hanya pernah tampil sekali di sini — salin dan
					simpan di password manager setelah generate.
				</p>
				<div class="mt-3 flex flex-wrap items-center gap-3 text-sm">
					{#if cfg.service_key_set}
						<Badge tone="success" label="Terpasang" />
						<span class="font-mono text-xs">{cfg.service_key_hint}</span>
					{:else}
						<Badge tone="warning" label="Belum di-set" />
					{/if}
				</div>
				<div class="mt-3 flex items-center gap-4 text-sm">
					<label class="flex items-center gap-2">
						<input
							type="radio"
							name="svc"
							value="keep"
							checked={serviceKeyAction === 'keep'}
							onchange={() => (serviceKeyAction = 'keep')}
						/> Biarkan
					</label>
					<label class="flex items-center gap-2">
						<input
							type="radio"
							name="svc"
							value="replace"
							checked={serviceKeyAction === 'replace'}
							onchange={() => (serviceKeyAction = 'replace')}
						/> Ganti
					</label>
					<label class="flex items-center gap-2">
						<input
							type="radio"
							name="svc"
							value="clear"
							checked={serviceKeyAction === 'clear'}
							onchange={() => (serviceKeyAction = 'clear')}
						/> Hapus
					</label>
				</div>
				{#if serviceKeyAction === 'replace'}
					<div class="mt-3 flex flex-wrap gap-2">
						<input
							class="app-input flex-1 font-mono text-xs"
							type="text"
							placeholder="Minimal 32 karakter"
							bind:value={serviceKey}
							autocomplete="off"
							spellcheck="false"
						/>
						<button
							class="app-btn app-btn-soft"
							type="button"
							onclick={generateServiceKey}>Generate 64 hex</button
						>
					</div>
				{/if}
			</div>

			<div class="app-card p-4">
				<h2 class="text-base font-semibold">Bot Telegram Persetujuan</h2>
				<p class="text-muted-foreground mt-1 text-xs">
					Token dari <strong>@BotFather</strong>. Bot ini khusus untuk tombol Setujui/Tolak —
					berbeda dari bot yang dipakai Hermes untuk percakapan. Rotasi token butuh
					<code>systemctl restart tauthub-mcp</code>.
				</p>
				<div class="mt-3 flex flex-wrap items-center gap-3 text-sm">
					{#if cfg.telegram_bot_token_set}
						<Badge tone="success" label="Terpasang" />
						<span class="font-mono text-xs">{cfg.telegram_bot_hint}</span>
					{:else}
						<Badge tone="warning" label="Belum di-set" />
					{/if}
				</div>
				<div class="mt-3 flex items-center gap-4 text-sm">
					<label class="flex items-center gap-2">
						<input
							type="radio"
							name="tg"
							value="keep"
							checked={botTokenAction === 'keep'}
							onchange={() => (botTokenAction = 'keep')}
						/> Biarkan
					</label>
					<label class="flex items-center gap-2">
						<input
							type="radio"
							name="tg"
							value="replace"
							checked={botTokenAction === 'replace'}
							onchange={() => (botTokenAction = 'replace')}
						/> Ganti
					</label>
					<label class="flex items-center gap-2">
						<input
							type="radio"
							name="tg"
							value="clear"
							checked={botTokenAction === 'clear'}
							onchange={() => (botTokenAction = 'clear')}
						/> Hapus
					</label>
				</div>
				{#if botTokenAction === 'replace'}
					<input
						class="app-input mt-3 w-full font-mono text-xs"
						type="text"
						placeholder="123456:AAA-…"
						bind:value={botToken}
						autocomplete="off"
						spellcheck="false"
					/>
				{/if}
			</div>

			<div class="app-card p-4">
				<h2 class="text-base font-semibold">Chat allowlist</h2>
				<p class="text-muted-foreground mt-1 text-xs">
					Daftar <code>chat_id</code> Telegram yang menerima kartu persetujuan dan notifikasi
					proaktif. Satu per baris. Dapatkan ID lewat bot <code>@userinfobot</code> (pribadi) atau
					<code>@getidsbot</code> (grup). Kosong = bot tidak mengirim ke mana pun.
				</p>
				<textarea
					class="app-input mt-3 w-full font-mono text-xs"
					rows="4"
					placeholder="123456789&#10;-1001234567890"
					bind:value={chatIdsText}
				></textarea>
			</div>

			<div class="app-card p-4">
				<h2 class="text-base font-semibold">Kill switch & batas</h2>
				<label class="mt-3 flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={writesEnabled} />
					Aksi tulis diaktifkan (uncheck = semua tool tulis menolak dengan error)
				</label>
				<label class="mt-3 block text-sm">
					<span class="text-muted-foreground block text-xs">
						Batas maksimum penyesuaian saldo per aksi (TC = Rp)
					</span>
					<input
						class="app-input mt-1 w-full"
						type="number"
						min="0"
						step="1000"
						bind:value={maxAdjust}
					/>
				</label>
			</div>

			<div class="flex items-center justify-between">
				<p class="text-muted-foreground text-xs">
					{cfg.updated_at ? `Terakhir diubah ${fmtDateTime(cfg.updated_at)}` : ''}
				</p>
				<div class="flex gap-2">
					<button class="app-btn app-btn-soft" type="button" onclick={() => load()} disabled={saving}
						>Batal</button
					>
					<button class="app-btn app-btn-primary" type="submit" disabled={saving}>
						{saving ? 'Menyimpan…' : 'Simpan konfigurasi'}
					</button>
				</div>
			</div>
		</form>
	{/if}
</section>
