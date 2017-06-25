#!/bin/bash
set -xe

export BUILDSCRIPT=$PWD/go-build-all.sh
export GOPATH=$PWD/go
export PATH=$GOPATH/bin:$PATH

cd $GOPATH/src/github.com/dweidenfeld/plexdrive

go get -v
./ci/scripts/go-build-all

mkdir $GOPATH/release
mv plexdrive-* $GOPATH/release