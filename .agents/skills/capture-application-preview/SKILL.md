---
name: capture-application-preview
description: Obtain a screenshot from a running Launcher application's preview HTTP interface. Use when application.json declares an interface with kind preview and a current screenshot is needed.
---

# Capture an application preview

Read `launcher/application.json` and find the interface whose `kind` is
`preview`:

```json
"preview": {
  "kind": "preview",
  "port": 6902,
  "path": "/preview.jpg"
}
```

The port in `application.json` is the container port. From inside the
container, download the screenshot with:

```sh
curl --fail --silent --show-error \
  --retry 20 --retry-all-errors --retry-delay 1 \
  --output /tmp/application-preview.jpg \
  http://127.0.0.1:6902/preview.jpg
```

From the host, use the resolved preview URL returned for the running agent by
`GET /api/instances`:

```json
"preview": {
  "kind": "preview",
  "url": "http://127.0.0.1:<resolved-port>/preview.jpg"
}
```

Do not assume the container port is also the resolved host port. Download the
exact resolved URL, then inspect `/tmp/application-preview.jpg`.

- HTTP `503` means the graphical session is not ready; retry.
- Connection refused means the container is stopped, the port is not
  published, or the wrong host port was used.
- `/healthz` checks service health but does not return the screenshot.
- If no interface has `kind: "preview"`, the application does not expose this
  capability. Do not guess a URL from its display interface.
