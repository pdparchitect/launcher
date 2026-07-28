// Bridge to the native macOS sidebar added by patches/wails/002-macos-native-sidebar.patch.
//
// The native side only exists in patched macOS builds. Everywhere else the
// configure call is a no-op and the HTML sidebar stays in place, so callers
// must wait for onReady before hiding their own navigation.

// The panel floats inside the window rather than filling its height, so the
// interface must reserve inset + width + inset for it.
const SIDEBAR_WIDTH = 250
const SIDEBAR_INSET = 8
const SIDEBAR_CORNER_RADIUS = 14
const SIDEBAR_TOP_INSET = 44

// macOS Tahoe's toolbar-style windows use roughly a 26pt continuous ("squircle")
// corner. Title-bar-only windows sit nearer 16pt.
const WINDOW_CORNER_RADIUS = 26

// The traffic lights are placed by the native side rather than left where
// AppKit puts them, so they sit comfortably inside the floating panel.
const TRAFFIC_LIGHT_X = 22
const TRAFFIC_LIGHT_Y = 26

// "glass" uses NSGlassEffectView on macOS 26 and falls back to vibrancy
// elsewhere. Set to "vibrancy" to force the fallback everywhere.
const SIDEBAR_MATERIAL = 'glass'

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
        windowCornerRadius: WINDOW_CORNER_RADIUS,
        topInset: SIDEBAR_TOP_INSET,
        trafficLightX: TRAFFIC_LIGHT_X,
        trafficLightY: TRAFFIC_LIGHT_Y,
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
