---
name: publish-launcher-application
description: Release, mint, publish, or update a Launcher application together with its container image. Use for products such as Hermes, OpenClaw, Petbox, Buzzbox, Buzznode, or Pantalk Ghost when source, product version, application metadata, OCI attachment publication, or release verification is involved.
---

# Publish a Launcher application

Read `../manage-images/SKILL.md` and
`../manage-application-registry/SKILL.md` completely before changing the
release unit.

## Establish release state

1. Identify the image release unit, its `VERSION`, `CHANGELOG.md`, and owning
   `launcher/application.json`.
2. For Launcher products, run `make --directory images versions` and
   `bash images/tools/units.sh` to resolve the composite image version.
3. Inspect the latest matching GitHub Release and GHCR tags and digests before
   choosing a version. A Git tag alone is not proof of publication.
4. Preserve unrelated working-tree changes and compare upstream source state
   when the product packages a third-party project.

## Prepare one release

1. Make the image source and deterministic test changes.
2. Update `launcher/application.json` and artwork in the owning image project.
   Preserve its UUID and omit the `image` field.
3. Bump the product semantic `VERSION`.
4. Add the matching `CHANGELOG.md` section.
5. Do not add a version to `application.json`; `VERSION` is the product's
   single version source.
6. Do not bump the root Launcher `VERSION`.
7. Run the owning image checks and relevant build or smoke test.

## Publish and verify

Merge to `main` only when authorized. Do not create release tags manually. The
image release workflows must perform this order:

1. Build and smoke-test native AMD64 and ARM64 images.
2. Assemble the final multi-architecture image and resolve its digest.
3. Package `application.json` and its artwork.
4. Attach the application artifact to the image digest as its OCI subject.
5. Tag the artifact `launcher-<image-version>`.
6. Move `launcher-stable` only for a stable release.
7. Create the GitHub Release that records the image digest.

Confirm all of the following before reporting the application published:

- The expected GitHub Release exists.
- The immutable image tag resolves to the recorded digest.
- The immutable `launcher-<image-version>` application tag exists.
- `launcher-stable` resolves to that application artifact for a stable release.
- The application artifact subject is the final multi-architecture image
  digest.

There is no second catalogue promotion. `publisher/feed.json` changes only
when adding or removing the application from discovery.

## Guard concurrent releases

- Re-read upstream, tags, and release state immediately before a version bump.
- Recompute the composite identity after any substrate change.
- Never reuse a published product version or immutable application tag.
- Do not move `launcher-stable` for a prerelease.
