# Tulisio — Rewrite Go + Svelte

Rewrite total platform Tulisio/Tauthub: backend **Go (chi + pgx + sqlc)**, frontend **SvelteKit 5 (shadcn-svelte + Tailwind v4)**, deployment **VPS systemd**, database PostgreSQL **`tauthub-go`** (tidak diubah).

Pedoman lengkap: [`docs/REWRITE/`](docs/REWRITE/README.md) — mulai dari `00_MASTER_PLAN.md`, kerjakan fase di `07_EXECUTION_PHASES.md`.

## Struktur

```
api/       # Go — cmd/{api,worker,scheduler,migrate}, internal/, queries/, migrations/
web/       # SvelteKit 5 — src/routes (peta: docs/REWRITE/05_UI_PAGES.md)
docs/      # REWRITE/ + dokumen legacy repo lama yang relevan
deploy/    # systemd units + nginx (diisi saat mendekati cutover)
```

## Development

```bash
# prasyarat lokal: Go 1.27 (~/sdk/go1.27.1), bun 1.4+, sqlc (~/sdk/bin), Postgres 16 berisi DB tauthub-go

# API (terminal 1)
cd api && cp .env.example .env && go run ./cmd/api        # :8080, cek /healthz

# Web (terminal 2)
cd web && bun install && bun run dev                      # :3001 (dev; /v1 diproxy ke :8080)

# Tools
cd api && make -f ../Makefile sqlc        # generate kode dari queries/*.sql
cd api && go run ./cmd/migrate status     # bandingkan migrations/additive vs schema_migrations
```

## Fase eksekusi

Fase 0 (bootstrap) ✅ → fase 1 auth → … → fase 12 cutover. Checklist tiap fase: `docs/REWRITE/07_EXECUTION_PHASES.md`.
