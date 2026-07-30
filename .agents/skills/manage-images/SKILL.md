---
name: manage-images
description: Manage Launcher container image sources, build graphs, tests, versions, and releases. Use when changing files under images, updating an application's packaged software, or publishing a substrate or product image release.
---

# Manage images

Image sources live under `images/`. The shared substrate and each product are
independent release units, and product release identities include the substrate
version they embed.

## Change an image

1. Identify the affected release unit. Use `images/VERSION` for the shared
   substrate or the product's own `VERSION` file.
2. Keep upstream dependencies pinned in Dockerfiles and preserve the declared
   parent linkage in `images/docker-bake.hcl`.
3. Update or add deterministic source checks and smoke-test coverage.
4. Bump the affected release unit's semantic version and add the matching
   section to its `CHANGELOG.md` in the same change.
5. Run `make images-check`. Run the relevant build or smoke test when the
   environment supports it.

Use `make --directory images versions` and
`bash images/tools/units.sh` to inspect the exact release identities.

## Release an image

Merge the version and changelog change to `main`. Do not create image tags
manually. After the **Images** workflow passes, **Release images** creates the
missing immutable tag and dispatches **Publish images**. Publishing builds and
smoke-tests native AMD64 and ARM64 images before assembling their manifest. A
product release then attaches its image-owned Launcher application bundle to
the final multi-architecture digest and moves its stable application channel.

Treat the GitHub Release and the resolved GHCR digest as proof of publication,
not the existence of a Git tag alone.

## Publish the Launcher application

Every product owns `launcher/application.json` and its artwork beside the
Dockerfile. Keep its version equal to the product `VERSION` and omit an image
field. The publish workflow derives the runnable image from the artifact's OCI
subject, so image and Launcher metadata ship as one release.

Use `publish-launcher-application` for end-to-end product releases and
`manage-application-registry` for application schema, artwork, feed, or cache
changes.
