BINARY := incrmit
# Static version, kept in sync by incrmit itself (see incrmit.toml). Override on
# the command line for one-off builds, e.g. `make build VERSION=1.2.3`.
VERSION ?= 0.1.4
LDFLAGS := -X github.com/sasmaq/incrmit/internal/buildinfo.version=$(VERSION)
COVER_THRESHOLD ?= 80
DIST := dist
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
LINUX_ARCHES := amd64 arm64
NFPM ?= nfpm
NFPM_CONFIG := packaging/nfpm.yaml

.PHONY: build dist dist-archives linux-binaries deb checksums release test cover vet fmt fmt-check lint check clean

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

# dist-archives packages each cross-compiled binary into a per-platform archive
# (.tar.gz on Unix, .zip on Windows) and writes dist/checksums.txt (SHA-256).
dist-archives: dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=""; \
		[ "$$os" = "windows" ] && ext=".exe"; \
		bin=$(BINARY)-$(VERSION)-$$os-$$arch$$ext; \
		archive=$(BINARY)-$(VERSION)-$$os-$$arch; \
		if [ "$$os" = "windows" ]; then \
			(cd $(DIST) && zip -q $$archive.zip $$bin); \
			echo "archive $(DIST)/$$archive.zip"; \
		else \
			(cd $(DIST) && tar czf $$archive.tar.gz $$bin); \
			echo "archive $(DIST)/$$archive.tar.gz"; \
		fi; \
	done
# linux-binaries builds stamped Linux binaries when missing from dist/ (used by deb).
linux-binaries:
	@mkdir -p $(DIST)
	@for arch in $(LINUX_ARCHES); do \
		out=$(DIST)/$(BINARY)-$(VERSION)-linux-$$arch; \
		if [ ! -f "$$out" ]; then \
			echo "building $$out"; \
			GOOS=linux GOARCH=$$arch CGO_ENABLED=0 \
				go build -ldflags "$(LDFLAGS)" -o $$out . || exit 1; \
		fi; \
	done

# deb packages Linux amd64 and arm64 binaries with nfpm (see packaging/nfpm.yaml).
deb: linux-binaries
	@command -v $(NFPM) >/dev/null 2>&1 || { \
		echo "nfpm not found; install with: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.43.0" >&2; \
		exit 1; \
	}
	@for arch in $(LINUX_ARCHES); do \
		echo "packaging deb for $$arch"; \
		DIST=$(DIST) VERSION=$(VERSION) DEB_ARCH=$$arch \
			$(NFPM) pkg -f $(NFPM_CONFIG) --packager deb --target $(DIST) || exit 1; \
	done
	@echo "debs written to $(DIST)/"

checksums:
	@cd $(DIST) && { \
		rm -f checksums.txt; \
		for f in $(BINARY)-$(VERSION)-*.tar.gz $(BINARY)-$(VERSION)-*.zip $(BINARY)_$(VERSION)_*.deb; do \
			[ -f "$$f" ] || continue; \
			if command -v sha256sum >/dev/null 2>&1; then \
				sha256sum "$$f"; \
			else \
				shasum -a 256 "$$f"; \
			fi; \
		done > checksums.txt; \
	}
	@echo "checksums written to $(DIST)/checksums.txt"

# release builds cross-compiled binaries, archives, .deb packages, and checksums for CI.
release: dist-archives deb checksums

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
