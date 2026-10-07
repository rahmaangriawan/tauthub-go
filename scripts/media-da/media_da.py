#!/usr/bin/env python3
"""Kelola laporan DA media dari tools.guestpostlinks.net.

Tiga perintah, semuanya idempoten:

  prepare  Cetak domain media aktif yang DA-nya belum ada / perlu dicek ulang,
           satu per baris. Ini daftar yang disubmit ke tools DA PA checker.
  merge    Gabungkan hasil scraping (JSON [{domain, da}] atau TSV "domain<TAB>da")
           ke dalam SATU CSV kumulatif. Domain lama diperbarui, domain baru
           ditambahkan; file tidak pernah dibuat ulang.
  apply    Tulis isi CSV ke public.media_seo_metrics (kolom da/checked_at/source).
           Cocokkan lewat host website_url, lewati domain yang ambigu, dan TIDAK
           menyentuh pa / spam_score / sinyal AI.

Kredensial DB dibaca dari api/.env (DB_HOST, DB_PORT, DB_NAME, DB_USER, DB_PASSWORD).
"""
from __future__ import annotations

import argparse
import csv
import datetime as dt
import json
import os
import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import urlparse

REPO = Path(__file__).resolve().parents[2]
ENV_FILE = REPO / "api" / ".env"
CSV_PATH = Path(__file__).resolve().parent / "media-da-report.csv"
SOURCE = "guestpostlinks-da-tool"
# DA dianggap perlu dicek ulang setelah sekian hari. Bisa ditimpa via env
# DA_FRESH_DAYS untuk pengujian atau untuk memaksa cek penuh.
FRESH_DAYS = int(os.environ.get("DA_FRESH_DAYS", "7"))

PSQL = os.environ.get("PSQL_BIN", "psql")


def norm_host(raw: str) -> str:
    raw = (raw or "").strip()
    if not raw:
        return ""
    p = urlparse(raw if "://" in raw else "https://" + raw)
    host = (p.hostname or "").lower().rstrip(".")
    if host.startswith("www."):
        host = host[4:]
    return host


def load_env() -> dict[str, str]:
    env: dict[str, str] = {}
    if ENV_FILE.exists():
        for line in ENV_FILE.read_text().splitlines():
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            k, v = line.split("=", 1)
            env[k.strip()] = v.strip().strip('"').strip("'")
    return env


def psql(sql: str) -> list[list[str]]:
    env = load_env()
    conn = [
        PSQL,
        "-h", env.get("DB_HOST", "127.0.0.1"),
        "-p", env.get("DB_PORT", "5432"),
        "-U", env.get("DB_USER", ""),
        "-d", env.get("DB_NAME", ""),
        "-At", "-F", "\t", "-v", "ON_ERROR_STOP=1", "-c", sql,
    ]
    run_env = dict(os.environ, PGPASSWORD=env.get("DB_PASSWORD", ""))
    out = subprocess.run(conn, capture_output=True, text=True, env=run_env)
    if out.returncode != 0:
        sys.exit(f"psql gagal: {out.stderr.strip()}")
    return [line.split("\t") for line in out.stdout.splitlines() if line]


def read_csv() -> list[dict[str, str]]:
    if not CSV_PATH.exists():
        return []
    with CSV_PATH.open(newline="", encoding="utf-8") as f:
        return list(csv.DictReader(f))


def write_csv(rows: list[dict[str, str]]) -> None:
    rows = sorted(rows, key=lambda r: (r["nama_media"].lower(), r["domain"]))
    with CSV_PATH.open("w", newline="", encoding="utf-8") as f:
        w = csv.DictWriter(f, fieldnames=["nama_media", "domain", "da", "checked_at", "source"])
        w.writeheader()
        w.writerows(rows)


def cmd_prepare(args: argparse.Namespace) -> None:
    rows = psql(
        f"""
        SELECT m.website_url
        FROM media m
        LEFT JOIN media_seo_metrics sm ON sm.media_id = m.id
        WHERE m.status = 'active'
          AND NULLIF(BTRIM(m.website_url), '') IS NOT NULL
          AND (sm.da IS NULL
               OR sm.checked_at IS NULL
               OR sm.checked_at < NOW() - INTERVAL '{FRESH_DAYS} days')
        ORDER BY sm.checked_at ASC NULLS FIRST, m.name
        """
    )
    hosts, seen = [], set()
    for (url,) in rows:
        h = norm_host(url)
        if h and h not in seen:
            seen.add(h)
            hosts.append(h)
    total = len(hosts)
    # Situs checker membatasi 100 URL per submit dan 1 kredit per URL. `--chunk-size`
    # memecah daftar jadi beberapa bagian yang bisa disubmit terpisah; `--offset`
    # memilih bagian mana (0-based) sehingga antrean bisa jalan beberapa malam.
    offset = getattr(args, "offset", 0)
    size = getattr(args, "chunk_size", 0)
    if size and size > 0:
        hosts = hosts[offset * size : offset * size + size]
    text = "\n".join(hosts)
    if args.out:
        Path(args.out).write_text(text + ("\n" if text else ""))
    print(text)
    print(f"# {len(hosts)} dari {total} domain perlu dicek", file=sys.stderr)


def cmd_merge(args: argparse.Namespace) -> None:
    raw = Path(args.from_).read_text().strip()
    parsed: list[tuple[str, int]] = []
    if raw.startswith("["):
        for item in json.loads(raw):
            h, da = norm_host(item.get("domain", "")), str(item.get("da", "")).strip()
            if h and re.fullmatch(r"\d+", da) and 0 <= int(da) <= 100:
                parsed.append((h, int(da)))
    else:
        for line in raw.splitlines():
            parts = line.split("\t")
            if len(parts) < 2:
                continue
            h, da = norm_host(parts[0]), parts[1].strip()
            if h and re.fullmatch(r"\d+", da) and 0 <= int(da) <= 100:
                parsed.append((h, int(da)))
    if not parsed:
        sys.exit("tidak ada baris valid pada sumber merge")

    # nama media dari DB, dipakai untuk baris baru
    name_by_host = {
        norm_host(url): name
        for name, url in psql("SELECT m.name, m.website_url FROM media m WHERE m.status='active'")
    }
    checked = args.checked_at or dt.datetime.now(dt.timezone.utc).astimezone().isoformat(timespec="seconds")

    rows = {r["domain"]: r for r in read_csv()}
    added = updated = 0
    for host, da in parsed:
        if host in rows:
            if rows[host]["da"] != str(da):
                updated += 1
            rows[host].update(da=str(da), checked_at=checked, source=SOURCE)
        else:
            rows[host] = {
                "nama_media": name_by_host.get(host, host),
                "domain": host,
                "da": str(da),
                "checked_at": checked,
                "source": SOURCE,
            }
            added += 1
    write_csv(list(rows.values()))
    print(f"merge: {added} domain baru, {updated} diperbarui, total {len(rows)} baris")


def cmd_apply(args: argparse.Namespace) -> None:
    rows = read_csv()
    if not rows:
        sys.exit("CSV kosong")
    simple = re.compile(r"^[a-z0-9.-]+$")
    q = chr(39)  # kutip tunggal
    values = []
    for r in rows:
        da = r["da"].strip()
        host = r["domain"].strip()
        if not simple.match(host) or not re.fullmatch(r"\d+", da):
            continue
        values.append(f"({q}{host}{q}, {int(da)}, timestamptz {q}{r['checked_at']}{q})")
    if not values:
        sys.exit("tidak ada baris valid untuk apply")
    values_sql = ",\n".join("  " + v for v in values)
    sql = f"""
BEGIN;
CREATE TEMP TABLE _da_import(host text PRIMARY KEY, da smallint, checked_at timestamptz) ON COMMIT DROP;
INSERT INTO _da_import(host, da, checked_at) VALUES
{values_sql};

CREATE TEMP TABLE _media_host ON COMMIT DROP AS
SELECT m.id AS media_id,
       regexp_replace(regexp_replace(regexp_replace(lower(btrim(m.website_url)),
         '^[a-z][a-z0-9+.-]*://', ''), '[/?#].*$', ''), '^www[.]', '') AS host
FROM media m
WHERE m.status = 'active' AND NULLIF(btrim(m.website_url), '') IS NOT NULL;

WITH target AS (
  SELECT host, (array_agg(media_id))[1] AS media_id
  FROM _media_host WHERE host <> ''
  GROUP BY host HAVING count(*) = 1
)
INSERT INTO public.media_seo_metrics(media_id, da, checked_at, source, created_at, updated_at)
SELECT t.media_id, i.da, i.checked_at, '{SOURCE}', now(), now()
FROM target t JOIN _da_import i ON i.host = t.host
ON CONFLICT (media_id) DO UPDATE
SET da = EXCLUDED.da, checked_at = EXCLUDED.checked_at,
    source = EXCLUDED.source, updated_at = now()
WHERE (media_seo_metrics.da, media_seo_metrics.source)
      IS DISTINCT FROM (EXCLUDED.da, EXCLUDED.source);

SELECT count(*) FROM _da_import;
COMMIT;
"""
    env = load_env()
    conn = [
        PSQL, "-h", env.get("DB_HOST", "127.0.0.1"), "-p", env.get("DB_PORT", "5432"),
        "-U", env.get("DB_USER", ""), "-d", env.get("DB_NAME", ""), "-v", "ON_ERROR_STOP=1",
    ]
    run_env = dict(os.environ, PGPASSWORD=env.get("DB_PASSWORD", ""))
    out = subprocess.run(conn, input=sql, capture_output=True, text=True, env=run_env)
    if out.returncode != 0:
        sys.exit(f"apply gagal: {out.stderr.strip()}")
    print(out.stdout.strip())


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = ap.add_subparsers(dest="cmd", required=True)

    p = sub.add_parser("prepare", help="cetak domain yang perlu dicek")
    p.add_argument("--out", help="tulis daftar ke file ini juga")
    p.add_argument("--chunk-size", type=int, default=0, help="ambil maksimum N domain (0 = semua); batas situs 100")
    p.add_argument("--offset", type=int, default=0, help="lewati N*chunk-size domain, untuk mengantre beberapa bagian")
    p.set_defaults(func=cmd_prepare)

    m = sub.add_parser("merge", help="gabung hasil scraping ke CSV kumulatif")
    m.add_argument("--from", dest="from_", required=True, help="file JSON [{domain,da}] atau TSV domain<TAB>da")
    m.add_argument("--checked-at", help="timestamp ISO; default sekarang")
    m.set_defaults(func=cmd_merge)

    a = sub.add_parser("apply", help="tulis CSV ke media_seo_metrics")
    a.set_defaults(func=cmd_apply)

    args = ap.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
