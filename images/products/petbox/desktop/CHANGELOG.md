# Changelog

Petbox source versions independently of the substrate. Its published image
version includes both versions because the image embeds both, and the
substrate version is also recorded on the image itself.

## [0.3.4]

- Update to Launcher desktop substrate 0.1.5 so Chromium retains software
  rendering when the VM has no GPU.

## [0.3.3]

- Update to Launcher desktop substrate 0.1.4 so unprivileged agent tools can
  authenticate to the X display when fixed Apple mounts require a root desktop
  session.

## [0.3.2]

- Replace the remaining Codex Pets terminal artwork with a shared Petbox
  banner used by both the sign-in and welcome flows.
- Refresh the Launcher screenshot with the current Petbox branding.

## [0.3.1]

- Expose the shared desktop screenshot endpoint as the Launcher `preview`
  interface.

## [0.3.0]

- Rename the product from Codex Pets to Petbox. The Launcher application slug
  becomes `petbox`, the build target and published image become
  `petbox-desktop` and `launcher-image-petbox-desktop`, and the habitat
  wallpapers move to `/usr/share/backgrounds/petbox`. The application UUID is
  unchanged, so installed agents keep their identity.

## [0.2.4]

- Store pet skills in Codex's discoverable `.agents/skills/` directory.
- Add the required name and description metadata to built-in skills.

## [0.2.3]

- Replace the single Launcher viewer and container port with the `desktop`
  `kasmweb` interface required by application schema version 2.

## [0.2.2]

- Remove redundant fixed-resolution metadata from the Launcher application
  definition so the viewer can determine its active desktop size.
- Remove the unused product-version copy from the Launcher application
  document. The product `VERSION` file remains authoritative.

## [0.2.1]

- Publish the Launcher application definition and artwork as an OCI artifact
  attached to the final multi-architecture image digest.

## [0.2.0]

- Pets can leave notes, doodles, gifts, trophies, warnings and found objects
  as visible pixel-art artifacts on the desktop.
- Every pet owns a `leaving-artifacts` skill and an `artifacts/` directory;
  existing pets gain the missing skill without overwriting anything they have
  changed.
- Artifacts open into readable cards, can be pinned or cleared from their
  right-click menu, expire automatically and are capped per pet.
- `petctl artifact` exposes the same leave, list, pin and cleanup lifecycle to
  agents and people.
- The habitat follows local time through distinct dawn, day, dusk and night
  pixel-art wallpapers, while preserving explicit wallpaper overrides.

## [0.1.0]

- Codex CLI pinned to 0.146.0, with `codex-setup` for device or API-key
  sign-in and credentials kept on their own volume.
- The pen: a pet is a directory under `/workspace/pets` holding its own
  `AGENTS.md`, `pet.toml`, skills, memory, journal and inbox.
- `petctl` for making, watching, poking, messaging and talking to pets, and as
  the Openbox pipe menu that lists the live pen.
- `petd` supervises sprites and schedules turns: per-pet heartbeats with
  jitter, inbox dispatch, a concurrency cap, and a pen-wide pause.
- `pet-run` runs one turn through `codex exec` in the pet's own directory, and
  the pet's closing line becomes its speech bubble. Turns run unsandboxed
  inside the container, which is the boundary that exists; `petctl doctor`
  reports whether Codex' own sandbox modes work on the host.
- `pet-sprite` draws each pet as a shaped X11 window, animated from 16x16
  text-authored art in four species and six colourways.
- 8-bit product overlay: wallpaper, Openbox theme, tint2 panel and terminal
  palette, all drawn from one twelve-colour set.
- Seeds a starter pet, `pip`, into a brand-new workspace volume.
