.PHONY: setup lint test build

setup:
	lefthook install

lint:
	golangci-lint run

test:
	go test ./...

build:
	go build ./...
