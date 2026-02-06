#!/bin/bash
# Run both smoke tests and compare results

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "=============================================="
echo "RUNNING PYTHON SMOKE TEST"
echo "=============================================="
uv run smoke_python.py 2>&1 | tee /tmp/smoke_python.log

echo ""
echo "=============================================="
echo "RUNNING GO SMOKE TEST"
echo "=============================================="
uv run smoke_go.py 2>&1 | tee /tmp/smoke_go.log

echo ""
echo "=============================================="
echo "COMPARISON COMPLETE"
echo "=============================================="
echo "Python log: /tmp/smoke_python.log"
echo "Go log: /tmp/smoke_go.log"
