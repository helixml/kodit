#!/bin/bash
set -ex

# Create a temporary directory
tmp_dir=$(mktemp -d)

# Write a dummy python file to the temporary directory
echo -e "def main():\n    print('Hello, world!')" > $tmp_dir/test.py

if [ -n "$DOCKER" ]; then
    echo "Running in Docker using $TEST_TAG"
    prefix="docker run -i -v $HOME/.kodit:/root/.kodit -v $tmp_dir:$tmp_dir $TEST_TAG "
else
    # If CI is set, no prefix because we're running in github actions
    if [ -n "$CI" ]; then
        prefix="kodit"
    else
        echo "Running in local"
        prefix="uv run kodit"
    fi
fi

# Check that the kodit data_dir does not exist
if [ -d "$HOME/.kodit" ]; then
    echo "Kodit data_dir is not empty, please rm -rf $HOME/.kodit"
    exit 1
fi

# Test version command
$prefix version

# Test index command
$prefix index $tmp_dir
$prefix index https://github.com/winderai/analytics-ai-agent-demo
$prefix index

# Test search command
$prefix search keyword "Hello"
$prefix search code "Hello"
$prefix search hybrid --keywords "main" --code "def main()" --text "main"

# Test serve command with timeout
timeout 2s $prefix serve || true
