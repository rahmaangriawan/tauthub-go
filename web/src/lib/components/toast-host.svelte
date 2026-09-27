<script lang="ts">
	// Port components/toast-host.tsx — satu toast, kanan-bawah, auto tutup 3,5s.
	import { CheckCircle, XCircle, X } from '@lucide/svelte';
	import { TOAST_EVENT, type ToastDetail } from '$lib/auth';

	let toast = $state<ToastDetail | null>(null);

	$effect(() => {
		const onToast = (event: Event) => {
			const detail = (event as CustomEvent<ToastDetail>).detail;
			toast = detail;
			window.setTimeout(() => {
				if (toast === detail) toast = null;
			}, 3500);
		};
		window.addEventListener(TOAST_EVENT, onToast);
		return () => window.removeEventListener(TOAST_EVENT, onToast);
	});
</script>

{#if toast}
	<div
		role="status"
		aria-live="polite"
		class="fixed bottom-5 right-5 z-[1000] flex min-w-[260px] max-w-[380px] items-center gap-2.5 rounded-xl border border-border bg-card px-3.5 py-3 shadow-[0_12px_32px_rgba(0,0,0,.18)]"
	>
		{#if toast.type === 'success'}
			<CheckCircle size={19} class="text-emerald-600" />
		{:else}
			<XCircle size={19} class="text-red-600" />
		{/if}
		<span class="flex-1 text-[13px] font-semibold">{toast.message}</span>
		<button
			type="button"
			aria-label="Tutup notifikasi"
			class="cursor-pointer rounded p-0.5 text-muted-foreground hover:bg-accent"
			onclick={() => (toast = null)}
		>
			<X size={16} />
		</button>
	</div>
{/if}
