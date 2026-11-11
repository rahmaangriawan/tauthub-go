<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Terima undangan anggota brand. Pratinjau undangan publik; untuk menerima,
	// pengguna harus masuk (atau mendaftar) dengan email yang diundang.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { apiRequest, readApiError, ensureValidSession } from '$lib/auth';
	import AuthShell from '$lib/components/auth/auth-shell.svelte';
	import '$lib/components/auth/auth.css';

	type Preview = { brand_name: string; role: string; email_hint: string; status: string };

	const token = $derived(page.params.token as string);
	const ROLE_LABEL: Record<string, string> = {
		admin: 'Admin', strategist: 'Strategist', writer: 'Writer', finance: 'Finance',
		reviewer: 'Reviewer', viewer: 'Viewer'
	};

	let loading = $state(true);
	let preview = $state<Preview | null>(null);
	let signedIn = $state(false);
	let busy = $state(false);
	let error = $state('');
	let missing = $state(false);

	onMount(() => {
		void (async () => {
			const res = await fetch(`/v1/invitations/${encodeURIComponent(token)}`);
			if (!res.ok) {
				missing = true;
				loading = false;
				return;
			}
			preview = await res.json();
			signedIn = Boolean(await ensureValidSession());
			loading = false;
		})();
	});

	const returnTo = $derived(encodeURIComponent(`/undangan/${token}`));

	async function accept() {
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/invitations/${encodeURIComponent(token)}/accept`, { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, tr('Gagal menerima undangan.'));
				return;
			}
			const j = await res.json();
			await goto(`/klien/${j.brand_id}`);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>{tr("Undangan brand · Tauthub")}</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<AuthShell>
	<div class="inv">
		{#if loading}
			<p class="muted">{tr("Memuat undangan…")}</p>
		{:else if missing || !preview}
			<h1>{tr("Undangan tidak ditemukan")}</h1>
			<p class="muted">{tr("Tautan ini salah atau sudah dicabut. Minta admin brand mengirim undangan baru.")}</p>
		{:else if preview.status !== 'open'}
			<h1>{preview.status === 'accepted' ? tr('Undangan sudah dipakai') : tr('Undangan tidak berlaku')}</h1>
			<p class="muted">
				{preview.status === 'expired'
					? tr('Undangan sudah kedaluwarsa. Minta admin brand mengirim ulang.')
					: preview.status === 'accepted'
						? tr('Undangan ini sudah diterima. Masuk untuk membuka brand.')
						: tr('Undangan ini sudah dicabut oleh admin brand.')}
			</p>
			<a class="btn" href="/login">{tr("Masuk")}</a>
		{:else}
			<h1>{tr("Bergabung ke")} {preview.brand_name}</h1>
			<p class="muted">
				{tr("Anda diundang sebagai")} <strong>{ROLE_LABEL[preview.role] ?? preview.role}</strong>.
				{tr("Undangan ini untuk")} {preview.email_hint}.
			</p>
			{#if error}<p class="err">{error}</p>{/if}
			{#if signedIn}
				<button class="btn" type="button" disabled={busy} onclick={accept}>
					{busy ? tr('Memproses…') : tr('Terima undangan')}
				</button>
				<p class="muted small">{tr("Akun yang sedang masuk harus memakai email yang diundang.")}</p>
			{:else}
				<a class="btn" href={`/login?returnTo=${returnTo}`}>{tr("Masuk untuk menerima")}</a>
				<a class="link" href={`/login?mode=register&returnTo=${returnTo}`}>{tr("Belum punya akun? Daftar dengan email undangan")}</a>
			{/if}
		{/if}
	</div>
</AuthShell>

<style>
	.inv { display: flex; flex-direction: column; gap: 12px; max-width: 420px; margin: 0 auto; padding: 32px 16px; }
	h1 { font-size: 22px; font-weight: 600; margin: 0; }
	.muted { color: var(--muted-foreground); font-size: 14px; margin: 0; }
	.small { font-size: 12px; }
	.err { color: var(--destructive); font-size: 14px; margin: 0; }
	.btn {
		display: inline-flex; align-items: center; justify-content: center; height: 44px; padding: 0 20px;
		border-radius: 10px; background: var(--primary); color: var(--primary-foreground, #fff);
		font-size: 14px; font-weight: 500; border: 0; cursor: pointer; text-decoration: none;
	}
	.btn:disabled { opacity: 0.6; cursor: not-allowed; }
	.link { font-size: 13px; color: var(--primary); text-decoration: underline; }
</style>
