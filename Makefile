default: build test

build:
	go build .

test:
	go test -race .

examples:
	go build -race ./example/demo

.PHONY: default build test examples
