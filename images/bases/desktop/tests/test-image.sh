#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="bases/desktop"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

grep -Fq 'kasmvncserver_noble_${KASMVNC_VERSION}_${arch}.deb' "$dockerfile"
grep -Fq 'apt-get install -y --no-install-recommends chromium' "$dockerfile"
grep -Fq 'ENTRYPOINT ["/init"]' "$dockerfile"
grep -Fq 'exec dbus-run-session -- openbox-session' "$project/init.sh"
grep -Fq 'desktop-bridge >>/var/log/launcher-desktop/bridge.log' \
    "$project/openbox/autostart"
grep -Fq 'DBUS_SESSION_BUS_ADDRESS" > /run/launcher-desktop/dbus-session-address' \
    "$project/openbox/autostart"
grep -Fq 'DESKTOP_PERSISTENT_PATHS' "$project/init.sh"
grep -Fq 'fixed-ownership mounts detected' "$project/init.sh"
grep -Fq 'XAUTHORITY=/run/launcher-desktop/Xauthority' "$dockerfile"
grep -Fq 'chown agent:agent "$XAUTHORITY"' "$project/init.sh"
grep -Fq 'install -o root -g root -m 600 "$authority_source" /root/.Xauthority' \
    "$project/shell/chromium"
grep -Fq 'export XAUTHORITY=/root/.Xauthority' "$project/shell/chromium"
grep -Fq 'gpu_args=(--ozone-platform=x11 --disable-gpu)' \
    "$project/shell/chromium"
grep -Fq 'display_args=("--display=$DISPLAY")' "$project/shell/chromium"
if grep -Fq -- '--disable-software-rasterizer' "$project/shell/chromium"; then
    echo "The Chromium wrapper disables its no-GPU software fallback." >&2
    exit 1
fi
if grep -Fq -- '--use-angle=swiftshader' "$project/shell/chromium"; then
    echo "The Chromium wrapper forces SwiftShader without requiring it." >&2
    exit 1
fi
bash -n "$project/shell/chromium"
awk '
    /if \[ "\$fixed_mounts" = false \]; then/ { guarded = 1 }
    guarded && /chown "\$runtime_user:\$runtime_group"/ { found = 1 }
    guarded && /^fi$/ { exit found ? 0 : 1 }
    END { if (!guarded || !found) exit 1 }
' "$project/init.sh"
grep -Fq '/etc/desktop/session.d' "$project/openbox/autostart"
grep -Fq '/etc/desktop/session.d' "$dockerfile"
grep -Fq 'chown -R "$runtime_user:$runtime_group" \' "$project/init.sh"

assert_installs() {
    local source="$1" destination="$2"

    if [ ! -e "$project/$source" ]; then
        echo "Missing desktop asset: $project/$source" >&2
        exit 1
    fi
    if ! grep -Eq "^COPY +${source} +${destination}\$" "$dockerfile"; then
        echo "The desktop base does not install $source at $destination." >&2
        exit 1
    fi
}

assert_installs openbox/rc.xml /etc/xdg/openbox/rc.xml
assert_installs openbox/menu.xml /etc/xdg/openbox/menu.xml
assert_installs openbox/autostart /etc/xdg/openbox/autostart
assert_installs openbox/theme /usr/share/themes/Desktop/openbox-3
assert_installs gtk/Desktop /usr/share/themes/Desktop
assert_installs tint2/tint2rc /etc/xdg/tint2/tint2rc
assert_installs shell/notify-send /usr/local/bin/notify-send
assert_installs cortile/cortilectl /usr/local/bin/cortilectl
assert_installs cortile/cortile-config.toml /home/agent/.config/cortile/config.toml
assert_installs kasm/custom.css /usr/share/kasmvnc/www/assets/custom.css
assert_installs kasm/favicon.svg /usr/share/kasmvnc/www/assets/favicon.svg
assert_installs kasm/patch.sh /usr/local/bin/kasm-patch
grep -Fq 'COPY --from=desktop-bridge-build /out/desktop-bridge /usr/local/bin/desktop-bridge' \
    "$dockerfile"
grep -Fq 'RUN go test ./... && \' "$dockerfile"
assert_installs browser /opt/browser

theme_name="$(
    sed -n 's|.*<name>\(.*\)</name>.*|\1|p' "$project/openbox/rc.xml" | head -1
)"
if [ "$theme_name" != "Desktop" ]; then
    echo "rc.xml names '$theme_name', but the base installs 'Desktop'." >&2
    exit 1
fi

for mask in close iconify max max_toggled; do
    if [ ! -e "$project/openbox/theme/${mask}.xbm" ]; then
        echo "Missing Openbox glyph mask: ${mask}.xbm" >&2
        exit 1
    fi
done

grep -Fq 'stamp="$www/.desktop-brand"' "$project/kasm/patch.sh"

# Openbox executes this with sh regardless of its shebang.
if command -v dash >/dev/null 2>&1; then
    dash -n "$project/openbox/autostart"
fi

echo "Desktop base image checks passed."
