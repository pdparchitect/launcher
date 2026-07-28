# syntax=docker/dockerfile:1.7

FROM ubuntu:24.04

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

ARG TARGETARCH
ENV DEBIAN_FRONTEND=noninteractive

RUN arch="${TARGETARCH:-$(dpkg --print-architecture)}"; \
    case "$arch" in \
        amd64|arm64) ;; \
        *) echo "Ubuntu core does not support linux/$arch" >&2; exit 1 ;; \
    esac

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    coreutils \
    curl \
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
    python3-dev \
    python3-pip \
    python3-venv \
    python-is-python3 \
    ripgrep \
    socat \
    sudo \
    tar \
    unzip \
    wget \
    xz-utils \
    zip \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /app /data /outputs /workspace

LABEL org.opencontainers.image.title="Launcher Ubuntu Core" \
    org.opencontainers.image.description="Ubuntu foundation for Launcher-managed agent images" \
    org.opencontainers.image.source="https://github.com/pdparchitect/launcher"

WORKDIR /workspace
