# syntax=docker/dockerfile:1.7
#
# Pi on the Launcher desktop base.
#
# Pi is a terminal coding harness, so its product surface is a persistent
# Kitty window in /workspace. The desktop base supplies Node, the terminal,
# browser access, and the shared preview interface.

ARG DESKTOP_IMAGE=pdparchitect/launcher-image-base-desktop:local
FROM ${DESKTOP_IMAGE}

USER root

# Pi uses fd for fast file discovery. Ubuntu names the packaged binary
# `fdfind`, so expose its conventional name and keep Pi from downloading a
# private copy into persistent state on first launch.
RUN apt-get update && apt-get install -y --no-install-recommends fd-find && \
    ln -s /usr/bin/fdfind /usr/local/bin/fd && \
    rm -rf /var/lib/apt/lists/*

# Pi's official npm installation does not require lifecycle scripts. Keeping
# them disabled matches upstream's hardened install path and avoids executing
# transitive package hooks while building the image.
ARG PI_SOURCE_URL=https://github.com/earendil-works/pi
ARG PI_VERSION=0.83.0
RUN npm_config_cache=/tmp/npm-cache \
        npm install -g --ignore-scripts \
        "@earendil-works/pi-coding-agent@${PI_VERSION}" && \
    rm -rf /tmp/npm-cache && \
    test "$(pi --version)" = "${PI_VERSION}"

# Credentials, settings, packages, extensions, and sessions all live below
# ~/.pi. Keeping the whole directory on one volume preserves every supported
# customization rather than selecting individual current subdirectories.
RUN mkdir -p /home/agent/.pi/agent && \
    chown -R agent:agent /home/agent

# Product entry points, terminal palette, wallpaper, browser page, and branding.
COPY overlay /
RUN chmod 0755 \
        /usr/local/bin/desktop-harness \
        /usr/local/bin/desktop-selftest \
        /usr/local/bin/desktop-welcome

# The shared Openbox theme adds a 6px black resize frame around every client.
# Remove that artificial gap without recolouring the inherited titlebar.
RUN sed -i \
        -e 's/^window.client.padding.width:.*/window.client.padding.width: 0/' \
        -e 's/^window.client.padding.height:.*/window.client.padding.height: 0/' \
        /usr/share/themes/Desktop/openbox-3/themerc

RUN kasm-patch "Pi Desktop"

ENV DESKTOP_TITLE="Pi" \
    DESKTOP_PERSISTENT_PATHS="/home/agent/.pi"

LABEL org.opencontainers.image.title="Pi Desktop" \
    org.opencontainers.image.description="Pi coding agent in a Launcher-managed browser desktop" \
    org.opencontainers.image.source="https://github.com/pdparchitect/launcher" \
    dev.pdparchitect.launcher.upstream.source="${PI_SOURCE_URL}" \
    dev.pdparchitect.launcher.upstream.version="${PI_VERSION}"

VOLUME ["/workspace", "/home/agent/.pi"]

WORKDIR /workspace
