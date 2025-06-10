#!/bin/bash
set -e

if [ -z "$TEST_TAG" ]; then
    echo "TEST_TAG is not set"
    exit 1
fi

# Create a temporary directory
tmp_dir=$HOME/tmp/kodit
mkdir -p $tmp_dir

# Write a dummy python file to the temporary directory
echo -e "def main():\n    print('Hello, world!')" > $tmp_dir/test.py


docker run -i -v $HOME/.kodit:/root/.kodit -v $tmp_dir:/code $TEST_TAG version
docker run -i -v $HOME/.kodit:/root/.kodit -v $tmp_dir:/code $TEST_TAG index
docker run -i -v $HOME/.kodit:/root/.kodit -v $tmp_dir:/code $TEST_TAG index /code
docker run -i -v $HOME/.kodit:/root/.kodit -v $tmp_dir:/code $TEST_TAG search keyword "Hello"
docker run -i -v $HOME/.kodit:/root/.kodit -v $tmp_dir:/code $TEST_TAG search code "Hello"
docker run -i -v $HOME/.kodit:/root/.kodit -v $tmp_dir:/code $TEST_TAG search hybrid --keywords "main" --code "def main()" --text "main"
docker run -i -v $HOME/.kodit:/root/.kodit -v $tmp_dir:/code $TEST_TAG serve