// vapidkeys: buat pasangan kunci VAPID untuk Web Push. Jalankan sekali,
// simpan hasilnya di .env (jangan pernah di-commit). Mengganti kunci akan
// membuat seluruh langganan yang ada berhenti menerima push.
//
//	go run ./cmd/vapidkeys
package main

import (
	"fmt"
	"os"

	"github.com/rahmaangriawan/tauthub-go/internal/webpush"
)

func main() {
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gagal membuat kunci:", err)
		os.Exit(1)
	}
	fmt.Println("# Tambahkan ke .env (kunci privat adalah rahasia):")
	fmt.Printf("VAPID_PRIVATE_KEY=%s\n", priv)
	fmt.Printf("# Kunci publik (diturunkan dari privat; hanya untuk referensi):\n# %s\n", pub)
	fmt.Println("VAPID_SUBJECT=mailto:ops@tauthub.com")
}
