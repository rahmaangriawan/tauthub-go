<script lang="ts">
	// Fase 4: halaman Generate + daftar job (port section Generate minimal).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);

	type Job = {
		id: string;
		status: string;
		current_step: string;
		progress: number;
		topic?: string;
		document_id?: string | null;
		error_message?: string | null;
		outline?: Array<{ level: number; title: string }>;
		evaluation?: Record<string, unknown>;
	};

	let jobs = $state<Job[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let msg = $state('');
	let form = $state({ topic: '', goal: '', content_type: 'press_release', require_outline_approval: false });

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		const res = await apiRequest(`/v1/brands/${brandId}/content-jobs?limit=50`);
		if (res.ok) {
			const data = await res.json();
			jobs = data.items ?? [];
			total = data.total ?? 0;
		}
	}

	async function generate(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		msg = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/content-jobs`, {
				method: 'POST',
				body: JSON.stringify({
					topic: form.topic.trim(),
					goal: form.goal.trim(),
					content_type: form.content_type,
					require_outline_approval: form.require_outline_approval,
					target_words: 500
				})
			});
			const data = await res.json();
			if (!res.ok) {
				error = await readApiError(res, 'Gagal membuat job generasi.');
				return;
			}
			msg = `Job ${data.status} — ${data.current_step}`;
			form.topic = '';
			form.goal = '';
			await load();
		} finally {
			busy = false;
		}
	}

	async function approveOutline(jobId: string) {
		busy = true;
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/content-jobs/${jobId}/approve-outline`, { method: 'POST' });
			if (res.ok) {
				msg = 'Outline disetujui, generasi artikel dijalankan.';
				await load();
			} else {
				error = await readApiError(res, 'Gagal menyetujui outline.');
			}
		} finally {
			busy = false;
		}
	}

	async function cancelJob(jobId: string) {
		busy = true;
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/content-jobs/${jobId}/cancel`, { method: 'POST' });
			if (res.ok) {
				msg = 'Job dibatalkan.';
				await load();
			}
		} finally {
			busy = false;
		}
	}

	const statusColor: Record<string, string> = {
		completed: 'bg-emerald-50 text-emerald-700',
		failed: 'bg-red-50 text-red-700',
		cancelled: 'bg-slate-100 text-slate-600',
		waiting_outline_approval: 'bg-amber-50 text-amber-700',
		queued: 'bg-blue-50 text-primary',
		running: 'bg-blue-50 text-primary'
	};
</script>

<section class="mx-auto max-w-5xl">
	<h1 class="text-2xl font-semibold tracking-tight">Generate Artikel</h1>
	<p class="mt-1 text-sm text-muted-foreground">
		Pipeline: brief → (opsional approval outline) → draft AI → gate moderasi → dokumen.
	</p>

	<form class="app-card mt-4 grid gap-3 p-4" onsubmit={generate}>
		<div class="grid gap-3 sm:grid-cols-2">
			<label class="flex flex-col gap-1 text-sm sm:col-span-2">Topik
				<input class="app-input" bind:value={form.topic} minlength={3} required />
			</label>
			<label class="flex flex-col gap-1 text-sm sm:col-span-2">Tujuan
				<input class="app-input" bind:value={form.goal} minlength={3} required />
			</label>
			<label class="flex flex-col gap-1 text-sm">Jenis konten
				<select class="app-input" bind:value={form.content_type}>
					<option value="press_release">Press Release</option>
					<option value="news">News</option>
					<option value="article">Article</option>
					<option value="feature">Feature</option>
				</select>
			</label>
			<label class="flex items-center gap-2 self-end text-sm">
				<input type="checkbox" bind:checked={form.require_outline_approval} />
				Perlu persetujuan outline dulu
			</label>
		</div>
		{#if error}<p class="text-sm text-destructive">{error}</p>{/if}
		{#if msg}<p class="text-sm text-muted-foreground">{msg}</p>{/if}
		<button class="app-btn app-btn-primary w-fit" disabled={busy} type="submit">
			{busy ? 'Memproses…' : 'Generate'}
		</button>
	</form>

	<section class="mt-6">
		<h2 class="font-semibold">Riwayat job <span class="text-sm font-normal text-muted-foreground">({total})</span></h2>
		{#if loading}
			<p class="mt-2 text-sm text-muted-foreground">Memuat…</p>
		{:else}
			<div class="mt-2 grid gap-2">
				{#each jobs as job (job.id)}
					<div class="app-card min-w-0 p-3">
						<div class="flex items-start justify-between gap-3">
							<div class="min-w-0">
								<p class="truncate text-sm font-medium">{job.topic ?? '(tanpa topik)'}</p>
								<p class="mt-0.5 text-xs text-muted-foreground">
									{job.current_step} · {job.progress}%
									{#if job.evaluation?.word_count} · {job.evaluation.word_count} kata{/if}
									{#if job.evaluation?.mocked} · <span class="text-amber-600">mock AI</span>{/if}
								</p>
								{#if job.error_message}
									<p class="mt-1 text-xs text-destructive">{job.error_message}</p>
								{/if}
							</div>
							<div class="flex shrink-0 items-center gap-2">
								<span class="rounded-full px-2 py-0.5 text-[10px] font-semibold {statusColor[job.status] ?? 'bg-muted'}">
									{job.status}
								</span>
								{#if job.document_id}
									<a class="text-xs underline" href={`/klien/${brandUrl}/editor/${job.document_id}`}>Buka editor</a>
								{/if}
								{#if job.status === 'waiting_outline_approval'}
									<button class="rounded-md bg-primary px-2 py-1 text-xs font-medium text-primary-foreground" disabled={busy} onclick={() => approveOutline(job.id)}>
										Setujui outline
									</button>
								{:else if !['completed', 'failed', 'cancelled'].includes(job.status)}
									<button class="text-xs text-destructive underline" disabled={busy} onclick={() => cancelJob(job.id)}>Batalkan</button>
								{/if}
							</div>
						</div>
						{#if job.status === 'waiting_outline_approval' && (job.outline?.length ?? 0) > 0}
							<ul class="mt-2 space-y-0.5 text-xs text-muted-foreground">
								{#each job.outline ?? [] as item, i (i)}
									<li style="padding-left:{(item.level - 2) * 12}px">{"#".repeat(item.level)} {item.title}</li>
								{/each}
							</ul>
						{/if}
					</div>
				{:else}
					<p class="text-sm text-muted-foreground">Belum ada job generasi.</p>
				{/each}
			</div>
		{/if}
	</section>
</section>
