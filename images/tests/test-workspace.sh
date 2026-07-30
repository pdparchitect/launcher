#!/bin/bash

# Integration checks owned by the image workspace itself: release units,
# publishing metadata, and links between independently owned image projects.
# Image contents and behavior belong to each project's tests/test-image.sh.

set -euo pipefail

images_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$images_dir"

for version in 1.2.3 1.2.3-rc.1; do
    bash tools/validate-version.sh "$version"
done
for version in 1.2.3+build.1 1.2; do
    if bash tools/validate-version.sh "$version" >/dev/null 2>&1; then
        echo "Version '$version' is not a publishable container tag." >&2
        exit 1
    fi
done

assert_release_metadata() {
    local dir="$1" label="$2" version

    if [ ! -f "$dir/VERSION" ]; then
        echo "$label has no VERSION file at $dir/VERSION." >&2
        exit 1
    fi
    version="$(tr -d '[:space:]' < "$dir/VERSION")"
    if ! bash tools/validate-version.sh "$version" >/dev/null 2>&1; then
        echo "$label VERSION is not publishable semantic versioning: $version" >&2
        exit 1
    fi
    if [ ! -f "$dir/CHANGELOG.md" ]; then
        echo "$label has no CHANGELOG.md at $dir/CHANGELOG.md." >&2
        exit 1
    fi
    if ! grep -q "^## \[$version\]" "$dir/CHANGELOG.md"; then
        echo "$label CHANGELOG.md has no section for $version." >&2
        exit 1
    fi
}

assert_release_metadata . "The substrate"

mapfile -t project_dirs < <(
    find core runtimes bases products -type f -name Dockerfile -printf '%h\n' |
        sort
)
mapfile -t product_dirs < <(
    find products -type f -name Dockerfile -printf '%h\n' | sort
)
if [ "${#project_dirs[@]}" -eq 0 ] || [ "${#product_dirs[@]}" -eq 0 ]; then
    echo "No image projects or products were discovered." >&2
    exit 1
fi

# Every image project owns its image checks. The workspace only discovers and
# invokes them; adding a Dockerfile without tests is an error.
for project_dir in "${project_dirs[@]}"; do
    if [ ! -f "$project_dir/tests/test-image.sh" ]; then
        echo "$project_dir has no project-owned tests/test-image.sh." >&2
        exit 1
    fi
done

for product_dir in "${product_dirs[@]}"; do
    assert_release_metadata "$product_dir" "Product $product_dir"

    application="$product_dir/launcher/application.json"
    if [ ! -f "$application" ]; then
        echo "Product $product_dir has no Launcher application document." >&2
        exit 1
    fi
    if [ "$(jq -er '.schemaVersion' "$application")" != 2 ]; then
        echo "$application has an unsupported schema version." >&2
        exit 1
    fi
    if jq -e 'has("image")' "$application" >/dev/null; then
        echo "$application must derive its image from the OCI subject." >&2
        exit 1
    fi
    if ! jq -e '
        .interfaces.preview.kind == "preview" and
        .interfaces.preview.port == 6902 and
        .interfaces.preview.path == "/preview.jpg"
    ' "$application" >/dev/null; then
        echo "$application does not expose the shared desktop preview." >&2
        exit 1
    fi
    while IFS= read -r asset; do
        if [[ "$asset" = /* || "$asset" = *..* || ! -f "$product_dir/launcher/$asset" ]]; then
            echo "$application references invalid media asset '$asset'." >&2
            exit 1
        fi
    done < <(
        jq -er '.media.icon, .media.cover, .media.screenshots[].source' \
            "$application"
    )
done

graph="$(docker buildx bake --print all 2>/dev/null)"
built_contexts="$(jq -r '.target[].context' <<<"$graph" | sort -u)"
for product_dir in "${product_dirs[@]}"; do
    if ! grep -Fqx "$product_dir" <<<"$built_contexts"; then
        echo "Product $product_dir has no build target." >&2
        exit 1
    fi
done

# Substrate projects release as one root unit; products version independently.
for substrate_dir in core/ubuntu runtimes/node bases/desktop; do
    if [ -f "$substrate_dir/VERSION" ]; then
        echo "$substrate_dir must not carry a separate VERSION." >&2
        exit 1
    fi
done

while IFS= read -r target; do
    bash tools/version-of.sh "$target" >/dev/null
done < <(jq -r '.target | keys[]' <<<"$graph")

unit_records="$(bash tools/units.sh)"
if [ -z "$unit_records" ]; then
    echo "tools/units.sh resolved no release units." >&2
    exit 1
fi

substrate_units="$(awk '$1 == "substrate"' <<<"$unit_records" | wc -l)"
product_units="$(awk '$1 == "product"' <<<"$unit_records" | wc -l)"
if [ "$substrate_units" -ne 1 ]; then
    echo "Expected one substrate release unit, found $substrate_units." >&2
    exit 1
fi
if [ "$product_units" -ne "${#product_dirs[@]}" ]; then
    echo "Expected ${#product_dirs[@]} product units, found $product_units." >&2
    exit 1
fi

substrate_version="$(tr -d '[:space:]' < VERSION)"
while IFS=$'\t' read -r kind unit version tag _ image_version _; do
    [ "$kind" = product ] || continue
    expected_image_version="${version}-substrate.${substrate_version}"
    expected_tag="${unit}-v${expected_image_version}"
    if [ "$image_version" != "$expected_image_version" ] ||
        [ "$tag" != "$expected_tag" ]; then
        echo "Product $unit does not include substrate $substrate_version." >&2
        exit 1
    fi
done <<<"$unit_records"

while IFS=$'\t' read -r _ unit_name _ _ _; do
    if ! bash tools/release-notes.sh "$unit_name" >/dev/null 2>&1; then
        echo "Release notes cannot be extracted for '$unit_name'." >&2
        exit 1
    fi
done <<<"$unit_records"

label_graph="$(
    TAG=1.2.3-amd64 RELEASE_VERSION=1.2.3 \
        docker buildx bake --print substrate 2>/dev/null
)"
jq -e '
    [.target[].labels["org.opencontainers.image.version"]] |
    all(. == "1.2.3")
' <<<"$label_graph" >/dev/null || {
    echo "Architecture staging tags leak into OCI image version labels." >&2
    exit 1
}

release_workflow="../.github/workflows/images-release.yaml"
publish_workflow="../.github/workflows/images-publish.yaml"
grep -Fq 'getReleaseByTag' "$release_workflow"
grep -Fq 'RELEASE_VERSION="${{ needs.verify.outputs.image_version }}"' \
    "$publish_workflow"
grep -Fq 'bash bases/desktop/tests/smoke-test.sh "$image"' "$publish_workflow"
grep -Fq 'tag_name: ${{ needs.verify.outputs.release_tag }}' "$publish_workflow"
grep -Fq 'application/vnd.pdparchitect.launcher.application.v1' \
    "$publish_workflow"
grep -Fq '"launcher-stable"' "$publish_workflow"

# Every product image must identify the upstream source it packages.
for product_dir in "${product_dirs[@]}"; do
    dockerfile="$product_dir/Dockerfile"
    for key in source version; do
        if ! grep -Fq "dev.pdparchitect.launcher.upstream.${key}=" "$dockerfile"; then
            echo "$dockerfile does not label upstream.${key}." >&2
            exit 1
        fi
    done
    if grep -Fq "dev.pdparchitect.launcher.upstream.name=" "$dockerfile"; then
        echo "$dockerfile uses the retired upstream.name label." >&2
        exit 1
    fi
    if grep -Eq '^ARG [A-Z0-9_]*SOURCE_SHA=' "$dockerfile" &&
        ! grep -Fq "dev.pdparchitect.launcher.upstream.revision=" "$dockerfile"; then
        echo "$dockerfile pins a commit without an upstream.revision label." >&2
        exit 1
    fi
done

assert_linked() {
    local child="$1" parent_target="$2" dockerfile="$3" arg_name="$4"
    local expected actual

    expected="$(
        sed -n "s/^ARG ${arg_name}=\(.*\)$/\1/p" "$dockerfile" | head -1
    )"
    actual="$(
        jq -r --arg target "$child" --arg key "$expected" \
            '.target[$target].contexts[$key] // ""' <<<"$graph"
    )"
    if [ -z "$expected" ] || [ "$actual" != "target:${parent_target}" ]; then
        echo "Target '$child' is not linked to '$parent_target'." >&2
        exit 1
    fi
}

assert_linked node core-ubuntu runtimes/node/Dockerfile CORE_IMAGE
assert_linked desktop node bases/desktop/Dockerfile NODE_IMAGE
assert_linked hermes-desktop desktop products/hermes/desktop/Dockerfile DESKTOP_IMAGE
assert_linked openclaw-desktop desktop \
    products/openclaw/desktop/Dockerfile DESKTOP_IMAGE
assert_linked petbox-desktop desktop \
    products/petbox/desktop/Dockerfile DESKTOP_IMAGE

echo "Image workspace release and build-graph checks passed."
