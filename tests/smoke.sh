#!/bin/bash
set -e

# Make sure curl is installed
if ! command -v curl &> /dev/null; then
    echo "curl could not be found"
    env
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

# Check that the kodit data_dir does not exist
if [ -d "$HOME/.kodit" ]; then
    echo "Kodit data_dir is not empty, please rm -rf $HOME/.kodit"
    exit 1
fi

# Test version command
$prefix kodit version

# Test indexes API endpoints
echo "Testing indexes API..."

# Start the server in the background
$prefix kodit serve --host 127.0.0.1 --port 8080 &
SERVER_PID=$!

# Wait for server to start up
sleep 10

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

# Wait for server to be ready
if wait_for_server; then
    # Test GET /api/v1/repositories (list indexes)
    echo "Testing GET /api/v1/repositories"
    curl -s -f http://127.0.0.1:8080/api/v1/repositories || echo "List repository test failed"
    
    # Test POST /api/v1/repositories (create repository)
    echo "Testing POST /api/v1/repositories"
    INDEX_RESPONSE=$(curl -s -f -X POST http://127.0.0.1:8080/api/v1/repositories \
        -H "Content-Type: application/json" \
        -d '{"data": {"type": "index", "attributes": {"uri": "https://gist.github.com/7aa38185e20433c04c533f2b28f4e217.git"}}}' \
        || echo "Create repository test failed")

    # Test search API as well
    echo "Testing POST /api/v1/search"
    curl -s -f -X POST http://127.0.0.1:8080/api/v1/search \
        -H "Content-Type: application/json" \
        -d '{"data": {"type": "search", "attributes": {"keywords": ["test"], "code": "def", "text": "function"}}, "limit": 5}' \
        || echo "Search API test failed"
fi

# Clean up: stop the server
if [ -n "$SERVER_PID" ]; then
    kill $SERVER_PID 2>/dev/null || true
    wait $SERVER_PID 2>/dev/null || true
fi
