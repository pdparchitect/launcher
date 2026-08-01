# Changelog

All notable Launcher changes are documented here, following
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
[Semantic Versioning](https://semver.org/).

## [0.6.0] - 2026-08-01

### Changed

- Require every agent creation request to explicitly select a catalogue
  application; `launcher create` no longer silently defaults to Pantalk Ghost.
- Include each application's resolved container image in `launcher catalog`
  output.
- Print CLI help when Launcher is invoked without arguments from a terminal,
  while continuing to open the desktop when launched graphically. Use
  `launcher desktop` to open it explicitly from a terminal.

### Added

- Add `launcher guide`, an embedded Markdown tutorial that gives automated
  agents a self-contained reference for safely using the installed Launcher.
- Add native macOS File-menu actions to agent viewer windows for opening the
  agent's host-mounted files, renaming the agent, stopping the agent and
  closing its window, or closing the window without stopping the agent.
- Add `launcher exec [--tty] NAME COMMAND [ARG...]` to execute commands inside
  running agents through Docker or Apple `container`, with streaming standard
  input, output, and error.

## [0.5.1] - 2026-08-01

### Fixed

- Deliver arrow keys to agents as cursor keys rather than as their keypad
  twins, in viewer windows on macOS. macOS tags the arrow keys with its
  numeric-pad modifier flag, and WebKit passes that through as a numeric-pad
  key location where Chrome and Firefox report a standard one. Every layer
  below then behaved correctly on a wrong value: the agent's VNC client read
  the location and sent `KP_Up` instead of `Up`, terminals reported a keypad
  key under the kitty keyboard protocol, and full-screen applications that
  negotiate it - vim among them - inserted the key's raw code instead of
  moving the cursor. The viewer window now corrects the location before the
  agent's interface reads it.

## [0.5.0] - 2026-08-01

### Added

- Replace the Home screen with Explore, which promotes one of the catalogue's
  first three agents at a time. Its artwork becomes the page backdrop, and a
  single control moves to the next promotion, each with its description, tags,
  and an install action. A grid of the first nine agents sits below it, and
  the Marketplace carries the rest.

### Changed

- Rename the Agents screen to Library, and move runtime and Launcher version
  information into a system panel in its sidebar. The recent agents, recent
  activity, and system overview sections are gone from the front page.
- List every matching agent in the Library instead of paginating it.

### Fixed

- Present the catalogue in the order publisher feeds declare their
  applications, rather than sorting it by name. The registry keeps that order
  through its cache, so a restored catalogue matches a refreshed one.

## [0.4.14] - 2026-07-31

### Added

- Publish native Linux ARM64 desktop packages alongside the existing x86-64
  release package.
- Track images pulled by Launcher and remove unreferenced images after a
  seven-day grace period through daily and manual cleanup. Cleanup targets
  exact tracked image IDs without pruning unrelated images or volumes.

### Changed

- Rename the Linux application build target to `make build-linux` for symmetry
  with `make build-macos`.

## [0.4.13] - 2026-07-31

### Added

- Poll each running agent's declared `notifications` interface and forward its
  cursor-based events to native desktop notifications without replaying noisy
  failures.

## [0.4.12] - 2026-07-31

### Fixed

- Isolate every agent on its own runtime network so containers cannot reach
  another agent's unauthenticated services. Existing agents migrate from the
  shared default network the next time they start, and their managed network
  is removed when they are deleted.

## [0.4.11] - 2026-07-31

### Fixed

- Create the native macOS Help menu when Wails does not provide one so the
  GitHub issue-reporting action is visible.

## [0.4.10] - 2026-07-31

### Added

- Add a native macOS Help-menu action for reporting bugs and requesting
  improvements through GitHub Issues.
- Let application mounts select `host` or `volume` storage. Docker and Apple
  `container` supply persistent runtime-managed volumes while continuing to
  expose ordinary workspaces and configuration as host bind mounts. This lets
  services such as PostgreSQL retain Linux ownership semantics on macOS.

### Fixed

- Place the native macOS update action in its own section at the top of the
  application menu instead of between the standard Hide commands.
- Delete runtime-native application volumes with their owning agent and clean
  them up when initial container creation fails, while preserving them across
  image updates.

## [0.4.9] - 2026-07-31

### Changed

- Make stopping an agent feel immediate by optimistically showing it offline,
  shielding that state from stale status polls, and restoring its running state
  when the stop request fails.
- Refresh live agent previews once per minute and keep the previous screenshot
  visible until its replacement has fully loaded and decoded.

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
