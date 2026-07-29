# syntax=docker/dockerfile:1.7
#
# Node.js runtime layer.
#
# Split out of the core so an image that needs no JavaScript runtime does not
# carry ~200MB of it. Versions match the Pantalk Ghost toolchain, which is the
# reference implementation for this chain.

ARG CORE_IMAGE=pdparchitect/launcher-image-core-ubuntu:local
FROM ${CORE_IMAGE}

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

ARG TARGETARCH

RUN curl -fsSL https://deb.nodesource.com/setup_24.x | bash - && \
    apt-get install -y --no-install-recommends nodejs && \
    rm -rf /var/lib/apt/lists/* && \
    node --version && npm --version

# Pin the package manager so it cannot drift between image rebuilds.
#
# COREPACK_HOME must be set and shared. Corepack stores the activated version
# under the *building* user's home, which is root's - so with the default the
# pin is invisible to the unprivileged desktop account, and `pnpm` silently
# downloads the newest release on first use at run time. Putting the cache on a
# world-readable system path makes the pin hold for every user, and removes a
# network call from the runtime path.
ENV COREPACK_HOME=/usr/local/share/corepack
ARG PNPM_VERSION=10.13.1
RUN corepack enable && \
    corepack prepare "pnpm@${PNPM_VERSION}" --activate && \
    chmod -R a+rX "$COREPACK_HOME" && \
    test "$(pnpm --version)" = "${PNPM_VERSION}"

ARG YQ_VERSION=4.44.6
RUN arch="${TARGETARCH:-$(dpkg --print-architecture)}"; \
    curl -fsSL \
        "https://github.com/mikefarah/yq/releases/download/v${YQ_VERSION}/yq_linux_${arch}" \
        -o /usr/local/bin/yq && \
    chmod 0755 /usr/local/bin/yq && \
    yq --version

LABEL org.opencontainers.image.title="Launcher Node Runtime" \
    org.opencontainers.image.description="Node.js, pnpm, and yq on the Launcher Ubuntu core" \
    org.opencontainers.image.source="https://github.com/pdparchitect/launcher"

WORKDIR /workspace
