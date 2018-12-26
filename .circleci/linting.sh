#!/bin/sh -ex
go get -v
golangci-lint run ./...

