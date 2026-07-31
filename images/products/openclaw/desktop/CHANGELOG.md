# Changelog

OpenClaw Desktop source versions independently of the substrate. Its published
image version includes both versions because the image embeds both.

Which OpenClaw release an image contains, and which substrate it was built on,
are recorded on the image itself as
`dev.pdparchitect.launcher.upstream.version` and `.substrate.version`. They are
not repeated here.

## [0.1.9]

- Update to Launcher desktop substrate 0.1.8 so `notify-send` reaches native
  notifications from OpenClaw and root container shells.

## [0.1.8]

- Update to Launcher desktop substrate 0.1.7 and declare its shared health and
  native-notification bridge interfaces.

## [0.1.7]

- Update the shared Launcher desktop base to `0.1.6`, allowing root-launched
  Chromium processes to authenticate to the X display and paint the OpenClaw
  Control UI in Apple fixed-mount sessions.

## [0.1.6]

- Update the shared Launcher desktop base to `0.1.5`, keeping Chromium usable
  through software rendering when the Apple Linux VM has no GPU render node.

## [0.1.5]

- Update the shared Launcher desktop base to `0.1.4`, allowing unprivileged
  OpenClaw tools to authenticate to the display when Apple fixed-ownership
  mounts require the desktop session itself to run as root.

## [0.1.4]

- Expose the shared desktop screenshot endpoint as the Launcher `preview`
  interface.

## [0.1.3]

- Replace the single Launcher viewer and container port with the `desktop`
  `kasmweb` interface required by application schema version 2.

## [0.1.2]

- Remove redundant fixed-resolution metadata from the Launcher application
  definition so the viewer can determine its active desktop size.
- Remove the unused product-version copy from the Launcher application
  document. The product `VERSION` file remains authoritative.

## [0.1.1]

- Publish the Launcher application definition and artwork as an OCI artifact
  attached to the final multi-architecture image digest.

## [0.1.0]

- OpenClaw in a browser-accessible desktop, installed from npm at a pinned
  version.
- The Gateway Control UI is the product's interface: the session opens it in
  its own window as soon as the gateway answers, and Control-Shift-G reopens
  it.
- `openclaw-gateway-supervise` waits for onboarding to write a configuration
  and then keeps the gateway running, which is what stands in for the systemd
  user service OpenClaw installs on a normal host.
- The first session opens `openclaw onboard --mode local --no-install-daemon`;
  later sessions open a greeting and a shell.
- The panel reports whether the gateway is listening.
- Configuration, credentials, sessions and the agent workspace persist in
  `/home/agent/.openclaw`; `/workspace` is the durable disk.
- Product overlay in OpenClaw's own colours - the design tokens openclaw.ai
  ships - across the wallpaper, Openbox theme, tint2 panel, root menu and
  terminal palette.
- `desktop-selftest` asserts the supervisor is alive, which a passing desktop
  smoke test does not.
