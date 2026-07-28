#!/bin/bash

set -euo pipefail

images_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
core="$images_dir/core/ubuntu/Dockerfile"
desktop="$images_dir/bases/desktop/Dockerfile"
hermes="$images_dir/products/hermes/desktop/Dockerfile"
makefile="$images_dir/Makefile"

bash -n \
    "$images_dir/bases/desktop/init.sh" \
    "$images_dir/bases/desktop/openbox/autostart" \
    "$images_dir/bases/desktop/shell/chromium" \
    "$images_dir/bases/desktop/shell/desktop-welcome" \
    "$images_dir/products/hermes/desktop/shell/desktop-welcome" \
    "$images_dir/products/hermes/desktop/shell/hermes"

grep -Fq 'FROM ubuntu:24.04' "$core"
grep -Fq 'amd64|arm64)' "$core"
if grep -Eq '^(ENTRYPOINT|CMD|HEALTHCHECK|EXPOSE)' "$core"; then
    echo "The Ubuntu core must not define a product runtime." >&2
    exit 1
fi

grep -Fq 'ARG CORE_IMAGE=pdparchitect/launcher-core-ubuntu:local' "$desktop"
grep -Fq 'FROM ${CORE_IMAGE}' "$desktop"
grep -Fq 'kasmvncserver_noble_${KASMVNC_VERSION}_${arch}.deb' "$desktop"
grep -Fq 'google-chrome-stable_current_amd64.deb' "$desktop"
grep -Fq 'apt-get install -y --no-install-recommends chromium' "$desktop"
grep -Fq 'ENTRYPOINT ["/init"]' "$desktop"
grep -Fq 'DESKTOP_PERSISTENT_PATHS' "$images_dir/bases/desktop/init.sh"
grep -Fq 'fixed-ownership mounts detected' "$images_dir/bases/desktop/init.sh"

grep -Fq 'ARG DESKTOP_IMAGE=pdparchitect/launcher-base-desktop:local' "$hermes"
grep -Fq 'FROM ${DESKTOP_IMAGE}' "$hermes"
grep -Fq 'ARG HERMES_VERSION=v2026.7.20' "$hermes"
grep -Fq \
    'ARG HERMES_SOURCE_SHA=3ef6bbd201263d354fd83ec55b3c306ded2eb72a' \
    "$hermes"
grep -Fq 'test "$(git -C /opt/hermes rev-parse HEAD)" = "$HERMES_SOURCE_SHA"' \
    "$hermes"
grep -Fq 'HERMES_HOME=/home/agent/.hermes' "$hermes"
grep -Fq 'VOLUME ["/workspace", "/home/agent/.hermes"]' "$hermes"

grep -Fq 'desktop: core-ubuntu' "$makefile"
grep -Fq 'hermes-desktop: desktop' "$makefile"
grep -Fq -- '--build-arg "CORE_IMAGE=$(CORE_UBUNTU_IMAGE)"' "$makefile"
grep -Fq -- '--build-arg "DESKTOP_IMAGE=$(DESKTOP_IMAGE)"' "$makefile"

echo "Launcher image inheritance tests passed."
