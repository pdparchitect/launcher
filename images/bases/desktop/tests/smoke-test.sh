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
fixed_mount_dir=""

cleanup() {
    if [ -n "$container" ]; then
        docker stop "$container" >/dev/null 2>&1 || true
    fi
    if [ -n "$snapshot" ]; then
        rm -f "$snapshot"
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

    # Every path a product asks the base to persist has to exist in the session
    # and be writable by the account that runs the product. A path that is only
    # a Dockerfile declaration is a volume the product cannot use.
    persistent_paths="$(
        docker image inspect -f '{{range .Config.Env}}{{println .}}{{end}}' \
            "$image" | sed -n 's/^DESKTOP_PERSISTENT_PATHS=//p'
    )"
    for path in /workspace $persistent_paths; do
        in_container "sudo -u agent test -w '$path'" ||
            fail "the session user cannot write the persistent path $path"
    done

    # The pnpm pin lives in a shared corepack home so it holds for the desktop
    # account too. With the default location the pin is invisible here and pnpm
    # silently downloads the newest release on first use, so refuse the network
    # to tell the two apart.
    in_container 'sudo -u agent env HOME=/home/agent COREPACK_ENABLE_NETWORK=0 pnpm --version >/dev/null' ||
        fail "the session user cannot run the pinned pnpm without downloading one"
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
in_container 'pgrep -x desktop-bridge >/dev/null' ||
    fail "the desktop bridge is not running"

curl -fsS "http://127.0.0.1:${preview_port}/healthz" |
    jq -e '
        .status == "ready" and
        .components.bridge == "ready" and
        .components.notifications == "ready"
    ' >/dev/null || fail "the desktop bridge health response is invalid"

# Real command-line callers do not know the private address created by
# dbus-run-session. Exercise both ways agents enter these images: a root
# container shell, and a privilege-dropped product process with a deliberately
# stripped environment. Neither may autolaunch a second, disconnected bus.
bus_daemons_before="$(in_container 'pgrep -cx dbus-daemon')"
docker exec "$container" env -u DBUS_SESSION_BUS_ADDRESS \
    notify-send --app-name=LauncherSmokeRoot \
    "Root bridge notification" "Delivered" ||
    fail "root notify-send could not reach the desktop bridge"
docker exec --user agent "$container" env -u DBUS_SESSION_BUS_ADDRESS \
    HOME=/home/agent PATH=/usr/local/bin:/usr/bin:/bin \
    notify-send --app-name=LauncherSmokeAgent \
    "Agent bridge notification" "Delivered" ||
    fail "privilege-dropped notify-send could not reach the desktop bridge"
bus_daemons_after="$(in_container 'pgrep -cx dbus-daemon')"
[ "$bus_daemons_after" = "$bus_daemons_before" ] ||
    fail "notify-send autolaunched a disconnected D-Bus session"
curl -fsS "http://127.0.0.1:${preview_port}/notifications" |
    jq -e '
        .nextCursor != "" and
        any(
            .notifications[];
            .app == "LauncherSmokeRoot" and
            .title == "Root bridge notification" and
            .body == "Delivered"
        ) and
        any(
            .notifications[];
            .app == "LauncherSmokeAgent" and
            .title == "Agent bridge notification" and
            .body == "Delivered"
        )
    ' >/dev/null || fail "the desktop bridge did not expose the notification"

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
# Launch the real headed browser and wait for its window. Normal sessions launch
# it as agent. Fixed-mount sessions also exercise root, which is the account
# Openbox uses in the Apple Linux VM.
smoke_browser() {
    local browser_user="$1"
    local browser_label="$2"
    local browser_profile="/tmp/launcher-browser-smoke-${browser_label}"
    local browser_log="/tmp/launcher-browser-smoke-${browser_label}.log"
    local browser_title="LauncherBrowserSmoke${browser_label}"
    local browser_url="data:text/html,%3Ctitle%3E${browser_title}%3C/title%3E"
    local launch_prefix="env"

    if [ "$browser_user" = agent ]; then
        launch_prefix="sudo -u agent env"
    fi

    in_container "rm -rf '$browser_profile' '$browser_log'; $launch_prefix HOME=/home/agent DISPLAY=:1 XAUTHORITY=/run/launcher-desktop/Xauthority setsid chromium --user-data-dir='$browser_profile' --app='$browser_url' --window-size=400,300 --window-position=100,100 >'$browser_log' 2>&1 </dev/null &"

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
}

if [ "${SMOKE_FIXED_MOUNTS:-false}" = "true" ]; then
    smoke_browser root Root
    in_container 'test "$(stat -c "%a:%U:%G" /root/.Xauthority)" = 600:root:root' ||
        fail "Chromium did not install root's default Xauthority securely"
    in_container 'cmp -s /run/launcher-desktop/Xauthority /root/.Xauthority' ||
        fail "root's default Xauthority does not contain the active display cookie"
fi
smoke_browser agent Agent

in_container 'pgrep -x cortile >/dev/null' || fail "the tiling helper is not running"

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
