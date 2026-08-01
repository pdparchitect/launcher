#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="products/hermes/desktop"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

# Upstream is a git checkout, so the tag alone does not pin it: a moved tag
# still builds. Keep the revision pinned and both pins well-formed.
grep -Eq '^ARG HERMES_VERSION=[0-9]+\.[0-9]+\.[0-9]+$' "$dockerfile"
grep -Eq '^ARG HERMES_SOURCE_TAG=v[0-9]{4}\.[0-9]{1,2}\.[0-9]{1,2}$' \
    "$dockerfile"
grep -Eq '^ARG HERMES_SOURCE_SHA=[0-9a-f]{40}$' "$dockerfile"
grep -Fq 'test "$(git -C /opt/hermes rev-parse HEAD)" = "$HERMES_SOURCE_SHA"' \
    "$dockerfile"

# Playwright's own installer would reinstall the browser stack the desktop base
# already carries, roughly doubling this layer for nothing.
if grep -Fq 'playwright install --with-deps' "$dockerfile"; then
    echo "Hermes reinstalls system dependencies supplied by the desktop base." >&2
    exit 1
fi

if grep -Eq '^ *org\.opencontainers\.image\.version="\$\{HERMES_VERSION\}"' \
    "$dockerfile"; then
    echo "Hermes reports the agent version as its product release version." >&2
    exit 1
fi

echo "Hermes image checks passed."
