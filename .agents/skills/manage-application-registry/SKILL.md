---
name: manage-application-registry
description: Manage Launcher OCI discovery sources, publisher feeds, image-owned application documents and artwork, registry validation, or cache behavior. Use when adding or removing a discoverable application, changing launcher/application.json, icons, screenshots, publisher/feed.json, internal/catalog/sources.json, OCI artifact schemas, or registry refresh behavior.
---

# Manage the application registry

Launcher uses OCI publisher feeds and image-owned application artifacts. It
does not publish a complete versioned catalogue snapshot.

For an application release, also read
`../publish-launcher-application/SKILL.md`. For a change to a Launcher-owned
product image, read `../manage-images/SKILL.md`.

## Locate ownership

- Launcher products own files under
  `images/products/<product>/desktop/launcher/`.
- Independently released image repositories own a root `launcher/` directory.
- The PDP Architect discovery list is `publisher/feed.json`.
- Launcher bootstrap feed references are in `internal/catalog/sources.json`.

## Change an application document

1. Read the owning `VERSION`, `CHANGELOG.md`, `launcher/application.json`, and
   its release workflow.
2. Preserve an existing UUID. Installed application records use it as their
   durable identity.
3. Keep the slug unique and preserve it unless an explicit rename is required.
4. Set `schemaVersion` to the supported schema. Do not add a `version` field;
   the owning product `VERSION` is authoritative.
5. Do not add an `image` field. Launcher derives
   `<repository>@<subject-digest>` from the OCI application artifact.
6. Keep media paths relative to the owning `launcher/` directory and ensure
   every referenced file exists. Track raster files with Git LFS.
7. Update schema, validation, collision, cache, or partial-failure tests when
   changing registry behavior.

An application change is part of the product image release. It does not bump
the root Launcher `VERSION` and has no separate catalogue version.

## Change discovery

Edit `publisher/feed.json` only to add or remove an application channel. Feed
entries point to stable application tags such as:

```text
ghcr.io/pdparchitect/launcher-image-hermes-desktop:launcher-stable
```

Manually dispatch **Publish application feed** after every newly listed
application channel is available. The workflow moves the publisher feed's
`stable` OCI tag after validation. A routine application release does not edit
the feed.

Adding a new publisher requires a stable feed artifact and a new entry in
`internal/catalog/sources.json`. Because sources are Launcher bootstrap
configuration, that change requires a Launcher binary release.

## Validate

Run:

```sh
go test ./internal/catalog
make check
```

For an independently released image repository, run its `make check`. Confirm
that release workflows check out Git LFS content and publish the application
artifact only after the final multi-architecture image digest exists.
