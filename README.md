# Launcher

Launcher is an experimental, consumer-friendly library for local AI agents. It
is intended to feel closer to a game launcher than a virtual machine or
container manager.

The project now includes the Phase 1 command-line interface and the first
Phase 2 local graphical interface. Both are built into the same Go executable
and use the same application service. Docker and Apple `container` are
implementation details hidden behind the runtime layer.

Pantalk Ghost and Buzznode are the first built-in catalogue applications.
Additional applications, such as Hermes Agent and OpenClaw, can be represented
by independent catalogue manifests as their packaging is developed.

## Built-in catalogue

The catalogue is embedded in the executable. Each application has a directory
under `internal/catalog/manifests/<slug>/` containing `manifest.json` and its
icon, cover, and screenshots. Every manifest has a permanent UUID identity and
a separate human-readable slug. The directory name is an organizational
convention and does not define either value. A manifest owns both its container
configuration and its user-facing presentation:

```json
{
  "id": "370a2228-322d-4089-846b-62fb8c15d154",
  "slug": "pantalk-ghost",
  "name": "Pantalk Ghost",
  "publisher": "Pantalk",
  "description": "A secure local desktop environment...",
  "tags": ["COMMUNICATION", "SECURE", "DESKTOP"],
  "media": {
    "icon": "pantalk-ghost/icon.svg",
    "cover": "pantalk-ghost/screenshot.png",
    "screenshots": [
      {
        "source": "pantalk-ghost/screenshot.png",
        "alt": "Pantalk Ghost in the Agent Launcher"
      }
    ]
  },
  "image": "ghcr.io/pantalk/ghost:0.0.8"
}
```

The UUID is stored in installed agent records and must never be changed. The
slug is used by people in commands such as `launcher create --app
pantalk-ghost`, and may be renamed while retaining the same UUID.

All media paths are validated as safe relative image paths, and every
referenced file must exist at build time. Adding another valid manifest and
its assets automatically makes it available to the CLI catalogue, HTTP API,
and Marketplace.

Raster catalogue artwork is tracked with Git LFS through the local
`.gitattributes`.
Install LFS once before cloning or contributing images:

```bash
git lfs install
git lfs pull
```

CI checks out LFS objects before testing and cross-compiling. The catalogue
tests also verify the embedded files are real PNG data, preventing an LFS
pointer file from being compiled into a release accidentally.

## Current commands

```text
launcher desktop
launcher serve
launcher catalog
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

Run the supplied Agent Launcher interface with:

```bash
make web
```

The interface is plain HTML, CSS, and standards-based Web Components embedded
in the Go executable. It has no JavaScript package dependencies, generated
template runtime, or frontend compilation step. Editing files under
`internal/httpapi/web/` and restarting `make web` is enough to test a change.
The same embedded files are served by development builds and packaged
executables.

This runs directly from source, prints the URL, and does not try to launch a
browser, making it suitable for a VS Code server or Kasm session. Forward or
open the printed `http://127.0.0.1:16900` address. To use another local port:

```bash
make web WEB_LISTEN=127.0.0.1:17000
```

Use `make web-open` when developing on a machine with a local desktop browser.
The equivalent built-binary command is `./dist/launcher serve --no-open`.

## Desktop application

Launcher uses Wails to host the same embedded interface as a frameless native
desktop window. The header is the window chrome: drag it to move the window,
double-click it to maximise or restore, or use its minimise, maximise, and
close buttons. Browser development continues to use the same files; the window
controls become active only when the Wails runtime is present.

On Ubuntu, install the Wails development libraries once:

```bash
sudo apt-get install build-essential libgtk-3-dev libwebkit2gtk-4.1-dev
```

Then run or build the desktop application:

```bash
make desktop
make build-desktop
```

On macOS, build the complete universal application bundle with:

```bash
make build-macos
open "build/bin/Agent Launcher.app"
```

This target must run on macOS because Wails links Apple's native Cocoa and
WebKit frameworks. The project's multi-stage `Build` workflow first checks the
code, then builds Linux and macOS desktop applications in parallel on
GitHub-hosted runners. It publishes a Linux x86-64 archive and a universal
Intel and Apple silicon macOS ZIP. The macOS application is ad-hoc signed but
not Developer ID signed or notarized, so macOS may require Control-clicking the
application and selecting **Open** the first time.

The packaged application remains a command-line tool:

```bash
"Agent Launcher.app/Contents/MacOS/launcher" doctor
```

The desktop-enabled executable remains a complete command-line program.
Opening it without arguments launches the window, while commands such as
`launcher-desktop doctor` and `launcher-desktop list` run directly in the
terminal without starting Wails.

`launcher create` uses the `pantalk-ghost` catalogue entry and starts the new
agent by default. The more explicit form is:

```bash
launcher create --app pantalk-ghost Ada
```

Use `--stopped`, `--image`, or `--port` when needed.

## Try it

```bash
cd incubator/pdparchitect/launcher
make check
make build

./dist/launcher doctor
./dist/launcher catalog
./dist/launcher create --image pantalk/ghost:local Ada
./dist/launcher list
./dist/launcher open Ada
```

The `open` command always prints the local URL. If no desktop browser opener is
available, as is common in a VS Code server, printing the URL is treated as a
successful fallback.

## Runtime selection

Launcher selects a runtime automatically:

- On an Apple silicon Mac, it prefers Apple `container` and falls back to
  Docker.
- On Linux, Windows, and Intel Macs, it uses Docker.

Override the selection when testing:

```bash
PDPARCHITECT_LAUNCHER_RUNTIME=container ./dist/launcher doctor
PDPARCHITECT_LAUNCHER_RUNTIME=docker ./dist/launcher doctor
```

If the selected runtime is missing, `launcher doctor` offers to open its
official installation page. It never downloads or executes an installer
silently. If Apple `container` is installed but its service is stopped,
`doctor` offers to start the service. Use `--no-prompt` in scripts and CI.
On macOS, Launcher checks both the inherited `PATH` and the standard
`/usr/local/bin/container` and `/opt/homebrew/bin/container` locations so the
packaged application works when opened from Finder. A successful `doctor`
report includes the exact runtime executable it resolved.

## Persistent files

Override the data folder with:

```bash
PDPARCHITECT_LAUNCHER_HOME=/tmp/launcher-test ./dist/launcher list
```

Defaults are:

- macOS: `~/Library/Application Support/Launcher`
- Linux: `$XDG_DATA_HOME/launcher`, or `~/.local/share/launcher`
- Windows: `%LOCALAPPDATA%\Launcher`

Each installed agent receives an `instance.json` file and private bind-mounted
directories under `agents/<id>/`. Deleting an agent removes its managed
container and this complete folder, so deletion requires `--force`.

Launcher labels every created container and verifies that label before
deletion. It will not delete a container owned by another application.

## Build targets

```bash
make check
make build
make build-desktop
make build-macos
make build-all
```

`make build-all` produces Linux and macOS command-line binaries for AMD64 and
ARM64 without requiring macOS. `make build-macos` produces the native universal
`.app` on macOS. Developer ID signing, notarization, and a polished `.dmg`
remain later release steps.

## Structure

```text
main.go                    composition and process exit
cli/                       commands, prompting, and browser opening
internal/agent/            application lifecycle
internal/catalog/          self-contained embedded application manifests
internal/config/           platform data-folder selection
internal/desktop/           Wails frameless desktop host
internal/domain/           persisted instance model
internal/httpapi/           loopback API and embedded Launcher interface
internal/runtime/          Docker and Apple container adapters
internal/store/            file-backed instance library
internal/webapp/            local server lifecycle and session setup
```

The graphical interface uses the supplied game-launcher design with live
catalogue and instance data. It can install Ghost or Buzznode, list instances,
show live CPU, memory, and uptime data, start and stop them, and open a running
agent desktop.

Browser mode binds the HTTP API to loopback and requires a random per-process
session token. Desktop mode passes the same protected handler directly to the
Wails asset server without opening a listening port.
