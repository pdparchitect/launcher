# Changelog

Codex versions independently of the substrate. Its published image version
includes both versions because the image embeds both, and the substrate
version is also recorded on the image itself.

## [0.1.0]

- Pin Codex CLI 0.146.0 and open its terminal interface in the persistent
  workspace. Petbox packages the same CLI as the engine behind its pets; this
  product is Codex itself.
- Resolve Codex state from an explicit `CODEX_HOME` rather than from `HOME`, so
  credentials, configuration, history, and rollout sessions land on the
  declared volume even in an Apple fixed-mount session that runs as root.
- Leave first-run sign-in to Codex's own onboarding, which already offers the
  device-code path this desktop needs: the browser-callback flow returns to a
  loopback port inside the container and cannot complete from a browser on
  another machine.
- Take the product's palette from OpenAI's own Codex artwork: a `#24273A`
  slate-navy terminal on a periwinkle field, `#1D1F2E` chrome, white text, and
  a single `#C77DEB` orchid accent on the cursor, the selection, and the shared
  shell prompt's `@user` segment. The focused window keeps the base's white
  outline, which is the detail that artwork leads with.
- Recolour the window decorations, panel, and Chromium's GTK surfaces by
  mapping the base's colour vocabulary onto that palette role by role, rather
  than replacing those files, so later substrate changes still reach this
  product.
- Build the wallpaper as one diagonal periwinkle ramp with a soft highlight,
  using gradients rather than a blurred shape: SVG filters are the one thing a
  wallpaper loader may not implement, and artwork that silently disappears is
  worse than an effect never used.
- Report sign-in state in the panel's status slot, treating `OPENAI_API_KEY` as
  signed in.
