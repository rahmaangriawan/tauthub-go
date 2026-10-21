/**
 * Contoh isian kotak "Instruksi Tambahan / Fokus Cerita" di modal naskah writer,
 * dipakai tombol "Contoh" agar klien tidak menulis instruksi dari nol.
 *
 * Kunci peta = slug angle dari tabel `taut_content_angles` (admin → Prompt
 * Angle), jadi mencocokkan pilihan angle klien di Jalur Distribusi. Angle baru
 * yang belum punya contoh di sini otomatis memakai contoh cadangan.
 *
 * Teks dibedakan per jenis media: `mainstream` memakai kaidah straight news
 * (tanpa heading, tanpa daftar berpoin, tanpa bahasa promosi) dan `blogger`
 * memakai gaya blog yang lebih santai.
 *
 * Sengaja tidak dimasukkan ke `phrases.ts`: ini naskah contoh berbahasa
 * Indonesia untuk artikel berbahasa Indonesia (bukan label antarmuka), jadi
 * tidak perlu padanan Inggris. Contoh juga tidak menyebut merek, angka, atau
 * tanggal tertentu — fakta tetap diambil AI dari Copilot/Fakta, bukan dari
 * instruksi ini.
 */

export type MediaKind = 'mainstream' | 'blogger';

/** Contoh cadangan untuk angle yang belum punya contoh sendiri. */
export const GENERIC_EXAMPLES: Record<MediaKind, string> = {
	mainstream:
		'Angkat sudut pembahasan sesuai angle yang sudah dipilih untuk media ini, lalu kembangkan menjadi berita yang informatif dan berimbang untuk pembaca media nasional Indonesia. Pakai format straight news dengan struktur piramida terbalik: awali lead yang memuat inti kabar dan informasi terpenting, lanjutkan dengan penjelasan pendukung secara berurutan kepentingannya. Terapkan 5W+1H secara alami — siapa, apa, kapan, di mana, mengapa, dan bagaimana. Jangan memakai heading atau subjudul di antara paragraf isi, hindari daftar berpoin, gaya artikel SEO, bahasa promosi, slogan pemasaran, serta klaim keunggulan yang tidak didukung bukti. Kalau ada kutipan langsung, hanya dari pernyataan asli yang dapat diverifikasi; jangan mengarang kutipan, nama narasumber, tanggal, lokasi, harga, maupun angka. Akhiri dengan informasi faktual yang relevan bagi pembaca, tanpa kesimpulan yang mengulang isi artikel.',
	blogger:
		'Angkat sudut pembahasan sesuai angle yang sudah dipilih untuk media ini, lalu tulis sebagai artikel blog yang santai tapi tetap berbasis fakta, dengan pembaca umum sebagai sasaran. Boleh memakai subjudul pendek dan daftar seperlunya supaya mudah dipindai, tapi jaga paragraf tetap ringkas dan alurnya mengalir. Jelaskan konteksnya lebih dulu, baru masuk ke poin-poin bermanfaat bagi pembaca. Hindari bahasa promosi berlebihan, klaim tanpa bukti, dan gaya artikel SEO yang dipaksa. Jangan mengarang fakta, angka, nama narasumber, atau kutipan — kalau datanya belum ada, tulis secara umum saja. Tutup dengan rangkuman praktis yang bisa langsung dipakai pembaca.'
};

export const ANGLE_EXAMPLES: Record<string, Record<MediaKind, string>> = {
	product_launch: {
		mainstream:
			'Tulis berita jurnalistik mengenai peluncuran produk (atau fitur baru) ini untuk pembaca media nasional Indonesia. Gunakan format straight news dengan struktur piramida terbalik: buat judul yang faktual dan menarik, lalu awali dengan lead yang menjelaskan inti peluncuran dan informasi terpenting bagi pembaca. Bahas pembaruan dibandingkan versi sebelumnya, fitur utama, spesifikasi, manfaat bagi pengguna, serta ketersediaan produk di Indonesia bila informasinya dapat diverifikasi. Terapkan prinsip 5W+1H secara alami, termasuk siapa yang meluncurkan, apa yang baru, kapan dan di mana peluncuran berlangsung, alasan pengembangan bila diketahui, serta bagaimana fitur tersebut bekerja. Kembangkan dengan paragraf jurnalistik yang ringkas, objektif, dan natural; jangan memakai heading atau subjudul, hindari daftar berpoin, bahasa promosi, slogan pemasaran, serta klaim keunggulan tanpa bukti. Gunakan kutipan langsung hanya bila ada pernyataan asli yang dapat diverifikasi dari perusahaan atau narasumber terkait, dan jangan mengarang kutipan, nama, tanggal, lokasi, harga, spesifikasi, maupun data penjualan. Akhiri dengan informasi faktual yang relevan bagi pembaca — bukan advertorial terselubung.',
		blogger:
			'Perkenalkan produk (atau fitur baru) ini lewat artikel blog yang mengalir dan enak dibaca: mulai dari masalah atau kebiasaan pembaca yang relevan, baru masuk ke apa yang ditawarkan produk ini dan apa bedanya dari versi sebelumnya. Bahas fitur utama, cara kerjanya, dan untuk siapa produk ini paling masuk akal, pakai subjudul pendek supaya mudah dipindai. Tetap faktual: jangan mengarang spesifikasi, harga, tanggal rilis, atau klaim keunggulan — tulis apa adanya sesuai informasi yang tersedia, dan sebutkan kalau detail tertentu belum diumumkan. Hindari bahasa promosi berlebihan dan jangan menyalin slogan. Tutup dengan catatan praktis: siapa yang sebaiknya mempertimbangkan produk ini dan hal apa yang perlu dicek sebelum membeli.'
	},
	product_review: {
		mainstream:
			'Tulis ulasan jurnalistik mengenai produk ini untuk pembaca media nasional Indonesia, berbasis pengalaman pemakaian atau data pengujian yang benar-benar tersedia. Fokuskan pada apa yang benar-benar berubah dibanding produk sejenis: kualitas, kegunaan, dan siapa yang paling terbantu, bukan pujian. Sebutkan kelebihan sekaligus keterbatasan secara proporsional dan berimbang, dengan alasan yang bisa diverifikasi. Jangan memakai heading atau subjudul di antara paragraf, hindari daftar berpoin dan gaya artikel SEO. Jangan mengarang skor, angka penjualan, kutipan, nama narasumber, harga, atau spesifikasi; bedakan jelas antara fakta resmi dan penilaian. Akhiri dengan kesimpulan faktual: untuk kebutuhan apa produk ini layak dipertimbangkan dan apa risikonya.',
		blogger:
			'Tulis ulasan personal yang jujur mengenai produk ini untuk pembaca blog: ceritakan konteks pemakaiannya (untuk kebutuhan apa, berapa lama dipakai), lalu bahas apa yang terasa menyenangkan dan apa yang mengganggu. Urutkan per bagian dengan subjudul pendek — misalnya pengalaman pertama, pemakaian harian, dan catatan minusnya — dan boleh memakai daftar poin untuk plus-minus. Jangan mengarang angka, spesifikasi, harga, atau pengalaman yang tidak ada; kalau baru mencoba sebentar, katakan apa adanya. Hindari nada iklan dan klaim berlebihan. Tutup dengan rekomendasi praktis: produk ini cocok untuk siapa, dan siapa sebaiknya memilih alternatif lain.'
	},
	unboxing: {
		mainstream:
			'Liput pengalaman membuka dan mencoba produk ini secara jurnalistik: apa isi paketnya, bagaimana kondisi saat diterima, dan kesan pertama pemakaian yang benar-benar teramati. Susun sebagai straight news ringan berstruktur piramida terbalik — hal paling penting di awal, lalu rincian pendukung. Terapkan 5W+1H secara natural dan tetap objektif: sebutkan keterbatasan pengamatan bila memang baru dicoba sebentar, jangan menyimpulkan daya tahan atau performa jangka panjang. Tanpa heading atau subjudul, tanpa daftar berpoin, tanpa bahasa promosi. Jangan mengarang isi paket, harga, tanggal, maupun spesifikasi. Akhiri dengan informasi faktual yang berguna bagi pembaca, misalnya hal yang perlu diperhatikan sebelum membeli.',
		blogger:
			'Ceritakan pengalaman membuka produk ini seperti bercerita ke teman: bagaimana paketnya datang, apa saja isinya, dan apa reaksi pertama saat dipakai. Boleh pakai subjudul pendek dan daftar isi paket supaya rapi, serta foto/kesan detail yang terasa konkret. Tetap jujur dan spesifik: sebutkan bagian yang terasa bagus maupun kurang, dan jangan mengarang isi maupun spesifikasi yang belum terverifikasi. Hindari nada iklan dan klaim berlebihan. Tutup dengan kesan awal yang jujur plus hal yang akan kamu amati pada pemakaian berikutnya.'
	},
	comparison: {
		mainstream:
			'Tulis perbandingan jurnalistik antara produk ini dengan alternatif yang relevan untuk pembaca media nasional Indonesia. Bandingkan pada kriteria yang bisa diverifikasi — fungsi, spesifikasi, kemudahan pemakaian, ketersediaan, dan biaya bila datanya ada — dan sebutkan secara jelas dasar penilaiannya. Sajikan berimbang: jangan menjatuhkan pihak lain tanpa bukti, dan jangan menyimpulkan pemenang mutlak bila keunggulannya bergantung pada kebutuhan pembaca. Gunakan format straight news tanpa heading atau subjudul dan tanpa daftar berpoin; kalau perbandingannya rumit, jelaskan dalam paragraf yang runtut. Jangan mengarang angka, harga, atau hasil pengujian. Akhiri dengan panduan faktual: dalam kondisi apa masing-masing pilihan lebih masuk akal.',
		blogger:
			'Buat perbandingan yang mudah dicerna antara produk ini dan alternatifnya, dari sudut pandang pembaca yang sedang bingung memilih. Tetapkan dulu kriterianya (misal harga, kemudahan, fitur yang sering dipakai), lalu bahas satu per satu dengan subjudul pendek atau daftar poin plus-minus. Jujur soal kekurangan produk yang dibahas, dan jangan mengarang spesifikasi maupun harga. Hindari kesimpulan menggantung: beri rekomendasi yang jelas tapi bersyarat, misalnya untuk kebutuhan A pilih yang ini, untuk kebutuhan B pertimbangkan alternatif itu.'
	},
	how_to: {
		mainstream:
			'Tulis panduan praktis bergaya jurnalistik mengenai cara melakukan hal ini, untuk pembaca media nasional Indonesia. Mulai dengan lead yang menyatakan manfaat dan siapa yang membutuhkannya, lalu jelaskan langkah-langkahnya secara berurutan dalam paragraf yang mengalir. Sebutkan bahan, alat, biaya, atau waktu bila datanya tersedia dan dapat diverifikasi; bila tidak, tulis secara umum tanpa menebak angka. Jangan memakai heading atau subjudul, hindari daftar berpoin dan gaya artikel SEO. Jangan mengarang angka, nama narasumber, atau kutipan. Tutup dengan catatan faktual yang relevan: hal yang sering salah dilakukan dan hal yang perlu diwaspadai.',
		blogger:
			'Tulis panduan langkah demi langkah yang ramah pembaca blog: mulai dari apa yang perlu disiapkan, lalu urutkan prosesnya dengan subjudul pendek atau daftar bernomor supaya gampang diikuti. Sisipkan tips praktis dari pengalaman, dan sebutkan kesalahan umum beserta cara menghindarinya. Tetap berbasis fakta: jangan mengarang angka, takaran, harga, atau klaim hasil yang pasti. Hindari bahasa promosi dan jangan memaksa menyisipkan penawaran. Tutup dengan ringkasan cepat yang bisa diikuti pembaca tanpa membuka ulang seluruh artikel.'
	},
	listicle: {
		mainstream:
			'Susun artikel bergaya daftar pilihan yang tetap jurnalistik: sebutkan beberapa opsi relevan beserta alasan masing-masing dipilih, berdasarkan kriteria yang jelas dan bisa diverifikasi seperti fungsi, keandalan, atau kecocokan kebutuhan. Untuk tiap pilihan, jelaskan untuk siapa opsi itu paling pas dan apa keterbatasannya, bukan sekadar menyebut keunggulan. Karena kaidah media nasional biasanya melarang daftar berpoin dan subjudul, sajikan urutannya di dalam paragraf yang mengalir dengan penanda alami. Jangan mengarang angka, harga, atau peringkat yang tidak berdasar. Akhiri dengan panduan memilih yang faktual sesuai kebutuhan pembaca.',
		blogger:
			'Buat artikel daftar yang enak dibaca: pilih beberapa opsi benar-benar relevan dengan kebutuhan pembaca, lalu bahas singkat tiap pilihan dengan subjudul atau daftar bernomor — apa keunggulannya, untuk siapa cocok, dan di mana batasnya. Urutkan berdasarkan kriteria yang masuk akal dan sebutkan kriterianya di awal supaya tidak terkesan asal. Jangan mengarang spesifikasi, harga, atau klaim populeritas. Hindari nada iklan. Tutup dengan satu rekomendasi utama dan alasan singkatnya, plus alternatif bila kebutuhan pembaca berbeda.'
	},
	niche_education: {
		mainstream:
			'Tulis artikel edukatif jurnalistik yang menjelaskan konsep ini kepada pembaca awam media nasional Indonesia. Awali dengan lead yang menyatakan mengapa konsep ini relevan sekarang, lalu jelaskan definisinya dengan bahasa lugas, contoh konkret, dan konteks pemakaiannya di lapangan. Kalau ada istilah teknis, jelaskan sekali dengan padanan sederhana. Gunakan gaya straight news tanpa heading atau subjudul dan tanpa daftar berpoin, paragraf pendek yang mengalir. Jangan mengarang data, angka, studi, atau kutipan pakar; kalau sumbernya belum tersedia, tulis penjelasan umum tanpa atribusi palsu. Akhiri dengan implikasi praktis yang bisa dipahami pembaca.',
		blogger:
			'Jelaskan konsep ini dengan bahasa sehari-hari seolah menjelaskan ke teman yang baru pertama kali dengar. Mulai dari masalah atau pertanyaan yang sering muncul, lalu uraikan pengertiannya, contoh nyatanya, dan kesalahpahaman yang lazim. Pakai subjudul pendek dan analogi sederhana supaya mudah dicerna; daftar poin boleh dipakai untuk merangkum. Jangan mengarang angka, riset, atau nama pakar. Hindari jargon berlebihan dan nada menggurui. Tutup dengan rangkuman singkat plus langkah pertama yang bisa dicoba pembaca.'
	},
	research_data_trends: {
		mainstream:
			'Tulis laporan jurnalistik mengenai data dan tren ini untuk pembaca media nasional Indonesia. Mulai dengan lead yang memuat temuan terpenting beserta angkanya dan periode datanya, lalu jelaskan sumber data, metodologi singkat, serta keterbatasannya supaya pembaca bisa menilai sendiri. Kaitkan temuan dengan konteks industri dan apa artinya bagi pembaca atau pelaku usaha. Gunakan struktur piramida terbalik tanpa heading atau subjudul dan tanpa daftar berpoin. Jangan mengarang angka, sumber, tanggal, atau kutipan; bedakan jelas antara temuan data dan analisis. Akhiri dengan informasi faktual yang relevan, termasuk hal yang masih perlu dipantau.',
		blogger:
			'Bahas data dan tren ini dengan gaya blog yang membumi: mulai dari angka atau temuan yang paling menarik perhatian, lalu jelaskan apa maksudnya dalam bahasa sederhana dan mengapa itu penting bagi pembaca. Pecah jadi beberapa bagian dengan subjudul, dan boleh memakai daftar poin untuk menyorot temuan utama. Sebutkan sumber dan periode datanya, dan jujur soal keterbatasan data. Jangan mengarang angka, prediksi, atau kutipan yang terdengar resmi. Tutup dengan kesimpulan praktis: apa yang bisa dilakukan pembaca menghadapi tren ini.'
	},
	thought_leadership: {
		mainstream:
			'Tulis artikel opini atau gagasan strategis dari sudut pandang pemimpin di bidang ini untuk pembaca media nasional Indonesia. Buka dengan tesis yang jelas tentang isu yang sedang dihadapi industri, lalu dukung dengan argumen yang runtut, contoh nyata, dan konteks yang bisa diverifikasi. Posisikan sebagai pandangan yang argumentatif dan transparan — sebutkan bahwa ini pendapat, bukan laporan fakta — dan akui hal yang belum pasti. Tanpa heading atau subjudul dan tanpa daftar berpoin. Jangan mengarang data, studi, atau kutipan untuk memperkuat argumen. Akhiri dengan rekomendasi atau pertanyaan terbuka yang relevan bagi pembaca.',
		blogger:
			'Tulis gagasan atau opini yang personal dan argumentatif tentang isu ini, dengan sudut pandang yang jelas sejak awal. Kembangkan satu ide utama per bagian memakai subjudul pendek, dan dukung dengan contoh atau pengalaman yang benar-benar ada. Transparan bahwa ini pandangan pribadi, bukan berita, dan jujur soal asumsi yang kamu pakai. Boleh memakai daftar poin untuk merangkum alasan. Jangan mengarang data atau kutipan. Tutup dengan ajakan berdiskusi atau langkah konkret yang bisa pembaca ambil.'
	},
	success_case: {
		mainstream:
			'Tulis studi kasus jurnalistik tentang pengalaman ini: siapa yang terlibat, masalah apa yang dihadapi, langkah yang diambil, dan hasil yang benar-benar tercatat. Buka dengan lead yang memuat hasil terpenting, lalu paparkan kronologinya secara runtut dengan angka atau waktu bila tersedia dan dapat diverifikasi. Sebutkan juga kendala dan hal yang belum berhasil supaya berimbang, serta bedakan antara klaim pihak terkait dan temuan yang bisa diperiksa. Gunakan format straight news tanpa heading atau subjudul dan tanpa daftar berpoin. Jangan mengarang hasil, angka, nama, atau kutipan. Akhiri dengan pelajaran faktual yang bisa dipakai pembaca di konteks lain.',
		blogger:
			'Ceritakan kisah keberhasilan ini secara mengalir: mulai dari kondisi sebelum perubahan, apa yang dicoba, rintangan yang muncul, dan hasil akhirnya. Pakai subjudul pendek untuk membagi tahapan, dan sisipkan rincian konkret yang membuat ceritanya terasa nyata — tanpa mengarang angka atau detail. Jujur soal bagian yang tidak berjalan mulus, karena itu yang paling berguna bagi pembaca. Hindari nada promosi. Tutup dengan pelajaran utama yang bisa ditiru pembaca sesuai skala dan konteks mereka sendiri.'
	},
	inspirational_story: {
		mainstream:
			'Tulis kisah inspiratif jurnalistik tentang orang atau komunitas ini untuk pembaca media nasional Indonesia. Bangun dari fakta: latar belakang, titik balik, pilihan sulit yang dihadapi, dan apa yang sudah dicapai, dengan kronologi yang jelas dan kutipan langsung hanya bila ada pernyataan asli yang dapat diverifikasi. Hindari menggombal atau mendramatisasi di luar fakta, dan jangan memakai heading atau subjudul di antara paragraf. Sebutkan konteks sosial atau ekonomi yang membuat perjalanan ini berarti, tanpa mengarang angka maupun nama. Akhiri dengan informasi atau pelajaran faktual yang relevan bagi pembaca.',
		blogger:
			'Tulis kisah inspiratif dengan gaya bercerita yang hangat: mulai dari momen yang paling menarik perhatian, lalu susun perjalanannya lewat subjudul pendek — latar belakang, titik sulit, dan apa yang berubah. Sisipkan detail manusiawi yang konkret tanpa mengarang fakta, nama, angka, atau kutipan. Hindari kesan menggurui dan bahasa berlebihan. Tutup dengan pelajaran yang bisa dipakai pembaca dalam skala mereka sendiri.'
	},
	funding_announcement: {
		mainstream:
			'Tulis berita jurnalistik mengenai pengumuman pendanaan atau investasi ini untuk pembaca media nasional Indonesia. Awali dengan lead yang memuat inti kabar: siapa yang mendapatkan dana, dari siapa, berapa nilainya, dan untuk apa dana itu dipakai — sejauh informasinya dapat diverifikasi. Lanjutkan dengan konteks: posisi perusahaan di industrinya, tahap pendanaan, rencana penggunaan, dan implikasi bagi pasar serta tenaga kerja. Terapkan struktur piramida terbalik dan 5W+1H secara alami, tanpa heading atau subjudul dan tanpa daftar berpoin. Jangan mengarang nilai investasi, nama investor, tanggal, atau kutipan; bedakan fakta resmi dari analisis. Akhiri dengan informasi faktual tentang langkah berikutnya atau hal yang perlu dipantau pembaca.',
		blogger:
			'Bahas kabar pendanaan ini dengan gaya blog yang menjelaskan: mulai dari ringkasan siapa dibiayai, oleh siapa, dan berapa besarannya, lalu jelaskan mengapa ini relevan bagi pembaca atau industri terkait. Uraikan dengan subjudul pendek: konteks perusahaan, rencana pemakaian dana, dan apa artinya untuk pengguna. Jangan mengarang angka, nama investor, atau prediksi masa depan. Boleh memakai daftar poin untuk merangkum. Tutup dengan hal praktis yang bisa pembaca perhatikan setelah pendanaan ini.'
	},
	strategic_partnership_b2b: {
		mainstream:
			'Tulis berita jurnalistik mengenai kemitraan strategis ini untuk pembaca media nasional Indonesia. Buka dengan lead yang memuat inti kerja sama: siapa bermitra dengan siapa, lingkup kerja samanya, dan tujuan yang dinyatakan — hanya sejauh informasinya dapat diverifikasi. Jelaskan konteks bisnis kedua pihak, alasan kemitraan ini masuk akal dari sisi pasar, serta dampaknya bagi pelanggan, mitra lain, atau persaingan industri. Susun sebagai straight news berstruktur piramida terbalik tanpa heading atau subjudul dan tanpa daftar berpoin. Jangan mengarang nilai kontrak, target, tanggal, atau kutipan. Akhiri dengan informasi faktual soal tahapan berikutnya atau hal yang masih belum jelas.',
		blogger:
			'Jelaskan kemitraan ini dengan gaya blog yang membumi: apa yang sebenarnya dilakukan kedua pihak, dan apa bedanya bagi pengguna atau pelaku usaha kecil. Susun dengan subjudul pendek — siapa bermitra, apa isi kerja samanya, dan apa dampaknya di lapangan. Jangan mengarang angka, target, atau kutipan resmi. Sebutkan hal yang belum diumumkan secara apa adanya. Tutup dengan catatan praktis: apa yang pantas ditunggu pembaca dari kerja sama ini.'
	},
	milestone_awards: {
		mainstream:
			'Tulis berita jurnalistik mengenai pencapaian atau penghargaan ini untuk pembaca media nasional Indonesia. Awali dengan lead yang menyebut pencapaiannya, siapa yang memberi pengakuan (bila ada), dan konteksnya, lalu jelaskan apa yang sebenarnya diukur atau dinilai sehingga pembaca paham artinya. Sebutkan data pendukung seperti tahun berdiri, jumlah layanan, atau pencapaian sebelumnya bila tersedia dan dapat diverifikasi. Gunakan straight news berstruktur piramida terbalik, tanpa heading atau subjudul, tanpa daftar berpoin, dan tanpa bahasa promosi. Jangan mengarang angka, nama penghargaan, tanggal, atau kutipan. Akhiri dengan informasi faktual tentang langkah berikutnya, bukan pujian.',
		blogger:
			'Ceritakan pencapaian ini dengan gaya blog: apa yang diraih, bagaimana prosesnya, dan mengapa itu berarti bagi tim atau pengguna. Pakai subjudul pendek dan rincian konkret (bukan slogan), misalnya hal yang paling sulit dilewati. Jangan mengarang angka, nama penghargaan, atau kutipan. Hindari nada menyombongkan diri. Tutup dengan apa yang akan dilakukan berikutnya, supaya pembaca tahu ini bukan akhir cerita.'
	},
	c_level_change: {
		mainstream:
			'Tulis berita jurnalistik mengenai pergantian manajemen puncak ini untuk pembaca media nasional Indonesia. Buka dengan lead yang menyebut posisi, siapa yang masuk dan siapa yang keluar, efektif sejak kapan, dan alasan yang dinyatakan resmi. Lanjutkan dengan rekam jejak singkat yang relevan, konteks perusahaan saat ini, serta apa yang biasanya berubah setelah pergantian seperti ini bagi karyawan, mitra, atau pelanggan. Gunakan struktur straight news piramida terbalik, tanpa heading atau subjudul dan tanpa daftar berpoin. Jangan mengarang alasan, tanggal, nama, atau kutipan; bedakan pernyataan resmi dari analisis pengamat. Akhiri dengan informasi faktual tentang agenda atau keputusan yang ditunggu berikutnya.',
		blogger:
			'Jelaskan pergantian manajemen puncak ini dengan gaya blog yang mudah diikuti: siapa yang berubah, sejak kapan, dan apa artinya untuk orang di luar perusahaan. Susun dengan subjudul pendek — siapa orangnya, kenapa penting, dan apa yang mungkin berubah. Jujur bila informasinya masih terbatas, dan jangan mengarang alasan atau kutipan. Tutup dengan hal yang perlu dipantau pembaca dalam beberapa bulan ke depan.'
	},
	rebranding_transformation: {
		mainstream:
			'Tulis berita jurnalistik mengenai rebranding atau transformasi bisnis ini untuk pembaca media nasional Indonesia. Awali lead dengan apa yang berubah (nama, logo, posisi, atau layanan), kapan berlaku, dan alasan resmi di baliknya. Lanjutkan dengan konteks: apa yang mendorong transformasi, apa yang tetap sama, dan implikasi praktisnya bagi pelanggan, mitra, atau karyawan — misalnya perubahan kontrak, aplikasi, atau layanan lama yang dihentikan. Gunakan straight news tanpa heading atau subjudul dan tanpa daftar berpoin. Jangan mengarang alasan, angka, tanggal, atau kutipan. Akhiri dengan informasi faktual tentang langkah berikutnya dari perusahaan.',
		blogger:
			'Ceritakan perubahan identitas atau transformasi ini dengan gaya blog: apa yang berubah, apa yang tetap, dan apa artinya buat pengguna atau pembaca yang sudah mengenal brand ini. Pakai subjudul pendek untuk memisahkan apa yang baru dan apa yang perlu kamu lakukan sebagai pengguna. Jangan mengarang alasan atau target perusahaan. Hindari bahasa korporat yang kaku. Tutup dengan langkah praktis yang perlu diperhatikan pengguna setelah perubahan ini.'
	},
	csr_sustainability: {
		mainstream:
			'Tulis berita jurnalistik mengenai inisiatif CSR atau keberlanjutan ini untuk pembaca media nasional Indonesia. Buka dengan lead yang memuat program, sasaran, lokasi, dan dampak yang benar-benar terukur bila datanya tersedia. Lanjutkan dengan konteks masalah yang hendak dijawab, pihak yang terlibat, serta mekanisme programnya, dan sebutkan apa yang masih perlu dinilai dari waktu ke waktu. Gunakan struktur straight news piramida terbalik, tanpa heading atau subjudul dan tanpa daftar berpoin. Jangan mengarang angka dampak, jumlah penerima manfaat, anggaran, atau kutipan. Akhiri dengan informasi faktual tentang tahapan berikutnya atau cara program dipantau.',
		blogger:
			'Bahas inisiatif ini dengan gaya blog yang dekat dengan pembaca: masalah apa yang ingin diselesaikan, siapa yang terlibat, dan bagaimana programnya berjalan di lapangan. Susun dengan subjudul pendek dan rincian konkret, bukan jargon keberlanjutan. Jujur bahwa dampaknya butuh waktu dinilai, dan jangan mengarang angka atau klaim lingkungan. Tutup dengan hal praktis yang bisa dilakukan pembaca untuk mendukung atau mengikuti isu ini.'
	},
	community_event: {
		mainstream:
			'Liput acara komunitas ini secara jurnalistik untuk pembaca media nasional Indonesia. Buka dengan lead yang menyebut acara, penyelenggara, waktu, lokasi, dan tujuan pelaksanaannya, lalu jelaskan siapa yang terlibat, kegiatan yang benar-benar berlangsung, dan respons peserta yang teramati. Kaitkan dengan isu yang relevan bagi masyarakat sekitar tanpa melebih-lebihkan. Gunakan format straight news tanpa heading atau subjudul dan tanpa daftar berpoin; kutipan hanya bila ada pernyataan asli yang dapat diverifikasi. Jangan mengarang jumlah peserta, angka, nama, atau kutipan. Akhiri dengan informasi faktual tentang rencana lanjutan kegiatan.',
		blogger:
			'Ceritakan acara komunitas ini dengan gaya blog yang hangat: apa yang terjadi, siapa yang hadir, dan bagian mana yang paling berkesan. Pakai subjudul pendek untuk memisahkan rangkaian acara, dan sisipkan detail konkret yang kamu amati sendiri. Jangan mengarang jumlah peserta, nama, atau kutipan. Hindari nada laporan resmi dan bahasa promosi. Tutup dengan kesan atau pelajaran singkat serta ajakan bagi pembaca yang ingin ikut kegiatan serupa.'
	},
	large_event_coverage: {
		mainstream:
			'Liput event atau konferensi berskala besar ini secara jurnalistik untuk pembaca media nasional Indonesia. Mulai dengan lead yang memuat inti acara: apa yang dibahas, siapa penyelenggara dan pembicaranya, kapan dan di mana berlangsung, serta mengapa isunya penting sekarang. Lanjutkan dengan pokok-pokok pembahasan terpenting dan hasil yang diumumkan, hanya sejauh dapat diverifikasi. Gunakan struktur piramida terbalik, tanpa heading atau subjudul dan tanpa daftar berpoin; terapkan 5W+1H secara alami. Jangan mengarang jumlah peserta, angka, kutipan, atau agenda yang tidak resmi. Akhiri dengan informasi faktual tentang tindak lanjut atau hal yang perlu dipantau pembaca.',
		blogger:
			'Rangkum event ini dengan gaya blog yang enak dibaca: apa acaranya, siapa yang bicara, dan gagasan mana yang paling berguna untuk pembaca yang tidak hadir. Susun dengan subjudul pendek per topik, boleh memakai daftar poin untuk poin penting. Sebutkan konteks acara secara jujur dan jangan mengarang angka, kutipan, atau agenda. Hindari kesan liputan resmi yang kaku. Tutup dengan kesimpulan praktis: apa yang bisa pembaca ambil dari acara ini.'
	},
	reputation_clarification: {
		mainstream:
			'Tulis pernyataan klarifikasi jurnalistik mengenai isu ini untuk pembaca media nasional Indonesia. Buka dengan lead yang menyatakan secara tegas dan netral: apa yang diklarifikasi, apa yang dinyatakan resmi, dan apa yang benar-benar terjadi menurut keterangan yang tersedia. Susun urutan kejadian secara kronologis dan sebutkan sumber tiap keterangan; bila ada hal yang belum bisa dikonfirmasi, tulis apa adanya sebagai belum terjawab. Jangan memakai heading atau subjudul, hindari nada membela diri, dan jangan mengarang fakta, kronologi, angka, atau kutipan. Akhiri dengan informasi faktual tentang langkah yang diambil atau hal yang masih ditunggu — ini laporan, bukan pembelaan atau iklan.',
		blogger:
			'Tulis klarifikasi ini dengan gaya blog yang jujur dan tidak defensif: mulai dari apa isu yang beredar, lalu sampaikan keterangan yang benar beserta dasarnya. Susun dengan subjudul pendek — apa yang terjadi, apa yang keliru dipahami, dan apa yang dilakukan selanjutnya. Akui secara terbuka hal yang belum bisa dikonfirmasi, dan jangan mengarang kronologi, angka, atau kutipan. Hindari nada menyerang pihak lain. Tutup dengan ajakan tenang kepada pembaca untuk menilai berdasarkan informasi yang tersedia.'
	},
	soft_sell_promo: {
		mainstream:
			'Tulis artikel yang memperkenalkan penawaran ini dengan gaya jurnalistik, bukan iklan. Awali dengan lead yang menjelaskan konteks kebutuhan pembaca dan apa isi penawaran yang berlaku — periode, cakupan, dan syaratnya bila datanya ada. Lanjutkan dengan penjelasan yang informatif: apa yang berubah dari penawaran sebelumnya dan siapa yang paling diuntungkan. Gunakan struktur straight news tanpa heading atau subjudul dan tanpa daftar berpoin. Hindari bahasa promosi, seruan ala iklan, dan klaim keunggulan tanpa bukti; jangan mengarang harga, tanggal, kuota, atau kutipan. Akhiri dengan informasi faktual yang berguna, termasuk hal yang perlu diperiksa pembaca sebelum memanfaatkan penawaran.',
		blogger:
			'Perkenalkan penawaran ini dengan gaya blog yang santai dan informatif: mulai dari situasi pembaca yang relevan, lalu jelaskan apa isi penawarannya, untuk siapa berguna, dan apa syaratnya. Bahas dengan subjudul pendek dan, bila perlu, daftar poin supaya detail mudah dibaca. Sebutkan batas waktu atau kuota apa adanya bila datanya tersedia, dan jangan mengarang harga maupun janji hasil. Hindari bahasa promosi berlebihan dan jangan memaksa ajakan bertindak di setiap paragraf. Tutup dengan catatan praktis: hal yang perlu pembaca periksa sebelum mengambil penawaran ini.'
	},
	special_offer_advertorial: {
		mainstream:
			'Tulis artikel promosi ini dengan tetap menjaga kaidah dasar jurnalistik: pembaca harus paham bahwa ini materi pemasaran, bukan berita. Buka dengan penjelasan jujur tentang produk atau penawaran yang dibahas, lengkap dengan periode dan syaratnya bila datanya tersedia, lalu uraikan apa yang ditawarkan dan untuk kebutuhan siapa. Kembangkan dalam paragraf yang ringkas dan lugas, tanpa heading atau subjudul dan tanpa daftar berpoin. Hindari klaim yang tidak bisa dibuktikan, superlatif tanpa dasar, dan jangan mengarang harga, angka, atau kutipan. Akhiri dengan informasi praktis supaya pembaca bisa mengambil keputusan sendiri.',
		blogger:
			'Tulis artikel promosi yang terasa wajar dibaca sebagai konten blog: jelaskan produk atau penawaran ini dari sudut kebutuhan pembaca, apa yang didapat, dan apa batasannya. Boleh memakai subjudul pendek serta daftar poin untuk rincian penawaran, dan tetap cantumkan periode serta syarat apa adanya. Hindari klaim berlebihan, janji hasil, dan angka yang tidak ada dasarnya. Jujur dan transparan bahwa ini penawaran. Tutup dengan ajakan yang wajar, tanpa memaksa, plus hal yang sebaiknya dicek pembaca sebelum memutuskan.'
	}
};

/** Ambil contoh sesuai slug angle dan jenis media; cadangan bila belum ada. */
export function angleInstructionExample(slug: string | undefined | null, mediaType: string | undefined | null): string {
	const kind: MediaKind = mediaType === 'blogger' ? 'blogger' : 'mainstream';
	return (slug ? ANGLE_EXAMPLES[slug]?.[kind] : undefined) ?? GENERIC_EXAMPLES[kind];
}
