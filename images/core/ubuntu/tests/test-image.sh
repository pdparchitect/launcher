#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../.." && pwd)"
dockerfile="$project_dir/Dockerfile"

cd "$images_dir"
bash tools/check-project-programs.sh core/ubuntu

# The core is a foundation, not an application.
if grep -Eq '^(ENTRYPOINT|CMD|HEALTHCHECK|EXPOSE)' "$dockerfile"; then
    echo "The Ubuntu core must not define a product runtime." >&2
    exit 1
fi

# Language runtimes belong in runtimes/, keeping the core lean.
if grep -Eq 'nodesource|install .*nodejs|corepack' "$dockerfile"; then
    echo "The Ubuntu core must not install a language runtime." >&2
    exit 1
fi

echo "Ubuntu core image checks passed."
