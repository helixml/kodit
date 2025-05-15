#!/bin/bash
set -e

# Create a temporary directory
tmp_dir=$(mktemp -d)

# Write a dummy python file to the temporary directory
echo "print('Hello, world!')" > $tmp_dir/test.py

# Test version command
kodit version

# Test sources commands
kodit sources list
kodit sources create $tmp_dir

# Test indexes commands
kodit indexes list
kodit indexes create 1
kodit indexes run 1

# Test retrieve command
kodit retrieve "Hello"

# Test serve command with timeout
timeout 2s kodit serve || true
