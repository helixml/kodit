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
echo -e "def main():\n    print('Hello, world!')" > $tmp_dir/test.py

# Test version command
$prefix kodit version

# Test auto-indexing
AUTO_INDEXING_SOURCES_0_URI=https://gist.github.com/7aa38185e20433c04c533f2b28f4e217.git \
AUTO_INDEXING_SOURCES_1_URI=https://gist.github.com/cbf0bd1f3338ddf9f98879148d2d752d.git \
 $prefix kodit index --auto-index

# Test index command
$prefix kodit index $tmp_dir
$prefix kodit index https://github.com/winderai/analytics-ai-agent-demo
$prefix kodit index

# Test search command
$prefix kodit search keyword "Hello"
$prefix kodit search code "Hello"
$prefix kodit search hybrid --keywords "main" --code "def main()" --text "main"

# Test show command
$prefix kodit show snippets --by-path test.py
$prefix kodit show snippets --by-source https://github.com/winderai/analytics-ai-agent-demo

# Test search command with filters
$prefix kodit search keyword "Hello" --language=python
$prefix kodit search code "Hello" --source-repo=winderai/analytics-ai-agent-demo
$prefix kodit search hybrid --keywords "main" --code "def main()" --text "main" --language=python

# Test serve command with timeout
timeout 2s $prefix kodit serve || true
