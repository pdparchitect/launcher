function invoke(method, ...args) {
  const action = globalThis.runtime?.[method]

  if (typeof action !== 'function') {
    return false
  }

  action(...args)

  return true
}

export const desktopWindow = {
  available() {
    return typeof globalThis.runtime?.WindowMinimise === 'function'
  },

  minimise() {
    return invoke('WindowMinimise')
  },

  toggleMaximise() {
    return invoke('WindowToggleMaximise')
  },

  close() {
    return invoke('Quit')
  },

  openExternal(url) {
    if (invoke('BrowserOpenURL', url)) {
      return true
    }

    return Boolean(window.open(url, '_blank', 'noopener'))
  },

  async readClipboardText() {
    const readNativeClipboard = globalThis.runtime?.ClipboardGetText

    if (typeof readNativeClipboard === 'function') {
      return readNativeClipboard()
    }

    if (typeof globalThis.navigator?.clipboard?.readText === 'function') {
      return globalThis.navigator.clipboard.readText()
    }

    throw new Error('Clipboard access is unavailable')
  },
}
