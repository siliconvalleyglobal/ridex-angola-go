#!/bin/zsh
set -e
cd /Users/dr.sazzadkhan/Downloads/Dev/ridex-angola-go
gofmt -w internal/payouts/
gofmt -l internal/payouts/
go build ./internal/payouts/
go test -count=1 ./internal/payouts/
echo ALL_PASS
