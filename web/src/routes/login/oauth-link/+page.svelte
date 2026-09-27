<script lang="ts">
	// Port /login/oauth-link — hubungkan Google ke akun password existing (wajib password).
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { apiRequest, authErrorMessage, storeSession } from '$lib/auth';

	let password = $state('');
	let busy = $state(false);
	let error = $state('');

	const email = $derived(new URLSearchParams(page.url.search).get('email') ?? '');

	async function link(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const response = await apiRequest('/v1/auth/google/link', {
				method: 'POST',
				body: JSON.stringify({ password })
			});
			if (!response.ok) {
				error = authErrorMessage(new Error(((await response.json()) as { detail?: string }).detail ?? 'Gagal menghubungkan akun.'));
				return;
			}
			const data = await response.json();
			storeSession(data);
			await goto('/klien');
		} catch (reason) {
			error = authErrorMessage(reason);
		} finally {
			busy = false;
		}
	}
</script>

<main class="flex min-h-screen items-center justify-center bg-background px-4">
	<div class="app-card w-full max-w-[440px] p-6" style="margin-top:10vh">
		<h1 class="text-lg font-bold">Hubungkan akun Google</h1>
		<p class="mt-1 text-sm text-muted-foreground">
			Akun dengan email <strong>{email}</strong> sudah terdaftar. Hubungkan akun Google ini dengan akun Tauthub Anda?
		</p>
		<form class="mt-4 flex flex-col gap-3" onsubmit={link}>
			<input
				class="app-input"
				type="password"
				required
				placeholder="Kata sandi akun Tauthub Anda"
				bind:value={password}
			/>
			{#if error}
				<p class="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>
			{/if}
			<div class="flex gap-2">
				<button
					type="submit"
					class="app-btn app-btn-primary"
					disabled={busy || !password}
				>
					{busy ? 'Menghubungkan…' : 'Hubungkan & Lanjutkan'}
				</button>
				<a class="rounded-md bg-muted px-3 py-2 text-sm" href="/login">Batal</a>
			</div>
		</form>
	</div>
</main>
