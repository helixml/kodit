#!/usr/bin/env bash
set -euo pipefail

DB_DUMP_FILE="${1:-kodit_dump.sql}"

if [ ! -f "$DB_DUMP_FILE" ]; then
  echo "ERROR: Dump file not found: $DB_DUMP_FILE" >&2
  echo "Usage: $0 [path/to/dump.sql]" >&2
  exit 1
fi

# ── 1. Start VectorChord Postgres ──────────────────────────────────────────────

echo "==> Starting VectorChord Postgres container..."
docker rm -f kodit-vectorchord 2>/dev/null || true
docker run \
  --name kodit-vectorchord \
  -e POSTGRES_DB=kodit \
  -e POSTGRES_PASSWORD=mysecretpassword \
  -p 5432:5432 \
  -d tensorchord/vchord-suite:pg17-20250601

echo "==> Waiting for Postgres to accept connections..."
for i in $(seq 1 30); do
  if docker exec kodit-vectorchord psql -U postgres -d kodit -c "SELECT 1" >/dev/null 2>&1; then
    echo "    Postgres is ready."
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "ERROR: Postgres did not become ready in time." >&2
    exit 1
  fi
  sleep 1
done

# ── 2. Load the SQL dump ───────────────────────────────────────────────────────

echo "==> Loading dump from $DB_DUMP_FILE ($(wc -c < "$DB_DUMP_FILE" | tr -d ' ') bytes)..."
docker exec -i kodit-vectorchord psql -U postgres -d kodit < "$DB_DUMP_FILE"
echo "    Done."

echo ""
echo "==> Database loaded. Container 'kodit-vectorchord' is running."
echo "    Connection: postgresql://postgres:mysecretpassword@localhost:5432/kodit"
echo "    To clean up:  docker rm -f kodit-vectorchord"
