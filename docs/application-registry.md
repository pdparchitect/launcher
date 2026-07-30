# Application registry

Launcher discovers applications through OCI artifacts in the same registries
that hold their container images. There is no separately versioned catalogue
snapshot.

Each publisher exposes a small feed artifact. Each feed lists stable
application channel references:

```json
{
  "schemaVersion": 1,
  "publisher": "PDP Architect",
  "applications": [
    "ghcr.io/pdparchitect/launcher-image-hermes-desktop:launcher-stable"
  ]
}
```

Launcher resolves the feeds listed in `internal/catalog/sources.json`, then
resolves each application channel independently. A failed application download
does not block updates from other applications. Launcher uses the last valid
cached copy for an unavailable feed or application.

Launcher refreshes on startup and every 30 minutes. Refresh immediately with:

```bash
launcher catalog --refresh
```

## Image-owned application bundle

An image repository publishes two application tags:

- `launcher-<image-version>` is immutable.
- `launcher-stable` points to the latest stable application artifact.

The application artifact uses
`application/vnd.pdparchitect.launcher.application.v1` and has the final
multi-architecture image manifest as its OCI subject. Its single ZIP layer uses
`application/vnd.pdparchitect.launcher.application.bundle.v1+zip` and contains
`application.json` plus its artwork.

Launcher derives the runnable image as `<repository>@<subject-digest>`.
`application.json` must not declare an image reference. This makes it
impossible for the metadata and published image to drift apart.

For Launcher-owned products, files live under:

```text
images/products/<product>/desktop/launcher/
  application.json
  icon.svg
  screenshot.png
```

An independently released image repository uses the equivalent root-level
`launcher/` directory.

## Application document

The document has a durable UUID, a human-readable slug, presentation data, and
runtime configuration:

```json
{
  "schemaVersion": 2,
  "id": "f726241a-ff31-423d-92ad-f2b43cca742f",
  "slug": "hermes",
  "name": "Hermes",
  "publisher": "Nous Research",
  "description": "A persistent browser desktop for running Hermes locally.",
  "tags": ["ASSISTANT", "AUTOMATION", "TERMINAL"],
  "media": {
    "icon": "icon.png",
    "cover": "screenshot.png",
    "screenshots": [
      {
        "source": "screenshot.png",
        "alt": "Hermes running in its persistent desktop"
      }
    ]
  },
  "interfaces": {
    "desktop": {
      "kind": "kasmweb",
      "port": 6901,
      "path": "/"
    }
  },
  "memory": "4g",
  "sharedMemory": "1g",
  "environment": {},
  "mounts": [
    {
      "name": "workspace",
      "target": "/workspace"
    }
  ]
}
```

Never change an existing UUID. Installed agent records use it as their durable
identity. Slugs must also be unique across all configured publisher feeds.
Media paths are relative to the owning `launcher/` directory.

`interfaces` maps application-defined interface IDs to the HTTP contracts
exposed by the container. IDs such as `desktop`, `agent`, or `filesystem` are
stable names within one application. `kind` selects the Launcher integration;
the initial kinds are `web`, `kasmweb`, `acp`, and `mcp`. Several interfaces
may use the same kind, port, or both. Launcher publishes each distinct
container port once and resolves every interface to its own local URL using
the declared path.

## Publish an application update

Application metadata ships with its image release:

1. Change the image source and its `launcher/application.json` or artwork.
2. Bump the product `VERSION` and add the matching `CHANGELOG.md` section.
3. Run the image checks and merge the release change to `main`.
4. Let the image release workflow build and smoke-test both architectures,
   create the multi-architecture image, attach the application bundle, and move
   `launcher-stable`.
5. Confirm the GitHub Release, image digest, immutable application tag, and
   stable application channel before calling the application published.

There is no second catalogue bump or catalogue release.

## Add or remove an application

Edit `publisher/feed.json` only when an application enters or leaves this
publisher's discovery set. The **Publish application feed** workflow validates
the document and moves `ghcr.io/pdparchitect/launcher-feed:stable`. Dispatch it
manually after verifying every newly listed application channel.

Adding another publisher requires adding its stable feed reference to
`internal/catalog/sources.json` and releasing a new Launcher binary. Normal
application updates do not change that bootstrap list.

## Initial migration order

The first deployment must seed the registry before Launcher 0.4.0 is released:

1. Publish the application-artifact release prepared for every current
   product. Each product receives a new patch version so its existing release
   identity is never reused.
2. Verify every `launcher-stable` application channel and its image subject.
3. Publish `publisher/feed.json`.
4. Verify `ghcr.io/pdparchitect/launcher-feed:stable`.
5. Publish Launcher 0.4.0.

Publish the feed last so a fresh Launcher never discovers a channel that has
not been seeded yet.

## Validation and trust

Launcher verifies OCI descriptor digests through the registry client, bounds
manifest and layer sizes, rejects unsafe archive paths and symbolic links,
strictly validates registry sources and publisher feeds, requires known
application fields to pass runtime and media validation while ignoring unknown
application metadata, requires the application artifact to have an image
subject, validates all referenced artwork, and rejects duplicate UUIDs or slugs
before activating a snapshot. Cache state and blobs are written atomically.

Raster artwork uses Git LFS. Release workflows must check out LFS content
before assembling application bundles.
