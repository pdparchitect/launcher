#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="products/openclaw/desktop"
overlay="$project/overlay"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

grep -Eq '^COPY +overlay +/$' "$dockerfile"
grep -Eq '^ARG OPENCLAW_VERSION=[0-9]{4}\.[0-9]{1,2}\.[0-9]+(-[0-9A-Za-z.]+)?$' \
    "$dockerfile"
grep -Fq 'grep -Fq "${OPENCLAW_VERSION}"' "$dockerfile"
grep -Fq 'kasm-patch "OpenClaw Desktop"' "$dockerfile"
grep -Fq 'DESKTOP_PERSISTENT_PATHS="/home/agent/.openclaw"' "$dockerfile"
grep -Fq 'VOLUME ["/workspace", "/home/agent/.openclaw"]' "$dockerfile"
grep -Fq 'test -e /usr/bin/openclaw && test ! -e /usr/local/bin/openclaw' \
    "$dockerfile"

test -f "$overlay/etc/desktop/startup.d/10-openclaw-gateway"
test -f "$overlay/etc/desktop/session.d/20-openclaw-control-ui"
grep -Fq 'waiting for onboarding' \
    "$overlay/usr/local/bin/openclaw-gateway-supervise"
grep -Fq 'waiting for onboarding' \
    "$overlay/usr/local/bin/desktop-selftest"

echo "OpenClaw image checks passed."
