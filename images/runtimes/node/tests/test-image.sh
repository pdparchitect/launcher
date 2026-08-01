#!/bin/bash

set -euo pipefail

project_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
images_dir="$(cd "$project_dir/../.." && pwd)"
dockerfile="$project_dir/Dockerfile"

cd "$images_dir"
bash tools/check-project-programs.sh runtimes/node

# Nothing else to assert about the source. The build itself fails when the
# pinned pnpm is not the one that ends up installed, and whether that pin is
# visible to the unprivileged desktop account - the failure that actually
# happens here - is asserted against a running container by the desktop
# smoke test.

echo "Node runtime image checks passed."
