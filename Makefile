.PHONY: build install test test-run tdd test-race cover cover-check fmt fmt-check vet check cross-build snapshot clean

VERSION ?= 0.1.20
COVERAGE_MIN ?= 72.4
PKG ?= ./...
RUN ?=
GO_TEST_FLAGS = $(if $(RUN),-run '$(RUN)',)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || printf none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/kan ./cmd/kan

install:
	CGO_ENABLED=0 go install -trimpath -ldflags "$(LDFLAGS)" ./cmd/kan

test:
	CGO_ENABLED=0 go test ./...

# Focused red-green run: make test-run PKG=./internal/tasks RUN=TestName
test-run:
	CGO_ENABLED=0 go test $(GO_TEST_FLAGS) $(PKG)

# Watch mode for the red-green-refactor loop: reruns the focused (or full)
# suite whenever a Go file under cmd/ or internal/ changes. Ctrl-C stops it.
tdd:
	@mkdir -p .cache; \
	stamp=.cache/tdd.stamp; \
	touch $$stamp; \
	echo "TDD watch: go test $(GO_TEST_FLAGS) $(PKG) on every change under cmd/ and internal/ (Ctrl-C stops)"; \
	CGO_ENABLED=0 go test $(GO_TEST_FLAGS) $(PKG); \
	while true; do \
		if [ -n "$$(find cmd internal -name '*.go' -type f -newer $$stamp -print -quit)" ]; then \
			touch $$stamp; \
			printf '\n=== %s ===\n' "$$(date +%H:%M:%S)"; \
			CGO_ENABLED=0 go test $(GO_TEST_FLAGS) $(PKG); \
		fi; \
		sleep 1; \
	done

test-race:
	CGO_ENABLED=1 go test -race ./...

cover:
	CGO_ENABLED=0 go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

cover-check: cover
	@total=$$(go tool cover -func=coverage.out | awk '/^total:/ { sub("%", "", $$3); print $$3 }'); \
	awk -v total="$$total" -v minimum="$(COVERAGE_MIN)" 'BEGIN { if (total + 0 < minimum + 0) { printf "coverage %.1f%% is below %s%%\n", total, minimum; exit 1 } }'

fmt:
	gofmt -w $$(find cmd internal -name '*.go' -type f)

fmt-check:
	test -z "$$(gofmt -l cmd internal)"

vet:
	CGO_ENABLED=0 go vet ./...

check: fmt-check vet cover-check build

cross-build:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/kan_linux_amd64 ./cmd/kan
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/kan_linux_arm64 ./cmd/kan
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/kan_darwin_amd64 ./cmd/kan
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/kan_darwin_arm64 ./cmd/kan
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/kan_windows_amd64.exe ./cmd/kan
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/kan_windows_arm64.exe ./cmd/kan

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf bin dist
