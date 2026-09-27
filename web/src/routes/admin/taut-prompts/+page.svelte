<script lang="ts">
	// Port app/admin/taut-prompts/page.tsx — Taut Prompt Studio.
	// Hierarki: Channel Media → Content Angle → Versi prompt (satu versi aktif,
	// bisa diedit selamanya) + uji prompt (generate artikel lalu analisis
	// kepatuhan angle). Hanya super admin.
	import { onMount } from 'svelte';
	import { Pencil, Plus, Save, Trash2, TriangleAlert } from '@lucide/svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import { fmtDate } from '$lib/admin-format';
	import { sanitizeHtml } from '$lib/sanitize-html';

	type Channel = { id: string; name: string; slug: string; description: string | null; active: boolean; angle_count?: number };
	type Angle = { id: string; channel_id: string; name: string; active: boolean; active_version?: number | null };
	type VersionMeta = { id: string; version: number; status: string; change_note: string | null; created_at: string };
	type VersionDetail = VersionMeta & { prompt: string | null };

	let channels = $state<Channel[]>([]);
	let angles = $state<Angle[]>([]);
	let versions = $state<VersionMeta[]>([]);
	let activeVersion = $state<VersionDetail | null>(null);
	let selectedChannel = $state<Channel | null>(null);
	let selectedAngle = $state<Angle | null>(null);

	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');

	let showChannel = $state(false);
	let showAngle = $state(false);
	let showEditAngle = $state(false);
	let channelForm = $state({ name: '', slug: '', description: '' });
	let angleForm = $state({ name: '', slug: '', description: '' });
	let editAngleForm = $state({ name: '', description: '', active: true });

	let newPromptText = $state('');
	let newPromptNote = $state('');

	let testBahan = $state('');
	let testLoading = $state(false);
	let testError = $state('');
	let testResult = $state<any>(null);

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await loadChannels();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function loadChannels() {
		const res = await apiRequest('/v1/platform/taut/channels');
		if (res.ok) channels = (await res.json()).items ?? [];
		else error = await readApiError(res, 'Gagal memuat channel.');
	}

	async function loadAngles(channelId: string) {
		const res = await apiRequest(`/v1/platform/taut/channels/${channelId}/angles`);
		if (res.ok) angles = (await res.json()).items ?? [];
		else error = await readApiError(res, 'Gagal memuat angle.');
	}

	async function loadVersions(angleId: string) {
		const res = await apiRequest(`/v1/platform/taut/angles/${angleId}/versions`);
		if (!res.ok) {
			error = await readApiError(res, 'Gagal memuat versi prompt.');
			return;
		}
		versions = (await res.json()).items ?? [];
		const active = versions.find((v) => v.status === 'active') ?? versions[0] ?? null;
		if (active) {
			const detail = await apiRequest(`/v1/platform/taut/angles/${angleId}/versions/${active.id}`);
			if (detail.ok) activeVersion = (await detail.json()).version;
		} else {
			activeVersion = null;
		}
	}

	function selectChannel(channel: Channel | null) {
		selectedChannel = channel;
		selectedAngle = null;
		versions = [];
		activeVersion = null;
		if (channel) void loadAngles(channel.id);
	}

	async function selectAngle(angle: Angle) {
		selectedAngle = angle;
		testResult = null;
		await loadVersions(angle.id);
	}

	async function createChannel(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		try {
			const res = await apiRequest('/v1/platform/taut/channels', { method: 'POST', body: JSON.stringify(channelForm) });
			if (!res.ok) {
				error = await readApiError(res, 'Gagal membuat channel.');
				return;
			}
			showChannel = false;
			channelForm = { name: '', slug: '', description: '' };
			await loadChannels();
		} finally {
			busy = false;
		}
	}

	async function deleteChannel(id: string) {
		if (!confirm('Hapus channel ini? Semua angle di dalamnya ikut terhapus.')) return;
		const res = await apiRequest(`/v1/platform/taut/channels/${id}`, { method: 'DELETE' });
		if (!res.ok) {
			error = await readApiError(res, 'Gagal menghapus channel.');
			return;
		}
		if (selectedChannel?.id === id) selectChannel(null);
		await loadChannels();
	}

	async function createAngle(e: SubmitEvent) {
		e.preventDefault();
		if (!selectedChannel) return;
		busy = true;
		try {
			const res = await apiRequest(`/v1/platform/taut/channels/${selectedChannel.id}/angles`, {
				method: 'POST',
				body: JSON.stringify(angleForm)
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal membuat angle.');
				return;
			}
			showAngle = false;
			angleForm = { name: '', slug: '', description: '' };
			await loadAngles(selectedChannel.id);
		} finally {
			busy = false;
		}
	}

	function openEditAngle(angle: Angle) {
		editAngleForm = { name: angle.name, description: '', active: angle.active };
		showEditAngle = true;
	}

	async function saveEditAngle(e: SubmitEvent) {
		e.preventDefault();
		if (!selectedAngle) return;
		busy = true;
		try {
			const res = await apiRequest(`/v1/platform/taut/angles/${selectedAngle.id}`, {
				method: 'PATCH',
				body: JSON.stringify(editAngleForm)
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal menyimpan angle.');
				return;
			}
			showEditAngle = false;
			if (selectedChannel) await loadAngles(selectedChannel.id);
		} finally {
			busy = false;
		}
	}

	async function deleteAngle(id: string) {
		if (!confirm('Hapus angle ini? Prompt-nya ikut terhapus.')) return;
		const res = await apiRequest(`/v1/platform/taut/angles/${id}`, { method: 'DELETE' });
		if (!res.ok) {
			error = await readApiError(res, 'Gagal menghapus angle.');
			return;
		}
		if (selectedAngle?.id === id) {
			selectedAngle = null;
			versions = [];
			activeVersion = null;
		}
		if (selectedChannel) await loadAngles(selectedChannel.id);
	}

	async function saveActivePrompt() {
		if (!selectedAngle || !activeVersion) return;
		const res = await apiRequest(`/v1/platform/taut/angles/${selectedAngle.id}/versions/${activeVersion.id}`, {
			method: 'PATCH',
			body: JSON.stringify({ prompt: activeVersion.prompt ?? '', change_note: activeVersion.change_note ?? '' })
		});
		if (!res.ok) {
			error = await readApiError(res, 'Gagal menyimpan prompt.');
			return;
		}
		notice = 'Prompt tersimpan.';
	}

	async function deleteVersion(versionId: string) {
		if (!selectedAngle) return;
		if (!confirm('Hapus versi prompt ini?')) return;
		const res = await apiRequest(`/v1/platform/taut/angles/${selectedAngle.id}/versions/${versionId}`, { method: 'DELETE' });
		if (!res.ok) {
			error = await readApiError(res, 'Gagal menghapus versi.');
			return;
		}
		await loadVersions(selectedAngle.id);
	}

	async function createFirstPrompt() {
		if (!selectedAngle) return;
		const res = await apiRequest(`/v1/platform/taut/angles/${selectedAngle.id}/versions`, {
			method: 'POST',
			body: JSON.stringify({ prompt: newPromptText, change_note: newPromptNote })
		});
		if (!res.ok) {
			error = await readApiError(res, 'Gagal membuat prompt.');
			return;
		}
		newPromptText = '';
		newPromptNote = '';
		await loadVersions(selectedAngle.id);
		if (selectedChannel) await loadAngles(selectedChannel.id);
	}

	async function runAngleTest() {
		if (!selectedAngle) return;
		const promptOverride = activeVersion?.prompt ?? newPromptText;
		if (!promptOverride?.trim()) {
			testError = 'Prompt masih kosong — isi dulu sebelum test.';
			return;
		}
		if (!testBahan.trim()) {
			testError = 'Bahan uji tidak boleh kosong.';
			return;
		}
		testLoading = true;
		testError = '';
		testResult = null;
		try {
			const res = await apiRequest(`/v1/platform/taut/angles/${selectedAngle.id}/test`, {
				method: 'POST',
				body: JSON.stringify({ bahan: testBahan, promptOverride })
			});
			const data = await res.json().catch(() => null);
			if (!res.ok) {
				testError = data?.detail ?? 'Uji prompt gagal.';
				return;
			}
			testResult = data;
		} finally {
			testLoading = false;
		}
	}

	const archivedVersions = $derived(versions.filter((v) => v.id !== activeVersion?.id));
</script>

<section>
	<PageHeader title="Taut Prompt Studio" description="Satu angle = satu prompt aktif yang bisa diedit selamanya. Hapus versi lama yang tidak dipakai untuk hemat resource.">
		<span class="rounded-full px-2.5 py-0.5 text-xs font-medium" style="background: var(--accent); color: var(--primary);">{channels.length} channel</span>
		<button class="app-btn app-btn-primary" onclick={() => (showChannel = true)}><Plus size={15} /> Buat Channel</button>
	</PageHeader>

	<Feedback {loading} {error} {notice} />

	{#if !loading}
		<div class="mt-4 grid gap-3 md:grid-cols-2">
			<!-- Channel -->
			<div class="app-card p-4">
				<h2 class="text-sm font-semibold">Channel Media</h2>
				<div class="mt-2 max-h-[380px] overflow-y-auto">
					<ul class="flex flex-col gap-1">
						{#each channels as c (c.id)}
							<li class="flex items-center gap-1">
								<button
									class="app-btn !min-h-9 flex-1 justify-start !px-3"
									class:app-btn-primary={selectedChannel?.id === c.id}
									class:app-btn-soft={selectedChannel?.id !== c.id}
									onclick={() => selectChannel(c)}
								>
									<span class="truncate">{c.name}</span>
									<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--secondary);">{c.angle_count ?? 0}</span>
								</button>
								<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label={`Hapus channel ${c.name}`} onclick={() => deleteChannel(c.id)}>
									<Trash2 size={14} />
								</button>
							</li>
						{/each}
						{#if channels.length === 0}
							<li class="py-6 text-center text-sm text-muted-foreground">Belum ada channel.</li>
						{/if}
					</ul>
				</div>
			</div>

			<!-- Angles -->
			<div class="app-card p-4">
				<div class="flex items-center justify-between gap-2">
					<h2 class="text-sm font-semibold">{selectedChannel ? `Angles · ${selectedChannel.name}` : 'Pilih channel dulu'}</h2>
					{#if selectedChannel}
						<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" onclick={() => (showAngle = true)}><Plus size={13} /> Angle</button>
					{/if}
				</div>
				<div class="mt-2 max-h-[380px] overflow-y-auto">
					<ul class="flex flex-col gap-1">
						{#each angles as a (a.id)}
							<li class="flex items-center gap-1">
								<button
									class="app-btn !min-h-9 flex-1 justify-start !px-3"
									class:app-btn-primary={selectedAngle?.id === a.id}
									class:app-btn-soft={selectedAngle?.id !== a.id}
									onclick={() => selectAngle(a)}
								>
									<span class="truncate">{a.name}</span>
									{#if a.active_version != null}
										<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--primary); color: var(--primary-foreground);">v{a.active_version}</span>
									{:else}
										<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--warning-soft); color: var(--warning);">tanpa prompt</span>
									{/if}
									{#if !a.active}
										<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--secondary); color: var(--muted-foreground);">nonaktif</span>
									{/if}
								</button>
								<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label={`Edit angle ${a.name}`} onclick={() => openEditAngle(a)}><Pencil size={14} /></button>
								<button class="rounded-lg p-1.5 hover:bg-secondary" aria-label={`Hapus angle ${a.name}`} onclick={() => deleteAngle(a.id)}><Trash2 size={14} /></button>
							</li>
						{/each}
						{#if angles.length === 0}
							<li class="py-6 text-center text-sm text-muted-foreground">{selectedChannel ? 'Belum ada angle untuk channel ini.' : '—'}</li>
						{/if}
					</ul>
				</div>
			</div>
		</div>

		{#if selectedAngle}
			<div class="app-card mt-4 p-4">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<div class="flex items-center gap-2">
						<h2 class="text-sm font-semibold">{selectedAngle.name}</h2>
						<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--success-soft); color: var(--success);">aktif</span>
						{#if activeVersion}<span class="text-xs text-muted-foreground">v{activeVersion.version}</span>{/if}
					</div>
					<button class="app-btn app-btn-primary" disabled={busy || !activeVersion} onclick={saveActivePrompt}>
						<Save size={15} /> Simpan
					</button>
				</div>

				{#if activeVersion}
					<label class="mt-4 block">
						<span class="text-xs font-medium text-muted-foreground">Prompt (satu-satunya yang aktif — edit & simpan kapan saja)</span>
						<textarea
							class="app-input mt-1 w-full font-mono text-xs"
							rows="14"
							placeholder="Tulis prompt lengkap angle ini dalam satu kesatuan. Dipakai AI The Writer untuk membuat konten."
							bind:value={activeVersion.prompt}
						></textarea>
					</label>
					<label class="mt-3 block">
						<span class="text-xs font-medium text-muted-foreground">Catatan perubahan</span>
						<input class="app-input mt-1 w-full" placeholder="Ringkasan perubahan (opsional)" bind:value={activeVersion.change_note} />
					</label>
				{:else}
					<div class="mt-4 rounded-lg px-3 py-2 text-sm" style="background: var(--warning-soft); color: var(--warning);">
						Angle ini belum punya prompt aktif — buat prompt pertamanya di sini (kamu super admin).
					</div>
					<label class="mt-3 block">
						<span class="text-xs font-medium text-muted-foreground">Prompt awal (akan langsung jadi versi aktif)</span>
						<textarea class="app-input mt-1 w-full font-mono text-xs" rows="14" placeholder="Tulis prompt lengkap angle ini…" bind:value={newPromptText}></textarea>
					</label>
					<label class="mt-3 block">
						<span class="text-xs font-medium text-muted-foreground">Catatan perubahan</span>
						<input class="app-input mt-1 w-full" placeholder="Versi awal (opsional)" bind:value={newPromptNote} />
					</label>
					<button class="app-btn app-btn-primary mt-3" disabled={!newPromptText.trim()} onclick={createFirstPrompt}>
						<Plus size={15} /> Buat Prompt Aktif
					</button>
				{/if}

				<!-- Uji prompt -->
				<div class="mt-4 rounded-lg border p-3" style="border-color: var(--primary);">
					<h3 class="text-sm font-semibold">Test Prompt — Bahan & Analisis Kepatuhan</h3>
					<p class="mt-1 text-xs text-muted-foreground">
						Isi bahan (fakta/produk/masalah), lalu Test. Artikel digenerate dari prompt angle di atas + bahan,
						lalu dianalisis apakah artikel benar-benar sesuai angle. Hanya testing — tidak membuat campaign.
					</p>
					<label class="mt-3 block">
						<span class="text-xs font-medium text-muted-foreground">Bahan uji</span>
						<textarea
							class="app-input mt-1 w-full"
							rows="6"
							maxlength="8000"
							placeholder="Tempel bahan / fakta / produk / masalah yang ingin diuji… (maks 8000 karakter)"
							bind:value={testBahan}
						></textarea>
					</label>
					<div class="mt-1 flex items-center justify-between text-[11px] text-muted-foreground">
						<span>{testBahan.length}/8000</span>
						<span>Pakai draft di textarea prompt (belum disimpan tetap kebawa).</span>
					</div>
					{#if testError}<p class="mt-2 text-sm" style="color: var(--destructive);">{testError}</p>{/if}
					<div class="mt-2 flex items-center gap-2">
						<button
							class="app-btn app-btn-primary"
							disabled={testLoading || !testBahan.trim() || !(activeVersion?.prompt ?? newPromptText).trim()}
							onclick={runAngleTest}
						>{testLoading ? 'Menguji… (generate + analisis)' : 'Test Generate & Analisis'}</button>
						{#if testResult}
							<button class="app-btn app-btn-soft" onclick={() => (testResult = null)}>Bersihkan hasil</button>
						{/if}
					</div>
					{#if testLoading}
						<p class="mt-3 text-center text-sm text-muted-foreground">Memanggil writer → lalu gatekeeper untuk analisis…</p>
					{/if}

					{#if testResult}
						<div class="mt-3 rounded-lg border p-3">
							<div class="flex flex-wrap items-center gap-2">
								<strong class="text-sm">{testResult.article?.title ?? '—'}</strong>
								{#if testResult.analysis}
									<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: {testResult.analysis.compliant ? 'var(--success-soft)' : 'var(--warning-soft)'}; color: {testResult.analysis.compliant ? 'var(--success)' : 'var(--warning)'};">
										{testResult.analysis.compliant ? 'Sesuai angle' : 'Perlu perbaikan'}
									</span>
									<span class="rounded-full px-2 py-0.5 text-[11px]" style="background: var(--accent); color: var(--primary);">Score {testResult.analysis.score ?? 0}/100</span>
								{/if}
							</div>
							{#if testResult.analysis?.summary}
								<p class="mt-2 rounded-lg px-3 py-2 text-sm" style="background: var(--accent);">{testResult.analysis.summary}</p>
							{/if}
							<p class="mt-3 text-xs font-semibold">Artikel (preview)</p>
							<div class="mt-1 max-h-[360px] overflow-y-auto rounded-lg border p-3">
								<!-- Konten server di-sanitasi DOMPurify (parity sanitizeHtml app lama). -->
								{@html sanitizeHtml(testResult.article?.content_html ?? '')}
							</div>
							{#if Array.isArray(testResult.analysis?.checks) && testResult.analysis.checks.length > 0}
								<div class="mt-3 flex flex-col gap-1">
									<p class="text-xs font-semibold">Checks</p>
									{#each testResult.analysis.checks as c, i (i)}
										<p class="flex items-center gap-2 text-xs">
											<span class="rounded px-1.5 py-0.5 text-[10px]" style="background: {c.pass ? 'var(--success-soft)' : 'var(--destructive-soft, #fdecee)'}; color: {c.pass ? 'var(--success)' : 'var(--destructive)'};">{c.pass ? 'OK' : 'Fail'}</span>
											<span>{c.criterion}{c.notes ? ` — ${c.notes}` : ''}</span>
										</p>
									{/each}
								</div>
							{/if}
							{#if Array.isArray(testResult.analysis?.issues) && testResult.analysis.issues.length > 0}
								<p class="mt-2 text-xs" style="color: var(--destructive);">Issues: {testResult.analysis.issues.join(' · ')}</p>
							{/if}
							{#if Array.isArray(testResult.analysis?.suggestions) && testResult.analysis.suggestions.length > 0}
								<p class="mt-2 text-xs" style="color: var(--primary);">Saran: {testResult.analysis.suggestions.join(' · ')}</p>
							{/if}
							<p class="mt-2 text-[11px] text-muted-foreground">Model: {testResult.usage?.generate?.model ?? '—'} · {testResult.durationMs ?? 0}ms</p>
						</div>
					{/if}
				</div>

				{#if archivedVersions.length > 0}
					<div class="mt-3 rounded-lg border p-3">
						<p class="text-xs font-semibold text-muted-foreground">Riwayat versi lama — hapus biar hemat</p>
						<ul class="mt-2 flex flex-col gap-1">
							{#each archivedVersions as v (v.id)}
								<li class="flex items-center justify-between gap-2">
									<span class="text-xs text-muted-foreground">
										v{v.version} · {v.status} · {fmtDate(v.created_at)}{v.change_note ? ` — ${v.change_note}` : ''}
									</span>
									<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-[11px]" onclick={() => deleteVersion(v.id)}>
										<Trash2 size={11} /> Hapus
									</button>
								</li>
							{/each}
						</ul>
					</div>
				{/if}
			</div>
		{/if}
	{/if}
</section>

{#if showChannel}
	<Modal title="Buat Channel Media" label="Buat channel" onClose={() => (showChannel = false)}>
		<form class="flex flex-col gap-3" onsubmit={createChannel}>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Nama</span>
				<input class="app-input mt-1 w-full" required placeholder="Media Mainstream / Nasional" bind:value={channelForm.name} />
			</label>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Slug</span>
				<input class="app-input mt-1 w-full" required placeholder="mainstream" bind:value={channelForm.slug} />
			</label>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Deskripsi</span>
				<textarea class="app-input mt-1 w-full" placeholder="Pendekatan jurnalistik…" bind:value={channelForm.description}></textarea>
			</label>
			<div class="flex justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (showChannel = false)}>Batal</button>
				<button class="app-btn app-btn-primary" type="submit" disabled={busy}>Buat Channel</button>
			</div>
		</form>
	</Modal>
{/if}

{#if showAngle}
	<Modal title="Buat Content Angle" label="Buat angle" onClose={() => (showAngle = false)}>
		<form class="flex flex-col gap-3" onsubmit={createAngle}>
			<p class="text-xs text-muted-foreground">Untuk channel: {selectedChannel?.name ?? '—'}</p>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Nama</span>
				<input class="app-input mt-1 w-full" required placeholder="Peluncuran Produk / Fitur Baru" bind:value={angleForm.name} />
			</label>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Slug</span>
				<input class="app-input mt-1 w-full" required placeholder="product_launch" bind:value={angleForm.slug} />
			</label>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Deskripsi</span>
				<textarea class="app-input mt-1 w-full" placeholder="Pengumuman peluncuran produk…" bind:value={angleForm.description}></textarea>
			</label>
			<div class="flex justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (showAngle = false)}>Batal</button>
				<button class="app-btn app-btn-primary" type="submit" disabled={busy}>Buat Angle</button>
			</div>
		</form>
	</Modal>
{/if}

{#if showEditAngle}
	<Modal title="Edit Angle" label="Edit angle" onClose={() => (showEditAngle = false)}>
		<form class="flex flex-col gap-3" onsubmit={saveEditAngle}>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Nama</span>
				<input class="app-input mt-1 w-full" required bind:value={editAngleForm.name} />
			</label>
			<label class="block">
				<span class="text-xs font-medium text-muted-foreground">Deskripsi</span>
				<textarea class="app-input mt-1 w-full" bind:value={editAngleForm.description}></textarea>
			</label>
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={editAngleForm.active} />
				{editAngleForm.active ? 'Aktif' : 'Nonaktif'}
			</label>
			<div class="flex justify-end gap-2">
				<button class="app-btn app-btn-soft" type="button" onclick={() => (showEditAngle = false)}>Batal</button>
				<button class="app-btn app-btn-primary" type="submit" disabled={busy}>Simpan</button>
			</div>
		</form>
	</Modal>
{/if}
