<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Dukungan mitra: daftar tiket + form buat tiket. Detail percakapan ada di
	// /dukungan/[id]. Balasan admin yang belum dibaca tampil sebagai angka di
	// kanan baris dan di badge sidebar.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';
	import PageTitle from '$lib/components/page-title.svelte';
	import Select from '$lib/components/select.svelte';
	import { ChevronRight, LifeBuoy, Plus, X } from '@lucide/svelte';
	import { refreshSupportUnreadNow } from '$lib/partner/support-unread.svelte';

	type Ticket = {
		id: string;
		code: string;
		subject: string;
		category: string;
		priority: string;
		status: string;
		last_message_at: string | null;
		last_message_by: string;
		unread: number;
		created_at: string | null;
	};

	const base = $derived(`/mitra/${page.params.partnerId}`);
	let rows = $state<Ticket[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let busy = $state(false);
	let error = $state('');
	let statusFilter = $state('');
	let showCreate = $state(false);
	let submitting = $state(false);
	let formError = $state('');
	let form = $state({ subject: '', category: 'lainnya', priority: 'normal', body: '' });

	const statusLabels = $derived<Record<string, string>>({
		buka: tr('Buka'),
		diproses: tr('Diproses'),
		selesai: tr('Selesai')
	});
	const categoryLabels = $derived<Record<string, string>>({
		teknis: tr('Teknis'),
		pembayaran: tr('Pembayaran'),
		publikasi: tr('Publikasi'),
		lainnya: tr('Lainnya')
	});
	const priorityLabels = $derived<Record<string, string>>({
		rendah: tr('Rendah'),
		normal: tr('Normal'),
		tinggi: tr('Tinggi')
	});

	const categoryOptions = $derived([
		{ value: 'teknis', label: tr('Teknis') },
		{ value: 'pembayaran', label: tr('Pembayaran') },
		{ value: 'publikasi', label: tr('Publikasi') },
		{ value: 'lainnya', label: tr('Lainnya') }
	]);
	const priorityOptions = $derived([
		{ value: 'rendah', label: tr('Rendah') },
		{ value: 'normal', label: tr('Normal') },
		{ value: 'tinggi', label: tr('Tinggi') }
	]);

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
		busy = true;
		error = '';
		try {
			const params = new URLSearchParams({ limit: '50' });
			if (statusFilter) params.set('status', statusFilter);
			const res = await apiRequest(`/v1/partner/tickets?${params}`);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat tiket.')));
			const data = await res.json();
			rows = (data.tickets ?? []) as Ticket[];
			total = Number(data.total ?? 0);
			await refreshSupportUnreadNow();
		} catch (e) {
			error = e instanceof Error ? e.message : tr('Gagal memuat tiket.');
		} finally {
			busy = false;
		}
	}

	function changeFilter(value: string) {
		if (statusFilter === value) return;
		statusFilter = value;
		void load();
	}

	function openCreate() {
		form = { subject: '', category: 'lainnya', priority: 'normal', body: '' };
		formError = '';
		showCreate = true;
	}

	async function submitTicket() {
		if (submitting) return;
		formError = '';
		if (form.subject.trim().length < 3) {
			formError = tr('Subjek minimal 3 karakter.');
			return;
		}
		if (!form.body.trim()) {
			formError = tr('Isi tiket wajib diisi.');
			return;
		}
		submitting = true;
		try {
			const res = await apiRequest('/v1/partner/tickets', {
				method: 'POST',
				body: JSON.stringify(form)
			});
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal membuat tiket.')));
			const data = await res.json();
			showCreate = false;
			await goto(`${base}/dukungan/${data.ticket.id}`);
		} catch (e) {
			formError = e instanceof Error ? e.message : tr('Gagal membuat tiket.');
		} finally {
			submitting = false;
		}
	}

	function when(iso: string | null): string {
		if (!iso) return '-';
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return '-';
		return d.toLocaleString('id-ID', { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' });
	}

	function statusStyle(status: string): string {
		if (status === 'buka') return 'background-color: color-mix(in srgb, var(--primary) 12%, transparent); color: var(--primary);';
		if (status === 'diproses') return 'background-color: color-mix(in srgb, #d97706 14%, transparent); color: #b45309;';
		return 'background-color: var(--secondary); color: var(--muted-foreground);';
	}
</script>

<PageTitle title={tr('Dukungan')} />

<svelte:window
	onkeydown={(e) => {
		if (e.key === 'Escape' && showCreate) showCreate = false;
	}}
/>

<section>
	<!-- Judul dan tombol berbagi satu baris (juga di HP); keterangan mengambil
	     satu baris penuh di bawahnya. -->
	<header class="flex flex-wrap items-start justify-between gap-x-3 gap-y-2">
		<h1 class="page-title">{tr('Dukungan')}</h1>
		<button class="app-btn app-btn-primary" onclick={openCreate}><Plus size={15} /> {tr('Buat tiket')}</button>
		<p class="page-desc w-full">
			{tr('Buat tiket untuk masalah teknis, pembayaran, atau publikasi. Admin membalas di tiket yang sama.')}
		</p>
	</header>

	{#if error}<div class="app-card mt-3 px-4 py-2.5 text-sm" style="color: var(--destructive);">{error}</div>{/if}

	<div class="mt-4 flex flex-wrap items-center gap-2">
		{#each [['', tr('Semua')], ['buka', tr('Buka')], ['diproses', tr('Diproses')], ['selesai', tr('Selesai')]] as [value, label] (value)}
			<button
				class="app-btn !min-h-8 !px-3 !text-xs"
				class:app-btn-primary={statusFilter === value}
				class:app-btn-soft={statusFilter !== value}
				onclick={() => changeFilter(value)}
			>{label}</button>
		{/each}
		<span class="ml-auto text-xs text-muted-foreground">{total} {tr('tiket')}</span>
	</div>

	<div class="mt-4 space-y-2">
		{#if loading}
			{#each [1, 2, 3] as i (i)}
				<div class="app-card h-[74px] animate-pulse"></div>
			{/each}
		{:else if rows.length === 0}
			<div class="app-card flex flex-col items-center gap-2 px-6 py-12 text-center">
				<LifeBuoy size={26} style="color: var(--muted-foreground);" />
				<p class="text-sm font-medium">{tr('Belum ada tiket.')}</p>
				<p class="text-xs text-muted-foreground">
					{tr('Kalau ada kendala, buat tiket dan admin akan membalas di sini.')}
				</p>
			</div>
		{:else}
			{#each rows as t (t.id)}
				<a
					href={`${base}/dukungan/${t.id}`}
					class="app-card flex items-center gap-3 px-4 py-3 transition-colors hover:bg-[var(--secondary)]"
				>
					<div class="min-w-0 flex-1">
						<div class="flex flex-wrap items-center gap-2">
							<span class="truncate text-sm font-semibold">{t.subject}</span>
							{#if t.unread > 0}
								<span class="inline-flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[10px] font-bold text-white" style="background-color: var(--destructive, #e11d48);">{t.unread}</span>
							{/if}
						</div>
						<p class="mt-1 flex flex-wrap items-center gap-x-2 text-[11px] text-muted-foreground">
							<span>#{t.code}</span>
							<span>·</span>
							<span>{categoryLabels[t.category] ?? t.category}</span>
							<span>·</span>
							<span>{tr('Prioritas')} {priorityLabels[t.priority] ?? t.priority}</span>
							<span>·</span>
							<span>{when(t.last_message_at)}</span>
						</p>
					</div>
					<span class="shrink-0 rounded-full px-2 py-0.5 text-[11px] font-semibold" style={statusStyle(t.status)}>
						{statusLabels[t.status] ?? t.status}
					</span>
					<ChevronRight size={15} class="shrink-0" style="color: var(--muted-foreground);" />
				</a>
			{/each}
		{/if}
	</div>

	{#if busy && !loading}
		<p class="mt-3 text-xs text-muted-foreground">{tr('Memuat…')}</p>
	{/if}
</section>

{#if showCreate}
	<div class="fixed inset-0 z-[1200] flex items-center justify-center p-4">
		<button type="button" class="absolute inset-0" style="background: rgba(15, 23, 42, 0.35);" aria-label={tr('Tutup')} onclick={() => (showCreate = false)}></button>
		<div
			class="relative z-10 max-h-[85vh] w-full max-w-xl overflow-y-auto rounded-xl border p-5 shadow-lg"
			style="border-color: var(--border); background: var(--card, #fff);"
			role="dialog"
			aria-modal="true"
			aria-labelledby="ticket-create-title"
		>
			<div class="flex items-start justify-between gap-3">
				<h2 id="ticket-create-title" class="text-base font-semibold">{tr('Buat tiket')}</h2>
				<button type="button" class="app-btn app-btn-soft !min-h-8 !w-8 !p-0" aria-label={tr('Tutup')} onclick={() => (showCreate = false)}><X size={15} /></button>
			</div>

			<form class="mt-4 space-y-3" onsubmit={(e) => { e.preventDefault(); void submitTicket(); }}>
				<label class="block text-xs font-medium">
					{tr('Subjek')}
					<input class="app-input mt-1 w-full" maxlength="200" bind:value={form.subject} placeholder={tr('Ringkas masalahnya, mis. Invoice belum masuk')} />
				</label>

				<div class="grid gap-3 sm:grid-cols-2">
					<div class="text-xs font-medium">
						<p class="mb-1">{tr('Kategori')}</p>
						<Select bind:value={form.category} options={categoryOptions} ariaLabel={tr('Kategori')} minWidth={220} />
					</div>
					<div class="text-xs font-medium">
						<p class="mb-1">{tr('Prioritas')}</p>
						<Select bind:value={form.priority} options={priorityOptions} ariaLabel={tr('Prioritas')} minWidth={220} />
					</div>
				</div>

				<label class="block text-xs font-medium">
					{tr('Detail')}
					<textarea
						class="app-input mt-1 min-h-32 w-full resize-y"
						maxlength="5000"
						bind:value={form.body}
						placeholder={tr('Jelaskan kronologi, tautan, atau nomor referensi yang relevan.')}
					></textarea>
				</label>

				{#if formError}<p class="text-xs" style="color: var(--destructive);">{formError}</p>{/if}

				<div class="flex justify-end gap-2 pt-1">
					<button type="button" class="app-btn app-btn-soft" onclick={() => (showCreate = false)}>{tr('Batal')}</button>
					<button type="submit" class="app-btn app-btn-primary" disabled={submitting}>
						{submitting ? tr('Mengirim…') : tr('Kirim tiket')}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}
