// Metadata & preset Pedoman Editorial AI (15 standar penulisan) per jenis konten.
// Port lib/content-standards/types.ts + defaults.ts app lama (kontrak parity:
// label, deskripsi, placeholder, dan preset harus sama persis).

export type EditorialSectionKey =
	| 'content_purpose'
	| 'news_angle'
	| 'journalistic_style'
	| 'required_information'
	| 'optional_information'
	| 'structure_guidance'
	| 'lead_guidance'
	| 'fact_attribution_rules'
	| 'quote_guidance'
	| 'language_tone'
	| 'what_to_avoid'
	| 'angle_variation_rules'
	| 'source_priority'
	| 'fact_conflict_handling'
	| 'editorial_quality_checklist';

export type EditorialSectionsData = Record<EditorialSectionKey, string>;

export type SectionGroupCategory =
	| 'purpose_angle'
	| 'style_structure'
	| 'fact_quotes'
	| 'info_sources'
	| 'guardrails_quality';

export interface EditorialSectionMeta {
	key: EditorialSectionKey;
	number: number;
	label: string;
	group: SectionGroupCategory;
	groupLabel: string;
	description: string;
	placeholder: string;
	priority: number;
}

export const EDITORIAL_GROUPS: Array<{ id: SectionGroupCategory; label: string }> = [
	{ id: 'purpose_angle', label: 'Tujuan & Angle' },
	{ id: 'style_structure', label: 'Gaya & Struktur' },
	{ id: 'fact_quotes', label: 'Fakta & Kutipan' },
	{ id: 'info_sources', label: 'Informasi & Sumber' },
	{ id: 'guardrails_quality', label: 'Larangan & Checklist' }
];

export const EDITORIAL_SECTIONS_META: EditorialSectionMeta[] = [
	{
		key: 'content_purpose',
		number: 1,
		label: 'Content Purpose',
		group: 'purpose_angle',
		groupLabel: 'Tujuan & Angle',
		description: 'Tujuan utama jenis konten tersebut dari sudut pandang editorial, bukan marketing copywriting.',
		placeholder:
			'Contoh: Menjelaskan program sosial perusahaan secara faktual, menunjukkan siapa penerima manfaat, bentuk bantuan, dan dampak nyata...',
		priority: 1
	},
	{
		key: 'news_angle',
		number: 2,
		label: 'News Angle',
		group: 'purpose_angle',
		groupLabel: 'Tujuan & Angle',
		description: 'Angle pemberitaan yang lazim digunakan agar AI tidak selalu memakai sudut pandang yang sama.',
		placeholder:
			'Contoh: Dampak kepada penerima manfaat, nilai program, jumlah penerima, perubahan terukur, relevansi sosial...',
		priority: 2
	},
	{
		key: 'journalistic_style',
		number: 3,
		label: 'Journalistic Style',
		group: 'style_structure',
		groupLabel: 'Gaya & Struktur',
		description: 'Karakter dan gaya penulisan jurnalistik Indonesia yang informatif, faktual, dan lugas.',
		placeholder:
			'Contoh: Gunakan gaya jurnalistik Indonesia yang informatif, faktual, dan lugas. Hindari gaya advertorial yang terlalu memuji perusahaan...',
		priority: 1
	},
	{
		key: 'required_information',
		number: 4,
		label: 'Required Information',
		group: 'info_sources',
		groupLabel: 'Informasi & Sumber',
		description: 'Informasi utama yang idealnya tersedia di bahan baku sebelum artikel dibuat.',
		placeholder:
			'Contoh: Nama program, penyelenggara, lokasi, tanggal, jumlah penerima manfaat, bentuk bantuan, kutipan narasumber...',
		priority: 3
	},
	{
		key: 'optional_information',
		number: 5,
		label: 'Optional Information',
		group: 'info_sources',
		groupLabel: 'Informasi & Sumber',
		description: 'Informasi pelengkap yang bagus jika tersedia namun tidak wajib.',
		placeholder:
			'Contoh: Sejarah program, data tahun sebelumnya, perkembangan penerima, konteks wilayah, data pembanding...',
		priority: 4
	},
	{
		key: 'structure_guidance',
		number: 6,
		label: 'Structure Guidance',
		group: 'style_structure',
		groupLabel: 'Gaya & Struktur',
		description: 'Panduan struktur artikel dan aturan fleksibilitas heading (heading tidak wajib jika artikel pendek).',
		placeholder:
			'Contoh: Untuk artikel pendek default tanpa heading (##). Bangun alur piramida terbalik yang mengalir natural...',
		priority: 2
	},
	{
		key: 'lead_guidance',
		number: 7,
		label: 'Lead / Opening Guidance',
		group: 'style_structure',
		groupLabel: 'Gaya & Struktur',
		description: 'Pedoman paragraf pembuka agar langsung menyampaikan peristiwa dan fakta bernilai berita.',
		placeholder:
			'Contoh: Lead harus segera menyampaikan peristiwa utama, siapa yang terlibat, dan fakta paling bernilai berita tanpa pembukaan promosi...',
		priority: 1
	},
	{
		key: 'fact_attribution_rules',
		number: 8,
		label: 'Fact & Attribution Rules',
		group: 'fact_quotes',
		groupLabel: 'Fakta & Kutipan',
		description:
			'Aturan ketat membedakan fakta independen vs klaim perusahaan/narasumber yang wajib ber-atribusi.',
		placeholder:
			"Contoh: Jika angka atau klaim berasal dari perusahaan, wajib ber-atribusi ('menurut perusahaan', 'pihaknya menyebut'). Jangan ubah klaim jadi fakta mutlak...",
		priority: 1
	},
	{
		key: 'quote_guidance',
		number: 9,
		label: 'Quote Guidance',
		group: 'fact_quotes',
		groupLabel: 'Fakta & Kutipan',
		description:
			'Pedoman penggunaan kutipan: atribusi jelas, jangan membuat kutipan palsu, jangan mengulang isi kutipan di paragraf lain.',
		placeholder:
			'Contoh: Prioritaskan satu kutipan terbaik per narasumber. Dilarang mengarang kutipan baru atau mengubah makna...',
		priority: 2
	},
	{
		key: 'language_tone',
		number: 10,
		label: 'Language & Tone',
		group: 'style_structure',
		groupLabel: 'Gaya & Struktur',
		description: 'Tone dan karakter bahasa yang spesifik untuk jenis konten ini.',
		placeholder:
			'Contoh: Informatif, human-centered, faktual, tanpa frasa superlatif dan tanpa bahasa promosi berlebihan...',
		priority: 2
	},
	{
		key: 'what_to_avoid',
		number: 11,
		label: 'What to Avoid',
		group: 'guardrails_quality',
		groupLabel: 'Larangan & Checklist',
		description: 'Larangan khusus editorial (buzzword, meta-komentar, fakta fiktif, heading berlebihan).',
		placeholder:
			'Contoh: Dilarang menggunakan bahasa promosi berlebihan, dilarang mengarang angka/kutipan, dilarang meta-komentar...',
		priority: 1
	},
	{
		key: 'angle_variation_rules',
		number: 12,
		label: 'Angle Variation Rules',
		group: 'purpose_angle',
		groupLabel: 'Tujuan & Angle',
		description: 'Aturan pergeseran fokus narasi saat memilih angle berbeda tanpa mengubah standar jurnalistik dan fakta.',
		placeholder:
			'Contoh: Angle Dampak Komunitas menggeser fokus ke penerima; Angle Nilai Berita menggeser fokus ke kebaruan/skala. Fakta dan gaya jurnalistik tetap sama...',
		priority: 1
	},
	{
		key: 'source_priority',
		number: 13,
		label: 'Source Priority',
		group: 'info_sources',
		groupLabel: 'Informasi & Sumber',
		description: 'Prioritas sumber bahan (kuesioner > dokumen existing > konteks tambahan).',
		placeholder:
			'Contoh: Jawaban kuesioner menjadi sumber utama untuk fakta program. Dokumen pendukung menjadi sumber sekunder...',
		priority: 2
	},
	{
		key: 'fact_conflict_handling',
		number: 14,
		label: 'Fact Conflict Handling',
		group: 'fact_quotes',
		groupLabel: 'Fakta & Kutipan',
		description:
			'Instruksi apa yang harus dilakukan AI saat menemukan perbedaan tanggal, nama, atau angka antar sumber.',
		placeholder:
			'Contoh: Jangan menggabungkan dua fakta yang bertentangan menjadi satu secara sembarangan. Gunakan atribusi jelas atau prioritaskan sumber primer...',
		priority: 2
	},
	{
		key: 'editorial_quality_checklist',
		number: 15,
		label: 'Editorial Quality Checklist',
		group: 'guardrails_quality',
		groupLabel: 'Larangan & Checklist',
		description: 'Daftar periksa kualitas sebelum artikel dianggap selesai.',
		placeholder:
			'Contoh: Pastikan lead langsung ke inti berita, fakta memiliki sumber jelas, klaim ber-atribusi, tidak ada kutipan palsu, dan angle terlihat jelas...',
		priority: 3
	}
];

export interface ContentTypeStandardVersion {
	id: string;
	standard_id: string;
	content_type_code: string;
	version: number;
	status: 'draft' | 'active' | 'archived';
	change_note: string | null;
	sections: EditorialSectionsData;
	created_by: string | null;
	created_at: string;
}

export interface ContentTypeStandardDetail {
	id: string;
	content_type_code: string;
	content_type_name: string;
	content_type_description: string | null;
	status: 'active' | 'inactive';
	active_version: number | null;
	active_standard: ContentTypeStandardVersion | null;
	versions: Array<{
		id: string;
		version: number;
		status: 'draft' | 'active' | 'archived';
		change_note: string | null;
		created_at: string;
	}>;
	chunk_count: number;
	updated_at: string;
}

export function getEmptyEditorialSections(): EditorialSectionsData {
	return {
		content_purpose: '',
		news_angle: '',
		journalistic_style: '',
		required_information: '',
		optional_information: '',
		structure_guidance: '',
		lead_guidance: '',
		fact_attribution_rules: '',
		quote_guidance: '',
		language_tone: '',
		what_to_avoid: '',
		angle_variation_rules: '',
		source_priority: '',
		fact_conflict_handling: '',
		editorial_quality_checklist: ''
	};
}

/** Preset rekomendasi Tauthub per kode jenis konten (parity DEFAULT_STANDARDS_BY_TYPE). */
export const DEFAULT_STANDARDS_BY_TYPE: Record<string, { name: string; sections: EditorialSectionsData }> = {
	csr: {
		name: 'CSR / Program Sosial',
		sections: {
			content_purpose:
				'Menjelaskan program tanggung jawab sosial perusahaan (CSR) atau inisiatif kemanusiaan secara faktual, berimbang, dan berorientasi pada penerima manfaat. Fokus pada masalah riil yang dihadapi komunitas, bentuk bantuan atau intervensi konkret, pelaksanaan di lapangan, serta dampak terukur yang didukung data valid, bukan sekadar memoles citra korporat.',
			news_angle:
				'- Dampak langsung terhadap kehidupan penerima manfaat (keluarga, komunitas, sekolah, lingkungan).\n- Nilai dan alokasi program (dana bantuan, beasiswa, fasilitas, sarana air bersih).\n- Skala dan jumlah penerima manfaat yang terdata.\n- Perubahan sosial atau ekonomi terukur sebelum dan sesudah program.\n- Kolaborasi lintas pihak (masyarakat, pemerintah daerah, yayasan mitra).\n- Respons dan suara penerima manfaat.',
			journalistic_style:
				'Gunakan gaya jurnalistik Indonesia yang lugas, informatif, human-centered, dan faktual. Bangun narasi berita yang mengalir dengan struktur piramida terbalik. Hindari gaya advertorial atau bahasa promosi yang mengagung-agungkan kebaikan perusahaan secara subjektif.',
			required_information:
				'- Nama resmi program sosial/CSR.\n- Penyelenggara program dan mitra pelaksana.\n- Lokasi pelaksanaan (desa/kecamatan/kota) dan tanggal kegiatan.\n- Target dan jumlah penerima manfaat konkret.\n- Bentuk riil bantuan (uang tunai, renovasi, pelatihan, sembako, dll).\n- Latar belakang masalah yang diselesaikan.\n- Kutipan narasumber (penerima manfaat atau perwakilan penyelenggara).\n- Sumber data capaian.',
			optional_information:
				'- Sejarah atau kesinambungan program dari tahun-tahun sebelumnya.\n- Rencana tindak lanjut atau monitoring berkala program.\n- Konteks geografis atau sosial wilayah setempat.\n- Komentar tokoh masyarakat atau pejabat daerah terkait.',
			structure_guidance:
				'Artikel dapat menggunakan alur berita piramida terbalik. Untuk rilis pendek (300–500 kata), jangan memaksakan heading (##/###) bertingkat. Susun paragraf yang saling terhubung secara padu. Jika materi mencakup multi-tahap yang panjang, gunakan subjudul deskriptif singkat tanpa mengorbankan kelancaran baca.',
			lead_guidance:
				'Lead harus langsung menyampaikan peristiwa utama: siapa penyelenggara, program apa yang diserahkan/dilaksanakan, siapa penerima manfaat, di mana lokasinya, dan fakta paling bernilai berita. Hindari membuka lead dengan kalimat motivasi generik, definisi CSR, atau pujian korporat.',
			fact_attribution_rules:
				"Bedakan secara tegas antara data independen dan klaim internal perusahaan. Jika angka penerima manfaat atau klaim dampak berasal dari materi rilis perusahaan, WAJIB sertakan atribusi wajar (misalnya 'menurut data perusahaan', 'pihak manajemen menyebutkan'). Jangan mengubah klaim target masa depan menjadi fakta yang sudah terjadi.",
			quote_guidance:
				'Gunakan kutipan yang memberikan perspektif baru atau penjelasan emosional/humanis dari penerima manfaat atau pemangku kepentingan. Jangan menggunakan kutipan hanya untuk formalitas. Dilarang keras mengarang kutipan palsu atau mengulang isi kutipan dalam kalimat narasi berikutnya.',
			language_tone:
				"Informatif, empatik, berbasis fakta, dan profesional. Hindari bahasa korporat yang kaku dan hindari superlatif tanpa bukti seperti 'sangat mulia', 'solusi paling sempurna', atau 'pemberian tanpa tandingan'.",
			what_to_avoid:
				"- Dilarang menggunakan bahasa promosi atau nada 'pencitraan' berlebihan.\n- Dilarang membuat angka atau persentase dampak tanpa dasar dari bahan.\n- Dilarang mengarang narasumber atau kutipan.\n- Dilarang meta-komentar (mis. 'berdasarkan kuesioner', 'dalam materi yang dibagikan').\n- Dilarang membuka kalimat dengan kata 'Di', 'Agar', atau 'Supaya'.\n- Dilarang repetisi frasa 'tidak hanya... tetapi juga'.",
			angle_variation_rules:
				'Ketika angle = Dampak Komunitas: fokuskan alur cerita pada perubahan nyata yang dirasakan masyarakat penerima.\nKetika angle = Pencapaian & Tonggak: tonjolkan angka, milestone, volume bantuan, dan kesinambungan program.\nKetika angle = Nilai Berita: tonjolkan unsur kebaruan, urgensi momentum, atau relevansi dengan isu sosial terkini.\nKetika angle = Solusi Masalah: buka dari persoalan riil komunitas dan bagaimana program ini menjadi jalan keluar konkret.\nPerubahan angle HANYA mengubah fokus narasi; fakta dasar, angka, dan standar jurnalistik TIDAK boleh berubah.',
			source_priority:
				'1. Jawaban kuesioner & fakta primer dari brief klien adalah kebenaran utama.\n2. Dokumen atau rilis sebelumnya menjadi referensi pelengkap.\n3. Jika terjadi perbedaan angka atau nama, jangan memilih asumsi sendiri; cantumkan atribusi yang jelas sesuai sumber primer.',
			fact_conflict_handling:
				'Jika ditemukan perbedaan data antara narasumber dan dokumen, gunakan angka resmi yang ber-atribusi. Jangan mencampur dua versi angka menjadi angka baru.',
			editorial_quality_checklist:
				'1. Lead langsung menjawab 5W+1H inti program.\n2. Semua angka penerima manfaat sesuai bahan baku.\n3. Klaim capaian ber-atribusi wajar.\n4. Kutipan akurat dan tidak diulang-ulang.\n5. Sudut pandang angle terlihat jelas di paragraf awal.\n6. Bahasa bersih dari jargon promosi berlebihan.'
		}
	},
	product_launch: {
		name: 'Launching Produk',
		sections: {
			content_purpose:
				'Menyampaikan informasi peluncuran produk atau layanan baru secara informatif, faktual, dan bernilai berita untuk konsumen atau industri terkait. Fokus pada kebaruan produk, spesifikasi/fitur utama, ketersediaan, harga, dan nilai gunanya bagi pengguna tanpa berubah menjadi iklan brosur atau advertorial jualan langsung.',
			news_angle:
				'- Spesifikasi dan inovasi fitur baru yang diperkenalkan.\n- Ketersediaan produk di pasar, tanggal rilis, dan informasi harga resmi.\n- Solusi konkret yang ditawarkan produk bagi masalah konsumen/pasar.\n- Target segmen pengguna atau industri yang disasar.\n- Latar belakang tren atau kebutuhan pasar yang melandasi peluncuran.',
			journalistic_style:
				'Gaya berita bisnis/teknologi yang lugas, presisi, dan objektif. Sajikan keunggulan produk sebagai fakta fitur dan kapabilitas teknis yang jelas, bukan sekadar rangkaian kata sifat atau pujian subjektif.',
			required_information:
				'- Nama resmi produk/layanan dan varian jika ada.\n- Nama perusahaan atau brand produsen.\n- Tanggal peluncuran dan kanal ketersediaan (toko fisik, e-commerce, aplikasi).\n- Fitur unggulan dan spesifikasi kunci.\n- Harga resmi atau skema langganan (jika tersedia).\n- Kutipan pimpinan perusahaan atau pengembang produk.',
			optional_information:
				'- Program promosi peluncuran (early bird, garansi khusus).\n- Perbandingan fitur dengan generasi sebelumnya.\n- Sertifikasi atau pengujian resmi yang telah dikantongi.',
			structure_guidance:
				'Piramida terbalik: Lead langsung mengumumkan peluncuran dan nilai utama produk, diikuti detail spesifikasi dan ketersediaan, kutipan pimpinan, dan penutup mengenai target pasar. Jangan memaksakan bullet list panjang jika kalimat narasi mengalir lebih baik.',
			lead_guidance:
				"Sampaikan langsung: nama brand meluncurkan produk X pada tanggal Y di Z dengan keunggulan utama W. Hindari kalimat pembuka klise seperti 'Seiring perkembangan zaman' atau 'PT X kembali memanjakan konsumen'.",
			fact_attribution_rules:
				"Klaim performa (seperti 'hemat energi hingga 50%', 'kecepatan 2x lipat') WAJIB ber-atribusi kepada pengujian internal atau keterangan resmi perusahaan ('perusahaan mengklaim', 'berdasarkan hasil uji internal'). Jangan tulis klaim performa sebagai fakta independen mutlak.",
			quote_guidance:
				'Kutipan pimpinan harus menjelaskan visi, alasan strategis, atau komitmen di balik produk, bukan sekadar membaca ulang daftar spesifikasi brosur. Jangan membuat kutipan fiktif.',
			language_tone:
				"Modern, profesional, informatif, dan ringkas. Hindari jargon marketing usang seperti 'solusi masa depan yang revolusioner' atau 'terobosan tiada tanding'.",
			what_to_avoid:
				"- Dilarang hard-selling seperti 'Beli sekarang juga di toko terdekat' atau 'Jangan lewatkan promo terbatas'.\n- Dilarang mengarang harga atau tanggal rilis jika tidak ada di bahan.\n- Dilarang membuat klaim superlatif tanpa dasar ('terbaik di dunia', 'satu-satunya di Indonesia' kecuali ada data riset resmi di bahan).",
			angle_variation_rules:
				'Ketika angle = Inovasi & Fitur: tekankan kebaruan teknologi dan spesifikasi teknis.\nKetika angle = Solusi Pengguna: tekankan masalah konsumen yang terpecahkan oleh hadirnya produk.\nKetika angle = Nilai Pasar & Tren: tekankan relevansi produk terhadap perkembangan industri dan tren konsumen.\nKetika angle = Ketersediaan & Peluncuran: tekankan momentum rilis, harga, dan kemudahan akses produk.',
			source_priority:
				'1. Lembar fakta spesifikasi resmi dari brief.\n2. Kutipan resmi juru bicara/pimpinan.\n3. Jangan mengimpor spesifikasi produk lain dari luar bahan.',
			fact_conflict_handling:
				'Jika ada perbedaan harga atau nama varian, gunakan varian nama resmi yang tertera pada lembar spesifikasi utama.',
			editorial_quality_checklist:
				'1. Nama produk, spesifikasi, dan harga tertulis presisi.\n2. Klaim efisiensi/performa memiliki atribusi yang jelas.\n3. Nada tulisan informatif dan bernilai berita, bukan teks sales.\n4. Front matter metadata terisi lengkap.'
		}
	},
	event: {
		name: 'Acara / Event',
		sections: {
			content_purpose:
				'Memberitakan penyelenggaraan acara (seminar, pameran, festival, konser, konferensi, atau perayaan) secara menarik, faktual, dan menyeluruh. Menginformasikan siapa penyelenggara, apa agenda utama, di mana dan kapan berlangsung, siapa pembicara/tamu penting yang hadir, serta capaian atau antusiasme peserta yang terlibat.',
			news_angle:
				'- Kemeriahan atau capaian jumlah peserta/pengunjung acara.\n- Gagasan penting, pengumuman strategis, atau wawasan dari pembicara utama.\n- Tema besar acara dan relevansinya bagi peserta/industri.\n- Rangkaian kegiatan unggulan (workshop, pameran, awarding, pertunjukan).\n- Komitmen bersama atau kesepakatan yang dihasilkan selama acara.',
			journalistic_style:
				"Bahasa jurnalistik dinamis, deskriptif faktual, dan kontekstual. Tangkap atmosfer kegiatan dengan fakta konkret (mis. 'dihadiri lebih dari 1.200 peserta dari 15 kota') tanpa tergelincir menjadi glorifikasi subjektif.",
			required_information:
				'- Nama resmi acara dan tema utama.\n- Penyelenggara acara dan pihak pendukung.\n- Tanggal, durasi, dan lokasi (gedung/kota/platform online).\n- Jumlah peserta atau pengunjung yang hadir.\n- Tokoh penting, pemateri, atau pejabat yang hadir/memberikan sambutan.\n- Kutipan inti sambutan narasumber utama.',
			optional_information:
				'- Agenda lanjutan atau rencana edisi acara di masa mendatang.\n- Booth pameran atau peserta sponsor penting.\n- Penghargaan atau pemenang kompetisi (jika ada sesi awarding).',
			structure_guidance:
				'Lead mengabarkan penyelenggaraan acara dan highlight terpenting. Paragraf berikutnya merinci tema dan angka partisipasi, lalu poin-poin pidato pembicara, highlight kegiatan, dan prospek ke depan.',
			lead_guidance:
				'Lead harus langsung menjawab 5W+1H: Acara apa, siapa yang menyelenggarakan/menghadiri, kapan dan di mana berlangsung, serta apa sorotan utama acara.',
			fact_attribution_rules:
				"Angka jumlah pengunjung atau nilai transaksi selama acara harus diatribusikan kepada panitia penyelenggara (misalnya 'menurut keterangan ketua panitia'). Jangan membulatkan angka secara sepihak jika bahan menyebut angka pasti.",
			quote_guidance:
				'Kutipkan pesan substantif dari pidato atau sambutan ketua panitia/tokoh utama yang mencerminkan tujuan acara. Hindari kutipan yang hanya berisi ucapan terima kasih standar.',
			language_tone: 'Energik, faktual, profesional, dan mengalir natural.',
			what_to_avoid:
				"- Dilarang melebih-lebihkan antusiasme dengan kata klise ('membuat jutaan pasang mata terbius', 'mengguncang jagat raya').\n- Dilarang mengarang nama tokoh yang tidak hadir di bahan.\n- Dilarang membuat kesimpulan buatan di akhir artikel.",
			angle_variation_rules:
				'Ketika angle = Gagasan & Wawasan: soroti intisari presentasi narasumber dan wawasan strategis.\nKetika angle = Antusiasme & Partisipasi: soroti jumlah peserta, keragaman daerah asal, dan atmosfer kegiatan.\nKetika angle = Momentum & Masa Depan: soroti deklarasi bersama atau dampak jangka panjang dari acara tersebut.',
			source_priority:
				'1. Catatan rilis pers dan sambutan resmi panitia.\n2. Rundown dan fakta pelaksanaan acara di brief.',
			fact_conflict_handling:
				'Jika ada perbedaan jam pelaksanaan atau lokasi, gunakan data yang tercatat pada laporan akhir penyelenggaraan.',
			editorial_quality_checklist:
				'1. Waktu dan lokasi acara akurat.\n2. Nama pejabat/pembicara dan jabatan dieja benar.\n3. Angka kehadiran peserta ber-atribusi panitia.\n4. Tidak ada pujian hiperbolis tanpa data.'
		}
	},
	personal_branding: {
		name: 'Personal Branding',
		sections: {
			content_purpose:
				'Mengulas profil tokoh, profesional, praktisi, atau pemimpin organisasi berdasarkan aktivitas nyata, keahlian, rekam jejak, dan kontribusi pemikirannya. Membangun kredibilitas publik berbasis bukti dan wawasan substantif, bukan kultus individu atau pencitraan kosong.',
			news_angle:
				'- Pandangan atau analisis tokoh terhadap isu industri terkini.\n- Perjalanan kepemimpinan dan pencapaian nyata organisasi yang dipimpin.\n- Penghargaan, sertifikasi, atau pengakuan profesi yang diraih.\n- Inisiatif atau terobosan kerja yang digagas oleh tokoh tersebut.\n- Prinsip kerja, etos profesional, dan kontribusi bagi masyarakat luas.',
			journalistic_style:
				'Gaya penulisan profil tokoh/fitur jurnalistik yang berwibawa, objektif, dan berorientasi fakta. Ceritakan rekam jejak melalui aksi nyata dan data konkret.',
			required_information:
				'- Nama lengkap tokoh beserta gelar/jabatan resmi saat ini.\n- Nama institusi, perusahaan, atau organisasi yang terafiliasi.\n- Bidang keahlian spesifik atau ranah industri.\n- Inisiatif, terobosan, atau pemikiran utama yang dibahas.\n- Kutipan pandangan atau refleksi dari tokoh bersangkutan.\n- Data pencapaian atau jejak rekam yang relevan.',
			optional_information:
				'- Latar belakang pendidikan atau pengalaman kepemimpinan masa lalu.\n- Penghargaan profesional yang relevan dengan topik bahasan.\n- Proyek atau target masa depan yang sedang dijalankan.',
			structure_guidance:
				'Buka dengan momentum kepemimpinan atau pandangan mutakhir sang tokoh, lalu elaborasi kontribusi nyatanya, kutipan gagasan, dan konteks relevansinya bagi pembaca industri.',
			lead_guidance:
				"Buka dengan tindakan konkret, pencapaian nyata, atau pandangan kuat tokoh terhadap tantangan terkini. Hindari pembukaan dengan biografi panjang dari masa kecil atau pujian subjektif seperti 'Sosok panutan yang luar biasa'.",
			fact_attribution_rules:
				"Gunakan atribusi langsung terhadap opini atau pandangan sang tokoh ('menurutnya', 'ia menegaskan bahwa'). Jangan menjadikan opini pribadi tokoh sebagai fakta sains atau hukum tanpa verifikasi independen.",
			quote_guidance:
				'Pilih kutipan pemikiran yang menunjukkan kedalaman analisis, motivasi kerja, atau visi strategis. Jangan menggunakan kutipan basa-basi.',
			language_tone: 'Inspiratif, profesional, santun, dan objektif.',
			what_to_avoid:
				'- Dilarang kultus individu, sanjungan berlebihan, atau bahasa propaganda.\n- Dilarang memasukkan data pribadi sensitif (alamat rumah, nomor HP pribadi, NIK).\n- Dilarang membuat klaim kesuksesan tanpa bukti data atau konteks pekerjaan.',
			angle_variation_rules:
				'Ketika angle = Pemikiran Strategis: fokuskan pada analisis tokoh terhadap tren industri.\nKetika angle = Rekam Jejak & Terobosan: fokuskan pada proyek konkret dan transformasi yang dihasilkan.\nKetika angle = Nilai Kepemimpinan: fokuskan pada budaya kerja dan pendekatan pemberdayaan tim.',
			source_priority:
				'1. Transkrip wawancara/pernyataan langsung sang tokoh.\n2. Profil institusional resmi dan data portofolio dari brief.',
			fact_conflict_handling: 'Gunakan jabatan resmi terkini dan ejaan nama lengkap yang terverifikasi.',
			editorial_quality_checklist:
				'1. Nama dan jabatan resmi tokoh tertulis akurat.\n2. Gagasan dan pemikiran tersampaikan dengan bernas.\n3. Bebas dari sanjungan hiperbolis.\n4. Kerahasiaan data pribadi terjaga.'
		}
	},
	press_release: {
		name: 'Siaran Pers Umum',
		sections: {
			content_purpose:
				'Menyampaikan pengumuman korporat, perkembangan organisasi, kerja sama bisnis, atau laporan kinerja secara ringkas, faktual, dan siap diterbitkan media massa. Memberikan informasi yang jelas bagi jurnalis dan pembaca umum.',
			news_angle:
				'- Poin penting pengumuman dan pengaruhnya bagi industri/publik.\n- Angka pertumbuhan atau kinerja bisnis yang dirilis resmi.\n- Penandatanganan kerja sama (MoU/kemitraan) dan ruang lingkupnya.\n- Perubahan kepengurusan atau ekspansi operasional perusahaan.\n- Tanggapan resmi manajemen terhadap dinamika pasar.',
			journalistic_style:
				'Bahasa jurnalistik berita lempang (straight news) yang ringkas, presisi, dan netral. Paragraf pendek (2–3 kalimat per paragraf), tanpa kalimat berbunga-bunga, dan langsung fokus pada fakta.',
			required_information:
				'- Tanggal dan kota rilis (dateline).\n- Nama lengkap perusahaan/organisasi yang merilis.\n- Poin utama pengumuman (what, why, when, where, who, how).\n- Angka atau data pendukung yang valid.\n- Pernyataan resmi juru bicara atau direksi perusahaan.',
			optional_information:
				'- Sekilas profil singkat perusahaan (boilerplate text) di bagian akhir rilis.\n- Rencana strategis lanjutan dalam 1–2 kuartal ke depan.',
			structure_guidance:
				'Struktur straight news murni: Dateline + Lead (5W+1H) -> Paragraf penjelas data -> Kutipan manajemen -> Dampak/langkah berikutnya -> Boilerplate singkat.',
			lead_guidance:
				"Lead harus langsung menjawab unsur pokok berita dalam kalimat pertama atau kedua. Contoh: 'JAKARTA — PT ABC resmi menandatangani kerja sama strategis dengan XYZ untuk memperluas jaringan logistik...'",
			fact_attribution_rules:
				'Seluruh pernyataan subjektif, target pendapatan, atau proyeksi masa depan WAJIB ber-atribusi kepada juru bicara perusahaan. Bedakan angka realisasi masa lalu dengan target perkiraan.',
			quote_guidance:
				'Kutipan harus mewakili sikap resmi korporat. Satu kutipan padat dari pejabat berwenang lebih berharga dibanding beberapa kutipan panjang tanpa inti.',
			language_tone: 'Formal, netral, tegas, dan berwibawa.',
			what_to_avoid:
				"- Dilarang bahasa bombastis ('merajai pasar', 'tak terkalahkan').\n- Dilarang membuat data finansial palsu.\n- Dilarang meta-komentar tentang dokumen rilis pers.",
			angle_variation_rules:
				'Ketika angle = Dampak Bisnis: soroti pertumbuhan ekonomi dan efisiensi operasional.\nKetika angle = Kolaborasi Strategis: soroti sinergi antar pihak dan komitmen bersama.\nKetika angle = Ekspansi Layanan: soroti jangkauan wilayah baru dan kemudahan bagi konsumen.',
			source_priority:
				'1. Keterangan rilis resmi dan angka yang tercantum di brief.\n2. Laporan keterbukaan informasi publik jika relevan.',
			fact_conflict_handling: 'Gunakan angka yang tercantum pada lembar audit atau rilis resmi terbaru.',
			editorial_quality_checklist:
				'1. Dateline dan nama entitas akurat.\n2. Format 5W+1H terpenuhi di paruh awal artikel.\n3. Kutipan manajemen tepat dan berbobot.\n4. Tidak ada klaim tanpa atribusi.'
		}
	}
};
