#!/bin/bash
set -xe

export ORIGIN=$PWD
export GOPATH=$PWD/go
export PATH=$GOPATH/bin:$PATH
export TS=$(date +%s)

cd $GOPATH/src/github.com/dweidenfeld/plexdrive

export VERSION="$(cat docs/version)-$TS"
echo "Got version $VERSION"

sed -i.bak s/%VERSION%/$VERSION/g main.go

go get -v
./ci/scripts/go-build-all

mv plexdrive-* $ORIGIN/release
echo $VERSION > $ORIGIN/metadata/version

cd $ORIGIN
ls -lah release
ls -lah metadata