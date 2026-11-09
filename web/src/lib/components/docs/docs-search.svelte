<script lang="ts">
	import { tick } from 'svelte';
	import { goto } from '$app/navigation';
	import { Search } from '@lucide/svelte';
	import { searchUi } from '$lib/docs/search-state.svelte';

	type Entry = { u: string; t: string; g: string; s: string; h: string[]; x: string };

	let index = $state<Entry[] | null>(null);
	let failed = $state(false);
	let q = $state('');
	let active = $state(0);
	let input = $state<HTMLInputElement | null>(null);

	async function load() {
		if (index || failed) return;
		try {
			const res = await fetch('/dokumentasi/search.json');
			if (!res.ok) throw new Error(String(res.status));
			index = (await res.json()) as Entry[];
		} catch {
			failed = true;
		}
	}

	$effect(() => {
		if (searchUi.open) {
			q = '';
			active = 0;
			void load();
			void tick().then(() => input?.focus());
		}
	});

	const results = $derived.by(() => {
		const terms = q.toLowerCase().split(/\s+/).filter(Boolean);
		if (!index || terms.length === 0) return [];
		const out: { e: Entry; score: number }[] = [];
		for (const e of index) {
			const t = e.t.toLowerCase();
			const h = e.h.join(' ').toLowerCase();
			const s = e.s.toLowerCase();
			const x = e.x.toLowerCase();
			let score = 0;
			let ok = true;
			for (const term of terms) {
				let sc = 0;
				if (t.includes(term)) sc += t.startsWith(term) ? 15 : 10;
				if (h.includes(term)) sc += 5;
				if (s.includes(term)) sc += 3;
				if (x.includes(term)) sc += 1;
				if (sc === 0) {
					ok = false;
					break;
				}
				score += sc;
			}
			if (ok) out.push({ e, score });
		}
		return out.sort((a, b) => b.score - a.score).slice(0, 8);
	});

	$effect(() => {
		void results;
		active = 0;
	});

	function close() {
		searchUi.open = false;
	}

	function open(url: string) {
		close();
		void goto(url);
	}

	function onkeydown(ev: KeyboardEvent) {
		if (ev.key === 'Escape') close();
		else if (ev.key === 'ArrowDown') {
			ev.preventDefault();
			active = Math.min(active + 1, results.length - 1);
		} else if (ev.key === 'ArrowUp') {
			ev.preventDefault();
			active = Math.max(active - 1, 0);
		} else if (ev.key === 'Enter' && results[active]) {
			ev.preventDefault();
			open(results[active].e.u);
		}
	}
</script>

{#if searchUi.open}
	<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
	<div class="dk-search-backdrop" onclick={close}>
		<div
			class="dk-search"
			role="dialog"
			aria-modal="true"
			aria-label="Cari dokumentasi"
			tabindex="-1"
			{onkeydown}
			onclick={(e) => e.stopPropagation()}
		>
			<div class="dk-search-field">
				<Search size={18} strokeWidth={1.5} aria-hidden="true" />
				<input
					bind:this={input}
					bind:value={q}
					type="search"
					placeholder="Cari panduan"
					aria-label="Kata kunci"
					autocomplete="off"
				/>
				<kbd>Esc</kbd>
			</div>
			<div class="dk-search-body">
				{#if failed}
					<p class="dk-search-note">Pencarian tidak dapat dimuat. Muat ulang halaman lalu coba lagi.</p>
				{:else if !index}
					<p class="dk-search-note">Memuat</p>
				{:else if q.trim() === ''}
					<p class="dk-search-note">Ketik kata kunci, misalnya "top up", "persetujuan", atau "WordPress".</p>
				{:else if results.length === 0}
					<p class="dk-search-note">Tidak ada hasil untuk "{q}".</p>
				{:else}
					<ul role="listbox">
						{#each results as r, i (r.e.u)}
							<li role="option" aria-selected={i === active}>
								<a
									href={r.e.u}
									class:active={i === active}
									onmouseenter={() => (active = i)}
									onclick={(ev) => {
										ev.preventDefault();
										open(r.e.u);
									}}
								>
									<span class="dk-search-title">{r.e.t}</span>
									<span class="dk-search-tag">{r.e.g}</span>
									{#if r.e.s}<span class="dk-search-sum">{r.e.s}</span>{/if}
								</a>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		</div>
	</div>
{/if}
