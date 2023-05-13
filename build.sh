#!/usr/bin/env bash

set -euo pipefail

#
VERSION=0.1
PKG="github.com/dhnt/nomad/internal"

#
export GOPATH=
export GO111MODULE=on

function build_bin() {
    local os=$1
    local arch=$2
    local name="${os}-$VERSION"

    local dist="dist/${name}"
    FLAGS="-X '${PKG}.Version=$VERSION'"
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -o $dist/ -ldflags="-w -extldflags '-static' $FLAGS" .
}

##
go mod tidy
go fmt ./...
go vet ./...
go test -short ./...

#
mkdir -p dist
rm -rf dist/*

build_bin darwin amd64
build_bin linux amd64
# build_bin windows amd64

##