# syntax=docker/dockerfile:1.7
#
# OpenClaw on the Launcher desktop base.
#
# OpenClaw's own interface is the Gateway Control UI, a web application the
# gateway serves from its own port, so this product is shaped around a browser
# window rather than around a terminal: a supervised gateway on loopback, and a
# Chromium app window opened onto it when the session starts.
#
# Everything visual comes from the base. This image adds OpenClaw, then
# installs its branding through overlay/, which is copied over the base's
# defaults.

ARG DESKTOP_IMAGE=pdparchitect/launcher-image-base-desktop:local
FROM ${DESKTOP_IMAGE}

USER root

# OpenClaw is a Node application and Node comes from the runtime layer in the
# base chain, so the whole product is one npm install.
#
# npm's cache follows HOME, which the desktop base points at the session user's
# home. Without an explicit cache directory this root-run install leaves
# /home/agent/.npm owned by root, and then kitty cannot start and the session
# opens with no terminal at all.
ARG OPENCLAW_SOURCE_URL=https://github.com/openclaw/openclaw
ARG OPENCLAW_VERSION=2026.7.1-2
ENV npm_config_cache=/tmp/npm-cache
RUN npm install -g "openclaw@${OPENCLAW_VERSION}" && \
    rm -rf /tmp/npm-cache && \
    test -f /usr/lib/node_modules/openclaw/openclaw.mjs && \
    node /usr/lib/node_modules/openclaw/openclaw.mjs --version | \
        grep -Fq "${OPENCLAW_VERSION}"

# The overlay ships /usr/local/bin/openclaw, which shadows npm's launcher on
# PATH rather than replacing it. If npm's global prefix ever moved to
# /usr/local the overlay would overwrite the real launcher instead, and the
# wrapper would be the only OpenClaw left in the image.
RUN test -e /usr/bin/openclaw && test ! -e /usr/local/bin/openclaw

# OpenClaw keeps its config, credentials, sessions and agent workspace here,
# and this path is a volume, so a rebuilt image resumes onboarded and signed
# in. Creating it now means the entrypoint's ownership pass has something to
# normalise on a brand-new volume.
RUN mkdir -p /home/agent/.openclaw && \
    rm -rf /home/agent/.cache /home/agent/.npm && \
    chown -R agent:agent /home/agent

# Product branding and the programs that drive the gateway. Everything under
# overlay/ is installed over the base's defaults, so a file here wins without
# the base knowing this product exists.
COPY overlay /
RUN chmod 0755 \
        /etc/desktop/session.d/20-openclaw-control-ui \
        /etc/desktop/startup.d/10-openclaw-gateway \
        /usr/local/bin/desktop-harness \
        /usr/local/bin/desktop-panel-status \
        /usr/local/bin/desktop-selftest \
        /usr/local/bin/desktop-welcome \
        /usr/local/bin/openclaw \
        /usr/local/bin/openclaw-control-ui \
        /usr/local/bin/openclaw-gateway-supervise \
        /usr/local/bin/openclaw-greeting

# Rebrand the KasmVNC client. The base already patched it, so this replaces the
# brand rather than injecting the asset links a second time.
RUN kasm-patch "OpenClaw Desktop"

# The last two are this image's own variables, not OpenClaw's.
#
# OPENCLAW_CONTROL_PORT is what the panel status and the session's readiness
# probe test, because a TCP probe costs nothing and starting Node every few
# seconds does not. The Control UI window resolves the real address through
# `openclaw dashboard --json` instead, so this only has to be changed alongside
# `gateway.port`.
#
# OPENCLAW_GATEWAY_BIND is the bind mode the supervisor passes. OpenClaw
# detects a container and binds 0.0.0.0 by default; this image publishes the
# desktop and never the gateway, so loopback is the honest default here.
ENV DESKTOP_TITLE="OpenClaw" \
    DESKTOP_PERSISTENT_PATHS="/home/agent/.openclaw" \
    OPENCLAW_CONTROL_PORT=18789 \
    OPENCLAW_GATEWAY_BIND=loopback

LABEL org.opencontainers.image.title="OpenClaw Desktop" \
    org.opencontainers.image.description="OpenClaw and its Gateway Control UI in a Launcher-managed browser desktop" \
    org.opencontainers.image.source="https://github.com/pdparchitect/launcher" \
    dev.pdparchitect.launcher.upstream.source="${OPENCLAW_SOURCE_URL}" \
    dev.pdparchitect.launcher.upstream.version="${OPENCLAW_VERSION}"

VOLUME ["/workspace", "/home/agent/.openclaw"]

WORKDIR /workspace
