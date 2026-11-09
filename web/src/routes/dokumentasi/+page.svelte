<script lang="ts">
	import { ArrowRight, BookText, PlugZap, Search, Send, ShieldCheck, UserPlus, Wallet } from '@lucide/svelte';
	import { searchUi } from '$lib/docs/search-state.svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const icons = {
		'user-plus': UserPlus,
		wallet: Wallet,
		send: Send,
		plug: PlugZap,
		shield: ShieldCheck,
		book: BookText
	} as const;
</script>

<main class="dk-home">
	<section class="dk-hero">
		<h1>Dokumentasi Tauthub</h1>
		<p>Panduan memakai Tauthub, dari mendaftar sampai artikel terbit, untuk Klien dan Mitra media.</p>
		<button class="dk-hero-search" type="button" onclick={() => (searchUi.open = true)}>
			<Search size={18} strokeWidth={1.5} aria-hidden="true" />
			<span>Cari panduan, misalnya "top up" atau "persetujuan"</span>
			<kbd>/</kbd>
		</button>
	</section>

	<section class="dk-doors" aria-label="Pilih peran Anda">
		<article class="dk-door">
			<h2>Saya Klien</h2>
			<p>Pilih media, siapkan artikel, lalu pantau sampai terbit di media pilihan Anda.</p>
			<div class="dk-door-actions">
				<a class="dk-btn dk-btn-primary" href={data.start.klien}>Mulai cepat <ArrowRight size={16} strokeWidth={1.5} /></a>
				<a class="dk-link" href="/dokumentasi/klien">Peta panduan Klien</a>
			</div>
		</article>
		<article class="dk-door">
			<h2>Saya Mitra</h2>
			<p>Kelola daftar media, setujui pesanan publikasi, dan pantau penghasilan Anda.</p>
			<div class="dk-door-actions">
				<a class="dk-btn dk-btn-primary" href={data.start.mitra}>Mulai cepat <ArrowRight size={16} strokeWidth={1.5} /></a>
				<a class="dk-link" href="/dokumentasi/mitra">Peta panduan Mitra</a>
			</div>
		</article>
	</section>

	<section class="dk-quick" aria-labelledby="quick-title">
		<h2 id="quick-title">Sering dibuka</h2>
		<ul>
			{#each data.quick as q (q.url)}
				{@const Icon = icons[q.icon as keyof typeof icons]}
				<li>
					<a href={q.url}>
						<span class="dk-quick-icon" aria-hidden="true">
							<Icon size={18} strokeWidth={1.5} />
						</span>
						<span class="dk-quick-text">
							<span class="dk-quick-title">{q.title}</span>
							<span class="dk-quick-note">{q.note}</span>
						</span>
					</a>
				</li>
			{/each}
		</ul>
	</section>

</main>
