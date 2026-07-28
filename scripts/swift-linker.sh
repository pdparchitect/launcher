#!/bin/bash

set -euo pipefail

launcher_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
swift_archive="$launcher_dir/macos/.build/arm64-apple-macosx/release/libLauncherNative.a"

if [[ ! -f "$swift_archive" ]]; then
    echo "Swift native shell archive was not built: $swift_archive" >&2
    exit 1
fi

swiftc_path="$(xcrun --find swiftc)"
sdk_path="$(xcrun --sdk macosx --show-sdk-path)"
deployment_target="${MACOSX_DEPLOYMENT_TARGET:-26.0}"
target_arch="arm64"
swift_link_args=()

# Go invokes its external linker using clang's argument spelling. Swiftc is the
# correct final-link driver for a binary containing Swift because it supplies
# the runtime and autolink dependencies itself. Normalize the small part of
# clang's syntax that swiftc does not accept.
while (($# > 0)); do
    argument="$1"
    shift

    case "$argument" in
        -arch)
            if (($# == 0)); then
                echo "Missing architecture after -arch" >&2
                exit 1
            fi
            target_arch="$1"
            shift
            ;;
        -mmacosx-version-min=*)
            # The Go toolchain currently emits 10.13. The SwiftUI library and
            # application deliberately target macOS 26.
            ;;
        -Wl,*)
            IFS=',' read -r -a linker_options <<<"${argument#-Wl,}"
            for linker_option in "${linker_options[@]}"; do
                swift_link_args+=("-Xlinker" "$linker_option")
            done
            ;;
        *)
            swift_link_args+=("$argument")
            ;;
    esac
done

"$swiftc_path" \
    -target "${target_arch}-apple-macosx${deployment_target}" \
    -sdk "$sdk_path" \
    "${swift_link_args[@]}" \
    -Xlinker -rpath \
    -Xlinker /usr/lib/swift
