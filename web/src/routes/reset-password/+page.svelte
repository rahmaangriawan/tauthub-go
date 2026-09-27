<script lang="ts">
	// Port /reset-password — token dari query, password baru min 10.
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import PageTitle from '$lib/components/page-title.svelte';

	let newPassword = $state('');
	let busy = $state(false);
	let error = $state('');
	let success = $state(false);

	const token = $derived(new URLSearchParams(page.url.search).get('token') ?? '');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (busy) return;
		busy = true;
		error = '';
		try {
			const res = await fetch('/v1/auth/reset-password', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ token, new_password: newPassword })
			});
			const data = await res.json();
			if (res.ok && data.ok) {
				success = true;
				setTimeout(() => void goto('/login'), 1500);
			} else {
				error =
					typeof data.detail === 'string'
						? data.detail
						: 'Token reset tidak valid atau sudah kedaluwarsa.';
			}
		} catch {
			error = 'Tidak dapat terhubung ke server. Coba lagi.';
		} finally {
			busy = false;
		}
	}
</script>

<PageTitle title="Reset Password" />

<main class="flex min-h-screen items-center justify-center bg-background px-4">
	<div class="app-card w-full max-w-sm p-6">
		<h1 class="text-xl font-semibold tracking-tight">Reset password</h1>
		{#if !token}
			<p class="mt-4 text-sm text-destructive">Token reset tidak ditemukan. Minta tautan baru.</p>
		{:else if success}
			<p class="mt-4 text-sm text-emerald-600">Password berhasil diubah. Mengarahkan ke halaman login…</p>
		{:else}
			<form class="mt-4 flex flex-col gap-3" onsubmit={submit}>
				<label class="flex flex-col gap-1 text-sm">
					Password baru (min. 10 karakter)
					<input
						class="app-input"
						type="password" bind:value={newPassword} minlength={10} required
					/>
				</label>
				{#if error}
					<p class="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>
				{/if}
				<button
					type="submit"
					disabled={busy}
					class="app-btn app-btn-primary"
				>
					{busy ? 'Menyimpan…' : 'Simpan password baru'}
				</button>
			</form>
		{/if}
	</div>
</main>
