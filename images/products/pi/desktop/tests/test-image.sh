#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="products/pi/desktop"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

# The npm release must stay pinned to an exact version; everything else about
# this image is asserted against a running container by the shared smoke test
# and by overlay/usr/local/bin/desktop-selftest.
grep -Eq '^ARG PI_VERSION=[0-9]+\.[0-9]+\.[0-9]+$' "$dockerfile"

echo "Pi image checks passed."
