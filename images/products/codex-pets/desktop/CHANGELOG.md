# Changelog

Codex Pets source versions independently of the substrate. Its published image
version includes both versions because the image embeds both, and the
substrate version is also recorded on the image itself.

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
