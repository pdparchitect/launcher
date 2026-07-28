# syntax=docker/dockerfile:1.7

ARG DESKTOP_IMAGE=pdparchitect/launcher-base-desktop:local

FROM ghcr.io/astral-sh/uv:0.11.6-python3.13-trixie@sha256:b3c543b6c4f23a5f2df22866bd7857e5d304b67a564f4feab6ac22044dde719b AS uv-source
FROM node:22-bookworm-slim@sha256:7af03b14a13c8cdd38e45058fd957bf00a72bbe17feac43b1c15a689c029c732 AS node-source
FROM ${DESKTOP_IMAGE}

USER root

RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake \
    ffmpeg \
    g++ \
    gcc \
    libffi-dev \
    libolm-dev \
    make \
    && rm -rf /var/lib/apt/lists/*

COPY --from=uv-source /usr/local/bin/uv /usr/local/bin/uv
COPY --from=uv-source /usr/local/bin/uvx /usr/local/bin/uvx
COPY --from=node-source /usr/local/bin/node /usr/local/bin/node
COPY --from=node-source /usr/local/lib/node_modules/npm /usr/local/lib/node_modules/npm
COPY --from=node-source /usr/local/lib/node_modules/corepack /usr/local/lib/node_modules/corepack

RUN ln -sf /usr/local/lib/node_modules/npm/bin/npm-cli.js /usr/local/bin/npm && \
    ln -sf /usr/local/lib/node_modules/npm/bin/npx-cli.js /usr/local/bin/npx && \
    ln -sf /usr/local/lib/node_modules/corepack/dist/corepack.js /usr/local/bin/corepack

ARG HERMES_VERSION=v2026.7.20
ARG HERMES_SOURCE_SHA=3ef6bbd201263d354fd83ec55b3c306ded2eb72a

RUN git clone --branch "${HERMES_VERSION}" --depth 1 \
        https://github.com/NousResearch/hermes-agent.git /opt/hermes && \
    test "$(git -C /opt/hermes rev-parse HEAD)" = "$HERMES_SOURCE_SHA"

WORKDIR /opt/hermes

ENV UV_PYTHON=/usr/bin/python3 \
    PLAYWRIGHT_BROWSERS_PATH=/opt/hermes/.playwright \
    npm_config_install_links=false

# Match the dependency profiles baked into the upstream Hermes container while
# retaining this image's Launcher desktop as the final parent.
RUN uv sync --frozen \
        --extra all \
        --extra messaging \
        --extra anthropic \
        --extra bedrock \
        --extra azure-identity \
        --extra hindsight \
        --extra matrix

RUN npm install --prefer-offline --no-audit --fetch-retries=5 && \
    for attempt in 1 2 3; do \
        npx playwright install --with-deps chromium --only-shell && break; \
        if [ "$attempt" -eq 3 ]; then exit 1; fi; \
        sleep 10; \
    done && \
    npm cache clean --force && \
    npm --prefix web run build && \
    npm --prefix ui-tui run build

RUN mkdir -p \
        /home/agent/.hermes/lazy-packages \
        /opt/hermes/bin && \
    printf 'docker\n' > /opt/hermes/.install_method && \
    printf '%s\n' "$HERMES_SOURCE_SHA" > /opt/hermes/.hermes_build_sha && \
    chown -R agent:agent /home/agent/.hermes

COPY shell/desktop-welcome /usr/local/bin/desktop-welcome
COPY shell/hermes /usr/local/bin/hermes
RUN chmod 0755 /usr/local/bin/desktop-welcome /usr/local/bin/hermes

ENV DESKTOP_TITLE="Hermes Desktop" \
    DESKTOP_PERSISTENT_PATHS="/home/agent/.hermes" \
    HERMES_DISABLE_LAZY_INSTALLS=1 \
    HERMES_HOME=/home/agent/.hermes \
    HERMES_LAZY_INSTALL_TARGET=/home/agent/.hermes/lazy-packages \
    HERMES_TUI_DIR=/opt/hermes/ui-tui \
    HERMES_WEB_DIST=/opt/hermes/hermes_cli/web_dist \
    HERMES_WRITE_SAFE_ROOT=/home/agent/.hermes \
    PATH="/usr/local/bin:/opt/hermes/.venv/bin:/home/agent/.local/bin:${PATH}"

LABEL org.opencontainers.image.title="Hermes Desktop" \
    org.opencontainers.image.description="Hermes Agent in a Launcher-managed browser desktop" \
    org.opencontainers.image.source="https://github.com/pdparchitect/launcher" \
    org.opencontainers.image.version="${HERMES_VERSION}" \
    org.opencontainers.image.revision="${HERMES_SOURCE_SHA}"

VOLUME ["/workspace", "/home/agent/.hermes"]

WORKDIR /workspace
