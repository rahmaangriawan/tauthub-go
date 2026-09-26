package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/db"
)

// Runner migration additive — port dari scripts/db/migrate-additive.mjs:
// - satu file = satu transaksi
// - toleran error duplicate-object (42P07, 42701, 42710, 42704, 23505, 42723)
//   dan tetap mencatatnya sebagai applied (parity perilaku lama)
// - tidak pernah DROP data
//
// Pemakaian:
//   migrate status           bandingkan file migration vs schema_migrations
//   migrate up               terapkan file yang belum applied
//   migrate mark <versi...>  catat versi tanpa eksekusi (bootstrap DB yang sudah jalan)
func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("migrate gagal", "err", err)
		os.Exit(1)
	}
}

// tolerantCodes: error PG yang dianggap "sudah ada" oleh runner lama.
var tolerantCodes = map[string]bool{
	"42P07": true, // duplicate_table
	"42701": true, // duplicate_column
	"42710": true, // duplicate_object
	"42704": true, // undefined_file/object (DROP IF EXISTS path)
	"23505": true, // unique_violation
	"42723": true, // duplicate_function
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("pemakaian: migrate [status|up|mark <versi...>]")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	database, err := db.Open(cfg.DBDSN(), 2)
	if err != nil {
		return err
	}
	defer database.Close()

	files, err := listMigrations("migrations/additive")
	if err != nil {
		return err
	}
	ctx := context.Background()

	switch args[0] {
	case "status":
		return printStatus(ctx, database, files)
	case "up":
		return applyPending(ctx, database, files)
	case "mark":
		if len(args) < 2 {
			return fmt.Errorf("migrate mark butuh minimal satu versi (nama file)")
		}
		for _, v := range args[1:] {
			if err := markApplied(ctx, database, v); err != nil {
				return err
			}
			slog.Info("ditandai applied (tanpa eksekusi)", "version", v)
		}
		return nil
	default:
		return fmt.Errorf("subperintah tak dikenal: %s", args[0])
	}
}

func listMigrations(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("baca %s: %w", dir, err)
	}
	out := []string{}
	for _, e := range entries {
		if !e.IsDir() && hasSQLExt(e.Name()) {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func hasSQLExt(name string) bool {
	return len(name) > 4 && name[len(name)-4:] == ".sql"
}

func appliedVersions(ctx context.Context, d *db.DB) (map[string]bool, error) {
	rows, err := d.Pool.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("baca schema_migrations: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func printStatus(ctx context.Context, d *db.DB, files []string) error {
	applied, err := appliedVersions(ctx, d)
	if err != nil {
		return err
	}
	pending := 0
	for _, f := range files {
		if applied[f] {
			fmt.Printf("  [x] %s\n", f)
			continue
		}
		pending++
		fmt.Printf("  [ ] %s\n", f)
	}
	fmt.Printf("\n%d file, %d belum applied\n", len(files), pending)
	return nil
}

func applyPending(ctx context.Context, d *db.DB, files []string) error {
	applied, err := appliedVersions(ctx, d)
	if err != nil {
		return err
	}
	for _, f := range files {
		if applied[f] {
			continue
		}
		started := time.Now()
		err := d.WithTx(ctx, func(tx pgx.Tx) error {
			sqlBytes, err := os.ReadFile("migrations/additive/" + f)
			if err != nil {
				return fmt.Errorf("baca file: %w", err)
			}
			if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
				if pgErr, ok := err.(*pgconn.PgError); ok && tolerantCodes[pgErr.Code] {
					slog.Warn("duplikat object, dicatat applied (parity runner lama)", "version", f, "code", pgErr.Code)
					return nil
				}
				return fmt.Errorf("eksekusi: %w", err)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("migration %s: %w", f, err)
		}
		if err := markApplied(ctx, d, f); err != nil {
			return err
		}
		slog.Info("applied", "version", f, "durasi", time.Since(started).Round(time.Millisecond))
	}
	slog.Info("selesai", "total_file", len(files))
	return nil
}

func markApplied(ctx context.Context, d *db.DB, version string) error {
	_, err := d.Pool.Exec(ctx,
		"INSERT INTO schema_migrations (version, applied_at) VALUES ($1, now()) ON CONFLICT (version) DO NOTHING",
		version)
	if err != nil {
		return fmt.Errorf("catat schema_migrations %s: %w", version, err)
	}
	return nil
}
