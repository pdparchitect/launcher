# Launcher

Launcher is a local-first home for AI agents. Each agent you install gets a
name and a space of its own, so keeping several of them feels more like using
a game launcher than managing infrastructure.

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

## Who Launcher is for

Launcher is for people who like experimenting with new AI agents and
harnesses. They ship constantly and differ in what they can actually do:
which models they use, which tools and MCP servers they bring, and whether
they work in a terminal or drive a full desktop and browser. The only way to
find the one that suits your work is to run it on your own machine.

An unfamiliar agent is unfamiliar code, and running several of them on one
computer creates recurring problems:

- A new agent runs with your shell access, your credentials, and your files
- Global installs collide over configuration, credentials, and runtime
  versions
- Agents compete for ports and shared state
- Removing an agent rarely removes everything it installed

Launcher makes each agent easy to try and easy to discard. Each one runs in
its own space and can only reach the workspace you give it. Install an agent,
run it on real work, and remove it completely if it does not suit you. The
agents you keep stay installed side by side, each with its own workspace and
history.

## Why Launcher

- **Try anything, keep what works** - Install an agent, run it on real work,
  and remove it completely if it does not suit you. The ones you keep hold on
  to their own workspace and history.
- **Discover what is out there** - Browse a catalogue of agents and harnesses
  in one place instead of hunting down install instructions for each one.
- **Contained by default** - Each agent is isolated from your system and from
  the others, and is reachable only from your own machine.
- **Let agents work unattended** - Because an agent can only reach its own
  space, you can let it work without approving every step, and check in when
  it has something to show you.
- **Several at once** - Keep agents side by side without them competing for
  configuration, credentials, ports, or runtime versions.
- **Local control** - Agents run on your own computer with no hosted control
  plane, and their files stay where you can reach them.
- **Nothing to wire up** - Launcher handles the runtime and the agent
  lifecycle, so installing an agent is one action rather than a setup guide.

## What you can do

Launcher provides a catalogue of packaged AI applications and a private
library of the agents installed on your computer. Each agent runs in its own
space, which makes a few things practical:

- Let an agent work on a task without approving every step
- Give an agent its own browser and desktop, with its own logged-in sessions
- Run the same task through different agents and compare the results
- Keep separate agents for separate projects, each with its own workspace,
  credentials, and history

Managing them is the ordinary part:

- Browse available applications
- Install multiple local agent instances
- Start, stop, update, duplicate, rename, and delete agents
- Inspect an agent's files, mounts, local interfaces, isolated network, IP
  addresses, and live resource usage from the CLI
- Update through a health-checked candidate while retaining the previous
  runtime for rollback, and recover a missing runtime from its stored manifest
- Open an agent's local interface
- Save a running agent's live preview image
- Inspect logs and runtime health
- Execute commands inside running agents
- Clean up old local images previously pulled by Launcher

Each application controls its own experience and capabilities. Launcher
provides the consistent local installation and management layer around it.

Run `launcher guide` for the self-contained, agent-readable tutorial embedded
in the installed binary.

## Supported platforms

Published releases currently support:

- Apple silicon Macs running macOS 26
- Linux x86-64 and ARM64 systems

Launcher uses Apple `container` on macOS and Docker on Linux. If the required
runtime is missing, the application guides you to the official installer.

## Get started

1. [Download the latest release](https://github.com/pdparchitect/launcher/releases/latest)
   for your platform.
2. Extract the archive and open Launcher.
3. Follow the runtime setup if prompted.
4. Browse the Marketplace and deploy an agent.

## Documentation

- [Using Launcher](docs/using-launcher.md)
- [Serving the web interface](docs/web-interface.md)
- [Developing Launcher](docs/development.md)
- [Application registry](docs/application-registry.md)
- [Container image sources](images/README.md)
- [Release notes](CHANGELOG.md)
- [Releasing Launcher](RELEASES.md)

Bug reports and ideas are welcome in
[GitHub Issues](https://github.com/pdparchitect/launcher/issues).
