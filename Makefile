.PHONY: build cleanup-package run-example test test-check test-cover

build:
	mkdir -p bin
	go build -o ./bin/locq ./cmd/locq

cleanup-package:
	go mod tidy

run-example:
	go run examples/languages/main.go
	go run examples/files/main.go

test:
	go test ./...

test-check:
	go test -race -tags=integration ./...

test-cover:
	go test -coverprofile=coverage.out ./...
