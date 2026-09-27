<script lang="ts">
	// Port /forgot-password — selalu netral (anti-enumerasi, parity API).
	let email = $state('');
	let busy = $state(false);
	let sent = $state(false);
	let error = $state('');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (busy) return;
		busy = true;
		error = '';
		try {
			const res = await fetch('/v1/auth/forgot-password', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ email: email.trim().toLowerCase() })
			});
			if (res.ok) {
				sent = true;
			} else {
				error = 'Gagal mengirim permintaan. Coba lagi.';
			}
		} catch {
			error = 'Tidak dapat terhubung ke server. Coba lagi.';
		} finally {
			busy = false;
		}
	}
</script>

<main class="flex min-h-screen items-center justify-center bg-background px-4">
	<div class="app-card w-full max-w-sm p-6">
		<h1 class="text-xl font-semibold tracking-tight">Lupa password</h1>
		{#if sent}
			<p class="mt-4 text-sm text-muted-foreground">
				Jika email terdaftar dan aktif, tautan reset sudah dikirim. Periksa kotak masuk Anda.
			</p>
		{:else}
			<form class="mt-4 flex flex-col gap-3" onsubmit={submit}>
				<label class="flex flex-col gap-1 text-sm">
					Email
					<input
						class="app-input"
						type="email" bind:value={email} required
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
					{busy ? 'Mengirim…' : 'Kirim tautan reset'}
				</button>
			</form>
		{/if}
		<p class="mt-3 text-center text-sm text-muted-foreground"><a class="underline" href="/login">Kembali ke login</a></p>
	</div>
</main>
