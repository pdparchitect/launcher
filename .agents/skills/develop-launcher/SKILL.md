---
name: develop-launcher
description: Develop and validate the Launcher application across its Go CLI, service, HTTP API, embedded web interface, Wails desktop shell, and native macOS integration. Use for Launcher behavior, UI, runtime, persistence, packaging, or binary build changes outside catalogue entries and container image sources.
---

# Develop Launcher

Launcher is one Go executable. Its CLI, web server, and desktop application
share `internal/agent`; Docker and Apple `container` are hidden behind
`internal/runtime`.

## Implement a change

1. Trace the behavior through the shared service before changing a surface.
   Avoid implementing separate CLI, HTTP, and desktop rules.
2. Follow the existing package boundaries and return contextual Go errors.
3. Keep the web application dependency-free. Files under
   `internal/httpapi/web/` are embedded directly and need no frontend build.
4. Preserve the Wails and SwiftUI ownership boundary when changing macOS code.
   The macOS application targets Apple silicon and macOS 26.
5. Add focused Go tests for behavior changes and regression fixes.
6. Run `gofmt` on changed Go files and finish with `make check`.

Use `make web` for browser development and `make desktop` for native
development. Use the narrowest relevant build target from `make help` when
packaging behavior changes.

## Choose the correct version

The root `VERSION` belongs only to Launcher binaries. Do not change it for
catalogue-only or image-only work. Use `manage-catalogue` for
`internal/catalog/manifests/` and `manage-images` for `images/`.
