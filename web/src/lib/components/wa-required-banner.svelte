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
		style="border-color: #F3BE7A; background-color: color-mix(in srgb, #F3BE7A 26%, #ffffff);"
		role="alert"
	>
		<div class="flex items-start gap-3">
			<TriangleAlert size={18} class="mt-0.5 shrink-0" style="color: #B86B14;" />
			<div>
				<p class="font-semibold" style="color: #7C4509;">{tr('Nomor WhatsApp wajib diisi')}</p>
				<p class="mt-1 text-sm" style="color: #5C3A14;">
					{tr('Media Anda sudah')}
					<strong>{mediaCount}</strong>{tr('. Media berikutnya baru bisa ditambahkan setelah nomor WhatsApp aktif dan izin dihubungi diisi, supaya pesanan dari klien bisa kami konfirmasi lebih cepat.')}
				</p>
				{#if profileHref}
					<a
						class="app-btn mt-3 !min-h-8 !px-3 !text-xs font-semibold"
						style="background-color: #F3BE7A; border-color: #E2A558; color: #3D2204;"
						href={profileHref}
					>
						{tr('Isi nomor WhatsApp sekarang')}
					</a>
				{/if}
			</div>
		</div>
	</div>
{/if}
