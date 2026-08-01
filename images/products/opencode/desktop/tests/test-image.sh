#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="products/opencode/desktop"
overlay="$project/overlay"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

grep -Eq '^ARG OPENCODE_VERSION=[0-9]+\.[0-9]+\.[0-9]+$' "$dockerfile"

# The overlay wrapper shadows the upstream binary on PATH. It must call the
# real one by absolute path or it recurses into itself until the session dies.
grep -Fq 'exec /usr/bin/opencode' "$overlay/usr/local/bin/opencode"

if grep -Eiq 'opencode-desktop|AppImage|\.deb|\.rpm' "$dockerfile"; then
    echo "OpenCode image installs a desktop application package." >&2
    exit 1
fi

echo "OpenCode image checks passed."
