#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
patches_dir="${GO_MODULE_PATCHES_DIR:-"$project_dir/patches"}"
target_goos="$(go env GOOS)"

if [[ ! -d "$patches_dir" ]]; then
    exec "$@"
fi

shopt -s nullglob
patch_sets=()
for patch_set in "$patches_dir"/*; do
    [[ -d "$patch_set" ]] || continue
    if [[ ! -f "$patch_set/module" ]]; then
        echo "Go module patch set has no module file: $patch_set" >&2
        exit 1
    fi
    if [[ -f "$patch_set/goos" ]] &&
        ! grep -Fqx "$target_goos" "$patch_set/goos"; then
        continue
    fi
    patch_files=("$patch_set"/*.patch)
    if [[ "${#patch_files[@]}" -eq 0 ]]; then
        echo "Go module patch set has no patches: $patch_set" >&2
        exit 1
    fi
    patch_sets+=("$patch_set")
done

if [[ "${#patch_sets[@]}" -eq 0 ]]; then
    exec "$@"
fi

temporary_dir="$(mktemp -d)"
trap 'rm -rf "$temporary_dir"' EXIT

modfile="$temporary_dir/launcher.mod"
cp "$project_dir/go.mod" "$modfile"
sumfile="$temporary_dir/launcher.sum"
cp "$project_dir/go.sum" "$sumfile"

declare -A patched_modules=()
for patch_set in "${patch_sets[@]}"; do
    module="$(tr -d '[:space:]' <"$patch_set/module")"
    if [[ -z "$module" ]]; then
        echo "Go module patch set has an empty module file: $patch_set" >&2
        exit 1
    fi
    if [[ -n "${patched_modules[$module]:-}" ]]; then
        echo "Go module has multiple applicable patch sets: $module" >&2
        exit 1
    fi
    patched_modules["$module"]=1

    (
        cd "$project_dir"
        go mod download "$module"
    )
    module_dir="$(
        cd "$project_dir"
        go list -m -f '{{.Dir}}' "$module"
    )"
    if [[ ! -d "$module_dir" ]]; then
        echo "Go module source was not found: $module" >&2
        exit 1
    fi

    patch_name="$(basename "$patch_set")"
    patched_module="$temporary_dir/modules/$patch_name"
    mkdir -p "$patched_module"
    cp -R "$module_dir/." "$patched_module/"
    chmod -R u+w "$patched_module"

    patch_files=("$patch_set"/*.patch)
    for patch_file in "${patch_files[@]}"; do
        patch -d "$patched_module" -p1 -f -F 0 -s <"$patch_file"
    done

    if [[ -f "$patched_module/go.sum" ]]; then
        cat "$patched_module/go.sum" >>"$sumfile"
    fi
    printf '\nreplace %s => %s\n' "$module" "$patched_module" >>"$modfile"
done

sort -u -o "$sumfile" "$sumfile"

export GOFLAGS="${GOFLAGS:+$GOFLAGS }-modfile=$modfile"
"$@"
