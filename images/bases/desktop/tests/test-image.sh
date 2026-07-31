#!/bin/bash

# Source checks for the desktop base.
#
# Deliberately thin. What this image does - the session coming up, the bridge
# answering, the X cookie working for the unprivileged account, the browser
# rendering without a GPU, the preview capturing the real display - is asserted
# against a running container by tests/smoke-test.sh. Restating those lines as
# greps here catches no failure the smoke test misses, and makes the Dockerfile
# and the session scripts unrefactorable.

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="bases/desktop"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

# The bridge's Go tests run in its build stage and nowhere else, so losing
# this line silently stops running them.
grep -Fq 'RUN go test ./... && \' "$dockerfile"

# Openbox executes this with sh regardless of its shebang.
if command -v dash >/dev/null 2>&1; then
    dash -n "$project/openbox/autostart"
fi

echo "Desktop base image checks passed."
