#!/bin/bash
set -xe

export ORIGIN=$PWD
export GOPATH=$PWD/go
export PATH=$GOPATH/bin:$PATH

cd $GOPATH/src/github.com/dweidenfeld/plexdrive

go get -v
./ci/scripts/go-build-all

mv plexdrive-* $ORIGIN/release

cd $ORIGIN
ls -lah release