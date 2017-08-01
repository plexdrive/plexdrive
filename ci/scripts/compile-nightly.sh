#!/bin/bash
set -xe

# Configuration
export ORIGIN=$PWD
export GOPATH=$PWD/go
export PATH=$GOPATH/bin:$PATH
export TS=$(date +%s)
cd $GOPATH/src/github.com/dweidenfeld/plexdrive

# Version
export VERSION="$(cat ci/meta/version)-beta.$TS"
echo "Got version $VERSION"

sed -i.bak s/%VERSION%/$VERSION/g main.go
echo $VERSION > $ORIGIN/metadata/version
sed s/%VERSION%/$VERSION/g ci/meta/notification > $ORIGIN/metadata/notification

# Build 
go get -v
./ci/scripts/go-build-all

mv plexdrive-* $ORIGIN/release

# Check
cd $ORIGIN
ls -lah release
ls -lah metadata