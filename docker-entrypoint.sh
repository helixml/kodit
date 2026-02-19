#!/bin/sh
set -e

# Fix ownership of the data directory so the kodit user can write to it.
# This handles upgrades from older images that ran as root.
dir="${DATA_DIR:-/data}"
if [ "$(id -u)" = "0" ]; then
    chown -R kodit:kodit "$dir"
    exec gosu kodit "$@"
fi

exec "$@"
