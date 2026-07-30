---
name: promote-image-to-catalogue
description: Release a Launcher product image and promote its immutable published version into the application catalogue. Use for end-to-end requests to ship, mint, publish, or update a product such as Hermes, OpenClaw, or Codex Pets when the work spans both images/products and internal/catalog, or when checking whether a product change is fully released and available through the catalogue.
---

# Promote an image to the catalogue

Keep image publication and catalogue publication as two ordered releases.
Read `../manage-images/SKILL.md` and `../manage-catalogue/SKILL.md` completely
before changing either release unit.

## Establish release state

1. Identify the product target, its `VERSION` and `CHANGELOG.md`, and its
   manifest under `internal/catalog/manifests/`.
2. Run `make --directory images versions` and `bash images/tools/units.sh` to
   resolve the composite product and substrate version.
3. Inspect the latest matching product and `catalogue-v*` GitHub Releases.
   Treat a GitHub Release plus its resolved GHCR digest as publication proof;
   do not treat a Git tag alone as proof.
4. Preserve unrelated working-tree changes and report whether the task is at
   the image stage, the catalogue stage, or already complete.

## Stage 1: release the product image

1. Make the product source and deterministic test changes.
2. Bump the product's semantic `VERSION` and add the matching changelog section
   in the same change. Do not bump the root Launcher `VERSION`.
3. Run `make images-check`, then run the relevant build or smoke test when the
   environment supports it.
4. Confirm the intended immutable identity with `bash images/tools/units.sh`.
5. Merge to `main` only when authorized. Let **Images**, **Release images**, and
   **Publish images** create the tag, build both architectures, smoke-test, and
   publish the GitHub Release.
6. Wait for the GitHub Release and GHCR digest before starting Stage 2. If
   publication fails or remains incomplete, stop and diagnose it. Do not
   advance the catalogue.

## Stage 2: promote the published image

1. Update the product manifest to the exact published composite tag or digest.
   Never use `latest`.
2. Update tests that assert the manifest's image identity.
3. Inspect the latest `catalogue-v*` GitHub Release before editing
   `internal/catalog/catalogue.json`. If its declared version is unpublished,
   keep it for this snapshot. If that version is already published, declare
   the next unused semantic version; use a patch bump for an ordinary image
   pin update.
4. Do not change the root Launcher `VERSION`.
5. Run `make catalogue-package` and `make check`.
6. Merge to `main` only when authorized. Let **Release catalogue** create the
   tag and publish `launcher-catalogue.zip`.
7. Confirm the catalogue GitHub Release and bundle before reporting that the
   product is available. Launcher refreshes on startup, every 30 minutes, or
   immediately with `launcher catalog --refresh`.

## Guard concurrent releases

- Re-read release state immediately before each version change.
- Rebase or recompute the catalogue version if another catalogue release lands.
- Use the composite identity reported by `units.sh` after a substrate bump,
  even when the product's own version did not change.
- Never create Launcher, image, or catalogue release tags manually.
