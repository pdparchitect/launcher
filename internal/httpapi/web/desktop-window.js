function invoke(method) {
  const action = globalThis.runtime?.[method]

  if (typeof action !== 'function') {
    return false
  }

  action()
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
}
