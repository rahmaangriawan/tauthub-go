<script lang="ts">
	import SiteShell from '$lib/components/public/site-shell.svelte';
	import { idr, compact, tierLabel } from '$lib/public/format';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const presets = [1_000_000, 3_000_000, 5_000_000, 10_000_000, 25_000_000];

	const packages = $derived(data.result?.packages ?? []);
	const hasAny = $derived(packages.some((p) => p.count > 0));
	const cheapest = $derived(
		packages
			.flatMap((p) => p.items.map((i) => i.price))
			.reduce<number | null>((min, v) => (min === null || v < min ? v : min), null)
	);

	function presetHref(value: number): string {
		const qs = new URLSearchParams();
		qs.set('anggaran', String(value));
		if (data.form.kategori) qs.set('kategori', data.form.kategori);
		if (data.form.tier) qs.set('tier', data.form.tier);
		if (data.form.da) qs.set('da', data.form.da);
		if (data.form.jumlah) qs.set('jumlah', data.form.jumlah);
		return `/simulasi?${qs.toString()}`;
	}
</script>

<SiteShell>
	<main id="main" class="pub-main">
		<div class="pub-container sim">
			<section class="pub-hero" aria-labelledby="sim-title">
				<h1 id="sim-title">Simulasi paket publikasi</h1>
				<p>
					Masukkan anggaran Anda. Kami susun tiga paket contoh dari media yang ada di katalog, dengan harga yang sama
					seperti di katalog.
				</p>
			</section>

			<form class="pub-panel sim-form" method="get" action="/simulasi">
				<div class="sim-field sim-field-wide">
					<label for="sim-anggaran">Anggaran (Rp)</label>
					<input
						id="sim-anggaran"
						name="anggaran"
						type="number"
						inputmode="numeric"
						min="100000"
						step="1000"
						placeholder="5000000"
						value={data.form.anggaran}
						required
						aria-describedby={data.budgetError ? 'sim-anggaran-error' : undefined}
					/>
					{#if data.budgetError}
						<p id="sim-anggaran-error" class="sim-error" role="alert">{data.budgetError}</p>
					{/if}
				</div>
				<div class="sim-field">
					<label for="sim-kategori">Kategori</label>
					<select id="sim-kategori" name="kategori">
						<option value="">Semua kategori</option>
						{#each data.categories as c (c.slug)}
							<option value={c.slug} selected={data.form.kategori === c.slug}>{c.name}</option>
						{/each}
					</select>
				</div>
				<div class="sim-field">
					<label for="sim-tier">Tier media</label>
					<select id="sim-tier" name="tier">
						<option value="">Semua tier</option>
						<option value="tier1" selected={data.form.tier === 'tier1'}>Tier 1</option>
						<option value="tier2" selected={data.form.tier === 'tier2'}>Tier 2</option>
						<option value="tier3" selected={data.form.tier === 'tier3'}>Tier 3</option>
					</select>
				</div>
				<div class="sim-field">
					<label for="sim-da">DA minimal</label>
					<input id="sim-da" name="da" type="number" min="0" max="100" placeholder="0" value={data.form.da} />
				</div>
				<div class="sim-field">
					<label for="sim-jumlah">Jumlah media (opsional)</label>
					<input id="sim-jumlah" name="jumlah" type="number" min="1" max="30" placeholder="Otomatis" value={data.form.jumlah} />
				</div>
				<div class="sim-actions">
					<button class="pub-btn pub-btn-primary" type="submit">Susun paket</button>
				</div>
			</form>

			<p class="sim-presets">
				Cepat:
				{#each presets as p (p)}
					<a href={presetHref(p)}>{idr(p)}</a>
				{/each}
			</p>

			{#if data.failed}
				<div class="pub-panel" role="alert">
					<p>Simulasi belum bisa dimuat. Coba lagi sebentar lagi.</p>
				</div>
			{:else if data.result}
				{#if !hasAny}
					<div class="pub-panel" role="status">
						<h2>Belum ada media yang cocok</h2>
						<p>
							Tidak ada media dengan harga di bawah {idr(data.result.budget)} untuk filter ini. Naikkan anggaran atau
							longgarkan kategori, tier, dan DA minimal.
						</p>
					</div>
				{:else}
					<div class="sim-grid">
						{#each packages as pkg (pkg.key)}
							<section class="pub-panel sim-pkg" aria-labelledby={`pkg-${pkg.key}`}>
								<h2 id={`pkg-${pkg.key}`}>{pkg.label}</h2>
								<p class="sim-summary">{pkg.summary}</p>
								<dl class="sim-stats">
									<div><dt>Total</dt><dd>{idr(pkg.total)}</dd></div>
									<div><dt>Media</dt><dd>{pkg.count}</dd></div>
									<div><dt>Rata-rata DA</dt><dd>{pkg.avg_da ?? '-'}</dd></div>
									<div><dt>Trafik gabungan</dt><dd>{compact(pkg.traffic_total) ?? '-'}/bln</dd></div>
								</dl>
								<p class="sim-left">Sisa anggaran {idr(pkg.leftover)}</p>
								<ol class="sim-list">
									{#each pkg.items as m (m.slug)}
										<li>
											<a href={m.url}>{m.name}</a>
											<span class="sim-meta">{tierLabel(m.tier)} · DA {m.da ?? '-'}</span>
											<span class="sim-price">{idr(m.price)}</span>
										</li>
									{/each}
								</ol>
							</section>
						{/each}
					</div>
					<p class="sim-note">
						Ini simulasi dari harga katalog saat ini{cheapest !== null ? `, media termurah ${idr(cheapest)}` : ''}. Harga
						dan ketersediaan bisa berubah. Pesan lewat akun Tauthub untuk harga final.
					</p>
				{/if}
			{/if}
		</div>
	</main>
</SiteShell>

<style>
	.sim {
		padding-bottom: 48px;
	}
	.sim-form {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 16px;
		align-items: end;
	}
	.sim-field {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.sim-field-wide {
		grid-column: span 2;
	}
	.sim-field label {
		font-size: 13px;
		font-weight: 500;
	}
	.sim-field input,
	.sim-field select {
		min-height: 44px;
		padding: 0 12px;
		border: 1px solid var(--border, #dfe5ef);
		border-radius: 10px;
		background: #fff;
		font: inherit;
	}
	.sim-error {
		color: #b42318;
		font-size: 13px;
		margin: 0;
	}
	.sim-actions {
		display: flex;
		align-items: end;
	}
	.sim-presets {
		margin: 12px 0 24px;
		font-size: 14px;
		display: flex;
		flex-wrap: wrap;
		gap: 8px 16px;
	}
	.sim-grid {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 16px;
		align-items: start;
	}
	.sim-summary {
		color: var(--muted-foreground, #5b6475);
		font-size: 14px;
	}
	.sim-stats {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 12px 16px;
		margin: 16px 0 8px;
	}
	.sim-stats dt {
		font-size: 12px;
		color: var(--muted-foreground, #5b6475);
	}
	.sim-stats dd {
		margin: 2px 0 0;
		font-size: 18px;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}
	.sim-left {
		font-size: 13px;
		color: var(--muted-foreground, #5b6475);
		margin: 0 0 16px;
	}
	.sim-list {
		list-style: none;
		margin: 0;
		padding: 0;
	}
	.sim-list li {
		display: grid;
		grid-template-columns: 1fr auto;
		gap: 2px 12px;
		padding: 10px 0;
		border-top: 1px solid var(--border, #dfe5ef);
		font-size: 14px;
	}
	.sim-meta {
		grid-column: 1;
		font-size: 12px;
		color: var(--muted-foreground, #5b6475);
	}
	.sim-price {
		grid-column: 2;
		grid-row: 1 / span 2;
		align-self: center;
		font-variant-numeric: tabular-nums;
		font-weight: 500;
	}
	.sim-note {
		margin-top: 20px;
		font-size: 13px;
		color: var(--muted-foreground, #5b6475);
	}
	@media (max-width: 960px) {
		.sim-form {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
		.sim-grid {
			grid-template-columns: 1fr;
		}
	}
	@media (max-width: 560px) {
		.sim-form {
			grid-template-columns: 1fr;
		}
		.sim-field-wide {
			grid-column: auto;
		}
	}
</style>
