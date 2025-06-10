#!/bin/bash
set -e

if [ -z "$TEST_TAG" ]; then
    echo "TEST_TAG is not set"
    exit 1
fi

# Get the directory of this script
script_dir=$(dirname "$0")

# Start the container, mount the smoke test and run it
docker run -i -v $script_dir:/tests --entrypoint /bin/bash --env CI=true $TEST_TAG -c "/tests/smoke.sh"