.PHONY: setup lint test build

setup:
	go tool lefthook install

lint:
	go tool golangci-lint run

test:
	go test ./...

build:
	go build ./...
