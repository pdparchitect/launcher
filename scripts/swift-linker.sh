#!/bin/bash

set -euo pipefail

launcher_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
swift_archive="$launcher_dir/macos/.build/arm64-apple-macosx/release/libLauncherNative.a"

if [[ ! -f "$swift_archive" ]]; then
    echo "Swift native shell archive was not built: $swift_archive" >&2
    exit 1
fi

autolink_file="$(mktemp)"
trap 'rm -f "$autolink_file"' EXIT

xcrun swift-autolink-extract "$swift_archive" -o "$autolink_file"
swiftc_path="$(xcrun --find swiftc)"
swift_lib_dir="$(cd "$(dirname "$swiftc_path")/../lib/swift/macosx" && pwd)"

xcrun clang \
    "$@" \
    -L"$swift_lib_dir" \
    -Wl,-rpath,/usr/lib/swift \
    @"$autolink_file"
