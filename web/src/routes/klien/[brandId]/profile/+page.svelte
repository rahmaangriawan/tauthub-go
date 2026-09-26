<script lang="ts">
	// Fase 2: profil brand — profile/voice/rules + aktivasi (readiness gate).
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const brandId = $derived(page.params.brandId as string);

	let profile = $state({ positioning: '', value_proposition: '', target_audience: '' });
	let rules = $state({ required_rules: '', forbidden_rules: '', required_disclaimer: '', citation_policy: '' });
	let score = $state<number | null>(null);
	let status = $state('');
	let busy = $state(false);
	let msg = $state('');
	let error = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			await load();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function load() {
		const res = await apiRequest(`/v1/brands/${brandId}/configuration`);
		if (!res.ok) return;
		const data = await res.json();
		if (data.profile) {
			profile = {
				positioning: data.profile.positioning ?? '',
				value_proposition: data.profile.value_proposition ?? '',
				target_audience: data.profile.target_audience ?? ''
			};
		}
		if (data.editorial_rules) {
			const rulesData = data.editorial_rules;
			rules = {
				required_rules: (rulesData.required_rules ?? []).join(', '),
				forbidden_rules: (rulesData.forbidden_rules ?? []).join(', '),
				required_disclaimer: rulesData.required_disclaimer ?? '',
				citation_policy: rulesData.citation_policy ?? ''
			};
		}
		const detail = await apiRequest(`/v1/brands/${brandId}`);
		if (detail.ok) {
			const d = await detail.json();
			score = d.readiness_score;
			status = d.status;
		}
	}

	async function saveProfile(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/profile`, {
				method: 'PUT',
				body: JSON.stringify({
					positioning: profile.positioning || null,
					value_proposition: profile.value_proposition || null,
					target_audience: profile.target_audience || null
				})
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal menyimpan profil.');
				return;
			}
			msg = 'Profil tersimpan.';
			await load();
		} finally {
			busy = false;
		}
	}

	async function saveRules(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/editorial-rules`, {
				method: 'PUT',
				body: JSON.stringify({
					required_rules: rules.required_rules.split(',').map((s: string) => s.trim()).filter(Boolean),
					forbidden_rules: rules.forbidden_rules.split(',').map((s: string) => s.trim()).filter(Boolean),
					required_disclaimer: rules.required_disclaimer,
					citation_policy: rules.citation_policy
				})
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal menyimpan aturan.');
				return;
			}
			msg = 'Aturan editorial tersimpan.';
			await load();
		} finally {
			busy = false;
		}
	}

	async function activate() {
		busy = true;
		error = '';
		msg = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/activate`, { method: 'POST' });
			const data = await res.json();
			if (res.ok) {
				status = 'active';
				score = data.readiness_score;
				msg = 'Brand diaktifkan.';
			} else if (typeof data.detail === 'object') {
				score = data.detail.score;
				error = `${data.detail.message} (skor saat ini ${data.detail.score}).`;
			} else {
				error = typeof data.detail === 'string' ? data.detail : 'Gagal mengaktifkan.';
			}
		} finally {
			busy = false;
		}
	}
</script>

<main class="mx-auto max-w-3xl px-4 py-8">
	<a class="text-sm text-muted-foreground underline" href={`/klien/${brandId}`}>← Dashboard</a>
	<div class="mt-2 flex items-center justify-between">
		<h1 class="text-2xl font-semibold tracking-tight">Profil Brand</h1>
		<div class="flex items-center gap-3">
			{#if score !== null}
				<span class="text-sm text-muted-foreground">readiness {score}%</span>
				<span class="rounded-full px-2 py-0.5 text-xs {status === 'active' ? 'bg-emerald-100 text-emerald-700' : 'bg-muted text-muted-foreground'}">{status}</span>
			{/if}
			<button class="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50" disabled={busy} onclick={activate}>
				Aktifkan brand
			</button>
		</div>
	</div>
	{#if msg}<p class="mt-2 text-sm text-emerald-600">{msg}</p>{/if}
	{#if error}<p class="mt-2 text-sm text-destructive">{error}</p>{/if}

	<form class="mt-6 rounded-lg border border-border bg-card p-4" onsubmit={saveProfile}>
		<h2 class="font-medium">Profil</h2>
		<div class="mt-3 grid gap-3">
			<label class="flex flex-col gap-1 text-sm">Positioning
				<input class="rounded-md border border-input bg-background px-3 py-2 text-sm" bind:value={profile.positioning} />
			</label>
			<label class="flex flex-col gap-1 text-sm">Value proposition
				<input class="rounded-md border border-input bg-background px-3 py-2 text-sm" bind:value={profile.value_proposition} />
			</label>
			<label class="flex flex-col gap-1 text-sm">Target audiens
				<input class="rounded-md border border-input bg-background px-3 py-2 text-sm" bind:value={profile.target_audience} />
			</label>
		</div>
		<button class="mt-3 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground" type="submit" disabled={busy}>Simpan profil</button>
	</form>

	<form class="mt-4 rounded-lg border border-border bg-card p-4" onsubmit={saveRules}>
		<h2 class="font-medium">Aturan editorial</h2>
		<div class="mt-3 grid gap-3">
			<label class="flex flex-col gap-1 text-sm">Aturan wajib (pisah koma)
				<input class="rounded-md border border-input bg-background px-3 py-2 text-sm" bind:value={rules.required_rules} />
			</label>
			<label class="flex flex-col gap-1 text-sm">Aturan terlarang (pisah koma)
				<input class="rounded-md border border-input bg-background px-3 py-2 text-sm" bind:value={rules.forbidden_rules} />
			</label>
			<label class="flex flex-col gap-1 text-sm">Disclaimer wajib
				<input class="rounded-md border border-input bg-background px-3 py-2 text-sm" bind:value={rules.required_disclaimer} />
			</label>
			<label class="flex flex-col gap-1 text-sm">Kebijakan sitasi
				<input class="rounded-md border border-input bg-background px-3 py-2 text-sm" bind:value={rules.citation_policy} />
			</label>
		</div>
		<button class="mt-3 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground" type="submit" disabled={busy}>Simpan aturan</button>
	</form>
</main>
