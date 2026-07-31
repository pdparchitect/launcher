#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="products/cursor/desktop"
overlay="$project/overlay"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

grep -Eq '^COPY +overlay +/$' "$dockerfile"
grep -Eq '^ARG CURSOR_AGENT_VERSION=[0-9]{4}\.[0-9]{2}\.[0-9]{2}-[0-9a-f]+$' \
    "$dockerfile"
grep -Eq '^ARG CURSOR_AGENT_SHA256_AMD64=[0-9a-f]{64}$' "$dockerfile"
grep -Eq '^ARG CURSOR_AGENT_SHA256_ARM64=[0-9a-f]{64}$' "$dockerfile"
grep -Fq 'agent-cli-package.tar.gz' "$dockerfile"
grep -Fq 'ln -s /opt/cursor-agent/cursor-agent /usr/local/bin/agent' \
    "$dockerfile"
grep -Fq 'ln -s /opt/cursor-agent/cursor-agent /usr/local/bin/cursor-agent' \
    "$dockerfile"
grep -Fq 'kasm-patch "Cursor Agent"' "$dockerfile"
grep -Fq 'DESKTOP_PERSISTENT_PATHS="/home/agent/.cursor"' "$dockerfile"
grep -Fq 'VOLUME ["/workspace", "/home/agent/.cursor"]' "$dockerfile"

grep -Fq 'exec cursor-agent' "$overlay/usr/local/bin/desktop-welcome"
grep -Fq 'exec cursor-agent' "$overlay/usr/local/bin/desktop-harness"
grep -Fq 'cursor-agent --version' "$overlay/usr/local/bin/desktop-selftest"
grep -Fq 'background            #1B1913' \
    "$overlay/etc/xdg/kitty/theme.conf"
grep -Fq 'cursor_text_color     #1B1913' \
    "$overlay/etc/xdg/kitty/theme.conf"
grep -Fiq '#EDECEC' "$overlay/etc/xdg/kitty/theme.conf"
grep -Fiq '#14120B' "$overlay/usr/share/backgrounds/desktop-wallpaper.jpg"
grep -Fiq '#14120B' "$project/launcher/icon.svg"
grep -Fiq '#14120B' "$overlay/opt/browser/index.html"
grep -Fiq '#EDECEC' "$overlay/opt/browser/index.html"
grep -Fq 'window.client.padding.width: 0' "$dockerfile"
grep -Fq 'window.client.padding.height: 0' "$dockerfile"
grep -Fq "s/#000000/#14120B/g" "$dockerfile"
grep -Fq "s/#070707/#14120B/g" "$dockerfile"
grep -Fq "s/#1a1a1a/#14120B/g" "$dockerfile"
grep -Fq "s/#020303/#14120B/g" "$dockerfile"
grep -Fq '/usr/share/themes/Desktop/openbox-3/themerc' "$dockerfile"
grep -Fq '/etc/xdg/tint2/tint2rc' "$dockerfile"
grep -Fq '/usr/share/themes/Desktop/gtk-3.0/gtk.css' "$dockerfile"

if grep -Eiq 'AppImage|cursor[^ ]*\.deb|cursor[^ ]*\.rpm|cursor.com/downloads' \
    "$dockerfile"; then
    echo "Cursor image installs the graphical editor instead of only Agent CLI." >&2
    exit 1
fi

echo "Cursor Agent image checks passed."
