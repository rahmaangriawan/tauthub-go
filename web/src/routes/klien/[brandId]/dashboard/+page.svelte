<script lang="ts">
	// Dashboard klien — port section `dashboard` app lama (brand-workspace.tsx
	// merender ContentIntakePanel): wizard intake 4 langkah (Mulai → Jenis &
	// topik → Lengkapi → Analisis), autosave draft lokal, readiness, rekomendasi
	// Tauthub, mode AI, lalu lanjut ke AI Writer.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import {
		ArrowRight, CheckCircle, Diamond, FileText, Gauge, Scale, Sparkles, TriangleAlert, XCircle
	} from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import Select from '$lib/components/select.svelte';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);

	type Question = {
		key: string; label: string; type: string; required: boolean; weight: number;
		helper?: string | null; placeholder?: string | null; options?: string[];
	};
	type IntakeType = { id: string; code: string; name: string; description: string | null; questions: Question[]; status: string };
	type Check = { key: string; label: string; status: 'ok' | 'warn' | 'missing'; weight: number; required: boolean };
	type Readiness = { score: number; level: 'ready' | 'needs_more' | 'insufficient'; checks: Check[]; missing: string[] };
	type Recommendation = {
		angle: string; topic_suggestion: string; structure: string[]; missing_info: string[];
		follow_up_questions: string[]; notes: string;
	};
	type Intake = {
		id: string; mode: string; content_type_code: string | null; topic: string | null;
		answers: Record<string, unknown>; article_text: string | null; article_filename: string | null;
		key_messages: string[]; readiness_score: number | null; readiness_checks: Check[];
		recommendation: Recommendation | null; status: string; job_id: string | null; created_at: string;
		updated_at?: string;
	};

	const MODES: Array<{ value: string; title: string; desc: string }> = [
		{ value: 'bahan', title: 'Saya punya bahan', desc: 'Punya data, fakta, atau info mentah — minta dibuatkan artikel.' },
		{ value: 'artikel', title: 'Saya sudah punya artikel', desc: 'Naskah sudah ada, mau direview, diedit, lalu didistribusikan.' },
		{ value: 'keduanya', title: 'Saya punya bahan + artikel', desc: 'Artikel lama + bahan terbaru untuk diperbarui/disempurnakan.' },
		{ value: 'rekomendasi', title: 'Belum punya bahan', desc: 'Minta rekomendasi arah konten dari Tauthub.' }
	];

	const LEVEL_LABEL: Record<Readiness['level'], string> = {
		ready: 'Siap diproses', needs_more: 'Perlu dilengkapi', insufficient: 'Belum cukup'
	};
	const LEVEL_COLOR: Record<Readiness['level'], string> = {
		ready: 'var(--success)', needs_more: 'var(--warning)', insufficient: 'var(--destructive)'
	};

	/** Pengelompokan VISUAL pertanyaan intake (murni layout — field & validasi tidak berubah). */
	const QUESTION_GROUPS: Array<{ id: string; title: string; hint: string; keys: string[] }> = [
		{ id: 'inti', title: 'Informasi utama', hint: 'Inti kejadian dan alasan mengapa layak diberitakan.', keys: ['who', 'what', 'why'] },
		{ id: 'waktu', title: 'Waktu & lokasi', hint: 'Kapan dan di mana hal ini terjadi.', keys: ['when', 'where'] },
		{ id: 'pihak', title: 'Pihak & entitas', hint: 'Orang, perusahaan, produk, atau acara yang terlibat.', keys: ['event_type', 'company', 'product', 'person'] },
		{ id: 'dukungan', title: 'Data & dukungan', hint: 'Angka, kutipan, atau ajakan yang memperkuat cerita.', keys: ['data', 'quote', 'cta'] }
	];

	function isWideQuestion(question: Question) {
		return ['textarea', 'radio', 'checkbox', 'multi_select'].includes(question.type);
	}

	function groupQuestions(questions: Question[]) {
		const groups = QUESTION_GROUPS.map((g) => ({ ...g, questions: [] as Question[] }));
		const others: Question[] = [];
		for (const question of questions) {
			const group = groups.find((g) => g.keys.includes(question.key));
			if (group) group.questions.push(question);
			else others.push(question);
		}
		const result = groups.filter((g) => g.questions.length > 0);
		if (others.length > 0) {
			result.push({ id: 'lainnya', title: 'Informasi lainnya', hint: 'Pertanyaan tambahan dari jenis konten ini.', keys: [], questions: others });
		}
		return result;
	}

	/* ─── Autosave draft (localStorage) ─── */
	const DRAFT_PREFIX = 'tauthub.intake.draft.';
	type Draft = {
		answers: Record<string, unknown>; article_text: string | null; article_filename: string | null;
		key_messages: string[]; topic: string; mode?: string | null; content_type_code?: string | null; updated_at: number;
	};

	function loadDraft(intakeId: string): Draft | null {
		try {
			const raw = window.localStorage.getItem(DRAFT_PREFIX + intakeId);
			return raw ? (JSON.parse(raw) as Draft) : null;
		} catch {
			return null;
		}
	}
	function saveDraft(intakeId: string, draft: Draft) {
		try {
			window.localStorage.setItem(DRAFT_PREFIX + intakeId, JSON.stringify(draft));
		} catch {
			/* non-fatal */
		}
	}
	function clearDraft(intakeId: string) {
		try {
			window.localStorage.removeItem(DRAFT_PREFIX + intakeId);
		} catch {
			/* non-fatal */
		}
	}
	function listDrafts(): Array<{ intake_id: string; draft: Draft }> {
		try {
			const entries: Array<{ intake_id: string; draft: Draft }> = [];
			for (let i = 0; i < window.localStorage.length; i += 1) {
				const key = window.localStorage.key(i);
				if (!key || !key.startsWith(DRAFT_PREFIX)) continue;
				const intakeId = key.slice(DRAFT_PREFIX.length);
				const draft = loadDraft(intakeId);
				if (draft) entries.push({ intake_id: intakeId, draft });
			}
			return entries.sort((a, b) => b.draft.updated_at - a.draft.updated_at);
		} catch {
			return [];
		}
	}

	/* ─── State ─── */
	let types = $state<IntakeType[]>([]);
	let recent = $state<Intake[]>([]);
	let mode = $state<string | null>(null);
	let typeCode = $state('');
	let topic = $state('');
	let answers = $state<Record<string, unknown>>({});
	let articleText = $state('');
	let articleFilename = $state('');
	let keyMessagesText = $state('');
	let intake = $state<Intake | null>(null);
	let readiness = $state<Readiness | null>(null);
	let recommendation = $state<Recommendation | null>(null);
	let step = $state(0);
	let busy = $state(false);
	let genMode = $state<'economy' | 'premium'>('premium');
	let error = $state('');
	let draftSavedAt = $state<number | null>(null);
	let localDrafts = $state<Array<{ intake_id: string; draft: Draft }>>([]);
	let fileInput = $state<HTMLInputElement | null>(null);
	let loading = $state(true);

	const stepperLabels = ['Mulai', 'Jenis & topik', 'Lengkapi', 'Analisis'];
	const currentType = $derived(types.find((t) => t.code === typeCode) ?? null);
	const showArticle = $derived(mode === 'artikel' || mode === 'keduanya');
	const showRecommendation = $derived(mode === 'rekomendasi' || (readiness !== null && readiness.score < 80));
	const questionGroups = $derived(currentType ? groupQuestions(currentType.questions ?? []) : []);

	const GEN_MODES: Array<{ value: 'economy' | 'premium'; title: string; desc: string; icon: typeof Gauge }> = [
		{ value: 'economy', title: 'Mode Hemat', desc: 'Cepat dan hemat. Cocok untuk draf awal & artikel rutin.', icon: Gauge },
		{ value: 'premium', title: 'Mode Premium', desc: 'Kualitas terbaik, penalaran mendalam. Untuk konten penting & analitis.', icon: Diamond }
	];

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await Promise.all([loadTypes(), loadRecent()]);
			localDrafts = listDrafts();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	// Autosave jawaban form (debounce 600ms) — hanya saat ada konten.
	$effect(() => {
		if (!intake || step !== 2 || busy) return;
		const hasContent =
			topic.trim() !== '' ||
			articleText.trim() !== '' ||
			keyMessagesText.trim() !== '' ||
			Object.keys(answers).some((key) => String(answers[key] ?? '').trim() !== '');
		if (!hasContent) return;
		const timer = window.setTimeout(() => {
			saveDraft(intake!.id, {
				answers,
				article_text: articleText.trim() || null,
				article_filename: articleFilename || null,
				key_messages: keyMessagesText.split('\n').map((s) => s.trim()).filter(Boolean),
				topic: topic.trim(),
				mode,
				content_type_code: typeCode,
				updated_at: Date.now()
			});
			draftSavedAt = Date.now();
			localDrafts = listDrafts();
		}, 600);
		return () => window.clearTimeout(timer);
	});

	async function loadTypes() {
		const res = await apiRequest(`/v1/brands/${brandId}/content-intake-types`);
		if (res.ok) types = await res.json();
	}

	async function loadRecent() {
		const res = await apiRequest(`/v1/brands/${brandId}/content-intakes`);
		if (res.ok) recent = await res.json();
	}

	function applyDraft(draft: Draft | null) {
		if (!draft) return;
		if (draft.answers && Object.keys(draft.answers).length > 0) answers = draft.answers;
		articleText = draft.article_text ?? '';
		articleFilename = draft.article_filename ?? '';
		keyMessagesText = (draft.key_messages ?? []).join('\n');
		if (draft.topic) topic = draft.topic;
		draftSavedAt = draft.updated_at ?? null;
	}

	function resetAfterMode() {
		intake = null;
		answers = {};
		readiness = null;
		recommendation = null;
		articleText = '';
		articleFilename = '';
		keyMessagesText = '';
	}

	async function nextToForm() {
		error = '';
		if (!mode || !typeCode) {
			error = 'Pilih cara Anda ingin mulai dan jenis kontennya dulu.';
			return;
		}
		busy = true;
		try {
			let id: string;
			if (!intake) {
				const res = await apiRequest(`/v1/brands/${brandId}/content-intakes`, {
					method: 'POST',
					body: JSON.stringify({ mode, content_type_code: typeCode, topic: topic.trim() || null })
				});
				if (!res.ok) throw new Error(await readApiError(res, 'Gagal membuat intake.'));
				const created = (await res.json()) as Intake;
				id = created.id;
				intake = created;
			} else {
				const res = await apiRequest(`/v1/brands/${brandId}/content-intakes/${intake.id}`, {
					method: 'PUT',
					body: JSON.stringify({ content_type_code: typeCode, topic: topic.trim() || null })
				});
				if (!res.ok) throw new Error(await readApiError(res, 'Gagal menyimpan intake.'));
				const saved = (await res.json()) as Intake;
				id = saved.id;
				intake = saved;
			}
			const draft = loadDraft(id);
			if (draft && draft.topic && draft.topic !== topic.trim()) clearDraft(id);
			else applyDraft(draft);
			step = 2;
			void loadRecent();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Gagal menyimpan intake.';
		} finally {
			busy = false;
		}
	}

	async function saveAndAnalyze(goResult = true) {
		error = '';
		if (!intake) return;
		busy = true;
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/content-intakes/${intake.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					content_type_code: typeCode,
					topic: topic.trim() || null,
					answers,
					article_text: articleText.trim() || null,
					article_filename: articleFilename || null,
					key_messages: keyMessagesText.split('\n').map((s) => s.trim()).filter(Boolean)
				})
			});
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal menyimpan & menganalisis.'));
			const payload = (await res.json()) as Intake & { readiness: Readiness };
			intake = payload;
			readiness = payload.readiness ?? null;
			recommendation =
				payload.recommendation && Object.keys(payload.recommendation).length > 0 ? payload.recommendation : null;
			clearDraft(payload.id);
			draftSavedAt = null;
			localDrafts = listDrafts();
			if (goResult) step = 3;
			void loadRecent();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Gagal menyimpan & menganalisis.';
		} finally {
			busy = false;
		}
	}

	async function requestRecommendation() {
		error = '';
		if (!intake) return;
		busy = true;
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/content-intakes/${intake.id}/recommend`, {
				method: 'POST',
				body: '{}'
			});
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal membuat rekomendasi.'));
			const payload = (await res.json()) as { recommendation: Recommendation; readiness: Readiness };
			recommendation = payload.recommendation;
			readiness = payload.readiness ?? null;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Gagal membuat rekomendasi.';
		} finally {
			busy = false;
		}
	}

	function useRecommendation() {
		if (!recommendation) return;
		if (recommendation.topic_suggestion && recommendation.topic_suggestion !== topic) {
			topic = recommendation.topic_suggestion;
		}
		if (recommendation.angle) answers = { ...answers, angle: recommendation.angle };
		error = '';
	}

	async function createJob() {
		error = '';
		if (!intake) return;
		busy = true;
		try {
			const topicValue = topic.trim() || null;
			const articleValue = articleText.trim() || null;
			if (!topicValue && !articleValue) {
				error = 'Isi topik atau bahan dulu sebelum generate.';
				busy = false;
				return;
			}
			await apiRequest(`/v1/brands/${brandId}/content-intakes/${intake.id}`, {
				method: 'PUT',
				body: JSON.stringify({
					topic: topicValue,
					answers,
					article_text: articleValue,
					article_filename: articleFilename || null,
					key_messages: keyMessagesText.split('\n').map((s) => s.trim()).filter(Boolean)
				})
			});
			const res = await apiRequest(`/v1/brands/${brandId}/content-intakes/${intake.id}/create-job`, {
				method: 'POST',
				body: JSON.stringify({ model_tier: genMode })
			});
			if (!res.ok) throw new Error(await readApiError(res, 'Gagal membuat artikel dari intake.'));
			const payload = (await res.json()) as { job_id: string; status?: string };
			window.location.href = `/klien/${brandUrl}/generate?outline=${payload.job_id}`;
		} catch (e) {
			error = e instanceof Error ? e.message : 'Gagal membuat artikel.';
		} finally {
			busy = false;
		}
	}

	async function readArticleFile(file: File) {
		if (file.size > 2 * 1024 * 1024) {
			error = 'Ukuran file maksimal 2 MB.';
			return;
		}
		try {
			const text = await file.text();
			articleText = articleText ? `${articleText}\n\n${text}` : text;
			articleFilename = file.name;
		} catch {
			error = 'File tidak bisa dibaca sebagai teks.';
		}
	}

	function resumeDraft(intakeId: string, draft: Draft) {
		error = '';
		intake = {
			id: intakeId,
			mode: draft.mode ?? mode ?? '',
			content_type_code: draft.content_type_code ?? typeCode ?? null,
			topic: draft.topic || null,
			answers: draft.answers ?? {},
			article_text: draft.article_text,
			article_filename: draft.article_filename,
			key_messages: draft.key_messages ?? [],
			readiness_score: null,
			readiness_checks: [],
			recommendation: null,
			status: 'draft',
			job_id: null,
			created_at: new Date().toISOString()
		};
		if (draft.mode) mode = draft.mode;
		if (draft.content_type_code) typeCode = draft.content_type_code;
		topic = draft.topic ?? '';
		answers = draft.answers ?? {};
		articleText = draft.article_text ?? '';
		articleFilename = draft.article_filename ?? '';
		keyMessagesText = (draft.key_messages ?? []).join('\n');
		readiness = null;
		recommendation = null;
		draftSavedAt = draft.updated_at ?? null;
		step = 2;
	}

	async function resume(target: Intake) {
		error = '';
		const res = await apiRequest(`/v1/brands/${brandId}/content-intakes/${target.id}`);
		if (!res.ok) {
			error = await readApiError(res, 'Gagal memuat intake.');
			return;
		}
		const data = (await res.json()) as Intake;
		intake = data;
		mode = data.mode;
		typeCode = data.content_type_code ?? '';
		topic = data.topic ?? '';
		answers = data.answers ?? {};
		articleText = data.article_text ?? '';
		articleFilename = data.article_filename ?? '';
		keyMessagesText = (data.key_messages ?? []).join('\n');
		recommendation =
			data.recommendation && Object.keys(data.recommendation).length > 0 ? data.recommendation : null;
		readiness =
			data.readiness_score !== null
				? { score: data.readiness_score, level: 'needs_more', checks: data.readiness_checks ?? [], missing: [] }
				: null;
		applyDraft(loadDraft(target.id));
		step = 2;
	}

	function setAnswer(key: string, value: unknown) {
		answers = { ...answers, [key]: value };
	}

	const fmtDate = (v: string | null | undefined) =>
		v ? new Date(v).toLocaleString('id-ID', { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' }) : '—';
</script>

<section>
	<header>
		<p class="text-[11px] font-semibold tracking-wide text-muted-foreground">CONTENT INTAKE</p>
		<h1 class="mt-1 text-2xl font-semibold tracking-tight">Buat konten baru</h1>
		<p class="mt-1 text-sm text-muted-foreground">
			Ceritakan dulu apa yang Anda miliki — bahan, artikel, atau belum ada apa-apa. Tauthub menyesuaikan
			pertanyaan, menilai kelengkapan, lalu mengantar Anda ke AI Writer.
		</p>
	</header>

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">Memuat…</p>
	{:else}
		<div class="app-card mt-4 p-5">
			<div class="flex flex-wrap gap-2">
				{#each stepperLabels as label, index (label)}
					<span
						class="inline-flex items-center gap-1 rounded-full px-3 py-1 text-xs font-medium"
						style={index === step
							? 'background-color: var(--primary); color: var(--primary-foreground);'
							: index < step
								? 'background-color: var(--success-soft); color: var(--success);'
								: 'background-color: var(--muted); color: var(--muted-foreground);'}
					>
						{#if index < step}<CheckCircle size={12} />{/if}
						{index + 1}. {label}
					</span>
				{/each}
			</div>

			{#if error}
				<div class="mt-3.5 rounded-lg border px-3.5 py-2.5 text-sm" style="border-color: color-mix(in srgb, var(--destructive) 30%, transparent); background-color: color-mix(in srgb, var(--destructive) 8%, transparent); color: var(--destructive);">
					{error}
				</div>
			{/if}

			<!-- Step 0 — Mulai -->
			{#if step === 0}
				<div class="mt-4 grid gap-3 sm:grid-cols-2">
					{#each MODES as item (item.value)}
						<button
							type="button"
							class="rounded-xl border p-4 text-left transition-colors hover:bg-secondary"
							style={mode === item.value ? 'border-color: var(--primary); background-color: var(--accent);' : ''}
							onclick={() => { mode = item.value; resetAfterMode(); }}
						>
							<span class="block text-sm font-semibold">{item.title}</span>
							<span class="mt-1 block text-xs text-muted-foreground">{item.desc}</span>
						</button>
					{/each}
				</div>
				<div class="mt-4 flex flex-wrap gap-2">
					<button
						class="app-btn app-btn-primary"
						type="button"
						disabled={busy}
						onclick={() => {
							if (!mode) { error = 'Pilih salah satu cara memulai dulu.'; return; }
							step = 1;
						}}
					>Lanjut <ArrowRight size={16} /></button>
					<a class="app-btn app-btn-soft" href={`/klien/${brandUrl}/generate`}>Langsung isi brief (tanpa intake)</a>
				</div>
			{/if}

			<!-- Step 1 — Jenis & topik -->
			{#if step === 1}
				<div class="mt-4 grid gap-3 sm:grid-cols-2">
					<label class="block text-sm">
						<span class="text-xs font-medium text-muted-foreground">Jenis konten</span>
						<div class="mt-1">
							<Select
								bind:value={typeCode}
								ariaLabel="Jenis konten"
								placeholder="Pilih jenis konten…"
								minWidth={200}
								options={[
									{ value: '', label: 'Pilih jenis konten…' },
									...types.map((t) => ({ value: t.code, label: t.name }))
								]}
							/>
						</div>
						{#if currentType?.description}<span class="mt-1 block text-xs text-muted-foreground">{currentType.description}</span>{/if}
					</label>
					<label class="block text-sm">
						<span class="text-xs font-medium text-muted-foreground">Topik / judul kerja</span>
						<input class="app-input mt-1 w-full" bind:value={topic} placeholder="Contoh: Peluncuran layanan baru PT Nusantara" />
					</label>
				</div>
				<div class="mt-4 flex flex-wrap gap-2">
					<button class="app-btn app-btn-soft" type="button" onclick={() => (step = 0)}>Kembali</button>
					<button class="app-btn app-btn-primary" type="button" disabled={!typeCode || busy} onclick={() => void nextToForm()}>
						{busy ? 'Menyimpan…' : 'Lanjut ke pertanyaan'} <ArrowRight size={16} />
					</button>
				</div>
			{/if}

			<!-- Step 2 — Lengkapi bahan -->
			{#if step === 2 && intake}
				<div class="mt-4 grid gap-3 sm:grid-cols-2">
					<label class="block text-sm">
						<span class="text-xs font-medium text-muted-foreground">Jenis konten</span>
						<div class="mt-1">
							<Select
								bind:value={typeCode}
								ariaLabel="Jenis konten"
								minWidth={200}
								options={types.map((t) => ({ value: t.code, label: t.name }))}
							/>
						</div>
					</label>
					<label class="block text-sm">
						<span class="text-xs font-medium text-muted-foreground">Topik / judul kerja</span>
						<input class="app-input mt-1 w-full" bind:value={topic} placeholder="Topik artikel" />
					</label>
				</div>

				{#if questionGroups.length > 0}
					<div class="mt-5">
						<h3 class="text-sm font-semibold">{mode === 'rekomendasi' ? 'Isi sebanyak yang Anda tahu' : 'Bahan (informasi mentah)'}</h3>
						<p class="mt-1 text-xs text-muted-foreground">
							Jawaban ini dipakai sebagai dasar penulisan artikel. Field bertanda * wajib untuk siap diproses.
						</p>
						{#each questionGroups as group (group.id)}
							<div class="mt-4">
								<h4 class="text-xs font-bold tracking-wide text-muted-foreground">{group.title}</h4>
								{#if group.hint}<p class="mt-0.5 text-[11px] text-muted-foreground">{group.hint}</p>{/if}
								<div class="mt-2 grid gap-3 sm:grid-cols-2">
									{#each group.questions as question (question.key)}
										<label class="block text-sm {isWideQuestion(question) ? 'sm:col-span-2' : ''}">
											<span class="text-xs font-medium text-muted-foreground">
												{question.label} {#if question.required}<span style="color: var(--destructive);">*</span>{/if}
											</span>
											{#if question.type === 'textarea'}
												<textarea class="app-input mt-1 w-full" rows={3} placeholder={question.placeholder ?? ''} value={String(answers[question.key] ?? '')} oninput={(e) => setAnswer(question.key, (e.currentTarget as HTMLTextAreaElement).value)}></textarea>
											{:else if question.type === 'number'}
												<input class="app-input mt-1 w-full" type="number" placeholder={question.placeholder ?? ''} value={String(answers[question.key] ?? '')} oninput={(e) => setAnswer(question.key, (e.currentTarget as HTMLInputElement).value)} />
											{:else if question.type === 'date'}
												<input class="app-input mt-1 w-full" type="date" value={String(answers[question.key] ?? '')} oninput={(e) => setAnswer(question.key, (e.currentTarget as HTMLInputElement).value)} />
											{:else if question.type === 'url'}
												<input class="app-input mt-1 w-full" type="url" placeholder="https://…" value={String(answers[question.key] ?? '')} oninput={(e) => setAnswer(question.key, (e.currentTarget as HTMLInputElement).value)} />
											{:else if question.type === 'select'}
												<span class="mt-1 block">
													<Select
														value={String(answers[question.key] ?? '')}
														ariaLabel={question.label}
														placeholder="Pilih…"
														minWidth={200}
														onchange={(v) => setAnswer(question.key, v)}
														options={[
															{ value: '', label: 'Pilih…' },
															...(question.options ?? []).map((option) => ({ value: option, label: option }))
														]}
													/>
												</span>
											{:else if question.type === 'radio'}
												<span class="mt-1 grid gap-1.5">
													{#each question.options ?? [] as option (option)}
														<label class="flex items-center gap-2 text-xs">
															<input type="radio" name={question.key} checked={String(answers[question.key] ?? '') === option} onchange={() => setAnswer(question.key, option)} /> {option}
														</label>
													{/each}
												</span>
											{:else if question.type === 'checkbox' || question.type === 'multi_select'}
												{@const arr = Array.isArray(answers[question.key]) ? (answers[question.key] as string[]) : []}
												<span class="mt-1 grid gap-1.5">
													{#each question.options ?? [] as option (option)}
														<label class="flex items-center gap-2 text-xs">
															<input
																type="checkbox"
																checked={arr.includes(option)}
																onchange={(e) => setAnswer(question.key, (e.currentTarget as HTMLInputElement).checked ? [...arr, option] : arr.filter((o) => o !== option))}
															/> {option}
														</label>
													{/each}
												</span>
											{:else}
												<input class="app-input mt-1 w-full" type="text" placeholder={question.placeholder ?? ''} value={String(answers[question.key] ?? '')} oninput={(e) => setAnswer(question.key, (e.currentTarget as HTMLInputElement).value)} />
											{/if}
											{#if question.helper}<span class="mt-1 block text-xs text-muted-foreground">{question.helper}</span>{/if}
										</label>
									{/each}
								</div>
							</div>
						{/each}
					</div>
				{/if}

				{#if mode !== 'rekomendasi'}
					<div class="mt-4 rounded-xl border p-4">
						<h3 class="text-sm font-semibold">Key messages (pesan utama)</h3>
						<p class="mt-1 text-xs text-muted-foreground">
							Satu pesan per baris. Pesan ini diverifikasi pada artikel yang terbit untuk mengukur
							message consistency (PR Calculator) — contoh: &quot;Produk X resmi diluncurkan untuk UMKM&quot;.
						</p>
						<textarea class="app-input mt-2 w-full" rows={4} bind:value={keyMessagesText} placeholder={"Produk X resmi diluncurkan\nMenggunakan teknologi Y\nDitujukan untuk UMKM"}></textarea>
					</div>
				{/if}

				{#if showArticle}
					<div class="mt-4 rounded-xl border p-4">
						<h3 class="text-sm font-semibold">Artikel existing</h3>
						<p class="mt-1 text-xs text-muted-foreground">
							Tempel naskah Anda atau unggah file (teks/.md/.txt paling akurat). Artikel asli tidak diubah —
							Tauthub memprosesnya sebagai bahan.
						</p>
						<textarea class="app-input mt-2 w-full" rows={8} bind:value={articleText} placeholder="Tempel artikel di sini…"></textarea>
						<div class="mt-2 flex flex-wrap items-center gap-3">
							<button class="app-btn app-btn-soft !min-h-8 !text-xs" type="button" onclick={() => fileInput?.click()}><FileText size={15} /> Unggah dokumen</button>
							{#if articleFilename}<span class="text-[11px] text-muted-foreground">File: {articleFilename} · {articleText.length.toLocaleString('id-ID')} karakter</span>{/if}
							<input
								bind:this={fileInput}
								type="file"
								accept=".txt,.md,.markdown,.doc,.docx,.pdf"
								class="hidden"
								onchange={(e) => { const f = (e.currentTarget as HTMLInputElement).files?.[0]; if (f) void readArticleFile(f); (e.currentTarget as HTMLInputElement).value = ''; }}
							/>
						</div>
					</div>
				{/if}

				{#if draftSavedAt}
					<p class="mt-3 flex items-center gap-1.5 text-xs text-muted-foreground">
						<CheckCircle size={13} style="color: var(--success);" />
						Draft tersimpan otomatis · {new Date(draftSavedAt).toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })}
					</p>
				{/if}

				<div class="mt-4 flex flex-wrap gap-2">
					<button class="app-btn app-btn-soft" type="button" onclick={() => (step = 1)}>Kembali</button>
					<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void saveAndAnalyze()}>
						{busy ? 'Menganalisis…' : 'Simpan & analisis kelengkapan'}
					</button>
				</div>
			{/if}

			<!-- Step 3 — Analisis -->
			{#if step === 3 && intake}
				{#if !readiness}
					<div class="mt-4">
						<h3 class="text-sm font-semibold">Analisis kelengkapan</h3>
						<p class="mt-1.5 text-xs text-muted-foreground">
							Klik &ldquo;Simpan &amp; analisis kelengkapan&rdquo; untuk menilai kesiapan bahan Anda.
						</p>
						<div class="mt-3 flex flex-wrap gap-2">
							<button class="app-btn app-btn-soft" type="button" onclick={() => (step = 2)}>Edit bahan</button>
							<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void saveAndAnalyze()}>
								{busy ? 'Menganalisis…' : 'Simpan & analisis kelengkapan'}
							</button>
						</div>
					</div>
				{/if}

				{#if readiness}
					<div class="mt-4">
						<div class="flex flex-wrap items-baseline justify-between gap-2">
							<h3 class="text-sm font-semibold">Content Readiness</h3>
							<span class="text-xs text-muted-foreground">
								{readiness.score}% · <strong style="color: {LEVEL_COLOR[readiness.level]};">{LEVEL_LABEL[readiness.level]}</strong>
							</span>
						</div>
						<div class="mt-2 h-2 overflow-hidden rounded-full" style="background-color: var(--muted);">
							<div class="h-full rounded-full" style="width: {readiness.score}%; background-color: {LEVEL_COLOR[readiness.level]};"></div>
						</div>
						<div class="mt-3 grid gap-1">
							{#each readiness.checks as check (check.key)}
								<div class="flex items-center gap-2 text-sm">
									{#if check.status === 'ok'}
										<CheckCircle size={16} style="color: var(--success);" />
									{:else if check.status === 'warn'}
										<TriangleAlert size={16} style="color: var(--warning);" />
									{:else}
										<XCircle size={16} style="color: var(--destructive);" />
									{/if}
									<span class="flex-1">{check.label}</span>
									<span class="rounded px-1.5 py-0.5 text-[11px] text-muted-foreground" style="background-color: var(--muted);">
										{check.status === 'ok' ? 'lengkap' : check.status === 'warn' ? 'opsional' : 'belum diisi'}
									</span>
								</div>
							{/each}
						</div>
					</div>
				{/if}

				{#if showRecommendation}
					<div class="mt-4">
						{#if !recommendation}
							<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void requestRecommendation()}>
								<Sparkles size={16} /> {busy ? 'Menyusun rekomendasi…' : 'Minta Rekomendasi Tauthub'}
							</button>
						{:else}
							<div class="rounded-xl border p-4" style="border-color: color-mix(in srgb, var(--primary) 30%, transparent); background-color: var(--accent);">
								<div class="flex items-center gap-2">
									<Sparkles size={18} style="color: var(--accent-foreground);" />
									<strong class="text-sm">Rekomendasi Tauthub</strong>
								</div>
								<p class="mt-2 text-xs text-muted-foreground">{recommendation.notes}</p>
								{#if recommendation.angle}
									<div class="mt-2.5">
										<strong class="text-xs">Sudut pemberitaan</strong>
										<p class="mt-1 text-xs leading-relaxed text-muted-foreground">{recommendation.angle}</p>
									</div>
								{/if}
								{#if recommendation.topic_suggestion}
									<div class="mt-2.5">
										<strong class="text-xs">Topik kerja</strong>
										<p class="mt-1 text-xs text-muted-foreground">{recommendation.topic_suggestion}</p>
									</div>
								{/if}
								{#if (recommendation.structure?.length ?? 0) > 0}
									<div class="mt-2.5">
										<strong class="text-xs">Struktur konten</strong>
										<ul class="mt-1 list-disc pl-4 text-xs text-muted-foreground">
											{#each recommendation.structure ?? [] as s (s)}<li>{s}</li>{/each}
										</ul>
									</div>
								{/if}
								{#if (recommendation.missing_info?.length ?? 0) > 0}
									<div class="mt-2.5">
										<strong class="text-xs" style="color: var(--warning);">Informasi yang perlu disiapkan</strong>
										<ul class="mt-1 list-disc pl-4 text-xs text-muted-foreground">
											{#each recommendation.missing_info ?? [] as s (s)}<li>{s}</li>{/each}
										</ul>
									</div>
								{/if}
								<div class="mt-3.5 flex flex-wrap gap-2">
									<button class="app-btn app-btn-primary" type="button" onclick={useRecommendation}>Gunakan rekomendasi</button>
									<button class="app-btn app-btn-soft" type="button" onclick={() => (step = 2)}>Edit</button>
									<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => void requestRecommendation()}>Coba rekomendasi lain</button>
								</div>
							</div>
						{/if}
					</div>
				{/if}

				<div class="mt-4">
					<p class="text-xs font-semibold text-muted-foreground">
						Mode AI <span class="font-normal">— pilih kualitas &amp; biaya</span>
					</p>
					<div class="mt-2 grid gap-2 sm:grid-cols-2">
						{#each GEN_MODES as gm (gm.value)}
							{@const active = genMode === gm.value}
							<button
								type="button"
								disabled={busy}
								title={gm.desc}
								class="flex flex-col items-start gap-0.5 rounded-lg px-3 py-2.5 text-left text-xs"
								style={active ? 'border: 1.5px solid var(--primary); background-color: var(--accent);' : 'border: 1px solid var(--border);'}
								onclick={() => (genMode = gm.value)}
							>
								<span class="flex items-center gap-1.5 text-xs font-semibold"><gm.icon size={16} /> {gm.title}</span>
								<span class="text-[11px] leading-snug text-muted-foreground">{gm.desc.slice(0, 90)}…</span>
							</button>
						{/each}
					</div>
				</div>

				<div class="mt-4 flex flex-wrap gap-2">
					<button class="app-btn app-btn-soft" type="button" onclick={() => (step = 2)}>Edit bahan</button>
					<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void createJob()}>
						<Sparkles size={16} /> {busy ? 'Menyiapkan…' : 'Lanjut ke AI Writer'}
					</button>
				</div>
			{/if}
		</div>

		<!-- Draft tersimpan (lokal) -->
		{#if localDrafts.length > 0}
			<section class="app-card mt-4 p-4">
				<div class="flex items-center justify-between">
					<h2 class="text-sm font-semibold">Draft tersimpan</h2>
					<span class="rounded-full px-2 py-0.5 text-xs" style="background-color: var(--muted);">{localDrafts.length}</span>
				</div>
				<div class="mt-2 divide-y">
					{#each localDrafts.slice(0, 6) as item (item.intake_id)}
						<div class="flex items-center justify-between gap-3 py-2.5">
							<div class="min-w-0">
								<p class="truncate text-sm font-medium">
									{item.draft.topic || (types.find((t) => t.code === item.draft.content_type_code)?.name ?? 'Draft tanpa topik')}
								</p>
								<p class="text-xs text-muted-foreground">
									{MODES.find((m) => m.value === item.draft.mode)?.title ?? item.draft.mode ?? 'Intake'} ·
									{types.find((t) => t.code === item.draft.content_type_code)?.name ?? '-'} ·
									<span class="rounded px-1.5 py-0.5" style="background-color: var(--muted);">draft</span> ·
									Tersimpan {fmtDate(new Date(item.draft.updated_at).toISOString())}
								</p>
							</div>
							<div class="flex shrink-0 items-center gap-1.5">
								<button class="app-btn app-btn-soft !min-h-7 !text-xs" type="button" onclick={() => resumeDraft(item.intake_id, item.draft)}>Lanjutkan</button>
								<button
									class="text-xs underline"
									type="button"
									style="color: var(--destructive);"
									onclick={() => { clearDraft(item.intake_id); localDrafts = listDrafts(); if (intake?.id === item.intake_id) draftSavedAt = null; }}
								>Hapus</button>
							</div>
						</div>
					{/each}
				</div>
			</section>
		{/if}

		<!-- Intake terakhir (server) -->
		{#if recent.length > 0}
			<section class="app-card mt-4 p-4">
				<div class="flex items-center justify-between">
					<h2 class="text-sm font-semibold">Intake terakhir</h2>
					<span class="rounded-full px-2 py-0.5 text-xs" style="background-color: var(--muted);">{recent.length}</span>
				</div>
				<div class="mt-2 divide-y">
					{#each recent.slice(0, 6) as item (item.id)}
						<div class="flex items-center justify-between gap-3 py-2.5">
							<div class="min-w-0">
								<p class="truncate text-sm font-medium">
									{item.topic || (types.find((t) => t.code === item.content_type_code)?.name ?? 'Intake')}
								</p>
								<p class="text-xs text-muted-foreground">
									{MODES.find((m) => m.value === item.mode)?.title ?? item.mode} ·
									{types.find((t) => t.code === item.content_type_code)?.name ?? '—'} ·
									<span class="rounded px-1.5 py-0.5" style="background-color: var(--muted);">{item.status}</span>
									{#if item.readiness_score !== null}<span class="ml-2">Readiness {item.readiness_score}%</span>{/if}
								</p>
							</div>
							<div class="flex shrink-0 items-center gap-1.5">
								{#if item.job_id}
									<a class="app-btn app-btn-soft !min-h-7 !text-xs" href={`/klien/${brandUrl}/generate?outline=${item.job_id}`}>Lihat artikel</a>
								{/if}
								{#if item.status !== 'job_created'}
									<button class="app-btn app-btn-soft !min-h-7 !text-xs" type="button" onclick={() => void resume(item)}>Lanjutkan</button>
								{/if}
							</div>
						</div>
					{/each}
				</div>
			</section>
		{/if}
	{/if}
</section>
