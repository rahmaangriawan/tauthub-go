<script lang="ts">
	// Popup bantuan dasbor klien: penjelasan alur kerja Tauthub dan standar
	// Gatekeeper. Isi mengikuti perilaku platform yang sebenarnya (empat
	// pemeriksaan Gatekeeper, standar konten per media, dan alur kredit).
	import { BookOpen, ShieldCheck, X } from '@lucide/svelte';
	import { tr } from '$lib/i18n/phrases';

	let {
		open = $bindable(false),
		tab = 'how'
	}: {
		open?: boolean;
		tab?: 'how' | 'gate';
	} = $props();

	let panel = $state<HTMLDivElement | null>(null);
	let activeTab = $state<'how' | 'gate'>('how');

	// Tab awal mengikuti kartu yang diklik; diubah hanya saat popup dibuka
	// supaya pergantian tab oleh pengguna tidak ditimpa.
	let wasOpen = false;
	$effect(() => {
		if (open && !wasOpen) activeTab = tab;
		wasOpen = open;
	});

	const HOW_STEPS = [
		{
			title: 'Isi saldo kredit',
			body: 'Kredit dipakai saat artikel didistribusikan. 1 kredit setara Rp 1.000, dan saldo bisa ditambah dari menu Keuangan.'
		},
		{
			title: 'Siapkan artikel',
			body: 'Tulis sendiri di editor, atau minta Agen AI menyusunnya dari brief kampanye Anda. Artikel bisa diperiksa berulang kali sebelum dikirim.'
		},
		{
			title: 'Lolos Gatekeeper',
			body: 'Empat pemeriksaan otomatis berjalan atas artikel: keamanan konten, kesehatan tautan, kemiripan teks, dan batas gaya bahasa.'
		},
		{
			title: 'Pilih media lalu distribusi',
			body: 'Pilih media sendiri dari katalog, atau ikuti rekomendasi AI Matchmaker. Kredit diamankan lebih dulu dan baru diteruskan setelah publikasi sah.'
		},
		{
			title: 'Mitra menerbitkan artikel',
			body: 'Mitra menerbitkan otomatis lewat integrasi (WordPress, Blogger, atau Web Custom API) atau secara manual, lalu mengisi URL terbitnya.'
		},
		{
			title: 'Verifikasi, pantau, dan laporan',
			body: 'URL terbit diverifikasi dan Link Monitor terus memantau tautannya. Laporan jangkauan, klik smart link, dan skor PR tersedia di dasbor.'
		}
	];

	const GATE_CHECKS = [
		{
			title: 'Illegal Content Guard',
			gate: true,
			body: 'Menilai teks dan tautan artikel terhadap konten terlarang seperti judi, pornografi, dan malware.'
		},
		{
			title: 'Link Health Validator',
			gate: true,
			body: 'Menguji setiap tautan di dalam artikel. Tautan mati atau yang mengarah ke situs berisiko ditolak.'
		},
		{
			title: 'AI Overview Checker',
			gate: false,
			body: 'Menilai kesiapan artikel untuk Google AI Overviews: struktur (BLUF), kepadatan entitas, sinyal E-E-A-T, dan penyelarasan niat pengguna. Skor total dari 40; di bawah ambang berarti perlu revisi. Bersifat informasi, bukan penghalang distribusi.'
		},
		{
			title: 'Brand Voice Limits',
			gate: false,
			body: 'Membandingkan jumlah kata dengan batas media, menghitung istilah promosi, dan menandai frasa terlarang.'
		}
	];

	const MEDIA_STANDARDS = [
		['Jumlah kata', 'Setiap media menetapkan batas minimum dan maksimum kata.'],
		['Gaya jurnalistik', 'Nada penulisan yang diharapkan media, misalnya gaya berita atau ulasan.'],
		['Toleransi hard selling', 'Seberapa jauh artikel boleh bernada promosi.'],
		['Kewajiban kutipan', 'Media tertentu mewajibkan adanya narasumber atau kutipan.']
	];

	$effect(() => {
		if (!open) return;
		panel?.focus();
		const onKey = (event: KeyboardEvent) => {
			if (event.key === 'Escape') open = false;
		};
		window.addEventListener('keydown', onKey);
		const previous = document.body.style.overflow;
		document.body.style.overflow = 'hidden';
		return () => {
			window.removeEventListener('keydown', onKey);
			document.body.style.overflow = previous;
		};
	});
</script>

{#if open}
	<div
		class="help-backdrop"
		role="presentation"
		onclick={() => (open = false)}
		onkeydown={(event) => {
			if (event.key === 'Enter' || event.key === ' ') open = false;
		}}
	></div>
	<div
		bind:this={panel}
		class="help-dialog"
		role="dialog"
		aria-modal="true"
		aria-labelledby="help-title"
		tabindex="-1"
	>
		<header class="help-head">
			<div class="help-head-text">
				<h2 id="help-title">{tr('Panduan & Bantuan')}</h2>
				<p>{tr('Alur kerja Tauthub dan standar yang harus dilewati artikel Anda.')}</p>
			</div>
			<button class="help-close" type="button" aria-label={tr('Tutup panduan')} onclick={() => (open = false)}>
				<X size={16} />
			</button>
		</header>

		<div class="help-tabs" role="tablist" aria-label={tr('Bagian panduan')}>
			<button
				class="help-tab"
				class:help-tab-active={activeTab === 'how'}
				type="button"
				role="tab"
				aria-selected={activeTab === 'how'}
				onclick={() => (activeTab = 'how')}
			>
				<BookOpen size={14} /> {tr('Cara Kerja Tauthub')}
			</button>
			<button
				class="help-tab"
				class:help-tab-active={activeTab === 'gate'}
				type="button"
				role="tab"
				aria-selected={activeTab === 'gate'}
				onclick={() => (activeTab = 'gate')}
			>
				<ShieldCheck size={14} /> {tr('Standar Gatekeeper')}
			</button>
		</div>

		<div class="help-body">
			{#if activeTab === 'how'}
				<ol class="help-steps">
					{#each HOW_STEPS as step, index (step.title)}
						<li>
							<span class="help-step-no">{index + 1}</span>
							<div>
								<p class="help-step-title">{tr(step.title)}</p>
								<p class="help-step-body">{tr(step.body)}</p>
							</div>
						</li>
					{/each}
				</ol>
				<p class="help-note">
					{tr('Kredit hanya terpakai setelah publikasi sah. Kalau artikel gagal di tengah jalan, kredit dikembalikan ke saldo Anda.')}
				</p>
			{:else}
				<ul class="help-checks">
					{#each GATE_CHECKS as check (check.title)}
						<li>
							<div class="help-check-head">
								<span class="help-check-title">{check.title}</span>
								{#if check.gate}
									<span class="help-badge help-badge-gate">{tr('Hard gate')}</span>
								{:else}
									<span class="help-badge">{tr('Informasi')}</span>
								{/if}
							</div>
							<p class="help-check-body">{tr(check.body)}</p>
						</li>
					{/each}
				</ul>
				<h3 class="help-sub">{tr('Standar konten dari media')}</h3>
				<p class="help-check-body">
					{tr('Selain pemeriksaan platform, setiap media punya standar sendiri yang diperiksa sebelum artikel diterbitkan:')}
				</p>
				<dl class="help-media">
					{#each MEDIA_STANDARDS as [label, value] (label)}
						<div>
							<dt>{tr(label)}</dt>
							<dd>{tr(value)}</dd>
						</div>
					{/each}
				</dl>
				<p class="help-note">
					{tr('Artikel yang tidak lolos hard gate tidak bisa didistribusikan. Perbaiki di editor lalu jalankan pemeriksaan ulang — tidak ada biaya tambahan sampai publikasinya sah.')}
				</p>
			{/if}
		</div>
	</div>
{/if}

<style>
	.help-backdrop {
		position: fixed;
		inset: 0;
		z-index: 1200;
		border: none;
		background: rgba(15, 23, 42, 0.45);
	}
	.help-dialog {
		position: fixed;
		z-index: 1210;
		top: 50%;
		left: 50%;
		display: flex;
		flex-direction: column;
		width: min(560px, calc(100vw - 32px));
		max-height: min(640px, calc(100vh - 48px));
		transform: translate(-50%, -50%);
		overflow: hidden;
		border: 1px solid var(--border);
		border-radius: 16px;
		background: var(--card, #fff);
		color: var(--card-foreground, #111827);
		box-shadow: 0 24px 60px rgba(15, 23, 42, 0.24);
	}
	.help-dialog:focus {
		outline: none;
	}
	.help-head {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 12px;
		padding: 18px 20px 12px;
	}
	.help-head-text h2 {
		margin: 0;
		font-size: 17px;
		font-weight: 700;
		letter-spacing: -0.01em;
	}
	.help-head-text p {
		margin: 4px 0 0;
		font-size: 13px;
		line-height: 1.5;
		color: var(--muted-foreground);
	}
	.help-close {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		flex: none;
		border: none;
		border-radius: 8px;
		background: transparent;
		color: var(--muted-foreground);
		cursor: pointer;
	}
	.help-close:hover {
		background: var(--secondary);
		color: var(--foreground);
	}
	.help-close:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: 2px;
	}
	.help-tabs {
		display: flex;
		gap: 6px;
		padding: 0 20px;
		border-bottom: 1px solid var(--border);
	}
	.help-tab {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		margin-bottom: -1px;
		padding: 9px 4px 10px;
		border: none;
		border-bottom: 2px solid transparent;
		background: transparent;
		font-size: 13px;
		font-weight: 600;
		color: var(--muted-foreground);
		cursor: pointer;
	}
	.help-tab + .help-tab {
		margin-left: 14px;
	}
	.help-tab-active {
		border-bottom-color: var(--primary);
		color: var(--primary);
	}
	.help-tab:focus-visible {
		outline: 2px solid var(--primary);
		outline-offset: -2px;
		border-radius: 6px;
	}
	.help-body {
		overflow-y: auto;
		padding: 18px 20px 22px;
	}
	.help-steps {
		margin: 0;
		padding: 0;
		list-style: none;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	.help-steps li {
		display: flex;
		gap: 12px;
	}
	.help-step-no {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 22px;
		height: 22px;
		flex: none;
		border-radius: 999px;
		background: var(--secondary, #eef2f8);
		color: var(--foreground);
		font-size: 11.5px;
		font-weight: 700;
	}
	.help-step-title {
		margin: 0;
		font-size: 14px;
		font-weight: 600;
	}
	.help-step-body {
		margin: 3px 0 0;
		font-size: 13px;
		line-height: 1.6;
		color: var(--muted-foreground);
	}
	.help-checks {
		margin: 0;
		padding: 0;
		list-style: none;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	.help-check-head {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.help-check-title {
		font-size: 14px;
		font-weight: 600;
	}
	.help-badge {
		padding: 2px 8px;
		border-radius: 999px;
		background: var(--secondary, #eef2f8);
		color: var(--muted-foreground);
		font-size: 10.5px;
		font-weight: 700;
		letter-spacing: 0.02em;
	}
	.help-badge-gate {
		background: var(--warning-soft, #fff4e5);
		color: var(--warning, #b45309);
	}
	.help-check-body {
		margin: 4px 0 0;
		font-size: 13px;
		line-height: 1.6;
		color: var(--muted-foreground);
	}
	.help-sub {
		margin: 22px 0 0;
		font-size: 13px;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--muted-foreground);
	}
	.help-media {
		margin: 12px 0 0;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.help-media dt {
		font-size: 13px;
		font-weight: 600;
	}
	.help-media dd {
		margin: 2px 0 0;
		font-size: 13px;
		line-height: 1.6;
		color: var(--muted-foreground);
	}
	.help-note {
		margin: 18px 0 0;
		padding: 10px 12px;
		border-radius: 10px;
		background: var(--secondary, #eef2f8);
		font-size: 12.5px;
		line-height: 1.6;
		color: var(--muted-foreground);
	}
	@media (max-width: 640px) {
		.help-dialog {
			width: 100vw;
			max-height: 100vh;
			height: 100%;
			border: none;
			border-radius: 0;
			transform: none;
			top: 0;
			left: 0;
		}
		.help-tab {
			padding: 12px 4px;
		}
	}
</style>
