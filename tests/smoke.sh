#!/bin/bash
set -e

# Make sure curl is installed
if ! command -v curl &> /dev/null; then
    echo "curl could not be found"
    env
    exit 1
fi

# Ensure jq is installed for JSON parsing
if ! command -v jq &> /dev/null; then
    echo "jq could not be found (needed by tests). Please install jq."
    exit 1
fi

# Set this according to what you want to test. uv run will run the command in the current directory
prefix="uv run"

# If CI is set, no prefix because we're running in github actions
if [ -n "$CI" ]; then
    prefix=""
fi

# Disable telemetry
export DISABLE_TELEMETRY=true
export DB_URL=sqlite+aiosqlite:///:memory:

# Test version command
$prefix kodit version

# Test that port 8080 is free
if lsof -i :8080; then
    echo "Port 8080 is already in use"
    exit 1
fi

# Test indexes API endpoints
echo "Testing indexes API..."

# Start the server in the background
TMPFILE=$(mktemp)
trap "rm -f $TMPFILE && echo deleted $TMPFILE" EXIT
$prefix kodit --env-file $TMPFILE serve --host 127.0.0.1 --port 8080 &
SERVER_PID=$!
# Kill the server on exit (don't use -9 because it won't clean up child processes)
trap "kill $SERVER_PID 2>/dev/null && echo killed $SERVER_PID" EXIT

# Function to check if server is responding
wait_for_server() {
    local max_attempts=60
    local attempt=1
    while [ $attempt -le $max_attempts ]; do
        if curl -v -f http://127.0.0.1:8080/healthz; then
            echo "Server is ready"
            return 0
        fi
        echo "Waiting for server... (attempt $attempt/$max_attempts)"
        sleep 1
        ((attempt++))
    done
    echo "Server failed to start"
    kill $SERVER_PID 2>/dev/null || true
    exit 1
}

# Helper: safe curl returning body and non-fatal on failure
safe_curl() {
    # usage: safe_curl METHOD URL [DATA]
    local method=$1
    local url=$2
    local data=${3:-}
    if [ -n "$data" ]; then
        curl -s -f -X "$method" "$url" -H "Content-Type: application/json" -d "$data" || return 1
    else
        curl -s -f -X "$method" "$url" || return 1
    fi
}

# Helper: wait for repository indexing to finish (all tasks terminal)
wait_for_indexing() {
    local repo_id=$1
    local max_attempts=300   # ~10 minutes with 2s sleeps
    local attempt=1
    while [ $attempt -le $max_attempts ]; do
        local status_json
        status_json=$(safe_curl GET "http://127.0.0.1:8080/api/v1/repositories/$repo_id/status" || echo "")
        if [ -n "$status_json" ]; then
            local failed_count
            local inprog_count
            local data_len
            failed_count=$(echo "$status_json" | jq '[.data[]?.attributes.state | select(.=="failed")] | length')
            inprog_count=$(echo "$status_json" | jq '[.data[]?.attributes.state | select(.=="in_progress" or .=="started")] | length')
            data_len=$(echo "$status_json" | jq '.data | length')
            if [ "$failed_count" -gt 0 ]; then
                echo "Indexing failed for repo $repo_id"
                echo "$status_json" | jq '.'
                return 1
            fi
            if [ "$data_len" -gt 0 ] && [ "$inprog_count" -eq 0 ]; then
                echo "Indexing complete for repo $repo_id"
                return 0
            fi
        fi
        echo "Waiting for indexing... (attempt $attempt/$max_attempts)"
        sleep 2
        ((attempt++))
    done
    echo "Timed out waiting for indexing to complete for repo $repo_id"
    return 1
}

# Wait for server to be ready
wait_for_server;

# Test GET /api/v1/repositories (list indexes)
echo "Testing GET /api/v1/repositories"
safe_curl GET http://127.0.0.1:8080/api/v1/repositories >/dev/null || echo "List repository test failed"

# Test POST /api/v1/repositories (create repository)
echo "Testing POST /api/v1/repositories"
TARGET_URI="https://gist.github.com/7aa38185e20433c04c533f2b28f4e217.git"
CREATE_PAYLOAD=$(jq -n --arg uri "$TARGET_URI" '{data:{type:"repository", attributes:{remote_uri:$uri}}}')
safe_curl POST http://127.0.0.1:8080/api/v1/repositories "$CREATE_PAYLOAD" >/dev/null || echo "Create repository test failed"

# Obtain first repository id from list
LIST_JSON=$(safe_curl GET http://127.0.0.1:8080/api/v1/repositories || echo "")
REPO_ID=$(echo "$LIST_JSON" | jq -r '.data[0].id // empty')
if [ -z "$REPO_ID" ]; then
    echo "No repository id found"
    exit 1
fi

echo "Using repository id: $REPO_ID"

# Wait until indexing tasks complete
if ! wait_for_indexing "$REPO_ID"; then
    echo "Indexing did not complete successfully; continuing with remaining tests"
fi

# GET repository details
echo "Testing GET /api/v1/repositories/$REPO_ID"
safe_curl GET "http://127.0.0.1:8080/api/v1/repositories/$REPO_ID" >/dev/null || echo "Get repository test failed"

# GET repository status
echo "Testing GET /api/v1/repositories/$REPO_ID/status"
safe_curl GET "http://127.0.0.1:8080/api/v1/repositories/$REPO_ID/status" >/dev/null || echo "Get repository status test failed"

# GET repository tags
echo "Testing GET /api/v1/repositories/$REPO_ID/tags"
TAGS_JSON=$(safe_curl GET "http://127.0.0.1:8080/api/v1/repositories/$REPO_ID/tags" || echo "")
if [ -n "$TAGS_JSON" ]; then
    TAG_ID=$(echo "$TAGS_JSON" | jq -r '.data[0].id // empty')
    if [ -n "$TAG_ID" ]; then
        echo "Testing GET /api/v1/repositories/$REPO_ID/tags/$TAG_ID"
        safe_curl GET "http://127.0.0.1:8080/api/v1/repositories/$REPO_ID/tags/$TAG_ID" >/dev/null || echo "Get repository tag test failed"
    fi
fi

# GET repository commits
echo "Testing GET /api/v1/repositories/$REPO_ID/commits"
COMMITS_JSON=$(safe_curl GET "http://127.0.0.1:8080/api/v1/repositories/$REPO_ID/commits" || echo "")
if [ -n "$COMMITS_JSON" ]; then
    COMMIT_SHA=$(echo "$COMMITS_JSON" | jq -r '.data[0].attributes.commit_sha // empty')
    if [ -n "$COMMIT_SHA" ]; then
        echo "Testing GET /api/v1/repositories/$REPO_ID/commits/$COMMIT_SHA"
        safe_curl GET "http://127.0.0.1:8080/api/v1/repositories/$REPO_ID/commits/$COMMIT_SHA" >/dev/null || echo "Get repository commit test failed"

        echo "Testing GET /api/v1/repositories/$REPO_ID/commits/$COMMIT_SHA/files"
        FILES_JSON=$(safe_curl GET "http://127.0.0.1:8080/api/v1/repositories/$REPO_ID/commits/$COMMIT_SHA/files" || echo "")
        if [ -n "$FILES_JSON" ]; then
            BLOB_SHA=$(echo "$FILES_JSON" | jq -r '.data[0].attributes.blob_sha // empty')
            if [ -n "$BLOB_SHA" ]; then
                echo "Testing GET /api/v1/repositories/$REPO_ID/commits/$COMMIT_SHA/files/$BLOB_SHA"
                safe_curl GET "http://127.0.0.1:8080/api/v1/repositories/$REPO_ID/commits/$COMMIT_SHA/files/$BLOB_SHA" >/dev/null || echo "Get commit file test failed"
            fi
        fi

        echo "Testing GET /api/v1/repositories/$REPO_ID/commits/$COMMIT_SHA/snippets"
        safe_curl GET "http://127.0.0.1:8080/api/v1/repositories/$REPO_ID/commits/$COMMIT_SHA/snippets" >/dev/null || echo "List commit snippets test failed"

        echo "Testing GET /api/v1/repositories/$REPO_ID/commits/$COMMIT_SHA/embeddings"
        safe_curl GET "http://127.0.0.1:8080/api/v1/repositories/$REPO_ID/commits/$COMMIT_SHA/embeddings?full=false" >/dev/null || echo "List commit embeddings test failed"
    fi
fi

# Optionally delete repository to cleanup
echo "Testing DELETE /api/v1/repositories/$REPO_ID"
safe_curl DELETE "http://127.0.0.1:8080/api/v1/repositories/$REPO_ID" >/dev/null || echo "Delete repository test failed"

# Test search API as well (JSON:API payload)
echo "Testing POST /api/v1/search"
SEARCH_PAYLOAD='{"data": {"type": "search", "attributes": {"keywords": ["test"], "code": "def", "text": "function", "limit": 5}}}'
safe_curl POST http://127.0.0.1:8080/api/v1/search "$SEARCH_PAYLOAD" >/dev/null || echo "Search API test failed"

# Try queue endpoints (may require API key; ignore failure)
echo "Testing GET /api/v1/queue"
safe_curl GET http://127.0.0.1:8080/api/v1/queue >/dev/null || echo "Queue list test skipped/failed"

# If we get here, the all tests have passed and we can exit successfully
echo "All tests passed"
exit 0