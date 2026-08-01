#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="products/openclaw/desktop"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

grep -Eq '^ARG OPENCLAW_VERSION=[0-9]{4}\.[0-9]{1,2}\.[0-9]+(-[0-9A-Za-z.]+)?$' \
    "$dockerfile"

echo "OpenClaw image checks passed."
