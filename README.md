# Launcher

Launcher is a local-first home for AI agents. It makes installing and running
an agent feel more like using a game launcher than managing containers,
services, and virtual machines.

[![Download the latest release](https://img.shields.io/badge/Download-latest%20release-2ea44f?style=for-the-badge&logo=github)](https://github.com/pdparchitect/launcher/releases/latest)

> Launcher is experimental and evolving quickly.

<table>
  <tr>
    <td><img width="1808" height="1264" alt="Xnapper-2026-07-31-01 55 00" src="https://github.com/user-attachments/assets/1633f11c-ae15-4a9c-b9a5-530b3743ba21" /></td>
    <td><img width="1808" height="1264" alt="Xnapper-2026-07-31-01 51 13" src="https://github.com/user-attachments/assets/49c8bbc6-d26d-4c54-a131-cfba5d1e42cc" /></td>
    <td><img width="1808" height="1264" alt="Xnapper-2026-07-31-01 51 22" src="https://github.com/user-attachments/assets/c5273bec-a575-46da-b599-d72a17dceab1" /></td>
  </tr>
</table>

<table>
  <tr>
    <td><img width="1432" height="952" alt="Xnapper-2026-07-31-01 52 09" src="https://github.com/user-attachments/assets/4ad9fda5-5515-417f-a564-5af5dc5c9f47" /></td>
    <td><img width="1432" height="952" alt="Xnapper-2026-07-31-01 52 41" src="https://github.com/user-attachments/assets/4bbb25a2-0a47-4341-8068-6b2f28ebd689" /></td>
    <td><img width="1432" height="952" alt="Xnapper-2026-07-31-01 51 48" src="https://github.com/user-attachments/assets/4e70e53a-c90e-4437-967e-61458db34fe9" /></td>
  </tr>
</table>

## Why Launcher

- **Local control** - Run agents on your own computer without a hosted control
  plane.
- **A familiar experience** - Discover, install, launch, stop, and update
  agents from one desktop application.
- **Less infrastructure work** - Launcher handles the local container runtime
  and agent lifecycle behind a simple interface.
- **Useful visibility** - See agent status, CPU use, memory use, and uptime in
  one place.
- **Desktop and command line** - Use the graphical interface for everyday work
  and the CLI for automation or troubleshooting.
- **Update awareness** - See when a newer stable Launcher release is available
  without interrupting your work.
- **Resilient registry** - Browse cached applications even when one of their
  publishers is temporarily unavailable.

## What you can do

Launcher provides a marketplace of packaged AI applications and a private
library of the agents installed on your computer. From the same interface, you
can:

- Browse available applications
- Install multiple local agent instances
- Start, stop, update, rename, and delete agents
- Open an agent's local interface
- Inspect logs and runtime health

Each application controls its own experience and capabilities. Launcher
provides the consistent local installation and management layer around it.

## Supported platforms

Published releases currently support:

- Apple silicon Macs running macOS 26
- Linux x86-64 systems

Launcher uses Apple `container` on macOS and Docker on Linux. If the required
runtime is missing, the application guides you to the official installer.

## Get started

1. [Download the latest release](https://github.com/pdparchitect/launcher/releases/latest)
   for your platform.
2. Extract the archive and open Launcher.
3. Follow the runtime setup if prompted.
4. Browse the Marketplace and deploy an agent.

The macOS release is Developer ID signed and notarized by Apple. Every release
includes SHA-256 checksums for its downloadable packages.

## Documentation

- [Using Launcher](docs/using-launcher.md)
- [Developing Launcher](docs/development.md)
- [Application registry](docs/application-registry.md)
- [Container image sources](images/README.md)
- [Release notes](CHANGELOG.md)
- [Releasing Launcher](RELEASES.md)

Bug reports and ideas are welcome in
[GitHub Issues](https://github.com/pdparchitect/launcher/issues).
