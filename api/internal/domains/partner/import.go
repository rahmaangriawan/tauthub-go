package partner

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port lib/media/bulk-import.ts — impor massal media dari XLSX.
//
// Parser XLSX ditulis dengan archive/zip + encoding/xml (stdlib) supaya tidak
// menambah dependensi: cukup membaca sharedStrings + sheet pertama, yang memang
// seluruh isi template bulk import.

const (
	bulkMediaMaxBytes = 5 * 1024 * 1024
	bulkMediaMaxRows  = 500
)

// bulkHeaderAliases: alias header (lowercase, spasi dinormalkan) → key kolom.
var bulkHeaderAliases = map[string]string{
	"nama media":                  "nama_media",
	"nama":                        "nama_media",
	"name":                        "nama_media",
	"website / domain":            "website_url",
	"website":                     "website_url",
	"domain":                      "website_url",
	"website_url":                 "website_url",
	"url":                         "website_url",
	"kategori":                    "kategori",
	"category":                    "kategori",
	"jenis media":                 "jenis_media",
	"jenis":                       "jenis_media",
	"media type":                  "jenis_media",
	"media_type":                  "jenis_media",
	"platform / integrasi":        "platform",
	"platform":                    "platform",
	"integrasi":                   "platform",
	"harga pengajuan (idr)":       "harga",
	"harga":                       "harga",
	"price":                       "harga",
	"sifat harga":                 "sifat_harga",
	"pricing_type":                "sifat_harga",
	"mode publikasi":              "mode_publikasi",
	"mode posting":                "mode_publikasi",
	"mode":                        "mode_publikasi",
	"publication_mode":            "mode_publikasi",
	"bahasa artikel":              "bahasa_artikel",
	"bahasa":                      "bahasa_artikel",
	"language":                    "bahasa_artikel",
	"nomor sertifikat dewan pers": "sertifikat_dewan_pers",
	"sertifikat dewan pers":       "sertifikat_dewan_pers",
	"sertifikat_dewan_pers":       "sertifikat_dewan_pers",
	"dewan pers":                  "sertifikat_dewan_pers",
	"certificate":                 "sertifikat_dewan_pers",
	"estimasi trafik":             "estimasi_trafik",
	"deskripsi":                   "deskripsi",
	"catatan":                     "catatan",
}

// bulkRequiredHeaders: kolom wajib (parity BULK_MEDIA_SHEET_COLUMNS).
var bulkRequiredHeaders = []struct{ Key, Label string }{
	{"website_url", "Website / Domain"},
	{"kategori", "Kategori"},
	{"harga", "Harga Pengajuan (IDR)"},
}

var bulkAllKeys = []string{
	"website_url", "kategori", "jenis_media", "bahasa_artikel", "harga",
	"nama_media", "platform", "sifat_harga", "mode_publikasi", "sertifikat_dewan_pers",
	"estimasi_trafik", "deskripsi", "catatan",
}

var bahasaMap = map[string]string{
	"indonesia": "Indonesia", "indo": "Indonesia",
	"inggris": "Inggris", "english": "Inggris",
	"china": "China", "mandarin": "China", "cina": "China",
	"arab": "Arab", "arabia": "Arab",
}

var bulkNoCertValues = map[string]bool{
	"tidak": true, "tidak ada": true, "tidak punya": true, "belum": true,
	"belum ada": true, "belum punya": true, "none": true, "no": true,
	"n/a": true, "na": true, "kosong": true, "-": true,
}

// ---------- parser XLSX ----------

type xlsxSheetCell struct {
	Column int
	Text   string
}

// parseXLSXSheet: kembalikan baris (index 1..n) berisi nilai teks per kolom.
func parseXLSXSheet(data []byte) (map[int]map[int]string, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	shared, err := readSharedStrings(reader)
	if err != nil {
		return nil, err
	}
	sheetXML, err := readZipEntry(reader, "xl/worksheets/sheet1.xml")
	if err != nil {
		return nil, err
	}
	return parseSheetXML(sheetXML, shared)
}

func readZipEntry(reader *zip.Reader, name string) ([]byte, error) {
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(io.LimitReader(rc, 32*1024*1024))
	}
	return nil, errXLSXMissingSheet
}

var errXLSXMissingSheet = &PartnerError{http.StatusUnprocessableEntity,
	"Gagal membaca file XLSX. Pastikan file adalah hasil download Google Spreadsheet (XLSX) dengan header di baris 1."}

type xlsxSST struct {
	Items []struct {
		Text string `xml:",chardata"`
		Runs []struct {
			Text string `xml:"t"`
		} `xml:"r"`
		T string `xml:"t"`
	} `xml:"si"`
}

func readSharedStrings(reader *zip.Reader) ([]string, error) {
	raw, err := readZipEntry(reader, "xl/sharedStrings.xml")
	if err != nil {
		// Tanpa sharedStrings (mis. semua inline string) bukan error fatal.
		return nil, nil
	}
	var sst xlsxSST
	if err := xml.Unmarshal(raw, &sst); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(sst.Items))
	for _, item := range sst.Items {
		if len(item.Runs) > 0 {
			var b strings.Builder
			for _, run := range item.Runs {
				b.WriteString(run.Text)
			}
			out = append(out, b.String())
			continue
		}
		text := item.T
		if text == "" {
			text = item.Text
		}
		out = append(out, text)
	}
	return out, nil
}

type xlsxRow struct {
	R     int `xml:"r,attr"`
	Cells []struct {
		R  string `xml:"r,attr"`
		T  string `xml:"t,attr"`
		V  string `xml:"v"`
		IS struct {
			T string `xml:"t"`
			R []struct {
				T string `xml:"t"`
			} `xml:"r"`
		} `xml:"is"`
	} `xml:"c"`
}

func parseSheetXML(raw []byte, shared []string) (map[int]map[int]string, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	out := map[int]map[int]string{}
	rowNumber := 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "row" {
			continue
		}
		var row xlsxRow
		if err := decoder.DecodeElement(&row, &start); err != nil {
			return nil, err
		}
		rowNumber++
		if row.R > 0 {
			rowNumber = row.R
		}
		cells := map[int]string{}
		for _, cell := range row.Cells {
			col := columnIndex(cell.R)
			if col == 0 {
				continue
			}
			text := cellText(cell.T, cell.V, cell.IS.T, cell.IS.R, shared)
			if strings.TrimSpace(text) != "" {
				cells[col] = strings.TrimSpace(text)
			}
		}
		if len(cells) > 0 {
			out[rowNumber] = cells
		}
	}
	return out, nil
}

// columnIndex: "B12" → 2.
func columnIndex(ref string) int {
	col := 0
	for _, ch := range ref {
		if ch >= 'A' && ch <= 'Z' {
			col = col*26 + int(ch-'A') + 1
			continue
		}
		if ch >= 'a' && ch <= 'z' {
			col = col*26 + int(ch-'a') + 1
			continue
		}
		break
	}
	return col
}

func cellText(cellType, value, inlineText string, inlineRuns []struct {
	T string `xml:"t"`
}, shared []string) string {
	switch cellType {
	case "s":
		idx, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || idx < 0 || idx >= len(shared) {
			return ""
		}
		return shared[idx]
	case "inlineStr":
		if len(inlineRuns) > 0 {
			var b strings.Builder
			for _, run := range inlineRuns {
				b.WriteString(run.T)
			}
			return b.String()
		}
		return inlineText
	case "str":
		return value
	default:
		// Numerik/tanggal: nilai mentah. Tanggal serial Excel tidak dikenali
		// karena template bulk tidak memakai kolom tanggal.
		return value
	}
}

// ---------- validasi baris (port validateBulkMediaRow) ----------

type bulkRowParsed struct {
	RowNumber int
	Raw       map[string]string
	Parsed    bulkRowValues
	Errors    []string
	Warnings  []string
}

type bulkRowValues struct {
	Name            string
	WebsiteURL      any
	Category        any
	MediaType       string
	IntegrationType string
	Price           float64
	PricingType     string
	TrafficEstimate float64
	PublicationMode string
	BahasaArtikel   []string
	Description     any
	Notes           any
	CertificateNo   any
}

func normalizeHeaderText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

func normalizePlatform(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "wordpress", "wp", "word press":
		return "wordpress", true
	case "blogger", "blog", "blogspot":
		return "blogger", true
	case "custom_web", "custom web", "custom", "manual", "webhook", "tulisio_api", "api":
		return "manual", true
	}
	return "", false
}

func normalizeMediaType(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "media mainstream", "mainstream", "media berita", "berita", "news":
		return "mainstream", true
	case "media blogger", "blogger", "blog", "blogspot":
		return "blogger", true
	}
	return "", false
}

func normalizePricingType(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "nett", "net", "netto":
		return "nett", true
	case "nego", "negosiasi", "negotiable":
		return "nego", true
	}
	return "", false
}

func normalizePublicationMode(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "draf", "draft", "default", "manual", "mode manual":
		return "draft", true
	case "terbit", "publish", "published", "langsung terbit", "integrasi", "mode integrasi", "otomatis":
		return "publish", true
	}
	return "", false
}

// normalizeBahasaValue: nil saat ada token tak dikenal, []string{} saat kosong.
func normalizeBahasaValue(value string) ([]string, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return []string{}, true
	}
	out := []string{}
	for _, token := range strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == ';' || r == '/'
	}) {
		key := strings.ToLower(strings.TrimSpace(token))
		if key == "" {
			continue
		}
		mapped, ok := bahasaMap[key]
		if !ok {
			return nil, false
		}
		duplicate := false
		for _, existing := range out {
			if existing == mapped {
				duplicate = true
				break
			}
		}
		if !duplicate {
			out = append(out, mapped)
		}
	}
	return out, true
}

func parseBulkNumber(value string) (float64, bool) {
	s := strings.NewReplacer(".", "", ",", "", " ", "").Replace(strings.TrimSpace(value))
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
		return 0, false
	}
	return n, true
}

func isValidBulkURL(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	parsed, err := urlParse(ensureScheme(trimmed))
	if err != nil {
		return false
	}
	return parsed.Hostname() != "" && strings.Contains(parsed.Hostname(), ".")
}

func normalizeWebsiteURLValue(value string) any {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if strings.Contains(trimmed, "://") {
		return trimmed
	}
	return "https://" + trimmed
}

// deriveMediaNameFromWebsite: parity deriveMediaNameFromWebsite.
func deriveMediaNameFromWebsite(websiteURL string) string {
	raw := strings.TrimSpace(websiteURL)
	if raw == "" {
		return ""
	}
	host := raw
	if parsed, err := urlParse(ensureScheme(raw)); err == nil && parsed.Hostname() != "" {
		host = parsed.Hostname()
	} else {
		host = strings.Split(raw, "/")[0]
	}
	host = strings.TrimPrefix(strings.ToLower(host), "www.")
	parts := []string{}
	for _, p := range strings.Split(host, ".") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) > 2 {
		lastTwo := strings.Join(parts[len(parts)-2:], ".")
		matched := false
		for _, prefix := range []string{"co.", "or.", "ac.", "go.", "net.", "web.", "sch."} {
			if strings.HasPrefix(lastTwo, prefix) {
				matched = true
				break
			}
		}
		if matched {
			parts = parts[:len(parts)-2]
		} else {
			parts = parts[:len(parts)-1]
		}
	} else if len(parts) > 1 {
		parts = parts[:len(parts)-1]
	}
	label := strings.Join(strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(strings.Join(parts, " "))), " ")
	if label == "" {
		return truncateRunes(raw, 160)
	}
	words := strings.Split(label, " ")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return truncateRunes(strings.Join(words, " "), 160)
}

func validateBulkRow(rowNumber int, raw map[string]string) bulkRowParsed {
	errors := []string{}
	warnings := []string{}

	websiteRaw := strings.TrimSpace(raw["website_url"])
	var websiteURL any
	if websiteRaw == "" {
		errors = append(errors, "Website / Domain wajib diisi.")
	} else if !isValidBulkURL(websiteRaw) {
		errors = append(errors, "Website / Domain tidak valid. Contoh: https://medianasional.example")
	} else {
		websiteURL = normalizeWebsiteURLValue(websiteRaw)
		if s, ok := websiteURL.(string); ok && len(s) > 500 {
			errors = append(errors, "Website / Domain maksimal 500 karakter.")
		}
	}

	name := strings.TrimSpace(raw["nama_media"])
	if name == "" {
		derived := ""
		if s, ok := websiteURL.(string); ok {
			derived = deriveMediaNameFromWebsite(s)
		}
		name = derived
		if name != "" {
			warnings = append(warnings, `Nama Media diisi otomatis dari domain: "`+name+`". Ubah di dashboard mitra bila kurang pas.`)
		}
	}
	if name == "" {
		errors = append(errors, "Website / Domain wajib diisi supaya nama media bisa dibuat otomatis.")
	} else if len(name) < 2 {
		errors = append(errors, "Nama Media minimal 2 karakter.")
	} else if len(name) > 160 {
		errors = append(errors, "Nama Media maksimal 160 karakter.")
	}

	category := strings.TrimSpace(raw["kategori"])
	var categoryVal any
	if category == "" {
		errors = append(errors, "Kategori wajib diisi.")
	} else if len(category) > 15 {
		errors = append(errors, "Kategori maksimal 15 karakter.")
	} else {
		categoryVal = category
	}

	mediaType := "mainstream"
	jenisRaw := strings.TrimSpace(raw["jenis_media"])
	if jenisRaw != "" {
		mt, ok := normalizeMediaType(jenisRaw)
		if !ok {
			errors = append(errors, "Jenis Media harus Media Mainstream atau Media Blogger.")
		} else {
			mediaType = mt
		}
	}

	integrationType := "wordpress"
	if mediaType == "blogger" {
		integrationType = "blogger"
	}
	platformRaw := strings.TrimSpace(raw["platform"])
	if platformRaw != "" {
		mapped, ok := normalizePlatform(platformRaw)
		if !ok {
			errors = append(errors, "Platform harus WordPress, Blogspot, atau Custom Web.")
		} else {
			integrationType = mapped
		}
		if jenisRaw == "" && integrationType == "blogger" {
			mediaType = "blogger"
		}
	}
	if integrationType == "blogger" && mediaType != "blogger" {
		errors = append(errors, "Platform Blogger hanya berlaku untuk Media Blogger — isi Jenis Media = Media Blogger.")
	}

	hargaRaw := strings.TrimSpace(raw["harga"])
	price := 0.0
	if hargaRaw == "" {
		errors = append(errors, "Harga Pengajuan (IDR) wajib diisi.")
	} else if n, ok := parseBulkNumber(hargaRaw); !ok {
		errors = append(errors, "Harga harus angka. Contoh: 50000 atau 50.000")
	} else if n < 0 {
		errors = append(errors, "Harga tidak boleh negatif.")
	} else if n > 1_000_000_000 {
		errors = append(errors, "Harga maksimal 1.000.000.000.")
	} else {
		price = math.Round(n)
	}

	pricingType := "nett"
	if sifatRaw := strings.TrimSpace(raw["sifat_harga"]); sifatRaw != "" {
		pt, ok := normalizePricingType(sifatRaw)
		if !ok {
			errors = append(errors, "Sifat Harga harus NETT atau NEGO.")
		} else {
			pricingType = pt
		}
	}

	traffic := 0.0
	if trafikRaw := strings.TrimSpace(raw["estimasi_trafik"]); trafikRaw != "" {
		n, ok := parseBulkNumber(trafikRaw)
		switch {
		case !ok:
			errors = append(errors, "Estimasi Trafik harus angka.")
		case n < 0 || n > 10_000_000_000:
			errors = append(errors, "Estimasi Trafik 0–10.000.000.000.")
		default:
			traffic = math.Round(n)
		}
	}

	publicationMode := "draft"
	if modeRaw := strings.TrimSpace(raw["mode_publikasi"]); modeRaw != "" {
		m, ok := normalizePublicationMode(modeRaw)
		if !ok {
			errors = append(errors, "Mode harus DRAFT atau PUBLISH.")
		} else {
			publicationMode = m
		}
	}

	bahasa, ok := normalizeBahasaValue(strings.TrimSpace(raw["bahasa_artikel"]))
	var bahasaArtikel []string
	if !ok {
		errors = append(errors, "Bahasa Artikel harus Indonesia, Inggris, China, atau Arab — bisa lebih dari satu, pisahkan dengan koma.")
		bahasaArtikel = []string{}
	} else if len(bahasa) == 0 {
		bahasaArtikel = []string{"Indonesia"}
	} else {
		bahasaArtikel = bahasa
	}

	description := strings.TrimSpace(raw["deskripsi"])
	var descriptionVal any
	if description != "" {
		if len(description) > 2000 {
			errors = append(errors, "Deskripsi maksimal 2000 karakter.")
		} else {
			descriptionVal = description
		}
	}
	notes := strings.TrimSpace(raw["catatan"])
	var notesVal any
	if notes != "" {
		if len(notes) > 2000 {
			errors = append(errors, "Catatan Internal maksimal 2000 karakter.")
		} else {
			notesVal = notes
		}
	}

	certRaw := strings.TrimSpace(raw["sertifikat_dewan_pers"])
	var certificateNo any
	if certRaw != "" && !bulkNoCertValues[strings.ToLower(certRaw)] {
		certificateNo = certRaw
	}
	if s, isString := certificateNo.(string); isString {
		if len(s) > 200 {
			errors = append(errors, "Nomor Sertifikat Dewan Pers maksimal 200 karakter.")
		} else if !validDewanPersCert(s) {
			errors = append(errors,
				`Format Nomor Sertifikat tidak valid: "`+s+`". Contoh: 206/DP-Verifikasi/K/X/2025. `+
					`Kalau media belum punya sertifikat, tulis "tidak" atau kosongkan kolomnya.`)
		}
	}

	return bulkRowParsed{
		RowNumber: rowNumber,
		Raw:       raw,
		Parsed: bulkRowValues{
			Name: name, WebsiteURL: websiteURL, Category: categoryVal,
			MediaType: mediaType, IntegrationType: integrationType, Price: price,
			PricingType: pricingType, TrafficEstimate: traffic, PublicationMode: publicationMode,
			BahasaArtikel: bahasaArtikel, Description: descriptionVal, Notes: notesVal,
			CertificateNo: certificateNo,
		},
		Errors:   errors,
		Warnings: warnings,
	}
}

// ---------- handler (port media/import/route.ts) ----------

func (d *Deps) MediaImport(w http.ResponseWriter, r *http.Request) {
	user, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(bulkMediaMaxBytes + 1024*1024); err != nil {
		httpx.Error(w, http.StatusBadRequest, "Form data tidak valid.")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "File XLSX wajib diunggah (field 'file').")
		return
	}
	defer file.Close()
	if header.Size > bulkMediaMaxBytes {
		httpx.Error(w, http.StatusRequestEntityTooLarge, "File terlalu besar. Maksimal 5 MB.")
		return
	}
	if header.Size == 0 {
		httpx.Error(w, http.StatusBadRequest, "File kosong.")
		return
	}
	confirm := strings.EqualFold(strings.TrimSpace(r.FormValue("confirm")), "true")

	data, err := io.ReadAll(io.LimitReader(file, bulkMediaMaxBytes+1))
	if err != nil || len(data) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, errXLSXMissingSheet.Message)
		return
	}
	sheet, err := parseXLSXSheet(data)
	if err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, errXLSXMissingSheet.Message)
		return
	}
	headerCells := sheet[1]
	if len(headerCells) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, errXLSXMissingSheet.Message)
		return
	}

	// Petakan kolom → key; deteksi header duplikat dan header wajib hilang.
	headerMap := map[int]string{}
	headerErrors := []string{}
	seen := map[string]bool{}
	columns := make([]int, 0, len(headerCells))
	for col := range headerCells {
		columns = append(columns, col)
	}
	sort.Ints(columns)
	for _, col := range columns {
		key, ok := bulkHeaderAliases[normalizeHeaderText(headerCells[col])]
		if !ok {
			continue
		}
		if seen[key] {
			headerErrors = append(headerErrors, `Header duplikat: "`+headerCells[col]+`"`)
		}
		seen[key] = true
		headerMap[col] = key
	}
	missing := []string{}
	for _, req := range bulkRequiredHeaders {
		if !seen[req.Key] {
			missing = append(missing, req.Label)
		}
	}
	if len(missing) > 0 {
		headerErrors = append(headerErrors,
			"Header wajib tidak ditemukan: "+strings.Join(missing, ", ")+". Pastikan baris 1 adalah header template.")
	}
	if len(headerErrors) > 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, strings.Join(headerErrors, " "))
		return
	}

	// Susun baris data.
	rowNumbers := make([]int, 0, len(sheet))
	for n := range sheet {
		if n > 1 {
			rowNumbers = append(rowNumbers, n)
		}
	}
	sort.Ints(rowNumbers)
	if len(rowNumbers) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "File tidak berisi data. Isi minimal 1 baris data di bawah header.")
		return
	}
	if len(rowNumbers) > bulkMediaMaxRows {
		httpx.Error(w, http.StatusUnprocessableEntity, "Jumlah baris melebihi batas 500. Bagi menjadi beberapa file.")
		return
	}

	parsedRows := make([]bulkRowParsed, 0, len(rowNumbers))
	for _, n := range rowNumbers {
		raw := map[string]string{}
		for _, key := range bulkAllKeys {
			raw[key] = ""
		}
		for col, key := range headerMap {
			if v, present := sheet[n][col]; present {
				raw[key] = v
			}
		}
		hasAny := false
		for _, v := range raw {
			if strings.TrimSpace(v) != "" {
				hasAny = true
				break
			}
		}
		if !hasAny {
			continue
		}
		parsedRows = append(parsedRows, validateBulkRow(n, raw))
	}
	if len(parsedRows) == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "File tidak berisi data. Isi minimal 1 baris data di bawah header.")
		return
	}

	validCount, invalidCount := 0, 0
	for _, row := range parsedRows {
		if len(row.Errors) > 0 {
			invalidCount++
		} else {
			validCount++
		}
	}

	if !confirm {
		preview := make([]map[string]any, 0, len(parsedRows))
		for _, row := range parsedRows {
			preview = append(preview, map[string]any{
				"rowNumber": row.RowNumber,
				"raw":       row.Raw,
				"parsed":    bulkParsedJSON(row.Parsed),
				"errors":    row.Errors,
				"warnings":  row.Warnings,
				"valid":     len(row.Errors) == 0,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"mode": "preview", "total": len(parsedRows), "validCount": validCount,
			"invalidCount": invalidCount, "rows": preview,
		})
		return
	}

	if invalidCount > 0 {
		invalidRows := []map[string]any{}
		for _, row := range parsedRows {
			if len(row.Errors) > 0 {
				invalidRows = append(invalidRows, map[string]any{"rowNumber": row.RowNumber, "errors": row.Errors})
			}
		}
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"detail":      "Masih ada " + itoa(invalidCount) + " baris dengan error. Perbaiki file XLSX sebelum import.",
			"invalidRows": invalidRows,
		})
		return
	}
	if validCount == 0 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Tidak ada baris valid untuk diimport.")
		return
	}
	// Mitra dengan lebih dari 2 media wajib mengisi nomor WhatsApp + izin dihubungi
	// (dihitung dari media lama + baris yang akan diimpor).
	if d.waRequirementMissing(r.Context(), partner.UserID, partner.ID, validCount) {
		writeWARequired(w)
		return
	}

	tcCfg := d.getTcConfig(r)
	markup := d.getMarkupPercent(r.Context())
	results := []map[string]any{}
	rowErrors := []map[string]any{}
	imported := 0
	for _, row := range parsedRows {
		id := newUUID()
		slug := slugifyMediaName(row.Parsed.Name)
		website, _ := row.Parsed.WebsiteURL.(string)
		category, _ := row.Parsed.Category.(string)
		description, _ := row.Parsed.Description.(string)
		notes, _ := row.Parsed.Notes.(string)
		clientPrice := computeClientPrice(row.Parsed.Price, markup)
		creditCost := 0
		if clientPrice > 0 && tcCfg.RateIDR > 0 {
			creditCost = int(math.Ceil(clientPrice / tcCfg.RateIDR))
			if creditCost < 1 {
				creditCost = 1
			}
		}
		err := d.exec(r.Context(), `
			INSERT INTO media (id, name, slug, category, media_type, description, traffic_estimate, website_url, adapter_url, price, pricing_type, price_status, credit_cost, status, integration_type, partner_id, notes, publication_mode, bahasa_artikel, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULL, $9, $10, 'menunggu_review', $11, 'pending_review', $12, $13, $14, $15, $16, $17)`,
			id, row.Parsed.Name, slug, nullIfEmpty(category), row.Parsed.MediaType, nullIfEmpty(description),
			row.Parsed.TrafficEstimate, nullIfEmpty(website), row.Parsed.Price, row.Parsed.PricingType,
			creditCost, row.Parsed.IntegrationType, partner.ID, nullIfEmpty(notes), row.Parsed.PublicationMode,
			marshalJSONArr(row.Parsed.BahasaArtikel), user.ID)
		if err != nil {
			detail := truncateRunes(err.Error(), 500)
			lower := strings.ToLower(detail)
			if strings.Contains(lower, "duplicate") || strings.Contains(lower, "uq_media_slug") || strings.Contains(lower, "23505") {
				detail = `Gagal: slug "` + slug + `" sudah dipakai media lain.`
			}
			rowErrors = append(rowErrors, map[string]any{"rowNumber": row.RowNumber, "detail": detail})
			continue
		}
		_ = d.exec(r.Context(),
			`INSERT INTO media_price_history (id, media_id, actor_id, from_price, to_price, from_pricing_type, to_pricing_type, from_price_status, to_price_status, note)
			 VALUES (gen_random_uuid(), $1, $2, NULL, $3, NULL, $4, NULL, 'menunggu_review', 'Bulk import XLSX')`,
			id, user.ID, row.Parsed.Price, row.Parsed.PricingType)
		d.auditLog(r.Context(), user.ID, "media.submitted", "media", id)
		if cert, isString := row.Parsed.CertificateNo.(string); isString && cert != "" {
			if _, certErr := d.submitDewanPersVerification(r.Context(), id, partner.ID, cert, user.ID); certErr != nil {
				rowErrors = append(rowErrors, map[string]any{"rowNumber": row.RowNumber,
					"detail": `Media "` + row.Parsed.Name + `" berhasil dibuat, tapi sertifikat Dewan Pers gagal: ` + certErr.Error()})
			}
		}
		link := "/admin/media?status=pending_review#tab-pending"
		d.notifyPlatformAdmins(r.Context(), "media.submitted", "Media baru menunggu review",
			`Mitra mengajukan media "`+row.Parsed.Name+`" (bulk import) untuk direview.`, &link)
		results = append(results, map[string]any{
			"rowNumber": row.RowNumber, "id": id, "name": row.Parsed.Name, "status": "pending_review",
		})
		imported++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"mode": "imported", "total": len(parsedRows), "imported": imported,
		"failed": len(rowErrors), "results": results, "errors": rowErrors,
	})
}

func bulkParsedJSON(v bulkRowValues) map[string]any {
	return map[string]any{
		"name":             v.Name,
		"website_url":      v.WebsiteURL,
		"category":         v.Category,
		"media_type":       v.MediaType,
		"integration_type": v.IntegrationType,
		"price":            v.Price,
		"pricing_type":     v.PricingType,
		"traffic_estimate": v.TrafficEstimate,
		"publication_mode": v.PublicationMode,
		"bahasa_artikel":   v.BahasaArtikel,
		"description":      v.Description,
		"notes":            v.Notes,
		"certificate_no":   v.CertificateNo,
	}
}
