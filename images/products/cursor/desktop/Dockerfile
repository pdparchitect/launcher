# syntax=docker/dockerfile:1.7
#
# Cursor Agent CLI on the Launcher desktop base.
#
# This product installs only Cursor's terminal agent package. It deliberately
# does not install the Cursor editor or any graphical Cursor application.

ARG DESKTOP_IMAGE=pdparchitect/launcher-image-base-desktop:local
FROM ${DESKTOP_IMAGE}

USER root

ARG TARGETARCH
ARG CURSOR_AGENT_SOURCE_URL=https://cursor.com/cli
ARG CURSOR_AGENT_VERSION=2026.07.23-e383d2b
ARG CURSOR_AGENT_SHA256_AMD64=702ad595213bee5df0268be9f80a19f29fcceaa2a42fc55e39f2b5199051f0c4
ARG CURSOR_AGENT_SHA256_ARM64=f40b99647cb24e0da885e97620a2048034f1fe8961910d573d827d77c4d26dcb

RUN case "${TARGETARCH}" in \
        amd64) cursor_arch=x64; cursor_sha="${CURSOR_AGENT_SHA256_AMD64}" ;; \
        arm64) cursor_arch=arm64; cursor_sha="${CURSOR_AGENT_SHA256_ARM64}" ;; \
        *) echo "unsupported Cursor Agent architecture: ${TARGETARCH}" >&2; exit 1 ;; \
    esac && \
    archive=/tmp/cursor-agent.tar.gz && \
    curl -fsSL --retry 5 \
        "https://downloads.cursor.com/lab/${CURSOR_AGENT_VERSION}/linux/${cursor_arch}/agent-cli-package.tar.gz" \
        -o "${archive}" && \
    printf '%s  %s\n' "${cursor_sha}" "${archive}" | sha256sum -c - && \
    mkdir -p /opt/cursor-agent && \
    tar --strip-components=1 -xzf "${archive}" -C /opt/cursor-agent && \
    rm -f "${archive}" && \
    chmod 0755 /opt/cursor-agent/cursor-agent && \
    ln -s /opt/cursor-agent/cursor-agent /usr/local/bin/agent && \
    ln -s /opt/cursor-agent/cursor-agent /usr/local/bin/cursor-agent && \
    test "$(HOME=/tmp/cursor-agent-test cursor-agent --version)" = \
        "${CURSOR_AGENT_VERSION}"

RUN mkdir -p /home/agent/.cursor && \
    chown -R agent:agent /home/agent

COPY overlay /
RUN chmod 0755 \
        /usr/local/bin/desktop-harness \
        /usr/local/bin/desktop-selftest \
        /usr/local/bin/desktop-welcome

# Carry Cursor's warm-black surface through the inherited desktop chrome. The
# white focused-window outline and hover colours remain distinct, while both
# active and inactive titlebar fills, client-side GTK chrome, and the system
# panel use the same background as the terminal.
RUN sed -i \
        -e 's/#000000/#14120B/g' \
        -e 's/#070707/#14120B/g' \
        -e 's/^window.client.padding.width:.*/window.client.padding.width: 0/' \
        -e 's/^window.client.padding.height:.*/window.client.padding.height: 0/' \
        /usr/share/themes/Desktop/openbox-3/themerc && \
    sed -i \
        -e 's/#000000/#14120B/g' \
        -e 's/#1a1a1a/#14120B/g' \
        /etc/xdg/tint2/tint2rc && \
    sed -i \
        -e 's/#000000/#14120B/g' \
        -e 's/#020303/#14120B/g' \
        /usr/share/themes/Desktop/gtk-3.0/gtk.css

RUN kasm-patch "Cursor Agent"

ENV DESKTOP_TITLE="Cursor Agent" \
    DESKTOP_PERSISTENT_PATHS="/home/agent/.cursor"

LABEL org.opencontainers.image.title="Cursor Agent" \
    org.opencontainers.image.description="Cursor Agent CLI in a Launcher-managed browser desktop" \
    org.opencontainers.image.source="https://github.com/pdparchitect/launcher" \
    dev.pdparchitect.launcher.upstream.source="${CURSOR_AGENT_SOURCE_URL}" \
    dev.pdparchitect.launcher.upstream.version="${CURSOR_AGENT_VERSION}"

VOLUME ["/workspace", "/home/agent/.cursor"]

WORKDIR /workspace
