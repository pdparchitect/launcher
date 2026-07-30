#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../.." && pwd)"
dockerfile="$project_dir/Dockerfile"

cd "$images_dir"
bash tools/check-project-programs.sh runtimes/node

grep -Fq 'corepack prepare' "$dockerfile"

# Corepack records its activation under the building user's home unless this
# shared location makes the pin visible to the desktop account too.
if ! grep -Eq '^ENV COREPACK_HOME=' "$dockerfile"; then
    echo "The Node runtime must set COREPACK_HOME." >&2
    exit 1
fi
grep -Fq 'test "$(pnpm --version)" = "${PNPM_VERSION}"' "$dockerfile"

echo "Node runtime image checks passed."
