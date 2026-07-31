# Changelog

All notable Launcher changes are documented here, following
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning](https://semver.org/).

## [0.4.8] - 2026-07-31

### Changed

- Keep the deployment dialog open after a successful installation and make
  opening the newly started agent its focused primary action.

## [0.4.7] - 2026-07-30

### Added

- Show live agent thumbnails from a declared `preview` interface while an
  agent is running, refreshing every 15 seconds and falling back to its
  packaged catalogue screenshot whenever the preview is unavailable.

## [0.4.6] - 2026-07-30

### Added

- Add a native macOS application-menu action that checks immediately for a
  Launcher update, then changes into a download action when a newer release is
  available.

### Fixed

- Isolate invalid or unreadable stored agents so one damaged record cannot
  prevent Launcher from managing healthy agents, and surface every isolated
  record and its error in the application.

## [0.4.5] - 2026-07-30

### Changed

- **Breaking.** Replace the single application `viewer` and `containerPort`
  with schema-version-2 named `interfaces`. Launcher now resolves every
  declared `web`, `kasmweb`, `acp`, or `mcp` interface and publishes shared
  container ports only once.
- Store and return resolved interface URLs for installed agents instead of one
  global port and URL. The built-in viewer opens the agent's display interface
  and applies Kasm-specific browser settings only to `kasmweb`.

### Fixed

- Let macOS reveal the native traffic-light controls with its transient
  fullscreen title bar while keeping the viewer content-only at rest.

## [0.4.4] - 2026-07-30

### Fixed

- Keep agent viewers content-only in macOS fullscreen by collapsing the custom
  title strip and suppressing both sets of window chrome.
- Keep the available Launcher version on the Home screen clickable after the
  larger update banner is dismissed.
- Ignore unknown fields in application documents while continuing to validate
  their known runtime and media configuration, keeping catalogue parsing
  compatible with both old and newer publisher metadata.

## [0.4.3] - 2026-07-30

### Changed

- Remove application-declared desktop resolutions and environment-variable
  names. KasmVNC now supplies its own startup dimensions and resizes the remote
  desktop to the connected viewer.
- Make each product's `VERSION` file its single version source. Launcher
  application documents no longer duplicate a product version that the
  catalogue does not use.

### Fixed

- Force an application-catalogue refresh once per Launcher startup so newly
  published image updates are not hidden by the persisted 30-minute cache.
- Remove deleted agents from the interface immediately and prevent an
  overlapping stale status poll from temporarily restoring them.
- Let the dimmed area around an open dialog drag the native macOS window while
  keeping the dialog panel and its controls interactive.

## [0.4.2] - 2026-07-30

### Fixed

- Revalidate the cached Launcher update status once after every launch so
  recently published releases appear without waiting up to 24 hours.
- Keep the native macOS scrollbar sized to the current page when navigating
  from a longer page without restoring whole-window rubber-banding.

## [0.4.1] - 2026-07-30

### Added

- Show an explicit Marketplace loading state while Launcher requests the
  application catalogue, then distinguish an unavailable catalogue from one
  that is still loading.

### Fixed

- Limit image pulls to the platform Launcher will run so Apple container does
  not download and unpack both AMD64 and ARM64 variants on Apple silicon.

## [0.4.0] - 2026-07-30

### Changed

- Replace the centrally versioned catalogue bundle with OCI publisher feeds
  and independently cached, image-owned application artifacts.
- Derive every published application image from its OCI subject digest so
  Launcher metadata cannot drift from the final multi-architecture image.

## [0.3.1] - 2026-07-30

### Fixed

- Restore the native traffic-light controls in fullscreen agent windows so
  they can leave fullscreen without being closed.
- Prevent the macOS Launcher window from rubber-banding past the page bounds
  while preserving native scrollbar insets around its rounded corners.

## [0.3.0] - 2026-07-30

### Added

- Check GitHub for newer stable Launcher releases and show an update banner
  plus an update notice on the home screen when one is available.
- Cache successful update checks for 24 hours so opening Launcher does not
  repeatedly contact GitHub.

### Changed

- Simplify the project README and move usage, development, catalogue, and
  release details into dedicated documentation.

## [0.2.0] - 2026-07-30

### Added

- Publish the initial signed and notarized macOS arm64 application and Linux
  x86-64 desktop package.
