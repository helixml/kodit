#!/bin/bash
set -euo pipefail

# Project startup script
# This runs when agents start working on this project



echo "🚀 Starting project" 
sudo apt-get install -y make
curl -LsSf https://astral.sh/uv/install.sh | sh
source $HOME/.local/bin/env

make dev
echo "✅ Project startup complete"
