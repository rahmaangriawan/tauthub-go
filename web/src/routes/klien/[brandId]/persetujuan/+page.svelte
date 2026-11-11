<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Persetujuan belanja: admin/owner mengatur ambang dan memutuskan pengajuan
	// distribusi kampanye yang melebihi ambang. Peran lain melihat pengajuannya
	// sendiri. Persetujuan berlaku 7 hari dan sekali pakai.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';

	type Approval = {
		id: string; campaign_id: string; campaign_name: string; cost_tc: number; status: string;
		requester: string; created_at: string; decided_at: string | null; decision_note: string | null;
		expires_at: string | null;
	};

	const brandId = $derived(page.params.brandId as string);
	const brandUrl = $derived(getBrandUrlId() || brandId);

	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let notice = $state('');
	let canApprove = $state(false);
	let enabled = $state(false);
	let threshold = $state('');
	let items = $state<Approval[]>([]);
	let showAll = $state(false);
	let notes = $state<Record<string, string>>({});

	const STATUS: Record<string, string> = {
		pending: 'Menunggu', approved: 'Disetujui', rejected: 'Ditolak', used: 'Terpakai', cancelled: 'Ditarik'
	};
	const nf = new Intl.NumberFormat('id-ID');
	const dt = (v: string | null) =>
		v ? new Date(v).toLocaleDateString('id-ID', { day: 'numeric', month: 'short', year: 'numeric' }) : '';

	async function load() {
		error = '';
		const [p, a] = await Promise.all([
			apiRequest(`/v1/brands/${brandId}/approval-policy`),
			apiRequest(`/v1/brands/${brandId}/approvals?status=${showAll ? 'all' : 'pending'}`)
		]);
		if (!p.ok) {
			error = await readApiError(p, tr('Gagal memuat kebijakan persetujuan.'));
			return;
		}
		const pj = await p.json();
		enabled = Boolean(pj.enabled);
		canApprove = Boolean(pj.can_approve);
		threshold = pj.threshold_tc > 0 ? String(pj.threshold_tc) : '';
		if (a.ok) items = (await a.json()).items ?? [];
	}

	onMount(() => {
		void (async () => {
			if (!(await ensureValidSession())) return;
			await load();
			loading = false;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function savePolicy(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/approval-policy`, {
				method: 'PUT',
				body: JSON.stringify({ enabled, threshold_tc: Number(threshold) })
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menyimpan kebijakan.'));
				return;
			}
			notice = enabled
				? `Kampanye di atas ${nf.format(Number(threshold))} TC perlu persetujuan admin atau owner.`
				: 'Persetujuan dimatikan. Semua peran penulis bisa mendistribusikan langsung.';
		} finally {
			busy = false;
		}
	}

	async function act(a: Approval, kind: 'approve' | 'reject' | 'cancel') {
		busy = true;
		error = '';
		notice = '';
		try {
			const url = `/v1/brands/${brandId}/approvals/${a.id}/${kind === 'cancel' ? 'cancel' : 'decide'}`;
			const res = await apiRequest(url, {
				method: 'POST',
				body: JSON.stringify(kind === 'cancel' ? {} : { decision: kind, note: notes[a.id] ?? '' })
			});
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal memproses pengajuan.'));
				return;
			}
			notice = kind === 'approve' ? 'Pengajuan disetujui.' : kind === 'reject' ? 'Pengajuan ditolak.' : 'Pengajuan ditarik.';
			await load();
		} finally {
			busy = false;
		}
	}
</script>

<PageTitle title="Persetujuan belanja" />

<section>
	<header>
		<h1 class="page-title">{tr("Persetujuan belanja")}</h1>
		<p class="page-desc">{tr("Tahan distribusi kampanye di atas ambang sampai admin atau owner menyetujuinya.")}</p>
	</header>

	{#if notice}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>{/if}
	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	{#if loading}
		<p class="mt-6 text-sm text-muted-foreground">{tr("Memuat…")}</p>
	{:else}
		{#if canApprove}
			<form class="app-card mt-4 p-4" onsubmit={savePolicy}>
				<h2 class="text-sm font-semibold">{tr("Kebijakan")}</h2>
				<label class="mt-3 flex items-center gap-2 text-sm">
					<input type="checkbox" bind:checked={enabled} />
					{tr("Minta persetujuan untuk kampanye di atas ambang")}
				</label>
				<label class="mt-3 block">
					<span class="text-xs font-medium text-muted-foreground">{tr("Ambang biaya kampanye (TC)")}</span>
					<input class="app-input mt-1 w-full max-w-[220px]" type="number" min="1" step="1" required bind:value={threshold} />
				</label>
				<p class="mt-2 text-xs text-muted-foreground">
					{tr("Admin dan owner tidak ditahan. Berlaku untuk distribusi kampanye Taut; jalur pemesanan lain belum memakai persetujuan.")}
				</p>
				<div class="mt-3 flex justify-end">
					<button class="app-btn app-btn-primary" type="submit" disabled={busy || !threshold}>{tr("Simpan")}</button>
				</div>
			</form>
		{/if}

		<div class="mt-6 flex items-center justify-between">
			<h2 class="text-sm font-semibold">{canApprove ? tr('Antrean pengajuan') : tr('Pengajuan Anda')}</h2>
			<label class="flex items-center gap-2 text-xs text-muted-foreground">
				<input type="checkbox" bind:checked={showAll} onchange={() => void load()} />
				{tr("Tampilkan semua status")}
			</label>
		</div>

		{#if items.length === 0}
			<div class="app-card mt-3 px-4 py-8 text-center text-sm text-muted-foreground">{tr("Tidak ada pengajuan.")}</div>
		{:else}
			<div class="mt-3 grid gap-3">
				{#each items as a (a.id)}
					<div class="app-card p-4">
						<div class="flex flex-wrap items-start justify-between gap-2">
							<div>
								<a class="text-sm font-medium underline" href={`/klien/${brandUrl}/taut-pintar/${a.campaign_id}/writer`}>{a.campaign_name}</a>
								<p class="mt-0.5 text-xs text-muted-foreground">
									{a.requester} · {nf.format(a.cost_tc)} TC · {dt(a.created_at)}
								</p>
							</div>
							<span class="text-xs font-medium">{tr(STATUS[a.status] ?? a.status)}</span>
						</div>
						{#if a.decision_note}<p class="mt-2 text-xs text-muted-foreground">{tr("Catatan")}: {a.decision_note}</p>{/if}
						{#if a.status === 'approved' && a.expires_at}
							<p class="mt-2 text-xs text-muted-foreground">{tr("Berlaku sampai")} {dt(a.expires_at)}</p>
						{/if}
						{#if a.status === 'pending'}
							{#if canApprove}
								<input class="app-input mt-3 w-full" placeholder={tr("Catatan (opsional)")} maxlength="500" bind:value={notes[a.id]} />
								<div class="mt-3 flex justify-end gap-2">
									<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => void act(a, 'reject')}>{tr("Tolak")}</button>
									<button class="app-btn app-btn-primary" type="button" disabled={busy} onclick={() => void act(a, 'approve')}>{tr("Setujui")}</button>
								</div>
							{:else}
								<div class="mt-3 flex justify-end">
									<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => void act(a, 'cancel')}>{tr("Tarik pengajuan")}</button>
								</div>
							{/if}
						{/if}
					</div>
				{/each}
			</div>
		{/if}
	{/if}
</section>
