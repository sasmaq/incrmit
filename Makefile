BINARY := incrmit
# Static version, kept in sync by incrmit itself (see incrmit.toml). Override on
# the command line for one-off builds, e.g. `make build VERSION=1.2.3`.
VERSION ?= 0.1.4
LDFLAGS := -X github.com/sasmaq/incrmit/internal/buildinfo.version=$(VERSION)
COVER_THRESHOLD ?= 80
DIST := dist
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: build dist test cover vet fmt fmt-check lint check clean

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

dist:
	@rm -rf $(DIST)
	@mkdir -p $(DIST)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=""; \
		[ "$$os" = "windows" ] && ext=".exe"; \
		out=$(DIST)/$(BINARY)-$(VERSION)-$$os-$$arch$$ext; \
		echo "building $$out"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
			go build -ldflags "$(LDFLAGS)" -o $$out . || exit 1; \
	done
	@echo "binaries written to $(DIST)/"

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
	rm -rf $(DIST)
	go clean
