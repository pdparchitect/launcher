#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="products/opencode/desktop"
overlay="$project/overlay"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

grep -Eq '^COPY +overlay +/$' "$dockerfile"
grep -Eq '^ARG OPENCODE_VERSION=[0-9]+\.[0-9]+\.[0-9]+$' "$dockerfile"
grep -Fq 'npm install -g "opencode-ai@${OPENCODE_VERSION}"' "$dockerfile"
grep -Fq '/usr/bin/opencode --version' "$dockerfile"
grep -Fq 'kasm-patch "OpenCode Desktop"' "$dockerfile"
grep -Fq 'DESKTOP_PERSISTENT_PATHS="/home/agent/.opencode"' "$dockerfile"
grep -Fq 'VOLUME ["/workspace", "/home/agent/.opencode"]' "$dockerfile"

grep -Fq 'exec /usr/bin/opencode' "$overlay/usr/local/bin/opencode"
grep -Fq '/usr/share/opencode/themes/launcher.json' \
    "$overlay/usr/local/bin/opencode"
grep -Fq 'if [ ! -e "$XDG_CONFIG_HOME/opencode/tui.json" ]' \
    "$overlay/usr/local/bin/opencode"
grep -Fq 'exec opencode' "$overlay/usr/local/bin/desktop-welcome"
grep -Fq 'exec opencode' "$overlay/usr/local/bin/desktop-harness"
grep -Fq 'opencode --version' "$overlay/usr/local/bin/desktop-selftest"
grep -Fq 'the Launcher theme is selected' \
    "$overlay/usr/local/bin/desktop-selftest"
grep -Fq 'the Launcher theme is installed' \
    "$overlay/usr/local/bin/desktop-selftest"
grep -Fq 'background            #1B1818' \
    "$overlay/etc/xdg/kitty/theme.conf"
grep -Fq 'cursor_text_color     #1B1818' \
    "$overlay/etc/xdg/kitty/theme.conf"
grep -Fiq '#FDFFCC' "$overlay/etc/xdg/kitty/theme.conf"
grep -Fiq '#131010' "$overlay/usr/share/backgrounds/desktop-wallpaper.svg"
grep -Fiq '#131010' "$project/launcher/icon.svg"
grep -Fiq '#F2EDED' "$project/launcher/icon.svg"
grep -Fiq '#131010' "$overlay/opt/browser/index.html"
grep -Fiq '#F2EDED' "$overlay/opt/browser/index.html"
jq -e '.theme == "launcher"' \
    "$overlay/usr/share/opencode/tui.json" >/dev/null
jq -e '
    .defs.siteBackground == "#1B1818" and
    .defs.siteChrome == "#131010" and
    .defs.siteSurface == "#292424" and
    .defs.siteText == "#F2EDED" and
    .defs.sitePrimary == "#FDFFCC" and
    .theme.background == "siteBackground" and
    .theme.backgroundPanel == "siteSurface" and
    .theme.backgroundElement == "siteSurface"
' "$overlay/usr/share/opencode/themes/launcher.json" >/dev/null
grep -Fq 'window.client.padding.width: 0' "$dockerfile"
grep -Fq 'window.client.padding.height: 0' "$dockerfile"
grep -Fq "s/#000000/#131010/g" "$dockerfile"
grep -Fq "s/#070707/#131010/g" "$dockerfile"
grep -Fq "s/#1a1a1a/#131010/g" "$dockerfile"
grep -Fq "s/#020303/#131010/g" "$dockerfile"
grep -Fq '/usr/share/themes/Desktop/openbox-3/themerc' "$dockerfile"
grep -Fq '/etc/xdg/tint2/tint2rc' "$dockerfile"
grep -Fq '/usr/share/themes/Desktop/gtk-3.0/gtk.css' "$dockerfile"

if grep -Eiq 'opencode-desktop|AppImage|\.deb|\.rpm' "$dockerfile"; then
    echo "OpenCode image installs a desktop application package." >&2
    exit 1
fi

echo "OpenCode image checks passed."
