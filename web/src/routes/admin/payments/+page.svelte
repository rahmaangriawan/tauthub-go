<script lang="ts">
	// Payment gateway & transaksi — port app/admin/payments/page.tsx:
	// konfigurasi multi-provider (Xendit, Duitku, Midtrans, Tripay, dll) dengan
	// hanya satu gateway aktif, test koneksi, dan riwayat 50 transaksi terbaru.
	import { onMount } from 'svelte';
	import { Coins } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Select from '$lib/components/select.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDateTime, fmtRp } from '$lib/admin-format';
	import { platformError } from '../_platform';

	type CredentialField = { key: string; label: string; secret?: boolean; required?: boolean; placeholder?: string };
	type Gateway = {
		id: string; provider: string; label: string; environment: 'sandbox' | 'production';
		is_configured: boolean; is_active: boolean; last_tested_at: string | null;
		last_test_ok: boolean | null; credentials: Record<string, string>;
		credential_fields: CredentialField[];
	};
	type TxLog = {
		id: string; reference: string; provider: string; environment: string; amount: number;
		currency: string; status: string; purpose: string; purpose_ref_id: string | null;
		payment_method: string | null; has_invoice: boolean; created_at: string; paid_at: string | null;
	};

	const TX_STATUS_META: Record<string, { label: string; tone: 'success' | 'warning' | 'danger' | 'neutral' }> = {
		paid: { label: 'Sukses', tone: 'success' },
		succeeded: { label: 'Sukses', tone: 'success' },
		settled: { label: 'Sukses', tone: 'success' },
		pending: { label: 'Menunggu', tone: 'warning' },
		failed: { label: 'Gagal', tone: 'danger' },
		expired: { label: 'Kedaluwarsa', tone: 'neutral' },
		cancelled: { label: 'Dibatalkan', tone: 'neutral' }
	};

	let gateways = $state<Gateway[]>([]);
	let error = $state('');
	let loading = $state(true);
	let busyId = $state<string | null>(null);
	let forms = $state<Record<string, { environment: string; values: Record<string, string> }>>({});
	let txs = $state<TxLog[] | null>(null);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
			// Log transaksi dimuat terpisah: gagal tidak boleh memblokir halaman gateway.
			const txRes = await apiRequest('/v1/platform/payments/transactions?limit=50');
			if (txRes.ok) {
				const data = await txRes.json();
				txs = data.transactions ?? [];
			} else {
				txs = [];
			}
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		loading = true;
		error = '';
		const res = await apiRequest('/v1/platform/payments/gateways');
		if (res.ok) {
			const data = await res.json();
			gateways = data.gateways ?? [];
			const next: Record<string, { environment: string; values: Record<string, string> }> = {};
			for (const gateway of gateways) {
				next[gateway.provider] = forms[gateway.provider] ?? { environment: gateway.environment, values: {} };
			}
			forms = next;
		} else {
			error = await platformError(res);
		}
		loading = false;
	}

	function setEnvironment(provider: string, environment: string) {
		forms = { ...forms, [provider]: { environment, values: forms[provider]?.values ?? {} } };
	}

	function setValue(provider: string, key: string, value: string) {
		forms = {
			...forms,
			[provider]: {
				environment: forms[provider]?.environment ?? 'sandbox',
				values: { ...(forms[provider]?.values ?? {}), [key]: value }
			}
		};
	}

	async function save(gateway: Gateway) {
		busyId = gateway.id;
		error = '';
		const form = forms[gateway.provider];
		const credentials: Record<string, string> = {};
		for (const field of gateway.credential_fields) {
			const value = form?.values[field.key] ?? '';
			// Kirim field hanya saat admin mengetik sesuatu (menjaga secret tersimpan).
			if (value.trim()) credentials[field.key] = value.trim();
		}
		const res = await apiRequest('/v1/platform/payments/gateways', {
			method: 'POST',
			body: JSON.stringify({ provider: gateway.provider, environment: form?.environment, credentials })
		});
		if (!res.ok) {
			error = await platformError(res);
			busyId = null;
			return;
		}
		await load();
		// Kosongkan nilai yang diketik agar secret tidak tertinggal di DOM.
		forms = {
			...forms,
			[gateway.provider]: { environment: forms[gateway.provider]?.environment ?? gateway.environment, values: {} }
		};
		busyId = null;
	}

	async function activate(gateway: Gateway) {
		busyId = gateway.id;
		error = '';
		const res = await apiRequest(`/v1/platform/payments/gateways/${gateway.id}/activate`, { method: 'POST' });
		if (!res.ok) {
			error = await platformError(res);
			busyId = null;
			return;
		}
		await load();
		busyId = null;
	}

	async function test(gateway: Gateway) {
		busyId = gateway.id;
		error = '';
		const res = await apiRequest(`/v1/platform/payments/gateways/${gateway.id}/test`, { method: 'POST' });
		if (!res.ok) {
			error = await platformError(res);
			busyId = null;
			return;
		}
		const result = await res.json();
		error = result.ok ? '' : result.message;
		await load();
		busyId = null;
	}

	const activeProvider = $derived(gateways.find((g) => g.is_active)?.provider ?? null);
	const denied = $derived(Boolean(!loading && gateways.length === 0 && error.includes('Akses platform')));
</script>

<section class="mx-auto max-w-7xl">
	<PageHeader
		title="Payment Gateway"
		description="Kelola konfigurasi multi-provider (Xendit, Duitku, Midtrans, Tripay). Hanya SATU gateway yang aktif untuk transaksi baru; konfigurasi lain tetap tersimpan saat diganti."
	>
		<span
			class="rounded-full px-3 py-1 text-xs font-medium"
			style="background-color: {activeProvider ? 'var(--success-soft)' : 'var(--warning-soft)'}; color: {activeProvider ? 'var(--success)' : 'var(--warning)'};"
		>{activeProvider ? `${activeProvider} aktif` : 'Belum ada gateway aktif'}</span>
	</PageHeader>

	{#if denied}
		<div class="app-card mt-6 px-6 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke pengelolaan payment gateway.</p>
		</div>
	{:else}
		<Feedback {loading} {error} loadingText="Memuat payment gateway…" />

		<div class="mt-4 grid gap-4 md:grid-cols-2">
			{#each gateways as gateway (gateway.id)}
				{@const form = forms[gateway.provider]}
				<div class="app-card p-5">
					<div class="flex items-start justify-between">
						<div>
							<h2 class="text-base font-semibold">{gateway.label}</h2>
							<p class="mt-1 text-xs text-muted-foreground">Gateway provider • {gateway.provider}</p>
						</div>
						<div class="flex items-center gap-2">
							{#if gateway.is_active}<Badge label="Active" tone="success" />{/if}
							{#if !gateway.is_active && gateway.is_configured}<Badge label="Configured" tone="neutral" />{/if}
							{#if !gateway.is_configured}<Badge label="Not Configured" tone="neutral" />{/if}
						</div>
					</div>

					{#if gateway.is_configured}
						<p class="mt-3 text-xs text-muted-foreground">
							{gateway.last_tested_at
								? `Terakhir diuji: ${fmtDateTime(gateway.last_tested_at)} · ${gateway.last_test_ok ? 'OK' : 'gagal'}`
								: 'Belum pernah diuji koneksinya.'}
						</p>
					{/if}

					<div class="mt-3 flex flex-col gap-2">
						<label class="block">
							<span class="mb-1 block text-xs font-semibold text-muted-foreground">Environment</span>
							<Select
								ariaLabel="Environment"
								minWidth={200}
								value={form?.environment ?? 'sandbox'}
								onchange={(v) => setEnvironment(gateway.provider, v)}
								options={[
									{ value: 'sandbox', label: 'Sandbox' },
									{ value: 'production', label: 'Production' }
								]}
							/>
						</label>
						{#each gateway.credential_fields as field (field.key)}
							{@const current = gateway.credentials[field.key] ?? ''}
							<label class="block">
								<span class="mb-1 block text-xs font-semibold text-muted-foreground">{field.label}{current ? ` · tersimpan ${current}` : ''}</span>
								<input
									class="app-input w-full"
									type={field.secret ? 'password' : 'text'}
									placeholder={field.placeholder ?? (current ? 'Kosongkan untuk mempertahankan nilai tersimpan' : '')}
									value={form?.values[field.key] ?? ''}
									oninput={(e) => setValue(gateway.provider, field.key, (e.currentTarget as HTMLInputElement).value)}
								/>
							</label>
						{/each}
					</div>

					<div class="mt-4 flex flex-wrap gap-2">
						<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" type="button" disabled={busyId === gateway.id} onclick={() => void save(gateway)}>Simpan konfigurasi</button>
						{#if gateway.is_configured && !gateway.is_active}
							<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={busyId === gateway.id} onclick={() => void activate(gateway)}>Aktifkan gateway</button>
						{/if}
						{#if gateway.is_configured}
							<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={busyId === gateway.id} onclick={() => void test(gateway)}>Test koneksi</button>
						{/if}
					</div>
				</div>
			{/each}
		</div>

		{#if !loading && gateways.length === 0}
			<div class="flex flex-col items-center px-4 py-12 text-center">
				<Coins size={26} class="text-muted-foreground" />
				<p class="mt-2 text-sm text-muted-foreground">Belum ada konfigurasi payment gateway. Simpan kredensial provider untuk mulai.</p>
			</div>
		{/if}

		<div class="app-card mt-6 p-5">
			<div class="flex items-center justify-between">
				<h2 class="text-base font-semibold">Riwayat Transaksi</h2>
				<span class="text-xs text-muted-foreground">50 terbaru · status sukses/gagal dari webhook gateway</span>
			</div>
			{#if txs === null}
				<p class="mt-3 text-sm text-muted-foreground">Memuat log transaksi…</p>
			{:else if txs.length === 0}
				<p class="mt-3 text-sm text-muted-foreground">Belum ada transaksi pembayaran.</p>
			{:else}
				<div class="mt-3 overflow-x-auto">
					<table class="w-full text-sm" style="min-width: 1180px;">
						<thead>
							<tr class="text-left text-xs text-muted-foreground">
								<th class="px-4 py-2.5 font-medium">Referensi</th>
								<th class="px-4 py-2.5 font-medium">Gateway</th>
								<th class="px-4 py-2.5 font-medium">Nominal</th>
								<th class="px-4 py-2.5 font-medium">Status</th>
								<th class="px-4 py-2.5 font-medium">Tujuan</th>
								<th class="px-4 py-2.5 font-medium">Faktur</th>
								<th class="px-4 py-2.5 font-medium">Dibuat</th>
								<th class="px-4 py-2.5 font-medium">Dibayar</th>
							</tr>
						</thead>
						<tbody>
							{#each txs as t (t.id)}
								{@const meta = TX_STATUS_META[t.status] ?? { label: t.status, tone: 'neutral' as const }}
								<tr class="border-t">
									<td class="px-4 py-2.5 font-medium">{t.reference}</td>
									<td class="px-4 py-2.5">{t.provider} · {t.environment}</td>
									<td class="px-4 py-2.5 font-bold tabular-nums">{fmtRp(t.amount)}</td>
									<td class="px-4 py-2.5"><Badge label={meta?.label ?? t.status} tone={meta?.tone ?? 'neutral'} /></td>
									<td class="px-4 py-2.5">{t.purpose === 'topup' ? `Top Up ${t.purpose_ref_id ?? ''} TC` : t.purpose}</td>
									<td class="px-4 py-2.5">
										{#if t.has_invoice}<Badge label="Ya" tone="accent" />{:else}<span class="text-xs text-muted-foreground">-</span>{/if}
									</td>
									<td class="px-4 py-2.5 text-xs text-muted-foreground">{fmtDateTime(t.created_at)}</td>
									<td class="px-4 py-2.5 text-xs text-muted-foreground">{t.paid_at ? fmtDateTime(t.paid_at) : '-'}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</div>
	{/if}
</section>
