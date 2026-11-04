<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Domain email yang diblokir dari pendaftaran (tab "Domain Email" di
	// Konfigurasi): daftar tambahan kelolaan admin, batas akun Google/Workspace
	// per domain per hari, dan daftar temp-mail bawaan (hanya baca, bisa dicari).
	import { onMount } from 'svelte';
	import { Search } from '@lucide/svelte';
	import Select from '$lib/components/select.svelte';
	import Pagination from '$lib/components/admin/pagination.svelte';
	import { apiRequest, ensureValidSession } from '$lib/auth';
	import { fmtDate, fmtNum } from '$lib/admin-format';
	import { platformError } from '../_platform';

	type BlockedDomain = { domain: string; reason: string; created_at: string | null };

	const BUILTIN_PAGE = 100;

	let domains = $state<BlockedDomain[]>([]);
	let builtinCount = $state(0);
	let domainPerDay = $state(3);
	let domainPerDayInput = $state('3');
	let error = $state('');
	let notice = $state('');
	let newDomain = $state('');
	let newDomainReason = $state('manual');
	let busy = $state(false);
	let loaded = $state(false);

	// Daftar bawaan: dicari di server, satu halaman per permintaan.
	let builtin = $state<string[]>([]);
	let builtinTotal = $state(0);
	let builtinOffset = $state(0);
	let builtinQuery = $state('');
	let builtinLoading = $state(false);
	let searchTimer: ReturnType<typeof setTimeout> | null = null;

	async function load() {
		error = '';
		const res = await apiRequest('/v1/platform/operations/blocked-domains');
		if (!res.ok) {
			error = await platformError(res);
			loaded = true;
			return;
		}
		const data = (await res.json()) as { items?: BlockedDomain[]; builtin_count?: number; domain_per_day?: number };
		domains = data.items ?? [];
		builtinCount = data.builtin_count ?? 0;
		domainPerDay = data.domain_per_day ?? 3;
		domainPerDayInput = String(domainPerDay);
		loaded = true;
	}

	async function loadBuiltin() {
		builtinLoading = true;
		const params = new URLSearchParams({ limit: String(BUILTIN_PAGE), offset: String(builtinOffset) });
		if (builtinQuery.trim()) params.set('q', builtinQuery.trim());
		const res = await apiRequest(`/v1/platform/operations/blocked-domains/builtin?${params}`);
		if (res.ok) {
			const data = (await res.json()) as { items?: string[]; total?: number };
			builtin = data.items ?? [];
			builtinTotal = data.total ?? 0;
		} else {
			error = await platformError(res);
		}
		builtinLoading = false;
	}

	function onBuiltinSearch(value: string) {
		builtinQuery = value;
		builtinOffset = 0;
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => void loadBuiltin(), 300);
	}

	async function addDomain(e: SubmitEvent) {
		e.preventDefault();
		if (busy || !newDomain.trim()) return;
		busy = true;
		error = '';
		notice = '';
		const res = await apiRequest('/v1/platform/operations/blocked-domains', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ domain: newDomain.trim(), reason: newDomainReason })
		});
		if (res.ok) {
			newDomain = '';
			await load();
			notice = tr('Domain ditambahkan ke daftar blokir.');
		} else {
			error = await platformError(res);
		}
		busy = false;
	}

	async function removeDomain(domain: string) {
		busy = true;
		error = '';
		notice = '';
		const res = await apiRequest('/v1/platform/operations/blocked-domains', {
			method: 'DELETE',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ domain })
		});
		if (res.ok) domains = domains.filter((d) => d.domain !== domain);
		else error = await platformError(res);
		busy = false;
	}

	async function saveDomainPerDay() {
		const n = Number(domainPerDayInput);
		if (!Number.isInteger(n) || n < 1 || n > 50) {
			error = tr('Batas per domain harus bilangan bulat 1–50.');
			return;
		}
		busy = true;
		error = '';
		notice = '';
		const res = await apiRequest('/v1/platform/operations/registration-guard', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ domain_per_day: n })
		});
		if (res.ok) {
			domainPerDay = n;
			notice = tr('Batas per domain disimpan.');
		} else {
			error = await platformError(res);
		}
		busy = false;
	}

	onMount(() => {
		void (async () => {
			if (!(await ensureValidSession())) return;
			await Promise.all([load(), loadBuiltin()]);
		})();
		return () => {
			if (searchTimer) clearTimeout(searchTimer);
		};
	});
</script>

<div class="space-y-4">
	<div class="app-card p-5">
		<div class="flex flex-wrap items-start justify-between gap-3">
			<div>
				<h2 class="text-sm font-medium">{tr('Batas akun per domain')}</h2>
				<p class="mt-0.5 max-w-xl text-xs text-muted-foreground">
					{tr('Pendaftaran dari satu domain non-Gmail (mis. Google Workspace) dibatasi per hari. Lewat batas, pendaftaran baru dari domain itu ditolak sampai besok atau batas dinaikkan.')}
				</p>
			</div>
			<form class="flex items-center gap-2" onsubmit={(e) => { e.preventDefault(); void saveDomainPerDay(); }}>
				<label class="text-xs text-muted-foreground" for="domain-per-day">{tr('Batas akun Google per domain/hari')}</label>
				<input id="domain-per-day" class="app-input !min-h-8 w-16 !px-2 !py-1 text-center text-xs tabular-nums" type="number" min="1" max="50" bind:value={domainPerDayInput} />
				<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="submit" disabled={busy || !loaded || domainPerDayInput === String(domainPerDay)}>{tr('Simpan')}</button>
			</form>
		</div>
	</div>

	<div class="app-card overflow-x-auto">
		<div class="border-b px-5 py-4" style="border-color: var(--border);">
			<h2 class="text-sm font-medium">{tr('Domain email diblokir')}</h2>
			<p class="mt-0.5 text-xs text-muted-foreground">
				{tr('Domain tambahan dari admin. Pendaftaran email dan Google dari domain ini (dan subdomainnya) ditolak, selain daftar bawaan di bawah.')}
			</p>
		</div>
		<form class="flex flex-wrap items-center gap-2 px-5 py-3" onsubmit={addDomain}>
			<input class="app-input !min-h-8 min-w-[220px] flex-1 !py-1 text-sm" type="text" placeholder="tempmail.io" bind:value={newDomain} aria-label={tr('Domain yang diblokir')} />
			<Select ariaLabel={tr('Alasan')} minWidth={140} value={newDomainReason} onchange={(v) => (newDomainReason = v)}
				options={[{ value: 'manual', label: tr('Manual') }, { value: 'disposable', label: 'Temp-mail' }, { value: 'abuse', label: tr('Penyalahgunaan') }]} />
			<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" type="submit" disabled={busy || !newDomain.trim()}>{tr('Tambah')}</button>
		</form>
		{#if error}<p class="px-5 pb-2 text-xs" style="color: var(--destructive);">{error}</p>{/if}
		{#if notice}<p class="px-5 pb-2 text-xs" style="color: var(--success, #15803d);">{notice}</p>{/if}
		{#if domains.length > 0}
			<table class="admin-table w-full text-sm">
				<thead>
					<tr class="text-left text-xs text-muted-foreground">
						<th class="px-5 py-2.5 font-medium">Domain</th>
						<th class="px-5 py-2.5 font-medium">{tr('Alasan')}</th>
						<th class="px-5 py-2.5 font-medium">{tr('Ditambahkan')}</th>
						<th class="px-5 py-2.5 text-right font-medium">Aksi</th>
					</tr>
				</thead>
				<tbody>
					{#each domains as d (d.domain)}
						<tr class="border-t">
							<td class="px-5 py-2.5 font-mono text-xs">{d.domain}</td>
							<td class="px-5 py-2.5 text-xs text-muted-foreground">{d.reason}</td>
							<td class="px-5 py-2.5 text-xs text-muted-foreground">{d.created_at ? fmtDate(d.created_at) : '—'}</td>
							<td class="px-5 py-2.5 text-right">
								<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={busy} onclick={() => void removeDomain(d.domain)}>{tr('Hapus')}</button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{:else if loaded}
			<p class="px-5 pb-4 text-xs text-muted-foreground">{tr('Belum ada domain tambahan dari admin.')}</p>
		{/if}
	</div>

	<div class="app-card overflow-x-auto">
		<div class="flex flex-wrap items-center justify-between gap-3 border-b px-5 py-4" style="border-color: var(--border);">
			<div>
				<h2 class="text-sm font-medium">{tr('Daftar temp-mail bawaan')}</h2>
				<p class="mt-0.5 text-xs text-muted-foreground">
					<span class="tabular-nums">{fmtNum(builtinCount)}</span> {tr('domain email sementara yang selalu ditolak. Hanya baca; untuk menambah gunakan form di atas.')}
				</p>
			</div>
			<label class="app-input flex min-w-[240px] items-center gap-2 !py-1.5">
				<Search size={15} class="text-muted-foreground" />
				<input
					class="w-full bg-transparent text-sm outline-none"
					placeholder={tr('Cari domain…')}
					aria-label={tr('Cari domain temp-mail')}
					value={builtinQuery}
					oninput={(e) => onBuiltinSearch((e.currentTarget as HTMLInputElement).value)}
				/>
			</label>
		</div>
		{#if builtin.length > 0}
			<ul class="grid grid-cols-1 gap-x-6 px-5 py-3 text-xs sm:grid-cols-2 lg:grid-cols-3" class:opacity-60={builtinLoading}>
				{#each builtin as d (d)}
					<li class="truncate border-b py-1.5 font-mono" style="border-color: var(--border);" title={d}>{d}</li>
				{/each}
			</ul>
		{:else if !builtinLoading}
			<p class="px-5 py-4 text-xs text-muted-foreground">{tr('Tidak ada domain yang cocok.')}</p>
		{/if}
		<div class="px-5 pb-4">
			<Pagination
				total={builtinTotal}
				limit={BUILTIN_PAGE}
				offset={builtinOffset}
				onChange={(next) => {
					builtinOffset = next;
					void loadBuiltin();
				}}
			/>
		</div>
	</div>
</div>
