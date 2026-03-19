#!/bin/bash
set -euo pipefail

# Project startup script
# This runs when agents start working on this project

echo "Starting project"

# Ensure make is available (idempotent)
sudo apt-get install -y make

# Install uv (required by download-model step which uses Python to convert the embedding model)
curl -LsSf https://astral.sh/uv/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"

# Start Docker development environment (idempotent, non-blocking)
# Note: use docker-dev not dev — 'make dev' tails logs forever and never exits
cd /home/retro/work/kodit && make docker-dev

echo "Project startup complete"
