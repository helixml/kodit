#!/bin/bash
set -e
# Get the directory of this script
script_dir=$(dirname "$0")

# Check if the container is already running
if docker ps | grep -q kodit-vectorchord; then
    echo "Kodit Vectorchord container is already running, please docker rm -f kodit-vectorchord first"
    exit 1
fi

# Start the container
docker run \
    --name kodit-vectorchord \
    -e POSTGRES_DB=kodit \
    -e POSTGRES_PASSWORD=mysecretpassword \
    -p 5432:5432 \
    -d tensorchord/vchord-suite:pg17-20250601

# Wait for the container to be ready
docker exec -it kodit-vectorchord /bin/bash -c "while ! pg_isready -U postgres -d kodit; do sleep 1; done"

# Run the smokes against the vectorchord container
(
    SMOKE_DB_URL=postgresql+asyncpg://postgres:mysecretpassword@localhost:5432/kodit \
    DEFAULT_SEARCH_PROVIDER=vectorchord \
    uv run ./${script_dir}/smoke.py
)