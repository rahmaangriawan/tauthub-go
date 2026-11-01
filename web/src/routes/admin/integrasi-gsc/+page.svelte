<script lang="ts">
	// Integrasi GSC — halaman tipis. Seluruh isi tinggal di gsc-panel.svelte
	// supaya panel yang sama bisa dirender sebagai tab di /admin/konfigurasi
	// (tab "Integrasi GSC") tanpa duplikasi kode.
	import GscPanel from './gsc-panel.svelte';
</script>

<GscPanel />
