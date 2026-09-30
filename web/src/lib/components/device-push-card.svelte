<script lang="ts">
	// Kartu "Notifikasi perangkat": dipakai di tab Preferensi notifikasi klien
	// dan mitra. Menjelaskan keadaan yang gagal (izin ditolak, iOS belum
	// dipasang ke Home Screen) alih-alih menyembunyikan tombolnya.
	import { onMount } from 'svelte';
	import { BellRing, LoaderCircle, Send, Smartphone } from '@lucide/svelte';
	import { tr } from '$lib/i18n/phrases';
	import { showToast } from '$lib/auth';
	import {
		currentState,
		disableDevicePush,
		enableDevicePush,
		fetchPushConfig,
		sendTestPush,
		type DevicePushState,
		type PushConfig
	} from '$lib/push/device-push';

	let deviceState = $state<DevicePushState>('default');
	let config = $state<PushConfig | null>(null);
	let busy = $state(false);
	let error = $state('');

	onMount(async () => {
		try {
			config = await fetchPushConfig();
			deviceState = await currentState(config);
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal memuat status notifikasi.');
		}
	});

	async function toggle() {
		if (!config) return;
		busy = true;
		error = '';
		try {
			deviceState = deviceState === 'subscribed' ? await disableDevicePush() : await enableDevicePush(config);
			if (deviceState === 'subscribed') showToast(tr('Notifikasi perangkat aktif.'));
			else if (deviceState === 'denied') error = tr('Izin notifikasi diblokir. Aktifkan kembali dari pengaturan situs di browser Anda.');
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal mengubah status notifikasi.');
		} finally {
			busy = false;
		}
	}

	async function test() {
		busy = true;
		error = '';
		try {
			const res = await sendTestPush();
			showToast(`${res.sent}/${res.devices} ${tr('perangkat menerima notifikasi uji.')}`);
		} catch (reason) {
			error = reason instanceof Error ? reason.message : tr('Gagal mengirim notifikasi uji.');
		} finally {
			busy = false;
		}
	}

	const label = $derived(
		deviceState === 'subscribed'
			? tr('Nonaktifkan perangkat ini')
			: tr('Aktifkan notifikasi di perangkat ini')
	);
</script>

<div class="mt-4 border-t pt-4" style="border-color: var(--border);">
	<div class="flex items-center gap-3">
		<BellRing size={16} style="color: var(--primary);" />
		<h3 class="text-sm font-medium">{tr('Notifikasi perangkat')}</h3>
	</div>
	<p class="mt-1 text-xs text-muted-foreground">
		{tr('Terima pemberitahuan status publikasi, revisi, dan pengumuman langsung di desktop atau ponsel Anda.')}
	</p>

	{#if deviceState === 'unsupported'}
		<p class="mt-3 text-sm" style="color: var(--muted-foreground);">
			{tr('Browser ini tidak mendukung notifikasi perangkat.')}
		</p>
	{:else if deviceState === 'ios-needs-install'}
		<div class="mt-3 flex items-start gap-2 rounded-lg p-3 text-sm" style="background-color: var(--muted);">
			<Smartphone size={16} class="mt-0.5 flex-none" style="color: var(--primary);" />
			<p>
				{tr('Di iPhone/iPad, notifikasi baru aktif setelah app dipasang: tekan tombol Bagikan di Safari, pilih “Tambahkan ke Layar Utama”, lalu buka app dari ikon tersebut.')}
			</p>
		</div>
	{:else if deviceState === 'unconfigured'}
		<p class="mt-3 text-sm" style="color: var(--muted-foreground);">
			{tr('Notifikasi perangkat belum diaktifkan oleh admin platform.')}
		</p>
	{:else}
		<div class="mt-3 flex flex-wrap items-center gap-2">
			<button class="app-btn app-btn-primary" type="button" onclick={toggle} disabled={busy}>
				{#if busy}<LoaderCircle size={14} class="animate-spin" />{/if}
				{label}
			</button>
			{#if deviceState === 'subscribed'}
				<button class="app-btn app-btn-soft" type="button" onclick={test} disabled={busy}>
					<Send size={14} /> {tr('Kirim notifikasi uji')}
				</button>
			{/if}
		</div>
		{#if deviceState === 'denied'}
			<p class="mt-2 text-sm" style="color: var(--destructive);">
				{tr('Izin notifikasi diblokir di browser ini. Buka pengaturan situs (ikon gembok di address bar) lalu izinkan notifikasi.')}
			</p>
		{/if}
		{#if deviceState === 'subscribed' && config && config.subscriptions > 1}
			<p class="mt-2 text-xs text-muted-foreground">
				{config.subscriptions} {tr('perangkat terdaftar untuk akun ini.')}
			</p>
		{/if}
	{/if}

	{#if error}
		<p class="mt-2 text-sm" style="color: var(--destructive);">{error}</p>
	{/if}
</div>
