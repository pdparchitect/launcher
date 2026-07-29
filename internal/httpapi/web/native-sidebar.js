// Bridge to the SwiftUI NavigationSplitView that hosts Wails' own WKWebView.
//
// The message handler is installed directly on the existing WKWebView before
// this page loads. Everywhere else configure is a no-op and the HTML sidebar
// remains, so callers must wait for onReady before hiding their navigation.

// The panel floats inside the window rather than filling its height, so the
// interface must reserve inset + width + inset for it.
export const SIDEBAR_WIDTH = 250
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
  return globalThis.webkit?.messageHandlers?.launcherNative
}

function isMac() {
  const platform =
    globalThis.navigator?.platform || globalThis.navigator?.userAgent || ''

  return /mac/i.test(platform)
}

let windowDragBridgeInstalled = false

function installWindowDragBridge() {
  if (windowDragBridgeInstalled) {
    return
  }
  windowDragBridgeInstalled = true

  /*
   * Wails' built-in drag message retains its original NSWindow. The SwiftUI
   * shell reparents the WKWebView into a different window, so native builds
   * forward the same CSS-defined drag regions through launcherNative instead.
   *
   * Capture phase is intentional: it prevents Wails' bubble listener from
   * trying to drag the now-hidden bootstrap window.
   */
  globalThis.addEventListener(
    'mousedown',
    (event) => {
      const handler = messageHandler()

      if (
        !nativeSidebar.ready ||
        !handler ||
        event.button !== 0 ||
        event.detail !== 1 ||
        !(event.target instanceof Element)
      ) {
        return
      }

      const draggable = globalThis
        .getComputedStyle(event.target)
        .getPropertyValue('--wails-draggable')
        .trim()

      if (draggable !== 'drag') {
        return
      }

      event.preventDefault()
      event.stopImmediatePropagation()
      handler.postMessage({ action: 'dragWindow' })
    },
    { capture: true }
  )
}

export const nativeSidebar = {
  // True only once SwiftUI has accepted the page's configuration.
  get ready() {
    return globalThis.wailsNativeSidebar === true
  },

  available() {
    return isMac() && Boolean(messageHandler())
  },

  // How far the native sidebar reaches across the window. The webview spans the
  // whole window and draws underneath it, so this is the page's own left inset.
  // Zero until SwiftUI has measured its layout, and again once the sidebar is
  // collapsed.
  get inset() {
    const inset = globalThis.wailsSidebarInset

    return typeof inset === 'number' ? inset : 0
  },

  onInset(callback) {
    globalThis.addEventListener('wails:sidebar-inset', (event) => {
      const inset = event.detail?.inset

      if (typeof inset === 'number') {
        callback(inset)
      }
    })
  },

  // items: [{id, title, symbol, tint} | {group}]
  configure(items, selected) {
    const handler = messageHandler()

    if (!handler || !isMac()) {
      return false
    }

    installWindowDragBridge()
    handler.postMessage({
      width: SIDEBAR_WIDTH,
      inset: SIDEBAR_INSET,
      cornerRadius: SIDEBAR_CORNER_RADIUS,
      topInset: SIDEBAR_TOP_INSET,
      selected,
      items,
    })

    return true
  },

  // Re-sends the configuration purely to move the native selection.
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
