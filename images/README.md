# Launcher images

This directory contains the source for images consumed by Launcher. The first
inheritance chain is:

```text
core/ubuntu
  -> bases/desktop
       -> products/hermes/desktop
```

## Responsibilities

`core/ubuntu` is a non-runnable Ubuntu 24.04 foundation. It contains common
shell, Git, network, archive, and Python tooling but no user, port, health
check, or entrypoint.

`bases/desktop` adds the neutral browser desktop:

- KasmVNC and Openbox
- Chrome on AMD64 and Chromium on ARM64
- a non-root `agent` user
- `/workspace`
- a generic desktop entrypoint and health check
- fixed-ownership mount handling for Apple `container`

`products/hermes/desktop` inherits the desktop base and adds a source-pinned
Hermes Agent installation. Hermes state persists in `/home/agent/.hermes`;
the first desktop session opens `hermes setup`, and later sessions open the
Hermes TUI.

Hermes is pinned to tag `v2026.7.20` and resolved commit
`3ef6bbd201263d354fd83ec55b3c306ded2eb72a`. Update both values together after
reviewing an upstream release.

## Build

From this directory:

```sh
make check
make build
```

The build is deliberately ordered. Each child receives the locally built
parent image through an explicit build argument:

```sh
make core-ubuntu
make desktop
make hermes-desktop
```

Build ARM64 images with:

```sh
make build PLATFORM=linux/arm64
```

The local output images are:

```text
pdparchitect/launcher-core-ubuntu:local
pdparchitect/launcher-base-desktop:local
pdparchitect/hermes-desktop:local
```

Launcher should catalogue only the final product image. Core and base images
are build-time implementation details rather than installable applications.
