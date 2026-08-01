# Changelog

Claude Code versions independently of the substrate. Its published image
version includes both versions because the image embeds both, and the
substrate version is also recorded on the image itself.

## [0.2.0]

- Answer the questions this desktop has already answered, from a startup hook
  that runs before the session: onboarding and appearance, the trust prompt for
  `/workspace` - the only directory this agent can reach - and a default
  permission mode of `acceptEdits`, so an agent installed to work unattended is
  not stopped on every edit. Each value is written only when absent, and
  `CLAUDE_PERMISSION_MODE`, `CLAUDE_THEME` and `CLAUDE_TRUST_WORKSPACE`
  override them.
- Stop short of `bypassPermissions`. Claude Code refuses that mode when it runs
  with root privileges, and an Apple fixed-mount session runs the whole desktop
  as root, so it would work under Docker and fail on the platform Launcher
  targets first.
- Retire the `claude` wrapper that seeded appearance and onboarding. The
  startup hook does that work before the session, with the ownership handling
  a fixed-mount session needs, so the wrapper was a second place doing one job.
- Assert the running container's own configuration in the product selftest
  rather than only its sources, so a hook that silently stops writing fails the
  smoke test.

  Existing agents keep the configuration on their state volume; these defaults
  apply to a new one.

## [0.1.0]

- Pin Claude Code 2.1.220 and open its terminal interface in the persistent
  workspace.
- Keep the whole of Claude Code's state on one declared volume. It writes
  `~/.claude.json` beside `~/.claude`, so mounting only the directory would
  re-onboard on every container replacement and drop the configured MCP
  servers; `CLAUDE_CONFIG_DIR` moves both below `/home/agent/.claude`.
- Disable the CLI's own updater. The image is the unit Launcher updates, and a
  self-update would replace the version its labels record.
- Seed the light appearance and the onboarding it belongs to on first run, so
  the session opens on the workspace trust check rather than on a theme
  question the desktop has already answered. A user's own `settings.json` is
  left untouched.
- Make this the one light desktop in the catalogue. Anthropic's surface is
  paper, so the ground is `#F0EEE6`, the terminal canvas `#FAF9F5`, and the
  text ink - with `#D97757` clay on the cursor, the focused window outline, the
  selection, and the shared shell prompt's `@user` segment. The ANSI palette is
  re-tuned for a light ground, where a bright colour gains contrast by getting
  deeper rather than paler.
- Recolour the window decorations, panel, root menu, and Chromium's GTK
  surfaces by mapping the base's colour vocabulary onto that palette role by
  role, rather than replacing those files, so later substrate changes still
  reach this product.
- Compose the wallpaper from flat shapes cropped by the screen edges - manilla,
  kraft, and an oversized clay mark - placed in the margins a full-size
  terminal leaves visible.
- Report sign-in state in the panel's status slot, treating `ANTHROPIC_API_KEY`
  as signed in.
