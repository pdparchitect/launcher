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

swiftc_path="$(xcrun --find swiftc)"
swift_bin_dir="$(cd "$(dirname "$swiftc_path")" && pwd)"
swift_autolink_extract="$swift_bin_dir/swift-autolink-extract"

# Xcode 26.5 ships swift-autolink-extract in the Swift toolchain but no longer
# exposes it through `xcrun --find`. Resolve it beside swiftc, with the explicit
# default-toolchain location as a fallback for installations whose swiftc is a
# shim in /usr/bin.
if [[ ! -x "$swift_autolink_extract" ]]; then
    developer_dir="$(xcode-select -p)"
    swift_autolink_extract="$developer_dir/Toolchains/XcodeDefault.xctoolchain/usr/bin/swift-autolink-extract"
fi
if [[ ! -x "$swift_autolink_extract" ]]; then
    echo "Unable to find swift-autolink-extract in the active Xcode toolchain" >&2
    exit 1
fi

"$swift_autolink_extract" "$swift_archive" -o "$autolink_file"
swift_toolchain_bin="$(cd "$(dirname "$swift_autolink_extract")" && pwd)"
swift_lib_dir="$(cd "$swift_toolchain_bin/../lib/swift/macosx" && pwd)"
deployment_target="${MACOSX_DEPLOYMENT_TARGET:-15.0}"

xcrun clang \
    "$@" \
    -L"$swift_lib_dir" \
    -Wl,-rpath,/usr/lib/swift \
    -mmacosx-version-min="$deployment_target" \
    @"$autolink_file"
