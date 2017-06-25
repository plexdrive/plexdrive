#!/bin/bash
set -xe

export ORIGIN=$PWD
export GOPATH=$PWD/go
export PATH=$GOPATH/bin:$PATH
export TS=$(date +%s)

cd $GOPATH/src/github.com/dweidenfeld/plexdrive

export VERSION="$(cat docs/version)-$TS"
echo "Got version $VERSION from docs/version"
echo $VERSION > docs/version

sed -i.bak s/%VERSION%/$VERSION/g main.go
sed -i.bak s/%VERSION%/$VERSION/g docs/slack-notification

go get -v
./ci/scripts/go-build-all

mv plexdrive-* $ORIGIN/release

cd $ORIGIN
ls -lah release