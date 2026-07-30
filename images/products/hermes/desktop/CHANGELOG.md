# Changelog

Hermes Desktop source versions independently of the substrate. Its published
image version includes both versions because the image embeds both.

Which Hermes Agent release an image contains, and which substrate it was built
on, are recorded on the image itself as
`dev.pdparchitect.launcher.upstream.version` and `.substrate.version`. They are
not repeated here.

## [0.1.6]

- Replace the single Launcher viewer and container port with the `desktop`
  `kasmweb` interface required by application schema version 2.

## [0.1.5]

- Remove the unused product-version copy from the Launcher application
  document. The product `VERSION` file remains authoritative.

## [0.1.4]

- Copy Python dependencies into the Hermes virtual environment so removing the
  uv build cache cannot corrupt packages when Apple container applies the image
  layers.
- Remove redundant fixed-resolution metadata from the Launcher application
  definition so the viewer can determine its active desktop size.

## [0.1.3]

- Publish the Launcher application definition and artwork as an OCI artifact
  attached to the final multi-architecture image digest.

## [0.1.2]

- Separate the upstream Hermes Agent version from its Git release tag so image
  metadata reports version `0.19.0` while the source remains pinned to the
  verified `v2026.7.20` tag and commit.
- Record the upstream Git tag alongside its semantic version and revision.

## [0.1.1]

- Fixed the Hermes package installation so both the CLI and its Python package
  are available in the published image.
- Run Hermes as the session user when the desktop starts from a root-owned
  working directory, keeping its persistent state writable.
- Added a product self-test that verifies the Hermes CLI, Python package and
  persistent state directory before an image passes its smoke test.
- Avoid reinstalling operating-system dependencies while fetching the bundled
  Playwright browser.
- Record the upstream Hermes source URL in the image metadata.

## [0.1.0]

- Hermes Agent in a browser-accessible desktop.
- The first session opens `hermes setup`; later sessions open the Hermes TUI.
- Workspace and Hermes state persist across restarts.
- Hermes colour scheme across the desktop, panel, menus, and terminal.
