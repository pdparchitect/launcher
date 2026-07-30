SHELL := /bin/bash
.DEFAULT_GOAL := help

DOCKER ?= docker
BAKE := $(DOCKER) buildx bake

# Local development defaults. An empty REGISTRY yields unqualified
# `pdparchitect/...` names; an empty PLATFORMS yields the builder's native
# platform. Release builds override all three.
REGISTRY ?=
TAG ?= local
RELEASE_VERSION ?= $(TAG)
EXTRA_TAGS ?=
PLATFORMS ?=

# The substrate shares the VERSION at the root of images/. A product declares
# its own VERSION next to its Dockerfile; tools/version-of.sh applies that rule
# so adding a product needs no change here.
SUBSTRATE_VERSION := $(shell tr -d '[:space:]' < VERSION 2>/dev/null)
version-of = $(shell bash tools/version-of.sh $(1))
image-version-of = $(shell bash tools/units.sh | \
	awk -F '\t' -v target='$(1)' '$$2 == target { print $$6 }')

# Multi-platform and registry output need the container driver. The default
# `docker` driver is single-platform and is the fast path for local work, so
# BUILDER is empty until a build asks for something that driver cannot do.
BUILDER_NAME ?= launcher-images
BUILDER ?=
BUILDER_FLAG := $(if $(BUILDER),--builder $(BUILDER),)
GROUP ?= all

BAKE_ENV := REGISTRY="$(REGISTRY)" TAG="$(TAG)" \
	RELEASE_VERSION="$(RELEASE_VERSION)" EXTRA_TAGS="$(EXTRA_TAGS)" \
	PLATFORMS="$(PLATFORMS)" SUBSTRATE_VERSION="$(SUBSTRATE_VERSION)"
BAKE_CMD := $(BAKE) $(BUILDER_FLAG)

# `make run` serves a target from the build graph. TARGET names which one, and
# the image reference, the ports, and the state volumes are all derived from
# that target rather than hardcoded, so a new product needs no change here.
#
# The default is the desktop base, not a product. It is the layer most local
# work happens in and the cheapest to rebuild, so `make run` stays a fast loop.
# Name a product explicitly to run one.
#
# RUN_STATE names the volume prefix that keeps a session's state across
# rebuilds. Setting it empty runs a throwaway session.
TARGET ?= desktop
RUN_IMAGE ?=
RUN_PORT ?= 6901
RUN_PREVIEW_PORT ?= 6902
RUN_STATE ?= launcher-$(TARGET)

.PHONY: help check smoke print builder core-ubuntu desktop hermes-desktop \
	openclaw-desktop petbox-desktop build rebuild images run push \
	push-substrate push-product versions clean

help:
	@echo "Launcher image sources"
	@echo
	@echo "  make check           Validate image sources and the build graph"
	@echo "  make smoke           Run the built product image and assert it comes up"
	@echo "  make print           Show the resolved build graph as JSON"
	@echo "  make build           Build the product image and everything it needs"
	@echo "  make rebuild         Build again from scratch, ignoring all cache"
	@echo "  make images          Build and load every image in the chain"
	@echo "  make run             Build and serve the desktop base on localhost"
	@echo "  make run TARGET=hermes-desktop   Serve a product image instead"
	@echo
	@echo "  make core-ubuntu     Build only the Ubuntu core"
	@echo "  make desktop         Build only the desktop base"
	@echo "  make hermes-desktop  Build only the Hermes product image"
	@echo "  make openclaw-desktop    Build only the OpenClaw product image"
	@echo "  make petbox-desktop  Build only the Petbox product image"
	@echo
	@echo "Ordering is derived from the graph in docker-bake.hcl, so any target"
	@echo "builds the parents it depends on and rebuilds when they change."
	@echo
	@echo "  make versions        Show the release version of every target"
	@echo "  make push-substrate  Publish core+node+desktop at the root VERSION"
	@echo "  make push-product TARGET=hermes-desktop   Publish one product"
	@echo "  make builder         Create the container driver builder"
	@echo "  make build PLATFORMS=linux/amd64,linux/arm64 TAG=v1 REGISTRY=ghcr.io/pdparchitect"

# Targets that only need the parent as a dependency are built `cacheonly` and
# are not loaded into the image store. Naming one explicitly loads it.
core-ubuntu desktop hermes-desktop openclaw-desktop petbox-desktop:
	$(BAKE_ENV) $(BAKE_CMD) $@

build:
	$(BAKE_ENV) $(BAKE_CMD) hermes-desktop

images:
	$(BAKE_ENV) $(BAKE_CMD) all

rebuild:
	$(BAKE_ENV) $(BAKE_CMD) --no-cache hermes-desktop

print:
	@$(BAKE_ENV) $(BAKE_CMD) --print all

check:
	bash tests/test-workspace.sh
	@set -euo pipefail; while IFS= read -r test_script; do \
		bash "$$test_script"; \
	done < <(find core runtimes bases products -path '*/tests/test-image.sh' | sort)

# Runs the built product image and asserts the desktop session comes up.
smoke:
	bash bases/desktop/tests/smoke-test.sh $(SMOKE_IMAGE)

# Depends on the target, so `make run` can never serve a stale image.
run: $(TARGET)
	@image="$(RUN_IMAGE)"; \
	if [ -z "$$image" ]; then \
		image="$$($(BAKE_ENV) $(BAKE_CMD) --print $(TARGET) 2>/dev/null | \
			jq -r --arg t "$(TARGET)" '.target[$$t].tags[0] // empty')"; \
	fi; \
	if [ -z "$$image" ]; then \
		echo "no image is defined for target '$(TARGET)'" >&2; exit 1; \
	fi; \
	mounts=(); \
	if [ -n "$(RUN_STATE)" ]; then \
		paths="/workspace $$($(DOCKER) image inspect \
			-f '{{range .Config.Env}}{{println .}}{{end}}' "$$image" | \
			sed -n 's/^DESKTOP_PERSISTENT_PATHS=//p')"; \
		for path in $$paths; do \
			volume="$(RUN_STATE)$$(printf '%s' "$$path" | \
				tr -c 'a-zA-Z0-9' '-' | sed 's/-\+/-/g;s/-$$//')"; \
			mounts+=(-v "$$volume:$$path"); \
		done; \
	fi; \
	echo "Desktop: http://localhost:$(RUN_PORT)"; \
	echo "Preview: http://localhost:$(RUN_PREVIEW_PORT)/preview.jpg"; \
	echo "Press Ctrl-C to stop"; \
	$(DOCKER) run --rm -it --shm-size=1g \
		-p $(RUN_PORT):6901 \
		-p $(RUN_PREVIEW_PORT):6902 \
		"$${mounts[@]}" "$$image"

# Prints what each target would publish as.
versions:
	@printf '%-16s %-10s %-8s %s\n' TARGET KIND VERSION FILE; \
	for target in $$($(BAKE_ENV) $(BAKE_CMD) --print all 2>/dev/null | \
			jq -r '.target | keys[]' | sort); do \
		printf '%-16s %-10s %-8s %s\n' "$$target" \
			"$$(bash tools/version-of.sh --kind $$target)" \
			"$$(bash tools/version-of.sh $$target)" \
			"$$(bash tools/version-of.sh --file $$target)"; \
	done

# Publishes the substrate as one unit, at the root VERSION.
push-substrate:
	@$(MAKE) --no-print-directory push GROUP=substrate TAG="$(SUBSTRATE_VERSION)"

# Publishes one product at its own VERSION, leaving every other image alone.
# TARGET defaults to the desktop base for `make run`, so an unset TARGET here
# would silently publish-check the wrong thing. Require it to have been given
# explicitly rather than inherited from that default.
push-product:
	@$(if $(filter command line environment,$(origin TARGET)),:,{ \
		echo "TARGET must name a product, e.g. TARGET=hermes-desktop" >&2; \
		exit 1; \
	})
	@test "$$(bash tools/version-of.sh --kind $(TARGET))" = product || { \
		echo "$(TARGET) is substrate; use make push-substrate" >&2; \
		exit 1; \
	}
	@$(MAKE) --no-print-directory push GROUP="$(TARGET)" \
		TAG="$(call image-version-of,$(TARGET))" \
		RELEASE_VERSION="$(call image-version-of,$(TARGET))"

# Publishes every image in the chain. The docker driver cannot write to a
# registry, so this needs the container driver - pass BUILDER, or run
# `make builder` first and pass BUILDER=$(BUILDER_NAME).
push:
	@test -n "$(REGISTRY)" || { \
		echo "REGISTRY must be set to publish, e.g. ghcr.io/pdparchitect" >&2; \
		exit 1; \
	}
	@test "$(TAG)" != "local" || { \
		echo "TAG must be an immutable release tag to publish, not 'local'" >&2; \
		exit 1; \
	}
	@test -n "$(BUILDER)" || { \
		echo "BUILDER must name a container-driver builder to publish" >&2; \
		exit 1; \
	}
	$(BAKE_ENV) $(BAKE_CMD) --push $(GROUP)

builder:
	$(DOCKER) buildx inspect $(BUILDER_NAME) >/dev/null 2>&1 || \
		$(DOCKER) buildx create --name $(BUILDER_NAME) --driver docker-container --bootstrap

clean:
	-$(DOCKER) image rm \
		pdparchitect/launcher-image-hermes-desktop:$(TAG) \
		pdparchitect/launcher-image-base-desktop:$(TAG) \
		pdparchitect/launcher-image-runtime-node:$(TAG) \
		pdparchitect/launcher-image-core-ubuntu:$(TAG)
