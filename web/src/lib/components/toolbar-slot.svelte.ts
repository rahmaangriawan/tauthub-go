import { getContext, setContext } from 'svelte';

// Slot sisi kiri utility bar — parity slot `utilityBar > div:first-child` app
// lama (agent-steps.tsx mem-portal AgentStepsHeader ke sana). Halaman yang
// ingin menaruh sesuatu di bar tidak bisa merender langsung ke layout, jadi
// elemen anchor-nya dibagikan lewat context dan node-nya dipindahkan.

const KEY = Symbol('toolbar-left-slot');

export type ToolbarSlot = {
	/** Elemen anchor di utility bar; null sebelum bar ter-mount. */
	el: HTMLElement | null;
};

/** Dipanggil layout sekali saat init; anak-anaknya membaca lewat getToolbarSlot(). */
export function setToolbarSlot(): ToolbarSlot {
	const slot = $state<ToolbarSlot>({ el: null });
	setContext(KEY, slot);
	return slot;
}

/** Null bila komponen dipakai di luar layout yang menyediakan slot. */
export function getToolbarSlot(): ToolbarSlot | null {
	return getContext<ToolbarSlot | null>(KEY) ?? null;
}
