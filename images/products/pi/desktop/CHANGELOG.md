# Changelog

Pi Desktop versions independently of the substrate. Its published image
version includes both versions because the image embeds both, and the
substrate version is also recorded on the image itself.

## [0.1.0]

- Pin Pi 0.83.0 and its `fd` file-discovery dependency, then open its terminal
  interface in the persistent workspace without first-run tool downloads.
- Preserve Pi credentials, settings, packages, extensions, and sessions across
  restarts.
- Base the terminal and browser landing page on Pi's `#142433` terminal colour,
  while separating the desktop wallpaper with a gradient based on `#161d27`.
  Remap the shared shell prompt's `@user` colour to Pi's `#E8993A` orange, and
  remove the black client-padding gap without changing the inherited titlebar.
- Add live packaged-desktop captures plus official press-kit artwork to the
  Launcher application.
- Present Pi's own terminal ASCII logo in the browser surface, using the same
  minimal full-page treatment as the Pantalk image.
