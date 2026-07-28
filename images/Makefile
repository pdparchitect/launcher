SHELL := /bin/bash
.DEFAULT_GOAL := help

DOCKER ?= docker
NATIVE_ARCH := $(shell uname -m | sed \
	-e 's/^x86_64$$/amd64/' \
	-e 's/^aarch64$$/arm64/')
PLATFORM ?= linux/$(NATIVE_ARCH)
TARGETARCH ?= $(word 2,$(subst /, ,$(PLATFORM)))

CORE_UBUNTU_IMAGE ?= pdparchitect/launcher-core-ubuntu:local
DESKTOP_IMAGE ?= pdparchitect/launcher-base-desktop:local
HERMES_DESKTOP_IMAGE ?= pdparchitect/hermes-desktop:local
HERMES_VERSION ?= v2026.7.20
HERMES_SOURCE_SHA ?= 3ef6bbd201263d354fd83ec55b3c306ded2eb72a

.PHONY: help check build core-ubuntu desktop hermes-desktop

help:
	@echo "Launcher image sources"
	@echo
	@echo "  make check           Validate image source and inheritance"
	@echo "  make core-ubuntu     Build $(CORE_UBUNTU_IMAGE)"
	@echo "  make desktop         Build $(DESKTOP_IMAGE)"
	@echo "  make hermes-desktop  Build $(HERMES_DESKTOP_IMAGE)"
	@echo "  make build           Build the complete inheritance chain"
	@echo
	@echo "Override PLATFORM=linux/arm64 to build the ARM64 chain."

check:
	bash tests/test-images.sh

core-ubuntu:
	$(DOCKER) build \
		--platform "$(PLATFORM)" \
		--build-arg "TARGETARCH=$(TARGETARCH)" \
		--tag "$(CORE_UBUNTU_IMAGE)" \
		core/ubuntu

desktop: core-ubuntu
	$(DOCKER) build \
		--platform "$(PLATFORM)" \
		--build-arg "TARGETARCH=$(TARGETARCH)" \
		--build-arg "CORE_IMAGE=$(CORE_UBUNTU_IMAGE)" \
		--tag "$(DESKTOP_IMAGE)" \
		bases/desktop

hermes-desktop: desktop
	$(DOCKER) build \
		--platform "$(PLATFORM)" \
		--build-arg "DESKTOP_IMAGE=$(DESKTOP_IMAGE)" \
		--build-arg "HERMES_VERSION=$(HERMES_VERSION)" \
		--build-arg "HERMES_SOURCE_SHA=$(HERMES_SOURCE_SHA)" \
		--tag "$(HERMES_DESKTOP_IMAGE)" \
		products/hermes/desktop

build: hermes-desktop
