# Changelog

Hermes Desktop source versions independently of the substrate. Its published
image version includes both versions because the image embeds both.

Which Hermes Agent release an image contains, and which substrate it was built
on, are recorded on the image itself as
`dev.pdparchitect.launcher.upstream.version` and `.substrate.version`. They are
not repeated here.

## [0.1.0]

- Hermes Agent in a browser-accessible desktop.
- The first session opens `hermes setup`; later sessions open the Hermes TUI.
- Workspace and Hermes state persist across restarts.
- Hermes colour scheme across the desktop, panel, menus, and terminal.
