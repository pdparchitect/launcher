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

// The window shape the panel is laid out against. macOS rounds the window
// itself, so this is never applied to anything - it exists purely so the
// panel's own corners can be derived from it below. 26 is a community
// measurement of the Tahoe toolbar-window corner, not a published Apple value;
// adjust it if the panel's corners look wrong against the window's.
const WINDOW_CORNER_RADIUS = 26

// Concentric corners: an inner shape nests inside an outer one when its radius
// is the outer radius minus the gap between them. Nesting a fixed radius
// inside a different one is what makes panels look subtly wrong.
const SIDEBAR_CORNER_RADIUS = Math.max(4, WINDOW_CORNER_RADIUS - SIDEBAR_INSET)

export const SIDEBAR_COLUMN_WIDTH = SIDEBAR_WIDTH + SIDEBAR_INSET * 2

// The panel's own geometry, so a browser preview can mirror it from one source
// rather than repeating the numbers in CSS.
export const SIDEBAR_METRICS = {
  inset: SIDEBAR_INSET,
  width: SIDEBAR_WIDTH,
  radius: SIDEBAR_CORNER_RADIUS,
  topInset: SIDEBAR_TOP_INSET,
  windowRadius: WINDOW_CORNER_RADIUS,
}

function messageHandler() {
  return globalThis.webkit?.messageHandlers?.external
}

function isMac() {
  const platform =
    globalThis.navigator?.platform || globalThis.navigator?.userAgent || ''

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
        topInset: SIDEBAR_TOP_INSET,
        selected,
        items,
      })}`
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
