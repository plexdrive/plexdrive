#!/bin/bash
set -xe

export ORIGIN=$PWD
export GOPATH=$PWD/go
export PATH=$GOPATH/bin:$PATH

cd $GOPATH/src/github.com/dweidenfeld/plexdrive

export VERSION=$(cat docs/version)
echo "Got version $VERSION from docs/version"

sed -i.bak s/%VERSION%/$VERSION/g main.go

go get -v
./ci/scripts/go-build-all

mv plexdrive-* $ORIGIN/release

cd $ORIGIN
ls -lah release