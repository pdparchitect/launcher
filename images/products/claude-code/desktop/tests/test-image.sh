#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../../.." && pwd)"
dockerfile="$project_dir/Dockerfile"
project="products/claude-code/desktop"

cd "$images_dir"
bash tools/check-project-programs.sh "$project"

# The npm release must stay pinned to an exact version; everything else about
# this image is asserted against a running container by the shared smoke test
# and by overlay/usr/local/bin/desktop-selftest.
grep -Eq '^ARG CLAUDE_CODE_VERSION=[0-9]+\.[0-9]+\.[0-9]+$' "$dockerfile"

# Claude Code writes ~/.claude.json beside ~/.claude, so the declared volume
# only holds the whole of its state while CLAUDE_CONFIG_DIR names it.
grep -Fq 'CLAUDE_CONFIG_DIR="/home/agent/.claude"' "$dockerfile"

# The product wrapper must reach the packaged CLI directly. Calling it by name
# would re-enter the wrapper, because /usr/local/bin precedes /usr/bin.
grep -Fq 'exec /usr/bin/claude "$@"' \
    "$project_dir/overlay/usr/local/bin/claude"

# The desktop is light, so the seeded appearance has to be light too: a dark
# Claude Code inside a paper terminal is the one combination that looks broken.
jq -e '.theme == "light"' \
    "$project_dir/overlay/usr/share/claude-code/settings.json" >/dev/null
jq -e '.theme == "light" and .hasCompletedOnboarding == true' \
    "$project_dir/overlay/usr/share/claude-code/.claude.json" >/dev/null

bash -n "$project_dir/theme/apply-theme.sh"

echo "Claude Code image checks passed."
