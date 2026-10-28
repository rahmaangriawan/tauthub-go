<script lang="ts">
	// Tooltip bergaya (parity Radix Tooltip app lama): muncul saat hover DAN
	// saat fokus keyboard, diposisikan fixed supaya tidak terpotong overflow
	// tabel/kartu, dengan panah kecil dan dukungan teks kaya via snippet.
	import type { Snippet } from 'svelte';

	let {
		content,
		rich,
		side = 'top',
		align = 'center',
		delay = 150,
		maxWidth = 260,
		label,
		children
	}: {
		content?: string;
		rich?: Snippet;
		side?: 'top' | 'bottom';
		align?: 'start' | 'center' | 'end';
		delay?: number;
		maxWidth?: number;
		label?: string;
		children: Snippet;
	} = $props();

	let anchor = $state<HTMLElement | null>(null);
	let panel = $state<HTMLDivElement | null>(null);
	let open = $state(false);
	let pos = $state({ left: 0, top: 0, arrowLeft: 0, place: 'top' as 'top' | 'bottom' });
	let timer: number | undefined;

	function show() {
		window.clearTimeout(timer);
		timer = window.setTimeout(() => {
			open = true;
			requestAnimationFrame(place);
		}, delay);
	}

	function hide() {
		window.clearTimeout(timer);
		open = false;
	}

	function place() {
		if (!anchor || !panel) return;
		const a = anchor.getBoundingClientRect();
		const p = panel.getBoundingClientRect();
		const gap = 8;
		// `side` adalah preferensi, bukan paksaan: tetap dibalik bila ruang di
		// sisi itu tidak cukup (mis. badge dekat tepi atas viewport).
		const fitsAbove = a.top - p.height - gap >= 8;
		const fitsBelow = a.bottom + p.height + gap <= window.innerHeight - 8;
		const above = side === 'bottom' ? !fitsBelow && fitsAbove : fitsAbove || !fitsBelow;
		const top = above ? a.top - p.height - gap : a.bottom + gap;
		let left: number;
		if (align === 'start') left = a.left;
		else if (align === 'end') left = a.right - p.width;
		else left = a.left + a.width / 2 - p.width / 2;
		left = Math.min(Math.max(8, left), Math.max(8, window.innerWidth - p.width - 8));
		const arrowLeft = Math.min(Math.max(12, a.left + a.width / 2 - left - 5), p.width - 22);
		pos = { left, top, arrowLeft, place: above ? 'top' : 'bottom' };
	}

	$effect(() => {
		if (!open) return;
		const onScroll = () => (open = false);
		window.addEventListener('scroll', onScroll, true);
		window.addEventListener('resize', onScroll);
		return () => {
			window.removeEventListener('scroll', onScroll, true);
			window.removeEventListener('resize', onScroll);
		};
	});
</script>

<span
	bind:this={anchor}
	class="zs-tip-anchor"
	role="presentation"
	aria-label={label}
	onmouseenter={show}
	onmouseleave={hide}
	onfocusin={show}
	onfocusout={hide}
>
	{@render children()}
</span>

{#if open}
	<div
		bind:this={panel}
		class="zs-tip"
		class:zs-tip-bottom={pos.place === 'bottom'}
		role="tooltip"
		aria-hidden="true"
		style="left: {pos.left}px; top: {pos.top}px; max-width: {maxWidth}px;"
	>
		{#if rich}
			{@render rich()}
		{:else}
			{content}
		{/if}
		<span class="zs-tip-arrow" style="left: {pos.arrowLeft}px;"></span>
	</div>
{/if}

<style>
	.zs-tip-anchor {
		display: inline-flex;
		align-items: center;
	}
	.zs-tip {
		position: fixed;
		z-index: 1300;
		padding: 7px 10px;
		border-radius: 8px;
		background: #0f172a;
		color: #f8fafc;
		font-size: 12px;
		font-weight: 400;
		line-height: 1.45;
		text-align: left;
		white-space: normal;
		box-shadow: 0 8px 24px rgba(15, 23, 42, 0.28);
		pointer-events: none;
		animation: zsTipIn 0.1s cubic-bezier(0.16, 1, 0.3, 1);
	}
	.zs-tip-arrow {
		position: absolute;
		width: 10px;
		height: 10px;
		background: #0f172a;
		transform: rotate(45deg);
	}
	/* Panah menghadap ke bawah (tooltip di atas anchor). */
	.zs-tip:not(.zs-tip-bottom) .zs-tip-arrow {
		bottom: -5px;
	}
	.zs-tip-bottom .zs-tip-arrow {
		top: -5px;
	}
	@keyframes zsTipIn {
		from {
			opacity: 0;
		}
		to {
			opacity: 1;
		}
	}
</style>
