# Changelog

The shared substrate: `core/ubuntu`, `runtimes/node`, and `bases/desktop`.
These release together because each embeds its parent.

Product source versions independently. Published product image versions also
include the substrate version they embed. See each product's own CHANGELOG.

Pinned component versions live in the Dockerfiles and are recorded on the
images as labels. They are not repeated here.

## [0.1.9]

- Provide a Secret Service for the desktop session. Ubuntu implements
  `org.freedesktop.secrets` nowhere, so every libsecret client in every product
  either failed - Buzz Desktop reports `Platform secure storage failure` and
  loses its identity - or fell back to a plaintext store, as Chrome did.
  `desktop-keyring` starts `gnome-keyring` with an unlocked default collection
  before any session program runs, which is also what keeps an activated
  password-less daemon from blocking the first caller on an unlock dialog.
- Give shells started outside the session the desktop's D-Bus address, so
  `secret-tool`, agent CLIs, and any other libsecret or libnotify client reach
  the session's services rather than autolaunching a private bus. `notify-send`
  needed its own adapter for this; nothing else does now.
- Assert the keyring in the live desktop smoke test: a secret stored, read
  back, and cleared from a login shell holding no bus address, under a timeout
  that fails on a prompt instead of hanging on one.

## [0.1.8]

- Route `notify-send` from container-exec shells and privilege-dropped agent
  processes to the desktop's actual D-Bus session instead of allowing an
  isolated session bus to be autolaunched.
- Exercise notifications as plain root and agent callers in the live desktop
  smoke test, without injecting the bridge process's environment.

## [0.1.7]

- Replace the Python preview daemon with a small Go desktop bridge that shares
  the graphical D-Bus session, owns `org.freedesktop.Notifications`, and
  exposes bounded in-memory notifications, health, and preview capture over
  port 6902.
- Install `notify-send` for command-line agents and start the entire Openbox
  session under one explicit D-Bus session.

## [0.1.6]

- Copy the active display cookie to root's default Xauthority location before
  launching Chromium in Apple fixed-mount sessions, so Chromium's renderer
  subprocesses authenticate and paint the browser surface.

## [0.1.5]

- Keep Chromium's software rasterizer available when the container has no GPU,
  so its windows remain usable in the Apple Linux VM.

## [0.1.4]

- Let unprivileged product processes authenticate to the KasmVNC display when
  Apple fixed-ownership mounts require the desktop session itself to run as
  root.

## [0.1.3]

- Preserve host-managed ownership after detecting fixed-ownership mounts, so
  products can persist XDG configuration and data directories under Apple
  `container` without aborting desktop startup on a rejected `chown`.

## [0.1.2]

- Add a lightweight desktop preview service on port 6902 that captures the
  active X display as a short-lived JPEG at `/preview.jpg`, independently from
  the interactive KasmVNC service.

## [0.1.1]

- Let KasmVNC select its initial framebuffer and resize it to the connected
  viewer instead of configuring a fixed desktop resolution.

## [0.1.0]

- Ubuntu 24.04 foundation with operating-system tooling only; no language
  runtime.
- Node.js runtime layer, available to images that need it.
- Browser-accessible Openbox desktop: window manager, panel, optional tiling,
  terminal, file manager, and a browser with matching native theming.
- Products customise the desktop through an `overlay/` directory, with drop-in
  hooks for the wallpaper, panel status, harness, terminal palette, and startup
  services.
