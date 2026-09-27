<script lang="ts">
	// PR Calculator admin — port app/admin/pr-calculator/page.tsx.
	// Scoring profiles (versioned, bobot wajib 100%), media quality/relevance rules,
	// standar PR value internal, dan maintenance (rerun analisis + recompute skor).
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/admin/page-header.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type ScoringProfile = {
		id: string;
		version: number;
		name: string;
		weights: Record<string, number>;
		thresholds: Record<string, number>;
		status: string;
		created_at: string;
	};

	const INDICATOR_LABEL: Record<string, string> = {
		publication: 'Publication Performance',
		coverage: 'Coverage Quality',
		message: 'Message Consistency',
		sentiment: 'Sentiment / Tone',
		compliance: 'Publication Compliance'
	};
	const INDICATORS = ['publication', 'coverage', 'message', 'sentiment', 'compliance'];

	const PAGE_SIZE = 20;

	let loading = $state(true);
	let error = $state('');
	let notice = $state('');
	let busy = $state(false);
	let profiles = $state<ScoringProfile[]>([]);
	let tierQuality = $state({ tier1: '90', tier2: '80', tier3: '70' });
	let relevanceText = $state('');
	let prRate = $state('5000');
	let prRateTier1 = $state('');
	let prRateTier2 = $state('');
	let prRateTier3 = $state('');
	let newName = $state('');
	let weights = $state<Record<string, string>>({
		publication: '25',
		coverage: '30',
		message: '20',
		sentiment: '15',
		compliance: '10'
	});
	let rerunCampaign = $state('');
	let maintenanceResult = $state('');
	let profilePage = $state(1);

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
		loading = true;
		error = '';
		try {
			const [profilesRes, rulesRes, configRes] = await Promise.all([
				apiRequest('/v1/platform/pr/scoring-profiles'),
				apiRequest('/v1/platform/pr/media-rules'),
				apiRequest('/v1/platform/pr/config')
			]);
			if (!profilesRes.ok) {
				const msg = await readApiError(profilesRes, 'Permintaan platform gagal.');
				error = msg === 'Resource not found' ? 'Akses platform administrator tidak tersedia.' : msg;
				return;
			}
			profiles = (await profilesRes.json()).profiles ?? [];
			if (rulesRes.ok) {
				const rules = await rulesRes.json();
				const tier = rules.tier_default_quality ?? {};
				tierQuality = {
					tier1: String(tier.tier1 ?? 90),
					tier2: String(tier.tier2 ?? 80),
					tier3: String(tier.tier3 ?? 70)
				};
				relevanceText = Object.entries(rules.category_relevance ?? {})
					.map(([k, v]) => `${k}: ${v}`)
					.join('\n');
			}
			if (configRes.ok) {
				const cfg = await configRes.json();
				prRate = String(cfg.rate_per_1k_reach ?? 5000);
				prRateTier1 = cfg.rate_tier1 != null ? String(cfg.rate_tier1) : '';
				prRateTier2 = cfg.rate_tier2 != null ? String(cfg.rate_tier2) : '';
				prRateTier3 = cfg.rate_tier3 != null ? String(cfg.rate_tier3) : '';
			}
		} catch {
			error = 'Tidak dapat terhubung ke server API. Pastikan API aktif lalu coba lagi.';
		} finally {
			loading = false;
		}
	}

	const weightTotal = $derived(INDICATORS.reduce((acc, key) => acc + (Number(weights[key]) || 0), 0));
	const weightsValid = $derived(Math.abs(weightTotal - 100) <= 0.01);

	async function createProfile(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		notice = '';
		try {
			if (!weightsValid) {
				error = `Total bobot harus 100% (sekarang ${Math.round(weightTotal * 100) / 100}%) — profile versi baru tidak dibuat.`;
				return;
			}
			const payload = {
				name: newName.trim(),
				weights: Object.fromEntries(INDICATORS.map((k) => [k, Number(weights[k]) || 0]))
			};
			const res = await apiRequest('/v1/platform/pr/scoring-profiles', { method: 'POST', body: JSON.stringify(payload) });
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			newName = '';
			notice = 'Profile versi baru dibuat (status draft). Aktivasi untuk menjadikannya active.';
			await load();
		} finally {
			busy = false;
		}
	}

	async function activateProfile(profile: ScoringProfile) {
		busy = true;
		error = '';
		notice = '';
		try {
			const res = await apiRequest(`/v1/platform/pr/scoring-profiles/${profile.id}/activate`, { method: 'POST' });
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			notice = `Profile v${profile.version} aktif. Kampanye lama tetap memakai versi skor saat dihitung.`;
			await load();
		} finally {
			busy = false;
		}
	}

	async function savePrConfig(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const tier = (value: string) => (value.trim() === '' ? null : Math.max(0, Number(value) || 0));
			const res = await apiRequest('/v1/platform/pr/config', {
				method: 'PUT',
				body: JSON.stringify({
					rate_per_1k_reach: Math.max(0, Number(prRate) || 0),
					rate_tier1: tier(prRateTier1),
					rate_tier2: tier(prRateTier2),
					rate_tier3: tier(prRateTier3)
				})
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			notice = 'Standar PR value Tauthub tersimpan (global + per tier).';
		} finally {
			busy = false;
		}
	}

	async function saveRules(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		notice = '';
		try {
			const categoryRelevance: Record<string, number> = {};
			for (const line of relevanceText.split('\n')) {
				const separator = line.indexOf(':');
				if (separator <= 0) continue;
				const key = line.slice(0, separator).trim();
				const value = Number(line.slice(separator + 1).trim());
				if (key && Number.isFinite(value)) categoryRelevance[key] = value;
			}
			const res = await apiRequest('/v1/platform/pr/media-rules', {
				method: 'PUT',
				body: JSON.stringify({
					tier_default_quality: {
						tier1: Number(tierQuality.tier1),
						tier2: Number(tierQuality.tier2),
						tier3: Number(tierQuality.tier3)
					},
					category_relevance: categoryRelevance
				})
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			notice = 'Media rules tersimpan.';
		} finally {
			busy = false;
		}
	}

	async function rerunAnalysis() {
		busy = true;
		error = '';
		notice = '';
		maintenanceResult = '';
		try {
			const res = await apiRequest('/v1/platform/pr/rerun-analysis', {
				method: 'POST',
				body: JSON.stringify({ campaign_id: rerunCampaign || null, all: false })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			const data = await res.json();
			maintenanceResult = `Rerun analisis: ${data.queued} publikasi di-queue (pr_analysis + pr_verify).`;
		} finally {
			busy = false;
		}
	}

	async function recomputeScores() {
		busy = true;
		error = '';
		notice = '';
		maintenanceResult = '';
		try {
			const res = await apiRequest('/v1/platform/pr/recompute-scores', {
				method: 'POST',
				body: JSON.stringify({ campaign_id: rerunCampaign || null })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Permintaan platform gagal.');
				return;
			}
			const data = await res.json();
			const summary = (data.results ?? [])
				.slice(0, 5)
				.map((r: { campaign_id: string; score: number | null; status: string }) => `${r.campaign_id.slice(0, 8)}: ${r.score ?? '—'}/${r.status}`)
				.join(' · ');
			maintenanceResult = `Skor dihitung ulang untuk ${data.recalculated} kampanye.${summary ? ` ${summary}${(data.results ?? []).length > 5 ? ' …' : ''}` : ''}`;
		} finally {
			busy = false;
		}
	}

	const visibleProfiles = $derived(profiles.slice((profilePage - 1) * PAGE_SIZE, profilePage * PAGE_SIZE));
	const activeProfile = $derived(profiles.find((p) => p.status === 'active'));
</script>

<section>
	<PageHeader
		title="PR Calculator"
		description="Scoring profiles (versioned), media quality/relevance rules, dan maintenance analisis — semuanya platform-admin only."
	>
		<Badge tone="accent" label={activeProfile ? `Profile v${activeProfile.version} aktif` : 'Belum ada profile aktif'} />
	</PageHeader>

	<Feedback {loading} {error} {notice} loadingText="Memuat PR Calculator…" />

	{#if profiles.length === 0 && error.includes('Akses platform')}
		<div class="app-card mt-4 px-4 py-10 text-center">
			<p class="text-sm text-muted-foreground">Akun ini tidak memiliki akses ke PR Calculator.</p>
		</div>
	{:else}
		<div class="app-card mt-4 p-4">
			<h2 class="text-sm font-semibold">Scoring profiles</h2>
			<p class="mt-0.5 text-xs text-muted-foreground">
				Bobot di versi lama tidak pernah berubah — histori skor kampanye tetap reproducible. Total bobot wajib 100%.
			</p>
			<div class="mt-2 overflow-x-auto">
				<table class="w-full text-sm" style="min-width: 900px;">
					<thead>
						<tr class="text-xs text-muted-foreground">
							<th class="px-3 py-2 text-left font-medium">Versi</th>
							<th class="px-3 py-2 text-left font-medium">Nama</th>
							<th class="px-3 py-2 text-left font-medium">Bobot</th>
							<th class="px-3 py-2 text-left font-medium">Status</th>
							<th class="px-3 py-2 text-left font-medium"></th>
						</tr>
					</thead>
					<tbody>
						{#each visibleProfiles as profile (profile.id)}
							<tr class="border-t">
								<td class="px-3 py-2">v{profile.version}</td>
								<td class="px-3 py-2">{profile.name}</td>
								<td class="px-3 py-2">
									<div class="flex flex-wrap gap-1">
										{#each INDICATORS as key (key)}
											<Badge tone="neutral" label="{key.slice(0, 4)} {profile.weights[key] ?? 0}%" />
										{/each}
									</div>
								</td>
								<td class="px-3 py-2">
									<Badge
										tone={profile.status === 'active' ? 'success' : profile.status === 'draft' ? 'warning' : 'neutral'}
										label={profile.status}
									/>
								</td>
								<td class="px-3 py-2">
									{#if profile.status !== 'active'}
										<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" disabled={busy} onclick={() => void activateProfile(profile)}>Aktifkan</button>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
				{#if profiles.length === 0}
					<p class="px-4 py-8 text-center text-sm text-muted-foreground">Belum ada profile. Buat versi pertama di bawah.</p>
				{/if}
			</div>
			<Pagination total={profiles.length} limit={PAGE_SIZE} offset={(profilePage - 1) * PAGE_SIZE} onChange={(next) => (profilePage = next / PAGE_SIZE + 1)} />

			<form class="mt-4" onsubmit={createProfile}>
				<div class="grid gap-3 md:grid-cols-[1fr_2fr]">
					<label class="block">
						<span class="text-sm font-medium">Nama profile baru</span>
						<input class="app-input mt-1 w-full" required placeholder="Contoh: Baseline v2 — Brand Awareness" bind:value={newName} />
					</label>
					<div class="flex flex-wrap items-end gap-2">
						{#each INDICATORS as key (key)}
							<label class="block w-[150px]">
								<span class="text-xs text-muted-foreground">{INDICATOR_LABEL[key]}</span>
								<input
									class="app-input mt-1 w-full"
									type="number"
									min={0}
									max={100}
									value={weights[key] ?? '0'}
									oninput={(e) => (weights = { ...weights, [key]: (e.currentTarget as HTMLInputElement).value })}
								/>
							</label>
						{/each}
					</div>
				</div>
				<div class="mt-3 flex flex-wrap items-center gap-3">
					<p class="text-sm" style="color: {weightsValid ? 'var(--success)' : 'var(--destructive)'};">
						Total bobot: {Math.round(weightTotal * 100) / 100}% {weightsValid ? '✓' : '(wajib 100%)'}
					</p>
					<button class="app-btn app-btn-primary" type="submit" disabled={busy || !weightsValid || newName.trim().length < 2}>Buat profile versi baru</button>
				</div>
			</form>
		</div>

		<div class="app-card mt-4 p-4">
			<h2 class="text-sm font-semibold">Media quality &amp; relevance rules</h2>
			<p class="mt-0.5 text-xs text-muted-foreground">
				Default kualitas per tier dipakai saat media belum punya media_quality_score sendiri. Relevance per kategori dipakai
				kolom Relevansi di tabel per-media (partial match case-insensitive).
			</p>
			<form class="mt-3" onsubmit={saveRules}>
				<div class="flex flex-wrap gap-3">
					{#each ['tier1', 'tier2', 'tier3'] as tier (tier)}
						<label class="block w-[150px]">
							<span class="text-xs text-muted-foreground">{tier} default quality (0-100)</span>
							<input
								class="app-input mt-1 w-full"
								type="number"
								min={0}
								max={100}
								value={tierQuality[tier as 'tier1' | 'tier2' | 'tier3']}
								oninput={(e) => (tierQuality = { ...tierQuality, [tier]: (e.currentTarget as HTMLInputElement).value })}
							/>
						</label>
					{/each}
				</div>
				<label class="mt-3 block">
					<span class="text-sm font-medium">Category → relevance (satu per baris: Kategori: skor)</span>
					<textarea class="app-input mt-1 w-full" rows="5" placeholder={'Teknologi: 95\nLifestyle: 70\nBisnis: 85'} bind:value={relevanceText}></textarea>
				</label>
				<button class="app-btn app-btn-soft mt-2" type="submit" disabled={busy}>Simpan media rules</button>
			</form>
		</div>

		<div class="app-card mt-4 p-4">
			<h2 class="text-sm font-semibold">PR value standard (internal Tauthub)</h2>
			<p class="mt-0.5 text-xs text-muted-foreground">
				Standarisasi PR untuk semua dashboard klien — klien tidak mengatur ini, mereka hanya menerima angka.
				Estimated PR value = nilai media + Σ(reach tier ÷ 1.000 × rate tier). Rate tier kosong = memakai rate global.
				Diterapkan saat laporan dihitung; angka historis tidak berubah otomatis. Override per brand dikelola di menu
				<strong>Standar PR per Brand</strong>.
			</p>
			<form class="mt-3" onsubmit={savePrConfig}>
				<div class="flex flex-wrap items-end gap-3">
					<label class="block w-[220px]">
						<span class="text-xs text-muted-foreground">Rate global per 1.000 reach (Rp)</span>
						<input class="app-input mt-1 w-full" type="number" min={0} max={10000000} bind:value={prRate} />
					</label>
					<label class="block w-[150px]">
						<span class="text-xs text-muted-foreground">Tier 1 (kosong = global)</span>
						<input class="app-input mt-1 w-full" type="number" min={0} max={10000000} bind:value={prRateTier1} />
					</label>
					<label class="block w-[150px]">
						<span class="text-xs text-muted-foreground">Tier 2 (kosong = global)</span>
						<input class="app-input mt-1 w-full" type="number" min={0} max={10000000} bind:value={prRateTier2} />
					</label>
					<label class="block w-[150px]">
						<span class="text-xs text-muted-foreground">Tier 3 (kosong = global)</span>
						<input class="app-input mt-1 w-full" type="number" min={0} max={10000000} bind:value={prRateTier3} />
					</label>
					<button class="app-btn app-btn-soft" type="submit" disabled={busy}>Simpan standar PR</button>
				</div>
			</form>
		</div>

		<div class="app-card mt-4 p-4">
			<h2 class="text-sm font-semibold">Maintenance</h2>
			<p class="mt-0.5 text-xs text-muted-foreground">
				Jalankan ulang analisis (message + sentiment + verifikasi URL) dan hitung ulang skor untuk publikasi published
				yang sudah ada sebelum fitur ini.
			</p>
			<div class="mt-3 flex flex-wrap items-center gap-3">
				<label class="block min-w-[260px]">
					<span class="text-xs text-muted-foreground">Campaign ID (opsional — kosongkan untuk semua)</span>
					<input class="app-input mt-1 w-full" placeholder="uuid kampanye" bind:value={rerunCampaign} />
				</label>
				<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => void rerunAnalysis()}>Rerun analisis (yang belum)</button>
				<button class="app-btn app-btn-soft" type="button" disabled={busy} onclick={() => void recomputeScores()}>Hitung ulang skor</button>
			</div>
			{#if maintenanceResult}<p class="mt-3 text-sm" style="color: var(--success);">{maintenanceResult}</p>{/if}
			<p class="mt-3 text-xs text-muted-foreground">
				Catatan: job analisis/verifikasi diproses oleh queue worker di background. Skor direkalkulasi otomatis setelah tiap job selesai.
			</p>
		</div>
	{/if}
</section>
