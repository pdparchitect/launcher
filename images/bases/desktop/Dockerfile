# syntax=docker/dockerfile:1.7
#
# The Openbox/KasmVNC desktop substrate, ported from Pantalk Ghost.
#
# This layer is product-neutral. A product image customises it by installing
# files over the defaults through its own overlay directory - see the overlay
# section in README.md - rather than by editing anything here.

ARG GO_IMAGE=golang:1.25.12-bookworm
ARG NODE_IMAGE=pdparchitect/launcher-image-runtime-node:local
FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS desktop-bridge-build

ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY bridge/go.mod bridge/go.sum ./
RUN go mod download
COPY bridge/ ./
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/desktop-bridge .

FROM ${NODE_IMAGE}

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

ARG TARGETARCH

# KasmVNC supplies its own X server (Xvnc), so the `xorg` metapackage is not
# installed: it would add xserver-xorg-core, input/video drivers,
# keyboard-configuration, and udev/systemd for hardware this container never
# has. `x11-xserver-utils` is skipped for the same reason - its only consumer
# would be the xrdb call in KasmVNC's generated xstartup, and xstartup is
# replaced with `exec openbox-session`. Together they cost ~90 MiB.
#
# xauth, xkb-data, and x11-xkb-utils are listed explicitly even though
# kasmvncserver depends on them, so an autoremove can never take them out.
# xfonts-base supplies the core font path Xvnc is started with.
RUN apt-get update && apt-get install -y --no-install-recommends \
    xdg-utils ssl-cert \
    xauth xkb-data x11-xkb-utils xfonts-base \
    xterm dbus-x11 libnotify-bin x11-utils \
    scrot \
    openbox obconf tint2 kitty ranger feh picom htop xdotool wmctrl \
    fonts-noto fonts-noto-color-emoji \
    libnss3 libatk1.0-0t64 libatk-bridge2.0-0t64 libcups2t64 libdrm2 \
    libxkbcommon0 libxcomposite1 libxdamage1 libxrandr2 libgbm1 \
    libpango-1.0-0 libasound2t64 libxshmfence1 \
    && rm -rf /var/lib/apt/lists/*

# Cortile provides optional dynamic tiling on top of Openbox.
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

# KasmVNC exposes the desktop in a browser.
ARG KASMVNC_VERSION=1.4.0
RUN arch="${TARGETARCH:-$(dpkg --print-architecture)}"; \
    curl -fsSL \
        "https://github.com/kasmtech/KasmVNC/releases/download/v${KASMVNC_VERSION}/kasmvncserver_noble_${KASMVNC_VERSION}_${arch}.deb" \
        -o /tmp/kasmvnc.deb && \
    apt-get update && \
    apt-get install -y --no-install-recommends /tmp/kasmvnc.deb && \
    rm -f /tmp/kasmvnc.deb && \
    rm -rf /var/lib/apt/lists/*

# Google does not publish Chrome for Linux ARM64. Keep Chrome on AMD64 and use
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
    fi; \
    rm -rf /var/lib/apt/lists/*

# The unprivileged desktop account. Named `agent` and homed at /home/agent so
# no product name leaks into the substrate.
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
        /home/agent/.config/cortile \
        /home/agent/.local/share/applications \
        /home/agent/.vnc \
        /etc/desktop/startup.d \
        /etc/desktop/session.d \
        /var/log/launcher-desktop && \
    echo "agent ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/agent && \
    chmod 0440 /etc/sudoers.d/agent && \
    touch /home/agent/.sudo_as_admin_successful /home/agent/.hushlogin

# KasmVNC client customisation: hide the upstream control bar, connect dialog,
# status overlays, spinner, and version badge, and set the brand.
COPY kasm/custom.css /usr/share/kasmvnc/www/assets/custom.css
COPY kasm/favicon.svg /usr/share/kasmvnc/www/assets/favicon.svg
COPY kasm/patch.sh /usr/local/bin/kasm-patch
ARG DESKTOP_BRAND=Desktop
RUN chmod 0755 /usr/local/bin/kasm-patch && kasm-patch "$DESKTOP_BRAND"

# Browser popup menus come from Linux's native colour pipeline rather than from
# extension-theme colours. A GTK system theme therefore styles the menus,
# dialogs, toolbar, tabs, and omnibox as one coherent near-black surface.
COPY gtk/Desktop /usr/share/themes/Desktop
COPY openbox/theme /usr/share/themes/Desktop/openbox-3
COPY gtk/generate-resource-overlay.py /tmp/generate-gtk-resource-overlay.py
# Chrome and Chromium ask GTK for embedded symbolic window-control resources.
# Generate real symbolic PNGs from the Openbox XBM masks so the browser's own
# window controls match the ones Openbox draws.
RUN python3 /tmp/generate-gtk-resource-overlay.py \
        /usr/share/themes/Desktop/openbox-3 \
        /usr/share/launcher-desktop/gtk-overlay && \
    rm -f /tmp/generate-gtk-resource-overlay.py

COPY openbox/rc.xml /etc/xdg/openbox/rc.xml
COPY openbox/menu.xml /etc/xdg/openbox/menu.xml
COPY openbox/autostart /etc/xdg/openbox/autostart
COPY tint2/tint2rc /etc/xdg/tint2/tint2rc
COPY kitty/theme.conf /etc/xdg/kitty/theme.conf
COPY cortile/cortilectl /usr/local/bin/cortilectl
COPY cortile/cortile-config.toml /home/agent/.config/cortile/config.toml
COPY shell/chromium /usr/local/bin/chromium
COPY --from=desktop-bridge-build /out/desktop-bridge /usr/local/bin/desktop-bridge
COPY shell/desktop-welcome /usr/local/bin/desktop-welcome
COPY shell/desktop-harness /usr/local/bin/desktop-harness
COPY shell/desktop-panel-status /usr/local/bin/desktop-panel-status
COPY shell/bashrc /etc/bash.bashrc.d/desktop-prompt.sh
COPY browser /opt/browser
COPY wallpaper/launcher-desktop.svg /usr/share/backgrounds/launcher-desktop.svg
COPY init.sh /init

RUN chmod 0755 \
        /etc/xdg/openbox/autostart \
        /init \
        /usr/local/bin/chromium \
        /usr/local/bin/cortilectl \
        /usr/local/bin/desktop-bridge \
        /usr/local/bin/desktop-harness \
        /usr/local/bin/desktop-panel-status \
        /usr/local/bin/desktop-welcome && \
    echo '[ -d /etc/bash.bashrc.d ] && for f in /etc/bash.bashrc.d/*.sh; do . "$f"; done' \
        >> /etc/bash.bashrc && \
    mkdir -p /usr/share/xsessions && \
    printf '[Desktop Entry]\nName=Openbox\nExec=openbox-session\nType=Application\n' \
        > /usr/share/xsessions/openbox.desktop

# Browser preferences. `system_theme: 1` selects GTK on Linux; unlike an
# extension theme it also reaches native menus and other popup surfaces.
RUN for config_dir in google-chrome chromium; do \
        mkdir -p "/home/agent/.config/$config_dir/Default"; \
        printf '{\n  "browser": {\n    "has_seen_welcome_page": true,\n    "check_default_browser": false\n  },\n  "bookmark_bar": { "show_on_all_tabs": false },\n  "distribution": {\n    "skip_first_run_ui": true,\n    "show_welcome_page": false,\n    "import_bookmarks": false,\n    "make_chrome_default_for_user": false,\n    "suppress_first_run_default_browser_prompt": true\n  },\n  "extensions": {\n    "theme": {\n      "system_theme": 1\n    }\n  }\n}' \
            > "/home/agent/.config/$config_dir/Default/Preferences"; \
        touch "/home/agent/.config/$config_dir/First Run"; \
    done

# Suppress the default-browser prompt via managed policy. Do not set
# BrowserThemeColor here: that policy overrides the GTK system theme.
RUN mkdir -p \
        /etc/opt/chrome/policies/managed \
        /etc/chromium/policies/managed && \
    printf '{\n  "DefaultBrowserSettingEnabled": false,\n  "BrowserSignin": 0,\n  "HomepageLocation": "file:///opt/browser/index.html",\n  "HomepageIsNewTabPage": false,\n  "ShowHomeButton": true,\n  "RestoreOnStartup": 4,\n  "RestoreOnStartupURLs": [\n    "file:///opt/browser/index.html"\n  ]\n}' \
        > /etc/opt/chrome/policies/managed/desktop-policy.json && \
    cp /etc/opt/chrome/policies/managed/desktop-policy.json \
        /etc/chromium/policies/managed/desktop-policy.json

# Register the architecture's browser as the system default.
RUN if [ -x /opt/google/chrome/google-chrome ]; then \
        browser=/opt/google/chrome/google-chrome; \
    else \
        browser=/usr/bin/chromium; \
    fi && \
    update-alternatives --install /usr/bin/x-www-browser x-www-browser "$browser" 200 2>/dev/null || true && \
    update-alternatives --set x-www-browser "$browser" 2>/dev/null || true

RUN chown -R agent:agent /home/agent /workspace /var/log/launcher-desktop

ENV HOME=/home/agent \
    BROWSER=chromium \
    DISPLAY=:1 \
    GTK_THEME=Desktop \
    G_RESOURCE_OVERLAYS=/org/gtk/libgtk=/usr/share/launcher-desktop/gtk-overlay \
    XAUTHORITY=/run/launcher-desktop/Xauthority \
    DESKTOP_TITLE="Agent Desktop"

LABEL org.opencontainers.image.title="Launcher Desktop Base" \
    org.opencontainers.image.description="Openbox and KasmVNC desktop substrate for Launcher products" \
    org.opencontainers.image.source="https://github.com/pdparchitect/launcher"

EXPOSE 6901 6902
WORKDIR /workspace

HEALTHCHECK --interval=10s --timeout=3s --start-period=30s --retries=5 \
    CMD curl -fsS http://127.0.0.1:6901/ >/dev/null && \
        curl -fsS http://127.0.0.1:6902/healthz >/dev/null || exit 1

ENTRYPOINT ["/init"]
