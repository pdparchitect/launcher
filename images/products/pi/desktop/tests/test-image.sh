#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="products/pi/desktop"
overlay="$project/overlay"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

grep -Eq '^COPY +overlay +/$' "$dockerfile"
grep -Eq '^ARG PI_VERSION=[0-9]+\.[0-9]+\.[0-9]+$' "$dockerfile"
grep -Fq 'npm install -g --ignore-scripts' "$dockerfile"
grep -Fq '"@earendil-works/pi-coding-agent@${PI_VERSION}"' "$dockerfile"
grep -Fq 'test "$(pi --version)" = "${PI_VERSION}"' "$dockerfile"
grep -Fq 'apt-get install -y --no-install-recommends fd-find' "$dockerfile"
grep -Fq 'ln -s /usr/bin/fdfind /usr/local/bin/fd' "$dockerfile"
grep -Fq 'kasm-patch "Pi Desktop"' "$dockerfile"
grep -Fq 'DESKTOP_PERSISTENT_PATHS="/home/agent/.pi"' "$dockerfile"
grep -Fq 'VOLUME ["/workspace", "/home/agent/.pi"]' "$dockerfile"

grep -Fq 'exec pi' "$overlay/usr/local/bin/desktop-welcome"
grep -Fq 'exec pi' "$overlay/usr/local/bin/desktop-harness"
grep -Fq 'pi --version' "$overlay/usr/local/bin/desktop-selftest"
grep -Fiq '#142433' "$overlay/etc/xdg/kitty/theme.conf"
grep -Fiq '#161d27' \
    "$overlay/usr/share/backgrounds/desktop-wallpaper.svg"
if grep -Fiq '#142433' \
    "$overlay/usr/share/backgrounds/desktop-wallpaper.svg"; then
    echo "Pi wallpaper blends into the terminal background." >&2
    exit 1
fi
grep -Fiq '#142433' "$overlay/opt/browser/index.html"
grep -Fq '██████' "$overlay/opt/browser/index.html"
grep -Fq '██  ██' "$overlay/opt/browser/index.html"
grep -Fq '████  ██' "$overlay/opt/browser/index.html"
grep -Fq '██    ██' "$overlay/opt/browser/index.html"
if grep -Fq '<svg' "$overlay/opt/browser/index.html"; then
    echo "Pi browser page uses a substitute SVG instead of Pi's ASCII logo." >&2
    exit 1
fi
grep -Fq 'window.client.padding.width: 0' "$dockerfile"
grep -Fq 'window.client.padding.height: 0' "$dockerfile"
grep -Fq 'color46 #E8993A' "$overlay/etc/xdg/kitty/theme.conf"
if grep -Fq 'window.active.title.bg.color' "$dockerfile"; then
    echo "Pi recolours the inherited Openbox titlebar." >&2
    exit 1
fi

echo "Pi image checks passed."
