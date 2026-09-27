<script lang="ts">
	// Form The Profiler — 4 seksi: konteks bisnis, backlink, target media &
	// bahan baku, kendali anggaran. Port profiler/sections.tsx.
	import { Calculator, ChartLine, ChevronDown, Coins, Info, Link2, Search, Sparkles, Trash, Upload, Users } from '@lucide/svelte';
	import { apiRequest, getStoredSession } from '$lib/auth';
	import { fmtBytes, fmtIdr } from '../../_shared';
	import type { ProfilerDraft, DynamicAngleField } from '../../_shared';
	import {
		ANGLE_DYNAMIC_FIELDS,
		DEFAULT_AUDIENCE_OPTIONS,
		OBJECTIVES,
		OBJECTIVE_DESCRIPTIONS,
		OTHER_AUDIENCE
	} from '../../_shared';

	type ChannelOption = {
		id: string;
		name: string;
		slug: string;
		description: string;
		angles: Array<{ id: string; name: string; slug: string; description: string }>;
	};
	type CampaignFile = {
		id: string;
		original_filename: string;
		mime_type: string | null;
		size_bytes: number | null;
		parse_status: 'pending' | 'parsing' | 'ready' | 'failed';
		parse_error?: string | null;
	};
	type DynamicQuestion = { key: string; label: string; suggested_answer: string };

	let {
		draft = $bindable(),
		apiBase,
		creditsHref,
		options,
		walletAvailable,
		crawling,
		dynamicQuestions = {},
		onPatch,
		onCrawl
	}: {
		draft: ProfilerDraft;
		apiBase: string;
		creditsHref: string;
		options: ChannelOption[];
		walletAvailable: number;
		crawling: boolean;
		dynamicQuestions?: Record<string, DynamicQuestion[]>;
		onPatch: (patch: Partial<ProfilerDraft>) => void;
		onCrawl: (url: string) => void;
	} = $props();

	const ACCEPT_DOCS = '.pdf,.docx,.txt,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document,text/plain';
	const MAX_UPLOAD_BYTES = 10 * 1024 * 1024;
	const PARSE_LABEL: Record<string, { text: string; tone: string }> = {
		ready: { text: 'Siap dipakai AI', tone: 'success' },
		parsing: { text: 'Memproses isi dokumen…', tone: 'warning' },
		pending: { text: 'Menunggu proses', tone: 'warning' },
		failed: { text: 'Gagal dibaca', tone: 'destructive' }
	};

	let crawlUrl = $state(draft.business_profile?.website ?? '');
	let guardrailsExpanded = $state(false);
	let audienceSearch = $state('');
	let audienceOpen = $state(false);

	// Upload zone state
	let files = $state<CampaignFile[]>([]);
	let dragging = $state(false);
	let uploadingFile = $state<{ name: string; progress: number } | null>(null);
	let uploadSuccess = $state<string | null>(null);
	let uploadError = $state('');
	let fileInput: HTMLInputElement | null = $state(null);

	$effect(() => {
		void loadFiles();
	});

	async function loadFiles() {
		try {
			const res = await apiRequest(`${apiBase}/files`);
			if (!res.ok) return;
			const data = await res.json();
			files = data.files ?? [];
		} catch {
			/* daftar file opsional saat load */
		}
	}

	function parseLabel(status: string | undefined) {
		if (status && PARSE_LABEL[status]) return PARSE_LABEL[status];
		return PARSE_LABEL.pending;
	}

	const toneStyle = (tone: string) =>
		tone === 'success'
			? 'color: var(--success);'
			: tone === 'warning'
				? 'color: var(--warning);'
				: tone === 'destructive'
					? 'color: var(--destructive);'
					: 'color: var(--muted-foreground);';

	async function upload(file: File) {
		uploadError = '';
		if (file.size > MAX_UPLOAD_BYTES) {
			uploadError = `${file.name} melebihi 10 MB.`;
			return;
		}
		const body = new FormData();
		body.append('file', file);
		// XHR karena fetch tidak melaporkan progres unggah; token ditambah manual.
		const token = getStoredSession()?.accessToken ?? null;
		const xhr = new XMLHttpRequest();
		uploadingFile = { name: file.name, progress: 0 };
		try {
			const uploaded = await new Promise<CampaignFile>((resolve, reject) => {
				xhr.open('POST', `${apiBase}/files`);
				xhr.withCredentials = true;
				if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`);
				xhr.upload.onprogress = (e) => {
					if (e.lengthComputable) {
						const p = Math.round((e.loaded / e.total) * 100);
						uploadingFile = uploadingFile ? { ...uploadingFile, progress: p } : uploadingFile;
					}
				};
				xhr.onerror = () => reject(new Error('Koneksi terputus saat mengunggah.'));
				xhr.onload = () => {
					let parsed: { file?: CampaignFile; detail?: string } = {};
					try {
						parsed = JSON.parse(xhr.responseText);
					} catch {
						/* detail default */
					}
					if (xhr.status >= 200 && xhr.status < 300 && parsed.file) resolve(parsed.file);
					else reject(new Error(parsed.detail ?? 'Dokumen gagal diunggah.'));
				};
				xhr.send(body);
			});
			files = [uploaded, ...files.filter((f) => f.id !== uploaded.id)];
			if (uploaded.parse_status === 'ready' && !(draft.materials?.file_ids ?? []).includes(uploaded.id)) {
				patchMaterials({ file_ids: [...(draft.materials?.file_ids ?? []), uploaded.id] });
			}
			uploadingFile = null;
			uploadSuccess = uploaded.original_filename;
			window.setTimeout(() => (uploadSuccess = null), 2500);
		} catch (reason) {
			uploadError = reason instanceof Error ? reason.message : 'Dokumen gagal diunggah.';
			uploadingFile = null;
		}
	}

	async function pick(list: FileList | null) {
		for (const file of Array.from(list ?? [])) await upload(file);
		if (fileInput) fileInput.value = '';
	}

	async function removeFile(id: string) {
		patchMaterials({ file_ids: (draft.materials?.file_ids ?? []).filter((x) => x !== id) });
		files = files.filter((f) => f.id !== id);
		try {
			await apiRequest(`${apiBase}/files/${id}`, { method: 'DELETE' });
		} catch {
			/* baris sudah hilang dari UI */
		}
	}

	const objectiveLabel = $derived(OBJECTIVES.find((o) => o.value === draft.objective)?.label ?? '');
	const audienceOptions = $derived.by(() => {
		const q = audienceSearch.trim().toLowerCase();
		const base: string[] = q
			? DEFAULT_AUDIENCE_OPTIONS.filter((a) => a.toLowerCase().includes(q))
			: [...DEFAULT_AUDIENCE_OPTIONS];
		if (!base.includes(OTHER_AUDIENCE)) base.push(OTHER_AUDIENCE);
		return base;
	});

	function patchBusiness(p: Partial<ProfilerDraft['business_profile']>) {
		onPatch({ business_profile: { ...draft.business_profile, ...p } });
	}
	function patchAudience(p: Partial<ProfilerDraft['audience']>) {
		onPatch({ audience: { ...draft.audience, ...p } });
	}
	function patchSpokepersons(list: ProfilerDraft['spokepersons']) {
		onPatch({ spokepersons: list });
	}
	function patchBacklink(p: Partial<ProfilerDraft['backlink']>) {
		onPatch({ backlink: { ...draft.backlink, ...p } });
	}
	function patchBudget(p: Partial<ProfilerDraft['budget']>) {
		onPatch({ budget: { ...draft.budget, ...p } });
	}
	function patchMaterials(p: Partial<ProfilerDraft['materials']>) {
		onPatch({ materials: { ...draft.materials, ...p } });
	}

	function toggleChannel(slug: string, checked: boolean) {
		const cur = draft.channels ?? [];
		const exists = cur.some((c) => c.channel === slug);
		if (checked) {
			if (exists || cur.length >= 2) return;
			onPatch({ channels: [...cur, { channel: slug as 'mainstream' | 'blogger', angle_slug: '' }] });
		} else {
			onPatch({ channels: cur.filter((c) => c.channel !== slug) });
		}
	}
	function setAngle(channel: string, angleSlug: string) {
		onPatch({ channels: (draft.channels ?? []).map((c) => (c.channel === channel ? { ...c, angle_slug: angleSlug } : c)) });
	}
	function toggleAudience(option: string) {
		const cur = draft.audience?.selected ?? [];
		patchAudience({ selected: cur.includes(option) ? cur.filter((x) => x !== option) : [...cur, option] });
	}

	const efficientInsufficient = $derived(
		draft.budget?.mode === 'efficient' && draft.budget?.amount > 0 && draft.budget.amount > walletAvailable
	);

	// Angka dynamic answers pembantu tipe.
	function dynamicAnswersFor(angleSlug: string): Record<string, string> {
		const d = (draft.materials?.dynamic ?? {}) as Record<string, unknown>;
		return (d[angleSlug] as Record<string, string> | undefined) ?? {};
	}

	function setDynamicAnswer(angleSlug: string, key: string, value: string) {
		const d = { ...((draft.materials?.dynamic ?? {}) as Record<string, unknown>) };
		d[angleSlug] = { ...((d[angleSlug] as Record<string, string>) ?? {}), [key]: value };
		patchMaterials({ dynamic: d });
	}

	/** Jawaban generik (tanpa angle terpilih) → key langsung di `dynamic`. */
	function setRootDynamicAnswer(key: string, value: string) {
		const d = { ...((draft.materials?.dynamic ?? {}) as Record<string, unknown>) };
		d[key] = value;
		patchMaterials({ dynamic: d });
	}

	const activeAngleGroups = $derived(
		(draft.channels ?? []).filter(
			(c) => c.angle_slug && ((dynamicQuestions?.[c.angle_slug]?.length ?? 0) > 0 || ANGLE_DYNAMIC_FIELDS[c.angle_slug])
		)
	);
</script>

<div class="space-y-4">
	<!-- ── 01 Konteks Bisnis & Tujuan Kampanye ── -->
	<section class="app-card p-5">
		<header class="flex items-start gap-3">
			<span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-bold" style="background-color: var(--accent); color: var(--accent-foreground);">01</span>
			<div>
				<h2 class="font-semibold">Konteks Bisnis &amp; Tujuan Kampanye</h2>
				<p class="text-sm text-muted-foreground">Profil bisnis, tujuan, target audiens, dan guardrails kampanye Anda.</p>
			</div>
		</header>

		<div class="mt-4 space-y-4">
			<div class="rounded-xl border p-4" style="background-color: var(--accent);">
				<div class="flex items-start justify-between gap-2">
					<span class="flex items-center gap-2 font-medium">
						<Sparkles size={15} style="color: var(--primary);" /> Auto-Fill Profil Bisnis
					</span>
					<span class="rounded-full px-2 py-0.5 text-[10px] font-semibold" style="background-color: var(--primary); color: var(--primary-foreground);">AI ASSISTED</span>
				</div>
				<p class="mt-1 text-sm text-muted-foreground">Masukkan URL website klien. AI akan mengekstrak data industri dan tone-of-voice secara otomatis.</p>
				<div class="mt-3 flex gap-2">
					<label class="app-input flex flex-1 items-center gap-2 !py-1.5">
						<Link2 size={15} class="text-muted-foreground" />
						<input class="w-full bg-transparent text-sm outline-none" placeholder="https://website-klien.com" bind:value={crawlUrl} />
					</label>
					<button class="app-btn app-btn-primary" type="button" disabled={crawling} onclick={() => onCrawl(crawlUrl)}>
						{crawling ? 'Menarik Data...' : 'Tarik Data'}
					</button>
				</div>
			</div>

			<div>
				<div class="flex items-center gap-1">
					<span class="text-sm font-medium">Tujuan Utama Kampanye *</span>
					<span title="99% media yang terlisting di Tauthub.com menerima penempatan backlink. Pemilihan tujuan kampanye menegaskan prioritas output konten yang dihasilkan AI.">
						<Info size={14} class="text-muted-foreground" />
					</span>
				</div>
				<div class="mt-2 grid gap-2 md:grid-cols-2">
					{#each OBJECTIVES as o (o.value)}
						{@const sel = draft.objective === o.value}
						<button
							type="button"
							class="flex items-start gap-3 rounded-xl border p-3 text-left"
							style="border-color: {sel ? 'var(--primary)' : 'var(--border)'}; background-color: {sel ? 'var(--accent)' : 'transparent'};"
							role="radio"
							aria-checked={sel}
							onclick={() => onPatch({ objective: o.value })}
						>
							<span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg" style="background-color: {sel ? 'var(--primary)' : 'var(--muted)'}; color: {sel ? '#fff' : 'var(--muted-foreground)'};">
								<Coins size={17} />
							</span>
							<span>
								<span class="block text-sm font-medium">{o.label}</span>
								<span class="mt-0.5 block text-xs text-muted-foreground">{OBJECTIVE_DESCRIPTIONS[o.value]}</span>
							</span>
						</button>
					{/each}
				</div>
				{#if !draft.objective}<p class="mt-1 text-xs" style="color: var(--warning);">Pilih satu tujuan utama kampanye.</p>{/if}
			</div>

			<div>
				<span class="text-sm font-medium">Target Audiens Spesifik</span>
				<p class="text-xs text-muted-foreground">Pilih target utama yang paling relevan dengan kampanye.</p>
				<div class="mt-1 flex flex-wrap items-center gap-3">
					<button class="app-btn app-btn-soft" type="button" onclick={() => (audienceOpen = !audienceOpen)}>
						<Users size={14} />
						{draft.audience?.selected?.length ? `${draft.audience.selected.length} audiens dipilih` : 'Pilih audiens…'}
						<ChevronDown size={12} />
					</button>
					{#if objectiveLabel}<span class="text-xs text-muted-foreground">Berdasarkan: {objectiveLabel}</span>{/if}
				</div>
				{#if audienceOpen}
					<div class="mt-2 max-w-md rounded-xl border p-2">
						<label class="app-input flex items-center gap-2 !py-1">
							<Search size={14} class="text-muted-foreground" />
							<input class="w-full bg-transparent text-sm outline-none" placeholder="Cari audiens…" bind:value={audienceSearch} />
						</label>
						<div class="mt-2 max-h-64 overflow-y-auto">
							{#each audienceOptions as opt (opt)}
								<label class="flex cursor-pointer items-center gap-2 rounded px-2 py-1 hover:bg-secondary">
									<input type="checkbox" checked={(draft.audience?.selected ?? []).includes(opt)} onchange={() => toggleAudience(opt)} />
									<span class="text-sm">{opt}</span>
								</label>
							{/each}
						</div>
					</div>
				{/if}
				{#if draft.audience?.selected?.length}
					<div class="mt-2 flex flex-wrap gap-1">
						{#each draft.audience.selected as a (a)}
							<span class="rounded-full px-2 py-0.5 text-xs" style="background-color: var(--accent); color: var(--accent-foreground);">{a}</span>
						{/each}
					</div>
				{/if}
				{#if (draft.audience?.selected ?? []).includes(OTHER_AUDIENCE)}
					<label class="mt-2 block">
						<span class="text-xs font-medium">Target Audiens Lainnya</span>
						<input class="app-input mt-1 w-full" value={draft.audience?.custom ?? ''} placeholder="Contoh: Pemilik startup teknologi tahap awal" oninput={(e) => patchAudience({ custom: (e.currentTarget as HTMLInputElement).value })} />
					</label>
				{/if}
			</div>

			<div>
				<div class="flex items-center justify-between">
					<span class="flex items-center gap-2 text-sm font-medium">
						<Info size={15} style="color: var(--warning);" /> Guardrails (Do's &amp; Don'ts Merek)
					</span>
					<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" aria-expanded={guardrailsExpanded} onclick={() => (guardrailsExpanded = !guardrailsExpanded)}>
						{guardrailsExpanded ? 'Collapse Guardrails' : 'Expand Guardrails'}
						<ChevronDown size={12} style="transform: {guardrailsExpanded ? 'rotate(180deg)' : 'none'};" />
					</button>
				</div>
				<textarea
					class="app-input mt-1 w-full"
					rows={guardrailsExpanded ? 12 : 3}
					placeholder={'Dilarang sebut kata "Murah", ganti dengan "Terjangkau".\nJangan sebut kompetitor X.'}
					value={(draft.guardrails?.rules ?? []).join('\n')}
					oninput={(e) => onPatch({ guardrails: { rules: (e.currentTarget as HTMLTextAreaElement).value.split('\n') } })}
				></textarea>
				<p class="mt-1 text-xs text-muted-foreground">Aturan keras untuk pipeline AI: kata yang dilarang, kompetitor yang tidak boleh disebut, klaim yang tidak boleh dibuat.</p>
			</div>
		</div>
	</section>

	<!-- ── 02 Penanaman Backlink & SEO ── -->
	<section class="app-card p-5">
		<header class="flex items-start gap-3">
			<span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-bold" style="background-color: var(--accent); color: var(--accent-foreground);">02</span>
			<div>
				<h2 class="font-semibold">Penanaman Backlink &amp; SEO</h2>
				<p class="text-sm text-muted-foreground">Opsional — aktifkan bila kampanye membutuhkan penanaman hyperlink.</p>
			</div>
		</header>
		<div class="mt-4 space-y-3">
			<label class="flex items-start gap-3">
				<input type="checkbox" checked={draft.backlink?.enabled ?? false} onchange={(e) => patchBacklink({ enabled: (e.currentTarget as HTMLInputElement).checked })} />
				<span>
					<span class="block text-sm font-medium">Aktifkan Penanaman Backlink (Hyperlink)</span>
					<span class="block text-xs text-muted-foreground">Biarkan tidak dicentang jika kampanye murni untuk Brand Awareness/Mention. Lebih aman dari penalty Google untuk rilis serentak.</span>
				</span>
			</label>
			{#if draft.backlink?.enabled}
				<div class="rounded-xl border p-4" style="background-color: var(--accent);">
					<p class="text-sm text-muted-foreground">Data ini diperlukan sejak awal agar Agent Matchmaker hanya mencarikan media yang memperbolehkan tanam link, dan Agent Writer bisa menyusun narasi organik menuju link tersebut.</p>
					<div class="mt-3 grid gap-4 md:grid-cols-2">
						<label class="block">
							<span class="text-xs font-medium">URL Target Utama</span>
							<input class="app-input mt-1 w-full" value={draft.backlink.target_url ?? ''} placeholder="https://..." oninput={(e) => patchBacklink({ target_url: (e.currentTarget as HTMLInputElement).value })} />
						</label>
						<label class="block">
							<span class="text-xs font-medium">Anchor Text Utama</span>
							<input class="app-input mt-1 w-full" value={draft.backlink.anchor_text ?? ''} placeholder="Contoh: sewa server NVMe murah" oninput={(e) => patchBacklink({ anchor_text: (e.currentTarget as HTMLInputElement).value })} />
						</label>
					</div>
					<label class="mt-3 flex items-start gap-3">
						<input type="checkbox" checked={draft.backlink.smart_variations ?? true} onchange={(e) => patchBacklink({ smart_variations: (e.currentTarget as HTMLInputElement).checked })} />
						<span>
							<span class="block text-sm font-medium">Gunakan AI Smart Variations</span>
							<span class="block text-xs text-muted-foreground">Otomatis rotasi LSI &amp; Naked URL agar 100% natural.</span>
						</span>
					</label>
				</div>
			{/if}
		</div>
	</section>

	<!-- ── 03 Target Media & Bahan Baku ── -->
	<section class="app-card p-5">
		<header class="flex items-start gap-3">
			<span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-bold" style="background-color: var(--accent); color: var(--accent-foreground);">03</span>
			<div>
				<h2 class="font-semibold">Target Media &amp; Bahan Baku</h2>
				<p class="text-sm text-muted-foreground">Pilih jalur distribusi (maksimal dua channel) dan bahan referensi kampanye.</p>
			</div>
		</header>
		<div class="mt-4 space-y-3">
			<p class="text-sm font-medium">Jalur Distribusi (Multi-Kanal)</p>
			<div class="space-y-3">
				{#each options as ch (ch.slug)}
					{@const selected = (draft.channels ?? []).some((c) => c.channel === ch.slug)}
					{@const selectedAngle = (draft.channels ?? []).find((c) => c.channel === ch.slug)?.angle_slug ?? ''}
					{@const maxed = (draft.channels ?? []).length >= 2}
					<div class="rounded-xl border p-4" style="border-color: {selected ? 'var(--primary)' : 'var(--border)'};">
						<label class="flex items-start gap-3">
							<input type="checkbox" checked={selected} disabled={!selected && maxed} onchange={(e) => toggleChannel(ch.slug, (e.currentTarget as HTMLInputElement).checked)} />
							<span>
								<span class="block text-sm font-medium">{ch.name}</span>
								<span class="block text-xs text-muted-foreground">{ch.description}</span>
							</span>
						</label>
						{#if selected}
							<div class="mt-3 space-y-3">
								<label class="block">
									<span class="text-xs font-medium">Angle Artikel {ch.slug === 'mainstream' ? 'Jurnalistik' : 'Blogger'}</span>
									<select class="app-input mt-1 w-full" value={selectedAngle} onchange={(e) => setAngle(ch.slug, (e.currentTarget as HTMLSelectElement).value)}>
										<option value="">Pilih angle…</option>
										{#each ch.angles as a (a.slug)}<option value={a.slug}>{a.name}</option>{/each}
									</select>
								</label>
								{#if ch.slug === 'mainstream'}
									<div class="rounded-lg border p-3">
										<span class="text-sm font-medium">Juru Bicara / Narasumber *</span>
										<p class="text-xs text-muted-foreground">Wajib untuk media Mainstream — isi lengkap: nama, jabatan, dan kutipan. Kutipan disuntikkan AI ke artikel sebagai narasumber.</p>
										{#if (draft.spokepersons ?? []).length === 0}
											<p class="mt-1 text-xs text-muted-foreground">Belum ada narasumber — klik tombol + Tambah Narasumber.</p>
										{/if}
										{#each draft.spokepersons ?? [] as sp, i (i)}
											<div class="mt-2 grid grid-cols-2 gap-2 rounded-lg border p-3">
												<div class="col-span-2 flex items-center justify-between">
													<span class="text-xs font-bold text-muted-foreground">Narasumber {i + 1}</span>
													<button class="text-xs" style="color: var(--destructive);" type="button" onclick={() => patchSpokepersons((draft.spokepersons ?? []).filter((_, xi) => xi !== i))}>Hapus</button>
												</div>
												<input class="app-input w-full" placeholder="Nama — contoh: Budi Santoso" value={sp.name ?? ''} oninput={(e) => patchSpokepersons((draft.spokepersons ?? []).map((x, xi) => (xi === i ? { ...x, name: (e.currentTarget as HTMLInputElement).value } : x)))} />
												<input class="app-input w-full" placeholder="Jabatan — contoh: Head of Product" value={sp.role ?? ''} oninput={(e) => patchSpokepersons((draft.spokepersons ?? []).map((x, xi) => (xi === i ? { ...x, role: (e.currentTarget as HTMLInputElement).value } : x)))} />
												<input class="app-input col-span-2 w-full" placeholder="Kutipan — contoh: Produk ini memangkas biaya hingga 40%" value={sp.quote ?? ''} oninput={(e) => patchSpokepersons((draft.spokepersons ?? []).map((x, xi) => (xi === i ? { ...x, quote: (e.currentTarget as HTMLInputElement).value } : x)))} />
											</div>
										{/each}
										<div class="mt-3 flex flex-wrap items-center gap-2">
											<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={() => patchSpokepersons([...(draft.spokepersons ?? []), { name: '', role: '', quote: '' }])}>+ Tambah Narasumber</button>
											{#if (draft.spokepersons ?? []).length > 0}
												<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" style="color: var(--destructive);" onclick={() => patchSpokepersons([])}>Hapus semua</button>
											{/if}
										</div>
										<p class="mt-2 text-xs italic" style="color: var(--warning);">Isi manual — AI tidak memiliki data narasumber yang akurat dari situs.</p>
									</div>
								{/if}
							</div>
						{/if}
					</div>
				{/each}
			</div>

			<hr />

			<div class="flex items-center justify-between">
				<span class="text-sm font-medium">Bahan Referensi Kampanye</span>
				<select class="app-input !w-auto" value={draft.materials?.mode ?? 'dynamic'} onchange={(e) => patchMaterials({ mode: (e.currentTarget as HTMLSelectElement).value as 'upload' | 'dynamic' })}>
					<option value="upload">Saya Punya Dokumen (Upload)</option>
					<option value="dynamic">Saya Tidak Punya Bahan</option>
				</select>
			</div>

			{#if draft.materials?.mode === 'upload'}
				<div>
					<div
						class="flex cursor-pointer flex-col items-center justify-center gap-1 rounded-xl border-2 border-dashed p-6 text-center"
						style="border-color: {dragging ? 'var(--primary)' : 'var(--border)'}; background-color: {dragging ? 'var(--accent)' : 'transparent'};"
						role="button"
						tabindex="0"
						onclick={() => fileInput?.click()}
						onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') fileInput?.click(); }}
						ondragover={(e) => { e.preventDefault(); dragging = true; }}
						ondragleave={() => (dragging = false)}
						ondrop={(e) => {
							e.preventDefault();
							dragging = false;
							void pick((e as DragEvent).dataTransfer?.files ?? null);
						}}
					>
						{#if uploadingFile}
							<Upload size={22} style="color: var(--primary);" />
							<p class="text-sm font-medium">Mengunggah dokumen...</p>
							<p class="text-xs text-muted-foreground">{uploadingFile.name}</p>
							<div class="mt-2 h-2 w-64 overflow-hidden rounded-full" style="background-color: var(--muted);">
								<div class="h-full" style="width: {uploadingFile.progress}%; background-color: var(--primary);"></div>
							</div>
							<p class="text-xs text-muted-foreground">{uploadingFile.progress}%</p>
						{:else if uploadSuccess}
							<p class="text-sm font-medium">Upload berhasil</p>
							<p class="text-xs text-muted-foreground">{uploadSuccess}</p>
							<p class="text-xs text-muted-foreground">Dokumen siap diproses oleh AI.</p>
						{:else}
							<Upload size={24} class="text-muted-foreground" />
							<p class="text-base font-bold">Upload Press Release / Brosur / Materi</p>
							<p class="text-xs text-muted-foreground">PDF, DOCX, TXT · AI akan mengekstrak fakta dari dokumen ini.</p>
							<span class="app-btn app-btn-soft mt-2">Pilih dokumen atau tarik ke sini</span>
						{/if}
					</div>
					<input
						bind:this={fileInput}
						type="file"
						multiple
						accept={ACCEPT_DOCS}
						class="hidden"
						onchange={(e) => void pick((e.currentTarget as HTMLInputElement).files)}
					/>
					{#if uploadError}<p class="mt-2 text-xs" style="color: var(--destructive);">{uploadError}</p>{/if}
					<div class="mt-2 space-y-1">
						{#each files as f (f.id)}
							{@const label = parseLabel(f.parse_status)}
							<div class="flex items-center justify-between gap-2 rounded-lg border px-3 py-2">
								<div class="min-w-0">
									<p class="truncate text-xs font-medium">{f.original_filename}</p>
									<p class="text-xs" style={toneStyle(label.tone)}>{fmtBytes(f.size_bytes)} · {label.text}{f.parse_error ? ` — ${f.parse_error}` : ''}</p>
								</div>
								<button class="rounded p-1 hover:bg-secondary" type="button" aria-label={`Hapus ${f.original_filename}`} onclick={() => void removeFile(f.id)}><Trash size={14} /></button>
							</div>
						{/each}
						{#if files.length === 0}<p class="text-xs text-muted-foreground">Belum ada dokumen diunggah.</p>{/if}
					</div>
				</div>
			{:else}
				<div class="rounded-xl border p-4">
					<div class="flex items-center gap-2">
						<Sparkles size={16} style="color: var(--success);" />
						<p class="font-bold">Dynamic PR Template</p>
					</div>
					<p class="mt-1 text-xs text-muted-foreground">Jawab singkat saja (1 kalimat). Pertanyaan menyesuaikan angle yang Anda pilih di Jalur Distribusi; AI akan meramunya dengan Konteks Bisnis URL Anda menjadi artikel berbobot.</p>
					{#if activeAngleGroups.length > 0}
						{#each activeAngleGroups as c (c.channel + '-' + c.angle_slug)}
							{@const dq = dynamicQuestions?.[c.angle_slug] ?? []}
							{@const fields: DynamicAngleField[] = dq.length ? dq.map((x) => ({ key: x.key, label: x.label, placeholder: '' })) : (ANGLE_DYNAMIC_FIELDS[c.angle_slug] ?? [])}
							{@const isDynamic = dq.length > 0}
							{@const angleName = options.flatMap((o) => o.angles).find((a) => a.slug === c.angle_slug)?.name ?? c.angle_slug}
							{@const answers = dynamicAnswersFor(c.angle_slug)}
							<div class="mt-3 rounded-lg border p-3">
								<div class="flex flex-wrap items-center gap-2">
									<span class="rounded-full px-2 py-0.5 text-[11px]" style="background-color: var(--accent); color: var(--accent-foreground);">{c.channel === 'mainstream' ? 'Mainstream' : 'Blogger'}</span>
									<span class="text-sm font-bold">{angleName}</span>
									{#if isDynamic}<span class="rounded-full px-2 py-0.5 text-[10px] font-semibold" style="background-color: var(--success-soft); color: var(--success);">Disesuaikan dari website Anda</span>{/if}
								</div>
								{#each fields as f, i (f.key)}
									<label class="mt-2 block">
										<span class="text-xs font-medium">{i + 1}. {f.label}</span>
										<input class="app-input mt-1 w-full" value={answers[f.key] ?? ''} placeholder={isDynamic ? 'Jawab singkat (1 kalimat)...' : (ANGLE_DYNAMIC_FIELDS[c.angle_slug]?.find((x) => x.key === f.key)?.placeholder ?? '')} oninput={(e) => setDynamicAnswer(c.angle_slug, f.key, (e.currentTarget as HTMLInputElement).value)} />
									</label>
								{/each}
							</div>
						{/each}
					{:else}
						<p class="mt-2 text-sm text-muted-foreground">Belum ada angle terpilih — pertanyaan khusus muncul setelah Anda memilih angle di Jalur Distribusi. Sementara isi dulu pertanyaan umum berikut.</p>
						{#each [{ key: 'product_function', label: 'Apa nama produk/program dan fungsi utamanya?', ph: 'Contoh: Fitur Auto-Backup, untuk mengamankan data server...' }, { key: 'problem_solved', label: 'Apa masalah terbesar yang diselesaikan?', ph: 'Contoh: Mencegah kerugian finansial akibat server down...' }, { key: 'offer_or_launch', label: 'Ada penawaran khusus atau target rilis?', ph: 'Contoh: Diskon 50% untuk pelanggan baru bulan ini...' }] as q, i (q.key)}
							<label class="mt-2 block">
								<span class="text-xs font-medium">{i + 1}. {q.label}</span>
								<input class="app-input mt-1 w-full" placeholder={q.ph} value={String(((draft.materials?.dynamic ?? {}) as Record<string, unknown>)[q.key] ?? '')} oninput={(e) => setRootDynamicAnswer(q.key, (e.currentTarget as HTMLInputElement).value)} />
							</label>
						{/each}
					{/if}
				</div>
			{/if}
		</div>
	</section>

	<!-- ── 04 Kendali Anggaran ── -->
	<section class="app-card p-5">
		<header class="flex items-start gap-3">
			<span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-xs font-bold" style="background-color: var(--accent); color: var(--accent-foreground);">04</span>
			<div>
				<h2 class="font-semibold">Kendali Anggaran</h2>
				<p class="text-sm text-muted-foreground">Pilih mode anggaran dan nominal Kredit Saldo kampanye.</p>
			</div>
		</header>
		<div class="mt-4 grid gap-3 md:grid-cols-2">
			{#each [{ value: 'efficient', title: 'Mode Efisien (Kalkulator)', desc: 'AI akan mencari kombinasi media (DA sedang) yang total harganya tidak melebihi batas anggaran Anda. Fokus ROI.' }, { value: 'optimal', title: 'Mode Optimal (Strategis)', desc: 'AI bertindak sebagai Planner. Ia meracik daftar media tier atas yang paling berdampak untuk objektif Anda. AI akan merekomendasikan skala terbaik.' }] as mode (mode.value)}
				{@const sel = draft.budget?.mode === mode.value}
				{@const insufficient = mode.value === 'efficient' && draft.budget?.mode === 'efficient' && efficientInsufficient}
				<div class="rounded-xl border p-3" style="border-color: {sel ? 'var(--primary)' : 'var(--border)'}; background-color: {sel ? 'var(--accent)' : 'transparent'};">
					<div class="flex items-start gap-3">
						<span class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg" style="background-color: {sel ? 'var(--primary)' : 'var(--muted)'}; color: {sel ? '#fff' : 'var(--muted-foreground)'};">
							{#if mode.value === 'efficient'}<Calculator size={17} />{:else}<ChartLine size={17} />{/if}
						</span>
						<div class="flex-1">
							<button class="block w-full text-left" type="button" onclick={() => patchBudget({ mode: mode.value as 'efficient' | 'optimal' })}>
								<span class="block text-sm font-medium">{mode.title}</span>
								<span class="mt-0.5 block text-xs text-muted-foreground">{mode.desc}</span>
							</button>
							{#if mode.value === 'efficient'}
								<p class="mt-2 inline-flex items-center gap-1 text-xs" style="color: var(--success);">
									<Coins size={13} /> Saldo: {fmtIdr(walletAvailable)} Kredit
								</p>
							{/if}
							{#if mode.value === 'efficient' && sel}
								<label class="mt-2 block">
									<span class="text-xs text-muted-foreground">Nominal (5.000 – 100.000 Kredit)</span>
									<input class="app-input mt-1 w-full" type="number" min="5000" max="100000" placeholder="5.000 – 100.000 Kredit" value={draft.budget?.amount || ''} oninput={(e) => patchBudget({ amount: Math.min(100000, Number((e.currentTarget as HTMLInputElement).value) || 0) })} />
								</label>
							{/if}
							{#if mode.value === 'efficient' && insufficient}
								<div class="mt-2 rounded-lg border p-2">
									<p class="text-xs" style="color: var(--destructive);">Kredit Saldo tidak mencukupi untuk menjalankan kampanye ini.</p>
									<a class="app-btn app-btn-soft mt-2 !min-h-7 !px-2 !text-xs" href={creditsHref}>Top Up Kredit Saldo</a>
								</div>
							{/if}
							{#if mode.value === 'optimal' && sel}
								<p class="mt-2 text-xs text-muted-foreground">AI akan merekomendasikan skala terbaik.</p>
							{/if}
						</div>
					</div>
				</div>
			{/each}
		</div>
	</section>
</div>
