#!/bin/bash

set -euo pipefail

export HOME=/home/agent
export XDG_CONFIG_HOME="$HOME/.config"
export XDG_DATA_HOME="$HOME/.local/share"

resolution="${DESKTOP_RESOLUTION:-1920x1080}"
if [[ ! "$resolution" =~ ^[0-9]{3,5}x[0-9]{3,5}$ ]]; then
    echo "[desktop] invalid DESKTOP_RESOLUTION: $resolution" >&2
    exit 1
fi

width="${resolution%x*}"
height="${resolution#*x}"

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

# Apple container exposes macOS bind mounts as fixed-ownership VirtioFS
# shares. Use root inside that dedicated VM when the mounts cannot be chowned.
runtime_user=agent
if ! chown agent:agent "${persistent_paths[@]}" 2>/dev/null; then
    runtime_user=root
    echo "[desktop] fixed-ownership mounts detected; using the VM root account"
fi

runtime_group="$runtime_user"
runtime_uid="$(id -u "$runtime_user")"
export XDG_RUNTIME_DIR="/run/user/$runtime_uid"
mkdir -p "$XDG_RUNTIME_DIR"
chmod 0700 "$XDG_RUNTIME_DIR"
chmod 1777 /tmp/.X11-unix

chown "$runtime_user:$runtime_group" \
    "$HOME" \
    "$HOME/.config" \
    "$HOME/.local" \
    "$HOME/.local/share" \
    "$HOME/.local/share/applications" \
    "$HOME/.vnc" \
    "$XDG_RUNTIME_DIR" \
    /var/log/launcher-desktop

if [ "$runtime_user" = agent ]; then
    ownership_stamp="$HOME/.config/.launcher-ownership-normalized"
    if [ ! -e "$ownership_stamp" ]; then
        chown -R agent:agent "${persistent_paths[@]}"
        touch "$ownership_stamp"
        chown agent:agent "$ownership_stamp"
    fi
fi

if [ "$runtime_user" = agent ] &&
    getent group ssl-cert >/dev/null 2>&1; then
    usermod -a -G ssl-cert agent
fi

gpu_node=""
for node in /dev/dri/renderD*; do
    [ -e "$node" ] || continue
    printf -v quoted_node '%q' "$node"
    if su -s /bin/bash -c \
        "test -r $quoted_node && test -w $quoted_node" "$runtime_user"; then
        gpu_node="$node"
        break
    fi
done

if [ -n "$gpu_node" ]; then
    gpu_config="  gpu:
    hw3d: true
    drinode: $gpu_node"
else
    gpu_config="  gpu:
    hw3d: false"
fi

cat > "$HOME/.vnc/kasmvnc.yaml" <<YAML
network:
  protocol: http
  ssl:
    require_ssl: false
  interface: 0.0.0.0
  websocket_port: 6901

desktop:
  resolution:
    width: $width
    height: $height
  pixel_depth: 24
$gpu_config

encoding:
  max_frame_rate: 30

security:
  brute_force_protection:
    blacklist_threshold: 0
YAML

cat > "$HOME/.vnc/xstartup" <<'XSTARTUP'
#!/bin/bash
exec openbox-session
XSTARTUP
chmod 0755 "$HOME/.vnc/xstartup"
touch "$HOME/.vnc/.de-was-selected"
chown -R "$runtime_user:$runtime_group" "$HOME/.vnc"

if [ ! -s "$HOME/.vnc/self.pem" ]; then
    su -s /bin/bash -c '
        export HOME=/home/agent
        openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
            -keyout "$HOME/.vnc/self.pem" \
            -out "$HOME/.vnc/self.pem" \
            -subj "/CN=launcher-desktop" >/dev/null 2>&1
        printf "launcher\nlauncher\n" |
            kasmvncpasswd -u agent -wo >/dev/null 2>&1
    ' "$runtime_user"
fi

cleanup() {
    echo "[desktop] stopping"
    su -s /bin/bash -c '
        export HOME=/home/agent
        kasmvncserver -kill :1 >/dev/null 2>&1 || true
    ' "$runtime_user"
}
trap cleanup EXIT INT TERM

su -s /bin/bash -c '
    export HOME=/home/agent
    kasmvncserver -kill :1 >/dev/null 2>&1 || true
' "$runtime_user"
rm -f /tmp/.X1-lock /tmp/.X11-unix/X1

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
        -geometry '$resolution' \
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
