BINARY := incrmit
VERSION ?= $(shell git describe --tags --dirty 2>/dev/null || echo 0.1.2)
LDFLAGS := -X github.com/sasmaq/incrmit/internal/buildinfo.version=$(VERSION)
COVER_THRESHOLD ?= 80

.PHONY: build test cover vet fmt fmt-check lint check clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

cover:
	go test -coverprofile=coverage.out ./...
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/ {print $$3}' | tr -d '%'); \
	echo "total coverage: $${total}% (threshold $(COVER_THRESHOLD)%)"; \
	awk "BEGIN { exit !($${total} >= $(COVER_THRESHOLD)) }" || \
		{ echo "FAIL: coverage $${total}% is below threshold $(COVER_THRESHOLD)%"; exit 1; }

vet:
	go vet ./...

fmt:
	gofmt -w .

fmt-check:
	@diff=$$(gofmt -l .); if [ -n "$$diff" ]; then echo "gofmt needed:"; echo "$$diff"; exit 1; fi

lint:
	golangci-lint run

check: fmt-check vet lint cover

clean:
	rm -f $(BINARY)
	go clean
