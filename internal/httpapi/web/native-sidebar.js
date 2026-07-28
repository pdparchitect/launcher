// Bridge to the native macOS sidebar added by patches/wails/002-macos-native-sidebar.patch.
//
// The native side only exists in patched macOS builds. Everywhere else the
// configure call is a no-op and the HTML sidebar stays in place, so callers
// must wait for onReady before hiding their own navigation.

const SIDEBAR_TOP_INSET = 52
const SIDEBAR_WIDTH = 250

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
        topInset: SIDEBAR_TOP_INSET,
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
