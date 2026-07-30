#!/bin/bash

# Runs a built desktop image and asserts the desktop session actually comes
# up. Source inspection cannot catch this class of failure: an image whose
# Dockerfile is entirely well-formed can still boot to an empty desktop
# because a build step left the session user unable to write its own home.

set -euo pipefail

image="${1:-pdparchitect/launcher-image-hermes-desktop:local}"
port="${SMOKE_PORT:-16999}"
preview_port="${SMOKE_PREVIEW_PORT:-17000}"
container=""
snapshot=""

cleanup() {
    if [ -n "$container" ]; then
        docker stop "$container" >/dev/null 2>&1 || true
    fi
    if [ -n "$snapshot" ]; then
        rm -f "$snapshot"
    fi
}
trap cleanup EXIT INT TERM

fail() {
    echo "smoke: FAIL: $*" >&2
    if [ -n "$container" ]; then
        echo "--- container log ---" >&2
        docker logs "$container" 2>&1 | tail -30 >&2
    fi
    exit 1
}

docker image inspect "$image" >/dev/null 2>&1 ||
    fail "image not built: $image"

container="$(
    docker run -d --rm --shm-size=1g \
        -p "${port}:6901" \
        -p "${preview_port}:6902" \
        "$image"
)"

# The desktop must serve KasmVNC.
ready=""
for _ in $(seq 1 90); do
    if curl -fsS "http://127.0.0.1:${port}/" >/dev/null 2>&1; then
        ready=yes
        break
    fi
    if ! docker inspect -f '{{.State.Running}}' "$container" 2>/dev/null |
        grep -Fqx true; then
        fail "container exited before serving KasmVNC"
    fi
    sleep 1
done
[ -n "$ready" ] || fail "KasmVNC never became ready on port ${port}"

in_container() {
    docker exec "$container" bash -c "$1"
}

# The session user must own its home. A root-owned directory here means a
# build step ran as root with HOME=/home/agent, which silently breaks every
# application that caches under it.
strays="$(
    in_container 'find /home/agent -maxdepth 1 -mindepth 1 ! -user agent -printf "%f\n" 2>/dev/null || true'
)"
[ -z "$strays" ] || fail "not owned by the session user in /home/agent: $strays"

# Openbox runs the autostart program, which brings up the panel and the
# welcome terminal. Give the session a moment to settle.
settled=""
for _ in $(seq 1 30); do
    if in_container 'pgrep -x kitty >/dev/null'; then
        settled=yes
        break
    fi
    sleep 1
done
[ -n "$settled" ] || fail "the welcome terminal never started"

in_container 'pgrep -x openbox >/dev/null' || fail "openbox is not running"
in_container 'pgrep -x tint2 >/dev/null' || fail "the panel is not running"

# The desktop substrate has to actually be configured, not merely installed.
# Openbox, tint2, and cortile all start happily with stock defaults, so a
# session can look "up" while none of this image's configuration is in effect.

# tint2 must be running against the image's panel layout, not a stale copy in
# the user config.
in_container 'ps -eo args --no-headers | grep -q -- "[t]int2 -c /etc/xdg/tint2/tint2rc"' ||
    fail "tint2 is not running against /etc/xdg/tint2/tint2rc"

# The Openbox theme rc.xml names must exist, or the window manager silently
# falls back to its built-in appearance.
theme="$(in_container 'sed -n "s|.*<name>\(.*\)</name>.*|\1|p" /etc/xdg/openbox/rc.xml | head -1' | tr -d '\r')"
[ -n "$theme" ] || fail "rc.xml does not name an Openbox theme"
in_container "test -f /usr/share/themes/${theme}/openbox-3/themerc" ||
    fail "rc.xml names Openbox theme '${theme}', which is not installed"

in_container 'test -f /etc/xdg/openbox/menu.xml' || fail "no root menu installed"
in_container 'pgrep -x cortile >/dev/null' || fail "the tiling helper is not running"

# The KasmVNC client must be the patched one: branded, with the upstream
# control bar and connect dialog hidden.
in_container 'test -f /usr/share/kasmvnc/www/.desktop-brand' ||
    fail "the KasmVNC client was never patched"
in_container 'grep -q "assets/custom.css" /usr/share/kasmvnc/www/index.html' ||
    fail "the KasmVNC client is missing this desktop's stylesheet"
if in_container 'grep -Rq "\"KasmVNC\"" /usr/share/kasmvnc/www/assets/ui-*.js'; then
    fail "the KasmVNC brand string was not replaced"
fi

# Chrome's window controls are drawn from PNGs generated out of the Openbox
# glyph masks at build time.
in_container 'test -n "$(find /usr/share/launcher-desktop/gtk-overlay -name "*.png" -print -quit)"' ||
    fail "the GTK symbolic resource overlay was not generated"

# feh sets the root pixmap and exits, so assert the effect rather than the
# process.
in_container 'DISPLAY=:1 xprop -root _XROOTPMAP_ID 2>/dev/null | grep -q "pixmap id"' ||
    fail "no wallpaper was applied to the root window"

# The preview endpoint must return a real capture of the running X display,
# not merely report that its HTTP process is alive.
snapshot="$(mktemp)"
preview_ready=""
for _ in $(seq 1 20); do
    if curl -fsS "http://127.0.0.1:${preview_port}/preview.jpg" \
        -o "$snapshot"; then
        preview_ready=yes
        break
    fi
    sleep 1
done
[ -n "$preview_ready" ] || fail "desktop preview never became available"
magic="$(od -An -tx1 -N3 "$snapshot" | tr -d '[:space:]')"
[ "$magic" = "ffd8ff" ] || fail "desktop preview is not a JPEG image"
[ "$(wc -c < "$snapshot")" -gt 1024 ] ||
    fail "desktop preview is unexpectedly small"

# Whatever the product itself considers "up". Everything above proves a desktop
# is running; only the product knows whether the product is. A product opts in
# by installing this program through its overlay, so the base and any product
# without one are unaffected.
if in_container 'test -x /usr/local/bin/desktop-selftest'; then
    echo "smoke: running the product selftest"
    docker exec "$container" bash -lc \
        'DISPLAY=:1 sudo -u agent -E /usr/local/bin/desktop-selftest' ||
        fail "the product selftest failed"
fi

echo "smoke: ${image} came up cleanly"
