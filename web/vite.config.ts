import adapter from '@sveltejs/adapter-node';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

declare const process: { env: Record<string, string | undefined> };

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			// Client diberi tahu bila ada build baru (aset lama bisa 404 setelah deploy).
			// Reaksinya ada di src/routes/+layout.svelte (updated + vite:preloadError).
			version: { pollInterval: 60_000 },
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			// deploy.sh membangun ke folder sementara lalu menukarnya, supaya node
			// yang sedang jalan tidak melayani file build yang setengah tertulis
			// (nginx: "upstream prematurely closed" pada /_app/version.json).
			adapter: adapter({ out: process.env.SVELTE_BUILD_OUT || 'build' })
		})
	],
	server: {
		// Dev di 3001 supaya tidak bentrok dengan app lama (Next.js :3000).
		// Produksi: nginx melayani web ini di 3000 dan mem-proxy /v1, /uploads,
		// /t, /feed ke API Go :8080 (REWRITE/02_FRONTEND_SVELTE.md §9).
		port: 3001,
		// Uji routing dua host secara lokal: curl -H "Host: data.tauthub.com" ...
		// (Vite menolak Host tak dikenal secara bawaan.)
		allowedHosts: ['data.tauthub.com', 'app.tauthub.com', '.tauthub.com'],
		proxy: {
			'/v1': 'http://localhost:8080',
			'/healthz': 'http://localhost:8080',
			'/uploads': 'http://localhost:8080',
			'/t': 'http://localhost:8080',
			'/feed': 'http://localhost:8080'
		}
	},
	ssr: {
		// @lucide/svelte mengirim ikon sebagai file .svelte — Vite harus
		// mengompilasinya di SSR, bukan membiarkannya jadi external Node.
		noExternal: ['@lucide/svelte']
	}
});
