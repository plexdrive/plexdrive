#!/bin/bash
set -xe

export GOPATH=$PWD/go
export PATH=$GOPATH/bin:$PATH

cd $GOPATH/src/github.com/dweidenfeld/plexdrive

go get
go build -o $GOPATH/binary/plexdrive