<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Executive PR Brief view — hasil review kerja The Profiler, bukan form.
	// Port profiler/summary-render.tsx. Satu-satunya action adalah "Lanjut ke
	// Matchmaker"; dokumen bersifat locked.
	import { ArrowRight, Check, CircleCheck, FileText, Globe, Link, List, ListChecks, Lock, Pencil, ShieldCheck, Sparkles, Target, TriangleAlert, User, Users, X } from '@lucide/svelte';

	type ExecutiveBrief = {
		campaign_name: string;
		objective: { type: string; label: string; priority?: string };
		business_summary?: string;
		audience?: { selected?: string[]; custom?: string; note?: string };
		spokesperson?: { name?: string; role?: string } | null;
		guardrails?: { active?: boolean; summary?: string };
		channels?: Array<{ channel: string; angle: string; writing_instruction?: string }>;
		seo?: { enabled?: boolean; target_url?: string; anchor_text?: string; smart_variations?: boolean };
		strategic_notes?: string[];
		sources?: string[];
		needs_verification?: string[];
		verification_state?: Record<string, { status: 'answered' | 'ignored'; response?: string }>;
	};

	let {
		brief,
		onAdvance,
		onBack,
		onVerify,
		advancing = false
	}: {
		brief: ExecutiveBrief;
		onAdvance: () => void;
		onBack: () => void;
		onVerify: (index: number, action: 'answered' | 'ignored', response?: string) => Promise<void>;
		advancing?: boolean;
	} = $props();

	const OBJECTIVE_LABELS: Record<string, string> = {
		brand_awareness: 'Brand Awareness & Edukasi',
		seo_dominance: 'Dominasi SEO & Backlink',
		conversion: 'Konversi & Promosi Langsung',
		reputation_management: 'Manajemen Reputasi / Klarifikasi'
	};
	const CHANNEL_LABELS: Record<string, string> = { mainstream: 'Mainstream', blogger: 'Blogger' };

	/** Ubah slug mentah ("brand_awareness") jadi teks layak baca. */
	function humanize(value: string): string {
		return value.replace(/_/g, ' ').trim().replace(/\b\w/g, (c) => c.toUpperCase());
	}

	// State editor inline per item verifikasi (index → {open, draft, busy}).
	let verifyOpen = $state<Record<number, boolean>>({});
	let verifyDraft = $state<Record<number, string>>({});
	let verifyBusy = $state<Record<number, boolean>>({});

	async function submitVerify(index: number, action: 'answered' | 'ignored') {
		if (verifyBusy[index]) return;
		verifyBusy = { ...verifyBusy, [index]: true };
		try {
			await onVerify(index, action, action === 'answered' ? verifyDraft[index] : undefined);
			verifyOpen = { ...verifyOpen, [index]: false };
			verifyDraft = { ...verifyDraft, [index]: '' };
		} finally {
			verifyBusy = { ...verifyBusy, [index]: false };
		}
	}

	const objectiveLabel = $derived(
		OBJECTIVE_LABELS[brief.objective?.type ?? ''] ?? brief.objective?.label ?? humanize(brief.objective?.type ?? '') ?? '—'
	);
	const hasSeo = $derived(Boolean(brief.seo?.enabled));
	const sources = $derived(brief.sources ?? []);
	const spokesperson = $derived(
		brief.spokesperson?.name
			? `${brief.spokesperson.name}${brief.spokesperson.role ? `, ${brief.spokesperson.role}` : ''}`
			: tr('Belum terkonfirmasi — Perwakilan brand')
	);
	const audience = $derived(
		[...(brief.audience?.selected ?? []), ...(brief.audience?.custom ? [brief.audience.custom] : [])].map((a) => humanize(a)).join(', ') || '—'
	);
	const guardrails = $derived(
		brief.guardrails?.active ? `Aktif — ${brief.guardrails.summary || 'ada pantangan khusus.'}` : tr('Tidak ada pantangan khusus.')
	);
</script>

<div class="space-y-5">
	<!-- 1. Success header -->
	<section class="app-card flex flex-wrap items-center gap-4 p-5">
		<CircleCheck size={32} style="color: var(--success);" />
		<div class="min-w-[220px] flex-1">
			<p class="text-lg font-bold">{tr("Executive PR Brief Selesai")}</p>
			<p class="text-sm text-muted-foreground">{tr("Dokumen master telah dikunci dan siap diteruskan ke Agent Matchmaker.")}</p>
		</div>
		<div class="flex flex-col items-end gap-2">
			<button class="app-btn app-btn-primary" type="button" disabled={advancing} onclick={onAdvance}>
				Lanjut ke Matchmaker <ArrowRight size={16} />
			</button>
			<button class="text-xs underline" type="button" onclick={onBack}>Edit ulang isian profiler</button>
		</div>
	</section>

	<div class="grid gap-4 lg:grid-cols-3">
		<div class="space-y-4 lg:col-span-2">
			<!-- Kompilasi Strategi Bisnis -->
			<section class="app-card p-5">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">Kompilasi Strategi Bisnis</p>
					<span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium" style="background-color: var(--success-soft); color: var(--success);">
						<ShieldCheck size={12} /> Terverifikasi
					</span>
				</div>
				<p class="mt-2 text-lg font-bold">{brief.campaign_name}</p>
				<div class="mt-4 space-y-3 text-sm">
					<div class="flex items-start gap-3"><Target size={16} style="color: var(--success);" class="mt-0.5 shrink-0" /><div><p class="text-xs text-muted-foreground">Tujuan</p><p class="font-medium">{objectiveLabel}</p></div></div>
					<div class="flex items-start gap-3"><User size={16} style="color: var(--success);" class="mt-0.5 shrink-0" /><div><p class="text-xs text-muted-foreground">Juru Bicara</p><p class="font-medium">{spokesperson}</p></div></div>
					<div class="flex items-start gap-3"><Users size={16} style="color: var(--success);" class="mt-0.5 shrink-0" /><div><p class="text-xs text-muted-foreground">Target Audiens</p><p class="font-medium">{audience}</p></div></div>
					<div class="flex items-start gap-3"><ShieldCheck size={16} style="color: var(--success);" class="mt-0.5 shrink-0" /><div><p class="text-xs text-muted-foreground">Guardrails</p><p class="font-medium">{guardrails}</p></div></div>
				</div>
				{#if brief.business_summary}
					<hr class="my-4" />
					<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">Gambaran</p>
					<p class="mt-2 text-sm text-muted-foreground">{brief.business_summary}</p>
				{/if}
			</section>

			<!-- Instruksi Anatomi Penulisan -->
			<section class="app-card p-5">
				<p class="flex items-center gap-2 text-xs font-bold uppercase tracking-wide text-muted-foreground">
					<Lock size={14} /> Instruksi Anatomi Penulisan (Terkunci)
				</p>
				<div class="mt-3 space-y-3">
					{#each brief.channels ?? [] as c (c.channel + '-' + c.angle)}
						<div class="rounded-lg border p-3">
							<div class="flex flex-wrap items-center gap-2">
								<span class="rounded-full px-2 py-0.5 text-[11px]" style="background-color: {c.channel === 'mainstream' ? 'var(--accent)' : 'var(--success-soft)'}; color: {c.channel === 'mainstream' ? 'var(--accent-foreground)' : 'var(--success)'};">
									{CHANNEL_LABELS[c.channel] ?? c.channel}
								</span>
								<span class="font-medium">{humanize(c.angle)}</span>
							</div>
							<div class="mt-2">
								<p class="text-xs text-muted-foreground">Instruksi AI</p>
								<p class="text-sm text-muted-foreground">{c.writing_instruction || tr('Instruksi tidak tersedia untuk angle ini.')}</p>
							</div>
						</div>
					{/each}
				</div>
			</section>

			<!-- Catatan Strategis & Verifikasi -->
			{#if (brief.strategic_notes?.length ?? 0) || (brief.needs_verification?.length ?? 0)}
				<section class="app-card p-5">
					<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">Catatan Strategis & Verifikasi</p>
					{#if brief.strategic_notes?.length}
						<div class="mt-3 space-y-2">
							{#each brief.strategic_notes as n (n)}
								<div class="flex items-start gap-2">
									<ListChecks size={14} style="color: var(--success);" class="mt-0.5 shrink-0" />
									<p class="text-sm text-muted-foreground">{n}</p>
								</div>
							{/each}
						</div>
					{/if}
					{#if brief.needs_verification?.length}
						<hr class="my-4" />
						<p class="flex items-center gap-2 text-sm font-bold" style="color: var(--warning);">
							<TriangleAlert size={16} /> Perlu verifikasi
						</p>
						<div class="mt-3 space-y-3">
							{#each brief.needs_verification as question, idx (idx + '-' + question.slice(0, 24))}
								{@const state = brief.verification_state?.[String(idx)]}
								{#if state?.status === 'answered'}
									<div>
										<div class="flex items-start gap-2">
											<CircleCheck size={14} style="color: var(--success);" class="mt-0.5 shrink-0" />
											<p class="text-sm">{question}</p>
										</div>
										{#if state.response}
											<div class="ml-5 mt-1 rounded-lg p-2" style="background-color: var(--success-soft);">
												<p class="text-xs font-bold" style="color: var(--success);">Tanggapan</p>
												<p class="text-sm">{state.response}</p>
											</div>
										{/if}
									</div>
								{:else}
									{@const ignored = state?.status === 'ignored'}
									<div>
										<div class="flex flex-wrap items-start justify-between gap-2">
											<div class="flex items-start gap-2">
												<span class="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full" style="background-color: var(--warning);"></span>
												<p class="text-sm" style="color: {ignored ? 'var(--muted-foreground)' : 'var(--foreground)'}; text-decoration: {ignored ? 'line-through' : 'none'}; opacity: {ignored ? 0.7 : 1};">{question}</p>
											</div>
											{#if !verifyOpen[idx]}
												<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={() => (verifyOpen = { ...verifyOpen, [idx]: true })}>
													<Pencil size={12} /> Tanggapi
												</button>
											{/if}
										</div>
										{#if ignored && !verifyOpen[idx]}
											<p class="ml-5 mt-1 text-xs text-muted-foreground">Diabaikan — tidak ditindaklanjuti.</p>
										{/if}
										{#if verifyOpen[idx]}
											<div class="ml-5 mt-2 rounded-lg p-3" style="background-color: var(--warning-soft);">
												<textarea class="app-input w-full" rows="3" disabled={verifyBusy[idx]} placeholder={tr("Ketikkan jawaban atau klarifikasi verifikasi di sini...")} value={verifyDraft[idx] ?? ''} oninput={(e) => (verifyDraft = { ...verifyDraft, [idx]: (e.currentTarget as HTMLTextAreaElement).value })}></textarea>
												<div class="mt-2 flex justify-end gap-2">
													<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" disabled={verifyBusy[idx]} onclick={() => void submitVerify(idx, 'ignored')}>
														<X size={12} /> Abaiakan
													</button>
													<button class="app-btn app-btn-primary !min-h-7 !px-2 !text-xs" type="button" disabled={verifyBusy[idx] || !(verifyDraft[idx] ?? '').trim()} onclick={() => void submitVerify(idx, 'answered')}>
														<Check size={12} /> Tambahkan
													</button>
												</div>
											</div>
										{/if}
									</div>
								{/if}
							{/each}
						</div>
					{/if}
				</section>
			{/if}
		</div>

		<!-- Sidebar -->
		<div class="space-y-4">
			<section class="app-card p-4">
				<p class="text-sm font-bold text-muted-foreground">Ringkasan</p>
				<div class="mt-3 space-y-3 text-sm">
					<div class="flex items-start gap-3">
						<List size={16} style="color: var(--primary);" class="mt-0.5 shrink-0" />
						<div>
							<p class="text-xs text-muted-foreground">Channel & Angle</p>
							{#each brief.channels ?? [] as c (c.channel + '-' + c.angle)}
								<p class="font-medium">{CHANNEL_LABELS[c.channel] ?? c.channel} · {humanize(c.angle)}</p>
							{/each}
						</div>
					</div>
					<div class="flex items-start gap-3">
						<Link size={16} style="color: var(--primary);" class="mt-0.5 shrink-0" />
						<div>
							<p class="text-xs text-muted-foreground">Backlink</p>
							<p class="font-medium">{hasSeo ? 'Penanaman link aktif' : 'Penanaman link tidak diaktifkan'}</p>
						</div>
					</div>
					<div class="flex items-start gap-3">
						<FileText size={16} style="color: var(--primary);" class="mt-0.5 shrink-0" />
						<div>
							<p class="text-xs text-muted-foreground">Sumber</p>
							<p class="font-medium">{sources.length > 0 ? `${sources.length} sumber` : tr('Tidak ada sumber eksternal')}</p>
						</div>
					</div>
				</div>
			</section>

			<section class="app-card p-4" style="background-color: var(--accent);">
				<div class="flex items-center gap-2">
					<Sparkles size={18} style="color: var(--primary);" />
					<p class="font-bold" style="color: var(--accent-foreground);">Insight Utama</p>
				</div>
				<p class="mt-2 text-sm" style="color: var(--accent-foreground);">{tr("AI telah merangkum strategi inti kampanye berdasarkan input Anda. Pastikan semua data sudah akurat sebelum melanjutkan ke Agent Matchmaker.")}</p>
			</section>

			<section class="app-card p-4">
				<p class="text-xs font-bold uppercase tracking-wide text-muted-foreground">{tr("Detail Sumber (")}{sources.length})</p>
				{#if sources.length > 0}
					<div class="mt-2 space-y-3">
						{#each sources as s, i (s + '-' + i)}
							{@const isUrl = /^https?:\/\//i.test(s)}
							<div>
								{#if i > 0}<hr class="mb-3" />{/if}
								<div class="flex flex-wrap items-center gap-2">
									{#if isUrl}<Globe size={14} class="text-muted-foreground" />{:else}<FileText size={14} class="text-muted-foreground" />{/if}
									<span class="font-medium">{isUrl ? 'Website Resmi' : tr('Input Klien')}</span>
									<span class="rounded-full px-2 py-0.5 text-[10px] font-semibold" style="background-color: {i === 0 && isUrl ? 'var(--accent)' : 'var(--muted)'}; color: {i === 0 && isUrl ? 'var(--accent-foreground)' : 'var(--muted-foreground)'};">
										{i === 0 && isUrl ? 'Utama' : 'Pendukung'}
									</span>
								</div>
								{#if isUrl}
									<a class="block break-all text-sm" style="color: var(--primary);" href={s} target="_blank" rel="noopener noreferrer">{s}</a>
								{:else}
									<p class="text-sm text-muted-foreground">{s}</p>
								{/if}
								<p class="text-xs text-muted-foreground">{tr("Diakses saat analisis")}</p>
							</div>
						{/each}
					</div>
				{:else}
					<p class="mt-2 text-sm text-muted-foreground">{tr("Analisis menggunakan input kampanye Anda tanpa sumber eksternal.")}</p>
				{/if}
			</section>
		</div>
	</div>

	<div class="flex items-center justify-center gap-2 py-2">
		<Lock size={12} class="text-muted-foreground" />
		<p class="text-xs text-muted-foreground">{tr("Dokumen ini dikunci secara otomatis. Perubahan tidak dapat dilakukan setelah diteruskan.")}</p>
	</div>
</div>
