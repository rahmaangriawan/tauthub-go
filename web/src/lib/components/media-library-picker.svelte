<script lang="ts">
	// Modal pustaka media (gambar AI) untuk satu artikel. Isi pustaka DIISOLASI
	// per scope (brand + campaign + media + article) — server memfilter penuh,
	// jadi gambar milik artikel lain tidak pernah muncul di sini.
	//
	// Dipakai dua tempat: pemilih cover (mode 'cover') dan tombol gambar di
	// toolbar editor (mode 'inline'). Ada juga area "Generate baru" yang
	// menampilkan biaya per gambar + saldo brand dalam rupiah, lalu memanggil
	// endpoint generate dan menyegarkan grid.
		import { Check, CircleAlert, Image as ImageIcon, Info, LoaderCircle, Sparkles, Trash2, Upload, X } from '@lucide/svelte';
	import { apiRequest, readApiError } from '$lib/auth';
	import { tr } from '$lib/i18n/phrases';
	import { tcToRp } from '$lib/credits/rupiah';

	type LibraryImage = {
		id: string;
		url: string;
		provider: string;
		model: string;
		prompt: string;
		heading: string | null;
		role: string;
		width: number;
		height: number;
		cost_credits: number;
		created_at: string;
	};

	let {
		open = false,
		brandId,
		campaignId,
		mediaId,
		articleId = null,
		mode = 'inline',
		multi = false,
		defaultHeading = '',
		onSelect,
		onSelectMany,
		onClose
	}: {
		open?: boolean;
		brandId: string;
		campaignId: string;
		mediaId: string;
		articleId?: string | null;
		mode?: 'cover' | 'inline';
		/** Mode pilih-banyak: tiap kartu punya kotak centang dan tombol sisip massal. */
		multi?: boolean;
		defaultHeading?: string;
		onSelect: (url: string) => void;
		/** Dipanggil dengan seluruh URL terpilih saat pengguna menekan tombol sisip massal. */
		onSelectMany?: (urls: string[]) => void;
		onClose: () => void;
	} = $props();

	const apiBase = $derived(`/v1/brands/${brandId}/taut/campaigns/${campaignId}`);
	let images = $state<LibraryImage[]>([]);
	let loading = $state(false);
	let listError = $state('');
		let cost = $state(0);
		let balance = $state<number | null>(null);
		let rateIdr = $state(0);
		// Biaya & saldo tampil dalam rupiah memakai kurs aktif dari server.
		const costRp = $derived(cost > 0 ? tcToRp(cost, rateIdr) : tr('Gratis'));
		const balanceRp = $derived(balance == null ? '—' : tcToRp(balance, rateIdr));
		const remainingRp = $derived(balance == null ? '—' : tcToRp(Math.max(0, balance - cost), rateIdr));
		let prompt = $state('');
		let generating = $state(false);
		let confirmOpen = $state(false);
		let genError = $state('');

		// Unggah manual (drag & drop, browse, paste)
		let fileInputEl = $state<HTMLInputElement | undefined>(undefined);
		let uploading = $state(false);
		// Progres unggah nyata: jumlah file selesai dari total yang dipilih sekaligus.
		let uploadDone = $state(0);
		let uploadTotal = $state(0);
		let uploadDragOver = $state(false);
		let uploadError = $state('');
		let uploadSuccess = $state('');

		// Hapus gambar
		let deleteConfirmOpen = $state(false);
		let imageToDelete = $state<LibraryImage | null>(null);
		let deleteBusy = $state(false);
		let deleteError = $state('');

		// Pilih banyak (multi-select) untuk mode inline
		let selectedIds = $state<Set<string>>(new Set());
		const selectedImages = $derived(images.filter((img) => selectedIds.has(img.id)));
		const allSelected = $derived(images.length > 0 && selectedIds.size === images.length);

		function toggleSelect(id: string) {
			const next = new Set(selectedIds);
			if (next.has(id)) next.delete(id);
			else next.add(id);
			selectedIds = next;
		}

		function toggleSelectAll() {
			selectedIds = allSelected ? new Set() : new Set(images.map((img) => img.id));
		}

		function insertSelected() {
			if (selectedImages.length === 0) return;
			const urls = selectedImages.map((img) => img.url);
			if (onSelectMany) onSelectMany(urls);
			else urls.forEach((url) => onSelect(url));
			selectedIds = new Set();
		}

		let loadedFor = '';

		$effect(() => {
			if (!open) {
				loadedFor = '';
				selectedIds = new Set();
				return;
			}
			const key = `${brandId}:${campaignId}:${mediaId}:${articleId ?? ''}`;
			if (key !== loadedFor) {
				loadedFor = key;
				selectedIds = new Set();
				prompt = defaultHeading || '';
				void load();
			}
		});

		$effect(() => {
			if (!open) return;
			const onKey = (e: KeyboardEvent) => {
				if (confirmOpen || deleteConfirmOpen) return;
				if (e.key === 'Escape' && !generating && !uploading && !deleteBusy) onClose();
			};
			const onPaste = (e: ClipboardEvent) => {
				if (generating || uploading || deleteBusy || confirmOpen || deleteConfirmOpen) return;
				const files = Array.from(e.clipboardData?.files ?? []).filter((f) => f.type.startsWith('image/'));
				if (files.length > 0) {
					e.preventDefault();
					void handleUploadMany(files);
				}
			};
			window.addEventListener('keydown', onKey);
			window.addEventListener('paste', onPaste);
			return () => {
				window.removeEventListener('keydown', onKey);
				window.removeEventListener('paste', onPaste);
			};
		});

	async function load() {
		loading = true;
		listError = '';
		try {
			const qs = new URLSearchParams({ media_id: mediaId });
			if (articleId) qs.set('article_id', articleId);
			const res = await apiRequest(`${apiBase}/images?${qs.toString()}`);
			const data = await res.json().catch(() => null);
			if (!res.ok) throw new Error(await readApiError(res, tr('Gagal memuat pustaka gambar.')));
			images = Array.isArray(data?.images) ? (data.images as LibraryImage[]) : [];
			cost = Number(data?.cost_credits ?? 0);
			balance = Number.isFinite(Number(data?.balance_remaining)) ? Number(data.balance_remaining) : null;
			if (Number(data?.rate_idr) > 0) rateIdr = Number(data.rate_idr);
		} catch (reason) {
			listError = reason instanceof Error ? reason.message : tr('Gagal memuat pustaka gambar.');
		} finally {
			loading = false;
		}
	}

		function requestGenerate() {
			if (generating || !prompt.trim()) return;
			confirmOpen = true;
		}

			async function executeGenerate() {
				if (generating || !prompt.trim()) return;
				confirmOpen = false;
				generating = true;
				genError = '';
				try {
					const res = await apiRequest(
						`${apiBase}/images/generate`,
						{
							method: 'POST',
							body: JSON.stringify({
								media_id: mediaId,
								article_id: articleId ?? undefined,
								heading: defaultHeading || undefined,
								prompt: prompt.trim(),
								role: mode
							})
						},
						180_000
					);
					const data = await res.json().catch(() => null);
					if (!res.ok) throw new Error(await readApiError(res, tr('Gagal membuat gambar AI.')));
					if (typeof data?.balance_remaining === 'number') balance = data.balance_remaining;
					if (Number(data?.rate_idr) > 0) rateIdr = Number(data.rate_idr);
					await load();
				} catch (reason) {
					genError = reason instanceof Error ? reason.message : tr('Gagal membuat gambar AI.');
				} finally {
					generating = false;
				}
			}

			// ---------- unggah manual (drag & drop, browse, paste) ----------
			/**
			 * Unggah satu berkas ke pustaka artikel ini. Endpoint server menerima
			 * satu file per request, jadi pemanggil yang menggabungkan hasilnya.
			 * Melempar Error bila server menolak, supaya batch bisa melaporkannya.
			 */
			async function uploadOne(file: File): Promise<LibraryImage> {
				const form = new FormData();
				form.append('file', file);
				form.append('media_id', mediaId);
				if (articleId) form.append('article_id', articleId);
				form.append('role', mode);

				const res = await apiRequest(`${apiBase}/images/upload`, {
					method: 'POST',
					body: form
				});
				const data = await res.json().catch(() => null);
				if (!res.ok) throw new Error(await readApiError(res, tr('Gagal mengunggah gambar.')));
				if (!data?.id) throw new Error(tr('Gagal mengunggah gambar.'));
				return data as LibraryImage;
			}

			/**
			 * Unggah banyak gambar sekaligus. Dijalankan berurutan (bukan paralel)
			 * supaya server tidak dibanjiri dan progres yang ditampilkan jujur.
			 */
			async function handleUploadMany(files: File[]) {
				if (uploading || files.length === 0) return;
				const valid = files.filter((f) => f.type.startsWith('image/'));
				if (valid.length === 0) {
					uploadError = tr('Format file tidak didukung. Unggah file gambar (PNG, JPG, WebP).');
					return;
				}
				uploading = true;
				uploadDone = 0;
				uploadTotal = valid.length;
				uploadError = '';
				uploadSuccess = '';
				const failed: string[] = [];
				const uploaded: LibraryImage[] = [];
				try {
					for (const file of valid) {
						try {
							uploaded.push(await uploadOne(file));
						} catch (err) {
							const name = file.name || 'file';
							const reason = err instanceof Error ? err.message : tr('Gagal mengunggah gambar.');
							failed.push(`${name}: ${reason}`);
						} finally {
							uploadDone += 1;
						}
					}
					if (uploaded.length > 0) {
						images = [...uploaded.reverse(), ...images];
						uploadSuccess = uploaded.length === 1
							? tr('Gambar berhasil diunggah & dicrop ke 16:9.')
							: tr('{n} gambar berhasil diunggah & dicrop ke 16:9.').replace('{n}', String(uploaded.length));
						setTimeout(() => {
							uploadSuccess = '';
						}, 4000);
					}
					if (failed.length > 0) {
						uploadError = failed.join(' · ');
					}
				} finally {
					uploading = false;
					uploadDone = 0;
					uploadTotal = 0;
				}
			}

			function onDropUpload(e: DragEvent) {
				e.preventDefault();
				uploadDragOver = false;
				const files = Array.from(e.dataTransfer?.files ?? []);
				if (files.length > 0) void handleUploadMany(files);
			}

			function onFileSelected(e: Event) {
				const input = e.currentTarget as HTMLInputElement;
				const files = Array.from(input.files ?? []);
				if (files.length > 0) void handleUploadMany(files);
				input.value = '';
			}

			// ---------- hapus gambar dari pustaka ----------
			function promptDelete(img: LibraryImage) {
				imageToDelete = img;
				deleteError = '';
				deleteConfirmOpen = true;
			}

			async function executeDelete() {
				if (!imageToDelete || deleteBusy) return;
				deleteBusy = true;
				deleteError = '';
				try {
					const res = await apiRequest(`${apiBase}/images/${imageToDelete.id}`, {
						method: 'DELETE'
					});
					if (!res.ok) throw new Error(await readApiError(res, tr('Gagal menghapus gambar.')));
					images = images.filter((img) => img.id !== imageToDelete?.id);
					deleteConfirmOpen = false;
					imageToDelete = null;
				} catch (err) {
					deleteError = err instanceof Error ? err.message : tr('Gagal menghapus gambar.');
				} finally {
					deleteBusy = false;
				}
			}
</script>

{#if open}
	<div
		class="fixed inset-0 z-[70] flex items-center justify-center p-3 sm:p-5"
		style="background-color: rgba(15, 23, 42, 0.5);"
		role="presentation"
		onclick={(e) => {
			if (e.target === e.currentTarget && !generating) onClose();
		}}
	>
		<div
			class="flex max-h-[92vh] w-full max-w-4xl flex-col overflow-hidden rounded-2xl border shadow-2xl"
			style="border-color: var(--border); background-color: var(--card, #fff);"
			role="dialog"
			aria-modal="true"
			aria-label={tr('Pustaka Media')}
			tabindex="-1"
			onkeydown={(e) => {
				if (e.key === 'Escape' && !generating) onClose();
			}}
		>
			<div class="flex items-start justify-between gap-3 border-b px-5 py-4" style="border-color: var(--border);">
				<div class="min-w-0">
					<h3 class="flex items-center gap-2 text-base font-semibold">
						<ImageIcon size={17} style="color: var(--primary);" />
						{mode === 'cover' ? tr('Pilih Gambar Utama (Pustaka AI)') : tr('Pustaka Gambar AI')}
					</h3>
					<p class="mt-0.5 text-xs text-muted-foreground">{tr('Gambar hasil AI untuk artikel ini saja — pustaka terpisah antar artikel.')}</p>
				</div>
				<button class="rounded-lg p-1.5 hover:bg-secondary" type="button" aria-label={tr('Tutup')} disabled={generating} onclick={onClose}>
					<X size={18} />
				</button>
			</div>

				<div class="flex-1 overflow-y-auto p-5">
					<!-- Box Unggah Manual (Drag & Drop, Browse, Paste Ctrl+V) -->
					<div
						class="mb-4 flex cursor-pointer flex-col items-center justify-center rounded-xl border-2 border-dashed p-4 text-center transition-colors"
						class:border-primary={uploadDragOver}
						style="border-color: {uploadDragOver ? 'var(--primary)' : 'var(--border)'}; background-color: var(--muted, #f8fafc);"
						role="button"
						tabindex="0"
						onclick={() => fileInputEl?.click()}
						onkeydown={(e) => {
							if (e.key === 'Enter' || e.key === ' ') {
								e.preventDefault();
								fileInputEl?.click();
							}
						}}
						ondragover={(e) => {
							e.preventDefault();
							uploadDragOver = true;
						}}
						ondragleave={() => (uploadDragOver = false)}
						ondrop={onDropUpload}
					>
						<input
							bind:this={fileInputEl}
							type="file"
							accept="image/*"
							multiple
							class="hidden"
							onchange={onFileSelected}
						/>
						{#if uploading}
							<div class="flex items-center gap-2 py-1 text-xs font-semibold" style="color: var(--primary);">
								<LoaderCircle size={18} class="animate-spin" />
								<span>
									{#if uploadTotal > 1}
										{tr('Mengunggah {n} dari {m} gambar…').replace('{n}', String(Math.min(uploadDone + 1, uploadTotal))).replace('{m}', String(uploadTotal))}
									{:else}
										{tr('Mengunggah & menormalkan gambar (16:9)…')}
									{/if}
								</span>
							</div>
						{:else}
							<div class="flex items-center gap-2 text-xs font-semibold text-foreground">
								<Upload size={15} class="text-muted-foreground" />
								<span>{tr('Tarik & letakkan gambar ke sini, klik untuk memilih, atau tempel (Ctrl+V)')}</span>
							</div>
							<p class="mt-0.5 text-[11px] text-muted-foreground">
								{tr('PNG, JPG, WebP · bisa pilih beberapa sekaligus, otomatis dicrop ke rasio 16:9 & disimpan ke pustaka artikel ini')}
							</p>
						{/if}
					</div>

					{#if uploadError}
						<p class="mb-3 flex items-center gap-1.5 text-xs" style="color: var(--destructive);">
							<CircleAlert size={14} /> {uploadError}
						</p>
					{:else if uploadSuccess}
						<p class="mb-3 flex items-center gap-1.5 text-xs" style="color: var(--success);">
							<Check size={14} /> {uploadSuccess}
						</p>
					{/if}

					{#if loading}
						<div class="flex items-center gap-2 py-10 text-sm text-muted-foreground">
							<LoaderCircle size={18} class="animate-spin" /> {tr('Memuat pustaka…')}
						</div>
					{:else if listError}
						<p class="flex items-center gap-1.5 text-sm" style="color: var(--destructive);"><CircleAlert size={15} /> {listError}</p>
					{:else if images.length === 0}
						<div class="rounded-xl border-2 border-dashed px-6 py-10 text-center" style="border-color: var(--border); background-color: var(--muted);">
							<p class="text-sm font-semibold">{tr('Belum ada gambar di pustaka ini.')}</p>
							<p class="mt-1 text-xs text-muted-foreground">{tr('Unggah gambar di atas atau buat gambar baru lewat kotak di bawah.')}</p>
						</div>
					{:else}
						{#if multi && images.length > 0}
							<div class="mb-3 flex flex-wrap items-center justify-between gap-2">
								<label class="inline-flex cursor-pointer items-center gap-2 text-xs font-semibold text-foreground">
									<input
										type="checkbox"
										class="lg-check"
										checked={allSelected}
										aria-label={tr('Pilih semua')}
										onchange={toggleSelectAll}
									/>
									<span>{allSelected ? tr('Batal pilih') : tr('Pilih semua')}</span>
								</label>
								<span class="text-[11.5px] text-muted-foreground">
									{#if selectedIds.size > 0}
										{tr('{n} gambar dipilih').replace('{n}', String(selectedIds.size))}
									{:else}
										{tr('Centang gambar yang ingin disisipkan')}
									{/if}
								</span>
							</div>
						{/if}

						<div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
							{#each images as img (img.id)}
								{@const picked = selectedIds.has(img.id)}
								<div
									class="group relative overflow-hidden rounded-xl border text-left transition-colors"
									style="border-color: {multi && picked ? 'var(--primary)' : 'var(--border)'}; background-color: {multi && picked ? 'var(--accent)' : 'transparent'};"
								>
									<button
										class="block w-full text-left"
										type="button"
										title={img.heading || img.prompt}
										aria-pressed={multi ? picked : undefined}
										onclick={() => (multi ? toggleSelect(img.id) : onSelect(img.url))}
									>
										<img src={img.url} alt={img.heading || img.prompt} class="aspect-video w-full object-cover transition-transform group-hover:scale-[1.03]" />
										<div class="flex items-center justify-between gap-1 px-2 py-1.5">
											<span class="truncate text-[11px] font-medium">{img.heading || img.prompt || img.role}</span>
											{#if img.provider === 'placeholder'}
												<span class="shrink-0 rounded-full px-1.5 py-0.5 text-[9px] font-bold" style="background-color: var(--warning-soft); color: var(--warning);">{tr('placeholder')}</span>
											{:else if img.provider === 'upload'}
												<span class="shrink-0 rounded-full px-1.5 py-0.5 text-[9px] font-bold" style="background-color: var(--secondary); color: var(--foreground);">{tr('upload')}</span>
											{/if}
										</div>
									</button>

									{#if multi}
										<!-- Kotak centang selalu terlihat supaya jelas kartu bisa dipilih banyak -->
										<label
											class="absolute left-1.5 top-1.5 flex h-7 w-7 cursor-pointer items-center justify-center rounded-lg border bg-card/95"
											style="border-color: {picked ? 'var(--primary)' : 'var(--border)'};"
											title={tr('Pilih gambar ini')}
										>
											<input
												type="checkbox"
												class="lg-check"
												checked={picked}
												aria-label={tr('Pilih gambar ini')}
												onchange={() => toggleSelect(img.id)}
												onclick={(e) => e.stopPropagation()}
											/>
										</label>
									{/if}

									<!-- Tombol Hapus Gambar -->
									<button
										type="button"
										class="absolute right-1.5 top-1.5 flex h-7 w-7 items-center justify-center rounded-lg border bg-card/90 text-muted-foreground opacity-0 shadow-sm transition-all hover:border-destructive hover:bg-destructive hover:text-white group-hover:opacity-100"
										style="border-color: var(--border);"
										aria-label={tr('Hapus gambar')}
										title={tr('Hapus gambar dari pustaka')}
										onclick={(e) => {
											e.stopPropagation();
											promptDelete(img);
										}}
									>
										<Trash2 size={13} />
									</button>
								</div>
							{/each}
						</div>

						{#if multi && selectedIds.size > 0}
							<!-- Bar aksi menempel di dasar area gulir: tombol sisip selalu terlihat -->
							<div class="sticky bottom-0 -mx-5 mt-4 flex flex-wrap items-center justify-between gap-2 border-t px-5 py-3" style="border-color: var(--border); background-color: var(--card, #fff);">
								<span class="text-xs font-semibold text-foreground">
									{tr('{n} gambar dipilih').replace('{n}', String(selectedIds.size))}
								</span>
								<div class="flex items-center gap-2">
									<button class="app-btn app-btn-soft !min-h-8 !px-3 !text-xs" type="button" onclick={() => (selectedIds = new Set())}>
										{tr('Batal pilih')}
									</button>
									<button class="app-btn app-btn-primary !min-h-8 !px-3 !text-xs" type="button" onclick={insertSelected}>
										<ImageIcon size={13} />
										{tr('Sisipkan ke artikel')}
									</button>
								</div>
							</div>
						{/if}
					{/if}
				</div>

			<div class="border-t px-5 py-4" style="border-color: var(--border);">
				<div
					class="mb-3 flex items-start gap-2 rounded-lg px-3 py-2 text-xs leading-relaxed"
					style="background-color: var(--accent); color: var(--accent-foreground);"
				>
					<Info size={14} class="mt-px flex-none" />
					<p>
						{tr('Anda sebaiknya pakai gambar asli, bukan hasil generate AI. Mitra media lebih sering menyetujui artikel yang gambarnya asli, misalnya foto produk atau materi brand Anda sendiri.')}
					</p>
				</div>
				<div class="flex flex-wrap items-center justify-between gap-2">
					<p class="flex items-center gap-1.5 text-xs font-bold uppercase tracking-wide text-muted-foreground">
						<Sparkles size={13} /> {tr('Generate Baru')}
					</p>
					<p class="text-[11px] text-muted-foreground">
						{tr('Biaya')} <strong class="text-foreground">{costRp}</strong> {tr('per gambar')} ·
						{tr('Saldo')} <strong class="text-foreground">{balanceRp}</strong>
					</p>
				</div>
				<div class="mt-2 flex flex-col gap-2 sm:flex-row sm:items-end">
					<textarea
						class="app-input min-h-[64px] w-full resize-y text-sm"
						placeholder={mode === 'cover' ? tr('Deskripsi gambar cover…') : tr('Deskripsi gambar…')}
						bind:value={prompt}
						maxlength={2000}
						disabled={generating}
					></textarea>
						<button class="app-btn app-btn-primary shrink-0" type="button" disabled={generating || !prompt.trim()} onclick={requestGenerate}>
							{#if generating}<LoaderCircle size={15} class="animate-spin" />{:else}<Sparkles size={15} />{/if}
							{generating ? tr('Membuat…') : `${tr('Generate')} · ${costRp}`}
						</button>
					</div>
					{#if genError}
						<p class="mt-2 flex items-center gap-1.5 text-sm" style="color: var(--destructive);"><CircleAlert size={15} /> {genError}</p>
					{/if}
				</div>
			</div>
		</div>

		<!-- Modal Konfirmasi Generate Gambar (Bukan browser confirm bawaan) -->
		{#if confirmOpen}
			<div
				class="fixed inset-0 z-[80] flex items-center justify-center p-4"
				style="background-color: rgba(15, 23, 42, 0.55);"
				role="presentation"
				onclick={(e) => {
					if (e.target === e.currentTarget && !generating) confirmOpen = false;
				}}
				onkeydown={(e) => {
					if (e.key === 'Escape' && !generating) {
						e.stopPropagation();
						confirmOpen = false;
					}
				}}
			>
				<div
					class="flex w-full max-w-md flex-col overflow-hidden rounded-2xl border shadow-2xl"
					style="border-color: var(--border); background-color: var(--card, #fff);"
					role="dialog"
					aria-modal="true"
					aria-label={mode === 'cover' ? tr('Buat Gambar Cover dengan AI') : tr('Buat Gambar Artikel dengan AI')}
					tabindex="-1"
					onclick={(e) => e.stopPropagation()}
					onkeydown={(e) => e.stopPropagation()}
				>
					<!-- Header -->
					<div class="flex items-start justify-between gap-3 border-b px-5 py-4" style="border-color: var(--border);">
						<div class="flex items-center gap-3">
							<div class="flex h-10 w-10 items-center justify-center rounded-xl" style="background-color: var(--accent); color: var(--primary);">
								<Sparkles size={20} />
							</div>
							<div>
								<h4 class="text-sm font-bold leading-tight text-foreground">
									{mode === 'cover' ? tr('Buat Gambar Cover dengan AI') : tr('Buat Gambar Artikel dengan AI')}
								</h4>
								<p class="mt-0.5 text-xs text-muted-foreground">
									{tr('Konfirmasi pembuatan gambar menggunakan saldo Anda')}
								</p>
							</div>
						</div>
						<button
							class="rounded-lg p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground transition-colors"
							type="button"
							aria-label={tr('Tutup')}
							disabled={generating}
							onclick={() => (confirmOpen = false)}
						>
							<X size={16} />
						</button>
					</div>

					<!-- Body -->
					<div class="space-y-4 p-5">
						<!-- Prompt preview -->
						<div class="rounded-xl border p-3" style="border-color: var(--border); background-color: var(--muted, #f8fafc);">
							<span class="text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
								{tr('Prompt Gambar')}
							</span>
							<p class="mt-1 text-xs font-medium leading-relaxed text-foreground line-clamp-3">
								{prompt.trim()}
							</p>
						</div>

						<!-- Financial breakdown -->
						<div class="rounded-xl border p-3.5 space-y-2.5" style="border-color: var(--border);">
							<div class="flex items-center justify-between text-xs">
								<span class="text-muted-foreground">{tr('Biaya Pembuatan')}</span>
								<strong class="font-bold text-foreground">{costRp}</strong>
							</div>

							<div class="flex items-center justify-between text-xs">
								<span class="text-muted-foreground">{tr('Saldo Saat Ini')}</span>
								<span class="font-medium text-foreground">{balanceRp}</span>
							</div>

							<div class="h-px w-full" style="background-color: var(--border);"></div>

							<div class="flex items-center justify-between text-xs">
								<span class="font-semibold text-foreground">{tr('Estimasi Sisa Saldo')}</span>
								<strong class="font-bold" style="color: var(--primary);">
									{remainingRp}
								</strong>
							</div>
						</div>

						{#if balance != null && balance < cost}
							<div class="flex items-center gap-2 rounded-xl p-3 text-xs" style="background-color: var(--destructive-soft, #fef2f2); color: var(--destructive, #dc2626);">
								<CircleAlert size={15} class="shrink-0" />
								<span>{tr('Saldo tidak mencukupi untuk membuat gambar. Silakan top up terlebih dahulu.')}</span>
							</div>
						{/if}
					</div>

						<!-- Footer Actions -->
						<div class="flex items-center justify-end gap-2.5 border-t px-5 py-3.5" style="border-color: var(--border); background-color: var(--muted, #f8fafc);">
							<button
								class="app-btn app-btn-soft text-xs"
								type="button"
								disabled={generating}
								onclick={() => (confirmOpen = false)}
							>
								{tr('Batal')}
							</button>
							<button
								class="app-btn app-btn-primary text-xs"
								type="button"
								disabled={generating || (balance != null && balance < cost)}
								onclick={executeGenerate}
							>
								{#if generating}
									<LoaderCircle size={14} class="animate-spin" />
									<span>{tr('Memproses…')}</span>
								{:else}
									<Sparkles size={14} />
									<span>{tr('Ya, Buat Gambar')} ({costRp})</span>
								{/if}
							</button>
						</div>
					</div>
				</div>
			{/if}

			<!-- Modal Konfirmasi Hapus Gambar (Bukan browser confirm bawaan) -->
			{#if deleteConfirmOpen && imageToDelete}
				<div
					class="fixed inset-0 z-[80] flex items-center justify-center p-4"
					style="background-color: rgba(15, 23, 42, 0.55);"
					role="presentation"
					onclick={(e) => {
						if (e.target === e.currentTarget && !deleteBusy) deleteConfirmOpen = false;
					}}
					onkeydown={(e) => {
						if (e.key === 'Escape' && !deleteBusy) {
							e.stopPropagation();
							deleteConfirmOpen = false;
						}
					}}
				>
					<div
						class="flex w-full max-w-sm flex-col overflow-hidden rounded-2xl border shadow-2xl"
						style="border-color: var(--border); background-color: var(--card, #fff);"
						role="dialog"
						aria-modal="true"
						aria-label={tr('Hapus Gambar')}
						tabindex="-1"
						onclick={(e) => e.stopPropagation()}
						onkeydown={(e) => e.stopPropagation()}
					>
						<div class="flex items-start justify-between gap-3 border-b px-5 py-4" style="border-color: var(--border);">
							<div class="flex items-center gap-2.5 text-sm font-bold" style="color: var(--destructive);">
								<Trash2 size={18} />
								<span>{tr('Hapus Gambar dari Pustaka?')}</span>
							</div>
							<button
								class="rounded-lg p-1.5 text-muted-foreground hover:bg-secondary hover:text-foreground transition-colors"
								type="button"
								aria-label={tr('Tutup')}
								disabled={deleteBusy}
								onclick={() => (deleteConfirmOpen = false)}
							>
								<X size={16} />
							</button>
						</div>

						<div class="space-y-3 p-5">
							<img
								src={imageToDelete.url}
								alt={imageToDelete.heading || imageToDelete.prompt}
								class="aspect-video w-full rounded-lg border object-cover"
								style="border-color: var(--border);"
							/>
							<p class="text-xs leading-relaxed text-muted-foreground">
								{tr('Gambar ini akan dihapus secara permanen dari pustaka media artikel ini.')}
							</p>
							{#if deleteError}
								<p class="flex items-center gap-1.5 text-xs" style="color: var(--destructive);">
									<CircleAlert size={14} /> {deleteError}
								</p>
							{/if}
						</div>

						<div class="flex items-center justify-end gap-2.5 border-t px-5 py-3.5" style="border-color: var(--border); background-color: var(--muted, #f8fafc);">
							<button
								class="app-btn app-btn-soft text-xs"
								type="button"
								disabled={deleteBusy}
								onclick={() => (deleteConfirmOpen = false)}
							>
								{tr('Batal')}
							</button>
							<button
								class="app-btn text-xs font-semibold"
								style="background-color: var(--destructive); color: #fff;"
								type="button"
								disabled={deleteBusy}
								onclick={executeDelete}
							>
								{#if deleteBusy}
									<LoaderCircle size={14} class="animate-spin" />
									<span>{tr('Menghapus…')}</span>
								{:else}
									<Trash2 size={13} />
									<span>{tr('Hapus Permanen')}</span>
								{/if}
							</button>
						</div>
					</div>
				</div>
			{/if}
		{/if}

<style>
	/* Kotak centang memakai warna aksen sistem, ukuran tetap agar mudah diklik */
	.lg-check {
		width: 16px;
		height: 16px;
		margin: 0;
		accent-color: var(--primary);
		cursor: pointer;
	}
</style>
