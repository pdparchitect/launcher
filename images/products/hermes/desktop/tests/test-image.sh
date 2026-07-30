#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="products/hermes/desktop"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

grep -Eq '^COPY +overlay +/$' "$dockerfile"
test -d "$project/overlay"
grep -Eq '^ARG HERMES_VERSION=[0-9]+\.[0-9]+\.[0-9]+$' "$dockerfile"
grep -Eq '^ARG HERMES_SOURCE_TAG=v[0-9]{4}\.[0-9]{1,2}\.[0-9]{1,2}$' \
    "$dockerfile"
grep -Eq '^ARG HERMES_SOURCE_SHA=[0-9a-f]{40}$' "$dockerfile"
grep -Fq 'git clone --branch "${HERMES_SOURCE_TAG}" --depth 1' "$dockerfile"
grep -Fq 'test "$(git -C /opt/hermes rev-parse HEAD)" = "$HERMES_SOURCE_SHA"' \
    "$dockerfile"
grep -Fq 'dev.pdparchitect.launcher.upstream.version="${HERMES_VERSION}"' \
    "$dockerfile"
grep -Fq 'dev.pdparchitect.launcher.upstream.ref="${HERMES_SOURCE_TAG}"' \
    "$dockerfile"
grep -Fq 'HERMES_HOME=/home/agent/.hermes' "$dockerfile"
grep -Fq 'UV_LINK_MODE=copy' "$dockerfile"
grep -Fq 'VOLUME ["/workspace", "/home/agent/.hermes"]' "$dockerfile"
grep -Fq 'kasm-patch "Hermes Desktop"' "$dockerfile"

if grep -Fq 'playwright install --with-deps' "$dockerfile"; then
    echo "Hermes reinstalls system dependencies supplied by the desktop base." >&2
    exit 1
fi
grep -Fq 'npx playwright install chromium --only-shell' "$dockerfile"

if grep -Eq '^ *org\.opencontainers\.image\.version="\$\{HERMES_VERSION\}"' \
    "$dockerfile"; then
    echo "Hermes reports the agent version as its product release version." >&2
    exit 1
fi

echo "Hermes image checks passed."
