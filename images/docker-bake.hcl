# Launcher image build graph.
#
# Every image is a target. A child receives its parent through a named context
# bound to `target:<parent>`, so BuildKit resolves the whole chain inside one
# build. Two consequences matter:
#
#   1. Order is derived, never declared. Building `hermes-desktop` builds
#      `core-ubuntu` and `desktop` first because it depends on them.
#   2. A change anywhere propagates downward. Editing a file under
#      `core/ubuntu` produces a different core image, which invalidates the
#      `FROM` of `desktop`, which invalidates the `FROM` of `hermes-desktop`.
#      Editing only `products/hermes/desktop` rebuilds just that image.
#
# The named context keys below are the image references the child Dockerfiles
# resolve their `FROM` to by default. Matching those strings is what links the
# targets, so the Dockerfiles stay buildable on their own with plain
# `docker build`.

variable "REGISTRY" {
  # Empty means local, unqualified names. Release builds set this to the
  # registry namespace the images are published under.
  default = ""
}

variable "TAG" {
  default = "local"
}

variable "RELEASE_VERSION" {
  # The architecture-neutral identity recorded in image metadata. Publishing
  # uses architecture-suffixed staging tags, so this must be independent from
  # TAG.
  default = "local"
}

variable "EXTRA_TAGS" {
  # Comma-separated additional tags, e.g. "latest,sha-abc1234". The primary TAG
  # should stay immutable; moving tags belong here.
  default = ""
}

variable "SUBSTRATE_VERSION" {
  # Stamped onto product images so each one records the substrate release it
  # was built against. Set by the Makefile from the root VERSION file.
  default = ""
}

variable "PLATFORMS" {
  # Empty means the builder's native platform. Release builds set the full
  # platform list.
  default = ""
}

# Every image in this chain is published as `launcher-image-*`, including the
# products. The prefix keeps the whole chain in one obvious group instead of
# scattering four repositories through the account's package namespace.
function "ref" {
  params = [image]
  result = REGISTRY == "" ? "pdparchitect/launcher-image-${image}" : "${REGISTRY}/launcher-image-${image}"
}

function "tag_list" {
  params = [image]
  result = concat(
    ["${ref(image)}:${TAG}"],
    EXTRA_TAGS == "" ? [] : [for extra in split(",", EXTRA_TAGS) : "${ref(image)}:${extra}"]
  )
}

# The default `docker` driver builds for one platform only. Leaving PLATFORMS
# empty keeps local development on that fast path.
function "platform_list" {
  params = []
  result = PLATFORMS == "" ? [] : split(",", PLATFORMS)
}

group "default" {
  targets = ["hermes-desktop"]
}

group "all" {
  targets = [
    "core-ubuntu", "node", "desktop",
    "hermes-desktop", "openclaw-desktop", "petbox-desktop",
  ]
}

# The substrate releases as one unit: each layer embeds its parent, so a change
# to the core necessarily produces a new desktop. Products release
# independently - a wallpaper change in one product must not re-mint the core
# or every other product.
group "substrate" {
  targets = ["core-ubuntu", "node", "desktop"]
}

group "products" {
  targets = ["hermes-desktop", "openclaw-desktop", "petbox-desktop"]
}

target "_common" {
  platforms = platform_list()
  args = {
    IMAGE_VERSION = RELEASE_VERSION
  }
  labels = {
    "org.opencontainers.image.source"  = "https://github.com/pdparchitect/launcher"
    "org.opencontainers.image.version" = RELEASE_VERSION
  }
}

target "core-ubuntu" {
  inherits   = ["_common"]
  context    = "core/ubuntu"
  dockerfile = "Dockerfile"
  tags       = tag_list("core-ubuntu")
}

target "node" {
  inherits   = ["_common"]
  context    = "runtimes/node"
  dockerfile = "Dockerfile"
  tags       = tag_list("runtime-node")

  # Overrides `FROM ${CORE_IMAGE}` with the freshly built core-ubuntu target.
  contexts = {
    "pdparchitect/launcher-image-core-ubuntu:local" = "target:core-ubuntu"
  }
}

target "desktop" {
  inherits   = ["_common"]
  context    = "bases/desktop"
  dockerfile = "Dockerfile"
  tags       = tag_list("base-desktop")

  # Overrides `FROM ${NODE_IMAGE}` with the freshly built node target.
  contexts = {
    "pdparchitect/launcher-image-runtime-node:local" = "target:node"
  }
}

target "hermes-desktop" {
  inherits   = ["_common"]
  labels = {
    "dev.pdparchitect.launcher.substrate.version" = SUBSTRATE_VERSION
  }
  context    = "products/hermes/desktop"
  dockerfile = "Dockerfile"
  tags       = tag_list("hermes-desktop")

  # Overrides `FROM ${DESKTOP_IMAGE}` with the freshly built desktop target.
  contexts = {
    "pdparchitect/launcher-image-base-desktop:local" = "target:desktop"
  }
}

target "openclaw-desktop" {
  inherits = ["_common"]
  labels = {
    "dev.pdparchitect.launcher.substrate.version" = SUBSTRATE_VERSION
  }
  context    = "products/openclaw/desktop"
  dockerfile = "Dockerfile"
  tags       = tag_list("openclaw-desktop")

  # Overrides `FROM ${DESKTOP_IMAGE}` with the freshly built desktop target.
  contexts = {
    "pdparchitect/launcher-image-base-desktop:local" = "target:desktop"
  }
}

target "petbox-desktop" {
  inherits = ["_common"]
  labels = {
    "dev.pdparchitect.launcher.substrate.version" = SUBSTRATE_VERSION
  }
  context    = "products/petbox/desktop"
  dockerfile = "Dockerfile"
  tags       = tag_list("petbox-desktop")

  # Overrides `FROM ${DESKTOP_IMAGE}` with the freshly built desktop target.
  contexts = {
    "pdparchitect/launcher-image-base-desktop:local" = "target:desktop"
  }
}
