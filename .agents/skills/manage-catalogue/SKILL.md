---
name: manage-catalogue
description: Manage Launcher application catalogue entries and independent catalogue releases. Use when adding or changing a manifest, application image reference, icon, cover, screenshot, catalogue metadata, catalogue validation, or the catalogue release workflow under internal/catalog.
---

# Manage the catalogue

Keep catalogue updates independent from Launcher binary releases. Treat every
published bundle as a complete, immutable snapshot.

For a product image release followed by a catalogue update, use
`promote-image-to-catalogue` to preserve the required ordering.

## Update an entry

1. Read `internal/catalog/catalog.go`, its tests, and the existing manifest
   closest to the requested application.
2. Edit the complete entry under `internal/catalog/manifests/<slug>/`.
3. Preserve an existing manifest UUID. It is the installed application's
   durable identity. Keep slugs unique and human-readable.
4. Use an immutable container image tag or digest. Never use `latest`.
5. Keep media paths relative to the manifest collection and include every
   referenced file. Track raster artwork with Git LFS.
6. Add or update validation tests when changing the schema or its rules.

## Version the snapshot

Before changing `internal/catalog/catalogue.json`, inspect the latest
`catalogue-v*` GitHub Release. Ensure the branch declares the next unused
semantic version whenever its catalogue differs from the latest published
snapshot.

Do not bump repeatedly for several changes that are intentionally part of the
same unpublished snapshot. Never reuse a version after its release exists.
Catalogue-only work does not change the root `VERSION`.

## Validate and release

Run:

```sh
make catalogue-package
make check
```

Merge the validated change to `main`. The **Release catalogue** workflow owns
tag creation and publishes `launcher-catalogue.zip`; do not create
`catalogue-v*` tags manually. Confirm the GitHub Release and its bundle before
calling the catalogue published.

Launcher checks for the new snapshot on startup and every 30 minutes while it
is running. `launcher catalog --refresh` bypasses the interval for a manual
check.
