# Changelog

OpenClaw Desktop source versions independently of the substrate. Its published
image version includes both versions because the image embeds both.

Which OpenClaw release an image contains, and which substrate it was built on,
are recorded on the image itself as
`dev.pdparchitect.launcher.upstream.version` and `.substrate.version`. They are
not repeated here.

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
