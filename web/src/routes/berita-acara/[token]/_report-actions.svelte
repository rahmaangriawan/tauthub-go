<script lang="ts">
	// Aksi Berita Acara — parity report-actions.tsx app lama: dua ikon (unduh PDF
	// & tutup) di kanan atas kartu. Pengunjung yang belum login dan menekan
	// Tutup diarahkan ke dialog ajakan daftar, bukan langsung ditutup.
	import { Download, X } from '@lucide/svelte';
	import { goto } from '$app/navigation';
	import { getStoredSession } from '$lib/auth';
	import { tr } from '$lib/i18n/phrases';

	let signupOpen = $state(false);
	let dialogEl = $state<HTMLDivElement | null>(null);

	const signedIn = getStoredSession()?.accessToken ? true : false;

	function close() {
		if (signedIn) {
			window.close();
			window.setTimeout(() => {
				if (!window.closed) void goto('/');
			}, 250);
			return;
		}
		signupOpen = true;
	}

	$effect(() => {
		if (!signupOpen) return;
		dialogEl?.focus();
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape') signupOpen = false;
		};
		window.addEventListener('keydown', onKey);
		return () => window.removeEventListener('keydown', onKey);
	});
</script>

<div class="icon-actions">
	<button type="button" class="icon-button" aria-label={tr("Unduh PDF")} title={tr("Unduh PDF")} onclick={() => window.print()}>
		<Download size={18} />
	</button>
	<button type="button" class="icon-button" aria-label={tr("Tutup")} title={tr("Tutup")} onclick={close}>
		<X size={18} />
	</button>
</div>

{#if signupOpen}
	<div class="signup-overlay" role="dialog" aria-modal="true" aria-label={tr("Daftar akun Tauthub")}>
		<div class="signup-dialog" bind:this={dialogEl} tabindex="-1">
			<h2 class="signup-title">{tr("Buat akun Tauthub")}</h2>
			<p class="signup-text">{tr("Anda melihat berita acara kampanye dari klien Tauthub. Daftar untuk membuat kampanye, mendistribusikan artikel ke media, dan mencetak berita acara milik Anda sendiri.")}</p>
			<div class="signup-actions">
				<a class="signup-primary" href="/login?mode=register">{tr("Daftar sekarang")}</a>
				<button type="button" class="signup-secondary" onclick={() => (signupOpen = false)}>Nanti saja</button>
			</div>
		</div>
	</div>
{/if}

<style>
	.icon-actions {
		display: flex;
		gap: 8px;
	}
	.icon-button {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 34px;
		height: 34px;
		border: 1px solid var(--border);
		border-radius: 10px;
		background: var(--card);
		color: var(--muted-foreground);
		cursor: pointer;
		transition: background 150ms ease, color 150ms ease, border-color 150ms ease, transform 150ms ease;
	}
	.icon-button:hover {
		background: var(--secondary);
		color: var(--foreground);
		transform: translateY(-1px) scale(1.04);
	}
	.icon-button:active {
		transform: translateY(0) scale(0.96);
	}
	.icon-button:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
	}

	.signup-overlay {
		position: fixed;
		inset: 0;
		z-index: 60;
		display: grid;
		place-items: center;
		padding: 16px;
		background: rgba(15, 23, 42, 0.45);
	}
	.signup-dialog {
		width: min(440px, 100%);
		display: grid;
		gap: 12px;
		border: 1px solid var(--border);
		border-radius: 16px;
		padding: 24px;
		background: var(--card);
		color: var(--foreground);
		box-shadow: var(--shadow-md);
		outline: none;
	}
	.signup-title {
		margin: 0;
		font-size: 17px;
		font-weight: 800;
		letter-spacing: -0.01em;
	}
	.signup-text {
		margin: 0;
		color: var(--muted-foreground);
		font-size: 13.5px;
		line-height: 1.6;
	}
	.signup-actions {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
		margin-top: 4px;
	}
	.signup-primary {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-height: 38px;
		border: 0;
		border-radius: 10px;
		padding: 8px 16px;
		font-size: 13px;
		font-weight: 700;
		background: var(--primary);
		color: var(--primary-foreground);
		cursor: pointer;
		text-decoration: none;
	}
	.signup-primary:hover {
		background: color-mix(in srgb, var(--primary) 88%, black);
	}
	.signup-secondary {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-height: 38px;
		border: 1px solid var(--border);
		border-radius: 10px;
		padding: 8px 16px;
		font-size: 13px;
		font-weight: 650;
		background: var(--card);
		color: var(--foreground);
		cursor: pointer;
	}
	.signup-secondary:hover {
		background: var(--secondary);
	}
	.signup-primary:focus-visible,
	.signup-secondary:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
	}
</style>
