// Bridge to the native macOS sidebar added by patches/wails/002-macos-native-sidebar.patch.
//
// The native side only exists in patched macOS builds. Everywhere else the
// configure call is a no-op and the HTML sidebar stays in place, so callers
// must wait for onReady before hiding their own navigation.

// The panel floats inside the window rather than filling its height, so the
// interface must reserve inset + width + inset for it.
const SIDEBAR_WIDTH = 250
const SIDEBAR_INSET = 8
const SIDEBAR_TOP_INSET = 44

// The window shape we lay out against. Recent macOS rounds windows itself and
// Apple advises against clipping that shape by hand, so this figure is mainly
// used for the concentric maths below. 26 is a community measurement of the
// Tahoe toolbar-window corner, not a published Apple value.
const WINDOW_CORNER_RADIUS = 26

// Off: built against the macOS 26 SDK the system rounds the window itself, so
// clipping it here only added a non-opaque window with a masked content layer
// — which showed as a seam along the top edge and let the desktop through
// behind the sidebar instead of the page. Apple advises against it anyway.
const SELF_CLIP_WINDOW = false

// Concentric corners: an inner shape nests inside an outer one when its radius
// is the outer radius minus the gap between them. Nesting a fixed radius
// inside a different one is what makes panels look subtly wrong.
const SIDEBAR_CORNER_RADIUS = Math.max(4, WINDOW_CORNER_RADIUS - SIDEBAR_INSET)

// NSGlassEffectView blurs what is behind the *window*, so the sidebar showed
// the desktop rather than the page underneath it. Vibrancy with within-window
// blending samples the webview's own content, which is the effect we want.
// Set to "glass" to try NSGlassEffectView again.
const SIDEBAR_MATERIAL = 'vibrancy'

export const SIDEBAR_COLUMN_WIDTH = SIDEBAR_WIDTH + SIDEBAR_INSET * 2

function messageHandler() {
  return globalThis.webkit?.messageHandlers?.external
}

function isMac() {
  const platform = globalThis.navigator?.platform || globalThis.navigator?.userAgent || ''

  return /mac/i.test(platform)
}

export const nativeSidebar = {
  // True only once the native side has confirmed itself. Never true on Linux,
  // Windows, the browser, or an unpatched macOS build.
  get ready() {
    return globalThis.wailsNativeSidebar === true
  },

  available() {
    return isMac() && Boolean(messageHandler())
  },

  // items: [{id, title, symbol, tint} | {group}]
  configure(items, selected) {
    const handler = messageHandler()

    if (!handler || !isMac()) {
      return false
    }

    handler.postMessage(
      `sidebar:${JSON.stringify({
        width: SIDEBAR_WIDTH,
        inset: SIDEBAR_INSET,
        cornerRadius: SIDEBAR_CORNER_RADIUS,
        windowCornerRadius: SELF_CLIP_WINDOW ? WINDOW_CORNER_RADIUS : 0,
        topInset: SIDEBAR_TOP_INSET,
        material: SIDEBAR_MATERIAL,
        selected,
        items,
      })}`,
    )

    return true
  },

  // Re-sends the config purely to move the native selection.
  select(items, selected) {
    return this.configure(items, selected)
  },

  onReady(callback) {
    if (this.ready) {
      callback()

      return
    }

    globalThis.addEventListener('wails:sidebar-ready', () => callback(), {
      once: true,
    })
  },

  onSelect(callback) {
    globalThis.addEventListener('wails:sidebar', (event) => {
      const id = event.detail?.id

      if (id) {
        callback(id)
      }
    })
  },
}
