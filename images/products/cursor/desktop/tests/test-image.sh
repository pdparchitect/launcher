#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="products/cursor/desktop"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

# Upstream is a tarball fetched at build time. The version and both digests
# have to stay pinned: a floating download is the one failure here that a
# passing build and a passing smoke test would both hide.
grep -Eq '^ARG CURSOR_AGENT_VERSION=[0-9]{4}\.[0-9]{2}\.[0-9]{2}-[0-9a-f]+$' \
    "$dockerfile"
grep -Eq '^ARG CURSOR_AGENT_SHA256_AMD64=[0-9a-f]{64}$' "$dockerfile"
grep -Eq '^ARG CURSOR_AGENT_SHA256_ARM64=[0-9a-f]{64}$' "$dockerfile"

if grep -Eiq 'AppImage|cursor[^ ]*\.deb|cursor[^ ]*\.rpm|cursor.com/downloads' \
    "$dockerfile"; then
    echo "Cursor image installs the graphical editor instead of only Agent CLI." >&2
    exit 1
fi

echo "Cursor Agent image checks passed."
