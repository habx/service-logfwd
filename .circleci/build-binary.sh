#!/bin/sh -ex

go get -v
CGO_ENABLED=0 go build -o logfwd

mkdir -pv /tmp/artifacts
cp logfwd /tmp/artifacts
