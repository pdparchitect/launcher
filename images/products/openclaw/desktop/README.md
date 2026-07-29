# OpenClaw Desktop

OpenClaw is a personal assistant that answers on the messaging channels you
already use, and its own interface is the Gateway Control UI - a web
application the gateway serves from its own port. That single fact shapes this
image: the product is a browser window, not a terminal, and the desktop exists
to host it.

```text
bases/desktop
  -> products/openclaw/desktop
```

## The shape of it

Three things run, and only the first is OpenClaw's:

| Program | Job |
| --- | --- |
| `openclaw gateway` | the product. Channels, sessions, tools, and the Control UI, all on `127.0.0.1:18789`. |
| `openclaw-gateway-supervise` | starts the gateway when there is a configuration to start it with, and restarts it when it exits. |
| `openclaw-control-ui` | resolves the Control UI's address and opens it in a Chromium app window. |

The supervisor is the piece that would not exist on a normal host. OpenClaw
installs a systemd user service during onboarding; there is no init system in
this container, so `--no-install-daemon` is passed to onboarding and this image
owns the service instead. The supervisor is started from
`/etc/desktop/startup.d/10-openclaw-gateway`, before X exists, because a daemon
with no window has no reason to wait for a session.

It **waits** for `~/.openclaw/openclaw.json` rather than failing without it.
The gateway refuses to start until onboarding has written that file, and
onboarding happens inside the session, minutes later. Because the supervisor
polls, the ordering never has to be arranged: complete onboarding whenever, and
the gateway comes up three seconds later. Restarting the gateway is the same
mechanism - `openclaw gateway stop` returns control to the loop, which starts
it again.

`/etc/desktop/session.d/20-openclaw-control-ui` is the other half. It runs
inside the session, waits for the gateway's port to answer, and opens the
Control UI window once. On a desktop that has never been onboarded nothing
answers, no window opens, and the welcome terminal - which is running the
wizard - is the whole interface.

## First run

The session opens with `openclaw onboard --mode local --no-install-daemon`.
`--mode local` puts the gateway here rather than pointing this desktop at a
remote one; `--no-install-daemon` keeps the wizard from trying to talk to
systemd.

Onboarding establishes a model provider first and everything else after, so it
needs either an API key or a provider subscription to sign in with. When it
finishes, the gateway starts on its own, the Control UI window opens, and the
panel's status slot turns green.

Channels - WhatsApp, Telegram, Slack, Discord, Signal, and the rest - are
configured from the Control UI or from `openclaw channels` in a terminal. The
QR-code flows work in this desktop's terminal.

## The address

The Control UI is at `http://127.0.0.1:18789/` inside the container, but that
URL alone does not get you in: the gateway requires auth, and the token reaches
the browser as a `#token=` fragment that `openclaw dashboard` assembles and
hands to `xdg-open`. So `openclaw-control-ui` runs that command rather than
building an address itself, and shims `xdg-open` for the length of the call to
turn the browser window it would open into an app window.

The tokenized URL is deliberately not available any other way. `--no-open`
prints the URL **without** the token, and `--json` - which the documentation
describes as returning one - does not exist in the pinned release. Anything
that assembled the URL here would be reimplementing token resolution, base
paths and TLS, and putting a secret on a command line.

When the gateway is not ready the command opens nothing, and the fallback opens
the bare loopback URL so Control-Shift-G always puts something on screen. The
Control UI then asks for the token itself.

The port is **not** published by this image. The desktop is what is exposed, on
6901, and the gateway is reachable from inside it. Publishing 18789 puts an
authenticated control plane for an agent with shell access on the host network;
read OpenClaw's [exposure runbook](https://docs.openclaw.ai/gateway/security/exposure-runbook)
before doing it.

`OPENCLAW_CONTROL_PORT` is this image's variable, not OpenClaw's. The panel
status and the session's readiness probe use it because a TCP probe costs
nothing and starting Node every few seconds does not. Moving `gateway.port` in
`openclaw.json` means setting this to match.

## State

```text
/home/agent/.openclaw          configuration, credentials, sessions, agent workspace
/workspace                     the durable disk
/var/log/launcher-desktop/openclaw-gateway.log   what the supervisor and the gateway wrote
```

`/home/agent/.openclaw` is a volume, so a rebuilt image resumes onboarded and
signed in. Remove that volume, or run with `RUN_STATE=`, to retest first-run
behaviour.

## The desktop

`kasm-patch "OpenClaw Desktop"` rebrands the KasmVNC client. The wallpaper,
Openbox theme, tint2 panel, root menu and terminal palette all come from one
colour set, installed through `overlay/` over the base's defaults. Nothing in
`bases/desktop` knows this product exists.

The colour set is OpenClaw's own. These are the design tokens `openclaw.ai`
ships, dark theme, and they are named here so a future change can be checked
against the source rather than guessed at:

| Token | Value | Used for |
| --- | --- | --- |
| `ink-950` | `#101012` | the page: terminal and menu ground |
| `ink-900` | `#19191C` | surfaces: the panel and window titlebars |
| `ink-850` | `#202024` | elevated: the active task, separators |
| `ink-50` / `ink-300` / `ink-500` | `#EDEDED` / `#BCBCC4` / `#9A9AA2` | text, in three weights |
| coral | `#F5654A` / `#E05540` / `#B23A28` | the accent: focus, cursor, selection, the claw |
| sea | `#4FC8AE` / `#2FA48D` | the secondary accent: hover, and "gateway up" |

Three colours are **not** OpenClaw's, because the brand has none and a terminal
needs all three: the yellow, blue and magenta in the ANSI palette. They are
desaturated to sit with the ink greys rather than compete with the accents.
