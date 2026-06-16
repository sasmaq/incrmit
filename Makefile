BINARY := incrmit
VERSION ?= $(shell git describe --tags --dirty 2>/dev/null || echo 0.1.0)
LDFLAGS := -X github.com/sasmaq/incrmit/internal/buildinfo.version=$(VERSION)

.PHONY: build test vet fmt fmt-check lint check clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

fmt-check:
	@diff=$$(gofmt -l .); if [ -n "$$diff" ]; then echo "gofmt needed:"; echo "$$diff"; exit 1; fi

lint:
	golangci-lint run

check: fmt-check vet lint test

clean:
	rm -f $(BINARY)
	go clean
