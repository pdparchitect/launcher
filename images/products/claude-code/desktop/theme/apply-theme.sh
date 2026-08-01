#!/bin/bash
# Recolour the inherited desktop chrome into Claude's paper palette.
#
# The base ships one dark theme for every product. Replacing themerc, tint2rc
# and gtk.css outright would give this product a prettier desktop and freeze it
# against every later base change, so instead this maps the base's colour
# vocabulary onto Anthropic's, role by role. Structure keeps inheriting; only
# the palette is ours.
#
# This is the one product on the desktop that is light. Anthropic's surface is
# paper, not a terminal void, and a warm cream desktop is the single change
# that makes the difference visible before anything else has loaded.

set -euo pipefail

# Paper, ink and clay, plus the two warm neutrals Anthropic sets against them.
PAPER=F0EEE6      # the desktop ground
CARD=FAF9F5       # anything raised off it: terminal canvas, menus, popovers
SHADE=E3DFD2      # pressed and hovered surfaces
LINE=DDD9CC       # hairlines and unfocused edges
INK=141413        # primary text
INK_SOFT=3D3D3A   # secondary text
MUTED=A5A199      # disabled and inactive text
CLAY=D97757       # the accent: focus, selection, the cursor
RUST=BF4722       # the accent under pressure

recolour() {
    local file="$1"
    shift

    local rule from to
    for rule in "$@"; do
        from="${rule%%=*}"
        to="${rule##*=}"
        # -I because the base writes hex in both cases.
        sed -i "s/#${from}/#${to}/gI" "$file"
    done
}

themerc=/usr/share/themes/Desktop/openbox-3/themerc
tint2rc=/etc/xdg/tint2/tint2rc
gtkcss=/usr/share/themes/Desktop/gtk-3.0/gtk.css

# Window decorations. The base outlines the focused window in white because its
# desktop is near-black; on paper the same job falls to clay.
recolour "$themerc" \
    "ffffff=${CLAY}" \
    "070707=${LINE}" \
    "000000=${PAPER}" \
    "222222=${LINE}" \
    "D3DAE3=${INK}" \
    "a8adb5=${INK_SOFT}" \
    "7F8388=${MUTED}" \
    "76797F=${MUTED}" \
    "aeb0b6=${MUTED}" \
    "1F2328=${MUTED}" \
    "afb8c5=${CLAY}" \
    "DC143C=${RUST}"

# The shared theme adds a 6px resize frame around every client, which on a
# light desktop reads as a cream gutter rather than as grab room.
sed -i \
    -e 's/^window.client.padding.width:.*/window.client.padding.width: 0/' \
    -e 's/^window.client.padding.height:.*/window.client.padding.height: 0/' \
    "$themerc"

# Panel. #ffffff carries two jobs here - the hover tint and the active task's
# text - and both want ink on a cream panel, so one mapping covers them.
recolour "$tint2rc" \
    "000000=${PAPER}" \
    "1a1a1a=${SHADE}" \
    "222222=${SHADE}" \
    "ffffff=${INK}" \
    "cccccc=${INK_SOFT}" \
    "b0b0b0=${INK_SOFT}" \
    "a0a0a0=${MUTED}" \
    "666666=${MUTED}" \
    "ff4444=${RUST}"

sed -i "s/^button_font_color = .*/button_font_color = #${CLAY} 100/" "$tint2rc"

# GTK, which is what Chromium's menus, dialogs and window edge follow. The
# focused-window border is the one #ffffff that must not become paper-white, so
# it is remapped by its own line before the general rule runs.
sed -i "s/border-color: #ffffff/border-color: #${CLAY}/g" "$gtkcss"
recolour "$gtkcss" \
    "000000=${PAPER}" \
    "020303=${CARD}" \
    "050707=${SHADE}" \
    "101716=${SHADE}" \
    "172020=${LINE}" \
    "132221=${CLAY}" \
    "24dbc9=${CLAY}" \
    "edf2f2=${INK}" \
    "d3dae3=${INK_SOFT}" \
    "1f2328=${MUTED}" \
    "778282=${MUTED}" \
    "afb8c5=${CLAY}" \
    "dc143c=${RUST}" \
    "ffffff=${CARD}"
