#!/bin/sh -ex

if [ -f version.txt ]; then
  version=$(cat version.txt)
  echo "package main

const VERSION=\"$version\"
" >version.go
fi

go get -v
CGO_ENABLED=0 go build -o logfwd

mkdir -pv /tmp/artifacts
cp logfwd /tmp/artifacts
