default: build test

build:
	go build .
	go build ./cmd/sfab

test:
	go test -race .

examples:
	go build -race ./example/demo

LDFLAGS := -X main.Version=$(VERSION)
release:
	@echo "Checking that VERSION was defined in the calling environment"
	@test -n "$(VERSION)"
	@echo "OK.  VERSION=$(VERSION)"
	for GOOS in darwin linux; do for GOARCH in amd64; do \
		GOOS=$$GOOS GOARCH=$$GOOARCH go build -ldflags="$(LDFLAGS)" -o sfab_$${GOOS}_$${GOARCH} ./cmd/sfab; \
	done; done

.PHONY: default build test examples
