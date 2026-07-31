#!/bin/bash

# Runs a built desktop image and asserts the desktop session actually comes
# up. Source inspection cannot catch this class of failure: an image whose
# Dockerfile is entirely well-formed can still boot to an empty desktop
# because a build step left the session user unable to write its own home.

set -euo pipefail

image="${1:-pdparchitect/launcher-image-hermes-desktop:local}"
port="${SMOKE_PORT:-16999}"
preview_port="${SMOKE_PREVIEW_PORT:-17000}"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
container=""
snapshot=""
browser_snapshot=""
fixed_mount_dir=""

cleanup() {
    if [ -n "$container" ]; then
        docker stop "$container" >/dev/null 2>&1 || true
    fi
    if [ -n "$snapshot" ]; then
        rm -f "$snapshot"
    fi
    if [ -n "$browser_snapshot" ]; then
        rm -f "$browser_snapshot"
    fi
    if [ -n "$fixed_mount_dir" ]; then
        rmdir "$fixed_mount_dir" 2>/dev/null || true
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

docker_mount_args=()
if [ "${SMOKE_FIXED_MOUNTS:-false}" = "true" ]; then
    fixed_mount_dir="$(mktemp -d)"
    docker_mount_args+=(--volume "$fixed_mount_dir:/workspace:ro")
fi

container="$(
    docker run -d --rm --shm-size=1g \
        -p "${port}:6901" \
        -p "${preview_port}:6902" \
        "${docker_mount_args[@]}" \
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

if [ "${SMOKE_FIXED_MOUNTS:-false}" = "true" ]; then
    docker logs "$container" 2>&1 |
        grep -F 'fixed-ownership mounts detected; using the VM root account' \
            >/dev/null ||
        fail "the read-only workspace did not exercise fixed-mount mode"
fi

in_container() {
    docker exec "$container" bash -c "$1"
}

# The session user must own its home. A root-owned directory here means a
# build step ran as root with HOME=/home/agent, which silently breaks every
# application that caches under it.
if [ "${SMOKE_FIXED_MOUNTS:-false}" != "true" ]; then
    strays="$(
        in_container 'find /home/agent -maxdepth 1 -mindepth 1 ! -user agent -printf "%f\n" 2>/dev/null || true'
    )"
    [ -z "$strays" ] || fail "not owned by the session user in /home/agent: $strays"
fi

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

# Products can deliberately run as the unprivileged agent even when an Apple
# fixed-ownership mount makes the desktop session itself run as root. The base
# must grant that account access to the display instead of making every product
# understand how KasmVNC created its Xauthority cookie.
x_access=""
for _ in $(seq 1 20); do
    if in_container 'sudo -u agent env HOME=/home/agent DISPLAY=:1 XAUTHORITY=/run/launcher-desktop/Xauthority xprop -root >/dev/null 2>&1'; then
        x_access=yes
        break
    fi
    sleep 1
done
[ -n "$x_access" ] || fail "the agent account cannot authenticate to the X display"

# A successful xprop only proves that the cookie works for a small X11 client.
# Launch the real headed browser and capture a known rendered frame. Normal
# sessions launch it as agent. Fixed-mount sessions also exercise root, which
# is the account Openbox uses in the Apple Linux VM.
smoke_browser() {
    local browser_user="$1"
    local browser_label="$2"
    local browser_profile="/tmp/launcher-browser-smoke-${browser_label}"
    local browser_log="/tmp/launcher-browser-smoke-${browser_label}.log"
    local browser_frame="/tmp/launcher-browser-smoke-${browser_label}.ppm"
    local browser_title="LauncherBrowserSmoke${browser_label}"
    local browser_url="data:text/html,%3Ctitle%3E${browser_title}%3C/title%3E%3Cbody%20style=margin:0%3Bbackground:%2300ff00%3E"
    local launch_prefix="env"

    if [ "$browser_user" = agent ]; then
        launch_prefix="sudo -u agent env"
    fi

    in_container "rm -rf '$browser_profile' '$browser_log' '$browser_frame'; $launch_prefix HOME=/home/agent DISPLAY=:1 XAUTHORITY=/run/launcher-desktop/Xauthority setsid chromium --user-data-dir='$browser_profile' --app='$browser_url' --window-size=400,300 --window-position=100,100 >'$browser_log' 2>&1 </dev/null &"

    local browser_ready=""
    for _ in $(seq 1 30); do
        if in_container "env DISPLAY=:1 XAUTHORITY=/run/launcher-desktop/Xauthority xdotool search --onlyvisible --name '^${browser_title}$' >/dev/null 2>&1"; then
            browser_ready=yes
            break
        fi
        sleep 1
    done
    [ -n "$browser_ready" ] ||
        fail "Chromium did not create a visible ${browser_user} software-rendered window"

    if in_container "ps -eo args | grep '[l]auncher-browser-smoke-${browser_label}' | grep -Fq -- '--disable-software-rasterizer'"; then
        fail "Chromium disabled its software rasterizer without a GPU"
    fi

    local browser_window
    browser_window="$(
        in_container "env DISPLAY=:1 XAUTHORITY=/run/launcher-desktop/Xauthority xdotool search --onlyvisible --name '^${browser_title}$' | head -1"
    )"
    in_container "env DISPLAY=:1 XAUTHORITY=/run/launcher-desktop/Xauthority xdotool windowactivate --sync '$browser_window'; env DISPLAY=:1 XAUTHORITY=/run/launcher-desktop/Xauthority scrot --focused '$browser_frame'"
    browser_snapshot="$(mktemp)"
    docker cp "$container:$browser_frame" "$browser_snapshot" >/dev/null
    python3 "$script_dir/assert-browser-frame.py" "$browser_snapshot" ||
        fail "Chromium created a ${browser_user} window but did not paint its software-rendered page"
    rm -f "$browser_snapshot"
    browser_snapshot=""
}

if [ "${SMOKE_FIXED_MOUNTS:-false}" = "true" ]; then
    smoke_browser root Root
    in_container 'test "$(stat -c "%a:%U:%G" /root/.Xauthority)" = 600:root:root' ||
        fail "Chromium did not install root's default Xauthority securely"
    in_container 'cmp -s /run/launcher-desktop/Xauthority /root/.Xauthority' ||
        fail "root's default Xauthority does not contain the active display cookie"
fi
smoke_browser agent Agent

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
