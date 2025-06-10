#!/bin/bash
set -e

if [ -z "$TEST_TAG" ]; then
    echo "TEST_TAG is not set"
    exit 1
fi

docker run -i -v $HOME/.kodit:/root/.kodit $TEST_TAG version
docker run -i -v $HOME/.kodit:/root/.kodit $TEST_TAG index
docker run -i -v $HOME/.kodit:/root/.kodit $TEST_TAG search keyword "Hello"
docker run -i -v $HOME/.kodit:/root/.kodit $TEST_TAG search code "Hello"
docker run -i -v $HOME/.kodit:/root/.kodit $TEST_TAG search hybrid --keywords "main" --code "def main()" --text "main"
docker run -i -v $HOME/.kodit:/root/.kodit $TEST_TAG serve