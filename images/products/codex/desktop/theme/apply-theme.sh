#!/bin/bash
# Recolour the inherited desktop chrome into Codex's palette.
#
# Taken from OpenAI's own Codex artwork: a slate-navy terminal sitting on a
# periwinkle field, hairlines and window edges in white, and one orchid accent
# carrying the interactive bits. The desktop is not black, and the accent is
# not the chrome - white does the outlining, exactly as the base already does.
#
# Mapping the base's colours role by role, rather than replacing themerc,
# tint2rc and gtk.css, keeps this product inheriting later base changes.

set -euo pipefail

CHROME=1D1F2E     # panel, titlebars, root menu
CANVAS=24273A     # the terminal and anything raised off the chrome
RAISED=30344A     # pressed and hovered surfaces
EDGE=3A3E52       # hairlines and separators
DIM=33374A        # unfocused window edges

WHITE=FFFFFF      # primary text - and the focused window outline, untouched
SOFT=C9CCD8       # secondary text
MUTED=6B7086      # disabled and inactive text

ORCHID=C77DEB     # the accent: cursor, selection, the one coloured token
ROSE=E5678F       # urgent

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

# Window decorations. #ffffff is deliberately not remapped anywhere in this
# file: the white focused-window outline is the detail the artwork leads with.
recolour "$themerc" \
    "000000=${CHROME}" \
    "070707=${DIM}" \
    "222222=${EDGE}" \
    "D3DAE3=${WHITE}" \
    "a8adb5=${SOFT}" \
    "7F8388=${MUTED}" \
    "76797F=${MUTED}" \
    "aeb0b6=${MUTED}" \
    "1F2328=${MUTED}" \
    "afb8c5=${SOFT}" \
    "DC143C=${ORCHID}"

# The shared theme adds a 6px resize frame around every client, which reads as
# a gutter between the titlebar and the terminal.
sed -i \
    -e 's/^window.client.padding.width:.*/window.client.padding.width: 0/' \
    -e 's/^window.client.padding.height:.*/window.client.padding.height: 0/' \
    "$themerc"

# Panel.
recolour "$tint2rc" \
    "000000=${CHROME}" \
    "1a1a1a=${RAISED}" \
    "222222=${EDGE}" \
    "cccccc=${SOFT}" \
    "b0b0b0=${SOFT}" \
    "a0a0a0=${MUTED}" \
    "666666=${MUTED}" \
    "ff4444=${ROSE}"

sed -i "s/^button_font_color = .*/button_font_color = #${WHITE} 100/" "$tint2rc"

# GTK, which is what Chromium's menus, dialogs and window edge follow. Its
# focused-window border is white already and stays that way.
recolour "$gtkcss" \
    "000000=${CHROME}" \
    "020303=${CANVAS}" \
    "050707=${RAISED}" \
    "101716=${RAISED}" \
    "172020=${EDGE}" \
    "132221=${ORCHID}" \
    "24dbc9=${ORCHID}" \
    "edf2f2=${WHITE}" \
    "d3dae3=${SOFT}" \
    "1f2328=${MUTED}" \
    "778282=${MUTED}" \
    "afb8c5=${SOFT}" \
    "dc143c=${ROSE}"
