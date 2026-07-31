#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="products/petbox/desktop"
overlay="$project/overlay"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

grep -Eq '^ARG CODEX_VERSION=[0-9]+\.[0-9]+\.[0-9]+$' "$dockerfile"

# Everything this product runs needs DISPLAY. startup.d runs before X exists,
# so a program placed there fails at boot and the pen comes up empty.
if [ -d "$overlay/etc/desktop/startup.d" ]; then
    echo "Petbox installs an X-dependent program under startup.d." >&2
    exit 1
fi

# The pet state machine - expiry, pinning, limits, the petctl round trip.
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH="$overlay/usr/local/lib/pets" \
    python3 "$project/tests/test-artifacts.py"

echo "Petbox image checks passed."
