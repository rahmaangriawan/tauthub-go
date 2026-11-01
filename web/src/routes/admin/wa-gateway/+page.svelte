<script lang="ts">
	// WA Gateway — halaman tipis. Seluruh isi tinggal di wa-gateway-panel.svelte
	// supaya panel yang sama bisa dirender sebagai tab di /admin/konfigurasi
	// (tab "WA Gateway") tanpa duplikasi kode.
	import WaGatewayPanel from './wa-gateway-panel.svelte';
</script>

<WaGatewayPanel />
