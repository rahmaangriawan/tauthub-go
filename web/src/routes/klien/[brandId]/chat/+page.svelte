<script lang="ts">
	// Chat AI brand (fase 4): grounding RAG + brief wizard. Mendukung
	// conversation_id lanjutan dan tombol aksi dari server.
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Bot, Send, Sparkles, User } from '@lucide/svelte';
	import { getBrandUrlId } from '$lib/brand-url.svelte';
	import { apiRequest, ensureValidSession, AUTH_EXPIRED_EVENT } from '$lib/auth';

	const brandId = $derived(page.params.brandId as string);
	// URL navigasi memakai public_id (parity app lama; UUID tetap valid di route).
	const brandUrl = $derived(getBrandUrlId() || brandId);

	type Msg = { role: 'user' | 'ai'; text: string; actions?: Array<{ label: string; href: string }> };

	let messages = $state<Msg[]>([]);
	let input = $state('');
	let busy = $state(false);
	let conversationId = $state<string | null>(null);
	let error = $state('');

	onMount(() => {
		void (async () => {
			const fresh = await ensureValidSession();
			if (!fresh) return;
		})();
		const onExpired = () => (window.location.href = '/login');
		window.addEventListener(AUTH_EXPIRED_EVENT, onExpired);
		return () => window.removeEventListener(AUTH_EXPIRED_EVENT, onExpired);
	});

	async function send(e: SubmitEvent) {
		e.preventDefault();
		const text = input.trim();
		if (!text || busy) return;
		busy = true;
		error = '';
		messages = [...messages, { role: 'user', text }];
		input = '';
		try {
			const res = await apiRequest(`/v1/brands/${brandId}/chat`, {
				method: 'POST',
				body: JSON.stringify({ message: text, conversation_id: conversationId })
			});
			const data = await res.json();
			if (!res.ok) {
				error = typeof data.detail === 'string' ? data.detail : 'Gagal menghubungi agen.';
				return;
			}
			conversationId = data.conversation_id ?? conversationId;
			messages = [...messages, { role: 'ai', text: data.reply, actions: data.actions ?? [] }];
		} catch {
			error = 'Tidak dapat menghubungi server. Coba lagi.';
		} finally {
			busy = false;
		}
	}
</script>

<section class="app-card flex h-[calc(100dvh-160px)] flex-col">
	<header class="flex items-center gap-2.5 border-b px-5 py-3.5">
		<div class="flex h-9 w-9 items-center justify-center rounded-xl text-primary-foreground" style="background-color: var(--primary);">
			<Bot size={18} />
		</div>
		<div>
			<p class="text-sm font-semibold">Agen Tauthub</p>
			<p class="text-xs text-muted-foreground">Grounding knowledge base + brand context</p>
		</div>
	</header>

	<div class="flex-1 space-y-3 overflow-y-auto px-5 py-4">
		{#if messages.length === 0}
			<div class="py-10 text-center text-sm text-muted-foreground">
				<Sparkles size={22} class="mx-auto mb-2" style="color: var(--primary);" />
				<p>Tanya apa saja soal brand ini, atau tulis</p>
				<p class="mt-1 font-medium text-foreground">"buat artikel tentang …"</p>
			</div>
		{/if}

		{#each messages as m, i (i)}
			<div class="flex gap-2.5 {m.role === 'user' ? 'flex-row-reverse' : ''}">
				<div
					class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-semibold"
					style={m.role === 'user'
						? 'background-color: var(--secondary); color: var(--secondary-foreground);'
						: 'background-color: var(--accent); color: var(--accent-foreground);'}
				>
					{#if m.role === 'user'}<User size={15} />{:else}<Bot size={15} />{/if}
				</div>
				<div class="min-w-0 max-w-[80%] rounded-xl px-3.5 py-2.5 text-sm {m.role === 'user' ? 'text-right' : ''}"
					style={m.role === 'user' ? 'background-color: var(--accent);' : 'background-color: var(--secondary);'}>
					<p class="whitespace-pre-wrap leading-relaxed">{m.text}</p>
					{#if (m.actions?.length ?? 0) > 0}
						<div class="mt-2 flex flex-wrap gap-2">
							{#each m.actions ?? [] as a (a.href)}
								<a class="app-btn app-btn-primary !py-1 !text-xs" href={a.href}>{a.label}</a>
							{/each}
						</div>
					{/if}
				</div>
			</div>
		{/each}

		{#if busy}
			<p class="text-xs text-muted-foreground">Agen sedang menyusun balasan…</p>
		{/if}
		{#if error}
			<p class="rounded-md px-3 py-2 text-sm" style="background-color: color-mix(in srgb, var(--destructive) 10%, transparent); color: var(--destructive);">{error}</p>
		{/if}
	</div>

	<form class="flex items-center gap-2 border-t px-4 py-3" onsubmit={send}>
		<input class="app-input" placeholder="Tulis pertanyaan atau perintah…" bind:value={input} disabled={busy} />
		<button class="app-btn app-btn-primary !px-3" type="submit" disabled={busy || !input.trim()} aria-label="Kirim">
			<Send size={16} />
		</button>
	</form>
</section>
