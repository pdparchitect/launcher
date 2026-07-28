# syntax=docker/dockerfile:1.7

ARG CORE_IMAGE=pdparchitect/launcher-core-ubuntu:local
FROM ${CORE_IMAGE}

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

ARG TARGETARCH

RUN apt-get update && apt-get install -y --no-install-recommends \
    dbus-x11 \
    feh \
    fonts-noto \
    fonts-noto-color-emoji \
    htop \
    kitty \
    libasound2t64 \
    libatk-bridge2.0-0t64 \
    libatk1.0-0t64 \
    libcups2t64 \
    libdrm2 \
    libgbm1 \
    libnss3 \
    libpango-1.0-0 \
    libxcomposite1 \
    libxdamage1 \
    libxkbcommon0 \
    libxrandr2 \
    libxshmfence1 \
    openbox \
    picom \
    ranger \
    scrot \
    ssl-cert \
    tint2 \
    wmctrl \
    x11-utils \
    x11-xkb-utils \
    xauth \
    xdg-utils \
    xdotool \
    xfonts-base \
    xkb-data \
    xterm \
    && rm -rf /var/lib/apt/lists/*

ARG CORTILE_VERSION=2.5.2
RUN set -eux; \
    arch="${TARGETARCH:-$(dpkg --print-architecture)}"; \
    case "$arch" in \
        amd64) cortile_arch=amd64 ;; \
        arm64) cortile_arch=arm64 ;; \
        *) echo "Unsupported Cortile architecture: $arch" >&2; exit 1 ;; \
    esac; \
    temporary_dir="$(mktemp -d)"; \
    curl -fsSL \
        "https://github.com/leukipp/cortile/releases/download/v${CORTILE_VERSION}/cortile_${CORTILE_VERSION}_linux_${cortile_arch}.tar.gz" \
        | tar -xz -C "$temporary_dir"; \
    install -m 0755 "$temporary_dir/cortile" /usr/local/bin/cortile; \
    rm -rf "$temporary_dir"

ARG KASMVNC_VERSION=1.4.0
RUN arch="${TARGETARCH:-$(dpkg --print-architecture)}"; \
    curl -fsSL \
        "https://github.com/kasmtech/KasmVNC/releases/download/v${KASMVNC_VERSION}/kasmvncserver_noble_${KASMVNC_VERSION}_${arch}.deb" \
        -o /tmp/kasmvnc.deb && \
    apt-get update && \
    apt-get install -y --no-install-recommends /tmp/kasmvnc.deb && \
    rm -f /tmp/kasmvnc.deb && \
    rm -rf /var/lib/apt/lists/*

# Google does not publish Chrome for Linux ARM64. Use Chrome on AMD64 and
# Debian's signed Chromium package on ARM64 behind one launcher command.
RUN set -eux; \
    arch="${TARGETARCH:-$(dpkg --print-architecture)}"; \
    if [ "$arch" = "amd64" ]; then \
        curl -fsSL https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb \
            -o /tmp/browser.deb; \
        apt-get update; \
        apt-get install -y --no-install-recommends /tmp/browser.deb; \
        rm -f /tmp/browser.deb; \
    else \
        mkdir -p /etc/apt/keyrings; \
        curl -fsSL https://ftp-master.debian.org/keys/archive-key-12.asc \
            -o /etc/apt/keyrings/debian-archive-key-12.asc; \
        printf '%s\n' \
            'deb [arch=arm64 signed-by=/etc/apt/keyrings/debian-archive-key-12.asc] https://deb.debian.org/debian bookworm main' \
            > /etc/apt/sources.list.d/debian-bookworm.list; \
        printf '%s\n' \
            'Package: *' \
            'Pin: release n=bookworm' \
            'Pin-Priority: 100' \
            > /etc/apt/preferences.d/debian-bookworm; \
        apt-get update; \
        apt-get install -y --no-install-recommends chromium; \
        rm -f \
            /etc/apt/keyrings/debian-archive-key-12.asc \
            /etc/apt/preferences.d/debian-bookworm \
            /etc/apt/sources.list.d/debian-bookworm.list; \
        rm -rf /var/lib/apt/lists/*; \
    fi

RUN if id -u agent >/dev/null 2>&1; then \
        :; \
    elif id -u ubuntu >/dev/null 2>&1; then \
        usermod -l agent -d /home/agent -m ubuntu && \
        groupmod -n agent ubuntu; \
    else \
        groupadd --gid 1000 agent && \
        useradd --uid 1000 --gid agent --create-home \
            --home-dir /home/agent --shell /bin/bash agent; \
    fi && \
    mkdir -p \
        /home/agent/.config \
        /home/agent/.local/share/applications \
        /home/agent/.vnc \
        /var/log/launcher-desktop && \
    chown -R agent:agent /home/agent /workspace /var/log/launcher-desktop && \
    echo "agent ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/agent && \
    chmod 0440 /etc/sudoers.d/agent

COPY openbox/autostart /etc/xdg/openbox/autostart
COPY shell/chromium /usr/local/bin/chromium
COPY shell/desktop-welcome /usr/local/bin/desktop-welcome
COPY wallpaper/launcher-desktop.svg /usr/share/backgrounds/launcher-desktop.svg
COPY init.sh /init
RUN chmod 0755 \
    /etc/xdg/openbox/autostart \
    /init \
    /usr/local/bin/chromium \
    /usr/local/bin/desktop-welcome && \
    mkdir -p /usr/share/xsessions && \
    printf '[Desktop Entry]\nName=Openbox\nExec=openbox-session\nType=Application\n' \
        > /usr/share/xsessions/openbox.desktop

RUN for config_dir in google-chrome chromium; do \
        mkdir -p "/home/agent/.config/$config_dir/Default"; \
        printf '%s\n' \
            '{"browser":{"check_default_browser":false,"has_seen_welcome_page":true},"distribution":{"skip_first_run_ui":true,"suppress_first_run_default_browser_prompt":true}}' \
            > "/home/agent/.config/$config_dir/Default/Preferences"; \
        touch "/home/agent/.config/$config_dir/First Run"; \
    done && \
    chown -R agent:agent /home/agent/.config

ENV HOME=/home/agent \
    BROWSER=chromium \
    DESKTOP_TITLE="Agent Desktop" \
    DESKTOP_RESOLUTION=1920x1080

LABEL org.opencontainers.image.title="Launcher Desktop Base" \
    org.opencontainers.image.description="Neutral KasmVNC and Openbox desktop for Launcher products" \
    org.opencontainers.image.source="https://github.com/pdparchitect/launcher"

EXPOSE 6901
WORKDIR /workspace

HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --retries=5 \
    CMD curl -fsS http://127.0.0.1:6901/ >/dev/null || exit 1

ENTRYPOINT ["/init"]
