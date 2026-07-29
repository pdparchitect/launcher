#!/bin/bash

set -euo pipefail

images_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
core="$images_dir/core/ubuntu/Dockerfile"
node="$images_dir/runtimes/node/Dockerfile"
desktop="$images_dir/bases/desktop/Dockerfile"
hermes="$images_dir/products/hermes/desktop/Dockerfile"
openclaw="$images_dir/products/openclaw/desktop/Dockerfile"
pets="$images_dir/products/codex-pets/desktop/Dockerfile"

cd "$images_dir"

# Every program that ships inside an image, discovered rather than listed, so a
# newly added script cannot skip the syntax check. A product may write its
# programs in more than one language - the Codex Pets pen is Python - so each
# file is checked with the interpreter it actually runs under rather than being
# assumed to be shell.
mapfile -t program_sources < <(
    find bases products -type f \( -path '*/shell/*' -o -name 'init.sh' \
        -o -path '*/openbox/autostart' -o -path '*/kasm/*.sh' \
        -o -path '*/overlay/usr/local/bin/*' \
        -o -path '*/overlay/usr/local/lib/*' \
        -o -path '*/overlay/etc/desktop/*' -o -name 'cortilectl' \) | sort
)
test "${#program_sources[@]}" -gt 0

shell_sources=()
python_sources=()
for source in "${program_sources[@]}"; do
    if [[ "$source" == *.py ]] || head -c 64 "$source" | grep -q python; then
        python_sources+=("$source")
    else
        shell_sources+=("$source")
    fi
done

test "${#shell_sources[@]}" -gt 0
bash -n "${shell_sources[@]}"

if [ "${#python_sources[@]}" -gt 0 ]; then
    # The cache prefix keeps __pycache__ directories out of the build contexts;
    # a stray one would otherwise be copied into an image by COPY overlay /.
    pycache="$(mktemp -d)"
    PYTHONPYCACHEPREFIX="$pycache" python3 -m py_compile "${python_sources[@]}"
    rm -rf "$pycache"
fi

# Openbox runs the autostart program with sh, not bash, whatever the shebang
# says. A bashism here fails only at run time, inside the session.
if command -v dash >/dev/null 2>&1; then
    dash -n bases/desktop/openbox/autostart
fi

# The core is a foundation, not an application. Nothing that makes an image
# runnable belongs in it.
grep -Fq 'FROM ubuntu:24.04' "$core"
grep -Fq 'amd64|arm64)' "$core"
if grep -Eq '^(ENTRYPOINT|CMD|HEALTHCHECK|EXPOSE)' "$core"; then
    echo "The Ubuntu core must not define a product runtime." >&2
    exit 1
fi

# The core stays lean. Language runtimes belong in runtimes/, so a product that
# needs none does not carry them.
if grep -Eq 'nodesource|install .*nodejs|corepack' "$core"; then
    echo "The Ubuntu core must not install a language runtime." >&2
    exit 1
fi
grep -Fq 'deb.nodesource.com/setup_24.x' "$node"
grep -Fq 'corepack prepare' "$node"

# Corepack records the activated package manager under the building user's
# home. Without a shared COREPACK_HOME the pin is invisible to the desktop
# account and pnpm silently downloads the newest release on first run.
if ! grep -Eq '^ENV COREPACK_HOME=' "$node"; then
    echo "The node runtime must set COREPACK_HOME, or the pnpm pin does not" \
        "survive to the desktop user." >&2
    exit 1
fi
grep -Fq 'test "$(pnpm --version)" = "${PNPM_VERSION}"' "$node"

# The desktop base owns the runtime contract every product inherits.
grep -Fq 'kasmvncserver_noble_${KASMVNC_VERSION}_${arch}.deb' "$desktop"
grep -Fq 'apt-get install -y --no-install-recommends chromium' "$desktop"
grep -Fq 'ENTRYPOINT ["/init"]' "$desktop"
grep -Fq 'DESKTOP_PERSISTENT_PATHS' "$images_dir/bases/desktop/init.sh"
grep -Fq 'fixed-ownership mounts detected' "$images_dir/bases/desktop/init.sh"

# Two product hooks with different timing, and getting them the wrong way round
# is silent: startup.d runs from the entrypoint before X exists, session.d runs
# inside the session where DISPLAY is set. Anything that draws needs the second
# one, so assert the base still offers it.
grep -Fq '/etc/desktop/session.d' "$images_dir/bases/desktop/openbox/autostart"
grep -Fq '/etc/desktop/session.d' "$desktop"

# Workspace seeds are copied as root and the ownership pass upstream of the
# copy is stamped, so without an explicit handover the session user gets a
# directory it cannot write into.
grep -Fq 'chown -R "$runtime_user:$runtime_group" \' "$images_dir/bases/desktop/init.sh"

# The desktop is the Pantalk Ghost substrate, not a bare package install. Each
# of these was absent once and the result was a session running stock Openbox
# defaults with an unconfigured panel, so assert both that the asset exists and
# that the image actually installs it.
assert_installs() {
    local source="$1" destination="$2"

    if [ ! -e "bases/desktop/$source" ]; then
        echo "Missing desktop asset: bases/desktop/$source" >&2
        exit 1
    fi
    if ! grep -Eq "^COPY +${source} +${destination}\$" "$desktop"; then
        echo "The desktop base does not install $source at $destination." >&2
        exit 1
    fi
}

assert_installs openbox/rc.xml /etc/xdg/openbox/rc.xml
assert_installs openbox/menu.xml /etc/xdg/openbox/menu.xml
assert_installs openbox/autostart /etc/xdg/openbox/autostart
assert_installs openbox/theme /usr/share/themes/Desktop/openbox-3
assert_installs gtk/Desktop /usr/share/themes/Desktop
assert_installs tint2/tint2rc /etc/xdg/tint2/tint2rc
assert_installs cortile/cortilectl /usr/local/bin/cortilectl
assert_installs cortile/cortile-config.toml /home/agent/.config/cortile/config.toml
assert_installs kasm/custom.css /usr/share/kasmvnc/www/assets/custom.css
assert_installs kasm/favicon.svg /usr/share/kasmvnc/www/assets/favicon.svg
assert_installs kasm/patch.sh /usr/local/bin/kasm-patch
assert_installs browser /opt/browser

# The Openbox theme the window manager names must be the one that is installed.
theme_name="$(
    sed -n 's|.*<name>\(.*\)</name>.*|\1|p' bases/desktop/openbox/rc.xml | head -1
)"
if [ "$theme_name" != "Desktop" ]; then
    echo "rc.xml names Openbox theme '$theme_name', but the base installs" \
        "'Desktop'." >&2
    exit 1
fi

# The window controls Chrome draws are generated from the Openbox glyph masks,
# so those masks have to be present.
for mask in close iconify max max_toggled; do
    if [ ! -e "bases/desktop/openbox/theme/${mask}.xbm" ]; then
        echo "Missing Openbox glyph mask: ${mask}.xbm" >&2
        exit 1
    fi
done

# The KasmVNC patch must stay re-runnable: a product rebrands by calling it a
# second time, and a non-idempotent patch would inject its assets twice.
grep -Fq 'kasm-patch "Hermes Desktop"' "$hermes"
grep -Fq 'stamp="$www/.desktop-brand"' "$images_dir/bases/desktop/kasm/patch.sh"

# Release metadata. The substrate versions as one unit at the root VERSION; a
# product declares its own next to its Dockerfile. Products are discovered
# rather than listed, so a new one is held to the same rules automatically.
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

# Any depth: a product is a directory under products/ holding a Dockerfile.
mapfile -t product_dirs < <(
    find products -type f -name Dockerfile -printf '%h\n' | sort
)
if [ "${#product_dirs[@]}" -eq 0 ]; then
    echo "No product Dockerfiles found under products/." >&2
    exit 1
fi
for product_dir in "${product_dirs[@]}"; do
    assert_release_metadata "$product_dir" "Product $product_dir"
done

# A product that exists on disk but has no build target is invisible: it never
# builds, never smoke-tests, and never publishes, while every other check
# passes. Assert that some target actually points at each product directory.
built_contexts="$(
    docker buildx bake --print all 2>/dev/null | jq -r '.target[].context' | sort -u
)"
for product_dir in "${product_dirs[@]}"; do
    if ! grep -Fqx "$product_dir" <<<"$built_contexts"; then
        echo "Product $product_dir has no target in docker-bake.hcl, so it is" \
            "never built or published. Add one and list it in the 'products'" \
            "and 'all' groups." >&2
        exit 1
    fi
done

# A substrate image must not carry its own VERSION: that is what would silently
# take it out of the shared release train.
for substrate_dir in core/ubuntu runtimes/node bases/desktop; do
    if [ -f "$substrate_dir/VERSION" ]; then
        echo "$substrate_dir has its own VERSION but is substrate; the" \
            "substrate releases together from the root VERSION." >&2
        exit 1
    fi
done

# Every target must resolve to a version, and every product must be reachable
# as a release unit. Publishing reads both. Discovered rather than listed - a
# hardcoded list here silently stops covering the next product.
while IFS= read -r target; do
    bash tools/version-of.sh "$target" >/dev/null
done < <(
    docker buildx bake --print all 2>/dev/null | jq -r '.target | keys[]'
)

unit_records="$(bash tools/units.sh)"
if [ -z "$unit_records" ]; then
    echo "tools/units.sh resolved no release units." >&2
    exit 1
fi

# Exactly one substrate unit: a second would mean a substrate directory grew
# its own VERSION and fell out of the shared release train.
substrate_units="$(awk '$1 == "substrate"' <<<"$unit_records" | wc -l)"
if [ "$substrate_units" -ne 1 ]; then
    echo "Expected exactly one substrate release unit, found" \
        "$substrate_units." >&2
    exit 1
fi

# One unit per product, or a product cannot be released at all.
product_units="$(awk '$1 == "product"' <<<"$unit_records" | wc -l)"
if [ "$product_units" -ne "${#product_dirs[@]}" ]; then
    echo "There are ${#product_dirs[@]} products on disk but $product_units" \
        "product release units." >&2
    exit 1
fi

# A product image embeds the substrate, so its release identity must change
# when either its own VERSION or the substrate VERSION changes. Otherwise a
# substrate-only release can never reach the final images Launcher runs.
substrate_version="$(tr -d '[:space:]' < VERSION)"
while IFS=$'\t' read -r kind unit version tag _ image_version _; do
    [ "$kind" = product ] || continue
    expected_image_version="${version}-substrate.${substrate_version}"
    expected_tag="${unit}-v${expected_image_version}"
    if [ "$image_version" != "$expected_image_version" ] ||
        [ "$tag" != "$expected_tag" ]; then
        echo "Product $unit does not include substrate $substrate_version in" \
            "its release identity." >&2
        exit 1
    fi
done <<<"$unit_records"

# Every unit must yield release notes. The changelog check above only proves the
# heading exists; a heading with nothing under it would fail at the very last
# step of publishing, after the images are already in the registry.
while IFS=$'\t' read -r _ unit_name _ _ _; do
    if ! bash tools/release-notes.sh "$unit_name" >/dev/null 2>&1; then
        echo "Release notes cannot be extracted for '$unit_name'; its" \
            "changelog section for the current version is empty." >&2
        exit 1
    fi
done <<<"$unit_records"

# Publishing uses architecture-suffixed staging tags, but the image's OCI
# version must remain its architecture-neutral release identity.
label_graph="$(
    TAG=1.2.3-amd64 RELEASE_VERSION=1.2.3 \
        docker buildx bake --print substrate 2>/dev/null
)"
if jq -e '
    [.target[].labels["org.opencontainers.image.version"]] |
    all(. == "1.2.3")
' <<<"$label_graph" >/dev/null; then
    :
else
    echo "Architecture staging tags leak into OCI image version labels." >&2
    exit 1
fi

# A Git tag is created before publishing so the workflow can dispatch at an
# immutable ref. It is not proof that publishing completed: only a GitHub
# Release is. Existing tags without releases must therefore be retried.
release_workflow="../.github/workflows/images-release.yaml"
publish_workflow="../.github/workflows/images-publish.yaml"
grep -Fq 'getReleaseByTag' "$release_workflow"
grep -Fq 'RELEASE_VERSION="${{ needs.verify.outputs.image_version }}"' \
    "$publish_workflow"
grep -Fq 'bash tests/smoke-test.sh "$image"' "$publish_workflow"
grep -Fq 'tag_name: ${{ needs.verify.outputs.release_tag }}' "$publish_workflow"

# Every product records where the software it packages came from, in one
# convention. Left unenforced this drifted three ways across three products:
# one had a source URL and a commit, one a bare name, one nothing resolvable.
#
#   upstream.source    required - a URL, so the version below is resolvable
#   upstream.version   required
#   upstream.revision  required only when the product pins a commit
#
# upstream.name is retired: a source URL already names the project.
for product_dir in "${product_dirs[@]}"; do
    product_dockerfile="$product_dir/Dockerfile"

    for key in source version; do
        if ! grep -Fq "dev.pdparchitect.launcher.upstream.${key}=" \
            "$product_dockerfile"; then
            echo "$product_dockerfile does not label" \
                "dev.pdparchitect.launcher.upstream.${key}, so what this image" \
                "packages cannot be identified from the image." >&2
            exit 1
        fi
    done

    if grep -Fq "dev.pdparchitect.launcher.upstream.name=" "$product_dockerfile"; then
        echo "$product_dockerfile uses the retired upstream.name label. Use" \
            "upstream.source with a URL instead." >&2
        exit 1
    fi

    # A product that resolves a commit must record it, or the pin is
    # unverifiable from the artifact.
    if grep -Eq '^ARG [A-Z0-9_]*SOURCE_SHA=' "$product_dockerfile" &&
        ! grep -Fq "dev.pdparchitect.launcher.upstream.revision=" \
            "$product_dockerfile"; then
        echo "$product_dockerfile pins a commit but does not label" \
            "dev.pdparchitect.launcher.upstream.revision." >&2
        exit 1
    fi
done

# The product's own release version and the upstream component it contains are
# different facts and must not share a label.
if grep -Eq '^ *org\.opencontainers\.image\.version="\$\{HERMES_VERSION\}"' "$hermes"; then
    echo "The Hermes image reports the agent version as its own release" \
        "version. Use dev.pdparchitect.launcher.upstream.version." >&2
    exit 1
fi

# Product customisation goes through the overlay, never by editing the base.
grep -Eq '^COPY +overlay +/$' "$hermes"
if [ ! -d "products/hermes/desktop/overlay" ]; then
    echo "The Hermes product has no overlay directory." >&2
    exit 1
fi

# Hermes must stay pinned to a resolved commit and verify it after cloning.
# Assert the pinning property, not the literal version, so a version bump is
# a one-line change in the Dockerfile.
grep -Eq '^ARG HERMES_VERSION=v[0-9]{4}\.[0-9]{1,2}\.[0-9]{1,2}$' "$hermes"
grep -Eq '^ARG HERMES_SOURCE_SHA=[0-9a-f]{40}$' "$hermes"
grep -Fq 'test "$(git -C /opt/hermes rev-parse HEAD)" = "$HERMES_SOURCE_SHA"' \
    "$hermes"
grep -Fq 'HERMES_HOME=/home/agent/.hermes' "$hermes"
grep -Fq 'VOLUME ["/workspace", "/home/agent/.hermes"]' "$hermes"

# The desktop base already supplies Playwright's system libraries. Asking
# Playwright to install them again runs apt against every configured repository,
# making a browser download depend on unrelated repository signatures.
if grep -Fq 'playwright install --with-deps' "$hermes"; then
    echo "Hermes reinstalls Playwright system dependencies already supplied" \
        "by the desktop base. Install only the browser binary." >&2
    exit 1
fi
grep -Fq 'npx playwright install chromium --only-shell' "$hermes"

# OpenClaw. Same rules as Hermes: customise through the overlay, pin the
# upstream release, and verify the pin after installing it.
grep -Eq '^COPY +overlay +/$' "$openclaw"
grep -Eq '^ARG OPENCLAW_VERSION=[0-9]{4}\.[0-9]{1,2}\.[0-9]+(-[0-9A-Za-z.]+)?$' \
    "$openclaw"
grep -Fq 'grep -Fq "${OPENCLAW_VERSION}"' "$openclaw"
grep -Fq 'kasm-patch "OpenClaw Desktop"' "$openclaw"
grep -Fq 'DESKTOP_PERSISTENT_PATHS="/home/agent/.openclaw"' "$openclaw"
grep -Fq 'VOLUME ["/workspace", "/home/agent/.openclaw"]' "$openclaw"

# The overlay ships a wrapper named `openclaw`, which works only because npm's
# global prefix is /usr and the wrapper shadows it from /usr/local. If that
# ever stopped being true, COPY overlay / would overwrite npm's launcher with a
# wrapper that then invokes itself.
grep -Fq 'test -e /usr/bin/openclaw && test ! -e /usr/local/bin/openclaw' \
    "$openclaw"

openclaw_overlay="products/openclaw/desktop/overlay"

# The gateway is a daemon with no window, and the Control UI is a window with
# no daemon. Installed the other way round, the gateway would wait for an X
# server it does not need and the browser would be asked to open before there
# is a screen to open it on.
if [ ! -f "$openclaw_overlay/etc/desktop/startup.d/10-openclaw-gateway" ]; then
    echo "OpenClaw does not start its gateway from the startup hook." >&2
    exit 1
fi
if [ ! -f "$openclaw_overlay/etc/desktop/session.d/20-openclaw-control-ui" ]; then
    echo "OpenClaw does not open its Control UI from the session hook." >&2
    exit 1
fi

# The selftest proves the supervisor is alive on a desktop that has never been
# onboarded by finding the line the supervisor logs while it waits. Two files,
# one string: if they drift apart, a perfectly healthy image starts failing its
# own smoke test.
grep -Fq 'waiting for onboarding' \
    "$openclaw_overlay/usr/local/bin/openclaw-gateway-supervise"
grep -Fq 'waiting for onboarding' \
    "$openclaw_overlay/usr/local/bin/desktop-selftest"

# Codex Pets. Same rules as Hermes: customise through the overlay, pin the
# upstream harness, and verify the pin after installing it.
grep -Eq '^COPY +overlay +/$' "$pets"
grep -Eq '^ARG CODEX_VERSION=[0-9]+\.[0-9]+\.[0-9]+$' "$pets"
grep -Fq 'test "$(codex --version)" = "codex-cli ${CODEX_VERSION}"' "$pets"
grep -Fq 'kasm-patch "Codex Pets"' "$pets"
grep -Fq 'CODEX_HOME=/home/agent/.codex' "$pets"
grep -Fq 'VOLUME ["/workspace", "/home/agent/.codex"]' "$pets"

# The supervisor puts sprites on the screen, so it has to start from the
# session hook. Installed under startup.d it would run before there is an X
# server, fail, and leave a desktop with no pets and no error anybody sees.
pets_overlay="products/codex-pets/desktop/overlay"
if [ ! -f "$pets_overlay/etc/desktop/session.d/10-pets" ]; then
    echo "Codex Pets does not start its supervisor from the session hook." >&2
    exit 1
fi
if [ -d "$pets_overlay/etc/desktop/startup.d" ]; then
    echo "Codex Pets installs a startup.d program; the pen needs DISPLAY and" \
        "belongs in session.d." >&2
    exit 1
fi

# The pets' art is authored as text, one character per pixel. A ragged row
# would only ever surface as a torn silhouette inside a running session.
# PYTHONDONTWRITEBYTECODE, because importing the module byte-compiles it and
# would leave a __pycache__ directory inside the build context - which the
# check further down then fails on.
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH="$pets_overlay/usr/local/lib/pets" \
    python3 -c 'import petsprites; petsprites.validate()'

# Byte-compiled leftovers sit next to their sources and would be copied into
# the image by COPY overlay /.
if find products -type d -name __pycache__ -print -quit | grep -q .; then
    echo "A product overlay contains __pycache__; remove it." >&2
    exit 1
fi

# The live pen reaches the root menu as an Openbox pipe menu. Without the
# execute attribute the menu silently renders as an empty submenu.
grep -Fq 'execute="petctl menu"' "$pets_overlay/etc/xdg/openbox/menu.xml"

# The build graph is what orders the chain and propagates changes downward.
# A child is linked to its parent by a named context keyed on the image
# reference the child's FROM resolves to by default. If a Dockerfile default
# and its bake key drift apart the link silently breaks and the child builds
# against a stale image, so verify they still agree.
graph="$(docker buildx bake --print all 2>/dev/null)"

assert_linked() {
    local child="$1" parent_target="$2" parent_dockerfile="$3" arg_name="$4"
    local expected actual

    expected="$(
        sed -n "s/^ARG ${arg_name}=\(.*\)$/\1/p" "$parent_dockerfile" | head -1
    )"
    if [ -z "$expected" ]; then
        echo "No ${arg_name} default found in ${parent_dockerfile}." >&2
        exit 1
    fi

    actual="$(
        jq -r --arg t "$child" --arg k "$expected" \
            '.target[$t].contexts[$k] // ""' <<<"$graph"
    )"
    if [ "$actual" != "target:${parent_target}" ]; then
        echo "Target '${child}' is not linked to '${parent_target}'." >&2
        echo "  ${arg_name} resolves to: ${expected}" >&2
        echo "  bake context for that reference: ${actual:-<missing>}" >&2
        exit 1
    fi
}

assert_linked node core-ubuntu "$node" CORE_IMAGE
assert_linked desktop node "$desktop" NODE_IMAGE
assert_linked hermes-desktop desktop "$hermes" DESKTOP_IMAGE
assert_linked openclaw-desktop desktop "$openclaw" DESKTOP_IMAGE
assert_linked codex-pets-desktop desktop "$pets" DESKTOP_IMAGE

# Building the product must pull in the whole chain.
for target in core-ubuntu node desktop hermes-desktop openclaw-desktop \
    codex-pets-desktop; do
    jq -e --arg t "$target" '.target[$t]' <<<"$graph" >/dev/null
done

echo "Launcher image inheritance tests passed."
