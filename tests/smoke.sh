#!/bin/bash
set -e

# Set this according to what you want to test
# prefix=""
prefix="uv run"

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

# Test sources commands
$prefix kodit sources list
$prefix kodit sources create $tmp_dir

# Test indexes commands
$prefix kodit indexes list
$prefix kodit indexes create 1
$prefix kodit indexes run 1

# Test retrieve command
$prefix kodit retrieve "Hello"

# Test serve command with timeout
timeout 2s $prefix kodit serve || true
