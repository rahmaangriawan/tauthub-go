.PHONY: dev-api dev-web build-api build-web sqlc sqlc-schema test migrate-status api-web-check

# Prasyarat: PATH berisi Go, bun, sqlc (lihat README).
GO ?= $(HOME)/sdk/go1.27.1/bin/go
SQLC ?= $(HOME)/sdk/bin/sqlc
PG_DUMP ?= /Applications/Postgres.app/Contents/Versions/latest/bin/pg_dump
DB_DSN ?= postgres://ai_writer:ai_writer_local_2026@127.0.0.1:5432/tauthub-go

dev-api:
	cd api && $(GO) run ./cmd/api

dev-web:
	cd web && bun run dev

build-api:
	cd api && $(GO) build -o bin/api ./cmd/api && $(GO) build -o bin/worker ./cmd/worker && $(GO) build -o bin/scheduler ./cmd/scheduler

build-web:
	cd web && bun run build

# Regenerasi schema gabungan dari DB live untuk sqlc (jalankan ulang tiap kali
# ada migration additive baru yang menambah tabel/kolom).
sqlc-schema:
	$(PG_DUMP) "$(DB_DSN)" --schema-only --no-owner --no-privileges --no-comments | grep -v '^\\' > api/migrations/schema-full.sql

sqlc:
	cd api && $(SQLC) generate

test:
	cd api && $(GO) test ./...
	cd web && bun run check

migrate-status:
	cd api && $(GO) run ./cmd/migrate status
