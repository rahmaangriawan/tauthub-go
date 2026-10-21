<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Banner wajib nomor WhatsApp: muncul saat mitra sudah punya lebih dari satu
	// media tetapi nomor WA + izin dihubungi belum lengkap (aturan server:
	// notification_preferences.wa_number + wa_consent; ambang di waMediaThreshold).
	import { TriangleAlert } from '@lucide/svelte';

	let {
		show = false,
		mediaCount = 0,
		profileHref = ''
	}: { show?: boolean; mediaCount?: number; profileHref?: string } = $props();
</script>

{#if show}
	<div
		class="app-card p-4"
		style="border-color: var(--destructive); background-color: var(--warning-soft, #fff7ed);"
		role="alert"
	>
		<div class="flex items-start gap-3">
			<TriangleAlert size={18} class="mt-0.5 shrink-0" style="color: var(--destructive);" />
			<div>
				<p class="font-semibold" style="color: var(--destructive);">{tr('Nomor WhatsApp wajib diisi')}</p>
				<p class="mt-1 text-sm text-muted-foreground">
					{tr('Media Anda sudah')}
					<strong>{mediaCount}</strong>{tr('. Media berikutnya baru bisa ditambahkan setelah nomor WhatsApp aktif dan izin dihubungi diisi, supaya pesanan dari klien bisa kami konfirmasi lebih cepat.')}
				</p>
				{#if profileHref}
					<a class="app-btn app-btn-primary mt-3 !min-h-8 !px-3 !text-xs" href={profileHref}>
						{tr('Isi nomor WhatsApp sekarang')}
					</a>
				{/if}
			</div>
		</div>
	</div>
{/if}
