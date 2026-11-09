import type { PageServerLoad } from './$types';
import { appOrigin } from '$lib/server/data-site';
import { urlForSource } from '$lib/server/docs';

export const load: PageServerLoad = () => {
	const quick = [
		{ icon: 'user-plus', title: 'Daftar dan masuk', note: 'Buat akun dan pilih peran', path: 'umum/02-daftar-dan-masuk.md' },
		{ icon: 'wallet', title: 'Saldo dan Top Up', note: 'Isi saldo untuk menerbitkan artikel', path: 'klien/08-saldo-dan-top-up.md' },
		{ icon: 'send', title: 'Cara artikel terbit', note: 'Dari persetujuan sampai tayang', path: 'mitra/03-publikasi/07-cara-artikel-terbit.md' },
		{ icon: 'plug', title: 'Integrasi WordPress', note: 'Sambungkan situs media Anda', path: 'mitra/13-integrasi/01-wordpress.md' },
		{ icon: 'shield', title: 'Link Guardian', note: 'Pantau artikel tetap tayang', path: 'klien/06-link-guardian.md' },
		{ icon: 'book', title: 'Glosarium', note: 'Arti istilah di Tauthub', path: 'umum/05-glosarium.md' }
	].map((q) => ({ icon: q.icon, title: q.title, note: q.note, url: urlForSource(q.path) }));

	return {
		quick,
		start: {
			klien: urlForSource('klien/00-mulai-cepat.md'),
			mitra: urlForSource('mitra/00-mulai-cepat.md'),
			umum: urlForSource('umum/README.md')
		},
		seo: {
			title: 'Dokumentasi Tauthub: panduan Klien dan Mitra',
			description:
				'Panduan lengkap memakai Tauthub, dari mendaftar sampai artikel terbit, untuk Klien dan Mitra media.',
			canonical: `${appOrigin()}/dokumentasi`
		}
	};
};
