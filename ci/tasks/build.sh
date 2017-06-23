#!/bin/bash
set -e -x

pushd plexdrive
    go get
    go test ./...
popd