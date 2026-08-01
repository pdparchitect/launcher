# syntax=docker/dockerfile:1.7
#
# Claude Code on the Launcher desktop base.
#
# Claude Code is a terminal coding harness, so the product surface is a
# persistent Kitty window in /workspace. The product directory is named
# claude-code rather than claude, because "Claude Desktop" is a different
# Anthropic application and this image is not it.

ARG DESKTOP_IMAGE=pdparchitect/launcher-image-base-desktop:local
FROM ${DESKTOP_IMAGE}

USER root

# npm's cache follows HOME, which the desktop base points at the session user's
# home. Without an explicit cache directory this root-run install leaves
# /home/agent/.npm owned by root, and then kitty cannot start and the session
# opens with no terminal at all.
#
# `claude --version` prints "<version> (Claude Code)", so the pin is compared
# against the first field rather than the whole line.
ARG CLAUDE_CODE_SOURCE_URL=https://github.com/anthropics/claude-code
ARG CLAUDE_CODE_VERSION=2.1.220
RUN npm_config_cache=/tmp/npm-cache \
        npm install -g "@anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}" && \
    rm -rf /tmp/npm-cache && \
    test "$(claude --version | awk '{print $1}')" = "${CLAUDE_CODE_VERSION}"

# Claude Code splits its state in two: a ~/.claude directory for credentials,
# projects, history, and plugins, and a ~/.claude.json file beside it for
# onboarding state and MCP server configuration. Mounting only the directory
# would leave the file on the container filesystem, so every replacement would
# re-onboard. CLAUDE_CONFIG_DIR moves both below one declared volume.
RUN mkdir -p /home/agent/.claude && \
    chown -R agent:agent /home/agent

# Product entry points, terminal palette, wallpaper, browser page, and branding.
COPY overlay /
RUN chmod 0755 \
        /etc/desktop/startup.d/05-claude-runtime \
        /usr/local/bin/desktop-harness \
        /usr/local/bin/desktop-panel-status \
        /usr/local/bin/desktop-selftest \
        /usr/local/bin/desktop-welcome

# Turn the inherited dark chrome into Claude's paper palette: window
# decorations, the panel, the root menu, and Chromium's GTK surfaces. The
# script maps the base's colours role by role rather than replacing those
# files, so later base changes still reach this product - see the comments in
# theme/apply-theme.sh.
COPY theme/apply-theme.sh /tmp/apply-theme.sh
RUN bash /tmp/apply-theme.sh && rm /tmp/apply-theme.sh

RUN kasm-patch "Claude Code"

# The image is the unit Launcher updates, so the CLI's own updater is disabled:
# a self-update would replace the pinned version recorded in the image labels
# with something the image never claimed to ship.
ENV DESKTOP_TITLE="Claude Code" \
    DESKTOP_PERSISTENT_PATHS="/home/agent/.claude" \
    CLAUDE_CONFIG_DIR="/home/agent/.claude" \
    DISABLE_AUTOUPDATER="1"

LABEL org.opencontainers.image.title="Claude Code" \
    org.opencontainers.image.description="Claude Code in a Launcher-managed browser desktop" \
    org.opencontainers.image.source="https://github.com/pdparchitect/launcher" \
    dev.pdparchitect.launcher.upstream.source="${CLAUDE_CODE_SOURCE_URL}" \
    dev.pdparchitect.launcher.upstream.version="${CLAUDE_CODE_VERSION}"

VOLUME ["/workspace", "/home/agent/.claude"]

WORKDIR /workspace
