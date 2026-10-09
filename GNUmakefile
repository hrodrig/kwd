# kwd — build, test, release (GNU Make). On FreeBSD use gmake (pkg install gmake).
# GNU Make prefers GNUmakefile over Makefile; the Makefile stub forwards to gmake.

BINARY   := kwd
PLUGIN   := kubectl-kwd
DIST     := dist
PREFIX   ?= /usr/local
BINDIR   ?= $(PREFIX)/bin
MANDIR   ?= $(PREFIX)/share/man
FREEBSD_ARCH ?= amd64
OPENBSD_ARCH ?= amd64
# Minimum total statement coverage for `make cover-check`.
COVERAGE_MIN ?= 80
VERSION  ?= $(shell v=$$(cat VERSION 2>/dev/null | tr -d '\n\r'); [ -n "$$v" ] && echo "v$$v" || echo "v0.1.0")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BRANCH   := $(shell b=$$(git rev-parse --abbrev-ref HEAD 2>/dev/null); [ -n "$$b" ] && [ "$$b" != "HEAD" ] && echo "$$b" || echo "unknown")
BUILDDATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -ldflags "-s -w -X github.com/hrodrig/kwd/internal/cli.Version=$(VERSION) -X github.com/hrodrig/kwd/internal/cli.Commit=$(COMMIT) -X github.com/hrodrig/kwd/internal/cli.BuildDate=$(BUILDDATE) -X github.com/hrodrig/kwd/internal/cli.Branch=$(BRANCH)"
PORT_VERSION := $(shell cat VERSION 2>/dev/null | tr -d '\n\r' | sed 's/^v//')

GREEN  := \033[0;32m
YELLOW := \033[0;33m
CYAN   := \033[0;36m
RESET  := \033[0m
ifneq ($(NO_COLOR),)
  GREEN  :=
  YELLOW :=
  CYAN   :=
  RESET  :=
endif

.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo "$(GREEN)kwd$(RESET) — Kubernetes Workload Watchdog"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "$(YELLOW)Build:$(RESET)"
	@echo "  $(GREEN)build$(RESET)                     Build ./bin/kwd for current platform"
	@echo "  $(GREEN)build-all$(RESET)                 Cross-compile to $(DIST)/ (linux, darwin, windows, freebsd, openbsd)"
	@echo ""
	@echo "$(YELLOW)Install & clean:$(RESET)"
	@echo "  $(GREEN)install$(RESET)                   go install to \$$GOBIN"
	@echo "  $(GREEN)clean$(RESET)                     Remove ./bin/kwd, coverage.out, and $(DIST)/"
	@echo ""
	@echo "$(YELLOW)Test:$(RESET)"
	@echo "  $(GREEN)test$(RESET)                      Unit tests (go test -race ./...)"
	@echo "  $(GREEN)cover$(RESET)                     Unit tests with coverage.out"
	@echo "  $(GREEN)cover-check$(RESET)               Fail if total statement coverage < $(COVERAGE_MIN)% (override: COVERAGE_MIN=70)"
	@echo ""
	@echo "$(YELLOW)Quality:$(RESET)"
	@echo "  $(GREEN)lint$(RESET)                      check-pins, check-man-version, gofmt -s, go vet, gocyclo (<=14)"
	@echo "  $(GREEN)lint-fix$(RESET)                  gofmt -s -w"
	@echo "  $(GREEN)check-pins$(RESET)                Fail if a CVE-pinned module drops below its fixed version"
	@echo "  $(GREEN)check-man-version$(RESET)         Fail if a man page .TH drifts from VERSION"
	@echo "  $(GREEN)gocyclo$(RESET)                   Cyclomatic complexity gate (<=14)"
	@echo "  $(GREEN)govulncheck$(RESET)               Known-vulnerability scan of the module graph"
	@echo "  $(GREEN)grype$(RESET)                     Vulnerability scan of the working tree"
	@echo "  $(GREEN)security$(RESET)                  govulncheck + gocyclo + grype"
	@echo "  $(GREEN)tools$(RESET)                     Install govulncheck and gocyclo to \$$GOBIN"
	@echo ""
	@echo "$(YELLOW)Docker:$(RESET)"
	@echo "  $(GREEN)docker-build$(RESET)              Build the image locally (kwd:$(VERSION))"
	@echo "  $(GREEN)docker-scan$(RESET)               Build and grype-scan the image (requires Docker)"
	@echo ""
	@echo "$(YELLOW)Release:$(RESET)"
	@echo "  $(GREEN)release-check$(RESET)             VERSION semver + man .TH + lint + test + cover-check + security + docker-scan"
	@echo "  $(GREEN)release$(RESET)                   release-check then goreleaser (only from main)"
	@echo "  $(GREEN)snapshot$(RESET)                  Goreleaser snapshot to $(DIST)/ (no tag)"
	@echo ""
	@echo "$(CYAN)Current version:$(RESET) $$(cat VERSION 2>/dev/null | tr -d '\n\r' || echo '?') (ldflags $(VERSION), branch $(BRANCH))"
	@echo ""
	@echo "$(CYAN)Examples:$(RESET)"
	@echo "  make build"
	@echo "  make cover-check"
	@echo "  make release-check"

.PHONY: build build-all install clean docker-build docker-scan install-man install-kubectl-plugin
.PHONY: test cover cover-check lint lint-fix tools security release-check release snapshot
.PHONY: check-pins check-man-version check-docker govulncheck gocyclo grype

build:
	@mkdir -p bin
	go build -trimpath $(LDFLAGS) -o bin/$(BINARY) ./cmd/kwd

build-all:
	@mkdir -p $(DIST)
	GOOS=linux GOARCH=amd64 go build -trimpath $(LDFLAGS) -o $(DIST)/$(BINARY)-linux-amd64 ./cmd/kwd
	GOOS=linux GOARCH=arm64 go build -trimpath $(LDFLAGS) -o $(DIST)/$(BINARY)-linux-arm64 ./cmd/kwd
	GOOS=darwin GOARCH=amd64 go build -trimpath $(LDFLAGS) -o $(DIST)/$(BINARY)-darwin-amd64 ./cmd/kwd
	GOOS=darwin GOARCH=arm64 go build -trimpath $(LDFLAGS) -o $(DIST)/$(BINARY)-darwin-arm64 ./cmd/kwd
	GOOS=windows GOARCH=amd64 go build -trimpath $(LDFLAGS) -o $(DIST)/$(BINARY)-windows-amd64.exe ./cmd/kwd
	GOOS=freebsd GOARCH=amd64 go build -trimpath $(LDFLAGS) -o $(DIST)/$(BINARY)-freebsd-amd64 ./cmd/kwd
	GOOS=freebsd GOARCH=arm64 go build -trimpath $(LDFLAGS) -o $(DIST)/$(BINARY)-freebsd-arm64 ./cmd/kwd
	GOOS=openbsd GOARCH=amd64 go build -trimpath $(LDFLAGS) -o $(DIST)/$(BINARY)-openbsd-amd64 ./cmd/kwd
	GOOS=openbsd GOARCH=arm64 go build -trimpath $(LDFLAGS) -o $(DIST)/$(BINARY)-openbsd-arm64 ./cmd/kwd

install:
	go install -trimpath $(LDFLAGS) ./cmd/kwd

clean:
	rm -f bin/$(BINARY) coverage.out
	rm -rf $(DIST)

# check-docker fails fast with an actionable message instead of letting a
# Docker-dependent target fail halfway through.
check-docker:
	@command -v docker >/dev/null 2>&1 || { echo "Error: docker not found — install Docker (or skip the Docker gates)."; exit 1; }
	@docker info >/dev/null 2>&1 || { echo "Error: the Docker daemon is not reachable — start Docker."; exit 1; }

docker-build: check-docker
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) \
	  --build-arg BUILDDATE=$(BUILDDATE) --build-arg BRANCH=$(BRANCH) \
	  -t kwd:$(VERSION) .

docker-scan: check-docker
	@docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) \
	  --build-arg BUILDDATE=$(BUILDDATE) --build-arg BRANCH=$(BRANCH) \
	  -t kwd:scan .
	@if ! command -v grype >/dev/null 2>&1; then \
	  curl -sSfL https://get.anchore.io/grype | sh -s -- -b $(shell go env GOPATH)/bin; \
	fi
	grype kwd:scan -c .grype.yaml --fail-on high

install-man:
	@mkdir -p $(DESTDIR)$(MANDIR)/man1
	install -m644 contrib/man/man1/kwd.1 $(DESTDIR)$(MANDIR)/man1/kwd.1
	install -m644 contrib/man/man1/kubectl-kwd.1 $(DESTDIR)$(MANDIR)/man1/kubectl-kwd.1

install-kubectl-plugin:
	@mkdir -p $(DESTDIR)$(BINDIR)
	go build -trimpath $(LDFLAGS) -o $(DESTDIR)$(BINDIR)/$(PLUGIN) ./cmd/kwd

test:
	go test -race ./...

cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	@go tool cover -func=coverage.out | tail -1

cover-check:
	go test ./internal/... -coverprofile=coverage.out -covermode=atomic
	@pct=$$(go tool cover -func=coverage.out | grep '^total:' | awk '{print $$NF}' | tr -d '%'); \
	echo "Total statement coverage (internal/): $$pct% (minimum $(COVERAGE_MIN)% )"; \
	awk -v p="$$pct" -v m="$(COVERAGE_MIN)" 'BEGIN { if (p+0 < m+0) { print "Error: coverage is below " m "% — add tests or set COVERAGE_MIN="; exit 1 } }'

# check-pins keeps the CVE pins from being walked back; check-man-version keeps
# the man pages honest against VERSION. Both gate `lint`, hence release-check.
check-pins:
	@sh contrib/scripts/check-dependency-pins.sh

check-man-version:
	@expected="kwd v$$(tr -d '\n\r' < VERSION)"; \
	status=0; \
	for page in contrib/man/man1/kwd.1 contrib/man/man1/kubectl-kwd.1; do \
		got=$$(grep -m1 '^\.TH' "$$page" | sed -n 's/.*"\(kwd v[^"]*\)".*/\1/p'); \
		if [ "$$got" != "$$expected" ]; then \
			echo "check-man-version: FAIL ($$page .TH says '$$got', VERSION is '$$expected')"; \
			status=1; \
		else \
			echo "check-man-version: PASS ($$page $$got)"; \
		fi; \
	done; \
	exit $$status

gocyclo:
	@echo "Running gocyclo (complexity <= 14)..."
	@go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
	@"$(shell go env GOPATH)/bin/gocyclo" -over 14 .

grype:
	@echo "Running grype (working tree, --fail-on high)..."
	@grype . -c .grype.yaml --fail-on high

lint: check-pins check-man-version
	@echo "Checking gofmt -s..."
	@unformatted=$$(gofmt -s -l .); [ -z "$$unformatted" ] || { echo "Files not formatted (run: make lint-fix):"; echo "$$unformatted"; exit 1; }
	@echo "Running go vet..."
	@go vet ./...
	@$(MAKE) --no-print-directory gocyclo

lint-fix:
	gofmt -s -w .

tools:
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/fzipp/gocyclo/cmd/gocyclo@latest

govulncheck:
	@echo "Running govulncheck..."
	@tmp=$$(mktemp); \
	go run golang.org/x/vuln/cmd/govulncheck@latest ./... >"$$tmp" 2>&1 || true; \
	output=$$(grep -v '^exit status [0-9]*$$' "$$tmp" || true); \
	rm -f "$$tmp"; \
	ignored_ids=$$(grep 'id:' .govulncheck-ignore.yaml 2>/dev/null | cut -d'"' -f2); \
	total_vulns=$$(echo "$$output" | grep -c 'Vulnerability #' || true); \
	matching=0; \
	for id in $$ignored_ids; do \
		c=$$(echo "$$output" | grep -c "^Vulnerability.*$$id" || true); \
		matching=$$((matching + c)); \
	done; \
	if [ $$total_vulns -eq 0 ]; then \
		echo "$$output"; \
		echo ""; \
		echo "=== security: PASS (govulncheck clean) ==="; \
	elif [ $$total_vulns -eq $$matching ]; then \
		echo "$$output"; \
		echo ""; \
		echo "=== security: PASS (known false positives only) ==="; \
		echo "govulncheck reported $$matching advisories filtered — see .govulncheck-ignore.yaml."; \
	else \
		echo "$$output"; \
		echo ""; \
		echo "ERROR: $$((total_vulns - matching)) unfiltered govulncheck finding(s)."; \
		echo "Add to .govulncheck-ignore.yaml only with documented false-positive rationale."; \
		exit 1; \
	fi

security: govulncheck gocyclo grype

.PHONY: release-check
release-check:
	@set -e; \
	test -f VERSION || { echo "Error: VERSION file is required"; exit 1; }; \
	ver_raw=$$(cat VERSION | tr -d '\n\r'); ver=$${ver_raw#v}; \
	echo "$$ver" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "Error: VERSION must be semantic MAJOR.MINOR.PATCH (got: $$ver_raw)"; exit 1; }; \
	echo "Release version: $$ver (tag v$$ver)"
	@$(MAKE) lint
	@$(MAKE) test
	@$(MAKE) cover-check
	@$(MAKE) security
	@$(MAKE) docker-scan
	@echo "All release checks passed."

release: release-check
	@branch=$$(git branch --show-current 2>/dev/null); \
	if [ "$$branch" != "main" ]; then \
	  echo "Error: release only from main (current: $$branch). Merge develop → main first."; \
	  exit 1; \
	fi; \
	goreleaser release --clean

snapshot:
	@ver_raw=$$(cat VERSION 2>/dev/null | tr -d '\n\r'); \
	[ -n "$$ver_raw" ] || { echo "Error: VERSION file is required for snapshot"; exit 1; }; \
	ver=$${ver_raw#v}; \
	echo "$$ver" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "Error: VERSION must be semantic MAJOR.MINOR.PATCH (got: $$ver_raw)"; exit 1; }; \
	KWD_SNAPSHOT_VERSION="$$ver-next" goreleaser release --snapshot --clean
