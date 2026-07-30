# Changelog

All notable Launcher changes are documented here, following
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning](https://semver.org/).

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
