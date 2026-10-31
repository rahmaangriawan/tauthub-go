package notify

// Saklar kanal email per event yang diatur admin dari UI.
//
// Registry policy.go tetap menjadi sumber kebenaran default. Baris
// email_event_channels hanya bisa MENYALAKAN email untuk event yang default-nya
// off — berguna untuk event yang perusahaannya ingin dikirim lewat email juga.
// Cache 30 detik supaya ShouldSend (dipanggil per kirim) tidak menembak DB
// terus-menerus, tetapi perubahan admin tetap cepat berlaku.

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/email"
)

const emailChannelTTL = 30 * time.Second

var (
	emailChannelMu     sync.Mutex
	emailChannelCache  map[string]bool
	emailChannelAt     time.Time
	emailChannelLoaded bool
)

// EmailChannelOverride: true bila admin menyalakan email untuk event ini.
func EmailChannelOverride(ctx context.Context, pool *pgxpool.Pool, event string) bool {
	emailChannelMu.Lock()
	if emailChannelLoaded && time.Since(emailChannelAt) < emailChannelTTL {
		enabled := emailChannelCache[event]
		emailChannelMu.Unlock()
		return enabled
	}
	emailChannelMu.Unlock()

	states, err := email.LoadChannels(ctx, pool)
	if err != nil {
		return false
	}
	cache := make(map[string]bool, len(states))
	for ev, st := range states {
		cache[ev] = st.Enabled
	}

	emailChannelMu.Lock()
	emailChannelCache = cache
	emailChannelAt = time.Now()
	emailChannelLoaded = true
	emailChannelMu.Unlock()
	return cache[event]
}

// InvalidateEmailChannelCache dipanggil setelah admin mengubah saklar.
func InvalidateEmailChannelCache() {
	emailChannelMu.Lock()
	emailChannelLoaded = false
	emailChannelMu.Unlock()
}
