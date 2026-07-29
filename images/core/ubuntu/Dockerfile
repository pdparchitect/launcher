# syntax=docker/dockerfile:1.7
#
# Ubuntu foundation for Launcher-managed images.
#
# This layer is deliberately minimal: operating system tooling only. Language
# runtimes live in `runtimes/*` and the desktop lives in `bases/desktop`, so an
# image that needs neither pays for neither. Nothing here makes the image
# runnable - no user, port, health check, or entrypoint.

FROM ubuntu:24.04

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

ARG TARGETARCH
ENV DEBIAN_FRONTEND=noninteractive

RUN arch="${TARGETARCH:-$(dpkg --print-architecture)}"; \
    case "$arch" in \
        amd64|arm64) ;; \
        *) echo "Ubuntu core does not support linux/$arch" >&2; exit 1 ;; \
    esac

# Shell, network, archive, VCS, and scripting tooling shared by every image.
# The Python data stack (numpy, pandas, scipy, ipython) and every language
# runtime are deliberately absent - see runtimes/.
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    coreutils \
    curl \
    dnsutils \
    file \
    git \
    git-lfs \
    gnupg \
    haveged \
    iproute2 \
    jq \
    openssh-client \
    openssl \
    procps \
    python3 \
    python3-pip \
    python-is-python3 \
    ripgrep \
    socat \
    sqlite3 \
    sudo \
    tar \
    unzip \
    vim \
    wget \
    xz-utils \
    zip \
    && rm -rf /var/lib/apt/lists/*

RUN ln -sf /usr/bin/pip3 /usr/bin/pip

RUN mkdir -p /app /data /outputs /workspace

LABEL org.opencontainers.image.title="Launcher Ubuntu Core" \
    org.opencontainers.image.description="Minimal Ubuntu foundation for Launcher-managed images" \
    org.opencontainers.image.source="https://github.com/pdparchitect/launcher"

WORKDIR /workspace
