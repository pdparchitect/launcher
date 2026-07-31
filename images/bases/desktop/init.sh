#!/bin/bash
# Desktop base entrypoint. Starts the Openbox/KasmVNC session.
#
# Product images do not replace this file. They declare extra persistent paths
# through DESKTOP_PERSISTENT_PATHS and drop executables into one of two hooks:
#
#   /etc/desktop/startup.d/  runs here, before X exists - for daemons
#   /etc/desktop/session.d/  runs from the Openbox autostart, inside the
#                            session - for anything that needs DISPLAY

set -euo pipefail

export HOME=/home/agent
export XDG_CONFIG_HOME="$HOME/.config"
export XDG_DATA_HOME="$HOME/.local/share"

persistent_paths=(/workspace)
if [ -n "${DESKTOP_PERSISTENT_PATHS:-}" ]; then
    read -r -a product_paths <<<"$DESKTOP_PERSISTENT_PATHS"
    persistent_paths+=("${product_paths[@]}")
fi

mkdir -p \
    "$HOME/.config" \
    "$HOME/.local/share/applications" \
    "$HOME/.vnc" \
    "${persistent_paths[@]}" \
    /tmp/.X11-unix \
    /var/log/launcher-desktop

# Keep the durable workspace and the desktop user's home available as Ranger
# bookmarks without replacing bookmarks the user has already assigned.
ranger_data_dir="$XDG_DATA_HOME/ranger"
ranger_bookmarks="$ranger_data_dir/bookmarks"
mkdir -p "$ranger_data_dir"
touch "$ranger_bookmarks"
grep -q '^W:' "$ranger_bookmarks" || printf 'W:/workspace\n' >> "$ranger_bookmarks"
grep -q '^H:' "$ranger_bookmarks" || printf 'H:%s\n' "$HOME" >> "$ranger_bookmarks"

# Apple container exposes macOS bind mounts as fixed-ownership VirtioFS shares
# and rejects chown on them. In that environment keep the desktop process as
# root inside its dedicated Linux VM. Docker volumes remain ownership-mutable
# and continue to run the desktop as the unprivileged agent account.
runtime_user=agent
fixed_mounts=false
if ! chown agent:agent "${persistent_paths[@]}" 2>/dev/null; then
    runtime_user=root
    fixed_mounts=true
    echo "[desktop] fixed-ownership mounts detected; using the VM root account"
fi

runtime_group="$runtime_user"
runtime_uid="$(id -u "$runtime_user")"
export XDG_RUNTIME_DIR="/run/user/$runtime_uid"
mkdir -p "$XDG_RUNTIME_DIR"
chmod 0700 "$XDG_RUNTIME_DIR"
chmod 1777 /tmp/.X11-unix

if [ "$fixed_mounts" = false ]; then
    chown "$runtime_user:$runtime_group" \
        "$HOME" \
        "$HOME/.config" \
        "$HOME/.local" \
        "$HOME/.local/share" \
        "$HOME/.local/share/applications" \
        "$HOME/.vnc" \
        "$ranger_data_dir" \
        "$ranger_bookmarks" \
        "$XDG_RUNTIME_DIR" \
        /var/log/launcher-desktop
fi

# Named-volume ownership only needs normalizing once. Recursing through the
# workspace and runtime state on every boot becomes minutes of startup latency
# once they hold real repositories and session histories.
ownership_stamp="$HOME/.config/.launcher-ownership-normalized"
if [ "$runtime_user" = agent ] && [ ! -e "$ownership_stamp" ]; then
    chown -R agent:agent "${persistent_paths[@]}" /var/log/launcher-desktop
    touch "$ownership_stamp"
    chown agent:agent "$ownership_stamp"
    echo "[desktop] normalized ownership of the persistent volumes"
fi

# Seed image-owned workspace defaults without replacing files already in the
# persistent workspace. Keeping the source outside /workspace makes new
# defaults available to brand-new volumes and to existing volumes after an
# image upgrade.
workspace_seed_dir=/usr/local/share/launcher-desktop/workspace
if [ -d "$workspace_seed_dir" ]; then
    if [ "$runtime_user" = agent ]; then
        cp --archive --update=none "$workspace_seed_dir/." /workspace/
    else
        cp --recursive --update=none "$workspace_seed_dir/." /workspace/
    fi

    # The seed is root-owned in the image and `cp --archive` preserves that,
    # so hand what was copied to the session user. Ownership normalisation ran
    # further up and is stamped, so it will not come back for these files - and
    # a seeded directory the session user cannot write into is a directory the
    # product cannot use. Only the seed's own top-level entries are touched.
    for seed_entry in "$workspace_seed_dir"/*; do
        [ -e "$seed_entry" ] || continue
        chown -R "$runtime_user:$runtime_group" \
            "/workspace/$(basename "$seed_entry")" 2>/dev/null || true
    done
fi

# The KasmVNC launcher validates Ubuntu's snake-oil key even when TLS is
# disabled. Its directory is restricted to members of the ssl-cert group.
if [ "$runtime_user" = agent ] && getent group ssl-cert >/dev/null 2>&1; then
    usermod -a -G ssl-cert agent
fi

# Use a GPU only when the host exposes a render node and the desktop user can
# open it. A passed-through node is normally root:render 0660 and the host's
# render group may not exist in this image, so presence alone does not mean it
# is usable.
gpu_node=""
gpu_node_blocked=""
for node in /dev/dri/renderD*; do
    [ -e "$node" ] || continue
    printf -v quoted_node '%q' "$node"
    if su -s /bin/bash -c \
        "test -r $quoted_node && test -w $quoted_node" "$runtime_user"; then
        gpu_node="$node"
        break
    fi
    gpu_node_blocked="$node"
done

if [ -n "$gpu_node" ]; then
    gpu_config="  gpu:
    hw3d: true
    drinode: $gpu_node"
    echo "[desktop] GPU acceleration enabled via $gpu_node"
else
    gpu_config="  gpu:
    hw3d: false"
    if [ -n "$gpu_node_blocked" ]; then
        echo "[desktop] $gpu_node_blocked is not readable by the desktop user;" \
            "using software rendering"
    else
        echo "[desktop] no GPU render node found; using software rendering"
    fi
fi

cat > "$HOME/.vnc/kasmvnc.yaml" <<YAML
network:
  protocol: http
  ssl:
    require_ssl: false
  interface: 0.0.0.0
  websocket_port: 6901

desktop:
  pixel_depth: 24
$gpu_config

encoding:
  max_frame_rate: 30

security:
  brute_force_protection:
    blacklist_threshold: 0
YAML

# Optional encoder statistics. KasmVNC's EncodeManager writes "Framebuffer
# updates" and "Max encoding time during the last N frames" to the session log,
# which is how a session gets profiled without enabling authentication for the
# /api/get_bottleneck_stats endpoint. Off by default because level 100 is
# chatty.
#
# @note KasmVNC builds a single "writer:dest:level" argument from these three
# keys, so log_dest must be set explicitly and the writer name replaces rather
# than extends the default "*" - Xvnc's other log writers go quiet while this
# is enabled.
if [ "${DESKTOP_VNC_STATS:-false}" = "true" ]; then
    cat >> "$HOME/.vnc/kasmvnc.yaml" <<'YAML'

logging:
  log_writer_name: EncodeManager
  log_dest: logfile
  level: 100
YAML
    echo "[desktop] KasmVNC encoder statistics enabled"
fi

cat > "$HOME/.vnc/xstartup" <<'XSTARTUP'
#!/bin/bash
exec openbox-session
XSTARTUP
chmod 0755 "$HOME/.vnc/xstartup"
touch "$HOME/.vnc/.de-was-selected"
chown -R "$runtime_user:$runtime_group" "$HOME/.vnc"

if [ "$fixed_mounts" = true ]; then
    echo "[desktop] keeping host-managed permissions on persistent mounts"
fi

# KasmVNC checks for these files even though browser authentication and TLS are
# disabled for this loopback-only image.
if [ ! -s "$HOME/.vnc/self.pem" ]; then
    su -s /bin/bash -c '
        export HOME=/home/agent
        openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
            -keyout "$HOME/.vnc/self.pem" \
            -out "$HOME/.vnc/self.pem" \
            -subj "/CN=launcher-desktop" >/dev/null 2>&1
        printf "launcher\nlauncher\n" |
            kasmvncpasswd -u agent -wo >/dev/null 2>&1 || true
    ' "$runtime_user"
fi

cleanup() {
    echo "[desktop] stopping"
    su -s /bin/bash -c '
        export HOME=/home/agent
        kasmvncserver -kill :1 >/dev/null 2>&1 || true
    ' "$runtime_user"
    jobs -pr | xargs -r kill 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# Product services. A product image drops executables here through its overlay
# instead of replacing this entrypoint.
if [ -d /etc/desktop/startup.d ]; then
    for startup_script in /etc/desktop/startup.d/*; do
        [ -x "$startup_script" ] || continue
        echo "[desktop] running $(basename "$startup_script")"
        DESKTOP_RUNTIME_USER="$runtime_user" "$startup_script" || \
            echo "[desktop] $(basename "$startup_script") failed" >&2
    done
fi

su -s /bin/bash -c '
    export HOME=/home/agent
    kasmvncserver -kill :1 >/dev/null 2>&1 || true
' "$runtime_user"
rm -f /tmp/.X1-lock /tmp/.X11-unix/X1

# Keep the non-interactive preview independent from KasmVNC. It captures the
# same X display on demand, but a failure here cannot interrupt the desktop
# stream on port 6901.
su -s /bin/bash -c "
    export HOME='$HOME'
    export DISPLAY=:1
    exec /usr/local/bin/desktop-preview
" "$runtime_user" \
    >>/var/log/launcher-desktop/preview.log 2>&1 &

su -s /bin/bash -c "
    export HOME='$HOME'
    export DISPLAY=:1
    export XDG_CONFIG_HOME='$XDG_CONFIG_HOME'
    export XDG_DATA_HOME='$XDG_DATA_HOME'
    export XDG_RUNTIME_DIR='$XDG_RUNTIME_DIR'
    exec kasmvncserver :1 \
        -disableBasicAuth \
        -interface 0.0.0.0 \
        -websocketPort 6901 \
        -publicIP 127.0.0.1 \
        -depth 24 \
        -httpd /usr/share/kasmvnc/www \
        -BlacklistThreshold 0 \
        -FreeKeyMappings
" "$runtime_user" >>/var/log/launcher-desktop/kasmvnc.log 2>&1 &

for attempt in $(seq 1 40); do
    if curl -fsS http://127.0.0.1:6901/ >/dev/null 2>&1; then
        echo "[desktop] ready at http://localhost:6901"
        break
    fi
    if [ "$attempt" -eq 40 ]; then
        echo "[desktop] KasmVNC did not become ready" >&2
        tail -n 100 /var/log/launcher-desktop/kasmvnc.log >&2 || true
        exit 1
    fi
    sleep 1
done

while curl -fsS http://127.0.0.1:6901/ >/dev/null 2>&1; do
    sleep 5
done

echo "[desktop] browser environment stopped unexpectedly" >&2
exit 1
