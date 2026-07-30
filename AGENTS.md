# Launcher

Launcher is a Go application for installing and running local AI applications
through Docker or Apple `container`. The CLI, HTTP API, browser interface, and
Wails desktop application share the service in `internal/agent`.

The main areas are:

- `internal/catalog/`: embedded and remotely updated application catalogue
- `internal/httpapi/web/`: dependency-free embedded web interface
- `internal/runtime/`: container runtime abstraction
- `internal/desktop/` and `macos/`: Wails and native macOS integration
- `images/`: independently versioned container image sources
- `.github/workflows/`: validation, build, catalogue, and image automation

Use `make check` for the complete local validation. Use `make web` for browser
development, `make desktop` for the native development application, and
`make catalogue-package` to validate and package a catalogue snapshot.

## Release boundaries

Launcher binaries, the application catalogue, and container images have
separate versions:

- Launcher code uses the root `VERSION`.
- The catalogue uses `internal/catalog/catalogue.json`.
- Image release units use the `VERSION` files under `images/`.

Do not rebuild or bump Launcher for a catalogue-only change. A Launcher binary
release must bump the root `VERSION`, the sole Launcher version source, then
merge to `main`. The **Release Launcher** workflow signs and notarizes the macOS
application, creates the `v<version>` tag, and publishes the Linux and macOS
assets. Never create that tag manually.

Every catalogue snapshot intended for users must declare a new, unused
semantic version and be merged to `main`. The **Release catalogue** workflow
creates the `catalogue-v<version>` tag and publishes
`launcher-catalogue.zip`. Never reuse a published catalogue version or create
its tag manually.

## Skills

Read the matching skill before starting specialized work:

- [manage-catalogue](.agents/skills/manage-catalogue/SKILL.md): add, update,
  validate, and release catalogue entries
- [manage-images](.agents/skills/manage-images/SKILL.md): build, version, and
  release container images
- [develop-launcher](.agents/skills/develop-launcher/SKILL.md): change and
  validate the Go, web, Wails, or macOS application
