#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="products/petbox/desktop"
overlay="$project/overlay"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

grep -Eq '^COPY +overlay +/$' "$dockerfile"
grep -Eq '^ARG CODEX_VERSION=[0-9]+\.[0-9]+\.[0-9]+$' "$dockerfile"
grep -Fq 'test "$(codex --version)" = "codex-cli ${CODEX_VERSION}"' "$dockerfile"
grep -Fq 'kasm-patch "Petbox"' "$dockerfile"
grep -Fq 'CODEX_HOME=/home/agent/.codex' "$dockerfile"
grep -Fq 'VOLUME ["/workspace", "/home/agent/.codex"]' "$dockerfile"

test -f "$overlay/etc/desktop/session.d/10-pets"
test -f "$overlay/etc/desktop/session.d/05-pet-wallpaper"
grep -Fxq 'petbox-banner' "$overlay/usr/local/bin/codex-setup"
grep -Fxq 'petbox-banner' "$overlay/usr/local/bin/pen-greeting"
if [ -d "$overlay/etc/desktop/startup.d" ]; then
    echo "Petbox installs an X-dependent program under startup.d." >&2
    exit 1
fi

PYTHONDONTWRITEBYTECODE=1 PYTHONPATH="$overlay/usr/local/lib/pets" \
    python3 -c 'import petsprites; petsprites.validate()'
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH="$overlay/usr/local/lib/pets" \
    python3 "$project/tests/test-artifacts.py"
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH="$overlay/usr/local/lib/pets" \
    python3 "$project/tests/test-wallpaper.py"

grep -Fq 'execute="petctl menu"' "$overlay/etc/xdg/openbox/menu.xml"
grep -Fq 'os.execvpe("codex", ["codex"], environment)' \
    "$overlay/usr/local/bin/petctl"
grep -Fq "cd /workspace && exec codex'" \
    "$overlay/usr/local/bin/desktop-harness"

echo "Petbox image checks passed."
