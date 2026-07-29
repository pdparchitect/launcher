#!/bin/bash

# Print the changelog section for a release unit's current version.
#
# The section is what `make check` already requires to exist, so release notes
# need no separate authoring step. A unit's changelog sits next to its VERSION:
# products/<name>/<variant>/CHANGELOG.md for a product, CHANGELOG.md at the root
# for the substrate.
#
# Usage: release-notes.sh <unit>

set -euo pipefail

images_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$images_dir"

unit="${1:-}"
if [ -z "$unit" ]; then
    echo "usage: release-notes.sh <unit>" >&2
    exit 1
fi

record="$(bash tools/units.sh | awk -F'\t' -v u="$unit" '$2 == u')"
if [ -z "$record" ]; then
    echo "'$unit' is not a release unit" >&2
    exit 1
fi

version="$(cut -f3 <<<"$record")"
version_file="$(cut -f5 <<<"$record")"
changelog="$(dirname "$version_file")/CHANGELOG.md"

if [ ! -f "$changelog" ]; then
    echo "$changelog does not exist" >&2
    exit 1
fi

# From the matching heading to the line before the next one.
notes="$(
    awk -v version="$version" '
        $0 == "## [" version "]" { capture = 1; next }
        capture && /^## / { exit }
        capture { print }
    ' "$changelog"
)"

# Trim leading and trailing blank lines without collapsing the middle.
notes="$(sed -e '/./,$!d' <<<"$notes" | sed -e :a -e '/^\n*$/{$d;N;ba' -e '}')"

if [ -z "$notes" ]; then
    echo "$changelog has no content under '## [$version]'" >&2
    exit 1
fi

printf '%s\n' "$notes"
