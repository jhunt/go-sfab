default: build test

build:
	go build .

test:
	go test -race .

examples:
	go build -race ./example/sfab

.PHONY: default build test example
