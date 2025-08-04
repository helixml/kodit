#!/bin/bash
set -e

# Set this according to what you want to test. uv run will run the command in the current directory
prefix="uv run"

# If CI is set, no prefix because we're running in github actions
if [ -n "$CI" ]; then
    prefix=""
fi

# Disable telemetry
export DISABLE_TELEMETRY=true

# Check that the kodit data_dir does not exist
if [ -d "$HOME/.kodit" ]; then
    echo "Kodit data_dir is not empty, please rm -rf $HOME/.kodit"
    exit 1
fi

#

# Create a temporary directory
tmp_dir=$(mktemp -d)

# Write a dummy python file to the temporary directory
echo -e "def main():\n    print('Hello, world!')" > $tmp_dir/test.py

# Test version command
$prefix kodit version

# Test auto-indexing
AUTO_INDEXING_SOURCES_0_URI=https://gist.github.com/7aa38185e20433c04c533f2b28f4e217.git \
 $prefix kodit index --auto-index

# Test index command
$prefix kodit index $tmp_dir
$prefix kodit index https://github.com/winderai/analytics-ai-agent-demo
$prefix kodit index

# Test search command
$prefix kodit search keyword "Hello"
$prefix kodit search code "Hello"
$prefix kodit search hybrid --keywords "main" --code "def main()" --text "main"

# Test show command
$prefix kodit show snippets --by-path test.py
$prefix kodit show snippets --by-source https://github.com/winderai/analytics-ai-agent-demo

# Test search command with filters
result=$($prefix kodit search keyword "list_bigquery_fields,client" --top-k=3 --output-format=json | head -n 1)
# Check that result CONTAINS "def list_bigquery_fields() -> str:"
if [[ "$result" != *"def list_bigquery_fields() -> str:"* ]]; then
    echo "Result does not contain 'def list_bigquery_fields() -> str:'"
    echo "Result: $result"
    exit 1
fi
$prefix kodit search code "Hello" --source-repo=winderai/analytics-ai-agent-demo
$prefix kodit search hybrid --keywords "main" --code "def main()" --text "main" --language=python

# Test indexes API endpoints
echo "Testing indexes API..."

# Start the server in the background
$prefix kodit serve --host 127.0.0.1 --port 8080 &
SERVER_PID=$!

# Wait for server to start up
sleep 3

# Function to check if server is responding
wait_for_server() {
    local max_attempts=10
    local attempt=1
    while [ $attempt -le $max_attempts ]; do
        if curl -s -f http://127.0.0.1:8080/ > /dev/null 2>&1; then
            echo "Server is ready"
            return 0
        fi
        echo "Waiting for server... (attempt $attempt/$max_attempts)"
        sleep 1
        ((attempt++))
    done
    echo "Server failed to start"
    return 1
}

# Wait for server to be ready
if wait_for_server; then
    # Test GET /api/v1/indexes (list indexes)
    echo "Testing GET /api/v1/indexes"
    curl -s -f http://127.0.0.1:8080/api/v1/indexes || echo "List indexes test failed"
    
    # Test POST /api/v1/indexes (create index)
    echo "Testing POST /api/v1/indexes"
    INDEX_RESPONSE=$(curl -s -f -X POST http://127.0.0.1:8080/api/v1/indexes \
        -H "Content-Type: application/json" \
        -d '{"data": {"type": "index", "attributes": {"uri": "https://gist.github.com/7aa38185e20433c04c533f2b28f4e217.git"}}}' \
        || echo "Create index test failed")
    
    # Test search API as well
    echo "Testing POST /api/v1/search"
    curl -s -f -X POST http://127.0.0.1:8080/api/v1/search \
        -H "Content-Type: application/json" \
        -d '{"data": {"type": "search", "attributes": {"keywords": ["test"], "code": "def", "text": "function"}}, "limit": 5}' \
        || echo "Search API test failed"
    
    # Test DELETE /api/v1/indexes/$INDEX_ID (delete index)
    if [[ "$INDEX_RESPONSE" == "Create index test failed" ]]; then
        echo "Delete index test skipped"
    else
        INDEX_ID=$(echo "$INDEX_RESPONSE" | jq -r '.data.id')
        echo "Testing DELETE /api/v1/indexes/$INDEX_ID"
        curl -s -f -X DELETE http://127.0.0.1:8080/api/v1/indexes/$INDEX_ID \
            -H "Content-Type: application/json" \
            || echo "Delete index test failed"
    fi
fi

# Clean up: stop the server
if [ -n "$SERVER_PID" ]; then
    kill $SERVER_PID 2>/dev/null || true
    wait $SERVER_PID 2>/dev/null || true
fi

echo "API tests completed"
