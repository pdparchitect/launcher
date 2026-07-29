#!/bin/bash

# Resolve the release version of a build target.
#
# The rule is a convention, not a list: a product declares its own VERSION file
# next to its Dockerfile, and anything without one is substrate and shares the
# VERSION at the root of images/. Adding a product therefore needs no change
# here, in the Makefile, or in any workflow - just a VERSION file beside its
# Dockerfile.
#
# Usage: version-of.sh <bake-target>          -> 1.4.0
#        version-of.sh --kind <bake-target>   -> product | substrate
#        version-of.sh --file <bake-target>   -> path to the VERSION file

set -euo pipefail

images_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$images_dir"

mode=version
case "${1:-}" in
    --kind|--file) mode="${1#--}"; shift ;;
esac

target="${1:-}"
if [ -z "$target" ]; then
    echo "usage: version-of.sh [--kind|--file] <bake-target>" >&2
    exit 1
fi

context="$(
    docker buildx bake --print "$target" 2>/dev/null |
        jq -r --arg t "$target" '.target[$t].context // empty'
)"
if [ -z "$context" ]; then
    echo "unknown build target: $target" >&2
    exit 1
fi

if [ -f "$context/VERSION" ]; then
    version_file="$context/VERSION"
    kind=product
else
    version_file="VERSION"
    kind=substrate
fi

case "$mode" in
    kind) printf '%s\n' "$kind"; exit 0 ;;
    file) printf '%s\n' "$version_file"; exit 0 ;;
esac

version="$(tr -d '[:space:]' < "$version_file")"
if ! bash tools/validate-version.sh "$version" >/dev/null 2>&1; then
    echo "$version_file is not publishable semantic versioning: $version" >&2
    exit 1
fi

printf '%s\n' "$version"
