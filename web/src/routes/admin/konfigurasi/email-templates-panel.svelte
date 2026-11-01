<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Panel Template untuk tab Email/SMTP di /admin/konfigurasi.
	// Dua audiens dipisah tegas (Klien / Mitra); setiap event punya subjek,
	// body HTML, dan saklar kirim email. Gaya email berlaku untuk semua.
	import { onMount } from 'svelte';
	import { Eye, MailCheck, RefreshCw, Save } from '@lucide/svelte';
	import Badge from '$lib/components/admin/badge.svelte';
	import Feedback from '$lib/components/admin/feedback.svelte';
	import Modal from '$lib/components/admin/modal.svelte';
	import { apiRequest, readApiError, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	type EmailTemplateItem = {
		audience: 'client' | 'partner';
		event: string;
		label: string;
		sender: 'auth' | 'direct' | 'generic';
		variables: string[];
		subject: string;
		body_html: string;
		default_subject: string;
		default_body: string;
		enabled: boolean;
		customized: boolean;
		channel_enabled: boolean;
		updated_at: string | null;
	};

	type EmailStyle = {
		brand_name: string;
		logo_url: string;
		primary_color: string;
		accent_color: string;
		page_bg: string;
		card_bg: string;
		text_color: string;
		muted_color: string;
		font_stack: string;
		button_radius: number;
		footer_text: string;
		support_email: string;
	};

	const FONT_OPTIONS = [
		"Arial,Helvetica,'Segoe UI',sans-serif",
		'Arial,Helvetica,sans-serif',
		'Verdana,Geneva,sans-serif',
		'Tahoma,Verdana,sans-serif',
		"Georgia,'Times New Roman',serif",
		"'Courier New',Courier,monospace"
	];

	let items = $state<EmailTemplateItem[]>([]);
	let style = $state<EmailStyle | null>(null);
	let styleDraft = $state<EmailStyle | null>(null);
	let audience = $state<'client' | 'partner'>('client');
	let loading = $state(true);
	let error = $state('');
	let notice = $state('');
	let styleBusy = $state(false);
	let channelBusy = $state<string | null>(null);
	let bulkBusy = $state(false);

	let editing = $state<EmailTemplateItem | null>(null);
	let draftSubject = $state('');
	let draftBody = $state('');
	let editorBusy = $state(false);
	let editorError = $state('');
	let recipient = $state('');
	let testBusy = $state(false);
	let testMsg = $state({ text: '', ok: false });
	let previewHtml = $state('');
	let previewBusy = $state(false);
	let previewTimer: ReturnType<typeof setTimeout> | undefined;

	let ready = $state(false);
	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
			ready = true;
			await load();
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	const visibleItems = $derived(items.filter((i) => i.audience === audience));
	/** True bila seluruh template audiens aktif memakai email. */
	const allChannelsOn = $derived(visibleItems.length > 0 && visibleItems.every((i) => i.channel_enabled));

	async function load() {
		error = '';
		loading = true;
		try {
			const res = await apiRequest('/v1/platform/email/templates');
			if (!res.ok) {
				error = await readApiError(res, 'Gagal memuat template email.');
				return;
			}
			const data = (await res.json()) as { items?: EmailTemplateItem[]; style?: EmailStyle };
			items = data.items ?? [];
			style = data.style ?? null;
			styleDraft = style ? { ...style } : null;
		} catch {
			error = tr('Tidak dapat terhubung ke server API.');
		} finally {
			loading = false;
		}
	}

	async function saveStyle() {
		if (!styleDraft) return;
		styleBusy = true;
		notice = '';
		try {
			const res = await apiRequest('/v1/platform/email/style', {
				method: 'PUT',
				body: JSON.stringify(styleDraft)
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal menyimpan gaya email.');
				return;
			}
			style = (await res.json()) as EmailStyle;
			styleDraft = { ...style };
			error = '';
			notice = tr('Gaya email disimpan.');
		} finally {
			styleBusy = false;
		}
	}

	function openEditor(item: EmailTemplateItem) {
		editing = item;
		draftSubject = item.subject;
		draftBody = item.body_html;
		editorError = '';
		testMsg = { text: '', ok: false };
		recipient = '';
		schedulePreview();
	}

	function insertVariable(v: string) {
		draftBody = draftBody + v;
		schedulePreview();
	}

	function schedulePreview() {
		if (previewTimer) clearTimeout(previewTimer);
		previewTimer = setTimeout(() => void refreshPreview(), 500);
	}

	async function refreshPreview() {
		if (!editing) return;
		previewBusy = true;
		try {
			const res = await apiRequest('/v1/platform/email/preview', {
				method: 'POST',
				body: JSON.stringify({
					audience: editing.audience,
					event: editing.event,
					subject: draftSubject,
					body_html: draftBody,
					style: styleDraft
				})
			});
			if (!res.ok) return;
			const data = (await res.json()) as { html?: string };
			previewHtml = data.html ?? '';
		} finally {
			previewBusy = false;
		}
	}

	async function saveTemplate() {
		if (!editing) return;
		editorBusy = true;
		editorError = '';
		try {
			const res = await apiRequest('/v1/platform/email/templates', {
				method: 'PUT',
				body: JSON.stringify({
					audience: editing.audience,
					event: editing.event,
					subject: draftSubject,
					body_html: draftBody,
					enabled: editing.enabled
				})
			});
			if (!res.ok) {
				editorError = await readApiError(res, 'Gagal menyimpan template.');
				return;
			}
			editing = null;
			notice = tr('Template disimpan.');
			await load();
		} finally {
			editorBusy = false;
		}
	}

	async function resetTemplate(item: EmailTemplateItem) {
		if (!confirm(`Kembalikan template "${item.label}" ke bawaan?`)) return;
		const res = await apiRequest(
			`/v1/platform/email/templates?audience=${encodeURIComponent(item.audience)}&event=${encodeURIComponent(item.event)}`,
			{ method: 'DELETE' }
		);
		if (!res.ok) {
			error = await readApiError(res, 'Gagal mengembalikan template.');
			return;
		}
		notice = tr('Template dikembalikan ke bawaan.');
		await load();
	}

	async function toggleChannel(item: EmailTemplateItem) {
		const next = !item.channel_enabled;
		if (
			next &&
			!confirm(
				`Aktifkan pengiriman email untuk "${item.label}"?\n\n` +
					'Email hanya dikirim untuk notifikasi BARU setelah ini, mengikuti preferensi email pengguna dan kuota.'
			)
		)
			return;
		channelBusy = item.event;
		try {
			const res = await apiRequest('/v1/platform/email/channels', {
				method: 'PUT',
				body: JSON.stringify({ event: item.event, enabled: next })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal mengubah kanal email.');
				return;
			}
			items = items.map((i) => (i.event === item.event ? { ...i, channel_enabled: next } : i));
		} finally {
			channelBusy = null;
		}
	}

	async function toggleAllChannels() {
		const next = !allChannelsOn;
		const audienceLabel = audience === 'client' ? tr('Klien') : tr('Mitra');
		if (
			!confirm(
				(next
					? `Aktifkan email untuk semua ${visibleItems.length} template ${audienceLabel}?`
					: `Matikan email untuk semua ${visibleItems.length} template ${audienceLabel}?`) +
					'\n\nEmail hanya dikirim untuk notifikasi BARU setelah ini, mengikuti preferensi email pengguna dan kuota.'
			)
		)
			return;
		bulkBusy = true;
		error = '';
		try {
			const res = await apiRequest('/v1/platform/email/channels', {
				method: 'PUT',
				body: JSON.stringify({ audience, all: true, enabled: next })
			});
			if (!res.ok) {
				error = await readApiError(res, 'Gagal mengubah kanal email.');
				return;
			}
			notice = next ? tr('Semua kanal email diaktifkan.') : tr('Semua kanal email dimatikan.');
			await load();
		} finally {
			bulkBusy = false;
		}
	}

	async function sendTest() {
		if (!editing) return;
		if (!recipient.trim()) {
			testMsg = { text: 'Isi alamat email tujuan.', ok: false };
			return;
		}
		testBusy = true;
		testMsg = { text: '', ok: false };
		try {
			const res = await apiRequest('/v1/platform/email/test-send', {
				method: 'POST',
				body: JSON.stringify({
					audience: editing.audience,
					event: editing.event,
					recipient_email: recipient.trim()
				})
			});
			const data = (await res.json().catch(() => ({}))) as { detail?: string; ok?: boolean };
			if (!res.ok) {
				testMsg = { text: data.detail ?? 'Gagal mengirim email uji.', ok: false };
				return;
			}
			testMsg = { text: data.detail ?? 'Email uji terkirim.', ok: true };
		} finally {
			testBusy = false;
		}
	}

	const badgeTone = (item: EmailTemplateItem) => (item.customized ? 'accent' : 'neutral');
</script>

<div class="space-y-3">
	{#if error}<Feedback error={error} />{/if}
	{#if notice}
		<div class="app-card px-4 py-2.5 text-sm" style="color: var(--success);">{notice}</div>
	{/if}

	<!-- Gaya email -->
	<section class="app-card p-4">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div>
				<h2 class="text-base font-semibold">{tr('Gaya email')}</h2>
				<p class="mt-1 text-xs text-muted-foreground">
					{tr('Berlaku untuk semua email keluar. Perubahan tersimpan ke server dan dipakai pengirim dalam ±1 menit.')}
				</p>
			</div>
			<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" type="button" disabled={styleBusy || !styleDraft} onclick={() => void saveStyle()}>
				<Save size={14} /> {styleBusy ? tr('Menyimpan…') : tr('Simpan gaya')}
			</button>
		</div>

		{#if styleDraft}
			<div class="mt-3 grid gap-3 sm:grid-cols-2">
				<label class="block">
					<span class="text-sm font-medium">{tr('Nama brand')}</span>
					<input class="app-input mt-1 w-full" bind:value={styleDraft.brand_name} />
				</label>
				<label class="block">
					<span class="text-sm font-medium">{tr('URL logo')}</span>
					<input class="app-input mt-1 w-full" placeholder="https://…" bind:value={styleDraft.logo_url} />
				</label>
				{#each [
					{ key: 'primary_color', label: tr('Warna primer') },
					{ key: 'accent_color', label: tr('Warna tombol/aksen') },
					{ key: 'page_bg', label: tr('Latar halaman') },
					{ key: 'card_bg', label: tr('Latar kartu') },
					{ key: 'text_color', label: tr('Warna teks') },
					{ key: 'muted_color', label: tr('Warna teks sekunder') }
				] as c (c.key)}
					<label class="block">
						<span class="text-sm font-medium">{c.label}</span>
						<span class="mt-1 flex items-center gap-2">
							<input
								type="color"
								class="h-9 w-12 cursor-pointer rounded border"
								style="border-color: var(--border);"
								value={styleDraft[c.key as keyof EmailStyle] as string}
								oninput={(e) => {
									(styleDraft as Record<string, unknown>)[c.key] = (e.currentTarget as HTMLInputElement).value;
								}}
							/>
							<input
								class="app-input w-full font-mono text-xs"
								value={styleDraft[c.key as keyof EmailStyle] as string}
								oninput={(e) => {
									(styleDraft as Record<string, unknown>)[c.key] = (e.currentTarget as HTMLInputElement).value;
								}}
							/>
						</span>
					</label>
				{/each}
				<label class="block">
					<span class="text-sm font-medium">{tr('Font email')}</span>
					<select class="app-input mt-1 w-full" bind:value={styleDraft.font_stack}>
						{#each FONT_OPTIONS as f (f)}<option value={f}>{f}</option>{/each}
					</select>
				</label>
				<label class="block">
					<span class="text-sm font-medium">{tr('Radius tombol (0-24)')}</span>
					<input class="app-input mt-1 w-full" type="number" min="0" max="24" bind:value={styleDraft.button_radius} />
				</label>
				<label class="block sm:col-span-2">
					<span class="text-sm font-medium">{tr('Teks footer')}</span>
					<input class="app-input mt-1 w-full" bind:value={styleDraft.footer_text} />
				</label>
				<label class="block sm:col-span-2">
					<span class="text-sm font-medium">{tr('Email dukungan')}</span>
					<input class="app-input mt-1 w-full" type="email" bind:value={styleDraft.support_email} />
				</label>
			</div>
		{:else if loading}
			<p class="mt-3 text-sm text-muted-foreground">{tr('Memuat gaya email…')}</p>
		{/if}
	</section>

	<!-- Daftar template per audiens -->
	<section class="app-card p-4">
		<div class="flex flex-wrap items-center gap-2">
			<h2 class="text-base font-semibold">{tr('Template email')}</h2>
			<div class="flex-1"></div>
			<button class="app-btn !min-h-8 !px-3 !text-xs" class:app-btn-primary={audience === 'client'} class:app-btn-soft={audience !== 'client'} type="button" onclick={() => (audience = 'client')}>
				{tr('Klien')}
			</button>
			<button class="app-btn !min-h-8 !px-3 !text-xs" class:app-btn-primary={audience === 'partner'} class:app-btn-soft={audience !== 'partner'} type="button" onclick={() => (audience = 'partner')}>
				{tr('Mitra')}
			</button>
			<button
				class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs"
				type="button"
				disabled={bulkBusy || loading || visibleItems.length === 0}
				onclick={() => void toggleAllChannels()}
				title={allChannelsOn ? tr('Matikan email untuk semua template audiens ini.') : tr('Aktifkan email untuk semua template audiens ini.')}
			>
				<MailCheck size={14} />
				{bulkBusy ? tr('Memproses…') : allChannelsOn ? tr('Matikan semua') : tr('Aktifkan semua')}
			</button>
			<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={loading} onclick={() => void load()}>
				<RefreshCw size={14} class={loading ? 'animate-spin' : ''} />
			</button>
		</div>
		<p class="mt-1 text-xs text-muted-foreground">
			{audience === 'client' ? tr('Template yang dikirim ke akun klien.') : tr('Template yang dikirim ke akun mitra.')}
			{' '}{tr('Event bertanda "langsung" punya pengirim sendiri; mengaktifkan kanal cukup menyalakan email-nya.')}
		</p>

		{#if loading && items.length === 0}
			<p class="mt-4 text-sm text-muted-foreground">{tr('Memuat template…')}</p>
		{:else if visibleItems.length === 0}
			<p class="mt-4 text-sm text-muted-foreground">{tr('Belum ada template untuk audiens ini.')}</p>
		{:else}
			<div class="mt-3 divide-y">
				{#each visibleItems as item (item.audience + ':' + item.event)}
					<div class="flex flex-wrap items-start gap-3 py-3">
						<div class="min-w-0 flex-1">
							<div class="flex flex-wrap items-center gap-2">
								<p class="text-sm font-medium">{item.label}</p>
								<Badge tone={badgeTone(item)} label={item.customized ? tr('Dikustom') : tr('Bawaan')} />
								<Badge tone={item.channel_enabled ? 'success' : 'neutral'} label={item.channel_enabled ? tr('Email aktif') : tr('Email nonaktif')} />
								<Badge tone="neutral" label={item.sender === 'direct' ? tr('langsung') : item.sender === 'auth' ? tr('akun') : tr('notifikasi')} />
							</div>
							<p class="mt-1 truncate text-xs text-muted-foreground">{item.event}</p>
							<p class="mt-1 truncate text-xs">{item.subject}</p>
						</div>
						<div class="flex flex-wrap items-center gap-2">
							<label class="flex items-center gap-1 text-xs text-muted-foreground" title={tr('Kirim email untuk event ini')}>
								<input type="checkbox" checked={item.channel_enabled} disabled={channelBusy === item.event} onchange={() => void toggleChannel(item)} />
								{tr('Aktif')}
							</label>
							<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => openEditor(item)}>{tr('Edit')}</button>
							{#if item.customized}
								<button class="app-btn app-btn-soft !min-h-7 !px-2.5 !text-xs" type="button" onclick={() => void resetTemplate(item)}>{tr('Reset')}</button>
							{/if}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</section>
</div>

{#if editing}
	<Modal title={`Template — ${editing.label}`} maxWidth="4xl" onClose={() => (editing = null)}>
		<div class="grid gap-4 lg:grid-cols-2">
			<div class="space-y-3">
				<div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
					<Badge tone={editing.audience === 'client' ? 'accent' : 'neutral'} label={editing.audience === 'client' ? tr('Klien') : tr('Mitra')} />
					<span>{editing.event}</span>
				</div>
				<label class="block">
					<span class="text-sm font-medium">{tr('Subjek')}</span>
					<input class="app-input mt-1 w-full" maxlength="500" bind:value={draftSubject} oninput={schedulePreview} />
				</label>
				<label class="block">
					<span class="text-sm font-medium">{tr('Isi email (HTML)')}</span>
					<textarea class="app-input mt-1 min-h-[260px] w-full font-mono text-xs" maxlength="20000" bind:value={draftBody} oninput={schedulePreview}></textarea>
				</label>
				<div>
					<p class="text-xs font-medium text-muted-foreground">{tr('Sisipkan variabel')}</p>
					<div class="mt-1 flex flex-wrap gap-1.5">
						{#each editing.variables as v (v)}
							<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={() => insertVariable(v)}>{v}</button>
						{/each}
					</div>
				</div>
				<div class="flex flex-wrap items-end gap-2">
					<label class="block min-w-[200px] flex-1">
						<span class="text-xs font-medium text-muted-foreground">{tr('Kirim email uji ke')}</span>
						<input class="app-input mt-1 w-full" type="email" placeholder="admin@example.com" bind:value={recipient} />
					</label>
					<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" disabled={testBusy} onclick={() => void sendTest()}>
						{testBusy ? tr('Mengirim…') : tr('Kirim uji')}
					</button>
				</div>
				{#if testMsg.text}
					<p class="text-xs" style="color: {testMsg.ok ? 'var(--success)' : 'var(--destructive)'};">{testMsg.text}</p>
				{/if}
				{#if editorError}<Feedback error={editorError} />{/if}
			</div>
			<div>
				<div class="flex items-center gap-2">
					<p class="text-xs font-medium text-muted-foreground">{tr('Pratinjau')}</p>
					{#if previewBusy}<span class="text-xs text-muted-foreground">{tr('Memperbarui…')}</span>{/if}
					<div class="flex-1"></div>
					<button class="app-btn app-btn-soft !min-h-7 !px-2 !text-xs" type="button" onclick={() => void refreshPreview()}>
						<Eye size={13} /> {tr('Perbarui')}
					</button>
				</div>
				<iframe
					title={tr('Pratinjau email')}
					sandbox=""
					srcdoc={previewHtml}
					class="mt-1 h-[420px] w-full rounded-lg border"
					style="border-color: var(--border); background: #fff;"
				></iframe>
			</div>
		</div>
		{#snippet footer()}
			<button class="app-btn app-btn-soft" type="button" disabled={editorBusy} onclick={() => (editing = null)}>{tr('Batal')}</button>
			<button class="app-btn app-btn-primary" type="button" disabled={editorBusy} onclick={() => void saveTemplate()}>
				<Save size={14} /> {editorBusy ? tr('Menyimpan…') : tr('Simpan template')}
			</button>
		{/snippet}
	</Modal>
{/if}
