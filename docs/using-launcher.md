# Using Launcher

Launcher provides a desktop application and command-line interface for
installing and managing local AI agents. Both interfaces use the same local
service and the same agent library.

## Requirements

Published releases currently support:

- Apple silicon Macs running macOS 26, using Apple `container`
- Linux x86-64 systems, using Docker

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

On Linux, extract the `Linux-x86_64.tar.gz` archive and run `launcher`:

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

Opening the packaged application without arguments starts the desktop
interface. It provides:

- A marketplace of available agents
- Installation and local instance management
- Start, stop, update, and delete controls
- CPU, memory, status, and uptime information
- Access to a running agent's local interface

Launcher can also serve the same interface in a browser:

```bash
launcher serve
```

The browser server listens only on loopback and protects its API with a random
per-process session token.

## Command-line interface

```text
launcher desktop
launcher serve
launcher catalog [--refresh]
launcher create NAME
launcher list
launcher status NAME
launcher start NAME
launcher stop NAME
launcher open NAME
launcher logs [--follow] NAME
launcher delete --force NAME
launcher doctor
```

For example:

```bash
launcher doctor
launcher catalog
launcher create --app pantalk-ghost Ada
launcher list
launcher open Ada
```

`launcher open` prints the local URL when the system cannot open a desktop
browser.

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

## Launcher updates

Launcher checks for a newer stable release in the background. A new version
appears in a banner throughout the application and in the Home system
overview. **View release** opens the matching GitHub release page so you can
review its notes and download the package for your platform.

The banner can be dismissed for that version. The Home overview continues to
show that an update is available, and a later version displays a new banner.

Checks are cached for 24 hours, use conditional requests, and never block
startup. Development builds report their version as `dev` and do not check for
updates. Launcher does not download or install application updates
automatically.
