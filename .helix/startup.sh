#!/bin/bash
set -euo pipefail

# Project startup script
# This runs when agents start working on this project

echo "Starting project"

# Ensure make is available (idempotent)
sudo apt-get install -y make

# Start Docker development environment (idempotent, non-blocking)
cd /home/retro/work/kodit && make docker-dev

echo "Project startup complete"
