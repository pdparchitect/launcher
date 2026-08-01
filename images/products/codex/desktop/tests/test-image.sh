#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="products/codex/desktop"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

# The npm release must stay pinned to an exact version; everything else about
# this image is asserted against a running container by the shared smoke test
# and by overlay/usr/local/bin/desktop-selftest.
grep -Eq '^ARG CODEX_VERSION=[0-9]+\.[0-9]+\.[0-9]+$' "$dockerfile"

# Codex reads CODEX_HOME rather than deriving its state from HOME, because an
# Apple fixed-mount session runs as root. The declared volume and the variable
# have to name the same directory or a signed-in session is written outside it.
grep -Fq 'CODEX_HOME="/home/agent/.codex"' "$dockerfile"

bash -n "$project_dir/theme/apply-theme.sh"

echo "Codex image checks passed."
