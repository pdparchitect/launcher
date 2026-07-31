# Developing Launcher

Launcher is one Go executable. Its command-line interface, browser interface,
and desktop application use the shared service in `internal/agent`. Docker and
Apple `container` are hidden behind the runtime abstraction in
`internal/runtime`.

## Set up the repository

Raster application artwork uses Git LFS:

```bash
git lfs install
git lfs pull
```

Run the complete validation suite before and after a change:

```bash
make check
```

## Browser development

Run the embedded web interface directly from source:

```bash
make web
```

This prints the local URL without trying to launch a browser, which works well
in remote development environments. Use another loopback port when needed:

```bash
make web WEB_LISTEN=127.0.0.1:17000
```

Use `make web-open` to open a local desktop browser.

The interface uses plain HTML, CSS, and standards-based Web Components under
`internal/httpapi/web/`. It has no JavaScript package dependencies, generated
template runtime, or frontend build step. Restart `make web` after editing an
embedded file.

## Desktop development

Launcher uses Wails to host the embedded interface. Linux uses its native GTK
or window-manager title bar. On macOS, a statically linked SwiftUI shell places
Wails' `WKWebView` inside a native `NavigationSplitView`. Wails owns the
process, asset scheme, and JavaScript runtime. SwiftUI supplies the system
sidebar, toolbar, and background extension effect.

Install the Linux development libraries on Ubuntu:

```bash
sudo apt-get install build-essential libgtk-3-dev libwebkit2gtk-4.1-dev
```

Then run or build the desktop application:

```bash
make desktop
make build-linux
```

Build and open the native Apple silicon application on macOS:

```bash
make build-macos
open "build/bin/Launcher.app"
```

The native application requires macOS 26. The macOS target must run on macOS
because Wails links Apple's Cocoa and WebKit frameworks.

The Swift package under `macos/` compiles as a static library before Wails
links the application. The resulting bundle contains one `launcher` executable
and runs one main process. It does not use a helper executable or loopback web
server.

The desktop targets run Go through `scripts/with-go-module-patches.sh`. Patch
sets are discovered from `patches/`. Each set contains:

- A `module` file
- An optional `goos` filter with one operating system per line
- Numbered `.patch` files applied in filename order

The runner copies each matching pinned module into a temporary directory,
applies its patches, and uses temporary Go module replacements without
modifying the module cache.

## Build targets

```bash
make check
make build
make build-linux
make build-macos
make build-all
```

`make build-all` creates Linux command-line binaries for AMD64 and ARM64 plus a
macOS ARM64 command-line binary without requiring macOS.

`make build-linux` creates the Linux desktop executable for the build host's
architecture. Release automation runs it on native AMD64 and ARM64 Linux
runners because Wails links the host GTK and WebKitGTK libraries through cgo.

`make build-macos` creates an ad-hoc signed native Apple silicon application
for local development. The release workflow replaces that signature with a
Developer ID signature and notarizes the application.

Test a source build with:

```bash
make check
make build

./dist/launcher doctor
./dist/launcher catalog
./dist/launcher create --image pantalk/ghost:local Ada
./dist/launcher list
./dist/launcher open Ada
```

## Source structure

```text
main.go                    composition and process exit
cli/                       commands, prompting, and browser opening
internal/agent/            application lifecycle
internal/catalog/          OCI discovery, validation, and application cache
internal/config/           platform data-folder selection
internal/desktop/          Wails native desktop host
internal/domain/           persisted instance model
internal/httpapi/          loopback API and embedded Launcher interface
internal/imagecache/       persisted runtime image ownership and retention ledger
internal/runtime/          Docker and Apple container adapters
internal/store/            file-backed instance library
internal/updatecheck/      cached stable Launcher release checks
internal/webapp/           local server lifecycle and session setup
```

Desktop mode passes the protected HTTP handler directly to the Wails asset
server without opening a listening port. Browser mode binds the API to loopback
and requires a random per-process session token.

## Related development guides

- [Application registry](application-registry.md)
- [Container image sources](../images/README.md)
- [Releasing Launcher](../RELEASES.md)
