#!/usr/bin/env python3
"""Generate template brief .docx statis untuk Gerbang 2 (Instan Brief).

Docx minimal valid tanpa library: zip dengan [Content_Types].xml, _rels,
word/document.xml. Semua formatting di-inline (tanpa styles.xml) supaya
paketnya kecil dan pasti dibaca Word. Teks berasal dari daftar yang sama
dengan BRIEF_TEMPLATES di
web/src/routes/klien/[brandId]/profiler/[profileId]/+page.svelte — kalau
template chat diubah, jalankan ulang skrip ini.

Pakai: python3 web/scripts/make-brief-templates.py
Output: web/static/templates/brief-agensi.docx, brief-personal.docx, brief-produk.docx
"""
import os
import zipfile

OUT_DIR = os.path.join(os.path.dirname(__file__), "..", "static", "templates")

_content_types = """<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>
"""

_rels = """<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>
"""


def esc(text: str) -> str:
    return text.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")


def title_run(text: str) -> str:
    return f'<w:r><w:rPr><w:b/><w:sz w:val="30"/></w:rPr><w:t xml:space="preserve">{esc(text)}</w:t></w:r>'


def head_run(text: str) -> str:
    return f'<w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">{esc(text)}</w:t></w:r>'


def body_runs(line: str) -> str:
    """Baris biasa; bagian [KURUNG SIKU] diberi warna aksen supaya tak terlewati."""
    runs = []
    i = 0
    while i < len(line):
        start = line.find("[", i)
        if start < 0:
            runs.append(f'<w:r><w:t xml:space="preserve">{esc(line[i:])}</w:t></w:r>')
            break
        if start > i:
            runs.append(f'<w:r><w:t xml:space="preserve">{esc(line[i:start])}</w:t></w:r>')
        end = line.find("]", start)
        if end < 0:
            runs.append(f'<w:r><w:t xml:space="preserve">{esc(line[start:])}</w:t></w:r>')
            break
        runs.append(
            f'<w:r><w:rPr><w:b/><w:color w:val="2B67EC"/></w:rPr>'
            f'<w:t xml:space="preserve">{esc(line[start:end + 1])}</w:t></w:r>'
        )
        i = end + 1
    return "".join(runs).replace("&amp;#x0001;", "&amp;")


def document_xml(title: str, footer: str, lines: list) -> str:
    body = [f'<w:p>{title_run(title)}</w:p>']
    for line in lines:
        text = line.strip()
        if not text:
            body.append("<w:p/>")
        elif text.isupper() and len(text) > 4:
            body.append(f'<w:p><w:pPr><w:spacing w:before="160"/></w:pPr>{head_run(text)}</w:p>')
        else:
            body.append(f'<w:p>{body_runs(text)}</w:p>')
    body.append(
        f'<w:p><w:pPr><w:spacing w:before="240"/></w:pPr>'
        f'<w:r><w:rPr><w:i/><w:color w:val="667085"/></w:rPr>'
        f'<w:t xml:space="preserve">{esc(footer)}</w:t></w:r></w:p>'
    )
    sect = (
        '<w:sectPr><w:pgSz w:w="11906" w:h="16838"/>'
        '<w:pgMar w:top="1418" w:right="1418" w:bottom="1418" w:left="1418"/></w:sectPr>'
    )
    return (
        '<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">'
        f"<w:body>{''.join(body)}{sect}</w:body></w:document>"
    )


TEMPLATES = {
    "brief-agensi.docx": ("Template Brief — Agensi (Tauthub)", [
        "Brief gaya agensi — target profil entitas lengkap dalam satu sesi.",
        "IDENTITAS (WAJIB GANTI DENGAN DATA ASLI)",
        "Nama brand: [NAMA BRAND — ejaan resmi]",
        "Kategori: [PLATFORM / AGENSI / TOKO / LAINNYA]",
        "Deskripsi 1 kalimat: [apa yang membantu siapa untuk apa]",
        "Fakta resmi: berdiri [TAHUN] di [KOTA], [ANGKA MITRA/NASABAH/KLIEN] sampai hari ini — angka ini tidak boleh dikarang.",
        "Website resmi: [URL] — silakan baca dulu semua isinya.",
        "ANGGARAN & MEDIA",
        "Budget: maksimal Rp[NOMINAL] per media — angka ini plafon, bukan harga minimum.",
        "Jumlah media: [JUMLAH]",
        "Minimum DA: [ANGKA], minimum AI Trust: [ANGKA]",
        "Syarat artikel: minimal [ANGKA] kata, [ANGKA] backlink per artikel, tautan [permanen / tidak permanen]",
        "Jenis media: Media Mainstream dan Media Blogger",
        "ATURAN & TUJUAN",
        'Kata/frasa terlarang: [misal "nomor 1", "terbaik", "terjangkau"]',
        "Klaim yang boleh dikutip (dengan buktinya): [DAFTAR KLAIM]",
        "Pertanyaan yang ingin dimenangkan di ChatGPT/Perplexity: [2-3 pertanyaan]",
    ]),
    "brief-personal.docx": ("Template Brief — Personal Brand (Tauthub)", [
        "Brief personal brand.",
        "IDENTITAS (WAJIB GANTI DENGAN DATA ASLI)",
        "Nama: [NAMA LENGKAP + GELAR]",
        "Profesi: [JABATAN — founder / dokter / dosen / praktisi]",
        "Deskripsi 1 kalimat: [siapa, membantu siapa, untuk apa]",
        "Fakta resmi: pengalaman [TAHUN] di [BIDANG], afiliasi [PERUSAHAAN/INSTITUSI], prestasi [YANG BISA DIBUKTIKAN] — kalau tidak ada datanya, biarkan kosong.",
        "POSISI DIGITAL",
        "Audiens: [SEGMENTASI KLIEN — mis. korporat, pemula, rekan sebaya]",
        "Pertanyaan yang ingin dimenangkan di ChatGPT/Perplexity: [2-3 pertanyaan]",
        "Keyword prioritas: [KEYWORD] — target tautan: [URL]",
        "Kata/frasa terlarang: [KATA YANG DILARANG]",
        "GAYA & MEDIA",
        "Gaya bahasa: [PROFESIONAL / SANTAI PROFESIONAL]",
        "Budget: maksimal Rp[NOMINAL] per media, dengan [JUMLAH] media.",
        "Syarat artikel: minimal [ANGKA] kata, [ANGKA] backlink, tautan [permanen / tidak permanen]",
    ]),
    "brief-produk.docx": ("Template Brief — Peluncuran Produk (Tauthub)", [
        "Brief peluncuran produk.",
        "PRODUK (WAJIB GANTI DENGAN DATA ASLI)",
        "Nama brand/produk: [NAMA]",
        "Kategori: [PRODUK DIGITAL / PRODUK FISIK / JASA]",
        "Deskripsi 2 kalimat: [apa yang dijual, siapa targetnya, apa bedanya]",
        "Fakta resmi: diluncurkan [TANGGAL/TAHUN] oleh [NAMA PERUSAHAAN], harga mulai Rp[ANGKA]",
        "Keunggulan utama (USP): [FITUR YANG NYATA — jangan mengarang]",
        "DISTRIBUSI",
        "Budget: maksimal Rp[NOMINAL] per media — angka ini plafon, bukan harga minimum.",
        "Jumlah media: [JUMLAH]",
        "Minimum DA: [ANGKA], minimum AI Trust: [ANGKA]",
        "Syarat artikel: minimal [ANGKA] kata, [ANGKA] backlink per artikel, tautan [permanen / tidak permanen]",
        "Jenis media: Media Mainstream dan Media Blogger",
        "TUJUAN",
        "Pertanyaan yang ingin dimenangkan di ChatGPT/Perplexity: [2-3 pertanyaan]",
        "Kata/frasa terlarang: [KATA YANG DILARANG]",
    ]),
}

FOOTER = (
    "CATATAN: Isi semua bagian dalam tanda kurung siku [..] dengan data asli — "
    "Tauthub tidak mengarang angka yang tidak ia punya. Simpan berkas ini lalu "
    "unggah kembali di Taut Brief > Instan Brief."
)

os.makedirs(OUT_DIR, exist_ok=True)
for name, (title, lines) in TEMPLATES.items():
    path = os.path.join(OUT_DIR, name)
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as z:
        z.writestr("[Content_Types].xml", _content_types)
        z.writestr("_rels/.rels", _rels)
        z.writestr("word/document.xml", document_xml(title, FOOTER, lines))
    print("ok", path)
