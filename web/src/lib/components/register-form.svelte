<script lang="ts">
	import { tr } from '$lib/i18n/phrases';

	// Port /register/klien + /register/mitra (satu komponen, role via prop).
	import { goto } from '$app/navigation';
	import { authErrorMessage } from '$lib/auth';

	let { role }: { role: 'client' | 'partner' } = $props();

	let email = $state('');
	let password = $state('');
	let displayName = $state('');
	let partnerCategory = $state('');
	let referralCode = $state('');
	let busy = $state(false);
	let error = $state('');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (busy) return;
		busy = true;
		error = '';
		try {
			const payload: Record<string, unknown> = {
				email: email.trim().toLowerCase(),
				password,
				display_name: displayName.trim(),
				role
			};
			if (role === 'partner' && partnerCategory) payload.partner_category = partnerCategory;
			if (referralCode.trim()) payload.referral_code = referralCode.trim();

			const res = await fetch('/v1/auth/register', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(payload)
			});
			const data = await res.json();
			if (!res.ok) {
				error =
					typeof data.detail === 'string'
						? data.detail
						: 'Pendaftaran gagal. Periksa isian lalu coba lagi.';
				return;
			}
			// 201 user JSON (baru) atau {ok:true} — keduanya diarahkan ke login.
			await goto('/login?registered=1');
		} catch (reason) {
			error = authErrorMessage(reason);
		} finally {
			busy = false;
		}
	}
</script>

<main class="flex min-h-screen items-center justify-center bg-background px-4">
	<div class="w-full max-w-sm rounded-lg border border-border bg-card p-6">
		<h1 class="text-xl font-semibold tracking-tight"> {tr("Daftar sebagai")} {role === 'partner' ? 'Mitra Media' : tr('Klien')}
		</h1>

		<form class="mt-4 flex flex-col gap-3" onsubmit={submit}>
			<label class="flex flex-col gap-1 text-sm">{tr("Email")}<input
					class="rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring"
					type="email" bind:value={email} required
				/>
			</label>
			<label class="flex flex-col gap-1 text-sm">{tr("Password (min. 10 karakter)")}<input
					class="rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring"
					type="password" bind:value={password} minlength={10} required
				/>
			</label>
			<label class="flex flex-col gap-1 text-sm">{tr("Nama")}<input
					class="rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring"
					type="text" bind:value={displayName} minlength={2} maxlength={120} required
				/>
			</label>
			{#if role === 'partner'}
				<label class="flex flex-col gap-1 text-sm">
					Kategori media
					<select
						class="rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring"
						bind:value={partnerCategory} required
					>
						<option value="" disabled>{tr("Pilih kategori")}</option>
						<option value="berita">Berita</option>
						<option value="lifestyle">Lifestyle</option>
						<option value="lainnya">Lainnya</option>
					</select>
				</label>
			{/if}
			<label class="flex flex-col gap-1 text-sm">
				Kode referral (opsional)
				<input
					class="rounded-md border border-input bg-background px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-ring"
					type="text" bind:value={referralCode} maxlength={64}
				/>
			</label>

			{#if error}
				<p class="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</p>
			{/if}

			<button
				type="submit"
				disabled={busy}
				class="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
			>
				{busy ? 'Mendaftarkan…' : tr('Daftar')}
			</button>
		</form>
		<p class="mt-3 text-center text-sm text-muted-foreground">
			<a class="underline" href="/login">{tr("Masuk")}</a>
		</p>
	</div>
</main>
