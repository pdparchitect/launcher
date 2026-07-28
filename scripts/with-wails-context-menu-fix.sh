#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ "$(go env GOOS)" != "darwin" ]]; then
    exec "$@"
fi

(
    cd "$project_dir"
    go mod download github.com/wailsapp/wails/v2
)
wails_dir="$(
    cd "$project_dir"
    go list -m -f '{{.Dir}}' github.com/wailsapp/wails/v2
)"
wails_context="$wails_dir/internal/frontend/desktop/darwin/WailsContext.m"
old_source='window.wails.flags.disableDefaultContextMenu = true;'
new_source="if (window.wails && window.wails.flags) { window.wails.flags.disableDefaultContextMenu = true; } window.addEventListener('contextmenu', function(event) { event.preventDefault(); });"

if [[ ! -f "$wails_context" ]]; then
    echo "Wails context-menu source was not found: $wails_context" >&2
    exit 1
fi

temporary_dir="$(mktemp -d)"
trap 'rm -rf "$temporary_dir"' EXIT

patched_wails="$temporary_dir/wails"
mkdir -p "$patched_wails"
cp -R "$wails_dir/." "$patched_wails/"
chmod -R u+w "$patched_wails"

patched_context="$patched_wails/internal/frontend/desktop/darwin/WailsContext.m"
replacement_count=0
temporary_context="$temporary_dir/WailsContext.m"
while IFS= read -r line || [[ -n "$line" ]]; do
    if [[ "$line" == *"$old_source"* ]]; then
        line="${line/$old_source/$new_source}"
        replacement_count=$((replacement_count + 1))
    fi
    printf '%s\n' "$line"
done <"$wails_context" >"$temporary_context"

if [[ "$replacement_count" -ne 1 ]]; then
    echo "Wails context-menu patch expected one match, found $replacement_count" >&2
    exit 1
fi

mv "$temporary_context" "$patched_context"

modfile="$temporary_dir/launcher.mod"
cp "$project_dir/go.mod" "$modfile"
{
    cat "$project_dir/go.sum"
    cat "$patched_wails/go.sum"
} | sort -u >"$temporary_dir/launcher.sum"
printf '\nreplace github.com/wailsapp/wails/v2 => %s\n' \
    "$patched_wails" >>"$modfile"

export GOFLAGS="${GOFLAGS:+$GOFLAGS }-modfile=$modfile"
exec "$@"
