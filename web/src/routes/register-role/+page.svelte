<script lang="ts">
	// Port /register-role — pilih peran pendaftaran.
	import { goto } from '$app/navigation';
	let role = $state<'client' | 'partner' | null>(null);

	function lanjut() {
		if (!role) return;
		goto(role === 'partner' ? '/register/mitra' : '/register/klien');
	}
</script>

<main class="flex min-h-screen items-center justify-center bg-background px-4">
	<div class="app-card w-full max-w-md p-6">
		<h1 class="text-xl font-semibold tracking-tight">Daftar sebagai</h1>
		<div class="mt-4 grid gap-3">
			<button
				class="rounded-lg border p-4 text-left transition-colors {role === 'client'
					? 'border-primary ring-2 ring-ring'
					: 'border-border hover:bg-accent'}"
				onclick={() => (role = 'client')}
			>
				<span class="font-medium">Klien</span>
				<span class="block text-sm text-muted-foreground">Brand yang memproduksi & menyebarkan artikel PR</span>
			</button>
			<button
				class="rounded-lg border p-4 text-left transition-colors {role === 'partner'
					? 'border-primary ring-2 ring-ring'
					: 'border-border hover:bg-accent'}"
				onclick={() => (role = 'partner')}
			>
				<span class="font-medium">Mitra Media</span>
				<span class="block text-sm text-muted-foreground">Pemilik media yang menerima & mempublikasikan konten</span>
			</button>
		</div>
		<button
			class="mt-4 w-full rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
			disabled={!role}
			onclick={lanjut}
		>
			Lanjut
		</button>
		<p class="mt-3 text-center text-sm text-muted-foreground">
			Sudah punya akun? <a class="underline" href="/login">Masuk</a>
		</p>
	</div>
</main>
