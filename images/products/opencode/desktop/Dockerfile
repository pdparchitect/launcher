# syntax=docker/dockerfile:1.7
#
# OpenCode on the Launcher desktop base.
#
# OpenCode is a terminal coding harness, so the product surface is a persistent
# Kitty window in /workspace. The separate OpenCode desktop package is not
# installed.

ARG DESKTOP_IMAGE=pdparchitect/launcher-image-base-desktop:local
FROM ${DESKTOP_IMAGE}

USER root

ARG OPENCODE_SOURCE_URL=https://github.com/anomalyco/opencode
ARG OPENCODE_VERSION=1.18.10
RUN npm_config_cache=/tmp/npm-cache \
        npm install -g "opencode-ai@${OPENCODE_VERSION}" && \
    rm -rf /tmp/npm-cache && \
    test "$(HOME=/tmp/opencode-test /usr/bin/opencode --version)" = \
        "${OPENCODE_VERSION}"

# OpenCode normally spreads its XDG data, config, cache, and state across four
# home directories. The product wrapper scopes those variables to one private
# persistent mount without changing them for the rest of the desktop session.
RUN mkdir -p /home/agent/.opencode && \
    chown -R agent:agent /home/agent

COPY overlay /
RUN chmod 0755 \
        /usr/local/bin/desktop-harness \
        /usr/local/bin/desktop-selftest \
        /usr/local/bin/desktop-welcome \
        /usr/local/bin/opencode

# Carry opencode.ai's primary dark background through the inherited desktop
# chrome. The focused-window outline and hover colours remain distinct, while
# active and inactive titlebar fills, GTK chrome, and the system panel use the
# site's #131010 background around the #1B1818 terminal canvas.
RUN sed -i \
        -e 's/#000000/#131010/g' \
        -e 's/#070707/#131010/g' \
        -e 's/^window.client.padding.width:.*/window.client.padding.width: 0/' \
        -e 's/^window.client.padding.height:.*/window.client.padding.height: 0/' \
        /usr/share/themes/Desktop/openbox-3/themerc && \
    sed -i \
        -e 's/#000000/#131010/g' \
        -e 's/#1a1a1a/#131010/g' \
        /etc/xdg/tint2/tint2rc && \
    sed -i \
        -e 's/#000000/#131010/g' \
        -e 's/#020303/#131010/g' \
        /usr/share/themes/Desktop/gtk-3.0/gtk.css

RUN kasm-patch "OpenCode Desktop"

ENV DESKTOP_TITLE="OpenCode" \
    DESKTOP_PERSISTENT_PATHS="/home/agent/.opencode"

LABEL org.opencontainers.image.title="OpenCode Desktop" \
    org.opencontainers.image.description="OpenCode terminal agent in a Launcher-managed browser desktop" \
    org.opencontainers.image.source="https://github.com/pdparchitect/launcher" \
    dev.pdparchitect.launcher.upstream.source="${OPENCODE_SOURCE_URL}" \
    dev.pdparchitect.launcher.upstream.version="${OPENCODE_VERSION}"

VOLUME ["/workspace", "/home/agent/.opencode"]

WORKDIR /workspace
