<script lang="ts">
	import type { Snippet } from 'svelte';
	import { tr } from '$lib/i18n/phrases';
	import { X } from '@lucide/svelte';

	// Panel geser dari kanan. App belum punya pola ini — .app-drawer hanya
	// geser dari kiri untuk nav mobile — jadi pola aksesibilitasnya disamakan
	// dengan drawer itu supaya perilaku keyboard konsisten di seluruh app.
	let {
		open = true,
		title,
		subtitle = '',
		closeLabel = tr('Tutup'),
		onClose,
		children,
		footer
	}: {
		open?: boolean;
		title: string;
		subtitle?: string;
		closeLabel?: string;
		onClose: () => void;
		children: Snippet;
		footer?: Snippet;
	} = $props();

	let panel = $state<HTMLDivElement | null>(null);

	// Kunci scroll latar, Escape menutup, fokus pindah ke panel lalu kembali ke
	// tombol pemicu, dan Tab dijebak di dalam panel selama panel terbuka.
	$effect(() => {
		if (!open) return;
		const previous = document.body.style.overflow;
		const opener = document.activeElement as HTMLElement | null;
		document.body.style.overflow = 'hidden';
		panel?.focus();
		const focusables = () =>
			Array.from(
				panel?.querySelectorAll<HTMLElement>(
					'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'
				) ?? []
			).filter((el) => el.offsetParent !== null);
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape') {
				event.stopPropagation();
				onClose();
				return;
			}
			if (event.key !== 'Tab') return;
			const items = focusables();
			if (items.length === 0) return;
			const first = items[0];
			const last = items[items.length - 1];
			const active = document.activeElement as HTMLElement | null;
			if (event.shiftKey && (active === first || active === panel)) {
				event.preventDefault();
				last.focus();
			} else if (!event.shiftKey && active === last) {
				event.preventDefault();
				first.focus();
			}
		};
		window.addEventListener('keydown', onKey);
		return () => {
			document.body.style.overflow = previous;
			window.removeEventListener('keydown', onKey);
			opener?.focus();
		};
	});
</script>

{#if open}
	<div class="slideOver">
		<button class="slideOverBackdrop" type="button" tabindex="-1" aria-label={closeLabel} onclick={onClose}></button>
		<div class="slideOverPanel" role="dialog" aria-modal="true" aria-label={title} tabindex="-1" bind:this={panel}>
			<header class="slideOverHead">
				<div class="min-w-0 flex-1">
					<p class="slideOverTitle">{title}</p>
					{#if subtitle}<p class="slideOverSub">{subtitle}</p>{/if}
				</div>
				<button class="slideOverClose" type="button" aria-label={closeLabel} onclick={onClose}><X size={18} /></button>
			</header>
			<div class="slideOverBody">{@render children()}</div>
			{#if footer}<div class="slideOverFoot">{@render footer()}</div>{/if}
		</div>
	</div>
{/if}

<style>
	.slideOver {
		position: fixed;
		z-index: 40;
		inset: 0;
	}
	.slideOverBackdrop {
		position: absolute;
		inset: 0;
		border: 0;
		padding: 0;
		background-color: rgba(15, 23, 42, 0.45);
		cursor: default;
		animation: slideOverFade 140ms ease-out;
	}
	.slideOverPanel {
		position: absolute;
		top: 0;
		bottom: 0;
		right: 0;
		display: flex;
		flex-direction: column;
		width: min(92vw, 440px);
		height: 100dvh;
		border-left: 1px solid var(--border);
		background-color: var(--card);
		box-shadow: var(--shadow-md);
		animation: slideOverIn 200ms ease-out;
	}
	.slideOverPanel:focus {
		outline: none;
	}
	.slideOverHead {
		display: flex;
		align-items: flex-start;
		gap: 12px;
		flex: 0 0 auto;
		padding: 16px 18px 12px;
		border-bottom: 1px solid var(--border);
	}
	.slideOverTitle {
		font-weight: 700;
		line-height: 1.25;
	}
	.slideOverSub {
		margin-top: 2px;
		font-size: 12.5px;
		color: var(--muted-foreground);
	}
	.slideOverClose {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex: 0 0 auto;
		width: 32px;
		height: 32px;
		border-radius: 8px;
		color: var(--muted-foreground);
	}
	.slideOverClose:hover {
		background-color: var(--muted);
	}
	.slideOverBody {
		flex: 1 1 auto;
		overflow-y: auto;
		padding: 16px 18px;
	}
	.slideOverFoot {
		flex: 0 0 auto;
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		padding: 12px 18px;
		border-top: 1px solid var(--border);
		background-color: var(--card);
	}

	@keyframes slideOverIn {
		from {
			transform: translateX(28px);
			opacity: 0;
		}
		to {
			transform: none;
			opacity: 1;
		}
	}
	@keyframes slideOverFade {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.slideOverPanel,
		.slideOverBackdrop {
			animation: none;
		}
	}
</style>
