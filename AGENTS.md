# Launcher

Launcher is a Go application for installing and running local AI applications
through Docker or Apple `container`. The CLI, HTTP API, browser interface, and
Wails desktop application share the service in `internal/agent`.

The main areas are:

- `internal/catalog/`: OCI publisher-feed resolution, application validation,
  and resilient local caching
- `internal/httpapi/web/`: dependency-free embedded web interface
- `internal/runtime/`: container runtime abstraction
- `internal/desktop/` and `macos/`: Wails and native macOS integration
- `images/`: independently versioned container image sources
- `publisher/`: the PDP Architect application discovery feed
- `.github/workflows/`: validation, build, feed, and image automation

Use `make check` for the complete local validation. Use `make web` for browser
development and `make desktop` for the native development application.

## Release boundaries

Launcher binaries and container images have separate versions:

- Launcher code uses the root `VERSION`.
- Image release units use the `VERSION` files under `images/`.

A Launcher binary release must bump the root `VERSION`, the sole Launcher
version source, then merge to `main`. The **Release Launcher** workflow signs
and notarizes the macOS application, creates the `v<version>` tag, and
publishes the Linux and macOS assets. Never create that tag manually.

### Publish an image-owned Launcher application

Each product owns `launcher/application.json` and its artwork beside its image
source. Its `version` must match the product `VERSION`. Do not put an `image`
field in that document: the release workflow derives the immutable image from
the application artifact's OCI subject.

For a product update, change the image and application definition together,
bump the product `VERSION` and `CHANGELOG.md`, validate, and merge to `main`.
The image workflow builds the final multi-architecture image first, attaches
the application bundle to that digest, publishes an immutable
`launcher-<image-version>` tag, and moves `launcher-stable` for stable
releases. There is no separate catalogue version or promotion release.

Edit `publisher/feed.json` only to add or remove a discoverable application.
Normal application releases keep the same feed entry and only move the
image-owned `launcher-stable` channel. Use `publish-launcher-application` for
the complete workflow.

## Skills

Read the matching skill before starting specialized work:

- [publish-launcher-application](.agents/skills/publish-launcher-application/SKILL.md):
  release an image and its attached Launcher application artifact
- [manage-application-registry](.agents/skills/manage-application-registry/SKILL.md):
  manage image-owned application documents, publisher feeds, validation, and
  registry caching
- [manage-images](.agents/skills/manage-images/SKILL.md): build, version, and
  release container images
- [capture-application-preview](.agents/skills/capture-application-preview/SKILL.md):
  obtain a screenshot from a declared preview HTTP interface
- [develop-launcher](.agents/skills/develop-launcher/SKILL.md): change and
  validate the Go, web, Wails, or macOS application
