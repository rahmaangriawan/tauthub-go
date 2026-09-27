<script lang="ts">
	// Tahap 0 — Content Intake (parity blok `flowStatus === "intake"` app lama).
	import { RefreshCw, Sparkle, X } from '@lucide/svelte';
	import { MODES, LEVEL_COLOR, LEVEL_LABEL, readArticleFile, type Question } from './_flow-state.svelte';
	import type { FlowState } from './_flow-state.svelte';
	import TrailingDots from './_trailing-dots.svelte';

	let { flow }: { flow: FlowState } = $props();

	let fileEl = $state<HTMLInputElement | null>(null);
	let fileErr = $state('');

	const currentType = $derived(flow.currentType);
	const showArticle = $derived(flow.intakeMode === 'artikel' || flow.intakeMode === 'keduanya');
	const readiness = $derived(flow.readiness);

	const QUESTION_GROUPS = [
		{ id: 'inti', title: 'Informasi utama', hint: 'Inti kejadian dan alasan mengapa layak diberitakan.', keys: ['who', 'what', 'why'] },
		{ id: 'waktu', title: 'Waktu & lokasi', hint: 'Kapan dan di mana.', keys: ['when', 'where'] },
		{ id: 'pihak', title: 'Pihak & entitas', hint: 'Orang, perusahaan, produk, atau acara.', keys: ['event_type', 'company', 'product', 'person'] },
		{ id: 'dukungan', title: 'Data & dukungan', hint: 'Angka, kutipan, atau ajakan.', keys: ['data', 'quote', 'cta'] }
	];

	const groups = $derived.by(() => {
		const questions = currentType?.questions ?? [];
		const list = QUESTION_GROUPS.map((g) => ({ ...g, questions: [] as Question[] }));
		const others: Question[] = [];
		for (const q of questions) {
			const group = list.find((g) => g.keys.includes(q.key));
			if (group) group.questions.push(q);
			else others.push(q);
		}
		const result = list.filter((g) => g.questions.length > 0);
		if (others.length > 0) result.push({ id: 'lainnya', title: 'Informasi lainnya', hint: '', keys: [], questions: others });
		return result;
	});

	const isWide = (q: Question) => ['textarea', 'radio', 'checkbox', 'multi_select'].includes(q.type);

	function setAnswer(key: string, value: unknown) {
		flow.answers = { ...flow.answers, [key]: value };
	}

	async function pickFile(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		if (!file) return;
		fileErr = '';
		const result = await readArticleFile(file, (v) => (fileErr = v));
		if (result) {
			flow.articleText = result.text;
			flow.articleFilename = result.filename;
		}
		input.value = '';
	}
</script>

{#if !flow.campaign?.media_type}
	<h2 class="card-title">Pilih tujuan publikasi</h2>
	<p class="card-hint">Artikel ini akan diterbitkan ke mana? Pilih jenis media terlebih dahulu.</p>
	<div class="intake-mode-grid">
		<button type="button" class="intake-mode-card" disabled={flow.mediaTypeBusy} onclick={() => void flow.saveMediaType('blogger')}>
			<span class="intake-mode-title">Media Blogger</span>
			<span class="intake-mode-desc">Artikel untuk blog/publisher yang lebih fleksibel dan cocok untuk konten informasi, evergreen, tutorial, opini, dan konten niche.</span>
		</button>
		<button type="button" class="intake-mode-card" disabled={flow.mediaTypeBusy} onclick={() => void flow.saveMediaType('mainstream')}>
			<span class="intake-mode-title">Media Mainstream</span>
			<span class="intake-mode-desc">Artikel untuk media berita/publisher yang mengikuti standar jurnalistik dan newsworthiness.</span>
		</button>
	</div>
	{#if flow.mediaTypeBusy}<p class="card-hint" style="margin-top:8px;">Menyimpan…</p>{/if}
{:else}
	{#if flow.intakeStep === 0}
		<h2 class="card-title">Content Intake</h2>
		<p class="card-hint">Pilih cara Anda ingin memulai</p>
		<div class="intake-mode-grid">
			{#each MODES as item (item.value)}
				<button
					type="button"
					class={flow.intakeMode === item.value ? 'intake-mode-card intake-mode-active' : 'intake-mode-card'}
					onclick={() => (flow.intakeMode = item.value)}
				>
					<span class="intake-mode-title">{item.title}</span>
					<span class="intake-mode-desc">{item.desc}</span>
				</button>
			{/each}
		</div>
		<div class="actions">
			<button class="app-btn app-btn-primary" type="button" disabled={!flow.intakeMode || flow.busy} onclick={() => void flow.createIntake()}>
				Lanjut
			</button>
		</div>
	{/if}

	{#if flow.intakeStep === 2 && flow.intake}
		<h2 class="card-title">Lengkapi Informasi</h2>

		<div class="pair-fields" style="margin-top:14px;">
			<label class="field-label">
				Jenis konten
				<select class="app-input" bind:value={flow.typeCode}>
					<option value="">Pilih jenis naskah…</option>
					{#each flow.types as t (t.code)}<option value={t.code}>{t.name}</option>{/each}
				</select>
			</label>
			<label class="field-label">
				Topik / judul kerja
				<input class="app-input" bind:value={flow.topic} placeholder="Topik artikel" />
			</label>
			<span class="card-hint">Panjang artikel (jumlah kata) diatur admin di Pengaturan Konten Brand — tidak diubah di flow ini.</span>
		</div>

		{#if showArticle}
			<div class="intake-article-box" style="margin-top:16px;">
				<h3 style="font-size:15px;font-weight:650;margin:0;color:var(--foreground);">Masukkan Naskah</h3>
				<p class="card-hint" style="margin:4px 0 10px;">Unggah file <strong>.txt</strong> atau <strong>.docx</strong>, atau tempel naskah langsung.</p>
				<div class="intake-file-row" style="margin-bottom:10px;">
					<button class="app-btn app-btn-soft" type="button" onclick={() => fileEl?.click()}>Unggah naskah</button>
					{#if flow.articleFilename}
						<span class="intake-kbd">
							✓ {flow.articleFilename} · {flow.articleText.trim()
								? `${flow.articleText.trim().split(/\s+/).length.toLocaleString('id-ID')} kata`
								: 'kosong'}
						</span>
					{/if}
					<input
						bind:this={fileEl}
						type="file"
						accept=".txt,.docx"
						hidden
						onchange={pickFile}
					/>
				</div>
				{#if fileErr}<p class="card-hint" style="color:var(--destructive);">{fileErr}</p>{/if}
				<label class="field-label">
					Atau masukkan naskah langsung
					<textarea class="app-input" rows="12" bind:value={flow.articleText} placeholder="Tempel atau tulis naskah Anda di sini..."></textarea>
				</label>
				<p class="card-hint">Preview naskah tampil di atas — isi naskah inilah yang akan digunakan saat melanjutkan.</p>
			</div>
		{/if}

		{#if !showArticle}
			<div style="margin-top:18px;">
				<div style="display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;">
					<h3 style="font-size:15px;font-weight:650;margin:0;color:var(--foreground);">
						{flow.intakeMode === 'rekomendasi' ? 'Isi sebanyak yang Anda tahu' : 'Informasi Naskah'}
					</h3>
					<button
						type="button"
						class="app-btn app-btn-soft"
						title="Biaya 2 Kredit — mengisi field kosong + key messages sekaligus"
						onclick={() => {
							if (flow.topic.trim()) flow.aiFillTopic = flow.topic.trim();
							flow.aiFillOpen = true;
						}}
					>
						<Sparkle size={14} /> Isi Otomatis (AI) · 2 TC
					</button>
				</div>
				<p class="card-hint" style="margin:4px 0 12px;">
					{flow.aiFillNote || 'Malas mengisi manual? AI akan mengisi field yang masih kosong berdasarkan topik.'}
				</p>

				{#if (currentType?.questions ?? []).length > 0}
					<div>
						{#each groups as group (group.id)}
							<div class="intake-group">
								<h4 class="intake-group-title">{group.title}</h4>
								{#if group.hint}<p class="intake-group-hint">{group.hint}</p>{/if}
								<div class="intake-fields">
									{#each group.questions as q (q.key)}
										<label class={isWide(q) ? 'field-label intake-field-wide' : 'field-label'}>
											{q.label} {#if q.required}<span style="color:var(--destructive);">*</span>{/if}
											{#if q.type === 'textarea'}
												<textarea class="app-input" rows="3" placeholder={q.placeholder ?? ''} value={typeof flow.answers[q.key] === 'string' ? (flow.answers[q.key] as string) : ''} oninput={(e) => setAnswer(q.key, (e.currentTarget as HTMLTextAreaElement).value)}></textarea>
											{:else if q.type === 'number'}
												<input class="app-input" type="number" placeholder={q.placeholder ?? ''} value={typeof flow.answers[q.key] === 'string' ? (flow.answers[q.key] as string) : ''} oninput={(e) => setAnswer(q.key, (e.currentTarget as HTMLInputElement).value)} />
											{:else if q.type === 'date'}
												<input class="app-input" type="date" value={typeof flow.answers[q.key] === 'string' ? (flow.answers[q.key] as string) : ''} oninput={(e) => setAnswer(q.key, (e.currentTarget as HTMLInputElement).value)} />
											{:else if q.type === 'url'}
												<input class="app-input" type="url" placeholder="https://…" value={typeof flow.answers[q.key] === 'string' ? (flow.answers[q.key] as string) : ''} oninput={(e) => setAnswer(q.key, (e.currentTarget as HTMLInputElement).value)} />
											{:else if q.type === 'select'}
												<select class="app-input" value={typeof flow.answers[q.key] === 'string' ? (flow.answers[q.key] as string) : ''} onchange={(e) => setAnswer(q.key, (e.currentTarget as HTMLSelectElement).value)}>
													<option value="">Pilih…</option>
													{#each q.options ?? [] as o (o)}<option value={o}>{o}</option>{/each}
												</select>
											{:else if q.type === 'radio'}
												<div style="display:grid;gap:6px;">
													{#each q.options ?? [] as o (o)}
														<label style="display:flex;align-items:center;gap:8px;font-size:13px;cursor:pointer;font-weight:400;">
															<input type="radio" name={q.key} checked={flow.answers[q.key] === o} onchange={() => setAnswer(q.key, o)} /> {o}
														</label>
													{/each}
												</div>
											{:else if q.type === 'checkbox' || q.type === 'multi_select'}
												<div style="display:grid;gap:6px;">
													{#each q.options ?? [] as o (o)}
														<label style="display:flex;align-items:center;gap:8px;font-size:13px;cursor:pointer;font-weight:400;">
															<input
																type="checkbox"
																checked={Array.isArray(flow.answers[q.key]) && (flow.answers[q.key] as string[]).includes(o)}
																onchange={(e) => {
																	const arr = Array.isArray(flow.answers[q.key]) ? (flow.answers[q.key] as string[]) : [];
																	setAnswer(q.key, e.currentTarget.checked ? [...arr, o] : arr.filter((x) => x !== o));
																}}
															/> {o}
														</label>
													{/each}
												</div>
											{:else}
												<input class="app-input" type="text" placeholder={q.placeholder ?? ''} value={typeof flow.answers[q.key] === 'string' ? (flow.answers[q.key] as string) : ''} oninput={(e) => setAnswer(q.key, (e.currentTarget as HTMLInputElement).value)} />
											{/if}
											{#if q.helper}<span class="card-hint">{q.helper}</span>{/if}
										</label>
									{/each}
								</div>
							</div>
						{/each}
					</div>
				{/if}

				<div class="intake-article-box" style="margin-top:16px;">
					<div style="display:flex;align-items:center;justify-content:space-between;gap:8px;margin-bottom:6px;flex-wrap:wrap;">
						<h3 style="font-size:15px;font-weight:650;margin:0;color:var(--foreground);">Key messages</h3>
						<span style="font-size:11px;color:var(--muted-foreground);">Otomatis terisi via Isi Otomatis (AI) · 2 TC</span>
					</div>
					<textarea class="app-input" rows="4" bind:value={flow.keyMessagesText} placeholder={'Produk X resmi diluncurkan\nMenggunakan teknologi Y'}></textarea>
				</div>
			</div>
		{/if}

		{#if flow.draftNotice}
			<div class="notice-success">{flow.draftNotice}</div>
		{/if}

		{#if flow.intakeSaving === 'analyze'}
			<div class="banner-info">
				<TrailingDots />
				<div style="display:flex;flex-direction:column;gap:2px;">
					<span class="banner-info-title">Menganalisis kelengkapan konten…</span>
					<span class="banner-info-sub">Tauthub AI sedang memeriksa struktur naskah, topik, dan informasi penting untuk artikel Anda.</span>
				</div>
			</div>
		{/if}

		<div class="actions">
			<button class="app-btn app-btn-soft" type="button" disabled={flow.busy || !!flow.intakeSaving} onclick={() => (flow.intakeStep = 0)}>
				Kembali
			</button>
			<button
				class="app-btn app-btn-soft"
				type="button"
				disabled={flow.busy || !!flow.intakeSaving}
				onclick={() => void flow.saveAsDraft()}
			>
				{#if flow.intakeSaving === 'draft'}
					<RefreshCw size={14} class="spin-icon" />
					<span>Menyimpan draf…</span>
				{:else}
					<span>Simpan sebagai draft</span>
				{/if}
			</button>
			<button
				class="app-btn app-btn-primary"
				type="button"
				disabled={flow.busy || !!flow.intakeSaving}
				onclick={() => void flow.saveIntake(true)}
			>
				{#if flow.intakeSaving === 'analyze'}
					<RefreshCw size={14} class="spin-icon" />
					<span>Menganalisis…</span>
				{:else}
					<span>Simpan &amp; analisis kelengkapan</span>
				{/if}
			</button>
		</div>
		<p class="card-hint" style="margin-top:8px;">
			Draft tersimpan di database kampanye — buka kembali flow ini untuk edit (<code style="font-size:11px;">{flow.flowBase}/flow</code>). Tombol Lengkapi Informasi selalu bisa dibuka lagi sebelum checkout.
		</p>
	{/if}

	{#if flow.intakeStep === 3 && readiness}
		<h2 class="card-title">Analisis Kesiapan</h2>
		<div style="display:flex;justify-content:space-between;align-items:baseline;gap:12px;flex-wrap:wrap;margin-top:12px;">
			<span style="font-size:13px;color:var(--muted-foreground);">
				Skor: {readiness.score}% ·
				<strong style="color:{LEVEL_COLOR[readiness.level]};">{LEVEL_LABEL[readiness.level]}</strong>
			</span>
		</div>
		<div class="readiness-bar">
			<div class="readiness-fill" style="width:{readiness.score}%;background:{LEVEL_COLOR[readiness.level]};"></div>
		</div>
		<div style="display:grid;gap:2px;margin-top:12px;">
			{#each readiness.checks as check (check.key)}
				<div class="readiness-row">
					<span>{check.status === 'ok' ? '✅' : check.status === 'warn' ? '⚠️' : '❌'}</span>
					<span style="flex:1;">{check.label}</span>
					<span class="intake-kbd">{check.status === 'ok' ? 'lengkap' : check.status === 'warn' ? 'opsional' : 'belum diisi'}</span>
				</div>
			{/each}
		</div>

		{#if flow.advancingContent}
			<div class="banner-info">
				<TrailingDots />
				<div style="display:flex;flex-direction:column;gap:2px;">
					<span class="banner-info-title">Menyiapkan tahap Content…</span>
					<span class="banner-info-sub">Menyimpan kesiapan intake dan mengarahkan ke pembuatan artikel.</span>
				</div>
			</div>
		{/if}

		<div class="actions">
			<button class="app-btn app-btn-soft" type="button" disabled={flow.busy || !!flow.intakeSaving || flow.advancingContent} onclick={() => (flow.intakeStep = 2)}>
				← Kembali ke Lengkapi Informasi (edit draft)
			</button>
			{#if readiness.score < 100}
				<button
					class="app-btn app-btn-soft"
					type="button"
					disabled={flow.busy || !!flow.intakeSaving || flow.advancingContent}
					onclick={() => void flow.saveIntake(true)}
				>
					{#if flow.intakeSaving === 'analyze'}
						<RefreshCw size={14} class="spin-icon" />
						<span>Menganalisis…</span>
					{:else}
						<span>Analisis ulang</span>
					{/if}
				</button>
			{/if}
			<button
				class="app-btn app-btn-primary"
				type="button"
				disabled={flow.busy || !!flow.intakeSaving || flow.advancingContent}
				onclick={() => void flow.advanceAfterIntake()}
			>
				{#if flow.advancingContent}
					<RefreshCw size={14} class="spin-icon" />
					<span>Menyiapkan Content…</span>
				{:else}
					<span>Lanjut ke Content</span>
				{/if}
			</button>
		</div>
	{/if}

	{#if flow.intakeStep === 3 && !flow.readiness && flow.intake}
		<div class="actions">
			<button
				class="app-btn app-btn-primary"
				type="button"
				disabled={flow.busy || !!flow.intakeSaving}
				onclick={() => void flow.saveIntake(true)}
			>
				{#if flow.intakeSaving === 'analyze'}
					<RefreshCw size={14} class="spin-icon" />
					<span>Menganalisis kelengkapan…</span>
				{:else}
					<span>Analisis kelengkapan</span>
				{/if}
			</button>
		</div>
	{/if}
{/if}

<!-- Popup Isi Otomatis (AI) -->
{#if flow.aiFillOpen}
	<div
		class="overlay"
		role="presentation"
	>
		<dialog
			open
			class="modal-card"
			aria-label="Isi Otomatis (AI)"
			onclick={(e) => e.stopPropagation()}
			onkeydown={(e) => { if (e.key === 'Escape' && !flow.aiFillBusy) flow.aiFillOpen = false; }}
		>
			<div class="modal-head">
				<span style="display:inline-flex;align-items:center;gap:8px;"><Sparkle size={15} /> Isi Otomatis (AI)</span>
				<button type="button" class="modal-close" onclick={() => (flow.aiFillOpen = false)} disabled={flow.aiFillBusy} aria-label="Tutup">
					<X size={16} />
				</button>
			</div>
			<div style="padding:16px 18px;display:grid;gap:10px;">
				<label class="field-label">
					Topik / Judul Naskah
					<textarea
						class="app-input"
						rows="4"
						bind:value={flow.aiFillTopic}
						placeholder="Masukkan topik atau judul naskah yang ingin dibuat..."
						disabled={flow.aiFillBusy}
					></textarea>
				</label>
				<p class="card-hint" style="margin:0;">
					Contoh: Go-Digital UMKM: Digitalisasi Pemasaran dan Manajemen Keuangan untuk Pengrajin Batik di Desa Sukamaju.
				</p>
				{#if flow.aiFillBusy}
					<div style="display:flex;flex-direction:column;align-items:center;gap:8px;padding:8px 0 2px;">
						<svg width="72" height="72" viewBox="0 0 72 72">
							<circle cx="36" cy="36" r="30" fill="none" stroke="var(--border)" stroke-width="6" />
							<circle
								cx="36"
								cy="36"
								r="30"
								fill="none"
								stroke="var(--primary)"
								stroke-width="6"
								stroke-linecap="round"
								stroke-dasharray={`${2 * Math.PI * 30}`}
								stroke-dashoffset={`${2 * Math.PI * 30 * (1 - flow.aiFillProgress / 100)}`}
								transform="rotate(-90 36 36)"
								style="transition: stroke-dashoffset .2s linear;"
							/>
							<text x="36" y="41" text-anchor="middle" font-size="15" font-weight="700" fill="var(--foreground)">
								{Math.round(flow.aiFillProgress)}%
							</text>
						</svg>
						<p class="card-hint" style="margin:0;color:var(--accent-foreground);">AI sedang mengisi field yang masih kosong…</p>
					</div>
				{/if}
			</div>
			<div class="modal-foot">
				<button type="button" class="app-btn app-btn-soft" onclick={() => (flow.aiFillOpen = false)} disabled={flow.aiFillBusy}>
					Batal
				</button>
				<button
					type="button"
					class="app-btn app-btn-primary"
					disabled={flow.aiFillBusy || !flow.aiFillTopic.trim()}
					title="Biaya 2 Kredit"
					onclick={() => void flow.submitAiFill()}
				>
					<Sparkle size={14} />
					{flow.aiFillBusy ? 'Menghasilkan…' : 'Generate · 2 Kredit'}
				</button>
			</div>
		</dialog>
	</div>
{/if}
