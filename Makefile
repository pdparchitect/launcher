SHELL := /bin/bash
.DEFAULT_GOAL := help

VERSION ?= $(shell tr -d '[:space:]' < VERSION)
LDFLAGS := -s -w -X main.version=$(VERSION)
WEB_LISTEN ?= 127.0.0.1:16900
HOST_OS := $(shell go env GOOS)
# Built against whatever SDK the toolchain has (macOS 26 in CI, so the app gets
# the current design system), but still runnable on the oldest macOS Launcher
# supports. Without this, clang defaults the target to the build host and the
# binary refuses to launch on anything older.
MACOS_DEPLOYMENT_TARGET ?= 15.0
DESKTOP_TAGS := desktop
PATCHED_GO := ./scripts/with-go-module-patches.sh
SWIFT_LINKER := $(abspath scripts/swift-linker.sh)
NATIVE_MACOS_TARGET :=
DESKTOP_LDFLAGS := $(LDFLAGS)
ifeq ($(HOST_OS),linux)
DESKTOP_TAGS := desktop,webkit2_41
endif
ifeq ($(HOST_OS),darwin)
NATIVE_MACOS_TARGET := native-macos
# The wrapper delegates the final Mach-O link to the Swift driver so it adds
# the statically linked Swift object's runtime and autolink dependencies.
DESKTOP_LDFLAGS := $(LDFLAGS) -linkmode external -extld $(SWIFT_LINKER)
endif
# Every binary we mint ships with the Web Inspector: Cmd+Option+I on macOS,
# and the WebKitGTK inspector on Linux. Build with DEVTOOLS=0 to leave it out —
# needed only for a Mac App Store submission, since Wails' inspector shim calls
# the private _WKInspector API. Must follow DESKTOP_TAGS, which it extends.
DEVTOOLS ?= 1
ifeq ($(DEVTOOLS),1)
MACOS_BUILD_FLAGS := -devtools
DESKTOP_TAGS := $(DESKTOP_TAGS),devtools
endif

.PHONY: help web web-open desktop check test images-check images-build build build-desktop native-macos build-macos build-all clean

help:
	@echo "Launcher development"
	@echo
	@echo "  make web        Run the web interface for remote development"
	@echo "                  ?chrome=macos previews the packaged macOS layout"
	@echo "  make web-open   Run the web interface and open a local browser"
	@echo "  make desktop    Run the frameless Wails desktop application"
	@echo "  make check      Format-check, test, and vet Launcher"
	@echo "  make images-check  Validate the container image sources"
	@echo "  make images-build  Build the Ubuntu, desktop, and Hermes image chain"
	@echo "  make build      Build Launcher for this machine"
	@echo "  make build-desktop  Build the Wails desktop executable"
	@echo "  make build-macos  Build the single-binary SwiftUI/Wails macOS app"
	@echo "                    Web Inspector is on by default; DEVTOOLS=0 omits it"
	@echo "  make build-all  Cross-compile Linux and macOS binaries"
	@echo "  make clean      Remove generated binaries"

web:
	go run . serve --no-open --listen "$(WEB_LISTEN)"

web-open:
	go run . serve --listen "$(WEB_LISTEN)"

desktop: $(NATIVE_MACOS_TARGET)
	CGO_ENABLED=1 $(PATCHED_GO) go run -tags "$(DESKTOP_TAGS)" . desktop

check:
	@test -z "$$(gofmt -l .)" || { \
		echo "Run gofmt on:"; \
		gofmt -l .; \
		exit 1; \
	}
	go test ./...
	go vet ./...
	bash -n $(PATCHED_GO)
	bash -n scripts/swift-linker.sh
	GOOS=darwin $(PATCHED_GO) go list ./internal/desktop >/dev/null
	GOOS=darwin $(PATCHED_GO) go list github.com/wailsapp/wails/v2/cmd/wails >/dev/null
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

build-desktop: $(NATIVE_MACOS_TARGET)
	mkdir -p dist
	CGO_ENABLED=1 $(PATCHED_GO) go build \
		-tags "$(DESKTOP_TAGS),production" -trimpath \
		-ldflags "$(DESKTOP_LDFLAGS)" -o dist/launcher-desktop .

native-macos:
	@if [ "$(HOST_OS)" != "darwin" ]; then \
		echo "make native-macos must run on macOS"; \
		exit 1; \
	fi
	MACOSX_DEPLOYMENT_TARGET=$(MACOS_DEPLOYMENT_TARGET) \
		swift build \
		--package-path macos \
		-c release \
		--arch arm64 \
		--product LauncherNative

build-macos: native-macos
	@if [ "$(HOST_OS)" != "darwin" ]; then \
		echo "make build-macos must run on macOS (use the Build workflow from Linux)"; \
		exit 1; \
	fi
	cp internal/httpapi/web/assets/logo.png build/appicon.png
	MACOSX_DEPLOYMENT_TARGET=$(MACOS_DEPLOYMENT_TARGET) $(PATCHED_GO) go run \
		github.com/wailsapp/wails/v2/cmd/wails build \
		-platform darwin/arm64 \
		$(MACOS_BUILD_FLAGS) \
		-s \
		-skipbindings \
		-skipembedcreate \
		-trimpath \
		-ldflags "$(DESKTOP_LDFLAGS)"

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
	@if [ -d macos/.build ]; then rm -r macos/.build; fi
