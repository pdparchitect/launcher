# Petbox

A desktop with residents. Each one is a small 8-bit creature that walks around
the screen, and each one is a Codex agent with its own directory, character,
memory and schedule. The sprite is the interface; `codex exec` is what is
actually happening.

```text
bases/desktop
  -> products/petbox/desktop
```

## The shape of it

A pet is a directory. That is the whole design, and everything else follows
from it:

```text
/workspace/pets/pip/
  AGENTS.md      who this pet is - Codex loads it before every turn
  pet.toml       species, colour, heartbeat, sandbox, model
  .agents/skills/ one directory per skill, each with a SKILL.md
  memory/        what it wants to still know next time
  journal/       dated notes on what it did
  inbox/         messages waiting for it; handled ones move to inbox/done/
  artifacts/     visible things it has left in the shared habitat
  .state/        runtime only - state.json, triggers, turn transcripts
```

Because a pet is a directory and not a process, the pen survives a session
restart, a pet is copied by `cp -r`, and a pet is deleted with `rm -r`. There
is no registry to keep in step.

## The four programs

| Program | Job |
| --- | --- |
| `petd` | one per session. Keeps a sprite running per pet, decides when each pet takes a turn, dispatches the inbox, caps concurrency. |
| `pet-run` | one turn. Builds a short prompt, runs `codex exec` with the pet's directory as the working root, records the transcript, and turns the closing line into speech. |
| `pet-sprite` | one pet on screen. Reads `.state/state.json`, never writes it. |
| `pet-habitat` | one per session. Watches every pet's `artifacts/` directory and renders its live objects on the desktop floor. |

They never talk to each other directly. The pet's directory is the only
channel: `pet-sprite` polls `state.json`, and anything that wants a turn to
happen writes a trigger file into `.state/triggers/`. That is why a click on a
sprite, a menu entry and a shell command all reach the supervisor the same way,
and why the supervisor can be killed and restarted at any point.

`petd` is started by `/etc/desktop/session.d/10-pets`, which the desktop base
runs from inside the session - the sprites need `DISPLAY`, so the entrypoint's
`startup.d` hook would be too early. That script restarts the supervisor if it
exits, which is what makes `petctl restart` work.

## What a turn is

```sh
codex exec --cd <pet directory> --skip-git-repo-check \
    --sandbox danger-full-access --json -o .state/last-message.txt -
```

The prompt on stdin is short - what woke the pet, and the reminder that the
last line of the reply is what it says out loud. Who the pet *is* comes from
`AGENTS.md`, which Codex already loads from the working root, so the prompt
does not repeat it.

The sandbox is the one uncomfortable default here. Codex implements
`workspace-write` and `read-only` with bubblewrap, which needs unprivileged
user namespaces — and a container does not have them under Docker's default
profile, so a pet asking for `workspace-write` fails before it runs its first
command. The boundary that actually exists is the container: a disposable
desktop with one workspace volume. So pets run with `danger-full-access` inside
it, and `petctl doctor` reports — for no tokens, via `codex sandbox /bin/true` —
whether this particular host can do better. Where it can, tightening a pet is a
`sandbox` key in its own `pet.toml`.

Turns are triggered by:

- **the heartbeat** in `pet.toml`, spread by a per-pet jitter so a full pen
  does not wake at once
- **a poke** - right-click the sprite, or `petctl poke`
- **a message** - `petctl tell`, or any file dropped into `inbox/`

At most two pets think at a time (`PETS_MAX_TURNS`), and `petctl pause` stops
all of it while leaving the sprites walking.

## The sprites

Each pet is an override-redirect X window shaped to the silhouette of its
sprite, so there is no rectangle around it and a click that misses the pet
lands on whatever is underneath. No compositor is involved, which matters: this
desktop is encoded frame by frame and shipped to a browser, so every composited
pixel would be paid for twice.

The art is text. A species is 16x16 characters, authored twice - `idle` and
`sleep` - in `petsprites.py`; the walk cycle and the blink are derived. Adding
a species is a paste into that file, and `make check` renders every frame to
prove it is still rectangular.

| Interaction | |
| --- | --- |
| click | say the last line again |
| double-click | open a Codex session as that pet |
| right-click | poke it |
| drag | move it; it drops back to the floor |

Pets sit above the windows, the way desktop pets have always worked. A pet you
want out of the way for a while is `petctl enable <name> --off`, which stops
its sprite as well as its turns; `petctl enable <name>` brings it back.

## Time of day

The habitat has four vector backgrounds selected from the desktop's local
clock:

| Period | Local time |
| --- | --- |
| dawn | 05:00–07:59 |
| day | 08:00–16:59 |
| dusk | 17:00–19:59 |
| night | 20:00–04:59 |

`pet-wallpaper` changes the root wallpaper only when the period or display
resolution changes. This keeps an idle remote desktop still while ensuring a
browser resize receives the right scaled background. `TZ` controls what
"local" means. `DESKTOP_WALLPAPER` still takes precedence when explicitly set,
and `PETS_WALLPAPER_PERIOD=dawn|day|dusk|night` fixes a period for previews.

## Artifacts

Pets have a `.agents/skills/leaving-artifacts/SKILL.md` that tells them when a
turn is worth making visible and how to use `petctl artifact leave`. The
resulting JSON file belongs to the pet under `artifacts/`; the habitat only
renders the validated protocol and never executes artifact content.

Six built-in kinds have their own tiny pixel art: notes, doodles, gifts,
trophies, warnings and found objects. Click one to read its card. Its
right-click menu can pin it or clear it away. Unpinned objects expire after a
kind-specific lifetime, and leaving a seventh object clears the oldest
unpinned one, so a lively pen does not become permanent clutter.

```sh
petctl artifact leave pip --kind doodle \
    --title "A suspicious diagram" \
    --message "I mapped the services that wake up at night."
petctl artifact list pip
petctl artifact pin pip <artifact-id>
petctl artifact remove pip <artifact-id>
petctl artifact clear pip          # leaves pinned things alone
petctl artifact clear pip --all
```

During its own turn `PET_NAME` supplies the name, so the skill uses
`petctl artifact leave` without repeating it.

## Getting started

The first session opens `codex-setup`, because nothing thinks until Codex can
sign in - device authorisation or an API key, kept in `/home/agent/.codex` on
its own volume. Until then the pen still runs: the pets wander, they just do
not think.

```sh
petctl new                  # interactive, also on the desktop menu
petctl list
petctl tell pip "have a look at what is in /workspace and tell me what you find"
petctl chat pip
petctl doctor
```

A new workspace volume is seeded with one pet, `pip`, so the desktop is
inhabited on first boot.

## Knobs

| | |
| --- | --- |
| `PETS_ROOT` | where the pen lives. Default `/workspace/pets`. |
| `PETS_MAX_TURNS` | how many pets may think at once. Default 2. |
| `PETS_SPRITE_SCALE` | pixel size of a sprite. Default 4, so a pet is 64px. |
| `PETS_WALLPAPER_PERIOD` | optionally fix dawn, day, dusk or night. |
| `CODEX_HOME` | Codex credentials and config. Default `/home/agent/.codex`. |

## Deliberately not here

- **No per-pet Codex session continuity.** Each turn starts fresh and a pet's
  continuity comes from files it can read and you can edit. Resuming sessions
  would make a pet's memory something only Codex can see.
- **No pet-to-pet messaging.** `petctl tell` is a person's tool. Pets writing
  into each other's inboxes is one line of code and an unbounded token bill, so
  it should be a deliberate choice rather than a default.
- **No window-walking.** Pets stay on the floor. Climbing onto window frames
  means tracking every window's geometry, at a frame rate, over VNC.
