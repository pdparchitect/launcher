# Launcher agent guide

This guide is embedded in the installed Launcher binary. It describes the
commands available in that binary, so prefer it over assumptions about another
Launcher version.

Launcher installs and runs catalogue applications as isolated local containers.
It selects Docker or Apple `container` for the host. Use Launcher commands
instead of calling the container provider directly so instance state, mounts,
ports, and lifecycle remain consistent.

Running `launcher` without arguments in a terminal prints concise CLI help.
Use `launcher desktop` when the task explicitly requires the graphical Launcher
interface. Opening the packaged application from Finder starts that interface
directly.

## Discover the environment

Start by checking the binary, runtime, catalogue, and installed agents:

```sh
launcher version
launcher doctor
launcher catalog
launcher list
```

Use `launcher catalog --refresh` when the task requires the latest available
applications. Catalogue output contains the slug required by `create` and the
resolved container image that Launcher will use.

## Create an agent

Creation always requires an explicit catalogue application and a unique local
name:

```sh
launcher create --app SLUG_OR_ID NAME
```

Never guess or silently choose an application. Run `launcher catalog`, select
the application that matches the task, and pass its slug or ID with `--app`.
Use `--stopped` when the instance should be prepared without starting it.

`--image IMAGE` is an advanced override. It still requires `--app` because the
catalogue application defines the interfaces, mounts, environment, and resource
settings. Do not override the image unless the user or development workflow
requires it.

## Inspect and control installed agents

Most commands identify an installed agent by its unique name or 32-character
ID:

```sh
launcher list
launcher status NAME
launcher start NAME
launcher stop NAME
launcher open NAME
launcher viewer NAME
launcher preview --output PATH NAME
launcher logs NAME
launcher logs --follow NAME
```

`status` is the complete read-only inspection command. It reports the agent's
actual and desired state, image and container identity, creation time, managed
files and declared mounts, every local interface URL, managed network and
provider-reported IP addresses, and live CPU, memory, and uptime information
when available. Runtime inspection errors are shown inline so the remaining
stored details are still usable. Use `logs` separately for container output.

`start` also repairs a missing runtime container when the agent has a stored
runtime manifest. Launcher recreates the container with its installed image,
interfaces, mounts, network, and persistent data, then waits for any declared
health interface. It does not silently substitute a different catalogue image.

`open` opens the application's primary local interface. When no graphical
opener is available, it prints the local URL instead.

`viewer` opens the application in Launcher's specialized framed desktop window
when the installed build supports the desktop interface. On macOS, this viewer
also exposes native menus for opening the agent's files, renaming it, and
stopping it while closing the window.

`preview` saves one current image from a running agent's declared live-preview
interface:

```sh
launcher preview --output ./agent-preview.jpg NAME
launcher preview --output ./agent-preview.jpg --force NAME
```

The destination is never replaced unless `--force` is supplied. Launcher waits
for a preview that is still initializing, but does not guess a preview URL for
an application that has not declared one.

Stopping preserves the agent and its files. Starting resumes the same agent.

## Duplicate an agent

Duplicate an installed agent when the task requires an independent copy of its
persistent files and configuration:

```sh
launcher duplicate SOURCE NEW_NAME
launcher duplicate --start SOURCE NEW_NAME
```

The duplicate uses the source's exact image and catalogue application, but gets
a new identity, container, network, and ports. It is stopped by default. An
active source is stopped only while its persistent files are copied and is then
started again.

Duplication copies credentials, login sessions, private configuration, and
workspace contents stored in declared host mounts. Treat the duplicate as
sensitive. Applications using runtime-managed volumes cannot currently be
duplicated; Launcher rejects them instead of creating an incomplete copy.

## Execute commands inside an agent

The target agent must be running:

```sh
launcher exec NAME COMMAND [ARG...]
launcher exec NAME uname -a
launcher exec --tty NAME sh
```

`exec` runs the command directly without a host shell and streams standard
input, output, and error. Shell operators such as pipes, redirects, variables,
and command substitution are not interpreted unless a shell is invoked
explicitly:

```sh
launcher exec NAME sh -c 'ps aux | grep node'
```

Use `--tty` only for interactive terminal programs. For scripts and captured
output, omit it.

## Delete and clean up safely

Deletion is permanent and removes the installed agent together with its
Launcher-managed local files:

```sh
launcher delete --force NAME
```

Use `stop` when preservation is intended. Run `delete --force` only when the
user or task explicitly requires permanent removal.

`launcher cleanup` removes only old, unreferenced images tracked by Launcher;
it does not delete installed agents.

## Troubleshooting

Run `launcher doctor` first for runtime problems, then inspect the agent:

```sh
launcher status NAME
launcher logs NAME
```

Set `PDPARCHITECT_LAUNCHER_HOME` only when an isolated Launcher data directory
is intentionally required. Runtime selection is normally automatic; the
`PDPARCHITECT_LAUNCHER_RUNTIME` override is intended for development and is not
available for selecting Docker on macOS.

Run `launcher help` for the concise command list and `launcher guide` to print
this guide again.
