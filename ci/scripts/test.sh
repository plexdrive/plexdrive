#!/bin/bash
set -xe

export GOPATH=$PWD/go
export PATH=$GOPATH/bin:$PATH
export GO111MODULE=on

cd $GOPATH/src/github.com/plexdrive/plexdrive

go mod download
go test ./... -race -cover
