# Using Launcher

Launcher provides a desktop application and command-line interface for
installing and managing local AI agents. Both interfaces use the same local
service and the same agent library.

## Requirements

Published releases currently support:

- Apple silicon Macs running macOS 26, using Apple `container`
- Linux x86-64 and ARM64 systems, using Docker

Launcher selects the runtime for the platform automatically. Intel Macs are not
supported because Apple `container` requires Apple silicon.

## Install

Download the package for your platform from the
[latest Launcher release](https://github.com/pdparchitect/launcher/releases/latest).
The release also includes `SHA256SUMS` for verifying the downloaded archive.

On macOS, extract the `macOS-arm64.zip` archive and open `Launcher.app`. The
application is Developer ID signed, notarized by Apple, and includes the
command-line interface inside its application bundle.

Run a command from the macOS bundle with:

```bash
"Launcher.app/Contents/MacOS/launcher" doctor
```

On Linux, extract the archive matching your architecture and run `launcher`.
Use `Linux-x86_64.tar.gz` on AMD64 systems or `Linux-arm64.tar.gz` on ARM64
systems. For example:

```bash
tar -xzf Agent-Launcher-*-Linux-x86_64.tar.gz
./launcher
```

## Runtime setup

Run the diagnostic command to check whether the platform runtime is ready:

```bash
launcher doctor
```

If the runtime is missing, Launcher offers to open its official installation
page. It never downloads or executes an installer silently. If Apple
`container` is installed but stopped, Launcher can start its service after
asking for confirmation.

The desktop application provides the same process through a guided setup
dialog. Use `--no-prompt` when running commands in scripts or CI.

## Desktop and browser interfaces

Opening the packaged application from Finder or an application launcher starts
the desktop interface. Running `launcher` without arguments in a terminal
prints CLI help instead; use `launcher desktop` to open the desktop explicitly
from a terminal. The desktop interface provides:

- An Explore screen promoting the catalogue's first three agents, one at a
  time, with the promoted agent's artwork behind the page, an install action,
  and a grid of the first nine agents
- A marketplace of available agents
- Installation and local instance management
- Start, stop, update, and delete controls
- A Library of the agents installed on this computer, with CPU, memory,
  status, and uptime information, and the runtime and Launcher version in its
  system panel
- Access to a running agent's local interface

Launcher can also serve the same interface in a browser:

```bash
launcher serve
```

The browser server listens only on loopback and protects its API with a random
per-process session token. That token is a same-origin protection rather than
a login, so the interface has no authentication of its own.

Running Launcher on a dedicated machine and using the web interface to manage
agents is a supported way to work. See
[Serving the web interface](web-interface.md) for the listen address rules,
running it as a system service, reaching agent interfaces remotely, and the
recommended zero trust proxy in front of it.

## Command-line interface

```text
launcher desktop
launcher serve
launcher catalog [--refresh]
launcher create --app SLUG_OR_ID NAME
launcher list
launcher status NAME
launcher start NAME
launcher stop NAME
launcher open NAME
launcher viewer NAME
launcher logs [--follow] NAME
launcher exec [--tty] NAME COMMAND [ARG...]
launcher delete --force NAME
launcher cleanup
launcher doctor
launcher guide
```

For example:

```bash
launcher doctor
launcher catalog
launcher create --app pantalk-ghost Ada
launcher list
launcher open Ada
launcher viewer Ada
launcher exec Ada uname -a
launcher exec --tty Ada bash
```

`launcher guide` prints a self-contained Markdown tutorial embedded in the
binary. It is intended for both people and automated agents that need to learn
the installed Launcher's discovery, creation, lifecycle, execution, and safety
conventions without relying on external documentation.

`launcher open` prints the local URL when the system cannot open a desktop
browser.

`launcher viewer` opens the agent in Launcher's specialized framed desktop
window. On macOS, that window includes the native agent-management menus.

`launcher catalog` includes each application's slug, name, publisher, and
resolved container image. Use the slug with `launcher create --app`.

`launcher exec` runs the command directly without a host shell and streams its
standard input, output, and error through the selected container provider. Use
`--tty` for interactive terminal programs. To use pipes, redirects, or other
shell syntax inside the agent, invoke a shell explicitly, for example:

```bash
launcher exec Ada sh -c 'ps aux | grep node'
```

## Runtime selection

Launcher uses Apple `container` on Apple silicon Macs and Docker on Linux and
Windows. Docker and Podman are not fallbacks on macOS.

Developers can override runtime detection while testing:

```bash
PDPARCHITECT_LAUNCHER_RUNTIME=container launcher doctor
PDPARCHITECT_LAUNCHER_RUNTIME=docker launcher doctor
```

The Docker override is available only on non-macOS platforms.

## Data and deletion safety

Launcher stores its data in:

- macOS: `~/Library/Application Support/Launcher`
- Linux: `$XDG_DATA_HOME/launcher`, or `~/.local/share/launcher`
- Windows: `%LOCALAPPDATA%\Launcher`

Override the location when testing:

```bash
PDPARCHITECT_LAUNCHER_HOME=/tmp/launcher-test launcher list
```

Each installed agent has an `instance.json` file and private bind-mounted
directories under `agents/<id>/`. Deleting an agent removes its managed
container and complete data folder, so command-line deletion requires
`--force`.

Launcher labels every container it creates and verifies that label before
deletion. It will not delete a container owned by another application.

Launcher also records the exact runtime image IDs it pulls. Once per day, it
removes tracked images that no installed agent references and that have been
unused for at least seven days. It never runs a global runtime prune, so images
from other applications and all runtime volumes are left untouched. Run
`launcher cleanup` to apply the same safe cleanup policy manually.

## Launcher updates

Launcher checks for a newer stable release in the background. A new version
appears in a banner throughout the application and in the Library system
panel. **View release** opens the matching GitHub release page so you can
review its notes and download the package for your platform.

The banner can be dismissed for that version. The system panel continues to
show that an update is available, and a later version displays a new banner.

The last successful result is cached for immediate display and offline use.
Launcher revalidates that result with a conditional request once after each
launch, then caches checks for 24 hours while it remains open. Checks never
block startup. Development builds report their version as `dev` and do not
check for updates. Launcher does not download or install application updates
automatically.
