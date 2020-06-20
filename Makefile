default: build test

build:
	go build .
	go build ./cmd/sfab

test:
	go test -race .

examples:
	go build -race ./example/demo

.PHONY: default build test examples
