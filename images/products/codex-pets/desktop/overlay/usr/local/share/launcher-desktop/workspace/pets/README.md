# The pen

Every directory here is a pet. It is created by `petctl new`, it is supervised
by `petd`, and it thinks with `codex exec`.

```
pip/
  AGENTS.md      who this pet is. Codex reads it before every turn.
  pet.toml       species, colour, how often it wakes up
  skills/        one directory per skill, each with a SKILL.md
  memory/        what it wants to still know next time
  journal/       dated notes on what it actually did
  inbox/         messages waiting for it; handled ones move to inbox/done/
  .state/        runtime only - safe to delete, regenerated
```

A pet wakes on its own heartbeat, when you poke it, and when a message lands in
its inbox. Between turns it wanders around the desktop.

| | |
| --- | --- |
| `petctl new` | make a pet, interactively |
| `petctl list` | who lives here and what they are doing |
| `petctl tell pip "..."` | leave a message in a pet's inbox |
| `petctl chat pip` | talk to a pet directly, as itself |
| `petctl poke pip` | wake one up now |
| `petctl pause` | stop every pet thinking, without stopping the sprites |
| `petctl doctor` | check Codex, the pen and every pet |

On the desktop itself: click a pet to hear its last line again, right-click to
poke it, double-click to talk to it, and drag it wherever you like.

To change what a pet is, edit its `AGENTS.md`. To change what it looks like or
how often it wakes, edit its `pet.toml`. Both are picked up within a couple of
seconds - nothing needs restarting.

Deleting a directory here removes the pet. There is no other registry.
