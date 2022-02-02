#!/bin/bash
set -xe

# Configuration
export ORIGIN=$PWD
export GOPATH=$PWD/go
export PATH=$GOPATH/bin:$PATH
export GO111MODULE=on
cd $GOPATH/src/github.com/plexdrive/plexdrive

# Version
export VERSION="$(cat ci/meta/version)"
echo "Got version $VERSION"

echo $VERSION > $ORIGIN/metadata/version
sed s/%VERSION%/$VERSION/g ci/meta/notification > $ORIGIN/metadata/notification

# Build 
go mod download
./ci/scripts/go-build-all

mv plexdrive-* $ORIGIN/release

# Check
cd $ORIGIN
ls -lah release
ls -lah metadata
