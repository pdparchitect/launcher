SHELL := /bin/bash
.DEFAULT_GOAL := help

VERSION ?= $(shell tr -d '[:space:]' < VERSION)
LDFLAGS := -s -w -X main.version=$(VERSION)
WEB_LISTEN ?= 127.0.0.1:16900
HOST_OS := $(shell go env GOOS)
DESKTOP_TAGS := desktop
WAILS_GO := ./scripts/with-wails-context-menu-fix.sh
ifeq ($(HOST_OS),linux)
DESKTOP_TAGS := desktop,webkit2_41
endif

.PHONY: help web web-open desktop check test images-check images-build build build-desktop build-macos build-all clean

help:
	@echo "Launcher development"
	@echo
	@echo "  make web        Run the web interface for remote development"
	@echo "  make web-open   Run the web interface and open a local browser"
	@echo "  make desktop    Run the frameless Wails desktop application"
	@echo "  make check      Format-check, test, and vet Launcher"
	@echo "  make images-check  Validate the container image sources"
	@echo "  make images-build  Build the Ubuntu, desktop, and Hermes image chain"
	@echo "  make build      Build Launcher for this machine"
	@echo "  make build-desktop  Build the Wails desktop executable"
	@echo "  make build-macos  Build an Apple silicon macOS application (on macOS)"
	@echo "  make build-all  Cross-compile Linux and macOS binaries"
	@echo "  make clean      Remove generated binaries"

web:
	go run . serve --no-open --listen "$(WEB_LISTEN)"

web-open:
	go run . serve --listen "$(WEB_LISTEN)"

desktop:
	CGO_ENABLED=1 $(WAILS_GO) go run -tags "$(DESKTOP_TAGS)" . desktop

check:
	@test -z "$$(gofmt -l .)" || { \
		echo "Run gofmt on:"; \
		gofmt -l .; \
		exit 1; \
	}
	go test ./...
	go vet ./...
	bash -n $(WAILS_GO)
	GOOS=darwin $(WAILS_GO) go list ./internal/desktop >/dev/null
	$(MAKE) --directory images check

test:
	go test ./...

images-check:
	$(MAKE) --directory images check

images-build:
	$(MAKE) --directory images build

build:
	mkdir -p dist
	go build -trimpath -ldflags "$(LDFLAGS)" -o dist/launcher .

build-desktop:
	mkdir -p dist
	CGO_ENABLED=1 $(WAILS_GO) go build \
		-tags "$(DESKTOP_TAGS),production" -trimpath \
		-ldflags "$(LDFLAGS)" -o dist/launcher-desktop .

build-macos:
	@if [ "$(HOST_OS)" != "darwin" ]; then \
		echo "make build-macos must run on macOS (use the Build workflow from Linux)"; \
		exit 1; \
	fi
	cp internal/httpapi/web/assets/logo.png build/appicon.png
	$(WAILS_GO) go run \
		github.com/wailsapp/wails/v2/cmd/wails build \
		-platform darwin/arm64 \
		-s \
		-skipbindings \
		-skipembedcreate \
		-trimpath \
		-ldflags "$(LDFLAGS)"

build-all:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -trimpath -ldflags "$(LDFLAGS)" \
		-o dist/launcher-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build -trimpath -ldflags "$(LDFLAGS)" \
		-o dist/launcher-linux-arm64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
		go build -trimpath -ldflags "$(LDFLAGS)" \
		-o dist/launcher-darwin-arm64 .

clean:
	@if [ -d dist ]; then rm -r dist; fi
	@if [ -d build/bin ]; then rm -r build/bin; fi
	@if [ -d build/wailsjs ]; then rm -r build/wailsjs; fi
	@if [ -f build/appicon.png ]; then rm build/appicon.png; fi
