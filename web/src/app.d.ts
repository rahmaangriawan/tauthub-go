// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
	interface Window {
		// Cloudflare Turnstile (explicit render), dipakai komponen turnstile.svelte.
		turnstile?: {
			render: (el: HTMLElement, opts: Record<string, unknown>) => string;
			reset: (id?: string) => void;
		};
	}
	namespace App {
		// interface Error {}
		// interface Locals {}
		// interface PageData {}
		// interface PageState {}
		// interface Platform {}
	}
}

export {};
