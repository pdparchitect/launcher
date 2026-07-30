# Releasing Launcher

This document describes how to version and publish Launcher binaries.

## Overview

Releases are driven by the root **`VERSION` file**. Bumping it on `main`
starts the automated pipeline that validates Launcher, builds both platform
packages, signs and notarizes the macOS application, creates the matching tag,
and publishes a GitHub Release:

1. Update `VERSION` with a semantic version without a `v` prefix.
2. Add the matching version section to `CHANGELOG.md`.
3. Merge both changes to `main`.
4. The **Release Launcher** workflow validates the commit and release metadata.
5. After both packages pass validation, the workflow creates the annotated
   `v<version>` tag and publishes the packages, checksums, and changelog notes.

Existing tags and published releases are immutable. Do not create the version
tag manually.

Launcher binaries and container image products have independent versions.
Image-owned Launcher application metadata ships with the corresponding product
release. This document covers Launcher binaries only.

## Published platforms

| OS    | Architecture | Package |
| ----- | ------------ | ------- |
| Linux | x86-64       | `.tar.gz` |
| macOS | arm64        | `.zip` |

The macOS package is signed with a Developer ID Application identity,
notarized by Apple, stapled, and checked with Gatekeeper before publication.

## Version source

`VERSION` is the only source of the Launcher version. The macOS build also
writes it into the application bundle metadata.

Use [Semantic Versioning](https://semver.org/). Keep `VERSION` bare
(`0.3.0`, not `v0.3.0`); GitHub tags add the `v` prefix. Pre-release versions
may use a suffix such as `0.3.0-beta.1`.

The workflow can be dispatched manually from `main` to retry a release that
failed before its GitHub Release was created. A published version cannot be
reused for another commit.

## Configure macOS signing

The macOS job runs in the protected GitHub environment `release`. It needs an
exportable Developer ID Application identity and an App Store Connect team API
key for notarization.

### 1. Verify the Developer ID identity

On the Mac containing the certificate, run:

```bash
security find-identity -v -p codesigning
```

Confirm that the output contains an identity like:

```text
Developer ID Application: Example Company (A1B2C3D4E5)
```

The value in parentheses is the Apple Team ID. The workflow uses it as
`APPLE_TEAM_ID`.

In Keychain Access, open the **login** keychain and select **My Certificates**.
Expand the Developer ID Application certificate and verify that a private key
appears below it. A certificate without its private key cannot be used by CI.

### 2. Export the signing identity

In Keychain Access:

1. Select the Developer ID Application certificate under **My Certificates**.
2. Choose **File > Export Items**.
3. Select the Personal Information Exchange format (`.p12`).
4. Save it as `LauncherDeveloperID.p12`.
5. Give the export a strong password and store that password securely.
6. Enter the Mac keychain password when Keychain Access asks for permission.

The export must contain both the certificate and its private key. Do not use a
`.cer` export, because it does not contain the private key.

The base64 encoding of this file becomes
`MACOS_CERTIFICATE_P12_BASE64`. Its export password becomes
`MACOS_CERTIFICATE_PASSWORD`.

### 3. Create the notarization key

In [App Store Connect](https://appstoreconnect.apple.com/):

1. Open **Users and Access > Integrations**.
2. Request App Store Connect API access if the account has not enabled it.
3. Open **Team Keys**.
4. Generate a key named `Launcher Notarization`.
5. Give it the Developer role.
6. Download the `AuthKey_<KEY_ID>.p8` file.
7. Record the Key ID shown for the key.
8. Record the Issuer ID shown on the Integrations page.

The Account Holder must request initial API access. An Account Holder or Admin
can generate a team key. Apple permits the `.p8` file to be downloaded only
once, so keep the original in a secure credential store.

The file becomes `APPLE_NOTARY_KEY_P8_BASE64`. The two recorded identifiers
become `APPLE_NOTARY_KEY_ID` and `APPLE_NOTARY_ISSUER_ID`.

Before uploading anything to GitHub, validate the notary credentials:

```bash
NOTARY_KEY_FILE="/secure/path/AuthKey_KEY_ID.p8"
NOTARY_KEY_ID="KEY_ID"
NOTARY_ISSUER_ID="ISSUER_ID"

xcrun notarytool history \
  --key "$NOTARY_KEY_FILE" \
  --key-id "$NOTARY_KEY_ID" \
  --issuer "$NOTARY_ISSUER_ID"
```

An empty history is valid. An authentication error means the key, Key ID,
Issuer ID, or assigned access is incorrect.

### 4. Create the GitHub environment

In the `pdparchitect/launcher` repository:

1. Open **Settings > Environments**.
2. Create an environment named exactly `release`.
3. Under deployment branches and tags, allow only the `main` branch.
4. Optionally add a required reviewer if every release should require approval.

The repository-wide GitHub Actions permission can remain read-only. The release
workflow grants `contents: write` only to the final job that creates the tag
and GitHub Release.

### 5. Add the environment secrets and variables

Add these secrets to the `release` environment:

```text
MACOS_CERTIFICATE_P12_BASE64
MACOS_CERTIFICATE_PASSWORD
APPLE_NOTARY_KEY_P8_BASE64
```

Add these environment variables:

```text
APPLE_NOTARY_KEY_ID
APPLE_NOTARY_ISSUER_ID
APPLE_TEAM_ID
```

The certificate, its password, and the notary private key are credentials and
must be secrets. The Key ID, Issuer ID, and Team ID are public identifiers and
can be variables.

They can be added one at a time through **Settings > Environments > release**.
To avoid copying private key material through the clipboard, use the GitHub CLI
instead:

```bash
LAUNCHER_REPOSITORY="pdparchitect/launcher"
RELEASE_ENVIRONMENT="release"
CERTIFICATE_FILE="/secure/path/LauncherDeveloperID.p12"
NOTARY_KEY_FILE="/secure/path/AuthKey_KEY_ID.p8"
NOTARY_KEY_ID="KEY_ID"
NOTARY_ISSUER_ID="ISSUER_ID"
APPLE_TEAM_ID="A1B2C3D4E5"

base64 -i "$CERTIFICATE_FILE" |
  tr -d '\n' |
  gh secret set MACOS_CERTIFICATE_P12_BASE64 \
    --repo "$LAUNCHER_REPOSITORY" \
    --env "$RELEASE_ENVIRONMENT"

printf 'PKCS #12 export password: '
IFS= read -r -s P12_PASSWORD
printf '\n'
printf '%s' "$P12_PASSWORD" |
  gh secret set MACOS_CERTIFICATE_PASSWORD \
    --repo "$LAUNCHER_REPOSITORY" \
    --env "$RELEASE_ENVIRONMENT"
unset P12_PASSWORD

base64 -i "$NOTARY_KEY_FILE" |
  tr -d '\n' |
  gh secret set APPLE_NOTARY_KEY_P8_BASE64 \
    --repo "$LAUNCHER_REPOSITORY" \
    --env "$RELEASE_ENVIRONMENT"

printf '%s' "$NOTARY_KEY_ID" |
  gh variable set APPLE_NOTARY_KEY_ID \
    --repo "$LAUNCHER_REPOSITORY" \
    --env "$RELEASE_ENVIRONMENT"

printf '%s' "$NOTARY_ISSUER_ID" |
  gh variable set APPLE_NOTARY_ISSUER_ID \
    --repo "$LAUNCHER_REPOSITORY" \
    --env "$RELEASE_ENVIRONMENT"

printf '%s' "$APPLE_TEAM_ID" |
  gh variable set APPLE_TEAM_ID \
    --repo "$LAUNCHER_REPOSITORY" \
    --env "$RELEASE_ENVIRONMENT"
```

Authenticate the GitHub CLI with `gh auth login` first if necessary. Confirm
that all names exist:

```bash
gh secret list \
  --repo pdparchitect/launcher \
  --env release

gh variable list \
  --repo pdparchitect/launcher \
  --env release
```

The workflow imports the certificate and key into temporary files and a
temporary keychain on the GitHub-hosted macOS runner. It removes them after the
job.

## Release guarantees

No version tag or GitHub Release is created until both platform packages have
been built and the macOS application has passed:

- Developer ID signature validation
- Hardened runtime and secure timestamp checks
- Apple notarization
- Ticket stapling and validation
- Gatekeeper assessment

The regular **Build** workflow still produces temporary development artifacts.
Only assets attached to a versioned GitHub Release have passed the complete
signing and notarization process.
