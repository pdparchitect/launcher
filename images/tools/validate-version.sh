#!/bin/bash

# Validate a release version that is both semantic and safe to use verbatim as
# a container image tag. SemVer build metadata uses "+", which container tags
# do not permit, so release versions deliberately support prereleases only.

set -euo pipefail

version="${1:-}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
    echo "not publishable semantic versioning: ${version:-<empty>}" >&2
    exit 1
fi
