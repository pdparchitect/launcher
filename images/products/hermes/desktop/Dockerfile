# syntax=docker/dockerfile:1.7
#
# Hermes Agent on the Launcher desktop base.
#
# Everything visual comes from the base. This image adds Hermes, then installs
# its branding through overlay/, which is copied over the base's defaults.

ARG DESKTOP_IMAGE=pdparchitect/launcher-image-base-desktop:local

FROM ghcr.io/astral-sh/uv:0.11.6-python3.13-trixie@sha256:b3c543b6c4f23a5f2df22866bd7857e5d304b67a564f4feab6ac22044dde719b AS uv-source
FROM ${DESKTOP_IMAGE}

USER root

# Build dependencies for Hermes' native extensions. python3-dev and
# python3-venv are product build dependencies, so they live here rather than in
# the shared core.
RUN apt-get update && apt-get install -y --no-install-recommends \
    cmake \
    ffmpeg \
    g++ \
    gcc \
    libffi-dev \
    libolm-dev \
    make \
    python3-dev \
    python3-venv \
    && rm -rf /var/lib/apt/lists/*

# Node and npm come from the runtime layer in the base chain.
COPY --from=uv-source /usr/local/bin/uv /usr/local/bin/uv
COPY --from=uv-source /usr/local/bin/uvx /usr/local/bin/uvx

ARG HERMES_SOURCE_URL=https://github.com/NousResearch/hermes-agent
ARG HERMES_VERSION=0.19.0
ARG HERMES_SOURCE_TAG=v2026.7.20
ARG HERMES_SOURCE_SHA=3ef6bbd201263d354fd83ec55b3c306ded2eb72a

RUN git clone --branch "${HERMES_SOURCE_TAG}" --depth 1 \
        "${HERMES_SOURCE_URL}.git" /opt/hermes && \
    test "$(git -C /opt/hermes rev-parse HEAD)" = "$HERMES_SOURCE_SHA"

WORKDIR /opt/hermes

ENV UV_PYTHON=/usr/bin/python3 \
    PLAYWRIGHT_BROWSERS_PATH=/opt/hermes/.playwright \
    npm_config_install_links=false

# Match the dependency profiles baked into the upstream Hermes container while
# retaining this image's Launcher desktop as the final parent. Install only
# dependencies here; the project is linked after its generated frontends exist.
RUN uv sync --frozen \
        --no-install-project \
        --extra all \
        --extra messaging \
        --extra anthropic \
        --extra bedrock \
        --extra azure-identity \
        --extra hindsight \
        --extra matrix

RUN npm install --prefer-offline --no-audit --fetch-retries=5 && \
    for attempt in 1 2 3; do \
        npx playwright install chromium --only-shell && break; \
        if [ "$attempt" -eq 3 ]; then exit 1; fi; \
        sleep 10; \
    done && \
    npm cache clean --force && \
    npm --prefix web run build && \
    npm --prefix ui-tui run build

# Create Hermes' editable package link explicitly, as upstream's container
# does. Depending on uv sync to do this implicitly produced images with the
# console script present but no importable hermes_cli package on some builders.
RUN uv pip install \
        --python /opt/hermes/.venv/bin/python \
        --no-cache-dir \
        --no-deps \
        -e "." && \
    /opt/hermes/.venv/bin/python -c 'from hermes_cli.main import main'

# The desktop base exports HOME=/home/agent, so the root-run build steps above
# deposit their caches in the session user's home and leave them owned by root.
# Kitty then cannot create /home/agent/.cache/kitty, the welcome terminal dies,
# and the session opens with no terminal at all. None of these caches are used
# at run time, so drop them and leave the home directory owned by its user.
RUN rm -rf /home/agent/.cache /home/agent/.npm && \
    mkdir -p \
        /home/agent/.hermes/lazy-packages \
        /opt/hermes/bin && \
    printf 'docker\n' > /opt/hermes/.install_method && \
    printf '%s\n' "$HERMES_SOURCE_SHA" > /opt/hermes/.hermes_build_sha && \
    chown -R agent:agent /home/agent

# Product branding. Everything under overlay/ is installed over the base's
# defaults, so a file here wins without the base knowing this product exists.
COPY overlay /
RUN chmod 0755 \
        /usr/local/bin/desktop-selftest \
        /usr/local/bin/desktop-welcome \
        /usr/local/bin/hermes

# Rebrand the KasmVNC client. The base already patched it, so this replaces the
# brand rather than injecting the asset links a second time.
RUN kasm-patch "Hermes Desktop"

ENV DESKTOP_TITLE="Hermes" \
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
    dev.pdparchitect.launcher.upstream.source="${HERMES_SOURCE_URL}" \
    dev.pdparchitect.launcher.upstream.version="${HERMES_VERSION}" \
    dev.pdparchitect.launcher.upstream.ref="${HERMES_SOURCE_TAG}" \
    dev.pdparchitect.launcher.upstream.revision="${HERMES_SOURCE_SHA}"

VOLUME ["/workspace", "/home/agent/.hermes"]

WORKDIR /workspace
