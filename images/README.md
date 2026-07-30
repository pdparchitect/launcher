# Launcher images

This directory contains the source for images consumed by Launcher. The
inheritance chain is:

```text
core/ubuntu
  -> runtimes/node
       -> bases/desktop
            -> products/hermes/desktop
            -> products/openclaw/desktop
            -> products/petbox/desktop
```

The desktop substrate is ported from **Pantalk Ghost**
(`incubator/pantalk/ghost`), which remains the reference implementation. Its
Openbox configuration, theme, panel layout, KasmVNC client patch, GTK resource
overlay and Cortile setup carry fixes that are not obvious from the files
themselves; port changes from there rather than reinventing them here.

## Responsibilities

`core/ubuntu` is a non-runnable Ubuntu 24.04 foundation: shell, network,
archive, VCS and scripting tooling only. It deliberately contains **no language
runtime** and no Python data stack, so an image needing neither pays for
neither. No user, port, health check, or entrypoint.

`runtimes/node` adds Node.js 24, a pinned pnpm, and yq. Split out of the core
so it can be skipped or swapped. Versions match the Pantalk Ghost toolchain.

`bases/desktop` is the product-neutral desktop substrate:

- KasmVNC with its web client patched — branding, control bar, connect dialog,
  spinner, status overlays and version badge all removed
- Openbox with the ported window-manager configuration, root menu and theme
- tint2 panel, Cortile tiling (floating by default), kitty, ranger, feh, picom
- Chrome on AMD64 and Chromium on ARM64, with a GTK system theme and a
  symbolic resource overlay generated from the Openbox glyph masks, so the
  browser's native menus and window controls match the desktop
- an on-demand JPEG preview of the active X display, served independently from
  KasmVNC on port 6902
- a non-root `agent` user, `/workspace`, entrypoint and health check
- fixed-ownership mount handling for Apple `container`

`products/hermes/desktop` inherits the desktop base and adds a source-pinned
Hermes Agent installation. Hermes state persists in `/home/agent/.hermes`;
the first desktop session opens `hermes setup`, and later sessions open the
Hermes TUI.

`products/openclaw/desktop` inherits the desktop base and adds a pinned
OpenClaw installed from npm. OpenClaw's own interface is the Gateway Control
UI, so this product is a browser window rather than a terminal: a supervised
gateway on loopback, and a Chromium app window opened onto it once it answers.
Configuration, credentials and sessions persist in `/home/agent/.openclaw`. It
has its own [README](products/openclaw/desktop/README.md), which is where the
gateway supervisor and the onboarding path are explained.

`products/petbox/desktop` inherits the desktop base and adds a pinned Codex
CLI plus a pen of desktop pets: each pet is a directory under `/workspace/pets`
with its own `AGENTS.md`, drawn on screen as a shaped 8-bit sprite and thinking
through `codex exec`. Codex credentials persist in `/home/agent/.codex`. It has
its own [README](products/petbox/desktop/README.md), which is where the
supervisor, the turn model and the sprite renderer are explained.

## How a product customises the desktop

A product never edits the base. It ships an `overlay/` directory laid out as a
root filesystem, copied over the base's defaults:

```text
products/hermes/desktop/overlay/
  usr/local/bin/desktop-welcome              # replaces the base greeting
  usr/share/backgrounds/desktop-wallpaper.jpeg
```

```dockerfile
COPY overlay /
```

Any file the base installs can be replaced this way — `themerc` to restyle the
window decorations, `tint2rc` for the panel, `menu.xml` for the root menu,
`rc.xml` for window-manager behaviour. Three extension points avoid needing to
replace a whole file:

| Hook | Purpose |
| --- | --- |
| `/usr/share/backgrounds/desktop-wallpaper.*` | Wallpaper, any format `feh` reads |
| `/usr/local/bin/desktop-panel-status` | Text for the tint2 status slot |
| `/usr/local/bin/desktop-harness` | Bound to Control-Shift-G; opens the product's interface |
| `/etc/desktop/startup.d/*` | Executables the entrypoint runs **before X exists** — daemons |
| `/etc/desktop/session.d/*` | Executables the session runs **with `DISPLAY` set** — anything that draws |
| `/usr/local/bin/desktop-selftest` | Product assertions `make smoke` runs inside the live session |

`DESKTOP_PERSISTENT_PATHS` declares extra paths whose ownership the entrypoint
normalises, and `kasm-patch "<brand>"` rebrands the KasmVNC client. The patch is
re-runnable: a second call rebrands in place rather than injecting its assets
again.

Hermes is pinned to a tag and its resolved commit, both declared as build
arguments in `products/hermes/desktop/Dockerfile` and verified after cloning.
The values are not repeated here or in the changelog: every built image records
them as `dev.pdparchitect.launcher.upstream.*` labels, so there is one place to
change and one place to read.

## Build

Builds run through `docker buildx bake`, so the buildx plugin is required.

```sh
make check
make build
make smoke
```

`make check` validates image sources and the build graph without building.
Each image project owns a `tests/test-image.sh` beside its Dockerfile, and
`make check` discovers and runs those project-local checks. The remaining
`tests/test-workspace.sh` only validates release metadata and build links that
span multiple projects. The shared live-desktop smoke contract lives under
`bases/desktop/tests/`.

`make smoke` runs the built product image and asserts the desktop session
actually comes up: KasmVNC serving, openbox and the panel running, the welcome
terminal started, a wallpaper applied, and nothing in `/home/agent` owned by
anyone but the session user. If the image ships `/usr/local/bin/desktop-selftest`
that runs too, so a product can assert whatever "up" means for it — Petbox
uses it to prove a sprite actually reached the screen.

That last assertion is not hypothetical. The desktop base exports
`HOME=/home/agent`, so any product build step running as root deposits its
caches into the session user's home and leaves them root-owned. Kitty then
cannot create `/home/agent/.cache/kitty`, the welcome terminal dies, and the
product boots to an empty desktop — from a Dockerfile that inspects perfectly
cleanly. Products that add root-run build steps should drop their caches and
leave `/home/agent` owned by `agent`, as `products/hermes/desktop` does.

The output images are:

```text
pdparchitect/launcher-image-core-ubuntu:local
pdparchitect/launcher-image-runtime-node:local
pdparchitect/launcher-image-base-desktop:local
pdparchitect/launcher-image-hermes-desktop:local
pdparchitect/launcher-image-openclaw-desktop:local
pdparchitect/launcher-image-petbox-desktop:local
```

Every image in the chain, products included, is named `launcher-image-*`. The
prefix keeps the whole chain in one obvious group rather than scattering four
repositories through the account's package namespace.

### Run

```sh
make run
```

Then open <http://localhost:6901> for the interactive desktop or
<http://localhost:6902/preview.jpg> for the current non-interactive snapshot.
No login is required; the desktop entrypoint starts KasmVNC with basic auth
disabled. The preview is captured on demand and cached inside the container for
two seconds.

`run` depends on the target it serves, so it always builds first and can never
put a stale image on screen.

`TARGET` selects which image from the build graph to run. It defaults to the
desktop base: that is the layer most local work happens in and the cheapest to
rebuild, which keeps `make run` a fast loop. Name a product to run one.

Nothing about the image is hardcoded. The reference comes from the graph, and
the state volumes come from the image's own `DESKTOP_PERSISTENT_PATHS`, so a
new product is runnable without editing the Makefile.

```sh
make run                             # the desktop base
make run TARGET=hermes-desktop       # the Hermes product image
make run TARGET=openclaw-desktop     # the OpenClaw product image
make run TARGET=petbox-desktop       # the Petbox product image
make run RUN_PORT=7000               # serve on a different port
make run RUN_PREVIEW_PORT=7001       # serve the preview on a different port
make run RUN_STATE=                  # throwaway session, no volumes
```

State lives in named volumes derived from the target and the mount path, so a
rebuilt image resumes where the previous one left off:

```text
launcher-hermes-desktop-workspace            -> /workspace
launcher-hermes-desktop-home-agent-hermes    -> /home/agent/.hermes
```

Hermes only runs `hermes setup` on a session whose `/home/agent/.hermes` is
still empty, so use `RUN_STATE=` or remove that volume to retest first-run
behaviour. OpenClaw behaves the same way around `/home/agent/.openclaw`.

OpenClaw's gateway is deliberately **not** published by the image. The desktop
is what is exposed, on 6901, and the Control UI is reached from inside it.
Publishing 18789 puts an authenticated control plane for an agent with shell
access onto the host network.

The equivalent raw command, if you would rather not go through `make`:

```sh
docker run --rm -it --shm-size=1g \
  -p 6901:6901 -p 6902:6902 \
  pdparchitect/launcher-image-hermes-desktop:local
```

`--shm-size=1g` matters. Chromium needs the shared memory and will crash on
Docker's 64MB default.

### Versioning

The substrate and product source version separately, because they change for
different reasons.

```text
images/VERSION                        core/ubuntu + runtimes/node + bases/desktop
images/products/<name>/<variant>/VERSION   that product alone
```

The three substrate layers share one version because each embeds its parent: a
change to the core necessarily produces a new desktop, so separate numbers
would only be three ways of saying the same thing. A product's `VERSION`
describes its own source, but its published image embeds the substrate too.
Published product image versions are therefore composite:

```text
<product-version>-substrate.<substrate-version>
```

For example, Hermes `0.2.0` built on substrate `0.1.1` is published as
`0.2.0-substrate.0.1.1`. A product-only change creates a new product image
without re-minting the substrate or other products. A substrate change creates
new composite images for every product, because those final images are what
Launcher actually runs.

The rule is a convention, not a list. A directory holding a Dockerfile is a
product if it has a `VERSION` beside it, and substrate otherwise, so version
resolution, publishing, and the release checks need no change when a product is
added - see *Adding a product* below for the one thing that does. `make check`
enforces that a product has a `VERSION` and `CHANGELOG.md`, that the version is
semantic and safe as a container tag, that the changelog has a matching
section, and that no substrate directory has quietly taken itself out of the
shared release train.

```sh
make versions
```

```text
TARGET           KIND       VERSION  FILE
core-ubuntu      substrate  0.1.0    VERSION
desktop          substrate  0.1.0    VERSION
hermes-desktop   product    0.1.0    products/hermes/desktop/VERSION
node             substrate  0.1.0    VERSION
openclaw-desktop product    0.1.0    products/openclaw/desktop/VERSION
petbox-desktop   product    0.1.0    products/petbox/desktop/VERSION
```

Each image records what it is and what it contains, so those questions never
have to be answered from memory:

| Label | Meaning |
| --- | --- |
| `org.opencontainers.image.version` | This image's own release version |
| `dev.pdparchitect.launcher.substrate.version` | The substrate release it was built on |
| `dev.pdparchitect.launcher.upstream.source` | Where the packaged software comes from, as a URL |
| `dev.pdparchitect.launcher.upstream.version` | The version of it this image packages |
| `dev.pdparchitect.launcher.upstream.revision` | Its resolved commit, when the product pins one |

`upstream.source` is a URL rather than a bare name, so `upstream.version` and
`upstream.revision` are resolvable from the image alone. `make check` requires
`source` and `version` on every product, and `revision` on any product whose
Dockerfile pins a commit - left unenforced this drifted three ways across three
products.

### Adding a product

A product needs four things. Three are files beside its Dockerfile; the fourth
is the one piece that is not discovered automatically.

```text
products/<name>/desktop/
  Dockerfile          FROM the desktop base, then COPY overlay /
  VERSION             semantic; what marks this directory as a product
  CHANGELOG.md        with a section matching VERSION
  overlay/            anything that overrides the base defaults
```

Then give it a target in `docker-bake.hcl` and list it in the `products` and
`all` groups:

```hcl
target "<name>-desktop" {
  inherits   = ["_common"]
  context    = "products/<name>/desktop"
  dockerfile = "Dockerfile"
  tags       = tag_list("<name>-desktop")
  labels = {
    "dev.pdparchitect.launcher.substrate.version" = SUBSTRATE_VERSION
  }
  contexts = {
    "pdparchitect/launcher-image-base-desktop:local" = "target:desktop"
  }
}
```

Bake cannot discover targets from the filesystem, so this step is manual - and
a product without a target is invisible: it never builds, never smoke-tests and
never publishes while every other check still passes. `make check` therefore
fails when a product directory has no target pointing at it.

### Releasing

You never create a tag. The `VERSION` files are the release intent, and the
release runs from them:

1. Bump a unit's `VERSION` and add its `CHANGELOG.md` section, in the same PR.
2. Merge to `main`. The **Images** workflow builds every image and smoke-tests
   the base and every product.
3. On success, **Release images** compares each release identity against its
   completed GitHub Release. A missing release is tagged at the exact CI-tested
   commit and its publish is dispatched. A tag without a completed release is
   retried, so a failed build cannot permanently consume a version.
4. **Publish images** builds that unit on native AMD64 and ARM64 runners and
   smoke-tests both before merging them into one manifest per repository.

Bumping one product therefore publishes that product and nothing else. The
substrate and the other products are not rebuilt. Bumping the substrate
publishes the substrate plus a new composite release of every product.

`make versions` shows each source version, and `bash tools/units.sh` shows the
release units, Git tags, and composite image versions:

```text
substrate  substrate       0.1.0  images-v0.1.0
product    hermes-desktop  0.1.0  hermes-desktop-v0.1.0-substrate.0.1.0
```

Each release also creates a **GitHub Release** at that unit's tag. Its body is
the unit's changelog section - the same one `make check` already requires - plus
every image reference and its resolved digest. That digest mapping is otherwise
recorded nowhere in the repository, which is the main reason the release exists.

Substrate tags follow the Pantalk Ghost convention:

| Tag | When |
| --- | --- |
| `X.Y.Z` | Always; immutable |
| `X.Y` | Real releases only |
| `latest` | Real releases only, and only when the release moves it |

Products publish their immutable composite version and, for a real release,
move `latest`. They do not publish an `X.Y` alias because two independently
versioned components do not have one meaningful minor release line.

A prerelease in either component publishes only its immutable version and is
marked as a prerelease on GitHub. It moves neither `X.Y` nor `latest`. SemVer
build metadata is not supported because `+` is not valid in a container tag.

Two details worth knowing. The publish workflow re-reads the release identity
at the tagged commit and **refuses** to publish from any other ref, so a
mistyped dispatch cannot mint an artifact the repository does not declare. It
is dispatched explicitly rather than triggered by the tag push: a tag created
with `GITHUB_TOKEN` does not start another workflow.

### Publish

Publishing runs from CI at the release unit's immutable Git tag. A manual retry
must select that same tag. It builds each architecture on a native runner,
pushes and smoke-tests architecture-suffixed staging tags, then combines them
into one multi-architecture manifest per repository:

```text
ghcr.io/pdparchitect/launcher-image-core-ubuntu:0.1.1
ghcr.io/pdparchitect/launcher-image-runtime-node:0.1.1
ghcr.io/pdparchitect/launcher-image-base-desktop:0.1.1
ghcr.io/pdparchitect/launcher-image-hermes-desktop:0.2.0-substrate.0.1.1
ghcr.io/pdparchitect/launcher-image-openclaw-desktop:0.1.0-substrate.0.1.1
```

ARM64 is built on `ubuntu-24.04-arm`, not under QEMU. Launcher runs the desktop
through Apple `container` on macOS, so arm64 is a first-class target, and the
Hermes layer's native compiles and Playwright install take hours emulated.

Each image carries `org.opencontainers.image.source`, which is what links the
GHCR package back to this repository. Publishing by hand is possible but
guarded, and needs a container-driver builder because the `docker` driver
cannot write to a registry:

```sh
make builder
make push-substrate REGISTRY=ghcr.io/pdparchitect BUILDER=launcher-images
make push-product TARGET=hermes-desktop REGISTRY=ghcr.io/pdparchitect \
    BUILDER=launcher-images
```

The release helpers derive the immutable image version from the `VERSION`
files. For a direct `make push`, `TAG` should stay immutable and moving tags
such as `latest` belong in `EXTRA_TAGS`.

### Ordering is derived, not declared

`docker-bake.hcl` defines one target per image, and a child receives its parent
through a named context bound to `target:<parent>`. Build order therefore comes
from the dependency graph rather than from `make` prerequisites:

```sh
make hermes-desktop   # builds core-ubuntu and desktop first, because it needs them
```

Two properties follow from this, and both are what make local iteration safe:

- **Changes propagate downward.** Editing any file under `core/ubuntu` yields a
  different core image, which invalidates the `FROM` of `bases/desktop`, which
  invalidates the `FROM` of `products/hermes/desktop`. You never have to
  remember to rebuild the children.
- **Unrelated work stays cached.** Editing only a file under
  `products/hermes/desktop` rebuilds that image alone.

The linkage works because each child's `FROM` resolves by default to the image
reference used as its bake context key. `make check` asserts those two strings
still agree, so the chain cannot silently fall back to a stale image.

### Building one image versus the whole chain

A target named only as a dependency is built `cacheonly` and is **not** loaded
into the local image store, so its `:local` tag can lag behind the source:

```sh
make build      # hermes-desktop is current; the base tags may be stale
make images     # builds and loads all three, so every tag is current
```

Use `make images` when you want to run an intermediate image directly.

### Cross-architecture and release builds

The default `docker` driver is single-platform, which is the fast path for
local work. Multi-platform output needs the container driver:

```sh
make builder
make build BUILDER=launcher-images PLATFORMS=linux/amd64,linux/arm64
```

Release builds take their version from the VERSION files rather than a TAG on
the command line - see Versioning above and `make push-substrate` /
`make push-product`.

Building `linux/arm64` on an AMD64 host runs under QEMU. The Hermes layer
compiles native extensions and installs Playwright, so prefer a native ARM64
builder node over emulation.

Inspect the resolved graph at any time with:

```sh
make print
```

## Product wallpaper

The desktop base resolves its wallpaper in this order:

1. `DESKTOP_WALLPAPER`, for a run-time override
2. `/usr/share/backgrounds/desktop-wallpaper.*`, the product drop-in
3. the neutral wallpaper shipped by the base

A product image only has to install a file at the drop-in path, in any format
`feh` reads:

```dockerfile
COPY wallpaper/launcher-desktop.jpeg /usr/share/backgrounds/desktop-wallpaper.jpeg
```

No environment variable and no change to the base are required.

## Adding files to an image

Each image directory has a `.dockerignore` that excludes everything and then
re-includes what belongs in the build context. The allowlists are directory
globs, so a new file under an already-included directory is picked up
automatically. A new *top-level* directory must be added to the allowlist, or
it will be invisible to the build and will not trigger a rebuild.

Only a final product image owns and publishes a Launcher application artifact.
Core and base images are build-time implementation details rather than
installable applications.
