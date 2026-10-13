package brief

import "errors"

var (
	// errNotConfigured: admin belum mengisi konfigurasi agent Taut Brief.
	errNotConfigured = errors.New("Fitur Taut Brief belum dikonfigurasi oleh admin. Silakan hubungi admin platform untuk mengaktifkannya.")

	// errBadOutput: model tidak membalas JSON sesuai kontrak.
	errBadOutput = errors.New("Model tidak membalas format brief yang diharapkan. Coba kirim ulang pesanmu.")
)
