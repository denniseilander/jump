.PHONY: build install test clean release-dry

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o jump ./cmd/jump

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/jump

test:
	go test ./...

clean:
	rm -f jump

release-dry:
	goreleaser release --snapshot --clean
