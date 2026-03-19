# Design: Fix Project Startup Script

## Issues Found

### 1. `make dev` hangs indefinitely
`make dev` calls `make docker-dev` (starts containers) then runs `docker compose logs -f kodit`, which follows logs with no exit. A startup script must terminate.

**Fix:** Use `make docker-dev` instead of `make dev`. This builds and starts the containers (idempotent via `--wait`) and then exits.

### 2. Unnecessary uv installation
The script installs `uv` (a Python package manager) and sources its env. Kodit is a Go project with no Python runtime dependency. uv is not used by any Makefile target.

**Fix:** Remove the uv install and source lines.

### 3. Relative `cd kodit` path
The script uses `cd kodit` which depends on the working directory when the script is invoked. This is fragile.

**Fix:** Use the absolute path `cd /home/retro/work/kodit`.

## Fixed Script

```bash
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
```

## Notes for Future Agents

- The kodit project is pure Go. Do not add Python/uv tooling unless the project explicitly requires it.
- `make dev` is for interactive use (tails logs). `make docker-dev` is the correct target for automated/CI startup.
- `make docker-dev` is already idempotent — it calls `docker compose up -d --wait` which is safe to run repeatedly.
- The Makefile lives at `/home/retro/work/kodit/Makefile`. The `dev` target is defined at line 71.
