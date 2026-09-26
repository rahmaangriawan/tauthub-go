package main

import (
	"fmt"
	"os"

	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

// Dev utility: seal nilai (mis. password SMTP) dengan APP_SECRET dari .env,
// keluarkan string ter-seal v2. Dipakai untuk mengisi kolom password_sealed
// saat menyiapkan data dev (mis. smtp_settings).
//
//	go run ./cmd/devseal "password-rahasia"
func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	plain := ""
	if len(os.Args) > 1 {
		plain = os.Args[1]
	}
	if plain == "" {
		fmt.Fprintln(os.Stderr, "pemakaian: devseal <nilai>")
		os.Exit(1)
	}
	sealed, err := crypto.Seal(plain, cfg.Secret)
	if err != nil {
		fmt.Fprintln(os.Stderr, "seal:", err)
		os.Exit(1)
	}
	fmt.Println(sealed)
}
