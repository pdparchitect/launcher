#!/bin/bash

# Enumerate the release units in this directory.
#
# A unit is something that versions and publishes on its own: the substrate as
# a whole, plus each product. Units are derived from the filesystem and the
# build graph, never listed, so adding a product adds a unit.
#
# Emits one tab-separated record per unit:
#
#   kind  unit       version  tag  version_file  image_version  prerelease
#
# `unit` is what `make push-substrate` / `make push-product TARGET=` and the
# publish workflow take, so the tag and the thing published cannot disagree.
#
# A product embeds the substrate. Its image release identity therefore includes
# both versions, so changing the substrate creates a new immutable product image
# even when the product's own VERSION is unchanged.

set -euo pipefail

images_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$images_dir"

graph="$(docker buildx bake --print all 2>/dev/null)"
if [ -z "$graph" ]; then
    echo "could not resolve the build graph" >&2
    exit 1
fi

emit() {
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
        "$1" "$2" "$3" "$4" "$5" "$6" "$7"
}

version_in() {
    tr -d '[:space:]' < "$1"
}

is_prerelease() {
    case "$1" in
        *-*) printf 'true\n' ;;
        *)   printf 'false\n' ;;
    esac
}

# The substrate: one unit covering every target without its own VERSION.
substrate_version="$(version_in VERSION)"
emit substrate substrate "$substrate_version" "images-v${substrate_version}" \
    VERSION "$substrate_version" "$(is_prerelease "$substrate_version")"

# One unit per product, identified by its build target so the tag names the
# thing that gets published.
while IFS=$'\t' read -r target context; do
    [ -n "$target" ] || continue
    [ -f "$context/VERSION" ] || continue
    version="$(version_in "$context/VERSION")"
    image_version="${version}-substrate.${substrate_version}"
    prerelease=false
    if [ "$(is_prerelease "$version")" = true ] ||
        [ "$(is_prerelease "$substrate_version")" = true ]; then
        prerelease=true
    fi
    emit product "$target" "$version" "${target}-v${image_version}" \
        "$context/VERSION" "$image_version" "$prerelease"
done < <(
    jq -r '.target | to_entries[] | [.key, .value.context] | @tsv' <<<"$graph" | sort
)
