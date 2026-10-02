<script lang="ts">
	// Halaman error aplikasi (404 & lainnya) — port fallback SvelteKit default
	// menjadi halaman utuh sesuai desain Tauthub: ilustrasi, judul, pesan, dan
	// dua aksi (kembali ke beranda / halaman sebelumnya).
	import { page } from '$app/state';
	import { Home } from '@lucide/svelte';
	import { tr } from '$lib/i18n/phrases';

	const is404 = $derived(page.status === 404);

	const judul = $derived(
		is404 ? tr('Halaman tidak ditemukan') : tr('Terjadi kesalahan')
	);
	const pesan = $derived(
		is404
			? tr('Maaf, halaman yang kamu cari tidak tersedia atau mungkin sudah dipindahkan.')
			: (page.error?.message ?? tr('Coba muat ulang halaman, atau kembali ke beranda.'))
	);

</script>

<svelte:head>
	<title>{judul} · Tauthub</title>
</svelte:head>

<div class="errPage">
	<img class="errIlustrasi" src="/images/error-404.webp" alt="" aria-hidden="true" loading="eager" />

	<h1 class="errJudul">{judul}</h1>
	<p class="errPesan">{pesan}</p>

	<div class="errAksi">
		<a class="errBtn errBtnPrimary" href="/">
			<Home size={17} strokeWidth={2.2} />
			<span>{tr('Kembali ke Beranda')}</span>
		</a>
	</div>
</div>

<style>
	.errPage {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		min-height: 100dvh;
		padding: 40px 20px;
		text-align: center;
		background: var(--background, #ffffff);
		color: var(--foreground, #111827);
	}
	.errIlustrasi {
		width: min(560px, 100%);
		height: auto;
		user-select: none;
		pointer-events: none;
	}
	.errJudul {
		margin: 8px 0 0;
		font-size: clamp(28px, 5vw, 40px);
		font-weight: 800;
		letter-spacing: -0.02em;
		color: #111827;
	}
	.errPesan {
		margin: 12px auto 0;
		max-width: 560px;
		font-size: 16px;
		line-height: 1.65;
		color: #667085;
	}
	.errAksi {
		display: flex;
		flex-wrap: wrap;
		justify-content: center;
		gap: 12px;
		margin-top: 32px;
	}
	.errBtn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		min-height: 48px;
		padding: 0 22px;
		border-radius: 10px;
		font-size: 14.5px;
		font-weight: 600;
		text-decoration: none;
		cursor: pointer;
		transition: filter 0.15s ease, background-color 0.15s ease;
	}
	.errBtn:focus-visible {
		outline: 2px solid var(--primary, #2563eb);
		outline-offset: 2px;
	}
	.errBtnPrimary {
		border: 1px solid transparent;
		background: var(--primary, #2563eb);
		color: #ffffff;
	}
	.errBtnPrimary:hover {
		filter: brightness(1.08);
	}
	@media (max-width: 480px) {
		.errAksi {
			flex-direction: column;
			width: 100%;
			max-width: 320px;
		}
		.errBtn {
			width: 100%;
		}
	}
</style>
