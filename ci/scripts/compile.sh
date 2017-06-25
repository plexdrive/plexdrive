#!/bin/bash
set -xe

export ORIGIN=$PWD
export GOPATH=$PWD/go
export PATH=$GOPATH/bin:$PATH
export TS=$(date +%s)

cd $GOPATH/src/github.com/dweidenfeld/plexdrive

export DOCVERSION=$(cat docs/version)
export VERSION="$DOCVERSION-$TS"

echo "Got version $VERSION"

sed -i.bak s/$DOCVERSION/$VERSION/g docs/version
cat docs/version

sed -i.bak s/%VERSION%/$VERSION/g main.go
sed -i.bak s/%VERSION%/$VERSION/g docs/slack-notification

go get -v
./ci/scripts/go-build-all

mv plexdrive-* $ORIGIN/release

cd $ORIGIN
ls -lah release