# syntax=docker/dockerfile:1.7
#
# Codex CLI on the Launcher desktop base.
#
# Codex is a terminal coding harness, so the product surface is a persistent
# Kitty window in /workspace. Petbox packages the same CLI, but as the engine
# behind a pen of desktop pets; this product is Codex itself.

ARG DESKTOP_IMAGE=pdparchitect/launcher-image-base-desktop:local
FROM ${DESKTOP_IMAGE}

USER root

# The platform binary arrives as an optional dependency of this package, so the
# install resolves per architecture without a separate download step.
#
# npm's cache follows HOME, which the desktop base points at the session user's
# home. Without an explicit cache directory this root-run install leaves
# /home/agent/.npm owned by root, and then kitty cannot start and the session
# opens with no terminal at all.
ARG CODEX_SOURCE_URL=https://github.com/openai/codex
ARG CODEX_VERSION=0.146.0
RUN npm_config_cache=/tmp/npm-cache \
        npm install -g "@openai/codex@${CODEX_VERSION}" && \
    rm -rf /tmp/npm-cache && \
    test "$(codex --version)" = "codex-cli ${CODEX_VERSION}"

# Credentials, configuration, history, and rollout sessions all live below
# ~/.codex. CODEX_HOME is set explicitly rather than left to resolve from HOME,
# because an Apple fixed-mount session runs as root and would otherwise put a
# signed-in Codex somewhere that is not the declared volume.
RUN mkdir -p /home/agent/.codex && \
    chown -R agent:agent /home/agent

# Product entry points, terminal palette, wallpaper, browser page, and branding.
COPY overlay /
RUN chmod 0755 \
        /usr/local/bin/desktop-harness \
        /usr/local/bin/desktop-panel-status \
        /usr/local/bin/desktop-selftest \
        /usr/local/bin/desktop-welcome

# Turn the inherited chrome monochrome: window decorations, the panel, the root
# menu, and Chromium's GTK surfaces go to true black, and the states the base
# paints in neutral greys are painted in one band of the spectrum instead. The
# script maps the base's colours role by role rather than replacing those
# files, so later base changes still reach this product.
COPY theme/apply-theme.sh /tmp/apply-theme.sh
RUN bash /tmp/apply-theme.sh && rm /tmp/apply-theme.sh

RUN kasm-patch "Codex"

ENV DESKTOP_TITLE="Codex" \
    DESKTOP_PERSISTENT_PATHS="/home/agent/.codex" \
    CODEX_HOME="/home/agent/.codex"

LABEL org.opencontainers.image.title="Codex" \
    org.opencontainers.image.description="Codex CLI in a Launcher-managed browser desktop" \
    org.opencontainers.image.source="https://github.com/pdparchitect/launcher" \
    dev.pdparchitect.launcher.upstream.source="${CODEX_SOURCE_URL}" \
    dev.pdparchitect.launcher.upstream.version="${CODEX_VERSION}"

VOLUME ["/workspace", "/home/agent/.codex"]

WORKDIR /workspace
