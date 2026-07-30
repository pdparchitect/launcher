# syntax=docker/dockerfile:1.7
#
# Codex Pets on the Launcher desktop base.
#
# The desktop is a pen. Each pet is a directory under /workspace/pets whose
# AGENTS.md says who it is; a 16x16 sprite walks around the screen on its
# behalf, and behind the sprite `codex exec` runs its turns. `petd` supervises
# both halves. See README.md next to this file for how the pieces fit.
#
# Everything visual comes from the base. This image adds Codex and the pen,
# then installs its branding through overlay/, which is copied over the base's
# defaults.

ARG DESKTOP_IMAGE=pdparchitect/launcher-image-base-desktop:local
FROM ${DESKTOP_IMAGE}

USER root

# The sprite renderer draws with cairo into a shaped X11 window: GTK supplies
# the window and the shape combine, cairo the pixels. This is what lets a pet
# be a silhouette rather than a rectangle without a compositor running - and no
# compositor runs here, because every composited frame is one KasmVNC has to
# encode and ship to a browser.
RUN apt-get update && apt-get install -y --no-install-recommends \
    gir1.2-gtk-3.0 \
    python3-cairo \
    python3-gi \
    python3-gi-cairo \
    && rm -rf /var/lib/apt/lists/*

# The harness behind the pets. Node comes from the runtime layer in the base
# chain; the platform binary arrives as an optional dependency of this package.
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

# Product branding and the pen itself. Everything under overlay/ is installed
# over the base's defaults, so a file here wins without the base knowing this
# product exists.
COPY overlay /
RUN chmod 0755 \
        /etc/desktop/session.d/05-pet-wallpaper \
        /etc/desktop/session.d/10-pets \
        /usr/local/bin/codex-setup \
        /usr/local/bin/desktop-harness \
        /usr/local/bin/desktop-panel-status \
        /usr/local/bin/desktop-selftest \
        /usr/local/bin/desktop-welcome \
        /usr/local/bin/pen-greeting \
        /usr/local/bin/pet-habitat \
        /usr/local/bin/pet-run \
        /usr/local/bin/pet-sprite \
        /usr/local/bin/pet-wallpaper \
        /usr/local/bin/petctl \
        /usr/local/bin/petd

# Seed the pen with one resident, built by the same command a person uses.
# Generating the starter pet rather than shipping a hand-written copy means the
# seed cannot drift away from the templates, and it fails the build if petctl
# cannot even scaffold a pet.
#
# The seed lives outside /workspace on purpose: the base copies this directory
# into the workspace volume without overwriting anything, so the starter pet
# reaches a brand-new volume and never overwrites a pen somebody has been
# living in.
RUN PETS_ROOT=/usr/local/share/launcher-desktop/workspace/pets \
    petctl new pip \
        --species blob \
        --colour mint \
        --heartbeat 30m \
        --persona "You are the first resident of this pen. You are curious about the machine you live on and you keep notes about it."

# Codex keeps credentials and configuration here, and this path is a volume, so
# a rebuilt image resumes signed in.
RUN mkdir -p /home/agent/.codex && \
    printf '%s\n' \
        '# Codex configuration for this desktop.' \
        '#' \
        '# Empty on purpose: pets pass the settings that differ between them' \
        '# - model, sandbox - on the command line from their own pet.toml.' \
        '# Anything set here applies to every pet and to the Codex you open' \
        '# yourself with Control-Shift-G.' \
        > /home/agent/.codex/config.toml && \
    chown -R agent:agent /home/agent

# Rebrand the KasmVNC client. The base already patched it, so this replaces the
# brand rather than injecting the asset links a second time.
RUN kasm-patch "Codex Pets"

ENV DESKTOP_TITLE="Codex Pets" \
    DESKTOP_PERSISTENT_PATHS="/home/agent/.codex" \
    CODEX_HOME=/home/agent/.codex \
    PETS_ROOT=/workspace/pets

LABEL org.opencontainers.image.title="Codex Pets Desktop" \
    org.opencontainers.image.description="A pen of Codex-backed desktop pets in a Launcher-managed browser desktop" \
    org.opencontainers.image.source="https://github.com/pdparchitect/launcher" \
    dev.pdparchitect.launcher.upstream.source="${CODEX_SOURCE_URL}" \
    dev.pdparchitect.launcher.upstream.version="${CODEX_VERSION}"

VOLUME ["/workspace", "/home/agent/.codex"]

WORKDIR /workspace
