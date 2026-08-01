# Serving the web interface

Launcher is a single Go binary. The desktop application is one front end for
it; the same service also serves a complete web interface over HTTP. Running
that server is an excellent option if you prefer to build a server rather
than run agents on a laptop: install Launcher on the machine, start the
service, and drive the whole agent library from a browser.

```bash
launcher serve
```

The command starts the HTTP service, prints the address, and opens a browser
window. The interface it serves is the same one the desktop application uses:
the marketplace, installation, start and stop controls, logs, runtime health,
and access to each agent's own interface.

## Why run it as a server

Agents are containers, and containers like machines that stay on. A dedicated
host suits the way agents actually get used:

- **Agents keep working while you do not.** A long task continues after you
  close the lid on your laptop, and the results are waiting the next time you
  open the interface.
- **The hardware matches the workload.** A mini PC, a workstation under a
  desk, or a virtual machine can hold more agents than a laptop, with more
  memory and disk for the desktop-and-browser images.
- **Any device becomes a client.** A browser is the only requirement, so a
  tablet or a second computer reaches the same library.
- **One place for the data.** Workspaces, credentials, and history live on
  the server rather than being spread across the machines you work from.

The desktop application remains the better choice on a machine you sit in
front of. The server is for the machine you leave running.

## Starting the service

```bash
launcher serve --listen 127.0.0.1:16900 --no-open
```

- `--listen` sets the address. It defaults to `127.0.0.1:16900`.
- `--no-open` skips the browser. Use it on a headless machine, where
  launching a browser would otherwise fail and stop the server.

Launcher accepts only loopback listen addresses. `--listen 0.0.0.0:16900` is
rejected with an error rather than binding a public port. Reaching the
interface from another machine is therefore always done through something in
front of it: a reverse proxy, a tunnel, or an SSH port forward.

The quickest remote access, with nothing to install on the server:

```bash
ssh -N -L 16900:127.0.0.1:16900 user@server
```

The interface is then at `http://127.0.0.1:16900` on your own machine.

## Running it as a system service

Run Launcher as the ordinary user that owns the container runtime and its
data directory, not as root. A minimal systemd user service:

```ini
# ~/.config/systemd/user/launcher.service
[Unit]
Description=Launcher web interface
After=network-online.target

[Service]
ExecStart=/usr/local/bin/launcher serve --listen 127.0.0.1:16900 --no-open
Restart=on-failure

[Install]
WantedBy=default.target
```

```bash
systemctl --user enable --now launcher.service
loginctl enable-linger "$USER"
```

`enable-linger` keeps the service running when you are not logged in.
Launcher stores its data under `$XDG_DATA_HOME/launcher`, or
`~/.local/share/launcher`, for that user.

## Reaching agent interfaces

Launcher publishes each agent's interface on its own loopback port on the
host, and the web interface links to it as `http://127.0.0.1:<port>`. That
URL resolves in the browser you are using, so opening an agent works when the
browser and the agents are on the same machine, or when your tunnel or proxy
maps the same port.

When working over SSH, forward the agent ports alongside the interface:

```bash
ssh -N -L 16900:127.0.0.1:16900 -L 17123:127.0.0.1:17123 user@server
```

`launcher list` and the agent's card both show the port an agent uses. With a
reverse proxy, publish each agent port you want to reach under its own
hostname or route.

## Differences from the desktop application

The web interface covers the same agent management, with a few desktop-only
behaviours:

- Agents open in a browser tab instead of an independent framed window.
- Opening an agent's files acts on the machine running the service, so it has
  no effect over a remote session on a headless host.
- Launcher update notices link to the release page; installing an update
  means replacing the binary on the server.

## Authentication

**The web interface has no authentication, and that is by design.** Anyone
who can reach the port can manage every agent on the host.

Launcher issues a random session token per process and embeds it in the page
it serves; the API requires that token and checks the request origin. Those
checks stop another site in the same browser from driving your Launcher, and
that is all they are for. They are not a login: the token is handed to
whoever loads the page. Restarting the service issues a new token, and open
tabs recover by reloading.

Launcher does not implement accounts because a local-first tool should not
own an identity system, and a hand-rolled one would be worse than what you
already run. The loopback-only listener is what keeps the default safe.

### Put a zero trust proxy in front of it

For any access beyond loopback and SSH forwarding, we recommend running the
interface behind a zero trust proxy connected to a social or corporate
identity provider — Google, GitHub, Microsoft Entra, or your own SSO. The
proxy authenticates the request before it ever reaches Launcher, and you
inherit the protections that provider already offers: two-factor
authentication, passkeys, device and location policy, session expiry,
group-based access, revocation, and an audit trail.

Options that work well for a single host include Cloudflare Access with a
tunnel, Tailscale on a private network, Pomerium, oauth2-proxy in front of
nginx or Caddy, and Authelia or Authentik. Any of them is a better answer
than a password Launcher would have to store.

A workable arrangement:

- Launcher listens on `127.0.0.1:16900` and is never published directly.
- The proxy terminates TLS, authenticates against your identity provider,
  and forwards to that loopback address.
- Access is restricted to named accounts or a group, not to anyone holding
  the link.
- Each agent interface you want to reach is published the same way, through
  the same policy. An agent port left open bypasses the proxy entirely.
- The host firewall allows only the proxy's inbound port.

Treat access to the interface as access to the host: an agent can be given a
workspace, credentials, and a browser, so whoever reaches Launcher can create
one that acts on your behalf.
