# Application catalogue

Launcher uses a versioned application catalogue for agent metadata, container
configuration, icons, covers, and screenshots.

## Loading and caching

The executable includes an embedded catalogue as an offline fallback. Launcher
also checks this repository for independently published catalogue releases. A
valid release is cached in the Launcher data folder and becomes the active
source for the command-line interface, HTTP API, Marketplace, and artwork.

A missing network connection or rejected download leaves the last valid
catalogue active. Launcher checks in the background when it starts and every
30 minutes while it is running. Refresh it immediately with:

```bash
launcher catalog --refresh
```

## Manifest structure

Each application has a directory under
`internal/catalog/manifests/<slug>/`. It contains `manifest.json` and the
application's media.

Every manifest has a permanent UUID identity and a separate human-readable
slug:

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
  "image": "ghcr.io/pantalk/ghost:0.0.10"
}
```

The directory name is an organizational convention. It does not define the
UUID or slug.

The UUID is stored in installed agent records and must never change. People use
the slug in commands such as:

```bash
launcher create --app pantalk-ghost Ada
```

A slug can be renamed while retaining the same UUID.

## Validation and security

Media paths are validated as safe relative image paths, and every referenced
file must exist in the complete catalogue snapshot. Release downloads are:

- Bounded in size
- Checked against the SHA-256 digest reported by GitHub
- Validated as a complete snapshot
- Written atomically
- Activated only after every entry and asset passes validation

Raster catalogue artwork is tracked with Git LFS. Install and fetch LFS objects
before contributing artwork:

```bash
git lfs install
git lfs pull
```

CI verifies that embedded raster files contain real image data rather than Git
LFS pointer files.

## Publish a catalogue version

Catalogue releases use `catalogue-v<version>` tags and attach one
`launcher-catalogue.zip` bundle.

1. Change the manifests or artwork.
2. Bump `version` in `internal/catalog/catalogue.json`.
3. Merge the change to `main`.

The **Release catalogue** workflow validates the entries, creates the tag, and
publishes the bundle without rebuilding Launcher. Do not reuse a published
catalogue version or create the tag manually.

Create the same package locally with:

```bash
make catalogue-package
```
