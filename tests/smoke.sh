#!/bin/bash
set -e

# Set this according to what you want to test. uv run will run the command in the current directory
prefix="uv run"

# If CI is set, no prefix because we're running in github actions
if [ -n "$CI" ]; then
    prefix=""
fi

# Check that the kodit data_dir does not exist
if [ -d "$HOME/.kodit" ]; then
    echo "Kodit data_dir is not empty, please rm -rf $HOME/.kodit"
    exit 1
fi

# Create a temporary directory
tmp_dir=$(mktemp -d)

# Write a dummy python file to the temporary directory
echo "print('Hello, world!')" > $tmp_dir/test.py

# Test version command
$prefix kodit version

# Test index command
$prefix kodit index $tmp_dir
$prefix kodit index https://github.com/winderai/analytics-ai-agent-demo
$prefix kodit index

# Test search command
$prefix kodit search keyword "Hello"
$prefix kodit search code "Hello"
$prefix kodit search hybrid --keywords "main" --code "def main()"

# Test serve command with timeout
timeout 2s $prefix kodit serve || true
