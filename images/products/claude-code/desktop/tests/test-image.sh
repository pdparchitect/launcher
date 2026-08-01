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

# bypassPermissions is refused when Claude Code runs with root privileges, and
# an Apple fixed-mount session runs the desktop as root. Defaulting to it would
# work under Docker and fail on the platform Launcher targets first.
runtime="$project_dir/overlay/etc/desktop/startup.d/05-claude-runtime"
grep -Fq 'permission_mode="${CLAUDE_PERMISSION_MODE-acceptEdits}"' "$runtime"
if grep -v '^#' "$runtime" | grep -Fq 'bypassPermissions'; then
    echo "$runtime sets bypassPermissions, which a root session refuses." >&2
    exit 1
fi

# The desktop is light, so the seeded appearance has to be light too: a dark
# Claude Code inside a paper terminal is the one combination that looks broken.
grep -Fq 'theme="${CLAUDE_THEME-light}"' "$runtime"

bash -n "$project_dir/theme/apply-theme.sh"

echo "Claude Code image checks passed."
