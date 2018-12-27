#!/bin/sh -ex

pwd

go list -f '{{.Dir}}' ./...

curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(go env GOPATH)/bin v1.12.5

golangci-lint run -v  ./... # --enable-all -D lll
