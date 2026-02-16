#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${1:-https://github.com/winderai/analytics-ai-agent-demo}"
KODIT_IMAGE="${KODIT_IMAGE:-registry.helixml.tech/helix/kodit:0.5.24}"
KODIT_API="http://localhost:8080/api/v1"
POLL_INTERVAL=5
DB_DUMP_FILE="kodit_dump.sql"

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
  if docker exec kodit-vectorchord pg_isready -U postgres -d kodit >/dev/null 2>&1; then
    echo "    Postgres is ready."
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "ERROR: Postgres did not become ready in time." >&2
    exit 1
  fi
  sleep 1
done

# ── 2. Write temporary env file for Kodit ──────────────────────────────────────

KODIT_ENV=$(mktemp)
trap 'rm -f "$KODIT_ENV"' EXIT

cat > "$KODIT_ENV" <<'EOF'
ENRICHMENT_ENDPOINT_BASE_URL=https://openrouter.ai/api/v1
ENRICHMENT_ENDPOINT_MODEL=openrouter/mistralai/ministral-8b-2512
ENRICHMENT_ENDPOINT_API_KEY=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
DEFAULT_SEARCH_PROVIDER=vectorchord
DB_URL=postgresql+asyncpg://postgres:mysecretpassword@host.docker.internal:5432/kodit
EOF

# Merge any vars from the repo .env that aren't already set above
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ENV="$SCRIPT_DIR/../.env"
if [ -f "$REPO_ENV" ]; then
  while IFS= read -r line; do
    # skip blanks and comments
    [[ -z "$line" || "$line" == \#* ]] && continue
    key="${line%%=*}"
    # only add if not already present in our env file
    if ! grep -q "^${key}=" "$KODIT_ENV"; then
      echo "$line" >> "$KODIT_ENV"
    fi
  done < "$REPO_ENV"
fi

# ── 3. Start Kodit ────────────────────────────────────────────────────────────

echo "==> Starting Kodit container..."
docker rm -f kodit 2>/dev/null || true
docker run -d \
  --name kodit \
  -p 8080:8080 \
  --env-file "$KODIT_ENV" \
  "$KODIT_IMAGE" \
  serve --host 0.0.0.0

echo "==> Waiting for Kodit API to be reachable..."
for i in $(seq 1 60); do
  if curl -sf "$KODIT_API/repositories" >/dev/null 2>&1; then
    echo "    Kodit API is ready."
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "ERROR: Kodit API did not become ready in time." >&2
    echo "       Logs:" >&2
    docker logs --tail 30 kodit >&2
    exit 1
  fi
  sleep 1
done

# ── 4. Create repository and trigger indexing ─────────────────────────────────

echo "==> Creating repository: $REPO_URL"
RESPONSE=$(curl -sf \
  --request POST \
  --url "$KODIT_API/repositories" \
  --header 'Content-Type: application/json' \
  --data "{
  \"data\": {
    \"type\": \"repository\",
    \"attributes\": {
      \"remote_uri\": \"$REPO_URL\"
    }
  }
}")

REPO_ID=$(echo "$RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['id'])")
echo "    Repository ID: $REPO_ID"

# ── 5. Poll status/summary until completed or failed ─────────────────────────

echo "==> Waiting for indexing to complete (polling every ${POLL_INTERVAL}s)..."
while true; do
  STATUS_JSON=$(curl -sf "$KODIT_API/repositories/$REPO_ID/status/summary")
  STATUS=$(echo "$STATUS_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['attributes']['status'])")

  case "$STATUS" in
    completed)
      echo "    Indexing completed successfully."
      break
      ;;
    failed)
      MESSAGE=$(echo "$STATUS_JSON" | python3 -c "import sys,json; print(json.load(sys.stdin)['data']['attributes'].get('message',''))")
      echo "ERROR: Indexing failed: $MESSAGE" >&2
      exit 1
      ;;
    *)
      printf "    Status: %s ...\r" "$STATUS"
      sleep "$POLL_INTERVAL"
      ;;
  esac
done

# ── 6. Dump the database ─────────────────────────────────────────────────────

echo "==> Dumping database to $DB_DUMP_FILE ..."
docker exec kodit-vectorchord pg_dump -U postgres -d kodit > "$DB_DUMP_FILE"
echo "    Done. Database dump saved to $DB_DUMP_FILE ($(wc -c < "$DB_DUMP_FILE" | tr -d ' ') bytes)"

echo ""
echo "==> All done. Containers 'kodit-vectorchord' and 'kodit' are still running."
echo "    To clean up:  docker rm -f kodit kodit-vectorchord"
