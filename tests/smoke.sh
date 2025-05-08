#!/bin/bash
set -e

# Test version command
kodit version

# Test sources commands
kodit sources list
kodit sources create .

# Test indexes commands
kodit indexes list
kodit indexes create 1
kodit indexes run 1

# Test retrieve command
kodit retrieve "test query"

# Test serve command with timeout
timeout 2s kodit serve || true
