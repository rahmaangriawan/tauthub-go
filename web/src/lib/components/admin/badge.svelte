<script lang="ts">
	// Badge status admin. `tone` mengikuti token app lama; label default =
	// nilai status apa adanya (parity Badge radix di app lama).
	let {
		label,
		tone = 'neutral',
		value
	}: { label?: string; tone?: 'neutral' | 'accent' | 'success' | 'warning' | 'danger'; value?: string } = $props();

	const styles: Record<string, { background: string; color: string }> = {
		neutral: { background: 'var(--secondary)', color: 'var(--muted-foreground)' },
		accent: { background: 'var(--accent)', color: 'var(--accent-foreground)' },
		success: { background: 'var(--success-soft)', color: 'var(--success)' },
		warning: { background: 'var(--warning-soft)', color: 'var(--warning)' },
		danger: { background: 'var(--destructive-soft, #fdecee)', color: 'var(--destructive)' }
	};
	const style = $derived(styles[tone] ?? styles.neutral);
</script>

<span class="rounded-full px-2 py-0.5 text-[11px] font-medium" style="background-color: {style.background}; color: {style.color};">
	{label ?? value ?? '—'}
</span>
